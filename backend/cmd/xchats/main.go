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
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/llmprovider"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/internal/simulator"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telemetry"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/migrations"
	"github.com/yerassyldanay/xchats/backend/response"
)

const webhookTokenHeader = "X-Webhook-Token"

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
	default:
		log.Error("unknown command", "cmd", cmd)
		os.Exit(2)
	}
}

func runServe(cfg *config.Config, log *slog.Logger) {
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
	// (internal/responsestore, below) — it backs the still-active /kb/* live
	// editor and the disconnected Playground's dormant-but-compiling fields
	// (KB/Extract/Builder are nil here; Playground routes stay registered but
	// unreachable from any production entry point). No demo/seed content is
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
	responseService := &response.Service{
		Conversations: &responsestore.ConversationRepo{Store: st},
		KnowledgeBase: &responsestore.KnowledgeBaseRepo{Pool: st.Pool()},
		Drafts:        &responsestore.DraftRepo{Store: st},
		Engine:        engine,
	}
	log.Info("response service active",
		"provider", defaultModel.Provider, "model", defaultModel.Model, "prompt_ref", aiprompt.PromptRefShopKBV4)

	q := queue.NewInMem(2048, cfg.QueueWorkers, log)
	hub := realtime.NewHub()
	evo := evolution.NewHTTP(cfg.EvolutionBaseURL, cfg.EvolutionAPIKey, cfg.EvolutionInstance, log)

	senders := messaging.NewSenderRegistry()
	senders.Register(messaging.ChannelWhatsApp, evolution.NewChannelSender(evo))
	senders.Register(messaging.ChannelSimulator, simulator.NewChannelSender())

	w := &worker.Worker{
		Store: st, Queue: q, Evo: evo, Blob: blobStore, Hub: hub,
		Response: responseService, Senders: senders, KB: kb, Log: log,
	}
	q.Start(ctx, w.Handle)

	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Queue: q, Hub: hub, Blob: blobStore,
		Response: responseService, Evo: evo, KB: kb, OrgID: orgID, Log: log,
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
		ID:              id,
		OrganizationID:  nullUUID(org.ID),
		DisplayName:     orDefault(cfg.WaAccountDisplayName, "WhatsApp"),
		OwnerJID:        config.CanonicalJID(ownerJID),
		PhoneNumber:     config.PhoneFromJID(config.CanonicalJID(ownerJID)),
		InstanceName:    cfg.EvolutionInstance,
		ConnectionState: "connected",
	})
	if err != nil {
		fatal("seed account", err)
	}
	log.Info("account seeded", "id", acct.ID, "owner_jid", acct.OwnerJID)
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
