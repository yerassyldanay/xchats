# AI Assistant & Playground — Design Decisions (July 2026)

Status: **agreed in discussion, not yet implemented.** The concrete KB schema
and customer-response JSON contract were consolidated here on 2026-07-21.
Where this file conflicts with `plan/*.md` or today's code, **this file wins**.
It is the single
authoritative decision record: it merges and **supersedes** `DECISION-BY-CLAUDE.md`
and `DECISION-BY-CODEX.md` (merged 2026-07-10).

---

## Part 1 — the idea, in plain words

1. **Only the approved live KB goes into a customer-response prompt — no vector
   search / RAG.** The draft and builder materials are never included. One org's
   approved KB is small; if the model always sees everything, nothing relevant
   can fail to be "found". Cost is covered by the provider's prompt caching (the
   prompt only changes when the approved KB changes). Caching affects price and
   speed, never answers. If a KB ever outgrows the prompt, the fallback is
   **deterministic narrowing** (shortlist by category/entity in code) — not
   free-form vector retrieval (threshold: see Open questions).

2. **Exact facts live in typed, purpose-named columns.** Column names explain
   themselves (`price`, `delivery_in_days`, `in_stock`); text values preserve
   approved money, ranges, times, and links, while `in_stock` is a boolean that
   code renders as reviewed Russian wording. V1 KB prose and replies are Russian
   only. Language-neutral exact storage keeps later language expansion possible
   without duplicating business records. Word-bearing values such as an address
   or warranty explanation are trusted Russian prose, not generic facts.

3. **The model never writes numbers — it writes placeholders.** A reply carries
   `{{product.sofa-loft.price}}`; code substitutes the stored value. An unknown
   placeholder blocks the whole draft for manual check. A made-up number is
   silent; a made-up placeholder fails loudly. Wrong prices in writing are a
   business and legal risk.

4. **Media references are concrete, purpose-named columns on complete KB rows.**
   Every input — text, URL, instruction, or uploaded file — lives in
   `kbd_materials`. A file-backed material holds its own storage/extraction
   metadata. Draft and live KB rows store stable `kbd_materials.id` values in
   semantic columns such as `ai_products.featured_image`,
   `ai_products.gallery_images`, and `ai_products.certificate_documents`.
   Database ids and storage paths are
   **backend-only**: no LLM ever sees or emits them. The builder model sees short
   request-scoped handles that code resolves; the customer-response model sees
   only approved semantic media tokens such as
   `products.acer-laptop-444.gallery_images`. Unknown handles or tokens fail
   closed.

5. **The KB is built in the playground in two passes.** The operator drops any mix
   of material (text, URLs, images, PDFs, audio, video). Pass 1 reads each file
   separately; pass 2 turns everything into draft KB entries in the exact KB
   schema — written into the **one draft immediately** (no per-job mini-drafts,
   no intermediate approvals). A human reviews **once**, at draft → live.
   Nothing touches the live KB before Approve.

6. **Every uploaded file has `customer_visibility`: `auto | invisible | visible`.**
   Default `auto`: the system decides whether a file is only *evidence* (a
   screenshot, a voice note — its information enters the KB, the file never
   reaches a customer) or *customer-sendable media* (a product photo). The
   operator's explicit choice always wins over the model. A file becomes
   customer-sendable only when its backend material reference is in a concrete
   media column on an **approved live KB row** and the material passes the
   customer-visibility checks.

---

## Part 2 — how it works (technical)

### Core principles

1. **Draft and live schemas are the same.** Draft entries mirror live rows —
   same business columns. Approval validates and upserts; it never translates.
2. **Raw files are stored before extraction.** Upload success is final for the
   bytes; a failed extraction never loses the file.
3. **Parse each material separately.** Every input gets its own row and
   `processing_status`.
4. **Synthesize after extraction**, over the whole batch + current draft + live KB.
5. **The model proposes; code validates.** Every model-visible handle, semantic
   token, ref, column, and value is checked before anything is stored or sent.
   Internal ids and storage locators never enter an LLM prompt or response.
6. **Exact values are handled carefully.** Typed columns only, confirmed by the
   draft approval; a separate request is required only for ambiguity/conflict.
7. **The customer-facing assistant reads approved live rows only.** Never
   `kbd_draft`, never pending changes, and never raw `kbd_materials` content.

### Physical table names and boundaries

All tables live in the PostgreSQL schema `xchats`. Prefixes describe ownership
and lifecycle; they are part of the table name, not optional documentation:

```text
ai_assistants
ai_topics
ai_products
ai_tariffs
ai_contacts
ai_policies
ai_audit_log

kbd_draft
kbd_materials
kbd_requests

rp_suggestions
```

- `ai_*` is the approved live KB used to build the customer-response prompt.
- `kbd` means **Knowledge Base Development**. `kbd_*` is the playground working
  area: source materials, extraction workflow, unresolved requests, and the
  unapproved KB draft. The customer-facing model never receives draft or
  extraction content from `kbd_*`; after the model selects an approved semantic
  media token, backend code may resolve the corresponding live column through
  `kbd_materials` to load file bytes.
- `rp_*` is response-suggestion state.
- The model-facing draft uses concise domain keys with a fixed mapping:
  `assistant → ai_assistants`, `topics → ai_topics`, `products → ai_products`,
  `tariffs → ai_tariffs`, `contacts → ai_contacts`, and
  `policies → ai_policies`. This mapping is code-owned and exhaustive; the model
  cannot invent a target table.
- Files are not KB entities and there is no generic attachment relationship.

### Canonical knowledge-base schema

This section is the closed v1 database contract. Migrations, Go types, builder
schemas, draft validation, prompt rendering, eval fixtures, and documentation
must use these exact names. Aliases are rejected: for example, use
`delivery_in_days`, never `delivery_time`; `working_hours`, never `work_hours`;
and `gallery_images`, never generic `images`. There is no generic business
`data`, `values`, `media`, `files`, `assets`, or `attachments` bag.

