package httpapi_test

// mcp_integration_test.go covers the HTTP-transport layer of the MCP
// connector (plan/mcp.md) end to end: discovery endpoints, the OAuth 2.1 +
// PKCE authorize/consent/token flow through actual HTTP (not just the
// mcpauth package's in-process tests), the per-request tenant re-check, and
// a full kb_media_upload → PUT bytes → kb_*_upsert media-attach round trip.
// KB business logic itself (duplicate detection, concurrent draft writes,
// tool argument validation) is already exercised in internal/kbstore's and
// internal/mcpserver's own test suites — this file only fills the gap those
// leave: the wiring in server.go/mcp_oauth.go/mcp.go/mcp_upload.go.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/config"
	"github.com/yerassyldanay/xchats/backend/internal/httpapi"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/mcpserver"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

const (
	mcpAdminEmail  = "mcp-admin@xchats.test"
	mcpAdminPass   = "password123"
	mcpAPIBase     = "https://xchats.test" // fixed issuer/audience identity; this test never dials it
	mcpRedirectURI = "https://mcp-client.test/callback"
)

// mcpHarness holds one seeded org/user plus the running server; every test
// builds its own http.Client (fresh cookie jar) rather than sharing one off
// the harness, since some need a browser-like client that follows redirects
// and others need one that stops at the first to inspect the Location
// header — see decide()'s callers.
type mcpHarness struct {
	t      *testing.T
	srv    *httptest.Server
	cfg    *config.Config
	store  *store.Store
	blob   blob.Store
	kb     *kbstore.Store
	key    *mcpauth.SigningKey
	orgID  uuid.UUID
	userID uuid.UUID
}

func newMCPHarness(t *testing.T) *mcpHarness {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping MCP DB integration test")
	}
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, `DROP SCHEMA IF EXISTS xchats CASCADE; DROP TABLE IF EXISTS public.xchats_schema_migrations`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := store.RunMigrations(ctx, st.Pool(), migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{
		SessionTTLHours: 1, MinPasswordLen: 8, PageSize: 50, CORSOrigins: []string{"*"},
		APIBaseURL:               mcpAPIBase,
		MCPAccessTokenTTLSeconds: 900, MCPRefreshTokenTTLDays: 30, MCPAuthCodeTTLSeconds: 300,
		MCPUploadTokenTTLSeconds: 900, MCPReviewHandoffTTLSeconds: 300,
		FrontendBaseURL: "https://frontend.test",
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	org, err := st.SeedOrganization(ctx, "xchats-mcp")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	hash, err := httpapi.HashPassword(mcpAdminPass)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, mcpAdminEmail, hash, "Admin")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	kb := kbstore.New(st.Pool())
	blobStore, err := blob.NewDisk(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}

	key := mcpauth.NewEphemeralSigningKey()
	mcpStore := mcpauth.NewStore(st.Pool())
	authorizer := mcpauth.New(mcpStore, key, mcpauth.Config{
		Issuer: cfg.APIBaseURL, Audience: cfg.MCPResourceURL(),
		AccessTokenTTL:  time.Duration(cfg.MCPAccessTokenTTLSeconds) * time.Second,
		RefreshTokenTTL: time.Duration(cfg.MCPRefreshTokenTTLDays) * 24 * time.Hour,
		AuthCodeTTL:     time.Duration(cfg.MCPAuthCodeTTLSeconds) * time.Second,
	})
	uploadSigner := mcpauth.NewUploadTokenSigner(key)
	mcpSrv := mcpserver.New(mcpserver.Deps{
		KB: kb, Blob: blobStore, Log: log,
		UploadBaseURL: cfg.APIBaseURL, SignUpload: uploadSigner.Sign, UploadTTLSeconds: cfg.MCPUploadTokenTTLSeconds,
	})

	srv := httpapi.New(httpapi.Deps{
		Cfg: cfg, Store: st, Blob: blobStore, KB: kb,
		OrgID: org.ID, Log: log,
		MCPAuth: authorizer, MCPServer: mcpSrv,
	})
	ts := httptest.NewServer(srv.Router())
	h := &mcpHarness{
		t: t, srv: ts, cfg: cfg, store: st, blob: blobStore, kb: kb, key: key, orgID: org.ID, userID: user.ID,
	}
	t.Cleanup(func() { ts.Close(); st.Close() })
	return h
}

func (h *mcpHarness) login(t *testing.T, client *http.Client) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": mcpAdminEmail, "password": mcpAdminPass})
	resp, err := client.Post(h.srv.URL+"/xchats/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d", resp.StatusCode)
	}
}

