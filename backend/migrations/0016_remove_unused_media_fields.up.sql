-- 0016_remove_unused_media_fields — remove four media categories that are no
-- longer part of the product surface. Draft values are scrubbed before the
-- live columns are dropped so stale drafts cannot reintroduce the fields.
SET search_path = xchats, public;

UPDATE xchats.kbd_draft AS d
SET draft = jsonb_set(
    jsonb_set(
        d.draft,
        '{topics}',
        COALESCE((
            SELECT jsonb_agg(topic.value - 'narration_audio_files' ORDER BY topic.ordinality)
            FROM jsonb_array_elements(COALESCE(d.draft->'topics', '[]'::jsonb))
                WITH ORDINALITY AS topic(value, ordinality)
        ), '[]'::jsonb),
        true
    ),
    '{products}',
    COALESCE((
        SELECT jsonb_agg(
            product.value - ARRAY[
                'audio_description_files',
                'manual_documents',
                'specification_documents'
            ]
            ORDER BY product.ordinality
        )
        FROM jsonb_array_elements(COALESCE(d.draft->'products', '[]'::jsonb))
            WITH ORDINALITY AS product(value, ordinality)
    ), '[]'::jsonb),
    true
);

ALTER TABLE xchats.ai_topics
    DROP COLUMN narration_audio_files;

ALTER TABLE xchats.ai_products
    DROP COLUMN audio_description_files,
    DROP COLUMN manual_documents,
    DROP COLUMN specification_documents;
