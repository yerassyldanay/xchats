package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
	"github.com/yerassyldanay/xchats/backend/internal/dbx"
)

// ---------------------------------------------------------------------------
// Channel dispatch
// ---------------------------------------------------------------------------
// The unified views are read-only; every write has to land in the channel's own
// transport table. These two helpers are the single place that mapping lives.
//
// The wa_* gateway carries BOTH the whatsapp and simulator channels (the
// simulator is wa_accounts rows with channel='simulator'), so both map there.
// Instagram, Messenger and WhatsApp Cloud all carry the SAME generic
// channel_chats/channel_messages tables (see migration 0011) — one arm, not
// three, and the reason a future fourth channel costs no new table-dispatch
// case at all as long as it also fits the generic core. An empty channel
// means "a caller that predates the channel column" and is treated as
// whatsapp — the same table it has always written to. Anything else is a
// programming error and is reported, never silently routed to wa_*.

func chatsTableFor(channel string) (string, error) {
	switch channel {
	case string(chanTelegram):
		return "tg_chats", nil
	case string(chanInstagram), string(chanMessenger), string(chanWhatsAppCloud):
		return "channel_chats", nil
	case string(chanWhatsApp), string(chanSimulator), "":
		return "wa_chats", nil
	default:
		return "", fmt.Errorf("store: no chat table for channel %q", channel)
	}
}

func messagesTableFor(channel string) (string, error) {
	switch channel {
	case string(chanTelegram):
		return "tg_messages", nil
	case string(chanInstagram), string(chanMessenger), string(chanWhatsAppCloud):
		return "channel_messages", nil
	case string(chanWhatsApp), string(chanSimulator), "":
		return "wa_messages", nil
	default:
		return "", fmt.Errorf("store: no message table for channel %q", channel)
	}
}

// The channel names, duplicated here as plain strings: internal/store must not
// import backend/messaging (the contracts package deliberately depends on
// nothing, and the dependency would run the wrong way).
type channelName string

const (
	chanWhatsApp      channelName = "whatsapp"
	chanSimulator     channelName = "simulator"
	chanTelegram      channelName = "telegram"
	chanInstagram     channelName = "instagram"
	chanMessenger     channelName = "messenger"
	chanWhatsAppCloud channelName = "whatsapp_cloud"
)

// isChannelCoreChannel reports whether channel is one of the generic
// channel_* tables' channels — the dispatch UpsertOutboundMedia and
// MediaStorageURL need beyond chatsTableFor/messagesTableFor, since those
// two functions dispatch by TABLE NAME while media additionally needs its
// own per-gateway table (message_media / tg_message_media /
// channel_message_media).
func isChannelCoreChannel(channel string) bool {
	switch channel {
	case string(chanInstagram), string(chanMessenger), string(chanWhatsAppCloud):
		return true
	default:
		return false
	}
}

// InboundUpsert is the worker's normalized input for one message event.
type InboundUpsert struct {
	AccountID         uuid.UUID
	PhoneJID          string
	LidJID            string
	RemoteJID         string
	PhoneNumber       string
	PushName          string
	Direction         string // in|out
	SenderKind        string // contact|external_account
	ExternalMessageID string
	MessageKind       string
	Body              string
	Preview           string // chat-list preview (media → placeholder)
	Source            string
	Raw               []byte
	MessageTS         time.Time
}

// InboundResult reports what the upsert did, so the worker knows which SSE to emit.
type InboundResult struct {
	ContactID uuid.UUID
	ChatID    uuid.UUID
	MessageID uuid.UUID
	// CustomerID is the CRM customer this contact resolved to, or uuid.Nil
	// when the owning account belongs to no organization (see UpsertInbound).
	CustomerID      uuid.UUID
	ChatCreated     bool
	MessageInserted bool
}

