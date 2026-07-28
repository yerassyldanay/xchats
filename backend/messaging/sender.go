package messaging

import (
	"context"
	"fmt"
)

// ChannelSender delivers an approved outbound message through its channel.
type ChannelSender interface {
	Send(ctx context.Context, out OutboundMessage) (SendResult, error)
}

// SenderRegistry resolves a ChannelSender by channel. Registered once at the
// composition root; looking up an unregistered channel fails explicitly
// rather than silently dropping the send or guessing another channel.
type SenderRegistry struct {
	senders map[Channel]ChannelSender
}

// NewSenderRegistry returns an empty registry.
func NewSenderRegistry() *SenderRegistry {
	return &SenderRegistry{senders: make(map[Channel]ChannelSender)}
}

// Register associates a sender with a channel, overwriting any prior one.
func (r *SenderRegistry) Register(ch Channel, s ChannelSender) {
	r.senders[ch] = s
}

// Sender returns the sender registered for ch, or an error if none is.
func (r *SenderRegistry) Sender(ch Channel) (ChannelSender, error) {
	s, ok := r.senders[ch]
	if !ok {
		return nil, fmt.Errorf("messaging: no channel sender registered for channel %q", ch)
	}
	return s, nil
}
