-- Deletes only the fixed UUIDs 0008 may have inserted. Safe no-op wherever 0008
-- itself no-opped (no organization existed, or the zone guard skipped) — a
-- DELETE against a row that was never inserted simply matches nothing.
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
