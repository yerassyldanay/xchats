package dbtest

import (
	"context"
	"errors"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

// openPreCampaignDiagnostics opens a fresh database migrated through every
// file EXCEPT 0020_campaign_diagnostics.up.sql — the pre-migration schema
// shape (no campaign_send_attempts/wa_account_connection_events tables, and
// campaign_events/campaign_recipients without this migration's additions),
// mirroring openPreKBGapTelemetry's pattern for 0018.
func openPreCampaignDiagnostics(t testing.TB) *dbx.DB {
	t.Helper()
	db := OpenRawEmpty(t)
	pre := fsWithout(t, sqlitemigrations.FS, "0020_campaign_diagnostics.up.sql")
	if err := dbx.RunMigrations(context.Background(), db, pre); err != nil {
		t.Fatalf("migrate (pre-0020): %v", err)
	}
	return db
}

func applyCampaignDiagnosticsMigration(t testing.TB, db *dbx.DB) {
	t.Helper()
	if err := dbx.RunMigrations(context.Background(), db, sqlitemigrations.FS); err != nil {
		t.Fatalf("migrate (0020): %v", err)
	}
}

const diagTestOrgSQL = `INSERT INTO organizations (id, name) VALUES ('44444444-4444-4444-4444-444444444444', 'Diagnostics Test Org')`

// TestMigration0020_UpgradesADeployedDatabase proves the upgrade path: a
// database already carrying a real campaign/recipient/event from before this
// migration must gain the new tables/columns, keep every existing row
// completely untouched, and accept a new attempt row referencing that
// pre-existing recipient immediately afterward.
func TestMigration0020_UpgradesADeployedDatabase(t *testing.T) {
	db := openPreCampaignDiagnostics(t)
	ctx := context.Background()

	mustExec(t, db, ctx, diagTestOrgSQL)
	mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('55555555-5555-5555-5555-555555555555', 'diag@example.com', 'x')`)
	var campaignID string
	if err := db.QueryRow(ctx, `INSERT INTO campaigns (organization_id, name, account_id, channel, created_by)
		VALUES ('44444444-4444-4444-4444-444444444444', 'Pre-0020 campaign', 'acct-1', 'whatsapp', '55555555-5555-5555-5555-555555555555')
		RETURNING id`).Scan(&campaignID); err != nil {
		t.Fatalf("insert pre-existing campaign: %v", err)
	}
	var recipientID string
	if err := db.QueryRow(ctx, `INSERT INTO campaign_recipients (campaign_id, normalized_identity)
		VALUES ($1, '77000000001') RETURNING id`, campaignID).Scan(&recipientID); err != nil {
		t.Fatalf("insert pre-existing recipient: %v", err)
	}
	var eventID string
	if err := db.QueryRow(ctx, `INSERT INTO campaign_events (campaign_id, event) VALUES ($1, 'started') RETURNING id`, campaignID).Scan(&eventID); err != nil {
		t.Fatalf("insert pre-existing event: %v", err)
	}

	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('campaign_send_attempts','wa_account_connection_events')`).Scan(&n); err != nil {
		t.Fatalf("sqlite_master (pre-migration): %v", err)
	}
	if n != 0 {
		t.Fatalf("diagnostics tables already exist before 0020 ran — test setup is wrong")
	}

	applyCampaignDiagnosticsMigration(t, db)

	if err := db.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('campaign_send_attempts','wa_account_connection_events')`).Scan(&n); err != nil {
		t.Fatalf("sqlite_master (post-migration): %v", err)
	}
	if n != 2 {
		t.Fatalf("expected both new tables after 0020, sqlite_master matched %d", n)
	}

	// Pre-existing rows survived untouched.
	var identity string
	if err := db.QueryRow(ctx, `SELECT normalized_identity FROM campaign_recipients WHERE id = $1`, recipientID).Scan(&identity); err != nil {
		t.Fatalf("read back pre-existing recipient: %v", err)
	}
	if identity != "77000000001" {
		t.Errorf("normalized_identity = %q, want unchanged", identity)
	}
	var event string
	if err := db.QueryRow(ctx, `SELECT event FROM campaign_events WHERE id = $1`, eventID).Scan(&event); err != nil {
		t.Fatalf("read back pre-existing event: %v", err)
	}
	if event != "started" {
		t.Errorf("event = %q, want unchanged", event)
	}

	// The new FK resolves against a recipient/campaign that predate the migration.
	var attemptID string
	if err := db.QueryRow(ctx, `INSERT INTO campaign_send_attempts
		(organization_id, campaign_id, campaign_recipient_id, account_id, attempt_number, started_at)
		VALUES ('44444444-4444-4444-4444-444444444444', $1, $2, 'acct-1', 1, strftime('%Y-%m-%d %H:%M:%f','now'))
		RETURNING id`, campaignID, recipientID).Scan(&attemptID); err != nil {
		t.Fatalf("insert attempt referencing pre-existing campaign/recipient: %v", err)
	}
	if attemptID == "" {
		t.Fatal("expected a generated attempt id")
	}

	// A diagnostic event can now reference that attempt, on the pre-existing campaign_events table.
	mustExec(t, db, ctx, `INSERT INTO campaign_events (campaign_id, event, campaign_recipient_id, attempt_id)
		VALUES ($1, 'send_attempt_started', $2, $3)`, campaignID, recipientID, attemptID)
}

// TestMigration0020_AttemptDefaultsAndChecks pins the new table's defaults
// (outcome starts 'sending', error_detail empty) and its two CHECK
// constraints (attempt_number > 0, outcome in the closed four-value set).
func TestMigration0020_AttemptDefaultsAndChecks(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	campaignID, recipientID := seedDiagCampaign(t, db, ctx)

	var outcome, errDetail string
	if err := db.QueryRow(ctx, `INSERT INTO campaign_send_attempts
		(organization_id, campaign_id, campaign_recipient_id, account_id, attempt_number, started_at)
		VALUES ('44444444-4444-4444-4444-444444444444', $1, $2, 'acct-1', 1, strftime('%Y-%m-%d %H:%M:%f','now'))
		RETURNING outcome, error_detail`, campaignID, recipientID).Scan(&outcome, &errDetail); err != nil {
		t.Fatalf("insert with only required columns: %v", err)
	}
	if outcome != "sending" {
		t.Errorf("outcome default = %q, want sending", outcome)
	}
	if errDetail != "" {
		t.Errorf("error_detail default = %q, want empty", errDetail)
	}

	if _, err := db.Exec(ctx, `INSERT INTO campaign_send_attempts
		(organization_id, campaign_id, campaign_recipient_id, account_id, attempt_number, started_at, outcome)
		VALUES ('44444444-4444-4444-4444-444444444444', $1, $2, 'acct-1', 1, strftime('%Y-%m-%d %H:%M:%f','now'), 'bogus')`,
		campaignID, recipientID); err == nil {
		t.Fatal("expected an out-of-vocabulary outcome to be rejected by the CHECK constraint")
	}

	if _, err := db.Exec(ctx, `INSERT INTO campaign_send_attempts
		(organization_id, campaign_id, campaign_recipient_id, account_id, attempt_number, started_at)
		VALUES ('44444444-4444-4444-4444-444444444444', $1, $2, 'acct-1', 0, strftime('%Y-%m-%d %H:%M:%f','now'))`,
		campaignID, recipientID); err == nil {
		t.Fatal("expected attempt_number = 0 to be rejected by the CHECK constraint")
	}
}

// TestMigration0020_AttemptsCascadeWithRecipientAndCampaign confirms attempts
// are removed when their recipient (or, transitively, campaign) is deleted —
// this table must never outlive the recipient it traces, unlike
// campaign_events which is a durable, independent audit log.
func TestMigration0020_AttemptsCascadeWithRecipientAndCampaign(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	campaignID, recipientID := seedDiagCampaign(t, db, ctx)
	mustExec(t, db, ctx, `INSERT INTO campaign_send_attempts
		(organization_id, campaign_id, campaign_recipient_id, account_id, attempt_number, started_at)
		VALUES ('44444444-4444-4444-4444-444444444444', $1, $2, 'acct-1', 1, strftime('%Y-%m-%d %H:%M:%f','now'))`,
		campaignID, recipientID)

	mustExec(t, db, ctx, `DELETE FROM campaigns WHERE id = $1`, campaignID)

	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM campaign_send_attempts WHERE campaign_id = $1`, campaignID).Scan(&n); err != nil {
		t.Fatalf("count attempts after campaign delete: %v", err)
	}
	if n != 0 {
		t.Errorf("expected ON DELETE CASCADE from campaigns to remove the attempt, found %d", n)
	}
}

