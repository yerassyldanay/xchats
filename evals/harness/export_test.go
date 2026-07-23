package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"xchats-evals-harness/internal/provenance"
)

// TestWriteExecutionsJSON_SchemaAndLaunchIDThreading proves executions.json is a
// dedicated, schema-versioned export (not a reused html/template struct — review
// amendment 5) and that LaunchID comes from the run's OWN manifest.json, matching
// index.go's buildRunsIndexRow's source of truth.
func TestWriteExecutionsJSON_SchemaAndLaunchIDThreading(t *testing.T) {
	runDir := t.TempDir()
	m := provenance.NewManifest(runDir, filepath.Base(runDir), "scenario", "run", nil)
	m.LaunchID = "2026-07-14_10-00-00-abcd"
	if err := provenance.WriteManifest(runDir, m); err != nil {
		t.Fatal(err)
	}

	execs := []VExecution{{Family: "scenario", Subject: VSubject{TestID: "t1"}, Variant: VVariant{Model: "m1"}}}
	if err := writeExecutionsJSON(runDir, execs); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(runDir, "executions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file ExecutionsFile
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatalf("executions.json is not valid JSON matching ExecutionsFile: %v", err)
	}
	if file.SchemaVersion != ExecutionsFileSchemaVersion {
		t.Errorf("want schema_version=%d, got %d", ExecutionsFileSchemaVersion, file.SchemaVersion)
	}
	if file.LaunchID != "2026-07-14_10-00-00-abcd" {
		t.Errorf("want launch_id threaded from manifest.json, got %q", file.LaunchID)
	}
	if len(file.Executions) != 1 || file.Executions[0].Variant.Model != "m1" {
		t.Fatalf("want the one execution round-tripped, got %+v", file.Executions)
	}

	// Field names are snake_case, explicit tags — never the bare Go identifiers a
	// struct with no json tags (like runsIndexRow) would produce.
	if _, ok := parseGeneric(t, b)["schema_version"]; !ok {
		t.Error("want literal JSON key \"schema_version\" present")
	}
}

