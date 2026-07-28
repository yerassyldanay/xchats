-- 0010_drop_topic_keywords (down) — re-adds the column, but the original
-- values are NOT recoverable (a DROP COLUMN is destructive; Postgres does not
-- retain dropped column data). Every topic comes back with keywords = ''.
-- This file is never run automatically (RunMigrations only ever applies
-- *.up.sql — see internal/store/migrate.go); it exists for manual rollback
-- only, matching 0009's down.sql precedent.
SET search_path = xchats, public;

ALTER TABLE xchats.ai_topics ADD COLUMN keywords text NOT NULL DEFAULT '';
