-- 0011_expand_eval_canonical_kb_schema — additive: adds every canonical
-- column the eval fixture (evals/harness/internal/kbfixture, cross-checked
-- against backend/aiprompt's media registry — see
-- evals/harness/internal/schemamanifest) and plan/DECISIONS.md's "Canonical
-- knowledge-base schema" declare, while every legacy column and the two
-- legacy asset tables (ai_assets, ai_draft_assets) stay in place. This is
-- the "expand" half of an expand -> switch -> enforce rollout: 0011 can
-- coexist with code that still reads/writes the legacy columns; a later,
-- separate migration (0012) drops those columns and the two legacy tables
-- once every reader/writer has switched to the canonical names.
--
-- Verified against the live data (2026-07-29): every ai_products/ai_tariffs/
-- ai_delivery_zones.status is 'active', every ai_contacts.legal and
-- ai_policies.delivery_time/return_period is empty, and every kbd_materials
-- row is source_type='text' status='ready'. The backfills below still
-- compute from the actual stored value rather than assuming this, so a
-- database that DOES carry divergent data is migrated correctly instead of
-- silently losing it — see the DO block at the end for the one case (a
-- legacy file-like kbd_materials row) this migration deliberately does NOT
-- resolve on its own.
SET search_path = xchats, public;

-- ---------------------------------------------------------------------------
-- ai_topics — 5 semantic media columns (plan/DECISIONS.md ai_topics section)
-- ---------------------------------------------------------------------------
ALTER TABLE xchats.ai_topics
    ADD COLUMN featured_image        uuid REFERENCES xchats.kbd_materials(id),
    ADD COLUMN illustration_images   uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN explainer_videos      uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN narration_audio_files uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN reference_documents   uuid[] NOT NULL DEFAULT '{}';

-- ---------------------------------------------------------------------------
-- ai_products — sales_status (backfilled from status) + 8 media columns
-- ---------------------------------------------------------------------------
ALTER TABLE xchats.ai_products
    ADD COLUMN sales_status              text,
    ADD COLUMN featured_image            uuid REFERENCES xchats.kbd_materials(id),
    ADD COLUMN gallery_images            uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN demo_videos               uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN audio_description_files   uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN certificate_documents     uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN manual_documents          uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN guarantee_documents       uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN specification_documents   uuid[] NOT NULL DEFAULT '{}';
UPDATE xchats.ai_products SET sales_status = COALESCE(NULLIF(status, ''), 'active');
ALTER TABLE xchats.ai_products
    ALTER COLUMN sales_status SET NOT NULL,
    ALTER COLUMN sales_status SET DEFAULT 'active';

-- ---------------------------------------------------------------------------
-- ai_tariffs — sales_status (backfilled from status) + 4 media columns
-- ---------------------------------------------------------------------------
ALTER TABLE xchats.ai_tariffs
    ADD COLUMN sales_status     text,
    ADD COLUMN featured_image   uuid REFERENCES xchats.kbd_materials(id),
    ADD COLUMN pricing_images   uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN explainer_videos uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN terms_documents  uuid[] NOT NULL DEFAULT '{}';
UPDATE xchats.ai_tariffs SET sales_status = COALESCE(NULLIF(status, ''), 'active');
ALTER TABLE xchats.ai_tariffs
    ALTER COLUMN sales_status SET NOT NULL,
    ALTER COLUMN sales_status SET DEFAULT 'active';

-- ---------------------------------------------------------------------------
-- ai_contacts — legal_information (backfilled from legal) + 3 media columns
-- ---------------------------------------------------------------------------
ALTER TABLE xchats.ai_contacts
    ADD COLUMN legal_information       text,
    ADD COLUMN contact_card_image      uuid REFERENCES xchats.kbd_materials(id),
    ADD COLUMN location_map_image      uuid REFERENCES xchats.kbd_materials(id),
    ADD COLUMN company_legal_documents uuid[] NOT NULL DEFAULT '{}';
UPDATE xchats.ai_contacts SET legal_information = NULLIF(legal, '');

-- ---------------------------------------------------------------------------
-- ai_policies — delivery_in_days / return_period_in_days (backfilled) + 1
-- media column. delivery_cost/free_delivery_from/min_order/prepayment/
-- installment/warranty/outside_zones_note already carry their canonical
-- names — nothing to add for them.
-- ---------------------------------------------------------------------------
ALTER TABLE xchats.ai_policies
    ADD COLUMN delivery_in_days          text,
    ADD COLUMN return_period_in_days     text,
    ADD COLUMN commerce_policy_documents uuid[] NOT NULL DEFAULT '{}';
