-- xchats SQLite schema, part 11: the generic channel core.
--
-- The official Meta channels — Instagram Direct, Facebook Messenger, and the
-- WhatsApp Cloud API — are the first channels that do NOT each get their own
-- wa_*/tg_*-shaped table family. All three speak close enough to the same
-- shape (Graph-API-style webhooks, an OAuth-issued token, a provider thread
-- id) that one generic core covers them, and every channel after them: a
-- future channel adds rows (new `channel` values, new `provider_meta` keys),
-- never new tables. See 0001_core.up.sql for the shared conventions this
-- file follows (uuidv4 default expression, timestamp defaults, json_valid
-- CHECKs) and 0002_channels.up.sql for the wa_*/tg_* precedent this
-- generalizes.
--
-- Deliberately NOT constrained to the 3 Meta channels: channel_accounts and
-- its children are the reusable core, so a future channel must not need a
-- CHECK-widening migration (a real cost — SQLite has no ALTER...ALTER CHECK,
-- only a full table rebuild) just to exist. meta_oauth_states and
-- meta_webhook_events, by contrast, encode Meta's own OAuth/webhook-signature
-- particulars and are genuinely Meta-only, so their channel CHECKs stay
-- closed to the 3 values this migration knows about.

CREATE TABLE channel_accounts (
    id                       TEXT PRIMARY KEY NOT NULL, -- derived: config.ChannelAccountID(ownerRef) — app-assigned, no DB default (see wa_accounts.id/tg_accounts.id)
    organization_id          TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    channel                  TEXT NOT NULL,
    external_account_id      TEXT NOT NULL, -- the provider's own id: IG user id, Page id, or WhatsApp phone_number_id
    display_name             TEXT NOT NULL DEFAULT '',
    handle                   TEXT NOT NULL DEFAULT '', -- @username / Page name / E.164 number — what an operator recognizes it by
    connection_state         TEXT NOT NULL DEFAULT 'connecting',
    webhook_url               TEXT NOT NULL DEFAULT '',
    webhook_registered_at    TEXT,
    webhook_last_checked_at  TEXT,
    webhook_last_error       TEXT NOT NULL DEFAULT '',
    last_live_event_at       TEXT,
    -- provider_meta carries everything provider-specific that isn't already a
    -- named column: waba_id, business_id, page_id, ig_user_id,
    -- quality_rating, registered, ... A new channel adds keys, not tables.
    provider_meta             TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(provider_meta)),
    deleted_at                TEXT,
    created_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (channel, external_account_id)
);
CREATE INDEX channel_accounts_org_idx ON channel_accounts(organization_id) WHERE deleted_at IS NULL;

CREATE TABLE channel_credentials (
    account_id              TEXT PRIMARY KEY NOT NULL REFERENCES channel_accounts(id) ON DELETE CASCADE,
    secret_enc               BLOB NOT NULL, -- secretbox.Seal() output — the same box that protects tg_credentials
    encryption_key_version   INTEGER NOT NULL DEFAULT 1,
    -- token_kind is a free-text label (e.g. "ig_long_lived", "page_token",
    -- "wa_business_token") — every provider's credential is the identical
    -- sealed-blob-plus-metadata shape, so one table serves all three rather
    -- than reintroducing per-channel credential tables.
    token_kind                TEXT NOT NULL DEFAULT '',
    expires_at                TEXT, -- NULL means non-expiring (a WhatsApp Cloud ES business token)
    refreshed_at              TEXT,
    refresh_last_error        TEXT NOT NULL DEFAULT '',
    created_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

CREATE TABLE channel_contacts (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id            TEXT NOT NULL REFERENCES channel_accounts(id) ON DELETE RESTRICT,
    external_contact_id   TEXT NOT NULL, -- the provider's own id for the person: IGSID / PSID / WhatsApp E.164
    handle                TEXT NOT NULL DEFAULT '',
    display_name          TEXT NOT NULL DEFAULT '',
    attributes            TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(attributes)),
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (account_id, external_contact_id)
);

