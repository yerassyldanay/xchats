-- Restores the removed schema shape with empty arrays. Values deleted by the
-- up migration cannot be reconstructed.
--
-- As in the up migration, topics/products are rewritten only when they hold a
-- JSON array — a nil Go slice marshals to `null`, which `->` yields as a jsonb
-- scalar that COALESCE cannot catch and jsonb_array_elements rejects with
-- SQLSTATE 22023.
SET search_path = xchats, public;

ALTER TABLE xchats.ai_topics
    ADD COLUMN narration_audio_files uuid[] NOT NULL DEFAULT '{}';

ALTER TABLE xchats.ai_products
    ADD COLUMN audio_description_files uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN manual_documents uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN specification_documents uuid[] NOT NULL DEFAULT '{}';

UPDATE xchats.kbd_draft AS d
SET draft = d.draft
    || CASE
           WHEN jsonb_typeof(d.draft -> 'topics') = 'array'
           THEN jsonb_build_object('topics', COALESCE((
               SELECT jsonb_agg(
                   jsonb_set(topic.value, '{narration_audio_files}', '[]'::jsonb, true)
                   ORDER BY topic.ordinality
               )
               FROM jsonb_array_elements(d.draft -> 'topics')
                   WITH ORDINALITY AS topic(value, ordinality)
           ), '[]'::jsonb))
           ELSE '{}'::jsonb
       END
    || CASE
           WHEN jsonb_typeof(d.draft -> 'products') = 'array'
           THEN jsonb_build_object('products', COALESCE((
               SELECT jsonb_agg(
                   jsonb_set(
                       jsonb_set(
                           jsonb_set(product.value, '{audio_description_files}', '[]'::jsonb, true),
                           '{manual_documents}',
                           '[]'::jsonb,
                           true
                       ),
                       '{specification_documents}',
                       '[]'::jsonb,
                       true
                   )
                   ORDER BY product.ordinality
               )
               FROM jsonb_array_elements(d.draft -> 'products')
                   WITH ORDINALITY AS product(value, ordinality)
           ), '[]'::jsonb))
           ELSE '{}'::jsonb
       END;
