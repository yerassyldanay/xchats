package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// csrfSecretFor resolves the HMAC key /oauth/authorize/decision's CSRF token
// is signed with — cfg.SessionSecret when configured, else the per-process
// random fallback generated once in New() (see server.go's csrfSecret doc
// comment for the multi-replica caveat).
func (s *Server) csrfSecretFor() []byte {
	if s.cfg.SessionSecret != "" {
		return []byte(s.cfg.SessionSecret)
	}
	return s.csrfSecret
}

// oauthCSRFToken derives a stateless CSRF token bound to the browser's
// existing xchats session id (the "signed double-submit cookie" pattern): an
// attacker forging a cross-site POST to /oauth/authorize/decision cannot read
// the HttpOnly session cookie, so they cannot compute this value, even though
// the consent page happily echoes it back as a plain (non-HttpOnly) hidden
// form field.
func (s *Server) oauthCSRFToken(sessionID string) string {
	mac := hmac.New(sha256.New, s.csrfSecretFor())
	mac.Write([]byte(sessionID))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyOAuthCSRFToken re-derives the expected token from the CURRENT
// session cookie and compares in constant time.
func (s *Server) verifyOAuthCSRFToken(sessionID, submitted string) bool {
	if submitted == "" {
		return false
	}
	want := s.oauthCSRFToken(sessionID)
	return subtle.ConstantTimeCompare([]byte(want), []byte(submitted)) == 1
}

// randomCSRFFallbackSecret generates the per-process fallback used when
// cfg.SessionSecret is not configured.
func randomCSRFFallbackSecret() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("httpapi: crypto/rand unavailable: " + err.Error())
	}
	return b
}
