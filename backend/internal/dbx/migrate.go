package dbx

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// RunMigrations applies every *.up.sql in mfs, in lexical order, recording
// applied versions in xchats_schema_migrations so re-runs are no-ops — the
// same contract as the old pgx-era runner (internal/store/migrate.go, now
// frozen under migrations/postgres as pgschema), ported onto *DB. The DDL
// itself is also IF NOT EXISTS, so this is safe even on a partially
// migrated database. Each file's whole body (which may hold several
// statements) runs as one Exec inside its own transaction — modernc.org/
// sqlite executes a multi-statement string as a sequential script, so the
// split migration files (0001_core.up.sql, 0002_channels.up.sql, ...) need
// no per-statement splitting here, matching how RunMigrations always ran
// each file against Postgres.
func RunMigrations(ctx context.Context, db *DB, mfs fs.FS) error {
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS xchats_schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
		)`); err != nil {
		return fmt.Errorf("dbx: create migrations table: %w", err)
	}

	entries, err := fs.ReadDir(mfs, ".")
	if err != nil {
		return fmt.Errorf("dbx: read migrations dir: %w", err)
	}
	var versions []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			versions = append(versions, e.Name())
		}
	}
	sort.Strings(versions)

	for _, name := range versions {
		version := strings.TrimSuffix(name, ".up.sql")
		var exists bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM xchats_schema_migrations WHERE version = $1)`, version).
			Scan(&exists); err != nil {
			return fmt.Errorf("dbx: check migration %s: %w", version, err)
		}
		if exists {
			continue
		}
		body, err := fs.ReadFile(mfs, name)
		if err != nil {
			return fmt.Errorf("dbx: read migration %s: %w", name, err)
		}
		if err := applyMigration(ctx, db, version, string(body)); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *DB, version, body string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("dbx: begin migration %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx, body); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("dbx: apply migration %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO xchats_schema_migrations(version) VALUES($1)`, version); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("dbx: record migration %s: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("dbx: commit migration %s: %w", version, err)
	}
	return nil
}
