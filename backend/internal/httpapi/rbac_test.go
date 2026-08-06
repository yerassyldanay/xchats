package httpapi_test

// rbac_test.go covers Track 2A's role-on-membership RBAC: RequireAdmin
// gating POST /users, PUT /users/:id/role, and PUT /organization; the
// last-admin demotion guard (domain.ErrLastAdmin); and the role surfaced
// through /me and GET /users.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/password"
)

const (
	memberEmail = "member@xchats.test"
	memberPass  = "password123"
)

// loginAs logs a fresh cookie-jar client in as an existing user — how these
// tests act as someone other than the harness's default admin (h.client,
// already logged in as adminEmail by newHarness).
func (h *harness) loginAs(t *testing.T, email, password string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, err := client.Post(h.srv.URL+"/xchats/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login as %s: %v", email, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login as %s: status=%d", email, resp.StatusCode)
	}
	return client
}

// createMember creates a "member"-role user in h's organization and returns
// their id.
func (h *harness) createMember(t *testing.T, email, plaintext, name string) uuid.UUID {
	t.Helper()
	hash, err := password.Hash(plaintext)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := h.store.CreateUser(context.Background(), h.orgID, email, hash, name, "member")
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	return u.ID
}

// adminUserID resolves the id of newHarness's seeded sentinel admin.
func (h *harness) adminUserID(t *testing.T) uuid.UUID {
	t.Helper()
	u, err := h.store.UserByEmail(context.Background(), adminEmail)
	if err != nil {
		t.Fatalf("lookup admin: %v", err)
	}
	return u.ID
}

// requestAs issues method/url with a JSON body through client — for a test
// acting as someone other than h.client's default admin, whose own
// postJSON/putJSON helpers are hardwired to h.client.
func requestAs(t *testing.T, client *http.Client, method, url string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(method, url, bytes.NewReader(b))
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func TestRequireAdminBlocksMemberFromAdminRoutes(t *testing.T) {
	h := newHarness(t)
	h.createMember(t, memberEmail, memberPass, "Member")
	member := h.loginAs(t, memberEmail, memberPass)
	adminID := h.adminUserID(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"create user", http.MethodPost, "/xchats/api/v1/users",
			map[string]string{"email": "blocked@xchats.test", "password": "password123", "name": "Blocked"}},
		{"update role", http.MethodPut, "/xchats/api/v1/users/" + adminID.String() + "/role",
			map[string]string{"role": "member"}},
		{"rename org", http.MethodPut, "/xchats/api/v1/organization",
			map[string]string{"name": "Renamed By Member"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := requestAs(t, member, tc.method, h.srv.URL+tc.path, tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("%s %s as member: status = %d, want 403", tc.method, tc.path, resp.StatusCode)
			}
		})
	}

	// None of the rejected calls may have taken effect.
	role, err := h.store.MembershipRole(context.Background(), h.orgID, adminID)
	if err != nil {
		t.Fatalf("MembershipRole: %v", err)
	}
	if role != "admin" {
		t.Fatalf("admin's role after rejected member requests = %q, want unchanged %q", role, "admin")
	}
}

