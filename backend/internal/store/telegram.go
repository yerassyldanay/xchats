package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
)

// ErrAccountClaimed means a bot is already owned by a DIFFERENT organization,
// or is orphaned (organization_id NULL after its org was deleted). Neither is
// claimable: the account row carries that other tenant's chats and messages,
// and adopting it would inherit their history. Re-claiming an orphaned bot
// needs an explicit admin purge (delete the transport rows first), which is
// deliberately not exposed here.
var ErrAccountClaimed = errors.New("store: this bot already belongs to another organization")

// ErrNoCredentialsKey means no encryption key was configured, so stored
// credentials can be neither written nor read. It is a configuration failure,
// never "the token is wrong".
var ErrNoCredentialsKey = errors.New("store: credentials encryption key not configured (set TG_CREDENTIALS_ENC_KEY)")

// UseCredentialsBox installs the AES-GCM box that protects provider credentials
// at rest. Wired once at the composition root; nil (unset) makes every
// credential read/write fail loudly with ErrNoCredentialsKey rather than
// silently storing plaintext.
func (s *Store) UseCredentialsBox(b *secretbox.Box) { s.creds = b }

// --- accounts ---------------------------------------------------------------

// TelegramAccount is one tg_accounts row.
type TelegramAccount struct {
	ID                   uuid.UUID
	OrganizationID       uuid.NullUUID
	DisplayName          string
	BotID                int64
	BotUsername          string
	ConnectionState      string
	WebhookURL           string
	WebhookRegisteredAt  *time.Time
	WebhookLastCheckedAt *time.Time
	WebhookLastError     string
	LastLiveEventAt      *time.Time
	DeletedAt            *time.Time
	CreatedAt            time.Time
}

const tgAccountCols = `id, organization_id, display_name, bot_id, bot_username, connection_state,
	webhook_url, webhook_registered_at, webhook_last_checked_at, webhook_last_error,
	last_live_event_at, deleted_at, created_at`

func scanTgAccount(row dbx.Scanner) (TelegramAccount, error) {
	var a TelegramAccount
	err := row.Scan(&a.ID, &a.OrganizationID, &a.DisplayName, &a.BotID, &a.BotUsername, &a.ConnectionState,
		&a.WebhookURL, &a.WebhookRegisteredAt, &a.WebhookLastCheckedAt, &a.WebhookLastError,
		&a.LastLiveEventAt, &a.DeletedAt, &a.CreatedAt)
	return a, err
}

// TelegramClaim is one "connect this bot to this organization" request.
type TelegramClaim struct {
	ID             uuid.UUID // uuidv5(ns, "telegram:bot:<bot_id>")
	OrganizationID uuid.UUID
	DisplayName    string
	BotID          int64
	BotUsername    string
	BotToken       string // plaintext; sealed inside the transaction, never stored raw
}

