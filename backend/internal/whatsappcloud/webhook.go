package whatsappcloud

import (
	"strconv"
	"time"
)

// WebhookPayload is the top-level shape of every WhatsApp Cloud API webhook
// delivery: {"object":"whatsapp_business_account","entry":[...]}.
type WebhookPayload struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

// Entry is one WABA's worth of changes in a single delivery — ID is the
// WABA id, the value store.ChannelAccountByExternalID resolves the owning
// channel_accounts row from.
type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

type Change struct {
	Field string `json:"field"`
	Value Value  `json:"value"`
}

// Value carries either inbound messages or outbound delivery statuses —
// never both in the same change in practice, but the shape allows it, so
// callers must range over both slices.
type Value struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Contacts         []Contact `json:"contacts"`
	Messages         []Message `json:"messages"`
	Statuses         []Status  `json:"statuses"`
}

// Metadata names which of the WABA's numbers this change is about —
// PhoneNumberID is channel_accounts.external_account_id.
type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

// Contact is the customer profile info accompanying an inbound message —
// present once per distinct sender in the delivery, keyed by WAID (the
// customer's E.164 number, digits only).
type Contact struct {
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	WAID string `json:"wa_id"`
}

// Message is one inbound customer message. WhatsApp Cloud never echoes our
// own outbound sends back through the webhook (§10) — every Message here is
// a genuine inbound.
type Message struct {
	From      string `json:"from"` // the customer's WAID
	ID        string `json:"id"`   // wamid.… — the dedup key
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"` // text|image|video|audio|document|sticker|location|contacts|button|interactive|...
	Text      *struct {
		Body string `json:"body"`
	} `json:"text,omitempty"`
	Image    *Media `json:"image,omitempty"`
	Video    *Media `json:"video,omitempty"`
	Audio    *Media `json:"audio,omitempty"`
	Document *Media `json:"document,omitempty"`
	Sticker  *Media `json:"sticker,omitempty"`
}

// Media is one attachment's provider handle — an id to resolve via
// Client.MediaInfo, never a direct URL.
type Media struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"` // documents only
}

// Status is one outbound message's delivery-status update.
type Status struct {
	ID          string `json:"id"` // the wamid this status is about
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	RecipientID string `json:"recipient_id"`
	Errors      []struct {
		Code  int    `json:"code"`
		Title string `json:"title"`
	} `json:"errors,omitempty"`
}

// Attachment returns m's media handle, or nil for a text/unsupported
// message type.
func (m Message) Attachment() *Media {
	switch m.Type {
	case "image":
		return m.Image
	case "video":
		return m.Video
	case "audio":
		return m.Audio
	case "document":
		return m.Document
	case "sticker":
		return m.Sticker
	}
	return nil
}

// MessageKind maps m to xchats' shared media vocabulary, mirroring
// tgingest.MessageKind's naming ("imageMessage", ...) so the inbox renders
// every channel's attachments the same way.
func (m Message) MessageKind() string {
	switch m.Type {
	case "image":
		return "imageMessage"
	case "video":
		return "videoMessage"
	case "audio":
		return "audioMessage"
	case "document":
		return "documentMessage"
	case "sticker":
		return "stickerMessage"
	default:
		return "conversation"
	}
}

// Body returns the text to store as the message's body: the text message's
// own body, or a media message's caption (a caption IS the message body —
// the same convention Telegram's ingest uses), or "" for a bare attachment
// with no caption.
func (m Message) Body() string {
	if m.Text != nil {
		return m.Text.Body
	}
	if a := m.Attachment(); a != nil {
		return a.Caption
	}
	return ""
}

// Preview is the chat-list line for m: the body when there is one, else an
// emoji placeholder matching whatsmeow's own media placeholders so the
// inbox reads consistently across every WhatsApp-shaped channel.
func (m Message) Preview() string {
	if body := m.Body(); body != "" {
		return body
	}
	switch m.Type {
	case "image":
		return "📷 Фото"
	case "video":
		return "🎥 Видео"
	case "audio":
		return "🎵 Аудио"
	case "document":
		return "📄 Документ"
	case "sticker":
		return "🌟 Стикер"
	default:
		return ""
	}
}

// Ignorable reports whether m is a message type this codebase does not
// model yet (location/contacts/button/interactive/reaction/unknown) —
// ignored (acked 200), never stored as an empty/garbled row.
func (m Message) Ignorable() (reason string, ignorable bool) {
	switch m.Type {
	case "text", "image", "video", "audio", "document", "sticker":
		return "", false
	default:
		return "unsupported message type " + m.Type, true
	}
}

// ParseTimestamp parses a webhook message/status's unix-seconds-as-string
// timestamp. An unparseable value is not fatal — the ingest processor falls
// back to "now" rather than dropping the message.
func ParseTimestamp(raw string) time.Time {
	sec, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

// DeliveryRank mirrors store.AdvanceDeliveryState's own monotonic ladder —
// duplicated here rather than exported from internal/store, the same
// reason whatsmeow's own deliveryRank is duplicated: it is the caller's job
// to know which rank it is asking to advance to.
func DeliveryRank(status string) int {
	switch status {
	case "sent":
		return 1
	case "delivered":
		return 2
	case "read":
		return 3
	case "failed":
		return 4
	default:
		return 0
	}
}
