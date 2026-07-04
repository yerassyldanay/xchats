-- 0004_kb_living — migrates the KB to the plan/9,12,13,14,15 target model.
--
-- Three prefix groups (ai_ live KB / kbd_ draft+staging / rp_ suggestions — 15
-- Decision 5); organization_id as the DIRECT key on every KB table, retiring the
-- snapshot_id indirection (15 Decision 1); a single draft jsonb blob (kbd_draft)
-- replacing the versioned-snapshot clone/publish/rollback lifecycle (14/15 Decision
-- 3): live tables hold LIVE ROWS ONLY — no review_state, no provenance, no
-- drafted_at. An entity is a pending "Черновик" iff it is present in kbd_draft;
-- approve materializes it into the live table (15 Decision 4).
--
-- ai_values is KEPT this migration as a re-keyed BRIDGE — typed ai_tariffs /
-- ai_products / ai_contacts land in a later migration once the runtime brain's
-- {{table.slug.field}} token grammar is ready (the split de-risks the lifecycle
-- rewrite from the facts rewrite; see the migration-plan record). ai_assets swaps
-- topic_slug for the polymorphic owner_kind/owner_ref (doc 9).
--
-- v1 has a single seeded org and no operator-curated content of lasting value yet,
-- so this is a clean drop+recreate + reseed on boot (SeedLiveIfEmpty), NOT a data
-- migration: any content curated under the old ai_snapshots/review_state model is
-- intentionally dropped, not carried forward.
--
-- Naming note (unchanged from 0003): the RUNTIME suggestion store stays
-- ai_drafts/ai_draft_assets (migration 0001); its rename to rp_suggestions (15
-- Decision 6) is a separate, later concern — out of scope here.

SET search_path = xchats, public;

DROP TABLE IF EXISTS xchats.ai_builder_requests;
DROP TABLE IF EXISTS xchats.ai_materials;
DROP TABLE IF EXISTS xchats.ai_values;
DROP TABLE IF EXISTS xchats.ai_assets;
DROP TABLE IF EXISTS xchats.ai_topics;
DROP TABLE IF EXISTS xchats.ai_snapshots;

-- ai_assistants — ONE row per org: the assistant config. Not a snapshot, not
-- versioned (renamed from ai_snapshots — 15 Decision 1).
CREATE TABLE xchats.ai_assistants (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    persona         text NOT NULL DEFAULT '',
    mission         text NOT NULL DEFAULT '',
    guardrails      text NOT NULL DEFAULT '',
    language_policy text NOT NULL DEFAULT '',
    reply_max_words int  NOT NULL DEFAULT 120,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id)
);

-- ai_topics — the Knowledge lane. Live rows only; body_md is pure prose (14
-- Decision 3) — no digits, no fact tokens.
CREATE TABLE xchats.ai_topics (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    slug            text NOT NULL,
    lang            text NOT NULL DEFAULT 'ru',
    title           text NOT NULL DEFAULT '',
    keywords        text NOT NULL DEFAULT '',
    body_md         text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, slug)
);

-- ai_assets — the media catalog; attaches to ANY entity via the polymorphic
-- (owner_kind, owner_ref) pair (doc 9) — replaces the old single-purpose
-- topic_slug column.
CREATE TABLE xchats.ai_assets (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    ref             text NOT NULL,                   -- stable id the model selects on (also the blob id)
    asset_kind      text NOT NULL DEFAULT 'image',    -- 'image'|'video'|'document'|'audio'
    owner_kind      text NOT NULL DEFAULT '',         -- 'topic'|'product'|'tariff'|'' (unattached)
    owner_ref       text NOT NULL DEFAULT '',         -- the owner's ref/slug
    title           text NOT NULL DEFAULT '',
    description     text NOT NULL DEFAULT '',
    asset_url       text NOT NULL DEFAULT '',
    lang            text NOT NULL DEFAULT 'ru',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, ref)
);

