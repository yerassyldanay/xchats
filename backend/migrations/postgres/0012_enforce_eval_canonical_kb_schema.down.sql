-- Manual-rollback-only — the migration runner (store.RunMigrations) never
-- applies .down.sql files automatically (see 0009/0010's own .down.sql for
-- the established convention). Run this by hand, with psql, against a
-- specific database if you need to revert 0012.
--
-- NOT FULLY REVERSIBLE: the per-language ('kk') rows 0012 deleted are gone —
-- this only recreates the compatibility SHAPE (columns, defaults, tables),
-- not their content. ai_assets/ai_draft_assets are recreated empty. Any
-- kbstore/httpapi code written after 0012 landed (which writes canonical
-- columns only) will not populate the legacy columns this restores.

SET search_path = xchats, public;

ALTER TABLE xchats.ai_products ALTER COLUMN in_stock SET DEFAULT true;
ALTER TABLE xchats.ai_delivery_zones ALTER COLUMN delivery_available SET DEFAULT true;

ALTER TABLE xchats.ai_products ADD COLUMN data jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE xchats.ai_products ADD COLUMN status text NOT NULL DEFAULT 'active';
ALTER TABLE xchats.ai_products ADD COLUMN availability text NOT NULL DEFAULT '';
ALTER TABLE xchats.ai_products ADD CONSTRAINT ai_products_status_check CHECK (status = ANY (ARRAY['active','inactive']));

ALTER TABLE xchats.ai_tariffs ADD COLUMN data jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE xchats.ai_tariffs ADD COLUMN status text NOT NULL DEFAULT 'active';
ALTER TABLE xchats.ai_tariffs ADD CONSTRAINT ai_tariffs_status_check CHECK (status = ANY (ARRAY['active','inactive']));

ALTER TABLE xchats.ai_contacts ADD COLUMN legal text NOT NULL DEFAULT '';
ALTER TABLE xchats.ai_contacts ADD COLUMN slug text NOT NULL DEFAULT 'support';
ALTER TABLE xchats.ai_policies ADD COLUMN slug text NOT NULL DEFAULT 'main';
ALTER TABLE xchats.ai_policies ADD COLUMN delivery_time text NOT NULL DEFAULT '';
ALTER TABLE xchats.ai_policies ADD COLUMN return_period text NOT NULL DEFAULT '';

ALTER TABLE xchats.ai_delivery_zones ADD COLUMN status text NOT NULL DEFAULT 'active';
ALTER TABLE xchats.ai_delivery_zones ADD CONSTRAINT ai_delivery_zones_status_check CHECK (status = ANY (ARRAY['active','inactive']));
UPDATE xchats.ai_delivery_zones SET status = sales_status;

ALTER TABLE xchats.ai_topics ADD COLUMN lang text NOT NULL DEFAULT 'ru';

ALTER TABLE xchats.ai_products ADD COLUMN lang text NOT NULL DEFAULT 'ru';
ALTER TABLE xchats.ai_products DROP CONSTRAINT ai_products_organization_id_ref_key;
ALTER TABLE xchats.ai_products ADD CONSTRAINT ai_products_organization_id_ref_lang_key UNIQUE (organization_id, ref, lang);

ALTER TABLE xchats.ai_tariffs ADD COLUMN lang text NOT NULL DEFAULT 'ru';
ALTER TABLE xchats.ai_tariffs DROP CONSTRAINT ai_tariffs_organization_id_ref_key;
ALTER TABLE xchats.ai_tariffs ADD CONSTRAINT ai_tariffs_organization_id_ref_lang_key UNIQUE (organization_id, ref, lang);

ALTER TABLE xchats.ai_contacts ADD COLUMN lang text NOT NULL DEFAULT '*';
ALTER TABLE xchats.ai_contacts DROP CONSTRAINT ai_contacts_organization_id_key;
ALTER TABLE xchats.ai_contacts ADD CONSTRAINT ai_contacts_organization_id_lang_key UNIQUE (organization_id, lang);

ALTER TABLE xchats.ai_policies ADD COLUMN lang text NOT NULL DEFAULT '*';
ALTER TABLE xchats.ai_policies DROP CONSTRAINT ai_policies_organization_id_key;
ALTER TABLE xchats.ai_policies ADD CONSTRAINT ai_policies_organization_id_lang_key UNIQUE (organization_id, lang);

CREATE TABLE IF NOT EXISTS xchats.ai_assets (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    ref             text NOT NULL,
    asset_kind      text NOT NULL DEFAULT 'image',
    owner_kind      text NOT NULL DEFAULT '',
    owner_ref       text NOT NULL DEFAULT '',
    title           text NOT NULL DEFAULT '',
    description     text NOT NULL DEFAULT '',
    asset_url       text NOT NULL DEFAULT '',
    lang            text NOT NULL DEFAULT 'ru',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, ref)
);

CREATE TABLE IF NOT EXISTS xchats.ai_draft_assets (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    draft_id   uuid NOT NULL REFERENCES xchats.ai_drafts(id) ON DELETE CASCADE,
    asset_ref  text NOT NULL DEFAULT '',
    media_kind text NOT NULL DEFAULT '',
    media_url  text NOT NULL DEFAULT '',
    ordinal    int NOT NULL,
    UNIQUE (draft_id, ordinal)
);
