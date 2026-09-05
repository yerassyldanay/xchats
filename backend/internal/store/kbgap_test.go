package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

func TestWriteDraftSet_EscalateTrueRecordsGapEventWithDefaults(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, _, chatID := seedOneChat(t, st)

	drafts, err := st.WriteDraftSet(ctx, "whatsapp", chatID, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "Секунду, уточню.", ReplyLanguage: "ru", Escalate: true, EscalationReason: "нет данных"},
	})
	if err != nil {
		t.Fatalf("WriteDraftSet: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("want 1 draft, got %d", len(drafts))
	}

	events, err := st.GapEventsForChat(ctx, chatID)
	if err != nil {
		t.Fatalf("GapEventsForChat: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want exactly 1 gap event for one escalating option, got %d: %+v", len(events), events)
	}
	e := events[0]
	if e.ReasonCode != "other" {
		t.Errorf("ReasonCode = %q, want %q (default when no structured diagnostic is given)", e.ReasonCode, "other")
	}
	if e.Source != "model" {
		t.Errorf("Source = %q, want %q (default)", e.Source, "model")
	}
	if e.EscalationReason != "нет данных" {
		t.Errorf("EscalationReason = %q, want %q", e.EscalationReason, "нет данных")
	}
	if !e.DraftID.Valid || e.DraftID.UUID != drafts[0].ID {
		t.Errorf("DraftID = %+v, want the written draft's id %s", e.DraftID, drafts[0].ID)
	}
	if e.Channel != "whatsapp" || e.ChatID != chatID {
		t.Errorf("Channel/ChatID = %q/%s, want whatsapp/%s", e.Channel, e.ChatID, chatID)
	}
	if len(e.MissingFields) != 0 {
		t.Errorf("MissingFields = %v, want none", e.MissingFields)
	}
}

func TestWriteDraftSet_EscalateTrueWithStructuredKBGap(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, _, chatID := seedOneChat(t, st)

	_, err := st.WriteDraftSet(ctx, "whatsapp", chatID, uuid.NullUUID{}, []store.DraftOption{
		{
			Ordinal: 1, Text: "Секунду, уточню.", ReplyLanguage: "ru", Escalate: true,
			EscalationReason: "нет цены", KBGapReasonCode: "missing_field",
			KBGapTargetEntityType: "product", KBGapTargetEntityRef: "coffee-machine",
			KBGapMissingFields: []string{"price", "warranty_terms"},
		},
	})
	if err != nil {
		t.Fatalf("WriteDraftSet: %v", err)
	}

	events, err := st.GapEventsForChat(ctx, chatID)
	if err != nil {
		t.Fatalf("GapEventsForChat: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want exactly 1 gap event, got %d", len(events))
	}
	e := events[0]
	if e.ReasonCode != "missing_field" {
		t.Errorf("ReasonCode = %q, want missing_field", e.ReasonCode)
	}
	if e.TargetEntityType != "product" || e.TargetEntityRef != "coffee-machine" {
		t.Errorf("target = %q/%q, want product/coffee-machine", e.TargetEntityType, e.TargetEntityRef)
	}
	wantFields := map[string]bool{"price": true, "warranty_terms": true}
	if len(e.MissingFields) != 2 || !wantFields[e.MissingFields[0]] || !wantFields[e.MissingFields[1]] {
		t.Errorf("MissingFields = %v, want [price warranty_terms] in some order", e.MissingFields)
	}
}

func TestWriteDraftSet_EscalateFalseRecordsNoEvent(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, _, chatID := seedOneChat(t, st)

	if _, err := st.WriteDraftSet(ctx, "whatsapp", chatID, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "Кофемашина стоит...", ReplyLanguage: "ru", Escalate: false},
	}); err != nil {
		t.Fatalf("WriteDraftSet: %v", err)
	}

	events, err := st.GapEventsForChat(ctx, chatID)
	if err != nil {
		t.Fatalf("GapEventsForChat: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("a non-escalating draft must record no gap event, got %+v", events)
	}
}

