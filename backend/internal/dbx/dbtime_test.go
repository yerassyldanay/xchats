package dbx

import (
	"context"
	"strings"
	"testing"
	"time"
)

// 2026-10-06T00:00:00Z as an independently known literal (the instant the
// migration seeds use), so these tests do not just re-derive the
// implementation's own arithmetic.
const seedInstantMS = int64(1791244800000)

var seedInstant = time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)

// TestTimeRoundTrip pins the timestamp contract: a time.Time bound as a query
// arg is stored as a BIGINT of UTC Unix milliseconds, and scanning it back, into
// both a plain time.Time (NOT NULL columns) and a *time.Time (nullable columns,
// NULL included), reproduces the same instant in UTC.
func TestTimeRoundTrip(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, `CREATE TABLE t (
		id INTEGER PRIMARY KEY,
		created_at BIGINT NOT NULL,
		deleted_at BIGINT
	)`); err != nil {
		t.Fatal(err)
	}

	// 2026-10-06T00:00:00.123456789Z written in another zone; the sub-millisecond
	// part must not survive the round trip, the millisecond part must.
	in := time.Date(2026, 10, 6, 3, 0, 0, 123_456_789, time.FixedZone("MSK", 3*3600))
	want := seedInstant.Add(123 * time.Millisecond)

	if _, err := db.Exec(ctx, `INSERT INTO t (id, created_at, deleted_at) VALUES (1, $1, $2)`,
		in, (*time.Time)(nil)); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (id, created_at, deleted_at) VALUES (2, $1, $2)`,
		in, &in); err != nil {
		t.Fatalf("insert with non-nil deleted_at: %v", err)
	}

	var raw int64
	if err := db.QueryRow(ctx, `SELECT created_at FROM t WHERE id = 1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != seedInstantMS+123 {
		t.Fatalf("stored value = %d, want %d (UTC Unix milliseconds)", raw, seedInstantMS+123)
	}

	var createdAt time.Time
	var deletedAt *time.Time
	if err := db.QueryRow(ctx, `SELECT created_at, deleted_at FROM t WHERE id = 1`).
		Scan(&createdAt, &deletedAt); err != nil {
		t.Fatalf("scan row 1: %v", err)
	}
	if !createdAt.Equal(want) {
		t.Errorf("row 1 created_at = %v, want %v", createdAt, want)
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
	if deletedAt == nil || !deletedAt.Equal(want) {
		t.Errorf("row 2 deleted_at = %v, want %v", deletedAt, want)
	}
}

// Integers order the way instants do: ORDER BY, MIN/MAX and < / > on a bound
// time.Time all agree with chronology, with no fixed-width-text requirement.
func TestTimeNumericOrdering(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	base := seedInstant
	var times []time.Time
	for i := 0; i < 20; i++ {
		times = append(times, base.Add(time.Duration(i)*137*time.Millisecond))
	}
	times = append(times,
		base.Add(999*time.Millisecond),
		base.Add(1000*time.Millisecond),
		base.Add(24*time.Hour).Add(-1*time.Millisecond),
		base.Add(24*time.Hour),
	)

	if _, err := db.Exec(ctx, `CREATE TABLE t (v BIGINT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for i := len(times) - 1; i >= 0; i-- { // reverse order, so ORDER BY does real work
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
	if len(got) != len(times) {
		t.Fatalf("got %d rows, want %d", len(got), len(times))
	}
	for i := 1; i < len(got); i++ {
		if !got[i-1].Before(got[i]) {
			t.Errorf("position %d: %v is not before %v", i, got[i-1], got[i])
		}
	}

	var n int
	cutoff := base.Add(24 * time.Hour)
	if err := db.QueryRow(ctx, `SELECT count(*) FROM t WHERE v < $1`, cutoff).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != len(times)-1 {
		t.Errorf("v < cutoff matched %d rows, want %d", n, len(times)-1)
	}

	var latest time.Time
	if err := db.QueryRow(ctx, `SELECT MAX(v) FROM t`).Scan(&latest); err != nil || !latest.Equal(cutoff) {
		t.Errorf("MAX(v) = %v (%v), want %v", latest, err, cutoff)
	}
}

func TestTimeScanNullIntoNonPointerFails(t *testing.T) {
	db := openTest(t)
	var at time.Time
	err := db.QueryRow(context.Background(), `SELECT NULL`).Scan(&at)
	if err == nil || !strings.Contains(err.Error(), "NULL") {
		t.Fatalf("scanning NULL into a time.Time returned %v, want an error naming NULL", err)
	}
}

// bindArgs has no dialect switch: the one conversion serves SQLite and
// PostgreSQL alike, so BIGINT timestamp columns mean the same on both.
func TestBindArgsConvertsTimesToUnixMillis(t *testing.T) {
	in := seedInstant.Add(5 * time.Millisecond)
	var absent *time.Time

	got := bindArgs([]any{in, &in, absent, "x", 7})
	if len(got) != 5 {
		t.Fatalf("got %d args, want 5", len(got))
	}
	if got[0] != seedInstantMS+5 || got[1] != seedInstantMS+5 {
		t.Errorf("times converted to %v and %v, want %d", got[0], got[1], seedInstantMS+5)
	}
	if got[2] != nil {
		t.Errorf("nil *time.Time converted to %v, want nil (SQL NULL)", got[2])
	}
	if got[3] != "x" || got[4] != 7 {
		t.Errorf("non-time args changed: %v, %v", got[3], got[4])
	}

	plain := []any{"a", 1}
	if again := bindArgs(plain); &again[0] != &plain[0] {
		t.Error("bindArgs copied an arg list that holds no time values")
	}
}
