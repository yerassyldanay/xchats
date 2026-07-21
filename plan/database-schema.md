# Target Database Schema

[`DECISIONS.md`](../DECISIONS.md) is authoritative. All tables are in PostgreSQL
schema `xchats`; timestamps are `timestamptz`; application-generated identifiers
are UUID unless a column says otherwise. Every tenant-owned query and unique key
includes `organization_id` (directly or through an owning row).

This is the target contract. Existing migrations may lag and must migrate toward
it; legacy `ai_assets`, generic value bags, or per-job draft tables are not part
of the target.

## Shared and channel transport

```text
organizations — tenant and response policy
  id, name, respond_mode, respond_window_start, respond_window_end,
  created_at, updated_at

users — operators
  id, email, password_hash, display_name, created_at, updated_at

organization_users — tenant membership
  organization_id, user_id, joined_at

sessions — authentication (`id` is text)
  id, user_id, created_at, expires_at

wa_accounts — WhatsApp numbers and Evolution instances
  id, organization_id, display_name, owner_jid, phone_number,
  evolution_instance_name, evolution_instance_id, connection_state,
  last_live_event_at, deleted_at, created_at, updated_at

wa_contacts — account-scoped WhatsApp contacts
  id, account_id, phone_number, phone_jid, lid_jid, push_name, display_name,
  avatar_url, attributes, created_at, updated_at

wa_chats — WhatsApp conversations
  id, account_id, contact_id, remote_jid, chat_state, assignee_user_id, stage,
  ai_summary, last_message_at, last_message_preview, unread_count,
  created_at, updated_at

wa_messages — normalized WhatsApp messages plus provider audit payload
  id, account_id, chat_id, direction, sender_kind, sender_user_id,
  evolution_message_id, participant_jid, message_kind, body, delivery_state,
  source, raw, message_ts, created_at, updated_at

message_media — media belonging to a conversation message
  id, message_id, media_type, mime_type, filename, size,
  storage_backend, storage_key, download_status, created_at, updated_at
```

Future channels add transport tables such as `ig_*` and `tg_*` behind the same
normalized contracts. They do not add channel-specific columns to shared AI,
authoring, or suggestion tables.

## Approved live knowledge (`ai_*`)

`DECISIONS.md` §"Canonical knowledge-base schema" is the authoritative
column-by-column contract; this section mirrors it and must never drift from it.
V1 trusted prose is Russian-only and there are no `lang` columns; exact values
are stored once in purpose-named columns. Aliases are rejected everywhere
(`delivery_in_days`, never `delivery_time`; `working_hours`, never `work_hours`;
`gallery_images`, never generic `images`). Prose columns are trusted meaning the
model phrases itself; exact facts must not be hidden inside a prose field or a
generic JSON object.

```text
ai_assistants — singleton per organization
  id, organization_id, persona, mission, guardrails, language_policy,
  reply_max_words, created_at, updated_at
  UNIQUE (organization_id)

ai_topics — explanatory trusted prose
  id, organization_id, slug, title, keywords, body_md,
  featured_image, illustration_images, explainer_videos,
  narration_audio_files, reference_documents,
  created_at, updated_at
  UNIQUE (organization_id, slug)

ai_products — products plus purpose-named exact/prose fields
  id, organization_id, ref, name, price, description, category, in_stock,
  sales_status, featured_image, gallery_images, demo_videos,
  audio_description_files, certificate_documents, manual_documents,
  guarantee_documents, specification_documents, created_at, updated_at
  UNIQUE (organization_id, ref)

ai_tariffs — plans plus purpose-named exact/prose fields
  id, organization_id, ref, name, price, limit_text, fee, summary,
  pricing_type, advantages, disadvantages, sales_status, featured_image,
  pricing_images, explainer_videos, terms_documents, created_at, updated_at
  UNIQUE (organization_id, ref)

ai_contacts — singleton approved organization contact facts/prose
  id, organization_id, whatsapp, email, address, legal_information,
  callback_time, working_hours, phone, website, instagram,
  contact_card_image, location_map_image, company_legal_documents,
  created_at, updated_at
  UNIQUE (organization_id)

ai_policies — singleton approved commerce policy facts/prose
  id, organization_id, delivery_cost, delivery_in_days, free_delivery_from,
  min_order, prepayment, installment, return_period_in_days, warranty,
  commerce_policy_documents, created_at, updated_at
  UNIQUE (organization_id)

ai_audit_log — append-only approval history
  id, organization_id, action, actor_user_id, note, created_at
```

