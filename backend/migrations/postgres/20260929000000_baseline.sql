-- Authoritative baseline. See docs/database.md for types, authoring and replay rules.

CREATE EXTENSION IF NOT EXISTS citext;

CREATE OR REPLACE FUNCTION xchats_now() RETURNS timestamptz LANGUAGE sql VOLATILE AS $$ SELECT clock_timestamp() $$;

CREATE OR REPLACE FUNCTION unicode_lower(text) RETURNS text LANGUAGE sql IMMUTABLE STRICT AS $$ SELECT lower($1) $$;

-- RFC 7396: recursively merge objects, remove null-valued keys, replace arrays.
CREATE OR REPLACE FUNCTION xchats_json_merge_patch(target jsonb, patch jsonb)
RETURNS jsonb LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE
    k text;
    v jsonb;
    result jsonb;
BEGIN
    IF jsonb_typeof(patch) <> 'object' THEN RETURN patch; END IF;
    result := CASE WHEN jsonb_typeof(target) = 'object' THEN target ELSE '{}'::jsonb END;
    FOR k, v IN SELECT key, value FROM jsonb_each(patch) LOOP
        IF v = 'null'::jsonb THEN
            result := result - k;
        ELSE
            result := jsonb_set(result, ARRAY[k], xchats_json_merge_patch(result -> k, v), true);
        END IF;
    END LOOP;
    RETURN result;
END;
$$;

CREATE TABLE IF NOT EXISTS organizations (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    name                 TEXT NOT NULL,
    respond_mode         TEXT NOT NULL DEFAULT 'NEVER',
    respond_window_start TEXT,
    respond_window_end   TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    timezone TEXT NOT NULL DEFAULT 'Asia/Almaty'
);

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    email         CITEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS ai_drafts (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    chat_id              TEXT NOT NULL,
    trigger_message_id   TEXT,
    option_ordinal       INTEGER NOT NULL,
    draft_text           TEXT NOT NULL DEFAULT '',
    sent_message_id      TEXT,
    context_state        TEXT NOT NULL DEFAULT 'full',
    confidence           NUMERIC,
    escalate              BOOLEAN NOT NULL DEFAULT FALSE,
    escalation_reason     TEXT NOT NULL DEFAULT '',
    draft_state           TEXT NOT NULL DEFAULT 'suggested',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    reply_language         TEXT NOT NULL DEFAULT '',
    channel                TEXT NOT NULL DEFAULT 'whatsapp'
);

