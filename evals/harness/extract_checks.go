package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"xchats-evals-harness/internal/evaltext"
)

var currencyRE = regexp.MustCompile(`(?i)₸|тенге|\bтг\b|kzt|\$`)

// parseExtractionJSON decodes a model's raw answer into ExtractionResult, tolerating a
// ```json ... ``` fence around the object (some models wrap fenced code even when asked
// for response_format: json_object).
func parseExtractionJSON(raw string) (ExtractionResult, error) {
	cleaned := evaltext.StripFences(raw)
	var r ExtractionResult
	if err := json.Unmarshal([]byte(cleaned), &r); err != nil {
		return ExtractionResult{}, err
	}
	return r, nil
}

// runExtractChecks grades one parsed ExtractionResult against its case's requirements.
// Every check is deterministic string/number matching — no LLM judge, matching this
// harness's existing judge.go philosophy.
func runExtractChecks(c ExtractCase, r ExtractionResult) []CheckResult {
	var out []CheckResult

	if len(c.Fields) > 0 {
		keys := make([]string, 0, len(c.Fields))
		for k := range c.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			want := c.Fields[key]
			got, known := fieldValue(r, key)
			switch {
			case !known:
				out = append(out, CheckResult{Name: "field:" + key, Pass: false,
					Detail: fmt.Sprintf("unknown field %q in case definition", key)})
			case !fieldMatchesAny(got, want):
				out = append(out, CheckResult{Name: "field:" + key, Pass: false,
					Detail: fmt.Sprintf("want %q, got %q", want, got)})
			default:
				out = append(out, CheckResult{Name: "field:" + key, Pass: true})
			}
		}
	}

	if len(c.TextContainsAll) > 0 {
		out = append(out, containsAllCheck("text_contains_all", evaltext.NormalizeText(r.ExtractedText), c.TextContainsAll))
	}
	if len(c.IdentifyContainsAll) > 0 {
		identify := evaltext.NormalizeText(r.Summary + " " + r.RelatesToHint)
		out = append(out, containsAllCheck("identify_contains_all", identify, c.IdentifyContainsAll))
	}
	if len(c.IdentifyContainsAny) > 0 {
		identify := evaltext.NormalizeText(r.Summary + " " + r.RelatesToHint)
		out = append(out, containsAnyCheck("identify_contains_any", identify, c.IdentifyContainsAny))
	}

	// Invented-number check always runs (fail-closed default: an absent allowed_numbers
	// key means "no numbers may appear", not "skip this check") — every case in
	// cases.yaml is expected to declare it explicitly, even as an empty list.
	out = append(out, inventedNumberCheck(c, r))

	// Reasoning-leak check always runs, unconditionally — not opt-in per case, and not
	// gated on whether this model/call ever requested reasoning (a model can emit inline
	// <think> tags on its own). Higher stakes here than in judge.go's analogous check:
	// -record freezes a fully-passing result straight into extract/fixtures/<case>.json,
	// reused by every future run — a missed leak here doesn't just fail one run, it
	// permanently corrupts a fixture.
	out = append(out, reasoningLeakCheck(r))

	// Required-number check is opt-in (a case with nothing recall-critical simply
	// doesn't declare it) — the recall complement to the precision-only check above.
	if len(c.RequiredNumbers) > 0 {
		out = append(out, requiredNumberCheck(c, r))
	}

	if c.ForbidCurrency {
		haystack := strings.ToLower(r.ExtractedText + " " + r.Summary + " " + r.RelatesToHint)
		if loc := currencyRE.FindString(haystack); loc != "" {
			out = append(out, CheckResult{Name: "forbid_currency", Pass: false,
				Detail: fmt.Sprintf("found currency mention %q, but this file shows no price", loc)})
		} else {
			out = append(out, CheckResult{Name: "forbid_currency", Pass: true})
		}
	}

	return out
}

// fieldMatchesAny checks `got` against `want`, where `want` may be several
// pipe-separated acceptable values (e.g. "infographic|tutorial") — for the rare field
// where a real image honestly straddles two categories in our own vocabulary (observed:
// a multi-panel numbered flow graphic gets called "tutorial" by one model, "infographic"
// by another; neither is wrong, the vocabulary boundary is genuinely fuzzy). This is not
// a way to launder an arbitrary wrong answer — cases should list only the alternatives
// that are each independently defensible for that specific file.
func fieldMatchesAny(got, want string) bool {
	for _, alt := range strings.Split(want, "|") {
		if evaltext.NormalizeText(got) == evaltext.NormalizeText(alt) {
			return true
		}
	}
	return false
}