-- ai_values — the confirmed (token, lang) → text fact bag. BRIDGE table: kept in
-- this migration re-keyed to organization_id; superseded by typed ai_tariffs /
-- ai_products / ai_contacts in a later migration (13 Decision 1).
CREATE TABLE xchats.ai_values (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    token           text NOT NULL,                    -- namespace.key
    lang            text NOT NULL DEFAULT '*',         -- 'ru'|'kk'|'*'
    value_text      text NOT NULL DEFAULT '',          -- confirmed value, any unit, substituted verbatim
    description     text NOT NULL DEFAULT '',          -- human-only
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, token, lang)
);

-- kbd_draft — the WHOLE pending KB as one jsonb document, one row per org (15
-- Decision 3). The playground reads/writes this blob; the brain NEVER touches it.
-- draft shape: { config, topics[], assets[], values[] (bridge), deletes[] }.
CREATE TABLE xchats.kbd_draft (
    organization_id uuid PRIMARY KEY REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    draft           jsonb NOT NULL DEFAULT '{}'::jsonb,
    base_version    bigint NOT NULL DEFAULT 0,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    updated_by      uuid REFERENCES xchats.users(id) ON DELETE SET NULL
);

-- kbd_materials — Stage-1 ⇄ Stage-2 ingest staging (was ai_materials); one row
-- per dropped input (the NormalizedMaterial contract). Playground-only.
CREATE TABLE xchats.kbd_materials (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    source_type     text NOT NULL,                     -- 'text'|'url'|'image'|'pdf'|'doc'|'video'|'audio'
    source_ref      text NOT NULL DEFAULT '',           -- url / filename / chat message id
    blob_id         text NOT NULL DEFAULT '',           -- stored bytes, if any (candidate asset)
    extracted_text  text NOT NULL DEFAULT '',           -- THE COMMON FORM the synthesis agent reads
    media_kind      text NOT NULL DEFAULT '',           -- if sendable: 'image'|'video'|'document'|'audio'
    status          text NOT NULL DEFAULT 'pending',    -- 'pending'|'extracting'|'ready'|'built'|'needs_human'|'failed'
    extraction      jsonb NOT NULL DEFAULT '{}'::jsonb, -- { method, model, confidence, error }
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX kbd_materials_org_idx ON xchats.kbd_materials(organization_id);

-- kbd_requests — the popup / human-in-the-loop queue (was ai_builder_requests).
-- Playground-only; keyed directly to the org (no draft-snapshot indirection).
CREATE TABLE xchats.kbd_requests (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    material_id     uuid REFERENCES xchats.kbd_materials(id) ON DELETE SET NULL,
    req_type        text NOT NULL,                      -- 'describe_media'|'confirm_value'|'choose_topic'|'resolve_duplicate'|'comment'|...
    prompt          text NOT NULL DEFAULT '',
    context         jsonb NOT NULL DEFAULT '{}'::jsonb,  -- thumbnail ref / detected value / candidate topics
    target          jsonb NOT NULL DEFAULT '{}'::jsonb,  -- which entity it resolves into
    state           text NOT NULL DEFAULT 'pending',     -- 'pending'|'resolved'|'dismissed'
    resolution      jsonb,                               -- the operator's answer (becomes the draft mutation)
    created_at      timestamptz NOT NULL DEFAULT now(),
    resolved_at     timestamptz
);
CREATE INDEX kbd_requests_org_idx ON xchats.kbd_requests(organization_id, state);

-- ai_audit_log — append-only KB-lifecycle history (approve/edit); who, when, why.
-- Defined now; populated once the Approve write path lands (doc 9 · v1: empty ok).
CREATE TABLE IF NOT EXISTS xchats.ai_audit_log (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    action          text NOT NULL,                       -- 'approve'|'edit'
    actor_user_id   uuid REFERENCES xchats.users(id) ON DELETE SET NULL,
    note            text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now()
);
