package aiprompt

import (
	"strings"
	"testing"
)

// This file covers renderTariffs and the v5 frame's %%TARIFFS%% slot.
//
// The bug it exists to prevent, stated plainly: before v5 every ai_tariffs row
// was invisible to the model. BuildCatalog built tariff facts and media the
// whole time, but renderBusinessFacts filtered tariff facts out and no frame
// slot carried them, so an operator who priced a tariff in its typed columns —
// exactly where the publish gate pushes a price — watched the assistant answer
// "у меня нет этой информации" and escalate.

// tariffKB extends baseKB (one plain active tariff, "basic") with the rest of
// the states the block must handle: a fee-bearing percentage plan, a fully
// prose-and-media populated plan, and an inactive one that must never render.
func tariffKB() *KB {
	kb := baseKB()
	kb.Materials = append(kb.Materials,
		testMaterial("m-tf-pricing-1"),
		testDocMaterial("m-tf-terms-1"),
	)
	kb.Tariffs = append(kb.Tariffs,
		Tariff{
			Ref: "pro", Name: "Профи", Price: "25 000 ₸",
			Summary: "Расширенный тариф для бизнеса.", LimitText: "До 100 заявок",
			Advantages: "Приоритетная поддержка.", Disadvantages: "Дороже базового.",
			PricingType: "fixed", SalesStatus: "active",
			PricingImages:  []string{"m-tf-pricing-1"},
			TermsDocuments: []string{"m-tf-terms-1"},
		},
		Tariff{
			Ref: "commission", Name: "Комиссионный", Fee: "1,5%",
			PricingType: "percentage", SalesStatus: "active",
		},
		Tariff{
			Ref: "retired", Name: "Архивный", Price: "1 000 ₸",
			PricingType: "fixed", SalesStatus: "inactive",
		},
	)
	return kb
}

// blockFor returns the single rendered block whose first line names ref.
// Assertions scope to one block because values like "pricing_type: fixed" are
// legitimately shared across tariffs — only within a block is each line unique.
func blockFor(t *testing.T, rendered, ref string) string {
	t.Helper()
	for _, b := range strings.Split(rendered, "\n\n") {
		if strings.HasPrefix(b, "tariff: "+ref+"\n") || b == "tariff: "+ref {
			return b
		}
	}
	t.Fatalf("no block rendered for tariff %q in:\n%s", ref, rendered)
	return ""
}

func TestRenderTariffs_BlockCarriesNamePricingTypeProseAndPlaceholders(t *testing.T) {
	kb := tariffKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	pro := blockFor(t, renderTariffs(kb.PromptInput(), cat), "pro")

	for _, want := range []string{
		"name: Профи",
		"pricing_type: fixed",
		"summary: Расширенный тариф для бизнеса.",
		"limit: До 100 заявок",
		"advantages: Приоритетная поддержка.",
		"disadvantages: Дороже базового.",
		"price_placeholder: {{tariff.pro.price}}",
		"pricing_images_ref: tariffs.pro.pricing_images",
		"terms_documents_ref: tariffs.pro.terms_documents",
	} {
		if n := strings.Count(pro, want); n != 1 {
			t.Errorf("want %q exactly once in the pro block, got %d:\n%s", want, n, pro)
		}
	}
}

// TestRenderTariffs_FeaturedImageFallsBackToFirstPricingImage documents a
// non-obvious inherited behavior the block surfaces: a tariff with no explicit
// featured_image still gets a featured_image_ref, because catalog.go's
// tariffMedia resolves that column through resolvedFeaturedImage(featured,
// pricing_images) — the same "explicit wins, empty falls back to the first
// primary image" rule products follow. The ref is therefore real and
// attachable, not phantom.
func TestRenderTariffs_FeaturedImageFallsBackToFirstPricingImage(t *testing.T) {
	kb := tariffKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	pro := blockFor(t, renderTariffs(kb.PromptInput(), cat), "pro")

	if !strings.Contains(pro, "featured_image_ref: tariffs.pro.featured_image") {
		t.Errorf("want the inherited featured_image ref on a tariff with pricing_images, got:\n%s", pro)
	}
	if cat.MediaByToken("tariffs.pro.featured_image") == nil {
		t.Error("the rendered featured_image ref must resolve in the media catalog, not dangle")
	}
}

// A percentage plan prices itself with a FEE and no price at all — the case
// pricing_type exists to disambiguate, since "1,5%" is a rate rather than an
// amount.
func TestRenderTariffs_FeeOnlyPlanRendersFeePlaceholderAndNoPricePlaceholder(t *testing.T) {
	kb := tariffKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	out := renderTariffs(kb.PromptInput(), cat)

	if !strings.Contains(out, "fee_placeholder: {{tariff.commission.fee}}") {
		t.Errorf("want the commission block to carry its fee placeholder, got:\n%s", out)
	}
	if !strings.Contains(out, "pricing_type: percentage") {
		t.Errorf("want pricing_type: percentage on the fee-only plan, got:\n%s", out)
	}
	if strings.Contains(out, "{{tariff.commission.price}}") {
		t.Errorf("commission has no price — it must not get a price placeholder:\n%s", out)
	}
}

