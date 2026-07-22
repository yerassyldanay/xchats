package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xchats-evals-harness/internal/provenance"
)

func TestResolveScenarioRunInputs_PrefersSnapshottedScenarioAndFixture(t *testing.T) {
	scenarioDir := t.TempDir()
	_, modelsPath := writeSchemaKBScenarioDir(t, scenarioDir, testSchemaKBFixture)
	if err := renderScenario(scenarioDir, modelsPath, ""); err != nil {
		t.Fatal(err)
	}
	original, err := loadScenario(scenarioDir)
	if err != nil {
		t.Fatal(err)
	}
	runDir := t.TempDir()
	fixturePath := filepath.Join(scenarioDir, original.Data)
	if _, err := provenance.SnapshotScenario(
		scenarioDir,
		runDir,
		original.Name,
		fixturePath,
	); err != nil {
		t.Fatal(err)
	}

	mutatedScenario := strings.Replace(
		mustReadFile(t, filepath.Join(scenarioDir, "scenario.yaml")),
		"tests: tests.yaml",
		"tests: tests.yaml\nlimits:\n  ai_products: 0",
		1,
	)
	mustWrite(t, filepath.Join(scenarioDir, "scenario.yaml"), mutatedScenario)
	mustWrite(t, fixturePath, "organization_id: changed-live-fixture\n")

	inputs, err := resolveScenarioRunInputs(scenarioDir, runDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs.Scenario.Limits) != 0 {
		t.Fatalf("resolved limits = %+v, want snapshotted no-limits config", inputs.Scenario.Limits)
	}
	if filepath.Base(inputs.FixturePath) != "kb-fixture.yaml" {
		t.Fatalf("fixture path = %s, want snapshotted kb-fixture.yaml", inputs.FixturePath)
	}
	fixture := mustReadFile(t, inputs.FixturePath)
	if strings.Contains(fixture, "changed-live-fixture") || !strings.Contains(fixture, "Demo Shop") {
		t.Fatalf("resolved fixture is not the immutable snapshot:\n%s", fixture)
	}
	if inputs.GeneratedDir != filepath.Join(runDir, "snapshots", original.Name) {
		t.Fatalf("generated dir = %s, want run snapshot", inputs.GeneratedDir)
	}
}

func TestJudgeScenario_UsesSnapshottedSchemaFixtureAfterLiveMutation(t *testing.T) {
	scenarioDir := t.TempDir()
	_, modelsPath := writeSchemaKBScenarioDir(t, scenarioDir, testSchemaKBFixture)
	if err := renderScenario(scenarioDir, modelsPath, ""); err != nil {
		t.Fatal(err)
	}
	scenario, err := loadScenario(scenarioDir)
	if err != nil {
		t.Fatal(err)
	}
	runDir := t.TempDir()
	fixturePath := filepath.Join(scenarioDir, scenario.Data)
	if _, err := provenance.SnapshotScenario(
		scenarioDir,
		runDir,
		scenario.Name,
		fixturePath,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := provenance.SnapshotFile(modelsPath, filepath.Join(runDir, "snapshots", "models.yaml")); err != nil {
		t.Fatal(err)
	}
	results := PromptfooResults{}
	row := schemaKBPromptfooRow(schemaKBResponse(
		t,
		"Цена — {{product.coffee-machine.price}}.",
		[]string{},
		"",
	))
	row.Provider.ID = "openrouter:google/gemini-2.5-flash"
	row.TestCase.Description = "1. price"
	results.Results.Results = []PromptfooRow{row}
	if err := writeJSON(filepath.Join(runDir, scenario.Name+".results.json"), results); err != nil {
		t.Fatal(err)
	}

	liveFixture := mustReadFile(t, fixturePath)
	liveFixture = strings.Replace(liveFixture, "129 900 ₸", "999 999 ₸", 1)
	mustWrite(t, fixturePath, liveFixture)
	if err := judgeScenario(scenarioDir, runDir, modelsPath); err != nil {
		t.Fatal(err)
	}
	var judged JudgedRun
	if err := readJSON(filepath.Join(runDir, scenario.Name+".judged.json"), &judged); err != nil {
		t.Fatal(err)
	}
	if len(judged.Verdicts) != 1 {
		t.Fatalf("verdict count = %d, want 1", len(judged.Verdicts))
	}
	injected := judged.Verdicts[0].InjectedText
	if !strings.Contains(injected, "129 900 ₸") || strings.Contains(injected, "999 999 ₸") {
		t.Fatalf("judge used the live fixture instead of the snapshot: %q", injected)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
