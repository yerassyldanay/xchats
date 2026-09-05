package dbtest

import (
	"context"
	"errors"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

// openPreKBGapTelemetry opens a fresh database migrated through every file
// EXCEPT 0018_kb_gap_telemetry.up.sql — the pre-migration schema shape (no
// ai_kb_gap_events/ai_kb_gap_missing_fields tables at all), mirroring
// openPreVirtualFacts's pattern for 0017.
func openPreKBGapTelemetry(t testing.TB) *dbx.DB {
	t.Helper()
	db := OpenRawEmpty(t)
	pre := fsWithout(t, sqlitemigrations.FS, "0018_kb_gap_telemetry.up.sql")
	if err := dbx.RunMigrations(context.Background(), db, pre); err != nil {
		t.Fatalf("migrate (pre-0018): %v", err)
	}
	return db
}

func applyKBGapTelemetryMigration(t testing.TB, db *dbx.DB) {
	t.Helper()
	if err := dbx.RunMigrations(context.Background(), db, sqlitemigrations.FS); err != nil {
		t.Fatalf("migrate (0018): %v", err)
	}
}

const gapTestOrgSQL = `INSERT INTO organizations (id, name) VALUES ('33333333-3333-3333-3333-333333333333', 'Gap Test Org')`

// TestMigration0018_UpgradesADeployedDatabase is the upgrade-path test: a
// database already carrying real ai_drafts/organizations rows from before
// this migration must gain the two new tables, keep its existing data
// completely untouched, and accept a gap event referencing that
// pre-existing data immediately afterward — proving the FKs (organizations,
// ai_drafts) resolve correctly against rows that predate the migration
// itself, not just freshly-inserted ones.
func TestMigration0018_UpgradesADeployedDatabase(t *testing.T) {
	db := openPreKBGapTelemetry(t)
	ctx := context.Background()

	if _, err := db.Exec(ctx, gapTestOrgSQL); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	var draftID string
	if err := db.QueryRow(ctx, `INSERT INTO ai_drafts (chat_id, option_ordinal, draft_text, escalate, escalation_reason)
		VALUES ('chat-pre-0018', 1, 'Секунду, уточню.', 1, 'нет цены') RETURNING id`).Scan(&draftID); err != nil {
		t.Fatalf("insert pre-existing draft: %v", err)
	}

	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('ai_kb_gap_events','ai_kb_gap_missing_fields')`).Scan(&n); err != nil {
		t.Fatalf("sqlite_master (pre-migration): %v", err)
	}
	if n != 0 {
		t.Fatalf("gap tables already exist before 0018 ran — test setup is wrong")
	}

	applyKBGapTelemetryMigration(t, db)

	if err := db.QueryRow(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('ai_kb_gap_events','ai_kb_gap_missing_fields')`).Scan(&n); err != nil {
		t.Fatalf("sqlite_master (post-migration): %v", err)
	}
	if n != 2 {
		t.Fatalf("expected both gap tables after 0018, sqlite_master matched %d", n)
	}

	// Pre-existing data survived untouched.
	var draftText string
	if err := db.QueryRow(ctx, `SELECT draft_text FROM ai_drafts WHERE id = $1`, draftID).Scan(&draftText); err != nil {
		t.Fatalf("read back pre-existing draft: %v", err)
	}
	if draftText != "Секунду, уточню." {
		t.Errorf("pre-existing draft_text = %q, want unchanged", draftText)
	}

	// The FKs resolve against rows that predate the migration.
	var eventID string
	if err := db.QueryRow(ctx, `INSERT INTO ai_kb_gap_events (organization_id, draft_id, chat_id, reason_code)
		VALUES ('33333333-3333-3333-3333-333333333333', $1, 'chat-pre-0018', 'missing_field') RETURNING id`, draftID).Scan(&eventID); err != nil {
		t.Fatalf("insert gap event referencing pre-existing org/draft: %v", err)
	}
	if eventID == "" {
		t.Fatal("expected a generated event id")
	}
}

// TestMigration0018_DefaultsMatchAppExpectations is a fresh-database test:
// the columns the store layer relies on having a safe default (channel,
// target_entity_type, target_entity_ref, escalation_reason, source) must
// actually default the way response/service.go and the store's insert path
// assume when they omit them.
func TestMigration0018_DefaultsMatchAppExpectations(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, gapTestOrgSQL)

	var channel, targetType, targetRef, reason, source string
	if err := db.QueryRow(ctx, `INSERT INTO ai_kb_gap_events (organization_id, chat_id, reason_code)
		VALUES ('33333333-3333-3333-3333-333333333333', 'chat-1', 'other')
		RETURNING channel, target_entity_type, target_entity_ref, escalation_reason, source`).
		Scan(&channel, &targetType, &targetRef, &reason, &source); err != nil {
		t.Fatalf("insert with only required columns: %v", err)
	}
	if channel != "whatsapp" {
		t.Errorf("channel default = %q, want whatsapp", channel)
	}
	if targetType != "" || targetRef != "" || reason != "" {
		t.Errorf("target_entity_type/ref/escalation_reason defaults = %q/%q/%q, want all empty", targetType, targetRef, reason)
	}
	if source != "model" {
		t.Errorf("source default = %q, want model", source)
	}
}

