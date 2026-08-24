// Package outbound is the shared outbound-delivery core: resolve the
// channel's sender, build the channel-neutral OutboundMessage (hydrating
// media from blob metadata), call Send, then stamp the outcome — sent with
// the provider's own id, or failed — and broadcast message.updated.
//
// internal/worker.handleOutboundSend and internal/campaign's Runner both
// call Deliver: the queue-consumed manual/AI/automation send path and the
// campaign runner's own claim-then-send path converge on this ONE function,
// so there is exactly one place that resolves a sender, hydrates media, and
// interprets a ChannelSender's result — never a second, drifting copy. The
// campaign runner calls Deliver directly rather than publishing to
// internal/queue: queue.InMem is explicitly lossy, at-most-once delivery
// requires the ledger commit to happen BEFORE the provider call, and at a
// campaign's own pace (about one message per interval) the queue would buy
// no throughput anyway.
package outbound

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// Task is one outbound send: text OR one media file, routed through the
// channel-neutral sender registry. Mirrors internal/worker.OutboundTask
// field-for-field — kept as its own type (rather than reusing that one)
// because internal/worker owns the queue payload shape, and
// internal/campaign must not import internal/worker just to build one.
type Task struct {
	MessageID uuid.UUID
	AccountID uuid.UUID
	Channel   messaging.Channel
	// Destination is the provider's opaque conversation address; the caller
	// must not normalize it (see internal/worker.OutboundTask's own doc
	// comment — doing so can turn a WhatsApp LID or group JID into a
	// different phone JID before the channel adapter sees it).
	Destination string
	Text        string
	MediaID     string // blob id; empty => text send
	Caption     string
}

// Deps is Deliver's dependency set — the narrow slice of a Worker's own
// fields the delivery core actually touches.
type Deps struct {
	Store   *store.Store
	Blob    blob.Store
	Hub     Broadcaster
	Senders *messaging.SenderRegistry
	Log     *slog.Logger
}

// Broadcaster is the realtime surface Deliver needs — satisfied by
// *realtime.Hub with zero adapter code. Declared as a narrow interface
// (rather than importing internal/realtime directly) so internal/campaign's
// Runner can pass the same testable Broadcaster it already builds for its
// own SSE events, matching internal/automation.Runner's precedent.
type Broadcaster interface {
	Broadcast(name string, data any)
}

// Deliver resolves t.Channel's sender, builds the OutboundMessage (hydrating
// media from blob metadata when t.MediaID is set), calls Send, then stamps
// the outcome on t.MessageID's own transport row — StampOutboundSent on
// success, SetDeliveryStateFor("failed") on any failure — and broadcasts
// message.updated either way. The returned SendResult is the zero value on
// error.
//
// Provider vocabulary never leaves this function: JIDs and chat ids travel
// only as the opaque Destination routing hint a channel-neutral
// OutboundMessage carries; each adapter resolves its own sending account
// from AccountID.
func Deliver(ctx context.Context, d Deps, t Task) (messaging.SendResult, error) {
	kind := "text"
	if t.MediaID != "" {
		kind = "media"
	}
	d.Log.Info("outbound send start", "message_id", t.MessageID, "account_id", t.AccountID,
		"channel", t.Channel, "to", maskDestination(t.Destination), "kind", kind)

	sender, err := d.Senders.Sender(t.Channel)
	if err != nil {
		d.Log.Error("outbound send failed", "message_id", t.MessageID, "channel", t.Channel, "err", err)
		d.markFailed(ctx, t)
		return messaging.SendResult{}, err
	}

	out := messaging.OutboundMessage{
		MessageID: t.MessageID.String(), AccountID: t.AccountID.String(), Channel: t.Channel,
		Text: t.Text, To: t.Destination,
	}
	if t.MediaID != "" {
		// The blob is the source of truth for kind/mimetype/filename; the
		// adapter re-reads the bytes itself, so nothing large rides this struct.
		media := &messaging.OutboundMedia{BlobID: t.MediaID, Caption: t.Caption}
		if _, meta, gerr := d.Blob.Get(t.MediaID); gerr == nil {
			media.Kind, media.Mimetype, media.FileName = meta.MediaType, meta.Mimetype, meta.FileName
		}
		out.Media = media
	}

	res, err := sender.Send(ctx, out)
	if err != nil {
		d.Log.Error("outbound send failed", "message_id", t.MessageID, "channel", t.Channel,
			"to", maskDestination(t.Destination), "kind", kind, "err", err)
		d.markFailed(ctx, t)
		return messaging.SendResult{}, err
	}
	if res.ExternalID == "" {
		// A success with no provider id means the gateway accepted the request
		// but produced no message; the bubble would silently stay unconfirmed.
		d.Log.Warn("outbound send returned no external id", "message_id", t.MessageID,
			"channel", t.Channel, "to", maskDestination(t.Destination))
	} else {
		d.Log.Info("outbound send ok", "message_id", t.MessageID, "channel", t.Channel,
			"external_id", res.ExternalID)
	}
	// Stamp the provider id so a real WhatsApp fromMe=true echo collapses onto
	// this row; a synthetic id (the simulator) has no echo to collapse, but
	// stamping it keeps the column's meaning ("this send's provider id")
	// consistent across channels.
	if err := d.Store.StampOutboundSent(ctx, string(t.Channel), t.MessageID, res.ExternalID); err != nil {
		return res, err
	}
	d.emitMessage(ctx, t.MessageID)
	return res, nil
}

func (d Deps) markFailed(ctx context.Context, t Task) {
	_ = d.Store.SetDeliveryStateFor(ctx, string(t.Channel), t.MessageID, "failed")
	d.emitMessage(ctx, t.MessageID)
}

func (d Deps) emitMessage(ctx context.Context, id uuid.UUID) {
	msg, err := d.Store.MessageByID(ctx, id)
	if err != nil {
		d.Log.Error("emit message", "err", err)
		return
	}
	d.Hub.Broadcast("message.updated", dto.MapMessage(msg))
}

// maskDestination keeps only the last four characters of the provider's user
// part for logs while preserving an address suffix that is useful to
// diagnose routing (for example, @lid versus @s.whatsapp.net).
func maskDestination(destination string) string {
	user, server, hasServer := strings.Cut(destination, "@")
	if len(user) > 4 {
		user = strings.Repeat("*", len(user)-4) + user[len(user)-4:]
	}
	if hasServer {
		return user + "@" + server
	}
	return user
}
