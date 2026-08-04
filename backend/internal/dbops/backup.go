// Package dbops implements durability tooling for the SQLite database
// internal/dbx opens: an online backup (Backup), a consistency check
// (IntegrityCheck), and an offline restore (Restore, in restore.go). It is
// deliberately separate from internal/dbx itself — dbx is "internal
// plumbing of the repository packages" (its own package doc), while this
// package is administrative tooling with a different audience (an operator
// or a CLI subcommand, not a request handler) — but it is still part of
// the persistence boundary (see internal/dbtest's architecture test) since
// it is the one other place in this module allowed to hold a *dbx.DB and
// run raw SQL against it.
//
// "Online" means Backup/IntegrityCheck need no downtime and never corrupt
// or block on a concurrently-running server — NOT that they run
// unnoticed: internal/dbx's one-physical-connection design (see its
// package doc) means a backup's VACUUM INTO, which reads the whole
// database, holds that one connection for its duration, so concurrent
// requests queue behind it rather than running in parallel with it. Plan
// backup timing accordingly; there is no lower-impact mode today (see
// dbx's own documented ReadQuery/BeginRead extension point for the
// unbuilt future split-reader-pool option).
package dbops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// Backup writes a consistent, compacted snapshot of db to destPath using
// SQLite's VACUUM INTO. Unlike a raw filesystem copy of the database file,
// this is safe to run against a live database: VACUUM INTO reads through
// the same connection every other operation on db uses (respecting WAL,
// see internal/dbx's package doc), rather than copying the main file and
// its -wal/-shm side-files as of possibly different moments. The produced
// file is fully self-contained (plain rollback-journal mode, no side-files
// of its own) and independently openable with any SQLite tool.
//
// destPath's parent directory is created if needed; destPath itself must
// NOT already exist — VACUUM INTO refuses to overwrite an existing file,
// and Backup does not second-guess that by deleting anything on the
// caller's behalf.
func Backup(ctx context.Context, db *dbx.DB, destPath string) error {
	if dir := filepath.Dir(destPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("dbops: create directory for %q: %w", destPath, err)
		}
	}
	if _, err := db.Exec(ctx, `VACUUM INTO ?`, destPath); err != nil {
		return fmt.Errorf("dbops: backup to %q: %w", destPath, err)
	}
	return nil
}

// IntegrityCheck runs SQLite's own consistency check (PRAGMA
// integrity_check: page structure, index/table row-count cross-checks,
// UNIQUE/NOT NULL constraint validity, and more — see SQLite's own docs for
// the full list) and returns every problem found, if any. A nil, nil result
// means the database is fully consistent. Read-only: safe against a live
// database, no exclusive lock or downtime required.
func IntegrityCheck(ctx context.Context, db *dbx.DB) ([]string, error) {
	rows, err := db.Query(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return nil, fmt.Errorf("dbops: integrity_check: %w", err)
	}
	defer rows.Close()

	var problems []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("dbops: scan integrity_check row: %w", err)
		}
		// A fully consistent database reports exactly one row containing
		// the literal string "ok" — every other row (there can be several,
		// one per problem found) is a real, description-carrying issue.
		if line != "ok" {
			problems = append(problems, line)
		}
	}
	return problems, rows.Err()
}
