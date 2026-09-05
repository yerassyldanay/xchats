package httpapi_test

// retranscribe_test.go covers POST /chats/:id/messages/:message_id/retranscribe
// (retranscribe.go) — the manual counterpart to worker.Worker.transcribeIfAudio.
// It uses its own lightweight harness (mirroring settingsHarness in
// settings_test.go) rather than the big newHarness in integration_test.go:
// this endpoint only touches Store/Blob/Credentials/Settings/Queue/Hub, none
// of the WhatsApp/Telegram/KB-import/chat-assistant machinery that harness
// stands up.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	zalandokeyring "github.com/zalando/go-keyring"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/password"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/stt"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
)

// recordingRetranscribeQueue is a minimal queue.Queue fake that just records
// every Publish call — mirrors internal/worker/meta_media_test.go's
// recordingQueue (unexported there, so re-declared here rather than
// imported).
type recordingRetranscribeQueue struct {
	mu        sync.Mutex
	published []queue.Message
}

func (q *recordingRetranscribeQueue) Publish(_ context.Context, m queue.Message) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.published = append(q.published, m)
	return nil
}
func (q *recordingRetranscribeQueue) Start(context.Context, queue.Handler) {}
func (q *recordingRetranscribeQueue) Wait()                                {}
func (q *recordingRetranscribeQueue) Close()                               {}
func (q *recordingRetranscribeQueue) items() []queue.Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]queue.Message, len(q.published))
	copy(out, q.published)
	return out
}

type retranscribeHarness struct {
	t         *testing.T
	srv       *httptest.Server
	client    *http.Client
	store     *store.Store
	blob      blob.Store
	orgID     uuid.UUID
	accountID uuid.UUID
	creds     *credentials.Chain
	sets      *settings.Store
	queue     *recordingRetranscribeQueue
}

func newRetranscribeHarness(t *testing.T) *retranscribeHarness {
	t.Helper()
	zalandokeyring.MockInitWithError(errors.New("OS keyring disabled in tests"))
	ctx := context.Background()
	st, _ := dbtest.Open(t)

	cfg := &config.Config{
		System:   config.SystemConfig{SessionTTLHours: 1, MinPasswordLen: 8},
		PageSize: 50,
		Server:   config.ServerConfig{CORSOrigins: []string{"*"}},
	}

	org, err := st.SeedOrganization(ctx, "xchats-retranscribe")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	hash, _ := password.Hash(adminPass)
	if _, err := st.SeedUser(ctx, org.ID, adminEmail, hash, "Admin"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	accountID := config.AccountID("77099999999@s.whatsapp.net")
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "WhatsApp", ExternalAccountRef: "77099999999@s.whatsapp.net",
		ExternalHandle: "77099999999", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	creds, err := credentials.Open(credentials.OpenOptions{AllowFile: true, ForceFile: true, DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("credentials.Open: %v", err)
	}
	sets := settings.NewStore(t.TempDir())
	q := &recordingRetranscribeQueue{}
	hub := realtime.NewHub()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Queue: q, Hub: hub, Blob: blobStore,
		Credentials: creds, Settings: sets, OrgID: org.ID, Log: log,
	})
	ts := httptest.NewServer(srv.Router())
	jar, _ := cookiejar.New(nil)
	h := &retranscribeHarness{
		t: t, srv: ts, client: &http.Client{Jar: jar}, store: st, blob: blobStore,
		orgID: org.ID, accountID: accountID, creds: creds, sets: sets, queue: q,
	}
	t.Cleanup(ts.Close)
	h.login()
	return h
}

func (h *retranscribeHarness) login() {
	h.t.Helper()
	body, _ := json.Marshal(map[string]string{"email": adminEmail, "password": adminPass})
	resp, err := h.client.Post(h.srv.URL+"/xchats/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		h.t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.t.Fatalf("login status=%d", resp.StatusCode)
	}
}

