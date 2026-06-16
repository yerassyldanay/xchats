package domain

import (
	"fmt"
	"regexp"
	"strconv"
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

// Tariff is the single source of price numbers (Decision 8).
type Tariff struct {
	Key          string
	PriceTenge   int64
	CashierLimit int64
}

// Placeholder is a non-tariff bilingual token (e.g. support.phone).
type Placeholder struct {
	Token   string
	ValueRU string
	ValueKK string
}

// PriceBook is the single source of numbers and the only thing that renders tokens.
type PriceBook struct {
	Tariffs      map[string]Tariff
	Placeholders map[string]Placeholder
}

// Snapshot is the immutable content the brain reasons from, loaded from the
// content source and hot-swapped on publish (docs/03 · snapshot).
type Snapshot struct {
	Config AssistantConfig
	Prices PriceBook
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

// Render replaces every {{namespace.key}} in text for lang. It errors if any
// token is unknown or any '{{' remains — the caller must not ship a half-rendered
// price (Decision 8; docs/03 · pricing & tokens).
//
//	price.<tariff> → formatted tenge (e.g. "19 900 ₸")
//	limit.<tariff> → cashier limit (e.g. "5")
//	<other>        → placeholders[token] value in lang
func (p *PriceBook) Render(text, lang string) (string, error) {
	var firstErr error
	out := tokenRE.ReplaceAllStringFunc(text, func(m string) string {
		sub := tokenRE.FindStringSubmatch(m)
		ns, key := sub[1], sub[2]
		switch ns {
		case "price":
			t, ok := p.Tariffs[key]
			if !ok {
				firstErr = orErr(firstErr, fmt.Errorf("unknown price token: %s.%s", ns, key))
				return m
			}
			return formatTenge(t.PriceTenge)
		case "limit":
			t, ok := p.Tariffs[key]
			if !ok {
				firstErr = orErr(firstErr, fmt.Errorf("unknown limit token: %s.%s", ns, key))
				return m
			}
			return strconv.FormatInt(t.CashierLimit, 10)
		default:
			ph, ok := p.Placeholders[ns+"."+key]
			if !ok {
				firstErr = orErr(firstErr, fmt.Errorf("unknown placeholder token: %s.%s", ns, key))
				return m
			}
			if lang == "kk" {
				return ph.ValueKK
			}
			return ph.ValueRU
		}
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

// formatTenge renders integer tenge with a space thousands separator and ₸,
// e.g. 19900 → "19 900 ₸".
func formatTenge(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatInt(v, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	res := b.String() + " ₸"
	if neg {
		res = "-" + res
	}
	return res
}
