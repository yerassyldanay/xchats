package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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

// --- draft read / discard ---------------------------------------------------

// handlePlaygroundDraft is a side-effect-free read: it returns the Черновик
// review payload — ONLY what is staged in kbd_draft, plus explicit deletion
// entries. There is no more "open a draft" step; the blob is created lazily
// on first write. An unchanged published row never appears here; the
// published counterpart a reviewer diffs against comes from GET /kb.
func (s *Server) handlePlaygroundDraft(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	changes, err := s.kb.DraftChanges(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, changes)
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
	if err := s.kb.ClearDraft(ctx(c), orgID, currentUser(c).ID); err != nil {
		s.kbFail(c, err)
		return
	}
	ok(c, nil)
}

// --- topics ----------------------------------------------------------------

type topicReq struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	BodyMD string `json:"body_md"`
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
	if err := s.kb.UpsertTopic(ctx(c), orgID, currentUser(c).ID, kbstore.TopicInput{
		Slug: req.Slug, Title: req.Title, BodyMD: req.BodyMD,
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
	if err := s.kb.DeleteTopic(ctx(c), orgID, currentUser(c).ID, c.Param("slug")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- typed facts: tariffs / products / contacts ----------------------------

type tariffReq struct {
	Ref           string `json:"ref"`
	Name          string `json:"name"`
	Price         string `json:"price"`
	LimitText     string `json:"limit_text"`
	Fee           string `json:"fee"`
	Summary       string `json:"summary"`
	PricingType   string `json:"pricing_type"`
	Advantages    string `json:"advantages"`
	Disadvantages string `json:"disadvantages"`
	SalesStatus   string `json:"sales_status"`
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
	if err := s.kb.UpsertTariff(ctx(c), orgID, currentUser(c).ID, kbstore.TariffInput{
		Ref: req.Ref, Name: req.Name, Price: req.Price, LimitText: req.LimitText, Fee: req.Fee,
		Summary: req.Summary, PricingType: req.PricingType, Advantages: req.Advantages, Disadvantages: req.Disadvantages,
		SalesStatus: req.SalesStatus,
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
	if err := s.kb.DeleteTariff(ctx(c), orgID, currentUser(c).ID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

type productReq struct {
	Ref         string `json:"ref"`
	Name        string `json:"name"`
	Price       string `json:"price"`
	Description string `json:"description"`
	Category    string `json:"category"`
	SalesStatus string `json:"sales_status"`
	// InStock is nil-able: omitted/null leaves the draft's current merged
	// value unchanged (kbstore.ProductInput's own nil-means-unchanged idiom).
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
	if err := s.kb.UpsertProduct(ctx(c), orgID, currentUser(c).ID, kbstore.ProductInput{
		Ref: req.Ref, Name: req.Name, Price: req.Price,
		Description: req.Description, Category: req.Category,
		SalesStatus: req.SalesStatus, InStock: req.InStock,
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
	if err := s.kb.DeleteProduct(ctx(c), orgID, currentUser(c).ID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- delivery zones ----------------------------------------------------------

func (s *Server) handlePlaygroundUpsertZone(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	var req zoneReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Ref == "" || req.ZoneLevel == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "ref and zone_level required")
		return
	}
	if err := s.kb.UpsertZone(ctx(c), orgID, currentUser(c).ID, kbstore.DeliveryZoneInput{
		Ref: req.Ref, Name: req.Name, ZoneLevel: req.ZoneLevel, ParentRef: req.ParentRef,
		DeliveryAvailable: req.DeliveryAvailable, DeliveryCost: req.DeliveryCost, DeliveryInDays: req.DeliveryInDays,
		Notes: req.Notes, SalesStatus: req.SalesStatus,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

func (s *Server) handlePlaygroundDeleteZone(c *gin.Context) {
	orgID, proceed := s.pgWrite(c)
	if !proceed {
		return
	}
	if err := s.kb.DeleteZone(ctx(c), orgID, currentUser(c).ID, c.Param("ref")); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

type contactsReq struct {
	WhatsApp         *string `json:"whatsapp"`
	Email            *string `json:"email"`
	Address          *string `json:"address"`
	LegalInformation *string `json:"legal_information"`
	CallbackTime     *string `json:"callback_time"`
	WorkingHours     *string `json:"working_hours"`
	Phone            *string `json:"phone"`
	Website          *string `json:"website"`
	Instagram        *string `json:"instagram"`
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
	if err := s.kb.PatchContacts(ctx(c), orgID, currentUser(c).ID, kbstore.ContactPatch{
		WhatsApp: req.WhatsApp, Email: req.Email, Address: req.Address,
		LegalInformation: req.LegalInformation, CallbackTime: req.CallbackTime,
		WorkingHours: req.WorkingHours, Phone: req.Phone, Website: req.Website, Instagram: req.Instagram,
	}); err != nil {
		s.kbFail(c, err)
		return
	}
	s.kbChanged(c, orgID)
}

// --- typed facts: commerce policies -----------------------------------------

type policiesReq struct {
	DeliveryCost       *string `json:"delivery_cost"`
	DeliveryInDays     *string `json:"delivery_in_days"`
	FreeDeliveryFrom   *string `json:"free_delivery_from"`
	MinOrder           *string `json:"min_order"`
	Prepayment         *string `json:"prepayment"`
	Installment        *string `json:"installment"`
	ReturnPeriodInDays *string `json:"return_period_in_days"`
	Warranty           *string `json:"warranty"`
	OutsideZonesNote   *string `json:"outside_zones_note"`
}

// zoneReq is the upsert payload shared by /kb/zones (live, handleKBUpsertZone)
// and /playground/draft/zones (handlePlaygroundUpsertZone) — same fields,
// same shape as every other live/draft pair.
type zoneReq struct {
	Ref               string `json:"ref"`
	Name              string `json:"name"`
	ZoneLevel         string `json:"zone_level"`
	ParentRef         string `json:"parent_ref"`
	DeliveryAvailable bool   `json:"delivery_available"`
	DeliveryCost      string `json:"delivery_cost"`
	DeliveryInDays    string `json:"delivery_in_days"`
	Notes             string `json:"notes"`
	SalesStatus       string `json:"sales_status"`
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
	if err := s.kb.PatchPolicies(ctx(c), orgID, currentUser(c).ID, kbstore.PolicyPatch{
		DeliveryCost: req.DeliveryCost, DeliveryInDays: req.DeliveryInDays,
		FreeDeliveryFrom: req.FreeDeliveryFrom, MinOrder: req.MinOrder, Prepayment: req.Prepayment,
		Installment: req.Installment, ReturnPeriodInDays: req.ReturnPeriodInDays, Warranty: req.Warranty,
		OutsideZonesNote: req.OutsideZonesNote,
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
	if err := s.kb.PatchConfig(ctx(c), orgID, currentUser(c).ID, kbstore.ConfigPatch{
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
// tables → clear them from the blob → hot-reload the brain. An optional
// If-Match is checked ATOMICALLY inside ApproveVersioned's own locked
// transaction (not a separate pre-check, unlike pgWrite) — see
// kbstore.ApproveVersioned's doc comment for why that distinction matters.
func (s *Server) handlePlaygroundApprove(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	expected, ok2 := ifMatchVersion(c)
	if !ok2 {
		fail(c, http.StatusBadRequest, ErrValidation, "If-Match must be an integer draft_version")
		return
	}
	if err := s.kb.ApproveVersioned(ctx(c), orgID, kbstore.ApproveSelector{}, expected, currentUser(c).ID); err != nil {
		s.kbFail(c, err)
		return
	}
	s.invalidateKBCache(orgID)
	changes, err := s.kb.DraftChanges(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.hub.Broadcast("kb.approved", gin.H{})
	ok(c, changes)
}

// handlePlaygroundApproveEntity approves ONE pending entity by natural key
// ("Подтвердить" on a single row). kind ∈
// topics|tariffs|products|contacts|policies|delivery_zones|config; id = slug |
// ref | ref | domain.ContactSlug | domain.PolicySlug | ref | kbstore.NaturalKeyMain.
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
	case "topics", "tariffs", "products", "contacts", "policies", "delivery_zones", "config":
	default:
		fail(c, http.StatusBadRequest, ErrValidation, "kind must be topics|tariffs|products|contacts|policies|delivery_zones|config")
		return
	}
	key := c.Param("id")
	expected, ok2 := ifMatchVersion(c)
	if !ok2 {
		fail(c, http.StatusBadRequest, ErrValidation, "If-Match must be an integer draft_version")
		return
	}
	if err := s.kb.ApproveVersioned(ctx(c), orgID, kbstore.ApproveSelector{Kind: kind, Key: key}, expected, currentUser(c).ID); err != nil {
		s.kbFail(c, err)
		return
	}
	s.invalidateKBCache(orgID)
	changes, err := s.kb.DraftChanges(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.hub.Broadcast("kb.approved", gin.H{"kind": kind})
	ok(c, changes)
}

// handlePlaygroundCancelChange cancels ONE pending addition, update, or
// removal — «Отменить изменение» on a Черновик card. If-Match is compared
// ATOMICALLY inside CancelChange's own locked transaction (unlike pgWrite's
// pre-check), and is deliberately not enforced when the requested outcome
// already holds (kbstore.CancelChange's doc comment).
func (s *Server) handlePlaygroundCancelChange(c *gin.Context) {
	if !s.kbReady(c) {
		return
	}
	orgID, proceed := s.pgOrg(c)
	if !proceed {
		return
	}
	expected, ok2 := ifMatchVersion(c)
	if !ok2 {
		fail(c, http.StatusBadRequest, ErrValidation, "If-Match must be an integer draft_version")
		return
	}
	result, err := s.kb.CancelChange(ctx(c), orgID, currentUser(c).ID, c.Param("kind"), c.Param("key"), expected)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	changes, err := s.kb.DraftChanges(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	if result.Changed {
		s.hub.Broadcast("kb.row.changed", gin.H{"base_version": changes.BaseVersion})
	}
	ok(c, gin.H{"changed": result.Changed, "changes": changes})
}

// ifMatchVersion parses an optional If-Match header (a bare integer
// draft_version, the same shape ifMatch() sends —
// frontend/src/stores/playground.ts) into a pointer ApproveVersioned can
// compare inside its own locked transaction. Unlike pgWrite's pre-check
// (which races the write it precedes: the check and the write are two
// separate calls with no shared lock), this value is compared atomically
// with the read it validates against, so it can never go stale in between.
// Returns ok=false only for a header present but not a valid integer;
// absent means "no If-Match", which returns (nil, true).
func ifMatchVersion(c *gin.Context) (expected *int64, ok bool) {
	tok := strings.Trim(strings.TrimSpace(c.GetHeader("If-Match")), `"`)
	if tok == "" {
		return nil, true
	}
	v, err := strconv.ParseInt(tok, 10, 64)
	if err != nil {
		return nil, false
	}
	return &v, true
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
// returns the refreshed change set so Черновик and the chat stay in sync.
func (s *Server) kbChanged(c *gin.Context, orgID uuid.UUID) {
	changes, err := s.kb.DraftChanges(ctx(c), orgID)
	if err != nil {
		s.kbFail(c, err)
		return
	}
	s.hub.Broadcast("kb.row.changed", gin.H{"base_version": changes.BaseVersion})
	ok(c, changes)
}
