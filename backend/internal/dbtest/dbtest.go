// Package dbtest provides isolated SQLite files or PostgreSQL schemas for tests.
package dbtest

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

// OpenRaw opens a fresh, migrated database at t.TempDir() and returns the
// bare *dbx.DB — for tests that verify the schema/migration machinery
// itself (the architecture and migration tests in this package) rather than
// exercising a repository package. Repository package tests want New (added
// in Phase 2 alongside internal/store), not this.
func OpenRaw(t testing.TB) *dbx.DB {
	t.Helper()
	ctx := context.Background()
	path := Target(t)
	db, err := dbx.Open(ctx, path)
	if err != nil {
		t.Fatalf("dbtest: open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("dbtest: close: %v", err)
		}
	})
	if err := dbx.RunMigrations(ctx, db, migrations.FS); err != nil {
		t.Fatalf("dbtest: migrate: %v", err)
	}
	return db
}

// reapplyMigrations re-runs the migration runner against an already-
// migrated database — for tests asserting idempotency (a second run must be
// a no-op, not an error and not a duplicate insert).
func reapplyMigrations(t testing.TB, db *dbx.DB) error {
	t.Helper()
	return dbx.RunMigrations(context.Background(), db, migrations.FS)
}

// moduleRoot returns the directory containing the backend module's go.mod —
// used by tests in this package that need a stable filesystem anchor
// (`go list` for the architecture check) independent
// of the working directory `go test` happens to run from.
func moduleRoot(t testing.TB) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("dbtest: go env GOMOD: %v", err)
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == "/dev/null" {
		t.Fatal("dbtest: go env GOMOD returned no module file — are we inside a Go module?")
	}
	return filepath.Dir(gomod)
}
