-- xchats Build 0 schema (subset of plan/9-database-schema.md).
-- Everything lives in the dedicated `xchats` schema; Evolution keeps its own.
CREATE SCHEMA IF NOT EXISTS xchats;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

SET search_path = xchats, public;

-- ---------------------------------------------------------------------------
-- Identity & organization
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS xchats.organizations (
    id                   uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    name                 text NOT NULL,
    respond_mode         text NOT NULL DEFAULT 'NEVER',
    respond_window_start time,
    respond_window_end   time,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS xchats.users (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         citext NOT NULL UNIQUE,
    password_hash text NOT NULL,
    display_name  text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS xchats.organization_users (
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    user_id         uuid NOT NULL REFERENCES xchats.users(id) ON DELETE CASCADE,
    joined_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE IF NOT EXISTS xchats.sessions (
    id         text PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES xchats.users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON xchats.sessions(user_id);

-- ---------------------------------------------------------------------------
-- WhatsApp transport
-- ---------------------------------------------------------------------------
-- id is DERIVED: uuidv5(XCHATS_WA_NS, owner_jid) — not random.
CREATE TABLE IF NOT EXISTS xchats.wa_accounts (
    id                      uuid PRIMARY KEY,
    organization_id         uuid REFERENCES xchats.organizations(id) ON DELETE SET NULL,
    display_name            text NOT NULL DEFAULT '',
    owner_jid               text NOT NULL UNIQUE,
    phone_number            text NOT NULL DEFAULT '',
    evolution_instance_name text NOT NULL DEFAULT '',
    evolution_instance_id   text NOT NULL DEFAULT '',
    connection_state        text NOT NULL DEFAULT 'connected',
    last_live_event_at      timestamptz,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS xchats.wa_contacts (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id   uuid NOT NULL REFERENCES xchats.wa_accounts(id) ON DELETE RESTRICT,
    phone_number text NOT NULL DEFAULT '',
    phone_jid    text NOT NULL,
    lid_jid      text,
    push_name    text NOT NULL DEFAULT '',
    display_name text NOT NULL DEFAULT '',
    avatar_url   text,
    attributes   jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, phone_jid)
);
CREATE INDEX IF NOT EXISTS wa_contacts_lid_idx ON xchats.wa_contacts(account_id, lid_jid);

CREATE TABLE IF NOT EXISTS xchats.wa_chats (
    id                   uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id           uuid NOT NULL REFERENCES xchats.wa_accounts(id) ON DELETE RESTRICT,
    contact_id           uuid NOT NULL REFERENCES xchats.wa_contacts(id) ON DELETE RESTRICT,
    remote_jid           text NOT NULL,
    chat_state           text NOT NULL DEFAULT 'open',
    assignee_user_id     uuid REFERENCES xchats.users(id) ON DELETE SET NULL,
    stage                text NOT NULL DEFAULT '',
    ai_summary           text NOT NULL DEFAULT '',
    last_message_at      timestamptz,
    last_message_preview text NOT NULL DEFAULT '',
    unread_count         int NOT NULL DEFAULT 0,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, remote_jid)
);

CREATE TABLE IF NOT EXISTS xchats.wa_messages (
    id                   uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id           uuid NOT NULL REFERENCES xchats.wa_accounts(id) ON DELETE RESTRICT,
    chat_id              uuid NOT NULL REFERENCES xchats.wa_chats(id) ON DELETE CASCADE,
    direction            text NOT NULL,
    sender_kind          text NOT NULL,
    sender_user_id       uuid REFERENCES xchats.users(id) ON DELETE SET NULL,
    -- nullable: a queued outbound row has no Evolution id until SendText returns
    -- (stamped on send). NULLs are distinct in the UNIQUE below, so many may queue.
    evolution_message_id text,
    participant_jid      text NOT NULL DEFAULT '',
    message_kind         text NOT NULL DEFAULT '',
    body                 text NOT NULL DEFAULT '',
    delivery_state       text NOT NULL DEFAULT 'queued',
    source               text NOT NULL DEFAULT 'live_webhook',
    raw                  jsonb,
    message_ts           timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, evolution_message_id)
);
CREATE INDEX IF NOT EXISTS wa_messages_chat_ts_idx ON xchats.wa_messages(chat_id, message_ts);

-- Media: shipped in Build 0. UNIQUE(message_id) makes the doubled webhook idempotent.
CREATE TABLE IF NOT EXISTS xchats.message_media (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id      uuid NOT NULL REFERENCES xchats.wa_messages(id) ON DELETE CASCADE,
    media_type      text NOT NULL,
    mimetype        text NOT NULL DEFAULT '',
    file_name       text NOT NULL DEFAULT '',
    file_size       int  NOT NULL DEFAULT 0,
    storage_url     text NOT NULL DEFAULT '',
    download_status text NOT NULL DEFAULT 'pending',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (message_id)
);

-- ---------------------------------------------------------------------------
-- AI drafts (Build 0 ships a stub Drafter that writes 1–3 options)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS xchats.ai_drafts (
    id                 uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    chat_id            uuid NOT NULL REFERENCES xchats.wa_chats(id) ON DELETE CASCADE,
    trigger_message_id uuid REFERENCES xchats.wa_messages(id) ON DELETE SET NULL,
    option_ordinal     int NOT NULL,
    draft_text         text NOT NULL DEFAULT '',
    sent_message_id    uuid REFERENCES xchats.wa_messages(id) ON DELETE SET NULL,
    context_state      text NOT NULL DEFAULT 'full',
    confidence         numeric,
    escalate           bool NOT NULL DEFAULT false,
    escalation_reason  text NOT NULL DEFAULT '',
    draft_state        text NOT NULL DEFAULT 'suggested',
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
-- The single pending-suggestion invariant the approve guard relies on.
CREATE UNIQUE INDEX IF NOT EXISTS ai_drafts_pending_uq
    ON xchats.ai_drafts(chat_id, option_ordinal)
    WHERE draft_state = 'suggested';

CREATE TABLE IF NOT EXISTS xchats.ai_draft_assets (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    draft_id   uuid NOT NULL REFERENCES xchats.ai_drafts(id) ON DELETE CASCADE,
    asset_ref  text NOT NULL DEFAULT '',
    media_kind text NOT NULL DEFAULT '',
    media_url  text NOT NULL DEFAULT '',
    ordinal    int NOT NULL,
    UNIQUE (draft_id, ordinal)
);
