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
// message (one provider call each). It inserts the row, broadcasts
// message.created, then enqueues the outbound_send task — shared by manual send
// (sender_kind=user) and AI approve (sender_kind=ai).
func (s *Server) sendParts(c *gin.Context, chat store.Chat, senderKind string, senderUserID uuid.NullUUID, text string, mediaIDs []string) ([]dto.Message, error) {
	var out []dto.Message
	// The channel comes from the chat itself (the view already carries it) when
	// set; falling back to a fresh account lookup only covers rows written
	// before the channel column existed.
	channel := chat.Channel
	if channel == "" {
		if acct, err := s.store.AccountByID(ctx(c), chat.AccountID); err == nil {
			channel = acct.Channel
		} else {
			s.log.Warn("send: account lookup failed", "chat_id", chat.ID, "account_id", chat.AccountID, "err", err)
		}
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
			Destination: chat.ExternalConversationRef, Text: text,
		}})
		s.log.Info("send queued", "chat_id", chat.ID, "message_id", msgID, "channel", channel,
			"sender_kind", senderKind, "kind", "text")
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
			Destination: chat.ExternalConversationRef, MediaID: mid, Caption: caption,
		}})
		s.log.Info("send queued", "chat_id", chat.ID, "message_id", msgID,
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

type assignChatReq struct {
	AssigneeUserID *string `json:"assignee_user_id"`
}

func (s *Server) handleAssignChat(c *gin.Context) {
	chatID, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	if _, ok := s.orgChat(c, chatID); !ok {
		return
	}
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	var req assignChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid request")
		return
	}
	assignee := uuid.NullUUID{}
	if req.AssigneeUserID != nil && strings.TrimSpace(*req.AssigneeUserID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.AssigneeUserID))
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid assignee_user_id")
			return
		}
		member, err := s.store.UserInOrg(ctx(c), id, org.ID)
		if err != nil {
			fail(c, http.StatusInternalServerError, ErrInternal, "membership check failed")
			return
		}
		if !member {
			fail(c, http.StatusBadRequest, ErrValidation, "assignee is not an organization member")
			return
		}
		assignee = uuid.NullUUID{UUID: id, Valid: true}
	}
	chat, err := s.store.AssignChat(ctx(c), chatID, assignee)
	if err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, "failed to update assignee")
		return
	}
	mapped := dto.MapChat(chat)
	s.hub.Broadcast("chat.updated", mapped)
	ok(c, mapped)
}

func (s *Server) handleUploadMedia(c *gin.Context) {
	// A9: gin's MaxMultipartMemory (server.go's Router) only bounds the
	// memory/disk spill point during multipart parsing — with no cap on the
	// body itself, an attacker could stream an arbitrarily large request
	// and have it fully read into memory below. MaxBytesReader is the
	// actual hard limit; it must wrap the body before any parsing touches
	// it, since c.FormFile below is what triggers the read.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)
	fh, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(c, http.StatusRequestEntityTooLarge, ErrValidation, "upload exceeds the maximum allowed size")
			return
		}
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
	// A3: reject a payload that sniffs as HTML unless HTML was actually
	// declared — the classic stored-XSS-via-mislabeled-upload vector this
	// route serves back on the app's own origin (see handleServeMedia's
	// header block below, its actual defense in depth). Same check the MCP
	// upload path already applies (mcp_upload.go); reused, not reimplemented.
	if reason := mimeSanityCheck(mimetype, data); reason != "" {
		fail(c, http.StatusBadRequest, ErrValidation, reason)
		return
	}
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
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return
	}
	// A public media id is usually a message_media.id (uuid), scoped to the
	// caller's own organization — see MediaStorageURL's own doc comment for
	// why (A4: this is what closes the cross-org media IDOR).
	if id, err := uuid.Parse(raw); err == nil {
		if storageURL, mimetype, fileName, merr := s.store.MediaStorageURL(ctx(c), org.ID, id); merr == nil {
			data, meta, gerr := s.blob.Get(storageURL)
			if gerr != nil {
				fail(c, http.StatusBadGateway, ErrMediaUnavailable, "blob missing")
				return
			}
			ct := mimetype
			if ct == "" {
				ct = meta.Mimetype
			}
			s.writeMediaResponse(c, ct, filenameOr(fileName, meta.FileName), data)
			return
		} else if !errors.Is(merr, store.ErrNotFound) {
			fail(c, http.StatusInternalServerError, ErrInternal, merr.Error())
			return
		}
	}
	// Fall back: a pending upload not yet attached to any message.
	// handleUploadMedia returns this exact blob key as the media id, so a
	// composer can preview it before the user hits send — there is no
	// message_media row, and so no organization, to scope this against
	// until UpsertOutboundMedia runs (at which point the row gets its OWN,
	// different id — see MediaStorageURL's doc comment — so this fallback
	// stops being reachable for it at all). What bounds exposure here
	// instead: the key is a v4 UUID (122 bits of randomness) minted by the
	// uploader and never listed or enumerated anywhere, and every upload is
	// already MIME-sanity-checked at write time (handleUploadMedia).
	if data, meta, err := s.blob.Get(raw); err == nil {
		s.writeMediaResponse(c, meta.Mimetype, meta.FileName, data)
		return
	}
	fail(c, http.StatusNotFound, ErrNotFound, "media not found")
}

// writeMediaResponse is handleServeMedia's shared response tail for both
// the org-scoped and pending-upload-fallback paths above — A3's hardening
// applies uniformly to both: nosniff, a locked-down CSP, and a
// Content-Disposition that renders images/video/audio inline and
// everything else as an attachment. contentDisposition is mcp_media.go's
// (reused, not reimplemented — same defense, same reasoning: this serves
// user-supplied bytes from the same origin that holds the app's session
// cookie).
func (s *Server) writeMediaResponse(c *gin.Context, mimetype, fileName string, data []byte) {
	ct := ctOrDefault(mimetype)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Disposition", contentDisposition(ct, fileName))
	c.Data(http.StatusOK, ct, data)
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
