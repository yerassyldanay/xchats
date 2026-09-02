package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/chatkb"
	"github.com/yerassyldanay/xchats/backend/internal/chatstore"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// ErrNotFound is returned when a conversation does not exist within the
// caller's scope — the same sentinel chatstore returns, re-exported so the
// HTTP layer maps one error to one status code.
var ErrNotFound = chatstore.ErrNotFound

// ErrEmptyMessage is returned when a request carries no message text. A
// blank turn would be persisted, sent, billed, and answered with nothing.
var ErrEmptyMessage = errors.New("chat: message is empty")

// Deps are the Service's collaborators — every one of them an interface or a
// repository package, none of them constructed here (see the package doc).
type Deps struct {
	Store *chatstore.Store
	KB    chatkb.Service
	LLMs  llm.Registry
	// Params resolves the model configuration for the NEXT request. A
	// function rather than a value so a Settings-UI change applies without a
	// restart, exactly like response.Engine.Params.
	Params func() Params
	// RenderOptions bounds the KB text one request may carry; the zero value
	// uses chatkb's own default budget.
	RenderOptions chatkb.RenderOptions
	Log           *slog.Logger
}

// Service is the chat feature's entry point.
type Service struct {
	store         *chatstore.Store
	kb            chatkb.Service
	llms          llm.Registry
	params        func() Params
	renderOptions chatkb.RenderOptions
	log           *slog.Logger
}

// New builds a Service. A nil Log is replaced by a discarding logger so
// every call site can log unconditionally.
func New(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{
		store: d.Store, kb: d.KB, llms: d.LLMs, params: d.Params,
		renderOptions: d.RenderOptions, log: log,
	}
}

// Scope is the (organization, user) pair every operation is keyed by.
type Scope = chatstore.Scope

// Conversation is one thread's header.
type Conversation = chatstore.Conversation

// ConversationDetail is a thread plus its full transcript — what the UI
// loads when a conversation is opened.
type ConversationDetail struct {
	Conversation Conversation `json:"conversation"`
	Messages     []Message    `json:"messages"`
}

// --- conversation lifecycle ------------------------------------------------

// CreateConversation starts an empty thread. It stays untitled until its
// first message names it (see Send).
func (s *Service) CreateConversation(ctx context.Context, scope Scope, title string) (Conversation, error) {
	return s.store.CreateConversation(ctx, scope, strings.TrimSpace(title))
}

// ListConversations returns the scope's threads, most recently active first.
func (s *Service) ListConversations(ctx context.Context, scope Scope, limit int) ([]Conversation, error) {
	return s.store.ListConversations(ctx, scope, limit)
}

// Conversation returns one thread with its whole transcript.
func (s *Service) Conversation(ctx context.Context, scope Scope, id uuid.UUID) (ConversationDetail, error) {
	conv, err := s.store.Conversation(ctx, scope, id)
	if err != nil {
		return ConversationDetail{}, err
	}
	msgs, err := s.store.Messages(ctx, scope, id)
	if err != nil {
		return ConversationDetail{}, err
	}
	return ConversationDetail{Conversation: conv, Messages: toMessages(msgs)}, nil
}

// RenameConversation sets a thread's title explicitly.
func (s *Service) RenameConversation(ctx context.Context, scope Scope, id uuid.UUID, title string) (Conversation, error) {
	return s.store.SetTitle(ctx, scope, id, strings.TrimSpace(title))
}

// DeleteConversation removes a thread and its transcript.
func (s *Service) DeleteConversation(ctx context.Context, scope Scope, id uuid.UUID) error {
	return s.store.DeleteConversation(ctx, scope, id)
}

// --- sending a message -----------------------------------------------------

// Sink receives one assistant turn as it is produced. Every field is
// optional; a nil one is skipped. Implemented once, by the SSE handler —
// the Service itself knows nothing about SSE framing.
type Sink struct {
	// Started fires once the operator's turn is persisted, carrying it and
	// the id the assistant's turn WILL have. The id is settled up front so
	// the UI can key a streaming bubble on it before any text exists.
	Started func(user Message, assistantID uuid.UUID)
	// Components fires once, after KB retrieval and before any text — the
	// cards are computed from the KB, not from the answer, so they need not
	// wait for it.
	Components func(components []chatkb.Component)
	// Delta fires for each incremental piece of the answer.
	Delta func(text string)
	// Done fires once with the persisted assistant turn.
	Done func(assistant Message)
}

// SendInput is one operator turn.
type SendInput struct {
	ConversationID uuid.UUID
	Text           string
}

