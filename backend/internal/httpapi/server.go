// Package httpapi is the UI-facing API edge. Every /xchats/api/v1 response
// uses the unified {payload, errcode, message} envelope; ops routes are the
// only ones outside it (per 7-api-contracts).
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/inboxmedia"
	"github.com/yerassyldanay/xchats/backend/internal/kbimport"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/metaingest"
	"github.com/yerassyldanay/xchats/backend/internal/providerhealth"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/tgingest"
	"github.com/yerassyldanay/xchats/backend/internal/tunnel"
	"github.com/yerassyldanay/xchats/backend/internal/updatecheck"
	"github.com/yerassyldanay/xchats/backend/internal/whatsapp"
	"github.com/yerassyldanay/xchats/backend/internal/whatsappcloud"
	"github.com/yerassyldanay/xchats/backend/response"
)

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

// tgPoller is the narrow slice of *tgpoller.Manager the telegram accounts
// lifecycle needs in polling mode — declared inline (rather than importing
// internal/tgpoller here) for the same "no new package edge" reason as
// kbInvalidator above. Production (main.go) always constructs and passes
// one, in either mode; every call site here still checks
// TelegramResolvedMode() first, so a test harness exercising only webhook
// mode (the existing 28 telegram tests) can safely leave Deps.TGPoller nil.
type tgPoller interface {
	Upsert(spec store.TelegramPollBot)
	Remove(id uuid.UUID)
}

// Server wires the HTTP edges to their dependencies.
type Server struct {
	cfg      *config.Config
	store    *store.Store
	queue    queue.Queue
	hub      *realtime.Hub
	blob     blob.Store
	response *response.Service   // the multichannel response engine's entry point (simulator API)
	wa       whatsapp.Manager    // nil when WhatsApp is not configured; every handler checks
	tg       telegram.Client     // nil when Telegram is not configured; every handler checks
	tgProc   *tgingest.Processor // the Telegram webhook ingress's shared ingest core (internal/tgingest) — also used by internal/tgpoller in polling mode
	tgPoller tgPoller            // the long-poll manager (polling mode only — see tgPoller's own doc comment)
	kb       *kbstore.Store
	kbImport *kbimport.Service // nil when the import pipeline is not wired (every handler checks)
	orgID    uuid.UUID
	log      *slog.Logger

	// Meta channels (Instagram Direct, Messenger, WhatsApp Cloud API — see
	// internal/meta's package doc). All five are constructed unconditionally
	// at boot, exactly like tg/tgProc above: a deployment with no Meta App
	// ID/Secret saved yet still serves every route, just failing each one
	// individually with META_APP_NOT_CONFIGURED (metaAppCredentials) rather
	// than 404ing in a way that would look like the feature doesn't exist.
	metaClient  *meta.Client
	metaCreds   meta.Source
	metaProc    *metaingest.Processor
	waCloud     *whatsappcloud.Client
	inboxSigner *inboxmedia.Signer

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
	// loginLimit bounds POST /auth/login and /auth/password — the two
	// routes that run argon2id against attacker-controlled input before any
	// session exists. Burst is deliberately low: at m=64 MiB per hash
	// (internal/password's argon2id params), unlimited concurrent attempts
	// are also a memory-exhaustion amplifier, not just a brute-force risk.
	loginLimit *ipRateLimiter

	// Settings surface (settings.go). Every one of these four is
	// nil-tolerant — a deployment with no secure credential store, or one
	// running without the tunnel feature wired up, still serves everything
	// else unaffected; each settings.go handler checks before using them.
	credentials *credentials.Chain
	settings    *settings.Store
	tunnel      tunnel.Tunnel
	// llmRefresh re-resolves the response engine's LLM provider registry
	// (internal/llmprovider.Registry) after a credential or LLM setting
	// changes — nil in any test/deployment that never constructs one.
	llmRefresh func()
	// providerHealth is the self-healing status surface (internal/
	// providerhealth) — nil in any test/deployment that never constructs
	// one, in which case GET /settings/provider-health reports no providers
	// rather than failing.
	providerHealth *providerhealth.Tracker
	// updateChecker is the update notifier (internal/updatecheck) — nil in
	// any test/deployment that never constructs one, in which case GET
	// /settings/update-check reports no update information rather than
	// failing.
	updateChecker *updatecheck.Checker

	// bootstrapAdminCredentialPath is the owner-only, one-time password file
	// created by cmd/xchats on first boot. Empty in tests or deployments that
	// do not use the sentinel admin bootstrap.
	bootstrapAdminCredentialPath string
}

