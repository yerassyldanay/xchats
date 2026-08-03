package httpapi

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// telegramSecretHeader is the header Telegram echoes back the secret_token in.
const telegramSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

// handleTelegramWebhook is the Telegram ingress. Unlike the Evolution webhook it
// does NOT enqueue-and-ack: the normalized rows are committed BEFORE the 200.
//
// That is the entire at-least-once story. Telegram redelivers any update it
// does not get a 2xx for, so:
//   - a DB failure answers 500 and the update comes back;
//   - a crash after the commit but before the ack also comes back, and lands on
//     UNIQUE(account_id, telegram_update_id) as a no-op;
//   - anything we correctly cannot store (a group chat, a service message, an
//     unknown account) answers 200, because a non-2xx would have Telegram retry
//     it forever.
//
// It is why there is no inbound_events table: the provider already owns the
// retry buffer, and duplicating it here would only add a second thing to lose.
func (s *Server) handleTelegramWebhook(c *gin.Context) {
	rawID := c.Param("account_id")

	// Secret check first: an attacker who guesses the URL must not be able to
	// probe which account ids exist.
	if secret := s.cfg.TelegramResolvedWebhookSecret(); secret != "" {
		if subtle.ConstantTimeCompare([]byte(c.GetHeader(telegramSecretHeader)), []byte(secret)) != 1 {
			s.log.Warn("telegram webhook auth rejected", "account_id", rawID, "reason", "bad secret token")
			fail(c, http.StatusUnauthorized, ErrWebhookUnauthorized, "bad webhook secret")
			return
		}
	}

	accountID, err := uuid.Parse(rawID)
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid account id")
		return
	}

	acct, err := s.store.TgAccountByID(ctx(c), accountID)
	if err != nil {
		// Unknown or already-disconnected bot: Telegram is still pointed at us
		// from an old registration. Ack so it stops, and store nothing.
		s.log.Info("telegram webhook discarded", "account_id", accountID, "result", "unknown_account")
		c.Status(http.StatusOK)
		return
	}

	raw, err := c.GetRawData()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "unreadable body")
		return
	}
	update, err := telegram.ParseUpdate(raw)
	if err != nil {
		// Malformed body: retrying it would produce the same parse failure, so
		// ack and log rather than looping.
		s.log.Warn("telegram webhook unparsable update", "account_id", accountID, "err", err)
		c.Status(http.StatusOK)
		return
	}
	if reason, ignorable := update.Ignorable(); ignorable {
		s.log.Info("telegram update dropped", "account_id", accountID, "result", reason)
		c.Status(http.StatusOK)
		return
	}

	msg := update.Message
	in := store.TgInbound{
		AccountID: accountID,
		UpdateID:  update.UpdateID,

		TelegramChatID: msg.Chat.ID,
		ChatType:       msg.Chat.Type,

		TelegramUserID: msg.From.ID,
		Username:       msg.From.Username,
		FirstName:      msg.From.FirstName,
		LastName:       msg.From.LastName,
		DisplayName:    msg.From.DisplayName(),

		TelegramMessageID: msg.MessageID,
		MessageKind:       telegramMessageKind(msg),
		// A caption IS the message body: a photo with "сколько стоит?" under it
		// must reach the AI as that question, not as an empty media bubble.
		Body:      firstNonEmpty(msg.Text, msg.Caption),
		Preview:   telegramPreview(msg),
		Raw:       raw,
		MessageTS: msg.Timestamp(),
	}
	if f := msg.Attachment(); f != nil {
		in.Media = &store.TgMedia{
			FileID: f.FileID, FileUniqueID: f.FileUniqueID, MediaType: f.Kind,
			Mimetype: f.Mimetype, FileName: f.FileName, Size: f.Size,
		}
	}

	res, err := s.store.IngestTelegramInbound(ctx(c), in)
	if err != nil {
		// 500 — do NOT ack. Telegram redelivers, and the unique keys make the
		// replay a no-op once the database is healthy again.
		s.log.Error("telegram ingest failed", "account_id", accountID, "update_id", update.UpdateID, "err", err)
		fail(c, http.StatusInternalServerError, ErrInternal, "ingest failed")
		return
	}
	_ = s.store.TouchTelegramAccount(ctx(c), accountID)

	if !res.MessageInserted {
		// A redelivery. The row is already there, so the ingest is settled — but
		// the FOLLOW-UP may not be: the first attempt could have died between the
		// commit and the enqueue. If no draft exists for this trigger, publish
		// again. WriteDraftSet's supersede semantics make a rare double
		// generation harmless.
		s.log.Info("telegram update processed", "account_id", accountID,
			"update_id", update.UpdateID, "result", "duplicate")
		s.emitTelegramMessage(c, "message.updated", res.MessageID)
		if !s.reenqueueMissingDraft(c, res) {
			return
		}
		c.Status(http.StatusOK)
		return
	}

	s.log.Info("telegram update processed", "account_id", accountID,
		"update_id", update.UpdateID, "chat_id", res.ChatID, "result", "new",
		"bot", acct.BotUsername)
	s.emitTelegramMessage(c, "message.created", res.MessageID)

	// Attachment bytes are fetched off the request path. A lost enqueue is not
	// fatal — the media row stays 'pending' and the sweeper picks it up — so
	// unlike the draft below this one never blocks the ack.
	if in.Media != nil {
		s.publishOrLog(ctx(c), queue.Message{Kind: queue.KindMediaDownload, Payload: worker.MediaDownloadTask{
			MessageID: res.MessageID, Channel: messaging.ChannelTelegram, AccountID: accountID,
			FileID: in.Media.FileID, BlobID: worker.TelegramBlobID(res.MessageID),
		}})
	}
	if chat, cerr := s.store.ChatByID(ctx(c), res.ChatID); cerr == nil {
		name := "chat.updated"
		if res.ChatCreated {
			name = "chat.created"
		}
		s.hub.Broadcast(name, dto.MapChat(chat))
	}

	// The debounce enqueue is part of the durable unit as far as the caller is
	// concerned: a queue that will not accept the task means this delivery has
	// NOT been fully handled, so refuse to ack and let Telegram bring it back.
	if err := s.publish(ctx(c), queue.Message{
		Kind: queue.KindDebounceTouch, Payload: worker.DebounceTouchTask{
			ChatID: res.ChatID, Channel: "telegram", TriggerMessageID: res.MessageID,
		},
	}); err != nil {
		s.log.Error("telegram debounce enqueue failed; not acking", "chat_id", res.ChatID, "err", err)
		fail(c, http.StatusInternalServerError, ErrInternal, "enqueue failed")
		return
	}
	c.Status(http.StatusOK)
}

