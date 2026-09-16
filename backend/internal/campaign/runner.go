// Package campaign is the restart-safe orchestration around
// backend/campaign's pure decisions and internal/store/campaigns.go's
// at-most-once claim ledger: a Scheduler that discovers which accounts have
// running campaigns and drains their eligible recipients, and a Runner that
// executes one claimed send — render, resolve the destination chat, deliver
// through the exact same internal/outbound.Deliver every manual/AI/
// automation send uses, and finalize the outcome. Mirrors
// internal/automation's own package shape closely; see that package's file
// header for the "the row is the queue" philosophy this follows for
// everything except the disconnect watch (see Config.DisconnectAfter).
package campaign

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/outbound"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// Broadcaster is the realtime surface Runner needs — satisfied by
// *realtime.Hub with zero adapter code, mirroring internal/automation's own
// Broadcaster and internal/outbound's own (structurally identical, but
// distinct to avoid this package importing internal/realtime just to name
// its concrete type).
//
// SSE payloads broadcast from this package carry only ids, enum status
// strings, and counters — NEVER a recipient's raw_input/normalized_identity/
// name. internal/realtime.Hub.Broadcast is process-global, not scoped to an
// organization: any connected client on ANY org receives every event, so a
// recipient's phone number (or Telegram chat id) must never ride one of
// these events. See plan/DECISIONS.md.
type Broadcaster interface {
	Broadcast(name string, data any)
	BroadcastScoped(orgID uuid.UUID, name string, data any)
}

// errNoExistingChat is resolveChat's error for a warm-only channel recipient
// with no existing conversation — a defensive re-check, not the expected
// path (see backend/campaign.ColdSendCapable's own doc comment: this is
// supposed to already be filtered out at preview time).
var errNoExistingChat = errors.New("campaign: no existing conversation found for this recipient")

// Runner executes one claimed send end to end.
type Runner struct {
	Store   *store.Store
	Blob    blob.Store
	Senders *messaging.SenderRegistry
	Hub     Broadcaster
	Log     *slog.Logger
}

// HandleAccount attempts exactly one claim-and-send cycle for accountID.
// claimed reports whether a recipient was actually claimed (regardless of
// the send's own outcome) — the Scheduler uses this to decide whether to
// immediately try the same account again (there may be more headroom right
// now) or wait for the next tick.
func (r *Runner) HandleAccount(ctx context.Context, accountID uuid.UUID) (claimed bool) {
	claim, ok, err := r.Store.ClaimNextRecipient(ctx, accountID, time.Now())
	if err != nil {
		r.Log.Error("campaign: claim next recipient failed", "account_id", accountID, "err", err)
		return false
	}
	if !ok {
		return false
	}
	r.send(ctx, claim)
	return true
}

