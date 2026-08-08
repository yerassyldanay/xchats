package meta

import (
	"net/url"
	"testing"
)

func TestChallengeAcceptsMatchingVerifyToken(t *testing.T) {
	q := url.Values{}
	q.Set("hub.mode", "subscribe")
	q.Set("hub.verify_token", "the-token-we-chose")
	q.Set("hub.challenge", "1158201444")
	challenge, ok := Challenge(q, "the-token-we-chose")
	if !ok {
		t.Fatal("expected ok=true for a matching verify token")
	}
	if challenge != "1158201444" {
		t.Fatalf("challenge = %q, want the hub.challenge value verbatim", challenge)
	}
}

func TestChallengeRejectsWrongVerifyToken(t *testing.T) {
	q := url.Values{}
	q.Set("hub.mode", "subscribe")
	q.Set("hub.verify_token", "attacker-guess")
	q.Set("hub.challenge", "123")
	if _, ok := Challenge(q, "the-real-token"); ok {
		t.Fatal("expected ok=false for a mismatched verify token")
	}
}

func TestChallengeRejectsWrongMode(t *testing.T) {
	q := url.Values{}
	q.Set("hub.mode", "unsubscribe")
	q.Set("hub.verify_token", "correct")
	q.Set("hub.challenge", "123")
	if _, ok := Challenge(q, "correct"); ok {
		t.Fatal("expected ok=false when hub.mode is not exactly 'subscribe'")
	}
}

func TestChallengeRejectsMissingMode(t *testing.T) {
	q := url.Values{}
	q.Set("hub.verify_token", "correct")
	q.Set("hub.challenge", "123")
	if _, ok := Challenge(q, "correct"); ok {
		t.Fatal("expected ok=false when hub.mode is absent")
	}
}

func TestChallengeRejectsEmptyVerifyTokenConfigured(t *testing.T) {
	// An unconfigured verify token (empty string) must never match an
	// equally-empty query param — fail closed, not "anything goes when
	// unset".
	q := url.Values{}
	q.Set("hub.mode", "subscribe")
	q.Set("hub.challenge", "123")
	if _, ok := Challenge(q, ""); ok {
		t.Fatal("expected ok=false when the configured verify token is empty")
	}
}
