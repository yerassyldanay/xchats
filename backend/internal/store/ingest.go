package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InboundUpsert is the worker's normalized input for one message event.
type InboundUpsert struct {
	AccountID          uuid.UUID
	PhoneJID           string
	LidJID             string
	RemoteJID          string
	PhoneNumber        string
	PushName           string
	Direction          string // in|out
	SenderKind         string // contact|external_account
	EvolutionMessageID string
	MessageKind        string
	Body               string
	Preview            string // chat-list preview (media → placeholder)
	Source             string
	Raw                []byte
	MessageTS          time.Time
}

// InboundResult reports what the upsert did, so the worker knows which SSE to emit.
type InboundResult struct {
	ContactID       uuid.UUID
	ChatID          uuid.UUID
	MessageID       uuid.UUID
	ChatCreated     bool
	MessageInserted bool
}

// UpsertInbound idempotently upserts contact → chat → message and maintains the
// chat aggregates, all in one transaction. Re-delivery (same evolution_message_id)
// is a no-op upsert (MessageInserted=false).
func (s *Store) UpsertInbound(ctx context.Context, in InboundUpsert) (InboundResult, error) {
	var res InboundResult
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx)

	// contact
	var lid any
	if in.LidJID != "" {
		lid = in.LidJID
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO xchats.wa_contacts (account_id, phone_jid, lid_jid, phone_number, push_name, display_name)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (account_id, phone_jid) DO UPDATE SET
			lid_jid = COALESCE(EXCLUDED.lid_jid, xchats.wa_contacts.lid_jid),
			push_name = CASE WHEN EXCLUDED.push_name <> '' THEN EXCLUDED.push_name ELSE xchats.wa_contacts.push_name END,
			updated_at = now()
		RETURNING id`,
		in.AccountID, in.PhoneJID, lid, in.PhoneNumber, in.PushName).Scan(&res.ContactID); err != nil {
		return res, wrap("upsert contact", err)
	}

	// chat
	if err := tx.QueryRow(ctx, `
		INSERT INTO xchats.wa_chats (account_id, contact_id, remote_jid)
		VALUES ($1, $2, $3)
		ON CONFLICT (account_id, remote_jid) DO UPDATE SET updated_at = now()
		RETURNING id, (xmax = 0)`,
		in.AccountID, res.ContactID, in.RemoteJID).Scan(&res.ChatID, &res.ChatCreated); err != nil {
		return res, wrap("upsert chat", err)
	}

	// message (dedup on the natural key; preserves sender_kind on conflict)
	var evID any
	if in.EvolutionMessageID != "" {
		evID = in.EvolutionMessageID
	}
	var ts any
	if !in.MessageTS.IsZero() {
		ts = in.MessageTS
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO xchats.wa_messages
			(account_id, chat_id, direction, sender_kind, evolution_message_id, message_kind, body, delivery_state, source, raw, message_ts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (account_id, evolution_message_id) DO UPDATE SET
			body = CASE WHEN xchats.wa_messages.body = '' THEN EXCLUDED.body ELSE xchats.wa_messages.body END,
			updated_at = now()
		RETURNING id, (xmax = 0)`,
		in.AccountID, res.ChatID, in.Direction, in.SenderKind, evID, in.MessageKind, in.Body,
		deliveryStateFor(in.Direction), in.Source, jsonbOrNil(in.Raw), ts).
		Scan(&res.MessageID, &res.MessageInserted); err != nil {
		return res, wrap("upsert message", err)
	}

	// aggregates — only on a genuine new message
	if res.MessageInserted {
		unreadDelta := "unread_count = unread_count + 1"
		if in.Direction == "out" {
			unreadDelta = "unread_count = 0"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE xchats.wa_chats SET
				last_message_at = COALESCE($2, now()),
				last_message_preview = $3,
				`+unreadDelta+`,
				updated_at = now()
			WHERE id = $1`, res.ChatID, ts, in.Preview); err != nil {
			return res, wrap("update aggregates", err)
		}
	}

	return res, tx.Commit(ctx)
}

func deliveryStateFor(direction string) string {
	if direction == "out" {
		return "sent" // an external/echo outbound is already at the gateway
	}
	return "delivered"
}

// AdvanceDeliveryState applies a status update, monotonically (never backwards).
// Returns the message + chat id on a real change, ErrNotFound otherwise.
func (s *Store) AdvanceDeliveryState(ctx context.Context, accountID uuid.UUID, evolutionMessageID, newState string, newRank int) (uuid.UUID, uuid.UUID, error) {
	var msgID, chatID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		UPDATE xchats.wa_messages SET delivery_state = $3, updated_at = now()
		WHERE account_id = $1 AND evolution_message_id = $2
		  AND (CASE delivery_state
				WHEN 'queued' THEN 0 WHEN 'sent' THEN 1 WHEN 'delivered' THEN 2
				WHEN 'read' THEN 3 WHEN 'failed' THEN 4 ELSE 0 END) < $4
		RETURNING id, chat_id`,
		accountID, evolutionMessageID, newState, newRank).Scan(&msgID, &chatID)
	if errors.Is(err, pgx.ErrNoRows) {
		return msgID, chatID, ErrNotFound
	}
	return msgID, chatID, err
}

