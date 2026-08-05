package httpapi_test

// mcp_review_handoff_test.go covers Task 15's organization-bound review
// handoff end to end: GET /playground/review-handoff redeeming a signed
// xchats-review token minted by mcpauth.ReviewHandoffSigner. It reuses
// mcpHarness/newMCPHarness from mcp_integration_test.go (same package) —
// this file only adds the handoff-specific helpers and test matrix the plan
// requires: valid, expired, tampered, removed-membership, wrong-user,
// replayed, and a two-organization user end to end.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
)

// browserClient returns an http.Client with a fresh cookie jar that stops at
// the first redirect (so a caller can inspect the Location header directly,
// exactly like fullAuthCodeFlow's noRedir client), already logged in as
// h.userID.
func (h *mcpHarness) browserClient(t *testing.T) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	h.login(t, client)
	return client
}

// addUserToOrg inserts a direct organization_users membership row. The store
// package has no public "add an existing user to a SECOND org" method —
// CreateUser only ever wires the one org it creates the user in — so this
// reaches the table directly, exactly mirroring CreateUser's own membership
// insert.
func (h *mcpHarness) addUserToOrg(t *testing.T, userID, orgID uuid.UUID) {
	t.Helper()
	_, err := h.db.Exec(context.Background(), `
		INSERT INTO organization_users (organization_id, user_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, orgID, userID)
	if err != nil {
		t.Fatalf("addUserToOrg: %v", err)
	}
}

// removeUserFromOrg simulates a membership revoked after a handoff token was
// minted but before it was redeemed — handleReviewHandoff must re-check
// CURRENT membership rather than trust the token's claim.
func (h *mcpHarness) removeUserFromOrg(t *testing.T, userID, orgID uuid.UUID) {
	t.Helper()
	_, err := h.db.Exec(context.Background(), `
		DELETE FROM organization_users WHERE user_id = $1 AND organization_id = $2`, userID, orgID)
	if err != nil {
		t.Fatalf("removeUserFromOrg: %v", err)
	}
}

// redeemReviewHandoff performs the widget-link GET; redirects are not
// followed by the caller's client (see browserClient), so both a successful
// 302 and a rendered error page can be inspected directly.
func (h *mcpHarness) redeemReviewHandoff(t *testing.T, client *http.Client, token string) *http.Response {
	t.Helper()
	resp, err := client.Get(h.srv.URL + "/playground/review-handoff?token=" + url.QueryEscape(token))
	if err != nil {
		t.Fatalf("review-handoff request: %v", err)
	}
	return resp
}

type meEnvelope struct {
	Payload struct {
		Organization struct {
			ID uuid.UUID `json:"id"`
		} `json:"organization"`
	} `json:"payload"`
}

// activeOrgID reads back GET /me's resolved organization — the observable
// effect of a handoff (or its absence) on the session, independent of
// whatever the redirect Location happens to say.
func (h *mcpHarness) activeOrgID(t *testing.T, client *http.Client) uuid.UUID {
	t.Helper()
	resp, err := client.Get(h.srv.URL + "/xchats/api/v1/me")
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me status=%d body=%s", resp.StatusCode, b)
	}
	var out meEnvelope
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode me response: %v body=%s", err, b)
	}
	return out.Payload.Organization.ID
}

// tamperToken flips one character of the signature segment so the payload
// is byte-identical (still parses) but no longer verifies — a forged/damaged
// token, as distinct from a genuinely expired one.
func tamperToken(t *testing.T, token string) string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(parts[2]) == 0 {
		t.Fatalf("malformed token to tamper with: %q", token)
	}
	sig := []byte(parts[2])
	if sig[0] == 'A' {
		sig[0] = 'B'
	} else {
		sig[0] = 'A'
	}
	return parts[0] + "." + parts[1] + "." + string(sig)
}

func TestReviewHandoff_ValidTokenSetsActiveOrgAndRedirects(t *testing.T) {
	h := newMCPHarness(t)
	client := h.browserClient(t)

	signer := mcpauth.NewReviewHandoffSigner(h.key, mcpAPIBase, 5*time.Minute)
	token, _, _, err := signer.Sign(h.userID, h.orgID)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	resp := h.redeemReviewHandoff(t, client, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, b)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("bad Location: %v", err)
	}
	if loc.Path != "/playground" {
		t.Fatalf("Location path=%q, want /playground", loc.Path)
	}

	if got := h.activeOrgID(t, client); got != h.orgID {
		t.Fatalf("active org=%s, want %s", got, h.orgID)
	}
}

func TestReviewHandoff_ExpiredTokenRejected(t *testing.T) {
	h := newMCPHarness(t)
	client := h.browserClient(t)

	signer := mcpauth.NewReviewHandoffSigner(h.key, mcpAPIBase, -1*time.Second)
	token, _, _, err := signer.Sign(h.userID, h.orgID)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	resp := h.redeemReviewHandoff(t, client, token)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400; body=%s", resp.StatusCode, b)
	}
	if !bytes.Contains(b, []byte("недействительна или истекла")) {
		t.Fatalf("body=%s, want expired/invalid message", b)
	}
}

func TestReviewHandoff_TamperedTokenRejected(t *testing.T) {
	h := newMCPHarness(t)
	client := h.browserClient(t)

	signer := mcpauth.NewReviewHandoffSigner(h.key, mcpAPIBase, 5*time.Minute)
	token, _, _, err := signer.Sign(h.userID, h.orgID)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	token = tamperToken(t, token)

	resp := h.redeemReviewHandoff(t, client, token)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400; body=%s", resp.StatusCode, b)
	}
	if !bytes.Contains(b, []byte("недействительна или истекла")) {
		t.Fatalf("body=%s, want invalid-token message", b)
	}
}

func TestReviewHandoff_RemovedMembershipRejected(t *testing.T) {
	h := newMCPHarness(t)
	client := h.browserClient(t)

	org2, err := h.store.SeedOrganization(context.Background(), "xchats-mcp-second")
	if err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	h.addUserToOrg(t, h.userID, org2.ID)

	signer := mcpauth.NewReviewHandoffSigner(h.key, mcpAPIBase, 5*time.Minute)
	token, _, _, err := signer.Sign(h.userID, org2.ID)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Membership held at mint time but is gone by redemption time.
	h.removeUserFromOrg(t, h.userID, org2.ID)

	resp := h.redeemReviewHandoff(t, client, token)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400; body=%s", resp.StatusCode, b)
	}
	if !bytes.Contains(b, []byte("больше не состоите")) {
		t.Fatalf("body=%s, want removed-membership message", b)
	}
}

func TestReviewHandoff_WrongUserRejected(t *testing.T) {
	h := newMCPHarness(t)
	client := h.browserClient(t) // logged in as h.userID

	hash, err := httpapi.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	other, err := h.store.SeedUser(context.Background(), h.orgID, "other-mcp-user@xchats.test", hash, "Other")
	if err != nil {
		t.Fatalf("seed other user: %v", err)
	}

	signer := mcpauth.NewReviewHandoffSigner(h.key, mcpAPIBase, 5*time.Minute)
	token, _, _, err := signer.Sign(other.ID, h.orgID) // minted for a DIFFERENT user
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	resp := h.redeemReviewHandoff(t, client, token)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400; body=%s", resp.StatusCode, b)
	}
	if !bytes.Contains(b, []byte("другого пользователя")) {
		t.Fatalf("body=%s, want wrong-user message", b)
	}
}

func TestReviewHandoff_ReplayedTokenRejected(t *testing.T) {
	h := newMCPHarness(t)
	client := h.browserClient(t)

	signer := mcpauth.NewReviewHandoffSigner(h.key, mcpAPIBase, 5*time.Minute)
	token, _, _, err := signer.Sign(h.userID, h.orgID)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	first := h.redeemReviewHandoff(t, client, token)
	first.Body.Close()
	if first.StatusCode != http.StatusFound {
		t.Fatalf("first redemption status=%d, want 302", first.StatusCode)
	}

	second := h.redeemReviewHandoff(t, client, token)
	defer second.Body.Close()
	b, _ := io.ReadAll(second.Body)
	if second.StatusCode != http.StatusBadRequest {
		t.Fatalf("replay status=%d, want 400; body=%s", second.StatusCode, b)
	}
	if !bytes.Contains(b, []byte("уже была использована")) {
		t.Fatalf("body=%s, want already-used message", b)
	}
}

func TestReviewHandoff_TwoOrganizationUserLandsOnHandoffOrg(t *testing.T) {
	h := newMCPHarness(t)
	client := h.browserClient(t)

	// Sanity: before any handoff, OrgForUser's single-org default applies —
	// otherwise a false negative below would prove nothing.
	if got := h.activeOrgID(t, client); got != h.orgID {
		t.Fatalf("active org before handoff=%s, want default org %s", got, h.orgID)
	}

	org2, err := h.store.SeedOrganization(context.Background(), "xchats-mcp-second")
	if err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	h.addUserToOrg(t, h.userID, org2.ID)

	signer := mcpauth.NewReviewHandoffSigner(h.key, mcpAPIBase, 5*time.Minute)
	token, _, _, err := signer.Sign(h.userID, org2.ID)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	resp := h.redeemReviewHandoff(t, client, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, b)
	}

	if got := h.activeOrgID(t, client); got != org2.ID {
		t.Fatalf("active org after handoff=%s, want handoff org %s", got, org2.ID)
	}
}
