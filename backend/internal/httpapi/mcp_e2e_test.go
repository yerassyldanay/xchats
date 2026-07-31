package httpapi_test

// mcp_e2e_test.go implements Task 19: one flow exercising the MCP OAuth
// layer and the /mcp resource layer TOGETHER, end to end, rather than as the
// fragmented per-concern tests the rest of mcp_integration_test.go already
// covers. It also fills three gaps those fragmented tests leave:
//   - a token exchange that supplies the CORRECT resource and succeeds (the
//     existing resource test only ever proves a WRONG resource is rejected —
//     fullAuthCodeFlow's helper never sends a resource param at all, so the
//     positive/matching path was never actually exercised end to end);
//   - POST /oauth/revoke exercised over real HTTP (mcpauth_test.go covers
//     Authorizer.Revoke in-process; nothing here drove it through the route);
//   - the explicit invariant plan/mcp.md's Task 19 calls out by name: the
//     xchats session cookie authenticates ONLY the browser-facing
//     authorization/review pages, never /mcp itself.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
)

func TestMCPAuthFlow_EndToEnd(t *testing.T) {
	h := newMCPHarness(t)
	clientID := h.registerClient(t, mcpRedirectURI)
	verifier, challenge := pkcePair()
	state := "e2e-full-flow-state"

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// 1. No session cookie yet: the authorize page must show the login form.
	q := authorizeQuery(clientID, mcpRedirectURI, challenge, state, "")
	resp, err := client.Get(h.srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("authorize (logged out): %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("auth/login")) {
		t.Fatalf("expected login page before auth, got status=%d body=%s", resp.StatusCode, body)
	}

	// 2. Authenticate with the Xchats session cookie.
	h.login(t, client)

	// 3. Authorize again: now the consent page, naming the client and
	// offering the organization picker.
	resp, err = client.Get(h.srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("authorize (logged in): %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("Test MCP Client")) {
		t.Fatalf("expected consent page naming the client, got status=%d body=%s", resp.StatusCode, body)
	}
	csrfToken := extractHiddenField(body, "csrf_token")
	if csrfToken == "" {
		t.Fatalf("no csrf_token in consent page: %s", body)
	}

	// 4. Submit consent, selecting an organization, with CSRF protection.
	noRedir := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	decResp := h.decide(t, noRedir, clientID, mcpRedirectURI, challenge, state, "", "approve", h.orgID.String(), csrfToken)
	defer decResp.Body.Close()
	if decResp.StatusCode != http.StatusFound {
		b, _ := io.ReadAll(decResp.Body)
		t.Fatalf("decision status=%d body=%s", decResp.StatusCode, b)
	}
	loc, err := url.Parse(decResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("bad Location: %v", err)
	}
	if got := loc.Query().Get("state"); got != state {
		t.Fatalf("state mismatch: got %q want %q", got, state)
	}
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatalf("no code in redirect Location=%s", loc)
	}

	// 5. Exchange the code with the correct PKCE verifier AND the correct
	// resource (the resource-mismatch test elsewhere only ever proves a WRONG
	// resource is rejected; this is the matching/positive path).
	form := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {clientID},
		"redirect_uri": {mcpRedirectURI}, "code": {code}, "code_verifier": {verifier},
		"resource": {h.cfg.MCPResourceURL()},
	}
	tokResp, err := http.PostForm(h.srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	tb, _ := io.ReadAll(tokResp.Body)
	tokResp.Body.Close()
	if tokResp.StatusCode != http.StatusOK {
		t.Fatalf("token status=%d body=%s", tokResp.StatusCode, tb)
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(tb, &tok); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Fatalf("incomplete token response: %s", tb)
	}

	// 6. Call /mcp with the access token.
	if r := h.rpc(t, tok.AccessToken, "ping", nil); r.Error != nil {
		t.Fatalf("ping: %+v", r.Error)
	}

	// 7. Refresh the token.
	refreshForm := url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "refresh_token": {tok.RefreshToken}}
	refResp, err := http.PostForm(h.srv.URL+"/oauth/token", refreshForm)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	rb, _ := io.ReadAll(refResp.Body)
	refResp.Body.Close()
	if refResp.StatusCode != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", refResp.StatusCode, rb)
	}
	var refreshed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(rb, &refreshed); err != nil {
		t.Fatalf("decode refresh: %v", err)
	}
	if refreshed.RefreshToken == "" || refreshed.RefreshToken == tok.RefreshToken {
		t.Fatalf("expected a NEW rotated refresh token, got %q", refreshed.RefreshToken)
	}
	if r := h.rpc(t, refreshed.AccessToken, "ping", nil); r.Error != nil {
		t.Fatalf("ping with refreshed token: %+v", r.Error)
	}

	// 8. The rotated-out refresh token must be rejected.
	reuse, err := http.PostForm(h.srv.URL+"/oauth/token", refreshForm)
	if err != nil {
		t.Fatalf("reuse refresh: %v", err)
	}
	reuse.Body.Close()
	if reuse.StatusCode == http.StatusOK {
		t.Fatalf("a rotated-away refresh token must be rejected, got 200")
	}

	// 9. Revoke the connection (the current access token) and verify the
	// documented behavior: /oauth/revoke always 200s, and the token stops
	// working immediately afterward.
	revokeResp, err := http.PostForm(h.srv.URL+"/oauth/revoke", url.Values{"token": {refreshed.AccessToken}})
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	revokeResp.Body.Close()
	if revokeResp.StatusCode != http.StatusOK {
		t.Fatalf("revoke status=%d, want 200 per RFC 7009", revokeResp.StatusCode)
	}
	revokedReq, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))
	revokedReq.Header.Set("Content-Type", "application/json")
	revokedReq.Header.Set("Authorization", "Bearer "+refreshed.AccessToken)
	revokedMCPResp, err := http.DefaultClient.Do(revokedReq)
	if err != nil {
		t.Fatalf("post /mcp with revoked token: %v", err)
	}
	revokedMCPResp.Body.Close()
	if revokedMCPResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a revoked token, got %d", revokedMCPResp.StatusCode)
	}

	// 10. Invariant: the xchats session cookie authenticates ONLY the
	// browser-facing authorization/review pages — /mcp must reject a request
	// bearing the session cookie but no bearer token at all, exactly like an
	// entirely unauthenticated request. `client` still carries the session
	// cookie from step 2's login.
	cookieOnlyReq, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))
	cookieOnlyReq.Header.Set("Content-Type", "application/json")
	cookieOnlyResp, err := client.Do(cookieOnlyReq)
	if err != nil {
		t.Fatalf("post /mcp with session cookie only: %v", err)
	}
	cookieOnlyBody, _ := io.ReadAll(cookieOnlyResp.Body)
	cookieOnlyResp.Body.Close()
	if cookieOnlyResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a session cookie alone must never authenticate /mcp; got status=%d body=%s", cookieOnlyResp.StatusCode, cookieOnlyBody)
	}
}

