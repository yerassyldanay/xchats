package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/stt"
	"github.com/yerassyldanay/xchats/backend/response"
)

// fakeTranscriber is a scripted stt.Transcriber double that records every
// call it receives.
type fakeTranscriber struct {
	text string
	err  error

	calls []fakeTranscribeCall
}

type fakeTranscribeCall struct {
	audio            []byte
	filename, mime   string
	language, prompt string
}

func (f *fakeTranscriber) Transcribe(ctx context.Context, audio []byte, filename, mime string, opts stt.TranscribeOptions) (string, error) {
	f.calls = append(f.calls, fakeTranscribeCall{audio: audio, filename: filename, mime: mime, language: opts.Language, prompt: opts.Prompt})
	if f.err != nil {
		return "", f.err
	}
	return f.text, nil
}

type fakeKBRepo struct{ kb *aiprompt.KB }

func (r *fakeKBRepo) Load(ctx context.Context, organizationID string) (*aiprompt.KB, error) {
	return r.kb, nil
}

// seedAudioMessage seeds one WhatsApp organization/account/inbound audio
// message with a "ready" media row (no transcript yet) and returns the
// account, chat and message ids transcribeIfAudio needs.
func seedAudioMessage(t *testing.T, st *store.Store, blobID string) (accountID, chatID, messageID uuid.UUID, orgID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	org, err := st.SeedOrganization(ctx, "worker-stt-test-"+blobID)
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	ownerJID := "77022200000@s.whatsapp.net"
	accountID = config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "WhatsApp", ExternalAccountRef: ownerJID, ExternalHandle: "77022200000", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	res, err := st.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: accountID, PhoneJID: "77000009999@s.whatsapp.net", RemoteJID: "77000009999@s.whatsapp.net",
		PhoneNumber: "77000009999", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "AUDIOMSG-" + blobID, MessageKind: "audioMessage", Source: "live_webhook",
		// UpdateChatPreviewIfCurrent guards on message_ts == chats.last_message_at,
		// so the message needs a real timestamp — a zero MessageTS leaves
		// message_ts NULL while last_message_at still gets stamped "now" by
		// UpsertInbound's own COALESCE, and the guard would then never match.
		MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	if _, _, err := st.UpsertMessageMedia(ctx, res.MessageID,
		store.MediaRef{MediaType: "audio", Mimetype: "audio/ogg"}, blobID, "ready"); err != nil {
		t.Fatalf("seed media: %v", err)
	}
	return accountID, res.ChatID, res.MessageID, org.ID
}