// TestMigration0020_CampaignEventsNewColumnsAreOptional confirms a plain
// campaign-lifecycle event (the only shape that existed before this
// migration) still inserts with every new column left NULL.
func TestMigration0020_CampaignEventsNewColumnsAreOptional(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	campaignID, _ := seedDiagCampaign(t, db, ctx)

	var recipID, attemptID, acctID, chatID, msgID, errCode *string
	if err := db.QueryRow(ctx, `INSERT INTO campaign_events (campaign_id, event) VALUES ($1, 'paused')
		RETURNING campaign_recipient_id, attempt_id, account_id, chat_id, message_id, error_code`, campaignID).
		Scan(&recipID, &attemptID, &acctID, &chatID, &msgID, &errCode); err != nil {
		t.Fatalf("insert lifecycle-only event: %v", err)
	}
	if recipID != nil || attemptID != nil || acctID != nil || chatID != nil || msgID != nil || errCode != nil {
		t.Errorf("expected every new campaign_events column to default to NULL, got %v %v %v %v %v %v",
			recipID, attemptID, acctID, chatID, msgID, errCode)
	}
}

// TestMigration0020_ConnectionEventsCascadeAndSetNull pins the two different
// delete behaviors: deleting the WhatsApp account removes its connection
// history (it is meaningless without the account); deleting the organization
// only clears the denormalized organization_id link, since
// wa_accounts.organization_id itself is ON DELETE SET NULL, not CASCADE.
func TestMigration0020_ConnectionEventsCascadeAndSetNull(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, diagTestOrgSQL)
	var accountID string
	if err := db.QueryRow(ctx, `INSERT INTO wa_accounts (id, organization_id, owner_jid)
		VALUES ('88888888-8888-8888-8888-888888888888', '44444444-4444-4444-4444-444444444444', 'owner-1@s.whatsapp.net')
		RETURNING id`).Scan(&accountID); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	var eventID string
	if err := db.QueryRow(ctx, `INSERT INTO wa_account_connection_events (account_id, organization_id, event, reason)
		VALUES ($1, '44444444-4444-4444-4444-444444444444', 'disconnected', 'stream replaced by another connection')
		RETURNING id`, accountID).Scan(&eventID); err != nil {
		t.Fatalf("insert connection event: %v", err)
	}

	mustExec(t, db, ctx, `DELETE FROM organizations WHERE id = '44444444-4444-4444-4444-444444444444'`)
	var orgID *string
	if err := db.QueryRow(ctx, `SELECT organization_id FROM wa_account_connection_events WHERE id = $1`, eventID).Scan(&orgID); err != nil {
		t.Fatalf("read back event after org delete: %v", err)
	}
	if orgID != nil {
		t.Errorf("organization_id = %v, want NULL after the organization was deleted", *orgID)
	}
	// wa_accounts.organization_id is itself ON DELETE SET NULL, so the
	// account row (and this event) must still exist.
	var stillThere int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM wa_account_connection_events WHERE id = $1`, eventID).Scan(&stillThere); err != nil {
		t.Fatalf("count event after org delete: %v", err)
	}
	if stillThere != 1 {
		t.Fatalf("expected the connection event to survive the organization delete, found %d", stillThere)
	}

	mustExec(t, db, ctx, `DELETE FROM wa_accounts WHERE id = $1`, accountID)
	var n int
	err := db.QueryRow(ctx, `SELECT count(*) FROM wa_account_connection_events WHERE id = $1`, eventID).Scan(&n)
	if err != nil && !errors.Is(err, dbx.ErrNoRows) {
		t.Fatalf("count event after account delete: %v", err)
	}
	if n != 0 {
		t.Errorf("expected ON DELETE CASCADE from wa_accounts to remove the connection event, found %d", n)
	}
}

// TestMigration0020_RecipientChatAndMessageIndexesArePartial confirms the two
// new indexes exist and only cover populated rows (a NULL chat_id/message_id
// — the common case for a not-yet-processed or permanently-failed-before-a-
// chat-existed recipient — is not indexed).
func TestMigration0020_RecipientChatAndMessageIndexesArePartial(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()

	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM pragma_index_list('campaign_recipients')
		WHERE name IN ('campaign_recipients_chat_idx', 'campaign_recipients_message_idx') AND partial = 1`).Scan(&n); err != nil {
		t.Fatalf("pragma_index_list: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected both new indexes to exist and be partial, matched %d", n)
	}
}

