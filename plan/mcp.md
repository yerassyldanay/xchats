# MCP Connector for Xchats Knowledge Base

This document defines the first implementation of `https://xchats.kz/mcp`.
`DECISIONS.md` and the canonical database schema remain authoritative.

## 1. Scope

ChatGPT, Claude, and other compatible MCP hosts can:

- authenticate an existing Xchats user;
- inspect the live and draft knowledge bases;
- detect possible duplicates before writing;
- create, partially update, or delete draft records;
- show the knowledge base and draft inside a chat widget;
- upload media and attach it to supported KB fields.

MCP changes go directly to `kbd_draft`. There is no approval for every draft
change. The single safety boundary remains:

```text
MCP change → kbd_draft → human review → live ai_* tables
```

Publishing stays in the existing authenticated Xchats review page for v1. The
chat widget links to that page. An in-chat publish tool can be added later.

A second, non-MCP entry point shares this exact `kbd_draft`-only boundary:
the structured knowledge base import pipeline (`internal/kbimport`,
`POST /kb/imports` — see `plan/playground.md`) lets an operator submit a URL
or document instead of typing it into a chat. Concretely, this is Черновик's
(`/playground`) ingestion panel — its Ссылки and Файлы tabs, sitting right
next to the ChatGPT/Claude tab this document's own `kb_media_upload`/widget
flow powers, so both ways of getting content into the KB live in one place.
Its pass-2 synthesis step emits the *same* typed upsert calls this document
defines, through the shared `internal/mcpserver.ParseUpsertCall`/
`UpsertTools` seam (§10) — one contract, two callers, so an imported draft
entry is indistinguishable from one an MCP-connected model created by hand.
`kb_assistant_upsert` is never reachable from that caller: an imported
document never sets assistant persona/mission/guardrails.

## 2. Data boundaries

There are seven writable live KB tables:

| KB type | Live table | Draft section | Stable key |
|---|---|---|---|
| Assistant | `ai_assistants` | `assistant` | `main` |
| Topic | `ai_topics` | `topics` | `slug` |
| Product | `ai_products` | `products` | `ref` |
| Tariff | `ai_tariffs` | `tariffs` | `ref` |
| Contacts | `ai_contacts` | `contacts` | `main` |
| Policies | `ai_policies` | `policies` | `main` |
| Delivery zone | `ai_delivery_zones` | `delivery_zones` | `ref` |

`ai_audit_log` is not writable through MCP. Files belong to `kbd_materials`;
the removed `ai_assets` and `ai_draft_assets` tables must not be recreated.

The current `kbd_draft` shape does not contain delivery zones. Before enabling
the delivery-zone MCP tool, add `delivery_zones: []` to the draft schema,
validation, diff, materialization, and frontend types.

The draft stores deltas, not a second copy of the live KB. A draft entry with
the same natural key shadows its live row. The backend reconstructs a complete
draft row when an MCP tool submits a partial update.

## 3. Authentication

Use OAuth 2.1 authorization-code flow with PKCE (`S256`).

The browser cookie and OAuth access token have different jobs:

- The existing Xchats cookie authenticates the user only on the browser login
  and consent pages.
- The MCP host stores an OAuth access token and sends it on every MCP request:

```http
Authorization: Bearer <access-token>
```

The cookie is never given to ChatGPT or Claude. The access token may be a
short-lived signed JWT or an opaque token. A JWT should contain or resolve to
`user_id`, `organization_id`, scopes, audience, issuer, and expiry. The chosen
organization is bound during authorization; tools never accept an arbitrary
`organization_id`.

The backend must still verify that the user is active and belongs to the bound
organization on every request.

### First connection

1. The user adds `https://xchats.kz/mcp` as a connector.
2. The host calls `/mcp` without a token.
3. Xchats returns `401` with OAuth protected-resource metadata.
4. The host opens the Xchats authorization page.
5. The existing login cookie is used, or the user signs in with email/password.
6. The user selects an organization and grants the requested permissions.
7. The host exchanges the authorization code for access and refresh tokens.
8. The host stores the tokens and sends the access token with every MCP call.

