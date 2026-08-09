package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// TestMetaSetupWorksWithoutAppCredentialsConfigured is the checklist's whole
// reason to exist: it must be usable BEFORE an operator has saved a Meta App
// ID/Secret (that is precisely the setup step it walks them through), unlike
// every other Meta handler which fails METAAPP_NOT_CONFIGURED.
func TestMetaSetupWorksWithoutAppCredentialsConfigured(t *testing.T) {
	h := newMetaHarness(t)
	resp, env := h.getJSON("/xchats/api/v1/settings/meta-setup")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, env["message"])
	}
	var info httpapi.MetaSetupInfo
	if err := json.Unmarshal(env["payload"], &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.PublicBaseURL != "https://xchats.test" {
		t.Fatalf("public_base_url = %q, want https://xchats.test", info.PublicBaseURL)
	}
	if info.VerifyToken == "" {
		t.Fatal("verify_token must not be empty")
	}
	// A nil Go slice and an empty one decode identically, so this is checked
	// against the RAW wire bytes: encoding/json renders a nil []T as the
	// JSON literal `null`, and the frontend checklist reads
	// stale_accounts.length unconditionally — a `null` here is a hard
	// runtime crash there, not just an empty list.
	if strings.Contains(string(env["payload"]), `"stale_accounts":null`) {
		t.Fatal("stale_accounts must serialize as [] when empty, not null (see metaStaleAccounts' own doc comment)")
	}
	if len(info.Channels) != 3 {
		t.Fatalf("channels = %d, want 3", len(info.Channels))
	}

	byChannel := map[string]httpapi.MetaChannelSetup{}
	for _, c := range info.Channels {
		byChannel[c.Channel] = c
	}
	ig, ok := byChannel["instagram"]
	if !ok {
		t.Fatal("no instagram entry")
	}
	if ig.WebhookCallback != "https://xchats.test/meta/api/v1/webhook/instagram" {
		t.Fatalf("instagram webhook_callback = %q", ig.WebhookCallback)
	}
	if ig.RedirectURI != "https://xchats.test/meta/api/v1/oauth/instagram/callback" {
		t.Fatalf("instagram redirect_uri = %q", ig.RedirectURI)
	}
	if !ig.DevelopmentNotice {
		t.Fatal("instagram must carry the Development Mode notice")
	}

	fb, ok := byChannel["messenger"]
	if !ok {
		t.Fatal("no messenger entry")
	}
	if fb.WebhookCallback != "https://xchats.test/meta/api/v1/webhook/messenger" {
		t.Fatalf("messenger webhook_callback = %q", fb.WebhookCallback)
	}
	if fb.RedirectURI != "https://xchats.test/meta/api/v1/oauth/messenger/callback" {
		t.Fatalf("messenger redirect_uri = %q", fb.RedirectURI)
	}

	wa, ok := byChannel["whatsapp_cloud"]
	if !ok {
		t.Fatal("no whatsapp_cloud entry")
	}
	if wa.WebhookCallback != "https://xchats.test/meta/api/v1/webhook/whatsapp" {
		t.Fatalf("whatsapp_cloud webhook_callback = %q", wa.WebhookCallback)
	}
	if wa.RedirectURI != "" {
		t.Fatalf("whatsapp_cloud redirect_uri = %q, want empty (manual connect, no OAuth redirect)", wa.RedirectURI)
	}
	if wa.DevelopmentNotice {
		t.Fatal("whatsapp_cloud needs no App Review in Development Mode — must not carry the notice")
	}
}

func TestMetaSetupListsOnlyStaleAccounts(t *testing.T) {
	h := newMetaHarness(t)
	ctx := context.Background()

	stale, err := h.store.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID: config.ChannelAccountID(config.InstagramOwnerRef("ig-stale")), OrganizationID: h.orgID,
		Channel: "instagram", ExternalAccountID: "ig-stale", DisplayName: "Stale Shop",
	})
	if err != nil {
		t.Fatalf("claim stale: %v", err)
	}
	if err := h.store.SetChannelWebhookState(ctx, stale.ID, store.ChannelWebhookState{
		State: "webhook_stale", URL: "https://old.example.com/meta/api/v1/webhook/instagram",
		LastError: "публичный адрес изменился — обновите webhook в Meta Dashboard", Checked: true,
	}); err != nil {
		t.Fatalf("set stale state: %v", err)
	}

	healthy, err := h.store.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID: config.ChannelAccountID(config.InstagramOwnerRef("ig-healthy")), OrganizationID: h.orgID,
		Channel: "instagram", ExternalAccountID: "ig-healthy", DisplayName: "Healthy Shop",
	})
	if err != nil {
		t.Fatalf("claim healthy: %v", err)
	}
	if err := h.store.SetChannelWebhookState(ctx, healthy.ID, store.ChannelWebhookState{
		State: "connected", URL: "https://xchats.test/meta/api/v1/webhook/instagram", Registered: true, Checked: true,
	}); err != nil {
		t.Fatalf("set healthy state: %v", err)
	}

	_, env := h.getJSON("/xchats/api/v1/settings/meta-setup")
	var info httpapi.MetaSetupInfo
	if err := json.Unmarshal(env["payload"], &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(info.StaleAccounts) != 1 {
		t.Fatalf("stale_accounts = %+v, want exactly 1", info.StaleAccounts)
	}
	if info.StaleAccounts[0].AccountID != stale.ID.String() {
		t.Fatalf("stale account id = %q, want %q", info.StaleAccounts[0].AccountID, stale.ID.String())
	}
	if info.StaleAccounts[0].Detail == "" {
		t.Fatal("stale account detail (webhook_last_error) must not be empty")
	}
}
