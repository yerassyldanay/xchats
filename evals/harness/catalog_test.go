package main

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// writeCatalogFixtureScenario hand-writes scenarios/<name>/{scenario.yaml,data.yaml,
// tests.yaml} under root, mirroring what a real scenario dir looks like, without
// needing loadScenario/loadData's yaml.Marshal round-trip to match production authoring
// exactly (these are read back by the SAME loaders production uses).
func writeCatalogFixtureScenario(t *testing.T, root, name string, sc ScenarioConfig, data Data, testsYAML string) {
	t.Helper()
	sc.Name = name
	dir := filepath.Join(root, "scenarios", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	scB, err := yaml.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), scB, 0o644); err != nil {
		t.Fatal(err)
	}
	dataB, err := yaml.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data.yaml"), dataB, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tests.yaml"), []byte(testsYAML), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveCatalogTests_IncludeOrderAndSource(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	if err := os.MkdirAll(filepath.Join(root, "common"), 0o755); err != nil {
		t.Fatal(err)
	}
	bankYAML := "tests:\n  - id: \"1. from bank\"\n    message: \"bank question\"\n"
	if err := os.WriteFile(filepath.Join(root, "common", "bank.yaml"), []byte(bankYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	scenarioDir := filepath.Join(root, "scenarios", "s1")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testsYAML := "include:\n  - common/bank.yaml\ntests:\n  - id: \"2. own\"\n    message: \"own question\"\n"
	if err := os.WriteFile(filepath.Join(scenarioDir, "tests.yaml"), []byte(testsYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	tests, err := resolveCatalogTests("s1", scenarioDir, "tests.yaml")
	if err != nil {
		t.Fatalf("resolveCatalogTests: %v", err)
	}
	if len(tests) != 2 {
		t.Fatalf("want 2 tests (bank then own), got %d: %+v", len(tests), tests)
	}
	if tests[0].ID != "1. from bank" || tests[0].Source != filepath.Join("common", "bank.yaml") {
		t.Errorf("want bank test first with Source=common/bank.yaml, got %+v", tests[0])
	}
	if tests[1].ID != "2. own" || tests[1].Source != filepath.Join(scenarioDir, "tests.yaml") {
		t.Errorf("want own test second with Source=%s, got %+v", filepath.Join(scenarioDir, "tests.yaml"), tests[1])
	}
}

func TestBuildCatalogScenario_LimitsAppliedBeforeFactsAreBuilt(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	data := Data{
		FactTables: []FactTable{{
			Table:  "product",
			Fields: []FieldSpec{{Name: "price", Label: "цена"}},
			Rows: []FactRow{
				{Ref: "p1", DisplayName: "Товар 1", Values: map[string]string{"price": "100"}},
				{Ref: "p2", DisplayName: "Товар 2", Values: map[string]string{"price": "200"}},
				{Ref: "p3", DisplayName: "Товар 3", Values: map[string]string{"price": "300"}},
			},
		}},
	}
	writeCatalogFixtureScenario(t, root, "capped", ScenarioConfig{
		Data: "data.yaml", Tests: "tests.yaml",
		Limits: map[string]int{"product": 1},
	}, data, "tests:\n")

	cs, err := buildCatalogScenario(filepath.Join(root, "scenarios", "capped"))
	if err != nil {
		t.Fatalf("buildCatalogScenario: %v", err)
	}
	if len(cs.Facts) != 1 {
		t.Fatalf("want exactly 1 fact (limits: product=1 applied before facts built), got %d: %+v", len(cs.Facts), cs.Facts)
	}
	if cs.Facts[0].Token != "{{product.p1.price}}" || cs.Facts[0].Value != "100" {
		t.Errorf("want the FIRST row's fact only, got %+v", cs.Facts[0])
	}
}

// TestBuildCatalogScenario_ArchivedFieldsCarried confirms the catalog's Archived/
// ArchivedReason are a direct copy of ScenarioConfig's own fields (writeCatalogJSON's
// doc comment: the catalog is a repository-state snapshot, never a re-derivation) —
// both for an archived scenario and for the (default) non-archived case, so a
// forgotten `archived: true` never silently defaults to "archived" for every scenario.
func TestBuildCatalogScenario_ArchivedFieldsCarried(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	writeCatalogFixtureScenario(t, root, "retired", ScenarioConfig{
		Data: "data.yaml", Tests: "tests.yaml", Archived: true, ArchivedReason: "superseded by v2",
	}, Data{}, "tests:\n")
	writeCatalogFixtureScenario(t, root, "active", ScenarioConfig{
		Data: "data.yaml", Tests: "tests.yaml",
	}, Data{}, "tests:\n")

	retired, err := buildCatalogScenario(filepath.Join(root, "scenarios", "retired"))
	if err != nil {
		t.Fatalf("buildCatalogScenario(retired): %v", err)
	}
	if !retired.Archived || retired.ArchivedReason != "superseded by v2" {
		t.Errorf("want Archived=true with the reason carried through, got Archived=%v Reason=%q", retired.Archived, retired.ArchivedReason)
	}

	active, err := buildCatalogScenario(filepath.Join(root, "scenarios", "active"))
	if err != nil {
		t.Fatalf("buildCatalogScenario(active): %v", err)
	}
	if active.Archived || active.ArchivedReason != "" {
		t.Errorf("want Archived=false with no reason for an unannotated scenario, got Archived=%v Reason=%q", active.Archived, active.ArchivedReason)
	}
}

// TestBuildCatalogScenario_TestsPathRecorded proves TestsPath names the actual
// tests.yaml a scenario resolves against — both for a scenario that owns its own file
// and for a "borrow case" (ScenarioConfig.Tests pointing at another scenario's dir via
// "../"), where filepath.Join must clean the traversal into a normal scenarios/-rooted
// path rather than leaking the borrower's own dir.
func TestBuildCatalogScenario_TestsPathRecorded(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	writeCatalogFixtureScenario(t, root, "owner", ScenarioConfig{
		Data: "data.yaml", Tests: "tests.yaml",
	}, Data{}, "tests:\n")

	borrowerDir := filepath.Join(root, "scenarios", "borrower")
	if err := os.MkdirAll(borrowerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sc := ScenarioConfig{Name: "borrower", Data: "data.yaml", Tests: "../owner/tests.yaml"}
	scB, err := yaml.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(borrowerDir, "scenario.yaml"), scB, 0o644); err != nil {
		t.Fatal(err)
	}
	dataB, err := yaml.Marshal(Data{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(borrowerDir, "data.yaml"), dataB, 0o644); err != nil {
		t.Fatal(err)
	}

	// buildCatalogScenario is called here the same way writeCatalogJSON actually calls
	// it — with a dir RELATIVE to the evals/ CWD (filepath.Glob's own matches are
	// relative) — since TestsPath's exact shape depends on that: an absolute dir would
	// make TestsPath absolute too, which is not what production ever produces.
	owner, err := buildCatalogScenario(filepath.Join("scenarios", "owner"))
	if err != nil {
		t.Fatalf("buildCatalogScenario(owner): %v", err)
	}
	if owner.TestsPath != filepath.ToSlash(filepath.Join("scenarios", "owner", "tests.yaml")) {
		t.Errorf("want TestsPath=scenarios/owner/tests.yaml, got %s", owner.TestsPath)
	}

	borrower, err := buildCatalogScenario(filepath.Join("scenarios", "borrower"))
	if err != nil {
		t.Fatalf("buildCatalogScenario(borrower): %v", err)
	}
	if borrower.TestsPath != filepath.ToSlash(filepath.Join("scenarios", "owner", "tests.yaml")) {
		t.Errorf("want the '../' borrow cleaned into scenarios/owner/tests.yaml, got %s", borrower.TestsPath)
	}
}

func TestBuildCatalogScenario_FactsResolvedToRealValuesAndSourceRecorded(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	data := Data{
		FactTables: []FactTable{{
			Table:  "product",
			Fields: []FieldSpec{{Name: "price", Label: "цена"}},
			Rows:   []FactRow{{Ref: "coffee-machine", DisplayName: "Кофемашина", Values: map[string]string{"price": "129 900 ₸"}}},
		}},
	}
	writeCatalogFixtureScenario(t, root, "shop", ScenarioConfig{
		Data: "data.yaml", Tests: "tests.yaml",
		Description: "Test shop", Setup: "shop-v1", Experiment: "shop-bakeoff",
	}, data, "tests:\n")

	cs, err := buildCatalogScenario(filepath.Join(root, "scenarios", "shop"))
	if err != nil {
		t.Fatalf("buildCatalogScenario: %v", err)
	}
	if len(cs.Facts) != 1 || cs.Facts[0].Token != "{{product.coffee-machine.price}}" || cs.Facts[0].Value != "129 900 ₸" {
		t.Fatalf("want the real resolved value, got %+v", cs.Facts)
	}
	wantSource := filepath.Join(root, "scenarios", "shop", "data.yaml")
	if cs.FactsSource != wantSource {
		t.Errorf("want FactsSource=%s, got %s", wantSource, cs.FactsSource)
	}
	if cs.Setup != "shop-v1" || cs.Experiment != "shop-bakeoff" || cs.Description != "Test shop" {
		t.Errorf("want scenario metadata carried through, got %+v", cs)
	}
}

// TestCatalogTestCaseFrom_EscalateNilVsFalse is the core proof for the "not checked"
// vs "active requirement" distinction: nil must be OMITTED from JSON (so the frontend
// can tell "no escalate key at all" apart from "false"), and an explicit false must be
// PRESENT — Go's `omitempty` on a *bool only omits a nil pointer, never a pointer to a
// zero value, so this is exercised end-to-end through json.Marshal, not just asserted
// on the Go struct.
func TestCatalogTestCaseFrom_EscalateNilVsFalse(t *testing.T) {
	notChecked := catalogTestCaseFrom(TestCase{ID: "t1", Message: "m"}, "tests.yaml")
	falseVal := false
	activeRequirement := catalogTestCaseFrom(TestCase{ID: "t2", Message: "m", Escalate: &falseVal}, "tests.yaml")

	b1, err := json.Marshal(notChecked)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b1), "escalate") {
		t.Errorf("want no escalate key at all when TestCase.Escalate is nil (not checked), got %s", b1)
	}

	b2, err := json.Marshal(activeRequirement)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b2), `"escalate":false`) {
		t.Errorf("want \"escalate\":false present (an ACTIVE must-not-escalate requirement), got %s", b2)
	}
}

func TestCatalogTestCaseFrom_MediaExpectCarriedWithJSONTags(t *testing.T) {
	tc := catalogTestCaseFrom(TestCase{ID: "t1", Message: "m", Media: &MediaExpect{AnyOf: []string{"photo-1"}}}, "tests.yaml")
	if tc.Media == nil || len(tc.Media.AnyOf) != 1 || tc.Media.AnyOf[0] != "photo-1" {
		t.Fatalf("want media carried through, got %+v", tc.Media)
	}
	b, err := json.Marshal(tc.Media)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"any_of"`) {
		t.Errorf("want snake_case json tags on CatalogMediaExpect, got %s", b)
	}

	// A TestCase with no media expectation must carry a nil Media, not an empty struct
	// (an empty CatalogMediaExpect would render as an active-but-empty requirement).
	noMedia := catalogTestCaseFrom(TestCase{ID: "t2", Message: "m"}, "tests.yaml")
	if noMedia.Media != nil {
		t.Errorf("want nil Media when the test declares none, got %+v", noMedia.Media)
	}

	forbid := catalogTestCaseFrom(TestCase{ID: "t3", Message: "m", Media: &MediaExpect{Forbid: true}}, "tests.yaml")
	if forbid.Media == nil || !forbid.Media.Forbid {
		t.Fatalf("want Forbid carried through, got %+v", forbid.Media)
	}
	fb, err := json.Marshal(forbid.Media)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fb), `"forbid":true`) {
		t.Errorf("want forbid:true present when set, got %s", fb)
	}
	notForbidden := catalogTestCaseFrom(TestCase{ID: "t4", Message: "m", Media: &MediaExpect{AnyOf: []string{"photo-1"}}}, "tests.yaml")
	nfb, err := json.Marshal(notForbidden.Media)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(nfb), "forbid") {
		t.Errorf("want forbid key OMITTED (omitempty) when false, got %s", nfb)
	}

	exclusive := catalogTestCaseFrom(TestCase{ID: "t5", Message: "m", Media: &MediaExpect{AnyOf: []string{"photo-1"}, Exclusive: true}}, "tests.yaml")
	if exclusive.Media == nil || !exclusive.Media.Exclusive {
		t.Fatalf("want Exclusive carried through, got %+v", exclusive.Media)
	}
	eb, err := json.Marshal(exclusive.Media)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(eb), `"exclusive":true`) {
		t.Errorf("want exclusive:true present when set, got %s", eb)
	}
	nonExclusive := catalogTestCaseFrom(TestCase{ID: "t6", Message: "m", Media: &MediaExpect{AnyOf: []string{"photo-1"}}}, "tests.yaml")
	neb, err := json.Marshal(nonExclusive.Media)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(neb), "exclusive") {
		t.Errorf("want exclusive key OMITTED (omitempty) when false, got %s", neb)
	}
}

// TestCatalogTestCaseFrom_StockCheckForbidTokensAndLLMChecks proves the v3 fields
// export under the yaml key names, not the Go field names — stock_check (not
// stock_check_ref: TestCase.StockCheckRef's yaml tag is "stock_check", and the JSON
// projection must match it), and that forbid_tokens/llm_checks are present when set,
// omitted (not empty arrays) when the test declares neither.
func TestCatalogTestCaseFrom_StockCheckForbidTokensAndLLMChecks(t *testing.T) {
	full := catalogTestCaseFrom(TestCase{
		ID: "t1", Message: "m",
		ForbidTokens:  []string{"product.blender-philips."},
		StockCheckRef: "blender-philips",
		LLMChecks:     []LLMCheck{{Claim: "no delivery date promised", Expect: true}},
	}, "tests.yaml")

	b, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"stock_check":"blender-philips"`) {
		t.Errorf("want stock_check key (matching the yaml key), got %s", s)
	}
	if strings.Contains(s, "stock_check_ref") {
		t.Errorf("want stock_check_ref NOT present (renamed to stock_check), got %s", s)
	}
	if !strings.Contains(s, `"forbid_tokens":["product.blender-philips."]`) {
		t.Errorf("want forbid_tokens carried through, got %s", s)
	}
	if !strings.Contains(s, `"llm_checks":[{"claim":"no delivery date promised","expect":true}]`) {
		t.Errorf("want llm_checks carried through with claim/expect tags, got %s", s)
	}

	bare := catalogTestCaseFrom(TestCase{ID: "t2", Message: "m"}, "tests.yaml")
	bb, err := json.Marshal(bare)
	if err != nil {
		t.Fatal(err)
	}
	bs := string(bb)
	for _, key := range []string{"stock_check", "forbid_tokens", "llm_checks"} {
		if strings.Contains(bs, key) {
			t.Errorf("want %q OMITTED (omitempty) when the test declares none, got %s", key, bs)
		}
	}
}

