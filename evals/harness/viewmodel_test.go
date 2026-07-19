package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"xchats-evals-harness/internal/provenance"
)

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

	// finish_reason_ok is evaluated independent of parsing (unlike everything in
	// `downstream` below) — a parse failure with no truncation signal (v.Truncated is
	// zero-value false here) must still report a REAL pass, not not_run.
	finishOk, ok := scoreByName(exec.Scores, "finish_reason_ok")
	if !ok || finishOk.Status != ScorePass {
		t.Errorf("want finish_reason_ok=pass (evaluated independently of ParseOK), got %+v (found=%v)", finishOk, ok)
	}

	downstream := []string{
		"contract_fields", "no_unknown_tokens", "no_leftover_braces", "no_reasoning_leak",
		"requires", "media", "escalate", "language", "language_text_ok", "language_field_ok",
		"must_not_contain", "outcomes", "no_invented_digits", "no_unit_issues", "no_unknown_media", "media_count",
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
		TestID:              "t2",
		Model:               "test/model",
		ParseOK:             true,
		ContractFields:      false, // e.g. missing reply_text — but judgeOne kept going anyway
		RequiresPass:        false,
		MediaPass:           true,
		EscalatePass:        true,
		LanguagePass:        true,
		MustNotContainPass:  true,
		ModelBehaviorPass:   false,
		ContractPass:        false,
		MediaCountEvaluated: true, // real judgeOne always sets this once ParseOK is true
	}
	exec := scenarioExecutionFromVerdict("fixture-scenario", v)

	for _, name := range []string{"contract_fields", "no_unknown_tokens", "no_leftover_braces",
		"no_reasoning_leak", "requires", "media", "escalate", "language", "language_text_ok",
		"language_field_ok", "must_not_contain", "no_invented_digits", "no_unit_issues", "no_unknown_media",
		"media_count"} {
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
	for _, name := range []string{"requires", "media", "escalate", "language", "language_text_ok",
		"language_field_ok", "must_not_contain", "no_invented_digits", "no_unit_issues", "no_unknown_media",
		"media_count"} {
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

// TestScenarioExecutionFromVerdict_ReplyTextExtractedFromRawOutput proves the eval
// comparison UI's "LLM reply" column (Output.ReplyText) is populated from the SAME
// parse RawOutput already went through in judgeOne — not left for the frontend to
// re-implement JSON extraction — and stays empty (not an error) when RawOutput never
// parsed, exactly like every other "not evaluated" field on this struct.
func TestScenarioExecutionFromVerdict_ReplyTextExtractedFromRawOutput(t *testing.T) {
	v := Verdict{
		TestID:    "t1",
		Model:     "test/model",
		RawOutput: `{"reply_text":"Доставка 1500 тг.","reply_language":"ru","attach_groups":[],"escalate":false}`,
		ParseOK:   true,
	}
	exec := scenarioExecutionFromVerdict("fixture-scenario", v)
	if exec.Output.ReplyText != "Доставка 1500 тг." {
		t.Errorf("want ReplyText extracted from RawOutput, got %q", exec.Output.ReplyText)
	}
}

func TestScenarioExecutionFromVerdict_ReplyTextEmptyOnParseFailure(t *testing.T) {
	v := Verdict{TestID: "t1", Model: "test/model", RawOutput: "not json", ParseOK: false}
	exec := scenarioExecutionFromVerdict("fixture-scenario", v)
	if exec.Output.ReplyText != "" {
		t.Errorf("want empty ReplyText when RawOutput doesn't parse, got %q", exec.Output.ReplyText)
	}
}

// writeSnapshotScenario is the enrichScenarioExecutions test helper: hand-writes a
// run dir's snapshots/<scenario>/{scenario.yaml,resolved_tests.json} exactly as
// provenance.SnapshotScenario would, without needing a real render pipeline.
func writeSnapshotScenario(t *testing.T, runDir, scenario string, sc ScenarioConfig, tests []TestCase) {
	t.Helper()
	sc.Name = scenario
	snapDir := filepath.Join(runDir, "snapshots", scenario)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scYAML, err := yaml.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "scenario.yaml"), scYAML, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(snapDir, "resolved_tests.json"), ResolvedTests{Tests: tests}); err != nil {
		t.Fatal(err)
	}
}