// ClaimTelegramAccount atomically claims a bot for an organization and stores
// its token, in ONE transaction.
//
// There is no check-then-write race to lose here, which is the point of the
// shape: on the single-writer pool (dbx's _txlock=immediate design — see the
// dbx package doc) there is no concurrent second writer to race in the first
// place, but the statement is written to be correct even without that
// guarantee: the INSERT ... ON CONFLICT (bot_id) DO UPDATE carries the
// ownership predicate in its own WHERE clause, so the update applies only
// when the existing row already belongs to this same organization. A
// conflicting row owned by someone else (or orphaned) matches nothing,
// RETURNING yields zero rows, and the whole transaction — credential write
// included — rolls back. Verified empirically that SQLite's conditional
// upsert WHERE clause has the identical "no match -> zero rows, row
// untouched" semantics as Postgres's, not merely a syntax-compatible one.
//
// The same statement is what revives a soft-deleted account of this org:
// deleted_at = NULL, and because the id is derived from the bot id, its chats
// and messages come back with it.
func (s *Store) ClaimTelegramAccount(ctx context.Context, in TelegramClaim) (TelegramAccount, error) {
	if s.creds == nil {
		return TelegramAccount{}, ErrNoCredentialsKey
	}
	sealed, err := s.creds.Seal([]byte(in.BotToken))
	if err != nil {
		return TelegramAccount{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return TelegramAccount{}, err
	}
	defer tx.Rollback(ctx)

	acct, err := scanTgAccount(tx.QueryRow(ctx, `
		INSERT INTO tg_accounts
			(id, organization_id, display_name, bot_id, bot_username, connection_state)
		VALUES ($1, $2, $3, $4, $5, 'connecting')
		ON CONFLICT (bot_id) DO UPDATE
		   SET display_name = CASE WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name
		                           ELSE tg_accounts.display_name END,
		       bot_username = EXCLUDED.bot_username,
		       connection_state = 'connecting',
		       deleted_at = NULL,
		       updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		 WHERE tg_accounts.organization_id = EXCLUDED.organization_id
		RETURNING `+tgAccountCols,
		in.ID, in.OrganizationID, in.DisplayName, in.BotID, in.BotUsername))
	if errors.Is(err, dbx.ErrNoRows) {
		return TelegramAccount{}, ErrAccountClaimed
	}
	if err != nil {
		return TelegramAccount{}, wrap("claim telegram account", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO tg_credentials (account_id, bot_token_enc, encryption_key_version)
		VALUES ($1, $2, $3)
		ON CONFLICT (account_id) DO UPDATE SET
			bot_token_enc = EXCLUDED.bot_token_enc,
			encryption_key_version = EXCLUDED.encryption_key_version,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`,
		acct.ID, sealed, secretbox.KeyVersion); err != nil {
		return TelegramAccount{}, wrap("store telegram credentials", err)
	}
	return acct, tx.Commit(ctx)
}

// ReplaceTelegramToken swaps an account's stored token. It refuses a token for
// a different bot: the account id is derived from bot_id, so accepting one
// would silently point an existing conversation history at a different bot.
func (s *Store) ReplaceTelegramToken(ctx context.Context, accountID uuid.UUID, botID int64, botUsername, token string) error {
	if s.creds == nil {
		return ErrNoCredentialsKey
	}
	sealed, err := s.creds.Seal([]byte(token))
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE tg_accounts SET bot_username = $3, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND bot_id = $2 AND deleted_at IS NULL
		RETURNING id`, accountID, botID, botUsername).Scan(&id)
	if errors.Is(err, dbx.ErrNoRows) {
		return ErrAccountClaimed
	}
	if err != nil {
		return wrap("replace telegram token", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tg_credentials (account_id, bot_token_enc, encryption_key_version)
		VALUES ($1, $2, $3)
		ON CONFLICT (account_id) DO UPDATE SET
			bot_token_enc = EXCLUDED.bot_token_enc,
			encryption_key_version = EXCLUDED.encryption_key_version,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`,
		accountID, sealed, secretbox.KeyVersion); err != nil {
		return wrap("store telegram credentials", err)
	}
	return tx.Commit(ctx)
}

// TelegramBotToken decrypts an account's bot token. Resolved at the moment of
// the call so the plaintext never has to live in a queue payload, a DTO or a
// log line to reach the sender.
func (s *Store) TelegramBotToken(ctx context.Context, accountID uuid.UUID) (string, error) {
	if s.creds == nil {
		return "", ErrNoCredentialsKey
	}
	var sealed []byte
	err := s.db.QueryRow(ctx,
		`SELECT bot_token_enc FROM tg_credentials WHERE account_id = $1`, accountID).Scan(&sealed)
	if errors.Is(err, dbx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	plain, err := s.creds.Open(sealed)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// TgAccountByID returns a live (non-deleted) Telegram account. A soft-deleted or
// unknown id is ErrNotFound — which is exactly what lets the webhook discard a
// stale delivery for a disconnected bot with a plain 200.
func (s *Store) TgAccountByID(ctx context.Context, id uuid.UUID) (TelegramAccount, error) {
	a, err := scanTgAccount(s.db.QueryRow(ctx, `SELECT `+tgAccountCols+`
		FROM tg_accounts WHERE id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, dbx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// TgAccountByIDForOrg is TgAccountByID plus the org membership guard.
func (s *Store) TgAccountByIDForOrg(ctx context.Context, id, orgID uuid.UUID) (TelegramAccount, error) {
	a, err := scanTgAccount(s.db.QueryRow(ctx, `SELECT `+tgAccountCols+`
		FROM tg_accounts
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`, id, orgID))
	if errors.Is(err, dbx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// TelegramWebhookState is one transition of the connection state machine.
type TelegramWebhookState struct {
	State string // connecting|connected|webhook_error|token_error|disconnect_pending|disconnect_error
	// URL, when non-empty, records the webhook we believe is registered.
	URL string
	// LastError is Telegram's own rejection text (empty clears it).
	LastError string
	// Registered stamps webhook_registered_at (a successful setWebhook).
	Registered bool
	// Checked stamps webhook_last_checked_at (a health probe ran).
	Checked bool
}

// SetTelegramWebhookState records a transition plus its health columns.
func (s *Store) SetTelegramWebhookState(ctx context.Context, id uuid.UUID, st TelegramWebhookState) error {
	_, err := s.db.Exec(ctx, `
		UPDATE tg_accounts SET
			connection_state = $2,
			webhook_url = CASE WHEN $3 <> '' THEN $3 ELSE webhook_url END,
			webhook_last_error = $4,
			webhook_registered_at = CASE WHEN $5 THEN strftime('%Y-%m-%d %H:%M:%f','now') ELSE webhook_registered_at END,
			webhook_last_checked_at = CASE WHEN $6 THEN strftime('%Y-%m-%d %H:%M:%f','now') ELSE webhook_last_checked_at END,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, id, st.State, st.URL, st.LastError, st.Registered, st.Checked)
	return err
}

// TouchTelegramAccount stamps live activity (an inbound update arrived).
func (s *Store) TouchTelegramAccount(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE tg_accounts SET last_live_event_at = strftime('%Y-%m-%d %H:%M:%f','now'), updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

// ConfirmTelegramDisconnect is the SECOND half of a delete: it runs only after
// Telegram has confirmed the webhook is gone. It hard-deletes the credential
// (the token is the thing worth destroying promptly) and soft-deletes the
// account, so the chats drop out of the inbox but survive for a re-connect.
func (s *Store) ConfirmTelegramDisconnect(ctx context.Context, id uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM tg_credentials WHERE account_id = $1`, id); err != nil {
		return wrap("delete telegram credentials", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE tg_accounts
		SET deleted_at = strftime('%Y-%m-%d %H:%M:%f','now'), connection_state = 'disconnected',
		    webhook_url = '', webhook_registered_at = NULL, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		return wrap("soft delete telegram account", err)
	}
	return tx.Commit(ctx)
}

// --- inbound ingestion ------------------------------------------------------

// TgMedia is the attachment on an inbound Telegram message. The bytes are NOT
// fetched at ingest time: the row is written with download_status='pending' and
// the file handles preserved, so the download can be retried from the row
// itself (there is no event to replay).
type TgMedia struct {
	FileID       string
	FileUniqueID string
	MediaType    string
	Mimetype     string
	FileName     string
	Size         int
}

// TgInbound is one normalized inbound Telegram message.
type TgInbound struct {
	AccountID uuid.UUID
	UpdateID  int64

	TelegramChatID int64
	ChatType       string

	TelegramUserID int64
	Username       string
	FirstName      string
	LastName       string
	DisplayName    string

	TelegramMessageID int64
	MessageKind       string
	Body              string
	Preview           string
	Raw               []byte
	MessageTS         time.Time

	Media *TgMedia
}

// TgInboundResult reports what the ingest did, so the webhook knows which SSE
// to emit and whether this was a redelivery.
type TgInboundResult struct {
	ContactID       uuid.UUID
	ChatID          uuid.UUID
	MessageID       uuid.UUID
	ChatCreated     bool
	MessageInserted bool
}

// IngestTelegramInbound upserts contact → chat → message and maintains the chat
// aggregates, all in ONE transaction that commits BEFORE the webhook acks. That
// is the whole durability story: Telegram redelivers anything we do not answer
// 2xx, and a redelivery lands on the unique keys below as a no-op
// (MessageInserted=false) — which is why there is no inbound_events table.
func (s *Store) IngestTelegramInbound(ctx context.Context, in TgInbound) (TgInboundResult, error) {
	var res TgInboundResult
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx, `
		INSERT INTO tg_contacts
			(account_id, telegram_user_id, username, first_name, last_name, display_name)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (account_id, telegram_user_id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			display_name = CASE WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name
			                    ELSE tg_contacts.display_name END,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		RETURNING id`,
		in.AccountID, in.TelegramUserID, in.Username, in.FirstName, in.LastName, in.DisplayName).
		Scan(&res.ContactID); err != nil {
		return res, wrap("upsert tg contact", err)
	}

	chatType := in.ChatType
	if chatType == "" {
		chatType = "private"
	}
	// two-step insert-detection (SQLite has no xmax) — see the per-PG-ism
	// translation table and internal/store/ingest.go's upsertChatTwoStep.
	err = tx.QueryRow(ctx, `
		INSERT INTO tg_chats (account_id, contact_id, telegram_chat_id, chat_type)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (account_id, telegram_chat_id) DO NOTHING
		RETURNING id`,
		in.AccountID, res.ContactID, in.TelegramChatID, chatType).Scan(&res.ChatID)
	switch {
	case err == nil:
		res.ChatCreated = true
	case errors.Is(err, dbx.ErrNoRows):
		if err := tx.QueryRow(ctx, `
			UPDATE tg_chats SET updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE account_id = $1 AND telegram_chat_id = $2
			RETURNING id`, in.AccountID, in.TelegramChatID).Scan(&res.ChatID); err != nil {
			return res, wrap("update tg chat", err)
		}
	default:
		return res, wrap("upsert tg chat", err)
	}

	var ts any
	if !in.MessageTS.IsZero() {
		ts = in.MessageTS
	}
	// Unconstrained ON CONFLICT DO NOTHING, not a targeted one: BOTH unique keys
	// on tg_messages can fire (the update id on a redelivery, the per-chat
	// message id if Telegram ever re-sends the same message under a new update
	// id). Naming only one would turn the other into a 500 that Telegram then
	// retries forever.
	var inserted bool
	err = tx.QueryRow(ctx, `
		INSERT INTO tg_messages
			(account_id, chat_id, direction, sender_kind, telegram_update_id, telegram_message_id,
			 message_kind, body, delivery_state, source, raw, message_ts)
		VALUES ($1, $2, 'in', 'contact', $3, $4, $5, $6, 'delivered', 'live_webhook', $7, $8)
		ON CONFLICT DO NOTHING
		RETURNING id`,
		in.AccountID, res.ChatID, in.UpdateID, nullableInt64(in.TelegramMessageID),
		in.MessageKind, in.Body, jsonbOrNil(in.Raw), ts).Scan(&res.MessageID)
	switch {
	case err == nil:
		inserted = true
	case errors.Is(err, dbx.ErrNoRows):
		// A redelivery: find the row the first delivery wrote, by whichever key
		// collided, so the caller can still reconcile against it.
		if err := tx.QueryRow(ctx, `
			SELECT id FROM tg_messages
			WHERE (account_id = $1 AND telegram_update_id = $2)
			   OR (chat_id = $3 AND telegram_message_id = $4)
			LIMIT 1`,
			in.AccountID, in.UpdateID, res.ChatID, nullableInt64(in.TelegramMessageID)).
			Scan(&res.MessageID); err != nil {
			return res, wrap("resolve duplicate tg message", err)
		}
	default:
		return res, wrap("insert tg message", err)
	}
	res.MessageInserted = inserted

	if inserted {
		if _, err := tx.Exec(ctx, `
			UPDATE tg_chats SET
				last_message_at = COALESCE($2, strftime('%Y-%m-%d %H:%M:%f','now')),
				last_message_preview = $3,
				unread_count = unread_count + 1,
				updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE id = $1`, res.ChatID, ts, in.Preview); err != nil {
			return res, wrap("update tg aggregates", err)
		}
		if in.Media != nil {
			if _, err := tx.Exec(ctx, `
				INSERT INTO tg_message_media
					(message_id, file_id, file_unique_id, media_type, mimetype, filename, size, download_status)
				VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
				ON CONFLICT (message_id) DO NOTHING`,
				res.MessageID, in.Media.FileID, in.Media.FileUniqueID, in.Media.MediaType,
				in.Media.Mimetype, in.Media.FileName, in.Media.Size); err != nil {
				return res, wrap("insert tg media", err)
			}
		}
	}
	return res, tx.Commit(ctx)
}

// --- media ------------------------------------------------------------------

// TgMediaMeta is a stored attachment's descriptive fields.
type TgMediaMeta struct {
	MediaType string
	Mimetype  string
	FileName  string
	FileID    string
}

// TelegramMediaMeta returns a message's attachment metadata.
func (s *Store) TelegramMediaMeta(ctx context.Context, messageID uuid.UUID) (TgMediaMeta, error) {
	var m TgMediaMeta
	err := s.db.QueryRow(ctx, `
		SELECT media_type, mimetype, filename, file_id
		FROM tg_message_media WHERE message_id = $1`, messageID).
		Scan(&m.MediaType, &m.Mimetype, &m.FileName, &m.FileID)
	if errors.Is(err, dbx.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}

// SetTelegramMediaReady records a completed download.
func (s *Store) SetTelegramMediaReady(ctx context.Context, messageID uuid.UUID, storageKey string, size int) error {
	_, err := s.db.Exec(ctx, `
		UPDATE tg_message_media
		SET storage_key = $2, size = $3, download_status = 'ready', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE message_id = $1`, messageID, storageKey, size)
	return err
}

// SetTelegramMediaFailed marks a download attempt failed. The row keeps its
// file handles, so the sweeper can retry it; touching updated_at is what gives
// that retry a backoff instead of a hot loop.
func (s *Store) SetTelegramMediaFailed(ctx context.Context, messageID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE tg_message_media
		SET download_status = 'failed', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE message_id = $1 AND download_status <> 'ready'`, messageID)
	return err
}

// PendingTelegramMedia is one attachment still awaiting its bytes.
type PendingTgMedia struct {
	MessageID uuid.UUID
	AccountID uuid.UUID
	FileID    string
}

// PendingTelegramMedia lists attachments whose bytes were never fetched and
// which have been quiet for at least olderThan — the sweeper's work list, and
// the reason a crashed or failed download is recoverable from the database
// alone. Soft-deleted accounts are skipped: their token is gone.
//
// now() - $1::interval becomes a Go-computed cutoff bound directly (see the
// per-PG-ism translation table) — SQLite has no INTERVAL type to cast a
// duration string into.
func (s *Store) PendingTelegramMedia(ctx context.Context, olderThan time.Duration, limit int) ([]PendingTgMedia, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	cutoff := time.Now().Add(-olderThan).UTC()
	rows, err := s.db.Query(ctx, `
		SELECT mm.message_id, m.account_id, mm.file_id
		FROM tg_message_media mm
		JOIN tg_messages m ON m.id = mm.message_id
		JOIN tg_accounts a ON a.id = m.account_id
		WHERE mm.download_status <> 'ready'
		  AND mm.file_id <> ''
		  AND a.deleted_at IS NULL
		  AND mm.updated_at < $1
		ORDER BY mm.updated_at
		LIMIT $2`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingTgMedia
	for rows.Next() {
		var p PendingTgMedia
		if err := rows.Scan(&p.MessageID, &p.AccountID, &p.FileID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// nullableInt64 keeps a zero id out of the column so the (chat_id,
// telegram_message_id) unique key stays free for other rows.
func nullableInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
