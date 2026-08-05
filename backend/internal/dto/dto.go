// Package dto holds the JSON entity shapes shared by the HTTP API (responses) and
// the realtime layer (SSE), with mappers from the store types. One shape, two
// emitters — so a Chat over SSE is byte-identical to a Chat from GET /chats.
package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

func decodeAttrs(b []byte) map[string]any {
	if len(b) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// Contact is the API contact shape.
type Contact struct {
	ID          string         `json:"id"`
	DisplayName string         `json:"display_name"`
	PhoneNumber string         `json:"phone_number"`
	PhoneJID    string         `json:"phone_jid"`
	LidJID      string         `json:"lid_jid"`
	PushName    string         `json:"push_name"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

// Chat is the API chat shape.
type Chat struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	AccountID string `json:"account_id"`
	// WaAccountID is the deprecated alias for AccountID, emitted only for chats
	// on the wa_* gateway (whatsapp/simulator) so an older client keeps working
	// through the transition. A Telegram chat deliberately omits it rather than
	// pretending its account is a WhatsApp one.
	WaAccountID        string  `json:"wa_account_id,omitempty"`
	Contact            Contact `json:"contact"`
	Status             string  `json:"status"`
	AssigneeUserID     *string `json:"assignee_user_id"`
	UnreadCount        int     `json:"unread_count"`
	LastMessageAt      *string `json:"last_message_at"`
	LastMessagePreview string  `json:"last_message_preview"`
}

// Media is one media item on a message (a "list of URLs", each enriched).
type Media struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
	Mimetype  string `json:"mimetype"`
	FileName  string `json:"file_name"`
	FileSize  int    `json:"file_size"`
}

// Message is the API message shape.
type Message struct {
	ID                string  `json:"id"`
	ChatID            string  `json:"chat_id"`
	Direction         string  `json:"direction"`
	SenderType        string  `json:"sender_type"`
	SenderUserID      *string `json:"sender_user_id"`
	ExternalMessageID string  `json:"external_message_id"`
	MessageType       string  `json:"message_type"`
	Content           string  `json:"content"`
	Media             []Media `json:"media"`
	Status            string  `json:"status"`
	Source            string  `json:"source"`
	Timestamp         *string `json:"timestamp"`
}

// AiDraft is one suggested option.
type AiDraft struct {
	ID               string   `json:"id"`
	ChatID           string   `json:"chat_id"`
	TriggerMessageID *string  `json:"trigger_message_id"`
	Ordinal          int      `json:"ordinal"`
	DraftText        string   `json:"draft_text"`
	ReplyLanguage    string   `json:"reply_language"`
	ContextStatus    string   `json:"context_status"`
	Confidence       *float64 `json:"confidence"`
	Escalate         bool     `json:"escalate"`
	EscalationReason string   `json:"escalation_reason"`
	Status           string   `json:"status"`
	CreatedAt        string   `json:"created_at"`
}

// WhatsAppAccount is the API shape for one connected number. "assigned" is
// derived (organization_id IS NOT NULL), never stored.
type WhatsAppAccount struct {
	ID               string  `json:"id"`
	DisplayName      string  `json:"display_name"`
	ConnectionStatus string  `json:"connection_status"`
	Assigned         bool    `json:"assigned"`
	OwnerJID         string  `json:"owner_jid"`
	PhoneNumber      string  `json:"phone_number"`
	LastLiveEventAt  *string `json:"last_live_event_at"`
	CreatedAt        *string `json:"created_at"`
}

// MapAccount maps a store.Account. liveStatus overrides the stored connection_state
// when a live probe is available (else pass the stored state). A blank
// display_name falls back to the phone number for the UI.
func MapAccount(a store.Account, liveStatus string) WhatsAppAccount {
	name := a.DisplayName
	if name == "" {
		name = a.ExternalHandle
	}
	status := liveStatus
	if status == "" {
		status = a.ConnectionState
	}
	return WhatsAppAccount{
		ID:               a.ID.String(),
		DisplayName:      name,
		ConnectionStatus: status,
		Assigned:         a.OrganizationID.Valid,
		OwnerJID:         a.ExternalAccountRef,
		PhoneNumber:      a.ExternalHandle,
		LastLiveEventAt:  tsPtr(a.LastLiveEventAt),
		CreatedAt:        tsPtr(&a.CreatedAt),
	}
}

// Account is the channel-neutral account shape GET /accounts returns — the one
// list the UI renders for every channel. The webhook_* fields are Telegram
// health and stay null on a WhatsApp row.
type Account struct {
	ID              string  `json:"id"`
	Channel         string  `json:"channel"`
	DisplayName     string  `json:"display_name"`
	ExternalHandle  string  `json:"external_handle"`
	ConnectionState string  `json:"connection_state"`
	Assigned        bool    `json:"assigned"`
	LastLiveEventAt *string `json:"last_live_event_at"`
	CreatedAt       *string `json:"created_at"`

	WebhookURL           *string `json:"webhook_url"`
	WebhookRegisteredAt  *string `json:"webhook_registered_at"`
	WebhookLastCheckedAt *string `json:"webhook_last_checked_at"`
	WebhookLastError     *string `json:"webhook_last_error"`
}

// MapNeutralAccount maps a store.Account (as read from inbox_accounts_v) to the
// neutral API shape. liveStatus overrides the stored state when a live probe is
// available, exactly like MapAccount.
func MapNeutralAccount(a store.Account, liveStatus string) Account {
	name := a.DisplayName
	if name == "" {
		name = a.ExternalHandle
	}
	status := liveStatus
	if status == "" {
		status = a.ConnectionState
	}
	out := Account{
		ID:              a.ID.String(),
		Channel:         a.Channel,
		DisplayName:     name,
		ExternalHandle:  a.ExternalHandle,
		ConnectionState: status,
		Assigned:        a.OrganizationID.Valid,
		LastLiveEventAt: tsPtr(a.LastLiveEventAt),
		CreatedAt:       tsPtr(&a.CreatedAt),
	}
	// Only a channel that actually has webhook health reports it; a WhatsApp row
	// leaves all four null rather than emitting empty strings that read as "set".
	if a.Channel == "telegram" {
		url, lastErr := a.WebhookURL, a.WebhookLastError
		out.WebhookURL = &url
		out.WebhookLastError = &lastErr
		out.WebhookRegisteredAt = tsPtr(a.WebhookRegisteredAt)
		out.WebhookLastCheckedAt = tsPtr(a.WebhookLastCheckedAt)
	}
	return out
}

func mediaURL(id uuid.UUID) string { return "/xchats/api/v1/media/" + id.String() }

func tsPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func nullUUIDPtr(u uuid.NullUUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.UUID.String()
	return &s
}

// MapContact maps a store.Contact.
func MapContact(c store.Contact) Contact {
	name := c.DisplayName
	if name == "" {
		name = c.PushName
	}
	if name == "" {
		name = c.PhoneNumber
	}
	return Contact{
		ID:          c.ID.String(),
		DisplayName: name,
		PhoneNumber: c.PhoneNumber,
		PhoneJID:    c.ExternalContactRef,
		LidJID:      c.LidJID,
		PushName:    c.PushName,
		Attributes:  decodeAttrs(c.Attributes),
	}
}

// MapChat maps a store.Chat (with embedded contact).
func MapChat(c store.Chat) Chat {
	channel := c.Channel
	if channel == "" {
		channel = "whatsapp"
	}
	out := Chat{
		ID:                 c.ID.String(),
		Channel:            channel,
		AccountID:          c.AccountID.String(),
		Contact:            MapContact(c.Contact),
		Status:             c.ChatState,
		AssigneeUserID:     nullUUIDPtr(c.AssigneeUserID),
		UnreadCount:        c.UnreadCount,
		LastMessageAt:      tsPtr(c.LastMessageAt),
		LastMessagePreview: c.LastMessagePreview,
	}
	if channel != "telegram" {
		out.WaAccountID = c.AccountID.String()
	}
	return out
}

// MapMessage maps a store.Message (with media).
func MapMessage(m store.Message) Message {
	media := make([]Media, 0, len(m.Media))
	for _, r := range m.Media {
		media = append(media, Media{
			ID: r.ID.String(), URL: mediaURL(r.ID), MediaType: r.MediaType,
			Mimetype: r.Mimetype, FileName: r.FileName, FileSize: r.FileSize,
		})
	}
	return Message{
		ID:                m.ID.String(),
		ChatID:            m.ChatID.String(),
		Direction:         m.Direction,
		SenderType:        m.SenderKind,
		SenderUserID:      nullUUIDPtr(m.SenderUserID),
		ExternalMessageID: m.ExternalMessageID,
		MessageType:       m.MessageKind,
		Content:           m.Body,
		Media:             media,
		Status:            m.DeliveryState,
		Source:            m.Source,
		Timestamp:         tsPtr(m.MessageTS),
	}
}

// MapDraft maps a store.Draft.
func MapDraft(d store.Draft) AiDraft {
	return AiDraft{
		ID:               d.ID.String(),
		ChatID:           d.ChatID.String(),
		TriggerMessageID: nullUUIDPtr(d.TriggerMessageID),
		Ordinal:          d.OptionOrdinal,
		DraftText:        d.DraftText,
		ReplyLanguage:    d.ReplyLanguage,
		ContextStatus:    d.ContextState,
		Confidence:       d.Confidence,
		Escalate:         d.Escalate,
		EscalationReason: d.EscalationReason,
		Status:           d.DraftState,
		CreatedAt:        d.CreatedAt.UTC().Format(time.RFC3339),
	}
}
