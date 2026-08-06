// Package tgingest is the transport-neutral core of turning one Telegram
// update into inbox state: normalize, upsert contact/chat/message, broadcast,
// enqueue follow-ups. It is the shared logic behind both the webhook ingress
// (internal/httpapi) and the long-poll ingress (internal/tgpoller) — ported
// out of the webhook handler unchanged in substance, with gin removed.
//
// Processor has no opinion on HOW it was reached: Process returns (Outcome,
// error) and nothing more. Each caller maps that onto its own transport
// semantics — the webhook handler onto an HTTP status, the poller onto
// whether to advance its offset — rather than tgingest knowing about either.
package tgingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// defaultPublishTimeout mirrors internal/httpapi's own publishTimeout: every
// enqueue is bounded so a full queue becomes an error the caller can act on
// (the webhook handler answers 500 so Telegram redelivers; the poller does
// not advance its offset) rather than a hung call.
const defaultPublishTimeout = 2 * time.Second

// Ingestor is the store surface Process needs — exactly the shape
// *store.Store already has, so it satisfies this interface with zero
// adapter code.
type Ingestor interface {
	IngestTelegramInbound(ctx context.Context, in store.TgInbound) (store.TgInboundResult, error)
	TouchTelegramAccount(ctx context.Context, id uuid.UUID) error
	HasDraftForTrigger(ctx context.Context, triggerMessageID uuid.UUID) (bool, error)
	MessageByID(ctx context.Context, id uuid.UUID) (store.Message, error)
	ChatByID(ctx context.Context, id uuid.UUID) (store.Chat, error)
}

// Publisher is the enqueue surface Process needs — satisfied by any
// queue.Queue implementation (queue.InMem in production) with zero adapter
// code.
type Publisher interface {
	Publish(ctx context.Context, m queue.Message) error
}

// Broadcaster is the realtime surface Process needs — satisfied by
// *realtime.Hub with zero adapter code.
type Broadcaster interface {
	Broadcast(name string, data any)
}

// Automation is the narrow surface Process needs from
// internal/automation.Scheduler to hand off a fresh (or redelivered but
// never-drafted) inbound customer message — satisfied with zero adapter
// code.
type Automation interface {
	OnInboundMessage(ctx context.Context, chatID, accountID uuid.UUID, channel string, lastMessageAt time.Time) error
}

// Deps is Processor's construction input.
type Deps struct {
	Store Ingestor
	Queue Publisher
	Hub   Broadcaster
	// Automation replaces a direct KindAIDraft enqueue with an
	// arm/reset-the-debounce-deadline call, so Telegram traffic gets the
	// exact same mode gating and debounce as WhatsApp. Optional: nil (as in
	// most of this package's own tests) falls back to the pre-automation
	// behavior of enqueueing an AI draft immediately.
	Automation Automation
	Log        *slog.Logger
	// PublishTimeout bounds every enqueue; <= 0 defaults to 2s.
	PublishTimeout time.Duration
}

// Processor is the shared ingest core. See the package doc.
type Processor struct {
	store          Ingestor
	queue          Publisher
	hub            Broadcaster
	automation     Automation
	log            *slog.Logger
	publishTimeout time.Duration
}

// New builds a Processor.
func New(d Deps) *Processor {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	timeout := d.PublishTimeout
	if timeout <= 0 {
		timeout = defaultPublishTimeout
	}
	return &Processor{store: d.Store, queue: d.Queue, hub: d.Hub, automation: d.Automation, log: log, publishTimeout: timeout}
}

// Outcome reports what Process did with one update.
type Outcome struct {
	// Status is "ignored" | "duplicate" | "stored".
	Status string
	// Reason is Update.Ignorable's reason string, set only when Status is
	// "ignored".
	Reason string
	// Result is IngestTelegramInbound's result, set for "duplicate"/"stored".
	Result store.TgInboundResult
}