// TestEnrichScenarioExecutions_SetupExperimentPromptRefAndHistory is the core proof
// for the comparison-matrix metadata (review amendments 2 & 3): Setup/Experiment come
// from the snapshotted scenario.yaml, Prompt is parsed from prompt_ref (not the
// scenario name), and History is attached per-execution by matching TestID against
// the snapshotted resolved_tests.json — none of which scenarioExecutionFromVerdict
// alone could ever know from a bare Verdict.
func TestEnrichScenarioExecutions_SetupExperimentPromptRefAndHistory(t *testing.T) {
	runDir := t.TempDir()
	writeSnapshotScenario(t, runDir, "lang-canary-v4-kk", ScenarioConfig{
		Setup: "lang-v4-routed", PromptRef: "lang-kk@v4", Experiment: "lang-bakeoff",
	}, []TestCase{
		{ID: "t1", Message: "Жеткізу қанша тұрады?", History: []HistoryTurn{
			{Role: "client", Text: "Сәлем!"}, {Role: "assistant", Text: "Сәлеметсіз бе!"},
		}},
		{ID: "t2", Message: "Рахмет"}, // no history
	})

	execs := []VExecution{
		{Subject: VSubject{TestID: "t1"}, Variant: VVariant{Model: "m1"}},
		{Subject: VSubject{TestID: "t2"}, Variant: VVariant{Model: "m1"}},
	}
	out := enrichScenarioExecutions(runDir, "lang-canary-v4-kk", "deadbeef", execs)

	if out[0].Variant.Setup != "lang-v4-routed" {
		t.Errorf("want Setup=lang-v4-routed, got %q", out[0].Variant.Setup)
	}
	if out[0].Variant.Experiment != "lang-bakeoff" {
		t.Errorf("want Experiment=lang-bakeoff, got %q", out[0].Variant.Experiment)
	}
	if out[0].Variant.Prompt.Name != "lang-kk" || out[0].Variant.Prompt.Version != 4 {
		t.Errorf("want Prompt=lang-kk@v4, got %+v", out[0].Variant.Prompt)
	}
	if out[0].Variant.Prompt.SHA256 != "deadbeef" {
		t.Errorf("want the manifest-sourced SHA256 attached to Prompt, got %q", out[0].Variant.Prompt.SHA256)
	}
	if len(out[0].Subject.History) != 2 || out[0].Subject.History[0].Text != "Сәлем!" {
		t.Errorf("want t1's history attached, got %+v", out[0].Subject.History)
	}
	if len(out[1].Subject.History) != 0 {
		t.Errorf("want t2 (no history in its TestCase) to stay empty, got %+v", out[1].Subject.History)
	}
}

// TestEnrichScenarioExecutions_RoutedStrategySharesOneSetupAcrossTwoPromptRefs is the
// concrete V4-kk/V4-ru story ScenarioConfig.Setup's doc comment describes: two
// DIFFERENT scenario dirs (two different prompt_refs, two different snapshots) must
// still report the SAME Setup value, so the comparison matrix shows one "lang-v4-routed"
// column, not two — while each execution still carries its OWN actual Prompt ref.
func TestEnrichScenarioExecutions_RoutedStrategySharesOneSetupAcrossTwoPromptRefs(t *testing.T) {
	runDir := t.TempDir()
	writeSnapshotScenario(t, runDir, "lang-canary-v4-kk",
		ScenarioConfig{Setup: "lang-v4-routed", PromptRef: "lang-kk@v4", Experiment: "lang-bakeoff"}, nil)
	writeSnapshotScenario(t, runDir, "lang-canary-v4-ru",
		ScenarioConfig{Setup: "lang-v4-routed", PromptRef: "lang-ru@v4", Experiment: "lang-bakeoff"}, nil)

	kk := enrichScenarioExecutions(runDir, "lang-canary-v4-kk", "", []VExecution{{Variant: VVariant{Model: "m1"}}})
	ru := enrichScenarioExecutions(runDir, "lang-canary-v4-ru", "", []VExecution{{Variant: VVariant{Model: "m1"}}})

	if kk[0].Variant.Setup != ru[0].Variant.Setup {
		t.Fatalf("want both scenarios to share one Setup column, got kk=%q ru=%q", kk[0].Variant.Setup, ru[0].Variant.Setup)
	}
	if kk[0].Variant.Prompt.Name == ru[0].Variant.Prompt.Name {
		t.Errorf("want DIFFERENT prompt refs per execution (kk vs ru), both got %q", kk[0].Variant.Prompt.Name)
	}
}

