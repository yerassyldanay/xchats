package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"xchats-evals-harness/internal/evaltext"
	"xchats-evals-harness/internal/provenance"
)

// PromptfooResults mirrors the shape of promptfoo's -o results.json — only the fields
// judge.go actually reads.
type PromptfooResults struct {
	Results struct {
		Results []PromptfooRow `json:"results"`
	} `json:"results"`
}

type PromptfooRow struct {
	Provider struct {
		ID string `json:"id"`
		// Label disambiguates two provider entries sharing the same ID (see
		// ModelProvider.Label in types.go) — e.g. a reasoning-on/reasoning-off pair of
		// the same underlying model, compared side by side in one run.
		Label string `json:"label"`
	} `json:"provider"`
	TestCase struct {
		Description string `json:"description"`
	} `json:"testCase"`
	// Prompt.Raw is promptfoo's own record of the EXACT rendered prompt string this
	// row's call sent — confirmed present in real results.json (row.prompt.raw). This
	// is the retry path's (retry.go) prompt source: reusing it means a retried call
	// sends byte-identical input to the original, no Nunjucks/snapshot reconstruction
	// needed.
	Prompt struct {
		Raw string `json:"raw"`
	} `json:"prompt"`
	Response struct {
		Output string `json:"output"`
		Cached bool   `json:"cached"`
		// FinishReason mirrors promptfoo's own response.finishReason (confirmed present
		// in real promptfoo output, sibling to Output/Cached — see evals/results/results.json,
		// a committed real run). "length" (OpenAI/OpenRouter's canonical truncation
		// signal) is normalized into a hard ContractPass failure by isTruncatedFinish —
		// a truncated response is a pipeline-safety issue, the same category as an
		// unknown token or a leftover brace, regardless of what the model said. Empty
		// string means "not reported" (older data, or a provider that doesn't send one)
		// and must NEVER be treated as truncation.
		FinishReason string `json:"finishReason"`
		// Reasoning is OpenRouter's own message.reasoning response field, IF promptfoo
		// ever surfaces it in results.json — not independently confirmed present in this
		// repo (only FinishReason is confirmed; see the comment above). Harmless if
		// promptfoo never populates it. Never folded into Output/RawOutput — see
		// judge.go's ReasoningLeak check, which scans the model's OWN reply_text field
		// instead, the actual customer-facing leak path.
		Reasoning string `json:"reasoning,omitempty"`
	} `json:"response"`
	Cost      float64 `json:"cost"`
	LatencyMs int     `json:"latencyMs"`
	// TokenUsage.Prompt/Completion are ONLY populated by promptfoo on a fresh (non-cached)
	// call — a cached row reports prompt:0, completion:0, cached:<total> instead. Confirmed
	// against a real stored run's results.json; costEstimate below depends on this exact
	// shape, see its comment.
	TokenUsage struct {
		Total      int `json:"total"`
		Prompt     int `json:"prompt"`
		Completion int `json:"completion"`
		Cached     int `json:"cached"`
	} `json:"tokenUsage"`
	// Retries is how many retry attempts retry.go has already made for this row (0 for
	// every row that has never gone through the retry path, including every row from
	// before this field existed — legacy JSON simply lacks the key, which unmarshals as
	// 0). retry.go's row predicate is `!parseModelJSON(...) && Retries == 0` — checking
	// this field is what makes re-running `harness retry` against an already-repaired
	// derivative a no-op instead of spending again.
	Retries int `json:"retries,omitempty"`
	// Attempts preserves EVERY attempt (original + each retry) for a row retry.go has
	// touched — never overwritten, so first-attempt reliability stays inspectable even
	// after a later attempt is selected as this row's reported output. Empty for every
	// row retry.go has never touched.
	Attempts []ResultAttempt `json:"attempts,omitempty"`
	// SelectedAttempt indexes which entry in Attempts this row's top-level
	// response.output/finishReason/tokenUsage were copied from — 0 means "the retry
	// call failed, the ORIGINAL attempt stayed selected" (see retry.go's HTTP-error
	// handling), not "no retry happened" (that case has Attempts entirely empty).
	SelectedAttempt int `json:"selected_attempt,omitempty"`
}

// ResultAttempt is one attempt (the original promptfoo call, or a later retry.go
// retry) at one results.json row — see PromptfooRow.Attempts. Every field the API
// actually reports is captured here, not just output/tokens/latency, so an attempt
// record is a complete account of what that specific call returned and under which
// exact model config.
type ResultAttempt struct {
	Output             string  `json:"output"`
	FinishReason       string  `json:"finishReason"`
	NativeFinishReason string  `json:"nativeFinishReason,omitempty"`
	ResponseID         string  `json:"responseId,omitempty"`
	UpstreamProvider   string  `json:"upstreamProvider,omitempty"`
	ReasoningTokens    int     `json:"reasoningTokens,omitempty"`
	ReportedCostUSD    float64 `json:"reportedCostUsd,omitempty"`
	TokenUsage         struct {
		Total      int `json:"total"`
		Prompt     int `json:"prompt"`
		Completion int `json:"completion"`
	} `json:"tokenUsage"`
	LatencyMs int `json:"latencyMs"`
	// ModelConfigSHA256 ties this SPECIFIC attempt to the exact models.yaml content
	// that produced it — the original attempt's is the PARENT run's snapshotted
	// models.yaml hash; a retry attempt's is the RETRY config's hash (see
	// provenance.Manifest's ParentModelsSHA256/RetryModelsSHA256 for the run-level
	// analogue of this same fact).
	ModelConfigSHA256 string `json:"modelConfigSHA256"`
	// Error is set instead of Output/FinishReason when this attempt's HTTP call itself
	// failed (network error, non-200, unparseable response body) — retry.go's
	// HTTP-error handling keeps the ORIGINAL attempt selected in that case, but still
	// records the failed retry attempt here so it's visible one happened.
	Error string `json:"error,omitempty"`
}

