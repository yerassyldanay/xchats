package desktop

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func stubAssets() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>stub SPA</html>")},
		"assets/app.js": {Data: []byte("console.log('stub')")},
	}
}

func stubAPI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"from":"api"}`))
	})
}

// TestServeE2EHTTP_DisabledByDefault proves that with the env var unset,
// ServeE2EHTTP returns next completely unwrapped — no readiness route, no
// SPA route, nothing but the handler that was always there. This is the
// "absent by default" requirement: production behavior must not change one
// bit just because this file exists in the binary.
func TestServeE2EHTTP_DisabledByDefault(t *testing.T) {
	t.Setenv(E2EHTTPEnv, "")
	next := stubAPI()
	got := ServeE2EHTTP(next, stubAPI(), stubAssets(), "127.0.0.1:8080", &Readiness{})

	req := httptest.NewRequest(http.MethodGet, readyPath, nil)
	rec := httptest.NewRecorder()
	got.ServeHTTP(rec, req)
	if rec.Body.String() != `{"from":"api"}` {
		t.Errorf("disabled mode routed %s through the wrapper instead of straight to next; body=%s", readyPath, rec.Body.String())
	}
}

// TestServeE2EHTTP_WrongEnvValueStaysDisabled proves only the exact literal
// "1" activates it — not "true", not "yes", nothing else.
func TestServeE2EHTTP_WrongEnvValueStaysDisabled(t *testing.T) {
	for _, v := range []string{"true", "yes", "on", "0", "TRUE"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv(E2EHTTPEnv, v)
			if E2EHTTPEnabled() {
				t.Errorf("E2EHTTPEnabled() with %s=%q = true, want false", E2EHTTPEnv, v)
			}
		})
	}
}

// TestServeE2EHTTP_RefusesNonLoopbackAddr proves a non-loopback listen
// address disables E2E HTTP mode even when the env var is set — defense in
// depth against exposing the SPA-serving/cookie-jar surface on a network
// interface other machines can reach, for a deployment whose config.yaml
// pinned server.http_addr somewhere ApplyDefaults' wildcard rewrite does
// not touch.
func TestServeE2EHTTP_RefusesNonLoopbackAddr(t *testing.T) {
	t.Setenv(E2EHTTPEnv, "1")
	for _, addr := range []string{"0.0.0.0:8080", "192.168.1.5:8080", "10.0.0.1:8080", "", "not-a-host-port"} {
		t.Run(addr, func(t *testing.T) {
			next := stubAPI()
			got := ServeE2EHTTP(next, stubAPI(), stubAssets(), addr, &Readiness{})
			req := httptest.NewRequest(http.MethodGet, readyPath, nil)
			rec := httptest.NewRecorder()
			got.ServeHTTP(rec, req)
			if rec.Body.String() != `{"from":"api"}` {
				t.Errorf("addr=%q: E2E routes were reachable, want refused (fall through to next); body=%s", addr, rec.Body.String())
			}
		})
	}
}

// TestServeE2EHTTP_AcceptsLoopbackAddrs proves every loopback spelling
// ApplyDefaults can produce activates E2E mode.
func TestServeE2EHTTP_AcceptsLoopbackAddrs(t *testing.T) {
	t.Setenv(E2EHTTPEnv, "1")
	for _, addr := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080"} {
		t.Run(addr, func(t *testing.T) {
			got := ServeE2EHTTP(stubAPI(), stubAPI(), stubAssets(), addr, &Readiness{})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			got.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || rec.Body.String() != "<html>stub SPA</html>" {
				t.Errorf("addr=%q: GET / = %d %q, want the SPA index", addr, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestServeE2EHTTP_ServesSPAAndAPI proves an enabled, loopback-addressed
// wrapper serves the embedded SPA for a UI route, an asset file verbatim,
// and routes an API path to api rather than the SPA — exactly what
// NewMiddleware/NewSPAHandler already guarantee for the Wails window,
// carried over unchanged onto the loopback listener.
func TestServeE2EHTTP_ServesSPAAndAPI(t *testing.T) {
	t.Setenv(E2EHTTPEnv, "1")
	handler := ServeE2EHTTP(stubAPI(), stubAPI(), stubAssets(), "127.0.0.1:8080", &Readiness{})

	t.Run("SPA route serves index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/knowledge-base", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "<html>stub SPA</html>" {
			t.Errorf("GET /knowledge-base = %d %q, want the SPA index (history-mode fallback)", rec.Code, rec.Body.String())
		}
	})

	t.Run("asset file serves verbatim", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET /assets/app.js = %d, want 200", rec.Code)
		}
	})

	t.Run("API path routes to the backend, not the SPA", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/xchats/api/v1/me", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Body.String() != `{"from":"api"}` {
			t.Errorf("GET /xchats/api/v1/me body = %q, want the API stub's response", rec.Body.String())
		}
	})

	t.Run("media path routes to the backend", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/xchats/api/v1/media/abc123", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Body.String() != `{"from":"api"}` {
			t.Errorf("GET media path body = %q, want the API stub's response", rec.Body.String())
		}
	})
}

// TestServeE2EHTTP_ReadinessEndpoint proves /__xchats_e2e/ready reports 503
// until both signals fire, then 200 with ready:true — and that the two
// signals are independent (backend alone is not enough).
func TestServeE2EHTTP_ReadinessEndpoint(t *testing.T) {
	t.Setenv(E2EHTTPEnv, "1")
	ready := &Readiness{}
	handler := ServeE2EHTTP(stubAPI(), stubAPI(), stubAssets(), "127.0.0.1:8080", ready)

	poll := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, readyPath, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := poll(); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("before anything is ready: status=%d, want 503; body=%s", rec.Code, rec.Body.String())
	}

	ready.SetBackendReady()
	if rec := poll(); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("backend ready but window not: status=%d, want 503 (both must be ready); body=%s", rec.Code, rec.Body.String())
	}

	ready.SetWindowReady()
	rec := poll()
	if rec.Code != http.StatusOK {
		t.Fatalf("both ready: status=%d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"backend":true,"ready":true,"window":true}`+"\n" {
		t.Errorf("ready body = %s, want backend/window/ready all true", got)
	}
}

// TestServeE2EHTTP_ReadinessWithNilReadiness proves a nil *Readiness (a
// caller that has not wired one up) fails safe as not-ready rather than
// panicking — the readiness endpoint is meant to be robust scaffolding for
// a test harness, not another thing that can crash the app.
func TestServeE2EHTTP_ReadinessWithNilReadiness(t *testing.T) {
	t.Setenv(E2EHTTPEnv, "1")
	handler := ServeE2EHTTP(stubAPI(), stubAPI(), stubAssets(), "127.0.0.1:8080", nil)
	req := httptest.NewRequest(http.MethodGet, readyPath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil Readiness: status=%d, want 503", rec.Code)
	}
}

func TestIsLoopbackHostPort(t *testing.T) {
	loopback := []string{"127.0.0.1:8080", "127.0.0.1:0", "localhost:8080", "[::1]:8080"}
	for _, addr := range loopback {
		if !isLoopbackHostPort(addr) {
			t.Errorf("isLoopbackHostPort(%q) = false, want true", addr)
		}
	}
	notLoopback := []string{"0.0.0.0:8080", "[::]:8080", "192.168.1.5:8080", "10.0.0.1:8080", "example.com:8080", "", "garbage", "127.0.0.1"}
	for _, addr := range notLoopback {
		if isLoopbackHostPort(addr) {
			t.Errorf("isLoopbackHostPort(%q) = true, want false", addr)
		}
	}
}