// TestEnrichScenarioExecutions_FallbacksForLegacyAndUnannotatedData proves nothing
// regresses for data that predates this feature: no snapshot at all (a run from
// before provenance snapshotting existed) leaves executions untouched; a snapshot
// with an unannotated scenario.yaml (no setup/prompt_ref/experiment — e.g. shop-*)
// falls back to the scenario name for both Setup and Prompt.Name, Experiment stays
// empty (never auto-compared against anything).
func TestEnrichScenarioExecutions_FallbacksForLegacyAndUnannotatedData(t *testing.T) {
	execs := []VExecution{{Variant: VVariant{Model: "m1"}}}

	t.Run("no snapshot at all (legacy run)", func(t *testing.T) {
		runDir := t.TempDir()
		out := enrichScenarioExecutions(runDir, "shop-current", "", execs)
		if out[0].Variant.Setup != "" || out[0].Variant.Prompt.Name != "" {
			t.Errorf("want untouched Variant with no snapshot, got %+v", out[0].Variant)
		}
	})

	t.Run("snapshot present, scenario.yaml unannotated", func(t *testing.T) {
		runDir := t.TempDir()
		writeSnapshotScenario(t, runDir, "shop-current", ScenarioConfig{}, nil)
		out := enrichScenarioExecutions(runDir, "shop-current", "", execs)
		if out[0].Variant.Setup != "shop-current" {
			t.Errorf("want Setup to fall back to the scenario name, got %q", out[0].Variant.Setup)
		}
		if out[0].Variant.Prompt.Name != "shop-current" || out[0].Variant.Prompt.Version != 0 {
			t.Errorf("want Prompt to fall back to (scenario name, v0), got %+v", out[0].Variant.Prompt)
		}
		if out[0].Variant.Experiment != "" {
			t.Errorf("want Experiment to stay empty (never auto-compared), got %q", out[0].Variant.Experiment)
		}
	})

	t.Run("malformed prompt_ref falls back rather than propagating garbage", func(t *testing.T) {
		runDir := t.TempDir()
		writeSnapshotScenario(t, runDir, "weird-scenario", ScenarioConfig{PromptRef: "not-a-valid-spec"}, nil)
		out := enrichScenarioExecutions(runDir, "weird-scenario", "", execs)
		if out[0].Variant.Prompt.Name != "weird-scenario" {
			t.Errorf("want fallback to scenario name on an unparseable prompt_ref, got %+v", out[0].Variant.Prompt)
		}
	})
}

// TestScenarioExecutionFromVerdict_ContractSafetyRows_AlwaysPresentPassOrFail proves
// the six universal safety rows attach unconditionally (no snapshot needed) and read
// their Pass/Actual straight back off the Scores this same function just built — never
// a second, independently-computed verdict. This fixture deliberately does NOT set
// MediaCountEvaluated (simulating a verdict judged by pre-upgrade code, ParseOK=true
// notwithstanding — see MediaCountEvaluated's own doc comment on Verdict) — the
// media_count row must render as not-applicable (Pass=nil), never a fabricated pass.
func TestScenarioExecutionFromVerdict_ContractSafetyRows_AlwaysPresentPassOrFail(t *testing.T) {
	v := Verdict{
		TestID: "t1", Model: "test/model", ParseOK: true, ContractFields: true,
		UnknownMedia: []string{"bogus-ref"}, InventedDigits: []string{"999"},
	}
	exec := scenarioExecutionFromVerdict("fixture-scenario", v)

	wantKeys := []string{"valid_json", "contract_fields", "no_unresolved_placeholders", "no_unknown_media", "no_invented_digits", "media_count", "no_control_chars"}
	if len(exec.Contract) != len(wantKeys) {
		t.Fatalf("want exactly %d safety rows, got %d: %+v", len(wantKeys), len(exec.Contract), exec.Contract)
	}
	for i, k := range wantKeys {
		if exec.Contract[i].Key != k || exec.Contract[i].Kind != "safety" {
			t.Errorf("row %d: want key=%q kind=safety, got %+v", i, k, exec.Contract[i])
		}
	}
	if p := contractRow(t, exec.Contract, "no_unknown_media"); p.Pass == nil || *p.Pass || p.Actual != "неизвестные ссылки: bogus-ref" {
		t.Errorf("want no_unknown_media=fail with the offending ref in Actual, got %+v", p)
	}
	if p := contractRow(t, exec.Contract, "no_invented_digits"); p.Pass == nil || *p.Pass || p.Actual != "999" {
		t.Errorf("want no_invented_digits=fail with the digit in Actual, got %+v", p)
	}
	if p := contractRow(t, exec.Contract, "valid_json"); p.Pass == nil || !*p.Pass || p.Actual != "разобран корректно" {
		t.Errorf("want valid_json=pass, got %+v", p)
	}
	// The core "no fabricated pass" guarantee: MediaCountEvaluated unset must render as
	// Pass=nil (not applicable/not run), never a silent true just because TooManyMedia's
	// zero value happens to be false.
	if p := contractRow(t, exec.Contract, "media_count"); p.Pass != nil {
		t.Errorf("want media_count Pass=nil for a verdict that never set MediaCountEvaluated, got %v (Actual=%q)", *p.Pass, p.Actual)
	}
}