Minimum scopes:

```text
kb:read
kb:draft:write
media:write
```

Required discovery and protocol surfaces:

```text
POST /mcp
GET  /.well-known/oauth-protected-resource
GET  /.well-known/oauth-authorization-server
GET  /oauth/authorize
POST /oauth/token
POST /oauth/revoke
GET  /oauth/jwks.json                 # when JWT access tokens are used
```

Support Client ID Metadata Documents. Dynamic client registration can remain a
compatibility fallback. Access tokens must be audience-bound to the MCP
resource, short-lived, revocable, and never accepted in query parameters.

## 4. Duplicate prevention

Duplicate prevention is based primarily on identity, not full record content:

- `ref` for products, tariffs, and delivery zones;
- `slug` for topics;
- `main` for singleton tables;
- title/name as a secondary comparison.

The model follows this sequence:

```text
kb_summary → possible match → kb_read exact record → upsert or create
```

The backend repeats the check because a model may skip a step:

1. Exact natural-key match means update.
2. Exact normalized title/name with another key means return a conflict.
3. Similar titles return candidates and require the user to choose.
4. No match means create.

Internal database UUIDs are not identity keys and are not exposed unless they
are required media references.

## 5. MCP tools

The initial contract has **12 tools**:

- 7 typed draft-upsert tools;
- 4 shared knowledge tools;
- 1 widget-oriented media-upload tool.

`organization_id` and `user_id` always come from the access token.
`kb_read`, `kb_summary`, `kb_info`, and mutation results may attach the shared
KB Manager resource with the appropriate initial view.

### Common write behavior

Every upsert accepts:

```text
expected_draft_version?  optimistic-concurrency value
provenance?              { source_url?, material_ids? }
```

For collection tables, the natural key is optional:

- existing key: partially update that record;
- new supplied key: create with that key;
- missing key: generate a stable key from the title/name after duplicate checks.

Omitted fields remain unchanged. Explicit `null` clears a nullable field.
Create operations must still supply all canonical required fields. The backend
merges the patch with the current draft/live row and stores a complete row in
`kbd_draft`.

### Typed upsert tools

| Tool | Main parameters | Responsibility |
|---|---|---|
| `kb_assistant_upsert` | `changes{persona?, mission?, guardrails?, language_policy?, reply_max_words?}` | Create or patch the assistant singleton in draft. |
| `kb_topic_upsert` | `slug?`, `changes{title?, body_md?, featured_image?, illustration_images?, explainer_videos?, narration_audio_files?, reference_documents?}` | Create or patch a topic. |
| `kb_product_upsert` | `ref?`, `changes{name?, price?, description?, category?, in_stock?, sales_status?, featured_image?, gallery_images?, demo_videos?, audio_description_files?, certificate_documents?, manual_documents?, guarantee_documents?, specification_documents?}` | Create or patch a product. `in_stock` is required on create. |
| `kb_tariff_upsert` | `ref?`, `changes{name?, price?, limit_text?, fee?, summary?, pricing_type?, advantages?, disadvantages?, sales_status?, featured_image?, pricing_images?, explainer_videos?, terms_documents?}` | Create or patch a tariff. `pricing_type` is required on create. |
| `kb_contacts_upsert` | `changes{whatsapp?, email?, address?, legal_information?, callback_time?, working_hours?, phone?, website?, instagram?, contact_card_image?, location_map_image?, company_legal_documents?}` | Create or patch the contacts singleton. |
| `kb_policies_upsert` | `changes{delivery_cost?, delivery_in_days?, free_delivery_from?, min_order?, prepayment?, installment?, return_period_in_days?, warranty?, outside_zones_note?, commerce_policy_documents?}` | Create or patch the policies singleton. |
| `kb_delivery_zone_upsert` | `ref?`, `changes{name?, zone_level?, parent_ref?, delivery_available?, delivery_cost?, delivery_in_days?, notes?, sales_status?}` | Create or patch a delivery zone. `delivery_available` is required on create. |

