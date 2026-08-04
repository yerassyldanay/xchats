package dbtest

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// NewKB returns a fresh, migrated *kbstore.Store sharing the SAME database
// as a *store.Store and the raw *dbx.DB (see internal/dbx.Open's path-based
// sharing). kbstore tests need all three: kb to exercise the package under
// test, st to seed organizations/users (kbstore has no such concept of its
// own — every kbstore row is keyed on an orgID a caller already has), and db
// for assertions no exported Store method covers.
func NewKB(t *testing.T) (*kbstore.Store, *store.Store, *dbx.DB) {
	t.Helper()
	st, db := Open(t)
	kb, err := kbstore.New(context.Background(), db.Path())
	if err != nil {
		t.Fatalf("dbtest: kbstore.New: %v", err)
	}
	t.Cleanup(kb.Close)
	return kb, st, db
}
