package store

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------
// account_id columns below carry no foreign key: an account is a wa_accounts,
// tg_accounts or channel_accounts row and a single FK cannot express an
// either/or reference — see migrations/sqlite/0012_campaigns.up.sql's file
// header for the identical, already-established reasoning.

// Campaign is one bulk-send campaign.
type Campaign struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	Name               string
	AccountID          uuid.UUID
	Channel            string
	Status             string
	MessageBody        string
	Variables          []string // detected {{variable}} names — informational, see ExtractVariables
	MinIntervalSeconds *int     // per-campaign pace override; nil = inherit the account's own pace
	JitterSeconds      *int
	ScheduleAt         *time.Time
	StartedAt          *time.Time
	CreatedBy          uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CampaignPatch is PATCH /campaigns/:id's write shape. Every non-nil/true
// field is applied exactly as given; internal/httpapi resolves which fields
// are safe to touch (backend/campaign.CanEditContent/CanEditPacing) BEFORE
// calling UpdateCampaign, and — when AccountID changes — also resolves the
// new account's Channel and sets both together, so this table's redundant
// channel column never drifts from the account it names.
//
// MinIntervalSeconds/JitterSeconds/ScheduleAt are nullable columns that a
// patch might need to explicitly CLEAR back to NULL, not just leave alone —
// the HasPace/HasScheduleAt flags are what make that three-state distinction
// possible without double pointers: false means "don't touch this field at
// all"; true with a nil pointer means "clear it to NULL"; true with a
// non-nil pointer means "set it to this value".
type CampaignPatch struct {
	Name        *string
	MessageBody *string
	AccountID   *uuid.UUID
	Channel     *string

	HasPace            bool
	MinIntervalSeconds *int
	JitterSeconds      *int

	HasScheduleAt bool
	ScheduleAt    *time.Time

	HasWindows bool
	Windows    []CampaignWindowInput
}

// CampaignRecipient is one persisted row in a campaign's recipient list.
type CampaignRecipient struct {
	ID                 uuid.UUID
	CampaignID         uuid.UUID
	NormalizedIdentity string
	RawInput           string
	Name               string
	Attributes         map[string]string
	Status             string
	FailureReason      string
	Attempts           int
	NextAttemptAt      *time.Time
	ChatID             uuid.NullUUID
	MessageID          uuid.NullUUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CampaignRecipientInput is one row of ReplaceCampaignRecipients' input —
// CampaignRecipient minus everything the store itself derives (id, status,
// lifecycle fields).
type CampaignRecipientInput struct {
	NormalizedIdentity string
	RawInput           string
	Name               string
	Attributes         map[string]string
}

// CampaignEvent is one row of a campaign's audit timeline.
type CampaignEvent struct {
	ID          uuid.UUID
	CampaignID  uuid.UUID
	Event       string
	ActorUserID uuid.NullUUID
	Detail      map[string]any
	CreatedAt   time.Time
}

// CampaignAccountSettings is one account's pace + manual pause switch.
type CampaignAccountSettings struct {
	AccountID          uuid.UUID
	LimitMode          string
	MinIntervalSeconds int
	JitterSeconds      int
	// Paused is a manual, ACCOUNT-WIDE kill switch distinct from any one
	// campaign's own status — the claim path skips the account entirely
	// while this is set, regardless of what state its individual campaigns
	// are in. Separate from (and in addition to) the automatic per-campaign
	// pause a >60s disconnect causes.
	Paused    bool
	UpdatedAt time.Time
}

// CampaignAccountSettingsInput is SetCampaignAccountLimits' settings half.
type CampaignAccountSettingsInput struct {
	LimitMode          string
	MinIntervalSeconds int
	JitterSeconds      int
	Paused             bool
}

// CampaignWindowInput is one recurring UTC window as accepted from an API
// caller — CampaignWindow minus its generated id, for a windows-replace call.
type CampaignWindowInput struct {
	Weekday     int
	StartMinute int
	EndMinute   int
}

// CampaignWindow is one recurring UTC window, stored (either an account's
// own campaign_account_windows row or a campaign's own campaign_windows
// row — both share this shape, see 0012_campaigns.up.sql).
type CampaignWindow struct {
	ID          uuid.UUID
	Weekday     int
	StartMinute int
	EndMinute   int
}

// Claim is one recipient ClaimNextRecipient has just claimed — everything
// the campaign runner needs to render and send, in one round trip.
type Claim struct {
	LogID              uuid.UUID
	RecipientID        uuid.UUID
	CampaignID         uuid.UUID
	AccountID          uuid.UUID
	Channel            string
	NormalizedIdentity string
	Name               string
	Attributes         map[string]string
	MessageBody        string
	Attempts           int // the NEW attempt count, post-increment (1 on a first try)
}

// FinalizeAttemptParams is FinalizeAttempt's input — see that method's doc
// comment for the at-most-once story this closes out.
type FinalizeAttemptParams struct {
	LogID         uuid.UUID
	RecipientID   uuid.UUID
	NewStatus     purecampaign.RecipientStatus // Sent, Pending (will retry) or Failed (terminal)
	FailureReason string
	ChatID        uuid.NullUUID
	MessageID     uuid.NullUUID
	// NextAttemptAt is the backoff floor a transient failure being retried
	// (NewStatus == RecipientPending) sets; ignored otherwise.
	NextAttemptAt *time.Time
}

// SendingBudgetTier is one tier's live usage, for the UI's budget widget.
type SendingBudgetTier struct {
	WindowSeconds int
	MaxSends      int
	Used          int
}

// SendingBudget is GET /accounts/:id/sending-budget's live shape.
type SendingBudget struct {
	AccountID          uuid.UUID
	Tiers              []SendingBudgetTier
	MinIntervalSeconds int
	JitterSeconds      int
	Paused             bool
	Allowed            bool // true if a send could be attempted right now
	ThrottledBy        int  // the WindowSeconds of the tightest tier currently at capacity, 0 if none
	NextSendAt         time.Time
}

// ErrInvalidTransition is returned by SetCampaignStatus when the requested
// transition is not one backend/campaign.CanTransition allows.
var ErrInvalidTransition = errors.New("store: invalid campaign status transition")

// ---------------------------------------------------------------------------
// Campaign CRUD
// ---------------------------------------------------------------------------

const campaignCols = `id, organization_id, name, account_id, channel, status, message_body, variables,
	min_interval_seconds, jitter_seconds, schedule_at, started_at, created_by, created_at, updated_at`

func scanCampaign(row dbx.Scanner) (Campaign, error) {
	var c Campaign
	var variablesRaw string
	err := row.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.AccountID, &c.Channel, &c.Status, &c.MessageBody, &variablesRaw,
		&c.MinIntervalSeconds, &c.JitterSeconds, &c.ScheduleAt, &c.StartedAt, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, err
	}
	c.Variables = decodeStringSlice(variablesRaw)
	return c, nil
}

