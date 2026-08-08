package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dto"
	"github.com/yerassyldanay/xchats/backend/internal/store"
)

// Follow-up (reminder) endpoints — "what needs to happen next".
//
// Day boundaries come from the caller (clientDayStart) because organizations
// carry no timezone; see crm.go's clientDayStart for why that is a request
// parameter rather than a stored setting.

var followupActions = map[string]bool{
	store.FollowupActionCall:    true,
	store.FollowupActionMessage: true,
	store.FollowupActionMeeting: true,
	store.FollowupActionOther:   true,
}

// followupReq is the create/reschedule body. DueAt is the authoritative UTC
// instant; due_date/due_minute are the wall clock the manager typed, echoed
// back unchanged so the form round-trips without a timezone drift.
type followupReq struct {
	CustomerID     string  `json:"customer_id"`
	ConversationID *string `json:"conversation_id"`
	Channel        string  `json:"channel"`
	DueAt          string  `json:"due_at"`
	DueDate        string  `json:"due_date"`
	DueMinute      *int    `json:"due_minute"`
	Action         string  `json:"action"`
	Note           string  `json:"note"`
	AssigneeUserID *string `json:"assignee_user_id"`
}

func (r followupReq) toInput() (store.FollowupInput, error) {
	var in store.FollowupInput
	due, err := time.Parse(time.RFC3339, r.DueAt)
	if err != nil {
		return in, errors.New("due_at must be an RFC3339 timestamp")
	}
	if r.DueDate == "" {
		return in, errors.New("due_date required")
	}
	if _, err := time.Parse("2006-01-02", r.DueDate); err != nil {
		return in, errors.New("due_date must be YYYY-MM-DD")
	}
	if r.DueMinute != nil && (*r.DueMinute < 0 || *r.DueMinute > 1439) {
		return in, errors.New("due_minute must be 0–1439")
	}
	action := r.Action
	if action == "" {
		action = store.FollowupActionCall
	}
	if !followupActions[action] {
		return in, errors.New("action must be call, message, meeting or other")
	}
	in = store.FollowupInput{
		Channel:   r.Channel,
		DueAt:     due.UTC(),
		DueDate:   r.DueDate,
		DueMinute: r.DueMinute,
		Action:    action,
		Note:      strings.TrimSpace(r.Note),
	}
	if r.ConversationID != nil {
		nu, err := parseNullUUID(*r.ConversationID)
		if err != nil {
			return in, errors.New("invalid conversation_id")
		}
		in.ConversationID = nu
	}
	if r.AssigneeUserID != nil {
		nu, err := parseNullUUID(*r.AssigneeUserID)
		if err != nil {
			return in, errors.New("invalid assignee_user_id")
		}
		in.AssigneeUserID = nu
	}
	return in, nil
}

func (s *Server) handleListFollowups(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	limit, offset, pageNum, pageSize := s.pageParams(c)
	f := store.FollowupFilter{
		OrgID:    org.ID,
		Assignee: c.Query("assignee"),
		MeUserID: currentUser(c).ID,
		State:    c.Query("state"),
		Bucket:   c.Query("bucket"),
		Now:      time.Now().UTC(),
		DayStart: clientDayStart(c),
		Limit:    limit,
		Offset:   offset,
	}
	if raw := c.Query("customer_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			fail(c, http.StatusBadRequest, ErrValidation, "invalid customer_id")
			return
		}
		f.CustomerID = uuid.NullUUID{UUID: id, Valid: true}
	}
	items, total, err := s.store.ListFollowups(ctx(c), f)
	if err != nil {
		crmFail(c, err)
		return
	}
	out := make([]dto.Followup, 0, len(items))
	for _, fu := range items {
		out = append(out, dto.MapFollowup(fu))
	}
	ok(c, page{Items: out, Page: pageNum, PageSize: pageSize, Total: total})
}

// handleFollowupBuckets is the reminder header: Today / Tomorrow / This week /
// Overdue counts. Overdue keeps counting an item until it is completed or
// rescheduled — it never ages out on its own.
func (s *Server) handleFollowupBuckets(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	buckets, err := s.store.FollowupBucketCounts(ctx(c), store.FollowupFilter{
		OrgID:    org.ID,
		Assignee: c.Query("assignee"),
		MeUserID: currentUser(c).ID,
		DayStart: clientDayStart(c),
	})
	if err != nil {
		crmFail(c, err)
		return
	}
	ok(c, buckets)
}

func (s *Server) handleCreateFollowup(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	var req followupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	customerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid customer_id")
		return
	}
	in, err := req.toInput()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	in.CustomerID = customerID
	fu, err := s.store.CreateFollowup(ctx(c), org.ID, in, actorOf(c))
	if err != nil {
		crmFail(c, err)
		return
	}
	mapped := dto.MapFollowup(fu)
	s.hub.Broadcast("followup.changed", mapped)
	s.broadcastCustomer(c, org.ID, customerID)
	created(c, mapped)
}

func (s *Server) handleRescheduleFollowup(c *gin.Context) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	var req followupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, "invalid body")
		return
	}
	in, err := req.toInput()
	if err != nil {
		fail(c, http.StatusBadRequest, ErrValidation, err.Error())
		return
	}
	fu, err := s.store.RescheduleFollowup(ctx(c), org.ID, id, in, actorOf(c))
	if err != nil {
		crmFail(c, err)
		return
	}
	mapped := dto.MapFollowup(fu)
	s.hub.Broadcast("followup.changed", mapped)
	s.broadcastCustomer(c, org.ID, fu.CustomerID)
	ok(c, mapped)
}

func (s *Server) handleCompleteFollowup(c *gin.Context) {
	s.setFollowupState(c, store.FollowupCompleted)
}

func (s *Server) handleCancelFollowup(c *gin.Context) {
	s.setFollowupState(c, store.FollowupCancelled)
}

func (s *Server) setFollowupState(c *gin.Context, state string) {
	org, okOrg := s.crmOrg(c)
	if !okOrg {
		return
	}
	id, okID := parseUUID(c, "id")
	if !okID {
		return
	}
	fu, err := s.store.SetFollowupState(ctx(c), org.ID, id, state, actorOf(c))
	if err != nil {
		crmFail(c, err)
		return
	}
	mapped := dto.MapFollowup(fu)
	s.hub.Broadcast("followup.changed", mapped)
	s.broadcastCustomer(c, org.ID, fu.CustomerID)
	ok(c, mapped)
}