// TestWriteDraftSet_SupersedingADraftKeepsItsGapEvent proves the telemetry
// log is genuinely append-only: superseding an earlier suggested draft
// (the normal "a newer generation replaces the old one" flow) must not
// touch — let alone remove — the gap event an earlier escalating draft
// already recorded.
func TestWriteDraftSet_SupersedingADraftKeepsItsGapEvent(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, _, chatID := seedOneChat(t, st)

	if _, err := st.WriteDraftSet(ctx, "whatsapp", chatID, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "first", ReplyLanguage: "ru", Escalate: true, KBGapReasonCode: "unsupported_request"},
	}); err != nil {
		t.Fatalf("WriteDraftSet (first): %v", err)
	}
	if _, err := st.WriteDraftSet(ctx, "whatsapp", chatID, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "second", ReplyLanguage: "ru", Escalate: true, KBGapReasonCode: "missing_entity"},
	}); err != nil {
		t.Fatalf("WriteDraftSet (second): %v", err)
	}

	events, err := st.GapEventsForChat(ctx, chatID)
	if err != nil {
		t.Fatalf("GapEventsForChat: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("want both events preserved across the supersede, got %d: %+v", len(events), events)
	}
}

func TestWriteDraftSetIfVersionCurrent_StaleGenerationRecordsNoEvent(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)

	if err := st.ArmDebounce(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("ArmDebounce: %v", err)
	}
	if err := st.ArmDebounce(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("ArmDebounce #2: %v", err)
	}

	drafts, written, err := st.WriteDraftSetIfVersionCurrent(ctx, "whatsapp", chatID, 1, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "stale escalation", ReplyLanguage: "ru", Escalate: true, KBGapReasonCode: "missing_field"},
	})
	if err != nil {
		t.Fatalf("stale write: %v", err)
	}
	if written || len(drafts) != 0 {
		t.Fatalf("stale write should discard: written=%v drafts=%v", written, drafts)
	}

	events, err := st.GapEventsForChat(ctx, chatID)
	if err != nil {
		t.Fatalf("GapEventsForChat: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("a stale automation generation must produce neither a draft nor a telemetry event, got %+v", events)
	}
}

// seedGapEvent escalates one draft in chatID with the given reason code —
// a thin wrapper around WriteDraftSet for tests that just need N events on
// the books to query back through KBGapReportFor.
func seedGapEvent(t *testing.T, st *store.Store, chatID uuid.UUID, reasonCode string) {
	t.Helper()
	if _, err := st.WriteDraftSet(context.Background(), "whatsapp", chatID, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "x", ReplyLanguage: "ru", Escalate: true, KBGapReasonCode: reasonCode},
	}); err != nil {
		t.Fatalf("seedGapEvent(%s): %v", reasonCode, err)
	}
}

