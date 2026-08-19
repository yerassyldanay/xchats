// Package store (this file): the generic channel core behind Instagram,
// Messenger and WhatsApp Cloud. Mirrors telegram.go's shape (claim, seal
// credentials, ingest inbound, sweep media) but parametrized by `channel`
// instead of hardcoded to one provider, since all three Meta channels share
// one set of tables (channel_accounts/.../channel_message_media — see
// migration 0011). Provider vocabulary (IGSID, PSID, phone_number_id, ...)
// stays out of this file entirely; callers translate it into the neutral
// External*/ProviderMeta fields below.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yerassyldanay/xchats/backend/internal/dbx"
	"github.com/yerassyldanay/xchats/backend/internal/secretbox"
)

// ErrChannelAccountClaimed mirrors ErrAccountClaimed for the generic core:
// this external account already belongs to a DIFFERENT organization (or is
// orphaned), and adopting it would inherit that tenant's history.
var ErrChannelAccountClaimed = errors.New("store: this account already belongs to another organization")

// --- accounts ---------------------------------------------------------------

// ChannelAccount is one channel_accounts row.
type ChannelAccount struct {
	ID                   uuid.UUID
	OrganizationID       uuid.NullUUID
	Channel              string
	ExternalAccountID    string
	DisplayName          string
	Handle               string
	ConnectionState      string
	WebhookURL           string
	WebhookRegisteredAt  *time.Time
	WebhookLastCheckedAt *time.Time
	WebhookLastError     string
	LastLiveEventAt      *time.Time
	ProviderMeta         []byte
	DeletedAt            *time.Time
	CreatedAt            time.Time
}

const channelAccountCols = `id, organization_id, channel, external_account_id, display_name, handle,
	connection_state, webhook_url, webhook_registered_at, webhook_last_checked_at, webhook_last_error,
	last_live_event_at, provider_meta, deleted_at, created_at`

func scanChannelAccount(row dbx.Scanner) (ChannelAccount, error) {
	var a ChannelAccount
	err := row.Scan(&a.ID, &a.OrganizationID, &a.Channel, &a.ExternalAccountID, &a.DisplayName, &a.Handle,
		&a.ConnectionState, &a.WebhookURL, &a.WebhookRegisteredAt, &a.WebhookLastCheckedAt, &a.WebhookLastError,
		&a.LastLiveEventAt, &a.ProviderMeta, &a.DeletedAt, &a.CreatedAt)
	return a, err
}

// ChannelAccountClaim is one "connect this external account to this
// organization" request. Unlike TelegramClaim, it does NOT carry a secret:
// Meta's OAuth flow resolves identity (account discovery) and the token
// itself in separate steps (a WhatsApp Cloud claim, for instance, only knows
// its phone_number_id after a Graph API call the business token itself
// authorizes), so claiming the account and storing its credentials are two
// calls here — SetChannelCredentials follows once the caller has both.
type ChannelAccountClaim struct {
	ID                uuid.UUID // config.ChannelAccountID(ownerRef) — see config.go's MetaOwnerRef-style helpers
	OrganizationID    uuid.UUID
	Channel           string
	ExternalAccountID string
	DisplayName       string
	Handle            string
	ProviderMeta      []byte // raw JSON; nil/empty becomes '{}'
}

