// Command xchats is the xchats backend: API edge + webhook ingress + in-process
// workers + the multichannel response service. Subcommands: serve (default),
// migrate, seed, seed-kb-demo, simulate-message, kb-load, backup, check,
// restore, admin-credential, reset-admin-password.
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
	"github.com/yerassyldanay/xchats/backend/internal/appdirs"
	"github.com/yerassyldanay/xchats/backend/internal/automation"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/chat"
	"github.com/yerassyldanay/xchats/backend/internal/chatkb"
	"github.com/yerassyldanay/xchats/backend/internal/chatstore"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/dbops"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/inboxmedia"
	"github.com/yerassyldanay/xchats/backend/internal/kbimport"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/llmprovider"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
	"github.com/yerassyldanay/xchats/backend/internal/messengerish"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/metaingest"
	"github.com/yerassyldanay/xchats/backend/internal/ngrokapi"
	"github.com/yerassyldanay/xchats/backend/internal/providerhealth"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/internal/simreceipts"
	"github.com/yerassyldanay/xchats/backend/internal/simulator"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/telemetry"
	"github.com/yerassyldanay/xchats/backend/internal/tgingest"
	"github.com/yerassyldanay/xchats/backend/internal/tgpoller"
	"github.com/yerassyldanay/xchats/backend/internal/tunnel"
	"github.com/yerassyldanay/xchats/backend/internal/updatecheck"
	"github.com/yerassyldanay/xchats/backend/internal/version"
	"github.com/yerassyldanay/xchats/backend/internal/whatsappcloud"
	"github.com/yerassyldanay/xchats/backend/internal/whatsmeow"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// Media sweep cadence (Telegram and Meta channels alike). The retry delay is
// what keeps a permanently broken file handle from becoming a hot loop; the
// batch bounds one pass.
const (
	mediaSweepEvery      = 5 * time.Minute
	mediaSweepRetryAfter = 2 * time.Minute
	mediaSweepBatch      = 100
)

// Instagram token refresh cadence — a long-lived user token is good for
// ~60 days; checking twice a day with a week's lead time leaves ample
// margin, and Meta itself refuses a refresh attempt on a token less than
// 24h old (tokenRefreshMinAge), so there is nothing to gain from checking
// more often than that anyway.
const (
	tokenRefreshEvery  = 12 * time.Hour
	tokenRefreshBefore = 7 * 24 * time.Hour
	tokenRefreshMinAge = 24 * time.Hour
)

// Channel automation (internal/automation) cadence. claimEvery is short —
// debounce waits are single-digit-to-60 seconds, so polling latency should
// be too. recoverEvery/stuckAfter bound the crash-recovery reconcile pass
// (also run once at startup): a dispatch job still 'processing' longer than
// stuckAfter is assumed orphaned by a crash and re-published.
const (
	automationDispatchWorkers = 4
	automationClaimEvery      = 1 * time.Second
	automationRecoverEvery    = 1 * time.Minute
	automationStuckAfter      = 2 * time.Minute
)

// Campaigns (internal/campaign) cadence. campaignTickEvery only bounds how
// promptly a newly running campaign (or a newly due retry) is noticed — the
// real send cadence is governed entirely by each account's own pacing (see
// campaign.Scheduler.tick's own doc comment), so this can stay short without
// risking a burst of sends. campaignMaintenanceEvery/campaignDisconnectEvery
// are both low-frequency bookkeeping passes; campaignDisconnectAfter matches
// the plan's ">60s disconnected" auto-pause threshold.
const (
	campaignTickEvery            = 2 * time.Second
	campaignMaintenanceEvery     = 15 * time.Second
	campaignDisconnectCheckEvery = 15 * time.Second
	campaignDisconnectAfter      = 60 * time.Second
	campaignPruneEvery           = 1 * time.Hour
	campaignAccountConcurrency   = 8
)

func main() {
	cfgPath := flag.String("config", "", "path to config.yaml (default: $XCHATS_CONFIG, then ./config.yaml, then the OS config directory)")
	flag.Parse()

	cmd := "serve"
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
	}

	// resolveConfigPath/applyDesktopDefaults are no-ops in the server build
	// (desktop_off.go) and, under -tags desktop, the two adjustments a
	// packaged app needs: read config from the OS config directory rather
	// than the launcher's working directory, and resolve storage paths
	// against the OS application data directory. Everything after this point
	// is identical in both builds.
	cfg, err := config.Load(resolveConfigPath(*cfgPath))
	if err != nil {
		fatal("load config", err)
	}
	if err := applyDesktopDefaults(cfg); err != nil {
		fatal("desktop config", err)
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
	case "admin-credential":
		runAdminCredential(flag.Args()[1:])
	case "reset-admin-password":
		runResetAdminPassword(cfg, log, flag.Args()[1:])
	default:
		log.Error("unknown command", "cmd", cmd)
		os.Exit(2)
	}
}

