package httpapi_test

// cookie_security_test.go covers requestIsSecure: the session cookie's
// Secure attribute must follow the request's real origin scheme, honoring
// X-Forwarded-Proto so it still gains Secure when reached through a
// TLS-terminating front door — the embedded ngrok tunnel (internal/tunnel)
// included — even though the app's own listener only ever sees plain HTTP
// locally in every one of these deployment shapes.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func loginSetCookieHeader(t *testing.T, h *harness, extraHeaders map[string]string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": adminEmail, "password": adminPass})
	req, err := http.NewRequest(http.MethodPost, h.srv.URL+"/xchats/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	// A jarless client: the cookie jar on h.client would otherwise silently
	// consume Set-Cookie before this test can inspect its raw attributes.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d", resp.StatusCode)
	}
	return resp.Header.Get("Set-Cookie")
}

func TestSessionCookieNotSecureOverPlainHTTPByDefault(t *testing.T) {
	h := newHarness(t)
	setCookie := loginSetCookieHeader(t, h, nil)
	if strings.Contains(strings.ToLower(setCookie), "secure") {
		t.Errorf("Set-Cookie = %q, want no Secure attribute over plain HTTP with no forwarded-proto signal", setCookie)
	}
}

func TestSessionCookieSecureWhenForwardedProtoIsHTTPS(t *testing.T) {
	h := newHarness(t)
	setCookie := loginSetCookieHeader(t, h, map[string]string{"X-Forwarded-Proto": "https"})
	if !strings.Contains(strings.ToLower(setCookie), "secure") {
		t.Errorf("Set-Cookie = %q, want a Secure attribute when X-Forwarded-Proto: https (the tunnel's own signal)", setCookie)
	}
}

func TestSessionCookieSecureWhenConfigForcesIt(t *testing.T) {
	h := newHarness(t)
	h.cfg.Server.SecureCookies = true
	setCookie := loginSetCookieHeader(t, h, nil)
	if !strings.Contains(strings.ToLower(setCookie), "secure") {
		t.Errorf("Set-Cookie = %q, want a Secure attribute when SecureCookies is force-enabled", setCookie)
	}
}
