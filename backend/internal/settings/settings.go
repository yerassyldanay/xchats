// Package settings is xchats' user-editable runtime configuration — the
// Settings UI's own durable state. Distinct from both config.yaml (static
// boot/infra tunables an operator commits or mounts, internal/config) and
// internal/credentials (secret values, never written here): everything in
// this package is safe to hold in a plain JSON file — provider base
// URLs/default models, ngrok's tunnel region/domain, and a couple of
// onboarding flags.
package settings

import "time"

// CurrentVersion is stamped on every Settings this package writes — not
// used for any migration today (this is the only version that has ever
// existed), but reserved so a future schema change has something to key a
// migration off of.
const CurrentVersion = 1

// LLMSettings is the response engine's live, editable model configuration —
// the Settings-UI-owned counterpart to config.go's now-legacy
// LLM_DEFAULT_PROVIDER/LLM_DEFAULT_MODEL/LLM_DRAFT_* env vars (see
// defaultsFromEnv, which seeds this section from them on first boot).
type LLMSettings struct {
	DefaultProvider string `json:"default_provider"`
	DefaultModel    string `json:"default_model"`
	// VisionModel is the model used for drafts that need to reason about an
	// attached image — "" means none configured (image attachments are
	// described to the model as text only).
	VisionModel    string  `json:"vision_model"`
	MaxTokens      int     `json:"max_tokens"`
	Temperature    float64 `json:"temperature"`
	TimeoutSeconds int     `json:"timeout_seconds"`
	Retry          bool    `json:"retry"`
}

// ProviderSettings is one integration's non-secret configuration — its
// credential (if any) lives in internal/credentials under
// "<id>.api_key"/etc, keyed by the same provider id this struct is stored
// under in Settings.Providers.
type ProviderSettings struct {
	// BaseURL overrides the provider's public default endpoint — a
	// self-hosted or proxied gateway. "" means use the built-in default.
	BaseURL string `json:"base_url,omitempty"`
	// DefaultModel is this provider's chosen default when it is NOT the
	// response engine's overall default provider (LLMSettings.DefaultProvider
	// picks which provider's DefaultModel actually drives responses).
	DefaultModel string `json:"default_model,omitempty"`
	// Disabled hides this provider from being selected as the active
	// default without discarding its saved credential/configuration.
	Disabled bool `json:"disabled"`
	// LastVerifiedAt/LastError record the most recent credential validation
	// (internal/credentials.Provider.Validate) result — nil/"" means never
	// checked, not "known good."
	LastVerifiedAt *time.Time `json:"last_verified_at,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
}

// NgrokSettings is the embedded tunnel's non-secret configuration. The
// authtoken and account API key live in internal/credentials; Domain may be
// entered manually or discovered from the account API during startup.
type NgrokSettings struct {
	Region string `json:"region,omitempty"`
	Domain string `json:"domain,omitempty"`
}

// Settings is the Settings UI's entire durable state, round-tripped to
// <configDir>/settings.json by Store.
type Settings struct {
	Version int         `json:"version"`
	LLM     LLMSettings `json:"llm"`
	// Providers is keyed by provider id ("openrouter", "ngrok", ...) — see
	// internal/credentials.Provider.ID. A missing entry means "every
	// default applies, never configured," not an error.
	Providers map[string]ProviderSettings `json:"providers"`
	Ngrok     NgrokSettings               `json:"ngrok"`
	// CredentialFileFallbackAccepted records that an operator explicitly
	// acknowledged the Settings UI's warning about FileStore being a
	// weaker guarantee than an OS keychain (internal/credentials.Open) —
	// shown once, not on every save.
	CredentialFileFallbackAccepted bool `json:"credential_file_fallback_accepted"`
}

// defaults returns a fresh Settings with its LLM section seeded from
// environment/config (see defaultsFromEnv) and every other section at its
// zero value — the shape Store.Load produces (and persists) the very first
// time it runs with no settings.json on disk yet.
func defaults() Settings {
	return Settings{
		Version:   CurrentVersion,
		LLM:       defaultsFromEnv(),
		Providers: map[string]ProviderSettings{},
	}
}
