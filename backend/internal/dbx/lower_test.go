package dbx

import (
	"context"
	"testing"
)

// SQLite's built-in lower() folds only ASCII, so Open's connections carry a Go
// replacement registered under the same name. Repository SQL then says plain
// lower(x) on both engines (PostgreSQL's own lower() folds Unicode on a UTF-8
// database) and a search for "али" finds a stored "Алия".
func TestLowerFoldsNonASCII(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	for _, tc := range []struct{ in, want string }{
		{"АЛИЯ", "алия"},
		{"Алия", "алия"},
		{"ABC", "abc"},
		{"ÜNÏCODE", "ünïcode"},
		{"already lower", "already lower"},
		{"", ""},
	} {
		var got string
		if err := db.QueryRow(ctx, `SELECT lower($1)`, tc.in).Scan(&got); err != nil {
			t.Fatalf("lower(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("lower(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLowerKeepsNullAndStringifiesNumbers(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	var isNull bool
	if err := db.QueryRow(ctx, `SELECT lower(NULL) IS NULL`).Scan(&isNull); err != nil || !isNull {
		t.Fatalf("lower(NULL) IS NULL = %v (%v), want true", isNull, err)
	}
	var num string
	if err := db.QueryRow(ctx, `SELECT lower(123)`).Scan(&num); err != nil || num != "123" {
		t.Fatalf("lower(123) = %q (%v), want \"123\"", num, err)
	}
}

// The search shape the CRM, inbox and template lists use: both sides lowered,
// pattern lowered in Go, LIKE on the result.
func TestLowerSupportsCaseInsensitiveLike(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE people (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"Алия", "Асем", "Alice"} {
		if _, err := db.Exec(ctx, `INSERT INTO people (id, name) VALUES ($1, $2)`, i+1, name); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM people WHERE lower(name) LIKE $1`, "%али%").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("matched %d rows for %%али%%, want 1 (Алия)", n)
	}
}

// unicode_lower was the pre-portable spelling; nothing may rely on it.
func TestUnicodeLowerIsRemoved(t *testing.T) {
	db := openTest(t)
	var s string
	if err := db.QueryRow(context.Background(), `SELECT unicode_lower('A')`).Scan(&s); err == nil {
		t.Fatalf("unicode_lower still exists and returned %q; repository SQL must use plain lower()", s)
	}
}