// TestScenarioExecutionFromVerdict_MediaCountEvaluated_RendersRealPassOrFail is the
// complementary case: once MediaCountEvaluated is true (a verdict judged by CURRENT
// code), the media_count row must show a REAL pass/fail, not the legacy not-applicable
// state — both the within-cap and over-cap outcomes.
func TestScenarioExecutionFromVerdict_MediaCountEvaluated_RendersRealPassOrFail(t *testing.T) {
	within := Verdict{TestID: "t1", Model: "m", ParseOK: true, ContractFields: true, MediaCountEvaluated: true, MediaCount: 2, TooManyMedia: false}
	exec := scenarioExecutionFromVerdict("fixture-scenario", within)
	if p := contractRow(t, exec.Contract, "media_count"); p.Pass == nil || !*p.Pass {
		t.Errorf("want media_count=pass when evaluated and within the cap, got %+v", p)
	}

	over := Verdict{TestID: "t1", Model: "m", ParseOK: true, ContractFields: true, MediaCountEvaluated: true, MediaCount: 5, TooManyMedia: true}
	exec2 := scenarioExecutionFromVerdict("fixture-scenario", over)
	if p := contractRow(t, exec2.Contract, "media_count"); p.Pass == nil || *p.Pass || p.Actual != "5 attachments" {
		t.Errorf("want media_count=fail with the count in Actual when over the cap, got %+v", p)
	}
}

// TestScenarioExecutionFromVerdict_ContractSafetyRows_ParseFailureIsNotRun proves a
// parse failure reports the placeholder-safety row as not-applicable (Pass=nil), not a
// false "fail" — mirroring evaluated()'s own not_run semantics on the Scores list right
// next to it.
func TestScenarioExecutionFromVerdict_ContractSafetyRows_ParseFailureIsNotRun(t *testing.T) {
	v := Verdict{TestID: "t1", Model: "test/model", ParseOK: false, Reason: "invalid JSON"}
	exec := scenarioExecutionFromVerdict("fixture-scenario", v)

	if p := contractRow(t, exec.Contract, "no_unresolved_placeholders"); p.Pass != nil {
		t.Errorf("want no_unresolved_placeholders Pass=nil (not evaluated) on a parse failure, got %v", *p.Pass)
	}
	if p := contractRow(t, exec.Contract, "valid_json"); p.Pass == nil || *p.Pass || p.Actual != "invalid JSON" {
		t.Errorf("want valid_json=fail with the parse reason as Actual, got %+v", p)
	}
	if p := contractRow(t, exec.Contract, "media_count"); p.Pass != nil {
		t.Errorf("want media_count Pass=nil on a parse failure (MediaCountEvaluated never set), got %v", *p.Pass)
	}
}

// contractRow finds one row by key, failing the test if absent.
func contractRow(t *testing.T, rows []VContractRow, key string) VContractRow {
	t.Helper()
	for _, r := range rows {
		if r.Key == key {
			return r
		}
	}
	t.Fatalf("contract row %q not found in %+v", key, rows)
	return VContractRow{}
}

