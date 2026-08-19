package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/dbtest"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/meta"
	"github.com/yerassyldanay/xchats/backend/internal/realtime"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// seedInstagramCredential claims an instagram account with a stored token
// whose expiry is `expiresIn` from now — the shape
// ChannelCredentialsExpiringSoon filters the "before" side on. Callers that
// also need to control the "at least minAge since the last refresh" side
// follow up with backdateRefreshedAt.
func seedInstagramCredential(t *testing.T, st *store.Store, igUserID, token string, expiresIn time.Duration) store.ChannelAccount {
	t.Helper()
	ctx := context.Background()
	box, err := secretbox.FromEnvValue(metaMediaTestEncKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	st.UseCredentialsBox(box)
	org, err := st.SeedOrganization(ctx, "worker-instagram-token-test-"+igUserID)
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	acct, err := st.ClaimChannelAccount(ctx, store.ChannelAccountClaim{
		ID: config.ChannelAccountID(config.InstagramOwnerRef(igUserID)), OrganizationID: org.ID,
		Channel: "instagram", ExternalAccountID: igUserID,
	})
	if err != nil {
		t.Fatalf("claim channel account: %v", err)
	}
	expiresAt := time.Now().Add(expiresIn)
	if err := st.SetChannelCredentials(ctx, store.ChannelCredentialsWrite{
		AccountID: acct.ID, Secret: token, TokenKind: "long_lived_user_token", ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatalf("set channel credentials: %v", err)
	}
	return acct
}

// backdateRefreshedAt pushes a credential's refreshed_at into the past —
// SetChannelCredentials always stamps "now", so a fresh fixture needs this
// to exercise ChannelCredentialsExpiringSoon's own minAge-since-last-refresh
// filter (Meta refuses to refresh a token less than 24h old).
func backdateRefreshedAt(t *testing.T, db *dbx.DB, accountID string, ago time.Duration) {
	t.Helper()
	if _, err := db.Exec(context.Background(),
		`UPDATE channel_credentials SET refreshed_at = $2 WHERE account_id = $1`,
		accountID, time.Now().Add(-ago)); err != nil {
		t.Fatalf("backdate refreshed_at: %v", err)
	}
}

func TestRefreshExpiringInstagramTokensRefreshesAndPersists(t *testing.T) {
	ctx := context.Background()
	st, db := dbtest.Open(t)
	acct := seedInstagramCredential(t, st, "178414000020", "old-token", 3*24*time.Hour)
	backdateRefreshedAt(t, db, acct.ID.String(), 48*time.Hour)

	var gotAuth, gotPath string
	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.URL.Query().Get("access_token"), r.URL.Path
		_, _ = w.Write([]byte(`{"access_token":"new-token","token_type":"bearer","expires_in":5184000}`))
	}))
	defer graphSrv.Close()

	metaClient := meta.NewHTTPWithHosts("v21.0", "", graphSrv.URL, "", testLogger())
	w := &Worker{Store: st, MetaClient: metaClient, Hub: realtime.NewHub(), Log: testLogger()}

	n, err := w.RefreshExpiringInstagramTokens(ctx, 7*24*time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("RefreshExpiringInstagramTokens: %v", err)
	}
	if n != 1 {
		t.Fatalf("refreshed = %d, want 1", n)
	}
	if gotAuth != "old-token" {
		t.Fatalf("refresh call used access_token=%q, want the OLD token", gotAuth)
	}
	if gotPath != "/refresh_access_token" {
		t.Fatalf("path = %q", gotPath)
	}
	secret, err := st.ChannelCredentialsSecret(ctx, acct.ID)
	if err != nil || secret != "new-token" {
		t.Fatalf("ChannelCredentialsSecret = (%q, %v), want (new-token, nil)", secret, err)
	}
}

func TestRefreshExpiringInstagramTokensSkipsTokensNotYetDue(t *testing.T) {
	ctx := context.Background()
	st := dbtest.New(t)
	// Expires in 30 days — well outside the 7-day lookahead window.
	seedInstagramCredential(t, st, "178414000021", "fresh-token", 30*24*time.Hour)

	w := &Worker{Store: st, MetaClient: meta.NewHTTP("v21.0", testLogger()), Hub: realtime.NewHub(), Log: testLogger()}
	n, err := w.RefreshExpiringInstagramTokens(ctx, 7*24*time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("RefreshExpiringInstagramTokens: %v", err)
	}
	if n != 0 {
		t.Fatalf("refreshed = %d, want 0 (nothing due yet)", n)
	}
}

func TestRefreshExpiringInstagramTokensRecordsFailureWithoutBlockingOthers(t *testing.T) {
	ctx := context.Background()
	st, db := dbtest.Open(t)
	failing := seedInstagramCredential(t, st, "178414000022", "tok-fail", 3*24*time.Hour)
	backdateRefreshedAt(t, db, failing.ID.String(), 48*time.Hour)
	ok := seedInstagramCredential(t, st, "178414000023", "tok-ok", 3*24*time.Hour)
	backdateRefreshedAt(t, db, ok.ID.String(), 48*time.Hour)

	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("access_token") == "tok-fail" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid token","type":"OAuthException","code":190,"error_subcode":463}}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"new-tok-ok","token_type":"bearer","expires_in":5184000}`))
	}))
	defer graphSrv.Close()

	metaClient := meta.NewHTTPWithHosts("v21.0", "", graphSrv.URL, "", testLogger())
	w := &Worker{Store: st, MetaClient: metaClient, Hub: realtime.NewHub(), Log: testLogger()}

	n, err := w.RefreshExpiringInstagramTokens(ctx, 7*24*time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("RefreshExpiringInstagramTokens: %v", err)
	}
	if n != 1 {
		t.Fatalf("refreshed = %d, want 1 (only the healthy account)", n)
	}
	if secret, _ := st.ChannelCredentialsSecret(ctx, failing.ID); secret != "tok-fail" {
		t.Fatalf("the failing account's OLD token must be left in place, got %q", secret)
	}
	if secret, _ := st.ChannelCredentialsSecret(ctx, ok.ID); secret != "new-tok-ok" {
		t.Fatalf("the healthy account's token = %q, want new-tok-ok", secret)
	}

	failingAcct, err := st.ChannelAccountByID(ctx, failing.ID)
	if err != nil || failingAcct.ConnectionState != "token_expiring" {
		t.Fatalf("failing account connection_state = %q (err %v), want token_expiring", failingAcct.ConnectionState, err)
	}
	okAcct, err := st.ChannelAccountByID(ctx, ok.ID)
	if err != nil || okAcct.ConnectionState != "connected" {
		t.Fatalf("healthy account connection_state = %q (err %v), want connected", okAcct.ConnectionState, err)
	}
}

func TestRefreshExpiringInstagramTokensNoopsWithoutAMetaClient(t *testing.T) {
	st := dbtest.New(t)
	w := &Worker{Store: st, Hub: realtime.NewHub(), Log: testLogger()}
	n, err := w.RefreshExpiringInstagramTokens(context.Background(), 7*24*time.Hour, 24*time.Hour)
	if err != nil || n != 0 {
		t.Fatalf("RefreshExpiringInstagramTokens with no MetaClient = (%d, %v), want (0, nil)", n, err)
	}
}
