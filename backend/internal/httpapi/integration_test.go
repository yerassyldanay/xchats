package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/evolution"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/simulator"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
	"github.com/yerassyldanay/xchats/backend/migrations"
	"github.com/yerassyldanay/xchats/backend/response"
	"log/slog"
)

// fakeLLMClient/fakeLLMRegistry stand in for a real provider in this harness.
// No test here asserts on generated draft CONTENT beyond what this scripted
// response fixes — its purpose is letting handleAIDraft/the simulator API run
// to completion instead of hitting a nil dependency or a real network call.
// fakeLLMRegistry only recognizes fakeLLMProvider (mirroring a real
// llm.Registry's per-provider rejection), so tests can still exercise the
// "unregistered provider" validation path with a provider name it doesn't know.
const fakeLLMProvider = "fake"

type fakeLLMClient struct{}

func (fakeLLMClient) Complete(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	return llm.ChatResponse{Text: `{"reply_text":"Секунду, уточню и вернусь.","reply_language":"ru","media_files_to_send":[],"escalate":false}`}, nil
}

type fakeLLMRegistry struct{ client llm.ChatClient }

func (r fakeLLMRegistry) Client(ref llm.ModelRef) (llm.ChatClient, error) {
	if ref.Provider != fakeLLMProvider {
		return nil, fmt.Errorf("fakeLLMRegistry: no client configured for provider %q", ref.Provider)
	}
	return r.client, nil
}

const (
	ownerJID     = "77011111111@s.whatsapp.net"
	customerJID  = "77000000000@s.whatsapp.net"
	customerNum  = "77000000000"
	webhookToken = "test-token"
	adminEmail   = "admin@xchats.test"
	adminPass    = "password123"

	// Telegram fixtures. The base URL must be https:// — the provisioning flow
	// refuses anything else, because Telegram itself does.
	telegramBaseURL = "https://xchats.test"
	testEncKey      = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 32 bytes, hex
	testBotID       = int64(8123456789)
	testBotUsername = "xchats_test_bot"
	testBotToken    = "8123456789:AAH-test-token_never-logged"

	// tgWebhookSecret is deliberately DIFFERENT from webhookToken: Telegram gets
	// its own TG_WEBHOOK_SECRET, distinct from Evolution's shared WEBHOOK_TOKEN.
	// Any code that resolves the wrong one fails these tests instead of passing
	// by coincidence.
	tgWebhookSecret = "tg-secret_distinct-1"
)

type harness struct {
	t         *testing.T
	srv       *httptest.Server
	client    *http.Client
	cfg       *config.Config
	fake      *evolution.Fake
	tg        *telegram.Fake
	queue     *queue.InMem
	store     *store.Store
	worker    *worker.Worker
	orgID     uuid.UUID
	accountID uuid.UUID
}

// setTelegramBase rewrites the configured public base URL mid-test. The Server
// holds the same *config.Config, so this exercises the real handler path for a
// misconfigured deployment without standing up a second harness.
func (h *harness) setTelegramBase(t *testing.T, base string) {
	t.Helper()
	h.cfg.TelegramWebhookPublicBaseURL = base
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWithLLM(t, fakeLLMClient{})
}

