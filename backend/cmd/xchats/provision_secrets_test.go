package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// forceFileCredentialStore points internal/credentials.Open at a throwaway
// file-backed store regardless of whether this host's OS keychain happens
// to be available — the deterministic way to exercise
// provisionSystemSecrets' "a secure store IS available" path from a test.
func forceFileCredentialStore(t *testing.T) {
	t.Helper()
	t.Setenv("XCHATS_ALLOW_FILE_CREDENTIALS", "1")
	t.Setenv("XCHATS_DATA_DIR", t.TempDir())
	t.Setenv("XCHATS_CONFIG_DIR", t.TempDir())
}

func TestProvisionSystemSecretsPopulatesAllFourFields(t *testing.T) {
	forceFileCredentialStore(t)
	cfg := &config.Config{}
	provisionSystemSecrets(context.Background(), cfg, discardLogger())

	for name, v := range map[string]string{
		"SessionSecret":             cfg.SessionSecret,
		"TelegramCredentialsEncKey": cfg.TelegramCredentialsEncKey,
		"MCPJWTSigningKey":          cfg.MCPJWTSigningKey,
		"TelegramWebhookSecret":     cfg.TelegramWebhookSecret,
	} {
		if len(v) != 64 {
			t.Errorf("cfg.%s = %q (len %d), want a 64-char hex string", name, v, len(v))
		}
	}
}

func TestProvisionSystemSecretsPersistsAcrossBoots(t *testing.T) {
	forceFileCredentialStore(t)

	first := &config.Config{}
	provisionSystemSecrets(context.Background(), first, discardLogger())

	// A second "boot" against the same forced data dir must resolve the
	// SAME secrets — this is the whole point: generated once, durable after.
	second := &config.Config{}
	provisionSystemSecrets(context.Background(), second, discardLogger())

	if first.SessionSecret != second.SessionSecret ||
		first.TelegramCredentialsEncKey != second.TelegramCredentialsEncKey ||
		first.MCPJWTSigningKey != second.MCPJWTSigningKey ||
		first.TelegramWebhookSecret != second.TelegramWebhookSecret {
		t.Errorf("secrets differ across two provisionSystemSecrets calls over the same data dir:\nfirst:  %+v\nsecond: %+v",
			first, second)
	}
}

func TestProvisionSystemSecretsLeavesConfigUntouchedWithNoSecureStore(t *testing.T) {
	// No AllowFile opt-in, and this sandboxed test environment has no OS
	// keychain — Open() falls through to ErrNoSecureStore, and cfg must be
	// left exactly as the caller built it (config/env values still apply
	// downstream, unchanged).
	t.Setenv("XCHATS_ALLOW_FILE_CREDENTIALS", "")
	cfg := &config.Config{TelegramWebhookSecret: "explicit-from-config-yaml"}
	provisionSystemSecrets(context.Background(), cfg, discardLogger())

	if cfg.SessionSecret != "" {
		t.Errorf("SessionSecret = %q, want unchanged empty string", cfg.SessionSecret)
	}
	if cfg.TelegramWebhookSecret != "explicit-from-config-yaml" {
		t.Errorf("TelegramWebhookSecret = %q, want the untouched original value", cfg.TelegramWebhookSecret)
	}
}
