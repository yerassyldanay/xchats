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

	"github.com/yerassyldanay/xchats/backend/aiprompt"
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
		// in retained real promptfoo run evidence, sibling to Output/Cached). "length"
		// (OpenAI/OpenRouter's canonical truncation
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
	// RetryReason labels WHY retry.go retried this row — "contract_shape" (unparseable
	// JSON or a missing/wrong-typed field) or "media_not_found" (a hallucinated media
	// token, or one that no longer resolves through kbd_materials) — see retryReason's
	// doc comment. Empty for every row retry.go has never touched.
	RetryReason string `json:"retry_reason,omitempty"`
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
	// RetryReason mirrors PromptfooRow.RetryReason — set on the RETRY attempt only
	// (the original attempt was never itself a retry, so it carries no reason).
	RetryReason string `json:"retry_reason,omitempty"`
}

// Verdict is one judged answer — the harness's contribution on top of promptfoo's own
// (now-trivial) pass/fail. See PLAYGROUND.md: "Promptfoo grades model behavior. The
// harness grades the contract." Both dimensions live here, kept explicitly separate.
type Verdict struct {
	TestID  string `json:"test_id"`
	Model   string `json:"model"`
	Message string `json:"message"`

	// RawOutput is the provider's complete response, byte-for-byte — the immutable
	// audit record. It is never rewritten by validation or re-judging; everything
	// judged is judged from a COPY (FinalOutput) selected out of it.
	RawOutput    string `json:"raw_output"`
	InjectedText string `json:"injected_text"`

	// FinalOutput/NonFinalOutput/ExtractionMethod record how the model's FINAL
	// customer-response JSON was separated from any reasoning text the provider
	// returned combined with it (aiprompt.ExtractFinalOutput). FinalOutput is the
	// exact span that was validated and scored; NonFinalOutput is the surrounding
	// reasoning/diagnostic text — kept for debugging, never scored and never
	// customer-facing; ExtractionMethod is one of aiprompt's Extract* constants.
	// All empty when ParseOK is false (no complete final answer existed) and for
	// verdicts judged before these fields existed.
	FinalOutput      string `json:"final_output,omitempty"`
	NonFinalOutput   string `json:"non_final_output,omitempty"`
	ExtractionMethod string `json:"extraction_method,omitempty"`

	ParseOK        bool     `json:"parse_ok"`
	ContractFields bool     `json:"contract_fields_ok"`
	UnknownTokens  []string `json:"unknown_tokens"` // -> the real product would BLOCK the whole draft on any of these
	Blocked        bool     `json:"blocked"`
	LeftoverBraces bool     `json:"leftover_braces"`
	UnknownMedia   []string `json:"unknown_media"` // -> the real product drops these (logged), does NOT block
	// EscalationReasonOK is retained for old judged.json compatibility but is now
	// ALWAYS true: escalation_reason is diagnostic-only telemetry (never shown to the
	// customer), so a nonempty value with escalate=false — or placeholders/braces
	// inside it — is no longer a failure of any kind and never gates ContractPass.
	EscalationReasonOK bool `json:"escalation_reason_ok"`
	// MediaResolveOK/MediaResolveIssue: schema_kb_v1 verdicts only (always true, no
	// issue, for a legacy verdict — there is nothing to resolve). True when every
	// media_files_to_send token that passed the catalog-membership check ALSO resolves
	// through aiprompt.ResolveSend against the live kbd_materials rows right now — i.e.
	// it isn't merely a name the catalog recognizes but a reference that would actually
	// still send. Folded into ContractPass: a token valid at catalog-build time but
	// stale at send time is a pipeline-safety issue, the same category as an unknown
	// token.
	MediaResolveOK    bool   `json:"media_resolve_ok"`
	MediaResolveIssue string `json:"media_resolve_issue,omitempty"`

	RequiresPass bool `json:"requires_pass"`
	// ForbidTokensPass mirrors MustNotContainPass but checks fact/media TOKENS rather
	// than free text, on the raw (pre-substitution) reply — vacuously true when the test
	// declares no forbid_tokens, same convention as MustNotContainPass. Built for cases
	// that must prove the model picked a DIFFERENT fact than the one it's tempted to
	// reuse — e.g. an unlisted-delivery-direction reply must never cite ANY zone's
	// cost/days token, which a text-only blocklist can't express (see
	// TestCase.ForbidTokens).
	ForbidTokensPass   bool     `json:"forbid_tokens_pass"`
	ForbiddenTokensHit []string `json:"forbidden_tokens_hit,omitempty"`
	MediaPass          bool     `json:"media_pass"`
	// MediaIssue is a human-readable detail for a MediaPass failure the generic "did not
	// attach the expected media" wording doesn't fit — today only the Forbid case
	// ("attached media, but this test forbids any attachment").
	MediaIssue string `json:"media_issue,omitempty"`
	// MediaCount/TooManyMedia are UNIVERSAL — checked on every test regardless of whether
	// it declares a `media:` expectation, mirroring every frame's own rule-3 attachment
	// cap (see maxMediaGroups). MediaCountEvaluated is a CODE-VERSION marker,
	// not derived from ParseOK: a verdict judged by pre-upgrade code has this field
	// entirely absent from its JSON, which unmarshals as false. Without this marker,
	// re-reading such a verdict would report TooManyMedia's zero value (false) as a
	// fabricated PASS for a check that was never actually run — MediaCountEvaluated lets
	// viewmodel.go render "not checked" instead.
	MediaCount          int  `json:"media_count"`
	TooManyMedia        bool `json:"too_many_media"`
	MediaCountEvaluated bool `json:"media_count_evaluated,omitempty"`
	EscalatePass        bool `json:"escalate_pass"`
	// EscalateTextConsistencyPass fails a row whose escalate is false while the
	// INJECTED reply text still hands the customer off to / deflects them to a manager
	// (see managerDeflectionPatterns) — the text-vs-flag contradiction observed
	// 2026-07-26: a model writing "I'll pass this to the manager" or "check the exact
	// terms with the manager" while leaving escalate=false, so the promise reaches the
	// customer but nothing downstream is ever told to act on it. Universal (every test,
	// no opt-in) and gates ModelBehaviorPass only — never ContractPass, never retry
	// candidacy (retry.go judges candidacy against an empty TestCase{}, which never sets
	// Escalate, so this check never even runs there). EscalateTextConsistencyEvaluated
	// is the CODE-VERSION + field-presence marker (MediaCountEvaluated pattern): a
	// verdict judged before this check existed, or a row whose ContractFields didn't
	// even include a parseable escalate value, renders "not run" rather than a
	// fabricated pass from the zero value.
	EscalateTextConsistencyPass      bool   `json:"escalate_text_consistency_pass"`
	EscalateTextConsistencyEvaluated bool   `json:"escalate_text_consistency_evaluated,omitempty"`
	DeflectionPhrase                 string `json:"deflection_phrase,omitempty"`
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
	Retries        int    `json:"retries,omitempty"`
	RetryRecovered bool   `json:"retry_recovered,omitempty"`
	RetryReason    string `json:"retry_reason,omitempty"`
	// FirstAttemptParseOK/FirstAttemptContractPass judge the ORIGINAL (attempt 0) output
	// independently of whichever attempt ended up selected — every other field on this
	// Verdict reflects the FINAL/selected attempt, so these are the only place a
	// report can recover "would the first try alone have passed" once a retry has
	// happened. Equal to this Verdict's own ParseOK/ContractPass when the row was never
	// retried (Attempts empty) — attempt 0 IS this verdict in that case. Computed by
	// judgeScenario/judgeSchemaKBRows, not judgeOne itself (judgeOne only ever sees one
	// output at a time).
	FirstAttemptParseOK      bool `json:"first_attempt_parse_ok"`
	FirstAttemptContractPass bool `json:"first_attempt_contract_pass"`
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

	// LLMCheckEvaluated is the CODE-VERSION + opt-in marker for TestCase.LLMChecks (same
	// convention as OutcomesDeclared/MediaCountEvaluated): false for every verdict the
	// plain `judge` command writes (it never reads or writes any LLMCheck field at all),
	// and false for a verdict whose test declares no llm_checks or whose ContractPass is
	// false (nothing sensible to judge). Only a run through the separate, opt-in
	// `judge-llm` command against a ContractPass row with llm_checks declared ever sets
	// this true — report.go/viewmodel.go must treat false as "not run," never as a
	// fabricated pass.
	LLMCheckEvaluated bool `json:"llm_check_evaluated,omitempty"`
	// LLMJudgePass is nil when not evaluated, else true only if EVERY declared claim's
	// judged verdict matched its Expect value (and none was Unverified). A *bool (not
	// bool) so "not evaluated" and "evaluated, failed" are never confused on
	// re-serialization — the same reasoning as every other nilable pass field here.
	// Deliberately NOT folded into ModelBehaviorPass or ContractPass: this is reported as
	// its own dimension, so the two core metrics stay identical whether or not judge-llm
	// ever runs (see judge_llm.go's doc comment).
	LLMJudgePass  *bool            `json:"llm_judge_pass,omitempty"`
	LLMJudgeModel string           `json:"llm_judge_model,omitempty"`
	LLMChecks     []LLMCheckResult `json:"llm_checks,omitempty"`
	// LLMJudgeCostUSD is this verdict's OWN spend from the run that last wrote its
	// LLMChecks — a run that reuses every claim from cache reports 0 here, honestly: no
	// new API call means no new spend, not a stale total carried forward.
	LLMJudgeCostUSD float64 `json:"llm_judge_cost_usd,omitempty"`

	// StockLLMEvaluated/LLMStockPass/StockLLMChecks/LLMStockModel/LLMStockCostUSD are
	// the auto-generated semantic stock-correctness dimension (2026-07 consolidation),
	// evaluated by the SAME opt-in `judge-llm` command as LLMChecks above but kept as a
	// wholly SEPARATE dimension: unlike LLMChecks (hand-declared per test), these are
	// synthesized from TestCase.StockCheckRef + the catalog's own ground-truth stock
	// state, never hand-authored, so they can never drift from the fixture. Never folds
	// into ContractPass/ModelBehaviorPass/LLMJudgePass — schema_kb_v1 only (a stock
	// classification needs a product ref to look up ground truth against).
	StockLLMEvaluated bool `json:"stock_llm_evaluated,omitempty"`
	// LLMStockPass is nil when not evaluated (StockLLMEvaluated false, e.g. the test
	// declares no StockCheckRef), else true only if the judge's classification matched
	// the catalog's ground-truth state AND the call was not Unverified.
	LLMStockPass    *bool                 `json:"llm_stock_pass,omitempty"`
	StockLLMChecks  []StockLLMCheckResult `json:"stock_llm_checks,omitempty"`
	LLMStockModel   string                `json:"llm_stock_model,omitempty"`
	LLMStockCostUSD float64               `json:"llm_stock_cost_usd,omitempty"`

	// LanguageRewriteEvaluated..LanguageRewriteCostUSD are the language safety net
	// (2026-07-27 design note; see rewrite_lang.go), the SEPARATE, opt-in `rewrite-lang`
	// command's own dimension — same doctrine as LLMCheckEvaluated/StockLLMEvaluated
	// above: false for every verdict the plain `judge` command writes, and every field
	// below stays at its zero value until `rewrite-lang` actually runs. On the happy
	// path (the target and the reply's own language already agree) rewrite-lang costs
	// nothing and sets only LanguageRewriteEvaluated + TargetLanguage — there is no
	// mismatch to record. See LanguageRewriteApplied's doc comment for what changes when
	// a rewrite IS applied.
	LanguageRewriteEvaluated bool `json:"language_rewrite_evaluated,omitempty"`
	// TargetLanguage is detectLang(customer message, history)'s verdict — the language
	// the reply SHOULD be in — recorded whenever LanguageRewriteEvaluated is true,
	// independent of whether a mismatch was ever found.
	TargetLanguage string `json:"target_language,omitempty"`
	// PreRewriteLanguage/PreRewriteInjectedText are set ONLY when a mismatch was
	// detected (TargetLanguage disagreed with classifyText's read of the pre-rewrite
	// InjectedText, or that text was too short/unclear to classify at all — an empty
	// PreRewriteLanguage means the latter). PreRewriteInjectedText is the discarded
	// original customer-facing text, kept for audit — the same role RawOutput plays for
	// the raw provider response, one level up the pipeline.
	PreRewriteLanguage     string `json:"pre_rewrite_language,omitempty"`
	PreRewriteInjectedText string `json:"pre_rewrite_injected_text,omitempty"`
	// LanguageRewriteApplied is true only when a mismatch was found AND the rewrite
	// model's replacement reply_text passed the full re-judge (judgeOne/judgeOneSchemaKB
	// against a synthetic row carrying the rewritten template) — at that point EVERY
	// other field on this Verdict (ParseOK, ContractPass, InjectedText,
	// ModelBehaviorPass, every requirement/behavior check, Reason, ...) already reflects
	// the REWRITTEN text, re-graded from scratch exactly as if the model had drafted it
	// that way originally. Only the fields that describe the ORIGINAL provider call
	// itself (RawOutput, Cost, LatencyMs, Tokens, TokensIn/Out, CostEstimateUSD,
	// CostBasis, Retries, Reasoning) are preserved from before the rewrite — the rewrite
	// model's own spend is tracked separately, in LanguageRewriteCostUSD. False with
	// PreRewriteLanguage non-empty means a mismatch WAS found but the rewrite was
	// discarded (LanguageRewriteFailReason explains why) — the verdict is then
	// COMPLETELY UNCHANGED from what plain `judge` produced, mismatch and all.
	LanguageRewriteApplied bool `json:"language_rewrite_applied,omitempty"`
	// LanguageRewriteFailReason is set only when a mismatch was found and the rewrite
	// attempt was discarded: the rewrite call itself failed (network/parse error), or
	// the rewritten template failed its re-judge (a corrupted/dropped placeholder is the
	// expected failure mode — judgeOne/judgeOneSchemaKB catch that exactly like they
	// would for any model's own mistake).
	LanguageRewriteFailReason string `json:"language_rewrite_fail_reason,omitempty"`
	// LanguageRewriteModel/LanguageRewriteCostUSD are set only when a rewrite call was
	// actually made (i.e. a mismatch was found) — the happy path never touches these,
	// which is the entire cost claim: $0 unless a mismatch is real.
	LanguageRewriteModel   string  `json:"language_rewrite_model,omitempty"`
	LanguageRewriteCostUSD float64 `json:"language_rewrite_cost_usd,omitempty"`
}

