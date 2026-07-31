package httpapi_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"testing"
)

// End-to-end over HTTP: drop a text material → build → confirm the detected fact
// via the resolve endpoint → approve one topic → GET /draft shows it live (not
// draft). Exercises the playground routing, the envelope, and applyResolution
// (confirm_fact → typed tariff column).
func TestPlayground_BuildConfirmApprove(t *testing.T) {
	h := newHarness(t)

	var draft struct {
		Config struct {
			BaseVersion int64 `json:"base_version"`
		} `json:"config"`
		Topics []struct {
			ID     string `json:"id"`
			Slug   string `json:"slug"`
			BodyMD string `json:"body_md"`
			Draft  bool   `json:"draft"`
		} `json:"topics"`
	}
	// GET is side-effect-free and always returns the merged view (live seed rows;
	// nothing pending yet).
	h.get("/xchats/api/v1/playground/draft", &draft)
	if len(draft.Topics) == 0 {
		t.Fatal("the seeded live topics should already be in the merged view")
	}
	for _, tp := range draft.Topics {
		if tp.Draft {
			t.Fatalf("freshly-seeded topic %q should not be flagged draft", tp.Slug)
		}
	}

	// 1. Drop raw material with a price.
	resp, _ := h.postJSON("/xchats/api/v1/playground/draft/materials",
		map[string]string{"text": "Тариф Рост — 25 000 ₸/мес, поддержка включена."})
	if resp.StatusCode != 201 {
		t.Fatalf("create material: status %d", resp.StatusCode)
	}

	// 2. Build the draft from materials.
	_, env := h.postJSON("/xchats/api/v1/playground/chat",
		map[string]string{"instruction": "Собери базу знаний по тарифу Рост"})
	var chat struct {
		Result struct {
			TopicsUpserted int `json:"topics_upserted"`
			Confirms       int `json:"confirms"`
		} `json:"result"`
	}
	mustPayload(t, env, &chat)
	if chat.Result.TopicsUpserted != 1 || chat.Result.Confirms != 1 {
		t.Fatalf("build: topics=%d confirms=%d", chat.Result.TopicsUpserted, chat.Result.Confirms)
	}

	// 3. Find the confirm_fact popup and the pending (draft) topic.
	var reqs struct {
		Items []struct {
			ID      string `json:"id"`
			ReqType string `json:"req_type"`
		} `json:"items"`
	}
	h.get("/xchats/api/v1/playground/requests", &reqs)
	var confirmID string
	for _, r := range reqs.Items {
		if r.ReqType == "confirm_fact" {
			confirmID = r.ID
		}
	}
	if confirmID == "" {
		t.Fatal("no confirm_fact request raised")
	}

	// 4. Confirm the fact (accept the detected amount onto its typed column) via the
	// resolve endpoint — the answer carries {value}.
	resp, _ = h.postJSON("/xchats/api/v1/playground/requests/"+confirmID+"/resolve",
		map[string]any{"resolution": map[string]string{"value": "25 000 ₸/мес"}})
	if resp.StatusCode != 200 {
		t.Fatalf("resolve: status %d", resp.StatusCode)
	}

	// 5. Approve the one pending topic by its natural key (slug).
	h.get("/xchats/api/v1/playground/draft", &draft)
	var topicSlug string
	for _, tp := range draft.Topics {
		if tp.Draft {
			topicSlug = tp.Slug
		}
	}
	if topicSlug == "" {
		t.Fatal("no pending topic to approve")
	}
	resp, env = h.postJSON("/xchats/api/v1/playground/draft/approve/topics/"+topicSlug, map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("approve topic: status %d body=%v", resp.StatusCode, env)
	}

	// 6. The approved topic is now live — no draft flag — and the value confirmed
	// by the popup materialized alongside it, so nothing pending remains.
	h.get("/xchats/api/v1/playground/draft", &draft)
	for _, tp := range draft.Topics {
		if tp.Slug == topicSlug && tp.Draft {
			t.Fatalf("topic %q should be live after approve, still flagged draft", topicSlug)
		}
	}
}

// mustPayload is defined in integration_test.go (same test package).

// The approve gate blocks (422) when a topic body is not pure prose (carries a
// fact token instead of quoting the fact only in replies).
func TestPlayground_ApproveGateBlocksUnconfirmed(t *testing.T) {
	h := newHarness(t)

	resp, _ := h.postJSON("/xchats/api/v1/playground/draft/topics",
		map[string]string{"slug": "broken", "body_md": "Цена {{tariff.ghost.price}}"})
	if resp.StatusCode != 200 {
		t.Fatalf("upsert topic: %d", resp.StatusCode)
	}
	resp, env := h.postJSON("/xchats/api/v1/playground/draft/approve", map[string]any{})
	if resp.StatusCode != 422 {
		t.Fatalf("want 422 gate failure, got %d (%v)", resp.StatusCode, env)
	}
}

