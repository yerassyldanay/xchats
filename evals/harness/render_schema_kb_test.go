package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

const testSchemaKBFixture = `
organization_id: "11111111-1111-1111-1111-111111111111"

ai_assistants:
  persona: "Ты — ассистент интернет-магазина «Demo Shop»."
  guardrails: "Никогда не выдумывай цены и факты."
  language_policy: "Отвечай только на русском языке."

ai_topics:
  - slug: delivery
    title: "Доставка"
    keywords: ["доставка"]
    body_md: "Доставляем по всему Алматы."

ai_products:
  - ref: coffee-machine
    name: "Кофемашина DeLonghi"
    price: "129 900 ₸"
    description: "Автоматическая кофемашина для дома."
    in_stock: true
    sales_status: active
    featured_image: "22222222-2222-2222-2222-222222222222"

ai_policies:
  delivery_cost: "2 000 ₸"

kbd_materials:
  - id: "22222222-2222-2222-2222-222222222222"
    source_type: file
    source_ref: "coffee-front-sentinel.jpg"
    filename: "coffee-front-sentinel.jpg"
    mime_type: "image/jpeg"
    size_bytes: 248315
    storage_backend: fake
    storage_key: "fake://bucket/sentinel-2222"
    processing_status: built
    customer_visibility: visible
`

const testSchemaKBFixtureInvalidMaterial = `
organization_id: "11111111-1111-1111-1111-111111111111"

ai_products:
  - ref: coffee-machine
    name: "Кофемашина DeLonghi"
    price: "129 900 ₸"
    in_stock: true
    sales_status: active
    featured_image: "dangling-material-id-does-not-exist"
`

const testSchemaKBFrame = `# ASSISTANT
%%ASSISTANT%%

# KNOWLEDGE BASE
%%KNOWLEDGE_BASE%%

# DESCRIPTIONS
%%DESCRIPTIONS%%

# FACTS
%%FACTS%%

# MEDIA
%%MEDIA%%

# MEDIA ABSENT
%%MEDIA_ABSENT%%

# RESPONSE SCHEMA
%%RESPONSE_SCHEMA%%
`

const testSchemaKBTests = `
tests:
  - id: "1. price"
    message: "Сколько стоит кофемашина?"
    requires:
      - ["product.coffee-machine.price"]
    escalate: false
    language: ru
`

const testModelsYAML = `
pricing_source: "test"
pricing_checked_at: "2026-01-01"
providers:
  - id: "openrouter:google/gemini-2.5-flash"
    temperature: 0.3
    max_tokens: 800
    default: true
`