// TestResolveCatalogTests_RejectsConflictedMediaExpectation proves the catalog export
// path (`export -all`, via resolveCatalogTests) enforces the same forbid+any_of
// invariant as render's resolveTests — a conflicted test must fail loudly here too, not
// just at render time, since a scenario could be exported without ever being rendered
// first in the same invocation.
func TestResolveCatalogTests_RejectsConflictedMediaExpectation(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	scenarioDir := filepath.Join(root, "scenarios", "s1")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testsYAML := "tests:\n  - id: \"bad\"\n    message: \"hi\"\n    media:\n      forbid: true\n      any_of: [\"r\"]\n"
	if err := os.WriteFile(filepath.Join(scenarioDir, "tests.yaml"), []byte(testsYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveCatalogTests("s1", scenarioDir, "tests.yaml")
	if err == nil {
		t.Fatal("want an error for a test declaring both media.forbid and any_of")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("want the error to name the conflicting test id, got %v", err)
	}
}

// TestResolveCatalogTests_RejectsExclusiveWithoutAnyOf proves the export path enforces
// Exclusive's own invariant too (it cannot stand alone without a non-empty any_of_* to
// narrow) — the same "fails loudly on export, not just render" guarantee as the
// forbid+any_of case above.
func TestResolveCatalogTests_RejectsExclusiveWithoutAnyOf(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	scenarioDir := filepath.Join(root, "scenarios", "s1")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testsYAML := "tests:\n  - id: \"bad\"\n    message: \"hi\"\n    media:\n      exclusive: true\n"
	if err := os.WriteFile(filepath.Join(scenarioDir, "tests.yaml"), []byte(testsYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveCatalogTests("s1", scenarioDir, "tests.yaml")
	if err == nil {
		t.Fatal("want an error for a test declaring media.exclusive with no any_of_groups/any_of_refs")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("want the error to name the conflicting test id, got %v", err)
	}
}

func writeCatalogFixtureImage(t *testing.T, path string, w, h int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestBuildCatalogExtractCases_ImagePathAndGroundTruthCarried(t *testing.T) {
	root := t.TempDir()
	extractDir := filepath.Join(root, "extract")
	writeCatalogFixtureImage(t, filepath.Join(root, "assets", "infographic.png"), 200, 100)

	casesYAML := `
cases:
  - id: infographic
    file: ../assets/infographic.png
    fields:
      content_kind: infographic
    text_contains_all: ["старт", "рост"]
    allowed_numbers: ["10 000"]
`
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}
	casesPath := filepath.Join(extractDir, "cases.yaml")
	if err := os.WriteFile(casesPath, []byte(casesYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	runsRoot := filepath.Join(root, "runs")

	cases, err := buildCatalogExtractCases(runsRoot, casesPath)
	if err != nil {
		t.Fatalf("buildCatalogExtractCases: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("want 1 case, got %d", len(cases))
	}
	c := cases[0]
	if c.ID != "infographic" || c.Source != casesPath {
		t.Errorf("want id=infographic source=%s, got %+v", casesPath, c)
	}
	if c.Image != filepath.ToSlash(filepath.Join("catalog", "inputs", "infographic.jpg")) {
		t.Errorf("want served image path catalog/inputs/infographic.jpg, got %s", c.Image)
	}
	if c.Fields["content_kind"] != "infographic" || len(c.TextContainsAll) != 2 || len(c.AllowedNumbers) != 1 {
		t.Errorf("want ground truth carried through, got %+v", c)
	}
	// The preprocessed (downscaled/re-encoded) image must actually be written where Image claims.
	if _, err := os.Stat(filepath.Join(runsRoot, "catalog", "inputs", "infographic.jpg")); err != nil {
		t.Errorf("want the processed image written to disk: %v", err)
	}
}

func TestWriteCatalogJSON_SkipsScenarioDirsWithoutScenarioYAML(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	data := Data{FactTables: []FactTable{{Table: "product", Fields: []FieldSpec{{Name: "price"}}, Rows: []FactRow{{Ref: "p1", Values: map[string]string{"price": "1"}}}}}}
	writeCatalogFixtureScenario(t, root, "real-scenario", ScenarioConfig{Data: "data.yaml", Tests: "tests.yaml"}, data, "tests:\n")

	// A dir with data but NO scenario.yaml (mirrors shop-scale's real shape) must be
	// silently skipped, never an error.
	noScenarioDir := filepath.Join(root, "scenarios", "shared-data-only")
	if err := os.MkdirAll(noScenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noScenarioDir, "data.yaml"), []byte("fact_tables: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeCatalogFixtureImage(t, filepath.Join(root, "assets", "x.png"), 10, 10)
	if err := os.MkdirAll(filepath.Join(root, "extract"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extract", "cases.yaml"), []byte("cases:\n  - id: x\n    file: ../assets/x.png\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeCatalogJSON("runs"); err != nil {
		t.Fatalf("writeCatalogJSON: %v", err)
	}

	var file CatalogFile
	b, err := os.ReadFile(filepath.Join(root, "runs", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Scenarios) != 1 || file.Scenarios[0].Name != "real-scenario" {
		t.Fatalf("want exactly the one real scenario, the scenario.yaml-less dir silently skipped, got %+v", file.Scenarios)
	}
	if file.GeneratedAt == "" {
		t.Error("want GeneratedAt populated")
	}
	if len(file.ExtractCases) != 1 {
		t.Errorf("want the extract case carried too, got %d", len(file.ExtractCases))
	}
}