All tables are in PostgreSQL schema `xchats`. All tenant-owned rows include
`organization_id uuid`; identifiers are `uuid`; timestamps are `timestamptz`.
V1 has no `lang` columns: trusted prose is Russian, while exact values are stored
once. A missing scalar is SQL `NULL`, never an empty or whitespace-only string.
A missing singular media reference is `NULL`; a missing plural media reference
is an empty `uuid[] NOT NULL DEFAULT '{}'`. Exact text values preserve approved
formatting verbatim because ranges and approximations do not fit a single numeric
type. Required text fields must be non-blank.

The fact vocabulary is closed as well. A new exact fact such as product weight,
power, or dimensions requires an explicit purpose-named column, migration,
placeholder allowlist entry, builder support, and tests. Until then it is not
stored as an exact live fact and must not be hidden in prose or JSON.

#### `kbd_materials` — source and uploaded-file registry

This is the only material registry. It stores text/URL/instruction content or a
file's metadata and storage locator; it never stores file bytes in PostgreSQL.

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `id` | `uuid` | primary key | — |
| `organization_id` | `uuid` | not null, FK `organizations(id)` | — |
| `source_type` | `text` | not null | `text`, `url`, `instruction`, or `file` |
| `source_ref` | `text` | null | URL, original filename, message id, or source label |
| `source_text` | `text` | null | Original pasted text or instruction |
| `operator_note` | `text` | null | Operator-only context for extraction/building |
| `storage_backend` | `text` | null | Blob adapter name for a file |
| `storage_key` | `text` | null | Provider-specific object locator |
| `filename` | `text` | null | Original customer/operator filename |
| `mime_type` | `text` | null | Detected file MIME type |
| `size_bytes` | `bigint` | null, `> 0` for files | File size in bytes |
| `sha256_checksum` | `text` | null | SHA-256 content checksum |
| `extracted_text` | `text` | null | Normalized textual evidence for synthesis |
| `visual_summary` | `text` | null | Visual description produced during extraction |
| `transcript_text` | `text` | null | Audio/video transcript |
| `extraction_metadata` | `jsonb` | not null, default `{}` | Extraction method, model, provenance, and errors |
| `processing_status` | `text` | not null, default `uploaded` | `uploaded`, `extracting`, `parsed`, `built`, `needs_human`, or `failed` |
| `customer_visibility` | `text` | null | `auto`, `invisible`, or `visible`; file-only |
| `created_at` | `timestamptz` | not null, default `now()` | — |
| `updated_at` | `timestamptz` | not null, default `now()` | — |

For `source_type=file`, `storage_backend`, `storage_key`, `filename`,
`mime_type`, and `size_bytes` are required. For non-file materials those fields
are `NULL`. A live media column may reference only a same-organization,
file-backed, non-empty, customer-sendable material with a compatible MIME type.

#### `kbd_draft` — one pending delta per organization

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `organization_id` | `uuid` | primary key, FK `organizations(id)` | — |
| `draft` | `jsonb` | not null | Delta-only document using the exact live business-column names |
| `base_version` | `bigint` | not null, default `0` | Compare-and-swap counter for concurrent edits |
| `updated_at` | `timestamptz` | not null, default `now()` | — |
| `updated_by` | `uuid` | null, FK `users(id)` | Last operator who changed the draft |

The document shape is fixed:

```json
{
  "assistant": null,
  "topics": [],
  "products": [],
  "tariffs": [],
  "contacts": null,
  "policies": null,
  "deletes": []
}
```

Entries contain business columns only: no database id, organization id, or
timestamps. `contacts` and `policies` are nullable singleton objects, not arrays.

#### `kbd_requests` — stored human-review questions

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `id` | `uuid` | primary key | — |
| `organization_id` | `uuid` | not null, FK `organizations(id)` | — |
| `material_id` | `uuid` | null, FK `kbd_materials(id)` | Material that caused the question, if applicable |
| `request_type` | `text` | not null | `confirm_value`, `describe_file`, `choose_media_column`, `resolve_duplicate`, or `resolve_conflict` |
| `question_text` | `text` | not null | Russian question shown to the operator |
| `question_context` | `jsonb` | not null, default `{}` | Evidence and alternatives needed to answer |
| `target_field` | `jsonb` | not null | Exact table, natural ref, and column being resolved |
| `request_status` | `text` | not null, default `open` | `open` or `resolved` |
| `resolution` | `jsonb` | null | Validated operator answer |
| `created_at` | `timestamptz` | not null, default `now()` | — |
| `resolved_at` | `timestamptz` | null | — |

#### `ai_assistants` — approved assistant configuration

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `id` | `uuid` | primary key | — |
| `organization_id` | `uuid` | not null, unique, FK `organizations(id)` | — |
| `persona` | `text` | not null | Russian role and voice of the assistant |
| `mission` | `text` | null | Russian business objective for replies |
| `guardrails` | `text` | not null | Russian behavior and safety rules |
| `language_policy` | `text` | not null | V1 rule requiring Russian output |
| `reply_max_words` | `integer` | not null, default `120`, `> 0` | Maximum suggested reply length |
| `created_at` | `timestamptz` | not null, default `now()` | — |
| `updated_at` | `timestamptz` | not null, default `now()` | — |

