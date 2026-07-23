package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"xchats-evals-harness/internal/evaltext"
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

// VContractRow is one line of the Requirements panel: what a test/case expected, what
// the model actually produced, and whether that passed — the human-readable projection
// of the same pass/fail already carried by Scores/Rollups above, NEVER a re-grade. Kind
// distinguishes a test-specific requirement (varies per test/case, from its own
// tests.yaml/cases.yaml declaration) from a universal safety check (same expectation on
// every execution in the family, regardless of what the test itself declares). Pass is a
// pointer so "not applicable to this test" (nil — e.g. a test with no `escalate:`
// expectation) is distinguishable from a real fail.
type VContractRow struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // "requirement" | "safety"
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Pass     *bool  `json:"pass,omitempty"`
}

func scorePassPtr(st ScoreStatus) *bool {
	if st == ScoreNotRun {
		return nil
	}
	b := st == ScorePass
	return &b
}

func scoreByName(scores []VScore, name string) (VScore, bool) {
	for _, s := range scores {
		if s.Name == name {
			return s, true
		}
	}
	return VScore{}, false
}

// VSubject identifies what was tested. Only the fields relevant to Family are set.
type VSubject struct {
	Scenario string `json:"scenario,omitempty"`
	TestID   string `json:"test_id,omitempty"`
	Message  string `json:"message,omitempty"`
	// History is the prior conversation turns this test's message was asked against
	// (TestCase.History) — populated by enrichScenarioExecutions from the run's own
	// snapshotted resolved_tests.json, keyed by TestID. Empty for extraction
	// executions and for scenario runs with no snapshot (legacy runs).
	History  []HistoryTurn `json:"history,omitempty"`
	CaseID   string        `json:"case_id,omitempty"`
	InputRef string        `json:"input_ref,omitempty"` // run-dir-relative path to the captured input, if any
}

// VVariant identifies how it was tested.
type VVariant struct {
	Model string `json:"model"`
	// Setup is the comparison-matrix COLUMN (ScenarioConfig.Setup for the scenario
	// family, Prompt.String() for extraction — set directly in
	// extractExecutionFromResult below) — see ScenarioConfig.Setup's doc comment for
	// why this can differ from Prompt (a routed strategy spanning two frames is ONE
	// setup, realized by two different prompt refs). Falls back to the scenario name
	// for an unannotated/legacy scenario execution (enrichScenarioExecutions).
	Setup string `json:"setup,omitempty"`
	// Experiment is the comparison GROUP (ScenarioConfig.Experiment) — only
	// executions sharing one Experiment value may be pooled into one matrix. Empty
	// for extraction (that family's comparison group is implicit: one case's own
	// prompt set) and for an unannotated/legacy scenario execution — empty means
	// "never auto-compared against anything," the safer default.
	Experiment string `json:"experiment,omitempty"`
	// Prompt is the ACTUAL prompt/frame used. Scenario executions populate it from
	// ScenarioConfig.PromptRef when set (enrichScenarioExecutions), falling back to
	// Name=scenario name/Version=0 (this field's pre-existing zero-value display);
	// extraction already always sets it (SHA256 included, from provenance.LoadPrompt).
	Prompt       provenance.PromptRef `json:"prompt,omitempty"`
	Preprocessor string               `json:"preprocessor,omitempty"`
}

