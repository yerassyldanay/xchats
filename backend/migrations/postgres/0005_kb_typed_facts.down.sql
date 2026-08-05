-- Reverse 0005_kb_typed_facts: drop the typed fact tables and restore the generic
-- ai_values bag (0004 shape). Content is not recovered — the seed re-populates on
-- the next boot (SeedLiveIfEmpty), on whichever fact model is then in force.
SET search_path = xchats, public;

DROP TABLE IF EXISTS xchats.ai_tariffs;
DROP TABLE IF EXISTS xchats.ai_products;
DROP TABLE IF EXISTS xchats.ai_contacts;

CREATE TABLE IF NOT EXISTS xchats.ai_values (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    token           text NOT NULL,
    lang            text NOT NULL DEFAULT '*',
    value_text      text NOT NULL DEFAULT '',
    description     text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, token, lang)
);
