-- xchats SQLite schema, part 12: Campaigns — bulk outbound sends against a
-- pasted/uploaded recipient list, delivered through the existing
-- messaging.OutboundMessage -> SenderRegistry seam (backend/internal/
-- outbound.Deliver; see internal/campaign's Runner). See 0001_core.up.sql
-- for the shared conventions this file follows (uuidv4 default expression,
-- timestamp defaults, json_valid CHECKs).
--
-- account_id columns below carry NO foreign key, on purpose: an account is
-- either a wa_accounts, tg_accounts or channel_accounts row, and a single
-- FK cannot express an either/or reference — identical, already-documented
-- compromise to 0009_automation.up.sql's account_id/chat_id columns.
-- Ownership is enforced in internal/httpapi instead (orgCampaign, mirroring
-- orgAnyAccount).
--
-- Deliberately NOT here: a saved-audience-list entity, a real customer
-- Contact/CRM record, and WhatsApp Cloud template sending — all out of
-- scope for this build (see plan/DECISIONS.md's Campaigns entry).

-- organizations gains a timezone: campaign quiet-hours windows are entered
-- by an operator in org-LOCAL time and converted to UTC tuples once, at
-- write time (internal/httpapi) — the same UTC-tuple shape
-- automation_schedule_windows already stores. Defaults to Asia/Almaty, this
-- deployment's home timezone; every pre-existing organization gets that
-- same default with no backfill needed (mirrors automation_settings' "a
-- missing row is the implicit default" philosophy, one level up: a missing
-- explicit value here just means "hasn't been changed from the default").
ALTER TABLE organizations ADD COLUMN timezone TEXT NOT NULL DEFAULT 'Asia/Almaty';

-- campaigns is the durable state machine driving one bulk send:
--   draft -> scheduled -> running <-> paused -> {completed,failed,cancelled}
-- (backend/campaign.CanTransition is the single source of truth for which
-- edges are legal; this table only stores the current state, never
-- interprets it). started_at is stamped the first time status becomes
-- 'running' — internal/campaign's account-scoped claim path orders
-- competing campaigns on the same account FIFO by this column, so a
-- campaign that has been waiting longest gets first claim on shared
-- capacity. message_body/variables are FROZEN (application-enforced, see
-- backend/campaign.CanEditContent) the instant campaign_recipients.attempts
-- has moved any row past 'pending' — this table has no CHECK for that, it
-- is a cross-table invariant internal/httpapi enforces before every write.
-- min_interval_seconds/jitter_seconds are a per-campaign pace override
-- (NULL = inherit the sending account's own pace from
-- campaign_account_settings); they can only ever make THIS campaign wait
-- longer between its own attempts, never bypass the account-wide rolling-
-- window tiers in campaign_account_limits, which remain the hard,
-- shared-across-every-campaign ceiling regardless.
CREATE TABLE campaigns (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,
    account_id            TEXT NOT NULL,
    channel               TEXT NOT NULL CHECK (channel IN ('whatsapp','simulator','telegram','instagram','messenger','whatsapp_cloud')),
    status                TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','scheduled','running','paused','completed','failed','cancelled')),
    message_body          TEXT NOT NULL DEFAULT '',
    -- variables is a JSON array of the {{variable}} names last detected in
    -- message_body (backend/campaign.ExtractVariables) — informational,
    -- rendered by the UI to explain personalization; Render itself derives
    -- substitutions straight from each recipient's own attributes and does
    -- not consult this column.
    variables             TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(variables)),
    min_interval_seconds  INTEGER CHECK (min_interval_seconds IS NULL OR min_interval_seconds > 0),
    jitter_seconds        INTEGER CHECK (jitter_seconds IS NULL OR jitter_seconds >= 0),
    schedule_at           TEXT,
    started_at            TEXT,
    created_by            TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    -- The pace override is all-or-nothing: a campaign that narrows its pace
    -- always sets both fields together, never just one — see
    -- internal/store's campaignPaceOverride, the sole reader.
    CHECK ((min_interval_seconds IS NULL) = (jitter_seconds IS NULL))
);
CREATE INDEX campaigns_org_idx ON campaigns(organization_id);
CREATE INDEX campaigns_account_status_idx ON campaigns(account_id, status);

-- campaign_recipients is one row per (campaign, identity) — the recipient
-- list itself, its dedup rule, and its own delivery outcome all at once.
-- UNIQUE(campaign_id, normalized_identity) is both the preview-time dedup
-- key (backend/campaign.ParseRecipients buckets a repeat as 'duplicate'
-- before this table is ever touched) and the double-send guard: the runner
-- can never claim the same identity twice for the same campaign even under
-- a retried/duplicated claim attempt. status walks pending -> sending ->
-- {sent,failed,skipped} — sending is committed in the SAME transaction as
-- the campaign_send_log row that claims it (see internal/store's
-- ClaimNextRecipient), which is what makes the at-most-once guarantee real:
-- the ledger and the claim can never disagree about whether an attempt was
-- made. chat_id/message_id are stamped once the runner's find-or-create and
-- send actually happen; both carry no foreign key for the same either/or
-- reason account_id above does not (a chat is a wa_chats/tg_chats/
-- channel_chats row, a message a wa_messages/tg_messages/channel_messages
-- row).
CREATE TABLE campaign_recipients (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    normalized_identity   TEXT NOT NULL,
    raw_input             TEXT NOT NULL DEFAULT '',
    name                  TEXT NOT NULL DEFAULT '',
    attributes            TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(attributes)),
    status                TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sending','sent','failed','skipped')),
    failure_reason        TEXT NOT NULL DEFAULT '',
    attempts              INTEGER NOT NULL DEFAULT 0,
    -- next_attempt_at is a transient failure's backoff floor: ClaimNextRecipient
    -- excludes a 'pending' row until this passes. NULL for a row that has
    -- never failed (or is past its backoff) — never set at all for a
    -- permanent failure, which goes straight to 'failed' instead of back to
    -- 'pending'.
    next_attempt_at       TEXT,
    chat_id               TEXT,
    message_id            TEXT,
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (campaign_id, normalized_identity)
);
CREATE INDEX campaign_recipients_campaign_status_idx ON campaign_recipients(campaign_id, status);
-- Backs SuppressPendingForIdentity's cross-campaign lookup: an inbound (or
-- a manual send) from one identity has to find every still-pending row for
-- that SAME identity across every campaign sharing the account it arrived
-- on, not just the one campaign a caller happens to have in hand.
CREATE INDEX campaign_recipients_identity_status_idx ON campaign_recipients(normalized_identity, status);

