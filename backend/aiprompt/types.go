// Package aiprompt is the canonical customer-response prompt builder.
//
// It is the single component that turns approved live KB rows into the
// model-facing prompt artifacts: the fact-placeholder catalog, the semantic
// media catalog, the media-absence list, the rendered prompt prefix, and the
// strict response-contract validation. DECISIONS.md §"Canonical knowledge-base
// schema" and §"Customer-response JSON contract" are the authoritative spec;
// nothing here may drift from those names. Both the production backend and the
// eval harness use this package, so an eval run exercises the same
// KB-to-prompt path production uses.
//
// The package deliberately has no database or storage dependency: callers load
// rows however they like (SQL in production, YAML fixtures in evals) and pass
// plain structs. Material IDs are strings holding canonical UUID text.
package aiprompt

// Material mirrors the kbd_materials columns the customer path needs. It never
// carries file bytes; storage fields are backend-only metadata used for
// fail-closed validation and (fake or real) storage resolution.
type Material struct {
	ID                 string
	OrganizationID     string
	SourceType         string // text | url | instruction | file
	SourceRef          string
	Filename           string
	MimeType           string
	SizeBytes          int64
	StorageBackend     string
	StorageKey         string
	ProcessingStatus   string // uploaded | extracting | parsed | built | needs_human | failed
	CustomerVisibility string // auto | invisible | visible
}

// Assistant mirrors ai_assistants.
type Assistant struct {
	Persona        string
	Mission        string
	Guardrails     string
	LanguagePolicy string
	ReplyMaxWords  int
}

// Topic mirrors ai_topics.
type Topic struct {
	Slug                string
	Title               string
	BodyMD              string
	FeaturedImage       string
	IllustrationImages  []string
	ExplainerVideos     []string
	ReferenceDocuments  []string
}

// Product mirrors ai_products.
type Product struct {
	Ref                    string
	Name                   string
	Price                  string
	Description            string
	Category               string
	InStock                bool
	SalesStatus            string // active | inactive
	FeaturedImage          string
	GalleryImages          []string
	DemoVideos             []string
	CertificateDocuments   []string
	GuaranteeDocuments     []string
}

// Tariff mirrors ai_tariffs.
type Tariff struct {
	Ref             string
	Name            string
	Price           string
	LimitText       string
	Fee             string
	Summary         string
	PricingType     string // fixed | percentage | tiered | hybrid
	Advantages      string
	Disadvantages   string
	SalesStatus     string // active | inactive
	FeaturedImage   string
	PricingImages   []string
	ExplainerVideos []string
	TermsDocuments  []string
}

// Contacts mirrors the ai_contacts singleton (natural ref "main").
// contact_card_image and location_map_image are singular uuid columns.
type Contacts struct {
	WhatsApp              string
	Email                 string
	Address               string
	LegalInformation      string
	CallbackTime          string
	WorkingHours          string
	Phone                 string
	Website               string
	Instagram             string
	ContactCardImage      string
	LocationMapImage      string
	CompanyLegalDocuments []string
}

// Policies mirrors the ai_policies singleton (natural ref "main").
//
// DeliveryCost and DeliveryInDays are the KB-wide delivery answer used when
// ai_delivery_zones is empty (the zones feature is unused). The moment any
// DeliveryZone row exists, these two fields must be blank — BuildCatalog
// enforces this fail-closed, because a flat delivery answer would contradict
// per-zone pricing. OutsideZonesNote is the seller-authored closed-world
// refusal ("we don't deliver there") for a direction that matches no zone;
// it is required (non-blank) whenever ai_delivery_zones is non-empty, so the
// refusal is always backed by a real token, never invented model prose.
type Policies struct {
	DeliveryCost            string
	DeliveryInDays          string
	FreeDeliveryFrom        string
	MinOrder                string
	Prepayment              string
	Installment             string
	ReturnPeriodInDays      string
	Warranty                string
	OutsideZonesNote        string
	CommercePolicyDocuments []string
}

// DeliveryZone mirrors one ai_delivery_zones row: an explicit, seller-authored
// statement of where the shop delivers (with cost and days) or explicitly
// does not (DeliveryAvailable false). Zones form a shallow containment
// hierarchy via ParentRef (city -> region -> country); resolution to a
// specific zone is left to the model choosing the right FACTS token, not to
// code walking the hierarchy — the hierarchy only has to exist so a fixture
// author can give a country-level fallback without repeating it per city.
//
// A zone with DeliveryAvailable true must carry non-blank DeliveryCost and
// DeliveryInDays; one with DeliveryAvailable false must carry neither —
// BuildCatalog rejects a contradictory row rather than guessing which side
// of the flag is authoritative.
type DeliveryZone struct {
	Ref               string
	Name              string
	ZoneLevel         string // city | region | country
	ParentRef         string // empty for a top-level zone
	DeliveryAvailable bool
	DeliveryCost      string // required iff DeliveryAvailable; blank otherwise
	DeliveryInDays    string // required iff DeliveryAvailable; blank otherwise
	Notes             string
	SalesStatus       string // active | inactive
}

// KB is one organization's complete approved live knowledge base plus the
// kbd_materials registry rows its media columns reference. KB is the input to
// BuildCatalog, which is the only place kbd_materials data is read. KB is
// never passed to RenderPrompt — call PromptInput to derive the model-visible
// projection first.
type KB struct {
	OrganizationID string
	Assistant      *Assistant
	Topics         []Topic
	Products       []Product
	Tariffs        []Tariff
	Contacts       *Contacts
	Policies       *Policies
	DeliveryZones  []DeliveryZone
	Materials      []Material
}

// MaterialByID returns the material with the given id, or nil.
func (kb *KB) MaterialByID(id string) *Material {
	for i := range kb.Materials {
		if kb.Materials[i].ID == id {
			return &kb.Materials[i]
		}
	}
	return nil
}

// PromptInput is exactly what the model-facing prompt renderer may read:
// approved ai_* content. It deliberately has no Materials field — kbd_materials
// is a private lookup table used only by BuildCatalog (to validate and resolve
// media references) and by ResolveSend (to deliver a chosen token); it is
// never a prompt source. Keeping RenderPrompt's input type free of Materials
// makes the no-kbd_materials-in-the-prompt boundary a property of the API
// signature, not just a runtime discipline.
type PromptInput struct {
	Assistant     *Assistant
	Topics        []Topic
	Products      []Product
	Tariffs       []Tariff
	Contacts      *Contacts
	Policies      *Policies
	DeliveryZones []DeliveryZone
}

// PromptInput derives the model-visible projection of kb: approved ai_*
// content only, with no route back to kbd_materials.
func (kb *KB) PromptInput() *PromptInput {
	return &PromptInput{
		Assistant:     kb.Assistant,
		Topics:        kb.Topics,
		Products:      kb.Products,
		Tariffs:       kb.Tariffs,
		Contacts:      kb.Contacts,
		Policies:      kb.Policies,
		DeliveryZones: kb.DeliveryZones,
	}
}
