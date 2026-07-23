package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"xchats-evals-harness/internal/kbfixture"
	"xchats-evals-harness/internal/provenance"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// CatalogFileSchemaVersion follows export.go's own per-artifact versioning
// (ExecutionsFileSchemaVersion / RunsFileSchemaVersion) — independent of both, since
// the catalog's shape can evolve on its own timeline.
//
// v2 (2026-07): added CatalogScenario.Archived/ArchivedReason (the 2026-07
// schema_kb_v1 consolidation's archival mechanism — see ScenarioConfig.Archived).
const CatalogFileSchemaVersion = 2

// CatalogFile is runs/catalog.json — the ONE deliberately repository-state (not a run
// snapshot) artifact this harness exports: every scenario's and every extraction
// case's REQUIREMENTS. It is an internal human/audit artifact, so fact tokens include
// the values code would inject; those values are never copied into the model-facing
// prompt or generated scenario catalog. It exists to answer, before any model is ever
// run: what the model will receive, what code will inject, what is required, and where
// each requirement comes from. GeneratedAt is its only freshness signal.
type CatalogFile struct {
	SchemaVersion int                  `json:"schema_version"`
	GeneratedAt   string               `json:"generated_at"`
	Scenarios     []CatalogScenario    `json:"scenarios"`
	ExtractCases  []CatalogExtractCase `json:"extract_cases"`
}

// CatalogScenario is one scenario.yaml's fully-resolved audit view: Facts contain the
// private expected token->injected-value mapping, while Media and Tests describe what
// the model-facing scenario exposes and checks.
type CatalogScenario struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Setup       string `json:"setup,omitempty"`
	PromptRef   string `json:"prompt_ref,omitempty"`
	Experiment  string `json:"experiment,omitempty"`
	// Archived/ArchivedReason mirror ScenarioConfig's own fields verbatim — the
	// catalog is a repository-state snapshot (writeCatalogJSON's own doc comment), so
	// this is a direct copy, never a separate re-derivation.
	Archived       bool   `json:"archived,omitempty"`
	ArchivedReason string `json:"archived_reason,omitempty"`
	// Pipeline mirrors ScenarioConfig.Pipeline — empty for the legacy fact_tables path,
	// "schema_kb_v1" for the shop-kb-v1 family (see buildCatalogScenarioSchemaKB).
	Pipeline string `json:"pipeline,omitempty"`
	// FactsSource is the evals/-relative path of the data.yaml (or, for a schema_kb_v1
	// scenario, the fixture file) every Fact below and every test's `requires` token is
	// resolved against (may point into another scenario dir — several scenarios
	// deliberately share one data.yaml).
	FactsSource string        `json:"facts_source"`
	Facts       []CatalogFact `json:"facts"`
	// MediaTokens is every valid media_files_to_send token for this scenario.
	MediaTokens []string          `json:"media_tokens,omitempty"`
	Media       []CatalogMedia    `json:"media,omitempty"`
	Tests       []CatalogTestCase `json:"tests"`
}

// CatalogMedia is one selectable media entry (an old-style individual asset, or a
// grouped-media token) with the description the model sees as its ONLY selection cue.
type CatalogMedia struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"` // asset Kind (old-style), or "group" (grouped media)
	Description string `json:"description"`
}

// CatalogMediaExpect mirrors MediaExpect with explicit JSON tags — a DEDICATED type,
// never MediaExpect itself: MediaExpect has no json tags today (it inherits
// resolved_tests.json's untagged, capitalized-key shape via TestCase), and existing run
// snapshots depend on that exact shape. Adding tags here can never affect it.
type CatalogMediaExpect struct {
	AnyOf     []string `json:"any_of,omitempty"`
	AllOf     []string `json:"all_of,omitempty"`
	Forbid    bool     `json:"forbid,omitempty"`
	Exclusive bool     `json:"exclusive,omitempty"`
}

