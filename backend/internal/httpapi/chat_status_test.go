package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// TestChatResolveReopen covers INB-04: the thread header's Resolve button used
// to have no handler at all. chat_state already existed in the schema
// (default 'open') but nothing ever wrote it outside the campaign-recipient
// path — PATCH .../status is the first caller that can.
func TestChatResolveReopen(t *testing.T) {
	h := newHarness(t)

	chatID, _ := h.inject(customerJID, "WA-STATUS-1", "привет", false)
	chats := h.listChats()
	if len(chats) != 1 || chats[0]["status"] != "open" {
		t.Fatalf("want a fresh chat with status=open, got %v", chats)
	}

	// resolve
	resp, env := h.patchJSON("/xchats/api/v1/chats/"+chatID+"/status", map[string]any{"status": "resolved"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resolve status %d", resp.StatusCode)
	}
	var chat map[string]any
	mustPayload(t, env, &chat)
	if chat["status"] != "resolved" {
		t.Fatalf("status after resolve = %v, want resolved", chat["status"])
	}
	// persists — a GET (not just the mutation response) reflects it too.
	if got := h.listChats(); got[0]["status"] != "resolved" {
		t.Fatalf("GET /chats after resolve = %v, want resolved", got[0]["status"])
	}

	// reopen
	resp, env = h.patchJSON("/xchats/api/v1/chats/"+chatID+"/status", map[string]any{"status": "open"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reopen status %d", resp.StatusCode)
	}
	mustPayload(t, env, &chat)
	if chat["status"] != "open" {
		t.Fatalf("status after reopen = %v, want open", chat["status"])
	}

	// an unknown status value is rejected outright, not silently written.
	resp, _ = h.patchJSON("/xchats/api/v1/chats/"+chatID+"/status", map[string]any{"status": "archived"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bogus status = %d, want 400", resp.StatusCode)
	}
	if got := h.listChats(); got[0]["status"] != "open" {
		t.Fatalf("rejected status mutation still applied: %v", got[0]["status"])
	}
}

// TestChatResolveCrossOrg confirms the same org guard every other chat-scoped
// mutation gets (orgChat) — a chat from one org can't be resolved through
// another org's session.
func TestChatResolveCrossOrg(t *testing.T) {
	h := newHarness(t)
	chatID, _ := h.inject(customerJID, "WA-STATUS-XORG", "привет", false)

	other := newHarness(t)
	resp, _ := other.patchJSON("/xchats/api/v1/chats/"+chatID+"/status", map[string]any{"status": "resolved"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-org resolve status %d, want 404", resp.StatusCode)
	}
}

// TestGetChat covers INB-16's backing endpoint: a deep link to a specific
// chat (restored from the URL, or opened from Customers/Followups) must
// resolve even when that chat is not on the list's first page, and must 404
// — not silently succeed — for a chat that does not exist or belongs to a
// different org.
func TestGetChat(t *testing.T) {
	h := newHarness(t)
	chatID, _ := h.inject(customerJID, "WA-GETCHAT-1", "привет", false)

	var chat map[string]any
	h.get("/xchats/api/v1/chats/"+chatID, &chat)
	if chat["id"] != chatID {
		t.Fatalf("GET /chats/:id id = %v, want %v", chat["id"], chatID)
	}
	if chat["status"] != "open" {
		t.Fatalf("GET /chats/:id status = %v, want open", chat["status"])
	}

	code, _ := h.getBytes("/xchats/api/v1/chats/" + uuid.NewString())
	if code != http.StatusNotFound {
		t.Fatalf("GET /chats/:id for a nonexistent id = %d, want 404", code)
	}

	other := newHarness(t)
	code, _ = other.getBytes("/xchats/api/v1/chats/" + chatID)
	if code != http.StatusNotFound {
		t.Fatalf("cross-org GET /chats/:id = %d, want 404", code)
	}
}

// TestDismissDrafts covers INB-14: Dismiss used to only clear Pinia state
// locally, so the same options came back on the next GET (refetch/reselect).
// It is now a real backend transition — 'suggested' -> 'dismissed' — which
// PendingDrafts (draft_state='suggested') then excludes for good.
func TestDismissDrafts(t *testing.T) {
	h := newHarness(t)
	chatID, _ := h.inject(customerJID, "WA-DISMISS-1", "привет", false)

	h.postJSON("/xchats/api/v1/chats/"+chatID+"/ai-drafts", map[string]any{})
	var drafts struct{ Items []map[string]any }
	h.get("/xchats/api/v1/chats/"+chatID+"/ai-drafts", &drafts)
	if len(drafts.Items) != 1 {
		t.Fatalf("want 1 draft option before dismiss, got %d", len(drafts.Items))
	}

	resp, env := h.postJSON("/xchats/api/v1/chats/"+chatID+"/ai-drafts/dismiss", map[string]any{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dismiss status %d", resp.StatusCode)
	}
	var dismissed struct{ Items []map[string]any }
	mustPayload(t, env, &dismissed)
	if len(dismissed.Items) != 1 || dismissed.Items[0]["status"] != "dismissed" {
		t.Fatalf("dismiss response = %v, want 1 item with status=dismissed", dismissed.Items)
	}

	// refetch — the SAME options must not come back (this is the bug: they
	// used to, because nothing on the backend ever recorded the dismissal).
	var after struct{ Items []map[string]any }
	h.get("/xchats/api/v1/chats/"+chatID+"/ai-drafts", &after)
	if len(after.Items) != 0 {
		t.Fatalf("dismissed draft reappeared on refetch: %v", after.Items)
	}
}
