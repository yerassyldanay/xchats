-- xchats SQLite schema, part 2: messaging channels (WhatsApp + Telegram)
-- and the four inbox_*_v views that unify both legs for the read layer.
-- See 0001_core.up.sql for the shared conventions this file follows.
--
-- NULL-ordering note (tg_chats_last_message_idx): Postgres's DESC sorts
-- NULL FIRST by default, so the original index spelled out "DESC NULLS
-- LAST" to override that. SQLite's default is the opposite — NULL sorts as
-- the smallest value, so DESC already yields NULLs last with no qualifier
-- (verified empirically; SQLite also rejects NULLS LAST inside an index's
-- column list outright, so dropping the qualifier is not optional here).

CREATE TABLE wa_accounts (
    id                      TEXT PRIMARY KEY NOT NULL, -- derived: uuidv5(owner_jid) — assigned by app code, no DB default
    organization_id         TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    display_name            TEXT NOT NULL DEFAULT '',
    owner_jid               TEXT NOT NULL UNIQUE,
    phone_number            TEXT NOT NULL DEFAULT '',
    connection_state        TEXT NOT NULL DEFAULT 'connected',
    last_live_event_at      TEXT,
    created_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    deleted_at               TEXT,
    channel                 TEXT NOT NULL DEFAULT 'whatsapp' CHECK (channel IN ('whatsapp','simulator'))
);
CREATE INDEX wa_accounts_org_idx ON wa_accounts(organization_id) WHERE deleted_at IS NULL;

-- wa_credentials maps a wa_accounts row to whatsmeow's own device session (kept
-- in a SEPARATE SQLite file — whatsmeow's sqlstore manages its own schema there).
-- This table only stores the mapping needed to reconnect a saved account after
-- restart; keys/identity/session material never live here or in xchats.db.
CREATE TABLE wa_credentials (
    account_id  TEXT PRIMARY KEY NOT NULL REFERENCES wa_accounts(id) ON DELETE CASCADE,
    device_jid  TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

CREATE TABLE wa_contacts (
    id           TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id   TEXT NOT NULL REFERENCES wa_accounts(id) ON DELETE RESTRICT,
    phone_number TEXT NOT NULL DEFAULT '',
    phone_jid    TEXT NOT NULL,
    lid_jid      TEXT,
    push_name    TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    avatar_url   TEXT,
    attributes   TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(attributes)),
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (account_id, phone_jid)
);
CREATE INDEX wa_contacts_lid_idx ON wa_contacts(account_id, lid_jid);

CREATE TABLE wa_chats (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id           TEXT NOT NULL REFERENCES wa_accounts(id) ON DELETE RESTRICT,
    contact_id           TEXT NOT NULL REFERENCES wa_contacts(id) ON DELETE RESTRICT,
    remote_jid           TEXT NOT NULL,
    chat_state           TEXT NOT NULL DEFAULT 'open',
    assignee_user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    stage                TEXT NOT NULL DEFAULT '',
    ai_summary           TEXT NOT NULL DEFAULT '',
    last_message_at      TEXT,
    last_message_preview TEXT NOT NULL DEFAULT '',
    unread_count         INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (account_id, remote_jid)
);

CREATE TABLE wa_messages (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id           TEXT NOT NULL REFERENCES wa_accounts(id) ON DELETE RESTRICT,
    chat_id              TEXT NOT NULL REFERENCES wa_chats(id) ON DELETE CASCADE,
    direction            TEXT NOT NULL,
    sender_kind          TEXT NOT NULL,
    sender_user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
    -- nullable: a queued outbound row has no provider id until the send
    -- returns. SQLite, like Postgres, treats NULL as distinct in a UNIQUE
    -- constraint, so many queued rows for the same account may coexist.
    external_message_id TEXT,
    participant_jid      TEXT NOT NULL DEFAULT '',
    message_kind         TEXT NOT NULL DEFAULT '',
    body                 TEXT NOT NULL DEFAULT '',
    delivery_state       TEXT NOT NULL DEFAULT 'queued',
    source               TEXT NOT NULL DEFAULT 'live_webhook',
    raw                  TEXT CHECK (raw IS NULL OR json_valid(raw)),
    message_ts           TEXT,
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (account_id, external_message_id)
);
CREATE INDEX wa_messages_chat_ts_idx ON wa_messages(chat_id, message_ts);

