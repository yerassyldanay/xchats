package desktop

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
)

// backendPrefixes are the URL prefixes the desktop asset server hands to the
// backend's own router instead of the embedded SPA bundle. They are exactly
// the route groups internal/httpapi.Router mounts — /xchats/api/v1 plus the
// media, MCP, OAuth and webhook surfaces — so the WebView addresses the API
// at the same paths the browser build does and frontend/src/api/client.ts
// needs no desktop-specific base URL.
var backendPrefixes = []string{
	"/xchats/",
	"/mcp/",
	"/oauth/",
	"/telegram/",
	"/.well-known/",
}

// backendExactPaths are the unprefixed ops routes.
var backendExactPaths = map[string]bool{
	"/healthz": true,
	"/readyz":  true,
}

// realtimePath is the SSE stream, deliberately NOT served through the asset
// server — see serveHTTP's guard and realtime.go.
const realtimePath = "/xchats/api/v1/realtime"

// IsBackendPath reports whether p belongs to the backend router rather than
// to the SPA bundle.
func IsBackendPath(p string) bool {
	if backendExactPaths[p] {
		return true
	}
	for _, prefix := range backendPrefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// NewMiddleware returns the Wails asset-server middleware that splits every
// WebView request between the backend router and the embedded SPA bundle.
//
// This is what makes the desktop app same-origin: the WebView loads the page
// from Wails' asset server, and this middleware answers /xchats/api/v1/* from
// that same origin by calling the router in-process. No second HTTP hop, no
// CORS preflight, no cross-origin cookie question — the browser deployment's
// exact request shape, minus the network.
//
// api is the *gin.Engine internal/httpapi already built. It is called
// directly rather than proxied over the loopback listener runServe also
// starts: an http.Handler is an http.Handler, and skipping the round trip
// avoids re-serializing every request through a socket for no benefit. The
// loopback listener stays up regardless, so `curl localhost:8080/healthz`
// and a browser tab still work exactly as before.
func NewMiddleware(api http.Handler) func(next http.Handler) http.Handler {
	jar := &cookieJar{cookies: map[string]*http.Cookie{}}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsBackendPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if r.URL.Path == realtimePath {
				// Wails' Windows asset server buffers a response until the
				// handler returns (pkg/assetserver/webview's WebView2
				// responseWriter writes into a bytes.Buffer that only reaches
				// the WebView on Finish), so an SSE stream here would never
				// deliver an event AND would hold a WebView request slot open
				// for the life of the app. The desktop frontend uses Wails
				// events instead; this is the guard that keeps a stale bundle
				// from hanging the window rather than failing fast.
				http.Error(w, "realtime is delivered over Wails events in the desktop app", http.StatusNotImplemented)
				return
			}
			jar.inject(r)
			api.ServeHTTP(&jarWriter{ResponseWriter: w, jar: jar}, r)
		})
	}
}

// NewSPAHandler serves index.html for anything the asset bundle does not
// contain, which is what makes vue-router's createWebHistory work: a reload
// (or a WebView restore) on /chatboard asks the asset server for a file that
// was never built, and the SPA needs the app shell back so its own router can
// resolve the path. This is frontend/nginx.conf's `try_files ... /index.html`
// with the same job and the same one-line implementation.
//
// Wails calls this handler for two cases: a GET that missed the asset FS, and
// every non-GET request. A non-GET that reaches here is neither an API call
// (NewMiddleware already took those) nor a real asset, so it gets a 405
// rather than the app shell.
func NewSPAHandler(assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		f, err := assets.Open("index.html")
		if err != nil {
			http.Error(w, "frontend bundle is missing index.html", http.StatusInternalServerError)
			return
		}
		defer func() { _ = f.Close() }()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// No caching: index.html is the one file whose staleness strands the
		// WebView on a bundle whose hashed assets a later build deleted.
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		_, _ = io.Copy(w, f)
	})
}

// --- cookie jar -----------------------------------------------------------

// cookieJar stores the session cookie on behalf of the WebView.
//
// On macOS and Linux the window loads from a custom URL scheme
// (wails://wails/), and neither WKWebView nor WebKitGTK runs its cookie
// store for a custom scheme: Set-Cookie on a scheme-handler response is
// dropped, and no Cookie header is ever sent back. Session auth
// (internal/httpapi.requireSession reads the xchats_session cookie) would
// simply never work there.
//
// Rather than change how the backend authenticates — a real change to a
// security-relevant path, for a packaging concern — the shell does the one
// job the WebView is not doing: it keeps the cookies the router sets and
// replays them on the next request. That is precisely a browser's cookie
// jar, scoped to this process and this single origin, so every existing
// login, logout, session-expiry and org-switch behavior is preserved
// unchanged.
//
// On Windows the WebView loads from http://wails.localhost/ and does manage
// cookies, so a request can arrive already carrying one; inject leaves those
// alone rather than sending the header twice.
type cookieJar struct {
	mu      sync.Mutex
	cookies map[string]*http.Cookie
}

// inject adds every stored cookie the request is not already carrying.
func (j *cookieJar) inject(r *http.Request) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.cookies) == 0 {
		return
	}
	present := map[string]bool{}
	for _, c := range r.Cookies() {
		present[c.Name] = true
	}
	for name, c := range j.cookies {
		if !present[name] {
			// The stored cookie is a RESPONSE cookie, but Request.AddCookie
			// reads only Name/Value/Quoted from it — the Secure, HttpOnly,
			// SameSite and expiry attributes are instructions to a client and
			// have no representation in a Cookie request header at all. So
			// replaying the stored value verbatim is exactly what a browser
			// would send.
			r.AddCookie(c)
		}
	}
}

// capture reads the Set-Cookie headers a handler wrote. A deletion (the
// MaxAge<0 that setSessionCookie uses on logout) removes the entry instead of
// storing an empty one, so logging out really does end the session.
func (j *cookieJar) capture(h http.Header) {
	res := http.Response{Header: h}
	set := res.Cookies()
	if len(set) == 0 {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, c := range set {
		if c.MaxAge < 0 || c.Value == "" {
			delete(j.cookies, c.Name)
			continue
		}
		j.cookies[c.Name] = c
	}
}

// jarWriter feeds every response's Set-Cookie headers to the jar as they are
// committed. WriteHeader is the only point where the header map is final and
// still readable, so the capture hangs off it (and off the implicit
// WriteHeader that a bare Write performs).
type jarWriter struct {
	http.ResponseWriter
	jar         *cookieJar
	wroteHeader bool
}

func (w *jarWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.jar.capture(w.Header())
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *jarWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Flush keeps whatever streaming ability the underlying writer has. Wails'
// production WebView writers are not http.Flushers (see the realtime guard
// above), but the dev-mode asset server's are, and gin's ResponseWriter
// always calls through to this — so pass it on when it exists rather than
// swallowing it.
func (w *jarWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
