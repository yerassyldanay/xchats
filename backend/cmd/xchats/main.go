// Command xchats is the xchats backend: API edge + webhook ingress + in-process
// workers + the multichannel response service. Subcommands: serve (default),
// migrate, webhook-set, seed.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	// tzdata embeds the IANA timezone database in the binary — insurance
	// against a future scratch base image that doesn't ship
	// /usr/share/zoneinfo (today's gcr.io/distroless/static-debian12 does).
	// Auto-response schedules resolve their configured IANA zone through
	// time.LoadLocation; without this import (or a mounted zoneinfo), that
	// call would fail on such an image and every schedule would fail closed
	// with a logged error rather than silently misfiring — but it should
	// never come to that.
	_ "time/tzdata"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/llmprovider"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/simulator"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/telemetry"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/migrations"
	"github.com/yerassyldanay/xchats/backend/response"
)

const webhookTokenHeader = "X-Webhook-Token"

// Telegram media sweep cadence. The retry delay is what keeps a permanently
// broken file_id from becoming a hot loop; the batch bounds one pass.
const (
	telegramMediaSweepEvery = 5 * time.Minute
	telegramMediaRetryAfter = 2 * time.Minute
	telegramMediaSweepBatch = 100
)

// Draft debounce + auto-response sweeper cadence (plan/auto-response.md).
// quietPeriod/debounceCap tune the debounce table: quietPeriod is how long a
// chat must go silent before its draft generates; debounceCap bounds the
// total wait for a chat that never falls quiet. sweepEvery drives one shared
// goroutine that recovers due debounce rows, claims+fires due auto-response
// jobs, reaps stale claims, and purges old terminal rows — see
// worker.Worker.StartSweeper. maxDeliveryLag is what keeps a deploy that
// spans the sweep window from flushing a backlog of stale auto-replies the
// moment it comes back up; claimTimeout is the reaper's cutoff for a worker
// process that died mid-send; jobRetention is how long a terminal job row
// survives before purge.
const (
	quietPeriod         = 5 * time.Second
	debounceCap         = 60 * time.Second
	sweepEvery          = 10 * time.Second
	sweepBatch          = 50
	claimTimeout        = 5 * time.Minute
	maxDeliveryLag      = 10 * time.Minute
	jobRetention        = 30 * 24 * time.Hour
	sweeperShutdownWait = 3 * time.Second
)

func main() {
	cfgPath := flag.String("config", envOr("XCHATS_CONFIG", "config.yaml"), "path to config.yaml")
	envPath := flag.String("env", envOr("XCHATS_ENV", ".env"), "path to .env")
	flag.Parse()

	cmd := "serve"
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
	}

	cfg, err := config.Load(*cfgPath, *envPath)
	if err != nil {
		fatal("load config", err)
	}
	log := newLogger(cfg)

	switch cmd {
	case "serve":
		runServe(cfg, log)
	case "migrate":
		runMigrate(cfg, log)
	case "seed":
		st := mustStore(cfg, log)
		defer st.Close()
		seed(context.Background(), cfg, st, log)
	case "webhook-set":
		runWebhookSet(cfg, log)
	case "simulate-message":
		runSimulateMessage(flag.Args()[1:])
	case "kb-load":
		runKBLoad(cfg, log, flag.Args()[1:])
	default:
		log.Error("unknown command", "cmd", cmd)
		os.Exit(2)
	}
}