// Deps is the constructor input.
type Deps struct {
	Cfg           *config.Config
	Store         *store.Store
	Queue         queue.Queue
	Hub           *realtime.Hub
	Blob          blob.Store
	Response      *response.Service
	WA            whatsapp.Manager
	TG            telegram.Client
	TGProcessor   *tgingest.Processor
	TGPoller      tgPoller
	KB            *kbstore.Store
	KBImport      *kbimport.Service // nil to run without the structured import pipeline
	KBRepo        response.KnowledgeBaseRepository
	KBInvalidator kbInvalidator
	OrgID         uuid.UUID
	Log           *slog.Logger

	// Meta channels — see Server's own field doc comment.
	MetaClient       *meta.Client
	MetaProcessor    *metaingest.Processor
	WACloudClient    *whatsappcloud.Client
	InboxMediaSigner *inboxmedia.Signer

	// MCPAuth/MCPServer are nil to run without the MCP connector (every
	// route checks mcpAuthEnabled() first).
	MCPAuth   *mcpauth.Authorizer
	MCPServer *mcpserver.Server

	// Settings surface — see Server's own field doc comments; all six are
	// nil-tolerant.
	Credentials    *credentials.Chain
	Settings       *settings.Store
	Tunnel         tunnel.Tunnel
	LLMRefresh     func()
	ProviderHealth *providerhealth.Tracker
	UpdateChecker  *updatecheck.Checker

	BootstrapAdminCredentialPath string
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
		blob: d.Blob, response: d.Response, wa: d.WA, tg: d.TG,
		tgProc: d.TGProcessor, tgPoller: d.TGPoller, kb: d.KB, kbImport: d.KBImport,
		kbRepo: d.KBRepo, kbInvalidator: d.KBInvalidator,
		orgID: d.OrgID, log: d.Log,
		metaClient: d.MetaClient, metaCreds: metaCredentialsAdapter{chain: d.Credentials},
		metaProc: d.MetaProcessor, waCloud: d.WACloudClient, inboxSigner: d.InboxMediaSigner,
		mcpAuth: d.MCPAuth, mcpServer: d.MCPServer,
		mcpUploadSigner: uploadSigner, mcpMediaSigner: mediaSigner,
		csrfSecret:  randomCSRFFallbackSecret(),
		credentials: d.Credentials, settings: d.Settings, tunnel: d.Tunnel, llmRefresh: d.LLMRefresh,
		providerHealth: d.ProviderHealth, updateChecker: d.UpdateChecker,
		bootstrapAdminCredentialPath: d.BootstrapAdminCredentialPath,
		// Deliberately generous limits — these are abuse guards, not a
		// throttle on legitimate usage. See ratelimit.go's doc comment.
		oauthRegisterLimit:  newIPRateLimiter(5.0/60, 5),   // 5/min, 5 burst — registration spam is the highest-value target
		oauthTokenLimit:     newIPRateLimiter(30.0/60, 10), // 30/min, 10 burst — refresh-heavy legitimate usage needs headroom
		oauthAuthorizeLimit: newIPRateLimiter(20.0/60, 10), // 20/min, 10 burst
		loginLimit:          newIPRateLimiter(5.0/60, 5),   // 5/min, 5 burst — see the field's own doc comment
	}
}

// SetTunnel installs the tunnel controller after construction — the
// composition root's escape hatch for the circular dependency between the
// tunnel (which must serve THIS Server's own router) and Deps.Tunnel (read
// by settings.go's handlers, which the router references). Every settings.go
// tunnel route reads s.tunnel lazily at REQUEST time, never at Router()-build
// time, so calling this any time before the server actually starts accepting
// requests is safe — including nil, restoring the same "feature not
// configured" behavior Deps.Tunnel being nil at construction already has.
func (s *Server) SetTunnel(t tunnel.Tunnel) {
	s.tunnel = t
}

// Router builds the gin engine with all routes mounted.
// maxUploadBytes bounds POST /xchats/api/v1/media (A9) — matches
// frontend/nginx.conf's client_max_body_size, so a request that gets past
// the proxy either succeeds or fails the same way behind it.
const maxUploadBytes = 64 << 20 // 64 MiB

