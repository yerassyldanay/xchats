package aiprompt

import (
	"fmt"
	"regexp"
	"strings"
)

// Prompt slots a frame may carry. The frame authors all human wording
// (headers, rules); the renderers emit content rows only. Slots present in a
// frame are always filled; a leftover %%…%% marker fails validation.
//
// SlotKnowledgeBase/SlotDescriptions/SlotFacts/SlotMedia/SlotMediaAbsent are the
// v1 (flat) rendering: one shared topic list, one shared prose list, one flat
// FACTS table (all tables, including products), one global media catalog, one
// global media-absence list. Kept for legacy frame compatibility.
//
// SlotProductsInStock/SlotProductsOutOfStock/SlotTopics/SlotBusinessFacts are the
// v2 (canonical-block) rendering (2026-07): one block per in-stock product
// (name/description/fact placeholders/every populated media reference, each
// appearing exactly once), a name-only list for out-of-stock products (no facts,
// no media), one block per topic, and BUSINESS_FACTS carrying only policy/
// contact/delivery-zone placeholders (product facts moved into the per-product
// blocks). A frame uses EITHER the v1 slots OR the v2 slots for a given concept,
// never both — RenderPrompt fills whichever slots the frame actually contains.
//
// SlotDeliveryZones is a v2 addition (2026-07-24, eval-run follow-up): it exposes
// the zone containment hierarchy (zone_level/parent_ref) that BUSINESS_FACTS's
// per-zone fact lines never carried, so the frame's "most precise zone wins" rule
// has actual hierarchy data to reason over instead of only a flat list of zone refs.
const (
	SlotAssistant          = "%%ASSISTANT%%"
	SlotKnowledgeBase      = "%%KNOWLEDGE_BASE%%"
	SlotDescriptions       = "%%DESCRIPTIONS%%"
	SlotFacts              = "%%FACTS%%"
	SlotMedia              = "%%MEDIA%%"
	SlotMediaAbsent        = "%%MEDIA_ABSENT%%"
	SlotProductsInStock    = "%%PRODUCTS_IN_STOCK%%"
	SlotProductsOutOfStock = "%%PRODUCTS_OUT_OF_STOCK%%"
	SlotTopics             = "%%TOPICS%%"
	SlotBusinessFacts      = "%%BUSINESS_FACTS%%"
	SlotDeliveryZones      = "%%DELIVERY_ZONES%%"
	SlotResponseSchema     = "%%RESPONSE_SCHEMA%%"
)

// BuildPrompt is the explicit two-step orchestration: BuildCatalog validates
// and resolves kbd_materials (private, KB-only), then RenderPrompt renders
// from the material-free PromptInput plus the resulting public Catalog
// projection, and finally the rendered text is checked against kbd_materials
// once more for defense in depth. It returns the catalog alongside so callers
// validate responses against exactly what the model saw.
//
// Callers that already hold a built Catalog (for example to validate a
// response against exactly what an earlier render produced) call RenderPrompt
// directly; production and the eval harness are expected to call BuildCatalog
// and RenderPrompt as the same explicit sequence this function runs.
func BuildPrompt(frame string, kb *KB) (string, *Catalog, error) {
	cat, err := BuildCatalog(kb)
	if err != nil {
		return "", nil, err
	}
	out, err := RenderPrompt(frame, kb.PromptInput(), cat)
	if err != nil {
		return "", nil, err
	}
	if err := ValidateNoMaterialLeak(out, kb.Materials); err != nil {
		return "", nil, err
	}
	return out, cat, nil
}

