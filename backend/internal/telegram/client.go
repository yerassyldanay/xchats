// Package telegram is the Bot API client and the Telegram channel adapter.
// Provider vocabulary (update ids, chat ids, file ids, the bot token itself)
// stops here — everything above it sees the channel-neutral contracts in
// backend/messaging and the tg_* transport tables.
//
// The bot token is a per-call argument, never client state: one process serves
// many bots, and the token is resolved from encrypted storage at the moment of
// the call. It is also the URL path segment, which is why every log and error
// in this file goes through redact().
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// --- wire types ------------------------------------------------------------

// User is a Telegram user or bot.
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

// Chat is a Telegram conversation. Type is "private" for a 1:1 bot chat;
// anything else (group, supergroup, channel) is out of scope for now.
type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

// PhotoSize is one rendition of a photo. Telegram sends several; the largest is
// the one worth downloading.
type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int    `json:"file_size"`
}

// Document, Video, Audio, Voice and VideoNote all carry the same file handle
// shape; Doc covers them with the union of their optional fields.
type Doc struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int    `json:"file_size"`
	Duration     int    `json:"duration"`
}

// Sticker is a sticker attachment.
type Sticker struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Emoji        string `json:"emoji"`
	FileSize     int    `json:"file_size"`
}

// Message is one Telegram message. Only the fields the inbox actually ingests
// are decoded; the raw JSON is preserved separately by the caller.
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      *Chat  `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text"`
	Caption   string `json:"caption"`

	Photo     []PhotoSize `json:"photo"`
	Document  *Doc        `json:"document"`
	Video     *Doc        `json:"video"`
	Audio     *Doc        `json:"audio"`
	Voice     *Doc        `json:"voice"`
	VideoNote *Doc        `json:"video_note"`
	Sticker   *Sticker    `json:"sticker"`

	// Service messages: presence of any of these means "not a conversation
	// message" and the update is ignored.
	NewChatMembers  []User `json:"new_chat_members"`
	LeftChatMember  *User  `json:"left_chat_member"`
	NewChatTitle    string `json:"new_chat_title"`
	GroupChatMsg    bool   `json:"group_chat_created"`
	SuperGroupMsg   bool   `json:"supergroup_chat_created"`
	ChannelChatMsg  bool   `json:"channel_chat_created"`
	MigrateToChatID int64  `json:"migrate_to_chat_id"`
	PinnedMessage   *struct {
		MessageID int64 `json:"message_id"`
	} `json:"pinned_message"`
}

// Update is one webhook delivery. Only `message` is subscribed
// (allowed_updates), so every other field stays nil and the update is ignored.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// WebhookInfo is getWebhookInfo's payload — the reconciliation source of truth
// when a setWebhook call times out and we cannot tell whether it landed.
type WebhookInfo struct {
	URL                  string   `json:"url"`
	HasCustomCertificate bool     `json:"has_custom_certificate"`
	PendingUpdateCount   int      `json:"pending_update_count"`
	LastErrorDate        int64    `json:"last_error_date"`
	LastErrorMessage     string   `json:"last_error_message"`
	MaxConnections       int      `json:"max_connections"`
	AllowedUpdates       []string `json:"allowed_updates"`
}

// SentMessage is the outcome of a send call.
type SentMessage struct {
	MessageID int64
	ChatID    int64
}

// --- errors ----------------------------------------------------------------

// APIError is a Bot API `ok: false` response. Code is what distinguishes a bad
// token (401) from a blocked bot (403) from rate limiting (429), which the
// account state machine and the send path both branch on.
type APIError struct {
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram: api error %d: %s", e.Code, e.Description)
}

// IsUnauthorized reports whether err is a Bot API rejection of the token.
func IsUnauthorized(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Code == http.StatusUnauthorized
}

// APIErrorCode returns the Bot API error code behind err, or 0 if err is not
// one. 403 is a user who blocked the bot, 429 is rate limiting — both are
// terminal for a single send, neither means the account is broken.
func APIErrorCode(err error) int {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

// --- port ------------------------------------------------------------------

// FileInfo is getFile's payload: FilePath is the suffix of the download URL.
type FileInfo struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
	FileSize int    `json:"file_size"`
}

// Upload is one outbound attachment. ProviderRef, when set, is a Telegram
// file_id for content Telegram already hosts: sending it re-uses the existing
// upload instead of pushing the bytes again.
type Upload struct {
	Kind        string // image|video|audio|voice|document
	FileName    string
	Mimetype    string
	Caption     string
	Data        []byte
	ProviderRef string
}

