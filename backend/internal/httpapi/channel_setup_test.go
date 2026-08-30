package httpapi_test

// channel_setup_test.go covers GET/PUT /channel-setup — the installation-
// wide prerequisite checklist that supersedes the old admin-only
// /settings/meta-setup (see channel_setup.go's own doc comments). It
// exercises three things the old meta_setup_test.go never had to: the
// member/admin payload split, the five-entry dependency-order status
// matrix, and the two credential-saving endpoints.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/credentials"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
)

// channelSetup decodes GET /channel-setup's payload for h's default (admin)
// client.
func (h *metaHarness) channelSetup(t *testing.T) httpapi.ChannelSetupInfo {
	t.Helper()
	resp, env := h.getJSON("/xchats/api/v1/channel-setup")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /channel-setup: status = %d, body = %s", resp.StatusCode, env["message"])
	}
	var info httpapi.ChannelSetupInfo
	if err := json.Unmarshal(env["payload"], &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return info
}

func assertEntryStatuses(t *testing.T, info httpapi.ChannelSetupInfo, want map[string]string) {
	t.Helper()
	if len(info.Entries) != 5 {
		t.Fatalf("entries = %d, want 5: %+v", len(info.Entries), info.Entries)
	}
	got := map[string]string{}
	for _, e := range info.Entries {
		got[e.Key] = e.Status
	}
	for k, wantStatus := range want {
		if got[k] != wantStatus {
			t.Errorf("entry %q status = %q, want %q (all: %+v)", k, got[k], wantStatus, got)
		}
	}
}

func entryByKey(info httpapi.ChannelSetupInfo, key string) httpapi.ChannelSetupEntry {
	for _, e := range info.Entries {
		if e.Key == key {
			return e
		}
	}
	return httpapi.ChannelSetupEntry{}
}

// --- status matrix (B4) -----------------------------------------------------

func TestChannelSetupStatusMatrix(t *testing.T) {
	t.Run("fresh install: only public access is preconfigured by the harness", func(t *testing.T) {
		h := newMetaHarness(t)
		info := h.channelSetup(t)
		assertEntryStatuses(t, info, map[string]string{
			httpapi.SetupKeyPublicAccess:  "ready",
			httpapi.SetupKeyMetaApp:       "not_configured",
			httpapi.SetupKeyInstagram:     "setup_required",
			httpapi.SetupKeyMessenger:     "setup_required",
			httpapi.SetupKeyWhatsAppCloud: "setup_required",
		})
	})

	t.Run("no public HTTPS address at all", func(t *testing.T) {
		h := newMetaHarness(t)
		h.cfg.Meta.WebhookPublicBaseURL = ""
		h.cfg.Server.APIBaseURL = "http://xchats.test" // not https — ResolvedAPIBaseURL fallback must not kick in either
		info := h.channelSetup(t)
		if info.PublicBaseURL != "" {
			t.Fatalf("public_base_url = %q, want empty", info.PublicBaseURL)
		}
		assertEntryStatuses(t, info, map[string]string{
			httpapi.SetupKeyPublicAccess:  "not_configured",
			httpapi.SetupKeyMetaApp:       "not_configured",
			httpapi.SetupKeyInstagram:     "setup_required",
			httpapi.SetupKeyMessenger:     "setup_required",
			httpapi.SetupKeyWhatsAppCloud: "setup_required",
		})
	})

	t.Run("meta app configured unlocks messenger and whatsapp_cloud but not instagram", func(t *testing.T) {
		h := newMetaHarness(t)
		h.setAppCredentials("meta-id", "meta-secret")
		info := h.channelSetup(t)
		assertEntryStatuses(t, info, map[string]string{
			httpapi.SetupKeyPublicAccess:  "ready",
			httpapi.SetupKeyMetaApp:       "ready",
			httpapi.SetupKeyInstagram:     "setup_required",
			httpapi.SetupKeyMessenger:     "ready",
			httpapi.SetupKeyWhatsAppCloud: "ready",
		})
	})

	t.Run("instagram pair alone does not unlock instagram without the meta pair", func(t *testing.T) {
		h := newMetaHarness(t)
		h.setInstagramAppCredentials("ig-id", "ig-secret")
		info := h.channelSetup(t)
		assertEntryStatuses(t, info, map[string]string{
			httpapi.SetupKeyMetaApp:   "not_configured",
			httpapi.SetupKeyInstagram: "setup_required",
		})
	})

	t.Run("everything configured", func(t *testing.T) {
		h := newMetaHarness(t)
		h.setAppCredentials("meta-id", "meta-secret")
		h.setInstagramAppCredentials("ig-id", "ig-secret")
		info := h.channelSetup(t)
		assertEntryStatuses(t, info, map[string]string{
			httpapi.SetupKeyPublicAccess:  "ready",
			httpapi.SetupKeyMetaApp:       "ready",
			httpapi.SetupKeyInstagram:     "ready",
			httpapi.SetupKeyMessenger:     "ready",
			httpapi.SetupKeyWhatsAppCloud: "ready",
		})
	})
}

