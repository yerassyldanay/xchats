package response

import (
	"context"
	"fmt"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// HoldingText is the canned fallback the service substitutes only when
// Generate itself fails (a KB-load, provider, or contract-validation error) —
// never in place of a model's own eval-validated escalation text, which
// Engine.Generate already returns verbatim in GenerateResult.FinalText.
const HoldingText = "Сейчас у меня нет этой информации — передаю ваш вопрос менеджеру и вернусь с точным ответом."

// Service is the channel-neutral entry point every channel adapter calls
// through: given a channel and a conversation id, it loads context and the
// organization's knowledge base, generates a grounded reply through Engine,
// and persists exactly one suggested draft. It never sends a message or
// auto-approves a draft — that stays the existing review/approval flow's job.
type Service struct {
	Conversations ConversationRepository
	KnowledgeBase KnowledgeBaseRepository
	Drafts        DraftRepository
	Engine        *Engine
}

// RespondOptions customizes one Respond call. ModelOverride and KBOverride are
// settable only by the authenticated simulator handler — production traffic
// on every real channel never sets either.
type RespondOptions struct {
	ModelOverride *llm.ModelRef
	// KBOverride, when set, is used verbatim instead of loading the live KB
	// through KnowledgeBase.Load — the Simulator's "test against staged
	// draft" mode (KB-02), fed a kbstore Draft() view translated by
	// responsestore.BuildKBFromDraftView. nil (the default, and every
	// non-simulator caller) means the ordinary live load below runs exactly
	// as before.
	KBOverride *aiprompt.KB
}

// Respond loads the conversation's context, verifies the requested channel
// matches what the conversation is actually stored under, loads the
// organization's knowledge base, generates a reply, and persists exactly one
// suggested draft tied to the latest inbound message. A hard failure in the
// KB load or generation becomes an explicit holding/escalation draft instead
// of propagating — Respond only returns an error when it cannot even resolve
// the conversation or persist that draft.
func (s *Service) Respond(ctx context.Context, channel messaging.Channel, conversationID string, opts RespondOptions) ([]PersistedDraft, error) {
	convCtx, err := s.Conversations.LoadForResponse(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("response: load conversation: %w", err)
	}
	if convCtx.Channel != channel {
		return nil, fmt.Errorf("response: conversation %s belongs to channel %q, not %q", conversationID, convCtx.Channel, channel)
	}

	draft := s.generate(ctx, conversationID, convCtx, opts)
	persisted, err := s.Drafts.ReplaceSuggested(ctx, draft)
	if err != nil {
		return nil, fmt.Errorf("response: persist draft: %w", err)
	}
	return persisted, nil
}

func (s *Service) generate(ctx context.Context, conversationID string, convCtx ConversationContext, opts RespondOptions) DraftToPersist {
	draft := DraftToPersist{
		ConversationID:   conversationID,
		Channel:          convCtx.Channel,
		TriggerMessageID: convCtx.TriggerMessageID,
	}

	kb := opts.KBOverride
	if kb == nil {
		var err error
		kb, err = s.KnowledgeBase.Load(ctx, convCtx.OrganizationID)
		if err != nil {
			return holdingDraft(draft, err)
		}
	}

	result, err := s.Engine.Generate(ctx, GenerateRequest{
		OrganizationID: convCtx.OrganizationID,
		ConversationID: conversationID,
		Channel:        convCtx.Channel,
		History:        convCtx.History,
		IncomingText:   convCtx.CurrentMessage,
		KB:             kb,
		Customer:       convCtx.Customer,
		ModelOverride:  opts.ModelOverride,
	})
	if err != nil {
		return holdingDraft(draft, err)
	}

	draft.Text = result.FinalText
	draft.ReplyLanguage = result.ReplyLanguage
	draft.Escalate = result.Escalate
	draft.EscalationReason = result.EscalationReason
	draft.KBGap = result.KBGap
	if result.Confidence != 0 {
		c := result.Confidence
		draft.Confidence = &c
	}
	return draft
}

// holdingDraft is the explicit hard-failure fallback: the canned holding
// text, always escalated, with cause recorded as the (internal-only)
// escalation reason for operator visibility. KBGap is stamped
// KBGapReasonEngineError/KBGapSourceEngine — a hard Generate error (a KB
// load failure, a provider error, a response that still fails contract
// validation) never reached the model contract at all, so it must never be
// attributed to a fabricated KB entity or reason the model supposedly gave.
func holdingDraft(draft DraftToPersist, cause error) DraftToPersist {
	draft.Text = HoldingText
	draft.ReplyLanguage = "ru"
	draft.Escalate = true
	draft.EscalationReason = "engine_error: " + cause.Error()
	draft.KBGap = &aiprompt.KBGapDiagnostic{
		ReasonCode: aiprompt.KBGapReasonEngineError,
		Source:     aiprompt.KBGapSourceEngine,
	}
	return draft
}
