package mcpauth_test

import (
	"testing"
	"time"

	"github.com/yerassyldanay/xchats/backend/internal/mcpauth"
)

func futureUnix() int64 { return time.Now().Add(time.Hour).Unix() }

func TestMediaReadTokenSigner_RoundTrip(t *testing.T) {
	s := mcpauth.NewMediaReadTokenSigner(mcpauth.NewEphemeralSigningKey())
	tok := s.Sign("mat-1", futureUnix())
	if !s.Verify("mat-1", tok) {
		t.Fatal("a freshly minted token must verify for its own material")
	}
}

func TestMediaReadTokenSigner_RejectsOtherMaterial(t *testing.T) {
	s := mcpauth.NewMediaReadTokenSigner(mcpauth.NewEphemeralSigningKey())
	tok := s.Sign("mat-1", futureUnix())
	if s.Verify("mat-2", tok) {
		t.Fatal("a token for one material must not authorise another")
	}
}

func TestMediaReadTokenSigner_RejectsExpired(t *testing.T) {
	s := mcpauth.NewMediaReadTokenSigner(mcpauth.NewEphemeralSigningKey())
	tok := s.Sign("mat-1", time.Now().Add(-time.Second).Unix())
	if s.Verify("mat-1", tok) {
		t.Fatal("an expired token must not verify")
	}
}

func TestMediaReadTokenSigner_RejectsOtherKey(t *testing.T) {
	a := mcpauth.NewMediaReadTokenSigner(mcpauth.NewEphemeralSigningKey())
	b := mcpauth.NewMediaReadTokenSigner(mcpauth.NewEphemeralSigningKey())
	if b.Verify("mat-1", a.Sign("mat-1", futureUnix())) {
		t.Fatal("a token minted under a different signing key must not verify")
	}
}

// TestMediaAndUploadTokensAreNotInterchangeable is the reason the two signers
// derive their HMAC keys under different labels. Reading a material and
// overwriting it are different privileges, and the widget hands both kinds of
// URL to a browser — so a read token must never be usable as a write
// capability, in either direction, even though both are minted from the same
// server seed.
func TestMediaAndUploadTokensAreNotInterchangeable(t *testing.T) {
	key := mcpauth.NewEphemeralSigningKey()
	media := mcpauth.NewMediaReadTokenSigner(key)
	upload := mcpauth.NewUploadTokenSigner(key)
	exp := futureUnix()

	if upload.Verify("mat-1", media.Sign("mat-1", exp)) {
		t.Error("a media READ token must not verify as an upload token")
	}
	if media.Verify("mat-1", upload.Sign("mat-1", exp)) {
		t.Error("an UPLOAD token must not verify as a media read token")
	}
	if media.Sign("mat-1", exp) == upload.Sign("mat-1", exp) {
		t.Error("the two signers must not produce identical tokens")
	}
}
