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

// Topic is one ai_topics row (one language). Bodies are PURE PROSE — no digits,
// no fact tokens (14 Decision 3); the model combines this prose with the FACTS
// catalog when it drafts, quoting exact facts as {{table.slug.field}} tokens.
type Topic struct {
	Slug     string
	Language string
	Title    string
	Summary  string
	BodyMD   string
	Keywords string // comma-separated; shown to the model to aid topic/media selection
}

// Asset is one ai_assets row — the LLM-facing menu entry the model selects on.
type Asset struct {
	Ref         string
	TopicSlug   string
	Kind        string
	URL         string
	Title       string
	Description string
	Language    string
}

// --- Facts lane: typed entities, quoted as {{table.slug.field}} tokens --------
//
// Every exact fact (price, limit, fee, phone, e-mail, …) is a CONCRETE COLUMN on
// a typed entity row, never a generic key→text value. The model quotes a fact
// only as a 3-part token {{table.slug.field}} — table selects the fact table,
// slug the row (ref, or 'support' for the contact singleton), field the column —
// and code substitutes the stored column value verbatim (units included). This
// replaces the old ai_values bag (15 Decision 6).

// ContactSlug is the singleton slug for the org support-contact entity, so every
// contact token has the 3-part shape {{contact.support.<field>}}.
const ContactSlug = "support"

// Tariff is one ai_tariffs row (one language): a pricing plan whose exact numbers
// (price / limit_text / fee) are verbatim typed columns.
type Tariff struct {
	Ref, Lang, Name, Price, LimitText, Fee, Summary, PricingType, Advantages, Disadvantages string
}

// Product is one ai_products row (one language): a sellable item with a verbatim price.
type Product struct {
	Ref, Lang, Name, Price, Description, Category string
}

// Contact is one ai_contacts row (one language): the org's support scalars.
type Contact struct {
	Lang, WhatsApp, Email, Address, Legal, CallbackTime string
}

// Fact is one row of the prompt [F] catalog: the token the model must emit, a
// human label (its meaning — so the model picks the right fact), and the current
// value (shown so the model can choose; it still outputs the token, not the value).
type Fact struct {
	Token string
	Label string
	Value string
}

// FactBook resolves {{table.slug.field}} tokens to verbatim column values and
// lists the facts for the prompt [F] block. Built from the typed fact slices at
// snapshot load; the model never sees the raw numbers except as [F] hints.
type FactBook struct {
	index   map[string]string // table\x00slug\x00field\x00lang -> value
	entries []Fact            // for [F], one per numeric/contact token, insertion order
}

func factKey(table, slug, field, lang string) string {
	return table + "\x00" + slug + "\x00" + field + "\x00" + lang
}

// Field labels for the [F] catalog (the "meaning" column shown to the model).
var (
	tariffFieldLabel  = map[string]string{"price": "цена", "limit_text": "лимит", "fee": "комиссия"}
	productFieldLabel = map[string]string{"price": "цена"}
	contactFieldLabel = map[string]string{
		"whatsapp": "WhatsApp", "email": "e-mail", "address": "адрес",
		"legal": "реквизиты", "callback_time": "время обратного звонка",
	}
)