func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// A9: gin's own zero-value default (32 MiB) only controls multipart
	// parsing's memory/disk spill point, not a hard cap on the total
	// upload — handleUploadMedia's http.MaxBytesReader below is the actual
	// limit; this just keeps a large-but-under-the-limit upload from
	// spilling to a temp file unnecessarily.
	r.MaxMultipartMemory = maxUploadBytes
	// Trust NO proxy unless explicitly configured (plan Task 17): gin's own
	// zero-value default trusts every peer's X-Forwarded-* headers, which
	// would let any direct, unproxied caller spoof its own client IP. Every
	// discovery URL this server emits is already built from
	// cfg.ResolvedAPIBaseURL()/cfg.ResolvedFrontendBaseURL(), never from the
	// request's Host or X-Forwarded-*
	// (see mcpIssuer/MCPResourceURL) — this only affects c.ClientIP(), used
	// by the rate limiters.
	if err := r.SetTrustedProxies(s.cfg.Server.TrustedProxies); err != nil {
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

	// Telegram webhook ingress — commits the normalized rows before it acks;
	// see handleTelegramWebhook. WhatsApp has no webhook ingress: whatsmeow
	// connects out directly (internal/whatsmeow), so there is nothing for an
	// inbound HTTP route to receive.
	r.POST("/telegram/api/v1/webhook/:account_id", s.handleTelegramWebhook)

	// Meta channels — public, unauthenticated (Meta's own servers, or a
	// browser mid-OAuth-redirect, call these; there is no session cookie to
	// check in either case). The webhook paths are not account-specific
	// (see handleWhatsAppCloudWebhook's doc comment); the media route is
	// org-scoped by its signed token instead of a URL segment (see
	// handleMetaMediaRead).
	r.GET("/meta/api/v1/webhook/whatsapp", s.handleMetaWebhookVerify)
	r.POST("/meta/api/v1/webhook/whatsapp", s.handleWhatsAppCloudWebhook)
	r.GET("/meta/api/v1/webhook/instagram", s.handleMetaWebhookVerify)
	r.POST("/meta/api/v1/webhook/instagram", s.handleInstagramWebhook)
	r.GET("/meta/api/v1/webhook/messenger", s.handleMetaWebhookVerify)
	r.POST("/meta/api/v1/webhook/messenger", s.handleMessengerWebhook)
	r.GET("/meta/api/v1/media/:media_id", s.handleMetaMediaRead)
	// Instagram's and Messenger's OAuth redirects land here directly from
	// instagram.com/facebook.com — see handleInstagramOAuthCallback's own
	// doc comment for why these are public rather than behind
	// requireSession().
	r.GET(instagramCallbackPath, s.handleInstagramOAuthCallback)
	r.GET(messengerCallbackPath, s.handleMessengerOAuthCallback)

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
	api.POST("/auth/login", rateLimit(s.loginLimit), s.handleLogin)
	api.POST("/auth/logout", s.handleLogout)
	api.GET("/auth/bootstrap-status", s.handleBootstrapStatus)

	auth := api.Group("")
	auth.Use(s.requireSession())
	auth.Use(s.requirePasswordChanged())
	auth.GET("/me", s.handleMe)
	auth.POST("/auth/password", rateLimit(s.loginLimit), s.handleChangePassword)
	auth.GET("/users", s.handleListUsers)
	auth.POST("/users", s.RequireAdmin(), s.handleCreateUser)
	auth.PUT("/users/:id/role", s.RequireAdmin(), s.handleUpdateUserRole)
	auth.GET("/organization", s.handleGetOrg)
	auth.PUT("/organization", s.RequireAdmin(), s.handleUpdateOrg)
	// Task 15: explicit active-organization switch (the frontend selector) —
	// distinct from the MCP review-handoff redirect below, which sets the
	// SAME session field via a verified signed token instead of a direct
	// user action.
	auth.POST("/organization/active", s.handleSetActiveOrganization)

	// Channel-neutral account listing — every channel in one shape. The
	// per-channel routes below own each channel's own lifecycle.
	auth.GET("/accounts", s.handleListAccounts)
	// Channel-level debounce + scheduled auto-reply settings — one channel-
	// neutral route for every channel (see orgAnyAccount's own doc comment
	// on why it does not exclude Telegram the way orgAccount does).
	auth.PUT("/accounts/:id/automation", s.handleUpdateAccountAutomation)
	// Campaigns' per-account pacing surface — same orgAnyAccount scoping,
	// every channel (see campaigns.go's own file header).
	auth.GET("/accounts/:id/sending-budget", s.handleAccountSendingBudget)
	auth.GET("/accounts/:id/sending-limits", s.handleGetAccountSendingLimits)
	auth.PUT("/accounts/:id/sending-limits", s.handleSetAccountSendingLimits)

	// WhatsApp accounts manager: connect via whatsmeow's own QR pairing
	// (internal/whatsmeow), no external gateway involved.
	auth.GET("/whatsapp-accounts", s.handleListWhatsAppAccounts)
	auth.DELETE("/whatsapp-accounts/:id", s.handleDeleteWhatsAppAccount)
	auth.POST("/wa-accounts/pair", s.handlePairWhatsAppAccount)
	auth.GET("/wa-accounts/pair/:session_id", s.handlePairStatus)
	auth.POST("/wa-accounts/:id/logout", s.handleLogoutWhatsAppAccount)
	auth.GET("/wa-accounts/:id/status", s.handleWhatsAppAccountStatus)

	// Telegram accounts manager. A bot has no QR pairing, so it gets its own
	// lifecycle routes rather than sharing /whatsapp-accounts.
	auth.POST("/telegram-accounts", s.handleCreateTelegramAccount)
	auth.POST("/telegram-accounts/:id/retry-webhook", s.handleRetryTelegramWebhook)
	auth.POST("/telegram-accounts/:id/check", s.handleCheckTelegramAccount)
	auth.PUT("/telegram-accounts/:id/token", s.handleReplaceTelegramToken)
	auth.DELETE("/telegram-accounts/:id", s.handleDeleteTelegramAccount)

	// WhatsApp Cloud API accounts manager: BYO-App, manual WABA id + business
	// token connect (see whatsapp_cloud_accounts.go's own doc comments) —
	// discover lists a WABA's numbers without persisting anything, the POST
	// commits (activates the chosen number and registers the webhook).
	auth.POST("/whatsapp-cloud-accounts/discover", s.handleDiscoverWhatsAppCloudNumbers)
	auth.POST("/whatsapp-cloud-accounts", s.handleConnectWhatsAppCloudAccount)
	auth.DELETE("/whatsapp-cloud-accounts/:id", s.handleDeleteWhatsAppCloudAccount)

	// Instagram Direct accounts manager: OAuth-redirect connect (Instagram
	// Login — see meta_oauth.go's own doc comments). start mints the
	// authorize_url the frontend does a top-level navigation to; the
	// callback above is the public half of the same flow.
	auth.POST("/instagram-accounts/oauth/start", s.handleStartInstagramOAuth)
	auth.DELETE("/instagram-accounts/:id", s.handleDeleteInstagramAccount)

	// Facebook Messenger accounts manager: OAuth-redirect connect (plain
	// Facebook Login — see meta_oauth_messenger.go's own doc comments), same
	// start/callback shape as Instagram above.
	auth.POST("/messenger-accounts/oauth/start", s.handleStartMessengerOAuth)
	auth.DELETE("/messenger-accounts/:id", s.handleDeleteMessengerAccount)

	// Channel setup (channel_setup.go): installation-wide prerequisites
	// (public HTTPS access, the Meta Developer App, each channel's own
	// Dashboard checklist), as distinct from the per-account connect routes
	// above. GET is open to any member — see ChannelSetupInfo's own doc
	// comment for the member/admin payload split; only the two saves are
	// admin-only, matching every other credential-write route's own
	// per-route RequireAdmin() (e.g. POST /users above).
	auth.GET("/channel-setup", s.handleChannelSetup)
	auth.PUT("/channel-setup/meta-app", s.RequireAdmin(), s.handleSaveMetaApp)
	auth.PUT("/channel-setup/instagram-app", s.RequireAdmin(), s.handleSaveInstagramApp)

	auth.GET("/chats", s.handleListChats)
	auth.POST("/chats", s.handleCreateChat)
	auth.GET("/chats/:id/messages", s.handleListMessages)
	auth.POST("/chats/:id/messages", s.handleSendMessage)
	auth.POST("/chats/:id/read", s.handleReadChat)
	auth.PATCH("/chats/:id/assignee", s.handleAssignChat)

	auth.GET("/chats/:id/ai-drafts", s.handleListDrafts)
	auth.POST("/chats/:id/ai-drafts", s.handleSuggest)
	auth.POST("/ai-drafts/:id/approve", s.handleApprove)

	// Campaigns (plan/DECISIONS.md) — bulk outbound messaging against a
	// pasted/uploaded recipient list. See campaigns.go's own file header:
	// every route here talks to the store directly, never to internal/
	// campaign's Scheduler/Runner, which run independently in the background.
	auth.GET("/campaigns", s.handleListCampaigns)
	auth.POST("/campaigns", s.handleCreateCampaign)
	auth.GET("/campaigns/:id", s.handleGetCampaign)
	auth.PATCH("/campaigns/:id", s.handleUpdateCampaign)
	auth.DELETE("/campaigns/:id", s.handleDeleteCampaign)
	auth.POST("/campaigns/:id/duplicate", s.handleDuplicateCampaign)
	auth.POST("/campaigns/:id/preview", s.handleCampaignPreview)
	auth.PUT("/campaigns/:id/recipients", s.handleReplaceCampaignRecipients)
	auth.GET("/campaigns/:id/recipients", s.handleListCampaignRecipients)
	auth.POST("/campaigns/:id/recipients/retry-failed", s.handleRetryFailedCampaignRecipients)
	auth.GET("/campaigns/:id/events", s.handleListCampaignEvents)
	auth.POST("/campaigns/:id/start", s.handleStartCampaign)
	auth.POST("/campaigns/:id/pause", s.handlePauseCampaign)
	auth.POST("/campaigns/:id/resume", s.handleResumeCampaign)
	auth.POST("/campaigns/:id/stop", s.handleStopCampaign)
	// Customer management (CRM). Deliberately session-only, not RequireAdmin:
	// this is the working surface of the inbox — every manager who answers a
	// conversation needs to record who the customer is and what happens next.
	// Tenant isolation comes from crmOrg on every handler, not from a role.
	auth.GET("/customers", s.handleListCustomers)
	auth.POST("/customers", s.handleCreateCustomer)
	auth.GET("/customers/:id", s.handleGetCustomer)
	auth.PATCH("/customers/:id", s.handleUpdateCustomer)
	auth.GET("/customers/:id/conversations", s.handleCustomerConversations)
	auth.GET("/customers/:id/timeline", s.handleCustomerTimeline)
	auth.POST("/customers/:id/merge", s.handleMergeCustomers)
	auth.GET("/customers/:id/notes", s.handleListCustomerNotes)
	auth.POST("/customers/:id/notes", s.handleAddCustomerNote)
	auth.PATCH("/customers/:id/notes/:note_id", s.handleUpdateCustomerNote)
	auth.DELETE("/customers/:id/notes/:note_id", s.handleDeleteCustomerNote)
	auth.POST("/customers/:id/tags", s.handleAddCustomerTag)
	auth.DELETE("/customers/:id/tags/:tag_id", s.handleRemoveCustomerTag)

	// The org's own CRM vocabulary — tags, lifecycle statuses and custom
	// fields are rows an organization edits, never hard-coded lists.
	auth.GET("/crm/tags", s.handleListTags)
	auth.POST("/crm/tags", s.handleCreateTag)
	auth.PATCH("/crm/tags/:id", s.handleUpdateTag)
	auth.DELETE("/crm/tags/:id", s.handleDeleteTag)
	auth.GET("/crm/statuses", s.handleListStatuses)
	auth.POST("/crm/statuses", s.handleCreateStatus)
	auth.PATCH("/crm/statuses/:id", s.handleUpdateStatus)
	auth.DELETE("/crm/statuses/:id", s.handleDeleteStatus)
	auth.GET("/crm/custom-fields", s.handleListCustomFields)
	auth.POST("/crm/custom-fields", s.handleCreateCustomField)
	auth.PATCH("/crm/custom-fields/:id", s.handleUpdateCustomField)
	auth.DELETE("/crm/custom-fields/:id", s.handleDeleteCustomField)

	auth.GET("/followups", s.handleListFollowups)
	auth.GET("/followups/buckets", s.handleFollowupBuckets)
	auth.POST("/followups", s.handleCreateFollowup)
	auth.PATCH("/followups/:id", s.handleRescheduleFollowup)
	auth.POST("/followups/:id/complete", s.handleCompleteFollowup)
	auth.POST("/followups/:id/cancel", s.handleCancelFollowup)

	auth.POST("/media", s.handleUploadMedia)
	auth.GET("/media/:id", s.handleServeMedia)
	auth.GET("/realtime", s.handleRealtime)

	// MCP connector info — deliberately session-only (no RequireAdmin): any
	// logged-in user can see the connector URL to paste into ChatGPT/Claude,
	// even though configuring the tunnel that backs it stays admin-only.
	auth.GET("/mcp-connection", s.handleMCPConnectionInfo)

	// Simulator API (Phase 10) — gated: the route does not exist at all unless
	// explicitly enabled (SIMULATOR_ENABLED, default false outside dev).
	if s.cfg.System.SimulatorEnabled {
		auth.POST("/simulator/messages", s.handleSimulatorMessage)
		// Injects a synthetic whatsmeow-shaped event through the real
		// translate -> store -> queue -> worker chain, for automated testing
		// of the WhatsApp ingestion path without a phone — see
		// handleDebugWaEvent's doc comment.
		auth.POST("/debug/wa-event", s.handleDebugWaEvent)
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
	pg.DELETE("/draft/changes/:kind/:key", s.handlePlaygroundCancelChange)

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
	kb.POST("/materials", s.handleKBUploadMaterial)
	kb.GET("/materials/:id/content", s.handleKBMaterialContent)
	kb.PATCH("/config", s.handleKBPatchConfig)

	// Structured import pipeline (internal/kbimport) — submit URLs/documents,
	// extract, and synthesize into kbd_draft. Deliberately NOT an extension
	// of POST /kb/materials (see handleKBUploadMaterial's own doc comment):
	// that route's {material_id, processing_status} contract is synchronous,
	// no-AI, no-cost, and MediaFieldPicker depends on it staying that way.
	kb.POST("/imports", s.handleKBCreateImport)
	kb.GET("/imports", s.handleKBListImports)
	kb.GET("/imports/:id", s.handleKBGetImport)
	kb.GET("/import/providers", s.handleKBImportProviders)

	// Settings (settings.go) — every route here is admin-only. Handlers are
	// individually nil-tolerant for Credentials/Settings/Tunnel, but the
	// group itself is always registered: a deployment missing one of those
	// still gets a clear 503 from the affected routes rather than a 404
	// that would look like the feature doesn't exist at all.
	set := auth.Group("/settings")
	set.Use(s.RequireAdmin())
	set.GET("", s.handleGetSettings)
	set.GET("/integrations", s.handleListIntegrations)
	set.PUT("/integrations/:provider", s.handleUpdateIntegrationSettings)
	set.PUT("/integrations/:provider/credential", s.handleSaveIntegrationCredential)
	set.DELETE("/integrations/:provider/credential", s.handleDeleteIntegrationCredential)
	set.POST("/integrations/:provider/test", s.handleTestIntegrationCredential)
	set.PUT("/llm", s.handleUpdateLLMSettings)
	set.PUT("/credential-storage", s.handleUpdateCredentialStorage)
	set.PUT("/ngrok", s.handleUpdateNgrokSettings)
	set.GET("/backup/download", s.handleDownloadBackup)
	set.GET("/provider-health", s.handleProviderHealth)
	set.GET("/update-check", s.handleUpdateCheck)
	set.GET("/tunnel", s.handleGetTunnelStatus)
	set.POST("/tunnel/start", s.handleStartTunnel)
	set.POST("/tunnel/stop", s.handleStopTunnel)
	return r
}

// --- middleware -----------------------------------------------------------

func (s *Server) cors() gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range s.cfg.Server.CORSOrigins {
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
		switch {
		case origin == "":
			// no Origin header — not a CORS request at all.
		case allowed[origin]:
			// An exact, configured allowlist entry — a same-deployment
			// frontend the operator explicitly trusts. This is the only
			// case where riding the session cookie cross-origin is safe.
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		case allowed["*"]:
			// A7: a wildcard CORSOrigins config (dev/test convenience —
			// see config.yaml's own comment) must NEVER be paired with
			// credentials: that combination would let any website ride an
			// authenticated session simply by echoing its own Origin back.
			// Echo the origin (so the response is at least usable for a
			// non-credentialed fetch) but never Allow-Credentials.
			c.Header("Access-Control-Allow-Origin", origin)
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

// failWithPayload is fail with a non-nil payload attached — for the rare
// error response that still has something useful to show alongside the
// failure (e.g. handleStartTunnel returning the tunnel's Status, LastError
// included, even when Start itself failed).
func failWithPayload(c *gin.Context, status int, code, msg string, payload any) {
	c.AbortWithStatusJSON(status, envelope{payload, code, msg})
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
