package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

func TestStartInstagramOAuthRequiresAppCredentials(t *testing.T) {
	h := newMetaHarness(t)
	resp, env := h.postJSON("/xchats/api/v1/instagram-accounts/oauth/start", nil)
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	if string(env["errcode"]) != `"INSTAGRAM_APP_NOT_CONFIGURED"` {
		t.Fatalf("errcode = %s", env["errcode"])
	}
}

// TestStartInstagramOAuthDoesNotFallBackToMetaAppCredentials is the
// regression test for the bug this feature fixes: Instagram Login
// (Business Login for Instagram) rejects the Meta Developer App id outright
// — see internal/meta/oauth.go's InstagramAuthorizeURL doc comment. With
// only the "meta" pair saved (exactly the state a fresh Messenger/WhatsApp
// Cloud install already has), starting Instagram must still 412 rather than
// silently reuse it.
func TestStartInstagramOAuthDoesNotFallBackToMetaAppCredentials(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("meta-app-id", "meta-app-secret")
	resp, env := h.postJSON("/xchats/api/v1/instagram-accounts/oauth/start", nil)
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	if string(env["errcode"]) != `"INSTAGRAM_APP_NOT_CONFIGURED"` {
		t.Fatalf("errcode = %s, want INSTAGRAM_APP_NOT_CONFIGURED even with a meta pair saved", env["errcode"])
	}
}

// TestStartInstagramOAuthReturnsAuthorizeURLAndPersistsState saves BOTH
// pairs with DIFFERENT ids and asserts the authorize_url's client_id is the
// INSTAGRAM one — proof the start handler reads the right credential, not
// merely that some credential was accepted.
func TestStartInstagramOAuthReturnsAuthorizeURLAndPersistsState(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("meta-app-id", "meta-app-secret")
	h.setInstagramAppCredentials("app-123", "app-secret-123")
	resp, env := h.postJSON("/xchats/api/v1/instagram-accounts/oauth/start", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	var payload struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	if err := json.Unmarshal(env["payload"], &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	u, err := url.Parse(payload.AuthorizeURL)
	if err != nil {
		t.Fatalf("authorize_url is not a URL: %v", err)
	}
	if u.Host != "www.instagram.com" {
		t.Fatalf("authorize_url host = %q", u.Host)
	}
	if got := u.Query().Get("client_id"); got != "app-123" {
		t.Fatalf("client_id = %q, want the Instagram app id (app-123), not the meta app id", got)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("no state in authorize_url")
	}
	if _, err := h.store.MetaOAuthStateByID(context.Background(), state); err != nil {
		t.Fatalf("MetaOAuthStateByID: %v — the state minted in the URL must be persisted", err)
	}
}

// instagramOAuthMockHandler answers the three-hosts-in-one mock server for a
// successful Instagram connect: code exchange (instagramAPIHost),
// long-lived exchange (instagramGraphHostBase), Me, and Subscribe (both
// under the versioned InstagramGraphURL). wantAppID/wantAppSecret assert
// EXACTLY which app pair reaches Meta on each leg — the Instagram pair,
// never a Meta Developer App pair a test may have separately configured.
func instagramOAuthMockHandler(t *testing.T, igUserID, username, name, wantAppID, wantAppSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/access_token":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			if got := r.FormValue("client_id"); got != wantAppID {
				t.Fatalf("/oauth/access_token client_id = %q, want %q (the Instagram app id)", got, wantAppID)
			}
			if got := r.FormValue("client_secret"); got != wantAppSecret {
				t.Fatalf("/oauth/access_token client_secret = %q, want %q (the Instagram app secret)", got, wantAppSecret)
			}
			_, _ = w.Write([]byte(`{"access_token":"short-lived-token","user_id":"` + igUserID + `"}`))
		case r.URL.Path == "/access_token":
			// ig_exchange_token carries no client_id — only the secret.
			if got := r.URL.Query().Get("client_secret"); got != wantAppSecret {
				t.Fatalf("/access_token client_secret = %q, want %q (the Instagram app secret)", got, wantAppSecret)
			}
			_, _ = w.Write([]byte(`{"access_token":"long-lived-token","token_type":"bearer","expires_in":5184000}`))
		case strings.HasSuffix(r.URL.Path, "/me"):
			_, _ = w.Write([]byte(`{"user_id":"` + igUserID + `","username":"` + username + `","name":"` + name + `"}`))
		case strings.HasSuffix(r.URL.Path, "/subscribed_apps"):
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			t.Fatalf("unexpected mock call: %s %s", r.Method, r.URL.Path)
		}
	}
}

