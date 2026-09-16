# MCP connector

xchats exposes one [Model Context Protocol](https://modelcontextprotocol.io)
server that lets ChatGPT, Claude, or any other MCP client read and (for the
knowledge base only) stage changes to a running instance over OAuth 2.1. It
has two tool groups:

- **Knowledge base tools** (`kb_*`, 14 tools) — read the live/draft KB and
  stage edits for human review. See the in-app `/draft` page.
- **Campaign diagnostics tools** (4 tools, this document's main subject) —
  explain what happened to a WhatsApp campaign and its recipients: whether a
  send was attempted, whether the provider accepted it, whether delivery is
  confirmed, what failed, whether a retry ran, and which conversation holds
  the message. **Read-only**: no diagnostic tool sends a message, retries a
  campaign, reconnects an account, or otherwise mutates application state.

Both groups share one server (`backend/internal/mcpserver`), one OAuth 2.1
authorization server (`backend/internal/mcpauth`), and one connector URL —
an LLM client authorizes once and is granted exactly the scopes it (or the
person approving the consent screen) requested.

## Quick start (in-app)

The fastest way to connect ChatGPT or Claude to your own instance is the
**"Connect ChatGPT or Claude"** card on the `/draft` page: it shows the
connector URL for your deployment, has one-click links into ChatGPT's and
Claude's own connector settings, and (for an admin, when a public tunnel
isn't already configured) can start one. This section exists for everything
that card doesn't cover: starting the server, environment variables,
transports, and how to test the connector from a terminal.

## Starting the server

The MCP connector starts automatically with the backend (`go run
./cmd/xchats`, `make dev-backend`, or the Docker image) — there is no
separate process and no feature flag to turn it on. What differs by
configuration is whether it is *durable* across a restart:

- **`MCP_JWT_SIGNING_KEY`** (env, secret) seeds the Ed25519 key that signs
  every access/refresh token. If it is unset, the server logs a loud warning
  and falls back to an ephemeral in-process key: the connector still works,
  but every token issued before a restart stops verifying afterward, and a
  multi-instance deployment would mint tokens no sibling instance can check.
  Set this to a 64-character hex string (or a 32-character literal, or
  base64) for anything beyond a single local process.
- **`API_BASE_URL`** (env, or `server.api_base_url` in `config.yaml`) is the
  public HTTPS origin the connector URL is built from
  (`<API_BASE_URL>/mcp`). Unset, it falls back to
  `http://localhost:<http_addr port>` — fine for a same-machine client, not
  reachable from ChatGPT's or Claude's own servers.
- **`FRONTEND_BASE_URL`** (env, or `server.frontend_base_url`) is used to
  build the KB Manager widget's "Review and publish in Xchats" link and the
  OAuth consent page's branding; it does not affect the diagnostics tools.

None of the campaign-diagnostics tools need configuration of their own —
they read through the same `*store.Store` and `whatsapp.Manager` the rest of
the backend already uses.

### Optional tunables (all have working defaults)

| Env var | Meaning | Default |
|---|---|---|
| `MCP_ACCESS_TOKEN_TTL_SECONDS` | Access token lifetime | 900 (15 min) |
| `MCP_REFRESH_TOKEN_TTL_DAYS` | Refresh token lifetime | 30 days |
| `MCP_AUTH_CODE_TTL_SECONDS` | PKCE authorization-code lifetime | 300 |
| `MCP_UPLOAD_TOKEN_TTL_SECONDS` | KB media signed-upload URL lifetime | 900 |
| `MCP_MEDIA_TOKEN_TTL_SECONDS` | KB media signed-read URL lifetime | 3600 |
| `MCP_REVIEW_HANDOFF_TTL_SECONDS` | One-time "review in Xchats" link lifetime | 300 |

None of these are secrets; they are safe to commit in `config.yaml`.

## Authentication and authorization

The connector is an OAuth 2.1 authorization server with PKCE, Dynamic Client
Registration (`POST /oauth/register`), and support for
[Client ID Metadata Documents](https://modelcontextprotocol.io) as a
registration-free alternative. A human logs into xchats and approves a
consent screen naming the exact scopes the client asked for and the exact
organization the grant applies to — a client never supplies its own
organization id, and nothing downstream trusts one if it tried.

Scopes (a client requests exactly the ones it needs; there is no wildcard):

| Scope | Grants |
|---|---|
| `kb:read` | `kb_read`, `kb_summary`, `kb_info` |
| `kb:draft:write` | Every `kb_*_upsert` tool, `kb_delete`, `kb_media_attach` |
| `media:write` | `kb_media_upload` (app/widget-invoked only) |
| `campaigns:read` | `diagnose_campaign`, `campaign_recipients`, `whatsapp_connection_status`, `campaign_events` |

`campaigns:read` is deliberately its own scope: granting it never implies
`kb:read` or vice versa, since campaign/recipient/connection data is a
different resource domain from knowledge-base content. No scope grants
mutation of campaign or connection state — that capability does not exist
in this connector at all.

Every campaign-diagnostics call re-derives the organization from the
verified access token, never from a request parameter, and checks that the
requested campaign/account belongs to that organization before returning
anything. A campaign or account that exists in a *different* organization
produces the exact same "not found" response as one that does not exist at
all — a client can never learn that a resource exists somewhere else by
probing IDs. Recipient identities are masked (all but the last 4
characters) and every ID returned is this application's own opaque UUID —
no raw provider payload, message body, credential, token, or session
secret is ever returned or persisted through this surface.

## Transport

Streamable HTTP only (`POST`/`GET`/`DELETE /mcp`), matching the current MCP
specification's recommended transport. There is no stdio transport and no
legacy HTTP+SSE transport — a client must support Streamable HTTP.

Discovery endpoints (unauthenticated):

- `GET /.well-known/oauth-protected-resource` (and `/mcp` suffix variant)
- `GET /.well-known/oauth-authorization-server`

OAuth endpoints:

- `GET /oauth/authorize`, `POST /oauth/authorize/decision` (human consent)
- `POST /oauth/token` (code/refresh exchange)
- `POST /oauth/register` (Dynamic Client Registration)

## Verified Claude configuration

Claude (claude.ai → Settings → Connectors → Add custom connector) speaks
Streamable HTTP with OAuth discovery natively: paste the connector URL
(`https://your-domain/mcp`, or the URL the `/draft` page's connect card
shows for your deployment), and Claude discovers the authorization server,
registers itself via DCR, and opens the consent screen. There is no manual
JSON to write.

What "verified" means here: the exact protocol sequence a Claude connection
performs — discovery, DCR, PKCE authorize/decision, token exchange, then
`tools/list` and `tools/call` — is exercised end to end by this repository's
own automated test suite (`backend/internal/httpapi/mcp_integration_test.go`,
`TestMCPOAuthFullFlow_ThroughToolsCall` and neighboring tests), against this
server's real implementation of each endpoint. This is protocol-level
verification, not a manual click-through against the live claude.ai product
— this repository does not claim to have done the latter.

## Verified ChatGPT connection instructions

ChatGPT (chatgpt.com → Settings → Connectors → Create) uses the same
discovery + DCR + PKCE flow. Requirements specific to ChatGPT, both already
satisfied by this server and worth knowing if something looks wrong:

- `securitySchemes` on every scope-gated tool must serialize as a JSON
  **array** of `{"type": "oauth2", "scopes": [...]}` objects, not a keyed
  object — ChatGPT rejects the whole `tools/list` response otherwise. This
  repository has a standing regression test for exactly that shape
  (`TestToolsList_SecuritySchemesSerializeAsJSONArray`).
- ChatGPT requires a **public HTTPS** origin — `http://localhost:...` is not
  reachable from ChatGPT's own servers. If you are not deploying behind a
  real domain, use the `/draft` page's tunnel button (admin only, requires
  an ngrok credential under Settings → Integrations) to get a temporary
  public HTTPS URL for testing.

As with Claude, "verified" means the underlying protocol behavior ChatGPT
depends on (the array-shaped `securitySchemes`, the OAuth sequence, scope
enforcement) is covered by this repository's automated tests — not a manual
session against the live chatgpt.com product from this environment.

## Generic MCP client configuration

Any client implementing MCP's Streamable HTTP transport with OAuth 2.1 +
PKCE discovery works the same way: point it at `<API_BASE_URL>/mcp` and let
it follow the discovery documents. There is no client-specific server-side
configuration — the same connector, same URL, same OAuth server answers
every client identically. A client that only supports a static bearer token
(no OAuth) is not supported; there is no such path in this server.

## Local testing (no ChatGPT/Claude account needed)

With the backend running (`make dev-backend`, default `http://localhost:8080`,
API prefix `/xchats/api/v1`):

```bash
# 1. Register a client (Dynamic Client Registration) — no auth required.
curl -s -X POST http://localhost:8080/oauth/register \
  -H 'Content-Type: application/json' \
  -d '{"redirect_uris": ["http://localhost:9999/callback"], "client_name": "local-test"}'
# -> {"client_id": "...", ...}

# 2. Log in to xchats normally in a browser (or via POST /xchats/api/v1/auth/login
#    with a cookie jar) so you have an authenticated session, then visit
#    GET /oauth/authorize?client_id=<id>&redirect_uri=http://localhost:9999/callback
#    &response_type=code&code_challenge=<S256 of a verifier>&code_challenge_method=S256
#    &state=xyz&scope=campaigns:read
#    and approve the consent screen for the organization you want to query.
#    The redirect carries ?code=... — copy it.

# 3. Exchange the code for an access token.
curl -s -X POST http://localhost:8080/oauth/token \
  -d grant_type=authorization_code -d code=<code> \
  -d redirect_uri=http://localhost:9999/callback \
  -d client_id=<id> -d code_verifier=<verifier>
# -> {"access_token": "...", "refresh_token": "...", ...}

# 4. Call a tool.
curl -s -X POST http://localhost:8080/mcp \
  -H "Authorization: Bearer <access_token>" \
  -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"diagnose_campaign","arguments":{"campaign_id":"<uuid>"}}}'
```

Steps 2–3 are the part a real MCP client automates for you; this walkthrough
exists so the connector can be smoke-tested from a terminal without
installing ChatGPT or Claude. `backend/internal/httpapi/mcp_integration_test.go`
runs the exact same sequence programmatically on every test run.

## Campaign diagnostics tools

| Tool | Purpose |
|---|---|
| `diagnose_campaign(campaign_id)` | One campaign's lifecycle status, non-overlapping recipient outcome counts, delivery/read counts, failures grouped by structured reason, a retry summary, the sending account's connection history/status, recent events, and time-correlated likely causes. |
| `campaign_recipients(campaign_id, status?, cursor?, limit?)` | Paginated recipients with masked identity, sending outcome, delivery status, latest-attempt error, retry state, and linked chat/message. |
| `whatsapp_connection_status(account_id)` | An account's current connection state (live-checked when possible), freshness, recent connect/disconnect history, and whether it looks ready to send right now. |
| `campaign_events(campaign_id, cursor?, limit?)` | The campaign's persisted diagnostic timeline (lifecycle, per-attempt, retry, delivery-receipt events), plus the sending account's connection events relevant to that campaign's own active window. |

Every tool's full input/output JSON Schema is served by this connector's own
`tools/list` — this table is an index, not the contract. Pagination follows
the same opaque decimal-string cursor the `kb_read`/`kb_summary` tools
already use: pass `next_cursor` from one call as `cursor` on the next; its
absence means the last page. `limit` defaults to 50 and caps at 100.

### Sending and delivery status definitions

These tools distinguish **sending outcome** (did we get the message to the
provider) from **delivery evidence** (did the provider confirm the
recipient's device got it) — the second is never inferred from the first.

| `campaign_recipients` outcome | Meaning |
|---|---|
| `pending` | Not yet attempted (or a retry is scheduled — see `retry_state`). |
| `sending` | An attempt is currently in flight. Should be transient. |
| `sent` | The provider **accepted** the message on at least one attempt. This is the existing `sent` status, defined here precisely: **provider acceptance, not confirmed delivery.** Creating or queueing a local message is never treated as proof of sending, and provider acceptance is never reported as confirmed delivery. |
| `failed` | Reached a terminal state — either a permanent cause, or the retry ladder was exhausted, or the outcome is genuinely unresolved (see below). |
| `skipped` | Suppressed, typically because the recipient replied or was messaged out of band during the campaign. |

Delivery/read evidence is reported **separately** and only from real
provider receipts:

- `campaign_recipients[].delivery_status`: the linked message's own
  `queued`/`sent`/`delivered`/`read`/`failed` state. `null` when no message
  is linked yet.
- `diagnose_campaign` counts `delivered`/`read` recipients distinctly from
  `sent` (provider-accepted). **Zero delivered/read does not mean confirmed
  non-delivery** — it may simply mean this channel/account has never
  reported a receipt. The tools say so explicitly rather than implying it.

**Unknown outcome** is a first-class, separately reported value, never
folded into "success" or "confirmed failure": a send that timed out, or was
interrupted by a crash after submission, may never learn whether the
provider actually received it. `diagnose_campaign.counts.unknown_outcome`
and `campaign_recipients(status="unknown")` both surface exactly these
recipients. Their coarse `outcome` still reads `failed` (the existing
five-value vocabulary is unchanged), but every unknown-outcome recipient is
counted in `unknown_outcome` too, specifically so it is never silently read
as either a success or a confirmed failure.

Campaign-level execution status (`diagnose_campaign.status`:
`draft`/`scheduled`/`running`/`paused`/`completed`/`failed`/`cancelled`) is
reported separately from recipient outcomes — a `completed` campaign does
**not** imply every message was delivered, or even that every message was
sent; check `counts`/`delivery` for that.

### Retry and unknown-outcome handling

`diagnose_campaign.retry_summary` and each recipient's `retry_state` explain
what the retry ladder actually did:

- **`recovered`** — now provider-accepted, but only after at least one
  prior failed attempt.
- **`scheduled`** — currently pending with an automatic retry queued
  (`next_retry_at` is set).
- **`exhausted`** — failed because the bounded retry ladder was fully used.
  The **original cause is preserved** — an exhausted recipient's
  `failures_by_reason` entry is the real underlying error code (e.g.
  `whatsapp_disconnected`), never a generic "max retries exceeded" that
  replaces it.
- **`unresolved`** (== `unknown_outcome`) — an ambiguous timeout or a
  crash-interrupted send. These are **never automatically retried**: a
  send that may have already reached the provider must not be blindly
  resent, since that could double-send. They stay `unknown` until an
  operator reconciles them through the existing operational workflow
  (checking provider evidence, or manually re-queuing after confirming
  non-delivery) — a process this read-only connector can *report on* but
  never perform.

Every failure's structured `error_code` is derived from actual adapter
evidence (a real WhatsApp/whatsmeow error type, a network timeout, …) —
never guessed from free-text error strings. A recipient whose retry
eventually succeeded shows `outcome=sent`; it never still reads `failed`
just because an earlier attempt was.

### Historical data and provider-capability limitations

- **Connection history** (`whatsapp_connection_status.history`,
  `diagnose_campaign.connection.observed_during_campaign`) is only recorded
  from the point this diagnostics feature was deployed onward. A deployment
  upgrading from an older version has no connection history for anything
  that happened before the upgrade; both tools report an empty history in
  that case rather than fabricating one, and `whatsapp_connection_status`
  names this explicitly under `missing_evidence`.
- **A current disconnect is never presented as the cause of an earlier
  failure** unless a connection event's own timestamp falls within the
  failing send's own time window — `diagnose_campaign.likely_causes` only
  cites time-correlated evidence, and `missing_evidence` says so when no
  such correlation exists.
- **Delivery/read receipts** depend entirely on the provider actually
  reporting them for a given account/message. Some accounts or message
  types may never receive a receipt; this shows as `delivered: 0` with an
  explanatory fact in `diagnose_campaign.facts`, never as a false claim of
  non-delivery.
- **Legacy recipients with no attempt record at all** (sent before this
  feature existed) show up in `failures_by_reason` grouped under the
  literal string `"unknown"` — an honest "no evidence available", not a
  guess.

## Example prompts

Once connected, these are answerable directly from the tools above:

- Why did campaign `<id>` fail? Separate confirmed facts from likely causes.
- Which recipients from campaign `<id>` failed or have an unknown sending outcome?
- Which messages were accepted by WhatsApp, and which have confirmed delivery?
- Is the account for campaign `<id>` connected now, and was it disconnected during sending?
- Show the errors, retries, and unresolved outcomes for campaign `<id>`.
- Which conversations participated in campaign `<id>`?

## A note on `.mcp.json`

The repository's own `.mcp.json` (root) configures MCP servers *for this
coding assistant's own development session* (currently `chrome-devtools`,
for browser automation while working on the frontend) — it is unrelated to
the xchats product's own MCP connector documented above, which end users
reach remotely over Streamable HTTP + OAuth, not as a local stdio process
this file's `command`/`args`/`env` shape assumes. Nothing about the
connector documented here belongs in `.mcp.json`, so it was left untouched
(the existing Chrome DevTools entry is preserved as-is).
