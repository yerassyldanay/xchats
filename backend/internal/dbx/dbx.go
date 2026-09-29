// Package dbx provides the shared SQLite/PostgreSQL persistence boundary.
package dbx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/gofrs/flock"
	_ "modernc.org/sqlite"
)

// ErrNoRows is returned by QueryRow.Scan when the query matches no row —
// identical to sql.ErrNoRows (dbx does not wrap it), so callers may use
// database/sql's own ErrNoRows too; this alias exists for symmetry with the
// old pgx.ErrNoRows call sites being ported.
var ErrNoRows = sql.ErrNoRows

// DB is a refcounted shared database handle, keyed by SQLite path or PostgreSQL URL.
type DB struct {
	path    string
	dialect Dialect
	sdb     *sql.DB
	// standalone marks a DB returned by OpenReadOnly: not in registry, no
	// refcount, no flock — Close just closes sdb directly. See Close.
	standalone bool
}

type sharedDB struct {
	db       *DB
	fl       *flock.Flock
	refCount int
}

var (
	registryMu sync.Mutex
	registry   = map[string]*sharedDB{}
)

// Open detects postgres:// and postgresql:// URLs; other targets are SQLite
// file paths. Repeated opens share a refcounted pool. PostgreSQL uses a bounded
// multi-connection pool; SQLite creates the parent directory and file.
//
// Every connection this process opens for a given path shares one *sql.DB
// with MaxOpenConns(1), WAL journaling, busy_timeout(5000), synchronous
// NORMAL, foreign_keys on, and _txlock=immediate — see the package doc.
// A gofrs/flock lock file next to the database is held for as long as any
// caller in this process holds the handle open, so a second OS process
// pointed at the same path fails fast at Open instead of corrupting state.
//
// Safe for concurrent use. Every successful Open must be paired with a
// Close; the underlying connection and lock are only released when the
// last reference is closed.
func Open(ctx context.Context, path string) (*DB, error) {
	if strings.HasPrefix(path, "postgres://") || strings.HasPrefix(path, "postgresql://") {
		return openPostgres(ctx, path)
	}
	if strings.Contains(path, "://") {
		return nil, fmt.Errorf("dbx: unsupported database URL scheme")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("dbx: resolve path %q: %w", path, err)
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	if s, ok := registry[abs]; ok {
		s.refCount++
		return s.db, nil
	}

	if dir := filepath.Dir(abs); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("dbx: create directory for %q: %w", abs, err)
		}
	}

	fl := flock.New(lockPath(abs))
	locked, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("dbx: acquire lock for %q: %w", abs, err)
	}
	if !locked {
		return nil, fmt.Errorf("dbx: database %q: %w", abs, errSingleProcessLock)
	}

	sdb, err := sql.Open("sqlite", dsn(abs))
	if err != nil {
		_ = fl.Unlock()
		return nil, fmt.Errorf("dbx: open %q: %w", abs, err)
	}
	// The whole design rests on this: exactly one physical connection, so
	// every writer serializes through Go's own pool queueing rather than
	// SQLite's busy-retry loop. See the package doc.
	sdb.SetMaxOpenConns(1)
	sdb.SetMaxIdleConns(1)

	if err := sdb.PingContext(ctx); err != nil {
		_ = sdb.Close()
		_ = fl.Unlock()
		return nil, fmt.Errorf("dbx: ping %q: %w", abs, err)
	}

	db := &DB{path: abs, sdb: sdb, dialect: SQLite}
	registry[abs] = &sharedDB{db: db, fl: fl, refCount: 1}
	return db, nil
}

// OpenReadOnly opens path in SQLite's genuinely read-only connection mode
// (URI mode=ro) — every write, including a pragma that would otherwise
// mutate the file (journal_mode=WAL is the one that matters here: Open's
// own DSN sets it, and connecting with it against a plain rollback-journal
// file silently upgrades that file to WAL mode as a side effect of merely
// pinging it — verified empirically, not documented behavior an unrelated
// reader would expect), fails outright instead.
//
// This is NOT Open: no refcounted registry entry, no single-process flock
// — a read-only inspection of a file some other handle (this process's own
// Open, or another process entirely) already has open is exactly the safe
// case this exists for, and taking a lock here would defeat that. It
// exists for internal/dbops, to validate a backup file's integrity without
// disturbing its on-disk format or contending with a live server.
//
// The returned *DB fails at Ping (wrapped below) rather than succeeding for
// a path that does not exist or is not a SQLite database at all — verified
// empirically — so callers do not need their own existence/format
// pre-check.
func OpenReadOnly(ctx context.Context, path string) (*DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("dbx: resolve path %q: %w", path, err)
	}

	v := url.Values{}
	v.Set("mode", "ro")
	sdb, err := sql.Open("sqlite", "file:"+filepath.ToSlash(abs)+"?"+v.Encode())
	if err != nil {
		return nil, fmt.Errorf("dbx: open %q read-only: %w", abs, err)
	}
	if err := sdb.PingContext(ctx); err != nil {
		_ = sdb.Close()
		return nil, fmt.Errorf("dbx: open %q read-only: %w", abs, err)
	}
	return &DB{path: abs, sdb: sdb, standalone: true}, nil
}

