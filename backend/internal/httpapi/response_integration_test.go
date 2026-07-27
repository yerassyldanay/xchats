package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/llm"
)

// groundedFakeLLMClient scripts a reply that names a real fact placeholder
// from the harness's seeded KB (brain.SeedSnapshot's "coffee-machine" product,
// price "129 900 ₸") — unlike fakeLLMClient's fixed reply, this lets a test
// assert on end-to-end grounding: the engine must substitute the placeholder
// with the real value before the draft is ever persisted.
type groundedFakeLLMClient struct{}

func (groundedFakeLLMClient) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{Text: `{"reply_text":"Кофемашина DeLonghi стоит {{product.coffee-machine.price}}.",` +
		`"reply_language":"ru","media_files_to_send":[],"escalate":false}`}, nil
}

// TestWhatsAppInboundProducesGroundedDraftAndApprovalDelivers is the
// WhatsApp-channel end-to-end proof this milestone replaces production
// drafting with the evaluated pipeline: a real inbound webhook is stored,
// auto-queues the response engine, persists exactly one grounded draft (the
// model's placeholder substituted with the KB's real price, no literal `{{`
// left), and approving that draft delivers it through the fake Evolution
// sender — never a direct Evolution call from the response service itself.
func TestWhatsAppInboundProducesGroundedDraftAndApprovalDelivers(t *testing.T) {
	h := newHarnessWithLLM(t, groundedFakeLLMClient{})

	// 1. Inbound webhook: a genuine customer message, not the (broken) stale
	// capture() fixture — craftInbound (accounts_test.go) builds one inline.
	h.webhook(craftInbound(ownerJID, customerJID, "wa-grounded-1", "Сколько стоит кофемашина?"))

	chats := h.listChats()
	if len(chats) != 1 {
		t.Fatalf("want 1 chat after the inbound webhook, got %d", len(chats))
	}
	chatID := chats[0]["id"].(string)
	msgs := h.listMessages(chatID)
	if len(msgs) != 1 || msgs[0]["content"] != "Сколько стоит кофемашина?" {
		t.Fatalf("inbound text not stored as expected: %v", msgs)
	}
	if msgs[0]["direction"] != "in" || msgs[0]["sender_type"] != "contact" {
		t.Fatalf("wrong direction/sender on the stored inbound message: %v", msgs[0])
	}

	// 2. h.webhook already drained the queue, so the auto-triggered AIDraftTask
	// has run to completion: exactly one grounded draft must be persisted.
	var drafts struct {
		Items []map[string]any `json:"items"`
	}
	h.get("/xchats/api/v1/chats/"+chatID+"/ai-drafts", &drafts)
	if len(drafts.Items) != 1 {
		t.Fatalf("want exactly 1 persisted draft, got %d: %v", len(drafts.Items), drafts.Items)
	}
	draft := drafts.Items[0]
	text, _ := draft["draft_text"].(string)
	if !strings.Contains(text, "129 900 ₸") {
		t.Fatalf("draft text = %q, want the substituted real price 129 900 ₸", text)
	}
	if strings.Contains(text, "{{") {
		t.Fatalf("draft text still has an unsubstituted placeholder: %q", text)
	}
	if escalate, _ := draft["escalate"].(bool); escalate {
		t.Fatalf("draft escalate = %v, want false", draft["escalate"])
	}
	if draft["reply_language"] != "ru" {
		t.Fatalf("draft reply_language = %v, want ru", draft["reply_language"])
	}

	// 3. Approving the draft must deliver it — through the channel-neutral
	// sender registry's fake Evolution sender, since response.Service itself
	// has no route to Evolution at all (see backend/response's dependency test).
	sendTextBefore := len(h.fake.CallsFor("sendText"))
	draftID := draft["id"].(string)
	resp, _ := h.postJSON("/xchats/api/v1/ai-drafts/"+draftID+"/approve", map[string]any{"media_ids": []string{}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", resp.StatusCode)
	}
	calls := h.fake.CallsFor("sendText")
	if len(calls) != sendTextBefore+1 {
		t.Fatalf("approve should have sent exactly one text via the fake Evolution sender; before=%d after=%d", sendTextBefore, len(calls))
	}
	last := calls[len(calls)-1]
	if !strings.Contains(last.Text, "129 900 ₸") {
		t.Fatalf("delivered text = %q, want the substituted price", last.Text)
	}
	if last.Number != customerNum {
		t.Fatalf("delivered to %q, want the customer phone %q (not the @lid)", last.Number, customerNum)
	}
	if !aiMessageExists(t, h, chatID) {
		t.Fatalf("approved send was not recorded as sender_kind='ai'")
	}
}

// TestSimulatorApprovalNeverCallsEvolution is the full-stack complement to
// internal/simulator's TestChannelSender_PackageHasNoNetworkingCapability:
// that test proves the simulator channel adapter cannot reach the network by
// construction; this one proves the running server actually routes a
// simulator-channel draft approval to that adapter — not to the fake
// Evolution client sitting in the very same messaging.SenderRegistry, which a
// channel-routing bug could easily send it to instead.
func TestSimulatorApprovalNeverCallsEvolution(t *testing.T) {
	h := newHarness(t)

	resp, env := h.postJSON("/xchats/api/v1/simulator/messages", map[string]any{
		"contact_ref": "sim-no-network-contact", "conversation_ref": "sim-no-network-conv",
		"text": "Здравствуйте!", "wait_for_response": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("simulator message status = %d, want 200", resp.StatusCode)
	}
	var out struct {
		ConversationID string `json:"conversation_id"`
		Draft          struct {
			ID string `json:"id"`
		} `json:"draft"`
	}
	mustPayload(t, env, &out)
	if out.Draft.ID == "" {
		t.Fatal("missing draft id")
	}

	callsBefore := len(h.fake.Calls)

	resp2, _ := h.postJSON("/xchats/api/v1/ai-drafts/"+out.Draft.ID+"/approve", map[string]any{"media_ids": []string{}})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", resp2.StatusCode)
	}

	if got := len(h.fake.Calls); got != callsBefore {
		t.Fatalf("simulator approval made %d fake Evolution call(s), want 0 (routed to the WhatsApp fake instead of the simulator sender)", got-callsBefore)
	}
	if !aiMessageExists(t, h, out.ConversationID) {
		t.Fatalf("approved simulator send was not recorded as sender_kind='ai'")
	}
}
