package dbops_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbops"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// openSeeded opens a fresh database at t.TempDir()/name and seeds one table
// with a couple of rows — dbops works on any SQLite file, so its own tests
// deliberately do not depend on the app's real migrations/schema.
func openSeeded(t *testing.T, name string) (*dbx.DB, string) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), name)
	db, err := dbx.Open(ctx, path)
	if err != nil {
		t.Fatalf("dbx.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(ctx, `CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO widgets (id, name) VALUES (1, 'first'), (2, 'second')`); err != nil {
		t.Fatalf("seed rows: %v", err)
	}
	return db, path
}

func TestBackup_RoundTrip(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	dest := filepath.Join(t.TempDir(), "out", "backup.db")

	if err := dbops.Backup(ctx, db, dest); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	check, err := dbx.OpenReadOnly(ctx, dest)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer check.Close()

	rows, err := check.Query(ctx, `SELECT id, name FROM widgets ORDER BY id`)
	if err != nil {
		t.Fatalf("query backup: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("backup rows = %v, want [first second]", got)
	}
}

func TestBackup_CreatesDestinationDirectory(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	dest := filepath.Join(t.TempDir(), "does", "not", "exist", "yet", "backup.db")

	if err := dbops.Backup(ctx, db, dest); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
}

func TestBackup_RefusesToOverwriteExistingDestination(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	dest := filepath.Join(t.TempDir(), "backup.db")

	if err := dbops.Backup(ctx, db, dest); err != nil {
		t.Fatalf("first Backup: %v", err)
	}
	if err := dbops.Backup(ctx, db, dest); err == nil {
		t.Fatal("second Backup to the SAME destination unexpectedly succeeded")
	}
}

func TestBackup_OutputIsSelfContainedNoWALSidecar(t *testing.T) {
	// The produced file must be independently inspectable with plain
	// filesystem tools (copy, hash, upload) — no -wal/-shm side-files of
	// its own, even though the source database is WAL-mode.
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")
	dest := filepath.Join(t.TempDir(), "backup.db")

	if err := dbops.Backup(ctx, db, dest); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	if _, err := os.Stat(dest + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("backup has a -wal sidecar: stat err = %v", err)
	}
	if _, err := os.Stat(dest + "-shm"); !os.IsNotExist(err) {
		t.Fatalf("backup has a -shm sidecar: stat err = %v", err)
	}
}

func TestIntegrityCheck_HealthyDatabaseReportsNoProblems(t *testing.T) {
	ctx := context.Background()
	db, _ := openSeeded(t, "live.db")

	problems, err := dbops.IntegrityCheck(ctx, db)
	if err != nil {
		t.Fatalf("IntegrityCheck: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none for a freshly created database", problems)
	}
}

func TestIntegrityCheck_DetectsATruncatedFile(t *testing.T) {
	ctx := context.Background()
	db, path := openSeeded(t, "live.db")
	// Force everything onto disk under the live path before truncating a
	// copy of it out from under the connection.
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() < 4096 {
		t.Fatalf("sanity: test db is smaller than one page (%d bytes), can't truncate meaningfully", info.Size())
	}
	if err := os.Truncate(path, info.Size()/2); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// A truncated file can be caught either at open (SQLite validates the
	// header/page count as part of Ping) or later by IntegrityCheck's own
	// page-level walk — both are a correct "this file is unusable" signal,
	// and which one fires is a SQLite-version/corruption-shape detail this
	// test should not be fragile to. dbops.Restore's own validateBackup
	// treats both paths identically (see TestRestore_
	// RejectsCorruptBackupWithoutTouchingDestination for that end-to-end
	// contract); this test only needs ONE of the two to fire.
	ro, err := dbx.OpenReadOnly(ctx, path)
	if err != nil {
		t.Logf("truncated file rejected at open (a valid detection point): %v", err)
		return
	}
	defer ro.Close()

	problems, err := dbops.IntegrityCheck(ctx, ro)
	if err != nil {
		t.Logf("truncated file rejected by IntegrityCheck's own query (a valid detection point): %v", err)
		return
	}
	if len(problems) == 0 {
		t.Fatal("neither OpenReadOnly nor IntegrityCheck reported any problem for a truncated (corrupt) database file")
	}
}
