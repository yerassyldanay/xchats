// Package messengerish is the Instagram Direct and Facebook Messenger
// channel adapter — the internal/whatsappcloud analogue for these two
// products, built on the shared backend/internal/meta transport. Both speak
// the SAME Messenger-Platform-shaped webhook envelope (entry[].messaging[],
// unlike WhatsApp Cloud's entry[].changes[].value.messages[]) and the same
// /messages send endpoint, so one shared package serves both channels
// rather than two near-duplicates. Provider vocabulary that DOES differ
// (Instagram Login vs Facebook Login OAuth, graph.instagram.com vs
// graph.facebook.com hosts, "instagram" vs "page" webhook object values) is
// parametrized at the call site, never hardcoded here.
package messengerish

import "time"

// WebhookPayload is the top-level shape both deliveries share:
// {"object":"instagram"|"page","entry":[...]}.
type WebhookPayload struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

// Entry is one connected account's (an IG user id, or a Page id) worth of
// events in a single delivery — ID is exactly channel_accounts.
// external_account_id (unlike WhatsApp Cloud's WABA-vs-phone-number
// indirection, Instagram/Messenger's entry.id names the connected account
// directly).
type Entry struct {
	ID        string           `json:"id"`
	Time      int64            `json:"time"`
	Messaging []MessagingEvent `json:"messaging"`
}

// Party is an opaque, provider-scoped participant id (an IGSID or a PSID) —
// meaningless outside the one connected account it arrived on.
type Party struct {
	ID string `json:"id"`
}

// MessagingEvent is one inbound message, or Meta's echo of our own outbound
// send (see Message.IsEcho) — unlike WhatsApp Cloud (§10), Instagram and
// Messenger both echo every outbound send back through this same webhook.
type MessagingEvent struct {
	Sender    Party    `json:"sender"`
	Recipient Party    `json:"recipient"`
	Timestamp int64    `json:"timestamp"` // unix MILLISECONDS
	Message   *Message `json:"message,omitempty"`
}

// Message is the inbound (or echoed-outbound) message body.
type Message struct {
	MID         string       `json:"mid"`
	Text        string       `json:"text,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	// IsEcho is true when this event is Meta echoing OUR OWN outbound send
	// back through the webhook — the ingest processor's Direction/SenderKind
	// signal (see the httpapi Normalize functions built on this package).
	IsEcho bool `json:"is_echo,omitempty"`
	// IsDeleted marks a message the sender unsent — not modeled as its own
	// inbox state in v1; Ignorable treats it as unsupported.
	IsDeleted bool `json:"is_deleted,omitempty"`
}

// Attachment is one inbound (or echoed) media item — Payload.URL is a
// direct, Meta-hosted CDN url fetchable with NO auth at all, unlike
// WhatsApp Cloud's two-step media-id -> signed-url resolution.
type Attachment struct {
	Type    string `json:"type"` // image|video|audio|file|share|...
	Payload struct {
		URL string `json:"url"`
	} `json:"payload"`
}

// Ignorable reports whether e is a webhook delivery this codebase does not
// model as an inbox message — a non-message event (a postback, a reaction,
// a read/delivery receipt — Instagram/Messenger DMs surface these on the
// same messaging[] array), a deleted message, an attachment type this
// codebase does not render, or a message with neither text nor a supported
// attachment.
func (e MessagingEvent) Ignorable() (reason string, ignorable bool) {
	if e.Message == nil {
		return "non_message_event", true
	}
	if e.Message.IsDeleted {
		return "message_deleted", true
	}
	if e.Message.Text != "" {
		return "", false
	}
	if att := e.Attachment(); att != nil {
		switch att.Type {
		case "image", "video", "audio", "file":
			return "", false
		default:
			return "unsupported attachment type " + att.Type, true
		}
	}
	return "empty_message", true
}

// Attachment returns e's first attachment, or nil for a text-only message —
// mirrors whatsappcloud.Message.Attachment. Instagram/Messenger deliver at
// most one attachment per message in practice; this codebase, like
// WhatsApp Cloud, only ever renders the first.
func (e MessagingEvent) Attachment() *Attachment {
	if e.Message == nil || len(e.Message.Attachments) == 0 {
		return nil
	}
	return &e.Message.Attachments[0]
}

// MessageKind maps e to this codebase's shared media vocabulary — mirrors
// whatsappcloud.Message.MessageKind's identical naming.
func (e MessagingEvent) MessageKind() string {
	att := e.Attachment()
	if att == nil {
		return "conversation"
	}
	switch att.Type {
	case "image":
		return "imageMessage"
	case "video":
		return "videoMessage"
	case "audio":
		return "audioMessage"
	case "file":
		return "documentMessage"
	default:
		return "conversation"
	}
}

// Body returns the text to store as the message's body. Unlike WhatsApp
// Cloud, Instagram/Messenger attachments carry no caption field at all —
// an attachment message's body is always empty.
func (e MessagingEvent) Body() string {
	if e.Message == nil {
		return ""
	}
	return e.Message.Text
}

// Preview is the chat-list line for e — mirrors whatsappcloud.Message.
// Preview's identical placeholder convention.
func (e MessagingEvent) Preview() string {
	if body := e.Body(); body != "" {
		return body
	}
	att := e.Attachment()
	if att == nil {
		return ""
	}
	switch att.Type {
	case "image":
		return "📷 Фото"
	case "video":
		return "🎥 Видео"
	case "audio":
		return "🎵 Аудио"
	case "file":
		return "📄 Документ"
	default:
		return ""
	}
}

// ParseTimestampMillis parses a webhook event's unix-MILLISECONDS
// Timestamp — distinct from WhatsApp Cloud's unix-seconds-as-string
// convention (see whatsappcloud.ParseTimestamp). <= 0 is not fatal — the
// ingest processor falls back to "now" rather than dropping the message.
func ParseTimestampMillis(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
