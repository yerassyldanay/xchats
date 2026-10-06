-- 2/5 Channels & inbox: WhatsApp (wa_*), Telegram (tg_*) and the generic Meta-style
-- (channel_*) accounts, contacts, chats, messages and media, Meta OAuth/webhook
-- bookkeeping, and the four inbox_*_v views that unify the three transports.
-- Depends on 20261006000001_identity_access.
-- Shared by SQLite and PostgreSQL: the migration runner expands the dialect macros
-- for the open engine (docs/database.md). Idempotent; the runner owns the transaction.

CREATE TABLE IF NOT EXISTS wa_accounts (
    id                      TEXT PRIMARY KEY NOT NULL,
    organization_id         TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    display_name            TEXT NOT NULL DEFAULT '',
    owner_jid               TEXT NOT NULL UNIQUE,
    phone_number            TEXT NOT NULL DEFAULT '',
    connection_state        TEXT NOT NULL DEFAULT 'connected',
    last_live_event_at      {{timestamp}},
    created_at              {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at              {{timestamp}} NOT NULL DEFAULT {{now}},
    deleted_at               {{timestamp}},
    channel                 TEXT NOT NULL DEFAULT 'whatsapp' CHECK (channel IN ('whatsapp','simulator'))
);

CREATE INDEX IF NOT EXISTS wa_accounts_org_idx ON wa_accounts(organization_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS tg_accounts (
    id                       TEXT PRIMARY KEY NOT NULL,
    organization_id          TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    display_name             TEXT NOT NULL DEFAULT '',
    bot_id                   BIGINT NOT NULL UNIQUE,
    bot_username             TEXT NOT NULL DEFAULT '',
    connection_state         TEXT NOT NULL DEFAULT 'connecting',
    webhook_url               TEXT NOT NULL DEFAULT '',
    webhook_registered_at    {{timestamp}},
    webhook_last_checked_at  {{timestamp}},
    webhook_last_error       TEXT NOT NULL DEFAULT '',
    last_live_event_at       {{timestamp}},
    deleted_at                {{timestamp}},
    created_at                {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at                {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE INDEX IF NOT EXISTS tg_accounts_org_idx ON tg_accounts(organization_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS channel_accounts (
    id                       TEXT PRIMARY KEY NOT NULL,
    organization_id          TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    channel                  TEXT NOT NULL,
    external_account_id      TEXT NOT NULL,
    display_name             TEXT NOT NULL DEFAULT '',
    handle                   TEXT NOT NULL DEFAULT '',
    connection_state         TEXT NOT NULL DEFAULT 'connecting',
    webhook_url               TEXT NOT NULL DEFAULT '',
    webhook_registered_at    {{timestamp}},
    webhook_last_checked_at  {{timestamp}},
    webhook_last_error       TEXT NOT NULL DEFAULT '',
    last_live_event_at       {{timestamp}},
    provider_meta             {{json "provider_meta"}} NOT NULL DEFAULT '{}',
    deleted_at                {{timestamp}},
    created_at                {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at                {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (channel, external_account_id)
);

CREATE INDEX IF NOT EXISTS channel_accounts_org_idx ON channel_accounts(organization_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS wa_contacts (
    id           TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id   TEXT NOT NULL REFERENCES wa_accounts(id) ON DELETE RESTRICT,
    phone_number TEXT NOT NULL DEFAULT '',
    phone_jid    TEXT NOT NULL,
    lid_jid      TEXT,
    push_name    TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    avatar_url   TEXT,
    attributes   {{json "attributes"}} NOT NULL DEFAULT '{}',
    created_at   {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at   {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (account_id, phone_jid)
);

CREATE INDEX IF NOT EXISTS wa_contacts_lid_idx ON wa_contacts(account_id, lid_jid);

CREATE TABLE IF NOT EXISTS tg_credentials (
    account_id              TEXT PRIMARY KEY NOT NULL REFERENCES tg_accounts(id) ON DELETE CASCADE,
    bot_token_enc           {{bytes}} NOT NULL,
    encryption_key_version  INTEGER NOT NULL DEFAULT 1,
    created_at              {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at              {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE TABLE IF NOT EXISTS tg_contacts (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id        TEXT NOT NULL REFERENCES tg_accounts(id) ON DELETE RESTRICT,
    telegram_user_id  BIGINT NOT NULL,
    username          TEXT NOT NULL DEFAULT '',
    first_name        TEXT NOT NULL DEFAULT '',
    last_name         TEXT NOT NULL DEFAULT '',
    display_name      TEXT NOT NULL DEFAULT '',
    created_at        {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at        {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (account_id, telegram_user_id)
);

CREATE TABLE IF NOT EXISTS tg_poll_state (
    account_id      TEXT PRIMARY KEY NOT NULL REFERENCES tg_accounts(id) ON DELETE CASCADE,
    last_update_id  BIGINT NOT NULL,
    updated_at      {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE TABLE IF NOT EXISTS wa_credentials (
    account_id  TEXT PRIMARY KEY NOT NULL REFERENCES wa_accounts(id) ON DELETE CASCADE,
    device_jid  TEXT NOT NULL,
    created_at  {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at  {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE TABLE IF NOT EXISTS channel_credentials (
    account_id              TEXT PRIMARY KEY NOT NULL REFERENCES channel_accounts(id) ON DELETE CASCADE,
    secret_enc               {{bytes}} NOT NULL,
    encryption_key_version   INTEGER NOT NULL DEFAULT 1,
    token_kind                TEXT NOT NULL DEFAULT '',
    expires_at                {{timestamp}},
    refreshed_at              {{timestamp}},
    refresh_last_error        TEXT NOT NULL DEFAULT '',
    created_at                {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at                {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE TABLE IF NOT EXISTS channel_contacts (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id            TEXT NOT NULL REFERENCES channel_accounts(id) ON DELETE RESTRICT,
    external_contact_id   TEXT NOT NULL,
    handle                TEXT NOT NULL DEFAULT '',
    display_name          TEXT NOT NULL DEFAULT '',
    attributes            {{json "attributes"}} NOT NULL DEFAULT '{}',
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (account_id, external_contact_id)
);

CREATE TABLE IF NOT EXISTS meta_oauth_states (
    state             TEXT PRIMARY KEY NOT NULL,
    channel           TEXT NOT NULL CHECK (channel IN ('instagram','messenger','whatsapp_cloud')),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri      TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','needs_selection','connected','failed','expired')),
    account_id        TEXT REFERENCES channel_accounts(id) ON DELETE SET NULL,
    last_error        TEXT NOT NULL DEFAULT '',
    expires_at        {{timestamp}} NOT NULL,
    settled_at        {{timestamp}},
    created_at        {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE INDEX IF NOT EXISTS meta_oauth_states_expires_idx ON meta_oauth_states(expires_at);

CREATE TABLE IF NOT EXISTS meta_webhook_events (
    id            TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    channel       TEXT NOT NULL CHECK (channel IN ('instagram','messenger','whatsapp_cloud')),
    object_id     TEXT NOT NULL DEFAULT '',
    account_id    TEXT REFERENCES channel_accounts(id) ON DELETE SET NULL,
    event_key     TEXT NOT NULL,
    outcome       TEXT NOT NULL CHECK (outcome IN ('stored','duplicate','ignored','unknown_account','bad_signature','error')),
    detail        TEXT NOT NULL DEFAULT '',
    received_at   {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (channel, event_key)
);

CREATE INDEX IF NOT EXISTS meta_webhook_events_account_idx ON meta_webhook_events(account_id);

CREATE INDEX IF NOT EXISTS meta_webhook_events_received_idx ON meta_webhook_events(received_at);

CREATE TABLE IF NOT EXISTS wa_chats (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id           TEXT NOT NULL REFERENCES wa_accounts(id) ON DELETE RESTRICT,
    contact_id           TEXT NOT NULL REFERENCES wa_contacts(id) ON DELETE RESTRICT,
    remote_jid           TEXT NOT NULL,
    chat_state           TEXT NOT NULL DEFAULT 'open',
    assignee_user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    stage                TEXT NOT NULL DEFAULT '',
    ai_summary           TEXT NOT NULL DEFAULT '',
    last_message_at      {{timestamp}},
    last_message_preview TEXT NOT NULL DEFAULT '',
    unread_count         INTEGER NOT NULL DEFAULT 0,
    created_at           {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at           {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (account_id, remote_jid)
);

CREATE TABLE IF NOT EXISTS tg_chats (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id            TEXT NOT NULL REFERENCES tg_accounts(id) ON DELETE RESTRICT,
    contact_id            TEXT NOT NULL REFERENCES tg_contacts(id) ON DELETE RESTRICT,
    telegram_chat_id      BIGINT NOT NULL,
    chat_type             TEXT NOT NULL DEFAULT 'private',
    chat_state            TEXT NOT NULL DEFAULT 'open',
    assignee_user_id      TEXT REFERENCES users(id) ON DELETE SET NULL,
    last_message_at       {{timestamp}},
    last_message_preview  TEXT NOT NULL DEFAULT '',
    unread_count          INTEGER NOT NULL DEFAULT 0,
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (account_id, telegram_chat_id)
);

CREATE INDEX IF NOT EXISTS tg_chats_last_message_idx ON tg_chats(last_message_at DESC);

CREATE TABLE IF NOT EXISTS channel_chats (
    id                       TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id               TEXT NOT NULL REFERENCES channel_accounts(id) ON DELETE RESTRICT,
    contact_id               TEXT NOT NULL REFERENCES channel_contacts(id) ON DELETE RESTRICT,
    external_thread_id       TEXT NOT NULL,
    chat_state                TEXT NOT NULL DEFAULT 'open',
    assignee_user_id          TEXT REFERENCES users(id) ON DELETE SET NULL,
    last_inbound_at           {{timestamp}},
    last_message_at           {{timestamp}},
    last_message_preview      TEXT NOT NULL DEFAULT '',
    unread_count               INTEGER NOT NULL DEFAULT 0,
    created_at                 {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at                 {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (account_id, external_thread_id)
);

CREATE INDEX IF NOT EXISTS channel_chats_last_message_idx ON channel_chats(last_message_at DESC);

CREATE TABLE IF NOT EXISTS wa_messages (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id           TEXT NOT NULL REFERENCES wa_accounts(id) ON DELETE RESTRICT,
    chat_id              TEXT NOT NULL REFERENCES wa_chats(id) ON DELETE CASCADE,
    direction            TEXT NOT NULL,
    sender_kind          TEXT NOT NULL,
    sender_user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
    external_message_id TEXT,
    participant_jid      TEXT NOT NULL DEFAULT '',
    message_kind         TEXT NOT NULL DEFAULT '',
    body                 TEXT NOT NULL DEFAULT '',
    delivery_state       TEXT NOT NULL DEFAULT 'queued',
    source               TEXT NOT NULL DEFAULT 'live_webhook',
    raw                  {{json "raw"}},
    message_ts           {{timestamp}},
    created_at           {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at           {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (account_id, external_message_id)
);

CREATE INDEX IF NOT EXISTS wa_messages_chat_ts_idx ON wa_messages(chat_id, message_ts);

CREATE TABLE IF NOT EXISTS tg_messages (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id            TEXT NOT NULL REFERENCES tg_accounts(id) ON DELETE RESTRICT,
    chat_id               TEXT NOT NULL REFERENCES tg_chats(id) ON DELETE CASCADE,
    direction             TEXT NOT NULL,
    sender_kind           TEXT NOT NULL,
    sender_user_id        TEXT REFERENCES users(id) ON DELETE SET NULL,
    telegram_update_id    BIGINT,
    telegram_message_id   BIGINT,
    message_kind          TEXT NOT NULL DEFAULT '',
    body                  TEXT NOT NULL DEFAULT '',
    delivery_state        TEXT NOT NULL DEFAULT 'queued',
    source                TEXT NOT NULL DEFAULT 'live_webhook',
    raw                   {{json "raw"}},
    message_ts            {{timestamp}},
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (account_id, telegram_update_id),
    UNIQUE (chat_id, telegram_message_id)
);

CREATE INDEX IF NOT EXISTS tg_messages_chat_ts_idx ON tg_messages(chat_id, message_ts);

CREATE TABLE IF NOT EXISTS channel_messages (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id            TEXT NOT NULL REFERENCES channel_accounts(id) ON DELETE RESTRICT,
    chat_id               TEXT NOT NULL REFERENCES channel_chats(id) ON DELETE CASCADE,
    direction              TEXT NOT NULL,
    sender_kind             TEXT NOT NULL,
    sender_user_id          TEXT REFERENCES users(id) ON DELETE SET NULL,
    external_message_id    TEXT,
    message_kind            TEXT NOT NULL DEFAULT '',
    body                    TEXT NOT NULL DEFAULT '',
    delivery_state          TEXT NOT NULL DEFAULT 'queued',
    failure_reason          TEXT NOT NULL DEFAULT '',
    source                  TEXT NOT NULL DEFAULT 'live_webhook',
    raw                      {{json "raw"}},
    message_ts               {{timestamp}},
    created_at                {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at                {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (account_id, external_message_id)
);

CREATE INDEX IF NOT EXISTS channel_messages_chat_ts_idx ON channel_messages(chat_id, message_ts);

CREATE TABLE IF NOT EXISTS message_media (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    message_id      TEXT NOT NULL UNIQUE REFERENCES wa_messages(id) ON DELETE CASCADE,
    media_type      TEXT NOT NULL,
    mimetype        TEXT NOT NULL DEFAULT '',
    file_name       TEXT NOT NULL DEFAULT '',
    file_size       INTEGER NOT NULL DEFAULT 0,
    storage_url     TEXT NOT NULL DEFAULT '',
    download_status TEXT NOT NULL DEFAULT 'pending',
    created_at      {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at      {{timestamp}} NOT NULL DEFAULT {{now}},
    transcript TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS tg_message_media (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    message_id      TEXT NOT NULL UNIQUE REFERENCES tg_messages(id) ON DELETE CASCADE,
    file_id         TEXT NOT NULL,
    file_unique_id  TEXT NOT NULL DEFAULT '',
    media_type      TEXT NOT NULL,
    mimetype        TEXT NOT NULL DEFAULT '',
    filename        TEXT NOT NULL DEFAULT '',
    size            INTEGER NOT NULL DEFAULT 0,
    storage_key     TEXT NOT NULL DEFAULT '',
    download_status TEXT NOT NULL DEFAULT 'pending',
    created_at      {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at      {{timestamp}} NOT NULL DEFAULT {{now}},
    transcript TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS tg_message_media_pending_idx ON tg_message_media(updated_at) WHERE download_status <> 'ready';

CREATE TABLE IF NOT EXISTS channel_message_media (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    message_id      TEXT NOT NULL UNIQUE REFERENCES channel_messages(id) ON DELETE CASCADE,
    provider_ref     TEXT NOT NULL DEFAULT '',
    source_url       TEXT NOT NULL DEFAULT '',
    media_type       TEXT NOT NULL,
    mimetype         TEXT NOT NULL DEFAULT '',
    filename         TEXT NOT NULL DEFAULT '',
    size              INTEGER NOT NULL DEFAULT 0,
    storage_key       TEXT NOT NULL DEFAULT '',
    download_status   TEXT NOT NULL DEFAULT 'pending',
    created_at         {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at         {{timestamp}} NOT NULL DEFAULT {{now}},
    transcript TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS channel_message_media_pending_idx ON channel_message_media(updated_at) WHERE download_status <> 'ready';

-- Views are dropped and recreated so a replay is idempotent on both engines (SQLite has no
-- CREATE OR REPLACE VIEW); nothing depends on them.

DROP VIEW IF EXISTS inbox_accounts_v;

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
    CAST(NULL AS {{timestamp}}) AS webhook_registered_at,
    CAST(NULL AS {{timestamp}}) AS webhook_last_checked_at,
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

DROP VIEW IF EXISTS inbox_chats_v;

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
    CAST(NULL AS {{timestamp}}) AS last_inbound_at,
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
    CAST(NULL AS {{timestamp}}) AS last_inbound_at,
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

DROP VIEW IF EXISTS inbox_messages_v;

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

DROP VIEW IF EXISTS inbox_message_media_v;

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
    mm.transcript,
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
    tm.transcript,
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
    cmm.transcript,
    cmm.created_at
FROM channel_message_media cmm
    JOIN channel_messages m ON m.id = cmm.message_id
    JOIN channel_accounts ca ON ca.id = m.account_id;