// CreateCampaign inserts a new draft. c.Variables is derived from
// c.MessageBody (see backend/campaign.ExtractVariables), never taken from
// the input struct.
func (s *Store) CreateCampaign(ctx context.Context, c Campaign) (Campaign, error) {
	variablesJSON, err := json.Marshal(purecampaign.ExtractVariables(c.MessageBody))
	if err != nil {
		return Campaign{}, wrap("marshal campaign variables", err)
	}
	out, err := scanCampaign(s.db.QueryRow(ctx, `
		INSERT INTO campaigns (organization_id, name, account_id, channel, message_body, variables, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+campaignCols,
		c.OrganizationID, c.Name, c.AccountID, c.Channel, c.MessageBody, string(variablesJSON), c.CreatedBy))
	return out, wrap("create campaign", err)
}

// CampaignByID returns a campaign by id, on any organization.
func (s *Store) CampaignByID(ctx context.Context, id uuid.UUID) (Campaign, error) {
	out, err := scanCampaign(s.db.QueryRow(ctx, `SELECT `+campaignCols+` FROM campaigns WHERE id = $1`, id))
	if errors.Is(err, dbx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

// CampaignByIDForOrg returns a campaign only if it belongs to orgID —
// the membership guard behind every /campaigns/:id endpoint (mirrors
// ChatByIDForOrg). ErrNotFound for a cross-org id, indistinguishable from a
// missing one.
func (s *Store) CampaignByIDForOrg(ctx context.Context, id, orgID uuid.UUID) (Campaign, error) {
	out, err := scanCampaign(s.db.QueryRow(ctx, `SELECT `+campaignCols+` FROM campaigns WHERE id = $1 AND organization_id = $2`, id, orgID))
	if errors.Is(err, dbx.ErrNoRows) {
		return out, ErrNotFound
	}
	return out, err
}

// ListCampaignsForOrg returns the org's campaigns, newest first, plus the total.
func (s *Store) ListCampaignsForOrg(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]Campaign, int, error) {
	rows, err := s.db.Query(ctx, `SELECT `+campaignCols+`
		FROM campaigns WHERE organization_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM campaigns WHERE organization_id = $1`, orgID).Scan(&total)
	return out, total, nil
}

// UpdateCampaign applies p to id unconditionally — see CampaignPatch's own
// doc comment for the safety split between this store method (an
// unconditional write) and internal/httpapi (which decides what is safe to
// include in p at all).
func (s *Store) UpdateCampaign(ctx context.Context, id uuid.UUID, p CampaignPatch) (Campaign, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Campaign{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var sets []string
	var args []any
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+" = $"+itoa(len(args)))
	}
	if p.Name != nil {
		set("name", *p.Name)
	}
	if p.MessageBody != nil {
		set("message_body", *p.MessageBody)
		vj, err := json.Marshal(purecampaign.ExtractVariables(*p.MessageBody))
		if err != nil {
			return Campaign{}, wrap("marshal campaign variables", err)
		}
		set("variables", string(vj))
	}
	if p.AccountID != nil {
		set("account_id", *p.AccountID)
	}
	if p.Channel != nil {
		set("channel", *p.Channel)
	}
	if p.HasPace {
		if p.MinIntervalSeconds != nil {
			set("min_interval_seconds", *p.MinIntervalSeconds)
		} else {
			sets = append(sets, "min_interval_seconds = NULL")
		}
		if p.JitterSeconds != nil {
			set("jitter_seconds", *p.JitterSeconds)
		} else {
			sets = append(sets, "jitter_seconds = NULL")
		}
	}
	if p.HasScheduleAt {
		if p.ScheduleAt != nil {
			set("schedule_at", *p.ScheduleAt)
		} else {
			sets = append(sets, "schedule_at = NULL")
		}
	}
	sets = append(sets, "updated_at = strftime('%Y-%m-%d %H:%M:%f','now')")

	args = append(args, id)
	q := `UPDATE campaigns SET ` + joinComma(sets) + ` WHERE id = $` + itoa(len(args)) + ` RETURNING ` + campaignCols
	out, err := scanCampaign(tx.QueryRow(ctx, q, args...))
	if errors.Is(err, dbx.ErrNoRows) {
		return Campaign{}, ErrNotFound
	}
	if err != nil {
		return Campaign{}, wrap("update campaign", err)
	}

	if p.HasWindows {
		if err := replaceCampaignWindowsTx(ctx, tx, id, p.Windows); err != nil {
			return Campaign{}, err
		}
	}
	return out, tx.Commit(ctx)
}

// SetCampaignStatus atomically checks backend/campaign.CanTransition, applies
// the new status (stamping started_at the first time it becomes Running —
// COALESCE leaves an existing value alone on a later paused->running resume),
// and appends the campaign_events row, all in one transaction.
func (s *Store) SetCampaignStatus(ctx context.Context, id uuid.UUID, to purecampaign.Status, actorUserID uuid.NullUUID, event string, detail map[string]any) (Campaign, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Campaign{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var from string
	if err := tx.QueryRow(ctx, `SELECT status FROM campaigns WHERE id = $1`, id).Scan(&from); err != nil {
		if errors.Is(err, dbx.ErrNoRows) {
			return Campaign{}, ErrNotFound
		}
		return Campaign{}, err
	}
	if !purecampaign.CanTransition(purecampaign.Status(from), to) {
		return Campaign{}, ErrInvalidTransition
	}

	setClause := `status = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`
	if to == purecampaign.StatusRunning {
		setClause += `, started_at = COALESCE(started_at, strftime('%Y-%m-%d %H:%M:%f','now'))`
	}
	out, err := scanCampaign(tx.QueryRow(ctx, `UPDATE campaigns SET `+setClause+` WHERE id = $1 RETURNING `+campaignCols, id, string(to)))
	if err != nil {
		return Campaign{}, wrap("set campaign status", err)
	}
	if err := insertCampaignEvent(ctx, tx, id, event, actorUserID, detail); err != nil {
		return Campaign{}, err
	}
	return out, tx.Commit(ctx)
}

// DuplicateCampaign creates a new draft copying name/account/channel/
// message body/pace/windows from srcID, and copies only its never-contacted
// recipients (status pending or failed — never sent or skipped) as fresh
// pending rows with attempts reset to zero.
func (s *Store) DuplicateCampaign(ctx context.Context, srcID, actorUserID uuid.UUID) (Campaign, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Campaign{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	src, err := scanCampaign(tx.QueryRow(ctx, `SELECT `+campaignCols+` FROM campaigns WHERE id = $1`, srcID))
	if errors.Is(err, dbx.ErrNoRows) {
		return Campaign{}, ErrNotFound
	}
	if err != nil {
		return Campaign{}, err
	}
	variablesJSON, err := json.Marshal(src.Variables)
	if err != nil {
		return Campaign{}, wrap("marshal campaign variables", err)
	}
	out, err := scanCampaign(tx.QueryRow(ctx, `
		INSERT INTO campaigns (organization_id, name, account_id, channel, message_body, variables, min_interval_seconds, jitter_seconds, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+campaignCols,
		src.OrganizationID, src.Name+" (copy)", src.AccountID, src.Channel, src.MessageBody, string(variablesJSON),
		src.MinIntervalSeconds, src.JitterSeconds, actorUserID))
	if err != nil {
		return Campaign{}, wrap("duplicate campaign", err)
	}

	srcWindows, err := campaignWindows(ctx, tx, srcID)
	if err != nil {
		return Campaign{}, err
	}
	for _, w := range srcWindows {
		if _, err := tx.Exec(ctx, `INSERT INTO campaign_windows (campaign_id, weekday, start_minute, end_minute) VALUES ($1,$2,$3,$4)`,
			out.ID, int(w.Weekday), w.StartMinute, w.EndMinute); err != nil {
			return Campaign{}, wrap("copy campaign window", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO campaign_recipients (campaign_id, normalized_identity, raw_input, name, attributes)
		SELECT $1, normalized_identity, raw_input, name, attributes
		FROM campaign_recipients WHERE campaign_id = $2 AND status IN ('pending','failed')`,
		out.ID, srcID); err != nil {
		return Campaign{}, wrap("copy never-contacted recipients", err)
	}

	if err := insertCampaignEvent(ctx, tx, out.ID, "duplicated",
		uuid.NullUUID{UUID: actorUserID, Valid: true},
		map[string]any{"source_campaign_id": srcID.String()}); err != nil {
		return Campaign{}, err
	}
	return out, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Campaign events (audit timeline)
// ---------------------------------------------------------------------------

func insertCampaignEvent(ctx context.Context, q dbx.DBTX, campaignID uuid.UUID, event string, actorUserID uuid.NullUUID, detail map[string]any) error {
	var actor any
	if actorUserID.Valid {
		actor = actorUserID.UUID
	}
	if detail == nil {
		detail = map[string]any{}
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return wrap("marshal campaign event detail", err)
	}
	_, err = q.Exec(ctx, `INSERT INTO campaign_events (campaign_id, event, actor_user_id, detail) VALUES ($1,$2,$3,$4)`,
		campaignID, event, actor, string(detailJSON))
	return wrap("insert campaign event", err)
}

// AppendCampaignEvent records a standalone timeline entry (not tied to a
// status transition — e.g. "recipients_replaced", "retried_failed").
func (s *Store) AppendCampaignEvent(ctx context.Context, campaignID uuid.UUID, event string, actorUserID uuid.NullUUID, detail map[string]any) error {
	return insertCampaignEvent(ctx, s.db, campaignID, event, actorUserID, detail)
}

// ListCampaignEvents returns a campaign's timeline, newest first, plus the total.
func (s *Store) ListCampaignEvents(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]CampaignEvent, int, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, campaign_id, event, actor_user_id, detail, created_at
		FROM campaign_events WHERE campaign_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, campaignID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []CampaignEvent
	for rows.Next() {
		var e CampaignEvent
		var detailRaw string
		if err := rows.Scan(&e.ID, &e.CampaignID, &e.Event, &e.ActorUserID, &detailRaw, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		e.Detail = decodeAnyMap(detailRaw)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM campaign_events WHERE campaign_id = $1`, campaignID).Scan(&total)
	return out, total, nil
}

// ---------------------------------------------------------------------------
// Recipients
// ---------------------------------------------------------------------------

const campaignRecipientCols = `id, campaign_id, normalized_identity, raw_input, name, attributes,
	status, failure_reason, attempts, next_attempt_at, chat_id, message_id, created_at, updated_at`

func scanCampaignRecipient(row dbx.Scanner) (CampaignRecipient, error) {
	var r CampaignRecipient
	var attrsRaw string
	err := row.Scan(&r.ID, &r.CampaignID, &r.NormalizedIdentity, &r.RawInput, &r.Name, &attrsRaw,
		&r.Status, &r.FailureReason, &r.Attempts, &r.NextAttemptAt, &r.ChatID, &r.MessageID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return r, err
	}
	r.Attributes = decodeStringMap(attrsRaw)
	return r, nil
}

// ReplaceCampaignRecipients reconciles campaignID's recipient set against
// rows: every 'pending' row not present in rows (by normalized identity) is
// deleted, every row in rows is upserted (updating raw_input/name/
// attributes only while still 'pending' — the ON CONFLICT's WHERE guard
// leaves a sent/failed/skipped row's data exactly as it was, since content
// on an already-contacted recipient is frozen). A duplicate identity within
// rows itself keeps only the LAST occurrence.
func (s *Store) ReplaceCampaignRecipients(ctx context.Context, campaignID uuid.UUID, rows []CampaignRecipientInput) error {
	deduped := make(map[string]CampaignRecipientInput, len(rows))
	order := make([]string, 0, len(rows))
	for _, r := range rows {
		if _, ok := deduped[r.NormalizedIdentity]; !ok {
			order = append(order, r.NormalizedIdentity)
		}
		deduped[r.NormalizedIdentity] = r
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		DELETE FROM campaign_recipients
		WHERE campaign_id = $1 AND status = 'pending'
		  AND normalized_identity NOT IN (SELECT value FROM json_each($2))`,
		campaignID, dbx.StringArray(order)); err != nil {
		return wrap("prune campaign recipients", err)
	}

	for _, identity := range order {
		r := deduped[identity]
		attrsJSON, err := json.Marshal(r.Attributes)
		if err != nil {
			return wrap("marshal recipient attributes", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO campaign_recipients (campaign_id, normalized_identity, raw_input, name, attributes)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (campaign_id, normalized_identity) DO UPDATE SET
				raw_input = excluded.raw_input, name = excluded.name, attributes = excluded.attributes,
				updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE campaign_recipients.status = 'pending'`,
			campaignID, identity, r.RawInput, r.Name, string(attrsJSON)); err != nil {
			return wrap("upsert campaign recipient", err)
		}
	}
	return tx.Commit(ctx)
}

// ListCampaignRecipients returns a campaign's recipients, oldest first
// (recipient list order), optionally filtered to one status, plus the total.
func (s *Store) ListCampaignRecipients(ctx context.Context, campaignID uuid.UUID, status string, limit, offset int) ([]CampaignRecipient, int, error) {
	where := "campaign_id = $1"
	args := []any{campaignID}
	if status != "" {
		args = append(args, status)
		where += " AND status = $" + itoa(len(args))
	}
	countArgs := append([]any(nil), args...)
	args = append(args, limit, offset)
	q := `SELECT ` + campaignRecipientCols + ` FROM campaign_recipients WHERE ` + where +
		` ORDER BY created_at LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []CampaignRecipient
	for rows.Next() {
		r, err := scanCampaignRecipient(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM campaign_recipients WHERE `+where, countArgs...).Scan(&total)
	return out, total, nil
}

// CampaignRecipientCounts returns campaignID's recipient counts grouped by
// status — the list/detail views' progress numbers, always computed live
// rather than cached, since a campaign's recipient count is bounded (never a
// per-message-frequency hot path).
func (s *Store) CampaignRecipientCounts(ctx context.Context, campaignID uuid.UUID) (map[string]int, error) {
	rows, err := s.db.Query(ctx, `SELECT status, count(*) FROM campaign_recipients WHERE campaign_id = $1 GROUP BY status`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

// RetryFailedRecipients resets selected (or, with recipientIDs empty, every)
// 'failed' recipient back to 'pending' with attempts and backoff cleared —
// a fresh, deliberate operator action starts the transient-retry ladder over.
func (s *Store) RetryFailedRecipients(ctx context.Context, campaignID uuid.UUID, recipientIDs []uuid.UUID) (int, error) {
	const setClause = `status = 'pending', failure_reason = '', attempts = 0, next_attempt_at = NULL,
		updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`
	var tag dbx.CommandTag
	var err error
	if len(recipientIDs) == 0 {
		tag, err = s.db.Exec(ctx, `UPDATE campaign_recipients SET `+setClause+` WHERE campaign_id = $1 AND status = 'failed'`, campaignID)
	} else {
		tag, err = s.db.Exec(ctx, `
			UPDATE campaign_recipients SET `+setClause+`
			WHERE campaign_id = $1 AND status = 'failed' AND id IN (SELECT value FROM json_each($2))`,
			campaignID, dbx.UUIDArray(recipientIDs))
	}
	if err != nil {
		return 0, wrap("retry failed campaign recipients", err)
	}
	return int(tag.RowsAffected()), nil
}

// SuppressPendingForIdentity marks every still-pending campaign_recipients
// row for identity, across every RUNNING campaign sharing accountID, as
// 'skipped' with reason. A campaign not yet running when the trigger fires
// is untouched — by the time it starts, this event has already passed, so
// there is nothing to suppress retroactively; its own send to the same
// identity proceeds normally.
func (s *Store) SuppressPendingForIdentity(ctx context.Context, accountID uuid.UUID, identity, reason string) (int, error) {
	return suppressPendingForIdentity(ctx, s.db, accountID, identity, reason)
}

// suppressPendingForIdentity is SuppressPendingForIdentity's core,
// parameterized on dbx.DBTX so UpsertInbound (internal/store/ingest.go) can
// run it inside its OWN transaction — the inbound upsert and the
// suppression it triggers commit atomically together, rather than as two
// separate round trips a crash between them could split.
func suppressPendingForIdentity(ctx context.Context, q dbx.DBTX, accountID uuid.UUID, identity, reason string) (int, error) {
	tag, err := q.Exec(ctx, `
		UPDATE campaign_recipients SET status = 'skipped', failure_reason = $3, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE status = 'pending' AND normalized_identity = $2
		  AND campaign_id IN (SELECT id FROM campaigns WHERE account_id = $1 AND status = 'running')`,
		accountID, identity, reason)
	if err != nil {
		return 0, wrap("suppress pending campaign recipients", err)
	}
	return int(tag.RowsAffected()), nil
}

// ---------------------------------------------------------------------------
// Campaign windows (read: with id, for the API; write: replace-set)
// ---------------------------------------------------------------------------

// CampaignWindowsFor returns a campaign's own narrowing windows.
func (s *Store) CampaignWindowsFor(ctx context.Context, campaignID uuid.UUID) ([]CampaignWindow, error) {
	rows, err := s.db.Query(ctx, `SELECT id, weekday, start_minute, end_minute FROM campaign_windows WHERE campaign_id = $1 ORDER BY weekday, start_minute`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCampaignWindowRows(rows)
}

// CampaignAccountWindowsFor returns an account's own quiet-hours windows.
func (s *Store) CampaignAccountWindowsFor(ctx context.Context, accountID uuid.UUID) ([]CampaignWindow, error) {
	rows, err := s.db.Query(ctx, `SELECT id, weekday, start_minute, end_minute FROM campaign_account_windows WHERE account_id = $1 ORDER BY weekday, start_minute`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCampaignWindowRows(rows)
}

func scanCampaignWindowRows(rows *dbx.Rows) ([]CampaignWindow, error) {
	var out []CampaignWindow
	for rows.Next() {
		var w CampaignWindow
		if err := rows.Scan(&w.ID, &w.Weekday, &w.StartMinute, &w.EndMinute); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func replaceCampaignWindowsTx(ctx context.Context, tx dbx.DBTX, campaignID uuid.UUID, windows []CampaignWindowInput) error {
	if _, err := tx.Exec(ctx, `DELETE FROM campaign_windows WHERE campaign_id = $1`, campaignID); err != nil {
		return wrap("clear campaign windows", err)
	}
	for _, w := range windows {
		if _, err := tx.Exec(ctx, `INSERT INTO campaign_windows (campaign_id, weekday, start_minute, end_minute) VALUES ($1,$2,$3,$4)`,
			campaignID, w.Weekday, w.StartMinute, w.EndMinute); err != nil {
			return wrap("insert campaign window", err)
		}
	}
	return nil
}

// campaignWindows and campaignAccountWindows below are the PURE-Window
// (no id) projections Budget/WindowsOK consume — a distinct, narrower query
// from CampaignWindowsFor/CampaignAccountWindowsFor above (which the API
// needs ids from), parameterized on dbx.DBTX so ClaimNextRecipient can read
// a consistent snapshot inside its own transaction while every other caller
// just passes s.db.

func campaignWindows(ctx context.Context, q dbx.DBTX, campaignID uuid.UUID) ([]purecampaign.Window, error) {
	rows, err := q.Query(ctx, `SELECT weekday, start_minute, end_minute FROM campaign_windows WHERE campaign_id = $1`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPureWindows(rows)
}

func campaignAccountWindows(ctx context.Context, q dbx.DBTX, accountID uuid.UUID) ([]purecampaign.Window, error) {
	rows, err := q.Query(ctx, `SELECT weekday, start_minute, end_minute FROM campaign_account_windows WHERE account_id = $1`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPureWindows(rows)
}

func scanPureWindows(rows *dbx.Rows) ([]purecampaign.Window, error) {
	var out []purecampaign.Window
	for rows.Next() {
		var w purecampaign.Window
		var weekday int
		if err := rows.Scan(&weekday, &w.StartMinute, &w.EndMinute); err != nil {
			return nil, err
		}
		w.Weekday = time.Weekday(weekday)
		out = append(out, w)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Account pace + tiers
// ---------------------------------------------------------------------------

func defaultCampaignAccountSettings(accountID uuid.UUID, channel string) CampaignAccountSettings {
	interval, jitter := purecampaign.DefaultPacingFor(channel)
	return CampaignAccountSettings{AccountID: accountID, LimitMode: "default", MinIntervalSeconds: interval, JitterSeconds: jitter}
}

func campaignAccountSettings(ctx context.Context, q dbx.DBTX, accountID uuid.UUID, channel string) (CampaignAccountSettings, error) {
	var out CampaignAccountSettings
	err := q.QueryRow(ctx, `
		SELECT account_id, limit_mode, min_interval_seconds, jitter_seconds, paused, updated_at
		FROM campaign_account_settings WHERE account_id = $1`, accountID).
		Scan(&out.AccountID, &out.LimitMode, &out.MinIntervalSeconds, &out.JitterSeconds, &out.Paused, &out.UpdatedAt)
	if errors.Is(err, dbx.ErrNoRows) {
		return defaultCampaignAccountSettings(accountID, channel), nil
	}
	return out, err
}

// CampaignAccountSettingsFor returns accountID's pace + pause switch, or the
// implicit channel default (backend/campaign.DefaultPacingFor) if never
// configured — never ErrNotFound, mirroring
// AutomationSettingsForAccount's own "unconfigured is a valid state" shape.
func (s *Store) CampaignAccountSettingsFor(ctx context.Context, accountID uuid.UUID, channel string) (CampaignAccountSettings, error) {
	return campaignAccountSettings(ctx, s.db, accountID, channel)
}

func campaignAccountLimits(ctx context.Context, q dbx.DBTX, accountID uuid.UUID, channel string) ([]purecampaign.Tier, error) {
	rows, err := q.Query(ctx, `SELECT window_seconds, max_sends FROM campaign_account_limits WHERE account_id = $1 ORDER BY window_seconds`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []purecampaign.Tier
	for rows.Next() {
		var t purecampaign.Tier
		if err := rows.Scan(&t.WindowSeconds, &t.MaxSends); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return purecampaign.DefaultTiersFor(channel), nil
	}
	return out, nil
}

// CampaignAccountLimitsFor returns accountID's rolling-window tiers, or the
// implicit channel default (backend/campaign.DefaultTiersFor) if never
// configured.
func (s *Store) CampaignAccountLimitsFor(ctx context.Context, accountID uuid.UUID, channel string) ([]purecampaign.Tier, error) {
	return campaignAccountLimits(ctx, s.db, accountID, channel)
}

// SetCampaignAccountLimits replaces accountID's settings, full tier set, and
// full quiet-hours window set in one transaction — windows and tiers are
// always a complete replace (delete-and-reinsert), matching
// SetAutomationSettings' own "one save = the whole schedule" shape.
func (s *Store) SetCampaignAccountLimits(ctx context.Context, accountID uuid.UUID, settings CampaignAccountSettingsInput, tiers []purecampaign.Tier, windows []CampaignWindowInput) (CampaignAccountSettings, []purecampaign.Tier, []CampaignWindow, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return CampaignAccountSettings{}, nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var out CampaignAccountSettings
	if err := tx.QueryRow(ctx, `
		INSERT INTO campaign_account_settings (account_id, limit_mode, min_interval_seconds, jitter_seconds, paused)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (account_id) DO UPDATE SET
			limit_mode = excluded.limit_mode, min_interval_seconds = excluded.min_interval_seconds,
			jitter_seconds = excluded.jitter_seconds, paused = excluded.paused,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		RETURNING account_id, limit_mode, min_interval_seconds, jitter_seconds, paused, updated_at`,
		accountID, settings.LimitMode, settings.MinIntervalSeconds, settings.JitterSeconds, settings.Paused).
		Scan(&out.AccountID, &out.LimitMode, &out.MinIntervalSeconds, &out.JitterSeconds, &out.Paused, &out.UpdatedAt); err != nil {
		return CampaignAccountSettings{}, nil, nil, wrap("upsert campaign account settings", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM campaign_account_limits WHERE account_id = $1`, accountID); err != nil {
		return CampaignAccountSettings{}, nil, nil, wrap("clear campaign account limits", err)
	}
	tiersOut := make([]purecampaign.Tier, 0, len(tiers))
	for _, t := range tiers {
		if _, err := tx.Exec(ctx, `INSERT INTO campaign_account_limits (account_id, window_seconds, max_sends) VALUES ($1,$2,$3)`,
			accountID, t.WindowSeconds, t.MaxSends); err != nil {
			return CampaignAccountSettings{}, nil, nil, wrap("insert campaign account limit", err)
		}
		tiersOut = append(tiersOut, t)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM campaign_account_windows WHERE account_id = $1`, accountID); err != nil {
		return CampaignAccountSettings{}, nil, nil, wrap("clear campaign account windows", err)
	}
	windowsOut := make([]CampaignWindow, 0, len(windows))
	for _, w := range windows {
		var row CampaignWindow
		if err := tx.QueryRow(ctx, `
			INSERT INTO campaign_account_windows (account_id, weekday, start_minute, end_minute)
			VALUES ($1,$2,$3,$4) RETURNING id, weekday, start_minute, end_minute`,
			accountID, w.Weekday, w.StartMinute, w.EndMinute).
			Scan(&row.ID, &row.Weekday, &row.StartMinute, &row.EndMinute); err != nil {
			return CampaignAccountSettings{}, nil, nil, wrap("insert campaign account window", err)
		}
		windowsOut = append(windowsOut, row)
	}

	return out, tiersOut, windowsOut, tx.Commit(ctx)
}

// campaignPaceOverride returns a campaign's own min-interval/jitter pace
// override, if it has one — see the CHECK on campaigns that keeps the pair
// all-or-nothing.
func campaignPaceOverride(ctx context.Context, q dbx.DBTX, campaignID uuid.UUID) (minIntervalSeconds, jitterSeconds int, ok bool, err error) {
	var interval, jitter *int
	if err := q.QueryRow(ctx, `SELECT min_interval_seconds, jitter_seconds FROM campaigns WHERE id = $1`, campaignID).Scan(&interval, &jitter); err != nil {
		return 0, 0, false, err
	}
	if interval == nil {
		return 0, 0, false, nil
	}
	return *interval, *jitter, true, nil
}

// jitteredSeconds returns base randomized by up to ±jitter seconds
// (uniform), floored at 0 — a fresh draw per claim, so consecutive sends
// don't settle into an exactly-periodic, obviously-automated cadence.
func jitteredSeconds(base, jitter int) int {
	if jitter <= 0 {
		return base
	}
	delta := rand.Intn(2*jitter+1) - jitter
	if base+delta < 0 {
		return 0
	}
	return base + delta
}

// SendingBudget reports accountID's LIVE sending headroom — the shared
// widget the Accounts page, the campaign wizard, and a campaign's own detail
// page all render from, so the number can never disagree between them. The
// reported NextSendAt is un-jittered (backend/campaign.Estimate's own
// documented approximation) and reflects tiers + interval + the account's
// own quiet hours; it does not know about any one campaign's further
// narrowing, since this endpoint is account-, not campaign-, scoped.
func (s *Store) SendingBudget(ctx context.Context, accountID uuid.UUID, channel string, now time.Time) (SendingBudget, error) {
	settings, err := s.CampaignAccountSettingsFor(ctx, accountID, channel)
	if err != nil {
		return SendingBudget{}, err
	}
	tiers, err := s.CampaignAccountLimitsFor(ctx, accountID, channel)
	if err != nil {
		return SendingBudget{}, err
	}
	accountWindows, err := campaignAccountWindows(ctx, s.db, accountID)
	if err != nil {
		return SendingBudget{}, err
	}

	attempts, err := campaignSendAttempts(ctx, s.db, accountID, tiers, time.Duration(settings.MinIntervalSeconds)*time.Second, now)
	if err != nil {
		return SendingBudget{}, err
	}
	var lastSend time.Time
	if len(attempts) > 0 {
		lastSend = attempts[len(attempts)-1]
	}
	baseInterval := time.Duration(settings.MinIntervalSeconds) * time.Second
	decision := purecampaign.Budget(tiers, attempts, baseInterval, lastSend, now)
	windowsOK := purecampaign.WindowsOK(accountWindows, nil, now)
	// NextSendAt is seeded with the SAME real attempts decision was just
	// computed from — Estimate has no knowledge of history on its own, and
	// projecting from an empty slate here would silently ignore an
	// already-partly-consumed budget (see backend/campaign.Estimate's own
	// priorAttempts doc comment).
	nextSendAt := purecampaign.Estimate(1, attempts, tiers, baseInterval, accountWindows, nil, now)

	out := SendingBudget{
		AccountID: accountID, MinIntervalSeconds: settings.MinIntervalSeconds, JitterSeconds: settings.JitterSeconds,
		Paused: settings.Paused, Allowed: !settings.Paused && decision.Allowed && windowsOK,
		ThrottledBy: decision.ThrottledBy, NextSendAt: nextSendAt,
	}
	for _, t := range tiers {
		windowStart := now.Add(-time.Duration(t.WindowSeconds) * time.Second)
		used := 0
		for _, a := range attempts {
			if a.After(windowStart) {
				used++
			}
		}
		out.Tiers = append(out.Tiers, SendingBudgetTier{WindowSeconds: t.WindowSeconds, MaxSends: t.MaxSends, Used: used})
	}
	return out, nil
}

// campaignSendAttempts reads every campaign-origin attempt timestamp within
// the widest window either the tiers or minInterval could need, sorted
// ascending — the shared query SendingBudget and ClaimNextRecipient both
// build their backend/campaign.Budget call from.
func campaignSendAttempts(ctx context.Context, q dbx.DBTX, accountID uuid.UUID, tiers []purecampaign.Tier, minInterval time.Duration, now time.Time) ([]time.Time, error) {
	maxWindowSeconds := 0
	for _, t := range tiers {
		if t.WindowSeconds > maxWindowSeconds {
			maxWindowSeconds = t.WindowSeconds
		}
	}
	lookback := time.Duration(maxWindowSeconds) * time.Second
	if minInterval > lookback {
		lookback = minInterval
	}
	cutoff := now.Add(-lookback)

	rows, err := q.Query(ctx, `
		SELECT attempted_at FROM campaign_send_log
		WHERE account_id = $1 AND origin = 'campaign' AND attempted_at > $2
		ORDER BY attempted_at ASC`, accountID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Claim + finalize (the at-most-once send path)
// ---------------------------------------------------------------------------

// claimBatch bounds how many candidate pending recipients ClaimNextRecipient
// considers per call before giving up on finding one whose campaign's own
// window is currently open — generous enough that a handful of narrowly-
// windowed campaigns sharing an account never starve a wide-open one behind
// them in FIFO order.
const claimBatch = 50

// ClaimNextRecipient runs in ONE transaction: pick the next eligible
// recipient across every RUNNING campaign on accountID (FIFO by campaign
// start time, ties broken by recipient creation order), evaluate every tier
// + interval + window simultaneously (backend/campaign.Budget/WindowsOK)
// and, if permitted, insert the campaign_send_log row (outcome recorded
// pessimistically as 'failed' — see FinalizeAttempt) and flip the recipient
// to 'sending'. The commit precedes the provider call the caller is about
// to make — that ordering is what makes at-most-once true: a crash after
// this returns but before the send completes leaves the recipient correctly
// stuck in 'sending', which ReconcileStuckSending turns into a terminal
// 'failed' at the next boot, never silently retried.
//
// ok is false — with no error — whenever there is genuinely nothing to
// claim right now: the account is manually paused, no running campaign has
// an eligible recipient, every eligible candidate's own campaign window is
// currently closed, or the account-wide budget (tiers, interval) has no
// headroom this instant. The caller (the campaign Scheduler) treats every
// case identically: try again next tick.
//
// Only the FIRST FIFO-and-window-eligible candidate's own tier/interval
// budget is evaluated — not a search across every candidate for one whose
// (possibly faster) per-campaign pace override happens to be satisfied
// right now. This keeps the claim path to the one account-scoped pass the
// plan calls for; the account's shared tiers bound every campaign
// identically regardless, so the practical effect of this simplification is
// bounded to the interval-only portion of the budget.
func (s *Store) ClaimNextRecipient(ctx context.Context, accountID uuid.UUID, now time.Time) (Claim, bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Claim{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var paused bool
	if err := tx.QueryRow(ctx, `SELECT paused FROM campaign_account_settings WHERE account_id = $1`, accountID).Scan(&paused); err != nil && !errors.Is(err, dbx.ErrNoRows) {
		return Claim{}, false, err
	}
	if paused {
		return Claim{}, false, tx.Commit(ctx)
	}

	rows, err := tx.Query(ctx, `
		SELECT cr.id, cr.campaign_id, cr.normalized_identity, cr.raw_input, cr.name, cr.attributes,
			c.channel, c.message_body, cr.attempts
		FROM campaign_recipients cr
		JOIN campaigns c ON c.id = cr.campaign_id
		WHERE c.account_id = $1 AND c.status = 'running' AND cr.status = 'pending'
		  AND (cr.next_attempt_at IS NULL OR cr.next_attempt_at <= $2)
		ORDER BY c.started_at ASC, cr.created_at ASC
		LIMIT $3`, accountID, now, claimBatch)
	if err != nil {
		return Claim{}, false, err
	}
	type candidate struct {
		recipientID uuid.UUID
		campaignID  uuid.UUID
		identity    string
		rawInput    string
		name        string
		attrsRaw    string
		channel     string
		messageBody string
		attempts    int
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.recipientID, &c.campaignID, &c.identity, &c.rawInput, &c.name, &c.attrsRaw, &c.channel, &c.messageBody, &c.attempts); err != nil {
			rows.Close()
			return Claim{}, false, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return Claim{}, false, err
	}
	if err := rows.Close(); err != nil {
		return Claim{}, false, err
	}
	if len(candidates) == 0 {
		return Claim{}, false, tx.Commit(ctx)
	}

	accountWindows, err := campaignAccountWindows(ctx, tx, accountID)
	if err != nil {
		return Claim{}, false, err
	}
	campaignWindowsCache := map[uuid.UUID][]purecampaign.Window{}

	var chosen *candidate
	for i := range candidates {
		c := &candidates[i]
		cw, ok := campaignWindowsCache[c.campaignID]
		if !ok {
			cw, err = campaignWindows(ctx, tx, c.campaignID)
			if err != nil {
				return Claim{}, false, err
			}
			campaignWindowsCache[c.campaignID] = cw
		}
		if purecampaign.WindowsOK(accountWindows, cw, now) {
			chosen = c
			break
		}
	}
	if chosen == nil {
		return Claim{}, false, tx.Commit(ctx)
	}

	settings, err := campaignAccountSettings(ctx, tx, accountID, chosen.channel)
	if err != nil {
		return Claim{}, false, err
	}
	tiers, err := campaignAccountLimits(ctx, tx, accountID, chosen.channel)
	if err != nil {
		return Claim{}, false, err
	}

	minIntervalSeconds, jitterSeconds := settings.MinIntervalSeconds, settings.JitterSeconds
	if ci, cj, ok, err := campaignPaceOverride(ctx, tx, chosen.campaignID); err != nil {
		return Claim{}, false, err
	} else if ok {
		minIntervalSeconds, jitterSeconds = ci, cj
	}
	minInterval := time.Duration(jitteredSeconds(minIntervalSeconds, jitterSeconds)) * time.Second
	// The attempts lookback must cover the WORST CASE interval (base +
	// jitter), never just this call's own random draw, or a large-jitter
	// draw could read a lookback window narrower than what Budget actually
	// needs to see.
	lookbackFloor := time.Duration(minIntervalSeconds+jitterSeconds) * time.Second

	attempts, err := campaignSendAttempts(ctx, tx, accountID, tiers, lookbackFloor, now)
	if err != nil {
		return Claim{}, false, err
	}
	var lastSend time.Time
	if len(attempts) > 0 {
		lastSend = attempts[len(attempts)-1]
	}

	decision := purecampaign.Budget(tiers, attempts, minInterval, lastSend, now)
	if !decision.Allowed {
		return Claim{}, false, tx.Commit(ctx)
	}

	// attempted_at is stamped from the caller's own now, not the column's
	// strftime('now') default — Budget's interval math must see the exact
	// same clock reading claim-time compared everything else against, or a
	// test-injected (or merely skewed) now could disagree with the DB's real
	// wall clock by enough to misjudge the very interval this row exists to
	// enforce.
	var logID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO campaign_send_log (account_id, campaign_id, recipient_id, outcome, attempted_at)
		VALUES ($1, $2, $3, 'failed', $4) RETURNING id`,
		accountID, chosen.campaignID, chosen.recipientID, now).Scan(&logID); err != nil {
		return Claim{}, false, wrap("insert campaign send log", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE campaign_recipients SET status = 'sending', attempts = attempts + 1, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND status = 'pending'`, chosen.recipientID)
	if err != nil {
		return Claim{}, false, wrap("claim campaign recipient", err)
	}
	if tag.RowsAffected() == 0 {
		// Lost a race — under this pool's single-writer serialization this
		// should be unreachable, but defensively: nothing to send this tick.
		return Claim{}, false, tx.Commit(ctx)
	}

	claim := Claim{
		LogID: logID, RecipientID: chosen.recipientID, CampaignID: chosen.campaignID, AccountID: accountID,
		Channel: chosen.channel, NormalizedIdentity: chosen.identity, Name: chosen.name,
		Attributes: decodeStringMap(chosen.attrsRaw), MessageBody: chosen.messageBody, Attempts: chosen.attempts + 1,
	}
	return claim, true, tx.Commit(ctx)
}

// FinalizeAttempt records a claimed attempt's outcome after the provider
// call returns. On RecipientSent it flips the pessimistic campaign_send_log
// row (written at claim time) to 'sent'; every other outcome leaves it
// 'failed', which is already correct — the ledger only needs to know an
// attempt happened and when, not its eventual disposition, and every
// non-sent disposition genuinely IS a failed attempt regardless of whether
// the recipient itself will be retried.
func (s *Store) FinalizeAttempt(ctx context.Context, p FinalizeAttemptParams) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if p.NewStatus == purecampaign.RecipientSent {
		if _, err := tx.Exec(ctx, `UPDATE campaign_send_log SET outcome = 'sent' WHERE id = $1`, p.LogID); err != nil {
			return wrap("finalize campaign send log", err)
		}
	}

	var chatArg, msgArg any
	if p.ChatID.Valid {
		chatArg = p.ChatID.UUID
	}
	if p.MessageID.Valid {
		msgArg = p.MessageID.UUID
	}
	if _, err := tx.Exec(ctx, `
		UPDATE campaign_recipients SET
			status = $2, failure_reason = $3,
			chat_id = COALESCE($4, chat_id), message_id = COALESCE($5, message_id),
			next_attempt_at = $6, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`,
		p.RecipientID, string(p.NewStatus), p.FailureReason, chatArg, msgArg, p.NextAttemptAt); err != nil {
		return wrap("finalize campaign recipient", err)
	}
	return tx.Commit(ctx)
}

// ReconcileStuckSending flips every recipient still 'sending' (a claim that
// committed but whose outcome was never recorded — the process crashed, or
// otherwise died, between the claim and the provider call returning) to
// 'failed' with a reason that says delivery is genuinely unknown. It is
// NEVER retried automatically: a duplicate send is worse than a missed one.
// Called once at the campaign Scheduler's boot.
func (s *Store) ReconcileStuckSending(ctx context.Context) (int, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE campaign_recipients SET status = 'failed', failure_reason = $1, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE status = 'sending'`, "interrupted — delivery unknown")
	if err != nil {
		return 0, wrap("reconcile stuck campaign sends", err)
	}
	return int(tag.RowsAffected()), nil
}

// PruneCampaignSendLog deletes every ledger row older than olderThan — a
// rolling OPERATIONAL ledger, not the permanent audit trail (that lives on
// campaign_recipients and campaign_events, neither touched here). See
// backend/campaign.MaxTierWindowSeconds' own doc comment for why the
// retention window this is called with must never be shorter than the
// widest configurable tier.
func (s *Store) PruneCampaignSendLog(ctx context.Context, olderThan time.Time) (int, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM campaign_send_log WHERE attempted_at < $1`, olderThan)
	if err != nil {
		return 0, wrap("prune campaign send log", err)
	}
	return int(tag.RowsAffected()), nil
}

// ---------------------------------------------------------------------------
// Runner support: account discovery, scheduled promotion, auto-pause, and
// the send-time chat resolution + outbound insert internal/campaign's
// Runner needs around ClaimNextRecipient/FinalizeAttempt.
// ---------------------------------------------------------------------------

// ListRunningCampaignAccounts returns the distinct accounts with at least one
// RUNNING campaign — the campaign Scheduler's own tick fan-out list.
func (s *Store) ListRunningCampaignAccounts(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT account_id FROM campaigns WHERE status = 'running'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanUUIDRows(rows)
	if err != nil {
		return nil, err
	}
	return out, rows.Err()
}

// RunningCampaignIDs returns every campaign currently in status 'running' —
// the Scheduler's own periodic "is this one actually done?" sweep, covering
// a campaign whose recipient list was empty (or fully suppressed) before its
// first claim ever ran: the post-send completion check in internal/campaign's
// Runner only fires as a SEND's side effect, so it can never catch that
// empty-from-the-start case on its own.
func (s *Store) RunningCampaignIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.Query(ctx, `SELECT id FROM campaigns WHERE status = 'running'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanUUIDRows(rows)
	if err != nil {
		return nil, err
	}
	return out, rows.Err()
}

// DueScheduledCampaigns returns every 'scheduled' campaign whose schedule_at
// has passed — see backend/campaign.CanTransition's own doc comment: "start"
// only ever produces draft->scheduled or draft->running; the campaign
// Scheduler is what promotes scheduled->running once schedule_at actually
// arrives.
func (s *Store) DueScheduledCampaigns(ctx context.Context, now time.Time) ([]uuid.UUID, error) {
	rows, err := s.db.Query(ctx, `SELECT id FROM campaigns WHERE status = 'scheduled' AND schedule_at IS NOT NULL AND schedule_at <= $1`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanUUIDRows(rows)
	if err != nil {
		return nil, err
	}
	return out, rows.Err()
}

func scanUUIDRows(rows *dbx.Rows) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// AutoPauseCampaignsForAccount pauses every RUNNING campaign on accountID —
// the campaign Scheduler's disconnect watch, called once accountID has been
// continuously disconnected past its own threshold, so a dead connection
// never keeps failing sends (and burning through the retry ladder) instead
// of just waiting for the operator to reconnect and resume. A manual pause
// via CampaignAccountSettings.Paused is a distinct, ACCOUNT-level switch —
// this instead moves each individual campaign's own status, exactly like an
// operator pressing "Pause" on every one of them, so resuming later is the
// normal per-campaign resume action.
func (s *Store) AutoPauseCampaignsForAccount(ctx context.Context, accountID uuid.UUID, reason string) (int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `SELECT id FROM campaigns WHERE account_id = $1 AND status = 'running'`, accountID)
	if err != nil {
		return 0, err
	}
	ids, err := scanUUIDRows(rows)
	rows.Close()
	if err != nil {
		return 0, err
	}

	detail := map[string]any{"reason": reason}
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `UPDATE campaigns SET status = 'paused', updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE id = $1`, id); err != nil {
			return 0, wrap("auto-pause campaign", err)
		}
		if err := insertCampaignEvent(ctx, tx, id, "auto_paused", uuid.NullUUID{}, detail); err != nil {
			return 0, err
		}
	}
	return len(ids), tx.Commit(ctx)
}

// ExistingChatForIdentity finds a chat already open on accountID whose
// contact matches identity, across any channel — the WARM-only send path's
// chat resolution (backend/campaign.ColdSendCapable is false): Telegram,
// Instagram, Messenger and WhatsApp Cloud campaigns require a recipient to
// already have a conversation (checked once at preview time via
// ApplyUnreachable, re-verified here since a campaign can run long after its
// recipients were saved). identity is matched against whichever of the
// chat's own external identifiers is shaped like it — its own
// external_conversation_ref (a Telegram chat id), its contact's
// external_contact_ref (WhatsApp Cloud's E.164 contact id; Instagram/
// Messenger's provider-scoped user id, when an operator pastes one
// directly), or its contact's phone number. WhatsApp/simulator campaigns
// never call this — they are cold-send capable and use FindOrCreateChat
// instead. ok is false when no match exists.
func (s *Store) ExistingChatForIdentity(ctx context.Context, accountID uuid.UUID, identity string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		SELECT id FROM inbox_chats_v
		WHERE account_id = $1
		  AND (external_conversation_ref = $2 OR external_contact_ref = $2 OR contact_phone_number = $2)
		LIMIT 1`, accountID, identity).Scan(&id)
	if errors.Is(err, dbx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

// UnreachableForWarmChannel is ExistingChatForIdentity batched across many
// identities in one query — the preview-time reachability check across a
// whole pasted/uploaded recipient list, which would otherwise run one round
// trip per row. Returns the subset of identities with NO existing chat on
// accountID; identities absent from the result ARE reachable.
func (s *Store) UnreachableForWarmChannel(ctx context.Context, accountID uuid.UUID, identities []string) (map[string]bool, error) {
	if len(identities) == 0 {
		return map[string]bool{}, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT v.value FROM json_each($2) v
		WHERE NOT EXISTS (
			SELECT 1 FROM inbox_chats_v c
			WHERE c.account_id = $1
			  AND (c.external_conversation_ref = v.value OR c.external_contact_ref = v.value OR c.contact_phone_number = v.value)
		)`, accountID, dbx.StringArray(identities))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// MarkChatCampaignOnly flags a freshly cold-send-created chat as
// chat_state='campaign' — hidden from the default inbox listing (see
// ListChatsForOrg's own doc comment) until the recipient replies (or the
// operator messages them out of band), at which point UpsertInbound
// promotes it back to 'open'. Only ever called by internal/campaign's
// Runner immediately after FindOrCreateChat reports a chat it just created:
// an EXISTING chat a warm-only channel campaign reuses already has its own
// real chat_state and must never be touched.
func (s *Store) MarkChatCampaignOnly(ctx context.Context, channel string, chatID uuid.UUID) error {
	table, err := chatsTableFor(channel)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `UPDATE `+table+` SET chat_state = 'campaign', updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE id = $1`, chatID)
	return wrap("mark chat campaign-only", err)
}

// InsertCampaignOutbound records one campaign send exactly like
// InsertAutomationOutbound (internal/store/automation.go) — sender_kind
// 'campaign', sender_user_id always NULL (no human sender), and unread_count
// deliberately left untouched: a campaign send is not an operator reading
// the conversation, so the badge must stay exactly as an operator would see
// it if they opened the chat right after.
func (s *Store) InsertCampaignOutbound(ctx context.Context, channel string, chatID, accountID uuid.UUID, body, preview string) (uuid.UUID, error) {
	msgTable, err := messagesTableFor(channel)
	if err != nil {
		return uuid.Nil, err
	}
	chatTable, err := chatsTableFor(channel)
	if err != nil {
		return uuid.Nil, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO `+msgTable+`
			(account_id, chat_id, direction, sender_kind, sender_user_id, message_kind, body, delivery_state, source, message_ts)
		VALUES ($1, $2, 'out', 'campaign', NULL, 'conversation', $3, 'queued', 'app', strftime('%Y-%m-%d %H:%M:%f','now'))
		RETURNING id`,
		accountID, chatID, body).Scan(&id); err != nil {
		return uuid.Nil, wrap("insert campaign outbound", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE `+chatTable+` SET last_message_at = strftime('%Y-%m-%d %H:%M:%f','now'), last_message_preview = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, chatID, preview); err != nil {
		return uuid.Nil, wrap("update aggregates", err)
	}
	return id, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

func decodeStringMap(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

func decodeStringSlice(raw string) []string {
	if raw == "" {
		return nil
	}
	var s []string
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil
	}
	return s
}

func decodeAnyMap(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
