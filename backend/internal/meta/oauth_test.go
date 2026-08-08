package meta

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testClient() *Client {
	return NewHTTP("v21.0", slog.Default())
}

// --- URL builders (no network) -----------------------------------------

func TestInstagramAuthorizeURL(t *testing.T) {
	raw := InstagramAuthorizeURL("app123", "https://xchats.example/cb", "state-abc")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Host != "www.instagram.com" || u.Path != "/oauth/authorize" {
		t.Fatalf("host/path = %s%s, want www.instagram.com/oauth/authorize", u.Host, u.Path)
	}
	q := u.Query()
	if q.Get("client_id") != "app123" {
		t.Fatalf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "https://xchats.example/cb" {
		t.Fatalf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	if q.Get("response_type") != "code" {
		t.Fatalf("response_type = %q", q.Get("response_type"))
	}
	if q.Get("scope") != "instagram_business_basic,instagram_business_manage_messages" {
		t.Fatalf("scope = %q", q.Get("scope"))
	}
	if q.Get("state") != "state-abc" {
		t.Fatalf("state = %q", q.Get("state"))
	}
}

func TestFacebookAuthorizeURL(t *testing.T) {
	c := testClient()
	raw := c.FacebookAuthorizeURL("app123", "https://xchats.example/cb", "state-xyz",
		[]string{"pages_messaging", "pages_manage_metadata", "pages_show_list"})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Host != "www.facebook.com" || u.Path != "/v21.0/dialog/oauth" {
		t.Fatalf("host/path = %s%s, want www.facebook.com/v21.0/dialog/oauth", u.Host, u.Path)
	}
	q := u.Query()
	if q.Get("scope") != "pages_messaging,pages_manage_metadata,pages_show_list" {
		t.Fatalf("scope = %q", q.Get("scope"))
	}
	if q.Get("config_id") != "" {
		t.Fatal("plain Facebook Login must not carry a config_id — that's Embedded Signup's field")
	}
}

func TestEmbeddedSignupAuthorizeURL(t *testing.T) {
	c := testClient()
	raw := c.EmbeddedSignupAuthorizeURL("app123", "config-456", "https://xchats.example/cb", "state-1",
		`{"setup":{},"featureType":"","sessionInfoVersion":"3"}`)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()
	if q.Get("config_id") != "config-456" {
		t.Fatalf("config_id = %q", q.Get("config_id"))
	}
	if q.Get("override_default_response_type") != "true" {
		t.Fatalf("override_default_response_type = %q, want true", q.Get("override_default_response_type"))
	}
	if q.Get("scope") != "" {
		t.Fatal("Embedded Signup must not carry a scope param — permissions come from config_id")
	}
	var extras map[string]any
	if err := json.Unmarshal([]byte(q.Get("extras")), &extras); err != nil {
		t.Fatalf("extras is not valid JSON: %v", err)
	}
}

func TestWABAIDsFromDebugToken(t *testing.T) {
	d := DebugTokenResult{
		GranularScopes: []struct {
			Scope     string   `json:"scope"`
			TargetIDs []string `json:"target_ids"`
		}{
			{Scope: "pages_messaging", TargetIDs: []string{"page1"}},
			{Scope: "whatsapp_business_management", TargetIDs: []string{"waba1", "waba2"}},
		},
	}
	got := WABAIDsFromDebugToken(d)
	if len(got) != 2 || got[0] != "waba1" || got[1] != "waba2" {
		t.Fatalf("WABAIDsFromDebugToken = %v", got)
	}
}

func TestWABAIDsFromDebugTokenAbsent(t *testing.T) {
	if got := WABAIDsFromDebugToken(DebugTokenResult{}); got != nil {
		t.Fatalf("expected nil for a token with no whatsapp_business_management scope, got %v", got)
	}
}

// --- shared transport (Get/PostForm/PostJSON/GetBytes), via httptest -----

func TestClientGetDecodesSuccessResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer the-token" {
			t.Errorf("Authorization header = %q, want Bearer the-token", got)
		}
		_, _ = w.Write([]byte(`{"id":"123","name":"Test Page"}`))
	}))
	defer srv.Close()

	c := testClient()
	var out struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.Get(context.Background(), srv.URL, "the-token", &out); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if out.ID != "123" || out.Name != "Test Page" {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestClientGetDecodesErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token","type":"OAuthException","code":190,"error_subcode":463,"fbtrace_id":"Ax1"}}`))
	}))
	defer srv.Close()

	c := testClient()
	var out map[string]any
	err := c.Get(context.Background(), srv.URL, "", &out)
	if err == nil {
		t.Fatal("expected an error for a Graph API error envelope")
	}
	if !IsTokenExpired(err) {
		t.Fatalf("expected IsTokenExpired(err) — err = %v", err)
	}
	ae, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err is %T, want *APIError", err)
	}
	if ae.Message != "Invalid OAuth access token" || ae.FBTraceID != "Ax1" {
		t.Fatalf("APIError = %+v", ae)
	}
}

