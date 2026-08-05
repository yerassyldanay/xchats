// Package config loads the two-file config model: secrets from .env (environment)
// and non-secret tunables + seed from config.yaml. It also owns the deterministic
// wa_accounts.id derivation (uuidv5(XCHATS_WA_NS, owner_jid)).
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// xchatsWaNS is the fixed namespace for deriving wa_accounts.id from owner_jid.
// It must never change — the account id is uuidv5(xchatsWaNS, canonical(owner_jid)).
var xchatsWaNS = uuid.MustParse("a3f1d2c4-5e6b-47a8-9c0d-1e2f3a4b5c6d")

// Config is the merged application configuration.
type Config struct {
	// --- server / logging (config.yaml, overridable by env) ---
	HTTPAddr    string   `yaml:"http_addr" env:"HTTP_ADDR"`
	CORSOrigins []string `yaml:"cors_origins" env:"CORS_ORIGINS" envSeparator:","`
	LogFormat   string   `yaml:"log_format" env:"LOG_FORMAT"`
	LogLevel    string   `yaml:"log_level" env:"LOG_LEVEL"`
	APIBaseURL  string   `yaml:"api_base_url" env:"API_BASE_URL"`
	// Environment is "development" (the default) or "production" — the
	// switch that turns dev-only conveniences (an ephemeral MCP signing key,
	// a localhost base URL) into hard startup failures instead of warnings
	// (plan Task 17). Anything other than the literal "production" is
	// treated as development, so an unset/misspelled value never accidentally
	// disables the checks it's supposed to enable.
	Environment string `yaml:"environment" env:"ENVIRONMENT"`
	// TrustedProxies lists the reverse-proxy IPs/CIDRs gin should trust
	// X-Forwarded-* headers from (gin's SetTrustedProxies). Empty means trust
	// NONE — c.ClientIP() falls back to the raw connection address rather
	// than any client-suppliable header. This must never default to "trust
	// everyone" (gin's own zero-value behavior): Host and X-Forwarded-* are
	// attacker-controlled on any request that doesn't actually come through
	// a configured proxy (plan Task 17).
	TrustedProxies []string `yaml:"trusted_proxies" env:"TRUSTED_PROXIES" envSeparator:","`
	// FrontendBaseURL is the deployed Vue SPA's own public origin (a separate
	// service from the backend — see deploy/docker-compose.yaml). The MCP KB
	// Manager widget's "Review and publish in Xchats" link and the OAuth
	// consent page both need it to build a real URL into /playground.
	FrontendBaseURL string `yaml:"frontend_base_url" env:"FRONTEND_BASE_URL"`

	// --- MCP connector (plan/mcp.md) ---
	// MCPJWTSigningKey seeds the Ed25519 keypair that signs MCP OAuth access
	// tokens (internal/mcpauth), parsed the same three ways as
	// TG_CREDENTIALS_ENC_KEY (64 hex chars, base64, or a 32-char literal). Unset
	// in dev falls back to an ephemeral in-memory key (logged loudly): every
	// previously issued token stops verifying on restart, and every replica in
	// a multi-instance deployment would mint tokens no other replica can check
	// — set this for anything beyond a single local process.
	MCPJWTSigningKey string `env:"MCP_JWT_SIGNING_KEY"`
	// MCPAccessTokenTTLSeconds / MCPRefreshTokenTTLDays: short-lived access
	// tokens, long-lived rotated-on-use refresh tokens (plan/mcp.md §3).
	MCPAccessTokenTTLSeconds int `yaml:"mcp_access_token_ttl_seconds" env:"MCP_ACCESS_TOKEN_TTL_SECONDS"`
	MCPRefreshTokenTTLDays   int `yaml:"mcp_refresh_token_ttl_days" env:"MCP_REFRESH_TOKEN_TTL_DAYS"`
	// MCPAuthCodeTTLSeconds bounds the PKCE authorization code's lifetime
	// between /oauth/authorize and the POST /oauth/token exchange.
	MCPAuthCodeTTLSeconds int `yaml:"mcp_auth_code_ttl_seconds" env:"MCP_AUTH_CODE_TTL_SECONDS"`
	// MCPUploadTokenTTLSeconds bounds a kb_media_upload signed upload URL.
	MCPUploadTokenTTLSeconds int `yaml:"mcp_upload_token_ttl_seconds" env:"MCP_UPLOAD_TOKEN_TTL_SECONDS"`
	// MCPMediaTokenTTLSeconds bounds a signed media-READ URL (the widget's
	// preview thumbnails and open/download links). Longer than the upload TTL
	// on purpose: expiry here means a broken <img> in a record the user left
	// open, the grant is read-only and single-object, and every kb_read mints
	// fresh URLs anyway — so the practical window is one navigation, not an
	// hour.
	MCPMediaTokenTTLSeconds int `yaml:"mcp_media_token_ttl_seconds" env:"MCP_MEDIA_TOKEN_TTL_SECONDS"`
	// MCPReviewHandoffTTLSeconds bounds the one-time signed URL a tool result
	// hands the KB Manager widget for "Review and publish in Xchats" (plan
	// Task 15) — short-lived: it only needs to survive the click.
	MCPReviewHandoffTTLSeconds int `yaml:"mcp_review_handoff_ttl_seconds" env:"MCP_REVIEW_HANDOFF_TTL_SECONDS"`

	// --- secrets (.env only) ---
	DBPath string `env:"DB_PATH"`
	// WADeviceDBPath is whatsmeow's own device-session SQLite file — kept
	// separate from DBPath since whatsmeow's sqlstore manages that schema
	// entirely on its own (see internal/whatsmeow/store.go).
	WADeviceDBPath string `yaml:"wa_device_db_path" env:"WA_DEVICE_DB_PATH"`
	SessionSecret  string `env:"SESSION_SECRET"`

	// --- Telegram (Bot API) ---
	// TelegramWebhookPublicBaseURL must be a public HTTPS origin — Telegram
	// refuses to register a webhook that is anything else. Validated at
	// provisioning time.
	TelegramWebhookPublicBaseURL string `env:"TG_WEBHOOK_PUBLIC_BASE_URL"`
	// TelegramAPIBaseURL overrides https://api.telegram.org (a local Bot API
	// server, or a test double).
	TelegramAPIBaseURL string `yaml:"tg_api_base_url" env:"TG_API_BASE_URL"`
	// TelegramCredentialsEncKey is the AES-256-GCM key protecting stored bot
	// tokens (internal/secretbox, xchats.tg_credentials). Losing it means
	// re-pasting every bot token; it is never logged.
	TelegramCredentialsEncKey string `env:"TG_CREDENTIALS_ENC_KEY"`
	// TelegramWebhookSecret is the secret_token registered with setWebhook and
	// verified on the Telegram ingress. Empty means the ingress skips the
	// check — fine for local dev, never for a public deployment. Use
	// TelegramResolvedWebhookSecret to read the effective value.
	TelegramWebhookSecret string `env:"TG_WEBHOOK_SECRET"`

	// --- auth / session (config.yaml) ---
	SessionTTLHours int  `yaml:"session_ttl_hours" env:"SESSION_TTL_HOURS"`
	SecureCookies   bool `yaml:"secure_cookies" env:"SECURE_COOKIES"`
	MinPasswordLen  int  `yaml:"min_password_len"`

	// --- blob / queue (config.yaml) ---
	BlobDir      string `yaml:"blob_dir" env:"BLOB_DIR"`
	QueueDriver  string `yaml:"queue_driver" env:"QUEUE_DRIVER"`
	QueueWorkers int    `yaml:"queue_workers" env:"QUEUE_WORKERS"`

	// --- LLM / AI brain (key is a secret via .env; the rest are tunables) ---
	// When LLMAPIKey is empty the app falls back to the hardcoded Stub drafter.
	LLMProvider    string  `yaml:"llm_provider" env:"LLM_PROVIDER"`     // openrouter|openai|gemini
	LLMAPIKey      string  `env:"LLM_API_KEY"`                          // secret
	LLMBaseURL     string  `yaml:"llm_base_url" env:"LLM_BASE_URL"`     // overrides the provider default
	LLMFastModel   string  `yaml:"llm_fast_model" env:"LLM_FAST_MODEL"` // drafting model
	LLMMaxTokens   int     `yaml:"llm_max_tokens" env:"LLM_MAX_TOKENS"`
	LLMTemperature float64 `yaml:"llm_temperature" env:"LLM_TEMPERATURE"`
	// KBAllowPrivateFetch lets trusted local connector metadata discovery fetch
	// private/loopback hosts. Default false (SSRF-safe).
	KBAllowPrivateFetch bool `yaml:"kb_allow_private_fetch" env:"KB_ALLOW_PRIVATE_FETCH"`

	// --- multichannel response-service LLM layer (backend/llm + internal/llmprovider) ---
	// Provider-neutral: LLMDefaultProvider/LLMDefaultModel select WHICH configured
	// provider/model the response engine calls; switching either is configuration
	// only. Distinct from the legacy LLMProvider/LLMAPIKey/... fields above, which
	// the dormant internal/brain path still reads — the two must never be conflated.
	LLMDefaultProvider     string  `yaml:"llm_default_provider" env:"LLM_DEFAULT_PROVIDER"`
	LLMDefaultModel        string  `yaml:"llm_default_model" env:"LLM_DEFAULT_MODEL"`
	OpenRouterAPIKey       string  `env:"OPENROUTER_API_KEY"`
	OpenRouterBaseURL      string  `yaml:"openrouter_base_url" env:"OPENROUTER_BASE_URL"`
	OpenAIAPIKey           string  `env:"OPENAI_API_KEY"`
	OpenAIBaseURL          string  `yaml:"openai_base_url" env:"OPENAI_BASE_URL"`
	GeminiAPIKey           string  `env:"GEMINI_API_KEY"`
	GeminiBaseURL          string  `yaml:"gemini_base_url" env:"GEMINI_BASE_URL"`
	LLMDraftMaxTokens      int     `yaml:"llm_draft_max_tokens" env:"LLM_DRAFT_MAX_TOKENS"`
	LLMDraftTemperature    float64 `yaml:"llm_draft_temperature" env:"LLM_DRAFT_TEMPERATURE"`
	LLMDraftTimeoutSeconds int     `yaml:"llm_draft_timeout_seconds" env:"LLM_DRAFT_TIMEOUT_SECONDS"`
	LLMDraftRetry          bool    `yaml:"llm_draft_retry" env:"LLM_DRAFT_RETRY"`

	// SimulatorEnabled gates the /simulator/messages API (Phase 10): the route is
	// not registered at all when false. Defaults false — only a dev/staging
	// deployment should set SIMULATOR_ENABLED=true.
	SimulatorEnabled bool `yaml:"simulator_enabled" env:"SIMULATOR_ENABLED"`

	// --- Langfuse (LLM observability; secrets via .env) ---
	// Tracing is best-effort: when disabled or keys are missing the LLM clients
	// emit to OTel's no-op tracer (≈ free). See internal/telemetry.NewLangfuseProvider.
	LangfuseEnabled         bool   `env:"LANGFUSE_ENABLED"`
	LangfuseHost            string `env:"LANGFUSE_HOST"` // e.g. https://cloud.langfuse.com
	LangfusePublicKey       string `env:"LANGFUSE_PUBLIC_KEY"`
	LangfuseSecretKey       string `env:"LANGFUSE_SECRET_KEY"`
	LangfuseFlushIntervalMS int    `env:"LANGFUSE_FLUSH_INTERVAL_MS"` // span batch timeout; 0 → OTel default

	// --- seed (config.yaml, overridable by env) ---
	// SeedAdminEmail and SeedAdminPassword are intentionally removed: the
	// initial admin user (admin@xchat.kz) is created by migration
	// 0006_init_admin.up.sql with a precomputed argon2id hash. No boot-time
	// seeding is performed. WhatsApp accounts are no longer pre-configured
	// either — they are paired dynamically via the UI (see WADeviceDBPath).
	OrgName string `yaml:"org_name" env:"ORG_NAME"`

	PageSize int `yaml:"page_size"`
}