// registerClient performs Dynamic Client Registration (RFC 7591) and returns
// the assigned client_id.
func (h *mcpHarness) registerClient(t *testing.T, redirectURIs ...string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"client_name": "Test MCP Client", "redirect_uris": redirectURIs})
	resp, err := http.Post(h.srv.URL+"/oauth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", resp.StatusCode, b)
	}
	var out struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if out.ClientID == "" {
		t.Fatalf("register returned empty client_id: %s", b)
	}
	return out.ClientID
}

func pkcePair() (verifier, challenge string) {
	verifier = "test-code-verifier-0123456789-abcdefghijklmnopqrstuvwxyz-ok"
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// authorizeQuery builds the GET /oauth/authorize query string shared by every
// step of one authorization attempt.
func authorizeQuery(clientID, redirectURI, challenge, state, scope string) url.Values {
	v := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	if scope != "" {
		v.Set("scope", scope)
	}
	return v
}

// hiddenFieldRe extracts one <input type="hidden" name="X" value="Y"> field's
// value from a rendered oauth_consent.html.tmpl page — the template always
// emits this exact attribute order (templates/oauth_consent.html.tmpl).
var hiddenFieldRe = regexp.MustCompile(`name="([a-z_]+)" value="([^"]*)"`)

// extractHiddenField reads one hidden form field's value out of a rendered
// consent page — the same information a real browser reads before
// submitting the decision form.
func extractHiddenField(html []byte, name string) string {
	for _, m := range hiddenFieldRe.FindAllSubmatch(html, -1) {
		if string(m[1]) == name {
			return string(m[2])
		}
	}
	return ""
}

// oauthCSRFTokenFor fetches a real consent-page render for a fresh, validly
// registered client and returns its embedded csrf_token — the token is
// bound only to client's session (mcp_oauth_csrf.go), not to which
// client_id/params the page happened to render for, so it's valid to reuse
// against a DIFFERENT client_id in a decide() call that is deliberately
// testing bad client_id/redirect_uri handling.
func (h *mcpHarness) oauthCSRFTokenFor(t *testing.T, client *http.Client) string {
	t.Helper()
	probeClientID := h.registerClient(t, mcpRedirectURI)
	_, challenge := pkcePair()
	q := authorizeQuery(probeClientID, mcpRedirectURI, challenge, "csrf-probe-state", "")
	resp, err := client.Get(h.srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("authorize (csrf probe): %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	tok := extractHiddenField(body, "csrf_token")
	if tok == "" {
		t.Fatalf("no csrf_token found in consent page: %s", body)
	}
	return tok
}

// decide POSTs the consent decision directly (the real oauth_consent.html
// form does exactly this — the hidden fields it carries are just what's
// already in the GET's query string plus organization_id and csrf_token) and
// returns the raw *http.Response (redirect not followed) so callers can
// inspect the Location header or a rendered error page. code_verifier is
// never part of this step (only the token exchange needs it) — the decision
// endpoint only re-validates the code_challenge it will bind into the issued
// code.
func (h *mcpHarness) decide(t *testing.T, client *http.Client, clientID, redirectURI, challenge, state, scope, decision, orgID, csrfToken string) *http.Response {
	t.Helper()
	form := url.Values{
		"client_id": {clientID}, "redirect_uri": {redirectURI},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"},
		"state": {state}, "decision": {decision}, "organization_id": {orgID},
		"csrf_token": {csrfToken},
	}
	if scope != "" {
		form.Set("scope", scope)
	}
	resp, err := client.PostForm(h.srv.URL+"/oauth/authorize/decision", form)
	if err != nil {
		t.Fatalf("decision: %v", err)
	}
	return resp
}

// fullAuthCodeFlow drives DCR → login → GET authorize (both before and after
// login) → approve → token exchange, and returns the issued token pair. scope
// "" requests every scope (mirrors the consent page's own default).
func (h *mcpHarness) fullAuthCodeFlow(t *testing.T, scope string) (accessToken, refreshToken, clientID string) {
	t.Helper()
	clientID = h.registerClient(t, mcpRedirectURI)
	verifier, challenge := pkcePair()
	state := "state-abc-123"

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// Unauthenticated: the authorize page must show the login form, not consent.
	q := authorizeQuery(clientID, mcpRedirectURI, challenge, state, scope)
	resp, err := client.Get(h.srv.URL + "/oauth/authorize?" + q.Encode())
	if err != nil {
		t.Fatalf("authorize (logged out): %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("auth/login")) {
		t.Fatalf("expected login page before auth, got status=%d body=%s", resp.StatusCode, body)
	}

	h.login(t, client)

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
		t.Fatalf("no csrf_token found in consent page: %s", body)
	}

	// Same cookie jar as the browser-like client above (it already carries
	// the session cookie from login), just configured to stop at the first
	// redirect so the Location header can be inspected directly instead of
	// the client trying to actually dial the (non-existent) client callback.
	noRedir := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	decResp := h.decide(t, noRedir, clientID, mcpRedirectURI, challenge, state, scope, "approve", h.orgID.String(), csrfToken)
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

	form := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {clientID},
		"redirect_uri": {mcpRedirectURI}, "code": {code}, "code_verifier": {verifier},
	}
	tokResp, err := http.PostForm(h.srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	defer tokResp.Body.Close()
	tb, _ := io.ReadAll(tokResp.Body)
	if tokResp.StatusCode != http.StatusOK {
		t.Fatalf("token status=%d body=%s", tokResp.StatusCode, tb)
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(tb, &tok); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" || tok.TokenType != "Bearer" {
		t.Fatalf("incomplete token response: %s", tb)
	}
	return tok.AccessToken, tok.RefreshToken, clientID
}

// rpc POSTs one JSON-RPC request to /mcp with the given bearer token and
// decodes the response. Fails the test on a transport-level (HTTP or
// JSON-RPC envelope) problem; a tool-level error (isError:true) is left for
// the caller to inspect in Result.
func (h *mcpHarness) rpc(t *testing.T, token, method string, params any) mcpserver.Response {
	t.Helper()
	req := mcpserver.Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: method}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		req.Params = b
	}
	reqBody, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, h.srv.URL+"/mcp", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("mcp request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mcp %s status=%d body=%s", method, resp.StatusCode, b)
	}
	var out mcpserver.Response
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode mcp response: %v body=%s", err, b)
	}
	return out
}

