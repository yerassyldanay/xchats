package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_MockExternalsAndPprofAddrPrecedence covers the hermetic-profiling
// harness's two new SystemConfig fields: config.yaml sets a value, then
// MOCK_EXTERNALS/PPROF_ADDR (env overrides config.yaml for ops — see Load's
// own doc comment) overrides it. CLI-over-both is cmd/xchats' own concern
// (applyCLIOverrides), not this package's.
func TestLoad_MockExternalsAndPprofAddrPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := "system:\n  mock_externals: true\n  pprof_addr: \"127.0.0.1:9000\"\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	t.Run("yaml alone", func(t *testing.T) {
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !cfg.System.MockExternals || cfg.System.PprofAddr != "127.0.0.1:9000" {
			t.Errorf("got MockExternals=%v PprofAddr=%q, want true/127.0.0.1:9000", cfg.System.MockExternals, cfg.System.PprofAddr)
		}
	})

	t.Run("env overrides yaml", func(t *testing.T) {
		t.Setenv("MOCK_EXTERNALS", "false")
		t.Setenv("PPROF_ADDR", "127.0.0.1:6060")
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.System.MockExternals {
			t.Error("MOCK_EXTERNALS=false must override config.yaml's mock_externals: true")
		}
		if cfg.System.PprofAddr != "127.0.0.1:6060" {
			t.Errorf("PPROF_ADDR must override config.yaml, got %q", cfg.System.PprofAddr)
		}
	})

	t.Run("defaults are off/empty with no config or env at all", func(t *testing.T) {
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.System.MockExternals || cfg.System.PprofAddr != "" {
			t.Errorf("expected zero-value defaults, got MockExternals=%v PprofAddr=%q", cfg.System.MockExternals, cfg.System.PprofAddr)
		}
	})
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

// chdir switches the test process's CWD to dir and restores it on cleanup —
// safe here because this package's tests never run in parallel (t.Parallel
// is never called), so there is only ever one CWD in flight.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore chdir %s: %v", old, err)
		}
	})
}

func TestResolveConfigPath(t *testing.T) {
	t.Run("explicit path always wins", func(t *testing.T) {
		t.Setenv("XCHATS_CONFIG", "/from/env.yaml")
		if got := ResolveConfigPath("/explicit/path.yaml"); got != "/explicit/path.yaml" {
			t.Errorf("ResolveConfigPath = %q, want the explicit path", got)
		}
	})

	t.Run("XCHATS_CONFIG wins over everything but explicit", func(t *testing.T) {
		t.Setenv("XCHATS_CONFIG", "/from/env.yaml")
		chdir(t, t.TempDir()) // no ./config.yaml here — must not matter
		if got := ResolveConfigPath(""); got != "/from/env.yaml" {
			t.Errorf("ResolveConfigPath = %q, want $XCHATS_CONFIG's value", got)
		}
	})

	t.Run("./config.yaml wins when present in the working directory", func(t *testing.T) {
		t.Setenv("XCHATS_CONFIG", "")
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("environment: test\n"), 0o644); err != nil {
			t.Fatalf("write config.yaml: %v", err)
		}
		chdir(t, dir)
		if got := ResolveConfigPath(""); got != "config.yaml" {
			t.Errorf("ResolveConfigPath = %q, want the relative %q", got, "config.yaml")
		}
	})

	t.Run("falls back to the OS config directory when no ./config.yaml exists", func(t *testing.T) {
		t.Setenv("XCHATS_CONFIG", "")
		chdir(t, t.TempDir()) // empty — no config.yaml here
		configDir := t.TempDir()
		t.Setenv("XCHATS_CONFIG_DIR", configDir)
		want := filepath.Join(configDir, "config.yaml")
		if got := ResolveConfigPath(""); got != want {
			t.Errorf("ResolveConfigPath = %q, want %q", got, want)
		}
	})
}

func TestTelegramResolvedWebhookSecret(t *testing.T) {
	cases := []struct {
		name                  string
		telegramWebhookSecret string
		want                  string
	}{
		{"set: returned as-is", "tg-secret", "tg-secret"},
		{"unset: empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{TelegramWebhookSecret: tc.telegramWebhookSecret}
			if got := c.TelegramResolvedWebhookSecret(); got != tc.want {
				t.Errorf("TelegramResolvedWebhookSecret() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTelegramResolvedWebhookBaseURL(t *testing.T) {
	cases := []struct {
		name       string
		explicit   string
		apiBaseURL string
		want       string
	}{
		{"explicit wins over the api origin", "https://tg.example.com", "https://app.example.com", "https://tg.example.com"},
		{"explicit trailing slash trimmed", "https://tg.example.com/", "", "https://tg.example.com"},
		// The whole point of the fallback: an ngrok-fronted deploy never has to
		// restate the tunnel hostname in a second setting.
		{"falls back to the https api origin (the ngrok tunnel)", "", "https://x.ngrok-free.app", "https://x.ngrok-free.app"},
		{"api origin trailing slash trimmed", "", "https://x.ngrok-free.app/", "https://x.ngrok-free.app"},
		// http:// would only trade a clear "not configured" for a Bot API 400.
		{"http api origin is not a usable fallback", "", "http://localhost:8080", ""},
		{"nothing configured", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{}
			c.Telegram.WebhookPublicBaseURL = tc.explicit
			c.Server.APIBaseURL = tc.apiBaseURL
			if got := c.TelegramResolvedWebhookBaseURL(); got != tc.want {
				t.Errorf("TelegramResolvedWebhookBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTelegramResolvedModeFollowsTheTunnelOrigin(t *testing.T) {
	cases := []struct {
		name       string
		mode       string
		explicit   string
		apiBaseURL string
		want       string
	}{
		{"explicit mode always wins", "polling", "https://tg.example.com", "https://x.ngrok-free.app", "polling"},
		{"explicit webhook wins with nothing configured", "webhook", "", "", "webhook"},
		// Was polling before TelegramResolvedWebhookBaseURL existed: webhook
		// delivery is genuinely available on the tunnel origin, so use it.
		{"https api origin alone implies webhook", "", "", "https://x.ngrok-free.app", "webhook"},
		{"no public https origin means polling", "", "", "http://localhost:8080", "polling"},
		{"zero config means polling", "", "", "", "polling"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{}
			c.Telegram.Mode = tc.mode
			c.Telegram.WebhookPublicBaseURL = tc.explicit
			c.Server.APIBaseURL = tc.apiBaseURL
			if got := c.TelegramResolvedMode(); got != tc.want {
				t.Errorf("TelegramResolvedMode() = %q, want %q", got, tc.want)
			}
		})
	}
}
