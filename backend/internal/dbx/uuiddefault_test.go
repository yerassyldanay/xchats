package dbx

import (
	"context"
	"regexp"
	"testing"

	"github.com/google/uuid"
)

// The SQLite UUIDv4 column default is sqliteUUIDv4Expr (render.go), what the
// {{uuid}} migration macro expands to on SQLite. These tests pin its
// correctness: a valid UUIDv4 (version nibble '4', variant nibble one of
// 8/9/a/b) and no duplicates, before it is trusted in DDL.

var uuidV4RE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUIDv4DefaultExpression(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (id TEXT PRIMARY KEY DEFAULT `+sqliteUUIDv4Expr+`)`); err != nil {
		t.Fatalf("create table with UUIDv4 default: %v", err)
	}

	const n = 500
	for i := 0; i < n; i++ {
		if _, err := db.Exec(ctx, `INSERT INTO t DEFAULT VALUES`); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	rows, err := db.Query(ctx, `SELECT id FROM t`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	seen := make(map[string]bool, n)
	count := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		count++
		if !uuidV4RE.MatchString(id) {
			t.Fatalf("generated id %q is not a well-formed UUIDv4", id)
		}
		if _, err := uuid.Parse(id); err != nil {
			t.Fatalf("generated id %q does not parse as a UUID: %v", id, err)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q generated across %d rows", id, n)
		}
		seen[id] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != n {
		t.Fatalf("got %d rows, want %d", count, n)
	}
}

// TestUUIDv4DefaultOmittedOnInsert pins the call-site pattern both engines
// share: an INSERT that binds every column except id, then RETURNING id (on
// PostgreSQL the same default is gen_random_uuid()::text).
func TestUUIDv4DefaultOmittedOnInsert(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE orgs (
		id   TEXT PRIMARY KEY DEFAULT `+sqliteUUIDv4Expr+`,
		name TEXT NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}

	var id string
	err := db.QueryRow(ctx, `INSERT INTO orgs (name) VALUES ($1) RETURNING id`, "acme").Scan(&id)
	if err != nil {
		t.Fatalf("insert omitting id: %v", err)
	}
	if _, err := uuid.Parse(id); err != nil {
		t.Fatalf("returned id %q does not parse as a UUID: %v", id, err)
	}
}
