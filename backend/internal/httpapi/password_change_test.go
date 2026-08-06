package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/password"
)

// attemptLogin POSTs credentials with a fresh, independent client and
// returns the status code — unlike harness.login()/loginAs, it does not
// fatal on a non-200, since several tests below need to assert ON a
// rejected login.
func attemptLogin(t *testing.T, srvURL, email, plaintext string) (*http.Client, int) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp := requestAs(t, client, http.MethodPost, srvURL+"/xchats/api/v1/auth/login",
		map[string]string{"email": email, "password": plaintext})
	resp.Body.Close()
	return client, resp.StatusCode
}

func TestChangePassword_HappyPathRotatesCredentialAndEvictsOtherSessions(t *testing.T) {
	h := newHarness(t)

	// A second, independent session for the SAME admin — the one the
	// change below must evict.
	otherClient, status := attemptLogin(t, h.srv.URL, adminEmail, adminPass)
	if status != http.StatusOK {
		t.Fatalf("second admin session login: status=%d", status)
	}

	const newPass = "a-new-strong-password-1"
	resp, env := h.postJSON("/xchats/api/v1/auth/password", map[string]string{
		"current_password": adminPass, "new_password": newPass,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("change password: status=%d body=%v", resp.StatusCode, env)
	}
	var payload struct {
		User struct {
			MustChangePassword bool `json:"must_change_password"`
		} `json:"user"`
	}
	mustPayload(t, env, &payload)
	if payload.User.MustChangePassword {
		t.Error("must_change_password still true in the response after a successful change")
	}

	// The OTHER session is now dead.
	if resp2 := requestAs(t, otherClient, http.MethodGet, h.srv.URL+"/xchats/api/v1/me", nil); resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("other session GET /me status=%d, want 401 (evicted)", resp2.StatusCode)
	}

	// The old password no longer logs in; the new one does.
	if _, status := attemptLogin(t, h.srv.URL, adminEmail, adminPass); status != http.StatusUnauthorized {
		t.Errorf("old password login status=%d, want 401", status)
	}
	if _, status := attemptLogin(t, h.srv.URL, adminEmail, newPass); status != http.StatusOK {
		t.Errorf("new password login status=%d, want 200", status)
	}
}

func TestChangePassword_WrongCurrentPasswordRejected(t *testing.T) {
	h := newHarness(t)
	resp, _ := h.postJSON("/xchats/api/v1/auth/password", map[string]string{
		"current_password": "totally-the-wrong-password", "new_password": "a-new-strong-password-1",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401", resp.StatusCode)
	}
	// Rejected — the old password must still work.
	if _, status := attemptLogin(t, h.srv.URL, adminEmail, adminPass); status != http.StatusOK {
		t.Errorf("original password login after a rejected attempt: status=%d, want 200", status)
	}
}

func TestChangePassword_ShortNewPasswordRejected(t *testing.T) {
	h := newHarness(t)
	resp, _ := h.postJSON("/xchats/api/v1/auth/password", map[string]string{
		"current_password": adminPass, "new_password": "short",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", resp.StatusCode)
	}
}

func TestChangePassword_SameAsCurrentRejected(t *testing.T) {
	h := newHarness(t)
	resp, _ := h.postJSON("/xchats/api/v1/auth/password", map[string]string{
		"current_password": adminPass, "new_password": adminPass,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", resp.StatusCode)
	}
}

// seedMustChangeUser creates an admin user already flagged
// must_change_password — the state migration 0008 + the first-boot
// bootstrap leave the sentinel admin in (see cmd/xchats' bootstrap admin
// logic), reproduced directly here since this package sits outside the
// persistence boundary migration 0008 itself belongs to.
func seedMustChangeUser(t *testing.T, h *harness, email, plaintext string) {
	t.Helper()
	hash, err := password.Hash(plaintext)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := h.store.CreateUser(context.Background(), h.orgID, email, hash, "Bootstrap Admin", "admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := h.db.Exec(context.Background(), `UPDATE users SET must_change_password = 1 WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("flag must_change_password: %v", err)
	}
}

func TestRequirePasswordChanged_BlocksUntilChanged(t *testing.T) {
	h := newHarness(t)
	const email, onceOff = "must-change@xchats.test", "one-time-password-123"
	seedMustChangeUser(t, h, email, onceOff)

	client, status := attemptLogin(t, h.srv.URL, email, onceOff)
	if status != http.StatusOK {
		t.Fatalf("login with the one-time password: status=%d", status)
	}

	// Blocked: an ordinary gated route.
	if resp := requestAs(t, client, http.MethodGet, h.srv.URL+"/xchats/api/v1/accounts", nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("GET /accounts before change: status=%d, want 403", resp.StatusCode)
	}

	// Allowed even while gated: /me and /auth/password themselves.
	if resp := requestAs(t, client, http.MethodGet, h.srv.URL+"/xchats/api/v1/me", nil); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /me before change: status=%d, want 200", resp.StatusCode)
	}
	resp := requestAs(t, client, http.MethodPost, h.srv.URL+"/xchats/api/v1/auth/password", map[string]string{
		"current_password": onceOff, "new_password": "a-real-password-now-1",
	})
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("change password: status=%d body=%s", resp.StatusCode, b)
	}

	// Unblocked afterward.
	if resp := requestAs(t, client, http.MethodGet, h.srv.URL+"/xchats/api/v1/accounts", nil); resp.StatusCode != http.StatusOK {
		t.Errorf("GET /accounts after change: status=%d, want 200", resp.StatusCode)
	}
}

// TestRequirePasswordChanged_OAuthAuthorizeGated is the MCP-side twin of
// TestRequirePasswordChanged_BlocksUntilChanged: /oauth/authorize sits
// outside the `auth` route group entirely (it resolves its session
// manually — see oauthSessionUser — so a browser gets a rendered error
// page instead of a 401 for "not logged in"), which means
// requirePasswordChanged's gate does not cover it and it needs its own
// check (handleOAuthAuthorize, mcp_oauth.go). This proves that check runs.
func TestRequirePasswordChanged_OAuthAuthorizeGated(t *testing.T) {
	h := newMCPHarness(t)
	const email, onceOff = "must-change-mcp@xchats.test", "one-time-password-123"

	hash, err := password.Hash(onceOff)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := h.store.CreateUser(context.Background(), h.orgID, email, hash, "Bootstrap Admin", "admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := h.db.Exec(context.Background(), `UPDATE users SET must_change_password = 1 WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("flag must_change_password: %v", err)
	}

	client, status := attemptLogin(t, h.srv.URL, email, onceOff)
	if status != http.StatusOK {
		t.Fatalf("login with the one-time password: status=%d", status)
	}

	clientID := h.registerClient(t, "https://client.test/callback")
	_, challenge := pkcePair()
	q := authorizeQuery(clientID, "https://client.test/callback", challenge, "test-state", "")
	resp, err := client.Get(h.srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("GET /oauth/authorize: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status=%d, want 400 (renderOAuthError)", resp.StatusCode)
	}
	if !strings.Contains(string(body), "смените пароль") {
		t.Errorf("body does not mention the password-change requirement: %s", body)
	}
}