// Verdict is one judged answer — the harness's contribution on top of promptfoo's own
// (now-trivial) pass/fail. See PLAYGROUND.md: "Promptfoo grades model behavior. The
// harness grades the contract." Both dimensions live here, kept explicitly separate.
type Verdict struct {
	TestID  string `json:"test_id"`
	Model   string `json:"model"`
	Message string `json:"message"`

	RawOutput    string `json:"raw_output"`
	InjectedText string `json:"injected_text"`

	ParseOK        bool     `json:"parse_ok"`
	ContractFields bool     `json:"contract_fields_ok"`
	UnknownTokens  []string `json:"unknown_tokens"` // -> the real product would BLOCK the whole draft on any of these
	Blocked        bool     `json:"blocked"`
	LeftoverBraces bool     `json:"leftover_braces"`
	UnknownMedia   []string `json:"unknown_media"` // -> the real product drops these (logged), does NOT block

	RequiresPass bool `json:"requires_pass"`
	MediaPass    bool `json:"media_pass"`
	// MediaIssue is a human-readable detail for a MediaPass failure the generic "did not
	// attach the expected media" wording doesn't fit — today only the Forbid case
	// ("attached media, but this test forbids any attachment").
	MediaIssue string `json:"media_issue,omitempty"`
	// MediaCount/TooManyMedia are UNIVERSAL — checked on every test regardless of whether
	// it declares a `media:` expectation, mirroring every frame's own rule-3 attachment
	// cap (see maxMediaRefs/maxMediaGroups). MediaCountEvaluated is a CODE-VERSION marker,
	// not derived from ParseOK: a verdict judged by pre-upgrade code has this field
	// entirely absent from its JSON, which unmarshals as false. Without this marker,
	// re-reading such a verdict would report TooManyMedia's zero value (false) as a
	// fabricated PASS for a check that was never actually run — MediaCountEvaluated lets
	// viewmodel.go render "not checked" instead.
	MediaCount          int  `json:"media_count"`
	TooManyMedia        bool `json:"too_many_media"`
	MediaCountEvaluated bool `json:"media_count_evaluated,omitempty"`
	EscalatePass        bool `json:"escalate_pass"`
	// LanguagePass = LanguageTextOK && LanguageFieldOK — kept as the single pass/fail
	// gate everything else already depends on. The two components are ALSO reported
	// separately (Phase 0.4 of the language plan): looksKazakh is a cheap heuristic, not
	// a whole-reply classifier, so telling "the text didn't look Kazakh" apart from "the
	// model declared the wrong reply_language" matters when manually inspecting a Kazakh
	// canary run, not just when reading the one combined boolean.
	LanguagePass    bool   `json:"language_pass"`
	LanguageTextOK  bool   `json:"language_text_ok"`
	LanguageFieldOK bool   `json:"language_field_ok"`
	LanguageIssue   string `json:"language_issue,omitempty"`
	// LanguageAliasUsed flags a pass that only succeeded because normalizeLangCode
	// mapped the model's declared reply_language onto the expected code (e.g. "kz" ->
	// "kk") — a visible warning, not a silent free pass: production only benefits from
	// the same leniency if its own reply_language consumer applies the identical
	// normalization (see normalizeLangCode's doc comment).
	LanguageAliasUsed  bool   `json:"language_alias_used,omitempty"`
	MustNotContainPass bool   `json:"must_not_contain_pass"`
	ForbiddenPhrase    string `json:"forbidden_phrase,omitempty"`
	// MustContainAnyPass mirrors MustNotContainPass for the positive-evidence check (see
	// TestCase.MustContainAny) — true when the test declares no such expectation at all
	// (nothing to fail), same "vacuously true" convention as MustNotContainPass.
	MustContainAnyPass bool `json:"must_contain_any_pass"`
	// MustContainAnyExpected echoes TestCase.MustContainAny verbatim (not just on
	// failure) — unlike MustNotContain's single ForbiddenPhrase (the one phrase that DID
	// match), a MustContainAny failure has no single culprit to name; the useful fact to
	// report is the whole expected set none of which matched.
	MustContainAnyExpected []string `json:"must_contain_any_expected,omitempty"`
	// OutcomesDeclared is the CODE-VERSION + test-shape marker for the alternative-
	// outcomes gate (TestCase.Outcomes), same pattern as MediaCountEvaluated: a verdict
	// judged before this check existed — or for a test that declares no outcomes — has
	// this false, and viewmodel.go renders the check "not run" instead of fabricating a
	// pass/fail from OutcomesPass's zero value.
	OutcomesDeclared bool `json:"outcomes_declared,omitempty"`
	// OutcomesPass: at least ONE declared OutcomeCase block had all its checks pass
	// (vacuously true when the test declares none, same convention as
	// MustNotContainPass). OutcomeMatched names the first block that passed —
	// declaration order, deterministic; OutcomeLabels echoes every declared block's
	// label so a failure can report the full set of alternatives that were on the
	// table (the MustContainAnyExpected pattern).
	OutcomesPass   bool     `json:"outcomes_pass"`
	OutcomeMatched string   `json:"outcome_matched,omitempty"`
	OutcomeLabels  []string `json:"outcome_labels,omitempty"`
	InventedDigits []string `json:"invented_digits"`
	UnitIssues     []string `json:"unit_issues"`

	// FinishReason/Truncated, ReasoningLeak, and ControlChars are all pipeline-safety
	// facts, folded into ContractPass unconditionally — independent of model, prompt
	// variant, or whether reasoning was even requested for this call (see their doc
	// comments where computed, in judgeOne).
	FinishReason  string `json:"finish_reason,omitempty"`
	Truncated     bool   `json:"truncated"`
	ReasoningLeak bool   `json:"reasoning_leak"`
	// ControlChars flags a C0 control character (other than \n/\t/\r) found in
	// reply_text — a real product must never forward a byte like \x08 (backspace) to a
	// customer. Confirmed as a real failure mode: a minimax-m2.5 response literally
	// contained "...сейчас.\x08r\n\nУход за ней..." Independent of ReasoningLeak
	// (different character class) and of Truncated (this reply parsed and finished
	// cleanly — the garbage character is INSIDE an otherwise well-formed answer).
	ControlChars      bool   `json:"control_chars"`
	ControlCharsIssue string `json:"control_chars_issue,omitempty"`
	// Retries/RetryRecovered mirror PromptfooRow.Retries — surfaced on the Verdict so
	// report.go can show per-model retry stats and a recovered row never silently
	// looks identical to a clean first-attempt pass. RetryRecovered = Retries>0 &&
	// ParseOK — the row needed a retry AND the SELECTED attempt is now parseable.
	Retries        int  `json:"retries,omitempty"`
	RetryRecovered bool `json:"retry_recovered,omitempty"`
	// Reasoning mirrors PromptfooRow.Response.Reasoning verbatim, IF promptfoo ever
	// populates it — kept as its own field (never merged into RawOutput or any
	// customer-facing text) so a captured value isn't silently dropped once something
	// downstream (viewmodel.go) starts reading it. Empty for every verdict today, since
	// that upstream field isn't confirmed populated by promptfoo (see its own doc
	// comment on PromptfooRow.Response.Reasoning).
	Reasoning string `json:"reasoning,omitempty"`

	// ContractPass: did the pipeline behave safely regardless of what the model said
	// (parses, right shape, every token resolves, no leftover brace, response not
	// truncated, no leaked reasoning/thinking content). This is the property that must
	// never fail once this design ships for real.
	ContractPass bool `json:"contract_pass"`
	// ModelBehaviorPass: did the model itself do the right thing (right token, right
	// media, right escalate, right language, no invented digits, no duplicated units, no
	// unknown media, no forbidden invented-answer phrase). This is what changes when you
	// tweak a prompt or try a different model.
	ModelBehaviorPass bool `json:"model_behavior_pass"`

	Cost      float64 `json:"cost"`
	LatencyMs int     `json:"latency_ms"`
	Tokens    int     `json:"tokens"`
	Reason    string  `json:"reason"` // first failure found, human-readable

	// Cost estimate fields — see CostBasis's doc comment for what each basis means and why
	// the number must never be shown without it.
	TokensIn        int     `json:"tokens_in"`
	TokensOut       int     `json:"tokens_out"`
	CostEstimateUSD float64 `json:"cost_estimate_usd"`
	CostBasis       string  `json:"cost_basis"`
}

