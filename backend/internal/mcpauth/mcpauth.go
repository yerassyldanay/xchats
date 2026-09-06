package mcpauth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Config is the resolved configuration Authorizer needs — a narrow view of
// config.Config so this package does not import it (avoiding an import edge
// from a low-level auth package back up to the app's config type).
type Config struct {
	Issuer   string // config.Config.MCPResourceURL() — the AS also names itself as this MCP server
	Audience string // config.Config.MCPResourceURL() — RFC 8707 resource-indicator binding
	// IssuerFunc/AudienceFunc make the advertised endpoint runtime-aware for
	// embedded tunnels. Tests and fixed deployments can keep using the string
	// fields; when present, the functions are the live source of truth.
	IssuerFunc      func() string
	AudienceFunc    func() string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	AuthCodeTTL     time.Duration
}

// Authorizer is the composed OAuth 2.1 authorization server: DB-backed
// client/code/token state (Store) plus the signing key that mints/verifies
// access tokens (SigningKey). This is what internal/httpapi wires into the
// /oauth/* and /mcp routes.
type Authorizer struct {
	Store *Store
	Key   *SigningKey
	Cfg   Config
}

// New builds an Authorizer.
func New(store *Store, key *SigningKey, cfg Config) *Authorizer {
	return &Authorizer{Store: store, Key: key, Cfg: cfg}
}

func (a *Authorizer) Issuer() string {
	if a.Cfg.IssuerFunc != nil {
		return a.Cfg.IssuerFunc()
	}
	return a.Cfg.Issuer
}

func (a *Authorizer) Audience() string {
	if a.Cfg.AudienceFunc != nil {
		return a.Cfg.AudienceFunc()
	}
	return a.Cfg.Audience
}

// IssueTokenPair mints a fresh access JWT + refresh token for a grant that
// has already been authenticated and authorized (a consumed authorization
// code, or a rotated refresh token) — never called with a caller-supplied
// organization_id that has not itself just been validated by the caller.
func (a *Authorizer) IssueTokenPair(ctx context.Context, clientID string, userID, orgID uuid.UUID, scope, resource string) (TokenResponse, error) {
	now := time.Now()
	jti := randomID(16)
	issuer, audience := a.Issuer(), a.Audience()
	if resource != "" && resource != audience {
		return TokenResponse{}, ErrInvalidGrant
	}
	access, err := a.Key.Sign(Claims{
		Issuer: issuer, Audience: audience, Subject: userID.String(),
		OrganizationID: orgID.String(), ClientID: clientID, Scopes: ParseScope(scope),
		IssuedAt: now.Unix(), ExpiresAt: now.Add(a.Cfg.AccessTokenTTL).Unix(), JTI: jti,
	})
	if err != nil {
		return TokenResponse{}, fmt.Errorf("mcpauth: sign access token: %w", err)
	}
	refresh, err := a.Store.IssueRefreshToken(ctx, clientID, userID, orgID, scope, audience, a.Cfg.RefreshTokenTTL)
	if err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: int(a.Cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refresh, Scope: scope,
	}, nil
}

// ExchangeAuthorizationCode implements the token endpoint's
// grant_type=authorization_code path end to end: consume the code (PKCE +
// single-use + client/redirect + resource binding all checked), then mint a
// token pair. resource is the token request's own optional RFC 8707
// parameter — re-checked against what was bound at authorize time, not
// merely carried through unchecked.
func (a *Authorizer) ExchangeAuthorizationCode(ctx context.Context, clientID, redirectURI, resource, code, codeVerifier string) (TokenResponse, error) {
	row, err := a.Store.ConsumeAuthorizationCode(ctx, clientID, redirectURI, resource, code, codeVerifier)
	if err != nil {
		return TokenResponse{}, err
	}
	if row.Resource != "" && row.Resource != a.Audience() {
		return TokenResponse{}, ErrInvalidGrant
	}
	return a.IssueTokenPair(ctx, clientID, row.UserID, row.OrganizationID, row.Scope, row.Resource)
}

// RefreshTokenPair implements grant_type=refresh_token: rotate the refresh
// token and mint a fresh access token alongside it.
func (a *Authorizer) RefreshTokenPair(ctx context.Context, clientID, refreshToken string) (TokenResponse, error) {
	row, newRefresh, err := a.Store.RotateRefreshToken(ctx, clientID, refreshToken, a.Cfg.RefreshTokenTTL)
	if err != nil {
		return TokenResponse{}, err
	}
	now := time.Now()
	jti := randomID(16)
	issuer, audience := a.Issuer(), a.Audience()
	if row.Resource != "" && row.Resource != audience {
		return TokenResponse{}, ErrInvalidGrant
	}
	access, err := a.Key.Sign(Claims{
		Issuer: issuer, Audience: audience, Subject: row.UserID.String(),
		OrganizationID: row.OrganizationID.String(), ClientID: clientID, Scopes: ParseScope(row.Scope),
		IssuedAt: now.Unix(), ExpiresAt: now.Add(a.Cfg.AccessTokenTTL).Unix(), JTI: jti,
	})
	if err != nil {
		return TokenResponse{}, fmt.Errorf("mcpauth: sign access token: %w", err)
	}
	return TokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: int(a.Cfg.AccessTokenTTL.Seconds()),
		RefreshToken: newRefresh, Scope: row.Scope,
	}, nil
}

// VerifyAccessToken checks the JWT's signature/issuer/audience/expiry, then
// the denylist. It returns a cryptographically-trustworthy Principal, but
// per plan/mcp.md §3 ("The backend must still verify that the user is active
// and belongs to the bound organization on every request") the caller MUST
// additionally re-check org membership against the live xchats.users /
// organization_users tables (internal/store) before trusting Principal for a
// KB write — this package intentionally has no dependency on internal/store
// to make that composition explicit at the call site (internal/httpapi).
func (a *Authorizer) VerifyAccessToken(ctx context.Context, token string) (Principal, error) {
	claims, err := a.Key.Verify(token, a.Issuer(), a.Audience())
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	denied, err := a.Store.IsJTIDenied(ctx, claims.JTI)
	if err != nil {
		return Principal{}, err
	}
	if denied {
		return Principal{}, ErrInvalidToken
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	orgID, err := uuid.Parse(claims.OrganizationID)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	return Principal{UserID: userID, OrganizationID: orgID, ClientID: claims.ClientID, Scopes: claims.Scopes}, nil
}

// Revoke implements POST /oauth/revoke (RFC 7009): it always looks like
// success to the caller (a token-hint mismatch or an unknown token is not
// distinguishable from "already revoked" — RFC 7009 §2.2). It tries the
// refresh-token table first (the common case — MCP hosts revoke on
// disconnect), then falls back to treating the token as an access-token JWT
// and denylisting its jti when the signature verifies.
func (a *Authorizer) Revoke(ctx context.Context, token string) error {
	if err := a.Store.RevokeRefreshToken(ctx, token); err != nil {
		return err
	}
	if claims, err := a.Key.verifyIgnoringExpiry(token, a.Issuer(), a.Audience()); err == nil {
		return a.Store.DenylistJTI(ctx, claims.JTI, time.Unix(claims.ExpiresAt, 0))
	}
	return nil
}
