package config

import "testing"

// TestDefaults pins the non-secret tunable defaults, including the LLM_VISION_MODEL
// contract: empty by default (no multimodal model → the playground raises describe
// popups instead of auto-captioning media).
func TestDefaults(t *testing.T) {
	d := defaults()
	if d.LLMProvider != "openrouter" {
		t.Errorf("default LLMProvider = %q, want openrouter", d.LLMProvider)
	}
	if d.LLMFastModel != "openai/gpt-4o-mini" {
		t.Errorf("default LLMFastModel = %q", d.LLMFastModel)
	}
	if d.LLMVisionModel != "" {
		t.Errorf("default LLMVisionModel = %q, want empty (describe-popup fallback)", d.LLMVisionModel)
	}
	if d.LLMMaxTokens != 1024 {
		t.Errorf("default LLMMaxTokens = %d, want 1024", d.LLMMaxTokens)
	}
}

// TestLoadEnvOverrides proves the recently-added env vars are wired through Load:
// LLM_VISION_MODEL (KB media extraction) and the LANGFUSE_* observability toggle.
// No config.yaml / .env files are read (both paths empty), so this is hermetic.
func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("LLM_VISION_MODEL", "openai/gpt-4o")
	t.Setenv("LLM_PROVIDER", "openai")
	t.Setenv("LANGFUSE_ENABLED", "true")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "pk-test")
	t.Setenv("LANGFUSE_SECRET_KEY", "sk-test")

	cfg, err := Load("", "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LLMVisionModel != "openai/gpt-4o" {
		t.Errorf("LLM_VISION_MODEL not bound: %q", cfg.LLMVisionModel)
	}
	if cfg.LLMProvider != "openai" {
		t.Errorf("LLM_PROVIDER override not applied: %q", cfg.LLMProvider)
	}
	if !cfg.LangfuseEnabled {
		t.Errorf("LANGFUSE_ENABLED not bound")
	}
	// The override must flow through the provider → base-URL resolution.
	if got := cfg.LLMResolvedBaseURL(); got != "https://api.openai.com/v1" {
		t.Errorf("LLMResolvedBaseURL after override = %q", got)
	}
}

// TestLLMResolvedBaseURL covers provider → OpenAI-compatible base URL mapping,
// case-insensitivity, the empty-provider fallback, and the explicit-override path
// (which wins and gets its trailing slash trimmed).
func TestLLMResolvedBaseURL(t *testing.T) {
	cases := []struct {
		name, provider, baseURL, want string
	}{
		{"openrouter default", "openrouter", "", "https://openrouter.ai/api/v1"},
		{"empty provider falls back to openrouter", "", "", "https://openrouter.ai/api/v1"},
		{"openai", "openai", "", "https://api.openai.com/v1"},
		{"gemini", "gemini", "", "https://generativelanguage.googleapis.com/v1beta/openai"},
		{"case-insensitive provider", "OpenAI", "", "https://api.openai.com/v1"},
		{"explicit base url wins, trailing slash trimmed", "openai", "http://localhost:1234/v1/", "http://localhost:1234/v1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{LLMProvider: tc.provider, LLMBaseURL: tc.baseURL}
			if got := c.LLMResolvedBaseURL(); got != tc.want {
				t.Errorf("LLMResolvedBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLangfuseTracingEnabled covers the observability gate: it requires the explicit
// toggle AND both keys (a half-configured client must stay on the no-op tracer).
func TestLangfuseTracingEnabled(t *testing.T) {
	cases := []struct {
		name        string
		enabled     bool
		pub, secret string
		want        bool
	}{
		{"off by default", false, "", "", false},
		{"enabled but no keys", true, "", "", false},
		{"enabled with only public key", true, "pk", "", false},
		{"enabled with only secret key", true, "", "sk", false},
		{"enabled with both keys", true, "pk", "sk", true},
		{"keys present but toggle off", false, "pk", "sk", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{LangfuseEnabled: tc.enabled, LangfusePublicKey: tc.pub, LangfuseSecretKey: tc.secret}
			if got := c.LangfuseTracingEnabled(); got != tc.want {
				t.Errorf("LangfuseTracingEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAccountIDDeterministic(t *testing.T) {
	a := AccountID("77011111111@s.whatsapp.net")
	b := AccountID(" 77011111111@S.WhatsApp.Net ") // different case/spacing, same number
	if a != b {
		t.Fatalf("account id not stable across canonicalization: %s vs %s", a, b)
	}
	c := AccountID("77000000000@s.whatsapp.net") // a different number (the customer)
	if a == c {
		t.Fatalf("different numbers collided on the same account id")
	}
}

func TestCanonicalAndPhone(t *testing.T) {
	if got := CanonicalJID("77011111111"); got != "77011111111@s.whatsapp.net" {
		t.Errorf("bare phone not coerced: %q", got)
	}
	if got := PhoneFromJID("77000000000@s.whatsapp.net"); got != "77000000000" {
		t.Errorf("phone extraction: %q", got)
	}
}