### Shared tools

#### `kb_read`

Reads complete records for the model or widget.

```text
types?    one or more KB types
source    live | draft | both (default: both)
key?      exact ref, slug, or main
query?    text search
limit?    default 50, maximum 100
cursor?   pagination cursor
```

When `source=both`, live and draft origins remain explicit. There is no
`effective` source.

#### `kb_delete`

Adds a delete marker to the draft.

```text
type
key                       ref, slug, or main
expected_draft_version?
```

The backend validates the type/key and blocks a delete that would make the
publishable KB invalid.

#### `kb_summary`

Returns a compact identity index for duplicate detection and widget lists.

```text
types?
source    live | draft | both (default: both)
query?
limit?
cursor?
```

Example result:

```json
{
  "draft_version": 12,
  "items": [
    {
      "type": "tariff",
      "key": "business",
      "title": "Business",
      "exists_in_live": true,
      "exists_in_draft": true,
      "state": "changed"
    }
  ]
}
```

It intentionally omits prices and other full content unless they are needed to
identify the record.

#### `kb_info`

Explains:

- available KB types and their natural keys;
- required and supported fields;
- duplicate-check workflow;
- draft-only mutation rule;
- media-field meanings;
- how to review and publish.

The same short rules should also appear in the MCP server instructions so the
model normally does not need to call this tool.

### Media upload tool

#### `kb_media_upload`

This widget-oriented tool creates a pending `kbd_materials` row and returns a
short-lived signed upload target. It is app-only: the widget invokes it, not
the model.

```text
filename
mime_type
size_bytes
sha256_checksum?
target?            { type, key, field }
```

The widget uploads bytes directly to object storage, never through a JSON MCP
payload. Xchats verifies size, MIME type, checksum, organization ownership, and
the object before it can be attached. The result contains `material_id`,
upload instructions, expiry, and processing status.

## 6. Widgets

Tools perform backend work. Widgets are optional user interfaces rendered by
the MCP host. A widget receives tool results and can call the same tools; it
never bypasses backend authentication or validation.

Use one reusable fullscreen resource:

```text
ui://xchats/kb-manager.html
```

It has these views:

### All

Shows live and draft records grouped by stable key. Each row displays:

- type;
- `ref`, `slug`, or `main`;
- title/name;
- presence in live and/or draft;
- state: `published`, `new`, `changed`, or `to_delete`.

This is a grouped comparison, not an `effective` merged dataset.

### Live

Shows the currently published KB. Type filters allow the user to view only
products, tariffs, topics, contacts, policies, assistant configuration, or
delivery zones.

### Draft

Shows only pending draft changes, including field-level differences from live.
After an MCP upsert/delete, the widget opens this view and confirms what
changed. It does not ask for approval for each draft edit.

### Record details

Loads one full record with `kb_read`. Large collections are paginated; the
widget never loads the entire KB into one MCP result.

### Media

Media lives **on the record**, not in a separate screen. Opening a record shows
every media column that record actually has, each with what is currently
attached and — for columns that are valid attachment targets — its own
multi-file upload input supporting selection and drag-and-drop. Choosing files
stages, uploads and attaches them in one motion, with per-file progress and
per-file errors; one rejected file never discards the rest of a batch.

Uploads run **sequentially**: the signed-PUT handler reads each body fully into
memory, and every attach takes the organization's draft row lock, so
parallelism would buy nothing but server memory and lock contention.

Previews come from `kb_read`'s `_meta["xchats/media"]` — per-material filename,
MIME type, size, kind and a short-lived signed URL served by
`GET /mcp/media/:id`. That metadata deliberately does **not** ride in
`structuredContent`: `kb_read`'s declared `outputSchema` is
`additionalProperties: false` and clients cache `tools/list`, so a new key
there could make a validating host reject `kb_read` itself. A host that strips
`_meta` degrades to identifier chips, never to an error.

