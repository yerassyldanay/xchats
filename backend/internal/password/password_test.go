package password_test

import (
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/internal/password"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	hash, err := password.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash %q does not look like argon2id", hash)
	}
	if !password.Verify("correct horse battery staple", hash) {
		t.Error("Verify: correct password rejected")
	}
	if password.Verify("wrong password", hash) {
		t.Error("Verify: wrong password accepted")
	}
}

func TestVerifyRejectsMalformedOrEmptyHash(t *testing.T) {
	cases := []string{"", "not-a-hash", "$argon2id$v=19$bad", "$bcrypt$v=1$x$y"}
	for _, encoded := range cases {
		if password.Verify("anything", encoded) {
			t.Errorf("Verify(%q) = true, want false (fail closed)", encoded)
		}
	}
}

func TestHashProducesUniqueSaltPerCall(t *testing.T) {
	a, err := password.Hash("same-input")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	b, err := password.Hash("same-input")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if a == b {
		t.Error("two hashes of the same plaintext must differ (random salt)")
	}
	if !password.Verify("same-input", a) || !password.Verify("same-input", b) {
		t.Error("both hashes must still verify against their own plaintext")
	}
}