CREATE TABLE IF NOT EXISTS mcp_oauth_clients (
    client_id                    TEXT PRIMARY KEY NOT NULL,
    client_name                  TEXT NOT NULL DEFAULT '',
    redirect_uris                JSONB NOT NULL DEFAULT '[]',
    registration_source          TEXT NOT NULL DEFAULT 'dcr' CHECK (registration_source IN ('dcr','cimd')),
    token_endpoint_auth_method   TEXT NOT NULL DEFAULT 'none',
    metadata                     JSONB NOT NULL DEFAULT '{}',
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS mcp_access_token_denylist (
    jti          TEXT PRIMARY KEY NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS automation_settings (
    account_id             TEXT PRIMARY KEY NOT NULL,
    mode                   TEXT NOT NULL DEFAULT 'suggestions' CHECK (mode IN ('off','suggestions','scheduled_auto')),
    wait_seconds_override  INTEGER CHECK (wait_seconds_override IS NULL OR (wait_seconds_override BETWEEN 0 AND 60)),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS automation_schedule_windows (
    id            TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    account_id    TEXT NOT NULL,
    weekday       INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute  INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute    INTEGER NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    CHECK (end_minute <> start_minute)
);

CREATE TABLE IF NOT EXISTS automation_debounce_jobs (
    chat_id        TEXT PRIMARY KEY NOT NULL,
    account_id     TEXT NOT NULL,
    channel        TEXT NOT NULL,
    deadline_at    TIMESTAMPTZ NOT NULL,
    burst_version  BIGINT NOT NULL DEFAULT 1,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','claimed')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS automation_dispatch_jobs (
    id             TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    chat_id        TEXT NOT NULL,
    account_id     TEXT NOT NULL,
    channel        TEXT NOT NULL,
    burst_version  BIGINT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing')),
    attempts       INTEGER NOT NULL DEFAULT 0,
    last_error     TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS campaign_account_settings (
    account_id            TEXT PRIMARY KEY NOT NULL,
    limit_mode            TEXT NOT NULL DEFAULT 'default' CHECK (limit_mode IN ('default','custom')),
    min_interval_seconds  INTEGER NOT NULL DEFAULT 90 CHECK (min_interval_seconds > 0),
    jitter_seconds        INTEGER NOT NULL DEFAULT 30 CHECK (jitter_seconds >= 0),
    paused                BOOLEAN NOT NULL DEFAULT FALSE CHECK (paused IN (FALSE, TRUE)),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS campaign_account_limits (
    account_id            TEXT NOT NULL,
    window_seconds        INTEGER NOT NULL CHECK (window_seconds > 0),
    max_sends             INTEGER NOT NULL CHECK (max_sends > 0),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    PRIMARY KEY (account_id, window_seconds)
);

CREATE TABLE IF NOT EXISTS campaign_account_windows (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    account_id            TEXT NOT NULL,
    weekday               INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute          INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute            INTEGER NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    CHECK (end_minute <> start_minute)
);

CREATE TABLE IF NOT EXISTS organization_users (
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member')),
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE IF NOT EXISTS sessions (
    id                      TEXT PRIMARY KEY NOT NULL,
    user_id                 TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    expires_at              TIMESTAMPTZ NOT NULL,
    active_organization_id  TEXT REFERENCES organizations(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS wa_accounts (
    id                      TEXT PRIMARY KEY NOT NULL,
    organization_id         TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    display_name            TEXT NOT NULL DEFAULT '',
    owner_jid               TEXT NOT NULL UNIQUE,
    phone_number            TEXT NOT NULL DEFAULT '',
    connection_state        TEXT NOT NULL DEFAULT 'connected',
    last_live_event_at      TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    deleted_at               TIMESTAMPTZ,
    channel                 TEXT NOT NULL DEFAULT 'whatsapp' CHECK (channel IN ('whatsapp','simulator'))
);

CREATE TABLE IF NOT EXISTS tg_accounts (
    id                       TEXT PRIMARY KEY NOT NULL,
    organization_id          TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    display_name             TEXT NOT NULL DEFAULT '',
    bot_id                   BIGINT NOT NULL UNIQUE,
    bot_username             TEXT NOT NULL DEFAULT '',
    connection_state         TEXT NOT NULL DEFAULT 'connecting',
    webhook_url               TEXT NOT NULL DEFAULT '',
    webhook_registered_at    TIMESTAMPTZ,
    webhook_last_checked_at  TIMESTAMPTZ,
    webhook_last_error       TEXT NOT NULL DEFAULT '',
    last_live_event_at       TIMESTAMPTZ,
    deleted_at                TIMESTAMPTZ,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS ai_assistants (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id  TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    persona          TEXT NOT NULL DEFAULT '',
    mission          TEXT NOT NULL DEFAULT '',
    guardrails       TEXT NOT NULL DEFAULT '',
    language_policy  TEXT NOT NULL DEFAULT '',
    reply_max_words  INTEGER NOT NULL DEFAULT 120,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS ai_policies (
    id                          TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id             TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    delivery_cost               TEXT NOT NULL DEFAULT '',
    free_delivery_from          TEXT NOT NULL DEFAULT '',
    min_order                   TEXT NOT NULL DEFAULT '',
    prepayment                  TEXT NOT NULL DEFAULT '',
    installment                 TEXT NOT NULL DEFAULT '',
    warranty                    TEXT NOT NULL DEFAULT '',
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    outside_zones_note          TEXT NOT NULL DEFAULT '',
    delivery_in_days            TEXT,
    return_period_in_days       TEXT,
    commerce_policy_documents   JSONB NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS ai_delivery_zones (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref                  TEXT NOT NULL,
    name                 TEXT NOT NULL DEFAULT '',
    zone_level           TEXT NOT NULL CHECK (zone_level IN ('city','region','country')),
    parent_ref           TEXT NOT NULL DEFAULT '',
    delivery_available   BOOLEAN NOT NULL,
    delivery_cost        TEXT NOT NULL DEFAULT '',
    delivery_in_days     TEXT NOT NULL DEFAULT '',
    notes                TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    sales_status         TEXT NOT NULL DEFAULT 'active',
    UNIQUE (organization_id, ref)
);

CREATE TABLE IF NOT EXISTS ai_audit_log (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    action            TEXT NOT NULL,
    actor_user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    note              TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS kbd_draft (
    organization_id  TEXT PRIMARY KEY NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    draft            JSONB NOT NULL DEFAULT '{}',
    base_version     BIGINT NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_by       TEXT REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS kbd_materials (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    source_type           TEXT NOT NULL,
    source_ref            TEXT NOT NULL DEFAULT '',
    blob_id               TEXT NOT NULL DEFAULT '',
    extracted_text        TEXT NOT NULL DEFAULT '',
    media_kind            TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'pending',
    extraction            JSONB NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    source_text           TEXT,
    operator_note         TEXT,
    storage_backend       TEXT,
    storage_key           TEXT,
    filename              TEXT,
    mime_type             TEXT,
    size_bytes            BIGINT,
    sha256_checksum       TEXT,
    visual_summary        TEXT,
    transcript_text       TEXT,
    extraction_metadata   JSONB NOT NULL DEFAULT '{}',
    processing_status     TEXT NOT NULL DEFAULT 'uploaded',
    customer_visibility   TEXT
);

CREATE TABLE IF NOT EXISTS mcp_authorization_codes (
    code_hash              TEXT PRIMARY KEY NOT NULL,
    client_id              TEXT NOT NULL REFERENCES mcp_oauth_clients(client_id) ON DELETE CASCADE,
    redirect_uri            TEXT NOT NULL,
    code_challenge           TEXT NOT NULL,
    code_challenge_method   TEXT NOT NULL DEFAULT 'S256' CHECK (code_challenge_method = 'S256'),
    user_id                 TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id         TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scope                   TEXT NOT NULL DEFAULT '',
    resource                TEXT NOT NULL DEFAULT '',
    expires_at               TIMESTAMPTZ NOT NULL,
    consumed_at              TIMESTAMPTZ,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS mcp_refresh_tokens (
    token_hash        TEXT PRIMARY KEY NOT NULL,
    client_id         TEXT NOT NULL REFERENCES mcp_oauth_clients(client_id) ON DELETE CASCADE,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scope             TEXT NOT NULL DEFAULT '',
    resource          TEXT NOT NULL DEFAULT '',
    expires_at        TIMESTAMPTZ NOT NULL,
    revoked_at        TIMESTAMPTZ,
    replaced_by       TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS channel_accounts (
    id                       TEXT PRIMARY KEY NOT NULL,
    organization_id          TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    channel                  TEXT NOT NULL,
    external_account_id      TEXT NOT NULL,
    display_name             TEXT NOT NULL DEFAULT '',
    handle                   TEXT NOT NULL DEFAULT '',
    connection_state         TEXT NOT NULL DEFAULT 'connecting',
    webhook_url               TEXT NOT NULL DEFAULT '',
    webhook_registered_at    TIMESTAMPTZ,
    webhook_last_checked_at  TIMESTAMPTZ,
    webhook_last_error       TEXT NOT NULL DEFAULT '',
    last_live_event_at       TIMESTAMPTZ,


    provider_meta             JSONB NOT NULL DEFAULT '{}',
    deleted_at                TIMESTAMPTZ,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (channel, external_account_id)
);

CREATE TABLE IF NOT EXISTS campaigns (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,
    account_id            TEXT NOT NULL,
    channel               TEXT NOT NULL CHECK (channel IN ('whatsapp','simulator','telegram','instagram','messenger','whatsapp_cloud')),
    status                TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','scheduled','running','paused','completed','failed','cancelled')),
    message_body          TEXT NOT NULL DEFAULT '',



    variables             JSONB NOT NULL DEFAULT '[]',
    min_interval_seconds  INTEGER CHECK (min_interval_seconds IS NULL OR min_interval_seconds > 0),
    jitter_seconds        INTEGER CHECK (jitter_seconds IS NULL OR jitter_seconds >= 0),
    schedule_at           TIMESTAMPTZ,
    started_at            TIMESTAMPTZ,
    created_by            TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),


    CHECK ((min_interval_seconds IS NULL) = (jitter_seconds IS NULL))
);

CREATE TABLE IF NOT EXISTS crm_statuses (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug             TEXT NOT NULL,
    name             TEXT NOT NULL,
    color            TEXT NOT NULL DEFAULT '',
    position         INTEGER NOT NULL DEFAULT 0,
    is_default       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (organization_id, slug)
);

CREATE TABLE IF NOT EXISTS crm_tags (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug             TEXT NOT NULL,
    name             TEXT NOT NULL,
    color            TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (organization_id, slug)
);

CREATE TABLE IF NOT EXISTS crm_custom_field_defs (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key              TEXT NOT NULL,
    label            TEXT NOT NULL,
    field_type       TEXT NOT NULL DEFAULT 'text' CHECK (field_type IN ('text','number','date','select')),
    options          JSONB NOT NULL DEFAULT '[]',
    position         INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (organization_id, key)
);

CREATE TABLE IF NOT EXISTS campaign_templates (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,
    message_body          TEXT NOT NULL DEFAULT '',



    variables             JSONB NOT NULL DEFAULT '[]',
    is_archived           BOOLEAN NOT NULL DEFAULT FALSE CHECK (is_archived IN (FALSE, TRUE)),
    created_by            TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS chat_conversations (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS ai_tariff_info (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id   TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    additional_facts  JSONB NOT NULL DEFAULT '[]',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS ai_kb_gap_events (
    id                  TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    draft_id            TEXT REFERENCES ai_drafts(id) ON DELETE SET NULL,
    channel             TEXT NOT NULL DEFAULT 'whatsapp',
    chat_id             TEXT NOT NULL,
    trigger_message_id  TEXT,
    reason_code         TEXT NOT NULL,
    target_entity_type  TEXT NOT NULL DEFAULT '',
    target_entity_ref   TEXT NOT NULL DEFAULT '',
    escalation_reason   TEXT NOT NULL DEFAULT '',
    source              TEXT NOT NULL DEFAULT 'model',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (draft_id)
);

CREATE TABLE IF NOT EXISTS ai_specialists (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref               TEXT NOT NULL,
    full_name         TEXT NOT NULL DEFAULT '',
    title             TEXT NOT NULL DEFAULT '',
    experience        TEXT NOT NULL DEFAULT '',
    schedule          JSONB NOT NULL DEFAULT '[]',
    booking_url       TEXT NOT NULL DEFAULT '',
    portfolio_images  JSONB NOT NULL DEFAULT '[]',
    sales_status      TEXT NOT NULL DEFAULT 'active',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (organization_id, ref)
);

CREATE TABLE IF NOT EXISTS ai_services (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref              TEXT NOT NULL,
    parent_ref       TEXT NOT NULL DEFAULT '',
    service_type     TEXT NOT NULL DEFAULT 'base',
    category         TEXT NOT NULL DEFAULT '',
    name             TEXT NOT NULL DEFAULT '',
    price            TEXT NOT NULL DEFAULT '',
    duration         INTEGER,
    description      TEXT NOT NULL DEFAULT '',
    specialist_refs  JSONB NOT NULL DEFAULT '[]',
    sales_status     TEXT NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (organization_id, ref)
);

CREATE TABLE IF NOT EXISTS wa_contacts (
    id           TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    account_id   TEXT NOT NULL REFERENCES wa_accounts(id) ON DELETE RESTRICT,
    phone_number TEXT NOT NULL DEFAULT '',
    phone_jid    TEXT NOT NULL,
    lid_jid      TEXT,
    push_name    TEXT NOT NULL DEFAULT '',
    display_name TEXT NOT NULL DEFAULT '',
    avatar_url   TEXT,
    attributes   JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (account_id, phone_jid)
);

CREATE TABLE IF NOT EXISTS tg_credentials (
    account_id              TEXT PRIMARY KEY NOT NULL REFERENCES tg_accounts(id) ON DELETE CASCADE,
    bot_token_enc           BYTEA NOT NULL,
    encryption_key_version  INTEGER NOT NULL DEFAULT 1,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS tg_contacts (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    account_id        TEXT NOT NULL REFERENCES tg_accounts(id) ON DELETE RESTRICT,
    telegram_user_id  BIGINT NOT NULL,
    username          TEXT NOT NULL DEFAULT '',
    first_name        TEXT NOT NULL DEFAULT '',
    last_name         TEXT NOT NULL DEFAULT '',
    display_name      TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (account_id, telegram_user_id)
);

CREATE TABLE IF NOT EXISTS tg_poll_state (
    account_id      TEXT PRIMARY KEY NOT NULL REFERENCES tg_accounts(id) ON DELETE CASCADE,
    last_update_id  BIGINT NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS ai_topics (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug                 TEXT NOT NULL,
    title                TEXT NOT NULL DEFAULT '',
    body_md              TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    featured_image       TEXT REFERENCES kbd_materials(id),
    illustration_images  JSONB NOT NULL DEFAULT '[]',
    explainer_videos     JSONB NOT NULL DEFAULT '[]',
    reference_documents  JSONB NOT NULL DEFAULT '[]',
    UNIQUE (organization_id, slug)
);

CREATE TABLE IF NOT EXISTS ai_products (
    id                     TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id        TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref                    TEXT NOT NULL,
    name                   TEXT NOT NULL DEFAULT '',
    price                  TEXT NOT NULL DEFAULT '',
    description            TEXT NOT NULL DEFAULT '',
    category               TEXT NOT NULL DEFAULT '',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    sales_status           TEXT NOT NULL DEFAULT 'active',
    featured_image         TEXT REFERENCES kbd_materials(id),
    gallery_images         JSONB NOT NULL DEFAULT '[]',
    demo_videos            JSONB NOT NULL DEFAULT '[]',
    certificate_documents  JSONB NOT NULL DEFAULT '[]',
    guarantee_documents    JSONB NOT NULL DEFAULT '[]',
    brand TEXT NOT NULL DEFAULT '',
    advantages TEXT NOT NULL DEFAULT '',
    disadvantages TEXT NOT NULL DEFAULT '',
    best_for TEXT NOT NULL DEFAULT '',
    not_for TEXT NOT NULL DEFAULT '',
    availability_status TEXT NOT NULL DEFAULT 'in_stock',
    availability_note TEXT NOT NULL DEFAULT '',
    installation_terms TEXT NOT NULL DEFAULT '',
    warranty_terms TEXT NOT NULL DEFAULT '',
    additional_facts JSONB NOT NULL DEFAULT '[]',
    UNIQUE (organization_id, ref)
);

CREATE TABLE IF NOT EXISTS ai_tariffs (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref               TEXT NOT NULL,
    name              TEXT NOT NULL DEFAULT '',
    price             TEXT NOT NULL DEFAULT '',
    limit_text        TEXT NOT NULL DEFAULT '',
    fee               TEXT NOT NULL DEFAULT '',
    summary           TEXT NOT NULL DEFAULT '',
    pricing_type      TEXT NOT NULL DEFAULT 'fixed',
    advantages        TEXT NOT NULL DEFAULT '',
    disadvantages     TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    sales_status      TEXT NOT NULL DEFAULT 'active',
    featured_image    TEXT REFERENCES kbd_materials(id),
    pricing_images    JSONB NOT NULL DEFAULT '[]',
    explainer_videos  JSONB NOT NULL DEFAULT '[]',
    terms_documents   JSONB NOT NULL DEFAULT '[]',
    best_for TEXT NOT NULL DEFAULT '',
    not_for TEXT NOT NULL DEFAULT '',
    additional_facts JSONB NOT NULL DEFAULT '[]',
    UNIQUE (organization_id, ref)
);

CREATE TABLE IF NOT EXISTS ai_contacts (
    id                        TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id           TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    whatsapp                  TEXT NOT NULL DEFAULT '',
    email                     TEXT NOT NULL DEFAULT '',
    address                   TEXT NOT NULL DEFAULT '',
    callback_time             TEXT NOT NULL DEFAULT '',
    created_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    working_hours             TEXT NOT NULL DEFAULT '',
    phone                     TEXT NOT NULL DEFAULT '',
    website                   TEXT NOT NULL DEFAULT '',
    instagram                 TEXT NOT NULL DEFAULT '',
    legal_information         TEXT,
    contact_card_image        TEXT REFERENCES kbd_materials(id),
    location_map_image        TEXT REFERENCES kbd_materials(id),
    company_legal_documents   JSONB NOT NULL DEFAULT '[]',
    booking_url TEXT NOT NULL DEFAULT '',
    schedule JSONB NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS kbd_requests (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    material_id       TEXT REFERENCES kbd_materials(id) ON DELETE SET NULL,
    req_type          TEXT NOT NULL,
    prompt            TEXT NOT NULL DEFAULT '',
    context           JSONB NOT NULL DEFAULT '{}',
    target            JSONB NOT NULL DEFAULT '{}',
    state             TEXT NOT NULL DEFAULT 'pending',
    resolution        JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    resolved_at       TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS wa_credentials (
    account_id  TEXT PRIMARY KEY NOT NULL REFERENCES wa_accounts(id) ON DELETE CASCADE,
    device_jid  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS channel_credentials (
    account_id              TEXT PRIMARY KEY NOT NULL REFERENCES channel_accounts(id) ON DELETE CASCADE,
    secret_enc               BYTEA NOT NULL,
    encryption_key_version   INTEGER NOT NULL DEFAULT 1,



    token_kind                TEXT NOT NULL DEFAULT '',
    expires_at                TIMESTAMPTZ,
    refreshed_at              TIMESTAMPTZ,
    refresh_last_error        TEXT NOT NULL DEFAULT '',
    created_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS channel_contacts (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    account_id            TEXT NOT NULL REFERENCES channel_accounts(id) ON DELETE RESTRICT,
    external_contact_id   TEXT NOT NULL,
    handle                TEXT NOT NULL DEFAULT '',
    display_name          TEXT NOT NULL DEFAULT '',
    attributes            JSONB NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
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
    expires_at        TIMESTAMPTZ NOT NULL,
    settled_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS meta_webhook_events (
    id            TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    channel       TEXT NOT NULL CHECK (channel IN ('instagram','messenger','whatsapp_cloud')),
    object_id     TEXT NOT NULL DEFAULT '',
    account_id    TEXT REFERENCES channel_accounts(id) ON DELETE SET NULL,
    event_key     TEXT NOT NULL,
    outcome       TEXT NOT NULL CHECK (outcome IN ('stored','duplicate','ignored','unknown_account','bad_signature','error')),
    detail        TEXT NOT NULL DEFAULT '',
    received_at   TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (channel, event_key)
);

CREATE TABLE IF NOT EXISTS campaign_recipients (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    normalized_identity   TEXT NOT NULL,
    raw_input             TEXT NOT NULL DEFAULT '',
    name                  TEXT NOT NULL DEFAULT '',
    attributes            JSONB NOT NULL DEFAULT '{}',
    status                TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sending','sent','failed','skipped')),
    failure_reason        TEXT NOT NULL DEFAULT '',
    attempts              INTEGER NOT NULL DEFAULT 0,



    next_attempt_at       TIMESTAMPTZ,
    chat_id               TEXT,
    message_id            TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (campaign_id, normalized_identity)
);

CREATE TABLE IF NOT EXISTS campaign_events (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    event                 TEXT NOT NULL,
    actor_user_id         TEXT REFERENCES users(id) ON DELETE SET NULL,
    detail                JSONB NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS campaign_windows (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    weekday               INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute          INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute            INTEGER NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    CHECK (end_minute <> start_minute)
);

CREATE TABLE IF NOT EXISTS crm_customers (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    display_name      TEXT NOT NULL DEFAULT '',
    phone             TEXT NOT NULL DEFAULT '',
    email             TEXT NOT NULL DEFAULT '',
    avatar_url        TEXT NOT NULL DEFAULT '',
    status_id         TEXT REFERENCES crm_statuses(id) ON DELETE SET NULL,
    assignee_user_id  TEXT REFERENCES users(id) ON DELETE SET NULL,
    custom_fields     JSONB NOT NULL DEFAULT '{}',
    merged_into_id    TEXT REFERENCES crm_customers(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,





    seq             INTEGER NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('user','assistant','system')),
    content         TEXT NOT NULL DEFAULT '',



    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS ai_kb_gap_missing_fields (
    id          TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    event_id    TEXT NOT NULL REFERENCES ai_kb_gap_events(id) ON DELETE CASCADE,
    field_name  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (event_id, field_name)
);

CREATE TABLE IF NOT EXISTS wa_chats (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    account_id           TEXT NOT NULL REFERENCES wa_accounts(id) ON DELETE RESTRICT,
    contact_id           TEXT NOT NULL REFERENCES wa_contacts(id) ON DELETE RESTRICT,
    remote_jid           TEXT NOT NULL,
    chat_state           TEXT NOT NULL DEFAULT 'open',
    assignee_user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    stage                TEXT NOT NULL DEFAULT '',
    ai_summary           TEXT NOT NULL DEFAULT '',
    last_message_at      TIMESTAMPTZ,
    last_message_preview TEXT NOT NULL DEFAULT '',
    unread_count         INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (account_id, remote_jid)
);

CREATE TABLE IF NOT EXISTS tg_chats (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    account_id            TEXT NOT NULL REFERENCES tg_accounts(id) ON DELETE RESTRICT,
    contact_id            TEXT NOT NULL REFERENCES tg_contacts(id) ON DELETE RESTRICT,
    telegram_chat_id      BIGINT NOT NULL,
    chat_type             TEXT NOT NULL DEFAULT 'private',
    chat_state            TEXT NOT NULL DEFAULT 'open',
    assignee_user_id      TEXT REFERENCES users(id) ON DELETE SET NULL,
    last_message_at       TIMESTAMPTZ,
    last_message_preview  TEXT NOT NULL DEFAULT '',
    unread_count          INTEGER NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (account_id, telegram_chat_id)
);

CREATE TABLE IF NOT EXISTS channel_chats (
    id                       TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    account_id               TEXT NOT NULL REFERENCES channel_accounts(id) ON DELETE RESTRICT,
    contact_id               TEXT NOT NULL REFERENCES channel_contacts(id) ON DELETE RESTRICT,
    external_thread_id       TEXT NOT NULL,
    chat_state                TEXT NOT NULL DEFAULT 'open',
    assignee_user_id          TEXT REFERENCES users(id) ON DELETE SET NULL,



    last_inbound_at           TIMESTAMPTZ,
    last_message_at           TIMESTAMPTZ,
    last_message_preview      TEXT NOT NULL DEFAULT '',
    unread_count               INTEGER NOT NULL DEFAULT 0,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (account_id, external_thread_id)
);

CREATE TABLE IF NOT EXISTS campaign_send_log (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    account_id            TEXT NOT NULL,
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    recipient_id          TEXT NOT NULL REFERENCES campaign_recipients(id) ON DELETE CASCADE,
    attempted_at          TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    outcome               TEXT NOT NULL CHECK (outcome IN ('sent','failed')),
    origin                TEXT NOT NULL DEFAULT 'campaign' CHECK (origin IN ('campaign','manual','ai'))
);

CREATE TABLE IF NOT EXISTS crm_customer_identities (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id      TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    channel          TEXT NOT NULL,
    account_id       TEXT NOT NULL,
    contact_id       TEXT NOT NULL,
    external_id      TEXT NOT NULL DEFAULT '',
    username         TEXT NOT NULL DEFAULT '',
    phone            TEXT NOT NULL DEFAULT '',
    display_name     TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (channel, contact_id),
    UNIQUE (organization_id, channel, account_id, external_id)
);

CREATE TABLE IF NOT EXISTS crm_customer_tags (
    customer_id  TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    tag_id       TEXT NOT NULL REFERENCES crm_tags(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    PRIMARY KEY (customer_id, tag_id)
);

CREATE TABLE IF NOT EXISTS crm_customer_notes (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id      TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    author_user_id   TEXT REFERENCES users(id) ON DELETE SET NULL,
    body             TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS crm_followups (
    id                  TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id         TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    conversation_id     TEXT,
    channel             TEXT NOT NULL DEFAULT '',
    due_at              TIMESTAMPTZ NOT NULL,
    due_date            TEXT NOT NULL,
    due_minute          INTEGER CHECK (due_minute IS NULL OR (due_minute BETWEEN 0 AND 1439)),
    action              TEXT NOT NULL DEFAULT 'call' CHECK (action IN ('call','message','meeting','other')),
    note                TEXT NOT NULL DEFAULT '',
    assignee_user_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    state               TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open','completed','cancelled')),
    completed_at        TIMESTAMPTZ,
    created_by_user_id  TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS crm_timeline (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id      TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    kind             TEXT NOT NULL CHECK (kind IN (
                         'customer_created','identity_linked','note_added','status_changed',
                         'tag_added','tag_removed','assignee_changed','followup_created',
                         'followup_completed','followup_rescheduled','followup_cancelled',
                         'customers_merged')),
    actor_user_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    summary          TEXT NOT NULL DEFAULT '',
    detail           JSONB NOT NULL DEFAULT '{}',
    occurred_at      TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT (xchats_now())
);

CREATE TABLE IF NOT EXISTS wa_messages (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
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
    raw                  JSONB,
    message_ts           TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (account_id, external_message_id)
);

CREATE TABLE IF NOT EXISTS tg_messages (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
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
    raw                   JSONB,
    message_ts            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (account_id, telegram_update_id),
    UNIQUE (chat_id, telegram_message_id)
);

CREATE TABLE IF NOT EXISTS channel_messages (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
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
    raw                      JSONB,
    message_ts               TIMESTAMPTZ,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    UNIQUE (account_id, external_message_id)
);

CREATE TABLE IF NOT EXISTS message_media (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    message_id      TEXT NOT NULL UNIQUE REFERENCES wa_messages(id) ON DELETE CASCADE,
    media_type      TEXT NOT NULL,
    mimetype        TEXT NOT NULL DEFAULT '',
    file_name       TEXT NOT NULL DEFAULT '',
    file_size       INTEGER NOT NULL DEFAULT 0,
    storage_url     TEXT NOT NULL DEFAULT '',
    download_status TEXT NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    transcript TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS tg_message_media (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    message_id      TEXT NOT NULL UNIQUE REFERENCES tg_messages(id) ON DELETE CASCADE,
    file_id         TEXT NOT NULL,
    file_unique_id  TEXT NOT NULL DEFAULT '',
    media_type      TEXT NOT NULL,
    mimetype        TEXT NOT NULL DEFAULT '',
    filename        TEXT NOT NULL DEFAULT '',
    size            INTEGER NOT NULL DEFAULT 0,
    storage_key     TEXT NOT NULL DEFAULT '',
    download_status TEXT NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    transcript TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS channel_message_media (
    id              TEXT PRIMARY KEY NOT NULL DEFAULT (gen_random_uuid()::text),
    message_id      TEXT NOT NULL UNIQUE REFERENCES channel_messages(id) ON DELETE CASCADE,


    provider_ref     TEXT NOT NULL DEFAULT '',


    source_url       TEXT NOT NULL DEFAULT '',
    media_type       TEXT NOT NULL,
    mimetype         TEXT NOT NULL DEFAULT '',
    filename         TEXT NOT NULL DEFAULT '',
    size              INTEGER NOT NULL DEFAULT 0,
    storage_key       TEXT NOT NULL DEFAULT '',
    download_status   TEXT NOT NULL DEFAULT 'pending',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT (xchats_now()),
    transcript TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions(user_id);

CREATE INDEX IF NOT EXISTS wa_accounts_org_idx ON wa_accounts(organization_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS wa_contacts_lid_idx ON wa_contacts(account_id, lid_jid);

CREATE INDEX IF NOT EXISTS wa_messages_chat_ts_idx ON wa_messages(chat_id, message_ts);

CREATE INDEX IF NOT EXISTS tg_accounts_org_idx ON tg_accounts(organization_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS tg_chats_last_message_idx ON tg_chats(last_message_at DESC);

CREATE INDEX IF NOT EXISTS tg_messages_chat_ts_idx ON tg_messages(chat_id, message_ts);

CREATE INDEX IF NOT EXISTS tg_message_media_pending_idx ON tg_message_media(updated_at) WHERE download_status <> 'ready';

CREATE INDEX IF NOT EXISTS ai_topics_featured_image_idx ON ai_topics(featured_image);

CREATE INDEX IF NOT EXISTS ai_products_featured_image_idx ON ai_products(featured_image);

CREATE INDEX IF NOT EXISTS ai_tariffs_featured_image_idx ON ai_tariffs(featured_image);

CREATE INDEX IF NOT EXISTS ai_contacts_contact_card_image_idx ON ai_contacts(contact_card_image);

CREATE INDEX IF NOT EXISTS ai_contacts_location_map_image_idx ON ai_contacts(location_map_image);

CREATE UNIQUE INDEX IF NOT EXISTS ai_drafts_pending_uq
    ON ai_drafts(channel, chat_id, option_ordinal)
    WHERE draft_state = 'suggested';

CREATE INDEX IF NOT EXISTS kbd_materials_org_idx ON kbd_materials(organization_id);

CREATE INDEX IF NOT EXISTS kbd_materials_org_status_idx ON kbd_materials(organization_id, processing_status);

CREATE INDEX IF NOT EXISTS kbd_requests_org_idx ON kbd_requests(organization_id, state);

CREATE INDEX IF NOT EXISTS mcp_authorization_codes_expires_idx ON mcp_authorization_codes(expires_at);

CREATE INDEX IF NOT EXISTS mcp_refresh_tokens_client_user_idx ON mcp_refresh_tokens(client_id, user_id);

CREATE INDEX IF NOT EXISTS mcp_access_token_denylist_expires_idx ON mcp_access_token_denylist(expires_at);

CREATE INDEX IF NOT EXISTS automation_schedule_windows_account_idx ON automation_schedule_windows(account_id);

CREATE INDEX IF NOT EXISTS automation_debounce_jobs_due_idx ON automation_debounce_jobs(status, deadline_at);

CREATE INDEX IF NOT EXISTS automation_debounce_jobs_account_idx ON automation_debounce_jobs(account_id);

CREATE INDEX IF NOT EXISTS automation_dispatch_jobs_status_idx ON automation_dispatch_jobs(status, updated_at);

CREATE INDEX IF NOT EXISTS automation_dispatch_jobs_chat_idx ON automation_dispatch_jobs(chat_id);

CREATE UNIQUE INDEX IF NOT EXISTS automation_dispatch_jobs_burst_idx
    ON automation_dispatch_jobs(chat_id, burst_version);

CREATE INDEX IF NOT EXISTS channel_accounts_org_idx ON channel_accounts(organization_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS channel_chats_last_message_idx ON channel_chats(last_message_at DESC);

CREATE INDEX IF NOT EXISTS channel_messages_chat_ts_idx ON channel_messages(chat_id, message_ts);

CREATE INDEX IF NOT EXISTS channel_message_media_pending_idx ON channel_message_media(updated_at) WHERE download_status <> 'ready';

CREATE INDEX IF NOT EXISTS meta_oauth_states_expires_idx ON meta_oauth_states(expires_at);

CREATE INDEX IF NOT EXISTS meta_webhook_events_account_idx ON meta_webhook_events(account_id);

CREATE INDEX IF NOT EXISTS meta_webhook_events_received_idx ON meta_webhook_events(received_at);

CREATE OR REPLACE VIEW inbox_accounts_v AS
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
    CAST(NULL AS TIMESTAMPTZ) AS webhook_registered_at,
    CAST(NULL AS TIMESTAMPTZ) AS webhook_last_checked_at,
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

CREATE OR REPLACE VIEW inbox_chats_v AS
SELECT c.id,
    c.account_id,
    a.organization_id,
    a.channel,
    a.deleted_at AS account_deleted_at,
    c.remote_jid AS external_conversation_ref,
    c.chat_state,
    c.assignee_user_id,
    c.last_message_at,
    CAST(NULL AS TIMESTAMPTZ) AS last_inbound_at,
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
    CAST(NULL AS TIMESTAMPTZ) AS last_inbound_at,
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

CREATE OR REPLACE VIEW inbox_messages_v AS
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

CREATE INDEX IF NOT EXISTS campaigns_org_idx ON campaigns(organization_id);

CREATE INDEX IF NOT EXISTS campaigns_account_status_idx ON campaigns(account_id, status);

CREATE INDEX IF NOT EXISTS campaign_recipients_campaign_status_idx ON campaign_recipients(campaign_id, status);

CREATE INDEX IF NOT EXISTS campaign_recipients_identity_status_idx ON campaign_recipients(normalized_identity, status);

CREATE INDEX IF NOT EXISTS campaign_events_campaign_idx ON campaign_events(campaign_id, created_at);

CREATE INDEX IF NOT EXISTS campaign_account_windows_account_idx ON campaign_account_windows(account_id);

CREATE INDEX IF NOT EXISTS campaign_windows_campaign_idx ON campaign_windows(campaign_id);

CREATE INDEX IF NOT EXISTS campaign_send_log_account_time_idx ON campaign_send_log(account_id, attempted_at);

CREATE UNIQUE INDEX IF NOT EXISTS crm_statuses_default_uq ON crm_statuses(organization_id) WHERE is_default = TRUE;

CREATE INDEX IF NOT EXISTS crm_customers_org_idx ON crm_customers(organization_id) WHERE merged_into_id IS NULL;

CREATE INDEX IF NOT EXISTS crm_customers_org_assignee_idx ON crm_customers(organization_id, assignee_user_id);

CREATE INDEX IF NOT EXISTS crm_customers_org_status_idx ON crm_customers(organization_id, status_id);

CREATE INDEX IF NOT EXISTS crm_customer_identities_customer_idx ON crm_customer_identities(customer_id);

CREATE INDEX IF NOT EXISTS crm_customer_tags_tag_idx ON crm_customer_tags(tag_id);

CREATE INDEX IF NOT EXISTS crm_customer_notes_customer_idx ON crm_customer_notes(customer_id, created_at);

CREATE INDEX IF NOT EXISTS crm_followups_due_idx ON crm_followups(organization_id, state, due_at);

CREATE INDEX IF NOT EXISTS crm_followups_assignee_idx ON crm_followups(assignee_user_id, state, due_at);

CREATE INDEX IF NOT EXISTS crm_followups_customer_idx ON crm_followups(customer_id, due_at);

CREATE INDEX IF NOT EXISTS crm_timeline_customer_idx ON crm_timeline(customer_id, occurred_at);

CREATE INDEX IF NOT EXISTS campaign_templates_org_idx ON campaign_templates(organization_id);

CREATE INDEX IF NOT EXISTS campaign_templates_org_archived_idx ON campaign_templates(organization_id, is_archived);

CREATE INDEX IF NOT EXISTS chat_conversations_owner_idx
    ON chat_conversations(organization_id, user_id, updated_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS chat_messages_conversation_seq_idx
    ON chat_messages(conversation_id, seq);

CREATE INDEX IF NOT EXISTS ai_kb_gap_events_org_created_idx ON ai_kb_gap_events(organization_id, created_at);

CREATE INDEX IF NOT EXISTS ai_kb_gap_events_org_reason_idx ON ai_kb_gap_events(organization_id, reason_code);

CREATE INDEX IF NOT EXISTS ai_kb_gap_events_org_entity_idx ON ai_kb_gap_events(organization_id, target_entity_type, target_entity_ref);

CREATE INDEX IF NOT EXISTS ai_kb_gap_missing_fields_event_idx ON ai_kb_gap_missing_fields(event_id);

CREATE OR REPLACE VIEW inbox_message_media_v AS
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

-- Required initial state; replay never overwrites an operator's changes.
INSERT INTO organizations (id, name, respond_mode)
VALUES ('00000000-0000-0000-0000-000000000001', 'xchats', 'NEVER') ON CONFLICT DO NOTHING;
INSERT INTO users (id, email, password_hash, display_name, must_change_password)
VALUES ('00000000-0000-0000-0000-000000000002', 'admin@xchat.kz',
'$argon2id$v=19$m=65536,t=1,p=4$eZE9z7aFgeOEeYVAUCJTxg$3x3PW6uhMxX+nhuXZZZ79JQOKAoImKMB/ACkGsqq9io',
'Admin', TRUE) ON CONFLICT DO NOTHING;
INSERT INTO organization_users (organization_id, user_id, role)
VALUES ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'admin') ON CONFLICT DO NOTHING;
INSERT INTO crm_statuses (organization_id, slug, name, color, position, is_default)
VALUES
('00000000-0000-0000-0000-000000000001', 'new', 'Новый', '#64748b', 1, TRUE),
('00000000-0000-0000-0000-000000000001', 'interested', 'Интересуется', '#0ea5e9', 2, FALSE),
('00000000-0000-0000-0000-000000000001', 'in_progress', 'В работе', '#f59e0b', 3, FALSE),
('00000000-0000-0000-0000-000000000001', 'customer', 'Клиент', '#10b981', 4, FALSE),
('00000000-0000-0000-0000-000000000001', 'lost', 'Потерян', '#ef4444', 5, FALSE)
ON CONFLICT DO NOTHING;

