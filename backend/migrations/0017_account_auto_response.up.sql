-- 0017_account_auto_response — per-account auto-response: a debounce table so
-- a burst of inbound messages produces one AI draft instead of one per
-- message, plus the per-weekday schedule, delay and durable job queue that
-- fire an unattended reply when nobody acts on the draft in time.
--
-- account_auto_response has no FK on account_id: ids span wa_accounts and
-- tg_accounts (no shared parent table), the same reason migration 0013
-- dropped ai_drafts' cross-table FKs. Windows are non-wrapping half-open
-- [starts_minute, ends_minute) intervals on ONE weekday each — minutes as
-- smallint sidestep a Postgres `time` column's inability to represent
-- "24:00" at all (pgtype rejects it, time.Parse("15:04","24:00") fails, and
-- make_interval(mins=>1440)::time silently wraps to 00:00). See
-- internal/autoresponse for the normalization/coverage logic this feeds.
--
-- auto_response_jobs is the durable work item the sweeper claims and fires,
-- following the pattern worker.go documents for Telegram media: the ROW is
-- the durable work item, not a queue message. auto_response_jobs_trigger_uq
-- is load-bearing, not belt-and-braces: it is what makes "one inbound
-- message produces at most one auto-reply, ever" a database-enforced
-- invariant rather than an application convention.
SET search_path = xchats, public;

-- ---------------------------------------------------------------------------
-- Debounce: coalesce a burst of inbound messages into one draft generation.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS xchats.chat_draft_debounce (
    chat_id            uuid PRIMARY KEY,
    channel            text        NOT NULL,
    trigger_message_id uuid        NOT NULL,
    first_seen_at      timestamptz NOT NULL DEFAULT now(),
    generate_after     timestamptz NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS chat_draft_debounce_due_idx
    ON xchats.chat_draft_debounce (generate_after);

-- ---------------------------------------------------------------------------
-- account_auto_response — one policy row per account (any channel).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS xchats.account_auto_response (
    account_id          uuid PRIMARY KEY,
    organization_id      uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    enabled              boolean NOT NULL DEFAULT false,
    reply_mode            text NOT NULL DEFAULT 'ai' CHECK (reply_mode IN ('ai', 'fixed')),
    fixed_text            text NOT NULL DEFAULT '',
    timezone              text NOT NULL DEFAULT 'Asia/Almaty',
    delay_seconds         int NOT NULL DEFAULT 120,
    cooldown_seconds      int NOT NULL DEFAULT 300,
    skip_when_escalated   boolean NOT NULL DEFAULT true,
    pause_when_assigned   boolean NOT NULL DEFAULT false,
    updated_by_user_id    uuid REFERENCES xchats.users(id) ON DELETE SET NULL,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS account_auto_response_org_idx
    ON xchats.account_auto_response(organization_id);

-- Canonical, non-wrapping storage: every row is [starts_minute, ends_minute)
-- on ONE weekday (0=Sunday..6=Saturday, matching both Go's time.Weekday and
-- Postgres EXTRACT(DOW ...), so SQL and Go can never disagree on the
-- convention). "18:00-09:00 on Monday" is never stored as one row — it is
-- two: Monday [0,540) + Monday [1080,1440). internal/autoresponse.Normalize
-- is what performs that split before a write reaches this table.
CREATE TABLE IF NOT EXISTS xchats.account_auto_response_window (
    account_id    uuid NOT NULL REFERENCES xchats.account_auto_response(account_id) ON DELETE CASCADE,
    weekday       smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    starts_minute smallint NOT NULL CHECK (starts_minute BETWEEN 0 AND 1439),
    ends_minute   smallint NOT NULL CHECK (ends_minute BETWEEN 1 AND 1440),
    CHECK (starts_minute < ends_minute),
    PRIMARY KEY (account_id, weekday, starts_minute)
);

-- ---------------------------------------------------------------------------
-- auto_response_jobs — the durable, at-most-one-send-per-trigger work item.
-- Loose UUID references throughout (account_id/chat_id/draft_id/
-- sent_message_id): account_id spans wa_accounts/tg_accounts and chat_id
-- spans wa_chats/tg_chats, so there is no single table to FK into — the same
-- reasoning migration 0013 already applied to ai_drafts.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS xchats.auto_response_jobs (
    id                  uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id     uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    account_id          uuid NOT NULL,
    chat_id             uuid NOT NULL,
    channel             text NOT NULL,
    trigger_message_id  uuid NOT NULL,
    trigger_arrived_at  timestamptz NOT NULL,
    due_at              timestamptz NOT NULL,
    draft_id            uuid,
    state               text NOT NULL DEFAULT 'pending'
                         CHECK (state IN ('pending', 'claimed', 'sent', 'cancelled', 'failed')),
    cancel_reason       text NOT NULL DEFAULT '',
    sent_message_id     uuid,
    last_error          text NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- At most one pending job per chat: arming re-targets this same row (see
-- store.ArmAutoResponseJob's ON CONFLICT (chat_id) WHERE state='pending')
-- instead of accumulating one job per debounced burst.
CREATE UNIQUE INDEX IF NOT EXISTS auto_response_jobs_chat_pending_uq
    ON xchats.auto_response_jobs(chat_id) WHERE state = 'pending';

-- The invariant: one inbound message produces at most one auto-reply, ever.
-- claimed/sent are included (not just pending) so a job that already fired,
-- or is mid-send, still fences a redelivery of the same trigger message.
CREATE UNIQUE INDEX IF NOT EXISTS auto_response_jobs_trigger_uq
    ON xchats.auto_response_jobs(trigger_message_id)
    WHERE state IN ('pending', 'claimed', 'sent');

CREATE INDEX IF NOT EXISTS auto_response_jobs_due_idx
    ON xchats.auto_response_jobs(due_at) WHERE state = 'pending';
-- Backs the reaper's stale-claim scan and the purge's terminal-row sweep.
CREATE INDEX IF NOT EXISTS auto_response_jobs_terminal_idx
    ON xchats.auto_response_jobs(updated_at) WHERE state IN ('sent', 'cancelled', 'failed');
CREATE INDEX IF NOT EXISTS auto_response_jobs_claimed_idx
    ON xchats.auto_response_jobs(updated_at) WHERE state = 'claimed';
