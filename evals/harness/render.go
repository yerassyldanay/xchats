package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"xchats-evals-harness/internal/provenance"
)

func cmdRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	scenarioDir := fs.String("scenario", "", "path to the scenario directory, e.g. scenarios/shop-current")
	modelsPath := fs.String("models-file", "models.yaml", "path to models.yaml")
	modelsFilter := fs.String("models", "", "comma-separated model ids to render for (default: every provider in models.yaml)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scenarioDir == "" {
		return fmt.Errorf("render: -scenario is required")
	}
	return renderScenario(*scenarioDir, *modelsPath, *modelsFilter)
}

// filterProviders returns the providers from mf whose (de-prefixed) ID is named in the
// comma-separated modelsFilter, in modelsFilter's order; an empty modelsFilter returns every
// provider unchanged (today's default). Unlike extraction's resolveVisionModels, an unknown ID
// is an error, not a one-off fallback — a chat run's providers must already be configured
// (temperature/max_tokens/pricing) in models.yaml, not improvised at the command line.
//
// byID maps one id to ALL provider entries sharing it, not just one — two entries CAN
// legitimately share an id with different Label values (e.g. models-reasoning.yaml's
// reasoning-on/reasoning-off pair), and naming that id in modelsFilter must select BOTH,
// not silently keep only the last-registered one. Confirmed by a real bug this fixes: an
// earlier single-value byID map made `-models <id>` against a labeled-pair models file
// silently return 1 provider instead of 2 — exactly the "collapse into one bucket"
// failure providerModelKey (judge.go) exists to prevent everywhere else.
func filterProviders(mf *ModelsFile, modelsFilter string) ([]ModelProvider, error) {
	if modelsFilter == "" {
		return mf.Providers, nil
	}
	byID := map[string][]ModelProvider{}
	for _, p := range mf.Providers {
		key := orModelID(p.ID)
		byID[key] = append(byID[key], p)
	}
	out := make([]ModelProvider, 0, len(mf.Providers))
	for _, id := range splitCSV(modelsFilter) {
		ps, ok := byID[orModelID(id)]
		if !ok {
			return nil, fmt.Errorf("model %q not found in models.yaml", id)
		}
		out = append(out, ps...)
	}
	return out, nil
}

