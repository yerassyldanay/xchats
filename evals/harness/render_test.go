package main

import (
	"os"
	"path/filepath"
	"strings"
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
	cat := buildCatalog(data)
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

// TestValidateTestMedia_ForbidWithAnyOfConflicts proves render's fail-closed gate against
// a self-contradictory media expectation: Forbid (no attachment allowed) combined with
// AnyOf/AnyOf (attach one of these) can never both be satisfied by one reply.
func TestValidateTestMedia_ForbidWithAnyOfConflicts(t *testing.T) {
	if err := validateTestMedia("s1", "t1", nil); err != nil {
		t.Errorf("nil media should never conflict, got %v", err)
	}
	if err := validateTestMedia("s1", "t1", &MediaExpect{}); err != nil {
		t.Errorf("Forbid=false with no any_of should never conflict, got %v", err)
	}
	if err := validateTestMedia("s1", "t1", &MediaExpect{Forbid: true}); err != nil {
		t.Errorf("Forbid alone (no any_of) should never conflict, got %v", err)
	}
	if err := validateTestMedia("s1", "t1", &MediaExpect{AnyOf: []string{"g"}}); err != nil {
		t.Errorf("any_of alone (Forbid=false) should never conflict, got %v", err)
	}

	err := validateTestMedia("s1", "t1", &MediaExpect{Forbid: true, AnyOf: []string{"g"}})
	if err == nil {
		t.Fatal("want an error when Forbid is combined with any_of_groups")
	}
	if !strings.Contains(err.Error(), "s1") || !strings.Contains(err.Error(), "t1") {
		t.Errorf("want the error to name the scenario and test, got %v", err)
	}

	if err := validateTestMedia("s1", "t1", &MediaExpect{Forbid: true, AnyOf: []string{"r"}}); err == nil {
		t.Error("want an error when Forbid is combined with any_of_refs too")
	}
}

// TestValidateTestMedia_ExclusiveConstraints proves the two invariants Exclusive adds:
// it cannot stand alone without a non-empty any_of_* to narrow, and it cannot combine
// with Forbid (which already means "nothing is allowed", leaving nothing for Exclusive
// to scope).
func TestValidateTestMedia_ExclusiveConstraints(t *testing.T) {
	if err := validateTestMedia("s1", "t1", &MediaExpect{AnyOf: []string{"g"}, Exclusive: true}); err != nil {
		t.Errorf("Exclusive with a non-empty any_of_groups should never conflict, got %v", err)
	}
	if err := validateTestMedia("s1", "t1", &MediaExpect{AnyOf: []string{"r"}, Exclusive: true}); err != nil {
		t.Errorf("Exclusive with a non-empty any_of_refs should never conflict, got %v", err)
	}

	err := validateTestMedia("s1", "t1", &MediaExpect{Exclusive: true})
	if err == nil {
		t.Fatal("want an error when Exclusive is set with neither any_of_groups nor any_of_refs")
	}
	if !strings.Contains(err.Error(), "s1") || !strings.Contains(err.Error(), "t1") {
		t.Errorf("want the error to name the scenario and test, got %v", err)
	}

	if err := validateTestMedia("s1", "t1", &MediaExpect{Forbid: true, Exclusive: true}); err == nil {
		t.Error("want an error when Forbid is combined with Exclusive, even with no any_of_* set")
	}
	if err := validateTestMedia("s1", "t1", &MediaExpect{Forbid: true, Exclusive: true, AnyOf: []string{"r"}}); err == nil {
		t.Error("want an error when Forbid, Exclusive, and any_of_refs are all combined")
	}
}

// TestValidateResolvedTests_RunsMediaValidationOverEveryTest proves the render-time gate
// scans the WHOLE resolved list, not just the first test — and that a single valid test
// among conflicted ones doesn't mask the failure.
func TestValidateResolvedTests_RunsMediaValidationOverEveryTest(t *testing.T) {
	valid := []TestCase{{ID: "ok", Media: &MediaExpect{Forbid: true}}}
	if err := validateResolvedTests("s1", valid); err != nil {
		t.Errorf("want no error for a valid test list, got %v", err)
	}

	conflicted := []TestCase{
		{ID: "ok"},
		{ID: "bad", Media: &MediaExpect{Forbid: true, AnyOf: []string{"r"}}},
	}
	if err := validateResolvedTests("s1", conflicted); err == nil {
		t.Fatal("want an error when ANY test in the list conflicts, not just the first")
	} else if !strings.Contains(err.Error(), "bad") {
		t.Errorf("want the error to name the conflicting test id, got %v", err)
	}
}

// TestFilterProviders_SameIDDifferentLabelBothSelected is the regression test for a
// real bug a review caught: byID used to be keyed by bare id only, so naming an id
// shared by two Label-disambiguated entries (e.g. models-reasoning.yaml's reasoning-on/
// off pair) silently returned only the LAST-registered one instead of both — exactly
// the "collapse into one bucket" failure providerModelKey (judge.go) exists to prevent
// everywhere else, just not applied here.
func TestFilterProviders_SameIDDifferentLabelBothSelected(t *testing.T) {
	mf := &ModelsFile{Providers: []ModelProvider{
		{ID: "openrouter:google/gemini-2.5-flash", Label: "reasoning-off", Temperature: 0.3, MaxTokens: 500},
		{ID: "openrouter:google/gemini-2.5-flash", Label: "reasoning-on", Temperature: 0.3, MaxTokens: 500},
		{ID: "openrouter:openai/gpt-4o-mini", Temperature: 0.3, MaxTokens: 500},
	}}

	got, err := filterProviders(mf, "google/gemini-2.5-flash")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want both labeled entries selected, got %d: %+v", len(got), got)
	}
	labels := map[string]bool{got[0].Label: true, got[1].Label: true}
	if !labels["reasoning-off"] || !labels["reasoning-on"] {
		t.Errorf("want both reasoning-off and reasoning-on present, got labels %+v", labels)
	}

	// Unfiltered (no -models) must still return every entry, unaffected.
	all, err := filterProviders(mf, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("want all 3 entries with no filter, got %d", len(all))
	}
}

