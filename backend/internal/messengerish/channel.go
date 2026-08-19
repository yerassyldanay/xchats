package messengerish

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/messaging"
)

// AccountSource resolves an account's provider specifics at send time — its
// own Graph-API id (an IG user id or a Page id) and its access token.
// Satisfied by *store.Store without this package importing internal/store
// at all — identical shape to whatsappcloud.AccountSource (and *store.Store
// already implements both from the same two methods).
type AccountSource interface {
	ChannelExternalAccountID(ctx context.Context, accountID uuid.UUID) (string, error)
	ChannelCredentialsSecret(ctx context.Context, accountID uuid.UUID) (string, error)
}

// MediaSource resolves what an outbound attachment needs to mint a signed
// public URL — identical shape to whatsappcloud.MediaSource; see that
// type's doc comment for the full reasoning (BlobID vs the
// channel_message_media row's own id).
type MediaSource interface {
	ChannelOutboundMediaForSigning(ctx context.Context, messageID uuid.UUID) (mediaID, orgID uuid.UUID, err error)
}

// MediaURLSigner mints the short-lived, org-scoped public URL an outbound
// attachment is sent by — see internal/inboxmedia's package doc.
type MediaURLSigner interface {
	Sign(orgID, mediaID uuid.UUID, expiresAtUnix int64) string
}

// mediaLinkTTL mirrors whatsappcloud's own — comfortably longer than Meta
// needs to fetch it once.
const mediaLinkTTL = 10 * time.Minute

// ChannelSender wraps a Client behind messaging.ChannelSender for ONE of
// Instagram or Messenger — InstagramHost picks which Graph host/OAuth
// product this instance's sends resolve against; the two channels share one
// underlying Client (and one meta.Client transport) but get separate
// ChannelSender registrations (see cmd/xchats's SenderRegistry wiring),
// exactly because that flag differs per channel.
type ChannelSender struct {
	Client        *Client
	Accounts      AccountSource
	Media         MediaSource
	Signer        MediaURLSigner
	InstagramHost bool
	// MediaBaseURL is cfg.ResolvedAPIBaseURL() + "/meta/api/v1/media" — see
	// whatsappcloud.ChannelSender.MediaBaseURL's identical doc comment.
	MediaBaseURL string
}

var _ messaging.ChannelSender = (*ChannelSender)(nil)

// NewChannelSender builds a ChannelSender.
func NewChannelSender(client *Client, accounts AccountSource, media MediaSource, signer MediaURLSigner, instagramHost bool, mediaBaseURL string) *ChannelSender {
	return &ChannelSender{Client: client, Accounts: accounts, Media: media, Signer: signer, InstagramHost: instagramHost, MediaBaseURL: mediaBaseURL}
}

// Send resolves the sending account/token at the moment of the call, then
// dispatches to a text or media send — mirrors whatsappcloud.ChannelSender.
// Send's identical shape.
func (s *ChannelSender) Send(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	accountID, err := uuid.Parse(out.AccountID)
	if err != nil {
		return messaging.SendResult{}, err
	}
	externalAccountID, err := s.Accounts.ChannelExternalAccountID(ctx, accountID)
	if err != nil {
		return messaging.SendResult{}, err
	}
	token, err := s.Accounts.ChannelCredentialsSecret(ctx, accountID)
	if err != nil {
		return messaging.SendResult{}, err
	}

	var res SendResult
	if out.Media != nil {
		res, err = s.sendMedia(ctx, externalAccountID, out, token)
	} else {
		res, err = s.Client.SendText(ctx, s.InstagramHost, externalAccountID, out.To, out.Text, token)
	}
	if err != nil {
		return messaging.SendResult{}, err
	}
	return messaging.SendResult{ExternalID: res.MessageID, Delivered: res.MessageID != ""}, nil
}

// sendMedia mints a short-lived, org-scoped public URL for the attachment
// (Instagram/Messenger fetch our bytes themselves, never receive an
// upload) and sends by link — mirrors whatsappcloud.ChannelSender.
// sendMedia's identical shape.
//
// Unlike WhatsApp Cloud/Telegram, the Send API's attachment payload has no
// caption field — a caption can only reach the customer as a second,
// separate text bubble. When present, it's sent as a best-effort follow-up
// after the attachment: its failure doesn't undo (or get reflected in) the
// media send that already reached the customer, so the returned SendResult
// is always the attachment's own.
func (s *ChannelSender) sendMedia(ctx context.Context, externalAccountID string, out messaging.OutboundMessage, token string) (SendResult, error) {
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
	case MediaImage, MediaVideo, MediaAudio, MediaFile:
	default:
		// An unrecognized kind still delivers, as a generic file, rather
		// than rejecting the send outright — mirrors whatsappcloud's
		// identical fallback.
		kind = MediaFile
	}
	res, err := s.Client.SendMedia(ctx, s.InstagramHost, externalAccountID, out.To, kind, link, token)
	if err != nil {
		return res, err
	}
	if caption := strings.TrimSpace(out.Media.Caption); caption != "" {
		if _, err := s.Client.SendText(ctx, s.InstagramHost, externalAccountID, out.To, caption, token); err != nil {
			slog.Warn("messengerish: caption follow-up send failed", "account_id", out.AccountID, "err", err)
		}
	}
	return res, nil
}
