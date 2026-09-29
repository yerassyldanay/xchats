package dbtest

import (
	"context"
	"errors"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"testing"
)

const gapTestOrgSQL = `INSERT INTO organizations (id, name) VALUES ('33333333-3333-3333-3333-333333333333', 'Gap Test Org')`

func TestBaseline_DefaultsMatchAppExpectations(t *testing.T) {
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

// TestBaseline_DraftIDIsUniquePerEvent enforces "at most one gap event
// per draft" at the database level (WriteDraftSet's shared transaction
// helper relies on this never silently double-inserting).
func TestBaseline_DraftIDIsUniquePerEvent(t *testing.T) {
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

// TestBaseline_MissingFieldsChildTable covers the child table's own
// shape: a field name is queryable directly (no comma-separated parsing),
// duplicate field names for the same event are rejected, and deleting the
// parent event cascades to its missing_fields rows.
func TestBaseline_MissingFieldsChildTable(t *testing.T) {
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

// TestBaseline_CascadeAndSetNullBehavior pins the two different delete
// behaviors the migration's header documents: deleting the organization
// removes its gap events (append-only telemetry is still organization-
// scoped, private data); deleting the draft the event pointed at does NOT
// remove the event — only its draft_id link is cleared, since the whole
// point of an append-only log is to outlive the specific draft row.
func TestBaseline_CascadeAndSetNullBehavior(t *testing.T) {
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
