-- xchats SQLite schema, part 20: Beauty Salon Knowledge Base extension
-- (PLAN.md, TEST.md). Adds a structured specialist roster and a service
-- catalog with a base/variant/addon hierarchy alongside the existing
-- ai_contacts/ai_products/ai_tariffs KB tables, plus a booking handoff URL
-- and a structured weekly schedule on ai_contacts for the salon's own
-- fallback hours.
--
-- available_spots is intentionally never added anywhere in this schema:
-- the external booking page is the only source of live availability, so
-- nothing here stores a free-text "open slots" value for the model to read
-- back or restate.
--
-- ai_contacts.working_hours is left untouched for non-salon tenants; it is
-- not parsed or backfilled into the new structured `schedule` column, which
-- is authoritative only where the salon prompt is selected (see PLAN.md).
--
-- `schedule` (here and on ai_specialists) stores the shared Schedule
-- contract from PLAN.md — a JSON array of at most 7 { ref, day, start, end,
-- breaks[] } entries, one per weekday actually worked, kept in canonical
-- Monday-first chronological order. `ref` (mon..sun) is authoritative for
-- schedule-reasoning code; `day` is just the display label captured at
-- authoring/import time. Validity beyond well-formed JSON (no duplicate
-- weekdays, valid HH:MM times, breaks contained in the shift and
-- non-overlapping, no overnight shifts) is enforced in Go
-- (internal/kbstore), matching this repo's existing convention of
-- validating structured/enum content in application code rather than in a
-- SQL CHECK — see migration 0017's comment.
--
-- ai_services.parent_ref follows ai_delivery_zones.parent_ref's precedent:
-- a plain TEXT column (empty string = no parent / base service), not a SQL
-- foreign key, because SQLite cannot express a FK into one column of a
-- composite UNIQUE(organization_id, ref) and this repo validates that kind
-- of same-organization cross-row reference in Go instead (see
-- internal/kbstore's service-hierarchy validation: variants/add-ons must
-- name a base service in the same organization, and only one hierarchy
-- level is allowed).
--
-- service_type/sales_status carry no SQL CHECK on their vocabulary, for the
-- same reason sales_status/pricing_type/availability_status don't (0017's
-- comment): validated in Go so the vocabulary can grow without a schema
-- migration.

ALTER TABLE ai_contacts ADD COLUMN booking_url TEXT NOT NULL DEFAULT '';
ALTER TABLE ai_contacts ADD COLUMN schedule TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(schedule));

CREATE TABLE ai_specialists (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref               TEXT NOT NULL,
    full_name         TEXT NOT NULL DEFAULT '',
    title             TEXT NOT NULL DEFAULT '',
    experience        TEXT NOT NULL DEFAULT '',
    schedule          TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(schedule)),
    booking_url       TEXT NOT NULL DEFAULT '',
    portfolio_images  TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(portfolio_images)),
    sales_status      TEXT NOT NULL DEFAULT 'active',
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (organization_id, ref)
);

CREATE TABLE ai_services (
    id               TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id  TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ref              TEXT NOT NULL,
    parent_ref       TEXT NOT NULL DEFAULT '',
    service_type     TEXT NOT NULL DEFAULT 'base',
    category         TEXT NOT NULL DEFAULT '',
    name             TEXT NOT NULL DEFAULT '',
    price            TEXT NOT NULL DEFAULT '',
    duration         INTEGER,
    description      TEXT NOT NULL DEFAULT '',
    specialist_refs  TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(specialist_refs)),
    sales_status     TEXT NOT NULL DEFAULT 'active',
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    UNIQUE (organization_id, ref)
);
