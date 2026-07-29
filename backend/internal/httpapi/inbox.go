package httpapi

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

func (s *Server) handleListChats(c *gin.Context) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	limit, offset, pageNum, pageSize := s.pageParams(c)
	f := store.ChatFilter{
		OrgID:    org.ID,
		Status:   c.Query("status"),
		Assignee: c.Query("assignee"),
		MeUserID: currentUser(c).ID,
		Query:    c.Query("q"),
		Limit:    limit,
		Offset:   offset,
	}
	// account_id is the neutral filter; wa_account_id is kept as a deprecated
	// alias so an older client keeps working through the transition.
	raw, param := c.Query("account_id"), "account_id"
	if raw == "" {
		raw, param = c.Query("wa_account_id"), "wa_account_id"
	}
	if raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid "+param)
			return
		}
		f.AccountID = uuid.NullUUID{UUID: id, Valid: true}
	}
	chats, total, err := s.store.ListChatsForOrg(ctx(c), f)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	items := make([]dto.Chat, 0, len(chats))
	for _, ch := range chats {
		items = append(items, dto.MapChat(ch))
	}
	ok(c, page{Items: items, Page: pageNum, PageSize: pageSize, Total: total})
}

// orgChat resolves a chat param and enforces org membership; fails 404 otherwise.
// It is the guard behind every chat-scoped endpoint (messages, send, read, drafts).
func (s *Server) orgChat(c *gin.Context, chatID uuid.UUID) (store.Chat, bool) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return store.Chat{}, false
	}
	chat, err := s.store.ChatByIDForOrg(ctx(c), chatID, org.ID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "chat not found")
		return store.Chat{}, false
	}
	return chat, true
}

