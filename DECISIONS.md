# AI Assistant & Playground — Design Decisions (July 2026)

Status: **agreed in discussion, not yet implemented.** Where this file conflicts
with `plan/*.md` or today's code, **this file wins**. It is the single
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

2. **Exact facts live in typed columns, values language-neutral.** Column names
   explain themselves (`price`, `delivery_in_days`); values are digits, money,
   times, links — never words. One stored value serves Russian and Kazakh replies
   alike. A value that needs words («в наличии», address) is not a fact — it is
   trusted plain text the model phrases itself.

3. **The model never writes numbers — it writes placeholders.** A reply carries
   `{{product.sofa-loft.price}}`; code substitutes the stored value. An unknown
   placeholder blocks the whole draft for manual check. A made-up number is
   silent; a made-up placeholder fails loudly. Wrong prices in writing are a
   business and legal risk.

4. **Media references are concrete, purpose-named columns on complete KB rows.**
   Every input — text, URL, instruction, or uploaded file — lives in
   `kbd_materials`. A file-backed material holds its own storage/extraction
   metadata. Draft and live KB rows store stable `kbd_materials.id` values in
   semantic columns such as `ai_products.featured_image`, `ai_products.images`,
   and `ai_products.certificates`. Database ids and storage paths are
   **backend-only**: no LLM ever sees or emits them. The builder model sees short
   request-scoped handles that code resolves; the customer-response model sees
   only approved semantic send tokens such as
   `products.acer-laptop-444.images`. Unknown handles or tokens fail closed.

5. **The KB is built in the playground in two passes.** The operator drops any mix
   of material (text, URLs, images, PDFs, audio, video). Pass 1 reads each file
   separately; pass 2 turns everything into draft KB entries in the exact KB
   schema — written into the **one draft immediately** (no per-job mini-drafts,
   no intermediate approvals). A human reviews **once**, at draft → live.
   Nothing touches the live KB before Approve.

6. **Every uploaded file has a visibility: `auto | invisible | visible`.**
   Default `auto`: the system decides whether a file is only *evidence* (a
   screenshot, a voice note — its information enters the KB, the file never
   reaches a customer) or *customer-sendable media* (a product photo). The
   operator's explicit choice always wins over the model. A file becomes
   customer-sendable only when its backend material reference is in a concrete
   media column on an **approved live KB row** and the material passes the
   visibility checks.

---

## Part 2 — how it works (technical)

### Core principles

1. **Draft and live schemas are the same.** Draft entries mirror live rows —
   same business columns. Approval validates and upserts; it never translates.
2. **Raw files are stored before extraction.** Upload success is final for the
   bytes; a failed extraction never loses the file.
3. **Parse each material separately.** Every input gets its own row and status.
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
  send token, backend code may resolve the corresponding live column through
  `kbd_materials` to load file bytes.
- `rp_*` is response-suggestion state.
- The model-facing draft uses concise domain keys with a fixed mapping:
  `config → ai_assistants`, `topics → ai_topics`, `products → ai_products`,
  `tariffs → ai_tariffs`, `contacts → ai_contacts`, and
  `policies → ai_policies`. This mapping is code-owned and exhaustive; the model
  cannot invent a target table.
- Files are not KB entities and there is no generic attachment relationship.

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
   organization, storage, visibility, media type, columns, and values
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
files — a visual/audio summary and visibility suggestion. Pass 1
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
`upload.1`–`upload.2` may fill one product's `images`, `upload.3` its
`certificates`, and `upload.4` a tariff's `pricing_images`. Each proposed entity
contains its own concrete media fields; pass 2 never emits a separate attachment
or relationship operation.

Manifest in (trimmed example):

```json
{ "materials": [
    { "handle": "upload.1",
      "visibility": "visible", "source_type": "file", "mime_type": "image/jpeg",
      "status": "parsed",
      "visual_summary": "Front photo of a magnetic drill.", "extracted_text": "" },
    { "handle": "upload.2",
      "visibility": "visible", "source_type": "file", "mime_type": "image/jpeg",
      "status": "parsed",
      "visual_summary": "Nameplate: model ZT-40H, 1450W.",
      "extracted_text": "ZT-40H, 1450W, 820 r/min",
      "detected_facts": [ { "field_hint": "power_watts", "value": "1450",
                            "provenance": "upload.2" } ] },
    { "handle": "evidence.1", "visibility": "invisible",
      "source_type": "file", "mime_type": "image/png", "status": "parsed",
      "visual_summary": "Screenshot of a supplier pricing page.",
      "extracted_text": "ZT-40H wholesale 180 000 ₸" } ] }
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
      "name": "ZT-40H magnetic drill",
      "power_watts": "1450",
      "featured_image": "upload.1",
      "images": ["upload.2"]
    } ] },
  "requests": [] }
```

### Draft = same schema, stored as one jsonb blob per org

- The entire pending KB is **one jsonb document** (`kbd_draft`), one row per org:
  `{ config, topics[], products[], tariffs[], contacts[], policies[], deletes[] }`
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
  file table. It stores pasted text, URLs, operator instructions, and uploaded
  files in one normalized ingestion lifecycle:

  ```text
  id                 uuid primary key
  organization_id    uuid
  source_type        text  -- text | url | instruction | file
  source_ref         text  -- URL, original filename, message id, or source label
  source_text        text  -- raw pasted text/instruction; empty for file
  operator_note      text
  storage_backend    text  -- local | minio | s3 | ...; null for non-file
  storage_key        text  -- provider-specific object key; null for non-file
  filename           text  -- null for non-file
  mime_type          text  -- null for non-file
  size               bigint
  checksum           text
  extracted_text     text
  visual_summary     text
  transcript_text    text
  extraction_json    jsonb
  status             text  -- uploaded | extracting | parsed | built |
                          -- needs_human | failed
  visibility         text  -- auto | invisible | visible; null for non-file
  created_at         timestamptz
  updated_at         timestamptz
  ```

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

