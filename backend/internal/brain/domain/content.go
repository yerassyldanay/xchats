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

// PolicySlug is the singleton slug for the org commerce-policy entity, so every
// policy token has the 3-part shape {{policy.main.<field>}}.
const PolicySlug = "main"

// Tariff is one ai_tariffs row (one language): a pricing plan whose exact numbers
// (price / limit_text / fee) are verbatim typed columns.
type Tariff struct {
	Ref, Lang, Name, Price, LimitText, Fee, Summary, PricingType, Advantages, Disadvantages string
}

// Product is one ai_products row (one language): a sellable item with a verbatim price.
type Product struct {
	Ref, Lang, Name, Price, Description, Category, Availability string
}

// Contact is one ai_contacts row (one language): the org's support scalars.
type Contact struct {
	Lang, WhatsApp, Email, Address, Legal, CallbackTime string
	WorkingHours, Phone, Website, Instagram             string
}

// Policy is one ai_policies row (one language): org commerce-policy scalars
// (delivery/payment/returns terms) — a structural clone of Contact, quoted as
// {{policy.main.<field>}}.
type Policy struct {
	Lang, DeliveryCost, DeliveryTime, FreeDeliveryFrom, MinOrder string
	Prepayment, Installment, ReturnPeriod, Warranty              string
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
	productFieldLabel = map[string]string{"price": "цена", "availability": "наличие"}
	contactFieldLabel = map[string]string{
		"whatsapp": "WhatsApp", "email": "e-mail", "address": "адрес",
		"legal": "реквизиты", "callback_time": "время обратного звонка",
		"working_hours": "график работы", "phone": "телефон", "website": "сайт", "instagram": "Instagram",
	}
	policyFieldLabel = map[string]string{
		"delivery_cost": "стоимость доставки", "delivery_time": "срок доставки",
		"free_delivery_from": "бесплатная доставка от", "min_order": "минимальный заказ",
		"prepayment": "предоплата", "installment": "рассрочка",
		"return_period": "срок возврата", "warranty": "гарантия",
	}
)

// NewFactBook indexes every column of the typed fact rows and builds the [F]
// catalog entries (numeric/contact/policy fields only — names stay prose in bodies).
func NewFactBook(tariffs []Tariff, products []Product, contacts []Contact, policies []Policy) FactBook {
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
		put("product", p.Ref, "availability", p.Lang, p.Availability)
	}
	for _, c := range contacts {
		put("contact", ContactSlug, "whatsapp", c.Lang, c.WhatsApp)
		put("contact", ContactSlug, "email", c.Lang, c.Email)
		put("contact", ContactSlug, "address", c.Lang, c.Address)
		put("contact", ContactSlug, "legal", c.Lang, c.Legal)
		put("contact", ContactSlug, "callback_time", c.Lang, c.CallbackTime)
		put("contact", ContactSlug, "working_hours", c.Lang, c.WorkingHours)
		put("contact", ContactSlug, "phone", c.Lang, c.Phone)
		put("contact", ContactSlug, "website", c.Lang, c.Website)
		put("contact", ContactSlug, "instagram", c.Lang, c.Instagram)
	}
	for _, p := range policies {
		put("policy", PolicySlug, "delivery_cost", p.Lang, p.DeliveryCost)
		put("policy", PolicySlug, "delivery_time", p.Lang, p.DeliveryTime)
		put("policy", PolicySlug, "free_delivery_from", p.Lang, p.FreeDeliveryFrom)
		put("policy", PolicySlug, "min_order", p.Lang, p.MinOrder)
		put("policy", PolicySlug, "prepayment", p.Lang, p.Prepayment)
		put("policy", PolicySlug, "installment", p.Lang, p.Installment)
		put("policy", PolicySlug, "return_period", p.Lang, p.ReturnPeriod)
		put("policy", PolicySlug, "warranty", p.Lang, p.Warranty)
	}

	// [F] entries in a stable order (tariffs → products → contacts → policies),
	// one per numeric/contact/policy token. Names are excluded — the model
	// writes them as prose.
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
		name := p.Name
		for _, f := range []string{"price", "availability"} {
			entry("product", p.Ref, f, fmt.Sprintf("Товар «%s» — %s", name, productFieldLabel[f]))
		}
	}
	seen = map[string]bool{}
	for _, c := range contacts {
		_ = c
		if !once("contact") {
			continue
		}
		for _, f := range []string{"whatsapp", "email", "address", "legal", "callback_time",
			"working_hours", "phone", "website", "instagram"} {
			entry("contact", ContactSlug, f, "Контакты поддержки — "+contactFieldLabel[f])
		}
	}
	seen = map[string]bool{}
	for _, p := range policies {
		_ = p
		if !once("policy") {
			continue
		}
		for _, f := range []string{"delivery_cost", "delivery_time", "free_delivery_from", "min_order",
			"prepayment", "installment", "return_period", "warranty"} {
			entry("policy", PolicySlug, f, "Условия — "+policyFieldLabel[f])
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
	Tariffs  []Tariff
	Products []Product
	Contacts []Contact
	Policies []Policy
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
// tariff|product|contact|policy, slug a ref (hyphens allowed), field a column name.
var tokenRE = regexp.MustCompile(`\{\{\s*(tariff|product|contact|policy)\.([a-zA-Z0-9_-]+)\.([a-z_]+)\s*\}\}`)

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
