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

// --- v6: virtual fact columns (0017_kb_virtual_facts). Pinned exactly as
// v4/v5 are; the same doctrine applies going forward (cut a v7, never edit
// v6 in place once shipped). v4 and v5 stay embedded and pinned above,
// completely untouched by this migration. ------------------------------

const frameShopKBV6RUSHA256 = "4ee9bda3f0862fda6a01a8fcd43f655c075593e0c986daa2eb23be79c66ebc11"

func TestFrameShopKBV6RU_MatchesPinnedHash(t *testing.T) {
	sum := sha256.Sum256([]byte(FrameShopKBV6RU()))
	got := hex.EncodeToString(sum[:])
	if got != frameShopKBV6RUSHA256 {
		t.Fatalf("frames/shop-kb-v6-ru.txt sha256 = %s, want %s (the embedded frame changed — see doc comment)", got, frameShopKBV6RUSHA256)
	}
}

const frameShopKBV6TGRUSHA256 = "6bc800df6d24e9021ba0288e7ce5cf85563d1eafc2a741546120f24660060135"

func TestFrameShopKBV6TGRU_MatchesPinnedHash(t *testing.T) {
	sum := sha256.Sum256([]byte(FrameShopKBV6TGRU()))
	got := hex.EncodeToString(sum[:])
	if got != frameShopKBV6TGRUSHA256 {
		t.Fatalf("frames/shop-kb-v6-tg-ru.txt sha256 = %s, want %s (the embedded frame changed — see doc comment)", got, frameShopKBV6TGRUSHA256)
	}
}

// TestFrameShopKBV6TGRU_BodyMatchesGradedFrame is v6's copy of the
// guarantee TestFrameShopKBV5TGRU_BodyMatchesGradedFrame makes for v5: the
// RU and TG frames may differ ONLY in their persona line.
func TestFrameShopKBV6TGRU_BodyMatchesGradedFrame(t *testing.T) {
	ru := FrameShopKBV6RU()
	tg := FrameShopKBV6TGRU()

	ri := strings.Index(ru, "\n")
	ti := strings.Index(tg, "\n")
	if ri < 0 || ti < 0 {
		t.Fatal("a frame has no newline — cannot split off the persona line")
	}
	if ru[ri:] != tg[ti:] {
		t.Fatal("the v6 Telegram frame's body diverged from the v6 RU frame — only the first line may differ")
	}
	if strings.Contains(strings.ToLower(tg[:ti]), "whatsapp") {
		t.Fatalf("the v6 Telegram frame still names WhatsApp in its persona line: %q", tg[:ti])
	}
	if ru[:ri] == tg[:ti] {
		t.Fatal("the v6 Telegram frame's persona line is unchanged — it should be channel-neutral")
	}
}

// TestFrameShopKBV6_IsStrictSupersetOfSlots pins v6's slot surface: every
// v5 slot EXCEPT the two v6 replaces outright (SlotProductsInStock/
// SlotProductsOutOfStock -> SlotProductsAvailable/SlotProductsUnavailable,
// SlotTariffs -> SlotTariffCatalog) must still be present, plus the new
// SlotTariffInfo — the cheap guard against a v6 edit that quietly drops a
// slot its renderer still expects to fill.
func TestFrameShopKBV6_IsStrictSupersetOfSlots(t *testing.T) {
	for _, f := range []struct{ name, text string }{
		{"v6 RU", FrameShopKBV6RU()},
		{"v6 TG", FrameShopKBV6TGRU()},
	} {
		for _, slot := range []string{
			SlotResponseSchema, SlotAssistant, SlotTopics, SlotDeliveryZones, SlotBusinessFacts,
			SlotProductsAvailable, SlotProductsUnavailable, SlotTariffCatalog, SlotTariffInfo,
		} {
			if !strings.Contains(f.text, slot) {
				t.Errorf("%s is missing the %s slot", f.name, slot)
			}
		}
		for _, retired := range []string{SlotProductsInStock, SlotProductsOutOfStock, SlotTariffs} {
			if strings.Contains(f.text, retired) {
				t.Errorf("%s unexpectedly contains the retired slot %s — v6 uses its own product/tariff slots", f.name, retired)
			}
		}
	}
	// v4/v5 must stay completely untouched by this migration.
	if strings.Contains(FrameShopKBV4RU(), SlotProductsAvailable) || strings.Contains(FrameShopKBV5RU(), SlotProductsAvailable) {
		t.Error("v4/v5 unexpectedly contain a v6-only slot — they must stay frozen")
	}
}

func TestPromptRefShopKBV6_Value(t *testing.T) {
	if PromptRefShopKBV6 != "shop-kb@v6" {
		t.Fatalf("PromptRefShopKBV6 = %q, want %q", PromptRefShopKBV6, "shop-kb@v6")
	}
	if PromptRefShopKBV6TG != "shop-kb@v6-tg" {
		t.Fatalf("PromptRefShopKBV6TG = %q, want %q", PromptRefShopKBV6TG, "shop-kb@v6-tg")
	}
}