func TestKBGapReportFor_DefaultReportIsZeroFilledAndSplitsOperational(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, _, chatID := seedOneChat(t, st)

	seedGapEvent(t, st, chatID, "missing_field")
	seedGapEvent(t, st, chatID, "missing_field")
	seedGapEvent(t, st, chatID, "unsupported_request")
	seedGapEvent(t, st, chatID, "human_requested")

	report, err := st.KBGapReportFor(ctx, store.KBGapFilter{OrgID: orgID})
	if err != nil {
		t.Fatalf("KBGapReportFor: %v", err)
	}

	// Default report: all 5 content-gap codes present, zero-filled, in order.
	wantOrder := []string{"missing_entity", "missing_field", "ambiguous_entity", "conflicting_kb_data", "unavailable_entity"}
	if len(report.Counts) != len(wantOrder) {
		t.Fatalf("Counts = %+v, want %d entries", report.Counts, len(wantOrder))
	}
	for i, code := range wantOrder {
		if report.Counts[i].ReasonCode != code {
			t.Errorf("Counts[%d].ReasonCode = %q, want %q", i, report.Counts[i].ReasonCode, code)
		}
	}
	if got := countFor(report.Counts, "missing_field"); got != 2 {
		t.Errorf("missing_field count = %d, want 2", got)
	}
	if got := countFor(report.Counts, "missing_entity"); got != 0 {
		t.Errorf("missing_entity count = %d, want 0 (zero-filled, not absent)", got)
	}

	// Operational codes never leak into the default report...
	if countFor(report.Counts, "unsupported_request") != -1 {
		t.Error("unsupported_request must not appear in the default (content-gap) Counts at all")
	}
	// ...but stay distinguishable in their own bucket.
	if got := countFor(report.OperationalCounts, "unsupported_request"); got != 1 {
		t.Errorf("OperationalCounts unsupported_request = %d, want 1", got)
	}
	if got := countFor(report.OperationalCounts, "human_requested"); got != 1 {
		t.Errorf("OperationalCounts human_requested = %d, want 1", got)
	}
	if got := countFor(report.OperationalCounts, "engine_error"); got != 0 {
		t.Errorf("OperationalCounts engine_error = %d, want 0 (zero-filled)", got)
	}

	if len(report.Recent) != 4 {
		t.Fatalf("Recent = %d events, want 4", len(report.Recent))
	}
}

func countFor(counts []store.KBGapReasonCount, code string) int {
	for _, c := range counts {
		if c.ReasonCode == code {
			return c.Count
		}
	}
	return -1
}

func TestKBGapReportFor_FiltersByReasonAndEntity(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, _, chatID := seedOneChat(t, st)

	if _, err := st.WriteDraftSet(ctx, "whatsapp", chatID, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "x", ReplyLanguage: "ru", Escalate: true, KBGapReasonCode: "missing_field",
			KBGapTargetEntityType: "product", KBGapTargetEntityRef: "coffee-machine"},
	}); err != nil {
		t.Fatalf("seed event 1: %v", err)
	}
	if _, err := st.WriteDraftSet(ctx, "whatsapp", chatID, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "x", ReplyLanguage: "ru", Escalate: true, KBGapReasonCode: "missing_field",
			KBGapTargetEntityType: "product", KBGapTargetEntityRef: "other-widget"},
	}); err != nil {
		t.Fatalf("seed event 2: %v", err)
	}
	seedGapEvent(t, st, chatID, "unsupported_request")

	report, err := st.KBGapReportFor(ctx, store.KBGapFilter{
		OrgID: orgID, TargetEntityType: "product", TargetEntityRef: "coffee-machine",
	})
	if err != nil {
		t.Fatalf("KBGapReportFor: %v", err)
	}
	if len(report.Recent) != 1 || report.Recent[0].TargetEntityRef != "coffee-machine" {
		t.Fatalf("entity-ref filter: Recent = %+v, want exactly the coffee-machine event", report.Recent)
	}

	byReason, err := st.KBGapReportFor(ctx, store.KBGapFilter{OrgID: orgID, ReasonCode: "unsupported_request"})
	if err != nil {
		t.Fatalf("KBGapReportFor: %v", err)
	}
	if len(byReason.Recent) != 1 || byReason.Recent[0].ReasonCode != "unsupported_request" {
		t.Fatalf("reason filter: Recent = %+v, want exactly the unsupported_request event", byReason.Recent)
	}
}

func TestKBGapReportFor_DateRangeFilter(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, _, chatID := seedOneChat(t, st)
	seedGapEvent(t, st, chatID, "missing_field")

	future := time.Now().Add(24 * time.Hour)
	empty, err := st.KBGapReportFor(ctx, store.KBGapFilter{OrgID: orgID, From: &future})
	if err != nil {
		t.Fatalf("KBGapReportFor: %v", err)
	}
	if len(empty.Recent) != 0 {
		t.Fatalf("a From in the future must exclude every existing event, got %+v", empty.Recent)
	}

	past := time.Now().Add(-24 * time.Hour)
	full, err := st.KBGapReportFor(ctx, store.KBGapFilter{OrgID: orgID, From: &past})
	if err != nil {
		t.Fatalf("KBGapReportFor: %v", err)
	}
	if len(full.Recent) != 1 {
		t.Fatalf("a From in the past must include the existing event, got %+v", full.Recent)
	}
}

