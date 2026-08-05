package dbtest

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// NewMCPAuthStore returns a fresh, migrated *mcpauth.Store sharing the SAME
// database as a *store.Store and the raw *dbx.DB (see NewKB, the same shape
// for internal/kbstore). mcpauth tests need st to seed organizations/users
// (mcpauth has no such concept of its own) and build their own
// mcpauth.Authorizer around the returned Store with whatever signing
// key/Config the test needs — those aren't persistence concerns, so this
// helper stops at the Store.
func NewMCPAuthStore(t testing.TB) (*mcpauth.Store, *store.Store, *dbx.DB) {
	t.Helper()
	st, db := Open(t)
	ms, err := mcpauth.NewStore(context.Background(), db.Path())
	if err != nil {
		t.Fatalf("dbtest: mcpauth.NewStore: %v", err)
	}
	t.Cleanup(ms.Close)
	return ms, st, db
}