// writeSchemaKBScenarioDir materializes a full schema_kb_v1 scenario directory
// (scenario.yaml + fixture + frame + tests) under dir, returning the models.yaml
// path (written as a sibling, matching how evals/ is laid out for real).
func writeSchemaKBScenarioDir(t *testing.T, dir, fixtureYAML string) (scenarioYAMLPath, modelsPath string) {
	t.Helper()
	mustWrite(t, filepath.Join(dir, "data-ru.yaml"), fixtureYAML)
	mustWrite(t, filepath.Join(dir, "frame-ru.txt"), testSchemaKBFrame)
	mustWrite(t, filepath.Join(dir, "tests.yaml"), testSchemaKBTests)
	scenarioYAML := `
name: shop-kb-v1-test
description: "test fixture"
pipeline: schema_kb_v1
frame: frame-ru.txt
data: data-ru.yaml
tests: tests.yaml
`
	scenarioYAMLPath = filepath.Join(dir, "scenario.yaml")
	mustWrite(t, scenarioYAMLPath, scenarioYAML)
	modelsPath = filepath.Join(dir, "models.yaml")
	mustWrite(t, modelsPath, testModelsYAML)
	return scenarioYAMLPath, modelsPath
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestRenderSchemaKBScenario_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	_, modelsPath := writeSchemaKBScenarioDir(t, dir, testSchemaKBFixture)

	if err := renderScenario(dir, modelsPath, ""); err != nil {
		t.Fatalf("renderScenario: %v", err)
	}

	genDir := filepath.Join(dir, "generated")
	promptBytes, err := os.ReadFile(filepath.Join(genDir, "prompt.txt"))
	if err != nil {
		t.Fatalf("read generated prompt.txt: %v", err)
	}
	prompt := string(promptBytes)
	if !strings.Contains(prompt, "{{product.coffee-machine.price}}") {
		t.Error("expected the price placeholder token in the rendered prompt")
	}
	if !strings.Contains(prompt, "products.coffee-machine.featured_image") {
		t.Error("expected the featured_image media token in the rendered prompt")
	}
	if !strings.Contains(prompt, "media_files_to_send") {
		t.Error("expected the response schema block to mention media_files_to_send")
	}
	if strings.Contains(prompt, "%%") {
		t.Errorf("unfilled slot marker leaked into the prompt:\n%s", prompt)
	}
	// The fixture's material fields are deliberately sentinel-flavored (see the
	// "sentinel" substrings above) so this check is unambiguous.
	for _, leak := range []string{
		"22222222-2222-2222-2222-222222222222", "coffee-front-sentinel.jpg",
		"fake://bucket/sentinel-2222", "129 900 ₸", "2 000 ₸",
	} {
		if strings.Contains(prompt, leak) {
			t.Errorf("prompt leaks kbd_materials value %q", leak)
		}
	}

	catBytes, err := os.ReadFile(filepath.Join(genDir, "catalog.json"))
	if err != nil {
		t.Fatalf("read generated catalog.json: %v", err)
	}
	var cat aiprompt.Catalog
	if err := json.Unmarshal(catBytes, &cat); err != nil {
		t.Fatalf("catalog.json did not parse as aiprompt.Catalog: %v\n%s", err, catBytes)
	}
	if len(cat.Facts) == 0 {
		t.Error("expected at least one fact in the exported catalog")
	}
	if len(cat.Media) == 0 {
		t.Error("expected at least one media entry in the exported catalog")
	}
	for _, leak := range []string{
		"22222222-2222-2222-2222-222222222222", "MaterialIDs", "material_ids",
		"129 900 ₸", "2 000 ₸", `"value":`,
	} {
		if strings.Contains(string(catBytes), leak) {
			t.Errorf("exported catalog.json leaks %q", leak)
		}
	}
	stock := cat.FactByToken("{{product.coffee-machine.in_stock}}")
	if stock == nil || stock.ReasoningState != "in_stock" {
		t.Fatalf("stock fact = %+v, want semantic reasoning state", stock)
	}

	if _, err := os.Stat(filepath.Join(genDir, "resolved_tests.json")); err != nil {
		t.Errorf("expected resolved_tests.json to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(genDir, "promptfooconfig.yaml")); err != nil {
		t.Errorf("expected promptfooconfig.yaml to be written: %v", err)
	}
}

// TestRenderSchemaKBScenario_InvalidMaterialFailsClosed proves a dangling media
// reference fails the ENTIRE render before any generated file is written — no
// partial render, matching aiprompt's own fail-closed contract.
func TestRenderSchemaKBScenario_InvalidMaterialFailsClosed(t *testing.T) {
	dir := t.TempDir()
	_, modelsPath := writeSchemaKBScenarioDir(t, dir, testSchemaKBFixtureInvalidMaterial)

	err := renderScenario(dir, modelsPath, "")
	if err == nil {
		t.Fatal("expected renderScenario to fail for a dangling material reference")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "generated", "prompt.txt")); !os.IsNotExist(statErr) {
		t.Error("expected no generated/prompt.txt to be written on a failed render")
	}
}

// TestRenderScenario_DispatchesOnPipeline proves the shared renderScenario entry
// point (used by both `harness render` and `harness run`) routes a
// pipeline:schema_kb_v1 scenario to the new path without needing a separate CLI
// command or caller-side branching.
func TestRenderScenario_DispatchesOnPipeline(t *testing.T) {
	dir := t.TempDir()
	writeSchemaKBScenarioDir(t, dir, testSchemaKBFixture)
	// A legacy scenario (no pipeline: field, Contract required) must still fail
	// fast with the legacy validation error, not silently succeed via the new
	// path or vice versa. Confirms the two paths are genuinely distinct.
	legacyDir := t.TempDir()
	mustWrite(t, filepath.Join(legacyDir, "scenario.yaml"), "name: legacy\ndescription: x\nframe: f.txt\ndata: d.yaml\ntests: t.yaml\n")
	if err := renderScenario(legacyDir, "models.yaml", ""); err == nil {
		t.Fatal("expected the legacy path to fail its own Contract validation")
	} else if strings.Contains(err.Error(), "kbfixture") {
		t.Errorf("a scenario with no pipeline: field must never touch the kbfixture loader, got: %v", err)
	}
}
