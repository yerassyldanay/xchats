package dbx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"time"
)

var migrationName = regexp.MustCompile(`^([0-9]{14})_[a-z][a-z0-9_]*\.sql$`)

// MigrationOptions permits explicit development replay. Empty Force applies
// only pending files; "all" or a full identifier replays the chosen files and
// replaces their checksums. Replay relies on idempotent SQL, never implicit DDL
// error suppression. Reset a development database by recreating it, not by
// deleting history while leaving its schema behind.
type MigrationOptions struct{ Force string }

// migration is one plain-SQL file. body is the file exactly as written — there
// is no template step and no per-engine variant — and checksum is the SHA-256 of
// those same bytes with CRLF read as LF (see checksumOf), so one recorded checksum
// means the same on every engine and for every checkout.
type migration struct{ id, body, checksum string }

// checksumOf is the SHA-256 of a migration file with Windows line endings read as
// Unix ones. go:embed embeds a file as it was checked out, and a checkout with
// autocrlf (the Windows desktop build) turns LF into CRLF; the migration is the
// same migration, so a database migrated by one build must not be rejected by the other.
func checksumOf(b []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))))
}

// RunMigrations applies every unrecorded identifier, including files older than
// the newest applied one. The full timestamp AND description are the identity.
// Each file is plain SQL handed to the database verbatim, the same bytes on
// SQLite and PostgreSQL, inside one transaction per file.
func RunMigrations(ctx context.Context, db *DB, mfs fs.FS) error {
	return RunMigrationsWithOptions(ctx, db, mfs, MigrationOptions{})
}

func RunMigrationsWithOptions(ctx context.Context, db *DB, mfs fs.FS, opts MigrationOptions) error {
	files, err := loadMigrations(mfs)
	if err != nil {
		return err
	}
	found := opts.Force == "" || opts.Force == "all"
	for _, m := range files {
		if m.id == opts.Force {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("dbx: unknown migration to replay: %s", opts.Force)
	}
	if err := refuseOlderBuildDatabase(ctx, db); err != nil {
		return err
	}
	// Initialize under the same cross-process lock as application write
	// transactions. PostgreSQL CREATE IF NOT EXISTS alone can race in pg_class.
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
  identifier TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at BIGINT NOT NULL
 )`)
	if err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("dbx: create migration history: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, m := range files {
		if err := applyMigration(ctx, db, m, opts.Force == "all" || opts.Force == m.id); err != nil {
			return err
		}
	}
	return nil
}

func loadMigrations(mfs fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(mfs, ".")
	if err != nil {
		return nil, fmt.Errorf("dbx: read migrations: %w", err)
	}
	var files []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		parts := migrationName.FindStringSubmatch(e.Name())
		if parts == nil {
			return nil, fmt.Errorf("dbx: invalid migration filename %q; use YYYYMMDDHHMMSS_name.sql", e.Name())
		}
		if _, err := time.Parse("20060102150405", parts[1]); err != nil {
			return nil, fmt.Errorf("dbx: invalid UTC timestamp in %q", e.Name())
		}
		b, err := fs.ReadFile(mfs, e.Name())
		if err != nil {
			return nil, fmt.Errorf("dbx: read migration %s: %w", e.Name(), err)
		}
		if strings.TrimSpace(string(b)) == "" {
			return nil, fmt.Errorf("dbx: empty migration %s", e.Name())
		}
		files = append(files, migration{strings.TrimSuffix(e.Name(), ".sql"), string(b), checksumOf(b)})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("dbx: no SQL migrations found")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].id < files[j].id })
	return files, nil
}

func applyMigration(ctx context.Context, db *DB, m migration, force bool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("dbx: begin migration %s: %w", m.id, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var checksum string
	err = tx.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE identifier = $1`, m.id).Scan(&checksum)
	if err != nil && !errors.Is(err, ErrNoRows) {
		return err
	}
	if err == nil && !force {
		if checksum != m.checksum {
			return fmt.Errorf("dbx: checksum mismatch for migration %s; restore the file or explicitly replay in development", m.id)
		}
		return tx.Commit(ctx)
	}
	// No statement splitting: drivers understand complete scripts, including
	// quoted semicolons. A file that is only comments is a recorded no-op.
	if _, err := tx.Exec(ctx, m.body); err != nil {
		return fmt.Errorf("dbx: apply migration %s: %w", m.id, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (identifier, checksum, applied_at) VALUES ($1,$2,$3)
 ON CONFLICT (identifier) DO UPDATE SET checksum=excluded.checksum, applied_at=excluded.applied_at`,
		m.id, m.checksum, time.Now().UnixMilli()); err != nil {
		return fmt.Errorf("dbx: record migration %s: %w", m.id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("dbx: commit migration %s: %w", m.id, err)
	}
	return nil
}

// legacyHistoryTable is where the pre-squash runner (before the five plain-SQL baselines) kept
// its history.
const legacyHistoryTable = "xchats_schema_migrations"

// refuseOlderBuildDatabase stops a database created by an older xchats build from being
// migrated. Its tables have a different shape, and the baseline files are all
// CREATE ... IF NOT EXISTS: they would be recorded as applied over the old tables as silent
// no-ops, and the application would then write BIGINT milliseconds into the old columns. There
// is no upgrade path while xchats is unreleased, so the only safe answer is a clear refusal that
// changes nothing.
func refuseOlderBuildDatabase(ctx context.Context, db *DB) error {
	legacy, err := tableExists(ctx, db, legacyHistoryTable)
	if err != nil || !legacy {
		return err
	}
	current, err := tableExists(ctx, db, "schema_migrations")
	if err != nil || current {
		return err
	}
	return fmt.Errorf("dbx: this database was created by an older xchats build (it has the table %s) and its schema is not compatible with the current migrations; recreate it (delete the SQLite file, or drop and recreate the PostgreSQL database) and run migrate again", legacyHistoryTable)
}

// tableExists asks the engine's own catalog: this is the engine boundary, so it may.
func tableExists(ctx context.Context, db *DB, name string) (bool, error) {
	query := `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = $1`
	if db.Dialect() == Postgres {
		query = `SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = $1`
	}
	var n int
	if err := db.QueryRow(ctx, query, name).Scan(&n); err != nil {
		return false, fmt.Errorf("dbx: look for table %s: %w", name, err)
	}
	return n > 0, nil
}