CREATE TABLE channel_chats (
    id                       TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id               TEXT NOT NULL REFERENCES channel_accounts(id) ON DELETE RESTRICT,
    contact_id               TEXT NOT NULL REFERENCES channel_contacts(id) ON DELETE RESTRICT,
    external_thread_id       TEXT NOT NULL,
    chat_state                TEXT NOT NULL DEFAULT 'open',
    assignee_user_id          TEXT REFERENCES users(id) ON DELETE SET NULL,
    -- last_inbound_at is distinct from last_message_at: only an INBOUND
    -- message reopens the 24-hour customer-service window every Meta channel
    -- enforces — an outbound reply bumps last_message_at (so the chat list
    -- still sorts by recency) but must not itself reopen the window.
    last_inbound_at           TEXT,
    last_message_at           TEXT,
    last_message_preview      TEXT NOT NULL DEFAULT '',
    unread_count               INTEGER NOT NULL DEFAULT 0,
    created_at                 TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at                 TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (account_id, external_thread_id)
);
-- See 0002_channels.up.sql's NULL-ordering note: SQLite's DESC already sorts
-- NULLs last, so no qualifier is needed (unlike the Postgres original).
CREATE INDEX channel_chats_last_message_idx ON channel_chats(last_message_at DESC);

CREATE TABLE channel_messages (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id            TEXT NOT NULL REFERENCES channel_accounts(id) ON DELETE RESTRICT,
    chat_id               TEXT NOT NULL REFERENCES channel_chats(id) ON DELETE CASCADE,
    direction              TEXT NOT NULL,
    sender_kind             TEXT NOT NULL,
    sender_user_id          TEXT REFERENCES users(id) ON DELETE SET NULL,
    -- external_message_id, not "mid"/"wamid": this exact column name is what
    -- lets store/ingest.go's channel-dispatched InsertOutbound/
    -- StampOutboundSent/SetDeliveryStateFor work against this table with zero
    -- new dispatch code beyond the table-name switch.
    external_message_id    TEXT,
    message_kind            TEXT NOT NULL DEFAULT '',
    body                    TEXT NOT NULL DEFAULT '',
    delivery_state          TEXT NOT NULL DEFAULT 'queued',
    failure_reason          TEXT NOT NULL DEFAULT '',
    source                  TEXT NOT NULL DEFAULT 'live_webhook',
    raw                      TEXT CHECK (raw IS NULL OR json_valid(raw)),
    message_ts               TEXT,
    created_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (account_id, external_message_id)
);
CREATE INDEX channel_messages_chat_ts_idx ON channel_messages(chat_id, message_ts);

