package mcpauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// mediaReadHMACKey derives MediaReadTokenSigner's symmetric key the same way
// uploadHMACKey derives the upload one, but under a DIFFERENT label. That
// difference is the whole point: a token that authorises reading one material
// must never be accepted as authorisation to overwrite it, and vice versa.
// Same seed, same construction, disjoint key material.
func (k *SigningKey) mediaReadHMACKey() []byte {
	sum := sha256.Sum256(append(append([]byte{}, k.priv.Seed()...), []byte("xchats-mcp-media-read")...))
	return sum[:]
}

// MediaReadTokenSigner mints and verifies the short-lived signed URL the KB
// Manager widget uses to DISPLAY an already-attached material — an <img> in a
// record's media section, or an explicit open/download of a video, audio file
// or document.
//
// It exists because the widget runs in an iframe on a host origin we cannot
// predict (chatgpt.com's per-app sandbox), so it can carry no session cookie
// and can never be covered by the CORS allowlist the rest of the API uses.
// The grant is deliberately narrow: read-only, exactly one material (the id is
// bound into the signature), and time-boxed. There is no ambient authority —
// the token IS the authorisation — which is what makes the permissive CORS on
// that route safe, for the same reason it is safe on /mcp/uploads.
//
// Org scoping is structural rather than encoded in the token: a URL is only
// ever minted while answering an authorised kb_read for that organization's
// principal, so a caller cannot obtain a URL for a material it could not
// already read.
type MediaReadTokenSigner struct{ key []byte }

// NewMediaReadTokenSigner builds a signer bound to key's derived HMAC key.
func NewMediaReadTokenSigner(key *SigningKey) *MediaReadTokenSigner {
	return &MediaReadTokenSigner{key: key.mediaReadHMACKey()}
}

// Sign returns a read token valid only for materialID, expiring at
// expiresAtUnix.
func (m *MediaReadTokenSigner) Sign(materialID string, expiresAtUnix int64) string {
	payload := materialID + "." + strconv.FormatInt(expiresAtUnix, 10)
	mac := hmac.New(sha256.New, m.key)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// Verify checks token was signed by this server for exactly materialID and has
// not yet expired. An upload token never verifies here.
func (m *MediaReadTokenSigner) Verify(materialID, token string) bool {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 || parts[0] != materialID {
		return false
	}
	expiresAtUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > expiresAtUnix {
		return false
	}
	want := m.Sign(materialID, expiresAtUnix)
	return hmac.Equal([]byte(token), []byte(want))
}
