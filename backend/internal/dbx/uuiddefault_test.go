package dbx

import (
	"context"
	"regexp"
	"testing"

	"github.com/google/uuid"
)

// uuidV4DefaultExpr is the SQLite column-default expression the sqlite
// migrations use everywhere a Postgres uuid_generate_v4() default stood, so
// "INSERT ... RETURNING id" sites that omit id stay untouched (see the
// per-PG-ism translation table). It must stay byte-for-byte identical to
// the one pasted into every migrations/sqlite/*.up.sql file — this test is
// what pins the expression's correctness (valid UUIDv4: version nibble '4',
// variant nibble one of 8/9/a/b) before it's trusted in DDL.
const uuidV4DefaultExpr = `(lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6))))`

var uuidV4RE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUIDv4DefaultExpression(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (id TEXT PRIMARY KEY DEFAULT `+uuidV4DefaultExpr+`)`); err != nil {
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

// TestUUIDv4DefaultOmittedOnInsert pins the exact call-site pattern the
// translation table promises: an INSERT that binds every column except id,
// then RETURNING id, works unchanged (matching Postgres's
// "id uuid PRIMARY KEY DEFAULT uuid_generate_v4()" + "RETURNING id" shape).
func TestUUIDv4DefaultOmittedOnInsert(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE orgs (
		id   TEXT PRIMARY KEY DEFAULT `+uuidV4DefaultExpr+`,
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
