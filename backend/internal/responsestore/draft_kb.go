package responsestore

import (
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/aiprompt"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// BuildKBFromDraftView adapts a kbstore.DraftView — live rows with pending
// Черновик changes already overlaid, i.e. exactly what the org's knowledge
// base would look like the instant every pending change was published (see
// kbstore.Store.Draft's own doc comment) — into the SAME aiprompt.KB shape
// KnowledgeBaseRepo.Load builds from the live tables directly. This is what
// lets the Simulator answer against staged draft content (KB-02) with no
// second prompt-building code path: BuildCatalog/RenderPrompt never know the
// difference between this and a live-loaded KB.
//
// Materials carry no StorageBackend/StorageKey here — kbstore.Material
// deliberately never exposes that internal blob-store locator to callers
// outside kbstore (see materials.go's own doc comment on Material.HasContent)
// — so every media reference is treated as unresolvable by BuildCatalog's own
// fail-closed StorageBackend/StorageKey check and left out of the catalog. A
// draft-simulated reply can therefore undersell media the real, published KB
// would actually offer; text-only facts are unaffected either way.
func BuildKBFromDraftView(orgID string, dv *kbstore.DraftView) *aiprompt.KB {
	kb := &aiprompt.KB{
		OrganizationID: orgID,
		Assistant: &aiprompt.Assistant{
			Persona:        dv.Config.Persona,
			Mission:        dv.Config.Mission,
			Guardrails:     dv.Config.Guardrails,
			LanguagePolicy: dv.Config.LanguagePolicy,
			ReplyMaxWords:  dv.Config.ReplyMaxWords,
		},
	}
	for _, t := range dv.Topics {
		kb.Topics = append(kb.Topics, aiprompt.Topic{
			Slug: t.Slug, Title: t.Title, BodyMD: t.BodyMD,
			FeaturedImage:      uuidPtrString(t.FeaturedImage),
			IllustrationImages: mediaArray(t.IllustrationImages),
			ExplainerVideos:    mediaArray(t.ExplainerVideos),
			ReferenceDocuments: mediaArray(t.ReferenceDocuments),
		})
	}
	for _, p := range dv.Products {
		kb.Products = append(kb.Products, aiprompt.Product{
			Ref: p.Ref, Name: p.Name, Price: p.Price, Description: p.Description,
			Category: p.Category, InStock: p.InStock, SalesStatus: p.SalesStatus,
			FeaturedImage:        uuidPtrString(p.FeaturedImage),
			GalleryImages:        mediaArray(p.GalleryImages),
			DemoVideos:           mediaArray(p.DemoVideos),
			CertificateDocuments: mediaArray(p.CertificateDocuments),
			GuaranteeDocuments:   mediaArray(p.GuaranteeDocuments),
		})
	}
	for _, t := range dv.Tariffs {
		kb.Tariffs = append(kb.Tariffs, aiprompt.Tariff{
			Ref: t.Ref, Name: t.Name, Price: t.Price, LimitText: t.LimitText, Fee: t.Fee,
			Summary: t.Summary, PricingType: t.PricingType, Advantages: t.Advantages,
			Disadvantages: t.Disadvantages, SalesStatus: t.SalesStatus,
			FeaturedImage:   uuidPtrString(t.FeaturedImage),
			PricingImages:   mediaArray(t.PricingImages),
			ExplainerVideos: mediaArray(t.ExplainerVideos),
			TermsDocuments:  mediaArray(t.TermsDocuments),
		})
	}
	for _, z := range dv.Zones {
		kb.DeliveryZones = append(kb.DeliveryZones, aiprompt.DeliveryZone{
			Ref: z.Ref, Name: z.Name, ZoneLevel: z.ZoneLevel, ParentRef: z.ParentRef,
			DeliveryAvailable: z.DeliveryAvailable, DeliveryCost: z.DeliveryCost,
			DeliveryInDays: z.DeliveryInDays, Notes: z.Notes, SalesStatus: z.SalesStatus,
		})
	}
	// Contacts/Policies are true singletons carried as a 0-or-1-row slice —
	// same shape the frontend reads (pg.live?.contacts[0]).
	if len(dv.Contacts) > 0 {
		c := dv.Contacts[0]
		kb.Contacts = &aiprompt.Contacts{
			WhatsApp: c.WhatsApp, Email: c.Email, Address: c.Address,
			LegalInformation: c.LegalInformation, CallbackTime: c.CallbackTime,
			WorkingHours: c.WorkingHours, Phone: c.Phone, Website: c.Website, Instagram: c.Instagram,
			ContactCardImage:      uuidPtrString(c.ContactCardImage),
			LocationMapImage:      uuidPtrString(c.LocationMapImage),
			CompanyLegalDocuments: mediaArray(c.CompanyLegalDocuments),
		}
	}
	if len(dv.Policies) > 0 {
		p := dv.Policies[0]
		kb.Policies = &aiprompt.Policies{
			DeliveryCost: p.DeliveryCost, DeliveryInDays: p.DeliveryInDays,
			FreeDeliveryFrom: p.FreeDeliveryFrom, MinOrder: p.MinOrder, Prepayment: p.Prepayment,
			Installment: p.Installment, ReturnPeriodInDays: p.ReturnPeriodInDays, Warranty: p.Warranty,
			OutsideZonesNote:        p.OutsideZonesNote,
			CommercePolicyDocuments: mediaArray(p.CommercePolicyDocuments),
		}
	}
	for _, m := range dv.Materials {
		kb.Materials = append(kb.Materials, aiprompt.Material{
			ID: m.ID.String(), OrganizationID: orgID, SourceType: m.SourceType, SourceRef: m.SourceRef,
			Filename: m.Filename, MimeType: m.MimeType, SizeBytes: m.SizeBytes,
			ProcessingStatus: m.ProcessingStatus, CustomerVisibility: m.CustomerVisibility,
		})
	}
	return kb
}

func uuidPtrString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
