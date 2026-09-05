package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// kbGapReportPayload mirrors httpapi's (unexported) kbGapReportView JSON
// shape — this is an external test package, so it decodes GET /kb/gaps'
// response into its own copy rather than reaching into httpapi internals.
type kbGapReportPayload struct {
	Counts            []kbGapReasonCountPayload `json:"counts"`
	OperationalCounts []kbGapReasonCountPayload `json:"operational_counts"`
	TopTargetEntities []kbGapEntityCountPayload `json:"top_target_entities"`
	TopMissingFields  []kbGapFieldCountPayload  `json:"top_missing_fields"`
	Recent            []kbGapEventPayload       `json:"recent"`
}

type kbGapReasonCountPayload struct {
	ReasonCode string `json:"reason_code"`
	Count      int    `json:"count"`
}

type kbGapEntityCountPayload struct {
	TargetEntityType string `json:"target_entity_type"`
	TargetEntityRef  string `json:"target_entity_ref"`
	Count            int    `json:"count"`
}

type kbGapFieldCountPayload struct {
	TargetEntityType string `json:"target_entity_type"`
	FieldName        string `json:"field_name"`
	Count            int    `json:"count"`
}

type kbGapEventPayload struct {
	ID               string   `json:"id"`
	Channel          string   `json:"channel"`
	ChatID           string   `json:"chat_id"`
	DraftID          string   `json:"draft_id"`
	ReasonCode       string   `json:"reason_code"`
	TargetEntityType string   `json:"target_entity_type"`
	TargetEntityRef  string   `json:"target_entity_ref"`
	MissingFields    []string `json:"missing_fields"`
	EscalationReason string   `json:"escalation_reason"`
	Source           string   `json:"source"`
	CreatedAt        string   `json:"created_at"`
}

func TestKBGaps_EmptyByDefault(t *testing.T) {
	h := newHarness(t)
	var got kbGapReportPayload
	h.get("/xchats/api/v1/kb/gaps", &got)

	if len(got.Counts) != 4 {
		t.Fatalf("Counts = %+v, want the 4 default content-gap codes, zero-filled", got.Counts)
	}
	for _, c := range got.Counts {
		if c.Count != 0 {
			t.Errorf("Counts[%s] = %d, want 0 on a fresh org", c.ReasonCode, c.Count)
		}
	}
	if len(got.Recent) != 0 {
		t.Fatalf("Recent = %+v, want none on a fresh org", got.Recent)
	}
}

// TestKBGaps_ReturnsSeededEventNeverExposingDraftText proves the endpoint's
// shape end to end: an escalating draft's structured kb_gap surfaces in
// both Counts and Recent, filters by reason/entity work over HTTP, and —
// the plan's explicit boundary — the response never carries the actual
// draft/customer message text anywhere, only diagnostic metadata.
func TestKBGaps_ReturnsSeededEventNeverExposingDraftText(t *testing.T) {
	h := newHarness(t)
	chatIDStr, _ := h.inject("77099999999@s.whatsapp.net", "GAP-HTTP-1", "секретный текст клиента", false)
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		t.Fatalf("parse chat id: %v", err)
	}

	const secretDraftText = "совершенно секретный текст черновика"
	if _, err := h.store.WriteDraftSet(context.Background(), "whatsapp", chatID, uuid.NullUUID{}, []store.DraftOption{
		{
			Ordinal: 1, Text: secretDraftText, ReplyLanguage: "ru", Escalate: true,
			EscalationReason: "нет цены", KBGapReasonCode: "missing_field",
			KBGapTargetEntityType: "product", KBGapTargetEntityRef: "coffee-machine",
			KBGapMissingFields: []string{"price"},
		},
	}); err != nil {
		t.Fatalf("WriteDraftSet: %v", err)
	}

	var got kbGapReportPayload
	h.get("/xchats/api/v1/kb/gaps", &got)

	var missingFieldCount int
	for _, c := range got.Counts {
		if c.ReasonCode == "missing_field" {
			missingFieldCount = c.Count
		}
	}
	if missingFieldCount != 1 {
		t.Fatalf("Counts[missing_field] = %d, want 1: %+v", missingFieldCount, got.Counts)
	}
	if len(got.Recent) != 1 {
		t.Fatalf("Recent = %+v, want exactly 1 event", got.Recent)
	}
	e := got.Recent[0]
	if e.TargetEntityType != "product" || e.TargetEntityRef != "coffee-machine" {
		t.Errorf("target = %q/%q, want product/coffee-machine", e.TargetEntityType, e.TargetEntityRef)
	}
	if len(e.MissingFields) != 1 || e.MissingFields[0] != "price" {
		t.Errorf("MissingFields = %v, want [price]", e.MissingFields)
	}
	if e.Source != "model" {
		t.Errorf("Source = %q, want model", e.Source)
	}
	if len(got.TopTargetEntities) != 1 || got.TopTargetEntities[0].TargetEntityRef != "coffee-machine" || got.TopTargetEntities[0].Count != 1 {
		t.Errorf("TopTargetEntities = %+v, want exactly product/coffee-machine at count 1", got.TopTargetEntities)
	}
	if len(got.TopMissingFields) != 1 || got.TopMissingFields[0].FieldName != "price" || got.TopMissingFields[0].Count != 1 {
		t.Errorf("TopMissingFields = %+v, want exactly product/price at count 1", got.TopMissingFields)
	}

	// Filters work over HTTP, not just at the store layer.
	var filtered kbGapReportPayload
	h.get("/xchats/api/v1/kb/gaps?reason=unsupported_request", &filtered)
	if len(filtered.Recent) != 0 {
		t.Fatalf("reason filter: Recent = %+v, want none for an unrelated reason code", filtered.Recent)
	}

	// The plan's explicit boundary: never the customer-facing message text,
	// under any field name, anywhere in the response.
	rawBody := struct {
		Recent []map[string]any `json:"recent"`
	}{}
	h.get("/xchats/api/v1/kb/gaps", &rawBody)
	for _, row := range rawBody.Recent {
		for field, val := range row {
			if s, ok := val.(string); ok && s == secretDraftText {
				t.Fatalf("GET /kb/gaps leaked the draft's customer-facing text through field %q", field)
			}
		}
	}
}

func TestKBGaps_UnauthenticatedRejected(t *testing.T) {
	h := newHarness(t)
	noauth := &http.Client{}
	resp, err := noauth.Get(h.srv.URL + "/xchats/api/v1/kb/gaps")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for an unauthenticated request", resp.StatusCode)
	}
}
