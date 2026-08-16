-- xchats SQLite schema, part 11: the customer-management (CRM) layer.
-- See 0001_core.up.sql for the shared conventions this file follows.
--
-- This layer sits ABOVE channel transport, never inside it. A person today
-- exists only as a per-channel transport contact (wa_contacts keyed by phone
-- JID, tg_contacts keyed by telegram user id), each scoped to one account, so
-- the same human writing from two channels is two unrelated rows. crm_customers
-- is the org-owned identity those transport rows hang off, and
-- crm_customer_identities is the single place that link is stored — which is
-- what makes a manual merge one UPDATE on one table rather than a rewrite of
-- every channel's tables, and what makes a future channel (Instagram) one new
-- CHECK value rather than a schema change.
--
-- FK-less columns: crm_customer_identities.account_id/contact_id and
-- crm_followups.conversation_id can each reference a wa_* OR a tg_* row, and a
-- single foreign key cannot express an either/or reference — the same
-- constraint ai_drafts.chat_id and the automation_* tables already live with
-- (see 0003_ai_engine.up.sql's and 0009_automation.up.sql's file headers).
-- Ownership is enforced in application code, which always resolves them through
-- an organization-scoped read first.

-- crm_statuses: the org's customer lifecycle. Rows, not a CHECK constraint,
-- because the product promise is that an organization edits its own list; the
-- five seeded below are a starting point, not a closed vocabulary. is_default
-- marks the status auto-assigned to a customer created by inbound ingest —
-- at most one per organization, enforced by the partial unique index.
CREATE TABLE crm_statuses (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug             TEXT NOT NULL,
    name             TEXT NOT NULL,
    color            TEXT NOT NULL DEFAULT '',
    position         INTEGER NOT NULL DEFAULT 0,
    is_default       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (organization_id, slug)
);
CREATE UNIQUE INDEX crm_statuses_default_uq ON crm_statuses(organization_id) WHERE is_default = TRUE;

-- crm_tags: free-form, org-owned labels. The examples in the product brief
-- (VIP, xpayment, «Не готов», …) are deliberately NOT seeded — an organization
-- creates its own.
CREATE TABLE crm_tags (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug             TEXT NOT NULL,
    name             TEXT NOT NULL,
    color            TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (organization_id, slug)
);

-- crm_customers: the unified person. There is deliberately NO scalar `notes`
-- column — crm_customer_notes is the single notes model (authored, timestamped,
-- and part of the timeline), and the sidebar's note block renders its latest
-- row. Two sources of truth for "the note" is exactly the drift this avoids.
--
-- merged_into_id is a tombstone, not a delete: a merged-away customer keeps its
-- row so an id captured before the merge still resolves (to its survivor) and
-- so the merge itself stays auditable. Every list query filters
-- merged_into_id IS NULL.
CREATE TABLE crm_customers (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    display_name      TEXT NOT NULL DEFAULT '',
    phone             TEXT NOT NULL DEFAULT '',
    email             TEXT NOT NULL DEFAULT '',
    avatar_url        TEXT NOT NULL DEFAULT '',
    status_id         TEXT REFERENCES crm_statuses(id) ON DELETE SET NULL,
    assignee_user_id  TEXT REFERENCES users(id) ON DELETE SET NULL,
    custom_fields     TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(custom_fields)),
    merged_into_id    TEXT REFERENCES crm_customers(id) ON DELETE SET NULL,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX crm_customers_org_idx ON crm_customers(organization_id) WHERE merged_into_id IS NULL;
CREATE INDEX crm_customers_org_assignee_idx ON crm_customers(organization_id, assignee_user_id);
CREATE INDEX crm_customers_org_status_idx ON crm_customers(organization_id, status_id);

-- crm_customer_identities: one row per transport contact, linking it to a
-- customer. UNIQUE (channel, contact_id) is both the invariant (a transport
-- contact belongs to exactly one customer) and the lookup index the inbox read
-- joins on — inbox_chats_v hands out exactly (channel, contact_id).
--
-- UNIQUE (organization_id, channel, account_id, external_id) is the
-- find-or-create key used at ingest. Resolution is by provider identity ONLY:
-- names and phone numbers are never consulted, so two channels never collapse
-- into one customer without an explicit manual merge.
--
-- The channel CHECK lists the channels that exist today. Adding Instagram means
-- adding 'instagram' here and nothing else in this layer.
CREATE TABLE crm_customer_identities (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id      TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    channel          TEXT NOT NULL CHECK (channel IN ('whatsapp','simulator','telegram')),
    account_id       TEXT NOT NULL, -- no FK — see file header
    contact_id       TEXT NOT NULL, -- no FK — see file header
    external_id      TEXT NOT NULL DEFAULT '',
    username         TEXT NOT NULL DEFAULT '',
    phone            TEXT NOT NULL DEFAULT '',
    display_name     TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (channel, contact_id),
    UNIQUE (organization_id, channel, account_id, external_id)
);
CREATE INDEX crm_customer_identities_customer_idx ON crm_customer_identities(customer_id);

