package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"xchats-evals-harness/internal/provenance"
)

// ScoreStatus is a named check's evaluated state. "not_run" exists specifically so a
// check that was never evaluated (e.g. every downstream check when JSON parsing
// itself failed) is distinguishable from a check that WAS evaluated and failed —
// collapsing the two to a single "fail" would misreport "the model used the wrong
// token" when the true story is "the model's output wasn't even valid JSON, so no
// token check ever ran."
type ScoreStatus string

const (
	ScorePass   ScoreStatus = "pass"
	ScoreFail   ScoreStatus = "fail"
	ScoreNotRun ScoreStatus = "not_run"
	ScoreError  ScoreStatus = "error"
)

// VScore is one named check's result — the shape both eval families' checks (judge.go
// Verdict fields, extract_checks.go CheckResult) are adapted into.
type VScore struct {
	Name   string      `json:"name"`
	Status ScoreStatus `json:"status"`
	Detail string      `json:"detail,omitempty"`
}

// VRollup is a headline aggregate boolean, e.g. contract_pass / model_behavior_pass.
// Unlike VScore, a rollup has no not_run state: PLAYGROUND.md's doctrine is that
// contract_pass and model_behavior_pass are DEFINED to require a successful parse
// (see judge.go's `v.ContractPass = v.ParseOK && ...`), so "false because parsing
// failed" is not a missing evaluation — it is the correct, meaningful value of the
// aggregate. Keeping rollups and component scores separate is what lets the two carry
// different not-run semantics without contradicting each other.
type VRollup struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Pass  bool   `json:"pass"`
}

// VSubject identifies what was tested. Only the fields relevant to Family are set.
type VSubject struct {
	Scenario string `json:"scenario,omitempty"`
	TestID   string `json:"test_id,omitempty"`
	Message  string `json:"message,omitempty"`
	CaseID   string `json:"case_id,omitempty"`
	InputRef string `json:"input_ref,omitempty"` // run-dir-relative path to the captured input, if any
}

// VVariant identifies how it was tested. Prompt/Preprocessor are zero-valued for the
// scenario family for now — scenario prompt variants are not modeled yet.
type VVariant struct {
	Model        string               `json:"model"`
	Prompt       provenance.PromptRef `json:"prompt,omitempty"`
	Preprocessor string               `json:"preprocessor,omitempty"`
}

// VOutput carries the raw model output and its parse state.
type VOutput struct {
	Raw        string `json:"raw,omitempty"`
	ParseOK    bool   `json:"parse_ok"`
	ParseError string `json:"parse_error,omitempty"`
	Error      string `json:"error,omitempty"`
}

// VCost mirrors the existing cost-basis discipline: EstimateUSD must never be read
// without Basis, and a basis other than a "measured"/"borrowed" one means the number
// is 0 and should be displayed as "no estimate", not "free".
type VCost struct {
	TokensIn    int     `json:"tokens_in"`
	TokensOut   int     `json:"tokens_out"`
	EstimateUSD float64 `json:"estimate_usd"`
	Basis       string  `json:"basis"`
}

// VScenarioDetails is the family-specific evidence a scenario verdict carries that
// doesn't fit the generic Scores list (free text and lists, not named pass/fail checks).
type VScenarioDetails struct {
	InjectedText    string   `json:"injected_text,omitempty"`
	UnknownTokens   []string `json:"unknown_tokens,omitempty"`
	UnknownMedia    []string `json:"unknown_media,omitempty"`
	InventedDigits  []string `json:"invented_digits,omitempty"`
	UnitIssues      []string `json:"unit_issues,omitempty"`
	ForbiddenPhrase string   `json:"forbidden_phrase,omitempty"`
	Blocked         bool     `json:"blocked"`
	LeftoverBraces  bool     `json:"leftover_braces"`
}

// VExtractDetails is the parsed ExtractionResult, carried alongside Scores so the
// viewer can render a fields table without re-decoding Output.Raw itself.
type VExtractDetails struct {
	ContentKind          string `json:"content_kind,omitempty"`
	Summary              string `json:"summary,omitempty"`
	ExtractedText        string `json:"extracted_text,omitempty"`
	Language             string `json:"language,omitempty"`
	VisibilitySuggestion string `json:"visibility_suggestion,omitempty"`
	MediaRoleHint        string `json:"media_role_hint,omitempty"`
	RelatesToHint        string `json:"relates_to_hint,omitempty"`
}