// LLMCheckResult is one TestCase.LLMChecks claim's judged outcome — see judge_llm.go.
type LLMCheckResult struct {
	Claim   string `json:"claim"`
	Expect  bool   `json:"expect"`
	Verdict bool   `json:"verdict"`
	Pass    bool   `json:"pass"`
	// Unverified is true when the judge call failed outright or its response could not
	// be parsed into a boolean verdict — Verdict/Pass are meaningless zero values in that
	// case (Pass is left false: fail-closed, an unverified claim is never silently
	// treated as passed).
	Unverified bool `json:"unverified"`
	// CacheKey is sha256(message, reply text, claim, judge model id) — a later judge-llm
	// run reuses this exact entry (no API call) when the key still matches; see
	// llmCheckCacheKey.
	CacheKey string `json:"cache_key"`
}

// StockLLMCheckResult is one auto-generated stock-correctness classification's judged
// outcome — see judge_llm.go's autoStockChecks/judgeStockClaim. Classification is one
// of "in_stock" | "out_of_stock" | "unclear" | "contradictory" (never a binary yes/no —
// this is the 4-way semantic classification the LLM judge produces, distinct from a
// plain LLMCheckResult).
type StockLLMCheckResult struct {
	ProductRef     string `json:"product_ref"`
	ExpectedState  string `json:"expected_state"` // "in_stock" | "out_of_stock" — from the catalog, never hand-authored
	Classification string `json:"classification"`
	Pass           bool   `json:"pass"`
	// Unverified is true when the judge call failed outright or its response could not
	// be parsed into a recognized classification — Classification/Pass are meaningless
	// zero values in that case (Pass left false: fail-closed).
	Unverified bool   `json:"unverified"`
	CacheKey   string `json:"cache_key"`
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
	// managerDeflectionPatterns catch a reply that hands the customer off to / deflects
	// them to a manager — used by managerDeflection to fail escalate=false rows that
	// contradict their own text (see Verdict.EscalateTextConsistencyPass). Regexes, not a
	// fixed substring list: the observed real failure «Уточняйте условия у менеджера при
	// оформлении заказа» (bd13, 2026-07-26) defeats any substring check anchored on
	// "уточняйте у менеджера" alone. The [^.!?\n]{0,40} window keeps every match inside
	// roughly one clause/sentence, so an unrelated later mention of "менеджер" elsewhere
	// in a long reply can't retroactively pair with an earlier verb. Deliberately NARROW
	// verb lists (not a bare "менеджер" anywhere in the text): the pickup topic's honest,
	// correctly non-escalating «...по предварительной договорённости с менеджером» must
	// NOT match (see judge_test.go's pinned negative control) — broadening these lists
	// must re-check that sentence.
	managerDeflectionPatterns = []unitIssuePattern{
		{label: "clarify-with-manager", re: regexp.MustCompile(`(?i)уточн[а-яё]*[^.!?\n]{0,40}менеджер`)},
		{label: "contact-the-manager", re: regexp.MustCompile(`(?i)(свяжитесь|связаться|обратитесь|обращайтесь|обратиться|напишите|позвоните|спросите|спросить|узнайте|узнать)[^.!?\n]{0,40}менеджер`)},
		{label: "handoff-promise", re: regexp.MustCompile(`(?i)переда(ю|м|ём|ем|дим|ст)[^.!?\n]{0,40}менеджер`)},
		{label: "manager-will-follow-up", re: regexp.MustCompile(`(?i)менеджер[^.!?\n]{0,40}(свяжется|ответит|перезвонит|напишет|уточнит|подскажет)`)},
		{label: "kk-forward-to-manager", re: regexp.MustCompile(`(?i)менеджерге[^.!?\n]{0,40}(жолда|жібер|бер|тапсыр|хабарла)`)},
		{label: "kk-ask-the-manager", re: regexp.MustCompile(`(?i)менеджер(ден|ге|мен)[^.!?\n]{0,40}(сұра|хабарлас|байланыс|жүгін)`)},
		{label: "kk-manager-will-contact", re: regexp.MustCompile(`(?i)менеджер[^.!?\n]{0,40}(хабарласады|жауап береді|байланысады)`)},
	}
)

