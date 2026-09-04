package kbfixture

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"gopkg.in/yaml.v3"
)

// Load reads and decodes a schema-shaped fixture file from path and converts
// it into an *aiprompt.KB. See Decode for the strict-decode and validation
// rules.
func Load(path string) (*aiprompt.KB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("kbfixture: read %s: %w", path, err)
	}
	kb, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("kbfixture: %s: %w", path, err)
	}
	return kb, nil
}

// Decode strictly parses a schema-shaped fixture document and converts it 1:1
// into an *aiprompt.KB. Unknown top-level or nested keys — including every
// generic alias this family does not support (values, media, files,
// availability, delivery_time, work_hours, ...) — are decode errors.
// delivery_available must be an explicit YAML boolean; a quoted "yes"/"no"/
// "true"/"false" string is rejected, and an omitted delivery_available key is
// rejected too (the column is NOT NULL with no default). availability_status
// may be omitted (it defaults to "in_stock", mirroring the column's own DB
// default) but an explicit value must be one of the closed
// in_stock|preorder|on_demand|unavailable vocabulary. Closed enum columns
// (source_type, processing_status, customer_visibility, sales_status,
// pricing_type, availability_status) are validated against their exact
// DECISIONS.md vocabulary. Every additional_facts entry is validated by
// aiprompt.ValidateFacts (ref syntax, uniqueness, no collision with a
// concrete column, instruction hygiene, ...) before it ever reaches the
// returned KB.
func Decode(data []byte) (*aiprompt.KB, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var fx Fixture
	if err := dec.Decode(&fx); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return toKB(&fx)
}

var (
	validSourceTypes        = set("text", "url", "instruction", "file")
	validProcessingStatus   = set("uploaded", "extracting", "parsed", "built", "needs_human", "failed")
	validCustomerVisibility = set("", "auto", "invisible", "visible")
	validSalesStatus        = set("active", "inactive")
	validPricingType        = set("fixed", "percentage", "tiered", "hybrid")
	validZoneLevel          = set("city", "region", "country")
	validAvailabilityStatus = set("in_stock", "preorder", "on_demand", "unavailable")
)

// toAdditionalFacts converts a fixture's additional_facts wire list into
// aiprompt's own type — a 1:1 field copy; every semantic rule (ref syntax,
// duplicates, reserved-column collisions, instruction hygiene, ...) is
// enforced once, centrally, by aiprompt.ValidateFacts (called from each of
// this file's three call sites below), not duplicated here.
func toAdditionalFacts(facts []AIAdditionalFact) []aiprompt.AdditionalFact {
	if len(facts) == 0 {
		return nil
	}
	out := make([]aiprompt.AdditionalFact, len(facts))
	for i, f := range facts {
		out[i] = aiprompt.AdditionalFact{Ref: f.Ref, Value: f.Value.V, Instruction: f.Instruction}
	}
	return out
}

