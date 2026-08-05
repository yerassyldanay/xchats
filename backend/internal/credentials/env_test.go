package credentials

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnvNameConversion(t *testing.T) {
	cases := []struct {
		key  Key
		want string
	}{
		{"openrouter.api_key", "OPENROUTER_API_KEY"},
		{"langfuse.public_key", "LANGFUSE_PUBLIC_KEY"},
		{"ngrok.authtoken", "NGROK_AUTHTOKEN"},
		{"system.session_secret", "SYSTEM_SESSION_SECRET"},
	}
	for _, tc := range cases {
		if got := envName(tc.key); got != tc.want {
			t.Errorf("envName(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestEnvStorePlainVar(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-from-env")
	got, err := EnvStore{}.Get(context.Background(), "openrouter.api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "sk-from-env" {
		t.Errorf("Get = %q, want %q", got, "sk-from-env")
	}
}

func TestEnvStoreNotFound(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY_FILE", "")
	t.Setenv("XCHATS_SECRETS_DIR", "")
	if _, err := (EnvStore{}).Get(context.Background(), "openrouter.api_key"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get with nothing set: err = %v, want ErrNotFound", err)
	}
}

func TestEnvStoreFileTierWinsOverPlainVar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.txt")
	if err := os.WriteFile(path, []byte("sk-from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENROUTER_API_KEY_FILE", path)
	t.Setenv("OPENROUTER_API_KEY", "sk-from-plain-var")
	got, err := (EnvStore{}).Get(context.Background(), "openrouter.api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// Trailing whitespace/newline trimmed — a file written by `echo` or an
	// editor commonly has one.
	if got != "sk-from-file" {
		t.Errorf("Get = %q, want %q (the _FILE tier, trimmed)", got, "sk-from-file")
	}
}

func TestEnvStoreFileTierMissingFileIsHardError(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY_FILE", filepath.Join(t.TempDir(), "does-not-exist"))
	t.Setenv("OPENROUTER_API_KEY", "sk-from-plain-var")
	_, err := (EnvStore{}).Get(context.Background(), "openrouter.api_key")
	if err == nil {
		t.Fatal("Get with a missing _FILE target: want an error, got nil")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("a missing _FILE target must be a hard error, not silently fall through as ErrNotFound")
	}
}

func TestEnvStoreSecretsDirTier(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "openrouter.api_key"), []byte("sk-from-secrets-dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XCHATS_SECRETS_DIR", dir)
	t.Setenv("OPENROUTER_API_KEY", "sk-from-plain-var")
	got, err := (EnvStore{}).Get(context.Background(), "openrouter.api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "sk-from-secrets-dir" {
		t.Errorf("Get = %q, want %q (the secrets-dir tier)", got, "sk-from-secrets-dir")
	}
}

func TestEnvStoreSecretsDirTierFallsThroughWhenKeyAbsent(t *testing.T) {
	dir := t.TempDir() // empty — no file for this key
	t.Setenv("XCHATS_SECRETS_DIR", dir)
	t.Setenv("OPENROUTER_API_KEY", "sk-from-plain-var")
	got, err := (EnvStore{}).Get(context.Background(), "openrouter.api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "sk-from-plain-var" {
		t.Errorf("Get = %q, want the plain-var tier %q", got, "sk-from-plain-var")
	}
}

func TestEnvStoreSetAndDeleteAreReadOnly(t *testing.T) {
	ctx := context.Background()
	if err := (EnvStore{}).Set(ctx, "openrouter.api_key", "x"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Set: err = %v, want ErrReadOnly", err)
	}
	if err := (EnvStore{}).Delete(ctx, "openrouter.api_key"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Delete: err = %v, want ErrReadOnly", err)
	}
}

var _ CredentialStore = EnvStore{}
