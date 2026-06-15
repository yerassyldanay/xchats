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
)

func (s *Server) handleListChats(c *gin.Context) {
	limit, offset, pageNum, pageSize := s.pageParams(c)
	f := store.ChatFilter{
		Status:   c.Query("status"),
		Assignee: c.Query("assignee"),
		MeUserID: currentUser(c).ID,
		Query:    c.Query("q"),
		Limit:    limit,
		Offset:   offset,
	}
	chats, total, err := s.store.ListChats(ctx(c), s.accountID, f)
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

func (s *Server) handleListMessages(c *gin.Context) {
	chatID, okID := parseUUID(c, "id")
	if !okID {
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
	chat, err := s.store.ChatByID(ctx(c), chatID)
	if err != nil {
		fail(c, http.StatusNotFound, ErrNotFound, "chat not found")
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

	if strings.TrimSpace(text) != "" {
		msgID, err := s.store.InsertOutboundMessage(ctx(c), chat.ID, chat.AccountID, senderKind, senderUserID, "conversation", text, preview(text))
		if err != nil {
			return nil, err
		}
		msg, _ := s.store.MessageByID(ctx(c), msgID)
		s.hub.Broadcast("message.created", dto.MapMessage(msg))
		_ = s.queue.Publish(queue.Message{Kind: queue.KindOutboundSend, Payload: worker.OutboundTask{
			MessageID: msgID, AccountID: chat.AccountID, PhoneJID: chat.RemoteJID, Text: text,
		}})
		out = append(out, dto.MapMessage(msg))
	}

	for _, mid := range mediaIDs {
		meta, found := s.blob.Meta(mid)
		if !found {
			continue
		}
		kind := mediaMessageKind(meta.MediaType)
		msgID, err := s.store.InsertOutboundMessage(ctx(c), chat.ID, chat.AccountID, senderKind, senderUserID, kind, "", placeholderFor(meta.MediaType))
		if err != nil {
			return nil, err
		}
		_, _, _ = s.store.UpsertMessageMedia(ctx(c), msgID, store.MediaRef{
			MediaType: meta.MediaType, Mimetype: meta.Mimetype, FileName: meta.FileName, FileSize: int(meta.FileSize),
		}, mid, "ready")
		msg, _ := s.store.MessageByID(ctx(c), msgID)
		s.hub.Broadcast("message.created", dto.MapMessage(msg))
		_ = s.queue.Publish(queue.Message{Kind: queue.KindOutboundSend, Payload: worker.OutboundTask{
			MessageID: msgID, AccountID: chat.AccountID, PhoneJID: chat.RemoteJID, MediaID: mid,
		}})
		out = append(out, dto.MapMessage(msg))
	}
	return out, nil
}

type composeReq struct {
	Phone    string   `json:"phone"`
	Text     string   `json:"text"`
	MediaIDs []string `json:"media_ids"`
}

// handleCreateChat starts (or reuses) a conversation by phone number and sends the
// first message — the "compose to a new number" entry point. It find-or-creates the
// contact+chat, then reuses sendParts so the outbound text/media path is identical
// to replying inside an existing chat.
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

	// Pre-flight: fail fast with a clear reason if the number isn't on WhatsApp,
	// instead of creating a chat whose first message silently fails to send.
	if s.evo != nil {
		exists, err := s.evo.OnWhatsApp(ctx(c), phone)
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

	chatID, _, err := s.store.FindOrCreateChat(ctx(c), s.accountID, jid, phone)
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
