// Package config loads the merged application configuration: a committed,
// secrets-free config.yaml for boot/infra tunables, plus real process env
// vars as overrides/secrets on top of it (internal/credentials.EnvStore
// reads the same process env, so an operator's existing export keeps
// working — see that package's own doc comment). It also owns the
// deterministic wa_accounts.id derivation (uuidv5(XCHATS_WA_NS, owner_jid)).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/yerassyldanay/xchats/backend/internal/appdirs"
)

// xchatsWaNS is the fixed namespace for deriving wa_accounts.id from owner_jid.
// It must never change — the account id is uuidv5(xchatsWaNS, canonical(owner_jid)).
var xchatsWaNS = uuid.MustParse("a3f1d2c4-5e6b-47a8-9c0d-1e2f3a4b5c6d")

// ServerConfig is the HTTP edge's own boot/infra shape: listen address,
// discovery/link base URLs, and the CORS/proxy trust lists. Every field here
// is safe to commit — none of it is a secret.
type ServerConfig struct {
	HTTPAddr   string `yaml:"http_addr" env:"HTTP_ADDR"`
	APIBaseURL string `yaml:"api_base_url" env:"API_BASE_URL"`
	// FrontendBaseURL is the deployed Vue SPA's own public origin (a separate
	// service from the backend — see deploy/docker-compose.yaml). The MCP KB
	// Manager widget's "Review and publish in Xchats" link and the OAuth
	// consent page both need it to build a real URL into /playground.
	FrontendBaseURL string   `yaml:"frontend_base_url" env:"FRONTEND_BASE_URL"`
	CORSOrigins     []string `yaml:"cors_origins" env:"CORS_ORIGINS" envSeparator:","`
	// TrustedProxies lists the reverse-proxy IPs/CIDRs gin should trust
	// X-Forwarded-* headers from (gin's SetTrustedProxies). Empty means trust
	// NONE — c.ClientIP() falls back to the raw connection address rather
	// than any client-suppliable header. This must never default to "trust
	// everyone" (gin's own zero-value behavior): Host and X-Forwarded-* are
	// attacker-controlled on any request that doesn't actually come through
	// a configured proxy (plan Task 17).
	TrustedProxies []string `yaml:"trusted_proxies" env:"TRUSTED_PROXIES" envSeparator:","`
	SecureCookies  bool     `yaml:"secure_cookies" env:"SECURE_COOKIES"`
}

// StorageConfig is where every on-disk file this process owns lives.
// db_path has a committed default now (./data/xchats.db) — a fresh clone
// boots without DB_PATH set at all.
type StorageConfig struct {
	DBPath string `yaml:"db_path" env:"DB_PATH"`
	// WADeviceDBPath is whatsmeow's own device-session SQLite file — kept
	// separate from DBPath since whatsmeow's sqlstore manages that schema
	// entirely on its own (see internal/whatsmeow/store.go).
	WADeviceDBPath string `yaml:"wa_device_db_path" env:"WA_DEVICE_DB_PATH"`
	BlobDir        string `yaml:"blob_dir" env:"BLOB_DIR"`
}

// SystemConfig is process-wide operational tunables: logging, the queue's
// worker pool, session/auth limits, and feature gates.
type SystemConfig struct {
	LogLevel        string `yaml:"log_level" env:"LOG_LEVEL"`
	LogFormat       string `yaml:"log_format" env:"LOG_FORMAT"`
	QueueWorkers    int    `yaml:"queue_workers" env:"QUEUE_WORKERS"`
	SessionTTLHours int    `yaml:"session_ttl_hours" env:"SESSION_TTL_HOURS"`
	MinPasswordLen  int    `yaml:"min_password_len"`
	// SimulatorEnabled gates the /simulator/messages API (Phase 10): the route is
	// not registered at all when false. Defaults false — only a dev/staging
	// deployment should set SIMULATOR_ENABLED=true.
	SimulatorEnabled bool `yaml:"simulator_enabled" env:"SIMULATOR_ENABLED"`
	// CustomerMessageWaitSeconds is the system-wide default debounce wait for
	// channel-level automation (internal/automation): how long a chat must go
	// quiet after a customer message before a response is generated. Each
	// channel may override this within [automation.MinWaitSeconds,
	// automation.MaxWaitSeconds] via PUT /accounts/:id/automation — see
	// automation.EffectiveWaitSeconds, the one place default-vs-override
	// resolution happens.
	CustomerMessageWaitSeconds int `yaml:"customer_message_wait_seconds" env:"CUSTOMER_MESSAGE_WAIT_SECONDS"`
}

