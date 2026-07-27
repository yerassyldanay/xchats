package main

import (
	"os"
	"strings"
	"testing"

	"xchats-evals-harness/internal/kbfixture"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// shopKBV1FixturePath/shopKBV1FramePath are the real shared source pool for
// the shop-kb-v1-10/-50/-100 scenario family (see scenario.yaml's frame:/
// data: fields) — relative to evals/harness, this package's `go test` working
// directory.
const (
	shopKBV1FixturePath = "../scenarios/shop-kb-v1/data-ru.yaml"
	shopKBV1FramePath   = "../scenarios/shop-kb-v1/frame-ru.txt"
)

// TestShopKBV1_FrameRendersAtEverySize renders the REAL shop-kb-v1 frame
// against the REAL shared fixture at every catalog size the family ships
// (10/50/100), exactly the way renderSchemaKBScenario does for a live run —
// without needing tests.yaml/models.yaml, since this test only asserts the
// render succeeds and inspects its output.
//
// This is the regression test for the 2026-07-24 blocker where an uncommitted
// frame edit contained a literal "{{...}}" brace pair: since ValidatePrompt
// treats any "{{...}}" not in the fact catalog as a leaked placeholder, that
// edit made every shop-kb-v1-* scenario fail to render at all — a failure
// this test would have caught immediately, at the frame level, without
// needing a scenario.yaml or a live model call.
//
// It also locks the "catalog size is the only swept variable" invariant for
// the new %%DELIVERY_ZONES%% block specifically: ai_delivery_zones is never
// subject to ApplyLimits (only ai_topics/ai_products/ai_tariffs are), so the
// rendered zones section must be byte-identical across all three sizes.
func TestShopKBV1_FrameRendersAtEverySize(t *testing.T) {
	frameBytes, err := os.ReadFile(shopKBV1FramePath)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}

	zonesBlocks := map[int]string{}
	for _, size := range []int{10, 50, 100} {
		kb, err := kbfixture.Load(shopKBV1FixturePath)
		if err != nil {
			t.Fatalf("size %d: kbfixture.Load: %v", size, err)
		}
		kb, err = kbfixture.ApplyLimits(kb, map[string]int{"ai_products": size})
		if err != nil {
			t.Fatalf("size %d: ApplyLimits: %v", size, err)
		}
		cat, err := aiprompt.BuildCatalog(kb)
		if err != nil {
			t.Fatalf("size %d: BuildCatalog: %v", size, err)
		}
		prompt, err := aiprompt.RenderPrompt(string(frameBytes), kb.PromptInput(), cat)
		if err != nil {
			t.Fatalf("size %d: RenderPrompt: %v (this is the regression this test guards: a frame edit that hard-fails rendering at every size)", size, err)
		}
		if err := aiprompt.ValidateNoMaterialLeak(prompt, kb.Materials); err != nil {
			t.Fatalf("size %d: ValidateNoMaterialLeak: %v", size, err)
		}
		if strings.Contains(prompt, "%%") {
			t.Errorf("size %d: prompt still contains an unfilled slot marker", size)
		}

		// Anchor on "справочник зон" (the section header) rather than "ЗОНЫ
		// ДОСТАВКИ" alone — that phrase also appears earlier in the frame's
		// ИСТОЧНИК ФАКТОВ intro bullet, so a bare Index would find the wrong
		// occurrence.
		start := strings.Index(prompt, "ЗОНЫ ДОСТАВКИ — справочник зон")
		end := strings.Index(prompt, "BUSINESS FACTS:")
		if start == -1 || end == -1 || end < start {
			t.Fatalf("size %d: could not locate the ЗОНЫ ДОСТАВКИ section in the rendered prompt", size)
		}
		zonesBlocks[size] = prompt[start:end]
	}

	if zonesBlocks[10] != zonesBlocks[50] || zonesBlocks[50] != zonesBlocks[100] {
		t.Errorf("ЗОНЫ ДОСТАВКИ section must be byte-identical across catalog sizes (zones are never limited), got:\nsize 10:\n%s\n\nsize 50:\n%s\n\nsize 100:\n%s",
			zonesBlocks[10], zonesBlocks[50], zonesBlocks[100])
	}
}
