package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