// TelegramModeConfig groups the Telegram settings that are boot/infra shaped
// — safe to commit to config.yaml — as opposed to the flat Telegram* secret
// fields below (bot tokens, encryption keys), which are credentials, or the
// runtime-editable settings that live in the Settings UI.
type TelegramModeConfig struct {
	// Mode is "webhook" | "polling" | "" (auto — see TelegramResolvedMode).
	Mode string `yaml:"mode" env:"TG_MODE"`
	// WebhookPublicBaseURL must be a public HTTPS origin — Telegram refuses
	// to register a webhook that is anything else. Leave it empty whenever
	// the embedded ngrok tunnel fronts this deployment: the tunnel's own
	// domain is then used automatically (see TelegramResolvedWebhookBaseURL),
	// so there is nothing to restate here. Set it only to front Telegram with
	// a DIFFERENT hostname than the rest of the app.
	WebhookPublicBaseURL string `yaml:"webhook_public_base_url" env:"TG_WEBHOOK_PUBLIC_BASE_URL"`
	// APIBaseURL overrides https://api.telegram.org (a local Bot API server,
	// or a test double) — an advanced override, rarely set.
	APIBaseURL string `yaml:"api_base_url" env:"TG_API_BASE_URL"`
}

// MetaModeConfig groups the official Meta channels' (Instagram Direct,
// Messenger, WhatsApp Cloud API) boot/infra settings — safe to commit. The
// App ID/Secret themselves are NOT here: BYO-App means every install's app
// credential is an operator-entered value in Settings (internal/credentials'
// "meta" provider), never a config.yaml/env value, so one xchats binary
// carries no default Meta app of its own.
type MetaModeConfig struct {
	// WebhookPublicBaseURL is the public HTTPS origin all three Meta webhook
	// callbacks (and the OAuth redirect URIs) are registered against. Empty
	// means the embedded ngrok tunnel's own domain is used automatically —
	// see MetaResolvedPublicBaseURL, the exact TelegramResolvedWebhookBaseURL
	// pattern. Set only to front Meta with a DIFFERENT hostname than the rest
	// of the app.
	WebhookPublicBaseURL string `yaml:"webhook_public_base_url" env:"META_WEBHOOK_PUBLIC_BASE_URL"`
	// GraphAPIVersion is the "/vXX.X/" segment every graph.facebook.com /
	// graph.instagram.com call uses. Meta deprecates versions on a schedule
	// independent of this codebase's release cadence, so this is a plain
	// config value an operator can bump without a code change or rebuild —
	// never hardcoded into a URL string. Empty falls back to
	// metaDefaultGraphAPIVersion.
	GraphAPIVersion string `yaml:"graph_api_version" env:"META_GRAPH_API_VERSION"`
}

