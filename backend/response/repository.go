package response

import (
	"context"
	"time"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// KnowledgeBaseRepository loads an organization's approved knowledge base.
// Implementations own the SQL row shapes; only aiprompt.KB crosses this
// interface.
type KnowledgeBaseRepository interface {
	Load(ctx context.Context, organizationID string) (*aiprompt.KB, error)
}

// ConversationContext is everything the engine needs about one conversation:
// its resolved organization (walked conversation -> account -> organization,
// never a global org id), the account's channel, recent history, and the
// current inbound message.
type ConversationContext struct {
	OrganizationID   string
	AccountID        string
	Channel          messaging.Channel
	History          []aiprompt.HistoryTurn
	CurrentMessage   string
	TriggerMessageID string
}

// ConversationRepository loads the response-relevant context for a
// conversation. Implementations own the schema (today's wa_* tables) — this
// interface is what keeps the response engine multichannel-ready without
// knowing it.
type ConversationRepository interface {
	LoadForResponse(ctx context.Context, conversationID string) (ConversationContext, error)
}

// DraftToPersist is one generated draft ready to become a conversation's
// single suggested option.
type DraftToPersist struct {
	ConversationID   string
	TriggerMessageID string
	Text             string
	ReplyLanguage    string
	Confidence       *float64
	Escalate         bool
	EscalationReason string
}

// PersistedDraft is a DraftToPersist after being written, with its assigned id.
type PersistedDraft struct {
	ID               string
	ConversationID   string
	TriggerMessageID string
	Text             string
	ReplyLanguage    string
	Confidence       *float64
	Escalate         bool
	EscalationReason string
	CreatedAt        time.Time
}

// DraftRepository persists a generated draft, superseding any prior suggested
// option for the same conversation (the existing ai_drafts supersede
// semantics, unchanged).
type DraftRepository interface {
	ReplaceSuggested(ctx context.Context, draft DraftToPersist) ([]PersistedDraft, error)
}
