// Package worker consumes the in-memory queue: downloads Telegram and Meta-
// channel media, performs outbound sends, and runs the multichannel response
// engine. WhatsApp's own inbound ingestion and media download live entirely
// inside internal/whatsmeow (the manager calls the store/hub directly and
// enqueues only the AI-draft step here) — this package never sees a
// whatsmeow type. Workers expose no HTTP — the queue is their only interface.
package worker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/outbound"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/stt"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/whatsappcloud"
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

// MediaDownloadTask fetches a Telegram or Meta-channel attachment's bytes
// when they weren't inline. It carries no credential: the bot/business token
// is resolved from the store by AccountID, so a token never rides the queue.
// WhatsApp media download has no equivalent task here — internal/whatsmeow
// downloads inline, since only it holds the whatsmeow-specific message
// object the download call needs.
type MediaDownloadTask struct {
	MessageID uuid.UUID
	Channel   messaging.Channel
	AccountID uuid.UUID
	BlobID    string
	// FileID is the provider's own attachment handle: a Telegram file_id
	// (resolved through getFile), or a WhatsApp Cloud media object id
	// (resolved through Client.MediaInfo).
	FileID string
}

// AIDraftTask runs the response engine for a chat's latest inbound message.
type AIDraftTask struct {
	ChatID uuid.UUID
}

// Worker holds the dependencies for all queue handlers.
type Worker struct {
	Store   *store.Store
	Queue   queue.Queue
	TG      telegram.Client       // nil in a WhatsApp-only wiring; the media sweep no-ops
	WACloud *whatsappcloud.Client // nil when no Meta channel is configured; the media sweep no-ops
	// MetaClient backs RefreshExpiringInstagramTokens (meta_tokens.go) — nil
	// in a wiring with no Meta channels configured; the refresher no-ops.
	MetaClient *meta.Client
	Blob       blob.Store
	Hub        *realtime.Hub
	Response   *response.Service         // the multichannel response engine's entry point
	Senders    *messaging.SenderRegistry // channel -> ChannelSender, for outbound sends
	// STT resolves the CURRENT speech-to-text configuration — called fresh
	// on every audio attachment (mirrors response.Engine.Params), so a
	// Settings UI change takes effect on the very next voice note with no
	// restart. nil (a wiring with no STT provider configured at all) makes
	// transcribeIfAudio a no-op — a composition root that never sets this
	// simply never transcribes, exactly like a nil VisionModel never routes
	// to vision.
	STT func(ctx context.Context) stt.Params
	// Automation, when set, re-arms a chat's debounce/version-gated
	// generation once a voice note's transcript arrives — see
	// TranscribeAudio's own doc comment. nil falls back to a direct
	// KindAIDraft enqueue, the pre-automation behavior.
	Automation Automation
	Log        *slog.Logger
}

