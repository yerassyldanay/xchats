// Package worker consumes the in-memory queue: it ingests raw Evolution events
// (normalize → idempotent upsert → SSE), downloads media, performs outbound
// sends, and runs the multichannel response engine. Workers expose no HTTP —
// the queue is their only interface.
package worker

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
	"github.com/yerassyldanay/xchats/backend/internal/normalize"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/response"
)

// OutboundTask sends one part (text OR one media file). Channel selects the
// messaging.ChannelSender it routes through (Worker.Senders) — text AND media
// alike: media used to bypass the registry on a direct Evolution call, which
// meant every new channel had to re-implement it.
type OutboundTask struct {
	MessageID uuid.UUID
	AccountID uuid.UUID
	Channel   messaging.Channel
	Instance  string // the sending account's Evolution instance (multi-account)
	PhoneJID  string
	Text      string
	MediaID   string // blob id; empty => text send
	Caption   string
}

// MediaDownloadTask fetches media bytes when they weren't inline. It carries no
// credential: the Telegram branch resolves the bot token from the store by
// AccountID, so a token never rides the queue.
type MediaDownloadTask struct {
	MessageID uuid.UUID
	Channel   messaging.Channel
	AccountID uuid.UUID
	BlobID    string

	// WhatsApp: the Evolution instance + message id to re-fetch from.
	Instance           string
	EvolutionMessageID string

	// Telegram: the file handle to resolve through getFile.
	FileID string
}

// AIDraftTask runs the response engine for a chat's latest inbound message.
type AIDraftTask struct {
	ChatID uuid.UUID
}

// Worker holds the dependencies for all queue handlers.
type Worker struct {
	Store    *store.Store
	Queue    queue.Queue
	Evo      evolution.Client
	TG       telegram.Client // nil in a WhatsApp-only wiring; the media sweep no-ops
	Blob     blob.Store
	Hub      *realtime.Hub
	Response *response.Service         // the multichannel response engine's entry point
	Senders  *messaging.SenderRegistry // channel -> ChannelSender, for outbound text sends
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
	case queue.KindWaEvent:
		return w.handleWaEvent(ctx, m.Payload.([]byte))
	case queue.KindMediaDownload:
		return w.handleMediaDownload(ctx, m.Payload.(MediaDownloadTask))
	case queue.KindOutboundSend:
		return w.handleOutboundSend(ctx, m.Payload.(OutboundTask))
	case queue.KindAIDraft:
		return w.handleAIDraft(ctx, m.Payload.(AIDraftTask))
	}
	return nil
}

// --- inbound events -------------------------------------------------------

func (w *Worker) handleWaEvent(ctx context.Context, raw []byte) error {
	env, err := normalize.ParseEnvelope(raw)
	if err != nil {
		return err
	}
	if env.Event == "messages.update" {
		return w.handleStatus(ctx, env)
	}
	if env.Event == "connection.update" {
		return w.handleConnection(ctx, env)
	}
	m, err := env.Message()
	if err != nil || m == nil {
		return err
	}
	if m.IsGroup {
		w.Log.Info("wa event dropped", "event", env.Event, "result", "group")
		return nil
	}
	acct := config.AccountID(m.OwnerJID)
	account, err := w.Store.AccountByID(ctx, acct)
	if err != nil {
		w.Log.Warn("wa event dropped", "event", env.Event, "result", "unknown_account")
		w.Log.Debug("unknown account detail", "owner_jid", m.OwnerJID)
		return nil
	}
	if m.PhoneJID == "" {
		w.Log.Warn("wa event dropped", "event", env.Event, "result", "no_phone")
		return nil
	}

	senderKind := "contact"
	if m.Direction == "out" {
		senderKind = "external_account"
	}
	res, err := w.Store.UpsertInbound(ctx, store.InboundUpsert{
		AccountID:          acct,
		PhoneJID:           m.PhoneJID,
		LidJID:             m.LidJID,
		RemoteJID:          m.ContactJID,
		PhoneNumber:        config.PhoneFromJID(m.PhoneJID),
		PushName:           m.PushName,
		Direction:          m.Direction,
		SenderKind:         senderKind,
		EvolutionMessageID: m.EvolutionMessageID,
		MessageKind:        m.MessageKind,
		Body:               m.Text,
		Preview:            previewFor(m),
		Source:             "live_webhook",
		Raw:                m.Raw,
		MessageTS:          m.Timestamp,
	})
	if err != nil {
		return err
	}

	if !res.MessageInserted {
		// dedup or own-send echo collapsing onto the existing row: enrichment only.
		w.Log.Info("wa event processed", "event", env.Event, "account_id", acct,
			"direction", m.Direction, "result", "duplicate")
		w.emitMessage(ctx, "message.updated", res.MessageID)
		return nil
	}
	w.Log.Info("wa event processed", "event", env.Event, "account_id", acct,
		"direction", m.Direction, "result", "new")
	w.emitMessage(ctx, "message.created", res.MessageID)
	if res.ChatCreated {
		w.emitChat(ctx, "chat.created", res.ChatID)
	} else {
		w.emitChat(ctx, "chat.updated", res.ChatID)
	}
	if m.Media != nil {
		w.ingestMedia(ctx, res.MessageID, account.InstanceName, m)
	}
	// Auto-draft: a brand-new inbound customer message gets a fresh AI suggestion
	// pushed to the inbox without anyone pressing "Suggest". handleAIDraft's
	// WriteDraftSet supersedes the chat's prior pending options, so the panel
	// always reflects the latest message instead of a stale first draft. Outbound
	// echoes (direction "out") and dedup hits (handled above) never reach here.
	if m.Direction == "in" {
		_ = w.publish(ctx, queue.Message{Kind: queue.KindAIDraft, Payload: AIDraftTask{ChatID: res.ChatID}})
	}
	return nil
}

