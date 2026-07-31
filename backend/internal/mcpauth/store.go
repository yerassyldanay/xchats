package mcpauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store owns the four mcp_oauth_* tables (migration 0014) — client
// registrations, authorization codes, refresh tokens, and the access-token
// denylist. It never touches xchats.users/organizations directly: resolving
// "who is this user" and "does this user still belong to this org" is the
// HTTP layer's job (internal/httpapi), composed with internal/store — see
// Principal's doc comment.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore builds a Store over an existing pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// ---------------------------------------------------------------------------
// Clients — Dynamic Client Registration + Client ID Metadata Documents.
// ---------------------------------------------------------------------------

// RegisterClient performs Dynamic Client Registration (RFC 7591): every
// registered client is public (PKCE-secured, no client_secret — plan/mcp.md
// §3's OAuth 2.1 + PKCE model has no confidential-client story for an MCP
// host). redirectURIs must be non-empty and each pass validateRedirectURI.
func (s *Store) RegisterClient(ctx context.Context, clientName string, redirectURIs []string) (Client, error) {
	if len(redirectURIs) == 0 {
		return Client{}, errors.New("mcpauth: redirect_uris required")
	}
	for _, ru := range redirectURIs {
		if err := validateRedirectURI(ru); err != nil {
			return Client{}, err
		}
	}
	clientID := "dcr_" + randomID(16)
	_, err := s.pool.Exec(ctx, `INSERT INTO xchats.mcp_oauth_clients
		(client_id, client_name, redirect_uris, registration_source, token_endpoint_auth_method)
		VALUES ($1,$2,$3,'dcr','none')`,
		clientID, clientName, redirectURIs)
	if err != nil {
		return Client{}, fmt.Errorf("mcpauth: register client: %w", err)
	}
	return Client{ClientID: clientID, ClientName: clientName, RedirectURIs: redirectURIs, Source: "dcr"}, nil
}

// ResolveClient looks up a previously registered client; for a client_id that
// is itself an https:// URL and not already cached, it fetches and caches the
// Client ID Metadata Document (plan/mcp.md §3). allowPrivateHosts is forwarded
// to the CIMD fetch's SSRF guard (config.KBAllowPrivateFetch — a trusted
// self-hosted-only escape hatch).
func (s *Store) ResolveClient(ctx context.Context, clientID string, allowPrivateHosts bool) (Client, error) {
	c, err := s.getClient(ctx, clientID)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Client{}, err
	}
	if !looksLikeCIMDClientID(clientID) {
		return Client{}, ErrClientNotFound
	}
	fetched, ferr := FetchCIMD(ctx, clientID, allowPrivateHosts)
	if ferr != nil {
		return Client{}, fmt.Errorf("%w: %s", ErrClientNotFound, ferr)
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO xchats.mcp_oauth_clients
		(client_id, client_name, redirect_uris, registration_source, token_endpoint_auth_method)
		VALUES ($1,$2,$3,'cimd','none')
		ON CONFLICT (client_id) DO UPDATE SET
			client_name=EXCLUDED.client_name, redirect_uris=EXCLUDED.redirect_uris, updated_at=now()`,
		fetched.ClientID, fetched.ClientName, fetched.RedirectURIs); err != nil {
		return Client{}, fmt.Errorf("mcpauth: cache CIMD client: %w", err)
	}
	return fetched, nil
}

func (s *Store) getClient(ctx context.Context, clientID string) (Client, error) {
	var c Client
	err := s.pool.QueryRow(ctx, `SELECT client_id, client_name, redirect_uris, registration_source
		FROM xchats.mcp_oauth_clients WHERE client_id = $1`, clientID).
		Scan(&c.ClientID, &c.ClientName, &c.RedirectURIs, &c.Source)
	return c, err
}

// ---------------------------------------------------------------------------
// Authorization codes — single-use, PKCE-bound, short-lived.
// ---------------------------------------------------------------------------

// AuthorizationCodeInput is what /oauth/authorize stages once the user has
// authenticated, picked an organization, and granted the requested scopes.
type AuthorizationCodeInput struct {
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	UserID              uuid.UUID
	OrganizationID      uuid.UUID
	Scope               string
	Resource            string
	TTL                 time.Duration
}

// IssueAuthorizationCode generates a fresh code, stores only its sha256 hash
// (the raw code is a bearer secret and is never persisted), and returns the
// raw code for the one redirect back to the client.
func (s *Store) IssueAuthorizationCode(ctx context.Context, in AuthorizationCodeInput) (string, error) {
	code := randomID(32)
	_, err := s.pool.Exec(ctx, `INSERT INTO xchats.mcp_authorization_codes
		(code_hash, client_id, redirect_uri, code_challenge, code_challenge_method,
		 user_id, organization_id, scope, resource, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		sha256Hex(code), in.ClientID, in.RedirectURI, in.CodeChallenge, orDefault(in.CodeChallengeMethod, "S256"),
		in.UserID, in.OrganizationID, in.Scope, in.Resource, time.Now().Add(in.TTL))
	if err != nil {
		return "", fmt.Errorf("mcpauth: issue authorization code: %w", err)
	}
	return code, nil
}

