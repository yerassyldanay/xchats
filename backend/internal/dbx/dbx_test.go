package dbx

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return db
}

func TestOpenCloseReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "reopen.db")

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := db.Exec(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (v) VALUES ($1)`, "hello"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopening the same path after a full close must see the persisted data.
	db2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()

	var v string
	if err := db2.QueryRow(ctx, `SELECT v FROM t WHERE id = 1`).Scan(&v); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if v != "hello" {
		t.Errorf("v = %q, want %q", v, "hello")
	}
}

func TestOpenSharesHandleByPath(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "shared.db")

	db1, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	db2, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	// This is the crux of the store/kbstore/mcpauth/responsestore
	// composition design: everyone who opens the same path gets the SAME
	// underlying single-connection *sql.DB, not a second competing one.
	if db1.sdb != db2.sdb {
		t.Fatal("Open with the same path returned two different underlying *sql.DB")
	}

	// A write through db1 must be immediately visible through db2 — they
	// really are the same connection/pool, not just equal-looking configs.
	if _, err := db1.Exec(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db2.Exec(ctx, `INSERT INTO t DEFAULT VALUES`); err != nil {
		t.Fatalf("insert via db2: %v", err)
	}

	// First Close (of 2 refs) must not actually close the shared connection.
	if err := db1.Close(); err != nil {
		t.Fatalf("Close 1: %v", err)
	}
	var n int
	if err := db2.QueryRow(ctx, `SELECT count(*) FROM t`).Scan(&n); err != nil {
		t.Fatalf("query after first close: %v", err)
	}
	if n != 1 {
		t.Errorf("count = %d, want 1", n)
	}

	if err := db2.Close(); err != nil {
		t.Fatalf("Close 2: %v", err)
	}

	// A THIRD Open for the same (now fully closed) path must start fresh,
	// not resurrect the closed *sql.DB.
	db3, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open 3: %v", err)
	}
	defer db3.Close()
	if err := db3.Ping(ctx); err != nil {
		t.Fatalf("ping after reopen: %v", err)
	}
}

func TestSingleProcessLock(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "locked.db")

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// A second, INDEPENDENT dbx instance (simulating a second OS process —
	// same path, but not sharing this test's in-process registry entry)
	// must fail fast rather than silently opening a second connection to
	// the same file. We fake "a second process" by removing the registry
	// entry so Open takes the fresh-open path against an already-locked
	// file.
	registryMu.Lock()
	saved := registry[db.path]
	delete(registry, db.path)
	registryMu.Unlock()
	t.Cleanup(func() {
		registryMu.Lock()
		registry[db.path] = saved
		registryMu.Unlock()
	})

	_, err = Open(ctx, path)
	if err == nil {
		t.Fatal("Open of an already-locked database unexpectedly succeeded")
	}
	if !IsSingleProcessLockErr(err) {
		t.Errorf("Open error = %v, want IsSingleProcessLockErr", err)
	}
}

// TestLockPathMatchesOpensActualLockFile pins LockPath to Open's own,
// unexported lock-file naming: external tooling (internal/dbops) that needs
// to probe or hold the same exclusion Open enforces must compute the exact
// same path Open itself locks, not a close approximation.
func TestLockPathMatchesOpensActualLockFile(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sub", "test.db")

	want, err := LockPath(path)
	if err != nil {
		t.Fatalf("LockPath: %v", err)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("sanity: lock file should not exist before Open, stat err = %v", err)
	}

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(want); err != nil {
		t.Fatalf("LockPath(%q) = %q, but Open did not create a lock file there: %v", path, want, err)
	}
}

func TestLockPathResolvesRelativePaths(t *testing.T) {
	rel := filepath.Join(t.TempDir(), "x.db")
	abs1, err := LockPath(rel)
	if err != nil {
		t.Fatalf("LockPath: %v", err)
	}
	if !filepath.IsAbs(abs1) {
		t.Fatalf("LockPath(%q) = %q, want an absolute path", rel, abs1)
	}
	if abs1 != rel+".lock" {
		t.Fatalf("LockPath(%q) = %q, want %q", rel, abs1, rel+".lock")
	}
}

// TestOpenReadOnlyDoesNotMutateJournalMode pins the exact regression
// OpenReadOnly exists to avoid: connecting to a plain rollback-journal-mode
// file with Open's own DSN (journal_mode=WAL in a _pragma param) silently
// upgrades that file to WAL mode as a side effect of Ping alone, with no
// write ever explicitly requested — verified empirically before this
// function existed. A file OpenReadOnly has inspected must come back out
// byte-for-byte the same journal mode it went in as.
func TestOpenReadOnlyDoesNotMutateJournalMode(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "plain.db")

	// A plain (non-WAL) database, the shape VACUUM INTO produces.
	plain, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := plain.ExecContext(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	var before string
	if err := plain.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&before); err != nil {
		t.Fatalf("journal_mode before: %v", err)
	}
	if err := plain.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if before != "delete" {
		t.Fatalf("sanity: fresh db journal_mode = %q, want delete", before)
	}

	ro, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	if _, err := ro.Query(ctx, `PRAGMA integrity_check`); err != nil {
		t.Fatalf("integrity_check via OpenReadOnly: %v", err)
	}
	if err := ro.Close(); err != nil {
		t.Fatalf("close read-only handle: %v", err)
	}
	if _, err := os.Stat(path + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("a -wal side file appeared after OpenReadOnly: stat err = %v", err)
	}

	after, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer after.Close()
	var got string
	if err := after.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&got); err != nil {
		t.Fatalf("journal_mode after: %v", err)
	}
	if got != "delete" {
		t.Fatalf("journal_mode after OpenReadOnly = %q, want unchanged %q", got, before)
	}
}

func TestOpenReadOnlyFailsForMissingFile(t *testing.T) {
	_, err := OpenReadOnly(context.Background(), filepath.Join(t.TempDir(), "nope.db"))
	if err == nil {
		t.Fatal("OpenReadOnly of a nonexistent file unexpectedly succeeded")
	}
}

func TestOpenReadOnlyFailsForNonSQLiteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage.db")
	if err := os.WriteFile(path, []byte("not a sqlite file"), 0o644); err != nil {
		t.Fatalf("write garbage file: %v", err)
	}
	_, err := OpenReadOnly(context.Background(), path)
	if err == nil {
		t.Fatal("OpenReadOnly of a non-SQLite file unexpectedly succeeded")
	}
}

// TestOpenReadOnlyRejectsWrites is a narrower, in-process confirmation of
// the same read-only guarantee OpenReadOnly's doc comment relies on:
// whatever the destination file's actual content, a write through this
// handle must fail, never silently succeed.
func TestOpenReadOnlyRejectsWrites(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ro-write.db")

	seed, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := seed.Exec(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed handle: %v", err)
	}

	ro, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer ro.Close()
	if _, err := ro.Exec(ctx, `INSERT INTO t (id) VALUES (1)`); err == nil {
		t.Fatal("write through a read-only handle unexpectedly succeeded")
	}
}

func TestPragmasApplied(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	cases := map[string]string{
		"journal_mode": "wal",
		"synchronous":  "1", // NORMAL
		"foreign_keys": "1",
	}
	for pragma, want := range cases {
		var got string
		if err := db.QueryRow(ctx, "PRAGMA "+pragma).Scan(&got); err != nil {
			t.Fatalf("PRAGMA %s: %v", pragma, err)
		}
		if got != want {
			t.Errorf("PRAGMA %s = %q, want %q", pragma, got, want)
		}
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE parent (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `CREATE TABLE child (
		id INTEGER PRIMARY KEY,
		parent_id INTEGER NOT NULL REFERENCES parent(id)
	)`); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(ctx, `INSERT INTO child (id, parent_id) VALUES (1, 999)`)
	if err == nil {
		t.Fatal("insert with a dangling FK unexpectedly succeeded")
	}
}
