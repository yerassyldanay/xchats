-- 0013_telegram_channel — Telegram as the second channel.
--
-- Shape follows plan/architecture.md "Channel adapters": provider payloads and
-- identifiers stay in channel-prefixed transport tables (tg_*), shared AI /
-- suggestion state stays channel-neutral, and the unified inbox reads across
-- providers through read-only UNION ALL views. There is deliberately NO generic
-- inbound_events table: Telegram's webhook retry (non-2xx => redeliver) is the
-- at-least-once mechanism, normalized rows are committed synchronously before
-- the 200 ack, and the unique constraints below make a redelivery a no-op.
SET search_path = xchats, public;

-- ---------------------------------------------------------------------------
-- ai_drafts becomes channel-neutral
-- ---------------------------------------------------------------------------
-- The three FKs pointed into wa_chats/wa_messages, which is exactly what stopped
-- a draft from belonging to a Telegram conversation. Every draft query is flat
-- (keyed by chat_id / id, zero joins — internal/store/read.go), so dropping them
-- costs no SQL rewrite; ids stay loose UUID references and orphan cleanup on a
-- hard delete becomes code-level.
ALTER TABLE xchats.ai_drafts
    DROP CONSTRAINT IF EXISTS ai_drafts_chat_id_fkey,
    DROP CONSTRAINT IF EXISTS ai_drafts_trigger_message_id_fkey,
    DROP CONSTRAINT IF EXISTS ai_drafts_sent_message_id_fkey,
    ADD COLUMN IF NOT EXISTS channel text NOT NULL DEFAULT 'whatsapp';

-- The single-pending-suggestion invariant the approve guard relies on, now
-- per (channel, chat). Chat ids are UUIDs and never collide across providers,
-- but carrying the channel keeps the index honest about what it scopes.
DROP INDEX IF EXISTS xchats.ai_drafts_pending_uq;
CREATE UNIQUE INDEX ai_drafts_pending_uq
    ON xchats.ai_drafts(channel, chat_id, option_ordinal)
    WHERE draft_state = 'suggested';

-- ---------------------------------------------------------------------------
-- Telegram transport (tg_*)
-- ---------------------------------------------------------------------------
-- One bot = one account. id is DERIVED: uuidv5(XCHATS_CHANNEL_NS,
-- 'telegram:bot:<bot_id>') — re-pasting the same token revives the soft-deleted
-- row (history intact) and keeps the webhook URL (which embeds this uuid) stable.
CREATE TABLE IF NOT EXISTS xchats.tg_accounts (
    id                      uuid PRIMARY KEY,
    organization_id         uuid REFERENCES xchats.organizations(id) ON DELETE SET NULL,
    display_name            text NOT NULL DEFAULT '',
    bot_id                  bigint NOT NULL UNIQUE,
    bot_username            text NOT NULL DEFAULT '',
    -- connecting | connected | webhook_error | token_error
    --            | disconnect_pending | disconnect_error
    connection_state        text NOT NULL DEFAULT 'connecting',
    webhook_url             text NOT NULL DEFAULT '',
    webhook_registered_at   timestamptz,
    webhook_last_checked_at timestamptz,
    webhook_last_error      text NOT NULL DEFAULT '',
    last_live_event_at      timestamptz,
    deleted_at              timestamptz,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);
-- Mirrors wa_accounts_org_idx (0002): listing an org's live accounts and the
-- multi-account inbox join both filter on organization_id + deleted_at.
CREATE INDEX IF NOT EXISTS tg_accounts_org_idx
    ON xchats.tg_accounts(organization_id)
    WHERE deleted_at IS NULL;

-- bot_token_enc is AES-256-GCM ciphertext (internal/secretbox, key from
-- TG_CREDENTIALS_ENC_KEY); encryption_key_version exists so a key rotation can
-- re-wrap rows without a schema change. The plaintext token never leaves this
-- table — not into queue payloads, DTOs, log URLs, or errors.
CREATE TABLE IF NOT EXISTS xchats.tg_credentials (
    account_id             uuid PRIMARY KEY REFERENCES xchats.tg_accounts(id) ON DELETE CASCADE,
    bot_token_enc          bytea NOT NULL,
    encryption_key_version int NOT NULL DEFAULT 1,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS xchats.tg_contacts (
    id               uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id       uuid NOT NULL REFERENCES xchats.tg_accounts(id) ON DELETE RESTRICT,
    telegram_user_id bigint NOT NULL,
    username         text NOT NULL DEFAULT '',
    first_name       text NOT NULL DEFAULT '',
    last_name        text NOT NULL DEFAULT '',
    display_name     text NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, telegram_user_id)
);

