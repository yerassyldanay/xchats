package dbx

import (
	"context"
	"testing"
)

// TestDollarPlaceholders pins the Phase-1 decision the plan calls out
// explicitly: modernc.org/sqlite matches $N placeholders by NUMBER against
// the Go args slice's ordinal position (see modernc.org/sqlite's conn.go
// bind(), which special-cases the '$'/'?' NNN forms specifically so pgx-
// style "$1, $2, ..." queries work without sql.Named) — so all ~270 ported
// call sites' SQL strings can keep their pgx-era "$1, $2, ..." verbatim. No
// mechanical $N -> ?N rewrite is needed.
func TestDollarPlaceholders(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (a TEXT, b TEXT, c TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (a, b, c) VALUES ($1, $2, $3)`, "A", "B", "C"); err != nil {
		t.Fatalf("insert with $N placeholders: %v", err)
	}

	var a, b, c string
	if err := db.QueryRow(ctx, `SELECT a, b, c FROM t WHERE a = $1`, "A").Scan(&a, &b, &c); err != nil {
		t.Fatalf("select with $N: %v", err)
	}
	if a != "A" || b != "B" || c != "C" {
		t.Errorf("got (%q, %q, %q), want (A, B, C)", a, b, c)
	}
}

// TestDollarPlaceholderReuse confirms the SAME $N appearing more than once
// in one statement binds to the SAME arg every time (Postgres/pgx
// semantics) — a pattern this codebase's upsertConfigRow-style
// "COALESCE($2, ...)" queries rely on, reusing $2..$6 once per column.
func TestDollarPlaceholderReuse(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (a TEXT, b TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (a, b) VALUES ($1, $1 || '-' || $1)`, "x"); err != nil {
		t.Fatalf("insert reusing $1 twice: %v", err)
	}
	var a, b string
	if err := db.QueryRow(ctx, `SELECT a, b FROM t`).Scan(&a, &b); err != nil {
		t.Fatal(err)
	}
	if a != "x" || b != "x-x" {
		t.Errorf("got (a=%q, b=%q), want (a=%q, b=%q)", a, b, "x", "x-x")
	}
}

// TestReturningThroughQueryRow confirms "INSERT ... RETURNING" reads
// cleanly through QueryRow — the pgx-parity pattern every scanInserted-style
// upsert in store/kbstore depends on.
func TestReturningThroughQueryRow(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatal(err)
	}
	var id int64
	err := db.QueryRow(ctx, `INSERT INTO t (v) VALUES ($1) RETURNING id`, "hi").Scan(&id)
	if err != nil {
		t.Fatalf("insert ... returning: %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}
}
