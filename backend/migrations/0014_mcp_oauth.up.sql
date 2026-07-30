-- 0014_mcp_oauth — the OAuth 2.1 authorization-server state for the MCP
-- connector (plan/mcp.md §3 "Authentication"). Four tables, all mcp_-
-- prefixed (a fourth lifecycle group alongside ai_/kbd_/rp_ — this state
-- belongs to neither the live KB nor its draft; it is connector-session
-- plumbing, closer to xchats.sessions than to anything KB-shaped):
--
--   mcp_oauth_clients      — registered OAuth clients (Dynamic Client
--                            Registration and/or a cached Client ID Metadata
--                            Document — plan/mcp.md's CIMD support).
--   mcp_authorization_codes — short-lived PKCE authorization codes, single-use.
--   mcp_refresh_tokens      — long-lived, rotated-on-use refresh tokens.
--   mcp_access_token_denylist — jti blacklist for revoking an otherwise
--                            stateless, self-contained signed access token
--                            before its exp.
--
-- Secrets (the authorization code / refresh token themselves) are never
-- stored in plaintext — only their sha256 hash, the same discipline
-- xchats.sessions applies to nothing (session ids are low-value, short-TTL,
-- DB-generated) but a bearer-style OAuth credential deserves: a leaked DB
-- backup must not itself hand out working tokens.
SET search_path = xchats, public;

CREATE TABLE xchats.mcp_oauth_clients (
    client_id                  text PRIMARY KEY,
    client_name                text NOT NULL DEFAULT '',
    redirect_uris               text[] NOT NULL DEFAULT '{}',
    registration_source         text NOT NULL DEFAULT 'dcr', -- 'dcr' | 'cimd'
    token_endpoint_auth_method  text NOT NULL DEFAULT 'none', -- public clients only (PKCE-secured)
    metadata                    jsonb NOT NULL DEFAULT '{}'::jsonb, -- raw client metadata doc, for debugging
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now(),
    CHECK (registration_source IN ('dcr', 'cimd'))
);

CREATE TABLE xchats.mcp_authorization_codes (
    code_hash             text PRIMARY KEY, -- sha256(code), hex
    client_id             text NOT NULL REFERENCES xchats.mcp_oauth_clients(client_id) ON DELETE CASCADE,
    redirect_uri          text NOT NULL,
    code_challenge        text NOT NULL,
    code_challenge_method text NOT NULL DEFAULT 'S256',
    user_id               uuid NOT NULL REFERENCES xchats.users(id) ON DELETE CASCADE,
    organization_id       uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    scope                 text NOT NULL DEFAULT '',
    resource              text NOT NULL DEFAULT '',
    expires_at            timestamptz NOT NULL,
    consumed_at           timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    CHECK (code_challenge_method = 'S256')
);
CREATE INDEX mcp_authorization_codes_expires_idx ON xchats.mcp_authorization_codes(expires_at);

CREATE TABLE xchats.mcp_refresh_tokens (
    token_hash      text PRIMARY KEY, -- sha256(token), hex
    client_id       text NOT NULL REFERENCES xchats.mcp_oauth_clients(client_id) ON DELETE CASCADE,
    user_id         uuid NOT NULL REFERENCES xchats.users(id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES xchats.organizations(id) ON DELETE CASCADE,
    scope           text NOT NULL DEFAULT '',
    resource        text NOT NULL DEFAULT '',
    expires_at      timestamptz NOT NULL,
    revoked_at      timestamptz,
    replaced_by     text, -- the rotated-in token_hash, if any (audit trail only)
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX mcp_refresh_tokens_client_user_idx ON xchats.mcp_refresh_tokens(client_id, user_id);

-- Every issued access token's jti is NOT recorded here by default (that would
-- make the token store-backed, defeating the point of a self-contained JWT);
-- a row here is an EXPLICIT early revocation (POST /oauth/revoke, user logout,
-- org/user deactivation) that must be honored before the token's own exp.
CREATE TABLE xchats.mcp_access_token_denylist (
    jti        text PRIMARY KEY,
    expires_at timestamptz NOT NULL, -- mirrors the token's own exp, so a cleanup job can prune safely
    revoked_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX mcp_access_token_denylist_expires_idx ON xchats.mcp_access_token_denylist(expires_at);
