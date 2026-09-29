package dbx

import (
	"context"
	"testing"
	"testing/fstest"
)

func TestRunMigrationsFreshAndRepeated(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	mfs := fstest.MapFS{
		"20260101000000_a.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE a (id INTEGER PRIMARY KEY, v TEXT);
			CREATE TABLE b (id INTEGER PRIMARY KEY);
			CREATE INDEX a_v_idx ON a(v);
		`)},
		"20260102000000_b.sql": &fstest.MapFile{Data: []byte(`
			ALTER TABLE b ADD COLUMN name TEXT NOT NULL DEFAULT '';
			INSERT INTO a (v) VALUES ('seed');
		`)},
	}

	if err := RunMigrations(ctx, db, mfs); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}

	var v string
	if err := db.QueryRow(ctx, `SELECT v FROM a WHERE id = 1`).Scan(&v); err != nil {
		t.Fatalf("seed row missing after migration: %v", err)
	}
	if v != "seed" {
		t.Errorf("v = %q, want %q", v, "seed")
	}

	var versionCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if versionCount != 2 {
		t.Fatalf("recorded %d versions, want 2", versionCount)
	}

	// Re-running must be a complete no-op: 0002's INSERT must NOT run again.
	if err := RunMigrations(ctx, db, mfs); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}
	var rowCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM a`).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if rowCount != 1 {
		t.Errorf("row count after repeated migration = %d, want 1 (0002 re-ran)", rowCount)
	}
}

func TestRunMigrationsAppliesNewFilesOnly(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	mfs1 := fstest.MapFS{
		"20260101000000_a.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY)`)},
	}
	if err := RunMigrations(ctx, db, mfs1); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}

	// Simulate a later deploy that adds 0002 on top of an already-migrated
	// database — only the new file should run.
	mfs2 := fstest.MapFS{
		"20260101000000_a.sql": mfs1["20260101000000_a.sql"],
		"20260102000000_b.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE b (id INTEGER PRIMARY KEY)`)},
	}
	if err := RunMigrations(ctx, db, mfs2); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	for _, table := range []string{"a", "b"} {
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=$1`, table).
			Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("table %s missing after incremental migration", table)
		}
	}
}

func TestRunMigrationsRollsBackOnFailure(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()

	mfs := fstest.MapFS{
		"20260101000000_bad.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE a (id INTEGER PRIMARY KEY);
			INSERT INTO a (id) VALUES (1);
			this is not valid sql;
		`)},
	}
	if err := RunMigrations(ctx, db, mfs); err == nil {
		t.Fatal("expected an error from invalid SQL in a migration")
	}

	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='a'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("table 'a' exists after a failed migration — the transaction did not roll back")
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("a failed migration was recorded as applied")
	}
}

func TestMigrationsOutOfOrderChecksumAndReplay(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	mfs := fstest.MapFS{
		"20260202000000_later.sql": {Data: []byte(`CREATE TABLE IF NOT EXISTS later (id INTEGER PRIMARY KEY); INSERT INTO later VALUES (1) ON CONFLICT DO NOTHING;`)},
	}
	if err := RunMigrations(ctx, db, mfs); err != nil {
		t.Fatal(err)
	}
	// Full IDs distinguish same-second branch work; earlier arrivals are applied.
	mfs["20260101000000_earlier.sql"] = &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS earlier (id INTEGER PRIMARY KEY);`)}
	mfs["20260202000000_parallel.sql"] = &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS parallel (id INTEGER PRIMARY KEY);`)}
	if err := RunMigrations(ctx, db, mfs); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("history count=%d: %v", n, err)
	}
	mfs["20260202000000_later.sql"].Data = append(mfs["20260202000000_later.sql"].Data, []byte("\n-- intentional edit")...)
	if err := RunMigrations(ctx, db, mfs); err == nil {
		t.Fatal("changed applied migration accepted")
	}
	if err := RunMigrationsWithOptions(ctx, db, mfs, MigrationOptions{Force: "20260202000000_later"}); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrationsWithOptions(ctx, db, mfs, MigrationOptions{Force: "all"}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM later`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("replay duplicated seed: %d: %v", n, err)
	}
	if err := RunMigrations(ctx, db, mfs); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrationsWithOptions(ctx, db, mfs, MigrationOptions{Force: "missing"}); err == nil {
		t.Fatal("unknown replay accepted")
	}
}

func TestMigrationNamesValidatedBeforeChanges(t *testing.T) {
	for _, name := range []string{"0001_legacy.sql", "20260230000000_invalid.sql", "20260101000000_Upper.sql", "20260101000000_name.up.sql"} {
		t.Run(name, func(t *testing.T) {
			db := openTest(t)
			if err := RunMigrations(context.Background(), db, fstest.MapFS{name: {Data: []byte(`CREATE TABLE invalid (id INTEGER)`)}}); err == nil {
				t.Fatal("invalid name accepted")
			}
		})
	}
}