func TestRequireAdminAllowsAdmin(t *testing.T) {
	h := newHarness(t)
	memberID := h.createMember(t, memberEmail, memberPass, "Member")

	resp, env := h.postJSON("/xchats/api/v1/users", map[string]string{
		"email": "created-by-admin@xchats.test", "password": "password123", "name": "Created",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create user as admin: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var created struct {
		Role string `json:"role"`
	}
	mustPayload(t, env, &created)
	if created.Role != "member" {
		t.Fatalf("newly created user role = %q, want %q", created.Role, "member")
	}

	resp, env = h.putJSON("/xchats/api/v1/users/"+memberID.String()+"/role", map[string]string{"role": "admin"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("promote member: status=%d body=%s", resp.StatusCode, env["message"])
	}
	role, err := h.store.MembershipRole(context.Background(), h.orgID, memberID)
	if err != nil {
		t.Fatalf("MembershipRole: %v", err)
	}
	if role != "admin" {
		t.Fatalf("promoted member's role = %q, want %q", role, "admin")
	}

	resp, env = h.putJSON("/xchats/api/v1/organization", map[string]string{"name": "Renamed Org"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename org: status=%d body=%s", resp.StatusCode, env["message"])
	}
	var org struct {
		Name string `json:"name"`
	}
	mustPayload(t, env, &org)
	if org.Name != "Renamed Org" {
		t.Fatalf("org name after rename = %q, want %q", org.Name, "Renamed Org")
	}
}

func TestSetMembershipRoleRefusesToDemoteLastAdmin(t *testing.T) {
	h := newHarness(t)
	adminID := h.adminUserID(t)

	// newHarness's org IS migration 0006's own sentinel org: SeedOrganization
	// matches "xchats" by name and reuses that row rather than creating a
	// second one, so the org starts with TWO admins — the migration's own
	// sentinel (admin@xchat.kz) alongside the harness's freshly seeded one.
	// Demote the sentinel to member first, so h's own admin is unambiguously
	// the LAST admin this test means to exercise the guard against.
	sentinel, err := h.store.UserByEmail(context.Background(), "admin@xchat.kz")
	if err != nil {
		t.Fatalf("lookup sentinel admin: %v", err)
	}
	if err := h.store.SetMembershipRole(context.Background(), h.orgID, sentinel.ID, "member"); err != nil {
		t.Fatalf("demote sentinel admin: %v", err)
	}

	resp, env := h.putJSON("/xchats/api/v1/users/"+adminID.String()+"/role", map[string]string{"role": "member"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("demote last admin: status = %d, want 409; body=%s", resp.StatusCode, env["message"])
	}

	role, err := h.store.MembershipRole(context.Background(), h.orgID, adminID)
	if err != nil {
		t.Fatalf("MembershipRole: %v", err)
	}
	if role != "admin" {
		t.Fatalf("admin's role after refused demotion = %q, want unchanged %q", role, "admin")
	}
}

func TestSetMembershipRoleAllowsDemotingASecondAdmin(t *testing.T) {
	h := newHarness(t)
	adminID := h.adminUserID(t)
	secondID := h.createMember(t, memberEmail, memberPass, "Second")
	if err := h.store.SetMembershipRole(context.Background(), h.orgID, secondID, "admin"); err != nil {
		t.Fatalf("promote second admin: %v", err)
	}

	// Two admins now — demoting the ORIGINAL one must succeed, since the
	// second one keeps the organization manageable.
	resp, env := h.putJSON("/xchats/api/v1/users/"+adminID.String()+"/role", map[string]string{"role": "member"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("demote one of two admins: status=%d body=%s", resp.StatusCode, env["message"])
	}
	role, err := h.store.MembershipRole(context.Background(), h.orgID, adminID)
	if err != nil {
		t.Fatalf("MembershipRole: %v", err)
	}
	if role != "member" {
		t.Fatalf("original admin's role after demotion = %q, want %q", role, "member")
	}
}

func TestMePayloadIncludesRole(t *testing.T) {
	h := newHarness(t)
	var me struct {
		User struct {
			Role string `json:"role"`
		} `json:"user"`
	}
	h.get("/xchats/api/v1/me", &me)
	if me.User.Role != "admin" {
		t.Fatalf("admin's /me role = %q, want %q", me.User.Role, "admin")
	}

	h.createMember(t, memberEmail, memberPass, "Member")
	member := h.loginAs(t, memberEmail, memberPass)
	resp, err := member.Get(h.srv.URL + "/xchats/api/v1/me")
	if err != nil {
		t.Fatalf("GET /me as member: %v", err)
	}
	defer resp.Body.Close()
	var meMember struct {
		User struct {
			Role string `json:"role"`
		} `json:"user"`
	}
	decodeEnvelope(t, resp, &meMember)
	if meMember.User.Role != "member" {
		t.Fatalf("member's /me role = %q, want %q", meMember.User.Role, "member")
	}
}

func TestListUsersReportsEachMembersRole(t *testing.T) {
	h := newHarness(t)
	h.createMember(t, memberEmail, memberPass, "Member")

	var page struct {
		Items []struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		} `json:"items"`
	}
	h.get("/xchats/api/v1/users", &page)
	roles := map[string]string{}
	for _, item := range page.Items {
		roles[item.Email] = item.Role
	}
	if roles[adminEmail] != "admin" {
		t.Fatalf("roles[%q] = %q, want %q", adminEmail, roles[adminEmail], "admin")
	}
	if roles[memberEmail] != "member" {
		t.Fatalf("roles[%q] = %q, want %q", memberEmail, roles[memberEmail], "member")
	}
}

func TestRequireAdminReflectsRoleChangeImmediately(t *testing.T) {
	h := newHarness(t)
	memberID := h.createMember(t, memberEmail, memberPass, "Member")
	member := h.loginAs(t, memberEmail, memberPass)

	// Before promotion: blocked.
	resp := requestAs(t, member, http.MethodPut, h.srv.URL+"/xchats/api/v1/organization", map[string]string{"name": "First Try"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("rename org before promotion: status = %d, want 403", resp.StatusCode)
	}

	if err := h.store.SetMembershipRole(context.Background(), h.orgID, memberID, "admin"); err != nil {
		t.Fatalf("promote: %v", err)
	}

	// Same still-logged-in session, no re-login: promotion must take effect
	// on the very next request, since RequireAdmin re-derives the role from
	// organization_users on every call rather than caching it on the session.
	resp = requestAs(t, member, http.MethodPut, h.srv.URL+"/xchats/api/v1/organization", map[string]string{"name": "Second Try"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename org after promotion: status = %d, want 200", resp.StatusCode)
	}
}
