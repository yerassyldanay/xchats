package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"xchats-evals-harness/internal/kbfixture"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// renderSchemaKBScenario is the pipeline: schema_kb_v1 render path — the
// shop-kb-v1 family's counterpart to renderScenario. It loads a
// database-schema-shaped Russian fixture (internal/kbfixture) instead of the
// generic fact_tables YAML, and builds the prompt through the SAME
// backend/aiprompt package production will use, instead of this harness's
// own buildCatalog/buildPrompt.
//
// Sequence: load fixture -> apply limits -> aiprompt.BuildCatalog (validates
// kbd_materials fail-closed, using the private KB) -> aiprompt.RenderPrompt
// (material-free PromptInput + the resulting public Catalog) -> leak gates ->
// write generated files. Any invalid non-empty media reference anywhere in
// the fixture fails this whole call before prompt.txt is ever written —
// there is no partial render.
func renderSchemaKBScenario(scenarioDir string, scenario *ScenarioConfig, modelsPath, modelsFilter string) error {
	fixturePath := filepath.Join(scenarioDir, scenario.Data)
	kb, err := kbfixture.Load(fixturePath)
	if err != nil {
		return fmt.Errorf("scenario %q: load fixture %s: %w", scenario.Name, fixturePath, err)
	}
	if kb, err = kbfixture.ApplyLimits(kb, scenario.Limits); err != nil {
		return fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}

	framePath := filepath.Join(scenarioDir, scenario.Frame)
	frameBytes, err := os.ReadFile(framePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", framePath, err)
	}

	tests, err := resolveTests(scenarioDir, scenario.Tests)
	if err != nil {
		return fmt.Errorf("resolve tests: %w", err)
	}
	if err := validateResolvedTests(scenario.Name, tests); err != nil {
		return err
	}

	models, err := loadModels(modelsPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", modelsPath, err)
	}
	filtered, err := filterProviders(models, modelsFilter)
	if err != nil {
		return fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}
	models = &ModelsFile{PricingSource: models.PricingSource, PricingCheckedAt: models.PricingCheckedAt, Providers: filtered}

	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		return fmt.Errorf("scenario %q: build catalog: %w", scenario.Name, err)
	}
	rendered, err := aiprompt.RenderPrompt(string(frameBytes), kb.PromptInput(), cat)
	if err != nil {
		return fmt.Errorf("scenario %q: render prompt: %w", scenario.Name, err)
	}
	if err := aiprompt.ValidateNoMaterialLeak(rendered, kb.Materials); err != nil {
		return fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}
	if err := validateNoStorageLocatorLeak(rendered); err != nil {
		return fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}
	prompt := wrapPromptfooTemplate(rendered)

	genDir := filepath.Join(scenarioDir, "generated")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(genDir, "prompt.txt"), []byte(prompt), 0o644); err != nil {
		return err
	}
	// cat is the PUBLIC catalog projection only (aiprompt.Catalog's Facts/Media/Absent —
	// see aiprompt/catalog.go's doc comments): no material IDs, filenames, MIME types, or
	// any other kbd_materials metadata is a field of these types, so this export can never
	// serialize what the prompt itself is forbidden from containing.
	if err := writeJSON(filepath.Join(genDir, "catalog.json"), cat); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(genDir, "resolved_tests.json"), ResolvedTests{Tests: tests}); err != nil {
		return err
	}
	if err := writePromptfooConfig(genDir, scenario, tests, models); err != nil {
		return err
	}

	fmt.Printf("rendered %s: %d fact tokens, %d media entries, %d tests\n",
		scenario.Name, len(cat.Facts), len(cat.Media), len(tests))
	return nil
}

// storageLocatorPattern is a harness-level defense-in-depth gate on top of
// aiprompt.ValidatePrompt's own UUID/file-extension regexes (aiprompt/prompt.go)
// and ValidateNoMaterialLeak's per-material substring check: any
// "scheme://..." shaped span (storage keys like "fake://bucket/…", or a
// literal http(s) URL) has no legitimate reason to appear in a shop-kb-v1
// prompt at all, so its mere shape is a hard failure regardless of whether it
// happens to match a real fixture material.
var storageLocatorPattern = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://\S+`)

func validateNoStorageLocatorLeak(prompt string) error {
	if m := storageLocatorPattern.FindString(prompt); m != "" {
		return fmt.Errorf("prompt leaks a storage-key/URL-shaped string: %s", m)
	}
	return nil
}
