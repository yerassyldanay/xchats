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
	"testing"

	"github.com/google/uuid"
	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/brain"
	"github.com/yerassyldanay/xchats/backend/internal/chat"
	"github.com/yerassyldanay/xchats/backend/internal/chatkb"
	"github.com/yerassyldanay/xchats/backend/internal/chatstore"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/kbimport"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/password"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/responsestore"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/internal/simulator"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/telegram"
	"github.com/yerassyldanay/xchats/backend/internal/tgingest"
	"github.com/yerassyldanay/xchats/backend/internal/whatsapp"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/llm"
	"github.com/yerassyldanay/xchats/backend/messaging"
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
	ownerJID    = "77011111111@s.whatsapp.net"
	customerJID = "77000000000@s.whatsapp.net"
	adminEmail  = "admin@xchats.test"
	adminPass   = "password123"

	// Telegram fixtures. The base URL must be https:// — the provisioning flow
	// refuses anything else, because Telegram itself does.
	telegramBaseURL = "https://xchats.test"
	testEncKey      = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 32 bytes, hex
	testBotID       = int64(8123456789)
	testBotUsername = "xchats_test_bot"
	testBotToken    = "8123456789:AAH-test-token_never-logged"

	tgWebhookSecret = "tg-secret_distinct-1"
)

type harness struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
	cfg    *config.Config
	fake   *whatsapp.Fake
	tg     *telegram.Fake
	queue  *queue.InMem
	store  *store.Store
	// kb is the same *kbstore.Store the Server was built with — exposed so
	// tests can seed kbd_materials rows directly (CreateUploadMaterial /
	// CompleteMaterialUpload) rather than going through the MCP tool surface
	// just to get a material into the database.
	kb *kbstore.Store
	// blob is the same blob.Store the Server serves bytes from — exposed so
	// a seeded kbd_materials row's storage_key resolves to real bytes.
	blob blob.Store
	// db is the raw handle on the same database store/kb/kbRepo share, for the
	// assertions no exported Store method covers. It replaces the old
	// h.store.Pool() escape hatch, which internal/store deliberately no longer
	// offers; internal/dbtest hands this out for exactly this purpose.
	db        *dbx.DB
	worker    *worker.Worker
	orgID     uuid.UUID
	accountID uuid.UUID
	// tgPoller records every Upsert/Remove the telegram accounts lifecycle
	// makes in polling mode (see telegram_polling_test.go) — nil-tolerant
	// production behavior means the 26 webhook-mode tests never touch it.
	tgPoller *fakeTGPoller
	// kbImport is the same *kbimport.Service the Server was built with,
	// already Start()ed (Stop is registered via t.Cleanup, before kb.Close
	// so background workers stop first) — exposed so a test can seed
	// settings/credentials the pipeline reads at call time.
	kbImport *kbimport.Service
	// kbImportSettings/kbImportCreds are the same stores kbImport itself
	// reads from — exposed so a test can configure a default model or a
	// Firecrawl/LlamaParse credential directly, without a round trip
	// through the Settings HTTP API (which this harness does not wire
	// Deps.Settings/Deps.Credentials to by default).
	kbImportSettings *settings.Store
	kbImportCreds    *credentials.Chain
	// chatLLM is the scripted model behind the /chat assistant — a separate
	// client from llmClient above because the chat surface is the only one
	// that streams, and its tests script answers per call (see
	// chat_assistant_test.go).
	chatLLM *scriptedStreamingLLM
}

