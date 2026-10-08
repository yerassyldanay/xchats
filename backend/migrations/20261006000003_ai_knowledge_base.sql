-- 3/5 AI assistant & knowledge base: assistant persona and policies, the typed knowledge
-- tables, the editor's draft/material/request tables, AI drafts, KB-gap telemetry and the
-- KB chat assistant's conversations. Depends on 20261006000001_identity_access.
--
-- Live ai_* tables hold LIVE ROWS ONLY — no review_state, no provenance, no drafted_at:
-- a pending edit lives in kbd_draft's JSON blob until it is approved.
--
-- ai_drafts.chat_id / trigger_message_id / sent_message_id carry NO foreign key: a draft's
-- chat is a wa_chats, tg_chats or channel_chats row depending on its channel, and a single
-- FK cannot express an either/or reference. Ownership is enforced in application code. The
-- same applies to every account_id / chat_id / contact_id column in files 4 and 5.
--
-- Closed-vocabulary TEXT columns (sales_status, pricing_type, availability_status and
-- ai_kb_gap_events.reason_code / target_entity_type / source) carry no CHECK on purpose: Go
-- validates them, so a vocabulary can gain a value without a schema migration.
-- Plain SQL, executed verbatim and identically on SQLite and PostgreSQL (portable types and
-- rules: docs/database.md). Idempotent; the runner owns the transaction.

CREATE TABLE IF NOT EXISTS ai_drafts (
    id                   TEXT PRIMARY KEY NOT NULL,
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
    created_at             BIGINT NOT NULL,
    updated_at             BIGINT NOT NULL,
    reply_language         TEXT NOT NULL DEFAULT '',
    channel                TEXT NOT NULL DEFAULT 'whatsapp'
);

CREATE UNIQUE INDEX IF NOT EXISTS ai_drafts_pending_uq
    ON ai_drafts(channel, chat_id, option_ordinal)
    WHERE draft_state = 'suggested';

