package desktop

import (
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"os"
	"sync/atomic"
)

// e2e_http.go is the opt-in local-testing surface docs/desktop.md's "Local
// Executable UI Testing" section describes: Playwright cannot drive Wails'
// own asset server (a custom URL scheme on macOS/Linux, WebView2's buffered
// writer on Windows — none of it a normal browser can navigate to), so
// XCHATS_DESKTOP_E2E_HTTP=1 instead mounts the EXACT SAME SPA-vs-API split
// the Wails window gets (NewMiddleware/NewSPAHandler, handler.go) onto the
// desktop executable's existing loopback HTTP listener, where a real
// Chromium instance can reach it like any other page. Nothing here changes
// production behavior: the env var defaults unset, and ServeE2EHTTP returns
// next completely unwrapped whenever it is unset — no extra route, no extra
// branch, byte-for-byte the handler that always shipped.

// E2EHTTPEnv is the opt-in switch — see make desktop-test-ui
// (Makefile) and frontend/tests/desktop-e2e/.
const E2EHTTPEnv = "XCHATS_DESKTOP_E2E_HTTP"

// readyPath is the readiness endpoint the Playwright harness polls before
// driving the SPA — see Readiness.
const readyPath = "/__xchats_e2e/ready"

// E2EHTTPEnabled reports whether XCHATS_DESKTOP_E2E_HTTP=1 was set at
// startup. This is a test-only escape hatch, not a general boolean-env
// parser: any spelling other than the literal "1" (including unset) is
// disabled, so there is exactly one way to turn it on.
func E2EHTTPEnabled() bool {
	return os.Getenv(E2EHTTPEnv) == "1"
}

// Readiness tracks the two independent "has this actually started" signals
// an E2E test needs before it is safe to drive the SPA: the backend HTTP
// listener accepting connections, and the Wails window finishing its own
// OnStartup (shell.go — a desktop-tagged file with no other way to tell
// this untagged package "the window is up" than through a shared flag like
// this one). Both start unset, so a harness polling before either fires is
// correctly told to keep waiting rather than observing a stale default.
// Safe for concurrent use: SetBackendReady/SetWindowReady run from runServe
// and shell.go's OnStartup respectively, Ready from every request goroutine
// the readiness endpoint serves.
type Readiness struct {
	backend atomic.Bool
	window  atomic.Bool
}

// SetBackendReady marks the HTTP listener as accepting connections.
func (r *Readiness) SetBackendReady() { r.backend.Store(true) }

// SetWindowReady marks the Wails window as having completed OnStartup.
func (r *Readiness) SetWindowReady() { r.window.Store(true) }

// Ready reports whether both signals have fired.
func (r *Readiness) Ready() bool { return r.backend.Load() && r.window.Load() }

// ServeE2EHTTP wraps next — the ordinary loopback listener's handler, the
// plain API router in production — with the SPA-vs-API split the Wails
// window gets, plus the readiness endpoint, but ONLY when E2EHTTPEnabled
// and addr is a loopback address. Any other case (disabled, or a listener
// some operator's config.yaml pointed at a non-loopback host) returns next
// completely unchanged: no test-only route is ever registered outside the
// one opt-in case, so a misconfigured public-facing deployment cannot
// expose this surface no matter how the env var ends up set.
//
// api is the backend router NewMiddleware should answer API/media requests
// from (internal/httpapi's *gin.Engine in production — the same value as
// next). assets is the SPA bundle (desktop.Assets in production; a stub FS
// in tests, since this file carries no build tag and assets.go's real embed
// does). ready is polled by readyPath; a nil ready reports not-ready rather
// than panicking, so a caller that has not wired one up yet fails safe.
func ServeE2EHTTP(next http.Handler, api http.Handler, assets fs.FS, addr string, ready *Readiness) http.Handler {
	if !E2EHTTPEnabled() {
		return next
	}
	if !isLoopbackHostPort(addr) {
		return next
	}
	assetServer := NewMiddleware(api)(NewSPAHandler(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == readyPath {
			serveReadiness(w, ready)
			return
		}
		assetServer.ServeHTTP(w, r)
	})
}

func serveReadiness(w http.ResponseWriter, ready *Readiness) {
	backendReady, windowReady := false, false
	if ready != nil {
		backendReady, windowReady = ready.backend.Load(), ready.window.Load()
	}
	w.Header().Set("Content-Type", "application/json")
	if !backendReady || !windowReady {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(map[string]bool{
		"ready":   backendReady && windowReady,
		"backend": backendReady,
		"window":  windowReady,
	})
}

// isLoopbackHostPort reports whether addr (a net.Listen-style "host:port",
// as cfg.Server.HTTPAddr always is once loaded) names a loopback host. A
// host that fails to split (empty, malformed) is treated as non-loopback —
// fail closed, matching ServeE2EHTTP's doc comment.
func isLoopbackHostPort(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