// ClaimChannelAccount atomically claims an external account for an
// organization. The ownership predicate lives in the UPDATE's own WHERE
// clause (exactly like ClaimTelegramAccount): a conflicting row owned by a
// different organization matches nothing, RETURNING yields zero rows, and
// the caller gets ErrChannelAccountClaimed rather than silently adopting
// someone else's conversation history. Re-claiming a soft-deleted account of
// this SAME org revives it (deleted_at = NULL), bringing its chats back.
func (s *Store) ClaimChannelAccount(ctx context.Context, in ChannelAccountClaim) (ChannelAccount, error) {
	meta := in.ProviderMeta
	if len(meta) == 0 {
		meta = []byte("{}")
	}
	acct, err := scanChannelAccount(s.db.QueryRow(ctx, `
		INSERT INTO channel_accounts
			(id, organization_id, channel, external_account_id, display_name, handle, connection_state, provider_meta)
		VALUES ($1, $2, $3, $4, $5, $6, 'connecting', $7)
		ON CONFLICT (channel, external_account_id) DO UPDATE
		   SET display_name = CASE WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name
		                           ELSE channel_accounts.display_name END,
		       handle = CASE WHEN EXCLUDED.handle <> '' THEN EXCLUDED.handle ELSE channel_accounts.handle END,
		       provider_meta = EXCLUDED.provider_meta,
		       connection_state = 'connecting',
		       deleted_at = NULL,
		       updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		 WHERE channel_accounts.organization_id = EXCLUDED.organization_id
		RETURNING `+channelAccountCols,
		in.ID, in.OrganizationID, in.Channel, in.ExternalAccountID, in.DisplayName, in.Handle, string(meta)))
	if errors.Is(err, dbx.ErrNoRows) {
		return ChannelAccount{}, ErrChannelAccountClaimed
	}
	if err != nil {
		return ChannelAccount{}, wrap("claim channel account", err)
	}
	return acct, nil
}

