package credentials

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// The four secrets xchats manages internally rather than asking an operator
// to configure — infrastructure plumbing, not a user-facing integration, so
// there is no Settings UI card for these the way there is for openrouter.api_key
// et al. Every one of them decodes as 64 hex chars, the format both
// secretbox.FromEnvValue and mcpauth.NewSigningKeyFromSeed already accept.
const (
	// KeySessionSecret is the HMAC key the MCP OAuth CSRF token is signed
	// with (internal/httpapi's mcp_oauth_csrf.go) — cfg.SessionSecret's role
	// before this package existed.
	KeySessionSecret Key = "system.session_secret"
	// KeyTGCredentialsEncKey is the AES-256-GCM key protecting stored
	// Telegram bot tokens at rest (internal/secretbox) —
	// cfg.TelegramCredentialsEncKey's role before this package existed.
	KeyTGCredentialsEncKey Key = "system.tg_credentials_key"
	// KeyMCPJWTSigningKey seeds the Ed25519 keypair that signs MCP OAuth
	// access/refresh tokens (internal/mcpauth) — cfg.MCPJWTSigningKey's role
	// before this package existed.
	KeyMCPJWTSigningKey Key = "system.mcp_jwt_key"
	// KeyWebhookSecret is the secret_token registered with Telegram's
	// setWebhook and checked on every inbound webhook call —
	// cfg.TelegramWebhookSecret's role before this package existed.
	KeyWebhookSecret Key = "system.webhook_secret"
)

// systemSecretSpec pairs a system secret's Key with the legacy env var
// Provision adopts a value from on a deployment's first boot after
// upgrading (see provisionOne).
type systemSecretSpec struct {
	key       Key
	legacyEnv string
}

var systemSecrets = []systemSecretSpec{
	{KeySessionSecret, "SESSION_SECRET"},
	{KeyTGCredentialsEncKey, "TG_CREDENTIALS_ENC_KEY"},
	{KeyMCPJWTSigningKey, "MCP_JWT_SIGNING_KEY"},
	{KeyWebhookSecret, "TG_WEBHOOK_SECRET"},
}

// Secrets holds the resolved value of every internally-managed system
// secret main.go needs at boot.
type Secrets struct {
	SessionSecret       string
	TGCredentialsEncKey string
	MCPJWTSigningKey    string
	WebhookSecret       string
}

// Provision resolves, and durably persists if not already persisted, every
// system secret (see the Key* constants above) through store. For each one:
//
//  1. Already in store? Use it — idempotent across restarts, and
//     authoritative over the legacy env var below (so this survives an
//     operator later unsetting an env var they only needed for migration).
//  2. Not yet in store, but the pre-Settings legacy env var (SESSION_SECRET,
//     TG_CREDENTIALS_ENC_KEY, MCP_JWT_SIGNING_KEY, TG_WEBHOOK_SECRET) is
//     set? Adopt it: write it into store once, so every later boot finds it
//     at step 1 without needing the env var to still be set.
//  3. Neither? Generate 32 random bytes (64 hex chars) and persist that.
//
// store must be writable (a Chain from a successful Open, never one that
// only has EnvStore as its primary) — Provision has no fallback for a
// write failure, by design: main.go decides what to do when no secure store
// is available at all (see credentials.Open's own doc comment).
func Provision(ctx context.Context, store CredentialStore) (Secrets, error) {
	resolved := make(map[Key]string, len(systemSecrets))
	for _, spec := range systemSecrets {
		v, err := provisionOne(ctx, store, spec.key, spec.legacyEnv)
		if err != nil {
			return Secrets{}, fmt.Errorf("credentials: provision %s: %w", spec.key, err)
		}
		resolved[spec.key] = v
	}
	return Secrets{
		SessionSecret:       resolved[KeySessionSecret],
		TGCredentialsEncKey: resolved[KeyTGCredentialsEncKey],
		MCPJWTSigningKey:    resolved[KeyMCPJWTSigningKey],
		WebhookSecret:       resolved[KeyWebhookSecret],
	}, nil
}

func provisionOne(ctx context.Context, store CredentialStore, key Key, legacyEnv string) (string, error) {
	if v, err := store.Get(ctx, key); err == nil {
		return v, nil
	} else if !errors.Is(err, ErrNotFound) {
		return "", err
	}

	if v := os.Getenv(legacyEnv); v != "" {
		if err := store.Set(ctx, key, v); err != nil {
			return "", err
		}
		return v, nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	v := hex.EncodeToString(raw)
	if err := store.Set(ctx, key, v); err != nil {
		return "", err
	}
	return v, nil
}