// UpsertInbound idempotently upserts contact → chat → message and maintains the
// chat aggregates, all in one transaction. Re-delivery (same external_message_id)
// is a no-op upsert (MessageInserted=false).
func (s *Store) UpsertInbound(ctx context.Context, in InboundUpsert) (InboundResult, error) {
	var res InboundResult
	tx, err := s.db.Begin(ctx)
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
		INSERT INTO wa_contacts (account_id, phone_jid, lid_jid, phone_number, push_name, display_name)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (account_id, phone_jid) DO UPDATE SET
			lid_jid = COALESCE(EXCLUDED.lid_jid, wa_contacts.lid_jid),
			push_name = CASE WHEN EXCLUDED.push_name <> '' THEN EXCLUDED.push_name ELSE wa_contacts.push_name END,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		RETURNING id`,
		in.AccountID, in.PhoneJID, lid, in.PhoneNumber, in.PushName).Scan(&res.ContactID); err != nil {
		return res, wrap("upsert contact", err)
	}

	// chat — two-step insert-detection (SQLite has no xmax): try a
	// conflict-free insert first; a duplicate falls through to an explicit
	// update. See dbx package doc and the per-PG-ism translation table.
	res.ChatID, res.ChatCreated, err = upsertChatTwoStep(ctx, tx, in.AccountID, res.ContactID, in.RemoteJID)
	if err != nil {
		return res, wrap("upsert chat", err)
	}

	// CRM identity — find-or-create the customer this contact belongs to, in
	// this same transaction so a message and its customer link commit together
	// and a redelivery cannot produce a second customer. An account with no
	// organization resolves to uuid.Nil and is skipped: its chats are already
	// invisible to every org's inbox, so there is no tenant to own the customer.
	orgID, channel, err := accountOwner(ctx, tx, "wa_accounts", in.AccountID)
	if err != nil {
		return res, err
	}
	res.CustomerID, err = ResolveCustomerForContact(ctx, tx, orgID, IdentityInput{
		Channel:     channel,
		AccountID:   in.AccountID,
		ContactID:   res.ContactID,
		ExternalID:  in.PhoneJID,
		Phone:       in.PhoneNumber,
		DisplayName: in.PushName,
	})
	if err != nil {
		return res, err
	}

	// message (dedup on the natural key; preserves sender_kind on conflict)
	var extID any
	if in.ExternalMessageID != "" {
		extID = in.ExternalMessageID
	}
	var ts any
	if !in.MessageTS.IsZero() {
		ts = in.MessageTS
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO wa_messages
			(account_id, chat_id, direction, sender_kind, external_message_id, message_kind, body, delivery_state, source, raw, message_ts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (account_id, external_message_id) DO NOTHING
		RETURNING id`,
		in.AccountID, res.ChatID, in.Direction, in.SenderKind, extID, in.MessageKind, in.Body,
		deliveryStateFor(in.Direction), in.Source, jsonbOrNil(in.Raw), ts).Scan(&res.MessageID)
	switch {
	case err == nil:
		res.MessageInserted = true
	case errors.Is(err, dbx.ErrNoRows):
		// Already exists: an explicit UPDATE (preserving body if already set,
		// same as the old ON CONFLICT DO UPDATE) + RETURNING to get its id.
		if err := tx.QueryRow(ctx, `
			UPDATE wa_messages SET
				body = CASE WHEN body = '' THEN $3 ELSE body END,
				updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE account_id = $1 AND external_message_id = $2
			RETURNING id`,
			in.AccountID, extID, in.Body).Scan(&res.MessageID); err != nil {
			return res, wrap("update duplicate message", err)
		}
	default:
		return res, wrap("upsert message", err)
	}

	// aggregates — only on a genuine new message
	if res.MessageInserted {
		unreadDelta := "unread_count = unread_count + 1"
		if in.Direction == "out" {
			unreadDelta = "unread_count = 0"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE wa_chats SET
				last_message_at = COALESCE($2, strftime('%Y-%m-%d %H:%M:%f','now')),
				last_message_preview = $3,
				`+unreadDelta+`,
				updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE id = $1`, res.ChatID, ts, in.Preview); err != nil {
			return res, wrap("update aggregates", err)
		}

		// Campaign-only chats (chat_state='campaign' — see
		// MarkChatCampaignOnly's own doc comment) graduate to a normal 'open'
		// chat the moment ANY genuine activity lands on them, by the same
		// MessageInserted reasoning the suppression block below uses: a
		// customer reply or an operator's own out-of-band send both mean this
		// is no longer just a cold campaign send sitting outside the normal
		// inbox. A no-op UPDATE (WHERE chat_state = 'campaign' matches
		// nothing) for every chat that was never campaign-only to begin with.
		if _, err := tx.Exec(ctx, `
			UPDATE wa_chats SET chat_state = 'open', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE id = $1 AND chat_state = 'campaign'`, res.ChatID); err != nil {
			return res, wrap("graduate campaign chat", err)
		}

		// Reply suppression (plan: Campaigns): a genuinely new message on
		// this identity — a customer's inbound reply, or an operator's own
		// send from their phone that bypassed xchats' compose UI entirely
		// (which is why this is keyed on MessageInserted rather than
		// Direction: a send made THROUGH xchats' own compose already has its
		// row from InsertOutbound, so its later fromMe echo collapses onto
		// that existing row here rather than inserting a new one, and
		// correctly does NOT re-trigger this) — means the identity has now
		// been contacted outside the campaign machinery. Every still-pending
		// row for it, in every RUNNING campaign on this same account, is
		// marked 'skipped' in the SAME transaction as this upsert, so the
		// two can never disagree about whether it happened.
		if in.PhoneNumber != "" {
			if identity, ok := purecampaign.NormalizePhone(in.PhoneNumber, purecampaign.DefaultCountryCode); ok {
				if _, err := suppressPendingForIdentity(ctx, tx, in.AccountID, identity, "customer contact detected outside the campaign"); err != nil {
					return res, err
				}
			}
		}
	}

	return res, tx.Commit(ctx)
}

// upsertChatTwoStep is UpsertInbound and FindOrCreateChat's shared wa_chats
// upsert-with-insert-detection — see the two-step comment on the call above.
func upsertChatTwoStep(ctx context.Context, tx *dbx.Tx, accountID, contactID uuid.UUID, remoteJID string) (uuid.UUID, bool, error) {
	var chatID uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO wa_chats (account_id, contact_id, remote_jid)
		VALUES ($1, $2, $3)
		ON CONFLICT (account_id, remote_jid) DO NOTHING
		RETURNING id`,
		accountID, contactID, remoteJID).Scan(&chatID)
	switch {
	case err == nil:
		return chatID, true, nil
	case errors.Is(err, dbx.ErrNoRows):
		err := tx.QueryRow(ctx, `
			UPDATE wa_chats SET updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE account_id = $1 AND remote_jid = $2
			RETURNING id`, accountID, remoteJID).Scan(&chatID)
		return chatID, false, err
	default:
		return chatID, false, err
	}
}