// MCPConfig groups the MCP connector's TTL tunables (plan/mcp.md §3) — every
// field here is a lifetime in seconds/days, never a secret.
type MCPConfig struct {
	// AccessTokenTTLSeconds / RefreshTokenTTLDays: short-lived access tokens,
	// long-lived rotated-on-use refresh tokens.
	AccessTokenTTLSeconds int `yaml:"access_token_ttl_seconds" env:"MCP_ACCESS_TOKEN_TTL_SECONDS"`
	RefreshTokenTTLDays   int `yaml:"refresh_token_ttl_days" env:"MCP_REFRESH_TOKEN_TTL_DAYS"`
	// AuthCodeTTLSeconds bounds the PKCE authorization code's lifetime
	// between /oauth/authorize and the POST /oauth/token exchange.
	AuthCodeTTLSeconds int `yaml:"auth_code_ttl_seconds" env:"MCP_AUTH_CODE_TTL_SECONDS"`
	// UploadTokenTTLSeconds bounds a kb_media_upload signed upload URL.
	UploadTokenTTLSeconds int `yaml:"upload_token_ttl_seconds" env:"MCP_UPLOAD_TOKEN_TTL_SECONDS"`
	// MediaTokenTTLSeconds bounds a signed media-READ URL (the widget's
	// preview thumbnails and open/download links). Longer than the upload TTL
	// on purpose: expiry here means a broken <img> in a record the user left
	// open, the grant is read-only and single-object, and every kb_read mints
	// fresh URLs anyway — so the practical window is one navigation, not an
	// hour.
	MediaTokenTTLSeconds int `yaml:"media_token_ttl_seconds" env:"MCP_MEDIA_TOKEN_TTL_SECONDS"`
	// ReviewHandoffTTLSeconds bounds the one-time signed URL a tool result
	// hands the KB Manager widget for "Review and publish in Xchats" (plan
	// Task 15) — short-lived: it only needs to survive the click.
	ReviewHandoffTTLSeconds int `yaml:"review_handoff_ttl_seconds" env:"MCP_REVIEW_HANDOFF_TTL_SECONDS"`
}