// ErrIngest wraps a failure to durably store the update — the persistence
// write itself failed. ErrEnqueue wraps a failure to publish a downstream
// task: the update WAS stored, but a follow-up queue publish was refused.
// Both mean the same thing to a caller: this update is NOT durably handled
// — a webhook caller must answer without a 2xx so Telegram redelivers; a
// poller caller must NOT advance its offset, so the next GetUpdates call
// sees this update again.
var (
	ErrIngest  = errors.New("tgingest: ingest failed")
	ErrEnqueue = errors.New("tgingest: enqueue failed")
)

// Process turns one Telegram update into inbox state: ignorable check,
// normalize, ingest (upsert contact/chat/message), then branch on whether
// this is a genuinely new message or a redelivery of one already stored.
func (p *Processor) Process(ctx context.Context, acct store.TelegramAccount, upd *telegram.Update, raw []byte) (Outcome, error) {
	if reason, ignorable := upd.Ignorable(); ignorable {
		return Outcome{Status: "ignored", Reason: reason}, nil
	}

	msg := upd.Message
	in := store.TgInbound{
		AccountID: acct.ID,
		UpdateID:  upd.UpdateID,

		TelegramChatID: msg.Chat.ID,
		ChatType:       msg.Chat.Type,

		TelegramUserID: msg.From.ID,
		Username:       msg.From.Username,
		FirstName:      msg.From.FirstName,
		LastName:       msg.From.LastName,
		DisplayName:    msg.From.DisplayName(),

		TelegramMessageID: msg.MessageID,
		MessageKind:       MessageKind(msg),
		// A caption IS the message body: a photo with "сколько стоит?" under
		// it must reach the AI as that question, not as an empty media bubble.
		Body:      firstNonEmpty(msg.Text, msg.Caption),
		Preview:   Preview(msg),
		Raw:       raw,
		MessageTS: msg.Timestamp(),
	}
	if f := msg.Attachment(); f != nil {
		in.Media = &store.TgMedia{
			FileID: f.FileID, FileUniqueID: f.FileUniqueID, MediaType: f.Kind,
			Mimetype: f.Mimetype, FileName: f.FileName, Size: f.Size,
		}
	}

	res, err := p.store.IngestTelegramInbound(ctx, in)
	if err != nil {
		p.log.Error("tgingest: ingest failed", "account_id", acct.ID, "update_id", upd.UpdateID, "err", err)
		return Outcome{}, fmt.Errorf("%w: %v", ErrIngest, err)
	}
	// Best-effort: a failed activity stamp never invalidates an otherwise
	// successful ingest.
	if err := p.store.TouchTelegramAccount(ctx, acct.ID); err != nil {
		p.log.Warn("tgingest: touch account failed", "account_id", acct.ID, "err", err)
	}

	if !res.MessageInserted {
		// A redelivery. The row is already there, so the ingest is settled —
		// but the FOLLOW-UP may not be: the first attempt could have died
		// between the commit and the enqueue. If no draft exists for this
		// trigger, publish again. WriteDraftSet's supersede semantics make a
		// rare double generation harmless.
		p.emitMessage(ctx, "message.updated", res.MessageID)
		if err := p.reenqueueMissingDraft(ctx, res, acct.ID); err != nil {
			return Outcome{Status: "duplicate", Result: res}, fmt.Errorf("%w: %v", ErrEnqueue, err)
		}
		return Outcome{Status: "duplicate", Result: res}, nil
	}

	p.emitMessage(ctx, "message.created", res.MessageID)

	// Attachment bytes are fetched off the request path. A lost enqueue is
	// not fatal — the media row stays 'pending' and the sweeper picks it up
	// — so unlike the draft below this one never fails the whole update.
	if in.Media != nil {
		p.publishOrLog(ctx, queue.Message{Kind: queue.KindMediaDownload, Payload: worker.MediaDownloadTask{
			MessageID: res.MessageID, Channel: messaging.ChannelTelegram, AccountID: acct.ID,
			FileID: in.Media.FileID, BlobID: worker.TelegramBlobID(res.MessageID),
		}})
	}
	if chat, cerr := p.store.ChatByID(ctx, res.ChatID); cerr == nil {
		name := "chat.updated"
		if res.ChatCreated {
			name = "chat.created"
		}
		p.hub.Broadcast(name, dto.MapChat(chat))
	}

	// Handing this off to automation is part of the durable unit as far as
	// the caller is concerned: a failure here means this update has NOT
	// been fully handled (an armed debounce deadline, or a fallback direct
	// enqueue, is exactly as durable/required as the old unconditional
	// enqueue was).
	if err := p.armAutomation(ctx, res.ChatID, acct.ID, in.MessageTS); err != nil {
		p.log.Error("tgingest: automation arm failed; not acking", "chat_id", res.ChatID, "err", err)
		return Outcome{Status: "stored", Result: res}, fmt.Errorf("%w: %v", ErrEnqueue, err)
	}
	return Outcome{Status: "stored", Result: res}, nil
}

