-- 0011_expand_eval_canonical_kb_schema (down) — drops exactly the columns
-- and indexes 0011.up.sql added. Purely additive forward migration, so this
-- is lossless for every column EXCEPT the three backfilled-from-legacy ones
-- (ai_products/ai_tariffs.sales_status, ai_delivery_zones.sales_status,
-- ai_contacts.legal_information, ai_policies.delivery_in_days/
-- return_period_in_days, kbd_materials.source_text/extraction_metadata/
-- processing_status) — those hold copies of data the legacy columns still
-- carry too, so nothing is actually lost by dropping the canonical copy.
--
-- This file is never run automatically (RunMigrations only ever applies
-- *.up.sql — see internal/store/migrate.go); it exists for manual rollback
-- only, matching the 0009/0010 down.sql precedent.
SET search_path = xchats, public;

DROP INDEX IF EXISTS kbd_materials_org_status_idx;
DROP INDEX IF EXISTS ai_contacts_location_map_image_idx;
DROP INDEX IF EXISTS ai_contacts_contact_card_image_idx;
DROP INDEX IF EXISTS ai_tariffs_featured_image_idx;
DROP INDEX IF EXISTS ai_products_featured_image_idx;
DROP INDEX IF EXISTS ai_topics_featured_image_idx;

ALTER TABLE xchats.kbd_materials
    DROP COLUMN IF EXISTS customer_visibility,
    DROP COLUMN IF EXISTS processing_status,
    DROP COLUMN IF EXISTS extraction_metadata,
    DROP COLUMN IF EXISTS transcript_text,
    DROP COLUMN IF EXISTS visual_summary,
    DROP COLUMN IF EXISTS sha256_checksum,
    DROP COLUMN IF EXISTS size_bytes,
    DROP COLUMN IF EXISTS mime_type,
    DROP COLUMN IF EXISTS filename,
    DROP COLUMN IF EXISTS storage_key,
    DROP COLUMN IF EXISTS storage_backend,
    DROP COLUMN IF EXISTS operator_note,
    DROP COLUMN IF EXISTS source_text;

ALTER TABLE xchats.ai_delivery_zones DROP COLUMN IF EXISTS sales_status;

ALTER TABLE xchats.ai_policies
    DROP COLUMN IF EXISTS commerce_policy_documents,
    DROP COLUMN IF EXISTS return_period_in_days,
    DROP COLUMN IF EXISTS delivery_in_days;

ALTER TABLE xchats.ai_contacts
    DROP COLUMN IF EXISTS company_legal_documents,
    DROP COLUMN IF EXISTS location_map_image,
    DROP COLUMN IF EXISTS contact_card_image,
    DROP COLUMN IF EXISTS legal_information;

ALTER TABLE xchats.ai_tariffs
    DROP COLUMN IF EXISTS terms_documents,
    DROP COLUMN IF EXISTS explainer_videos,
    DROP COLUMN IF EXISTS pricing_images,
    DROP COLUMN IF EXISTS featured_image,
    DROP COLUMN IF EXISTS sales_status;

ALTER TABLE xchats.ai_products
    DROP COLUMN IF EXISTS specification_documents,
    DROP COLUMN IF EXISTS guarantee_documents,
    DROP COLUMN IF EXISTS manual_documents,
    DROP COLUMN IF EXISTS certificate_documents,
    DROP COLUMN IF EXISTS audio_description_files,
    DROP COLUMN IF EXISTS demo_videos,
    DROP COLUMN IF EXISTS gallery_images,
    DROP COLUMN IF EXISTS featured_image,
    DROP COLUMN IF EXISTS sales_status;

ALTER TABLE xchats.ai_topics
    DROP COLUMN IF EXISTS reference_documents,
    DROP COLUMN IF EXISTS narration_audio_files,
    DROP COLUMN IF EXISTS explainer_videos,
    DROP COLUMN IF EXISTS illustration_images,
    DROP COLUMN IF EXISTS featured_image;
