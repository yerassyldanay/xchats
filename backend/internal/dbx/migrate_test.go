package dbx

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
	"time"
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

func TestRunMigrationsRendersMacros(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	mfs := fstest.MapFS{"20260101000000_macros.sql": {Data: []byte(`
CREATE TABLE macros (
    id   TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    at   {{timestamp}} NOT NULL DEFAULT {{now}},
    meta {{json "meta"}} NOT NULL DEFAULT '{}',
    raw  {{json "raw"}},
    mail {{citext}} NOT NULL DEFAULT 'x@example.com'
);
{{if postgres}}CREATE TABLE only_postgres (id INTEGER);{{end}}
{{if sqlite}}CREATE TABLE only_sqlite (id INTEGER);{{end}}
`)}}
	if err := RunMigrations(ctx, db, mfs); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var id, at string
	if err := db.QueryRow(ctx, `INSERT INTO macros DEFAULT VALUES RETURNING id, at`).Scan(&id, &at); err != nil {
		t.Fatalf("insert defaults: %v", err)
	}
	if !uuidV4RE.MatchString(id) {
		t.Errorf("id default %q is not a UUIDv4", id)
	}
	parsed, err := ParseTime(at)
	if err != nil {
		t.Fatalf("at default %q does not parse: %v", at, err)
	}
	if d := time.Since(parsed); d < -time.Minute || d > time.Minute {
		t.Errorf("at default %q is %v away from now", at, d)
	}
	if _, err := db.Exec(ctx, `INSERT INTO macros (meta) VALUES ('not json')`); err == nil {
		t.Error("meta accepted invalid JSON")
	}
	if _, err := db.Exec(ctx, `INSERT INTO macros (raw) VALUES (NULL)`); err != nil {
		t.Errorf("nullable json column rejected NULL: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO macros (raw) VALUES ('also not json')`); err == nil {
		t.Error("raw accepted invalid JSON")
	}
	if _, err := db.Exec(ctx, `INSERT INTO macros (mail) VALUES ('X@EXAMPLE.COM')`); err != nil {
		t.Fatalf("insert mixed-case mail: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO macros (mail) VALUES ('x@example.com')`); err != nil {
		t.Fatalf("mail is not unique, so this must succeed: %v", err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM macros WHERE mail = 'x@example.com'`).Scan(&n); err != nil || n != 4 {
		t.Errorf("case-insensitive mail match: %d rows, %v; want 4 (default row, NULL-raw row, and both inserts)", n, err)
	}
	for table, want := range map[string]int{"only_sqlite": 1, "only_postgres": 0} {
		if err := db.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=$1`, table).Scan(&n); err != nil || n != want {
			t.Errorf("%s exists=%d (%v), want %d", table, n, err, want)
		}
	}
}

func TestRunMigrationsRecordsDialectNoOp(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	mfs := fstest.MapFS{"20260101000000_pg_only.sql": {Data: []byte(`{{if postgres}}CREATE TABLE pg_only (id INTEGER);{{end}}`)}}
	if err := RunMigrations(ctx, db, mfs); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE identifier = '20260101000000_pg_only'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("no-op migration recorded %d times (%v), want 1", n, err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE name = 'pg_only'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("pg_only exists=%d (%v) on SQLite", n, err)
	}
	if err := RunMigrations(ctx, db, mfs); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}
}

func TestMigrationChecksumIsSourceHash(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	src := []byte(`CREATE TABLE IF NOT EXISTS hashed (at {{timestamp}} NOT NULL DEFAULT {{now}});`)
	mfs := fstest.MapFS{"20260101000000_hashed.sql": {Data: src}}
	if err := RunMigrations(ctx, db, mfs); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	var got string
	if err := db.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE identifier = '20260101000000_hashed'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("%x", sha256.Sum256(src)); got != want {
		t.Fatalf("checksum = %s, want the SHA-256 of the unrendered source %s", got, want)
	}
}

func TestRunMigrationsTemplateErrorAppliesNothing(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	mfs := fstest.MapFS{
		"20260101000000_ok.sql":  {Data: []byte(`CREATE TABLE ok (id INTEGER PRIMARY KEY)`)},
		"20260102000000_bad.sql": {Data: []byte(`{{nope}}`)},
	}
	err := RunMigrations(ctx, db, mfs)
	if err == nil {
		t.Fatal("RunMigrations accepted a template error")
	}
	if !strings.Contains(err.Error(), "20260102000000_bad.sql") {
		t.Errorf("error %q does not name the bad file", err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE name = 'ok'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("earlier valid file was applied (%d tables, %v) although a later file does not render", n, err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&n); err == nil && n != 0 {
		t.Errorf("%d history rows recorded for a run that must apply nothing", n)
	}
}
