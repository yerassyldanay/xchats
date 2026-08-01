package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

// changeSetDTO mirrors kbstore.DraftChangeSet's JSON shape loosely enough to
// assert on it without importing kbstore's concrete row types — every kind's
// entries decode as a generic map so a test can assert on a couple of fields
// (slug/ref, title/name) without pinning the whole row shape.
type changeSetDTO struct {
	BaseVersion int64            `json:"base_version"`
	UpdatedAt   string           `json:"updated_at"`
	Config      json.RawMessage  `json:"config"`
	Topics      []map[string]any `json:"topics"`
	Tariffs     []map[string]any `json:"tariffs"`
	Products    []map[string]any `json:"products"`
	Contacts    []map[string]any `json:"contacts"`
	Policies    []map[string]any `json:"policies"`
	Zones       []map[string]any `json:"zones"`
	Deletes     []map[string]any `json:"deletes"`
}

type cancelResponseDTO struct {
	Changed bool         `json:"changed"`
	Changes changeSetDTO `json:"changes"`
}

// delJSON is del's header-aware sibling — needed here (and nowhere else in
// this package yet) to send If-Match on a DELETE, the same optimistic-
// concurrency header the POST/PATCH draft-write helpers below carry.
func (h *harness) delJSON(path string, headers map[string]string) (*http.Response, map[string]json.RawMessage) {
	req, err := http.NewRequest(http.MethodDelete, h.srv.URL+path, nil)
	if err != nil {
		h.t.Fatalf("DELETE %s: %v", path, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("DELETE %s: %v", path, err)
	}
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	h.queue.Wait()
	return resp, env
}

func (h *harness) draftChanges() changeSetDTO {
	var out changeSetDTO
	h.get("/xchats/api/v1/playground/draft", &out)
	return out
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// TestPlaygroundDraft_ReturnsChangeSetNotMergedView asserts the review
// payload's shape: base_version present, and NO materials/requests keys at
// all — those are DraftView-only concepts (kbstore.DraftChangeSet carries
// neither field), so decoding the payload as a generic map must never find
// them, regardless of what is staged.
func TestPlaygroundDraft_ReturnsChangeSetNotMergedView(t *testing.T) {
	h := newHarness(t)
	h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{"slug": "t-shape", "title": "T", "body_md": "Body."})

	resp, env := h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{"slug": "t-shape-2", "title": "T2", "body_md": "Body."})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(env["payload"], &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := payload["base_version"]; !ok {
		t.Fatalf("payload %s missing base_version", env["payload"])
	}
	if _, ok := payload["materials"]; ok {
		t.Fatalf("payload %s must not carry materials — that is a DraftView-only concept", env["payload"])
	}
	if _, ok := payload["requests"]; ok {
		t.Fatalf("payload %s must not carry requests — that is a DraftView-only concept", env["payload"])
	}
}

// An unchanged published record must never appear in the review payload —
// only what this test itself staged.
func TestPlaygroundDraft_UpsertThenGetShowsOnlyThatRow(t *testing.T) {
	h := newHarness(t)
	before := h.draftChanges()
	if len(before.Topics) != 0 {
		t.Fatalf("a fresh org's Черновик must start empty, got %d topics", len(before.Topics))
	}

	resp, _ := h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{"slug": "only-me", "title": "Only me", "body_md": "Body."})
	if resp.StatusCode != 200 {
		t.Fatalf("upsert status = %d", resp.StatusCode)
	}

	after := h.draftChanges()
	if len(after.Topics) != 1 {
		t.Fatalf("Topics = %+v, want exactly the one staged topic", after.Topics)
	}
	if stringField(after.Topics[0], "slug") != "only-me" {
		t.Fatalf("Topics[0] = %+v, want slug only-me", after.Topics[0])
	}
	if len(after.Tariffs) != 0 || len(after.Products) != 0 || len(after.Contacts) != 0 || len(after.Policies) != 0 {
		t.Fatalf("every other kind must stay empty, got %+v", after)
	}
}