func (h *retranscribeHarness) do(method, path string, body any) (*http.Response, map[string]json.RawMessage) {
	h.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, h.srv.URL+path, reader)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	return resp, env
}

// seedAudioMessage seeds one inbound audio message on h's account, with a
// media row in the given download_status (bytes are NOT put in blob storage
// here — callers that need "ready" bytes retrievable call h.blob.Put
// themselves with the same blobID first).
func (h *retranscribeHarness) seedAudioMessage(blobID, downloadStatus string) (chatID, messageID uuid.UUID) {
	h.t.Helper()
	ctx := context.Background()
	// jid is keyed by blobID so distinct calls land in distinct chats
	// (UpsertInbound upserts the chat on (account_id, remote_jid) — reusing
	// one jid across calls would silently collapse every "different chat"
	// fixture onto the same row).
	jid := "77088" + blobID + "@s.whatsapp.net"
	res, err := h.store.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: h.accountID, PhoneJID: jid, RemoteJID: jid,
		PhoneNumber: "77088" + blobID, Direction: "in", SenderKind: "contact",
		ExternalMessageID: "AUDIO-" + blobID, MessageKind: "audioMessage", MessageTS: time.Now(), Source: "live_webhook",
	})
	if err != nil {
		h.t.Fatalf("seed inbound: %v", err)
	}
	if _, _, err := h.store.UpsertMessageMedia(ctx, res.MessageID,
		store.MediaRef{MediaType: "audio", Mimetype: "audio/ogg"}, blobID, downloadStatus); err != nil {
		h.t.Fatalf("seed media: %v", err)
	}
	return res.ChatID, res.MessageID
}

// configureSTT points LLMSettings.STTProvider/STTModel at a mock
// /audio/transcriptions server via the ProviderSettings.BaseURL override
// (the same mechanism internal/credentials' validator tests use) and seeds
// the matching credential.
func (h *retranscribeHarness) configureSTT(t *testing.T, baseURL string) {
	t.Helper()
	if err := h.creds.Set(context.Background(), "openai.api_key", "test-key"); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	if _, err := h.sets.Update(func(s *settings.Settings) {
		s.LLM.STTProvider, s.LLM.STTModel, s.LLM.STTLanguage = "openai", "whisper-1", "auto"
		s.Providers = map[string]settings.ProviderSettings{"openai": {BaseURL: baseURL}}
	}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
}

func parseRetranscribeLanguage(t *testing.T, r *http.Request) string {
	t.Helper()
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	var language string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		if part.FormName() == "language" {
			data, _ := io.ReadAll(part)
			language = string(data)
		}
	}
	return language
}

func TestHandleRetranscribeMessage_HappyPath(t *testing.T) {
	h := newRetranscribeHarness(t)
	var gotLanguage string
	sttSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLanguage = parseRetranscribeLanguage(t, r)
		io.WriteString(w, `{"text":"здравствуйте, привет"}`)
	}))
	defer sttSrv.Close()
	h.configureSTT(t, sttSrv.URL)

	chatID, messageID := h.seedAudioMessage("blob-1", "ready")
	if _, err := h.blob.Put("blob-1", []byte("audio-bytes"), blob.Meta{Mimetype: "audio/ogg", FileName: "voice.ogg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/chats/"+chatID.String()+"/messages/"+messageID.String()+"/retranscribe", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, env["message"])
	}
	if gotLanguage != "" {
		t.Errorf("language sent to provider = %q, want empty (auto-detect)", gotLanguage)
	}

	var got struct {
		Media []struct {
			Transcript string `json:"transcript"`
		} `json:"media"`
	}
	mustDecode(t, env, &got)
	if len(got.Media) != 1 || got.Media[0].Transcript != "здравствуйте, привет" {
		t.Fatalf("response media = %+v", got.Media)
	}

	msg, err := h.store.MessageByID(context.Background(), messageID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if len(msg.Media) != 1 || msg.Media[0].Transcript != "здравствуйте, привет" {
		t.Fatalf("persisted media = %+v, want the transcribed text", msg.Media)
	}

	published := h.queue.items()
	if len(published) != 1 {
		t.Fatalf("published = %d, want exactly 1 (a fresh AI draft)", len(published))
	}
	task, ok := published[0].Payload.(worker.AIDraftTask)
	if !ok || task.ChatID != chatID {
		t.Fatalf("published task = %+v (%T), want AIDraftTask{ChatID: %s}", published[0].Payload, published[0].Payload, chatID)
	}
}