func TestInstagramOAuthCallbackFullFlow(t *testing.T) {
	h := newMetaHarness(t)
	// A different Meta pair is ALSO saved (the ordinary state once Messenger
	// setup is done) — instagramOAuthMockHandler fails the test outright if
	// this one ever reaches api.instagram.com/graph.instagram.com instead of
	// the Instagram pair below.
	h.setAppCredentials("meta-app-id", "meta-app-secret")
	h.setInstagramAppCredentials("app-123", "app-secret-123")
	h.graphHandler = instagramOAuthMockHandler(t, "178414000001", "my_shop", "My Shop", "app-123", "app-secret-123")

	_, startEnv := h.postJSON("/xchats/api/v1/instagram-accounts/oauth/start", nil)
	var started struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	if err := json.Unmarshal(startEnv["payload"], &started); err != nil {
		t.Fatalf("decode start payload: %v", err)
	}
	state := mustParseState(t, started.AuthorizeURL)

	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/instagram/callback?code=abc123&state=" + state)
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "instagram_connected") {
		t.Fatalf("redirect Location = %q, want it to carry instagram_connected", loc)
	}

	acctID := config.ChannelAccountID(config.InstagramOwnerRef("178414000001"))
	acct, err := h.store.ChannelAccountByID(context.Background(), acctID)
	if err != nil {
		t.Fatalf("ChannelAccountByID: %v", err)
	}
	if acct.Channel != "instagram" || acct.Handle != "@my_shop" || acct.DisplayName != "My Shop" {
		t.Fatalf("account = %+v", acct)
	}
	secret, err := h.store.ChannelCredentialsSecret(context.Background(), acctID)
	if err != nil || secret != "long-lived-token" {
		t.Fatalf("ChannelCredentialsSecret = (%q, %v)", secret, err)
	}

	st, err := h.store.MetaOAuthStateByID(context.Background(), state)
	if err == nil || err != store.ErrNotFound {
		t.Fatalf("state must no longer be pending after settling, got (%+v, %v)", st, err)
	}
}

func TestInstagramOAuthCallbackUnknownStateRedirectsWithError(t *testing.T) {
	h := newMetaHarness(t)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/instagram/callback?code=abc&state=does-not-exist")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "instagram_error") {
		t.Fatalf("Location = %q, want instagram_error", loc)
	}
}

func TestInstagramOAuthCallbackStateIsSingleUse(t *testing.T) {
	h := newMetaHarness(t)
	h.setInstagramAppCredentials("app-123", "app-secret-123")
	h.graphHandler = instagramOAuthMockHandler(t, "178414000002", "shop2", "Shop Two", "app-123", "app-secret-123")

	_, startEnv := h.postJSON("/xchats/api/v1/instagram-accounts/oauth/start", nil)
	var started struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	_ = json.Unmarshal(startEnv["payload"], &started)
	state := mustParseState(t, started.AuthorizeURL)

	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	first, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/instagram/callback?code=abc&state=" + state)
	if err != nil {
		t.Fatalf("first callback: %v", err)
	}
	first.Body.Close()
	if !strings.Contains(first.Header.Get("Location"), "instagram_connected") {
		t.Fatalf("first attempt Location = %q, want instagram_connected", first.Header.Get("Location"))
	}

	second, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/instagram/callback?code=abc&state=" + state)
	if err != nil {
		t.Fatalf("second (replayed) callback: %v", err)
	}
	defer second.Body.Close()
	if !strings.Contains(second.Header.Get("Location"), "instagram_error") {
		t.Fatalf("replayed attempt Location = %q, want instagram_error (single-use state)", second.Header.Get("Location"))
	}
}

func TestDeleteInstagramAccountUnsubscribesAndSoftDeletes(t *testing.T) {
	h := newMetaHarness(t)
	h.setInstagramAppCredentials("app-123", "app-secret-123")
	h.graphHandler = instagramOAuthMockHandler(t, "178414000003", "shop3", "Shop Three", "app-123", "app-secret-123")

	_, startEnv := h.postJSON("/xchats/api/v1/instagram-accounts/oauth/start", nil)
	var started struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	_ = json.Unmarshal(startEnv["payload"], &started)
	state := mustParseState(t, started.AuthorizeURL)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	connectResp, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/instagram/callback?code=abc&state=" + state)
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	connectResp.Body.Close()

	acctID := config.ChannelAccountID(config.InstagramOwnerRef("178414000003"))

	var unsubscribed bool
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/subscribed_apps") {
			unsubscribed = true
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}
	resp, _ := h.deleteJSON("/xchats/api/v1/instagram-accounts/" + acctID.String())
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d", resp.StatusCode)
	}
	if !unsubscribed {
		t.Fatal("Unsubscribe was never called")
	}
	if _, err := h.store.ChannelAccountByID(context.Background(), acctID); err != store.ErrNotFound {
		t.Fatalf("ChannelAccountByID after delete = %v, want ErrNotFound", err)
	}
}

func mustParseState(t *testing.T, authorizeURL string) string {
	t.Helper()
	u, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("parse authorize_url: %v", err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("no state in authorize_url")
	}
	return state
}
