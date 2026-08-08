package meta

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// sha256Prefix is the required prefix on every X-Hub-Signature-256 header
// value Meta sends — a bare hex digest with no prefix is never valid, so
// VerifySignature refuses it rather than guessing the caller meant sha256.
const sha256Prefix = "sha256="

// VerifySignature checks a webhook delivery's X-Hub-Signature-256 header
// against the RAW, unmodified request body — HMAC-SHA256 keyed by the
// operator's own Meta App Secret. header must be exactly what Meta sent
// ("sha256=<hex>"); raw must be the exact bytes read off the wire, before
// any JSON decoding (re-marshaling would not byte-for-byte reproduce what
// Meta signed). Uses hmac.Equal throughout — never a plain byte/string
// comparison — so verification time does not leak how many leading bytes of
// the signature matched.
func VerifySignature(appSecret string, raw []byte, header string) bool {
	if !strings.HasPrefix(header, sha256Prefix) {
		return false
	}
	got, err := hex.DecodeString(header[len(sha256Prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(raw)
	want := mac.Sum(nil)
	return hmac.Equal(got, want)
}
