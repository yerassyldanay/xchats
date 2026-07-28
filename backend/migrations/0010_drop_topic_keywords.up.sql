-- 0010_drop_topic_keywords — drops xchats.ai_topics.keywords.
--
-- It was never used for search, matching, ranking, or filtering anywhere in
-- the codebase — every topic is always loaded and rendered unconditionally
-- (backend/internal/responsestore/kb.go's loadTopics has no filter/LIMIT).
-- Its only real effect was being pasted verbatim into the production system
-- prompt as a "keywords: a, b, c" line per topic
-- (backend/aiprompt/prompt.go's renderTopicBlocks) — synonym hints for the
-- model, not a mechanism. The value also came from a junk heuristic
-- (backend/internal/playground/builder.go's keywordsOf: first 8 words of
-- ≥4 runes from the instruction text), not curated input.
--
-- Frame untouched deliberately: renderTopicBlocks already omits the
-- "keywords:" line when the value is empty, so a topic with the column
-- gone renders byte-identical to one that already had blank keywords — the
-- embedded, SHA256-pinned frame (aiprompt/frame_test.go) and prompt_ref
-- shop-kb@v4 stay exactly as evaluated. No eval run needed for this change.
SET search_path = xchats, public;

ALTER TABLE xchats.ai_topics DROP COLUMN keywords;