// Client is the Telegram port. The provisioning handlers, the send path and the
// media downloader depend on this interface so tests substitute a fake.
type Client interface {
	// GetMe validates a token and returns the bot's identity.
	GetMe(ctx context.Context, token string) (User, error)
	// SetWebhook points the bot at url. dropPending discards Telegram's
	// server-side backlog — only ever true on a deliberate first connect.
	SetWebhook(ctx context.Context, token, url, secret string, allowedUpdates []string, dropPending bool) error
	DeleteWebhook(ctx context.Context, token string, dropPending bool) error
	GetWebhookInfo(ctx context.Context, token string) (WebhookInfo, error)
	SendMessage(ctx context.Context, token string, chatID int64, text string) (SentMessage, error)
	// SendMedia delivers one attachment, picking the Bot API method that fits
	// its kind (sendPhoto / sendVideo / sendAudio / sendVoice / sendDocument).
	SendMedia(ctx context.Context, token string, chatID int64, up Upload) (SentMessage, error)
	// GetFile resolves a file_id to a download path.
	GetFile(ctx context.Context, token, fileID string) (FileInfo, error)
	// DownloadFile fetches the bytes at a path returned by GetFile.
	DownloadFile(ctx context.Context, token, filePath string) ([]byte, error)
}

// MediaMethod maps our media vocabulary to the Bot API method and its file
// field. Anything unrecognized falls back to sendDocument, which accepts any
// file — a wrong-but-delivered attachment beats a rejected send.
func MediaMethod(kind string) (method, field string) {
	switch kind {
	case "image", "photo":
		return "sendPhoto", "photo"
	case "video":
		return "sendVideo", "video"
	case "audio":
		return "sendAudio", "audio"
	case "voice":
		return "sendVoice", "voice"
	default:
		return "sendDocument", "document"
	}
}

// DefaultAllowedUpdates is the subscription: conversation messages only. Adding
// an update type here without an ingest path for it just burns webhook traffic.
var DefaultAllowedUpdates = []string{"message"}

// HTTP is the real Bot API client.
type HTTP struct {
	base string
	hc   *http.Client
	log  *slog.Logger
}

// NewHTTP builds a client against base (default https://api.telegram.org).
// log may be nil (falls back to slog.Default).
func NewHTTP(base string, log *slog.Logger) *HTTP {
	if base == "" {
		base = "https://api.telegram.org"
	}
	if log == nil {
		log = slog.Default()
	}
	return &HTTP{
		base: strings.TrimRight(base, "/"),
		hc:   &http.Client{Timeout: 20 * time.Second},
		log:  log,
	}
}

// GetMe validates the token and returns the bot identity behind it.
func (h *HTTP) GetMe(ctx context.Context, token string) (User, error) {
	var out User
	err := h.call(ctx, token, "getMe", nil, &out)
	return out, err
}

// SetWebhook registers url as the bot's webhook.
func (h *HTTP) SetWebhook(ctx context.Context, token, url, secret string, allowedUpdates []string, dropPending bool) error {
	if len(allowedUpdates) == 0 {
		allowedUpdates = DefaultAllowedUpdates
	}
	body := map[string]any{
		"url":                  url,
		"allowed_updates":      allowedUpdates,
		"drop_pending_updates": dropPending,
	}
	if secret != "" {
		body["secret_token"] = secret
	}
	return h.call(ctx, token, "setWebhook", body, nil)
}

// DeleteWebhook unregisters the bot's webhook.
func (h *HTTP) DeleteWebhook(ctx context.Context, token string, dropPending bool) error {
	return h.call(ctx, token, "deleteWebhook", map[string]any{"drop_pending_updates": dropPending}, nil)
}

// GetWebhookInfo reports what Telegram currently believes the webhook is.
func (h *HTTP) GetWebhookInfo(ctx context.Context, token string) (WebhookInfo, error) {
	var out WebhookInfo
	err := h.call(ctx, token, "getWebhookInfo", nil, &out)
	return out, err
}

// SendMessage delivers plain text. No parse_mode is set: draft text is
// operator-reviewed prose, and Markdown/HTML parsing would turn a stray
// underscore or asterisk into a 400 from Telegram.
func (h *HTTP) SendMessage(ctx context.Context, token string, chatID int64, text string) (SentMessage, error) {
	var out struct {
		MessageID int64 `json:"message_id"`
		Chat      *Chat `json:"chat"`
	}
	if err := h.call(ctx, token, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	}, &out); err != nil {
		return SentMessage{}, err
	}
	res := SentMessage{MessageID: out.MessageID}
	if out.Chat != nil {
		res.ChatID = out.Chat.ID
	}
	return res, nil
}

