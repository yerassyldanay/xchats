package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReviewContentHash_NoCollisionAcrossFieldBoundaries is the regression test for a
// hash-collision review caught: a raw NUL byte inside a Message can shift where a
// naive delimiter-joined encoding thinks the Message/ReplyText boundary falls. Row set
// A's Message ends where row set B's Message stops one field early — under the old
// scheme (join with a literal NUL, "id\x00msg\x00reply") these two row sets produced the
// IDENTICAL byte string, hence the identical hash, even though they're genuinely
// different content. The current JSON-based encoding must tell them apart.
func TestReviewContentHash_NoCollisionAcrossFieldBoundaries(t *testing.T) {
	a := []BlindReviewRow{{OpaqueID: "R1", Message: "foo\x00bar", ReplyText: "baz"}}
	b := []BlindReviewRow{{OpaqueID: "R1", Message: "foo", ReplyText: "bar\x00baz"}}

	hashA := reviewContentHash(a)
	hashB := reviewContentHash(b)
	if hashA == hashB {
		t.Fatalf("row sets with a NUL byte straddling the Message/ReplyText boundary hashed identically (%s) — this is the exact collision a raw-byte-separator encoding was vulnerable to", hashA)
	}
}

// TestReviewContentHash_DeterministicAndOrderSensitive pins the two properties
// blind-report's mismatch check depends on: same input always hashes the same, and
// reordering rows (a plausible hand-edit) changes the hash rather than being ignored.
func TestReviewContentHash_DeterministicAndOrderSensitive(t *testing.T) {
	rows := []BlindReviewRow{
		{OpaqueID: "R1", Message: "Сколько стоит кофемашина?", ReplyText: "129 900 ₸."},
		{OpaqueID: "R2", Message: "Кофемашина DeLonghi қанша тұрады?", ReplyText: "129 900 ₸."},
	}
	if reviewContentHash(rows) != reviewContentHash(rows) {
		t.Fatal("want reviewContentHash to be deterministic for identical input")
	}
	reordered := []BlindReviewRow{rows[1], rows[0]}
	if reviewContentHash(rows) == reviewContentHash(reordered) {
		t.Fatal("want a reordered row set to hash differently")
	}
}

// TestReadBlindReviewCSV_StripsLeadingBOM is the regression test for a real-world gotcha
// review caught: Excel's "CSV UTF-8 (Comma delimited)" save option — a natural choice for
// a reviewer editing Cyrillic text — prepends a UTF-8 byte-order mark, which
// encoding/csv does not strip on its own. Without stripping it here, a review.csv saved
// this way would fail the header check with a confusing "hand-edited beyond the label
// column" error, even though the reviewer only touched the label column.
func TestReadBlindReviewCSV_StripsLeadingBOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.csv")
	content := append(append([]byte{}, utf8BOM...), []byte("opaque_id,message,reply_text,label\nR1,Сколько стоит кофемашина?,129 900 ₸.,ru\n")...)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := readBlindReviewCSV(path)
	if err != nil {
		t.Fatalf("readBlindReviewCSV should tolerate a leading BOM, got: %v", err)
	}
	if len(rows) != 1 || rows[0].OpaqueID != "R1" || rows[0].Label != "ru" {
		t.Fatalf("want 1 row (R1, label=ru), got %+v", rows)
	}
}

func TestComputeRoutingAccuracy(t *testing.T) {
	tests := []TestCase{
		{ID: "1", Message: "Кофемашина DeLonghi қанша тұрады?", Language: "kk"},
		{ID: "2", Message: "Сколько стоит кофемашина?", Language: "ru"},
		// No Language annotation at all — must be skipped, not silently counted either way.
		{ID: "3", Message: "Здравствуйте!"},
		// A message detectLang would get WRONG relative to this (deliberately mismatched)
		// Language annotation, to prove mismatches are actually detected.
		{ID: "4", Message: "Сколько стоит кофемашина?", Language: "kk"},
	}
	report := computeRoutingAccuracy(tests)

	if report.SkippedNoLanguage != 1 {
		t.Errorf("want 1 skipped (no language annotation), got %d", report.SkippedNoLanguage)
	}
	if report.Total != 3 {
		t.Errorf("want 3 total (annotated tests only), got %d", report.Total)
	}
	if report.Correct != 2 {
		t.Errorf("want 2 correct, got %d", report.Correct)
	}
	if len(report.Mismatches) != 1 || report.Mismatches[0].TestID != "4" {
		t.Fatalf("want exactly 1 mismatch on test 4, got %+v", report.Mismatches)
	}
	if report.Mismatches[0].Got != "ru" || report.Mismatches[0].Want != "kk" {
		t.Errorf("want mismatch got=ru want=kk, got %+v", report.Mismatches[0])
	}
}