// toolCallResult decodes a tools/call Response.Result into the {content,
// isError, structuredContent} shape every tool handler returns.
func toolCallResult(t *testing.T, resp mcpserver.Response) map[string]any {
	t.Helper()
	m, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("tools/call result is not an object: %#v (rpc error: %#v)", resp.Result, resp.Error)
	}
	return m
}

// --- discovery -------------------------------------------------------------

func TestMCPDiscoveryEndpoints(t *testing.T) {
	h := newMCPHarness(t)

	var prm map[string]any
	h.get200Into(t, "/.well-known/oauth-protected-resource", &prm)
	if prm["resource"] != mcpAPIBase+"/mcp" {
		t.Fatalf("protected-resource metadata resource=%v", prm["resource"])
	}
	servers, _ := prm["authorization_servers"].([]any)
	if len(servers) != 1 || servers[0] != mcpAPIBase {
		t.Fatalf("authorization_servers=%v", prm["authorization_servers"])
	}

	var asMeta map[string]any
	h.get200Into(t, "/.well-known/oauth-authorization-server", &asMeta)
	for key, want := range map[string]string{
		"issuer": mcpAPIBase, "authorization_endpoint": mcpAPIBase + "/oauth/authorize",
		"token_endpoint": mcpAPIBase + "/oauth/token", "registration_endpoint": mcpAPIBase + "/oauth/register",
		"jwks_uri": mcpAPIBase + "/oauth/jwks.json",
	} {
		if asMeta[key] != want {
			t.Fatalf("authorization-server metadata[%s]=%v want %v", key, asMeta[key], want)
		}
	}
	methods, _ := asMeta["code_challenge_methods_supported"].([]any)
	if len(methods) != 1 || methods[0] != "S256" {
		t.Fatalf("code_challenge_methods_supported=%v", asMeta["code_challenge_methods_supported"])
	}
	// Task 11: advertise Client ID Metadata Document support (this server
	// already resolves that client_id shape — mcpauth.FetchCIMD /
	// Store.ResolveClient) so a host can discover it instead of guessing.
	if asMeta["client_id_metadata_document_supported"] != true {
		t.Fatalf("expected client_id_metadata_document_supported=true, got %v", asMeta["client_id_metadata_document_supported"])
	}

	var jwks map[string]any
	h.get200Into(t, "/oauth/jwks.json", &jwks)
	keys, _ := jwks["keys"].([]any)
	if len(keys) != 1 {
		t.Fatalf("jwks keys=%v", jwks["keys"])
	}
	key := keys[0].(map[string]any)
	if key["kty"] != "OKP" || key["crv"] != "Ed25519" || key["x"] == "" || key["kid"] == "" {
		t.Fatalf("unexpected jwk shape: %#v", key)
	}
}

func (h *mcpHarness) get200Into(t *testing.T, path string, out any) {
	t.Helper()
	resp, err := http.Get(h.srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%s", path, resp.StatusCode, b)
	}
	if err := json.Unmarshal(b, out); err != nil {
		t.Fatalf("GET %s decode: %v body=%s", path, err, b)
	}
}

