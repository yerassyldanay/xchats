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
	problems := validateProductionConfig(cfg, true)
	if len(problems) != 2 {
		t.Fatalf("expected 2 problems (api base, frontend base), got %d: %v", len(problems), problems)
	}
}

func TestValidateProductionConfig_AcceptsPublicURLs(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIBaseURL:      "https://api.xchats.example",
			FrontendBaseURL: "https://app.xchats.example",
			CORSOrigins:     []string{"https://app.xchats.example"},
		},
		// MCPJWTSigningKey deliberately left empty: an ephemeral/missing MCP
		// signing key is no longer a production-blocking problem — see
		// validateProductionConfig's own doc comment (provisionSystemSecrets
		// durably provisions it before this function runs, whenever a secure
		// credential store is available).
	}
	if problems := validateProductionConfig(cfg, true); len(problems) != 0 {
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
	problems := validateProductionConfig(cfg, true)
	if len(problems) != 1 {
		t.Fatalf("expected exactly 1 problem (frontend base URL), got %d: %v", len(problems), problems)
	}
}

// TestValidateProductionConfig_RejectsWildcardCORS is A10's regression
// guard: a wildcard CORS origin has no legitimate production justification
// (A7's cors() hardening keeps it from combining with credentials, but the
// release gate should still steer an operator away from it entirely).
func TestValidateProductionConfig_RejectsWildcardCORS(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			APIBaseURL: "https://api.xchats.example", FrontendBaseURL: "https://app.xchats.example",
			CORSOrigins: []string{"*"},
		},
	}
	problems := validateProductionConfig(cfg, true)
	if len(problems) != 1 {
		t.Fatalf("expected exactly 1 problem (wildcard CORS), got %d: %v", len(problems), problems)
	}
}

// TestValidateProductionConfig_RejectsNoCredentialStore is A10's other
// regression guard: independent of A5's webhook-mode-specific fatal check,
// ANY production-labeled deployment with no secure credential store means
// the Settings UI's credential storage and the ngrok tunnel are silently
// disabled — worth failing loudly rather than an operator discovering it
// later.
func TestValidateProductionConfig_RejectsNoCredentialStore(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{APIBaseURL: "https://api.xchats.example", FrontendBaseURL: "https://app.xchats.example"},
	}
	problems := validateProductionConfig(cfg, false)
	if len(problems) != 1 {
		t.Fatalf("expected exactly 1 problem (no credential store), got %d: %v", len(problems), problems)
	}
}

// TestShouldValidateProductionConfig_TriggersOnRealURLsRegardlessOfEnvironment
// is A10's core hardening: deploy/docker-compose.yaml never sets
// ENVIRONMENT=production (its shipped URLs default to localhost precisely so
// the stock quickstart stays ungated), so a real deployment that configures
// real public URLs but never touches ENVIRONMENT must still trigger the
// gate — before this it silently didn't.
func TestShouldValidateProductionConfig_TriggersOnRealURLsRegardlessOfEnvironment(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{"explicit production, local URLs", &config.Config{Environment: "production"}, true},
		{"unset environment, real API URL", &config.Config{Server: config.ServerConfig{APIBaseURL: "https://api.xchats.example"}}, true},
		{"unset environment, real frontend URL", &config.Config{Server: config.ServerConfig{FrontendBaseURL: "https://app.xchats.example"}}, true},
		{"unset environment, both local (the stock docker-compose default)", &config.Config{
			Server: config.ServerConfig{APIBaseURL: "http://localhost:8080", FrontendBaseURL: "http://localhost:8081"},
		}, false},
		{"zero-value config (dev, no URLs set at all)", &config.Config{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldValidateProductionConfig(tc.cfg); got != tc.want {
				t.Errorf("shouldValidateProductionConfig() = %v, want %v", got, tc.want)
			}
		})
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