// send renders claim's message, resolves its destination chat, records and
// delivers the outbound message, and finalizes the recipient's outcome.
// Every exit path ends in exactly one FinalizeAttempt call, since
// ClaimNextRecipient already committed this attempt's ledger/attempt rows
// and flipped the recipient to 'sending' before this method ever ran;
// leaving it there is what ReconcileStuckSending exists to clean up, so
// this method must never return without finalizing.
//
// If claim.MessageID is already set (a PRIOR attempt for this same
// recipient already resolved a chat and created the outbound message row —
// see Claim.MessageID's own doc comment), this retry reuses that exact chat
// and message and skips resolveChat/InsertCampaignOutbound entirely: only
// delivery is re-attempted. Doing otherwise would create a second logical
// message in the conversation for what the recipient (and every diagnostic
// tool) must see as one send with several attempts.
func (r *Runner) send(ctx context.Context, claim store.Claim) {
	retry := claim.MessageID.Valid
	var chatID, msgID uuid.UUID
	var created bool

	if retry {
		msgID = claim.MessageID.UUID
		if claim.ChatID.Valid {
			chatID = claim.ChatID.UUID
		} else {
			// Defensive: InsertCampaignOutbound always has a resolved chat
			// before it ever creates a message, so this should not happen —
			// but if the two ever disagree, resolve the chat from the
			// message itself rather than guessing or re-resolving.
			msg, err := r.Store.MessageByID(ctx, msgID)
			if err != nil {
				r.finalizeLocal(ctx, claim, ErrorCodeInternalError, err, uuid.NullUUID{}, uuid.NullUUID{UUID: msgID, Valid: true})
				return
			}
			chatID = msg.ChatID
		}
	} else {
		var err error
		chatID, created, err = r.resolveChat(ctx, claim)
		if err != nil {
			r.finalize(ctx, claim, err, "", uuid.NullUUID{}, uuid.NullUUID{})
			return
		}
	}

	// name and phone are always the recipient's own real values, never a
	// CSV-supplied attribute of the same key — phone in particular is
	// never itself a named Attribute (it is the identity column every
	// recipient is parsed and deduplicated by, see backend/campaign's own
	// ParseRecipients doc comment), so it would otherwise render as
	// literally missing: Render silently drops an unmapped {{token}}
	// rather than leaving it as-is. The frontend wizard already offers
	// {{phone}} as a quick-insert chip and excludes it from its own
	// unmatched-variable warning on that same assumption — this is what
	// makes that assumption actually true at send time. Re-rendering on a
	// retry is safe: it is a pure function of claim's own fields, which
	// never change between attempts, so it reproduces the exact same text.
	vars := make(map[string]string, len(claim.Attributes)+2)
	for k, v := range claim.Attributes {
		vars[k] = v
	}
	vars["name"] = claim.Name
	vars["phone"] = claim.NormalizedIdentity
	text := purecampaign.Render(claim.MessageBody, vars)

	if !retry {
		var err error
		msgID, err = r.Store.InsertCampaignOutbound(ctx, claim.Channel, chatID, claim.AccountID, text, previewText(text))
		if err != nil {
			r.Log.Error("campaign: insert outbound failed", "recipient_id", claim.RecipientID, "err", err)
			r.finalizeLocal(ctx, claim, ErrorCodeMessageCreationFailed, err, uuid.NullUUID{UUID: chatID, Valid: true}, uuid.NullUUID{})
			return
		}
	}

	// Fetched AFTER InsertCampaignOutbound so a first attempt's broadcast
	// chat row (and the ExternalConversationRef Deliver routes the send
	// through) reflect this message's own last_message_at/preview bump, not
	// a stale pre-send state. On a retry nothing about the chat/message
	// content changed since the prior attempt, so this is just the current
	// destination lookup Deliver needs.
	chat, err := r.Store.ChatByID(ctx, chatID)
	if err != nil {
		r.Log.Error("campaign: load chat after send failed", "chat_id", chatID, "err", err)
		r.finalizeLocal(ctx, claim, ErrorCodeInternalError, err, uuid.NullUUID{UUID: chatID, Valid: true}, uuid.NullUUID{UUID: msgID, Valid: true})
		return
	}
	if !retry {
		chatEvent := "chat.updated"
		if created {
			chatEvent = "chat.created"
		}
		r.Hub.BroadcastScoped(claim.OrganizationID, chatEvent, dto.MapChatWithCampaigns(ctx, r.Store, chat))
		if msg, merr := r.Store.MessageByID(ctx, msgID); merr == nil {
			r.Hub.BroadcastScoped(claim.OrganizationID, "message.created", dto.MapMessage(msg))
		}
	}

	res, sendErr := outbound.Deliver(ctx, outbound.Deps{
		Store: r.Store, Blob: r.Blob, Hub: r.Hub, Senders: r.Senders, Log: r.Log,
	}, outbound.Task{
		MessageID: msgID, AccountID: claim.AccountID, Channel: messaging.Channel(claim.Channel),
		Destination: chat.ExternalConversationRef, Text: text,
	})
	r.finalize(ctx, claim, sendErr, res.ExternalID, uuid.NullUUID{UUID: chatID, Valid: true}, uuid.NullUUID{UUID: msgID, Valid: true})
}

// resolveChat finds (or, for a cold-send-capable channel, creates) claim's
// destination chat. A freshly created chat is flagged chat_state='campaign'
// (hidden from the default inbox listing until the recipient replies — see
// store.MarkChatCampaignOnly's own doc comment); an existing chat a
// warm-only channel reuses already has its own real state and is left
// untouched.
func (r *Runner) resolveChat(ctx context.Context, claim store.Claim) (chatID uuid.UUID, created bool, err error) {
	if purecampaign.ColdSendCapable(claim.Channel) {
		jid := config.CanonicalJID(claim.NormalizedIdentity)
		chatID, created, err = r.Store.FindOrCreateChat(ctx, claim.AccountID, jid, claim.NormalizedIdentity)
		if err != nil {
			return uuid.Nil, false, err
		}
		if created {
			if err := r.Store.MarkChatCampaignOnly(ctx, claim.Channel, chatID); err != nil {
				r.Log.Error("campaign: mark chat campaign-only failed", "chat_id", chatID, "err", err)
			}
		}
		return chatID, created, nil
	}
	chatID, ok, err := r.Store.ExistingChatForIdentity(ctx, claim.AccountID, claim.NormalizedIdentity)
	if err != nil {
		return uuid.Nil, false, err
	}
	if !ok {
		return uuid.Nil, false, errNoExistingChat
	}
	return chatID, false, nil
}

// finalize records the outcome of one send attempt using classifySendError's
// real-evidence-based classification of sendErr (nil meaning success):
//
//   - Not retryable (service-window closed, invalid/unreachable recipient,
//     no existing conversation) -> permanently failed. None of these is a
//     transport hiccup a retry could fix — the provider's own service-window
//     rule needs the customer to message in again, a still-missing chat
//     needs preview-time reachability re-checked rather than a resend, and
//     an unreachable destination stays unreachable.
//   - Ambiguous (a send timeout) -> the recipient's coarse status still
//     becomes 'failed' (existing vocabulary, unchanged API), but the
//     ATTEMPT is honestly recorded 'unknown', and — critically — NOT stepped
//     through the retry ladder at all, however many attempts remain: a
//     timeout does not prove the provider never received the message, so a
//     blind resend risks a real duplicate delivery. This mirrors
//     ReconcileStuckSending's identical choice for a crash mid-send.
//   - Otherwise retryable -> stepped through backend/campaign.NextRetry's
//     fixed backoff ladder until it is exhausted, at which point it too
//     becomes permanently failed, preserving (never discarding) the
//     original cause in FailureReason.
func (r *Runner) finalize(ctx context.Context, claim store.Claim, sendErr error, providerMessageID string, chatID, messageID uuid.NullUUID) {
	cls := classifySendError(sendErr)
	r.finalizeClassified(ctx, claim, cls, providerMessageID, chatID, messageID)
}

