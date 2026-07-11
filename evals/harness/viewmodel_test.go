package main

import (
	"os"
	"path/filepath"
	"testing"

	"xchats-evals-harness/internal/provenance"
)

func scoreByName(scores []VScore, name string) (VScore, bool) {
	for _, s := range scores {
		if s.Name == name {
			return s, true
		}
	}
	return VScore{}, false
}

// TestScenarioExecutionFromVerdict_ParseFailure_EverythingDownstreamIsNotRun is the
// golden test for the exact bug an earlier review caught: a hand-built Verdict where
// only ParseOK/Reason are set (mirrors judgeOne's early return on an unparseable
// model answer) must produce parse_ok=fail and EVERY other score=not_run — never
// "fail", since none of those checks ever executed.
func TestScenarioExecutionFromVerdict_ParseFailure_EverythingDownstreamIsNotRun(t *testing.T) {
	v := Verdict{
		TestID:    "t1",
		Model:     "test/model",
		RawOutput: "not json",
		ParseOK:   false,
		Reason:    "could not parse JSON output",
		// Every other field left at zero value — exactly what judgeOne produces on its
		// early return (judge.go: `if !ok { v.Reason = ...; return v }`).
	}
	exec := scenarioExecutionFromVerdict("fixture-scenario", v)

	parseOk, ok := scoreByName(exec.Scores, "parse_ok")
	if !ok || parseOk.Status != ScoreFail {
		t.Fatalf("want parse_ok=fail, got %+v (found=%v)", parseOk, ok)
	}
	if parseOk.Detail != "could not parse JSON output" {
		t.Errorf("want parse_ok detail to carry the reason, got %q", parseOk.Detail)
	}

	downstream := []string{
		"contract_fields", "no_unknown_tokens", "no_leftover_braces", "requires",
		"media", "escalate", "language", "must_not_contain", "no_invented_digits",
		"no_unit_issues", "no_unknown_media",
	}
	for _, name := range downstream {
		s, ok := scoreByName(exec.Scores, name)
		if !ok {
			t.Errorf("score %q missing entirely", name)
			continue
		}
		if s.Status != ScoreNotRun {
			t.Errorf("score %q: want not_run (never evaluated — judgeOne returned before reaching it), got %s", name, s.Status)
		}
		if s.Status == ScoreFail {
			t.Errorf("score %q: reported as FAIL when it was never evaluated — this is the exact bug this test guards against", name)
		}
	}

	// Rollups ARE legitimately false here (they're defined to require ParseOK), not
	// not_run — a parse failure genuinely means the contract did not pass.
	for _, r := range exec.Rollups {
		if r.Pass {
			t.Errorf("rollup %q: want false on a parse failure, got true", r.Key)
		}
	}
}

// TestScenarioExecutionFromVerdict_ParseOK_EveryScoreReflectsRealEvaluation proves the
// complementary case: once parsing succeeds, EVERY score — even one whose gate
// (contract_fields) itself failed — reports a real pass/fail, never not_run. judgeOne
// has no second early return; contract_fields failing does not skip the
// model-behavior checks that run after it.
func TestScenarioExecutionFromVerdict_ParseOK_EveryScoreReflectsRealEvaluation(t *testing.T) {
	v := Verdict{
		TestID:             "t2",
		Model:              "test/model",
		ParseOK:            true,
		ContractFields:     false, // e.g. missing reply_text — but judgeOne kept going anyway
		RequiresPass:       false,
		MediaPass:          true,
		EscalatePass:       true,
		LanguagePass:       true,
		MustNotContainPass: true,
		ModelBehaviorPass:  false,
		ContractPass:       false,
	}
	exec := scenarioExecutionFromVerdict("fixture-scenario", v)

	for _, name := range []string{"contract_fields", "no_unknown_tokens", "no_leftover_braces",
		"requires", "media", "escalate", "language", "must_not_contain",
		"no_invented_digits", "no_unit_issues", "no_unknown_media"} {
		s, ok := scoreByName(exec.Scores, name)
		if !ok {
			t.Fatalf("score %q missing", name)
		}
		if s.Status == ScoreNotRun {
			t.Errorf("score %q: want a real pass/fail (judgeOne evaluated it even though contract_fields failed), got not_run", name)
		}
	}
	if cf, _ := scoreByName(exec.Scores, "contract_fields"); cf.Status != ScoreFail {
		t.Errorf("want contract_fields=fail, got %s", cf.Status)
	}
	if req, _ := scoreByName(exec.Scores, "requires"); req.Status != ScoreFail {
		t.Errorf("want requires=fail, got %s", req.Status)
	}
	if med, _ := scoreByName(exec.Scores, "media"); med.Status != ScorePass {
		t.Errorf("want media=pass, got %s", med.Status)
	}
}