// setTelegramBase rewrites the configured public base URL mid-test. The Server
// holds the same *config.Config, so this exercises the real handler path for a
// misconfigured deployment without standing up a second harness.
func (h *harness) setTelegramBase(t *testing.T, base string) {
	t.Helper()
	h.cfg.Telegram.WebhookPublicBaseURL = base
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
	ctx := context.Background()
	// A fresh, migrated database per test, created under t.TempDir() by
	// internal/dbtest. This replaces a DATABASE_URL gate that skipped this
	// whole suite whenever no Postgres was reachable — which, with no CI in
	// this repo, was always — plus the DROP SCHEMA CASCADE teardown that made
	// these packages unsafe to run in parallel against one shared database.
	st, db := dbtest.Open(t)

	cfg := &config.Config{
		System:                config.SystemConfig{SessionTTLHours: 1, MinPasswordLen: 8, SimulatorEnabled: true, CustomerMessageWaitSeconds: 5},
		PageSize:              50,
		Server:                config.ServerConfig{CORSOrigins: []string{"*"}},
		Telegram:              config.TelegramModeConfig{WebhookPublicBaseURL: telegramBaseURL},
		TelegramWebhookSecret: tgWebhookSecret,
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
	hash, _ := password.Hash(adminPass)
	if _, err := st.SeedUser(ctx, org.ID, adminEmail, hash, "Admin"); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	accountID := config.AccountID(ownerJID)
	if _, err := st.SeedAccount(ctx, store.Account{
		ID: accountID, OrganizationID: uuid.NullUUID{UUID: org.ID, Valid: true},
		DisplayName: "WhatsApp", ExternalAccountRef: config.CanonicalJID(ownerJID),
		ExternalHandle: config.PhoneFromJID(ownerJID), ConnectionState: "connected",
	}); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	// A blob under a NON-uuid key. TestDemoLoop asserts the media route serves
	// these, which matters because every other id in these tests is a uuid and
	// a route that quietly required one would still pass.
	//
	// This used to arrive via internal/assistant.NewStub, which seeded three
	// sample assets. That function is now unreferenced anywhere in the module —
	// internal/assistant is dead code — so the fixture is seeded directly here
	// rather than reviving a dependency on it.
	if _, err := blobStore.Put("sample-image", []byte("\x89PNG\r\n\x1a\nfake-sample"), blob.Meta{
		MediaType: "image", Mimetype: "image/png", FileName: "sample-image.png", FileSize: 18,
	}); err != nil {
		t.Fatalf("seed non-uuid blob: %v", err)
	}
	q := queue.NewInMem(256, 2, log)
	hub := realtime.NewHub()
	fake := whatsapp.NewFake(st, hub, q)

	// Seed the org's live KB for response and structured draft tests. kb and
	// kbRepo below open the same path st already holds, exactly as runServe
	// does — dbx.Open refcounts one connection per path, so all three share it.
	kb, err := kbstore.New(ctx, db.Path())
	if err != nil {
		t.Fatalf("kbstore: %v", err)
	}
	t.Cleanup(kb.Close)
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
	kbRepo, err := responsestore.NewKnowledgeBaseRepo(ctx, db.Path())
	if err != nil {
		t.Fatalf("kb repo: %v", err)
	}
	t.Cleanup(kbRepo.Close)
	cachedKB := responsestore.NewCachedKBRepo(kbRepo)
	responseService := &response.Service{
		Conversations: &responsestore.ConversationRepo{Store: st},
		KnowledgeBase: cachedKB,
		Drafts:        &responsestore.DraftRepo{Store: st},
		Engine: &response.Engine{
			LLMs: fakeLLMRegistry{client: llmClient},
			Params: func() response.LLMParams {
				return response.LLMParams{
					DefaultModel: llm.ModelRef{Provider: fakeLLMProvider, Model: "fake"},
					MaxTokens:    500, Temperature: 0.3, RetryEnabled: true,
				}
			},
		},
	}
	tgFake := telegram.NewFake(testBotID, testBotUsername)
	senders := messaging.NewSenderRegistry()
	senders.Register(messaging.ChannelWhatsApp, fake.ChannelSender())
	senders.Register(messaging.ChannelSimulator, simulator.NewChannelSender())
	senders.Register(messaging.ChannelTelegram, telegram.NewChannelSender(tgFake, st, blobStore))

	w := &worker.Worker{Store: st, Queue: q, TG: tgFake, Blob: blobStore, Hub: hub,
		Response: responseService, Senders: senders, Log: log}
	q.Start(context.Background(), w.Handle)

	// tgProc is the webhook ingress's shared ingest core (internal/tgingest)
	// — production wiring (main.go) always constructs one, so the harness
	// must too, over the SAME store/queue/hub every other dependency above
	// shares.
	tgProc := tgingest.New(tgingest.Deps{Store: st, Queue: q, Hub: hub, Log: log})
	tgPoller := &fakeTGPoller{}

	// kbImportSettings gives kbimport.Service.resolveSynthModel a default
	// model that resolves through the SAME fakeLLMRegistry/llmClient the
	// response engine above already uses — so a test wanting a real pass-2
	// synthesis run just scripts llmClient's calls list, the same way
	// TestWhatsAppInboundProducesGroundedDraftAndApprovalDelivers scripts a
	// customer-response fixture.
	kbImportSettings := settings.NewStore(t.TempDir())
	if _, err := kbImportSettings.Update(func(s *settings.Settings) {
		s.LLM.DefaultProvider, s.LLM.DefaultModel = fakeLLMProvider, "fake"
	}); err != nil {
		t.Fatalf("seed kb import settings: %v", err)
	}
	// kbImportCreds is a real, file-backed credentials.Chain (not wired into
	// Deps.Credentials, so /settings/integrations itself stays untouched by
	// default) — a test wanting a configured Firecrawl/LlamaParse credential
	// calls h.kbImportCreds.Set(ctx, "firecrawl.api_key", "...") directly
	// rather than round-tripping through the Settings HTTP API.
	kbImportCreds, err := credentials.Open(credentials.OpenOptions{AllowFile: true, ForceFile: true, DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("open kb import credentials store: %v", err)
	}
	kbImportSvc := kbimport.New(kbimport.Deps{
		KB: kb, Blob: blobStore, Settings: kbImportSettings, Credentials: kbImportCreds,
		LLM: fakeLLMRegistry{client: llmClient},
		Hub: hub, Log: log, AllowPrivateFetch: true,
	}, kbimport.DefaultConfig())
	kbImportCtx, cancelKBImport := context.WithCancel(context.Background())
	kbImportSvc.Start(kbImportCtx)
	t.Cleanup(func() { cancelKBImport(); kbImportSvc.Stop() })

	// The Knowledge Base chat assistant, wired exactly as main.go does it —
	// same database, the same kbstore behind chatkb retrieval — but with a
	// scripted, streaming model so the SSE surface can be exercised without a
	// network call.
	chatDB, err := chatstore.New(ctx, db.Path())
	if err != nil {
		t.Fatalf("chatstore: %v", err)
	}
	t.Cleanup(chatDB.Close)
	chatLLM := &scriptedStreamingLLM{}
	chatSvc := chat.New(chat.Deps{
		Store: chatDB,
		KB:    chatkb.NewStoreService(kb),
		LLMs:  fakeLLMRegistry{client: chatLLM},
		Params: func() chat.Params {
			return chat.Params{
				Model:     llm.ModelRef{Provider: fakeLLMProvider, Model: "fake"},
				MaxTokens: 500, Temperature: 0.3, HistoryWindow: 4,
			}
		},
		Log: log,
	})

	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Queue: q, Hub: hub, Blob: blobStore,
		Response: responseService, WA: fake, TG: tgFake, TGProcessor: tgProc, TGPoller: tgPoller, KB: kb,
		KBImport: kbImportSvc,
		KBRepo:   cachedKB, KBInvalidator: cachedKB,
		Chat:  chatSvc,
		OrgID: org.ID, Log: log,
	})
	ts := httptest.NewServer(srv.Router())
	jar, _ := cookiejar.New(nil)
	h := &harness{t: t, srv: ts, client: &http.Client{Jar: jar}, cfg: cfg, fake: fake, tg: tgFake,
		queue: q, store: st, kb: kb, blob: blobStore, db: db, worker: w, orgID: org.ID, accountID: accountID, tgPoller: tgPoller,
		kbImport: kbImportSvc, kbImportSettings: kbImportSettings, kbImportCreds: kbImportCreds,
		chatLLM: chatLLM}
	// st/db/kb/kbRepo are all closed by dbtest's own t.Cleanup registrations.
	t.Cleanup(func() { ts.Close(); q.Close() })
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

// inject feeds one synthetic WhatsApp message through the exact same
// ingest path a real whatsmeow event (or a POST /debug/wa-event call) takes
// — see whatsapp.Fake.InjectDebugEvent. senderJID is the customer's JID for
// both an inbound message and the fromMe echo of our own send (the contact
// identity never changes with direction — see internal/whatsmeow/jid.go).
func (h *harness) inject(senderJID, externalID, text string, fromMe bool) (chatID, messageID string) {
	return h.injectFor(h.accountID.String(), senderJID, externalID, text, fromMe)
}

// injectFor is inject, parameterized on the account — for multi-account tests.
func (h *harness) injectFor(accountID, senderJID, externalID, text string, fromMe bool) (chatID, messageID string) {
	res, err := h.fake.InjectDebugEvent(context.Background(), whatsapp.DebugEvent{
		EventType: "message", AccountID: accountID, SenderJID: senderJID,
		ExternalID: externalID, Text: text, IsFromMe: fromMe,
	})
	if err != nil {
		h.t.Fatalf("inject message: %v", err)
	}
	h.queue.Wait()
	return res.ChatID, res.MessageID
}

// injectReceipt advances the delivery ladder for externalID — receiptType is
// "" (delivered), "read", or "played", matching whatsapp.DebugEvent's own
// ReceiptType contract.
func (h *harness) injectReceipt(externalID, receiptType string) {
	if _, err := h.fake.InjectDebugEvent(context.Background(), whatsapp.DebugEvent{
		EventType: "receipt", AccountID: h.accountID.String(), ExternalID: externalID, ReceiptType: receiptType,
	}); err != nil {
		h.t.Fatalf("inject receipt: %v", err)
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

// --- the demo loop --------------------------------------------------------

func TestDemoLoop(t *testing.T) {
	h := newHarness(t)

	// 1. inbound text appears, and survives a "refresh" (GET hydrates).
	h.inject(customerJID, "WA-TEXT-1", "привет от клиента", false)
	chats := h.listChats()
	if len(chats) != 1 {
		t.Fatalf("want 1 chat, got %d", len(chats))
	}
	chatID := chats[0]["id"].(string)
	msgs := h.listMessages(chatID)
	if len(msgs) != 1 || msgs[0]["content"] != "привет от клиента" {
		t.Fatalf("inbound text not stored: %v", msgs)
	}
	if msgs[0]["direction"] != "in" || msgs[0]["sender_type"] != "contact" {
		t.Fatalf("wrong direction/sender: %v", msgs[0])
	}

	// 2. dedup: the SAME external id delivered twice is a no-op.
	h.inject(customerJID, "WA-TEXT-1", "привет от клиента", false)
	if got := len(h.listMessages(chatID)); got != 1 {
		t.Fatalf("dedup failed: %d messages after replay", got)
	}

	// 3. send fan-out: text + 2 media → 2 outbound messages, each to the
	// provider's original conversation address. The text rides as the CAPTION
	// of the first resolvable asset
	// rather than becoming its own bubble — see sendParts in inbox.go, which
	// does this deliberately "so the customer (and the inbox) see ONE message —
	// image + caption — instead of a text bubble followed by an empty media
	// bubble". So there is no standalone text-only send at all here.
	m1 := h.upload("a.png", "image/png", []byte("\x89PNG\r\n\x1a\nfake"))
	m2 := h.upload("b.pdf", "application/pdf", []byte("%PDF-1.4 fake"))
	resp, env := h.postJSON("/xchats/api/v1/chats/"+chatID+"/messages",
		map[string]any{"text": "привет", "media_ids": []string{m1, m2}})
	if resp.StatusCode != 200 {
		t.Fatalf("send status %d", resp.StatusCode)
	}
	var sent struct{ Items []map[string]any }
	mustPayload(t, env, &sent)
	if len(sent.Items) != 2 {
		t.Fatalf("fan-out: want 2 messages (caption + captionless), got %d", len(sent.Items))
	}
	if got := len(h.fake.TextCalls()); got != 0 {
		t.Fatalf("want 0 text-only sends (the text is the first asset's caption), got %d", got)
	}
	if got := len(h.fake.MediaCalls()); got != 2 {
		t.Fatalf("want 2 media sends, got %d", got)
	}
	for _, call := range h.fake.Calls {
		if call.To != customerJID {
			t.Fatalf("send went to %q, want the original conversation ref %q", call.To, customerJID)
		}
	}

	// the outbound text row was stamped with the provider's own message id.
	stampedID := stampedKeyOf(t, h, chatID, "привет")

	// 4. echo: the fromMe=true echo of our own send collapses onto the row — no dup.
	before := len(h.listMessages(chatID))
	h.inject(customerJID, stampedID, "привет", true)
	if after := len(h.listMessages(chatID)); after != before {
		t.Fatalf("echo created a duplicate bubble: %d -> %d", before, after)
	}

	// 5. status advances monotonically: delivered, then read, then a stale ack is ignored.
	h.injectReceipt(stampedID, "")
	if st := deliveryOf(t, h, chatID, stampedID); st != "delivered" {
		t.Fatalf("status after delivered receipt = %q", st)
	}
	h.injectReceipt(stampedID, "read")
	if st := deliveryOf(t, h, chatID, stampedID); st != "read" {
		t.Fatalf("status after read receipt = %q", st)
	}
	h.injectReceipt(stampedID, "") // backwards — must be ignored
	if st := deliveryOf(t, h, chatID, stampedID); st != "read" {
		t.Fatalf("status regressed to %q (not monotonic)", st)
	}

	// 6. Suggest → one option; approve once sends as sender_type=ai; approve
	// again → 409.
	//
	// response.Service's DraftRepository ReplaceSuggested persists ONE
	// suggested option per conversation and supersedes any prior — enforced
	// in the schema by the partial unique index ai_drafts_pending_uq.
	sendTextBefore := len(h.fake.TextCalls())
	h.postJSON("/xchats/api/v1/chats/"+chatID+"/ai-drafts", map[string]any{})
	var drafts struct{ Items []map[string]any }
	h.get("/xchats/api/v1/chats/"+chatID+"/ai-drafts", &drafts)
	if len(drafts.Items) != 1 {
		t.Fatalf("want 1 draft option, got %d", len(drafts.Items))
	}
	draftID := drafts.Items[0]["id"].(string)
	resp, _ = h.postJSON("/xchats/api/v1/ai-drafts/"+draftID+"/approve", map[string]any{"media_ids": []string{}})
	if resp.StatusCode != 200 {
		t.Fatalf("approve status %d", resp.StatusCode)
	}
	if got := len(h.fake.TextCalls()); got != sendTextBefore+1 {
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
	if got := len(h.fake.TextCalls()); got != sendTextBefore+1 {
		t.Fatalf("double approve double-sent: %d", got)
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
			if id, _ := m["external_message_id"].(string); id != "" {
				return id
			}
		}
	}
	t.Fatalf("no stamped outbound message for %q", text)
	return ""
}

func deliveryOf(t *testing.T, h *harness, chatID, extID string) string {
	for _, m := range h.listMessages(chatID) {
		if m["external_message_id"] == extID {
			return m["status"].(string)
		}
	}
	t.Fatalf("message %s not found", extID)
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
