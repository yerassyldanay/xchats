package chat_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/chat"
	"github.com/yerassyldanay/xchats/backend/internal/chatkb"
	"github.com/yerassyldanay/xchats/backend/internal/chatstore"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// --- fakes -----------------------------------------------------------------

// fakeKB is a chatkb.Service that returns fixed snapshots — the retrieval
// seam the Service is written against, standing in for kbstore.
type fakeKB struct {
	real, draft chatkb.Snapshot
	err         error
	queries     []string
}

func (f *fakeKB) SearchReal(_ context.Context, _ uuid.UUID, query string) (chatkb.Snapshot, error) {
	f.queries = append(f.queries, query)
	return f.real, f.err
}

func (f *fakeKB) SearchDraft(_ context.Context, _ uuid.UUID, query string) (chatkb.Snapshot, error) {
	f.queries = append(f.queries, query)
	return f.draft, f.err
}

// fakeLLM records the request it was handed and replays a canned answer,
// delta by delta when asked to stream.
type fakeLLM struct {
	deltas   []string
	err      error
	requests []llm.ChatRequest
	streamed bool
}

func (f *fakeLLM) Complete(_ context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return llm.ChatResponse{}, f.err
	}
	return llm.ChatResponse{Text: strings.Join(f.deltas, ""), FinishReason: "stop"}, nil
}

func (f *fakeLLM) Stream(_ context.Context, req llm.ChatRequest, onDelta func(string)) (llm.ChatResponse, error) {
	f.requests = append(f.requests, req)
	f.streamed = true
	if f.err != nil {
		return llm.ChatResponse{}, f.err
	}
	var text strings.Builder
	for _, d := range f.deltas {
		text.WriteString(d)
		onDelta(d)
	}
	return llm.ChatResponse{Text: text.String(), FinishReason: "stop", PromptTokens: 42, CompletionTokens: 7}, nil
}

// completeOnlyLLM is a provider adapter that cannot stream — the fallback
// path Service.complete must still handle.
type completeOnlyLLM struct{ text string }

func (c completeOnlyLLM) Complete(_ context.Context, _ llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{Text: c.text, FinishReason: "stop"}, nil
}

type fakeRegistry struct {
	client llm.ChatClient
	err    error
}

func (f fakeRegistry) Client(llm.ModelRef) (llm.ChatClient, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.client, nil
}

// --- harness ---------------------------------------------------------------

type harness struct {
	svc   *chat.Service
	scope chat.Scope
	kb    *fakeKB
	llm   *fakeLLM
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	cs, st, _ := dbtest.NewChat(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "acme")
	if err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, "alice@acme.test", "hash", "Alice")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	kb := &fakeKB{
		real: chatkb.Snapshot{Source: chatkb.SourceReal, Records: []chatkb.Record{{
			Kind: chatkb.KindProducts, Key: "vitamin-d", Title: "Vitamin D", Source: chatkb.SourceReal,
			Fields: []chatkb.Field{{Key: "price", Label: "Price", Value: "12 000 KZT"}},
		}}},
		draft: chatkb.Snapshot{Source: chatkb.SourceDraft, Records: []chatkb.Record{{
			Kind: chatkb.KindProducts, Key: "vitamin-d", Title: "Vitamin D", Source: chatkb.SourceDraft,
			Fields: []chatkb.Field{{Key: "price", Label: "Price", Value: "10 800 KZT"}},
		}}},
	}
	model := &fakeLLM{deltas: []string{"The draft price ", "is 1 200 KZT lower."}}

	svc := chat.New(chat.Deps{
		Store: cs, KB: kb, LLMs: fakeRegistry{client: model},
		Params: func() chat.Params {
			return chat.Params{
				Model:         llm.ModelRef{Provider: "openrouter", Model: "google/gemini-2.5-flash"},
				Temperature:   0.3,
				MaxTokens:     2000,
				HistoryWindow: 4,
			}
		},
	})
	return &harness{svc: svc, scope: chat.Scope{OrgID: org.ID, UserID: user.ID}, kb: kb, llm: model}
}

// recorder captures everything a Sink is handed, in order.
type recorder struct {
	user        chat.Message
	assistantID uuid.UUID
	components  []chatkb.Component
	deltas      []string
	done        chat.Message
	order       []string
}

