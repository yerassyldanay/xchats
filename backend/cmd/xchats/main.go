// Command xchats is the xchats backend: API edge + webhook ingress + in-process
// workers + the multichannel response service. Subcommands: serve (default),
// migrate, seed, seed-kb-demo, simulate-message, kb-load, backup, check,
// restore.
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

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbops"
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
	"github.com/yerassyldanay/xchats/backend/internal/tgingest"
	"github.com/yerassyldanay/xchats/backend/internal/tgpoller"
	"github.com/yerassyldanay/xchats/backend/internal/whatsmeow"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// Telegram media sweep cadence. The retry delay is what keeps a permanently
// broken file_id from becoming a hot loop; the batch bounds one pass.
const (
	telegramMediaSweepEvery = 5 * time.Minute
	telegramMediaRetryAfter = 2 * time.Minute
	telegramMediaSweepBatch = 100
)

func main() {
	cfgPath := flag.String("config", "", "path to config.yaml (default: $XCHATS_CONFIG, then ./config.yaml)")
	envPath := flag.String("env", envOr("XCHATS_ENV", ".env"), "path to .env")
	flag.Parse()

	cmd := "serve"
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
	}

	cfg, err := config.Load(config.ResolveConfigPath(*cfgPath), *envPath)
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
	case "seed-kb-demo":
		st := mustStore(cfg, log)
		defer st.Close()
		runSeedKBDemo(context.Background(), cfg, st, log)
	case "simulate-message":
		runSimulateMessage(flag.Args()[1:])
	case "kb-load":
		runKBLoad(cfg, log, flag.Args()[1:])
	case "backup":
		runBackup(cfg, log, flag.Args()[1:])
	case "check":
		runCheck(cfg, log)
	case "restore":
		runRestore(log, flag.Args()[1:])
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

	// mustStore -> store.New already applies every pending migration on open
	// (see internal/store.New's doc comment), so there is no separate migrate
	// step here any more.
	st := mustStore(cfg, log)
	defer st.Close()
	seed(ctx, cfg, st, log)
	orgID := seededOrgID(ctx, cfg, st, log)

	// kb (internal/kbstore) is unrelated to the response service's own KB loader
	// (internal/responsestore, below) — it backs the /kb live editor and the
	// structured draft editor at /playground. No demo/seed content is
	// written here: SeedLiveIfEmpty(brain.SeedSnapshot()) is intentionally not
	// called — migration 0008 is the only source of temporary demo KB data.
	// Opening the same path mustStore already opened is deliberate and cheap:
	// internal/dbx.Open refcounts one connection per path, so kb shares st's
	// connection rather than racing a second one against the same file.
	kb, err := kbstore.New(ctx, cfg.Storage.DBPath)
	if err != nil {
		fatal("kbstore", err)
	}
	defer kb.Close()

	blobStore, err := blob.NewDisk(cfg.Storage.BlobDir)
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
	kbRepo, err := responsestore.NewKnowledgeBaseRepo(ctx, cfg.Storage.DBPath)
	if err != nil {
		fatal("kb repo", err)
	}
	cachedKB := responsestore.NewCachedKBRepo(kbRepo)
	responseService := &response.Service{
		Conversations: &responsestore.ConversationRepo{Store: st},
		KnowledgeBase: cachedKB,
		Drafts:        &responsestore.DraftRepo{Store: st},
		Engine:        engine,
	}
	log.Info("response service active",
		"provider", defaultModel.Provider, "model", defaultModel.Model, "prompt_ref", aiprompt.PromptRefShopKBV4)

	q := queue.NewInMem(2048, cfg.System.QueueWorkers, log)
	hub := realtime.NewHub()
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

	// tgProc is the shared Telegram ingest core: both the webhook handler
	// (internal/httpapi) and the long-poll manager below feed updates
	// through the exact same Process call, so a bot's history is identical
	// regardless of which ingress delivered it.
	tgProc := tgingest.New(tgingest.Deps{Store: st, Queue: q, Hub: hub, Log: log})
	// tgMgr is constructed unconditionally (its own Close is a cheap no-op
	// with nothing registered) so webhook-mode deployments can still flip
	// TG_MODE later without a restart-time wiring change; only Start is
	// gated on the resolved mode.
	tgMgr := tgpoller.NewManager(tgpoller.Deps{
		TG: tg, Tokens: st, Offsets: st, Processor: tgProc, State: st, Bots: st, Log: log,
	})
	telegramMode := cfg.TelegramResolvedMode()
	if telegramMode == "polling" {
		log.Info("telegram polling mode active — no public URL required")
		go tgMgr.Start(ctx)
	}

	waMgr, err := whatsmeow.NewManager(ctx, whatsmeow.ManagerConfig{
		DeviceDBPath: cfg.Storage.WADeviceDBPath,
		Store:        st,
		Blob:         blobStore,
		Queue:        q,
		Hub:          hub,
		Log:          log,
	})
	if err != nil {
		fatal("whatsmeow", err)
	}
	defer waMgr.Close()

	senders := messaging.NewSenderRegistry()
	senders.Register(messaging.ChannelWhatsApp, waMgr.ChannelSender())
	senders.Register(messaging.ChannelSimulator, simulator.NewChannelSender())
	senders.Register(messaging.ChannelTelegram, telegram.NewChannelSender(tg, st, blobStore))

	w := &worker.Worker{
		Store: st, Queue: q, TG: tg, Blob: blobStore, Hub: hub,
		Response: responseService, Senders: senders, Log: log,
	}
	q.Start(ctx, w.Handle)
	// Attachments whose bytes never arrived are retried from their own media
	// row — the durable work item that replaces an inbound-event table. The
	// startup pass picks up whatever a crash or an outage left behind.
	w.StartTelegramMediaSweeper(ctx, telegramMediaSweepEvery, telegramMediaRetryAfter, telegramMediaSweepBatch)
	// Reconnects every saved WhatsApp account without re-scanning a QR code.
	go waMgr.Start(ctx)

	mcpAuthorizer, mcpSrv := buildMCPConnector(ctx, cfg, kb, blobStore, log)

	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Queue: q, Hub: hub, Blob: blobStore,
		Response: responseService, WA: waMgr, TG: tg, TGProcessor: tgProc, TGPoller: tgMgr, KB: kb,
		KBRepo: cachedKB, KBInvalidator: cachedKB,
		OrgID: orgID, Log: log,
		MCPAuth: mcpAuthorizer, MCPServer: mcpSrv,
	})
	httpServer := &http.Server{Addr: cfg.Server.HTTPAddr, Handler: srv.Router()}

	go func() {
		log.Info("backend listening", "addr", cfg.Server.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatal("listen", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	// tgMgr before q: the poller publishes to the queue via tgProc, so it
	// must stop producing before the queue stops accepting. Explicit here
	// (not a defer) so the ordering relative to q.Close() is guaranteed
	// rather than left to defer's LIFO stacking.
	tgMgr.Close()
	q.Close()
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
	if isLocalBaseURL(cfg.Server.APIBaseURL) {
		problems = append(problems, fmt.Sprintf("API_BASE_URL=%q still points at localhost/loopback", cfg.Server.APIBaseURL))
	}
	if isLocalBaseURL(cfg.Server.FrontendBaseURL) {
		problems = append(problems, fmt.Sprintf("FRONTEND_BASE_URL=%q still points at localhost/loopback", cfg.Server.FrontendBaseURL))
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
func buildMCPConnector(ctx context.Context, cfg *config.Config, kb *kbstore.Store, blobStore blob.Store, log *slog.Logger) (*mcpauth.Authorizer, *mcpserver.Server) {
	key, err := mcpauth.NewSigningKeyFromSeed(cfg.MCPJWTSigningKey)
	if err != nil {
		log.Warn("MCP_JWT_SIGNING_KEY not set; using an ephemeral key for this process — issued MCP tokens will not survive a restart",
			"reason", err.Error())
		key = mcpauth.NewEphemeralSigningKey()
	}
	// Shares runServe's connection via dbx.Open's per-path refcounting, same
	// as kb above — this is not a second pool against the same file.
	mcpStore, err := mcpauth.NewStore(ctx, cfg.Storage.DBPath)
	if err != nil {
		fatal("mcpauth store", err)
	}
	authorizer := mcpauth.New(mcpStore, key, mcpauth.Config{
		Issuer:            cfg.ResolvedAPIBaseURL(),
		Audience:          cfg.MCPResourceURL(),
		AllowPrivateHosts: cfg.KBAllowPrivateFetch,
		AccessTokenTTL:    time.Duration(cfg.MCP.AccessTokenTTLSeconds) * time.Second,
		RefreshTokenTTL:   time.Duration(cfg.MCP.RefreshTokenTTLDays) * 24 * time.Hour,
		AuthCodeTTL:       time.Duration(cfg.MCP.AuthCodeTTLSeconds) * time.Second,
	})
	uploadSigner := mcpauth.NewUploadTokenSigner(key)
	mediaSigner := mcpauth.NewMediaReadTokenSigner(key)
	apiBase := cfg.ResolvedAPIBaseURL()
	reviewSigner := mcpauth.NewReviewHandoffSigner(key, apiBase, time.Duration(cfg.MCP.ReviewHandoffTTLSeconds)*time.Second)
	mcpSrv := mcpserver.New(mcpserver.Deps{
		KB: kb, Blob: blobStore, Log: log,
		UploadBaseURL:    apiBase,
		SignUpload:       uploadSigner.Sign,
		UploadTTLSeconds: cfg.MCP.UploadTokenTTLSeconds,
		SignMediaRead:    mediaSigner.Sign,
		MediaTTLSeconds:  cfg.MCP.MediaTokenTTLSeconds,
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

// runSeedKBDemo inserts the fixed "Demo Shop" KB dataset (kbstore.SeedDemoKB)
// into the seeded org — explicit and opt-in only ("xchats seed-kb-demo" /
// "make seed-kb-demo"), never called from runServe or RunMigrations. Requires
// an org to already exist (run the "seed" command first on a fresh database);
// unlike migration 0008's old auto-run version this has no reason to no-op
// quietly on a missing org, so it fails loudly instead.
func runSeedKBDemo(ctx context.Context, cfg *config.Config, st *store.Store, log *slog.Logger) {
	orgID := seededOrgID(ctx, cfg, st, log)
	if orgID == uuid.Nil {
		fatal("seed-kb-demo", fmt.Errorf("no organization found — run the \"seed\" command first"))
	}
	kb, err := kbstore.New(ctx, cfg.Storage.DBPath)
	if err != nil {
		fatal("seed-kb-demo", err)
	}
	defer kb.Close()
	inserted, err := kb.SeedDemoKB(ctx, orgID)
	if err != nil {
		fatal("seed-kb-demo", err)
	}
	if !inserted {
		log.Info("seed-kb-demo: org already has KB content — skipped", "org_id", orgID)
		return
	}
	log.Info("seed-kb-demo: demo KB content inserted", "org_id", orgID)
}

// runMigrate applies every pending migration and exits. Opening the store IS
// the migration (store.New migrates on open), so this subcommand is now a way
// to run migrations without starting the server — useful as a deploy step
// before the first serve, and as a check that the schema is current. It stays
// a distinct subcommand rather than being removed precisely because callers
// (the Makefile, deploy scripts) treat "migrate then serve" as two steps.
func runMigrate(cfg *config.Config, log *slog.Logger) {
	st := mustStore(cfg, log)
	defer st.Close()
	log.Info("migrations applied")
}

// runBackup writes a consistent, compacted snapshot of DB_PATH to the given
// destination path (VACUUM INTO; the destination must not already exist).
// Opening the store acquires internal/dbx's single-process lock, so this
// subcommand — like "check" below — cannot run against a DB_PATH that
// "xchats serve" already has open; stop the server first, or use the
// in-app "Download Backup" action (internal/httpapi's settings surface),
// which runs inside the already-open server process instead.
func runBackup(cfg *config.Config, log *slog.Logger, args []string) {
	if len(args) != 1 {
		fatal("backup", errString("usage: xchats backup <dest-path>"))
	}
	st := mustStore(cfg, log)
	defer st.Close()
	if err := st.Backup(context.Background(), args[0]); err != nil {
		fatal("backup", err)
	}
	log.Info("backup complete", "dest", args[0])
}

// runCheck runs SQLite's own consistency check against DB_PATH and reports
// every problem found, if any. See runBackup's doc comment for why this
// needs the server stopped (or run the "Download Backup" flow instead,
// whose zip manifest records the same check).
func runCheck(cfg *config.Config, log *slog.Logger) {
	st := mustStore(cfg, log)
	defer st.Close()
	problems, err := st.IntegrityCheck(context.Background())
	if err != nil {
		fatal("check", err)
	}
	if len(problems) > 0 {
		for _, p := range problems {
			log.Error("integrity check problem", "detail", p)
		}
		fatal("check", fmt.Errorf("%d integrity problem(s) found", len(problems)))
	}
	log.Info("integrity check passed")
}

// runRestore atomically replaces destPath with backupPath's contents. Pure
// path arguments, no config/store involved: it is an offline operation that
// validates backupPath's integrity before touching anything at destPath (see
// dbops.Restore), and calls dbops directly rather than going through a Store
// — there is nothing at destPath to open yet, and mustStore would just
// contend with Restore's own flock (internal/dbx.LockPath) for no reason.
func runRestore(log *slog.Logger, args []string) {
	if len(args) != 2 {
		fatal("restore", errString("usage: xchats restore <backup-path> <dest-path>"))
	}
	if err := dbops.Restore(context.Background(), args[0], args[1]); err != nil {
		fatal("restore", err)
	}
	log.Info("restore complete", "backup", args[0], "dest", args[1])
}

// seed ensures the configured organization exists. WhatsApp accounts are no
// longer pre-configured or seeded here — they are paired dynamically via the
// UI (internal/whatsmeow), so the derived account id only ever comes into
// existence once a phone actually completes pairing. Admin user credentials
// are created by migration 0006_init_admin — no boot-time user creation is
// performed here either.
func seed(ctx context.Context, cfg *config.Config, st *store.Store, log *slog.Logger) {
	if _, err := st.SeedOrganization(ctx, cfg.OrgName); err != nil {
		fatal("seed org", err)
	}
}

// --- small helpers --------------------------------------------------------

func mustStore(cfg *config.Config, log *slog.Logger) *store.Store {
	if cfg.Storage.DBPath == "" {
		fatal("config", errString("DB_PATH is required"))
	}
	st, err := store.New(context.Background(), cfg.Storage.DBPath)
	if err != nil {
		fatal("connect db", err)
	}
	return st
}

func newLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.System.LogLevel) {
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

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}

type errString string

func (e errString) Error() string { return string(e) }