// reenqueueMissingDraft re-touches the debounce for a redelivered inbound
// message that never produced a draft. It reports whether the request may be
// acked.
func (s *Server) reenqueueMissingDraft(c *gin.Context, res store.TgInboundResult) bool {
	has, err := s.store.HasDraftForTrigger(ctx(c), res.MessageID)
	if err != nil {
		s.log.Warn("telegram duplicate: draft lookup failed", "message_id", res.MessageID, "err", err)
		return true // the message itself is stored; do not make Telegram retry for this
	}
	if has {
		return true
	}
	if err := s.publish(ctx(c), queue.Message{
		Kind: queue.KindDebounceTouch, Payload: worker.DebounceTouchTask{
			ChatID: res.ChatID, Channel: "telegram", TriggerMessageID: res.MessageID,
		},
	}); err != nil {
		s.log.Error("telegram duplicate: debounce enqueue failed; not acking", "chat_id", res.ChatID, "err", err)
		fail(c, http.StatusInternalServerError, ErrInternal, "enqueue failed")
		return false
	}
	s.log.Info("telegram duplicate: re-touched the missing draft's debounce", "chat_id", res.ChatID)
	return true
}

func (s *Server) emitTelegramMessage(c *gin.Context, name string, id uuid.UUID) {
	msg, err := s.store.MessageByID(ctx(c), id)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.log.Error("emit telegram message", "err", err)
		}
		return
	}
	s.hub.Broadcast(name, dto.MapMessage(msg))
}

// telegramMessageKind maps a Telegram message to our message_kind vocabulary,
// mirroring the WhatsApp names so the inbox renders both the same way.
func telegramMessageKind(m *telegram.Message) string {
	switch m.MediaKind() {
	case "image":
		if m.Sticker != nil {
			return "stickerMessage"
		}
		return "imageMessage"
	case "video":
		return "videoMessage"
	case "audio":
		return "audioMessage"
	case "document":
		return "documentMessage"
	case "sticker":
		return "stickerMessage"
	}
	return "conversation"
}

// telegramPreview is the chat-list line. Text (or a caption) wins; a
// caption-less attachment falls back to the same placeholders WhatsApp uses.
func telegramPreview(m *telegram.Message) string {
	if body := firstNonEmpty(m.Text, m.Caption); body != "" {
		return preview(body)
	}
	switch m.MediaKind() {
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
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