func runServe(cfg *config.Config, log *slog.Logger) {
	if cfg.IsProduction() {
		if problems := validateProductionConfig(cfg); len(problems) > 0 {
			fatal("production config", fmt.Errorf("%s", strings.Join(problems, "; ")))
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Langfuse LLM tracing (best-effort): install a global OTel TracerProvider so
	// the LLM clients export each call as a generation. Never fatal.
	if cfg.LangfuseTracingEnabled() {
		if tp, err := telemetry.NewLangfuseProvider(ctx, cfg, "xchats"); err != nil {
			log.Warn("langfuse tracing init failed; continuing without it", "err", err)
		} else {
			log.Info("langfuse tracing enabled", "host", cfg.LangfuseHost)
			defer func() {
				shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = tp.Shutdown(shutCtx)
			}()
		}
	}

	st := mustStore(cfg, log)
	defer st.Close()
	if err := store.RunMigrations(ctx, st.Pool(), migrations.FS); err != nil {
		fatal("migrate", err)
	}
	accountID := seed(ctx, cfg, st, log)
	orgID := seededOrgID(ctx, cfg, st, log)

	// kb (internal/kbstore) is unrelated to the response service's own KB loader
	// (internal/responsestore, below) — it backs the /kb live editor and the
	// structured draft editor at /playground. No demo/seed content is
	// written here: SeedLiveIfEmpty(brain.SeedSnapshot()) is intentionally not
	// called — migration 0008 is the only source of temporary demo KB data.
	kb := kbstore.New(st.Pool())

	blobStore, err := blob.NewDisk(cfg.BlobDir)
	if err != nil {
		fatal("blob", err)
	}

	llms, defaultModel := buildLLMRegistry(cfg)
	engine := &response.Engine{
		LLMs:         llms,
		DefaultModel: defaultModel,
		MaxTokens:    cfg.LLMDraftMaxTokens,
		Temperature:  cfg.LLMDraftTemperature,
		RetryEnabled: cfg.LLMDraftRetry,
	}
	// cachedKB is the ONE shared, cached build of the prompt-facing KB: the
	// response engine's hot path (every customer reply) and GET /kb/prompt
	// (the /knowledge-base "Промпт" tab) both read through it, so the tab is
	// never a second, possibly-divergent rendering of the same data.
	cachedKB := responsestore.NewCachedKBRepo(&responsestore.KnowledgeBaseRepo{Pool: st.Pool()})
	responseService := &response.Service{
		Conversations: &responsestore.ConversationRepo{Store: st},
		KnowledgeBase: cachedKB,
		Drafts:        &responsestore.DraftRepo{Store: st},
		Engine:        engine,
	}
	log.Info("response service active",
		"provider", defaultModel.Provider, "model", defaultModel.Model, "prompt_ref", aiprompt.PromptRefShopKBV4)

	q := queue.NewInMem(2048, cfg.QueueWorkers, log)
	hub := realtime.NewHub()
	evo := evolution.NewHTTP(cfg.EvolutionBaseURL, cfg.EvolutionAPIKey, cfg.EvolutionInstance, log)
	tg := telegram.NewHTTP(cfg.TelegramResolvedAPIBaseURL(), log)

	// Credentials at rest. Without a key the Telegram lifecycle refuses to store
	// or read a bot token (an explicit error, never silent plaintext); WhatsApp
	// is unaffected, so this is a warning rather than a boot failure.
	if box, err := secretbox.FromEnvValue(cfg.TelegramCredentialsEncKey); err != nil {
		log.Warn("credentials encryption disabled; Telegram accounts cannot be connected",
			"reason", err.Error())
	} else {
		st.UseCredentialsBox(box)
		log.Info("credentials encryption enabled", "key_version", secretbox.KeyVersion)
	}

	senders := messaging.NewSenderRegistry()
	senders.Register(messaging.ChannelWhatsApp, evolution.NewChannelSender(evo, blobStore))
	senders.Register(messaging.ChannelSimulator, simulator.NewChannelSender())
	senders.Register(messaging.ChannelTelegram, telegram.NewChannelSender(tg, st, blobStore))

	w := &worker.Worker{
		Store: st, Queue: q, Evo: evo, TG: tg, Blob: blobStore, Hub: hub,
		Response: responseService, Senders: senders, Log: log,
		DebounceQuiet: quietPeriod, DebounceCap: debounceCap, DebounceSweepBatch: sweepBatch,
		AutoResponseSweepBatch: sweepBatch, AutoResponseClaimTimeout: claimTimeout,
		AutoResponseMaxDeliveryLag: maxDeliveryLag, AutoResponseJobRetention: jobRetention,
	}
	q.Start(ctx, w.Handle)
	// Attachments whose bytes never arrived are retried from their own media
	// row — the durable work item that replaces an inbound-event table. The
	// startup pass picks up whatever a crash or an outage left behind.
	mediaSweepDone := w.StartTelegramMediaSweeper(ctx, telegramMediaSweepEvery, telegramMediaRetryAfter, telegramMediaSweepBatch)
	// One shared goroutine: debounce recovery, auto-response claim/fire, stale
	// claim reap, terminal-row purge.
	autoResponseSweepDone := w.StartSweeper(ctx, sweepEvery)

	mcpAuthorizer, mcpSrv := buildMCPConnector(cfg, st, kb, blobStore, log)

	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Queue: q, Hub: hub, Blob: blobStore,
		Response: responseService, Evo: evo, TG: tg, KB: kb,
		KBRepo: cachedKB, KBInvalidator: cachedKB,
		OrgID: orgID, Log: log,
		MCPAuth: mcpAuthorizer, MCPServer: mcpSrv,
	})
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: srv.Router()}

	go func() {
		log.Info("backend listening", "addr", cfg.HTTPAddr, "account_id", accountID)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatal("listen", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	// Two-phase, both bounded. Phase 1: stop anything that could publish a
	// NEW task — both sweepers select on ctx.Done() (already cancelled above)
	// and close their done channel only once fully exited; StopDebounceTimers
	// cancels every pending timer and waits for any already-firing callback
	// to finish its publish. Phase 2, only once phase 1 is done: q.Wait()
	// really does mean "nothing left" once nothing new can be added, so it is
	// safe to drain before q.Close() — otherwise a task a timer just
	// published in phase 1 could still be mid-process on a worker goroutine
	// when q.Close() (and the subsequent st.Close()) runs out from under it.
	waitSweepers(mediaSweepDone, autoResponseSweepDone, w.StopDebounceTimers())
	queueDrained := make(chan struct{})
	go func() { q.Wait(); close(queueDrained) }()
	waitSweepers(queueDrained)
	q.Close()
}

// waitSweepers blocks until every done channel closes, or timeout elapses —
// whichever comes first. Work still running past the timeout is not fatal
// (a sweeper will observe ctx.Done() on its next select and exit; an
// in-flight publish, if any, just races the impending q.Close(), which
// queue.Publish now recovers from), but in practice this settles in
// milliseconds, so the bound is rarely if ever hit.
func waitSweepers(dones ...<-chan struct{}) {
	deadline := time.After(sweeperShutdownWait)
	for _, done := range dones {
		select {
		case <-done:
		case <-deadline:
			return
		}
	}
}

// buildLLMRegistry resolves the configured LLM providers into an llm.Registry
// and the default ModelRef the response engine calls. Missing configuration
// for the DEFAULT provider (LLM_DEFAULT_PROVIDER, default "openrouter") is a
// startup failure with a clear message — there is no stub-LLM fallback.
func buildLLMRegistry(cfg *config.Config) (llm.Registry, llm.ModelRef) {
	provider := orDefault(cfg.LLMDefaultProvider, "openrouter")
	model := orDefault(cfg.LLMDefaultModel, "google/gemini-2.5-flash")
	if cfg.LLMProviderKey(provider) == "" {
		fatal("llm config", fmt.Errorf(
			"missing API key for LLM_DEFAULT_PROVIDER=%q — set OPENROUTER_API_KEY, OPENAI_API_KEY, or GEMINI_API_KEY to match", provider))
	}
	timeoutSeconds := cfg.LLMDraftTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	reg := llmprovider.BuildRegistry([]llmprovider.ProviderConfig{
		{Name: "openrouter", APIKey: cfg.OpenRouterAPIKey, BaseURL: cfg.OpenRouterBaseURL},
		{Name: "openai", APIKey: cfg.OpenAIAPIKey, BaseURL: cfg.OpenAIBaseURL},
		{Name: "gemini", APIKey: cfg.GeminiAPIKey, BaseURL: cfg.GeminiBaseURL},
	}, time.Duration(timeoutSeconds)*time.Second)
	return reg, llm.ModelRef{Provider: provider, Model: model}
}

// validateProductionConfig returns every reason cfg is unsafe to run with
// ENVIRONMENT=production (plan Task 17's release gate): an ephemeral MCP
// signing key (MCPJWTSigningKey unset or otherwise invalid — every replica
// would then mint tokens no other replica, and no restart, can verify), or a
// base URL still pointing at localhost/loopback. Development (the default —
// anything other than the literal "production") never calls this: those are
// legitimate conveniences there, not misconfigurations. An empty return means
// clear to boot.
func validateProductionConfig(cfg *config.Config) []string {
	var problems []string
	if _, err := mcpauth.NewSigningKeyFromSeed(cfg.MCPJWTSigningKey); err != nil {
		problems = append(problems, "MCP_JWT_SIGNING_KEY is unset or invalid — production would mint an ephemeral per-process key that a restart or a second replica can never verify")
	}
	if isLocalBaseURL(cfg.APIBaseURL) {
		problems = append(problems, fmt.Sprintf("API_BASE_URL=%q still points at localhost/loopback", cfg.APIBaseURL))
	}
	if isLocalBaseURL(cfg.FrontendBaseURL) {
		problems = append(problems, fmt.Sprintf("FRONTEND_BASE_URL=%q still points at localhost/loopback", cfg.FrontendBaseURL))
	}
	return problems
}

// isLocalBaseURL reports whether raw is empty, unparseable, or points at a
// loopback/localhost host — fine for local development, a startup-blocking
// misconfiguration in production.
func isLocalBaseURL(raw string) bool {
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return true
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1", "":
		return true
	default:
		return false
	}
}