#### `ai_topics` — approved explanatory knowledge

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `id` | `uuid` | primary key | — |
| `organization_id` | `uuid` | not null, FK `organizations(id)` | — |
| `slug` | `text` | not null, unique per organization | Stable natural key used in tokens |
| `title` | `text` | not null | Russian topic title |
| `body_md` | `text` | not null | Approved Russian Markdown knowledge |
| `featured_image` | `uuid` | null, FK `kbd_materials(id)` | Single main topic image |
| `illustration_images` | `uuid[]` | not null, default `{}` | Supporting topic illustrations, in send order |
| `explainer_videos` | `uuid[]` | not null, default `{}` | Videos that explain the topic |
| `narration_audio_files` | `uuid[]` | not null, default `{}` | Customer-sendable spoken explanations |
| `reference_documents` | `uuid[]` | not null, default `{}` | Documents supporting the topic |
| `created_at` | `timestamptz` | not null, default `now()` | — |
| `updated_at` | `timestamptz` | not null, default `now()` | — |

#### `ai_products` — approved sellable products

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `id` | `uuid` | primary key | — |
| `organization_id` | `uuid` | not null, FK `organizations(id)` | — |
| `ref` | `text` | not null, unique per organization | Stable product natural key |
| `name` | `text` | not null | Approved Russian product name |
| `price` | `text` | null | Exact approved price, including currency/range formatting |
| `description` | `text` | null | Trusted Russian product description |
| `category` | `text` | null | Product category used for deterministic narrowing |
| `in_stock` | `boolean` | not null | Stock state; code localizes it into Russian wording |
| `sales_status` | `text` | not null, default `active` | `active` or `inactive` |
| `featured_image` | `uuid` | null, FK `kbd_materials(id)` | Single main product image |
| `gallery_images` | `uuid[]` | not null, default `{}` | Product gallery images, in send order |
| `demo_videos` | `uuid[]` | not null, default `{}` | Product demonstration videos |
| `audio_description_files` | `uuid[]` | not null, default `{}` | Spoken product descriptions |
| `certificate_documents` | `uuid[]` | not null, default `{}` | Product certificates |
| `manual_documents` | `uuid[]` | not null, default `{}` | Product user/installation manuals |
| `guarantee_documents` | `uuid[]` | not null, default `{}` | Product guarantee documents |
| `specification_documents` | `uuid[]` | not null, default `{}` | Technical product specifications |
| `created_at` | `timestamptz` | not null, default `now()` | — |
| `updated_at` | `timestamptz` | not null, default `now()` | — |

#### `ai_tariffs` — approved plans and tariffs

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `id` | `uuid` | primary key | — |
| `organization_id` | `uuid` | not null, FK `organizations(id)` | — |
| `ref` | `text` | not null, unique per organization | Stable tariff natural key |
| `name` | `text` | not null | Approved Russian tariff name |
| `price` | `text` | null | Exact approved tariff price |
| `limit_text` | `text` | null | Trusted Russian explanation of usage limits |
| `fee` | `text` | null | Exact approved fee when applicable |
| `summary` | `text` | null | Short trusted Russian tariff summary |
| `pricing_type` | `text` | not null | `fixed`, `percentage`, `tiered`, or `hybrid` |
| `advantages` | `text` | null | Trusted Russian advantages |
| `disadvantages` | `text` | null | Trusted Russian limitations/disadvantages |
| `sales_status` | `text` | not null, default `active` | `active` or `inactive` |
| `featured_image` | `uuid` | null, FK `kbd_materials(id)` | Single main tariff image |
| `pricing_images` | `uuid[]` | not null, default `{}` | Price cards and pricing illustrations |
| `explainer_videos` | `uuid[]` | not null, default `{}` | Videos explaining the tariff |
| `terms_documents` | `uuid[]` | not null, default `{}` | Tariff terms and conditions documents |
| `created_at` | `timestamptz` | not null, default `now()` | — |
| `updated_at` | `timestamptz` | not null, default `now()` | — |

#### `ai_contacts` — approved organization contacts

Exactly one row exists per organization. Its model-facing natural ref is the
code-owned constant `main`; there is no mutable `slug` column.

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `id` | `uuid` | primary key | — |
| `organization_id` | `uuid` | not null, unique, FK `organizations(id)` | — |
| `whatsapp` | `text` | null | Exact approved WhatsApp contact |
| `email` | `text` | null | Exact approved support email |
| `address` | `text` | null | Trusted Russian business address |
| `legal_information` | `text` | null | Trusted Russian legal/company details |
| `callback_time` | `text` | null | Trusted Russian callback expectation |
| `working_hours` | `text` | null | Exact approved working-hours display |
| `phone` | `text` | null | Exact approved support phone |
| `website` | `text` | null | Exact approved website |
| `instagram` | `text` | null | Exact approved Instagram account |
| `contact_card_image` | `uuid` | null, FK `kbd_materials(id)` | Single contact-card image |
| `location_map_image` | `uuid` | null, FK `kbd_materials(id)` | Single location/map image |
| `company_legal_documents` | `uuid[]` | not null, default `{}` | Customer-sendable company/legal documents |
| `created_at` | `timestamptz` | not null, default `now()` | — |
| `updated_at` | `timestamptz` | not null, default `now()` | — |

#### `ai_policies` — approved commerce policies

Exactly one row exists per organization. Its model-facing natural ref is `main`.

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `id` | `uuid` | primary key | — |
| `organization_id` | `uuid` | not null, unique, FK `organizations(id)` | — |
| `delivery_cost` | `text` | null | Exact approved delivery price; MUST be `NULL` whenever `ai_delivery_zones` has any row for this organization (see that table) |
| `delivery_in_days` | `text` | null | Exact delivery duration/range in days; same mutual-exclusion rule as `delivery_cost` |
| `free_delivery_from` | `text` | null | Exact order value that qualifies for free delivery |
| `min_order` | `text` | null | Exact minimum order value |
| `prepayment` | `text` | null | Trusted Russian prepayment policy |
| `installment` | `text` | null | Trusted Russian installment policy |
| `return_period_in_days` | `text` | null | Exact return duration in days |
| `warranty` | `text` | null | Trusted Russian warranty policy; not an exact-duration fact |
| `outside_zones_note` | `text` | null | Exact approved refusal shown for a direction matching no `ai_delivery_zones` row; required (non-null) whenever `ai_delivery_zones` has any row for this organization |
| `commerce_policy_documents` | `uuid[]` | not null, default `{}` | Customer-sendable commerce-policy documents |
| `created_at` | `timestamptz` | not null, default `now()` | — |
| `updated_at` | `timestamptz` | not null, default `now()` | — |

