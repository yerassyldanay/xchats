package dbx

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestUUIDArrayRoundTrip(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (
		id INTEGER PRIMARY KEY,
		images TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(images))
	)`); err != nil {
		t.Fatal(err)
	}

	ids := UUIDArray{uuid.New(), uuid.New(), uuid.New()}
	if _, err := db.Exec(ctx, `INSERT INTO t (id, images) VALUES (1, $1)`, ids); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (id) VALUES (2)`); err != nil { // exercise the schema default
		t.Fatalf("insert with default: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (id, images) VALUES (3, $1)`, UUIDArray(nil)); err != nil {
		t.Fatalf("insert nil array: %v", err)
	}

	var got UUIDArray
	if err := db.QueryRow(ctx, `SELECT images FROM t WHERE id = 1`).Scan(&got); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != len(ids) {
		t.Fatalf("got %d ids, want %d", len(got), len(ids))
	}
	for i := range ids {
		if got[i] != ids[i] {
			t.Errorf("id[%d] = %s, want %s", i, got[i], ids[i])
		}
	}

	var deflt UUIDArray
	if err := db.QueryRow(ctx, `SELECT images FROM t WHERE id = 2`).Scan(&deflt); err != nil {
		t.Fatalf("scan default: %v", err)
	}
	if len(deflt) != 0 {
		t.Errorf("default images = %v, want empty", deflt)
	}

	var nilRow UUIDArray
	if err := db.QueryRow(ctx, `SELECT images FROM t WHERE id = 3`).Scan(&nilRow); err != nil {
		t.Fatalf("scan nil-inserted row: %v", err)
	}
	if len(nilRow) != 0 {
		t.Errorf("nil-inserted images = %v, want empty (never SQL NULL)", nilRow)
	}
}

func TestStringArrayRoundTrip(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (
		id INTEGER PRIMARY KEY,
		uris TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(uris))
	)`); err != nil {
		t.Fatal(err)
	}

	uris := StringArray{"https://a.example/cb", "https://b.example/cb"}
	if _, err := db.Exec(ctx, `INSERT INTO t (id, uris) VALUES (1, $1)`, uris); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got StringArray
	if err := db.QueryRow(ctx, `SELECT uris FROM t WHERE id = 1`).Scan(&got); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != 2 || got[0] != uris[0] || got[1] != uris[1] {
		t.Errorf("got %v, want %v", got, uris)
	}
}

// TestArrayColumnInAnyTranslation pins the `= ANY($n)` -> `IN (SELECT value
// FROM json_each(?))` translation from the per-PG-ism table.
func TestArrayColumnInAnyTranslation(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"alice", "bob", "carol"} {
		if _, err := db.Exec(ctx, `INSERT INTO t (id, name) VALUES ($1, $2)`, i+1, name); err != nil {
			t.Fatal(err)
		}
	}

	names := StringArray{"alice", "carol", "nobody"}
	rows, err := db.Query(ctx,
		`SELECT name FROM t WHERE name IN (SELECT value FROM json_each($1)) ORDER BY name`, names)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		got = append(got, n)
	}
	want := []string{"alice", "carol"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %v, want %v", got, want)
	}
}