// TestWriteExecutionsJSON_NoManifestLeavesLaunchIDEmpty proves a run with no
// manifest.json (a standalone `judge` invocation, or a run from before provenance
// existed) exports cleanly with an empty launch_id, not an error.
func TestWriteExecutionsJSON_NoManifestLeavesLaunchIDEmpty(t *testing.T) {
	runDir := t.TempDir()
	if err := writeExecutionsJSON(runDir, nil); err != nil {
		t.Fatal(err)
	}
	var file ExecutionsFile
	b, err := os.ReadFile(filepath.Join(runDir, "executions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatal(err)
	}
	if file.LaunchID != "" {
		t.Errorf("want empty launch_id with no manifest.json, got %q", file.LaunchID)
	}
}

// TestRunSummaryFromIndexRow_IsAPureFieldCopy proves the dedicated runs.json export
// shape never re-derives counts itself — it only copies what buildRunsIndexRows
// already computed (review amendment 5's "never a second computation that could
// drift"), including translating IndexHref's presence into the explicit HasIndexHTML
// boolean the frontend needs (raw href strings are an internal detail of the
// html/template row, not part of the export contract).
func TestRunSummaryFromIndexRow_IsAPureFieldCopy(t *testing.T) {
	row := runsIndexRow{
		RunID: "r1", LaunchID: "l1", HasManifest: true, Family: "scenario",
		Models: []string{"m1", "m2"}, Prompts: []string{"p@v1"},
		StartedAt: "2026-01-01T00:00:00Z", FinishedAt: "2026-01-01T00:24:18Z",
		ScenarioTotal: 10, ScenarioBehaviorPass: 8, ScenarioContractPass: 10,
		IndexHref: "r1/index.html",
	}
	s := runSummaryFromIndexRow(row)
	if s.RunID != "r1" || s.LaunchID != "l1" || s.Family != "scenario" {
		t.Errorf("want identity fields copied verbatim, got %+v", s)
	}
	if s.ScenarioBehaviorPass != 8 || s.ScenarioTotal != 10 {
		t.Errorf("want counts copied verbatim (not recomputed), got %+v", s)
	}
	if s.FinishedAt != "2026-01-01T00:24:18Z" {
		t.Errorf("want FinishedAt copied verbatim, got %q", s.FinishedAt)
	}
	if !s.HasIndexHTML {
		t.Error("want HasIndexHTML=true when IndexHref is non-empty")
	}

	// An interrupted/still-running run has no FinishedAt — must stay empty, never
	// guessed from StartedAt or "now".
	if got := runSummaryFromIndexRow(runsIndexRow{RunID: "r3", StartedAt: "2026-01-01T00:00:00Z"}).FinishedAt; got != "" {
		t.Errorf("want empty FinishedAt for an unfinished run, got %q", got)
	}

	rowNoIndex := runsIndexRow{RunID: "r2"}
	if runSummaryFromIndexRow(rowNoIndex).HasIndexHTML {
		t.Error("want HasIndexHTML=false when IndexHref is empty")
	}
}

// TestWriteRunsJSON_WrapsEveryRow proves writeRunsJSON's schema envelope and that it
// carries every row buildRunsIndexRows produced — the launches-list page's one data
// source.
func TestWriteRunsJSON_WrapsEveryRow(t *testing.T) {
	runsRoot := t.TempDir()
	rows := []runsIndexRow{{RunID: "r1", LaunchID: "l1"}, {RunID: "r2", LaunchID: "l1"}}
	if err := writeRunsJSON(runsRoot, rows); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(runsRoot, "runs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file RunsFile
	if err := json.Unmarshal(b, &file); err != nil {
		t.Fatal(err)
	}
	if file.SchemaVersion != RunsFileSchemaVersion {
		t.Errorf("want schema_version=%d, got %d", RunsFileSchemaVersion, file.SchemaVersion)
	}
	if len(file.Runs) != 2 {
		t.Fatalf("want 2 runs, got %d", len(file.Runs))
	}
}

// TestWriteRunsIndex_AlsoWritesRunsJSON proves the wiring (analogous to
// TestWriteRunHTML_AlsoRegeneratesRunsIndex in index_test.go): writeRunsIndex, the
// function run/extract/report/html already call, now ALSO produces runs.json
// alongside index.html — no separate opt-in needed.
func TestWriteRunsIndex_AlsoWritesRunsJSON(t *testing.T) {
	runsRoot := t.TempDir()
	if err := writeRunsIndex(runsRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runsRoot, "runs.json")); err != nil {
		t.Fatalf("expected runs.json to be written alongside index.html: %v", err)
	}
}

// TestCmdExport_RunRegeneratesExecutionsJSON proves `harness export -run <dir>`
// actually (re)writes executions.json from a real judged.json on disk.
func TestCmdExport_RunRegeneratesExecutionsJSON(t *testing.T) {
	runDir := t.TempDir()
	jr := JudgedRun{Scenario: "fixture-scenario", Verdicts: []Verdict{{TestID: "t1", Model: "m1", ParseOK: true}}}
	if err := writeJSON(filepath.Join(runDir, "fixture-scenario.judged.json"), jr); err != nil {
		t.Fatal(err)
	}
	if err := cmdExport([]string{"-run", runDir}); err != nil {
		t.Fatalf("cmdExport -run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "executions.json")); err != nil {
		t.Fatalf("want executions.json written: %v", err)
	}
}

// TestCmdExport_RunFailsFatallyOnUnreadableJudgedJSON is the regression test for
// review amendment 5's "fatal on the first error, unlike the best-effort auto-hooks":
// a corrupt *.judged.json must make `export` itself fail, not silently skip the run
// or degrade to a warning the way writeRunHTMLBestEffort's callers do.
func TestCmdExport_RunFailsFatallyOnUnreadableJudgedJSON(t *testing.T) {
	runDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runDir, "broken.judged.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdExport([]string{"-run", runDir}); err == nil {
		t.Fatal("want cmdExport -run to fail fatally on an unreadable judged.json, got nil error")
	}
}

// TestCmdExport_AllSkipsLaunchesDirAndWritesRunsJSON proves -all iterates every run
// dir under runs/ (never the sibling launches/ directory, which holds LaunchManifest
// files with a different shape entirely) and produces the top-level runs.json.
func TestCmdExport_AllSkipsLaunchesDirAndWritesRunsJSON(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	run1 := filepath.Join("runs", "2026-01-01_00-00-00-aaaa")
	if err := os.MkdirAll(run1, 0o755); err != nil {
		t.Fatal(err)
	}
	jr := JudgedRun{Scenario: "s1", Verdicts: []Verdict{{TestID: "t1", Model: "m1", ParseOK: true}}}
	if err := writeJSON(filepath.Join(run1, "s1.judged.json"), jr); err != nil {
		t.Fatal(err)
	}

	// A launches/ dir sitting alongside real run dirs — export -all must not try to
	// treat it as a run and fail on it.
	launchesDir := filepath.Join("runs", "launches")
	if err := os.MkdirAll(launchesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(launchesDir, "l1.json"), []byte(`{"launch_id":"l1"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmdExport([]string{"-all"}); err != nil {
		t.Fatalf("cmdExport -all: %v", err)
	}
	if _, err := os.Stat(filepath.Join(run1, "executions.json")); err != nil {
		t.Errorf("want run1's executions.json written: %v", err)
	}
	if _, err := os.Stat(filepath.Join("runs", "runs.json")); err != nil {
		t.Errorf("want runs/runs.json written: %v", err)
	}
}

func TestCmdExport_AllSkipsSupportAndZeroExecutionDirs(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	for _, name := range []string{"catalog", provenance.IncompleteRunsDirName, "empty-run"} {
		if err := os.MkdirAll(filepath.Join("runs", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := cmdExport([]string{"-all"}); err != nil {
		t.Fatalf("cmdExport -all: %v", err)
	}
	for _, name := range []string{"catalog", provenance.IncompleteRunsDirName, "empty-run"} {
		if _, err := os.Stat(filepath.Join("runs", name, "index.html")); !os.IsNotExist(err) {
			t.Errorf("%s must not receive a bogus 0/0 index.html, stat err = %v", name, err)
		}
	}
}

// TestCmdExport_RunAndAllAreMutuallyExclusive guards the flag contract.
func TestCmdExport_RunAndAllAreMutuallyExclusive(t *testing.T) {
	if err := cmdExport([]string{"-run", "x", "-all"}); err == nil {
		t.Fatal("want an error when both -run and -all are passed")
	}
}

func parseGeneric(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
