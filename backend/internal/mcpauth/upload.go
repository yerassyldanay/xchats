package mcpauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// uploadHMACKey derives a symmetric key for UploadTokenSigner from the same
// Ed25519 seed that signs access tokens — a distinct derivation (SHA-256 of
// the seed plus a fixed label), not the raw private-key bytes themselves, so
// the two primitives (EdDSA signing, HMAC) never share literal key material.
func (k *SigningKey) uploadHMACKey() []byte {
	sum := sha256.Sum256(append(append([]byte{}, k.priv.Seed()...), []byte("xchats-mcp-upload")...))
	return sum[:]
}

// UploadTokenSigner mints and verifies the short-lived signed upload URL
// kb_media_upload returns (plan/mcp.md §5): a widget PUTs bytes straight to
// this URL, so it must be independently checkable without a database round
// trip on every byte-range/retry request, single-object-scoped (the
// material_id is bound into the signature), and time-boxed.
type UploadTokenSigner struct{ key []byte }

// NewUploadTokenSigner builds a signer bound to key's derived HMAC key.
func NewUploadTokenSigner(key *SigningKey) *UploadTokenSigner {
	return &UploadTokenSigner{key: key.uploadHMACKey()}
}

// Sign returns a token valid only for materialID, expiring at expiresAtUnix.
func (u *UploadTokenSigner) Sign(materialID string, expiresAtUnix int64) string {
	payload := materialID + "." + strconv.FormatInt(expiresAtUnix, 10)
	mac := hmac.New(sha256.New, u.key)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// Verify checks token was signed by this server, for exactly materialID, and
// has not yet expired.
func (u *UploadTokenSigner) Verify(materialID, token string) bool {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 || parts[0] != materialID {
		return false
	}
	expiresAtUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > expiresAtUnix {
		return false
	}
	want := u.Sign(materialID, expiresAtUnix)
	return hmac.Equal([]byte(token), []byte(want))
}