// handleConnection maps a connection.update (instance + state) to its account and
// updates the live status badge. Unknown/pre-connect instances are a no-op.
func (w *Worker) handleConnection(ctx context.Context, env normalize.Envelope) error {
	state := env.ConnectionState()
	mapped := map[string]string{"open": "connected", "close": "disconnected", "connecting": "connecting"}[state]
	if mapped == "" {
		return nil
	}
	id, err := w.Store.SetAccountStateByInstance(ctx, env.Instance, mapped)
	if err == store.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if acct, aerr := w.Store.AccountByID(ctx, id); aerr == nil {
		w.Hub.Broadcast("wa_account.status_changed", dto.MapAccount(acct, mapped))
	}
	return nil
}

func (w *Worker) handleStatus(ctx context.Context, env normalize.Envelope) error {
	su, err := env.StatusUpdate()
	if err != nil || su == nil || su.DeliveryState == "" {
		return err
	}
	acct := config.AccountID(env.Sender)
	msgID, _, err := w.Store.AdvanceDeliveryState(ctx, acct, su.EvolutionMessageID, su.DeliveryState, normalize.DeliveryRank(su.DeliveryState))
	if err == store.ErrNotFound {
		return nil // unknown id or not a forward transition
	}
	if err != nil {
		return err
	}
	// Surface delivery outcomes — especially failures, which otherwise only show as
	// a silent ⚠️ in the UI. A send can return 2xx yet still fail delivery here.
	if su.DeliveryState == "failed" {
		w.Log.Warn("delivery failed", "message_id", msgID, "evolution_message_id", su.EvolutionMessageID,
			"state", su.DeliveryState)
	} else {
		w.Log.Info("delivery update", "message_id", msgID, "evolution_message_id", su.EvolutionMessageID,
			"state", su.DeliveryState)
	}
	w.emitMessage(ctx, "message.updated", msgID)
	return nil
}

// ingestMedia writes the blob (inline or via a download task) and the media row.
func (w *Worker) ingestMedia(ctx context.Context, messageID uuid.UUID, instance string, m *normalize.Message) {
	blobID := "msg-" + m.EvolutionMessageID // deterministic: a re-delivery overwrites
	ref := store.MediaRef{
		MediaType: m.Media.Kind,
		Mimetype:  m.Media.Mimetype,
		FileName:  m.Media.FileName,
		FileSize:  int(m.Media.FileSize),
	}
	status := "pending"
	if m.Media.Base64 != "" {
		if data, err := base64.StdEncoding.DecodeString(m.Media.Base64); err == nil && len(data) > 0 {
			if _, err := w.Blob.Put(blobID, data, blob.Meta{
				MediaType: ref.MediaType, Mimetype: ref.Mimetype, FileName: ref.FileName, FileSize: int64(len(data)),
			}); err == nil {
				status = "ready"
				ref.FileSize = len(data)
			}
		}
	}
	if _, _, err := w.Store.UpsertMessageMedia(ctx, messageID, ref, blobID, status); err != nil {
		w.Log.Error("upsert media", "err", err)
		return
	}
	if status == "pending" {
		_ = w.publish(ctx, queue.Message{Kind: queue.KindMediaDownload, Payload: MediaDownloadTask{
			MessageID: messageID, Channel: messaging.ChannelWhatsApp,
			Instance: instance, EvolutionMessageID: m.EvolutionMessageID, BlobID: blobID,
		}})
	}
	w.emitMessage(ctx, "message.updated", messageID)
}

