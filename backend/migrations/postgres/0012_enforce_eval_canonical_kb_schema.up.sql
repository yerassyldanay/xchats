-- Enforce the eval-canonical KB schema (evals/harness/internal/kbfixture is the
-- source of truth — plan/database-schema.md): migration 0011 added every
-- canonical column additively; every kbstore/httpapi/frontend write path has
-- since switched over to canonical names/shapes (no more `lang` dimension on
-- ai_topics/ai_products/ai_tariffs, ai_contacts/ai_policies are true
-- singletons, and the two legacy media tables have no code path left that
-- writes or reads them — see plan/knowledge-base.md "PR 1: retire legacy
-- asset code"). This migration drops what production no longer reads:
-- lang, data, status, availability, legal, delivery_time, return_period, the
-- DEFAULT on the two booleans kbfixture models as *StrictBool (NOT NULL, no
-- default), and the two legacy asset tables. It is DESTRUCTIVE — see
-- 0012_enforce_eval_canonical_kb_schema.down.sql for the manual-rollback
-- caveats before running this beyond a scratch/dev database.

SET search_path = xchats, public;

-- ---------------------------------------------------------------------------
-- Guards — fail loudly instead of silently discarding data. The exploration
-- behind this migration found zero 'kk' rows and zero non-empty legacy
-- fields on the live dev database, but another environment must not find out
-- the hard way.
-- ---------------------------------------------------------------------------

DELETE FROM xchats.ai_topics WHERE lang = 'kk';
DELETE FROM xchats.ai_products WHERE lang = 'kk';
DELETE FROM xchats.ai_tariffs WHERE lang = 'kk';
DELETE FROM xchats.ai_contacts WHERE lang = 'kk';
DELETE FROM xchats.ai_policies WHERE lang = 'kk';

DO $$
DECLARE n int;
BEGIN
    SELECT count(*) INTO n FROM xchats.ai_topics WHERE lang <> 'ru';
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_topics has % row(s) with a language other than ru — resolve before dropping the lang column', n;
    END IF;
    SELECT count(*) INTO n FROM xchats.ai_products WHERE lang <> 'ru';
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_products has % row(s) with a language other than ru — resolve before dropping the lang column', n;
    END IF;
    SELECT count(*) INTO n FROM xchats.ai_tariffs WHERE lang <> 'ru';
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_tariffs has % row(s) with a language other than ru — resolve before dropping the lang column', n;
    END IF;
    SELECT count(*) INTO n FROM xchats.ai_contacts WHERE lang <> '*';
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_contacts has % row(s) with a language other than * — resolve before dropping the lang column', n;
    END IF;
    SELECT count(*) INTO n FROM xchats.ai_policies WHERE lang <> '*';
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_policies has % row(s) with a language other than * — resolve before dropping the lang column', n;
    END IF;

    SELECT count(*) INTO n FROM xchats.ai_products WHERE data <> '{}'::jsonb OR availability <> '';
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_products has % row(s) with non-empty data/availability — resolve before dropping those columns', n;
    END IF;
    SELECT count(*) INTO n FROM xchats.ai_tariffs WHERE data <> '{}'::jsonb;
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_tariffs has % row(s) with non-empty data — resolve before dropping that column', n;
    END IF;
    SELECT count(*) INTO n FROM xchats.ai_contacts WHERE legal <> '';
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_contacts has % row(s) with non-empty legacy legal (legal_information already holds the canonical value) — resolve before dropping', n;
    END IF;
    SELECT count(*) INTO n FROM xchats.ai_policies WHERE delivery_time <> '' OR return_period <> '';
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_policies has % row(s) with non-empty legacy delivery_time/return_period — resolve before dropping those columns', n;
    END IF;

    SELECT count(*) INTO n FROM xchats.ai_delivery_zones WHERE status <> sales_status;
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_delivery_zones has % row(s) where legacy status diverges from sales_status — resolve before dropping status', n;
    END IF;

    SELECT count(*) INTO n FROM xchats.ai_assets;
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_assets has % row(s) — cannot drop a non-empty table', n;
    END IF;
    SELECT count(*) INTO n FROM xchats.ai_draft_assets;
    IF n > 0 THEN
        RAISE EXCEPTION 'ai_draft_assets has % row(s) — cannot drop a non-empty table', n;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- Drop `lang` + re-point the unique keys that included it. ai_products/
-- ai_tariffs collapse from (organization_id, ref, lang) to (organization_id,
-- ref); ai_contacts/ai_policies become true singletons — (organization_id)
-- alone, since kbfixture models them as one row per org, not per language.
-- ADD CONSTRAINT below fails loudly if the guards above missed a duplicate.
-- ---------------------------------------------------------------------------

ALTER TABLE xchats.ai_topics DROP COLUMN lang;

ALTER TABLE xchats.ai_products DROP CONSTRAINT ai_products_organization_id_ref_lang_key;
ALTER TABLE xchats.ai_products DROP COLUMN lang;
ALTER TABLE xchats.ai_products ADD CONSTRAINT ai_products_organization_id_ref_key UNIQUE (organization_id, ref);

ALTER TABLE xchats.ai_tariffs DROP CONSTRAINT ai_tariffs_organization_id_ref_lang_key;
ALTER TABLE xchats.ai_tariffs DROP COLUMN lang;
ALTER TABLE xchats.ai_tariffs ADD CONSTRAINT ai_tariffs_organization_id_ref_key UNIQUE (organization_id, ref);

ALTER TABLE xchats.ai_contacts DROP CONSTRAINT ai_contacts_organization_id_lang_key;
ALTER TABLE xchats.ai_contacts DROP COLUMN lang;
ALTER TABLE xchats.ai_contacts ADD CONSTRAINT ai_contacts_organization_id_key UNIQUE (organization_id);

ALTER TABLE xchats.ai_policies DROP CONSTRAINT ai_policies_organization_id_lang_key;
ALTER TABLE xchats.ai_policies DROP COLUMN lang;
ALTER TABLE xchats.ai_policies ADD CONSTRAINT ai_policies_organization_id_key UNIQUE (organization_id);

-- ---------------------------------------------------------------------------
-- Drop the remaining forbidden legacy business columns (plan/database-
-- schema.md "Exact parity contract"). The two status CHECK constraints are
-- scoped to the column being dropped — drop them explicitly rather than rely
-- on an implicit CASCADE.
-- ---------------------------------------------------------------------------

ALTER TABLE xchats.ai_tariffs DROP CONSTRAINT IF EXISTS ai_tariffs_status_check;
ALTER TABLE xchats.ai_products DROP CONSTRAINT IF EXISTS ai_products_status_check;
ALTER TABLE xchats.ai_delivery_zones DROP CONSTRAINT IF EXISTS ai_delivery_zones_status_check;

ALTER TABLE xchats.ai_products DROP COLUMN data, DROP COLUMN status, DROP COLUMN availability;
ALTER TABLE xchats.ai_tariffs DROP COLUMN data, DROP COLUMN status;
ALTER TABLE xchats.ai_contacts DROP COLUMN legal;
-- slug ('support'/'main') is a redundant static marker now that these tables
-- are true singletons keyed by organization_id alone — kbstore has never
-- read it back, only ever writing the fixed literal (see
-- kbfixture.ContactSlug/PolicySlug, which are Go constants, not a column).
ALTER TABLE xchats.ai_contacts DROP COLUMN slug;
ALTER TABLE xchats.ai_policies DROP COLUMN slug;
ALTER TABLE xchats.ai_policies DROP COLUMN delivery_time, DROP COLUMN return_period;
-- ai_delivery_zones.status is the same superseded-by-sales_status legacy
-- column as products/tariffs' status — kbstore/zones.go has never read or
-- written it (only sales_status).
ALTER TABLE xchats.ai_delivery_zones DROP COLUMN status;

-- ---------------------------------------------------------------------------
-- kbfixture models in_stock/delivery_available as *StrictBool specifically so
-- an omitted key is REJECTED, not defaulted — every kbstore writer already
-- supplies the value explicitly, so dropping the schema DEFAULT only removes
-- a safety net no code relies on.
-- ---------------------------------------------------------------------------

ALTER TABLE xchats.ai_products ALTER COLUMN in_stock DROP DEFAULT;
ALTER TABLE xchats.ai_delivery_zones ALTER COLUMN delivery_available DROP DEFAULT;

-- ---------------------------------------------------------------------------
-- Retire the two legacy media tables. kb-preflight guarded these empty at
-- every PR 1 deploy; every write/read path (Playground's global asset editor,
-- draft-suggested-media) was already deleted in that PR. kbd_materials is now
-- the one media registry.
-- ---------------------------------------------------------------------------

DROP TABLE xchats.ai_draft_assets;
DROP TABLE xchats.ai_assets;
