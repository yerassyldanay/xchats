package dbtest

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFailInsertsAbortsOnlyThatTableUntilRemoved(t *testing.T) {
	now := time.Now()

	db := OpenRaw(t)
	ctx := context.Background()
	const org = `INSERT INTO organizations (id, name, created_at, updated_at) VALUES ('c1111111-1111-1111-1111-111111111111', 'acme', 1791244800000, 1791244800000)`

	remove := FailInserts(t, db, "organizations", "forced ingest failure")
	if _, err := db.Exec(ctx, org); err == nil || !strings.Contains(err.Error(), "forced ingest failure") {
		t.Fatalf("insert into the failing table: err = %v, want the forced failure", err)
	}
	// Inserts into other tables are untouched.
	mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u-fail-1', 'fail-1@example.com', 'x', $1, $1)`, now)

	remove()
	mustExec(t, db, ctx, org)
	remove() // removing twice is harmless
}