// CostBasis values — what CostEstimateUSD is actually computed from, so a report can never
// present a number as more certain than it is:
//   - "measured_split": this row is a fresh (non-cached) API call with promptfoo's own
//     prompt/completion split — the estimate multiplies real token counts by models.yaml's
//     hand-maintained price.
//   - "cached_replay_borrowed": this row was a promptfoo cache hit (prompt/completion both
//     report 0), but another row in the SAME judged run made a fresh call for the same
//     (model, test) and reported a split — that split is borrowed to estimate this row's
//     cost too, since a cache hit means the same request would have cost the same.
//   - "cached_replay_unpriceable": a cache hit with no fresh row to borrow a split from in
//     this run. No number is reported — CostEstimateUSD stays 0 and must be read as "no
//     estimate", not "free".
//   - "unknown_pricing": models.yaml has no input_per_mtok/output_per_mtok for this model
//     (or one of the two is missing). No number is reported, regardless of cache state.
const (
	CostBasisMeasured       = "measured_split"
	CostBasisCachedBorrowed = "cached_replay_borrowed"
	CostBasisCachedUnpriced = "cached_replay_unpriceable"
	CostBasisUnknownPricing = "unknown_pricing"
)

// tokenSplit is a (prompt, completion) token count borrowed across rows of the same run —
// see judgeScenario's freshSplit map for why a cached row needs one from elsewhere.
type tokenSplit struct{ in, out int }

// applyCostEstimate fills a Verdict's cost-estimate fields. See CostBasis's doc comment for
// the four possible outcomes; this is the one place that decides between them.
func applyCostEstimate(v *Verdict, row PromptfooRow, priceByModel map[string]ModelProvider, freshSplit map[string]tokenSplit) {
	modelKey := providerModelKey(row.Provider.ID, row.Provider.Label)
	price, known := priceByModel[modelKey]
	if !known || price.InputPerMTok == nil || price.OutputPerMTok == nil {
		v.CostBasis = CostBasisUnknownPricing
		return
	}

	in, out := row.TokenUsage.Prompt, row.TokenUsage.Completion
	basis := CostBasisMeasured
	if row.Response.Cached || in == 0 {
		key := modelKey + "|" + row.TestCase.Description
		borrowed, ok := freshSplit[key]
		if !ok {
			v.CostBasis = CostBasisCachedUnpriced
			return
		}
		in, out = borrowed.in, borrowed.out
		basis = CostBasisCachedBorrowed
	}

	v.TokensIn = in
	v.TokensOut = out
	v.CostEstimateUSD = float64(in)/1_000_000*(*price.InputPerMTok) + float64(out)/1_000_000*(*price.OutputPerMTok)
	v.CostBasis = basis
}

// JudgedRun is generated per run: runs/<id>/<scenario>.judged.json.
type JudgedRun struct {
	Scenario string    `json:"scenario"`
	Verdicts []Verdict `json:"verdicts"`
}

// loadJudgedRuns globs and reads every *.judged.json in runDir, in sorted order — the
// ONE place this glob+sort+read sequence happens, shared by report.go's reportRun and
// blind.go's cmdBlindExport (which previously each re-implemented it, and a third call
// inside writeRoutingAccuracyReport re-read the same files cmdBlindExport had just
// loaded moments earlier — collecting runs once here removes that redundant I/O too).
// An empty result is NOT itself an error here — callers report that with their own
// wording, since "did you run judge first?" (report.go) and "did you run judge/run
// first?" (blind.go) are both accurate but different framings for their own commands.
func loadJudgedRuns(runDir string) (runs []JudgedRun, files []string, err error) {
	files, err = filepath.Glob(filepath.Join(runDir, "*.judged.json"))
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(files)
	for _, f := range files {
		var jr JudgedRun
		if err := readJSON(f, &jr); err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", f, err)
		}
		runs = append(runs, jr)
	}
	return runs, files, nil
}

var (
	tokenSpanRE = regexp.MustCompile(`\{\{[^}]*\}\}`)
	// digitRunRE matches ANY digit run — a model inventing even a single-digit fact (e.g.
	// "осталось 5 штук" without the token) is still an invented fact. listMarkerRE strips
	// numbered-list markers ("1. ", "2) " at line start) first, the one legitimate place a
	// model writes its own digits (step-by-step ordering instructions) — confirmed against
	// a real run that models use BOTH "1." and "1)" for the same kind of list.
	digitRunRE   = regexp.MustCompile(`\d+`)
	listMarkerRE = regexp.MustCompile(`(?m)^\s*\d+[.)]\s`)
	// inlineListMarkerRE strips the SAME kind of numbered-list marker when it appears
	// after a colon or semicolon on the same line, not just at true line/string start —
	// confirmed against a real run: minimax-m2.5 wrote "Для оформления заказа: 1)
	// укажите адрес доставки; 2) подтвердите заказ" as one continuous sentence, which
	// listMarkerRE's line-start anchor alone doesn't reach. Deliberately narrow (only
	// ":"/";" -immediately-preceded, not any digit+")"): a bare "код 7)х" in running
	// prose must stay flagged as a possible invented number — RE2 has no lookbehind, so
	// this captures the delimiter+whitespace in $1$2 and re-emits it, removing only the
	// marker itself.
	inlineListMarkerRE = regexp.MustCompile(`([:;])(\s*)\d+[.)]\s`)
	unitIssueREs       = []unitIssuePattern{
		{label: "duplicated tenge symbol", re: regexp.MustCompile(`₸\s*₸`)},
		{label: "duplicated tenge word", re: regexp.MustCompile(`₸\s*тенге`)},
		{label: "duplicated Алматы qualifier", re: regexp.MustCompile(`(?i)по\s+алматы\s+по\s+алматы`)},
		{label: "duplicated day unit", re: regexp.MustCompile(`(?i)(дня|дней)\s+(дня|дней)`)},
		{label: "mixed Russian/Kazakh day-unit hint", re: regexp.MustCompile(`(?i)дня\s*/\s*күнде`)},
	}
)

