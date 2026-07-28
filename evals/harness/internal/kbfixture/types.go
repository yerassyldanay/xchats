// Package kbfixture loads Russian, database-schema-shaped knowledge-base
// fixtures for the shop-kb-v1 eval family and converts them 1:1 into
// backend/aiprompt types. This is deliberately NOT a generic eval fixture
// format: every top-level key is an exact DECISIONS.md / plan/database-schema.md
// table name, and every nested key is an exact business column name. There is
// no generic values/media/files/data bag anywhere in this package — that is
// the whole point of this loader versus the harness's older fact_tables YAML.
//
// # Fixture projection
//
// A fixture file is a documented projection of the live DB rows it stands in
// for, not a byte-for-byte dump:
//
//   - Every business column defined in DECISIONS.md §"Canonical knowledge-base
//     schema" is preserved under its exact column name (no aliases: e.g.
//     "in_stock" not "availability", "working_hours" not "work_hours",
//     "delivery_in_days" not "delivery_time").
//   - Infrastructure columns the prompt path never reads are omitted entirely
//     from ai_* rows: no "id", "created_at", or "updated_at". Business rows are
//     addressed by their natural key (ref/slug, or the fixed "main" singleton)
//     exactly as the prompt builder addresses them — a fixture-only surrogate
//     ID would be actively misleading here, not just unused.
//   - kbd_materials rows keep "id" (it is the value every media column
//     references) but drop the extraction/authoring-only columns that never
//     reach the customer-response path: no "sha256_checksum", "extracted_text",
//     "visual_summary", "transcript_text", "extraction_metadata",
//     "operator_note", "source_text", "created_at", "updated_at". Those belong
//     to the playground/builder path (plan/playground.md), not this evaluation
//     boundary.
//   - "organization_id" is fixture-level (one value, applied to every row and
//     to every kbd_materials entry that does not set its own) rather than
//     repeated on every row, because every fixture in this family models
//     exactly one organization. A kbd_materials entry MAY still set its own
//     organization_id explicitly — this exists solely so a negative fixture can
//     construct a cross-organization material for the "wrong organization"
//     fail-closed test; every positive fixture leaves it unset.
//
// Unknown keys anywhere in the document — including every generic alias this
// family deliberately does not support, such as "values", "media", "files",
// "availability", "delivery_time", or "work_hours" — are decode errors, not
// silently-ignored fields. See load.go for the strict-decode mechanics.
package kbfixture

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Fixture is the top-level schema-shaped YAML document. Its keys are exactly
// organization_id, ai_assistants, ai_topics, ai_products, ai_tariffs,
// ai_contacts, ai_policies, ai_delivery_zones, and kbd_materials — nothing
// else.
type Fixture struct {
	OrganizationID  string           `yaml:"organization_id"`
	AIAssistants    *AIAssistant     `yaml:"ai_assistants"`
	AITopics        []AITopic        `yaml:"ai_topics"`
	AIProducts      []AIProduct      `yaml:"ai_products"`
	AITariffs       []AITariff       `yaml:"ai_tariffs"`
	AIContacts      *AIContacts      `yaml:"ai_contacts"`
	AIPolicies      *AIPolicies      `yaml:"ai_policies"`
	AIDeliveryZones []AIDeliveryZone `yaml:"ai_delivery_zones"`
	KBDMaterials    []KBDMaterial    `yaml:"kbd_materials"`
}

// AIAssistant mirrors ai_assistants (singleton per organization_id).
type AIAssistant struct {
	Persona        string `yaml:"persona"`
	Mission        string `yaml:"mission"`
	Guardrails     string `yaml:"guardrails"`
	LanguagePolicy string `yaml:"language_policy"`
	ReplyMaxWords  int    `yaml:"reply_max_words"`
}

// AITopic mirrors one ai_topics row.
type AITopic struct {
	Slug                string   `yaml:"slug"`
	Title               string   `yaml:"title"`
	BodyMD              string   `yaml:"body_md"`
	FeaturedImage       string   `yaml:"featured_image"`
	IllustrationImages  []string `yaml:"illustration_images"`
	ExplainerVideos     []string `yaml:"explainer_videos"`
	NarrationAudioFiles []string `yaml:"narration_audio_files"`
	ReferenceDocuments  []string `yaml:"reference_documents"`
}

// StrictBool decodes only the literal YAML booleans true/false — never the
// legacy YAML 1.1 words (yes/no/on/off, in any letter case, quoted or not)
// that gopkg.in/yaml.v3 otherwise still accepts as a backward-compatibility
// fallback when unmarshaling into a plain Go bool, and never a quoted
// "true"/"false" string either (quoting forces a !!str node tag). See
// UnmarshalYAML below for exactly what is and is not accepted; load_test.go's
// TestDecode_InStockStrictBoolean pins the full accept/reject matrix this
// exists to guarantee.
type StrictBool bool

// UnmarshalYAML accepts only an unquoted, exact-lowercase true/false scalar
// (YAML tag !!bool with value "true" or "false"). Everything else — yes, no,
// on, off, Yes, TRUE, "true" (quoted), 1, or any other representation — is a
// decode error naming the offending value.
func (b *StrictBool) UnmarshalYAML(value *yaml.Node) error {
	if value.Tag != "!!bool" || (value.Value != "true" && value.Value != "false") {
		return fmt.Errorf("must be exactly true or false, got %q (yes/no/on/off and quoted forms are not accepted)", value.Value)
	}
	*b = StrictBool(value.Value == "true")
	return nil
}

