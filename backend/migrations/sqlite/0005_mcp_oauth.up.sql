-- xchats SQLite schema, part 5: the MCP connector's OAuth 2.1 authorization
-- server tables. See 0001_core.up.sql for the shared conventions.

CREATE TABLE mcp_oauth_clients (
    client_id                    TEXT PRIMARY KEY NOT NULL,
    client_name                  TEXT NOT NULL DEFAULT '',
    redirect_uris                TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(redirect_uris)),
    registration_source          TEXT NOT NULL DEFAULT 'dcr' CHECK (registration_source IN ('dcr','cimd')),
    token_endpoint_auth_method   TEXT NOT NULL DEFAULT 'none',
    metadata                     TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(metadata)),
    created_at                   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now')),
    updated_at                   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);

CREATE TABLE mcp_authorization_codes (
    code_hash              TEXT PRIMARY KEY NOT NULL,
    client_id              TEXT NOT NULL REFERENCES mcp_oauth_clients(client_id) ON DELETE CASCADE,
    redirect_uri            TEXT NOT NULL,
    code_challenge           TEXT NOT NULL,
    code_challenge_method   TEXT NOT NULL DEFAULT 'S256' CHECK (code_challenge_method = 'S256'),
    user_id                 TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id         TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scope                   TEXT NOT NULL DEFAULT '',
    resource                TEXT NOT NULL DEFAULT '',
    expires_at               TEXT NOT NULL,
    consumed_at              TEXT,
    created_at                TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX mcp_authorization_codes_expires_idx ON mcp_authorization_codes(expires_at);

CREATE TABLE mcp_refresh_tokens (
    token_hash        TEXT PRIMARY KEY NOT NULL,
    client_id         TEXT NOT NULL REFERENCES mcp_oauth_clients(client_id) ON DELETE CASCADE,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id   TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scope             TEXT NOT NULL DEFAULT '',
    resource          TEXT NOT NULL DEFAULT '',
    expires_at        TEXT NOT NULL,
    revoked_at        TEXT,
    replaced_by       TEXT,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX mcp_refresh_tokens_client_user_idx ON mcp_refresh_tokens(client_id, user_id);

CREATE TABLE mcp_access_token_denylist (
    jti          TEXT PRIMARY KEY NOT NULL,
    expires_at   TEXT NOT NULL,
    revoked_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now'))
);
CREATE INDEX mcp_access_token_denylist_expires_idx ON mcp_access_token_denylist(expires_at);
