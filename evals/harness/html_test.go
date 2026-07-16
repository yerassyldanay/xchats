package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xchats-evals-harness/internal/provenance"
)

// TestWriteRunHTML_BothFamilies exercises full template EXECUTION (not just parsing)
// against a scratch run dir containing a scenario judged.json, an extraction result
// with a captured input image, manifest.json, and the pre-existing Markdown reports —
// the closest thing to a real run this test suite can build without paid API calls.
func TestWriteRunHTML_BothFamilies(t *testing.T) {
	runDir := t.TempDir()

	// Scenario side: one passing verdict, one parse-failure verdict (to prove not_run
	// scores render without breaking the template) — reuses judgeOne so the fixture
	// is exactly what real code produces.
	catalog := &Catalog{Contract: "attach_groups", Tokens: []CatalogFact{
		{Token: "{{policy.main.delivery_cost}}", Value: "1 500 ₸"},
	}}
	passRow := PromptfooRow{}
	passRow.Provider.ID = "test/model"
	passRow.Response.Output = `{"reply_text":"Доставка {{policy.main.delivery_cost}}.","reply_language":"ru","attach_groups":[],"escalate":false}`
	passVerdict := judgeOne(TestCase{ID: "delivery", Requires: [][]string{{"policy.main.delivery_cost"}}}, passRow, catalog, map[string]string{"{{policy.main.delivery_cost}}": "1 500 ₸"}, nil, map[string]bool{})

	failRow := PromptfooRow{}
	failRow.Provider.ID = "test/model"
	failRow.Response.Output = "not json"
	failVerdict := judgeOne(TestCase{ID: "broken"}, failRow, catalog, map[string]string{}, nil, map[string]bool{})

	jr := JudgedRun{Scenario: "fixture-scenario", Verdicts: []Verdict{passVerdict, failVerdict}}
	if err := writeJSON(filepath.Join(runDir, "fixture-scenario.judged.json"), jr); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "SUMMARY.md"), []byte("# summary\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Extract side: one successful result with a captured input, one error result.
	outputsDir := filepath.Join(runDir, "extract_outputs")
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	okResult := extractRunResult{
		CaseID: "screenshot", Model: "test/model",
		Prompt: provenance.PromptRef{Name: "extract", Version: 1, SHA256: "abc"},
		Parsed: &ExtractionResult{ContentKind: "screenshot", Summary: "a screenshot", ExtractedText: "some <text> & \"quotes\""},
		Checks: []CheckResult{{Name: "field:content_kind", Pass: true}, {Name: "no_invented_numbers", Pass: false, Detail: "bad"}},
	}
	if err := writeJSON(filepath.Join(outputsDir, "screenshot__test_model__extract-v1.json"), okResult); err != nil {
		t.Fatal(err)
	}
	errResult := extractRunResult{CaseID: "broken-case", Model: "test/model", Error: "openrouter: http 500"}
	if err := writeJSON(filepath.Join(outputsDir, "broken-case__test_model__extract-v1.json"), errResult); err != nil {
		t.Fatal(err)
	}
	inputsDir := filepath.Join(runDir, "inputs")
	if err := os.MkdirAll(inputsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputsDir, "screenshot.jpg"), []byte{0xFF, 0xD8, 0xFF}, 0o644); err != nil {
		t.Fatal(err)
	}
	// broken-case has NO captured input — proves the "not captured" placeholder path.

	manifest := provenance.NewManifest(runDir, "2026-01-01_00-00-00-abcd", "extract", "extract", []string{"-case", "screenshot"})
	manifest.Finish()
	if err := provenance.WriteManifest(runDir, manifest); err != nil {
		t.Fatal(err)
	}

	if err := writeRunHTML(runDir); err != nil {
		t.Fatalf("writeRunHTML: %v", err)
	}

	htmlPath := filepath.Join(runDir, "index.html")
	out, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	page := string(out)

	mustContain := []string{
		"fixture-scenario",         // scenario section
		"delivery",                 // passing test id
		"broken",                   // failing test id
		"not_run",                  // not_run status class rendered somewhere
		"screenshot",               // extraction case
		"broken-case",              // extraction error case
		"input not captured",       // missing-input placeholder for broken-case
		"openrouter: http 500",     // surfaced error
		"no_invented_numbers",      // check name rendered
		"2026-01-01_00-00-00-abcd", // manifest run id doesn't appear directly, but RunID (dir name) should
	}
	for _, want := range mustContain {
		if !strings.Contains(page, want) && want != "2026-01-01_00-00-00-abcd" {
			t.Errorf("expected HTML output to contain %q, it did not", want)
		}
	}
	// The run ID shown is the directory's base name (filepath.Base(runDir)), which is
	// a random t.TempDir() path — just confirm it's present, proving RunID wiring works.
	if !strings.Contains(page, filepath.Base(runDir)) {
		t.Error("expected page to show the run ID (run dir base name)")
	}

	// Escaping: html/template must have escaped the extracted_text's angle brackets —
	// a literal "<text>" in the output would mean auto-escaping was bypassed somehow.
	if strings.Contains(page, "<text>") {
		t.Error("raw '<text>' found unescaped in HTML output — auto-escaping failed")
	}
	if !strings.Contains(page, "&lt;text&gt;") {
		t.Error("expected extracted_text's angle brackets to be HTML-escaped")
	}

	// The captured input image is linked via a relative path, not embedded/absolute.
	if !strings.Contains(page, `src="inputs/screenshot.jpg"`) {
		t.Error("expected a relative <img src> to the captured input")
	}
}

