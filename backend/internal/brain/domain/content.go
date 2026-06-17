package domain

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// AssistantConfig is the published "soul" — persona + guardrails (docs/03 DDL).
type AssistantConfig struct {
	Version        int
	Persona        string
	Mission        string
	Guardrails     string
	LanguagePolicy string
	ReplyMaxWords  int
}

// Topic is one kb_topics row (one language). Bodies hold price TOKENS, never numerals.
type Topic struct {
	Slug     string
	Language string
	Title    string
	Summary  string
	BodyMD   string
	Keywords string // comma-separated; shown to the model to aid topic/media selection
}

// Asset is one kb_assets row — the LLM-facing menu entry the model selects on.
type Asset struct {
	Ref         string
	TopicSlug   string
	Kind        string
	URL         string
	Title       string
	Description string
	Language    string
}

// Value is one ai_values row: a confirmed (token, lang) → text fact. It is the
// single source of every number/contact/limit the assistant may state — prices,
// SLAs, addresses, counts — and the only thing token rendering substitutes. The
// text is free-form precisely so any unit fits ("25 000 ₸/мес", "до 2 000
// платежей/мес", "5 минут"): the renderer substitutes it verbatim, never
// reformatting it (the old typed Tariff/formatTenge path silently dropped units).
type Value struct {
	Token       string // namespace.key — e.g. 'price.growth', 'contact.whatsapp'
	Lang        string // 'ru' | 'kk' | '*'  ('*' = language-neutral)
	Text        string // the confirmed value, substituted verbatim
	Description string // human-only (editor / confirm popup); never injected into the prompt
}

// ValueBook is the snapshot's value store: a (token, lang) → text lookup with a
// '*' language-neutral fallback. It replaces the old PriceBook's typed Tariff
// map, so unit-bearing values round-trip losslessly.
type ValueBook struct {
	list  []Value
	index map[string]map[string]string // token -> lang -> text
}

// NewValueBook builds a ValueBook from rows (seed literal, DB load, or tests).
func NewValueBook(values ...Value) ValueBook {
	b := ValueBook{index: map[string]map[string]string{}}
	for _, v := range values {
		b.Add(v)
	}
	return b
}

// Add records one value row (last write wins per token+lang).
func (b *ValueBook) Add(v Value) {
	if b.index == nil {
		b.index = map[string]map[string]string{}
	}
	if b.index[v.Token] == nil {
		b.index[v.Token] = map[string]string{}
	}
	b.index[v.Token][v.Lang] = v.Text
	b.list = append(b.list, v)
}

// List returns the value rows in insertion order (for the editor/kbstore).
func (b ValueBook) List() []Value { return b.list }

// lookup resolves a token for a language, falling back to the '*' row.
func (b ValueBook) lookup(token, lang string) (string, bool) {
	langs := b.index[token]
	if langs == nil {
		return "", false
	}
	if t, ok := langs[lang]; ok {
		return t, true
	}
	if t, ok := langs["*"]; ok {
		return t, true
	}
	return "", false
}

// Snapshot is the immutable content the brain reasons from, loaded from the
// content source and hot-swapped on publish (docs/03 · snapshot).
type Snapshot struct {
	Config AssistantConfig
	Values ValueBook
	Topics []Topic
	Assets []Asset
	Loaded time.Time
}

// Content holds the live snapshot behind an atomic pointer (the ContentSource port).
type Content struct {
	snap atomic.Pointer[Snapshot]
}

// Get returns the current snapshot (may be nil before the first load).
func (c *Content) Get() *Snapshot { return c.snap.Load() }

// Set atomically swaps the live snapshot.
func (c *Content) Set(s *Snapshot) { c.snap.Store(s) }

// tokenRE matches {{namespace.key}} with simple identifier segments.
var tokenRE = regexp.MustCompile(`\{\{\s*([a-zA-Z_]+)\.([a-zA-Z0-9_]+)\s*\}\}`)

// Render replaces every {{namespace.key}} in text with its confirmed value for
// lang (falling back to the '*' language-neutral row). It errors if any token is
// unresolved or any '{{' remains — the caller must not ship a half-rendered value
// (Decision 8; docs/03 · pricing & tokens). The value is substituted verbatim, so
// units carried in the value text survive intact.
func (b ValueBook) Render(text, lang string) (string, error) {
	var firstErr error
	out := tokenRE.ReplaceAllStringFunc(text, func(m string) string {
		sub := tokenRE.FindStringSubmatch(m)
		token := sub[1] + "." + sub[2]
		if v, ok := b.lookup(token, lang); ok {
			return v
		}
		firstErr = orErr(firstErr, fmt.Errorf("unresolved value token: %s", token))
		return m
	})
	if firstErr != nil {
		return "", firstErr
	}
	// No token may remain — a leftover '{{' means a malformed/partial token.
	if strings.Contains(out, "{{") {
		return "", fmt.Errorf("leftover token in rendered text")
	}
	return out, nil
}

func orErr(existing, e error) error {
	if existing != nil {
		return existing
	}
	return e
}
