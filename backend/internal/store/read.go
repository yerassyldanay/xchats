package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ChatFilter holds the inbox list filters.
type ChatFilter struct {
	Status   string
	Assignee string // me|unassigned|<uuid>
	MeUserID uuid.UUID
	Query    string
	Limit    int
	Offset   int
}

// ListChats returns inbox rows (with contact) ordered by recency, plus the total.
func (s *Store) ListChats(ctx context.Context, accountID uuid.UUID, f ChatFilter) ([]Chat, int, error) {
	var where []string
	var args []any
	args = append(args, accountID)
	where = append(where, "c.account_id = $1")

	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, "c.chat_state = $"+itoa(len(args)))
	}
	switch {
	case f.Assignee == "me":
		args = append(args, f.MeUserID)
		where = append(where, "c.assignee_user_id = $"+itoa(len(args)))
	case f.Assignee == "unassigned":
		where = append(where, "c.assignee_user_id IS NULL")
	case f.Assignee != "":
		if id, err := uuid.Parse(f.Assignee); err == nil {
			args = append(args, id)
			where = append(where, "c.assignee_user_id = $"+itoa(len(args)))
		}
	}
	if f.Query != "" {
		args = append(args, "%"+strings.ToLower(f.Query)+"%")
		i := itoa(len(args))
		where = append(where, "(lower(ct.display_name) LIKE $"+i+" OR lower(ct.phone_number) LIKE $"+i+" OR ct.phone_jid LIKE $"+i+")")
	}
	clause := strings.Join(where, " AND ")

	args = append(args, f.Limit, f.Offset)
	q := `
		SELECT c.id, c.account_id, c.contact_id, c.remote_jid, c.chat_state, c.assignee_user_id,
		       c.last_message_at, c.last_message_preview, c.unread_count,
		       ct.id, ct.phone_number, ct.phone_jid, COALESCE(ct.lid_jid,''), ct.push_name, ct.display_name, ct.attributes
		FROM xchats.wa_chats c JOIN xchats.wa_contacts ct ON ct.id = c.contact_id
		WHERE ` + clause + `
		ORDER BY c.last_message_at DESC NULLS LAST
		LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Chat
	for rows.Next() {
		var c Chat
		if err := rows.Scan(&c.ID, &c.AccountID, &c.ContactID, &c.RemoteJID, &c.ChatState, &c.AssigneeUserID,
			&c.LastMessageAt, &c.LastMessagePreview, &c.UnreadCount,
			&c.Contact.ID, &c.Contact.PhoneNumber, &c.Contact.PhoneJID, &c.Contact.LidJID, &c.Contact.PushName, &c.Contact.DisplayName, &c.Contact.Attributes); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	var total int
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM xchats.wa_chats c JOIN xchats.wa_contacts ct ON ct.id=c.contact_id WHERE `+clause, args[:len(args)-2]...).Scan(&total)
	return out, total, rows.Err()
}

// ChatByID returns a chat with its contact.
func (s *Store) ChatByID(ctx context.Context, id uuid.UUID) (Chat, error) {
	var c Chat
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.account_id, c.contact_id, c.remote_jid, c.chat_state, c.assignee_user_id,
		       c.last_message_at, c.last_message_preview, c.unread_count,
		       ct.id, ct.phone_number, ct.phone_jid, COALESCE(ct.lid_jid,''), ct.push_name, ct.display_name, ct.attributes
		FROM xchats.wa_chats c JOIN xchats.wa_contacts ct ON ct.id = c.contact_id
		WHERE c.id = $1`, id).
		Scan(&c.ID, &c.AccountID, &c.ContactID, &c.RemoteJID, &c.ChatState, &c.AssigneeUserID,
			&c.LastMessageAt, &c.LastMessagePreview, &c.UnreadCount,
			&c.Contact.ID, &c.Contact.PhoneNumber, &c.Contact.PhoneJID, &c.Contact.LidJID, &c.Contact.PushName, &c.Contact.DisplayName, &c.Contact.Attributes)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// MarkChatRead zeroes the unread badge and returns the refreshed chat.
func (s *Store) MarkChatRead(ctx context.Context, id uuid.UUID) (Chat, error) {
	_, err := s.pool.Exec(ctx, `UPDATE xchats.wa_chats SET unread_count = 0, updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return Chat{}, err
	}
	return s.ChatByID(ctx, id)
}

