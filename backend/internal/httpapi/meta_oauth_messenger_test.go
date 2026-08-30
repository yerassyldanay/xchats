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

func TestStartMessengerOAuthRequiresAppCredentials(t *testing.T) {
	h := newMetaHarness(t)
	resp, env := h.postJSON("/xchats/api/v1/messenger-accounts/oauth/start", nil)
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	if string(env["errcode"]) != `"META_APP_NOT_CONFIGURED"` {
		t.Fatalf("errcode = %s", env["errcode"])
	}
}

func TestStartMessengerOAuthReturnsAuthorizeURLAndPersistsState(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-123", "app-secret-123")
	resp, env := h.postJSON("/xchats/api/v1/messenger-accounts/oauth/start", nil)
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
	if u.Host != "www.facebook.com" {
		t.Fatalf("authorize_url host = %q", u.Host)
	}
	if u.Query().Get("client_id") != "app-123" {
		t.Fatalf("client_id = %q", u.Query().Get("client_id"))
	}
	scope := u.Query().Get("scope")
	for _, want := range []string{"pages_show_list", "pages_messaging", "pages_manage_metadata"} {
		if !strings.Contains(scope, want) {
			t.Fatalf("scope = %q, missing %q", scope, want)
		}
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("no state in authorize_url")
	}
	if _, err := h.store.MetaOAuthStateByID(context.Background(), state); err != nil {
		t.Fatalf("MetaOAuthStateByID: %v — the state minted in the URL must be persisted", err)
	}
}

// messengerOAuthMockHandler answers the one-mock-server-in-one-Graph-host
// stand-in for facebook.com's connect flow: the plain code exchange and the
// fb_exchange_token long-lived exchange both land on the SAME
// oauth/access_token path (see ExchangeFacebookCode/ExchangeFacebookLongLived's
// own doc comments) and are told apart by the grant_type query parameter;
// PageAccounts and Subscribe are their own paths. pages lets a test script
// zero, one, or several Pages onto the mocked /me/accounts response, to
// exercise finishMessengerConnect's "exactly one Page" rule.
func messengerOAuthMockHandler(t *testing.T, pages []map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth/access_token") && r.URL.Query().Get("grant_type") == "fb_exchange_token":
			_, _ = w.Write([]byte(`{"access_token":"long-lived-user-token","token_type":"bearer","expires_in":5184000}`))
		case strings.HasSuffix(r.URL.Path, "/oauth/access_token"):
			_, _ = w.Write([]byte(`{"access_token":"short-lived-user-token","token_type":"bearer","expires_in":3600}`))
		case strings.HasSuffix(r.URL.Path, "/me/accounts"):
			data, _ := json.Marshal(pages)
			_, _ = w.Write([]byte(`{"data":` + string(data) + `}`))
		case strings.HasSuffix(r.URL.Path, "/subscribed_apps"):
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			t.Fatalf("unexpected mock call: %s %s", r.Method, r.URL.Path)
		}
	}
}

func onePage(id, name, token string) []map[string]string {
	return []map[string]string{{"id": id, "name": name, "access_token": token}}
}

func TestMessengerOAuthCallbackFullFlow(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-123", "app-secret-123")
	h.graphHandler = messengerOAuthMockHandler(t, onePage("998877", "My Shop Page", "page-token-abc"))

	_, startEnv := h.postJSON("/xchats/api/v1/messenger-accounts/oauth/start", nil)
	var started struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	if err := json.Unmarshal(startEnv["payload"], &started); err != nil {
		t.Fatalf("decode start payload: %v", err)
	}
	state := mustParseState(t, started.AuthorizeURL)

	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/messenger/callback?code=abc123&state=" + state)
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "messenger_connected") {
		t.Fatalf("redirect Location = %q, want it to carry messenger_connected", loc)
	}

	acctID := config.ChannelAccountID(config.MessengerOwnerRef("998877"))
	acct, err := h.store.ChannelAccountByID(context.Background(), acctID)
	if err != nil {
		t.Fatalf("ChannelAccountByID: %v", err)
	}
	if acct.Channel != "messenger" || acct.Handle != "My Shop Page" || acct.DisplayName != "My Shop Page" {
		t.Fatalf("account = %+v", acct)
	}
	secret, err := h.store.ChannelCredentialsSecret(context.Background(), acctID)
	if err != nil || secret != "page-token-abc" {
		t.Fatalf("ChannelCredentialsSecret = (%q, %v)", secret, err)
	}

	st, err := h.store.MetaOAuthStateByID(context.Background(), state)
	if err == nil || err != store.ErrNotFound {
		t.Fatalf("state must no longer be pending after settling, got (%+v, %v)", st, err)
	}
}

