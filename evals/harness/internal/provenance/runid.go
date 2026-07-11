// Package provenance holds everything the eval harness needs to make a run
// reproducible and auditable: collision-safe run IDs, atomic writes, the run
// manifest, snapshot helpers, and versioned prompt loading. It is a leaf package —
// no dependency on eval-specific types (Verdict, ExtractCase, extractRunResult,
// etc., which stay in package main) — so it can be imported without pulling in
// either eval family's grading logic.
package provenance

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// NewRunDir creates a fresh run directory under runsRoot with a collision-safe ID:
// <timestamp>-<4 hex chars>. Two harness processes started in the same second (a real
// risk — extraction and scenario runs are both sub-second fast) must never share a run
// dir, so directory creation uses os.Mkdir (fails if the dir already exists, unlike
// MkdirAll) and retries with a fresh random suffix on collision.
func NewRunDir(runsRoot string) (id, dir string, err error) {
	if err := os.MkdirAll(runsRoot, 0o755); err != nil {
		return "", "", err
	}
	const maxAttempts = 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		candidateID := time.Now().Format("2006-01-02_15-04-05") + "-" + randSuffix(4)
		candidateDir := filepath.Join(runsRoot, candidateID)
		if mkErr := os.Mkdir(candidateDir, 0o755); mkErr == nil {
			return candidateID, candidateDir, nil
		} else if !os.IsExist(mkErr) {
			return "", "", mkErr
		}
	}
	return "", "", fmt.Errorf("NewRunDir: could not create a unique run dir under %s after %d attempts", runsRoot, maxAttempts)
}

// randSuffix returns n lowercase hex characters. crypto/rand failure is effectively
// unrecoverable in this environment (no entropy source), so fall back to a
// nanosecond-derived suffix rather than fail run creation outright — collision
// avoidance degrades gracefully instead of blocking the eval.
func randSuffix(n int) string {
	b := make([]byte, n/2+1)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%04x", time.Now().UnixNano()&0xffff)[:n]
	}
	return hex.EncodeToString(b)[:n]
}

// AtomicWriteFile writes data to path via a temp file in the same directory followed
// by a rename, so a crash mid-write or a concurrent reader (e.g. the HTML viewer
// running against a live run) never observes a partially written file.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
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