func catalogMediaExpect(m *MediaExpect) *CatalogMediaExpect {
	if m == nil {
		return nil
	}
	return &CatalogMediaExpect{AnyOf: m.AnyOf, AllOf: m.AllOf, Forbid: m.Forbid, Exclusive: m.Exclusive}
}

// CatalogOutcomeCase mirrors OutcomeCase with explicit JSON tags — a dedicated
// projection for the same reason as CatalogMediaExpect (see its comment): the untagged
// resolved_tests.json shape must never change as a side effect of catalog needs.
type CatalogOutcomeCase struct {
	Label          string              `json:"label"`
	Requires       [][]string          `json:"requires,omitempty"`
	Media          *CatalogMediaExpect `json:"media,omitempty"`
	Escalate       *bool               `json:"escalate,omitempty"`
	Language       string              `json:"language,omitempty"`
	MustNotContain []string            `json:"must_not_contain,omitempty"`
	MustContainAny []string            `json:"must_contain_any,omitempty"`
}

func catalogOutcomeCases(ocs []OutcomeCase) []CatalogOutcomeCase {
	if len(ocs) == 0 {
		return nil
	}
	out := make([]CatalogOutcomeCase, 0, len(ocs))
	for _, oc := range ocs {
		out = append(out, CatalogOutcomeCase{
			Label:          oc.Label,
			Requires:       oc.Requires,
			Media:          catalogMediaExpect(oc.Media),
			Escalate:       oc.Escalate,
			Language:       oc.Language,
			MustNotContain: oc.MustNotContain,
			MustContainAny: oc.MustContainAny,
		})
	}
	return out
}

// CatalogTestCase is a catalog-specific projection of TestCase — deliberately NOT
// TestCase itself (see CatalogMediaExpect's comment; the same reasoning applies here:
// resolved_tests.json's existing shape must never change). The one addition beyond
// TestCase's own fields is Source: the evals/-relative path of the file that actually
// DECLARED this test (the scenario's own tests.yaml, or the specific included common
// bank) — resolveTests (render.go) doesn't track this, so the catalog uses its own
// resolveCatalogTests instead of calling it.
//
// Escalate is a pointer for the same reason TestCase.Escalate is: nil (the yaml simply
// has no `escalate:` key) means "not checked by this test" and must render completely
// differently from an explicit `escalate: false` (an ACTIVE requirement that the model
// must NOT escalate) — collapsing the two would silently hide a real requirement.
type CatalogTestCase struct {
	ID             string               `json:"id"`
	Message        string               `json:"message"`
	History        []HistoryTurn        `json:"history,omitempty"`
	Requires       [][]string           `json:"requires,omitempty"`
	Language       string               `json:"language,omitempty"`
	Escalate       *bool                `json:"escalate,omitempty"`
	MustNotContain []string             `json:"must_not_contain,omitempty"`
	MustContainAny []string             `json:"must_contain_any,omitempty"`
	Media          *CatalogMediaExpect  `json:"media,omitempty"`
	Outcomes       []CatalogOutcomeCase `json:"outcomes,omitempty"`
	StockCheckRef  string               `json:"stock_check_ref,omitempty"`
	Source         string               `json:"source"`
}

// CatalogExtractCase is one extract/cases.yaml entry with its ground truth carried
// alongside the served path of its (already downscaled/re-encoded) image — the same
// preprocessing pipeline a real extraction call uses, so what's shown is exactly what
// a model would receive.
type CatalogExtractCase struct {
	ID                  string            `json:"id"`
	Image               string            `json:"image"` // served path, catalog/inputs/<id>.jpg
	Fields              map[string]string `json:"fields,omitempty"`
	TextContainsAll     []string          `json:"text_contains_all,omitempty"`
	IdentifyContainsAll []string          `json:"identify_contains_all,omitempty"`
	IdentifyContainsAny []string          `json:"identify_contains_any,omitempty"`
	AllowedNumbers      []string          `json:"allowed_numbers,omitempty"`
	RequiredNumbers     []string          `json:"required_numbers,omitempty"`
	ForbidCurrency      bool              `json:"forbid_currency,omitempty"`
	Source              string            `json:"source"`
}

