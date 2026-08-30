package httpapi_test

import (
	"net/http"
	"net/http/cookiejar"
	"testing"
)

// TestBootstrapStatus pins GET /auth/bootstrap-status — the public, session-
// less probe Login.vue's "Fill default admin credentials" helper
// (docs/ux/flows/01-onboarding.md, friction point 1) polls before a user has
// signed in at all. newHarness's database still carries migration 0006's
// sentinel admin (admin@xchat.kz) alongside the harness's own separately
// seeded admin — see rbac_test.go's TestSetMembershipRoleRefusesToDemoteLastAdmin
// for that same fact — and 0014_force_default_admin_password_change leaves
// that sentinel on the documented default password with must_change_password
// set, so a fresh harness must report the helper as available.
func TestBootstrapStatus(t *testing.T) {
	h := newHarness(t)

	// A bare, cookie-less client: this route must work with NO session at all.
	jar, _ := cookiejar.New(nil)
	anon := &http.Client{Jar: jar}

	resp := requestAs(t, anon, http.MethodGet, h.srv.URL+"/xchats/api/v1/auth/bootstrap-status", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /auth/bootstrap-status unauthenticated: status=%d, want 200", resp.StatusCode)
	}
	var payload struct {
		DefaultAdminAvailable bool `json:"default_admin_available"`
	}
	decodeEnvelope(t, resp, &payload)
	if !payload.DefaultAdminAvailable {
		t.Error("default_admin_available = false on a fresh harness, want true (sentinel still on the documented default)")
	}

	// Once the sentinel admin's forced change succeeds, the helper must stop
	// offering the now-stale documented credential.
	sentinelClient, status := attemptLogin(t, h.srv.URL, "admin@xchat.kz", "xchat-admin-change-me")
	if status != http.StatusOK {
		t.Fatalf("login as sentinel with the documented default: status=%d", status)
	}
	changeResp := requestAs(t, sentinelClient, http.MethodPost, h.srv.URL+"/xchats/api/v1/auth/password", map[string]string{
		"current_password": "xchat-admin-change-me",
		"new_password":     "a-new-strong-password-1",
	})
	changeResp.Body.Close()
	if changeResp.StatusCode != http.StatusOK {
		t.Fatalf("sentinel change password: status=%d", changeResp.StatusCode)
	}

	resp2 := requestAs(t, anon, http.MethodGet, h.srv.URL+"/xchats/api/v1/auth/bootstrap-status", nil)
	defer resp2.Body.Close()
	var payload2 struct {
		DefaultAdminAvailable bool `json:"default_admin_available"`
	}
	decodeEnvelope(t, resp2, &payload2)
	if payload2.DefaultAdminAvailable {
		t.Error("default_admin_available = true after the sentinel's forced change succeeded, want false")
	}
}