#### `ai_delivery_zones` — approved delivery-coverage zones

Zero or more rows per organization; empty means the organization has not opted
into per-zone delivery and `ai_policies.delivery_cost`/`delivery_in_days`
answer every delivery-cost question instead (the pre-existing, still-supported
behavior). The moment any row exists for an organization, the flat
`ai_policies.delivery_cost`/`delivery_in_days` fields must both be `NULL` for
that organization (a flat answer would contradict per-zone pricing) and
`ai_policies.outside_zones_note` must be non-null (every direction that
matches no zone still needs a real, seller-approved refusal — never invented
by the model). Zones form a shallow containment hierarchy via `parent_ref`
(city → region → country); which zone answers a given customer message is
resolved by the model choosing the most specific matching FACTS token, not by
code walking the hierarchy at answer time. A row with `delivery_available =
true` must carry non-null `delivery_cost` and `delivery_in_days`; one with
`delivery_available = false` (an explicit "we do not deliver here" row, e.g.
a city excluded from an otherwise-covered region) must carry neither — a
contradictory row is rejected at write time, never guessed at read time. No
media columns exist on this table in v1.

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `id` | `uuid` | primary key | — |
| `organization_id` | `uuid` | not null, FK `organizations(id)` | — |
| `ref` | `text` | not null, unique per organization | Stable zone natural key |
| `name` | `text` | not null | Approved Russian zone display name |
| `zone_level` | `text` | not null | `city`, `region`, or `country` |
| `parent_ref` | `text` | null | This organization's own `ai_delivery_zones.ref` this zone is nested under; null for a top-level zone |
| `delivery_available` | `boolean` | not null | Whether this zone is served at all; code localizes it into Russian wording, the same discipline as `ai_products.in_stock` |
| `delivery_cost` | `text` | null | Exact approved delivery price for this zone; required iff `delivery_available`, otherwise must be null |
| `delivery_in_days` | `text` | null | Exact delivery duration/range in days for this zone; same requirement as `delivery_cost` |
| `notes` | `text` | null | Trusted Russian prose about this zone (e.g. why an excluded city is excluded) |
| `sales_status` | `text` | not null, default `active` | `active` or `inactive`; an inactive zone produces no facts but remains a valid `parent_ref` target |
| `created_at` | `timestamptz` | not null, default `now()` | — |
| `updated_at` | `timestamptz` | not null, default `now()` | — |

#### `ai_audit_log` — append-only KB approval history

| Column | Type | Null/default | Short description |
|---|---|---|---|
| `id` | `uuid` | primary key | — |
| `organization_id` | `uuid` | not null, FK `organizations(id)` | — |
| `action` | `text` | not null | `approve`, `edit`, or `delete` |
| `actor_user_id` | `uuid` | null, FK `users(id)` | Operator responsible for the action |
| `note` | `text` | null | Optional Russian audit explanation |
| `created_at` | `timestamptz` | not null, default `now()` | — |

#### Example records (three, infrastructure fields shortened)

The UUID in the product's `featured_image` is a backend reference to the first
record. No file bytes, filename, UUID, or storage locator enters the customer
prompt.

```json
{
  "table": "kbd_materials",
  "id": "22222222-2222-2222-2222-222222222222",
  "organization_id": "11111111-1111-1111-1111-111111111111",
  "source_type": "file",
  "source_ref": "coffee-machine-front.jpg",
  "storage_backend": "s3",
  "storage_key": "org/1111/materials/2222",
  "filename": "coffee-machine-front.jpg",
  "mime_type": "image/jpeg",
  "size_bytes": 248315,
  "sha256_checksum": "8a4e...f190",
  "visual_summary": "Фронтальная фотография кофемашины DeLonghi.",
  "processing_status": "built",
  "customer_visibility": "visible"
}
```

```json
{
  "table": "ai_products",
  "ref": "coffee-machine",
  "name": "Кофемашина DeLonghi",
  "price": "129 900 ₸",
  "description": "Автоматическая кофемашина для дома.",
  "category": "Кофемашины",
  "in_stock": true,
  "sales_status": "active",
  "featured_image": "22222222-2222-2222-2222-222222222222",
  "gallery_images": [],
  "demo_videos": [],
  "audio_description_files": [],
  "certificate_documents": [],
  "manual_documents": [],
  "guarantee_documents": [],
  "specification_documents": []
}
```

```json
{
  "table": "ai_topics",
  "slug": "how-to-order",
  "title": "Как оформить заказ",
  "body_md": "Напишите, какой товар вас интересует, и укажите адрес доставки.",
  "featured_image": null,
  "illustration_images": [],
  "explainer_videos": [],
  "narration_audio_files": [],
  "reference_documents": []
}
```

### Lifecycle terminology

Use these terms consistently in code, prompts, UI, and documentation:

- **Source material** — one operator input stored in `kbd_materials`: text, URL,
  instruction, or file.
- **Parsed material** — a source material whose extraction finished and whose
  evidence is ready for synthesis.
- **Candidate draft patch** — raw pass-2 model output before deterministic schema
  and reference validation; it is not stored or shown as approved knowledge.
- **KB draft** (user-facing) / **pending KB changes** (technical) — the validated,
  unapproved changes accumulated in `kbd_draft`. This is the standard term for
  the knowledge base generated through the playground but not yet approved.
