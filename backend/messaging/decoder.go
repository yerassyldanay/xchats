package messaging

import (
	"context"
	"fmt"
)

// InboundDecoder turns channel-specific input into a normalized Message. ok is
// false (never an error) when input is not a message-bearing event — e.g. a
// WhatsApp status/connection-update webhook, which is an expected, non-error
// shape, not a failure to decode.
type InboundDecoder interface {
	Decode(ctx context.Context, input any) (msg Message, ok bool, err error)
}

// DecoderRegistry resolves an InboundDecoder by channel. Registered once at
// the composition root; looking up an unregistered channel fails explicitly
// rather than silently falling back to some default decoder.
type DecoderRegistry struct {
	decoders map[Channel]InboundDecoder
}

// NewDecoderRegistry returns an empty registry.
func NewDecoderRegistry() *DecoderRegistry {
	return &DecoderRegistry{decoders: make(map[Channel]InboundDecoder)}
}

// Register associates a decoder with a channel, overwriting any prior one.
func (r *DecoderRegistry) Register(ch Channel, d InboundDecoder) {
	r.decoders[ch] = d
}

// Decoder returns the decoder registered for ch, or an error if none is.
func (r *DecoderRegistry) Decoder(ch Channel) (InboundDecoder, error) {
	d, ok := r.decoders[ch]
	if !ok {
		return nil, fmt.Errorf("messaging: no inbound decoder registered for channel %q", ch)
	}
	return d, nil
}