// resolveCatalogTests mirrors resolveTests' (render.go) include-then-own-tests
// resolution order EXACTLY (same bank order, same "scenario's own tests appended
// last") but additionally records, per test, the evals/-relative path of the file
// that declared it — resolveTests itself is untouched (judge.go's grading path must
// keep reading the exact same resolved_tests.json shape it always has).
func resolveCatalogTests(scenarioName, scenarioDir, testsRel string) ([]CatalogTestCase, error) {
	testsPath := filepath.Join(scenarioDir, testsRel)
	b, err := os.ReadFile(testsPath)
	if err != nil {
		return nil, err
	}
	var tf TestsFile
	if err := yaml.Unmarshal(b, &tf); err != nil {
		return nil, err
	}
	var out []CatalogTestCase
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
		for _, tc := range bank.Tests {
			// Validated here too (not just render.go's resolveTests path) so a
			// forbid+any_of conflict fails `export -all` as loudly as it fails `render` —
			// see validateTestMedia's own comment for why both paths must agree.
			if err := validateTestMedia(scenarioName, tc.ID, tc.Media); err != nil {
				return nil, err
			}
			if err := validateTestOutcomes(scenarioName, tc.ID, tc.Outcomes); err != nil {
				return nil, err
			}
			out = append(out, catalogTestCaseFrom(tc, incPath))
		}
	}
	for _, tc := range tf.Tests {
		if err := validateTestMedia(scenarioName, tc.ID, tc.Media); err != nil {
			return nil, err
		}
		if err := validateTestOutcomes(scenarioName, tc.ID, tc.Outcomes); err != nil {
			return nil, err
		}
		out = append(out, catalogTestCaseFrom(tc, testsPath))
	}
	return out, nil
}

func catalogTestCaseFrom(tc TestCase, source string) CatalogTestCase {
	return CatalogTestCase{
		ID:             tc.ID,
		Message:        tc.Message,
		History:        tc.History,
		Requires:       tc.Requires,
		Language:       tc.Language,
		Escalate:       tc.Escalate,
		MustNotContain: tc.MustNotContain,
		MustContainAny: tc.MustContainAny,
		Media:          catalogMediaExpect(tc.Media),
		Outcomes:       catalogOutcomeCases(tc.Outcomes),
		StockCheckRef:  tc.StockCheckRef,
		Source:         source,
	}
}

