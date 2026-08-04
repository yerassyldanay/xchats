-- xchats SQLite schema, part 1: identity & organization.
--
-- Ported from the frozen Postgres history (migrations/postgres/0001-0016,
-- captured as migrations/postgres/SCHEMA.snapshot.sql +
-- migrations/sqlite/schema_contract.json — see plan/*sqlite-cutover* for the
-- full per-column translation table this and the other sqlite/*.up.sql
-- files apply). Conventions used throughout this file and its siblings:
--
--   * No "xchats." schema prefix — SQLite has one namespace per database
--     file, so there is nothing to prefix.
--   * uuid -> TEXT. Every uuid PRIMARY KEY that had a Postgres
--     uuid_generate_v4() default gets the same DB-side default here (so
--     "INSERT ... RETURNING id" sites that omit id are unchanged) via the
--     expression below — a lowercase, RFC-4122-shaped v4 UUID built from
--     SQLite's own random(): 8-4-4-4-12 hex digits, version nibble '4',
--     variant nibble one of 8/9/a/b. Pinned by
--     internal/dbx/uuiddefault_test.go, which must stay byte-for-byte
--     identical to every copy pasted below.
--   * timestamptz -> TEXT, the canonical format internal/dbx.TimeLayout
--     enforces on every Go-bound time.Time ("2006-01-02 15:04:05.000",
--     always UTC, always 3 fractional digits). now() -> SQL-side
--     strftime('%Y-%m-%d %H:%M:%f','now'), which produces that exact same
--     format/width (pinned by internal/dbx/dbtime_test.go).
--   * jsonb -> TEXT + a json_valid CHECK (nullable columns allow NULL too).
--   * citext -> TEXT COLLATE NOCASE (SQLite applies a column's declared
--     collation to comparisons AND to any UNIQUE constraint/index on it, so
--     this alone reproduces citext's case-insensitive uniqueness).
--
-- This file: organizations, organization_users, users, sessions — the
-- group with no forward references to any later file.

CREATE TABLE organizations (
    id                   TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    name                 TEXT NOT NULL,
    respond_mode         TEXT NOT NULL DEFAULT 'NEVER',
    respond_window_start TEXT, -- 'HH:MM:SS'
    respond_window_end   TEXT, -- 'HH:MM:SS'
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

CREATE TABLE users (
    id            TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    email         TEXT NOT NULL COLLATE NOCASE UNIQUE,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

CREATE TABLE organization_users (
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE sessions (
    id                      TEXT PRIMARY KEY NOT NULL,
    user_id                 TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at              TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    expires_at              TEXT NOT NULL,
    active_organization_id  TEXT REFERENCES organizations(id) ON DELETE SET NULL
);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);