// Config is the merged application configuration. Server/Storage/System/
// Telegram/MCP are the committed config.yaml's own nested shape (see the
// repo root config.yaml) — every env tag on their fields is unchanged from
// before this grouping, so existing env-var overrides (Docker/k8s pins)
// keep working unchanged; only the Go access path grew a `.Server`/
// `.Storage`/... prefix. Everything below them is either a secret (env-only,
// never in config.yaml) or a runtime setting Track 2 moves into the
// Settings UI/credential store — both stay flat and largely dormant here.
type Config struct {
	Server   ServerConfig       `yaml:"server"`
	Storage  StorageConfig      `yaml:"storage"`
	System   SystemConfig       `yaml:"system"`
	Telegram TelegramModeConfig `yaml:"telegram"`
	Meta     MetaModeConfig     `yaml:"meta"`
	MCP      MCPConfig          `yaml:"mcp"`

	// Environment is "development" (the default) or "production" — the
	// switch that turns dev-only conveniences (an ephemeral MCP signing key,
	// a localhost base URL) into hard startup failures instead of warnings
	// (plan Task 17). Anything other than the literal "production" is
	// treated as development, so an unset/misspelled value never accidentally
	// disables the checks it's supposed to enable.
	Environment string `yaml:"environment" env:"ENVIRONMENT"`

	// --- MCP connector secret (plan/mcp.md) ---
	// MCPJWTSigningKey seeds the Ed25519 keypair that signs MCP OAuth access
	// tokens (internal/mcpauth), parsed the same three ways as
	// TG_CREDENTIALS_ENC_KEY (64 hex chars, base64, or a 32-char literal). Unset
	// in dev falls back to an ephemeral in-memory key (logged loudly): every
	// previously issued token stops verifying on restart, and every replica in
	// a multi-instance deployment would mint tokens no other replica can check
	// — set this for anything beyond a single local process. Track 2C
	// auto-provisions and persists this once the credential store exists;
	// until then it is env-only.
	MCPJWTSigningKey string `env:"MCP_JWT_SIGNING_KEY"`

	// --- secrets (env only; Track 2C auto-provisions/persists these) ---
	SessionSecret string `env:"SESSION_SECRET"`

	// --- Telegram (Bot API) ---
	// The two non-secret Telegram URLs that used to live here moved into the
	// committed `telegram:` config.yaml section (TelegramModeConfig above) —
	// they are boot/infra config, not credentials, and keeping them env-only
	// forced every deployment to carry an environment block just to set them.
	// TelegramCredentialsEncKey is the AES-256-GCM key protecting stored bot
	// tokens (internal/secretbox, xchats.tg_credentials). Losing it means
	// re-pasting every bot token; it is never logged.
	TelegramCredentialsEncKey string `env:"TG_CREDENTIALS_ENC_KEY"`
	// TelegramWebhookSecret is the secret_token registered with setWebhook and
	// verified on the Telegram ingress. Empty means the ingress skips the
	// check — fine for local dev, never for a public deployment. Use
	// TelegramResolvedWebhookSecret to read the effective value.
	TelegramWebhookSecret string `env:"TG_WEBHOOK_SECRET"`

	// --- LLM / AI brain (key is a secret via env; the rest are dormant legacy
	// tunables — response.Service's own provider/model/tunables below are the
	// live path). When LLMAPIKey is empty the app falls back to the hardcoded
	// Stub drafter. ---
	LLMProvider    string  `env:"LLM_PROVIDER"`   // openrouter|openai|gemini
	LLMAPIKey      string  `env:"LLM_API_KEY"`    // secret
	LLMBaseURL     string  `env:"LLM_BASE_URL"`   // overrides the provider default
	LLMFastModel   string  `env:"LLM_FAST_MODEL"` // drafting model
	LLMMaxTokens   int     `env:"LLM_MAX_TOKENS"`
	LLMTemperature float64 `env:"LLM_TEMPERATURE"`
	// KBAllowPrivateFetch lets trusted local connector metadata discovery fetch
	// private/loopback hosts. Default false (SSRF-safe).
	KBAllowPrivateFetch bool `yaml:"kb_allow_private_fetch" env:"KB_ALLOW_PRIVATE_FETCH"`

	// --- multichannel response-service LLM layer (backend/llm + internal/llmprovider) ---
	// Provider-neutral: LLMDefaultProvider/LLMDefaultModel select WHICH configured
	// provider/model the response engine calls. As of Track 2D/2G these are seed
	// values only: internal/settings.Store (settings.json + the Settings UI) is
	// the live source of truth on every boot after the first, so config.yaml no
	// longer carries this block at all — these fields stay env-only, for a
	// from-scratch first boot or a scripted/CI deployment with no settings.json yet.
	LLMDefaultProvider     string  `env:"LLM_DEFAULT_PROVIDER"`
	LLMDefaultModel        string  `env:"LLM_DEFAULT_MODEL"`
	OpenRouterAPIKey       string  `env:"OPENROUTER_API_KEY"`
	OpenRouterBaseURL      string  `env:"OPENROUTER_BASE_URL"`
	OpenAIAPIKey           string  `env:"OPENAI_API_KEY"`
	OpenAIBaseURL          string  `env:"OPENAI_BASE_URL"`
	GeminiAPIKey           string  `env:"GEMINI_API_KEY"`
	GeminiBaseURL          string  `env:"GEMINI_BASE_URL"`
	LLMDraftMaxTokens      int     `env:"LLM_DRAFT_MAX_TOKENS"`
	LLMDraftTemperature    float64 `env:"LLM_DRAFT_TEMPERATURE"`
	LLMDraftTimeoutSeconds int     `env:"LLM_DRAFT_TIMEOUT_SECONDS"`
	LLMDraftRetry          bool    `env:"LLM_DRAFT_RETRY"`

	// --- Knowledge Base chat assistant (/chat — internal/chat) ---
	// ChatHistoryWindow is how many past turns of a conversation one request
	// sends to the model. The WHOLE conversation is always persisted; this
	// bounds only what the model is shown, which is both a cost control and
	// a relevance one (an hour-old topic dragged into an unrelated question
	// makes answers worse, not better). ChatMaxTokens bounds the answer —
	// deliberately its own knob rather than sharing LLM_DRAFT_MAX_TOKENS,
	// which is sized for a short customer reply, not for an explanation of
	// what changed across a knowledge base.
	ChatHistoryWindow int `env:"CHAT_HISTORY_WINDOW"`
	ChatMaxTokens     int `env:"CHAT_MAX_TOKENS"`

	// --- Langfuse (LLM observability; secrets via env) ---
	// Tracing is best-effort: when disabled or keys are missing the LLM clients
	// emit to OTel's no-op tracer (≈ free). See internal/telemetry.NewLangfuseProvider.
	LangfuseEnabled         bool   `env:"LANGFUSE_ENABLED"`
	LangfuseHost            string `env:"LANGFUSE_HOST"` // e.g. https://cloud.langfuse.com
	LangfusePublicKey       string `env:"LANGFUSE_PUBLIC_KEY"`
	LangfuseSecretKey       string `env:"LANGFUSE_SECRET_KEY"`
	LangfuseFlushIntervalMS int    `env:"LANGFUSE_FLUSH_INTERVAL_MS"` // span batch timeout; 0 → OTel default

	// QueueDriver selects the queue backend ("inmem" is the only one built).
	// Not part of the committed config.yaml schema (nothing else to pick
	// today) but kept configurable for a future driver.
	QueueDriver string `env:"QUEUE_DRIVER"`

	// OrgName seeds the single default organization's display name on an
	// empty database. Renaming an existing org happens in-app (PUT
	// /organization, Track 2J's Team management UI) — this is a first-boot
	// value only, not re-applied afterward.
	OrgName string `env:"ORG_NAME"`

	PageSize int `yaml:"page_size"`
}