func runServe(cfg *config.Config, log *slog.Logger) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	credsChain := openCredentialsAndProvisionSecrets(ctx, cfg, log)
	settingsStore := settings.NewStore(resolveConfigDir(log))
	// applyNgrokPublicOrigin overwrites cfg.Server.APIBaseURL with the tunnel's
	// HTTPS domain, so the operator's own configured value — the only honest
	// signal of deploy-for-real intent — has to be read before that call, not
	// after. See shouldValidateProductionConfig.
	configuredAPIBaseURL := cfg.Server.APIBaseURL
	autoStartTunnel, err := applyNgrokPublicOrigin(ctx, cfg, ngrokCredsFrom(credsChain), settingsStore, ngrokapi.NewClient())
	if err != nil {
		log.Warn("ngrok static domain is not usable as the MCP public origin", "err", err)
	}
	if autoStartTunnel {
		log.Info("using ngrok static domain as the MCP public origin", "public_origin", cfg.ResolvedAPIBaseURL())
	}

	if shouldValidateProductionConfig(cfg, configuredAPIBaseURL) {
		if problems := validateProductionConfig(cfg, credsChain != nil); len(problems) > 0 {
			fatal("production config", fmt.Errorf("%s", strings.Join(problems, "; ")))
		}
	}

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
	bootstrapCredentialPath, minted, err := ensureBootstrapAdminPassword(ctx, cfg, st)
	if err != nil {
		fatal("bootstrap admin password", err)
	}
	if minted {
		log.Info("one-time admin password created", "retrieve_with", "xchats admin-credential show")
	}
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

	llmRegistry := llmprovider.NewRegistry()
	populateLLMRegistry(ctx, llmRegistry, credsChain, settingsStore, cfg)
	// llmRefresh re-resolves every provider's client from scratch — the
	// Settings UI's save/delete-credential handlers call this (httpapi.Deps.
	// LLMRefresh) so a just-saved key (or a just-deleted one) takes effect
	// on the very next draft, with no restart. context.Background(): this
	// runs on whatever goroutine an HTTP handler triggers it from, well
	// after that request's own context is a meaningful deadline for a
	// background maintenance op like this one.
	llmRefresh := func() {
		populateLLMRegistry(context.Background(), llmRegistry, credsChain, settingsStore, cfg)
	}
	llmParams := func() response.LLMParams { return resolveLLMParams(settingsStore, cfg) }
	engine := &response.Engine{LLMs: llmRegistry, Params: llmParams}
	startupParams := llmParams()
	if _, err := llmRegistry.Client(startupParams.DefaultModel); err != nil {
		log.Warn("no API key configured for the default LLM provider; AI drafts will fail until one is added via Settings",
			"provider", startupParams.DefaultModel.Provider, "err", err)
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
		"provider", startupParams.DefaultModel.Provider, "model", startupParams.DefaultModel.Model, "prompt_ref", aiprompt.PromptRefShopKBV5)

	q := queue.NewInMem(2048, cfg.System.QueueWorkers, log)
	hub := realtime.NewHub()

	// automationScheduler is channel-level debounce + scheduled auto-reply
	// (internal/automation): whatsmeow and tgingest hand every genuinely new
	// inbound customer message to it instead of enqueueing an AI draft
	// directly, and its Runner reuses responseService's own
	// Conversations/KnowledgeBase/Engine (swapping in a version-gated
	// Drafts writer per generation) so a debounced/scheduled reply is
	// generated through the exact same grounded pipeline as every other
	// draft.
	automationRunner := &automation.Runner{Store: st, Response: responseService, Queue: q, Hub: hub, Log: log}
	automationScheduler := automation.NewScheduler(st,
		automation.Config{SystemDefaultWaitSeconds: cfg.System.CustomerMessageWaitSeconds},
		automationRunner, log, 0)

	// providerHealth is the self-healing status surface: every LLM call the
	// engine makes reports through it (engine.StatusHook, set below), and
	// every genuine health transition (not routine transient errors — see
	// Tracker.Report's own doc comment) broadcasts live over hub for the
	// NavRail badge, on top of the on-demand snapshot GET /settings/
	// provider-health serves a client that connects after the fact.
	providerHealth := providerhealth.NewTracker(func(s providerhealth.Status) {
		hub.Broadcast("integration.status_changed", s)
	})
	engine.StatusHook = providerHealth.Report

	// updateChecker asks GitHub's public releases API, cached well past any
	// single request — see internal/updatecheck's own doc comment for why a
	// failed/empty check is never treated as an error.
	updateChecker := updatecheck.NewChecker("yerassyldanay/xchats", version.Version)

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
	tgProc := tgingest.New(tgingest.Deps{Store: st, Queue: q, Hub: hub, Automation: automationScheduler, Log: log})
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
		Automation:   automationScheduler,
		Log:          log,
	})
	if err != nil {
		fatal("whatsmeow", err)
	}
	defer waMgr.Close()

	// Meta channels (Instagram Direct, Messenger, WhatsApp Cloud API — see
	// internal/meta's package doc). metaClient/waCloudClient/inboxSigner are
	// stateless HTTP wrappers built unconditionally at boot, exactly like tg
	// above: BYO-App credentials are resolved per-call from credsChain
	// (metaCredentialsAdapter, internal/httpapi), not baked in here, so a
	// deployment with no Meta App ID/Secret saved yet still boots cleanly —
	// every Meta route just answers META_APP_NOT_CONFIGURED until one is.
	metaClient := meta.NewHTTP(cfg.MetaResolvedGraphAPIVersion(), log)
	waCloudClient := whatsappcloud.NewClient(metaClient)
	// messengerishClient is shared by Instagram Direct and Messenger (Phase
	// 5) — both speak the same Graph send/webhook shape, see
	// internal/messengerish's own package doc.
	messengerishClient := messengerish.NewClient(metaClient)
	inboxSigner := inboxmedia.NewSigner(cfg.SessionSecret)
	// metaProc is the shared Meta ingest core (internal/metaingest) — the
	// exact tgProc pattern, generalized: every Meta channel's webhook
	// handler (today: WhatsApp Cloud; Instagram/Messenger from Phase 4/5)
	// feeds its normalized events through this one Process/ApplyStatus pair.
	metaProc := metaingest.New(metaingest.Deps{Store: st, Queue: q, Hub: hub, Automation: automationScheduler, Log: log})

	senders := messaging.NewSenderRegistry()
	senders.Register(messaging.ChannelWhatsApp, waMgr.ChannelSender())
	senders.Register(messaging.ChannelSimulator, simulator.NewChannelSender())
	senders.Register(messaging.ChannelTelegram, telegram.NewChannelSender(tg, st, blobStore))
	// *store.Store satisfies whatsappcloud.AccountSource and MediaSource
	// directly (ChannelExternalAccountID/ChannelCredentialsSecret/
	// ChannelOutboundMediaForSigning) — no adapter needed, unlike telegram's
	// own NewChannelSender which still takes st for the same reason.
	senders.Register(messaging.ChannelWhatsAppCloud,
		whatsappcloud.NewChannelSender(waCloudClient, st, st, inboxSigner, cfg.ResolvedAPIBaseURL()+"/meta/api/v1/media"))
	senders.Register(messaging.ChannelInstagram,
		messengerish.NewChannelSender(messengerishClient, st, st, inboxSigner, true, cfg.ResolvedAPIBaseURL()+"/meta/api/v1/media"))
	senders.Register(messaging.ChannelMessenger,
		messengerish.NewChannelSender(messengerishClient, st, st, inboxSigner, false, cfg.ResolvedAPIBaseURL()+"/meta/api/v1/media"))

	w := &worker.Worker{
		Store: st, Queue: q, TG: tg, WACloud: waCloudClient, MetaClient: metaClient, Blob: blobStore, Hub: hub,
		Response: responseService, Senders: senders, Log: log,
	}
	q.Start(ctx, w.Handle)

	// campaignScheduler is Campaigns' own independent background process
	// (internal/campaign): it discovers which accounts have a RUNNING
	// campaign and drains their eligible recipients through the exact same
	// internal/outbound.Deliver every manual/AI/automation send uses —
	// called directly, never through q, since at-most-once delivery needs
	// the ledger commit to happen before the provider call (see
	// internal/outbound's own package doc comment).
	campaignRunner := &campaign.Runner{Store: st, Blob: blobStore, Senders: senders, Hub: hub, Log: log}
	campaignScheduler := campaign.NewScheduler(st, campaignRunner, campaign.Config{
		TickEvery: campaignTickEvery, MaintenanceEvery: campaignMaintenanceEvery,
		DisconnectCheckEvery: campaignDisconnectCheckEvery, DisconnectAfter: campaignDisconnectAfter,
		PruneEvery: campaignPruneEvery, AccountConcurrency: campaignAccountConcurrency,
	}, log)
	// simulatorReceipts is the Simulator channel's own delivery-receipt
	// source: a real channel's sent->delivered->read progression is driven
	// by an inbound webhook (see store.Store.AdvanceDeliveryState's own doc
	// comment); Simulator has no such webhook, so this sweep plays that
	// same role for it, on the exact same store method, so a campaign sent
	// through Simulator completes and updates its message statuses through
	// the identical pipeline a live channel would.
	simulatorReceipts := simreceipts.NewReceiptSimulator(st, hub, simreceipts.Config{}, log)
	// Instagram's long-lived user token is the one Meta-channel credential
	// that silently expires with no renewal otherwise (WhatsApp Cloud's
	// business token and Telegram's bot token do not) — see
	// worker/meta_tokens.go's own doc comment.
	w.StartInstagramTokenRefresher(ctx, tokenRefreshEvery, tokenRefreshBefore, tokenRefreshMinAge)
	// Attachments whose bytes never arrived are retried from their own media
	// row — the durable work item that replaces an inbound-event table. The
	// startup pass picks up whatever a crash or an outage left behind.
	w.StartMediaSweeper(ctx, mediaSweepEvery, mediaSweepRetryAfter, mediaSweepBatch)
	// One boot-time pass: compare every connected Meta account's registered
	// webhook origin against the public base URL just resolved above. See
	// worker.RepairStaleMetaOrigins' own doc comment for why a single pass
	// (not a ticker, not a live tunnel.Deps.OnStatus hook — the latter is
	// explicitly out of scope) is the right amount of machinery here.
	go func() {
		repaired, stale, err := w.RepairStaleMetaOrigins(ctx, cfg.MetaResolvedPublicBaseURL(), meta.DeriveVerifyToken(cfg.SessionSecret))
		if err != nil {
			log.Error("meta origin repair", "err", err)
			return
		}
		if repaired > 0 || stale > 0 {
			log.Info("meta origin repair", "repaired", repaired, "stale", stale)
		}
	}()
	// Reconnects every saved WhatsApp account without re-scanning a QR code.
	go waMgr.Start(ctx)
	// Claims due debounce deadlines into dispatch jobs and runs them; its
	// own startup pass re-publishes whatever a crash left mid-flight, the
	// same recovery philosophy as the Telegram media sweeper above.
	automationScheduler.Start(ctx, automationDispatchWorkers, automationClaimEvery, automationRecoverEvery, automationStuckAfter)
	// Its own startup pass reconciles any send left 'sending' by a crash
	// (ReconcileStuckSending) before the tick loop starts claiming new ones.
	campaignScheduler.Start(ctx)
	simulatorReceipts.Start(ctx)

	mcpAuthorizer, mcpSrv := buildMCPConnector(ctx, cfg, kb, blobStore, log)

	// kbImportSvc runs the structured KB import pipeline's background
	// workers (internal/kbimport): pass-1 extraction and pass-2 synthesis,
	// both landing in kbd_draft only — see that package's own doc comment
	// for the safety boundary. Started before httpapi.New so the first
	// request the server ever handles already sees a non-nil pipeline.
	kbImportSvc := kbimport.New(kbimport.Deps{
		KB: kb, Blob: blobStore, Credentials: credsChain, Settings: settingsStore,
		LLM: llmRegistry, Hub: hub, Log: log, AllowPrivateFetch: cfg.KBAllowPrivateFetch,
	}, kbimport.DefaultConfig())
	kbImportSvc.Start(ctx)

	// The Knowledge Base chat assistant (/chat). It shares this process's one
	// database connection (internal/dbx.Open refcounts by path, exactly like
	// kb above) and the SAME llmRegistry every other AI path already uses, so
	// a key saved in Settings takes effect here too with no extra wiring.
	// Retrieval goes through chatkb over the existing kbstore — the chat
	// never reads a KB table itself.
	chatDB, err := chatstore.New(ctx, cfg.Storage.DBPath)
	if err != nil {
		fatal("chatstore", err)
	}
	defer chatDB.Close()
	chatSvc := chat.New(chat.Deps{
		Store:  chatDB,
		KB:     chatkb.NewStoreService(kb),
		LLMs:   llmRegistry,
		Params: func() chat.Params { return resolveChatParams(settingsStore, cfg) },
		Log:    log,
	})

	// The tunnel needs to serve the SAME router httpapi.New's Server owns —
	// but httpapi.Deps also needs the tunnel (for /settings/tunnel/*)
	// before that router exists. Broken by constructing the Server with no
	// tunnel, building its router, THEN constructing the tunnel manager
	// against that exact router and handing it back via SetTunnel — routes
	// only read s.tunnel at REQUEST time, never at Router()-build time, so
	// this ordering is safe.
	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Queue: q, Hub: hub, Blob: blobStore,
		Response: responseService, WA: waMgr, TG: tg, TGProcessor: tgProc, TGPoller: tgMgr, KB: kb,
		KBRepo: cachedKB, KBInvalidator: cachedKB,
		OrgID: orgID, Log: log,
		MetaClient: metaClient, MetaProcessor: metaProc, WACloudClient: waCloudClient, InboxMediaSigner: inboxSigner,
		MCPAuth: mcpAuthorizer, MCPServer: mcpSrv,
		BootstrapAdminCredentialPath: bootstrapCredentialPath,
		Credentials:                  credsChain, Settings: settingsStore, LLMRefresh: llmRefresh,
		ProviderHealth: providerHealth, UpdateChecker: updateChecker,
		KBImport: kbImportSvc, Chat: chatSvc,
	})
	router := srv.Router()

	// Only constructed when a secure credential store is available — with
	// none, the tunnel has no way to ever retrieve an authtoken, so it is
	// absent (nil) rather than present-but-guaranteed-to-fail; every
	// /settings/tunnel/* route already handles a nil Tunnel as "feature not
	// available" (503 ErrTunnelUnavailable).
	var tunnelMgr tunnel.Tunnel
	if credsChain != nil {
		tunnelMgr = tunnel.NewManager(tunnel.Deps{Creds: credsChain, Settings: settingsStore, Handler: router, Log: log})
	}
	srv.SetTunnel(tunnelMgr)

	httpServer := &http.Server{
		Addr:    cfg.Server.HTTPAddr,
		Handler: router,
		// A8: an unauthenticated peer that opens a connection and trickles
		// headers/body one byte at a time (Slowloris) previously tied up a
		// connection indefinitely — net/http's zero-value Server has no
		// timeouts at all. WriteTimeout is deliberately left at its zero
		// value (unlimited): GET /xchats/api/v1/realtime is a long-lived SSE
		// stream (internal/httpapi/sse.go) that must not be cut off mid-
		// stream, and frontend/nginx.conf's proxy already assumes exactly
		// that (proxy_read_timeout 1h).
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	go func() {
		log.Info("backend listening", "addr", cfg.Server.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatal("listen", err)
		}
	}()

	var tunnelStartDone chan struct{}
	if autoStartTunnel && tunnelMgr != nil {
		tunnelStartDone = make(chan struct{})
		go func() {
			defer close(tunnelStartDone)
			if err := tunnelMgr.Start(ctx); err != nil {
				log.Warn("automatic ngrok tunnel start failed", "last_error", tunnelMgr.Status().LastError)
				return
			}
			log.Info("ngrok tunnel started automatically", "public_url", tunnelMgr.Status().PublicURL)
		}()
	}

	// The server build blocks here until SIGINT/SIGTERM. The desktop build
	// (-tags desktop) instead runs the Wails window on this goroutine and
	// returns when the user closes it, cancelling ctx on the way out — so
	// the teardown below is the same sequence in both cases.
	runUntilShutdown(ctx, stop, shellDeps{Router: router, Hub: hub, Log: log, Addr: cfg.Server.HTTPAddr})
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	if tunnelStartDone != nil {
		<-tunnelStartDone
	}
	if tunnelMgr != nil {
		_ = tunnelMgr.Stop(shutdownCtx)
	}
	// tgMgr and automationScheduler before q: the poller (via tgProc) and a
	// scheduled_auto auto-send (via Runner.dispatchSend) both publish to the
	// queue, so both producers must stop before the queue stops accepting.
	// Explicit here (not a defer) so the ordering relative to q.Close() is
	// guaranteed rather than left to defer's LIFO stacking. campaignScheduler
	// never publishes to q (see its own construction comment above) so has
	// no ordering requirement against q.Close() — stopped alongside the
	// others here anyway, before the deferred waMgr.Close()/st.Close(), so a
	// tick already in flight finishes against still-live dependencies.
	tgMgr.Close()
	automationScheduler.Stop()
	campaignScheduler.Stop()
	simulatorReceipts.Stop()
	q.Close()
	// kbImportSvc has no producer/consumer relationship with q (its job
	// queue is kbd_materials, claimed directly via kbstore) — it only needs
	// to stop before the deferred kb.Close()/st.Close() run.
	kbImportSvc.Stop()
}

