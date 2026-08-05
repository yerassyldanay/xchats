-- 0005_kb_typed_facts — replaces the generic ai_values bag with typed FACT tables
-- (15 Decision 6, 13 Decision 1): every exact fact is a CONCRETE COLUMN on a typed
-- entity row, quoted in replies only as a 3-part {{table.slug.field}} token. No
-- generic value store remains.
--
-- Facts lane = ai_tariffs (pricing plans) / ai_products (sellable items) /
-- ai_contacts (org support scalars). Language is a ROW (one row per (entity,lang),
-- '*' for language-neutral) — no per-language columns. Values stored verbatim WITH
-- units; code never reformats. Media still attaches polymorphically via
-- ai_assets(owner_kind='product'|'tariff', owner_ref=<ref>).
--
-- ACCEPTED LOSS (v1, single seeded org): ai_values is dropped, and ai_topics /
-- ai_assets are TRUNCATED — their old bodies carry 2-part {{namespace.key}} tokens
-- the new 3-part grammar can't resolve, so we reset the org's KB content and let
-- SeedLiveIfEmpty re-seed the pure-prose topics + typed fact rows on boot. The
-- kbd_draft blob and kbd_requests queue are reset too (they may hold stale value
-- entries / confirm_value targets). Any operator-curated content is discarded.

SET search_path = xchats, public;

DROP TABLE IF EXISTS xchats.ai_values;

TRUNCATE xchats.ai_topics;
TRUNCATE xchats.ai_assets;
TRUNCATE xchats.kbd_draft;
TRUNCATE xchats.kbd_requests;

-- ai_tariffs — pricing plans. Exact numbers (price/limit_text/fee) are verbatim
-- typed columns; the model quotes them as {{tariff.<ref>.<field>}}.
CREATE TABLE xchats.ai_tariffs (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    ref             text NOT NULL,                     -- stable handle: 'basic'|'growth'|...
    lang            text NOT NULL DEFAULT 'ru',        -- 'ru'|'kk'|'*' — language is a ROW
    name            text NOT NULL DEFAULT '',          -- verbatim per language ('Рост'/'Өсу')
    price           text NOT NULL DEFAULT '',          -- verbatim with units ('25 000 ₸/мес'); '' if fee-based
    limit_text      text NOT NULL DEFAULT '',          -- verbatim ('до 2 000 платежей/мес')
    fee             text NOT NULL DEFAULT '',          -- '1.5 % за транзакцию'; '' otherwise
    summary         text NOT NULL DEFAULT '',
    pricing_type    text NOT NULL DEFAULT 'fixed',     -- 'fixed'|'percentage'|'tiered'|'hybrid'
    advantages      text NOT NULL DEFAULT '',
    disadvantages   text NOT NULL DEFAULT '',
    data            jsonb NOT NULL DEFAULT '{}'::jsonb,-- descriptive prose only (conditions/tiers)
    status          text NOT NULL DEFAULT 'active',    -- 'active'|'inactive'
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, ref, lang)
);

-- ai_products — sellable items. price is a verbatim typed column, {{product.<ref>.price}}.
CREATE TABLE xchats.ai_products (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    ref             text NOT NULL,                     -- stable handle: 'nike-x'
    lang            text NOT NULL DEFAULT 'ru',        -- language is a ROW
    name            text NOT NULL DEFAULT '',
    price           text NOT NULL DEFAULT '',          -- verbatim WITH units ('25 000 ₸')
    description     text NOT NULL DEFAULT '',
    category        text NOT NULL DEFAULT '',
    data            jsonb NOT NULL DEFAULT '{}'::jsonb,-- sphere-specific attrs (size, color, …)
    status          text NOT NULL DEFAULT 'active',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, ref, lang)
);

-- ai_contacts — the org support scalars (singleton slug 'support'), one row per
-- language. Language-neutral fields (phone/e-mail/address) live on the '*' row;
-- language-bearing fields (callback_time) on the ru/kk rows. {{contact.support.<field>}}.
CREATE TABLE xchats.ai_contacts (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    slug            text NOT NULL DEFAULT 'support',   -- singleton — keeps the 3-part token grammar
    lang            text NOT NULL DEFAULT '*',         -- 'ru'|'kk'|'*'
    whatsapp        text NOT NULL DEFAULT '',
    email           text NOT NULL DEFAULT '',
    address         text NOT NULL DEFAULT '',
    legal           text NOT NULL DEFAULT '',
    callback_time   text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, lang)
);