// TestEnrichScenarioExecutions_ContractRequirementRows_OnlyDeclaredOnesShown is the
// core proof for the Requirements panel's scenario-family join: rows for
// Requires/Language/Escalate/Media/MustNotContain appear ONLY when the test itself
// declares that requirement, prepended before the five safety rows, with Expected read
// from the snapshotted TestCase and Actual read from the model's own persisted RawOutput
// — never a re-grade (Pass always matches the Verdict-derived Score already on Scores).
func TestEnrichScenarioExecutions_ContractRequirementRows_OnlyDeclaredOnesShown(t *testing.T) {
	runDir := t.TempDir()
	writeSnapshotScenario(t, runDir, "shop-current", ScenarioConfig{Contract: "asset_refs"}, []TestCase{
		{
			ID:             "t1",
			Message:        "Сколько стоит кофемашина?",
			Requires:       [][]string{{"product.coffee-machine.price"}},
			Language:       "ru",
			Escalate:       boolPtr(false),
			Media:          &MediaExpect{AnyOfRefs: []string{"coffee-photo-1"}},
			MustNotContain: []string{"бесплатно"},
		},
		{ID: "t2", Message: "no requirements declared at all"},
	})

	raw := `{"reply_text":"Кофемашина стоит {{product.coffee-machine.price}}","reply_language":"ru","escalate":false,"asset_refs":["coffee-photo-1"]}`
	v1 := Verdict{
		TestID: "t1", Model: "m1", ParseOK: true, ContractFields: true, RawOutput: raw,
		RequiresPass: true, LanguagePass: true, EscalatePass: true, MediaPass: true, MustNotContainPass: true,
	}
	v2 := Verdict{TestID: "t2", Model: "m1", ParseOK: true, ContractFields: true, RawOutput: `{"reply_text":"ок"}`}
	execs := []VExecution{scenarioExecutionFromVerdict("shop-current", v1), scenarioExecutionFromVerdict("shop-current", v2)}

	out := enrichScenarioExecutions(runDir, "shop-current", "", execs)

	t1 := out[0]
	wantOrder := []string{"requires", "language", "escalate", "media", "must_not_contain",
		"valid_json", "contract_fields", "no_unresolved_placeholders", "no_unknown_media", "no_invented_digits", "media_count", "no_control_chars"}
	if len(t1.Contract) != len(wantOrder) {
		t.Fatalf("want %d rows (5 requirement + 7 safety), got %d: %+v", len(wantOrder), len(t1.Contract), t1.Contract)
	}
	for i, k := range wantOrder {
		if t1.Contract[i].Key != k {
			t.Errorf("row %d: want key=%q, got %q (requirement rows must come before safety rows)", i, k, t1.Contract[i].Key)
		}
	}

	req := contractRow(t, t1.Contract, "requires")
	if req.Expected != "product.coffee-machine.price" || req.Actual != "product.coffee-machine.price" || req.Pass == nil || !*req.Pass {
		t.Errorf("want requires expected==actual==the resolved token, pass=true, got %+v", req)
	}
	lang := contractRow(t, t1.Contract, "language")
	if lang.Expected != "ru" || lang.Actual != "ru" {
		t.Errorf("want language expected=ru actual=ru (from reply_language), got %+v", lang)
	}
	esc := contractRow(t, t1.Contract, "escalate")
	if esc.Expected != "нет" || esc.Actual != "нет" {
		t.Errorf("want escalate expected=нет actual=нет, got %+v", esc)
	}
	media := contractRow(t, t1.Contract, "media")
	if media.Expected != "coffee-photo-1" || media.Actual != "coffee-photo-1" {
		t.Errorf("want media expected=actual=coffee-photo-1, got %+v", media)
	}
	mnc := contractRow(t, t1.Contract, "must_not_contain")
	if mnc.Expected != "бесплатно" || mnc.Actual != "нет" {
		t.Errorf("want must_not_contain expected=бесплатно actual=нет (nothing matched), got %+v", mnc)
	}

	t2 := out[1]
	if len(t2.Contract) != 7 {
		t.Fatalf("want t2 (no requirements declared) to carry ONLY the 7 safety rows, got %d: %+v", len(t2.Contract), t2.Contract)
	}
	for _, r := range t2.Contract {
		if r.Kind != "safety" {
			t.Errorf("want every t2 row to be kind=safety, got %+v", r)
		}
	}
}

