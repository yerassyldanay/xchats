package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

func TestDiscoverWhatsAppCloudNumbersReturnsListFromGraph(t *testing.T) {
	h := newMetaHarness(t)
	var gotPath string
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"data":[{"id":"pn1","display_phone_number":"+1 555 000 1111","verified_name":"Acme","quality_rating":"GREEN"}]}`))
	}

	resp, env := h.postJSON("/xchats/api/v1/whatsapp-cloud-accounts/discover", map[string]string{
		"waba_id": "waba-1", "business_token": "tok-1",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	if !strings.HasSuffix(gotPath, "/waba-1/phone_numbers") {
		t.Fatalf("graph path = %q", gotPath)
	}
	var payload struct {
		PhoneNumbers []struct {
			ID string `json:"id"`
		} `json:"phone_numbers"`
	}
	if err := json.Unmarshal(env["payload"], &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload.PhoneNumbers) != 1 || payload.PhoneNumbers[0].ID != "pn1" {
		t.Fatalf("phone_numbers = %+v", payload.PhoneNumbers)
	}
}

func TestDiscoverWhatsAppCloudNumbersRequiresFields(t *testing.T) {
	h := newMetaHarness(t)
	resp, env := h.postJSON("/xchats/api/v1/whatsapp-cloud-accounts/discover", map[string]string{"waba_id": "waba-1"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if string(env["errcode"]) != `"VALIDATION_ERROR"` {
		t.Fatalf("errcode = %s", env["errcode"])
	}
}

func TestDiscoverWhatsAppCloudNumbersSurfacesGraphError(t *testing.T) {
	h := newMetaHarness(t)
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token","type":"OAuthException","code":190,"error_subcode":463}}`))
	}
	resp, env := h.postJSON("/xchats/api/v1/whatsapp-cloud-accounts/discover", map[string]string{
		"waba_id": "waba-1", "business_token": "expired-tok",
	})
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	if string(env["errcode"]) != `"META_ERROR"` {
		t.Fatalf("errcode = %s", env["errcode"])
	}
	if !strings.Contains(string(env["message"]), "Invalid OAuth access token") {
		t.Fatalf("message = %s, want it to surface Meta's own error text", env["message"])
	}
}

func TestConnectWhatsAppCloudAccountRejectsBadPin(t *testing.T) {
	h := newMetaHarness(t)
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no Graph call should happen when the PIN itself is malformed")
	}
	resp, _ := h.postJSON("/xchats/api/v1/whatsapp-cloud-accounts", map[string]string{
		"waba_id": "waba-1", "business_token": "tok", "phone_number_id": "pn1", "pin": "123",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestConnectWhatsAppCloudAccountRequiresAppCredentials(t *testing.T) {
	h := newMetaHarness(t)
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no Graph call should happen before the App ID/Secret precondition is checked")
	}
	resp, env := h.postJSON("/xchats/api/v1/whatsapp-cloud-accounts", map[string]string{
		"waba_id": "waba-1", "business_token": "tok", "phone_number_id": "pn1", "pin": "123456",
	})
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("status = %d, want 412", resp.StatusCode)
	}
	if string(env["errcode"]) != `"META_APP_NOT_CONFIGURED"` {
		t.Fatalf("errcode = %s", env["errcode"])
	}
}

// registerAndOverrideOK is a graphHandler that succeeds both calls the
// connect flow makes after credential validation: POST .../register and
// POST .../subscribed_apps.
func registerAndOverrideOK(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/register"), strings.HasSuffix(r.URL.Path, "/subscribed_apps"):
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			t.Fatalf("unexpected graph call: %s %s", r.Method, r.URL.Path)
		}
	}
}

