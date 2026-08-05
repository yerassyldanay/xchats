package credentials

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvStore reads credential values from the process environment — the
// read-only backend that keeps every existing *_API_KEY / *_AUTHTOKEN env
// var working as an override even after Track 2J removes .env entirely.
// For a Key like "openrouter.api_key" (env name OPENROUTER_API_KEY, see
// envName), Get checks, in order:
//
//  1. $OPENROUTER_API_KEY_FILE — the path to a file holding the value (the
//     Docker/Kubernetes "secret mounted as a file" convention). An operator
//     who set this variable explicitly promised a readable file there, so a
//     missing/unreadable file is a hard error, not a fall-through.
//  2. $XCHATS_SECRETS_DIR/openrouter.api_key — a file named after the RAW
//     key inside a shared secrets directory (the Docker Compose/Swarm
//     secrets-mount convention). Unlike the _FILE case, this key simply not
//     being present under the shared directory is normal — falls through.
//  3. $OPENROUTER_API_KEY — the literal env var.
//
// Set and Delete always fail with ErrReadOnly: this process cannot rewrite
// its own environment, or a file some other process mounted for it.
type EnvStore struct{}

var _ CredentialStore = EnvStore{}

func (EnvStore) Get(_ context.Context, key Key) (string, error) {
	name := envName(key)
	if path := os.Getenv(name + "_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("credentials: read %s (from $%s_FILE): %w", path, name, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	if dir := os.Getenv("XCHATS_SECRETS_DIR"); dir != "" {
		path := filepath.Join(dir, string(key))
		b, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(b)), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("credentials: read %s: %w", path, err)
		}
		// Not present under the shared secrets dir — try the next tier.
	}
	if v := os.Getenv(name); v != "" {
		return v, nil
	}
	return "", ErrNotFound
}

func (EnvStore) Set(_ context.Context, _ Key, _ string) error {
	return ErrReadOnly
}

func (EnvStore) Delete(_ context.Context, _ Key) error {
	return ErrReadOnly
}

// envName converts a Key ("openrouter.api_key") into the conventional env
// var name (OPENROUTER_API_KEY) every field's config.go env tag already
// used before Settings existed — so an operator's existing export keeps
// working completely unchanged.
func envName(key Key) string {
	return strings.ToUpper(strings.ReplaceAll(string(key), ".", "_"))
}
