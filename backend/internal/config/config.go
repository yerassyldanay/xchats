// Package config loads the two-file config model: secrets from .env (environment)
// and non-secret tunables + seed from config.yaml. It also owns the deterministic
// wa_accounts.id derivation (uuidv5(XCHATS_WA_NS, owner_jid)).
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// XchatsWaNS is the fixed namespace for deriving wa_accounts.id from owner_jid.
// It must never change — the account id is uuidv5(XchatsWaNS, canonical(owner_jid)).
var XchatsWaNS = uuid.MustParse("a3f1d2c4-5e6b-47a8-9c0d-1e2f3a4b5c6d")

// Config is the merged application configuration.
type Config struct {
	// --- server / logging (config.yaml, overridable by env) ---
	HTTPAddr    string   `yaml:"http_addr" env:"HTTP_ADDR"`
	CORSOrigins []string `yaml:"cors_origins" env:"CORS_ORIGINS" envSeparator:","`
	LogFormat   string   `yaml:"log_format" env:"LOG_FORMAT"`
	LogLevel    string   `yaml:"log_level" env:"LOG_LEVEL"`
	APIBaseURL  string   `yaml:"api_base_url" env:"API_BASE_URL"`

	// --- secrets (.env only) ---
	DatabaseURL          string `env:"DATABASE_URL"`
	EvolutionBaseURL     string `env:"EVOLUTION_BASE_URL"`
	EvolutionAPIKey      string `env:"EVOLUTION_API_KEY"`
	EvolutionInstance    string `yaml:"evolution_instance" env:"EVOLUTION_INSTANCE"`
	WebhookToken         string `env:"WEBHOOK_TOKEN"`
	WebhookPublicBaseURL string `env:"WEBHOOK_PUBLIC_BASE_URL"`
	SessionSecret        string `env:"SESSION_SECRET"`

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
	LLMProvider    string  `yaml:"llm_provider" env:"LLM_PROVIDER"`         // openrouter|openai|gemini
	LLMAPIKey      string  `env:"LLM_API_KEY"`                              // secret
	LLMBaseURL     string  `yaml:"llm_base_url" env:"LLM_BASE_URL"`         // overrides the provider default
	LLMFastModel   string  `yaml:"llm_fast_model" env:"LLM_FAST_MODEL"`     // drafting model
	LLMVisionModel string  `yaml:"llm_vision_model" env:"LLM_VISION_MODEL"` // multimodal model for KB media extraction (empty → describe popups)
	LLMMaxTokens   int     `yaml:"llm_max_tokens" env:"LLM_MAX_TOKENS"`
	LLMTemperature float64 `yaml:"llm_temperature" env:"LLM_TEMPERATURE"`
	// KBAllowPrivateFetch lets the playground URL adapter fetch private/loopback
	// hosts. Default false (SSRF-safe); enable only for trusted self-hosted setups.
	KBAllowPrivateFetch bool `yaml:"kb_allow_private_fetch" env:"KB_ALLOW_PRIVATE_FETCH"`

	// --- seed + account identity (config.yaml, secrets via env) ---
	OrgName              string `yaml:"org_name" env:"ORG_NAME"`
	WaAccountDisplayName string `yaml:"wa_account_display_name"`
	WaOwnerJID           string `yaml:"wa_owner_jid" env:"WA_OWNER_JID"`
	SeedAdminEmail       string `yaml:"seed_admin_email" env:"SEED_ADMIN_EMAIL"`
	SeedAdminPassword    string `yaml:"seed_admin_password" env:"SEED_ADMIN_PASSWORD"`

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
		OrgName:         "XChats",
		PageSize:        50,
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
	return uuid.NewSHA1(XchatsWaNS, []byte(CanonicalJID(ownerJID)))
}

// PhoneFromJID returns the numeric phone part of a phone JID ("7700@s.whatsapp.net" → "7700").
func PhoneFromJID(jid string) string {
	at := strings.Index(jid, "@")
	if at < 0 {
		return jid
	}
	return jid[:at]
}