// Automation is the narrow surface TranscribeAudio needs to re-arm a chat's
// debounce/version-gated draft generation once a transcript becomes
// available — the same shape internal/tgingest.Automation and
// internal/whatsmeow.AutomationScheduler already declare independently for
// the exact same "hand off a customer message without a direct import"
// reason, so *automation.Scheduler satisfies this with zero adapter code.
type Automation interface {
	OnInboundMessage(ctx context.Context, chatID, accountID uuid.UUID, channel string, lastMessageAt time.Time) error
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

// --- inbound events (Telegram media) ---------------------------------------

func (w *Worker) handleMediaDownload(ctx context.Context, t MediaDownloadTask) error {
	switch t.Channel {
	case messaging.ChannelTelegram:
		return w.downloadTelegramMedia(ctx, t)
	case messaging.ChannelWhatsAppCloud:
		return w.downloadMetaMedia(ctx, t)
	case messaging.ChannelInstagram, messaging.ChannelMessenger:
		return w.downloadDirectMedia(ctx, t)
	default:
		w.Log.Error("media download: unsupported channel", "channel", t.Channel, "message_id", t.MessageID)
		return nil
	}
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
	w.transcribeIfAudio(ctx, t.MessageID, t.AccountID, string(t.Channel), meta.MediaType, t.BlobID)
	w.rearmAfterMediaReady(ctx, t.MessageID, t.AccountID, string(t.Channel), meta.MediaType)
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

// sweepAllMedia runs both media sweeps once — the tick body StartMediaSweeper
// shares between its startup pass and its ticker.
func (w *Worker) sweepAllMedia(ctx context.Context, olderThan time.Duration, limit int) {
	if _, err := w.SweepTelegramMedia(ctx, olderThan, limit); err != nil {
		w.Log.Error("telegram media sweep", "err", err)
	}
	if _, err := w.SweepChannelMedia(ctx, olderThan, limit); err != nil {
		w.Log.Error("meta channel media sweep", "err", err)
	}
}

// StartMediaSweeper runs SweepTelegramMedia and SweepChannelMedia once at
// startup (picking up anything a crash left behind) and then together on ONE
// ticker until ctx is done — both are the same "retry attachments whose
// bytes never arrived" concern, so they share a single goroutine rather than
// each running its own.
func (w *Worker) StartMediaSweeper(ctx context.Context, every, olderThan time.Duration, limit int) {
	go func() {
		w.sweepAllMedia(ctx, 0, limit)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.sweepAllMedia(ctx, olderThan, limit)
			}
		}
	}()
}

// TelegramBlobID is the deterministic blob key for a Telegram attachment: keyed
// by the message row, so a retried download overwrites rather than accumulates.
func TelegramBlobID(messageID uuid.UUID) string { return "tg-" + messageID.String() }

// --- Meta-channel media (WhatsApp Cloud today; Instagram/Messenger from
// Phase 4/5 share this same channel_message_media table) ------------------

// downloadMetaMedia fetches a WhatsApp Cloud attachment's bytes via the
// two-step id -> URL -> bytes resolution its Graph API requires (§10): the
// business token is resolved fresh from the store (never carried on the
// task), MediaInfo turns the webhook's opaque media id into a short-lived
// URL, and DownloadMedia fetches it — the SAME bearer token is required on
// both calls.
func (w *Worker) downloadMetaMedia(ctx context.Context, t MediaDownloadTask) error {
	if w.WACloud == nil {
		return nil
	}
	token, err := w.Store.ChannelCredentialsSecret(ctx, t.AccountID)
	if err != nil {
		return fmt.Errorf("meta media: resolve token: %w", err)
	}
	info, err := w.WACloud.MediaInfo(ctx, t.FileID, token)
	if err != nil {
		w.markChannelMediaFailed(ctx, t.MessageID, err)
		return fmt.Errorf("meta media: media info: %w", err)
	}
	data, mimetype, err := w.WACloud.DownloadMedia(ctx, info.URL, token)
	if err != nil {
		w.markChannelMediaFailed(ctx, t.MessageID, err)
		return fmt.Errorf("meta media: download: %w", err)
	}
	meta, err := w.Store.ChannelMediaMetaFor(ctx, t.MessageID)
	if err != nil {
		return err
	}
	if mimetype == "" {
		mimetype = meta.Mimetype
	}
	if _, err := w.Blob.Put(t.BlobID, data, blob.Meta{
		MediaType: meta.MediaType, Mimetype: mimetype, FileName: meta.FileName, FileSize: int64(len(data)),
	}); err != nil {
		return err
	}
	if err := w.Store.SetChannelMediaReady(ctx, t.MessageID, t.BlobID, len(data)); err != nil {
		return err
	}
	w.Log.Info("meta media downloaded", "message_id", t.MessageID, "bytes", len(data))
	w.emitMessage(ctx, "message.updated", t.MessageID)
	w.transcribeIfAudio(ctx, t.MessageID, t.AccountID, string(t.Channel), meta.MediaType, t.BlobID)
	w.rearmAfterMediaReady(ctx, t.MessageID, t.AccountID, string(t.Channel), meta.MediaType)
	return nil
}

