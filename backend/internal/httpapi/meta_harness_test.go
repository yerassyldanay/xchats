package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	zalandokeyring "github.com/zalando/go-keyring"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/inboxmedia"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/metaingest"
	"github.com/yerassyldanay/xchats/backend/internal/password"
	"github.com/yerassyldanay/xchats/backend/internal/queue"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/settings"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/whatsappcloud"
	"github.com/yerassyldanay/xchats/backend/internal/worker"
	"github.com/yerassyldanay/xchats/backend/messaging"
)

const (
	metaTestSessionSecret = "meta-test-session-secret-do-not-use-in-prod"
	metaTestEncKey        = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 32 bytes, hex
	metaAdminEmail        = "meta-admin@xchats.test"
	metaAdminPass         = "password123"
)

// metaHarness is a lighter-weight sibling of the shared harness
// (integration_test.go): it wires only what the Meta-channel routes need —
// no response engine, no KB, no Telegram — plus a mock Graph API server
// whose handler each test installs for itself (graphHandler), since the
// three connect-flow tests each need a different scripted Graph response.
type metaHarness struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
	cfg    *config.Config
	store  *store.Store
	queue  *queue.InMem
	blob   blob.Store
	creds  *credentials.Chain
	// sets is wired into httpapi.Deps.Settings so handleSaveMetaApp's
	// baseURLValueFor can find a real settings.Store — needed only by tests
	// that point validateMeta's debug_token check at a mock server via
	// meta.base_url (see channel_setup_test.go's setMetaBaseURL helper).
	sets  *settings.Store
	orgID uuid.UUID

	// graphHandler answers every call the mock Graph API server receives.
	// nil (the default) answers 500 — a test that reaches the mock without
	// installing one first has a bug in its own setup, not a legitimate
	// "no handler needed" case.
	graphHandler http.HandlerFunc
}

func newMetaHarness(t *testing.T) *metaHarness {
	t.Helper()
	// See settings_test.go's identical call: forces credentials.Open onto
	// the file-backed store so this harness never touches a developer's
	// real OS keychain.
	zalandokeyring.MockInitWithError(errors.New("OS keyring disabled in tests"))

	ctx := context.Background()
	st, _ := dbtest.Open(t)

	h := &metaHarness{t: t, store: st}
	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.graphHandler == nil {
			http.Error(w, `{"error":{"message":"no graphHandler installed for this test","code":1}}`, http.StatusInternalServerError)
			return
		}
		h.graphHandler(w, r)
	}))
	t.Cleanup(graphSrv.Close)

	cfg := &config.Config{
		System:        config.SystemConfig{SessionTTLHours: 1, MinPasswordLen: 8},
		PageSize:      50,
		Server:        config.ServerConfig{CORSOrigins: []string{"*"}, APIBaseURL: "https://xchats.test"},
		Meta:          config.MetaModeConfig{WebhookPublicBaseURL: "https://xchats.test"},
		SessionSecret: metaTestSessionSecret,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	org, err := st.SeedOrganization(ctx, "meta-test-org")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	hash, _ := password.Hash(metaAdminPass)
	if _, err := st.SeedUser(ctx, org.ID, metaAdminEmail, hash, "Admin"); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	box, err := secretbox.FromEnvValue(metaTestEncKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	st.UseCredentialsBox(box)

	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	q := queue.NewInMem(64, 2, log)
	hub := realtime.NewHub()

	// All three Graph/OAuth hosts point at the SAME mock server — a test's
	// graphHandler distinguishes by URL path (e.g. "/oauth/access_token" vs
	// "/access_token" vs "/v21.0/..."), simpler than standing up three.
	metaClient := meta.NewHTTPWithHosts("v21.0", graphSrv.URL, graphSrv.URL, graphSrv.URL, log)
	waCloudClient := whatsappcloud.NewClient(metaClient)
	inboxSigner := inboxmedia.NewSigner(cfg.SessionSecret)
	metaProc := metaingest.New(metaingest.Deps{Store: st, Queue: q, Hub: hub, Log: log})

	senders := messaging.NewSenderRegistry()
	senders.Register(messaging.ChannelWhatsAppCloud,
		whatsappcloud.NewChannelSender(waCloudClient, st, st, inboxSigner, cfg.ResolvedAPIBaseURL()+"/meta/api/v1/media"))
	w := &worker.Worker{Store: st, Queue: q, WACloud: waCloudClient, Blob: blobStore, Hub: hub, Senders: senders, Log: log}
	q.Start(ctx, w.Handle)

	dataDir := t.TempDir()
	creds, err := credentials.Open(credentials.OpenOptions{AllowFile: true, DataDir: dataDir})
	if err != nil {
		t.Fatalf("credentials.Open: %v", err)
	}
	sets := settings.NewStore(t.TempDir())

	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Queue: q, Hub: hub, Blob: blobStore,
		OrgID: org.ID, Log: log,
		MetaClient: metaClient, MetaProcessor: metaProc, WACloudClient: waCloudClient, InboxMediaSigner: inboxSigner,
		Credentials: creds, Settings: sets,
	})
	ts := httptest.NewServer(srv.Router())
	jar, _ := cookiejar.New(nil)
	h.srv, h.client, h.cfg, h.creds, h.sets, h.queue, h.blob, h.orgID = ts, &http.Client{Jar: jar}, cfg, creds, sets, q, blobStore, org.ID
	t.Cleanup(func() { ts.Close(); q.Close() })
	h.login()
	return h
}