// VExecution is the unified, in-memory-only view of one graded attempt from EITHER
// family. Nothing of this shape is persisted to disk — it is rebuilt at read time
// from the existing *.judged.json / extract_outputs/*.json files, which remain the
// single source of truth (see the plan's rejection of a persisted executions.jsonl:
// two writers of the same fact WILL diverge eventually).
type VExecution struct {
	Family    string    `json:"family"` // "scenario" | "extract"
	Subject   VSubject  `json:"subject"`
	Variant   VVariant  `json:"variant"`
	Output    VOutput   `json:"output"`
	Scores    []VScore  `json:"scores"`
	Rollups   []VRollup `json:"rollups"`
	Cost      VCost     `json:"cost"`
	LatencyMs int       `json:"latency_ms,omitempty"`

	Scenario *VScenarioDetails `json:"scenario,omitempty"`
	Extract  *VExtractDetails  `json:"extract,omitempty"`
}

// scenarioExecutionFromVerdict maps one judge.go Verdict onto the unified model.
// judgeOne (judge.go) has exactly ONE early-return gate: a JSON parse failure (see
// judge.go ~line 300-305, `if !ok { ...; return v }`). Every check after that point —
// contract_fields, token resolution, requires/media/escalate/language/invented-digits/
// unit-issues — executes unconditionally; there is no second early return, even when
// contract_fields itself fails. So a score is "not_run" ONLY when ParseOK is false;
// once parsing succeeds, every score reflects a real, evaluated result (verified by
// the golden tests in viewmodel_test.go, which is what an earlier reviewer's "not_run
// bug" report was about).
func scenarioExecutionFromVerdict(scenario string, v Verdict) VExecution {
	evaluated := func(pass bool) ScoreStatus {
		if !v.ParseOK {
			return ScoreNotRun
		}
		if pass {
			return ScorePass
		}
		return ScoreFail
	}

	parseStatus := ScoreFail
	parseDetail := v.Reason
	if v.ParseOK {
		parseStatus = ScorePass
		parseDetail = ""
	}

	scores := []VScore{
		{Name: "parse_ok", Status: parseStatus, Detail: parseDetail},
		{Name: "contract_fields", Status: evaluated(v.ContractFields)},
		{Name: "no_unknown_tokens", Status: evaluated(len(v.UnknownTokens) == 0), Detail: strings.Join(v.UnknownTokens, ", ")},
		{Name: "no_leftover_braces", Status: evaluated(!v.LeftoverBraces)},
		{Name: "requires", Status: evaluated(v.RequiresPass)},
		{Name: "media", Status: evaluated(v.MediaPass)},
		{Name: "escalate", Status: evaluated(v.EscalatePass)},
		{Name: "language", Status: evaluated(v.LanguagePass), Detail: v.LanguageIssue},
		// Reported separately from the combined "language" row above (Phase 0.4): looksKazakh
		// is a cheap presence heuristic, not a whole-reply classifier — telling "the text
		// didn't look Kazakh" apart from "the model declared the wrong reply_language field"
		// matters when manually inspecting a Kazakh canary run's actual replies.
		{Name: "language_text_ok", Status: evaluated(v.LanguageTextOK)},
		{Name: "language_field_ok", Status: evaluated(v.LanguageFieldOK)},
		{Name: "must_not_contain", Status: evaluated(v.MustNotContainPass), Detail: v.ForbiddenPhrase},
		{Name: "no_invented_digits", Status: evaluated(len(v.InventedDigits) == 0), Detail: strings.Join(v.InventedDigits, ", ")},
		{Name: "no_unit_issues", Status: evaluated(len(v.UnitIssues) == 0), Detail: strings.Join(v.UnitIssues, ", ")},
		{Name: "no_unknown_media", Status: evaluated(len(v.UnknownMedia) == 0), Detail: strings.Join(v.UnknownMedia, ", ")},
	}

	rollups := []VRollup{
		{Key: "contract_pass", Label: "Contract pass", Pass: v.ContractPass},
		{Key: "model_behavior_pass", Label: "Model-behavior pass", Pass: v.ModelBehaviorPass},
	}

	return VExecution{
		Family:  "scenario",
		Subject: VSubject{Scenario: scenario, TestID: v.TestID, Message: v.Message},
		Variant: VVariant{Model: v.Model},
		Output: VOutput{
			Raw:     v.RawOutput,
			ParseOK: v.ParseOK,
		},
		Scores:    scores,
		Rollups:   rollups,
		Cost:      VCost{TokensIn: v.TokensIn, TokensOut: v.TokensOut, EstimateUSD: v.CostEstimateUSD, Basis: v.CostBasis},
		LatencyMs: v.LatencyMs,
		Scenario: &VScenarioDetails{
			InjectedText:    v.InjectedText,
			UnknownTokens:   v.UnknownTokens,
			UnknownMedia:    v.UnknownMedia,
			InventedDigits:  v.InventedDigits,
			UnitIssues:      v.UnitIssues,
			ForbiddenPhrase: v.ForbiddenPhrase,
			Blocked:         v.Blocked,
			LeftoverBraces:  v.LeftoverBraces,
		},
	}
}