- **Draft entry** — one pending create/update/delete inside the KB draft.
- **Live KB** / **approved KB** — rows materialized into `ai_*`; this is the only
  knowledge the customer-facing model receives.

### The flow, end to end

```text
1. operator submits inputs (files / url / text / instruction)
2. ingest: one `kbd_materials` row per input (text / url / instruction / file);
   file bytes → configured blob store and their storage metadata stays on that
   same material row; one extraction job per material
3. extraction (pass 1): parallel per material → parsed | needs_human | failed
4. synthesis (pass 2): parsed evidence + instruction + model-safe live/draft views
   → KB-shaped draft patch + requests
5. validation: resolve request-scoped handles to internal material ids; verify
   organization, storage, customer visibility, media type, columns, and values
6. draft upsert: atomic merge into the org's `kbd_draft` blob (by natural keys)
7. review: operator edits / confirms / rejects on the accumulated draft
8. approve: gate → upsert/delete complete `ai_*` rows → clear from draft →
   reload brain
```

### Two passes

**Pass 1 — per-material understanding (parallel, one request per material).**
Text is passed through, URLs are fetched, and each uploaded file is parsed in
isolation. Each parse
gets context: the operator's message + a compact index of the draft and live KB
(natural refs, slugs, and allowed media columns, but no database ids or storage
paths). Output for each material is stored on its `kbd_materials` row: extracted
text, detected facts with provenance, a "relates to" hint, and — for applicable
files — a visual/audio summary and customer-visibility suggestion. Pass 1
**describes, never decides** KB structure. Its summaries are working notes —
they never enter the live KB. Rejected: one bundled parse request for all files
(kills per-file isolation, retries, and provenance).

**Pass 2 — batch synthesis (one text-only call, no bytes or internal ids).**
Input: the operator's instruction + **the content of every parsed material** +
short request-scoped handles **only for customer-visible files** + model-safe
views of the **full draft** and **full live KB** (so consecutive uploads build on
each other and update-vs-create is real diffing). A model-safe view preserves
business values and semantic media columns but replaces every internal material
id and storage locator with a short handle such as `upload.1`. The handle exists
only for that model call and has no meaning outside the backend mapping.

Output: a **draft patch in the KB schema** plus requests. Pass 2 is
side-effect-free while running; code validates every handle, translates it to
the mapped `kbd_materials.id`, materializes a complete storage row, and merges
that row into the draft atomically. It must not assume one file = one entity:
`upload.1`–`upload.2` may fill one product's `gallery_images`, `upload.3` its
`certificate_documents`, and `upload.4` a tariff's `pricing_images`. Each
proposed entity contains its own concrete media fields; pass 2 never emits a
separate attachment or relationship operation.

Manifest in (trimmed example):

```json
{ "materials": [
    { "handle": "upload.1",
      "customer_visibility": "visible", "source_type": "file", "mime_type": "image/jpeg",
      "processing_status": "parsed",
      "visual_summary": "Фронтальная фотография магнитного сверлильного станка.",
      "extracted_text": null },
    { "handle": "upload.2",
      "customer_visibility": "visible", "source_type": "file", "mime_type": "image/jpeg",
      "processing_status": "parsed",
      "visual_summary": "Шильдик модели ZT-40H и карточка с ценой.",
      "extracted_text": "ZT-40H, 180 000 ₸",
      "detected_facts": [ { "field_hint": "price", "value": "180 000 ₸",
                            "provenance": "upload.2" } ] },
    { "handle": "evidence.1", "customer_visibility": "invisible",
      "source_type": "file", "mime_type": "image/png", "processing_status": "parsed",
      "visual_summary": "Скриншот страницы поставщика.",
      "extracted_text": "ZT-40H, оптовая цена 160 000 ₸" } ] }
```

No manifest field exposes a database id, path, storage key, or public URL.
Eligible customer-sendable files get `upload.*` handles. Invisible evidence gets
an `evidence.*` handle valid only in provenance metadata. Validation rejects an
evidence handle, expired handle, or invented handle inside a concrete KB media
column.

Draft patch out (trimmed example):

```json
{ "draft_patch": {
    "products": [ {
      "ref": "drill-zt40h",
      "name": "Магнитный сверлильный станок ZT-40H",
      "price": "180 000 ₸",
      "description": "Магнитный сверлильный станок для монтажных работ.",
      "category": "Сверлильные станки",
      "in_stock": true,
      "sales_status": "active",
      "featured_image": "upload.1",
      "gallery_images": ["upload.2"]
    } ] },
  "requests": [] }
```

### Draft = same schema, stored as one jsonb blob per org

- The entire pending KB is **one jsonb document** (`kbd_draft`), one row per org:
  `{ assistant, topics[], products[], tariffs[], contacts, policies, deletes[] }`
  — each entry mirrors the business columns of its mapped `ai_*` live-table row.
  The draft holds **deltas only**, not a copy of the KB. The brain never reads it.
  There is no separate file-assignment section: media references are fields of
  the complete topic/product/tariff/contact/policy entries, while their source
  rows stay in `kbd_materials`.
- Jobs upsert into the draft **immediately** — rejected: per-job "mini drafts"
  with their own approval step (double review, stale snapshots, broken
  build-on-each-other). The safety boundary is draft → live, not job → draft.
- Pass 2 proposes all creates and updates as complete rows in the mapped draft
  section, and all removals as `deletes[]` markers. The operator reviews and
  approves the result; there is no manual step that links files to rows.
- An update = a draft entry **shadowing** the live row by natural key
  (`ai_products.ref`, `ai_tariffs.ref`, `ai_topics.slug`, singletons by org).
  Entity states are **derived, never stored**: no shadow = published · draft only =
  **Новый** · draft + live = **Изменён** · `deletes[]` marker = **К удалению**.
  A patch entry identical to the live row is dropped (no badge noise).
