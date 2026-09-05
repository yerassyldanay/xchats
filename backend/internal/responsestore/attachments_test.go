package responsestore_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/response"
)

func TestConversationRepo_LoadForResponse_AudioAttachmentCarriesTranscript(t *testing.T) {
	st, _, accountID := newTestStore(t)
	ctx := context.Background()

	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77000000001@s.whatsapp.net", RemoteJID: "77000000001@s.whatsapp.net",
		PhoneNumber: "77000000001", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "AUDIO1", MessageKind: "audioMessage", Body: "",
		Source: "live_webhook",
	})
	if err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if _, _, err := st.UpsertMessageMedia(ctx, res.MessageID,
		store.MediaRef{MediaType: "audio", Mimetype: "audio/ogg"}, "blob-audio-1", "ready"); err != nil {
		t.Fatalf("seed media: %v", err)
	}
	if err := st.UpdateMediaTranscript(ctx, "whatsapp", res.MessageID, "здравствуйте, а есть доставка?"); err != nil {
		t.Fatalf("set transcript: %v", err)
	}

	repo := &responsestore.ConversationRepo{Store: st}
	got, err := repo.LoadForResponse(ctx, res.ChatID.String())
	if err != nil {
		t.Fatalf("LoadForResponse: %v", err)
	}
	if len(got.Attachments) != 1 {
		t.Fatalf("Attachments = %+v, want exactly 1", got.Attachments)
	}
	a := got.Attachments[0]
	if a.Kind != response.AttachmentAudio || a.Transcript != "здравствуйте, а есть доставка?" {
		t.Fatalf("attachment = %+v", a)
	}
}

func TestConversationRepo_LoadForResponse_PendingAudioHasNoAttachment(t *testing.T) {
	st, _, accountID := newTestStore(t)
	ctx := context.Background()

	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77000000002@s.whatsapp.net", RemoteJID: "77000000002@s.whatsapp.net",
		PhoneNumber: "77000000002", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "AUDIO2", MessageKind: "audioMessage", Body: "",
		Source: "live_webhook",
	})
	if err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	// Media row exists but transcript is still empty — STT hasn't finished
	// (or is not configured) yet.
	if _, _, err := st.UpsertMessageMedia(ctx, res.MessageID,
		store.MediaRef{MediaType: "audio", Mimetype: "audio/ogg"}, "blob-audio-2", "ready"); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	repo := &responsestore.ConversationRepo{Store: st}
	got, err := repo.LoadForResponse(ctx, res.ChatID.String())
	if err != nil {
		t.Fatalf("LoadForResponse: %v", err)
	}
	if len(got.Attachments) != 0 {
		t.Fatalf("Attachments = %+v, want none while transcript is still empty", got.Attachments)
	}
}

func TestConversationRepo_LoadForResponse_ImageAttachmentEmbedsDataURI(t *testing.T) {
	st, _, accountID := newTestStore(t)
	ctx := context.Background()
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob.NewDisk: %v", err)
	}
	const imageBytes = "fake-jpeg-bytes"
	if _, err := blobStore.Put("blob-image-1", []byte(imageBytes), blob.Meta{Mimetype: "image/jpeg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}

	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77000000003@s.whatsapp.net", RemoteJID: "77000000003@s.whatsapp.net",
		PhoneNumber: "77000000003", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "IMG1", MessageKind: "imageMessage", Body: "это в наличии?",
		Source: "live_webhook",
	})
	if err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if _, _, err := st.UpsertMessageMedia(ctx, res.MessageID,
		store.MediaRef{MediaType: "image", Mimetype: "image/jpeg"}, "blob-image-1", "ready"); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	repo := &responsestore.ConversationRepo{Store: st, Blob: blobStore}
	got, err := repo.LoadForResponse(ctx, res.ChatID.String())
	if err != nil {
		t.Fatalf("LoadForResponse: %v", err)
	}
	if len(got.Attachments) != 1 {
		t.Fatalf("Attachments = %+v, want exactly 1", got.Attachments)
	}
	a := got.Attachments[0]
	if a.Kind != response.AttachmentImage {
		t.Fatalf("Kind = %q, want image", a.Kind)
	}
	if a.Caption != "это в наличии?" {
		t.Fatalf("Caption = %q, want the message body", a.Caption)
	}
	wantURI := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte(imageBytes))
	if a.DataURI != wantURI {
		t.Fatalf("DataURI = %q, want %q", a.DataURI, wantURI)
	}
}

