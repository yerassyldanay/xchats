package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ChatFilter holds the inbox list filters. Build 1 scopes the inbox to an org
// (chats across all that org's live accounts, on every channel) with an optional
// single-account filter (the "from account" chip).
type ChatFilter struct {
	OrgID     uuid.UUID
	AccountID uuid.NullUUID // optional: restrict to one account
	Status    string
	Assignee  string // me|unassigned|<uuid>
	MeUserID  uuid.UUID
	Query     string
	Limit     int
	Offset    int
}

// chatCols is the canonical inbox_chats_v projection — one shape for every
// channel, with provider vocabulary already folded into neutral column names.
const chatCols = `c.id, c.account_id, c.contact_id, c.channel, c.external_conversation_ref,
	c.chat_state, c.assignee_user_id,
	c.last_message_at, c.last_inbound_at, c.last_message_preview, c.unread_count,
	c.contact_id, c.contact_phone_number, c.external_contact_ref, c.contact_lid_jid,
	c.contact_push_name, c.contact_display_name, c.contact_attributes`

func scanChatDst(c *Chat) []any {
	return []any{
		&c.ID, &c.AccountID, &c.ContactID, &c.Channel, &c.ExternalConversationRef,
		&c.ChatState, &c.AssigneeUserID,
		&c.LastMessageAt, &c.LastInboundAt, &c.LastMessagePreview, &c.UnreadCount,
		&c.Contact.ID, &c.Contact.PhoneNumber, &c.Contact.ExternalContactRef, &c.Contact.LidJID,
		&c.Contact.PushName, &c.Contact.DisplayName, &c.Contact.Attributes,
	}
}

