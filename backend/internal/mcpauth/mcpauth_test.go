package mcpauth_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
	"github.com/yerassyldanay/xchats/backend/internal/store"
	"github.com/yerassyldanay/xchats/backend/migrations"
)

// newTestAuthorizer resets the schema, migrates, seeds an org+user, and
// returns a ready Authorizer — the same pattern kbstore_test.go's newTestKB
// uses.
func newTestAuthorizer(t *testing.T) (*mcpauth.Authorizer, uuid.UUID, uuid.UUID) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping mcpauth DB test")
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
	org, err := st.SeedOrganization(ctx, "xchats")
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}
	user, err := st.SeedUser(ctx, org.ID, "operator@example.com", "hash", "Operator")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(st.Close)

	authz := mcpauth.New(mcpauth.NewStore(st.Pool()), mcpauth.NewEphemeralSigningKey(), mcpauth.Config{
		Issuer: "https://xchats.kz/mcp", Audience: "https://xchats.kz/mcp",
		AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour, AuthCodeTTL: time.Minute,
	})
	return authz, org.ID, user.ID
}

func pkcePair() (verifier, challenge string) {
	verifier = "a-fixed-length-code-verifier-1234567890-abcdefgh"
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return
}

// TestFullAuthorizationCodeFlow exercises the whole happy path plan/mcp.md §3
// describes: register a client, issue a PKCE-bound code, exchange it for a
// token pair, verify the access token resolves the right principal + scopes,
// then refresh and confirm the OLD refresh token is now dead (rotation).
func TestFullAuthorizationCodeFlow(t *testing.T) {
	authz, orgID, userID := newTestAuthorizer(t)
	ctx := context.Background()

	client, err := authz.Store.RegisterClient(ctx, "Test MCP Host", []string{"https://host.example/callback"})
	if err != nil {
		t.Fatalf("register client: %v", err)
	}

	verifier, challenge := pkcePair()
	code, err := authz.Store.IssueAuthorizationCode(ctx, mcpauth.AuthorizationCodeInput{
		ClientID: client.ClientID, RedirectURI: "https://host.example/callback",
		CodeChallenge: challenge, CodeChallengeMethod: "S256",
		UserID: userID, OrganizationID: orgID,
		Scope: "kb:read kb:draft:write", Resource: "https://xchats.kz/mcp",
		TTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("issue code: %v", err)
	}

	tok, err := authz.ExchangeAuthorizationCode(ctx, client.ClientID, "https://host.example/callback", code, verifier)
	if err != nil {
		t.Fatalf("exchange code: %v", err)
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Fatalf("expected both tokens, got %+v", tok)
	}

	principal, err := authz.VerifyAccessToken(ctx, tok.AccessToken)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}
	if principal.UserID != userID || principal.OrganizationID != orgID {
		t.Fatalf("principal mismatch: got user=%s org=%s", principal.UserID, principal.OrganizationID)
	}
	if !principal.HasScope(mcpauth.ScopeKBRead) || !principal.HasScope(mcpauth.ScopeKBDraftWrite) {
		t.Fatalf("expected both granted scopes, got %v", principal.Scopes)
	}
	if principal.HasScope(mcpauth.ScopeMediaWrite) {
		t.Fatalf("media:write was never granted, but principal has it")
	}

	// Replay is rejected: the code is single-use.
	if _, err := authz.ExchangeAuthorizationCode(ctx, client.ClientID, "https://host.example/callback", code, verifier); !errors.Is(err, mcpauth.ErrInvalidGrant) {
		t.Fatalf("expected ErrInvalidGrant on replay, got %v", err)
	}

	// Refresh rotates: the new pair works, and the OLD refresh token is dead.
	oldRefresh := tok.RefreshToken
	tok2, err := authz.RefreshTokenPair(ctx, client.ClientID, oldRefresh)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if tok2.RefreshToken == oldRefresh {
		t.Fatalf("refresh token was not rotated")
	}
	if _, err := authz.RefreshTokenPair(ctx, client.ClientID, oldRefresh); !errors.Is(err, mcpauth.ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken reusing a rotated-away token, got %v", err)
	}

	// Revoking the current refresh token kills it too.
	if err := authz.Revoke(ctx, tok2.RefreshToken); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := authz.RefreshTokenPair(ctx, client.ClientID, tok2.RefreshToken); !errors.Is(err, mcpauth.ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken after revoke, got %v", err)
	}
}

