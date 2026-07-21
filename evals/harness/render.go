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
	modelsFilter := fs.String("models", "", "comma-separated model ids to render for (default: providers marked default:true in models.yaml, or every provider if none are marked; pass \"all\" for every provider explicitly)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scenarioDir == "" {
		return fmt.Errorf("render: -scenario is required")
	}
	return renderScenario(*scenarioDir, *modelsPath, *modelsFilter)
}

// filterProviders returns the providers from mf whose (de-prefixed) ID is named in the
// comma-separated modelsFilter, in modelsFilter's order. Unlike extraction's
// resolveVisionModels, an unknown ID is an error, not a one-off fallback — a chat run's
// providers must already be configured (temperature/max_tokens/pricing) in models.yaml,
// not improvised at the command line.
//
// An empty modelsFilter no longer always means "every provider" (see ModelProvider.Default
// for why): it returns only the providers with Default true, if any exist in mf. If none do
// — e.g. models-reasoning.yaml, which predates the Default field — it falls back to every
// provider, so files that don't opt into this feature keep their old behavior unchanged.
// `-models all` is a reserved keyword that always returns every provider regardless of
// Default, for the times you deliberately want the full/expanded set.
//
// byID maps one id to ALL provider entries sharing it, not just one — two entries CAN
// legitimately share an id with different Label values (e.g. models-reasoning.yaml's
// reasoning-on/reasoning-off pair), and naming that id in modelsFilter must select BOTH,
// not silently keep only the last-registered one. Confirmed by a real bug this fixes: an
// earlier single-value byID map made `-models <id>` against a labeled-pair models file
// silently return 1 provider instead of 2 — exactly the "collapse into one bucket"
// failure providerModelKey (judge.go) exists to prevent everywhere else.
func filterProviders(mf *ModelsFile, modelsFilter string) ([]ModelProvider, error) {
	if modelsFilter == "all" {
		return mf.Providers, nil
	}
	if modelsFilter == "" {
		var defaults []ModelProvider
		for _, p := range mf.Providers {
			if p.Default {
				defaults = append(defaults, p)
			}
		}
		if len(defaults) > 0 {
			return defaults, nil
		}
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
	if scenario.Pipeline == "schema_kb_v1" {
		return renderSchemaKBScenario(scenarioDir, scenario, modelsPath, modelsFilter)
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

	catalog := buildCatalog(data)
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
		scenario.Name, len(catalog.Tokens), len(catalog.MediaTokens), len(tests))
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
//
// MediaTokens unions BOTH media shapes a data.yaml can use: the old per-file Assets list
// (each ref is its own token) and grouped FactRow/Topic Media (each group is one token,
// "{pluralOwnerType}.{ref}.{field}"). A real data.yaml only ever populates one of the two
// in practice — this union exists so buildCatalog never has to be told in advance which
// shape a given scenario uses; it simply reflects whatever the data actually contains.
func buildCatalog(data *Data) *Catalog {
	cat := &Catalog{}
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
			for _, m := range row.Media {
				if len(m.Files) == 0 {
					continue
				}
				cat.MediaTokens = append(cat.MediaTokens, mediaGroupToken(ft.Table, row.Ref, m.Field))
			}
		}
	}
	for _, t := range data.Topics {
		for _, m := range t.Media {
			if len(m.Files) == 0 {
				continue
			}
			cat.MediaTokens = append(cat.MediaTokens, mediaGroupToken("topic", t.Ref, m.Field))
		}
	}
	for _, a := range data.Assets {
		cat.MediaTokens = append(cat.MediaTokens, a.Ref)
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
	sort.Strings(cat.MediaTokens)
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

// validateTestMedia rejects a test whose media expectation is self-contradictory:
//   - Forbid (no attachment allowed) combined with AnyOf/AllOf (attach these) can never
//     both be satisfied by the same reply.
//   - Forbid combined with Exclusive: Forbid already means "nothing may be attached",
//     leaving Exclusive nothing to scope.
//   - Exclusive without a non-empty AnyOf/AllOf: Exclusive is a MODIFIER on an any_of/
//     all_of declaration ("this set and nothing else"), not a standalone expectation — it
//     has no set to narrow without one.
//
// Shared by both resolution paths — render's resolveTests/renderScenario and the catalog
// export's resolveCatalogTests — so a conflicted test fails the free render gate AND
// `export -all`, not just whichever a scenario author happens to run first.
func validateTestMedia(scenarioName, testID string, m *MediaExpect) error {
	if m == nil {
		return nil
	}
	hasExpectation := len(m.AnyOf) > 0 || len(m.AllOf) > 0
	if m.Forbid {
		if hasExpectation {
			return fmt.Errorf("scenario %q: test %q: media.forbid is set together with any_of/all_of — a test cannot both require and forbid an attachment", scenarioName, testID)
		}
		if m.Exclusive {
			return fmt.Errorf("scenario %q: test %q: media.forbid is set together with media.exclusive — forbid already means no attachment is allowed at all, exclusive has nothing left to scope", scenarioName, testID)
		}
	}
	if m.Exclusive && !hasExpectation {
		return fmt.Errorf("scenario %q: test %q: media.exclusive is set without any_of/all_of — exclusive narrows an existing any_of/all_of declaration, it cannot stand alone", scenarioName, testID)
	}
	return nil
}

// validateTestOutcomes rejects a malformed alternative-outcomes declaration
// (TestCase.Outcomes) at authoring time, so the judge never has to interpret one:
//   - exactly one block is meaningless — a single alternative is just a plain test; its
//     checks belong at the TestCase top level, where they read as what they are.
//   - a block with no Label can't be named in failure reasons or the UI ("which
//     alternative failed?" must always be answerable).
//   - a block declaring zero checks is vacuously true, which would silently make the
//     whole OR pass for every reply — an authoring bug, never an intent.
//   - Language outside ""/"kk"/"ru" would silently skip the language check (judgeOne
//     gates on those two values), reading as an expectation while checking nothing.
//   - each block's Media must satisfy the same self-consistency rules as a top-level
//     media expectation (validateTestMedia).
//
// Shared by both resolution paths, same as validateTestMedia — see its comment.
func validateTestOutcomes(scenarioName, testID string, ocs []OutcomeCase) error {
	if len(ocs) == 0 {
		return nil
	}
	if len(ocs) == 1 {
		return fmt.Errorf("scenario %q: test %q: outcomes has a single block — one alternative is just a plain test; declare its checks at the test's top level instead", scenarioName, testID)
	}
	for i, oc := range ocs {
		if strings.TrimSpace(oc.Label) == "" {
			return fmt.Errorf("scenario %q: test %q: outcomes[%d] has no label — every alternative must be nameable in failure reasons", scenarioName, testID, i)
		}
		declares := len(oc.Requires) > 0 || oc.Media != nil || oc.Escalate != nil ||
			oc.Language != "" || len(oc.MustNotContain) > 0 || len(oc.MustContainAny) > 0
		if !declares {
			return fmt.Errorf("scenario %q: test %q: outcomes[%d] (%q) declares no checks — a check-less block passes every reply, which can only be an authoring mistake", scenarioName, testID, i, oc.Label)
		}
		if oc.Language != "" && oc.Language != "kk" && oc.Language != "ru" {
			return fmt.Errorf("scenario %q: test %q: outcomes[%d] (%q) has language %q — only \"kk\" or \"ru\" are checked", scenarioName, testID, i, oc.Label, oc.Language)
		}
		if err := validateTestMedia(scenarioName, testID, oc.Media); err != nil {
			return err
		}
	}
	return nil
}

// validateResolvedTests runs validateTestMedia + validateTestOutcomes (and any future
// cross-test invariant) over an already-resolved test list.
func validateResolvedTests(scenarioName string, tests []TestCase) error {
	for _, tc := range tests {
		if err := validateTestMedia(scenarioName, tc.ID, tc.Media); err != nil {
			return err
		}
		if err := validateTestOutcomes(scenarioName, tc.ID, tc.Outcomes); err != nil {
			return err
		}
	}
	return nil
}

func factToken(table, ref, field string) string {
	return "{{" + table + "." + ref + "." + field + "}}"
}

// mediaOwnerTypePlural maps a fact/media owner's SINGULAR type name (FactTable.Table, or
// the hardcoded "topic") to the PLURAL domain segment backend/aiprompt's own semantic
// media tokens use (topics/products/tariffs/contacts/policies — see DECISIONS.md
// §"Concrete media-column naming"). Fact tokens stay singular ({{product.ref.field}});
// only media-group tokens pluralize, so a legacy scenario's media_files_to_send values
// read in the same shape as a schema_kb_v1 scenario's, even though the two pipelines
// build their catalogs completely differently.
var mediaOwnerTypePlural = map[string]string{
	"product": "products", "tariff": "tariffs", "topic": "topics",
	"contact": "contacts", "policy": "policies",
}

// mediaGroupToken names a media group as "{pluralOwnerType}.{ref}.{field}" — matching
// the FACTS token shape ({{table.ref.field}}) instead of dropping the owner type, except
// pluralized (see mediaOwnerTypePlural). A review caught that "coffee-machine.images" (no
// type prefix) collides as soon as two owner kinds could share a ref (e.g. a topic and a
// product both called "delivery"); the fix is to name it the same way a fact is named,
// everywhere.
func mediaGroupToken(ownerType, ref, field string) string {
	plural, ok := mediaOwnerTypePlural[ownerType]
	if !ok {
		plural = ownerType // defensive fallback; every real owner type is in the map above
	}
	return plural + "." + ref + "." + field
}

// buildPrompt fills frame.txt's %%SLOTS%% from data.yaml, then wraps the result via
// wrapPromptfooTemplate. There is no more %%MEDIA_FIELD%% slot: every scenario's response
// field is named media_files_to_send, so a frame.txt states that literally instead of
// asking render to fill in which name applies.
func buildPrompt(frame string, scenario *ScenarioConfig, data *Data, cat *Catalog) string {
	filled := frame
	filled = strings.ReplaceAll(filled, "%%KNOWLEDGE_BASE%%", renderKnowledgeBase(data.Topics, scenario.TopicFormat))
	filled = strings.ReplaceAll(filled, "%%MEDIA%%", renderMedia(data))
	filled = strings.ReplaceAll(filled, "%%FACTS%%", renderFacts(data.FactTables))
	filled = strings.ReplaceAll(filled, "%%DESCRIPTIONS%%", renderDescriptions(data.FactTables))
	return wrapPromptfooTemplate(filled)
}

// wrapPromptfooTemplate wraps an already-fully-rendered stable prefix in
// {% raw %}/{% endraw %} (so promptfoo's own Nunjucks-style templating never
// interprets a literal {{token}} span meant for OUR later substitution step,
// whether that span is a legacy {{table.ref.field}} fact placeholder or an
// aiprompt one) and appends the {{history}}/{{message}} tail every scenario's
// prompt needs. The ONE place this wrapping happens, shared by every
// pipeline, so no scenario author or pipeline implementation needs to
// remember the promptfoo-templating-safety trick by hand.
func wrapPromptfooTemplate(filled string) string {
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
//
// Unions both media shapes a data.yaml can use, same as buildCatalog — see its doc
// comment for why this never actually mixes shapes within one real scenario.
func renderMedia(data *Data) string {
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
	for _, a := range data.Assets {
		lines = append(lines, fmt.Sprintf("%s | %s | %s | %s", a.Ref, a.Kind, a.Topic, a.Description))
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