// buildBlindFixtureRun writes a run dir with two judged.json files (two scenarios) and
// their resolved_tests.json snapshots, mixing ContractPass and non-ContractPass verdicts
// across two different models — realistic enough to exercise export's filtering,
// shuffling, and the routing-accuracy snapshot lookup all at once. Built via the real
// judgeOne, not hand-mocked Verdicts, matching this codebase's existing test style.
func buildBlindFixtureRun(t *testing.T) (runDir string) {
	t.Helper()
	runDir = t.TempDir()

	catalog := &Catalog{Contract: "attach_groups", Tokens: []CatalogFact{
		{Token: "{{product.coffee-machine.price}}", Value: "129 900 ₸"},
	}}
	tokenValue := map[string]string{"{{product.coffee-machine.price}}": "129 900 ₸"}

	kkTC := TestCase{ID: "kk-price", Message: "Кофемашина DeLonghi қанша тұрады?", Language: "kk",
		Requires: [][]string{{"product.coffee-machine.price"}}}
	ruTC := TestCase{ID: "ru-price", Message: "Сколько стоит кофемашина?", Language: "ru",
		Requires: [][]string{{"product.coffee-machine.price"}}}

	rowA1 := PromptfooRow{}
	rowA1.Provider.ID = "openrouter:model-a"
	rowA1.Response.Output = `{"reply_text":"Кофемашина {{product.coffee-machine.price}}.","reply_language":"kk","attach_groups":[],"escalate":false}`
	vA1 := judgeOne(kkTC, rowA1, catalog, tokenValue, nil, map[string]bool{})

	rowA2 := PromptfooRow{}
	rowA2.Provider.ID = "openrouter:model-a"
	rowA2.Response.Output = `not json` // -> ContractPass=false, must be excluded from export
	vA2 := judgeOne(ruTC, rowA2, catalog, tokenValue, nil, map[string]bool{})

	rowB1 := PromptfooRow{}
	rowB1.Provider.ID = "openrouter:model-b"
	rowB1.Response.Output = `{"reply_text":"Кофемашина {{product.coffee-machine.price}}.","reply_language":"ru","attach_groups":[],"escalate":false}`
	vB1 := judgeOne(ruTC, rowB1, catalog, tokenValue, nil, map[string]bool{})

	if !vA1.ContractPass || vA2.ContractPass || !vB1.ContractPass {
		t.Fatalf("fixture precondition failed: vA1.ContractPass=%v vA2.ContractPass=%v vB1.ContractPass=%v", vA1.ContractPass, vA2.ContractPass, vB1.ContractPass)
	}

	jr := JudgedRun{Scenario: "fixture-scenario", Verdicts: []Verdict{vA1, vA2, vB1}}
	if err := writeJSON(filepath.Join(runDir, "fixture-scenario.judged.json"), jr); err != nil {
		t.Fatal(err)
	}

	// Snapshot the resolved test set (mirrors provenance.SnapshotScenario's real shape)
	// so writeRoutingAccuracyReport has something to read.
	snapDir := filepath.Join(runDir, "snapshots", "fixture-scenario")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved := ResolvedTests{Tests: []TestCase{kkTC, ruTC}}
	if err := writeJSON(filepath.Join(snapDir, "resolved_tests.json"), resolved); err != nil {
		t.Fatal(err)
	}

	return runDir
}