func defaults() Config {
	return Config{
		HTTPAddr:        ":8080",
		CORSOrigins:     []string{"http://localhost:5173"},
		LogFormat:       "logfmt",
		LogLevel:        "info",
		APIBaseURL:      "http://localhost:8080",
		SessionTTLHours: 720,
		SecureCookies:   false,
		MinPasswordLen:  8,
		BlobDir:         "./blobdata",
		QueueDriver:     "inmem",
		QueueWorkers:    4,
		LLMProvider:     "openrouter",
		LLMFastModel:    "openai/gpt-4o-mini",
		LLMMaxTokens:    1024,
		LLMTemperature:  0.3,
		OrgName:         "xchats",
		PageSize:        50,
		FrontendBaseURL: "http://localhost:5173",
		Environment:     "development",
		WADeviceDBPath:  "/data/whatsmeow.db",

		MCPAccessTokenTTLSeconds:   900, // 15 minutes
		MCPRefreshTokenTTLDays:     30,
		MCPAuthCodeTTLSeconds:      300,  // 5 minutes
		MCPUploadTokenTTLSeconds:   900,  // 15 minutes
		MCPMediaTokenTTLSeconds:    3600, // 1 hour
		MCPReviewHandoffTTLSeconds: 300,  // 5 minutes

		LLMDefaultProvider:     "openrouter",
		LLMDefaultModel:        "google/gemini-2.5-flash",
		LLMDraftMaxTokens:      500,
		LLMDraftTemperature:    0.3,
		LLMDraftTimeoutSeconds: 60,
		LLMDraftRetry:          true,
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

// MCPResourceURL is the canonical MCP resource identifier every OAuth access
// token must be audience-bound to (RFC 8707) — the exact `/mcp` endpoint URL,
// derived from APIBaseURL rather than a separate setting so it can never
// drift from where POST /mcp actually listens.
func (c *Config) MCPResourceURL() string {
	return strings.TrimRight(c.APIBaseURL, "/") + "/mcp"
}

// MCPResolvedFrontendBaseURL returns the frontend origin with no trailing
// slash, for building links like {base}/playground.
func (c *Config) MCPResolvedFrontendBaseURL() string {
	return strings.TrimRight(c.FrontendBaseURL, "/")
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

// Load reads optional .env then config.yaml, then applies env overrides.
func Load(configPath, envPath string) (*Config, error) {
	if envPath != "" {
		if err := loadDotenv(envPath); err != nil {
			return nil, fmt.Errorf("load .env: %w", err)
		}
	}
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

// loadDotenv loads KEY=VALUE pairs from a file, without clobbering already-set env.
func loadDotenv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
	return sc.Err()
}

// TelegramResolvedAPIBaseURL returns the Bot API root to call.
func (c *Config) TelegramResolvedAPIBaseURL() string {
	if c.TelegramAPIBaseURL != "" {
		return strings.TrimRight(c.TelegramAPIBaseURL, "/")
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
