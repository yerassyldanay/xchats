// Package worker consumes the in-memory queue: it ingests raw Evolution events
// (normalize → idempotent upsert → SSE), downloads media, performs outbound
// sends, and runs the AI-draft stub. Workers expose no HTTP — the queue is their
// only interface.
package worker

import (
	"context"
	"encoding/base64"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/assistant"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/normalize"
	"github.com/yerassyldanay/xchats/backend/internal/playground"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// OutboundTask sends one part (text OR one media file) to WhatsApp.
type OutboundTask struct {
	MessageID uuid.UUID
	AccountID uuid.UUID
	Instance  string // the sending account's Evolution instance (multi-account)
	PhoneJID  string
	Text      string
	MediaID   string // blob id; empty => text send
	Caption   string
}

// MediaDownloadTask fetches media bytes when they weren't inline.
type MediaDownloadTask struct {
	MessageID          uuid.UUID
	Instance           string // the receiving account's Evolution instance
	EvolutionMessageID string
	BlobID             string
}

// AIDraftTask runs the stub Drafter for a chat's latest inbound.
type AIDraftTask struct {
	ChatID uuid.UUID
}

// ExtractMaterialTask runs a Stage-1 ingest adapter for one dropped material.
type ExtractMaterialTask struct {
	MaterialID uuid.UUID
	OrgID      uuid.UUID
}

// Worker holds the dependencies for all queue handlers.
type Worker struct {
	Store   *store.Store
	Queue   queue.Queue
	Evo     evolution.Client
	Blob    blob.Store
	Hub     *realtime.Hub
	Drafter assistant.Drafter
	KB      *kbstore.Store        // playground KB (nil in transport-only setups)
	Extract *playground.Extractor // Stage-1 ingest adapters (nil when KB is nil)
	Log     *slog.Logger
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
	case queue.KindExtractMaterial:
		return w.handleExtractMaterial(ctx, m.Payload.(ExtractMaterialTask))
	}
	return nil
}

// handleExtractMaterial runs the Stage-1 ingest adapter for one dropped material.
func (w *Worker) handleExtractMaterial(ctx context.Context, t ExtractMaterialTask) error {
	if w.KB == nil || w.Extract == nil {
		return nil
	}
	return w.Extract.Extract(ctx, w.KB, w.Blob, w.Hub, t.OrgID, t.MaterialID)
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
		_ = w.Queue.Publish(queue.Message{Kind: queue.KindAIDraft, Payload: AIDraftTask{ChatID: res.ChatID}})
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
		_ = w.Queue.Publish(queue.Message{Kind: queue.KindMediaDownload, Payload: MediaDownloadTask{
			MessageID: messageID, Instance: instance, EvolutionMessageID: m.EvolutionMessageID, BlobID: blobID,
		}})
	}
	w.emitMessage(ctx, "message.updated", messageID)
}

func (w *Worker) handleMediaDownload(ctx context.Context, t MediaDownloadTask) error {
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

// --- outbound sends -------------------------------------------------------

// maskPhone keeps only the last 4 digits for logs (PII redaction): "77058686509"
// → "*******6509". Short/empty values are returned as-is.
func maskPhone(p string) string {
	if len(p) <= 4 {
		return p
	}
	return strings.Repeat("*", len(p)-4) + p[len(p)-4:]
}

func (w *Worker) handleOutboundSend(ctx context.Context, t OutboundTask) error {
	number := config.PhoneFromJID(t.PhoneJID) // phone, never the @lid
	kind := "text"
	if t.MediaID != "" {
		kind = "media"
	}
	// instance="" means the worker falls back to the client's default instance —
	// surface it so a missing account→instance mapping is visible in logs.
	w.Log.Info("outbound send start", "message_id", t.MessageID, "account_id", t.AccountID,
		"instance", t.Instance, "phone", maskPhone(number), "kind", kind)

	var res evolution.SendResult
	var err error
	if t.MediaID == "" {
		res, err = w.Evo.SendText(ctx, t.Instance, number, t.Text)
	} else {
		data, meta, gerr := w.Blob.Get(t.MediaID)
		if gerr != nil {
			err = gerr
		} else {
			b64 := base64.StdEncoding.EncodeToString(data)
			res, err = w.Evo.SendMedia(ctx, t.Instance, number, meta.MediaType, meta.Mimetype, b64, meta.FileName, t.Caption)
		}
	}
	if err != nil {
		w.Log.Error("outbound send failed", "message_id", t.MessageID, "instance", t.Instance,
			"phone", maskPhone(number), "kind", kind, "err", err)
		_ = w.Store.SetDeliveryState(ctx, t.MessageID, "failed")
		w.emitMessage(ctx, "message.updated", t.MessageID)
		return err
	}
	// A 2xx with no key id means the gateway accepted the request but produced no
	// message — usually the number isn't on WhatsApp or the instance dropped. The
	// fromMe echo can't correlate, so the bubble would silently stay unconfirmed.
	if res.KeyID == "" {
		w.Log.Warn("outbound send returned no message id", "message_id", t.MessageID,
			"instance", t.Instance, "phone", maskPhone(number), "status", res.Status)
	} else {
		w.Log.Info("outbound send ok", "message_id", t.MessageID, "instance", t.Instance,
			"key_id", res.KeyID, "status", res.Status)
	}
	// Stamp the gateway id so the fromMe=true echo collapses onto this row.
	if err := w.Store.StampEvolutionID(ctx, t.MessageID, res.KeyID); err != nil {
		return err
	}
	w.emitMessage(ctx, "message.updated", t.MessageID)
	return nil
}

// --- AI draft stub --------------------------------------------------------

func (w *Worker) handleAIDraft(ctx context.Context, t AIDraftTask) error {
	chat, err := w.Store.ChatByID(ctx, t.ChatID)
	if err != nil {
		return err
	}
	trigger, err := w.Store.LatestInboundMessageID(ctx, t.ChatID)
	if err != nil {
		return err
	}
	opts, err := w.Drafter.Draft(ctx, assistant.Input{
		ChatID:      t.ChatID.String(),
		ContactName: chat.Contact.PushName,
	})
	if err != nil {
		return err
	}
	dopts := make([]store.DraftOption, 0, len(opts))
	for _, o := range opts {
		so := store.DraftOption{
			Ordinal: o.Ordinal, Text: o.Text, Confidence: o.Confidence,
			Escalate: o.Escalate, EscalationReason: o.Reason,
		}
		for i, md := range o.Media {
			so.Assets = append(so.Assets, store.DraftAsset{
				AssetRef: md.Ref, MediaKind: md.Kind, MediaURL: md.URL, Ordinal: i + 1,
			})
		}
		dopts = append(dopts, so)
	}
	drafts, err := w.Store.WriteDraftSet(ctx, t.ChatID, trigger, dopts)
	if err != nil {
		return err
	}
	for _, d := range drafts {
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
