package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DraftDebounceRow is one chat_draft_debounce row, returned when it fires.
type DraftDebounceRow struct {
	ChatID           uuid.UUID
	Channel          string
	TriggerMessageID uuid.UUID
}

func scanDraftDebounceRow(row pgx.Row) (DraftDebounceRow, error) {
	var r DraftDebounceRow
	err := row.Scan(&r.ChatID, &r.Channel, &r.TriggerMessageID)
	return r, err
}

// TouchDraftDebounce upserts the chat's debounce row: generate_after resets to
// quiet from now on every touch, capped at first_seen_at+cap so a chat that
// never falls quiet is still answered. Returns the resulting generate_after
// so the caller can arm an in-process timer for exactly that instant.
func (s *Store) TouchDraftDebounce(ctx context.Context, chatID uuid.UUID, channel string, triggerMessageID uuid.UUID, quiet, cap time.Duration) (time.Time, error) {
	var generateAfter time.Time
	err := s.pool.QueryRow(ctx, `
		INSERT INTO xchats.chat_draft_debounce (chat_id, channel, trigger_message_id, generate_after)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4))
		ON CONFLICT (chat_id) DO UPDATE
		   SET trigger_message_id = EXCLUDED.trigger_message_id,
		       generate_after     = LEAST(EXCLUDED.generate_after,
		                                  xchats.chat_draft_debounce.first_seen_at + make_interval(secs => $5)),
		       updated_at         = now()
		RETURNING generate_after`,
		chatID, channel, triggerMessageID, quiet.Seconds(), cap.Seconds()).Scan(&generateAfter)
	return generateAfter, err
}

// FireDueDraftDebounce conditionally deletes chatID's debounce row if it is
// actually due (generate_after <= now()) and returns it. ok is false when a
// newer touch pushed generate_after into the future after the caller's timer
// was scheduled — a fresh timer for that later instant already exists, so
// there is nothing to do here.
func (s *Store) FireDueDraftDebounce(ctx context.Context, chatID uuid.UUID) (DraftDebounceRow, bool, error) {
	row, err := scanDraftDebounceRow(s.pool.QueryRow(ctx, `
		DELETE FROM xchats.chat_draft_debounce
		WHERE chat_id = $1 AND generate_after <= now()
		RETURNING chat_id, channel, trigger_message_id`, chatID))
	if errors.Is(err, pgx.ErrNoRows) {
		return DraftDebounceRow{}, false, nil
	}
	if err != nil {
		return DraftDebounceRow{}, false, err
	}
	return row, true, nil
}

// ClaimDueDraftDebounce deletes and returns up to limit rows whose
// generate_after has passed — the sweeper's recovery path, so a message that
// arrived seconds before a restart (when the in-process timer map is empty)
// still gets its draft.
func (s *Store) ClaimDueDraftDebounce(ctx context.Context, limit int) ([]DraftDebounceRow, error) {
	rows, err := s.pool.Query(ctx, `
		DELETE FROM xchats.chat_draft_debounce
		WHERE chat_id IN (
			SELECT chat_id FROM xchats.chat_draft_debounce
			WHERE generate_after <= now()
			ORDER BY generate_after
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING chat_id, channel, trigger_message_id`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DraftDebounceRow
	for rows.Next() {
		r, err := scanDraftDebounceRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