// kazakhOnlyLetters are the Cyrillic letters that exist in the Kazakh alphabet but not
// the Russian one. і (U+0456) belongs here too: Russian dropped that letter in 1918, and
// a fluent Kazakh sentence can be distinguished from Russian by і ALONE — run
// 2026-07-19_02-50-37-ef6e falsely failed «Сізге онымен бірге келетін суреттерді
// жіберейін бе?» (six і, zero other Kazakh-only letters) as "does not look like Kazakh"
// while і was missing from this set. langdetect.go's kazakhOnlySpecificLetters is a
// deliberate independent copy; the і addition was made to both together, on purpose.
const kazakhOnlyLetters = "әғқңөұүһіӘҒҚҢӨҰҮҺІ"

// normalizeLangCode maps a model's declared reply_language onto the code this harness
// expects, tolerating the one confusion actually observed in real runs: a model writing
// "kz" (Kazakhstan's ISO 3166-1 COUNTRY code) when it means "kk" (Kazakh's ISO 639-1
// LANGUAGE code) — same intent, wrong standard. This is a FIELD-level alias only; the
// text-content heuristic (looksKazakh) is never touched by it, so a model that writes
// "kz" but replies in Russian still fails on textOK. Every other value passes through
// unchanged — this is a targeted fix for one confirmed real confusion, not a general
// language-code normalizer. See Verdict.LanguageAliasUsed for how a pass via this alias
// stays visible rather than silent, and the production-parity note in models.yaml/README:
// this alias must ALSO be applied wherever production actually consumes reply_language,
// or the eval would green-light output production would misroute.
func normalizeLangCode(code string) string {
	if code == "kz" {
		return "kk"
	}
	return code
}

// maxMediaRefs/maxMediaGroups mirror each frame's own attachment cap — rule 3 in every
// frame.txt: asset_refs frames say "Maximum 3" (e.g. shop-current/frame.txt,
// lang-canary-v1/frame.txt), attach_groups frames say "Максимум 2 группы" (e.g.
// shop-scale/frame.txt, shop-decisions-v1/frame.txt, xpayment-decisions-v1/frame.txt). If
// either frame's stated cap ever changes, this constant must change with it.
const (
	maxMediaRefs   = 3
	maxMediaGroups = 2
)

// providerModelKey is the ONE key every grouping of judged results (Verdict.Model,
// priceByModel, freshSplit) must use once two provider entries can share the same
// underlying ID — e.g. a reasoning-on/reasoning-off comparison pair (ModelProvider.Label
// in types.go). Without this, two such entries would silently merge into one bucket
// downstream (report.go's byModel, viewmodel.go), corrupting exactly the side-by-side
// comparison a Label exists to enable. Label empty (every model before this existed)
// returns id unchanged, so nothing about today's single-entry-per-model runs changes.
//
// Known, accepted assumption: this concatenates id/label with a human-readable
// " [label]" separator, not an escaped/length-prefixed encoding, so a contrived id or
// label containing that exact " [...]" pattern could theoretically collide with a
// different entry's key. Deliberately not hardened against this: OpenRouter model IDs
// are a controlled slash/hyphen/colon vocabulary that never contains literal brackets,
// and Verdict.Model is displayed verbatim in every report/viewer — an unambiguous but
// unreadable encoding (e.g. length-prefixed) would defeat that display purpose to guard
// against an id shape that doesn't occur in practice.
func providerModelKey(id, label string) string {
	if label == "" {
		return id
	}
	return id + " [" + label + "]"
}

// truncationNote is the one wording for "this response was truncated" — shared by
// judgeOne's two call sites (the early parse-failure return, and the later fallback for
// a response that parsed despite being cut short) so the fact reads identically in
// SUMMARY.md/CONTRACT.md regardless of which path produced it.
func truncationNote(finishReason string) string {
	return fmt.Sprintf("response was truncated (finish_reason=%s)", finishReason)
}

// isTruncatedFinish normalizes a promptfoo/OpenAI-style finish_reason into "was this
// response cut off before the model finished." "length" is the canonical truncation
// signal (confirmed against extract.go's own documented observation of models truncating
// exactly at their token budget). Deliberately narrow: an empty string ("not reported")
// or any other value (e.g. "stop", "content_filter", "tool_calls") is NOT treated as
// truncation — "content_filter"/"tool_calls" are real but different failure modes this
// function doesn't claim to cover, and treating "" as truncation would break every
// existing PromptfooRow fixture built before this field existed.
func isTruncatedFinish(reason string) bool {
	return reason == "length"
}

type unitIssuePattern struct {
	label string
	re    *regexp.Regexp
}

func cmdJudge(args []string) error {
	fs := flag.NewFlagSet("judge", flag.ExitOnError)
	scenarioDir := fs.String("scenario", "", "path to the scenario directory")
	runDir := fs.String("run", "", "path to the run directory (contains results.json)")
	modelsPath := fs.String("models", "models.yaml", "path to models.yaml (for cost-estimate pricing)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scenarioDir == "" || *runDir == "" {
		return fmt.Errorf("judge: -scenario and -run are both required")
	}
	return judgeScenario(*scenarioDir, *runDir, *modelsPath)
}