// --- OAuth flow --------------------------------------------------------

func TestMCPOAuthFullFlow_ThroughToolsCall(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, refreshToken, _ := h.fullAuthCodeFlow(t, "")
	if refreshToken == "" {
		t.Fatalf("expected a refresh token")
	}

	init := h.rpc(t, accessToken, "initialize", map[string]any{})
	if init.Error != nil {
		t.Fatalf("initialize error: %+v", init.Error)
	}

	list := h.rpc(t, accessToken, "tools/list", nil)
	if list.Error != nil {
		t.Fatalf("tools/list error: %+v", list.Error)
	}
	result, ok := list.Result.(map[string]any)
	if !ok {
		t.Fatalf("tools/list result not an object: %#v", list.Result)
	}
	tools, _ := result["tools"].([]any)
	if len(tools) != 13 {
		t.Fatalf("expected 13 tools, got %d: %#v", len(tools), tools)
	}
}

func TestMCPOAuthDecision_DenyRedirectsWithError(t *testing.T) {
	h := newMCPHarness(t)
	clientID := h.registerClient(t, mcpRedirectURI)
	_, challenge := pkcePair()

	client := &http.Client{Jar: mustJar(t)}
	h.login(t, client)
	noRedir := &http.Client{Jar: client.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	csrfToken := h.oauthCSRFTokenFor(t, client)

	resp := h.decide(t, noRedir, clientID, mcpRedirectURI, challenge, "st1", "", "deny", h.orgID.String(), csrfToken)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("deny status=%d body=%s", resp.StatusCode, b)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("bad Location: %v", err)
	}
	if loc.Query().Get("error") != "access_denied" {
		t.Fatalf("expected error=access_denied in %s", loc)
	}
	if loc.Query().Get("code") != "" {
		t.Fatalf("a denied decision must not carry a code: %s", loc)
	}
}

// TestMCPOAuthDecision_UnknownClientNeverRedirects is the open-redirect
// regression test for the security property handleOAuthDecision is built
// around: it re-resolves client_id/redirect_uri from the POSTed form BEFORE
// branching on approve/deny, so a request naming an unregistered client (or
// one whose redirect_uri isn't its own) renders the in-app error page
// instead of ever issuing a 302 to an attacker-supplied URL.
func TestMCPOAuthDecision_UnknownClientNeverRedirects(t *testing.T) {
	h := newMCPHarness(t)
	client := &http.Client{Jar: mustJar(t)}
	h.login(t, client)
	noRedir := &http.Client{Jar: client.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	csrfToken := h.oauthCSRFTokenFor(t, client)

	resp := h.decide(t, noRedir, "unregistered-client-id", "https://evil.test/steal", "c", "st1", "", "approve", h.orgID.String(), csrfToken)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusFound {
		t.Fatalf("must not redirect for an unknown client, got Location=%s", resp.Header.Get("Location"))
	}
}

// TestMCPOAuthDecision_MissingOrWrongCSRFTokenIsRejected is Task 11's
// regression guard: the decision endpoint previously relied only on
// SameSite=Lax to keep a cross-site POST from ever carrying the session
// cookie — that alone does not stop a top-level cross-site navigation (e.g.
// an auto-submitting form on an attacker's page), so a real CSRF token is now
// required and checked explicitly.
func TestMCPOAuthDecision_MissingOrWrongCSRFTokenIsRejected(t *testing.T) {
	h := newMCPHarness(t)
	clientID := h.registerClient(t, mcpRedirectURI)
	_, challenge := pkcePair()

	client := &http.Client{Jar: mustJar(t)}
	h.login(t, client)
	noRedir := &http.Client{Jar: client.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	for name, token := range map[string]string{"missing": "", "wrong": "deadbeef-not-the-real-token"} {
		resp := h.decide(t, noRedir, clientID, mcpRedirectURI, challenge, "st1", "", "approve", h.orgID.String(), token)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusFound {
			t.Fatalf("%s csrf_token: must not redirect (would issue a code), got Location=%s", name, resp.Header.Get("Location"))
		}
		if !bytes.Contains(body, []byte("CSRF")) {
			t.Fatalf("%s csrf_token: expected the CSRF error page, got status=%d body=%s", name, resp.StatusCode, body)
		}
	}

	// Sanity check: the SAME request shape with the real token succeeds —
	// proves the rejections above are specifically about the token, not some
	// other broken precondition in this test.
	csrfToken := h.oauthCSRFTokenFor(t, client)
	okResp := h.decide(t, noRedir, clientID, mcpRedirectURI, challenge, "st1", "", "approve", h.orgID.String(), csrfToken)
	okResp.Body.Close()
	if okResp.StatusCode != http.StatusFound {
		t.Fatalf("expected a valid csrf_token to succeed, got status=%d", okResp.StatusCode)
	}
}

// TestMCPOAuthToken_ResourceMismatchRejected is Task 11's httpapi-level
// regression guard for the RFC 8707 resource re-check at token exchange
// (internal/mcpauth's own test covers the Store/Authorizer layer directly;
// this proves the real /oauth/token route wires the resource form field
// through rather than silently ignoring it).
func TestMCPOAuthToken_ResourceMismatchRejected(t *testing.T) {
	h := newMCPHarness(t)
	clientID := h.registerClient(t, mcpRedirectURI)
	verifier, challenge := pkcePair()
	state := "state-resource-mismatch"

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

	noRedir := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	decResp := h.decide(t, noRedir, clientID, mcpRedirectURI, challenge, state, "", "approve", h.orgID.String(), csrfToken)
	loc, _ := url.Parse(decResp.Header.Get("Location"))
	decResp.Body.Close()
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatalf("no code in redirect Location=%s", loc)
	}

	form := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {clientID},
		"redirect_uri": {mcpRedirectURI}, "code": {code}, "code_verifier": {verifier},
		"resource": {"https://attacker.example/mcp"},
	}
	tokResp, err := http.PostForm(h.srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	tb, _ := io.ReadAll(tokResp.Body)
	tokResp.Body.Close()
	if tokResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for a mismatched resource, got status=%d body=%s", tokResp.StatusCode, tb)
	}
	if tokResp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("expected Cache-Control: no-store on the token error response, got %q", tokResp.Header.Get("Cache-Control"))
	}
}

