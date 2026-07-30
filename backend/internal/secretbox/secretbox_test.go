package secretbox

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"
)

func mustBox(t *testing.T, key []byte) *Box {
	t.Helper()
	b, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

func TestSealOpenRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand: %v", err)
	}
	b := mustBox(t, key)

	const token = "8123456789:AAH-fake-bot-token-for-tests_0123456789"
	sealed, err := b.Seal([]byte(token))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Contains(sealed, []byte(token)) {
		t.Fatal("ciphertext contains the plaintext token")
	}
	got, err := b.Open(sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if string(got) != token {
		t.Fatalf("round trip = %q, want %q", got, token)
	}

	// Two seals of the same plaintext must differ (fresh nonce each time).
	again, err := b.Seal([]byte(token))
	if err != nil {
		t.Fatalf("Seal again: %v", err)
	}
	if bytes.Equal(sealed, again) {
		t.Fatal("two seals produced identical ciphertext — the nonce is not random")
	}
}

func TestOpenWithWrongKeyFails(t *testing.T) {
	k1, k2 := make([]byte, 32), make([]byte, 32)
	if _, err := rand.Read(k1); err != nil {
		t.Fatalf("rand: %v", err)
	}
	copy(k2, k1)
	k2[0] ^= 0xff

	sealed, err := mustBox(t, k1).Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if out, err := mustBox(t, k2).Open(sealed); err == nil {
		t.Fatalf("Open with the wrong key succeeded, returned %q", out)
	}
}

func TestOpenRejectsTamperedAndTruncated(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand: %v", err)
	}
	b := mustBox(t, key)
	sealed, err := b.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := b.Open(tampered); err == nil {
		t.Fatal("Open accepted tampered ciphertext")
	}
	if _, err := b.Open(sealed[:4]); err == nil {
		t.Fatal("Open accepted a truncated blob")
	}
}

func TestFromEnvValueAcceptsHexBase64AndLiteral(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 1)
	}
	cases := map[string]string{
		"hex":        hex.EncodeToString(raw),
		"base64":     base64.StdEncoding.EncodeToString(raw),
		"base64 raw": base64.RawURLEncoding.EncodeToString(raw),
	}
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			b, err := FromEnvValue(v)
			if err != nil {
				t.Fatalf("FromEnvValue: %v", err)
			}
			sealed, err := b.Seal([]byte("x"))
			if err != nil {
				t.Fatalf("Seal: %v", err)
			}
			// All four spellings must yield the SAME key: a blob sealed with one
			// has to open with another.
			ref, err := New(raw)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if _, err := ref.Open(sealed); err != nil {
				t.Fatalf("spelling %q produced a different key: %v", name, err)
			}
		})
	}
}

// A 32-character printable passphrase is the last-resort spelling: it is used
// verbatim as the key, so it must survive the round trip unchanged.
func TestFromEnvValueAcceptsPrintableLiteral(t *testing.T) {
	const literal = "xchats-credentials-key-01234567" + "8" // exactly 32 chars
	b, err := FromEnvValue(literal)
	if err != nil {
		t.Fatalf("FromEnvValue: %v", err)
	}
	sealed, err := b.Seal([]byte("token"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	ref, err := New([]byte(literal))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := ref.Open(sealed)
	if err != nil {
		t.Fatalf("literal spelling produced a different key: %v", err)
	}
	if string(got) != "token" {
		t.Fatalf("round trip = %q", got)
	}
}

func TestFromEnvValueRejectsEmptyAndShort(t *testing.T) {
	if _, err := FromEnvValue("  "); !errors.Is(err, ErrNoKey) {
		t.Fatalf("empty key err = %v, want ErrNoKey", err)
	}
	if _, err := FromEnvValue("too-short"); err == nil {
		t.Fatal("a short key was accepted")
	}
}
