// Package evaltext holds string/number normalization used by the eval harness's
// grading code (judge.go and extract_checks.go, both in package main). It is a leaf
// package on purpose: no dependency on eval-specific types (Verdict, ExtractCase,
// etc.), so it can be imported without pulling in either family's logic.
package evaltext

import (
	"regexp"
	"strings"
)

// NumberRunRE matches a run of digits that may contain internal spaces, commas, or
// hyphens as grouping separators (e.g. "25 000" — a source image's own Russian-style
// thousands grouping; "25,000" — observed in practice: a model's prose `summary`
// re-formatting the exact same value English-style even though its own `extracted_text`
// correctly kept the source's spacing; "+7702-976-65-09" — a phone number re-grouped
// with hyphens throughout instead of the source's space-then-hyphens mix). All of these
// must be treated as ONE number, not split into fragments that then look "invented"
// against an allow-list. Unlike judge.go's own digit-run matching (\d+), which
// intentionally splits on any non-digit (correct for ITS job: digits inside an
// already-injected customer reply, no grouping left once a token is substituted) —
// that regex stays in judge.go, it is not part of this shared package.
//
// Known edge case: this also merges a bare, separator-joined ENUMERATION of distinct
// numbers with nothing else between them (e.g. "10, 25, 60 тысяч" or "10-25-60") into
// one run. If that ever needs distinguishing, the merged run simply won't match any
// single allow-list entry and the check fails closed (flagged for review) rather than
// silently passing something wrong.
var NumberRunRE = regexp.MustCompile(`\d(?:[\d, \-]*\d)?`)

// CanonicalNumber strips grouping separators (space, NBSP, comma, hyphen) so "25 000",
// "25,000", "25-000" and "25000" all compare equal — the check cares about the VALUE
// shown, not which grouping convention a particular field happened to use.
func CanonicalNumber(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, " ", "") // NBSP
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

// FoldNBSP folds NBSP to a normal space and nothing else — for a haystack where a real
// line break must survive as a hard boundary. Unlike NormalizeText, this does NOT
// lowercase or collapse newlines/multi-space runs.
func FoldNBSP(s string) string {
	return strings.ReplaceAll(s, " ", " ")
}

// NormalizeText lowercases, folds NBSP to a normal space, folds ё->е (a genuine,
// common Russian orthographic equivalence — printed text and casual writing routinely
// drop the diaeresis, e.g. "Петр"/"Пётр" are both the same, correct name; this is not
// an error to penalize), and collapses whitespace runs (including newlines) to a
// single space, so a check like "contains 10 000" matches regardless of the model's
// exact spacing/line-wrapping.
func NormalizeText(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", " ") // NBSP -> normal space
	s = strings.ReplaceAll(s, "ё", "е")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

var (
	fenceOpenRE  = regexp.MustCompile("^\\s*```[a-zA-Z]*\\s*")
	fenceCloseRE = regexp.MustCompile("\\s*```\\s*$")
)

// StripFences removes a leading/trailing markdown code fence (```json ... ```) from a
// raw model answer, tolerating one even when the model was explicitly asked for
// response_format: json_object — some models wrap fenced code anyway. Used by both
// judge.go's parseModelJSON and extract_checks.go's parseExtractionJSON.
func StripFences(raw string) string {
	cleaned := fenceOpenRE.ReplaceAllString(raw, "")
	cleaned = fenceCloseRE.ReplaceAllString(cleaned, "")
	return cleaned
}

// ReasoningMarkerRE matches the documented `<think>`/`<thinking>` tag pair some models
// emit in-band inside their own content string — a real, observed OpenRouter failure
// mode across providers, independent of whether reasoning was even requested for that
// call. Deliberately narrow: this is NOT a natural-language "did the model reason out
// loud" heuristic (which would false-positive on any legitimate customer-facing text that
// happens to contain the word "think") — only the specific tag markers.
var ReasoningMarkerRE = regexp.MustCompile(`(?i)</?think(ing)?>`)

// HasReasoningMarkers reports whether s contains a reasoning/thinking tag marker — used
// both as a hard gate (judge.go's ReasoningLeak, extract_checks.go's analogous check,
// scanning the model's customer-facing text fields) and as a visibility flag on raw,
// unfiltered debug output (viewmodel.go's VOutput.RawHasReasoningMarkers).
func HasReasoningMarkers(s string) bool {
	return ReasoningMarkerRE.MatchString(s)
}