func TestConnectWhatsAppCloudAccountFullFlow(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	h.graphHandler = registerAndOverrideOK(t)

	resp, env := h.postJSON("/xchats/api/v1/whatsapp-cloud-accounts", map[string]string{
		"waba_id": "waba-full", "business_token": "tok-full", "phone_number_id": "pn-full",
		"pin": "654321", "display_name": "Acme Support", "display_phone_number": "+1 555 000 1111",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	var payload struct {
		Account struct {
			ID              string `json:"id"`
			Channel         string `json:"channel"`
			DisplayName     string `json:"display_name"`
			ExternalHandle  string `json:"external_handle"`
			ConnectionState string `json:"connection_state"`
		} `json:"account"`
	}
	if err := json.Unmarshal(env["payload"], &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Account.Channel != "whatsapp_cloud" || payload.Account.DisplayName != "Acme Support" {
		t.Fatalf("account = %+v", payload.Account)
	}
	if payload.Account.ExternalHandle != "+1 555 000 1111" {
		t.Fatalf("external_handle = %q, want the discover step's display_phone_number carried through", payload.Account.ExternalHandle)
	}
	if payload.Account.ConnectionState != "connected" {
		t.Fatalf("connection_state = %q, want connected (SetOverride succeeded)", payload.Account.ConnectionState)
	}

	// The account is now visible through the channel-neutral listing too.
	getResp, err := h.client.Get(h.srv.URL + "/xchats/api/v1/accounts")
	if err != nil {
		t.Fatalf("GET /accounts: %v", err)
	}
	defer getResp.Body.Close()
	var accountsEnv struct {
		Payload struct {
			Items []struct {
				ID      string `json:"id"`
				Channel string `json:"channel"`
			} `json:"items"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&accountsEnv); err != nil {
		t.Fatalf("decode /accounts: %v", err)
	}
	found := false
	for _, it := range accountsEnv.Payload.Items {
		if it.ID == payload.Account.ID && it.Channel == "whatsapp_cloud" {
			found = true
		}
	}
	if !found {
		t.Fatalf("GET /accounts items = %+v, want the just-connected account", accountsEnv.Payload.Items)
	}

	// The business token was sealed and is resolvable server-side (send-time
	// resolution — see whatsappcloud.ChannelSender).
	acctID := config.ChannelAccountID(config.WhatsAppCloudOwnerRef("pn-full"))
	secret, err := h.store.ChannelCredentialsSecret(context.Background(), acctID)
	if err != nil || secret != "tok-full" {
		t.Fatalf("ChannelCredentialsSecret = (%q, %v), want (tok-full, nil)", secret, err)
	}
}

func TestConnectWhatsAppCloudAccountRegisterFailureStoresNothing(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid PIN","type":"OAuthException","code":100}}`))
	}

	resp, env := h.postJSON("/xchats/api/v1/whatsapp-cloud-accounts", map[string]string{
		"waba_id": "waba-fail", "business_token": "tok-fail", "phone_number_id": "pn-fail", "pin": "000000",
	})
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	if _, err := h.store.ChannelAccountByExternalID(context.Background(), "whatsapp_cloud", "pn-fail"); err != store.ErrNotFound {
		t.Fatalf("ChannelAccountByExternalID err = %v, want ErrNotFound — a failed Register must leave no row behind", err)
	}
}

func TestConnectWhatsAppCloudAccountAlreadyClaimedConflicts(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")

	// A different organization already owns this phone number.
	otherOrg, err := h.store.SeedOrganization(context.Background(), "some-other-org")
	if err != nil {
		t.Fatalf("seed other org: %v", err)
	}
	if _, err := h.store.ClaimChannelAccount(context.Background(), store.ChannelAccountClaim{
		ID: config.ChannelAccountID(config.WhatsAppCloudOwnerRef("pn-taken")), OrganizationID: otherOrg.ID,
		Channel: "whatsapp_cloud", ExternalAccountID: "pn-taken",
	}); err != nil {
		t.Fatalf("seed claim: %v", err)
	}

	h.graphHandler = registerAndOverrideOK(t)
	resp, env := h.postJSON("/xchats/api/v1/whatsapp-cloud-accounts", map[string]string{
		"waba_id": "waba-taken", "business_token": "tok", "phone_number_id": "pn-taken", "pin": "111222",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
}

func TestDeleteWhatsAppCloudAccountClearsOverrideAndSoftDeletes(t *testing.T) {
	h := newMetaHarness(t)
	h.setAppCredentials("app-1", "app-secret-1")
	h.graphHandler = registerAndOverrideOK(t)

	_, env := h.postJSON("/xchats/api/v1/whatsapp-cloud-accounts", map[string]string{
		"waba_id": "waba-del", "business_token": "tok-del", "phone_number_id": "pn-del", "pin": "222333",
	})
	var payload struct {
		Account struct {
			ID string `json:"id"`
		} `json:"account"`
	}
	if err := json.Unmarshal(env["payload"], &payload); err != nil {
		t.Fatalf("decode connect payload: %v", err)
	}

	var clearedOverride bool
	h.graphHandler = func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/subscribed_apps") {
			clearedOverride = true
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}
	resp, _ := h.deleteJSON("/xchats/api/v1/whatsapp-cloud-accounts/" + payload.Account.ID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d", resp.StatusCode)
	}
	if !clearedOverride {
		t.Fatal("ClearOverride was never called")
	}

	acctID := config.ChannelAccountID(config.WhatsAppCloudOwnerRef("pn-del"))
	if _, err := h.store.ChannelAccountByID(context.Background(), acctID); err != store.ErrNotFound {
		t.Fatalf("ChannelAccountByID after delete = %v, want ErrNotFound (soft-deleted)", err)
	}
	if _, err := h.store.ChannelCredentialsSecret(context.Background(), acctID); err != store.ErrNotFound {
		t.Fatalf("ChannelCredentialsSecret after delete = %v, want ErrNotFound (credential row hard-deleted)", err)
	}
}

func TestDeleteWhatsAppCloudAccountUnknownIDNotFound(t *testing.T) {
	h := newMetaHarness(t)
	resp, env := h.deleteJSON("/xchats/api/v1/whatsapp-cloud-accounts/" + config.ChannelAccountID("nope").String())
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
}
