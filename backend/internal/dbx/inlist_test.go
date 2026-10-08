package dbx

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestInListNumbersAfterBoundArgs(t *testing.T) {
	list, args := InList([]any{"org", 42}, []string{"x", "y", "z"})
	if want := "($3, $4, $5)"; list != want {
		t.Errorf("list = %q, want %q", list, want)
	}
	if want := []any{"org", 42, "x", "y", "z"}; !reflect.DeepEqual(args, want) {
		t.Errorf("args = %v, want %v", args, want)
	}

	list, args = InList(nil, []int{7})
	if list != "($1)" || !reflect.DeepEqual(args, []any{7}) {
		t.Errorf("single value without prior args = %q, %v", list, args)
	}
}

// The empty list is (NULL): `x IN (NULL)` is never true, so a caller that forgot
// to return early still matches nothing instead of producing the invalid `IN ()`.
func TestInListEmptyMatchesNothing(t *testing.T) {
	list, args := InList([]any{"keep"}, []string{})
	if list != "(NULL)" {
		t.Errorf("empty list = %q, want (NULL)", list)
	}
	if !reflect.DeepEqual(args, []any{"keep"}) {
		t.Errorf("args = %v, want the caller's args unchanged", args)
	}

	db := openTest(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `CREATE TABLE t (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (id) VALUES ('a')`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM t WHERE id IN `+list, args[1:]...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("IN (NULL) matched %d rows, want 0", n)
	}
}

func TestInListDoesNotMutateCallerArgs(t *testing.T) {
	backing := make([]any, 1, 10) // spare capacity a naive append would write into
	backing[0] = "first"
	_, args := InList(backing, []string{"a", "b"})
	if len(backing) != 1 || backing[0] != "first" {
		t.Errorf("caller's slice changed: %v", backing)
	}
	if len(args) != 3 {
		t.Fatalf("args = %v, want 3 values", args)
	}
	if &args[0] == &backing[0] {
		t.Error("returned args share the caller's backing array")
	}
}

func TestInListBindsUUIDsAndStrings(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `CREATE TABLE t (id TEXT PRIMARY KEY, org TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, id := range ids {
		if _, err := db.Exec(ctx, `INSERT INTO t (id, org) VALUES ($1, 'o1')`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, `INSERT INTO t (id, org) VALUES ('other', 'o2')`); err != nil {
		t.Fatal(err)
	}

	list, args := InList([]any{"o1"}, ids[:2])
	rows, err := db.Query(ctx, `SELECT id FROM t WHERE org = $1 AND id IN `+list+` ORDER BY id`, args...)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		got = append(got, id)
	}
	want := []string{ids[0].String(), ids[1].String()}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// The batching idiom for unbounded input: one statement per MaxInList chunk.
func TestInListChunksOverMaxInList(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `CREATE TABLE t (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := 0; i < 2*MaxInList+37; i++ {
		id := fmt.Sprintf("id-%04d", i)
		ids = append(ids, id)
		if _, err := db.Exec(ctx, `INSERT INTO t (id) VALUES ($1)`, id); err != nil {
			t.Fatal(err)
		}
	}

	total := 0
	for chunk := range slices.Chunk(ids, MaxInList) {
		list, args := InList(nil, chunk)
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM t WHERE id IN `+list, args...).Scan(&n); err != nil {
			t.Fatalf("chunk of %d: %v", len(chunk), err)
		}
		total += n
	}
	if total != len(ids) {
		t.Errorf("chunks matched %d rows, want %d", total, len(ids))
	}
}

// QueryInChunks is the read-side of the chunking idiom: one statement per
// MaxInList-sized chunk, each chunk's rows closed before the next statement runs
// (SQLite's single connection would otherwise deadlock on the second query).
func TestQueryInChunksRunsOncePerChunkAndClosesRows(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `CREATE TABLE t (id TEXT PRIMARY KEY, org TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := 0; i < 2*MaxInList+3; i++ {
		id := fmt.Sprintf("id-%04d", i)
		ids = append(ids, id)
		org := "o1"
		if i%10 == 0 {
			org = "o2"
		}
		if _, err := db.Exec(ctx, `INSERT INTO t (id, org) VALUES ($1, $2)`, id, org); err != nil {
			t.Fatal(err)
		}
	}

	statements := 0
	var got []string
	err := QueryInChunks(ctx, db, []any{"o1"}, ids, func(list string) string {
		statements++
		return `SELECT id FROM t WHERE org = $1 AND id IN ` + list
	}, func(rows *Rows) error {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		got = append(got, id)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if statements != 3 {
		t.Errorf("ran %d statements for %d values, want 3 chunks of at most %d", statements, len(ids), MaxInList)
	}
	want := 0
	for i := range ids {
		if i%10 != 0 {
			want++
		}
	}
	if len(got) != want {
		t.Errorf("scanned %d rows, want %d (the o1 rows across every chunk)", len(got), want)
	}

	calls := 0
	if err := QueryInChunks(ctx, db, nil, []string(nil), func(string) string { calls++; return "" }, nil); err != nil || calls != 0 {
		t.Errorf("empty values ran %d statements (err %v), want none", calls, err)
	}

	boom := errors.New("scan failed")
	err = QueryInChunks(ctx, db, nil, ids[:5], func(list string) string { return `SELECT id FROM t WHERE id IN ` + list },
		func(*Rows) error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("scan error not returned: %v", err)
	}
	// The failed chunk's rows were closed: the single connection is free for the next statement.
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM t`).Scan(&n); err != nil || n != len(ids) {
		t.Errorf("connection unusable after a failed chunk: n=%d err=%v", n, err)
	}
}
