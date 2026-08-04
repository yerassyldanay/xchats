package dbops

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// Restore atomically replaces the database at destPath with backupPath's
// contents. It is an offline operation: destPath must not be open by any
// process under internal/dbx's single-process model (a running "xchats
// serve" holds exactly this exclusion via dbx.Open) — Restore acquires
// that SAME lock (dbx.LockPath) for the duration of the copy, so a
// concurrent Open attempt (this app starting up mid-restore) blocks
// instead of racing a half-written file, and a database already in use
// makes Restore fail fast instead of corrupting live data.
//
// backupPath is validated — it must open (dbx.OpenReadOnly, which never
// mutates the file being inspected — see its doc comment) and pass
// IntegrityCheck — before anything at destPath is touched, so a missing,
// non-database, or internally corrupt backupPath is rejected without side
// effects.
func Restore(ctx context.Context, backupPath, destPath string) error {
	if err := validateBackup(ctx, backupPath); err != nil {
		return err
	}

	// The lock file must be creatable before TryLock can even try — a
	// brand-new destPath (restoring into a fresh environment) may name a
	// directory that does not exist yet, so this has to happen before the
	// lock attempt below, not after.
	if dir := filepath.Dir(destPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("dbops: create directory for %q: %w", destPath, err)
		}
	}

	lockFile, err := dbx.LockPath(destPath)
	if err != nil {
		return err
	}
	fl := flock.New(lockFile)
	locked, err := fl.TryLock()
	if err != nil {
		return fmt.Errorf("dbops: acquire lock for %q: %w", destPath, err)
	}
	if !locked {
		return fmt.Errorf("dbops: %q is currently open by another process — stop it before restoring", destPath)
	}
	defer fl.Unlock()

	// Copy to a temp file beside destPath, then atomically rename over it —
	// a crash or interruption mid-copy leaves destPath as either the OLD
	// complete file or untouched, never a half-written one. Remove any
	// leftover temp file from a PRIOR crashed/interrupted restore first —
	// safe, since we now hold destPath's lock, so nothing else can be
	// legitimately mid-restore against the same temp path concurrently —
	// otherwise copyFile's O_EXCL would wrongly refuse to proceed forever.
	tmp := destPath + ".restoring"
	_ = os.Remove(tmp)
	if err := copyFile(backupPath, tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("dbops: copy %q to %q: %w", backupPath, tmp, err)
	}
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("dbops: replace %q: %w", destPath, err)
	}

	// A prior database at destPath may have left WAL/SHM side-files
	// belonging to ITS content, not the freshly restored one — remove them
	// so the next Open never replays a stale WAL against unrelated data.
	// Best-effort: their absence (the common case, since Backup's VACUUM
	// INTO output has none of its own) is not an error.
	_ = os.Remove(destPath + "-wal")
	_ = os.Remove(destPath + "-shm")
	return nil
}

// validateBackup confirms backupPath opens as a database and passes
// IntegrityCheck, without ever holding it open past this call and without
// risking any mutation of it (see dbx.OpenReadOnly's doc comment).
func validateBackup(ctx context.Context, backupPath string) error {
	src, err := dbx.OpenReadOnly(ctx, backupPath)
	if err != nil {
		return fmt.Errorf("dbops: open backup %q: %w", backupPath, err)
	}
	defer src.Close()

	problems, err := IntegrityCheck(ctx, src)
	if err != nil {
		return fmt.Errorf("dbops: check backup %q: %w", backupPath, err)
	}
	if len(problems) > 0 {
		return fmt.Errorf("dbops: backup %q failed integrity check: %v", backupPath, problems)
	}
	return nil
}

// copyFile copies src to dst, refusing to overwrite an existing dst (the
// caller, Restore, always passes a temp path it owns exclusively).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
