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

// shortMessageCyrillicThreshold: a live message with this few Cyrillic letters IN TOTAL
// (summed across the whole message, not just one clause) is too thin a sample to trust
// on its own for language routing — even a clause that happens to cross
// minRussianClauseRunes can be an accident of a single short word. Motivating example:
// "Бар ма?" (the Kazakh "is it available?" question-particle pair) has zero Kazakh-only
// letters, so nothing marks it Kazakh, but its 5 Cyrillic letters read as ONE clause and
// already cross minRussianClauseRunes(4) on raw letter count alone — the exact kind of
// accidental-confidence detectLang's per-clause rule can't tell apart from a real short
// Russian word. No purely orthographic signal can cleanly separate "Бар ма" from a short
// Russian reply, since both live entirely in the shared alphabet — "Рахмет" (Kazakh
// "thank you", 6 letters, also zero Kazakh-only letters) is just as ambiguous. Below this
// threshold, prefer what recent customer history already established (see
// detectLangFromHistory) over a shaky single-message read; deliberately set wide (8, not
// a razor-thin 5) since a generous threshold is low-risk — it only ever changes the
// outcome when history's most recent client turn gives an unambiguous verdict, and with
// no history (the common case: greetings open a conversation) the result is unchanged.
//
// KNOWN, ACCEPTED COST (raised independently by several reviewers — worth being explicit
// about, not just implicit in the reasoning above): this threshold does not distinguish
// "ambiguous short word" from "unambiguous but short Russian word." A single, ordinary
// Russian reply like "Скажите." (7 letters — minRussianClauseRunes's OWN doc comment
// above calls this one out as unambiguous on a Cyrillic-letter-count basis), "Привет."
// (6), or "Спасибо." (7) falls at or under this threshold exactly like "Бар ма" (5) or
// "Рахмет" (6) does, so it CAN be overridden by a disagreeing prior Kazakh customer turn
// even though a human would never read it as anything but confidently Russian. There is
// no single letter-count threshold that fixes this: "Привет"(6) and "Рахмет"(6) have the
// identical length despite one being unambiguous and the other genuinely ambiguous, so
// no number here can separate them by length alone. Accepted deliberately (see the
// "low-risk" reasoning above — the failure mode requires BOTH a short reply AND
// disagreeing prior history, and even then falls back to the SAME "ru" default a
// message-only classifier would have produced whenever history offers no verdict) rather
// than chased with a narrower threshold that would just as certainly miss "Бар ма" or
// "Рахмет" instead. Calibrated against worked examples only, same caveat as
// minRussianClauseRunes: needs native-speaker review before a billed comparison, not
// proof of correctness.
const shortMessageCyrillicThreshold = 8

// scanClauses is detectLang's core per-clause read, factored out so it can be applied
// identically to the live message and, via detectLangFromHistory, to a prior customer
// turn. See detectLang's doc comment for the clause-splitting and letter-counting rules.
func scanClauses(message string) (hasKazakhClause, hasRussianClause bool) {
	clauses := strings.FieldsFunc(message, func(r rune) bool {
		switch r {
		case '!', '?', ',', '.', ';', '\n':
			return true
		default:
			return false
		}
	})

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
	return hasKazakhClause, hasRussianClause
}

// cyrillicLetterCount counts every Cyrillic letter in the WHOLE message, ignoring clause
// boundaries — the message-level "is this even long enough to trust" signal
// shortMessageCyrillicThreshold is calibrated against, distinct from scanClauses' own
// per-clause minRussianClauseRunes check.
func cyrillicLetterCount(message string) int {
	n := 0
	for _, r := range message {
		if unicode.Is(unicode.Cyrillic, r) {
			n++
		}
	}
	return n
}

// detectLangFromHistory looks at ONLY the single most recent role=="client" turn in
// history (not a walk back through several turns): a walk needs "stop on ANY unambiguous
// verdict, not just kk" to avoid an older turn overriding a more recent code-switch
// mid-conversation — real subtlety this eval harness's actual motivating case (a short,
// ambiguous message immediately following one Kazakh turn) doesn't need to take on.
// Returns ok=false when there's no client turn in history, or that one turn is itself
// too ambiguous to read either way (scanClauses finds neither signal) — the caller must
// fall through to its own default in that case, never invent a verdict from silence.
func detectLangFromHistory(history []HistoryTurn) (lang string, ok bool) {
	for i := len(history) - 1; i >= 0; i-- {
		turn := history[i]
		if turn.Role != "client" {
			continue
		}
		hasKazakhClause, hasRussianClause := scanClauses(turn.Text)
		switch {
		case hasKazakhClause && hasRussianClause:
			return "ru", true // that turn was itself genuinely mixed
		case hasKazakhClause:
			return "kk", true
		case hasRussianClause:
			return "ru", true
		default:
			return "", false // the most recent client turn is ALSO ambiguous
		}
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
// ("mixed language -> reply in Russian"). Otherwise, if the message has a Kazakh-reading
// clause (and no competing Russian one), it routes "kk" unconditionally — a Kazakh-only
// letter is diagnostic regardless of message length, so history is never consulted here.
//
// Every OTHER case (no Kazakh-reading clause at all — the common all-Russian message, a
// mixed-but-short message, or pure Latin/digits/empty) would, on the message alone,
// default to "ru". Before committing to that default, though: if the message's TOTAL
// Cyrillic letter count is at or below shortMessageCyrillicThreshold, the message is too
// short a sample to trust on its own — fall back to detectLangFromHistory and use
// whatever verdict it finds, if any (see shortMessageCyrillicThreshold's doc comment for
// why, and the Бар ма? example it's calibrated against). With no history, or history
// that's itself ambiguous, or a message too long to qualify for the fallback at all: "ru"
// — today's exact default, unchanged.
func detectLang(message string, history []HistoryTurn) string {
	hasKazakhClause, hasRussianClause := scanClauses(message)
	if hasKazakhClause {
		if hasRussianClause {
			return "ru" // genuinely mixed -> existing policy: reply in Russian
		}
		return "kk"
	}

	if n := cyrillicLetterCount(message); n > 0 && n <= shortMessageCyrillicThreshold {
		if lang, ok := detectLangFromHistory(history); ok {
			return lang
		}
	}
	return "ru"
}
