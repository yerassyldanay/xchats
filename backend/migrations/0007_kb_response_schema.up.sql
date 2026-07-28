-- 0007_kb_response_schema — final text-response KB schema for the multichannel
-- ResponseService. Adds only what backend/aiprompt's evaluated shop-kb-v1 prompt
-- needs beyond the existing typed fact tables, plus the channel discriminator
-- three later phases pivot on (channel-vs-conversation check, approval ->
-- sender-registry routing, the simulator account). Additive only — no existing
-- row is dropped or overwritten.
SET search_path = xchats, public;

-- ai_delivery_zones — matches aiprompt.DeliveryZone exactly (backend/aiprompt/types.go):
-- an explicit, seller-authored statement of where the org delivers (with cost and
-- days) or explicitly does not. Zones form a shallow containment hierarchy via
-- parent_ref (city -> region -> country); aiprompt.BuildCatalog enforces the
-- fail-closed consistency rules (delivery_available true requires cost+days set,
-- false requires both blank; parent_ref chains must resolve with no cycle) — this
-- table only has to store what BuildCatalog validates, not re-derive it.
CREATE TABLE IF NOT EXISTS xchats.ai_delivery_zones (
    id                 uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id    uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    ref                text NOT NULL,
    name               text NOT NULL DEFAULT '',
    zone_level         text NOT NULL,                   -- 'city'|'region'|'country'
    parent_ref         text NOT NULL DEFAULT '',         -- '' for a top-level zone
    delivery_available bool NOT NULL DEFAULT true,
    delivery_cost      text NOT NULL DEFAULT '',
    delivery_in_days   text NOT NULL DEFAULT '',
    notes              text NOT NULL DEFAULT '',
    status             text NOT NULL DEFAULT 'active',  -- 'active'|'inactive'
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, ref),
    CHECK (zone_level IN ('city', 'region', 'country')),
    CHECK (status IN ('active', 'inactive'))
);

-- wa_accounts.channel — the channel discriminator. No such column exists today;
-- every account seeded so far is a WhatsApp account, so the default backfills
-- every existing row correctly with no ambiguity.
ALTER TABLE xchats.wa_accounts
    ADD COLUMN IF NOT EXISTS channel text NOT NULL DEFAULT 'whatsapp';
ALTER TABLE xchats.wa_accounts
    DROP CONSTRAINT IF EXISTS wa_accounts_channel_check;
ALTER TABLE xchats.wa_accounts
    ADD CONSTRAINT wa_accounts_channel_check CHECK (channel IN ('whatsapp', 'simulator'));

-- ai_products.in_stock — the evaluated prompt's stock signal. availability (free-form
-- seller prose) stays; the AI-facing path stops reading it in favor of this boolean.
-- Every existing row becomes "in stock", INCLUDING rows whose availability text
-- disagrees — acceptable because those rows are brain-seeded dev data, and
-- migration 0008 sets the flag explicitly on its own demo rows.
ALTER TABLE xchats.ai_products
    ADD COLUMN IF NOT EXISTS in_stock boolean NOT NULL DEFAULT true;

-- status enum guards: no upsert path has ever written anything but the 'active'
-- default for ai_products/ai_tariffs (upsertProductRow/upsertTariffRow never set
-- the column), so every existing row already satisfies this.
ALTER TABLE xchats.ai_products
    DROP CONSTRAINT IF EXISTS ai_products_status_check;
ALTER TABLE xchats.ai_products
    ADD CONSTRAINT ai_products_status_check CHECK (status IN ('active', 'inactive'));
ALTER TABLE xchats.ai_tariffs
    DROP CONSTRAINT IF EXISTS ai_tariffs_status_check;
ALTER TABLE xchats.ai_tariffs
    ADD CONSTRAINT ai_tariffs_status_check CHECK (status IN ('active', 'inactive'));

-- ai_policies.outside_zones_note — the seller-authored closed-world refusal for a
-- direction matching no ai_delivery_zones row; required (non-blank) by
-- aiprompt.BuildCatalog whenever any zone row exists.
ALTER TABLE xchats.ai_policies
    ADD COLUMN IF NOT EXISTS outside_zones_note text NOT NULL DEFAULT '';

-- ai_drafts.reply_language — the validated reply_language ("ru"|"kk") the
-- ResponseService's engine returns alongside every persisted draft. Blank means
-- "not produced by the new engine" (legacy/holding drafts) — not itself a language.
ALTER TABLE xchats.ai_drafts
    ADD COLUMN IF NOT EXISTS reply_language text NOT NULL DEFAULT '';
