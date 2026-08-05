// Package whatsapp defines provider-neutral WhatsApp types: the events the
// adapter emits and the ports httpapi depends on. Nothing here imports
// go.mau.fi/whatsmeow (or any other provider client) — internal/whatsmeow is
// the sole importer of that library, so httpapi and the rest of the app only
// ever see the vocabulary defined in this package.
package whatsapp

import "time"

// Media describes one attachment on a MessageReceived event. It carries
// metadata only — the adapter downloads the bytes itself (it holds the blob
// store) and writes them out of band, so this type never grows a payload.
type Media struct {
	Kind     string // image|video|audio|document|sticker
	Mimetype string
	FileName string
	FileSize int64
}

// MessageReceived is one inbound message, or the fromMe echo of a message
// this account itself sent, translated from the provider's own event shape.
type MessageReceived struct {
	AccountID string // wa_accounts.id, already resolved by the adapter
	// ExternalID is the provider's own message id — the echo-collapse key
	// (UNIQUE(account_id, external_message_id) in wa_messages).
	ExternalID string
	// ChatJID is the identity the chat is keyed on: see ChatKey.
	ChatJID string
	// PhoneJID is the contact's stable phone identity; "" for a LID-only
	// contact (see the LID-only-contacts risk in the integration plan).
	PhoneJID string
	// LidJID is the contact's Linked-Device (privacy) identity; "" when the
	// contact exposes none.
	LidJID   string
	PushName string
	FromMe   bool
	IsGroup  bool
	// Kind is xchats' message_kind vocabulary: "conversation" for plain
	// text, else one of imageMessage|videoMessage|audioMessage|
	// documentMessage|stickerMessage.
	Kind      string
	Text      string
	Media     *Media
	Timestamp time.Time
	// Raw is a best-effort JSON snapshot for debugging; never parsed back.
	Raw []byte
}

// ReceiptUpdated advances (or attempts to advance) the delivery ladder for
// one or more previously exchanged messages. DeliveryState is always
// "delivered" or "read" here — a receipt never reports "queued", "sent", or
// "failed" (see internal/store.AdvanceDeliveryState's monotonic ladder).
type ReceiptUpdated struct {
	AccountID     string
	ExternalIDs   []string
	DeliveryState string
}

// ConnectionChanged reports a change in an account's live connection state.
type ConnectionChanged struct {
	AccountID string
	State     string // connecting|connected|disconnected|logged_out
	Reason    string // human-readable detail for logs; never parsed
}

// PairingUpdate is a snapshot of a QR pairing flow in progress, polled via
// Manager.PairStatus.
type PairingUpdate struct {
	SessionID string
	Status    string // qr_required|connected|timeout|error
	QRCode    string
	QRBase64  string
	AccountID string // set once Status == "connected"
	Message   string // human-readable detail on error/timeout
}