func (r *recorder) sink() chat.Sink {
	return chat.Sink{
		Started: func(user chat.Message, assistantID uuid.UUID) {
			r.user, r.assistantID = user, assistantID
			r.order = append(r.order, "started")
		},
		Components: func(components []chatkb.Component) {
			r.components = components
			r.order = append(r.order, "components")
		},
		Delta: func(text string) {
			r.deltas = append(r.deltas, text)
			r.order = append(r.order, "delta")
		},
		Done: func(assistant chat.Message) {
			r.done = assistant
			r.order = append(r.order, "done")
		},
	}
}

// --- tests -----------------------------------------------------------------

func TestSendStreamsPersistsAndAttachesComponents(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	conv, err := h.svc.CreateConversation(ctx, h.scope, "")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	var rec recorder
	assistant, err := h.svc.Send(ctx, h.scope,
		chat.SendInput{ConversationID: conv.ID, Text: "What is the draft price of Vitamin D?"}, rec.sink())
	if err != nil {
		t.Fatalf("send: %v", err)
	}

	if !h.llm.streamed {
		t.Error("a streaming-capable client must be streamed, not Completed")
	}
	if got := strings.Join(rec.deltas, ""); got != "The draft price is 1 200 KZT lower." {
		t.Errorf("streamed text = %q, want the full answer", got)
	}
	if assistant.Content != strings.Join(rec.deltas, "") {
		t.Errorf("persisted content = %q, want the same text that was streamed", assistant.Content)
	}
	if assistant.ID != rec.assistantID {
		t.Errorf("persisted id = %s, want the %s announced up front", assistant.ID, rec.assistantID)
	}
	if rec.done.ID != assistant.ID {
		t.Errorf("done event carried id %s, want %s", rec.done.ID, assistant.ID)
	}

	// The KB card is computed from retrieval, so it is emitted before the
	// first token rather than after the answer.
	if len(rec.components) != 1 || rec.components[0].Type != chatkb.ComponentComparison {
		t.Fatalf("components = %+v, want one kb_comparison", rec.components)
	}
	if got := strings.Join(rec.order[:3], ","); got != "started,components,delta" {
		t.Errorf("event order began %q, want started,components,delta", got)
	}
	if rec.order[len(rec.order)-1] != "done" {
		t.Errorf("last event = %q, want done", rec.order[len(rec.order)-1])
	}

	// Components survive a reload: the transcript renders identically to the
	// live stream.
	detail, err := h.svc.Conversation(ctx, h.scope, conv.ID)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if len(detail.Messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(detail.Messages))
	}
	if len(detail.Messages[1].Components) != 1 {
		t.Errorf("reloaded assistant message has %d components, want 1", len(detail.Messages[1].Components))
	}
	if len(detail.Messages[0].Components) != 0 {
		t.Errorf("a user message must carry no components, got %+v", detail.Messages[0].Components)
	}

	// Provenance is recorded alongside the answer.
	var meta struct {
		Provider         string `json:"provider"`
		Model            string `json:"model"`
		PromptTokens     int    `json:"prompt_tokens"`
		CompletionTokens int    `json:"completion_tokens"`
		KBPendingChanges int    `json:"kb_pending_changes"`
	}
	if err := json.Unmarshal(detail.Messages[1].Metadata, &meta); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if meta.Provider != "openrouter" || meta.Model != "google/gemini-2.5-flash" {
		t.Errorf("metadata provider/model = %s/%s, want openrouter/google/gemini-2.5-flash", meta.Provider, meta.Model)
	}
	if meta.PromptTokens != 42 || meta.CompletionTokens != 7 {
		t.Errorf("metadata usage = %d/%d, want 42/7", meta.PromptTokens, meta.CompletionTokens)
	}
	if meta.KBPendingChanges != 1 {
		t.Errorf("metadata kb_pending_changes = %d, want 1", meta.KBPendingChanges)
	}
}