// TestMigration0018_DraftIDIsUniquePerEvent enforces "at most one gap event
// per draft" at the database level (WriteDraftSet's shared transaction
// helper relies on this never silently double-inserting).
func TestMigration0018_DraftIDIsUniquePerEvent(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, gapTestOrgSQL)
	var draftID string
	if err := db.QueryRow(ctx, `INSERT INTO ai_drafts (chat_id, option_ordinal) VALUES ('chat-1', 1) RETURNING id`).Scan(&draftID); err != nil {
		t.Fatalf("insert draft: %v", err)
	}
	mustExec(t, db, ctx, `INSERT INTO ai_kb_gap_events (organization_id, draft_id, chat_id, reason_code)
		VALUES ('33333333-3333-3333-3333-333333333333', $1, 'chat-1', 'missing_field')`, draftID)

	_, err := db.Exec(ctx, `INSERT INTO ai_kb_gap_events (organization_id, draft_id, chat_id, reason_code)
		VALUES ('33333333-3333-3333-3333-333333333333', $1, 'chat-1', 'other')`, draftID)
	if err == nil {
		t.Fatal("expected a second gap event for the same draft_id to be rejected by UNIQUE(draft_id)")
	}

	// A NULL draft_id must NOT be constrained by the same uniqueness (SQLite
	// treats NULLs as distinct) — multiple engine-error events with no draft
	// yet resolved must remain insertable.
	mustExec(t, db, ctx, `INSERT INTO ai_kb_gap_events (organization_id, chat_id, reason_code, source)
		VALUES ('33333333-3333-3333-3333-333333333333', 'chat-2', 'engine_error', 'engine')`)
	mustExec(t, db, ctx, `INSERT INTO ai_kb_gap_events (organization_id, chat_id, reason_code, source)
		VALUES ('33333333-3333-3333-3333-333333333333', 'chat-3', 'engine_error', 'engine')`)
}

// TestMigration0018_MissingFieldsChildTable covers the child table's own
// shape: a field name is queryable directly (no comma-separated parsing),
// duplicate field names for the same event are rejected, and deleting the
// parent event cascades to its missing_fields rows.
func TestMigration0018_MissingFieldsChildTable(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, gapTestOrgSQL)
	var eventID string
	if err := db.QueryRow(ctx, `INSERT INTO ai_kb_gap_events (organization_id, chat_id, reason_code)
		VALUES ('33333333-3333-3333-3333-333333333333', 'chat-1', 'missing_field') RETURNING id`).Scan(&eventID); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	mustExec(t, db, ctx, `INSERT INTO ai_kb_gap_missing_fields (event_id, field_name) VALUES ($1, 'price')`, eventID)

	if _, err := db.Exec(ctx, `INSERT INTO ai_kb_gap_missing_fields (event_id, field_name) VALUES ($1, 'price')`, eventID); err == nil {
		t.Fatal("expected a duplicate (event_id, field_name) to be rejected")
	}

	mustExec(t, db, ctx, `DELETE FROM ai_kb_gap_events WHERE id = $1`, eventID)
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM ai_kb_gap_missing_fields WHERE event_id = $1`, eventID).Scan(&n); err != nil {
		t.Fatalf("count children after parent delete: %v", err)
	}
	if n != 0 {
		t.Errorf("expected ON DELETE CASCADE to remove missing_fields rows, %d remain", n)
	}
}

// TestMigration0018_CascadeAndSetNullBehavior pins the two different delete
// behaviors the migration's header documents: deleting the organization
// removes its gap events (append-only telemetry is still organization-
// scoped, private data); deleting the draft the event pointed at does NOT
// remove the event — only its draft_id link is cleared, since the whole
// point of an append-only log is to outlive the specific draft row.
func TestMigration0018_CascadeAndSetNullBehavior(t *testing.T) {
	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, gapTestOrgSQL)
	var draftID string
	if err := db.QueryRow(ctx, `INSERT INTO ai_drafts (chat_id, option_ordinal) VALUES ('chat-1', 1) RETURNING id`).Scan(&draftID); err != nil {
		t.Fatalf("insert draft: %v", err)
	}
	var eventID string
	if err := db.QueryRow(ctx, `INSERT INTO ai_kb_gap_events (organization_id, draft_id, chat_id, reason_code)
		VALUES ('33333333-3333-3333-3333-333333333333', $1, 'chat-1', 'missing_field') RETURNING id`, draftID).Scan(&eventID); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	mustExec(t, db, ctx, `DELETE FROM ai_drafts WHERE id = $1`, draftID)
	var gotDraftID *string
	if err := db.QueryRow(ctx, `SELECT draft_id FROM ai_kb_gap_events WHERE id = $1`, eventID).Scan(&gotDraftID); err != nil {
		t.Fatalf("read back event after draft delete: %v", err)
	}
	if gotDraftID != nil {
		t.Errorf("draft_id = %v, want NULL after the referenced draft was deleted", *gotDraftID)
	}

	mustExec(t, db, ctx, `DELETE FROM organizations WHERE id = '33333333-3333-3333-3333-333333333333'`)
	var n int
	err := db.QueryRow(ctx, `SELECT count(*) FROM ai_kb_gap_events WHERE id = $1`, eventID).Scan(&n)
	if err != nil && !errors.Is(err, dbx.ErrNoRows) {
		t.Fatalf("count event after org delete: %v", err)
	}
	if n != 0 {
		t.Errorf("expected ON DELETE CASCADE from organizations to remove the gap event, found %d", n)
	}
}