// armAutomation hands a genuinely new (or never-drafted-redelivered)
// inbound customer message to p.automation, replacing a direct KindAIDraft
// enqueue. p.automation is nil only in tests that don't wire one up, in
// which case this falls back to that original immediate-enqueue behavior.
func (p *Processor) armAutomation(ctx context.Context, chatID, accountID uuid.UUID, lastMessageAt time.Time) error {
	if p.automation == nil {
		return p.publish(ctx, queue.Message{Kind: queue.KindAIDraft, Payload: worker.AIDraftTask{ChatID: chatID}})
	}
	return p.automation.OnInboundMessage(ctx, chatID, accountID, string(messaging.ChannelTelegram), lastMessageAt)
}

// reenqueueMissingDraft hands off a redelivered inbound message that never
// produced a draft the first time around — the automation-arm equivalent of
// the original direct re-enqueue. There is no original message timestamp
// available here (only TgInboundResult, not the full TgInbound); using the
// current time as "last message" is a reasonable choice for this rare
// crash-recovery path.
func (p *Processor) reenqueueMissingDraft(ctx context.Context, res store.TgInboundResult, accountID uuid.UUID) error {
	has, err := p.store.HasDraftForTrigger(ctx, res.MessageID)
	if err != nil {
		// The message itself is durably stored; a lookup failure here is
		// tolerated rather than failing the whole update.
		p.log.Warn("tgingest: duplicate: draft lookup failed", "message_id", res.MessageID, "err", err)
		return nil
	}
	if has {
		return nil
	}
	if err := p.armAutomation(ctx, res.ChatID, accountID, time.Now()); err != nil {
		p.log.Error("tgingest: duplicate: draft enqueue failed", "chat_id", res.ChatID, "err", err)
		return err
	}
	p.log.Info("tgingest: duplicate: re-enqueued the missing draft", "chat_id", res.ChatID)
	return nil
}

func (p *Processor) emitMessage(ctx context.Context, name string, id uuid.UUID) {
	msg, err := p.store.MessageByID(ctx, id)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			p.log.Error("tgingest: emit message failed", "err", err)
		}
		return
	}
	p.hub.Broadcast(name, dto.MapMessage(msg))
}

// publish enqueues with a bounded deadline (see Deps.PublishTimeout).
func (p *Processor) publish(parent context.Context, m queue.Message) error {
	pctx, cancel := context.WithTimeout(parent, p.publishTimeout)
	defer cancel()
	return p.queue.Publish(pctx, m)
}

// publishOrLog is the fire-and-forget form for paths that have already
// produced a durable row: a lost enqueue degrades a background nicety (a
// media download), it never invalidates the ingest already committed.
func (p *Processor) publishOrLog(parent context.Context, m queue.Message) {
	if err := p.publish(parent, m); err != nil {
		p.log.Error("tgingest: enqueue failed", "kind", m.Kind, "err", err)
	}
}