// seedDiagCampaign inserts a minimal org/user/campaign/recipient chain for
// tests that only care about campaign_send_attempts/campaign_events, not the
// upgrade path itself.
func seedDiagCampaign(t *testing.T, db *dbx.DB, ctx context.Context) (campaignID, recipientID string) {
	t.Helper()
	mustExec(t, db, ctx, diagTestOrgSQL)
	mustExec(t, db, ctx, `INSERT INTO users (id, email, password_hash) VALUES ('66666666-6666-6666-6666-666666666666', 'diag2@example.com', 'x')`)
	if err := db.QueryRow(ctx, `INSERT INTO campaigns (organization_id, name, account_id, channel, created_by)
		VALUES ('44444444-4444-4444-4444-444444444444', 'Diag campaign', 'acct-1', 'whatsapp', '66666666-6666-6666-6666-666666666666')
		RETURNING id`).Scan(&campaignID); err != nil {
		t.Fatalf("insert campaign: %v", err)
	}
	if err := db.QueryRow(ctx, `INSERT INTO campaign_recipients (campaign_id, normalized_identity)
		VALUES ($1, '77000000002') RETURNING id`, campaignID).Scan(&recipientID); err != nil {
		t.Fatalf("insert recipient: %v", err)
	}
	return campaignID, recipientID
}
