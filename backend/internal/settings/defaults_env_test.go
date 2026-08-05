package settings

import "testing"

func TestDefaultsFromEnvFallsBackWhenUnset(t *testing.T) {
	for _, name := range []string{
		"LLM_DEFAULT_PROVIDER", "LLM_DEFAULT_MODEL",
		"LLM_DRAFT_MAX_TOKENS", "LLM_DRAFT_TEMPERATURE",
		"LLM_DRAFT_TIMEOUT_SECONDS", "LLM_DRAFT_RETRY",
	} {
		t.Setenv(name, "")
	}
	got := defaultsFromEnv()
	want := LLMSettings{
		DefaultProvider: "openrouter",
		DefaultModel:    "google/gemini-2.5-flash",
		MaxTokens:       500,
		Temperature:     0.3,
		TimeoutSeconds:  60,
		Retry:           true,
	}
	if got != want {
		t.Errorf("defaultsFromEnv() = %+v, want %+v", got, want)
	}
}

func TestDefaultsFromEnvAdoptsSetValues(t *testing.T) {
	t.Setenv("LLM_DEFAULT_PROVIDER", "openai")
	t.Setenv("LLM_DEFAULT_MODEL", "gpt-4o")
	t.Setenv("LLM_DRAFT_MAX_TOKENS", "750")
	t.Setenv("LLM_DRAFT_TEMPERATURE", "0.7")
	t.Setenv("LLM_DRAFT_TIMEOUT_SECONDS", "30")
	t.Setenv("LLM_DRAFT_RETRY", "false")

	got := defaultsFromEnv()
	want := LLMSettings{
		DefaultProvider: "openai",
		DefaultModel:    "gpt-4o",
		MaxTokens:       750,
		Temperature:     0.7,
		TimeoutSeconds:  30,
		Retry:           false,
	}
	if got != want {
		t.Errorf("defaultsFromEnv() = %+v, want %+v", got, want)
	}
}

func TestEnvHelpersIgnoreUnparseableValues(t *testing.T) {
	t.Setenv("X_INT", "not-a-number")
	if got := envIntOr("X_INT", 42); got != 42 {
		t.Errorf("envIntOr with garbage input = %d, want fallback %d", got, 42)
	}
	t.Setenv("X_FLOAT", "not-a-float")
	if got := envFloatOr("X_FLOAT", 1.5); got != 1.5 {
		t.Errorf("envFloatOr with garbage input = %v, want fallback %v", got, 1.5)
	}
	t.Setenv("X_BOOL", "not-a-bool")
	if got := envBoolOr("X_BOOL", true); got != true {
		t.Errorf("envBoolOr with garbage input = %v, want fallback %v", got, true)
	}
}

func TestDefaultsSeedsFromEnvAndInitializesProviders(t *testing.T) {
	t.Setenv("LLM_DEFAULT_PROVIDER", "")
	d := defaults()
	if d.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", d.Version, CurrentVersion)
	}
	if d.LLM.DefaultProvider != "openrouter" {
		t.Errorf("LLM.DefaultProvider = %q, want %q", d.LLM.DefaultProvider, "openrouter")
	}
	if d.Providers == nil {
		t.Error("Providers is nil, want a non-nil empty map")
	}
	if len(d.Providers) != 0 {
		t.Errorf("Providers = %v, want empty", d.Providers)
	}
}