func scenarioExecutionsFromJudgedRun(jr JudgedRun) []VExecution {
	out := make([]VExecution, 0, len(jr.Verdicts))
	for _, v := range jr.Verdicts {
		out = append(out, scenarioExecutionFromVerdict(jr.Scenario, v))
	}
	return out
}

// extractExecutionFromResult maps one extract.go extractRunResult onto the unified
// model. runOneExtraction has two distinct "nothing was checked" outcomes — a real
// HTTP/network error (Error != "", never retried) and a parse failure surviving every
// retry (ParseError != "", Checks was simply never populated, extract_checks.go's
// runExtractChecks is only ever called once r.Parsed is set). In BOTH cases Checks is
// empty, and unlike the scenario family there is no fixed list of check names to
// backfill as "not_run" — which checks would have run depends on the case's
// declarations (extract/cases.yaml), not on the result alone. So an error/parse
// failure here reports zero Scores (nothing to show) with the reason surfaced via
// Output.Error/ParseError instead — honest about what wasn't evaluated rather than
// fabricating placeholder rows.
func extractExecutionFromResult(r extractRunResult) VExecution {
	var scores []VScore
	if r.Parsed != nil {
		for _, c := range r.Checks {
			st := ScoreFail
			if c.Pass {
				st = ScorePass
			}
			scores = append(scores, VScore{Name: c.Name, Status: st, Detail: c.Detail})
		}
	}

	var extractDetails *VExtractDetails
	if r.Parsed != nil {
		extractDetails = &VExtractDetails{
			ContentKind:          r.Parsed.ContentKind,
			Summary:              r.Parsed.Summary,
			ExtractedText:        r.Parsed.ExtractedText,
			Language:             r.Parsed.Language,
			VisibilitySuggestion: r.Parsed.VisibilitySuggestion,
			MediaRoleHint:        r.Parsed.MediaRoleHint,
			RelatesToHint:        r.Parsed.RelatesToHint,
		}
	}

	return VExecution{
		Family:  "extract",
		Subject: VSubject{CaseID: r.CaseID, InputRef: filepath.Join("inputs", r.CaseID+".jpg")},
		Variant: VVariant{Model: r.Model, Prompt: r.Prompt, Preprocessor: r.Preprocessor},
		Output: VOutput{
			Raw:        r.Raw,
			ParseOK:    r.Parsed != nil,
			ParseError: r.ParseError,
			Error:      r.Error,
		},
		Scores: scores,
		Rollups: []VRollup{
			{Key: "all_checks_pass", Label: "All checks pass", Pass: allChecksPass(r.Checks)},
		},
		Cost:    VCost{TokensIn: r.Usage.PromptTokens, TokensOut: r.Usage.CompletionTokens, EstimateUSD: r.Cost, Basis: r.CostBasis},
		Extract: extractDetails,
	}
}

// loadScenarioExecutions reads every *.judged.json in runDir and adapts each verdict.
// Absent entirely for an extraction-only run — that's fine, the caller combines both.
func loadScenarioExecutions(runDir string) ([]VExecution, error) {
	files, err := filepath.Glob(filepath.Join(runDir, "*.judged.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var out []VExecution
	for _, f := range files {
		var jr JudgedRun
		if err := readJSON(f, &jr); err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		out = append(out, scenarioExecutionsFromJudgedRun(jr)...)
	}
	return out, nil
}

// loadExtractExecutions reads every extract_outputs/*.json in runDir and adapts each.
func loadExtractExecutions(runDir string) ([]VExecution, error) {
	files, err := filepath.Glob(filepath.Join(runDir, "extract_outputs", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var out []VExecution
	for _, f := range files {
		var r extractRunResult
		if err := readJSON(f, &r); err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		out = append(out, extractExecutionFromResult(r))
	}
	return out, nil
}

// loadRunExecutions loads every VExecution present in runDir, across both families —
// the one entry point the HTML viewer (step 5) and runs index (step 6) both use.
func loadRunExecutions(runDir string) ([]VExecution, error) {
	scenarioExecs, err := loadScenarioExecutions(runDir)
	if err != nil {
		return nil, err
	}
	extractExecs, err := loadExtractExecutions(runDir)
	if err != nil {
		return nil, err
	}
	return append(scenarioExecs, extractExecs...), nil
}