**Only images render inline.** Video, audio and documents render as an icon,
filename, size and link, so nothing but an image is fetched without the user
asking for it. The thumbnail is CSS sizing on the original file — there is no
server-side resizing yet, so the upload size cap is also the ceiling on a
single preview allocation. A resizing thumbnail endpoint and a streaming blob
read are tracked separately.

`featured_image` is **preview-only here**: it is shown with a preview but has
no upload input, because it is not an attachment target. It remains a real,
independently writable column — set through the typed upsert tools — and at
prompt-resolution time an explicit value wins, while a `NULL` falls back to the
type's first primary image.

Because upload and attach are now one flow, orphaned uploads are much rarer,
but they are still possible: a PUT can succeed and the attach then fail. Reaping
unreferenced materials is separate work.

The widget should prefer shared MCP Apps APIs. ChatGPT-specific file-library
helpers may be used only after capability detection. Hosts without widget or
file APIs still retain all model-facing tools; the fallback is the normal
Xchats upload/review page.

### Publish

The draft view displays a computed diff and a **Review and publish in Xchats**
button. For v1 this opens the existing authenticated Xchats review page, where
the user performs the only required approval. Widget state is never treated as
authorization.

## 7. Complete tariff-image flow

Assume the first-time OAuth connection above has completed.

1. The user uploads a tariff image in chat and asks to add its information.
2. The LLM reads the image and extracts candidate tariff names and fields.
3. It calls `kb_summary(types=["tariffs"], source="both", query=...)`.
4. For every possible match, it calls `kb_read` with the exact tariff `ref`.
5. The media widget stores the original image through `kb_media_upload` and
   receives a `material_id`.
6. If the `ref` exists, the LLM calls `kb_tariff_upsert` with only changed
   fields and attaches the material to `featured_image` or `pricing_images`.
7. If no record matches, it creates a new tariff with a stable `ref`.
8. If names are similar but identity is uncertain, it asks the user which
   tariff should be updated; it does not create a duplicate.
9. The backend repeats duplicate checks, validates canonical tariff fields,
   reconstructs the complete row, writes it to `kbd_draft`, and increments
   `base_version`.
10. The KB widget opens the Draft view and shows the new/changed tariff and its
    media.
11. The user may continue editing; later draft changes build on the same draft.
12. The user selects **Review and publish in Xchats**, reviews the complete
    diff, and approves.
13. Xchats validates again, materializes the draft into `ai_tariffs`, writes the
    audit entry, clears the published delta, and invalidates the KB cache.

## 8. Complete product-URL flow

1. The user sends a coffee-machine product URL and asks to store the product.
2. The LLM host opens the page and extracts the product name, site SKU/slug,
   price, description, category, stock state, and supported media.
3. It derives a candidate `ref` from the stable site SKU/slug and calls
   `kb_summary(types=["products"], source="both", query=...)`.
4. If a possible product exists, it calls `kb_read` for the exact record.
5. Exact `ref` means update. The same normalized name under another `ref`
   produces a conflict, and the LLM asks the user before proceeding.
6. With no match, it calls `kb_product_upsert` to create the product. With a
   match, it sends only changed fields. The URL is passed as provenance, not
   placed into an unsupported product column.
7. The backend stores URL provenance in `kbd_materials`, validates the product,
   reconstructs a complete draft row, and updates `kbd_draft`.
8. The widget shows the product under Draft and shows whether a live version
   already exists.
9. The user reviews and publishes through the Xchats review page.
10. Xchats validates again, writes the live `ai_products` row, records the
    audit event, clears the published delta, and refreshes the live KB.

