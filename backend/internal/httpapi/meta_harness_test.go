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
	orgID  uuid.UUID

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

	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Queue: q, Hub: hub, Blob: blobStore,
		OrgID: org.ID, Log: log,
		MetaClient: metaClient, MetaProcessor: metaProc, WACloudClient: waCloudClient, InboxMediaSigner: inboxSigner,
		Credentials: creds,
	})
	ts := httptest.NewServer(srv.Router())
	jar, _ := cookiejar.New(nil)
	h.srv, h.client, h.cfg, h.creds, h.queue, h.blob, h.orgID = ts, &http.Client{Jar: jar}, cfg, creds, q, blobStore, org.ID
	t.Cleanup(func() { ts.Close(); q.Close() })
	h.login()
	return h
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