// NewFactBook indexes every column of the typed fact rows and builds the [F]
// catalog entries (numeric/contact fields only — names stay prose in bodies).
func NewFactBook(tariffs []Tariff, products []Product, contacts []Contact) FactBook {
	b := FactBook{index: map[string]string{}}
	put := func(table, slug, field, lang, value string) {
		if strings.TrimSpace(value) != "" {
			b.index[factKey(table, slug, field, lang)] = value
		}
	}
	entry := func(table, slug, field, label string) {
		v, ok := b.Resolve(table, slug, field, "ru")
		if !ok {
			return
		}
		b.entries = append(b.entries, Fact{
			Token: "{{" + table + "." + slug + "." + field + "}}",
			Label: label,
			Value: v,
		})
	}

	for _, t := range tariffs {
		put("tariff", t.Ref, "name", t.Lang, t.Name)
		put("tariff", t.Ref, "price", t.Lang, t.Price)
		put("tariff", t.Ref, "limit_text", t.Lang, t.LimitText)
		put("tariff", t.Ref, "fee", t.Lang, t.Fee)
	}
	for _, p := range products {
		put("product", p.Ref, "name", p.Lang, p.Name)
		put("product", p.Ref, "price", p.Lang, p.Price)
	}
	for _, c := range contacts {
		put("contact", ContactSlug, "whatsapp", c.Lang, c.WhatsApp)
		put("contact", ContactSlug, "email", c.Lang, c.Email)
		put("contact", ContactSlug, "address", c.Lang, c.Address)
		put("contact", ContactSlug, "legal", c.Lang, c.Legal)
		put("contact", ContactSlug, "callback_time", c.Lang, c.CallbackTime)
	}

	// [F] entries in a stable order (tariffs → products → contacts), one per
	// numeric/contact token. Names are excluded — the model writes them as prose.
	seen := map[string]bool{}
	once := func(ref string) bool {
		if seen[ref] {
			return false
		}
		seen[ref] = true
		return true
	}
	for _, t := range tariffs {
		if !once("tariff|"+t.Ref) {
			continue
		}
		name := t.Name
		for _, f := range []string{"price", "limit_text", "fee"} {
			entry("tariff", t.Ref, f, fmt.Sprintf("Тариф «%s» — %s", name, tariffFieldLabel[f]))
		}
	}
	seen = map[string]bool{}
	for _, p := range products {
		if !once("product|"+p.Ref) {
			continue
		}
		entry("product", p.Ref, "price", fmt.Sprintf("Товар «%s» — %s", p.Name, productFieldLabel["price"]))
	}
	seen = map[string]bool{}
	for _, c := range contacts {
		_ = c
		if !once("contact") {
			continue
		}
		for _, f := range []string{"whatsapp", "email", "address", "legal", "callback_time"} {
			entry("contact", ContactSlug, f, "Контакты поддержки — "+contactFieldLabel[f])
		}
	}
	return b
}

// Resolve returns the verbatim value for a fact token, trying the reply language,
// then the org default (ru), then the '*' language-neutral row.
func (b FactBook) Resolve(table, slug, field, lang string) (string, bool) {
	for _, l := range []string{lang, "ru", "*"} {
		if l == "" {
			continue
		}
		if v, ok := b.index[factKey(table, slug, field, l)]; ok {
			return v, true
		}
	}
	return "", false
}

// List returns the [F] catalog rows (token, label, value) in a stable order.
func (b FactBook) List() []Fact { return b.entries }

// Snapshot is the immutable content the brain reasons from, loaded from the live
// KB and hot-swapped on approve (docs/03 · snapshot).
type Snapshot struct {
	Config   AssistantConfig
	Topics   []Topic
	Assets   []Asset
	Tariffs  []Tariff
	Products []Product
	Contacts []Contact
	Facts    FactBook // derived from the typed slices for render + the [F] block
	Loaded   time.Time
}

// Content holds the live snapshot behind an atomic pointer (the ContentSource port).
type Content struct {
	snap atomic.Pointer[Snapshot]
}

// Get returns the current snapshot (may be nil before the first load).
func (c *Content) Get() *Snapshot { return c.snap.Load() }

// Set atomically swaps the live snapshot.
func (c *Content) Set(s *Snapshot) { c.snap.Store(s) }

// tokenRE matches a 3-part fact token {{table.slug.field}} — table ∈
// tariff|product|contact, slug a ref (hyphens allowed), field a column name.
var tokenRE = regexp.MustCompile(`\{\{\s*(tariff|product|contact)\.([a-zA-Z0-9_-]+)\.([a-z_]+)\s*\}\}`)

// Render replaces every {{table.slug.field}} in text with its stored column value
// for lang (fallback reply-lang → ru → '*'). It errors if any token is unresolved
// or any '{{' remains — the caller must not ship a half-rendered fact (fail closed;
// Decision 8). The value is substituted verbatim, so units survive intact.
func (b FactBook) Render(text, lang string) (string, error) {
	var firstErr error
	out := tokenRE.ReplaceAllStringFunc(text, func(m string) string {
		sub := tokenRE.FindStringSubmatch(m)
		table, slug, field := sub[1], sub[2], sub[3]
		if v, ok := b.Resolve(table, slug, field, lang); ok {
			return v
		}
		firstErr = orErr(firstErr, fmt.Errorf("unresolved fact token: %s.%s.%s", table, slug, field))
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