func fieldValue(r ExtractionResult, key string) (string, bool) {
	switch key {
	case "content_kind":
		return r.ContentKind, true
	case "summary":
		return r.Summary, true
	case "extracted_text":
		return r.ExtractedText, true
	case "language":
		return r.Language, true
	case "visibility_suggestion":
		return r.VisibilitySuggestion, true
	case "media_role_hint":
		return r.MediaRoleHint, true
	case "relates_to_hint":
		return r.RelatesToHint, true
	default:
		return "", false
	}
}

func containsAllCheck(name, haystack string, wants []string) CheckResult {
	var missing []string
	for _, w := range wants {
		if !strings.Contains(haystack, evaltext.NormalizeText(w)) {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		return CheckResult{Name: name, Pass: false,
			Detail: "missing: " + strings.Join(missing, ", ")}
	}
	return CheckResult{Name: name, Pass: true}
}

func containsAnyCheck(name, haystack string, wants []string) CheckResult {
	for _, w := range wants {
		if strings.Contains(haystack, evaltext.NormalizeText(w)) {
			return CheckResult{Name: name, Pass: true}
		}
	}
	return CheckResult{Name: name, Pass: false,
		Detail: "none of: " + strings.Join(wants, ", ")}
}

// inventedNumberCheck finds every number-like run in the model's text fields and fails
// if any of them is not in the case's allow-list (normalized the same way) — this is the
// eval's stand-in for DECISIONS.md's core rule that a model must never invent an exact
// value it cannot see.
func inventedNumberCheck(c ExtractCase, r ExtractionResult) CheckResult {
	allowed := map[string]bool{}
	for _, a := range c.AllowedNumbers {
		allowed[evaltext.CanonicalNumber(evaltext.NormalizeText(a))] = true
	}
	// evaltext.FoldNBSP, NOT evaltext.NormalizeText: a real line break is a genuine hard
	// boundary between two numbers (e.g. an address "77/7" immediately followed, on the
	// next line, by an unrelated count "2 кассира") — NormalizeText's newline-to-space
	// collapsing would destroy that boundary before evaltext.NumberRunRE ever sees it, merging
	// two unrelated numbers into one bogus "invented" one. Case doesn't matter for
	// digit/separator matching, so no lowercasing is needed here either.
	haystack := evaltext.FoldNBSP(r.ExtractedText + " " + r.Summary + " " + r.RelatesToHint)
	found := evaltext.NumberRunRE.FindAllString(haystack, -1)

	var invented []string
	seen := map[string]bool{}
	for _, n := range found {
		key := evaltext.CanonicalNumber(n)
		if allowed[key] || seen[key] {
			continue
		}
		seen[key] = true
		invented = append(invented, n) // report as originally seen (readable in Detail)
	}
	if len(invented) > 0 {
		return CheckResult{Name: "no_invented_numbers", Pass: false,
			Detail: "not in the allowed list: " + strings.Join(invented, ", ")}
	}
	return CheckResult{Name: "no_invented_numbers", Pass: true}
}

// reasoningLeakCheck scans every one of ExtractionResult's free-text fields for a
// <think>/<thinking> tag marker (evaltext.HasReasoningMarkers) — the same real, observed
// OpenRouter failure mode judge.go's analogous check guards against, applied here to the
// extraction eval's own free-text fields instead of a chat reply_text.
func reasoningLeakCheck(r ExtractionResult) CheckResult {
	haystack := strings.Join([]string{r.Summary, r.ExtractedText, r.RelatesToHint}, " ")
	if evaltext.HasReasoningMarkers(haystack) {
		return CheckResult{Name: "no_reasoning_leak", Pass: false,
			Detail: "reasoning/thinking content leaked into a customer-facing text field"}
	}
	return CheckResult{Name: "no_reasoning_leak", Pass: true}
}

// requiredNumberCheck is the RECALL complement to inventedNumberCheck's PRECISION-only
// check (Phase 0.4 of the language/extraction plan): allowed_numbers proves a model
// didn't invent anything, but says nothing about whether it dropped a real, visible,
// business-relevant figure on a busy multi-panel image — a failure mode observed in
// practice (a model correctly avoiding invention while also silently omitting a phone
// number or price). Every entry in c.RequiredNumbers must be found, canonicalized the
// same way as inventedNumberCheck, somewhere in the model's text fields.
func requiredNumberCheck(c ExtractCase, r ExtractionResult) CheckResult {
	haystack := evaltext.FoldNBSP(r.ExtractedText + " " + r.Summary + " " + r.RelatesToHint)
	found := map[string]bool{}
	for _, n := range evaltext.NumberRunRE.FindAllString(haystack, -1) {
		found[evaltext.CanonicalNumber(n)] = true
	}

	var missing []string
	for _, want := range c.RequiredNumbers {
		if !found[evaltext.CanonicalNumber(want)] {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		return CheckResult{Name: "required_numbers", Pass: false,
			Detail: "missing required number(s): " + strings.Join(missing, ", ")}
	}
	return CheckResult{Name: "required_numbers", Pass: true}
}
