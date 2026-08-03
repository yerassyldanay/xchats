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
	if err := s.kb.PutLiveTopic(ctx(c), orgID, currentUser(c).ID, kbstore.TopicInput{
		Slug: req.Slug, Title: req.Title, BodyMD: req.BodyMD,
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
	if err := s.kb.PutLiveTariff(ctx(c), orgID, currentUser(c).ID, kbstore.TariffInput{
		Ref: req.Ref, Name: req.Name, Price: req.Price, LimitText: req.LimitText, Fee: req.Fee,
		Summary: req.Summary, PricingType: req.PricingType, Advantages: req.Advantages, Disadvantages: req.Disadvantages,
		SalesStatus: req.SalesStatus,
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
	if err := s.kb.PutLiveProduct(ctx(c), orgID, currentUser(c).ID, kbstore.ProductInput{
		Ref: req.Ref, Name: req.Name, Price: req.Price,
		Description: req.Description, Category: req.Category,
		InStock: req.InStock, SalesStatus: req.SalesStatus,
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
	if err := s.kb.PatchLiveContacts(ctx(c), orgID, currentUser(c).ID, kbstore.ContactPatch{
		WhatsApp: req.WhatsApp, Email: req.Email, Address: req.Address,
		LegalInformation: req.LegalInformation, CallbackTime: req.CallbackTime,
		WorkingHours: req.WorkingHours, Phone: req.Phone, Website: req.Website, Instagram: req.Instagram,
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
	if err := s.kb.PatchLivePolicies(ctx(c), orgID, currentUser(c).ID, kbstore.PolicyPatch{
		DeliveryCost: req.DeliveryCost, DeliveryInDays: req.DeliveryInDays,
		FreeDeliveryFrom: req.FreeDeliveryFrom, MinOrder: req.MinOrder, Prepayment: req.Prepayment,
		Installment: req.Installment, ReturnPeriodInDays: req.ReturnPeriodInDays, Warranty: req.Warranty,
		OutsideZonesNote: req.OutsideZonesNote,
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