`price`, `fee`, `delivery_cost`, `delivery_in_days`, `free_delivery_from`,
`min_order`, `return_period_in_days`, `whatsapp`, `phone`, `email`, `website`,
and language-neutral `working_hours` are examples of semantic exact-value
columns. They are stored as text because approved values may contain symbols and
ranges (`25 000 ₸`, `≈ 5 000 ₸`, `1–3`), but their meaning/unit comes from the
column name. `in_stock` is the deliberate `boolean` exception: code renders
`true`/`false` as reviewed Russian wording and the model never writes the
boolean literal to the customer. A missing scalar is SQL `NULL`, never an empty
or whitespace-only string; an empty exact value generates no placeholder, so a
question that needs it must escalate. Word-bearing information belongs in a
named trusted-prose column. `address`, `legal_information`, `callback_time`,
`limit_text`, `summary`, `advantages`, `disadvantages`, `prepayment`,
`installment`, and `warranty` are prose and must not be used to smuggle exact
numeric facts past the placeholder system. A new kind of exact fact requires an
explicit column migration and placeholder-validator support; there is no
generic `data` fact bag.

For model-facing singleton references, code uses the fixed natural ref `main`
(for example `{{policy.main.delivery_cost}}` and
`policies.main.commerce_policy_documents`). `main` is code-owned and does not
require a mutable database slug.

### Concrete media columns

The closed v1 media schema (purpose-named; generic `image/images/videos/audio/
documents/media/files/assets/attachments` are forbidden as column names):

```text
ai_topics:
  featured_image; illustration_images; explainer_videos;
  narration_audio_files; reference_documents

ai_products:
  featured_image; gallery_images; demo_videos; audio_description_files;
  certificate_documents; manual_documents; guarantee_documents;
  specification_documents

ai_tariffs:
  featured_image; pricing_images; explainer_videos; terms_documents

ai_contacts:
  contact_card_image; location_map_image; company_legal_documents

ai_policies:
  commerce_policy_documents

ai_assistants:
  no media columns in v1
```

Singular media columns are nullable UUIDs with a normal foreign key to
`kbd_materials.id`. Plural media columns are `uuid[] NOT NULL DEFAULT '{}'`.
PostgreSQL cannot enforce a foreign key on each array element, so code validates
every element before draft write, approval, and runtime delivery. Every
reference must resolve to a same-organization, file-backed material with a
storage locator, compatible MIME type, and customer-sendable
`customer_visibility`.

No live row stores paths, storage keys, public object URLs, model handles, or
generic attachments. Adding a media purpose requires a migration plus builder,
approval, prompt-catalog, and runtime-validator support.

## Knowledge Base Development (`kbd_*`)

### `kbd_draft`

One row per organization:

```text
organization_id  uuid primary key references organizations(id)
draft            jsonb not null
base_version     bigint not null default 0
updated_at       timestamptz not null
updated_by       uuid null references users(id)
```

`base_version` is the compare-and-swap counter for concurrent draft edits. The
historical name does not mean “base live version”; it is not a live KB version
and not a per-entry lifecycle field. A stale write returns HTTP 409.

The JSON document is delta-only:

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

Sections map exhaustively to physical tables:
`assistant → ai_assistants`, `topics → ai_topics`, `products → ai_products`,
`tariffs → ai_tariffs`, `contacts → ai_contacts`, and
`policies → ai_policies`; `contacts` and `policies` are nullable singleton
objects, not arrays. Every create/update is a complete business row (no
database ID, tenant ID, or timestamps inside the entry); delete markers name a
mapped table and natural key. Media fields already contain resolved internal
`kbd_materials.id` values when stored. There is no attachment section.

