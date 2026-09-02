// Package simulator implements the simulator channel adapter and (its HTTP API
// and CLI client) the authenticated entry point that injects synthetic channel
// messages into the same ingestion/response path WhatsApp uses. The simulator
// is not a knowledge-base editor — it only creates channel-neutral messages
// and conversations from stable, caller-chosen refs.
package simulator

import (
	"context"
	"fmt"
	"strings"

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
// persisted outbound message's own id, and a deterministic simulated
// outcome (see Outcome) derived from the destination itself. Actually
// persisting the outbound message row is the caller's job — the same
// delivery-state persistence step every channel shares — this sender only
// reports the outcome a real network call would have reported.
//
// Send runs through the exact same internal/campaign.Runner ->
// internal/outbound.Deliver path every other channel uses (see
// cmd/xchats's SenderRegistry wiring) — there is no separate simulator
// pipeline. The only thing this sender does differently from a real
// provider is never touch the network; a campaign, its recipients, its
// pacing, and its template rendering behave identically whether the
// account is Simulator or a live channel.
type ChannelSender struct{}

// NewChannelSender returns a simulator ChannelSender.
func NewChannelSender() *ChannelSender { return &ChannelSender{} }

// Send never calls the network. A small, fixed fraction of destinations
// simulate a permanent provider rejection (Outcome below); every other
// destination succeeds immediately — ReceiptSimulator (receipts.go) is what
// later advances a successful send's delivery_state from 'sent' through
// 'delivered' and, for most of them, 'read', mirroring how a real
// provider's own delivery receipts arrive asynchronously after the initial
// send accepts.
func (s *ChannelSender) Send(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	if SimulatedOutcome(out.To) == OutcomeFailed {
		return messaging.SendResult{}, messaging.ErrRecipientUnreachable
	}
	return messaging.SendResult{ExternalID: "sim-" + out.MessageID, Delivered: true}, nil
}

// Outcome classifies how a simulated send to `destination` (the channel-
// neutral OutboundMessage.To / a wa_chats.remote_jid — a phone-JID like
// "77011234567@s.whatsapp.net") will play out. It is a PURE function of the
// destination string alone, deliberately: the same number always produces
// the same outcome, both here (Send, deciding success/failure up front) and
// in ReceiptSimulator's later sweep (deciding whether a successful send
// eventually reaches 'read') — with nothing extra to persist or thread
// between the two, and a reproducible way for an operator (or an e2e test)
// to deliberately exercise the failure/no-read paths by choosing a number
// ending in the right digit.
//
// The split — keyed on the destination's last digit — is a fixed,
// documented convention, not randomness: digit 0 fails outright (~10% of
// destinations), 1-2 deliver but are never read (~20%), 3-9 are read
// (~70%), a realistic-looking mix without any test ever being flaky.
type Outcome int

const (
	// OutcomeRead: the common case — sent, then delivered, then read.
	OutcomeRead Outcome = iota
	// OutcomeDeliveredOnly: sent and delivered, but the simulated recipient
	// never opens it — delivery_state stalls at 'delivered'.
	OutcomeDeliveredOnly
	// OutcomeFailed: Send itself rejects with messaging.ErrRecipientUnreachable —
	// the campaign recipient finalizes as permanently failed, never sent at all.
	OutcomeFailed
)

func lastDestinationDigit(destination string) (digit int, ok bool) {
	user, _, _ := strings.Cut(destination, "@")
	for i := len(user) - 1; i >= 0; i-- {
		if user[i] >= '0' && user[i] <= '9' {
			return int(user[i] - '0'), true
		}
	}
	return 0, false
}

// SimulatedOutcome computes destination's fixed Outcome — see Outcome's own
// doc comment. A destination with no recoverable digit (malformed input)
// defaults to OutcomeRead rather than failing, so an edge case in the
// input never masquerades as a deliberately-chosen failure test case.
func SimulatedOutcome(destination string) Outcome {
	digit, ok := lastDestinationDigit(destination)
	if !ok {
		return OutcomeRead
	}
	switch {
	case digit == 0:
		return OutcomeFailed
	case digit == 1 || digit == 2:
		return OutcomeDeliveredOnly
	default:
		return OutcomeRead
	}
}
