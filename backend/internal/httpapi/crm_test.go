package httpapi_test

// crm_test.go covers the customer-management API end to end through the real
// router: the inbound-message → customer link the sidebar depends on, the
// note/tag/status/follow-up surface a manager works through, and — the part
// that matters most — the tenant boundary. Section 14 of the product brief is
// unconditional: a customer from organization A must never be visible or
// accessible to organization B, on any of these routes.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

type crmCustomer struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Status      *struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	} `json:"status"`
	Tags []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"tags"`
	Identities []struct {
		Channel    string `json:"channel"`
		ExternalID string `json:"external_id"`
	} `json:"identities"`
}

type crmProfile struct {
	Customer   crmCustomer `json:"customer"`
	LatestNote *struct {
		Body       string `json:"body"`
		AuthorName string `json:"author_name"`
	} `json:"latest_note"`
	NextFollowup *struct {
		ID      string `json:"id"`
		Action  string `json:"action"`
		DueDate string `json:"due_date"`
	} `json:"next_followup"`
	Conversations []map[string]any `json:"conversations"`
}

// firstCustomerID injects one inbound message and returns the customer the
// ingest path created for it, plus the chat id.
func firstCustomerID(t *testing.T, h *harness) (customerID, chatID string) {
	t.Helper()
	chatID, _ = h.inject(customerJID, "WA-CRM-1", "привет", false)

	var page struct {
		Items []crmCustomer `json:"items"`
		Total int           `json:"total"`
	}
	h.get("/xchats/api/v1/customers", &page)
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("expected exactly 1 customer after one inbound message, got %+v", page)
	}
	return page.Items[0].ID, chatID
}

// TestInboundMessageProducesCustomer is the P0 path: a message from a new
// external identity creates a customer, links the conversation to it, and the
// chat row itself names the customer so the sidebar needs no extra lookup.
func TestInboundMessageProducesCustomer(t *testing.T) {
	h := newHarness(t)
	customerID, chatID := firstCustomerID(t, h)

	chats := h.listChats()
	if len(chats) != 1 {
		t.Fatalf("want 1 chat, got %d", len(chats))
	}
	if got, _ := chats[0]["customer_id"].(string); got != customerID {
		t.Fatalf("chat.customer_id = %q, want %q", got, customerID)
	}

	var profile crmProfile
	h.get("/xchats/api/v1/customers/"+customerID, &profile)
	if len(profile.Customer.Identities) != 1 {
		t.Fatalf("expected 1 channel identity, got %+v", profile.Customer.Identities)
	}
	if profile.Customer.Identities[0].Channel != "whatsapp" {
		t.Errorf("identity channel = %q", profile.Customer.Identities[0].Channel)
	}
	if profile.Customer.Status == nil || profile.Customer.Status.Slug != "new" {
		t.Errorf("expected the seeded default status, got %+v", profile.Customer.Status)
	}
	if len(profile.Conversations) != 1 {
		t.Fatalf("expected the conversation on the profile, got %d", len(profile.Conversations))
	}
	if profile.Conversations[0]["id"] != chatID {
		t.Errorf("profile conversation = %v, want chat %s", profile.Conversations[0]["id"], chatID)
	}
}

// TestCustomerSidebarRoundTrip walks what a manager actually does without
// leaving the conversation: set a status, add a tag, write a note, schedule the
// next action — and read it all back off one profile call.
func TestCustomerSidebarRoundTrip(t *testing.T) {
	h := newHarness(t)
	customerID, chatID := firstCustomerID(t, h)

	var statuses struct {
		Items []struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"items"`
	}
	h.get("/xchats/api/v1/crm/statuses", &statuses)
	var interestedID string
	for _, s := range statuses.Items {
		if s.Slug == "interested" {
			interestedID = s.ID
		}
	}
	if interestedID == "" {
		t.Fatalf("default statuses were not seeded: %+v", statuses.Items)
	}

	resp, _ := h.patchJSON("/xchats/api/v1/customers/"+customerID,
		map[string]any{"display_name": "Иван Смирнов", "status_id": interestedID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch customer status=%d", resp.StatusCode)
	}

	resp, tagEnv := h.postJSON("/xchats/api/v1/crm/tags", map[string]any{"name": "Интересуется", "color": "#0ea5e9"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create tag status=%d", resp.StatusCode)
	}
	var tag struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(tagEnv["payload"], &tag); err != nil {
		t.Fatalf("decode tag: %v", err)
	}
	// A Cyrillic label must still produce a usable slug — an ASCII-only
	// slugifier would collapse it to "" and collide on the next tag.
	if tag.Slug == "" {
		t.Errorf("tag slug is empty for a Cyrillic name — slugify dropped everything")
	}
	// A second Cyrillic tag proves the point: it must not 409 on the slug.
	resp, _ = h.postJSON("/xchats/api/v1/crm/tags", map[string]any{"name": "Не готов"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("second Cyrillic tag status=%d, want 201", resp.StatusCode)
	}

	resp, _ = h.postJSON("/xchats/api/v1/customers/"+customerID+"/tags", map[string]any{"tag_id": tag.ID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add tag status=%d", resp.StatusCode)
	}

	resp, _ = h.postJSON("/xchats/api/v1/customers/"+customerID+"/notes",
		map[string]any{"body": "Пока не готов интегрировать xpayment, просил перезвонить."})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add note status=%d", resp.StatusCode)
	}

	due := time.Now().Add(48 * time.Hour).UTC()
	resp, _ = h.postJSON("/xchats/api/v1/followups", map[string]any{
		"customer_id":     customerID,
		"conversation_id": chatID,
		"due_at":          due.Format(time.RFC3339),
		"due_date":        due.Format("2006-01-02"),
		"due_minute":      660,
		"action":          "call",
		"note":            "Позвонить про xpayment",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create followup status=%d", resp.StatusCode)
	}

	var profile crmProfile
	h.get("/xchats/api/v1/customers/"+customerID, &profile)
	if profile.Customer.DisplayName != "Иван Смирнов" {
		t.Errorf("display name = %q", profile.Customer.DisplayName)
	}
	if profile.Customer.Status == nil || profile.Customer.Status.Slug != "interested" {
		t.Errorf("status = %+v", profile.Customer.Status)
	}
	if len(profile.Customer.Tags) != 1 || profile.Customer.Tags[0].ID != tag.ID {
		t.Errorf("tags = %+v", profile.Customer.Tags)
	}
	if profile.LatestNote == nil || profile.LatestNote.AuthorName == "" {
		t.Errorf("latest note = %+v (author must be recorded)", profile.LatestNote)
	}
	if profile.NextFollowup == nil || profile.NextFollowup.Action != "call" {
		t.Errorf("next followup = %+v", profile.NextFollowup)
	}

	// The timeline shows the CRM milestones and nothing else — TODO.md
	// "Filter out routine message logs from Timeline": it is an executive
	// audit log of account changes, not a transcript (see crm_timeline.go's
	// own doc comment).
	var timeline struct {
		Items []struct {
			Source string `json:"source"`
			Kind   string `json:"kind"`
		} `json:"items"`
	}
	h.get("/xchats/api/v1/customers/"+customerID+"/timeline", &timeline)
	kinds := map[string]bool{}
	sources := map[string]bool{}
	for _, e := range timeline.Items {
		kinds[e.Kind] = true
		sources[e.Source] = true
	}
	for _, want := range []string{"note_added", "status_changed", "tag_added", "followup_created"} {
		if !kinds[want] {
			t.Errorf("timeline is missing %q: %+v", want, kinds)
		}
	}
	if sources["message"] {
		t.Errorf("timeline includes routine conversation activity, want CRM milestones only: %+v", sources)
	}
}

// TestFollowupLifecycle covers complete / reschedule / cancel and the bucket
// counts the reminder view renders.
func TestFollowupLifecycle(t *testing.T) {
	h := newHarness(t)
	customerID, _ := firstCustomerID(t, h)

	// Due yesterday: overdue from the moment it is created, and it must stay
	// visible until it is completed or rescheduled.
	yesterday := time.Now().Add(-24 * time.Hour).UTC()
	_, env := h.postJSON("/xchats/api/v1/followups", map[string]any{
		"customer_id": customerID,
		"due_at":      yesterday.Format(time.RFC3339),
		"due_date":    yesterday.Format("2006-01-02"),
		"action":      "call",
	})
	var fu struct {
		ID    string `json:"id"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(env["payload"], &fu); err != nil {
		t.Fatalf("decode followup: %v", err)
	}

	var buckets struct {
		Today    int `json:"today"`
		Tomorrow int `json:"tomorrow"`
		ThisWeek int `json:"this_week"`
		Overdue  int `json:"overdue"`
	}
	h.get("/xchats/api/v1/followups/buckets?tz_offset_minutes=0", &buckets)
	if buckets.Overdue != 1 {
		t.Fatalf("overdue = %d, want 1 (buckets=%+v)", buckets.Overdue, buckets)
	}

	// Rescheduling to tomorrow moves it out of overdue.
	tomorrow := time.Now().Add(30 * time.Hour).UTC()
	resp, _ := h.patchJSON("/xchats/api/v1/followups/"+fu.ID, map[string]any{
		"due_at":   tomorrow.Format(time.RFC3339),
		"due_date": tomorrow.Format("2006-01-02"),
		"action":   "meeting",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reschedule status=%d", resp.StatusCode)
	}
	h.get("/xchats/api/v1/followups/buckets?tz_offset_minutes=0", &buckets)
	if buckets.Overdue != 0 {
		t.Errorf("overdue = %d after rescheduling, want 0", buckets.Overdue)
	}

	resp, _ = h.postJSON("/xchats/api/v1/followups/"+fu.ID+"/complete", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("complete status=%d", resp.StatusCode)
	}
	h.get("/xchats/api/v1/followups/buckets?tz_offset_minutes=0", &buckets)
	if buckets.Today+buckets.Tomorrow+buckets.ThisWeek+buckets.Overdue != 0 {
		t.Errorf("a completed follow-up still counts: %+v", buckets)
	}

	// A bad action is rejected, not silently coerced.
	resp, _ = h.postJSON("/xchats/api/v1/followups", map[string]any{
		"customer_id": customerID,
		"due_at":      tomorrow.Format(time.RFC3339),
		"due_date":    tomorrow.Format("2006-01-02"),
		"action":      "teleport",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid action status=%d, want 400", resp.StatusCode)
	}
}

// TestCustomerMergeEndpoint covers section 13 through the API: two identities,
// one manual merge, everything preserved on the survivor.
func TestCustomerMergeEndpoint(t *testing.T) {
	h := newHarness(t)
	h.inject(customerJID, "WA-MERGE-1", "привет", false)
	h.inject("77022222222@s.whatsapp.net", "WA-MERGE-2", "это тоже я", false)

	var page struct {
		Items []crmCustomer `json:"items"`
	}
	h.get("/xchats/api/v1/customers", &page)
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 customers before the merge, got %d", len(page.Items))
	}
	target, source := page.Items[0].ID, page.Items[1].ID

	resp, _ := h.postJSON("/xchats/api/v1/customers/"+target+"/merge",
		map[string]any{"source_customer_id": source})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("merge status=%d", resp.StatusCode)
	}

	h.get("/xchats/api/v1/customers", &page)
	if len(page.Items) != 1 || page.Items[0].ID != target {
		t.Fatalf("after merge expected only the survivor, got %+v", page.Items)
	}

	var profile crmProfile
	h.get("/xchats/api/v1/customers/"+target, &profile)
	if len(profile.Customer.Identities) != 2 {
		t.Fatalf("survivor lost an identity: %+v", profile.Customer.Identities)
	}
	if len(profile.Conversations) != 2 {
		t.Fatalf("survivor lost a conversation: %d", len(profile.Conversations))
	}

	// Merging a customer into itself is a request error, not a 500.
	resp, _ = h.postJSON("/xchats/api/v1/customers/"+target+"/merge",
		map[string]any{"source_customer_id": target})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("self-merge status=%d, want 400", resp.StatusCode)
	}
}

// TestCrmTenantIsolation is section 14: every CRM route, exercised as a user
// whose session is scoped to a DIFFERENT organization, must behave as if the
// other tenant's data does not exist.
func TestCrmTenantIsolation(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	customerID, _ := firstCustomerID(t, h)

	// Give the logged-in admin a second organization and switch the session to
	// it. Same user, same cookie — only the active organization differs, which
	// is exactly the boundary orgOf enforces.
	u, err := h.store.UserByEmail(ctx, adminEmail)
	if err != nil {
		t.Fatalf("user by email: %v", err)
	}
	org2, err := h.store.SeedOrganization(ctx, "crm-isolation-second")
	if err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	if _, err := h.db.Exec(ctx, `
		INSERT INTO organization_users (organization_id, user_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, org2.ID, u.ID); err != nil {
		t.Fatalf("add membership: %v", err)
	}
	resp, _ := h.postJSON("/xchats/api/v1/organization/active", map[string]any{"organization_id": org2.ID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch org status=%d", resp.StatusCode)
	}

	// The list is empty for the other tenant...
	var page struct {
		Items []crmCustomer `json:"items"`
		Total int           `json:"total"`
	}
	h.get("/xchats/api/v1/customers", &page)
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("org B sees org A's customers: %+v", page)
	}

	// ...and every id-addressed route 404s rather than leaking.
	reads := []string{
		"/xchats/api/v1/customers/" + customerID,
		"/xchats/api/v1/customers/" + customerID + "/conversations",
		"/xchats/api/v1/customers/" + customerID + "/timeline",
		"/xchats/api/v1/customers/" + customerID + "/notes",
	}
	for _, path := range reads {
		status, body := h.getBytes(path)
		if status != http.StatusNotFound {
			t.Errorf("GET %s as org B: status=%d, want 404 (body=%s)", path, status, body)
		}
	}

	writes := []struct {
		method, path string
		body         any
	}{
		{"PATCH", "/xchats/api/v1/customers/" + customerID, map[string]any{"display_name": "hijack"}},
		{"POST", "/xchats/api/v1/customers/" + customerID + "/notes", map[string]any{"body": "чужая заметка"}},
		{"POST", "/xchats/api/v1/customers/" + customerID + "/tags", map[string]any{"tag_id": uuid.NewString()}},
		{"POST", "/xchats/api/v1/followups", map[string]any{
			"customer_id": customerID, "due_at": time.Now().UTC().Format(time.RFC3339),
			"due_date": time.Now().UTC().Format("2006-01-02"), "action": "call",
		}},
	}
	for _, w := range writes {
		var status int
		if w.method == "PATCH" {
			r, _ := h.patchJSON(w.path, w.body)
			status = r.StatusCode
		} else {
			r, _ := h.postJSON(w.path, w.body)
			status = r.StatusCode
		}
		if status != http.StatusNotFound {
			t.Errorf("%s %s as org B: status=%d, want 404", w.method, w.path, status)
		}
	}

	// Switching back, org A's customer is untouched by any of the above.
	resp, _ = h.postJSON("/xchats/api/v1/organization/active", map[string]any{"organization_id": h.orgID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch back status=%d", resp.StatusCode)
	}
	var profile crmProfile
	h.get("/xchats/api/v1/customers/"+customerID, &profile)
	if profile.Customer.DisplayName == "hijack" {
		t.Fatal("org B's write reached org A's customer")
	}
	if profile.LatestNote != nil {
		t.Fatalf("org B's note reached org A's customer: %+v", profile.LatestNote)
	}
}
