// Package normalize flattens raw Evolution v2.3.7 webhook events into the fields
// xchats persists. Package-owned webhook fixtures verify the contract.
package normalize

import (
	"encoding/json"
	"strings"
	"time"
)

// MediaTypes are the messageType values that carry a media payload.
var mediaTypes = map[string]bool{
	"imageMessage":    true,
	"videoMessage":    true,
	"audioMessage":    true,
	"documentMessage": true,
	"stickerMessage":  true,
}

// Envelope is the common webhook wrapper every Evolution event shares.
type Envelope struct {
	Event    string          `json:"event"`
	Instance string          `json:"instance"`
	Sender   string          `json:"sender"` // the ACCOUNT's own JID (owner_jid), NOT the customer
	Data     json.RawMessage `json:"data"`
	DateTime string          `json:"date_time"`
}

// Media is the normalized media descriptor for a message.
type Media struct {
	Kind              string // image|video|audio|document|sticker
	Mimetype          string
	FileName          string
	FileSize          int64
	Base64            string // inline full bytes when the webhook carries them
	NeedsDownloadByID string // evolution message id, for the getBase64 fallback
}

// Message is a normalized inbound/outbound message event.
type Message struct {
	EvolutionMessageID string
	Direction          string // in|out
	OwnerJID           string // account's own JID (envelope.sender)
	LidJID             string // @lid alias, when present
	PhoneJID           string // stable contact identity (remoteJidAlt)
	ContactJID         string // phone preferred, lid fallback
	ParticipantJID     string
	PushName           string
	MessageKind        string // messageType
	Text               string // text or media caption
	Media              *Media
	Timestamp          time.Time
	IsGroup            bool
	Raw                json.RawMessage
}

// StatusUpdate is a normalized delivery/read status event (messages.update).
type StatusUpdate struct {
	EvolutionMessageID string // data.keyId
	Status             string // raw WhatsApp status (DELIVERY_ACK, READ, …)
	DeliveryState      string // mapped xchats delivery_state
}

type keyT struct {
	RemoteJid      string `json:"remoteJid"`
	RemoteJidAlt   string `json:"remoteJidAlt"`
	FromMe         bool   `json:"fromMe"`
	ID             string `json:"id"`
	Participant    string `json:"participant"`
	AddressingMode string `json:"addressingMode"`
}

type messageData struct {
	Key              keyT                       `json:"key"`
	PushName         string                     `json:"pushName"`
	MessageType      string                     `json:"messageType"`
	Message          map[string]json.RawMessage `json:"message"`
	MessageTimestamp json.RawMessage            `json:"messageTimestamp"`
	Status           string                     `json:"status"`
}

type updateData struct {
	KeyId     string `json:"keyId"`
	RemoteJid string `json:"remoteJid"`
	FromMe    bool   `json:"fromMe"`
	Status    string `json:"status"`
}

// ParseEnvelope decodes the outer webhook wrapper.
func ParseEnvelope(raw []byte) (Envelope, error) {
	var e Envelope
	err := json.Unmarshal(raw, &e)
	return e, err
}

// Message normalizes a messages.upsert / send.message event into a Message.
// Returns (nil, nil) when the event isn't a message-bearing event.
func (e Envelope) Message() (*Message, error) {
	switch e.Event {
	case "messages.upsert", "send.message", "messages.set":
	default:
		return nil, nil
	}
	var d messageData
	if err := json.Unmarshal(e.Data, &d); err != nil {
		return nil, err
	}
	if d.Key.ID == "" {
		return nil, nil
	}

	lid := ""
	if strings.HasSuffix(d.Key.RemoteJid, "@lid") {
		lid = d.Key.RemoteJid
	}
	phone := d.Key.RemoteJidAlt
	if phone == "" && strings.HasSuffix(d.Key.RemoteJid, "@s.whatsapp.net") {
		phone = d.Key.RemoteJid
	}
	contact := phone
	if contact == "" {
		contact = d.Key.RemoteJid
	}

	m := &Message{
		EvolutionMessageID: d.Key.ID,
		Direction:          directionOf(d.Key.FromMe),
		OwnerJID:           e.Sender,
		LidJID:             lid,
		PhoneJID:           phone,
		ContactJID:         contact,
		ParticipantJID:     d.Key.Participant,
		PushName:           d.PushName,
		MessageKind:        d.MessageType,
		Timestamp:          unixToTime(parseLong(d.MessageTimestamp)),
		IsGroup:            strings.HasSuffix(d.Key.RemoteJid, "@g.us") || strings.HasSuffix(contact, "@g.us"),
		Raw:                append(json.RawMessage(nil), e.Data...),
	}
	m.Text, m.Media = extractContent(d.MessageType, d.Message)
	if m.Media != nil {
		m.Media.NeedsDownloadByID = d.Key.ID
	}
	return m, nil
}