// dsn builds the modernc.org/sqlite DSN for abs: WAL journaling, a 5s busy
// timeout, synchronous NORMAL (safe under WAL — only synchronous FULL
// protects against an OS crash, not the process crashes this app cares
// about), foreign keys on, and immediate-mode write transactions (see the
// package doc for why that's what makes the single-pool design correct by
// construction).
func dsn(abs string) string {
	v := url.Values{}
	v.Add("_pragma", "foreign_keys(1)")
	v.Add("_pragma", "journal_mode(WAL)")
	v.Add("_pragma", "busy_timeout(5000)")
	v.Add("_pragma", "synchronous(NORMAL)")
	v.Set("_txlock", "immediate")
	return "file:" + filepath.ToSlash(abs) + "?" + v.Encode()
}

// Close releases this handle's reference. The underlying connection and the
// single-process lock are only actually released once every Open caller for
// this path has called Close. For a standalone (OpenReadOnly) handle, which
// was never added to the registry, Close just closes the connection.
func (db *DB) Close() error {
	if db.standalone {
		return db.sdb.Close()
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	s, ok := registry[db.path]
	if !ok {
		return nil
	}
	s.refCount--
	if s.refCount > 0 {
		return nil
	}
	delete(registry, db.path)

	err := db.sdb.Close()
	if s.fl != nil {
		if unlockErr := s.fl.Unlock(); unlockErr != nil && err == nil {
			err = fmt.Errorf("dbx: release lock for %q: %w", db.path, unlockErr)
		}
	}
	return err
}

// Path returns the SQLite path or PostgreSQL connection string for internal
// pool sharing. It can contain credentials: never expose it in logs or APIs.
func (db *DB) Path() string { return db.path }

// LockPath returns the flock file path Open uses to enforce single-process
// exclusivity for a database at path — exported so tooling that must hold
// (or merely probe) that same exclusion without going through the heavier
// Open/Ping/pragma dance (a restore, say — see internal/dbops) shares the
// exact same lock, and the naming convention lives in exactly one place.
func LockPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("dbx: resolve path %q: %w", path, err)
	}
	return lockPath(abs), nil
}

// lockPath is LockPath's core over an already-absolute path — Open calls
// this directly since it has already resolved abs itself.
func lockPath(abs string) string { return abs + ".lock" }

// Ping verifies the database is reachable.
func (db *DB) Ping(ctx context.Context) error { return db.sdb.PingContext(ctx) }

// Query runs a query expected to return rows.
func (db *DB) Query(ctx context.Context, query string, args ...any) (*Rows, error) {
	r, err := db.sdb.QueryContext(ctx, query, db.bind(args)...)
	if err != nil {
		return nil, err
	}
	return &Rows{r: r}, nil
}

// QueryRow runs a query expected to return at most one row.
func (db *DB) QueryRow(ctx context.Context, query string, args ...any) *Row {
	return &Row{r: db.sdb.QueryRowContext(ctx, query, db.bind(args)...)}
}

// Exec runs a query that doesn't return rows.
func (db *DB) Exec(ctx context.Context, query string, args ...any) (CommandTag, error) {
	res, err := db.sdb.ExecContext(ctx, query, db.bind(args)...)
	if err != nil {
		return CommandTag{}, err
	}
	return newCommandTag(res), nil
}

// Begin starts a write transaction: SQLite uses BEGIN IMMEDIATE; PostgreSQL
// uses a transaction-scoped advisory lock to preserve read-modify-write invariants.
func (db *DB) Begin(ctx context.Context) (*Tx, error) {
	tx, err := db.sdb.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	// Serialize read-modify-write transactions across processes just as SQLite's
	// BEGIN IMMEDIATE does. Reads outside transactions still use the pool.
	if db.Dialect() == Postgres {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext(current_database()), hashtext(current_schema()))`); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}
	return &Tx{tx: tx, dialect: db.Dialect()}, nil
}

// DBTX is satisfied by both *DB and *Tx — the shape kbstore's old dbtx and
// execer interfaces had, now a shared type so every ported package's
// dbtx-parameterized helpers (a read that runs identically on the pool or
// inside an already-open caller transaction) take the same interface.
type DBTX interface {
	Query(ctx context.Context, query string, args ...any) (*Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *Row
	Exec(ctx context.Context, query string, args ...any) (CommandTag, error)
}

var (
	_ DBTX = (*DB)(nil)
	_ DBTX = (*Tx)(nil)
)

// IsSingleProcessLockErr reports whether err is the "already open by
// another process" error Open returns when it fails to acquire the
// single-process flock. Exists so callers (e.g. a CLI's error message) can
// give the user a specific, actionable message instead of a generic open
// failure.
func IsSingleProcessLockErr(err error) bool {
	return err != nil && errors.Is(err, errSingleProcessLock)
}

var errSingleProcessLock = errors.New("dbx: database is already open by another process")

// openPostgres shares one bounded pool per connection string. Error messages
// deliberately omit the connection string, which can contain credentials.
func openPostgres(ctx context.Context, dsn string) (*DB, error) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if s, ok := registry[dsn]; ok {
		s.refCount++
		return s.db, nil
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("dbx: invalid PostgreSQL connection string")
	}
	cfg.RuntimeParams["timezone"] = "UTC"
	sdb := stdlib.OpenDB(*cfg)
	sdb.SetMaxOpenConns(8)
	sdb.SetMaxIdleConns(2)
	if err := sdb.PingContext(ctx); err != nil {
		_ = sdb.Close()
		return nil, fmt.Errorf("dbx: PostgreSQL connection failed: %w", err)
	}
	db := &DB{path: dsn, sdb: sdb, dialect: Postgres}
	registry[dsn] = &sharedDB{db: db, refCount: 1}
	return db, nil
}