func defaults() Config {
	return Config{
		Server: ServerConfig{
			HTTPAddr:        ":8080",
			APIBaseURL:      "http://localhost:8080",
			FrontendBaseURL: "http://localhost:5173",
			CORSOrigins:     []string{"http://localhost:5173"},
			SecureCookies:   false,
		},
		Storage: StorageConfig{
			DBPath:         "./data/xchats.db",
			WADeviceDBPath: "./data/whatsmeow.db",
			BlobDir:        "./blobdata",
		},
		System: SystemConfig{
			LogFormat:                  "logfmt",
			LogLevel:                   "info",
			QueueWorkers:               4,
			SessionTTLHours:            720,
			MinPasswordLen:             8,
			CustomerMessageWaitSeconds: 5,
		},
		MCP: MCPConfig{
			AccessTokenTTLSeconds:   900, // 15 minutes
			RefreshTokenTTLDays:     30,
			AuthCodeTTLSeconds:      300,  // 5 minutes
			UploadTokenTTLSeconds:   900,  // 15 minutes
			MediaTokenTTLSeconds:    3600, // 1 hour
			ReviewHandoffTTLSeconds: 300,  // 5 minutes
		},

		Environment: "development",

		LLMProvider:    "openrouter",
		LLMFastModel:   "openai/gpt-4o-mini",
		LLMMaxTokens:   1024,
		LLMTemperature: 0.3,
		OrgName:        "xchats",
		PageSize:       50,

		LLMDefaultProvider:     "openrouter",
		LLMDefaultModel:        "google/gemini-2.5-flash",
		LLMDraftMaxTokens:      500,
		LLMDraftTemperature:    0.3,
		LLMDraftTimeoutSeconds: 60,
		LLMDraftRetry:          true,

		ChatHistoryWindow: 10,
		ChatMaxTokens:     2000,

		QueueDriver: "inmem",
	}
}

// LLMProviderKey returns the configured API key for the named response-service
// LLM provider ("openrouter" | "openai" | "gemini"), or "" if none is set.
func (c *Config) LLMProviderKey(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return c.OpenAIAPIKey
	case "gemini":
		return c.GeminiAPIKey
	default:
		return c.OpenRouterAPIKey
	}
}