- Diffs are **computed** (draft entry vs live row, field by field — trivial
  because the schemas are identical), not stored. Rejected: per-row lifecycle
  columns (`change_type`, `review_status`, `base_live_version`,
  `changed_fields_json`) on draft/live tables — derived state cannot drift.
- A new upload touching an already-pending entity **overwrites its draft entry,
  building on it** (never resetting to live). Field-merge rule: empty field +
  new value → fill with provenance; same value → keep, add provenance;
  **different exact value → confirmation request**; different prose → update
  with a visible field-level diff. The second change is loud (chat + activity
  log), never silent.
- Merges are idempotent (natural keys) — re-running a pass never duplicates.
  One version counter on the blob catches concurrent writes (stale → 409).
- How the user sees changes: the **chat narrates** each turn; **badges + a
  field-level diff vs live** show the state now; **provenance** (source files
  per entry) explains why; `ai_audit_log` records approves. "Reject this
  whole upload" = a bulk action over provenance, not a separate approval layer.
- Approve (whole draft or selected entities): run the gate → upsert complete
  rows into their mapped `ai_*` tables on natural keys → apply `deletes[]` →
  remove from the blob → reload the brain. The backend has already translated
  model-visible handles into `kbd_materials.id` values; approval copies those
  internal values from the draft row into the concrete `ai_*` media columns. It
  never writes a separate attachment relationship. No versioning, no rollback
  in v1 (accepted trade-off).

### Materials: every builder input, including stored files

- `kbd_materials` is the **only** material/file registry. There is no second
  file table. Its exact columns and types are defined in the canonical schema
  above. It stores pasted text, URLs, operator instructions, and uploaded files
  in one normalized ingestion lifecycle.
- For a URL, `source_ref` is the URL and `extracted_text` is the fetched readable
  snapshot. For text/instructions, `source_text` preserves the original input
  and `extracted_text` carries the normalized synthesis input. For a file, the
  storage/file/extraction columns carry its bytes locator and evidence.
- `storage_backend + storage_key` are interpreted by the storage adapter and may
  address local bare-metal storage, MinIO, S3, or another provider. Approved KB
  rows never store these values or public URLs; they store stable
  `kbd_materials.id` values. These ids also stay out of every model prompt and
  response. Moving bytes changes the material's storage fields, not every KB row
  that uses the file.
- A file-backed `kbd_materials` row is durable while any approved `ai_*` media
  column references it. `built` means its evidence has been consumed by pass 2;
  it does **not** mean the row or bytes may be deleted. Cleanup must first prove
  that no live or draft media column references the material.
- Prompt/evaluation fixtures exercise media with deterministic `kbd_materials`
  metadata and a fake storage adapter. They do not require committed image, PDF,
  audio, or video bytes; byte-transfer tests belong to the storage adapter
  boundary, not the KB-to-prompt evaluation.

### Concrete media-column naming

- A column name must communicate the **business purpose or recognizable media
  group**, because that same name becomes the final segment of the semantic media
  token. Use snake_case. Never use generic columns such as `image`, `images`,
  `video`, `videos`, `audio`, `document`, `documents`, `media`, `files`, `assets`,
  or `attachments` by themselves.
- `featured_image` is the standard name for an entity's single main image.
  `hero_image` is acceptable only if it specifically means the website hero;
  `primary_image` is avoided because it is easily confused with a primary key.
- The closed v1 media-column list is:

  ```text
  ai_topics:
    featured_image
    illustration_images
    explainer_videos
    narration_audio_files
    reference_documents

  ai_products:
    featured_image
    gallery_images
    demo_videos
    audio_description_files
    certificate_documents
    manual_documents
    guarantee_documents
    specification_documents

  ai_tariffs:
    featured_image
    pricing_images
    explainer_videos
    terms_documents

  ai_contacts:
    contact_card_image
    location_map_image
    company_legal_documents

  ai_policies:
    commerce_policy_documents

  ai_assistants:
    no media columns in v1
  ```

- Despite their semantic names, the physical values are internal references:
  singular columns such as `featured_image` are nullable `uuid`; group columns
  such as `gallery_images` are `uuid[] NOT NULL DEFAULT '{}'`. Every stored UUID
  is a `kbd_materials.id`; no KB media column stores a path, storage key, URL, or
  model token. Scalar columns can use a normal foreign key; PostgreSQL cannot
  apply an element-level foreign key to an array, so code validates every array
  element before draft write, approval, and runtime delivery.
- Validation requires each referenced material to belong to the same org, have
  `source_type=file`, contain a storage locator, match the media type named by
  the column, and be customer-sendable. A URL, text, instruction, invisible
  file, or wrong MIME type cannot be stored in a live media column.
- These columns are a closed schema, not model-created keys. New purposes or
  media types require an explicit migration plus validator/prompt support. The
  model cannot invent one.
- A pass-2 result contains complete model-facing entity rows with request-scoped
  handles in these fields. Code replaces the handles with internal ids before
  writing `kbd_draft`. The same material may be used in several entity rows when
  reuse is intentional.
- The customer-response prompt exposes a **derived media catalog** using the
  exact format `<table>.<natural_ref>.<column>`. `table` is the unprefixed
  model-facing domain name, not the physical `ai_*` name. Examples:
  `products.acer-laptop-444.featured_image`,
  `products.acer-laptop-444.gallery_images`,
  `products.acer-laptop-444.certificate_documents`, and
  `products.acer-laptop-444.guarantee_documents`.
- The token grammar is deliberately closed: exactly three dot-separated
  segments; `table` must be one of the configured model-facing table names;
  `natural_ref` must match an approved row; and `column` must be one of that
  table's declared sendable media columns. Natural refs and column names cannot
  contain dots.