func deliveryStateFor(direction string) string {
	if direction == "out" {
		return "sent" // an external/echo outbound is already at the gateway
	}
	return "delivered"
}

// AdvanceDeliveryState applies a status update, monotonically (never
// backwards), on the channel's own message table. Returns the message + chat
// id on a real change, ErrNotFound otherwise. Today's only callers are all
// WhatsApp (whatsmeow receipts); WhatsApp Cloud API status webhooks are the
// next caller this dispatch is for — Telegram bots get no delivery receipts
// at all, and Instagram/Messenger DMs have no delivery-status webhook either.
func (s *Store) AdvanceDeliveryState(ctx context.Context, channel string, accountID uuid.UUID, externalMessageID, newState string, newRank int) (uuid.UUID, uuid.UUID, error) {
	table, err := messagesTableFor(channel)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	var msgID, chatID uuid.UUID
	err = s.db.QueryRow(ctx, `
		UPDATE `+table+` SET delivery_state = $3, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE account_id = $1 AND external_message_id = $2
		  AND (CASE delivery_state
				WHEN 'queued' THEN 0 WHEN 'sent' THEN 1 WHEN 'delivered' THEN 2
				WHEN 'read' THEN 3 WHEN 'failed' THEN 4 ELSE 0 END) < $4
		RETURNING id, chat_id`,
		accountID, externalMessageID, newState, newRank).Scan(&msgID, &chatID)
	if errors.Is(err, dbx.ErrNoRows) {
		return msgID, chatID, ErrNotFound
	}
	return msgID, chatID, err
}

// UpsertOutboundMedia records the attachment on an outbound message in the
// channel's own media table. The WhatsApp table's FK points at wa_messages, so
// a Telegram row written there would be rejected — this is the dispatch that
// keeps the send path channel-neutral above it.
//
// The Telegram row carries an empty file_id on purpose: these bytes are OURS
// (a blob we already hold), not something Telegram hosts, and the download
// sweeper explicitly skips rows without a file_id.
func (s *Store) UpsertOutboundMedia(ctx context.Context, channel string, messageID uuid.UUID, m MediaRef, storageKey string) error {
	if channel == string(chanTelegram) {
		_, err := s.db.Exec(ctx, `
			INSERT INTO tg_message_media
				(message_id, file_id, file_unique_id, media_type, mimetype, filename, size, storage_key, download_status)
			VALUES ($1, '', '', $2, $3, $4, $5, $6, 'ready')
			ON CONFLICT (message_id) DO UPDATE SET
				storage_key = EXCLUDED.storage_key,
				download_status = 'ready',
				updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`,
			messageID, m.MediaType, m.Mimetype, m.FileName, m.FileSize, storageKey)
		return err
	}
	if isChannelCoreChannel(channel) {
		// provider_ref/source_url are the INBOUND fetch-from-provider fields
		// (see channels.go's InsertChannelMediaPending) and stay empty here:
		// an outbound attachment is always bytes we already hold.
		_, err := s.db.Exec(ctx, `
			INSERT INTO channel_message_media
				(message_id, media_type, mimetype, filename, size, storage_key, download_status)
			VALUES ($1, $2, $3, $4, $5, $6, 'ready')
			ON CONFLICT (message_id) DO UPDATE SET
				storage_key = EXCLUDED.storage_key,
				download_status = 'ready',
				updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`,
			messageID, m.MediaType, m.Mimetype, m.FileName, m.FileSize, storageKey)
		return err
	}
	_, _, err := s.UpsertMessageMedia(ctx, messageID, m, storageKey, "ready")
	return err
}