// resolveConfigDir resolves the OS-appropriate per-user config directory
// settings.Store persists settings.json under. A resolution failure (only
// on the exotic $HOME/%APPDATA%-unset edge case — see appdirs.ConfigDir)
// falls back to "." rather than refusing to boot over what is fundamentally
// a non-essential feature: Settings still works for this run, just written
// wherever the process's current directory happens to be.
func resolveConfigDir(log *slog.Logger) string {
	dir, err := appdirs.ConfigDir("xchats")
	if err != nil {
		log.Warn("could not resolve the OS application config directory; Settings will be written to the current directory instead", "err", err)
		return "."
	}
	return dir
}

// openCredentialsAndProvisionSecrets opens the credential store (the OS
// keychain, or a file store the deployment opted into — see
// credentials.Open) and, when one is available, resolves xchats' four
// internally-managed secrets through it — the MCP OAuth CSRF signing key
// (cfg.SessionSecret), the Telegram bot-token-at-rest encryption key
// (cfg.TelegramCredentialsEncKey), the MCP JWT signing key
// (cfg.MCPJWTSigningKey), and the Telegram webhook secret_token
// (cfg.TelegramWebhookSecret) — writing the resolved values straight back
// into cfg, so every existing consumer of those four fields
// (secretbox.FromEnvValue below, mcpauth.NewSigningKeyFromSeed in
// buildMCPConnector, httpapi's CSRF secret, TelegramResolvedWebhookSecret)
// picks them up completely unchanged.
//
// Each of the four is durably generated once and reused on every later
// boot: this is what turns "set TG_CREDENTIALS_ENC_KEY/MCP_JWT_SIGNING_KEY
// by hand before first boot" from a required step into a zero-config
// default. When no store is available, cfg keeps whatever raw config/env
// value it already had (most likely empty), and every downstream
// consumer's own existing warn-and-degrade behavior (an ephemeral MCP key,
// credentials-at-rest disabled, ...) applies exactly as it did before this
// function existed.
//
// The returned *credentials.Chain is nil exactly when no secure store was
// available — callers (runServe, for the LLM registry and the Settings/
// tunnel surfaces) must treat that as "the credential-backed features are
// unavailable this boot," never dereference it unconditionally.
func openCredentialsAndProvisionSecrets(ctx context.Context, cfg *config.Config, log *slog.Logger) *credentials.Chain {
	dataDir, err := appdirs.DataDir("xchats")
	if err != nil {
		log.Warn("could not resolve the OS application data directory; the file-based credential fallback is unavailable", "err", err)
	}
	store, err := credentials.Open(credentials.OpenOptions{
		DataDir:   dataDir,
		AllowFile: credentials.AllowFileFromEnv(),
	})
	if err != nil {
		logDegradedSecrets(log, cfg, "no secure credential store available", err)
		return nil
	}
	secrets, err := credentials.Provision(ctx, store)
	if err != nil {
		logDegradedSecrets(log, cfg, "provisioning internal secrets failed", err)
		return store
	}
	cfg.SessionSecret = secrets.SessionSecret
	cfg.TelegramCredentialsEncKey = secrets.TGCredentialsEncKey
	cfg.MCPJWTSigningKey = secrets.MCPJWTSigningKey
	cfg.TelegramWebhookSecret = secrets.WebhookSecret
	return store
}

