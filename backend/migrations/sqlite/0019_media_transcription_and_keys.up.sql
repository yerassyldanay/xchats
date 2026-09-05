-- xchats SQLite schema, part 19: audio transcript storage.
--
-- Adds a `transcript` column to all three media-attachment tables (the wa_*
-- gateway's message_media, tg_message_media, and the generic channel_*
-- core's channel_message_media) so a voice note's speech-to-text result is
-- persisted exactly once per attachment — worker.go's transcription step
-- runs after the bytes finish downloading and writes here, so a redelivery
-- or a page reload never re-runs the STT call. Empty string (not NULL)
-- means "not transcribed yet" (still pending, not audio, or STT is not
-- configured), matching every other optional TEXT column in this schema
-- (mimetype, filename, ...).
--
-- inbox_message_media_v already projects storage_key and download_status
-- (every arm has carried them since 0002/0011) — only the NEW `transcript`
-- column needs adding to the view; it must still be DROP+CREATE (see
-- 0011_channel_core.up.sql's own note: SQLite refuses to ALTER a view, and
-- refuses to drop a base-table column a live view depends on).
ALTER TABLE message_media ADD COLUMN transcript TEXT NOT NULL DEFAULT '';
ALTER TABLE tg_message_media ADD COLUMN transcript TEXT NOT NULL DEFAULT '';
ALTER TABLE channel_message_media ADD COLUMN transcript TEXT NOT NULL DEFAULT '';

DROP VIEW inbox_message_media_v;

CREATE VIEW inbox_message_media_v AS
SELECT mm.id,
    mm.message_id,
    a.channel,
    mm.media_type,
    mm.mimetype,
    mm.file_name AS filename,
    mm.file_size AS size,
    mm.storage_url AS storage_key,
    mm.download_status,
    mm.transcript,
    mm.created_at
FROM message_media mm
    JOIN wa_messages m ON m.id = mm.message_id
    JOIN wa_accounts a ON a.id = m.account_id
UNION ALL
SELECT tm.id,
    tm.message_id,
    'telegram' AS channel,
    tm.media_type,
    tm.mimetype,
    tm.filename,
    tm.size,
    tm.storage_key,
    tm.download_status,
    tm.transcript,
    tm.created_at
FROM tg_message_media tm
UNION ALL
SELECT cmm.id,
    cmm.message_id,
    ca.channel,
    cmm.media_type,
    cmm.mimetype,
    cmm.filename,
    cmm.size,
    cmm.storage_key,
    cmm.download_status,
    cmm.transcript,
    cmm.created_at
FROM channel_message_media cmm
    JOIN channel_messages m ON m.id = cmm.message_id
    JOIN channel_accounts ca ON ca.id = m.account_id;
