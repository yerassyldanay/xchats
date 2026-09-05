package whatsmeow

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/stt"
)

// fakeTranscriber and fakeAutomation mirror internal/worker's own test
// doubles of the same name — kept local rather than exported from that
// package, since this is the only other package that needs them.
//
// fakeTranscriber is context-aware (unlike a dumb stub that always
// succeeds) so a test can prove which context TranscribeAudio actually
// calls Transcribe with: delay, when set, makes it block until either that
// time elapses (success) or ctx is done first (ctx.Err()) — a real
// network client behaves the same way once its request context expires.
type fakeTranscriber struct {
	text  string
	delay time.Duration
}

func (f *fakeTranscriber) Transcribe(ctx context.Context, audio []byte, filename, mime string, opts stt.TranscribeOptions) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(f.delay):
		return f.text, nil
	}
}

// fakeAutomation is mutex-protected (unlike internal/worker's own copy of
// this type) because downloadAndAttachMedia calls it from a background
// goroutine while a test polls calls() from the test goroutine.
type fakeAutomation struct {
	mu    sync.Mutex
	calls []fakeAutomationCall
}

type fakeAutomationCall struct {
	chatID, accountID uuid.UUID
	channel           string
}

func (f *fakeAutomation) OnInboundMessage(ctx context.Context, chatID, accountID uuid.UUID, channel string, lastMessageAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeAutomationCall{chatID: chatID, accountID: accountID, channel: channel})
	return nil
}

func (f *fakeAutomation) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeAutomation) lastCall() fakeAutomationCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[len(f.calls)-1]
}

// TestDownloadAndAttachMedia_TranscribesVoiceNote is the production-wiring
// proof Codex's review asked for: a real inbound WhatsApp voice note, once
// its media finishes downloading, must actually reach STT — internal/
// worker's own tests only ever call transcribeIfAudio directly and cannot
// catch a gap in the wiring between this package's own media-download path
// and worker.TranscribeAudio.
func TestDownloadAndAttachMedia_TranscribesVoiceNote(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	q := queue.NewInMem(64, 1, testLogger())
	t.Cleanup(q.Close)
	transcriber := &fakeTranscriber{text: "привет, есть доставка?"}
	auto := &fakeAutomation{}

	mgr, err := NewManager(ctx, ManagerConfig{
		DeviceDBPath: filepath.Join(t.TempDir(), "wa-device.db"),
		Store:        st,
		Blob:         blobStore,
		Queue:        q,
		Hub:          realtime.NewHub(),
		Automation:   auto,
		STT:          func(ctx context.Context) stt.Params { return stt.Params{Transcriber: transcriber} },
		Log:          testLogger(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(mgr.Close)

	_, accountID := seedAccount(t, st, "77011111111@s.whatsapp.net")
	fake := newFakeWAClient()
	fake.downloadResp = []byte("audio-bytes")
	mgr.registerClient(accountID.String(), fake)

	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat: types.JID{User: "77000000000", Server: types.DefaultUserServer},
			},
			ID: "VOICE1", Timestamp: time.Now(),
		},
		Message: &waE2E.Message{
			AudioMessage: &waE2E.AudioMessage{
				Mimetype: proto.String("audio/ogg"), FileLength: proto.Uint64(11), PTT: proto.Bool(true),
			},
		},
	}
	mgr.handleMessageEvent(ctx, accountID.String(), evt)

	// downloadAndAttachMedia (and the transcription/re-arm it now triggers)
	// runs in its own goroutine — poll for the second automation call rather
	// than assuming it has landed by the time handleMessageEvent returns.
	// Two calls are expected in total: ingestMessage arms automation once
	// when the (still-untranscribed) voice note first arrives, and
	// TranscribeAudio arms it again once the transcript lands — the second
	// is what this test is actually proving exists.
	waitForAutomationCalls(t, auto, 2)

	msgs, _, err := st.MessagesForChat(ctx, mustChatFor(t, st, accountID, "77000000000@s.whatsapp.net"), time.Time{}, 1)
	if err != nil {
		t.Fatalf("MessagesForChat: %v", err)
	}
	if len(msgs) != 1 || len(msgs[0].Media) != 1 || msgs[0].Media[0].Transcript != "привет, есть доставка?" {
		t.Fatalf("messages = %+v, want the transcribed text", msgs)
	}
	last := auto.lastCall()
	if last.accountID != accountID || last.channel != "whatsapp" {
		t.Fatalf("post-transcript automation call = %+v, want account %s / channel whatsapp", last, accountID)
	}
}

