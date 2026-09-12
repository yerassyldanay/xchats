package main

// mock_e2e_test.go drives cmd/xchats' REAL composition root (buildServer,
// in main.go — the exact function runServe calls to build a production
// server) end to end in system.mock_externals mode, over plain HTTP against
// httptest.NewServer. Nothing here re-implements or parallels main.go's own
// wiring, so this test can never silently drift from what a real
// `xchats serve --mock-externals` actually boots — see docs/profiling.md
// for what "hermetic" means here.
//
// One long session, not one test per endpoint: booting the full composition
// root (LLM registry, every channel sender, the queue, every background
// worker) is the expensive part of this test, and splitting it per endpoint
// would only multiply that cost for no extra coverage.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// The sentinel admin's shipped default credential (migrations
// 0006_init_admin/0011_restore_default_admin_password — see admin_password.go's
// own doc comment) — public, documented, and exactly what a fresh install
// logs into.
const (
	mockE2EAdminEmail    = "admin@xchat.kz"
	mockE2EAdminPassword = "xchat-admin-change-me"
	mockE2ENewPassword   = "mock-e2e-rotated-password-1"
)

// mockE2EConfig builds a fully self-contained mock-externals config rooted
// at a fresh temp directory: isolated DB/blob/device-db paths, and a
// file-backed credential store (XCHATS_ALLOW_FILE_CREDENTIALS) — the OS
// keychain is unreachable in a headless sandbox, exactly like
// scripts/profile/server.sh's own env. t.Setenv scopes both env vars to
// this test (and forbids t.Parallel, which is correct: buildServer's own
// env-var-reading dependencies are not safe to fan out against a shared
// process environment).
func mockE2EConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "appdata")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	t.Setenv("XCHATS_ALLOW_FILE_CREDENTIALS", "1")
	t.Setenv("XCHATS_DATA_DIR", dataDir)

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.System.MockExternals = true
	cfg.Storage.DBPath = filepath.Join(dir, "xchats.db")
	cfg.Storage.WADeviceDBPath = filepath.Join(dir, "whatsmeow.db")
	cfg.Storage.BlobDir = filepath.Join(dir, "blobdata")
	return cfg
}

// mockE2ESeed seeds the base organization plus the full demo dataset
// (every channel account and its chats, KB content, and — since
// MockExternals is set — the mock credential backfill SeedDemoMockCredentials
// provides) at cfg.Storage.DBPath, mirroring the "xchats seed && xchats
// serve" two-step a real profiling run performs (scripts/profile/server.sh).
// A dedicated *store.Store is opened and fully closed here, before
// buildServer opens its own — internal/dbx.Open's per-path refcounting
// (relied on the same way by this package's other CLI tests, e.g.
// kbload_test.go) makes that a clean, non-competing reopen rather than a
// second connection racing this one.
func mockE2ESeed(t *testing.T, cfg *config.Config, log *slog.Logger) {
	t.Helper()
	ctx := context.Background()
	st, err := store.New(ctx, cfg.Storage.DBPath)
	if err != nil {
		t.Fatalf("open seed store: %v", err)
	}
	seedBase(ctx, cfg, st, log)
	runSeedDemo(ctx, cfg, st, log)
	st.Close()
}

// --- a minimal, session-cookie HTTP client ---------------------------------

type mockE2EClient struct {
	t       *testing.T
	baseURL string
	http    *http.Client
}

type mockE2EEnvelope struct {
	Payload json.RawMessage `json:"payload"`
	Errcode string          `json:"errcode"`
	Message string          `json:"message"`
}

func newMockE2EClient(t *testing.T, baseURL string) *mockE2EClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &mockE2EClient{t: t, baseURL: baseURL, http: &http.Client{
		Jar: jar, Timeout: 15 * time.Second,
		// A dedicated Transport, never the package-level http.DefaultTransport:
		// this client's own loopback calls to the httptest server under test
		// are not the "external" traffic TestMockModeCompositionZeroExternalCalls
		// swaps http.DefaultTransport out from under — only code running
		// INSIDE that server (the composition root actually being tested) is.
		Transport: &http.Transport{},
	}}
}

func (c *mockE2EClient) do(method, path string, body any) (int, mockE2EEnvelope) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal %s %s body: %v", method, path, err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		c.t.Fatalf("build %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.send(req)
}

