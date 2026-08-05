package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/yerassyldanay/xchats/backend/internal/appdirs"
)

// settingsFileName is the file Store reads and writes under its configDir.
const settingsFileName = "settings.json"

// Store is a mutex-guarded, atomically-persisted settings.json — one
// process-wide instance, shared by every httpapi handler that reads or
// writes Settings, so a concurrent GET always sees a fully-applied write
// and never a half-written one.
type Store struct {
	path string

	mu     sync.Mutex
	data   Settings
	loaded bool
}

// NewStore returns a Store rooted at <configDir>/settings.json. Nothing is
// read from (or written to) disk until the first Load/Save/Update call.
func NewStore(configDir string) *Store {
	return &Store{path: filepath.Join(configDir, settingsFileName)}
}

// Load returns the current settings, reading them from disk on the first
// call (and caching them in memory for every call after). A missing file is
// not an error: it is treated as a fresh install, and the resulting
// defaults (see the defaults function) are written to disk immediately, so
// they are durable from this very first Load rather than only once
// something later happens to call Save. A present-but-corrupt file (invalid
// JSON) IS an error — this package never silently discards settings an
// operator or the Settings UI already saved.
func (s *Store) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return Settings{}, err
	}
	return s.data, nil
}

// Save overwrites the entire stored Settings and persists it atomically.
// Most callers want Update instead — Save is for a caller that has already
// loaded, fully reconstructed, and wants no partial-merge behavior at all.
func (s *Store) Save(data Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
	s.loaded = true
	return s.saveLocked()
}

// Update loads the current settings (seeding defaults on a first run
// exactly like Load), applies fn to mutate them in place, persists the
// result, and returns it — the read-modify-write every Settings UI PUT
// handler uses, done under this Store's own mutex so two concurrent saves
// can never interleave and silently drop one's change.
func (s *Store) Update(fn func(*Settings)) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return Settings{}, err
	}
	fn(&s.data)
	if err := s.saveLocked(); err != nil {
		return Settings{}, err
	}
	return s.data, nil
}

func (s *Store) loadLocked() error {
	if s.loaded {
		return nil
	}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.data = defaults()
		s.loaded = true
		return s.saveLocked()
	}
	if err != nil {
		return fmt.Errorf("settings: read %s: %w", s.path, err)
	}
	var data Settings
	if err := json.Unmarshal(b, &data); err != nil {
		return fmt.Errorf("settings: parse %s: %w", s.path, err)
	}
	if data.Providers == nil {
		data.Providers = map[string]ProviderSettings{}
	}
	s.data = data
	s.loaded = true
	return nil
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := appdirs.EnsureDir(filepath.Dir(s.path)); err != nil {
		return err
	}
	return writeFileAtomic(s.path, b, 0o600)
}

// writeFileAtomic writes data to a temp file alongside path, fsyncs it,
// then renames it over path — the rename is atomic on every platform this
// runs on, so a concurrent reader (or a crash mid-write) never observes a
// partially written settings.json. Deliberately duplicated from
// internal/credentials' own writeFileAtomic (identical few lines) rather
// than shared: a generic-file-write micro-helper does not warrant a new
// package edge between the two.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".settings-*.tmp")
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
