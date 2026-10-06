-- 5/5 Campaigns & automation: bulk-send campaigns with their recipients, events and
-- quiet-hours windows, reusable message templates, per-account sending pace, limits and send
-- log, and per-channel automation (debounce, scheduled auto-reply, dispatch jobs).
-- Depends on 20261006000001_identity_access.
--
-- account_id columns carry NO foreign key: an account is a wa_accounts, tg_accounts or
-- channel_accounts row, and a single FK cannot express an either/or reference (the same rule
-- as ai_drafts.chat_id in 20261006000003_ai_knowledge_base). Ownership is enforced in
-- internal/httpapi.
--
-- Every weekday/time window here is a UTC tuple: weekday 0=Sunday..6=Saturday for the START
-- day, and end_minute <= start_minute wraps past UTC midnight into the next day.
-- Shared by SQLite and PostgreSQL: the migration runner expands the dialect macros
-- for the open engine (docs/database.md). Idempotent; the runner owns the transaction.

-- One row per account; a missing row means the implicit default (mode 'suggestions', no wait
-- override), so accounts created later need no backfill.
CREATE TABLE IF NOT EXISTS automation_settings (
    account_id             TEXT PRIMARY KEY NOT NULL,
    mode                   TEXT NOT NULL DEFAULT 'suggestions' CHECK (mode IN ('off','suggestions','scheduled_auto')),
    wait_seconds_override  INTEGER CHECK (wait_seconds_override IS NULL OR (wait_seconds_override BETWEEN 0 AND 60)),
    created_at             {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at             {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE TABLE IF NOT EXISTS automation_schedule_windows (
    id            TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id    TEXT NOT NULL,
    weekday       INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute  INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute    INTEGER NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    created_at    {{timestamp}} NOT NULL DEFAULT {{now}},
    CHECK (end_minute <> start_minute)
);

CREATE INDEX IF NOT EXISTS automation_schedule_windows_account_idx ON automation_schedule_windows(account_id);

CREATE TABLE IF NOT EXISTS automation_debounce_jobs (
    chat_id        TEXT PRIMARY KEY NOT NULL,
    account_id     TEXT NOT NULL,
    channel        TEXT NOT NULL,
    deadline_at    {{timestamp}} NOT NULL,
    burst_version  BIGINT NOT NULL DEFAULT 1,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','claimed')),
    created_at     {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at     {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE INDEX IF NOT EXISTS automation_debounce_jobs_due_idx ON automation_debounce_jobs(status, deadline_at);

CREATE INDEX IF NOT EXISTS automation_debounce_jobs_account_idx ON automation_debounce_jobs(account_id);

CREATE TABLE IF NOT EXISTS automation_dispatch_jobs (
    id             TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    chat_id        TEXT NOT NULL,
    account_id     TEXT NOT NULL,
    channel        TEXT NOT NULL,
    burst_version  BIGINT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing')),
    attempts       INTEGER NOT NULL DEFAULT 0,
    last_error     TEXT NOT NULL DEFAULT '',
    created_at     {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at     {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE INDEX IF NOT EXISTS automation_dispatch_jobs_status_idx ON automation_dispatch_jobs(status, updated_at);

CREATE INDEX IF NOT EXISTS automation_dispatch_jobs_chat_idx ON automation_dispatch_jobs(chat_id);

CREATE UNIQUE INDEX IF NOT EXISTS automation_dispatch_jobs_burst_idx
    ON automation_dispatch_jobs(chat_id, burst_version);

CREATE TABLE IF NOT EXISTS campaign_account_settings (
    account_id            TEXT PRIMARY KEY NOT NULL,
    limit_mode            TEXT NOT NULL DEFAULT 'default' CHECK (limit_mode IN ('default','custom')),
    min_interval_seconds  INTEGER NOT NULL DEFAULT 90 CHECK (min_interval_seconds > 0),
    jitter_seconds        INTEGER NOT NULL DEFAULT 30 CHECK (jitter_seconds >= 0),
    paused                BOOLEAN NOT NULL DEFAULT FALSE CHECK (paused IN (FALSE, TRUE)),
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at            {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE TABLE IF NOT EXISTS campaign_account_limits (
    account_id            TEXT NOT NULL,
    window_seconds        INTEGER NOT NULL CHECK (window_seconds > 0),
    max_sends             INTEGER NOT NULL CHECK (max_sends > 0),
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    PRIMARY KEY (account_id, window_seconds)
);

CREATE TABLE IF NOT EXISTS campaign_account_windows (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id            TEXT NOT NULL,
    weekday               INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute          INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute            INTEGER NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    CHECK (end_minute <> start_minute)
);

CREATE INDEX IF NOT EXISTS campaign_account_windows_account_idx ON campaign_account_windows(account_id);

CREATE TABLE IF NOT EXISTS campaigns (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    organization_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,
    account_id            TEXT NOT NULL,
    channel               TEXT NOT NULL CHECK (channel IN ('whatsapp','simulator','telegram','instagram','messenger','whatsapp_cloud')),
    status                TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','scheduled','running','paused','completed','failed','cancelled')),
    message_body          TEXT NOT NULL DEFAULT '',
    variables             {{json "variables"}} NOT NULL DEFAULT '[]',
    min_interval_seconds  INTEGER CHECK (min_interval_seconds IS NULL OR min_interval_seconds > 0),
    jitter_seconds        INTEGER CHECK (jitter_seconds IS NULL OR jitter_seconds >= 0),
    schedule_at           {{timestamp}},
    started_at            {{timestamp}},
    created_by            TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    CHECK ((min_interval_seconds IS NULL) = (jitter_seconds IS NULL))
);

CREATE INDEX IF NOT EXISTS campaigns_org_idx ON campaigns(organization_id);

CREATE INDEX IF NOT EXISTS campaigns_account_status_idx ON campaigns(account_id, status);

-- Its own table, not a campaign with no recipients: a template is pure content (no account,
-- channel, status, pace or schedule). is_archived is a soft hide; campaigns copy message_body and
-- variables at creation and carry no FK back, so archiving never affects a campaign that used it.
CREATE TABLE IF NOT EXISTS campaign_templates (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    organization_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name                  TEXT NOT NULL,
    message_body          TEXT NOT NULL DEFAULT '',
    variables             {{json "variables"}} NOT NULL DEFAULT '[]',
    is_archived           BOOLEAN NOT NULL DEFAULT FALSE CHECK (is_archived IN (FALSE, TRUE)),
    created_by            TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at            {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE INDEX IF NOT EXISTS campaign_templates_org_idx ON campaign_templates(organization_id);

CREATE INDEX IF NOT EXISTS campaign_templates_org_archived_idx ON campaign_templates(organization_id, is_archived);

CREATE TABLE IF NOT EXISTS campaign_recipients (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    normalized_identity   TEXT NOT NULL,
    raw_input             TEXT NOT NULL DEFAULT '',
    name                  TEXT NOT NULL DEFAULT '',
    attributes            {{json "attributes"}} NOT NULL DEFAULT '{}',
    status                TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sending','sent','failed','skipped')),
    failure_reason        TEXT NOT NULL DEFAULT '',
    attempts              INTEGER NOT NULL DEFAULT 0,
    next_attempt_at       {{timestamp}},
    chat_id               TEXT,
    message_id            TEXT,
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (campaign_id, normalized_identity)
);

CREATE INDEX IF NOT EXISTS campaign_recipients_campaign_status_idx ON campaign_recipients(campaign_id, status);

CREATE INDEX IF NOT EXISTS campaign_recipients_identity_status_idx ON campaign_recipients(normalized_identity, status);

CREATE TABLE IF NOT EXISTS campaign_events (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    event                 TEXT NOT NULL,
    actor_user_id         TEXT REFERENCES users(id) ON DELETE SET NULL,
    detail                {{json "detail"}} NOT NULL DEFAULT '{}',
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE INDEX IF NOT EXISTS campaign_events_campaign_idx ON campaign_events(campaign_id, created_at);

CREATE TABLE IF NOT EXISTS campaign_windows (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    weekday               INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute          INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute            INTEGER NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    created_at            {{timestamp}} NOT NULL DEFAULT {{now}},
    CHECK (end_minute <> start_minute)
);

CREATE INDEX IF NOT EXISTS campaign_windows_campaign_idx ON campaign_windows(campaign_id);

CREATE TABLE IF NOT EXISTS campaign_send_log (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    account_id            TEXT NOT NULL,
    campaign_id           TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    recipient_id          TEXT NOT NULL REFERENCES campaign_recipients(id) ON DELETE CASCADE,
    attempted_at          {{timestamp}} NOT NULL DEFAULT {{now}},
    outcome               TEXT NOT NULL CHECK (outcome IN ('sent','failed')),
    origin                TEXT NOT NULL DEFAULT 'campaign' CHECK (origin IN ('campaign','manual','ai'))
);

CREATE INDEX IF NOT EXISTS campaign_send_log_account_time_idx ON campaign_send_log(account_id, attempted_at);
