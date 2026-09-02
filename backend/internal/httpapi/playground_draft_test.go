package httpapi_test

// playground_draft_test.go is the first HTTP-level test coverage of any
// /playground/draft route. It exists specifically to guard the Черновик
// redesign's contract: GET /playground/draft must return ONLY what is
// staged (kbstore.DraftChangeSet), never the old merged live-plus-draft
// view, and the new /playground/draft/changes/:kind/:key cancel endpoint
// must behave exactly as kbstore.CancelChange's own doc comment promises.
// KB business logic itself (idempotency edge cases, the vocabulary table)
// is exercised at the kbstore layer (changes_test.go) — this file only
// covers the transport: routing, status codes, and response shape.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// delJSON issues a DELETE with an optional If-Match (or other) header set —
// playground_draft_test.go's own addition to the harness, alongside
// get/postJSON/patchJSON, since CancelChange's stale-If-Match behavior is
// central to what this file tests.
func (h *harness) delJSON(path string, headers map[string]string) (*http.Response, map[string]json.RawMessage) {
	req, err := http.NewRequest(http.MethodDelete, h.srv.URL+path, bytes.NewReader(nil))
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

func TestPlaygroundDraft_ReturnsChangeSetNotMergedView(t *testing.T) {
	h := newHarness(t)

	status, body := h.getBytes("/xchats/api/v1/playground/draft")
	if status != http.StatusOK {
		t.Fatalf("GET /playground/draft: status=%d", status)
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	raw := env["payload"]

	// base_version must be present (optimistic concurrency for later writes).
	var probe struct {
		BaseVersion *int64 `json:"base_version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if probe.BaseVersion == nil {
		t.Fatalf("payload has no base_version: %s", raw)
	}

	// materials/requests are DraftView-only concepts — DraftChangeSet must not
	// carry them at all, not even as empty arrays.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := fields["materials"]; ok {
		t.Fatalf("payload carries materials — the merged DraftView shape leaked through: %s", raw)
	}
	if _, ok := fields["requests"]; ok {
		t.Fatalf("payload carries requests — the merged DraftView shape leaked through: %s", raw)
	}

	// Nothing is staged, yet the org has a full seeded live KB (topics,
	// tariffs, products, contacts, policies) — every collection must come
	// back empty, not "every live row with draft:false" (the old Draft()
	// merged-view behavior this endpoint used to have).
	var changes kbstore.DraftChangeSet
	if err := json.Unmarshal(raw, &changes); err != nil {
		t.Fatalf("unmarshal DraftChangeSet: %v", err)
	}
	if len(changes.Topics) != 0 {
		t.Fatalf("topics = %d, want 0 (an unchanged published topic leaked into the review payload)", len(changes.Topics))
	}
	if len(changes.Tariffs) != 0 || len(changes.Products) != 0 || len(changes.Contacts) != 0 || len(changes.Policies) != 0 || len(changes.Zones) != 0 {
		t.Fatalf("a published collection leaked into the review payload: %+v", changes)
	}
	if changes.Config != nil {
		t.Fatalf("config = %+v, want nil (no pending config edit)", changes.Config)
	}
}

func TestPlaygroundDraft_UpsertThenGetShowsOnlyThatRow(t *testing.T) {
	h := newHarness(t)

	_, _ = h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{
		"slug": "e2e-new-topic", "title": "New topic", "body_md": "Body.",
	})

	var changes kbstore.DraftChangeSet
	h.get("/xchats/api/v1/playground/draft", &changes)

	if len(changes.Topics) != 1 {
		t.Fatalf("topics = %d, want exactly 1 (the seeded live topics must not appear)", len(changes.Topics))
	}
	if changes.Topics[0].Slug != "e2e-new-topic" {
		t.Fatalf("topics[0].Slug = %q, want e2e-new-topic", changes.Topics[0].Slug)
	}
}

func TestPlaygroundDraft_WriteResponseIsTheChangeSet(t *testing.T) {
	h := newHarness(t)

	_, env := h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{
		"slug": "e2e-write-response", "title": "T", "body_md": "B.",
	})
	var changes kbstore.DraftChangeSet
	mustPayload(t, env, &changes)

	if len(changes.Topics) != 1 || changes.Topics[0].Slug != "e2e-write-response" {
		t.Fatalf("upsert response is not the change set: %+v", changes)
	}
}

func TestPlaygroundDraft_DeleteStagesARemovalEntry(t *testing.T) {
	h := newHarness(t) // seeds a live topic slug "pricing" (brain.SeedSnapshot)

	_, env := h.delJSON("/xchats/api/v1/playground/draft/topics/pricing", nil)
	var changes kbstore.DraftChangeSet
	mustPayload(t, env, &changes)

	if len(changes.Deletes) != 1 {
		t.Fatalf("deletes = %d, want 1: %+v", len(changes.Deletes), changes)
	}
	if changes.Deletes[0].Kind != "topics" || changes.Deletes[0].Key != "pricing" {
		t.Fatalf("deletes[0] = %+v, want {topics pricing}", changes.Deletes[0])
	}
}

func TestPlaygroundCancelChange_RemovesAPendingAddition(t *testing.T) {
	h := newHarness(t)
	h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{
		"slug": "e2e-cancel-add", "title": "T", "body_md": "B.",
	})

	resp, env := h.delJSON("/xchats/api/v1/playground/draft/changes/topics/e2e-cancel-add", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel: status=%d body=%s", resp.StatusCode, mustRaw(env))
	}
	var out struct {
		Changed bool                   `json:"changed"`
		Changes kbstore.DraftChangeSet `json:"changes"`
	}
	mustPayload(t, env, &out)
	if !out.Changed {
		t.Fatalf("changed = false, want true")
	}
	for _, tp := range out.Changes.Topics {
		if tp.Slug == "e2e-cancel-add" {
			t.Fatalf("e2e-cancel-add still present in change set after cancel")
		}
	}
}

func TestPlaygroundCancelChange_RemovesAStagedRemoval(t *testing.T) {
	h := newHarness(t) // "pricing" is live
	h.delJSON("/xchats/api/v1/playground/draft/topics/pricing", nil)

	resp, env := h.delJSON("/xchats/api/v1/playground/draft/changes/topics/pricing", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel: status=%d body=%s", resp.StatusCode, mustRaw(env))
	}
	var out struct {
		Changed bool                   `json:"changed"`
		Changes kbstore.DraftChangeSet `json:"changes"`
	}
	mustPayload(t, env, &out)
	if !out.Changed {
		t.Fatalf("changed = false, want true")
	}
	if len(out.Changes.Deletes) != 0 {
		t.Fatalf("deletes = %+v, want none (the staged removal must be cancelled)", out.Changes.Deletes)
	}

	// Live must be untouched — "pricing" still published.
	var live kbstore.DraftView
	h.get("/xchats/api/v1/kb", &live)
	found := false
	for _, tp := range live.Topics {
		if tp.Slug == "pricing" {
			found = true
		}
	}
	if !found {
		t.Fatalf("live topic \"pricing\" is gone — cancelling a staged removal must never touch live")
	}
}

func TestPlaygroundCancelChange_SecondCallReturnsChangedFalseSameVersion(t *testing.T) {
	h := newHarness(t)
	h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{
		"slug": "e2e-cancel-twice", "title": "T", "body_md": "B.",
	})

	_, env1 := h.delJSON("/xchats/api/v1/playground/draft/changes/topics/e2e-cancel-twice", nil)
	var first struct {
		Changed bool                   `json:"changed"`
		Changes kbstore.DraftChangeSet `json:"changes"`
	}
	mustPayload(t, env1, &first)
	if !first.Changed {
		t.Fatalf("first cancel: changed = false, want true")
	}

	resp2, env2 := h.delJSON("/xchats/api/v1/playground/draft/changes/topics/e2e-cancel-twice", nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second cancel: status=%d body=%s", resp2.StatusCode, mustRaw(env2))
	}
	var second struct {
		Changed bool                   `json:"changed"`
		Changes kbstore.DraftChangeSet `json:"changes"`
	}
	mustPayload(t, env2, &second)
	if second.Changed {
		t.Fatalf("second cancel: changed = true, want false — repeating a cancel must be a no-op")
	}
	if second.Changes.BaseVersion != first.Changes.BaseVersion {
		t.Fatalf("second cancel advanced base_version: %d -> %d, want unchanged",
			first.Changes.BaseVersion, second.Changes.BaseVersion)
	}
}

func TestPlaygroundCancelChange_UnknownKindIs400(t *testing.T) {
	h := newHarness(t)
	resp, _ := h.delJSON("/xchats/api/v1/playground/draft/changes/bogus/whatever", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestPlaygroundCancelChange_StaleIfMatchOnPresentTargetIs409(t *testing.T) {
	h := newHarness(t)
	_, env := h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{
		"slug": "e2e-cancel-stale", "title": "T", "body_md": "B.",
	})
	var staged kbstore.DraftChangeSet
	mustPayload(t, env, &staged)

	stale := staged.BaseVersion + 1000 // deliberately wrong
	resp, env2 := h.delJSON("/xchats/api/v1/playground/draft/changes/topics/e2e-cancel-stale",
		map[string]string{"If-Match": strconv.FormatInt(stale, 10)})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", resp.StatusCode, mustRaw(env2))
	}
}

// TestPlaygroundCancelChange_AbsentTargetIgnoresStaleIfMatch is
// CancelChange's step 3 at the HTTP layer: cancelling something that is not
// pending must succeed (changed:false) even with a wildly wrong If-Match —
// there is no payload to clobber, so 409 does not apply.
func TestPlaygroundCancelChange_AbsentTargetIgnoresStaleIfMatch(t *testing.T) {
	h := newHarness(t)
	resp, env := h.delJSON("/xchats/api/v1/playground/draft/changes/topics/never-staged",
		map[string]string{"If-Match": "999999"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, mustRaw(env))
	}
	var out struct {
		Changed bool `json:"changed"`
	}
	mustPayload(t, env, &out)
	if out.Changed {
		t.Fatalf("changed = true, want false")
	}
}

// TestPlaygroundApprove_GateViolationReturnsStructuredReasons is KB-09: the
// page-level 422 used to carry only a flat joined message, leaving the
// operator unable to tell WHICH staged entity actually caused a gate
// failure that a different, valid card's own publish attempt tripped
// over. The payload's reasons[] now names it directly.
func TestPlaygroundApprove_GateViolationReturnsStructuredReasons(t *testing.T) {
	h := newHarness(t)
	h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{
		"slug": "broken-token-topic", "title": "Broken", "body_md": "Цена {{tariff.x.price}}.",
	})

	resp, env := h.postJSON("/xchats/api/v1/playground/draft/approve", map[string]any{})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("approve with a gate violation: status=%d, want 422 (body=%s)", resp.StatusCode, mustRaw(env))
	}
	var payload struct {
		Reasons []struct {
			Kind    string `json:"kind"`
			Key     string `json:"key"`
			Message string `json:"message"`
		} `json:"reasons"`
	}
	mustPayload(t, env, &payload)
	if len(payload.Reasons) == 0 {
		t.Fatalf("expected at least one structured reason, got none (message=%s)", env["message"])
	}
	found := false
	for _, r := range payload.Reasons {
		if r.Kind == "topics" && r.Key == "broken-token-topic" {
			found = true
			if !strings.Contains(r.Message, "pure prose") {
				t.Errorf("reason message = %q, want it to explain the pure-prose rule", r.Message)
			}
		}
	}
	if !found {
		t.Fatalf("no reason named kind=topics key=broken-token-topic, got %+v", payload.Reasons)
	}
}

func TestPlaygroundApproveEntity_MovesRowLiveAndClearsTheChange(t *testing.T) {
	h := newHarness(t)
	h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{
		"slug": "e2e-approve-entity", "title": "Approved topic", "body_md": "B.",
	})

	resp, env := h.postJSON("/xchats/api/v1/playground/draft/approve/topics/e2e-approve-entity", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve: status=%d body=%s", resp.StatusCode, mustRaw(env))
	}
	var changes kbstore.DraftChangeSet
	mustPayload(t, env, &changes)
	for _, tp := range changes.Topics {
		if tp.Slug == "e2e-approve-entity" {
			t.Fatalf("approved topic still present in the change set")
		}
	}

	var live kbstore.DraftView
	h.get("/xchats/api/v1/kb", &live)
	found := false
	for _, tp := range live.Topics {
		if tp.Slug == "e2e-approve-entity" {
			found = true
		}
	}
	if !found {
		t.Fatalf("approved topic did not land in live /kb")
	}
}

// TestPlaygroundApproveAll_PublishesAndEmptiesTheWholeDraft covers
// POST /playground/draft/approve — the "Опубликовать всё" button — which had
// no functional test at all (only an OPTIONS preflight in cors_test.go, which
// never reaches the handler). That gap hid a slice-aliasing bug in
// kbstore.ApproveVersioned that published every staged row to live but left
// every OTHER one sitting in the draft, while still answering 200/OK — so the
// counter never reached zero and a second click was needed. See
// TestApproveVersioned_WholeDraftClearsEveryEntry for the mechanism.
//
// Five topics, not one: the bug leaves nothing behind at n<=2, which is why
// the entity-scoped test above (a single topic) passed throughout.
func TestPlaygroundApproveAll_PublishesAndEmptiesTheWholeDraft(t *testing.T) {
	h := newHarness(t)
	const n = 5
	slugs := make([]string, 0, n)
	for i := 0; i < n; i++ {
		slug := fmt.Sprintf("e2e-approve-all-%d", i)
		slugs = append(slugs, slug)
		h.postJSON("/xchats/api/v1/playground/draft/topics", map[string]any{
			"slug": slug, "title": "Topic " + slug, "body_md": "Plain prose, no tokens and no amounts.",
		})
	}

	resp, env := h.postJSON("/xchats/api/v1/playground/draft/approve", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve all: status=%d body=%s", resp.StatusCode, mustRaw(env))
	}

	// The handler answers with the POST-approve change set, so the response
	// body alone proves the draft was emptied.
	var changes kbstore.DraftChangeSet
	mustPayload(t, env, &changes)
	if got := len(changes.Topics); got != 0 {
		t.Fatalf("approve all returned %d topic(s) still pending, want 0 — the draft was not fully cleared", got)
	}

	var live kbstore.DraftView
	h.get("/xchats/api/v1/kb", &live)
	for _, want := range slugs {
		found := false
		for _, tp := range live.Topics {
			if tp.Slug == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("topic %q did not land in live /kb after approving the whole draft", want)
		}
	}
}

func mustRaw(env map[string]json.RawMessage) string {
	b, _ := json.Marshal(env)
	return string(b)
}