### Concrete media-column naming

- A column name must communicate the **business purpose or recognizable media
  group**, because that same name becomes the final segment of the semantic send
  token. Use snake_case. Never use generic columns such as `media`, `files`,
  `assets`, or `attachments`.
- `featured_image` is the standard name for an entity's single main image.
  `hero_image` is acceptable only if it specifically means the website hero;
  `primary_image` is avoided because it is easily confused with a primary key.
- Use correct, explicit group names: `images`, `certificates`, and
  `guarantee_documents`, not concatenated or ambiguous forms such as
  `heroimage` or `garanteelist`.
- Initial media columns are:

  ```text
  ai_topics:
    featured_image
    images
    videos
    audio
    documents

  ai_products:
    featured_image
    images
    demo_videos
    audio
    certificates
    manuals
    guarantee_documents
    documents

  ai_tariffs:
    featured_image
    pricing_images
    explainer_videos
    documents

  ai_contacts:
    contact_card_image
    map_image
    documents

  ai_policies:
    documents

  ai_assistants:
    no media columns in v1
  ```

- Despite their semantic names, the physical values are internal references:
  singular columns such as `featured_image` are nullable `uuid`; group columns
  such as `images` are `uuid[] NOT NULL DEFAULT '{}'`. Every stored UUID is a
  `kbd_materials.id`; no KB media column stores a path, storage key, URL, or
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
- The customer-response prompt exposes a **derived send catalog** using the
  exact format `<table>.<natural_ref>.<column>`. `table` is the unprefixed
  model-facing domain name, not the physical `ai_*` name. Examples:
  `products.acer-laptop-444.featured_image`,
  `products.acer-laptop-444.images`,
  `products.acer-laptop-444.certificates`, and
  `products.acer-laptop-444.guarantee_documents`.
- The token grammar is deliberately closed: exactly three dot-separated
  segments; `table` must be one of the configured model-facing table names;
  `natural_ref` must match an approved row; and `column` must be one of that
  table's declared sendable media columns. Natural refs and column names cannot
  contain dots.
- The send catalog is generated **only from approved `ai_*` rows** and contains
  only tokens for non-empty, currently sendable columns. It contains no draft
  rows, UUIDs, paths, storage keys, filenames, or individual-file entries. The
  model must copy an exact token from the catalog; it cannot construct a valid
  token merely by guessing the format.
- The response contract keeps text and actions separate, for example
  `{ "text": "I will send the photos and certificate.", "send":
  ["products.acer-laptop-444.images",
  "products.acer-laptop-444.certificates"] }`. Tokens are control data for the
  backend and are not shown as prose to the customer.
- When the customer-response model returns a token, code parses the three
  segments, loads the approved row by its natural ref, reads that exact semantic
  column, resolves its internal ids through `kbd_materials`, and sends the one
  file or the complete group. For example, `.featured_image` sends one file and
  `.images` sends every file in that array. No per-file picking from a group.
- Deleting a KB row or removing a material id from a media column never deletes
  the material row or blob automatically. Cleanup is a separate retention job.

### Visibility: `auto | invisible | visible`

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
  handle for evidence-only files; the customer send catalog is materialized
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

- `kbd_requests` — workflow state, **never part of the KB schema**. Types:
  `confirm_value | describe_file | choose_media_column | resolve_duplicate |
  conflict`.
- Anti-staleness rule: every request targets data **by natural key**
  (table+ref+field / material id). Editing or resolving the target **auto-resolves
  the request** — a request can never ask about a state that no longer exists.
- Approve is blocked only by open requests attached to the rows being approved;
  unrelated open requests never block.
- Ambiguity that needs conversation (mixed intent, unclear instruction) is
  asked **in the chat**, not stored; the operator's next message answers it.

### Errors: skip the failed, proceed with the rest

- `kbd_materials` statuses: `uploaded → extracting → parsed | needs_human | failed`,
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
- An answered request makes the material `parsed`; it **rejoins the next turn** —
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

- **Every exact value** (price, count, limit, phone, dimension) reaches a live
  typed column **only through approval**. Pass 2 may propose it directly in the
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

Language-neutral **text** values: «25 000 ₸», «от 5 000», «1–3». Symbols and
ranges allowed; units live in the column name (`delivery_in_days`); word-bearing
values are trusted prose instead. Considered and rejected: numeric
`price_amount` + `price_currency` split — cleaner typing, but cannot hold the
ranges and approximations small sellers actually use.

---

## Open questions

- **Cleanup policy** for rejected, unreferenced, or no-longer-referenced
  `kbd_materials` rows and their blobs. Referenced file-backed rows are durable.
- **Extraction models & data boundary**: which models run in production, and
  the PII / cross-border stance before enabling extraction on real customer data.
- **In-memory queue loses jobs on restart** → re-enqueue materials stuck in
  `uploaded`/`extracting` at startup.
- **KB-size threshold**: at what size does the whole-KB-in-prompt model stop
  working, and what deterministic shortlisting (by category/entity) kicks in
  then.
