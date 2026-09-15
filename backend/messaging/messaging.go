// Package messaging defines the channel-neutral contracts every inbound
// message is decoded into and every outbound send is expressed through.
// Neither this package nor its Message/OutboundMessage/SendResult types
// depend on any specific provider (whatsmeow, OpenRouter, ...), on PostgreSQL,
// or on an HTTP framework — channel adapters implementing InboundDecoder and
// ChannelSender live in backend/internal and are wired together only at the
// composition root (cmd/xchats). Adding a channel means registering a new
// adapter there; this package never needs a code change.
package messaging

import (
	"errors"
	"time"
)

// Channel names a message's originating or destination channel.
type Channel string

const (
	ChannelWhatsApp  Channel = "whatsapp"
	ChannelSimulator Channel = "simulator"
	ChannelTelegram  Channel = "telegram"
	// ChannelInstagram, ChannelMessenger and ChannelWhatsAppCloud are the
	// official Meta channels: Instagram Direct, Facebook Messenger, and the
	// WhatsApp Cloud API. They run alongside ChannelWhatsApp (whatsmeow, the
	// unofficial QR-paired leg) rather than replacing it.
	ChannelInstagram     Channel = "instagram"
	ChannelMessenger     Channel = "messenger"
	ChannelWhatsAppCloud Channel = "whatsapp_cloud"
)

// ErrOutsideServiceWindow is returned by a ChannelSender.Send when the
// provider's free-form customer-service window (24 hours since the
// customer's last inbound message, on every Meta channel) has closed and
// the outbound is not a template message. It is a distinct, expected
// failure mode — not a transport error — so callers can render it as its
// own delivery_state/failure_reason rather than a generic send failure.
var ErrOutsideServiceWindow = errors.New("messaging: outside the provider's free-form messaging window")

// ErrRecipientUnreachable is returned by a ChannelSender.Send when the
// provider has permanently rejected this specific destination (an invalid
// or deactivated number, a hard bounce) — never a transient network/rate
// hiccup. Like ErrOutsideServiceWindow, this is an expected failure mode a
// retry cannot fix: callers finalize it immediately as a permanent failure
// rather than stepping it through the transient-retry ladder.
var ErrRecipientUnreachable = errors.New("messaging: recipient is permanently unreachable")

// The sentinels below give ChannelSender implementations (and
// internal/outbound.Deliver itself) a channel-neutral vocabulary for the
// failure modes a caller needs to classify without guessing from an
// adapter's free-text error string — see internal/campaign's error
// classifier, the first consumer. An adapter that cannot attribute a
// failure to any of these (or to ErrOutsideServiceWindow/
// ErrRecipientUnreachable above) should leave it unwrapped: an unrecognized
// error is classified as an unknown reason, never guessed from its text.

// ErrChannelUnavailable is returned when no ChannelSender is registered for
// the destination channel at all — a configuration/wiring problem, not a
// live connection's own state.
var ErrChannelUnavailable = errors.New("messaging: no sender is registered for this channel")

// ErrAccountDisconnected is returned when the sending account has no live
// provider connection right now (e.g. WhatsApp's own client is not
// connected). Distinct from ErrChannelUnavailable: the channel itself is
// wired up, this one specific account just is not currently reachable.
var ErrAccountDisconnected = errors.New("messaging: sending account is not connected")

// ErrInvalidRecipient is returned when the destination address itself is
// malformed (cannot be parsed into the provider's own addressing format) —
// a local, pre-flight rejection, never a provider round trip. Like
// ErrRecipientUnreachable, a retry cannot fix a malformed address, so
// callers treat the two identically (permanent, no retry).
var ErrInvalidRecipient = errors.New("messaging: destination address is invalid")

// ErrMessageBuildFailed is returned when building the outbound message
// content itself fails (e.g. reading or uploading attached media) — before
// any send request ever reached the provider.
var ErrMessageBuildFailed = errors.New("messaging: failed to build the outbound message content")

// ErrSendTimeout is returned when a send request was made but no response
// was received in time. This is the AMBIGUOUS case: the provider may have
// received and even accepted the message, so callers must not treat this as
// a confirmed failure automatically eligible for the same retry ladder as
// other transient errors — see internal/campaign.Runner.finalize's own doc
// comment.
var ErrSendTimeout = errors.New("messaging: send timed out waiting for a provider response")

// ErrNetwork is returned for a network-level failure that occurred before
// any request was known to reach the provider (connection refused, DNS
// failure, and similar) — unlike ErrSendTimeout, safe to treat as an
// ordinary transient error, since nothing indicates the provider ever saw
// the request.
var ErrNetwork = errors.New("messaging: network error communicating with the provider")

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
