package dbtest

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// TestEnumChecksEnforced behaviorally verifies the enum-shaped CHECK
// constraints in migrations/*.sql actually reject an out-of-vocabulary
// value. The schema contract test deliberately does not compare CHECK
// definitions as text, so this is what pins their behavior on each engine.
// farFuture is 2030-01-01T00:00:00Z, the expiry the fixtures below use.
var farFuture = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

func TestEnumChecksEnforced(t *testing.T) {
	now := time.Now()

	db := OpenRaw(t)
	ctx := context.Background()

	mustExec(t, db, ctx, `INSERT INTO organizations (id, name, created_at, updated_at) VALUES ('11111111-1111-1111-1111-111111111111', 'acme', $1, $1)`, now)

	t.Run("wa_accounts.channel", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO wa_accounts (id, owner_jid, channel, created_at, updated_at) VALUES ('a1', 'jid1', 'whatsapp', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO wa_accounts (id, owner_jid, channel, created_at, updated_at) VALUES ('a2', 'jid2', 'simulator', $1, $1)`, now)
		mustReject(t, db, ctx, `INSERT INTO wa_accounts (id, owner_jid, channel, created_at, updated_at) VALUES ('a3', 'jid3', 'telegram', $1, $1)`, now)
	})

	t.Run("ai_delivery_zones.zone_level", func(t *testing.T) {
		for _, level := range []string{"city", "region", "country"} {
			mustExec(t, db, ctx, `INSERT INTO ai_delivery_zones
				(organization_id, ref, zone_level, delivery_available, id, created_at, updated_at)
				VALUES ('11111111-1111-1111-1111-111111111111', $1, $1, TRUE, $2, $3, $3)`, level, uuid.New(), now)
		}
		mustReject(t, db, ctx, `INSERT INTO ai_delivery_zones
			(organization_id, ref, zone_level, delivery_available, id, created_at, updated_at)
			VALUES ('11111111-1111-1111-1111-111111111111', 'bad', 'planet', 1, $1, $2, $2)`, uuid.New(), now)
	})

	t.Run("mcp_oauth_clients.registration_source", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO mcp_oauth_clients (client_id, registration_source, created_at, updated_at) VALUES ('c1', 'dcr', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO mcp_oauth_clients (client_id, registration_source, created_at, updated_at) VALUES ('c2', 'cimd', $1, $1)`, now)
		mustReject(t, db, ctx, `INSERT INTO mcp_oauth_clients (client_id, registration_source, created_at, updated_at) VALUES ('c3', 'other', $1, $1)`, now)
	})

	t.Run("organization_users.role", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u2', 'u2@example.com', 'x', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u3', 'u3@example.com', 'x', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u4', 'u4@example.com', 'x', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO organization_users (organization_id, user_id, role, joined_at) VALUES ('11111111-1111-1111-1111-111111111111', 'u2', 'admin', $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO organization_users (organization_id, user_id, role, joined_at) VALUES ('11111111-1111-1111-1111-111111111111', 'u3', 'member', $1)`, now)
		mustReject(t, db, ctx, `INSERT INTO organization_users (organization_id, user_id, role, joined_at) VALUES ('11111111-1111-1111-1111-111111111111', 'u4', 'owner', $1)`, now)
	})

	t.Run("meta_oauth_states.channel", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u5', 'u5@example.com', 'x', $1, $1)`, now)
		for i, ch := range []string{"instagram", "messenger", "whatsapp_cloud"} {
			mustExec(t, db, ctx, `INSERT INTO meta_oauth_states
				(state, channel, organization_id, user_id, expires_at, created_at)
				VALUES ($1, $2, '11111111-1111-1111-1111-111111111111', 'u5', 1893456000000, $3)`,
				"state-channel-"+strconv.Itoa(i), ch, now)
		}
		mustReject(t, db, ctx, `INSERT INTO meta_oauth_states
			(state, channel, organization_id, user_id, expires_at, created_at)
			VALUES ('state-channel-bad', 'discord', '11111111-1111-1111-1111-111111111111', 'u5', 1893456000000, $1)`, now)
	})

	t.Run("meta_oauth_states.status", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u6', 'u6@example.com', 'x', $1, $1)`, now)
		for i, status := range []string{"pending", "needs_selection", "connected", "failed", "expired"} {
			mustExec(t, db, ctx, `INSERT INTO meta_oauth_states
				(state, channel, organization_id, user_id, status, expires_at, created_at)
				VALUES ($1, 'instagram', '11111111-1111-1111-1111-111111111111', 'u6', $2, 1893456000000, $3)`,
				"state-status-"+strconv.Itoa(i), status, now)
		}
		mustReject(t, db, ctx, `INSERT INTO meta_oauth_states
			(state, channel, organization_id, user_id, status, expires_at, created_at)
			VALUES ('state-status-bad', 'instagram', '11111111-1111-1111-1111-111111111111', 'u6', 'bogus', 1893456000000, $1)`, now)
	})

	t.Run("meta_webhook_events.channel", func(t *testing.T) {
		for i, ch := range []string{"instagram", "messenger", "whatsapp_cloud"} {
			mustExec(t, db, ctx, `INSERT INTO meta_webhook_events
				(channel, event_key, outcome, id, received_at) VALUES ($1, $2, 'stored', $3, $4)`,
				ch, "event-channel-"+strconv.Itoa(i), uuid.New(), now)
		}
		mustReject(t, db, ctx, `INSERT INTO meta_webhook_events
			(channel, event_key, outcome, id, received_at) VALUES ('discord', 'event-channel-bad', 'stored', $1, $2)`, uuid.New(), now)
	})

	t.Run("meta_webhook_events.outcome", func(t *testing.T) {
		for i, outcome := range []string{"stored", "duplicate", "ignored", "unknown_account", "bad_signature", "error"} {
			mustExec(t, db, ctx, `INSERT INTO meta_webhook_events
				(channel, event_key, outcome, id, received_at) VALUES ('instagram', $1, $2, $3, $4)`,
				"event-outcome-"+strconv.Itoa(i), outcome, uuid.New(), now)
		}
		mustReject(t, db, ctx, `INSERT INTO meta_webhook_events
			(channel, event_key, outcome, id, received_at) VALUES ('instagram', 'event-outcome-bad', 'bogus', $1, $2)`, uuid.New(), now)
	})

	t.Run("mcp_authorization_codes.code_challenge_method", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u1', 'u1@example.com', 'x', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO mcp_oauth_clients (client_id, created_at, updated_at) VALUES ('c4', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO mcp_authorization_codes
			(code_hash, client_id, redirect_uri, code_challenge, code_challenge_method, user_id, organization_id, expires_at, created_at)
			VALUES ('h1', 'c4', 'https://example.com/cb', 'challenge', 'S256', 'u1', '11111111-1111-1111-1111-111111111111', $1, $2)`,
			farFuture, now)
		mustReject(t, db, ctx, `INSERT INTO mcp_authorization_codes
			(code_hash, client_id, redirect_uri, code_challenge, code_challenge_method, user_id, organization_id, expires_at, created_at)
			VALUES ('h2', 'c4', 'https://example.com/cb', 'challenge', 'plain', 'u1', '11111111-1111-1111-1111-111111111111', $1, $2)`,
			farFuture, now)
	})

	t.Run("campaigns.channel", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u-camp-1', 'u-camp-1@example.com', 'x', $1, $1)`, now)
		for i, ch := range []string{"whatsapp", "simulator", "telegram", "instagram", "messenger", "whatsapp_cloud"} {
			mustExec(t, db, ctx, `INSERT INTO campaigns
				(id, organization_id, name, account_id, channel, created_by, created_at, updated_at)
				VALUES ($1, '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', $2, 'u-camp-1', $3, $3)`,
				"camp-channel-"+strconv.Itoa(i), ch, now)
		}
		mustReject(t, db, ctx, `INSERT INTO campaigns
			(id, organization_id, name, account_id, channel, created_by, created_at, updated_at)
			VALUES ('camp-channel-bad', '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', 'discord', 'u-camp-1', $1, $1)`, now)
	})

	t.Run("campaigns.status", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u-camp-2', 'u-camp-2@example.com', 'x', $1, $1)`, now)
		for i, status := range []string{"draft", "scheduled", "running", "paused", "completed", "failed", "cancelled"} {
			mustExec(t, db, ctx, `INSERT INTO campaigns
				(id, organization_id, name, account_id, channel, status, created_by, created_at, updated_at)
				VALUES ($1, '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', 'whatsapp', $2, 'u-camp-2', $3, $3)`,
				"camp-status-"+strconv.Itoa(i), status, now)
		}
		mustReject(t, db, ctx, `INSERT INTO campaigns
			(id, organization_id, name, account_id, channel, status, created_by, created_at, updated_at)
			VALUES ('camp-status-bad', '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', 'whatsapp', 'active', 'u-camp-2', $1, $1)`, now)
	})

	t.Run("campaign_recipients.status", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u-camp-3', 'u-camp-3@example.com', 'x', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO campaigns
			(id, organization_id, name, account_id, channel, created_by, created_at, updated_at)
			VALUES ('camp-for-recipients', '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', 'whatsapp', 'u-camp-3', $1, $1)`, now)
		for i, status := range []string{"pending", "sending", "sent", "failed", "skipped"} {
			mustExec(t, db, ctx, `INSERT INTO campaign_recipients
				(campaign_id, normalized_identity, status, id, created_at, updated_at) VALUES ('camp-for-recipients', $1, $2, $3, $4, $4)`,
				"7700000"+strconv.Itoa(i), status, uuid.New(), now)
		}
		mustReject(t, db, ctx, `INSERT INTO campaign_recipients
			(campaign_id, normalized_identity, status, id, created_at, updated_at) VALUES ('camp-for-recipients', '77000009', 'queued', $1, $2, $2)`, uuid.New(), now)
	})

	t.Run("campaign_account_settings.limit_mode", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO campaign_account_settings (account_id, limit_mode, created_at, updated_at) VALUES ('acct-mode-1', 'default', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO campaign_account_settings (account_id, limit_mode, created_at, updated_at) VALUES ('acct-mode-2', 'custom', $1, $1)`, now)
		mustReject(t, db, ctx, `INSERT INTO campaign_account_settings (account_id, limit_mode, created_at, updated_at) VALUES ('acct-mode-3', 'unlimited', $1, $1)`, now)
	})

	t.Run("campaign_send_log.outcome and origin", func(t *testing.T) {
		mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ('u-camp-4', 'u-camp-4@example.com', 'x', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO campaigns
			(id, organization_id, name, account_id, channel, created_by, created_at, updated_at)
			VALUES ('camp-for-log', '11111111-1111-1111-1111-111111111111', 'campaign', 'acct-1', 'whatsapp', 'u-camp-4', $1, $1)`, now)
		mustExec(t, db, ctx, `INSERT INTO campaign_recipients
			(id, campaign_id, normalized_identity, created_at, updated_at) VALUES ('recip-for-log', 'camp-for-log', '77000001', $1, $1)`, now)
		for _, outcome := range []string{"sent", "failed"} {
			mustExec(t, db, ctx, `INSERT INTO campaign_send_log
				(account_id, campaign_id, recipient_id, outcome, id, attempted_at) VALUES ('acct-1', 'camp-for-log', 'recip-for-log', $1, $2, $3)`,
				outcome, uuid.New(), now)
		}
		mustReject(t, db, ctx, `INSERT INTO campaign_send_log
			(account_id, campaign_id, recipient_id, outcome, id, attempted_at) VALUES ('acct-1', 'camp-for-log', 'recip-for-log', 'queued', $1, $2)`, uuid.New(), now)
		for _, origin := range []string{"campaign", "manual", "ai"} {
			mustExec(t, db, ctx, `INSERT INTO campaign_send_log
				(account_id, campaign_id, recipient_id, outcome, origin, id, attempted_at) VALUES ('acct-1', 'camp-for-log', 'recip-for-log', 'sent', $1, $2, $3)`,
				origin, uuid.New(), now)
		}
		mustReject(t, db, ctx, `INSERT INTO campaign_send_log
			(account_id, campaign_id, recipient_id, outcome, origin, id, attempted_at) VALUES ('acct-1', 'camp-for-log', 'recip-for-log', 'sent', 'automation', $1, $2)`, uuid.New(), now)
	})
}

func mustExec(t *testing.T, db *dbx.DB, ctx context.Context, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(ctx, query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// mustReject asserts the statement fails on a CHECK constraint specifically, so a
// fixture cannot pass by tripping an unrelated NOT NULL or FOREIGN KEY error.
func mustReject(t *testing.T, db *dbx.DB, ctx context.Context, query string, args ...any) {
	t.Helper()
	_, err := db.Exec(ctx, query, args...)
	if err == nil {
		t.Fatalf("exec %q unexpectedly succeeded — CHECK constraint did not reject it", query)
	}
	if !dbx.IsCheckViolation(err) {
		t.Fatalf("exec %q was rejected, but not by a CHECK constraint: %v", query, err)
	}
}