// Every draft-write response IS the fresh change set, not the old merged
// DraftView — a second write's response must show only what is staged, same
// as a plain GET would.
func TestPlaygroundDraft_WriteResponseIsTheChangeSet(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postJSON("/xchats/api/v1/playground/draft/tariffs", map[string]any{"ref": "resp-check", "name": "N", "price": "1000"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var payload changeSetDTO
	mustPayload(t, env, &payload)
	if len(payload.Tariffs) != 1 || stringField(payload.Tariffs[0], "ref") != "resp-check" {
		t.Fatalf("write response Tariffs = %+v, want exactly the staged tariff", payload.Tariffs)
	}
}

// A staged deletion of a published record surfaces as a Deletes entry in the
// PLURAL vocabulary this route family speaks (topics, not topic).
func TestPlaygroundDraft_DeleteStagesARemovalEntry(t *testing.T) {
	h := newHarness(t)
	live := h.liveView()
	if len(live.Topics) == 0 {
		t.Fatal("harness seed has no live topic to delete")
	}
	slug := stringField(live.Topics[0], "slug")

	status, _ := h.del("/xchats/api/v1/playground/draft/topics/" + slug)
	if status != 200 {
		t.Fatalf("delete status = %d", status)
	}
	changes := h.draftChanges()
	if len(changes.Deletes) != 1 {
		t.Fatalf("Deletes = %+v, want exactly one marker", changes.Deletes)
	}
	if stringField(changes.Deletes[0], "kind") != "topics" || stringField(changes.Deletes[0], "key") != slug {
		t.Fatalf("Deletes[0] = %+v, want {topics %s}", changes.Deletes[0], slug)
	}
}

func (h *harness) liveView() changeSetDTO {
	var out changeSetDTO
	h.get("/xchats/api/v1/kb", &out)
	return out
}

// --- CancelChange -----------------------------------------------------------

func TestPlaygroundCancelChange_RemovesAPendingAddition(t *testing.T) {
	h := newHarness(t)
	h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{"slug": "cancel-me", "title": "T", "body_md": "Body."})

	status, env := h.del("/xchats/api/v1/playground/draft/changes/topics/cancel-me")
	if status != 200 {
		t.Fatalf("cancel status = %d body=%s", status, env["payload"])
	}
	var out cancelResponseDTO
	mustPayload2(t, env, &out)
	if !out.Changed {
		t.Fatal("changed = false, want true")
	}
	if len(out.Changes.Topics) != 0 {
		t.Fatalf("Topics = %+v, want none after cancelling the only addition", out.Changes.Topics)
	}
}

func TestPlaygroundCancelChange_RemovesAStagedRemoval(t *testing.T) {
	h := newHarness(t)
	live := h.liveView()
	if len(live.Tariffs) == 0 {
		t.Fatal("harness seed has no live tariff")
	}
	ref := stringField(live.Tariffs[0], "ref")
	h.del("/xchats/api/v1/playground/draft/tariffs/" + ref)

	status, env := h.del("/xchats/api/v1/playground/draft/changes/tariffs/" + ref)
	if status != 200 {
		t.Fatalf("cancel status = %d", status)
	}
	var out cancelResponseDTO
	mustPayload2(t, env, &out)
	if !out.Changed {
		t.Fatal("changed = false, want true — a staged removal existed")
	}
	if len(out.Changes.Deletes) != 0 {
		t.Fatalf("Deletes = %+v, want none after cancel", out.Changes.Deletes)
	}
}

func TestPlaygroundCancelChange_SecondCallReturnsChangedFalseSameVersion(t *testing.T) {
	h := newHarness(t)
	h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{"slug": "twice", "title": "T", "body_md": "Body."})

	_, firstEnv := h.del("/xchats/api/v1/playground/draft/changes/topics/twice")
	var first cancelResponseDTO
	mustPayload2(t, firstEnv, &first)
	if !first.Changed {
		t.Fatal("first cancel: changed = false, want true")
	}

	status, secondEnv := h.del("/xchats/api/v1/playground/draft/changes/topics/twice")
	if status != 200 {
		t.Fatalf("second cancel status = %d", status)
	}
	var second cancelResponseDTO
	mustPayload2(t, secondEnv, &second)
	if second.Changed {
		t.Fatal("second cancel: changed = true, want false")
	}
	if second.Changes.BaseVersion != first.Changes.BaseVersion {
		t.Fatalf("second BaseVersion = %d, want unchanged %d", second.Changes.BaseVersion, first.Changes.BaseVersion)
	}
}

func TestPlaygroundCancelChange_UnknownKindIs400(t *testing.T) {
	h := newHarness(t)
	status, _ := h.del("/xchats/api/v1/playground/draft/changes/bogus/whatever")
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

// A PRESENT target with a stale If-Match is a genuine conflict (409) — the
// already-absent case (tested via the "second call" test above) is the one
// exemption, not this one.
func TestPlaygroundCancelChange_StaleIfMatchOnPresentTargetIs409(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{"slug": "stale-target", "title": "T", "body_md": "Body."})
	if resp.StatusCode != 200 {
		t.Fatalf("seed upsert status = %d", resp.StatusCode)
	}
	var afterFirst changeSetDTO
	mustPayload(t, env, &afterFirst)
	staleVersion := afterFirst.BaseVersion

	// Bump the version again so staleVersion is now behind current.
	resp2, _ := h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{"slug": "another", "title": "T2", "body_md": "Body."})
	if resp2.StatusCode != 200 {
		t.Fatalf("bump upsert status = %d", resp2.StatusCode)
	}

	resp3, env3 := h.delJSON("/xchats/api/v1/playground/draft/changes/topics/stale-target", map[string]string{"If-Match": strconv.FormatInt(staleVersion, 10)})
	if resp3.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d body=%s, want 409", resp3.StatusCode, env3["payload"])
	}
}

// --- approve entity -----------------------------------------------------------

func TestPlaygroundApproveEntity_MovesRowLiveAndClearsTheChange(t *testing.T) {
	h := newHarness(t)
	resp, _ := h.postJSON("/xchats/api/v1/playground/draft/tariffs", map[string]any{"ref": "approve-me", "name": "N", "price": "500"})
	if resp.StatusCode != 200 {
		t.Fatalf("upsert status = %d", resp.StatusCode)
	}

	resp2, _ := h.postJSON("/xchats/api/v1/playground/draft/approve/tariffs/approve-me", map[string]any{})
	if resp2.StatusCode != 200 {
		t.Fatalf("approve status = %d", resp2.StatusCode)
	}

	live := h.liveView()
	found := false
	for _, tr := range live.Tariffs {
		if stringField(tr, "ref") == "approve-me" {
			found = true
		}
	}
	if !found {
		t.Fatal("approved tariff did not appear live")
	}

	changes := h.draftChanges()
	for _, tr := range changes.Tariffs {
		if stringField(tr, "ref") == "approve-me" {
			t.Fatalf("tariff still pending after approve: %+v", tr)
		}
	}
}

// mustPayload2 is mustPayload's non-fatal-on-missing-key sibling for a
// payload shape (changed/changes) that isn't a flat DraftView.
func mustPayload2(t *testing.T, env map[string]json.RawMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(env["payload"], out); err != nil {
		t.Fatalf("payload %s: %v", env["payload"], err)
	}
}
