// Package httpapi is the UI-facing API edge + the Evolution webhook ingress.
// Every /xchats/api/v1 response uses the unified {payload, errcode, message}
// envelope; ops and the webhook are the only routes outside it (per 7-api-contracts).
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/response"
)

// webhookTokenHeader is the header Evolution echoes back so we can verify it
// (header only — never a query param, so it never lands in access logs).
const webhookTokenHeader = "X-Webhook-Token"

// publishTimeout bounds every enqueue from an HTTP handler. queue.Publish blocks
// while the buffer is full; without a deadline that turns backpressure into a
// hung request. With one it becomes an error the handler can act on — the
// Telegram webhook answers 500 so Telegram redelivers, others log and continue.
const publishTimeout = 2 * time.Second

// kbInvalidator is the narrow slice of CachedKBRepo (internal/responsestore)
// the /kb/* live-write epilogue needs — declared inline (rather than importing
// responsestore here) so httpapi keeps its existing single dependency edge on
// backend/response, per this PR's "no new package edge" constraint.
type kbInvalidator interface {
	Invalidate(orgID uuid.UUID)
}

// Server wires the HTTP edges to their dependencies.
type Server struct {
	cfg      *config.Config
	store    *store.Store
	queue    queue.Queue
	hub      *realtime.Hub
	blob     blob.Store
	response *response.Service // the multichannel response engine's entry point (simulator API)
	evo      evolution.Client
	tg       telegram.Client // nil when Telegram is not configured; every handler checks
	kb       *kbstore.Store
	orgID    uuid.UUID
	log      *slog.Logger

	// MCP connector (plan/mcp.md). mcpAuth/mcpServer are nil when the
	// connector is not configured (no MCP_JWT_SIGNING_KEY resolvable at
	// boot) — every MCP route checks mcpAuthEnabled() first rather than
	// assuming these are always set, so a deployment that hasn't set up the
	// connector yet keeps serving everything else unaffected.
	mcpAuth         *mcpauth.Authorizer
	mcpServer       *mcpserver.Server
	mcpUploadSigner *mcpauth.UploadTokenSigner
	mcpMediaSigner  *mcpauth.MediaReadTokenSigner

	// kbRepo/kbInvalidator are the response engine's own KB reader — a
	// CachedKBRepo in production (main.go), the SAME cached build GET
	// /kb/prompt renders from and every /kb/* write invalidates. Distinct from
	// kb (*kbstore.Store) above: kb is the older, kbstore-domain live editor +
	// dormant Playground path; kbRepo reads the aiprompt.KB shape the response
	// engine and the Промпт tab both need (in_stock, delivery zones,
	// outside_zones_note — see internal/responsestore/kb.go's doc comment).
	kbRepo        response.KnowledgeBaseRepository
	kbInvalidator kbInvalidator

	// pendingNames carries the display_name from "add account" (POST) to the QR
	// connect step (where the wa_accounts row is finally written): pre-connect
	// there is no row to hold it. Keyed by instance name; best-effort (process-
	// local) — a lost entry just falls back to the instance name.
	pendingMu    sync.Mutex
	pendingNames map[string]string

	// csrfSecret HMAC-signs the /oauth/authorize/decision CSRF token (see
	// mcp_oauth_csrf.go) — cfg.SessionSecret when configured, else a
	// per-process random fallback generated once here. The fallback is safe
	// only for a single-replica deployment (a token minted on one replica
	// won't verify on another); Task 17 hardens missing secrets into a hard
	// startup failure for production generally.
	csrfSecret []byte

	// oauthRegisterLimit/oauthTokenLimit/oauthAuthorizeLimit bound abuse
	// against the OAuth surface's most exposed routes — see ratelimit.go.
	oauthRegisterLimit  *ipRateLimiter
	oauthTokenLimit     *ipRateLimiter
	oauthAuthorizeLimit *ipRateLimiter
}

// Deps is the constructor input.
type Deps struct {
	Cfg           *config.Config
	Store         *store.Store
	Queue         queue.Queue
	Hub           *realtime.Hub
	Blob          blob.Store
	Response      *response.Service
	Evo           evolution.Client
	TG            telegram.Client
	KB            *kbstore.Store
	KBRepo        response.KnowledgeBaseRepository
	KBInvalidator kbInvalidator
	OrgID         uuid.UUID
	Log           *slog.Logger

	// MCPAuth/MCPServer are nil to run without the MCP connector (every
	// route checks mcpAuthEnabled() first).
	MCPAuth   *mcpauth.Authorizer
	MCPServer *mcpserver.Server
}

