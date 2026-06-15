SET search_path = xchats, public;
DROP INDEX IF EXISTS xchats.wa_accounts_org_idx;
ALTER TABLE xchats.wa_accounts DROP COLUMN IF EXISTS deleted_at;