// TestWriteRunHTML_EmptyRunDir proves the viewer degrades gracefully (no panic, no
// error) for a run dir with nothing gradeable in it yet.
func TestWriteRunHTML_EmptyRunDir(t *testing.T) {
	runDir := t.TempDir()
	if err := writeRunHTML(runDir); err != nil {
		t.Fatalf("writeRunHTML on empty dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "index.html")); err != nil {
		t.Fatal(err)
	}
}

// TestWriteRunHTML_ReasoningLeakWarningRenders proves the visibility flag added for a
// raw response containing a <think> tag marker actually surfaces as a warning in the
// rendered page, not just on the in-memory VOutput — the concrete guarantee behind
// "never leaks into any report meant to show customer-facing output" for the debug raw
// panel specifically (the hard gate against reply_text itself is judge_test.go's job).
func TestWriteRunHTML_ReasoningLeakWarningRenders(t *testing.T) {
	runDir := t.TempDir()

	catalog := &Catalog{Contract: "attach_groups"}
	row := PromptfooRow{}
	row.Provider.ID = "test/model"
	row.Response.Output = "<think>internal chain of thought, never meant for a customer</think>"
	v := judgeOne(TestCase{ID: "leaky"}, row, catalog, map[string]string{}, nil, map[string]bool{})

	jr := JudgedRun{Scenario: "fixture-scenario", Verdicts: []Verdict{v}}
	if err := writeJSON(filepath.Join(runDir, "fixture-scenario.judged.json"), jr); err != nil {
		t.Fatal(err)
	}

	if err := writeRunHTML(runDir); err != nil {
		t.Fatalf("writeRunHTML: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(runDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(out)

	if !strings.Contains(page, "Raw output contains a reasoning/thinking tag marker") {
		t.Error("expected the reasoning-marker warning to render for a raw output containing <think>")
	}
	if !strings.Contains(page, "no_reasoning_leak") {
		t.Error("expected the no_reasoning_leak check name to render in the scores table")
	}
}

// TestWriteRunHTML_EvidenceDivShowsTruncatedAndReasoningLeak proves the per-verdict
// evidence block gives Truncated/ReasoningLeak the same dedicated, at-a-glance callout
// Blocked/LeftoverBraces already got — a review caught that these were only visible
// buried in the generic Scores table, unlike CONTRACT.md which already flagged them
// prominently.
func TestWriteRunHTML_EvidenceDivShowsTruncatedAndReasoningLeak(t *testing.T) {
	runDir := t.TempDir()

	catalog := &Catalog{Contract: "attach_groups"}
	row := PromptfooRow{}
	row.Provider.ID = "test/model"
	row.Response.Output = `{"reply_text":"ok","reply_language":"ru","attach_groups":[],"escalate":false}`
	row.Response.FinishReason = "length"
	v := judgeOne(TestCase{ID: "truncated-case"}, row, catalog, map[string]string{}, nil, map[string]bool{})
	if !v.Truncated {
		t.Fatal("precondition failed: want Truncated=true")
	}

	jr := JudgedRun{Scenario: "fixture-scenario", Verdicts: []Verdict{v}}
	if err := writeJSON(filepath.Join(runDir, "fixture-scenario.judged.json"), jr); err != nil {
		t.Fatal(err)
	}
	if err := writeRunHTML(runDir); err != nil {
		t.Fatalf("writeRunHTML: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(runDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(out)

	if !strings.Contains(page, "<b>TRUNCATED</b>") {
		t.Error("expected a dedicated TRUNCATED evidence callout, not just a Scores table row")
	}
	if !strings.Contains(page, "finish_reason=length") {
		t.Error("expected the evidence callout to name the actual finish_reason value")
	}
}

// TestWriteRunHTML_ReasoningContentRendersSeparatelyFromRawOutput proves captured
// reasoning content (Verdict.Reasoning) is actually visible in the viewer, not silently
// dropped — a review caught that PromptfooRow.Response.Reasoning was parsed but never
// forwarded anywhere downstream once captured.
func TestWriteRunHTML_ReasoningContentRendersSeparatelyFromRawOutput(t *testing.T) {
	runDir := t.TempDir()

	catalog := &Catalog{Contract: "attach_groups"}
	row := PromptfooRow{}
	row.Provider.ID = "test/model"
	row.Response.Output = `{"reply_text":"ok","reply_language":"ru","attach_groups":[],"escalate":false}`
	row.Response.Reasoning = "the customer wants the price, I should state it plainly"
	v := judgeOne(TestCase{ID: "reasoning-case"}, row, catalog, map[string]string{}, nil, map[string]bool{})
	if v.Reasoning == "" {
		t.Fatal("precondition failed: want Verdict.Reasoning populated from row.Response.Reasoning")
	}

	jr := JudgedRun{Scenario: "fixture-scenario", Verdicts: []Verdict{v}}
	if err := writeJSON(filepath.Join(runDir, "fixture-scenario.judged.json"), jr); err != nil {
		t.Fatal(err)
	}
	if err := writeRunHTML(runDir); err != nil {
		t.Fatalf("writeRunHTML: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(runDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(out)

	if !strings.Contains(page, "the customer wants the price, I should state it plainly") {
		t.Error("expected captured reasoning content to actually render in the viewer")
	}
	if !strings.Contains(page, "never part of the graded reply_text") {
		t.Error("expected the reasoning block to be clearly labeled as separate from the graded output")
	}
}
