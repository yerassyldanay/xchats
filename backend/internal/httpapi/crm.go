package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// Customer-management (CRM) endpoints.
//
// Multi-tenancy: every handler here resolves the caller's organization with
// crmOrg (orgOf plus the default-status seeding) and passes org.ID into the
// store call, which puts it in the WHERE clause. A customer, tag, note or
// follow-up id from another organization therefore matches no row and comes
// back as 404 — indistinguishable from a missing one, the same rule orgChat
// already applies to conversations.

// crmOrg is the org guard every CRM route opens with. It also seeds the
// default lifecycle for an organization that has none, so an org created after
// migration 0013 ran gets the same starting statuses without a backfill job.
func (s *Server) crmOrg(c *gin.Context) (store.Organization, bool) {
	org, okOrg := s.orgOf(c)
	if !okOrg {
		return store.Organization{}, false
	}
	if err := s.store.EnsureDefaultStatuses(ctx(c), org.ID); err != nil {
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
		return store.Organization{}, false
	}
	return org, true
}

// crmFail maps a store error onto the shared error vocabulary: a missing (or
// cross-org) row is 404, a uniqueness collision is 409, everything else 500.
// isUniqueViolation is auth.go's engine-neutral domain.ErrDuplicate check —
// the CRM store translates at its own exported boundary, exactly like
// store.CreateUser, so no handler here ever sees a driver error.
func crmFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(c, http.StatusNotFound, ErrNotFound, "not found")
	case errors.Is(err, store.ErrMergeSameCustomer):
		fail(c, http.StatusBadRequest, ErrValidation, "cannot merge a customer into itself")
	case isUniqueViolation(err):
		fail(c, http.StatusConflict, ErrConflict, "already exists")
	default:
		fail(c, http.StatusInternalServerError, ErrInternal, err.Error())
	}
}

// crmCustomer resolves a customer-scoped route's id and enforces org
// membership, mirroring orgChat's contract exactly: a cross-org (or missing)
// id is 404, never an empty 200. Sub-resource reads need this explicitly —
// their own queries are org-scoped and would otherwise answer "no notes" for
// another tenant's customer, which is the right data but the wrong answer.
//
// It returns the RESOLVED id: a merged-away customer resolves to its survivor,
// so a link captured before a merge keeps working.
func (s *Server) crmCustomer(c *gin.Context, orgID uuid.UUID) (uuid.UUID, bool) {
	id, okID := parseUUID(c, "id")
	if !okID {
		return uuid.Nil, false
	}
	cust, err := s.store.CustomerByID(ctx(c), orgID, id)
	if err != nil {
		crmFail(c, err)
		return uuid.Nil, false
	}
	return cust.ID, true
}

// actorOf is the current user as a nullable id, for authorship columns.
func actorOf(c *gin.Context) uuid.NullUUID {
	return uuid.NullUUID{UUID: currentUser(c).ID, Valid: true}
}

// clientDayStart is the start of the CALLER's local day, expressed in UTC.
//
// Organizations carry no timezone, and "today"/"overdue" are local-day
// concepts, so the client sends its own UTC offset (frontend
// lib/schedule.ts's currentUtcOffsetMinutes) and the server does the day
// arithmetic. A missing or out-of-range offset falls back to UTC rather than
// guessing, which is at worst a few hours of bucket skew and never a wrong row.
func clientDayStart(c *gin.Context) time.Time {
	offset := 0
	if raw := c.Query("tz_offset_minutes"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > -900 && n < 900 {
			offset = n
		}
	}
	local := time.Now().UTC().Add(time.Duration(offset) * time.Minute)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	return midnight.Add(-time.Duration(offset) * time.Minute)
}

// --- customers -------------------------------------------------------------