func (s *Server) handleListMessages(c *gin.Context) {
	chatID, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	if _, ok := s.orgChat(c, chatID); !ok {
		return
	}
	var before time.Time
	if b := c.Query("before"); b != "" {
		if t, err := time.Parse(time.RFC3339, b); err == nil {
			before = t
		}
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	msgs, next, err := s.store.MessagesForChat(ctx(c), chatID, before, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	items := make([]dto.Message, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, dto.MapMessage(m))
	}
	var nextBefore *string
	if next != nil {
		s := next.UTC().Format(time.RFC3339Nano)
		nextBefore = &s
	}
	ok(c, gin.H{"items": items, "next_before": nextBefore})
}

type sendReq struct {
	Text     string   `json:"text"`
	MediaIDs []string `json:"media_ids"`
}

func (s *Server) handleSendMessage(c *gin.Context) {
	chatID, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req sendReq
	_ = c.ShouldBindJSON(&req)
	if strings.TrimSpace(req.Text) == "" && len(req.MediaIDs) == 0 {
		fail(c, http.StatusBadRequest, ErrValidation, "text or media_ids required")
		return
	}
	chat, okChat := s.orgChat(c, chatID)
	if !okChat {
		return
	}
	u := currentUser(c)
	items, err := s.sendParts(c, chat, "user", uuid.NullUUID{UUID: u.ID, Valid: true}, req.Text, req.MediaIDs)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	ok(c, gin.H{"items": items})
}

// sendParts is the fan-out: text is one message, each media_id is its own
// message (one Evolution call each). It inserts the row, broadcasts
// message.created, then enqueues the outbound_send task — shared by manual send
// (sender_kind=user) and AI approve (sender_kind=ai).
func (s *Server) sendParts(c *gin.Context, chat store.Chat, senderKind string, senderUserID uuid.NullUUID, text string, mediaIDs []string) ([]dto.Message, error) {
	var out []dto.Message
	// Resolve the chat's account once → its Evolution instance, so the send goes
	// out from the right number (multi-account). Empty instance falls back to the
	// client's default instance in the worker. The channel comes from the chat
	// itself (the view already carries it), so it is never guessed.
	channel := chat.Channel
	instance := ""
	if acct, err := s.store.AccountByID(ctx(c), chat.AccountID); err == nil {
		instance = acct.InstanceName
		if channel == "" {
			channel = acct.Channel
		}
	} else {
		// No account row → the worker falls back to the default Evolution instance.
		// If that default is wrong, sends go nowhere; make the fallback visible.
		s.log.Warn("send: account lookup failed; using default instance",
			"chat_id", chat.ID, "account_id", chat.AccountID, "err", err)
	}

	text = strings.TrimSpace(text)

	// sendText emits a standalone text message (no media, or no media ref resolved).
	sendText := func() error {
		msgID, err := s.store.InsertOutbound(ctx(c), channel, chat.ID, chat.AccountID, senderKind, senderUserID, "conversation", text, preview(text))
		if err != nil {
			return err
		}
		msg, _ := s.store.MessageByID(ctx(c), msgID)
		s.hub.Broadcast("message.created", dto.MapMessage(msg))
		s.publishOrLog(ctx(c), queue.Message{Kind: queue.KindOutboundSend, Payload: worker.OutboundTask{
			MessageID: msgID, AccountID: chat.AccountID, Channel: messaging.Channel(channel),
			Instance: instance, PhoneJID: chat.ExternalConversationRef, Text: text,
		}})
		s.log.Info("send queued", "chat_id", chat.ID, "message_id", msgID, "channel", channel,
			"instance", instance, "sender_kind", senderKind, "kind", "text")
		out = append(out, dto.MapMessage(msg))
		return nil
	}

	if len(mediaIDs) == 0 {
		if text == "" {
			return out, nil
		}
		return out, sendText()
	}

	// With media: the first resolvable asset carries the text as its caption, so the
	// customer (and the inbox) see ONE message — image + caption — instead of a text
	// bubble followed by an empty media bubble. Extra assets follow captionless.
	textPlaced := false
	for _, mid := range mediaIDs {
		meta, found := s.blob.Meta(mid)
		if !found {
			continue
		}
		caption := ""
		if !textPlaced {
			caption = text // "" for a media-only send — fine
			textPlaced = true
		}
		prev := placeholderFor(meta.MediaType)
		if caption != "" {
			prev = preview(caption)
		}
		kind := mediaMessageKind(meta.MediaType)
		msgID, err := s.store.InsertOutbound(ctx(c), channel, chat.ID, chat.AccountID, senderKind, senderUserID, kind, caption, prev)
		if err != nil {
			return nil, err
		}
		if err := s.store.UpsertOutboundMedia(ctx(c), channel, msgID, store.MediaRef{
			MediaType: meta.MediaType, Mimetype: meta.Mimetype, FileName: meta.FileName, FileSize: int(meta.FileSize),
		}, mid); err != nil {
			return nil, err
		}
		msg, _ := s.store.MessageByID(ctx(c), msgID)
		s.hub.Broadcast("message.created", dto.MapMessage(msg))
		s.publishOrLog(ctx(c), queue.Message{Kind: queue.KindOutboundSend, Payload: worker.OutboundTask{
			MessageID: msgID, AccountID: chat.AccountID, Channel: messaging.Channel(channel),
			Instance: instance, PhoneJID: chat.ExternalConversationRef, MediaID: mid, Caption: caption,
		}})
		s.log.Info("send queued", "chat_id", chat.ID, "message_id", msgID, "instance", instance,
			"sender_kind", senderKind, "kind", kind, "media_id", mid)
		out = append(out, dto.MapMessage(msg))
	}

	// No media ref resolved (all unknown) but we still have text — don't drop it.
	if !textPlaced && text != "" {
		return out, sendText()
	}
	return out, nil
}

// selectComposeAccount picks the "from" account for a new conversation: the
// explicit account_id if given (and owned by the org), else the sole account
// when exactly one is connected. With zero or many it returns an error so the UI
// shows the "from number" picker (multiple) or prompts to connect one (none).
//
// It deliberately lists only the wa_* gateway's accounts: a Telegram bot cannot
// start a conversation (Telegram requires the user to message the bot first), so
// it must never be offered as a "from" here.
func (s *Server) selectComposeAccount(c *gin.Context, rawID string) (store.Account, bool) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return store.Account{}, false
	}
	accts, err := s.store.ListWaAccountsForOrg(ctx(c), org.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return store.Account{}, false
	}
	if rawID != "" {
		id, perr := uuid.Parse(rawID)
		if perr != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid account_id")
			return store.Account{}, false
		}
		for _, a := range accts {
			if a.ID == id {
				return a, true
			}
		}
		fail(c, http.StatusBadRequest, ErrValidation, "account_id not found in your WhatsApp accounts")
		return store.Account{}, false
	}
	switch len(accts) {
	case 1:
		return accts[0], true
	case 0:
		fail(c, http.StatusConflict, ErrAccountNotConnected, "no connected WhatsApp account")
		return store.Account{}, false
	default:
		fail(c, http.StatusBadRequest, ErrValidation, "account_id required (multiple accounts connected)")
		return store.Account{}, false
	}
}