// MessagesForChat returns up to limit messages older than `before` (chronological asc),
// with their media refs attached, plus the next cursor.
func (s *Store) MessagesForChat(ctx context.Context, chatID uuid.UUID, before time.Time, limit int) ([]Message, *time.Time, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var beforeArg any = before
	if before.IsZero() {
		beforeArg = nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, chat_id, direction, sender_kind, sender_user_id, COALESCE(evolution_message_id,''),
		       message_kind, body, delivery_state, source, message_ts
		FROM xchats.wa_messages
		WHERE chat_id = $1 AND ($2::timestamptz IS NULL OR message_ts < $2)
		ORDER BY message_ts DESC, id DESC
		LIMIT $3`, chatID, beforeArg, limit+1)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var desc []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Direction, &m.SenderKind, &m.SenderUserID, &m.EvolutionMessageID,
			&m.MessageKind, &m.Body, &m.DeliveryState, &m.Source, &m.MessageTS); err != nil {
			return nil, nil, err
		}
		desc = append(desc, m)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	var next *time.Time
	if len(desc) > limit {
		last := desc[limit]
		if last.MessageTS != nil {
			next = last.MessageTS
		}
		desc = desc[:limit]
	}
	// reverse to chronological order
	out := make([]Message, len(desc))
	for i := range desc {
		out[len(desc)-1-i] = desc[i]
	}
	if err := s.attachMedia(ctx, out); err != nil {
		return nil, nil, err
	}
	return out, next, nil
}

// MessageByID returns one message with media (used for send responses + SSE).
func (s *Store) MessageByID(ctx context.Context, id uuid.UUID) (Message, error) {
	var m Message
	err := s.pool.QueryRow(ctx, `
		SELECT id, chat_id, direction, sender_kind, sender_user_id, COALESCE(evolution_message_id,''),
		       message_kind, body, delivery_state, source, message_ts
		FROM xchats.wa_messages WHERE id = $1`, id).
		Scan(&m.ID, &m.ChatID, &m.Direction, &m.SenderKind, &m.SenderUserID, &m.EvolutionMessageID,
			&m.MessageKind, &m.Body, &m.DeliveryState, &m.Source, &m.MessageTS)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrNotFound
	}
	if err != nil {
		return m, err
	}
	one := []Message{m}
	if err := s.attachMedia(ctx, one); err != nil {
		return m, err
	}
	return one[0], nil
}

func (s *Store) attachMedia(ctx context.Context, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(msgs))
	idx := make(map[uuid.UUID]int, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
		idx[m.ID] = i
	}
	rows, err := s.pool.Query(ctx, `
		SELECT message_id, id, media_type, mimetype, file_name, file_size
		FROM xchats.message_media WHERE message_id = ANY($1) ORDER BY created_at`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var mid uuid.UUID
		var r MediaRef
		if err := rows.Scan(&mid, &r.ID, &r.MediaType, &r.Mimetype, &r.FileName, &r.FileSize); err != nil {
			return err
		}
		if i, ok := idx[mid]; ok {
			msgs[i].Media = append(msgs[i].Media, r)
		}
	}
	return rows.Err()
}

// ---------------------------------------------------------------------------
// AI drafts
// ---------------------------------------------------------------------------

// DraftOption is one suggested reply option to be written.
type DraftOption struct {
	Ordinal          int
	Text             string
	Confidence       *float64
	Escalate         bool
	EscalationReason string
	Assets           []DraftAsset
}

// WriteDraftSet supersedes any prior pending options for the chat and inserts the
// new 1–3 options (+ assets) atomically, returning the written drafts.
func (s *Store) WriteDraftSet(ctx context.Context, chatID uuid.UUID, trigger uuid.NullUUID, opts []DraftOption) ([]Draft, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE xchats.ai_drafts SET draft_state='superseded', updated_at=now()
		WHERE chat_id = $1 AND draft_state='suggested'`, chatID); err != nil {
		return nil, err
	}
	var out []Draft
	for _, o := range opts {
		var d Draft
		if err := tx.QueryRow(ctx, `
			INSERT INTO xchats.ai_drafts (chat_id, trigger_message_id, option_ordinal, draft_text, confidence, escalate, escalation_reason, draft_state)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'suggested')
			RETURNING id, chat_id, trigger_message_id, option_ordinal, draft_text, context_state, confidence, escalate, escalation_reason, draft_state, created_at`,
			chatID, trigger, o.Ordinal, o.Text, o.Confidence, o.Escalate, o.EscalationReason).
			Scan(&d.ID, &d.ChatID, &d.TriggerMessageID, &d.OptionOrdinal, &d.DraftText, &d.ContextState, &d.Confidence, &d.Escalate, &d.EscalationReason, &d.DraftState, &d.CreatedAt); err != nil {
			return nil, err
		}
		for _, a := range o.Assets {
			var da DraftAsset
			if err := tx.QueryRow(ctx, `
				INSERT INTO xchats.ai_draft_assets (draft_id, asset_ref, media_kind, media_url, ordinal)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id, asset_ref, media_kind, media_url, ordinal`,
				d.ID, a.AssetRef, a.MediaKind, a.MediaURL, a.Ordinal).
				Scan(&da.ID, &da.AssetRef, &da.MediaKind, &da.MediaURL, &da.Ordinal); err != nil {
				return nil, err
			}
			d.Assets = append(d.Assets, da)
		}
		out = append(out, d)
	}
	return out, tx.Commit(ctx)
}