- The media catalog is generated **only from approved `ai_*` rows** and contains
  only tokens for non-empty, currently sendable columns. It contains no draft
  rows, UUIDs, paths, storage keys, filenames, or individual-file entries. The
  model must copy an exact token from the catalog; it cannot construct a valid
  token merely by guessing the format.
- A `NULL` singular column and an empty plural array generate no catalog token.
  A row is advertised as having no media only when all of its semantic media
  columns are empty. If one column is populated and another is empty, only the
  populated column is catalogued. A non-empty column containing an invalid or
  stale material reference fails prompt rendering; it is not silently treated
  as empty.
- When the customer-response model returns a token, code parses the three
  segments, loads the approved row by its natural ref, reads that exact semantic
  column, resolves its internal ids through `kbd_materials`, and sends the one
  file or the complete group. For example, `.featured_image` sends one file and
  `.gallery_images` sends every file in that array. No per-file picking from a
  group.
- Deleting a KB row or removing a material id from a media column never deletes
  the material row or blob automatically. Cleanup is a separate retention job.

### Customer-response JSON contract

There is one response field for media selection: `media_files_to_send`. The
names `asset_refs`, `attach_groups`, and `send` are not aliases and must not
appear in prompts, schemas, Go types, eval fixtures, retry logic, or validators.
Each `media_files_to_send` item is a semantic media-catalog token, never a UUID,
filename, URL, path, or storage key.

Every property in the LLM tool/JSON Schema must carry the following description;
the descriptions are part of the contract, not optional comments:

| JSON property | Type | Required | Description used in the JSON Schema |
|---|---|---|---|
| `reply_text` | `string` | yes | Russian customer-facing reply. Exact business values must be represented by approved placeholders, never model-written literals. |
| `reply_language` | `string` | yes | Language of `reply_text`; the only allowed v1 value is `ru`. |
| `media_files_to_send` | `string[]` | yes | Ordered semantic tokens copied exactly from the media catalog. An empty array means no media. |
| `escalate` | `boolean` | yes | `true` when approved live knowledge is insufficient and human review is required. |
| `escalation_reason` | `string` | yes | Internal Russian reason for escalation; empty when `escalate` is false and never shown to the customer. |
| `confidence` | `number` | yes | Model confidence from `0` to `1`; informational only and never a safety gate. |

Unknown JSON properties are rejected. Example:

```json
{
  "reply_text": "Стоимость — {{product.coffee-machine.price}}. Отправлю основное изображение.",
  "reply_language": "ru",
  "media_files_to_send": [
    "products.coffee-machine.featured_image"
  ],
  "escalate": false,
  "escalation_reason": "",
  "confidence": 0.91
}
```

Before storing a response suggestion, code validates the exact JSON shape,
rejects unknown or stale placeholders and media tokens, re-reads the approved
row and semantic column, validates every material again, substitutes exact
values, and resolves the storage records. Any invalid placeholder or requested
media file blocks the complete suggestion; partial rendering is forbidden.

A markdown code fence (` ```json … ``` `) wrapping the object is stripped as
transport noise before validation — run 2026-07-22_23-37-42-94b6 showed 5 of 7
models fence by default even when told «строго JSON», so rejecting fences
measured formatting luck, not contract compliance. Everything else stays
strict: prose outside the fence, a second object, or any content deviation
still fails the parse.

### `customer_visibility`: `auto | invisible | visible`

- Per-file, set at upload (default `auto`), editable in review. **The operator's
  choice always wins over the model**: user `invisible` → the model cannot make
  it visible; user `visible` → the builder model may place its request-scoped
  handle in an allowed media column; `auto` → extraction suggests whether it is
  customer-sendable and approval of a row containing it confirms that
  suggestion.
- Pass 2 receives the **content of every parsed file**, but attachable `upload.*`
  handles only for customer-visible files. Invisible files feed knowledge;
  their provenance travels under non-attachable `evidence.*` handles, never as
  something accepted in a media column — so attaching one is impossible, not
  merely forbidden.
- Enforcement has two independent layers: pass 2 never sees an attachable
  handle for evidence-only files; the customer media catalog is materialized
  only from concrete media columns on approved `ai_*` rows, and every internal
  reference must resolve to a same-org, file-backed, customer-sendable
  `kbd_materials` row. Unknown handles/tokens and cross-org, non-file, or
  invisible internal references are rejected fail-closed.
- The UI shows every file's fate plainly: «использован как источник, клиентам
  не отправляется» vs «products.sofa-loft.featured_image». Flipping a
  wrong call is one action; no KB reference becomes live until approve.

### Requests: a small sidecar table

> Note: this **reverses** an earlier "no stored question queue" decision — the
> review settled on a sidecar table as useful workflow state.

- `kbd_requests` — workflow state, **never part of the live KB schema**. Its
  `request_type` values are `confirm_value | describe_file |
  choose_media_column | resolve_duplicate | resolve_conflict`.
- Anti-staleness rule: every request targets data **by natural key**
  (table+ref+field / material id). Editing or resolving the target **auto-resolves
  the request** — a request can never ask about a state that no longer exists.
- Approve is blocked only by open requests attached to the rows being approved;
  unrelated open requests never block.
- Ambiguity that needs conversation (mixed intent, unclear instruction) is
  asked **in the chat**, not stored; the operator's next message answers it.

### Errors: skip the failed, proceed with the rest

- `kbd_materials.processing_status` transitions are
  `uploaded → extracting → parsed | needs_human | failed`,
  plus **`built`** once a synthesis pass has consumed the evidence (prevents
  re-feeding old materials every turn).
- Per-material failures never abort the batch: transient → 2–3 retries; then (and
  for permanent failures) → `needs_human` with a `describe_file` request.
  Retrying updates the **same** row, never duplicates. Extraction failure never
  deletes bytes.