// VOutput carries the raw model output and its parse state.
type VOutput struct {
	Raw     string `json:"raw,omitempty"`
	ParseOK bool   `json:"parse_ok"`
	// ReplyText is the model's OWN, pre-injection reply_text field, extracted from Raw
	// when it parses — the "LLM reply" column in the comparison UI, shown alongside
	// VScenarioDetails.InjectedText ("after injection") so the two read as visibly
	// different things: what the model wrote vs. what a customer would actually
	// receive once fact tokens are substituted. Empty when Raw doesn't parse, or for
	// the extraction family (no reply_text field there — Extract.Summary/
	// ExtractedText play that role instead).
	ReplyText  string `json:"reply_text,omitempty"`
	ParseError string `json:"parse_error,omitempty"`
	Error      string `json:"error,omitempty"`
	// RawHasReasoningMarkers flags Raw as containing a <think>/<thinking> tag marker —
	// a visibility flag for the HTML viewer's raw-output panel, NOT a redaction: Raw's
	// whole job is showing the unfiltered response for debugging in what is an internal
	// eval-reviewer artifact, not a real customer-facing surface (the hard gate against
	// reasoning leaking into an actual customer-facing field is judge.go's
	// Verdict.ReasoningLeak / VScenarioDetails.ReasoningLeak, which scans reply_text).
	RawHasReasoningMarkers bool `json:"raw_has_reasoning_markers,omitempty"`
	// Reasoning is the model's separate reasoning/thinking content, captured
	// independently of Raw (never concatenated into it) — surfaced here so a value
	// that's actually captured (Verdict.Reasoning / extractRunResult.Reasoning) isn't
	// silently invisible to every downstream report/viewer, same debug-only status as Raw.
	Reasoning string `json:"reasoning,omitempty"`
	// Final/NonFinal/ExtractionMethod mirror Verdict.FinalOutput/NonFinalOutput/
	// ExtractionMethod: the exact final customer-response JSON that was judged, the
	// reasoning/preamble text separated off it (debug-only, never scored), and how the
	// final answer was located. All empty when the row never parsed or predates
	// extraction-aware judging — Raw stays the complete provider response either way.
	Final            string `json:"final,omitempty"`
	NonFinal         string `json:"non_final,omitempty"`
	ExtractionMethod string `json:"extraction_method,omitempty"`
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
	FinishReason    string   `json:"finish_reason,omitempty"`
	Truncated       bool     `json:"truncated"`
	ReasoningLeak   bool     `json:"reasoning_leak"`
	// MediaCount/TooManyMedia/MediaCountEvaluated mirror the same fields on judge.go's
	// Verdict — see MediaCountEvaluated's doc comment there for why a verdict predating
	// this check must never render as a silent pass.
	MediaCount          int  `json:"media_count,omitempty"`
	TooManyMedia        bool `json:"too_many_media,omitempty"`
	MediaCountEvaluated bool `json:"media_count_evaluated,omitempty"`
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
	Family  string    `json:"family"` // "scenario" | "extract"
	Subject VSubject  `json:"subject"`
	Variant VVariant  `json:"variant"`
	Output  VOutput   `json:"output"`
	Scores  []VScore  `json:"scores"`
	Rollups []VRollup `json:"rollups"`
	// Contract is the Requirements-panel projection of Scores/Rollups above — see
	// VContractRow's doc comment. Populated in two passes: family-general safety rows
	// at build time (scenarioExecutionFromVerdict / extractExecutionFromResult, always
	// available, no snapshot needed), then test/case-specific requirement rows
	// PREPENDED by the enrich step once this run's own snapshot is loaded
	// (enrichScenarioExecutions / enrichExtractExecutions) — absent entirely only when
	// no snapshot exists at all (a run from before snapshotting existed).
	Contract  []VContractRow `json:"contract,omitempty"`
	Cost      VCost          `json:"cost"`
	LatencyMs int            `json:"latency_ms,omitempty"`
	// Retries mirrors Verdict.Retries (judge.go) — how many times retry.go retried this
	// row. 0 for every execution the retry path never touched, including every
	// execution from before this field existed. Lets the frontend show a "повтор ×N"
	// badge next to a recovered row instead of it looking identical to a clean
	// first-attempt pass.
	Retries int `json:"retries,omitempty"`

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

	// finish_reason_ok is evaluated INDEPENDENTLY of parsing (row.Response.FinishReason
	// is available regardless of whether the model's content parsed as JSON — and
	// truncation is often the very CAUSE of a parse failure), so it deliberately does
	// NOT go through the evaluated() gate above: routing it through evaluated() would
	// wrongly report not_run on every parse failure, exactly the "reported not_run as
	// if never evaluated" bug class evaluated() itself exists to prevent for every OTHER
	// check. Follows parse_ok's own direct-status pattern instead.
	finishReasonStatus := ScorePass
	if v.Truncated {
		finishReasonStatus = ScoreFail
	}

	// media_count is evaluated independently of the generic evaluated() gate above:
	// MediaCountEvaluated is a CODE-VERSION marker, not just "did parsing succeed" — a
	// verdict from before this check existed has ParseOK=true (old code fully judged it)
	// but MediaCountEvaluated=false (the field didn't exist to be set), and must render as
	// not_run, not as a fabricated pass just because TooManyMedia's zero value is false.
	// See MediaCountEvaluated's own doc comment on Verdict (judge.go).
	mediaCountStatus := ScoreNotRun
	mediaCountDetail := ""
	if v.MediaCountEvaluated {
		mediaCountStatus = ScorePass
		if v.TooManyMedia {
			mediaCountStatus = ScoreFail
			mediaCountDetail = fmt.Sprintf("%d attachments", v.MediaCount)
		}
	}

	// outcomes follows the media_count pattern (OutcomesDeclared is the same kind of
	// marker as MediaCountEvaluated): a verdict judged before the check existed — or a
	// test that declares no alternative outcomes — renders not_run, never a fabricated
	// verdict from OutcomesPass's zero value. Detail names the matched alternative on a
	// pass and the full declared set on a fail (MustContainAnyExpected's reasoning: a
	// failed OR has no single culprit).
	outcomesStatus := ScoreNotRun
	outcomesDetail := ""
	if v.OutcomesDeclared {
		outcomesStatus = ScorePass
		outcomesDetail = v.OutcomeMatched
		if !v.OutcomesPass {
			outcomesStatus = ScoreFail
			outcomesDetail = strings.Join(v.OutcomeLabels, " | ")
		}
	}

	// llm_checks follows the same not_run-by-default pattern as media_count/outcomes:
	// LLMCheckEvaluated is an opt-in marker (only the separate judge-llm command ever
	// sets it), so a verdict never touched by judge-llm renders not_run, never a
	// fabricated pass from LLMJudgePass's nil/zero value. Detail lists the claim(s) that
	// failed or came back unverified, mirroring OutcomesPass's "name what went wrong"
	// convention.
	llmChecksStatus := ScoreNotRun
	llmChecksDetail := ""
	if v.LLMCheckEvaluated {
		llmChecksStatus = ScorePass
		if v.LLMJudgePass == nil || !*v.LLMJudgePass {
			llmChecksStatus = ScoreFail
		}
		var issues []string
		for _, r := range v.LLMChecks {
			if r.Unverified {
				issues = append(issues, r.Claim+" (unverified)")
			} else if !r.Pass {
				issues = append(issues, r.Claim)
			}
		}
		llmChecksDetail = strings.Join(issues, " | ")
	}

	scores := []VScore{
		{Name: "parse_ok", Status: parseStatus, Detail: parseDetail},
		{Name: "finish_reason_ok", Status: finishReasonStatus, Detail: v.FinishReason},
		{Name: "contract_fields", Status: evaluated(v.ContractFields)},
		{Name: "no_unknown_tokens", Status: evaluated(len(v.UnknownTokens) == 0), Detail: strings.Join(v.UnknownTokens, ", ")},
		{Name: "no_leftover_braces", Status: evaluated(!v.LeftoverBraces)},
		{Name: "no_reasoning_leak", Status: evaluated(!v.ReasoningLeak)},
		{Name: "requires", Status: evaluated(v.RequiresPass)},
		{Name: "forbid_tokens", Status: evaluated(v.ForbidTokensPass), Detail: strings.Join(v.ForbiddenTokensHit, ", ")},
		{Name: "media", Status: evaluated(v.MediaPass)},
		{Name: "escalate", Status: evaluated(v.EscalatePass)},
		{Name: "language", Status: evaluated(v.LanguagePass), Detail: v.LanguageIssue},
		// Reported separately from the combined "language" row above (Phase 0.4): looksKazakh
		// is a cheap presence heuristic, not a whole-reply classifier — telling "the text
		// didn't look Kazakh" apart from "the model declared the wrong reply_language field"
		// matters when manually inspecting a Kazakh canary run's actual replies.
		{Name: "language_text_ok", Status: evaluated(v.LanguageTextOK)},
		{Name: "language_field_ok", Status: evaluated(v.LanguageFieldOK)},
		// language_alias_used is informational, not gating — it never affects
		// LanguagePass itself. It flags when a pass only happened because
		// normalizeLangCode aliased the model's declared code (e.g. "kz" -> "kk"), so a
		// reviewer can tell the two apart without diffing raw_output. Status is a plain
		// bool render (true = alias was used), not a pass/fail evaluation, hence no
		// evaluated() wrapper — ScorePass here means "alias was used," not "check passed."
		{Name: "language_alias_used", Status: evaluated(v.LanguageAliasUsed)},
		{Name: "no_control_chars", Status: evaluated(!v.ControlChars), Detail: v.ControlCharsIssue},
		{Name: "must_not_contain", Status: evaluated(v.MustNotContainPass), Detail: v.ForbiddenPhrase},
		{Name: "must_contain_any", Status: evaluated(v.MustContainAnyPass), Detail: strings.Join(v.MustContainAnyExpected, ", ")},
		{Name: "outcomes", Status: outcomesStatus, Detail: outcomesDetail},
		{Name: "llm_checks", Status: llmChecksStatus, Detail: llmChecksDetail},
		{Name: "no_invented_digits", Status: evaluated(len(v.InventedDigits) == 0), Detail: strings.Join(v.InventedDigits, ", ")},
		{Name: "no_unit_issues", Status: evaluated(len(v.UnitIssues) == 0), Detail: strings.Join(v.UnitIssues, ", ")},
		{Name: "no_unknown_media", Status: evaluated(len(v.UnknownMedia) == 0), Detail: strings.Join(v.UnknownMedia, ", ")},
		{Name: "media_count", Status: mediaCountStatus, Detail: mediaCountDetail},
	}

	rollups := []VRollup{
		{Key: "contract_pass", Label: "Contract pass", Pass: v.ContractPass},
		{Key: "model_behavior_pass", Label: "Model-behavior pass", Pass: v.ModelBehaviorPass},
	}

	// Universal safety rows — same expectation on every scenario execution regardless
	// of what the test itself declares, so built here (unconditionally) rather than in
	// the snapshot-dependent enrich step. Values are read back off `scores` above, not
	// recomputed, so this can never disagree with the Scores list sitting right next to it.
	placeholdersOK, _ := scoreByName(scores, "no_unknown_tokens")
	bracesOK, _ := scoreByName(scores, "no_leftover_braces")
	placeholderActual := "ок"
	if placeholdersOK.Detail != "" {
		placeholderActual = "неизвестные токены: " + placeholdersOK.Detail
	} else if bracesOK.Status == ScoreFail {
		placeholderActual = "в ответе осталась фигурная скобка после подстановки"
	}
	var placeholderPass *bool
	if placeholdersOK.Status != ScoreNotRun {
		b := placeholdersOK.Status == ScorePass && bracesOK.Status == ScorePass
		placeholderPass = &b
	}
	jsonActual := "разобран корректно"
	if !v.ParseOK {
		jsonActual = v.Reason
	}
	mediaOK, _ := scoreByName(scores, "no_unknown_media")
	mediaActual := "ок"
	if mediaOK.Detail != "" {
		mediaActual = "неизвестные ссылки: " + mediaOK.Detail
	}
	digitsOK, _ := scoreByName(scores, "no_invented_digits")
	digitsActual := "не найдены"
	if digitsOK.Detail != "" {
		digitsActual = digitsOK.Detail
	}
	contractFieldsOK, _ := scoreByName(scores, "contract_fields")
	contractFieldsActual := "ок"
	if contractFieldsOK.Status == ScoreFail {
		contractFieldsActual = "структура ответа не соответствует контракту"
	}
	mediaCountRow, _ := scoreByName(scores, "media_count")
	mediaCountActual := "ок"
	switch {
	case mediaCountRow.Status == ScoreNotRun:
		mediaCountActual = "не проверялось (verdict до добавления этой проверки)"
	case mediaCountRow.Detail != "":
		mediaCountActual = mediaCountRow.Detail
	}
	controlCharsRow, _ := scoreByName(scores, "no_control_chars")
	controlCharsActual := "ок"
	if controlCharsRow.Detail != "" {
		controlCharsActual = controlCharsRow.Detail
	}
	safetyContract := []VContractRow{
		{Key: "valid_json", Label: "Валидный JSON", Kind: "safety", Expected: "корректный JSON-объект", Actual: jsonActual, Pass: scorePassPtr(parseStatus)},
		{Key: "contract_fields", Label: "Поля контракта", Kind: "safety", Expected: "все обязательные поля присутствуют", Actual: contractFieldsActual, Pass: scorePassPtr(contractFieldsOK.Status)},
		{Key: "no_unresolved_placeholders", Label: "Без неразрешённых плейсхолдеров", Kind: "safety", Expected: "нет неизвестных токенов, ответ не заблокирован", Actual: placeholderActual, Pass: placeholderPass},
		{Key: "no_unknown_media", Label: "Валидные ссылки на медиа", Kind: "safety", Expected: "все ссылки существуют в каталоге", Actual: mediaActual, Pass: scorePassPtr(mediaOK.Status)},
		{Key: "no_invented_digits", Label: "Без выдуманных чисел", Kind: "safety", Expected: "нет чисел вне ответа модели", Actual: digitsActual, Pass: scorePassPtr(digitsOK.Status)},
		{Key: "media_count", Label: "Не больше вложений, чем разрешает промпт", Kind: "safety", Expected: "не более 3 ссылок / 2 групп (правило 3 фрейма)", Actual: mediaCountActual, Pass: scorePassPtr(mediaCountRow.Status)},
		{Key: "no_control_chars", Label: "Без управляющих символов", Kind: "safety", Expected: "нет служебных байтов (например backspace) в reply_text", Actual: controlCharsActual, Pass: scorePassPtr(controlCharsRow.Status)},
	}

	// ReplyText re-parses the judged output the same way judgeOne itself did — a second
	// parse of already-validated JSON, not a new trust boundary; empty (not an error)
	// when ParseOK is false, matching every other "not evaluated" field on this struct.
	// FinalOutput (the extracted final answer) is preferred when set; RawOutput remains
	// the fallback for verdicts judged before extraction existed.
	var replyText string
	if obj, ok := parseModelJSON(judgedOutput(v)); ok {
		replyText, _ = obj["reply_text"].(string)
	}

	return VExecution{
		Family:  "scenario",
		Subject: VSubject{Scenario: scenario, TestID: v.TestID, Message: v.Message},
		Variant: VVariant{Model: v.Model},
		Output: VOutput{
			Raw:                    v.RawOutput,
			ParseOK:                v.ParseOK,
			ReplyText:              replyText,
			RawHasReasoningMarkers: evaltext.HasReasoningMarkers(v.RawOutput),
			Reasoning:              v.Reasoning,
			Final:                  v.FinalOutput,
			NonFinal:               v.NonFinalOutput,
			ExtractionMethod:       v.ExtractionMethod,
		},
		Scores:    scores,
		Rollups:   rollups,
		Contract:  safetyContract,
		Cost:      VCost{TokensIn: v.TokensIn, TokensOut: v.TokensOut, EstimateUSD: v.CostEstimateUSD, Basis: v.CostBasis},
		LatencyMs: v.LatencyMs,
		Retries:   v.Retries,
		Scenario: &VScenarioDetails{
			InjectedText:        v.InjectedText,
			UnknownTokens:       v.UnknownTokens,
			UnknownMedia:        v.UnknownMedia,
			InventedDigits:      v.InventedDigits,
			UnitIssues:          v.UnitIssues,
			ForbiddenPhrase:     v.ForbiddenPhrase,
			Blocked:             v.Blocked,
			LeftoverBraces:      v.LeftoverBraces,
			FinishReason:        v.FinishReason,
			Truncated:           v.Truncated,
			ReasoningLeak:       v.ReasoningLeak,
			MediaCount:          v.MediaCount,
			TooManyMedia:        v.TooManyMedia,
			MediaCountEvaluated: v.MediaCountEvaluated,
		},
	}
}

