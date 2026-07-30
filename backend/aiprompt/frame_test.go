package aiprompt

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
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

// frameShopKBV4TGRUSHA256 pins the Telegram variant the same way. It is not an
// independently evaluated prompt: the assertion below that its body (everything
// after the persona line) is byte-identical to the graded frame is what keeps it
// covered by the schema_kb_v1 eval.
const frameShopKBV4TGRUSHA256 = "eed1585d369d6e2d34750bee14a9959a70801f8cf1f64184ee1e0db7f290c029"

func TestFrameShopKBV4TGRU_MatchesPinnedHash(t *testing.T) {
	sum := sha256.Sum256([]byte(FrameShopKBV4TGRU()))
	got := hex.EncodeToString(sum[:])
	if got != frameShopKBV4TGRUSHA256 {
		t.Fatalf("frames/shop-kb-v4-tg-ru.txt sha256 = %s, want %s (the embedded frame changed — see doc comment)", got, frameShopKBV4TGRUSHA256)
	}
}

// TestFrameShopKBV4TGRU_BodyMatchesGradedFrame is the real guarantee: the two
// frames may differ ONLY in their first line. Any rule that drifts between them
// would ship an unevaluated prompt to Telegram customers.
func TestFrameShopKBV4TGRU_BodyMatchesGradedFrame(t *testing.T) {
	graded := FrameShopKBV4RU()
	tg := FrameShopKBV4TGRU()

	gi := strings.Index(graded, "\n")
	ti := strings.Index(tg, "\n")
	if gi < 0 || ti < 0 {
		t.Fatal("a frame has no newline — cannot split off the persona line")
	}
	if graded[gi:] != tg[ti:] {
		t.Fatal("the Telegram frame's body diverged from the graded frame — only the first line may differ")
	}
	if strings.Contains(strings.ToLower(tg[:ti]), "whatsapp") {
		t.Fatalf("the Telegram frame still names WhatsApp in its persona line: %q", tg[:ti])
	}
	if graded[:gi] == tg[:ti] {
		t.Fatal("the Telegram frame's persona line is unchanged — it should be channel-neutral")
	}
}

func TestPromptRefShopKBV4_Value(t *testing.T) {
	if PromptRefShopKBV4 != "shop-kb@v4" {
		t.Fatalf("PromptRefShopKBV4 = %q, want %q", PromptRefShopKBV4, "shop-kb@v4")
	}
}