// TestEnrichScenarioExecutions_ContractRequirementRows_FailedRequirementShowsMismatch
// proves a genuinely failed requirement surfaces the ACTUAL wrong value, not just
// pass=false — the whole point of the panel over a bare pass/fail badge.
func TestEnrichScenarioExecutions_ContractRequirementRows_FailedRequirementShowsMismatch(t *testing.T) {
	runDir := t.TempDir()
	writeSnapshotScenario(t, runDir, "lang-canary-v1", ScenarioConfig{}, []TestCase{
		{ID: "t1", Message: "Кофемашина қанша тұрады?", Language: "kk", Escalate: boolPtr(true)},
	})
	raw := `{"reply_text":"Кофемашина стоит 129900","reply_language":"ru","escalate":false}`
	v := Verdict{
		TestID: "t1", Model: "m1", ParseOK: true, ContractFields: true, RawOutput: raw,
		LanguagePass: false, EscalatePass: false,
	}
	execs := enrichScenarioExecutions(runDir, "lang-canary-v1", "",
		[]VExecution{scenarioExecutionFromVerdict("lang-canary-v1", v)})

	lang := contractRow(t, execs[0].Contract, "language")
	if lang.Expected != "kk" || lang.Actual != "ru" || lang.Pass == nil || *lang.Pass {
		t.Errorf("want language expected=kk actual=ru (the model's real declared field) pass=false, got %+v", lang)
	}
	esc := contractRow(t, execs[0].Contract, "escalate")
	if esc.Expected != "да" || esc.Actual != "нет" || esc.Pass == nil || *esc.Pass {
		t.Errorf("want escalate expected=да actual=нет pass=false, got %+v", esc)
	}
}

// TestEnrichScenarioExecutions_ForbidMediaShowsDedicatedExpectedText proves the
// Requirements panel's media row reads "медиа быть не должно" (not a joined empty-string
// list) when the test declares Media.Forbid — distinguishing it from an ordinary
// any_of_groups/any_of_refs expectation, which would render the allowed list instead.
func TestEnrichScenarioExecutions_ForbidMediaShowsDedicatedExpectedText(t *testing.T) {
	runDir := t.TempDir()
	writeSnapshotScenario(t, runDir, "shop-current", ScenarioConfig{Contract: "asset_refs"}, []TestCase{
		{ID: "t1", Message: "Здравствуйте!", Media: &MediaExpect{Forbid: true}},
	})
	v := Verdict{TestID: "t1", Model: "m1", ParseOK: true, ContractFields: true, RawOutput: `{"reply_text":"Здравствуйте!","reply_language":"ru","asset_refs":[],"escalate":false}`, MediaPass: true}
	execs := enrichScenarioExecutions(runDir, "shop-current", "", []VExecution{scenarioExecutionFromVerdict("shop-current", v)})

	media := contractRow(t, execs[0].Contract, "media")
	if media.Expected != "медиа быть не должно" {
		t.Errorf("want Expected=%q for a forbid expectation, got %+v", "медиа быть не должно", media)
	}
	if media.Actual != "нет" || media.Pass == nil || !*media.Pass {
		t.Errorf("want Actual=нет pass=true (no media attached, correctly), got %+v", media)
	}
}

func boolPtr(b bool) *bool { return &b }