// buildMCPConnector wires the MCP connector's OAuth 2.1 authorization server
// and JSON-RPC tool server (plan/mcp.md). Without MCP_JWT_SIGNING_KEY set it
// degrades to an ephemeral per-process key rather than failing to boot —
// the same warn-and-continue call runServe already makes for
// TelegramCredentialsEncKey — so every route stays reachable for local/dev
// use; the cost is that every token issued before a restart stops verifying
// after one, which is unacceptable only in a real deployment (where the
// operator is expected to set the env var).
func buildMCPConnector(cfg *config.Config, st *store.Store, kb *kbstore.Store, blobStore blob.Store, log *slog.Logger) (*mcpauth.Authorizer, *mcpserver.Server) {
	key, err := mcpauth.NewSigningKeyFromSeed(cfg.MCPJWTSigningKey)
	if err != nil {
		log.Warn("MCP_JWT_SIGNING_KEY not set; using an ephemeral key for this process — issued MCP tokens will not survive a restart",
			"reason", err.Error())
		key = mcpauth.NewEphemeralSigningKey()
	}
	mcpStore := mcpauth.NewStore(st.Pool())
	authorizer := mcpauth.New(mcpStore, key, mcpauth.Config{
		Issuer:            strings.TrimRight(cfg.APIBaseURL, "/"),
		Audience:          cfg.MCPResourceURL(),
		AllowPrivateHosts: cfg.KBAllowPrivateFetch,
		AccessTokenTTL:    time.Duration(cfg.MCPAccessTokenTTLSeconds) * time.Second,
		RefreshTokenTTL:   time.Duration(cfg.MCPRefreshTokenTTLDays) * 24 * time.Hour,
		AuthCodeTTL:       time.Duration(cfg.MCPAuthCodeTTLSeconds) * time.Second,
	})
	uploadSigner := mcpauth.NewUploadTokenSigner(key)
	mediaSigner := mcpauth.NewMediaReadTokenSigner(key)
	apiBase := strings.TrimRight(cfg.APIBaseURL, "/")
	reviewSigner := mcpauth.NewReviewHandoffSigner(key, apiBase, time.Duration(cfg.MCPReviewHandoffTTLSeconds)*time.Second)
	mcpSrv := mcpserver.New(mcpserver.Deps{
		KB: kb, Blob: blobStore, Log: log,
		UploadBaseURL:    apiBase,
		SignUpload:       uploadSigner.Sign,
		UploadTTLSeconds: cfg.MCPUploadTokenTTLSeconds,
		SignMediaRead:    mediaSigner.Sign,
		MediaTTLSeconds:  cfg.MCPMediaTokenTTLSeconds,
		FrontendBaseURL:  cfg.MCPResolvedFrontendBaseURL(),
		SignReviewHandoff: func(userID, orgID uuid.UUID) (string, error) {
			token, _, _, err := reviewSigner.Sign(userID, orgID)
			if err != nil {
				return "", err
			}
			return apiBase + "/playground/review-handoff?token=" + url.QueryEscape(token), nil
		},
	})
	return authorizer, mcpSrv
}

