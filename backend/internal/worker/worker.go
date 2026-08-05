// Package worker consumes the in-memory queue: downloads Telegram media,
// performs outbound sends, and runs the multichannel response engine.
// WhatsApp's own inbound ingestion and media download live entirely inside
// internal/whatsmeow (the manager calls the store/hub directly and enqueues
// only the AI-draft step here) — this package never sees a whatsmeow type.
// Workers expose no HTTP — the queue is their only interface.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// OutboundTask sends one part (text OR one media file), routed through the
// channel-neutral sender registry (Worker.Senders) — text AND media alike.
type OutboundTask struct {
	MessageID uuid.UUID
	AccountID uuid.UUID
	Channel   messaging.Channel
	// Destination is the provider's opaque conversation address. The worker
	// must not normalize it: doing so can turn a WhatsApp LID or group JID into
	// a different phone JID before the channel adapter sees it.
	Destination string
	Text        string
	MediaID     string // blob id; empty => text send
	Caption     string
}

// MediaDownloadTask fetches a Telegram attachment's bytes when they weren't
// inline. It carries no credential: the bot token is resolved from the store
// by AccountID, so a token never rides the queue. WhatsApp media download has
// no equivalent task here — internal/whatsmeow downloads inline, since only
// it holds the whatsmeow-specific message object the download call needs.
type MediaDownloadTask struct {
	MessageID uuid.UUID
	Channel   messaging.Channel
	AccountID uuid.UUID
	BlobID    string
	FileID    string // the file handle to resolve through getFile
}

// AIDraftTask runs the response engine for a chat's latest inbound message.
type AIDraftTask struct {
	ChatID uuid.UUID
}

// Worker holds the dependencies for all queue handlers.
type Worker struct {
	Store    *store.Store
	Queue    queue.Queue
	TG       telegram.Client // nil in a WhatsApp-only wiring; the media sweep no-ops
	Blob     blob.Store
	Hub      *realtime.Hub
	Response *response.Service         // the multichannel response engine's entry point
	Senders  *messaging.SenderRegistry // channel -> ChannelSender, for outbound sends
	Log      *slog.Logger
}

// publishTimeout bounds every enqueue a handler makes. A worker that fans work
// back onto a full queue must fail fast (and say so) rather than occupy a pool
// slot forever — the pool is what would drain that buffer.
const publishTimeout = 2 * time.Second

// publish enqueues with a bounded deadline; a failure is logged and returned so
// the caller decides whether it is fatal to its task.
func (w *Worker) publish(parent context.Context, m queue.Message) error {
	pctx, cancel := context.WithTimeout(parent, publishTimeout)
	defer cancel()
	if err := w.Queue.Publish(pctx, m); err != nil {
		w.Log.Error("enqueue failed", "kind", m.Kind, "err", err)
		return err
	}
	return nil
}

// Handle is the queue dispatcher.
func (w *Worker) Handle(ctx context.Context, m queue.Message) error {
	switch m.Kind {
	case queue.KindMediaDownload:
		return w.handleMediaDownload(ctx, m.Payload.(MediaDownloadTask))
	case queue.KindOutboundSend:
		return w.handleOutboundSend(ctx, m.Payload.(OutboundTask))
	case queue.KindAIDraft:
		return w.handleAIDraft(ctx, m.Payload.(AIDraftTask))
	}
	return nil
}

// --- inbound events (Telegram media only — see the package doc comment) ---

func (w *Worker) handleMediaDownload(ctx context.Context, t MediaDownloadTask) error {
	return w.downloadTelegramMedia(ctx, t)
}

