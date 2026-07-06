package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	} `json:"provider"`
	TestCase struct {
		Description string `json:"description"`
	} `json:"testCase"`
	Response struct {
		Output string `json:"output"`
	} `json:"response"`
	Cost       float64 `json:"cost"`
	LatencyMs  int     `json:"latencyMs"`
	TokenUsage struct {
		Total int `json:"total"`
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

	RequiresPass       bool     `json:"requires_pass"`
	MediaPass          bool     `json:"media_pass"`
	EscalatePass       bool     `json:"escalate_pass"`
	LanguagePass       bool     `json:"language_pass"`
	MustNotContainPass bool     `json:"must_not_contain_pass"`
	ForbiddenPhrase    string   `json:"forbidden_phrase,omitempty"`
	InventedDigits     []string `json:"invented_digits"`
	UnitIssues         []string `json:"unit_issues"`

	// ContractPass: did the pipeline behave safely regardless of what the model said
	// (parses, right shape, every token resolves, no leftover brace). This is the
	// property that must never fail once this design ships for real.
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
}

// JudgedRun is generated per run: runs/<id>/<scenario>.judged.json.
type JudgedRun struct {
	Scenario string    `json:"scenario"`
	Verdicts []Verdict `json:"verdicts"`
}

var (
	fenceOpenRE  = regexp.MustCompile("^\\s*```[a-zA-Z]*\\s*")
	fenceCloseRE = regexp.MustCompile("\\s*```\\s*$")
	tokenSpanRE  = regexp.MustCompile(`\{\{[^}]*\}\}`)
	digitRunRE   = regexp.MustCompile(`\d{2,}`)
	unitIssueREs = []unitIssuePattern{
		{label: "duplicated tenge symbol", re: regexp.MustCompile(`₸\s*₸`)},
		{label: "duplicated tenge word", re: regexp.MustCompile(`₸\s*тенге`)},
		{label: "duplicated Алматы qualifier", re: regexp.MustCompile(`(?i)по\s+алматы\s+по\s+алматы`)},
		{label: "duplicated day unit", re: regexp.MustCompile(`(?i)(дня|дней)\s+(дня|дней)`)},
		{label: "mixed Russian/Kazakh day-unit hint", re: regexp.MustCompile(`(?i)дня\s*/\s*күнде`)},
	}
)

const kazakhOnlyLetters = "әғқңөұүһӘҒҚҢӨҰҮҺ"

type unitIssuePattern struct {
	label string
	re    *regexp.Regexp
}

func cmdJudge(args []string) error {
	fs := flag.NewFlagSet("judge", flag.ExitOnError)
	scenarioDir := fs.String("scenario", "", "path to the scenario directory")
	runDir := fs.String("run", "", "path to the run directory (contains results.json)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *scenarioDir == "" || *runDir == "" {
		return fmt.Errorf("judge: -scenario and -run are both required")
	}
	return judgeScenario(*scenarioDir, *runDir)
}

func judgeScenario(scenarioDir, runDir string) error {
	scenario, err := loadScenario(scenarioDir)
	if err != nil {
		return fmt.Errorf("load scenario.yaml: %w", err)
	}

	var catalog Catalog
	if err := readJSON(filepath.Join(scenarioDir, "generated", "catalog.json"), &catalog); err != nil {
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
	if err := readJSON(filepath.Join(scenarioDir, "generated", "resolved_tests.json"), &resolved); err != nil {
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

	var verdicts []Verdict
	for _, row := range results.Results.Results {
		tc, ok := testByID[row.TestCase.Description]
		if !ok {
			continue // belongs to a different scenario's tests, if results were ever merged
		}
		verdicts = append(verdicts, judgeOne(tc, row, &catalog, tokenValue, validMediaRef, validMediaGroup))
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
		TestID:    tc.ID,
		Model:     row.Provider.ID,
		Message:   tc.Message,
		RawOutput: row.Response.Output,
		Cost:      row.Cost,
		LatencyMs: row.LatencyMs,
		Tokens:    row.TokenUsage.Total,
	}

	obj, ok := parseModelJSON(row.Response.Output)
	v.ParseOK = ok
	if !ok {
		v.Reason = "could not parse JSON output"
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

	// Fail-closed: every token the model used must resolve, or the real product would
	// block the whole draft rather than ship a half-rendered fact.
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
		v.Reason = "unknown token(s), draft would be BLOCKED: " + strings.Join(v.UnknownTokens, ", ")
	} else {
		v.InjectedText = injected
		if strings.Contains(injected, "{{") {
			v.LeftoverBraces = true
			v.Reason = "leftover '{{' survived injection"
		}
	}
	v.ContractPass = v.ParseOK && v.ContractFields && !v.Blocked && !v.LeftoverBraces

	// Model-behavior checks (only meaningful once we know the contract held).
	stripped := tokenSpanRE.ReplaceAllString(replyText, "")
	v.InventedDigits = digitRunRE.FindAllString(stripped, -1)

	v.RequiresPass = requiresSatisfied(tc.Requires, replyText, tokenValue)

	v.MediaPass = true
	if tc.Media != nil {
		v.MediaPass = mediaExpectationMet(tc.Media, obj, mediaField)
	}

	v.EscalatePass = true
	if tc.Escalate != nil {
		v.EscalatePass = escalateVal == *tc.Escalate
	}

	v.LanguagePass = true
	if tc.Language == "kk" {
		checkText := v.InjectedText
		if checkText == "" {
			checkText = replyText // e.g. blocked before injection — still check what the model wrote
		}
		v.LanguagePass = looksKazakh(checkText)
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
		return "reply does not look like Kazakh (too few Kazakh-specific letters)"
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
	cleaned := fenceOpenRE.ReplaceAllString(raw, "")
	cleaned = fenceCloseRE.ReplaceAllString(cleaned, "")
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
