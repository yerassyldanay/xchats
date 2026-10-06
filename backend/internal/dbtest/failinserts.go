package dbtest

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// FailInserts makes every INSERT into table fail with message, on whichever
// engine db runs: a BEFORE INSERT trigger that raises. Tests use it to break
// one write path and nothing else, then heal it, e.g. to prove a failed ingest
// emits nothing and the redelivery lands once the database recovers. The
// returned function removes the trigger (a second call is a no-op); a test that
// never heals it can ignore the result, since the database is discarded with
// the test.
//
// table and message are test-controlled literals, not user input.
func FailInserts(t testing.TB, db *dbx.DB, table, message string) (remove func()) {
	t.Helper()
	ctx := context.Background()
	name := "force_fail_" + table
	msg := strings.ReplaceAll(message, "'", "''")
	create := `CREATE TRIGGER ` + name + ` BEFORE INSERT ON ` + table + `
 BEGIN SELECT RAISE(ABORT, '` + msg + `'); END`
	drop := `DROP TRIGGER ` + name
	if db.Dialect() == dbx.Postgres {
		create = `CREATE FUNCTION ` + name + `() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION '` + msg + `'; END $$;
 CREATE TRIGGER ` + name + ` BEFORE INSERT ON ` + table + ` FOR EACH ROW EXECUTE FUNCTION ` + name + `();`
		drop = `DROP TRIGGER ` + name + ` ON ` + table + `; DROP FUNCTION ` + name + `();`
	}
	if _, err := db.Exec(ctx, create); err != nil {
		t.Fatalf("dbtest: install failing trigger on %s: %v", table, err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if _, err := db.Exec(ctx, drop); err != nil {
				t.Fatalf("dbtest: remove failing trigger on %s: %v", table, err)
			}
		})
	}
}
