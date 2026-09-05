-- xchats SQLite schema, part 18: structured KB-gap telemetry
-- (backend/aiprompt/kbgap.go, response contract v7 — see
-- backend/aiprompt/frame.go's PromptRefShopKBV7 doc comment). Until now an
-- escalation's only recorded cause was ai_drafts.escalation_reason, free
-- Russian prose meant for a human reviewing one chat — never queryable
-- across a whole organization's history. This migration adds an
-- append-only event log alongside it, keyed by a small closed reason-code
-- vocabulary (aiprompt.AllKBGapReasonCodes) so "what is our knowledge base
-- missing" becomes a report instead of a transcript-reading exercise.
--
-- No CHECK constrains reason_code/target_entity_type/source to their closed
-- vocabularies — sales_status/pricing_type/availability_status already set
-- this precedent (see migration 0017's own comment): validating a
-- closed-vocabulary TEXT column in Go (aiprompt.sanitizeKBGap and the store
-- write path) rather than a SQL CHECK lets the vocabulary gain a value
-- without a schema migration.
--
-- ai_kb_gap_events.draft_id references ai_drafts ON DELETE SET NULL, not
-- CASCADE: nothing deletes ai_drafts rows today (supersede/dismiss/claim are
-- all UPDATEs), but the whole point of an append-only telemetry log is that
-- it must outlive the specific draft that produced it if that ever changes —
-- losing the draft link is acceptable, silently losing the telemetry row is
-- not. chat_id and trigger_message_id deliberately carry NO foreign key,
-- mirroring ai_drafts' own channel-neutral chat_id (migration 0013 dropped
-- its FKs into wa_*/tg_* so the same column works for either channel).
--
-- UNIQUE(draft_id) enforces "at most one gap event per draft" at the
-- database level, not just by convention in the store's shared transaction
-- helper: SQLite's UNIQUE treats multiple NULLs as distinct, so it does not
-- constrain the (should-never-happen) case of an event recorded with no
-- draft at all.
CREATE TABLE ai_kb_gap_events (
    id                  TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id     TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    draft_id            TEXT REFERENCES ai_drafts(id) ON DELETE SET NULL,
    channel             TEXT NOT NULL DEFAULT 'whatsapp',
    chat_id             TEXT NOT NULL,
    trigger_message_id  TEXT,
    reason_code         TEXT NOT NULL,
    target_entity_type  TEXT NOT NULL DEFAULT '',
    target_entity_ref   TEXT NOT NULL DEFAULT '',
    escalation_reason   TEXT NOT NULL DEFAULT '',
    source              TEXT NOT NULL DEFAULT 'model',
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (draft_id)
);

CREATE INDEX ai_kb_gap_events_org_created_idx ON ai_kb_gap_events(organization_id, created_at);
CREATE INDEX ai_kb_gap_events_org_reason_idx ON ai_kb_gap_events(organization_id, reason_code);
CREATE INDEX ai_kb_gap_events_org_entity_idx ON ai_kb_gap_events(organization_id, target_entity_type, target_entity_ref);

-- ai_kb_gap_missing_fields is a child table rather than a comma-separated
-- column so a single field name can be queried/counted directly (e.g. "how
-- often is price the missing field") without parsing text.
CREATE TABLE ai_kb_gap_missing_fields (
    id          TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    event_id    TEXT NOT NULL REFERENCES ai_kb_gap_events(id) ON DELETE CASCADE,
    field_name  TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (event_id, field_name)
);

CREATE INDEX ai_kb_gap_missing_fields_event_idx ON ai_kb_gap_missing_fields(event_id);
