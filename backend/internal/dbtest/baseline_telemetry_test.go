package dbtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

const gapTestOrgSQL = `INSERT INTO organizations (id, name, created_at, updated_at) VALUES ('33333333-3333-3333-3333-333333333333', 'Gap Test Org', 1791244800000, 1791244800000)`

func TestBaseline_DefaultsMatchAppExpectations(t *testing.T) {
	now := time.Now()

	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, gapTestOrgSQL)

	var channel, targetType, targetRef, reason, source string
	if err := db.QueryRow(ctx, `INSERT INTO ai_kb_gap_events (organization_id, chat_id, reason_code, id, created_at)
		VALUES ('33333333-3333-3333-3333-333333333333', 'chat-1', 'other', $1, $2)
		RETURNING channel, target_entity_type, target_entity_ref, escalation_reason, source`, uuid.New(), now).
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
	now := time.Now()

	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, gapTestOrgSQL)
	var draftID string
	if err := db.QueryRow(ctx, `INSERT INTO ai_drafts (chat_id, option_ordinal, id, created_at, updated_at) VALUES ('chat-1', 1, $1, $2, $2) RETURNING id`, uuid.New(), now).Scan(&draftID); err != nil {
		t.Fatalf("insert draft: %v", err)
	}
	mustExec(t, db, ctx, `INSERT INTO ai_kb_gap_events (organization_id, draft_id, chat_id, reason_code, id, created_at)
		VALUES ('33333333-3333-3333-3333-333333333333', $1, 'chat-1', 'missing_field', $2, $3)`, draftID, uuid.New(), now)

	_, err := db.Exec(ctx, `INSERT INTO ai_kb_gap_events (organization_id, draft_id, chat_id, reason_code, id, created_at)
		VALUES ('33333333-3333-3333-3333-333333333333', $1, 'chat-1', 'other', $2, $3)`, draftID, uuid.New(), now)
	if !dbx.IsUniqueViolation(err) {
		t.Fatalf("second gap event for the same draft_id: err = %v, want a UNIQUE(draft_id) violation", err)
	}

	// A NULL draft_id must NOT be constrained by the same uniqueness (SQLite
	// treats NULLs as distinct) — multiple engine-error events with no draft
	// yet resolved must remain insertable.
	mustExec(t, db, ctx, `INSERT INTO ai_kb_gap_events (organization_id, chat_id, reason_code, source, id, created_at)
		VALUES ('33333333-3333-3333-3333-333333333333', 'chat-2', 'engine_error', 'engine', $1, $2)`, uuid.New(), now)
	mustExec(t, db, ctx, `INSERT INTO ai_kb_gap_events (organization_id, chat_id, reason_code, source, id, created_at)
		VALUES ('33333333-3333-3333-3333-333333333333', 'chat-3', 'engine_error', 'engine', $1, $2)`, uuid.New(), now)
}

// TestBaseline_MissingFieldsChildTable covers the child table's own
// shape: a field name is queryable directly (no comma-separated parsing),
// duplicate field names for the same event are rejected, and deleting the
// parent event cascades to its missing_fields rows.
func TestBaseline_MissingFieldsChildTable(t *testing.T) {
	now := time.Now()

	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, gapTestOrgSQL)
	var eventID string
	if err := db.QueryRow(ctx, `INSERT INTO ai_kb_gap_events (organization_id, chat_id, reason_code, id, created_at)
		VALUES ('33333333-3333-3333-3333-333333333333', 'chat-1', 'missing_field', $1, $2) RETURNING id`, uuid.New(), now).Scan(&eventID); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	mustExec(t, db, ctx, `INSERT INTO ai_kb_gap_missing_fields (event_id, field_name, id, created_at) VALUES ($1, 'price', $2, $3)`, eventID, uuid.New(), now)

	if _, err := db.Exec(ctx, `INSERT INTO ai_kb_gap_missing_fields (event_id, field_name, id, created_at) VALUES ($1, 'price', $2, $3)`, eventID, uuid.New(), now); !dbx.IsUniqueViolation(err) {
		t.Fatalf("duplicate (event_id, field_name): err = %v, want a uniqueness violation", err)
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
	now := time.Now()

	db := OpenRaw(t)
	ctx := context.Background()
	mustExec(t, db, ctx, gapTestOrgSQL)
	var draftID string
	if err := db.QueryRow(ctx, `INSERT INTO ai_drafts (chat_id, option_ordinal, id, created_at, updated_at) VALUES ('chat-1', 1, $1, $2, $2) RETURNING id`, uuid.New(), now).Scan(&draftID); err != nil {
		t.Fatalf("insert draft: %v", err)
	}
	var eventID string
	if err := db.QueryRow(ctx, `INSERT INTO ai_kb_gap_events (organization_id, draft_id, chat_id, reason_code, id, created_at)
		VALUES ('33333333-3333-3333-3333-333333333333', $1, 'chat-1', 'missing_field', $2, $3) RETURNING id`, draftID, uuid.New(), now).Scan(&eventID); err != nil {
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