// TestChannelSetupNeverReportsConnected pins down that "connected" — the
// per-ACCOUNT connection state — never leaks into an installation
// prerequisite's status vocabulary, on any entry, in any configuration.
func TestChannelSetupNeverReportsConnected(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("meta-id", "meta-secret")
	h.setInstagramAppCredentials("ig-id", "ig-secret")
	info := h.channelSetup(t)
	for _, e := range info.Entries {
		if e.Status == "connected" {
			t.Fatalf("entry %q status = connected — that vocabulary is reserved for account connection state", e.Key)
		}
	}
}

// --- member vs admin payload split ------------------------------------------

func TestChannelSetupMemberPayloadCarriesOnlyStatusesAndPublicURL(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("meta-id", "meta-secret")
	h.setInstagramAppCredentials("ig-id", "ig-secret")
	h.createMember(t, memberEmail, memberPass, "Member")
	member := h.loginAs(t, memberEmail, memberPass)

	resp, err := member.Get(h.srv.URL + "/xchats/api/v1/channel-setup")
	if err != nil {
		t.Fatalf("GET as member: %v", err)
	}
	defer resp.Body.Close()
	var env map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	raw := string(env["payload"])

	// Admin-only keys must be entirely ABSENT — not present-but-empty.
	for _, forbidden := range []string{
		`"verify_token"`, `"graph_api_version"`, `"dashboard_url"`, `"stale_accounts"`,
		`"webhook_callback"`, `"redirect_uri"`, `"scopes"`, `"subscribe_fields"`, `"dashboard_path"`, `"dashboard_fields"`,
	} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("member payload must not contain %s: %s", forbidden, raw)
		}
	}

	var info httpapi.ChannelSetupInfo
	if err := json.Unmarshal(env["payload"], &info); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if info.CanConfigure {
		t.Error("can_configure = true for a member")
	}
	if info.PublicBaseURL == "" {
		t.Error("public_base_url must still be present for a member — it is a public address, never a secret")
	}
	assertEntryStatuses(t, info, map[string]string{
		httpapi.SetupKeyPublicAccess:  "ready",
		httpapi.SetupKeyMetaApp:       "ready",
		httpapi.SetupKeyInstagram:     "ready",
		httpapi.SetupKeyMessenger:     "ready",
		httpapi.SetupKeyWhatsAppCloud: "ready",
	})
}

// TestChannelSetupMemberPayloadListsAdminContacts pins
// docs/ux/flows/03b-connect-instagram-messenger.md's friction point 2: a
// member blocked on a missing prerequisite gets someone to ask, and an
// admin — who has no use for this — never gets the field at all.
func TestChannelSetupMemberPayloadListsAdminContacts(t *testing.T) {
	h := newMetaHarness(t)
	h.createMember(t, memberEmail, memberPass, "Member")
	member := h.loginAs(t, memberEmail, memberPass)

	resp, err := member.Get(h.srv.URL + "/xchats/api/v1/channel-setup")
	if err != nil {
		t.Fatalf("GET as member: %v", err)
	}
	defer resp.Body.Close()
	var env map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	var info httpapi.ChannelSetupInfo
	if err := json.Unmarshal(env["payload"], &info); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(info.AdminContacts) != 1 {
		t.Fatalf("admin_contacts = %+v, want exactly the one seeded admin", info.AdminContacts)
	}
	if info.AdminContacts[0].Email != metaAdminEmail {
		t.Errorf("admin_contacts[0].email = %q, want %q", info.AdminContacts[0].Email, metaAdminEmail)
	}

	// The admin payload must never carry this — it is a member-only field.
	adminInfo := h.channelSetup(t)
	if adminInfo.AdminContacts != nil {
		t.Errorf("admin payload must not carry admin_contacts, got %+v", adminInfo.AdminContacts)
	}
}

