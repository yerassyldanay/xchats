package whatsmeow

import (
	"context"
	"fmt"

	wm "go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// mediaTypeFor maps xchats' media-kind vocabulary to whatsmeow's own
// MediaType for Upload. Stickers upload as MediaImage — whatsmeow has no
// separate sticker media type (see its own download.go classToMediaType map).
func mediaTypeFor(kind string) wm.MediaType {
	switch kind {
	case "video":
		return wm.MediaVideo
	case "audio":
		return wm.MediaAudio
	case "document":
		return wm.MediaDocument
	default: // image, sticker, and anything unrecognized
		return wm.MediaImage
	}
}

// buildOutboundMessage constructs the waE2E.Message to send. uploaded is nil
// for a plain text send; otherwise kind picks which media sub-message wraps
// the uploaded reference.
func buildOutboundMessage(kind, text, mimetype, fileName, caption string, uploaded *wm.UploadResponse) (*waE2E.Message, error) {
	if uploaded == nil {
		return &waE2E.Message{Conversation: proto.String(text)}, nil
	}
	switch kind {
	case "image":
		return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
			MediaKey: uploaded.MediaKey, FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256,
			FileLength: proto.Uint64(uploaded.FileLength), Mimetype: proto.String(mimetype), Caption: proto.String(caption),
		}}, nil
	case "video":
		return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
			URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
			MediaKey: uploaded.MediaKey, FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256,
			FileLength: proto.Uint64(uploaded.FileLength), Mimetype: proto.String(mimetype), Caption: proto.String(caption),
		}}, nil
	case "audio":
		return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
			URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
			MediaKey: uploaded.MediaKey, FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256,
			FileLength: proto.Uint64(uploaded.FileLength), Mimetype: proto.String(mimetype),
		}}, nil
	case "sticker":
		return &waE2E.Message{StickerMessage: &waE2E.StickerMessage{
			URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
			MediaKey: uploaded.MediaKey, FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256,
			FileLength: proto.Uint64(uploaded.FileLength), Mimetype: proto.String(mimetype),
		}}, nil
	default: // document, and anything unrecognized — a wrong-but-delivered
		// attachment beats a rejected send (mirrors telegram.MediaMethod's
		// own fallback).
		return &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
			URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
			MediaKey: uploaded.MediaKey, FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256,
			FileLength: proto.Uint64(uploaded.FileLength), Mimetype: proto.String(mimetype),
			FileName: proto.String(fileName), Caption: proto.String(caption),
		}}, nil
	}
}

// downloadable returns the message's downloadable media sub-message (any of
// ImageMessage/VideoMessage/AudioMessage/DocumentMessage/StickerMessage all
// implement whatsmeow.DownloadableMessage via their generated getters), or
// nil if msg carries no media whatsmeow.Client.Download can fetch.
func downloadable(msg *waE2E.Message) wm.DownloadableMessage {
	switch {
	case msg == nil:
		return nil
	case msg.GetImageMessage() != nil:
		return msg.GetImageMessage()
	case msg.GetVideoMessage() != nil:
		return msg.GetVideoMessage()
	case msg.GetAudioMessage() != nil:
		return msg.GetAudioMessage()
	case msg.GetDocumentMessage() != nil:
		return msg.GetDocumentMessage()
	case msg.GetStickerMessage() != nil:
		return msg.GetStickerMessage()
	default:
		return nil
	}
}

// downloader is the seam media download depends on — satisfied by
// *whatsmeow.Client, faked in tests.
type downloader interface {
	Download(ctx context.Context, msg wm.DownloadableMessage) ([]byte, error)
}

// downloadMedia fetches an inbound attachment's bytes given the original
// message, returning a clear error for the "no media on this message" case
// rather than silently succeeding with zero bytes.
func downloadMedia(ctx context.Context, cli downloader, msg *waE2E.Message) ([]byte, error) {
	d := downloadable(msg)
	if d == nil {
		return nil, fmt.Errorf("whatsmeow: message has no downloadable media")
	}
	return cli.Download(ctx, d)
}
