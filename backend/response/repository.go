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
	// Customer is the CRM context for whoever is on the other end, or nil when
	// the conversation has no customer. Implementations that do not know about
	// the CRM layer simply leave it nil, and the prompt is unchanged.
	Customer *aiprompt.CustomerContext
}

// ConversationRepository loads the response-relevant context for a
// conversation. Implementations own the schema (today's wa_* tables) — this
// interface is what keeps the response engine multichannel-ready without
// knowing it.
type ConversationRepository interface {
	LoadForResponse(ctx context.Context, conversationID string) (ConversationContext, error)
}

// DraftToPersist is one generated draft ready to become a conversation's
// single suggested option. Channel is carried so the persisted row records
// which transport the conversation lives on — the suggestion store is
// channel-neutral and has no other way to know.
type DraftToPersist struct {
	ConversationID   string
	Channel          messaging.Channel
	TriggerMessageID string
	Text             string
	ReplyLanguage    string
	Confidence       *float64
	Escalate         bool
	EscalationReason string
	// KBGap is the optional v7 structured escalation diagnostic
	// (0018_kb_gap_telemetry) — nil for a plain (pre-v7, or undiagnosed)
	// escalation. A DraftRepository implementation persists it as an
	// ai_kb_gap_events row alongside the draft, in the same transaction,
	// whenever Escalate is true; see internal/responsestore.DraftRepo and
	// internal/automation's versionGatedDrafts.
	KBGap *aiprompt.KBGapDiagnostic
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
