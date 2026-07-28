package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/blob"
	"github.com/yerassyldanay/xchats/backend/internal/brain/domain"
	"github.com/yerassyldanay/xchats/backend/internal/kbstore"
)

// kbFail maps a kbstore error to the right HTTP status + machine code, so callers
// don't bucket everything as 500. Unknown errors stay 500.
func (s *Server) kbFail(c *gin.Context, err error) {
	var ge *kbstore.GateError
	switch {
	case errors.As(err, &ge):
		fail(c, http.StatusUnprocessableEntity, ErrValidation, ge.Error())
	case errors.Is(err, kbstore.ErrUnknownKind):
		fail(c, http.StatusBadRequest, ErrValidation, "unknown row kind")
	case errors.Is(err, kbstore.ErrStale):
		fail(c, http.StatusConflict, ErrDraftStale, "draft changed since you loaded it; reload and retry")
	default:
		fail(c, http.StatusInternalServerError, ErrInternal, "kb: "+err.Error())
	}
}

// brainReloader is implemented by the real drafter: it hot-swaps the live KB
// the brain drafts from. The stub drafter does not implement it (no-op reload).
type brainReloader interface {
	SetSnapshot(*domain.Snapshot)
}

// kbReady fails the request if the KB layer isn't wired (stub/no-DB boot).
func (s *Server) kbReady(c *gin.Context) bool {
	if s.kb == nil {
		fail(c, http.StatusServiceUnavailable, ErrInternal, "knowledge base not available")
		return false
	}
	return true
}

// pgOrg resolves the org for a playground request (v1: the operator's single org).
func (s *Server) pgOrg(c *gin.Context) (uuid.UUID, bool) {
	org, ok := s.orgOf(c)
	if !ok {
		return uuid.Nil, false
	}
	return org.ID, true
}

// reloadBrain reloads the live KB into the drafter after an approve.
func (s *Server) reloadBrain(c *gin.Context, orgID uuid.UUID) {
	r, ok := s.drafter.(brainReloader)
	if !ok {
		return
	}
	snap, err := s.kb.LoadLive(ctx(c), orgID)
	if err != nil {
		s.log.Warn("reload brain after approve failed", "err", err)
		return
	}
	r.SetSnapshot(snap)
	s.log.Info("brain KB reloaded", "topics", len(snap.Topics))
}

// --- draft read / discard ---------------------------------------------------

// handlePlaygroundDraft is a side-effect-free read: it always returns the
// merged working view — live rows overlaid by any pending blob entries. There
// is no more "open a draft" step; the blob is created lazily on first write.
func (s *Server) handlePlaygroundDraft(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	view, err := s.kb.Draft(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, view)
}

// handlePlaygroundDiscardDraft clears every pending edit ("Отменить изменения").
// Live rows are untouched.
func (s *Server) handlePlaygroundDiscardDraft(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	if err := s.kb.ClearDraft(ctx(c), orgID); err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, nil)
}

// --- topics ----------------------------------------------------------------

type topicReq struct {
	Slug     string `json:"slug"`
	Lang     string `json:"lang"`
	Title    string `json:"title"`
	Keywords string `json:"keywords"`
	BodyMD   string `json:"body_md"`
}

