package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yerassyldanay/xchats/backend/internal/autoresponse"
)

// chatLockSeed namespaces this package's chat-scoped advisory locks: a draft
// generation (WriteDraftSet), an auto-response send (SendAutoResponse), and
// an operator cancel (CancelPendingAutoResponseForChat) for the SAME chat
// take this lock as their first statement, so their critical sections can
// never execute concurrently. hashtextextended(text, seed) is this repo's
// existing advisory-lock idiom (internal/kbstore/draft.go's draftLockSeed);
// this seed just needs to differ from that one so the two never needlessly
// serialize against each other.
const chatLockSeed int64 = 1917

// lockChat takes a transaction-scoped advisory lock keyed on chatID.
// pg_advisory_xact_lock auto-releases at commit/rollback, so it can never
// leak on an early return.
func lockChat(ctx context.Context, tx pgx.Tx, chatID uuid.UUID) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, $2))`, chatID.String(), chatLockSeed)
	return err
}

// isUniqueViolation reports whether err is a 23505 on the named constraint
// (or any 23505 when constraint is "").
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && (constraint == "" || pgErr.ConstraintName == constraint)
	}
	return false
}

// ---------------------------------------------------------------------------
// Policy
// ---------------------------------------------------------------------------

const autoResponsePolicyCols = `enabled, reply_mode, fixed_text, timezone, delay_seconds, cooldown_seconds,
	skip_when_escalated, pause_when_assigned`

func scanAutoResponsePolicy(row pgx.Row) (autoresponse.Policy, error) {
	var p autoresponse.Policy
	err := row.Scan(&p.Enabled, &p.ReplyMode, &p.FixedText, &p.Timezone, &p.DelaySeconds, &p.CooldownSeconds,
		&p.SkipWhenEscalated, &p.PauseWhenAssigned)
	return p, err
}

// AutoResponsePolicy loads accountID's policy plus its windows. ErrNotFound
// means the account has never been configured — callers use
// autoresponse.DefaultPolicy() (disabled) in that case.
func (s *Store) AutoResponsePolicy(ctx context.Context, accountID uuid.UUID) (autoresponse.Policy, error) {
	p, err := scanAutoResponsePolicy(s.pool.QueryRow(ctx, `
		SELECT `+autoResponsePolicyCols+`
		FROM xchats.account_auto_response WHERE account_id = $1`, accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	windows, err := s.autoResponseWindows(ctx, accountID)
	if err != nil {
		return p, err
	}
	p.Windows = windows
	return p, nil
}

func (s *Store) autoResponseWindows(ctx context.Context, accountID uuid.UUID) ([]autoresponse.Window, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT weekday, starts_minute, ends_minute
		FROM xchats.account_auto_response_window
		WHERE account_id = $1
		ORDER BY weekday, starts_minute`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	windows := make([]autoresponse.Window, 0, 8)
	for rows.Next() {
		var w autoresponse.Window
		var weekday int
		if err := rows.Scan(&weekday, &w.StartMin, &w.EndMin); err != nil {
			return nil, err
		}
		w.Weekday = time.Weekday(weekday)
		windows = append(windows, w)
	}
	return windows, rows.Err()
}

