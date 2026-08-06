package httpapi_test

// cors_test.go guards a real bug a user hit: approving a Playground draft
// from a browser where the frontend and backend are on different origins
// failed with a CORS error, because If-Match (the draft store's
// optimistic-concurrency header — see stores/playground.ts's ifMatch())
// was never added to Access-Control-Allow-Headers when it was introduced.
// Go's httptest never exercises real browser CORS preflight enforcement, so
// nothing else in this suite would have caught it.

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
)

func TestCORS_PreflightAllowsIfMatchHeader(t *testing.T) {
	h := newHarness(t)

	req, err := http.NewRequest(http.MethodOptions, h.srv.URL+"/xchats/api/v1/playground/draft/approve", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", "*")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "If-Match")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer resp.Body.Close()

	allowed := resp.Header.Get("Access-Control-Allow-Headers")
	found := false
	for _, h := range strings.Split(allowed, ",") {
		if strings.EqualFold(strings.TrimSpace(h), "If-Match") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Access-Control-Allow-Headers=%q must include If-Match, or a cross-origin approve is blocked by the browser before it's ever sent", allowed)
	}
}

// TestCORS_UploadPreflightAllowsAnyWidgetOrigin guards a second real bug: a
// file upload from the ChatGPT widget never reached the server at all. The
// widget iframe runs on a per-app sandbox host
// (https://asdk_app_<hash>.web-sandbox.oaiusercontent.com) that can never be
// in CORSOrigins, and the global cors() middleware answered its preflight
// 204 WITHOUT Access-Control-Allow-Origin — aborting the chain before the
// permissive group-level uploadCORS could run. A 204 with no Allow-Origin
// reads as "denied", so the browser dropped the PUT silently and the logs
// showed only OPTIONS.
func TestCORS_UploadPreflightAllowsAnyWidgetOrigin(t *testing.T) {
	// A RESTRICTED allowlist, mirroring a real deployment — not newHarness's
	// CORSOrigins:["*"], under which the global middleware would happily echo
	// any origin and the bug would not reproduce. cors() snapshots the
	// allowlist when Router() builds the middleware, so this has to be set at
	// construction time rather than mutated afterwards. No database needed:
	// an OPTIONS preflight is answered by middleware and never reaches a
	// handler that touches the store.
	srv := httpapi.New(httpapi.Deps{
		Cfg: &config.Config{Server: config.ServerConfig{CORSOrigins: []string{"http://localhost:8081"}}},
		// requestLog() logs unconditionally on every request, so a nil Log
		// panics (gin.Recovery would swallow it, leaving a confusing trace in
		// otherwise-passing output). Discard it — this test asserts on headers.
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const widgetOrigin = "https://asdk_app_6a6dc61062a081918ebdc3b4f2aeca5f.web-sandbox.oaiusercontent.com"

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/mcp/uploads/09bc4a59-10d6-465c-bfd7-d9da4b01f2fe", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", widgetOrigin)
	req.Header.Set("Access-Control-Request-Method", "PUT")
	req.Header.Set("Access-Control-Request-Headers", "content-type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer resp.Body.Close()

	// The critical one: without this the browser blocks the PUT outright.
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != widgetOrigin {
		t.Fatalf("Access-Control-Allow-Origin=%q, want the widget origin %q echoed back", got, widgetOrigin)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(got, "PUT") {
		t.Fatalf("Access-Control-Allow-Methods=%q must include PUT", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(strings.ToLower(got), "content-type") {
		t.Fatalf("Access-Control-Allow-Headers=%q must include Content-Type", got)
	}
	// Never credentials: this endpoint echoes an ARBITRARY origin, and its
	// auth is the unguessable signed token in the URL, not a cookie.
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials=%q must not be set when echoing arbitrary origins", got)
	}
}

// TestCORS_MediaPreflightAllowsAnyWidgetOrigin is the read-direction twin of
// the upload test above, and guards the same trap: /mcp/media needs its own
// entry in cors()'s early-bail, or the global OPTIONS branch aborts the chain
// before the permissive group middleware runs and every preview image is
// blocked with a 204-and-no-Allow-Origin.
func TestCORS_MediaPreflightAllowsAnyWidgetOrigin(t *testing.T) {
	srv := httpapi.New(httpapi.Deps{
		Cfg: &config.Config{Server: config.ServerConfig{CORSOrigins: []string{"http://localhost:8081"}}},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const widgetOrigin = "https://asdk_app_6a6dc61062a081918ebdc3b4f2aeca5f.web-sandbox.oaiusercontent.com"

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/mcp/media/09bc4a59-10d6-465c-bfd7-d9da4b01f2fe", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", widgetOrigin)
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != widgetOrigin {
		t.Fatalf("Access-Control-Allow-Origin=%q, want the widget origin %q echoed back", got, widgetOrigin)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(got, "GET") {
		t.Fatalf("Access-Control-Allow-Methods=%q must include GET", got)
	}
	// Same rule as uploads: an arbitrary echoed origin must never be paired
	// with credentials.
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials=%q must not be set when echoing arbitrary origins", got)
	}
}

// TestCORS_WildcardNeverPairsWithCredentials is A7's regression guard on
// the GLOBAL cors() middleware (not the signed-token /mcp/uploads or
// /mcp/media groups above, which are correct by a different mechanism —
// they never set Allow-Credentials at all). A CORSOrigins:["*"] config
// (a supported dev/test convenience) must still never combine an echoed,
// attacker-controlled Origin with Access-Control-Allow-Credentials: true —
// that combination is a full session-riding hole from any website. An
// EXACT allowlist entry, by contrast, is what the operator explicitly
// trusts and must keep getting credentials, or every existing cross-origin
// deployment (frontend and backend on different origins, cookie-based
// auth) breaks.
func TestCORS_WildcardNeverPairsWithCredentials(t *testing.T) {
	srv := httpapi.New(httpapi.Deps{
		Cfg: &config.Config{Server: config.ServerConfig{CORSOrigins: []string{"*", "https://trusted.example"}}},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	get := func(origin string) *http.Response {
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/healthz", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Origin", origin)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		return resp
	}

	arbitrary := get("https://attacker.example")
	defer arbitrary.Body.Close()
	if got := arbitrary.Header.Get("Access-Control-Allow-Origin"); got != "https://attacker.example" {
		t.Errorf("wildcard-matched Allow-Origin=%q, want the origin echoed back (still usable non-credentialed)", got)
	}
	if got := arbitrary.Header.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("wildcard-matched Allow-Credentials=%q, want unset — this is the actual vulnerability", got)
	}

	trusted := get("https://trusted.example")
	defer trusted.Body.Close()
	if got := trusted.Header.Get("Access-Control-Allow-Origin"); got != "https://trusted.example" {
		t.Errorf("exact-allowlist Allow-Origin=%q, want the origin echoed back", got)
	}
	if got := trusted.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("exact-allowlist Allow-Credentials=%q, want \"true\" — an explicitly trusted origin must keep working", got)
	}
}
