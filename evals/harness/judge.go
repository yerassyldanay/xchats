package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	EscalatePass bool `json:"escalate_pass"`
	// LanguagePass = LanguageTextOK && LanguageFieldOK — kept as the single pass/fail
	// gate everything else already depends on. The two components are ALSO reported
	// separately (Phase 0.4 of the language plan): looksKazakh is a cheap heuristic, not
	// a whole-reply classifier, so telling "the text didn't look Kazakh" apart from "the
	// model declared the wrong reply_language" matters when manually inspecting a Kazakh
	// canary run, not just when reading the one combined boolean.
	LanguagePass       bool     `json:"language_pass"`
	LanguageTextOK     bool     `json:"language_text_ok"`
	LanguageFieldOK    bool     `json:"language_field_ok"`
	LanguageIssue      string   `json:"language_issue,omitempty"`
	MustNotContainPass bool     `json:"must_not_contain_pass"`
	ForbiddenPhrase    string   `json:"forbidden_phrase,omitempty"`
	InventedDigits     []string `json:"invented_digits"`
	UnitIssues         []string `json:"unit_issues"`

	// FinishReason/Truncated and ReasoningLeak are both pipeline-safety facts, folded
	// into ContractPass unconditionally — independent of model, prompt variant, or
	// whether reasoning was even requested for this call (see their doc comments where
	// computed, in judgeOne).
	FinishReason  string `json:"finish_reason,omitempty"`
	Truncated     bool   `json:"truncated"`
	ReasoningLeak bool   `json:"reasoning_leak"`

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

var (
	tokenSpanRE = regexp.MustCompile(`\{\{[^}]*\}\}`)
	// digitRunRE matches ANY digit run — a model inventing even a single-digit fact (e.g.
	// "осталось 5 штук" without the token) is still an invented fact. listMarkerRE strips
	// numbered-list markers ("1. ", "2) " at line start) first, the one legitimate place a
	// model writes its own digits (step-by-step ordering instructions) — confirmed against
	// a real run that models use BOTH "1." and "1)" for the same kind of list.
	digitRunRE   = regexp.MustCompile(`\d+`)
	listMarkerRE = regexp.MustCompile(`(?m)^\s*\d+[.)]\s`)
	unitIssueREs = []unitIssuePattern{
		{label: "duplicated tenge symbol", re: regexp.MustCompile(`₸\s*₸`)},
		{label: "duplicated tenge word", re: regexp.MustCompile(`₸\s*тенге`)},
		{label: "duplicated Алматы qualifier", re: regexp.MustCompile(`(?i)по\s+алматы\s+по\s+алматы`)},
		{label: "duplicated day unit", re: regexp.MustCompile(`(?i)(дня|дней)\s+(дня|дней)`)},
		{label: "mixed Russian/Kazakh day-unit hint", re: regexp.MustCompile(`(?i)дня\s*/\s*күнде`)},
	}
)

const kazakhOnlyLetters = "әғқңөұүһӘҒҚҢӨҰҮҺ"

// providerModelKey is the ONE key every grouping of judged results (Verdict.Model,
// priceByModel, freshSplit) must use once two provider entries can share the same
// underlying ID — e.g. a reasoning-on/reasoning-off comparison pair (ModelProvider.Label
// in types.go). Without this, two such entries would silently merge into one bucket
// downstream (report.go's byModel, viewmodel.go), corrupting exactly the side-by-side
// comparison a Label exists to enable. Label empty (every model before this existed)
// returns id unchanged, so nothing about today's single-entry-per-model runs changes.
func providerModelKey(id, label string) string {
	if label == "" {
		return id
	}
	return id + " [" + label + "]"
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
	}

	obj, ok := parseModelJSON(row.Response.Output)
	v.ParseOK = ok
	if !ok {
		v.Reason = "could not parse JSON output"
		if v.Truncated {
			v.Reason += fmt.Sprintf(" (response was truncated, finish_reason=%s)", v.FinishReason)
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
	_, hasMediaField := obj[mediaField].([]any)
	v.ContractFields = hasReplyText && replyText != "" && hasLang && hasEscalate && hasMediaField
	if !v.ContractFields {
		v.Reason = fmt.Sprintf("missing or wrong-typed contract field (need string reply_text, string reply_language, bool escalate, array %s)", mediaField)
	}

	// Reasoning/thinking content must never leak into reply_text — the one field the
	// real product would forward to a customer after human review. Scans replyText
	// directly, not v.InjectedText: InjectedText is only set once injection succeeds
	// (see the Blocked branch below), so a response that's ALSO blocked would otherwise
	// have nothing set to scan and silently miss the leak. Unconditional — not gated on
	// whether this call even requested reasoning (ReasoningConfig) — because a model can
	// emit inline <think> tags on its own, independent of what was asked for.
	v.ReasoningLeak = evaltext.HasReasoningMarkers(replyText)
	if v.ReasoningLeak && v.Reason == "" {
		v.Reason = "reasoning/thinking content leaked into reply_text"
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
		v.Reason = "unknown token(s), draft would be BLOCKED: " + strings.Join(v.UnknownTokens, ", ")
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
		if v.Reason == "" {
			v.Reason = "leftover brace survived injection"
		}
	}
	if v.Truncated && v.Reason == "" {
		v.Reason = fmt.Sprintf("response truncated before completion (finish_reason=%s)", v.FinishReason)
	}
	v.ContractPass = v.ParseOK && v.ContractFields && !v.Blocked && !v.LeftoverBraces && !v.Truncated && !v.ReasoningLeak

	// Model-behavior checks (only meaningful once we know the contract held).
	stripped := tokenSpanRE.ReplaceAllString(replyText, "")
	stripped = listMarkerRE.ReplaceAllString(stripped, "")
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
		v.MediaPass = mediaExpectationMet(tc.Media, obj, mediaField)
	}

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
		var textOK bool
		switch tc.Language {
		case "kk":
			textOK = looksKazakh(checkText)
		case "ru":
			textOK = !looksKazakh(checkText)
		}
		fieldOK := replyLang == tc.Language
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
		lower := strings.ToLower(replyText)
		for _, phrase := range tc.MustNotContain {
			if strings.Contains(lower, strings.ToLower(phrase)) {
				v.MustNotContainPass = false
				v.ForbiddenPhrase = phrase
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
		v.MustNotContainPass && len(v.InventedDigits) == 0 && len(v.UnitIssues) == 0 && len(v.UnknownMedia) == 0

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
		return "did not attach the expected media"
	case !v.EscalatePass:
		return "escalate did not match expectation"
	case !v.LanguagePass:
		return v.LanguageIssue
	case !v.MustNotContainPass:
		return "escalated, but reply_text still commits to an invented answer (\"" + v.ForbiddenPhrase + "\")"
	case len(v.InventedDigits) > 0:
		return "invented digits outside any token: " + strings.Join(v.InventedDigits, ", ")
	case len(v.UnitIssues) > 0:
		return "unit/currency issue after injection: " + strings.Join(v.UnitIssues, ", ")
	case len(v.UnknownMedia) > 0:
		return "attached media not in the catalog: " + strings.Join(v.UnknownMedia, ", ")
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
