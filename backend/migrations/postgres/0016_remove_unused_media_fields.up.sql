-- 0016_remove_unused_media_fields — remove four media categories that are no
-- longer part of the product surface. Draft values are scrubbed before the
-- live columns are dropped so stale drafts cannot reintroduce the fields.
--
-- topics/products are rewritten only when they actually hold a JSON array. A
-- nil Go slice marshals to JSON `null` rather than `[]`, and `->` returns that
-- as a jsonb scalar rather than SQL NULL, so COALESCE does not catch it and
-- jsonb_array_elements fails with "cannot extract elements from a scalar"
-- (SQLSTATE 22023). Anything that is not an array has no members to scrub, so
-- it is left exactly as it is.
SET search_path = xchats, public;

UPDATE xchats.kbd_draft AS d
SET draft = d.draft
    || CASE
           WHEN jsonb_typeof(d.draft -> 'topics') = 'array'
           THEN jsonb_build_object('topics', COALESCE((
               SELECT jsonb_agg(topic.value - 'narration_audio_files' ORDER BY topic.ordinality)
               FROM jsonb_array_elements(d.draft -> 'topics')
                   WITH ORDINALITY AS topic(value, ordinality)
           ), '[]'::jsonb))
           ELSE '{}'::jsonb
       END
    || CASE
           WHEN jsonb_typeof(d.draft -> 'products') = 'array'
           THEN jsonb_build_object('products', COALESCE((
               SELECT jsonb_agg(
                   product.value - ARRAY[
                       'audio_description_files',
                       'manual_documents',
                       'specification_documents'
                   ]
                   ORDER BY product.ordinality
               )
               FROM jsonb_array_elements(d.draft -> 'products')
                   WITH ORDINALITY AS product(value, ordinality)
           ), '[]'::jsonb))
           ELSE '{}'::jsonb
       END;

ALTER TABLE xchats.ai_topics
    DROP COLUMN narration_audio_files;

ALTER TABLE xchats.ai_products
    DROP COLUMN audio_description_files,
    DROP COLUMN manual_documents,
    DROP COLUMN specification_documents;
