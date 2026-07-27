// Package simulator implements the simulator channel adapter and (its HTTP API
// and CLI client) the authenticated entry point that injects synthetic channel
// messages into the same ingestion/response path WhatsApp uses. The simulator
// is not a knowledge-base editor — it only creates channel-neutral messages
// and conversations from stable, caller-chosen refs.
package simulator

import (
	"context"
	"fmt"

	"github.com/yerassyldanay/xchats/backend/messaging"
)

// InboundRequest is the simulator's own (non-provider) inbound shape: a
// contact and conversation identified by stable, human-chosen refs rather
// than a provider-issued id, plus the message text.
type InboundRequest struct {
	ContactRef      string
	ConversationRef string
	Text            string
}

// ChannelDecoder builds a normalized Message directly from an InboundRequest —
// there is no provider and no wire format to parse. Resolving (or creating)
// the account/contact/conversation the stable refs name is a database
// operation performed by the caller (the simulator HTTP handler), not by this
// pure decoder.
type ChannelDecoder struct{}

// NewChannelDecoder returns a simulator InboundDecoder.
func NewChannelDecoder() *ChannelDecoder { return &ChannelDecoder{} }

// Decode expects input to be an InboundRequest.
func (d *ChannelDecoder) Decode(ctx context.Context, input any) (messaging.Message, bool, error) {
	req, ok := input.(InboundRequest)
	if !ok {
		return messaging.Message{}, false, fmt.Errorf("simulator: ChannelDecoder expects InboundRequest input, got %T", input)
	}
	if req.Text == "" {
		return messaging.Message{}, false, nil
	}
	return messaging.Message{
		Channel:   messaging.ChannelSimulator,
		Direction: "in",
		Text:      req.Text,
	}, true, nil
}

// ChannelSender records an approved outbound send with no network call: it
// reports a deterministic synthetic external id, derived from the already-
// persisted outbound message's own id, and immediate delivery. Actually
// persisting the outbound message row is the caller's job — the same
// delivery-state persistence step every channel shares — this sender only
// reports the outcome a real network call would have reported.
type ChannelSender struct{}

// NewChannelSender returns a simulator ChannelSender.
func NewChannelSender() *ChannelSender { return &ChannelSender{} }

// Send never calls the network; it always reports success.
func (s *ChannelSender) Send(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	return messaging.SendResult{ExternalID: "sim-" + out.MessageID, Delivered: true}, nil
}