// managerDeflection reports the first managerDeflectionPatterns match in text (the
// INJECTED reply, same convention as MustNotContain/firstForbidden) — matched is the
// literal substring matched, for DeflectionPhrase's failure detail.
func managerDeflection(text string) (matched string, hit bool) {
	for _, p := range managerDeflectionPatterns {
		if m := p.re.FindString(text); m != "" {
			return m, true
		}
	}
	return "", false
}

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

// maxMediaGroups mirrors every frame's own attachment cap — rule 3 in every frame.txt,
// "Максимум 2" (e.g. shop-scale/frame.txt, shop-decisions-v1/frame.txt,
// xpayment-decisions-v1/frame.txt, shop-current/frame.txt, frame-ru.txt). Every scenario
// shares this one cap now — the historical separate asset_refs cap of 3 no longer
// exists now that every scenario returns media_files_to_send. If a frame's stated cap
// ever changes, this constant must change with it.
const maxMediaGroups = 2

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
	inputs, err := resolveScenarioRunInputs(scenarioDir, runDir)
	if err != nil {
		return err
	}
	scenario := inputs.Scenario

	// Prefer this run's own snapshot of what render produced over the live, mutable
	// scenarios/*/generated/ — the whole point of snapshotting: re-judging an old run
	// must grade against the requirements THAT RUN saw, not whatever the scenario looks
	// like today. Legacy runs (no snapshot) and a standalone `judge` right after a fresh
	// `render` fall back to genDir/modelsPath exactly as before this existed.
	genDir := inputs.GeneratedDir
	resolvedModelsPath := modelsPath
	if inputs.SnapshotDir != "" {
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

	freshSplit := buildFreshSplit(results)

	var verdicts []Verdict
	if scenario.Pipeline == "schema_kb_v1" {
		// The public catalog deliberately excludes exact values and kbd_materials, so
		// schema judging rebuilds its private resolution context from the snapshotted
		// fixture selected by resolveScenarioRunInputs.
		verdicts, err = judgeSchemaKBRows(inputs.FixturePath, scenario, results, testByID, priceByModel, freshSplit)
		if err != nil {
			return err
		}
	} else {
		var catalog Catalog
		if err := readJSON(filepath.Join(genDir, "catalog.json"), &catalog); err != nil {
			return fmt.Errorf("read catalog.json (did you run render first?): %w", err)
		}
		tokenValue := map[string]string{}
		for _, f := range catalog.Tokens {
			tokenValue[f.Token] = f.Value
		}
		validMedia := map[string]bool{}
		for _, tok := range catalog.MediaTokens {
			validMedia[tok] = true
		}
		for _, row := range results.Results.Results {
			tc, ok := testByID[row.TestCase.Description]
			if !ok {
				continue // belongs to a different scenario's tests, if results were ever merged
			}
			v := judgeOne(tc, row, tokenValue, validMedia, catalog.TrustedDigits)
			v.FirstAttemptParseOK, v.FirstAttemptContractPass = firstAttemptOutcome(row, v, func(r PromptfooRow) Verdict {
				return judgeOne(TestCase{}, r, tokenValue, validMedia, catalog.TrustedDigits)
			})
			applyCostEstimate(&v, row, priceByModel, freshSplit)
			verdicts = append(verdicts, v)
		}
	}

	out := JudgedRun{Scenario: scenario.Name, Verdicts: verdicts}
	outPath := filepath.Join(runDir, scenario.Name+".judged.json")
	if err := writeJSON(outPath, out); err != nil {
		return err
	}
	fmt.Printf("judged %s: %d verdicts -> %s\n", scenario.Name, len(verdicts), outPath)
	return nil
}

