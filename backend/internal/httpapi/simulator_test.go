package httpapi_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
)

func TestSimulatorMessage_SyncPathPersistsOneDraft(t *testing.T) {
	h := newHarness(t)

	resp, env := h.postJSON("/xchats/api/v1/simulator/messages", map[string]any{
		"contact_ref": "sim-contact-1", "conversation_ref": "sim-conv-1",
		"text": "Здравствуйте!", "wait_for_response": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var out struct {
		ConversationID string `json:"conversation_id"`
		MessageID      string `json:"message_id"`
		Draft          struct {
			ID            string `json:"id"`
			Text          string `json:"text"`
			ReplyLanguage string `json:"reply_language"`
			Escalate      bool   `json:"escalate"`
		} `json:"draft"`
	}
	mustPayload(t, env, &out)
	if out.ConversationID == "" || out.MessageID == "" || out.Draft.ID == "" {
		t.Fatalf("missing ids in response: %+v", out)
	}
	if out.Draft.ReplyLanguage != "ru" {
		t.Fatalf("reply_language = %q, want ru", out.Draft.ReplyLanguage)
	}
	// newHarness's org has a live KB (kb.SeedLiveIfEmpty), so this reaches the
	// fake LLM client and returns its scripted response verbatim — this test
	// verifies the HTTP plumbing end to end, not engine correctness (covered by
	// backend/response's own tests).
	if out.Draft.Text != "Секунду, уточню и вернусь." || out.Draft.Escalate {
		t.Fatalf("unexpected draft: %+v", out.Draft)
	}

	// Re-injecting the same conversation_ref must reuse the SAME conversation,
	// not create a second one.
	resp2, env2 := h.postJSON("/xchats/api/v1/simulator/messages", map[string]any{
		"contact_ref": "sim-contact-1", "conversation_ref": "sim-conv-1",
		"text": "Второе сообщение", "wait_for_response": true,
	})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second call status = %d, want 200", resp2.StatusCode)
	}
	var out2 struct {
		ConversationID string `json:"conversation_id"`
	}
	mustPayload(t, env2, &out2)
	if out2.ConversationID != out.ConversationID {
		t.Fatalf("conversation_ref reuse: got a new conversation %q, want %q", out2.ConversationID, out.ConversationID)
	}
}

func TestSimulatorMessage_AsyncPathEnqueuesAndReturns202(t *testing.T) {
	h := newHarness(t)

	resp, env := h.postJSON("/xchats/api/v1/simulator/messages", map[string]any{
		"contact_ref": "sim-contact-2", "conversation_ref": "sim-conv-2",
		"text": "Привет", "wait_for_response": false,
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}
	var out struct {
		ConversationID string `json:"conversation_id"`
		MessageID      string `json:"message_id"`
	}
	mustPayload(t, env, &out)
	if out.ConversationID == "" || out.MessageID == "" {
		t.Fatalf("missing ids in async response: %+v", out)
	}

	// postJSON already drains the queue (h.queue.Wait()); the async ai_draft
	// task must have completed by now (asserting there's no lingering panic /
	// unhandled task, not asserting draft content — see the sync test above).
	var drafts struct{ Items []map[string]any }
	h.get("/xchats/api/v1/chats/"+out.ConversationID+"/ai-drafts", &drafts)
	if len(drafts.Items) != 1 {
		t.Fatalf("want exactly 1 persisted draft after the async task runs, got %d", len(drafts.Items))
	}
}

func TestSimulatorMessage_ValidationErrors(t *testing.T) {
	h := newHarness(t)

	cases := []map[string]any{
		{"contact_ref": "", "conversation_ref": "c", "text": "x"},
		{"contact_ref": "a", "conversation_ref": "", "text": "x"},
		{"contact_ref": "a", "conversation_ref": "c", "text": ""},
		{"contact_ref": "a", "conversation_ref": "c", "text": "x", "provider": "openrouter"}, // model missing
	}
	for _, body := range cases {
		resp, _ := h.postJSON("/xchats/api/v1/simulator/messages", body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("body %+v: status = %d, want 400", body, resp.StatusCode)
		}
	}
}

func TestSimulatorMessage_UnregisteredProviderRejected(t *testing.T) {
	h := newHarness(t)
	resp, _ := h.postJSON("/xchats/api/v1/simulator/messages", map[string]any{
		"contact_ref": "a", "conversation_ref": "c", "text": "x",
		"provider": "not-a-real-provider", "model": "not-a-real-model",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unregistered provider", resp.StatusCode)
	}
}

// TestSimulatorMessage_DisabledByDefault verifies the route simply doesn't
// exist unless SIMULATOR_ENABLED is set — no store or LLM dependency needed,
// since an unregistered route never reaches per-group middleware.
func TestSimulatorMessage_DisabledByDefault(t *testing.T) {
	srv := httpapi.New(httpapi.Deps{
		Cfg: &config.Config{SimulatorEnabled: false, CORSOrigins: []string{"*"}},
		Log: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	})
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/xchats/api/v1/simulator/messages", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when the route is not registered", resp.StatusCode)
	}
}