func TestConversationRepo_LoadForResponse_PendingImageHasNoAttachment(t *testing.T) {
	st, _, accountID := newTestStore(t)
	ctx := context.Background()
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob.NewDisk: %v", err)
	}

	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77000000004@s.whatsapp.net", RemoteJID: "77000000004@s.whatsapp.net",
		PhoneNumber: "77000000004", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "IMG2", MessageKind: "imageMessage", Body: "",
		Source: "live_webhook",
	})
	if err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	// UpsertMessageMedia defaults to "pending" download_status when none is
	// supplied via UpdateMediaTranscript/SetMediaReady — no bytes yet.
	if _, _, err := st.UpsertMessageMedia(ctx, res.MessageID,
		store.MediaRef{MediaType: "image", Mimetype: "image/jpeg"}, "blob-image-2", "pending"); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	repo := &responsestore.ConversationRepo{Store: st, Blob: blobStore}
	got, err := repo.LoadForResponse(ctx, res.ChatID.String())
	if err != nil {
		t.Fatalf("LoadForResponse: %v", err)
	}
	if len(got.Attachments) != 0 {
		t.Fatalf("Attachments = %+v, want none while the image is still downloading", got.Attachments)
	}
}

func TestConversationRepo_LoadForResponse_OversizedImageIsSkipped(t *testing.T) {
	st, _, accountID := newTestStore(t)
	ctx := context.Background()
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob.NewDisk: %v", err)
	}
	huge := strings.Repeat("x", 9<<20) // over maxVisionImageBytes (8 MiB)
	if _, err := blobStore.Put("blob-image-huge", []byte(huge), blob.Meta{Mimetype: "image/jpeg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}

	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77000000005@s.whatsapp.net", RemoteJID: "77000000005@s.whatsapp.net",
		PhoneNumber: "77000000005", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "IMG3", MessageKind: "imageMessage", Body: "",
		Source: "live_webhook",
	})
	if err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if _, _, err := st.UpsertMessageMedia(ctx, res.MessageID,
		store.MediaRef{MediaType: "image", Mimetype: "image/jpeg"}, "blob-image-huge", "ready"); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	repo := &responsestore.ConversationRepo{Store: st, Blob: blobStore}
	got, err := repo.LoadForResponse(ctx, res.ChatID.String())
	if err != nil {
		t.Fatalf("LoadForResponse: %v", err)
	}
	if len(got.Attachments) != 0 {
		t.Fatalf("Attachments = %+v, want the oversized image skipped", got.Attachments)
	}
}

func TestConversationRepo_LoadForResponse_NoBlobStoreSkipsImages(t *testing.T) {
	st, _, accountID := newTestStore(t)
	ctx := context.Background()

	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77000000006@s.whatsapp.net", RemoteJID: "77000000006@s.whatsapp.net",
		PhoneNumber: "77000000006", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "IMG4", MessageKind: "imageMessage", Body: "",
		Source: "live_webhook",
	})
	if err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if _, _, err := st.UpsertMessageMedia(ctx, res.MessageID,
		store.MediaRef{MediaType: "image", Mimetype: "image/jpeg"}, "blob-image-3", "ready"); err != nil {
		t.Fatalf("seed media: %v", err)
	}

	// Blob deliberately left nil.
	repo := &responsestore.ConversationRepo{Store: st}
	got, err := repo.LoadForResponse(ctx, res.ChatID.String())
	if err != nil {
		t.Fatalf("LoadForResponse: %v", err)
	}
	if len(got.Attachments) != 0 {
		t.Fatalf("Attachments = %+v, want none with no Blob store configured", got.Attachments)
	}
}
