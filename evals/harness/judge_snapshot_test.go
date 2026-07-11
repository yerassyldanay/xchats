package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeMinimalScenarioFiles writes just enough for judgeScenario to load: scenario.yaml
// (Name only matters) plus a generated/ dir with a catalog.json holding exactly one
// token ({{x}} -> tokenValue) and a resolved_tests.json with one test requiring it.
func writeMinimalScenarioFiles(t *testing.T, dir, tokenValue string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "scenario.yaml"), []byte("name: fixture-scenario\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	genDir := filepath.Join(dir, "generated")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cat := Catalog{
		Contract: "attach_groups",
		Tokens:   []CatalogFact{{Token: "{{x}}", Value: tokenValue}},
	}
	if err := writeJSON(filepath.Join(genDir, "catalog.json"), cat); err != nil {
		t.Fatal(err)
	}
	resolved := ResolvedTests{Tests: []TestCase{{
		ID:       "t1",
		Message:  "hi",
		Requires: [][]string{{"x"}},
	}}}
	if err := writeJSON(filepath.Join(genDir, "resolved_tests.json"), resolved); err != nil {
		t.Fatal(err)
	}
}

func writeMinimalResults(t *testing.T, runDir, scenarioName, output string) {
	t.Helper()
	results := PromptfooResults{}
	row := PromptfooRow{}
	row.Provider.ID = "openrouter:test/model"
	row.TestCase.Description = "t1"
	row.Response.Output = output
	results.Results.Results = []PromptfooRow{row}
	if err := writeJSON(filepath.Join(runDir, scenarioName+".results.json"), results); err != nil {
		t.Fatal(err)
	}
}

// TestJudgeScenario_FallsBackToLiveGeneratedWhenNoSnapshot proves the legacy path:
// a run dir with no runs/<id>/snapshots/<scenario>/ (every run created before this
// provenance feature existed, or a standalone `judge` run right after a fresh
// `render`) must grade against the live scenarioDir/generated exactly as before.
func TestJudgeScenario_FallsBackToLiveGeneratedWhenNoSnapshot(t *testing.T) {
	scenarioDir := t.TempDir()
	writeMinimalScenarioFiles(t, scenarioDir, "LIVE-VALUE")

	runDir := t.TempDir()
	// {{x}} injects to the live catalog's value; requires "x" is satisfied only if the
	// resolved value flows through — this doubles as the fallback-source proof AND a
	// sanity check that judging still works end to end.
	writeMinimalResults(t, runDir, "fixture-scenario", `{"reply_text":"val={{x}}","reply_language":"ru","attach_groups":[],"escalate":false}`)

	if err := judgeScenario(scenarioDir, runDir, "../models.yaml"); err != nil {
		t.Fatalf("judgeScenario: %v", err)
	}
	var jr JudgedRun
	if err := readJSON(filepath.Join(runDir, "fixture-scenario.judged.json"), &jr); err != nil {
		t.Fatal(err)
	}
	if len(jr.Verdicts) != 1 {
		t.Fatalf("want 1 verdict, got %d", len(jr.Verdicts))
	}
	v := jr.Verdicts[0]
	if !v.RequiresPass {
		t.Fatalf("want requires to pass using the live generated/catalog.json, got reason=%q injected=%q", v.Reason, v.InjectedText)
	}
	if v.InjectedText != "val=LIVE-VALUE" {
		t.Fatalf("want injected text from the live catalog (LIVE-VALUE), got %q", v.InjectedText)
	}
}

// TestJudgeScenario_PrefersRunSnapshotOverLiveGenerated proves the whole point of
// snapshotting: once a run has its own runs/<id>/snapshots/<scenario>/, judging that
// run MUST use the snapshot's requirements even if the live scenarioDir/generated has
// since changed to something different — re-judging an old run must never silently
// grade against today's requirements.
func TestJudgeScenario_PrefersRunSnapshotOverLiveGenerated(t *testing.T) {
	scenarioDir := t.TempDir()
	writeMinimalScenarioFiles(t, scenarioDir, "LIVE-VALUE-CHANGED-SINCE")

	runDir := t.TempDir()
	// Snapshot holds a DIFFERENT token value than the live dir — if judgeScenario ever
	// reads the live dir instead, the injected text below would prove it by mismatching.
	snapDir := filepath.Join(runDir, "snapshots", "fixture-scenario")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "scenario.yaml"), []byte("name: fixture-scenario\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := Catalog{Contract: "attach_groups", Tokens: []CatalogFact{{Token: "{{x}}", Value: "SNAPSHOT-VALUE"}}}
	if err := writeJSON(filepath.Join(snapDir, "catalog.json"), cat); err != nil {
		t.Fatal(err)
	}
	resolved := ResolvedTests{Tests: []TestCase{{ID: "t1", Message: "hi", Requires: [][]string{{"x"}}}}}
	if err := writeJSON(filepath.Join(snapDir, "resolved_tests.json"), resolved); err != nil {
		t.Fatal(err)
	}

	writeMinimalResults(t, runDir, "fixture-scenario", `{"reply_text":"val={{x}}","reply_language":"ru","attach_groups":[],"escalate":false}`)

	if err := judgeScenario(scenarioDir, runDir, "../models.yaml"); err != nil {
		t.Fatalf("judgeScenario: %v", err)
	}
	var jr JudgedRun
	if err := readJSON(filepath.Join(runDir, "fixture-scenario.judged.json"), &jr); err != nil {
		t.Fatal(err)
	}
	if len(jr.Verdicts) != 1 {
		t.Fatalf("want 1 verdict, got %d", len(jr.Verdicts))
	}
	v := jr.Verdicts[0]
	if v.InjectedText != "val=SNAPSHOT-VALUE" {
		t.Fatalf("want injected text from the RUN'S OWN SNAPSHOT (SNAPSHOT-VALUE), got %q — judge read the live (changed) scenario dir instead", v.InjectedText)
	}
}
