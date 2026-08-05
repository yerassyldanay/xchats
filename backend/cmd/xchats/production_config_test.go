package main

// production_config_test.go covers Task 17's release gate: validateProductionConfig
// / isLocalBaseURL, the checks runServe runs (and fails startup on) whenever
// cfg.Environment is "production".

import (
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
)

func TestValidateProductionConfig_RejectsEphemeralKeyAndLocalURLs(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIBaseURL:      "http://localhost:8080",
			FrontendBaseURL: "http://localhost:5173",
		},
		// MCPJWTSigningKey left empty — the ephemeral-fallback case.
	}
	problems := validateProductionConfig(cfg)
	if len(problems) != 3 {
		t.Fatalf("expected 3 problems (key, api base, frontend base), got %d: %v", len(problems), problems)
	}
}

func TestValidateProductionConfig_AcceptsPersistentKeyAndPublicURLs(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIBaseURL:      "https://api.xchats.example",
			FrontendBaseURL: "https://app.xchats.example",
		},
		MCPJWTSigningKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	if problems := validateProductionConfig(cfg); len(problems) != 0 {
		t.Fatalf("expected no problems for a fully configured production setup, got: %v", problems)
	}
}

func TestValidateProductionConfig_PartialMisconfiguration(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIBaseURL:      "https://api.xchats.example",
			FrontendBaseURL: "http://localhost:5173", // forgot to update this one
		},
		MCPJWTSigningKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	problems := validateProductionConfig(cfg)
	if len(problems) != 1 {
		t.Fatalf("expected exactly 1 problem (frontend base URL), got %d: %v", len(problems), problems)
	}
}

func TestIsLocalBaseURL(t *testing.T) {
	cases := map[string]bool{
		"":                           true,
		"not a url at all \x7f":      true,
		"http://localhost:5173":      true,
		"http://127.0.0.1:8080":      true,
		"http://[::1]:8080":          true,
		"https://api.xchats.example": false,
		"https://xchats.example":     false,
	}
	for raw, want := range cases {
		if got := isLocalBaseURL(raw); got != want {
			t.Errorf("isLocalBaseURL(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestConfig_IsProduction(t *testing.T) {
	cases := map[string]bool{
		"":             false,
		"development":  false,
		"dev":          false,
		"production":   true,
		"Production":   true,
		" production ": true,
	}
	for env, want := range cases {
		cfg := &config.Config{Environment: env}
		if got := cfg.IsProduction(); got != want {
			t.Errorf("Config{Environment: %q}.IsProduction() = %v, want %v", env, got, want)
		}
	}
}
