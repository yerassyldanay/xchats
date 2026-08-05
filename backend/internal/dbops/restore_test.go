package dbops_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"

	"github.com/yerassyldanay/xchats/backend/internal/dbops"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

func TestRestore_RoundTrip(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := dbops.Backup(ctx, db, backupPath); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// Restore into a brand-new location — the common "provision a fresh
	// environment from a backup" case.
	destPath := filepath.Join(t.TempDir(), "restored.db")
	if err := dbops.Restore(ctx, backupPath, destPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	restored, err := dbx.Open(ctx, destPath)
	if err != nil {
		t.Fatalf("open restored db: %v", err)
	}
	defer restored.Close()
	var count int
	if err := restored.QueryRow(ctx, `SELECT count(*) FROM widgets`).Scan(&count); err != nil {
		t.Fatalf("count restored rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("restored row count = %d, want 2", count)
	}
}

func TestRestore_ReplacesExistingDestination(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := dbops.Backup(ctx, db, backupPath); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// destPath already has DIFFERENT content — Restore must fully replace
	// it, not merge or append.
	destPath := filepath.Join(t.TempDir(), "existing.db")
	pre, err := dbx.Open(ctx, destPath)
	if err != nil {
		t.Fatalf("open pre-existing dest: %v", err)
	}
	if _, err := pre.Exec(ctx, `CREATE TABLE unrelated (x INTEGER)`); err != nil {
		t.Fatalf("seed pre-existing dest: %v", err)
	}
	if err := pre.Close(); err != nil {
		t.Fatalf("close pre-existing dest: %v", err)
	}

	if err := dbops.Restore(ctx, backupPath, destPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	restored, err := dbx.Open(ctx, destPath)
	if err != nil {
		t.Fatalf("open restored db: %v", err)
	}
	defer restored.Close()
	var count int
	if err := restored.QueryRow(ctx, `SELECT count(*) FROM widgets`).Scan(&count); err != nil {
		t.Fatalf("restored db does not have the backup's widgets table: %v", err)
	}
	if count != 2 {
		t.Fatalf("restored row count = %d, want 2", count)
	}
	if err := restored.QueryRow(ctx, `SELECT count(*) FROM unrelated`).Scan(&count); err == nil {
		t.Fatalf("restored db still has the old 'unrelated' table (count=%d) — Restore did not fully replace it", count)
	}
}

func TestRestore_CreatesDestinationDirectory(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := dbops.Backup(ctx, db, backupPath); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	destPath := filepath.Join(t.TempDir(), "nested", "dir", "restored.db")
	if err := dbops.Restore(ctx, backupPath, destPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("restored file missing: %v", err)
	}
}

// TestRestore_SucceedsDespiteLeftoverTempFileFromACrashedRestore guards
// against a stale destPath+".restoring" file (left behind by a process
// that died mid-copy on a PRIOR restore attempt) permanently blocking every
// future restore.
func TestRestore_SucceedsDespiteLeftoverTempFileFromACrashedRestore(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := dbops.Backup(ctx, db, backupPath); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	destPath := filepath.Join(t.TempDir(), "restored.db")
	if err := os.WriteFile(destPath+".restoring", []byte("leftover from a crashed restore"), 0o644); err != nil {
		t.Fatalf("seed leftover temp file: %v", err)
	}

	if err := dbops.Restore(ctx, backupPath, destPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	restored, err := dbx.Open(ctx, destPath)
	if err != nil {
		t.Fatalf("open restored db: %v", err)
	}
	defer restored.Close()
	var count int
	if err := restored.QueryRow(ctx, `SELECT count(*) FROM widgets`).Scan(&count); err != nil {
		t.Fatalf("count restored rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("restored row count = %d, want 2", count)
	}
}

func TestRestore_RemovesStaleWALSidecarsAtDestination(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := dbops.Backup(ctx, db, backupPath); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	destPath := filepath.Join(t.TempDir(), "restored.db")
	if err := os.WriteFile(destPath, []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("seed placeholder dest: %v", err)
	}
	if err := os.WriteFile(destPath+"-wal", []byte("stale wal"), 0o644); err != nil {
		t.Fatalf("seed stale -wal: %v", err)
	}
	if err := os.WriteFile(destPath+"-shm", []byte("stale shm"), 0o644); err != nil {
		t.Fatalf("seed stale -shm: %v", err)
	}

	if err := dbops.Restore(ctx, backupPath, destPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(destPath + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("stale -wal survived restore: stat err = %v", err)
	}
	if _, err := os.Stat(destPath + "-shm"); !os.IsNotExist(err) {
		t.Fatalf("stale -shm survived restore: stat err = %v", err)
	}
}

func TestRestore_RejectsMissingBackup(t *testing.T) {
	ctx := context.Background()
	err := dbops.Restore(ctx, filepath.Join(t.TempDir(), "nope.db"), filepath.Join(t.TempDir(), "dest.db"))
	if err == nil {
		t.Fatal("Restore from a nonexistent backup unexpectedly succeeded")
	}
}

func TestRestore_RejectsNonSQLiteBackup(t *testing.T) {
	ctx := context.Background()
	backupPath := filepath.Join(t.TempDir(), "garbage.db")
	if err := os.WriteFile(backupPath, []byte("not a sqlite file"), 0o644); err != nil {
		t.Fatalf("write garbage backup: %v", err)
	}
	err := dbops.Restore(ctx, backupPath, filepath.Join(t.TempDir(), "dest.db"))
	if err == nil {
		t.Fatal("Restore from a non-SQLite backup unexpectedly succeeded")
	}
}

func TestRestore_RejectsCorruptBackupWithoutTouchingDestination(t *testing.T) {
	ctx := context.Background()
	db, path := openSeeded(t, "corrupt-source.db")
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if err := os.Truncate(path, info.Size()/2); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// destPath pre-exists with real content that must survive the rejected
	// restore attempt untouched.
	destPath := filepath.Join(t.TempDir(), "dest.db")
	pre, err := dbx.Open(ctx, destPath)
	if err != nil {
		t.Fatalf("open dest: %v", err)
	}
	if _, err := pre.Exec(ctx, `CREATE TABLE keep (x INTEGER)`); err != nil {
		t.Fatalf("seed dest: %v", err)
	}
	if err := pre.Close(); err != nil {
		t.Fatalf("close dest: %v", err)
	}

	if err := dbops.Restore(ctx, path, destPath); err == nil {
		t.Fatal("Restore from a corrupt (truncated) backup unexpectedly succeeded")
	}

	still, err := dbx.Open(ctx, destPath)
	if err != nil {
		t.Fatalf("reopen dest after rejected restore: %v", err)
	}
	defer still.Close()
	var count int
	if err := still.QueryRow(ctx, `SELECT count(*) FROM keep`).Scan(&count); err != nil {
		t.Fatalf("destination was altered by the rejected restore: %v", err)
	}
}

// TestRestore_RejectsWhenDestinationIsLocked simulates "the server is
// running against destPath": holding the SAME lock file dbx.Open would
// (dbx.LockPath), from an independent *flock.Flock instance — gofrs/flock's
// TryLock genuinely contends across independent instances on the same
// file even within one process (verified empirically; this is not simply
// asserting our own mock's behavior), so this is a faithful stand-in for a
// second OS process already holding the database open.
func TestRestore_RejectsWhenDestinationIsLocked(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := dbops.Backup(ctx, db, backupPath); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	destPath := filepath.Join(t.TempDir(), "in-use.db")
	pre, err := dbx.Open(ctx, destPath)
	if err != nil {
		t.Fatalf("open dest: %v", err)
	}
	if _, err := pre.Exec(ctx, `CREATE TABLE keep (x INTEGER)`); err != nil {
		t.Fatalf("seed dest: %v", err)
	}
	if err := pre.Close(); err != nil {
		t.Fatalf("close dest (releases the real dbx lock before we take our own probe lock): %v", err)
	}

	lockPath, err := dbx.LockPath(destPath)
	if err != nil {
		t.Fatalf("LockPath: %v", err)
	}
	holder := flock.New(lockPath)
	locked, err := holder.TryLock()
	if err != nil || !locked {
		t.Fatalf("test setup: could not acquire simulated server lock: locked=%v err=%v", locked, err)
	}

	if err := dbops.Restore(ctx, backupPath, destPath); err == nil {
		t.Fatal("Restore against a locked (in-use) destination unexpectedly succeeded")
	}

	// The destination's actual content must be untouched by the rejected
	// attempt — release our simulated-server lock first, since dbx.Open
	// would otherwise see it as still in use too.
	if err := holder.Unlock(); err != nil {
		t.Fatalf("release simulated server lock: %v", err)
	}
	still, err := dbx.Open(ctx, destPath)
	if err != nil {
		t.Fatalf("reopen dest after rejected restore: %v", err)
	}
	defer still.Close()
	var count int
	if err := still.QueryRow(ctx, `SELECT count(*) FROM keep`).Scan(&count); err != nil {
		t.Fatalf("destination was altered despite being locked: %v", err)
	}
}