// firstAttemptOutcome reports whether row's ORIGINAL (attempt 0) output would itself
// have parsed and passed the strict contract, independent of whichever attempt ended up
// selected. A row never retried (Attempts empty) has attempt 0 already reflected in v,
// so its own ParseOK/ContractPass is returned directly rather than re-judging identical
// input. Otherwise judgeOneOnAttempt0 re-judges a copy of row with Response.Output
// swapped to Attempts[0].Output, against an empty TestCase{} — only contract shape
// matters here, not any test's specific requires/media/escalate/language expectations
// (those are about MODEL BEHAVIOR on the ultimately-selected attempt, a different
// question from "did the pipeline itself hold up on the first try").
func firstAttemptOutcome(row PromptfooRow, v Verdict, judgeOneOnAttempt0 func(PromptfooRow) Verdict) (parseOK, contractPass bool) {
	if len(row.Attempts) == 0 {
		return v.ParseOK, v.ContractPass
	}
	synthetic := row
	synthetic.Response.Output = row.Attempts[0].Output
	fv := judgeOneOnAttempt0(synthetic)
	return fv.ParseOK, fv.ContractPass
}

// buildFreshSplit lets a cached row (promptfoo reports prompt:0, completion:0 on a cache
// hit) borrow the real in/out split from another row in THIS SAME run that made a fresh
// call for the identical (model, test) — a cache hit means an identical request would
// have cost the same, so the split is still a valid estimate, just not this row's own
// measurement. Shared by both the legacy and schema_kb_v1 judging paths.
func buildFreshSplit(results PromptfooResults) map[string]tokenSplit {
	freshSplit := map[string]tokenSplit{}
	for _, row := range results.Results.Results {
		if !row.Response.Cached && row.TokenUsage.Prompt > 0 {
			key := providerModelKey(row.Provider.ID, row.Provider.Label) + "|" + row.TestCase.Description
			freshSplit[key] = tokenSplit{row.TokenUsage.Prompt, row.TokenUsage.Completion}
		}
	}
	return freshSplit
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

func judgeOne(tc TestCase, row PromptfooRow, tokenValue map[string]string, validMedia map[string]bool, trustedDigits []string) Verdict {
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
		RetryReason:  row.RetryReason,
		// MediaResolveOK has no meaning on the legacy pipeline (nothing to resolve — its
		// data.yaml never had a kbd_materials-shaped registry to re-validate against), so
		// it is fixed true here rather than left at bool's false zero value, which would
		// otherwise misreport as a failure.
		MediaResolveOK: true,
	}

	obj, ok := extractModelJSON(row.Response.Output, &v)
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
	// presence-only check would have let that through as a valid contract. The four
	// OPERATIONAL fields are checked (DECISIONS.md §"Customer-response JSON contract");
	// escalation_reason and confidence are diagnostic-only telemetry — absent,
	// mistyped, or out-of-range diagnostics never fail an otherwise valid reply (the
	// raw output preserves whatever the model actually wrote there).
	replyText, hasReplyText := obj["reply_text"].(string)
	_, hasLang := obj["reply_language"].(string)
	escalateVal, hasEscalate := obj["escalate"].(bool)
	// mediaRaw is the UNTYPED array — kept around (not just mediaEntries' string-filtered
	// view) so MediaCount below counts every entry the model actually wrote, including a
	// stray non-string one, instead of silently undercounting by dropping it first.
	mediaRaw, hasMediaField := obj[mediaFilesField].([]any)
	mediaAllStrings := true
	for _, e := range mediaRaw {
		if _, ok := e.(string); !ok {
			mediaAllStrings = false
			break
		}
	}
	v.ContractFields = hasReplyText && replyText != "" && hasLang && hasEscalate &&
		hasMediaField && mediaAllStrings
	if !v.ContractFields {
		v.appendReason(fmt.Sprintf("missing or wrong-typed contract field (need string reply_text, string reply_language, bool escalate, array %s of strings)", mediaFilesField))
	}
	// escalation_reason/escalate consistency is a diagnostic observation, not a
	// contract gate — EscalationReasonOK stays true unconditionally (field kept so
	// old judged.json rows keep their meaning on re-read).
	v.EscalationReasonOK = true

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
	// block the whole draft rather than ship a half-rendered fact. Only reply_text is
	// scanned: escalation_reason is diagnostic-only — placeholders or braces there are
	// never substituted and never a failure (the field is never shown to the customer).
	spans := tokenSpanRE.FindAllString(replyText, -1)
	injected := replyText
	for _, tok := range spans {
		val, known := tokenValue[tok]
		if !known {
			v.UnknownTokens = append(v.UnknownTokens, tok)
			continue
		}
		injected = strings.ReplaceAll(injected, tok, val)
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
	v.ContractPass = v.ParseOK && v.ContractFields &&
		!v.Blocked && !v.LeftoverBraces && !v.Truncated && !v.ReasoningLeak && !v.ControlChars

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
	v.InventedDigits = filterInventedDigits(digitRunRE.FindAllString(stripped, -1), tc.Message, trustedDigits)

	v.RequiresPass = requiresSatisfied(tc.Requires, replyText, tokenValue)
	v.ForbiddenTokensHit = forbiddenTokensHit(tc.ForbidTokens, replyText)
	v.ForbidTokensPass = len(v.ForbiddenTokensHit) == 0

	entries := mediaEntries(obj)
	v.MediaPass = true
	if tc.Media != nil {
		v.MediaPass, v.MediaIssue = checkMediaExpect(tc.Media, entries)
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
	v.TooManyMedia = v.MediaCount > maxMediaGroups

	v.EscalatePass = true
	if tc.Escalate != nil {
		v.EscalatePass = escalateVal == *tc.Escalate
	}

	// Universal, independent of whether the test declares an `escalate:` expectation at
	// all (unlike EscalatePass above) — gated only on the contract actually having a
	// parseable escalate value to check against (hasEscalate; a missing/mistyped field is
	// already a ContractFields failure, no need to stack a second, noisier complaint on
	// top of it). See Verdict.EscalateTextConsistencyPass's doc comment.
	v.EscalateTextConsistencyPass = true
	if hasEscalate {
		v.EscalateTextConsistencyEvaluated = true
		if !escalateVal {
			if m, hit := managerDeflection(injected); hit {
				v.EscalateTextConsistencyPass = false
				v.DeflectionPhrase = m
			}
		}
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
			replyText:    replyText,
			injected:     injected,
			checkText:    checkText,
			replyLang:    replyLang,
			escalate:     escalateVal,
			mediaEntries: entries,
			tokenValue:   tokenValue,
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

	for _, entry := range entries {
		if !validMedia[entry] {
			v.UnknownMedia = append(v.UnknownMedia, entry)
		}
	}

	v.ModelBehaviorPass = v.RequiresPass && v.ForbidTokensPass && v.MediaPass && v.EscalatePass && v.EscalateTextConsistencyPass && v.LanguagePass &&
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
	case !v.ForbidTokensPass:
		return "reply_text cites a forbidden fact token: " + strings.Join(v.ForbiddenTokensHit, ", ")
	case !v.MediaPass:
		if v.MediaIssue != "" {
			return v.MediaIssue
		}
		return "did not attach the expected media"
	case !v.EscalatePass:
		return "escalate did not match expectation"
	case v.EscalateTextConsistencyEvaluated && !v.EscalateTextConsistencyPass:
		return "reply hands the customer off to a manager while escalate=false: \"" + v.DeflectionPhrase + "\""
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

// extractModelJSON locates the model's FINAL customer-response JSON in raw provider
// output that may combine reasoning text with the answer (aiprompt.ExtractFinalOutput —
// direct JSON, one outer fence, or the last complete operational object inside combined
// text; a truncated/incomplete object is never repaired and fails). On success the
// extraction record (final span, non-final remainder, method) is written onto v; raw
// itself is never modified — v.RawOutput keeps the provider response byte-for-byte.
// Distinct from parseModelJSON on purpose: that one stays a plain fence-tolerant syntax
// parse for outputs that are NOT customer-response objects (e.g. the judge-llm verdict
// {"verdict": true}, which extraction would rightly refuse for having no operational
// fields).
func extractModelJSON(raw string, v *Verdict) (map[string]any, bool) {
	ex, ok := aiprompt.ExtractFinalOutput(raw)
	if !ok {
		return nil, false
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(ex.Final), &obj); err != nil {
		return nil, false // unreachable in practice: ex.Final is a validated JSON object
	}
	if v != nil {
		v.FinalOutput = ex.Final
		v.NonFinalOutput = ex.NonFinal
		v.ExtractionMethod = ex.Method
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

// forbiddenTokensHit scans every {{table.ref.column}} span in replyText (the RAW,
// pre-substitution reply — forbid_tokens is about which fact the model picked, not what
// the injected text reads) and returns the spans matching one of forbid's entries. An
// entry ending in "." is a dotted PREFIX (e.g. "delivery." matches every
// "delivery.<zone>.*" token); any other entry must match the span's dotted form exactly.
// Returns nil (not just empty) when forbid is empty, so callers can treat a nil result as
// "no forbid_tokens declared" the same way MustNotContain's absence is vacuously true.
func forbiddenTokensHit(forbid []string, replyText string) []string {
	if len(forbid) == 0 {
		return nil
	}
	var hits []string
	for _, span := range tokenSpanRE.FindAllString(replyText, -1) {
		dotted := strings.TrimSuffix(strings.TrimPrefix(span, "{{"), "}}")
		for _, f := range forbid {
			if strings.HasSuffix(f, ".") {
				if strings.HasPrefix(dotted, f) {
					hits = append(hits, span)
					break
				}
			} else if dotted == f {
				hits = append(hits, span)
				break
			}
		}
	}
	return hits
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

// checkMediaExpect evaluates one MediaExpect block (Forbid / any_of / all_of / Exclusive)
// against the reply's already-extracted media_files_to_send entries — judgeOne's
// universal media check verbatim, extracted so OutcomeCase blocks share it. Returns pass
// plus the human-readable issue detail for the failure modes that have one.
func checkMediaExpect(exp *MediaExpect, entries []string) (bool, string) {
	if exp.Forbid {
		if len(entries) > 0 {
			return false, "attached media, but this test forbids any attachment"
		}
		return true, ""
	}
	if !mediaExpectationMet(exp, entries) {
		return false, ""
	}
	if exp.Exclusive {
		if outsiders := mediaOutsideExpectation(exp, entries); len(outsiders) > 0 {
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
// all computed once by the caller (judgeOne or judgeOneSchemaKB) and shared by every
// block, so evaluating N alternatives never re-derives (or diverges on) what the reply
// actually said.
type outcomeInputs struct {
	replyText    string // raw model reply_text (requires matches the un-substituted {{token}} spans)
	injected     string // token-injected text (must_not_contain / must_contain_any scan this)
	checkText    string // language-check text: InjectedText with the caller's replyText fallback
	replyLang    string // the model's declared reply_language field
	escalate     bool   // the model's escalate field (type-checked earlier)
	mediaEntries []string
	tokenValue   map[string]string
}

// outcomeSatisfied evaluates ONE OutcomeCase block: every check the block declares must
// pass; a knob left absent is vacuously true — the same convention as absent
// TestCase-level knobs. Each check delegates to the same helper judgeOne's universal
// checks use, so block semantics and top-level semantics are one implementation.
func outcomeSatisfied(oc OutcomeCase, in outcomeInputs) bool {
	if !requiresSatisfied(oc.Requires, in.replyText, in.tokenValue) {
		return false
	}
	if len(forbiddenTokensHit(oc.ForbidTokens, in.replyText)) > 0 {
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
		if pass, _ := checkMediaExpect(oc.Media, in.mediaEntries); !pass {
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

// mediaFilesField is the ONE canonical response property every scenario returns media
// selections in, regardless of pipeline or which token vocabulary this scenario's own
// catalog happens to populate it with (DECISIONS.md §"Customer-response JSON contract").
const mediaFilesField = "media_files_to_send"

func mediaEntries(obj map[string]any) []string {
	raw, ok := obj[mediaFilesField].([]any)
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

func hasEntry(entries []string, want string) bool {
	for _, e := range entries {
		if e == want {
			return true
		}
	}
	return false
}

// mediaExpectationMet checks entries against exp's AnyOf (at least one present) and
// AllOf (every one present) declarations — both vacuously true when empty, same
// "nothing declared, nothing to fail" convention as every other optional TestCase knob.
func mediaExpectationMet(exp *MediaExpect, entries []string) bool {
	for _, w := range exp.AllOf {
		if !hasEntry(entries, w) {
			return false
		}
	}
	if len(exp.AnyOf) > 0 {
		met := false
		for _, w := range exp.AnyOf {
			if hasEntry(entries, w) {
				met = true
				break
			}
		}
		if !met {
			return false
		}
	}
	return true
}

// mediaOutsideExpectation returns every attached entry NOT in the test's declared
// any_of/all_of union — used only when Exclusive is true, on top of
// mediaExpectationMet's existing "these must be present" check, to enforce "this set AND
// NOTHING ELSE".
func mediaOutsideExpectation(exp *MediaExpect, entries []string) []string {
	allowed := map[string]bool{}
	for _, w := range exp.AnyOf {
		allowed[w] = true
	}
	for _, w := range exp.AllOf {
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
