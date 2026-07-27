package responsestore

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/response"
)

// DraftRepo implements response.DraftRepository over the existing ai_drafts
// lifecycle (store.WriteDraftSet) — supersede semantics unchanged.
type DraftRepo struct {
	Store *store.Store
}

// ReplaceSuggested persists draft as the conversation's single suggested
// option, superseding any prior suggested option (store.WriteDraftSet's
// existing semantics).
func (r *DraftRepo) ReplaceSuggested(ctx context.Context, draft response.DraftToPersist) ([]response.PersistedDraft, error) {
	chatID, err := uuid.Parse(draft.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("responsestore: invalid conversation id %q: %w", draft.ConversationID, err)
	}
	var trigger uuid.NullUUID
	if draft.TriggerMessageID != "" {
		tid, err := uuid.Parse(draft.TriggerMessageID)
		if err != nil {
			return nil, fmt.Errorf("responsestore: invalid trigger message id %q: %w", draft.TriggerMessageID, err)
		}
		trigger = uuid.NullUUID{UUID: tid, Valid: true}
	}

	written, err := r.Store.WriteDraftSet(ctx, chatID, trigger, []store.DraftOption{{
		Ordinal:          1,
		Text:             draft.Text,
		ReplyLanguage:    draft.ReplyLanguage,
		Confidence:       draft.Confidence,
		Escalate:         draft.Escalate,
		EscalationReason: draft.EscalationReason,
	}})
	if err != nil {
		return nil, fmt.Errorf("responsestore: write draft: %w", err)
	}

	out := make([]response.PersistedDraft, 0, len(written))
	for _, d := range written {
		out = append(out, response.PersistedDraft{
			ID:               d.ID.String(),
			ConversationID:   d.ChatID.String(),
			TriggerMessageID: nullUUIDString(d.TriggerMessageID),
			Text:             d.DraftText,
			ReplyLanguage:    d.ReplyLanguage,
			Confidence:       d.Confidence,
			Escalate:         d.Escalate,
			EscalationReason: d.EscalationReason,
			CreatedAt:        d.CreatedAt,
		})
	}
	return out, nil
}

func nullUUIDString(id uuid.NullUUID) string {
	if !id.Valid {
		return ""
	}
	return id.UUID.String()
}
