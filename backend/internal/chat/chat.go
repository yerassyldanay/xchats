// Package chat is the Knowledge Base chat assistant's service layer — the
// thing that sits between the HTTP edge and the three collaborators it
// orchestrates:
//
//	chat.Service  ->  chatkb.Service   (which KB facts this question needs)
//	              ->  chatstore.Store  (conversation history, persistence)
//	              ->  llm.Registry     (the model, whoever the provider is)
//
// Each of those is an interface or a repository package this one does not
// reach around: the service never builds a provider request body, never
// writes SQL, and never reads the KB tables. That is the whole design
// constraint (spec §12) — it is what makes replacing lexical KB retrieval
// with vector search, or OpenRouter with another provider, a change in one
// package rather than a redesign.
//
// What this package owns is the shape of a turn: persist the operator's
// message, retrieve both KB states, build the components the UI will draw,
// assemble a prompt out of the system instructions + KB context + the last N
// turns, stream the answer, and persist it with its components attached.
package chat

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/chatkb"
	"github.com/yerassyldanay/xchats/backend/internal/chatstore"
	"github.com/yerassyldanay/xchats/backend/llm"
)

// DefaultHistoryWindow is how many past turns a request carries when no
// window is configured — five exchanges, which covers the follow-up
// questions this UI is actually for ("and the draft price?") without
// dragging an hour-old topic into an unrelated question. Spec §3: the whole
// transcript is persisted regardless; this only bounds what the model sees.
const DefaultHistoryWindow = 10

// maxTitleChars bounds an auto-generated conversation title — the sidebar is
// narrow, and a title is a label, not a summary.
const maxTitleChars = 60

// Params is the model configuration one request runs under. The Service
// re-reads it per request (see Service.Params) so a Settings change takes
// effect on the very next message, matching how the response engine already
// treats its own LLM settings.
type Params struct {
	Model         llm.ModelRef
	Temperature   float64
	MaxTokens     int
	HistoryWindow int
}

// Message is one turn as the API returns it. Components is the decoded
// structured payload from the stored metadata, hoisted to the top level so
// the frontend renders a reloaded conversation exactly like a live one
// rather than reaching into a metadata blob.
type Message struct {
	ID         uuid.UUID          `json:"id"`
	Role       string             `json:"role"`
	Content    string             `json:"content"`
	Components []chatkb.Component `json:"components"`
	Metadata   json.RawMessage    `json:"metadata"`
	CreatedAt  time.Time          `json:"created_at"`
}

// metadata is what an assistant turn stores in chat_messages.metadata:
// the components the UI draws, plus enough provenance to answer "which model
// said this, and what did it cost" later without a separate table.
type metadata struct {
	Components       []chatkb.Component `json:"components,omitempty"`
	Provider         string             `json:"provider,omitempty"`
	Model            string             `json:"model,omitempty"`
	PromptTokens     int                `json:"prompt_tokens,omitempty"`
	CompletionTokens int                `json:"completion_tokens,omitempty"`
	FinishReason     string             `json:"finish_reason,omitempty"`
	// KBRecords/KBPendingChanges record how much KB context the answer was
	// grounded in — the first thing worth checking when an answer looks
	// wrong ("did it even see the product?").
	KBRecords        int `json:"kb_records,omitempty"`
	KBPendingChanges int `json:"kb_pending_changes,omitempty"`
}

// toMessage decodes a stored row into the API shape, lifting components out
// of metadata. A metadata blob that fails to parse is not an error: the
// prose is the message, and components are an enhancement on top of it.
func toMessage(m chatstore.Message) Message {
	out := Message{
		ID: m.ID, Role: m.Role, Content: m.Content,
		Components: []chatkb.Component{}, Metadata: m.Metadata, CreatedAt: m.CreatedAt,
	}
	var meta metadata
	if len(m.Metadata) > 0 {
		if err := json.Unmarshal(m.Metadata, &meta); err == nil && len(meta.Components) > 0 {
			out.Components = meta.Components
		}
	}
	return out
}

// toMessages maps a transcript into the API shape.
func toMessages(msgs []chatstore.Message) []Message {
	out := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toMessage(m))
	}
	return out
}

// titleFrom derives a conversation title from its first message: one line,
// clipped at a word boundary near maxTitleChars. Returns "" for a message
// with no usable text, leaving the conversation untitled rather than named
// something meaningless.
func titleFrom(text string) string {
	line := strings.TrimSpace(text)
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	line = strings.Join(strings.Fields(line), " ")
	if line == "" {
		return ""
	}
	runes := []rune(line)
	if len(runes) <= maxTitleChars {
		return line
	}
	clipped := string(runes[:maxTitleChars])
	if i := strings.LastIndex(clipped, " "); i > maxTitleChars/2 {
		clipped = clipped[:i]
	}
	return strings.TrimSpace(clipped) + "…"
}
