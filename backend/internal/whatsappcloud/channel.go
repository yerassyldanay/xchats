package whatsappcloud

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/messaging"
)

// AccountSource resolves an account's WA Cloud specifics at send time: its
// phone_number_id (the Graph API path segment) and its business token. Two
// methods, not one, because — unlike a Telegram bot token, which already
// implies which bot is sending — a WA Cloud business token authenticates a
// whole WABA, and a send must ALSO name which of its numbers to send from.
// Satisfied by *store.Store without this package importing internal/store
// at all, exactly mirroring telegram.TokenSource.
type AccountSource interface {
	ChannelExternalAccountID(ctx context.Context, accountID uuid.UUID) (string, error)
	ChannelCredentialsSecret(ctx context.Context, accountID uuid.UUID) (string, error)
}

// MediaSource resolves what an outbound attachment needs to mint a signed
// public URL, from the outbound message's OWN id: the
// channel_message_media row's id (distinct from messaging.OutboundMedia's
// BlobID, which names the bytes in internal/blob — UpsertOutboundMedia
// already created this row, with storage_key = BlobID, before the send was
// ever enqueued) and that row's owning organization, resolved in one query
// so the sender never has to trust a caller-supplied org. Satisfied by
// *store.Store's ChannelOutboundMediaForSigning.
type MediaSource interface {
	ChannelOutboundMediaForSigning(ctx context.Context, messageID uuid.UUID) (mediaID, orgID uuid.UUID, err error)
}

// MediaURLSigner mints the short-lived, org-scoped public URL an outbound
// attachment is sent by — see internal/inboxmedia's package doc for why
// this cannot be mcpauth's own media-read signer.
type MediaURLSigner interface {
	Sign(orgID, mediaID uuid.UUID, expiresAtUnix int64) string
}

// mediaLinkTTL is how long a signed outbound-media URL stays fetchable —
// comfortably longer than Meta needs to retrieve it once (§7: 10 minutes).
const mediaLinkTTL = 10 * time.Minute

// ChannelSender wraps a Client behind messaging.ChannelSender. out.To is the
// customer's E.164 phone number (as text — the channel-neutral contract
// carries an opaque string); the sending phone number and its token are
// resolved from out.AccountID via Accounts, never carried on
// OutboundMessage itself.
type ChannelSender struct {
	Client   *Client
	Accounts AccountSource
	Media    MediaSource
	Signer   MediaURLSigner
	// MediaBaseURL is cfg.ResolvedAPIBaseURL() + "/meta/api/v1/media" — never
	// built from a request Host header (see config.ResolvedAPIBaseURL's own
	// doc comment on why every URL this server mints reads through it).
	MediaBaseURL string
}

var _ messaging.ChannelSender = (*ChannelSender)(nil)

// NewChannelSender builds a ChannelSender.
func NewChannelSender(client *Client, accounts AccountSource, media MediaSource, signer MediaURLSigner, mediaBaseURL string) *ChannelSender {
	return &ChannelSender{Client: client, Accounts: accounts, Media: media, Signer: signer, MediaBaseURL: mediaBaseURL}
}

// Send resolves the sending number/token at the moment of the call, then
// dispatches to a text or media send.
func (s *ChannelSender) Send(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	accountID, err := uuid.Parse(out.AccountID)
	if err != nil {
		return messaging.SendResult{}, err
	}
	phoneNumberID, err := s.Accounts.ChannelExternalAccountID(ctx, accountID)
	if err != nil {
		return messaging.SendResult{}, err
	}
	token, err := s.Accounts.ChannelCredentialsSecret(ctx, accountID)
	if err != nil {
		return messaging.SendResult{}, err
	}

	var res SendResult
	if out.Media != nil {
		res, err = s.sendMedia(ctx, phoneNumberID, out, token)
	} else {
		res, err = s.Client.SendText(ctx, phoneNumberID, out.To, out.Text, token)
	}
	if err != nil {
		return messaging.SendResult{}, err
	}
	var externalID string
	if len(res.Messages) > 0 {
		externalID = res.Messages[0].ID
	}
	return messaging.SendResult{ExternalID: externalID, Delivered: externalID != ""}, nil
}

// sendMedia mints a short-lived, org-scoped public URL for the attachment
// (WhatsApp Cloud never receives our bytes directly, only a link it fetches
// itself) and sends by link.
func (s *ChannelSender) sendMedia(ctx context.Context, phoneNumberID string, out messaging.OutboundMessage, token string) (SendResult, error) {
	messageID, err := uuid.Parse(out.MessageID)
	if err != nil {
		return SendResult{}, err
	}
	mediaID, orgID, err := s.Media.ChannelOutboundMediaForSigning(ctx, messageID)
	if err != nil {
		return SendResult{}, err
	}
	exp := time.Now().Add(mediaLinkTTL).Unix()
	sig := s.Signer.Sign(orgID, mediaID, exp)
	link := s.MediaBaseURL + "/" + mediaID.String() + "?token=" + sig + "&exp=" + strconv.FormatInt(exp, 10)

	kind := MediaKind(out.Media.Kind)
	switch kind {
	case MediaImage, MediaVideo, MediaAudio, MediaDocument:
	default:
		// An unrecognized kind still delivers, as a document, rather than
		// rejecting the send outright.
		kind = MediaDocument
	}
	return s.Client.SendMedia(ctx, phoneNumberID, out.To, kind, link, out.Media.Caption, out.Media.FileName, token)
}