// /kb/* is the live-only editor: GET returns the seeded live rows directly (no
// draft/blob concept at all), and a write applies immediately — no approve step
// — while staying fully invisible to the Playground draft view.
func TestKB_GetAndUpsertTopic_AppliesLiveImmediately(t *testing.T) {
	h := newHarness(t)

	var kbView struct {
		Topics []struct {
			Slug   string `json:"slug"`
			Draft  bool   `json:"draft"`
			BodyMD string `json:"body_md"`
		} `json:"topics"`
		Materials []any `json:"materials"`
		Requests  []any `json:"requests"`
	}
	h.get("/xchats/api/v1/kb", &kbView)
	if len(kbView.Topics) == 0 {
		t.Fatal("the seeded live topics should already be in /kb")
	}
	for _, tp := range kbView.Topics {
		if tp.Draft {
			t.Fatalf("every /kb row must be draft:false, got %+v", tp)
		}
	}
	if kbView.Materials == nil || kbView.Requests == nil {
		t.Fatal("/kb materials/requests must be [] (non-nil), never null")
	}

	// A write applies straight to the live table — no approve step.
	resp, _ := h.postJSON("/xchats/api/v1/kb/topics",
		map[string]string{"slug": "kb_live_topic", "body_md": "Обычный текст без фактов."})
	if resp.StatusCode != 200 {
		t.Fatalf("upsert live topic: %d", resp.StatusCode)
	}
	h.get("/xchats/api/v1/kb", &kbView)
	found := false
	for _, tp := range kbView.Topics {
		if tp.Slug == "kb_live_topic" {
			found = true
			if tp.Draft {
				t.Fatal("a /kb write must never be flagged draft")
			}
		}
	}
	if !found {
		t.Fatal("the live-written topic should appear in /kb immediately")
	}

	// It must NOT leak into the Playground draft view as a pending entity — the
	// two flows are fully separate.
	var draft struct {
		Topics []struct {
			Slug  string `json:"slug"`
			Draft bool   `json:"draft"`
		} `json:"topics"`
	}
	h.get("/xchats/api/v1/playground/draft", &draft)
	for _, tp := range draft.Topics {
		if tp.Slug == "kb_live_topic" && tp.Draft {
			t.Fatal("a /kb write must not show up as a pending draft entity")
		}
	}
}

// The /kb topic write is held to the same gate the approve path enforces: a
// fact token in the body is rejected (422), not silently written live.
func TestKB_UpsertTopic_RejectsNonProseBody(t *testing.T) {
	h := newHarness(t)
	resp, env := h.postJSON("/xchats/api/v1/kb/topics",
		map[string]string{"slug": "kb_broken", "body_md": "Цена {{tariff.ghost.price}}"})
	if resp.StatusCode != 422 {
		t.Fatalf("want 422 gate failure, got %d (%v)", resp.StatusCode, env)
	}
}

