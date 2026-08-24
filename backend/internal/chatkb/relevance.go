package chatkb

import (
	"sort"
	"strings"
	"unicode"
)

// Lexical relevance — which KB records a question is actually about.
//
// This is deliberately NOT the retrieval seam (Service is, see the package
// doc). Retrieval decides what the model gets to read; this decides what the
// UI draws a card for. Keeping them apart is what lets v1 hand the model the
// whole KB while still showing exactly the two products a question named.
//
// The matching is substring-based on lowercased, folded tokens because the
// KB is routinely Russian or Kazakh: stemming rules for those are a research
// project, but "витамин" matching "Витамин D" as a prefix is not.

// minToken is the shortest query token worth matching on. Below this,
// substring matching stops discriminating — a two-letter token appears
// somewhere in almost every record.
const minToken = 3

// scoreWeights: a hit in what the record IS (its title or natural key)
// outranks a hit somewhere in its prose, so asking about "delivery" surfaces
// the delivery-zone records rather than every product whose description
// happens to mention delivery.
const (
	weightTitle = 4
	weightKey   = 3
	weightField = 1
)

// tokenize splits a query into lowercase, letter/digit-only tokens of at
// least minToken runes. Unicode-aware throughout: the queries this sees are
// as likely to be Cyrillic as Latin.
func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len([]rune(f)) >= minToken {
			out = append(out, f)
		}
	}
	return out
}

// score rates one record against a set of query tokens. Zero means the
// question said nothing that points at this record. A token counts at most
// once per record, at the strongest place it was found.
func score(rec Record, tokens []string) int {
	if len(tokens) == 0 {
		return 0
	}
	title := strings.ToLower(rec.Title)
	key := strings.ToLower(rec.Key)
	// Field LABELS are searched alongside values, which is what makes
	// "what is the warranty?" find the policies record: "warranty" is the
	// name of a field there, never the content of one.
	var body strings.Builder
	for _, f := range rec.Fields {
		body.WriteString(strings.ToLower(f.Label))
		body.WriteByte('\n')
		body.WriteString(strings.ToLower(f.Value))
		body.WriteByte('\n')
	}
	values := body.String()

	total := 0
	for _, tok := range tokens {
		switch {
		case strings.Contains(title, tok):
			total += weightTitle
		case strings.Contains(key, tok):
			total += weightKey
		case strings.Contains(values, tok):
			total += weightField
		}
	}
	return total
}

// identityScore is score restricted to the record's identity — see
// rankIdentity for why that is a different question from relevance.
func identityScore(rec Record, tokens []string) int {
	title := strings.ToLower(rec.Title)
	key := strings.ToLower(rec.Key)
	total := 0
	for _, tok := range tokens {
		switch {
		case strings.Contains(title, tok):
			total += weightTitle
		case strings.Contains(key, tok):
			total += weightKey
		}
	}
	return total
}

// scored pairs a record with its relevance, keeping the record's original
// position so ties resolve deterministically instead of by map order.
type scored struct {
	record   Record
	score    int
	position int
}

// rank returns the records a query is RELEVANT to, best first, dropping
// everything that scored zero. Matches anywhere in the record, labels and
// values included, so "what are your prices?" returns the priced records.
// limit <= 0 means no cap.
func rank(records []Record, query string, limit int) []Record {
	return rankBy(records, query, limit, score)
}

// rankIdentity returns the records a query NAMES — matched only against what
// each record is (its title and natural key), never its contents.
//
// The distinction from rank matters: "the draft price of Vitamin D" is
// relevant to every priced product, but it names exactly one, and it is the
// named one that deserves a comparison card. Ranking identity by relevance
// instead would pull every product with a price into the answer.
func rankIdentity(records []Record, query string, limit int) []Record {
	return rankBy(records, query, limit, identityScore)
}

func rankBy(records []Record, query string, limit int, scoreFn func(Record, []string) int) []Record {
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil
	}
	hits := make([]scored, 0, len(records))
	for i, rec := range records {
		if s := scoreFn(rec, tokens); s > 0 {
			hits = append(hits, scored{record: rec, score: s, position: i})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].score != hits[b].score {
			return hits[a].score > hits[b].score
		}
		return hits[a].position < hits[b].position
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Record, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.record)
	}
	return out
}

// draftIntentWords are the words that make a question about the draft state
// itself ("what's pending?", "что изменилось в черновике?") rather than
// about some particular entity. Matched as substrings against the whole
// lowercased query, which is what makes the Russian entries work across
// cases and endings ("черновике", "изменения", "сравни").
var draftIntentWords = []string{
	// English
	"draft", "pending", "compare", "comparison", "difference", "differ", "changed", "changes", "staged", "unapproved",
	// Russian
	"черновик", "ожидающ", "сравн", "различ", "разниц", "изменен", "изменил", "изменит", "правк", "неутвержд",
	// Kazakh
	"жоба", "салыстыр", "өзгер",
}

// hasDraftIntent reports whether the question is about the real-vs-draft
// relationship at all. It is what decides between "show the two products you
// named" and "show everything that is pending" when a question names no
// entity — and, when a question names one, whether its card is a comparison
// or a plain item.
func hasDraftIntent(query string) bool {
	q := strings.ToLower(query)
	for _, w := range draftIntentWords {
		if strings.Contains(q, w) {
			return true
		}
	}
	return false
}