// TestExtractExecutionFromResult_Error_NoFabricatedScores proves the extraction
// adapter's honesty rule: an HTTP/network error means Checks was never computed, and
// there is no fixed check-name list to backfill (unlike the scenario family) — so
// Scores must be empty, with the failure surfaced via Output.Error instead of fake
// per-check rows.
func TestExtractExecutionFromResult_Error_NoFabricatedScores(t *testing.T) {
	r := extractRunResult{CaseID: "c1", Model: "test/model", Error: "openrouter: http 500: boom"}
	exec := extractExecutionFromResult(r)

	if len(exec.Scores) != 0 {
		t.Errorf("want zero scores on a call error, got %+v", exec.Scores)
	}
	if exec.Output.Error != "openrouter: http 500: boom" {
		t.Errorf("want Output.Error to carry the error, got %q", exec.Output.Error)
	}
	if exec.Rollups[0].Pass {
		t.Error("want all_checks_pass=false on an error result")
	}
	if exec.Extract != nil {
		t.Error("want nil Extract details when nothing was parsed")
	}
}

// TestExtractExecutionFromResult_ParseFailure_NoFabricatedScores mirrors the error
// case for a parse failure that survived every retry.
func TestExtractExecutionFromResult_ParseFailure_NoFabricatedScores(t *testing.T) {
	r := extractRunResult{CaseID: "c1", Model: "test/model", Raw: "not json", ParseError: "unexpected end of JSON input"}
	exec := extractExecutionFromResult(r)

	if len(exec.Scores) != 0 {
		t.Errorf("want zero scores on a parse failure, got %+v", exec.Scores)
	}
	if exec.Output.ParseError != "unexpected end of JSON input" {
		t.Errorf("want Output.ParseError set, got %q", exec.Output.ParseError)
	}
	if exec.Output.ParseOK {
		t.Error("want ParseOK=false")
	}
	if exec.Rollups[0].Pass {
		t.Error("want all_checks_pass=false on a parse failure")
	}
}

// TestExtractExecutionFromResult_Success_ChecksMap1to1 proves the success path: every
// CheckResult becomes exactly one VScore with a real (never not_run) status.
func TestExtractExecutionFromResult_Success_ChecksMap1to1(t *testing.T) {
	r := extractRunResult{
		CaseID: "c1",
		Model:  "test/model",
		Prompt: provenance.PromptRef{Name: "extract", Version: 1, SHA256: "abc"},
		Parsed: &ExtractionResult{ContentKind: "product_photo", Summary: "a drill"},
		Checks: []CheckResult{
			{Name: "field:content_kind", Pass: true},
			{Name: "no_invented_numbers", Pass: false, Detail: "not in the allowed list: 999"},
		},
	}
	exec := extractExecutionFromResult(r)

	if len(exec.Scores) != 2 {
		t.Fatalf("want 2 scores, got %d: %+v", len(exec.Scores), exec.Scores)
	}
	pass, _ := scoreByName(exec.Scores, "field:content_kind")
	if pass.Status != ScorePass {
		t.Errorf("want field:content_kind=pass, got %s", pass.Status)
	}
	fail, _ := scoreByName(exec.Scores, "no_invented_numbers")
	if fail.Status != ScoreFail || fail.Detail != "not in the allowed list: 999" {
		t.Errorf("want no_invented_numbers=fail with detail, got %+v", fail)
	}
	if exec.Extract == nil || exec.Extract.ContentKind != "product_photo" {
		t.Fatalf("want Extract details populated, got %+v", exec.Extract)
	}
	if exec.Variant.Prompt.Name != "extract" || exec.Variant.Prompt.Version != 1 {
		t.Errorf("want variant prompt ref carried through, got %+v", exec.Variant.Prompt)
	}
	if exec.Subject.InputRef != filepath.Join("inputs", "c1.jpg") {
		t.Errorf("want input_ref inputs/c1.jpg, got %s", exec.Subject.InputRef)
	}
}

