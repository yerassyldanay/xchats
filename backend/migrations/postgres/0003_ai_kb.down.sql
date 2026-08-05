-- Reverse 0003_ai_kb. Children first (FK order), then the snapshot root.
SET search_path = xchats, public;

DROP TABLE IF EXISTS xchats.ai_builder_requests;
DROP TABLE IF EXISTS xchats.ai_materials;
DROP TABLE IF EXISTS xchats.ai_values;
DROP TABLE IF EXISTS xchats.ai_assets;
DROP TABLE IF EXISTS xchats.ai_topics;
DROP TABLE IF EXISTS xchats.ai_snapshots;