// Empty fields are omitted rather than rendered blank, exactly as
// renderProductsInStock treats an empty description.
func TestRenderTariffs_OmitsEmptyFields(t *testing.T) {
	kb := tariffKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	blocks := strings.Split(renderTariffs(kb.PromptInput(), cat), "\n\n")

	var commission string
	for _, b := range blocks {
		if strings.HasPrefix(b, "tariff: commission") {
			commission = b
		}
	}
	if commission == "" {
		t.Fatal("no commission block rendered")
	}
	for _, absent := range []string{"summary:", "limit:", "advantages:", "disadvantages:", "_ref:"} {
		if strings.Contains(commission, absent) {
			t.Errorf("commission has no %s value — the line must be omitted, got:\n%s", absent, commission)
		}
	}
}

func TestRenderTariffs_SkipsInactiveTariff(t *testing.T) {
	kb := tariffKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	out := renderTariffs(kb.PromptInput(), cat)

	if strings.Contains(out, "retired") || strings.Contains(out, "Архивный") {
		t.Errorf("an inactive tariff must never render, got:\n%s", out)
	}
}

func TestRenderTariffs_NoActiveTariffsRendersEmDash(t *testing.T) {
	kb := baseKB()
	kb.Tariffs = nil
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	if got := renderTariffs(kb.PromptInput(), cat); got != "—" {
		t.Errorf("renderTariffs with no tariffs = %q, want the em dash placeholder", got)
	}
}

// TestRenderBusinessFacts_StillExcludesTariffFacts is the other half of the
// one-place rule: now that tariffs render in their own block, BUSINESS FACTS
// must NOT also carry them, or every tariff price would appear in the prompt
// twice.
func TestRenderBusinessFacts_StillExcludesTariffFacts(t *testing.T) {
	kb := tariffKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	if got := renderBusinessFacts(cat.Facts); strings.Contains(got, "{{tariff.") {
		t.Errorf("BUSINESS FACTS must not repeat tariff facts (they live in the tariff block), got:\n%s", got)
	}
}

// TestFrameShopKBV5RU_RendersTariffFacts is the end-to-end proof through the
// REAL v5 frame: the exact reported symptom was a priced tariff the model never
// saw, so this asserts the placeholder and the tariff's name both survive a
// full BuildPrompt into the rendered text.
func TestFrameShopKBV5RU_RendersTariffFacts(t *testing.T) {
	kb := tariffKB()
	prompt, cat, err := BuildPrompt(FrameShopKBV5RU(), kb)
	if err != nil {
		t.Fatalf("BuildPrompt(v5): %v", err)
	}

	for _, want := range []string{
		"ТАРИФЫ",
		"tariff: pro",
		"name: Профи",
		"{{tariff.pro.price}}",
		"{{tariff.commission.fee}}",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("v5 prompt is missing %q", want)
		}
	}
	if strings.Contains(prompt, "Архивный") {
		t.Error("v5 prompt leaked an inactive tariff")
	}
	// Every tariff placeholder the prompt shows must be a real catalog entry —
	// ValidatePrompt already enforces this inside BuildPrompt, but asserting it
	// here documents that the tariff tokens are resolvable, not decorative.
	for _, tok := range []string{"{{tariff.pro.price}}", "{{tariff.commission.fee}}"} {
		if cat.FactByToken(tok) == nil {
			t.Errorf("%s is rendered but absent from the fact catalog", tok)
		}
	}
}

// TestFrameShopKBV4RU_CannotRenderTariffs pins the regression itself: v4 is
// kept precisely because old drafts name it, and rendering one must still
// reproduce v4's behavior — tariffs absent. If this ever starts passing
// tariffs through, v4 has been edited in place and every "shop-kb@v4" draft
// has been silently reinterpreted.
func TestFrameShopKBV4RU_CannotRenderTariffs(t *testing.T) {
	kb := tariffKB()
	prompt, _, err := BuildPrompt(FrameShopKBV4RU(), kb)
	if err != nil {
		t.Fatalf("BuildPrompt(v4): %v", err)
	}
	for _, absent := range []string{"{{tariff.pro.price}}", "{{tariff.commission.fee}}", "tariff: pro"} {
		if strings.Contains(prompt, absent) {
			t.Errorf("v4 unexpectedly rendered %q — v4 must stay frozen at its graded behavior", absent)
		}
	}
}
