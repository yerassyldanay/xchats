package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ---------------------------------------------------------------------------
// Channel automation settings + recurring UTC schedule windows
// ---------------------------------------------------------------------------
// account_id carries no foreign key, on purpose: an account is either a
// wa_accounts or tg_accounts row and a single FK cannot express an
// either/or reference — see 0003_ai_engine.up.sql's file header for the
// identical reasoning behind ai_drafts.chat_id. Ownership is enforced in
// internal/httpapi instead.

// AutomationSettings is one channel's automation mode and optional
// wait-time override. A missing row (AutomationSettingsForAccount's default
// return) means the channel is on its implicit default: automation.
// DefaultMode ("suggestions"), no override.
type AutomationSettings struct {
	AccountID           uuid.UUID
	Mode                string
	WaitSecondsOverride *int
	UpdatedAt           time.Time
}

// AutomationWindow is one recurring UTC weekday/time-of-day range, stored.
type AutomationWindow struct {
	ID          uuid.UUID
	AccountID   uuid.UUID
	Weekday     int
	StartMinute int
	EndMinute   int
}

// AutomationWindowInput is one window as accepted from an API caller —
// AutomationWindow minus the generated id/account, for SetAutomationSettings.
type AutomationWindowInput struct {
	Weekday     int
	StartMinute int
	EndMinute   int
}

// defaultAutomationSettings is what AutomationSettingsForAccount (and the
// bulk form) returns for an account with no row yet.
func defaultAutomationSettings(accountID uuid.UUID) AutomationSettings {
	return AutomationSettings{AccountID: accountID, Mode: "suggestions"}
}

// AutomationSettingsForAccount returns accountID's automation settings, or
// the implicit default ("suggestions", no override) if it has never been
// configured — never ErrNotFound, since "unconfigured" is itself a valid,
// meaningful state here.
func (s *Store) AutomationSettingsForAccount(ctx context.Context, accountID uuid.UUID) (AutomationSettings, error) {
	var out AutomationSettings
	err := s.db.QueryRow(ctx, `
		SELECT account_id, mode, wait_seconds_override, updated_at
		FROM automation_settings WHERE account_id = $1`, accountID).
		Scan(&out.AccountID, &out.Mode, &out.WaitSecondsOverride, &out.UpdatedAt)
	if errors.Is(err, dbx.ErrNoRows) {
		return defaultAutomationSettings(accountID), nil
	}
	return out, err
}