// authorizationCodeRow is the resolved, still-valid code's grant data.
type authorizationCodeRow struct {
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	UserID              uuid.UUID
	OrganizationID      uuid.UUID
	Scope               string
	Resource            string
}

// ConsumeAuthorizationCode validates and atomically consumes a code — a
// second exchange attempt (replay) always fails, per OAuth 2.1's single-use
// requirement. clientID and redirectURI must match exactly what the code was
// issued for; codeVerifier is checked against the stored code_challenge.
// resource (RFC 8707) is optional at the token endpoint — a client may repeat
// it from the authorize request, and if it does, it must match exactly what
// was bound there; omitting it is not an error (the code's own bound resource
// still governs the minted token).
func (s *Store) ConsumeAuthorizationCode(ctx context.Context, clientID, redirectURI, resource, code, codeVerifier string) (authorizationCodeRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return authorizationCodeRow{}, err
	}
	defer tx.Rollback(ctx)

	var row authorizationCodeRow
	var expiresAt time.Time
	var consumedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT client_id, redirect_uri, code_challenge, code_challenge_method,
		user_id, organization_id, scope, resource, expires_at, consumed_at
		FROM xchats.mcp_authorization_codes WHERE code_hash = $1 FOR UPDATE`, sha256Hex(code)).
		Scan(&row.ClientID, &row.RedirectURI, &row.CodeChallenge, &row.CodeChallengeMethod,
			&row.UserID, &row.OrganizationID, &row.Scope, &row.Resource, &expiresAt, &consumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return authorizationCodeRow{}, ErrInvalidGrant
	}
	if err != nil {
		return authorizationCodeRow{}, err
	}
	if consumedAt != nil || time.Now().After(expiresAt) ||
		row.ClientID != clientID || row.RedirectURI != redirectURI ||
		(resource != "" && row.Resource != resource) {
		return authorizationCodeRow{}, ErrInvalidGrant
	}
	if !VerifyPKCE(codeVerifier, row.CodeChallenge, row.CodeChallengeMethod) {
		return authorizationCodeRow{}, ErrInvalidGrant
	}
	if _, err := tx.Exec(ctx, `UPDATE xchats.mcp_authorization_codes SET consumed_at = now() WHERE code_hash = $1`,
		sha256Hex(code)); err != nil {
		return authorizationCodeRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return authorizationCodeRow{}, err
	}
	return row, nil
}

// ---------------------------------------------------------------------------
// Refresh tokens — rotated on every use.
// ---------------------------------------------------------------------------

type refreshTokenRow struct {
	ClientID       string
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Scope          string
	Resource       string
}

// IssueRefreshToken mints and stores a fresh refresh token, returning the raw
// value (only its hash is persisted).
func (s *Store) IssueRefreshToken(ctx context.Context, clientID string, userID, orgID uuid.UUID, scope, resource string, ttl time.Duration) (string, error) {
	token := randomID(32)
	_, err := s.pool.Exec(ctx, `INSERT INTO xchats.mcp_refresh_tokens
		(token_hash, client_id, user_id, organization_id, scope, resource, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		sha256Hex(token), clientID, userID, orgID, scope, resource, time.Now().Add(ttl))
	if err != nil {
		return "", fmt.Errorf("mcpauth: issue refresh token: %w", err)
	}
	return token, nil
}

