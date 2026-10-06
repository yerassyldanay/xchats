-- 4/5 CRM: org-owned customers and their per-channel identities, statuses, tags, custom
-- fields, notes, follow-ups and the timeline. Depends on 20261006000001_identity_access.
--
-- This layer sits ABOVE channel transport: crm_customer_identities is the single place a
-- person is linked to wa_* / tg_* / channel_* contacts, so a manual merge is one UPDATE and a
-- new channel needs no migration. crm_customer_identities.account_id / contact_id and
-- crm_followups.conversation_id carry NO foreign key (see the ai_drafts note in
-- 20261006000003_ai_knowledge_base); every read and write is organization-scoped instead.
-- Shared by SQLite and PostgreSQL: the migration runner expands the dialect macros
-- for the open engine (docs/database.md). Idempotent; the runner owns the transaction.

-- Rows, not a CHECK, because an organization edits its own lifecycle; the five seeded at the
-- bottom of this file are a starting point. is_default marks the status given to customers
-- created by inbound ingest: at most one per organization (partial unique index).
CREATE TABLE IF NOT EXISTS crm_statuses (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug             TEXT NOT NULL,
    name             TEXT NOT NULL,
    color            TEXT NOT NULL DEFAULT '',
    position         INTEGER NOT NULL DEFAULT 0,
    is_default       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at       {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (organization_id, slug)
);

CREATE UNIQUE INDEX IF NOT EXISTS crm_statuses_default_uq ON crm_statuses(organization_id) WHERE is_default = TRUE;

CREATE TABLE IF NOT EXISTS crm_tags (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug             TEXT NOT NULL,
    name             TEXT NOT NULL,
    color            TEXT NOT NULL DEFAULT '',
    created_at       {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at       {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (organization_id, slug)
);

CREATE TABLE IF NOT EXISTS crm_custom_field_defs (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key              TEXT NOT NULL,
    label            TEXT NOT NULL,
    field_type       TEXT NOT NULL DEFAULT 'text' CHECK (field_type IN ('text','number','date','select')),
    options          {{json "options"}} NOT NULL DEFAULT '[]',
    position         INTEGER NOT NULL DEFAULT 0,
    created_at       {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at       {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (organization_id, key)
);

CREATE TABLE IF NOT EXISTS crm_customers (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    display_name      TEXT NOT NULL DEFAULT '',
    phone             TEXT NOT NULL DEFAULT '',
    email             TEXT NOT NULL DEFAULT '',
    avatar_url        TEXT NOT NULL DEFAULT '',
    status_id         TEXT REFERENCES crm_statuses(id) ON DELETE SET NULL,
    assignee_user_id  TEXT REFERENCES users(id) ON DELETE SET NULL,
    custom_fields     {{json "custom_fields"}} NOT NULL DEFAULT '{}',
    merged_into_id    TEXT REFERENCES crm_customers(id) ON DELETE SET NULL,
    created_at        {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at        {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE INDEX IF NOT EXISTS crm_customers_org_idx ON crm_customers(organization_id) WHERE merged_into_id IS NULL;

CREATE INDEX IF NOT EXISTS crm_customers_org_assignee_idx ON crm_customers(organization_id, assignee_user_id);

CREATE INDEX IF NOT EXISTS crm_customers_org_status_idx ON crm_customers(organization_id, status_id);

CREATE TABLE IF NOT EXISTS crm_customer_identities (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id      TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    channel          TEXT NOT NULL,
    account_id       TEXT NOT NULL,
    contact_id       TEXT NOT NULL,
    external_id      TEXT NOT NULL DEFAULT '',
    username         TEXT NOT NULL DEFAULT '',
    phone            TEXT NOT NULL DEFAULT '',
    display_name     TEXT NOT NULL DEFAULT '',
    created_at       {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at       {{timestamp}} NOT NULL DEFAULT {{now}},
    UNIQUE (channel, contact_id),
    UNIQUE (organization_id, channel, account_id, external_id)
);

CREATE INDEX IF NOT EXISTS crm_customer_identities_customer_idx ON crm_customer_identities(customer_id);

CREATE TABLE IF NOT EXISTS crm_customer_tags (
    customer_id  TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    tag_id       TEXT NOT NULL REFERENCES crm_tags(id) ON DELETE CASCADE,
    created_at   {{timestamp}} NOT NULL DEFAULT {{now}},
    PRIMARY KEY (customer_id, tag_id)
);

CREATE INDEX IF NOT EXISTS crm_customer_tags_tag_idx ON crm_customer_tags(tag_id);

CREATE TABLE IF NOT EXISTS crm_customer_notes (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id      TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    author_user_id   TEXT REFERENCES users(id) ON DELETE SET NULL,
    body             TEXT NOT NULL,
    created_at       {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at       {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE INDEX IF NOT EXISTS crm_customer_notes_customer_idx ON crm_customer_notes(customer_id, created_at);

-- Time is stored twice on purpose: due_at is the UTC instant everything orders and buckets by;
-- due_date and due_minute keep the wall clock the manager typed so the edit form round-trips
-- without timezone drift (due_minute is NULL for an all-day follow-up).
CREATE TABLE IF NOT EXISTS crm_followups (
    id                  TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    organization_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id         TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    conversation_id     TEXT,
    channel             TEXT NOT NULL DEFAULT '',
    due_at              {{timestamp}} NOT NULL,
    due_date            TEXT NOT NULL,
    due_minute          INTEGER CHECK (due_minute IS NULL OR (due_minute BETWEEN 0 AND 1439)),
    action              TEXT NOT NULL DEFAULT 'call' CHECK (action IN ('call','message','meeting','other')),
    note                TEXT NOT NULL DEFAULT '',
    assignee_user_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    state               TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open','completed','cancelled')),
    completed_at        {{timestamp}},
    created_by_user_id  TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at          {{timestamp}} NOT NULL DEFAULT {{now}},
    updated_at          {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE INDEX IF NOT EXISTS crm_followups_due_idx ON crm_followups(organization_id, state, due_at);

CREATE INDEX IF NOT EXISTS crm_followups_assignee_idx ON crm_followups(assignee_user_id, state, due_at);

CREATE INDEX IF NOT EXISTS crm_followups_customer_idx ON crm_followups(customer_id, due_at);

CREATE TABLE IF NOT EXISTS crm_timeline (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT {{uuid}},
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id      TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    kind             TEXT NOT NULL CHECK (kind IN (
                         'customer_created','identity_linked','note_added','status_changed',
                         'tag_added','tag_removed','assignee_changed','followup_created',
                         'followup_completed','followup_rescheduled','followup_cancelled',
                         'customers_merged')),
    actor_user_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    summary          TEXT NOT NULL DEFAULT '',
    detail           {{json "detail"}} NOT NULL DEFAULT '{}',
    occurred_at      {{timestamp}} NOT NULL DEFAULT {{now}},
    created_at       {{timestamp}} NOT NULL DEFAULT {{now}}
);

CREATE INDEX IF NOT EXISTS crm_timeline_customer_idx ON crm_timeline(customer_id, occurred_at);

-- Required initial state; replay never overwrites an operator's changes.
INSERT INTO crm_statuses (organization_id, slug, name, color, position, is_default)
VALUES
('00000000-0000-0000-0000-000000000001', 'new', 'Новый', '#64748b', 1, TRUE),
('00000000-0000-0000-0000-000000000001', 'interested', 'Интересуется', '#0ea5e9', 2, FALSE),
('00000000-0000-0000-0000-000000000001', 'in_progress', 'В работе', '#f59e0b', 3, FALSE),
('00000000-0000-0000-0000-000000000001', 'customer', 'Клиент', '#10b981', 4, FALSE),
('00000000-0000-0000-0000-000000000001', 'lost', 'Потерян', '#ef4444', 5, FALSE)
ON CONFLICT DO NOTHING;