func TestTranscribeIfAudio_PersistsTranscriptAndEnqueuesDraft(t *testing.T) {
	ctx := context.Background()
	stStore := dbtest.New(t)
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	accountID, chatID, messageID, _ := seedAudioMessage(t, stStore, "blob-1")
	if _, err := blobStore.Put("blob-1", []byte("audio-bytes"), blob.Meta{Mimetype: "audio/ogg", FileName: "voice.ogg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}

	transcriber := &fakeTranscriber{text: "привет, есть доставка?"}
	pub := &recordingQueue{}
	w := &Worker{
		Store: stStore, Blob: blobStore, Hub: realtime.NewHub(), Queue: pub, Log: testLogger(),
		STT: func(ctx context.Context) stt.Params {
			return stt.Params{Transcriber: transcriber, Language: "ru", Vocabulary: "Kaspi"}
		},
	}

	w.transcribeIfAudio(ctx, messageID, accountID, "whatsapp", "audio", "blob-1")

	msg, err := stStore.MessageByID(ctx, messageID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if len(msg.Media) != 1 || msg.Media[0].Transcript != "привет, есть доставка?" {
		t.Fatalf("media = %+v, want the transcribed text", msg.Media)
	}

	chat, err := stStore.ChatByID(ctx, chatID)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if chat.LastMessagePreview != "🎙 привет, есть доставка?" {
		t.Errorf("LastMessagePreview = %q, want the transcribed snippet", chat.LastMessagePreview)
	}

	if len(pub.published) != 1 {
		t.Fatalf("published = %d, want exactly 1 (a fresh AI draft)", len(pub.published))
	}
	task, ok := pub.published[0].Payload.(AIDraftTask)
	if !ok || task.ChatID != chatID {
		t.Fatalf("published task = %+v (%T), want AIDraftTask{ChatID: %s}", pub.published[0].Payload, pub.published[0].Payload, chatID)
	}

	if len(transcriber.calls) != 1 {
		t.Fatalf("Transcribe called %d times, want 1", len(transcriber.calls))
	}
	call := transcriber.calls[0]
	if string(call.audio) != "audio-bytes" || call.filename != "voice.ogg" || call.mime != "audio/ogg" {
		t.Errorf("call = %+v, want the blob's own bytes/filename/mime", call)
	}
	if call.language != "ru" {
		t.Errorf("language = %q, want ru", call.language)
	}
	if call.prompt != "Kaspi" {
		t.Errorf("prompt = %q, want the custom vocabulary alone (no Response/KB wired)", call.prompt)
	}
}

// TestTranscribeIfAudio_PrimesPromptFromKnowledgeBase proves the worker
// actually wires internal/stt.BuildPrompt to the organization's live
// catalog when w.Response.KnowledgeBase can resolve one — not just the
// operator's own custom vocabulary.
func TestTranscribeIfAudio_PrimesPromptFromKnowledgeBase(t *testing.T) {
	ctx := context.Background()
	stStore := dbtest.New(t)
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	accountID, _, messageID, orgID := seedAudioMessage(t, stStore, "blob-2")
	if _, err := blobStore.Put("blob-2", []byte("audio-bytes"), blob.Meta{Mimetype: "audio/ogg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}

	transcriber := &fakeTranscriber{text: "ok"}
	kb := &aiprompt.KB{OrganizationID: orgID.String(), Products: []aiprompt.Product{{Name: "iPhone 15 Pro", SalesStatus: "active"}}}
	w := &Worker{
		Store: stStore, Blob: blobStore, Hub: realtime.NewHub(), Queue: &recordingQueue{}, Log: testLogger(),
		Response: &response.Service{KnowledgeBase: &fakeKBRepo{kb: kb}},
		STT: func(ctx context.Context) stt.Params {
			return stt.Params{Transcriber: transcriber, Vocabulary: "Kaspi"}
		},
	}

	w.transcribeIfAudio(ctx, messageID, accountID, "whatsapp", "audio", "blob-2")

	if len(transcriber.calls) != 1 {
		t.Fatalf("Transcribe called %d times, want 1", len(transcriber.calls))
	}
	prompt := transcriber.calls[0].prompt
	if !strings.Contains(prompt, "Kaspi") || !strings.Contains(prompt, "iPhone 15 Pro") {
		t.Errorf("prompt = %q, want both the custom vocabulary and the KB's product name", prompt)
	}
}

func TestTranscribeIfAudio_SkipsNonAudioMedia(t *testing.T) {
	ctx := context.Background()
	stStore := dbtest.New(t)
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	transcriber := &fakeTranscriber{text: "should never run"}
	pub := &recordingQueue{}
	w := &Worker{
		Store: stStore, Blob: blobStore, Hub: realtime.NewHub(), Queue: pub, Log: testLogger(),
		STT: func(ctx context.Context) stt.Params { return stt.Params{Transcriber: transcriber} },
	}
	w.transcribeIfAudio(ctx, uuid.New(), uuid.New(), "whatsapp", "image", "irrelevant-blob")
	if len(transcriber.calls) != 0 || len(pub.published) != 0 {
		t.Fatal("must not transcribe or enqueue anything for a non-audio attachment")
	}
}

func TestTranscribeIfAudio_NoSTTResolverIsANoop(t *testing.T) {
	ctx := context.Background()
	stStore := dbtest.New(t)
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	accountID, _, messageID, _ := seedAudioMessage(t, stStore, "blob-3")
	pub := &recordingQueue{}
	w := &Worker{Store: stStore, Blob: blobStore, Hub: realtime.NewHub(), Queue: pub, Log: testLogger()} // STT left nil

	w.transcribeIfAudio(ctx, messageID, accountID, "whatsapp", "audio", "blob-3")

	msg, err := stStore.MessageByID(ctx, messageID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if len(msg.Media) != 1 || msg.Media[0].Transcript != "" {
		t.Fatalf("media = %+v, want no transcript with STT unconfigured", msg.Media)
	}
	if len(pub.published) != 0 {
		t.Fatal("must not enqueue an AI draft when STT is not configured")
	}
}

func TestTranscribeIfAudio_UnconfiguredTranscriberIsANoop(t *testing.T) {
	ctx := context.Background()
	stStore := dbtest.New(t)
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	accountID, _, messageID, _ := seedAudioMessage(t, stStore, "blob-4")
	pub := &recordingQueue{}
	w := &Worker{
		Store: stStore, Blob: blobStore, Hub: realtime.NewHub(), Queue: pub, Log: testLogger(),
		STT: func(ctx context.Context) stt.Params { return stt.Params{} }, // no provider configured
	}

	w.transcribeIfAudio(ctx, messageID, accountID, "whatsapp", "audio", "blob-4")

	msg, err := stStore.MessageByID(ctx, messageID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.Media[0].Transcript != "" {
		t.Fatal("must not have a transcript when no Transcriber is configured")
	}
	if len(pub.published) != 0 {
		t.Fatal("must not enqueue an AI draft when no Transcriber is configured")
	}
}

func TestTranscribeIfAudio_TranscribeErrorLeavesTranscriptEmpty(t *testing.T) {
	ctx := context.Background()
	stStore := dbtest.New(t)
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	accountID, _, messageID, _ := seedAudioMessage(t, stStore, "blob-5")
	if _, err := blobStore.Put("blob-5", []byte("audio-bytes"), blob.Meta{Mimetype: "audio/ogg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}
	transcriber := &fakeTranscriber{err: context.DeadlineExceeded}
	pub := &recordingQueue{}
	w := &Worker{
		Store: stStore, Blob: blobStore, Hub: realtime.NewHub(), Queue: pub, Log: testLogger(),
		STT: func(ctx context.Context) stt.Params { return stt.Params{Transcriber: transcriber} },
	}

	w.transcribeIfAudio(ctx, messageID, accountID, "whatsapp", "audio", "blob-5")

	msg, err := stStore.MessageByID(ctx, messageID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.Media[0].Transcript != "" {
		t.Fatal("a failed Transcribe call must not leave a partial/wrong transcript")
	}
	if len(pub.published) != 0 {
		t.Fatal("must not enqueue an AI draft when transcription failed")
	}
}

func TestTranscribeIfAudio_EmptyTranscriptIsNotPersisted(t *testing.T) {
	ctx := context.Background()
	stStore := dbtest.New(t)
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	accountID, _, messageID, _ := seedAudioMessage(t, stStore, "blob-6")
	if _, err := blobStore.Put("blob-6", []byte("audio-bytes"), blob.Meta{Mimetype: "audio/ogg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}
	transcriber := &fakeTranscriber{text: "   "} // whitespace-only, e.g. silence
	pub := &recordingQueue{}
	w := &Worker{
		Store: stStore, Blob: blobStore, Hub: realtime.NewHub(), Queue: pub, Log: testLogger(),
		STT: func(ctx context.Context) stt.Params { return stt.Params{Transcriber: transcriber} },
	}

	w.transcribeIfAudio(ctx, messageID, accountID, "whatsapp", "audio", "blob-6")

	msg, err := stStore.MessageByID(ctx, messageID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.Media[0].Transcript != "" {
		t.Fatal("a blank transcription result must not be persisted or trigger a redraft")
	}
	if len(pub.published) != 0 {
		t.Fatal("must not enqueue an AI draft for a blank transcription result")
	}
}