// AIProduct mirrors one ai_products row. InStock is a *StrictBool, not bool:
// the pointer lets the loader tell "key omitted" (nil — rejected, in_stock is
// NOT NULL with no default in the schema) apart from "explicitly false", and
// StrictBool itself rejects every non-canonical boolean spelling.
type AIProduct struct {
	Ref                    string      `yaml:"ref"`
	Name                   string      `yaml:"name"`
	Price                  string      `yaml:"price"`
	Description            string      `yaml:"description"`
	Category               string      `yaml:"category"`
	InStock                *StrictBool `yaml:"in_stock"`
	SalesStatus            string      `yaml:"sales_status"`
	FeaturedImage          string      `yaml:"featured_image"`
	GalleryImages          []string    `yaml:"gallery_images"`
	DemoVideos             []string    `yaml:"demo_videos"`
	AudioDescriptionFiles  []string    `yaml:"audio_description_files"`
	CertificateDocuments   []string    `yaml:"certificate_documents"`
	ManualDocuments        []string    `yaml:"manual_documents"`
	GuaranteeDocuments     []string    `yaml:"guarantee_documents"`
	SpecificationDocuments []string    `yaml:"specification_documents"`
}

// AITariff mirrors one ai_tariffs row.
type AITariff struct {
	Ref             string   `yaml:"ref"`
	Name            string   `yaml:"name"`
	Price           string   `yaml:"price"`
	LimitText       string   `yaml:"limit_text"`
	Fee             string   `yaml:"fee"`
	Summary         string   `yaml:"summary"`
	PricingType     string   `yaml:"pricing_type"`
	Advantages      string   `yaml:"advantages"`
	Disadvantages   string   `yaml:"disadvantages"`
	SalesStatus     string   `yaml:"sales_status"`
	FeaturedImage   string   `yaml:"featured_image"`
	PricingImages   []string `yaml:"pricing_images"`
	ExplainerVideos []string `yaml:"explainer_videos"`
	TermsDocuments  []string `yaml:"terms_documents"`
}

// AIContacts mirrors the ai_contacts singleton (natural ref "main").
type AIContacts struct {
	WhatsApp              string   `yaml:"whatsapp"`
	Email                 string   `yaml:"email"`
	Address               string   `yaml:"address"`
	LegalInformation      string   `yaml:"legal_information"`
	CallbackTime          string   `yaml:"callback_time"`
	WorkingHours          string   `yaml:"working_hours"`
	Phone                 string   `yaml:"phone"`
	Website               string   `yaml:"website"`
	Instagram             string   `yaml:"instagram"`
	ContactCardImage      string   `yaml:"contact_card_image"`
	LocationMapImage      string   `yaml:"location_map_image"`
	CompanyLegalDocuments []string `yaml:"company_legal_documents"`
}

// AIPolicies mirrors the ai_policies singleton (natural ref "main").
//
// DeliveryCost/DeliveryInDays are the KB-wide delivery answer used only when
// ai_delivery_zones is empty; they must be blank once any zone row exists
// (aiprompt.BuildCatalog enforces this — see AIDeliveryZone). OutsideZonesNote
// is the seller-authored closed-world refusal for a direction matching no
// zone; required (non-blank) whenever ai_delivery_zones is non-empty.
type AIPolicies struct {
	DeliveryCost            string   `yaml:"delivery_cost"`
	DeliveryInDays          string   `yaml:"delivery_in_days"`
	FreeDeliveryFrom        string   `yaml:"free_delivery_from"`
	MinOrder                string   `yaml:"min_order"`
	Prepayment              string   `yaml:"prepayment"`
	Installment             string   `yaml:"installment"`
	ReturnPeriodInDays      string   `yaml:"return_period_in_days"`
	Warranty                string   `yaml:"warranty"`
	OutsideZonesNote        string   `yaml:"outside_zones_note"`
	CommercePolicyDocuments []string `yaml:"commerce_policy_documents"`
}

// AIDeliveryZone mirrors one ai_delivery_zones row — see
// aiprompt.DeliveryZone's doc comment for the full containment-hierarchy and
// fail-closed-consistency model this loader converts 1:1 into.
// DeliveryAvailable is a *StrictBool for the same reason AIProduct.InStock
// is: the pointer distinguishes "key omitted" (rejected — NOT NULL, no
// default) from "explicitly false", and StrictBool itself rejects every
// non-canonical boolean spelling.
type AIDeliveryZone struct {
	Ref               string      `yaml:"ref"`
	Name              string      `yaml:"name"`
	ZoneLevel         string      `yaml:"zone_level"`
	ParentRef         string      `yaml:"parent_ref"`
	DeliveryAvailable *StrictBool `yaml:"delivery_available"`
	DeliveryCost      string      `yaml:"delivery_cost"`
	DeliveryInDays    string      `yaml:"delivery_in_days"`
	Notes             string      `yaml:"notes"`
	SalesStatus       string      `yaml:"sales_status"`
}

// KBDMaterial mirrors the subset of kbd_materials columns the customer
// prompt/response path needs (see the package doc comment for exactly which
// columns are intentionally omitted). OrganizationID is optional and defaults
// to Fixture.OrganizationID; set it explicitly only to construct a
// cross-organization negative-fixture material.
type KBDMaterial struct {
	ID                 string `yaml:"id"`
	OrganizationID     string `yaml:"organization_id"`
	SourceType         string `yaml:"source_type"`
	SourceRef          string `yaml:"source_ref"`
	Filename           string `yaml:"filename"`
	MimeType           string `yaml:"mime_type"`
	SizeBytes          int64  `yaml:"size_bytes"`
	StorageBackend     string `yaml:"storage_backend"`
	StorageKey         string `yaml:"storage_key"`
	ProcessingStatus   string `yaml:"processing_status"`
	CustomerVisibility string `yaml:"customer_visibility"`
}
