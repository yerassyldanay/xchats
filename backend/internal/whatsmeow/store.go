package whatsmeow

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	// The pure-Go SQLite driver — registered under "sqlite", matching
	// internal/dbx's own choice (no CGO). This is the one place outside
	// internal/dbx allowed to import it; see internal/dbtest's architecture
	// test for why whatsmeow's own device-session schema is a deliberate
	// exception to the "only dbx touches the database driver" rule.
	_ "modernc.org/sqlite"

	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// openDeviceStore opens (creating if needed) whatsmeow's own device-session
// SQLite database at path and applies ITS schema via sqlstore's own
// Upgrade() migrator. This is a separate file from xchats.db on purpose:
// whatsmeow's sqlstore owns this schema entirely, so it must never share a
// connection pool or migration runner with internal/dbx.
func openDeviceStore(ctx context.Context, path string, log waLog.Logger) (*sqlstore.Container, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: resolve device db path %q: %w", path, err)
	}
	if dir := filepath.Dir(abs); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("whatsmeow: create directory for %q: %w", abs, err)
		}
	}

	db, err := sql.Open("sqlite", deviceDSN(abs))
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: open device db %q: %w", abs, err)
	}
	// One physical connection, mirroring internal/dbx's design: every writer
	// serializes through Go's own pool queueing instead of SQLite's
	// busy-retry loop, which is what makes WAL + a single connection safe
	// under whatsmeow's own concurrent session read/writes.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("whatsmeow: ping device db %q: %w", abs, err)
	}

	// "sqlite" (any "sqlite"-prefixed string) selects util/dbutil's SQLite
	// dialect for query generation — it is not a database/sql driver lookup
	// here, db is already open.
	container := sqlstore.NewWithDB(db, "sqlite", log)
	if err := container.Upgrade(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("whatsmeow: upgrade device db schema: %w", err)
	}
	return container, nil
}

// deviceDSN mirrors internal/dbx's own pragma set (WAL, 5s busy timeout,
// synchronous NORMAL, immediate write transactions) plus foreign_keys, which
// sqlstore.Container.Upgrade refuses to proceed without.
func deviceDSN(abs string) string {
	v := url.Values{}
	v.Add("_pragma", "foreign_keys(1)")
	v.Add("_pragma", "journal_mode(WAL)")
	v.Add("_pragma", "busy_timeout(5000)")
	v.Add("_pragma", "synchronous(NORMAL)")
	v.Set("_txlock", "immediate")
	return "file:" + filepath.ToSlash(abs) + "?" + v.Encode()
}