func (s *Server) handleListCustomers(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	limit, offset, pageNum, pageSize := s.pageParams(c)
	dayStart := clientDayStart(c)
	f := store.CustomerFilter{
		OrgID:       org.ID,
		Query:       c.Query("q"),
		Channel:     c.Query("channel"),
		Assignee:    c.Query("assignee"),
		MeUserID:    currentUser(c).ID,
		Followup:    c.Query("followup"),
		DayStartUTC: dayStart,
		DayEndUTC:   dayStart.AddDate(0, 0, 1),
		Limit:       limit,
		Offset:      offset,
	}
	if raw := c.Query("status_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid status_id")
			return
		}
		f.StatusID = uuid.NullUUID{UUID: id, Valid: true}
	}
	if raw := c.Query("tag_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid tag_id")
			return
		}
		f.TagID = uuid.NullUUID{UUID: id, Valid: true}
	}
	customers, total, err := s.store.ListCustomers(ctx(c), f)
	if err != nil {
		crmFail(c, err)
		return
	}
	items := make([]dto.Customer, 0, len(customers))
	for _, cu := range customers {
		items = append(items, dto.MapCustomer(cu))
	}
	ok(c, page{Items: items, Page: pageNum, PageSize: pageSize, Total: total})
}

// customerReq is the create/patch body. Every field is a pointer so an omitted
// key means "leave this alone" — the same absent-vs-empty distinction
// kbstore.DraftConfigPatch draws.
type customerReq struct {
	DisplayName    *string         `json:"display_name"`
	Phone          *string         `json:"phone"`
	Email          *string         `json:"email"`
	AvatarURL      *string         `json:"avatar_url"`
	StatusID       *string         `json:"status_id"`
	AssigneeUserID *string         `json:"assignee_user_id"`
	CustomFields   *map[string]any `json:"custom_fields"`
}

// toPatch converts the request into a store patch. A JSON null on status_id or
// assignee_user_id clears the field; an absent key leaves it untouched — which
// is why both are **string rather than *string.
func (r customerReq) toPatch() (store.CustomerPatch, error) {
	p := store.CustomerPatch{
		DisplayName: r.DisplayName, Phone: r.Phone, Email: r.Email, AvatarURL: r.AvatarURL,
	}
	if r.StatusID != nil {
		nu, err := parseNullUUID(*r.StatusID)
		if err != nil {
			return p, errors.New("invalid status_id")
		}
		p.StatusID = &nu
	}
	if r.AssigneeUserID != nil {
		nu, err := parseNullUUID(*r.AssigneeUserID)
		if err != nil {
			return p, errors.New("invalid assignee_user_id")
		}
		p.AssigneeUserID = &nu
	}
	if r.CustomFields != nil {
		raw, err := json.Marshal(*r.CustomFields)
		if err != nil {
			return p, errors.New("invalid custom_fields")
		}
		p.CustomFields = raw
	}
	return p, nil
}

// parseNullUUID treats "" as an explicit clear, so the UI can null a select
// without a separate endpoint.
func parseNullUUID(raw string) (uuid.NullUUID, error) {
	if raw == "" {
		return uuid.NullUUID{}, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.NullUUID{}, err
	}
	return uuid.NullUUID{UUID: id, Valid: true}, nil
}

func (s *Server) handleCreateCustomer(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	var req customerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	patch, err := req.toPatch()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	cust, err := s.store.CreateCustomer(ctx(c), org.ID, patch, actorOf(c))
	if err != nil {
		crmFail(c, err)
		return
	}
	mapped := dto.MapCustomer(cust)
	s.hub.Broadcast("customer.updated", mapped)
	created(c, mapped)
}

func (s *Server) handleGetCustomer(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	profile, err := s.customerProfile(c, org.ID, id)
	if err != nil {
		crmFail(c, err)
		return
	}
	ok(c, profile)
}

// customerProfile assembles the sidebar's single hydrate payload: the customer
// plus the latest note, the next open follow-up and every conversation across
// their channels.
func (s *Server) customerProfile(c *gin.Context, orgID, id uuid.UUID) (dto.CustomerProfile, error) {
	cust, err := s.store.CustomerByID(ctx(c), orgID, id)
	if err != nil {
		return dto.CustomerProfile{}, err
	}
	out := dto.CustomerProfile{Customer: dto.MapCustomer(cust), Conversations: []dto.Chat{}}

	note, err := s.store.LatestCustomerNote(ctx(c), orgID, cust.ID)
	switch {
	case err == nil:
		mapped := dto.MapCustomerNote(note)
		out.LatestNote = &mapped
	case !errors.Is(err, store.ErrNotFound):
		return dto.CustomerProfile{}, err
	}

	fu, err := s.store.NextOpenFollowup(ctx(c), orgID, cust.ID)
	switch {
	case err == nil:
		mapped := dto.MapFollowup(fu)
		out.NextFollowup = &mapped
	case !errors.Is(err, store.ErrNotFound):
		return dto.CustomerProfile{}, err
	}

	chats, err := s.store.CustomerConversations(ctx(c), orgID, cust.ID)
	if err != nil {
		return dto.CustomerProfile{}, err
	}
	for _, ch := range chats {
		out.Conversations = append(out.Conversations, dto.MapChat(ch))
	}
	return out, nil
}

