package credentials

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
)

func testBox(t *testing.T) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(make([]byte, 32)) // deterministic, test-only key
	if err != nil {
		t.Fatalf("secretbox.New: %v", err)
	}
	return box
}

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir, testBox(t))
	ctx := context.Background()

	if _, err := fs.Get(ctx, "openrouter.api_key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get before Set: err = %v, want ErrNotFound", err)
	}
	if err := fs.Set(ctx, "openrouter.api_key", "sk-test"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := fs.Get(ctx, "openrouter.api_key")
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if got != "sk-test" {
		t.Errorf("Get = %q, want %q", got, "sk-test")
	}
}

func TestFileStorePersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	box := testBox(t)
	ctx := context.Background()

	if err := NewFileStore(dir, box).Set(ctx, "openrouter.api_key", "sk-persisted"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// A brand new FileStore over the same dir/box must see it — proves the
	// value actually reached disk, not just this instance's in-memory map.
	got, err := NewFileStore(dir, box).Get(ctx, "openrouter.api_key")
	if err != nil {
		t.Fatalf("Get from a fresh FileStore: %v", err)
	}
	if got != "sk-persisted" {
		t.Errorf("Get = %q, want %q", got, "sk-persisted")
	}
}

func TestFileStoreFilePermissionsAre0600(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir, testBox(t))
	if err := fs.Set(context.Background(), "openrouter.api_key", "sk-test"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, credentialsFileName))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %o, want %o", perm, 0o600)
	}
}

func TestFileStoreDelete(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir, testBox(t))
	ctx := context.Background()
	if err := fs.Set(ctx, "openrouter.api_key", "sk-test"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Delete(ctx, "openrouter.api_key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := fs.Get(ctx, "openrouter.api_key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: err = %v, want ErrNotFound", err)
	}
	if err := fs.Delete(ctx, "openrouter.api_key"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete of an already-deleted key: err = %v, want ErrNotFound", err)
	}
}

func TestFileStoreMultipleKeysIndependentlySealed(t *testing.T) {
	dir := t.TempDir()
	fs := NewFileStore(dir, testBox(t))
	ctx := context.Background()
	if err := fs.Set(ctx, "openrouter.api_key", "a"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set(ctx, "openai.api_key", "b"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Delete(ctx, "openrouter.api_key"); err != nil {
		t.Fatal(err)
	}
	got, err := fs.Get(ctx, "openai.api_key")
	if err != nil || got != "b" {
		t.Errorf("openai.api_key survived a sibling's delete: got (%q, %v), want (%q, nil)", got, err, "b")
	}
}

func TestFileStoreWrongKeyFailsToDecrypt(t *testing.T) {
	dir := t.TempDir()
	box1 := testBox(t)
	if err := NewFileStore(dir, box1).Set(context.Background(), "openrouter.api_key", "sk-test"); err != nil {
		t.Fatal(err)
	}
	box2, err := secretbox.New(append(make([]byte, 31), 1)) // a DIFFERENT 32-byte key
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewFileStore(dir, box2).Get(context.Background(), "openrouter.api_key"); err == nil {
		t.Fatal("Get with the wrong key: want an error, got nil")
	}
}

func TestFileStoreMissingFileIsEmptyNotError(t *testing.T) {
	dir := t.TempDir() // no credentials.enc written yet
	if _, err := NewFileStore(dir, testBox(t)).Get(context.Background(), "openrouter.api_key"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get against a directory with no credentials.enc: err = %v, want ErrNotFound", err)
	}
}

func TestFileKeyFromEnvOverride(t *testing.T) {
	rawHex := strings.Repeat("0123456789abcdef", 4) // 64 hex chars = 32 bytes
	t.Setenv("XCHATS_CREDENTIALS_KEY", rawHex)
	dir := t.TempDir()
	box, generated, err := FileKey(dir)
	if err != nil {
		t.Fatalf("FileKey: %v", err)
	}
	if generated {
		t.Error("generated = true when $XCHATS_CREDENTIALS_KEY was set, want false")
	}
	if box == nil {
		t.Fatal("box is nil")
	}
	// No credentials.key file should have been written — the env override
	// bypasses file-based key storage entirely.
	if _, err := os.Stat(filepath.Join(dir, credentialsKeyFileName)); !os.IsNotExist(err) {
		t.Errorf("credentials.key exists despite the env override: stat err = %v", err)
	}
}

func TestFileKeyGeneratesAndPersists(t *testing.T) {
	t.Setenv("XCHATS_CREDENTIALS_KEY", "")
	dir := t.TempDir()
	box1, generated, err := FileKey(dir)
	if err != nil {
		t.Fatalf("FileKey (first call): %v", err)
	}
	if !generated {
		t.Error("generated = false on first call with no existing key file, want true")
	}
	keyPath := filepath.Join(dir, credentialsKeyFileName)
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat %s: %v", keyPath, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("credentials.key permissions = %o, want %o", perm, 0o600)
	}

	// A second call must read the SAME key back, not generate a new one —
	// otherwise every restart would silently lose access to everything
	// FileStore already sealed with the first key.
	box2, generated2, err := FileKey(dir)
	if err != nil {
		t.Fatalf("FileKey (second call): %v", err)
	}
	if generated2 {
		t.Error("generated = true on second call reading an existing key file, want false")
	}
	sealed, err := box1.Seal([]byte("probe"))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := box2.Open(sealed)
	if err != nil || string(opened) != "probe" {
		t.Error("the two FileKey calls returned boxes sealed with different keys")
	}
}

func TestFileKeyEnvOverrideRejectsInvalidValue(t *testing.T) {
	t.Setenv("XCHATS_CREDENTIALS_KEY", "not-a-valid-key")
	if _, _, err := FileKey(t.TempDir()); err == nil {
		t.Fatal("FileKey with an invalid $XCHATS_CREDENTIALS_KEY: want an error, got nil")
	}
}

func TestWriteFileAtomicFailsWhenParentDirectoryIsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist", "credentials.enc")
	if err := writeFileAtomic(path, []byte("data"), 0o600); err == nil {
		t.Fatal("want an error when the parent directory does not exist")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("target file should not have been created; stat err = %v", err)
	}
}

var _ CredentialStore = (*FileStore)(nil)
