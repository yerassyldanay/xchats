package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ---------------------------------------------------------------------------
// Follow-ups — "what needs to happen next"
// ---------------------------------------------------------------------------
// Time is stored twice on purpose (see 0011_crm.up.sql). DueAt is a UTC
// instant and is the only thing ordering, overdue and bucketing read; DueDate
// and DueMinute preserve the wall clock the manager typed so the edit form
// round-trips exactly instead of drifting by a timezone offset.
//
// organizations carry no timezone, so every day-boundary decision takes an
// explicit UTC window the caller computed from the client's offset. That keeps
// the store honest: it never guesses what "today" means.

// Follow-up action types and states — the crm_followups CHECK values.
const (
	FollowupActionCall    = "call"
	FollowupActionMessage = "message"
	FollowupActionMeeting = "meeting"
	FollowupActionOther   = "other"

	FollowupOpen      = "open"
	FollowupCompleted = "completed"
	FollowupCancelled = "cancelled"
)

// Followup is one scheduled next action.
type Followup struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	CustomerID     uuid.UUID
	ConversationID uuid.NullUUID
	Channel        string
	DueAt          time.Time
	DueDate        string
	DueMinute      *int
	Action         string
	Note           string
	AssigneeUserID uuid.NullUUID
	State          string
	CompletedAt    *time.Time
	CreatedByUser  uuid.NullUUID
	CreatedAt      time.Time
	UpdatedAt      time.Time

	// Resolved for display by the list/lookup reads.
	CustomerName string
	AssigneeName string
}

// FollowupInput is a create/reschedule payload. DueMinute nil means an all-day
// follow-up (the time is optional, per the product brief).
type FollowupInput struct {
	CustomerID     uuid.UUID
	ConversationID uuid.NullUUID
	Channel        string
	DueAt          time.Time
	DueDate        string
	DueMinute      *int
	Action         string
	Note           string
	AssigneeUserID uuid.NullUUID
}

// FollowupFilter selects the follow-up views. Bucket is resolved against the
// caller-supplied UTC window; an empty Bucket means "no time filter".
type FollowupFilter struct {
	OrgID      uuid.UUID
	Assignee   string // me|unassigned|<uuid>
	MeUserID   uuid.UUID
	CustomerID uuid.NullUUID
	State      string
	Bucket     string // today|tomorrow|week|overdue
	Now        time.Time
	DayStart   time.Time // start of the client's local "today", in UTC
	Limit      int
	Offset     int
}

// FollowupBuckets is the reminder view's header: how many open follow-ups fall
// into each window. Overdue stays populated until each item is completed or
// rescheduled — an overdue follow-up never silently ages out.
type FollowupBuckets struct {
	Today    int `json:"today"`
	Tomorrow int `json:"tomorrow"`
	ThisWeek int `json:"this_week"`
	Overdue  int `json:"overdue"`
}

const followupCols = `f.id, f.organization_id, f.customer_id, f.conversation_id, f.channel,
	f.due_at, f.due_date, f.due_minute, f.action, f.note, f.assignee_user_id, f.state,
	f.completed_at, f.created_by_user_id, f.created_at, f.updated_at,
	COALESCE(c.display_name, ''), COALESCE(NULLIF(u.display_name, ''), u.email, '')`

const followupFrom = `crm_followups f
	LEFT JOIN crm_customers c ON c.id = f.customer_id
	LEFT JOIN users u ON u.id = f.assignee_user_id`

func scanFollowupDst(f *Followup) []any {
	return []any{&f.ID, &f.OrganizationID, &f.CustomerID, &f.ConversationID, &f.Channel,
		&f.DueAt, &f.DueDate, &f.DueMinute, &f.Action, &f.Note, &f.AssigneeUserID, &f.State,
		&f.CompletedAt, &f.CreatedByUser, &f.CreatedAt, &f.UpdatedAt,
		&f.CustomerName, &f.AssigneeName}
}

