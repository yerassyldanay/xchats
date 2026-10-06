package dbtest

import (
	"context"
	"strings"
	"testing"
)

func TestFailInsertsAbortsOnlyThatTableUntilRemoved(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	const org = `INSERT INTO organizations (id, name) VALUES ('c1111111-1111-1111-1111-111111111111', 'acme')`

	remove := FailInserts(t, db, "organizations", "forced ingest failure")
	if _, err := db.Exec(ctx, org); err == nil || !strings.Contains(err.Error(), "forced ingest failure") {
		t.Fatalf("insert into the failing table: err = %v, want the forced failure", err)
	}
	// Inserts into other tables are untouched.
	mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u-fail-1', 'fail-1@example.com', 'x')`)

	remove()
	mustExec(t, db, ctx, org)
	remove() // removing twice is harmless
}
