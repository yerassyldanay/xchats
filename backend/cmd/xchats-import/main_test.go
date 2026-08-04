package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

func TestRun_MissingFlagsPrintsUsageAndReturns2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected a usage message on stderr for missing required flags")
	}
}

func TestRun_MissingSqlitePathReturns2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-postgres-url", "postgres://x/y"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRun_UnreachablePostgresReturns1(t *testing.T) {
	// A syntactically valid but unreachable Postgres URL: dbx.Open must
	// succeed (a fresh SQLite path) and migrate cleanly, then pgimport.Import
	// itself must fail to connect — the failure this test pins is at the
	// Postgres-connect step, not an earlier one.
	var stdout, stderr bytes.Buffer
	path := filepath.Join(t.TempDir(), "x.db")
	code := run([]string{
		"-postgres-url", "postgres://nobody:nothing@127.0.0.1:1/doesnotexist?connect_timeout=1",
		"-sqlite-path", path,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (unreachable postgres)", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("expected an error message on stderr")
	}
}

// TestRun_RejectsSqlitePathAlreadyOpenByThisProcess simulates "the server
// is already running against this path": since this test runs IN this
// same process, dbx.Open's path-based sharing (see its package doc) would
// normally hand back the SAME shared handle rather than hitting the
// single-process lock — so this test holds the exact lock file dbx.Open
// itself takes (dbx.LockPath), the same technique internal/dbops' own
// equivalent test uses, to force the real cross-process code path rather
// than the same-process refcounted one.
func TestRun_RejectsSqlitePathAlreadyOpenByThisProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	lockPath, err := dbx.LockPath(path)
	if err != nil {
		t.Fatalf("LockPath: %v", err)
	}
	holder := flock.New(lockPath)
	locked, err := holder.TryLock()
	if err != nil || !locked {
		t.Fatalf("test setup: could not acquire simulated server lock: locked=%v err=%v", locked, err)
	}
	defer holder.Unlock()

	var stdout, stderr bytes.Buffer
	code := run([]string{"-postgres-url", "postgres://x/y", "-sqlite-path", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (sqlite path already open)", code)
	}
}
