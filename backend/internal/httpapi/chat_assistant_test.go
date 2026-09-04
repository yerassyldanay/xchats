package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// chatAPI is the assistant's route prefix. The harness helpers take a full
// path, so every call below builds on this rather than repeating it.
const chatAPI = "/xchats/api/v1/chat"

// scriptedStreamingLLM is the /chat surface's model: it implements
// llm.StreamClient so the SSE path is genuinely exercised (rather than the
// Complete fallback), and it records the request it was handed so a test can
// assert on what the prompt actually carried.
type scriptedStreamingLLM struct {
	mu       sync.Mutex
	deltas   []string
	requests []llm.ChatRequest
}

// script sets what the next answer streams as. Defaults to a single chunk.
func (c *scriptedStreamingLLM) script(deltas ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deltas = deltas
}

func (c *scriptedStreamingLLM) lastRequest(t *testing.T) llm.ChatRequest {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.requests) == 0 {
		t.Fatal("the model was never called")
	}
	return c.requests[len(c.requests)-1]
}

func (c *scriptedStreamingLLM) answer(req llm.ChatRequest) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, req)
	if len(c.deltas) == 0 {
		return []string{"Ответ ассистента."}
	}
	return c.deltas
}

func (c *scriptedStreamingLLM) Complete(_ context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{Text: strings.Join(c.answer(req), ""), FinishReason: "stop"}, nil
}

func (c *scriptedStreamingLLM) Stream(_ context.Context, req llm.ChatRequest, onDelta func(string)) (llm.ChatResponse, error) {
	deltas := c.answer(req)
	var text strings.Builder
	for _, d := range deltas {
		text.WriteString(d)
		onDelta(d)
	}
	return llm.ChatResponse{Text: text.String(), FinishReason: "stop", PromptTokens: 100, CompletionTokens: 20}, nil
}

// sseEvent is one frame read off the chat stream.
type sseEvent struct {
	name string
	data string
}

// sendChatMessage POSTs one turn and drains the whole SSE response into
// ordered frames. The parsing is deliberately minimal and independent of the
// frontend's own parser — a bug shared by both would otherwise be invisible.
func (h *harness) sendChatMessage(conversationID, content string) []sseEvent {
	h.t.Helper()
	body, _ := json.Marshal(map[string]string{"content": content})
	resp, err := h.client.Post(
		h.srv.URL+chatAPI+"/conversations/"+conversationID+"/messages",
		"application/json", bytes.NewReader(body))
	if err != nil {
		h.t.Fatalf("send chat message: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		h.t.Fatalf("send chat message: status=%d body=%s", resp.StatusCode, raw)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		h.t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	var events []sseEvent
	var current sseEvent
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 8<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			current.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			current.data = strings.TrimPrefix(line, "data: ")
		case line == "" && current.name != "":
			events = append(events, current)
			current = sseEvent{}
		}
	}
	if err := sc.Err(); err != nil {
		h.t.Fatalf("read chat stream: %v", err)
	}
	return events
}

func eventNames(events []sseEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.name
	}
	return out
}

func firstEvent(t *testing.T, events []sseEvent, name string) sseEvent {
	t.Helper()
	for _, e := range events {
		if e.name == name {
			return e
		}
	}
	t.Fatalf("no %q event in %v", name, eventNames(events))
	return sseEvent{}
}

// newChatConversation creates a thread and returns its id.
func (h *harness) newChatConversation() string {
	h.t.Helper()
	resp, env := h.postJSON(chatAPI+"/conversations", map[string]string{})
	if resp.StatusCode != http.StatusCreated {
		h.t.Fatalf("create conversation: status=%d", resp.StatusCode)
	}
	var conversation struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(env["payload"], &conversation); err != nil {
		h.t.Fatalf("decode conversation: %v (%s)", err, env["payload"])
	}
	return conversation.ID
}

// seedChatKB gives the org a product that is priced differently in the live
// KB and in the draft — the spec's own worked example, and the shape every
// comparison assertion below needs.
func (h *harness) seedChatKB() {
	h.t.Helper()
	ctx := context.Background()
	availabilityStatus := "in_stock"
	live := kbstore.ProductInput{
		Ref: "vitamin-d", Name: "Vitamin D", Price: "12 000 KZT", AvailabilityStatus: &availabilityStatus, SalesStatus: "active",
	}
	actor := h.adminUserID(h.t)
	if err := h.kb.PutLiveProduct(ctx, h.orgID, actor, live); err != nil {
		h.t.Fatalf("seed live product: %v", err)
	}
	staged := live
	staged.Price = "10 800 KZT"
	if err := h.kb.UpsertProduct(ctx, h.orgID, actor, staged); err != nil {
		h.t.Fatalf("stage draft product: %v", err)
	}
}