// AutoResponsePoliciesForOrg loads every configured policy for orgID in two
// queries (policies, then their windows), keyed by account_id — GET /accounts
// needs this for every row without an N+1 query per account.
func (s *Store) AutoResponsePoliciesForOrg(ctx context.Context, orgID uuid.UUID) (map[uuid.UUID]autoresponse.Policy, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT account_id, `+autoResponsePolicyCols+`
		FROM xchats.account_auto_response WHERE organization_id = $1`, orgID)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]autoresponse.Policy{}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		var p autoresponse.Policy
		if err := rows.Scan(&id, &p.Enabled, &p.ReplyMode, &p.FixedText, &p.Timezone, &p.DelaySeconds, &p.CooldownSeconds,
			&p.SkipWhenEscalated, &p.PauseWhenAssigned); err != nil {
			rows.Close()
			return nil, err
		}
		p.Windows = []autoresponse.Window{}
		out[id] = p
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}

	wrows, err := s.pool.Query(ctx, `
		SELECT account_id, weekday, starts_minute, ends_minute
		FROM xchats.account_auto_response_window
		WHERE account_id = ANY($1)
		ORDER BY account_id, weekday, starts_minute`, ids)
	if err != nil {
		return nil, err
	}
	defer wrows.Close()
	for wrows.Next() {
		var id uuid.UUID
		var w autoresponse.Window
		var weekday int
		if err := wrows.Scan(&id, &weekday, &w.StartMin, &w.EndMin); err != nil {
			return nil, err
		}
		w.Weekday = time.Weekday(weekday)
		p := out[id]
		p.Windows = append(p.Windows, w)
		out[id] = p
	}
	return out, wrows.Err()
}

// UpsertAutoResponse writes accountID's policy in one transaction: upsert the
// policy row, then fully replace its windows (delete + re-insert — idempotent
// regardless of what was there before). Disabling the policy also cancels any
// pending auto-response job on the account's chats: an operator who turns
// auto-response off must not have it fire on a message that was already
// armed under the old policy.
func (s *Store) UpsertAutoResponse(ctx context.Context, accountID, orgID, userID uuid.UUID, policy autoresponse.Policy) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var updatedBy any
	if userID != uuid.Nil {
		updatedBy = userID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO xchats.account_auto_response
			(account_id, organization_id, enabled, reply_mode, fixed_text, timezone,
			 delay_seconds, cooldown_seconds, skip_when_escalated, pause_when_assigned,
			 updated_by_user_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
		ON CONFLICT (account_id) DO UPDATE SET
			enabled             = EXCLUDED.enabled,
			reply_mode          = EXCLUDED.reply_mode,
			fixed_text          = EXCLUDED.fixed_text,
			timezone            = EXCLUDED.timezone,
			delay_seconds       = EXCLUDED.delay_seconds,
			cooldown_seconds    = EXCLUDED.cooldown_seconds,
			skip_when_escalated = EXCLUDED.skip_when_escalated,
			pause_when_assigned = EXCLUDED.pause_when_assigned,
			updated_by_user_id  = EXCLUDED.updated_by_user_id,
			updated_at          = now()`,
		accountID, orgID, policy.Enabled, policy.ReplyMode, policy.FixedText, policy.Timezone,
		policy.DelaySeconds, policy.CooldownSeconds, policy.SkipWhenEscalated, policy.PauseWhenAssigned,
		updatedBy); err != nil {
		return wrap("upsert auto-response policy", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM xchats.account_auto_response_window WHERE account_id = $1`, accountID); err != nil {
		return wrap("clear auto-response windows", err)
	}
	for _, w := range autoresponse.Normalize(policy.Windows) {
		if _, err := tx.Exec(ctx, `
			INSERT INTO xchats.account_auto_response_window (account_id, weekday, starts_minute, ends_minute)
			VALUES ($1, $2, $3, $4)`, accountID, int(w.Weekday), w.StartMin, w.EndMin); err != nil {
			return wrap("insert auto-response window", err)
		}
	}

	if !policy.Enabled {
		if _, err := tx.Exec(ctx, `
			UPDATE xchats.auto_response_jobs SET state = 'cancelled', cancel_reason = 'policy_off', updated_at = now()
			WHERE account_id = $1 AND state = 'pending'`, accountID); err != nil {
			return wrap("cancel pending auto-response jobs", err)
		}
	}

	return tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

// AutoResponseJob is one auto_response_jobs row.
type AutoResponseJob struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	AccountID        uuid.UUID
	ChatID           uuid.UUID
	Channel          string
	TriggerMessageID uuid.UUID
	TriggerArrivedAt time.Time
	DueAt            time.Time
	DraftID          uuid.NullUUID
	State            string
	CancelReason     string
}

func scanAutoResponseJob(row pgx.Row) (AutoResponseJob, error) {
	var j AutoResponseJob
	err := row.Scan(&j.ID, &j.OrganizationID, &j.AccountID, &j.ChatID, &j.Channel,
		&j.TriggerMessageID, &j.TriggerArrivedAt, &j.DueAt, &j.DraftID, &j.State, &j.CancelReason)
	return j, err
}

// ArmAutoResponseJobArg is the input to ArmAutoResponseJob.
type ArmAutoResponseJobArg struct {
	OrganizationID   uuid.UUID
	AccountID        uuid.UUID
	ChatID           uuid.UUID
	Channel          string
	TriggerMessageID uuid.UUID
	TriggerArrivedAt time.Time
	DraftID          uuid.UUID
	DelaySeconds     int
}

// ArmAutoResponseJob upserts the chat's pending auto-response job in one
// statement: Postgres infers the partial unique index
// auto_response_jobs_chat_pending_uq from the ON CONFLICT predicate, so a
// newer debounced draft re-targets the SAME pending row (its due_at/
// draft_id/trigger) instead of leaving a stale duplicate behind — no
// separate cancel-then-insert, no first_armed_at bookkeeping, no pushback
// cap (that cap already lives on the debounce row).
// auto_response_jobs_trigger_uq still fences a redelivery of an
// already-answered message; on THAT conflict the arm is a no-op.
func (s *Store) ArmAutoResponseJob(ctx context.Context, arg ArmAutoResponseJobArg) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO xchats.auto_response_jobs
			(organization_id, account_id, chat_id, channel, trigger_message_id,
			 trigger_arrived_at, due_at, draft_id, state)
		VALUES ($1, $2, $3, $4, $5, $6, now() + make_interval(secs => $7), $8, 'pending')
		ON CONFLICT (chat_id) WHERE state = 'pending' DO UPDATE SET
			trigger_message_id = EXCLUDED.trigger_message_id,
			trigger_arrived_at = EXCLUDED.trigger_arrived_at,
			draft_id           = EXCLUDED.draft_id,
			due_at             = EXCLUDED.due_at,
			updated_at         = now()`,
		arg.OrganizationID, arg.AccountID, arg.ChatID, arg.Channel, arg.TriggerMessageID,
		arg.TriggerArrivedAt, arg.DelaySeconds, arg.DraftID)
	if err != nil && isUniqueViolation(err, "auto_response_jobs_trigger_uq") {
		return nil
	}
	return err
}

