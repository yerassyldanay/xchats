-- 0009_remove_kb_response_demo_data (down) — deliberately does NOT restore
-- the deleted demo rows: they were exact fixed-UUID application data, not
-- schema, and this file is never run automatically (RunMigrations only ever
-- applies *.up.sql — see internal/store/migrate.go). Re-load the demo
-- dataset on demand instead:
--
--   go run ./cmd/xchats kb-load -file testdata/demo_kb.json
--
-- (add -remove to delete those rows again).
SET search_path = xchats, public;

DO $$ BEGIN
    RAISE NOTICE '0009_remove_kb_response_demo_data (down): demo rows are not restored by migration — re-load with: go run ./cmd/xchats kb-load -file testdata/demo_kb.json';
END $$;
