// Package password is the sole owner of xchats' password hashing scheme
// (argon2id) — extracted from internal/httpapi so that non-HTTP callers
// (the first-boot admin bootstrap in cmd/xchats, notably) can hash and
// verify passwords without importing the HTTP edge.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

type argonParams struct {
	memory, time uint32
	threads      uint8
	keyLen       uint32
}

var defaultArgon = argonParams{memory: 64 * 1024, time: 1, threads: 4, keyLen: 32}

// Hash returns an encoded argon2id hash string.
func Hash(plaintext string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	p := defaultArgon
	key := argon2.IDKey([]byte(plaintext), salt, p.time, p.memory, p.threads, p.keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.time, p.threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify checks plaintext against an encoded argon2id hash. It returns
// false for a malformed or empty encoded string rather than erroring — the
// bootstrap-admin sentinel deliberately uses "" as an "unloginnable" hash
// (see cmd/xchats' bootstrap admin logic), and this must fail closed for it.
func Verify(plaintext, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var p argonParams
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	p.keyLen = uint32(len(want))
	got := argon2.IDKey([]byte(plaintext), salt, p.time, p.memory, p.threads, p.keyLen)
	return subtle.ConstantTimeCompare(got, want) == 1
}