// TestDownloadAndAttachMedia_TranscriptionSurvivesExhaustedDownloadContext
// is the regression test for a real bug: downloadAndAttachMedia's original
// fix reused the download-scoped context (bounded by mediaDownloadTimeout)
// for TranscribeAudio/RearmAfterMediaReady too. A slow download could leave
// that context nearly or fully expired by the time transcription started —
// and since SetMediaReady had already committed, no sweep would ever retry
// it, so the voice note stayed untranscribed forever. Fixed by giving the
// post-download step its own fresh postMediaReadyTimeout-bounded context;
// this test proves it by shrinking mediaDownloadTimeout to near-zero and
// making the transcriber slower than that (but well within
// postMediaReadyTimeout) — a shared context would show up here as
// ctx.Err() reaching the transcriber, a fresh one lets it complete.
func TestDownloadAndAttachMedia_TranscriptionSurvivesExhaustedDownloadContext(t *testing.T) {
	original := mediaDownloadTimeout
	mediaDownloadTimeout = 50 * time.Millisecond
	t.Cleanup(func() { mediaDownloadTimeout = original })

	ctx := context.Background()
	st := dbtest.New(t)
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	q := queue.NewInMem(64, 1, testLogger())
	t.Cleanup(q.Close)
	// Slower than mediaDownloadTimeout (which the download itself, and then
	// SetMediaReady/broadcastMessage, will have mostly or fully consumed by
	// the time Transcribe is called) but comfortably inside
	// postMediaReadyTimeout (3 minutes) — the exact gap the bug fell into.
	transcriber := &fakeTranscriber{text: "привет", delay: 200 * time.Millisecond}

	mgr, err := NewManager(ctx, ManagerConfig{
		DeviceDBPath: filepath.Join(t.TempDir(), "wa-device.db"),
		Store:        st,
		Blob:         blobStore,
		Queue:        q,
		Hub:          realtime.NewHub(),
		STT:          func(ctx context.Context) stt.Params { return stt.Params{Transcriber: transcriber} },
		Log:          testLogger(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(mgr.Close)

	_, accountID := seedAccount(t, st, "77011111113@s.whatsapp.net")
	fake := newFakeWAClient()
	fake.downloadResp = []byte("audio-bytes")
	mgr.registerClient(accountID.String(), fake)

	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat: types.JID{User: "77000000002", Server: types.DefaultUserServer},
			},
			ID: "VOICE2", Timestamp: time.Now(),
		},
		Message: &waE2E.Message{
			AudioMessage: &waE2E.AudioMessage{
				Mimetype: proto.String("audio/ogg"), FileLength: proto.Uint64(11), PTT: proto.Bool(true),
			},
		},
	}
	mgr.handleMessageEvent(ctx, accountID.String(), evt)

	chatID := mustChatFor(t, st, accountID, "77000000002@s.whatsapp.net")
	deadline := time.Now().Add(5 * time.Second)
	for {
		msgs, _, err := st.MessagesForChat(ctx, chatID, time.Time{}, 1)
		if err != nil {
			t.Fatalf("MessagesForChat: %v", err)
		}
		if len(msgs) == 1 && len(msgs[0].Media) == 1 && msgs[0].Media[0].Transcript != "" {
			return // the fix: transcription completed despite the tiny mediaDownloadTimeout
		}
		if time.Now().After(deadline) {
			t.Fatalf("transcript never appeared — TranscribeAudio is still sharing the download-scoped context; last seen messages: %+v", msgs)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitForAutomationCalls polls auto until it has recorded want calls,
// failing the test if that never happens within a generous deadline —
// downloadAndAttachMedia's transcription/re-arm step runs in a background
// goroutine, so asserting on auto immediately after handleMessageEvent
// returns would race it.
func waitForAutomationCalls(t *testing.T, auto *fakeAutomation, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if auto.callCount() >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("automation calls = %d after the deadline, want at least %d", auto.callCount(), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestDownloadAndAttachMedia_RearmsAutomationForImage is
// TestDownloadAndAttachMedia_TranscribesVoiceNote's image counterpart: a
// real inbound WhatsApp photo, once downloaded, must re-arm automation too
// (see worker.RearmAfterMediaReady) — otherwise a debounce that fired
// before the download finished would generate with no attachment, and
// nothing would ever try again once the image became ready.
func TestDownloadAndAttachMedia_RearmsAutomationForImage(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	q := queue.NewInMem(64, 1, testLogger())
	t.Cleanup(q.Close)
	auto := &fakeAutomation{}

	mgr, err := NewManager(ctx, ManagerConfig{
		DeviceDBPath: filepath.Join(t.TempDir(), "wa-device.db"),
		Store:        st,
		Blob:         blobStore,
		Queue:        q,
		Hub:          realtime.NewHub(),
		Automation:   auto,
		Log:          testLogger(),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(mgr.Close)

	_, accountID := seedAccount(t, st, "77011111112@s.whatsapp.net")
	fake := newFakeWAClient()
	fake.downloadResp = []byte("jpeg-bytes")
	mgr.registerClient(accountID.String(), fake)

	evt := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat: types.JID{User: "77000000001", Server: types.DefaultUserServer},
			},
			ID: "PHOTO1", Timestamp: time.Now(),
		},
		Message: &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				Mimetype: proto.String("image/jpeg"), FileLength: proto.Uint64(10),
			},
		},
	}
	mgr.handleMessageEvent(ctx, accountID.String(), evt)

	// Same two-call shape as the voice-note test: ingestMessage arms once on
	// arrival, RearmAfterMediaReady arms again once the image is ready.
	waitForAutomationCalls(t, auto, 2)

	chatID := mustChatFor(t, st, accountID, "77000000001@s.whatsapp.net")
	msgs, _, err := st.MessagesForChat(ctx, chatID, time.Time{}, 1)
	if err != nil {
		t.Fatalf("MessagesForChat: %v", err)
	}
	if len(msgs) != 1 || len(msgs[0].Media) != 1 || msgs[0].Media[0].DownloadStatus != "ready" {
		t.Fatalf("messages = %+v, want one ready image attachment", msgs)
	}
	last := auto.lastCall()
	if last.accountID != accountID || last.channel != "whatsapp" {
		t.Fatalf("post-download automation call = %+v, want account %s / channel whatsapp", last, accountID)
	}
}

// mustChatFor resolves the chat FindOrCreateChat would for remoteJID —
// handleMessageEvent already created it via ingestMessage; this just looks
// it back up for the polling loop above.
func mustChatFor(t *testing.T, st *store.Store, accountID uuid.UUID, remoteJID string) uuid.UUID {
	t.Helper()
	chatID, _, err := st.FindOrCreateChat(context.Background(), accountID, remoteJID, "")
	if err != nil {
		t.Fatalf("FindOrCreateChat: %v", err)
	}
	return chatID
}
