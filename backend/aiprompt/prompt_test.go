package aiprompt

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildPrompt_EndToEnd(t *testing.T) {
	kb := baseKB()
	prompt, cat, err := BuildPrompt(basicFrame, kb)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if cat == nil {
		t.Fatal("expected a non-nil catalog")
	}
	if strings.Contains(prompt, "%%") {
		t.Errorf("prompt still contains an unfilled slot marker:\n%s", prompt)
	}
	if !strings.Contains(prompt, "{{product.coffee-machine.price}}") {
		t.Error("expected the price placeholder token to appear in the rendered prompt")
	}
	if !strings.Contains(prompt, "products.coffee-machine.featured_image") {
		t.Error("expected the featured_image media token to appear in the rendered prompt")
	}
	if !strings.Contains(prompt, "\"media_files_to_send\"") {
		t.Error("expected the response schema block to mention media_files_to_send")
	}
}

func TestBuildPrompt_HidesExactFactValues(t *testing.T) {
	kb := baseKB()
	prompt, cat, err := BuildPrompt(basicFrame, kb)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		kb.Products[0].Price,
		kb.Contacts.Phone,
		kb.Contacts.WorkingHours,
		kb.Policies.DeliveryCost,
		kb.Policies.DeliveryInDays,
	} {
		if strings.Contains(prompt, value) {
			t.Errorf("model-facing prompt leaks exact fact value %q", value)
		}
	}
	catJSON, err := json.Marshal(cat)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{kb.Products[0].Price, kb.Contacts.Phone, kb.Policies.DeliveryCost} {
		if strings.Contains(string(catJSON), value) {
			t.Errorf("model-facing catalog leaks exact fact value %q", value)
		}
	}
	if strings.Contains(string(catJSON), `"value":`) {
		t.Fatalf("model-facing catalog still exposes a value property: %s", catJSON)
	}
	stock := cat.FactByToken("{{product.coffee-machine.in_stock}}")
	if stock == nil || stock.ReasoningState != "in_stock" {
		t.Fatalf("stock fact = %+v, want semantic in_stock reasoning state", stock)
	}
}

// TestRenderPrompt_BlankProseOmitted: an empty topic body produces no rendered
// entry (not a blank line), and an empty product description is likewise
// omitted from DESCRIPTIONS.
func TestRenderPrompt_BlankProseOmitted(t *testing.T) {
	kb := baseKB()
	kb.Topics = append(kb.Topics, Topic{Slug: "empty-topic", Title: "Пусто", BodyMD: "   "})
	kb.Products[0].Description = ""

	prompt, _, err := BuildPrompt(basicFrame, kb)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	// The topic itself is still a legitimate approved row (and, having no media
	// columns either, correctly appears in MEDIA ABSENT) — only its KNOWLEDGE
	// BASE entry is suppressed because body_md is blank.
	if strings.Contains(prompt, "# topic: empty-topic") {
		t.Error("a topic with blank body_md must not produce a KNOWLEDGE BASE entry")
	}
	if strings.Contains(prompt, "Кофемашина DeLonghi (Кофемашины): ") {
		t.Error("a product with an empty description must not produce a DESCRIPTIONS line")
	}
}

// TestPromptInput_Projection confirms KB.PromptInput() carries every
// model-visible field through unchanged.
func TestPromptInput_Projection(t *testing.T) {
	kb := baseKB()
	in := kb.PromptInput()
	if in.Assistant != kb.Assistant {
		t.Error("Assistant pointer must be carried through")
	}
	if len(in.Topics) != len(kb.Topics) {
		t.Error("Topics must be carried through")
	}
	if len(in.Products) != len(kb.Products) {
		t.Error("Products must be carried through")
	}
	if len(in.Tariffs) != len(kb.Tariffs) {
		t.Error("Tariffs must be carried through")
	}
	if in.Contacts != kb.Contacts {
		t.Error("Contacts pointer must be carried through")
	}
	if in.Policies != kb.Policies {
		t.Error("Policies pointer must be carried through")
	}
}