func judgeScenario(scenarioDir, runDir, modelsPath string) error {
	scenario, err := loadScenario(scenarioDir)
	if err != nil {
		return fmt.Errorf("load scenario.yaml: %w", err)
	}

	// Prefer this run's own snapshot of what render produced over the live, mutable
	// scenarios/*/generated/ — the whole point of snapshotting: re-judging an old run
	// must grade against the requirements THAT RUN saw, not whatever the scenario looks
	// like today. Legacy runs (no snapshot) and a standalone `judge` right after a fresh
	// `render` fall back to genDir/modelsPath exactly as before this existed.
	genDir := filepath.Join(scenarioDir, "generated")
	resolvedModelsPath := modelsPath
	if snapDir, ok := provenance.SnapshotDirFor(runDir, scenario.Name); ok {
		genDir = snapDir
		resolvedModelsPath = provenance.SnapshotModelsPath(runDir, modelsPath)
	}

	models, err := loadModels(resolvedModelsPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", resolvedModelsPath, err)
	}
	priceByModel := map[string]ModelProvider{}
	for _, p := range models.Providers {
		priceByModel[providerModelKey(p.ID, p.Label)] = p
	}

	var catalog Catalog
	if err := readJSON(filepath.Join(genDir, "catalog.json"), &catalog); err != nil {
		return fmt.Errorf("read catalog.json (did you run render first?): %w", err)
	}
	tokenValue := map[string]string{}
	for _, f := range catalog.Tokens {
		tokenValue[f.Token] = f.Value
	}
	validMediaRef := map[string]bool{}
	for _, r := range catalog.MediaRefs {
		validMediaRef[r] = true
	}
	validMediaGroup := map[string]bool{}
	for _, g := range catalog.MediaGroups {
		validMediaGroup[g] = true
	}

	var resolved ResolvedTests
	if err := readJSON(filepath.Join(genDir, "resolved_tests.json"), &resolved); err != nil {
		return fmt.Errorf("read resolved_tests.json: %w", err)
	}
	testByID := map[string]TestCase{}
	for _, t := range resolved.Tests {
		testByID[t.ID] = t
	}

	resultsPath := filepath.Join(runDir, scenario.Name+".results.json")
	var results PromptfooResults
	if err := readJSON(resultsPath, &results); err != nil {
		return fmt.Errorf("read %s: %w", resultsPath, err)
	}

	// freshSplit lets a cached row (promptfoo reports prompt:0, completion:0 on a cache
	// hit) borrow the real in/out split from another row in THIS SAME run that made a
	// fresh call for the identical (model, test) — a cache hit means an identical request
	// would have cost the same, so the split is still a valid estimate, just not this
	// row's own measurement.
	freshSplit := map[string]tokenSplit{}
	for _, row := range results.Results.Results {
		if !row.Response.Cached && row.TokenUsage.Prompt > 0 {
			key := providerModelKey(row.Provider.ID, row.Provider.Label) + "|" + row.TestCase.Description
			freshSplit[key] = tokenSplit{row.TokenUsage.Prompt, row.TokenUsage.Completion}
		}
	}

	var verdicts []Verdict
	for _, row := range results.Results.Results {
		tc, ok := testByID[row.TestCase.Description]
		if !ok {
			continue // belongs to a different scenario's tests, if results were ever merged
		}
		v := judgeOne(tc, row, &catalog, tokenValue, validMediaRef, validMediaGroup)
		applyCostEstimate(&v, row, priceByModel, freshSplit)
		verdicts = append(verdicts, v)
	}

	out := JudgedRun{Scenario: scenario.Name, Verdicts: verdicts}
	outPath := filepath.Join(runDir, scenario.Name+".judged.json")
	if err := writeJSON(outPath, out); err != nil {
		return err
	}
	fmt.Printf("judged %s: %d verdicts -> %s\n", scenario.Name, len(verdicts), outPath)
	return nil
}

// appendReason accumulates fact onto v.Reason, joined with "; " when something is
// already there — every contract-relevant fact judgeOne computes must show up in the
// one human-readable Reason string, not just whichever fact happened to run first.
// Before this existed, only the Blocked branch bothered to append onto a non-empty
// Reason (with its own hand-rolled special case for ReasoningLeak specifically); every
// other combination — e.g. ContractFields failing for a reason unrelated to reply_text
// while reply_text ALSO contains a leaked <think> block — silently dropped the later
// fact from Reason. SUMMARY.md's "Failures (verbatim)" section prints Reason verbatim
// and nothing else, so a dropped fact there was genuinely invisible from that report
// (CONTRACT.md and the HTML viewer are unaffected: both read v.ReasoningLeak/v.Truncated
// directly as their own booleans, never through this string).
func (v *Verdict) appendReason(fact string) {
	if v.Reason == "" {
		v.Reason = fact
	} else {
		v.Reason += "; " + fact
	}
}

