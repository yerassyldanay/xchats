package aiprompt

import (
	"regexp"
	"strings"
	"testing"
)

// v2KB extends baseKB with two more in-stock products to exercise every media
// state the canonical-block renderers must handle: blender-bosch has NO media
// columns populated at all (the block must render zero _ref lines, not an
// "unavailable:" placeholder), and blender-tefal has PARTIAL media (gallery +
// video, no certificate) so its block lists only the populated columns.
// coffee-machine (full media) and cookware-set (out of stock, zero media)
// already exist in baseKB.
func v2KB() *KB {
	kb := baseKB()
	videoMaterial := testMaterial("m-bt-video-1")
	videoMaterial.MimeType = "video/mp4"
	videoMaterial.Filename = "demo-m-bt-video-1.mp4"
	kb.Materials = append(kb.Materials, testMaterial("m-bt-gallery-1"), videoMaterial)
	kb.Products = append(kb.Products,
		Product{
			Ref: "blender-bosch", Name: "Блендер Bosch", Price: "9 900 ₸",
			AvailabilityStatus: "in_stock", SalesStatus: "active",
		},
		Product{
			Ref: "blender-tefal", Name: "Блендер Tefal", Price: "14 900 ₸",
			AvailabilityStatus: "in_stock",
			SalesStatus:        "active",
			GalleryImages:      []string{"m-bt-gallery-1"},
			DemoVideos:         []string{"m-bt-video-1"},
		},
	)
	return kb
}

func TestRenderProductsInStock_FullMediaBlockListsEveryPopulatedColumnOnce(t *testing.T) {
	kb := v2KB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	out := renderProductsInStock(kb.PromptInput(), cat)

	for _, want := range []string{
		"product: coffee-machine",
		"name: " + kb.Products[0].Name,
		"description: " + kb.Products[0].Description,
		"price_placeholder: {{product.coffee-machine.price}}",
		"featured_image_ref: products.coffee-machine.featured_image",
		"gallery_images_ref: products.coffee-machine.gallery_images",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want block to contain %q, got:\n%s", want, out)
		}
		if n := strings.Count(out, want); n != 1 {
			t.Errorf("want %q to appear exactly once, appeared %d times in:\n%s", want, n, out)
		}
	}
	// coffee-machine has no demo_videos/certificate_documents/etc. populated —
	// none of those _ref lines may appear for it. stock_placeholder must never
	// appear at all (2026-07-26: in_stock is no longer a fact token — see
	// registry.go).
	for _, absent := range []string{"demo_videos_ref: products.coffee-machine", "certificate_documents_ref: products.coffee-machine", "stock_placeholder"} {
		if strings.Contains(out, absent) {
			t.Errorf("did not want %q, an unpopulated column, got:\n%s", absent, out)
		}
	}
}

func TestRenderProductsInStock_MediaLessProductRendersZeroRefLines(t *testing.T) {
	kb := v2KB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	out := renderProductsInStock(kb.PromptInput(), cat)

	if !strings.Contains(out, "product: blender-bosch") {
		t.Fatalf("want blender-bosch's block present, got:\n%s", out)
	}
	if !strings.Contains(out, "price_placeholder: {{product.blender-bosch.price}}") {
		t.Errorf("want blender-bosch's price placeholder present, got:\n%s", out)
	}
	if strings.Contains(out, "_ref: products.blender-bosch.") {
		t.Errorf("blender-bosch has no media at all — want ZERO _ref lines for it, got:\n%s", out)
	}
	if strings.Contains(out, "unavailable") {
		t.Errorf("v2 blocks must never render an \"unavailable:\" placeholder line, got:\n%s", out)
	}
}

func TestRenderProductsInStock_PartialMediaListsOnlyPopulatedColumns(t *testing.T) {
	kb := v2KB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	out := renderProductsInStock(kb.PromptInput(), cat)

	if !strings.Contains(out, "gallery_images_ref: products.blender-tefal.gallery_images") {
		t.Errorf("want blender-tefal's populated gallery column present, got:\n%s", out)
	}
	if !strings.Contains(out, "demo_videos_ref: products.blender-tefal.demo_videos") {
		t.Errorf("want blender-tefal's populated video column present, got:\n%s", out)
	}
	if strings.Contains(out, "certificate_documents_ref: products.blender-tefal") {
		t.Errorf("blender-tefal has no certificate — must be omitted entirely, not a placeholder, got:\n%s", out)
	}
}

// TestRenderProductsOutOfStock_NameOnlyNoFactsNoMedia confirms cookware-set
// (baseKB's out-of-stock product) contributes ONLY its display name — no ref,
// no description, no fact placeholder, no media reference anywhere.
func TestRenderProductsOutOfStock_NameOnlyNoFactsNoMedia(t *testing.T) {
	kb := v2KB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	inStock := renderProductsInStock(kb.PromptInput(), cat)
	outOfStock := renderProductsOutOfStock(kb.PromptInput())

	if !strings.Contains(outOfStock, "- "+kb.Products[1].Name) {
		t.Fatalf("want the out-of-stock product's name present, got:\n%s", outOfStock)
	}
	if strings.Contains(outOfStock, "cookware-set") {
		t.Errorf("out-of-stock list must be name-only — no ref, got:\n%s", outOfStock)
	}
	if strings.Contains(outOfStock, kb.Products[1].Price) {
		t.Errorf("out-of-stock list must carry no price, got:\n%s", outOfStock)
	}
	if strings.Contains(inStock, "cookware-set") || strings.Contains(inStock, kb.Products[1].Name) {
		t.Errorf("an out-of-stock product must not appear in the in-stock block at all, got:\n%s", inStock)
	}
}

