package httpapi

import (
	"embed"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

//go:embed templates/*.tmpl
var oauthTemplatesFS embed.FS

var (
	oauthLoginTmpl   = template.Must(template.ParseFS(oauthTemplatesFS, "templates/oauth_login.html.tmpl"))
	oauthConsentTmpl = template.Must(template.ParseFS(oauthTemplatesFS, "templates/oauth_consent.html.tmpl"))
	oauthErrorTmpl   = template.Must(template.ParseFS(oauthTemplatesFS, "templates/oauth_error.html.tmpl"))
)

// authorizeParams is one validated GET /oauth/authorize request (plan/
// mcp.md §3). code_challenge_method is always "S256" by the time
// validateAuthorizeParams returns — the "plain" PKCE method is not offered.
type authorizeParams struct {
	ClientID, RedirectURI, Scope, State, CodeChallenge, CodeChallengeMethod, Resource string
}

// validateAuthorizeParams runs every check plan/mcp.md §3 requires BEFORE
// anything is rendered: response_type=code, PKCE S256 present, the resource
// indicator matches this server's one MCP resource (defaulted, never
// silently substituted, if the client omits it), the client_id resolves
// (DCR or CIMD), and redirect_uri is one of ITS registered URIs — an
// unvalidated redirect_uri is never used, even to report an error.
func (s *Server) validateAuthorizeParams(c *gin.Context) (authorizeParams, mcpauth.Client, string) {
	p := authorizeParams{
		ClientID: c.Query("client_id"), RedirectURI: c.Query("redirect_uri"), Scope: c.Query("scope"),
		State: c.Query("state"), CodeChallenge: c.Query("code_challenge"),
		CodeChallengeMethod: c.Query("code_challenge_method"), Resource: c.Query("resource"),
	}
	if rt := c.Query("response_type"); rt != "code" {
		return p, mcpauth.Client{}, `response_type must be "code"`
	}
	if p.ClientID == "" || p.RedirectURI == "" {
		return p, mcpauth.Client{}, "client_id and redirect_uri are required"
	}
	if p.CodeChallengeMethod == "" {
		p.CodeChallengeMethod = "S256"
	}
	if p.CodeChallengeMethod != "S256" || p.CodeChallenge == "" {
		return p, mcpauth.Client{}, "code_challenge with code_challenge_method=S256 is required (PKCE)"
	}
	if p.Resource == "" {
		p.Resource = s.cfg.MCPResourceURL()
	} else if p.Resource != s.cfg.MCPResourceURL() {
		return p, mcpauth.Client{}, "resource must be " + s.cfg.MCPResourceURL()
	}
	if _, err := mcpauth.ValidateScope(p.Scope); err != nil {
		return p, mcpauth.Client{}, err.Error()
	}
	client, err := s.mcpAuth.Store.ResolveClient(ctx(c), p.ClientID, s.cfg.KBAllowPrivateFetch)
	if err != nil {
		return p, mcpauth.Client{}, "unknown client_id — register it first (Dynamic Client Registration or a Client ID Metadata Document)"
	}
	if !client.AllowsRedirect(p.RedirectURI) {
		return p, mcpauth.Client{}, "redirect_uri is not registered for this client"
	}
	return p, client, ""
}

// handleOAuthAuthorize serves the browser-facing authorization page
// (plan/mcp.md §3 "First connection" steps 4-6): the existing session
// cookie authenticates the user here (and ONLY here / the decision
// endpoint — it is never sent to the MCP host).
func (s *Server) handleOAuthAuthorize(c *gin.Context) {
	if !s.mcpAuthEnabled() {
		s.renderOAuthError(c, "Коннектор MCP не настроен на этом сервере.")
		return
	}
	params, client, errMsg := s.validateAuthorizeParams(c)
	if errMsg != "" {
		s.renderOAuthError(c, errMsg)
		return
	}

	user, sid, ok := s.oauthSessionUser(c)
	if !ok {
		s.renderOAuthLogin(c, client)
		return
	}
	orgs, err := s.store.OrgsForUser(ctx(c), user.ID)
	if err != nil || len(orgs) == 0 {
		s.renderOAuthError(c, "У вашей учётной записи нет организации.")
		return
	}
	s.renderOAuthConsent(c, client, user, sid, orgs, params)
}

// oauthSessionUser reads the existing xchats session cookie — the SAME
// cookie requireSession checks — without failing the request when absent
// (the authorize page's whole job is to handle "not logged in" itself, by
// showing the login form instead of a 401). The raw session id is returned
// alongside the user because the CSRF token (mcp_oauth_csrf.go) is derived
// from it.
func (s *Server) oauthSessionUser(c *gin.Context) (store.User, string, bool) {
	sid, err := c.Cookie(sessionCookie)
	if err != nil || sid == "" {
		return store.User{}, "", false
	}
	user, err := s.store.UserForSession(ctx(c), sid)
	if err != nil {
		return store.User{}, "", false
	}
	return user, sid, true
}

func (s *Server) renderOAuthError(c *gin.Context, message string) {
	c.Status(http.StatusBadRequest)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = oauthErrorTmpl.Execute(c.Writer, map[string]any{"Message": message})
}

func (s *Server) renderOAuthLogin(c *gin.Context, client mcpauth.Client) {
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = oauthLoginTmpl.Execute(c.Writer, map[string]any{
		"ClientName": displayName(client), "APIBaseURL": s.mcpIssuer(),
	})
}

func (s *Server) renderOAuthConsent(c *gin.Context, client mcpauth.Client, user store.User, sessionID string, orgs []store.Organization, p authorizeParams) {
	hidden := map[string]string{
		"client_id": p.ClientID, "redirect_uri": p.RedirectURI, "scope": p.Scope, "state": p.State,
		"code_challenge": p.CodeChallenge, "code_challenge_method": p.CodeChallengeMethod, "resource": p.Resource,
		"csrf_token": s.oauthCSRFToken(sessionID),
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = oauthConsentTmpl.Execute(c.Writer, map[string]any{
		"ClientName": displayName(client), "UserEmail": user.Email, "APIBaseURL": s.mcpIssuer(),
		"HiddenFields": hidden, "Organizations": orgs,
		"ScopeDescriptions": scopeDescriptions(orDefault(p.Scope, mcpauth.FormatScope(mcpauth.AllScopes))),
	})
}

// orDefault returns v, or def if v is empty — used for the OAuth params
// (scope, resource) that are optional on the wire but must resolve to a
// concrete value before being persisted or compared.
func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func displayName(c mcpauth.Client) string {
	if c.ClientName != "" {
		return c.ClientName
	}
	return c.ClientID
}

func scopeDescriptions(scope string) []string {
	labels := map[string]string{
		mcpauth.ScopeKBRead:       "Просматривать базу знаний (товары, тарифы, темы, контакты, политики)",
		mcpauth.ScopeKBDraftWrite: "Создавать и изменять черновик базы знаний",
		mcpauth.ScopeMediaWrite:   "Загружать изображения и файлы",
	}
	var out []string
	for _, sc := range mcpauth.ParseScope(scope) {
		if lbl, ok := labels[sc]; ok {
			out = append(out, lbl)
		}
	}
	return out
}

// handleOAuthDecision is the consent page's form target (plan/mcp.md §3
// step 6: "user selects an organization and grants requested permissions").
// It re-validates client_id/redirect_uri/PKCE/resource from scratch — the
// posted hidden fields are never trusted merely because the GET page
// rendered them — so neither branch (approve or deny) ever redirects to an
// unvalidated URI.
func (s *Server) handleOAuthDecision(c *gin.Context) {
	if !s.mcpAuthEnabled() {
		s.renderOAuthError(c, "Коннектор MCP не настроен на этом сервере.")
		return
	}
	user, sid, ok := s.oauthSessionUser(c)
	if !ok {
		s.renderOAuthError(c, "Сессия истекла. Начните подключение заново.")
		return
	}
	// CSRF: the consent page embedded a token bound to THIS session id
	// (mcp_oauth_csrf.go) — SameSite=Lax on the session cookie alone still
	// allows a top-level cross-site navigation (e.g. an auto-submitting form
	// on an attacker's page) to carry the cookie, so this is checked
	// explicitly rather than relied on implicitly.
	if !s.verifyOAuthCSRFToken(sid, c.PostForm("csrf_token")) {
		s.renderOAuthError(c, "Недействительный запрос (CSRF). Начните подключение заново.")
		return
	}

	clientID := c.PostForm("client_id")
	redirectURI := c.PostForm("redirect_uri")
	state := c.PostForm("state")
	client, err := s.mcpAuth.Store.ResolveClient(ctx(c), clientID, s.cfg.KBAllowPrivateFetch)
	if err != nil || !client.AllowsRedirect(redirectURI) {
		s.renderOAuthError(c, "Недействительный запрос авторизации.")
		return
	}

	if c.PostForm("decision") != "approve" {
		s.redirectWithQuery(c, redirectURI, map[string]string{"error": "access_denied"}, state)
		return
	}

	codeChallenge := c.PostForm("code_challenge")
	codeChallengeMethod := c.PostForm("code_challenge_method")
	if codeChallengeMethod != "S256" || codeChallenge == "" {
		s.renderOAuthError(c, "Недействительный запрос авторизации (PKCE).")
		return
	}
	resource := orDefault(c.PostForm("resource"), s.cfg.MCPResourceURL())
	if resource != s.cfg.MCPResourceURL() {
		s.renderOAuthError(c, "Недействительный ресурс.")
		return
	}
	orgID, err := uuid.Parse(c.PostForm("organization_id"))
	if err != nil {
		s.renderOAuthError(c, "Недействительная организация.")
		return
	}
	inOrg, err := s.store.UserInOrg(ctx(c), user.ID, orgID)
	if err != nil || !inOrg {
		s.renderOAuthError(c, "Вы не состоите в выбранной организации.")
		return
	}

	requestedScopes, err := mcpauth.ValidateScope(orDefault(c.PostForm("scope"), mcpauth.FormatScope(mcpauth.AllScopes)))
	if err != nil {
		s.renderOAuthError(c, "Недействительная область доступа (scope): "+err.Error())
		return
	}
	scope := mcpauth.FormatScope(requestedScopes)
	ttl := time.Duration(s.cfg.MCPAuthCodeTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	code, err := s.mcpAuth.Store.IssueAuthorizationCode(ctx(c), mcpauth.AuthorizationCodeInput{
		ClientID: clientID, RedirectURI: redirectURI, CodeChallenge: codeChallenge, CodeChallengeMethod: codeChallengeMethod,
		UserID: user.ID, OrganizationID: orgID, Scope: scope, Resource: resource, TTL: ttl,
	})
	if err != nil {
		s.renderOAuthError(c, "Не удалось создать код авторизации.")
		return
	}
	s.redirectWithQuery(c, redirectURI, map[string]string{"code": code}, state)
}

func (s *Server) redirectWithQuery(c *gin.Context, redirectURI string, params map[string]string, state string) {
	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	var b strings.Builder
	b.WriteString(redirectURI)
	b.WriteString(sep)
	first := true
	for k, v := range params {
		if !first {
			b.WriteString("&")
		}
		first = false
		b.WriteString(k + "=" + url.QueryEscape(v))
	}
	if state != "" {
		b.WriteString("&state=" + url.QueryEscape(state))
	}
	c.Redirect(http.StatusFound, b.String())
}

// --- token / revoke / register --------------------------------------------

func (s *Server) handleOAuthToken(c *gin.Context) {
	// RFC 6749 §5.1: a token response (success or error) must never be
	// cached — set unconditionally, before any return path below.
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	if !s.mcpAuthEnabled() {
		oauthTokenError(c, http.StatusServiceUnavailable, "server_error", "MCP connector not configured")
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		oauthTokenError(c, http.StatusBadRequest, "invalid_request", "malformed form body")
		return
	}
	clientID := c.PostForm("client_id")
	if clientID == "" {
		oauthTokenError(c, http.StatusBadRequest, "invalid_request", "client_id is required")
		return
	}
	switch c.PostForm("grant_type") {
	case "authorization_code":
		// resource (RFC 8707) is optional here — a client MAY repeat it from
		// the authorize request; if it does, mcpauth re-checks it against
		// what was actually bound to the code, rather than only carrying it
		// through unchecked.
		tok, err := s.mcpAuth.ExchangeAuthorizationCode(ctx(c), clientID, c.PostForm("redirect_uri"), c.PostForm("resource"), c.PostForm("code"), c.PostForm("code_verifier"))
		if err != nil {
			oauthTokenError(c, http.StatusBadRequest, "invalid_grant", "the authorization code is invalid, expired, redirect_uri/resource mismatched, or already used")
			return
		}
		c.JSON(http.StatusOK, tok)
	case "refresh_token":
		tok, err := s.mcpAuth.RefreshTokenPair(ctx(c), clientID, c.PostForm("refresh_token"))
		if err != nil {
			oauthTokenError(c, http.StatusBadRequest, "invalid_grant", "the refresh token is invalid, revoked, or expired")
			return
		}
		c.JSON(http.StatusOK, tok)
	default:
		oauthTokenError(c, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token")
	}
}

func oauthTokenError(c *gin.Context, status int, code, description string) {
	c.JSON(status, gin.H{"error": code, "error_description": description})
}

// handleOAuthRevoke implements RFC 7009: always 200, whether or not the
// token existed or which kind it was (an access-token JWT is denylisted by
// jti; a refresh token is marked revoked).
func (s *Server) handleOAuthRevoke(c *gin.Context) {
	if !s.mcpAuthEnabled() {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "MCP connector not configured")
		return
	}
	_ = c.Request.ParseForm()
	if token := c.PostForm("token"); token != "" {
		_ = s.mcpAuth.Revoke(ctx(c), token)
	}
	c.Status(http.StatusOK)
}

// dcrRequest is a Dynamic Client Registration request (RFC 7591) — the
// compatibility fallback plan/mcp.md §3 keeps alongside Client ID Metadata
// Documents.
type dcrRequest struct {
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

func (s *Server) handleOAuthRegister(c *gin.Context) {
	if !s.mcpAuthEnabled() {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "MCP connector not configured")
		return
	}
	var req dcrRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.RedirectURIs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_metadata", "error_description": "redirect_uris is required"})
		return
	}
	client, err := s.mcpAuth.Store.RegisterClient(ctx(c), req.ClientName, req.RedirectURIs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_metadata", "error_description": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"client_id":                  client.ClientID,
		"client_name":                client.ClientName,
		"redirect_uris":              client.RedirectURIs,
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
}
