package dbtest

import (
	"context"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// TestEnumChecksEnforced behaviorally verifies the four enum-shaped CHECK
// constraints translated from Postgres's "= ANY (ARRAY[...])"/literal-equality
// checks (see migrations/sqlite/000{2,3,5}_*.up.sql and the per-PG-ism
// translation table) actually reject an out-of-vocabulary value — the
// schema contract test deliberately does not compare CHECK definitions as
// text (they read completely differently: IN (...) vs = ANY(ARRAY[...])),
// so this is what actually pins their behavior.
func TestEnumChecksEnforced(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()

	mustExec(t, db, ctx, `INSERT INTO organizations (id, name) VALUES ('11111111-1111-1111-1111-111111111111', 'acme')`)

	t.Run("wa_accounts.channel", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO wa_accounts (id, owner_jid, channel) VALUES ('a1', 'jid1', 'whatsapp')`)
		mustExec(t, db, ctx, `INSERT INTO wa_accounts (id, owner_jid, channel) VALUES ('a2', 'jid2', 'simulator')`)
		mustReject(t, db, ctx, `INSERT INTO wa_accounts (id, owner_jid, channel) VALUES ('a3', 'jid3', 'telegram')`)
	})

	t.Run("ai_delivery_zones.zone_level", func(t *testing.T) {
		for _, level := range []string{"city", "region", "country"} {
			mustExec(t, db, ctx, `INSERT INTO ai_delivery_zones
				(organization_id, ref, zone_level, delivery_available)
				VALUES ('11111111-1111-1111-1111-111111111111', $1, $1, 1)`, level)
		}
		mustReject(t, db, ctx, `INSERT INTO ai_delivery_zones
			(organization_id, ref, zone_level, delivery_available)
			VALUES ('11111111-1111-1111-1111-111111111111', 'bad', 'planet', 1)`)
	})

	t.Run("mcp_oauth_clients.registration_source", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO mcp_oauth_clients (client_id, registration_source) VALUES ('c1', 'dcr')`)
		mustExec(t, db, ctx, `INSERT INTO mcp_oauth_clients (client_id, registration_source) VALUES ('c2', 'cimd')`)
		mustReject(t, db, ctx, `INSERT INTO mcp_oauth_clients (client_id, registration_source) VALUES ('c3', 'other')`)
	})

	t.Run("organization_users.role", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u2', 'u2@example.com', 'x')`)
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u3', 'u3@example.com', 'x')`)
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u4', 'u4@example.com', 'x')`)
		mustExec(t, db, ctx, `INSERT INTO organization_users (organization_id, user_id, role) VALUES ('11111111-1111-1111-1111-111111111111', 'u2', 'admin')`)
		mustExec(t, db, ctx, `INSERT INTO organization_users (organization_id, user_id, role) VALUES ('11111111-1111-1111-1111-111111111111', 'u3', 'member')`)
		mustReject(t, db, ctx, `INSERT INTO organization_users (organization_id, user_id, role) VALUES ('11111111-1111-1111-1111-111111111111', 'u4', 'owner')`)
	})

	t.Run("mcp_authorization_codes.code_challenge_method", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u1', 'u1@example.com', 'x')`)
		mustExec(t, db, ctx, `INSERT INTO mcp_oauth_clients (client_id) VALUES ('c4')`)
		mustExec(t, db, ctx, `INSERT INTO mcp_authorization_codes
			(code_hash, client_id, redirect_uri, code_challenge, code_challenge_method, user_id, organization_id, expires_at)
			VALUES ('h1', 'c4', 'https://example.com/cb', 'challenge', 'S256', 'u1', '11111111-1111-1111-1111-111111111111', $1)`,
			"2030-01-01 00:00:00.000")
		mustReject(t, db, ctx, `INSERT INTO mcp_authorization_codes
			(code_hash, client_id, redirect_uri, code_challenge, code_challenge_method, user_id, organization_id, expires_at)
			VALUES ('h2', 'c4', 'https://example.com/cb', 'challenge', 'plain', 'u1', '11111111-1111-1111-1111-111111111111', $1)`,
			"2030-01-01 00:00:00.000")
	})
}

func mustExec(t *testing.T, db *dbx.DB, ctx context.Context, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(ctx, query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func mustReject(t *testing.T, db *dbx.DB, ctx context.Context, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(ctx, query, args...); err == nil {
		t.Fatalf("exec %q unexpectedly succeeded — CHECK constraint did not reject it", query)
	}
}
