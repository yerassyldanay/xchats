package httpapi

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
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

// kbLiveChanged is the /kb/* write epilogue: hot-reload the brain (the write is
// already live — there is no separate approve step), broadcast, and return the
// refreshed live view.
func (s *Server) kbLiveChanged(c *gin.Context, orgID uuid.UUID) {
	s.reloadBrain(c, orgID)
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
	if err := s.kb.PutLiveTopic(ctx(c), orgID, kbstore.TopicInput{
		Slug: req.Slug, Lang: req.Lang, Title: req.Title, Keywords: req.Keywords, BodyMD: req.BodyMD,
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
	if err := s.kb.DeleteLiveTopic(ctx(c), orgID, c.Param("slug")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

// --- assets (upload bytes + meta; description required) ----------------------

func (s *Server) handleKBUploadAsset(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	desc := strings.TrimSpace(c.PostForm("description"))
	if desc == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "description required")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "file part required")
		return
	}
	f, err := fh.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "cannot open file")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "cannot read file")
		return
	}
	mediaType, mimetype := detectMedia(fh.Filename, fh.Header.Get("Content-Type"))
	ref := uuid.NewString()
	if _, err := s.blob.Put(ref, data, blob.Meta{MediaType: mediaType, Mimetype: mimetype, FileName: fh.Filename, FileSize: int64(len(data))}); err != nil {
		fail(c, http.StatusBadGateway, ErrMediaUnavailable, "store failed")
		return
	}
	ownerKind := c.PostForm("owner_kind")
	ownerRef := c.PostForm("owner_ref")
	if ownerKind == "" && ownerRef != "" {
		ownerKind = "topic"
	}
	if err := s.kb.PutLiveAsset(ctx(c), orgID, kbstore.AssetInput{
		Ref: ref, Kind: mediaType, OwnerKind: ownerKind, OwnerRef: ownerRef,
		Title: fh.Filename, Description: desc, URL: "/xchats/api/v1/media/" + ref, Lang: c.PostForm("lang"),
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

func (s *Server) handleKBPatchAsset(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	var req assetPatchReq
	_ = c.ShouldBindJSON(&req)
	if req.Description != nil && strings.TrimSpace(*req.Description) == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "description required")
		return
	}
	if err := s.kb.PatchLiveAsset(ctx(c), orgID, c.Param("ref"), kbstore.AssetPatch{
		Description: req.Description, OwnerKind: req.OwnerKind, OwnerRef: req.OwnerRef,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}

func (s *Server) handleKBDeleteAsset(c *gin.Context) {
	orgID, proceed := s.kbWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteLiveAsset(ctx(c), orgID, c.Param("ref")); err != nil {
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
	if err := s.kb.PutLiveTariff(ctx(c), orgID, kbstore.TariffInput{
		Ref: req.Ref, Lang: req.Lang, Name: req.Name, Price: req.Price, LimitText: req.LimitText, Fee: req.Fee,
		Summary: req.Summary, PricingType: req.PricingType, Advantages: req.Advantages, Disadvantages: req.Disadvantages,
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
	if err := s.kb.DeleteLiveTariff(ctx(c), orgID, c.Param("ref")); err != nil {
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
	if err := s.kb.PutLiveProduct(ctx(c), orgID, kbstore.ProductInput{
		Ref: req.Ref, Lang: req.Lang, Name: req.Name, Price: req.Price,
		Description: req.Description, Category: req.Category,
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
	if err := s.kb.DeleteLiveProduct(ctx(c), orgID, c.Param("ref")); err != nil {
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
	if err := s.kb.PatchLiveContacts(ctx(c), orgID, kbstore.ContactPatch{
		Lang: req.Lang, WhatsApp: req.WhatsApp, Email: req.Email, Address: req.Address,
		Legal: req.Legal, CallbackTime: req.CallbackTime,
	}); err != nil {
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
	if err := s.kb.PatchLiveConfig(ctx(c), orgID, kbstore.ConfigPatch{
		Persona: req.Persona, Mission: req.Mission, Guardrails: req.Guardrails,
		LanguagePolicy: req.LanguagePolicy, ReplyMaxWords: req.ReplyMaxWords,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbLiveChanged(c, orgID)
}
