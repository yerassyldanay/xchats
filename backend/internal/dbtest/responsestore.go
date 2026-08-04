package dbtest

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// NewKBRepo returns a fresh, migrated *responsestore.KnowledgeBaseRepo
// sharing the SAME database as a *store.Store and the raw *dbx.DB (see
// NewKB/NewMCPAuthStore, the same shape for internal/kbstore/internal/
// mcpauth). responsestore tests need st to seed organizations/accounts and
// db for the direct ai_* table writes KnowledgeBaseRepo itself has no
// exported write path for (it is read-only — see kb.go).
func NewKBRepo(t testing.TB) (*responsestore.KnowledgeBaseRepo, *store.Store, *dbx.DB) {
	t.Helper()
	st, db := Open(t)
	repo, err := responsestore.NewKnowledgeBaseRepo(context.Background(), db.Path())
	if err != nil {
		t.Fatalf("dbtest: responsestore.NewKnowledgeBaseRepo: %v", err)
	}
	t.Cleanup(repo.Close)
	return repo, st, db
}