// ClaimDueAutoResponseJobs claims up to limit pending jobs whose due_at has
// passed. A job whose due_at is older than maxDeliveryLag is cancelled
// ("too_late") in the SAME statement instead of claimed, so a deploy spanning
// the window doesn't flush a backlog of stale replies the moment it comes
// back up — the caller distinguishes the two outcomes via State.
func (s *Store) ClaimDueAutoResponseJobs(ctx context.Context, limit int, maxDeliveryLag time.Duration) ([]AutoResponseJob, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE xchats.auto_response_jobs
		SET state = CASE WHEN now() - due_at > make_interval(secs => $2) THEN 'cancelled' ELSE 'claimed' END,
		    cancel_reason = CASE WHEN now() - due_at > make_interval(secs => $2) THEN 'too_late' ELSE cancel_reason END,
		    updated_at = now()
		WHERE id IN (
			SELECT id FROM xchats.auto_response_jobs
			WHERE state = 'pending' AND due_at <= now()
			ORDER BY due_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, organization_id, account_id, chat_id, channel, trigger_message_id,
		          trigger_arrived_at, due_at, draft_id, state, cancel_reason`,
		limit, maxDeliveryLag.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AutoResponseJob
	for rows.Next() {
		j, err := scanAutoResponseJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// CancelAutoResponseJob marks a pending or claimed job cancelled with reason.
// A terminal job (already sent/cancelled/failed) is left alone.
func (s *Store) CancelAutoResponseJob(ctx context.Context, jobID uuid.UUID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xchats.auto_response_jobs SET state = 'cancelled', cancel_reason = $2, updated_at = now()
		WHERE id = $1 AND state IN ('pending', 'claimed')`, jobID, reason)
	return err
}