// downloadTelegramMedia fetches an inbound attachment's bytes: getFile resolves
// the handle to a path, the file endpoint serves it, and the media row flips to
// 'ready'. A failure leaves the row 'pending' with its file_id intact, which is
// what the sweeper retries from — there is no event to replay.
func (w *Worker) downloadTelegramMedia(ctx context.Context, t MediaDownloadTask) error {
	if w.TG == nil {
		return nil
	}
	token, err := w.Store.TelegramBotToken(ctx, t.AccountID)
	if err != nil {
		return fmt.Errorf("telegram media: resolve token: %w", err)
	}
	info, err := w.TG.GetFile(ctx, token, t.FileID)
	if err != nil {
		w.markTelegramMediaFailed(ctx, t.MessageID, err)
		return fmt.Errorf("telegram media: getFile: %w", err)
	}
	data, err := w.TG.DownloadFile(ctx, token, info.FilePath)
	if err != nil {
		w.markTelegramMediaFailed(ctx, t.MessageID, err)
		return fmt.Errorf("telegram media: download: %w", err)
	}
	meta, err := w.Store.TelegramMediaMeta(ctx, t.MessageID)
	if err != nil {
		return err
	}
	if _, err := w.Blob.Put(t.BlobID, data, blob.Meta{
		MediaType: meta.MediaType, Mimetype: meta.Mimetype, FileName: meta.FileName, FileSize: int64(len(data)),
	}); err != nil {
		return err
	}
	if err := w.Store.SetTelegramMediaReady(ctx, t.MessageID, t.BlobID, len(data)); err != nil {
		return err
	}
	w.Log.Info("telegram media downloaded", "message_id", t.MessageID, "bytes", len(data))
	w.emitMessage(ctx, "message.updated", t.MessageID)
	return nil
}

// markTelegramMediaFailed records a permanent-looking failure so the sweeper's
// backoff sees a fresh timestamp instead of hammering the same broken file_id.
func (w *Worker) markTelegramMediaFailed(ctx context.Context, messageID uuid.UUID, cause error) {
	if err := w.Store.SetTelegramMediaFailed(ctx, messageID); err != nil {
		w.Log.Error("mark telegram media failed", "message_id", messageID, "err", err)
	}
	w.Log.Warn("telegram media download failed", "message_id", messageID, "err", cause)
}

// SweepTelegramMedia re-enqueues attachments whose bytes were never fetched:
// rows still 'pending' (or 'failed') past a quiet period. This is the retry
// mechanism the design trades an inbound-event table for — the media ROW is the
// durable work item, so a crashed or failed download is recoverable from the
// database alone.
func (w *Worker) SweepTelegramMedia(ctx context.Context, olderThan time.Duration, limit int) (int, error) {
	pending, err := w.Store.PendingTelegramMedia(ctx, olderThan, limit)
	if err != nil {
		return 0, err
	}
	queued := 0
	for _, p := range pending {
		if err := w.publish(ctx, queue.Message{Kind: queue.KindMediaDownload, Payload: MediaDownloadTask{
			MessageID: p.MessageID, Channel: messaging.ChannelTelegram, AccountID: p.AccountID,
			FileID: p.FileID, BlobID: TelegramBlobID(p.MessageID),
		}}); err != nil {
			return queued, err
		}
		queued++
	}
	if queued > 0 {
		w.Log.Info("telegram media sweep", "queued", queued)
	}
	return queued, nil
}

// StartTelegramMediaSweeper runs SweepTelegramMedia once at startup (picking up
// anything a crash left behind) and then on a ticker until ctx is done.
func (w *Worker) StartTelegramMediaSweeper(ctx context.Context, every, olderThan time.Duration, limit int) {
	go func() {
		if _, err := w.SweepTelegramMedia(ctx, 0, limit); err != nil {
			w.Log.Error("telegram media sweep (startup)", "err", err)
		}
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if _, err := w.SweepTelegramMedia(ctx, olderThan, limit); err != nil {
					w.Log.Error("telegram media sweep", "err", err)
				}
			}
		}
	}()
}

// TelegramBlobID is the deterministic blob key for a Telegram attachment: keyed
// by the message row, so a retried download overwrites rather than accumulates.
func TelegramBlobID(messageID uuid.UUID) string { return "tg-" + messageID.String() }

// --- outbound sends -------------------------------------------------------

// maskDestination keeps only the last four characters of the provider's user
// part for logs while preserving an address suffix that is useful to diagnose
// routing (for example, @lid versus @s.whatsapp.net).
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

