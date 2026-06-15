// Command xchats is the Build 0 backend: API edge + webhook ingress + in-process
// workers + the hardcoded AI stub. Subcommands: serve (default), migrate,
// webhook-set, seed.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/assistant"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

const webhookTokenHeader = "X-Webhook-Token"

var webhookEvents = []string{
	"messages.upsert", "messages.update", "send.message",
	"connection.update", "contacts.update", "chats.upsert",
}

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
	default:
		log.Error("unknown command", "cmd", cmd)
		os.Exit(2)
	}
}

func runServe(cfg *config.Config, log *slog.Logger) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st := mustStore(cfg, log)
	defer st.Close()
	if err := store.RunMigrations(ctx, st.Pool(), migrations.FS); err != nil {
		fatal("migrate", err)
	}
	accountID := seed(ctx, cfg, st, log)

	blobStore, err := blob.NewDisk(cfg.BlobDir)
	if err != nil {
		fatal("blob", err)
	}
	drafter, err := assistant.NewStub(blobStore, "")
	if err != nil {
		fatal("assistant", err)
	}
	q := queue.NewInMem(2048, cfg.QueueWorkers, log)
	hub := realtime.NewHub()
	evo := evolution.NewHTTP(cfg.EvolutionBaseURL, cfg.EvolutionAPIKey, cfg.EvolutionInstance)

	w := &worker.Worker{Store: st, Queue: q, Evo: evo, Blob: blobStore, Hub: hub, Drafter: drafter, Log: log}
	q.Start(ctx, w.Handle)

	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Queue: q, Hub: hub, Blob: blobStore,
		Drafter: drafter, AccountID: accountID, Log: log,
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
	evo := evolution.NewHTTP(cfg.EvolutionBaseURL, cfg.EvolutionAPIKey, cfg.EvolutionInstance)
	if err := evo.SetWebhook(ctx, url, webhookTokenHeader, cfg.WebhookToken, webhookEvents); err != nil {
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
	evo := evolution.NewHTTP(cfg.EvolutionBaseURL, cfg.EvolutionAPIKey, cfg.EvolutionInstance)
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
