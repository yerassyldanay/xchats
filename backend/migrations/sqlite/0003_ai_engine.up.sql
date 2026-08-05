-- xchats SQLite schema, part 3: the AI response engine's config + typed
-- knowledge tables. See 0001_core.up.sql for the shared conventions.
--
-- Forward references: ai_contacts/ai_products/ai_tariffs/ai_topics each
-- hold a featured/contact-card/location-map TEXT column REFERENCES
-- kbd_materials(id) — kbd_materials is created in the NEXT file
-- (0004_knowledge_base.up.sql). SQLite allows a FOREIGN KEY to name a table
-- that does not exist yet at CREATE TABLE time (only enforced later, once
-- both tables exist — verified empirically against this exact ordering),
-- so the file split below mirrors the Postgres migration grouping without
-- needing kbd_materials created first.
--
-- ai_drafts.chat_id / trigger_message_id / sent_message_id carry NO foreign
-- key: migration 0013_telegram_channel (frozen Postgres history) explicitly
-- dropped their FKs when drafts became channel-neutral, since chat_id can
-- point at either wa_chats or tg_chats depending on the channel column and
-- a single FK can't express an either/or reference. That is reproduced
-- as-is here (verified against the live Postgres schema, not just the
-- migration text).

CREATE TABLE ai_assistants (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id  TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    persona          TEXT NOT NULL DEFAULT '',
    mission          TEXT NOT NULL DEFAULT '',
    guardrails       TEXT NOT NULL DEFAULT '',
    language_policy  TEXT NOT NULL DEFAULT '',
    reply_max_words  INTEGER NOT NULL DEFAULT 120,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

CREATE TABLE ai_topics (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug                 TEXT NOT NULL,
    title                TEXT NOT NULL DEFAULT '',
    body_md              TEXT NOT NULL DEFAULT '',
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    featured_image       TEXT REFERENCES kbd_materials(id),
    illustration_images  TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(illustration_images)),
    explainer_videos     TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(explainer_videos)),
    reference_documents  TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(reference_documents)),
    UNIQUE (organization_id, slug)
);
CREATE INDEX ai_topics_featured_image_idx ON ai_topics(featured_image);

CREATE TABLE ai_products (
    id                     TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id        TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref                    TEXT NOT NULL,
    name                   TEXT NOT NULL DEFAULT '',
    price                  TEXT NOT NULL DEFAULT '',
    description            TEXT NOT NULL DEFAULT '',
    category               TEXT NOT NULL DEFAULT '',
    created_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    in_stock               BOOLEAN NOT NULL,
    sales_status           TEXT NOT NULL DEFAULT 'active',
    featured_image         TEXT REFERENCES kbd_materials(id),
    gallery_images         TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(gallery_images)),
    demo_videos            TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(demo_videos)),
    certificate_documents  TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(certificate_documents)),
    guarantee_documents    TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(guarantee_documents)),
    UNIQUE (organization_id, ref)
);
CREATE INDEX ai_products_featured_image_idx ON ai_products(featured_image);

CREATE TABLE ai_tariffs (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
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
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    sales_status      TEXT NOT NULL DEFAULT 'active',
    featured_image    TEXT REFERENCES kbd_materials(id),
    pricing_images    TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(pricing_images)),
    explainer_videos  TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(explainer_videos)),
    terms_documents   TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(terms_documents)),
    UNIQUE (organization_id, ref)
);
CREATE INDEX ai_tariffs_featured_image_idx ON ai_tariffs(featured_image);

CREATE TABLE ai_contacts (
    id                        TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id           TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    whatsapp                  TEXT NOT NULL DEFAULT '',
    email                     TEXT NOT NULL DEFAULT '',
    address                   TEXT NOT NULL DEFAULT '',
    callback_time             TEXT NOT NULL DEFAULT '',
    created_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    working_hours             TEXT NOT NULL DEFAULT '',
    phone                     TEXT NOT NULL DEFAULT '',
    website                   TEXT NOT NULL DEFAULT '',
    instagram                 TEXT NOT NULL DEFAULT '',
    legal_information         TEXT,
    contact_card_image        TEXT REFERENCES kbd_materials(id),
    location_map_image        TEXT REFERENCES kbd_materials(id),
    company_legal_documents   TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(company_legal_documents))
);
CREATE INDEX ai_contacts_contact_card_image_idx ON ai_contacts(contact_card_image);
CREATE INDEX ai_contacts_location_map_image_idx ON ai_contacts(location_map_image);

CREATE TABLE ai_policies (
    id                          TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id             TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    delivery_cost               TEXT NOT NULL DEFAULT '',
    free_delivery_from          TEXT NOT NULL DEFAULT '',
    min_order                   TEXT NOT NULL DEFAULT '',
    prepayment                  TEXT NOT NULL DEFAULT '',
    installment                 TEXT NOT NULL DEFAULT '',
    warranty                    TEXT NOT NULL DEFAULT '',
    created_at                  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at                  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    outside_zones_note          TEXT NOT NULL DEFAULT '',
    delivery_in_days            TEXT,
    return_period_in_days       TEXT,
    commerce_policy_documents   TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(commerce_policy_documents))
);

CREATE TABLE ai_delivery_zones (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id      TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref                  TEXT NOT NULL,
    name                 TEXT NOT NULL DEFAULT '',
    zone_level           TEXT NOT NULL CHECK (zone_level IN ('city','region','country')),
    parent_ref           TEXT NOT NULL DEFAULT '',
    delivery_available   BOOLEAN NOT NULL,
    delivery_cost        TEXT NOT NULL DEFAULT '',
    delivery_in_days     TEXT NOT NULL DEFAULT '',
    notes                TEXT NOT NULL DEFAULT '',
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    sales_status         TEXT NOT NULL DEFAULT 'active',
    UNIQUE (organization_id, ref)
);

CREATE TABLE ai_drafts (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    chat_id              TEXT NOT NULL, -- no FK — see file header
    trigger_message_id   TEXT,          -- no FK — see file header
    option_ordinal       INTEGER NOT NULL,
    draft_text           TEXT NOT NULL DEFAULT '',
    sent_message_id      TEXT,          -- no FK — see file header
    context_state        TEXT NOT NULL DEFAULT 'full',
    confidence           REAL, -- numeric(0..1 LLM score) -> REAL: already consumed as float64, no exact/monetary semantics
    escalate              BOOLEAN NOT NULL DEFAULT FALSE,
    escalation_reason     TEXT NOT NULL DEFAULT '',
    draft_state           TEXT NOT NULL DEFAULT 'suggested',
    created_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    reply_language         TEXT NOT NULL DEFAULT '',
    channel                TEXT NOT NULL DEFAULT 'whatsapp'
);
-- The single pending-suggestion invariant the approve guard relies on.
CREATE UNIQUE INDEX ai_drafts_pending_uq
    ON ai_drafts(channel, chat_id, option_ordinal)
    WHERE draft_state = 'suggested';

CREATE TABLE ai_audit_log (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    action            TEXT NOT NULL,
    actor_user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    note              TEXT NOT NULL DEFAULT '',
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
