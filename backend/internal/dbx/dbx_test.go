package dbx

import (
	"context"
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
