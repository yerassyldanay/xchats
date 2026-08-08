package meta

import (
	"fmt"
	"strings"
)

// APIError is Meta's Graph API error envelope:
//
//	{"error": {"message": "...", "type": "OAuthException", "code": 190,
//	           "error_subcode": 463, "fbtrace_id": "..."}}
//
// Error() formats ONLY fields already present in that envelope — Meta's own
// error body never echoes back the caller's access token — so it can never
// leak one on its own. The token can still reach a *transport*-level error
// (an access_token=... query param embedded in a *url.Error from a failed
// request), which is what redact() in graph.go guards separately, mirroring
// internal/telegram.redact's reasoning exactly.
type APIError struct {
	Message      string
	Type         string
	Code         int
	ErrorSubcode int
	FBTraceID    string
}

func (e *APIError) Error() string {
	if e.ErrorSubcode != 0 {
		return fmt.Sprintf("meta: %s (type=%s code=%d subcode=%d fbtrace_id=%s)",
			e.Message, e.Type, e.Code, e.ErrorSubcode, e.FBTraceID)
	}
	return fmt.Sprintf("meta: %s (type=%s code=%d fbtrace_id=%s)", e.Message, e.Type, e.Code, e.FBTraceID)
}

// IsTokenExpired reports whether err is a Graph API "your access token is no
// longer valid" error: OAuthException code 190, with subcode 463 (token
// expired) or 467 (invalid/invalidated session — a revoked or password-reset
// token). This is the token refresher worker's and every channel sender's
// signal to stop retrying and instead mark the account token_expired /
// prompt reconnection, rather than treat it as a transient send failure.
func IsTokenExpired(err error) bool {
	ae, ok := asAPIError(err)
	if !ok {
		return false
	}
	return ae.Code == 190 && (ae.ErrorSubcode == 463 || ae.ErrorSubcode == 467)
}

// IsPermission reports whether err is a Graph API permission/scope error —
// the app (or the token's granted scopes) lacks what the call needs, which
// no retry will fix. Covers the two shapes Meta's own documentation
// describes as permission failures: a bare "Permission Error" (code 10) and
// an OAuthException carrying the "missing permission" subcode (33). Treat
// this as a best-effort classification, not an exhaustive one — verify
// against a live Graph API response before leaning on it for anything more
// than "surface a clearer message to the operator".
func IsPermission(err error) bool {
	ae, ok := asAPIError(err)
	if !ok {
		return false
	}
	return ae.Code == 10 || ae.ErrorSubcode == 33
}

func asAPIError(err error) (*APIError, bool) {
	ae, ok := err.(*APIError)
	return ae, ok
}

// redact replaces every occurrence of token in s with a fixed placeholder —
// the same "strip the secret from whatever text is about to become an error
// message or a log line" idiom internal/telegram.redact uses, needed here
// because a Graph API call MAY carry its token as an access_token=... query
// parameter (rather than only an Authorization header), and a failed
// request's *url.Error embeds the full request URL.
func redact(s, token string) string {
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "<redacted>")
}
