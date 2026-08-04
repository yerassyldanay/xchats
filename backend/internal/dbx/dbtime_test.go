package dbx

import (
	"context"
	"sort"
	"testing"
	"time"
)

// TestTimeRoundTrip pins the canonical timestamp contract before any
// package is ported onto it: a time.Time bound as a query arg is stored as
// TEXT in TimeLayout, and scanning it back — into both a plain time.Time
// (NOT NULL columns) and a *time.Time (nullable columns, including the NULL
// case) — reproduces the same instant.
func TestTimeRoundTrip(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (
		id INTEGER PRIMARY KEY,
		created_at TEXT NOT NULL,
		deleted_at TEXT
	)`); err != nil {
		t.Fatal(err)
	}

	// Sub-millisecond precision must not survive the round trip (canonical
	// format is fixed 3-digit millis) but millisecond precision must — so
	// the input is pre-truncated to milliseconds, matching what every
	// Equal() check below expects back.
	in := time.Date(2026, 8, 4, 11, 23, 31, 123_456_789, time.FixedZone("MSK", 3*3600)).
		Truncate(time.Millisecond)

	if _, err := db.Exec(ctx, `INSERT INTO t (id, created_at, deleted_at) VALUES (1, $1, $2)`,
		in, (*time.Time)(nil)); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (id, created_at, deleted_at) VALUES (2, $1, $2)`,
		in, &in); err != nil {
		t.Fatalf("insert with non-nil deleted_at: %v", err)
	}

	var raw string
	if err := db.QueryRow(ctx, `SELECT created_at FROM t WHERE id = 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	wantRaw := in.UTC().Format(TimeLayout)
	if raw != wantRaw {
		t.Fatalf("stored text = %q, want %q", raw, wantRaw)
	}
	if got := len(raw) - len(raw[:19]); got != 4 { // ".mmm" == 4 bytes after the seconds field
		t.Fatalf("stored text %q does not have a fixed 3-digit millisecond suffix", raw)
	}

	var createdAt time.Time
	var deletedAt *time.Time
	if err := db.QueryRow(ctx, `SELECT created_at, deleted_at FROM t WHERE id = 1`).
		Scan(&createdAt, &deletedAt); err != nil {
		t.Fatalf("scan row 1: %v", err)
	}
	if !createdAt.Equal(in) {
		t.Errorf("row 1 created_at = %v, want %v", createdAt, in)
	}
	if createdAt.Location() != time.UTC {
		t.Errorf("row 1 created_at location = %v, want UTC", createdAt.Location())
	}
	if deletedAt != nil {
		t.Errorf("row 1 deleted_at = %v, want nil (SQL NULL)", deletedAt)
	}

	if err := db.QueryRow(ctx, `SELECT created_at, deleted_at FROM t WHERE id = 2`).
		Scan(&createdAt, &deletedAt); err != nil {
		t.Fatalf("scan row 2: %v", err)
	}
	if deletedAt == nil || !deletedAt.Equal(in) {
		t.Errorf("row 2 deleted_at = %v, want %v", deletedAt, in)
	}
}

// TestTimeLexicalOrdering pins that two canonical timestamps compare in the
// same order lexically (as SQLite TEXT, via <, >, ORDER BY, MIN/MAX) as they
// do chronologically — the whole reason the format is fixed-width.
func TestTimeLexicalOrdering(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var times []time.Time
	for i := 0; i < 20; i++ {
		times = append(times, base.Add(time.Duration(i)*137*time.Millisecond))
	}
	// Also cross a second boundary and a day boundary explicitly.
	times = append(times,
		base.Add(999*time.Millisecond),
		base.Add(1000*time.Millisecond),
		base.Add(24*time.Hour).Add(-1*time.Millisecond),
		base.Add(24*time.Hour),
	)

	if _, err := db.Exec(ctx, `CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatal(err)
	}
	// Insert in shuffled (reverse) order so ORDER BY is doing real work.
	for i := len(times) - 1; i >= 0; i-- {
		if _, err := db.Exec(ctx, `INSERT INTO t (v) VALUES ($1)`, times[i]); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := db.Query(ctx, `SELECT v FROM t ORDER BY v`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []time.Time
	for rows.Next() {
		var v time.Time
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		got = append(got, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	want := append([]time.Time(nil), times...)
	sort.Slice(want, func(i, j int) bool { return want[i].Before(want[j]) })

	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Errorf("position %d: ORDER BY v gave %v, want %v", i, got[i], want[i])
		}
	}
}

// TestTimeComparableWithStrftimeNow pins that a Go-formatted canonical
// timestamp and strftime('%Y-%m-%d %H:%M:%f','now') (the now() ->
// translation every SQL-side default/comparison uses) are on the exact same
// scale: a strftime('now') value inserted after a Go-bound "yesterday"
// value must sort after it, and vice versa for "tomorrow".
func TestTimeComparableWithStrftimeNow(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (
		id INTEGER PRIMARY KEY,
		v TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
	)`); err != nil {
		t.Fatal(err)
	}

	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	tomorrow := time.Now().UTC().Add(24 * time.Hour)

	if _, err := db.Exec(ctx, `INSERT INTO t (id, v) VALUES (1, $1)`, yesterday); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (id) VALUES (2)`); err != nil { // SQL-side default (now)
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (id, v) VALUES (3, $1)`, tomorrow); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query(ctx, `SELECT id FROM t ORDER BY v`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var order []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		order = append(order, id)
	}
	if want := []int{1, 2, 3}; !equalInts(order, want) {
		t.Errorf("ORDER BY v gave id order %v, want %v (yesterday < now < tomorrow)", order, want)
	}

	// The stored strftime('now') text must itself parse with ParseTime and
	// itself be exactly the fixed-width canonical form (dbtest/domain code
	// never reads this column via strftime output directly today, but a
	// future column default might, and the contract test asserts formats
	// match across every default, not just the ones currently read back).
	var nowText string
	if err := db.QueryRow(ctx, `SELECT v FROM t WHERE id = 2`).Scan(&nowText); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseTime(nowText); err != nil {
		t.Errorf("strftime('now') text %q does not parse as TimeLayout: %v", nowText, err)
	}
	if len(nowText) != len(TimeLayout) {
		t.Errorf("strftime('now') text %q has length %d, want %d (same width as TimeLayout)",
			nowText, len(nowText), len(TimeLayout))
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFormatParseTimeRoundTrip(t *testing.T) {
	cases := []time.Time{
		time.Date(2026, 8, 4, 11, 23, 31, 0, time.UTC),
		time.Date(2026, 8, 4, 11, 23, 31, 1_000_000, time.UTC),
		time.Date(2026, 8, 4, 11, 23, 31, 999_000_000, time.UTC),
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Unix(0, 0).UTC(),
	}
	for _, tc := range cases {
		s := FormatTime(tc)
		got, err := ParseTime(s)
		if err != nil {
			t.Fatalf("ParseTime(%q): %v", s, err)
		}
		if !got.Equal(tc) {
			t.Errorf("round trip %v -> %q -> %v, want %v", tc, s, got, tc)
		}
	}
}
