package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// --- /kb/* — the live-only editor surface ("База знаний" / /knowledge-base) --
//
// Unlike /playground/draft/*, every write here lands directly in the live ai_
// tables: there is no pending/blob step and no approve. This keeps live edits
// from ever mixing with — or being confused with — Playground draft work (see
// plan "Playground redesign": /playground owns the draft workflow end to end;
// /knowledge-base only ever shows and edits the final, live tables).

// kbWrite is the /kb/* write preamble: KB ready + org resolved. There is no
// optimistic-concurrency token here (unlike pgWrite/the draft blob) — a live
// write is immediately final, so there is nothing to stale-check against.
func (s *Server) kbWrite(c *gin.Context) (uuid.UUID, bool) {
	if !s.kbReady(c) {
		return uuid.Nil, false
	}
	return s.pgOrg(c)
}

// invalidateKBCache drops orgID's cached prompt-facing KB (see CachedKBRepo)
// so the NEXT Load — the next customer reply, or the next GET /kb/prompt —
// re-reads Postgres instead of serving a stale build. Called after every
// write that changes what the response engine reads: /kb/* live writes
// (kbLiveChanged) AND Playground approve (handlePlaygroundApprove/
// ApproveEntity), since approve also materializes rows into the same live
// tables the cache is built from.
func (s *Server) invalidateKBCache(orgID uuid.UUID) {
	if s.kbInvalidator != nil {
		s.kbInvalidator.Invalidate(orgID)
	}
}

