package main

import (
	"strings"
	"unicode"
)

// kazakhOnlySpecificLetters are the Cyrillic letters that exist in the Kazakh alphabet but
// not the Russian one — mirrors judge.go's kazakhOnlyLetters exactly (kept as an
// INDEPENDENT copy on purpose: judge.go's heuristic grades a MODEL'S OUTPUT after the
// fact; detectLang below makes a PRE-call routing decision on the CUSTOMER'S message — a
// different job that happens to reuse the same letter set today. If either needs to
// diverge later — detectLang gaining more signal, or judge.go's grading getting stricter —
// this duplication means changing one on purpose, not drifting both via shared state).
const kazakhOnlySpecificLetters = "әғқңөұүһӘҒҚҢӨҰҮҺ"

// minRussianClauseRunes: a Cyrillic clause needs at least this many letters, with zero
// Kazakh-only letters, to count as its own competing "this part is Russian" signal — a
// short shared-alphabet interjection or particle below this length is assumed to be
// riding along inside a Kazakh clause, not an independent Russian one. Calibrated against
// the existing canary set: "бе" (2 letters, inside "Сәлеметсіз бе!") must NOT count on its
// own; "Скажите" (7 letters) must.
const minRussianClauseRunes = 4

// detectLang is the deterministic, pre-call router for the language-routed frame variant
// (V4): given the customer's latest message, decide which system prompt to send — "kk" (a
// fully Kazakh instruction frame) or "ru" (the existing Russian frame). It runs BEFORE any
// model call, so — unlike judge.go's looksKazakh, which grades a reply already written —
// it has no model output to lean on.
//
// Rule: split the message into clauses on ! ? , . ; and newlines. A clause "reads Kazakh"
// if it contains a Kazakh-only letter — this keeps a Latin brand name or a short
// shared-alphabet word riding inside an otherwise-Kazakh clause from flipping the verdict
// (e.g. "Кофемашина DeLonghi қанша тұрады?" is ONE clause; "DeLonghi" being Latin and
// "Кофемашина" sharing the Russian alphabet do not override "қанша"/"тұрады"). A clause
// "reads Russian" if it has at least minRussianClauseRunes Cyrillic letters and NONE of
// them are Kazakh-only.
//
// If the message has at least one Kazakh-reading clause AND at least one Russian-reading
// clause, it is genuinely mixed and routes "ru" — matching the frames' existing rule
// ("mixed language -> reply in Russian"). Zero Kazakh-reading clauses routes "ru" outright
// (the common case: an all-Russian message). Otherwise (Kazakh-reading clauses present, no
// competing Russian clause) routes "kk".
//
// Known limitation: a run-on message that mixes real Kazakh and Russian prose with NO
// separating punctuation at all is read as one clause and will route on whichever signal
// appears — this is a canary-experiment router, not a production-hardened classifier; see
// Phase 0.2 in the plan for why per-message-at-render-time is enough for the eval bake-off
// without needing a promptfoo-level dynamic routing mechanism.
func detectLang(message string) string {
	clauses := strings.FieldsFunc(message, func(r rune) bool {
		switch r {
		case '!', '?', ',', '.', ';', '\n':
			return true
		default:
			return false
		}
	})

	hasKazakhClause := false
	hasRussianClause := false
	for _, clause := range clauses {
		kazakhLetters := 0
		cyrillicLetters := 0
		for _, r := range clause {
			if !unicode.Is(unicode.Cyrillic, r) {
				continue
			}
			cyrillicLetters++
			if strings.ContainsRune(kazakhOnlySpecificLetters, r) {
				kazakhLetters++
			}
		}
		switch {
		case kazakhLetters > 0:
			hasKazakhClause = true
		case cyrillicLetters >= minRussianClauseRunes:
			hasRussianClause = true
		}
	}

	if !hasKazakhClause {
		return "ru"
	}
	if hasRussianClause {
		return "ru" // genuinely mixed -> existing policy: reply in Russian
	}
	return "kk"
}
