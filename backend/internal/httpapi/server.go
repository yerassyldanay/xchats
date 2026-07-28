// Package httpapi is the UI-facing API edge + the Evolution webhook ingress.
// Every /xchats/api/v1 response uses the unified {payload, errcode, message}
// envelope; ops and the webhook are the only routes outside it (per 7-api-contracts).
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/assistant"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/playground"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/response"
)

// webhookTokenHeader is the header Evolution echoes back so we can verify it
// (header only — never a query param, so it never lands in access logs).
const webhookTokenHeader = "X-Webhook-Token"

// Server wires the HTTP edges to their dependencies.
type Server struct {
	cfg      *config.Config
	store    *store.Store
	queue    queue.Queue
	hub      *realtime.Hub
	blob     blob.Store
	drafter  assistant.Drafter // dormant: only the disconnected playground hot-swap reads this
	response *response.Service // the multichannel response engine's entry point (simulator API)
	evo      evolution.Client
	kb       *kbstore.Store
	builder  *playground.Builder
	orgID    uuid.UUID
	log      *slog.Logger

	// pendingNames carries the display_name from "add account" (POST) to the QR
	// connect step (where the wa_accounts row is finally written): pre-connect
	// there is no row to hold it. Keyed by instance name; best-effort (process-
	// local) — a lost entry just falls back to the instance name.
	pendingMu    sync.Mutex
	pendingNames map[string]string
}

// Deps is the constructor input.
type Deps struct {
	Cfg      *config.Config
	Store    *store.Store
	Queue    queue.Queue
	Hub      *realtime.Hub
	Blob     blob.Store
	Drafter  assistant.Drafter
	Response *response.Service
	Evo      evolution.Client
	KB       *kbstore.Store
	Builder  *playground.Builder
	OrgID    uuid.UUID
	Log      *slog.Logger
}

// New builds a Server.
func New(d Deps) *Server {
	return &Server{
		cfg: d.Cfg, store: d.Store, queue: d.Queue, hub: d.Hub,
		blob: d.Blob, drafter: d.Drafter, response: d.Response, evo: d.Evo, kb: d.KB, builder: d.Builder,
		orgID: d.OrgID, log: d.Log,
		pendingNames: map[string]string{},
	}
}