func TestMessengerOAuthCallbackZeroPagesFails(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-123", "app-secret-123")
	h.graphHandler = messengerOAuthMockHandler(t, nil)

	_, startEnv := h.postJSON("/xchats/api/v1/messenger-accounts/oauth/start", nil)
	var started struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	_ = json.Unmarshal(startEnv["payload"], &started)
	state := mustParseState(t, started.AuthorizeURL)

	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/messenger/callback?code=abc&state=" + state)
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "messenger_error") {
		t.Fatalf("Location = %q, want messenger_error (no Pages granted)", loc)
	}
	// docs/ux/flows/03b-connect-instagram-messenger.md, friction point 7: a
	// stable code travels alongside the message so the frontend can show a
	// localized string regardless of the active locale.
	if !strings.Contains(loc, "messenger_error_code=NO_PAGES") {
		t.Fatalf("Location = %q, want messenger_error_code=NO_PAGES", loc)
	}
}

func TestMessengerOAuthCallbackMultiplePagesFails(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-123", "app-secret-123")
	h.graphHandler = messengerOAuthMockHandler(t, []map[string]string{
		{"id": "1", "name": "Page One", "access_token": "tok1"},
		{"id": "2", "name": "Page Two", "access_token": "tok2"},
	})

	_, startEnv := h.postJSON("/xchats/api/v1/messenger-accounts/oauth/start", nil)
	var started struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	_ = json.Unmarshal(startEnv["payload"], &started)
	state := mustParseState(t, started.AuthorizeURL)

	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/messenger/callback?code=abc&state=" + state)
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "messenger_error") {
		t.Fatalf("Location = %q, want messenger_error (more than one Page granted)", loc)
	}
	if !strings.Contains(loc, "messenger_error_code=MULTIPLE_PAGES") {
		t.Fatalf("Location = %q, want messenger_error_code=MULTIPLE_PAGES", loc)
	}

	// Nothing must be persisted for either candidate Page — a rejected
	// multi-page connect attempt is a full no-op, not a partial one.
	if _, err := h.store.ChannelAccountByID(context.Background(), config.ChannelAccountID(config.MessengerOwnerRef("1"))); err != store.ErrNotFound {
		t.Fatalf("Page 1 must not have been claimed, got err=%v", err)
	}
	if _, err := h.store.ChannelAccountByID(context.Background(), config.ChannelAccountID(config.MessengerOwnerRef("2"))); err != store.ErrNotFound {
		t.Fatalf("Page 2 must not have been claimed, got err=%v", err)
	}
}

func TestMessengerOAuthCallbackUnknownStateRedirectsWithError(t *testing.T) {
	h := newMetaHarness(t)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/messenger/callback?code=abc&state=does-not-exist")
	if err != nil {
		t.Fatalf("GET callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "messenger_error") {
		t.Fatalf("Location = %q, want messenger_error", loc)
	}
}

func TestMessengerOAuthCallbackStateIsSingleUse(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-123", "app-secret-123")
	h.graphHandler = messengerOAuthMockHandler(t, onePage("998878", "Shop Two", "tok2"))

	_, startEnv := h.postJSON("/xchats/api/v1/messenger-accounts/oauth/start", nil)
	var started struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	_ = json.Unmarshal(startEnv["payload"], &started)
	state := mustParseState(t, started.AuthorizeURL)

	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	first, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/messenger/callback?code=abc&state=" + state)
	if err != nil {
		t.Fatalf("first callback: %v", err)
	}
	first.Body.Close()
	if !strings.Contains(first.Header.Get("Location"), "messenger_connected") {
		t.Fatalf("first attempt Location = %q, want messenger_connected", first.Header.Get("Location"))
	}

	second, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/messenger/callback?code=abc&state=" + state)
	if err != nil {
		t.Fatalf("second (replayed) callback: %v", err)
	}
	defer second.Body.Close()
	if !strings.Contains(second.Header.Get("Location"), "messenger_error") {
		t.Fatalf("replayed attempt Location = %q, want messenger_error (single-use state)", second.Header.Get("Location"))
	}
}

func TestDeleteMessengerAccountUnsubscribesAndSoftDeletes(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-123", "app-secret-123")
	h.graphHandler = messengerOAuthMockHandler(t, onePage("998879", "Shop Three", "tok3"))

	_, startEnv := h.postJSON("/xchats/api/v1/messenger-accounts/oauth/start", nil)
	var started struct {
		AuthorizeURL string `json:"authorize_url"`
	}
	_ = json.Unmarshal(startEnv["payload"], &started)
	state := mustParseState(t, started.AuthorizeURL)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	connectResp, err := noRedirect.Get(h.srv.URL + "/meta/api/v1/oauth/messenger/callback?code=abc&state=" + state)
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	connectResp.Body.Close()

	acctID := config.ChannelAccountID(config.MessengerOwnerRef("998879"))

	var unsubscribed bool
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/subscribed_apps") {
			unsubscribed = true
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}
	resp, _ := h.deleteJSON("/xchats/api/v1/messenger-accounts/" + acctID.String())
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