// UpsertMessageMedia inserts a media row for a message (idempotent on UNIQUE(message_id)).
func (s *Store) UpsertMessageMedia(ctx context.Context, messageID uuid.UUID, m MediaRef, storageURL, downloadStatus string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	var inserted bool
	err := s.db.QueryRow(ctx, `
		INSERT INTO message_media (message_id, media_type, mimetype, file_name, file_size, storage_url, download_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (message_id) DO NOTHING
		RETURNING id`,
		messageID, m.MediaType, m.Mimetype, m.FileName, m.FileSize, storageURL, downloadStatus).Scan(&id)
	switch {
	case err == nil:
		inserted = true
	case errors.Is(err, dbx.ErrNoRows):
		err = s.db.QueryRow(ctx, `
			UPDATE message_media SET updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE message_id = $1
			RETURNING id`, messageID).Scan(&id)
	}
	return id, inserted, err
}

// SetMediaReady marks a media row downloaded and records its byte size.
func (s *Store) SetMediaReady(ctx context.Context, messageID uuid.UUID, fileSize int) error {
	_, err := s.db.Exec(ctx, `
		UPDATE message_media SET download_status='ready', file_size=$2, updated_at=strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE message_id = $1`, messageID, fileSize)
	return err
}

// UpdateMediaTranscript persists an audio attachment's speech-to-text result
// on the channel's own media table — dispatched exactly like
// UpsertOutboundMedia, since "which table owns this message's media row"
// only ever depends on the channel. worker.go calls this once per audio
// note, right after internal/stt.Transcriber.Transcribe returns, so a
// redelivered download task (or a page reopened mid-transcription) never
// re-runs the STT call: the transcript column is the durable "already done"
// marker, not a queue state.
func (s *Store) UpdateMediaTranscript(ctx context.Context, channel string, messageID uuid.UUID, transcript string) error {
	table := "message_media"
	switch {
	case channel == string(chanTelegram):
		table = "tg_message_media"
	case isChannelCoreChannel(channel):
		table = "channel_message_media"
	}
	_, err := s.db.Exec(ctx, `
		UPDATE `+table+` SET transcript = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE message_id = $1`, messageID, transcript)
	return err
}

// MediaStorageURL resolves a public media id to its blob key, on any
// channel, SCOPED to orgID — a media row belonging to a different
// organization (or no row at all) is indistinguishable from ErrNotFound to
// the caller, by design: this is the query that closes the cross-org media
// IDOR (an authenticated user of one org could otherwise fetch any other
// org's media by guessing/enumerating uuids). Written directly against the
// base tables rather than inbox_message_media_v: that view has no
// organization_id column (it was never meant to carry authorization scope),
// and adding one there would change a view several other read paths share.
func (s *Store) MediaStorageURL(ctx context.Context, orgID, id uuid.UUID) (storageURL, mimetype, fileName string, err error) {
	err = s.db.QueryRow(ctx, `
		SELECT mm.storage_url, mm.mimetype, mm.file_name
		FROM message_media mm
		JOIN wa_messages m ON m.id = mm.message_id
		JOIN wa_accounts a ON a.id = m.account_id
		WHERE mm.id = $1 AND a.organization_id = $2
		UNION ALL
		SELECT tm.storage_key, tm.mimetype, tm.filename
		FROM tg_message_media tm
		JOIN tg_messages m ON m.id = tm.message_id
		JOIN tg_accounts a ON a.id = m.account_id
		WHERE tm.id = $1 AND a.organization_id = $2
		UNION ALL
		SELECT cmm.storage_key, cmm.mimetype, cmm.filename
		FROM channel_message_media cmm
		JOIN channel_messages m ON m.id = cmm.message_id
		JOIN channel_accounts a ON a.id = m.account_id
		WHERE cmm.id = $1 AND a.organization_id = $2`, id, orgID).
		Scan(&storageURL, &mimetype, &fileName)
	if errors.Is(err, dbx.ErrNoRows) {
		return "", "", "", ErrNotFound
	}
	return
}

