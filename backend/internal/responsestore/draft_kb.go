package responsestore

import (
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
// No media reference (FeaturedImage, GalleryImages, PortfolioImages, ...) is
// EVER carried into the result, for any entity, on purpose: kbstore.Material
// deliberately never exposes its internal StorageBackend/StorageKey blob-store
// locator to callers outside kbstore (see materials.go's own doc comment on
// Material.HasContent), so kb.Materials below is populated with every field
// EXCEPT those two — meaning aiprompt.BuildCatalog's fail-closed
// validateMaterialRef check (catalog.go: "a broken reference is never
// silently treated as empty") would hard-fail catalog construction for the
// ENTIRE org on the very first media reference it hit, not just skip that one
// photo, the moment any entity anywhere in the merged draft view had one
// attached. This was undetected for as long as Specialists/Services (the
// salon vertical's addition) were themselves missing from this projection
// (nothing to attach a photo to that would ever be reached) — restoring them
// surfaced it immediately (a specialist with a portfolio photo). Stripping
// every media field here, uniformly, is what actually delivers this
// function's own long-standing intent: a draft-simulated reply can undersell
// media the real, published KB would offer, but never crash the whole
// simulation over it — text-only facts are genuinely unaffected either way.
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
		})
	}
	for _, p := range dv.Products {
		kb.Products = append(kb.Products, aiprompt.Product{
			Ref: p.Ref, Name: p.Name, Price: p.Price, Description: p.Description,
			Category: p.Category, Brand: p.Brand, Advantages: p.Advantages, Disadvantages: p.Disadvantages,
			BestFor: p.BestFor, NotFor: p.NotFor,
			AvailabilityStatus: p.AvailabilityStatus, AvailabilityNote: p.AvailabilityNote,
			InstallationTerms: p.InstallationTerms, WarrantyTerms: p.WarrantyTerms,
			AdditionalFacts: p.AdditionalFacts, SalesStatus: p.SalesStatus,
		})
	}
	for _, t := range dv.Tariffs {
		kb.Tariffs = append(kb.Tariffs, aiprompt.Tariff{
			Ref: t.Ref, Name: t.Name, Price: t.Price, LimitText: t.LimitText, Fee: t.Fee,
			Summary: t.Summary, PricingType: t.PricingType, Advantages: t.Advantages,
			Disadvantages: t.Disadvantages, BestFor: t.BestFor, NotFor: t.NotFor,
			AdditionalFacts: t.AdditionalFacts, SalesStatus: t.SalesStatus,
		})
	}
	if len(dv.TariffInfo) > 0 {
		ti := dv.TariffInfo[0]
		kb.TariffInfo = &aiprompt.TariffInfo{AdditionalFacts: ti.AdditionalFacts}
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
			BookingURL: c.BookingURL,
			Schedule:   c.Schedule,
		}
	}
	if len(dv.Policies) > 0 {
		p := dv.Policies[0]
		kb.Policies = &aiprompt.Policies{
			DeliveryCost: p.DeliveryCost, DeliveryInDays: p.DeliveryInDays,
			FreeDeliveryFrom: p.FreeDeliveryFrom, MinOrder: p.MinOrder, Prepayment: p.Prepayment,
			Installment: p.Installment, ReturnPeriodInDays: p.ReturnPeriodInDays, Warranty: p.Warranty,
			OutsideZonesNote: p.OutsideZonesNote,
		}
	}
	// Specialists/Services (the salon vertical's addition, PLAN.md) were
	// missing from this projection entirely: isSalonOrganization(kb) — the
	// gate every salon-only prompt/contract rule keys off (schedule.go) —
	// reads kb.Specialists/kb.Services directly, so a draft-simulated
	// message for an org that HAS staged salon content still silently fell
	// back to the plain shop-kb frame with zero specialists/services facts
	// available, same as an org with no salon data at all.
	for _, sp := range dv.Specialists {
		kb.Specialists = append(kb.Specialists, aiprompt.Specialist{
			Ref: sp.Ref, FullName: sp.FullName, Title: sp.Title, Experience: sp.Experience,
			Schedule: sp.Schedule, BookingURL: sp.BookingURL, SalesStatus: sp.SalesStatus,
		})
	}
	for _, sv := range dv.Services {
		kb.Services = append(kb.Services, aiprompt.Service{
			Ref: sv.Ref, ParentRef: sv.ParentRef, ServiceType: sv.ServiceType, Category: sv.Category,
			Name: sv.Name, Price: sv.Price, Duration: sv.Duration, Description: sv.Description,
			SpecialistRefs: sv.SpecialistRefs, SalesStatus: sv.SalesStatus,
		})
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
