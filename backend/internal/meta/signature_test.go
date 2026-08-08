package meta

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestVerifySignatureKnownVector pins a signature computed independently
// (Python's hmac/hashlib, not this package's own sign() helper — a bug in
// Go's crypto/hmac usage here would not be caught by round-tripping through
// the same construction) for:
//
//	secret = "it-is-a-secret"
//	body   = {"object":"instagram","entry":[{"id":"123"}]}
//
// Recompute with: python3 -c "import hmac,hashlib;
// print(hmac.new(b'it-is-a-secret', b'{\"object\":\"instagram\",\"entry\":[{\"id\":\"123\"}]}', hashlib.sha256).hexdigest())"
func TestVerifySignatureKnownVector(t *testing.T) {
	const secret = "it-is-a-secret"
	const body = `{"object":"instagram","entry":[{"id":"123"}]}`
	const wantHeader = "sha256=f9b60a706d6bb4bc448c135ca31efd43ee2fbed01797f1a8f9407787a8dcb8d7"

	if !VerifySignature(secret, []byte(body), wantHeader) {
		t.Fatalf("known-vector signature %q did not verify", wantHeader)
	}
}

func TestVerifySignatureAcceptsExactMatch(t *testing.T) {
	body := []byte(`{"object":"page","entry":[{"id":"456","messaging":[]}]}`)
	header := sign("app-secret-value", body)
	if !VerifySignature("app-secret-value", body, header) {
		t.Fatal("valid signature rejected")
	}
}

func TestVerifySignatureRejectsSingleByteBodyMutation(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account","entry":[]}`)
	header := sign("secret", body)
	mutated := append([]byte(nil), body...)
	mutated[len(mutated)-2] ^= 0x01 // flip one bit near the end
	if VerifySignature("secret", mutated, header) {
		t.Fatal("signature verified against a mutated body")
	}
}

func TestVerifySignatureRejectsSingleByteSignatureMutation(t *testing.T) {
	body := []byte(`{"object":"instagram","entry":[]}`)
	header := sign("secret", body)
	mutatedHeader := header[:len(header)-1] + flipHexNibble(header[len(header)-1])
	if VerifySignature("secret", body, mutatedHeader) {
		t.Fatal("signature verified against a mutated signature")
	}
}

func flipHexNibble(c byte) string {
	if c == '0' {
		return "1"
	}
	return "0"
}

func TestVerifySignatureRejectsWrongKey(t *testing.T) {
	body := []byte(`{"object":"page","entry":[]}`)
	header := sign("correct-secret", body)
	if VerifySignature("wrong-secret", body, header) {
		t.Fatal("signature verified against the wrong app secret")
	}
}

func TestVerifySignatureRequiresSha256Prefix(t *testing.T) {
	body := []byte(`{}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	bareHex := hex.EncodeToString(mac.Sum(nil))
	if VerifySignature("secret", body, bareHex) {
		t.Fatal("a bare hex digest with no sha256= prefix verified")
	}
	if VerifySignature("secret", body, "sha1="+bareHex) {
		t.Fatal("a sha1= prefixed header verified")
	}
}

func TestVerifySignatureRejectsMalformedHex(t *testing.T) {
	if VerifySignature("secret", []byte("body"), "sha256=not-hex-at-all") {
		t.Fatal("malformed hex verified")
	}
}

func TestVerifySignatureRejectsEmptyHeader(t *testing.T) {
	if VerifySignature("secret", []byte("body"), "") {
		t.Fatal("empty header verified")
	}
}