// LLMResolvedBaseURL returns the OpenAI-compatible base URL for the configured
// provider. An explicit LLMBaseURL always wins (self-hosted / in-region model).
func (c *Config) LLMResolvedBaseURL() string {
	if c.LLMBaseURL != "" {
		return strings.TrimRight(c.LLMBaseURL, "/")
	}
	switch strings.ToLower(c.LLMProvider) {
	case "openai":
		return "https://api.openai.com/v1"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	default: // openrouter
		return "https://openrouter.ai/api/v1"
	}
}

// ResolvedAPIBaseURL returns Server.APIBaseURL with no trailing slash, or —
// when left empty, the committed config.yaml's own zero-config default — a
// same-host origin derived from Server.HTTPAddr (":8080" ->
// "http://localhost:8080"). Every discovery/link URL this server emits
// (MCPResourceURL, the OAuth issuer, upload/media link bases) reads through
// this rather than the raw field, so a fresh clone with nothing configured
// still advertises a URL that actually resolves; validateProductionConfig
// (cmd/xchats) deliberately checks the RAW field instead — an empty value is
// exactly the "you forgot to set this for production" case it exists to
// catch, and its derived fallback is a localhost URL anyway, so checking
// either produces the same verdict there.
func (c *Config) ResolvedAPIBaseURL() string {
	if c.Server.APIBaseURL != "" {
		return strings.TrimRight(c.Server.APIBaseURL, "/")
	}
	addr := c.Server.HTTPAddr
	if !strings.HasPrefix(addr, ":") {
		addr = ":8080"
	}
	return "http://localhost" + addr
}

// ResolvedFrontendBaseURL returns Server.FrontendBaseURL with no trailing
// slash, or the zero-config Vite dev-server default when left empty. See
// ResolvedAPIBaseURL's doc comment for the same reasoning.
func (c *Config) ResolvedFrontendBaseURL() string {
	if c.Server.FrontendBaseURL != "" {
		return strings.TrimRight(c.Server.FrontendBaseURL, "/")
	}
	return "http://localhost:5173"
}

// MCPResourceURL is the canonical MCP resource identifier every OAuth access
// token must be audience-bound to (RFC 8707) — the exact `/mcp` endpoint URL,
// derived from ResolvedAPIBaseURL rather than a separate setting so it can
// never drift from where POST /mcp actually listens.
func (c *Config) MCPResourceURL() string {
	return c.ResolvedAPIBaseURL() + "/mcp"
}

// MCPResolvedFrontendBaseURL is an alias for ResolvedFrontendBaseURL, kept
// under its original name for existing call sites (the MCP widget's "Review
// and publish in Xchats" link, the OAuth consent page).
func (c *Config) MCPResolvedFrontendBaseURL() string {
	return c.ResolvedFrontendBaseURL()
}

// IsProduction reports whether Environment is explicitly set to
// "production" (case-insensitive, surrounding whitespace ignored). Anything
// else — including unset — is development.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(c.Environment), "production")
}

// LangfuseTracingEnabled reports whether LLM calls should export traces to
// Langfuse. It requires the explicit toggle plus both keys (a host falls back to
// the public cloud default in the telemetry provider).
func (c *Config) LangfuseTracingEnabled() bool {
	return c.LangfuseEnabled && c.LangfusePublicKey != "" && c.LangfuseSecretKey != ""
}

// ResolveConfigPath implements the config file resolution order: an explicit
// path (the -config flag, when set) wins; else $XCHATS_CONFIG; else
// ./config.yaml if that file exists in the current working directory (the
// repo/dev-checkout default — this repo commits one at its root); else the
// OS-appropriate per-user config directory (internal/appdirs.ConfigDir),
// where an installed (non-repo) binary or an installer places it. A
// config.yaml missing at the finally resolved path is still not an error:
// Load treats that as "use defaults" (config.yaml is optional; the app
// boots on defaults with nothing on disk at all) — this resolution order
// only decides where to look, never whether something has to be there.
func ResolveConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("XCHATS_CONFIG"); v != "" {
		return v
	}
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	if dir, err := appdirs.ConfigDir("xchats"); err == nil {
		return filepath.Join(dir, "config.yaml")
	}
	return "config.yaml"
}