- Pass 2 fires when every material is terminal and runs **only over the parsed** —
  it is told what was skipped (name + reason) so it can flag gaps instead of
  building a confidently incomplete draft.
- **Skipped is never silent**: the chat reply accounts for every material
  («обработано 8 из 10; 2 требуют внимания»). Every material ends in the draft or
  in a visible request — no third bucket.
- An answered request sets the material's `processing_status` to `parsed`; it
  **rejoins the next turn** —
  no special path (idempotent merge does the rest).
- Partially invalid patch: keep valid entries, drop broken ones **plus anything
  referencing them**, report what was dropped. Malformed output → one re-ask
  with the validation errors, then a visible chat error; nothing written.
- A parse stuck beyond a timeout is force-failed; if all files fail, pass 2 is
  skipped (unless the operator's text alone is enough).

### Per input type (pass-1 extractors)

| Type | How it is read | Fallback (a request) |
|---|---|---|
| text | passthrough; still evidence, not trusted truth | — |
| url | guarded best-effort fetch → readable text; SSRF-safe, http(s) only, **no recursive crawling in v1**, no headless browser | "paste the text or drop a screenshot" |
| image | downscale in code, one vision call per file with KB context; OCR + visual summary kept separate; product photos → identification/attachment, infographics/nameplates/price cards → facts | "describe what this is" |
| pdf | native text first (cheap); OCR for scanned pages; page-level provenance; page cap ~10 — **large catalogs are chunked, then merged by natural keys** | "which pages matter / paste the key points" |
| docx | text layer read **in code** — no LLM; legacy `.doc` → fallback | "paste the key points" |
| excel / csv | sheets read **in code** → text table — no LLM; the best-structured source for `products[]` | — |
| audio | transcription **ON** (cap ~5 min): transcript + language + summary; **summarize before synthesis** (filler, corrections); respect temporal intent («старая цена 20, новая 25» → only the new value proposed); timestamps kept in provenance | "describe what this is" |
| video | **phased**: v1 = audio-track **transcript-first** + store on its file-backed `kbd_materials` row; pass 2 may place its id in an allowed video-material column; sampled keyframes optional next; full visual understanding later. Never every frame. | "describe what this is" |

All extractors emit the same evidence shape, so pass 2 is type-blind; upgrading
one extractor never touches synthesis.

**Provider seam:** all model calls go through **one aggregator integration
(OpenRouter, OpenAI-compatible) from day one**; model ids are config, so models
can be added or swapped without touching code. No direct per-vendor SDKs in v1.
Starting defaults and prices: `evals/parsing-costs.md`.

**Cost drivers (relative only; numbers live in `evals/parsing-costs.md`):**
video ≫ audio ≈ scanned-pdf ≈ images ≫ native-pdf ≫ text / url / excel ≈ free.
Cost scales with media detail and duration; a typical mixed batch is cheap
enough that we optimize for extraction quality, not pennies.

### Anti-hallucination & validation

- **Every supported exact value** (price, fee, delivery days, stock state, phone)
  reaches a live typed column **only through approval**. Pass 2 may propose it
  directly in the
  complete draft row with provenance; `confirm_value` is raised only when the
  evidence is ambiguous or conflicting. There are **no confidence thresholds**:
  model confidence may be recorded as metadata, but never gates a write.
  `deletes[]` are confirmed by the same approval — removing knowledge is as
  protected as adding a price.
- Validation before draft write: every request-scoped handle exists in the
  backend sidecar map, maps to a same-org `kbd_materials.id`, and is permitted in
  that field; each media reference is file-backed, has a storage locator,
  matches the column's declared media type, and is customer-sendable;
  evidence-only handles are absent from customer-sendable fields; every entity
  ref and field exists in the mapped `ai_*` schema; values are well-formed;
  duplicate refs merge or raise `resolve_duplicate`. The model cannot invent
  tables, columns, handles, ids, or paths. Unknown anything → that complete
  entry is dropped, fail-closed.
- Validation at approve: schema-valid complete rows, required fields present,
  every media array contains valid UUIDs, referenced files still exist and are
  customer-sendable for this org, and no open `kbd_requests` on the selected
  rows.
- Runtime (the brain): placeholders are resolved from live typed columns — an
  unknown placeholder blocks the reply. Media is selected only by exact
  `<table>.<natural_ref>.<column>` tokens generated from approved live rows;
  code then loads internal ids from that column and resolves file-backed
  `kbd_materials` rows. A guessed or stale token is rejected. The LLM never sees
  or has authority over draft data, ids, paths, prices, counts, or phone numbers.

### Fact value shape

Exact display values use language-neutral **text** such as `25 000 ₸` and `1–3`.
Symbols, ranges, and approved formatting are preserved; the unit/meaning lives
in the purpose-named column (`delivery_in_days`). `in_stock` is the deliberate
boolean exception: code renders `true`/`false` as reviewed Russian wording and
the model never writes the boolean literal to the customer. Other word-bearing
values are trusted Russian prose instead. Considered and rejected: numeric
`price_amount` + `price_currency` split — cleaner typing, but cannot hold the
ranges and approximations small sellers actually use.

---

## Open questions

- **Cleanup policy** for rejected, unreferenced, or no-longer-referenced
  `kbd_materials` rows and their blobs. Referenced file-backed rows are durable.
- **Extraction models & data boundary**: which models run in production, and
  the PII / cross-border stance before enabling extraction on real customer data.
- **In-memory queue loses jobs on restart** → re-enqueue materials whose
  `processing_status` is `uploaded` or `extracting` at startup.
- **KB-size threshold**: at what size does the whole-KB-in-prompt model stop
  working, and what deterministic shortlisting (by category/entity) kicks in
  then.
