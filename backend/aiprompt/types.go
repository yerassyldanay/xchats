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
	Keywords            []string
	BodyMD              string
	FeaturedImage       string
	IllustrationImages  []string
	ExplainerVideos     []string
	NarrationAudioFiles []string
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
	AudioDescriptionFiles  []string
	CertificateDocuments   []string
	ManualDocuments        []string
	GuaranteeDocuments     []string
	SpecificationDocuments []string
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
type Policies struct {
	DeliveryCost            string
	DeliveryInDays          string
	FreeDeliveryFrom        string
	MinOrder                string
	Prepayment              string
	Installment             string
	ReturnPeriodInDays      string
	Warranty                string
	CommercePolicyDocuments []string
}

// KB is one organization's complete approved live knowledge base plus the
// material registry rows its media columns reference.
type KB struct {
	OrganizationID string
	Assistant      *Assistant
	Topics         []Topic
	Products       []Product
	Tariffs        []Tariff
	Contacts       *Contacts
	Policies       *Policies
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