// SendMedia delivers one attachment. Content Telegram already hosts (a file_id
// carried over from an inbound message) is sent as a plain field, skipping the
// upload entirely; anything else goes as multipart.
func (h *HTTP) SendMedia(ctx context.Context, token string, chatID int64, up Upload) (SentMessage, error) {
	method, field := MediaMethod(up.Kind)

	var out struct {
		MessageID int64 `json:"message_id"`
		Chat      *Chat `json:"chat"`
	}
	if up.ProviderRef != "" {
		body := map[string]any{"chat_id": chatID, field: up.ProviderRef}
		if up.Caption != "" {
			body["caption"] = up.Caption
		}
		if err := h.call(ctx, token, method, body, &out); err != nil {
			return SentMessage{}, err
		}
		return SentMessage{MessageID: out.MessageID, ChatID: chatID}, nil
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("chat_id", strconv.FormatInt(chatID, 10))
	if up.Caption != "" {
		_ = mw.WriteField("caption", up.Caption)
	}
	name := up.FileName
	if name == "" {
		name = "file"
	}
	part, err := mw.CreateFormFile(field, name)
	if err != nil {
		return SentMessage{}, fmt.Errorf("telegram: %s: form file: %w", method, err)
	}
	if _, err := part.Write(up.Data); err != nil {
		return SentMessage{}, fmt.Errorf("telegram: %s: write file: %w", method, err)
	}
	if err := mw.Close(); err != nil {
		return SentMessage{}, fmt.Errorf("telegram: %s: close multipart: %w", method, err)
	}
	if err := h.callRaw(ctx, token, method, mw.FormDataContentType(), buf.Bytes(), &out); err != nil {
		return SentMessage{}, err
	}
	return SentMessage{MessageID: out.MessageID, ChatID: chatID}, nil
}

// GetFile resolves a file_id to the path DownloadFile needs.
func (h *HTTP) GetFile(ctx context.Context, token, fileID string) (FileInfo, error) {
	var out FileInfo
	err := h.call(ctx, token, "getFile", map[string]any{"file_id": fileID}, &out)
	return out, err
}

// DownloadFile fetches file bytes. Note the URL shape: /file/bot<token>/<path>,
// NOT the /bot<token>/<method> the API calls use — same token, different root.
func (h *HTTP) DownloadFile(ctx context.Context, token, filePath string) ([]byte, error) {
	url := h.base + "/file/bot" + token + "/" + filePath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: download: %w", redactErr(err, token))
	}
	resp, err := h.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram: download: %w", redactErr(err, token))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.log.Warn("telegram: download failed", "status", resp.StatusCode)
		return nil, fmt.Errorf("telegram: download: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("telegram: download: read body: %w", err)
	}
	return data, nil
}

// call performs one Bot API method and unwraps the {ok, result, description}
// envelope. `ok: false` becomes an *APIError carrying Telegram's own
// description — the operator-facing reason a connection failed.
func (h *HTTP) call(ctx context.Context, token, method string, body any, out any) error {
	var raw []byte
	contentType := ""
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("telegram: marshal %s: %w", method, err)
		}
		raw, contentType = b, "application/json"
	}
	return h.callRaw(ctx, token, method, contentType, raw, out)
}

