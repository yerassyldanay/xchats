package dbtest

import (
	"context"
	"encoding/json"
	"reflect"
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
	if err := dbx.RunMigrationsWithOptions(ctx, db, migrations.ForDialect(string(db.Dialect())), dbx.MigrationOptions{Force: "all"}); err != nil {
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

func TestJSONMergePatchParity(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	var result string
	if err := db.QueryRow(ctx, `SELECT `+db.JSONMergePatch("$1", "$2"),
		`{"nested":{"keep":1,"remove":2},"list":[1,2],"other":true}`,
		`{"nested":{"remove":null,"add":3},"list":[4]}`).Scan(&result); err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal([]byte(result), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"nested":{"keep":1,"add":3},"list":[4],"other":true}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merge patch result: %s", result)
	}
}