CREATE TABLE IF NOT EXISTS ai_assistants (
    id               TEXT PRIMARY KEY NOT NULL,
    organization_id  TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    persona          TEXT NOT NULL DEFAULT '',
    mission          TEXT NOT NULL DEFAULT '',
    guardrails       TEXT NOT NULL DEFAULT '',
    language_policy  TEXT NOT NULL DEFAULT '',
    reply_max_words  INTEGER NOT NULL DEFAULT 120,
    created_at       BIGINT NOT NULL,
    updated_at       BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_policies (
    id                          TEXT PRIMARY KEY NOT NULL,
    organization_id             TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    delivery_cost               TEXT NOT NULL DEFAULT '',
    free_delivery_from          TEXT NOT NULL DEFAULT '',
    min_order                   TEXT NOT NULL DEFAULT '',
    prepayment                  TEXT NOT NULL DEFAULT '',
    installment                 TEXT NOT NULL DEFAULT '',
    warranty                    TEXT NOT NULL DEFAULT '',
    created_at                  BIGINT NOT NULL,
    updated_at                  BIGINT NOT NULL,
    outside_zones_note          TEXT NOT NULL DEFAULT '',
    delivery_in_days            TEXT,
    return_period_in_days       TEXT,
    commerce_policy_documents   TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS ai_delivery_zones (
    id                   TEXT PRIMARY KEY NOT NULL,
    organization_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref                  TEXT NOT NULL,
    name                 TEXT NOT NULL DEFAULT '',
    zone_level           TEXT NOT NULL CHECK (zone_level IN ('city','region','country')),
    parent_ref           TEXT NOT NULL DEFAULT '',
    delivery_available   BOOLEAN NOT NULL,
    delivery_cost        TEXT NOT NULL DEFAULT '',
    delivery_in_days     TEXT NOT NULL DEFAULT '',
    notes                TEXT NOT NULL DEFAULT '',
    created_at           BIGINT NOT NULL,
    updated_at           BIGINT NOT NULL,
    sales_status         TEXT NOT NULL DEFAULT 'active',
    UNIQUE (organization_id, ref)
);

CREATE TABLE IF NOT EXISTS ai_audit_log (
    id                TEXT PRIMARY KEY NOT NULL,
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    action            TEXT NOT NULL,
    actor_user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    note              TEXT NOT NULL DEFAULT '',
    created_at        BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS kbd_draft (
    organization_id  TEXT PRIMARY KEY NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    draft            TEXT NOT NULL DEFAULT '{}',
    base_version     BIGINT NOT NULL DEFAULT 0,
    updated_at       BIGINT NOT NULL,
    updated_by       TEXT REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS kbd_materials (
    id                    TEXT PRIMARY KEY NOT NULL,
    organization_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    source_type           TEXT NOT NULL,
    source_ref            TEXT NOT NULL DEFAULT '',
    blob_id               TEXT NOT NULL DEFAULT '',
    extracted_text        TEXT NOT NULL DEFAULT '',
    media_kind            TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'pending',
    extraction            TEXT NOT NULL DEFAULT '{}',
    created_at            BIGINT NOT NULL,
    updated_at            BIGINT NOT NULL,
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
    extraction_metadata   TEXT NOT NULL DEFAULT '{}',
    processing_status     TEXT NOT NULL DEFAULT 'uploaded',
    customer_visibility   TEXT,
    -- Import-queue bookkeeping mirrored out of extraction_metadata.import so the queue can be
    -- filtered with plain SQL on both engines: the run a material belongs to, and the primary
    -- material's pass-2 status (NULL = not claimed yet). extraction_metadata stays the full
    -- document; Go reads, edits and rewrites it (internal/kbstore/import.go).
    import_run_id            TEXT,
    import_synthesis_status  TEXT
);

CREATE INDEX IF NOT EXISTS kbd_materials_org_idx ON kbd_materials(organization_id);

CREATE INDEX IF NOT EXISTS kbd_materials_org_status_idx ON kbd_materials(organization_id, processing_status);

CREATE INDEX IF NOT EXISTS kbd_materials_import_run_idx ON kbd_materials(import_run_id, import_synthesis_status) WHERE import_run_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS chat_conversations (
    id              TEXT PRIMARY KEY NOT NULL,
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title           TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL,
    updated_at      BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS chat_conversations_owner_idx
    ON chat_conversations(organization_id, user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS ai_tariff_info (
    id                TEXT PRIMARY KEY NOT NULL,
    organization_id   TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    additional_facts  TEXT NOT NULL DEFAULT '[]',
    created_at        BIGINT NOT NULL,
    updated_at        BIGINT NOT NULL
);

-- Append-only telemetry: draft_id is ON DELETE SET NULL so the event outlives its draft;
-- UNIQUE (draft_id) allows at most one event per draft (NULLs stay distinct).
CREATE TABLE IF NOT EXISTS ai_kb_gap_events (
    id                  TEXT PRIMARY KEY NOT NULL,
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
    created_at          BIGINT NOT NULL,
    UNIQUE (draft_id)
);

CREATE INDEX IF NOT EXISTS ai_kb_gap_events_org_created_idx ON ai_kb_gap_events(organization_id, created_at);

CREATE INDEX IF NOT EXISTS ai_kb_gap_events_org_reason_idx ON ai_kb_gap_events(organization_id, reason_code);

CREATE INDEX IF NOT EXISTS ai_kb_gap_events_org_entity_idx ON ai_kb_gap_events(organization_id, target_entity_type, target_entity_ref);

CREATE TABLE IF NOT EXISTS ai_specialists (
    id                TEXT PRIMARY KEY NOT NULL,
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref               TEXT NOT NULL,
    full_name         TEXT NOT NULL DEFAULT '',
    title             TEXT NOT NULL DEFAULT '',
    experience        TEXT NOT NULL DEFAULT '',
    schedule          TEXT NOT NULL DEFAULT '[]',
    booking_url       TEXT NOT NULL DEFAULT '',
    portfolio_images  TEXT NOT NULL DEFAULT '[]',
    sales_status      TEXT NOT NULL DEFAULT 'active',
    created_at        BIGINT NOT NULL,
    updated_at        BIGINT NOT NULL,
    UNIQUE (organization_id, ref)
);

CREATE TABLE IF NOT EXISTS ai_services (
    id               TEXT PRIMARY KEY NOT NULL,
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref              TEXT NOT NULL,
    parent_ref       TEXT NOT NULL DEFAULT '',
    service_type     TEXT NOT NULL DEFAULT 'base',
    category         TEXT NOT NULL DEFAULT '',
    name             TEXT NOT NULL DEFAULT '',
    price            TEXT NOT NULL DEFAULT '',
    duration         INTEGER,
    description      TEXT NOT NULL DEFAULT '',
    specialist_refs  TEXT NOT NULL DEFAULT '[]',
    sales_status     TEXT NOT NULL DEFAULT 'active',
    created_at       BIGINT NOT NULL,
    updated_at       BIGINT NOT NULL,
    UNIQUE (organization_id, ref)
);

CREATE TABLE IF NOT EXISTS ai_topics (
    id                   TEXT PRIMARY KEY NOT NULL,
    organization_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug                 TEXT NOT NULL,
    title                TEXT NOT NULL DEFAULT '',
    body_md              TEXT NOT NULL DEFAULT '',
    created_at           BIGINT NOT NULL,
    updated_at           BIGINT NOT NULL,
    featured_image       TEXT REFERENCES kbd_materials(id),
    illustration_images  TEXT NOT NULL DEFAULT '[]',
    explainer_videos     TEXT NOT NULL DEFAULT '[]',
    reference_documents  TEXT NOT NULL DEFAULT '[]',
    UNIQUE (organization_id, slug)
);

CREATE INDEX IF NOT EXISTS ai_topics_featured_image_idx ON ai_topics(featured_image);

-- additional_facts (here, in ai_tariffs and in ai_tariff_info) is a JSON array of
-- {ref, value, instruction} objects: seller-authored "virtual fact columns" validated and
-- substituted by backend/aiprompt/facts.go, never stored pre-rendered.
CREATE TABLE IF NOT EXISTS ai_products (
    id                     TEXT PRIMARY KEY NOT NULL,
    organization_id        TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref                    TEXT NOT NULL,
    name                   TEXT NOT NULL DEFAULT '',
    price                  TEXT NOT NULL DEFAULT '',
    description            TEXT NOT NULL DEFAULT '',
    category               TEXT NOT NULL DEFAULT '',
    created_at             BIGINT NOT NULL,
    updated_at             BIGINT NOT NULL,
    sales_status           TEXT NOT NULL DEFAULT 'active',
    featured_image         TEXT REFERENCES kbd_materials(id),
    gallery_images         TEXT NOT NULL DEFAULT '[]',
    demo_videos            TEXT NOT NULL DEFAULT '[]',
    certificate_documents  TEXT NOT NULL DEFAULT '[]',
    guarantee_documents    TEXT NOT NULL DEFAULT '[]',
    brand TEXT NOT NULL DEFAULT '',
    advantages TEXT NOT NULL DEFAULT '',
    disadvantages TEXT NOT NULL DEFAULT '',
    best_for TEXT NOT NULL DEFAULT '',
    not_for TEXT NOT NULL DEFAULT '',
    availability_status TEXT NOT NULL DEFAULT 'in_stock',
    availability_note TEXT NOT NULL DEFAULT '',
    installation_terms TEXT NOT NULL DEFAULT '',
    warranty_terms TEXT NOT NULL DEFAULT '',
    additional_facts TEXT NOT NULL DEFAULT '[]',
    UNIQUE (organization_id, ref)
);

CREATE INDEX IF NOT EXISTS ai_products_featured_image_idx ON ai_products(featured_image);

CREATE TABLE IF NOT EXISTS ai_tariffs (
    id                TEXT PRIMARY KEY NOT NULL,
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
    created_at        BIGINT NOT NULL,
    updated_at        BIGINT NOT NULL,
    sales_status      TEXT NOT NULL DEFAULT 'active',
    featured_image    TEXT REFERENCES kbd_materials(id),
    pricing_images    TEXT NOT NULL DEFAULT '[]',
    explainer_videos  TEXT NOT NULL DEFAULT '[]',
    terms_documents   TEXT NOT NULL DEFAULT '[]',
    best_for TEXT NOT NULL DEFAULT '',
    not_for TEXT NOT NULL DEFAULT '',
    additional_facts TEXT NOT NULL DEFAULT '[]',
    UNIQUE (organization_id, ref)
);

CREATE INDEX IF NOT EXISTS ai_tariffs_featured_image_idx ON ai_tariffs(featured_image);

CREATE TABLE IF NOT EXISTS ai_contacts (
    id                        TEXT PRIMARY KEY NOT NULL,
    organization_id           TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    whatsapp                  TEXT NOT NULL DEFAULT '',
    email                     TEXT NOT NULL DEFAULT '',
    address                   TEXT NOT NULL DEFAULT '',
    callback_time             TEXT NOT NULL DEFAULT '',
    created_at                BIGINT NOT NULL,
    updated_at                BIGINT NOT NULL,
    working_hours             TEXT NOT NULL DEFAULT '',
    phone                     TEXT NOT NULL DEFAULT '',
    website                   TEXT NOT NULL DEFAULT '',
    instagram                 TEXT NOT NULL DEFAULT '',
    legal_information         TEXT,
    contact_card_image        TEXT REFERENCES kbd_materials(id),
    location_map_image        TEXT REFERENCES kbd_materials(id),
    company_legal_documents   TEXT NOT NULL DEFAULT '[]',
    booking_url TEXT NOT NULL DEFAULT '',
    schedule TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS ai_contacts_contact_card_image_idx ON ai_contacts(contact_card_image);

CREATE INDEX IF NOT EXISTS ai_contacts_location_map_image_idx ON ai_contacts(location_map_image);

CREATE TABLE IF NOT EXISTS kbd_requests (
    id                TEXT PRIMARY KEY NOT NULL,
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    material_id       TEXT REFERENCES kbd_materials(id) ON DELETE SET NULL,
    req_type          TEXT NOT NULL,
    prompt            TEXT NOT NULL DEFAULT '',
    context           TEXT NOT NULL DEFAULT '{}',
    target            TEXT NOT NULL DEFAULT '{}',
    state             TEXT NOT NULL DEFAULT 'pending',
    resolution        TEXT,
    created_at        BIGINT NOT NULL,
    resolved_at       BIGINT
);

CREATE INDEX IF NOT EXISTS kbd_requests_org_idx ON kbd_requests(organization_id, state);

CREATE TABLE IF NOT EXISTS chat_messages (
    id              TEXT PRIMARY KEY NOT NULL,
    conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    seq             INTEGER NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('user','assistant','system')),
    content         TEXT NOT NULL DEFAULT '',
    metadata        TEXT NOT NULL DEFAULT '{}',
    created_at      BIGINT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS chat_messages_conversation_seq_idx
    ON chat_messages(conversation_id, seq);

CREATE TABLE IF NOT EXISTS ai_kb_gap_missing_fields (
    id          TEXT PRIMARY KEY NOT NULL,
    event_id    TEXT NOT NULL REFERENCES ai_kb_gap_events(id) ON DELETE CASCADE,
    field_name  TEXT NOT NULL,
    created_at  BIGINT NOT NULL,
    UNIQUE (event_id, field_name)
);

CREATE INDEX IF NOT EXISTS ai_kb_gap_missing_fields_event_idx ON ai_kb_gap_missing_fields(event_id);