CREATE TABLE IF NOT EXISTS xchats.tg_chats (
    id                   uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id           uuid NOT NULL REFERENCES xchats.tg_accounts(id) ON DELETE RESTRICT,
    contact_id           uuid NOT NULL REFERENCES xchats.tg_contacts(id) ON DELETE RESTRICT,
    telegram_chat_id     bigint NOT NULL,
    chat_type            text NOT NULL DEFAULT 'private',
    chat_state           text NOT NULL DEFAULT 'open',
    assignee_user_id     uuid REFERENCES xchats.users(id) ON DELETE SET NULL,
    last_message_at      timestamptz,
    last_message_preview text NOT NULL DEFAULT '',
    unread_count         int NOT NULL DEFAULT 0,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, telegram_chat_id)
);
-- The inbox orders by last_message_at DESC across both legs of inbox_chats_v.
CREATE INDEX IF NOT EXISTS tg_chats_last_message_idx
    ON xchats.tg_chats(last_message_at DESC NULLS LAST);

CREATE TABLE IF NOT EXISTS xchats.tg_messages (
    id                  uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id          uuid NOT NULL REFERENCES xchats.tg_accounts(id) ON DELETE RESTRICT,
    chat_id             uuid NOT NULL REFERENCES xchats.tg_chats(id) ON DELETE CASCADE,
    direction           text NOT NULL,
    sender_kind         text NOT NULL,
    sender_user_id      uuid REFERENCES xchats.users(id) ON DELETE SET NULL,
    -- inbound only; NULL on outbound. The dedup key for webhook redelivery.
    telegram_update_id  bigint,
    -- NULL until the sendMessage response is stamped onto an outbound row.
    telegram_message_id bigint,
    message_kind        text NOT NULL DEFAULT '',
    body                text NOT NULL DEFAULT '',
    delivery_state      text NOT NULL DEFAULT 'queued',
    source              text NOT NULL DEFAULT 'live_webhook',
    raw                 jsonb,
    message_ts          timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    -- Both nullable so many queued outbound rows coexist (NULLs are distinct).
    UNIQUE (account_id, telegram_update_id),
    UNIQUE (chat_id, telegram_message_id)
);
CREATE INDEX IF NOT EXISTS tg_messages_chat_ts_idx ON xchats.tg_messages(chat_id, message_ts);

-- Media rows are written at ingest time from day one (file_id/file_unique_id
-- preserved, download_status='pending'); only the file download itself is a
-- later phase, and its retry lives on this row — there is no event to replay.
CREATE TABLE IF NOT EXISTS xchats.tg_message_media (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id      uuid NOT NULL REFERENCES xchats.tg_messages(id) ON DELETE CASCADE,
    file_id         text NOT NULL,
    file_unique_id  text NOT NULL DEFAULT '',
    media_type      text NOT NULL,
    mimetype        text NOT NULL DEFAULT '',
    filename        text NOT NULL DEFAULT '',
    size            int  NOT NULL DEFAULT 0,
    storage_key     text NOT NULL DEFAULT '',
    download_status text NOT NULL DEFAULT 'pending',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (message_id)
);
CREATE INDEX IF NOT EXISTS tg_message_media_pending_idx
    ON xchats.tg_message_media(updated_at)
    WHERE download_status <> 'ready';

-- ---------------------------------------------------------------------------
-- The four unified read views (read-only; every write still goes to its own
-- provider table through a channel-dispatched store method).
-- ---------------------------------------------------------------------------
-- Neutral column names (external_handle / external_account_ref /
-- external_conversation_ref / external_contact_ref / external_message_id) keep
-- provider vocabulary out of the shared read path. The WhatsApp legs are
-- deliberately UNFILTERED on channel: the simulator lives in wa_* rows with
-- channel='simulator', so filtering them here would make simulator
-- conversations vanish from the inbox.