// kbLiveChanged is the /kb/* write epilogue: invalidate the response engine's
// cached prompt KB, broadcast, and return the refreshed live view.
func (s *Server) kbLiveChanged(c *gin.Context, orgID uuid.UUID) {
	// The response engine (backend/response, the production reply path) reads
	// the KB through responsestore.CachedKBRepo, a per-org cache with its own
	// 60s TTL backstop — this synchronous Invalidate is what makes a live edit
	// visible to the NEXT customer reply (and to GET /kb/prompt) immediately
	// instead of up to a minute later.
	s.invalidateKBCache(orgID)
	view, err := s.kb.LiveView(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.hub.Broadcast("kb.row.changed", gin.H{})
	ok(c, view)
}

func (s *Server) handleKBGet(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	view, err := s.kb.LiveView(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, view)
}

// --- topics ------------------------------------------------------------------

func (s *Server) handleKBUpsertTopic(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req topicReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Slug == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "slug required")
		return
	}
	illustrations, ok := s.validateMediaList(c, "illustration_images", req.IllustrationImages)
	if !ok {
		return
	}
	videos, ok := s.validateMediaList(c, "explainer_videos", req.ExplainerVideos)
	if !ok {
		return
	}
	docs, ok := s.validateMediaList(c, "reference_documents", req.ReferenceDocuments)
	if !ok {
		return
	}
	if err := s.kb.PutLiveTopic(ctx(c), orgID, currentUser(c).ID, kbstore.TopicInput{
		Slug: req.Slug, Title: req.Title, BodyMD: req.BodyMD,
		Media: kbstore.TopicMedia{
			FeaturedImage:      req.FeaturedImage.ptr(),
			IllustrationImages: illustrations,
			ExplainerVideos:    videos,
			ReferenceDocuments: docs,
		},
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

func (s *Server) handleKBDeleteTopic(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteLiveTopic(ctx(c), orgID, currentUser(c).ID, c.Param("slug")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

// --- typed facts: tariffs / products / contacts -------------------------------

func (s *Server) handleKBUpsertTariff(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req tariffReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Ref == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "ref required")
		return
	}
	pricingImages, ok := s.validateMediaList(c, "pricing_images", req.PricingImages)
	if !ok {
		return
	}
	videos, ok := s.validateMediaList(c, "explainer_videos", req.ExplainerVideos)
	if !ok {
		return
	}
	terms, ok := s.validateMediaList(c, "terms_documents", req.TermsDocuments)
	if !ok {
		return
	}
	if err := s.kb.PutLiveTariff(ctx(c), orgID, currentUser(c).ID, kbstore.TariffInput{
		Ref: req.Ref, Name: req.Name, Price: req.Price, LimitText: req.LimitText, Fee: req.Fee,
		Summary: req.Summary, PricingType: req.PricingType, Advantages: req.Advantages, Disadvantages: req.Disadvantages,
		BestFor: req.BestFor, NotFor: req.NotFor, AdditionalFacts: req.AdditionalFacts,
		SalesStatus: req.SalesStatus,
		Media: kbstore.TariffMedia{
			FeaturedImage:   req.FeaturedImage.ptr(),
			PricingImages:   pricingImages,
			ExplainerVideos: videos,
			TermsDocuments:  terms,
		},
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

func (s *Server) handleKBDeleteTariff(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteLiveTariff(ctx(c), orgID, currentUser(c).ID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

func (s *Server) handleKBUpsertProduct(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Ref == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "ref required")
		return
	}
	gallery, ok := s.validateMediaList(c, "gallery_images", req.GalleryImages)
	if !ok {
		return
	}
	demo, ok := s.validateMediaList(c, "demo_videos", req.DemoVideos)
	if !ok {
		return
	}
	certs, ok := s.validateMediaList(c, "certificate_documents", req.CertificateDocuments)
	if !ok {
		return
	}
	guarantee, ok := s.validateMediaList(c, "guarantee_documents", req.GuaranteeDocuments)
	if !ok {
		return
	}
	if err := s.kb.PutLiveProduct(ctx(c), orgID, currentUser(c).ID, kbstore.ProductInput{
		Ref: req.Ref, Name: req.Name, Price: req.Price,
		Description: req.Description, Category: req.Category,
		Brand: req.Brand, Advantages: req.Advantages, Disadvantages: req.Disadvantages,
		BestFor: req.BestFor, NotFor: req.NotFor,
		AvailabilityNote: req.AvailabilityNote, InstallationTerms: req.InstallationTerms, WarrantyTerms: req.WarrantyTerms,
		AdditionalFacts:    req.AdditionalFacts,
		AvailabilityStatus: req.AvailabilityStatus, SalesStatus: req.SalesStatus,
		Media: kbstore.ProductMedia{
			FeaturedImage:        req.FeaturedImage.ptr(),
			GalleryImages:        gallery,
			DemoVideos:           demo,
			CertificateDocuments: certs,
			GuaranteeDocuments:   guarantee,
		},
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

func (s *Server) handleKBDeleteProduct(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteLiveProduct(ctx(c), orgID, currentUser(c).ID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

func (s *Server) handleKBPatchContacts(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req contactsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad contacts")
		return
	}
	legalDocs, ok := s.validateMediaList(c, "company_legal_documents", req.CompanyLegalDocuments)
	if !ok {
		return
	}
	if err := s.kb.PatchLiveContacts(ctx(c), orgID, currentUser(c).ID, kbstore.ContactPatch{
		WhatsApp: req.WhatsApp, Email: req.Email, Address: req.Address,
		LegalInformation: req.LegalInformation, CallbackTime: req.CallbackTime,
		WorkingHours: req.WorkingHours, Phone: req.Phone, Website: req.Website, Instagram: req.Instagram,
		BookingURL: req.BookingURL, Schedule: req.Schedule,
		Media: kbstore.ContactsMedia{
			ContactCardImage:      req.ContactCardImage.ptr(),
			LocationMapImage:      req.LocationMapImage.ptr(),
			CompanyLegalDocuments: legalDocs,
		},
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

// --- typed facts: commerce policies -------------------------------------------

func (s *Server) handleKBPatchPolicies(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req policiesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad policies")
		return
	}
	docs, ok := s.validateMediaList(c, "commerce_policy_documents", req.CommercePolicyDocuments)
	if !ok {
		return
	}
	if err := s.kb.PatchLivePolicies(ctx(c), orgID, currentUser(c).ID, kbstore.PolicyPatch{
		DeliveryCost: req.DeliveryCost, DeliveryInDays: req.DeliveryInDays,
		FreeDeliveryFrom: req.FreeDeliveryFrom, MinOrder: req.MinOrder, Prepayment: req.Prepayment,
		Installment: req.Installment, ReturnPeriodInDays: req.ReturnPeriodInDays, Warranty: req.Warranty,
		OutsideZonesNote: req.OutsideZonesNote,
		Media: kbstore.PoliciesMedia{
			CommercePolicyDocuments: docs,
		},
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

// --- tariff info (organization-wide tariff facts) ----------------------------

func (s *Server) handleKBPatchTariffInfo(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req tariffInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad tariff info")
		return
	}
	if err := s.kb.PatchLiveTariffInfo(ctx(c), orgID, currentUser(c).ID, kbstore.TariffInfoPatch{
		AdditionalFacts: req.AdditionalFacts,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

// --- delivery zones ------------------------------------------------------------

func (s *Server) handleKBUpsertZone(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req zoneReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Ref == "" || req.ZoneLevel == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "ref and zone_level required")
		return
	}
	if err := s.kb.PutLiveZone(ctx(c), orgID, currentUser(c).ID, kbstore.ZoneInput{
		Ref: req.Ref, Name: req.Name, ZoneLevel: req.ZoneLevel, ParentRef: req.ParentRef,
		DeliveryAvailable: req.DeliveryAvailable, DeliveryCost: req.DeliveryCost, DeliveryInDays: req.DeliveryInDays,
		Notes: req.Notes, SalesStatus: req.SalesStatus,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

func (s *Server) handleKBDeleteZone(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteLiveZone(ctx(c), orgID, currentUser(c).ID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

// --- specialists / services (salon vertical, PLAN.md) -----------------------

func (s *Server) handleKBUpsertSpecialist(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req specialistReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Ref == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "ref required")
		return
	}
	portfolio, ok := s.validateMediaList(c, "portfolio_images", &req.PortfolioImages)
	if !ok {
		return
	}
	if err := s.kb.PutLiveSpecialist(ctx(c), orgID, currentUser(c).ID, kbstore.SpecialistInput{
		Ref: req.Ref, FullName: req.FullName, Title: req.Title, Experience: req.Experience,
		Schedule: req.Schedule, BookingURL: req.BookingURL, SalesStatus: req.SalesStatus,
		Media: kbstore.SpecialistMedia{PortfolioImages: portfolio},
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

func (s *Server) handleKBDeleteSpecialist(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteLiveSpecialist(ctx(c), orgID, currentUser(c).ID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

// salesStatusReq is the status-only archive/restore endpoints' whole body
// (handleKBSpecialistStatus/handleKBServiceStatus) — a direct, immediate
// live write with no confirmation step and no draft/approve involvement
// (PLAN.md: "no hard-delete UI is introduced" — archive/restore is just
// this status flip).
type salesStatusReq struct {
	SalesStatus string `json:"sales_status"`
}

// handleKBSpecialistStatus flips a specialist's sales_status (archive/
// restore) via the atomic SetLiveSpecialistSalesStatus (live.go) — a
// single-column UPDATE, not a read-modify-write of the whole row (see that
// method's own doc comment for why: the earlier PutLiveSpecialist-based
// version could silently clobber a concurrent edit to any other field).
// validateEnum (inside SetLiveSpecialistSalesStatus) is the enum's actual
// gate — an invalid value surfaces as the usual 422 via kbFail, not a
// separate check here. The existence pre-check below is only for a clean
// 404; SetLiveSpecialistSalesStatus itself would otherwise report a generic
// not-found error for the same case.
//
// Response shape: the payload is the single updated SpecialistRow (not the
// whole live view) — the frontend's setSpecialistStatus (stores/
// playground.ts) splices this one row back into its own already-loaded
// `live.specialists` array in place, exactly like campaignTemplates.ts's
// own archive()/restore() do for a single row.
func (s *Server) handleKBSpecialistStatus(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req salesStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad status")
		return
	}
	ref := c.Param("ref")
	if _, exists := s.findLiveSpecialist(c, orgID, ref); !exists {
		return
	}
	updated, err := s.kb.SetLiveSpecialistSalesStatus(ctx(c), orgID, currentUser(c).ID, ref, req.SalesStatus)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.invalidateKBCache(orgID)
	s.hub.Broadcast("kb.row.changed", gin.H{})
	ok(c, updated)
}

// findLiveSpecialist reads the org's live view and returns the one
// specialist row matching ref, or fails the request with 404/ErrNotFound
// and returns exists=false. Shared by handleKBSpecialistStatus's before/
// after reads — there is no dedicated "get one live row" store method (no
// other entity has one either; every existing caller that needs one row
// already reads the whole LiveView and filters, e.g. kbLiveChanged's own
// epilogue).
func (s *Server) findLiveSpecialist(c *gin.Context, orgID uuid.UUID, ref string) (kbstore.SpecialistRow, bool) {
	view, err := s.kb.LiveView(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return kbstore.SpecialistRow{}, false
	}
	for i := range view.Specialists {
		if view.Specialists[i].Ref == ref {
			return view.Specialists[i], true
		}
	}
	fail(c, http.StatusNotFound, ErrNotFound, "specialist not found")
	return kbstore.SpecialistRow{}, false
}

func (s *Server) handleKBUpsertService(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req serviceReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Ref == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "ref required")
		return
	}
	if err := s.kb.PutLiveService(ctx(c), orgID, currentUser(c).ID, kbstore.ServiceInput{
		Ref: req.Ref, ParentRef: req.ParentRef, ServiceType: req.ServiceType, Category: req.Category,
		Name: req.Name, Price: req.Price, Duration: req.Duration, Description: req.Description,
		SpecialistRefs: req.SpecialistRefs, SalesStatus: req.SalesStatus,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

func (s *Server) handleKBDeleteService(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteLiveService(ctx(c), orgID, currentUser(c).ID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

// handleKBServiceStatus is handleKBSpecialistStatus's twin, via the atomic
// SetLiveServiceSalesStatus (live.go) instead of a PutLiveService full-row
// overwrite (same reasoning as handleKBSpecialistStatus's own doc comment).
// SetLiveServiceSalesStatus still enforces both of PutLiveService's
// hierarchy rules: flipping a base service from active to inactive
// atomically cascades to its active children, and restoring a variant/addon
// requires its base to already be active — exactly the archive/restore UI
// flow PLAN.md describes ("Archiving a base service atomically archives its
// active children; restoring children is explicit"). The response payload
// is still just the ONE service named by :ref, not its cascaded children —
// findLiveService's own doc comment explains why a whole-view response is
// wrong here; the frontend re-syncs the rest of the roster from the
// realtime kb.row.changed broadcast this handler still sends.
func (s *Server) handleKBServiceStatus(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req salesStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad status")
		return
	}
	ref := c.Param("ref")
	if _, exists := s.findLiveService(c, orgID, ref); !exists {
		return
	}
	updated, err := s.kb.SetLiveServiceSalesStatus(ctx(c), orgID, currentUser(c).ID, ref, req.SalesStatus)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.invalidateKBCache(orgID)
	s.hub.Broadcast("kb.row.changed", gin.H{})
	ok(c, updated)
}

// findLiveService is findLiveSpecialist's twin for ai_services — see its
// doc comment. Response shape: the payload is the single updated ServiceRow
// (not the whole live view) — the frontend's setServiceStatus (stores/
// playground.ts) splices this one row back into its own already-loaded
// `live.services` array in place. A base service's cascaded children are
// NOT included in this response; the same kb.row.changed broadcast every
// other live write already sends tells open clients to re-fetch the whole
// roster (loadLive), which is where they pick up the cascaded rows.
func (s *Server) findLiveService(c *gin.Context, orgID uuid.UUID, ref string) (kbstore.ServiceRow, bool) {
	view, err := s.kb.LiveView(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return kbstore.ServiceRow{}, false
	}
	for i := range view.Services {
		if view.Services[i].Ref == ref {
			return view.Services[i], true
		}
	}
	fail(c, http.StatusNotFound, ErrNotFound, "service not found")
	return kbstore.ServiceRow{}, false
}

// --- config --------------------------------------------------------------------

func (s *Server) handleKBPatchConfig(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req configReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad config")
		return
	}
	if err := s.kb.PatchLiveConfig(ctx(c), orgID, currentUser(c).ID, kbstore.ConfigPatch{
		Persona: req.Persona, Mission: req.Mission, Guardrails: req.Guardrails,
		LanguagePolicy: req.LanguagePolicy, ReplyMaxWords: req.ReplyMaxWords,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

// --- read-only materials (Файлы (материалы) tab) --------------------------------

// handleKBListMaterials returns the org's kbd_materials rows, read-only — no
// playground endpoint touched, no writes. The media milestone (see plan) is
// the only place this tab grows upload/attach/edit.
func (s *Server) handleKBListMaterials(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	materials, err := s.kb.ListLiveMaterials(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, gin.H{"materials": materials})
}
