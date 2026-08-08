// Package inboxmedia mints and verifies the short-lived, ORG-BOUND signed
// URLs that let Instagram/Messenger fetch an outbound attachment's bytes:
// both platforms send media by public URL rather than accepting an upload,
// so our private blob needs a temporary public route for Meta's own servers
// to GET.
//
// It must not be confused with mcpauth.MediaReadTokenSigner, which binds
// only a material id and relies on the caller never minting a URL outside
// an already-authorized kb_read — safe there because nothing crosses an
// authorization boundary the caller hasn't already checked. A Meta channel
// send has no such structural guarantee (the URL is handed to a THIRD
// PARTY — Meta's fetch — not rendered inline for an already-authenticated
// browser), so the token itself must carry, and verification must check,
// which organization it authorizes.
package inboxmedia

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// keyLabel disambiguates this signer's derived key from every other
// purpose-specific HMAC key this codebase derives from a shared seed (see
// mcpauth's own media-read and upload signers, each under their own label,
// and httpapi's CSRF token under its own derivation) — same seed, same
// construction, disjoint key material.
const keyLabel = "xchats-inboxmedia-read"

// Signer mints and verifies GET /meta/api/v1/media/:media_id's `token` query
// parameter. The wire shape is `?token=<Sign(...)>&exp=<unix_seconds>` —
// `exp` rides as its own separate, human-readable query parameter (so an
// operator or a log line can see the expiry without decoding anything), but
// it is NOT trusted on its own: the HMAC signed here covers exp too, so
// editing it without knowing the key invalidates the token exactly like
// editing mediaID would.
//
// orgID is deliberately never taken from the request — Verify's caller must
// supply the ACTUAL owning organization, resolved by the handler's own
// media_id -> message -> account -> organization_id SQL lookup. That is the
// second of the two independent layers this package exists to combine: a
// perfectly-valid token for org A, replayed against a media row that
// belongs to org B, fails the moment the handler resolves B and asks
// Verify(B, mediaID, exp, token) — the signature was computed over A, not B.
type Signer struct {
	key []byte
}

// NewSigner derives a key from seed (cfg.SessionSecret — the durable,
// already-provisioned secret every other non-purpose-built signer in this
// codebase seeds from; see httpapi's csrfSecretFor) under keyLabel.
func NewSigner(seed string) *Signer {
	sum := sha256.Sum256(append([]byte(seed), []byte(keyLabel)...))
	return &Signer{key: sum[:]}
}

// Sign returns the token authorizing a GET for mediaID within organization
// orgID, valid until expiresAtUnix.
func (s *Signer) Sign(orgID, mediaID uuid.UUID, expiresAtUnix int64) string {
	mac := hmac.New(sha256.New, s.key)
	writeField(mac, orgID.String())
	writeField(mac, mediaID.String())
	writeField(mac, strconv.FormatInt(expiresAtUnix, 10))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Verify checks token authenticates exactly (orgID, mediaID, expiresAtUnix)
// — via hmac.Equal, never a plain byte comparison — and that expiresAtUnix
// has not passed. It does NOT parse expiresAtUnix out of token: the caller
// reads it from the request's own `exp` query parameter and passes it in,
// and an expiresAtUnix that does not match what Sign originally used simply
// fails to reproduce the same signature.
//
// Verify alone proves the token is unforged and addresses (orgID, mediaID)
// — it does not prove mediaID actually belongs to orgID. See the package
// and Signer doc comments: the HTTP handler must independently resolve
// mediaID's true owning organization in SQL and pass THAT as orgID, not a
// value read from the request.
func (s *Signer) Verify(orgID, mediaID uuid.UUID, expiresAtUnix int64, token string) bool {
	if time.Now().Unix() > expiresAtUnix {
		return false
	}
	want := s.Sign(orgID, mediaID, expiresAtUnix)
	return hmac.Equal([]byte(token), []byte(want))
}

// writeField feeds one length-prefixed field into the running MAC — the
// length prefix (not just a separator byte) is what makes the encoding
// unambiguous regardless of what bytes a field might contain, so
// "ab"+"c" and "a"+"bc" can never hash identically.
func writeField(mac io.Writer, s string) {
	b := []byte(s)
	var lenBuf [8]byte
	n := len(b)
	for i := 0; i < 8; i++ {
		lenBuf[i] = byte(n)
		n >>= 8
	}
	_, _ = mac.Write(lenBuf[:])
	_, _ = mac.Write(b)
}