// TestLoadRunExecutions_CombinesBothFamilies uses the actual run-loading path (glob +
// readJSON) against a scratch run dir containing both a judged.json and an
// extract_outputs file, proving loadRunExecutions merges both families correctly —
// nothing here is persisted beyond the pre-existing legacy files.
func TestLoadRunExecutions_CombinesBothFamilies(t *testing.T) {
	runDir := t.TempDir()
	jr := JudgedRun{Scenario: "shop-current", Verdicts: []Verdict{
		{TestID: "t1", Model: "m1", ParseOK: true, ContractPass: true, ModelBehaviorPass: true},
	}}
	if err := writeJSON(filepath.Join(runDir, "shop-current.judged.json"), jr); err != nil {
		t.Fatal(err)
	}
	outputsDir := filepath.Join(runDir, "extract_outputs")
	if err := os.MkdirAll(outputsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	r := extractRunResult{CaseID: "c1", Model: "m1", Parsed: &ExtractionResult{ContentKind: "other"}}
	if err := writeJSON(filepath.Join(outputsDir, "c1__m1__extract-v1.json"), r); err != nil {
		t.Fatal(err)
	}

	execs, err := loadRunExecutions(runDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(execs) != 2 {
		t.Fatalf("want 2 executions, got %d", len(execs))
	}
	var families []string
	for _, e := range execs {
		families = append(families, e.Family)
	}
	if families[0] != "scenario" || families[1] != "extract" {
		t.Fatalf("want [scenario, extract], got %v", families)
	}
}

// TestScenarioExecutionFromVerdict_RealJudgeOne_ContractFieldsFailStillEvaluatesBehavior
// cross-checks the adapter against the REAL judgeOne (judge.go), not a hand-built
// Verdict — using the same "wrong typed fields" case judge_test.go's
// TestJudgeOne_DeterministicChecks already exercises (ContractPass=false,
// ModelBehaviorPass=true). Confirms the adapter's not_run logic agrees with actual
// production code, not just a model of it.
func TestScenarioExecutionFromVerdict_RealJudgeOne_ContractFieldsFailStillEvaluatesBehavior(t *testing.T) {
	catalog := &Catalog{Contract: "attach_groups"}
	row := PromptfooRow{}
	row.Provider.ID = "test-model"
	row.Response.Output = `{"reply_text":"ok","reply_language":7,"attach_groups":[],"escalate":"true"}`

	v := judgeOne(TestCase{ID: "bad-contract"}, row, catalog, map[string]string{}, nil, map[string]bool{})
	if v.ContractPass || !v.ModelBehaviorPass {
		t.Fatalf("precondition failed: got ContractPass=%v ModelBehaviorPass=%v, want false/true", v.ContractPass, v.ModelBehaviorPass)
	}

	exec := scenarioExecutionFromVerdict("fixture-scenario", v)

	cf, _ := scoreByName(exec.Scores, "contract_fields")
	if cf.Status != ScoreFail {
		t.Errorf("want contract_fields=fail (real judgeOne evaluated it and it failed), got %s", cf.Status)
	}
	// Every downstream behavior check must be a REAL evaluated result, not not_run —
	// judgeOne kept going after contract_fields failed.
	for _, name := range []string{"requires", "media", "escalate", "language", "must_not_contain",
		"no_invented_digits", "no_unit_issues", "no_unknown_media"} {
		s, ok := scoreByName(exec.Scores, name)
		if !ok {
			t.Fatalf("score %q missing", name)
		}
		if s.Status == ScoreNotRun {
			t.Errorf("score %q: want a real evaluated result from judgeOne, got not_run", name)
		}
	}
}

// TestLoadRunExecutions_MissingFamilyIsFine proves an extraction-only (or
// scenario-only) run dir loads cleanly — the two loaders are independent.
func TestLoadRunExecutions_MissingFamilyIsFine(t *testing.T) {
	runDir := t.TempDir()
	execs, err := loadRunExecutions(runDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(execs) != 0 {
		t.Fatalf("want 0 executions for an empty run dir, got %d", len(execs))
	}
}