// The prompt is system + last N turns + the current question, in that order,
// with the KB context inside the system message (spec §4).
func TestSendBuildsThePromptFromSystemHistoryAndQuestion(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	conv, err := h.svc.CreateConversation(ctx, h.scope, "")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	for _, q := range []string{"first question", "second question", "third question"} {
		if _, err := h.svc.Send(ctx, h.scope, chat.SendInput{ConversationID: conv.ID, Text: q}, chat.Sink{}); err != nil {
			t.Fatalf("send %q: %v", q, err)
		}
	}

	req := h.llm.requests[len(h.llm.requests)-1]
	if req.Model != "google/gemini-2.5-flash" {
		t.Errorf("model = %q, want the configured one", req.Model)
	}
	if req.MaxTokens != 2000 || req.Temperature != 0.3 {
		t.Errorf("max_tokens/temperature = %d/%v, want 2000/0.3", req.MaxTokens, req.Temperature)
	}
	if req.Messages[0].Role != "system" {
		t.Fatalf("messages[0].Role = %q, want system", req.Messages[0].Role)
	}
	// Both KB states, explicitly labelled, live in the system message.
	system := req.Messages[0].Content
	for _, want := range []string{"REAL_KB", "DRAFT_KB", "12 000 KZT", "10 800 KZT"} {
		if !strings.Contains(system, want) {
			t.Errorf("system prompt is missing %q", want)
		}
	}
	last := req.Messages[len(req.Messages)-1]
	if last.Role != "user" || last.Content != "third question" {
		t.Errorf("last message = %s/%q, want user/%q", last.Role, last.Content, "third question")
	}
	// HistoryWindow is 4: the two turns before this one, plus system and the
	// current question.
	if len(req.Messages) != 6 {
		t.Fatalf("len(messages) = %d, want 6 (system + 4 history + current); got %+v", len(req.Messages), req.Messages)
	}
	if req.Messages[1].Content != "first question" {
		t.Errorf("oldest replayed turn = %q, want %q", req.Messages[1].Content, "first question")
	}
}