func (s *Server) handleUpdateCustomer(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req customerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	patch, err := req.toPatch()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	cust, err := s.store.UpdateCustomer(ctx(c), org.ID, id, patch, actorOf(c))
	if err != nil {
		crmFail(c, err)
		return
	}
	mapped := dto.MapCustomer(cust)
	s.hub.Broadcast("customer.updated", mapped)
	ok(c, mapped)
}

func (s *Server) handleCustomerConversations(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okCust := s.crmCustomer(c, org.ID)
	if !okCust {
		return
	}
	chats, err := s.store.CustomerConversations(ctx(c), org.ID, id)
	if err != nil {
		crmFail(c, err)
		return
	}
	items := make([]dto.Chat, 0, len(chats))
	for _, ch := range chats {
		items = append(items, dto.MapChat(ch))
	}
	ok(c, gin.H{"items": items})
}

func (s *Server) handleCustomerTimeline(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okCust := s.crmCustomer(c, org.ID)
	if !okCust {
		return
	}
	limit := 100
	if n, err := strconv.Atoi(c.Query("limit")); err == nil && n > 0 {
		limit = n
	}
	entries, err := s.store.CustomerTimeline(ctx(c), org.ID, id, limit)
	if err != nil {
		crmFail(c, err)
		return
	}
	items := make([]dto.TimelineEntry, 0, len(entries))
	for _, e := range entries {
		items = append(items, dto.MapTimelineEntry(e))
	}
	ok(c, gin.H{"items": items})
}

type mergeReq struct {
	SourceCustomerID string `json:"source_customer_id"`
}

// handleMergeCustomers folds the source customer into the one addressed by the
// URL. This is the only merge path — nothing auto-merges on name or phone.
func (s *Server) handleMergeCustomers(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	targetID, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req mergeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	sourceID, err := uuid.Parse(req.SourceCustomerID)
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid source_customer_id")
		return
	}
	cust, err := s.store.MergeCustomers(ctx(c), org.ID, targetID, sourceID, actorOf(c))
	if err != nil {
		crmFail(c, err)
		return
	}
	mapped := dto.MapCustomer(cust)
	s.hub.Broadcast("customer.updated", mapped)
	ok(c, mapped)
}

// --- notes -----------------------------------------------------------------

func (s *Server) handleListCustomerNotes(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okCust := s.crmCustomer(c, org.ID)
	if !okCust {
		return
	}
	notes, err := s.store.ListCustomerNotes(ctx(c), org.ID, id)
	if err != nil {
		crmFail(c, err)
		return
	}
	items := make([]dto.CustomerNote, 0, len(notes))
	for _, n := range notes {
		items = append(items, dto.MapCustomerNote(n))
	}
	ok(c, gin.H{"items": items})
}

type noteReq struct {
	Body string `json:"body"`
}

func (s *Server) handleAddCustomerNote(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req noteReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Body) == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "body required")
		return
	}
	note, err := s.store.AddCustomerNote(ctx(c), org.ID, id, actorOf(c), strings.TrimSpace(req.Body))
	if err != nil {
		crmFail(c, err)
		return
	}
	s.broadcastCustomer(c, org.ID, id)
	created(c, dto.MapCustomerNote(note))
}