// setMetaBaseURL points validateMeta's debug_token check at srv instead of
// the real graph.facebook.com — a test-only hook (see credentials/
// provider.go's own doc comment on the "meta" provider's HasBaseURL: false)
// with no UI equivalent, since an operator never self-hosts the Graph API.
func (h *metaHarness) setMetaBaseURL(srvURL string) {
	h.t.Helper()
	if _, err := h.sets.Update(func(s *settings.Settings) {
		ps := s.Providers["meta"]
		ps.BaseURL = srvURL
		s.Providers["meta"] = ps
	}); err != nil {
		h.t.Fatalf("setMetaBaseURL: %v", err)
	}
}

// loginAs logs a fresh cookie-jar client in as an existing user — metaHarness's
// analogue of rbac_test.go's harness.loginAs, for tests that need to act as
// someone other than h.client's default admin.
func (h *metaHarness) loginAs(t *testing.T, email, password string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := client.Post(h.srv.URL+"/xchats/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login as %s: %v", email, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login as %s: status=%d", email, resp.StatusCode)
	}
	return client
}

// createMember creates a "member"-role user in h's organization — metaHarness's
// analogue of rbac_test.go's harness.createMember.
func (h *metaHarness) createMember(t *testing.T, email, plaintext, name string) {
	t.Helper()
	hash, err := password.Hash(plaintext)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if _, err := h.store.CreateUser(context.Background(), h.orgID, email, hash, name, "member"); err != nil {
		t.Fatalf("create member: %v", err)
	}
}

// setAppCredentials saves the operator's Meta App ID/Secret exactly the way
// the Settings UI would (a plain credentials.Chain.Set), so
// metaAppCredentials/metaCreds resolve them on the next call.
func (h *metaHarness) setAppCredentials(appID, appSecret string) {
	h.t.Helper()
	ctx := context.Background()
	if err := h.creds.Set(ctx, "meta.app_id", appID); err != nil {
		h.t.Fatalf("set meta.app_id: %v", err)
	}
	if err := h.creds.Set(ctx, "meta.app_secret", appSecret); err != nil {
		h.t.Fatalf("set meta.app_secret: %v", err)
	}
}

// setInstagramAppCredentials saves the operator's Instagram App ID/Secret —
// setAppCredentials' twin for the SEPARATE pair Business Login for Instagram
// needs (see credentials.KeyInstagramAppID's own doc comment). Deliberately
// a distinct helper, never folded into setAppCredentials: a test must be
// able to set only one of the two pairs to prove neither silently falls
// back to the other (see TestStartInstagramOAuthRequiresOwnAppCredentials).
func (h *metaHarness) setInstagramAppCredentials(appID, appSecret string) {
	h.t.Helper()
	ctx := context.Background()
	if err := h.creds.Set(ctx, credentials.KeyInstagramAppID, appID); err != nil {
		h.t.Fatalf("set instagram.app_id: %v", err)
	}
	if err := h.creds.Set(ctx, credentials.KeyInstagramAppSecret, appSecret); err != nil {
		h.t.Fatalf("set instagram.app_secret: %v", err)
	}
}

func (h *metaHarness) login() {
	h.t.Helper()
	resp, _ := h.postJSON("/xchats/api/v1/auth/login", map[string]string{"email": metaAdminEmail, "password": metaAdminPass})
	if resp.StatusCode != http.StatusOK {
		h.t.Fatalf("login: status %d", resp.StatusCode)
	}
}

func (h *metaHarness) postJSON(path string, body any) (*http.Response, map[string]json.RawMessage) {
	h.t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		h.t.Fatalf("marshal: %v", err)
	}
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

func (h *metaHarness) putJSON(path string, body any) (*http.Response, map[string]json.RawMessage) {
	h.t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		h.t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, h.srv.URL+path, bytes.NewReader(b))
	if err != nil {
		h.t.Fatalf("build PUT %s: %v", path, err)
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

func (h *metaHarness) getJSON(path string) (*http.Response, map[string]json.RawMessage) {
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

func (h *metaHarness) deleteJSON(path string) (*http.Response, map[string]json.RawMessage) {
	h.t.Helper()
	req, err := http.NewRequest(http.MethodDelete, h.srv.URL+path, nil)
	if err != nil {
		h.t.Fatalf("build DELETE %s: %v", path, err)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("DELETE %s: %v", path, err)
	}
	var env map[string]json.RawMessage
	_ = json.NewDecoder(resp.Body).Decode(&env)
	resp.Body.Close()
	h.queue.Wait()
	return resp, env
}

// postRaw is postJSON without the JSON envelope decode — for the public
// webhook routes, which answer a plain status code / plain-text body, not
// {payload,errcode,message}.
func (h *metaHarness) postRaw(path, contentType string, body []byte, headers map[string]string) *http.Response {
	h.t.Helper()
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+path, bytes.NewReader(body))
	if err != nil {
		h.t.Fatalf("build POST %s: %v", path, err)
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("POST %s: %v", path, err)
	}
	h.queue.Wait()
	return resp
}