// ListFollowups returns one page of follow-ups plus the total.
func (s *Store) ListFollowups(ctx context.Context, f FollowupFilter) ([]Followup, int, error) {
	where := []string{"f.organization_id = $1"}
	args := []any{f.OrgID}

	if f.State != "" {
		args = append(args, f.State)
		where = append(where, "f.state = $"+itoa(len(args)))
	}
	if f.CustomerID.Valid {
		args = append(args, f.CustomerID.UUID)
		where = append(where, "f.customer_id = $"+itoa(len(args)))
	}
	switch {
	case f.Assignee == "me":
		args = append(args, f.MeUserID)
		where = append(where, "f.assignee_user_id = $"+itoa(len(args)))
	case f.Assignee == "unassigned":
		where = append(where, "f.assignee_user_id IS NULL")
	case f.Assignee != "":
		if id, err := uuid.Parse(f.Assignee); err == nil {
			args = append(args, id)
			where = append(where, "f.assignee_user_id = $"+itoa(len(args)))
		}
	}
	if lo, hi, ok := bucketWindow(f.Bucket, f.DayStart); ok {
		if !lo.IsZero() {
			args = append(args, lo)
			where = append(where, "f.due_at >= $"+itoa(len(args)))
		}
		args = append(args, hi)
		where = append(where, "f.due_at < $"+itoa(len(args)))
		// Only an open item can be "due"; a completed one has left the queue.
		where = append(where, "f.state = '"+FollowupOpen+"'")
	}

	clause := strings.Join(where, " AND ")
	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit, offset)
	rows, err := s.db.Query(ctx, `SELECT `+followupCols+` FROM `+followupFrom+`
		WHERE `+clause+`
		ORDER BY f.due_at ASC
		LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
	if err != nil {
		return nil, 0, wrap("list followups", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Followup{}
	for rows.Next() {
		var fu Followup
		if err := rows.Scan(scanFollowupDst(&fu)...); err != nil {
			return nil, 0, err
		}
		out = append(out, fu)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM `+followupFrom+` WHERE `+clause,
		args[:len(args)-2]...).Scan(&total); err != nil {
		return nil, 0, wrap("count followups", err)
	}
	return out, total, nil
}

// bucketWindow turns a bucket name into a [lo, hi) UTC window relative to the
// client's local midnight. A zero lo means "everything before hi" (overdue).
func bucketWindow(bucket string, dayStart time.Time) (lo, hi time.Time, ok bool) {
	if dayStart.IsZero() {
		return time.Time{}, time.Time{}, false
	}
	switch bucket {
	case "today":
		return dayStart, dayStart.AddDate(0, 0, 1), true
	case "tomorrow":
		return dayStart.AddDate(0, 0, 1), dayStart.AddDate(0, 0, 2), true
	case "week":
		// "This week" is the next seven days from today inclusive, not an
		// ISO calendar week: a manager reading the number wants "what is
		// coming up", and a Sunday-anchored week would show 1 on a Saturday.
		return dayStart, dayStart.AddDate(0, 0, 7), true
	case "overdue":
		return time.Time{}, dayStart, true
	default:
		return time.Time{}, time.Time{}, false
	}
}

// FollowupBucketCounts returns the reminder header's four numbers for one
// assignee scope. dayStart is the client's local midnight expressed in UTC.
func (s *Store) FollowupBucketCounts(ctx context.Context, f FollowupFilter) (FollowupBuckets, error) {
	var out FollowupBuckets
	targets := []struct {
		bucket string
		into   *int
	}{
		{"today", &out.Today},
		{"tomorrow", &out.Tomorrow},
		{"week", &out.ThisWeek},
		{"overdue", &out.Overdue},
	}
	for _, t := range targets {
		lo, hi, ok := bucketWindow(t.bucket, f.DayStart)
		if !ok {
			continue
		}
		where := []string{"organization_id = $1", "state = '" + FollowupOpen + "'"}
		args := []any{f.OrgID}
		switch {
		case f.Assignee == "me":
			args = append(args, f.MeUserID)
			where = append(where, "assignee_user_id = $"+itoa(len(args)))
		case f.Assignee == "unassigned":
			where = append(where, "assignee_user_id IS NULL")
		case f.Assignee != "":
			if id, err := uuid.Parse(f.Assignee); err == nil {
				args = append(args, id)
				where = append(where, "assignee_user_id = $"+itoa(len(args)))
			}
		}
		if !lo.IsZero() {
			args = append(args, lo)
			where = append(where, "due_at >= $"+itoa(len(args)))
		}
		args = append(args, hi)
		where = append(where, "due_at < $"+itoa(len(args)))

		if err := s.db.QueryRow(ctx, `SELECT count(*) FROM crm_followups WHERE `+
			strings.Join(where, " AND "), args...).Scan(t.into); err != nil {
			return out, wrap("count followup bucket", err)
		}
	}
	return out, nil
}

// NextOpenFollowup returns the customer's soonest open follow-up — what the
// sidebar shows under «Следующий шаг» and what the AI context block quotes.
// ErrNotFound when there is none.
func (s *Store) NextOpenFollowup(ctx context.Context, orgID, customerID uuid.UUID) (Followup, error) {
	var fu Followup
	err := s.db.QueryRow(ctx, `SELECT `+followupCols+` FROM `+followupFrom+`
		WHERE f.organization_id = $1 AND f.customer_id = $2 AND f.state = 'open'
		ORDER BY f.due_at ASC LIMIT 1`, orgID, customerID).Scan(scanFollowupDst(&fu)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return fu, ErrNotFound
	}
	return fu, wrap("next followup", err)
}

