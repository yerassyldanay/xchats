package desktop

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// serveThroughMiddleware runs one request through the asset-server
// middleware, with next standing in for Wails' own asset handler.
func serveThroughMiddleware(t *testing.T, mw func(http.Handler) http.Handler, next http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)
	return rec
}

// sessionCookie is internal/httpapi.setSessionCookie's shape: the same name,
// path and attribute set the real login handler writes. The jar reads only
// Name, Value and MaxAge, so the security attributes are fixture detail —
// spelled out rather than omitted so the fixture models a real response
// cookie instead of a bare name/value pair.
func sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     "xchats_session",
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func TestIsBackendPath(t *testing.T) {
	backend := []string{
		"/xchats/api/v1/me",
		"/xchats/api/v1/media/abc",
		"/healthz",
		"/readyz",
		"/mcp/uploads/x",
		"/oauth/authorize",
		"/telegram/api/v1/webhook/1",
		"/.well-known/oauth-authorization-server",
	}
	for _, p := range backend {
		if !IsBackendPath(p) {
			t.Errorf("IsBackendPath(%q) = false, want true", p)
		}
	}
	spa := []string{
		"/",
		"/chatboard",
		"/knowledge-base",
		"/assets/index-abc123.js",
		"/logo.png",
		"/xchats-not-the-api", // prefix match must require the slash
		"/healthzz",
	}
	for _, p := range spa {
		if IsBackendPath(p) {
			t.Errorf("IsBackendPath(%q) = true, want false", p)
		}
	}
}

func TestMiddlewareRoutesAPIToTheBackendAndAssetsToWails(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "api")
	})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "asset")
	})
	mw := NewMiddleware(api)

	rec := serveThroughMiddleware(t, mw, next, httptest.NewRequest(http.MethodGet, "/xchats/api/v1/me", nil))
	if got := rec.Body.String(); got != "api" {
		t.Errorf("API path served by %q, want the backend router", got)
	}

	rec = serveThroughMiddleware(t, mw, next, httptest.NewRequest(http.MethodGet, "/chatboard", nil))
	if got := rec.Body.String(); got != "asset" {
		t.Errorf("SPA path served by %q, want the Wails asset handler", got)
	}
}

func TestMiddlewareRefusesTheSSEStream(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "would stream forever")
	})
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	rec := serveThroughMiddleware(t, NewMiddleware(api), next,
		httptest.NewRequest(http.MethodGet, realtimePath, nil))

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d — the desktop app must never open an SSE stream through the asset server", rec.Code, http.StatusNotImplemented)
	}
}

func TestMiddlewareCarriesTheSessionCookieForTheWebView(t *testing.T) {
	const cookieName = "xchats_session"
	var seen []string
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(cookieName); err == nil {
			seen = append(seen, c.Value)
		} else {
			seen = append(seen, "")
		}
		if r.URL.Path == "/xchats/api/v1/auth/login" {
			http.SetCookie(w, sessionCookie("sid-1", 3600))
		}
		w.WriteHeader(http.StatusOK)
	})
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	mw := NewMiddleware(api)

	// 1. An unauthenticated call carries nothing.
	serveThroughMiddleware(t, mw, next, httptest.NewRequest(http.MethodGet, "/xchats/api/v1/me", nil))
	// 2. Login sets the cookie. The WebView on macOS/Linux drops it, so the
	//    shell has to be the one that remembers.
	serveThroughMiddleware(t, mw, next, httptest.NewRequest(http.MethodPost, "/xchats/api/v1/auth/login", nil))
	// 3. The next call must arrive authenticated.
	serveThroughMiddleware(t, mw, next, httptest.NewRequest(http.MethodGet, "/xchats/api/v1/me", nil))

	want := []string{"", "", "sid-1"}
	if len(seen) != len(want) {
		t.Fatalf("saw %d requests, want %d", len(seen), len(want))
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("request %d carried cookie %q, want %q", i, seen[i], want[i])
		}
	}
}

func TestMiddlewareForgetsAClearedCookie(t *testing.T) {
	const cookieName = "xchats_session"
	var last string
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last = ""
		if c, err := r.Cookie(cookieName); err == nil {
			last = c.Value
		}
		switch r.URL.Path {
		case "/xchats/api/v1/auth/login":
			http.SetCookie(w, sessionCookie("sid-1", 3600))
		case "/xchats/api/v1/auth/logout":
			// setSessionCookie's own deletion shape.
			http.SetCookie(w, sessionCookie("", -1))
		}
		w.WriteHeader(http.StatusOK)
	})
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	mw := NewMiddleware(api)

	serveThroughMiddleware(t, mw, next, httptest.NewRequest(http.MethodPost, "/xchats/api/v1/auth/login", nil))
	serveThroughMiddleware(t, mw, next, httptest.NewRequest(http.MethodPost, "/xchats/api/v1/auth/logout", nil))
	serveThroughMiddleware(t, mw, next, httptest.NewRequest(http.MethodGet, "/xchats/api/v1/me", nil))

	if last != "" {
		t.Errorf("request after logout carried cookie %q, want none — a logout that does not stick is a security bug", last)
	}
}

func TestMiddlewareDoesNotDuplicateACookieTheWebViewAlreadySent(t *testing.T) {
	const cookieName = "xchats_session"
	var count int
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count = len(r.Cookies())
		if r.URL.Path == "/xchats/api/v1/auth/login" {
			http.SetCookie(w, sessionCookie("sid-1", 3600))
		}
		w.WriteHeader(http.StatusOK)
	})
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	mw := NewMiddleware(api)

	serveThroughMiddleware(t, mw, next, httptest.NewRequest(http.MethodPost, "/xchats/api/v1/auth/login", nil))

	// Windows' WebView2 loads from http://wails.localhost/ and does keep
	// cookies, so it sends its own — the jar must not add a second copy.
	req := httptest.NewRequest(http.MethodGet, "/xchats/api/v1/me", nil)
	req.Header.Set("Cookie", cookieName+"=sid-1")
	serveThroughMiddleware(t, mw, next, req)

	if count != 1 {
		t.Errorf("request carried %d cookies, want 1", count)
	}
}

func TestSPAHandlerServesTheAppShellForClientRoutes(t *testing.T) {
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html><div id=app>")}}
	h := NewSPAHandler(assets)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/knowledge-base", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — vue-router's history mode needs index.html back", rec.Code)
	}
	if got := rec.Body.String(); got != "<!doctype html><div id=app>" {
		t.Errorf("body = %q, want index.html", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestSPAHandlerRejectsNonGET(t *testing.T) {
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("x")}}
	rec := httptest.NewRecorder()
	NewSPAHandler(assets).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/whatever", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405 — a non-GET that reaches the SPA fallback is not an API call", rec.Code)
	}
}

func TestSPAHandlerReportsAMissingBundle(t *testing.T) {
	rec := httptest.NewRecorder()
	NewSPAHandler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 with a message naming the missing bundle", rec.Code)
	}
}
