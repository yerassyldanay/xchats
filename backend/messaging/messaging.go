// Package messaging defines the channel-neutral contracts every inbound
// message is decoded into and every outbound send is expressed through.
// Neither this package nor its Message/OutboundMessage/SendResult types
// depend on any specific provider (Evolution, OpenRouter, ...), on PostgreSQL,
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
// ChannelSender. To and Route are opaque, channel-specific routing hints (for
// WhatsApp: the destination JID/phone number and the sending Evolution
// instance name) resolved by the caller before Send is called — a channel that
// doesn't need them (the simulator) simply ignores them, so no provider-
// specific concept has to leak into this type itself.
type OutboundMessage struct {
	MessageID      string // the already-persisted outbound message row's own id
	ConversationID string
	AccountID      string
	Channel        Channel
	Text           string
	To             string
	Route          string
}

// SendResult is the outcome of a ChannelSender.Send call.
type SendResult struct {
	ExternalID string // the provider's id for the sent message (real or synthetic)
	Delivered  bool
}