// ChannelAccountByID returns a live (non-deleted) channel account.
func (s *Store) ChannelAccountByID(ctx context.Context, id uuid.UUID) (ChannelAccount, error) {
	a, err := scanChannelAccount(s.db.QueryRow(ctx, `SELECT `+channelAccountCols+`
		FROM channel_accounts WHERE id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, dbx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// ChannelAccountByIDForOrg is ChannelAccountByID plus the org membership guard.
func (s *Store) ChannelAccountByIDForOrg(ctx context.Context, id, orgID uuid.UUID) (ChannelAccount, error) {
	a, err := scanChannelAccount(s.db.QueryRow(ctx, `SELECT `+channelAccountCols+`
		FROM channel_accounts
		WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`, id, orgID))
	if errors.Is(err, dbx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// ChannelAccountByExternalID resolves the account a webhook's own entry.id
// (IG_ID / PAGE_ID / WABA_ID) names — the lookup every webhook handler does
// before it can ingest anything. A soft-deleted or unknown pair is
// ErrNotFound, exactly what lets the webhook record "unknown_account" and
// ack with a plain 200 rather than erroring.
func (s *Store) ChannelAccountByExternalID(ctx context.Context, channel, externalAccountID string) (ChannelAccount, error) {
	a, err := scanChannelAccount(s.db.QueryRow(ctx, `SELECT `+channelAccountCols+`
		FROM channel_accounts WHERE channel = $1 AND external_account_id = $2 AND deleted_at IS NULL`,
		channel, externalAccountID))
	if errors.Is(err, dbx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// ChannelAccountsWithWebhook lists every live, previously-registered account
// on the given channel — deliberately install-wide, not org-scoped, since
// its two callers (the stale-origin repair pass and the channel-setup
// checklist's stale list) are both boot-time/admin maintenance concerns, not
// a single tenant's own view. "Previously-registered" (webhook_url <> "")
// excludes an account that was claimed but never got as far as a webhook
// registration — comparing an empty origin against the current public base
// URL would always spuriously read as stale.
func (s *Store) ChannelAccountsWithWebhook(ctx context.Context, channel string) ([]ChannelAccount, error) {
	rows, err := s.db.Query(ctx, `SELECT `+channelAccountCols+`
		FROM channel_accounts WHERE channel = $1 AND deleted_at IS NULL AND webhook_url <> ''
		ORDER BY created_at`, channel)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ChannelAccount
	for rows.Next() {
		a, err := scanChannelAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ChannelExternalAccountID resolves an account's provider-side identity
// (phone_number_id for whatsapp_cloud, an IGSID-space user id for
// instagram, a Page id for messenger) — the Graph API path segment a send
// needs, distinct from the account's own internal id. Exists as its own
// narrow method (rather than making every sender import store.ChannelAccount
// via ChannelAccountByID) so each channel adapter's own AccountSource-style
// interface can stay decoupled from this package, exactly mirroring how
// telegram.TokenSource never references a store type — see
// internal/whatsappcloud/channel.go.
func (s *Store) ChannelExternalAccountID(ctx context.Context, accountID uuid.UUID) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `
		SELECT external_account_id FROM channel_accounts WHERE id = $1 AND deleted_at IS NULL`, accountID).Scan(&id)
	if errors.Is(err, dbx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

// ChannelWebhookState is one transition of a channel account's connection
// state machine — mirrors TelegramWebhookState.
type ChannelWebhookState struct {
	State      string
	URL        string
	LastError  string
	Registered bool
	Checked    bool
}

// SetChannelWebhookState records a transition plus its health columns.
func (s *Store) SetChannelWebhookState(ctx context.Context, id uuid.UUID, st ChannelWebhookState) error {
	_, err := s.db.Exec(ctx, `
		UPDATE channel_accounts SET
			connection_state = $2,
			webhook_url = CASE WHEN $3 <> '' THEN $3 ELSE webhook_url END,
			webhook_last_error = $4,
			webhook_registered_at = CASE WHEN $5 THEN strftime('%Y-%m-%d %H:%M:%f','now') ELSE webhook_registered_at END,
			webhook_last_checked_at = CASE WHEN $6 THEN strftime('%Y-%m-%d %H:%M:%f','now') ELSE webhook_last_checked_at END,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1`, id, st.State, st.URL, st.LastError, st.Registered, st.Checked)
	return err
}

// SetChannelAccountState is the plain connection_state setter (no webhook
// health change) — used for states like 'token_expiring' the refresher
// worker sets without touching webhook bookkeeping.
func (s *Store) SetChannelAccountState(ctx context.Context, id uuid.UUID, state string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE channel_accounts SET connection_state = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND deleted_at IS NULL`, id, state)
	return err
}

// TouchChannelAccount stamps live activity (an inbound event arrived).
func (s *Store) TouchChannelAccount(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE channel_accounts SET last_live_event_at = strftime('%Y-%m-%d %H:%M:%f','now'), updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

// ConfirmChannelDisconnect is the SECOND half of a delete: it runs only
// after the provider side (an unsubscribe/override-clear call, made by the
// per-channel httpapi handler) has been confirmed. It hard-deletes the
// credential and soft-deletes the account, mirroring
// ConfirmTelegramDisconnect exactly.
func (s *Store) ConfirmChannelDisconnect(ctx context.Context, id uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM channel_credentials WHERE account_id = $1`, id); err != nil {
		return wrap("delete channel credentials", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE channel_accounts
		SET deleted_at = strftime('%Y-%m-%d %H:%M:%f','now'), connection_state = 'disconnected',
		    webhook_url = '', webhook_registered_at = NULL, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE id = $1 AND deleted_at IS NULL`, id); err != nil {
		return wrap("soft delete channel account", err)
	}
	return tx.Commit(ctx)
}

// --- credentials -------------------------------------------------------------
// One sealed-blob-plus-metadata shape for all three channels' tokens (an IG
// long-lived user token, a Page token, a WhatsApp Cloud business token) —
// see UseCredentialsBox in telegram.go, the same secretbox.Box this reuses.

// ChannelCredentialsWrite is one "store/replace this account's token" request.
type ChannelCredentialsWrite struct {
	AccountID uuid.UUID
	Secret    string // plaintext; sealed inside
	TokenKind string
	ExpiresAt *time.Time // nil means non-expiring
}

// SetChannelCredentials seals and (re)stores an account's token.
func (s *Store) SetChannelCredentials(ctx context.Context, in ChannelCredentialsWrite) error {
	if s.creds == nil {
		return ErrNoCredentialsKey
	}
	sealed, err := s.creds.Seal([]byte(in.Secret))
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO channel_credentials
			(account_id, secret_enc, encryption_key_version, token_kind, expires_at, refreshed_at, refresh_last_error)
		VALUES ($1, $2, $3, $4, $5, strftime('%Y-%m-%d %H:%M:%f','now'), '')
		ON CONFLICT (account_id) DO UPDATE SET
			secret_enc = EXCLUDED.secret_enc,
			encryption_key_version = EXCLUDED.encryption_key_version,
			token_kind = EXCLUDED.token_kind,
			expires_at = EXCLUDED.expires_at,
			refreshed_at = strftime('%Y-%m-%d %H:%M:%f','now'),
			refresh_last_error = '',
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`,
		in.AccountID, sealed, secretbox.KeyVersion, in.TokenKind, in.ExpiresAt)
	return err
}

// ChannelCredentialsSecret decrypts an account's token, resolved at the
// moment of the call so the plaintext never has to travel through a queue
// payload, a DTO or a log line — mirrors TelegramBotToken.
func (s *Store) ChannelCredentialsSecret(ctx context.Context, accountID uuid.UUID) (string, error) {
	if s.creds == nil {
		return "", ErrNoCredentialsKey
	}
	var sealed []byte
	err := s.db.QueryRow(ctx,
		`SELECT secret_enc FROM channel_credentials WHERE account_id = $1`, accountID).Scan(&sealed)
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

// ChannelCredentialsMeta is a credential row's non-secret bookkeeping — what
// the token refresher worker scans to decide who needs a refresh.
type ChannelCredentialsMeta struct {
	AccountID   uuid.UUID
	TokenKind   string
	ExpiresAt   *time.Time
	RefreshedAt *time.Time
}

// SetChannelCredentialsRefreshError records a failed refresh attempt without
// touching the stored secret — the token stays usable until it actually
// expires, but the error is visible for the setup checklist / health UI.
func (s *Store) SetChannelCredentialsRefreshError(ctx context.Context, accountID uuid.UUID, errMsg string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE channel_credentials SET refresh_last_error = $2, updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE account_id = $1`, accountID, errMsg)
	return err
}

