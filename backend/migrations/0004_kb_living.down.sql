-- Reverse 0004_kb_living. Children first (FK order), then re-lay the 0003 shape
-- so a rollback leaves the DB migratable forward again.
SET search_path = xchats, public;

DROP TABLE IF EXISTS xchats.ai_audit_log;
DROP TABLE IF EXISTS xchats.kbd_requests;
DROP TABLE IF EXISTS xchats.kbd_materials;
DROP TABLE IF EXISTS xchats.kbd_draft;
DROP TABLE IF EXISTS xchats.ai_values;
DROP TABLE IF EXISTS xchats.ai_assets;
DROP TABLE IF EXISTS xchats.ai_topics;
DROP TABLE IF EXISTS xchats.ai_assistants;

-- Re-create the 0003 (snapshot_id-keyed) shape.
CREATE TABLE IF NOT EXISTS xchats.ai_snapshots (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    version         int  NOT NULL DEFAULT 0,
    snapshot_state  text NOT NULL DEFAULT 'draft',
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
CREATE UNIQUE INDEX IF NOT EXISTS ai_snapshots_one_draft_uq
    ON xchats.ai_snapshots(organization_id)
    WHERE snapshot_state = 'draft';

CREATE TABLE IF NOT EXISTS xchats.ai_topics (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    slug        text NOT NULL,
    lang        text NOT NULL DEFAULT 'ru',
    title       text NOT NULL DEFAULT '',
    keywords    text NOT NULL DEFAULT '',
    body_md     text NOT NULL DEFAULT '',
    review_state text NOT NULL DEFAULT 'approved',
    provenance  jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (snapshot_id, slug)
);

CREATE TABLE IF NOT EXISTS xchats.ai_assets (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    ref         text NOT NULL,
    asset_kind  text NOT NULL DEFAULT 'image',
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

CREATE TABLE IF NOT EXISTS xchats.ai_values (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    token       text NOT NULL,
    lang        text NOT NULL DEFAULT '*',
    value_text  text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    review_state text NOT NULL DEFAULT 'approved',
    provenance  jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (snapshot_id, token, lang)
);

CREATE TABLE IF NOT EXISTS xchats.ai_materials (
    id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id    uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    source_type    text NOT NULL,
    source_ref     text NOT NULL DEFAULT '',
    blob_id        text NOT NULL DEFAULT '',
    extracted_text text NOT NULL DEFAULT '',
    media_kind     text NOT NULL DEFAULT '',
    status         text NOT NULL DEFAULT 'pending',
    extraction     jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ai_materials_snapshot_idx ON xchats.ai_materials(snapshot_id);

CREATE TABLE IF NOT EXISTS xchats.ai_builder_requests (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    material_id uuid REFERENCES xchats.ai_materials(id) ON DELETE SET NULL,
    req_type    text NOT NULL,
    prompt      text NOT NULL DEFAULT '',
    context     jsonb NOT NULL DEFAULT '{}'::jsonb,
    target      jsonb NOT NULL DEFAULT '{}'::jsonb,
    state       text NOT NULL DEFAULT 'pending',
    resolution  jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz
);
CREATE INDEX IF NOT EXISTS ai_builder_requests_snapshot_idx ON xchats.ai_builder_requests(snapshot_id, state);
