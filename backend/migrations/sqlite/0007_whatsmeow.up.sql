-- Replace the Evolution REST gateway with a direct whatsmeow connection.
--
-- This is a FORWARD migration rather than an edit to 0002_channels.up.sql,
-- because dbx.RunMigrations records applied versions and never re-runs one:
-- editing 0002 in place would silently leave every already-migrated database
-- (dev, staging, prod) without the changes below, while fresh databases got
-- them — two divergent schemas from one source tree.
--
-- The wa_* transport tables keep their names and data; only the columns tied
-- to Evolution's "instance" concept go away, and the provider message id
-- becomes provider-neutral. No message, chat, or contact row is touched.

-- wa_credentials maps a wa_accounts row to whatsmeow's own device session
-- (kept in a SEPARATE SQLite file — whatsmeow's sqlstore manages its own
-- schema there). This table only stores the mapping needed to reconnect a
-- saved account after restart; keys/identity/session material never live
-- here or in xchats.db.
CREATE TABLE wa_credentials (
    account_id  TEXT PRIMARY KEY NOT NULL REFERENCES wa_accounts(id) ON DELETE CASCADE,
    device_jid  TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

-- The two views below read the columns this migration renames and drops.
-- SQLite refuses to DROP a column a view still references, and its RENAME
-- COLUMN would rewrite the view bodies to point at names the new schema no
-- longer defines, so both are dropped up front and recreated at the end
-- against the final column names.
DROP VIEW inbox_messages_v;
DROP VIEW inbox_accounts_v;

-- evolution_message_id -> external_message_id. RENAME COLUMN carries the
-- UNIQUE (account_id, evolution_message_id) constraint over to the new name
-- automatically, so the echo-collapse guarantee survives the rename.
ALTER TABLE wa_messages RENAME COLUMN evolution_message_id TO external_message_id;

-- whatsmeow pairs a device directly; there is no instance to name or track.
ALTER TABLE wa_accounts DROP COLUMN evolution_instance_name;
ALTER TABLE wa_accounts DROP COLUMN evolution_instance_id;

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