// logDegradedSecrets is the shared A5 hardening for both ways
// openCredentialsAndProvisionSecrets can end up with cfg's four
// internally-managed secrets left empty: no store at all (credentials.Open
// failed) or a store that opened but couldn't provision (credentials.
// Provision failed). Both used to log a single Warn line and continue
// silently; this escalates to Error and enumerates exactly what stops
// working — and, the actual hardening rather than just louder logging,
// refuses to boot at all when Telegram webhook mode is configured. A6 makes
// an unconfigured webhook secret reject every call, so booting into that
// exact combination would silently and permanently break the Telegram
// channel instead of failing at the one moment an operator would see why.
func logDegradedSecrets(log *slog.Logger, cfg *config.Config, reason string, err error) {
	log.Error(reason+"; internally-managed secrets are only as durable as config/env, and several features are now OFF",
		"err", err,
		"disabled", []string{
			"Settings UI credential storage and the ngrok tunnel (no session/CSRF signing key)",
			"Telegram bot token encryption at rest (no encryption key)",
			"MCP connector token persistence across restarts (ephemeral signing key)",
			"the Telegram webhook (A6 rejects every call with no secret configured)",
		})
	if cfg.TelegramResolvedMode() == "webhook" {
		fatal("credentials", fmt.Errorf(
			"%s, and Telegram webhook mode is configured (a public webhook base URL is set) — "+
				"the webhook secret can never be provisioned, so every inbound Telegram update would be "+
				"rejected forever; set XCHATS_ALLOW_FILE_CREDENTIALS=1 (see docs/release/credentials.md) "+
				"or switch Telegram to polling mode", reason))
	}
}