CREATE VIEW xchats.inbox_accounts_v AS
  SELECT a.id, a.organization_id, a.channel, a.display_name,
         a.phone_number            AS external_handle,
         a.owner_jid               AS external_account_ref,
         a.evolution_instance_name AS instance_name,
         a.connection_state, a.last_live_event_at, a.deleted_at, a.created_at,
         NULL::text                AS webhook_url,
         NULL::timestamptz         AS webhook_registered_at,
         NULL::timestamptz         AS webhook_last_checked_at,
         NULL::text                AS webhook_last_error
    FROM xchats.wa_accounts a
  UNION ALL
  SELECT t.id, t.organization_id, 'telegram'::text, t.display_name,
         ('@' || t.bot_username)::text,
         ('telegram:bot:' || t.bot_id::text)::text,
         ''::text,
         t.connection_state, t.last_live_event_at, t.deleted_at, t.created_at,
         t.webhook_url, t.webhook_registered_at, t.webhook_last_checked_at,
         t.webhook_last_error
    FROM xchats.tg_accounts t;

CREATE VIEW xchats.inbox_chats_v AS
  SELECT c.id, c.account_id, a.organization_id, a.channel,
         a.deleted_at              AS account_deleted_at,
         c.remote_jid              AS external_conversation_ref,
         c.chat_state, c.assignee_user_id,
         c.last_message_at, c.last_message_preview, c.unread_count,
         c.created_at, c.updated_at,
         ct.id                     AS contact_id,
         ct.phone_number           AS contact_phone_number,
         ct.phone_jid              AS external_contact_ref,
         COALESCE(ct.lid_jid, '')  AS contact_lid_jid,
         ct.push_name              AS contact_push_name,
         ct.display_name           AS contact_display_name,
         ct.attributes             AS contact_attributes
    FROM xchats.wa_chats c
    JOIN xchats.wa_contacts ct ON ct.id = c.contact_id
    JOIN xchats.wa_accounts a  ON a.id = c.account_id
  UNION ALL
  SELECT c.id, c.account_id, t.organization_id, 'telegram'::text,
         t.deleted_at,
         c.telegram_chat_id::text,
         c.chat_state, c.assignee_user_id,
         c.last_message_at, c.last_message_preview, c.unread_count,
         c.created_at, c.updated_at,
         ct.id,
         ''::text,
         ct.telegram_user_id::text,
         ''::text,
         COALESCE(NULLIF(TRIM(ct.first_name || ' ' || ct.last_name), ''), ct.username),
         ct.display_name,
         '{}'::jsonb
    FROM xchats.tg_chats c
    JOIN xchats.tg_contacts ct ON ct.id = c.contact_id
    JOIN xchats.tg_accounts t  ON t.id = c.account_id;

CREATE VIEW xchats.inbox_messages_v AS
  SELECT m.id, m.chat_id, m.account_id, a.channel,
         m.direction, m.sender_kind, m.sender_user_id,
         COALESCE(m.evolution_message_id, '') AS external_message_id,
         m.message_kind, m.body, m.delivery_state, m.source,
         m.message_ts, m.created_at
    FROM xchats.wa_messages m
    JOIN xchats.wa_accounts a ON a.id = m.account_id
  UNION ALL
  SELECT m.id, m.chat_id, m.account_id, 'telegram'::text,
         m.direction, m.sender_kind, m.sender_user_id,
         COALESCE(m.telegram_message_id::text, ''),
         m.message_kind, m.body, m.delivery_state, m.source,
         m.message_ts, m.created_at
    FROM xchats.tg_messages m;

-- The WhatsApp leg's channel comes from wa_accounts, never a hardcoded
-- 'whatsapp': simulator media lives in these same rows and must not be
-- mislabeled.
CREATE VIEW xchats.inbox_message_media_v AS
  SELECT mm.id, mm.message_id, a.channel, mm.media_type, mm.mimetype,
         mm.file_name               AS filename,
         mm.file_size               AS size,
         mm.storage_url             AS storage_key,
         mm.download_status, mm.created_at
    FROM xchats.message_media mm
    JOIN xchats.wa_messages m ON m.id = mm.message_id
    JOIN xchats.wa_accounts a ON a.id = m.account_id
  UNION ALL
  SELECT tm.id, tm.message_id, 'telegram'::text, tm.media_type, tm.mimetype,
         tm.filename, tm.size, tm.storage_key, tm.download_status, tm.created_at
    FROM xchats.tg_message_media tm;