func TestCmdBlindExport_ExcludesNonContractPassAndNeverLeaksIdentity(t *testing.T) {
	runDir := buildBlindFixtureRun(t)
	outDir := filepath.Join(t.TempDir(), "review")

	if err := cmdBlindExport([]string{"-run", runDir, "-out", outDir, "-seed", "42"}); err != nil {
		t.Fatalf("cmdBlindExport: %v", err)
	}

	reviewBytes, err := os.ReadFile(filepath.Join(outDir, "review.csv"))
	if err != nil {
		t.Fatal(err)
	}
	review := string(reviewBytes)

	// Exactly 2 rows (vA1, vB1) — vA2 (ContractPass=false) must be excluded.
	rows, err := readBlindReviewCSV(filepath.Join(outDir, "review.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 exported rows (1 excluded for ContractPass=false), got %d:\n%s", len(rows), review)
	}

	// The reviewer-facing file must never mention the model id, the scenario name, or
	// the declared reply_language field — only message + reply text + a blank label.
	for _, leak := range []string{"model-a", "model-b", "fixture-scenario", "reply_language"} {
		if strings.Contains(review, leak) {
			t.Errorf("review.csv leaks identity: found %q in:\n%s", leak, review)
		}
	}
	for _, r := range rows {
		if r.Label != "" {
			t.Errorf("want a blank label column on export, row %s has %q", r.OpaqueID, r.Label)
		}
		if r.OpaqueID == "" {
			t.Error("want every row to have an opaque id")
		}
	}

	// The withheld mapping file DOES carry model/scenario/declared language, keyed by
	// the same opaque ids.
	var mapping BlindMappingFile
	if err := readJSON(filepath.Join(outDir, "mapping.DO-NOT-SHARE-WITH-REVIEWER.json"), &mapping); err != nil {
		t.Fatal(err)
	}
	if len(mapping.Entries) != 2 {
		t.Fatalf("want 2 mapping entries, got %d", len(mapping.Entries))
	}
	if mapping.Excluded != 1 {
		t.Errorf("want Excluded=1, got %d", mapping.Excluded)
	}
	gotModels := map[string]bool{}
	for _, e := range mapping.Entries {
		gotModels[e.Model] = true
		if e.DeclaredReplyLanguage == "" {
			t.Errorf("want a non-empty declared reply_language for entry %+v", e)
		}
	}
	if !gotModels["openrouter:model-a"] || !gotModels["openrouter:model-b"] {
		t.Errorf("want both models represented in the mapping, got %+v", gotModels)
	}

	// ROUTING_ACCURACY.md must exist and need no human input — both fixture tests are
	// annotated with a Language detectLang agrees with (see the fixture's IDs).
	routing, err := os.ReadFile(filepath.Join(outDir, "ROUTING_ACCURACY.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(routing), "2/2 (100%) correct") {
		t.Errorf("want 2/2 correct routing (both fixture tests route as their own Language says), got:\n%s", routing)
	}
}

func TestCmdBlindExport_RefusesToOverwriteWithoutForce(t *testing.T) {
	runDir := buildBlindFixtureRun(t)
	outDir := filepath.Join(t.TempDir(), "review")

	if err := cmdBlindExport([]string{"-run", runDir, "-out", outDir, "-seed", "1"}); err != nil {
		t.Fatalf("first export: %v", err)
	}
	if err := cmdBlindExport([]string{"-run", runDir, "-out", outDir, "-seed", "2"}); err == nil {
		t.Fatal("want the second export (same -out, no -force) to refuse to overwrite")
	}
	if err := cmdBlindExport([]string{"-run", runDir, "-out", outDir, "-seed", "2", "-force"}); err != nil {
		t.Fatalf("-force should allow overwriting: %v", err)
	}
}

func TestCmdBlindReport_DeclaredVsBlindedAgreementAndCompleteness(t *testing.T) {
	runDir := buildBlindFixtureRun(t)
	outDir := filepath.Join(t.TempDir(), "review")
	if err := cmdBlindExport([]string{"-run", runDir, "-out", outDir, "-seed", "7"}); err != nil {
		t.Fatalf("cmdBlindExport: %v", err)
	}

	rows, err := readBlindReviewCSV(filepath.Join(outDir, "review.csv"))
	if err != nil {
		t.Fatal(err)
	}
	var mapping BlindMappingFile
	if err := readJSON(filepath.Join(outDir, "mapping.DO-NOT-SHARE-WITH-REVIEWER.json"), &mapping); err != nil {
		t.Fatal(err)
	}
	byID := map[string]BlindMappingEntry{}
	for _, e := range mapping.Entries {
		byID[e.OpaqueID] = e
	}

	// Simulate a human reviewer: label one row correctly (agreeing with the declared
	// reply_language), leave the other UNLABELED (not yet reviewed).
	labeledCount := 0
	for i := range rows {
		entry := byID[rows[i].OpaqueID]
		if labeledCount == 0 {
			rows[i].Label = entry.DeclaredReplyLanguage // agrees
			labeledCount++
		}
		// the other row's Label stays "" — simulating an incomplete review
	}
	if err := writeBlindReviewCSV(filepath.Join(outDir, "review.csv"), rows); err != nil {
		t.Fatal(err)
	}

	if err := cmdBlindReport([]string{
		"-review", filepath.Join(outDir, "review.csv"),
		"-mapping", filepath.Join(outDir, "mapping.DO-NOT-SHARE-WITH-REVIEWER.json"),
	}); err != nil {
		t.Fatalf("cmdBlindReport: %v", err)
	}

	report, err := os.ReadFile(filepath.Join(outDir, "BLIND_REPORT.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(report)
	if !strings.Contains(content, "1 of 2 rows labeled (50%)") {
		t.Errorf("want completeness to report 1 of 2 labeled, got:\n%s", content)
	}
	if !strings.Contains(content, "1/1 (100%) agree") {
		t.Errorf("want 1/1 agreement (the one labeled row agreed by construction), got:\n%s", content)
	}
}

func TestCmdBlindReport_MismatchedPairIsRejected(t *testing.T) {
	runDir := buildBlindFixtureRun(t)
	outDir := filepath.Join(t.TempDir(), "review")
	if err := cmdBlindExport([]string{"-run", runDir, "-out", outDir, "-seed", "3"}); err != nil {
		t.Fatalf("cmdBlindExport: %v", err)
	}

	// A mapping file from a completely different (empty) export — mismatched on purpose.
	otherOut := filepath.Join(t.TempDir(), "other")
	otherRunDir := buildBlindFixtureRun(t)
	if err := cmdBlindExport([]string{"-run", otherRunDir, "-out", otherOut, "-seed", "9"}); err != nil {
		t.Fatalf("cmdBlindExport (other): %v", err)
	}

	err := cmdBlindReport([]string{
		"-review", filepath.Join(outDir, "review.csv"),
		"-mapping", filepath.Join(otherOut, "mapping.DO-NOT-SHARE-WITH-REVIEWER.json"),
	})
	if err == nil {
		t.Fatal("want blind-report to reject a review.csv paired with a mapping file from a DIFFERENT export")
	}
}