// TestMCPOAuthToken_SuccessResponseIsNoStore confirms RFC 6749 §5.1's
// Cache-Control: no-store requirement on a SUCCESSFUL token response too, not
// only the error path.
func TestMCPOAuthToken_SuccessResponseIsNoStore(t *testing.T) {
	h := newMCPHarness(t)
	clientID := h.registerClient(t, mcpRedirectURI)
	verifier, challenge := pkcePair()
	state := "state-no-store"

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

	noRedir := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	decResp := h.decide(t, noRedir, clientID, mcpRedirectURI, challenge, state, "", "approve", h.orgID.String(), csrfToken)
	loc, _ := url.Parse(decResp.Header.Get("Location"))
	decResp.Body.Close()

	form := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {clientID},
		"redirect_uri": {mcpRedirectURI}, "code": {loc.Query().Get("code")}, "code_verifier": {verifier},
	}
	tokResp, err := http.PostForm(h.srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	tb, _ := io.ReadAll(tokResp.Body)
	tokResp.Body.Close()
	if tokResp.StatusCode != http.StatusOK {
		t.Fatalf("token exchange should succeed, got status=%d body=%s", tokResp.StatusCode, tb)
	}
	if tokResp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("expected Cache-Control: no-store on a successful token response, got %q", tokResp.Header.Get("Cache-Control"))
	}
}

