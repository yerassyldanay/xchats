package mcpauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// VerifyPKCE checks a code_verifier against the code_challenge stored at
// authorization time. Only S256 is accepted (plan/mcp.md §3: "PKCE (S256)")
// — the plain method is not offered, so there is no downgrade path to reject.
func VerifyPKCE(codeVerifier, codeChallenge, method string) bool {
	if method != "S256" {
		return false
	}
	if codeVerifier == "" || codeChallenge == "" {
		return false
	}
	sum := sha256.Sum256([]byte(codeVerifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(codeChallenge)) == 1
}