CREATE TABLE message_media (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    message_id      TEXT NOT NULL UNIQUE REFERENCES wa_messages(id) ON DELETE CASCADE,
    media_type      TEXT NOT NULL,
    mimetype        TEXT NOT NULL DEFAULT '',
    file_name       TEXT NOT NULL DEFAULT '',
    file_size       INTEGER NOT NULL DEFAULT 0,
    storage_url     TEXT NOT NULL DEFAULT '',
    download_status TEXT NOT NULL DEFAULT 'pending',
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

CREATE TABLE tg_accounts (
    id                       TEXT PRIMARY KEY NOT NULL, -- derived, app-assigned — see wa_accounts.id
    organization_id          TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    display_name             TEXT NOT NULL DEFAULT '',
    bot_id                   INTEGER NOT NULL UNIQUE,
    bot_username             TEXT NOT NULL DEFAULT '',
    connection_state         TEXT NOT NULL DEFAULT 'connecting',
    webhook_url               TEXT NOT NULL DEFAULT '',
    webhook_registered_at    TEXT,
    webhook_last_checked_at  TEXT,
    webhook_last_error       TEXT NOT NULL DEFAULT '',
    last_live_event_at       TEXT,
    deleted_at                TEXT,
    created_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX tg_accounts_org_idx ON tg_accounts(organization_id) WHERE deleted_at IS NULL;

CREATE TABLE tg_credentials (
    account_id              TEXT PRIMARY KEY NOT NULL REFERENCES tg_accounts(id) ON DELETE CASCADE,
    bot_token_enc           BLOB NOT NULL,
    encryption_key_version  INTEGER NOT NULL DEFAULT 1,
    created_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

CREATE TABLE tg_contacts (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id        TEXT NOT NULL REFERENCES tg_accounts(id) ON DELETE RESTRICT,
    telegram_user_id  INTEGER NOT NULL,
    username          TEXT NOT NULL DEFAULT '',
    first_name        TEXT NOT NULL DEFAULT '',
    last_name         TEXT NOT NULL DEFAULT '',
    display_name      TEXT NOT NULL DEFAULT '',
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (account_id, telegram_user_id)
);

CREATE TABLE tg_chats (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id            TEXT NOT NULL REFERENCES tg_accounts(id) ON DELETE RESTRICT,
    contact_id            TEXT NOT NULL REFERENCES tg_contacts(id) ON DELETE RESTRICT,
    telegram_chat_id      INTEGER NOT NULL,
    chat_type             TEXT NOT NULL DEFAULT 'private',
    chat_state            TEXT NOT NULL DEFAULT 'open',
    assignee_user_id      TEXT REFERENCES users(id) ON DELETE SET NULL,
    last_message_at       TEXT,
    last_message_preview  TEXT NOT NULL DEFAULT '',
    unread_count          INTEGER NOT NULL DEFAULT 0,
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (account_id, telegram_chat_id)
);
-- See the NULL-ordering note at the top of this file — no NULLS LAST here.
CREATE INDEX tg_chats_last_message_idx ON tg_chats(last_message_at DESC);

CREATE TABLE tg_messages (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id            TEXT NOT NULL REFERENCES tg_accounts(id) ON DELETE RESTRICT,
    chat_id               TEXT NOT NULL REFERENCES tg_chats(id) ON DELETE CASCADE,
    direction             TEXT NOT NULL,
    sender_kind           TEXT NOT NULL,
    sender_user_id        TEXT REFERENCES users(id) ON DELETE SET NULL,
    telegram_update_id    INTEGER,
    telegram_message_id   INTEGER,
    message_kind          TEXT NOT NULL DEFAULT '',
    body                  TEXT NOT NULL DEFAULT '',
    delivery_state        TEXT NOT NULL DEFAULT 'queued',
    source                TEXT NOT NULL DEFAULT 'live_webhook',
    raw                   TEXT CHECK (raw IS NULL OR json_valid(raw)),
    message_ts            TEXT,
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (account_id, telegram_update_id),
    UNIQUE (chat_id, telegram_message_id)
);
CREATE INDEX tg_messages_chat_ts_idx ON tg_messages(chat_id, message_ts);

CREATE TABLE tg_message_media (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    message_id      TEXT NOT NULL UNIQUE REFERENCES tg_messages(id) ON DELETE CASCADE,
    file_id         TEXT NOT NULL,
    file_unique_id  TEXT NOT NULL DEFAULT '',
    media_type      TEXT NOT NULL,
    mimetype        TEXT NOT NULL DEFAULT '',
    filename        TEXT NOT NULL DEFAULT '',
    size            INTEGER NOT NULL DEFAULT 0,
    storage_key     TEXT NOT NULL DEFAULT '',
    download_status TEXT NOT NULL DEFAULT 'pending',
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX tg_message_media_pending_idx ON tg_message_media(updated_at) WHERE download_status <> 'ready';

-- ---------------------------------------------------------------------------
-- Unified read views — one neutral shape across the WhatsApp and Telegram
-- legs. TRIM(BOTH FROM x) (Postgres's verbose SQL-standard spelling) becomes
-- SQLite's TRIM(x), which already trims spaces from both ends by default.
-- ---------------------------------------------------------------------------

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
FROM tg_accounts t;

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
    JOIN tg_accounts t ON t.id = c.account_id;

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
FROM tg_message_media tm;

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
FROM tg_messages m;
