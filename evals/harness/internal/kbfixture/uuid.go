package kbfixture

import (
	"crypto/sha1"
	"fmt"
)

// dnsNamespace is the standard RFC 4122 "namespace for DNS names" UUID — the
// conventional root every application-specific v5 namespace derives from.
var dnsNamespace = [16]byte{
	0x6b, 0xa7, 0xb8, 0x10, 0x9d, 0xad, 0x11, 0xd1,
	0x80, 0xb4, 0x00, 0xc0, 0x4f, 0xd4, 0x30, 0xc8,
}

// uuidV5 derives an RFC 4122 version-5 (SHA-1, namespace-based) UUID from
// namespace and name. Deterministic: the same inputs always produce the same
// 16 bytes, which is the whole reason the generator uses v5 instead of random
// v4 IDs — regenerating the fixture must reproduce byte-identical material
// IDs so the drift test has something meaningful to compare.
func uuidV5(namespace [16]byte, name string) [16]byte {
	h := sha1.New()
	h.Write(namespace[:])
	h.Write([]byte(name))
	sum := h.Sum(nil)
	var out [16]byte
	copy(out[:], sum[:16])
	out[6] = (out[6] & 0x0f) | 0x50 // version 5
	out[8] = (out[8] & 0x3f) | 0x80 // RFC 4122 variant
	return out
}

func formatUUID(b [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// shopKBNamespace is the fixed v5 namespace every shop-kb-v1 fixture material
// ID derives from — itself a deterministic v5 UUID so the whole ID space
// traces back to one documented root instead of an arbitrary literal.
var shopKBNamespace = uuidV5(dnsNamespace, "xchats-evals-shop-kb-v1")

// deterministicMaterialID derives a stable material UUID from a human-legible
// key (e.g. "products/coffee-machine/gallery_images/0") — the same key always
// yields the same ID, so regenerating the fixture never reshuffles references.
func deterministicMaterialID(key string) string {
	return formatUUID(uuidV5(shopKBNamespace, key))
}