// Load reads config.yaml, then applies process env overrides.
func Load(configPath string) (*Config, error) {
	cfg := defaults()
	if configPath != "" {
		b, err := os.ReadFile(configPath)
		if err == nil {
			if err := yaml.Unmarshal(b, &cfg); err != nil {
				return nil, fmt.Errorf("parse %s: %w", configPath, err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", configPath, err)
		}
	}
	// env overrides config.yaml for ops.
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	return &cfg, nil
}

// TelegramResolvedAPIBaseURL returns the Bot API root to call.
func (c *Config) TelegramResolvedAPIBaseURL() string {
	if c.Telegram.APIBaseURL != "" {
		return strings.TrimRight(c.Telegram.APIBaseURL, "/")
	}
	return "https://api.telegram.org"
}

// TelegramResolvedWebhookSecret returns the secret_token to register with
// Telegram and to verify on the ingress. Empty means the ingress skips the
// check and registration omits secret_token entirely — fine for local dev,
// never for a public deployment.
func (c *Config) TelegramResolvedWebhookSecret() string {
	return c.TelegramWebhookSecret
}

// TelegramResolvedWebhookBaseURL returns the public HTTPS origin to register
// Telegram webhooks against, with no trailing slash.
//
// An explicit telegram.webhook_public_base_url always wins (a deployment
// fronting Telegram with a different hostname than the rest of the app).
// Otherwise it falls back to ResolvedAPIBaseURL when that is HTTPS — which is
// exactly the embedded ngrok tunnel's static domain, since
// cmd/xchats.applyNgrokPublicOrigin installs it there at boot as the single
// source of truth for the app's public origin. The tunnel fronts the whole
// app, so the Telegram ingress is already reachable on that origin; making an
// operator restate the same ngrok hostname in a second setting was pure
// duplication — and duplication that goes stale every time the domain
// changes. An http:// origin is deliberately NOT used as a fallback: Telegram
// refuses to register a non-HTTPS webhook, so returning it would only trade a
// clear "not configured" message for an opaque Bot API 400.
func (c *Config) TelegramResolvedWebhookBaseURL() string {
	if base := strings.TrimSpace(c.Telegram.WebhookPublicBaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	if api := c.ResolvedAPIBaseURL(); strings.HasPrefix(strings.ToLower(api), "https://") {
		return api
	}
	return ""
}

// TelegramResolvedMode resolves the long-polling-vs-webhook switch (Track 1):
// an explicit Telegram.Mode always wins; otherwise a resolvable public webhook
// base URL (see TelegramResolvedWebhookBaseURL — an explicit setting, or the
// ngrok tunnel's HTTPS origin) means webhook delivery is actually available,
// and its absence means polling — the zero-config path for a local/native
// install with no public URL at all.
func (c *Config) TelegramResolvedMode() string {
	switch strings.ToLower(strings.TrimSpace(c.Telegram.Mode)) {
	case "webhook":
		return "webhook"
	case "polling":
		return "polling"
	}
	if c.TelegramResolvedWebhookBaseURL() != "" {
		return "webhook"
	}
	return "polling"
}

// metaDefaultGraphAPIVersion is MetaResolvedGraphAPIVersion's fallback when
// meta.graph_api_version is unset. Meta typically keeps a version callable
// for about two years after a newer one ships, but versions DO eventually
// stop working — an operator hitting a version-deprecation error should set
// meta.graph_api_version rather than wait for a code change here.
const metaDefaultGraphAPIVersion = "v21.0"

// MetaResolvedGraphAPIVersion returns the Graph API version segment
// ("v21.0") every graph.facebook.com/graph.instagram.com call builds its
// path from.
func (c *Config) MetaResolvedGraphAPIVersion() string {
	if v := strings.TrimSpace(c.Meta.GraphAPIVersion); v != "" {
		return v
	}
	return metaDefaultGraphAPIVersion
}

// MetaResolvedPublicBaseURL returns the public HTTPS origin to register
// every Meta webhook and OAuth redirect URI against, with no trailing
// slash. Exactly TelegramResolvedWebhookBaseURL's pattern (see its own doc
// comment for the full reasoning): an explicit meta.webhook_public_base_url
// wins; otherwise ResolvedAPIBaseURL when that is HTTPS (the embedded ngrok
// tunnel's static domain); otherwise empty, since Meta — like Telegram —
// refuses to register a non-HTTPS callback.
func (c *Config) MetaResolvedPublicBaseURL() string {
	if base := strings.TrimSpace(c.Meta.WebhookPublicBaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	if api := c.ResolvedAPIBaseURL(); strings.HasPrefix(strings.ToLower(api), "https://") {
		return api
	}
	return ""
}

// xchatsChannelNS is the fixed namespace for deriving a non-WhatsApp channel
// account's id from its provider owner ref. Like xchatsWaNS it must never
// change: it is what makes re-pasting the same bot token land on the SAME
// account row (history intact) and keep the webhook URL — which embeds this
// uuid — stable.
var xchatsChannelNS = uuid.MustParse("7d6e1c2b-93a4-5f80-b1c7-2a4d6e8f0b13")

// TelegramOwnerRef is a Telegram bot's stable provider identity, the value
// inbox_accounts_v exposes as external_account_ref.
func TelegramOwnerRef(botID int64) string {
	return "telegram:bot:" + strconv.FormatInt(botID, 10)
}

// InstagramOwnerRef, MessengerOwnerRef and WhatsAppCloudOwnerRef are the
// Meta channels' stable provider identities — TelegramOwnerRef's siblings,
// each feeding the same ChannelAccountID derivation below. Re-connecting the
// same IG account / Page / phone number always lands on the same
// channel_accounts row (history intact), exactly like a re-pasted bot token.
func InstagramOwnerRef(igUserID string) string {
	return "instagram:user:" + igUserID
}

func MessengerOwnerRef(pageID string) string {
	return "messenger:page:" + pageID
}

func WhatsAppCloudOwnerRef(phoneNumberID string) string {
	return "whatsapp_cloud:phone:" + phoneNumberID
}

// ChannelAccountID derives a deterministic account id from a channel owner ref.
// Unlike AccountID it applies no JID coercion — a Telegram owner ref is not a
// phone number and must never grow an @s.whatsapp.net suffix.
func ChannelAccountID(ownerRef string) uuid.UUID {
	return uuid.NewSHA1(xchatsChannelNS, []byte(strings.ToLower(strings.TrimSpace(ownerRef))))
}

// CanonicalJID lowercases/trims a JID and coerces a bare phone to phone-JID form.
func CanonicalJID(jid string) string {
	j := strings.ToLower(strings.TrimSpace(jid))
	if j == "" {
		return ""
	}
	if !strings.Contains(j, "@") {
		j = j + "@s.whatsapp.net"
	}
	return j
}

// AccountID derives the deterministic wa_accounts.id from an owner JID.
func AccountID(ownerJID string) uuid.UUID {
	return uuid.NewSHA1(xchatsWaNS, []byte(CanonicalJID(ownerJID)))
}

// PhoneFromJID returns the numeric phone part of a phone JID ("7700@s.whatsapp.net" → "7700").
func PhoneFromJID(jid string) string {
	at := strings.Index(jid, "@")
	if at < 0 {
		return jid
	}
	return jid[:at]
}
