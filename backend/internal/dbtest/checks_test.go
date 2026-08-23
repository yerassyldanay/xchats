package dbtest

import (
	"context"
	"strconv"
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

	t.Run("meta_oauth_states.channel", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u5', 'u5@example.com', 'x')`)
		for i, ch := range []string{"instagram", "messenger", "whatsapp_cloud"} {
			mustExec(t, db, ctx, `INSERT INTO meta_oauth_states
				(state, channel, organization_id, user_id, expires_at)
				VALUES ($1, $2, '11111111-1111-1111-1111-111111111111', 'u5', '2030-01-01 00:00:00.000')`,
				"state-channel-"+strconv.Itoa(i), ch)
		}
		mustReject(t, db, ctx, `INSERT INTO meta_oauth_states
			(state, channel, organization_id, user_id, expires_at)
			VALUES ('state-channel-bad', 'discord', '11111111-1111-1111-1111-111111111111', 'u5', '2030-01-01 00:00:00.000')`)
	})

	t.Run("meta_oauth_states.status", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u6', 'u6@example.com', 'x')`)
		for i, status := range []string{"pending", "needs_selection", "connected", "failed", "expired"} {
			mustExec(t, db, ctx, `INSERT INTO meta_oauth_states
				(state, channel, organization_id, user_id, status, expires_at)
				VALUES ($1, 'instagram', '11111111-1111-1111-1111-111111111111', 'u6', $2, '2030-01-01 00:00:00.000')`,
				"state-status-"+strconv.Itoa(i), status)
		}
		mustReject(t, db, ctx, `INSERT INTO meta_oauth_states
			(state, channel, organization_id, user_id, status, expires_at)
			VALUES ('state-status-bad', 'instagram', '11111111-1111-1111-1111-111111111111', 'u6', 'bogus', '2030-01-01 00:00:00.000')`)
	})

	t.Run("meta_webhook_events.channel", func(t *testing.T) {
		for i, ch := range []string{"instagram", "messenger", "whatsapp_cloud"} {
			mustExec(t, db, ctx, `INSERT INTO meta_webhook_events
				(channel, event_key, outcome) VALUES ($1, $2, 'stored')`,
				ch, "event-channel-"+strconv.Itoa(i))
		}
		mustReject(t, db, ctx, `INSERT INTO meta_webhook_events
			(channel, event_key, outcome) VALUES ('discord', 'event-channel-bad', 'stored')`)
	})

	t.Run("meta_webhook_events.outcome", func(t *testing.T) {
		for i, outcome := range []string{"stored", "duplicate", "ignored", "unknown_account", "bad_signature", "error"} {
			mustExec(t, db, ctx, `INSERT INTO meta_webhook_events
				(channel, event_key, outcome) VALUES ('instagram', $1, $2)`,
				"event-outcome-"+strconv.Itoa(i), outcome)
		}
		mustReject(t, db, ctx, `INSERT INTO meta_webhook_events
			(channel, event_key, outcome) VALUES ('instagram', 'event-outcome-bad', 'bogus')`)
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

	t.Run("campaigns.channel", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u-camp-1', 'u-camp-1@example.com', 'x')`)
		for i, ch := range []string{"whatsapp", "simulator", "telegram", "instagram", "messenger", "whatsapp_cloud"} {
			mustExec(t, db, ctx, `INSERT INTO campaigns
				(id, organization_id, name, account_id, channel, created_by)
				VALUES ($1, '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', $2, 'u-camp-1')`,
				"camp-channel-"+strconv.Itoa(i), ch)
		}
		mustReject(t, db, ctx, `INSERT INTO campaigns
			(id, organization_id, name, account_id, channel, created_by)
			VALUES ('camp-channel-bad', '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', 'discord', 'u-camp-1')`)
	})

	t.Run("campaigns.status", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u-camp-2', 'u-camp-2@example.com', 'x')`)
		for i, status := range []string{"draft", "scheduled", "running", "paused", "completed", "failed", "cancelled"} {
			mustExec(t, db, ctx, `INSERT INTO campaigns
				(id, organization_id, name, account_id, channel, status, created_by)
				VALUES ($1, '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', 'whatsapp', $2, 'u-camp-2')`,
				"camp-status-"+strconv.Itoa(i), status)
		}
		mustReject(t, db, ctx, `INSERT INTO campaigns
			(id, organization_id, name, account_id, channel, status, created_by)
			VALUES ('camp-status-bad', '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', 'whatsapp', 'active', 'u-camp-2')`)
	})

	t.Run("campaign_recipients.status", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u-camp-3', 'u-camp-3@example.com', 'x')`)
		mustExec(t, db, ctx, `INSERT INTO campaigns
			(id, organization_id, name, account_id, channel, created_by)
			VALUES ('camp-for-recipients', '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', 'whatsapp', 'u-camp-3')`)
		for i, status := range []string{"pending", "sending", "sent", "failed", "skipped"} {
			mustExec(t, db, ctx, `INSERT INTO campaign_recipients
				(campaign_id, normalized_identity, status) VALUES ('camp-for-recipients', $1, $2)`,
				"7700000"+strconv.Itoa(i), status)
		}
		mustReject(t, db, ctx, `INSERT INTO campaign_recipients
			(campaign_id, normalized_identity, status) VALUES ('camp-for-recipients', '77000009', 'queued')`)
	})

	t.Run("campaign_account_settings.limit_mode", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO campaign_account_settings (account_id, limit_mode) VALUES ('acct-mode-1', 'default')`)
		mustExec(t, db, ctx, `INSERT INTO campaign_account_settings (account_id, limit_mode) VALUES ('acct-mode-2', 'custom')`)
		mustReject(t, db, ctx, `INSERT INTO campaign_account_settings (account_id, limit_mode) VALUES ('acct-mode-3', 'unlimited')`)
	})

	t.Run("campaign_send_log.outcome and origin", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('u-camp-4', 'u-camp-4@example.com', 'x')`)
		mustExec(t, db, ctx, `INSERT INTO campaigns
			(id, organization_id, name, account_id, channel, created_by)
			VALUES ('camp-for-log', '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', 'whatsapp', 'u-camp-4')`)
		mustExec(t, db, ctx, `INSERT INTO campaign_recipients
			(id, campaign_id, normalized_identity) VALUES ('recip-for-log', 'camp-for-log', '77000001')`)
		for _, outcome := range []string{"sent", "failed"} {
			mustExec(t, db, ctx, `INSERT INTO campaign_send_log
				(account_id, campaign_id, recipient_id, outcome) VALUES ('acct-1', 'camp-for-log', 'recip-for-log', $1)`,
				outcome)
		}
		mustReject(t, db, ctx, `INSERT INTO campaign_send_log
			(account_id, campaign_id, recipient_id, outcome) VALUES ('acct-1', 'camp-for-log', 'recip-for-log', 'queued')`)
		for _, origin := range []string{"campaign", "manual", "ai"} {
			mustExec(t, db, ctx, `INSERT INTO campaign_send_log
				(account_id, campaign_id, recipient_id, outcome, origin) VALUES ('acct-1', 'camp-for-log', 'recip-for-log', 'sent', $1)`,
				origin)
		}
		mustReject(t, db, ctx, `INSERT INTO campaign_send_log
			(account_id, campaign_id, recipient_id, outcome, origin) VALUES ('acct-1', 'camp-for-log', 'recip-for-log', 'sent', 'automation')`)
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