func (s *Server) handleUpdateCustomerNote(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	noteID, okID := parseUUID(c, "note_id")
	if !okID {
		return
	}
	var req noteReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Body) == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "body required")
		return
	}
	note, err := s.store.UpdateCustomerNote(ctx(c), org.ID, noteID, strings.TrimSpace(req.Body))
	if err != nil {
		crmFail(c, err)
		return
	}
	ok(c, dto.MapCustomerNote(note))
}

func (s *Server) handleDeleteCustomerNote(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	noteID, okID := parseUUID(c, "note_id")
	if !okID {
		return
	}
	if err := s.store.DeleteCustomerNote(ctx(c), org.ID, noteID); err != nil {
		crmFail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}

// --- tags ------------------------------------------------------------------

func (s *Server) handleListTags(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	tags, err := s.store.ListTags(ctx(c), org.ID)
	if err != nil {
		crmFail(c, err)
		return
	}
	items := make([]dto.CustomerTag, 0, len(tags))
	for _, t := range tags {
		items = append(items, dto.MapCustomerTag(t))
	}
	ok(c, gin.H{"items": items})
}

type tagReq struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (s *Server) handleCreateTag(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	var req tagReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "name required")
		return
	}
	name := strings.TrimSpace(req.Name)
	tag, err := s.store.CreateTag(ctx(c), org.ID, store.CustomerTag{
		Slug: slugify(name), Name: name, Color: req.Color,
	})
	if err != nil {
		crmFail(c, err)
		return
	}
	created(c, dto.MapCustomerTag(tag))
}

func (s *Server) handleUpdateTag(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req tagReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "name required")
		return
	}
	tag, err := s.store.UpdateTag(ctx(c), org.ID, id, store.CustomerTag{
		Name: strings.TrimSpace(req.Name), Color: req.Color,
	})
	if err != nil {
		crmFail(c, err)
		return
	}
	ok(c, dto.MapCustomerTag(tag))
}

func (s *Server) handleDeleteTag(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	if err := s.store.DeleteTag(ctx(c), org.ID, id); err != nil {
		crmFail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}

type customerTagReq struct {
	TagID string `json:"tag_id"`
}

func (s *Server) handleAddCustomerTag(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req customerTagReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	tagID, err := uuid.Parse(req.TagID)
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid tag_id")
		return
	}
	if err := s.store.AddCustomerTag(ctx(c), org.ID, id, tagID, actorOf(c)); err != nil {
		crmFail(c, err)
		return
	}
	s.respondCustomer(c, org.ID, id)
}

func (s *Server) handleRemoveCustomerTag(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	tagID, okTag := parseUUID(c, "tag_id")
	if !okTag {
		return
	}
	if err := s.store.RemoveCustomerTag(ctx(c), org.ID, id, tagID, actorOf(c)); err != nil {
		crmFail(c, err)
		return
	}
	s.respondCustomer(c, org.ID, id)
}

// --- statuses --------------------------------------------------------------

func (s *Server) handleListStatuses(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	statuses, err := s.store.ListStatuses(ctx(c), org.ID)
	if err != nil {
		crmFail(c, err)
		return
	}
	items := make([]dto.CustomerStatus, 0, len(statuses))
	for _, st := range statuses {
		items = append(items, dto.MapCustomerStatus(st))
	}
	ok(c, gin.H{"items": items})
}

type statusReq struct {
	Name     string `json:"name"`
	Color    string `json:"color"`
	Position int    `json:"position"`
}

func (s *Server) handleCreateStatus(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	var req statusReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "name required")
		return
	}
	name := strings.TrimSpace(req.Name)
	st, err := s.store.CreateStatus(ctx(c), org.ID, store.CustomerStatus{
		Slug: slugify(name), Name: name, Color: req.Color, Position: req.Position,
	})
	if err != nil {
		crmFail(c, err)
		return
	}
	created(c, dto.MapCustomerStatus(st))
}

func (s *Server) handleUpdateStatus(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req statusReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		fail(c, http.StatusBadRequest, ErrValidation, "name required")
		return
	}
	st, err := s.store.UpdateStatus(ctx(c), org.ID, id, store.CustomerStatus{
		Name: strings.TrimSpace(req.Name), Color: req.Color, Position: req.Position,
	})
	if err != nil {
		crmFail(c, err)
		return
	}
	ok(c, dto.MapCustomerStatus(st))
}