// StatusUpdate normalizes a messages.update event.
func (e Envelope) StatusUpdate() (*StatusUpdate, error) {
	if e.Event != "messages.update" {
		return nil, nil
	}
	var d updateData
	if err := json.Unmarshal(e.Data, &d); err != nil {
		return nil, err
	}
	if d.KeyId == "" {
		return nil, nil
	}
	return &StatusUpdate{
		EvolutionMessageID: d.KeyId,
		Status:             d.Status,
		DeliveryState:      MapDeliveryState(d.Status),
	}, nil
}

// ConnectionState returns the connection state from a connection.update event
// ("open" | "connecting" | "close"), or "" for any other event. The state may be
// at data.state (v2) — the instance name rides the envelope's top-level field.
func (e Envelope) ConnectionState() string {
	if e.Event != "connection.update" {
		return ""
	}
	var d struct {
		State string `json:"state"`
	}
	_ = json.Unmarshal(e.Data, &d)
	return d.State
}

func extractContent(mtype string, msg map[string]json.RawMessage) (string, *Media) {
	if msg == nil {
		return "", nil
	}
	switch mtype {
	case "conversation":
		return jsonString(msg["conversation"]), nil
	case "extendedTextMessage":
		var ext struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(msg["extendedTextMessage"], &ext)
		return ext.Text, nil
	}
	if mediaTypes[mtype] {
		var m struct {
			Mimetype   string          `json:"mimetype"`
			Caption    string          `json:"caption"`
			FileName   string          `json:"fileName"`
			FileLength json.RawMessage `json:"fileLength"`
		}
		_ = json.Unmarshal(msg[mtype], &m)
		media := &Media{
			Kind:     strings.TrimSuffix(mtype, "Message"),
			Mimetype: m.Mimetype,
			FileName: m.FileName,
			FileSize: parseLong(m.FileLength),
			Base64:   jsonString(msg["base64"]),
		}
		return m.Caption, media
	}
	return "", nil
}

func directionOf(fromMe bool) string {
	if fromMe {
		return "out"
	}
	return "in"
}

// parseLong tolerates both a plain JSON number and a protobuf Long
// ({"low":..,"high":..,"unsigned":..}); bytes = low + high*2^32.
func parseLong(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	var lng struct {
		Low  int64 `json:"low"`
		High int64 `json:"high"`
	}
	if err := json.Unmarshal(raw, &lng); err == nil {
		return lng.Low + lng.High*(1<<32)
	}
	return 0
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func unixToTime(sec int64) time.Time {
	if sec <= 0 {
		return time.Now().UTC()
	}
	return time.Unix(sec, 0).UTC()
}

// deliveryRank orders delivery states so the worker only advances monotonically.
var deliveryRank = map[string]int{
	"queued":    0,
	"sent":      1,
	"delivered": 2,
	"read":      3,
	"failed":    4,
}

// DeliveryRank returns the monotonic rank of a delivery_state.
func DeliveryRank(state string) int { return deliveryRank[state] }

// MapDeliveryState maps a raw WhatsApp/Baileys status to an xchats delivery_state.
func MapDeliveryState(waStatus string) string {
	switch strings.ToUpper(waStatus) {
	case "ERROR":
		return "failed"
	case "PENDING", "SERVER_ACK":
		return "sent"
	case "DELIVERY_ACK":
		return "delivered"
	case "READ", "PLAYED":
		return "read"
	default:
		return ""
	}
}