// InsertOutbound creates a queued outbound row (the provider's own message id
// stays NULL until stamped) in the channel's transport table and bumps the chat
// aggregates. Used by the send pipeline for every channel.
func (s *Store) InsertOutbound(ctx context.Context, channel string, chatID, accountID uuid.UUID, senderKind string, senderUserID uuid.NullUUID, messageKind, body, preview string) (uuid.UUID, error) {
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
	defer tx.Rollback(ctx)
	var id uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO `+msgTable+`
			(account_id, chat_id, direction, sender_kind, sender_user_id, message_kind, body, delivery_state, source, message_ts)
		VALUES ($1, $2, 'out', $3, $4, $5, $6, 'queued', 'app', strftime('%Y-%m-%d %H:%M:%f','now'))
		RETURNING id`,
		accountID, chatID, senderKind, senderUserID, messageKind, body).Scan(&id); err != nil {
		return uuid.Nil, wrap("insert outbound", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE `+chatTable+` SET last_message_at = strftime('%Y-%m-%d %H:%M:%f','now'), last_message_preview = $2, unread_count = 0, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, chatID, preview); err != nil {
		return uuid.Nil, wrap("update aggregates", err)
	}
	return id, tx.Commit(ctx)
}

// FindOrCreateChat upserts a contact + chat for an outbound-initiated conversation
// — composing to a phone number that may have no inbound history yet. It mirrors
// the contact/chat half of UpsertInbound (minus a message) and returns the chat id
// plus whether the chat row was freshly created. The contact's display_name seeds
// to the phone number so a brand-new chat shows something until a pushName arrives.
func (s *Store) FindOrCreateChat(ctx context.Context, accountID uuid.UUID, phoneJID, phoneNumber string) (uuid.UUID, bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, false, err
	}
	defer tx.Rollback(ctx)

	var contactID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO wa_contacts (account_id, phone_jid, phone_number, display_name)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (account_id, phone_jid) DO UPDATE SET updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		RETURNING id`,
		accountID, phoneJID, phoneNumber).Scan(&contactID); err != nil {
		return uuid.Nil, false, wrap("upsert contact", err)
	}

	chatID, created, err := upsertChatTwoStep(ctx, tx, accountID, contactID, phoneJID)
	if err != nil {
		return uuid.Nil, false, wrap("upsert chat", err)
	}
	return chatID, created, tx.Commit(ctx)
}

// StampOutboundSent records the provider's own id for a just-sent outbound row
// and flips it to 'sent'. On WhatsApp that id is what lets the fromMe=true echo
// collapse onto this row instead of duplicating; on Telegram it is the
// sendMessage response's message_id (bots get no echo, and no delivery
// receipts either, so 'sent' is the terminal success state there).
func (s *Store) StampOutboundSent(ctx context.Context, channel string, messageID uuid.UUID, externalID string) error {
	table, err := messagesTableFor(channel)
	if err != nil {
		return err
	}
	if channel == string(chanTelegram) {
		// telegram_message_id has INTEGER affinity; an empty id (a send that
		// reported no message) stays NULL rather than failing the cast. A
		// well-formed decimal string is converted to INTEGER by SQLite's
		// column-affinity coercion on write — verified empirically, no
		// explicit cast needed (the old ::bigint cast is dropped).
		var msgIDArg any
		if externalID != "" {
			msgIDArg = externalID
		}
		_, err := s.db.Exec(ctx, `
			UPDATE `+table+` SET telegram_message_id = $2, delivery_state = 'sent', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE id = $1`, messageID, msgIDArg)
		return err
	}
	_, err = s.db.Exec(ctx, `
		UPDATE `+table+` SET external_message_id = $2, delivery_state = 'sent', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, messageID, externalID)
	return err
}

// SetDeliveryStateFor forces a delivery state (e.g. 'failed' when a send errors)
// on the channel's own transport table.
func (s *Store) SetDeliveryStateFor(ctx context.Context, channel string, messageID uuid.UUID, state string) error {
	table, err := messagesTableFor(channel)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx,
		`UPDATE `+table+` SET delivery_state = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now') WHERE id = $1`, messageID, state)
	return err
}

func jsonbOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
