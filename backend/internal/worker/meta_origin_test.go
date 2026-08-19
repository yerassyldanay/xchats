package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/internal/whatsappcloud"
)

// seedOriginTestAccount claims a channel account already registered with an
// OLD webhook_url (and, for whatsapp_cloud, a WABA id in provider_meta and a
// business token) — the fixture shape RepairStaleMetaOrigins compares
// against a "current" origin that has since moved on.
func seedOriginTestAccount(t *testing.T, st *store.Store, channel, externalID, wabaID, oldWebhookURL, token string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	box, err := secretbox.FromEnvValue(metaMediaTestEncKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	st.UseCredentialsBox(box)
	org, err := st.SeedOrganization(ctx, "worker-origin-test-"+externalID)
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	var ownerRef string
	switch channel {
	case "whatsapp_cloud":
		ownerRef = config.WhatsAppCloudOwnerRef(externalID)
	case "instagram":
		ownerRef = config.InstagramOwnerRef(externalID)
	case "messenger":
		ownerRef = config.MessengerOwnerRef(externalID)
	}
	var providerMeta []byte
	if wabaID != "" {
		providerMeta, _ = json.Marshal(map[string]string{"waba_id": wabaID})
	}
	acct, err := st.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID: config.ChannelAccountID(ownerRef), OrganizationID: org.ID,
		Channel: channel, ExternalAccountID: externalID, ProviderMeta: providerMeta,
	})
	if err != nil {
		t.Fatalf("claim channel account: %v", err)
	}
	if err := st.SetChannelWebhookState(ctx, acct.ID, store.ChannelWebhookState{
		State: "connected", URL: oldWebhookURL, Registered: true, Checked: true,
	}); err != nil {
		t.Fatalf("set webhook state: %v", err)
	}
	if token != "" {
		if err := st.SetChannelCredentials(ctx, store.ChannelCredentialsWrite{
			AccountID: acct.ID, Secret: token, TokenKind: "business_token",
		}); err != nil {
			t.Fatalf("set credentials: %v", err)
		}
	}
	return acct.ID
}

func TestRepairStaleMetaOriginsNoopsWithEmptyPublicBaseURL(t *testing.T) {
	st := dbtest.New(t)
	w := &Worker{Store: st, Hub: realtime.NewHub(), Log: testLogger()}
	repaired, stale, err := w.RepairStaleMetaOrigins(context.Background(), "", "vt")
	if err != nil || repaired != 0 || stale != 0 {
		t.Fatalf("RepairStaleMetaOrigins with empty base URL = (%d, %d, %v), want (0, 0, nil)", repaired, stale, err)
	}
}

func TestRepairStaleMetaOriginsRepushesWhatsAppCloudOverride(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	acctID := seedOriginTestAccount(t, st, "whatsapp_cloud", "pn-stale-1", "waba-1",
		"https://old.example.com/meta/api/v1/webhook/whatsapp", "biz-token")

	var gotBody map[string]any
	var gotAuth string
	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer graphSrv.Close()

	waCloud := whatsappcloud.NewClient(meta.NewHTTPWithHosts("v21.0", graphSrv.URL, "", "", testLogger()))
	w := &Worker{Store: st, WACloud: waCloud, Hub: realtime.NewHub(), Log: testLogger()}

	repaired, stale, err := w.RepairStaleMetaOrigins(ctx, "https://new.example.com", "verify-tok")
	if err != nil {
		t.Fatalf("RepairStaleMetaOrigins: %v", err)
	}
	if repaired != 1 || stale != 0 {
		t.Fatalf("(repaired, stale) = (%d, %d), want (1, 0)", repaired, stale)
	}
	if gotAuth != "Bearer biz-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotBody["override_callback_uri"] != "https://new.example.com/meta/api/v1/webhook/whatsapp" {
		t.Fatalf("override_callback_uri = %v", gotBody["override_callback_uri"])
	}
	if gotBody["verify_token"] != "verify-tok" {
		t.Fatalf("verify_token = %v", gotBody["verify_token"])
	}

	acct, err := st.ChannelAccountByID(ctx, acctID)
	if err != nil {
		t.Fatalf("ChannelAccountByID: %v", err)
	}
	if acct.WebhookURL != "https://new.example.com/meta/api/v1/webhook/whatsapp" {
		t.Fatalf("webhook_url after repair = %q", acct.WebhookURL)
	}
	if acct.ConnectionState != "connected" {
		t.Fatalf("connection_state after repair = %q, want connected", acct.ConnectionState)
	}
}