// buildCatalogMedia mirrors buildCatalog's (render.go) exact media traversal — same
// rows, same union of both media shapes a data.yaml can use — but keeps each entry's
// Description (buildCatalog only needs the token/ref names, since Description is a
// prompt-rendering concern the model reads directly out of MEDIA%%, not something the
// grading catalog itself carries).
func buildCatalogMedia(data *Data) []CatalogMedia {
	var out []CatalogMedia
	for _, ft := range data.FactTables {
		for _, row := range ft.Rows {
			for _, m := range row.Media {
				if len(m.Files) == 0 {
					continue
				}
				out = append(out, CatalogMedia{
					Name: mediaGroupToken(ft.Table, row.Ref, m.Field), Kind: "group", Description: m.Description,
				})
			}
		}
	}
	for _, t := range data.Topics {
		for _, m := range t.Media {
			if len(m.Files) == 0 {
				continue
			}
			out = append(out, CatalogMedia{
				Name: mediaGroupToken("topic", t.Ref, m.Field), Kind: "group", Description: m.Description,
			})
		}
	}
	for _, a := range data.Assets {
		out = append(out, CatalogMedia{Name: a.Ref, Kind: a.Kind, Description: a.Description})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// buildCatalogScenario resolves one scenario dir's full requirements in memory —
// loadScenario/loadData/applyLimits/buildCatalog are the SAME functions renderScenario
// itself uses, called in the SAME order (limits applied before facts are built), so
// the catalog can never show a fact value or token the real render pipeline wouldn't
// also produce. Zero billed calls; nothing here calls a model.
func buildCatalogScenario(dir string) (*CatalogScenario, error) {
	scenario, err := loadScenario(dir)
	if err != nil {
		return nil, fmt.Errorf("load scenario.yaml: %w", err)
	}
	if scenario.Pipeline == "schema_kb_v1" {
		return buildCatalogScenarioSchemaKB(dir, scenario)
	}
	dataPath := filepath.Join(dir, scenario.Data)
	data, err := loadData(dataPath)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", dataPath, err)
	}
	if err := applyLimits(data, scenario.Limits); err != nil {
		return nil, fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}

	cat := buildCatalog(data)
	tests, err := resolveCatalogTests(scenario.Name, dir, scenario.Tests)
	if err != nil {
		return nil, fmt.Errorf("resolve tests: %w", err)
	}

	return &CatalogScenario{
		Name:           scenario.Name,
		Description:    scenario.Description,
		Setup:          scenario.Setup,
		PromptRef:      scenario.PromptRef,
		Experiment:     scenario.Experiment,
		Archived:       scenario.Archived,
		ArchivedReason: scenario.ArchivedReason,
		FactsSource:    dataPath,
		Facts:          cat.Tokens,
		MediaTokens:    cat.MediaTokens,
		Media:          buildCatalogMedia(data),
		Tests:          tests,
	}, nil
}

// buildCatalogScenarioSchemaKB is buildCatalogScenario's counterpart for the
// pipeline:schema_kb_v1 family — same "zero billed calls, real resolved values, same
// order render itself uses" contract, but loading via kbfixture and building the
// catalog via aiprompt.BuildCatalog instead of this file's legacy loadData/buildCatalog.
// Facts are resolved separately for this internal audit artifact via ResolveFact; the
// scenario's model-facing generated/catalog.json still serializes only the value-free
// aiprompt.Catalog. No material ID or other kbd_materials metadata reaches either file.
func buildCatalogScenarioSchemaKB(dir string, scenario *ScenarioConfig) (*CatalogScenario, error) {
	fixturePath := filepath.Join(dir, scenario.Data)
	kb, err := kbfixture.Load(fixturePath)
	if err != nil {
		return nil, fmt.Errorf("load fixture %s: %w", fixturePath, err)
	}
	if kb, err = kbfixture.ApplyLimits(kb, scenario.Limits); err != nil {
		return nil, fmt.Errorf("scenario %q: %w", scenario.Name, err)
	}
	cat, err := aiprompt.BuildCatalog(kb)
	if err != nil {
		return nil, fmt.Errorf("scenario %q: build catalog: %w", scenario.Name, err)
	}
	tests, err := resolveCatalogTests(scenario.Name, dir, scenario.Tests)
	if err != nil {
		return nil, fmt.Errorf("resolve tests: %w", err)
	}

	facts := make([]CatalogFact, 0, len(cat.Facts))
	for _, f := range cat.Facts {
		value, err := aiprompt.ResolveFact(f.Token, kb, cat)
		if err != nil {
			return nil, fmt.Errorf("scenario %q: resolve fact %s: %w", scenario.Name, f.Token, err)
		}
		facts = append(facts, CatalogFact{Token: f.Token, Value: value})
	}
	var mediaTokens []string
	media := make([]CatalogMedia, 0, len(cat.Media))
	for _, m := range cat.Media {
		mediaTokens = append(mediaTokens, m.Token)
		media = append(media, CatalogMedia{Name: m.Token, Kind: string(m.Kind), Description: m.Label})
	}
	sort.Strings(mediaTokens)
	sort.Slice(media, func(i, j int) bool { return media[i].Name < media[j].Name })

	return &CatalogScenario{
		Name:           scenario.Name,
		Description:    scenario.Description,
		Setup:          scenario.Setup,
		PromptRef:      scenario.PromptRef,
		Experiment:     scenario.Experiment,
		Archived:       scenario.Archived,
		ArchivedReason: scenario.ArchivedReason,
		Pipeline:       scenario.Pipeline,
		FactsSource:    fixturePath,
		Facts:          facts,
		MediaTokens:    mediaTokens,
		Media:          media,
		Tests:          tests,
	}, nil
}

// buildCatalogExtractCases loads extract/cases.yaml and, for every case, preprocesses
// its image through the SAME pipeline a real extraction call uses (preprocess —
// promptref.go) and writes the result under runsRoot/catalog/inputs/ — so the image
// shown is exactly what a model would receive, not the raw original file. Zero billed
// calls: preprocess only downscales/re-encodes locally.
func buildCatalogExtractCases(runsRoot, casesPath string) ([]CatalogExtractCase, error) {
	cf, err := loadExtractCases(casesPath)
	if err != nil {
		return nil, err
	}
	casesDir := filepath.Dir(casesPath)
	inputsDir := filepath.Join(runsRoot, "catalog", "inputs")
	if err := os.MkdirAll(inputsDir, 0o755); err != nil {
		return nil, err
	}

	var out []CatalogExtractCase
	for _, c := range cf.Cases {
		imgPath := filepath.Join(casesDir, c.File)
		imgData, _, _, err := preprocess(c.Kind, imgPath)
		if err != nil {
			return nil, fmt.Errorf("case %s: %w", c.ID, err)
		}
		dst := filepath.Join(inputsDir, c.ID+".jpg")
		if err := provenance.AtomicWriteFile(dst, imgData, 0o644); err != nil {
			return nil, fmt.Errorf("case %s: %w", c.ID, err)
		}
		out = append(out, CatalogExtractCase{
			ID:                  c.ID,
			Image:               filepath.ToSlash(filepath.Join("catalog", "inputs", c.ID+".jpg")),
			Fields:              c.Fields,
			TextContainsAll:     c.TextContainsAll,
			IdentifyContainsAll: c.IdentifyContainsAll,
			IdentifyContainsAny: c.IdentifyContainsAny,
			AllowedNumbers:      c.AllowedNumbers,
			RequiredNumbers:     c.RequiredNumbers,
			ForbidCurrency:      c.ForbidCurrency,
			Source:              casesPath,
		})
	}
	return out, nil
}

// writeCatalogJSON discovers every scenario under scenarios/*/scenario.yaml (the same
// glob -all uses in run.go/launch.go — a dir with no scenario.yaml, e.g. shop-scale's
// shared-data-only dir, is silently not a match, never an error) and every case in
// extract/cases.yaml, resolves both to real values, and writes runsRoot/catalog.json —
// the one export command a fresh clone (or a requirements review before any billed
// run) needs.
func writeCatalogJSON(runsRoot string) error {
	matches, err := filepath.Glob(filepath.Join("scenarios", "*", "scenario.yaml"))
	if err != nil {
		return err
	}
	sort.Strings(matches)

	var scenarios []CatalogScenario
	for _, m := range matches {
		cs, err := buildCatalogScenario(filepath.Dir(m))
		if err != nil {
			return fmt.Errorf("catalog: %s: %w", m, err)
		}
		scenarios = append(scenarios, *cs)
	}

	// A checkout with no extraction eval configured at all (extract/cases.yaml simply
	// absent — e.g. a scratch/test environment, or historically before this file
	// existed) is not an error: same fail-soft treatment as an unannotated/missing
	// scenario snapshot elsewhere in this package. A cases.yaml that EXISTS but is
	// malformed still fails loudly, via buildCatalogExtractCases below.
	var extractCases []CatalogExtractCase
	casesPath := filepath.Join("extract", "cases.yaml")
	if _, statErr := os.Stat(casesPath); statErr == nil {
		extractCases, err = buildCatalogExtractCases(runsRoot, casesPath)
	} else if !os.IsNotExist(statErr) {
		err = statErr
	}
	if err != nil {
		return fmt.Errorf("catalog: extract cases: %w", err)
	}

	file := CatalogFile{
		SchemaVersion: CatalogFileSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Scenarios:     scenarios,
		ExtractCases:  extractCases,
	}
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return provenance.AtomicWriteFile(filepath.Join(runsRoot, "catalog.json"), b, 0o644)
}