// TestExchangeRejectsWrongPKCEVerifier ensures a mismatched code_verifier is
// rejected — the whole point of PKCE.
func TestExchangeRejectsWrongPKCEVerifier(t *testing.T) {
	authz, orgID, userID := newTestAuthorizer(t)
	ctx := context.Background()
	client, err := authz.Store.RegisterClient(ctx, "Test Host", []string{"https://host.example/callback"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	_, challenge := pkcePair()
	code, err := authz.Store.IssueAuthorizationCode(ctx, mcpauth.AuthorizationCodeInput{
		ClientID: client.ClientID, RedirectURI: "https://host.example/callback",
		CodeChallenge: challenge, CodeChallengeMethod: "S256",
		UserID: userID, OrganizationID: orgID, Scope: "kb:read", TTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("issue code: %v", err)
	}
	if _, err := authz.ExchangeAuthorizationCode(ctx, client.ClientID, "https://host.example/callback", code, "wrong-verifier"); !errors.Is(err, mcpauth.ErrInvalidGrant) {
		t.Fatalf("expected ErrInvalidGrant for wrong verifier, got %v", err)
	}
}

// TestExchangeRejectsRedirectURIMismatch ensures the redirect_uri at exchange
// must match the one supplied at authorize time (open-redirect defense).
func TestExchangeRejectsRedirectURIMismatch(t *testing.T) {
	authz, orgID, userID := newTestAuthorizer(t)
	ctx := context.Background()
	client, err := authz.Store.RegisterClient(ctx, "Test Host", []string{"https://host.example/callback", "https://host.example/other"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	verifier, challenge := pkcePair()
	code, err := authz.Store.IssueAuthorizationCode(ctx, mcpauth.AuthorizationCodeInput{
		ClientID: client.ClientID, RedirectURI: "https://host.example/callback",
		CodeChallenge: challenge, CodeChallengeMethod: "S256",
		UserID: userID, OrganizationID: orgID, Scope: "kb:read", TTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("issue code: %v", err)
	}
	if _, err := authz.ExchangeAuthorizationCode(ctx, client.ClientID, "https://host.example/other", code, verifier); !errors.Is(err, mcpauth.ErrInvalidGrant) {
		t.Fatalf("expected ErrInvalidGrant for redirect_uri mismatch, got %v", err)
	}
}

// TestVerifyAccessToken_RejectsWrongAudience simulates a token minted for a
// different MCP resource being replayed here — the RFC 8707 audience check
// must reject it even though the signature itself is valid.
func TestVerifyAccessToken_RejectsWrongAudience(t *testing.T) {
	key := mcpauth.NewEphemeralSigningKey()
	tok, err := key.Sign(mcpauth.Claims{
		Issuer: "https://xchats.kz/mcp", Audience: "https://someone-elses-server.example/mcp",
		Subject: uuid.New().String(), OrganizationID: uuid.New().String(),
		IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix(), JTI: "x",
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := key.Verify(tok, "https://xchats.kz/mcp", "https://xchats.kz/mcp"); !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for wrong audience, got %v", err)
	}
}

// TestVerifyAccessToken_RejectsExpired confirms an expired-but-otherwise-
// valid token is rejected.
func TestVerifyAccessToken_RejectsExpired(t *testing.T) {
	key := mcpauth.NewEphemeralSigningKey()
	tok, err := key.Sign(mcpauth.Claims{
		Issuer: "iss", Audience: "aud", Subject: uuid.New().String(), OrganizationID: uuid.New().String(),
		IssuedAt: time.Now().Add(-time.Hour).Unix(), ExpiresAt: time.Now().Add(-time.Minute).Unix(), JTI: "x",
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := key.Verify(tok, "iss", "aud"); !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

// TestVerifyAccessToken_RejectsTamperedSignature confirms a flipped byte in
// the signature is rejected — the whole point of asymmetric signing.
func TestVerifyAccessToken_RejectsTamperedSignature(t *testing.T) {
	key := mcpauth.NewEphemeralSigningKey()
	tok, err := key.Sign(mcpauth.Claims{
		Issuer: "iss", Audience: "aud", Subject: uuid.New().String(), OrganizationID: uuid.New().String(),
		IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix(), JTI: "x",
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	// Flip the FIRST character of the base64url signature segment, not the
	// last: a 64-byte signature's raw-base64 encoding ends in a partial
	// 4-character group (64 = 21*3+1), and its final 1-2 characters carry
	// spare/padding bits Go's decoder ignores — so a naive last-character
	// edit can round-trip to the SAME decoded bytes and leave the signature
	// (and thus verification) unchanged, a real flake this fix avoids by
	// construction. The first character of any base64 group has no such
	// ambiguity: every one of its 6 encoded bits is real signature data.
	parts := strings.Split(tok, ".")
	if len(parts) != 3 || len(parts[2]) == 0 {
		t.Fatalf("unexpected token shape: %q", tok)
	}
	first := parts[2][0]
	replacement := byte('A')
	if first == replacement {
		replacement = 'B'
	}
	tamperedSig := string(replacement) + parts[2][1:]
	tampered := parts[0] + "." + parts[1] + "." + tamperedSig
	if tampered == tok {
		t.Fatal("test setup failed to tamper token")
	}
	if _, err := key.Verify(tampered, "iss", "aud"); !errors.Is(err, mcpauth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken for tampered signature, got %v", err)
	}
}

// TestResolveClient_UnregisteredNonURLIsNotFound ensures an opaque,
// unregistered client_id (not an https:// CIMD URL) fails closed rather than
// silently minting an implicit client.
func TestResolveClient_UnregisteredNonURLIsNotFound(t *testing.T) {
	authz, _, _ := newTestAuthorizer(t)
	ctx := context.Background()
	if _, err := authz.Store.ResolveClient(ctx, "some-opaque-unregistered-id", false); !errors.Is(err, mcpauth.ErrClientNotFound) {
		t.Fatalf("expected ErrClientNotFound, got %v", err)
	}
}
