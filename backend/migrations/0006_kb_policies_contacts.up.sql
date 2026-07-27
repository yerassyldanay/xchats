-- 0006_kb_policies_contacts — widens the contact + product fact vocabulary and
-- adds a new commerce-policy fact table, per the "a new category of org fact
-- gets its own typed table" doctrine (9-database-schema.md). Additive only —
-- no truncation; existing orgs simply see the new columns/table start empty.

SET search_path = xchats, public;

ALTER TABLE xchats.ai_contacts
    ADD COLUMN IF NOT EXISTS working_hours text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS phone         text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS website       text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS instagram     text NOT NULL DEFAULT '';

ALTER TABLE xchats.ai_products
    ADD COLUMN IF NOT EXISTS availability text NOT NULL DEFAULT '';

-- ai_policies — org commerce-policy scalars (delivery/payment/returns terms), an
-- exact structural clone of ai_contacts: singleton slug 'main', one row per
-- language ('*' for language-neutral), verbatim text values quoted in replies
-- only as {{policy.main.<field>}} tokens (never digits/prose).
CREATE TABLE IF NOT EXISTS xchats.ai_policies (
    id                 uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id    uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    slug               text NOT NULL DEFAULT 'main',
    lang               text NOT NULL DEFAULT '*',
    delivery_cost      text NOT NULL DEFAULT '',
    delivery_time      text NOT NULL DEFAULT '',
    free_delivery_from text NOT NULL DEFAULT '',
    min_order          text NOT NULL DEFAULT '',
    prepayment         text NOT NULL DEFAULT '',
    installment        text NOT NULL DEFAULT '',
    return_period      text NOT NULL DEFAULT '',
    warranty           text NOT NULL DEFAULT '',
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, lang)
);