// handleOutboundSend routes EVERY send — text and media alike — through the
// channel-neutral sender registry (Worker.Senders), so WhatsApp, simulator and
// Telegram conversations share one outbound path.
//
// Provider vocabulary never leaves this function: JIDs and chat ids travel
// only as the opaque To routing hint a channel-neutral OutboundMessage
// carries; each adapter resolves the sending account itself from AccountID.
func (w *Worker) handleOutboundSend(ctx context.Context, t OutboundTask) error {
	destination := t.Destination
	kind := "text"
	if t.MediaID != "" {
		kind = "media"
	}
	w.Log.Info("outbound send start", "message_id", t.MessageID, "account_id", t.AccountID,
		"channel", t.Channel, "to", maskDestination(destination), "kind", kind)

	sender, err := w.Senders.Sender(t.Channel)
	if err != nil {
		w.Log.Error("outbound send failed", "message_id", t.MessageID, "channel", t.Channel, "err", err)
		_ = w.Store.SetDeliveryStateFor(ctx, string(t.Channel), t.MessageID, "failed")
		w.emitMessage(ctx, "message.updated", t.MessageID)
		return err
	}

	out := messaging.OutboundMessage{
		MessageID: t.MessageID.String(), AccountID: t.AccountID.String(), Channel: t.Channel,
		Text: t.Text, To: destination,
	}
	if t.MediaID != "" {
		// The blob is the source of truth for kind/mimetype/filename; the adapter
		// re-reads the bytes itself, so nothing large rides this struct.
		media := &messaging.OutboundMedia{BlobID: t.MediaID, Caption: t.Caption}
		if _, meta, gerr := w.Blob.Get(t.MediaID); gerr == nil {
			media.Kind, media.Mimetype, media.FileName = meta.MediaType, meta.Mimetype, meta.FileName
		}
		out.Media = media
	}

	res, err := sender.Send(ctx, out)
	if err != nil {
		w.Log.Error("outbound send failed", "message_id", t.MessageID, "channel", t.Channel,
			"to", maskDestination(destination), "kind", kind, "err", err)
		_ = w.Store.SetDeliveryStateFor(ctx, string(t.Channel), t.MessageID, "failed")
		w.emitMessage(ctx, "message.updated", t.MessageID)
		return err
	}
	if res.ExternalID == "" {
		// A success with no provider id means the gateway accepted the request but
		// produced no message; the bubble would silently stay unconfirmed.
		w.Log.Warn("outbound send returned no external id", "message_id", t.MessageID,
			"channel", t.Channel, "to", maskDestination(destination))
	} else {
		w.Log.Info("outbound send ok", "message_id", t.MessageID, "channel", t.Channel,
			"external_id", res.ExternalID)
	}
	// Stamp the provider id so a real WhatsApp fromMe=true echo collapses onto
	// this row; the simulator's synthetic id has no echo to collapse, but
	// stamping it keeps the column's meaning ("this send's provider id")
	// consistent across channels.
	if err := w.Store.StampOutboundSent(ctx, string(t.Channel), t.MessageID, res.ExternalID); err != nil {
		return err
	}
	w.emitMessage(ctx, "message.updated", t.MessageID)
	return nil
}

// --- AI response engine ----------------------------------------------------

// handleAIDraft calls the multichannel response engine for a chat's latest
// inbound message and persists exactly one suggested draft. It derives the
// channel from the chat's own account (never hardcoded), so the exact same
// path serves a WhatsApp conversation and a simulator conversation's async
// ("wait_for_response=false") request — both ride this same queue task.
func (w *Worker) handleAIDraft(ctx context.Context, t AIDraftTask) error {
	chat, err := w.Store.ChatByID(ctx, t.ChatID)
	if err != nil {
		return err
	}
	account, err := w.Store.AccountByID(ctx, chat.AccountID)
	if err != nil {
		return err
	}
	persisted, err := w.Response.Respond(ctx, messaging.Channel(account.Channel), t.ChatID.String(), response.RespondOptions{})
	if err != nil {
		return err
	}
	for _, p := range persisted {
		id, perr := uuid.Parse(p.ID)
		if perr != nil {
			w.Log.Error("reload persisted draft: bad id", "draft_id", p.ID, "err", perr)
			continue
		}
		d, derr := w.Store.DraftByID(ctx, id)
		if derr != nil {
			w.Log.Error("reload persisted draft", "draft_id", p.ID, "err", derr)
			continue
		}
		w.Hub.Broadcast("ai_draft.created", dto.MapDraft(d))
	}
	return nil
}

// --- helpers --------------------------------------------------------------

func (w *Worker) emitMessage(ctx context.Context, name string, id uuid.UUID) {
	msg, err := w.Store.MessageByID(ctx, id)
	if err != nil {
		w.Log.Error("emit message", "err", err)
		return
	}
	w.Hub.Broadcast(name, dto.MapMessage(msg))
}
