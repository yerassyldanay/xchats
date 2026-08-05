package whatsmeow

import (
	"context"
	"fmt"

	wm "go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

// Sender implements messaging.ChannelSender over the manager's live
// per-account clients — the outbound half of the adapter, registered at the
// composition root via Manager.ChannelSender().
type Sender struct {
	manager *Manager
}

var _ messaging.ChannelSender = (*Sender)(nil)

// Send delivers out as a WhatsApp message — text, or one attachment when
// out.Media is set. out.AccountID selects which live client to send
// through; out.To is the provider's original conversation address. It may be
// a phone JID, LID, or group JID and must reach whatsmeow unchanged.
//
// The message id is pre-generated (SendRequestExtra.ID) rather than left to
// whatsmeow, because that same id is what lets the fromMe echo of this send
// collapse onto the row StampOutboundSent is about to stamp it on, instead
// of inserting a duplicate (UNIQUE(account_id, external_message_id)).
func (s *Sender) Send(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	mc, ok := s.manager.clientFor(out.AccountID)
	if !ok {
		return messaging.SendResult{}, fmt.Errorf("whatsmeow: account %s is not connected", out.AccountID)
	}
	to, err := parseSendTarget(out.To)
	if err != nil {
		return messaging.SendResult{}, err
	}

	var msg *waE2E.Message
	if out.Media != nil {
		msg, err = s.buildMediaMessage(ctx, mc, out.Media)
	} else {
		msg, err = buildOutboundMessage("", out.Text, "", "", "", nil)
	}
	if err != nil {
		return messaging.SendResult{}, err
	}

	id := mc.client.GenerateMessageID()
	resp, err := mc.client.SendMessage(ctx, to, msg, wm.SendRequestExtra{ID: id})
	if err != nil {
		return messaging.SendResult{}, err
	}
	return messaging.SendResult{ExternalID: resp.ID, Delivered: resp.ID != ""}, nil
}

// buildMediaMessage resolves out.Media's blob, uploads it to WhatsApp's
// media servers, and wraps the upload reference in the right proto message.
func (s *Sender) buildMediaMessage(ctx context.Context, mc *managedClient, media *messaging.OutboundMedia) (*waE2E.Message, error) {
	if s.manager.cfg.Blob == nil {
		return nil, fmt.Errorf("whatsmeow: media send requires a blob store")
	}
	data, meta, err := s.manager.cfg.Blob.Get(media.BlobID)
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: read media %q: %w", media.BlobID, err)
	}
	kind := orFallback(media.Kind, meta.MediaType)
	mimetype := orFallback(media.Mimetype, meta.Mimetype)
	fileName := orFallback(media.FileName, meta.FileName)

	uploaded, err := mc.client.Upload(ctx, data, mediaTypeFor(kind))
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: upload media: %w", err)
	}
	return buildOutboundMessage(kind, "", mimetype, fileName, media.Caption, &uploaded)
}

// parseSendTarget coerces a bare phone number (or an already-formed JID
// string) into a types.JID to send to.
func parseSendTarget(to string) (types.JID, error) {
	jid, err := types.ParseJID(config.CanonicalJID(to))
	if err != nil {
		return types.JID{}, fmt.Errorf("whatsmeow: invalid destination %q: %w", to, err)
	}
	return jid, nil
}

func orFallback(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