// TestFilterProviders_DefaultFlag covers ModelProvider.Default: an empty -models filter
// should narrow to the default-marked subset when any provider sets it, "all" should
// always bypass it, and a models.yaml with no Default set anywhere (e.g.
// models-reasoning.yaml, which predates this field) must keep returning every provider.
func TestFilterProviders_DefaultFlag(t *testing.T) {
	mf := &ModelsFile{Providers: []ModelProvider{
		{ID: "openrouter:openai/gpt-4o-mini", Temperature: 0.3, MaxTokens: 500, Default: true},
		{ID: "openrouter:google/gemini-2.5-flash-lite", Temperature: 0.3, MaxTokens: 500, Default: true},
		{ID: "openrouter:google/gemini-3.5-flash", Temperature: 0.3, MaxTokens: 1500},
	}}

	defaults, err := filterProviders(mf, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(defaults) != 2 {
		t.Fatalf("want only the 2 default:true providers with no filter, got %d: %+v", len(defaults), defaults)
	}
	for _, p := range defaults {
		if !p.Default {
			t.Errorf("got a non-default provider in the unfiltered result: %+v", p)
		}
	}

	all, err := filterProviders(mf, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("want every provider with -models all, got %d", len(all))
	}

	named, err := filterProviders(mf, "google/gemini-3.5-flash")
	if err != nil {
		t.Fatal(err)
	}
	if len(named) != 1 || named[0].ID != "openrouter:google/gemini-3.5-flash" {
		t.Fatalf("want the explicitly named non-default provider, got %+v", named)
	}

	noDefaults := &ModelsFile{Providers: []ModelProvider{
		{ID: "openrouter:google/gemini-2.5-flash", Label: "reasoning-off", Temperature: 0.3, MaxTokens: 500},
		{ID: "openrouter:google/gemini-2.5-flash", Label: "reasoning-on", Temperature: 0.3, MaxTokens: 500},
	}}
	got, err := filterProviders(noDefaults, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want fallback to every provider when none set Default, got %d", len(got))
	}
}

// TestFilterProviders_ExcludesArchivedModels is the archival counterpart to
// TestFilterProviders_DefaultFlag: an archived model must vanish from the default
// selection and from "-models all", but naming it EXPLICITLY must error loudly rather
// than silently drop it — a deliberate request against a retired/unverified model is a
// mistake worth surfacing before any spend, not a request to quietly honor.
func TestFilterProviders_ExcludesArchivedModels(t *testing.T) {
	mf := &ModelsFile{
		Providers: []ModelProvider{
			{ID: "openrouter:openai/gpt-4o-mini", Default: true},
			{ID: "openrouter:google/gemini-3.5-flash", Default: true},
			{ID: "openrouter:deepseek/deepseek-v3.2-exp"},
		},
		ArchivedModels: []ArchivedModel{
			{ID: "openrouter:google/gemini-3.5-flash", Reason: "not yet probe-verified"},
		},
	}

	defaults, err := filterProviders(mf, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range defaults {
		if orModelID(p.ID) == "google/gemini-3.5-flash" {
			t.Fatalf("archived model must be excluded from the default selection, got %+v", defaults)
		}
	}
	if len(defaults) != 1 {
		t.Fatalf("want only the one non-archived default provider, got %d: %+v", len(defaults), defaults)
	}

	all, err := filterProviders(mf, "all")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range all {
		if orModelID(p.ID) == "google/gemini-3.5-flash" {
			t.Fatalf("archived model must be excluded even from -models all, got %+v", all)
		}
	}
	if len(all) != 2 {
		t.Fatalf("want every non-archived provider under -models all, got %d: %+v", len(all), all)
	}

	if _, err := filterProviders(mf, "google/gemini-3.5-flash"); err == nil {
		t.Fatal("want an error when an archived model is named EXPLICITLY via -models, got nil")
	} else if !strings.Contains(err.Error(), "archived") {
		t.Errorf("want the error to mention archival, got: %v", err)
	}

	// A non-archived model named explicitly is unaffected.
	named, err := filterProviders(mf, "deepseek/deepseek-v3.2-exp")
	if err != nil {
		t.Fatal(err)
	}
	if len(named) != 1 {
		t.Fatalf("want the explicitly named non-archived provider, got %+v", named)
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
	scenario := &ScenarioConfig{Name: "fixture"}
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
// wire shape: passthrough is a real, recognized config key promptfoo's OpenRouter
// provider forwards upstream.
func TestWritePromptfooConfig_PassthroughAndLabelPresentWhenSet(t *testing.T) {
	dir := t.TempDir()
	scenario := &ScenarioConfig{Name: "fixture"}
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
