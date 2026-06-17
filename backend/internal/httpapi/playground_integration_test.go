package httpapi_test

import (
	"testing"
)

// End-to-end over HTTP: drop a text material → build → confirm the detected value
// via the resolve endpoint → approve the proposed topic → publish. Exercises the
// playground routing, the envelope, and applyResolution (confirm_value → value).
func TestPlayground_BuildConfirmPublish(t *testing.T) {
	h := newHarness(t)

	// POST /draft opens the working copy (clones the published seed); GET is read-only.
	var draft struct {
		Config struct {
			Version int `json:"version"`
		} `json:"config"`
		Topics []struct {
			ID     string `json:"id"`
			Slug   string `json:"slug"`
			BodyMD string `json:"body_md"`
			Review string `json:"review_state"`
		} `json:"topics"`
	}
	// Open the draft explicitly — GET is side-effect-free; POST clones the seed.
	_, openEnv := h.postJSON("/xchats/api/v1/playground/draft", map[string]any{})
	mustPayload(t, openEnv, &draft)
	if draft.Config.Version != 0 {
		t.Fatalf("draft version should be 0, got %d", draft.Config.Version)
	}
	if len(draft.Topics) == 0 {
		t.Fatal("opened draft should clone the seed topics")
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

	// 3. Find the confirm_value popup and the proposed topic.
	var reqs struct {
		Items []struct {
			ID      string `json:"id"`
			ReqType string `json:"req_type"`
		} `json:"items"`
	}
	h.get("/xchats/api/v1/playground/requests", &reqs)
	var confirmID string
	for _, r := range reqs.Items {
		if r.ReqType == "confirm_value" {
			confirmID = r.ID
		}
	}
	if confirmID == "" {
		t.Fatal("no confirm_value request raised")
	}

	// 4. Confirm the value (accept the detected text) via the resolve endpoint.
	resp, _ = h.postJSON("/xchats/api/v1/playground/requests/"+confirmID+"/resolve",
		map[string]any{"resolution": map[string]string{"value_text": "25 000 ₸/мес"}})
	if resp.StatusCode != 200 {
		t.Fatalf("resolve: status %d", resp.StatusCode)
	}

	// 5. Approve the proposed topic.
	h.get("/xchats/api/v1/playground/draft", &draft)
	var topicID string
	for _, tp := range draft.Topics {
		if tp.Review == "proposed" {
			topicID = tp.ID
		}
	}
	if topicID == "" {
		t.Fatal("no proposed topic to approve")
	}
	resp, _ = h.postJSON("/xchats/api/v1/playground/draft/review/topics/"+topicID,
		map[string]string{"state": "approved"})
	if resp.StatusCode != 200 {
		t.Fatalf("approve topic: status %d", resp.StatusCode)
	}

	// 6. Publish → gate passes, version bumps to 2.
	resp, env = h.postJSON("/xchats/api/v1/playground/publish", map[string]any{})
	if resp.StatusCode != 200 {
		t.Fatalf("publish: status %d body=%v", resp.StatusCode, env)
	}
	var pub struct {
		Version int `json:"version"`
	}
	mustPayload(t, env, &pub)
	if pub.Version != 2 {
		t.Fatalf("want published version 2, got %d", pub.Version)
	}
}

// mustPayload is defined in integration_test.go (same test package).

// The publish gate blocks (422) when a proposed topic uses an unconfirmed token.
func TestPlayground_PublishGateBlocksUnconfirmed(t *testing.T) {
	h := newHarness(t)
	h.postJSON("/xchats/api/v1/playground/draft", map[string]any{}) // open the draft (GET no longer auto-opens)

	resp, _ := h.postJSON("/xchats/api/v1/playground/draft/topics",
		map[string]string{"slug": "broken", "lang": "ru", "body_md": "Цена {{price.ghost}}"})
	if resp.StatusCode != 200 {
		t.Fatalf("upsert topic: %d", resp.StatusCode)
	}
	resp, env := h.postJSON("/xchats/api/v1/playground/publish", map[string]any{})
	if resp.StatusCode != 422 {
		t.Fatalf("want 422 gate failure, got %d (%v)", resp.StatusCode, env)
	}
}