// UpsertMessageMedia inserts a media row for a message (idempotent on UNIQUE(message_id)).
func (s *Store) UpsertMessageMedia(ctx context.Context, messageID uuid.UUID, m MediaRef, storageURL, downloadStatus string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	var inserted bool
	err := s.pool.QueryRow(ctx, `
		INSERT INTO xchats.message_media (message_id, media_type, mimetype, file_name, file_size, storage_url, download_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (message_id) DO UPDATE SET updated_at = now()
		RETURNING id, (xmax = 0)`,
		messageID, m.MediaType, m.Mimetype, m.FileName, m.FileSize, storageURL, downloadStatus).
		Scan(&id, &inserted)
	return id, inserted, err
}

// SetMediaReady marks a media row downloaded and records its byte size.
func (s *Store) SetMediaReady(ctx context.Context, messageID uuid.UUID, fileSize int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xchats.message_media SET download_status='ready', file_size=$2, updated_at=now()
		WHERE message_id = $1`, messageID, fileSize)
	return err
}

// MediaStorageURL resolves a public media id (message_media.id) to its blob key.
func (s *Store) MediaStorageURL(ctx context.Context, id uuid.UUID) (storageURL, mimetype, fileName string, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT storage_url, mimetype, file_name FROM xchats.message_media WHERE id = $1`, id).
		Scan(&storageURL, &mimetype, &fileName)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", ErrNotFound
	}
	return
}

// InsertOutboundMessage creates a queued outbound row (evolution_message_id NULL
// until stamped) and bumps the chat aggregates. Used by the send pipeline (B7).
func (s *Store) InsertOutboundMessage(ctx context.Context, chatID, accountID uuid.UUID, senderKind string, senderUserID uuid.NullUUID, messageKind, body, preview string) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var id uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO xchats.wa_messages
			(account_id, chat_id, direction, sender_kind, sender_user_id, message_kind, body, delivery_state, source, message_ts)
		VALUES ($1, $2, 'out', $3, $4, $5, $6, 'queued', 'app', now())
		RETURNING id`,
		accountID, chatID, senderKind, senderUserID, messageKind, body).Scan(&id); err != nil {
		return uuid.Nil, wrap("insert outbound", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE xchats.wa_chats SET last_message_at = now(), last_message_preview = $2, unread_count = 0, updated_at = now()
		WHERE id = $1`, chatID, preview); err != nil {
		return uuid.Nil, wrap("update aggregates", err)
	}
	return id, tx.Commit(ctx)
}

// StampEvolutionID records the gateway's key.id onto a sent outbound row; this is
// what lets the fromMe=true echo collapse onto it instead of duplicating.
func (s *Store) StampEvolutionID(ctx context.Context, messageID uuid.UUID, keyID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xchats.wa_messages SET evolution_message_id = $2, delivery_state = 'sent', updated_at = now()
		WHERE id = $1`, messageID, keyID)
	return err
}

// SetDeliveryState forces a delivery state (e.g. 'failed' when a send errors).
func (s *Store) SetDeliveryState(ctx context.Context, messageID uuid.UUID, state string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE xchats.wa_messages SET delivery_state = $2, updated_at = now() WHERE id = $1`, messageID, state)
	return err
}

func jsonbOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