// RenderPrompt renders the stable prompt prefix — frame + assistant config +
// approved prose + fact-placeholder catalog + semantic media catalog +
// media-absence list + response schema — from approved ai_* content (input)
// and an already-built public catalog (cat). It deliberately does not accept
// a KB or []Material: kbd_materials must already have been validated and
// reduced to a Catalog by BuildCatalog before this is called, so the renderer
// itself has no route to read a material record or ID.
func RenderPrompt(frame string, input *PromptInput, cat *Catalog) (string, error) {
	out := frame
	out = strings.ReplaceAll(out, SlotAssistant, renderAssistant(input.Assistant))
	out = strings.ReplaceAll(out, SlotKnowledgeBase, renderTopics(input.Topics))
	out = strings.ReplaceAll(out, SlotDescriptions, renderDescriptions(input))
	out = strings.ReplaceAll(out, SlotFacts, renderFacts(cat.Facts))
	out = strings.ReplaceAll(out, SlotMedia, renderMediaCatalog(cat.Media))
	out = strings.ReplaceAll(out, SlotMediaAbsent, renderMediaAbsent(cat.Absent))
	out = strings.ReplaceAll(out, SlotProductsInStock, renderProductsInStock(input, cat))
	out = strings.ReplaceAll(out, SlotProductsOutOfStock, renderProductsOutOfStock(input))
	out = strings.ReplaceAll(out, SlotTopics, renderTopicBlocks(input, cat))
	out = strings.ReplaceAll(out, SlotBusinessFacts, renderBusinessFacts(cat.Facts))
	out = strings.ReplaceAll(out, SlotDeliveryZones, renderDeliveryZones(input.DeliveryZones))
	out = strings.ReplaceAll(out, SlotResponseSchema, RenderResponseSchema())
	if err := ValidatePrompt(out, cat); err != nil {
		return "", err
	}
	return out, nil
}

// renderAssistant renders ai_assistants config; missing prose is omitted —
// never a blank line.
func renderAssistant(a *Assistant) string {
	if a == nil {
		return ""
	}
	var b []string
	if s := strings.TrimSpace(a.Persona); s != "" {
		b = append(b, "# РОЛЬ\n"+s)
	}
	if s := strings.TrimSpace(a.Mission); s != "" {
		b = append(b, "# ЗАДАЧА\n"+s)
	}
	if s := strings.TrimSpace(a.Guardrails); s != "" {
		b = append(b, "# ОГРАНИЧЕНИЯ\n"+s)
	}
	if s := strings.TrimSpace(a.LanguagePolicy); s != "" {
		b = append(b, "# ЯЗЫК\n"+s)
	}
	return strings.Join(b, "\n\n")
}

func renderTopics(topics []Topic) string {
	var lines []string
	for _, t := range topics {
		if strings.TrimSpace(t.BodyMD) == "" {
			continue // missing prose is omitted, not a blank entry
		}
		head := "# topic: " + t.Slug
		if strings.TrimSpace(t.Title) != "" {
			head += " — " + t.Title
		}
		if len(t.Keywords) > 0 {
			head += "\nключевые слова: " + strings.Join(t.Keywords, ", ")
		}
		lines = append(lines, head+"\n"+strings.TrimSpace(t.BodyMD))
	}
	return strings.Join(lines, "\n\n")
}

