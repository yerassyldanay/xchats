// Package metaingest is the transport-neutral core of turning one Meta
// channel webhook event — today WhatsApp Cloud API, and from Phase 4/5
// Instagram Direct and Facebook Messenger — into inbox state: upsert
// contact/chat/message via the generic channel core (migration 0011),
// broadcast, enqueue follow-ups. It mirrors internal/tgingest's shape
// exactly (see that package's doc comment for the design this ports); the
// one addition, ApplyStatus, has no tgingest equivalent because Telegram
// bots receive no delivery-status webhook at all.
//
// Process takes a NormalizedMessage rather than a provider-specific type so
// this package never imports whatsappcloud (or, later, messengerish) —
// each channel's own webhook.go decodes the provider's wire shape and maps
// it into NormalizedMessage; Process itself is "one path for all three."
package metaingest

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
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// defaultPublishTimeout mirrors tgingest's own — every enqueue is bounded so
// a full queue becomes an error the caller can act on (the webhook handler
// answers 500 so Meta redelivers) rather than a hung call.
const defaultPublishTimeout = 2 * time.Second

// Ingestor is the store surface Process/ApplyStatus need — exactly the shape
// *store.Store already has, so it satisfies this interface with zero
// adapter code, mirroring tgingest.Ingestor.
type Ingestor interface {
	IngestChannelInbound(ctx context.Context, in store.ChannelInbound) (store.ChannelInboundResult, error)
	TouchChannelAccount(ctx context.Context, id uuid.UUID) error
	InsertChannelMediaPending(ctx context.Context, messageID uuid.UUID, m store.ChannelMediaMeta) error
	HasDraftForTrigger(ctx context.Context, triggerMessageID uuid.UUID) (bool, error)
	MessageByID(ctx context.Context, id uuid.UUID) (store.Message, error)
	ChatByID(ctx context.Context, id uuid.UUID) (store.Chat, error)
	AdvanceDeliveryState(ctx context.Context, channel string, accountID uuid.UUID, externalMessageID, newState string, newRank int) (uuid.UUID, uuid.UUID, error)
}

// Publisher is the enqueue surface Process needs — satisfied by any
// queue.Queue implementation with zero adapter code.
type Publisher interface {
	Publish(ctx context.Context, m queue.Message) error
}

// Broadcaster is the realtime surface Process needs — satisfied by
// *realtime.Hub with zero adapter code.
type Broadcaster interface {
	Broadcast(name string, data any)
}

// Automation is the narrow surface Process needs from
// internal/automation.Scheduler — identical to tgingest.Automation, so the
// same *automation.Scheduler instance backs every channel.
type Automation interface {
	OnInboundMessage(ctx context.Context, chatID, accountID uuid.UUID, channel string, lastMessageAt time.Time) error
}