type composeReq struct {
	Phone     string   `json:"phone"`
	Text      string   `json:"text"`
	MediaIDs  []string `json:"media_ids"`
	AccountID string   `json:"account_id"`
	// WaAccountID is the deprecated alias for AccountID, still accepted so an
	// older client keeps working through the transition.
	WaAccountID string `json:"wa_account_id"`
}

// accountID resolves the compose "from" account, preferring the neutral field.
func (r composeReq) accountID() string {
	if r.AccountID != "" {
		return r.AccountID
	}
	return r.WaAccountID
}

// handleCreateChat starts (or reuses) a conversation by phone number and sends the
// first message — the "compose to a new number" entry point. It picks the sending
// account (the "from number"), find-or-creates the contact+chat under it, then
// reuses sendParts so the outbound path is identical to replying in an existing chat.
func (s *Server) handleCreateChat(c *gin.Context) {
	var req composeReq
	_ = c.ShouldBindJSON(&req)

	phone := digitsOnly(req.Phone)
	if len(phone) < 7 || len(phone) > 15 {
		fail(c, http.StatusBadRequest, ErrValidation, "phone must be 7–15 digits (country code + number)")
		return
	}
	if strings.TrimSpace(req.Text) == "" && len(req.MediaIDs) == 0 {
		fail(c, http.StatusBadRequest, ErrValidation, "text or media_ids required")
		return
	}
	acct, okAcct := s.selectComposeAccount(c, req.accountID())
	if !okAcct {
		return
	}

	// Pre-flight: fail fast with a clear reason if the number isn't on WhatsApp,
	// instead of creating a chat whose first message silently fails to send.
	if s.evo != nil {
		exists, err := s.evo.OnWhatsApp(ctx(c), acct.InstanceName, phone)
		if err != nil {
			fail(c, http.StatusBadGateway, ErrEvolution, "could not verify the number: "+err.Error())
			return
		}
		if !exists {
			fail(c, http.StatusBadRequest, ErrNotOnWhatsApp,
				"Этот номер не зарегистрирован в WhatsApp. Укажите номер с кодом страны, например 77001234567.")
			return
		}
	}
	jid := config.CanonicalJID(phone) // phone -> phone@s.whatsapp.net

	chatID, _, err := s.store.FindOrCreateChat(ctx(c), acct.ID, jid, phone)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	chat, err := s.store.ChatByID(ctx(c), chatID)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	s.hub.Broadcast("chat.updated", dto.MapChat(chat))

	u := currentUser(c)
	items, err := s.sendParts(c, chat, "user", uuid.NullUUID{UUID: u.ID, Valid: true}, req.Text, req.MediaIDs)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return
	}
	created(c, gin.H{"chat": dto.MapChat(chat), "items": items})
}