func (s *Server) handlePlaygroundUpsertTopic(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req topicReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Slug == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "slug required")
		return
	}
	if err := s.kb.UpsertTopic(ctx(c), orgID, kbstore.TopicInput{
		Slug: req.Slug, Lang: req.Lang, Title: req.Title, Keywords: req.Keywords, BodyMD: req.BodyMD,
		Provenance: `{"source":"manual"}`,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

func (s *Server) handlePlaygroundDeleteTopic(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteTopic(ctx(c), orgID, c.Param("slug")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- assets (upload bytes + meta) ------------------------------------------

func (s *Server) handlePlaygroundUploadAsset(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
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
	// owner_kind defaults to 'topic' when only owner_ref is given — the common
	// case of attaching media to a topic (owner_kind must be given explicitly to
	// attach to a product/tariff once those land).
	ownerKind := c.PostForm("owner_kind")
	ownerRef := c.PostForm("owner_ref")
	if ownerKind == "" && ownerRef != "" {
		ownerKind = "topic"
	}
	if err := s.kb.UpsertAsset(ctx(c), orgID, kbstore.AssetInput{
		Ref: ref, Kind: mediaType, OwnerKind: ownerKind, OwnerRef: ownerRef,
		Title: fh.Filename, Description: c.PostForm("description"),
		URL: "/xchats/api/v1/media/" + ref, Lang: c.PostForm("lang"),
		Provenance: `{"source":"manual"}`,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

type assetPatchReq struct {
	Description *string `json:"description"`
	OwnerKind   *string `json:"owner_kind"`
	OwnerRef    *string `json:"owner_ref"`
}

func (s *Server) handlePlaygroundPatchAsset(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req assetPatchReq
	_ = c.ShouldBindJSON(&req)
	if err := s.kb.PatchAsset(ctx(c), orgID, c.Param("ref"), kbstore.AssetPatch{
		Description: req.Description, OwnerKind: req.OwnerKind, OwnerRef: req.OwnerRef,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

func (s *Server) handlePlaygroundDeleteAsset(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteAsset(ctx(c), orgID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- typed facts: tariffs / products / contacts ----------------------------

type tariffReq struct {
	Ref           string `json:"ref"`
	Lang          string `json:"lang"`
	Name          string `json:"name"`
	Price         string `json:"price"`
	LimitText     string `json:"limit_text"`
	Fee           string `json:"fee"`
	Summary       string `json:"summary"`
	PricingType   string `json:"pricing_type"`
	Advantages    string `json:"advantages"`
	Disadvantages string `json:"disadvantages"`
}

func (s *Server) handlePlaygroundUpsertTariff(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req tariffReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Ref == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "ref required")
		return
	}
	if err := s.kb.UpsertTariff(ctx(c), orgID, kbstore.TariffInput{
		Ref: req.Ref, Lang: req.Lang, Name: req.Name, Price: req.Price, LimitText: req.LimitText, Fee: req.Fee,
		Summary: req.Summary, PricingType: req.PricingType, Advantages: req.Advantages, Disadvantages: req.Disadvantages,
		Provenance: `{"source":"manual"}`,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

func (s *Server) handlePlaygroundDeleteTariff(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteTariff(ctx(c), orgID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

type productReq struct {
	Ref          string `json:"ref"`
	Lang         string `json:"lang"`
	Name         string `json:"name"`
	Price        string `json:"price"`
	Description  string `json:"description"`
	Category     string `json:"category"`
	Availability string `json:"availability"`
	// InStock is read only by handleKBUpsertProduct (the live-write path) —
	// handlePlaygroundUpsertProduct never reads it, so a draft write's
	// behavior is unaffected by this field's presence.
	InStock *bool `json:"in_stock"`
}

func (s *Server) handlePlaygroundUpsertProduct(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req productReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Ref == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "ref required")
		return
	}
	if err := s.kb.UpsertProduct(ctx(c), orgID, kbstore.ProductInput{
		Ref: req.Ref, Lang: req.Lang, Name: req.Name, Price: req.Price,
		Description: req.Description, Category: req.Category, Availability: req.Availability,
		Provenance: `{"source":"manual"}`,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

func (s *Server) handlePlaygroundDeleteProduct(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteProduct(ctx(c), orgID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

type contactsReq struct {
	Lang         string  `json:"lang"`
	WhatsApp     *string `json:"whatsapp"`
	Email        *string `json:"email"`
	Address      *string `json:"address"`
	Legal        *string `json:"legal"`
	CallbackTime *string `json:"callback_time"`
	WorkingHours *string `json:"working_hours"`
	Phone        *string `json:"phone"`
	Website      *string `json:"website"`
	Instagram    *string `json:"instagram"`
}

func (s *Server) handlePlaygroundPatchContacts(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req contactsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad contacts")
		return
	}
	if err := s.kb.PatchContacts(ctx(c), orgID, kbstore.ContactPatch{
		Lang: req.Lang, WhatsApp: req.WhatsApp, Email: req.Email, Address: req.Address,
		Legal: req.Legal, CallbackTime: req.CallbackTime,
		WorkingHours: req.WorkingHours, Phone: req.Phone, Website: req.Website, Instagram: req.Instagram,
		Provenance: `{"source":"manual"}`,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- typed facts: commerce policies -----------------------------------------

type policiesReq struct {
	Lang             string  `json:"lang"`
	DeliveryCost     *string `json:"delivery_cost"`
	DeliveryTime     *string `json:"delivery_time"`
	FreeDeliveryFrom *string `json:"free_delivery_from"`
	MinOrder         *string `json:"min_order"`
	Prepayment       *string `json:"prepayment"`
	Installment      *string `json:"installment"`
	ReturnPeriod     *string `json:"return_period"`
	Warranty         *string `json:"warranty"`
	// OutsideZonesNote is read only by handleKBPatchPolicies (the live-write
	// path) — handlePlaygroundPatchPolicies never reads it.
	OutsideZonesNote *string `json:"outside_zones_note"`
}

// zoneReq is the /kb/zones upsert payload — no Playground/draft counterpart
// exists yet (draft milestone later), so this is read only by
// handleKBUpsertZone.
type zoneReq struct {
	Ref               string `json:"ref"`
	Name              string `json:"name"`
	ZoneLevel         string `json:"zone_level"`
	ParentRef         string `json:"parent_ref"`
	DeliveryAvailable bool   `json:"delivery_available"`
	DeliveryCost      string `json:"delivery_cost"`
	DeliveryInDays    string `json:"delivery_in_days"`
	Notes             string `json:"notes"`
	Status            string `json:"status"`
}

func (s *Server) handlePlaygroundPatchPolicies(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req policiesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad policies")
		return
	}
	if err := s.kb.PatchPolicies(ctx(c), orgID, kbstore.PolicyPatch{
		Lang: req.Lang, DeliveryCost: req.DeliveryCost, DeliveryTime: req.DeliveryTime,
		FreeDeliveryFrom: req.FreeDeliveryFrom, MinOrder: req.MinOrder, Prepayment: req.Prepayment,
		Installment: req.Installment, ReturnPeriod: req.ReturnPeriod, Warranty: req.Warranty,
		Provenance: `{"source":"manual"}`,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- config ----------------------------------------------------------------

type configReq struct {
	Persona        *string `json:"persona"`
	Mission        *string `json:"mission"`
	Guardrails     *string `json:"guardrails"`
	LanguagePolicy *string `json:"language_policy"`
	ReplyMaxWords  *int    `json:"reply_max_words"`
}

func (s *Server) handlePlaygroundPatchConfig(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req configReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "bad config")
		return
	}
	if err := s.kb.PatchConfig(ctx(c), orgID, kbstore.ConfigPatch{
		Persona: req.Persona, Mission: req.Mission, Guardrails: req.Guardrails,
		LanguagePolicy: req.LanguagePolicy, ReplyMaxWords: req.ReplyMaxWords,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- approve (materialize pending entries into the live KB — 15 Decision 4) -

// handlePlaygroundApprove approves the WHOLE pending draft ("Сохранить в базу"):
// gate over live ∪ pending → materialize every pending entry into the live
// tables → clear them from the blob → hot-reload the brain.
func (s *Server) handlePlaygroundApprove(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	if err := s.kb.Approve(ctx(c), orgID, kbstore.ApproveSelector{}, s.blob.Exists); err != nil {
		s.kbFail(c, err)
		return
	}
	s.reloadBrain(c, orgID)
	view, err := s.kb.Draft(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.hub.Broadcast("kb.approved", gin.H{})
	ok(c, view)
}

// handlePlaygroundApproveEntity approves ONE pending entity by natural key
// ("Подтвердить" on a single row). kind ∈ topics|assets|tariffs|products|contacts|policies;
// id = slug | ref | ref | ref | lang | lang.
func (s *Server) handlePlaygroundApproveEntity(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	kind := c.Param("kind")
	switch kind {
	case "topics", "assets", "tariffs", "products", "contacts", "policies":
	default:
		fail(c, http.StatusBadRequest, ErrValidation, "kind must be topics|assets|tariffs|products|contacts|policies")
		return
	}
	key := c.Param("id")
	if err := s.kb.Approve(ctx(c), orgID, kbstore.ApproveSelector{Kind: kind, Key: key}, s.blob.Exists); err != nil {
		s.kbFail(c, err)
		return
	}
	s.reloadBrain(c, orgID)
	view, err := s.kb.Draft(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.hub.Broadcast("kb.approved", gin.H{"kind": kind})
	ok(c, view)
}

// --- shared helpers --------------------------------------------------------

// pgWrite is the common preamble for a draft write: KB ready + org resolved, plus
// an OPTIONAL optimistic-concurrency check. A client that sends If-Match (the
// draft's base_version from its last load) gets a 409 DRAFT_STALE if the blob has
// since moved — so concurrent edits don't silently clobber each other. Clients
// that omit the header keep last-write-wins (v1 single-operator default).
func (s *Server) pgWrite(c *gin.Context) (uuid.UUID, bool) {
	if !s.kbReady(c) {
		return uuid.Nil, false
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return uuid.Nil, false
	}
	if tok := strings.Trim(strings.TrimSpace(c.GetHeader("If-Match")), `"`); tok != "" {
		cur, err := s.kb.DraftBaseVersion(ctx(c), orgID)
		if err != nil {
			s.kbFail(c, err)
			return uuid.Nil, false
		}
		want, perr := strconv.ParseInt(tok, 10, 64)
		if perr != nil || want != cur {
			fail(c, http.StatusConflict, ErrDraftStale, "draft changed since you loaded it; reload and retry")
			return uuid.Nil, false
		}
	}
	return orgID, true
}

// kbChanged is the common write epilogue: it broadcasts the row change and
// returns the refreshed draft view so the editor and the chat stay in sync.
func (s *Server) kbChanged(c *gin.Context, orgID uuid.UUID) {
	view, err := s.kb.Draft(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.hub.Broadcast("kb.row.changed", gin.H{"base_version": view.Config.BaseVersion})
	ok(c, view)
}