If the host cannot access the URL, it must ask the user to paste the product
content. Server-side URL fetching is not part of this initial MCP contract —
though an operator can now reach the same outcome outside a chat entirely
via the structured import pipeline (§1's note, `plan/playground.md`), which
does fetch URLs server-side under its own SSRF-hardened path.

## 9. Validation and implementation order

All writes must enforce canonical field names, required booleans, enum values,
same-organization media references, natural-key uniqueness, zone hierarchy,
policy/zone exclusivity, and optimistic concurrency.

Implementation order:

1. Add delivery zones to the `kbd_draft` shape and materialization path.
2. Add OAuth discovery, authorization, token rotation, revocation, and tenant
   checks.
3. Implement the 11 model-facing KB tools.
4. Implement `kb_media_upload` and signed object-storage uploads.
5. Implement the KB Manager widget and Xchats review-page handoff.
6. Test OAuth, tenant isolation, duplicate handling, concurrent draft writes,
   media ownership, pagination, host capability fallbacks, and publish
   validation with MCP Inspector, ChatGPT, and Claude.

## 10. Shared typed-upsert seam

`internal/mcpserver/apply.go` factors each `kb_*_upsert` tool's parameter
parsing and draft-write logic out of its own handler in `handlers.go`, so a
second caller — the structured import pipeline's pass-2 synthesis (§1's
note, `internal/kbimport`) — can drive the exact same validation and
`kbd_draft` write path without a live MCP connection at all:

```go
// The seven typed kb_*_upsert tools, in Tools()' own declared order.
// kb_delete is deliberately absent: there is structurally no code path
// from parsed web/document content to a delete marker.
var UpsertTools = []string{ /* ... */ }

// One parsed, not-yet-applied typed-upsert call.
type UpsertCall struct {
    Tool string // one of UpsertTools
    Key  string // natural ref/slug as supplied, "" = create with a derived key
}
func (c UpsertCall) Apply(ctx context.Context, kb *kbstore.Store, orgID, userID uuid.UUID,
    expectedVersion *int64, prov kbstore.MCPProvenance) (kbstore.UpsertResult, error)

// Parses one {tool, args} pair — the same {name, arguments} shape the MCP
// transport itself receives — through the SAME per-field parsers and
// validation every live handler already runs.
func ParseUpsertCall(tool string, args map[string]json.RawMessage) (UpsertCall, error)

// Returns {tool name: inputSchema} for a caller-chosen subset of
// UpsertTools — each tool's real declared JSON Schema, the same shape
// tools/list advertises to a connected MCP host.
func UpsertToolSchemas(names []string) map[string]map[string]any
```

- `ParseUpsertCall` decodes and validates exactly like a live `kb_*_upsert`
  tool call — same `rejectUnknownFields`, same enum/required-field checks —
  just invoked directly instead of through the MCP transport.
- `UpsertCall.Apply` performs the write: one call, one `kbd_draft` merge,
  identical to what a live connector's own upsert produces. `prov` is
  silently not forwarded for `kb_assistant_upsert` — `MCPUpsertAssistant`
  takes no provenance parameter; a persona/guardrails edit has never had a
  provenance concept.
- `UpsertToolSchemas` lets the import pipeline's synthesis prompt quote a
  tool's real input schema verbatim, so the model sees the identical
  contract whether it is a connected chat model or the batch synthesis
  call — `internal/kbimport` builds its own prompt from
  `UpsertToolSchemas(UpsertTools minus kb_assistant_upsert)`.
- A caller resolves every media-field handle in `args` to a real
  `material_id` **before** calling `ParseUpsertCall` — this seam carries no
  handle-resolution logic of its own (`internal/kbimport/handles.go` does
  it by walking `kbstore.MediaFieldKinds()`).

This is "one contract, two callers," not two implementations that could
drift apart: `internal/mcpserver/apply_test.go`'s
`TestParseUpsertCall_EquivalentToHandler` asserts the tools-call path and
the `ParseUpsertCall`+`Apply` path produce an identical `kbd_draft` result
for the same input.

## References

- MCP authorization: <https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization>
- MCP Apps: <https://modelcontextprotocol.io/extensions/apps/overview>
- ChatGPT authentication: <https://developers.openai.com/plugins/build/auth>
- ChatGPT MCP UI: <https://developers.openai.com/plugins/build/chatgpt-ui>