// judgedOutput returns the output text a verdict was actually judged from: the
// extracted final answer when extraction ran, else the raw provider response
// (legacy verdicts and rows that never parsed).
func judgedOutput(v Verdict) string {
	if v.FinalOutput != "" {
		return v.FinalOutput
	}
	return v.RawOutput
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

	// Universal safety rows — same expectation on every extraction execution regardless
	// of what the case itself declares, so built here unconditionally (works even when
	// Parsed is nil: an error/parse-failure execution still gets an honest "Валидный
	// JSON" fail row instead of no Requirements panel at all). no_reasoning_leak
	// always runs per extract_checks.go (never opt-in), so it's read straight off
	// Checks here rather than duplicated by the case-specific row builder below.
	jsonActual := "разобран корректно"
	jsonPass := r.Parsed != nil
	if r.Error != "" {
		jsonActual = "ошибка вызова: " + r.Error
	} else if r.ParseError != "" {
		jsonActual = "не разобран: " + r.ParseError
	}
	safetyContract := []VContractRow{
		{Key: "valid_json", Label: "Валидный JSON", Kind: "safety", Expected: "корректный JSON-объект", Actual: jsonActual, Pass: &jsonPass},
	}
	if leak, ok := scoreByName(toVScores(r.Checks), "no_reasoning_leak"); ok {
		safetyContract = append(safetyContract, VContractRow{
			Key: "no_reasoning_leak", Label: "Без утечки рассуждений", Kind: "safety",
			Expected: "нет <think>/<thinking> меток в тексте", Actual: leakActual(leak), Pass: scorePassPtr(leak.Status),
		})
	}

	return VExecution{
		Family:  "extract",
		Subject: VSubject{CaseID: r.CaseID, InputRef: filepath.Join("inputs", r.CaseID+".jpg")},
		// Setup = the prompt ref string ("extract@v1") — extraction's comparison
		// column IS its prompt version, unlike chat's setup/prompt split (there is no
		// routed-strategy concept here). Experiment deliberately left empty: every
		// case's own prompt set is its own implicit comparison group.
		Variant: VVariant{Model: r.Model, Setup: r.Prompt.String(), Prompt: r.Prompt, Preprocessor: r.Preprocessor},
		Output: VOutput{
			Raw:                    r.Raw,
			ParseOK:                r.Parsed != nil,
			ParseError:             r.ParseError,
			Error:                  r.Error,
			RawHasReasoningMarkers: evaltext.HasReasoningMarkers(r.Raw),
			Reasoning:              r.Reasoning,
		},
		Scores: scores,
		Rollups: []VRollup{
			{Key: "all_checks_pass", Label: "All checks pass", Pass: allChecksPass(r.Checks)},
		},
		Contract: safetyContract,
		Cost:     VCost{TokensIn: r.Usage.PromptTokens, TokensOut: r.Usage.CompletionTokens, EstimateUSD: r.Cost, Basis: r.CostBasis},
		Extract:  extractDetails,
	}
}