// Send runs one full turn: persist the operator's message, retrieve both KB
// states, emit the structured components, stream the model's answer, and
// persist it with those components attached.
//
// The operator's message is persisted BEFORE the model is called, so a
// provider failure loses the answer but never the question — the operator's
// own words are theirs, not something to discard because a third party was
// unavailable. The assistant's turn is persisted only once complete: a
// half-streamed answer that a caller disconnected from is not a fact worth
// keeping, and re-asking is cheap.
//
// A nil Sink field is skipped, so Send is equally usable without streaming.
func (s *Service) Send(ctx context.Context, scope Scope, in SendInput, sink Sink) (Message, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return Message{}, ErrEmptyMessage
	}
	conv, err := s.store.Conversation(ctx, scope, in.ConversationID)
	if err != nil {
		return Message{}, err
	}

	// Read the history BEFORE appending, so the window is the conversation
	// as it stood when the question was asked — appending first would spend
	// one of N slots on the message we are about to send separately.
	params := s.resolveParams()
	history, err := s.store.RecentMessages(ctx, scope, conv.ID, params.HistoryWindow)
	if err != nil {
		return Message{}, err
	}

	userMsg, err := s.store.AppendMessage(ctx, scope, conv.ID, chatstore.AppendInput{
		Role: chatstore.RoleUser, Content: text,
	})
	if err != nil {
		return Message{}, err
	}
	// The first message names the thread — asked of the store directly
	// rather than inferred from `history` being empty, which would also be
	// true for a long conversation if the window were ever misconfigured to
	// zero. Best-effort: a failed rename must not fail a turn that is
	// otherwise fine — the conversation just keeps showing as untitled.
	turns, err := s.store.CountMessages(ctx, scope, conv.ID)
	if err != nil {
		return Message{}, err
	}
	if conv.Title == "" && turns == 1 {
		if title := titleFrom(text); title != "" {
			if renamed, err := s.store.SetTitle(ctx, scope, conv.ID, title); err != nil {
				s.log.Warn("chat: auto-title failed", "conversation_id", conv.ID, "err", err)
			} else {
				conv = renamed
			}
		}
	}

	assistantID := uuid.New()
	if sink.Started != nil {
		sink.Started(toMessage(userMsg), assistantID)
	}

	retrieved, err := chatkb.Retrieve(ctx, s.kb, scope.OrgID, text)
	if err != nil {
		return Message{}, err
	}
	components := chatkb.Components(retrieved, text)
	if sink.Components != nil && len(components) > 0 {
		sink.Components(components)
	}

	req := llm.ChatRequest{
		Model:       params.Model.Model,
		Temperature: params.Temperature,
		MaxTokens:   params.MaxTokens,
		Messages:    buildMessages(buildSystemPrompt(retrieved, s.renderOptions), history, text),
	}
	answer, err := s.complete(ctx, params.Model, req, sink.Delta)
	if err != nil {
		return Message{}, err
	}
	if strings.TrimSpace(answer.Text) == "" {
		return Message{}, fmt.Errorf("chat: the model returned an empty answer")
	}

	meta := metadata{
		Components:       components,
		Provider:         params.Model.Provider,
		Model:            params.Model.Model,
		PromptTokens:     answer.PromptTokens,
		CompletionTokens: answer.CompletionTokens,
		FinishReason:     answer.FinishReason,
		KBRecords:        len(retrieved.Real.Records) + len(retrieved.Draft.Records),
		KBPendingChanges: len(retrieved.Differences()),
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return Message{}, fmt.Errorf("chat: encode message metadata: %w", err)
	}

	// Persistence deliberately does NOT inherit the request context: by the
	// time an answer is complete the operator may already have navigated
	// away, and losing a finished answer to a cancelled HTTP request would
	// be a bug, not a saving.
	saved, err := s.store.AppendMessage(context.WithoutCancel(ctx), scope, conv.ID, chatstore.AppendInput{
		ID: assistantID, Role: chatstore.RoleAssistant, Content: answer.Text, Metadata: raw,
	})
	if err != nil {
		return Message{}, err
	}
	out := toMessage(saved)
	if sink.Done != nil {
		sink.Done(out)
	}
	return out, nil
}

// complete runs the model call, streaming when the resolved provider client
// supports it and falling back to a single Complete when it does not — a
// provider adapter that cannot stream still answers, just all at once.
func (s *Service) complete(ctx context.Context, ref llm.ModelRef, req llm.ChatRequest, onDelta func(string)) (llm.ChatResponse, error) {
	client, err := s.llms.Client(ref)
	if err != nil {
		return llm.ChatResponse{}, err
	}
	if streamer, ok := client.(llm.StreamClient); ok {
		return streamer.Stream(ctx, req, func(delta string) {
			if onDelta != nil {
				onDelta(delta)
			}
		})
	}
	resp, err := client.Complete(ctx, req)
	if err == nil && onDelta != nil && resp.Text != "" {
		onDelta(resp.Text)
	}
	return resp, err
}

// buildMessages assembles the request exactly as spec §4 describes it:
// system prompt (with the KB context already inside it), then the last N
// turns, then the current question.
//
// Only user and assistant turns from history are replayed. A stored system
// turn is conversation bookkeeping, not dialogue, and re-sending one would
// hand the model a second, older set of instructions competing with the
// current one.
func buildMessages(system string, history []chatstore.Message, current string) []llm.Message {
	out := make([]llm.Message, 0, len(history)+2)
	out = append(out, llm.Message{Role: "system", Content: system})
	for _, m := range history {
		if m.Role != chatstore.RoleUser && m.Role != chatstore.RoleAssistant {
			continue
		}
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		out = append(out, llm.Message{Role: m.Role, Content: m.Content})
	}
	return append(out, llm.Message{Role: "user", Content: current})
}

// resolveParams reads the current model configuration, filling in defaults
// for anything a caller left unset.
func (s *Service) resolveParams() Params {
	var p Params
	if s.params != nil {
		p = s.params()
	}
	if p.HistoryWindow <= 0 {
		p.HistoryWindow = DefaultHistoryWindow
	}
	return p
}