UPDATE xchats.ai_policies SET
    delivery_in_days = NULLIF(delivery_time, ''),
    return_period_in_days = NULLIF(return_period, '');

-- ---------------------------------------------------------------------------
-- ai_delivery_zones — sales_status (backfilled from status). delivery_cost/
-- delivery_in_days already carry their canonical names; no media columns
-- exist on this table in v1 (plan/DECISIONS.md).
-- ---------------------------------------------------------------------------
ALTER TABLE xchats.ai_delivery_zones ADD COLUMN sales_status text;
UPDATE xchats.ai_delivery_zones SET sales_status = COALESCE(NULLIF(status, ''), 'active');
ALTER TABLE xchats.ai_delivery_zones
    ALTER COLUMN sales_status SET NOT NULL,
    ALTER COLUMN sales_status SET DEFAULT 'active';

-- ---------------------------------------------------------------------------
-- kbd_materials — widen from the ingest-staging shape to the canonical
-- registry/authoring shape (plan/DECISIONS.md's kbd_materials section).
-- Legacy columns (blob_id, media_kind, status, extraction) stay for now —
-- the Step 9 code cutover stops writing them; migration 0012 drops them.
-- ---------------------------------------------------------------------------
ALTER TABLE xchats.kbd_materials
    ADD COLUMN source_text         text,
    ADD COLUMN operator_note       text,
    ADD COLUMN storage_backend     text,
    ADD COLUMN storage_key         text,
    ADD COLUMN filename            text,
    ADD COLUMN mime_type           text,
    ADD COLUMN size_bytes          bigint,
    ADD COLUMN sha256_checksum     text,
    ADD COLUMN visual_summary      text,
    ADD COLUMN transcript_text     text,
    ADD COLUMN extraction_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN processing_status   text,
    ADD COLUMN customer_visibility text;

-- source_text: preserve existing text-material content (previously the only
-- place it landed was extracted_text) without disturbing extracted_text.
UPDATE xchats.kbd_materials SET source_text = extracted_text WHERE source_type = 'text';

-- extraction_metadata: carry over the existing jsonb blob verbatim.
UPDATE xchats.kbd_materials SET extraction_metadata = COALESCE(extraction, '{}'::jsonb);

-- processing_status: map the legacy status vocabulary to the canonical one.
-- extracting|built|needs_human|failed pass through unchanged — both
-- vocabularies share those four spellings exactly.
UPDATE xchats.kbd_materials SET processing_status = CASE status
    WHEN 'pending' THEN 'uploaded'
    WHEN 'ready'   THEN 'parsed'
    ELSE status
END;
ALTER TABLE xchats.kbd_materials
    ALTER COLUMN processing_status SET NOT NULL,
    ALTER COLUMN processing_status SET DEFAULT 'uploaded';

-- NOTE: source_type is NOT rewritten here — legacy file-like values
-- (image|pdf|doc|video|audio) stay as-is, because the OLD extractor still
-- switches on those exact values and does not understand the canonical
-- 'file'. Rewriting source_type, plus populating storage_backend/
-- storage_key/filename/mime_type/size_bytes/customer_visibility for
-- existing file rows, is the Step 9 cutover's job (an idempotent
-- reconciliation command), run AFTER old material writers are stopped. This
-- migration only warns if such a row exists — it must not silently corrupt
-- a live extraction pipeline by rewriting the value out from under it.
DO $$
DECLARE legacy_file_count int;
BEGIN
    SELECT count(*) INTO legacy_file_count FROM xchats.kbd_materials
        WHERE source_type IN ('image', 'pdf', 'doc', 'video', 'audio');
    IF legacy_file_count > 0 THEN
        RAISE NOTICE 'kbd_materials: % row(s) still use a legacy file-like source_type; run the Step 9 reconciliation command before enforcing (migration 0012)', legacy_file_count;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- Indexes — the singular-media FK columns and kbd_materials' documented
-- (organization_id, processing_status) access path (plan/database-schema.md
-- "Required integrity and indexes").
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS ai_topics_featured_image_idx ON xchats.ai_topics(featured_image);
CREATE INDEX IF NOT EXISTS ai_products_featured_image_idx ON xchats.ai_products(featured_image);
CREATE INDEX IF NOT EXISTS ai_tariffs_featured_image_idx ON xchats.ai_tariffs(featured_image);
CREATE INDEX IF NOT EXISTS ai_contacts_contact_card_image_idx ON xchats.ai_contacts(contact_card_image);
CREATE INDEX IF NOT EXISTS ai_contacts_location_map_image_idx ON xchats.ai_contacts(location_map_image);
CREATE INDEX IF NOT EXISTS kbd_materials_org_status_idx ON xchats.kbd_materials(organization_id, processing_status);
