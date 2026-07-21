package aiprompt

import (
	"fmt"
	"regexp"
	"strings"
)

// FactEntry is one row of the fact-placeholder catalog.
type FactEntry struct {
	Token     string    `json:"token"` // {{table.ref.column}}, singular table segment
	Table     string    `json:"table"`
	Ref       string    `json:"ref"`
	Column    string    `json:"column"`
	Label     string    `json:"label"`
	Value     string    `json:"value"` // display value; for in_stock: "да"/"нет"
	Kind      ValueKind `json:"value_kind"`
	UsageNote string    `json:"usage_note,omitempty"`

	stockValue bool // parsed in_stock value, valid when Kind == KindStockFlag
}

// MediaEntry is one row of the semantic media catalog.
type MediaEntry struct {
	Token       string    `json:"token"` // table.ref.column, domain table segment
	Table       string    `json:"table"`
	Ref         string    `json:"ref"`
	Column      string    `json:"column"`
	Label       string    `json:"label"`
	Kind        MediaKind `json:"kind"`
	Count       int       `json:"count"`
	MaterialIDs []string  `json:"-"` // backend-only; never rendered or exported
}

// AbsentEntry names an approved record whose media columns are ALL empty.
type AbsentEntry struct {
	Table       string `json:"table"` // domain table segment
	Ref         string `json:"ref"`
	DisplayName string `json:"display_name"`
}

// Catalog is everything derived from one approved KB for prompt building and
// response validation.
type Catalog struct {
	Facts  []FactEntry
	Media  []MediaEntry
	Absent []AbsentEntry
}

var refPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// BuildCatalog derives the fact catalog, media catalog, and media-absence list
// from approved rows only (sales_status active; singletons always approved).
// Any invalid or stale material reference in a non-empty media column is a
// hard error — rendering fails, it is never silently treated as empty.
func BuildCatalog(kb *KB) (*Catalog, error) {
	cat := &Catalog{}
	if err := buildFacts(kb, cat); err != nil {
		return nil, err
	}
	if err := buildMedia(kb, cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func active(salesStatus string) bool { return salesStatus == "" || salesStatus == "active" }

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
		Token:     "{{" + table + "." + ref + "." + col.Column + "}}",
		Table:     table, Ref: ref, Column: col.Column,
		Label: col.Label, Value: value, Kind: col.Kind, UsageNote: usageNote(col),
	})
}

func addStockFact(cat *Catalog, ref string, col FactColumn, inStock bool) {
	display := "да"
	if !inStock {
		display = "нет"
	}
	e := FactEntry{
		Token:     "{{product." + ref + "." + col.Column + "}}",
		Table:     "product", Ref: ref, Column: col.Column,
		Label: col.Label, Value: display, Kind: col.Kind, UsageNote: usageNote(col),
		stockValue: inStock,
	}
	cat.Facts = append(cat.Facts, e)
}

func buildFacts(kb *KB, cat *Catalog) error {
	for _, p := range kb.Products {
		if !active(p.SalesStatus) {
			continue
		}
		if err := validRef(p.Ref); err != nil {
			return err
		}
		for _, col := range factColumns["product"] {
			switch col.Column {
			case "price":
				addFact(cat, "product", p.Ref, col, p.Price)
			case "in_stock":
				addStockFact(cat, p.Ref, col, p.InStock)
			}
		}
	}
	for _, t := range kb.Tariffs {
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
		}
		for _, col := range factColumns["policy"] {
			addFact(cat, "policy", SingletonRef, col, vals[col.Column])
		}
	}
	return nil
}

// mediaValues returns each registry media column's referenced material ids for
// one row, in registry order. Singular columns yield zero or one id.
func topicMedia(t *Topic) map[string][]string {
	return map[string][]string{
		"featured_image":        singular(t.FeaturedImage),
		"illustration_images":   t.IllustrationImages,
		"explainer_videos":      t.ExplainerVideos,
		"narration_audio_files": t.NarrationAudioFiles,
		"reference_documents":   t.ReferenceDocuments,
	}
}

func productMedia(p *Product) map[string][]string {
	return map[string][]string{
		"featured_image":          singular(p.FeaturedImage),
		"gallery_images":          p.GalleryImages,
		"demo_videos":             p.DemoVideos,
		"audio_description_files": p.AudioDescriptionFiles,
		"certificate_documents":   p.CertificateDocuments,
		"manual_documents":        p.ManualDocuments,
		"guarantee_documents":     p.GuaranteeDocuments,
		"specification_documents": p.SpecificationDocuments,
	}
}

func tariffMedia(t *Tariff) map[string][]string {
	return map[string][]string{
		"featured_image":  singular(t.FeaturedImage),
		"pricing_images":  t.PricingImages,
		"explainer_videos": t.ExplainerVideos,
		"terms_documents": t.TermsDocuments,
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
		if !active(p.SalesStatus) {
			continue
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
			cat.Media = append(cat.Media, MediaEntry{
				Token: o.table + "." + o.ref + "." + spec.Column,
				Table: o.table, Ref: o.ref, Column: spec.Column,
				Label: spec.Label, Kind: spec.Kind, Count: len(ids), MaterialIDs: ids,
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
	if m.CustomerVisibility != "visible" {
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
