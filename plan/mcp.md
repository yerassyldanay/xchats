# MCP Connector, Authentication, and KB Approval

[`DECISIONS.md`](DECISIONS.md) and the eval-canonical schema are authoritative.
This document proposes the first production implementation for
`https://xchats.kz/mcp`. File upload and material-management tools are out of
scope.

## Outcome

Claude, ChatGPT, and other compatible hosts can read the live and pending KB,
propose partial changes, and render approval widgets in chat. A model can never
approve its own change.

```text
model proposes → human applies to kbd_draft → human publishes to live ai_*
```

Two approvals are intentional: the first protects the shared draft; the second
protects customer-facing knowledge.

The first approval is an MCP-specific ingress gate, not a second lifecycle on
`kbd_draft`: it closes when the external proposal is accepted or rejected.
The existing first-party builder keeps its current single draft-to-live gate.
Record this external-connector exception in `DECISIONS.md` before implementation.

## Authentication and authorization

Use OAuth 2.1 authorization code flow with PKCE (`S256`). Use a maintained OAuth
implementation; do not implement protocol or cryptography from scratch.

1. The host calls `/mcp` without a token.
2. xchats returns `401` with `WWW-Authenticate` pointing to protected-resource
   metadata.
3. The host opens `/oauth/authorize` in a browser.
4. The existing `xchats_session` cookie identifies an already logged-in user;
   otherwise the user signs in with the existing email/password page.
5. xchats shows requested scopes and consent, then returns a one-time code.
6. The host exchanges the code at `/oauth/token` and stores access/refresh
   tokens.
7. Every MCP call sends `Authorization: Bearer <access-token>`.

The web cookie is never given to the host. Use short-lived signed JWT access
tokens and rotating opaque refresh tokens stored only as hashes. Validate
signature, issuer, audience=`https://xchats.kz/mcp`, expiry, scopes, user status,
and current organization membership on every call.

Required endpoints:

```text
POST /mcp
GET  /.well-known/oauth-protected-resource
GET  /.well-known/oauth-authorization-server
GET  /oauth/authorize
POST /oauth/token
POST /oauth/revoke
POST /oauth/register
GET  /oauth/jwks.json
```

Prefer Client ID Metadata Documents; keep dynamic client registration as a
rate-limited fallback for compatible hosts. Validate exact redirect URIs and
protect metadata fetching from SSRF.

Scopes:

```text
kb:read          read live, pending, effective, and diff views
kb:draft:write   create proposals and apply an approved proposal
kb:publish       preview and publish the reviewed draft
```

## Storage additions

Add OAuth client/grant, authorization-code, and hashed refresh-token tables.
Add:

```text
kb_change_proposals
  id, organization_id, actor_user_id, entity_kind, entity_key, operation,
  before_json, after_json, draft_version, status, expires_at, created_at

kb_publish_reviews
  id, organization_id, actor_user_id, draft_version, draft_hash,
  diff_json, status, expires_at, created_at
```

Approval secrets are one-time, hashed, and bound to the OAuth user,
organization, exact content, and draft version. Send the plaintext secret only
in widget-only tool-result `_meta`; never expose it to the model or transcript.

Before MCP writes are enabled, extend `kbd_draft` to the complete canonical row
shape, including `in_stock`, `sales_status`, and delivery zones. This is needed
because every MCP write must pass through the draft; zones must no longer be
live-only.

## MCP tools

The initial contract has 18 tools: seven reads, seven proposal tools, and four
workflow tools. `organization_id` and `user_id` always come from the token.

Common read parameter:

```text
view: live | draft | effective | diff
```

`effective` means live rows overlaid with pending draft changes. Collection
reads also accept their natural key, `query`, `limit` (max 100), and `cursor`.

| Tool | Parameters beyond `view` | Responsibility |
|---|---|---|
| `kb_assistant_read` | — | Read assistant configuration. |
| `kb_topics_read` | `slug?`, `query?`, `limit?`, `cursor?` | Read/search topics. |
| `kb_products_read` | `ref?`, `query?`, `limit?`, `cursor?` | Read/search products. |
| `kb_tariffs_read` | `ref?`, `query?`, `limit?`, `cursor?` | Read/search tariffs. |
| `kb_contacts_read` | — | Read the contacts singleton. |
| `kb_policies_read` | — | Read the policies singleton. |
| `kb_delivery_zones_read` | `ref?`, `query?`, `limit?`, `cursor?` | Read/search zones. |