// llmProviderCfg is one provider's static config-side fallback — its raw,
// env-sourced API key and base URL, read only when creds has nothing for it
// (no secure store, or the key was never saved through the Settings UI).
var llmProviderCfg = []struct {
	name, credKey string
	apiKey        func(*config.Config) string
	baseURL       func(*config.Config) string
}{
	{"openrouter", "openrouter.api_key", func(c *config.Config) string { return c.OpenRouterAPIKey }, func(c *config.Config) string { return c.OpenRouterBaseURL }},
	{"openai", "openai.api_key", func(c *config.Config) string { return c.OpenAIAPIKey }, func(c *config.Config) string { return c.OpenAIBaseURL }},
	{"gemini", "gemini.api_key", func(c *config.Config) string { return c.GeminiAPIKey }, func(c *config.Config) string { return c.GeminiBaseURL }},
}

// populateLLMRegistry (re)registers one OpenAICompatible client per known
// LLM provider that has a resolvable API key, IN PLACE on reg — called once
// at boot and again by the LLMRefresh closure after any credential
// save/delete (internal/httpapi/settings.go), so response.Engine — which
// holds this exact *llmprovider.Registry as its llm.Registry, safe for
// concurrent Register/Deregister alongside Client via its own RWMutex —
// picks up the change on its very next Generate call, no restart.
//
// Per provider, in order: creds (fresh on every call — a Settings-UI-saved
// value, the OS keychain, or creds' own env-var overlay) when creds is
// non-nil, else cfg's raw env-sourced field. The base URL follows the same
// idea: settingsStore's ProviderSettings.BaseURL when set, else cfg's own
// *_BASE_URL field, else llmprovider.DefaultBaseURL. A provider with no key
// anywhere is deregistered — never left holding a stale client for a
// credential that was just deleted.
func populateLLMRegistry(ctx context.Context, reg *llmprovider.Registry, creds *credentials.Chain, settingsStore *settings.Store, cfg *config.Config) {
	var providerSettings map[string]settings.ProviderSettings
	if st, err := settingsStore.Load(); err == nil {
		providerSettings = st.Providers
	}
	timeoutSeconds := cfg.LLMDraftTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	timeout := time.Duration(timeoutSeconds) * time.Second

	for _, p := range llmProviderCfg {
		key := p.apiKey(cfg)
		if creds != nil {
			if v, err := creds.Get(ctx, credentials.Key(p.credKey)); err == nil && v != "" {
				key = v
			}
		}
		if strings.TrimSpace(key) == "" {
			reg.Deregister(p.name)
			continue
		}
		baseURL := p.baseURL(cfg)
		if ps, ok := providerSettings[p.name]; ok && ps.BaseURL != "" {
			baseURL = ps.BaseURL
		}
		if baseURL == "" {
			baseURL = llmprovider.DefaultBaseURL(p.name)
		}
		reg.Register(p.name, llmprovider.NewOpenAICompatible(baseURL, key, p.name, timeout))
	}
}

