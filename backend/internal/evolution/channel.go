package evolution

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/normalize"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// ChannelDecoder wraps internal/normalize behind messaging.InboundDecoder.
//
// It carries no real traffic this milestone: the Evolution webhook route keeps
// calling normalize.ParseEnvelope and internal/worker's handleWaEvent directly
// (account/chat resolution is a database operation this pure decoder has no
// way to perform), so nothing routes an inbound WhatsApp event through this
// type today. It exists so the WhatsApp channel has a real InboundDecoder
// implementation ready to register at the composition root, matching the
// architecture every other channel follows.
type ChannelDecoder struct{}

// NewChannelDecoder returns a WhatsApp InboundDecoder.
func NewChannelDecoder() *ChannelDecoder { return &ChannelDecoder{} }

// Decode expects input to be the raw Evolution webhook body ([]byte).
func (d *ChannelDecoder) Decode(ctx context.Context, input any) (messaging.Message, bool, error) {
	raw, ok := input.([]byte)
	if !ok {
		return messaging.Message{}, false, fmt.Errorf("evolution: ChannelDecoder expects []byte input, got %T", input)
	}
	env, err := normalize.ParseEnvelope(raw)
	if err != nil {
		return messaging.Message{}, false, err
	}
	m, err := env.Message()
	if err != nil {
		return messaging.Message{}, false, err
	}
	if m == nil {
		return messaging.Message{}, false, nil
	}
	return messaging.Message{
		ExternalID: m.EvolutionMessageID,
		Channel:    messaging.ChannelWhatsApp,
		Direction:  m.Direction,
		Text:       m.Text,
		Timestamp:  m.Timestamp,
	}, true, nil
}

// ChannelSender wraps a Client behind messaging.ChannelSender: out.Route is the
// sending account's Evolution instance name, out.To is the destination phone
// number — both resolved by the caller, so JIDs and instance names never
// become part of the channel-neutral OutboundMessage contract itself.
//
// Blobs resolves out.Media.BlobID to bytes. It may be nil in a text-only
// wiring; a media send then fails explicitly rather than sending an empty file.
type ChannelSender struct {
	Client Client
	Blobs  blob.Store
}

// NewChannelSender returns a WhatsApp ChannelSender over client.
func NewChannelSender(client Client, blobs blob.Store) *ChannelSender {
	return &ChannelSender{Client: client, Blobs: blobs}
}

// Send delivers out as a WhatsApp message — text, or one attachment when
// out.Media is set. Media used to bypass the registry entirely (a direct
// Evolution call in the worker); routing it here is what lets every channel
// share one outbound path.
func (s *ChannelSender) Send(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	if out.Media != nil {
		return s.sendMedia(ctx, out)
	}
	res, err := s.Client.SendText(ctx, out.Route, out.To, out.Text)
	if err != nil {
		return messaging.SendResult{}, err
	}
	return messaging.SendResult{ExternalID: res.KeyID, Delivered: res.KeyID != ""}, nil
}

func (s *ChannelSender) sendMedia(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	if s.Blobs == nil {
		return messaging.SendResult{}, fmt.Errorf("evolution: media send requires a blob store")
	}
	data, meta, err := s.Blobs.Get(out.Media.BlobID)
	if err != nil {
		return messaging.SendResult{}, fmt.Errorf("evolution: read media %q: %w", out.Media.BlobID, err)
	}
	kind := orFallback(out.Media.Kind, meta.MediaType)
	mimetype := orFallback(out.Media.Mimetype, meta.Mimetype)
	fileName := orFallback(out.Media.FileName, meta.FileName)

	res, err := s.Client.SendMedia(ctx, out.Route, out.To, kind, mimetype,
		base64.StdEncoding.EncodeToString(data), fileName, out.Media.Caption)
	if err != nil {
		return messaging.SendResult{}, err
	}
	return messaging.SendResult{ExternalID: res.KeyID, Delivered: res.KeyID != ""}, nil
}

func orFallback(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