// New builds a Server.
func New(d Deps) *Server {
	var uploadSigner *mcpauth.UploadTokenSigner
	var mediaSigner *mcpauth.MediaReadTokenSigner
	if d.MCPAuth != nil {
		uploadSigner = mcpauth.NewUploadTokenSigner(d.MCPAuth.Key)
		mediaSigner = mcpauth.NewMediaReadTokenSigner(d.MCPAuth.Key)
	}
	return &Server{
		cfg: d.Cfg, store: d.Store, queue: d.Queue, hub: d.Hub,
		blob: d.Blob, response: d.Response, evo: d.Evo, tg: d.TG, kb: d.KB,
		kbRepo: d.KBRepo, kbInvalidator: d.KBInvalidator,
		orgID: d.OrgID, log: d.Log,
		mcpAuth: d.MCPAuth, mcpServer: d.MCPServer,
		mcpUploadSigner: uploadSigner, mcpMediaSigner: mediaSigner,
		pendingNames: map[string]string{},
		csrfSecret:   randomCSRFFallbackSecret(),
		// Deliberately generous limits — these are abuse guards, not a
		// throttle on legitimate usage. See ratelimit.go's doc comment.
		oauthRegisterLimit:  newIPRateLimiter(5.0/60, 5),   // 5/min, 5 burst — registration spam is the highest-value target
		oauthTokenLimit:     newIPRateLimiter(30.0/60, 10), // 30/min, 10 burst — refresh-heavy legitimate usage needs headroom
		oauthAuthorizeLimit: newIPRateLimiter(20.0/60, 10), // 20/min, 10 burst
	}
}

