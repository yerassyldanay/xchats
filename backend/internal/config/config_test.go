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