// callRaw is call's transport half, shared with the multipart upload path.
func (h *HTTP) callRaw(ctx context.Context, token, method, contentType string, body []byte, out any) error {
	endpoint := h.base + "/bot" + token + "/" + method

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, reader)
	if err != nil {
		return fmt.Errorf("telegram: new request %s: %w", method, redactErr(err, token))
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := h.hc.Do(req)
	if err != nil {
		// The URL carries the token, and net/http puts the URL in its errors.
		return fmt.Errorf("telegram: %s: %w", method, redactErr(err, token))
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("telegram: %s: read body: %w", method, err)
	}

	var env struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
		ErrorCode   int             `json:"error_code"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		h.log.Warn("telegram: unparsable response", "method", method, "status", resp.StatusCode)
		return fmt.Errorf("telegram: %s: unparsable response (status %d)", method, resp.StatusCode)
	}
	if !env.OK {
		code := env.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		h.log.Warn("telegram: api error", "method", method, "code", code, "description", env.Description)
		return &APIError{Code: code, Description: env.Description}
	}
	h.log.Debug("telegram: call ok", "method", method, "status", resp.StatusCode)
	if out == nil || len(env.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Result, out); err != nil {
		return fmt.Errorf("telegram: %s: decode result: %w", method, err)
	}
	return nil
}

// --- parsing ---------------------------------------------------------------

// ParseUpdate decodes a webhook body.
func ParseUpdate(raw []byte) (*Update, error) {
	var u Update
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil, fmt.Errorf("telegram: parse update: %w", err)
	}
	return &u, nil
}

// Ignorable reports whether an update carries nothing this inbox stores, and
// why. A non-`message` update, a non-private chat, or a service message all get
// a plain 200: they are correctly delivered, just not conversation content, and
// answering non-2xx would make Telegram redeliver them forever.
func (u *Update) Ignorable() (string, bool) {
	if u == nil {
		return "empty_update", true
	}
	m := u.Message
	if m == nil {
		return "not_a_message_update", true
	}
	if m.Chat == nil {
		return "no_chat", true
	}
	if m.Chat.Type != "private" {
		return "non_private_chat", true
	}
	if m.From == nil || m.From.IsBot {
		return "no_human_sender", true
	}
	if m.IsService() {
		return "service_message", true
	}
	if m.Text == "" && m.Caption == "" && m.MediaKind() == "" {
		return "no_content", true
	}
	return "", false
}

// IsService reports whether the message is a chat-lifecycle notice rather than
// something a person typed.
func (m *Message) IsService() bool {
	return len(m.NewChatMembers) > 0 || m.LeftChatMember != nil || m.NewChatTitle != "" ||
		m.GroupChatMsg || m.SuperGroupMsg || m.ChannelChatMsg ||
		m.MigrateToChatID != 0 || m.PinnedMessage != nil
}

// MediaKind returns the message's media type in our own vocabulary ("image",
// "video", "audio", "document", "sticker"), or "" when it carries none.
func (m *Message) MediaKind() string {
	switch {
	case len(m.Photo) > 0:
		return "image"
	case m.Video != nil:
		return "video"
	case m.VideoNote != nil:
		return "video"
	case m.Voice != nil:
		return "audio"
	case m.Audio != nil:
		return "audio"
	case m.Sticker != nil:
		return "sticker"
	case m.Document != nil:
		return "document"
	}
	return ""
}

// File describes the downloadable attachment on a message.
type File struct {
	FileID       string
	FileUniqueID string
	Kind         string // image|video|audio|document|sticker
	Mimetype     string
	FileName     string
	Size         int
}

// Attachment returns the message's file handle, picking the largest rendition
// for a photo (Telegram orders them ascending, but do not rely on it).
func (m *Message) Attachment() *File {
	kind := m.MediaKind()
	if kind == "" {
		return nil
	}
	switch {
	case len(m.Photo) > 0:
		best := m.Photo[0]
		for _, p := range m.Photo[1:] {
			if p.Width*p.Height > best.Width*best.Height {
				best = p
			}
		}
		return &File{FileID: best.FileID, FileUniqueID: best.FileUniqueID, Kind: kind,
			Mimetype: "image/jpeg", Size: best.FileSize}
	case m.Sticker != nil:
		return &File{FileID: m.Sticker.FileID, FileUniqueID: m.Sticker.FileUniqueID, Kind: kind,
			Mimetype: "image/webp", Size: m.Sticker.FileSize}
	}
	for _, d := range []*Doc{m.Video, m.VideoNote, m.Voice, m.Audio, m.Document} {
		if d == nil {
			continue
		}
		return &File{FileID: d.FileID, FileUniqueID: d.FileUniqueID, Kind: kind,
			Mimetype: d.MimeType, FileName: d.FileName, Size: d.FileSize}
	}
	return nil
}

// DisplayName is how the contact is shown when they have no username.
func (u *User) DisplayName() string {
	full := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if full != "" {
		return full
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf("%d", u.ID)
}

// Timestamp converts the Unix `date` field.
func (m *Message) Timestamp() time.Time {
	if m.Date == 0 {
		return time.Time{}
	}
	return time.Unix(m.Date, 0).UTC()
}

// --- redaction -------------------------------------------------------------

// redact replaces every occurrence of the token in s. The Bot API puts the
// token in the URL path, so it otherwise leaks through any error that quotes
// the request URL.
func redact(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "bot<redacted>")
}

// redactedError carries a token-free message. It wraps the CAUSE (one level in)
// rather than the error it replaced: the outer *url.Error is exactly the value
// whose Error() embeds the request URL — and therefore the token — while its
// cause (a timeout, a DNS failure, context.DeadlineExceeded) does not, so
// errors.Is still works without handing callers a way to print the secret back.
type redactedError struct {
	msg   string
	cause error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.cause }

func redactErr(err error, token string) error {
	if err == nil {
		return nil
	}
	return &redactedError{msg: redact(err.Error(), token), cause: errors.Unwrap(err)}
}
