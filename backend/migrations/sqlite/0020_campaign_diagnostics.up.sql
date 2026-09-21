-- xchats SQLite schema, part 20: campaign send-attempt traceability,
-- diagnostic events, and WhatsApp connection history.
--
-- Until now a campaign recipient's only persisted attempt history was
-- campaign_send_log (internal/store/campaigns.go's rate-limit ledger: one
-- row per attempt, sent/failed only, no error detail, pruned after 7 days —
-- see 0012_campaigns.up.sql's own doc comment) and campaign_recipients.
-- failure_reason (a single mutable TEXT column, overwritten by every new
-- attempt). Neither can answer "what happened on THIS attempt" once a later
-- attempt has overwritten it, or survive past the ledger's 7-day prune.
--
-- campaign_send_attempts is the append-only, non-pruned counterpart:
-- one row per attempt (like campaign_send_log), but carrying a structured
-- error_code + sanitized error_detail + retryable flag + timing, and never
-- deleted. It is deliberately a SEPARATE table from campaign_send_log rather
-- than an extension of it: campaign_send_log's 7-day retention is
-- load-bearing for the rolling rate-limit tiers (backend/campaign.
-- MaxTierWindowSeconds), so widening its retention to serve diagnostics
-- would silently let a wide custom tier undercount. organization_id and
-- account_id are denormalized here purely for indexed, join-free tenant-
-- scoped queries (the MCP diagnostics tools' hot path) — authorization
-- itself is always re-derived from campaign ownership first (orgCampaign),
-- never trusted from this column alone, mirroring every other denormalized
-- organization_id/account_id in this schema. chat_id/message_id carry no
-- foreign key, same either/or reasoning as campaign_recipients.chat_id/
-- message_id (a chat is a wa_chats/tg_chats/channel_chats row, a message a
-- wa_messages/tg_messages/channel_messages row).
--
-- No CHECK constrains error_code to a closed vocabulary — like migration
-- 0018's reason_code, it is validated in Go (internal/campaign's error
-- classifier) so the taxonomy can grow without a schema migration. outcome
-- DOES have a CHECK: unlike error_code, its four values are a fixed,
-- unlikely-to-grow state a query needs to group by reliably. 'unknown' is a
-- first-class outcome, not a subtype of 'failed': an ambiguous timeout or a
-- crash-interrupted send (internal/store.ReconcileStuckSending) is recorded
-- here as 'unknown' even though the recipient's own coarse status still
-- becomes 'failed' (preserving that column's existing, already-shipped
-- vocabulary — see this migration's own note below on why
-- campaign_recipients itself is intentionally untouched).
CREATE TABLE campaign_send_attempts (
    id                     TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id        TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    campaign_id            TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    campaign_recipient_id  TEXT NOT NULL REFERENCES campaign_recipients(id) ON DELETE CASCADE,
    account_id             TEXT NOT NULL,
    attempt_number         INTEGER NOT NULL CHECK (attempt_number > 0),
    chat_id                TEXT,
    message_id             TEXT,
    provider_message_id    TEXT,
    started_at             TEXT NOT NULL,
    completed_at           TEXT,
    outcome                TEXT NOT NULL DEFAULT 'sending' CHECK (outcome IN ('sending','provider_accepted','failed','unknown')),
    error_code             TEXT,
    error_detail           TEXT NOT NULL DEFAULT '',
    retryable              INTEGER CHECK (retryable IS NULL OR retryable IN (0,1)),
    next_attempt_at        TEXT,
    created_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX campaign_send_attempts_org_campaign_idx ON campaign_send_attempts(organization_id, campaign_id);
CREATE INDEX campaign_send_attempts_recipient_idx ON campaign_send_attempts(campaign_recipient_id);
CREATE INDEX campaign_send_attempts_campaign_started_idx ON campaign_send_attempts(campaign_id, started_at);

-- campaign_events (0012_campaigns.up.sql) was, until now, campaign-lifecycle
-- only (started/paused/resumed/.../auto_paused). These new nullable columns
-- let the SAME append-only timeline also carry recipient/attempt-level
-- diagnostic events (send_attempt_started, provider_accepted,
-- provider_rejected, retry_scheduled, retry_exhausted,
-- delivery_receipt_received, recipient_terminal, ...) without a parallel
-- table — reusing the existing model rather than duplicating it, per this
-- feature's own design constraint. All nullable: a campaign-lifecycle event
-- leaves every one of them NULL, exactly as before this migration.
-- campaign_recipient_id CASCADEs with its recipient (a replaced,
-- never-contacted recipient's own diagnostic events go with it); attempt_id
-- SETS NULL instead (an attempt row is never deleted, but keeping the event
-- if it somehow were matters more than the FK).
ALTER TABLE campaign_events ADD COLUMN campaign_recipient_id TEXT REFERENCES campaign_recipients(id) ON DELETE CASCADE;
ALTER TABLE campaign_events ADD COLUMN attempt_id TEXT REFERENCES campaign_send_attempts(id) ON DELETE SET NULL;
ALTER TABLE campaign_events ADD COLUMN account_id TEXT;
ALTER TABLE campaign_events ADD COLUMN chat_id TEXT;
ALTER TABLE campaign_events ADD COLUMN message_id TEXT;
ALTER TABLE campaign_events ADD COLUMN error_code TEXT;
CREATE INDEX campaign_events_recipient_idx ON campaign_events(campaign_recipient_id);

-- Reverse lookup from an existing chat/message back to the campaign
-- recipient(s) that touched it — the "campaign-message relationship"
-- membership for the Inbox's Campaign/All views is determined FROM these
-- columns (never from chat_state alone: an existing warm chat a campaign
-- reuses is never chat_state='campaign', see MarkChatCampaignOnly's own doc
-- comment), and from campaign diagnostics resolving "which campaign did this
-- chat/message come from". Neither column existed with an index before this
-- migration (0012_campaigns.up.sql's own comment already documents chat_id/
-- message_id as intentionally FK-less; this only adds the missing indexes).
-- Partial (WHERE ... IS NOT NULL): most rows never reach a resolved chat/
-- message at all (pending, or permanently failed before one existed), so
-- indexing only the populated subset keeps both indexes small.
CREATE INDEX campaign_recipients_chat_idx ON campaign_recipients(chat_id) WHERE chat_id IS NOT NULL;
CREATE INDEX campaign_recipients_message_idx ON campaign_recipients(message_id) WHERE message_id IS NOT NULL;

-- wa_account_connection_events is WhatsApp connection history — durable,
-- append-only, and (unlike wa_accounts.connection_state + last_live_event_at,
-- which only ever hold the CURRENT transition) able to answer "was this
-- account disconnected between 14:02 and 14:15" after the fact. Written from
-- the single existing choke point every real connect/disconnect/logged-out
-- transition already flows through, internal/whatsmeow's setConnectionState
-- — this table adds a durable side effect there, it does not change what
-- triggers a transition. organization_id is nullable because wa_accounts.
-- organization_id itself is nullable (ON DELETE SET NULL there) — this
-- column is a denormalized convenience for a direct org-scoped query, never
-- the security boundary itself (every reader resolves account ownership via
-- the existing orgAnyAccount-style helper first, then queries this table by
-- account_id alone, exactly like campaign_send_attempts above).
CREATE TABLE wa_account_connection_events (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id        TEXT NOT NULL REFERENCES wa_accounts(id) ON DELETE CASCADE,
    organization_id   TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    event             TEXT NOT NULL,
    reason            TEXT NOT NULL DEFAULT '',
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX wa_account_connection_events_account_idx ON wa_account_connection_events(account_id, created_at);
CREATE INDEX wa_account_connection_events_org_idx ON wa_account_connection_events(organization_id, created_at);

-- campaign_recipients itself (status/failure_reason/attempts/next_attempt_at)
-- is intentionally NOT altered by this migration: its status CHECK
-- ('pending','sending','sent','failed','skipped') is unchanged, preserving
-- every existing API response and query exactly as shipped. An unresolved
-- ambiguous outcome (timeout, crash recovery) is recorded as status='failed'
-- — the same choice internal/store.ReconcileStuckSending already made before
-- this migration existed — with the true "outcome unknown, not a confirmed
-- failure" distinction now carried precisely on campaign_send_attempts.
-- outcome='unknown' instead of only in free-text failure_reason prose.
