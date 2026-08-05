package whatsmeow

import (
	"encoding/json"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/yerassyldanay/xchats/backend/internal/whatsapp"
)

// translateMessage converts a whatsmeow *events.Message into the
// provider-neutral MessageReceived shape. Pure — no I/O, no store/queue/hub
// access — so it is trivial to unit test and to drive from synthetic
// (debug-endpoint) input.
func translateMessage(accountID string, evt *events.Message) whatsapp.MessageReceived {
	var phoneJID, lidJID, chatJID string
	if evt.Info.IsGroup {
		// Groups are dropped by the ingestion service (see manager.go) —
		// only enough is resolved here to label the event safely for logs.
		chatJID = evt.Info.Chat.String()
	} else {
		phoneJID, lidJID = resolveDMRefs(evt.Info.MessageSource)
		chatJID = whatsapp.ChatKey(phoneJID, lidJID)
	}

	kind, text, media := extractContent(evt.Message)
	return whatsapp.MessageReceived{
		AccountID:  accountID,
		ExternalID: evt.Info.ID,
		ChatJID:    chatJID,
		PhoneJID:   phoneJID,
		LidJID:     lidJID,
		PushName:   evt.Info.PushName,
		FromMe:     evt.Info.IsFromMe,
		IsGroup:    evt.Info.IsGroup,
		Kind:       kind,
		Text:       text,
		Media:      media,
		Timestamp:  evt.Info.Timestamp,
		Raw:        rawSnapshot(evt),
	}
}

// extractContent pulls the text/caption and media descriptor out of a
// waE2E.Message, in xchats' own message_kind vocabulary. Returns
// ("", "", nil) for message types xchats does not ingest (reactions, polls,
// protocol messages, ...) — the caller treats that as a message with no
// displayable content rather than an error.
func extractContent(msg *waE2E.Message) (kind, text string, media *whatsapp.Media) {
	switch {
	case msg == nil:
		return "", "", nil
	case msg.GetConversation() != "":
		return "conversation", msg.GetConversation(), nil
	case msg.GetExtendedTextMessage() != nil:
		return "conversation", msg.GetExtendedTextMessage().GetText(), nil
	case msg.GetImageMessage() != nil:
		m := msg.GetImageMessage()
		return "imageMessage", m.GetCaption(), &whatsapp.Media{
			Kind: "image", Mimetype: m.GetMimetype(), FileSize: int64(m.GetFileLength()),
		}
	case msg.GetVideoMessage() != nil:
		m := msg.GetVideoMessage()
		return "videoMessage", m.GetCaption(), &whatsapp.Media{
			Kind: "video", Mimetype: m.GetMimetype(), FileSize: int64(m.GetFileLength()),
		}
	case msg.GetAudioMessage() != nil:
		m := msg.GetAudioMessage()
		return "audioMessage", "", &whatsapp.Media{
			Kind: "audio", Mimetype: m.GetMimetype(), FileSize: int64(m.GetFileLength()),
		}
	case msg.GetDocumentMessage() != nil:
		m := msg.GetDocumentMessage()
		return "documentMessage", m.GetCaption(), &whatsapp.Media{
			Kind: "document", Mimetype: m.GetMimetype(), FileName: m.GetFileName(), FileSize: int64(m.GetFileLength()),
		}
	case msg.GetStickerMessage() != nil:
		m := msg.GetStickerMessage()
		return "stickerMessage", "", &whatsapp.Media{
			Kind: "sticker", Mimetype: m.GetMimetype(), FileSize: int64(m.GetFileLength()),
		}
	default:
		return "", "", nil
	}
}

// rawSnapshot is a best-effort JSON debug snapshot of a message event; it is
// never parsed back, so a marshal failure (there should not be one) just
// yields no snapshot rather than an error the caller would have to handle.
func rawSnapshot(evt *events.Message) []byte {
	b, err := json.Marshal(struct {
		ID        string `json:"id"`
		Chat      string `json:"chat"`
		Sender    string `json:"sender"`
		FromMe    bool   `json:"from_me"`
		Timestamp int64  `json:"timestamp"`
	}{
		ID: evt.Info.ID, Chat: evt.Info.Chat.String(), Sender: evt.Info.Sender.String(),
		FromMe: evt.Info.IsFromMe, Timestamp: evt.Info.Timestamp.Unix(),
	})
	if err != nil {
		return nil
	}
	return b
}

// deliveryStateForReceipt maps a whatsmeow receipt type to xchats' delivery
// ladder, per the production rule: Delivered ("") -> delivered; Read/Played
// -> read; anything else (Sender/Retry/ReadSelf/...) is dropped (ok=false).
func deliveryStateForReceipt(t types.ReceiptType) (state string, ok bool) {
	switch t {
	case types.ReceiptTypeDelivered:
		return "delivered", true
	case types.ReceiptTypeRead, types.ReceiptTypePlayed:
		return "read", true
	default:
		return "", false
	}
}

// translateReceipt converts a whatsmeow *events.Receipt into a
// ReceiptUpdated, or ok=false for a receipt type/shape xchats does not act on.
func translateReceipt(accountID string, evt *events.Receipt) (whatsapp.ReceiptUpdated, bool) {
	state, ok := deliveryStateForReceipt(evt.Type)
	if !ok || len(evt.MessageIDs) == 0 {
		return whatsapp.ReceiptUpdated{}, false
	}
	return whatsapp.ReceiptUpdated{
		AccountID: accountID, ExternalIDs: evt.MessageIDs, DeliveryState: state,
	}, true
}