// Router builds the gin engine with all routes mounted.
func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
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

	api := r.Group("/xchats/api/v1")
	api.POST("/auth/login", s.handleLogin)
	api.POST("/auth/logout", s.handleLogout)

	auth := api.Group("")
	auth.Use(s.requireSession())
	auth.GET("/me", s.handleMe)
	auth.GET("/users", s.handleListUsers)
	auth.POST("/users", s.handleCreateUser)
	auth.GET("/organization", s.handleGetOrg)

	// WhatsApp accounts manager (Build 1).
	auth.GET("/whatsapp-accounts", s.handleListWhatsAppAccounts)
	auth.POST("/whatsapp-accounts", s.handleCreateWhatsAppAccount)
	auth.GET("/whatsapp-accounts/qr", s.handleWhatsAppAccountQR)
	auth.POST("/whatsapp-accounts/:id/reconnect", s.handleReconnectWhatsAppAccount)
	auth.DELETE("/whatsapp-accounts/:id", s.handleDeleteWhatsAppAccount)
	auth.GET("/whatsapp-instances", s.handleListInstances)
	auth.DELETE("/whatsapp-instances/:name", s.handleDeleteInstance)

	auth.GET("/chats", s.handleListChats)
	auth.POST("/chats", s.handleCreateChat)
	auth.GET("/chats/:id/messages", s.handleListMessages)
	auth.POST("/chats/:id/messages", s.handleSendMessage)
	auth.POST("/chats/:id/read", s.handleReadChat)

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

	// Playground — the KB builder (chat → materials → draft blob → approve into
	// the live KB, 15 Decisions 3–4). No more open/publish/rollback: GET always
	// returns the merged view; approve is the only write path to live.
	pg := auth.Group("/playground")
	pg.GET("/draft", s.handlePlaygroundDraft)
	pg.DELETE("/draft", s.handlePlaygroundDiscardDraft)
	pg.POST("/draft/topics", s.handlePlaygroundUpsertTopic)
	pg.DELETE("/draft/topics/:slug", s.handlePlaygroundDeleteTopic)
	pg.POST("/draft/assets", s.handlePlaygroundUploadAsset)
	pg.PATCH("/draft/assets/:ref", s.handlePlaygroundPatchAsset)
	pg.DELETE("/draft/assets/:ref", s.handlePlaygroundDeleteAsset)
	pg.POST("/draft/tariffs", s.handlePlaygroundUpsertTariff)
	pg.DELETE("/draft/tariffs/:ref", s.handlePlaygroundDeleteTariff)
	pg.POST("/draft/products", s.handlePlaygroundUpsertProduct)
	pg.DELETE("/draft/products/:ref", s.handlePlaygroundDeleteProduct)
	pg.PATCH("/draft/contacts", s.handlePlaygroundPatchContacts)
	pg.PATCH("/draft/policies", s.handlePlaygroundPatchPolicies)
	pg.PATCH("/draft/config", s.handlePlaygroundPatchConfig)
	pg.POST("/draft/materials", s.handlePlaygroundCreateMaterial)
	pg.GET("/draft/materials", s.handlePlaygroundListMaterials)
	pg.POST("/chat", s.handlePlaygroundChat)
	pg.GET("/requests", s.handlePlaygroundListRequests)
	pg.POST("/requests/:id/resolve", s.handlePlaygroundResolveRequest)
	pg.POST("/draft/approve", s.handlePlaygroundApprove)
	pg.POST("/draft/approve/:kind/:id", s.handlePlaygroundApproveEntity)

	// KB — the live-only editor (/knowledge-base). Every write here lands
	// directly in the live ai_ tables: no draft blob, no approve step, so it
	// can never mix with Playground's pending work (see plan "Playground
	// redesign").
	auth.GET("/kb", s.handleKBGet)
	kb := auth.Group("/kb")
	kb.POST("/topics", s.handleKBUpsertTopic)
	kb.DELETE("/topics/:slug", s.handleKBDeleteTopic)
	kb.POST("/assets", s.handleKBUploadAsset)
	kb.PATCH("/assets/:ref", s.handleKBPatchAsset)
	kb.DELETE("/assets/:ref", s.handleKBDeleteAsset)
	kb.POST("/tariffs", s.handleKBUpsertTariff)
	kb.DELETE("/tariffs/:ref", s.handleKBDeleteTariff)
	kb.POST("/products", s.handleKBUpsertProduct)
	kb.DELETE("/products/:ref", s.handleKBDeleteProduct)
	kb.PATCH("/contacts", s.handleKBPatchContacts)
	kb.PATCH("/policies", s.handleKBPatchPolicies)
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
		if tok != s.cfg.WebhookToken {
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
	_ = s.queue.Publish(queue.Message{Kind: queue.KindWaEvent, Payload: cp})

	// Cheap top-level peek for the event type + instance (no full parse). The full
	// body is only logged at debug to keep customer PII out of info logs.
	var peek struct {
		Event    string `json:"event"`
		Instance string `json:"instance"`
	}
	_ = json.Unmarshal(raw, &peek)
	s.log.Info("webhook received",
		"account_id", accountID, "event", peek.Event, "instance", peek.Instance,
		"bytes", len(raw), "queued", true)
	s.log.Debug("webhook body", "account_id", accountID, "body", string(raw))

	ok(c, nil)
}

// --- middleware -----------------------------------------------------------

func (s *Server) cors() gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range s.cfg.CORSOrigins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowed[origin] || allowed["*"]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Webhook-Token")
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