// resolveLLMParams reads the response engine's CURRENT model configuration
// from settingsStore — response.Engine calls this fresh on every Generate
// (see Engine.Params), so a Settings UI change takes effect on the very
// next request. A settings.Store.Load failure (a transient read problem —
// Load itself already treats a merely-missing file as "use defaults") falls
// back to cfg's own raw env-sourced fields rather than blocking a customer
// response over it.
func resolveLLMParams(settingsStore *settings.Store, cfg *config.Config) response.LLMParams {
	st, err := settingsStore.Load()
	if err != nil {
		return response.LLMParams{
			DefaultModel: llm.ModelRef{
				Provider: orDefault(cfg.LLMDefaultProvider, "openrouter"),
				Model:    orDefault(cfg.LLMDefaultModel, "google/gemini-2.5-flash"),
			},
			MaxTokens: cfg.LLMDraftMaxTokens, Temperature: cfg.LLMDraftTemperature, RetryEnabled: cfg.LLMDraftRetry,
		}
	}
	return response.LLMParams{
		DefaultModel: llm.ModelRef{Provider: st.LLM.DefaultProvider, Model: st.LLM.DefaultModel},
		MaxTokens:    st.LLM.MaxTokens,
		Temperature:  st.LLM.Temperature,
		RetryEnabled: st.LLM.Retry,
	}
}