// FailAutoResponseJob marks a job failed with the send error. Never retried —
// the suggestion stays in the panel for the operator, which is the fail-safe
// direction.
func (s *Store) FailAutoResponseJob(ctx context.Context, jobID uuid.UUID, lastError string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xchats.auto_response_jobs SET state = 'failed', last_error = $2, updated_at = now()
		WHERE id = $1 AND state IN ('pending', 'claimed')`, jobID, lastError)
	return err
}

// CancelPendingAutoResponseForChat cancels chatID's pending auto-response job,
// if any, under the same chat advisory lock SendAutoResponse takes — so this
// cancel and a concurrent sweeper claim cannot interleave. Called from both
// operator send paths (handleApprove, handleSendMessage) before the send
// itself proceeds.
func (s *Store) CancelPendingAutoResponseForChat(ctx context.Context, chatID uuid.UUID, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockChat(ctx, tx, chatID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE xchats.auto_response_jobs SET state = 'cancelled', cancel_reason = $2, updated_at = now()
		WHERE chat_id = $1 AND state = 'pending'`, chatID, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CancelPendingAutoResponseForAccount cancels every pending auto-response job
// across accountID's chats — used when the account itself is deleted or
// disconnected, so it can never auto-send after the fact.
func (s *Store) CancelPendingAutoResponseForAccount(ctx context.Context, accountID uuid.UUID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xchats.auto_response_jobs SET state = 'cancelled', cancel_reason = $2, updated_at = now()
		WHERE account_id = $1 AND state = 'pending'`, accountID, reason)
	return err
}

// ReapStaleAutoResponseClaims marks 'failed' every job 'claimed' longer than
// olderThan ago — a worker process that died mid-send. claimed implies
// nothing was written (SendAutoResponse's guarded steps mean a claimed job
// that never reached 'sent' produced no customer message), so the reaper
// only ever needs to flip a state, never undo one.
func (s *Store) ReapStaleAutoResponseClaims(ctx context.Context, olderThan time.Duration) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE xchats.auto_response_jobs
		SET state = 'failed', last_error = 'stale claim', updated_at = now()
		WHERE state = 'claimed' AND updated_at < now() - make_interval(secs => $1)`,
		olderThan.Seconds())
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// PurgeAutoResponseJobs deletes terminal (sent/cancelled/failed) jobs older
// than retention — routine cleanup, not a correctness requirement.
func (s *Store) PurgeAutoResponseJobs(ctx context.Context, retention time.Duration) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM xchats.auto_response_jobs
		WHERE state IN ('sent', 'cancelled', 'failed') AND updated_at < now() - make_interval(secs => $1)`,
		retention.Seconds())
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ---------------------------------------------------------------------------
// Sending
// ---------------------------------------------------------------------------

// ErrAutoResponseAborted is returned by SendAutoResponse when a guarded step
// inside its transaction matched zero rows — someone else (an operator, a
// redelivery) already acted, so nothing was sent. The caller
// (worker.fireAutoResponseJob) treats this like any other send failure: the
// job is marked 'failed' and is never retried.
var ErrAutoResponseAborted = errors.New("auto-response: aborted, guard lost")

// SendAutoResponseArg is the input to SendAutoResponse.
type SendAutoResponseArg struct {
	JobID            uuid.UUID
	ChatID           uuid.UUID
	AccountID        uuid.UUID
	Channel          string
	TriggerArrivedAt time.Time
	ReplyMode        string // "ai" | "fixed"
	DraftID          uuid.UUID
	FixedText        string // fixed mode only
}

// SendAutoResponse is the single, guarded transaction that turns an armed
// auto-response job into a real outbound customer message. Every step is a
// conditional write; a zero-row result at any of them returns
// ErrAutoResponseAborted and rolls the whole transaction back, so a job can
// never send twice and can never send after someone else already replied.
// "Quiet since armed" is checked HERE, inside the transaction — a pre-check
// outside it would just move the race, not close it. Returns the sent
// message id and its final text (the authoritative draft_text/fixed_text
// this transaction just committed to), so the caller can publish the
// outbound-send task without a second, possibly-racy read.
//
// resetUnread is deliberately NOT passed through to insertOutboundTx as true:
// an unattended auto-reply must not clear the chat's unread badge, or the
// operator arrives to a conversation that silently looks untouched overnight.
func (s *Store) SendAutoResponse(ctx context.Context, arg SendAutoResponseArg) (uuid.UUID, string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer tx.Rollback(ctx)

	if err := lockChat(ctx, tx, arg.ChatID); err != nil {
		return uuid.Nil, "", err
	}

	tag, err := tx.Exec(ctx, `
		UPDATE xchats.auto_response_jobs SET state = 'sent', updated_at = now()
		WHERE id = $1 AND state = 'claimed'
		  AND NOT EXISTS (
		      SELECT 1 FROM xchats.inbox_messages_v
		      WHERE chat_id = $2 AND direction = 'out' AND created_at > $3
		  )`, arg.JobID, arg.ChatID, arg.TriggerArrivedAt)
	if err != nil {
		return uuid.Nil, "", err
	}
	if tag.RowsAffected() == 0 {
		return uuid.Nil, "", ErrAutoResponseAborted
	}

	text := arg.FixedText
	if arg.ReplyMode == autoresponse.ReplyModeAI {
		var draftText string
		err := tx.QueryRow(ctx, `
			UPDATE xchats.ai_drafts SET draft_state = 'sent', updated_at = now()
			WHERE id = $1 AND draft_state = 'suggested'
			RETURNING draft_text`, arg.DraftID).Scan(&draftText)
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", ErrAutoResponseAborted
		}
		if err != nil {
			return uuid.Nil, "", err
		}
		text = draftText
	} else {
		// fixed mode: supersede (not "sent") the still-pending suggestion, so a
		// stray click on the now-stale card in the panel cannot send it too.
		tag, err := tx.Exec(ctx, `
			UPDATE xchats.ai_drafts SET draft_state = 'superseded', updated_at = now()
			WHERE id = $1 AND draft_state = 'suggested'`, arg.DraftID)
		if err != nil {
			return uuid.Nil, "", err
		}
		if tag.RowsAffected() == 0 {
			return uuid.Nil, "", ErrAutoResponseAborted
		}
	}

	msgID, err := insertOutboundTx(ctx, tx, arg.Channel, arg.ChatID, arg.AccountID,
		"ai", uuid.NullUUID{}, "conversation", text, previewText(text), false)
	if err != nil {
		return uuid.Nil, "", err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE xchats.auto_response_jobs SET sent_message_id = $2, updated_at = now() WHERE id = $1`,
		arg.JobID, msgID); err != nil {
		return uuid.Nil, "", err
	}
	if arg.ReplyMode == autoresponse.ReplyModeAI {
		if _, err := tx.Exec(ctx, `
			UPDATE xchats.ai_drafts SET sent_message_id = $2, updated_at = now() WHERE id = $1`,
			arg.DraftID, msgID); err != nil {
			return uuid.Nil, "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, "", err
	}
	return msgID, text, nil
}

// HasOutboundSince reports whether chatID has any outbound message created
// after since — the auto-response cooldown guard ("has anyone, operator or
// otherwise, replied recently").
func (s *Store) HasOutboundSince(ctx context.Context, chatID uuid.UUID, since time.Time) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM xchats.inbox_messages_v
			WHERE chat_id = $1 AND direction = 'out' AND created_at > $2
		)`, chatID, since).Scan(&exists)
	return exists, err
}

func previewText(text string) string {
	r := []rune(text)
	if len(r) > 120 {
		return string(r[:120])
	}
	return text
}