// seededOrgID returns the seeded org's id (idempotent re-seed) for the KB + playground.
func seededOrgID(ctx context.Context, cfg *config.Config, st *store.Store, log *slog.Logger) uuid.UUID {
	org, err := st.SeedOrganization(ctx, cfg.OrgName)
	if err != nil {
		log.Warn("resolve org id failed", "err", err)
		return uuid.Nil
	}
	return org.ID
}

func runMigrate(cfg *config.Config, log *slog.Logger) {
	ctx := context.Background()
	st := mustStore(cfg, log)
	defer st.Close()
	if err := store.RunMigrations(ctx, st.Pool(), migrations.FS); err != nil {
		fatal("migrate", err)
	}
	log.Info("migrations applied")
}

func runWebhookSet(cfg *config.Config, log *slog.Logger) {
	ctx := context.Background()
	ownerJID := resolveOwnerJID(ctx, cfg, log)
	if ownerJID == "" {
		fatal("webhook-set", errString("owner_jid unknown — set wa_owner_jid"))
	}
	accountID := config.AccountID(ownerJID)
	url := strings.TrimRight(cfg.WebhookPublicBaseURL, "/") + "/evolution/api/v1/webhook/" + accountID.String()
	evo := evolution.NewHTTP(cfg.EvolutionBaseURL, cfg.EvolutionAPIKey, cfg.EvolutionInstance, log)
	if err := evo.SetWebhook(ctx, cfg.EvolutionInstance, url, webhookTokenHeader, cfg.WebhookToken, evolution.WebhookEvents); err != nil {
		fatal("set webhook", err)
	}
	log.Info("webhook registered", "url", url)
}