// toVScores adapts extract_checks.go's CheckResult list to VScore, matching what
// scores above already computed — a tiny local helper so safety-row lookups can reuse
// scoreByName instead of a bespoke linear scan for one name.
func toVScores(checks []CheckResult) []VScore {
	out := make([]VScore, len(checks))
	for i, c := range checks {
		st := ScoreFail
		if c.Pass {
			st = ScorePass
		}
		out[i] = VScore{Name: c.Name, Status: st, Detail: c.Detail}
	}
	return out
}

func leakActual(s VScore) string {
	if s.Status == ScorePass {
		return "не найдено"
	}
	return s.Detail
}

// extractFieldLabel gives a Russian label to each extract_checks.go "field:<key>"
// check name — key must stay in sync with fieldValue's own switch (extract_checks.go).
func extractFieldLabel(key string) string {
	switch key {
	case "content_kind":
		return "Тип контента"
	case "summary":
		return "Описание"
	case "extracted_text":
		return "Извлечённый текст"
	case "language":
		return "Язык"
	case "visibility_suggestion":
		return "Видимость"
	case "media_role_hint":
		return "Роль медиа"
	case "relates_to_hint":
		return "Связано с"
	default:
		return key
	}
}

// extractFieldActual mirrors fieldValue (extract_checks.go) but reads off the already-
// adapted VExtractDetails instead of the original ExtractionResult — same field set,
// display-only.
func extractFieldActual(ext *VExtractDetails, key string) string {
	if ext == nil {
		return "—"
	}
	switch key {
	case "content_kind":
		return ext.ContentKind
	case "summary":
		return ext.Summary
	case "extracted_text":
		return ext.ExtractedText
	case "language":
		return ext.Language
	case "visibility_suggestion":
		return ext.VisibilitySuggestion
	case "media_role_hint":
		return ext.MediaRoleHint
	case "relates_to_hint":
		return ext.RelatesToHint
	default:
		return "—"
	}
}

func sortedFieldKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// extractListRow builds one row for a "does the text contain every/any of these
// phrases" check — Detail already carries the exact miss/hit list runExtractChecks
// computed (extract_checks.go's containsAllCheck/containsAnyCheck), reused verbatim as
// Actual on failure so this can never disagree with the persisted verdict.
func extractListRow(key, label string, scores []VScore, wants []string, passActual string) VContractRow {
	score, _ := scoreByName(scores, key)
	actual := passActual
	if score.Detail != "" {
		actual = score.Detail
	}
	return VContractRow{
		Key: key, Label: label, Kind: "requirement",
		Expected: strings.Join(wants, ", "), Actual: actual, Pass: scorePassPtr(score.Status),
	}
}

// extractRequirementRows builds the case-specific Requirements-panel rows for one
// extraction execution — one row per check the CASE ITSELF declares
// (extract/cases.yaml), skipped entirely when the case doesn't declare it (matching
// runExtractChecks's own "only run what's declared" gating, except no_invented_numbers
// which — like in runExtractChecks — always runs). Pass values are read back from
// exec's own already-computed Scores, never recomputed.
func extractRequirementRows(exec VExecution, c ExtractCase) []VContractRow {
	var rows []VContractRow
	for _, key := range sortedFieldKeys(c.Fields) {
		name := "field:" + key
		score, ok := scoreByName(exec.Scores, name)
		if !ok {
			continue
		}
		rows = append(rows, VContractRow{
			Key: name, Label: extractFieldLabel(key), Kind: "requirement",
			Expected: c.Fields[key], Actual: extractFieldActual(exec.Extract, key), Pass: scorePassPtr(score.Status),
		})
	}
	if len(c.TextContainsAll) > 0 {
		rows = append(rows, extractListRow("text_contains_all", "Видимый текст (обязательные фразы)", exec.Scores, c.TextContainsAll, "все найдены"))
	}
	if len(c.IdentifyContainsAll) > 0 {
		rows = append(rows, extractListRow("identify_contains_all", "Описание/тема (обязательно)", exec.Scores, c.IdentifyContainsAll, "все найдены"))
	}
	if len(c.IdentifyContainsAny) > 0 {
		rows = append(rows, extractListRow("identify_contains_any", "Описание/тема (любое из)", exec.Scores, c.IdentifyContainsAny, "найдено"))
	}
	// Expected is genuinely case-specific (the allowed list), so — despite always
	// running, same as no_reasoning_leak above — this is a requirement row, not a
	// family-general safety row.
	if score, ok := scoreByName(exec.Scores, "no_invented_numbers"); ok {
		expected := "числа не ожидаются"
		if len(c.AllowedNumbers) > 0 {
			expected = strings.Join(c.AllowedNumbers, ", ")
		}
		actual := "нет посторонних чисел"
		if score.Detail != "" {
			actual = score.Detail
		}
		rows = append(rows, VContractRow{
			Key: "no_invented_numbers", Label: "Разрешённые числа", Kind: "requirement",
			Expected: expected, Actual: actual, Pass: scorePassPtr(score.Status),
		})
	}
	if len(c.RequiredNumbers) > 0 {
		rows = append(rows, extractListRow("required_numbers", "Обязательные числа", exec.Scores, c.RequiredNumbers, "все найдены"))
	}
	if c.ForbidCurrency {
		if score, ok := scoreByName(exec.Scores, "forbid_currency"); ok {
			actual := "нет"
			if score.Detail != "" {
				actual = score.Detail
			}
			rows = append(rows, VContractRow{
				Key: "forbid_currency", Label: "Запрет валюты", Kind: "requirement",
				Expected: "упоминаний валюты быть не должно", Actual: actual, Pass: scorePassPtr(score.Status),
			})
		}
	}
	return rows
}

