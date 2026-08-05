package dbtest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// New returns a fresh, migrated *store.Store at t.TempDir() — the default
// entry point for any repository package's tests. The default organization
// and admin user already exist (migration 0006_init_admin); callers seed
// only their own per-test extras via the ordinary Store API (SeedOrganization,
// SeedAccount, ...).
func New(t testing.TB) *store.Store {
	t.Helper()
	st, _ := Open(t)
	return st
}

// Open is New plus the raw *dbx.DB backing the same database — for the rare
// fixture or assertion that needs a query no exported Store method covers
// (store.Store deliberately exposes no public DB accessor; dbtest, inside
// the persistence boundary, is where that escape hatch lives instead of
// consumer packages reaching for one). dbx.Open's path-based sharing (see
// its package doc) means db and the *dbx.DB inside st are the SAME
// connection, not a second one racing it.
func Open(t testing.TB) (*store.Store, *dbx.DB) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "xchats.db")

	db, err := dbx.Open(ctx, path)
	if err != nil {
		t.Fatalf("dbtest: open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("dbtest: close raw handle: %v", err)
		}
	})

	st, err := store.New(ctx, path)
	if err != nil {
		t.Fatalf("dbtest: store.New: %v", err)
	}
	t.Cleanup(st.Close)

	return st, db
}