CREATE TABLE channel_message_media (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    message_id      TEXT NOT NULL UNIQUE REFERENCES channel_messages(id) ON DELETE CASCADE,
    -- provider_ref is the provider's own handle for content it already hosts
    -- (a WhatsApp Cloud media id) — the escape hatch that lets a re-send skip
    -- re-uploading, mirroring messaging.OutboundMedia.ProviderRef.
    provider_ref     TEXT NOT NULL DEFAULT '',
    -- source_url is the short-lived provider CDN URL an inbound attachment is
    -- fetched from (Instagram/Messenger hand back a direct URL; WhatsApp
    -- Cloud hands back a media id that resolves to one via a signed GET).
    source_url       TEXT NOT NULL DEFAULT '',
    media_type       TEXT NOT NULL,
    mimetype         TEXT NOT NULL DEFAULT '',
    filename         TEXT NOT NULL DEFAULT '',
    size              INTEGER NOT NULL DEFAULT 0,
    storage_key       TEXT NOT NULL DEFAULT '',
    download_status   TEXT NOT NULL DEFAULT 'pending',
    created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX channel_message_media_pending_idx ON channel_message_media(updated_at) WHERE download_status <> 'ready';

-- meta_oauth_states is the redirect-based OAuth handoff's server-side state:
-- the callback lands on the tunnel origin (possibly a different origin than
-- the SPA), so the session cookie is not reliably present there — the
-- single-use, short-TTL state row IS the authorization, mirroring
-- mcp_authorization_codes' natural-key-as-PK shape.
CREATE TABLE meta_oauth_states (
    state             TEXT PRIMARY KEY NOT NULL, -- 32 random bytes, base64url — the nonce itself, not a derived id
    channel           TEXT NOT NULL CHECK (channel IN ('instagram','messenger','whatsapp_cloud')),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri      TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','needs_selection','connected','failed','expired')),
    account_id        TEXT REFERENCES channel_accounts(id) ON DELETE SET NULL,
    last_error        TEXT NOT NULL DEFAULT '',
    expires_at        TEXT NOT NULL,
    settled_at        TEXT,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX meta_oauth_states_expires_idx ON meta_oauth_states(expires_at);

-- meta_webhook_events is the webhook dedup/audit log: every accepted or
-- rejected delivery is recorded once, keyed on the provider's own item id
-- where one exists. There is deliberately no separate inbound_events queue —
-- see 0002_channels.up.sql's tg_* header for why (the provider is its own
-- retry buffer); this table exists only for dedup + operator-visible health
-- (a bad_signature streak, an unknown_account flood), not as a work list.
CREATE TABLE meta_webhook_events (
    id            TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    channel       TEXT NOT NULL CHECK (channel IN ('instagram','messenger','whatsapp_cloud')),
    object_id     TEXT NOT NULL DEFAULT '', -- the payload's top-level entry.id (IG_ID / PAGE_ID / WABA_ID)
    account_id    TEXT REFERENCES channel_accounts(id) ON DELETE SET NULL,
    event_key     TEXT NOT NULL, -- the dedup key: the item's own mid/wamid, or a synthesized hash when none exists
    outcome       TEXT NOT NULL CHECK (outcome IN ('stored','duplicate','ignored','unknown_account','bad_signature','error')),
    detail        TEXT NOT NULL DEFAULT '',
    received_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (channel, event_key)
);
CREATE INDEX meta_webhook_events_account_idx ON meta_webhook_events(account_id);
CREATE INDEX meta_webhook_events_received_idx ON meta_webhook_events(received_at);

-- The three inbox_*_v views gain one channel_* UNION ALL arm each (two arms
-- -> three, not to five: all three Meta channels share this one arm). SQLite
-- refuses to DROP/ALTER a column a live view depends on, but nothing here
-- touches an existing wa_*/tg_* column — this is DROP+CREATE only because
-- adding a UNION arm means restating the whole view body, exactly as
-- 0007_whatsmeow.up.sql did when it recreated inbox_accounts_v/
-- inbox_messages_v. inbox_message_media_v also gains an arm; only
-- inbox_chats_v gains a new COLUMN (last_inbound_at, NULL on the wa_*/tg_*
-- arms, which have no service window).

DROP VIEW inbox_accounts_v;
DROP VIEW inbox_chats_v;
DROP VIEW inbox_messages_v;
DROP VIEW inbox_message_media_v;

CREATE VIEW inbox_accounts_v AS
SELECT a.id,
    a.organization_id,
    a.channel,
    a.display_name,
    a.phone_number AS external_handle,
    a.owner_jid AS external_account_ref,
    a.connection_state,
    a.last_live_event_at,
    a.deleted_at,
    a.created_at,
    NULL AS webhook_url,
    NULL AS webhook_registered_at,
    NULL AS webhook_last_checked_at,
    NULL AS webhook_last_error
FROM wa_accounts a
UNION ALL
SELECT t.id,
    t.organization_id,
    'telegram' AS channel,
    t.display_name,
    ('@' || t.bot_username) AS external_handle,
    ('telegram:bot:' || CAST(t.bot_id AS TEXT)) AS external_account_ref,
    t.connection_state,
    t.last_live_event_at,
    t.deleted_at,
    t.created_at,
    t.webhook_url,
    t.webhook_registered_at,
    t.webhook_last_checked_at,
    t.webhook_last_error
FROM tg_accounts t
UNION ALL
SELECT ca.id,
    ca.organization_id,
    ca.channel,
    ca.display_name,
    ca.handle AS external_handle,
    ca.external_account_id AS external_account_ref,
    ca.connection_state,
    ca.last_live_event_at,
    ca.deleted_at,
    ca.created_at,
    ca.webhook_url,
    ca.webhook_registered_at,
    ca.webhook_last_checked_at,
    ca.webhook_last_error
FROM channel_accounts ca;

CREATE VIEW inbox_chats_v AS
SELECT c.id,
    c.account_id,
    a.organization_id,
    a.channel,
    a.deleted_at AS account_deleted_at,
    c.remote_jid AS external_conversation_ref,
    c.chat_state,
    c.assignee_user_id,
    c.last_message_at,
    NULL AS last_inbound_at,
    c.last_message_preview,
    c.unread_count,
    c.created_at,
    c.updated_at,
    ct.id AS contact_id,
    ct.phone_number AS contact_phone_number,
    ct.phone_jid AS external_contact_ref,
    COALESCE(ct.lid_jid, '') AS contact_lid_jid,
    ct.push_name AS contact_push_name,
    ct.display_name AS contact_display_name,
    ct.attributes AS contact_attributes
FROM wa_chats c
    JOIN wa_contacts ct ON ct.id = c.contact_id
    JOIN wa_accounts a ON a.id = c.account_id
UNION ALL
SELECT c.id,
    c.account_id,
    t.organization_id,
    'telegram' AS channel,
    t.deleted_at AS account_deleted_at,
    CAST(c.telegram_chat_id AS TEXT) AS external_conversation_ref,
    c.chat_state,
    c.assignee_user_id,
    c.last_message_at,
    NULL AS last_inbound_at,
    c.last_message_preview,
    c.unread_count,
    c.created_at,
    c.updated_at,
    ct.id AS contact_id,
    '' AS contact_phone_number,
    CAST(ct.telegram_user_id AS TEXT) AS external_contact_ref,
    '' AS contact_lid_jid,
    COALESCE(NULLIF(TRIM(ct.first_name || ' ' || ct.last_name), ''), ct.username) AS contact_push_name,
    ct.display_name AS contact_display_name,
    '{}' AS contact_attributes
FROM tg_chats c
    JOIN tg_contacts ct ON ct.id = c.contact_id
    JOIN tg_accounts t ON t.id = c.account_id
UNION ALL
SELECT cc.id,
    cc.account_id,
    ca.organization_id,
    ca.channel,
    ca.deleted_at AS account_deleted_at,
    cc.external_thread_id AS external_conversation_ref,
    cc.chat_state,
    cc.assignee_user_id,
    cc.last_message_at,
    cc.last_inbound_at,
    cc.last_message_preview,
    cc.unread_count,
    cc.created_at,
    cc.updated_at,
    cct.id AS contact_id,
    '' AS contact_phone_number,
    cct.external_contact_id AS external_contact_ref,
    '' AS contact_lid_jid,
    cct.handle AS contact_push_name,
    cct.display_name AS contact_display_name,
    cct.attributes AS contact_attributes
FROM channel_chats cc
    JOIN channel_contacts cct ON cct.id = cc.contact_id
    JOIN channel_accounts ca ON ca.id = cc.account_id;

CREATE VIEW inbox_messages_v AS
SELECT m.id,
    m.chat_id,
    m.account_id,
    a.channel,
    m.direction,
    m.sender_kind,
    m.sender_user_id,
    COALESCE(m.external_message_id, '') AS external_message_id,
    m.message_kind,
    m.body,
    m.delivery_state,
    m.source,
    m.message_ts,
    m.created_at
FROM wa_messages m
    JOIN wa_accounts a ON a.id = m.account_id
UNION ALL
SELECT m.id,
    m.chat_id,
    m.account_id,
    'telegram' AS channel,
    m.direction,
    m.sender_kind,
    m.sender_user_id,
    COALESCE(CAST(m.telegram_message_id AS TEXT), '') AS external_message_id,
    m.message_kind,
    m.body,
    m.delivery_state,
    m.source,
    m.message_ts,
    m.created_at
FROM tg_messages m
UNION ALL
SELECT m.id,
    m.chat_id,
    m.account_id,
    ca.channel,
    m.direction,
    m.sender_kind,
    m.sender_user_id,
    COALESCE(m.external_message_id, '') AS external_message_id,
    m.message_kind,
    m.body,
    m.delivery_state,
    m.source,
    m.message_ts,
    m.created_at
FROM channel_messages m
    JOIN channel_accounts ca ON ca.id = m.account_id;

CREATE VIEW inbox_message_media_v AS
SELECT mm.id,
    mm.message_id,
    a.channel,
    mm.media_type,
    mm.mimetype,
    mm.file_name AS filename,
    mm.file_size AS size,
    mm.storage_url AS storage_key,
    mm.download_status,
    mm.created_at
FROM message_media mm
    JOIN wa_messages m ON m.id = mm.message_id
    JOIN wa_accounts a ON a.id = m.account_id
UNION ALL
SELECT tm.id,
    tm.message_id,
    'telegram' AS channel,
    tm.media_type,
    tm.mimetype,
    tm.filename,
    tm.size,
    tm.storage_key,
    tm.download_status,
    tm.created_at
FROM tg_message_media tm
UNION ALL
SELECT cmm.id,
    cmm.message_id,
    ca.channel,
    cmm.media_type,
    cmm.mimetype,
    cmm.filename,
    cmm.size,
    cmm.storage_key,
    cmm.download_status,
    cmm.created_at
FROM channel_message_media cmm
    JOIN channel_messages m ON m.id = cmm.message_id
    JOIN channel_accounts ca ON ca.id = m.account_id;
