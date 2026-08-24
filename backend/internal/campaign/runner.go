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
// Every exit path — including a resolveChat failure — ends in exactly one
// FinalizeAttempt call, since ClaimNextRecipient already committed this
// attempt's ledger row and flipped the recipient to 'sending' before this
// method ever ran; leaving it there is what ReconcileStuckSending exists to
// clean up, so this method must never return without finalizing.
func (r *Runner) send(ctx context.Context, claim store.Claim) {
	chatID, created, err := r.resolveChat(ctx, claim)
	if err != nil {
		r.finalize(ctx, claim, err, uuid.NullUUID{}, uuid.NullUUID{})
		return
	}

	vars := make(map[string]string, len(claim.Attributes)+1)
	for k, v := range claim.Attributes {
		vars[k] = v
	}
	vars["name"] = claim.Name
	text := purecampaign.Render(claim.MessageBody, vars)

	msgID, err := r.Store.InsertCampaignOutbound(ctx, claim.Channel, chatID, claim.AccountID, text, previewText(text))
	if err != nil {
		r.Log.Error("campaign: insert outbound failed", "recipient_id", claim.RecipientID, "err", err)
		r.finalize(ctx, claim, err, uuid.NullUUID{UUID: chatID, Valid: true}, uuid.NullUUID{})
		return
	}

	// Fetched AFTER InsertCampaignOutbound so the broadcast chat row (and the
	// ExternalConversationRef Deliver routes the send through) reflect this
	// message's own last_message_at/preview bump, not a stale pre-send state.
	chat, err := r.Store.ChatByID(ctx, chatID)
	if err != nil {
		r.Log.Error("campaign: load chat after send failed", "chat_id", chatID, "err", err)
		r.finalize(ctx, claim, err, uuid.NullUUID{UUID: chatID, Valid: true}, uuid.NullUUID{UUID: msgID, Valid: true})
		return
	}
	chatEvent := "chat.updated"
	if created {
		chatEvent = "chat.created"
	}
	r.Hub.Broadcast(chatEvent, dto.MapChat(chat))
	if msg, merr := r.Store.MessageByID(ctx, msgID); merr == nil {
		r.Hub.Broadcast("message.created", dto.MapMessage(msg))
	}

	_, sendErr := outbound.Deliver(ctx, outbound.Deps{
		Store: r.Store, Blob: r.Blob, Hub: r.Hub, Senders: r.Senders, Log: r.Log,
	}, outbound.Task{
		MessageID: msgID, AccountID: claim.AccountID, Channel: messaging.Channel(claim.Channel),
		Destination: chat.ExternalConversationRef, Text: text,
	})
	r.finalize(ctx, claim, sendErr, uuid.NullUUID{UUID: chatID, Valid: true}, uuid.NullUUID{UUID: msgID, Valid: true})
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

// finalize records the outcome of one send attempt: nil sendErr -> sent;
// messaging.ErrOutsideServiceWindow or errNoExistingChat -> permanently
// failed (neither is a transport hiccup a retry could fix — the provider's
// own service-window rule needs the customer to message in again, and a
// still-missing chat needs preview-time reachability re-checked, not a
// resend); anything else -> transient, stepped through
// backend/campaign.NextRetry's fixed backoff ladder until it is exhausted,
// at which point it too becomes permanently failed.
func (r *Runner) finalize(ctx context.Context, claim store.Claim, sendErr error, chatID, messageID uuid.NullUUID) {
	p := store.FinalizeAttemptParams{LogID: claim.LogID, RecipientID: claim.RecipientID, ChatID: chatID, MessageID: messageID}
	switch {
	case sendErr == nil:
		p.NewStatus = purecampaign.RecipientSent
	case errors.Is(sendErr, messaging.ErrOutsideServiceWindow), errors.Is(sendErr, errNoExistingChat):
		p.NewStatus = purecampaign.RecipientFailed
		p.FailureReason = sendErr.Error()
	default:
		if wait, ok := purecampaign.NextRetry(claim.Attempts); ok {
			next := time.Now().Add(wait)
			p.NewStatus = purecampaign.RecipientPending
			p.FailureReason = sendErr.Error()
			p.NextAttemptAt = &next
		} else {
			p.NewStatus = purecampaign.RecipientFailed
			p.FailureReason = "max retries exceeded: " + sendErr.Error()
		}
	}
	if err := r.Store.FinalizeAttempt(ctx, p); err != nil {
		r.Log.Error("campaign: finalize attempt failed", "recipient_id", claim.RecipientID, "err", err)
		return
	}
	r.Hub.Broadcast("campaign.recipient_updated", dto.CampaignRecipientEvent{
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
	if _, err := r.Store.SetCampaignStatus(ctx, campaignID, purecampaign.StatusCompleted, uuid.NullUUID{}, "completed", nil); err != nil {
		if !errors.Is(err, store.ErrInvalidTransition) {
			r.Log.Error("campaign: auto-complete failed", "campaign_id", campaignID, "err", err)
		}
		return
	}
	r.Hub.Broadcast("campaign.status_changed", dto.CampaignStatusEvent{CampaignID: campaignID.String(), Status: string(purecampaign.StatusCompleted)})
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