func TestRenderTopicBlocks_EmptyFieldsOmitted(t *testing.T) {
	kb := v2KB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	out := renderTopicBlocks(kb.PromptInput(), cat)
	if !strings.Contains(out, "topic: delivery") {
		t.Fatalf("want the delivery topic's block present, got:\n%s", out)
	}
	if !strings.Contains(out, "title: "+kb.Topics[0].Title) {
		t.Errorf("want the topic's title present, got:\n%s", out)
	}
	if !strings.Contains(out, "body: "+kb.Topics[0].BodyMD) {
		t.Errorf("want the topic's body present, got:\n%s", out)
	}
	// baseKB's delivery topic has no media populated — no _ref line for it.
	if strings.Contains(out, "_ref: topics.delivery.") {
		t.Errorf("delivery topic has no media — want zero _ref lines, got:\n%s", out)
	}
}

func TestRenderBusinessFacts_ExcludesProductFactsIncludesRest(t *testing.T) {
	kb := v2KB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatal(err)
	}
	out := renderBusinessFacts(cat.Facts)
	if strings.Contains(out, "{{product.") {
		t.Errorf("BUSINESS_FACTS must never carry a product fact (those live in the per-product blocks), got:\n%s", out)
	}
	if !strings.Contains(out, "{{contact.main.phone}}") {
		t.Errorf("want the contact phone fact present, got:\n%s", out)
	}
	if !strings.Contains(out, "{{policy.main.delivery_cost}}") {
		t.Errorf("want the flat delivery_cost policy fact present, got:\n%s", out)
	}
}

// TestRenderPrompt_V2Frame_EveryTokenExactlyOnceNoLeftoverSlots is the exactly-
// once invariant across the WHOLE rendered v2 prompt: every fact placeholder and
// media reference the v2 renderers actually SHOW the model appears exactly one
// time (never duplicated across the per-product/topic blocks and BUSINESS_FACTS
// the way the v1 flat rendering could in principle repeat a concept across
// sections), and no %%SLOT%% marker survives. cat.Facts/cat.Media themselves
// still carry entries the v2 frame deliberately never renders — tariff facts
// (no v2 tariff block yet, see renderBusinessFacts) are correctly EXCLUDED
// from this check, not a violation of the invariant. An unavailable
// product's facts/media are no longer generated at all (productVisible,
// catalog.go), so there is nothing left to exclude for those.
func TestRenderPrompt_V2Frame_EveryTokenExactlyOnceNoLeftoverSlots(t *testing.T) {
	kb := v2KB()
	prompt, cat, err := BuildPrompt(v2Frame, kb)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if strings.Contains(prompt, "%%") {
		t.Errorf("prompt still contains an unfilled slot marker:\n%s", prompt)
	}
	visible := map[string]bool{}
	for i := range kb.Products {
		if productVisible(&kb.Products[i]) {
			visible[kb.Products[i].Ref] = true
		}
	}
	for _, f := range cat.Facts {
		switch f.Table {
		case "product":
			if !visible[f.Ref] {
				continue // unavailable product: no longer cataloged at all
			}
		case "tariff":
			continue // no v2 tariff block yet (renderBusinessFacts's own doc comment)
		}
		if n := strings.Count(prompt, f.Token); n != 1 {
			t.Errorf("fact token %s appears %d times, want exactly 1:\n%s", f.Token, n, prompt)
		}
	}
	for _, m := range cat.Media {
		if m.Table == "products" && !visible[m.Ref] {
			continue // unavailable product: no longer cataloged at all
		}
		if n := strings.Count(prompt, m.Token); n != 1 {
			t.Errorf("media token %s appears %d times, want exactly 1:\n%s", m.Token, n, prompt)
		}
	}
	if err := ValidatePrompt(prompt, cat); err != nil {
		t.Errorf("ValidatePrompt: %v", err)
	}
	if err := ValidateNoMaterialLeak(prompt, kb.Materials); err != nil {
		t.Errorf("ValidateNoMaterialLeak: %v", err)
	}
}

// TestRenderPrompt_V2Frame_NoUnavailablePlaceholderAnywhere confirms the
// empty-media decision holds across the WHOLE rendered prompt, not just one
// renderer in isolation: v2 never emits any per-entity absence marker.
func TestRenderPrompt_V2Frame_NoUnavailablePlaceholderAnywhere(t *testing.T) {
	kb := v2KB()
	prompt, _, err := BuildPrompt(v2Frame, kb)
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?i)unavailable`).MatchString(prompt) {
		t.Errorf("v2 prompt must never contain an \"unavailable\"-style placeholder, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "media_refs: []") {
		t.Errorf("v2 prompt must never render a media_refs: [] absence marker, got:\n%s", prompt)
	}
}

// TestRenderPrompt_V2Frame_RenderFailStopOnInvalidMaterial confirms the v2 path
// inherits BuildCatalog's existing fail-closed material validation unchanged —
// the new renderers read ONLY from the already-validated Catalog (cat.Media),
// never from kb.Materials directly, so an invalid reference still stops
// generation with an error rather than silently omitting the column.
func TestRenderPrompt_V2Frame_RenderFailStopOnInvalidMaterial(t *testing.T) {
	kb := v2KB()
	kb.Materials[0].StorageKey = "" // coffee-machine's featured_image material, now invalid
	if _, _, err := BuildPrompt(v2Frame, kb); err == nil {
		t.Fatal("want BuildPrompt to fail on an invalid material reference, got nil error")
	}
}
