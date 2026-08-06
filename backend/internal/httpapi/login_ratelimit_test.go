package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestLogin_RateLimited is A2's regression guard, mirroring
// TestMCPOAuthRegister_RateLimited's shape: burst exhaustion is instant, no
// timing dependency. newHarness's own setup already logs in once (consuming
// one token from the same per-Server loginLimit, keyed by client IP — the
// harness's own client and this test's ad hoc requests all originate from
// the same loopback address as far as the limiter is concerned), so this
// sends more than enough further attempts to guarantee exhaustion
// regardless of that head start.
func TestLogin_RateLimited(t *testing.T) {
	h := newHarness(t)
	var last *http.Response
	for i := 0; i < 6; i++ {
		body, _ := json.Marshal(map[string]string{"email": adminEmail, "password": "wrong-on-purpose"})
		resp, err := http.Post(h.srv.URL+"/xchats/api/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("login attempt %d: %v", i, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		last = resp
	}
	if last.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected a rapid run of login attempts to be rate-limited (429), got %d", last.StatusCode)
	}
}

// TestChangePassword_RateLimited is the same guard for POST /auth/password
// — it shares loginLimit with /auth/login (see the field's doc comment in
// server.go), so a caller who's authenticated can still only hammer the
// expensive argon2id verify a bounded number of times.
func TestChangePassword_RateLimited(t *testing.T) {
	h := newHarness(t)
	var last *http.Response
	for i := 0; i < 6; i++ {
		resp, _ := h.postJSON("/xchats/api/v1/auth/password", map[string]string{
			"current_password": "wrong-on-purpose", "new_password": "a-new-strong-password-1",
		})
		last = resp
	}
	if last.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected a rapid run of change-password attempts to be rate-limited (429), got %d", last.StatusCode)
	}
}
