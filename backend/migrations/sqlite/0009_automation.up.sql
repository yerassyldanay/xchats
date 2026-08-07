-- xchats SQLite schema, part 9: channel-level debounce + scheduled
-- auto-reply. See 0001_core.up.sql for the shared conventions this file
-- follows, and 0003_ai_engine.up.sql's file header for why account_id/
-- chat_id columns below carry no foreign key: an account can be a
-- wa_accounts or tg_accounts row, and a chat a wa_chats or tg_chats row —
-- a single FK cannot express an either/or reference, so ownership is
-- enforced in application code (internal/httpapi's orgAccount-style guards),
-- exactly like ai_drafts.chat_id already does.

-- automation_settings: one row per account. A missing row means the
-- account is on its implicit default — mode='suggestions', no wait
-- override — see store.AutomationSettingsForAccount, which is what
-- "default existing and new channels to suggestions" actually means: no
-- backfill needed for accounts created before this migration.
CREATE TABLE automation_settings (
    account_id             TEXT PRIMARY KEY NOT NULL,
    mode                   TEXT NOT NULL DEFAULT 'suggestions' CHECK (mode IN ('off','suggestions','scheduled_auto')),
    wait_seconds_override  INTEGER CHECK (wait_seconds_override IS NULL OR (wait_seconds_override BETWEEN 0 AND 60)),
    created_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at             TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

-- automation_schedule_windows: zero or more recurring UTC weekday/time
-- windows per account, consulted only in scheduled_auto mode. weekday is
-- 0=Sunday..6=Saturday (time.Weekday's own numbering) for the window's
-- START day. end_minute <= start_minute means the window wraps past UTC
-- midnight into (weekday+1) % 7 — see backend/automation.Window.Contains,
-- the pure function that interprets these two columns; this table only
-- stores the tuples, never a resolved decision.
CREATE TABLE automation_schedule_windows (
    id            TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    account_id    TEXT NOT NULL,
    weekday       INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute  INTEGER NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute    INTEGER NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    CHECK (end_minute <> start_minute)
);
CREATE INDEX automation_schedule_windows_account_idx ON automation_schedule_windows(account_id);

-- automation_debounce_jobs: one durable row per chat that has ever received
-- automation-tracked traffic — the restart-safe "when should we next act on
-- this chat" clock. Every inbound customer message, while the owning
-- account's mode is not 'off', upserts this row: deadline_at is pushed to
-- last_message + effective_wait and burst_version increments — there is no
-- maximum burst cap, so a chat that never goes quiet simply never comes due.
-- The scheduler claims rows whose deadline has passed (status
-- 'pending' -> 'claimed', see store.ClaimDueDispatchJobs) and promotes each
-- into automation_dispatch_jobs; a new inbound message arriving after a
-- claim still upserts this same row (chat_id is the primary key), flipping
-- it back to 'pending' with a fresh deadline and a bumped version, which is
-- exactly the fencing token a still-running generation for the OLD version
-- is checked against before it is allowed to persist (see
-- store.WriteDraftSetIfVersionCurrent) — a superseded burst is discarded,
-- never shown, never sent.
CREATE TABLE automation_debounce_jobs (
    chat_id        TEXT PRIMARY KEY NOT NULL,
    account_id     TEXT NOT NULL,
    channel        TEXT NOT NULL,
    deadline_at    TEXT NOT NULL,
    burst_version  INTEGER NOT NULL DEFAULT 1,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','claimed')),
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX automation_debounce_jobs_due_idx ON automation_debounce_jobs(status, deadline_at);
CREATE INDEX automation_debounce_jobs_account_idx ON automation_debounce_jobs(account_id);

-- automation_dispatch_jobs: one durable row per claimed debounce burst that
-- is (about to be) generating a response and possibly auto-sending it. A row
-- still 'pending', or 'processing' past a stuck cutoff, means the process
-- exited mid-flight — internal/automation's boot-time reconcile pass
-- re-publishes it, mirroring worker.StartTelegramMediaSweeper's own
-- "the row IS the durable queue" recovery pattern. A row is deleted once its
-- work is settled (drafted-and-discarded-as-stale, drafted-and-suggested, or
-- drafted-and-sent) — there is no terminal status to keep around.
CREATE TABLE automation_dispatch_jobs (
    id             TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    chat_id        TEXT NOT NULL,
    account_id     TEXT NOT NULL,
    channel        TEXT NOT NULL,
    burst_version  INTEGER NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing')),
    attempts       INTEGER NOT NULL DEFAULT 0,
    last_error     TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX automation_dispatch_jobs_status_idx ON automation_dispatch_jobs(status, updated_at);
CREATE INDEX automation_dispatch_jobs_chat_idx ON automation_dispatch_jobs(chat_id);
