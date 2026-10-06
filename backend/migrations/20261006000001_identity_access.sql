-- 1/5 Identity & access: organizations, users and membership, sessions, the MCP OAuth
-- tables, and the default organization and admin. Files 2-5 depend only on this one.
-- Plain SQL, executed verbatim and identically on SQLite and PostgreSQL (portable types and
-- rules: docs/database.md). Idempotent; the runner owns the transaction.

-- timezone is an IANA zone name, only a display default for the campaign quiet-hours picker:
-- stored windows are always UTC.
CREATE TABLE IF NOT EXISTS organizations (
    id                   TEXT PRIMARY KEY NOT NULL,
    name                 TEXT NOT NULL,
    respond_mode         TEXT NOT NULL DEFAULT 'NEVER',
    respond_window_start TEXT,
    respond_window_end   TEXT,
    created_at           BIGINT NOT NULL,
    updated_at           BIGINT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'Asia/Almaty'
);

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL DEFAULT '',
    created_at    BIGINT NOT NULL,
    updated_at    BIGINT NOT NULL,
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS mcp_oauth_clients (
    client_id                    TEXT PRIMARY KEY NOT NULL,
    client_name                  TEXT NOT NULL DEFAULT '',
    redirect_uris                TEXT NOT NULL DEFAULT '[]',
    registration_source          TEXT NOT NULL DEFAULT 'dcr' CHECK (registration_source IN ('dcr','cimd')),
    token_endpoint_auth_method   TEXT NOT NULL DEFAULT 'none',
    metadata                     TEXT NOT NULL DEFAULT '{}',
    created_at                   BIGINT NOT NULL,
    updated_at                   BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS mcp_access_token_denylist (
    jti          TEXT PRIMARY KEY NOT NULL,
    expires_at   BIGINT NOT NULL,
    revoked_at   BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS mcp_access_token_denylist_expires_idx ON mcp_access_token_denylist(expires_at);

CREATE TABLE IF NOT EXISTS organization_users (
    organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin','member')),
    joined_at       BIGINT NOT NULL,
    PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE IF NOT EXISTS sessions (
    id                      TEXT PRIMARY KEY NOT NULL,
    user_id                 TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at              BIGINT NOT NULL,
    expires_at              BIGINT NOT NULL,
    active_organization_id  TEXT REFERENCES organizations(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions(user_id);

CREATE TABLE IF NOT EXISTS mcp_authorization_codes (
    code_hash              TEXT PRIMARY KEY NOT NULL,
    client_id              TEXT NOT NULL REFERENCES mcp_oauth_clients(client_id) ON DELETE CASCADE,
    redirect_uri            TEXT NOT NULL,
    code_challenge           TEXT NOT NULL,
    code_challenge_method   TEXT NOT NULL DEFAULT 'S256' CHECK (code_challenge_method = 'S256'),
    user_id                 TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id         TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scope                   TEXT NOT NULL DEFAULT '',
    resource                TEXT NOT NULL DEFAULT '',
    expires_at               BIGINT NOT NULL,
    consumed_at              BIGINT,
    created_at                BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS mcp_authorization_codes_expires_idx ON mcp_authorization_codes(expires_at);

CREATE TABLE IF NOT EXISTS mcp_refresh_tokens (
    token_hash        TEXT PRIMARY KEY NOT NULL,
    client_id         TEXT NOT NULL REFERENCES mcp_oauth_clients(client_id) ON DELETE CASCADE,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scope             TEXT NOT NULL DEFAULT '',
    resource          TEXT NOT NULL DEFAULT '',
    expires_at        BIGINT NOT NULL,
    revoked_at        BIGINT,
    replaced_by       TEXT,
    created_at        BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS mcp_refresh_tokens_client_user_idx ON mcp_refresh_tokens(client_id, user_id);

-- Required initial state; replay never overwrites an operator's changes.
-- The default admin signs in with the documented default password and is forced to change it.
INSERT INTO organizations (id, name, respond_mode, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'xchats', 'NEVER', 1791244800000, 1791244800000) ON CONFLICT DO NOTHING;
INSERT INTO users (id, email, password_hash, display_name, must_change_password, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000002', 'admin@xchat.kz',
'$argon2id$v=19$m=65536,t=1,p=4$eZE9z7aFgeOEeYVAUCJTxg$3x3PW6uhMxX+nhuXZZZ79JQOKAoImKMB/ACkGsqq9io',
'Admin', TRUE, 1791244800000, 1791244800000) ON CONFLICT DO NOTHING;
INSERT INTO organization_users (organization_id, user_id, role, joined_at)
VALUES ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000002', 'admin', 1791244800000) ON CONFLICT DO NOTHING;