// TestHandleRetranscribeMessage_UpdatesChatPreview proves a manual
// re-transcribe refreshes the chat list's own preview text too, not just
// the message inside the open thread — worker.TranscribeAudio's automatic
// run already does this (see worker.TranscriptPreview), and the two paths
// must not drift out of sync.
func TestHandleRetranscribeMessage_UpdatesChatPreview(t *testing.T) {
	h := newRetranscribeHarness(t)
	sttSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"text":"здравствуйте, привет"}`)
	}))
	defer sttSrv.Close()
	h.configureSTT(t, sttSrv.URL)

	chatID, messageID := h.seedAudioMessage("blob-preview", "ready")
	if _, err := h.blob.Put("blob-preview", []byte("audio-bytes"), blob.Meta{Mimetype: "audio/ogg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/chats/"+chatID.String()+"/messages/"+messageID.String()+"/retranscribe", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, env["message"])
	}

	chat, err := h.store.ChatByID(context.Background(), chatID)
	if err != nil {
		t.Fatalf("ChatByID: %v", err)
	}
	if chat.LastMessagePreview != "🎙 здравствуйте, привет" {
		t.Fatalf("LastMessagePreview = %q, want the transcribed snippet", chat.LastMessagePreview)
	}
}

// TestHandleRetranscribeMessage_OversizedAudioIs413 proves a blob over
// stt.MaxAudioBytes is rejected before the bytes are ever handed to the
// transcription provider — both OpenAI and Groq reject it anyway.
func TestHandleRetranscribeMessage_OversizedAudioIs413(t *testing.T) {
	h := newRetranscribeHarness(t)
	called := false
	sttSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		io.WriteString(w, `{"text":"should never be requested"}`)
	}))
	defer sttSrv.Close()
	h.configureSTT(t, sttSrv.URL)

	chatID, messageID := h.seedAudioMessage("blob-big", "ready")
	oversized := make([]byte, stt.MaxAudioBytes+1)
	if _, err := h.blob.Put("blob-big", oversized, blob.Meta{Mimetype: "audio/ogg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/chats/"+chatID.String()+"/messages/"+messageID.String()+"/retranscribe", nil)
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want 413; body=%s", resp.StatusCode, env["message"])
	}
	if called {
		t.Fatal("must not call the transcription provider for audio over the size cap")
	}
}

func TestHandleRetranscribeMessage_LanguageOverrideReachesProvider(t *testing.T) {
	h := newRetranscribeHarness(t)
	var gotLanguage string
	sttSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLanguage = parseRetranscribeLanguage(t, r)
		io.WriteString(w, `{"text":"ok"}`)
	}))
	defer sttSrv.Close()
	h.configureSTT(t, sttSrv.URL)

	chatID, messageID := h.seedAudioMessage("blob-2", "ready")
	if _, err := h.blob.Put("blob-2", []byte("audio-bytes"), blob.Meta{Mimetype: "audio/ogg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/chats/"+chatID.String()+"/messages/"+messageID.String()+"/retranscribe",
		map[string]string{"language": "kk"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, env["message"])
	}
	if gotLanguage != "kk" {
		t.Errorf("language sent to provider = %q, want kk", gotLanguage)
	}
}

func TestHandleRetranscribeMessage_UnknownLanguageIs400(t *testing.T) {
	h := newRetranscribeHarness(t)
	chatID, messageID := h.seedAudioMessage("blob-3", "ready")

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/chats/"+chatID.String()+"/messages/"+messageID.String()+"/retranscribe",
		map[string]string{"language": "fr"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", resp.StatusCode, env["message"])
	}
}

func TestHandleRetranscribeMessage_NoAudioAttachmentIs400(t *testing.T) {
	h := newRetranscribeHarness(t)
	ctx := context.Background()
	res, err := h.store.UpsertInbound(ctx, store.InboundUpsert{
		AccountID: h.accountID, PhoneJID: "77077777777@s.whatsapp.net", RemoteJID: "77077777777@s.whatsapp.net",
		PhoneNumber: "77077777777", Direction: "in", SenderKind: "contact",
		ExternalMessageID: "TEXT1", MessageKind: "conversation", Body: "привет", MessageTS: time.Now(),
	})
	if err != nil {
		t.Fatalf("seed inbound: %v", err)
	}

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/chats/"+res.ChatID.String()+"/messages/"+res.MessageID.String()+"/retranscribe", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", resp.StatusCode, env["message"])
	}
}

func TestHandleRetranscribeMessage_AudioNotReadyIs409(t *testing.T) {
	h := newRetranscribeHarness(t)
	chatID, messageID := h.seedAudioMessage("blob-4", "pending")

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/chats/"+chatID.String()+"/messages/"+messageID.String()+"/retranscribe", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", resp.StatusCode, env["message"])
	}
}

func TestHandleRetranscribeMessage_NoSTTConfiguredIs503(t *testing.T) {
	h := newRetranscribeHarness(t) // no configureSTT call — STTProvider stays ""
	chatID, messageID := h.seedAudioMessage("blob-5", "ready")
	if _, err := h.blob.Put("blob-5", []byte("audio-bytes"), blob.Meta{Mimetype: "audio/ogg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/chats/"+chatID.String()+"/messages/"+messageID.String()+"/retranscribe", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s, want 503", resp.StatusCode, env["message"])
	}
}

func TestHandleRetranscribeMessage_MessageFromAnotherChatIs404(t *testing.T) {
	h := newRetranscribeHarness(t)
	chatID, _ := h.seedAudioMessage("blob-6a", "ready")
	_, otherMessageID := h.seedAudioMessage("blob-6b", "ready")

	// otherMessageID genuinely exists, just not under chatID — must 404, not
	// leak whether a message id exists elsewhere.
	resp, env := h.do(http.MethodPost, "/xchats/api/v1/chats/"+chatID.String()+"/messages/"+otherMessageID.String()+"/retranscribe", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", resp.StatusCode, env["message"])
	}
}

func TestHandleRetranscribeMessage_OverwritesPriorTranscript(t *testing.T) {
	h := newRetranscribeHarness(t)
	sttSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"text":"новая расшифровка"}`)
	}))
	defer sttSrv.Close()
	h.configureSTT(t, sttSrv.URL)

	chatID, messageID := h.seedAudioMessage("blob-7", "ready")
	if _, err := h.blob.Put("blob-7", []byte("audio-bytes"), blob.Meta{Mimetype: "audio/ogg"}); err != nil {
		t.Fatalf("put blob: %v", err)
	}
	if err := h.store.UpdateMediaTranscript(context.Background(), "whatsapp", messageID, "старая, неверная расшифровка"); err != nil {
		t.Fatalf("seed prior transcript: %v", err)
	}

	resp, env := h.do(http.MethodPost, "/xchats/api/v1/chats/"+chatID.String()+"/messages/"+messageID.String()+"/retranscribe", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, env["message"])
	}
	msg, err := h.store.MessageByID(context.Background(), messageID)
	if err != nil {
		t.Fatalf("MessageByID: %v", err)
	}
	if msg.Media[0].Transcript != "новая расшифровка" {
		t.Fatalf("transcript = %q, want the freshly re-transcribed text", msg.Media[0].Transcript)
	}
}