// ListChatsForOrg returns the org's inbox rows across every channel, ordered by
// recency, plus the total. inbox_chats_v carries the owning account's
// organization_id and deleted_at, so the org scope and the "hide soft-deleted
// accounts' chats" rule are the same two predicates they always were.
//
// A chat_state='campaign' row (a cold-send campaign's own recipient, before
// any reply — see MarkChatCampaignOnly) is excluded from the default,
// unfiltered listing: an inbox full of one-way campaign sends nobody has
// answered yet would bury the conversations an operator actually needs to
// see. f.Status="campaign" still reaches them explicitly (the campaign
// detail page's own "view in inbox" links use this), and any OTHER explicit
// f.Status is unaffected — this carve-out only changes the "no filter at
// all" default.
func (s *Store) ListChatsForOrg(ctx context.Context, f ChatFilter) ([]Chat, int, error) {
	var where []string
	var args []any
	args = append(args, f.OrgID)
	where = append(where, "c.organization_id = $1", "c.account_deleted_at IS NULL")

	if f.AccountID.Valid {
		args = append(args, f.AccountID.UUID)
		where = append(where, "c.account_id = $"+itoa(len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, "c.chat_state = $"+itoa(len(args)))
	} else {
		where = append(where, "c.chat_state <> 'campaign'")
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
		where = append(where, "(lower(c.contact_display_name) LIKE $"+i+
			" OR lower(c.contact_phone_number) LIKE $"+i+
			" OR c.external_contact_ref LIKE $"+i+")")
	}
	clause := strings.Join(where, " AND ")
	const from = `inbox_chats_v c`

	args = append(args, f.Limit, f.Offset)
	q := `SELECT ` + chatCols + ` FROM ` + from + `
		WHERE ` + clause + `
		ORDER BY c.last_message_at DESC NULLS LAST
		LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := s.db.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Chat
	for rows.Next() {
		var c Chat
		if err := rows.Scan(scanChatDst(&c)...); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	var total int
	_ = s.db.QueryRow(ctx, `SELECT count(*) FROM `+from+` WHERE `+clause, args[:len(args)-2]...).Scan(&total)
	return out, total, rows.Err()
}

// ChatByID returns a chat with its contact, on any channel.
func (s *Store) ChatByID(ctx context.Context, id uuid.UUID) (Chat, error) {
	var c Chat
	err := s.db.QueryRow(ctx, `SELECT `+chatCols+`
		FROM inbox_chats_v c WHERE c.id = $1`, id).Scan(scanChatDst(&c)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// ChatByIDForOrg returns a chat only if it belongs to one of the org's live
// accounts — the membership guard behind every chat-scoped endpoint. ErrNotFound
// otherwise (a cross-org id is indistinguishable from a missing one).
func (s *Store) ChatByIDForOrg(ctx context.Context, id, orgID uuid.UUID) (Chat, error) {
	var c Chat
	err := s.db.QueryRow(ctx, `SELECT `+chatCols+`
		FROM inbox_chats_v c
		WHERE c.id = $1 AND c.organization_id = $2 AND c.account_deleted_at IS NULL`, id, orgID).
		Scan(scanChatDst(&c)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

// MarkChatRead zeroes the unread badge on the chat's own transport table and
// returns the refreshed chat. The read resolves the channel; the write is
// dispatched to wa_chats or tg_chats — views are read-only.
func (s *Store) MarkChatRead(ctx context.Context, id uuid.UUID) (Chat, error) {
	chat, err := s.ChatByID(ctx, id)
	if err != nil {
		return Chat{}, err
	}
	table, err := chatsTableFor(chat.Channel)
	if err != nil {
		return Chat{}, err
	}
	if _, err := s.db.Exec(ctx,
		`UPDATE `+table+` SET unread_count = 0, updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE id = $1`, id); err != nil {
		return Chat{}, err
	}
	chat.UnreadCount = 0
	return chat, nil
}

// AssignChat sets (or clears) a chat's assignee on the transport-specific chat
// table and returns the refreshed channel-neutral view row.
func (s *Store) AssignChat(ctx context.Context, id uuid.UUID, assignee uuid.NullUUID) (Chat, error) {
	chat, err := s.ChatByID(ctx, id)
	if err != nil {
		return Chat{}, err
	}
	table, err := chatsTableFor(chat.Channel)
	if err != nil {
		return Chat{}, err
	}
	var value any
	if assignee.Valid {
		value = assignee.UUID
	}
	if _, err := s.db.Exec(ctx,
		`UPDATE `+table+` SET assignee_user_id = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE id = $1`, id, value); err != nil {
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
	rows, err := s.db.Query(ctx, `SELECT `+messageCols+`
		FROM inbox_messages_v
		WHERE chat_id = $1 AND ($2 IS NULL OR message_ts < $2)
		ORDER BY message_ts DESC, id DESC
		LIMIT $3`, chatID, beforeArg, limit+1)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var desc []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(scanMessageDst(&m)...); err != nil {
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

// messageCols is the canonical inbox_messages_v projection; scanMessageDst pairs
// with it.
const messageCols = `id, chat_id, channel, direction, sender_kind, sender_user_id,
	external_message_id, message_kind, body, delivery_state, source, message_ts`

func scanMessageDst(m *Message) []any {
	return []any{
		&m.ID, &m.ChatID, &m.Channel, &m.Direction, &m.SenderKind, &m.SenderUserID,
		&m.ExternalMessageID, &m.MessageKind, &m.Body, &m.DeliveryState, &m.Source, &m.MessageTS,
	}
}

// MessageByID returns one message with media (used for send responses + SSE).
func (s *Store) MessageByID(ctx context.Context, id uuid.UUID) (Message, error) {
	var m Message
	err := s.db.QueryRow(ctx, `SELECT `+messageCols+`
		FROM inbox_messages_v WHERE id = $1`, id).
		Scan(scanMessageDst(&m)...)
	if errors.Is(err, dbx.ErrNoRows) {
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
	// = ANY($1) -> IN (SELECT value FROM json_each($1)), binding the id list
	// as a dbx.UUIDArray (JSON array in TEXT) — see the per-PG-ism table.
	rows, err := s.db.Query(ctx, `
		SELECT message_id, id, media_type, mimetype, filename, size
		FROM inbox_message_media_v
		WHERE message_id IN (SELECT value FROM json_each($1))
		ORDER BY created_at`, dbx.UUIDArray(ids))
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

// draftCols is the canonical ai_drafts projection; scanDraftDst pairs with it.
const draftCols = `id, chat_id, channel, trigger_message_id, option_ordinal, draft_text,
	reply_language, context_state, confidence, escalate, escalation_reason, draft_state, created_at`

func scanDraftDst(d *Draft) []any {
	return []any{
		&d.ID, &d.ChatID, &d.Channel, &d.TriggerMessageID, &d.OptionOrdinal, &d.DraftText,
		&d.ReplyLanguage, &d.ContextState, &d.Confidence, &d.Escalate, &d.EscalationReason,
		&d.DraftState, &d.CreatedAt,
	}
}

// DraftOption is one suggested reply option to be written.
type DraftOption struct {
	Ordinal          int
	Text             string
	ReplyLanguage    string
	Confidence       *float64
	Escalate         bool
	EscalationReason string
}

// WriteDraftSet supersedes any prior pending options for the chat and inserts the
// new 1–3 options atomically, returning the written drafts. channel is stamped on
// each row: ai_drafts is channel-neutral (migration 0013 dropped its FKs into
// wa_*), so the column is the only record of which transport the chat lives on.
func (s *Store) WriteDraftSet(ctx context.Context, channel string, chatID uuid.UUID, trigger uuid.NullUUID, opts []DraftOption) ([]Draft, error) {
	if channel == "" {
		channel = "whatsapp"
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE ai_drafts SET draft_state='superseded', updated_at=strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE chat_id = $1 AND draft_state='suggested'`, chatID); err != nil {
		return nil, err
	}
	var out []Draft
	for _, o := range opts {
		var d Draft
		if err := tx.QueryRow(ctx, `
			INSERT INTO ai_drafts (chat_id, channel, trigger_message_id, option_ordinal, draft_text, reply_language, confidence, escalate, escalation_reason, draft_state)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'suggested')
			RETURNING id, chat_id, channel, trigger_message_id, option_ordinal, draft_text, reply_language, context_state, confidence, escalate, escalation_reason, draft_state, created_at`,
			chatID, channel, trigger, o.Ordinal, o.Text, o.ReplyLanguage, o.Confidence, o.Escalate, o.EscalationReason).
			Scan(&d.ID, &d.ChatID, &d.Channel, &d.TriggerMessageID, &d.OptionOrdinal, &d.DraftText, &d.ReplyLanguage, &d.ContextState, &d.Confidence, &d.Escalate, &d.EscalationReason, &d.DraftState, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, tx.Commit(ctx)
}

// HasDraftForTrigger reports whether any draft already exists for a trigger
// message. The Telegram webhook uses it on a duplicate delivery: a redelivery
// that finds no draft means the first attempt died between the commit and the
// enqueue, so the draft task is published again (at-least-once; WriteDraftSet's
// supersede semantics make a rare double-generation harmless).
func (s *Store) HasDraftForTrigger(ctx context.Context, triggerMessageID uuid.UUID) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ai_drafts WHERE trigger_message_id = $1)`, triggerMessageID).
		Scan(&exists)
	return exists, err
}

// PendingDrafts returns the chat's suggested options.
func (s *Store) PendingDrafts(ctx context.Context, chatID uuid.UUID) ([]Draft, error) {
	rows, err := s.db.Query(ctx, `
		SELECT `+draftCols+`
		FROM ai_drafts WHERE chat_id = $1 AND draft_state='suggested'
		ORDER BY option_ordinal`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Draft
	for rows.Next() {
		var d Draft
		if err := rows.Scan(scanDraftDst(&d)...); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// DraftByID loads one draft.
func (s *Store) DraftByID(ctx context.Context, id uuid.UUID) (Draft, error) {
	var d Draft
	err := s.db.QueryRow(ctx, `
		SELECT `+draftCols+`
		FROM ai_drafts WHERE id = $1`, id).
		Scan(scanDraftDst(&d)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	return d, nil
}

// ClaimDraft is the guarded single-send: it flips exactly one suggested option to
// 'sent' and supersedes its siblings, atomically. ErrNotFound means the guard lost
// (already approved or superseded) — the caller classifies via DraftByID.
func (s *Store) ClaimDraft(ctx context.Context, draftID uuid.UUID) (Draft, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Draft{}, err
	}
	defer tx.Rollback(ctx)
	var d Draft
	err = tx.QueryRow(ctx, `
		UPDATE ai_drafts SET draft_state='sent', updated_at=strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND draft_state='suggested'
		RETURNING `+draftCols+``,
		draftID).Scan(scanDraftDst(&d)...)
	if errors.Is(err, dbx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE ai_drafts SET draft_state='superseded', updated_at=strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE chat_id = $1 AND id <> $2 AND draft_state='suggested'`, d.ChatID, draftID); err != nil {
		return d, err
	}
	if err := tx.Commit(ctx); err != nil {
		return d, err
	}
	return d, nil
}

// SetDraftSent records the message a draft actually produced.
func (s *Store) SetDraftSent(ctx context.Context, draftID, sentMessageID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE ai_drafts SET sent_message_id = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE id = $1`, draftID, sentMessageID)
	return err
}

// ReopenDraft puts a claimed draft back to suggested (used when the send fails).
func (s *Store) ReopenDraft(ctx context.Context, draftID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `UPDATE ai_drafts SET draft_state='suggested', updated_at=strftime('%Y-%m-%d %H:%M:%f','now') WHERE id=$1`, draftID)
	return err
}

// LatestInboundMessageID returns the most recent inbound message for a chat (the
// trigger the stub answers).
func (s *Store) LatestInboundMessageID(ctx context.Context, chatID uuid.UUID) (uuid.NullUUID, error) {
	var id uuid.NullUUID
	err := s.db.QueryRow(ctx, `
		SELECT id FROM inbox_messages_v WHERE chat_id = $1 AND direction='in'
		ORDER BY message_ts DESC NULLS LAST, created_at DESC LIMIT 1`, chatID).Scan(&id)
	if errors.Is(err, dbx.ErrNoRows) {
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
