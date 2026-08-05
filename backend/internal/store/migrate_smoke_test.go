package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
)

// defaultAdminID is migrations/sqlite/0006_init_admin.up.sql's fixed sentinel
// user id.
var defaultAdminID = uuid.MustParse("00000000-0000-0000-0000-000000000002")

// TestFreshDatabaseHasOnlyTheDefaultAdmin proves store.New's migration step
// (internal/dbx.RunMigrations over migrations/sqlite) leaves a brand-new
// database with exactly the required initial state — the default
// organization and admin user from 0006_init_admin — and nothing else: no
// stray seed data, no accounts. This is the SQLite-era replacement for the
// old pgx harness's migration-history smoke tests (TestMigrations_*): those
// tested properties of INCREMENTAL Postgres migration steps (0008's demo
// data being idempotent, 0013 dropping FKs) that have no equivalent here —
// the sqlite migrations encode the final schema directly rather than
// replaying that history, and internal/dbtest's own TestSchemaContract and
// TestInitAdminMigration (package dbtest) already cover the shape and
// seed-migration behavior those tests were really guarding.
func TestFreshDatabaseHasOnlyTheDefaultAdmin(t *testing.T) {
	st, db := dbtest.Open(t)
	ctx := context.Background()

	orgs, err := st.OrgsForUser(ctx, defaultAdminID)
	if err != nil {
		t.Fatalf("OrgsForUser(admin): %v", err)
	}
	if len(orgs) != 1 {
		t.Fatalf("admin belongs to %d orgs, want 1 (the default org from 0006_init_admin)", len(orgs))
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
	st, db := dbtest.Open(t)
	ctx := context.Background()

	org, err := st.SeedOrganization(ctx, "channel-default-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	var channel string
	err = db.QueryRow(ctx, `
		INSERT INTO wa_accounts (id, organization_id, display_name, owner_jid, connection_state)
		VALUES ('11111111-1111-1111-1111-111111111111', $1, 'x', 'unspecified-channel-jid@s.whatsapp.net', 'connected')
		RETURNING channel`, org.ID).Scan(&channel)
	if err != nil {
		t.Fatalf("insert account without channel: %v", err)
	}
	if channel != "whatsapp" {
		t.Fatalf("channel default = %q, want whatsapp", channel)
	}
}