// seed upserts the org, admin user, and the single pre-connected account; returns
// the derived account id.
func seed(ctx context.Context, cfg *config.Config, st *store.Store, log *slog.Logger) (accountID uuid.UUID) {
	org, err := st.SeedOrganization(ctx, cfg.OrgName)
	if err != nil {
		fatal("seed org", err)
	}
	if cfg.SeedAdminEmail != "" && cfg.SeedAdminPassword != "" {
		hash, herr := httpapi.HashPassword(cfg.SeedAdminPassword)
		if herr != nil {
			fatal("hash admin", herr)
		}
		if _, err := st.SeedUser(ctx, org.ID, strings.TrimSpace(cfg.SeedAdminEmail), hash, "Admin"); err != nil {
			fatal("seed user", err)
		}
	}
	ownerJID := resolveOwnerJID(ctx, cfg, log)
	if ownerJID == "" {
		log.Warn("no owner_jid — account not seeded (set wa_owner_jid)")
		return accountID
	}
	id := config.AccountID(ownerJID)
	acct, err := st.SeedAccount(ctx, store.Account{
		ID:                 id,
		OrganizationID:     nullUUID(org.ID),
		DisplayName:        orDefault(cfg.WaAccountDisplayName, "WhatsApp"),
		ExternalAccountRef: config.CanonicalJID(ownerJID),
		ExternalHandle:     config.PhoneFromJID(config.CanonicalJID(ownerJID)),
		InstanceName:       cfg.EvolutionInstance,
		ConnectionState:    "connected",
	})
	if err != nil {
		fatal("seed account", err)
	}
	log.Info("account seeded", "id", acct.ID, "owner_jid", acct.ExternalAccountRef)
	return acct.ID
}

// resolveOwnerJID prefers config; otherwise learns it once from FetchInstances.
func resolveOwnerJID(ctx context.Context, cfg *config.Config, log *slog.Logger) string {
	if cfg.WaOwnerJID != "" {
		return cfg.WaOwnerJID
	}
	if cfg.EvolutionBaseURL == "" || cfg.EvolutionAPIKey == "" {
		return ""
	}
	evo := evolution.NewHTTP(cfg.EvolutionBaseURL, cfg.EvolutionAPIKey, cfg.EvolutionInstance, log)
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	insts, err := evo.FetchInstances(cctx)
	if err != nil {
		log.Warn("fetchInstances failed", "err", err)
		return ""
	}
	for _, in := range insts {
		if in.Name == cfg.EvolutionInstance && in.OwnerJID != "" {
			return in.OwnerJID
		}
	}
	if len(insts) == 1 {
		return insts[0].OwnerJID
	}
	return ""
}

// --- small helpers --------------------------------------------------------

func mustStore(cfg *config.Config, log *slog.Logger) *store.Store {
	if cfg.DatabaseURL == "" {
		fatal("config", errString("DATABASE_URL is required"))
	}
	st, err := store.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		fatal("connect db", err)
	}
	return st
}

func newLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	// logfmt == slog's TextHandler (key=value).
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func nullUUID(u uuid.UUID) uuid.NullUUID { return uuid.NullUUID{UUID: u, Valid: true} }

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}

type errString string

func (e errString) Error() string { return string(e) }