// enrichExtractExecutions adds case-specific Requirements-panel rows sourced entirely
// from this run's OWN snapshotted extract_cases.yaml (never the live, possibly-since-
// edited evals/extract/cases.yaml) — same "read the run's own snapshot" discipline as
// enrichScenarioExecutions. A run from before case-snapshotting existed, or whose
// snapshot fails to load, keeps only the safety rows extractExecutionFromResult already
// attached — not an error, just fewer rows than a fully-snapshotted run.
func enrichExtractExecutions(runDir string, execs []VExecution) []VExecution {
	cf, err := loadExtractCases(filepath.Join(runDir, "snapshots", "extract_cases.yaml"))
	if err != nil {
		return execs
	}
	caseByID := map[string]ExtractCase{}
	for _, c := range cf.Cases {
		caseByID[c.ID] = c
	}
	for i := range execs {
		if c, ok := caseByID[execs[i].Subject.CaseID]; ok {
			// Requirement rows go BEFORE the safety rows already attached — same
			// "test-specific rows + universal safety rows" order as the scenario side.
			execs[i].Contract = append(extractRequirementRows(execs[i], c), execs[i].Contract...)
		}
	}
	return execs
}

// loadScenarioExecutions reads every *.judged.json in runDir, adapts each verdict, and
// enriches the result with this run's own snapshotted scenario metadata (setup,
// experiment, prompt ref, conversation history — see enrichScenarioExecutions). Absent
// entirely for an extraction-only run — that's fine, the caller combines both.
func loadScenarioExecutions(runDir string) ([]VExecution, error) {
	files, err := filepath.Glob(filepath.Join(runDir, "*.judged.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	// promptSHAByScenario reads this run's OWN manifest (never the live,
	// possibly-since-changed generated/prompt.txt) for the exact rendered-prompt hash
	// per scenario — best-effort: a run from before provenance snapshotting existed,
	// or a standalone `judge` with no manifest, simply attaches no SHA, not an error.
	promptSHAByScenario := map[string]string{}
	var manifest provenance.Manifest
	if err := readJSON(filepath.Join(runDir, "manifest.json"), &manifest); err == nil {
		for _, sref := range manifest.Scenarios {
			promptSHAByScenario[sref.Scenario] = sref.PromptSHA256
		}
	}

	var out []VExecution
	for _, f := range files {
		var jr JudgedRun
		if err := readJSON(f, &jr); err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		execs := scenarioExecutionsFromJudgedRun(jr)
		execs = enrichScenarioExecutions(runDir, jr.Scenario, promptSHAByScenario[jr.Scenario], execs)
		out = append(out, execs...)
	}
	return out, nil
}

// enrichScenarioExecutions adds setup/experiment/prompt-ref/history context that
// scenarioExecutionFromVerdict alone can't know (it only sees one Verdict, not the
// scenario's own config or the customer's simulated conversation history) — sourced
// entirely from this run's OWN snapshot (provenance.SnapshotDirFor), never the live,
// possibly-since-changed scenarios/*/generated/. Legacy runs / scenarios with no
// snapshot, or a scenario.yaml with no setup/prompt_ref annotation, fall back to the
// scenario name for both Setup and Prompt.Name — the same value shown before this
// enrichment existed, so nothing regresses for unannotated data.
func enrichScenarioExecutions(runDir, scenario, promptSHA256 string, execs []VExecution) []VExecution {
	snapDir, ok := provenance.SnapshotDirFor(runDir, scenario)
	if !ok {
		return execs // legacy run — nothing to enrich with
	}
	sc, err := loadScenario(snapDir) // scenario.yaml is one of the 5 files SnapshotScenario copies
	if err != nil {
		return execs // snapshot dir exists but scenario.yaml is unreadable — fail soft
	}

	setup := sc.Setup
	if setup == "" {
		setup = scenario
	}
	promptRef := provenance.PromptRef{Name: scenario} // fallback: unversioned, scenario-named
	if sc.PromptRef != "" {
		if name, version, perr := provenance.ParsePromptSpec(sc.PromptRef); perr == nil {
			promptRef = provenance.PromptRef{Name: name, Version: version, SHA256: promptSHA256}
		}
	}

	historyByTestID := map[string][]HistoryTurn{}
	testByID := map[string]TestCase{}
	var resolved ResolvedTests
	if err := readJSON(filepath.Join(snapDir, "resolved_tests.json"), &resolved); err == nil {
		for _, tc := range resolved.Tests {
			testByID[tc.ID] = tc
			if len(tc.History) > 0 {
				historyByTestID[tc.ID] = tc.History
			}
		}
	}

	for i := range execs {
		execs[i].Variant.Setup = setup
		execs[i].Variant.Experiment = sc.Experiment
		execs[i].Variant.Prompt = promptRef
		if h, ok := historyByTestID[execs[i].Subject.TestID]; ok {
			execs[i].Subject.History = h
		}
		// Requirement rows go BEFORE the safety rows scenarioExecutionFromVerdict
		// already attached — "test-specific rows + universal safety rows" order.
		if tc, ok := testByID[execs[i].Subject.TestID]; ok {
			execs[i].Contract = append(scenarioRequirementRows(execs[i], tc), execs[i].Contract...)
		}
	}
	return execs
}

// scenarioRequirementRows builds the test-specific Requirements-panel rows for one
// execution — one row per requirement the TEST ITSELF declares (Requires/Media/
// Escalate/Language/MustNotContain), omitted entirely when the test doesn't declare
// it (a test with no `escalate:` field gets no Эскалация row, not a row showing
// "n/a"). Pass values are read back from `exec`'s own already-computed Scores —
// NEVER recomputed here — only Expected/Actual display strings and row presence are
// new. Actual values for language/escalate/media are read from the model's own parsed
// JSON (the extracted final answer when set, else exec.Output.Raw) — the same judged
// output already persisted with this execution, not a new trust boundary.
func scenarioRequirementRows(exec VExecution, tc TestCase) []VContractRow {
	parseSrc := exec.Output.Raw
	if exec.Output.Final != "" {
		parseSrc = exec.Output.Final
	}
	obj, _ := parseModelJSON(parseSrc)
	var rows []VContractRow

	if len(tc.Requires) > 0 {
		score, _ := scoreByName(exec.Scores, "requires")
		rows = append(rows, VContractRow{
			Key: "requires", Label: "Обязательные факты", Kind: "requirement",
			Expected: requiresExpectedDisplay(tc.Requires),
			Actual:   requiresActualDisplay(tc.Requires, exec.Output.ReplyText),
			Pass:     scorePassPtr(score.Status),
		})
	}
	if tc.Language == "kk" || tc.Language == "ru" {
		score, _ := scoreByName(exec.Scores, "language")
		actual := "—"
		if replyLang, ok := obj["reply_language"].(string); ok && replyLang != "" {
			actual = replyLang
		}
		rows = append(rows, VContractRow{
			Key: "language", Label: "Язык ответа", Kind: "requirement",
			Expected: tc.Language, Actual: actual, Pass: scorePassPtr(score.Status),
		})
	}
	if tc.Escalate != nil {
		score, _ := scoreByName(exec.Scores, "escalate")
		expected := "нет"
		if *tc.Escalate {
			expected = "да"
		}
		actual := "—"
		if b, ok := obj["escalate"].(bool); ok {
			actual = "нет"
			if b {
				actual = "да"
			}
		}
		rows = append(rows, VContractRow{
			Key: "escalate", Label: "Эскалация", Kind: "requirement",
			Expected: expected, Actual: actual, Pass: scorePassPtr(score.Status),
		})
	}
	if tc.Media != nil {
		score, _ := scoreByName(exec.Scores, "media")
		var expectedDisplay string
		if tc.Media.Forbid {
			expectedDisplay = "медиа быть не должно"
		} else {
			var parts []string
			if len(tc.Media.AllOf) > 0 {
				parts = append(parts, strings.Join(tc.Media.AllOf, " И "))
			}
			if len(tc.Media.AnyOf) > 0 {
				parts = append(parts, strings.Join(tc.Media.AnyOf, " ИЛИ "))
			}
			expectedDisplay = strings.Join(parts, " + ")
			if tc.Media.Exclusive {
				expectedDisplay += " (только из этого списка)"
			}
		}
		actual := mediaEntries(obj)
		actualDisplay := "нет"
		if len(actual) > 0 {
			actualDisplay = strings.Join(actual, ", ")
		}
		rows = append(rows, VContractRow{
			Key: "media", Label: "Медиа", Kind: "requirement",
			Expected: expectedDisplay, Actual: actualDisplay, Pass: scorePassPtr(score.Status),
		})
	}
	if len(tc.MustNotContain) > 0 {
		score, _ := scoreByName(exec.Scores, "must_not_contain")
		actual := "нет"
		if score.Detail != "" {
			actual = score.Detail
		}
		rows = append(rows, VContractRow{
			Key: "must_not_contain", Label: "Запрещённые фразы", Kind: "requirement",
			Expected: strings.Join(tc.MustNotContain, ", "), Actual: actual, Pass: scorePassPtr(score.Status),
		})
	}
	if len(tc.MustContainAny) > 0 {
		score, _ := scoreByName(exec.Scores, "must_contain_any")
		actual := "не найдено ни одной"
		if score.Status == ScorePass {
			actual = "найдено"
		}
		rows = append(rows, VContractRow{
			Key: "must_contain_any", Label: "Ожидаемые фразы (любая из)", Kind: "requirement",
			Expected: strings.Join(tc.MustContainAny, " ИЛИ "), Actual: actual, Pass: scorePassPtr(score.Status),
		})
	}
	if len(tc.Outcomes) > 0 {
		score, _ := scoreByName(exec.Scores, "outcomes")
		labels := make([]string, len(tc.Outcomes))
		for i, oc := range tc.Outcomes {
			labels[i] = oc.Label
		}
		// On a pass the score's Detail carries the matched block's label (see the scores
		// build); on a fail it carries the full declared set, but the row's Expected
		// already lists that, so the Actual cell states the miss in words instead.
		actual := "ни один исход не выполнен"
		if score.Status == ScorePass && score.Detail != "" {
			actual = score.Detail
		}
		rows = append(rows, VContractRow{
			Key: "outcomes", Label: "Допустимые исходы (любой из)", Kind: "requirement",
			Expected: strings.Join(labels, " ИЛИ "), Actual: actual, Pass: scorePassPtr(score.Status),
		})
	}
	return rows
}

// requiresExpectedDisplay renders TestCase.Requires (an AND of OR-groups of alternative
// token refs — see requiresSatisfied in judge.go) as one readable line.
func requiresExpectedDisplay(groups [][]string) string {
	parts := make([]string, len(groups))
	for i, g := range groups {
		parts[i] = strings.Join(g, " ИЛИ ")
	}
	return strings.Join(parts, "; ")
}

// requiresActualDisplay reports, per OR-group, which token (if any) the model's own
// pre-injection reply actually used — a direct substring scan for the literal
// "{{token}}" span, the same literal requiresSatisfied itself checks for.
func requiresActualDisplay(groups [][]string, replyText string) string {
	parts := make([]string, len(groups))
	for i, g := range groups {
		found := ""
		for _, tok := range g {
			if strings.Contains(replyText, "{{"+tok+"}}") {
				found = tok
				break
			}
		}
		if found == "" {
			parts[i] = "ни один из: " + strings.Join(g, " | ")
		} else {
			parts[i] = found
		}
	}
	return strings.Join(parts, "; ")
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
	return enrichExtractExecutions(runDir, out), nil
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
