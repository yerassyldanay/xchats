package kbstore

import "github.com/google/uuid"

// media_apply.go is the media-column half of every *Changes upsert (see
// mcp_write.go), factored out so the MCP patch-based lineage there and the
// typed draft-lane lineage (draft.go's *Input structs) apply the exact same
// nil-means-absent semantics from one place instead of two copies drifting
// apart. Each apply*Media function is a straight lift of what used to be
// inlined in its MCPUpsert* counterpart — same order, same helpers
// (nonNilUUIDs, mergeRefs, singularRef) — so this is a pure refactor with no
// behavior change.
//
// Deliberately NOT embedded into the existing *Changes structs (ProductChanges
// etc.): Go's composite-literal syntax cannot set a promoted field by name
// (`ProductChanges{FeaturedImage: x}` would stop compiling), and mcp_test.go/
// mcp_attach_test.go construct those structs that way throughout. Each
// MCPUpsert* instead builds the matching *Media value from its own *Changes
// fields at the call site — the struct SHAPES stay independent, only the
// apply LOGIC is shared.
//
// Pointer convention (mirrored by draft.go's *Input.Media fields in the
// typed lineage): a nullable singular reference is **uuid.UUID — outer nil
// means "no pending edit, leave the current value," a non-nil pointer to a
// nil *uuid.UUID means "explicit clear." A plural reference is *[]uuid.UUID
// — nil means "no pending edit," a non-nil pointer to an empty slice means
// "explicit clear everything" (detach-all).

// TopicMedia carries topic media edits shared by TopicChanges (MCP) and
// TopicInput.Media (draft lane).
type TopicMedia struct {
	FeaturedImage                                           **uuid.UUID
	IllustrationImages, ExplainerVideos, ReferenceDocuments *[]uuid.UUID
}

// applyTopicMedia merges m onto cur in place and returns the {field: [ids]}
// map validateMediaRefs should check.
func applyTopicMedia(cur *DraftTopic, m TopicMedia) map[string][]uuid.UUID {
	refs := map[string][]uuid.UUID{}
	if m.FeaturedImage != nil {
		cur.FeaturedImage = *m.FeaturedImage
		refs = mergeRefs(refs, singularRef("featured_image", *m.FeaturedImage))
	}
	if m.IllustrationImages != nil {
		cur.IllustrationImages = nonNilUUIDs(*m.IllustrationImages)
		refs = mergeRefs(refs, map[string][]uuid.UUID{"illustration_images": cur.IllustrationImages})
	}
	if m.ExplainerVideos != nil {
		cur.ExplainerVideos = nonNilUUIDs(*m.ExplainerVideos)
		refs = mergeRefs(refs, map[string][]uuid.UUID{"explainer_videos": cur.ExplainerVideos})
	}
	if m.ReferenceDocuments != nil {
		cur.ReferenceDocuments = nonNilUUIDs(*m.ReferenceDocuments)
		refs = mergeRefs(refs, map[string][]uuid.UUID{"reference_documents": cur.ReferenceDocuments})
	}
	return refs
}

// ProductMedia carries product media edits shared by ProductChanges (MCP)
// and ProductInput.Media (draft lane).
type ProductMedia struct {
	FeaturedImage                                                       **uuid.UUID
	GalleryImages, DemoVideos, CertificateDocuments, GuaranteeDocuments *[]uuid.UUID
}

func applyProductMedia(cur *DraftProduct, m ProductMedia) map[string][]uuid.UUID {
	refs := map[string][]uuid.UUID{}
	if m.FeaturedImage != nil {
		cur.FeaturedImage = *m.FeaturedImage
		refs = mergeRefs(refs, singularRef("featured_image", *m.FeaturedImage))
	}
	applyImageSet := func(field string, patch *[]uuid.UUID, dst *[]uuid.UUID) {
		if patch != nil {
			*dst = nonNilUUIDs(*patch)
			refs = mergeRefs(refs, map[string][]uuid.UUID{field: *dst})
		}
	}
	applyImageSet("gallery_images", m.GalleryImages, &cur.GalleryImages)
	applyImageSet("demo_videos", m.DemoVideos, &cur.DemoVideos)
	applyImageSet("certificate_documents", m.CertificateDocuments, &cur.CertificateDocuments)
	applyImageSet("guarantee_documents", m.GuaranteeDocuments, &cur.GuaranteeDocuments)
	return refs
}

// TariffMedia carries tariff media edits shared by TariffChanges (MCP) and
// TariffInput.Media (draft lane).
type TariffMedia struct {
	FeaturedImage                                  **uuid.UUID
	PricingImages, ExplainerVideos, TermsDocuments *[]uuid.UUID
}

func applyTariffMedia(cur *DraftTariff, m TariffMedia) map[string][]uuid.UUID {
	refs := map[string][]uuid.UUID{}
	if m.FeaturedImage != nil {
		cur.FeaturedImage = *m.FeaturedImage
		refs = mergeRefs(refs, singularRef("featured_image", *m.FeaturedImage))
	}
	if m.PricingImages != nil {
		cur.PricingImages = nonNilUUIDs(*m.PricingImages)
		refs = mergeRefs(refs, map[string][]uuid.UUID{"pricing_images": cur.PricingImages})
	}
	if m.ExplainerVideos != nil {
		cur.ExplainerVideos = nonNilUUIDs(*m.ExplainerVideos)
		refs = mergeRefs(refs, map[string][]uuid.UUID{"explainer_videos": cur.ExplainerVideos})
	}
	if m.TermsDocuments != nil {
		cur.TermsDocuments = nonNilUUIDs(*m.TermsDocuments)
		refs = mergeRefs(refs, map[string][]uuid.UUID{"terms_documents": cur.TermsDocuments})
	}
	return refs
}

// ContactsMedia carries contacts media edits shared by ContactsChanges (MCP)
// and ContactPatch.Media (draft lane).
type ContactsMedia struct {
	ContactCardImage, LocationMapImage **uuid.UUID
	CompanyLegalDocuments              *[]uuid.UUID
}

func applyContactsMedia(cur *DraftContact, m ContactsMedia) map[string][]uuid.UUID {
	refs := map[string][]uuid.UUID{}
	if m.ContactCardImage != nil {
		cur.ContactCardImage = *m.ContactCardImage
		refs = mergeRefs(refs, singularRef("contact_card_image", *m.ContactCardImage))
	}
	if m.LocationMapImage != nil {
		cur.LocationMapImage = *m.LocationMapImage
		refs = mergeRefs(refs, singularRef("location_map_image", *m.LocationMapImage))
	}
	if m.CompanyLegalDocuments != nil {
		cur.CompanyLegalDocuments = nonNilUUIDs(*m.CompanyLegalDocuments)
		refs = mergeRefs(refs, map[string][]uuid.UUID{"company_legal_documents": cur.CompanyLegalDocuments})
	}
	return refs
}

// PoliciesMedia carries policies media edits shared by PoliciesChanges (MCP)
// and PolicyPatch.Media (draft lane).
type PoliciesMedia struct {
	CommercePolicyDocuments *[]uuid.UUID
}

func applyPoliciesMedia(cur *DraftPolicy, m PoliciesMedia) map[string][]uuid.UUID {
	refs := map[string][]uuid.UUID{}
	if m.CommercePolicyDocuments != nil {
		cur.CommercePolicyDocuments = nonNilUUIDs(*m.CommercePolicyDocuments)
		refs = mergeRefs(refs, map[string][]uuid.UUID{"commerce_policy_documents": cur.CommercePolicyDocuments})
	}
	return refs
}
