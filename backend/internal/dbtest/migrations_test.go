package dbtest

import (
	"context"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

func TestBaselineReplayPreservesData(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, `UPDATE users SET password_hash='operator-changed', must_change_password=FALSE WHERE id='00000000-0000-0000-0000-000000000002'`)
	if err := dbx.RunMigrationsWithOptions(ctx, db, migrations.FS, dbx.MigrationOptions{Force: "all"}); err != nil {
		t.Fatal(err)
	}
	var hash string
	var change bool
	if err := db.QueryRow(ctx, `SELECT password_hash,must_change_password FROM users WHERE id='00000000-0000-0000-0000-000000000002'`).Scan(&hash, &change); err != nil {
		t.Fatal(err)
	}
	if hash != "operator-changed" || change {
		t.Fatal("replay overwrote an operator's password")
	}
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM crm_statuses`).Scan(&count); err != nil || count != 5 {
		t.Fatalf("seed duplicated: %d: %v", count, err)
	}
}

func TestConcurrentMigrationRollbackAndLateArrival(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	files := fstest.MapFS{"20261002000000_later.sql": {Data: []byte(`CREATE TABLE IF NOT EXISTS migration_probe (id INTEGER PRIMARY KEY, runs INTEGER NOT NULL);
 INSERT INTO migration_probe VALUES(1,1) ON CONFLICT(id) DO UPDATE SET runs=migration_probe.runs+1;`)}}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- dbx.RunMigrations(ctx, db, files) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var runs int
	if err := db.QueryRow(ctx, `SELECT runs FROM migration_probe WHERE id=1`).Scan(&runs); err != nil || runs != 1 {
		t.Fatalf("migration ran %d times: %v", runs, err)
	}
	files["20261001000000_earlier.sql"] = &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS earlier_probe (id INTEGER PRIMARY KEY);`)}
	if err := dbx.RunMigrations(ctx, db, files); err != nil {
		t.Fatal(err)
	}
	bad := fstest.MapFS{"20261003000000_failure.sql": {Data: []byte(`CREATE TABLE rollback_probe (id INTEGER); INSERT INTO missing_table VALUES (1);`)}}
	if err := dbx.RunMigrations(ctx, db, bad); err == nil {
		t.Fatal("invalid SQL succeeded")
	}
	if _, err := db.Exec(ctx, `SELECT * FROM rollback_probe`); err == nil {
		t.Fatal("failed migration left its table behind")
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE identifier='20261003000000_failure'`).Scan(&runs); err != nil || runs != 0 {
		t.Fatalf("failed migration recorded: %d: %v", runs, err)
	}
	// Failure is retryable after correcting the unrecorded file.
	bad["20261003000000_failure.sql"].Data = []byte(`CREATE TABLE IF NOT EXISTS rollback_probe (id INTEGER);`)
	if err := dbx.RunMigrations(ctx, db, bad); err != nil {
		t.Fatal(err)
	}
}

// A migration file that is only comments (what `make migration-new` generates before it
// is filled in) must apply as a recorded no-op on whichever engine runs it. SQLite used to
// panic on comment-only SQL.
func TestCommentOnlyMigrationIsRecordedOnBothEngines(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	files := fstest.MapFS{
		"29990101000000_header_only.sql":   {Data: []byte("-- header only\n-- Plain SQL executed verbatim on SQLite and PostgreSQL.\n")},
		"29990102000000_comments_only.sql": {Data: []byte("-- one\n\n/* two\n   lines */\n-- three\n")},
	}
	if err := dbx.RunMigrations(ctx, db, files); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE lower(identifier) LIKE '2999%'`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("recorded %d no-op migrations (%v), want 2", n, err)
	}
}

// A database created by an older xchats build keeps its history in xchats_schema_migrations and
// has tables of a different shape. The baseline files are all CREATE ... IF NOT EXISTS, so running
// them over such a database would be silent no-ops followed by writes in the new representation
// (BIGINT milliseconds into the old columns). The runner must refuse, and change nothing.
func TestMigrationsRefuseADatabaseFromAnOlderBuild(t *testing.T) {
	ctx := context.Background()
	db, err := dbx.Open(ctx, Target(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mustExec(t, db, ctx, `CREATE TABLE xchats_schema_migrations (version TEXT PRIMARY KEY)`)
	mustExec(t, db, ctx, `CREATE TABLE users (id TEXT PRIMARY KEY NOT NULL, created_at TEXT)`)

	err = dbx.RunMigrations(ctx, db, migrations.FS)
	if err == nil || !strings.Contains(err.Error(), "older xchats build") {
		t.Fatalf("RunMigrations over a database from an older build: err = %v, want a refusal naming the older build", err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err == nil {
		t.Fatal("the refused run still created schema_migrations")
	}
}
