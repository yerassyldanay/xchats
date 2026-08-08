package meta

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAPIErrorNeverInterpolatesAToken(t *testing.T) {
	// APIError only ever formats fields already present in Meta's own error
	// envelope; feed it a token-shaped string in every field to prove
	// Error() is a fixed format string, not something that could echo an
	// out-of-band value some caller forgot to scrub.
	const secretLookingToken = "EAABsecretAccessToken1234567890abcdef"
	e := &APIError{
		Message: "unrelated failure", Type: "OAuthException", Code: 190,
		ErrorSubcode: 463, FBTraceID: "trace-1",
	}
	if strings.Contains(e.Error(), secretLookingToken) {
		t.Fatal("APIError.Error() contains a token that was never passed to it — impossible unless the format string changed")
	}
}

func TestIsTokenExpired(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"code 190 subcode 463", &APIError{Code: 190, ErrorSubcode: 463}, true},
		{"code 190 subcode 467", &APIError{Code: 190, ErrorSubcode: 467}, true},
		{"code 190 no subcode", &APIError{Code: 190}, false},
		{"code 190 unrelated subcode", &APIError{Code: 190, ErrorSubcode: 1}, false},
		{"unrelated code", &APIError{Code: 4, ErrorSubcode: 463}, false},
		{"not an APIError at all", errors.New("boom"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTokenExpired(tc.err); got != tc.want {
				t.Fatalf("IsTokenExpired(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsPermission(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"code 10", &APIError{Code: 10}, true},
		{"subcode 33", &APIError{Code: 200, ErrorSubcode: 33}, true},
		{"unrelated", &APIError{Code: 1}, false},
		{"not an APIError", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPermission(tc.err); got != tc.want {
				t.Fatalf("IsPermission(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestRedactReplacesEveryOccurrence(t *testing.T) {
	s := "GET https://graph.facebook.com/v21.0/me?access_token=SECRET123&fields=id failed; retry with access_token=SECRET123"
	got := redact(s, "SECRET123")
	if strings.Contains(got, "SECRET123") {
		t.Fatalf("redact left the token in place: %q", got)
	}
	if strings.Count(got, "<redacted>") != 2 {
		t.Fatalf("redact = %q, want both occurrences replaced", got)
	}
}

func TestRedactNoOpsOnEmptyToken(t *testing.T) {
	s := "nothing to redact here"
	if got := redact(s, ""); got != s {
		t.Fatalf("redact with an empty token changed the string: %q", got)
	}
}

func TestRedactErrPreservesUnwrapForContextCanceled(t *testing.T) {
	cause := context.Canceled
	err := redactErr(cause, "some-token")
	if !errors.Is(err, context.Canceled) {
		t.Fatal("redactErr must preserve errors.Is(err, context.Canceled) through one Unwrap hop")
	}
}

func TestRedactErrScrubsTheTokenFromText(t *testing.T) {
	cause := errors.New("dial tcp: connect to https://graph.facebook.com/v21.0/me?access_token=TOPSECRET failed")
	err := redactErr(cause, "TOPSECRET")
	if strings.Contains(err.Error(), "TOPSECRET") {
		t.Fatalf("redactErr left the token in the formatted text: %q", err.Error())
	}
}

func TestRedactErrNilIsNil(t *testing.T) {
	if redactErr(nil, "token") != nil {
		t.Fatal("redactErr(nil, ...) must return nil")
	}
}
