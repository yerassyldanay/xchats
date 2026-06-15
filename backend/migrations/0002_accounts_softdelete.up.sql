-- Build 1: soft-delete for WhatsApp accounts. A "clean" deletes the Evolution
-- instance and sets deleted_at; re-adding the same number (same uuidv5(owner_jid))
-- clears it, reviving the row with its chats/messages intact.
SET search_path = xchats, public;

ALTER TABLE xchats.wa_accounts ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

-- Listing the org's live accounts (non-deleted) and the multi-account inbox join
-- both filter on organization_id + deleted_at.
CREATE INDEX IF NOT EXISTS wa_accounts_org_idx
    ON xchats.wa_accounts(organization_id)
    WHERE deleted_at IS NULL;