// Deps is Processor's construction input.
type Deps struct {
	Store Ingestor
	Queue Publisher
	Hub   Broadcaster
	// Automation replaces a direct KindAIDraft enqueue with an
	// arm/reset-the-debounce-deadline call — see tgingest.Deps.Automation.
	// Optional: nil falls back to an immediate AI-draft enqueue.
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

// NormalizedAttachment is one inbound message's attachment handle, channel-
// neutral: ProviderRef is what the media-download worker task resolves
// bytes from (a WhatsApp Cloud media object id today; an Instagram/Messenger
// CDN url from Phase 4/5 — see worker.MediaDownloadTask.FileID, which either
// shape rides unchanged).
type NormalizedAttachment struct {
	MediaType   string
	Mimetype    string
	FileName    string
	ProviderRef string
	SourceURL   string
}

// NormalizedMessage is one channel-neutral inbound (or provider-echoed
// outbound) message event, produced by each Meta channel's own webhook
// package — see the package doc.
type NormalizedMessage struct {
	ExternalContactID  string
	ContactHandle      string
	ContactDisplayName string
	ExternalThreadID   string
	Direction          string // "in" | "out" (a provider echo of our own send)
	SenderKind         string // "contact" | "external_account"
	ExternalMessageID  string
	MessageKind        string
	Body               string
	Preview            string
	Raw                []byte
	MessageTS          time.Time
	Attachment         *NormalizedAttachment
}

// Outcome reports what Process did with one message.
type Outcome struct {
	// Status is "duplicate" | "stored" (unlike tgingest, metaingest has no
	// "ignored" outcome: each channel's own webhook.go filters unsupported
	// message types before Process is ever called — see
	// whatsappcloud.Message.Ignorable).
	Status string
	// Result is IngestChannelInbound's result, set for "duplicate"/"stored".
	Result store.ChannelInboundResult
}

// ErrIngest wraps a failure to durably store the event — the persistence
// write itself failed. ErrEnqueue wraps a failure to publish a downstream
// task: the event WAS stored, but a follow-up queue publish was refused.
// Both mean the same thing to a caller: this event is NOT durably handled —
// a webhook caller must answer without a 2xx so Meta redelivers.
var (
	ErrIngest  = errors.New("metaingest: ingest failed")
	ErrEnqueue = errors.New("metaingest: enqueue failed")
)

// Process turns one normalized message into inbox state: ingest (upsert
// contact/chat/message), then branch on whether this is a genuinely new
// message or a redelivery of one already stored.
func (p *Processor) Process(ctx context.Context, acct store.ChannelAccount, msg NormalizedMessage) (Outcome, error) {
	in := store.ChannelInbound{
		AccountID:          acct.ID,
		ExternalContactID:  msg.ExternalContactID,
		ContactHandle:      msg.ContactHandle,
		ContactDisplayName: msg.ContactDisplayName,
		ExternalThreadID:   msg.ExternalThreadID,
		Direction:          msg.Direction,
		SenderKind:         msg.SenderKind,
		ExternalMessageID:  msg.ExternalMessageID,
		MessageKind:        msg.MessageKind,
		Body:               msg.Body,
		Preview:            msg.Preview,
		Source:             "webhook",
		Raw:                msg.Raw,
		MessageTS:          msg.MessageTS,
	}

	res, err := p.store.IngestChannelInbound(ctx, in)
	if err != nil {
		p.log.Error("metaingest: ingest failed", "account_id", acct.ID, "channel", acct.Channel, "err", err)
		return Outcome{}, fmt.Errorf("%w: %v", ErrIngest, err)
	}
	// Best-effort: a failed activity stamp never invalidates an otherwise
	// successful ingest.
	if err := p.store.TouchChannelAccount(ctx, acct.ID); err != nil {
		p.log.Warn("metaingest: touch account failed", "account_id", acct.ID, "err", err)
	}

	if !res.MessageInserted {
		// A redelivery. The row is already there, so the ingest is settled —
		// but the FOLLOW-UP may not be: the first attempt could have died
		// between the commit and the enqueue. See tgingest.Process's
		// identical reasoning.
		p.emitMessage(ctx, "message.updated", res.MessageID)
		if msg.Direction == "in" {
			if err := p.reenqueueMissingDraft(ctx, res, acct); err != nil {
				return Outcome{Status: "duplicate", Result: res}, fmt.Errorf("%w: %v", ErrEnqueue, err)
			}
		}
		return Outcome{Status: "duplicate", Result: res}, nil
	}

	p.emitMessage(ctx, "message.created", res.MessageID)

	if msg.Attachment != nil {
		if err := p.store.InsertChannelMediaPending(ctx, res.MessageID, store.ChannelMediaMeta{
			MediaType: msg.Attachment.MediaType, Mimetype: msg.Attachment.Mimetype,
			FileName: msg.Attachment.FileName, ProviderRef: msg.Attachment.ProviderRef, SourceURL: msg.Attachment.SourceURL,
		}); err != nil {
			// A failed pending-row insert degrades to "attachment never
			// downloads" rather than failing the whole webhook — the message
			// itself is already durably stored.
			p.log.Error("metaingest: insert pending media failed", "message_id", res.MessageID, "err", err)
		} else {
			p.publishOrLog(ctx, queue.Message{Kind: queue.KindMediaDownload, Payload: worker.MediaDownloadTask{
				MessageID: res.MessageID, Channel: messaging.Channel(acct.Channel), AccountID: acct.ID,
				FileID: msg.Attachment.ProviderRef, BlobID: worker.ChannelBlobID(res.MessageID),
			}})
		}
	}

	if chat, cerr := p.store.ChatByID(ctx, res.ChatID); cerr == nil {
		name := "chat.updated"
		if res.ChatCreated {
			name = "chat.created"
		}
		p.hub.Broadcast(name, dto.MapChat(chat))
	}

	// Only a genuinely inbound message ever arms automation — an echo of our
	// own outbound send (Instagram/Messenger's is_echo, from Phase 4/5) must
	// not re-arm/generate an AI draft. WhatsApp Cloud never reaches this
	// branch with Direction=="out": it has no echo at all (see
	// whatsappcloud.Message's own doc comment).
	if msg.Direction != "in" {
		return Outcome{Status: "stored", Result: res}, nil
	}

	// Handing this off to automation is part of the durable unit as far as
	// the caller is concerned — see tgingest.Process's identical comment.
	if err := p.armAutomation(ctx, res.ChatID, acct.ID, acct.Channel, in.MessageTS); err != nil {
		p.log.Error("metaingest: automation arm failed; not acking", "chat_id", res.ChatID, "err", err)
		return Outcome{Status: "stored", Result: res}, fmt.Errorf("%w: %v", ErrEnqueue, err)
	}
	return Outcome{Status: "stored", Result: res}, nil
}

// StatusUpdate is one outbound message's delivery-status update — today only
// WhatsApp Cloud API webhooks carry these (Instagram/Messenger DMs have no
// delivery-status webhook at all — see store.AdvanceDeliveryState's own doc
// comment).
type StatusUpdate struct {
	AccountID         uuid.UUID
	Channel           string
	ExternalMessageID string
	Status            string
	Rank              int
}

// ApplyStatus advances a message's delivery state, monotonically — mirrors
// internal/whatsmeow's own applyReceiptUpdate. A status for a message this
// xchats install never sent (an id this install has no record of) is not an
// error, just nothing to update: applied reports false rather than the
// caller treating it as a webhook failure worth a 500/redelivery.
func (p *Processor) ApplyStatus(ctx context.Context, upd StatusUpdate) (applied bool, err error) {
	msgID, _, err := p.store.AdvanceDeliveryState(ctx, upd.Channel, upd.AccountID, upd.ExternalMessageID, upd.Status, upd.Rank)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrIngest, err)
	}
	p.emitMessage(ctx, "message.updated", msgID)
	return true, nil
}

