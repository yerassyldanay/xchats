-- Restores the removed schema shape with empty arrays. Values deleted by the
-- up migration cannot be reconstructed.
SET search_path = xchats, public;

ALTER TABLE xchats.ai_topics
    ADD COLUMN narration_audio_files uuid[] NOT NULL DEFAULT '{}';

ALTER TABLE xchats.ai_products
    ADD COLUMN audio_description_files uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN manual_documents uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN specification_documents uuid[] NOT NULL DEFAULT '{}';

UPDATE xchats.kbd_draft AS d
SET draft = jsonb_set(
    jsonb_set(
        d.draft,
        '{topics}',
        COALESCE((
            SELECT jsonb_agg(
                jsonb_set(topic.value, '{narration_audio_files}', '[]'::jsonb, true)
                ORDER BY topic.ordinality
            )
            FROM jsonb_array_elements(COALESCE(d.draft->'topics', '[]'::jsonb))
                WITH ORDINALITY AS topic(value, ordinality)
        ), '[]'::jsonb),
        true
    ),
    '{products}',
    COALESCE((
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
        FROM jsonb_array_elements(COALESCE(d.draft->'products', '[]'::jsonb))
            WITH ORDINALITY AS product(value, ordinality)
    ), '[]'::jsonb),
    true
);