// --- v7: kb_gap structured escalation diagnostic (0018_kb_gap_telemetry).
// Pinned exactly as v4/v5/v6 are; the same doctrine applies going forward
// (cut a v8, never edit v7 in place once shipped). v4/v5/v6 stay embedded
// and pinned above, completely untouched by this addition. -----------------

const frameShopKBV7RUSHA256 = "bc96db14bbda4cc0c7fc3445892e874b16ef09d7772e2279187f7fe4e22ed183"

func TestFrameShopKBV7RU_MatchesPinnedHash(t *testing.T) {
	sum := sha256.Sum256([]byte(FrameShopKBV7RU()))
	got := hex.EncodeToString(sum[:])
	if got != frameShopKBV7RUSHA256 {
		t.Fatalf("frames/shop-kb-v7-ru.txt sha256 = %s, want %s (the embedded frame changed — see doc comment)", got, frameShopKBV7RUSHA256)
	}
}

const frameShopKBV7TGRUSHA256 = "df6b0aced899cc426e9c877e559a4db8e0427700a776d6ab85eb77ae137effb8"

func TestFrameShopKBV7TGRU_MatchesPinnedHash(t *testing.T) {
	sum := sha256.Sum256([]byte(FrameShopKBV7TGRU()))
	got := hex.EncodeToString(sum[:])
	if got != frameShopKBV7TGRUSHA256 {
		t.Fatalf("frames/shop-kb-v7-tg-ru.txt sha256 = %s, want %s (the embedded frame changed — see doc comment)", got, frameShopKBV7TGRUSHA256)
	}
}

// TestFrameShopKBV7TGRU_BodyMatchesGradedFrame is v7's copy of
// TestFrameShopKBV6TGRU_BodyMatchesGradedFrame: the RU and TG frames may
// differ ONLY in their persona line.
func TestFrameShopKBV7TGRU_BodyMatchesGradedFrame(t *testing.T) {
	ru := FrameShopKBV7RU()
	tg := FrameShopKBV7TGRU()

	ri := strings.Index(ru, "\n")
	ti := strings.Index(tg, "\n")
	if ri < 0 || ti < 0 {
		t.Fatal("a frame has no newline — cannot split off the persona line")
	}
	if ru[ri:] != tg[ti:] {
		t.Fatal("the v7 Telegram frame's body diverged from the v7 RU frame — only the first line may differ")
	}
	if strings.Contains(strings.ToLower(tg[:ti]), "whatsapp") {
		t.Fatalf("the v7 Telegram frame still names WhatsApp in its persona line: %q", tg[:ti])
	}
	if ru[:ri] == tg[:ti] {
		t.Fatal("the v7 Telegram frame's persona line is unchanged — it should be channel-neutral")
	}
}

// TestFrameShopKBV7_IsV6PlusKBGapRule pins v7's relationship to v6: v7 adds
// exactly one new rule (the "kb_gap" diagnostic, rule 9) and changes
// nothing else — every v6 slot stays present, no new %%SLOT%% token is
// introduced (kb_gap rides the existing %%RESPONSE_SCHEMA%% slot via
// RenderResponseSchemaV7), and v6 itself stays completely untouched.
func TestFrameShopKBV7_IsV6PlusKBGapRule(t *testing.T) {
	for _, f := range []struct{ name, text string }{
		{"v7 RU", FrameShopKBV7RU()},
		{"v7 TG", FrameShopKBV7TGRU()},
	} {
		for _, slot := range []string{
			SlotResponseSchema, SlotAssistant, SlotTopics, SlotDeliveryZones, SlotBusinessFacts,
			SlotProductsAvailable, SlotProductsUnavailable, SlotTariffCatalog, SlotTariffInfo,
		} {
			if !strings.Contains(f.text, slot) {
				t.Errorf("%s is missing the %s slot", f.name, slot)
			}
		}
		if !strings.Contains(f.text, "kb_gap") {
			t.Errorf("%s is missing the new kb_gap rule", f.name)
		}
	}
	if strings.Contains(FrameShopKBV6RU(), "kb_gap") || strings.Contains(FrameShopKBV6TGRU(), "kb_gap") {
		t.Error("v6 unexpectedly mentions kb_gap — it must stay frozen")
	}
}

func TestPromptRefShopKBV7_Value(t *testing.T) {
	if PromptRefShopKBV7 != "shop-kb@v7" {
		t.Fatalf("PromptRefShopKBV7 = %q, want %q", PromptRefShopKBV7, "shop-kb@v7")
	}
	if PromptRefShopKBV7TG != "shop-kb@v7-tg" {
		t.Fatalf("PromptRefShopKBV7TG = %q, want %q", PromptRefShopKBV7TG, "shop-kb@v7-tg")
	}
}
