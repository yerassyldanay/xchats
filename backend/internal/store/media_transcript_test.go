package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// TestUpdateMediaTranscriptDispatchesByChannel verifies UpdateMediaTranscript
// lands on each channel's own media table (dispatched exactly like
// UpsertOutboundMedia) and that the read path (MessagesForChat's attachMedia,
// backed by inbox_message_media_v) surfaces the transcript it wrote, along
// with the storage_key/download_status migration 0019 also exposed. The
// Telegram leg is covered separately by TestUpdateMediaTranscriptOnTelegram:
// its media row can only be created via IngestTelegramInbound's own Media
// field, unlike the other two channels' standalone insert helpers.
func TestUpdateMediaTranscriptDispatchesByChannel(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()
	_, waChat, _, _, metaChat := seedFourChannels(t, st)

	for _, tc := range []struct {
		name    string
		chat    uuid.UUID
		channel string
		seed    func(messageID uuid.UUID)
	}{
		{"whatsapp", waChat, "whatsapp", func(id uuid.UUID) {
			if _, _, err := st.UpsertMessageMedia(ctx, id,
				store.MediaRef{MediaType: "audio", Mimetype: "audio/ogg"}, "wa-blob", "ready"); err != nil {
				t.Fatalf("seed wa media: %v", err)
			}
		}},
		{"whatsapp_cloud", metaChat, "whatsapp_cloud", func(id uuid.UUID) {
			if err := st.InsertChannelMediaPending(ctx, id, store.ChannelMediaMeta{MediaType: "audio", Mimetype: "audio/ogg"}); err != nil {
				t.Fatalf("seed channel media: %v", err)
			}
			if err := st.SetChannelMediaReady(ctx, id, "meta-blob", 2048); err != nil {
				t.Fatalf("mark channel media ready: %v", err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trigger, err := st.LatestInboundMessageID(ctx, tc.chat)
			if err != nil || !trigger.Valid {
				t.Fatalf("LatestInboundMessageID(%s) = %v, %v", tc.name, trigger, err)
			}
			messageID := trigger.UUID
			tc.seed(messageID)

			const text = "здравствуйте, хочу узнать цену"
			if err := st.UpdateMediaTranscript(ctx, tc.channel, messageID, text); err != nil {
				t.Fatalf("UpdateMediaTranscript(%s): %v", tc.name, err)
			}

			msgs, _, err := st.MessagesForChat(ctx, tc.chat, time.Time{}, 10)
			if err != nil {
				t.Fatalf("MessagesForChat(%s): %v", tc.name, err)
			}
			if len(msgs) != 1 || len(msgs[0].Media) != 1 {
				t.Fatalf("%s messages/media = %+v", tc.name, msgs)
			}
			media := msgs[0].Media[0]
			if media.Transcript != text {
				t.Errorf("%s transcript = %q, want %q", tc.name, media.Transcript, text)
			}
			if media.DownloadStatus != "ready" {
				t.Errorf("%s download_status = %q, want ready", tc.name, media.DownloadStatus)
			}
			if media.StorageKey == "" {
				t.Errorf("%s storage_key is empty, want the seeded blob id", tc.name)
			}
		})
	}
}

// TestUpdateMediaTranscriptOnTelegram covers the Telegram leg, whose media
// row can only come from IngestTelegramInbound's own Media field (there is
// no standalone "insert pending" helper the way channels.go's
// InsertChannelMediaPending gives the generic core).
func TestUpdateMediaTranscriptOnTelegram(t *testing.T) {
	st := dbtest.New(t)
	ctx := context.Background()

	box, err := secretbox.FromEnvValue(viewsEncKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	st.UseCredentialsBox(box)

	org, err := st.SeedOrganization(ctx, "tg-transcript-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	const botID = int64(777)
	tgAccount, err := st.ClaimTelegramAccount(ctx, store.TelegramClaim{
		ID: uuid.New(), OrganizationID: org.ID, DisplayName: "TG",
		BotID: botID, BotUsername: "transcript_bot", BotToken: "777:token",
	})
	if err != nil {
		t.Fatalf("claim telegram account: %v", err)
	}
	res, err := st.IngestTelegramInbound(ctx, store.TgInbound{
		AccountID: tgAccount.ID, UpdateID: 1,
		TelegramChatID: 1, ChatType: "private",
		TelegramUserID: 1, FirstName: "TG", DisplayName: "TG Клиент",
		TelegramMessageID: 1, MessageKind: "audioMessage", Preview: "🎙 Аудио",
		Media: &store.TgMedia{FileID: "file1", MediaType: "audio", Mimetype: "audio/ogg"},
	})
	if err != nil {
		t.Fatalf("telegram inbound: %v", err)
	}
	if err := st.SetTelegramMediaReady(ctx, res.MessageID, "tg-blob-1", 4096); err != nil {
		t.Fatalf("mark telegram media ready: %v", err)
	}

	const text = "подскажите, пожалуйста, есть ли доставка"
	if err := st.UpdateMediaTranscript(ctx, "telegram", res.MessageID, text); err != nil {
		t.Fatalf("UpdateMediaTranscript: %v", err)
	}

	msg, err := st.MessageByID(ctx, res.MessageID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if len(msg.Media) != 1 || msg.Media[0].Transcript != text {
		t.Fatalf("media = %+v, want transcript %q", msg.Media, text)
	}
}
