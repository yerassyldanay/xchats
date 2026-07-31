package mcpauth

import (
	"time"

	"github.com/google/uuid"
)

// ReviewHandoffAudience is the fixed JWT audience a review-handoff token is
// bound to — distinct from Config.Audience (the MCP resource access tokens
// are bound to), so a token minted for one purpose can never be replayed as
// the other even though both are signed by the same key.
const ReviewHandoffAudience = "xchats-review"

// ReviewHandoffSigner mints and verifies the short-lived, one-time signed
// handoff a tool result hands the KB Manager widget for its "Review and
// publish in Xchats" link (plan Task 15: a user in multiple organizations who
// authorized the connector for org B must land on /playground scoped to B,
// not whichever org a single-user-default would otherwise pick). It reuses
// the SAME Ed25519 JWT engine access tokens use — Claims already carries
// everything this needs (user_id, organization_id, audience, expiry, jti) —
// bound to a distinct audience so the two token kinds can never be confused.
// One-time consumption is NOT enforced here: it reuses the
// mcp_access_token_denylist jti-tracking table (Store.IsJTIDenied /
// DenylistJTI) at the verifying call site, exactly as an early-revoked access
// token would be rejected.
type ReviewHandoffSigner struct {
	key    *SigningKey
	issuer string
	ttl    time.Duration
}

// NewReviewHandoffSigner builds a signer bound to key, self-identifying as
// issuer (the same value Config.Issuer uses elsewhere), minting tokens valid
// for ttl.
func NewReviewHandoffSigner(key *SigningKey, issuer string, ttl time.Duration) *ReviewHandoffSigner {
	return &ReviewHandoffSigner{key: key, issuer: issuer, ttl: ttl}
}

// Sign mints a one-time handoff token for userID/orgID. The caller is
// responsible for the "one-time" half: record the returned jti as consumed
// (Store.DenylistJTI) the moment the token is successfully redeemed.
func (s *ReviewHandoffSigner) Sign(userID, orgID uuid.UUID) (token, jti string, expiresAt time.Time, err error) {
	jti = randomID(16)
	now := time.Now()
	expiresAt = now.Add(s.ttl)
	token, err = s.key.Sign(Claims{
		Issuer: s.issuer, Audience: ReviewHandoffAudience,
		Subject: userID.String(), OrganizationID: orgID.String(),
		IssuedAt: now.Unix(), ExpiresAt: expiresAt.Unix(), JTI: jti,
	})
	return token, jti, expiresAt, err
}

// Verify checks signature, issuer, audience, and expiry ONLY — one-time
// consumption (jti) and current organization membership are the caller's job
// (internal/httpapi, which owns the Store and the users/organization_users
// tables), mirroring VerifyAccessToken's own doc comment on why this package
// never reaches into internal/store itself.
func (s *ReviewHandoffSigner) Verify(token string) (Claims, error) {
	return s.key.Verify(token, s.issuer, ReviewHandoffAudience)
}