func TestRenderPrompt_LeftoverSlotFails(t *testing.T) {
	kb := baseKB()
	cat, err := BuildCatalog(kb)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	_, err = RenderPrompt("hello %%NOT_A_REAL_SLOT%% world", kb.PromptInput(), cat)
	if err == nil {
		t.Fatal("expected an error for a leftover slot marker")
	}
	if !strings.Contains(err.Error(), "unfilled prompt slot") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRenderPrompt_UnresolvedPlaceholderFails: trusted prose that happens to
// contain {{...}}-shaped text not backed by a real fact must fail render, not
// pass through silently.
func TestRenderPrompt_UnresolvedPlaceholderFails(t *testing.T) {
	kb := baseKB()
	kb.Topics[0].BodyMD = "Цена уточняется по токену {{product.fake.price}}."
	_, _, err := BuildPrompt(basicFrame, kb)
	if err == nil {
		t.Fatal("expected an error for a placeholder-shaped string not in the fact catalog")
	}
	if !strings.Contains(err.Error(), "not in the fact catalog") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestBuildPrompt_SentinelLeakTest gives every usable kbd_materials field a
// long, unmistakable sentinel value, renders a prompt, and asserts none of
// them appear anywhere in the rendered text OR in the exported catalog JSON.
// It also asserts the catalog JSON has no route to MaterialIDs.
func TestBuildPrompt_SentinelLeakTest(t *testing.T) {
	const (
		sentinelID      = "sentinel-material-id-a1b2c3d4e5f6"
		sentinelRef     = "sentinel-source-ref-g7h8i9j0k1l2"
		sentinelFile    = "sentinel-filename-m3n4o5p6q7r8.jpg"
		sentinelBackend = "sentinel-storage-backend-s9t0u1v2"
		sentinelKey     = "sentinel-storage-key-w3x4y5z6a7b8"
	)
	sentinels := []string{sentinelID, sentinelRef, sentinelFile, sentinelBackend, sentinelKey}

	kb := baseKB()
	kb.Products[0].FeaturedImage = sentinelID
	kb.Materials = []Material{
		{
			ID:                 sentinelID,
			OrganizationID:     testOrg,
			SourceType:         "file",
			SourceRef:          sentinelRef,
			Filename:           sentinelFile,
			MimeType:           "image/jpeg",
			SizeBytes:          4096,
			StorageBackend:     sentinelBackend,
			StorageKey:         sentinelKey,
			ProcessingStatus:   "built",
			CustomerVisibility: "visible",
		},
	}
	// Drop the now-invalid gallery references from baseKB so only the sentinel
	// material is in play.
	kb.Products[0].GalleryImages = nil

	prompt, cat, err := BuildPrompt(basicFrame, kb)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	for _, s := range sentinels {
		if strings.Contains(prompt, s) {
			t.Errorf("rendered prompt leaks kbd_materials value %q", s)
		}
	}

	catJSON, err := json.Marshal(cat)
	if err != nil {
		t.Fatalf("marshal catalog: %v", err)
	}
	for _, s := range sentinels {
		if strings.Contains(string(catJSON), s) {
			t.Errorf("exported catalog.json leaks kbd_materials value %q: %s", s, catJSON)
		}
	}
	if strings.Contains(string(catJSON), "MaterialIDs") || strings.Contains(string(catJSON), "material_ids") {
		t.Errorf("exported catalog.json must not mention MaterialIDs at all: %s", catJSON)
	}

	// The token itself is still present — only the material's own identifying
	// fields must be absent.
	if !strings.Contains(prompt, "products.coffee-machine.featured_image") {
		t.Error("expected the semantic token itself to still be present in the prompt")
	}
}

// TestValidateNoMaterialLeak_DirectCall exercises the exported helper
// directly (not only through BuildPrompt), matching how a caller using the
// split BuildCatalog+RenderPrompt sequence would invoke it.
func TestValidateNoMaterialLeak_DirectCall(t *testing.T) {
	materials := []Material{{
		ID:             "leaked-material-id-1234567890",
		Filename:       "irrelevant.jpg",
		StorageKey:     "irrelevant-key",
		StorageBackend: "irrelevant-backend",
	}}
	clean := "У нас отличная кофемашина, доставка по всему городу."
	if err := ValidateNoMaterialLeak(clean, materials); err != nil {
		t.Fatalf("unexpected leak on clean text: %v", err)
	}
	leaked := clean + " leaked-material-id-1234567890"
	if err := ValidateNoMaterialLeak(leaked, materials); err == nil {
		t.Fatal("expected a leak error when the material ID literally appears in the text")
	}
}

// TestValidateNoMaterialLeak_ShortValuesSkipped documents that very short
// field values (len < 6) are not blindly substring-matched, since a short
// generic token like a storage backend named "s3" cannot be distinguished
// from ordinary prompt vocabulary.
func TestValidateNoMaterialLeak_ShortValuesSkipped(t *testing.T) {
	materials := []Material{{StorageBackend: "s3", ID: "ab"}}
	text := "s3 является частью названия сервиса, но это совпадение, не утечка."
	if err := ValidateNoMaterialLeak(text, materials); err != nil {
		t.Fatalf("short generic values must be skipped, got: %v", err)
	}
}
