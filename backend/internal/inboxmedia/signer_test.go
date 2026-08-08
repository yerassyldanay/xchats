package inboxmedia

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVerifyAcceptsAFreshlyMintedToken(t *testing.T) {
	s := NewSigner("test-seed")
	org := uuid.New()
	media := uuid.New()
	exp := time.Now().Add(10 * time.Minute).Unix()
	token := s.Sign(org, media, exp)
	if !s.Verify(org, media, exp, token) {
		t.Fatal("a freshly minted token failed to verify")
	}
}

// TestVerifyRejectsCrossOrgToken is the IDOR regression test: a token minted
// for org A must never authorize a read against org B, even though mediaID
// and exp are identical and the signer/key are the same.
func TestVerifyRejectsCrossOrgToken(t *testing.T) {
	s := NewSigner("test-seed")
	orgA := uuid.New()
	orgB := uuid.New()
	media := uuid.New()
	exp := time.Now().Add(10 * time.Minute).Unix()
	token := s.Sign(orgA, media, exp)
	if s.Verify(orgB, media, exp, token) {
		t.Fatal("a token minted for org A verified against org B — cross-org media IDOR")
	}
	// The rightful org still works with the exact same token.
	if !s.Verify(orgA, media, exp, token) {
		t.Fatal("the token stopped verifying against its own org")
	}
}

func TestVerifyRejectsCrossMediaToken(t *testing.T) {
	s := NewSigner("test-seed")
	org := uuid.New()
	mediaA := uuid.New()
	mediaB := uuid.New()
	exp := time.Now().Add(10 * time.Minute).Unix()
	token := s.Sign(org, mediaA, exp)
	if s.Verify(org, mediaB, exp, token) {
		t.Fatal("a token minted for media A verified against media B")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	s := NewSigner("test-seed")
	org := uuid.New()
	media := uuid.New()
	exp := time.Now().Add(-1 * time.Minute).Unix() // already expired
	token := s.Sign(org, media, exp)
	if s.Verify(org, media, exp, token) {
		t.Fatal("an expired token verified")
	}
}

// TestVerifyRejectsTamperedExpiry simulates an attacker editing the `exp`
// query parameter to extend a token's lifetime: Verify is asked to accept
// the ORIGINAL token string against a LATER expiry than the one it was
// actually signed for.
func TestVerifyRejectsTamperedExpiry(t *testing.T) {
	s := NewSigner("test-seed")
	org := uuid.New()
	media := uuid.New()
	realExp := time.Now().Add(10 * time.Minute).Unix()
	token := s.Sign(org, media, realExp)

	extendedExp := time.Now().Add(24 * time.Hour).Unix()
	if s.Verify(org, media, extendedExp, token) {
		t.Fatal("a token verified against a tampered (extended) expiry")
	}
}

func TestVerifyRejectsTokenFromADifferentSeed(t *testing.T) {
	s1 := NewSigner("seed-one")
	s2 := NewSigner("seed-two")
	org := uuid.New()
	media := uuid.New()
	exp := time.Now().Add(10 * time.Minute).Unix()
	token := s1.Sign(org, media, exp)
	if s2.Verify(org, media, exp, token) {
		t.Fatal("a token minted by one signer verified under a different seed's key")
	}
}

func TestVerifyRejectsGarbageToken(t *testing.T) {
	s := NewSigner("test-seed")
	org := uuid.New()
	media := uuid.New()
	exp := time.Now().Add(10 * time.Minute).Unix()
	if s.Verify(org, media, exp, "not-a-real-token") {
		t.Fatal("a garbage token verified")
	}
	if s.Verify(org, media, exp, "") {
		t.Fatal("an empty token verified")
	}
}

func TestSameSeedDerivesTheSameKeyDeterministically(t *testing.T) {
	org := uuid.New()
	media := uuid.New()
	exp := time.Now().Add(10 * time.Minute).Unix()
	a := NewSigner("same-seed").Sign(org, media, exp)
	b := NewSigner("same-seed").Sign(org, media, exp)
	if a != b {
		t.Fatal("two signers built from the same seed produced different tokens for identical input")
	}
}
