package credentials

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yerassyldanay/xchats/backend/internal/appdirs"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
)

// credentialsFileName/credentialsKeyFileName are the two files FileStore and
// FileKey manage under a data directory — always paired: a key file with no
// credentials file is inert, and a credentials file with no key file is
// unreadable.
const (
	credentialsFileName    = "credentials.enc"
	credentialsKeyFileName = "credentials.key"
)

// FileStore is the last-resort CredentialStore backend: every value is
// AES-256-GCM sealed individually (internal/secretbox — one seal per value,
// not one seal over the whole file, so a single corrupted entry can never
// take the rest down with it) and the result written to
// <dataDir>/credentials.enc, 0600. Chosen only when neither the OS keychain
// nor an env-only configuration is available, and the deployment has
// explicitly opted in — see Open — since a file on disk, even encrypted, is
// a weaker guarantee than an OS-native secret store.
type FileStore struct {
	path string
	box  *secretbox.Box

	mu     sync.Mutex
	values map[Key]string // decrypted, in-memory mirror; the file is the durable copy
	loaded bool
}

// NewFileStore returns a FileStore rooted at <dataDir>/credentials.enc,
// sealing every value with box (see FileKey for how that key is resolved).
func NewFileStore(dataDir string, box *secretbox.Box) *FileStore {
	return &FileStore{path: filepath.Join(dataDir, credentialsFileName), box: box}
}

var _ CredentialStore = (*FileStore)(nil)

// fileEntry is one credentials.enc record. Sealed is
// base64(nonce||ciphertext) — secretbox.Seal's output, encoded so the whole
// file stays valid JSON.
type fileEntry struct {
	Sealed string `json:"sealed"`
}

func (f *FileStore) loadLocked() error {
	if f.loaded {
		return nil
	}
	f.values = map[Key]string{}
	b, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		f.loaded = true
		return nil
	}
	if err != nil {
		return fmt.Errorf("credentials: read %s: %w", f.path, err)
	}
	var raw map[Key]fileEntry
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("credentials: parse %s: %w", f.path, err)
	}
	for k, entry := range raw {
		sealed, err := base64.StdEncoding.DecodeString(entry.Sealed)
		if err != nil {
			return fmt.Errorf("credentials: decode %s: %w", k, err)
		}
		plain, err := f.box.Open(sealed)
		if err != nil {
			return fmt.Errorf("credentials: decrypt %s: %w", k, err)
		}
		f.values[k] = string(plain)
	}
	f.loaded = true
	return nil
}

func (f *FileStore) Get(_ context.Context, key Key) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.loadLocked(); err != nil {
		return "", err
	}
	v, ok := f.values[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (f *FileStore) Set(_ context.Context, key Key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.loadLocked(); err != nil {
		return err
	}
	f.values[key] = value
	return f.saveLocked()
}

func (f *FileStore) Delete(_ context.Context, key Key) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.loadLocked(); err != nil {
		return err
	}
	if _, ok := f.values[key]; !ok {
		return ErrNotFound
	}
	delete(f.values, key)
	return f.saveLocked()
}

// saveLocked re-seals every in-memory value and rewrites the file
// atomically (see writeFileAtomic) — a crash mid-write leaves the previous,
// still-valid file in place rather than a truncated one.
func (f *FileStore) saveLocked() error {
	raw := make(map[Key]fileEntry, len(f.values))
	for k, v := range f.values {
		sealed, err := f.box.Seal([]byte(v))
		if err != nil {
			return fmt.Errorf("credentials: seal %s: %w", k, err)
		}
		raw[k] = fileEntry{Sealed: base64.StdEncoding.EncodeToString(sealed)}
	}
	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := appdirs.EnsureDir(filepath.Dir(f.path)); err != nil {
		return err
	}
	return writeFileAtomic(f.path, b, 0o600)
}

// writeFileAtomic writes data to a temp file alongside path, fsyncs it, then
// renames it over path — the rename is atomic on every platform this runs
// on, so a concurrent reader (or a crash mid-write) never observes a
// partially written file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below has succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// FileKey resolves the AES-256-GCM key FileStore seals values with:
//
//  1. $XCHATS_CREDENTIALS_KEY, if set (see secretbox.FromEnvValue for the
//     accepted formats: 64 hex chars, base64, or a 32-byte literal).
//  2. <dataDir>/credentials.key — read if it already exists.
//  3. Otherwise: 32 random bytes, hex-encoded and written to
//     <dataDir>/credentials.key (0600), and generated=true is returned so
//     the caller can warn that losing dataDir with no backup means losing
//     every stored credential.
func FileKey(dataDir string) (box *secretbox.Box, generated bool, err error) {
	if v := os.Getenv("XCHATS_CREDENTIALS_KEY"); v != "" {
		box, err = secretbox.FromEnvValue(v)
		return box, false, err
	}
	if err := appdirs.EnsureDir(dataDir); err != nil {
		return nil, false, err
	}
	keyPath := filepath.Join(dataDir, credentialsKeyFileName)
	b, err := os.ReadFile(keyPath)
	if err == nil {
		box, err = secretbox.FromEnvValue(strings.TrimSpace(string(b)))
		return box, false, err
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, fmt.Errorf("credentials: read %s: %w", keyPath, err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, false, fmt.Errorf("credentials: generate key: %w", err)
	}
	encoded := hex.EncodeToString(raw)
	if err := writeFileAtomic(keyPath, []byte(encoded), 0o600); err != nil {
		return nil, false, fmt.Errorf("credentials: write %s: %w", keyPath, err)
	}
	box, err = secretbox.New(raw)
	return box, true, err
}
