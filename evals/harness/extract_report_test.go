package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xchats-evals-harness/internal/provenance"
)

// TestWriteExtractReport_DistinguishesPromptVersions is Phase 0.3's required test: the
// same case and model, once under extract@v1 (failing) and once under extract@v2
// (passing), must produce two clearly distinguishable rows in EXTRACT.md — not
// indistinguishable duplicates a reader could misattribute to the wrong prompt.
func TestWriteExtractReport_DistinguishesPromptVersions(t *testing.T) {
	v1 := provenance.PromptRef{Name: "extract", Version: 1, SHA256: "aaa"}
	v2 := provenance.PromptRef{Name: "extract", Version: 2, SHA256: "bbb"}

	results := []extractRunResult{
		{
			CaseID: "how-to-cachier", Model: "google/gemini-2.5-flash", Prompt: v1,
			Parsed: &ExtractionResult{ContentKind: "tutorial", ExtractedText: "partial"},
			Checks: []CheckResult{
				{Name: "field:content_kind", Pass: true},
				{Name: "text_contains_all", Pass: false, Detail: "missing: 77/7"},
			},
			Usage: orUsage{PromptTokens: 1000, CompletionTokens: 200},
		},
		{
			CaseID: "how-to-cachier", Model: "google/gemini-2.5-flash", Prompt: v2,
			Parsed: &ExtractionResult{ContentKind: "tutorial", ExtractedText: "complete, incl. 77/7"},
			Checks: []CheckResult{
				{Name: "field:content_kind", Pass: true},
				{Name: "text_contains_all", Pass: true},
			},
			Usage: orUsage{PromptTokens: 1000, CompletionTokens: 260},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "EXTRACT.md")
	if err := writeExtractReport(path, results); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	report := string(out)

	// Per-case table rows must each show their own prompt version.
	if !strings.Contains(report, "extract@v1") || !strings.Contains(report, "extract@v2") {
		t.Fatalf("expected both prompt versions to appear in the report:\n%s", report)
	}

	// The failure heading must name model AND prompt — otherwise two failure blocks for
	// the same model (one per prompt) would print under identical, indistinguishable
	// headings.
	if !strings.Contains(report, "google/gemini-2.5-flash / extract@v1 failures:") {
		t.Errorf("expected a v1-specific failure heading, got:\n%s", report)
	}
	if strings.Contains(report, "google/gemini-2.5-flash / extract@v2 failures:") {
		t.Error("v2 passed every check — it must not have a failures block at all")
	}

	// Aggregate table: v1's row and v2's row must show DIFFERENT full-pass fractions
	// (0/1 vs 1/1) — the exact scenario this test exists to catch: it must be impossible
	// to confuse the two prompt versions' numbers.
	if !strings.Contains(report, "| extract@v1 | google/gemini-2.5-flash | 0/1") {
		t.Errorf("expected v1's aggregate row to show 0/1 full pass, got:\n%s", report)
	}
	if !strings.Contains(report, "| extract@v2 | google/gemini-2.5-flash | 1/1") {
		t.Errorf("expected v2's aggregate row to show 1/1 full pass, got:\n%s", report)
	}
}

// TestBuildExtractAggregate_SeparatesErrorsFromGradedChecks proves an error/parse-failure
// result contributes to Total but never silently counts as a passed or failed check.
func TestBuildExtractAggregate_SeparatesErrorsFromGradedChecks(t *testing.T) {
	ref := provenance.PromptRef{Name: "extract", Version: 1}
	results := []extractRunResult{
		{CaseID: "c1", Model: "m1", Prompt: ref, Error: "http 500"},
		{CaseID: "c2", Model: "m1", Prompt: ref, Parsed: &ExtractionResult{}, Checks: []CheckResult{{Name: "field:x", Pass: true}}},
	}
	out := buildExtractAggregate(results)
	if !strings.Contains(out, "1/1 (100%)") {
		t.Errorf("want the one graded (non-error) result to show 1/1 full pass, got:\n%s", out)
	}
	if !strings.Contains(out, "1 error/parse-fail") {
		t.Errorf("want the error to be called out separately, not folded into the pass fraction, got:\n%s", out)
	}
}
