package stt

import (
	"strings"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
)

// maxPromptRunes caps the assembled vocabulary-priming prompt. Whisper-family
// models only attend to roughly the last 224 tokens of this field regardless
// of how much is sent — capping in runes (not tokens, which would need a
// tokenizer this package has no reason to depend on) keeps the request body
// itself bounded without pretending to hit an exact token count.
const maxPromptRunes = 900

// BuildPrompt assembles the STT "prompt" bias string that primes a
// transcription call toward domain jargon it would otherwise mis-hear:
// operator-authored custom vocabulary first (a deliberate, hand-curated
// list — the least expendable if the result has to be truncated), then the
// organization's live catalog (active product/tariff names and category
// titles; a discontinued listing's name is no longer something a customer
// is likely to say). Terms are deduplicated case-insensitively and the whole
// string is capped to maxPromptRunes. kb may be nil (no knowledge base
// loaded yet, or the caller couldn't resolve one) — BuildPrompt then falls
// back to customVocabulary alone rather than failing.
//
// Only SalesStatus=="active" listings are read, mirroring aiprompt's own
// productVisible/active() gate — but not availability_status: a preorder or
// on_demand product is exactly as likely to come up by name in a voice note
// as an in-stock one, unlike the customer-facing prompt this isn't a
// grounding surface a AvailabilityStatus distinction needs to police.
func BuildPrompt(kb *aiprompt.KB, customVocabulary string) string {
	seen := make(map[string]bool)
	var terms []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if seen[key] {
			return
		}
		seen[key] = true
		terms = append(terms, s)
	}

	for _, w := range strings.FieldsFunc(customVocabulary, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	}) {
		add(w)
	}
	if kb != nil {
		for _, p := range kb.Products {
			if p.SalesStatus != "active" {
				continue
			}
			add(p.Name)
		}
		for _, tf := range kb.Tariffs {
			if tf.SalesStatus != "active" {
				continue
			}
			add(tf.Name)
		}
		for _, p := range kb.Products {
			if p.SalesStatus != "active" {
				continue
			}
			add(p.Category)
			add(p.Brand)
		}
	}

	prompt := strings.Join(terms, ", ")
	runes := []rune(prompt)
	if len(runes) > maxPromptRunes {
		runes = runes[:maxPromptRunes]
	}
	return string(runes)
}