func judgeOne(tc TestCase, row PromptfooRow, catalog *Catalog, tokenValue map[string]string, validRef, validGroup map[string]bool) Verdict {
	v := Verdict{
		TestID:       tc.ID,
		Model:        providerModelKey(row.Provider.ID, row.Provider.Label),
		Message:      tc.Message,
		RawOutput:    row.Response.Output,
		Cost:         row.Cost,
		LatencyMs:    row.LatencyMs,
		Tokens:       row.TokenUsage.Total,
		FinishReason: row.Response.FinishReason,
		Truncated:    isTruncatedFinish(row.Response.FinishReason),
		Reasoning:    row.Response.Reasoning,
		Retries:      row.Retries,
	}

	obj, ok := parseModelJSON(row.Response.Output)
	v.ParseOK = ok
	v.RetryRecovered = v.Retries > 0 && v.ParseOK
	if !ok {
		v.Reason = "could not parse JSON output"
		if v.Truncated {
			v.Reason += " (" + truncationNote(v.FinishReason) + ")"
		}
		return v
	}

	// Type-checked, not just presence-checked: a model can return the right KEYS with
	// the wrong shape (escalate: "true" as a string, reply_language: 7) and the older
	// presence-only check would have let that through as a valid contract.
	replyText, hasReplyText := obj["reply_text"].(string)
	_, hasLang := obj["reply_language"].(string)
	escalateVal, hasEscalate := obj["escalate"].(bool)
	mediaField := "asset_refs"
	if catalog.Contract == "attach_groups" {
		mediaField = "attach_groups"
	}
	// mediaRaw is the UNTYPED array — kept around (not just mediaEntries' string-filtered
	// view) so MediaCount below counts every entry the model actually wrote, including a
	// stray non-string one, instead of silently undercounting by dropping it first.
	mediaRaw, hasMediaField := obj[mediaField].([]any)
	mediaAllStrings := true
	for _, e := range mediaRaw {
		if _, ok := e.(string); !ok {
			mediaAllStrings = false
			break
		}
	}
	v.ContractFields = hasReplyText && replyText != "" && hasLang && hasEscalate && hasMediaField && mediaAllStrings
	if !v.ContractFields {
		v.appendReason(fmt.Sprintf("missing or wrong-typed contract field (need string reply_text, string reply_language, bool escalate, array %s of strings)", mediaField))
	}

	// Reasoning/thinking content must never leak into reply_text — the one field the
	// real product would forward to a customer after human review. Scans replyText
	// directly, not v.InjectedText: InjectedText is only set once injection succeeds
	// (see the Blocked branch below), so a response that's ALSO blocked would otherwise
	// have nothing set to scan and silently miss the leak. Unconditional — not gated on
	// whether this call even requested reasoning (ReasoningConfig) — because a model can
	// emit inline <think> tags on its own, independent of what was asked for.
	v.ReasoningLeak = evaltext.HasReasoningMarkers(replyText)
	if v.ReasoningLeak {
		v.appendReason("reasoning/thinking content leaked into reply_text")
	}

	if bad, ok := firstBadControlChar(replyText); ok {
		v.ControlChars = true
		v.ControlCharsIssue = fmt.Sprintf("reply_text contains control character %U", bad)
		v.appendReason(v.ControlCharsIssue)
	}

	// Fail-closed: every token the model used must resolve, or the real product would
	// block the whole draft rather than ship a half-rendered fact. escalation_reason is
	// internal (never shown to the customer) but still scanned: an unknown token there
	// means the model referenced a fact that doesn't exist, the same underlying bug.
	escalationReason, _ := obj["escalation_reason"].(string)
	spans := tokenSpanRE.FindAllString(replyText, -1)
	reasonSpans := tokenSpanRE.FindAllString(escalationReason, -1)
	injected := replyText
	for _, tok := range spans {
		val, known := tokenValue[tok]
		if !known {
			v.UnknownTokens = append(v.UnknownTokens, tok)
			continue
		}
		injected = strings.ReplaceAll(injected, tok, val)
	}
	for _, tok := range reasonSpans {
		if _, known := tokenValue[tok]; !known {
			v.UnknownTokens = append(v.UnknownTokens, tok)
		}
	}
	v.Blocked = len(v.UnknownTokens) > 0
	if v.Blocked {
		v.appendReason("unknown token(s), draft would be BLOCKED: " + strings.Join(v.UnknownTokens, ", "))
	} else if !v.ReasoningLeak {
		// InjectedText is documented and displayed everywhere (CONTRACT.md, the HTML
		// viewer) as "the actual customer-facing text" — a leaking reply must never
		// populate it, exactly like a Blocked reply doesn't, or the leaked content would
		// reach the one field explicitly presented as safe, ready-to-send output.
		v.InjectedText = injected
	}
	// Any brace surviving injection is a mangled placeholder — not just an unclosed
	// "{{", but also a single-brace typo like "{product.price}" that tokenSpanRE never
	// even recognized as a span, so it was never substituted at all. Catalog values are
	// guaranteed brace-free at render time (see validateCatalog), so ANY '{' or '}' left
	// in the customer-facing text after injection can only have come from the model.
	if strings.ContainsAny(injected, "{}") {
		v.LeftoverBraces = true
		v.appendReason("leftover brace survived injection")
	}
	if v.Truncated {
		v.appendReason(truncationNote(v.FinishReason))
	}
	v.ContractPass = v.ParseOK && v.ContractFields && !v.Blocked && !v.LeftoverBraces && !v.Truncated && !v.ReasoningLeak && !v.ControlChars

	// Model-behavior checks (only meaningful once we know the contract held).
	stripped := tokenSpanRE.ReplaceAllString(replyText, "")
	stripped = listMarkerRE.ReplaceAllString(stripped, "")
	stripped = inlineListMarkerRE.ReplaceAllString(stripped, "$1$2")
	// A digit run isn't an invented fact if it came from somewhere legitimate: the
	// CUSTOMER's own message (e.g. "iPhone 15 Pro" — the model is just echoing it back),
	// or a product's Description prose (this playground's own doctrine trusts that field
	// as paraphrasable, unlike FACTS — see FactRow.Description's comment). Confirmed
	// against real runs: a model repeating a customer-named off-catalog product's model
	// number, or correctly quoting a description's "1.7 л" / "7 режимов" spec, both got
	// wrongly flagged here before these exclusions existed.
	v.InventedDigits = filterInventedDigits(digitRunRE.FindAllString(stripped, -1), tc.Message, catalog.TrustedDigits)

	v.RequiresPass = requiresSatisfied(tc.Requires, replyText, tokenValue)

	v.MediaPass = true
	if tc.Media != nil {
		v.MediaPass, v.MediaIssue = checkMediaExpect(tc.Media, obj, mediaRaw, mediaField)
	}

	// MediaCount/TooManyMedia is UNIVERSAL — every frame's own rule 3 caps attachments, so
	// this is checked regardless of whether the test declares a `media:` expectation at
	// all, the same way UnknownMedia below is. Counts the RAW array length (mediaRaw, from
	// the ContractFields check above), not mediaEntries' string-filtered view: a
	// non-string element already fails ContractFields, but a response that's over the cap
	// AND has a stray non-string entry must still be counted as over the cap, not
	// undercounted by silently dropping the malformed entry first.
	v.MediaCountEvaluated = true
	v.MediaCount = len(mediaRaw)
	mediaLimit := maxMediaRefs
	if mediaField == "attach_groups" {
		mediaLimit = maxMediaGroups
	}
	v.TooManyMedia = v.MediaCount > mediaLimit

	v.EscalatePass = true
	if tc.Escalate != nil {
		v.EscalatePass = escalateVal == *tc.Escalate
	}

	// Two independent checks: does the TEXT read as the expected language (a Kazakh-letter
	// heuristic for kk, its absence for ru), and does the model's own declared
	// reply_language FIELD match — a model can write Russian prose while claiming
	// reply_language: "kk" and the text-only heuristic would never catch that.
	v.LanguagePass = true
	v.LanguageTextOK = true
	v.LanguageFieldOK = true
	if tc.Language == "kk" || tc.Language == "ru" {
		checkText := v.InjectedText
		if checkText == "" {
			checkText = replyText // e.g. blocked before injection — still check what the model wrote
		}
		replyLang, _ := obj["reply_language"].(string)
		textOK := languageTextOK(tc.Language, checkText)
		fieldOK := languageFieldOK(tc.Language, replyLang)
		v.LanguageAliasUsed = fieldOK && replyLang != tc.Language
		v.LanguageTextOK = textOK
		v.LanguageFieldOK = fieldOK
		v.LanguagePass = textOK && fieldOK
		switch {
		case !textOK && tc.Language == "kk":
			v.LanguageIssue = "reply does not look like Kazakh (too few Kazakh-specific letters)"
		case !textOK && tc.Language == "ru":
			v.LanguageIssue = "reply looks like Kazakh but a Russian reply was expected"
		case !fieldOK:
			v.LanguageIssue = fmt.Sprintf("reply_language field is %q, expected %q", replyLang, tc.Language)
		}
	}
	v.UnitIssues = findUnitIssues(v.InjectedText)

	v.MustNotContainPass = true
	if len(tc.MustNotContain) > 0 {
		// Scans the token-INJECTED text (not raw replyText): a forbidden phrase can
		// materialize only after substitution — e.g. "{{policy.main.return_period}}"
		// injects to "14 дней", so "в течение 14 дней" can appear in the text a customer
		// would actually receive even though the model never wrote those digits itself.
		// `injected` (computed above) is always populated, including when the response is
		// Blocked or leaking reasoning — unlike v.InjectedText, which is intentionally
		// left empty in both those cases.
		if phrase, hit := firstForbidden(tc.MustNotContain, injected); hit {
			v.MustNotContainPass = false
			v.ForbiddenPhrase = phrase
		}
	}

	v.MustContainAnyPass = true
	if len(tc.MustContainAny) > 0 {
		v.MustContainAnyExpected = tc.MustContainAny
		v.MustContainAnyPass = anyContained(tc.MustContainAny, injected)
	}

	// Alternative-outcome gate (TestCase.Outcomes): OR over the declared blocks, AND-ed
	// into ModelBehaviorPass alongside everything above. Each block re-uses the exact
	// helpers the universal checks above run through (languageTextOK/languageFieldOK,
	// checkMediaExpect, firstForbidden/anyContained, requiresSatisfied), so a block's
	// verdict can never drift from what the same expectation would mean at the top level.
	v.OutcomesDeclared = len(tc.Outcomes) > 0
	v.OutcomesPass = true // vacuously true when no outcomes are declared, same convention as MustNotContainPass
	if v.OutcomesDeclared {
		for _, oc := range tc.Outcomes {
			v.OutcomeLabels = append(v.OutcomeLabels, oc.Label)
		}
		checkText := v.InjectedText
		if checkText == "" {
			checkText = replyText // same fallback as the universal language check above
		}
		replyLang, _ := obj["reply_language"].(string)
		in := outcomeInputs{
			replyText:  replyText,
			injected:   injected,
			checkText:  checkText,
			replyLang:  replyLang,
			escalate:   escalateVal,
			obj:        obj,
			mediaRaw:   mediaRaw,
			mediaField: mediaField,
			tokenValue: tokenValue,
		}
		v.OutcomesPass = false
		for _, oc := range tc.Outcomes {
			if outcomeSatisfied(oc, in) {
				v.OutcomesPass = true
				v.OutcomeMatched = oc.Label // first passing block wins — declaration order, deterministic
				break
			}
		}
	}

	for _, entry := range mediaEntries(obj, mediaField) {
		if (mediaField == "asset_refs" && !validRef[entry]) || (mediaField == "attach_groups" && !validGroup[entry]) {
			v.UnknownMedia = append(v.UnknownMedia, entry)
		}
	}

	v.ModelBehaviorPass = v.RequiresPass && v.MediaPass && v.EscalatePass && v.LanguagePass &&
		v.MustNotContainPass && v.MustContainAnyPass && v.OutcomesPass && len(v.InventedDigits) == 0 && len(v.UnitIssues) == 0 && len(v.UnknownMedia) == 0 &&
		!v.TooManyMedia

	if v.Reason == "" && !v.ModelBehaviorPass {
		v.Reason = firstFailureReason(v)
	}
	if v.Reason == "" {
		v.Reason = "ok"
	}
	return v
}

