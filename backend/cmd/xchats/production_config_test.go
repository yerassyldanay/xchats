package main

// production_config_test.go covers Task 17's release gate: validateProductionConfig
// / isLocalBaseURL, the checks runServe runs (and fails startup on) whenever
// cfg.Environment is "production".

import (
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
)

func TestValidateProductionConfig_RejectsLocalURLs(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIBaseURL:      "http://localhost:8080",
			FrontendBaseURL: "http://localhost:5173",
		},
	}
	problems := validateProductionConfig(cfg)
	if len(problems) != 2 {
		t.Fatalf("expected 2 problems (api base, frontend base), got %d: %v", len(problems), problems)
	}
}

func TestValidateProductionConfig_AcceptsPublicURLs(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIBaseURL:      "https://api.xchats.example",
			FrontendBaseURL: "https://app.xchats.example",
		},
		// MCPJWTSigningKey deliberately left empty: an ephemeral/missing MCP
		// signing key is no longer a production-blocking problem — see
		// validateProductionConfig's own doc comment (provisionSystemSecrets
		// durably provisions it before this function runs, whenever a secure
		// credential store is available).
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
