// Package messaging defines the channel-neutral contracts every inbound
// message is decoded into and every outbound send is expressed through.
// Neither this package nor its Message/OutboundMessage/SendResult types
// depend on any specific provider (whatsmeow, OpenRouter, ...), on PostgreSQL,
// or on an HTTP framework — channel adapters implementing InboundDecoder and
// ChannelSender live in backend/internal and are wired together only at the
// composition root (cmd/xchats). Adding a channel means registering a new
// adapter there; this package never needs a code change.
package messaging

import "time"

// Channel names a message's originating or destination channel.
type Channel string

const (
	ChannelWhatsApp  Channel = "whatsapp"
	ChannelSimulator Channel = "simulator"
	ChannelTelegram  Channel = "telegram"
)

// Message is one normalized inbound (or outbound) message, independent of
// which channel produced it.
type Message struct {
	ExternalID     string // the provider's own id for this message, if any
	ConversationID string
	AccountID      string
	OrganizationID string
	Channel        Channel
	Direction      string // "in" | "out"
	Text           string
	Timestamp      time.Time
}

// OutboundMessage is a message approved for delivery through a channel's
// ChannelSender. To is an opaque, channel-specific routing hint (for
// WhatsApp: the destination JID/phone number) resolved by the caller before
// Send is called; AccountID is what an adapter resolves ITS OWN sending
// identity from (a live whatsmeow client, a Telegram bot token, ...) — a
// channel that needs neither (the simulator) simply ignores them, so no
// provider-specific concept has to leak into this type itself.
type OutboundMessage struct {
	MessageID      string // the already-persisted outbound message row's own id
	ConversationID string
	AccountID      string
	Channel        Channel
	Text           string
	To             string
	// Media, when set, makes this an attachment send rather than a text send.
	Media *OutboundMedia
}

// OutboundMedia is one attachment on an outbound message. BlobID names the
// bytes in our own storage (the adapter dereferences it), so this contract
// carries a reference rather than a payload and a channel that can stream is
// free to do so. ProviderRef is the escape hatch for content the provider
// already hosts — a Telegram file_id from an inbound message — letting a
// re-send skip the upload entirely.
type OutboundMedia struct {
	BlobID      string
	Kind        string // image|video|audio|document
	Mimetype    string
	FileName    string
	Caption     string
	ProviderRef string
}

// SendResult is the outcome of a ChannelSender.Send call.
type SendResult struct {
	ExternalID string // the provider's id for the sent message (real or synthetic)
	Delivered  bool
}