func TestChannelSetupAdminPayloadIncludesFullDashboardChecklist(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("meta-id", "meta-secret")
	h.setInstagramAppCredentials("ig-id", "ig-secret")
	info := h.channelSetup(t)

	if !info.CanConfigure {
		t.Fatal("can_configure = false for an admin")
	}
	if info.VerifyToken == "" {
		t.Error("verify_token must not be empty for an admin")
	}
	if info.GraphAPIVersion == "" {
		t.Error("graph_api_version must not be empty for an admin")
	}
	if info.DashboardURL == "" {
		t.Error("dashboard_url must not be empty for an admin")
	}
	if info.StaleAccounts == nil {
		t.Fatal("stale_accounts must be present (non-nil) for an admin, even with zero stale accounts")
	}

	ig := entryByKey(info, httpapi.SetupKeyInstagram)
	if ig.RedirectURI != "https://xchats.test/meta/api/v1/oauth/instagram/callback" {
		t.Errorf("instagram redirect_uri = %q", ig.RedirectURI)
	}
	if ig.WebhookCallback != "https://xchats.test/meta/api/v1/webhook/instagram" {
		t.Errorf("instagram webhook_callback = %q", ig.WebhookCallback)
	}
	if len(ig.DashboardFields) == 0 {
		t.Error("instagram must carry at least one dashboard_fields entry")
	}

	fb := entryByKey(info, httpapi.SetupKeyMessenger)
	if fb.RedirectURI != "https://xchats.test/meta/api/v1/oauth/messenger/callback" {
		t.Errorf("messenger redirect_uri = %q", fb.RedirectURI)
	}
	if len(fb.DashboardFields) < 3 {
		t.Errorf("messenger dashboard_fields = %+v, want at least Redirect URI + App Domains + Site URL", fb.DashboardFields)
	}

	wa := entryByKey(info, httpapi.SetupKeyWhatsAppCloud)
	if wa.RedirectURI != "" {
		t.Errorf("whatsapp_cloud redirect_uri = %q, want empty (manual connect, no OAuth redirect)", wa.RedirectURI)
	}
	if wa.WebhookCallback == "" {
		t.Error("whatsapp_cloud webhook_callback must not be empty")
	}
}

// TestChannelSetupAppDomainFieldIsBareHost pins down A1's exact fix: App
// Domains wants a bare host, not a scheme+host URL — pasting the full
// public_base_url there is the motivating bug this whole feature starts
// from (see this package's meta_oauth.go doc comments).
func TestChannelSetupAppDomainFieldIsBareHost(t *testing.T) {
	h := newMetaHarness(t)
	info := h.channelSetup(t)
	fb := entryByKey(info, httpapi.SetupKeyMessenger)
	for _, f := range fb.DashboardFields {
		if f.Field == "App Domains" {
			if f.Value != "xchats.test" {
				t.Fatalf("App Domains value = %q, want bare host %q", f.Value, "xchats.test")
			}
			return
		}
	}
	t.Fatal("no App Domains dashboard field found on the messenger entry")
}

// --- PUT /channel-setup/meta-app --------------------------------------------

