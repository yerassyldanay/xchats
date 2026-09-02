package dbtest

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/chatstore"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// NewChat returns a fresh, migrated *chatstore.Store sharing the SAME
// database as a *store.Store and the raw *dbx.DB — the same three-handle
// shape NewKB has, and for the same reason: chatstore has no concept of
// organizations or users of its own, so a test needs st to seed the
// (organization, user) scope every chat operation is keyed by.
func NewChat(t testing.TB) (*chatstore.Store, *store.Store, *dbx.DB) {
	t.Helper()
	st, db := Open(t)
	cs, err := chatstore.New(context.Background(), db.Path())
	if err != nil {
		t.Fatalf("dbtest: chatstore.New: %v", err)
	}
	t.Cleanup(cs.Close)
	return cs, st, db
}