// Router builds the gin engine with all routes mounted.
func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// Trust NO proxy unless explicitly configured (plan Task 17): gin's own
	// zero-value default trusts every peer's X-Forwarded-* headers, which
	// would let any direct, unproxied caller spoof its own client IP. Every
	// discovery URL this server emits is already built from cfg.APIBaseURL /
	// cfg.FrontendBaseURL, never from the request's Host or X-Forwarded-*
	// (see mcpIssuer/MCPResourceURL) — this only affects c.ClientIP(), used
	// by the rate limiters.
	if err := r.SetTrustedProxies(s.cfg.TrustedProxies); err != nil {
		panic("httpapi: invalid TRUSTED_PROXIES: " + err.Error())
	}
	r.Use(gin.Recovery(), s.requestLog(), s.cors())

	// Ops — unversioned, no envelope.
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/readyz", func(c *gin.Context) {
		if err := s.store.Ping(c.Request.Context()); err != nil {
			c.String(http.StatusServiceUnavailable, "not ready")
			return
		}
		c.String(http.StatusOK, "ready")
	})

	// Evolution webhook ingress.
	r.POST("/evolution/api/v1/webhook/:account_id", s.handleWebhook)
	r.POST("/evolution/api/v1/webhook/:account_id/*event", s.handleWebhook)

	// Telegram webhook ingress. Unlike Evolution's, this one commits the
	// normalized rows before it acks — see handleTelegramWebhook.
	r.POST("/telegram/api/v1/webhook/:account_id", s.handleTelegramWebhook)

	// MCP connector (plan/mcp.md) — discovery, OAuth 2.1 + PKCE, and the
	// JSON-RPC endpoint. Every handler here checks mcpAuthEnabled() itself,
	// so these routes stay registered (returning 503) even when the
	// connector has no signing key configured, rather than 404ing in a way
	// that would look like the feature doesn't exist at all.
	r.GET("/.well-known/oauth-protected-resource", s.handleProtectedResourceMetadata)
	// Path-suffixed discovery alias (2025-11-25 MCP auth spec: a host may
	// look for metadata at a URL derived from the resource's own path).
	r.GET("/.well-known/oauth-protected-resource/mcp", s.handleProtectedResourceMetadata)
	r.GET("/.well-known/oauth-authorization-server", s.handleAuthorizationServerMetadata)
	r.GET("/oauth/jwks.json", s.handleJWKS)
	r.GET("/oauth/authorize", rateLimit(s.oauthAuthorizeLimit), s.handleOAuthAuthorize)
	r.POST("/oauth/authorize/decision", rateLimit(s.oauthAuthorizeLimit), s.handleOAuthDecision)
	r.POST("/oauth/token", rateLimit(s.oauthTokenLimit), s.handleOAuthToken)
	r.POST("/oauth/revoke", s.handleOAuthRevoke)
	r.POST("/oauth/register", rateLimit(s.oauthRegisterLimit), s.handleOAuthRegister)
	// Task 15: redeem a tool result's one-time review-handoff URL — a
	// browser-navigated GET, like /oauth/authorize, so it renders an HTML
	// error page rather than a JSON envelope on failure.
	r.GET("/playground/review-handoff", s.handleReviewHandoff)
	r.POST("/mcp", s.handleMCP)
	// Streamable HTTP (plan Task 10): this server never pushes
	// server-initiated messages, so GET (the SSE-stream half of the
	// transport) is an explicit 405 rather than a stream with nothing to
	// send — still a spec-compliant response to the method, per the
	// transport's own "or reject with 405" allowance. DELETE acknowledges
	// session/transport close; see handleMCPDelete's doc for why that's a
	// no-op rather than state to clean up.
	r.GET("/mcp", s.handleMCPGet)
	r.DELETE("/mcp", s.handleMCPDelete)

	// The signed media-upload target kb_media_upload hands the widget: its
	// own permissive CORS (see uploadCORS's doc comment), since auth here is
	// the unguessable signed token, not a cookie.
	upload := r.Group("/mcp/uploads")
	upload.Use(s.uploadCORS())
	upload.PUT("/:material_id", s.handleMCPUpload)
	upload.OPTIONS("/:material_id", func(c *gin.Context) {}) // uploadCORS aborts+responds before this body runs

	// The signed media-READ target the widget renders previews from — same
	// reasoning and same shape as the upload group above, opposite direction.
	media := r.Group("/mcp/media")
	media.Use(s.mediaCORS())
	media.GET("/:material_id", s.handleMCPMediaRead)
	media.OPTIONS("/:material_id", func(c *gin.Context) {})

	api := r.Group("/xchats/api/v1")
	api.POST("/auth/login", s.handleLogin)
	api.POST("/auth/logout", s.handleLogout)

	auth := api.Group("")
	auth.Use(s.requireSession())
	auth.GET("/me", s.handleMe)
	auth.GET("/users", s.handleListUsers)
	auth.POST("/users", s.handleCreateUser)
	auth.GET("/organization", s.handleGetOrg)
	// Task 15: explicit active-organization switch (the frontend selector) —
	// distinct from the MCP review-handoff redirect below, which sets the
	// SAME session field via a verified signed token instead of a direct
	// user action.
	auth.POST("/organization/active", s.handleSetActiveOrganization)

	// Channel-neutral account listing — every channel in one shape. The
	// per-channel routes below own each channel's own lifecycle.
	auth.GET("/accounts", s.handleListAccounts)

	// WhatsApp accounts manager (Build 1).
	auth.GET("/whatsapp-accounts", s.handleListWhatsAppAccounts)
	auth.POST("/whatsapp-accounts", s.handleCreateWhatsAppAccount)
	auth.GET("/whatsapp-accounts/qr", s.handleWhatsAppAccountQR)
	auth.POST("/whatsapp-accounts/:id/reconnect", s.handleReconnectWhatsAppAccount)
	auth.DELETE("/whatsapp-accounts/:id", s.handleDeleteWhatsAppAccount)
	auth.GET("/whatsapp-instances", s.handleListInstances)
	auth.DELETE("/whatsapp-instances/:name", s.handleDeleteInstance)

	// Telegram accounts manager. A bot has no QR and no Evolution instance, so
	// it gets its own lifecycle routes rather than sharing /whatsapp-accounts.
	auth.POST("/telegram-accounts", s.handleCreateTelegramAccount)
	auth.POST("/telegram-accounts/:id/retry-webhook", s.handleRetryTelegramWebhook)
	auth.POST("/telegram-accounts/:id/check", s.handleCheckTelegramAccount)
	auth.PUT("/telegram-accounts/:id/token", s.handleReplaceTelegramToken)
	auth.DELETE("/telegram-accounts/:id", s.handleDeleteTelegramAccount)

	auth.GET("/chats", s.handleListChats)
	auth.POST("/chats", s.handleCreateChat)
	auth.GET("/chats/:id/messages", s.handleListMessages)
	auth.POST("/chats/:id/messages", s.handleSendMessage)
	auth.POST("/chats/:id/read", s.handleReadChat)
	auth.PATCH("/chats/:id/assignee", s.handleAssignChat)

	auth.GET("/chats/:id/ai-drafts", s.handleListDrafts)
	auth.POST("/chats/:id/ai-drafts", s.handleSuggest)
	auth.POST("/ai-drafts/:id/approve", s.handleApprove)

	auth.POST("/media", s.handleUploadMedia)
	auth.GET("/media/:id", s.handleServeMedia)
	auth.GET("/realtime", s.handleRealtime)

	// Simulator API (Phase 10) — gated: the route does not exist at all unless
	// explicitly enabled (SIMULATOR_ENABLED, default false outside dev).
	if s.cfg.SimulatorEnabled {
		auth.POST("/simulator/messages", s.handleSimulatorMessage)
	}

	// Playground — a draft copy of the structured knowledge base. Writes remain
	// pending until the operator approves them into the live KB.
	pg := auth.Group("/playground")
	pg.GET("/draft", s.handlePlaygroundDraft)
	pg.DELETE("/draft", s.handlePlaygroundDiscardDraft)
	pg.POST("/draft/topics", s.handlePlaygroundUpsertTopic)
	pg.DELETE("/draft/topics/:slug", s.handlePlaygroundDeleteTopic)
	pg.POST("/draft/tariffs", s.handlePlaygroundUpsertTariff)
	pg.DELETE("/draft/tariffs/:ref", s.handlePlaygroundDeleteTariff)
	pg.POST("/draft/products", s.handlePlaygroundUpsertProduct)
	pg.DELETE("/draft/products/:ref", s.handlePlaygroundDeleteProduct)
	pg.POST("/draft/zones", s.handlePlaygroundUpsertZone)
	pg.DELETE("/draft/zones/:ref", s.handlePlaygroundDeleteZone)
	pg.PATCH("/draft/contacts", s.handlePlaygroundPatchContacts)
	pg.PATCH("/draft/policies", s.handlePlaygroundPatchPolicies)
	pg.PATCH("/draft/config", s.handlePlaygroundPatchConfig)
	pg.POST("/draft/approve", s.handlePlaygroundApprove)
	pg.POST("/draft/approve/:kind/:id", s.handlePlaygroundApproveEntity)

	// KB — the live-only editor (/knowledge-base). Every write here lands
	// directly in the live ai_ tables: no draft blob, no approve step, so it
	// can never mix with Playground's pending work (see plan "Playground
	// redesign").
	auth.GET("/kb", s.handleKBGet)
	auth.GET("/kb/prompt", s.handleKBPrompt)
	kb := auth.Group("/kb")
	kb.POST("/topics", s.handleKBUpsertTopic)
	kb.DELETE("/topics/:slug", s.handleKBDeleteTopic)
	kb.POST("/tariffs", s.handleKBUpsertTariff)
	kb.DELETE("/tariffs/:ref", s.handleKBDeleteTariff)
	kb.POST("/products", s.handleKBUpsertProduct)
	kb.DELETE("/products/:ref", s.handleKBDeleteProduct)
	kb.PATCH("/contacts", s.handleKBPatchContacts)
	kb.PATCH("/policies", s.handleKBPatchPolicies)
	kb.POST("/zones", s.handleKBUpsertZone)
	kb.DELETE("/zones/:ref", s.handleKBDeleteZone)
	kb.GET("/materials", s.handleKBListMaterials)
	kb.PATCH("/config", s.handleKBPatchConfig)
	return r
}

