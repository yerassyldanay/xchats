package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateCatalog(t *testing.T) {
	if err := validateCatalog(&Catalog{Tokens: []CatalogFact{{Token: "{{a.b.c}}", Value: "129 900 ₸"}}}); err != nil {
		t.Fatalf("brace-free value should pass, got: %v", err)
	}
	err := validateCatalog(&Catalog{Tokens: []CatalogFact{{Token: "{{a.b.c}}", Value: "129{900 ₸"}}})
	if err == nil {
		t.Fatal("value containing a brace character should fail validateCatalog")
	}
}

func TestBuildCatalog_TrustedDigits(t *testing.T) {
	data := &Data{
		FactTables: []FactTable{
			{
				Table: "product",
				Fields: []FieldSpec{
					{Name: "price", ValueKind: "money_display"},
				},
				Rows: []FactRow{
					{
						Ref:         "kettle-tefal",
						DisplayName: "Чайник Tefal",
						Description: "электрочайник, 1.7 л, быстрое закипание.",
						Values:      map[string]string{"price": "12 900 ₸"},
					},
				},
			},
		},
	}
	cat := buildCatalog(data, "attach_groups")
	want := map[string]bool{"1": true, "7": true}
	got := map[string]bool{}
	for _, d := range cat.TrustedDigits {
		got[d] = true
	}
	for d := range want {
		if !got[d] {
			t.Fatalf("expected TrustedDigits to contain %q (from row Description), got %v", d, cat.TrustedDigits)
		}
	}
}

func TestValidatePrompt(t *testing.T) {
	cat := &Catalog{Tokens: []CatalogFact{{Token: "{{product.coffee-machine.price}}", Value: "129 900 ₸"}}}

	valid := "FACTS:\n{{product.coffee-machine.price}} | цена | 129 900 ₸\nКлиент пишет: {{message}}\nИстория: {{history}}\n"
	if err := validatePrompt(valid, cat); err != nil {
		t.Fatalf("prompt using only catalog tokens + promptfoo vars should pass, got: %v", err)
	}

	unknownToken := "Цена: {{product.unknown.field}}\nКлиент пишет: {{message}}\n"
	if err := validatePrompt(unknownToken, cat); err == nil {
		t.Fatal("prompt referencing a token not in the catalog should fail validatePrompt")
	}

	unfilledSlot := "%%FACTS%%\nКлиент пишет: {{message}}\n"
	if err := validatePrompt(unfilledSlot, cat); err == nil {
		t.Fatal("prompt with a leftover %%SLOT%% should fail validatePrompt")
	}
}

func TestBuildPassthrough(t *testing.T) {
	if got := buildPassthrough(ModelProvider{ID: "m"}); got != nil {
		t.Fatalf("want nil passthrough when neither Provider nor Reasoning is set, got %+v", got)
	}

	allowFallbacks := false
	route := buildPassthrough(ModelProvider{
		ID:       "m",
		Provider: &ProviderRoute{Order: []string{"Google AI Studio"}, AllowFallbacks: &allowFallbacks},
	})
	if route == nil {
		t.Fatal("want a non-nil passthrough map")
	}
	providerMap, ok := route["provider"].(map[string]any)
	if !ok {
		t.Fatalf("want a provider sub-map, got %+v", route)
	}
	if order, _ := providerMap["order"].([]string); len(order) != 1 || order[0] != "Google AI Studio" {
		t.Errorf("want order=[Google AI Studio], got %+v", providerMap["order"])
	}
	if fb, _ := providerMap["allow_fallbacks"].(bool); fb {
		t.Errorf("want allow_fallbacks=false, got %+v", providerMap["allow_fallbacks"])
	}
	if _, hasReasoning := route["reasoning"]; hasReasoning {
		t.Error("want no reasoning key when Reasoning is unset")
	}

	reasoningPT := buildPassthrough(ModelProvider{ID: "m", Reasoning: &ReasoningConfig{Enabled: true, MaxTokens: 500}})
	reasoningMap, ok := reasoningPT["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("want a reasoning sub-map, got %+v", reasoningPT)
	}
	if en, _ := reasoningMap["enabled"].(bool); !en {
		t.Error("want enabled=true")
	}
	if mt, _ := reasoningMap["max_tokens"].(int); mt != 500 {
		t.Errorf("want max_tokens=500, got %+v", reasoningMap["max_tokens"])
	}
	if _, hasProvider := reasoningPT["provider"]; hasProvider {
		t.Error("want no provider key when Provider is unset")
	}
}