// armAutomation hands a genuinely new (or never-drafted-redelivered) inbound
// customer message to p.automation — see tgingest.Processor.armAutomation.
func (p *Processor) armAutomation(ctx context.Context, chatID, accountID uuid.UUID, channel string, lastMessageAt time.Time) error {
	if p.automation == nil {
		return p.publish(ctx, queue.Message{Kind: queue.KindAIDraft, Payload: worker.AIDraftTask{ChatID: chatID}})
	}
	return p.automation.OnInboundMessage(ctx, chatID, accountID, channel, lastMessageAt)
}

// reenqueueMissingDraft is tgingest.Processor.reenqueueMissingDraft's
// metaingest twin — see its doc comment for the crash-recovery reasoning.
func (p *Processor) reenqueueMissingDraft(ctx context.Context, res store.ChannelInboundResult, acct store.ChannelAccount) error {
	has, err := p.store.HasDraftForTrigger(ctx, res.MessageID)
	if err != nil {
		p.log.Warn("metaingest: duplicate: draft lookup failed", "message_id", res.MessageID, "err", err)
		return nil
	}
	if has {
		return nil
	}
	if err := p.armAutomation(ctx, res.ChatID, acct.ID, acct.Channel, time.Now()); err != nil {
		p.log.Error("metaingest: duplicate: draft enqueue failed", "chat_id", res.ChatID, "err", err)
		return err
	}
	p.log.Info("metaingest: duplicate: re-enqueued the missing draft", "chat_id", res.ChatID)
	return nil
}

func (p *Processor) emitMessage(ctx context.Context, name string, id uuid.UUID) {
	msg, err := p.store.MessageByID(ctx, id)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			p.log.Error("metaingest: emit message failed", "err", err)
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
// produced a durable row — see tgingest.Processor.publishOrLog.
func (p *Processor) publishOrLog(parent context.Context, m queue.Message) {
	if err := p.publish(parent, m); err != nil {
		p.log.Error("metaingest: enqueue failed", "kind", m.Kind, "err", err)
	}
}