// PendingDrafts returns the chat's suggested options (with assets).
func (s *Store) PendingDrafts(ctx context.Context, chatID uuid.UUID) ([]Draft, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, chat_id, trigger_message_id, option_ordinal, draft_text, context_state, confidence, escalate, escalation_reason, draft_state, created_at
		FROM xchats.ai_drafts WHERE chat_id = $1 AND draft_state='suggested'
		ORDER BY option_ordinal`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Draft
	for rows.Next() {
		var d Draft
		if err := rows.Scan(&d.ID, &d.ChatID, &d.TriggerMessageID, &d.OptionOrdinal, &d.DraftText, &d.ContextState, &d.Confidence, &d.Escalate, &d.EscalationReason, &d.DraftState, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := s.loadDraftAssets(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) loadDraftAssets(ctx context.Context, d *Draft) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, asset_ref, media_kind, media_url, ordinal FROM xchats.ai_draft_assets
		WHERE draft_id = $1 ORDER BY ordinal`, d.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var a DraftAsset
		if err := rows.Scan(&a.ID, &a.AssetRef, &a.MediaKind, &a.MediaURL, &a.Ordinal); err != nil {
			return err
		}
		d.Assets = append(d.Assets, a)
	}
	return rows.Err()
}

// DraftByID loads one draft (+assets).
func (s *Store) DraftByID(ctx context.Context, id uuid.UUID) (Draft, error) {
	var d Draft
	err := s.pool.QueryRow(ctx, `
		SELECT id, chat_id, trigger_message_id, option_ordinal, draft_text, context_state, confidence, escalate, escalation_reason, draft_state, created_at
		FROM xchats.ai_drafts WHERE id = $1`, id).
		Scan(&d.ID, &d.ChatID, &d.TriggerMessageID, &d.OptionOrdinal, &d.DraftText, &d.ContextState, &d.Confidence, &d.Escalate, &d.EscalationReason, &d.DraftState, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	return d, s.loadDraftAssets(ctx, &d)
}

// ClaimDraft is the guarded single-send: it flips exactly one suggested option to
// 'sent' and supersedes its siblings, atomically. ErrNotFound means the guard lost
// (already approved or superseded) — the caller classifies via DraftByID.
func (s *Store) ClaimDraft(ctx context.Context, draftID uuid.UUID) (Draft, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Draft{}, err
	}
	defer tx.Rollback(ctx)
	var d Draft
	err = tx.QueryRow(ctx, `
		UPDATE xchats.ai_drafts SET draft_state='sent', updated_at=now()
		WHERE id = $1 AND draft_state='suggested'
		RETURNING id, chat_id, trigger_message_id, option_ordinal, draft_text, context_state, confidence, escalate, escalation_reason, draft_state, created_at`,
		draftID).Scan(&d.ID, &d.ChatID, &d.TriggerMessageID, &d.OptionOrdinal, &d.DraftText, &d.ContextState, &d.Confidence, &d.Escalate, &d.EscalationReason, &d.DraftState, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE xchats.ai_drafts SET draft_state='superseded', updated_at=now()
		WHERE chat_id = $1 AND id <> $2 AND draft_state='suggested'`, d.ChatID, draftID); err != nil {
		return d, err
	}
	if err := tx.Commit(ctx); err != nil {
		return d, err
	}
	return d, s.loadDraftAssets(ctx, &d)
}

// SetDraftSent records the message a draft actually produced.
func (s *Store) SetDraftSent(ctx context.Context, draftID, sentMessageID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE xchats.ai_drafts SET sent_message_id = $2, updated_at = now() WHERE id = $1`, draftID, sentMessageID)
	return err
}

// ReopenDraft puts a claimed draft back to suggested (used when the send fails).
func (s *Store) ReopenDraft(ctx context.Context, draftID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE xchats.ai_drafts SET draft_state='suggested', updated_at=now() WHERE id=$1`, draftID)
	return err
}

// LatestInboundMessageID returns the most recent inbound message for a chat (the
// trigger the stub answers).
func (s *Store) LatestInboundMessageID(ctx context.Context, chatID uuid.UUID) (uuid.NullUUID, error) {
	var id uuid.NullUUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM xchats.wa_messages WHERE chat_id = $1 AND direction='in'
		ORDER BY message_ts DESC NULLS LAST, created_at DESC LIMIT 1`, chatID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return id, nil
	}
	return id, err
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return string(b[p:])
}