// providerYAMLEntries parses generated promptfooconfig.yaml's providers list back into
// generic maps — structural parsing, not substring matching, since a substring search
// for "label:" would false-positive on the UNRELATED, pre-existing prompts[].label field
// (always set to the scenario name) that has nothing to do with ModelProvider.Label.
func providerYAMLEntries(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Providers []map[string]any `yaml:"providers"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse generated promptfooconfig.yaml: %v\n%s", err, b)
	}
	return doc.Providers
}

// TestWritePromptfooConfig_PassthroughOmittedWhenUnset locks in the "omitted entirely,
// not present-as-null" contract for today's 4 models.yaml entries (none set Provider,
// Reasoning, or Label) — this is what makes the addition backward-compatible: an
// existing scenario's generated promptfooconfig.yaml must not change shape at all.
func TestWritePromptfooConfig_PassthroughOmittedWhenUnset(t *testing.T) {
	dir := t.TempDir()
	scenario := &ScenarioConfig{Name: "fixture", Contract: "asset_refs"}
	models := &ModelsFile{Providers: []ModelProvider{{ID: "openrouter:x/y", Temperature: 0.3, MaxTokens: 500}}}
	if err := writePromptfooConfig(dir, scenario, nil, models); err != nil {
		t.Fatal(err)
	}
	entries := providerYAMLEntries(t, filepath.Join(dir, "promptfooconfig.yaml"))
	if len(entries) != 1 {
		t.Fatalf("want 1 provider entry, got %d", len(entries))
	}
	if _, has := entries[0]["label"]; has {
		t.Errorf("want no label key on the PROVIDER entry when ModelProvider.Label is unset, got %+v", entries[0])
	}
	cfg, ok := entries[0]["config"].(map[string]any)
	if !ok {
		t.Fatalf("want a config map, got %+v", entries[0])
	}
	if _, has := cfg["passthrough"]; has {
		t.Errorf("want no passthrough key when neither Provider nor Reasoning is set, got %+v", cfg)
	}
}

// TestWritePromptfooConfig_PassthroughAndLabelPresentWhenSet proves the opposite side:
// a provider entry with Label/Provider/Reasoning all set produces the real promptfoo
// wire shape (confirmed against evals/results/results.json — passthrough is a real,
// recognized config key promptfoo's OpenRouter provider forwards upstream).
func TestWritePromptfooConfig_PassthroughAndLabelPresentWhenSet(t *testing.T) {
	dir := t.TempDir()
	scenario := &ScenarioConfig{Name: "fixture", Contract: "asset_refs"}
	allowFallbacks := false
	models := &ModelsFile{Providers: []ModelProvider{{
		ID: "openrouter:google/gemini-2.5-flash", Temperature: 0.3, MaxTokens: 500,
		Label:     "reasoning-on",
		Provider:  &ProviderRoute{Order: []string{"Google AI Studio"}, AllowFallbacks: &allowFallbacks},
		Reasoning: &ReasoningConfig{Enabled: true, Effort: "low"},
	}}}
	if err := writePromptfooConfig(dir, scenario, nil, models); err != nil {
		t.Fatal(err)
	}
	entries := providerYAMLEntries(t, filepath.Join(dir, "promptfooconfig.yaml"))
	if len(entries) != 1 {
		t.Fatalf("want 1 provider entry, got %d", len(entries))
	}
	if entries[0]["label"] != "reasoning-on" {
		t.Errorf("want provider label=reasoning-on, got %+v", entries[0]["label"])
	}
	cfg, ok := entries[0]["config"].(map[string]any)
	if !ok {
		t.Fatalf("want a config map, got %+v", entries[0])
	}
	passthrough, ok := cfg["passthrough"].(map[string]any)
	if !ok {
		t.Fatalf("want a passthrough map, got %+v", cfg)
	}
	providerMap, ok := passthrough["provider"].(map[string]any)
	if !ok {
		t.Fatalf("want passthrough.provider, got %+v", passthrough)
	}
	if order, _ := providerMap["order"].([]any); len(order) != 1 || order[0] != "Google AI Studio" {
		t.Errorf("want passthrough.provider.order=[Google AI Studio], got %+v", providerMap["order"])
	}
	if fb, _ := providerMap["allow_fallbacks"].(bool); fb {
		t.Errorf("want passthrough.provider.allow_fallbacks=false, got %+v", providerMap["allow_fallbacks"])
	}
	reasoningMap, ok := passthrough["reasoning"].(map[string]any)
	if !ok {
		t.Fatalf("want passthrough.reasoning, got %+v", passthrough)
	}
	if reasoningMap["effort"] != "low" {
		t.Errorf("want passthrough.reasoning.effort=low, got %+v", reasoningMap["effort"])
	}
}
