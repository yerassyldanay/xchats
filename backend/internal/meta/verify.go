package meta

import (
	"crypto/subtle"
	"net/url"
)

// Challenge implements the webhook subscription handshake every one of the
// three Meta channels performs identically: Meta issues a GET with
// hub.mode=subscribe, hub.verify_token=<the token we chose when we
// registered the callback URL>, and hub.challenge=<a random int, as a
// string>. ok is true only when hub.mode is exactly "subscribe" and
// hub.verify_token matches verifyToken (constant-time, the same posture as
// the webhook secret check in internal/httpapi/telegram_webhook.go) — the
// handler must then echo challenge back as a PLAIN TEXT body, no JSON
// envelope, or Meta treats the subscription attempt as failed.
func Challenge(query url.Values, verifyToken string) (challenge string, ok bool) {
	if verifyToken == "" {
		// An unconfigured verify token must never match — fail closed, the
		// same posture as internal/httpapi's Telegram webhook secret check
		// on an unconfigured secret. Without this, ConstantTimeCompare's
		// own equal-empty-slices-are-equal semantics would let a request
		// with no hub.verify_token at all "verify" successfully.
		return "", false
	}
	if query.Get("hub.mode") != "subscribe" {
		return "", false
	}
	got := query.Get("hub.verify_token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(verifyToken)) != 1 {
		return "", false
	}
	return query.Get("hub.challenge"), true
}
