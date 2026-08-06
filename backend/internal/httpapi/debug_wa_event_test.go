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

// TestDebugWaEventHTTP_Message drives POST /debug/wa-event over real HTTP —
// JSON body in, envelope out — rather than h.inject's in-process shortcut
// straight to whatsapp.Fake. This is the literal contract a QA script or an
// automated smoke test hits, per the whatsmeow migration's Phase 4 gate.
func TestDebugWaEventHTTP_Message(t *testing.T) {
	h := newHarness(t)

	resp, env := h.postJSON("/xchats/api/v1/debug/wa-event", map[string]any{
		"event_type": "message", "account_id": h.accountID.String(),
		"sender_jid": customerJID, "external_id": "HTTPMSG1", "text": "привет по HTTP",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %v", resp.StatusCode, env)
	}
	var out struct {
		OK        bool   `json:"ok"`
		MessageID string `json:"message_id"`
		ChatID    string `json:"chat_id"`
	}
	mustPayload(t, env, &out)
	if !out.OK || out.MessageID == "" || out.ChatID == "" {
		t.Fatalf("debug wa-event response = %+v", out)
	}

	found := false
	for _, m := range h.listMessages(out.ChatID) {
		if m["external_message_id"] == "HTTPMSG1" && m["content"] == "привет по HTTP" && m["direction"] == "in" {
			found = true
		}
	}
	if !found {
		t.Fatalf("injected message not visible via GET /chats/:id/messages: %+v", h.listMessages(out.ChatID))
	}
}

// TestDebugWaEventHTTP_Receipt asserts a receipt event posted to the same
// real HTTP endpoint advances the delivery ladder of a prior outbound send.
func TestDebugWaEventHTTP_Receipt(t *testing.T) {
	h := newHarness(t)
	chatID, _ := h.inject(customerJID, "HTTPINBOUND1", "hi", false)

	resp, _ := h.postJSON("/xchats/api/v1/chats/"+chatID+"/messages", map[string]any{"text": "reply"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send message status = %d, want 200", resp.StatusCode)
	}
	extID := stampedKeyOf(t, h, chatID, "reply")

	resp, env := h.postJSON("/xchats/api/v1/debug/wa-event", map[string]any{
		"event_type": "receipt", "account_id": h.accountID.String(),
		"external_id": extID, "receipt_type": "read",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %v", resp.StatusCode, env)
	}
	if got := deliveryOf(t, h, chatID, extID); got != "read" {
		t.Fatalf("delivery state = %q, want read", got)
	}
}

// TestDebugWaEventHTTP_ConnectionState drives the connected/disconnected/
// logged_out event types over real HTTP and confirms each is reflected by
// GET /wa-accounts/:id/status — the same status badge the accounts UI polls.
func TestDebugWaEventHTTP_ConnectionState(t *testing.T) {
	h := newHarness(t)

	for _, state := range []string{"disconnected", "logged_out", "connected"} {
		resp, env := h.postJSON("/xchats/api/v1/debug/wa-event", map[string]any{
			"event_type": state, "account_id": h.accountID.String(),
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("event_type=%s status = %d, want 200: %v", state, resp.StatusCode, env)
		}
		var out struct {
			ConnectionState string `json:"connection_state"`
		}
		h.get("/xchats/api/v1/wa-accounts/"+h.accountID.String()+"/status", &out)
		if out.ConnectionState != state {
			t.Fatalf("after event_type=%s, connection_state = %q", state, out.ConnectionState)
		}
	}
}

// TestDebugWaEventHTTP_DefaultsToSimulatorAccount asserts account_id can be
// omitted from the request body: the endpoint resolves (creating if needed)
// the caller's organization's simulator account, so the whole ingest path is
// reachable over HTTP with no WhatsApp number ever paired.
func TestDebugWaEventHTTP_DefaultsToSimulatorAccount(t *testing.T) {
	h := newHarness(t)

	resp, env := h.postJSON("/xchats/api/v1/debug/wa-event", map[string]any{
		"event_type": "message", "sender_jid": "77099999999@s.whatsapp.net",
		"external_id": "SIMHTTP1", "text": "no account_id given",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %v", resp.StatusCode, env)
	}
	var out struct {
		MessageID string `json:"message_id"`
		ChatID    string `json:"chat_id"`
	}
	mustPayload(t, env, &out)
	if out.MessageID == "" || out.ChatID == "" {
		t.Fatalf("response = %+v, want a message_id and chat_id", out)
	}
}

// TestDebugWaEventHTTP_ValidationErrors covers the two request-shape
// failures the handler itself must reject before ever reaching the manager.
func TestDebugWaEventHTTP_ValidationErrors(t *testing.T) {
	h := newHarness(t)

	resp, env := h.postJSON("/xchats/api/v1/debug/wa-event", map[string]any{
		"account_id": h.accountID.String(),
	})
	if resp.StatusCode != http.StatusBadRequest || errcodeOf(env) != "VALIDATION_ERROR" {
		t.Fatalf("missing event_type: status=%d errcode=%q, want 400 VALIDATION_ERROR", resp.StatusCode, errcodeOf(env))
	}

	resp, env = h.postJSON("/xchats/api/v1/debug/wa-event", map[string]any{
		"event_type": "not-a-real-type", "account_id": h.accountID.String(),
	})
	if resp.StatusCode != http.StatusBadRequest || errcodeOf(env) != "VALIDATION_ERROR" {
		t.Fatalf("unknown event_type: status=%d errcode=%q, want 400 VALIDATION_ERROR", resp.StatusCode, errcodeOf(env))
	}
}

// TestDebugWaEventHTTP_DisabledByDefault verifies the route simply doesn't
// exist unless SIMULATOR_ENABLED is set — mirroring the same guarantee
// /simulator/messages makes (see TestSimulatorMessage_DisabledByDefault).
func TestDebugWaEventHTTP_DisabledByDefault(t *testing.T) {
	srv := httpapi.New(httpapi.Deps{
		Cfg: &config.Config{System: config.SystemConfig{SimulatorEnabled: false}, Server: config.ServerConfig{CORSOrigins: []string{"*"}}},
		Log: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	})
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/xchats/api/v1/debug/wa-event", "application/json", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when the route is not registered", resp.StatusCode)
	}
}
