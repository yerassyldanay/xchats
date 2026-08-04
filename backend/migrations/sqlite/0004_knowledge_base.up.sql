-- xchats SQLite schema, part 4: the Playground draft blob + material
-- library + builder requests. See 0001_core.up.sql for the shared
-- conventions. kbd_materials is what 0003_ai_engine.up.sql's
-- featured/contact-card/location-map columns forward-reference.

CREATE TABLE kbd_draft (
    organization_id  TEXT PRIMARY KEY NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    draft            TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(draft)),
    base_version     INTEGER NOT NULL DEFAULT 0,
    updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_by       TEXT REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE kbd_materials (
    id                    TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id       TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    source_type           TEXT NOT NULL,
    source_ref            TEXT NOT NULL DEFAULT '',
    blob_id               TEXT NOT NULL DEFAULT '',
    extracted_text        TEXT NOT NULL DEFAULT '',
    media_kind            TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'pending',
    extraction            TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(extraction)),
    created_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    source_text           TEXT,
    operator_note         TEXT,
    storage_backend       TEXT,
    storage_key           TEXT,
    filename              TEXT,
    mime_type             TEXT,
    size_bytes            INTEGER,
    sha256_checksum       TEXT,
    visual_summary        TEXT,
    transcript_text       TEXT,
    extraction_metadata   TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(extraction_metadata)),
    processing_status     TEXT NOT NULL DEFAULT 'uploaded',
    customer_visibility   TEXT
);
CREATE INDEX kbd_materials_org_idx ON kbd_materials(organization_id);
CREATE INDEX kbd_materials_org_status_idx ON kbd_materials(organization_id, processing_status);

CREATE TABLE kbd_requests (
    id                TEXT PRIMARY KEY NOT NULL DEFAULT (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)),2) || '-' || substr('89ab',abs(random()) % 4 + 1,1) || substr(hex(randomblob(2)),2) || '-' || hex(randomblob(6)))),
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    material_id       TEXT REFERENCES kbd_materials(id) ON DELETE SET NULL,
    req_type          TEXT NOT NULL,
    prompt            TEXT NOT NULL DEFAULT '',
    context           TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(context)),
    target            TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(target)),
    state             TEXT NOT NULL DEFAULT 'pending',
    resolution        TEXT CHECK (resolution IS NULL OR json_valid(resolution)),
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    resolved_at       TEXT
);
CREATE INDEX kbd_requests_org_idx ON kbd_requests(organization_id, state);
