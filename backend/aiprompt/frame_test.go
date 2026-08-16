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

// --- v5: v4 plus the ТАРИФЫ block. Pinned exactly as v4 is; the same doctrine
// applies to it in turn (cut a v6, never edit v5 in place once shipped). ------

const frameShopKBV5RUSHA256 = "f36741807075c619450e2bf54205df7a0c436a0feaf4de7b49a75b883b297bef"

func TestFrameShopKBV5RU_MatchesPinnedHash(t *testing.T) {
	sum := sha256.Sum256([]byte(FrameShopKBV5RU()))
	got := hex.EncodeToString(sum[:])
	if got != frameShopKBV5RUSHA256 {
		t.Fatalf("frames/shop-kb-v5-ru.txt sha256 = %s, want %s (the embedded frame changed — see doc comment)", got, frameShopKBV5RUSHA256)
	}
}

const frameShopKBV5TGRUSHA256 = "9b9d98ad140bf24d5d43150a0627621000b7a41da686e2d8a0593cee55d759a5"

func TestFrameShopKBV5TGRU_MatchesPinnedHash(t *testing.T) {
	sum := sha256.Sum256([]byte(FrameShopKBV5TGRU()))
	got := hex.EncodeToString(sum[:])
	if got != frameShopKBV5TGRUSHA256 {
		t.Fatalf("frames/shop-kb-v5-tg-ru.txt sha256 = %s, want %s (the embedded frame changed — see doc comment)", got, frameShopKBV5TGRUSHA256)
	}
}

// TestFrameShopKBV5TGRU_BodyMatchesGradedFrame is v5's copy of the guarantee
// TestFrameShopKBV4TGRU_BodyMatchesGradedFrame makes for v4: the RU and TG
// frames may differ ONLY in their persona line, so a rule can never drift
// between the two channels.
func TestFrameShopKBV5TGRU_BodyMatchesGradedFrame(t *testing.T) {
	ru := FrameShopKBV5RU()
	tg := FrameShopKBV5TGRU()

	ri := strings.Index(ru, "\n")
	ti := strings.Index(tg, "\n")
	if ri < 0 || ti < 0 {
		t.Fatal("a frame has no newline — cannot split off the persona line")
	}
	if ru[ri:] != tg[ti:] {
		t.Fatal("the v5 Telegram frame's body diverged from the v5 RU frame — only the first line may differ")
	}
	if strings.Contains(strings.ToLower(tg[:ti]), "whatsapp") {
		t.Fatalf("the v5 Telegram frame still names WhatsApp in its persona line: %q", tg[:ti])
	}
	if ru[:ri] == tg[:ti] {
		t.Fatal("the v5 Telegram frame's persona line is unchanged — it should be channel-neutral")
	}
}

// TestFrameShopKBV5_IsV4PlusTariffs pins the ONLY intended difference between
// the two versions: v5 adds the %%TARIFFS%% slot and v4 has none. It is the
// cheap guard against a v5 edit that quietly drops or renames the slot
// renderTariffs fills, which would silently return tariffs to being invisible
// — the exact bug v5 exists to fix.
func TestFrameShopKBV5_IsV4PlusTariffs(t *testing.T) {
	if strings.Contains(FrameShopKBV4RU(), SlotTariffs) {
		t.Fatalf("v4 unexpectedly contains %s — v4 must stay frozen", SlotTariffs)
	}
	for _, f := range []struct{ name, text string }{
		{"v5 RU", FrameShopKBV5RU()},
		{"v5 TG", FrameShopKBV5TGRU()},
	} {
		if !strings.Contains(f.text, SlotTariffs) {
			t.Errorf("%s is missing the %s slot", f.name, SlotTariffs)
		}
		// Every v4 slot must survive into v5: v5 is strictly additive.
		for _, slot := range []string{SlotResponseSchema, SlotAssistant, SlotProductsInStock, SlotProductsOutOfStock, SlotTopics, SlotDeliveryZones, SlotBusinessFacts} {
			if !strings.Contains(f.text, slot) {
				t.Errorf("%s lost the %s slot that v4 carried", f.name, slot)
			}
		}
	}
}

func TestPromptRefShopKBV5_Value(t *testing.T) {
	if PromptRefShopKBV5 != "shop-kb@v5" {
		t.Fatalf("PromptRefShopKBV5 = %q, want %q", PromptRefShopKBV5, "shop-kb@v5")
	}
	if PromptRefShopKBV5TG != "shop-kb@v5-tg" {
		t.Fatalf("PromptRefShopKBV5TG = %q, want %q", PromptRefShopKBV5TG, "shop-kb@v5-tg")
	}
}