// CreateFollowup schedules a next action and records it on the timeline.
func (s *Store) CreateFollowup(ctx context.Context, orgID uuid.UUID, in FollowupInput, actor uuid.NullUUID) (Followup, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Followup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := customerExists(ctx, tx, orgID, in.CustomerID); err != nil {
		return Followup{}, err
	}
	var id uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO crm_followups (organization_id, customer_id, conversation_id, channel,
			due_at, due_date, due_minute, action, note, assignee_user_id, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		orgID, in.CustomerID, in.ConversationID, in.Channel, in.DueAt, in.DueDate,
		in.DueMinute, in.Action, in.Note, in.AssigneeUserID, actor).Scan(&id); err != nil {
		return Followup{}, wrap("create followup", err)
	}
	if err := appendTimeline(ctx, tx, orgID, in.CustomerID, timelineEvent{
		Kind: TimelineFollowupCreated, Actor: actor, Summary: in.Action + " " + in.DueDate,
		Detail: jsonObject(map[string]any{
			"followup_id": id.String(), "action": in.Action,
			"due_date": in.DueDate, "note": in.Note,
		}),
	}); err != nil {
		return Followup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Followup{}, err
	}
	return s.FollowupByID(ctx, orgID, id)
}

func (s *Store) FollowupByID(ctx context.Context, orgID, id uuid.UUID) (Followup, error) {
	var fu Followup
	err := s.db.QueryRow(ctx, `SELECT `+followupCols+` FROM `+followupFrom+`
		WHERE f.organization_id = $1 AND f.id = $2`, orgID, id).Scan(scanFollowupDst(&fu)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return fu, ErrNotFound
	}
	return fu, wrap("load followup", err)
}

// RescheduleFollowup moves a follow-up (and optionally re-edits it). A
// rescheduled overdue item leaves the overdue bucket — that is the whole point
// of the action, and the timeline keeps the record that it slipped.
func (s *Store) RescheduleFollowup(ctx context.Context, orgID, id uuid.UUID, in FollowupInput, actor uuid.NullUUID) (Followup, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Followup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerID uuid.UUID
	var wasDate string
	if err := tx.QueryRow(ctx, `
		SELECT customer_id, due_date FROM crm_followups
		WHERE organization_id = $1 AND id = $2`, orgID, id).Scan(&customerID, &wasDate); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			return Followup{}, ErrNotFound
		}
		return Followup{}, wrap("load followup", err)
	}
	// state is reset to open: rescheduling a cancelled or completed item is how
	// a manager revives it, and leaving it terminal would hide the new date.
	if _, err := tx.Exec(ctx, `
		UPDATE crm_followups SET due_at = $3, due_date = $4, due_minute = $5, action = $6,
			note = $7, assignee_user_id = $8, state = 'open', completed_at = NULL,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE organization_id = $1 AND id = $2`,
		orgID, id, in.DueAt, in.DueDate, in.DueMinute, in.Action, in.Note, in.AssigneeUserID); err != nil {
		return Followup{}, wrap("reschedule followup", err)
	}
	if err := appendTimeline(ctx, tx, orgID, customerID, timelineEvent{
		Kind: TimelineFollowupRescheduled, Actor: actor, Summary: wasDate + " → " + in.DueDate,
		Detail: jsonObject(map[string]any{"followup_id": id.String(), "from": wasDate, "to": in.DueDate}),
	}); err != nil {
		return Followup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Followup{}, err
	}
	return s.FollowupByID(ctx, orgID, id)
}

// SetFollowupState completes or cancels a follow-up.
func (s *Store) SetFollowupState(ctx context.Context, orgID, id uuid.UUID, state string, actor uuid.NullUUID) (Followup, error) {
	kind := TimelineFollowupCompleted
	if state == FollowupCancelled {
		kind = TimelineFollowupCancelled
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Followup{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var customerID uuid.UUID
	var action, dueDate string
	if err := tx.QueryRow(ctx, `
		SELECT customer_id, action, due_date FROM crm_followups
		WHERE organization_id = $1 AND id = $2`, orgID, id).Scan(&customerID, &action, &dueDate); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			return Followup{}, ErrNotFound
		}
		return Followup{}, wrap("load followup", err)
	}
	completedAt := "NULL"
	if state == FollowupCompleted {
		completedAt = "strftime('%Y-%m-%d %H:%M:%f','now')"
	}
	if _, err := tx.Exec(ctx, `
		UPDATE crm_followups SET state = $3, completed_at = `+completedAt+`,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE organization_id = $1 AND id = $2`, orgID, id, state); err != nil {
		return Followup{}, wrap("set followup state", err)
	}
	if err := appendTimeline(ctx, tx, orgID, customerID, timelineEvent{
		Kind: kind, Actor: actor, Summary: action + " " + dueDate,
		Detail: jsonObject(map[string]any{"followup_id": id.String(), "action": action}),
	}); err != nil {
		return Followup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Followup{}, err
	}
	return s.FollowupByID(ctx, orgID, id)
}
