package telegram

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"

	"github.com/yerassyldanay/xchats/backend/messaging"
)

// TokenSource resolves an account's bot token at send time. It is an interface
// (satisfied by *store.Store) so the token never has to travel in a queue
// payload, a DTO or a log line to reach the sender — the sender fetches it,
// uses it, and drops it.
type TokenSource interface {
	TelegramBotToken(ctx context.Context, accountID uuid.UUID) (string, error)
}

// ChannelSender wraps a Client behind messaging.ChannelSender. out.To is the
// numeric Telegram chat id (as text — the channel-neutral contract carries an
// opaque string); the sending bot's token is resolved from out.AccountID via
// Tokens, never carried on OutboundMessage itself.
type ChannelSender struct {
	Client Client
	Tokens TokenSource
	// Blobs resolves out.Media.BlobID to bytes. May be nil in a text-only
	// wiring; a media send then fails explicitly.
	Blobs blob.Store
}

// NewChannelSender returns a Telegram ChannelSender.
func NewChannelSender(client Client, tokens TokenSource, blobs blob.Store) *ChannelSender {
	return &ChannelSender{Client: client, Tokens: tokens, Blobs: blobs}
}

// Send delivers out as a Telegram message — plain text, or one attachment when
// out.Media is set. The returned ExternalID is the Bot API's message_id, which
// the store stamps onto the outbound row.
//
// Bots receive no delivery receipts, so a successful call is the whole
// confirmation there will ever be — Delivered is reported true on a real
// message_id and the row's state caps at 'sent'.
func (s *ChannelSender) Send(ctx context.Context, out messaging.OutboundMessage) (messaging.SendResult, error) {
	accountID, err := uuid.Parse(out.AccountID)
	if err != nil {
		return messaging.SendResult{}, fmt.Errorf("telegram: invalid account id %q: %w", out.AccountID, err)
	}
	chatID, err := strconv.ParseInt(out.To, 10, 64)
	if err != nil {
		return messaging.SendResult{}, fmt.Errorf("telegram: destination %q is not a chat id: %w", out.To, err)
	}
	token, err := s.Tokens.TelegramBotToken(ctx, accountID)
	if err != nil {
		return messaging.SendResult{}, fmt.Errorf("telegram: resolve bot token: %w", err)
	}

	var sent SentMessage
	if out.Media != nil {
		sent, err = s.sendMedia(ctx, token, chatID, out.Media)
	} else {
		sent, err = s.Client.SendMessage(ctx, token, chatID, out.Text)
	}
	if err != nil {
		return messaging.SendResult{}, err
	}
	return messaging.SendResult{
		ExternalID: strconv.FormatInt(sent.MessageID, 10),
		Delivered:  sent.MessageID != 0,
	}, nil
}

func (s *ChannelSender) sendMedia(ctx context.Context, token string, chatID int64, m *messaging.OutboundMedia) (SentMessage, error) {
	up := Upload{
		Kind:        m.Kind,
		FileName:    m.FileName,
		Mimetype:    m.Mimetype,
		Caption:     m.Caption,
		ProviderRef: m.ProviderRef,
	}
	// A provider ref means Telegram already holds these bytes; re-sending it
	// costs one small API call instead of a full upload.
	if up.ProviderRef == "" {
		if s.Blobs == nil {
			return SentMessage{}, fmt.Errorf("telegram: media send requires a blob store")
		}
		data, meta, err := s.Blobs.Get(m.BlobID)
		if err != nil {
			return SentMessage{}, fmt.Errorf("telegram: read media %q: %w", m.BlobID, err)
		}
		up.Data = data
		if up.Kind == "" {
			up.Kind = meta.MediaType
		}
		if up.FileName == "" {
			up.FileName = meta.FileName
		}
		if up.Mimetype == "" {
			up.Mimetype = meta.Mimetype
		}
	}
	return s.Client.SendMedia(ctx, token, chatID, up)
}
