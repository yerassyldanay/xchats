// mcp_discovery.go, mcp_oauth.go, mcp.go, and mcp_upload.go wire the MCP
// connector (plan/mcp.md) into the existing Gin router. They are additive:
// every route here is new (POST /mcp, /.well-known/*, /oauth/*), and none of
// the existing /xchats/api/v1 surface changes shape.
package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
)

// mcpAuthEnabled reports whether the MCP connector's OAuth layer was wired
// (main.go skips it when MCP_JWT_SIGNING_KEY resolution fails hard — it
// never should in a real deployment, but a route handler must not panic on
// a nil dependency during local dev without full config).
func (s *Server) mcpAuthEnabled() bool { return s.mcpAuth != nil && s.mcpServer != nil }

func (s *Server) handleProtectedResourceMetadata(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"resource":                 s.cfg.MCPResourceURL(),
		"authorization_servers":    []string{s.mcpIssuer()},
		"bearer_methods_supported": []string{"header"},
		"scopes_supported":         mcpauth.AllScopes,
	})
}

func (s *Server) handleAuthorizationServerMetadata(c *gin.Context) {
	base := s.mcpIssuer()
	c.JSON(http.StatusOK, gin.H{
		"issuer":                                base,
		"authorization_endpoint":                base + "/oauth/authorize",
		"token_endpoint":                        base + "/oauth/token",
		"registration_endpoint":                 base + "/oauth/register",
		"revocation_endpoint":                   base + "/oauth/revoke",
		"jwks_uri":                              base + "/oauth/jwks.json",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                      mcpauth.AllScopes,
		// client_id_metadata_document_supported advertises that a client may
		// self-register by naming an https:// URL as its client_id (served
		// AT that URL) instead of calling POST /oauth/register first — this
		// server already resolves that shape (mcpauth.FetchCIMD /
		// Store.ResolveClient); this field just makes the capability
		// discoverable. Field name is this ecosystem's current emerging
		// convention — best-effort pending a real interop pass, same caveat
		// as mcpauth/cimd.go's own doc comment.
		"client_id_metadata_document_supported": true,
	})
}

func (s *Server) handleJWKS(c *gin.Context) {
	if !s.mcpAuthEnabled() {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "MCP connector not configured")
		return
	}
	c.JSON(http.StatusOK, s.mcpAuth.Key.JWKSDocument())
}

// mcpIssuer is the bare origin the authorization server identifies itself
// as (RFC 8414's "issuer") — distinct from MCPResourceURL(), which is the
// specific /mcp RESOURCE the access token's audience is bound to (RFC 8707).
func (s *Server) mcpIssuer() string {
	return strings.TrimRight(s.cfg.APIBaseURL, "/")
}

// unauthorizedMCP answers 401 with the WWW-Authenticate challenge pointing
// at protected-resource metadata (plan/mcp.md §3's "First connection" step
// 3) — the standard discovery trigger every compliant MCP host follows.
func (s *Server) unauthorizedMCP(c *gin.Context) {
	c.Header("WWW-Authenticate", `Bearer resource_metadata="`+s.mcpIssuer()+`/.well-known/oauth-protected-resource"`)
	c.AbortWithStatus(http.StatusUnauthorized)
}
