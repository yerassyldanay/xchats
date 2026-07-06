package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func cmdRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	scenarioDir := fs.String("scenario", "", "path to the scenario directory, e.g. scenarios/shop-current")
	modelsPath := fs.String("models", "models.yaml", "path to models.yaml")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scenarioDir == "" {
		return fmt.Errorf("render: -scenario is required")
	}
	return renderScenario(*scenarioDir, *modelsPath)
}

func renderScenario(scenarioDir, modelsPath string) error {
	scenario, err := loadScenario(scenarioDir)
	if err != nil {
		return fmt.Errorf("load scenario.yaml: %w", err)
	}
	if scenario.Contract != "asset_refs" && scenario.Contract != "attach_groups" {
		return fmt.Errorf("scenario %q: contract must be \"asset_refs\" or \"attach_groups\", got %q", scenario.Name, scenario.Contract)
	}

	dataPath := filepath.Join(scenarioDir, scenario.Data)
	data, err := loadData(dataPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", dataPath, err)
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

	models, err := loadModels(modelsPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", modelsPath, err)
	}

	catalog := buildCatalog(data, scenario.Contract)
	prompt := buildPrompt(string(frameBytes), scenario, data, catalog)

	genDir := filepath.Join(scenarioDir, "generated")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(genDir, "prompt.txt"), []byte(prompt), 0o644); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(genDir, "catalog.json"), catalog); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(genDir, "resolved_tests.json"), ResolvedTests{Tests: tests}); err != nil {
		return err
	}
	if err := writePromptfooConfig(genDir, scenario, tests, models); err != nil {
		return err
	}

	fmt.Printf("rendered %s: %d fact tokens, %d media entries, %d tests\n",
		scenario.Name, len(catalog.Tokens), len(catalog.MediaRefs)+len(catalog.MediaGroups), len(tests))
	return nil
}

