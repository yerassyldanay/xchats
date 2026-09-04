package aiprompt

import (
	"fmt"
	"regexp"
	"strings"
)

// FactEntry is one model-facing row of the fact-placeholder catalog. It carries
// selection metadata and, for delivery availability only, a semantic reasoning state;
// exact customer values remain private in KB and are available only through ResolveFact.
type FactEntry struct {
	Token          string    `json:"token"` // {{table.ref.column}}, singular table segment
	Table          string    `json:"table"`
	Ref            string    `json:"ref"`
	Column         string    `json:"column"`
	Label          string    `json:"label"`
	Kind           ValueKind `json:"value_kind"`
	ReasoningState string    `json:"reasoning_state,omitempty"` // delivers | no_delivery; delivery-availability facts only
	UsageNote      string    `json:"usage_note,omitempty"`
}

// MediaEntry is one row of the PUBLIC semantic media catalog: the exact token
// the model must copy, its purpose label, kind, and count. This is the only
// media shape ever passed to RenderPrompt or serialized as catalog.json — it
// has no route to a material record or ID, so the renderer cannot leak one
// even by accident.
type MediaEntry struct {
	Token  string    `json:"token"` // table.ref.column, domain table segment
	Table  string    `json:"table"`
	Ref    string    `json:"ref"`
	Column string    `json:"column"`
	Label  string    `json:"label"`
	Kind   MediaKind `json:"kind"`
	Count  int       `json:"count"`
}

// AbsentEntry names an approved record whose media columns are ALL empty.
type AbsentEntry struct {
	Table       string `json:"table"` // domain table segment
	Ref         string `json:"ref"`
	DisplayName string `json:"display_name"`
}

// Catalog is everything derived from one approved KB for prompt building and
// response validation. Facts, Media, and Absent are the public projection:
// safe to serialize as catalog.json and safe to hand to RenderPrompt.
type Catalog struct {
	Facts  []FactEntry   `json:"facts"`
	Media  []MediaEntry  `json:"media"`
	Absent []AbsentEntry `json:"absent"`
}

var refPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// BuildCatalog derives the fact catalog, media catalog, and media-absence list
// from approved rows only (sales_status active; singletons always approved).
// Any invalid or stale material reference in a non-empty media column is a
// hard error — rendering fails, it is never silently treated as empty.
func BuildCatalog(kb *KB) (*Catalog, error) {
	cat := &Catalog{
		Facts:  []FactEntry{},
		Media:  []MediaEntry{},
		Absent: []AbsentEntry{},
	}
	if err := buildFacts(kb, cat); err != nil {
		return nil, err
	}
	if err := buildMedia(kb, cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func active(salesStatus string) bool { return salesStatus == "active" }

// productVisible reports whether a product is eligible for ANY prompt
// visibility at all — prose, fact tokens, and media alike. An inactive
// product (sales_status) was already fully suppressed before availability_
// status existed; an unavailable product (availability_status) is the new
// rule this migration adds: in_stock/preorder/on_demand stay fully
// visible, unavailable becomes name-only (renderProductsUnavailable) and
// must never carry a fact or media token (buildFacts/buildMedia below, and
// currentProduct in contract.go for the same rule at substitution time).
func productVisible(p *Product) bool {
	return active(p.SalesStatus) && p.AvailabilityStatus != "unavailable"
}

// productConcreteColumns/tariffConcreteColumns are the reservedRefs
// ValidateFacts checks a virtual ref against — the same column names
// factColumns["product"]/["tariff"] declare, kept in lockstep by
// facts_test.go rather than derived by reflection (the registry is small
// and this keeps the dependency direction obvious).
var (
	productConcreteColumns = []string{"price"}
	tariffConcreteColumns  = []string{"price", "fee"}
)

// productProse collects every OTHER prompt-visible field a product carries
// — everything ValidateFacts must confirm no additional-fact value leaks
// into (facts.go's doc comment).
func productProse(p *Product) map[string]string {
	return map[string]string{
		"description":        p.Description,
		"advantages":         p.Advantages,
		"disadvantages":      p.Disadvantages,
		"best_for":           p.BestFor,
		"not_for":            p.NotFor,
		"availability_note":  p.AvailabilityNote,
		"installation_terms": p.InstallationTerms,
		"warranty_terms":     p.WarrantyTerms,
	}
}

func tariffProse(t *Tariff) map[string]string {
	return map[string]string{
		"summary":       t.Summary,
		"limit_text":    t.LimitText,
		"advantages":    t.Advantages,
		"disadvantages": t.Disadvantages,
		"best_for":      t.BestFor,
		"not_for":       t.NotFor,
	}
}

// addVirtualFacts appends one FactEntry per eligible entry of facts (a
// product/tariff/tariff_info's AdditionalFacts) — validated first
// (ValidateFacts; defense in depth, see facts.go's doc comment), then
// skipping any entry whose value resolves empty exactly as addFact does
// for a concrete column (an empty exact value generates no placeholder, so
// a question needing it escalates rather than quoting nothing).
func addVirtualFacts(cat *Catalog, table, ref string, facts []AdditionalFact, reservedRefs []string, prose map[string]string) error {
	if len(facts) == 0 {
		return nil
	}
	if err := ValidateFacts(facts, reservedRefs, prose); err != nil {
		return fmt.Errorf("aiprompt: %s %s: %w", table, ref, err)
	}
	for _, f := range facts {
		text, kind, ok := valueKindAndText(f.Value)
		if !ok {
			continue // unreachable once ValidateFacts passed; defensive only
		}
		if kind == KindVirtualString && strings.TrimSpace(text) == "" {
			continue
		}
		cat.Facts = append(cat.Facts, FactEntry{
			Token: "{{" + table + "." + ref + "." + f.Ref + "}}",
			Table: table, Ref: ref, Column: f.Ref,
			Label: f.Ref, Kind: kind, UsageNote: f.Instruction,
		})
	}
	return nil
}

func validRef(ref string) error {
	if !refPattern.MatchString(ref) || strings.Contains(ref, ".") {
		return fmt.Errorf("aiprompt: invalid natural ref %q", ref)
	}
	return nil
}

// addFact appends a fact entry unless the value is missing/blank — an empty
// exact value generates no placeholder, so questions needing it escalate.
func addFact(cat *Catalog, table, ref string, col FactColumn, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	cat.Facts = append(cat.Facts, FactEntry{
		Token: "{{" + table + "." + ref + "." + col.Column + "}}",
		Table: table, Ref: ref, Column: col.Column,
		Label: col.Label, Kind: col.Kind, UsageNote: usageNote(col),
	})
}

func addDeliveryFlagFact(cat *Catalog, ref string, col FactColumn, available bool) {
	state := "delivers"
	if !available {
		state = "no_delivery"
	}
	cat.Facts = append(cat.Facts, FactEntry{
		Token: "{{delivery." + ref + "." + col.Column + "}}",
		Table: "delivery", Ref: ref, Column: col.Column,
		Label: col.Label, Kind: col.Kind, ReasoningState: state, UsageNote: usageNote(col),
	})
}

func buildFacts(kb *KB, cat *Catalog) error {
	for i := range kb.Products {
		p := &kb.Products[i]
		if !productVisible(p) {
			continue // unavailable/inactive: no fact token at all — see productVisible
		}
		if err := validRef(p.Ref); err != nil {
			return err
		}
		for _, col := range factColumns["product"] {
			switch col.Column {
			case "price":
				addFact(cat, "product", p.Ref, col, p.Price)
			}
		}
		if err := addVirtualFacts(cat, "product", p.Ref, p.AdditionalFacts, productConcreteColumns, productProse(p)); err != nil {
			return err
		}
	}
	for i := range kb.Tariffs {
		t := &kb.Tariffs[i]
		if !active(t.SalesStatus) {
			continue
		}
		if err := validRef(t.Ref); err != nil {
			return err
		}
		for _, col := range factColumns["tariff"] {
			switch col.Column {
			case "price":
				addFact(cat, "tariff", t.Ref, col, t.Price)
			case "fee":
				addFact(cat, "tariff", t.Ref, col, t.Fee)
			}
		}
		if err := addVirtualFacts(cat, "tariff", t.Ref, t.AdditionalFacts, tariffConcreteColumns, tariffProse(t)); err != nil {
			return err
		}
	}
	if ti := kb.TariffInfo; ti != nil {
		if err := addVirtualFacts(cat, "tariff_info", SingletonRef, ti.AdditionalFacts, nil, nil); err != nil {
			return err
		}
	}
	if c := kb.Contacts; c != nil {
		vals := map[string]string{
			"phone": c.Phone, "whatsapp": c.WhatsApp, "email": c.Email,
			"website": c.Website, "instagram": c.Instagram, "working_hours": c.WorkingHours,
		}
		for _, col := range factColumns["contact"] {
			addFact(cat, "contact", SingletonRef, col, vals[col.Column])
		}
	}
	if p := kb.Policies; p != nil {
		vals := map[string]string{
			"delivery_cost": p.DeliveryCost, "delivery_in_days": p.DeliveryInDays,
			"free_delivery_from": p.FreeDeliveryFrom, "min_order": p.MinOrder,
			"return_period_in_days": p.ReturnPeriodInDays,
			"outside_zones_note":    p.OutsideZonesNote,
		}
		for _, col := range factColumns["policy"] {
			addFact(cat, "policy", SingletonRef, col, vals[col.Column])
		}
	}
	if err := buildDeliveryZoneFacts(kb, cat); err != nil {
		return err
	}
	return nil
}

var validZoneLevels = map[string]bool{"city": true, "region": true, "country": true}

// buildDeliveryZoneFacts derives per-zone delivery facts and enforces every
// zones-specific fail-closed rule. An empty DeliveryZones means the feature
// is unused: no rule below applies, and delivery questions fall back to the
// flat ai_policies.delivery_cost/delivery_in_days facts already added above.
//
// Once any zone exists, three invariants are enforced (any violation is a
// hard BuildCatalog error, never a silent drop):
//  1. each row is internally consistent — DeliveryAvailable true requires a
//     non-blank cost and days; false requires both blank. A contradictory row
//     could otherwise be read either way at substitution time.
//  2. ParentRef chains resolve to an existing zone with no cycle, so a model
//     token referencing a parent zone is always resolvable.
//  3. the flat ai_policies delivery facts are blank (they would contradict
//     per-zone pricing) and OutsideZonesNote is non-blank (every unlisted
//     direction must have a real refusal token, never invented prose).
func buildDeliveryZoneFacts(kb *KB, cat *Catalog) error {
	if len(kb.DeliveryZones) == 0 {
		return nil
	}

	byRef := make(map[string]*DeliveryZone, len(kb.DeliveryZones))
	for i := range kb.DeliveryZones {
		z := &kb.DeliveryZones[i]
		if err := validRef(z.Ref); err != nil {
			return err
		}
		if byRef[z.Ref] != nil {
			return fmt.Errorf("aiprompt: duplicate delivery zone ref %q", z.Ref)
		}
		byRef[z.Ref] = z
		if !validZoneLevels[z.ZoneLevel] {
			return fmt.Errorf("aiprompt: delivery zone %q has invalid zone_level %q (want city, region, or country)", z.Ref, z.ZoneLevel)
		}
		if z.DeliveryAvailable {
			if strings.TrimSpace(z.DeliveryCost) == "" || strings.TrimSpace(z.DeliveryInDays) == "" {
				return fmt.Errorf("aiprompt: delivery zone %q is available but delivery_cost or delivery_in_days is blank", z.Ref)
			}
		} else if strings.TrimSpace(z.DeliveryCost) != "" || strings.TrimSpace(z.DeliveryInDays) != "" {
			return fmt.Errorf("aiprompt: delivery zone %q is unavailable but delivery_cost or delivery_in_days is set", z.Ref)
		}
	}
	for ref, z := range byRef {
		seen := map[string]bool{ref: true}
		for parent := z.ParentRef; parent != ""; {
			if seen[parent] {
				return fmt.Errorf("aiprompt: delivery zone %q has a cyclic parent_ref chain", ref)
			}
			seen[parent] = true
			next, ok := byRef[parent]
			if !ok {
				return fmt.Errorf("aiprompt: delivery zone %q references unknown parent_ref %q", ref, parent)
			}
			parent = next.ParentRef
		}
	}

	if kb.Policies == nil {
		return fmt.Errorf("aiprompt: ai_policies is required when ai_delivery_zones is non-empty")
	}
	if strings.TrimSpace(kb.Policies.DeliveryCost) != "" || strings.TrimSpace(kb.Policies.DeliveryInDays) != "" {
		return fmt.Errorf("aiprompt: ai_policies.delivery_cost and delivery_in_days must be blank when ai_delivery_zones is non-empty — use per-zone facts instead")
	}
	if strings.TrimSpace(kb.Policies.OutsideZonesNote) == "" {
		return fmt.Errorf("aiprompt: ai_policies.outside_zones_note is required when ai_delivery_zones is non-empty")
	}

	for _, z := range kb.DeliveryZones {
		if !active(z.SalesStatus) {
			continue
		}
		for _, col := range factColumns["delivery"] {
			switch col.Column {
			case "delivery_cost":
				addFact(cat, "delivery", z.Ref, col, z.DeliveryCost)
			case "delivery_in_days":
				addFact(cat, "delivery", z.Ref, col, z.DeliveryInDays)
			case "delivery_available":
				addDeliveryFlagFact(cat, z.Ref, col, z.DeliveryAvailable)
			}
		}
	}
	return nil
}

// resolvedFeaturedImage implements featured_image's override-with-fallback
// rule: an explicit value always wins; a NULL/blank featured_image resolves
// to the type's primary plural image column's first entry (already in send
// order) when that column is non-empty; otherwise featured_image is simply
// absent — no token, no error (buildMedia's normal "empty column" path).
// This is deliberately a per-request RESOLUTION, not a stored value:
// featured_image and the plural column remain two independently-writable
// KB columns (kb_product_upsert etc. — see mcpserver), so a later edit to
// either one is reflected the next time a catalog is built, with no
// migration or write-time sync required.
func resolvedFeaturedImage(explicit string, primaryImages []string) []string {
	if explicit != "" {
		return []string{explicit}
	}
	if len(primaryImages) > 0 {
		return []string{primaryImages[0]}
	}
	return nil
}

// mediaValues returns each registry media column's referenced material ids for
// one row, in registry order. Singular columns yield zero or one id.
func topicMedia(t *Topic) map[string][]string {
	return map[string][]string{
		"featured_image":      resolvedFeaturedImage(t.FeaturedImage, t.IllustrationImages),
		"illustration_images": t.IllustrationImages,
		"explainer_videos":    t.ExplainerVideos,
		"reference_documents": t.ReferenceDocuments,
	}
}

func productMedia(p *Product) map[string][]string {
	return map[string][]string{
		"featured_image":        resolvedFeaturedImage(p.FeaturedImage, p.GalleryImages),
		"gallery_images":        p.GalleryImages,
		"demo_videos":           p.DemoVideos,
		"certificate_documents": p.CertificateDocuments,
		"guarantee_documents":   p.GuaranteeDocuments,
	}
}

func tariffMedia(t *Tariff) map[string][]string {
	return map[string][]string{
		"featured_image":   resolvedFeaturedImage(t.FeaturedImage, t.PricingImages),
		"pricing_images":   t.PricingImages,
		"explainer_videos": t.ExplainerVideos,
		"terms_documents":  t.TermsDocuments,
	}
}

func contactsMedia(c *Contacts) map[string][]string {
	return map[string][]string{
		"contact_card_image":      singular(c.ContactCardImage),
		"location_map_image":      singular(c.LocationMapImage),
		"company_legal_documents": c.CompanyLegalDocuments,
	}
}

func policiesMedia(p *Policies) map[string][]string {
	return map[string][]string{
		"commerce_policy_documents": p.CommercePolicyDocuments,
	}
}

func singular(id string) []string {
	if id == "" {
		return nil
	}
	return []string{id}
}

func buildMedia(kb *KB, cat *Catalog) error {
	type owner struct {
		table, ref, display string
		values              map[string][]string
	}
	var owners []owner
	for i := range kb.Topics {
		t := &kb.Topics[i]
		if err := validRef(t.Slug); err != nil {
			return err
		}
		owners = append(owners, owner{"topics", t.Slug, t.Title, topicMedia(t)})
	}
	for i := range kb.Products {
		p := &kb.Products[i]
		if !productVisible(p) {
			continue // unavailable/inactive: no media token, no Absent entry either
		}
		owners = append(owners, owner{"products", p.Ref, p.Name, productMedia(p)})
	}
	for i := range kb.Tariffs {
		t := &kb.Tariffs[i]
		if !active(t.SalesStatus) {
			continue
		}
		owners = append(owners, owner{"tariffs", t.Ref, t.Name, tariffMedia(t)})
	}
	if kb.Contacts != nil {
		owners = append(owners, owner{"contacts", SingletonRef, "Контакты", contactsMedia(kb.Contacts)})
	}
	if kb.Policies != nil {
		owners = append(owners, owner{"policies", SingletonRef, "Условия", policiesMedia(kb.Policies)})
	}

	for _, o := range owners {
		hasAny := false
		for _, spec := range mediaColumns[o.table] {
			ids := o.values[spec.Column]
			if len(ids) == 0 {
				continue // NULL singular / empty plural → no token, no error
			}
			if spec.Singular && len(ids) > 1 {
				return fmt.Errorf("aiprompt: singular media column %s.%s.%s holds %d references", o.table, o.ref, spec.Column, len(ids))
			}
			for _, id := range ids {
				if err := validateMaterialRef(kb, id, spec, o.table, o.ref); err != nil {
					return err
				}
			}
			hasAny = true
			token := o.table + "." + o.ref + "." + spec.Column
			cat.Media = append(cat.Media, MediaEntry{
				Token: token,
				Table: o.table, Ref: o.ref, Column: spec.Column,
				Label: spec.Label, Kind: spec.Kind, Count: len(ids),
			})
		}
		if !hasAny {
			cat.Absent = append(cat.Absent, AbsentEntry{Table: o.table, Ref: o.ref, DisplayName: o.display})
		}
	}
	return nil
}

// validateMaterialRef enforces the fail-closed media rules: same organization,
// file-backed, storage locator present, positive size, customer-sendable, and
// a MIME type compatible with the column's media kind. Any violation fails
// prompt rendering — a broken reference is never silently treated as empty.
func validateMaterialRef(kb *KB, id string, spec MediaColumn, table, ref string) error {
	at := fmt.Sprintf("%s.%s.%s", table, ref, spec.Column)
	m := kb.MaterialByID(id)
	if m == nil {
		return fmt.Errorf("aiprompt: %s references nonexistent material %s", at, id)
	}
	if kb.OrganizationID != "" && m.OrganizationID != kb.OrganizationID {
		return fmt.Errorf("aiprompt: %s references material %s from another organization", at, id)
	}
	if m.SourceType != "file" {
		return fmt.Errorf("aiprompt: %s references non-file material %s (source_type=%s)", at, id, m.SourceType)
	}
	if m.StorageBackend == "" || m.StorageKey == "" {
		return fmt.Errorf("aiprompt: %s references material %s without a storage locator", at, id)
	}
	if m.SizeBytes <= 0 {
		return fmt.Errorf("aiprompt: %s references zero-size material %s", at, id)
	}
	if m.CustomerVisibility != "visible" && m.CustomerVisibility != "auto" {
		return fmt.Errorf("aiprompt: %s references non-customer-sendable material %s (customer_visibility=%s)", at, id, m.CustomerVisibility)
	}
	if !mimeMatches(spec.Kind, m.MimeType) {
		return fmt.Errorf("aiprompt: %s expects %s but material %s has MIME %q", at, spec.Kind, id, m.MimeType)
	}
	return nil
}

func mimeMatches(kind MediaKind, mime string) bool {
	switch kind {
	case MediaImage:
		return strings.HasPrefix(mime, "image/")
	case MediaVideo:
		return strings.HasPrefix(mime, "video/")
	case MediaAudio:
		return strings.HasPrefix(mime, "audio/")
	case MediaDocument:
		return strings.HasPrefix(mime, "application/") || strings.HasPrefix(mime, "text/")
	default:
		return false
	}
}

// FactByToken returns the fact entry for an exact {{…}} token, or nil.
func (c *Catalog) FactByToken(token string) *FactEntry {
	for i := range c.Facts {
		if c.Facts[i].Token == token {
			return &c.Facts[i]
		}
	}
	return nil
}

// MediaByToken returns the media entry for an exact catalog token, or nil.
func (c *Catalog) MediaByToken(token string) *MediaEntry {
	for i := range c.Media {
		if c.Media[i].Token == token {
			return &c.Media[i]
		}
	}
	return nil
}