// TestMCPAccessToken_ScopedToMintOrgRegardlessOfActiveSession is Task 19's
// "confirm the token cannot reach another organization" — sharpened by Task
// 15's introduction of a session-scoped active organization, which is the
// only way this ambiguity could arise in the first place: a user in two
// orgs authorizes the connector for org B, then separately switches their
// BROWSER SESSION's active org to org A (the frontend selector / a review
// handoff). The access token's organization binding must come ONLY from the
// token's own claims (set at consent time), never from whatever the browser
// session's active_organization_id happens to be at call time — those are
// deliberately independent concepts (authenticateMCPRequest in mcp.go never
// reads the session at all). Proven here by writing through the token and
// checking directly which organization's draft actually received it.
func TestMCPAccessToken_ScopedToMintOrgRegardlessOfActiveSession(t *testing.T) {
	h := newMCPHarness(t)
	ctx := context.Background()

	orgB, err := h.store.SeedOrganization(ctx, "xchats-mcp-org-b")
	if err != nil {
		t.Fatalf("seed org B: %v", err)
	}
	if _, err := h.store.Pool().Exec(ctx, `
		INSERT INTO xchats.organization_users (organization_id, user_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`, orgB.ID, h.userID); err != nil {
		t.Fatalf("add membership to org B: %v", err)
	}

	clientID := h.registerClient(t, mcpRedirectURI)
	verifier, challenge := pkcePair()
	state := "cross-org-isolation-state"
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	h.login(t, client)

	q := authorizeQuery(clientID, mcpRedirectURI, challenge, state, "")
	resp, err := client.Get(h.srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	csrfToken := extractHiddenField(body, "csrf_token")
	if csrfToken == "" {
		t.Fatalf("no csrf_token in consent page: %s", body)
	}

	// Consent selects org B — this is the token's ONLY binding to an org.
	noRedir := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	decResp := h.decide(t, noRedir, clientID, mcpRedirectURI, challenge, state, "", "approve", orgB.ID.String(), csrfToken)
	loc, _ := url.Parse(decResp.Header.Get("Location"))
	decResp.Body.Close()
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatalf("no code in redirect Location=%s", loc)
	}

	form := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {clientID},
		"redirect_uri": {mcpRedirectURI}, "code": {code}, "code_verifier": {verifier},
	}
	tokResp, err := http.PostForm(h.srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	tb, _ := io.ReadAll(tokResp.Body)
	tokResp.Body.Close()
	if tokResp.StatusCode != http.StatusOK {
		t.Fatalf("token status=%d body=%s", tokResp.StatusCode, tb)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(tb, &tok); err != nil {
		t.Fatalf("decode token: %v", err)
	}

	// Now switch the BROWSER SESSION's active org to org A — the same user's
	// OTHER organization, unrelated to what the token above was minted for.
	setBody, _ := json.Marshal(map[string]any{"organization_id": h.orgID})
	setResp, err := client.Post(h.srv.URL+"/xchats/api/v1/organization/active", "application/json", bytes.NewReader(setBody))
	if err != nil {
		t.Fatalf("set active organization: %v", err)
	}
	setResp.Body.Close()
	if setResp.StatusCode != http.StatusOK {
		t.Fatalf("set active organization status=%d", setResp.StatusCode)
	}

	// Write through the MCP token — if org scoping ever started reading the
	// session instead of the token, this would land in org A.
	const title = "cross-org-isolation-check-topic"
	r := h.rpc(t, tok.AccessToken, "tools/call", map[string]any{
		"name":      "kb_topic_upsert",
		"arguments": map[string]any{"changes": map[string]any{"title": title, "body_md": "isolation check"}},
	})
	if r.Error != nil {
		t.Fatalf("kb_topic_upsert: %+v", r.Error)
	}
	if result, ok := r.Result.(map[string]any); ok && result["isError"] == true {
		t.Fatalf("kb_topic_upsert returned isError:true: %+v", result)
	}

	draftB, err := h.kb.Draft(ctx, orgB.ID)
	if err != nil {
		t.Fatalf("load org B draft: %v", err)
	}
	foundInB := false
	for _, topic := range draftB.Topics {
		if topic.Title == title {
			foundInB = true
		}
	}
	if !foundInB {
		t.Fatalf("expected the write to land in org B (the token's bound org), draft: %+v", draftB.Topics)
	}

	draftA, err := h.kb.Draft(ctx, h.orgID)
	if err != nil {
		t.Fatalf("load org A draft: %v", err)
	}
	for _, topic := range draftA.Topics {
		if topic.Title == title {
			t.Fatalf("write leaked into org A (the session's active org) — token scoping must be independent of session state")
		}
	}
}
