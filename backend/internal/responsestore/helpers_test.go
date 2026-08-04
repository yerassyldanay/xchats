package responsestore_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// newTestStore opens a fresh, migrated SQLite database and seeds one
// organization + one WhatsApp account — the newHarness pattern shared by
// the backend's other test suites (internal/kbstore/kbstore_test.go,
// internal/mcpauth/mcpauth_test.go).
func newTestStore(t *testing.T) (*store.Store, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	st := dbtest.New(t)
	org, err := st.SeedOrganization(ctx, "xchats-test")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	ownerJID := "77011111111@s.whatsapp.net"
	accountID := config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "WhatsApp", ExternalAccountRef: config.CanonicalJID(ownerJID),
		ExternalHandle: config.PhoneFromJID(ownerJID), InstanceName: "xpayment", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return st, org.ID, accountID
}

func mustExec(t testing.TB, db *dbx.DB, sql string, args ...any) {
	t.Helper()
	if _, err := db.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", s, err)
	}
	return id
}