Every proposal receives `expected_draft_version`. `create` requires a missing
natural key, `patch` preserves omitted fields, and `delete` stages deletion.
`null` clears a nullable field; empty/whitespace-only scalar values are rejected.

| Tool | Parameters | Responsibility |
|---|---|---|
| `kb_assistant_propose_change` | `changes{persona?, mission?, guardrails?, language_policy?, reply_max_words?}` | Patch the assistant singleton. |
| `kb_topic_propose_change` | `operation`, `slug`, `changes{title?, body_md?}` | Create, patch, or delete a topic. |
| `kb_product_propose_change` | `operation`, `ref`, `changes{name?, price?, description?, category?, in_stock?, sales_status?}` | Create, patch, or delete a product. `in_stock` is required on create. |
| `kb_tariff_propose_change` | `operation`, `ref`, `changes{name?, price?, limit_text?, fee?, summary?, pricing_type?, advantages?, disadvantages?, sales_status?}` | Create, patch, or delete a tariff. |
| `kb_contacts_propose_change` | `operation=upsert|delete`, `changes{whatsapp?, email?, address?, legal_information?, callback_time?, working_hours?, phone?, website?, instagram?}` | Change the contacts singleton. |
| `kb_policies_propose_change` | `operation=upsert|delete`, `changes{delivery_cost?, delivery_in_days?, free_delivery_from?, min_order?, prepayment?, installment?, return_period_in_days?, warranty?, outside_zones_note?}` | Change the policies singleton. |
| `kb_delivery_zone_propose_change` | `operation`, `ref`, `changes{name?, zone_level?, parent_ref?, delivery_available?, delivery_cost?, delivery_in_days?, notes?, sales_status?}` | Create, patch, or delete a zone. `delivery_available` is required on create. |

Media columns are returned as user-readable attachment summaries but are not
accepted in proposal parameters until file support is implemented.

Workflow tools:

| Tool | Visibility | Parameters | Responsibility |
|---|---|---|---|
| `kb_draft_apply_change` | app only | `proposal_id`, `approval_token` | Apply the exact reviewed proposal to `kbd_draft`. |
| `kb_draft_reject_change` | app only | `proposal_id`, `approval_token`, `reason?` | Reject without changing the draft. |
| `kb_publish_preview` | model | `expected_draft_version` | Validate the complete effective KB and render the publish diff. |
| `kb_publish` | app only | `publish_review_id`, `approval_token` | Revalidate the same draft hash and publish it to live `ai_*`. |

## Approval UI

Return one shared MCP Apps resource, for example
`ui://xchats/kb-review.html`, from proposal and publish-preview results. The
widget shows exact before/after values, warnings, and Approve/Reject buttons.
It calls app-only tools through `tools/call`; those tools use
`visibility: ["app"]`, so the model cannot invoke them.

Keep the authoritative proposal and review state on the server. Widget state is
presentation-only. If `base_version` or the draft hash changed, reject with a
stale error and render a fresh diff.

## Validation and audit

- Enforce migration `0012` names only: no `lang`, legacy aliases, or mutable
  slugs for contacts/policies.
- Require literal booleans for `in_stock` and `delivery_available`.
- Reject duplicate natural keys and warn about similar names.
- Validate tariff enums, zone parent chains, and the zone/policy exclusivity
  rules before draft apply and again before publish.
- Publish in one transaction, clear the published delta, invalidate the KB
  cache, reload the response brain, and append `ai_audit_log`.
- Redact tokens and approval secrets from logs and MCP results.

## Implementation order

1. Make `kbd_draft` canonical and add delivery-zone draft support.
2. Add proposal/review persistence and deterministic diff/validation services.
3. Add OAuth discovery, authorization, token issuance, rotation, and revocation.
4. Add the authenticated Streamable HTTP MCP server and 18 tools.
5. Add the shared MCP Apps approval widget.
6. Test with MCP Inspector, ChatGPT, and Claude: auth, tenant isolation,
   duplicate/stale changes, both approvals, revocation, and failed validation.

## Protocol references

- MCP authorization: <https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization>
- MCP Apps: <https://modelcontextprotocol.io/extensions/apps/overview>
- ChatGPT authentication: <https://developers.openai.com/plugins/build/auth>
- ChatGPT MCP UI: <https://developers.openai.com/plugins/build/chatgpt-ui>