func (s *Server) handleReadChat(c *gin.Context) {
	chatID, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	if _, ok := s.orgChat(c, chatID); !ok {
		return
	}
	chat, err := s.store.MarkChatRead(ctx(c), chatID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "chat not found")
		return
	}
	s.hub.Broadcast("chat.updated", dto.MapChat(chat))
	ok(c, gin.H{"unread_count": 0})
}

func (s *Server) handleUploadMedia(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "file part required")
		return
	}
	f, err := fh.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "cannot open file")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "cannot read file")
		return
	}
	mediaType, mimetype := detectMedia(fh.Filename, fh.Header.Get("Content-Type"))
	mid := uuid.NewString()
	if _, err := s.blob.Put(mid, data, blob.Meta{MediaType: mediaType, Mimetype: mimetype, FileName: fh.Filename, FileSize: int64(len(data))}); err != nil {
		fail(c, http.StatusBadGateway, ErrMediaUnavailable, "store failed")
		return
	}
	ok(c, gin.H{
		"media_id": mid, "media_type": mediaType, "mimetype": mimetype,
		"file_name": fh.Filename, "file_size": len(data),
		"url": "/xchats/api/v1/media/" + mid,
	})
}

func (s *Server) handleServeMedia(c *gin.Context) {
	raw := c.Param("id")
	// A public media id is usually a message_media.id (uuid). Parse leniently —
	// pending uploads and stub samples are keyed directly by their blob id.
	if id, err := uuid.Parse(raw); err == nil {
		if storageURL, mimetype, _, merr := s.store.MediaStorageURL(ctx(c), id); merr == nil {
			data, meta, gerr := s.blob.Get(storageURL)
			if gerr != nil {
				fail(c, http.StatusBadGateway, ErrMediaUnavailable, "blob missing")
				return
			}
			ct := mimetype
			if ct == "" {
				ct = meta.Mimetype
			}
			c.Data(http.StatusOK, ctOrDefault(ct), data)
			return
		} else if !errors.Is(merr, store.ErrNotFound) {
			fail(c, http.StatusInternalServerError, ErrInternal, merr.Error())
			return
		}
	}
	// fall back: a pending upload or a stub sample, keyed directly by the blob id.
	if data, meta, err := s.blob.Get(raw); err == nil {
		c.Data(http.StatusOK, ctOrDefault(meta.Mimetype), data)
		return
	}
	fail(c, http.StatusNotFound, ErrNotFound, "media not found")
}

// --- small helpers --------------------------------------------------------

// digitsOnly keeps only ASCII digits, so "+7 (702) 976-65-09" -> "77029766509".
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteByte(byte(r))
		}
	}
	return b.String()
}

func preview(text string) string {
	if len([]rune(text)) > 120 {
		return string([]rune(text)[:120])
	}
	return text
}

func placeholderFor(mediaType string) string {
	switch mediaType {
	case "image":
		return "📷 Фото"
	case "video":
		return "🎥 Видео"
	case "audio":
		return "🎙 Аудио"
	default:
		return "📄 Документ"
	}
}

func mediaMessageKind(mediaType string) string {
	switch mediaType {
	case "image":
		return "imageMessage"
	case "video":
		return "videoMessage"
	case "audio":
		return "audioMessage"
	default:
		return "documentMessage"
	}
}

func detectMedia(fileName, contentType string) (mediaType, mimetype string) {
	mimetype = contentType
	if mimetype == "" || mimetype == "application/octet-stream" {
		if m := mime.TypeByExtension(strings.ToLower(filepath.Ext(fileName))); m != "" {
			mimetype = m
		}
	}
	switch {
	case strings.HasPrefix(mimetype, "image/"):
		mediaType = "image"
	case strings.HasPrefix(mimetype, "video/"):
		mediaType = "video"
	case strings.HasPrefix(mimetype, "audio/"):
		mediaType = "audio"
	default:
		mediaType = "document"
	}
	if mimetype == "" {
		mimetype = "application/octet-stream"
	}
	return
}

func ctOrDefault(ct string) string {
	if ct == "" {
		return "application/octet-stream"
	}
	return ct
}