-- campaign_events is the append-only lifecycle timeline (plan: "actor + UTC
-- timestamp", retained read-only forever — there is no prune here, unlike
-- campaign_send_log below). actor_user_id is NULL for a system-originated
-- event (an auto-pause on disconnect, a run completing) — there is no
-- human to attribute those to.
CREATE TABLE campaign_events (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    event                 TEXT NOT NULL,
    actor_user_id         TEXT REFERENCES users(id) ON DELETE SET NULL,
    detail                TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(detail)),
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX campaign_events_campaign_idx ON campaign_events(campaign_id, created_at);

-- campaign_account_settings: one row per account, exactly like
-- automation_settings — a MISSING row means the account is on the implicit
-- built-in default (backend/campaign.DefaultTiersFor/DefaultPacingFor: the
-- whatsmeow/WhatsApp Cloud pacing for every real channel, unlimited for the
-- simulator). limit_mode is a UI-facing preference ("default" vs
-- "custom") that does not itself gate anything server-side — the stored
-- min_interval_seconds/jitter_seconds are always what the claim path reads
-- once this row exists, regardless of which mode produced them. paused is
-- a manual, account-WIDE kill switch distinct from any one campaign's own
-- status: the claim path skips the account entirely while it is set,
-- whatever state its individual campaigns are in — separate from (and in
-- addition to) the automatic per-campaign pause a >60s disconnect causes
-- (see internal/campaign's Runner).
CREATE TABLE campaign_account_settings (
    account_id            TEXT PRIMARY KEY NOT NULL,
    limit_mode            TEXT NOT NULL DEFAULT 'default' CHECK (limit_mode IN ('default','custom')),
    min_interval_seconds  INTEGER NOT NULL DEFAULT 90 CHECK (min_interval_seconds > 0),
    jitter_seconds        INTEGER NOT NULL DEFAULT 30 CHECK (jitter_seconds >= 0),
    paused                INTEGER NOT NULL DEFAULT 0 CHECK (paused IN (0,1)),
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

-- campaign_account_limits: one row PER TIER — a simultaneous rolling-window
-- cap, not a column on campaign_account_settings, because the number of
-- tiers itself varies (the built-in default is 5: 1h/2h/6h/12h/24h) and a
-- fixed column set could never hold a custom count. PRIMARY KEY
-- (account_id, window_seconds) is both the natural key and what makes a
-- "set the tiers" save a plain upsert-per-row rather than a delete-and-
-- reinsert. A missing set of rows for an account (same "row absence is the
-- default" rule as campaign_account_settings) means backend/campaign.
-- DefaultTiersFor's built-in five.
CREATE TABLE campaign_account_limits (
    account_id            TEXT NOT NULL,
    window_seconds        INTEGER NOT NULL CHECK (window_seconds > 0),
    max_sends             INTEGER NOT NULL CHECK (max_sends > 0),
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    PRIMARY KEY (account_id, window_seconds)
);

-- campaign_account_windows: an account's quiet hours — the SAME recurring
-- UTC weekday/time-of-day tuple shape automation_schedule_windows already
-- uses (see that table's own doc comment in 0009_automation.up.sql for the
-- overnight-wrap semantics end_minute <= start_minute encodes; backend/
-- campaign.Window is a type alias onto backend/automation.Window, the pure
-- function that interprets these columns). A hard constraint: no campaign
-- on this account may send outside these hours, only further narrow them
-- (see campaign_windows below and backend/campaign.WindowsOK). Zero rows
-- means the account has never set quiet hours and is UNRESTRICTED — the
-- opposite default from automation_schedule_windows' own "zero windows =
-- never in schedule", since that table gates a feature an operator must
-- opt into, while a fresh campaigns account must not be silently unusable
-- until someone remembers to configure hours.
CREATE TABLE campaign_account_windows (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id            TEXT NOT NULL,
    weekday               INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute          INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute            INTEGER NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    CHECK (end_minute <> start_minute)
);
CREATE INDEX campaign_account_windows_account_idx ON campaign_account_windows(account_id);

-- campaign_windows: a campaign's OWN window, narrowing (never widening) its
-- account's campaign_account_windows — same shape and semantics, see that
-- table's doc comment above. Zero rows means this campaign places no
-- narrowing of its own; the account's hours (if any) still apply.
CREATE TABLE campaign_windows (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    weekday               INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute          INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute            INTEGER NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    CHECK (end_minute <> start_minute)
);
CREATE INDEX campaign_windows_campaign_idx ON campaign_windows(campaign_id);

-- campaign_send_log is the rate-limit ledger: one row per PROVIDER ATTEMPT
-- (not per recipient — a 3x-retried send writes up to 4 rows), which is
-- what makes a retry genuinely consume quota instead of getting a second
-- try for free. backend/campaign.Budget's `attempts` parameter is read
-- straight from this table (attempted_at timestamps within the largest
-- configured tier's window). origin exists so a future change could widen
-- what counts against the shared budget without a schema change — v1 only
-- ever writes 'campaign' rows here (manual and AI sends are never gated
-- and never logged to this table at all, see plan/DECISIONS.md), and every
-- ledger read filters origin = 'campaign' explicitly, documenting that
-- restriction at the query rather than leaving it implicit. Pruned after 7
-- days (internal/campaign's Scheduler) — this table is a rolling operational
-- ledger, NOT the permanent audit trail; that lives on campaign_recipients
-- (attempts/status/failure_reason) and campaign_events, neither of which
-- this prune ever touches. See backend/campaign.MaxTierWindowSeconds' own
-- doc comment for why 7 days is the retention floor a custom tier's own
-- window is validated against, not just a cleanup convenience.
CREATE TABLE campaign_send_log (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id            TEXT NOT NULL,
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    recipient_id          TEXT NOT NULL REFERENCES campaign_recipients(id) ON DELETE CASCADE,
    attempted_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    outcome               TEXT NOT NULL CHECK (outcome IN ('sent','failed')),
    origin                TEXT NOT NULL DEFAULT 'campaign' CHECK (origin IN ('campaign','manual','ai'))
);
CREATE INDEX campaign_send_log_account_time_idx ON campaign_send_log(account_id, attempted_at);
