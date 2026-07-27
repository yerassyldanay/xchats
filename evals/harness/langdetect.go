package main

import (
	"strings"
	"unicode"

	"github.com/pemistahl/lingua-go"
)

// kazakhRussianDetector is a statistical (n-gram) language classifier restricted to just
// Kazakh and Russian, replacing the earlier hand-rolled Kazakh-letter/word-list heuristic
// (2026-07-27 decision: recognize Kazakh by words and grammar, not by the presence of
// Kazakh-specific letters — a customer routinely types Kazakh using only the Russian
// keyboard, e.g. "Сиздерде Филипс блендери бар ма? Билейин деген едим.", which has ZERO
// Kazakh-only letters and was invisible to the old letter-counting approach). Restricting
// FromLanguages to just these two (instead of all 75 lingua-go supports) makes every
// confidence value relative between exactly the two languages this harness ever routes
// between. Built once at package load — Build() is a cheap, offline, in-process operation
// (no network calls, no external service; empirically ~40µs for a 2-language detector) —
// and reused for every detectLang call in the process.
var kazakhRussianDetector = lingua.NewLanguageDetectorBuilder().
	FromLanguages(lingua.Kazakh, lingua.Russian).
	WithPreloadedLanguageModels().
	Build()

// shortMessageCyrillicThreshold: a message with this few Cyrillic letters IN TOTAL is too
// thin a sample for even a statistical model to trust — lingua-go is calibrated for
// natural-language text, and on 1-3 word fragments it can be confidently WRONG (observed:
// the bare Russian word "Да" — 2 letters, unambiguously "yes" in Russian — is classified
// Kazakh with 0.7+ confidence in isolation, because Cyrillic n-gram statistics on that few
// letters don't carry enough signal). Below this threshold, prefer what recent customer
// history already established (see detectLangFromHistory) over a shaky single-message
// read. Deliberately set wide (8, not a razor-thin 3-4) since a generous threshold is
// low-risk: it only changes the outcome when history's most recent client turn gives an
// unambiguous verdict, and with no history (the common case: greetings open a
// conversation) the result is unchanged. Calibrated against the harness's own test banks
// plus the 2026-07-27 worked examples — needs native-speaker review before a billed
// comparison, not proof of correctness.
const shortMessageCyrillicThreshold = 8

// cyrillicLetterCount counts every Cyrillic letter in the WHOLE message — the
// message-level "is this even long enough to trust a statistical read" signal
// shortMessageCyrillicThreshold is calibrated against.
func cyrillicLetterCount(message string) int {
	n := 0
	for _, r := range message {
		if unicode.Is(unicode.Cyrillic, r) {
			n++
		}
	}
	return n
}

// classifyText returns lingua-go's best read of a single piece of text ("kk" or "ru"),
// or ok=false when the text is too short to trust (see shortMessageCyrillicThreshold) or
// lingua-go finds no Kazakh/Russian content at all (pure Latin/digits/emoji, or a
// genuinely unclassifiable fragment).
//
// DetectMultipleLanguagesOf, not the simpler whole-string ComputeLanguageConfidenceValues,
// is the right primitive here: a genuinely mixed message ("Здравствуйте! Астанага жеткизу
// канша турады?" — a Russian greeting followed by a Kazakh question typed in Russian-only
// letters) reads as 0.673 Russian / 0.327 Kazakh as ONE whole-string confidence value —
// the Russian greeting's statistical weight drowns out the actual (Kazakh) question, and
// 0.673 clears most any reasonable "confident" threshold, so a whole-string approach would
// silently answer the wrong language. DetectMultipleLanguagesOf instead segments the text
// into contiguous single-language spans ("Здравствуйте! Астанага жеткизу " → Russian,
// "канша турады?" → Kazakh) — the SAME dominant-clause idea the old hand-rolled version
// used (reply in the language the question itself is asked in), just with each span
// classified by the statistical model instead of by counting Kazakh-only letters. Verified
// (2026-07-27) against every message in this repo's Kazakh/Russian test banks plus the
// mixed/shared-alphabet worked examples from the user's design note — see
// langdetect_test.go.
func classifyText(text string) (string, bool) {
	if cyrillicLetterCount(text) <= shortMessageCyrillicThreshold {
		return "", false
	}

	type span struct {
		lang       lingua.Language
		start, end int
	}
	var spans []span
	for _, r := range kazakhRussianDetector.DetectMultipleLanguagesOf(text) {
		if r.Language() == lingua.Unknown {
			continue // pure Latin/digit/emoji sections carry no kk-vs-ru signal
		}
		spans = append(spans, span{lang: r.Language(), start: r.StartIndex(), end: r.EndIndex()})
	}
	if len(spans) == 0 {
		return "", false
	}

	// Dominant-language rule (mirrors the frame's own drafting rule, "reply in the
	// language the question itself is asked in"): if the message ends its LAST clause in
	// '?', the span containing that '?' wins — the final question is what the reply will
	// actually answer. Otherwise the last span wins (a statement's own trailing clause,
	// or — for a single-span message, the overwhelmingly common case — that one span).
	dominant := spans[len(spans)-1]
	if lastQ := strings.LastIndex(text, "?"); lastQ >= 0 {
		for _, s := range spans {
			if lastQ >= s.start && lastQ < s.end {
				dominant = s
				break
			}
		}
	}
	if dominant.lang == lingua.Kazakh {
		return "kk", true
	}
	return "ru", true
}

// detectLangFromHistory looks at ONLY the single most recent role=="client" turn in
// history (not a walk back through several turns): a walk needs "stop on ANY unambiguous
// verdict, not just kk" to avoid an older turn overriding a more recent code-switch
// mid-conversation — real subtlety this eval harness's actual motivating case (a short,
// ambiguous message immediately following one Kazakh turn) doesn't need to take on.
// Returns ok=false when there's no client turn in history, or that one turn is itself too
// short/ambiguous to read either way (classifyText finds no usable signal) — the caller
// must fall through to its own default in that case, never invent a verdict from silence.
func detectLangFromHistory(history []HistoryTurn) (lang string, ok bool) {
	for i := len(history) - 1; i >= 0; i-- {
		turn := history[i]
		if turn.Role != "client" {
			continue
		}
		if l, ok := classifyText(turn.Text); ok {
			return l, true
		}
		return "", false // the most recent client turn is ALSO too short/ambiguous
	}
	return "", false
}

// detectLang is the deterministic, pre-call router for the language-routed frame variant
// (V4): given the customer's latest message (and, when the message alone isn't enough,
// recent conversation history), decide which system prompt to send — "kk" (a fully
// Kazakh instruction frame) or "ru" (the existing Russian frame). It runs BEFORE any
// model call, so — unlike judge.go's looksKazakh, which grades a reply already written —
// it has no model output to lean on.
//
// A message long enough to trust (see shortMessageCyrillicThreshold) is classified
// directly by lingua-go (classifyText) and that verdict is final — no history consulted,
// even a disagreeing one. A message with SOME Cyrillic content but too little to trust on
// its own falls back to the most recent client history turn's own (equally length-gated)
// verdict, if any. A message with NO Cyrillic content at all (pure Latin/digits/emoji, or
// empty) skips history entirely — that's a different case ("nothing to route on"), not "a
// short Cyrillic message" — and goes straight to the default. Every remaining case
// defaults to "ru".
func detectLang(message string, history []HistoryTurn) string {
	if lang, ok := classifyText(message); ok {
		return lang
	}
	if cyrillicLetterCount(message) > 0 {
		if lang, ok := detectLangFromHistory(history); ok {
			return lang
		}
	}
	return "ru"
}
