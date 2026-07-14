package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xchats-evals-harness/internal/provenance"
)

// TestWriteRunsIndex_MultipleRuns builds two scratch run dirs (one extraction, one
// scenario, one with a manifest, one legacy without) and proves the index page lists
// both, newest-first, with per-family stats and correct relative links.
func TestWriteRunsIndex_MultipleRuns(t *testing.T) {
	runsRoot := t.TempDir()

	// Run A: extraction, WITH manifest.
	runA := filepath.Join(runsRoot, "2026-01-01_00-00-00-aaaa")
	outputsA := filepath.Join(runA, "extract_outputs")
	if err := os.MkdirAll(outputsA, 0o755); err != nil {
		t.Fatal(err)
	}
	okResult := extractRunResult{
		CaseID: "c1", Model: "test/model",
		Prompt: provenance.PromptRef{Name: "extract", Version: 1},
		Parsed: &ExtractionResult{ContentKind: "other"},
		Checks: []CheckResult{{Name: "field:content_kind", Pass: true}},
	}
	if err := writeJSON(filepath.Join(outputsA, "c1__test_model__extract-v1.json"), okResult); err != nil {
		t.Fatal(err)
	}
	mA := provenance.NewManifest(runA, filepath.Base(runA), "extract", "extract", nil)
	mA.Finish()
	if err := provenance.WriteManifest(runA, mA); err != nil {
		t.Fatal(err)
	}
	if err := writeRunHTML(runA); err != nil {
		t.Fatal(err)
	}

	// Run B: scenario, NO manifest (legacy).
	runB := filepath.Join(runsRoot, "2026-01-02_00-00-00-bbbb")
	if err := os.MkdirAll(runB, 0o755); err != nil {
		t.Fatal(err)
	}
	jr := JudgedRun{Scenario: "shop-current", Verdicts: []Verdict{
		{TestID: "t1", Model: "test/model", ParseOK: true, ContractPass: true, ModelBehaviorPass: false},
	}}
	if err := writeJSON(filepath.Join(runB, "shop-current.judged.json"), jr); err != nil {
		t.Fatal(err)
	}
	// No writeRunHTML call for B — proves the index still lists a run with no per-run
	// index.html of its own (IndexHref stays empty, plain text instead of a link).

	if err := writeRunsIndex(runsRoot); err != nil {
		t.Fatalf("writeRunsIndex: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runsRoot, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(out)

	idxA := strings.Index(page, "2026-01-01_00-00-00-aaaa")
	idxB := strings.Index(page, "2026-01-02_00-00-00-bbbb")
	if idxA == -1 || idxB == -1 {
		t.Fatalf("expected both run ids present, got idxA=%d idxB=%d", idxA, idxB)
	}
	if idxB > idxA {
		t.Errorf("want newest-first order (B is newer than A), but A appeared before B in the page")
	}
	if !strings.Contains(page, `href="2026-01-01_00-00-00-aaaa/index.html"`) {
		t.Error("expected run A to link to its own index.html")
	}
	if strings.Contains(page, `href="2026-01-02_00-00-00-bbbb/index.html"`) {
		t.Error("run B has no index.html of its own — must not link to one that doesn't exist")
	}
	if !strings.Contains(page, "(legacy)") {
		t.Error("expected run B (no manifest) to be marked legacy")
	}
	if !strings.Contains(page, "test/model") {
		t.Error("expected model id to appear")
	}
	if !strings.Contains(page, "extract@v1") {
		t.Error("expected prompt ref to appear for the extraction run")
	}
}

// TestWriteRunsIndex_EmptyRunsDir proves a runs/ dir that doesn't exist yet (fresh
// checkout, nothing run) doesn't error.
func TestWriteRunsIndex_EmptyRunsDir(t *testing.T) {
	runsRoot := filepath.Join(t.TempDir(), "does-not-exist-yet")
	rows, err := buildRunsIndexRows(runsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("want 0 rows, got %d", len(rows))
	}
}

// TestBuildRunsIndexRows_SkipsLaunchesDir is the regression test for a real bug
// caught by hand during manual verification: runs/launches/ (LaunchManifest files,
// internal/provenance/launch.go) sits alongside real run dirs under the same
// runsRoot, and buildRunsIndexRows must never treat it as a run — it has no
// *.judged.json/extract_outputs/manifest.json of its own, so doing so produced a
// bogus "launches" row with family="unknown" in the real runs.json output.
func TestBuildRunsIndexRows_SkipsLaunchesDir(t *testing.T) {
	runsRoot := t.TempDir()
	realRun := filepath.Join(runsRoot, "2026-01-01_00-00-00-aaaa")
	if err := os.MkdirAll(realRun, 0o755); err != nil {
		t.Fatal(err)
	}
	jr := JudgedRun{Scenario: "s1", Verdicts: []Verdict{{TestID: "t1", Model: "m1", ParseOK: true}}}
	if err := writeJSON(filepath.Join(realRun, "s1.judged.json"), jr); err != nil {
		t.Fatal(err)
	}

	launchesDir := filepath.Join(runsRoot, "launches")
	if err := os.MkdirAll(launchesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(launchesDir, "l1.json"), []byte(`{"launch_id":"l1"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := buildRunsIndexRows(runsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want exactly 1 row (the real run, launches/ excluded), got %d: %+v", len(rows), rows)
	}
	if rows[0].RunID != "2026-01-01_00-00-00-aaaa" {
		t.Errorf("want the real run's row, got %+v", rows[0])
	}
}

// TestWriteRunHTML_AlsoRegeneratesRunsIndex proves the wiring: calling writeRunHTML
// for one run must rebuild the SIBLING runs/index.html too (step 6's whole point —
// "regenerated wholesale on every run/html invocation").
func TestWriteRunHTML_AlsoRegeneratesRunsIndex(t *testing.T) {
	runsRoot := t.TempDir()
	runDir := filepath.Join(runsRoot, "2026-01-01_00-00-00-cccc")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeRunHTML(runDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runsRoot, "index.html")); err != nil {
		t.Fatalf("expected runs/index.html to be regenerated as a side effect: %v", err)
	}
}