func set(vals ...string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

func toKB(fx *Fixture) (*aiprompt.KB, error) {
	orgID := strings.TrimSpace(fx.OrganizationID)
	if orgID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}

	kb := &aiprompt.KB{OrganizationID: orgID}

	if fx.AIAssistants != nil {
		a := fx.AIAssistants
		if strings.TrimSpace(a.Persona) == "" {
			return nil, fmt.Errorf("ai_assistants.persona is required")
		}
		if strings.TrimSpace(a.Guardrails) == "" {
			return nil, fmt.Errorf("ai_assistants.guardrails is required")
		}
		if strings.TrimSpace(a.LanguagePolicy) == "" {
			return nil, fmt.Errorf("ai_assistants.language_policy is required")
		}
		kb.Assistant = &aiprompt.Assistant{
			Persona: a.Persona, Mission: a.Mission, Guardrails: a.Guardrails,
			LanguagePolicy: a.LanguagePolicy, ReplyMaxWords: a.ReplyMaxWords,
		}
	}

	seenSlug := map[string]bool{}
	for i, t := range fx.AITopics {
		if strings.TrimSpace(t.Slug) == "" {
			return nil, fmt.Errorf("ai_topics[%d]: slug is required", i)
		}
		if seenSlug[t.Slug] {
			return nil, fmt.Errorf("ai_topics: duplicate slug %q", t.Slug)
		}
		seenSlug[t.Slug] = true
		if strings.TrimSpace(t.Title) == "" {
			return nil, fmt.Errorf("ai_topics[%s]: title is required", t.Slug)
		}
		if strings.TrimSpace(t.BodyMD) == "" {
			return nil, fmt.Errorf("ai_topics[%s]: body_md is required", t.Slug)
		}
		kb.Topics = append(kb.Topics, aiprompt.Topic{
			Slug: t.Slug, Title: t.Title, BodyMD: t.BodyMD,
			FeaturedImage: t.FeaturedImage, IllustrationImages: t.IllustrationImages,
			ExplainerVideos: t.ExplainerVideos, ReferenceDocuments: t.ReferenceDocuments,
		})
	}

	seenProductRef := map[string]bool{}
	for i, p := range fx.AIProducts {
		if strings.TrimSpace(p.Ref) == "" {
			return nil, fmt.Errorf("ai_products[%d]: ref is required", i)
		}
		if seenProductRef[p.Ref] {
			return nil, fmt.Errorf("ai_products: duplicate ref %q", p.Ref)
		}
		seenProductRef[p.Ref] = true
		if strings.TrimSpace(p.Name) == "" {
			return nil, fmt.Errorf("ai_products[%s]: name is required", p.Ref)
		}
		availabilityStatus := p.AvailabilityStatus
		if availabilityStatus == "" {
			availabilityStatus = "in_stock"
		}
		if !validAvailabilityStatus[availabilityStatus] {
			return nil, fmt.Errorf("ai_products[%s]: availability_status must be one of in_stock|preorder|on_demand|unavailable, got %q", p.Ref, availabilityStatus)
		}
		if !validSalesStatus[p.SalesStatus] {
			return nil, fmt.Errorf("ai_products[%s]: sales_status must be \"active\" or \"inactive\", got %q", p.Ref, p.SalesStatus)
		}
		productFacts := toAdditionalFacts(p.AdditionalFacts)
		if err := aiprompt.ValidateFacts(productFacts, []string{"price"}, map[string]string{
			"description": p.Description, "advantages": p.Advantages, "disadvantages": p.Disadvantages,
			"best_for": p.BestFor, "not_for": p.NotFor, "availability_note": p.AvailabilityNote,
			"installation_terms": p.InstallationTerms, "warranty_terms": p.WarrantyTerms,
		}); err != nil {
			return nil, fmt.Errorf("ai_products[%s]: additional_facts: %w", p.Ref, err)
		}
		kb.Products = append(kb.Products, aiprompt.Product{
			Ref: p.Ref, Name: p.Name, Price: p.Price, Description: p.Description,
			Category: p.Category, Brand: p.Brand, Advantages: p.Advantages, Disadvantages: p.Disadvantages,
			BestFor: p.BestFor, NotFor: p.NotFor, AvailabilityStatus: availabilityStatus,
			AvailabilityNote: p.AvailabilityNote, InstallationTerms: p.InstallationTerms, WarrantyTerms: p.WarrantyTerms,
			AdditionalFacts: productFacts, SalesStatus: p.SalesStatus,
			FeaturedImage: p.FeaturedImage, GalleryImages: p.GalleryImages,
			DemoVideos: p.DemoVideos, CertificateDocuments: p.CertificateDocuments,
			GuaranteeDocuments: p.GuaranteeDocuments,
		})
	}

	seenTariffRef := map[string]bool{}
	for i, tf := range fx.AITariffs {
		if strings.TrimSpace(tf.Ref) == "" {
			return nil, fmt.Errorf("ai_tariffs[%d]: ref is required", i)
		}
		if seenTariffRef[tf.Ref] {
			return nil, fmt.Errorf("ai_tariffs: duplicate ref %q", tf.Ref)
		}
		seenTariffRef[tf.Ref] = true
		if strings.TrimSpace(tf.Name) == "" {
			return nil, fmt.Errorf("ai_tariffs[%s]: name is required", tf.Ref)
		}
		if !validSalesStatus[tf.SalesStatus] {
			return nil, fmt.Errorf("ai_tariffs[%s]: sales_status must be \"active\" or \"inactive\", got %q", tf.Ref, tf.SalesStatus)
		}
		if !validPricingType[tf.PricingType] {
			return nil, fmt.Errorf("ai_tariffs[%s]: pricing_type must be one of fixed|percentage|tiered|hybrid, got %q", tf.Ref, tf.PricingType)
		}
		tariffFacts := toAdditionalFacts(tf.AdditionalFacts)
		if err := aiprompt.ValidateFacts(tariffFacts, []string{"price", "fee"}, map[string]string{
			"summary": tf.Summary, "limit_text": tf.LimitText, "advantages": tf.Advantages,
			"disadvantages": tf.Disadvantages, "best_for": tf.BestFor, "not_for": tf.NotFor,
		}); err != nil {
			return nil, fmt.Errorf("ai_tariffs[%s]: additional_facts: %w", tf.Ref, err)
		}
		kb.Tariffs = append(kb.Tariffs, aiprompt.Tariff{
			Ref: tf.Ref, Name: tf.Name, Price: tf.Price, LimitText: tf.LimitText,
			Fee: tf.Fee, Summary: tf.Summary, PricingType: tf.PricingType,
			Advantages: tf.Advantages, Disadvantages: tf.Disadvantages, BestFor: tf.BestFor, NotFor: tf.NotFor,
			AdditionalFacts: tariffFacts, SalesStatus: tf.SalesStatus,
			FeaturedImage: tf.FeaturedImage, PricingImages: tf.PricingImages,
			ExplainerVideos: tf.ExplainerVideos, TermsDocuments: tf.TermsDocuments,
		})
	}

	if c := fx.AIContacts; c != nil {
		kb.Contacts = &aiprompt.Contacts{
			WhatsApp: c.WhatsApp, Email: c.Email, Address: c.Address,
			LegalInformation: c.LegalInformation, CallbackTime: c.CallbackTime,
			WorkingHours: c.WorkingHours, Phone: c.Phone, Website: c.Website,
			Instagram: c.Instagram, ContactCardImage: c.ContactCardImage,
			LocationMapImage: c.LocationMapImage, CompanyLegalDocuments: c.CompanyLegalDocuments,
		}
	}

	if p := fx.AIPolicies; p != nil {
		kb.Policies = &aiprompt.Policies{
			DeliveryCost: p.DeliveryCost, DeliveryInDays: p.DeliveryInDays,
			FreeDeliveryFrom: p.FreeDeliveryFrom, MinOrder: p.MinOrder,
			Prepayment: p.Prepayment, Installment: p.Installment,
			ReturnPeriodInDays: p.ReturnPeriodInDays, Warranty: p.Warranty,
			OutsideZonesNote:        p.OutsideZonesNote,
			CommercePolicyDocuments: p.CommercePolicyDocuments,
		}
	}

	if ti := fx.AITariffInfo; ti != nil {
		tariffInfoFacts := toAdditionalFacts(ti.AdditionalFacts)
		if err := aiprompt.ValidateFacts(tariffInfoFacts, nil, nil); err != nil {
			return nil, fmt.Errorf("ai_tariff_info: additional_facts: %w", err)
		}
		kb.TariffInfo = &aiprompt.TariffInfo{AdditionalFacts: tariffInfoFacts}
	}

	seenZoneRef := map[string]bool{}
	for i, z := range fx.AIDeliveryZones {
		if strings.TrimSpace(z.Ref) == "" {
			return nil, fmt.Errorf("ai_delivery_zones[%d]: ref is required", i)
		}
		if seenZoneRef[z.Ref] {
			return nil, fmt.Errorf("ai_delivery_zones: duplicate ref %q", z.Ref)
		}
		seenZoneRef[z.Ref] = true
		if strings.TrimSpace(z.Name) == "" {
			return nil, fmt.Errorf("ai_delivery_zones[%s]: name is required", z.Ref)
		}
		if !validZoneLevel[z.ZoneLevel] {
			return nil, fmt.Errorf("ai_delivery_zones[%s]: zone_level must be one of city|region|country, got %q", z.Ref, z.ZoneLevel)
		}
		if z.DeliveryAvailable == nil {
			return nil, fmt.Errorf("ai_delivery_zones[%s]: delivery_available is required (NOT NULL, no default)", z.Ref)
		}
		if !validSalesStatus[z.SalesStatus] {
			return nil, fmt.Errorf("ai_delivery_zones[%s]: sales_status must be \"active\" or \"inactive\", got %q", z.Ref, z.SalesStatus)
		}
		kb.DeliveryZones = append(kb.DeliveryZones, aiprompt.DeliveryZone{
			Ref: z.Ref, Name: z.Name, ZoneLevel: z.ZoneLevel, ParentRef: z.ParentRef,
			DeliveryAvailable: bool(*z.DeliveryAvailable), DeliveryCost: z.DeliveryCost,
			DeliveryInDays: z.DeliveryInDays, Notes: z.Notes, SalesStatus: z.SalesStatus,
		})
	}

	seenMaterialID := map[string]bool{}
	for i, m := range fx.KBDMaterials {
		if strings.TrimSpace(m.ID) == "" {
			return nil, fmt.Errorf("kbd_materials[%d]: id is required", i)
		}
		if seenMaterialID[m.ID] {
			return nil, fmt.Errorf("kbd_materials: duplicate id %q", m.ID)
		}
		seenMaterialID[m.ID] = true
		if !validSourceTypes[m.SourceType] {
			return nil, fmt.Errorf("kbd_materials[%s]: source_type must be one of text|url|instruction|file, got %q", m.ID, m.SourceType)
		}
		if !validProcessingStatus[m.ProcessingStatus] {
			return nil, fmt.Errorf("kbd_materials[%s]: processing_status must be one of uploaded|extracting|parsed|built|needs_human|failed, got %q", m.ID, m.ProcessingStatus)
		}
		if !validCustomerVisibility[m.CustomerVisibility] {
			return nil, fmt.Errorf("kbd_materials[%s]: customer_visibility must be empty or one of auto|invisible|visible, got %q", m.ID, m.CustomerVisibility)
		}
		matOrgID := strings.TrimSpace(m.OrganizationID)
		if matOrgID == "" {
			matOrgID = orgID
		}
		kb.Materials = append(kb.Materials, aiprompt.Material{
			ID: m.ID, OrganizationID: matOrgID, SourceType: m.SourceType, SourceRef: m.SourceRef,
			Filename: m.Filename, MimeType: m.MimeType, SizeBytes: m.SizeBytes,
			StorageBackend: m.StorageBackend, StorageKey: m.StorageKey,
			ProcessingStatus: m.ProcessingStatus, CustomerVisibility: m.CustomerVisibility,
		})
	}

	return kb, nil
}

