// Package mcpauth is the OAuth 2.1 authorization server behind the MCP
// connector (plan/mcp.md §3 "Authentication"): PKCE-secured authorization
// codes, short-lived signed access tokens, rotated-on-use refresh tokens,
// revocation, Dynamic Client Registration, and Client ID Metadata Document
// (CIMD) support. It never touches the KB itself — see internal/kbstore for
// that; this package only answers "who is calling, for which organization,
// with which scopes."
//
// Access tokens are self-contained EdDSA (Ed25519) JWTs, hand-rolled against
// the stdlib crypto/ed25519 rather than pulling in a JWT dependency — the
// same "no new library for something this small" call the rest of the
// backend already makes (see internal/httpapi's argon2 encoding and
// internal/secretbox's AES-GCM box). A JWT needs exactly three things this
// file provides: a canonical header+payload encoding, a signature, and a
// verifier that rejects anything it didn't produce.
package mcpauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNoSigningKey is returned when no signing key is configured and none
// could be derived — callers surface it as a configuration error.
var ErrNoSigningKey = errors.New("mcpauth: no JWT signing key configured (set MCP_JWT_SIGNING_KEY)")

// ErrInvalidToken covers every way a presented access token can fail to
// verify: bad signature, wrong audience/issuer, expired, malformed. The
// caller (the /mcp handler) treats all of these identically — a 401 with
// the protected-resource WWW-Authenticate challenge, never a detailed reason
// (that would help an attacker fish for which check almost passed).
var ErrInvalidToken = errors.New("mcpauth: invalid access token")

// keyID is fixed rather than derived: this deployment has exactly one
// signing key at a time (KeyVersion-style rotation, like internal/secretbox,
// is future work — see plan/mcp.md's open questions analogue). A stable kid
// lets a client cache the JWKS response across restarts that reuse the same
// configured key.
const keyID = "xchats-mcp-1"

// SigningKey wraps the Ed25519 keypair that signs/verifies access tokens.
type SigningKey struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

// NewSigningKeyFromSeed builds a SigningKey from a 32-byte seed, accepting
// the same three encodings as secretbox.FromEnvValue (64 hex chars, base64,
// or a 32-char literal) so an operator can generate one with
// `openssl rand -hex 32`, exactly like TG_CREDENTIALS_ENC_KEY.
func NewSigningKeyFromSeed(v string) (*SigningKey, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, ErrNoSigningKey
	}
	seed, err := decode32(v)
	if err != nil {
		return nil, fmt.Errorf("mcpauth: MCP_JWT_SIGNING_KEY must decode to 32 bytes (64 hex chars, base64, or a 32-char literal): %w", err)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return &SigningKey{priv: priv, pub: priv.Public().(ed25519.PublicKey)}, nil
}

// NewEphemeralSigningKey generates a random key for dev/test use — tokens it
// signs stop verifying the moment the process restarts.
func NewEphemeralSigningKey() *SigningKey {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		// crypto/rand failing is a fatal environment problem, not a
		// recoverable error path — every other crypto call in this codebase
		// (session ids, argon2 salts) has the same assumption.
		panic("mcpauth: crypto/rand unavailable: " + err.Error())
	}
	return &SigningKey{priv: priv, pub: pub}
}

func decode32(v string) ([]byte, error) {
	if b, err := hex.DecodeString(v); err == nil && len(b) == 32 {
		return b, nil
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(v); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	if len(v) == 32 {
		return []byte(v), nil
	}
	return nil, errors.New("does not decode to 32 bytes")
}

// Claims is the closed set of JWT claims an xchats MCP access token carries
// (plan/mcp.md §3: "A JWT should contain or resolve to user_id,
// organization_id, scopes, audience, issuer, and expiry"). Every field is
// part of the contract — an MCP host never sees or needs anything else.
type Claims struct {
	Issuer         string   `json:"iss"`
	Audience       string   `json:"aud"`
	Subject        string   `json:"sub"` // user_id
	OrganizationID string   `json:"org_id"`
	ClientID       string   `json:"client_id"`
	Scopes         []string `json:"scope_list"`
	IssuedAt       int64    `json:"iat"`
	ExpiresAt      int64    `json:"exp"`
	JTI            string   `json:"jti"`
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

// Sign produces a compact EdDSA JWT: base64url(header).base64url(payload).base64url(signature).
func (k *SigningKey) Sign(c Claims) (string, error) {
	hdr, err := json.Marshal(jwtHeader{Alg: "EdDSA", Typ: "JWT", Kid: keyID})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signingInput := b64(hdr) + "." + b64(payload)
	sig := ed25519.Sign(k.priv, []byte(signingInput))
	return signingInput + "." + b64(sig), nil
}

// Verify checks the signature, then the audience/issuer/expiry, returning the
// parsed Claims only when every check passes.
func (k *SigningKey) Verify(token, wantIssuer, wantAudience string) (Claims, error) {
	c, err := k.verifyIgnoringExpiry(token, wantIssuer, wantAudience)
	if err != nil {
		return Claims{}, err
	}
	if time.Now().Unix() >= c.ExpiresAt {
		return Claims{}, ErrInvalidToken
	}
	return c, nil
}

// verifyIgnoringExpiry checks everything Verify does except expiry — used
// only by Revoke, which must be able to denylist a token's jti even after it
// has already expired (a harmless no-op) or is about to.
func (k *SigningKey) verifyIgnoringExpiry(token, wantIssuer, wantAudience string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	sig, err := unb64(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	if !ed25519.Verify(k.pub, []byte(parts[0]+"."+parts[1]), sig) {
		return Claims{}, ErrInvalidToken
	}
	payload, err := unb64(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if c.Issuer != wantIssuer || c.Audience != wantAudience {
		return Claims{}, ErrInvalidToken
	}
	return c, nil
}

// JWK is one entry of a JSON Web Key Set (RFC 7517), the OKP/Ed25519 shape.
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

// JWKSDocument returns the {"keys":[...]} body for GET /oauth/jwks.json.
func (k *SigningKey) JWKSDocument() map[string]any {
	return map[string]any{
		"keys": []JWK{{
			Kty: "OKP", Crv: "Ed25519", X: b64(k.pub), Use: "sig", Alg: "EdDSA", Kid: keyID,
		}},
	}
}

func b64(b []byte) string            { return base64.RawURLEncoding.EncodeToString(b) }
func unb64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
