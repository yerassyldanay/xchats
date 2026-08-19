// Package credentials is xchats' provider-secret storage: LLM API keys,
// ngrok's authtoken, Langfuse's key pair, and xchats' own internally
// generated secrets (session/CSRF signing, Telegram token encryption, MCP
// JWT signing, the Telegram webhook secret_token) — everything that used to
// live only in a hand-edited .env file, now readable and writable at
// runtime from the Settings UI.
//
// Three backends implement Store: Keyring (the OS-native secret store —
// preferred wherever one is available), FileStore (an AES-256-GCM-sealed
// file, the last resort), and EnvStore (read-only, the process
// environment — how every existing *_API_KEY-style deployment keeps working
// unchanged). Chain layers an env overlay over whichever of Keyring/
// FileStore Open selects, and Provision uses a Store to resolve xchats' own
// internally-managed secrets once at boot.
package credentials

import (
	"context"
	"errors"
)

// Key names one stored secret: "<provider>.<field>" for a Settings-UI
// integration (e.g. "openrouter.api_key", "langfuse.secret_key"), or
// "system.<name>" for one of xchats' own internally-managed secrets (see
// Provision). A Key never carries a value itself.
type Key string

// KeyInstagramAppID/KeyInstagramAppSecret store the Instagram App ID/Secret
// issued by the Meta Dashboard's "Instagram > API setup with Instagram
// login" panel — a DIFFERENT pair from meta.app_id/meta.app_secret (the
// "meta" provider below), which is the Meta Developer App id/secret
// Messenger and WhatsApp Cloud use. Business Login for Instagram
// (internal/meta.InstagramAuthorizeURL) rejects the Meta app id outright —
// see internal/meta/appcreds.go's InstagramAppCredentials doc comment —
// so there is deliberately no fallback to meta.app_id/meta.app_secret here:
// that value is exactly what breaks the Instagram connect dialog.
const (
	KeyInstagramAppID     Key = "instagram.app_id"
	KeyInstagramAppSecret Key = "instagram.app_secret"
)

var (
	// ErrNotFound is returned when no value is stored for a key.
	ErrNotFound = errors.New("credentials: not found")

	// ErrReadOnly is returned by Set/Delete on a store that cannot be
	// written to — EnvStore's values come from the process environment (and
	// mounted secret files), which this process has no way to rewrite.
	ErrReadOnly = errors.New("credentials: store is read-only")

	// ErrNoSecureStore is returned by Open when no backend is usable: the OS
	// keychain failed its availability probe, and the caller did not opt
	// into the file-based fallback (AllowFile / XCHATS_ALLOW_FILE_CREDENTIALS).
	ErrNoSecureStore = errors.New("credentials: no secure credential store available")

	// ErrInvalidCredential is returned by a Provider's Validate when the
	// remote API rejected the credential outright (401/403, or Gemini's own
	// 400 API_KEY_INVALID shape) — confirmed bad, not merely unchecked.
	ErrInvalidCredential = errors.New("credentials: invalid credential")

	// ErrValidationUnavailable is returned by a Provider's Validate when the
	// check itself could not complete (network error, timeout, an
	// unexpected response) — the credential's validity is simply unknown,
	// and callers must never treat this the same as a confirmed rejection.
	ErrValidationUnavailable = errors.New("credentials: validation unavailable")
)

// CredentialStore reads, writes, and deletes secret values by Key. Keyring,
// FileStore, EnvStore, and Chain all satisfy it.
type CredentialStore interface {
	Get(ctx context.Context, key Key) (string, error)
	Set(ctx context.Context, key Key, value string) error
	Delete(ctx context.Context, key Key) error
}