func TestRepairStaleMetaOriginsSkipsWhatsAppCloudAlreadyCurrent(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	seedOriginTestAccount(t, st, "whatsapp_cloud", "pn-current-1", "waba-2",
		"https://new.example.com/meta/api/v1/webhook/whatsapp", "biz-token")

	called := false
	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer graphSrv.Close()

	waCloud := whatsappcloud.NewClient(meta.NewHTTPWithHosts("v21.0", graphSrv.URL, "", "", testLogger()))
	w := &Worker{Store: st, WACloud: waCloud, Hub: realtime.NewHub(), Log: testLogger()}

	repaired, _, err := w.RepairStaleMetaOrigins(ctx, "https://new.example.com", "verify-tok")
	if err != nil {
		t.Fatalf("RepairStaleMetaOrigins: %v", err)
	}
	if repaired != 0 {
		t.Fatalf("repaired = %d, want 0 (already current)", repaired)
	}
	if called {
		t.Fatal("SetOverride must not be called for an account whose origin already matches")
	}
}

func TestRepairStaleMetaOriginsWhatsAppCloudFailureRecordsError(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	acctID := seedOriginTestAccount(t, st, "whatsapp_cloud", "pn-fail-1", "waba-3",
		"https://old.example.com/meta/api/v1/webhook/whatsapp", "biz-token")

	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid token","code":190}}`))
	}))
	defer graphSrv.Close()

	waCloud := whatsappcloud.NewClient(meta.NewHTTPWithHosts("v21.0", graphSrv.URL, "", "", testLogger()))
	w := &Worker{Store: st, WACloud: waCloud, Hub: realtime.NewHub(), Log: testLogger()}

	repaired, _, err := w.RepairStaleMetaOrigins(ctx, "https://new.example.com", "verify-tok")
	if err != nil {
		t.Fatalf("RepairStaleMetaOrigins: %v", err)
	}
	if repaired != 0 {
		t.Fatalf("repaired = %d, want 0 (SetOverride failed)", repaired)
	}

	acct, err := st.ChannelAccountByID(ctx, acctID)
	if err != nil {
		t.Fatalf("ChannelAccountByID: %v", err)
	}
	if acct.WebhookURL != "https://old.example.com/meta/api/v1/webhook/whatsapp" {
		t.Fatalf("webhook_url must be left at its OLD (still actually registered) value, got %q", acct.WebhookURL)
	}
	if acct.ConnectionState != "webhook_error" {
		t.Fatalf("connection_state = %q, want webhook_error", acct.ConnectionState)
	}
	if acct.WebhookLastError == "" {
		t.Fatal("webhook_last_error must explain the failed auto-repush")
	}
}

func TestRepairStaleMetaOriginsMarksInstagramAndMessengerStale(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	igID := seedOriginTestAccount(t, st, "instagram", "ig-stale-1", "",
		"https://old.example.com/meta/api/v1/webhook/instagram", "")
	fbID := seedOriginTestAccount(t, st, "messenger", "page-stale-1", "",
		"https://old.example.com/meta/api/v1/webhook/messenger", "")

	w := &Worker{Store: st, Hub: realtime.NewHub(), Log: testLogger()}
	repaired, stale, err := w.RepairStaleMetaOrigins(ctx, "https://new.example.com", "verify-tok")
	if err != nil {
		t.Fatalf("RepairStaleMetaOrigins: %v", err)
	}
	if repaired != 0 || stale != 2 {
		t.Fatalf("(repaired, stale) = (%d, %d), want (0, 2)", repaired, stale)
	}

	ig, err := st.ChannelAccountByID(ctx, igID)
	if err != nil || ig.ConnectionState != "webhook_stale" {
		t.Fatalf("instagram connection_state = %q (err %v), want webhook_stale", ig.ConnectionState, err)
	}
	if ig.WebhookURL != "https://old.example.com/meta/api/v1/webhook/instagram" {
		t.Fatalf("instagram webhook_url must be left at its OLD (still actually registered at Meta) value, got %q", ig.WebhookURL)
	}
	if ig.WebhookLastError == "" {
		t.Fatal("instagram webhook_last_error must explain what changed")
	}

	fb, err := st.ChannelAccountByID(ctx, fbID)
	if err != nil || fb.ConnectionState != "webhook_stale" {
		t.Fatalf("messenger connection_state = %q (err %v), want webhook_stale", fb.ConnectionState, err)
	}
}

func TestRepairStaleMetaOriginsSkipsInstagramAlreadyCurrent(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	igID := seedOriginTestAccount(t, st, "instagram", "ig-current-1", "",
		"https://new.example.com/meta/api/v1/webhook/instagram", "")

	w := &Worker{Store: st, Hub: realtime.NewHub(), Log: testLogger()}
	_, stale, err := w.RepairStaleMetaOrigins(ctx, "https://new.example.com", "verify-tok")
	if err != nil {
		t.Fatalf("RepairStaleMetaOrigins: %v", err)
	}
	if stale != 0 {
		t.Fatalf("stale = %d, want 0 (already current)", stale)
	}
	ig, err := st.ChannelAccountByID(ctx, igID)
	if err != nil || ig.ConnectionState != "connected" {
		t.Fatalf("connection_state = %q (err %v), want unchanged connected", ig.ConnectionState, err)
	}
}