CREATE TABLE crm_customer_tags (
    customer_id  TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    tag_id       TEXT NOT NULL REFERENCES crm_tags(id) ON DELETE CASCADE,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    PRIMARY KEY (customer_id, tag_id)
);
CREATE INDEX crm_customer_tags_tag_idx ON crm_customer_tags(tag_id);

-- crm_customer_notes: internal only. Nothing in this table is ever sent to a
-- customer — no channel adapter reads it.
CREATE TABLE crm_customer_notes (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id      TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    author_user_id   TEXT REFERENCES users(id) ON DELETE SET NULL,
    body             TEXT NOT NULL,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX crm_customer_notes_customer_idx ON crm_customer_notes(customer_id, created_at);

-- crm_followups: "what should happen next", the point of this whole layer.
--
-- Time is stored twice on purpose. due_at is a UTC instant and is the ONLY
-- thing overdue/ordering/bucketing reads — the same UTC-is-the-truth rule the
-- rest of the schema follows. due_date ('YYYY-MM-DD') and due_minute
-- (minutes since local midnight, NULL = all-day, since the time is optional)
-- preserve the wall clock the manager actually typed, so the edit form
-- round-trips exactly instead of drifting by an offset. organizations has no
-- timezone column, so day boundaries for the Today/Tomorrow/This week buckets
-- come from an explicit tz offset the caller supplies per request.
CREATE TABLE crm_followups (
    id                  TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id         TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    conversation_id     TEXT,          -- no FK — see file header; NULL when created from the customer page
    channel             TEXT NOT NULL DEFAULT '',
    due_at              TEXT NOT NULL,
    due_date            TEXT NOT NULL,
    due_minute          INTEGER CHECK (due_minute IS NULL OR (due_minute BETWEEN 0 AND 1439)),
    action              TEXT NOT NULL DEFAULT 'call' CHECK (action IN ('call','message','meeting','other')),
    note                TEXT NOT NULL DEFAULT '',
    assignee_user_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    state               TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open','completed','cancelled')),
    completed_at        TEXT,
    created_by_user_id  TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX crm_followups_due_idx ON crm_followups(organization_id, state, due_at);
CREATE INDEX crm_followups_assignee_idx ON crm_followups(assignee_user_id, state, due_at);
CREATE INDEX crm_followups_customer_idx ON crm_followups(customer_id, due_at);

-- crm_timeline: CRM events ONLY. The customer timeline the UI renders also
-- contains conversation activity ("customer sent a WhatsApp message", "the AI
-- replied"), but those are read live from inbox_messages_v and merged by
-- timestamp at read time — mirroring every message into this table would double
-- every message write to store what the messages table already knows.
CREATE TABLE crm_timeline (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id      TEXT NOT NULL REFERENCES crm_customers(id) ON DELETE CASCADE,
    kind             TEXT NOT NULL CHECK (kind IN (
                         'customer_created','identity_linked','note_added','status_changed',
                         'tag_added','tag_removed','assignee_changed','followup_created',
                         'followup_completed','followup_rescheduled','followup_cancelled',
                         'customers_merged')),
    actor_user_id    TEXT REFERENCES users(id) ON DELETE SET NULL,
    summary          TEXT NOT NULL DEFAULT '',
    detail           TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(detail)),
    occurred_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX crm_timeline_customer_idx ON crm_timeline(customer_id, occurred_at);

-- crm_custom_field_defs: the org's extra profile fields. Definitions live here;
-- the values live in crm_customers.custom_fields, keyed by `key`. Values are a
-- JSON object rather than a row-per-value table because they are only ever read
-- and written as a whole profile, never queried across customers.
CREATE TABLE crm_custom_field_defs (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key              TEXT NOT NULL,
    label            TEXT NOT NULL,
    field_type       TEXT NOT NULL DEFAULT 'text' CHECK (field_type IN ('text','number','date','select')),
    options          TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(options)),
    position         INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (organization_id, key)
);

-- Seed the default lifecycle for every organization that already exists. New
-- organizations get the same set from store.EnsureDefaultStatuses, which runs
-- the identical INSERT — this statement only covers rows that predate the
-- migration. Both are idempotent via UNIQUE (organization_id, slug).
INSERT INTO crm_statuses (organization_id, slug, name, color, position, is_default)
SELECT o.id, d.slug, d.name, d.color, d.position, d.is_default
FROM organizations o
CROSS JOIN (
              SELECT 'new'         AS slug, 'Новый'        AS name, '#64748b' AS color, 1 AS position, TRUE  AS is_default
    UNION ALL SELECT 'interested',       'Интересуется',        '#0ea5e9',       2,           FALSE
    UNION ALL SELECT 'in_progress',      'В работе',            '#f59e0b',       3,           FALSE
    UNION ALL SELECT 'customer',         'Клиент',              '#10b981',       4,           FALSE
    UNION ALL SELECT 'lost',             'Потерян',             '#ef4444',       5,           FALSE
) d;
