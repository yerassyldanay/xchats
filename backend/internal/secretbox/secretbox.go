// Package secretbox is the at-rest encryption used for provider credentials we
// must be able to read back (a Telegram bot token has to be replayed on every
// send, so it cannot be hashed). AES-256-GCM with a random 96-bit nonce
// prefixed to the ciphertext; the key comes from CREDENTIALS_ENC_KEY.
//
// Losing the key is recoverable but not silent: every stored token stops
// decrypting and has to be re-pasted. Rotating it is why the credential rows
// carry an encryption_key_version alongside the bytes.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// KeyVersion is the version stamped on rows this package writes. It exists so a
// future rotation can re-wrap old rows without a schema change; there is only
// one key today.
const KeyVersion = 1

// ErrNoKey is returned when no encryption key is configured. Callers surface it
// as a configuration error, never as "the token is wrong".
var ErrNoKey = errors.New("secretbox: no encryption key configured (set CREDENTIALS_ENC_KEY)")

// Box seals and opens secrets with one AES-256-GCM key.
type Box struct {
	aead cipher.AEAD
}

// New builds a Box from a raw 32-byte key.
func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secretbox: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secretbox: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: new gcm: %w", err)
	}
	return &Box{aead: aead}, nil
}

// FromEnvValue builds a Box from the configured key string. It accepts a
// 64-char hex key, a base64 (std or URL, padded or not) key that decodes to 32
// bytes, or a literal 32-byte string — so an operator can paste whatever
// `openssl rand -hex 32` or `head -c 32 /dev/urandom | base64` produced.
// An empty value returns ErrNoKey so the caller can say exactly that.
func FromEnvValue(v string) (*Box, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, ErrNoKey
	}
	if b, err := hex.DecodeString(v); err == nil && len(b) == 32 {
		return New(b)
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(v); err == nil && len(b) == 32 {
			return New(b)
		}
	}
	if len(v) == 32 {
		return New([]byte(v))
	}
	return nil, errors.New("secretbox: CREDENTIALS_ENC_KEY must decode to 32 bytes (64 hex chars, base64, or a 32-char literal)")
}

// Seal encrypts plaintext, returning nonce||ciphertext.
func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("secretbox: nonce: %w", err)
	}
	return b.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts a nonce||ciphertext blob produced by Seal. A wrong key, a
// truncated blob, or tampered bytes all come back as an error — never as
// garbage plaintext.
func (b *Box) Open(sealed []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("secretbox: ciphertext too short")
	}
	out, err := b.aead.Open(nil, sealed[:n], sealed[n:], nil)
	if err != nil {
		return nil, fmt.Errorf("secretbox: open: %w", err)
	}
	return out, nil
}