// TestKBGapReportFor_OrganizationScoped is the cross-tenant isolation
// guarantee: one org's events must never surface in another org's report.
// seedOneChat keys its account/chat off t.Name(), so each org is seeded in
// its own subtest to get a distinct identity rather than colliding on the
// same account the second call would otherwise upsert into.
func TestKBGapReportFor_OrganizationScoped(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()

	var orgA, chatA, orgB, chatB uuid.UUID
	t.Run("seed org A", func(t *testing.T) { orgA, _, chatA = seedOneChat(t, st) })
	seedGapEvent(t, st, chatA, "missing_field")

	t.Run("seed org B", func(t *testing.T) { orgB, _, chatB = seedOneChat(t, st) })
	seedGapEvent(t, st, chatB, "missing_field")
	seedGapEvent(t, st, chatB, "missing_field")

	reportA, err := st.KBGapReportFor(ctx, store.KBGapFilter{OrgID: orgA})
	if err != nil {
		t.Fatalf("KBGapReportFor(A): %v", err)
	}
	if len(reportA.Recent) != 1 {
		t.Fatalf("org A must see only its own event, got %+v", reportA.Recent)
	}
	reportB, err := st.KBGapReportFor(ctx, store.KBGapFilter{OrgID: orgB})
	if err != nil {
		t.Fatalf("KBGapReportFor(B): %v", err)
	}
	if len(reportB.Recent) != 2 {
		t.Fatalf("org B must see only its own 2 events, got %+v", reportB.Recent)
	}
}

func TestKBGapReportFor_RecentOrderedNewestFirstAndLimited(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	orgID, _, chatID := seedOneChat(t, st)
	for i := 0; i < 3; i++ {
		seedGapEvent(t, st, chatID, "missing_field")
	}

	report, err := st.KBGapReportFor(ctx, store.KBGapFilter{OrgID: orgID, Limit: 2})
	if err != nil {
		t.Fatalf("KBGapReportFor: %v", err)
	}
	if len(report.Recent) != 2 {
		t.Fatalf("Limit: 2 must cap Recent at 2, got %d", len(report.Recent))
	}
	if report.Counts[1].Count != 3 { // index 1 is missing_field in vocabulary order
		t.Fatalf("Counts must reflect ALL matching events regardless of the Recent page limit, got %+v", report.Counts)
	}
}

func TestWriteDraftSetIfVersionCurrent_CurrentGenerationRecordsEvent(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, accountID, chatID := seedOneChat(t, st)

	if err := st.ArmDebounce(ctx, chatID, accountID, "whatsapp", time.Now()); err != nil {
		t.Fatalf("ArmDebounce: %v", err)
	}

	drafts, written, err := st.WriteDraftSetIfVersionCurrent(ctx, "whatsapp", chatID, 1, uuid.NullUUID{}, []store.DraftOption{
		{Ordinal: 1, Text: "fresh escalation", ReplyLanguage: "ru", Escalate: true, KBGapReasonCode: "ambiguous_entity"},
	})
	if err != nil {
		t.Fatalf("fresh write: %v", err)
	}
	if !written || len(drafts) != 1 {
		t.Fatalf("want a normal write: written=%v drafts=%v", written, drafts)
	}

	events, err := st.GapEventsForChat(ctx, chatID)
	if err != nil {
		t.Fatalf("GapEventsForChat: %v", err)
	}
	if len(events) != 1 || events[0].ReasonCode != "ambiguous_entity" {
		t.Fatalf("want exactly 1 ambiguous_entity event, got %+v", events)
	}
}
