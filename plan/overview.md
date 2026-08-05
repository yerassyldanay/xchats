# xchats Target Overview

> This directory is an implementation plan. [`DECISIONS.md`](../DECISIONS.md)
> is the authoritative product and technical decision record. If this plan,
> current code, or a migration disagrees with it, `DECISIONS.md` wins and the
> plan/code must be corrected. The target is not necessarily implemented yet.

xchats is a multi-tenant social inbox with reusable, channel-neutral AI
assistance. WhatsApp, connected directly via whatsmeow, is the first transport.
Future Instagram, Telegram, and other adapters reuse the normalized
conversation, AI suggestion, approval, knowledge-authoring, and audit paths.

## One implementation vision

There are three deliberately separate lifecycles:

```text
customer channel → normalized conversation → suggestion → operator/policy gate → send

operator material → kbd_materials → extract → synthesize → kbd_draft
                                                        → human approval → ai_*

approved ai_* → cached prompt prefix → {reply_text, media_files_to_send}
              → deterministic validation/substitution → channel adapter
```

- `ai_*` is the approved live knowledge base. It is the only knowledge supplied
  to the customer-response model.
- `kbd_*` is Knowledge Base Development state: source materials, extraction,
  unresolved requests, and one accumulated pending draft per organization.
- `rp_*` is channel-neutral response-suggestion state.
- Channel-prefixed tables such as `wa_*` contain provider transport state; they
  do not leak provider details into shared AI or authoring contracts.

The Vue application talks to one Go backend. The backend is a modular monolith
with interfaces at channel, queue, blob-storage, and model-provider boundaries.
PostgreSQL is the transactional source of truth. File bytes live behind a
storage adapter; `kbd_materials` is their only registry.

## Non-negotiable invariants

1. Drafts, raw materials, extraction notes, internal UUIDs, and storage locators
   never enter a customer-response prompt.
2. The whole approved KB is used by default. There is no vector search or RAG in
   this design. If it becomes too large, code performs deterministic narrowing
   by known category/entity; the threshold remains an open decision.
3. Exact business values live in purpose-named columns and are language-neutral.
   The customer model receives placeholders for those values, not the stored
   numbers. Code substitutes from live rows; an unknown placeholder blocks the
   entire suggestion for manual review. `in_stock` is the deliberate boolean
   exception, and code renders it as reviewed Russian wording.
4. Every operator input is one `kbd_materials` row. Uploaded bytes are durable
   before extraction starts, and extraction failure never deletes them.
5. Files become customer-sendable only through purpose-named media columns on
   approved `ai_*` rows. There is no generic attachment table or relationship.
6. Builder models use short, request-scoped handles. Customer-response models
   use only generated semantic media tokens in `media_files_to_send`. Neither
   model sees or emits database IDs, storage keys, paths, or public object URLs.
7. Knowledge building has two model passes and one human gate: per-material
   extraction, batch synthesis directly into the single accumulated draft, then
   draft-to-live approval. There are no per-job mini-drafts or attachment-linking
   approval steps.
8. Models propose; backend code validates all tables, fields, natural keys,
   values, handles, material ownership, `customer_visibility`, MIME
   compatibility, and semantic media tokens. Unknown or stale control data
   fails closed.
9. V1 trusted KB prose and customer replies are Russian-only. There are no
   per-language live rows or `lang` columns.

## Shared vocabulary

- **Source material:** one text, URL, instruction, or uploaded file stored in
  `kbd_materials`.
- **Parsed material:** a source material whose extraction evidence is ready for
  synthesis.
- **Candidate draft patch:** raw pass-2 output before deterministic validation;
  it is never treated as stored or approved knowledge.
- **KB draft** (UI) / **pending KB changes** (technical): validated deltas held
  in the organization's one `kbd_draft` document.
- **Draft entry:** one complete pending create/update row or delete marker.
- **Live KB** / **approved KB:** rows in `ai_*`, and the only customer-model
  knowledge source.
- **Semantic media token:** an allowlisted `media_files_to_send` reference such
  as `products.sofa-loft.gallery_images`; it selects a complete approved media
  column, not an individual file or storage location.

## Document ownership

- [`architecture.md`](architecture.md): components, boundaries, and end-to-end
  runtime flows.
- [`database-schema.md`](database-schema.md): target tables, columns, keys, and
  invariants.
- [`playground.md`](playground.md): ingestion, two-pass building, failures,
  draft merge, review, and approval.
- [`knowledge-base.md`](knowledge-base.md): live prompt construction, exact-value
  placeholders, semantic media tokens, and response validation.

## Explicit v1 trade-offs and unresolved decisions

V1 has no KB history or rollback, no recursive URL crawler or headless browser,
and no full visual video understanding. The queue may initially be in-memory,
provided startup recovery re-enqueues stuck material work.

Do not silently decide the remaining items while implementing: material/blob
cleanup retention; production extraction models and the PII/cross-border data
boundary; durable-queue timing; and the whole-KB prompt size threshold plus its
deterministic narrowing rule. Capacity targets and production SLO/RPO/RTO also
need product/operational inputs before infrastructure is sized.