// ChannelCredentialsExpiringSoon lists live accounts on `channel` whose
// credential expires before `before` and was last refreshed at least
// `minAge` ago (Meta requires a token be >=24h old before it can be
// refreshed again) — the token refresher worker's scan list. A NULL
// expires_at (a non-expiring business token) never matches.
func (s *Store) ChannelCredentialsExpiringSoon(ctx context.Context, channel string, before time.Time, minAge time.Duration) ([]ChannelCredentialsMeta, error) {
	refreshCutoff := time.Now().Add(-minAge).UTC()
	rows, err := s.db.Query(ctx, `
		SELECT c.account_id, c.token_kind, c.expires_at, c.refreshed_at
		FROM channel_credentials c
		JOIN channel_accounts a ON a.id = c.account_id
		WHERE a.channel = $1 AND a.deleted_at IS NULL
		  AND c.expires_at IS NOT NULL AND c.expires_at < $2
		  AND (c.refreshed_at IS NULL OR c.refreshed_at < $3)
		ORDER BY c.expires_at`, channel, before, refreshCutoff)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ChannelCredentialsMeta
	for rows.Next() {
		var m ChannelCredentialsMeta
		if err := rows.Scan(&m.AccountID, &m.TokenKind, &m.ExpiresAt, &m.RefreshedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// --- inbound ingestion --------------------------------------------------------

// ChannelInbound is one normalized inbound (or provider-echoed outbound)
// event, channel-neutral. Direction/SenderKind are caller-supplied (not
// hardcoded to "in" the way IngestTelegramInbound is) because Instagram and
// Messenger echo the operator's own sends back through the same webhook
// (is_echo=true -> direction="out", sender_kind="external_account") and this
// single function has to handle both shapes.
type ChannelInbound struct {
	AccountID          uuid.UUID
	ExternalContactID  string
	ContactHandle      string
	ContactDisplayName string
	ExternalThreadID   string
	Direction          string // "in" | "out"
	SenderKind         string // "contact" | "external_account"
	ExternalMessageID  string
	MessageKind        string
	Body               string
	Preview            string
	Source             string
	Raw                []byte
	MessageTS          time.Time
}

// ChannelInboundResult reports what the ingest did, so the caller knows
// which SSE to emit and whether this was a redelivery.
type ChannelInboundResult struct {
	ContactID       uuid.UUID
	ChatID          uuid.UUID
	MessageID       uuid.UUID
	ChatCreated     bool
	MessageInserted bool
}

// IngestChannelInbound upserts contact -> chat -> message and maintains the
// chat aggregates, all in ONE transaction that commits before the webhook
// acks — mirrors IngestTelegramInbound's durability shape. The one
// aggregate rule with no Telegram equivalent: last_inbound_at only advances
// on an inbound message (direction "in") — an outbound echo bumps
// last_message_at (so the chat list still sorts by recency) but must not
// reopen the 24-hour service window.
func (s *Store) IngestChannelInbound(ctx context.Context, in ChannelInbound) (ChannelInboundResult, error) {
	var res ChannelInboundResult
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.QueryRow(ctx, `
		INSERT INTO channel_contacts (account_id, external_contact_id, handle, display_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (account_id, external_contact_id) DO UPDATE SET
			handle = CASE WHEN EXCLUDED.handle <> '' THEN EXCLUDED.handle ELSE channel_contacts.handle END,
			display_name = CASE WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name
			                    ELSE channel_contacts.display_name END,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		RETURNING id`,
		in.AccountID, in.ExternalContactID, in.ContactHandle, in.ContactDisplayName).
		Scan(&res.ContactID); err != nil {
		return res, wrap("upsert channel contact", err)
	}

	// two-step insert-detection (SQLite has no xmax) — see
	// store/ingest.go's upsertChatTwoStep and tgingest's identical shape.
	err = tx.QueryRow(ctx, `
		INSERT INTO channel_chats (account_id, contact_id, external_thread_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (account_id, external_thread_id) DO NOTHING
		RETURNING id`,
		in.AccountID, res.ContactID, in.ExternalThreadID).Scan(&res.ChatID)
	switch {
	case err == nil:
		res.ChatCreated = true
	case errors.Is(err, dbx.ErrNoRows):
		if err := tx.QueryRow(ctx, `
			UPDATE channel_chats SET updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE account_id = $1 AND external_thread_id = $2
			RETURNING id`, in.AccountID, in.ExternalThreadID).Scan(&res.ChatID); err != nil {
			return res, wrap("update channel chat", err)
		}
	default:
		return res, wrap("upsert channel chat", err)
	}

	var extID any
	if in.ExternalMessageID != "" {
		extID = in.ExternalMessageID
	}
	var ts any
	if !in.MessageTS.IsZero() {
		ts = in.MessageTS
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO channel_messages
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
		// NULL never conflicts under SQLite's UNIQUE semantics, so a real
		// conflict only happens when extID was set — it is always the id to
		// reconcile against here.
		if err := tx.QueryRow(ctx, `
			SELECT id FROM channel_messages WHERE account_id = $1 AND external_message_id = $2`,
			in.AccountID, extID).Scan(&res.MessageID); err != nil {
			return res, wrap("resolve duplicate channel message", err)
		}
	default:
		return res, wrap("insert channel message", err)
	}

	if res.MessageInserted {
		isInbound := in.Direction == "in"
		if _, err := tx.Exec(ctx, `
			UPDATE channel_chats SET
				last_message_at = COALESCE($2, strftime('%Y-%m-%d %H:%M:%f','now')),
				last_inbound_at = CASE WHEN $3 THEN COALESCE($2, strftime('%Y-%m-%d %H:%M:%f','now')) ELSE last_inbound_at END,
				last_message_preview = $4,
				unread_count = CASE WHEN $3 THEN unread_count + 1 ELSE 0 END,
				updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
			WHERE id = $1`, res.ChatID, ts, isInbound, in.Preview); err != nil {
			return res, wrap("update channel aggregates", err)
		}
	}
	return res, tx.Commit(ctx)
}

// ChannelContactDisplayName returns a contact's already-known display name,
// or "" if none is on file yet — either because no channel_contacts row
// exists for (accountID, externalContactID) at all, or because one exists
// but hasn't captured a name yet (IngestChannelInbound's own upsert already
// preserves a non-empty name once set — see its ON CONFLICT clause). Both
// cases mean the same thing to every caller (there is nothing usable to
// show yet), so unlike most lookups here this deliberately does not surface
// ErrNotFound: callers use "" as their own "go look this up live" signal
// (see messengerishNormalize's knownDisplayName parameter).
func (s *Store) ChannelContactDisplayName(ctx context.Context, accountID uuid.UUID, externalContactID string) (string, error) {
	var name string
	err := s.db.QueryRow(ctx,
		`SELECT display_name FROM channel_contacts WHERE account_id = $1 AND external_contact_id = $2`,
		accountID, externalContactID).Scan(&name)
	if errors.Is(err, dbx.ErrNoRows) {
		return "", nil
	}
	return name, err
}

// --- media --------------------------------------------------------------------

// ChannelMediaMeta is a stored (or pending) attachment's descriptive fields.
type ChannelMediaMeta struct {
	MediaType   string
	Mimetype    string
	FileName    string
	ProviderRef string
	SourceURL   string
}

// ChannelMediaMetaFor returns a message's attachment metadata.
func (s *Store) ChannelMediaMetaFor(ctx context.Context, messageID uuid.UUID) (ChannelMediaMeta, error) {
	var m ChannelMediaMeta
	err := s.db.QueryRow(ctx, `
		SELECT media_type, mimetype, filename, provider_ref, source_url
		FROM channel_message_media WHERE message_id = $1`, messageID).
		Scan(&m.MediaType, &m.Mimetype, &m.FileName, &m.ProviderRef, &m.SourceURL)
	if errors.Is(err, dbx.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}

// ChannelOutboundMediaForSigning resolves what an outbound attachment send
// needs to mint a signed public media URL (see internal/inboxmedia and
// internal/whatsappcloud.ChannelSender): the channel_message_media row's
// own id (distinct from the blob-store key messaging.OutboundMedia.BlobID
// carries — UpsertOutboundMedia already created this row, keyed by
// message_id, before the send was ever enqueued) and that row's owning
// organization, resolved in ONE query so a sender never has to trust a
// caller-supplied org for something this security-sensitive.
func (s *Store) ChannelOutboundMediaForSigning(ctx context.Context, messageID uuid.UUID) (mediaID, orgID uuid.UUID, err error) {
	var org uuid.NullUUID
	err = s.db.QueryRow(ctx, `
		SELECT cmm.id, a.organization_id
		FROM channel_message_media cmm
		JOIN channel_messages m ON m.id = cmm.message_id
		JOIN channel_accounts a ON a.id = m.account_id
		WHERE cmm.message_id = $1`, messageID).Scan(&mediaID, &org)
	if errors.Is(err, dbx.ErrNoRows) {
		return uuid.UUID{}, uuid.UUID{}, ErrNotFound
	}
	if err != nil {
		return uuid.UUID{}, uuid.UUID{}, err
	}
	if !org.Valid {
		// An outbound send only ever happens through an org-scoped compose
		// path — an unassigned account reaching this point is a programming
		// error, not a data shape to silently paper over.
		return uuid.UUID{}, uuid.UUID{}, fmt.Errorf("store: channel account for message %s has no organization", messageID)
	}
	return mediaID, org.UUID, nil
}

// ChannelMediaByID resolves an outbound attachment's stored bytes location
// (blob storage_key) plus its owning organization, from the
// channel_message_media row's OWN id — the exact id inboxmedia.Signer signs
// (see ChannelOutboundMediaForSigning). This is the second of the two
// independent IDOR-defense layers GET /meta/api/v1/media/:media_id combines:
// the signature alone proves the token was minted for THIS (org, mediaID)
// pair, but the HTTP handler never trusts a caller-supplied org — it passes
// THIS query's resolved value into Verify, so a token honestly signed for a
// different organization than the one that actually owns mediaID simply
// fails to reproduce the signature.
func (s *Store) ChannelMediaByID(ctx context.Context, mediaID uuid.UUID) (storageKey, mimetype, fileName string, orgID uuid.UUID, err error) {
	var org uuid.NullUUID
	err = s.db.QueryRow(ctx, `
		SELECT cmm.storage_key, cmm.mimetype, cmm.filename, a.organization_id
		FROM channel_message_media cmm
		JOIN channel_messages m ON m.id = cmm.message_id
		JOIN channel_accounts a ON a.id = m.account_id
		WHERE cmm.id = $1`, mediaID).Scan(&storageKey, &mimetype, &fileName, &org)
	if errors.Is(err, dbx.ErrNoRows) {
		return "", "", "", uuid.UUID{}, ErrNotFound
	}
	if err != nil {
		return "", "", "", uuid.UUID{}, err
	}
	if !org.Valid {
		return "", "", "", uuid.UUID{}, ErrNotFound
	}
	return storageKey, mimetype, fileName, org.UUID, nil
}

// InsertChannelMediaPending records an inbound attachment's handle without
// its bytes — download_status='pending', retried from this row alone by the
// media sweeper, exactly like tg_message_media.
func (s *Store) InsertChannelMediaPending(ctx context.Context, messageID uuid.UUID, m ChannelMediaMeta) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO channel_message_media (message_id, media_type, mimetype, filename, provider_ref, source_url, download_status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		ON CONFLICT (message_id) DO NOTHING`,
		messageID, m.MediaType, m.Mimetype, m.FileName, m.ProviderRef, m.SourceURL)
	return err
}

// SetChannelMediaReady records a completed download.
func (s *Store) SetChannelMediaReady(ctx context.Context, messageID uuid.UUID, storageKey string, size int) error {
	_, err := s.db.Exec(ctx, `
		UPDATE channel_message_media
		SET storage_key = $2, size = $3, download_status = 'ready', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE message_id = $1`, messageID, storageKey, size)
	return err
}

// SetChannelMediaFailed marks a download attempt failed, leaving the row's
// provider handles intact so the sweeper can retry it; touching updated_at
// is what gives that retry a backoff instead of a hot loop.
func (s *Store) SetChannelMediaFailed(ctx context.Context, messageID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE channel_message_media
		SET download_status = 'failed', updated_at = strftime('%Y-%m-%d %H:%M:%f','now')
		WHERE message_id = $1 AND download_status <> 'ready'`, messageID)
	return err
}

// PendingChannelMedia is one attachment still awaiting its bytes.
type PendingChannelMedia struct {
	MessageID   uuid.UUID
	AccountID   uuid.UUID
	Channel     string
	ProviderRef string
	SourceURL   string
}

// PendingChannelMedia lists attachments whose bytes were never fetched and
// which have been quiet for at least olderThan — mirrors
// (*Store).PendingTelegramMedia, generalized across all three channels in
// one query since they share one media table.
func (s *Store) PendingChannelMedia(ctx context.Context, olderThan time.Duration, limit int) ([]PendingChannelMedia, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	cutoff := time.Now().Add(-olderThan).UTC()
	rows, err := s.db.Query(ctx, `
		SELECT mm.message_id, m.account_id, a.channel, mm.provider_ref, mm.source_url
		FROM channel_message_media mm
		JOIN channel_messages m ON m.id = mm.message_id
		JOIN channel_accounts a ON a.id = m.account_id
		WHERE mm.download_status <> 'ready'
		  AND (mm.provider_ref <> '' OR mm.source_url <> '')
		  AND a.deleted_at IS NULL
		  AND mm.updated_at < $1
		ORDER BY mm.updated_at
		LIMIT $2`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []PendingChannelMedia
	for rows.Next() {
		var p PendingChannelMedia
		if err := rows.Scan(&p.MessageID, &p.AccountID, &p.Channel, &p.ProviderRef, &p.SourceURL); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