// resolveChatParams is resolveLLMParams' Knowledge-Base-chat counterpart:
// the chat assistant runs on the SAME provider and model the rest of xchats
// is configured with (one place to change the model, not two), but with its
// own answer-length and history-window knobs — a chat answer explaining what
// changed across a knowledge base is a different shape of output from a
// short customer reply, and LLM_DRAFT_MAX_TOKENS is sized for the latter.
// Read fresh per request, so a Settings change applies to the next message.
func resolveChatParams(settingsStore *settings.Store, cfg *config.Config) chat.Params {
	p := chat.Params{
		Model: llm.ModelRef{
			Provider: orDefault(cfg.LLMDefaultProvider, "openrouter"),
			Model:    orDefault(cfg.LLMDefaultModel, "google/gemini-2.5-flash"),
		},
		Temperature:   cfg.LLMDraftTemperature,
		MaxTokens:     cfg.ChatMaxTokens,
		HistoryWindow: cfg.ChatHistoryWindow,
	}
	if st, err := settingsStore.Load(); err == nil {
		p.Model = llm.ModelRef{Provider: st.LLM.DefaultProvider, Model: st.LLM.DefaultModel}
		p.Temperature = st.LLM.Temperature
	}
	return p
}

// shouldValidateProductionConfig (A10) reports whether runServe should run
// validateProductionConfig's safety gate: an explicit ENVIRONMENT=production,
// OR a real (non-localhost) API_BASE_URL or FRONTEND_BASE_URL configured
// regardless of ENVIRONMENT.
//
// The second condition is the actual A10 hardening. Before it, the gate only
// ever ran when an operator BOTH configured real public URLs AND remembered
// to also set ENVIRONMENT=production — and deploy/docker-compose.yaml never
// sets the latter (its shipped API_BASE_URL/FRONTEND_BASE_URL default to
// localhost precisely so the stock `docker compose up` stays a safe, ungated
// local quickstart), so a real deployment that only did the former — set
// real URLs, never touched ENVIRONMENT — silently skipped every check below.
// Configuring a real public URL is itself a strong enough signal of intent
// to deploy for real that it should trigger the gate on its own.
//
// configuredAPIBaseURL is cfg.Server.APIBaseURL as the OPERATOR set it,
// captured before applyNgrokPublicOrigin overwrote it with the embedded
// tunnel's HTTPS domain — and passing it is load-bearing, not tidiness.
// Reading the mutated cfg value here meant that merely connecting ngrok from
// the Settings UI (a local-dev convenience: it is how you expose a laptop to
// an MCP host or Telegram) handed this gate a non-localhost API_BASE_URL, so
// every such dev box silently promoted itself to "production" and then fatally
// failed the very next check for still having a localhost FRONTEND_BASE_URL —
// which is the stock compose default, and correct for a laptop. A tunnel the
// app started for itself says nothing about deploy intent; only what the
// operator configured does.
func shouldValidateProductionConfig(cfg *config.Config, configuredAPIBaseURL string) bool {
	return cfg.IsProduction() || !isLocalBaseURL(configuredAPIBaseURL) || !isLocalBaseURL(cfg.Server.FrontendBaseURL)
}

