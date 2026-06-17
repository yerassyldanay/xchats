-- 0003_ai_kb — the writable, DB-backed Knowledge Base + Playground draft lifecycle.
--
-- Until now the KB was a Go literal (internal/brain/seed.go); the brain read it
-- directly. This migration lands the KB tables designed in plan/9-database-schema.md
-- (ai_snapshots / ai_topics / ai_assets / ai_values) PLUS the playground's draft-side
-- additions (plan/12): review_state + provenance on every KB row, the ai_builder_requests
-- popup queue, and the ai_materials ingest-staging table. The brain switches from reading
-- the literal to reading the PUBLISHED snapshot loaded from these tables (literal kept as
-- the seed + boot-time fallback).
--
-- Naming note (plan/9 vs live): the RUNTIME suggestion store is ai_drafts/ai_draft_assets
-- in migration 0001 (plan/9 calls it ai_suggestions). The playground touches only the KB
-- tables below and is unaffected by that divergence.

SET search_path = xchats, public;

-- A versioned KB config the brain prompt is built from. One published snapshot is
-- live at a time; at most one draft per org (the playground's working copy).
CREATE TABLE IF NOT EXISTS xchats.ai_snapshots (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    version         int  NOT NULL DEFAULT 0,
    snapshot_state  text NOT NULL DEFAULT 'draft',   -- 'draft' | 'published'
    persona         text NOT NULL DEFAULT '',
    mission         text NOT NULL DEFAULT '',
    guardrails      text NOT NULL DEFAULT '',
    language_policy text NOT NULL DEFAULT '',
    reply_max_words int  NOT NULL DEFAULT 120,
    published_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, version)
);
-- One draft per org (the playground's single working copy). A published snapshot
-- has version > 0; the open draft is version 0 until it is published.
CREATE UNIQUE INDEX IF NOT EXISTS ai_snapshots_one_draft_uq
    ON xchats.ai_snapshots(organization_id)
    WHERE snapshot_state = 'draft';

-- Topics — the knowledge, each a container of body text (price TOKENS, never digits).
CREATE TABLE IF NOT EXISTS xchats.ai_topics (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    slug        text NOT NULL,
    lang        text NOT NULL DEFAULT 'ru',
    title       text NOT NULL DEFAULT '',
    keywords    text NOT NULL DEFAULT '',
    body_md     text NOT NULL DEFAULT '',
    review_state text NOT NULL DEFAULT 'approved',   -- 'proposed' | 'approved' | 'rejected'
    provenance  jsonb NOT NULL DEFAULT '{}'::jsonb,   -- { source, material_id, at }
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (snapshot_id, slug)
);

-- Assets — the media catalog; each asset carries its own selection-cue description.
CREATE TABLE IF NOT EXISTS xchats.ai_assets (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    ref         text NOT NULL,                        -- stable id the model selects on (also the blob id)
    asset_kind  text NOT NULL DEFAULT 'image',        -- 'image'|'video'|'document'|'audio'
    topic_slug  text NOT NULL DEFAULT '',
    title       text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    asset_url   text NOT NULL DEFAULT '',
    lang        text NOT NULL DEFAULT 'ru',
    review_state text NOT NULL DEFAULT 'approved',
    provenance  jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (snapshot_id, ref)
);

-- Values — the single source of confirmed numbers/contacts/limits, as (token,lang) → text.
CREATE TABLE IF NOT EXISTS xchats.ai_values (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    token       text NOT NULL,                        -- namespace.key
    lang        text NOT NULL DEFAULT '*',            -- 'ru'|'kk'|'*'
    value_text  text NOT NULL DEFAULT '',             -- confirmed value, any unit, substituted verbatim
    description text NOT NULL DEFAULT '',              -- human-only
    review_state text NOT NULL DEFAULT 'approved',
    provenance  jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (snapshot_id, token, lang)
);

-- ai_materials — Stage-1 ⇄ Stage-2 staging. One row per dropped input (the
-- NormalizedMaterial contract): every input type is driven to extracted_text here
-- before any KB reasoning, so the synthesis agent sees one uniform shape.
CREATE TABLE IF NOT EXISTS xchats.ai_materials (
    id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id    uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    source_type    text NOT NULL,                     -- 'text'|'url'|'image'|'pdf'|'doc'|'video'|'audio'
    source_ref     text NOT NULL DEFAULT '',          -- url / filename / chat message id
    blob_id        text NOT NULL DEFAULT '',          -- stored bytes, if any (candidate asset)
    extracted_text text NOT NULL DEFAULT '',          -- THE COMMON FORM the synthesis agent reads
    media_kind     text NOT NULL DEFAULT '',          -- if sendable: 'image'|'video'|'document'|'audio'
    status         text NOT NULL DEFAULT 'pending',   -- 'pending'|'extracting'|'ready'|'built'|'needs_human'|'failed'
    extraction     jsonb NOT NULL DEFAULT '{}'::jsonb,-- { method, model, confidence, error }
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ai_materials_snapshot_idx ON xchats.ai_materials(snapshot_id);

-- ai_builder_requests — the popup / human-in-the-loop queue, keyed to the DRAFT.
CREATE TABLE IF NOT EXISTS xchats.ai_builder_requests (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    material_id uuid REFERENCES xchats.ai_materials(id) ON DELETE SET NULL,
    req_type    text NOT NULL,                         -- 'describe_media'|'confirm_value'|'choose_topic'|'resolve_duplicate'|'comment'|...
    prompt      text NOT NULL DEFAULT '',
    context     jsonb NOT NULL DEFAULT '{}'::jsonb,    -- thumbnail ref / detected value / candidate topics
    target      jsonb NOT NULL DEFAULT '{}'::jsonb,    -- which draft row it resolves into
    state       text NOT NULL DEFAULT 'pending',       -- 'pending'|'resolved'|'dismissed'
    resolution  jsonb,                                 -- the operator's answer (becomes the row mutation)
    created_at  timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz
);
CREATE INDEX IF NOT EXISTS ai_builder_requests_snapshot_idx ON xchats.ai_builder_requests(snapshot_id, state);
