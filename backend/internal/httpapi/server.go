// Package httpapi is the UI-facing API edge + the Evolution webhook ingress.
// Every /xchats/api/v1 response uses the unified {payload, errcode, message}
// envelope; ops and the webhook are the only routes outside it (per 7-api-contracts).
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/assistant"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// webhookTokenHeader is the header Evolution echoes back so we can verify it
// (header only — never a query param, so it never lands in access logs).
const webhookTokenHeader = "X-Webhook-Token"

// Server wires the HTTP edges to their dependencies.
type Server struct {
	cfg     *config.Config
	store   *store.Store
	queue   queue.Queue
	hub     *realtime.Hub
	blob    blob.Store
	drafter assistant.Drafter
	evo     evolution.Client
	kb      *kbstore.Store
	orgID   uuid.UUID
	log     *slog.Logger

	// pendingNames carries the display_name from "add account" (POST) to the QR
	// connect step (where the wa_accounts row is finally written): pre-connect
	// there is no row to hold it. Keyed by instance name; best-effort (process-
	// local) — a lost entry just falls back to the instance name.
	pendingMu    sync.Mutex
	pendingNames map[string]string
}

// Deps is the constructor input.
type Deps struct {
	Cfg     *config.Config
	Store   *store.Store
	Queue   queue.Queue
	Hub     *realtime.Hub
	Blob    blob.Store
	Drafter assistant.Drafter
	Evo     evolution.Client
	KB      *kbstore.Store
	OrgID   uuid.UUID
	Log     *slog.Logger
}

// New builds a Server.
func New(d Deps) *Server {
	return &Server{
		cfg: d.Cfg, store: d.Store, queue: d.Queue, hub: d.Hub,
		blob: d.Blob, drafter: d.Drafter, evo: d.Evo, kb: d.KB, orgID: d.OrgID, log: d.Log,
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
	return r
}

// handleWebhook verifies the shared token (header only), enqueues the raw event,
// and returns 200 fast — no DB write at the edge.
func (s *Server) handleWebhook(c *gin.Context) {
	if s.cfg.WebhookToken != "" {
		tok := c.GetHeader(webhookTokenHeader)
		if tok == "" {
			tok = c.GetHeader("apikey")
		}
		if tok != s.cfg.WebhookToken {
			fail(c, http.StatusUnauthorized, ErrWebhookUnauthorized, "bad webhook token")
			return
		}
	}
	raw, err := c.GetRawData()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "unreadable body")
		return
	}
	cp := make([]byte, len(raw))
	copy(cp, raw)
	_ = s.queue.Publish(queue.Message{Kind: queue.KindWaEvent, Payload: cp})
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
		c.Next()
		s.log.Info("http request",
			"method", c.Request.Method, "path", c.Request.URL.Path,
			"status", c.Writer.Status())
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
