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
		Cfg: &config.Config{CORSOrigins: []string{"http://localhost:8081"}},
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