Entry state is derived by comparing draft and live: draft-only = **Новый**;
draft shadowing live = **Изменён**; delete marker = **К удалению**; no draft
entry = published. Field diffs are computed, not stored. There are no
`change_type`, `review_status`, `base_live_version`, or `changed_fields_json`
columns. An entry identical to live is removed from the delta.

Field/source provenance needed by review and “reject this upload” remains
backend-only merge metadata tied to material rows in `extraction_metadata`, using
natural table/ref/field targets. It is not a model-visible ID, a live KB column,
or a second approval lifecycle.

### `kbd_materials`

This is the only material and uploaded-file registry:

```text
id                  uuid primary key
organization_id     uuid not null references organizations(id)
source_type         text not null  -- text | url | instruction | file
source_ref          text
source_text         text
operator_note       text
storage_backend     text null      -- required only for file
storage_key         text null      -- required only for file
filename            text null      -- file only
mime_type           text null      -- file only
size_bytes          bigint null    -- > 0 for files
sha256_checksum     text null
extracted_text      text
visual_summary      text
transcript_text     text
extraction_metadata jsonb not null default '{}'
processing_status   text not null default 'uploaded'
                                  -- uploaded | extracting | parsed | built |
                                  -- needs_human | failed
customer_visibility text null      -- auto | invisible | visible; file only
created_at          timestamptz not null
updated_at          timestamptz not null
```

Checks enforce the closed `source_type`, `processing_status`, and
`customer_visibility` values and file/non-file storage shapes. For URLs,
`source_ref` is the URL and `extracted_text` is the
guarded readable snapshot. For text/instructions, `source_text` preserves the
input. For files, storage and extraction fields describe the already-durable
bytes. `built` means pass 2 consumed the evidence; it never implies that a
referenced row or blob may be deleted.

### `kbd_requests`

```text
id               uuid primary key
organization_id  uuid not null references organizations(id)
material_id      uuid null references kbd_materials(id)
request_type     text not null  -- confirm_value | describe_file |
                               -- choose_media_column | resolve_duplicate |
                               -- resolve_conflict
question_text    text not null
question_context jsonb not null default '{}'
target_field     jsonb not null
request_status   text not null default 'open'  -- open | resolved
resolution       jsonb null
created_at       timestamptz not null
resolved_at      timestamptz null
```

`target_field` addresses live/draft data by mapped table + natural ref + field,
or a material by backend UUID. Editing, deleting, or resolving the target
automatically resolves the request. Approval queries open requests only for the
selected entries; unrelated requests do not block.

## Response suggestions (`rp_*`)

```text
rp_suggestions — channel-neutral response suggestions and approval state
  id, organization_id, channel, conversation_id, trigger_message_id,
  requested_by_user_id, state, reply_language, confidence, escalate,
  escalation_reason, options, suggested_status, suggested_callback,
  chosen_ordinal, sent_message_id, context_state, created_at, updated_at
```

Each option stores the validated channel-neutral customer-response contract
(`reply_text`, `reply_language`, `media_files_to_send`, `escalate`,
`escalation_reason`, `confidence` — see DECISIONS.md §"Customer-response JSON
contract"). `media_files_to_send` entries are semantic media-catalog tokens —
control data, never customer prose, and never UUIDs, filenames, paths, or
storage keys. The legacy field names `asset_refs`, `attach_groups`, and `send`
are not aliases and must not appear anywhere. At most one active suggestion is
allowed per `(organization_id, channel, conversation_id)` (normally a partial
unique index over active states).

## Required integrity and indexes

- Natural-key unique constraints listed above drive idempotent draft/live
  upserts.
- Index all foreign keys and primary access paths: tenant membership, account
  chats/messages by time, open `kbd_requests`, `kbd_materials` by
  org/`processing_status`,
  suggestions by conversation/state, and audit rows by org/time.
- Approval and draft merge use transactions and row locks/version checks.
- Cleanup may remove a material/blob only after checking every live singular
  media reference, every live array element, and the current draft. The actual
  retention policy is intentionally unresolved in v1 planning.