// TestMCPOAuthRegister_RateLimited is Task 11's regression guard for the
// abuse limit on /oauth/register: the configured burst is exhausted quickly
// (no timing dependency — burst exhaustion is instant), so the next request
// from the SAME IP must be rejected with 429 before ever reaching DCR logic.
func TestMCPOAuthRegister_RateLimited(t *testing.T) {
	h := newMCPHarness(t)
	var last *http.Response
	for i := 0; i < 6; i++ {
		body, _ := json.Marshal(map[string]any{"client_name": "Burst Test", "redirect_uris": []string{mcpRedirectURI}})
		resp, err := http.Post(h.srv.URL+"/oauth/register", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("register attempt %d: %v", i, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		last = resp
	}
	if last.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected the 6th rapid registration to be rate-limited (429), got %d", last.StatusCode)
	}
}

func TestMCPRefreshToken_RotatesAndOldOneStopsWorking(t *testing.T) {
	h := newMCPHarness(t)
	_, refreshToken, clientID := h.fullAuthCodeFlow(t, "")

	form := url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "refresh_token": {refreshToken}}
	resp, err := http.PostForm(h.srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", resp.StatusCode, b)
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(b, &tok); err != nil {
		t.Fatalf("decode refresh: %v", err)
	}
	if tok.RefreshToken == "" || tok.RefreshToken == refreshToken {
		t.Fatalf("expected a NEW rotated refresh token, got %q (old %q)", tok.RefreshToken, refreshToken)
	}

	// The rotated-away old refresh token must no longer work.
	reuse, err := http.PostForm(h.srv.URL+"/oauth/token", form)
	if err != nil {
		t.Fatalf("reuse refresh: %v", err)
	}
	reuse.Body.Close()
	if reuse.StatusCode == http.StatusOK {
		t.Fatalf("a rotated-away refresh token must be rejected, got 200")
	}
}

func mustJar(t *testing.T) *cookiejar.Jar {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return jar
}

// --- tenant re-check --------------------------------------------------------

func TestMCPAccessToken_TenantRecheckRejectsRemovedUser(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, _, _ := h.fullAuthCodeFlow(t, "")

	// sanity: the token works before the membership is removed.
	if resp := h.rpc(t, accessToken, "ping", nil); resp.Error != nil {
		t.Fatalf("ping before removal: %+v", resp.Error)
	}

	ctx := context.Background()
	if _, err := h.store.Pool().Exec(ctx, `DELETE FROM xchats.organization_users WHERE user_id = $1 AND organization_id = $2`, h.userID, h.orgID); err != nil {
		t.Fatalf("remove membership: %v", err)
	}

	httpReq, _ := http.NewRequest(http.MethodPost, h.srv.URL+"/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("post /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 401 for a removed member's still-valid token, got %d body=%s", resp.StatusCode, b)
	}
	if resp.Header.Get("WWW-Authenticate") == "" {
		t.Fatalf("expected a WWW-Authenticate challenge header on 401")
	}
}

// --- scope enforcement -------------------------------------------------

func TestMCPToolsCall_MissingScopeIsToolErrorNotTransportFailure(t *testing.T) {
	h := newMCPHarness(t)
	// Grant only kb:read — kb_product_upsert requires kb:draft:write.
	accessToken, _, _ := h.fullAuthCodeFlow(t, mcpauth.ScopeKBRead)

	resp := h.rpc(t, accessToken, "tools/call", map[string]any{
		"name":      "kb_product_upsert",
		"arguments": map[string]any{"ref": "widget-1", "changes": map[string]any{"name": "Widget"}},
	})
	if resp.Error != nil {
		t.Fatalf("expected a successful JSON-RPC envelope carrying isError, got RPC error: %+v", resp.Error)
	}
	result := toolCallResult(t, resp)
	if result["isError"] != true {
		t.Fatalf("expected isError=true for a missing-scope tool call, got %#v", result)
	}
}

// --- media upload end to end --------------------------------------------

func TestMCPMediaUploadEndToEnd(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, _, _ := h.fullAuthCodeFlow(t, "")

	// Attaching never creates a record — the product must already exist
	// before kb_media_attach can target it (see kbstore.MCPAttachMedia's doc
	// comment: "The record is never created").
	create := h.rpc(t, accessToken, "tools/call", map[string]any{
		"name": "kb_product_upsert",
		"arguments": map[string]any{
			"ref":     "widget-1",
			"changes": map[string]any{"name": "Widget", "price": "1000", "in_stock": true},
		},
	})
	if create.Error != nil {
		t.Fatalf("kb_product_upsert (create) rpc error: %+v", create.Error)
	}
	if toolCallResult(t, create)["isError"] == true {
		t.Fatalf("kb_product_upsert (create) returned isError: %#v", toolCallResult(t, create))
	}

	upload := h.rpc(t, accessToken, "tools/call", map[string]any{
		"name": "kb_media_upload",
		"arguments": map[string]any{
			"filename": "photo.png", "mime_type": "image/png", "size_bytes": len(pngBytes),
			"sha256_checksum": pngSHA256Hex,
			"target":          map[string]any{"type": "product", "key": "widget-1", "field": "gallery_images"},
		},
	})
	if upload.Error != nil {
		t.Fatalf("kb_media_upload rpc error: %+v", upload.Error)
	}
	uploadResult := toolCallResult(t, upload)
	if uploadResult["isError"] == true {
		t.Fatalf("kb_media_upload returned isError: %#v", uploadResult)
	}
	structured, ok := uploadResult["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("kb_media_upload missing structuredContent: %#v", uploadResult)
	}
	materialID, _ := structured["material_id"].(string)
	uploadURLRaw, _ := structured["upload_url"].(string)
	if materialID == "" || uploadURLRaw == "" {
		t.Fatalf("kb_media_upload structuredContent missing material_id/upload_url: %#v", structured)
	}
	parsedUpload, err := url.Parse(uploadURLRaw)
	if err != nil {
		t.Fatalf("parse upload_url %q: %v", uploadURLRaw, err)
	}
	token := parsedUpload.Query().Get("token")
	if token == "" {
		t.Fatalf("upload_url has no signed token: %s", uploadURLRaw)
	}

	// PUT the actual bytes to the real test server (the upload_url's host is
	// the configured UploadBaseURL, not necessarily this httptest instance —
	// only the path + signed token are needed to reach the same route here).
	putReq, err := http.NewRequest(http.MethodPut, h.srv.URL+"/mcp/uploads/"+materialID+"?token="+url.QueryEscape(token), bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("new PUT: %v", err)
	}
	putReq.Header.Set("Content-Type", "image/png")
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("PUT upload: %v", err)
	}
	pb, _ := io.ReadAll(putResp.Body)
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT upload status=%d body=%s", putResp.StatusCode, pb)
	}

	// Attach the uploaded material to the product's gallery_images via the
	// dedicated app-only tool — not a kb_product_upsert changes patch.
	attach := h.rpc(t, accessToken, "tools/call", map[string]any{
		"name": "kb_media_attach",
		"arguments": map[string]any{
			"material_id": materialID, "type": "product", "key": "widget-1", "field": "gallery_images",
		},
	})
	if attach.Error != nil {
		t.Fatalf("kb_media_attach rpc error: %+v", attach.Error)
	}
	attachResult := toolCallResult(t, attach)
	if attachResult["isError"] == true {
		t.Fatalf("kb_media_attach returned isError: %#v", attachResult)
	}

	// Confirm the read-back record actually carries the material_id as the
	// first (and only) gallery entry. featured_image is intentionally NOT
	// asserted here: it stays whatever it already was — kb_media_attach only
	// ever writes the field it was asked for, and gallery_images[0]'s
	// relationship to featured_image is resolved at prompt-build time
	// (aiprompt), not stored as a second copy in the draft/live row.
	read := h.rpc(t, accessToken, "tools/call", map[string]any{
		"name":      "kb_read",
		"arguments": map[string]any{"types": []string{"product"}, "source": "draft", "key": "widget-1"},
	})
	if read.Error != nil {
		t.Fatalf("kb_read rpc error: %+v", read.Error)
	}
	readResult := toolCallResult(t, read)
	readStructured, _ := readResult["structuredContent"].(map[string]any)
	items, _ := readStructured["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 draft product record, got %#v", items)
	}
	record := items[0].(map[string]any)
	data, _ := record["data"].(map[string]any)
	gallery, _ := data["gallery_images"].([]any)
	if len(gallery) != 1 || gallery[0] != materialID {
		t.Fatalf("gallery_images=%v want [%v]", gallery, materialID)
	}
}