// renderDescriptions renders entity trusted prose (product descriptions,
// tariff summaries, contact/policy prose). Empty fields are omitted entirely.
func renderDescriptions(input *PromptInput) string {
	var lines []string
	add := func(name, text string) {
		if s := strings.TrimSpace(text); s != "" {
			lines = append(lines, "- "+name+": "+s)
		}
	}
	for _, p := range input.Products {
		if !active(p.SalesStatus) {
			continue
		}
		name := p.Name
		if strings.TrimSpace(p.Category) != "" {
			name += " (" + p.Category + ")"
		}
		add(name, p.Description)
	}
	for _, t := range input.Tariffs {
		if !active(t.SalesStatus) {
			continue
		}
		var parts []string
		for _, s := range []string{t.Summary, t.LimitText, t.Advantages, t.Disadvantages} {
			if strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		if len(parts) > 0 {
			add("Тариф "+t.Name, strings.Join(parts, " "))
		}
	}
	if c := input.Contacts; c != nil {
		add("Адрес", c.Address)
		add("Реквизиты", c.LegalInformation)
		add("Обратный звонок", c.CallbackTime)
	}
	if p := input.Policies; p != nil {
		add("Предоплата", p.Prepayment)
		add("Рассрочка", p.Installment)
		add("Гарантия", p.Warranty)
	}
	for _, z := range input.DeliveryZones {
		if !active(z.SalesStatus) {
			continue
		}
		add("Доставка — "+z.Name, z.Notes)
	}
	return strings.Join(lines, "\n")
}

func renderFacts(facts []FactEntry) string {
	var lines []string
	for _, f := range facts {
		state := "—"
		if f.ReasoningState != "" {
			state = f.ReasoningState
		}
		note := "—"
		if f.UsageNote != "" {
			note = f.UsageNote
		}
		line := strings.Join([]string{f.Token, factLabel(f), string(f.Kind), state, note}, " | ")
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func factLabel(f FactEntry) string {
	switch f.Table {
	case "product", "tariff", "delivery":
		return f.Ref + " — " + f.Label
	default:
		return f.Label
	}
}

func renderMediaCatalog(media []MediaEntry) string {
	if len(media) == 0 {
		return "—"
	}
	var lines []string
	for _, m := range media {
		lines = append(lines, fmt.Sprintf("%s — %d — %s", m.Token, m.Count, m.Label))
	}
	return strings.Join(lines, "\n")
}

// renderMediaAbsent lists records with NO media in any column, as 2-segment
// references (never mistakable for a 3-segment attachable token). "—" when
// every record has at least one populated column, so the slot never renders
// blank under its frame header.
func renderMediaAbsent(absent []AbsentEntry) string {
	if len(absent) == 0 {
		return "—"
	}
	var lines []string
	for _, a := range absent {
		lines = append(lines, a.Table+"."+a.Ref+" — "+a.DisplayName)
	}
	return strings.Join(lines, "\n")
}

// mediaRefLines returns one "<column>_ref: <table>.<ref>.<column>" line per
// POPULATED media column an owner (table.ref) has — read from cat.Media (the
// already-validated public projection), in registry column order, so a v2 block
// can never list a column BuildCatalog didn't also approve. The field name is
// deliberately the full registry column name plus "_ref" (not an abbreviation)
// so every block's field name maps 1:1 onto the exact token segment a model
// must copy — no second naming layer to keep in sync.
func mediaRefLines(cat *Catalog, table, ref string) []string {
	var lines []string
	for _, spec := range mediaColumns[table] {
		token := table + "." + ref + "." + spec.Column
		if cat.MediaByToken(token) == nil {
			continue // empty column: omitted entirely, never an "unavailable:" line
		}
		lines = append(lines, spec.Column+"_ref: "+token)
	}
	return lines
}

// renderProductsInStock renders the v2 canonical-block frame's in-stock product
// list (%%PRODUCTS_IN_STOCK%%): one block per in-stock, active product with its
// name, description, fact placeholders, and every populated media reference —
// each appearing exactly once, empty fields simply omitted. This is the v2
// replacement for the flat FACTS/DESCRIPTIONS/MEDIA rendering of product rows
// (see renderFacts/renderDescriptions/renderMediaCatalog, kept for legacy frame
// compatibility) — an out-of-stock product never appears here at all (see
// renderProductsOutOfStock).
func renderProductsInStock(input *PromptInput, cat *Catalog) string {
	var blocks []string
	for _, p := range input.Products {
		if !active(p.SalesStatus) || !p.InStock {
			continue
		}
		lines := []string{"product: " + p.Ref, "name: " + p.Name}
		if s := strings.TrimSpace(p.Description); s != "" {
			lines = append(lines, "description: "+s)
		}
		if cat.FactByToken("{{product."+p.Ref+".price}}") != nil {
			lines = append(lines, "price_placeholder: {{product."+p.Ref+".price}}")
		}
		lines = append(lines, mediaRefLines(cat, "products", p.Ref)...)
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	if len(blocks) == 0 {
		return "—"
	}
	return strings.Join(blocks, "\n\n")
}

// renderProductsOutOfStock renders the v2 canonical-block frame's out-of-stock
// list (%%PRODUCTS_OUT_OF_STOCK%%): NAME ONLY, one line per out-of-stock, active
// product — no ref, no description, no fact placeholders, no media references.
// The frame's own instruction text (not this renderer) tells the model these are
// known-but-unavailable products it must not quote facts, prices, or media for.
func renderProductsOutOfStock(input *PromptInput) string {
	var lines []string
	for _, p := range input.Products {
		if !active(p.SalesStatus) || p.InStock {
			continue
		}
		lines = append(lines, "- "+p.Name)
	}
	if len(lines) == 0 {
		return "—"
	}
	return strings.Join(lines, "\n")
}

// renderTopicBlocks renders the v2 canonical-block frame's topic list
// (%%TOPICS%%): one block per topic with prose (slug/title/keywords/body plus
// every populated media reference), each field appearing exactly once. This is
// the v2 replacement for renderTopics (SlotKnowledgeBase, kept for legacy frame
// compatibility) — topic prose appears ONLY here in a v2 frame, never repeated
// in a separate knowledge-base section.
func renderTopicBlocks(input *PromptInput, cat *Catalog) string {
	var blocks []string
	for _, t := range input.Topics {
		if strings.TrimSpace(t.BodyMD) == "" {
			continue // missing prose is omitted, not a blank entry
		}
		lines := []string{"topic: " + t.Slug}
		if s := strings.TrimSpace(t.Title); s != "" {
			lines = append(lines, "title: "+s)
		}
		if len(t.Keywords) > 0 {
			lines = append(lines, "keywords: "+strings.Join(t.Keywords, ", "))
		}
		lines = append(lines, "body: "+strings.TrimSpace(t.BodyMD))
		lines = append(lines, mediaRefLines(cat, "topics", t.Slug)...)
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	if len(blocks) == 0 {
		return "—"
	}
	return strings.Join(blocks, "\n\n")
}

// renderBusinessFacts renders the v2 canonical-block frame's %%BUSINESS_FACTS%%
// slot — the same renderFacts line format, filtered to policy/contact/delivery-
// zone facts ONLY; product facts moved into the per-product blocks above so no
// fact ever appears in two places. Tariff facts are deliberately NOT included:
// shop-kb-v1 (the only schema_kb_v1 family today) has no tariffs; a future
// tariff-bearing v2 frame would need its own canonical tariff block (mirroring
// renderProductsInStock) before this slot could safely carry them too.
func renderBusinessFacts(facts []FactEntry) string {
	var kept []FactEntry
	for _, f := range facts {
		switch f.Table {
		case "policy", "contact", "delivery":
			kept = append(kept, f)
		}
	}
	return renderFacts(kept)
}

// renderDeliveryZones renders the v2 %%DELIVERY_ZONES%% slot: one token-free
// reference line per active zone (ref, name, zone_level, parent_ref, and any
// seller notes), in fixture order. Each zone's actual FACT placeholders
// (delivery_cost/delivery_in_days/delivery_available) still live only in
// BUSINESS_FACTS, keyed by the same ref — this block exists solely to make the
// containment hierarchy (city -> region -> country) visible, since neither
// ZoneLevel nor ParentRef appears anywhere else in a v2 prompt. Without it the
// frame's "use the most precise matching zone" rule had no hierarchy data to
// apply it to, so an unlisted city (e.g. one belonging to a country that DOES
// have a zone) could not be distinguished from a direction outside every zone.
//
// Zones are never subject to ApplyLimits (unlike products/topics), so this
// block is byte-identical across every shop-kb-v1-* catalog-size scenario —
// it does not compromise the "catalog size is the only swept variable"
// invariant those scenarios rely on.
//
// Known limitation, accepted: mapping an unnamed place to its containing zone
// (e.g. recognizing a city belongs to a listed country) relies on the model's
// own world knowledge — this block supplies the KB's zone structure, not a
// gazetteer. The frame's existing "not sure which zone -> escalate" rule
// (never guess) is the fail-safe for a place the model cannot confidently
// place: it degrades to escalation, not a wrong answer.
//
// Name and Notes are seller-authored free text; any literal "|" in them is
// sanitized so it can never be mistaken for a field separator in this
// pipe-delimited line format.
func renderDeliveryZones(zones []DeliveryZone) string {
	var lines []string
	for _, z := range zones {
		if !active(z.SalesStatus) {
			continue
		}
		parent := "—"
		if s := strings.TrimSpace(z.ParentRef); s != "" {
			parent = s
		}
		line := strings.Join([]string{
			z.Ref,
			sanitizeZoneField(z.Name),
			"zone_level: " + z.ZoneLevel,
			"parent_ref: " + parent,
		}, " | ")
		if s := strings.TrimSpace(z.Notes); s != "" {
			line += " | " + sanitizeZoneField(s)
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return "—"
	}
	return strings.Join(lines, "\n")
}

// sanitizeZoneField replaces a literal "|" in seller-authored zone text with a
// visually similar character so it can never be mistaken for a field
// separator in renderDeliveryZones's pipe-delimited line format.
func sanitizeZoneField(s string) string {
	return strings.ReplaceAll(s, "|", "／")
}

var (
	leftoverSlotPattern = regexp.MustCompile(`%%[A-Z_]+%%`)
	fileExtPattern      = regexp.MustCompile(`(?i)\.(jpe?g|png|gif|webp|svg|mp4|mov|avi|mkv|pdf|docx?|xlsx?|pptx?|mp3|wav|ogg)\b`)
	uuidPattern         = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
)

// ValidatePrompt is the structural trust-boundary gate applied while
// rendering: no leftover %%SLOT%% markers, no placeholder outside the fact
// catalog, and no UUID- or file-extension-shaped substring anywhere in the
// text (defense in depth against a catalog or registry bug). It needs only
// the rendered text and the catalog the model was shown — never
// kbd_materials — because RenderPrompt itself never receives kbd_materials.
// A violation is a hard render failure.
func ValidatePrompt(prompt string, cat *Catalog) error {
	if m := leftoverSlotPattern.FindString(prompt); m != "" {
		return fmt.Errorf("aiprompt: unfilled prompt slot %s", m)
	}
	for _, tok := range placeholderPattern.FindAllString(prompt, -1) {
		if cat.FactByToken(tok) == nil {
			return fmt.Errorf("aiprompt: prompt contains placeholder %s that is not in the fact catalog", tok)
		}
	}
	if m := uuidPattern.FindString(prompt); m != "" {
		return fmt.Errorf("aiprompt: prompt leaks a UUID: %s", m)
	}
	if m := fileExtPattern.FindString(prompt); m != "" {
		return fmt.Errorf("aiprompt: prompt leaks a filename extension: %s", m)
	}
	return nil
}

// ValidateNoMaterialLeak re-checks a rendered prompt against the actual
// kbd_materials rows behind it: every distinguishing identifying field (id,
// source ref, filename, storage backend, storage key, MIME type) must be
// absent from the text verbatim. Short values (len < 6) are skipped because a
// short generic token cannot be reliably distinguished from ordinary prompt
// vocabulary — real material identifiers, filenames, and storage keys are
// always longer than that in practice, and tests use deliberately distinctive
// sentinel values. BuildPrompt runs this automatically; callers that render
// via the split BuildCatalog+RenderPrompt sequence should call it explicitly
// with the same materials passed to BuildCatalog.
func ValidateNoMaterialLeak(prompt string, materials []Material) error {
	for _, mat := range materials {
		for _, secret := range []string{mat.ID, mat.SourceRef, mat.Filename, mat.StorageBackend, mat.StorageKey, mat.MimeType} {
			if len(secret) < 6 {
				continue
			}
			if strings.Contains(prompt, secret) {
				return fmt.Errorf("aiprompt: prompt leaks kbd_materials value %q", secret)
			}
		}
	}
	return nil
}