// finalizeLocal is finalize's counterpart for a failure this package knows
// the cause of directly (resolveChat, InsertCampaignOutbound, or the
// defensive ChatByID/MessageByID re-fetch failed) — never a ChannelSender
// result, so there is nothing to pattern-match: the call site already knows
// exactly which structured code applies.
func (r *Runner) finalizeLocal(ctx context.Context, claim store.Claim, code ErrorCode, err error, chatID, messageID uuid.NullUUID) {
	r.finalizeClassified(ctx, claim, localFailure(code, err), "", chatID, messageID)
}

func (r *Runner) finalizeClassified(ctx context.Context, claim store.Claim, cls classification, providerMessageID string, chatID, messageID uuid.NullUUID) {
	p := store.FinalizeAttemptParams{
		LogID: claim.LogID, AttemptID: claim.AttemptID, RecipientID: claim.RecipientID,
		ChatID: chatID, MessageID: messageID, ProviderMessageID: providerMessageID,
		ErrorCode: string(cls.Code),
	}
	retryable := cls.Retryable
	p.Retryable = &retryable

	switch {
	case cls.Code == "": // success
		p.NewStatus = purecampaign.RecipientSent
		p.AttemptOutcome = "provider_accepted"
		p.Event = "provider_accepted"
	case cls.Ambiguous:
		p.NewStatus = purecampaign.RecipientFailed
		p.FailureReason = cls.Detail
		p.AttemptOutcome = "unknown"
		p.Event = "recipient_outcome_unknown"
	case !cls.Retryable:
		p.NewStatus = purecampaign.RecipientFailed
		p.FailureReason = cls.Detail
		p.AttemptOutcome = "failed"
		p.Event = "provider_rejected"
	default:
		if wait, ok := purecampaign.NextRetry(claim.Attempts); ok {
			next := time.Now().Add(wait)
			p.NewStatus = purecampaign.RecipientPending
			p.FailureReason = cls.Detail
			p.NextAttemptAt = &next
			p.AttemptOutcome = "failed"
			p.Event = "retry_scheduled"
		} else {
			p.NewStatus = purecampaign.RecipientFailed
			p.FailureReason = "max retries exceeded: " + cls.Detail
			p.AttemptOutcome = "failed"
			p.Event = "retry_exhausted"
		}
	}

	if err := r.Store.FinalizeAttempt(ctx, p); err != nil {
		r.Log.Error("campaign: finalize attempt failed", "recipient_id", claim.RecipientID, "err", err)
		return
	}
	r.Hub.BroadcastScoped(claim.OrganizationID, "campaign.recipient_updated", dto.CampaignRecipientEvent{
		CampaignID: claim.CampaignID.String(), RecipientID: claim.RecipientID.String(), Status: string(p.NewStatus),
	})
	r.completeIfDone(ctx, claim.CampaignID)
}

// completeIfDone marks campaignID Completed once nothing is left pending or
// mid-send — called after every finalize (the common case) and, for the
// empty-from-the-start edge case, from the Scheduler's own periodic sweep
// (see store.RunningCampaignIDs' own doc comment). A campaign already moved
// on by a concurrent operator action (e.g. paused or cancelled between the
// counts read and this call) is left alone: ErrInvalidTransition is expected
// and silent, not an error worth logging.
func (r *Runner) completeIfDone(ctx context.Context, campaignID uuid.UUID) {
	counts, err := r.Store.CampaignRecipientCounts(ctx, campaignID)
	if err != nil {
		r.Log.Error("campaign: load recipient counts failed", "campaign_id", campaignID, "err", err)
		return
	}
	if counts["pending"] > 0 || counts["sending"] > 0 {
		return
	}
	updated, err := r.Store.SetCampaignStatus(ctx, campaignID, purecampaign.StatusCompleted, uuid.NullUUID{}, "completed", nil)
	if err != nil {
		if !errors.Is(err, store.ErrInvalidTransition) {
			r.Log.Error("campaign: auto-complete failed", "campaign_id", campaignID, "err", err)
		}
		return
	}
	r.Hub.BroadcastScoped(updated.OrganizationID, "campaign.status_changed", dto.CampaignStatusEvent{CampaignID: campaignID.String(), Status: string(purecampaign.StatusCompleted)})
}

// previewText mirrors internal/httpapi's own preview() truncation — a
// chat-list snippet, not the delivered message, so a very long rendered
// template never bloats last_message_preview.
func previewText(text string) string {
	r := []rune(text)
	if len(r) > 120 {
		return string(r[:120])
	}
	return text
}
