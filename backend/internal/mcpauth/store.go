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

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	sqlitemigrations "github.com/yerassyldanay/xchats/backend/migrations/sqlite"
)

// Store owns the four mcp_oauth_* tables (migration 0005) — client
// registrations, authorization codes, refresh tokens, and the access-token
// denylist. It never touches users/organizations directly: resolving
// "who is this user" and "does this user still belong to this org" is the
// HTTP layer's job (internal/httpapi), composed with internal/store — see
// Principal's doc comment.
type Store struct {
	db *dbx.DB
}

// NewStore opens (or attaches to an already shared-by-path) dbPath and
// returns a ready Store — see internal/store.New's doc comment for how the
// persistence packages end up sharing one physical connection via
// internal/dbx.Open.
func NewStore(ctx context.Context, dbPath string) (*Store, error) {
	db, err := dbx.Open(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := dbx.RunMigrations(ctx, db, sqlitemigrations.FS); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases this Store's reference to the pool.
func (s *Store) Close() { _ = s.db.Close() }

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
	_, err := s.db.Exec(ctx, `INSERT INTO mcp_oauth_clients
		(client_id, client_name, redirect_uris, registration_source, token_endpoint_auth_method)
		VALUES ($1,$2,$3,'dcr','none')`,
		clientID, clientName, dbx.StringArray(redirectURIs))
	if err != nil {
		return Client{}, fmt.Errorf("mcpauth: register client: %w", err)
	}
	return Client{ClientID: clientID, ClientName: clientName, RedirectURIs: redirectURIs, Source: "dcr"}, nil
}

// ResolveClient looks up a previously registered client; for a client_id that
// is itself an https:// URL and not already cached, it fetches and caches the
// Client ID Metadata Document (plan/mcp.md §3). CIMD fetches are always
// restricted to public hosts because clientID is remote-client-controlled.
func (s *Store) ResolveClient(ctx context.Context, clientID string) (Client, error) {
	c, err := s.getClient(ctx, clientID)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, dbx.ErrNoRows) {
		return Client{}, err
	}
	if !looksLikeCIMDClientID(clientID) {
		return Client{}, ErrClientNotFound
	}
	fetched, ferr := FetchCIMD(ctx, clientID)
	if ferr != nil {
		return Client{}, fmt.Errorf("%w: %s", ErrClientNotFound, ferr)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO mcp_oauth_clients
		(client_id, client_name, redirect_uris, registration_source, token_endpoint_auth_method)
		VALUES ($1,$2,$3,'cimd','none')
		ON CONFLICT (client_id) DO UPDATE SET
			client_name=EXCLUDED.client_name, redirect_uris=EXCLUDED.redirect_uris, updated_at=strftime('%Y-%m-%d %H:%M:%f','now')`,
		fetched.ClientID, fetched.ClientName, dbx.StringArray(fetched.RedirectURIs)); err != nil {
		return Client{}, fmt.Errorf("mcpauth: cache CIMD client: %w", err)
	}
	return fetched, nil
}

func (s *Store) getClient(ctx context.Context, clientID string) (Client, error) {
	var c Client
	err := s.db.QueryRow(ctx, `SELECT client_id, client_name, redirect_uris, registration_source
		FROM mcp_oauth_clients WHERE client_id = $1`, clientID).
		Scan(&c.ClientID, &c.ClientName, (*dbx.StringArray)(&c.RedirectURIs), &c.Source)
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
	_, err := s.db.Exec(ctx, `INSERT INTO mcp_authorization_codes
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
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return authorizationCodeRow{}, err
	}
	defer tx.Rollback(ctx)

	// Claim the code and read it in ONE statement. The "is it still unclaimed?"
	// test has to be part of the write, not a separate SELECT before it:
	// read-then-write leaves a window where two concurrent token requests both
	// observe consumed_at IS NULL, both pass PKCE, and both mint a token off one
	// code — an OAuth 2.1 single-use violation. Matching zero rows here means
	// the code does not exist or someone else already claimed it; either way the
	// answer is the same deliberately indistinguishable ErrInvalidGrant.
	//
	// Validation still runs after the claim, and a failure still rolls the whole
	// transaction back, so a code rejected for a bad client/redirect/PKCE is
	// left unconsumed exactly as before. Only the race is gone.
	var row authorizationCodeRow
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `UPDATE mcp_authorization_codes
		SET consumed_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE code_hash = $1 AND consumed_at IS NULL
		RETURNING client_id, redirect_uri, code_challenge, code_challenge_method,
			user_id, organization_id, scope, resource, expires_at`, sha256Hex(code)).
		Scan(&row.ClientID, &row.RedirectURI, &row.CodeChallenge, &row.CodeChallengeMethod,
			&row.UserID, &row.OrganizationID, &row.Scope, &row.Resource, &expiresAt)
	if errors.Is(err, dbx.ErrNoRows) {
		return authorizationCodeRow{}, ErrInvalidGrant
	}
	if err != nil {
		return authorizationCodeRow{}, err
	}
	if time.Now().After(expiresAt) ||
		row.ClientID != clientID || row.RedirectURI != redirectURI ||
		(resource != "" && row.Resource != resource) {
		return authorizationCodeRow{}, ErrInvalidGrant
	}
	if !VerifyPKCE(codeVerifier, row.CodeChallenge, row.CodeChallengeMethod) {
		return authorizationCodeRow{}, ErrInvalidGrant
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
	_, err := s.db.Exec(ctx, `INSERT INTO mcp_refresh_tokens
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
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return refreshTokenRow{}, "", err
	}
	defer tx.Rollback(ctx)

	// Revoke and read in ONE statement, for the same reason
	// ConsumeAuthorizationCode does — see its comment. Read-then-write here
	// means two concurrent refreshes both see revoked_at IS NULL and both
	// rotate, turning one refresh token into two live ones and defeating the
	// point of rotation (a replayed stolen token should be detectably dead).
	// Zero rows matched means unknown or already-rotated: ErrInvalidRefreshToken
	// either way. Validation after the claim still rolls back on failure, so an
	// expired or wrong-client token is left unrevoked exactly as before.
	newToken := randomID(32)
	var row refreshTokenRow
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `UPDATE mcp_refresh_tokens
		SET revoked_at = strftime('%Y-%m-%d %H:%M:%f','now'), replaced_by = $2
		WHERE token_hash = $1 AND revoked_at IS NULL
		RETURNING client_id, user_id, organization_id, scope, resource, expires_at`,
		sha256Hex(token), sha256Hex(newToken)).
		Scan(&row.ClientID, &row.UserID, &row.OrganizationID, &row.Scope, &row.Resource, &expiresAt)
	if errors.Is(err, dbx.ErrNoRows) {
		return refreshTokenRow{}, "", ErrInvalidRefreshToken
	}
	if err != nil {
		return refreshTokenRow{}, "", err
	}
	if time.Now().After(expiresAt) || row.ClientID != clientID {
		return refreshTokenRow{}, "", ErrInvalidRefreshToken
	}

	if _, err := tx.Exec(ctx, `INSERT INTO mcp_refresh_tokens
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
	_, err := s.db.Exec(ctx, `UPDATE mcp_refresh_tokens SET revoked_at = strftime('%Y-%m-%d %H:%M:%f','now')
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
	_, err := s.db.Exec(ctx, `INSERT INTO mcp_access_token_denylist (jti, expires_at)
		VALUES ($1,$2) ON CONFLICT (jti) DO NOTHING`, jti, expiresAt)
	return err
}

// IsJTIDenied reports whether jti was explicitly revoked before its natural
// expiry.
func (s *Store) IsJTIDenied(ctx context.Context, jti string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM mcp_access_token_denylist WHERE jti = $1)`,
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