func TestSaveMetaAppRequiresAdmin(t *testing.T) {
	h := newMetaHarness(t)
	h.createMember(t, memberEmail, memberPass, "Member")
	member := h.loginAs(t, memberEmail, memberPass)
	resp := requestAs(t, member, http.MethodPut, h.srv.URL+"/xchats/api/v1/channel-setup/meta-app",
		map[string]any{"app_id": "x", "app_secret": "y"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestSaveMetaAppMissingFieldIs400(t *testing.T) {
	h := newMetaHarness(t)
	resp, env := h.putJSON("/xchats/api/v1/channel-setup/meta-app", map[string]any{"app_id": "x"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
}

func TestSaveMetaAppValidCredentialSavesAndReportsReady(t *testing.T) {
	h := newMetaHarness(t)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mock.Close()
	h.setMetaBaseURL(mock.URL)

	resp, env := h.putJSON("/xchats/api/v1/channel-setup/meta-app", map[string]any{"app_id": "new-id", "app_secret": "new-secret"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	if strings.Contains(string(env["payload"]), "new-secret") {
		t.Fatal("the saved secret must never be echoed back")
	}
	var info httpapi.ChannelSetupInfo
	if err := json.Unmarshal(env["payload"], &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if entryByKey(info, httpapi.SetupKeyMetaApp).Status != "ready" {
		t.Fatalf("meta_app status = %q, want ready", entryByKey(info, httpapi.SetupKeyMetaApp).Status)
	}
	got, err := h.creds.Get(context.Background(), "meta.app_id")
	if err != nil || got != "new-id" {
		t.Fatalf("meta.app_id after save = (%q, %v)", got, err)
	}
}

func TestSaveMetaAppInvalidCredentialNeverSavedEvenWithForce(t *testing.T) {
	h := newMetaHarness(t)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token","type":"OAuthException","code":190}}`))
	}))
	defer mock.Close()
	h.setMetaBaseURL(mock.URL)

	resp, env := h.putJSON("/xchats/api/v1/channel-setup/meta-app", map[string]any{"app_id": "bad", "app_secret": "bad", "force": true})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 even with force:true", resp.StatusCode)
	}
	if string(env["errcode"]) != `"CREDENTIAL_INVALID"` {
		t.Fatalf("errcode = %s", env["errcode"])
	}
	if _, err := h.creds.Get(context.Background(), "meta.app_id"); err == nil {
		t.Fatal("an invalid credential must never be saved, force or not")
	}
}

func TestSaveMetaAppUnverifiableRequiresForceThenSaves(t *testing.T) {
	h := newMetaHarness(t)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`not a Graph API error envelope at all`))
	}))
	defer mock.Close()
	h.setMetaBaseURL(mock.URL)

	resp, env := h.putJSON("/xchats/api/v1/channel-setup/meta-app", map[string]any{"app_id": "id1", "app_secret": "secret1"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	if string(env["errcode"]) != `"CREDENTIAL_UNVERIFIED"` {
		t.Fatalf("errcode = %s", env["errcode"])
	}
	if _, err := h.creds.Get(context.Background(), "meta.app_id"); err == nil {
		t.Fatal("must not be saved before force:true")
	}

	resp2, env2 := h.putJSON("/xchats/api/v1/channel-setup/meta-app", map[string]any{"app_id": "id1", "app_secret": "secret1", "force": true})
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("force save status = %d, body = %s", resp2.StatusCode, env2["message"])
	}
	got, err := h.creds.Get(context.Background(), "meta.app_id")
	if err != nil || got != "id1" {
		t.Fatalf("meta.app_id after force save = (%q, %v)", got, err)
	}
}

// --- PUT /channel-setup/instagram-app ---------------------------------------

func TestSaveInstagramAppRequiresAdmin(t *testing.T) {
	h := newMetaHarness(t)
	h.createMember(t, memberEmail, memberPass, "Member")
	member := h.loginAs(t, memberEmail, memberPass)
	resp := requestAs(t, member, http.MethodPut, h.srv.URL+"/xchats/api/v1/channel-setup/instagram-app",
		map[string]any{"app_id": "x", "app_secret": "y"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestSaveInstagramAppMissingFieldIs400(t *testing.T) {
	h := newMetaHarness(t)
	resp, env := h.putJSON("/xchats/api/v1/channel-setup/instagram-app", map[string]any{"app_id": "x"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
}

// TestSaveInstagramAppSavesImmediatelyWithoutValidation proves there is no
// `force` path and no Validate call for this pair — see
// handleSaveInstagramApp's own doc comment on why none exists.
func TestSaveInstagramAppSavesImmediatelyWithoutValidation(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("meta-id", "meta-secret") // messenger/whatsapp_cloud already ready; instagram is the one entry left
	resp, env := h.putJSON("/xchats/api/v1/channel-setup/instagram-app", map[string]any{"app_id": "ig-id", "app_secret": "ig-secret"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	if strings.Contains(string(env["payload"]), "ig-secret") {
		t.Fatal("the saved secret must never be echoed back")
	}
	var info httpapi.ChannelSetupInfo
	if err := json.Unmarshal(env["payload"], &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if entryByKey(info, httpapi.SetupKeyInstagram).Status != "ready" {
		t.Fatalf("instagram status = %q, want ready", entryByKey(info, httpapi.SetupKeyInstagram).Status)
	}
	got, err := h.creds.Get(context.Background(), credentials.KeyInstagramAppID)
	if err != nil || got != "ig-id" {
		t.Fatalf("instagram.app_id after save = (%q, %v)", got, err)
	}
}