// writeSnapshotExtractCases hand-writes a run dir's snapshots/extract_cases.yaml
// exactly as cmdExtract's SnapshotFile call would, without needing a real extraction
// run.
func writeSnapshotExtractCases(t *testing.T, runDir string, cases []ExtractCase) {
	t.Helper()
	snapDir := filepath.Join(runDir, "snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := yaml.Marshal(ExtractCasesFile{Cases: cases})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "extract_cases.yaml"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestExtractExecutionFromResult_ContractSafetyRows proves the extract family's two
// safety rows behave honestly across all three outcomes: a call error and a parse
// failure both still get a fail valid_json row (Requirements panel never goes empty
// just because nothing else could be graded), and no_reasoning_leak only ever appears
// when runExtractChecks actually ran it (Checks non-empty) — never fabricated.
func TestExtractExecutionFromResult_ContractSafetyRows(t *testing.T) {
	t.Run("call error", func(t *testing.T) {
		exec := extractExecutionFromResult(extractRunResult{CaseID: "c1", Error: "http 500"})
		if len(exec.Contract) != 1 {
			t.Fatalf("want exactly the valid_json row, got %+v", exec.Contract)
		}
		row := exec.Contract[0]
		if row.Key != "valid_json" || row.Pass == nil || *row.Pass || row.Actual != "ошибка вызова: http 500" {
			t.Errorf("want valid_json=fail with the call error as Actual, got %+v", row)
		}
	})

	t.Run("parse failure", func(t *testing.T) {
		exec := extractExecutionFromResult(extractRunResult{CaseID: "c1", ParseError: "unexpected EOF"})
		row := contractRow(t, exec.Contract, "valid_json")
		if row.Pass == nil || *row.Pass || row.Actual != "не разобран: unexpected EOF" {
			t.Errorf("want valid_json=fail with the parse error as Actual, got %+v", row)
		}
	})

	t.Run("success", func(t *testing.T) {
		exec := extractExecutionFromResult(extractRunResult{
			CaseID: "c1", Parsed: &ExtractionResult{ContentKind: "product_photo"},
			Checks: []CheckResult{{Name: "no_reasoning_leak", Pass: true}},
		})
		if len(exec.Contract) != 2 {
			t.Fatalf("want valid_json + no_reasoning_leak, got %+v", exec.Contract)
		}
		vj := contractRow(t, exec.Contract, "valid_json")
		if vj.Pass == nil || !*vj.Pass {
			t.Errorf("want valid_json=pass, got %+v", vj)
		}
		leak := contractRow(t, exec.Contract, "no_reasoning_leak")
		if leak.Pass == nil || !*leak.Pass || leak.Actual != "не найдено" {
			t.Errorf("want no_reasoning_leak=pass, got %+v", leak)
		}
	})
}

// TestEnrichExtractExecutions_ContractRequirementRows is the core proof for the
// Requirements panel's extract-family join: field/text/number checks become rows with
// Expected from the snapshotted case and Actual from the persisted parsed output or the
// check's own Detail, and a case absent from the snapshot (or no snapshot at all) is
// left with only the safety rows already attached — never a crash, never a fabricated row.
func TestEnrichExtractExecutions_ContractRequirementRows(t *testing.T) {
	runDir := t.TempDir()
	writeSnapshotExtractCases(t, runDir, []ExtractCase{
		{
			ID:              "infographic",
			File:            "../assets/infographic.png",
			Fields:          map[string]string{"content_kind": "infographic"},
			TextContainsAll: []string{"старт", "рост"},
			AllowedNumbers:  []string{"10 000", "25 000"},
		},
	})

	passing := extractExecutionFromResult(extractRunResult{
		CaseID: "infographic",
		Parsed: &ExtractionResult{ContentKind: "infographic", ExtractedText: "старт 10 000, рост 25 000"},
		Checks: []CheckResult{
			{Name: "field:content_kind", Pass: true},
			{Name: "text_contains_all", Pass: true},
			{Name: "no_invented_numbers", Pass: true},
		},
	})
	failing := extractExecutionFromResult(extractRunResult{
		CaseID: "infographic",
		Parsed: &ExtractionResult{ContentKind: "screenshot", ExtractedText: "старт"},
		Checks: []CheckResult{
			{Name: "field:content_kind", Pass: false, Detail: `want "infographic", got "screenshot"`},
			{Name: "text_contains_all", Pass: false, Detail: "missing: рост"},
			{Name: "no_invented_numbers", Pass: true},
		},
	})
	unknownCase := extractExecutionFromResult(extractRunResult{
		CaseID: "not-in-snapshot",
		Parsed: &ExtractionResult{},
		Checks: []CheckResult{{Name: "no_reasoning_leak", Pass: true}},
	})

	out := enrichExtractExecutions(runDir, []VExecution{passing, failing, unknownCase})

	p := contractRow(t, out[0].Contract, "field:content_kind")
	if p.Expected != "infographic" || p.Actual != "infographic" || p.Pass == nil || !*p.Pass {
		t.Errorf("want field:content_kind pass with expected=actual=infographic, got %+v", p)
	}
	pNums := contractRow(t, out[0].Contract, "no_invented_numbers")
	if pNums.Expected != "10 000, 25 000" {
		t.Errorf("want no_invented_numbers Expected to be the allowed list, got %q", pNums.Expected)
	}

	f := contractRow(t, out[1].Contract, "field:content_kind")
	if f.Expected != "infographic" || f.Actual != "screenshot" || f.Pass == nil || *f.Pass {
		t.Errorf("want field:content_kind fail showing expected=infographic actual=screenshot, got %+v", f)
	}
	fText := contractRow(t, out[1].Contract, "text_contains_all")
	if fText.Actual != "missing: рост" {
		t.Errorf("want text_contains_all Actual to be the missing-phrase detail, got %q", fText.Actual)
	}

	for _, r := range out[2].Contract {
		if r.Kind != "safety" {
			t.Errorf("want a case absent from the snapshot to carry ONLY safety rows, got %+v", r)
		}
	}
}

// TestScenarioExecutionFromVerdict_OutcomesScore pins the outcomes score's three-way
// rendering: not_run unless the verdict says the check was declared (OutcomesDeclared is
// a code-version + test-shape marker, the MediaCountEvaluated pattern — an old judged.json
// or an outcomes-less test must never fabricate a pass/fail from OutcomesPass's zero
// value), fail with the full declared label set, pass with the matched block's label.
func TestScenarioExecutionFromVerdict_OutcomesScore(t *testing.T) {
	t.Run("old verdict shape / no outcomes declared -> not_run", func(t *testing.T) {
		v := Verdict{TestID: "t", Model: "m", ParseOK: true, ContractFields: true}
		exec := scenarioExecutionFromVerdict("s", v)
		s, ok := scoreByName(exec.Scores, "outcomes")
		if !ok || s.Status != ScoreNotRun {
			t.Errorf("want outcomes=not_run without the OutcomesDeclared marker, got %+v (found=%v)", s, ok)
		}
	})

	t.Run("declared and failed -> fail, detail lists every alternative", func(t *testing.T) {
		v := Verdict{
			TestID: "t", Model: "m", ParseOK: true, ContractFields: true,
			OutcomesDeclared: true, OutcomesPass: false,
			OutcomeLabels: []string{"answers with the token", "asks which tariff"},
		}
		exec := scenarioExecutionFromVerdict("s", v)
		s, _ := scoreByName(exec.Scores, "outcomes")
		if s.Status != ScoreFail {
			t.Errorf("want outcomes=fail, got %s", s.Status)
		}
		if s.Detail != "answers with the token | asks which tariff" {
			t.Errorf("want detail to list the full declared set, got %q", s.Detail)
		}
	})

	t.Run("declared and passed -> pass, detail names the matched block", func(t *testing.T) {
		v := Verdict{
			TestID: "t", Model: "m", ParseOK: true, ContractFields: true,
			OutcomesDeclared: true, OutcomesPass: true,
			OutcomeMatched: "asks which tariff",
			OutcomeLabels:  []string{"answers with the token", "asks which tariff"},
		}
		exec := scenarioExecutionFromVerdict("s", v)
		s, _ := scoreByName(exec.Scores, "outcomes")
		if s.Status != ScorePass {
			t.Errorf("want outcomes=pass, got %s", s.Status)
		}
		if s.Detail != "asks which tariff" {
			t.Errorf("want detail to name the matched block, got %q", s.Detail)
		}
	})
}

// TestEnrichScenarioExecutions_OutcomesRequirementRow proves the Requirements panel gets
// an outcomes row ONLY when the snapshotted test declares alternatives, with Expected
// listing every block's label and Actual naming the matched one (or stating the miss).
func TestEnrichScenarioExecutions_OutcomesRequirementRow(t *testing.T) {
	runDir := t.TempDir()
	escalateFalse := false
	writeSnapshotScenario(t, runDir, "shop-current", ScenarioConfig{Contract: "asset_refs"}, []TestCase{
		{
			ID:      "t1",
			Message: "Кстати, а какой у него лимит платежей в месяц?",
			Outcomes: []OutcomeCase{
				{Label: "answers with the token", Requires: [][]string{{"tariff.business.payment_limit_monthly"}}, Escalate: &escalateFalse},
				{Label: "asks which tariff", Escalate: &escalateFalse, MustContainAny: []string{"уточните"}},
			},
		},
	})

	v := Verdict{
		TestID: "t1", Model: "m1", ParseOK: true, ContractFields: true,
		RawOutput:        `{"reply_text":"Уточните, пожалуйста, какой тариф вас интересует?","reply_language":"ru","escalate":false,"asset_refs":[]}`,
		OutcomesDeclared: true, OutcomesPass: true, OutcomeMatched: "asks which tariff",
		OutcomeLabels: []string{"answers with the token", "asks which tariff"},
	}
	out := enrichScenarioExecutions(runDir, "shop-current", "", []VExecution{scenarioExecutionFromVerdict("shop-current", v)})

	row := contractRow(t, out[0].Contract, "outcomes")
	if row.Expected != "answers with the token ИЛИ asks which tariff" {
		t.Errorf("want Expected to join the declared labels, got %q", row.Expected)
	}
	if row.Actual != "asks which tariff" {
		t.Errorf("want Actual to name the matched block, got %q", row.Actual)
	}
	if row.Pass == nil || !*row.Pass {
		t.Errorf("want the outcomes row to pass, got %+v", row)
	}
}