func renderScenario(scenarioDir, modelsPath, modelsFilter string) error {
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
	if err := applyLimits(data, scenario.Limits); err != nil {
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

	models, err := loadModels(modelsPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", modelsPath, err)
	}
	filtered, err := filterProviders(models, modelsFilter)
	if err != nil {
		return fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}
	models = &ModelsFile{PricingSource: models.PricingSource, PricingCheckedAt: models.PricingCheckedAt, Providers: filtered}

	catalog := buildCatalog(data, scenario.Contract)
	if err := validateCatalog(catalog); err != nil {
		return fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}
	prompt := buildPrompt(string(frameBytes), scenario, data, catalog)
	if err := validatePrompt(prompt, catalog); err != nil {
		return fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}

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

// applyLimits truncates a named fact_tables table's Rows in place, BEFORE buildCatalog
// or buildPrompt ever see the data — so the prompt and the grading catalog are always
// built from the exact same (already-truncated) rows and can never disagree about which
// products exist at a given scale. Only the table named by the limit key is touched;
// unrelated tables (policy, contact, ...) are untouched even if they have multiple rows.
func applyLimits(data *Data, limits map[string]int) error {
	for table, n := range limits {
		found := false
		for i := range data.FactTables {
			if data.FactTables[i].Table != table {
				continue
			}
			found = true
			if n < len(data.FactTables[i].Rows) {
				data.FactTables[i].Rows = data.FactTables[i].Rows[:n]
			}
		}
		if !found {
			return fmt.Errorf("limits references table %q, not found in fact_tables", table)
		}
	}
	return nil
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
	trustedDigits := map[string]bool{}
	for _, ft := range data.FactTables {
		for _, row := range ft.Rows {
			for _, d := range digitRunRE.FindAllString(row.Description, -1) {
				trustedDigits[d] = true
			}
		}
	}
	for d := range trustedDigits {
		cat.TrustedDigits = append(cat.TrustedDigits, d)
	}
	sort.Slice(cat.Tokens, func(i, j int) bool { return cat.Tokens[i].Token < cat.Tokens[j].Token })
	sort.Strings(cat.MediaRefs)
	sort.Strings(cat.MediaGroups)
	sort.Strings(cat.TrustedDigits)
	return cat
}

// validateCatalog enforces that no fact value itself contains a brace character. This is
// the invariant judge.go's "any brace surviving injection is a mangled placeholder" check
// depends on — if a real fact value could legitimately contain '{' or '}', that check would
// have false positives. Nothing in these scenarios' data ever needs a literal brace, so this
// is a hard render-time failure, not a warning.
func validateCatalog(cat *Catalog) error {
	for _, t := range cat.Tokens {
		if strings.ContainsAny(t.Value, "{}") {
			return fmt.Errorf("catalog value for %s contains a brace character: %q", t.Token, t.Value)
		}
	}
	return nil
}

var unfilledSlotRE = regexp.MustCompile(`%%[A-Z_]+%%`)

// validatePrompt makes the "prompt and catalog can never disagree" claim in buildCatalog's
// doc comment an enforced guarantee instead of a design convention: every {{token}} span
// anywhere in the rendered prompt (including a frame.txt author's own inline example, like
// the one in shop-decisions-v1/frame.txt) must resolve against this same render's catalog,
// or render fails loudly instead of shipping a prompt that quietly instructs the model to
// use a token nothing will ever inject a value for. Also fails on a leftover %%SLOT%% —
// evidence a frame.txt slot name and render.go's ReplaceAll calls have drifted apart.
// promptfooVarTokens are the ONLY "{{...}}" spans in a rendered prompt that are not fact
// tokens — promptfoo's own templating vars, filled per-test at eval time, not by render.
var promptfooVarTokens = map[string]bool{
	"{{message}}": true,
	"{{history}}": true,
}

func validatePrompt(prompt string, cat *Catalog) error {
	if m := unfilledSlotRE.FindString(prompt); m != "" {
		return fmt.Errorf("unfilled slot %s left in rendered prompt", m)
	}
	valid := map[string]bool{}
	for _, t := range cat.Tokens {
		valid[t.Token] = true
	}
	for _, span := range tokenSpanRE.FindAllString(prompt, -1) {
		if valid[span] || promptfooVarTokens[span] {
			continue
		}
		return fmt.Errorf("prompt contains token %s not in this scenario's catalog (check frame.txt example tokens against data.yaml)", span)
	}
	return nil
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
	b.WriteString("\n{% endraw %}\n")
	b.WriteString("История переписки (может быть пустой — тогда это начало разговора):\n{{history}}\n")
	b.WriteString("Клиент пишет: {{message}}\n")
	return b.String()
}

// renderHistory turns a test's prior turns into the plain-text block {{history}} fills —
// authored prose, never a {{token}} (history is what was ALREADY sent, i.e. already
// injected), so it can never trip validatePrompt or judge.go's token checks.
func renderHistory(turns []HistoryTurn) string {
	if len(turns) == 0 {
		return "(пусто — начало разговора)"
	}
	var lines []string
	for _, t := range turns {
		label := "Клиент"
		if t.Role == "assistant" {
			label = "Ассистент"
		}
		lines = append(lines, label+": "+t.Text)
	}
	return strings.Join(lines, "\n")
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
				line := fmt.Sprintf("%s | %s | %s", factToken(ft.Table, row.Ref, f.Name), label, v)
				if usage := renderFieldUsage(f); usage != "" {
					line += " | " + usage
				}
				lines = append(lines, line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func renderFieldUsage(f FieldSpec) string {
	switch f.ValueKind {
	case "money_display":
		return "value already includes currency; do not add ₸/тенге"
	case "time_display":
		return "value already includes the complete time; do not add extra time words"
	case "text_complete":
		return "value already includes its own unit word; do not add another (e.g. never turn \"1–3 дня\" into \"1–3 дня дней\")"
	case "percent_number":
		return "add % after the placeholder"
	case "number", "number_range":
		if f.UnitRU == "" && f.UnitKK == "" {
			return "number only"
		}
		return fmt.Sprintf("number only; add unit in reply language (ru: %s; kk: %s)", f.UnitRU, f.UnitKK)
	}
	return ""
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

// buildPassthrough turns a ModelProvider's optional Provider/Reasoning config into
// promptfoo's own "passthrough" config key — confirmed (against a real committed
// promptfoo results.json) as the mechanism promptfoo's OpenRouter provider uses to
// forward arbitrary extra fields straight into the upstream HTTP request body. Returns
// nil (not an empty map) when neither is set, so writePromptfooConfig omits the key
// entirely — today's 4 models.yaml entries, which set neither, must generate a
// byte-identical promptfooconfig.yaml to before this function existed.
func buildPassthrough(p ModelProvider) map[string]any {
	passthrough := map[string]any{}
	if p.Provider != nil {
		route := map[string]any{}
		if len(p.Provider.Order) > 0 {
			route["order"] = p.Provider.Order
		}
		if p.Provider.AllowFallbacks != nil {
			route["allow_fallbacks"] = *p.Provider.AllowFallbacks
		}
		if len(route) > 0 {
			passthrough["provider"] = route
		}
	}
	if p.Reasoning != nil {
		reasoning := map[string]any{"enabled": p.Reasoning.Enabled}
		if p.Reasoning.Effort != "" {
			reasoning["effort"] = p.Reasoning.Effort
		}
		if p.Reasoning.MaxTokens > 0 {
			reasoning["max_tokens"] = p.Reasoning.MaxTokens
		}
		if p.Reasoning.Exclude {
			reasoning["exclude"] = p.Reasoning.Exclude
		}
		passthrough["reasoning"] = reasoning
	}
	if len(passthrough) == 0 {
		return nil
	}
	return passthrough
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
		Label  string         `yaml:"label,omitempty"`
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
		pConfig := map[string]any{"temperature": p.Temperature, "max_tokens": p.MaxTokens}
		if pt := buildPassthrough(p); pt != nil {
			pConfig["passthrough"] = pt
		}
		cfg.Providers = append(cfg.Providers, providerEntry{
			ID:     p.ID,
			Label:  p.Label,
			Config: pConfig,
		})
	}
	for _, t := range tests {
		cfg.Tests = append(cfg.Tests, testEntry{
			Description: t.ID,
			Vars:        map[string]string{"message": t.Message, "history": renderHistory(t.History)},
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
	return provenance.AtomicWriteFile(path, b, 0o644)
}