// limitableTables are the fixture list sections a scenario may cap via
// scenario.yaml's limits map. ai_products is the authoritative, tested key
// for this family; ai_topics/ai_tariffs are supported for symmetry.
var limitableTables = map[string]bool{"ai_topics": true, "ai_products": true, "ai_tariffs": true}

// ApplyLimits truncates the given list sections of kb in place, in fixture
// order, to at most N rows each. It mutates kb and also returns it for
// convenient chaining. Referencing a table name outside limitableTables is a
// hard error — a typo in scenario.yaml's limits map must never be silently
// ignored.
func ApplyLimits(kb *aiprompt.KB, limits map[string]int) (*aiprompt.KB, error) {
	for table, n := range limits {
		if !limitableTables[table] {
			return nil, fmt.Errorf("kbfixture: limits references unknown table %q (valid: ai_topics, ai_products, ai_tariffs)", table)
		}
		if n < 0 {
			return nil, fmt.Errorf("kbfixture: limits[%q] must be >= 0, got %d", table, n)
		}
		switch table {
		case "ai_topics":
			if n < len(kb.Topics) {
				kb.Topics = kb.Topics[:n]
			}
		case "ai_products":
			if n < len(kb.Products) {
				kb.Products = kb.Products[:n]
			}
		case "ai_tariffs":
			if n < len(kb.Tariffs) {
				kb.Tariffs = kb.Tariffs[:n]
			}
		}
	}
	return kb, nil
}
