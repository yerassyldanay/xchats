-- 0009_remove_kb_response_demo_data — removes the temporary demo KB data
-- 0008 inserted now that real KB administration exists (this PR's
-- /knowledge-base rewrite: the Промпт + Зоны доставки tabs, in_stock/
-- outside_zones_note wired up, kb-load for reloading a demo dataset on
-- demand). Deletes ONLY 0008's exact fixed UUIDs — the same set 0008's own
-- down.sql already encodes, replayed here as a forward migration so it runs
-- automatically as part of the normal migrate step (RunMigrations only ever
-- applies *.up.sql files; a .down.sql is manual-rollback-only and never
-- runs on its own).
--
-- Dependency-safe and idempotent: none of ai_assistants/ai_contacts/
-- ai_policies/ai_topics/ai_products/ai_tariffs/ai_delivery_zones carries a
-- foreign key to another of these tables (each only references
-- xchats.organizations), so there is no ordering constraint to respect, and a
-- DELETE against an id that was never inserted (0008 no-opped on a fresh
-- database with no organization yet) or is already gone (this file re-applied
-- by hand) simply matches zero rows — never an error.
SET search_path = xchats, public;

DELETE FROM xchats.ai_delivery_zones WHERE id IN (
    '00000000-0000-4000-9000-000000000d41',
    '00000000-0000-4000-9000-000000000d42',
    '00000000-0000-4000-9000-000000000d43'
);
DELETE FROM xchats.ai_tariffs WHERE id IN (
    '00000000-0000-4000-9000-000000000d31',
    '00000000-0000-4000-9000-000000000d32'
);
DELETE FROM xchats.ai_products WHERE id IN (
    '00000000-0000-4000-9000-000000000d21',
    '00000000-0000-4000-9000-000000000d22',
    '00000000-0000-4000-9000-000000000d23',
    '00000000-0000-4000-9000-000000000d24',
    '00000000-0000-4000-9000-000000000d25'
);
DELETE FROM xchats.ai_topics WHERE id IN (
    '00000000-0000-4000-9000-000000000d11',
    '00000000-0000-4000-9000-000000000d12',
    '00000000-0000-4000-9000-000000000d13'
);
DELETE FROM xchats.ai_policies WHERE id = '00000000-0000-4000-9000-000000000d03';
DELETE FROM xchats.ai_contacts WHERE id = '00000000-0000-4000-9000-000000000d02';
DELETE FROM xchats.ai_assistants WHERE id = '00000000-0000-4000-9000-000000000d01';