// TestMCPMediaUpload_ReplayAfterSuccessRejected is the regression guard for
// the one-time-upload fix (mcp_upload.go, kbstore.CompleteMaterialUpload): a
// signed kb_media_upload URL stays valid for its whole TTL, not single-use,
// so before this fix a second PUT to an ALREADY-COMPLETED target silently
// replaced the first upload's bytes — including after that material had
// already been attached to a live/draft KB record via a kb_*_upsert call.
// This confirms the first PUT succeeds, a second PUT with DIFFERENT bytes to
// the SAME url is rejected 409, and the originally stored bytes are
// provably unchanged afterward.
func TestMCPMediaUpload_ReplayAfterSuccessRejected(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, _, _ := h.fullAuthCodeFlow(t, "")

	upload := h.rpc(t, accessToken, "tools/call", map[string]any{
		"name": "kb_media_upload",
		"arguments": map[string]any{
			"filename": "photo.png", "mime_type": "image/png", "size_bytes": len(pngBytes),
			"sha256_checksum": pngSHA256Hex,
		},
	})
	if upload.Error != nil {
		t.Fatalf("kb_media_upload rpc error: %+v", upload.Error)
	}
	structured, ok := toolCallResult(t, upload)["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("kb_media_upload missing structuredContent")
	}
	materialID, _ := structured["material_id"].(string)
	uploadURLRaw, _ := structured["upload_url"].(string)
	parsedUpload, err := url.Parse(uploadURLRaw)
	if err != nil {
		t.Fatalf("parse upload_url %q: %v", uploadURLRaw, err)
	}
	token := parsedUpload.Query().Get("token")
	putURL := h.srv.URL + "/mcp/uploads/" + materialID + "?token=" + url.QueryEscape(token)

	put := func(data []byte) (status int, body []byte) {
		req, err := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(data))
		if err != nil {
			t.Fatalf("new PUT: %v", err)
		}
		req.Header.Set("Content-Type", "image/png")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT: %v", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}

	if status, body := put(pngBytes); status != http.StatusOK {
		t.Fatalf("first PUT: status=%d body=%s", status, body)
	}

	// Same length as pngBytes (the declared size_bytes) so a rejection here
	// is provably about the already-completed upload, not an incidental
	// size mismatch — the PNG magic prefix keeps it mime-sniffable too.
	swapped := append(append([]byte{}, pngBytes[:8]...), 0xFF, 0xFF, 0xFF, 0xFF)
	status, body := put(swapped)
	if status != http.StatusConflict {
		t.Fatalf("replay PUT: expected 409, got status=%d body=%s", status, body)
	}

	var storageKey string
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT storage_key FROM xchats.kbd_materials WHERE id = $1`, materialID).Scan(&storageKey); err != nil {
		t.Fatalf("read storage_key: %v", err)
	}
	stored, _, err := h.blob.Get(storageKey)
	if err != nil {
		t.Fatalf("blob.Get(%q): %v", storageKey, err)
	}
	if !bytes.Equal(stored, pngBytes) {
		t.Fatalf("stored bytes were overwritten by the replay: got %x, want %x", stored, pngBytes)
	}
}

// TestMCPMediaUpload_ConcurrentPUTsOnlyOneWins is a stricter companion to
// the replay test above: it races two GENUINELY CONCURRENT PUTs against the
// SAME signed target, so both can pass the early processing_status
// pre-check (mcp_upload.go) before either one commits — this is the
// scenario only CompleteMaterialUpload's atomic, WHERE-guarded UPDATE (not
// that pre-check) can actually close. Exactly one must succeed, the other
// must get 409, and the material's final stored bytes must match whichever
// one actually won — never neither, and never a mix of both.
func TestMCPMediaUpload_ConcurrentPUTsOnlyOneWins(t *testing.T) {
	h := newMCPHarness(t)
	accessToken, _, _ := h.fullAuthCodeFlow(t, "")

	upload := h.rpc(t, accessToken, "tools/call", map[string]any{
		"name": "kb_media_upload",
		"arguments": map[string]any{
			"filename": "photo.png", "mime_type": "image/png", "size_bytes": len(pngBytes),
		},
	})
	if upload.Error != nil {
		t.Fatalf("kb_media_upload rpc error: %+v", upload.Error)
	}
	structured, ok := toolCallResult(t, upload)["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("kb_media_upload missing structuredContent")
	}
	materialID, _ := structured["material_id"].(string)
	uploadURLRaw, _ := structured["upload_url"].(string)
	parsedUpload, err := url.Parse(uploadURLRaw)
	if err != nil {
		t.Fatalf("parse upload_url %q: %v", uploadURLRaw, err)
	}
	token := parsedUpload.Query().Get("token")
	putURL := h.srv.URL + "/mcp/uploads/" + materialID + "?token=" + url.QueryEscape(token)

	payloadA := pngBytes
	// Same length as pngBytes (the declared size_bytes), same PNG magic
	// prefix (so it sniffs as image/png too) — the ONLY difference from
	// payloadA is content, so which one "wins" is decided purely by
	// CompleteMaterialUpload's atomic guard, not by either payload failing
	// an unrelated validation check.
	payloadB := append(append([]byte{}, pngBytes[:8]...), 0xFF, 0xFF, 0xFF, 0xFF)

	put := func(data []byte) int {
		req, err := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(data))
		if err != nil {
			t.Fatalf("new PUT: %v", err)
		}
		req.Header.Set("Content-Type", "image/png")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT: %v", err)
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}

	var wg sync.WaitGroup
	statuses := make([]int, 2)
	wg.Add(2)
	go func() { defer wg.Done(); statuses[0] = put(payloadA) }()
	go func() { defer wg.Done(); statuses[1] = put(payloadB) }()
	wg.Wait()

	okCount, conflictCount := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusOK:
			okCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Fatalf("unexpected status %d among %v", s, statuses)
		}
	}
	if okCount != 1 || conflictCount != 1 {
		t.Fatalf("expected exactly one 200 and one 409, got statuses=%v", statuses)
	}

	var storageKey string
	if err := h.store.Pool().QueryRow(context.Background(),
		`SELECT storage_key FROM xchats.kbd_materials WHERE id = $1`, materialID).Scan(&storageKey); err != nil {
		t.Fatalf("read storage_key: %v", err)
	}
	stored, _, err := h.blob.Get(storageKey)
	if err != nil {
		t.Fatalf("blob.Get(%q): %v", storageKey, err)
	}
	if !bytes.Equal(stored, payloadA) && !bytes.Equal(stored, payloadB) {
		t.Fatalf("stored bytes match NEITHER concurrent attempt (data corruption): %x", stored)
	}
}

// pngBytes is the minimal byte sequence http.DetectContentType recognizes as
// image/png (the 8-byte PNG signature; no valid IHDR/IDAT needed since only
// the sniff prefix is checked by mimeSanityCheck).
var pngBytes = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}

var pngSHA256Hex = func() string {
	sum := sha256.Sum256(pngBytes)
	return hex.EncodeToString(sum[:])
}()