// RotateRefreshToken validates an incoming refresh token (must be
// unrevoked, unexpired, and issued to clientID), revokes it, and issues a
// replacement — refresh token rotation, so a stolen-and-later-replayed old
// token is detectably dead rather than silently still valid.
func (s *Store) RotateRefreshToken(ctx context.Context, clientID, token string, newTTL time.Duration) (refreshTokenRow, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return refreshTokenRow{}, "", err
	}
	defer tx.Rollback(ctx)

	var row refreshTokenRow
	var expiresAt time.Time
	var revokedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT client_id, user_id, organization_id, scope, resource, expires_at, revoked_at
		FROM xchats.mcp_refresh_tokens WHERE token_hash = $1 FOR UPDATE`, sha256Hex(token)).
		Scan(&row.ClientID, &row.UserID, &row.OrganizationID, &row.Scope, &row.Resource, &expiresAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return refreshTokenRow{}, "", ErrInvalidRefreshToken
	}
	if err != nil {
		return refreshTokenRow{}, "", err
	}
	if revokedAt != nil || time.Now().After(expiresAt) || row.ClientID != clientID {
		return refreshTokenRow{}, "", ErrInvalidRefreshToken
	}

	newToken := randomID(32)
	if _, err := tx.Exec(ctx, `UPDATE xchats.mcp_refresh_tokens SET revoked_at = now(), replaced_by = $2
		WHERE token_hash = $1`, sha256Hex(token), sha256Hex(newToken)); err != nil {
		return refreshTokenRow{}, "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO xchats.mcp_refresh_tokens
		(token_hash, client_id, user_id, organization_id, scope, resource, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		sha256Hex(newToken), row.ClientID, row.UserID, row.OrganizationID, row.Scope, row.Resource,
		time.Now().Add(newTTL)); err != nil {
		return refreshTokenRow{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return refreshTokenRow{}, "", err
	}
	return row, newToken, nil
}

// RevokeRefreshToken marks a refresh token revoked (POST /oauth/revoke, or a
// user-initiated disconnect). A token that does not exist is treated as
// already revoked (RFC 7009 §2.2: revocation is idempotent and never signals
// whether a token existed).
func (s *Store) RevokeRefreshToken(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `UPDATE xchats.mcp_refresh_tokens SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL`, sha256Hex(token))
	return err
}

// ---------------------------------------------------------------------------
// Access-token denylist — explicit early revocation of an otherwise
// self-contained, stateless JWT.
// ---------------------------------------------------------------------------

// DenylistJTI records an early revocation; expiresAt should mirror the
// token's own exp so a cleanup job can eventually prune the row.
func (s *Store) DenylistJTI(ctx context.Context, jti string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO xchats.mcp_access_token_denylist (jti, expires_at)
		VALUES ($1,$2) ON CONFLICT (jti) DO NOTHING`, jti, expiresAt)
	return err
}

// IsJTIDenied reports whether jti was explicitly revoked before its natural
// expiry.
func (s *Store) IsJTIDenied(ctx context.Context, jti string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM xchats.mcp_access_token_denylist WHERE jti = $1)`,
		jti).Scan(&exists)
	return exists, err
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func randomID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("mcpauth: crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func sha256Hex(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