// --- tests -------------------------------------------------------------------

func TestChatStreamsAnAnswerWithItsKnowledgeBaseComponents(t *testing.T) {
	h := newHarness(t)
	h.seedChatKB()
	h.chatLLM.script("Черновая цена ", "на 1 200 ₸ ниже.")

	conversationID := h.newChatConversation()
	events := h.sendChatMessage(conversationID, "Сравни текущую и черновую цену Vitamin D")

	names := eventNames(events)
	if len(names) < 4 || names[0] != "message_created" || names[len(names)-1] != "done" {
		t.Fatalf("event sequence = %v, want message_created first and done last", names)
	}
	// Cards are computed from the knowledge base, not from the answer, so
	// they arrive before the first token rather than after the last.
	componentsAt, firstDeltaAt := indexOf(names, "components"), indexOf(names, "text_delta")
	if componentsAt < 0 || firstDeltaAt < 0 || componentsAt > firstDeltaAt {
		t.Errorf("components arrived at %d and the first delta at %d — cards must not wait for the answer", componentsAt, firstDeltaAt)
	}

	// The comparison carries both states, explicitly labelled.
	var components []struct {
		Type string `json:"type"`
		Data struct {
			Key    string `json:"key"`
			Change string `json:"change"`
			Fields []struct {
				Key   string `json:"key"`
				Real  string `json:"real"`
				Draft string `json:"draft"`
			} `json:"fields"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(firstEvent(t, events, "components").data), &components); err != nil {
		t.Fatalf("decode components: %v", err)
	}
	if len(components) != 1 || components[0].Type != "kb_comparison" {
		t.Fatalf("components = %+v, want one kb_comparison", components)
	}
	data := components[0].Data
	if data.Key != "vitamin-d" || data.Change != "updated" {
		t.Errorf("compared %s/%s, want vitamin-d/updated", data.Key, data.Change)
	}
	if len(data.Fields) != 1 || data.Fields[0].Real != "12 000 KZT" || data.Fields[0].Draft != "10 800 KZT" {
		t.Errorf("comparison fields = %+v, want the live and draft prices", data.Fields)
	}

	// Every delta reaches the client separately, and `done` carries the
	// persisted turn with its components attached.
	var deltas []string
	for _, e := range events {
		if e.name != "text_delta" {
			continue
		}
		var payload struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(e.data), &payload); err != nil {
			t.Fatalf("decode delta: %v", err)
		}
		deltas = append(deltas, payload.Text)
	}
	if len(deltas) != 2 || strings.Join(deltas, "") != "Черновая цена на 1 200 ₸ ниже." {
		t.Errorf("deltas = %v, want the answer delivered in two chunks", deltas)
	}

	var done struct {
		ID         string `json:"id"`
		Role       string `json:"role"`
		Content    string `json:"content"`
		Components []struct {
			Type string `json:"type"`
		} `json:"components"`
	}
	if err := json.Unmarshal([]byte(firstEvent(t, events, "done").data), &done); err != nil {
		t.Fatalf("decode done: %v", err)
	}
	if done.Role != "assistant" || done.Content != "Черновая цена на 1 200 ₸ ниже." {
		t.Errorf("done = %+v, want the assistant's full answer", done)
	}
	if len(done.Components) != 1 {
		t.Errorf("the persisted turn carries %d components, want 1", len(done.Components))
	}

	// The id announced up front is the id the answer is persisted under —
	// that is what lets a client key a streaming bubble on it.
	var started struct {
		AssistantID string `json:"assistant_id"`
		User        struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"user"`
	}
	if err := json.Unmarshal([]byte(firstEvent(t, events, "message_created").data), &started); err != nil {
		t.Fatalf("decode message_created: %v", err)
	}
	if started.AssistantID != done.ID {
		t.Errorf("assistant id = %s up front, %s once persisted", started.AssistantID, done.ID)
	}
	if started.User.Role != "user" || started.User.Content != "Сравни текущую и черновую цену Vitamin D" {
		t.Errorf("message_created carried %+v, want the operator's own question", started.User)
	}
}

// The whole conversation is persisted and reloads with its cards intact — a
// reopened thread must render exactly as it did live.
func TestChatConversationReloadsWithItsTranscript(t *testing.T) {
	h := newHarness(t)
	h.seedChatKB()
	h.chatLLM.script("Ответ.")

	conversationID := h.newChatConversation()
	h.sendChatMessage(conversationID, "Что изменилось в черновике?")

	var detail struct {
		Conversation struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"conversation"`
		Messages []struct {
			Role       string `json:"role"`
			Content    string `json:"content"`
			Components []struct {
				Type string `json:"type"`
			} `json:"components"`
		} `json:"messages"`
	}
	h.get(chatAPI+"/conversations/"+conversationID, &detail)

	if len(detail.Messages) != 2 {
		t.Fatalf("transcript holds %d messages, want the question and the answer", len(detail.Messages))
	}
	if detail.Messages[0].Role != "user" || detail.Messages[1].Role != "assistant" {
		t.Errorf("roles = %s, %s — want the transcript oldest first", detail.Messages[0].Role, detail.Messages[1].Role)
	}
	if len(detail.Messages[1].Components) != 1 {
		t.Errorf("the reloaded answer carries %d components, want 1", len(detail.Messages[1].Components))
	}
	// The first message names an untitled thread.
	if detail.Conversation.Title != "Что изменилось в черновике?" {
		t.Errorf("title = %q, want the first message", detail.Conversation.Title)
	}
}

// The prompt is what makes the whole feature safe, so its shape is asserted
// end to end and not only in internal/chat's own tests.
func TestChatPromptCarriesBothKnowledgeBaseStates(t *testing.T) {
	h := newHarness(t)
	h.seedChatKB()

	conversationID := h.newChatConversation()
	h.sendChatMessage(conversationID, "Какая цена Vitamin D?")

	req := h.chatLLM.lastRequest(t)
	if len(req.Messages) < 2 || req.Messages[0].Role != "system" {
		t.Fatalf("messages = %+v, want a system message first", req.Messages)
	}
	system := req.Messages[0].Content
	for _, want := range []string{"REAL_KB", "DRAFT_KB", "12 000 KZT", "10 800 KZT"} {
		if !strings.Contains(system, want) {
			t.Errorf("system prompt is missing %q", want)
		}
	}
	last := req.Messages[len(req.Messages)-1]
	if last.Role != "user" || last.Content != "Какая цена Vitamin D?" {
		t.Errorf("last message = %s/%q, want the operator's question", last.Role, last.Content)
	}
}

func TestChatConversationLifecycle(t *testing.T) {
	h := newHarness(t)

	conversationID := h.newChatConversation()

	var listed struct {
		Conversations []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"conversations"`
	}
	h.get(chatAPI+"/conversations", &listed)
	if len(listed.Conversations) != 1 || listed.Conversations[0].ID != conversationID {
		t.Fatalf("conversations = %+v, want exactly the created one", listed.Conversations)
	}

	resp, _ := h.patchJSON(chatAPI+"/conversations/"+conversationID, map[string]string{"title": "Обзор цен"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename: status=%d", resp.StatusCode)
	}
	h.get(chatAPI+"/conversations", &listed)
	if listed.Conversations[0].Title != "Обзор цен" {
		t.Errorf("title = %q after rename", listed.Conversations[0].Title)
	}

	req, _ := http.NewRequest(http.MethodDelete, h.srv.URL+chatAPI+"/conversations/"+conversationID, nil)
	delResp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete: status=%d", delResp.StatusCode)
	}
	h.get(chatAPI+"/conversations", &listed)
	if len(listed.Conversations) != 0 {
		t.Errorf("conversations = %+v after delete, want none", listed.Conversations)
	}
}

// Failures the client can cause before a stream opens stay ordinary HTTP
// errors — a caller that got a 404 never had a stream to parse.
func TestChatRejectsBadRequestsBeforeOpeningAStream(t *testing.T) {
	h := newHarness(t)
	conversationID := h.newChatConversation()

	empty, payload := h.postJSON(chatAPI+"/conversations/"+conversationID+"/messages", map[string]string{"content": "   "})
	if empty.StatusCode != http.StatusBadRequest {
		t.Errorf("empty message: status=%d, want 400", empty.StatusCode)
	}
	_ = payload

	unknown, _ := h.postJSON(chatAPI+"/conversations/"+uuid.New().String()+"/messages", map[string]string{"content": "hi"})
	if unknown.StatusCode != http.StatusNotFound {
		t.Errorf("unknown conversation: status=%d, want 404", unknown.StatusCode)
	}

	malformed, _ := h.postJSON(chatAPI+"/conversations/not-a-uuid/messages", map[string]string{"content": "hi"})
	if malformed.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed id: status=%d, want 400", malformed.StatusCode)
	}
}

func indexOf(list []string, want string) int {
	for i, v := range list {
		if v == want {
			return i
		}
	}
	return -1
}