// The window bounds what the model sees; the transcript keeps everything.
func TestHistoryWindowBoundsThePromptNotThePersistedTranscript(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	conv, err := h.svc.CreateConversation(ctx, h.scope, "")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	for _, q := range []string{"q1", "q2", "q3", "q4", "q5"} {
		if _, err := h.svc.Send(ctx, h.scope, chat.SendInput{ConversationID: conv.ID, Text: q}, chat.Sink{}); err != nil {
			t.Fatalf("send %q: %v", q, err)
		}
	}

	req := h.llm.requests[len(h.llm.requests)-1]
	if len(req.Messages) != 6 {
		t.Errorf("prompt carried %d messages, want 6 with a window of 4", len(req.Messages))
	}
	if strings.Contains(req.Messages[1].Content, "q1") {
		t.Error("the oldest turn is still in the prompt — the window is not being applied")
	}

	detail, err := h.svc.Conversation(ctx, h.scope, conv.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(detail.Messages) != 10 {
		t.Errorf("transcript holds %d messages, want all 10 (5 exchanges)", len(detail.Messages))
	}
	if detail.Messages[0].Content != "q1" {
		t.Errorf("transcript[0] = %q, want the very first question", detail.Messages[0].Content)
	}
}

func TestSendNamesAnUntitledConversationFromItsFirstMessage(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	conv, err := h.svc.CreateConversation(ctx, h.scope, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := h.svc.Send(ctx, h.scope,
		chat.SendInput{ConversationID: conv.ID, Text: "What is the draft price of Vitamin D?"}, chat.Sink{}); err != nil {
		t.Fatalf("send: %v", err)
	}
	detail, err := h.svc.Conversation(ctx, h.scope, conv.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if detail.Conversation.Title != "What is the draft price of Vitamin D?" {
		t.Errorf("title = %q, want the first message", detail.Conversation.Title)
	}

	// A second message must not rename it.
	if _, err := h.svc.Send(ctx, h.scope,
		chat.SendInput{ConversationID: conv.ID, Text: "And Omega 3?"}, chat.Sink{}); err != nil {
		t.Fatalf("second send: %v", err)
	}
	detail, err = h.svc.Conversation(ctx, h.scope, conv.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if detail.Conversation.Title != "What is the draft price of Vitamin D?" {
		t.Errorf("title = %q after a second message, want it unchanged", detail.Conversation.Title)
	}
}

func TestSendKeepsAnExplicitTitle(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	conv, err := h.svc.CreateConversation(ctx, h.scope, "Pricing review")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := h.svc.Send(ctx, h.scope,
		chat.SendInput{ConversationID: conv.ID, Text: "What is the draft price of Vitamin D?"}, chat.Sink{}); err != nil {
		t.Fatalf("send: %v", err)
	}
	detail, err := h.svc.Conversation(ctx, h.scope, conv.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if detail.Conversation.Title != "Pricing review" {
		t.Errorf("title = %q, want the explicit one to survive", detail.Conversation.Title)
	}
}

// A provider failure must not cost the operator their own question.
func TestSendPersistsTheQuestionEvenWhenTheModelFails(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.llm.err = errors.New("provider exploded")

	conv, err := h.svc.CreateConversation(ctx, h.scope, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := h.svc.Send(ctx, h.scope,
		chat.SendInput{ConversationID: conv.ID, Text: "will this survive?"}, chat.Sink{}); err == nil {
		t.Fatal("send succeeded, want the provider error surfaced")
	}

	detail, err := h.svc.Conversation(ctx, h.scope, conv.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(detail.Messages) != 1 {
		t.Fatalf("transcript holds %d messages, want just the question", len(detail.Messages))
	}
	if detail.Messages[0].Role != chatstore.RoleUser || detail.Messages[0].Content != "will this survive?" {
		t.Errorf("kept %+v, want the operator's own question", detail.Messages[0])
	}
}

func TestSendRejectsAnEmptyMessage(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	conv, err := h.svc.CreateConversation(ctx, h.scope, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := h.svc.Send(ctx, h.scope, chat.SendInput{ConversationID: conv.ID, Text: "   \n "}, chat.Sink{}); !errors.Is(err, chat.ErrEmptyMessage) {
		t.Errorf("err = %v, want ErrEmptyMessage", err)
	}
}

func TestSendToAnUnknownConversationIsNotFound(t *testing.T) {
	h := newHarness(t)
	_, err := h.svc.Send(context.Background(), h.scope, chat.SendInput{ConversationID: uuid.New(), Text: "hi"}, chat.Sink{})
	if !errors.Is(err, chat.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// A provider adapter with no streaming support must still answer — the whole
// point of StreamClient being a separate interface.
func TestSendFallsBackToCompleteForANonStreamingProvider(t *testing.T) {
	cs, st, _ := dbtest.NewChat(t)
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "acme")
	if err != nil {
		t.Fatalf("seed organization: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, "alice@acme.test", "hash", "Alice")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	svc := chat.New(chat.Deps{
		Store:  cs,
		KB:     &fakeKB{},
		LLMs:   fakeRegistry{client: completeOnlyLLM{text: "all at once"}},
		Params: func() chat.Params { return chat.Params{Model: llm.ModelRef{Provider: "p", Model: "m"}} },
	})
	scope := chat.Scope{OrgID: org.ID, UserID: user.ID}
	conv, err := svc.CreateConversation(ctx, scope, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var rec recorder
	msg, err := svc.Send(ctx, scope, chat.SendInput{ConversationID: conv.ID, Text: "hello"}, rec.sink())
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if msg.Content != "all at once" {
		t.Errorf("content = %q, want the completed answer", msg.Content)
	}
	if len(rec.deltas) != 1 || rec.deltas[0] != "all at once" {
		t.Errorf("deltas = %v, want the whole answer delivered as one", rec.deltas)
	}
}

// An empty KB must be reported as empty, not passed off as a silent gap the
// model fills with invention.
func TestAnEmptyKnowledgeBaseIsStatedExplicitly(t *testing.T) {
	h := newHarness(t)
	h.kb.real = chatkb.Snapshot{Source: chatkb.SourceReal}
	h.kb.draft = chatkb.Snapshot{Source: chatkb.SourceDraft}
	ctx := context.Background()

	conv, err := h.svc.CreateConversation(ctx, h.scope, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := h.svc.Send(ctx, h.scope, chat.SendInput{ConversationID: conv.ID, Text: "anything?"}, chat.Sink{}); err != nil {
		t.Fatalf("send: %v", err)
	}
	system := h.llm.requests[0].Messages[0].Content
	if !strings.Contains(system, "completely empty") {
		t.Errorf("system prompt does not state the KB is empty:\n%s", system)
	}
}

// Retrieval failures must fail the turn rather than answering from one state.
func TestRetrievalFailureFailsTheTurn(t *testing.T) {
	h := newHarness(t)
	h.kb.err = errors.New("kb unavailable")
	ctx := context.Background()

	conv, err := h.svc.CreateConversation(ctx, h.scope, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := h.svc.Send(ctx, h.scope, chat.SendInput{ConversationID: conv.ID, Text: "price?"}, chat.Sink{}); err == nil {
		t.Fatal("send succeeded despite a KB retrieval failure")
	}
	if len(h.llm.requests) != 0 {
		t.Error("the model was called even though retrieval failed")
	}
}