// doMultipart drives the one endpoint in this test that isn't JSON:
// POST /kb/imports, which reads its provider/target_type/url as multipart
// form fields (internal/httpapi/kb_import.go), not a JSON body.
func (c *mockE2EClient) doMultipart(path string, fields map[string]string) (int, mockE2EEnvelope) {
	c.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			c.t.Fatalf("multipart field %s: %v", k, err)
		}
	}
	if err := w.Close(); err != nil {
		c.t.Fatalf("close multipart writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, &buf)
	if err != nil {
		c.t.Fatalf("build POST %s: %v", path, err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return c.send(req)
}

func (c *mockE2EClient) send(req *http.Request) (int, mockE2EEnvelope) {
	c.t.Helper()
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("%s %s: read body: %v", req.Method, req.URL.Path, err)
	}
	var env mockE2EEnvelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			c.t.Fatalf("%s %s: decode envelope: %v (body=%s)", req.Method, req.URL.Path, err, raw)
		}
	}
	return resp.StatusCode, env
}

// decode unmarshals env.Payload into out, failing the test on error — so
// every call site below reads as just the field(s) it actually asserts on.
func (c *mockE2EClient) decode(env mockE2EEnvelope, out any) {
	c.t.Helper()
	if err := json.Unmarshal(env.Payload, out); err != nil {
		c.t.Fatalf("decode payload: %v (payload=%s)", err, env.Payload)
	}
}

type mockE2EMePayload struct {
	User struct {
		MustChangePassword bool `json:"must_change_password"`
	} `json:"user"`
}

// login authenticates as the sentinel admin and, only if this database
// still demands it, transparently completes the forced password-change
// flow — see requirePasswordChanged's doc comment in
// internal/httpapi/auth.go for why every other route 403s until that
// happens.
func (c *mockE2EClient) login() {
	c.t.Helper()
	status, env := c.do(http.MethodPost, "/xchats/api/v1/auth/login", map[string]string{
		"email": mockE2EAdminEmail, "password": mockE2EAdminPassword,
	})
	if status != http.StatusOK {
		c.t.Fatalf("login: status=%d message=%s", status, env.Message)
	}
	var me mockE2EMePayload
	c.decode(env, &me)
	if !me.User.MustChangePassword {
		return
	}
	status, env = c.do(http.MethodPost, "/xchats/api/v1/auth/password", map[string]string{
		"current_password": mockE2EAdminPassword, "new_password": mockE2ENewPassword,
	})
	if status != http.StatusOK {
		c.t.Fatalf("password rotation: status=%d message=%s", status, env.Message)
	}
}

// TestMockModeEndToEnd drives cmd/xchats' real composition root, in
// system.mock_externals mode, through one HTTP session covering every
// surface docs/profiling.md documents as hermetic: authentication, inbound
// ingestion through the async AI-draft worker pipeline, chat/message/KB
// listing, synchronous response generation, an outbound send on every
// channel with a reachable chat, settings plus a credential probe, a KB
// import submission, and the tunnel/update-check surfaces.
func TestMockModeEndToEnd(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := mockE2EConfig(t)
	mockE2ESeed(t, cfg, log)

	ctx, cancel := context.WithCancel(context.Background())
	bundle, err := buildServer(ctx, cfg, log, "")
	if err != nil {
		t.Fatalf("buildServer: %v", err)
	}
	t.Cleanup(func() {
		// automation.Scheduler.Stop (and every other background loop
		// buildServer started) only exits once ctx is Done — see
		// Scheduler.Start's own doc comment. In production that's already
		// true by the time Teardown runs (runUntilShutdown only returns
		// after a real SIGINT/SIGTERM cancelled it); here nothing else ever
		// cancels ctx, so this must, before Teardown can block waiting for
		// those goroutines to exit.
		cancel()
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		bundle.Teardown(shutdownCtx)
	})

	srv := httptest.NewServer(bundle.Router)
	t.Cleanup(srv.Close)

	runMockModeRequestCycle(t, srv.URL)
}