func firstFailureReason(v Verdict) string {
	switch {
	case !v.RequiresPass:
		return "did not use the required fact token(s)"
	case !v.MediaPass:
		if v.MediaIssue != "" {
			return v.MediaIssue
		}
		return "did not attach the expected media"
	case !v.EscalatePass:
		return "escalate did not match expectation"
	case !v.LanguagePass:
		return v.LanguageIssue
	case !v.MustNotContainPass:
		// Generic on purpose — this check is no longer escalation-only (e.g. test 28's
		// "don't claim to attach a video that doesn't exist" has no escalate expectation
		// at all), so wording that assumes escalation would be wrong here.
		return "reply_text contains forbidden phrase: \"" + v.ForbiddenPhrase + "\""
	case !v.MustContainAnyPass:
		return "reply_text contains none of the expected phrases: " + strings.Join(v.MustContainAnyExpected, ", ")
	case !v.OutcomesPass:
		return "none of the acceptable outcomes was satisfied: " + strings.Join(v.OutcomeLabels, " | ")
	case len(v.InventedDigits) > 0:
		return "invented digits outside any token: " + strings.Join(v.InventedDigits, ", ")
	case len(v.UnitIssues) > 0:
		return "unit/currency issue after injection: " + strings.Join(v.UnitIssues, ", ")
	case len(v.UnknownMedia) > 0:
		return "attached media not in the catalog: " + strings.Join(v.UnknownMedia, ", ")
	case v.TooManyMedia:
		return fmt.Sprintf("attached %d media entries — over the frame's cap", v.MediaCount)
	}
	return ""
}

func parseModelJSON(raw string) (map[string]any, bool) {
	cleaned := evaltext.StripFences(raw)
	var obj map[string]any
	if err := json.Unmarshal([]byte(cleaned), &obj); err != nil {
		return nil, false
	}
	return obj, true
}

