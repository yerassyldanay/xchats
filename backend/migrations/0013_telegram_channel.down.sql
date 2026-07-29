-- Down for 0013_telegram_channel. Dev-grade: restoring ai_drafts' FKs into the
-- wa_* tables FAILS if any Telegram draft rows exist (their chat_id points at
-- tg_chats), which is the honest outcome — delete those rows first if you
-- really mean to roll back a database that already served Telegram traffic.
SET search_path = xchats, public;

DROP VIEW IF EXISTS xchats.inbox_message_media_v;
DROP VIEW IF EXISTS xchats.inbox_messages_v;
DROP VIEW IF EXISTS xchats.inbox_chats_v;
DROP VIEW IF EXISTS xchats.inbox_accounts_v;

DROP TABLE IF EXISTS xchats.tg_message_media;
DROP TABLE IF EXISTS xchats.tg_messages;
DROP TABLE IF EXISTS xchats.tg_chats;
DROP TABLE IF EXISTS xchats.tg_contacts;
DROP TABLE IF EXISTS xchats.tg_credentials;
DROP TABLE IF EXISTS xchats.tg_accounts;

DROP INDEX IF EXISTS xchats.ai_drafts_pending_uq;
CREATE UNIQUE INDEX ai_drafts_pending_uq
    ON xchats.ai_drafts(chat_id, option_ordinal)
    WHERE draft_state = 'suggested';

ALTER TABLE xchats.ai_drafts
    DROP COLUMN IF EXISTS channel,
    ADD CONSTRAINT ai_drafts_chat_id_fkey
        FOREIGN KEY (chat_id) REFERENCES xchats.wa_chats(id) ON DELETE CASCADE,
    ADD CONSTRAINT ai_drafts_trigger_message_id_fkey
        FOREIGN KEY (trigger_message_id) REFERENCES xchats.wa_messages(id) ON DELETE SET NULL,
    ADD CONSTRAINT ai_drafts_sent_message_id_fkey
        FOREIGN KEY (sent_message_id) REFERENCES xchats.wa_messages(id) ON DELETE SET NULL;