func (s *Server) handleDeleteStatus(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	if err := s.store.DeleteStatus(ctx(c), org.ID, id); err != nil {
		crmFail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}

// --- custom field definitions ---------------------------------------------

func (s *Server) handleListCustomFields(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	defs, err := s.store.ListCustomFieldDefs(ctx(c), org.ID)
	if err != nil {
		crmFail(c, err)
		return
	}
	items := make([]dto.CustomFieldDef, 0, len(defs))
	for _, d := range defs {
		items = append(items, dto.MapCustomFieldDef(d))
	}
	ok(c, gin.H{"items": items})
}

type customFieldReq struct {
	Label     string   `json:"label"`
	FieldType string   `json:"field_type"`
	Options   []string `json:"options"`
	Position  int      `json:"position"`
}

var customFieldTypes = map[string]bool{"text": true, "number": true, "date": true, "select": true}

func (r customFieldReq) validate() (store.CustomFieldDef, error) {
	label := strings.TrimSpace(r.Label)
	if label == "" {
		return store.CustomFieldDef{}, errors.New("label required")
	}
	ft := r.FieldType
	if ft == "" {
		ft = "text"
	}
	if !customFieldTypes[ft] {
		return store.CustomFieldDef{}, errors.New("field_type must be text, number, date or select")
	}
	opts, err := json.Marshal(r.Options)
	if err != nil {
		return store.CustomFieldDef{}, errors.New("invalid options")
	}
	if r.Options == nil {
		opts = []byte("[]")
	}
	return store.CustomFieldDef{
		Key: slugify(label), Label: label, FieldType: ft, Options: opts, Position: r.Position,
	}, nil
}

func (s *Server) handleCreateCustomField(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	var req customFieldReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	in, err := req.validate()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	def, err := s.store.CreateCustomFieldDef(ctx(c), org.ID, in)
	if err != nil {
		crmFail(c, err)
		return
	}
	created(c, dto.MapCustomFieldDef(def))
}

func (s *Server) handleUpdateCustomField(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req customFieldReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	in, err := req.validate()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	def, err := s.store.UpdateCustomFieldDef(ctx(c), org.ID, id, in)
	if err != nil {
		crmFail(c, err)
		return
	}
	ok(c, dto.MapCustomFieldDef(def))
}

func (s *Server) handleDeleteCustomField(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	if err := s.store.DeleteCustomFieldDef(ctx(c), org.ID, id); err != nil {
		crmFail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}

// --- shared responders -----------------------------------------------------

// respondCustomer re-reads the customer and answers with it — the shape every
// mutation that is not itself a customer edit (tags) returns, so the sidebar
// never has to follow up with a GET.
func (s *Server) respondCustomer(c *gin.Context, orgID, id uuid.UUID) {
	cust, err := s.store.CustomerByID(ctx(c), orgID, id)
	if err != nil {
		crmFail(c, err)
		return
	}
	mapped := dto.MapCustomer(cust)
	s.hub.Broadcast("customer.updated", mapped)
	ok(c, mapped)
}

// broadcastCustomer pushes a refreshed customer over SSE without answering the
// request — for mutations whose own response is a different entity (a note).
// A failure here is not worth failing the request that already succeeded.
func (s *Server) broadcastCustomer(c *gin.Context, orgID, id uuid.UUID) {
	cust, err := s.store.CustomerByID(ctx(c), orgID, id)
	if err != nil {
		return
	}
	s.hub.Broadcast("customer.updated", dto.MapCustomer(cust))
}

// slugify derives a stable natural key from an operator-typed label.
//
// It keeps letters and digits of ANY script rather than reducing to [a-z0-9]:
// this product's operators write Russian, and an ASCII-only slug would collapse
// «Интересуется» and «Не готов» both to the empty string, so the second tag an
// organization created would collide on UNIQUE (organization_id, slug) and be
// rejected as a duplicate. SQLite stores UTF-8 natively and the slug is an
// internal key, never a URL path segment, so «интересуется» is a perfectly good
// one. A label with no letters or digits at all still yields "", which the
// unique constraint correctly treats as one reserved key per organization.
func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