// requiresSatisfied checks that the model used one of the EXPECTED tokens AND that the
// token is one this scenario's catalog actually resolves. Shared question banks often
// list both schemas' field names in one OR-group (e.g. "availability" or
// "available_pieces") so the same question works everywhere — without the tokenValue
// check, a model using the WRONG schema's field name would satisfy the OR-list by literal
// text match even though that exact token is unknown to this scenario (already flagged
// separately as Blocked) — hiding a real "used the wrong vocabulary for this schema" bug.
func requiresSatisfied(requires [][]string, replyText string, tokenValue map[string]string) bool {
	for _, group := range requires {
		found := false
		for _, tok := range group {
			literal := factTokenLiteral(tok)
			if _, known := tokenValue[literal]; known && strings.Contains(replyText, literal) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// factTokenLiteral turns a bare "table.ref.field" (as written in tests.yaml) into the
// literal "{{table.ref.field}}" the model would have to emit.
func factTokenLiteral(dotted string) string {
	return "{{" + dotted + "}}"
}

// languageTextOK is the text-side language check — the exact looksKazakh polarity switch
// judgeOne's universal check uses, extracted so OutcomeCase blocks run the identical
// logic. expected must be "kk" or "ru" (callers gate on that); any other value returns
// true (no expectation).
func languageTextOK(expected, text string) bool {
	switch expected {
	case "kk":
		return looksKazakh(text)
	case "ru":
		return !looksKazakh(text)
	}
	return true
}

// languageFieldOK is the field-side language check (declared reply_language vs expected),
// with the same kz->kk aliasing as the universal check (normalizeLangCode).
func languageFieldOK(expected, replyLang string) bool {
	return normalizeLangCode(replyLang) == expected
}

// checkMediaExpect evaluates one MediaExpect block (Forbid / any_of_* / Exclusive)
// against the reply's media array — judgeOne's universal media check verbatim, extracted
// so OutcomeCase blocks share it. Returns pass plus the human-readable issue detail for
// the failure modes that have one.
func checkMediaExpect(exp *MediaExpect, obj map[string]any, mediaRaw []any, mediaField string) (bool, string) {
	if exp.Forbid {
		if len(mediaRaw) > 0 {
			return false, "attached media, but this test forbids any attachment"
		}
		return true, ""
	}
	if !mediaExpectationMet(exp, obj, mediaField) {
		return false, ""
	}
	if exp.Exclusive {
		if outsiders := mediaOutsideExpectation(exp, obj, mediaField); len(outsiders) > 0 {
			return false, "attached media outside the expected set: " + strings.Join(outsiders, ", ")
		}
	}
	return true, ""
}

// firstForbidden reports the first phrase from list found in text (case-insensitive
// substring, same semantics as TestCase.MustNotContain — callers pass the token-INJECTED
// text).
func firstForbidden(list []string, text string) (string, bool) {
	lower := strings.ToLower(text)
	for _, phrase := range list {
		if strings.Contains(lower, strings.ToLower(phrase)) {
			return phrase, true
		}
	}
	return "", false
}

// anyContained reports whether at least one phrase from list appears in text
// (case-insensitive substring, same semantics as TestCase.MustContainAny).
func anyContained(list []string, text string) bool {
	lower := strings.ToLower(text)
	for _, phrase := range list {
		if strings.Contains(lower, strings.ToLower(phrase)) {
			return true
		}
	}
	return false
}

// outcomeInputs bundles the per-reply facts an OutcomeCase block is evaluated against —
// all computed once in judgeOne and shared by every block, so evaluating N alternatives
// never re-derives (or diverges on) what the reply actually said.
type outcomeInputs struct {
	replyText  string         // raw model reply_text (requires matches the un-substituted {{token}} spans)
	injected   string         // token-injected text (must_not_contain / must_contain_any scan this)
	checkText  string         // language-check text: InjectedText with judgeOne's replyText fallback
	replyLang  string         // the model's declared reply_language field
	escalate   bool           // the model's escalate field (type-checked earlier)
	obj        map[string]any // parsed reply object (media helpers read the array from it)
	mediaRaw   []any          // untyped media array, for Forbid's raw-length semantics
	mediaField string         // "asset_refs" | "attach_groups"
	tokenValue map[string]string
}

// outcomeSatisfied evaluates ONE OutcomeCase block: every check the block declares must
// pass; a knob left absent is vacuously true — the same convention as absent
// TestCase-level knobs. Each check delegates to the same helper judgeOne's universal
// checks use, so block semantics and top-level semantics are one implementation.
func outcomeSatisfied(oc OutcomeCase, in outcomeInputs) bool {
	if !requiresSatisfied(oc.Requires, in.replyText, in.tokenValue) {
		return false
	}
	if oc.Escalate != nil && in.escalate != *oc.Escalate {
		return false
	}
	if oc.Language == "kk" || oc.Language == "ru" {
		if !languageTextOK(oc.Language, in.checkText) || !languageFieldOK(oc.Language, in.replyLang) {
			return false
		}
	}
	if oc.Media != nil {
		if pass, _ := checkMediaExpect(oc.Media, in.obj, in.mediaRaw, in.mediaField); !pass {
			return false
		}
	}
	if len(oc.MustNotContain) > 0 {
		if _, hit := firstForbidden(oc.MustNotContain, in.injected); hit {
			return false
		}
	}
	if len(oc.MustContainAny) > 0 && !anyContained(oc.MustContainAny, in.injected) {
		return false
	}
	return true
}

// filterInventedDigits drops any digit run that isn't actually invented: one the customer
// already wrote in their own message (the model is just echoing it back, e.g. a product
// name like "iPhone 15 Pro"), or one that came from a product's Description prose (trusted,
// paraphrasable text per this playground's own doctrine — see Catalog.TrustedDigits).
func filterInventedDigits(found []string, message string, trustedDigits []string) []string {
	if len(found) == 0 {
		return nil
	}
	allowed := map[string]bool{}
	for _, d := range digitRunRE.FindAllString(message, -1) {
		allowed[d] = true
	}
	for _, d := range trustedDigits {
		allowed[d] = true
	}
	var out []string
	for _, d := range found {
		if !allowed[d] {
			out = append(out, d)
		}
	}
	return out
}

func mediaEntries(obj map[string]any, field string) []string {
	raw, ok := obj[field].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func mediaExpectationMet(exp *MediaExpect, obj map[string]any, field string) bool {
	entries := mediaEntries(obj, field)
	want := exp.AnyOfGroups
	if field == "asset_refs" {
		want = exp.AnyOfRefs
	}
	if len(want) == 0 {
		return true
	}
	for _, w := range want {
		for _, e := range entries {
			if e == w {
				return true
			}
		}
	}
	return false
}

// mediaOutsideExpectation returns every attached entry NOT in the test's declared
// any_of_* set — used only when Exclusive is true, on top of mediaExpectationMet's
// existing "at least one of these" check, to enforce "this set AND NOTHING ELSE". Same
// want-list side-selection (refs under asset_refs, groups under attach_groups) as
// mediaExpectationMet, so the two checks can never disagree about which list is active.
func mediaOutsideExpectation(exp *MediaExpect, obj map[string]any, field string) []string {
	entries := mediaEntries(obj, field)
	want := exp.AnyOfGroups
	if field == "asset_refs" {
		want = exp.AnyOfRefs
	}
	allowed := map[string]bool{}
	for _, w := range want {
		allowed[w] = true
	}
	var outsiders []string
	for _, e := range entries {
		if !allowed[e] {
			outsiders = append(outsiders, e)
		}
	}
	return outsiders
}

// looksKazakh is a cheap presence heuristic, NOT a whole-reply language classifier: two or
// more Kazakh-specific letters ANYWHERE in the text is enough to return true, even if the
// surrounding sentence is mostly Russian prose with one Kazakh word borrowed in. It catches
// the failure mode actually observed in eval runs (a reply that is entirely Russian, zero
// Kazakh-specific letters), but passing this check is not proof the ENTIRE reply reads as
// fluent Kazakh — see judgeOne's textOK/fieldOK split (reported separately on Verdict as
// LanguageTextOK/LanguageFieldOK) and the plan's Phase 0.4 note: do not use this score alone
// as a routing/production gate without manually inspecting the actual Kazakh canary replies
// first. A real lexical or model-based language check is future work, not this function.
func looksKazakh(text string) bool {
	count := 0
	for _, r := range text {
		if strings.ContainsRune(kazakhOnlyLetters, r) {
			count++
		}
	}
	return count >= 2
}

// firstBadControlChar reports the first C0 control character in text that a real
// product must never forward to a customer — every rune below 0x20 (plus DEL, 0x7F)
// EXCEPT \n, \t, \r, which are legitimate formatting whitespace already handled
// elsewhere (CRLF included, so a normal \r\n line ending is never flagged).
func firstBadControlChar(text string) (rune, bool) {
	for _, r := range text {
		if r == '\n' || r == '\t' || r == '\r' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return r, true
		}
	}
	return 0, false
}

func findUnitIssues(text string) []string {
	if text == "" {
		return nil
	}
	seen := map[string]bool{}
	var issues []string
	for _, p := range unitIssueREs {
		if p.re.MatchString(text) && !seen[p.label] {
			seen[p.label] = true
			issues = append(issues, p.label)
		}
	}
	return issues
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