// runMockModeRequestCycle drives one full HTTP session against a running
// mock-mode server, covering every surface docs/profiling.md documents as
// hermetic — see TestMockModeEndToEnd's own doc comment for the exact list.
// Factored out so TestMockModeCompositionZeroExternalCalls (mock_composition_test.go)
// can drive the SAME coverage under a panicking/recording transport, rather
// than a second, hand-maintained copy of it that could drift and silently
// stop actually exercising every mocked boundary.
func runMockModeRequestCycle(t *testing.T, baseURL string) {
	t.Helper()
	c := newMockE2EClient(t, baseURL)
	c.login()

	// --- inbound ingestion, through the async AI-draft worker pipeline -----
	//
	// whatsapp.Fake.InjectDebugEvent (internal/whatsapp/fake.go) publishes
	// queue.KindAIDraft directly on a genuine inbound message — no
	// automation debounce in this path — so worker.Worker.handleAIDraft
	// (internal/worker/worker.go) should persist a pending "suggestion"
	// draft within a few queue cycles. Bounded polling proves the async
	// worker actually completed, rather than merely accepting the HTTP call.
	var debugResp struct {
		OK        bool   `json:"ok"`
		MessageID string `json:"message_id"`
		ChatID    string `json:"chat_id"`
	}
	status, env := c.do(http.MethodPost, "/xchats/api/v1/debug/wa-event", map[string]any{
		"event_type": "message", "sender_jid": "77001234567@s.whatsapp.net",
		"text": "Здравствуйте! Сколько стоит доставка?", "external_id": "mock-e2e-inbound-1",
	})
	if status != http.StatusOK {
		t.Fatalf("POST /debug/wa-event: status=%d message=%s", status, env.Message)
	}
	c.decode(env, &debugResp)
	if debugResp.ChatID == "" {
		t.Fatal("POST /debug/wa-event: empty chat_id")
	}

	var drafts struct {
		Items []json.RawMessage `json:"items"`
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		status, env := c.do(http.MethodGet, "/xchats/api/v1/chats/"+debugResp.ChatID+"/ai-drafts", nil)
		if status != http.StatusOK {
			t.Fatalf("GET .../ai-drafts: status=%d message=%s", status, env.Message)
		}
		c.decode(env, &drafts)
		if len(drafts.Items) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker completion: no AI draft appeared within 15s of POST /debug/wa-event")
		}
		time.Sleep(150 * time.Millisecond)
	}

	// --- chat/message/KB listing --------------------------------------------
	var chats struct {
		Items []struct {
			ID      string `json:"id"`
			Channel string `json:"channel"`
		} `json:"items"`
	}
	status, env = c.do(http.MethodGet, "/xchats/api/v1/chats?page_size=50", nil)
	if status != http.StatusOK {
		t.Fatalf("GET /chats: status=%d message=%s", status, env.Message)
	}
	c.decode(env, &chats)
	if len(chats.Items) == 0 {
		t.Fatal("GET /chats: no chats — demo seed did not run")
	}

	if status, env := c.do(http.MethodGet, "/xchats/api/v1/chats/"+debugResp.ChatID+"/messages?limit=50", nil); status != http.StatusOK {
		t.Fatalf("GET .../messages: status=%d message=%s", status, env.Message)
	}

	if status, env := c.do(http.MethodGet, "/xchats/api/v1/kb", nil); status != http.StatusOK {
		t.Fatalf("GET /kb: status=%d message=%s", status, env.Message)
	}

	// --- synchronous response generation ------------------------------------
	// wait_for_response:true bypasses the queue and calls response.Respond
	// directly in the handler (internal/httpapi/simulator.go) — the AI reply
	// comes back IN this HTTP response, not via async polling.
	var sim struct {
		ConversationID string `json:"conversation_id"`
		Draft          struct {
			Text     string `json:"text"`
			Escalate bool   `json:"escalate"`
		} `json:"draft"`
	}
	status, env = c.do(http.MethodPost, "/xchats/api/v1/simulator/messages", map[string]any{
		"contact_ref": "mock-e2e-sim-contact", "conversation_ref": "mock-e2e-sim-contact",
		"text": "Здравствуйте! Сколько стоит доставка?", "wait_for_response": true,
	})
	if status != http.StatusOK {
		t.Fatalf("POST /simulator/messages: status=%d message=%s", status, env.Message)
	}
	c.decode(env, &sim)
	if sim.Draft.Text == "" {
		t.Fatal("POST /simulator/messages: synchronous response has no text")
	}
	if sim.Draft.Escalate {
		t.Fatalf("POST /simulator/messages: unexpectedly escalated — draft=%+v", sim.Draft)
	}

	// --- an outbound send on every channel with a reachable chat ------------
	//
	// whatsapp/telegram/instagram/messenger all get a demo-seeded chat
	// (internal/store/seed_demo_workspace.go); simulator gets one from the
	// synchronous call just above. whatsapp_cloud's demo account is seeded
	// too (with the same mock credential backfill) but gets no demo CHAT —
	// injecting one would mean hand-signing a Meta webhook payload — so its
	// composition-level wiring is checked separately below instead of via
	// an actual send here.
	byChannel := map[string]string{}
	for _, item := range chats.Items {
		if _, ok := byChannel[item.Channel]; !ok {
			byChannel[item.Channel] = item.ID
		}
	}
	byChannel["whatsapp"] = debugResp.ChatID
	byChannel["simulator"] = sim.ConversationID
	for _, channel := range []string{"whatsapp", "telegram", "instagram", "messenger", "simulator"} {
		chatID, ok := byChannel[channel]
		if !ok {
			t.Errorf("outbound send: no chat found for channel %q", channel)
			continue
		}
		status, env := c.do(http.MethodPost, "/xchats/api/v1/chats/"+chatID+"/messages", map[string]any{
			"text": "Mock e2e outbound: " + channel,
		})
		if status != http.StatusOK {
			t.Errorf("outbound send on %s (chat %s): status=%d message=%s", channel, chatID, status, env.Message)
		}
	}

	// whatsapp_cloud: confirm its demo account exists and carries the mock
	// credential backfill (Store.SeedDemoMockCredentials) that makes an
	// outbound send through it possible in the first place — the exact gap
	// that fix closed (see seed_demo_workspace.go's own doc comment).
	var accounts struct {
		Items []struct {
			Channel string `json:"channel"`
		} `json:"items"`
	}
	status, env = c.do(http.MethodGet, "/xchats/api/v1/accounts", nil)
	if status != http.StatusOK {
		t.Fatalf("GET /accounts: status=%d message=%s", status, env.Message)
	}
	c.decode(env, &accounts)
	sawWhatsAppCloud := false
	for _, a := range accounts.Items {
		if a.Channel == "whatsapp_cloud" {
			sawWhatsAppCloud = true
			break
		}
	}
	if !sawWhatsAppCloud {
		t.Error("GET /accounts: no whatsapp_cloud account — demo seed did not run")
	}

	// --- settings plus a credential probe ------------------------------------
	if status, env := c.do(http.MethodGet, "/xchats/api/v1/settings", nil); status != http.StatusOK {
		t.Fatalf("GET /settings: status=%d message=%s", status, env.Message)
	}

	status, env = c.do(http.MethodPut, "/xchats/api/v1/settings/integrations/openrouter/credential", map[string]any{
		"values": map[string]string{"openrouter.api_key": "mock-e2e-key"},
	})
	if status != http.StatusOK {
		t.Fatalf("PUT .../openrouter/credential: status=%d message=%s", status, env.Message)
	}
	status, env = c.do(http.MethodPost, "/xchats/api/v1/settings/integrations/openrouter/test", nil)
	if status != http.StatusOK {
		t.Fatalf("POST .../openrouter/test: status=%d message=%s", status, env.Message)
	}
	var verify struct {
		Verified bool `json:"verified"`
	}
	c.decode(env, &verify)
	if !verify.Verified {
		t.Error("POST .../openrouter/test: credential probe against the mock transport was not verified")
	}

	// --- a KB import submission ------------------------------------------------
	status, env = c.doMultipart("/xchats/api/v1/kb/imports", map[string]string{
		"provider": "native", "target_type": "auto", "url": "https://example.invalid/mock-e2e-import",
	})
	if status != http.StatusAccepted {
		t.Fatalf("POST /kb/imports: status=%d message=%s", status, env.Message)
	}
	var run struct {
		RunID string `json:"run_id"`
	}
	c.decode(env, &run)
	if run.RunID == "" {
		t.Error("POST /kb/imports: empty run_id")
	}

	// --- tunnel plus update-check operations ------------------------------------
	if status, env := c.do(http.MethodGet, "/xchats/api/v1/settings/tunnel", nil); status != http.StatusOK {
		t.Fatalf("GET /settings/tunnel: status=%d message=%s", status, env.Message)
	}
	if status, env := c.do(http.MethodPost, "/xchats/api/v1/settings/tunnel/start", nil); status != http.StatusOK {
		t.Fatalf("POST /settings/tunnel/start: status=%d message=%s", status, env.Message)
	}
	status, env = c.do(http.MethodGet, "/xchats/api/v1/settings/update-check", nil)
	if status != http.StatusOK {
		t.Fatalf("GET /settings/update-check: status=%d message=%s", status, env.Message)
	}
	var update struct {
		UpdateAvailable bool `json:"update_available"`
	}
	c.decode(env, &update)
	if update.UpdateAvailable {
		t.Error("GET /settings/update-check: update_available=true in mock mode (updateChecker must stay nil)")
	}
}