// handleWebhook verifies the shared token (header only), enqueues the raw event,
// and returns 200 fast — no DB write at the edge.
func (s *Server) handleWebhook(c *gin.Context) {
	accountID := c.Param("account_id")
	if s.cfg.WebhookToken != "" {
		tok := c.GetHeader(webhookTokenHeader)
		if tok == "" {
			tok = c.GetHeader("apikey")
		}
		if subtle.ConstantTimeCompare([]byte(tok), []byte(s.cfg.WebhookToken)) != 1 {
			s.log.Warn("webhook auth rejected", "account_id", accountID, "reason", "bad token")
			fail(c, http.StatusUnauthorized, ErrWebhookUnauthorized, "bad webhook token")
			return
		}
	}
	raw, err := c.GetRawData()
	if err != nil {
		s.log.Warn("webhook unreadable body", "account_id", accountID, "err", err)
		fail(c, http.StatusBadRequest, ErrValidation, "unreadable body")
		return
	}
	cp := make([]byte, len(raw))
	copy(cp, raw)
	queued := s.publish(ctx(c), queue.Message{Kind: queue.KindWaEvent, Payload: cp}) == nil

	// Cheap top-level peek for the event type + instance (no full parse). The full
	// body is only logged at debug to keep customer PII out of info logs.
	var peek struct {
		Event    string `json:"event"`
		Instance string `json:"instance"`
	}
	_ = json.Unmarshal(raw, &peek)
	s.log.Info("webhook received",
		"account_id", accountID, "event", peek.Event, "instance", peek.Instance,
		"bytes", len(raw), "queued", queued)
	s.log.Debug("webhook body", "account_id", accountID, "body", string(raw))

	// Evolution's retry semantics are not Telegram's — a 5xx here would have it
	// hammer the same event — so a dropped enqueue stays a logged 200 (the
	// pre-existing Build-0 loss window), now at least visible in the log line.
	if !queued {
		s.log.Error("webhook event dropped: queue full", "account_id", accountID, "event", peek.Event)
	}
	ok(c, nil)
}