func loadScenario(dir string) (*ScenarioConfig, error) {
	b, err := os.ReadFile(filepath.Join(dir, "scenario.yaml"))
	if err != nil {
		return nil, err
	}
	var s ScenarioConfig
	if err := yaml.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func loadData(path string) (*Data, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d Data
	if err := yaml.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func loadModels(path string) (*ModelsFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m ModelsFile
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// resolveTests loads tests.yaml (relative to scenarioDir) and follows its Include list
// (paths relative to CWD, i.e. the evals/ root) into one ordered test list: every included
// bank's tests, in the order listed, followed by the scenario's own tests.
func resolveTests(scenarioDir, testsRel string) ([]TestCase, error) {
	b, err := os.ReadFile(filepath.Join(scenarioDir, testsRel))
	if err != nil {
		return nil, err
	}
	var tf TestsFile
	if err := yaml.Unmarshal(b, &tf); err != nil {
		return nil, err
	}
	var out []TestCase
	for _, incPath := range tf.Include {
		ib, err := os.ReadFile(incPath)
		if err != nil {
			return nil, fmt.Errorf("include %s: %w", incPath, err)
		}
		var bank struct {
			Tests []TestCase `yaml:"tests"`
		}
		if err := yaml.Unmarshal(ib, &bank); err != nil {
			return nil, fmt.Errorf("include %s: %w", incPath, err)
		}
		out = append(out, bank.Tests...)
	}
	out = append(out, tf.Tests...)
	return out, nil
}

// buildCatalog is the ONLY place a token or media entry is considered "valid" — render
// derives the prompt from the same rows, so the prompt and the catalog can never disagree.
func buildCatalog(data *Data, contract string) *Catalog {
	cat := &Catalog{Contract: contract}
	for _, ft := range data.FactTables {
		for _, row := range ft.Rows {
			for _, f := range ft.Fields {
				v, ok := row.Values[f.Name]
				if !ok || strings.TrimSpace(v) == "" {
					continue
				}
				token := factToken(ft.Table, row.Ref, f.Name)
				cat.Tokens = append(cat.Tokens, CatalogFact{Token: token, Value: v})
			}
			if contract == "attach_groups" {
				for _, m := range row.Media {
					if len(m.Files) == 0 {
						continue
					}
					cat.MediaGroups = append(cat.MediaGroups, mediaGroupToken(ft.Table, row.Ref, m.Field))
				}
			}
		}
	}
	if contract == "attach_groups" {
		for _, t := range data.Topics {
			for _, m := range t.Media {
				if len(m.Files) == 0 {
					continue
				}
				cat.MediaGroups = append(cat.MediaGroups, mediaGroupToken("topic", t.Ref, m.Field))
			}
		}
	}
	if contract == "asset_refs" {
		for _, a := range data.Assets {
			cat.MediaRefs = append(cat.MediaRefs, a.Ref)
		}
	}
	sort.Slice(cat.Tokens, func(i, j int) bool { return cat.Tokens[i].Token < cat.Tokens[j].Token })
	sort.Strings(cat.MediaRefs)
	sort.Strings(cat.MediaGroups)
	return cat
}

func factToken(table, ref, field string) string {
	return "{{" + table + "." + ref + "." + field + "}}"
}

// mediaGroupToken names a media group as "{ownerType}.{ref}.{field}" — matching the
// FACTS token shape ({{table.ref.field}}) instead of dropping the owner type. A review
// caught that "coffee-machine.images" (no type prefix) collides as soon as two owner
// kinds could share a ref (e.g. a topic and a product both called "delivery"); the fix
// is to name it the same way a fact is named, everywhere.
func mediaGroupToken(ownerType, ref, field string) string {
	return ownerType + "." + ref + "." + field
}

// buildPrompt fills frame.txt's %%SLOTS%% from data.yaml, then wraps the result in
// {% raw %}/{% endraw %} + the {{message}} tail — the ONE place that wrapping happens, so
// no scenario author needs to remember the promptfoo-templating-safety trick by hand.
func buildPrompt(frame string, scenario *ScenarioConfig, data *Data, cat *Catalog) string {
	mediaField := "asset_refs"
	if scenario.Contract == "attach_groups" {
		mediaField = "attach_groups"
	}
	filled := frame
	filled = strings.ReplaceAll(filled, "%%MEDIA_FIELD%%", mediaField)
	filled = strings.ReplaceAll(filled, "%%KNOWLEDGE_BASE%%", renderKnowledgeBase(data.Topics, scenario.TopicFormat))
	filled = strings.ReplaceAll(filled, "%%MEDIA%%", renderMedia(data, scenario.Contract))
	filled = strings.ReplaceAll(filled, "%%FACTS%%", renderFacts(data.FactTables))
	filled = strings.ReplaceAll(filled, "%%DESCRIPTIONS%%", renderDescriptions(data.FactTables))

	var b strings.Builder
	b.WriteString("{% raw %}\n")
	b.WriteString(strings.TrimRight(filled, "\n"))
	b.WriteString("\n{% endraw %}\nКлиент пишет: {{message}}\n")
	return b.String()
}

func renderKnowledgeBase(topics []Topic, format string) string {
	if format == "" {
		format = "# topic: {ref} ({lang})\nkeywords: {keywords}\n{body}"
	}
	var parts []string
	for _, t := range topics {
		lang := t.Lang
		if lang == "" {
			lang = "ru"
		}
		s := format
		s = strings.ReplaceAll(s, "{ref}", t.Ref)
		s = strings.ReplaceAll(s, "{lang}", lang)
		s = strings.ReplaceAll(s, "{title}", t.Title)
		s = strings.ReplaceAll(s, "{keywords}", t.Keywords)
		s = strings.ReplaceAll(s, "{body}", strings.TrimSpace(t.Body))
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n\n")
}

// renderMedia returns only the CONTENT rows — no header line. The header ("MEDIA
// CATALOG:" vs. "MEDIA — группы, отправляются целиком:", in whatever language a scenario
// wants) is written by the scenario author directly in frame.txt, right before
// %%MEDIA%%, same as KNOWLEDGE BASE's header already works. Two scenarios were found to
// use genuinely different header wording for the same contract style, so this can't be
// hardcoded in Go without silently reproducing the wrong one.
func renderMedia(data *Data, contract string) string {
	if contract == "asset_refs" {
		var lines []string
		for _, a := range data.Assets {
			lines = append(lines, fmt.Sprintf("%s | %s | %s | %s", a.Ref, a.Kind, a.Topic, a.Description))
		}
		return strings.Join(lines, "\n")
	}
	var lines []string
	for _, ft := range data.FactTables {
		for _, row := range ft.Rows {
			for _, m := range row.Media {
				if len(m.Files) == 0 {
					continue
				}
				lines = append(lines, fmt.Sprintf("%s — %d %s", mediaGroupToken(ft.Table, row.Ref, m.Field), len(m.Files), m.Description))
			}
		}
	}
	for _, t := range data.Topics {
		for _, m := range t.Media {
			if len(m.Files) == 0 {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s — %s", mediaGroupToken("topic", t.Ref, m.Field), m.Description))
		}
	}
	return strings.Join(lines, "\n")
}

// renderFacts returns only the "{{token}} | label | value" rows — no header. The header
// (which varies in wording and even language between scenarios — see renderMedia's
// comment for why that can't be hardcoded) belongs in frame.txt, right before %%FACTS%%.
func renderFacts(factTables []FactTable) string {
	var lines []string
	for _, ft := range factTables {
		labelFormat := ft.LabelFormat
		if labelFormat == "" {
			labelFormat = "{display_name} — {field_label}"
		}
		for _, row := range ft.Rows {
			for _, f := range ft.Fields {
				v, ok := row.Values[f.Name]
				if !ok || strings.TrimSpace(v) == "" {
					continue
				}
				label := labelFormat
				label = strings.ReplaceAll(label, "{display_name}", row.DisplayName)
				label = strings.ReplaceAll(label, "{field_label}", f.Label)
				lines = append(lines, fmt.Sprintf("%s | %s | %s", factToken(ft.Table, row.Ref, f.Name), label, v))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func renderDescriptions(factTables []FactTable) string {
	var blocks []string
	for _, ft := range factTables {
		if ft.DescriptionsLabel == "" {
			continue
		}
		var lines []string
		for _, row := range ft.Rows {
			if strings.TrimSpace(row.Description) == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s: %s", row.DisplayName, row.Description))
		}
		if len(lines) == 0 {
			continue
		}
		blocks = append(blocks, ft.DescriptionsLabel+"\n"+strings.Join(lines, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

// writePromptfooConfig deliberately gives every test a trivial, always-passing assert —
// ALL real grading (token correctness, injection, fail-closed, media, escalation,
// language) happens in `judge`, in Go, against the raw answers this produces. Nothing
// here writes a hand-crafted assertion into the generated YAML, so promptfoo's own
// templating engine never has anything of ours to mangle (the exact bug class a prior
// version of this eval hit three times).
func writePromptfooConfig(genDir string, scenario *ScenarioConfig, tests []TestCase, models *ModelsFile) error {
	type promptEntry struct {
		ID    string `yaml:"id"`
		Label string `yaml:"label"`
	}
	type providerEntry struct {
		ID     string         `yaml:"id"`
		Config map[string]any `yaml:"config"`
	}
	type assertEntry struct {
		Type  string `yaml:"type"`
		Value string `yaml:"value"`
	}
	type testEntry struct {
		Description string            `yaml:"description"`
		Vars        map[string]string `yaml:"vars"`
		Assert      []assertEntry     `yaml:"assert"`
	}
	cfg := struct {
		Description string          `yaml:"description"`
		Prompts     []promptEntry   `yaml:"prompts"`
		Providers   []providerEntry `yaml:"providers"`
		Tests       []testEntry     `yaml:"tests"`
	}{
		Description: scenario.Description,
		Prompts:     []promptEntry{{ID: "file://prompt.txt", Label: scenario.Name}},
	}
	for _, p := range models.Providers {
		cfg.Providers = append(cfg.Providers, providerEntry{
			ID:     p.ID,
			Config: map[string]any{"temperature": p.Temperature, "max_tokens": p.MaxTokens},
		})
	}
	for _, t := range tests {
		cfg.Tests = append(cfg.Tests, testEntry{
			Description: t.ID,
			Vars:        map[string]string{"message": t.Message},
			// A bare expression, NOT "return true;" — promptfoo treats a short
			// javascript assert value as an expression to wrap (something like
			// "return (<value>)"), so an explicit "return" here becomes invalid
			// nested syntax. Confirmed against a real run: "return true;" failed
			// every single test with "Unexpected token 'return'" even though the
			// real API calls behind it succeeded — this is intentionally the
			// simplest value that works, not a stylistic choice.
			Assert: []assertEntry{{Type: "javascript", Value: "true"}},
		})
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	header := "# GENERATED by harness render — do not hand-edit. Edit scenario.yaml / data.yaml / tests.yaml instead.\n"
	return os.WriteFile(filepath.Join(genDir, "promptfooconfig.yaml"), append([]byte(header), b...), 0o644)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