// validateProductionConfig returns every reason cfg is unsafe to run for
// real (plan Task 17's release gate, extended by A10): a base URL still
// pointing at localhost/loopback, a wildcard CORS origin, or no secure
// credential store available. An empty return means clear to boot.
//
// This used to also fatal on an ephemeral MCP signing key
// (MCPJWTSigningKey unset or otherwise invalid — every replica would then
// mint tokens no other replica, and no restart, can verify). That check is
// gone: provisionSystemSecrets now durably generates and persists
// MCPJWTSigningKey (and the other three internally-managed secrets) through
// internal/credentials before this function ever runs, whenever a secure
// credential store is available — which the Docker image always opts into
// (XCHATS_ALLOW_FILE_CREDENTIALS=1). hasCredentialStore is exactly that
// condition, checked independently below rather than folded into the two
// base-URL checks: A5 already fatals immediately on no-store-plus-webhook-
// mode, but a production deployment with Telegram in polling mode (or no
// Telegram at all) would otherwise never learn its Settings UI credential
// storage and ngrok tunnel are silently disabled until an operator went
// looking for why.
//
// secure_cookies is deliberately NOT checked here: internal/httpapi's
// requestIsSecure already marks the session cookie Secure dynamically from
// X-Forwarded-Proto (the common case — a reverse proxy or the embedded
// ngrok tunnel terminates TLS in front of the app), so a plain-HTTP-behind-
// a-trusted-proxy deployment with the static flag left false is correctly
// configured, not a misconfiguration this gate should fail on.
func validateProductionConfig(cfg *config.Config, hasCredentialStore bool) []string {
	var problems []string
	if isLocalBaseURL(cfg.Server.APIBaseURL) {
		problems = append(problems, fmt.Sprintf("API_BASE_URL=%q still points at localhost/loopback", cfg.Server.APIBaseURL))
	}
	if isLocalBaseURL(cfg.Server.FrontendBaseURL) {
		problems = append(problems, fmt.Sprintf("FRONTEND_BASE_URL=%q still points at localhost/loopback", cfg.Server.FrontendBaseURL))
	}
	for _, o := range cfg.Server.CORSOrigins {
		if o == "*" {
			problems = append(problems, `CORS_ORIGINS contains "*" — list the real frontend origin(s) instead`)
			break
		}
	}
	if !hasCredentialStore {
		problems = append(problems, "no secure credential store is available — session signing, Telegram token "+
			"encryption, and MCP token persistence are only as durable as config/env (see docs/release/credentials.md); "+
			"set XCHATS_ALLOW_FILE_CREDENTIALS=1 or run where an OS keychain is reachable")
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