// --- middleware -----------------------------------------------------------

func (s *Server) cors() gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range s.cfg.CORSOrigins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		// /mcp/uploads carries its OWN, deliberately permissive CORS policy
		// (uploadCORS, in mcp_upload.go): the widget iframe posting there
		// lives on a per-app sandbox host — e.g.
		// https://asdk_app_<hash>.web-sandbox.oaiusercontent.com — whose name
		// is unpredictable, so it can never be listed in CORSOrigins. Bail out
		// BEFORE the OPTIONS branch below: that branch aborts the chain, so
		// without this the group-level uploadCORS would never run, and a
		// preflight would come back 204 but with no Access-Control-Allow-Origin
		// at all — which the browser treats as "denied" and silently drops the
		// real PUT (the server then only ever logs the OPTIONS).
		//
		// /mcp/media is the same story in the read direction (mediaCORS).
		// Kept as two explicit prefixes rather than a blanket "/mcp/" test —
		// that would silently hand a permissive policy to every future
		// /mcp/* route somebody adds.
		if strings.HasPrefix(c.Request.URL.Path, "/mcp/uploads") ||
			strings.HasPrefix(c.Request.URL.Path, "/mcp/media") {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin != "" && (allowed[origin] || allowed["*"]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			// If-Match: the Playground draft store's optimistic-concurrency
			// header (frontend/src/stores/playground.ts's ifMatch()) — every
			// draft write/approve call sends it whenever frontend and backend
			// are on different origins, this preflight must allow it or the
			// browser blocks the real request before it's ever sent.
			c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,If-Match,X-Webhook-Token")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *Server) requestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		s.log.Info("http request",
			"method", c.Request.Method, "path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds())
	}
}

// --- envelope helpers -----------------------------------------------------

type envelope struct {
	Payload any    `json:"payload"`
	Errcode string `json:"errcode"`
	Message string `json:"message"`
}

func ok(c *gin.Context, payload any)      { c.JSON(http.StatusOK, envelope{payload, ErrOK, ""}) }
func created(c *gin.Context, payload any) { c.JSON(http.StatusCreated, envelope{payload, ErrOK, ""}) }
func accepted(c *gin.Context, payload any) {
	c.JSON(http.StatusAccepted, envelope{payload, ErrOK, ""})
}
func fail(c *gin.Context, status int, code, msg string) {
	c.AbortWithStatusJSON(status, envelope{nil, code, msg})
}

type page struct {
	Items    any `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

func (s *Server) pageParams(c *gin.Context) (limit, offset, pageNum, pageSize int) {
	pageNum, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	if pageNum < 1 {
		pageNum = 1
	}
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(s.cfg.PageSize)))
	if pageSize < 1 || pageSize > 200 {
		pageSize = s.cfg.PageSize
	}
	return pageSize, (pageNum - 1) * pageSize, pageNum, pageSize
}

func parseUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

// ctx returns the request context.
func ctx(c *gin.Context) context.Context { return c.Request.Context() }

// publish enqueues with a bounded deadline (see publishTimeout).
func (s *Server) publish(parent context.Context, m queue.Message) error {
	pctx, cancel := context.WithTimeout(parent, publishTimeout)
	defer cancel()
	return s.queue.Publish(pctx, m)
}

// publishOrLog is the fire-and-forget form for paths that have already produced
// a durable row: a lost enqueue degrades a background nicety (a draft, a media
// download), it never invalidates the response we already committed.
func (s *Server) publishOrLog(parent context.Context, m queue.Message) {
	if err := s.publish(parent, m); err != nil {
		s.log.Error("enqueue failed", "kind", m.Kind, "err", err)
	}
}
