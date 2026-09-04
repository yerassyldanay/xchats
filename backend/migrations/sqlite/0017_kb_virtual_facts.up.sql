-- xchats SQLite schema, part 17: controlled "virtual fact columns" for
-- products, tariffs, and organization-wide tariff info (docs/DECISIONS.md
-- "Virtual fact columns" — see backend/aiprompt/facts.go for the token/
-- validation contract these columns feed).
--
-- additional_facts on all three tables holds a JSON array of
-- {ref, value, instruction} objects — seller-authored "virtual columns" that
-- become {{product.<ref>.<fact_ref>}} / {{tariff.<ref>.<fact_ref>}} /
-- {{tariff_info.main.<fact_ref>}} prompt tokens, validated and substituted
-- by aiprompt (never stored pre-rendered). json_valid is the same defense
-- every other JSON-blob column in this schema already carries.
--
-- availability_status REPLACES in_stock as the product availability source
-- of truth: four states (in_stock | preorder | on_demand | unavailable)
-- instead of a boolean, so a product can be visible-but-not-immediately-
-- shippable (preorder/on_demand) without lying that it is simply "in
-- stock". The backfill below derives it from the existing in_stock column
-- 1:1 (true -> in_stock, false -> unavailable) so no existing row's
-- model-visible behavior changes on this migration alone; in_stock is then
-- dropped so there is exactly one source of truth going forward — keeping
-- both would let them silently disagree (see kbstore/aiprompt: nothing
-- reads in_stock ever again after this file).
--
-- The kbd_draft JSON blob can hold a PENDING product edit shaped by the old
-- column (`"in_stock": true/false`, no "availability_status") staged before
-- this migration ran. The second UPDATE below rewrites every draft's
-- products array in place, entry by entry, the same true->in_stock /
-- false->unavailable mapping as the live backfill, so a pending edit is
-- never silently reinterpreted (or dropped) the next time the draft blob is
-- read as the new DraftProduct shape. Rewritten via SQLite's JSON1
-- functions since migrations here are plain SQL, applied only to rows whose
-- draft actually carries a products array (json_group_array's NULL-on-empty
-- result is coalesced back to '[]' so an org with a products:[] draft keeps
-- an empty array, not NULL).

ALTER TABLE ai_products ADD COLUMN brand TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_products ADD COLUMN advantages TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_products ADD COLUMN disadvantages TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_products ADD COLUMN best_for TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_products ADD COLUMN not_for TEXT NOT NULL DEFAULT '';
-- No CHECK on the enum itself — sales_status/pricing_type set the precedent
-- of validating closed-vocabulary TEXT columns in Go (kbstore.validateEnum),
-- not via a SQL CHECK, so the vocabulary can gain a value without a schema
-- migration.
ALTER TABLE ai_products ADD COLUMN availability_status TEXT NOT NULL DEFAULT 'in_stock';
ALTER TABLE ai_products ADD COLUMN availability_note TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_products ADD COLUMN installation_terms TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_products ADD COLUMN warranty_terms TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_products ADD COLUMN additional_facts TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(additional_facts));

UPDATE ai_products SET availability_status = CASE WHEN in_stock THEN 'in_stock' ELSE 'unavailable' END;

UPDATE kbd_draft
SET draft = json_set(
    draft,
    '$.products',
    (
        SELECT COALESCE(json_group_array(json(
            json_remove(
                json_set(
                    p.value,
                    '$.availability_status',
                    CASE
                        WHEN json_type(p.value, '$.in_stock') IS NULL THEN 'in_stock'
                        WHEN json_extract(p.value, '$.in_stock') THEN 'in_stock'
                        ELSE 'unavailable'
                    END
                ),
                '$.in_stock'
            )
        )), json('[]'))
        FROM json_each(kbd_draft.draft, '$.products') AS p
    )
)
WHERE json_valid(draft) AND json_type(draft, '$.products') = 'array';

ALTER TABLE ai_products DROP COLUMN in_stock;

ALTER TABLE ai_tariffs ADD COLUMN best_for TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_tariffs ADD COLUMN not_for TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_tariffs ADD COLUMN additional_facts TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(additional_facts));

-- ai_tariff_info is a singleton like ai_contacts/ai_policies (one row per
-- organization) holding only organization-wide tariff facts that do not
-- belong to any one tariff (e.g. a trial period shared by every plan) — a
-- tariff-specific fact still belongs in that tariff's own additional_facts.
CREATE TABLE ai_tariff_info (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id   TEXT NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    additional_facts  TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(additional_facts)),
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
