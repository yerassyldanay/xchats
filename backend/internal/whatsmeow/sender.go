package whatsmeow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"

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
// The message id is DERIVED from out.MessageID (stableMessageID), not
// whatsmeow's own GenerateMessageID(), for two reasons: it is what lets the
// fromMe echo of this send collapse onto the row StampOutboundSent is about
// to stamp it on instead of inserting a duplicate (UNIQUE(account_id,
// external_message_id)), AND — since it is deterministic — a caller that
// re-attempts the SAME logical message (a campaign retry after an ambiguous
// timeout, see internal/campaign.Runner's own doc comment) sends WhatsApp
// the identical wire message id every time. WhatsApp's protocol uses the
// message id to deduplicate on the receiving side, so a resend that
// actually reaches a device which already saw the first attempt is a
// no-op there rather than a visible second message — real provider
// idempotency, not just a local bookkeeping trick.
//
// Every failure path returns an error wrapping one of messaging.Err* (or
// leaves it unwrapped when no such sentinel applies) — see classifySendErr
// and this package's own errors.go-adjacent wrapping below. Nothing here
// pattern-matches an error's free-text message: classification is always
// against whatsmeow's own typed/sentinel errors, so an unrecognized failure
// mode is reported as unknown rather than guessed.
func (s *Sender) Send(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	mc, ok := s.manager.clientFor(out.AccountID)
	if !ok {
		return messaging.SendResult{}, fmt.Errorf("whatsmeow: account %s is not connected: %w", out.AccountID, messaging.ErrAccountDisconnected)
	}
	to, err := parseSendTarget(out.To)
	if err != nil {
		return messaging.SendResult{}, fmt.Errorf("%w: %v", messaging.ErrInvalidRecipient, err)
	}

	var msg *waE2E.Message
	if out.Media != nil {
		msg, err = s.buildMediaMessage(ctx, mc, out.Media)
	} else {
		msg, err = buildOutboundMessage("", out.Text, "", "", "", nil)
	}
	if err != nil {
		return messaging.SendResult{}, fmt.Errorf("%w: %v", messaging.ErrMessageBuildFailed, err)
	}

	id := stableMessageID(out.MessageID)
	resp, err := mc.client.SendMessage(ctx, to, msg, wm.SendRequestExtra{ID: id})
	if err != nil {
		return messaging.SendResult{}, classifySendErr(ctx, err)
	}
	return messaging.SendResult{ExternalID: resp.ID, Delivered: resp.ID != ""}, nil
}

// stableMessageID derives a WhatsApp-shaped message id deterministically
// from our own already-persisted message row's id: the same 16-uppercase-hex
// -characters-after-a-prefix shape wm.GenerateMessageID produces (send.go),
// but a pure function of appMessageID rather than fresh randomness — see
// Send's own doc comment for why that determinism matters across retries.
func stableMessageID(appMessageID string) types.MessageID {
	sum := sha256.Sum256([]byte(appMessageID))
	return wm.WebMessageIDPrefix + types.MessageID(strings.ToUpper(hex.EncodeToString(sum[:8])))
}

// classifySendErr wraps a raw error from Client.SendMessage in the
// channel-neutral messaging.Err* sentinel that matches ACTUAL evidence
// whatsmeow itself provides (a typed error, an exported sentinel, or ctx's
// own deadline) — never a substring match on err.Error(). An error this
// function does not recognize is returned unwrapped, which internal/
// campaign's classifier reports as an unknown reason rather than a guess.
func classifySendErr(ctx context.Context, err error) error {
	switch {
	case ctx.Err() != nil && errors.Is(ctx.Err(), context.DeadlineExceeded):
		return fmt.Errorf("%w: %v", messaging.ErrSendTimeout, err)
	case errors.Is(err, wm.ErrIQTimedOut), errors.Is(err, wm.ErrMessageTimedOut):
		return fmt.Errorf("%w: %v", messaging.ErrSendTimeout, err)
	case errors.Is(err, wm.ErrNotConnected), errors.Is(err, wm.ErrClientIsNil):
		return fmt.Errorf("%w: %v", messaging.ErrAccountDisconnected, err)
	case isDisconnectedIQ(err):
		return fmt.Errorf("%w: %v", messaging.ErrAccountDisconnected, err)
	case errors.Is(err, wm.ErrNoSession), errors.Is(err, wm.ErrRecipientADJID), errors.Is(err, wm.ErrUnknownServer):
		return fmt.Errorf("%w: %v", messaging.ErrInvalidRecipient, err)
	case isIQError(err), errors.Is(err, wm.ErrServerReturnedError):
		return fmt.Errorf("%w: %v", messaging.ErrRecipientUnreachable, err)
	default:
		var netErr net.Error
		if errors.As(err, &netErr) {
			if netErr.Timeout() {
				return fmt.Errorf("%w: %v", messaging.ErrSendTimeout, err)
			}
			return fmt.Errorf("%w: %v", messaging.ErrNetwork, err)
		}
		return err
	}
}

// isDisconnectedIQ reports whether err is (or wraps) a
// *wm.DisconnectedError — the websocket dropped before this specific info
// query returned a response, whatsmeow's own distinct shape from a plain
// "not connected right now" (wm.ErrNotConnected).
func isDisconnectedIQ(err error) bool {
	var de *wm.DisconnectedError
	return errors.As(err, &de)
}

// isIQError reports whether err is (or wraps) a *wm.IQError — a stanza-level
// error the WhatsApp server itself returned for this request (a numeric
// Code + optional human text), as opposed to a transport-level failure.
// Treated as provider rejection: the server received and explicitly refused
// the request, which a retry of the same content cannot fix.
func isIQError(err error) bool {
	var iqe *wm.IQError
	return errors.As(err, &iqe)
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