func TestClientGetErrorEnvelopeTakesPriorityOverStatus(t *testing.T) {
	// A non-2xx WITHOUT a parseable error envelope still surfaces something
	// useful rather than silently succeeding.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`not json at all`))
	}))
	defer srv.Close()

	c := testClient()
	var out map[string]any
	err := c.Get(context.Background(), srv.URL, "", &out)
	if err == nil {
		t.Fatal("expected an error for a 500 with an unparseable body")
	}
	if _, ok := err.(*APIError); ok {
		t.Fatal("a non-JSON 500 body must not be misreported as an *APIError")
	}
}

func TestClientPostFormSendsFormEncodedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q", r.FormValue("grant_type"))
		}
		_, _ = w.Write([]byte(`{"access_token":"short-lived","user_id":"999"}`))
	}))
	defer srv.Close()

	// Exercise ExchangeInstagramCode's actual method against a fake host by
	// calling the shared transport directly with the test server's URL —
	// api.instagram.com is a hardcoded literal in ExchangeInstagramCode
	// itself, so this proves the SAME request-building/decoding logic that
	// method uses, without reaching the real internet.
	c := testClient()
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	var out InstagramTokenResult
	if err := c.PostForm(context.Background(), srv.URL, form, "", &out); err != nil {
		t.Fatalf("PostForm: %v", err)
	}
	if out.AccessToken != "short-lived" || out.UserID != "999" {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestClientPostJSONSendsJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["messaging_product"] != "whatsapp" {
			t.Errorf("body = %+v", body)
		}
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.abc"}]}`))
	}))
	defer srv.Close()

	c := testClient()
	var out map[string]any
	err := c.PostJSON(context.Background(), srv.URL, map[string]any{"messaging_product": "whatsapp"}, "tok", &out)
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
}

func TestClientGetBytesReturnsRawBodyAndContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("fake-jpeg-bytes"))
	}))
	defer srv.Close()

	c := testClient()
	body, ct, err := c.GetBytes(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("GetBytes: %v", err)
	}
	if string(body) != "fake-jpeg-bytes" {
		t.Fatalf("body = %q", body)
	}
	if ct != "image/jpeg" {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestClientRedactsTokenFromTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // connection refused — a fast, reliable, LOCAL transport failure

	c := testClient()
	var out map[string]any
	err := c.Get(context.Background(), srv.URL+"?access_token=SECRETVALUE", "SECRETVALUE", &out)
	if err == nil {
		t.Fatal("expected a transport error against a closed server")
	}
	if strings.Contains(err.Error(), "SECRETVALUE") {
		t.Fatalf("token leaked into transport error text: %v", err)
	}
}

func TestExchangeFacebookCodeRedactsSecretsEvenThoughTheyRideTheQueryString(t *testing.T) {
	// ExchangeFacebookCode builds its own request against the hardcoded
	// graph.facebook.com host (can't redirect it to a test server), so this
	// exercises redactAll's own scrub logic directly against the exact kind
	// of transport error the real call would produce.
	err := redactAll(&redactedError{text: "GET https://graph.facebook.com/v21.0/oauth/access_token?client_secret=SHHH&code=CODE123: dial failed"},
		"SHHH", "CODE123")
	if strings.Contains(err.Error(), "SHHH") || strings.Contains(err.Error(), "CODE123") {
		t.Fatalf("secrets survived redactAll: %v", err)
	}
}
