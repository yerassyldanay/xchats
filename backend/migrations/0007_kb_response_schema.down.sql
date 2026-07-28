SET search_path = xchats, public;

ALTER TABLE xchats.ai_drafts
    DROP COLUMN IF EXISTS reply_language;

ALTER TABLE xchats.ai_policies
    DROP COLUMN IF EXISTS outside_zones_note;

ALTER TABLE xchats.ai_tariffs
    DROP CONSTRAINT IF EXISTS ai_tariffs_status_check;
ALTER TABLE xchats.ai_products
    DROP CONSTRAINT IF EXISTS ai_products_status_check;
ALTER TABLE xchats.ai_products
    DROP COLUMN IF EXISTS in_stock;

ALTER TABLE xchats.wa_accounts
    DROP CONSTRAINT IF EXISTS wa_accounts_channel_check;
ALTER TABLE xchats.wa_accounts
    DROP COLUMN IF EXISTS channel;

DROP TABLE IF EXISTS xchats.ai_delivery_zones;
