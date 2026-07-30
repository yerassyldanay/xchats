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
		MCPUploadTokenTTLSeconds: 900,
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
		t: t, srv: ts, cfg: cfg, store: st, orgID: org.ID, userID: user.ID,
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

// decide POSTs the consent decision directly (the real oauth_consent.html
// form does exactly this — the hidden fields it carries are just what's
// already in the GET's query string plus organization_id) and returns the
// raw *http.Response (redirect not followed) so callers can inspect the
// Location header or a rendered error page. code_verifier is never part of
// this step (only the token exchange needs it) — the decision endpoint only
// re-validates the code_challenge it will bind into the issued code.
func (h *mcpHarness) decide(t *testing.T, client *http.Client, clientID, redirectURI, challenge, state, scope, decision, orgID string) *http.Response {
	t.Helper()
	form := url.Values{
		"client_id": {clientID}, "redirect_uri": {redirectURI},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"},
		"state": {state}, "decision": {decision}, "organization_id": {orgID},
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

	// Same cookie jar as the browser-like client above (it already carries
	// the session cookie from login), just configured to stop at the first
	// redirect so the Location header can be inspected directly instead of
	// the client trying to actually dial the (non-existent) client callback.
	noRedir := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	decResp := h.decide(t, noRedir, clientID, mcpRedirectURI, challenge, state, scope, "approve", h.orgID.String())
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
	if len(tools) != 12 {
		t.Fatalf("expected 12 tools, got %d: %#v", len(tools), tools)
	}
}

func TestMCPOAuthDecision_DenyRedirectsWithError(t *testing.T) {
	h := newMCPHarness(t)
	clientID := h.registerClient(t, mcpRedirectURI)
	_, challenge := pkcePair()

	client := &http.Client{Jar: mustJar(t)}
	h.login(t, client)
	noRedir := &http.Client{Jar: client.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	resp := h.decide(t, noRedir, clientID, mcpRedirectURI, challenge, "st1", "", "deny", h.orgID.String())
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

	resp := h.decide(t, noRedir, "unregistered-client-id", "https://evil.test/steal", "c", "st1", "", "approve", h.orgID.String())
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusFound {
		t.Fatalf("must not redirect for an unknown client, got Location=%s", resp.Header.Get("Location"))
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

	upload := h.rpc(t, accessToken, "tools/call", map[string]any{
		"name": "kb_media_upload",
		"arguments": map[string]any{
			"filename": "photo.png", "mime_type": "image/png", "size_bytes": len(pngBytes),
			"sha256_checksum": pngSHA256Hex,
			"target":          map[string]any{"type": "product", "key": "widget-1", "field": "featured_image"},
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

	// Attach the uploaded material to a product's featured_image.
	upsert := h.rpc(t, accessToken, "tools/call", map[string]any{
		"name": "kb_product_upsert",
		"arguments": map[string]any{
			"ref": "widget-1",
			"changes": map[string]any{
				"name": "Widget", "price": "1000", "in_stock": true, "featured_image": materialID,
			},
		},
	})
	if upsert.Error != nil {
		t.Fatalf("kb_product_upsert rpc error: %+v", upsert.Error)
	}
	upsertResult := toolCallResult(t, upsert)
	if upsertResult["isError"] == true {
		t.Fatalf("kb_product_upsert returned isError: %#v", upsertResult)
	}

	// Confirm the read-back record actually carries the material_id.
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
	if data["featured_image"] != materialID {
		t.Fatalf("featured_image=%v want %v", data["featured_image"], materialID)
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