func (w *Worker) handleMediaDownload(ctx context.Context, t MediaDownloadTask) error {
	if t.Channel == messaging.ChannelTelegram {
		return w.downloadTelegramMedia(ctx, t)
	}
	b64, fileName, mimetype, err := w.Evo.GetBase64(ctx, t.Instance, t.EvolutionMessageID)
	if err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return err
	}
	if _, err := w.Blob.Put(t.BlobID, data, blob.Meta{Mimetype: mimetype, FileName: fileName, FileSize: int64(len(data))}); err != nil {
		return err
	}
	if err := w.Store.SetMediaReady(ctx, t.MessageID, len(data)); err != nil {
		return err
	}
	w.emitMessage(ctx, "message.updated", t.MessageID)
	return nil
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

// maskPhone keeps only the last 4 digits for logs (PII redaction): "77058686509"
// → "*******6509". Short/empty values are returned as-is.
func maskPhone(p string) string {
	if len(p) <= 4 {
		return p
	}
	return strings.Repeat("*", len(p)-4) + p[len(p)-4:]
}

// handleOutboundSend routes EVERY send — text and media alike — through the
// channel-neutral sender registry (Worker.Senders), so WhatsApp, simulator and
// Telegram conversations share one outbound path. Media used to bypass the
// registry on a direct Evolution call, which meant each new channel had to
// re-implement it; the Evolution adapter now owns that call.
//
// Provider vocabulary never leaves this function: JIDs, instance names and chat
// ids travel only as the opaque To/Route routing hints a channel-neutral
// OutboundMessage carries.
func (w *Worker) handleOutboundSend(ctx context.Context, t OutboundTask) error {
	// PhoneFromJID passes a non-JID string straight through, so a numeric
	// Telegram chat id survives untouched.
	destination := config.PhoneFromJID(t.PhoneJID) // phone, never the @lid
	kind := "text"
	if t.MediaID != "" {
		kind = "media"
	}
	w.Log.Info("outbound send start", "message_id", t.MessageID, "account_id", t.AccountID,
		"channel", t.Channel, "instance", t.Instance, "to", maskPhone(destination), "kind", kind)

	sender, err := w.Senders.Sender(t.Channel)
	if err != nil {
		w.Log.Error("outbound send failed", "message_id", t.MessageID, "channel", t.Channel, "err", err)
		_ = w.Store.SetDeliveryStateFor(ctx, string(t.Channel), t.MessageID, "failed")
		w.emitMessage(ctx, "message.updated", t.MessageID)
		return err
	}

	out := messaging.OutboundMessage{
		MessageID: t.MessageID.String(), AccountID: t.AccountID.String(), Channel: t.Channel,
		Text: t.Text, To: destination, Route: t.Instance,
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
			"instance", t.Instance, "to", maskPhone(destination), "kind", kind, "err", err)
		_ = w.Store.SetDeliveryStateFor(ctx, string(t.Channel), t.MessageID, "failed")
		w.emitMessage(ctx, "message.updated", t.MessageID)
		return err
	}
	if res.ExternalID == "" {
		// A success with no provider id means the gateway accepted the request but
		// produced no message; the bubble would silently stay unconfirmed.
		w.Log.Warn("outbound send returned no external id", "message_id", t.MessageID,
			"channel", t.Channel, "instance", t.Instance, "to", maskPhone(destination))
	} else {
		w.Log.Info("outbound send ok", "message_id", t.MessageID, "channel", t.Channel,
			"instance", t.Instance, "external_id", res.ExternalID)
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

func (w *Worker) emitChat(ctx context.Context, name string, id uuid.UUID) {
	chat, err := w.Store.ChatByID(ctx, id)
	if err != nil {
		w.Log.Error("emit chat", "err", err)
		return
	}
	w.Hub.Broadcast(name, dto.MapChat(chat))
}

func previewFor(m *normalize.Message) string {
	if m.Text != "" {
		if len([]rune(m.Text)) > 120 {
			return string([]rune(m.Text)[:120])
		}
		return m.Text
	}
	if m.Media != nil {
		switch m.Media.Kind {
		case "image":
			return "📷 Фото"
		case "video":
			return "🎥 Видео"
		case "audio":
			return "🎙 Аудио"
		case "document":
			return "📄 Документ"
		case "sticker":
			return "🌟 Стикер"
		}
	}
	return ""
}