// newHarnessWithLLM is newHarness parameterized on the scripted LLM client the
// org's response.Service reaches. Most tests don't care what the model
// "says" beyond a fixed, valid response (newHarness's fakeLLMClient); one
// (TestWhatsAppInboundProducesGroundedDraftAndApprovalDelivers) needs a
// scripted reply that names a real KB fact placeholder, to assert on
// grounding/substitution end to end rather than just plumbing.
func newHarnessWithLLM(t *testing.T, llmClient llm.ChatClient) *harness {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping DB integration test")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	// fresh schema each run
	if _, err := st.Pool().Exec(ctx, `DROP SCHEMA IF EXISTS xchats CASCADE; DROP TABLE IF EXISTS public.xchats_schema_migrations`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := store.RunMigrations(ctx, st.Pool(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{
		WebhookToken: webhookToken, SessionTTLHours: 1, MinPasswordLen: 8,
		PageSize: 50, CORSOrigins: []string{"*"}, SimulatorEnabled: true,
		TelegramWebhookPublicBaseURL: telegramBaseURL,
		TelegramWebhookSecret:        tgWebhookSecret,
	}
	// Credential encryption is required for the Telegram lifecycle; the key is
	// test-local and deterministic so a sealed token can be asserted on.
	box, err := secretbox.FromEnvValue(testEncKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	st.UseCredentialsBox(box)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// seed org + admin + account
	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	hash, _ := httpapi.HashPassword(adminPass)
	if _, err := st.SeedUser(ctx, org.ID, adminEmail, hash, "Admin"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	accountID := config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "WhatsApp", ExternalAccountRef: config.CanonicalJID(ownerJID),
		ExternalHandle: config.PhoneFromJID(ownerJID), InstanceName: "xpayment", ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	q := queue.NewInMem(256, 2, log)
	hub := realtime.NewHub()
	fake := evolution.NewFake("xpayment", ownerJID)

	// Seed the org's live KB for response and structured draft tests.
	kb := kbstore.New(st.Pool())
	if err := kb.SeedLiveIfEmpty(ctx, org.ID, brain.SeedSnapshot()); err != nil {
		t.Fatalf("seed kb: %v", err)
	}
	// A minimal, real (fake-LLM-backed) response.Service. kb.SeedLiveIfEmpty
	// above already gives the org an ai_assistants row (among other tables), so
	// KnowledgeBaseRepo.Load succeeds and Respond calls all the way through to
	// fakeLLMClient's scripted response — no test here asserts on generated
	// draft CONTENT beyond that fixed response, only that the async
	// ai_draft/outbound_send tasks complete instead of panicking on a nil
	// dependency. cachedKB mirrors main.go's production wiring exactly (a
	// CachedKBRepo, not the raw KnowledgeBaseRepo) so every test built on this
	// harness doubles as a regression check that caching the response engine's
	// KB read leaves production behavior byte-identical.
	cachedKB := responsestore.NewCachedKBRepo(&responsestore.KnowledgeBaseRepo{Pool: st.Pool()})
	responseService := &response.Service{
		Conversations: &responsestore.ConversationRepo{Store: st},
		KnowledgeBase: cachedKB,
		Drafts:        &responsestore.DraftRepo{Store: st},
		Engine: &response.Engine{
			LLMs: fakeLLMRegistry{client: llmClient}, DefaultModel: llm.ModelRef{Provider: fakeLLMProvider, Model: "fake"},
			MaxTokens: 500, Temperature: 0.3, RetryEnabled: true,
		},
	}
	tgFake := telegram.NewFake(testBotID, testBotUsername)
	senders := messaging.NewSenderRegistry()
	senders.Register(messaging.ChannelWhatsApp, evolution.NewChannelSender(fake, blobStore))
	senders.Register(messaging.ChannelSimulator, simulator.NewChannelSender())
	senders.Register(messaging.ChannelTelegram, telegram.NewChannelSender(tgFake, st, blobStore))

	w := &worker.Worker{Store: st, Queue: q, Evo: fake, TG: tgFake, Blob: blobStore, Hub: hub,
		Response: responseService, Senders: senders, Log: log}
	q.Start(context.Background(), w.Handle)

	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Queue: q, Hub: hub, Blob: blobStore,
		Response: responseService, Evo: fake, TG: tgFake, KB: kb,
		KBRepo: cachedKB, KBInvalidator: cachedKB,
		OrgID: org.ID, Log: log,
	})
	ts := httptest.NewServer(srv.Router())
	jar, _ := cookiejar.New(nil)
	h := &harness{t: t, srv: ts, client: &http.Client{Jar: jar}, cfg: cfg, fake: fake, tg: tgFake,
		queue: q, store: st, worker: w, orgID: org.ID, accountID: accountID}
	t.Cleanup(func() {
		ts.Close()
		// Mirror main.go's two-phase shutdown ordering. Phase 1: let any
		// debounce timer still firing finish its publish. Phase 2, only once
		// phase 1 is done (so nothing NEW can be published): drain the queue,
		// so a task a timer just published isn't still mid-process on a
		// worker goroutine when q.Close()/st.Close() run out from under it.
		// queue.Publish also recovers from that race now, but this keeps the
		// common case clean instead of relying on the recover.
		waitWithTimeout(w.StopDebounceTimers(), 2*time.Second)
		queueDrained := make(chan struct{})
		go func() { q.Wait(); close(queueDrained) }()
		waitWithTimeout(queueDrained, 2*time.Second)
		q.Close()
		st.Close()
	})
	h.login()
	return h
}

func (h *harness) login() {
	body, _ := json.Marshal(map[string]string{"email": adminEmail, "password": adminPass})
	resp, err := h.client.Post(h.srv.URL+"/xchats/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		h.t.Fatalf("login: %v status=%v body=%s", err, resp.StatusCode, b)
	}
}

func (h *harness) webhook(rawBody []byte) {
	req, _ := http.NewRequest("POST", h.srv.URL+"/evolution/api/v1/webhook/"+h.accountID.String(), bytes.NewReader(rawBody))
	req.Header.Set("X-Webhook-Token", webhookToken)
	resp, err := h.client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		h.t.Fatalf("webhook: %v status=%v", err, statusOf(resp))
	}
	h.queue.Wait()
}

func (h *harness) getBytes(path string) (int, []byte) {
	resp, err := h.client.Get(h.srv.URL + path)
	if err != nil {
		h.t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes()
}

func (h *harness) get(path string, out any) {
	resp, err := h.client.Get(h.srv.URL + path)
	if err != nil {
		h.t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	decodeEnvelope(h.t, resp, out)
}

func (h *harness) postJSON(path string, body any) (*http.Response, map[string]json.RawMessage) {
	b, _ := json.Marshal(body)
	resp, err := h.client.Post(h.srv.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		h.t.Fatalf("POST %s: %v", path, err)
	}
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	h.queue.Wait()
	return resp, env
}

func (h *harness) putJSON(path string, body any) (*http.Response, map[string]json.RawMessage) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPut, h.srv.URL+path, bytes.NewReader(b))
	if err != nil {
		h.t.Fatalf("PUT %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("PUT %s: %v", path, err)
	}
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	h.queue.Wait()
	return resp, env
}

// waitWithTimeout blocks until done closes or timeout elapses, whichever is
// first — the bounded-wait building block newHarnessWithLLM's Cleanup uses
// to mirror main.go's shutdown ordering.
func waitWithTimeout(done <-chan struct{}, timeout time.Duration) {
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// waitFor polls cond until it returns true, or fails the test after ~2s.
// The debounce touch (see internal/worker/debounce.go) arms a real timer on
// its own goroutine, outside anything h.queue.Wait() tracks — this is the
// one kind of async work in this harness that a bare postJSON/webhook call
// cannot wait out, so any assertion that depends on a debounced draft having
// been generated needs to poll for it instead of reading right away.
func (h *harness) waitFor(desc string, cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("timed out waiting for: %s", desc)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitForDraftCount polls chatID's ai-drafts list until it holds exactly
// want items (see waitFor) and returns the final list.
func (h *harness) waitForDraftCount(chatID string, want int) []map[string]any {
	h.t.Helper()
	var drafts struct {
		Items []map[string]any `json:"items"`
	}
	h.waitFor(fmt.Sprintf("chat %s to have %d draft(s)", chatID, want), func() bool {
		h.get("/xchats/api/v1/chats/"+chatID+"/ai-drafts", &drafts)
		return len(drafts.Items) == want
	})
	return drafts.Items
}

func (h *harness) patchJSON(path string, body any) (*http.Response, map[string]json.RawMessage) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPatch, h.srv.URL+path, bytes.NewReader(b))
	if err != nil {
		h.t.Fatalf("PATCH %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("PATCH %s: %v", path, err)
	}
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	h.queue.Wait()
	return resp, env
}

// --- fixtures + crafted events -------------------------------------------

func capture(t *testing.T, name string) []byte {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "plan", "captures", "samples", "webhook_bodies", name))
	if err != nil {
		t.Fatalf("read capture: %v", err)
	}
	return b
}

func craftEcho(keyID, text string) []byte {
	ev := map[string]any{
		"event": "send.message", "instance": "xpayment", "sender": ownerJID,
		"data": map[string]any{
			"key":         map[string]any{"remoteJid": customerJID, "fromMe": true, "id": keyID},
			"status":      "PENDING",
			"message":     map[string]any{"conversation": text},
			"messageType": "conversation", "messageTimestamp": 1781460000,
		},
	}
	b, _ := json.Marshal(ev)
	return b
}

func craftStatus(keyID, status string) []byte {
	ev := map[string]any{
		"event": "messages.update", "instance": "xpayment", "sender": ownerJID,
		"data": map[string]any{"keyId": keyID, "remoteJid": "5200000000000@lid", "fromMe": true, "status": status},
	}
	b, _ := json.Marshal(ev)
	return b
}

// --- the demo loop --------------------------------------------------------

func TestDemoLoop(t *testing.T) {
	h := newHarness(t)

	// 1. inbound text appears, and survives a "refresh" (GET hydrates).
	h.webhook(capture(t, "messages_upsert_text.json"))
	chats := h.listChats()
	if len(chats) != 1 {
		t.Fatalf("want 1 chat, got %d", len(chats))
	}
	chatID := chats[0]["id"].(string)
	msgs := h.listMessages(chatID)
	if len(msgs) != 1 || msgs[0]["content"] != "[snapshot] webhook capture - text" {
		t.Fatalf("inbound text not stored: %v", msgs)
	}
	if msgs[0]["direction"] != "in" || msgs[0]["sender_type"] != "contact" {
		t.Fatalf("wrong direction/sender: %v", msgs[0])
	}

	// 2. dedup: the SAME event delivered twice is a no-op.
	h.webhook(capture(t, "messages_upsert_text.json"))
	if got := len(h.listMessages(chatID)); got != 1 {
		t.Fatalf("dedup failed: %d messages after replay", got)
	}

	// 3. inbound image → a media row appears (list of urls non-empty).
	h.webhook(capture(t, "messages_upsert_image.json"))
	msgs = h.listMessages(chatID)
	var imageMsg map[string]any
	for _, m := range msgs {
		if m["message_type"] == "imageMessage" {
			imageMsg = m
		}
	}
	if imageMsg == nil {
		t.Fatalf("image message missing")
	}
	mediaList, _ := imageMsg["media"].([]any)
	if len(mediaList) == 0 {
		t.Fatalf("image message has no media urls")
	}
	// the media URL streams real bytes (downloaded via the fake getBase64 fallback).
	imgURL := mediaList[0].(map[string]any)["url"].(string)
	if st, body := h.getBytes(imgURL); st != 200 || len(body) == 0 {
		t.Fatalf("GET %s = %d, %d bytes", imgURL, st, len(body))
	}
	// the stub sample asset (a non-uuid blob id) also serves.
	if st, body := h.getBytes("/xchats/api/v1/media/sample-image"); st != 200 || len(body) == 0 {
		t.Fatalf("GET stub sample = %d, %d bytes", st, len(body))
	}

	// 4. send fan-out: text + 2 media → 3 outbound messages, each to the phone (not @lid).
	m1 := h.upload("a.png", "image/png", []byte("\x89PNG\r\n\x1a\nfake"))
	m2 := h.upload("b.pdf", "application/pdf", []byte("%PDF-1.4 fake"))
	resp, env := h.postJSON("/xchats/api/v1/chats/"+chatID+"/messages",
		map[string]any{"text": "привет", "media_ids": []string{m1, m2}})
	if resp.StatusCode != 200 {
		t.Fatalf("send status %d", resp.StatusCode)
	}
	var sent struct{ Items []map[string]any }
	mustPayload(t, env, &sent)
	if len(sent.Items) != 3 {
		t.Fatalf("fan-out: want 3 messages, got %d", len(sent.Items))
	}
	if got := len(h.fake.CallsFor("sendText")); got != 1 {
		t.Fatalf("want 1 sendText, got %d", got)
	}
	if got := len(h.fake.CallsFor("sendMedia")); got != 2 {
		t.Fatalf("want 2 sendMedia, got %d", got)
	}
	for _, call := range h.fake.Calls {
		if call.Number != customerNum {
			t.Fatalf("send went to %q, want the phone %q (not @lid)", call.Number, customerNum)
		}
	}

	// the outbound text row was stamped with the gateway key id.
	stampedID := stampedKeyOf(t, h, chatID, "привет")

	// 5. echo: the fromMe=true echo of our own send collapses onto the row — no dup.
	before := len(h.listMessages(chatID))
	h.webhook(craftEcho(stampedID, "привет"))
	if after := len(h.listMessages(chatID)); after != before {
		t.Fatalf("echo created a duplicate bubble: %d -> %d", before, after)
	}

	// 6. status advances monotonically: delivered, then read, then a stale ack is ignored.
	h.webhook(craftStatus(stampedID, "DELIVERY_ACK"))
	if st := deliveryOf(t, h, chatID, stampedID); st != "delivered" {
		t.Fatalf("status after DELIVERY_ACK = %q", st)
	}
	h.webhook(craftStatus(stampedID, "READ"))
	if st := deliveryOf(t, h, chatID, stampedID); st != "read" {
		t.Fatalf("status after READ = %q", st)
	}
	h.webhook(craftStatus(stampedID, "DELIVERY_ACK")) // backwards — must be ignored
	if st := deliveryOf(t, h, chatID, stampedID); st != "read" {
		t.Fatalf("status regressed to %q (not monotonic)", st)
	}

	// 7. Suggest → 3 options; approve once sends as sender_type=ai; approve again → 409.
	sendTextBefore := len(h.fake.CallsFor("sendText"))
	h.postJSON("/xchats/api/v1/chats/"+chatID+"/ai-drafts", map[string]any{})
	var drafts struct{ Items []map[string]any }
	h.get("/xchats/api/v1/chats/"+chatID+"/ai-drafts", &drafts)
	if len(drafts.Items) != 3 {
		t.Fatalf("want 3 draft options, got %d", len(drafts.Items))
	}
	draftID := drafts.Items[0]["id"].(string)
	resp, _ = h.postJSON("/xchats/api/v1/ai-drafts/"+draftID+"/approve", map[string]any{"media_ids": []string{}})
	if resp.StatusCode != 200 {
		t.Fatalf("approve status %d", resp.StatusCode)
	}
	if got := len(h.fake.CallsFor("sendText")); got != sendTextBefore+1 {
		t.Fatalf("approve should have sent exactly one text; before=%d after=%d", sendTextBefore, got)
	}
	if !aiMessageExists(t, h, chatID) {
		t.Fatalf("approved send was not recorded as sender_kind='ai'")
	}
	// approve again → guarded single-send rejects.
	resp, env = h.postJSON("/xchats/api/v1/ai-drafts/"+draftID+"/approve", map[string]any{})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second approve status = %d, want 409", resp.StatusCode)
	}
	if got := len(h.fake.CallsFor("sendText")); got != sendTextBefore+1 {
		t.Fatalf("double approve double-sent: %d", got)
	}
}

func TestWebhookRejectsBadToken(t *testing.T) {
	h := newHarness(t)
	req, _ := http.NewRequest("POST", h.srv.URL+"/evolution/api/v1/webhook/"+h.accountID.String(), bytes.NewReader(capture(t, "messages_upsert_text.json")))
	req.Header.Set("X-Webhook-Token", "wrong")
	resp, _ := h.client.Do(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad token status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthRequired(t *testing.T) {
	h := newHarness(t)
	noauth := &http.Client{}
	resp, _ := noauth.Get(h.srv.URL + "/xchats/api/v1/chats")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated chats = %d, want 401", resp.StatusCode)
	}
}

// --- helpers --------------------------------------------------------------

func (h *harness) listChats() []map[string]any {
	var out struct{ Items []map[string]any }
	h.get("/xchats/api/v1/chats", &out)
	return out.Items
}

func (h *harness) listMessages(chatID string) []map[string]any {
	var out struct{ Items []map[string]any }
	h.get("/xchats/api/v1/chats/"+chatID+"/messages", &out)
	return out.Items
}

func (h *harness) upload(name, ct string, data []byte) string {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(map[string][]string)
	hdr["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="file"; filename=%q`, name)}
	hdr["Content-Type"] = []string{ct}
	pw, _ := mw.CreatePart(hdr)
	pw.Write(data)
	mw.Close()
	req, _ := http.NewRequest("POST", h.srv.URL+"/xchats/api/v1/media", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	var env struct {
		Payload struct {
			MediaID string `json:"media_id"`
		} `json:"payload"`
	}
	json.NewDecoder(resp.Body).Decode(&env)
	if env.Payload.MediaID == "" {
		h.t.Fatalf("upload returned no media_id")
	}
	return env.Payload.MediaID
}

func decodeEnvelope(t *testing.T, resp *http.Response, out any) {
	var env struct {
		Payload json.RawMessage `json:"payload"`
		Errcode string          `json:"errcode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out != nil && len(env.Payload) > 0 {
		if err := json.Unmarshal(env.Payload, out); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
	}
}

func mustPayload(t *testing.T, env map[string]json.RawMessage, out any) {
	if err := json.Unmarshal(env["payload"], out); err != nil {
		t.Fatalf("payload: %v", err)
	}
}

func stampedKeyOf(t *testing.T, h *harness, chatID, text string) string {
	for _, m := range h.listMessages(chatID) {
		if m["content"] == text && m["direction"] == "out" {
			if id, _ := m["evolution_message_id"].(string); id != "" {
				return id
			}
		}
	}
	t.Fatalf("no stamped outbound message for %q", text)
	return ""
}

func deliveryOf(t *testing.T, h *harness, chatID, evID string) string {
	for _, m := range h.listMessages(chatID) {
		if m["evolution_message_id"] == evID {
			return m["status"].(string)
		}
	}
	t.Fatalf("message %s not found", evID)
	return ""
}

func aiMessageExists(t *testing.T, h *harness, chatID string) bool {
	for _, m := range h.listMessages(chatID) {
		if m["sender_type"] == "ai" {
			return true
		}
	}
	return false
}

func statusOf(r *http.Response) int {
	if r == nil {
		return 0
	}
	return r.StatusCode
}