// markChannelMediaFailed records a permanent-looking failure so the sweeper's
// backoff sees a fresh timestamp instead of hammering the same broken ref.
func (w *Worker) markChannelMediaFailed(ctx context.Context, messageID uuid.UUID, cause error) {
	if err := w.Store.SetChannelMediaFailed(ctx, messageID); err != nil {
		w.Log.Error("mark meta media failed", "message_id", messageID, "err", err)
	}
	w.Log.Warn("meta media download failed", "message_id", messageID, "err", cause)
}

// directMediaHTTPClient fetches Instagram/Messenger CDN attachment bytes —
// a bounded, package-private client (mirrors meta.Client's own 20s-timeout
// http.Client) rather than the zero-value http.DefaultClient, which has no
// timeout at all.
var directMediaHTTPClient = &http.Client{Timeout: 30 * time.Second}

// downloadDirectMedia fetches an Instagram/Messenger attachment's bytes
// directly from the CDN url the webhook payload already carried — unlike
// WhatsApp Cloud's two-step media-id -> signed-url resolution, that url is
// immediately fetchable with NO bearer token at all (Meta's CDN urls are
// themselves short-lived and self-authorizing), so there is no token to
// resolve from the store here.
func (w *Worker) downloadDirectMedia(ctx context.Context, t MediaDownloadTask) error {
	meta, err := w.Store.ChannelMediaMetaFor(ctx, t.MessageID)
	if err != nil {
		return err
	}
	if meta.SourceURL == "" {
		return fmt.Errorf("meta media: message %s has no source_url", t.MessageID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, meta.SourceURL, nil)
	if err != nil {
		return err
	}
	resp, err := directMediaHTTPClient.Do(req)
	if err != nil {
		w.markChannelMediaFailed(ctx, t.MessageID, err)
		return fmt.Errorf("meta media: direct download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		derr := fmt.Errorf("meta media: direct download: status %d", resp.StatusCode)
		w.markChannelMediaFailed(ctx, t.MessageID, derr)
		return derr
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		w.markChannelMediaFailed(ctx, t.MessageID, err)
		return fmt.Errorf("meta media: direct download: read body: %w", err)
	}
	mimetype := resp.Header.Get("Content-Type")
	if mimetype == "" {
		mimetype = meta.Mimetype
	}
	if _, err := w.Blob.Put(t.BlobID, data, blob.Meta{
		MediaType: meta.MediaType, Mimetype: mimetype, FileName: meta.FileName, FileSize: int64(len(data)),
	}); err != nil {
		return err
	}
	if err := w.Store.SetChannelMediaReady(ctx, t.MessageID, t.BlobID, len(data)); err != nil {
		return err
	}
	w.Log.Info("meta media downloaded (direct)", "message_id", t.MessageID, "bytes", len(data))
	w.emitMessage(ctx, "message.updated", t.MessageID)
	w.transcribeIfAudio(ctx, t.MessageID, t.AccountID, string(t.Channel), meta.MediaType, t.BlobID)
	w.rearmAfterMediaReady(ctx, t.MessageID, t.AccountID, string(t.Channel), meta.MediaType)
	return nil
}

// SweepChannelMedia re-enqueues Meta-channel attachments whose bytes were
// never fetched — the generic-core analogue of SweepTelegramMedia, covering
// every channel on channel_message_media through PendingChannelMedia's one
// query. Only whatsapp_cloud rows exist until Phase 4/5, and
// handleMediaDownload's own dispatch safely no-ops any other channel it
// might see.
func (w *Worker) SweepChannelMedia(ctx context.Context, olderThan time.Duration, limit int) (int, error) {
	pending, err := w.Store.PendingChannelMedia(ctx, olderThan, limit)
	if err != nil {
		return 0, err
	}
	queued := 0
	for _, p := range pending {
		if err := w.publish(ctx, queue.Message{Kind: queue.KindMediaDownload, Payload: MediaDownloadTask{
			MessageID: p.MessageID, Channel: messaging.Channel(p.Channel), AccountID: p.AccountID,
			FileID: p.ProviderRef, BlobID: ChannelBlobID(p.MessageID),
		}}); err != nil {
			return queued, err
		}
		queued++
	}
	if queued > 0 {
		w.Log.Info("meta channel media sweep", "queued", queued)
	}
	return queued, nil
}

// ChannelBlobID is the deterministic blob key for a Meta-channel attachment —
// see TelegramBlobID's identical reasoning.
func ChannelBlobID(messageID uuid.UUID) string { return "meta-" + messageID.String() }

// --- outbound sends -------------------------------------------------------

// handleOutboundSend routes EVERY send — text and media alike — through the
// channel-neutral sender registry (Worker.Senders), so WhatsApp, simulator
// and Telegram conversations share one outbound path. The actual delivery
// logic lives in internal/outbound.Deliver — internal/campaign's Runner
// calls that same function directly for a campaign send, so there is
// exactly one place that resolves a sender, hydrates media, and interprets
// a ChannelSender's result.
func (w *Worker) handleOutboundSend(ctx context.Context, t OutboundTask) error {
	_, err := outbound.Deliver(ctx, outbound.Deps{
		Store: w.Store, Blob: w.Blob, Hub: w.Hub, Senders: w.Senders, Log: w.Log,
	}, outbound.Task{
		MessageID: t.MessageID, AccountID: t.AccountID, Channel: t.Channel,
		Destination: t.Destination, Text: t.Text, MediaID: t.MediaID, Caption: t.Caption,
	})
	return err
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

// --- audio transcription (STT) ---------------------------------------------

// transcribeIfAudio is a thin TranscribeAudio wrapper over w's own
// dependencies — see TranscribeAudio for the actual logic.
func (w *Worker) transcribeIfAudio(ctx context.Context, messageID, accountID uuid.UUID, channel, mediaType, blobID string) {
	TranscribeAudio(ctx, TranscriptionDeps{
		Store: w.Store, Blob: w.Blob, Hub: w.Hub, Response: w.Response,
		Queue: w.Queue, STT: w.STT, Automation: w.Automation, Log: w.Log,
	}, messageID, accountID, channel, mediaType, blobID)
}

// TranscriptionDeps groups the dependencies TranscribeAudio needs. It exists
// as its own type, separate from *Worker, so internal/whatsmeow's own
// media-download completion path — which has no queue task of its own (see
// this package's doc comment: "WhatsApp's own inbound ingestion and media
// download live entirely inside internal/whatsmeow") — can call the exact
// same transcription logic Telegram/Meta channels use instead of
// duplicating it, by building one of these from its own fields.
type TranscriptionDeps struct {
	Store *store.Store
	Blob  blob.Store
	Hub   *realtime.Hub
	// Response resolves the organization's knowledge base for vocabulary
	// priming (see stt.BuildPrompt). nil skips priming, never transcription.
	Response *response.Service
	// Queue is the fallback path's enqueue target when Automation is nil.
	Queue queue.Queue
	STT   func(ctx context.Context) stt.Params
	// Automation, when set, re-arms a chat's debounce/version-gated
	// generation instead of a direct KindAIDraft enqueue — see
	// TranscribeAudio's own doc comment.
	Automation Automation
	Log        *slog.Logger
}

// TranscribeAudio runs the speech-to-text step for one attachment right
// after its bytes finish downloading (see the "Emit message.updated" call
// in each of Worker's three download functions above, and
// internal/whatsmeow.Manager.downloadAndAttachMedia's own call, right
// before this one runs in each). It is a best-effort enhancement, never a
// reason to fail a media download that has already succeeded: an
// unconfigured deps.STT, a Transcribe error, or anything but an audio
// attachment is logged (where relevant) and silently skipped rather than
// returned as an error.
//
// The transcript is persisted exactly once — store.UpdateMediaTranscript is
// the durable "already done" marker (see its own doc comment) — and this
// function itself checks it before calling Transcribe, so a second call for
// an attachment that already has one (today: only a hand-crafted call from
// a test; internal/queue's in-memory driver has no redelivery, and
// PendingTelegramMedia/PendingChannelMedia both exclude 'ready' rows — see
// their own WHERE clauses — so a legitimate redelivery cannot reach here
// twice yet, but a future queue driver, per internal/queue's own doc
// comment, might) never re-runs the paid STT call or re-enqueues a draft.
// Once persisted, this also refreshes the chat list's preview (audio's own
// placeholder, "🎙 Аудио", becomes the transcribed words) and hands the chat
// to Automation (or, with none configured, enqueues a direct AI draft) —
// the very first draft attempt, fired by the ingest webhook the moment the
// message arrived, ran with no transcript at all, so the customer's actual
// words deserve their own generation.
func TranscribeAudio(ctx context.Context, deps TranscriptionDeps, messageID, accountID uuid.UUID, channel, mediaType, blobID string) {
	if mediaType != "audio" || deps.STT == nil {
		return
	}
	params := deps.STT(ctx)
	if params.Transcriber == nil {
		return
	}

	if existing, err := deps.Store.MessageByID(ctx, messageID); err == nil {
		for _, md := range existing.Media {
			if md.MediaType == "audio" && md.Transcript != "" {
				return
			}
		}
	}

	data, meta, err := deps.Blob.Get(blobID)
	if err != nil {
		deps.Log.Error("transcribe: read blob", "message_id", messageID, "err", err)
		return
	}
	if int64(len(data)) > stt.MaxAudioBytes {
		deps.Log.Warn("transcribe: audio exceeds the transcription provider's size limit, skipping",
			"message_id", messageID, "bytes", len(data))
		return
	}

	// Vocabulary priming is best-effort: a KB load failure (or no
	// organization/KnowledgeBase at all) just falls back to the operator's
	// own custom vocabulary alone, never blocks transcription.
	prompt := params.Vocabulary
	if deps.Response != nil && deps.Response.KnowledgeBase != nil {
		if account, err := deps.Store.AccountByID(ctx, accountID); err == nil && account.OrganizationID.Valid {
			if kb, err := deps.Response.KnowledgeBase.Load(ctx, account.OrganizationID.UUID.String()); err == nil {
				prompt = stt.BuildPrompt(kb, params.Vocabulary)
			}
		}
	}

	text, err := params.Transcriber.Transcribe(ctx, data, meta.FileName, meta.Mimetype, stt.TranscribeOptions{
		Language: params.Language, Prompt: prompt,
	})
	if err != nil {
		deps.Log.Warn("audio transcription failed", "message_id", messageID, "err", err)
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	if err := deps.Store.UpdateMediaTranscript(ctx, channel, messageID, text); err != nil {
		deps.Log.Error("persist transcript", "message_id", messageID, "err", err)
		return
	}
	msg, err := deps.Store.MessageByID(ctx, messageID)
	if err != nil {
		deps.Log.Error("reload transcribed message", "message_id", messageID, "err", err)
		return
	}
	deps.Hub.Broadcast("message.updated", dto.MapMessage(msg))

	if err := deps.Store.UpdateChatPreviewIfCurrent(ctx, msg.ChatID, msg.MessageTS, TranscriptPreview(text)); err != nil {
		deps.Log.Error("update chat preview after transcription", "chat_id", msg.ChatID, "err", err)
	} else if chat, err := deps.Store.ChatByID(ctx, msg.ChatID); err == nil {
		deps.Hub.Broadcast("chat.updated", dto.MapChat(chat))
	}

	// A transcript becoming available is new customer content, exactly like
	// a fresh inbound message: re-arming Automation gives it the SAME
	// version-gated generation and scheduled_auto auto-send eligibility a
	// customer's own follow-up text would get (see
	// automation.Scheduler.OnInboundMessage), and correctly no-ops in "off"
	// mode instead of drafting regardless of the account's automation
	// setting. A nil Automation (a wiring with none configured, or a test)
	// falls back to the original direct KindAIDraft enqueue — the exact
	// optional-dependency shape internal/tgingest.Processor's own Automation
	// field already uses.
	rearmOrEnqueue(ctx, deps, msg.ChatID, accountID, channel, "automation: re-arm after transcription failed", "enqueue AI draft after transcription")
	deps.Log.Info("audio transcribed", "message_id", messageID, "chars", len(text))
}

// RearmAfterMediaReady hands messageID's chat to Automation (or, with none
// configured, enqueues a direct AI draft) once an IMAGE attachment finishes
// downloading — the same problem TranscribeAudio's own post-transcription
// re-arm solves for audio, but firing right after download instead of after
// an extra step, since an image's bytes are themselves what
// responsestore.resolveAttachments needs to embed it as vision input (there
// is no equivalent to a transcript to wait for). Without this, a debounce
// that fires (or a direct draft that generates) before a slow download
// finishes produces a response with no attachment at all, and — unlike
// audio, which always gets a second, transcript-bearing attempt — nothing
// would ever re-run Generate once the image became ready.
//
// Deliberately scoped to "image" alone, not every non-audio media type:
// resolveAttachments only ever acts on an image attachment today, so
// re-arming for a video/document/sticker download would only cost an extra
// LLM call for a request nothing downstream reads any differently.
func RearmAfterMediaReady(ctx context.Context, deps TranscriptionDeps, messageID, accountID uuid.UUID, channel, mediaType string) {
	if mediaType != "image" {
		return
	}
	msg, err := deps.Store.MessageByID(ctx, messageID)
	if err != nil {
		deps.Log.Error("rearm after media ready: reload message", "message_id", messageID, "err", err)
		return
	}
	rearmOrEnqueue(ctx, deps, msg.ChatID, accountID, channel, "automation: re-arm after media ready failed", "enqueue AI draft after media ready")
}

// rearmOrEnqueue is TranscribeAudio's and RearmAfterMediaReady's shared
// "tell automation about new content, or fall back to a raw enqueue"
// tail — see TranscribeAudio's own doc comment for why Automation is
// preferred whenever it is configured.
func rearmOrEnqueue(ctx context.Context, deps TranscriptionDeps, chatID, accountID uuid.UUID, channel, automationErrMsg, enqueueErrMsg string) {
	if deps.Automation != nil {
		if err := deps.Automation.OnInboundMessage(ctx, chatID, accountID, channel, time.Now()); err != nil {
			deps.Log.Error(automationErrMsg, "chat_id", chatID, "err", err)
		}
		return
	}
	if deps.Queue == nil {
		return
	}
	pctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()
	if err := deps.Queue.Publish(pctx, queue.Message{Kind: queue.KindAIDraft, Payload: AIDraftTask{ChatID: chatID}}); err != nil {
		deps.Log.Error(enqueueErrMsg, "chat_id", chatID, "err", err)
	}
}

// rearmAfterMediaReady is a thin RearmAfterMediaReady wrapper over w's own
// dependencies — see transcribeIfAudio's identical shape.
func (w *Worker) rearmAfterMediaReady(ctx context.Context, messageID, accountID uuid.UUID, channel, mediaType string) {
	RearmAfterMediaReady(ctx, TranscriptionDeps{
		Store: w.Store, Blob: w.Blob, Hub: w.Hub, Response: w.Response,
		Queue: w.Queue, STT: w.STT, Automation: w.Automation, Log: w.Log,
	}, messageID, accountID, channel, mediaType)
}

// TranscriptPreview mirrors previewFor's/Preview's own audio placeholder
// ("🎙 Аудио" — see internal/whatsmeow/manager.go and
// internal/tgingest/mapping.go) now that the actual words are known, capped
// the same way whatsmeow's previewFor already caps a text message (120
// runes) rather than a fresh, possibly very different, truncation rule.
// Exported so internal/httpapi's manual re-transcribe endpoint renders the
// exact same chat-list preview text this automatic path does.
func TranscriptPreview(text string) string {
	r := []rune(text)
	if len(r) > 120 {
		text = string(r[:120])
	}
	return "🎙 " + text
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