// AutomationSettingsForAccounts bulk-loads settings for exactly the given
// accounts (GET /accounts' "effective automation settings" needs one round
// trip, not N) — an account with no row is simply absent from the returned
// map; callers apply defaultAutomationSettings themselves (mirrors how a
// missing map entry is handled everywhere else in this codebase, rather
// than pre-filling defaults server-side for ids the caller might not even
// own).
func (s *Store) AutomationSettingsForAccounts(ctx context.Context, accountIDs []uuid.UUID) (map[uuid.UUID]AutomationSettings, error) {
	out := make(map[uuid.UUID]AutomationSettings, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT account_id, mode, wait_seconds_override, updated_at
		FROM automation_settings
		WHERE account_id IN (SELECT value FROM json_each($1))`, dbx.UUIDArray(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var a AutomationSettings
		if err := rows.Scan(&a.AccountID, &a.Mode, &a.WaitSecondsOverride, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out[a.AccountID] = a
	}
	return out, rows.Err()
}

// AutomationWindowsForAccounts bulk-loads schedule windows for the given
// accounts, grouped by account id.
func (s *Store) AutomationWindowsForAccounts(ctx context.Context, accountIDs []uuid.UUID) (map[uuid.UUID][]AutomationWindow, error) {
	out := make(map[uuid.UUID][]AutomationWindow, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, account_id, weekday, start_minute, end_minute
		FROM automation_schedule_windows
		WHERE account_id IN (SELECT value FROM json_each($1))
		ORDER BY weekday, start_minute`, dbx.UUIDArray(accountIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var w AutomationWindow
		if err := rows.Scan(&w.ID, &w.AccountID, &w.Weekday, &w.StartMinute, &w.EndMinute); err != nil {
			return nil, err
		}
		out[w.AccountID] = append(out[w.AccountID], w)
	}
	return out, rows.Err()
}

// AutomationWindowsForAccount returns one account's schedule windows.
func (s *Store) AutomationWindowsForAccount(ctx context.Context, accountID uuid.UUID) ([]AutomationWindow, error) {
	m, err := s.AutomationWindowsForAccounts(ctx, []uuid.UUID{accountID})
	if err != nil {
		return nil, err
	}
	return m[accountID], nil
}

// SetAutomationSettings replaces accountID's mode, wait override, and full
// schedule-window set in one transaction — windows are always a complete
// replace (delete-and-reinsert), matching the "one save = the whole
// schedule" shape the dialog presents. Switching to "off" also invalidates
// this account's not-yet-claimed pending work: every 'pending' debounce job
// and dispatch job for its chats is deleted, so a channel switched off does
// not go on to generate (or auto-send) a draft that was already queued up
// before the switch. A dispatch job already 'processing' (a live goroutine
// mid-generation) is deliberately left alone here; its own mode and atomic
// send rechecks are the defense-in-depth layer that catches it instead.
func (s *Store) SetAutomationSettings(ctx context.Context, accountID uuid.UUID, mode string, waitOverride *int, windows []AutomationWindowInput) (AutomationSettings, []AutomationWindow, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return AutomationSettings{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var out AutomationSettings
	if err := tx.QueryRow(ctx, `
		INSERT INTO automation_settings (account_id, mode, wait_seconds_override)
		VALUES ($1, $2, $3)
		ON CONFLICT (account_id) DO UPDATE SET
			mode = excluded.mode,
			wait_seconds_override = excluded.wait_seconds_override,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		RETURNING account_id, mode, wait_seconds_override, updated_at`,
		accountID, mode, waitOverride).
		Scan(&out.AccountID, &out.Mode, &out.WaitSecondsOverride, &out.UpdatedAt); err != nil {
		return AutomationSettings{}, nil, wrap("upsert automation settings", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM automation_schedule_windows WHERE account_id = $1`, accountID); err != nil {
		return AutomationSettings{}, nil, wrap("clear automation windows", err)
	}
	windowsOut := make([]AutomationWindow, 0, len(windows))
	for _, w := range windows {
		var row AutomationWindow
		if err := tx.QueryRow(ctx, `
			INSERT INTO automation_schedule_windows (account_id, weekday, start_minute, end_minute)
			VALUES ($1, $2, $3, $4)
			RETURNING id, account_id, weekday, start_minute, end_minute`,
			accountID, w.Weekday, w.StartMinute, w.EndMinute).
			Scan(&row.ID, &row.AccountID, &row.Weekday, &row.StartMinute, &row.EndMinute); err != nil {
			return AutomationSettings{}, nil, wrap("insert automation window", err)
		}
		windowsOut = append(windowsOut, row)
	}

	if mode == "off" {
		if _, err := tx.Exec(ctx, `DELETE FROM automation_debounce_jobs WHERE account_id = $1 AND status = 'pending'`, accountID); err != nil {
			return AutomationSettings{}, nil, wrap("invalidate pending debounce jobs", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM automation_dispatch_jobs WHERE account_id = $1 AND status = 'pending'`, accountID); err != nil {
			return AutomationSettings{}, nil, wrap("invalidate pending dispatch jobs", err)
		}
	}

	return out, windowsOut, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Durable debounce jobs
// ---------------------------------------------------------------------------

// ArmDebounce resets chatID's debounce deadline to deadline and bumps its
// burst version, creating the row on the first call for a chat. There is no
// maximum burst cap: every call — however many arrive back to back — simply
// pushes deadline_at forward and increments burst_version again. Called only
// once the caller has already confirmed the owning account's mode is not
// "off" (internal/automation.Scheduler.OnInboundMessage does that check).
func (s *Store) ArmDebounce(ctx context.Context, chatID, accountID uuid.UUID, channel string, deadline time.Time) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO automation_debounce_jobs (chat_id, account_id, channel, deadline_at, burst_version, status)
		VALUES ($1, $2, $3, $4, 1, 'pending')
		ON CONFLICT (chat_id) DO UPDATE SET
			account_id = excluded.account_id,
			channel = excluded.channel,
			deadline_at = excluded.deadline_at,
			burst_version = automation_debounce_jobs.burst_version + 1,
			status = 'pending',
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`,
		chatID, accountID, channel, deadline)
	return err
}

// ClaimDueDispatchJobs atomically claims due debounce rows and creates their
// durable dispatch jobs in the same transaction. If any dispatch insert
// fails, the claim is rolled back too, so no burst can become permanently
// stranded between the two tables. A newer inbound message still re-arms
// the debounce row with a higher burst version and is claimed independently.
func (s *Store) ClaimDueDispatchJobs(ctx context.Context, now time.Time, limit int) ([]DispatchJob, error) {
	if limit <= 0 {
		limit = 100
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT chat_id FROM automation_debounce_jobs
		WHERE status = 'pending' AND deadline_at <= $1
		ORDER BY deadline_at
		LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	if len(ids) == 0 {
		return nil, tx.Commit(ctx)
	}

	claimed, err := tx.Query(ctx, `
		UPDATE automation_debounce_jobs SET status = 'claimed', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE status = 'pending' AND chat_id IN (SELECT value FROM json_each($1))
		RETURNING chat_id, account_id, channel, burst_version`,
		dbx.UUIDArray(ids))
	if err != nil {
		return nil, err
	}
	type dueDebounce struct {
		chatID       uuid.UUID
		accountID    uuid.UUID
		channel      string
		burstVersion int64
	}
	var due []dueDebounce
	for claimed.Next() {
		var d dueDebounce
		if err := claimed.Scan(&d.chatID, &d.accountID, &d.channel, &d.burstVersion); err != nil {
			_ = claimed.Close()
			return nil, err
		}
		due = append(due, d)
	}
	if err := claimed.Err(); err != nil {
		_ = claimed.Close()
		return nil, err
	}
	if err := claimed.Close(); err != nil {
		return nil, err
	}

	out := make([]DispatchJob, 0, len(due))
	for _, d := range due {
		var j DispatchJob
		if err := tx.QueryRow(ctx, `
			INSERT INTO automation_dispatch_jobs (chat_id, account_id, channel, burst_version, status)
			VALUES ($1, $2, $3, $4, 'pending')
			RETURNING id, chat_id, account_id, channel, burst_version, status, attempts, last_error`,
			d.chatID, d.accountID, d.channel, d.burstVersion).
			Scan(&j.ID, &j.ChatID, &j.AccountID, &j.Channel, &j.BurstVersion, &j.Status, &j.Attempts, &j.LastError); err != nil {
			return nil, wrap("promote debounce to dispatch job", err)
		}
		out = append(out, j)
	}
	return out, tx.Commit(ctx)
}

// CurrentBurstVersion returns chatID's current debounce burst version, or 0
// if the chat has no debounce row (never armed, or invalidated by an "off"
// switch) — 0 never matches a real dispatch job's version (ArmDebounce's
// first INSERT always starts at 1), so a version-gated write against a
// vanished row is correctly treated as stale.
func (s *Store) CurrentBurstVersion(ctx context.Context, chatID uuid.UUID) (int64, error) {
	var v int64
	err := s.db.QueryRow(ctx, `SELECT burst_version FROM automation_debounce_jobs WHERE chat_id = $1`, chatID).Scan(&v)
	if errors.Is(err, dbx.ErrNoRows) {
		return 0, nil
	}
	return v, err
}

// ---------------------------------------------------------------------------
// Durable auto-reply dispatch jobs
// ---------------------------------------------------------------------------

// DispatchJob is one claimed debounce burst's durable "generate, then maybe
// send" row.
type DispatchJob struct {
	ID           uuid.UUID
	ChatID       uuid.UUID
	AccountID    uuid.UUID
	Channel      string
	BurstVersion int64
	Status       string
	Attempts     int
	LastError    string
}

// CreateDispatchJob durably records a dispatch job for callers that already
// have a burst version. It is idempotent per (chat, burst): duplicate calls
// return the existing id instead of creating two independently executable
// rows. Scheduler promotion uses ClaimDueDispatchJobs so its claim and
// insert are atomic.
func (s *Store) CreateDispatchJob(ctx context.Context, chatID, accountID uuid.UUID, channel string, version int64) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRow(ctx, `
		INSERT INTO automation_dispatch_jobs (chat_id, account_id, channel, burst_version, status)
		VALUES ($1, $2, $3, $4, 'pending')
		ON CONFLICT (chat_id, burst_version) DO UPDATE SET
			account_id = excluded.account_id,
			channel = excluded.channel
		RETURNING id`, chatID, accountID, channel, version).Scan(&id)
	return id, err
}

// DispatchJobByID reloads a dispatch job fresh — the queue payload carries
// only the id, exactly like AIDraftTask carries only a chat id, so every
// other field is always read current at execution time rather than trusted
// stale from enqueue time.
func (s *Store) DispatchJobByID(ctx context.Context, id uuid.UUID) (DispatchJob, error) {
	var j DispatchJob
	err := s.db.QueryRow(ctx, `
		SELECT id, chat_id, account_id, channel, burst_version, status, attempts, last_error
		FROM automation_dispatch_jobs WHERE id = $1`, id).
		Scan(&j.ID, &j.ChatID, &j.AccountID, &j.Channel, &j.BurstVersion, &j.Status, &j.Attempts, &j.LastError)
	if errors.Is(err, dbx.ErrNoRows) {
		return j, ErrNotFound
	}
	return j, err
}

// MarkDispatchProcessing flips a dispatch job to 'processing' and stamps
// updated_at — what RecoverableDispatchJobs' stuck-cutoff is measured
// against, so a crash mid-generation is detectable and recoverable on the
// next boot.
func (s *Store) MarkDispatchProcessing(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE automation_dispatch_jobs SET status = 'processing', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, id)
	return err
}

// ClaimDispatchJob grants one worker ownership by atomically moving a job
// from pending to processing. A duplicate queue delivery sees false and
// exits without generating or sending a second response.
func (s *Store) ClaimDispatchJob(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE automation_dispatch_jobs SET status = 'processing', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND status = 'pending'`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// DeleteDispatchJob removes a settled dispatch job — success, a stale
// (superseded) discard, or a permanent give-up all end the same way: there
// is no terminal status worth keeping a row around for.
func (s *Store) DeleteDispatchJob(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `DELETE FROM automation_dispatch_jobs WHERE id = $1`, id)
	return err
}

// RecordDispatchFailure increments a dispatch job's attempt counter and
// records the error, returning the new attempt count so the caller can
// decide whether to give up (delete) or leave it for the next recovery pass.
func (s *Store) RecordDispatchFailure(ctx context.Context, id uuid.UUID, cause string) (int, error) {
	var attempts int
	err := s.db.QueryRow(ctx, `
		UPDATE automation_dispatch_jobs SET
			attempts = attempts + 1,
			last_error = $2,
			status = 'pending',
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1
		RETURNING attempts`, id, cause).Scan(&attempts)
	return attempts, err
}

// RecoverableDispatchJobs claims dispatch jobs for a boot-time or periodic
// re-publish. Pending jobs must be older than stuckSince, so a job just
// enqueued by claimDue is not immediately delivered twice. Stale processing
// jobs are first returned to pending; selected rows have their recovery
// timestamp refreshed so another recovery tick does not enqueue them again
// before a worker starts. The worker still acquires final ownership through
// ClaimDispatchJob.
func (s *Store) RecoverableDispatchJobs(ctx context.Context, stuckSince time.Time, limit int) ([]DispatchJob, error) {
	if limit <= 0 {
		limit = 100
	}
	// updated_at is produced by SQLite's UTC strftime default. Bind the
	// cutoff in the identical representation so comparisons do not depend
	// on database/sql's time.Time serialization or the host time zone.
	cutoff := stuckSince.UTC().Format("2006-01-02 15:04:05.000")
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE automation_dispatch_jobs SET status = 'pending'
		WHERE status = 'processing' AND updated_at <= $1`, cutoff); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		UPDATE automation_dispatch_jobs
		SET updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id IN (
			SELECT id FROM automation_dispatch_jobs
			WHERE status = 'pending' AND updated_at <= $1
			ORDER BY updated_at
			LIMIT $2
		)
		RETURNING id, chat_id, account_id, channel, burst_version, status, attempts, last_error`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []DispatchJob
	for rows.Next() {
		var j DispatchJob
		if err := rows.Scan(&j.ID, &j.ChatID, &j.AccountID, &j.Channel, &j.BurstVersion, &j.Status, &j.Attempts, &j.LastError); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Version-gated draft persist + auto-send claim
// ---------------------------------------------------------------------------

// WriteDraftSetIfVersionCurrent behaves like WriteDraftSet, except the write
// is skipped — discarded, not an error — if chatID's live burst_version no
// longer equals expectedVersion at write time: a newer inbound message
// arrived (and re-armed the debounce, bumping the version) while this
// generation was still running. written=false on a stale discard;
// written=true on a normal persist. The check and the write happen in the
// same transaction, so no third writer can land between them.
func (s *Store) WriteDraftSetIfVersionCurrent(ctx context.Context, channel string, chatID uuid.UUID, expectedVersion int64, trigger uuid.NullUUID, opts []DraftOption) (drafts []Draft, written bool, err error) {
	if channel == "" {
		channel = "whatsapp"
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current int64
	verr := tx.QueryRow(ctx, `SELECT burst_version FROM automation_debounce_jobs WHERE chat_id = $1`, chatID).Scan(&current)
	if verr != nil && !errors.Is(verr, dbx.ErrNoRows) {
		return nil, false, verr
	}
	// dbx.ErrNoRows leaves current at its zero value (0), which never
	// matches a real expectedVersion (>=1) — correctly treated as stale.
	if current != expectedVersion {
		return nil, false, tx.Commit(ctx) // nothing written; not an error
	}

	if _, err := tx.Exec(ctx, `
		UPDATE ai_drafts SET draft_state='superseded', updated_at=strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE chat_id = $1 AND draft_state='suggested'`, chatID); err != nil {
		return nil, false, err
	}
	out := make([]Draft, 0, len(opts))
	for _, o := range opts {
		var d Draft
		if err := tx.QueryRow(ctx, `
			INSERT INTO ai_drafts (chat_id, channel, trigger_message_id, option_ordinal, draft_text, reply_language, confidence, escalate, escalation_reason, draft_state)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'suggested')
			RETURNING `+draftCols,
			chatID, channel, trigger, o.Ordinal, o.Text, o.ReplyLanguage, o.Confidence, o.Escalate, o.EscalationReason).
			Scan(scanDraftDst(&d)...); err != nil {
			return nil, false, err
		}
		out = append(out, d)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// ClaimDraftForAutoSend is scheduled_auto's guarded auto-send: in one
// transaction it re-reads the owning account's mode, confirms no human
// (operator or the WhatsApp number's own owner) has sent a message in this
// chat since expectedTrigger, confirms expectedTrigger is still the chat's
// latest inbound message (no newer customer message slipped in), and only
// then attempts the same conditional claim ClaimDraft uses
// (draft_state='suggested' -> 'sent'). Any failed check — including the
// claim losing a race to a concurrent manual approve — returns sent=false
// with no error: the fallback is simply to leave the draft visible exactly
// as though scheduled_auto had decided not to auto-send, never to retry or
// surface a failure.
func (s *Store) ClaimDraftForAutoSend(ctx context.Context, draftID, chatID, accountID uuid.UUID, expectedTrigger uuid.NullUUID) (Draft, bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Draft{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var mode string
	merr := tx.QueryRow(ctx, `SELECT mode FROM automation_settings WHERE account_id = $1`, accountID).Scan(&mode)
	switch {
	case errors.Is(merr, dbx.ErrNoRows):
		mode = "suggestions" // the implicit default is never scheduled_auto
	case merr != nil:
		return Draft{}, false, merr
	}
	if mode != "scheduled_auto" {
		return Draft{}, false, tx.Commit(ctx)
	}

	if !expectedTrigger.Valid {
		// Can't verify recency against an unknown trigger — conservatively
		// leave the draft as a suggestion rather than risk sending after a
		// newer message.
		return Draft{}, false, tx.Commit(ctx)
	}
	var triggerTS *time.Time
	if err := tx.QueryRow(ctx, `SELECT message_ts FROM inbox_messages_v WHERE id = $1`, expectedTrigger.UUID).Scan(&triggerTS); err != nil {
		return Draft{}, false, tx.Commit(ctx)
	}

	var humanReplied bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM inbox_messages_v
			WHERE chat_id = $1 AND direction = 'out' AND sender_kind IN ('user','external_account')
			  AND ($2 IS NULL OR message_ts IS NULL OR message_ts > $2)
		)`, chatID, triggerTS).Scan(&humanReplied); err != nil {
		return Draft{}, false, err
	}
	if humanReplied {
		return Draft{}, false, tx.Commit(ctx)
	}

	var latestInbound uuid.NullUUID
	if err := tx.QueryRow(ctx, `
		SELECT id FROM inbox_messages_v WHERE chat_id = $1 AND direction='in'
		ORDER BY message_ts DESC NULLS LAST, created_at DESC LIMIT 1`, chatID).Scan(&latestInbound); err != nil && !errors.Is(err, dbx.ErrNoRows) {
		return Draft{}, false, err
	}
	if !latestInbound.Valid || latestInbound.UUID != expectedTrigger.UUID {
		return Draft{}, false, tx.Commit(ctx)
	}

	var d Draft
	cerr := tx.QueryRow(ctx, `
		UPDATE ai_drafts SET draft_state='sent', updated_at=strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND draft_state='suggested'
		RETURNING `+draftCols, draftID).Scan(scanDraftDst(&d)...)
	if errors.Is(cerr, dbx.ErrNoRows) {
		return Draft{}, false, tx.Commit(ctx) // lost the race to a manual approve
	}
	if cerr != nil {
		return Draft{}, false, cerr
	}
	if _, err := tx.Exec(ctx, `
		UPDATE ai_drafts SET draft_state='superseded', updated_at=strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE chat_id = $1 AND id <> $2 AND draft_state='suggested'`, chatID, draftID); err != nil {
		return Draft{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Draft{}, false, err
	}
	return d, true, nil
}

// InsertAutomationOutbound records an AI auto-send exactly like
// InsertOutbound, except it deliberately does NOT zero the chat's
// unread_count: an auto-reply is not an operator reading the conversation,
// so the badge must stay exactly as an operator would see it if they opened
// the chat right after. sender_user_id is always NULL (an automated send
// has no human sender).
func (s *Store) InsertAutomationOutbound(ctx context.Context, channel string, chatID, accountID uuid.UUID, body, preview string) (uuid.UUID, error) {
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
		VALUES ($1, $2, 'out', 'ai', NULL, 'conversation', $3, 'queued', 'app', strftime('%Y-%m-%d %H:%M:%f','now'))
		RETURNING id`,
		accountID, chatID, body).Scan(&id); err != nil {
		return uuid.Nil, wrap("insert automation outbound", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE `+chatTable+` SET last_message_at = strftime('%Y-%m-%d %H:%M:%f','now'), last_message_preview = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, chatID, preview); err != nil {
		return uuid.Nil, wrap("update aggregates", err)
	}
	return id, tx.Commit(ctx)
}