// uploadMaterial posts a multipart file + optional description straight to
// POST /playground/draft/materials (distinct from harness.upload, which targets
// the chat-media endpoint) and decodes the returned KbMaterial fields we assert on.
func (h *harness) uploadMaterial(filename, ct, description string, data []byte) (status int, matStatus, extractedText string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(map[string][]string)
	hdr["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="file"; filename=%q`, filename)}
	hdr["Content-Type"] = []string{ct}
	pw, _ := mw.CreatePart(hdr)
	pw.Write(data)
	if description != "" {
		_ = mw.WriteField("description", description)
	}
	mw.Close()
	req, _ := http.NewRequest("POST", h.srv.URL+"/xchats/api/v1/playground/draft/materials", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("upload material: %v", err)
	}
	defer resp.Body.Close()
	var m struct {
		Status        string `json:"status"`
		ExtractedText string `json:"extracted_text"`
	}
	decodeEnvelope(h.t, resp, &m)
	return resp.StatusCode, m.Status, m.ExtractedText
}

// A file uploaded WITH a description over HTTP is born ready immediately (no
// extraction job, no describe_file popup) — the description substitutes for
// auto-extraction (see kbstore.MaterialInput.Description).
func TestPlayground_UploadMaterial_WithDescription_BornReady(t *testing.T) {
	h := newHarness(t)
	status, matStatus, text := h.uploadMaterial("shot.png", "image/png", "Скриншот прайса: Базовый 9900.", []byte("\x89PNG..."))
	if status != 201 {
		t.Fatalf("create material: status %d", status)
	}
	if matStatus != "ready" {
		t.Fatalf("described upload should be ready immediately, got %q", matStatus)
	}
	if text != "Скриншот прайса: Базовый 9900." {
		t.Fatalf("extracted_text should be the description verbatim, got %q", text)
	}
}

// zoneRow is the JSON shape of a kbstore.ZoneRow entry within DraftView.Zones.
type zoneRow struct {
	Ref               string `json:"ref"`
	Name              string `json:"name"`
	ZoneLevel         string `json:"zone_level"`
	DeliveryAvailable bool   `json:"delivery_available"`
	DeliveryCost      string `json:"delivery_cost"`
	DeliveryInDays    string `json:"delivery_in_days"`
	SalesStatus       string `json:"sales_status"`
	Draft             bool   `json:"draft"`
}

func findZone(zones []zoneRow, ref string) *zoneRow {
	for i := range zones {
		if zones[i].Ref == ref {
			return &zones[i]
		}
	}
	return nil
}

// TestPlaygroundZones_UpsertApproveDeleteRoundTrip is Task 13's regression
// guard: plan/mcp.md's own delivery-zone milestone required draft support at
// every layer — routes, kbstore.UpsertZone/DeleteZone, and the approve-entity
// whitelist — not just the kbstore plumbing MCP already exercised. This drives
// the actual HTTP routes (POST/DELETE /playground/draft/zones,
// /playground/draft/approve/delivery_zones/:ref), the same way an operator's
// browser would, end to end: create as a pending zone → approve it into live →
// delete it → approve the deletion.
func TestPlaygroundZones_UpsertApproveDeleteRoundTrip(t *testing.T) {
	h := newHarness(t)

	// zoneGateReasons (zones.go) requires every policy row to carry blank flat
	// delivery fields and a non-blank outside_zones_note the moment ANY zone
	// exists — the seeded org starts with the opposite (flat delivery, no
	// zones), so clear that first and approve it into live before a zone can
	// ever pass the gate.
	resp, env := h.patchJSON("/xchats/api/v1/playground/draft/policies", map[string]any{
		"delivery_cost": "", "delivery_in_days": "", "outside_zones_note": "Доставка только в перечисленные зоны.",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("patch policies: status %d body=%v", resp.StatusCode, env)
	}
	resp, env = h.postJSON("/xchats/api/v1/playground/draft/approve", map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("approve policy edit: status %d body=%v", resp.StatusCode, env)
	}

	resp, _ = h.postJSON("/xchats/api/v1/playground/draft/zones", map[string]any{
		"ref": "almaty", "name": "Алматы", "zone_level": "city",
		"delivery_available": true, "delivery_cost": "1500", "delivery_in_days": "1-2",
	})
	if resp.StatusCode != 200 {
		t.Fatalf("upsert zone: status %d", resp.StatusCode)
	}

	var draft struct {
		Zones []zoneRow `json:"zones"`
	}
	h.get("/xchats/api/v1/playground/draft", &draft)
	z := findZone(draft.Zones, "almaty")
	if z == nil {
		t.Fatal("newly upserted zone should appear in the merged draft view")
	}
	if !z.Draft {
		t.Fatal("a brand new zone should be flagged draft (pending) before approval")
	}
	if z.SalesStatus != "active" {
		t.Fatalf("sales_status should default to active when omitted, got %q", z.SalesStatus)
	}

	// Approve it individually by natural key — exercises the
	// handlePlaygroundApproveEntity whitelist entry for delivery_zones.
	resp, env = h.postJSON("/xchats/api/v1/playground/draft/approve/delivery_zones/almaty", map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("approve zone: status %d body=%v", resp.StatusCode, env)
	}

	h.get("/xchats/api/v1/playground/draft", &draft)
	z = findZone(draft.Zones, "almaty")
	if z == nil || z.Draft {
		t.Fatalf("approved zone should be live (draft=false), got %+v", z)
	}

	// Delete it — a live zone with no pending edit stages a plain deletion.
	// Draft() previews the RESULTING state (like every other entity kind, a
	// pending deletion is suppressed from the merged view immediately), so the
	// only observable difference between "staged" and "approved" is on the
	// live side: GET /kb must still show it until the deletion is approved.
	status, delEnv := h.del("/xchats/api/v1/playground/draft/zones/almaty")
	if status != 200 {
		t.Fatalf("delete zone: status %d body=%v", status, delEnv)
	}
	h.get("/xchats/api/v1/playground/draft", &draft)
	if findZone(draft.Zones, "almaty") != nil {
		t.Fatal("a zone staged for deletion should be suppressed from the merged draft view")
	}
	var live struct {
		Zones []zoneRow `json:"zones"`
	}
	h.get("/xchats/api/v1/kb", &live)
	if findZone(live.Zones, "almaty") == nil {
		t.Fatal("the zone must still be live — only staged for deletion, not yet approved")
	}

	// Approve the deletion — now it must actually disappear from live.
	resp, env = h.postJSON("/xchats/api/v1/playground/draft/approve/delivery_zones/almaty", map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("approve deletion: status %d body=%v", resp.StatusCode, env)
	}
	h.get("/xchats/api/v1/kb", &live)
	if findZone(live.Zones, "almaty") != nil {
		t.Fatal("zone should be gone from the live view after the deletion is approved")
	}
}
