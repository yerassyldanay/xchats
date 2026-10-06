package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
)

// defaultAdminID is the fixed sentinel user id seeded by
// migrations/20261006000001_identity_access.sql.
var defaultAdminID = uuid.MustParse("00000000-0000-0000-0000-000000000002")

// TestFreshDatabaseHasOnlyTheDefaultAdmin proves store.New's migration step
// (internal/dbx.RunMigrations over migrations.FS) leaves a brand-new
// database with exactly the required initial state — the default
// organization and admin user from 20261006000001_identity_access.sql — and
// nothing else: no stray seed data, no accounts. internal/dbtest's own
// TestSchemaContract and TestInitAdminMigration cover the schema shape and
// the seed rows in detail.
func TestFreshDatabaseHasOnlyTheDefaultAdmin(t *testing.T) {
	st, db := dbtest.Open(t)
	ctx := context.Background()

	orgs, err := st.OrgsForUser(ctx, defaultAdminID)
	if err != nil {
		t.Fatalf("OrgsForUser(admin): %v", err)
	}
	if len(orgs) != 1 {
		t.Fatalf("admin belongs to %d orgs, want 1 (the default org seeded by 20261006000001_identity_access.sql)", len(orgs))
	}

	var accountCount int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM wa_accounts`).Scan(&accountCount); err != nil {
		t.Fatalf("count wa_accounts: %v", err)
	}
	if accountCount != 0 {
		t.Fatalf("a fresh database has %d wa_accounts rows, want 0 — WA account bootstrap must never run outside the HTTP connect path", accountCount)
	}
}

// TestWaAccountChannelDefaultsToWhatsApp proves the channel column's default
// applies to an insert that omits it — every legacy write path.
func TestWaAccountChannelDefaultsToWhatsApp(t *testing.T) {
	now := time.Now()

	st, db := dbtest.Open(t)
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, "channel-default-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	var channel string
	err = db.QueryRow(ctx, `
		INSERT INTO wa_accounts (id, organization_id, display_name, owner_jid, connection_state, created_at, updated_at)
		VALUES ('11111111-1111-1111-1111-111111111111', $1, 'x', 'unspecified-channel-jid@s.whatsapp.net', 'connected', $2, $2)
		RETURNING channel`, org.ID, now).Scan(&channel)
	if err != nil {
		t.Fatalf("insert account without channel: %v", err)
	}
	if channel != "whatsapp" {
		t.Fatalf("channel default = %q, want whatsapp", channel)
	}
}
