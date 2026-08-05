package httpapi_test

// settings_test.go covers Track 2F's /settings/** surface: the
// integrations list/save/delete/test routes, LLM/credential-storage/
// setup-complete settings, and the tunnel status/start/stop routes. It uses
// its own lightweight harness (settingsHarness) rather than the big
// newHarness in integration_test.go — settings handlers never touch
// WhatsApp/Telegram/KB/the response engine, so none of that machinery is
// worth standing up here.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/providerhealth"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/tunnel"
)

// fakeTunnelController is an httpapi-local double for tunnel.Tunnel — real
// network access to ngrok is out of scope here; internal/tunnel has its
// own, much deeper test suite for the real Start/Stop/Status logic. This
// only proves httpapi calls it correctly and maps its results.
type fakeTunnelController struct {
	mu       sync.Mutex
	status   tunnel.Status
	startErr error
	stopErr  error
	starts   int
	stops    int
}

func (f *fakeTunnelController) Start(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	if f.startErr != nil {
		f.status = tunnel.Status{LastError: f.startErr.Error()}
		return f.startErr
	}
	now := time.Now().UTC()
	f.status = tunnel.Status{Running: true, PublicURL: "https://fake.ngrok.app", StartedAt: &now}
	return nil
}

func (f *fakeTunnelController) Stop(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	if f.stopErr != nil {
		return f.stopErr
	}
	f.status = tunnel.Status{}
	return nil
}

func (f *fakeTunnelController) Status() tunnel.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

var _ tunnel.Tunnel = (*fakeTunnelController)(nil)

type settingsHarness struct {
	t               *testing.T
	srv             *httptest.Server
	client          *http.Client
	store           *store.Store
	orgID           uuid.UUID
	creds           *credentials.Chain
	sets            *settings.Store
	tun             *fakeTunnelController
	llmRefreshCalls *int32
	health          *providerhealth.Tracker
}

// newSettingsHarness builds a harness with a REAL credentials.Chain
// (file-backed, forced via AllowFile so it never depends on this sandbox's
// OS keychain) and a REAL settings.Store, both rooted under t.TempDir() —
// exercising the actual production wiring, not a hand-rolled fake, for the
// two packages this surface is really about. Only the tunnel gets a fake,
// since a real one needs a live ngrok account.
func newSettingsHarness(t *testing.T) *settingsHarness {
	t.Helper()
	ctx := context.Background()
	st, _ := dbtest.Open(t)

	cfg := &config.Config{
		System:   config.SystemConfig{SessionTTLHours: 1, MinPasswordLen: 8},
		PageSize: 50,
		Server:   config.ServerConfig{CORSOrigins: []string{"*"}},
	}

	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	hash, _ := httpapi.HashPassword(adminPass)
	if _, err := st.SeedUser(ctx, org.ID, adminEmail, hash, "Admin"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	dataDir := t.TempDir()
	creds, err := credentials.Open(credentials.OpenOptions{AllowFile: true, DataDir: dataDir})
	if err != nil {
		t.Fatalf("credentials.Open: %v", err)
	}
	sets := settings.NewStore(t.TempDir())
	tun := &fakeTunnelController{}
	var llmRefreshCalls int32

	health := providerhealth.NewTracker(nil)

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, OrgID: org.ID, Log: log,
		Credentials: creds, Settings: sets, Tunnel: tun,
		LLMRefresh:     func() { atomic.AddInt32(&llmRefreshCalls, 1) },
		ProviderHealth: health,
	})
	ts := httptest.NewServer(srv.Router())
	jar, _ := cookiejar.New(nil)
	h := &settingsHarness{
		t: t, srv: ts, client: &http.Client{Jar: jar}, store: st, orgID: org.ID,
		creds: creds, sets: sets, tun: tun, llmRefreshCalls: &llmRefreshCalls,
		health: health,
	}
	t.Cleanup(ts.Close)
	h.login(h.client, adminEmail, adminPass)
	return h
}

func (h *settingsHarness) login(client *http.Client, email, password string) {
	h.t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := client.Post(h.srv.URL+"/xchats/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		h.t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.t.Fatalf("login status=%d", resp.StatusCode)
	}
}

// createMemberClient creates a "member"-role (non-admin) user in h's org
// and returns a client logged in as them — how RBAC-boundary tests exercise
// the "blocked" side of RequireAdmin.
func (h *settingsHarness) createMemberClient(email, password string) *http.Client {
	h.t.Helper()
	hash, err := httpapi.HashPassword(password)
	if err != nil {
		h.t.Fatalf("hash: %v", err)
	}
	if _, err := h.store.CreateUser(context.Background(), h.orgID, email, hash, "Member", "member"); err != nil {
		h.t.Fatalf("create member: %v", err)
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	h.login(client, email, password)
	return client
}

func (h *settingsHarness) llmRefreshCallCount() int32 {
	return atomic.LoadInt32(h.llmRefreshCalls)
}

func (h *settingsHarness) get(path string) (*http.Response, map[string]json.RawMessage) {
	h.t.Helper()
	resp, err := h.client.Get(h.srv.URL + path)
	if err != nil {
		h.t.Fatalf("GET %s: %v", path, err)
	}
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	return resp, env
}

func (h *settingsHarness) do(method, path string, body any) (*http.Response, map[string]json.RawMessage) {
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

func mustDecode(t *testing.T, env map[string]json.RawMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(env["payload"], out); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
}
