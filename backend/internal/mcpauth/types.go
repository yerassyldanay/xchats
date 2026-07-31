package mcpauth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// The three scopes plan/mcp.md §3 defines. There is no wildcard: a client
// requests exactly the scopes it wants and the consent page shows exactly
// those to the user.
const (
	ScopeKBRead       = "kb:read"
	ScopeKBDraftWrite = "kb:draft:write"
	ScopeMediaWrite   = "media:write"
)

// AllScopes is the closed, ordered vocabulary — used both to validate a
// requested scope string and to render the consent page's checklist.
var AllScopes = []string{ScopeKBRead, ScopeKBDraftWrite, ScopeMediaWrite}

// ParseScope splits a space-separated OAuth scope string into its known
// members, dropping anything unrecognized (fail-closed: an unknown scope
// requested is simply never granted, rather than silently passed through).
// Used only where the scope has already been validated (JWT claims, an
// already-consumed refresh token) — a NEW client-supplied scope string
// (authorize, the decision form) must go through ValidateScope instead, so a
// typo'd scope is rejected rather than silently narrowed to nothing.
func ParseScope(s string) []string {
	var out []string
	known := map[string]bool{}
	for _, sc := range AllScopes {
		known[sc] = true
	}
	for _, tok := range strings.Fields(s) {
		if known[tok] {
			out = append(out, tok)
		}
	}
	return out
}

// ErrInvalidScope means a requested scope string named a token outside
// AllScopes.
var ErrInvalidScope = errors.New("mcpauth: invalid scope")

// ValidateScope rejects a scope string containing any unrecognized token,
// returning the parsed list only when every token is known. An empty string
// is valid (it defaults to AllScopes at the call site) — this only guards
// against a scope the client actually supplied being silently narrowed.
func ValidateScope(s string) ([]string, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	known := map[string]bool{}
	for _, sc := range AllScopes {
		known[sc] = true
	}
	var out []string
	for _, tok := range strings.Fields(s) {
		if !known[tok] {
			return nil, fmt.Errorf("%w: %q", ErrInvalidScope, tok)
		}
		out = append(out, tok)
	}
	return out, nil
}

// FormatScope re-joins a scope list into the standard space-separated form.
func FormatScope(scopes []string) string { return strings.Join(scopes, " ") }

var (
	// ErrClientNotFound means the client_id does not resolve — neither a
	// registered client nor (for an https:// client_id) a fetchable CIMD.
	ErrClientNotFound = errors.New("mcpauth: unknown client")
	// ErrInvalidRedirectURI means the given redirect_uri is not in the
	// client's registered set.
	ErrInvalidRedirectURI = errors.New("mcpauth: redirect_uri not registered for this client")
	// ErrInvalidGrant covers a bad/expired/consumed authorization code, a bad
	// PKCE verifier, or a client_id/redirect_uri mismatch at the token
	// endpoint — deliberately one error for all of these (RFC 6749 §5.2's
	// own "invalid_grant", which does not distinguish the reason either).
	ErrInvalidGrant = errors.New("mcpauth: invalid grant")
	// ErrInvalidRefreshToken covers an unknown, revoked, or expired refresh
	// token.
	ErrInvalidRefreshToken = errors.New("mcpauth: invalid refresh token")
	// ErrUnauthorizedPrincipal means the access token verified cryptographically
	// but the user/organization binding it names no longer holds (the user
	// was removed from the organization since the token was issued).
	ErrUnauthorizedPrincipal = errors.New("mcpauth: user no longer belongs to the bound organization")
)

// Client is a registered OAuth client — either self-registered via Dynamic
// Client Registration or resolved from a fetched Client ID Metadata
// Document (plan/mcp.md §3's "Support Client ID Metadata Documents. Dynamic
// client registration can remain a compatibility fallback").
type Client struct {
	ClientID     string
	ClientName   string
	RedirectURIs []string
	Source       string // "dcr" | "cimd"
}

// AllowsRedirect reports whether uri is one of the client's registered
// redirect URIs — an exact string match, per OAuth 2.1's tightened rule (no
// prefix/partial matching, which historically enabled open-redirect attacks).
func (c Client) AllowsRedirect(uri string) bool {
	for _, u := range c.RedirectURIs {
		if u == uri {
			return true
		}
	}
	return false
}

// Principal is the authenticated identity + authorization an access token
// carries — resolved once per request by VerifyAccessToken and then read by
// every MCP tool handler. organization_id is bound at authorization time;
// nothing downstream ever accepts a caller-supplied organization_id instead
// (plan/mcp.md §3).
type Principal struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	ClientID       string
	Scopes         []string
}

// HasScope reports whether the principal's token was granted scope.
func (p Principal) HasScope(scope string) bool {
	for _, s := range p.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// TokenResponse is the JSON body POST /oauth/token returns on success (RFC
// 6749 §5.1). RefreshToken is empty on a client_credentials-less server that
// chooses not to rotate — here it is always present.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}
