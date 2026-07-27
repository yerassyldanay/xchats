package aiprompt

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// frameShopKBV4RUSHA256 pins frames/shop-kb-v4-ru.txt to the exact bytes of
// evals/scenarios/shop-kb-v1/frame-ru.txt at the moment it was copied in (see
// evals/harness/promptref_test.go's extractPromptV1SHA256 for the doctrine this
// mirrors). If this test ever fails, the embedded frame silently changed — bump it
// deliberately (cut a v5 frame instead) rather than editing this file in place.
const frameShopKBV4RUSHA256 = "5cd49be7fdc26fadf279e26114d180ff2451d82aaafeaec8fd58c9e4500f747f"

func TestFrameShopKBV4RU_MatchesPinnedHash(t *testing.T) {
	sum := sha256.Sum256([]byte(FrameShopKBV4RU()))
	got := hex.EncodeToString(sum[:])
	if got != frameShopKBV4RUSHA256 {
		t.Fatalf("frames/shop-kb-v4-ru.txt sha256 = %s, want %s (the embedded frame changed — see doc comment)", got, frameShopKBV4RUSHA256)
	}
}

func TestFrameShopKBV4RU_NonEmpty(t *testing.T) {
	if FrameShopKBV4RU() == "" {
		t.Fatal("want non-empty frame text")
	}
}

func TestPromptRefShopKBV4_Value(t *testing.T) {
	if PromptRefShopKBV4 != "shop-kb@v4" {
		t.Fatalf("PromptRefShopKBV4 = %q, want %q", PromptRefShopKBV4, "shop-kb@v4")
	}
}
