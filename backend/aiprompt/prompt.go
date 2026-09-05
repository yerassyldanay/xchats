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
//
// SlotTariffs is the v5 addition (2026-08): one canonical block per active tariff,
// exactly as renderBusinessFacts' own doc comment prescribed before it existed.
// Until v5 the whole tariff table was silently unreachable by the model —
// BuildCatalog built tariff facts and media (catalog.go) but no renderer emitted
// them, so an operator who priced a tariff in its typed columns watched the
// assistant escalate ("нет этой информации") rather than quote it. See
// renderTariffs.
//
// SlotProductsAvailable/SlotProductsUnavailable/SlotTariffCatalog/
// SlotTariffInfo are the v6 addition (virtual fact columns,
// 0017_kb_virtual_facts): v6 renders every product/tariff block through
// renderProductsAvailable/renderTariffCatalog instead of
// renderProductsInStock/renderTariffs, adding brand/advantages/
// disadvantages/best_for/not_for/availability/installation/warranty prose
// and each entity's virtual facts beside their instructions
// (renderAdditionalFactLines) — new slots, not new behavior grafted onto
// the v4/v5 ones, so an old pinned frame's rendering never changes shape.
// SlotTariffInfo carries the organization-wide tariff_info singleton's own
// virtual facts, which have no per-tariff or per-product block to live in.
const (
	SlotAssistant           = "%%ASSISTANT%%"
	SlotKnowledgeBase       = "%%KNOWLEDGE_BASE%%"
	SlotDescriptions        = "%%DESCRIPTIONS%%"
	SlotFacts               = "%%FACTS%%"
	SlotMedia               = "%%MEDIA%%"
	SlotMediaAbsent         = "%%MEDIA_ABSENT%%"
	SlotProductsInStock     = "%%PRODUCTS_IN_STOCK%%"
	SlotProductsOutOfStock  = "%%PRODUCTS_OUT_OF_STOCK%%"
	SlotTariffs             = "%%TARIFFS%%"
	SlotProductsAvailable   = "%%PRODUCTS_AVAILABLE%%"
	SlotProductsUnavailable = "%%PRODUCTS_UNAVAILABLE%%"
	SlotTariffCatalog       = "%%TARIFF_CATALOG%%"
	SlotTariffInfo          = "%%TARIFF_INFO%%"
	SlotTopics              = "%%TOPICS%%"
	SlotBusinessFacts       = "%%BUSINESS_FACTS%%"
	SlotDeliveryZones       = "%%DELIVERY_ZONES%%"
	SlotResponseSchema      = "%%RESPONSE_SCHEMA%%"
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

// BuildPromptV7 is BuildPrompt for the v7+ frame family — see RenderPromptV7.
func BuildPromptV7(frame string, kb *KB) (string, *Catalog, error) {
	cat, err := BuildCatalog(kb)
	if err != nil {
		return "", nil, err
	}
	out, err := RenderPromptV7(frame, kb.PromptInput(), cat)
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
	return renderPromptWithSchema(frame, input, cat, RenderResponseSchema())
}

// RenderPromptV7 is RenderPrompt for the v7+ frame family (PromptRefShopKBV7/
// V7TG): identical rendering for every slot except %%RESPONSE_SCHEMA%%, which
// substitutes RenderResponseSchemaV7 (the schema including the optional
// kb_gap diagnostic — see contract.go/kbgap.go) instead of
// RenderResponseSchema. RenderPrompt itself must keep substituting exactly
// the v6-and-earlier schema, unconditionally, for every existing caller — see
// ValidateResponseV7's doc comment for why that means a sibling function
// here rather than a parameter on RenderPrompt.
func RenderPromptV7(frame string, input *PromptInput, cat *Catalog) (string, error) {
	return renderPromptWithSchema(frame, input, cat, RenderResponseSchemaV7())
}

// renderPromptWithSchema is RenderPrompt/RenderPromptV7's shared
// implementation, parameterized only by which rendered %%RESPONSE_SCHEMA%%
// text to substitute — every other slot is identical across every frame
// version, so only the schema choice needs to vary, not this whole function.
func renderPromptWithSchema(frame string, input *PromptInput, cat *Catalog, schemaJSON string) (string, error) {
	out := frame
	out = strings.ReplaceAll(out, SlotAssistant, renderAssistant(input.Assistant))
	out = strings.ReplaceAll(out, SlotKnowledgeBase, renderTopics(input.Topics))
	out = strings.ReplaceAll(out, SlotDescriptions, renderDescriptions(input))
	out = strings.ReplaceAll(out, SlotFacts, renderFacts(cat.Facts))
	out = strings.ReplaceAll(out, SlotMedia, renderMediaCatalog(cat.Media))
	out = strings.ReplaceAll(out, SlotMediaAbsent, renderMediaAbsent(cat.Absent))
	out = strings.ReplaceAll(out, SlotProductsInStock, renderProductsInStock(input, cat))
	out = strings.ReplaceAll(out, SlotProductsOutOfStock, renderProductsOutOfStock(input))
	out = strings.ReplaceAll(out, SlotTariffs, renderTariffs(input, cat))
	out = strings.ReplaceAll(out, SlotProductsAvailable, renderProductsAvailable(input, cat))
	out = strings.ReplaceAll(out, SlotProductsUnavailable, renderProductsUnavailable(input))
	out = strings.ReplaceAll(out, SlotTariffCatalog, renderTariffCatalog(input, cat))
	out = strings.ReplaceAll(out, SlotTariffInfo, renderTariffInfo(input, cat))
	out = strings.ReplaceAll(out, SlotTopics, renderTopicBlocks(input, cat))
	out = strings.ReplaceAll(out, SlotBusinessFacts, renderBusinessFacts(cat.Facts))
	out = strings.ReplaceAll(out, SlotDeliveryZones, renderDeliveryZones(input.DeliveryZones))
	out = strings.ReplaceAll(out, SlotResponseSchema, schemaJSON)
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
		if !productVisible(&p) {
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
		if !active(p.SalesStatus) || p.AvailabilityStatus != "unavailable" {
			continue
		}
		lines = append(lines, "- "+p.Name)
	}
	if len(lines) == 0 {
		return "—"
	}
	return strings.Join(lines, "\n")
}

// availabilityStatusRUWords is the reviewed Russian wording for each
// availability_status value, shown as plain descriptive text directly in a
// v6 product block (like pricing_type on a tariff block) — not a hidden
// exact value, so it is interpolated as-is rather than hidden behind a
// fact token. unavailable never reaches this: renderProductsAvailable only
// calls it for a productVisible row.
var availabilityStatusRUWords = map[string]string{
	"in_stock":  "в наличии",
	"preorder":  "предзаказ",
	"on_demand": "под заказ",
}

func availabilityStatusRU(status string) string {
	s, ok := availabilityStatusRUWords[status]
	if !ok {
		// Unreachable through the normal Generate/BuildPrompt path:
		// BuildCatalog's validAvailabilityStatuses check (catalog.go) rejects
		// any status outside the four known values before RenderPrompt is
		// ever called, and renderProductsAvailable only calls this for a
		// productVisible row (in_stock/preorder/on_demand). A caller that
		// renders without building the catalog first is the only way to
		// reach this — rendering an unrecognized status as if it were safe
		// prose is exactly the leak this feature exists to prevent, so this
		// fails loud (panic, recovered by the HTTP layer's gin.Recovery())
		// rather than silently passing an unvetted string through.
		panic(fmt.Sprintf("aiprompt: availabilityStatusRU: unrecognized status %q — BuildCatalog should have rejected it first", status))
	}
	return s
}

// renderAdditionalFactLines renders one "fact: <instruction> — {{token}}"
// line per entry of facts that BuildCatalog actually approved a token for
// (cat.FactByToken != nil) — an entry BuildCatalog dropped (empty/invalid
// value) is omitted entirely, never rendered as an "unavailable:" line,
// matching mediaRefLines' own omission-only convention. Shared by every
// v6 block (product, tariff, tariff_info) that carries virtual facts.
func renderAdditionalFactLines(cat *Catalog, table, ref string, facts []AdditionalFact) []string {
	var lines []string
	for _, f := range facts {
		token := "{{" + table + "." + ref + "." + f.Ref + "}}"
		if cat.FactByToken(token) == nil {
			continue
		}
		lines = append(lines, "fact: "+strings.TrimSpace(f.Instruction)+" — "+token)
	}
	return lines
}

// renderProductsAvailable renders the v6 frame's available-product list
// (%%PRODUCTS_AVAILABLE%%): one block per productVisible product (in_stock
// | preorder | on_demand, active) with every prose field, the availability
// status/note, price placeholder, virtual facts beside their instructions,
// and every populated media reference — the v6 replacement for
// renderProductsInStock (kept, frozen, for v4/v5 — see this file's slot
// doc comment).
func renderProductsAvailable(input *PromptInput, cat *Catalog) string {
	var blocks []string
	for _, p := range input.Products {
		if !productVisible(&p) {
			continue
		}
		lines := []string{"product: " + p.Ref, "name: " + p.Name}
		for _, f := range []struct{ field, text string }{
			{"brand", p.Brand},
			{"description", p.Description},
			{"advantages", p.Advantages},
			{"disadvantages", p.Disadvantages},
			{"best_for", p.BestFor},
			{"not_for", p.NotFor},
		} {
			if s := strings.TrimSpace(f.text); s != "" {
				lines = append(lines, f.field+": "+s)
			}
		}
		lines = append(lines, "availability_status: "+availabilityStatusRU(p.AvailabilityStatus))
		for _, f := range []struct{ field, text string }{
			{"availability_note", p.AvailabilityNote},
			{"installation_terms", p.InstallationTerms},
			{"warranty_terms", p.WarrantyTerms},
		} {
			if s := strings.TrimSpace(f.text); s != "" {
				lines = append(lines, f.field+": "+s)
			}
		}
		if cat.FactByToken("{{product."+p.Ref+".price}}") != nil {
			lines = append(lines, "price_placeholder: {{product."+p.Ref+".price}}")
		}
		lines = append(lines, renderAdditionalFactLines(cat, "product", p.Ref, p.AdditionalFacts)...)
		lines = append(lines, mediaRefLines(cat, "products", p.Ref)...)
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	if len(blocks) == 0 {
		return "—"
	}
	return strings.Join(blocks, "\n\n")
}

// renderProductsUnavailable renders the v6 frame's unavailable-product list
// (%%PRODUCTS_UNAVAILABLE%%): NAME ONLY, the same shape as v4/v5's
// renderProductsOutOfStock — an unavailable product is known but must
// never carry a fact or media token (productVisible/buildFacts/buildMedia).
func renderProductsUnavailable(input *PromptInput) string {
	return renderProductsOutOfStock(input)
}

// renderTariffCatalog renders the v6 frame's tariff list
// (%%TARIFF_CATALOG%%) — renderTariffs plus best_for/not_for prose and
// virtual facts beside their instructions. The v6 replacement for
// renderTariffs (kept, frozen, for v5 — see this file's slot doc comment).
func renderTariffCatalog(input *PromptInput, cat *Catalog) string {
	var blocks []string
	for _, t := range input.Tariffs {
		if !active(t.SalesStatus) {
			continue
		}
		lines := []string{"tariff: " + t.Ref, "name: " + t.Name}
		if s := strings.TrimSpace(t.PricingType); s != "" {
			lines = append(lines, "pricing_type: "+s)
		}
		for _, f := range []struct{ field, text string }{
			{"summary", t.Summary},
			{"limit", t.LimitText},
			{"advantages", t.Advantages},
			{"disadvantages", t.Disadvantages},
			{"best_for", t.BestFor},
			{"not_for", t.NotFor},
		} {
			if s := strings.TrimSpace(f.text); s != "" {
				lines = append(lines, f.field+": "+s)
			}
		}
		for _, col := range []string{"price", "fee"} {
			token := "{{tariff." + t.Ref + "." + col + "}}"
			if cat.FactByToken(token) != nil {
				lines = append(lines, col+"_placeholder: "+token)
			}
		}
		lines = append(lines, renderAdditionalFactLines(cat, "tariff", t.Ref, t.AdditionalFacts)...)
		lines = append(lines, mediaRefLines(cat, "tariffs", t.Ref)...)
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	if len(blocks) == 0 {
		return "—"
	}
	return strings.Join(blocks, "\n\n")
}

// renderTariffInfo renders the v6 frame's organization-wide tariff_info
// block (%%TARIFF_INFO%%): every virtual fact BuildCatalog approved for
// the tariff_info singleton, beside its instruction — "—" when the
// singleton is absent or carries no (approved) facts, same convention as
// every other possibly-empty slot in this file.
func renderTariffInfo(input *PromptInput, cat *Catalog) string {
	if input.TariffInfo == nil {
		return "—"
	}
	lines := renderAdditionalFactLines(cat, "tariff_info", SingletonRef, input.TariffInfo.AdditionalFacts)
	if len(lines) == 0 {
		return "—"
	}
	return strings.Join(lines, "\n")
}

// renderTariffs renders the v5 frame's tariff list (%%TARIFFS%%): one block per
// ACTIVE tariff with its ref, name, seller prose, fact placeholders, and every
// populated media reference — each appearing exactly once, empty fields simply
// omitted. Structurally the tariff twin of renderProductsInStock, which is
// precisely what renderBusinessFacts' doc comment said a tariff-bearing frame
// would need before tariff facts could safely be rendered anywhere.
//
// There is no in-stock/out-of-stock split here: ai_tariffs has no stock concept
// (a plan is offered or it is not), so sales_status alone decides, and an
// inactive tariff is omitted entirely rather than getting a name-only line the
// way an out-of-stock product does.
//
// pricing_type is carried because it changes how the very same price/fee value
// must be read (a "percentage" fee is a rate, not an amount) — without it a
// model quoting {{tariff.x.fee}} has no way to tell those apart.
func renderTariffs(input *PromptInput, cat *Catalog) string {
	var blocks []string
	for _, t := range input.Tariffs {
		if !active(t.SalesStatus) {
			continue
		}
		lines := []string{"tariff: " + t.Ref, "name: " + t.Name}
		if s := strings.TrimSpace(t.PricingType); s != "" {
			lines = append(lines, "pricing_type: "+s)
		}
		for _, f := range []struct{ field, text string }{
			{"summary", t.Summary},
			{"limit", t.LimitText},
			{"advantages", t.Advantages},
			{"disadvantages", t.Disadvantages},
		} {
			if s := strings.TrimSpace(f.text); s != "" {
				lines = append(lines, f.field+": "+s)
			}
		}
		// price and fee are the two fact columns registry.go declares for
		// tariffs; each is emitted only when BuildCatalog actually approved a
		// token for it, exactly as renderProductsInStock gates its price.
		for _, col := range []string{"price", "fee"} {
			token := "{{tariff." + t.Ref + "." + col + "}}"
			if cat.FactByToken(token) != nil {
				lines = append(lines, col+"_placeholder: "+token)
			}
		}
		lines = append(lines, mediaRefLines(cat, "tariffs", t.Ref)...)
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	if len(blocks) == 0 {
		return "—"
	}
	return strings.Join(blocks, "\n\n")
}

// renderTopicBlocks renders the v2 canonical-block frame's topic list
// (%%TOPICS%%): one block per topic with prose (slug/title/body plus
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
// fact ever appears in two places.
//
// Tariff facts are excluded for that same one-place rule, NOT (as this comment
// previously claimed) because tariffs are unsupported: since v5 they render in
// their own canonical block, renderTariffs. Adding them back here would put
// every tariff price in the prompt twice.
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
