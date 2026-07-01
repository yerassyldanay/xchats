# Playground — Build Plan (component 3, made buildable)

The **concrete, buildable design** for the Playground: a **chat where an operator drops a mix of
material (text, URLs, images, PDFs, docs, and video), the assistant extracts and understands it,
builds *or updates* the **one living Knowledge Base**, and a human reviews & approves**. The *concept* and
trade-offs already live in [`10-knowledge-builder.md`](10-knowledge-builder.md) (the UX) and
[`11-ai-design-overview.md`](11-ai-design-overview.md) (the three components + the big decisions).
This doc turns that into layers, a data model, an agent tool-contract, and an endpoint list — the
thing the todo list ([`../todo-playground.md`](../todo-playground.md)) executes.

> Read 11 first for *why*. This doc is *how* and *in what order*.

---

## The core flow (what the operator does)

> The operator drops **any mix of materials** — a URL, several images, a PDF, a plain description,
> a video — and says *"here are my tariffs, understand them and build/update the KB."* The assistant
> **extracts** the content from each input, **synthesizes** it (cross-referencing the pieces and
> diffing against the live KB), proposes new/updated topics · assets · typed facts (products · tariffs · contacts),
> asks via **popups** when unsure, and the operator **reviews & approves**. New/edited rows land as
> **pending** (`drafted_at` set) in the playground; **approve** clears `drafted_at` so the brain reads them.

The hard part is *not* any single extractor — it's **dealing with all input types uniformly** and
turning a pile of heterogeneous material into a coherent, *updated* KB. The design solves that with a
**normalize stage** (below): every input is driven to one common form before any KB reasoning happens.

---

## Locked decisions (this round)

The forks open in doc 11 are settled for v1:

1. **Full chat-agent build** (not the deterministic editor-only slice). The operator bulk-drops
   material; an LLM agent creates/updates topics/assets/values, asks via popups, and the operator
   accepts/denies — per [`10-knowledge-builder.md`](10-knowledge-builder.md). (L2's editor is still
   built first, as the deterministic proof of the write contract — it's a layer, not the end state.)
2. **Normalize any input to a common form, then synthesize.** Per-type **ingest adapters** turn text /
   URL / image / PDF / doc / video into one `NormalizedMaterial` (text + optional bytes + provenance);
   a single **type-agnostic synthesis pass** reasons only over that. Auto-extraction depth is phased:
   **images & docs auto-extract** (vision / text); **URLs best-effort fetch with a paste/screenshot
   fallback**; **video is stored + operator-described** (no transcription in v1).
3. **Build *or* update the *one living KB* — no clone, no published copy.** There is a single living KB
   per org. Opening the playground does **not** clone anything; synthesis writes new/edited rows directly
   into that KB as **pending** (`drafted_at` set) and *diffs* candidates against the **live** rows
   (`drafted_at IS NULL`) to decide update-vs-create, so "update" is real, not blind append. Each content
   row carries `drafted_at`: `NULL` = LIVE (in the prompt), set = PENDING (playground-only, excluded
   from the prompt). This replaces the `review_state` enum *and* the draft-cloned-from-published model.
4. **Approve into the living KB — no version, no swap, no rollback.** The operator edits the one living KB
   freely (topic/asset text, add/replace/delete files, (re)assign an asset's owner, edit values, add
   products/tariffs, accept/deny); **Approve** (one row or all) runs the gate and clears `drafted_at` so
   the brain reads the row. There is no version copy and no rollback — **version history is an accepted
   dropped trade-off** in v1.

These tighten — they don't replace — the design in 10/11.

---

## The gap today (why this is non-trivial)

The KB is currently a **Go literal**: [`backend/internal/brain/seed.go`](../backend/internal/brain/seed.go)
`SeedSnapshot()` returns an in-memory `*domain.Snapshot`. The KB tables designed in
[`9-database-schema.md`](9-database-schema.md) — `ai_snapshots / ai_topics / ai_assets / ai_tariffs / ai_products / ai_contacts` —
**are not in any migration** (only `ai_drafts` / `ai_draft_assets`, the *runtime suggestion* storage,
exist). **A playground that writes the KB cannot write to a Go literal**, so the first thing this
feature must do is land the **writable KB DB layer** the design already specifies. The brain switches
from reading the literal to reading the **live rows** (`drafted_at IS NULL`) of the one living KB from
the DB (literal kept as the seed + fallback). `ai_snapshots` is kept as the table name but is now **one
row per org** (the assistant config: persona/mission/guardrails/language_policy/reply_max_words) — not
versioned.

---

## Ingestion architecture — normalize any input to one common form

The single most important decision. **Don't let the builder agent deal with input types at all.** Put
a per-type normalize stage in front; the agent reasons only over a uniform shape. Two stages, decoupled
by the `NormalizedMaterial` contract:

```text
INPUTS (any mix)        STAGE 1 — INGEST ADAPTERS         STAGE 2 — SYNTHESIS (type-agnostic)
  text ─┐               per type, pluggable;              reads only NormalizedMaterial[]
  url  ─┤   ─────────►  each emits the SAME shape  ─────► + the operator's instruction
  image─┤               ┌── NormalizedMaterial ──┐        + the LIVE KB (drafted_at IS NULL)
  pdf  ─┤               │ source_type, source_ref │         • cross-reference the batch
  doc  ─┤               │ extracted_text  ◄── the │         • diff vs live rows → update | create
  video─┘               │ blob_id?  media_kind?   │         • tokenize numbers → confirm popups
                        │ status, provenance      │         • emit PENDING rows + popups (requests)
                        └─────────────────────────┘
```

**Stage 1 — ingest adapters.** One per input type, all producing the identical `NormalizedMaterial`.
An adapter's only job is *"get the content out as text (+ keep the bytes if it's a sendable asset), or
admit it couldn't."* It never decides KB structure.

| `source_type` | adapter does | bytes kept as asset? | fallback if it can't (→ popup) |
|---|---|---|---|
| `text` | use as-is | no | — |
| `url` | best-effort fetch → readable main text | no (a screenshot routes through `image`) | *"paste the text or drop a screenshot"* |
| `image` | vision-caption — "what it shows **+ when to send it**" | yes | *"describe what this shows"* |
| `pdf` / `doc` | extract text (Go extractor or vision-OCR the pages) | yes (the file) | *"which sections matter / paste the key points"* |
| `video` | **store bytes only — no transcription** | yes | *"describe what this shows"* (always) |
| `audio` | store bytes only (transcription later) | yes | *"describe what this shows"* |

**Stage 2 — synthesis (the builder agent).** It never knows what was a URL vs an image — it sees a bag
of `extracted_text + provenance` and does the actual KB work: cross-reference (the page text *and* the
three tariff images describe the **same** tariffs → **one** `pricing` topic + several asset cards, not
four topics), **diff against the live rows** (update the existing `pricing` topic or create one),
and emit **pending** changes for review.

**Why this is the right shape:**
- **New input type = new adapter.** Nothing downstream changes; "deal with all of them" stops being an
  N-handler problem in the brain.
- **Auto-extraction is a per-adapter upgrade.** Video transcription, better OCR, or a headless-browser
  URL renderer drop in *behind the same shape* later — the synthesis pass never notices. (This is doc
  11 §8's "phased extraction behind the same UX," made structural.)
- **The human fallback IS the pipeline.** Whatever an adapter can't extract becomes a `describe_media`
  popup; the operator's answer lands in the *same* `extracted_text` field. A 403'd URL, a scanned PDF,
  and a video all converge on "operator describes it" with **no special code path**.
- **It reuses the existing seam.** Each material is staged in an `ai_materials` row and extracted by a
  **queue job** (the same in-memory queue + worker the transport uses). Drop 6 inputs → 6 extraction
  jobs → when ready, one synthesis pass. Provenance is free (KB row → material row → original bytes/URL).

> **URL is just one adapter.** Don't over-invest: a best-effort server-side fetch with browser-like
> headers, and on 403 / JS-empty / blocked → the paste-or-screenshot popup (the screenshot reuses the
> `image` adapter). A headless renderer is a later per-adapter upgrade if best-effort proves too weak.

---

## Target architecture

```text
  operator
     │ chat turn: instruction + dropped material (text / url / image / pdf / doc / video)
     ▼
┌──────────────────────────── PLAYGROUND (the only WRITER) ─────────────────────────────┐
│  STAGE 1: ingest adapters  →  ai_materials (NormalizedMaterial: text + bytes + prov.)  │
│                                          │                                              │
│  STAGE 2: Builder chat (LLM agent)       │        Editor (manual, deterministic)        │
│   • reads materials + LIVE KB rows       │         • topic/product/tariff list → body   │
│   • cross-reference + diff (update|create)│        • edit text / add·replace·delete files│
│   • write rows as PENDING (drafted_at)   │         • (re)assign asset owner, edit values│
│   • emit popups (requests) when unsure   │         • approve / deny each pending row     │
└───────────────┬──────────────────────────────────────────────┬────────────────────────┘
                │ writes PENDING rows (drafted_at, provenance)  │ resolves requests
                ▼                                               ▼
        ┌──────────────────── ONE LIVING KB (per org) ───────────────────────────────────┐
        │  ai_snapshots(config) · ai_topics · ai_assets · ai_products · ai_tariffs ·     │
        │  ai_contacts · ai_builder_requests · ai_materials                              │
        │     rows are LIVE   (drafted_at IS NULL)  → read by the brain                   │
        │           or PENDING (drafted_at set)     → playground-only, held out           │
        └───────────────────────────────────┬───────────────────────────────────────────┘
                                             │  [Approve] (one row | all) → gate (every asset
                                             │   described, every live fact token resolves, no
                                             │   open req.) → clear drafted_at → brain reloads
                                             ▼  (NO version, NO copy, NO swap, NO rollback)
                                    BRAIN reads LIVE rows (unchanged read contract)
                                             │
                                             ▼
                                    BRAIN → suggests → human approves → customer
```

The brain's **read contract** (`*domain.Snapshot`) is unchanged; its *source* moves literal → DB (the
**live** rows, `drafted_at IS NULL`), and
its fact rendering moves to a faithful `{{table.slug.field}}` token-substitution model (below). Everything
else new is on the write side.

---

## Build layers (the de-risk order)

Follows doc 11's "lock the contract → seed → editor → chat → approve" sequence. Each layer is
shippable and testable on its own.

### L1 — KB contract: writable, DB-backed *one living KB* + drafted_at lifecycle  *(unblocks everything)*
- Migration `0003_ai_kb.up.sql`: `ai_snapshots`(config, **one row per org**), `ai_topics`, `ai_assets`,
  the typed **fact** tables `ai_tariffs` / `ai_products` / `ai_contacts` (exact values as columns, language
  a row), the `drafted_at` + `provenance` (+ `owner_kind`/`owner_ref` on `ai_assets`) columns,
  `ai_builder_requests`, and `ai_materials` (data model below). `ai_snapshots` keeps its name and the `snapshot_id` FK (legacy names, migration
  continuity) but is no longer versioned — `UNIQUE (organization_id)`, one row per org.
- **Typed fact tables (fixes the lossy bridge).** Exact facts are **typed columns** on `ai_tariffs` /
  `ai_products` / `ai_contacts`, stored **verbatim with units**, one row per `(entity, language)`. Snapshot
  rendering becomes **pure substitution**: `{{table.slug.field}}` → the column value for the reply
  language, falling back to the org default then the `'*'` row; an unresolved token → escalate (never ship
  a half-rendered value). This **replaces** `PriceBook`'s typed `Tariff{PriceTenge int64}` + `formatTenge`
  path, which silently corrupts unit-bearing values (`"25 000 ₸/мес"` → `"25 000 ₸"`), and retires the old
  generic `ai_values` token bag. Re-express the Demo Shop seed as typed rows (`ai_tariffs: basic | ru |
  price "9 900 ₸"`, …). The `*domain.Snapshot` shape the brain consumes stays; only its fact carriers +
  `Render` internals change.
- `internal/kbstore/` — a `KBStore`: `LoadLive(orgID) (*domain.Snapshot, error)` (live rows only,
  `drafted_at IS NULL`); `Open(orgID)` (read live + pending rows — no clone, no copy); draft CRUD for
  topics/assets/**products**/**tariffs**/**contacts** that **stamps `drafted_at`** (pending) on write; material
  create/extract-update; request create/resolve; `Approve(orgID, rowIDs|all)` (run the gate, then **clear
  `drafted_at`** on the approved rows). No `OpenDraft`-clone, no `Publish`-swap, no `Rollback`,
  no `DiscardDraft` — rejecting a pending row is a plain delete.
- **Seed migration**: insert the seed content as **live rows** (`drafted_at IS NULL`) so the brain keeps
  answering. `SeedSnapshot()` stays as the code-level fallback when the table is empty/unreachable.
- **Brain source swap**: `cmd/xchats/main.go` loads the **live** rows from `KBStore` into
  `domain.Content`; reloads on approve; falls back to the literal if DB empty/unreachable.
- **Org scope (v1):** single org — the seeded org. `LoadLive`/`Open` take an `orgID`; v1
  passes the one seeded org.
- Note the `ai_drafts` (live) vs `ai_suggestions` (doc 9) naming divergence in the migration header.
- `go build ./... && go vet ./...`; `kbstore` round-trip test (seed → load → snapshot renders identically
  to the literal, **including unit-bearing values**).

### L2 — Editor (deterministic, no LLM)  *(proves the write contract)*
- Read endpoints over the living KB: config block, topic list (each topic → body + its media gallery,
  each asset with description + kind + owner), **product list**, **tariff list**, **contacts**, the
  request queue. Each row is flagged LIVE or PENDING by its `drafted_at`.
- Write endpoints (each stamps `drafted_at` = now → **pending** + `provenance`): create/update/delete
  topic; **create/update/delete product; upsert/delete tariff**; upload+attach asset with
  **`owner_kind`/`owner_ref`** (topic|product|tariff); update asset description / **reassign owner** /
  delete asset; edit the exact **fact columns** inline on a product/tariff (`price`, `limit_text`, `fee`, …)
  and the **contacts** row (`whatsapp`, `email`, `callback_time`, …); edit
  persona/mission/guardrails/language.
- **Approve / deny** per row: **approve** clears `drafted_at` (after the gate) → the row goes live;
  **deny** deletes the pending row. Manual editor edits land pending like any other write.
- **Optimistic concurrency:** writes carry the row's `updated_at`/version; a stale write → `409`
  (cheap protection so a human edit and a late agent write don't silently clobber). v1 otherwise
  assumes a **single active operator**.
- The **Approve gate** (deterministic, below) → clear `drafted_at` + brain reload. This
  layer is a complete, usable KB CMS *before* any LLM touches it.

### L3 — Ingest adapters (Stage 1: normalize any input → `NormalizedMaterial`)
- `POST /playground/draft/materials` (multipart or JSON): create an `ai_materials` row per input
  (`status='pending'`), store bytes to the blob store (record `media_kind` from MIME), enqueue an
  **extraction job** per material on the in-memory queue.
- Adapters (each fills `extracted_text` or flags `needs_human` → a `describe_media` popup):
  - **text** → pass through.
  - **url** → best-effort fetch → readable main text; on failure → paste/screenshot popup.
  - **image** → multimodal vision call ("what it shows + when to send it") → proposed asset description.
  - **pdf / doc** → extract text (Go extractor or hand pages to the multimodal LLM); detected numbers
    surface later as `confirm_fact` popups (writing the exact value into a typed column), never digits in a body.
  - **video / audio** → store bytes only + `describe_media` popup. No transcription in v1.
- Config: add `LLMVisionModel` (multimodal) alongside `LLMFastModel`; document in `.env.example` /
  `config.example.yaml`.
- All extraction output lands on the **material** (text + status + provenance); nothing is silently
  trusted. Extraction unit tests use a **fake** multimodal client.

### L4 — Builder chat agent (Stage 2: synthesis over the common form)
- A new `internal/playground/builder` agent: input = the operator's chat turn + the **ready materials**
  for this build + a **summary of the live KB** (topics/products/tariffs/contacts, capped); output =
  **tool calls** that write **pending** rows via `KBStore`, plus **requests** (popups) when unsure.
  Tool contract below.
- **Synthesis, not per-file captioning:** cross-reference the materials into coherent
  topics/assets/products/tariffs, **diff against the live rows** (update an existing row vs create),
  tokenize numbers. The agent decides update-vs-create; ambiguity → `resolve_duplicate` / `choose_topic`.
- **Turn budget:** the agent runs a bounded tool loop per turn (cap N tool calls; stop when no new
  material is unprocessed) — no open-ended looping. The live-KB summary, not the full KB, is its context.
- Rides the existing realtime hub: each written pending row and each new `pending` request is broadcast
  so the chat **and** the editor update live over the same KB.
- Tests: **parity** with a fake LLM (price in material → `{{table.slug.field}}` token + `confirm_fact`, never a digit in the
  body; a loose image → `choose_topic`; a re-fed topic → `resolve_duplicate`) **and** a small **golden
  set** of `material → expected pending structure` cases (judged loosely) so the agent's *judgment* —
  not just its plumbing — is measured.

### L5 — Approve → brain (wire the last mile)
- The L2 Approve action (per-row + all), now also surfacing unresolved `pending` requests as approve
  blockers; on success **clears `drafted_at`** on the approved rows + reloads the brain's live view.
  No version, no swap, no rollback. Broadcast `kb.approved`.

---

## Data-model deltas (additive to doc 9)

The core KB tables (config, topics, assets) are exactly doc 9's DDL. The playground adds the
**`drafted_at` lifecycle** fields, three **typed fact** tables (`ai_products` / `ai_tariffs` /
`ai_contacts` — exact values as columns, language a row), and two infra tables (the popup queue
and the ingest staging area) — all additive; the **one living KB** holds both LIVE and PENDING rows
(distinguished by `drafted_at`), and the brain reads only the LIVE rows.

```text
ai_snapshots                           ONE row per org (the assistant config: persona / mission /
   UNIQUE (organization_id)            guardrails / language_policy / reply_max_words) — not versioned.

ai_topics / ai_assets                  gain:
   drafted_at    timestamptz NULL  -- NULL = LIVE (read by the brain); set = PENDING (playground-only)
   provenance    jsonb   -- { "source":"material", "material_id":"…", "at":"…" }
   (this REPLACES the old `review_state` enum — pending/live is the only lifecycle now)

ai_assets                              additionally:
   DROP topic_slug
   owner_kind    text  -- 'topic' | 'product' | 'tariff' | ''   (which entity this MEDIA belongs to)
   owner_ref     text  -- the ref/slug of that entity
   -- NOTE: only MEDIA is polymorphic. Exact VALUES are typed columns on the fact tables below;
   -- `ai_values` is removed. The only way to quote a fact is `{{table.slug.field}}` (a typed column).

ai_products  (typed FACT table — one row per (product, language))
   id            uuid  PK
   snapshot_id   uuid  FK -> ai_snapshots
   ref           text  -- stable handle
   lang          text  -- 'ru' | 'kk' | '*'  — language is a ROW
   name          text  -- verbatim, per language
   price         text  -- confirmed price, verbatim WITH units ('25 000 ₸'); quoted as {{product.<ref>.price}}
   description   text
   category      text
   data          jsonb -- descriptive attrs only (size, color…); no exact numbers — those are columns
   status        text
   drafted_at    timestamptz NULL  -- NULL = LIVE ; set = PENDING
   provenance    jsonb
   created_at, updated_at
   UNIQUE (snapshot_id, ref, lang)

ai_tariffs  (typed FACT table — one row per (tariff, language))
   id            uuid  PK
   snapshot_id   uuid  FK -> ai_snapshots
   ref           text  -- stable handle
   lang          text  -- 'ru' | 'kk' | '*'  — language is a ROW
   name          text  -- verbatim, per language ('Рост' / 'Өсу')
   price         text  -- confirmed price, verbatim with units ('25 000 ₸/мес'); '' if fee-based
   limit_text    text  -- confirmed limit, verbatim ('до 2 000 платежей/мес')
   fee           text  -- per-transaction fee for percentage plans ('1.5 % за транзакцию'); '' otherwise
   summary       text
   pricing_type  text  -- 'fixed' | 'percentage' | 'tiered'
   advantages    text
   disadvantages text
   data          jsonb -- descriptive prose only (conditions / billing_period / tiers)
   status        text
   drafted_at    timestamptz NULL  -- NULL = LIVE ; set = PENDING
   provenance    jsonb
   created_at, updated_at
   UNIQUE (snapshot_id, ref, lang)

ai_contacts  (typed org-support FACT table — singleton slug 'support', one row per language)
   id            uuid  PK
   snapshot_id   uuid  FK -> ai_snapshots
   slug          text  -- 'support' (keeps the 3-part token grammar {{contact.support.<field>}})
   lang          text  -- 'ru' | 'kk' | '*'  — '*' = language-neutral (phones/e-mails/addresses)
   whatsapp      text  -- language-neutral (on the '*' row)
   email         text
   address       text
   legal         text
   callback_time text  -- language-bearing ('1 час' / '1 сағат')
   drafted_at    timestamptz NULL  -- NULL = LIVE ; set = PENDING
   provenance    jsonb
   UNIQUE (snapshot_id, lang)

ai_builder_requests  (the popup / human-in-the-loop queue)
   id            uuid  PK
   snapshot_id   uuid  FK -> ai_snapshots
   material_id   uuid  FK -> ai_materials  NULL  -- the input that raised it, if any
   req_type      text  -- 'describe_media'|'confirm_fact'|'approve_topic'|'approve_asset'
                       --  |'resolve_duplicate'|'choose_topic'|'comment'
   prompt        text  -- what the popup asks
   context       jsonb -- thumbnail ref / detected value / candidate topics — what the popup renders
   target        jsonb -- which row it resolves into (owner_kind/owner_ref / asset ref / table.slug.field)
   state         text  -- 'pending' | 'resolved' | 'dismissed'
   resolution    jsonb NULL  -- the operator's answer (becomes the row mutation)
   created_at, resolved_at

ai_materials  (Stage-1 ⇄ Stage-2 staging: one row per dropped input — the NormalizedMaterial contract)
   id             uuid  PK
   snapshot_id    uuid  FK -> ai_snapshots (the org this build targets)
   source_type    text  -- 'text'|'url'|'image'|'pdf'|'doc'|'video'|'audio'
   source_ref     text  -- url / filename / chat message id
   blob_id        text  NULL  -- stored bytes, if any (candidate asset)
   extracted_text text         -- THE COMMON FORM the synthesis agent reads
   media_kind     text  NULL  -- if sendable as an asset: 'image'|'video'|'document'|'audio'
   status         text  -- 'pending'|'extracting'|'ready'|'needs_human'|'failed'
   extraction     jsonb -- { method, model, confidence, error }
   created_at, updated_at
```

> **Naming note to settle in L1.** Doc 9 calls the runtime suggestion table `ai_suggestions`; the live
> migration uses `ai_drafts` / `ai_draft_assets`. The playground only touches the **KB** tables, so
> it's unaffected — but record the divergence when writing `0003` so reviewers aren't confused.

---

## Builder agent — tool contract

The agent never writes SQL; it calls these (each maps to a `KBStore` mutation or a request). `create_topic`,
`create_product`, `upsert_tariff` and `set_contacts` are **upserts** — the same call updates an existing
row (matched on slug/ref + lang) or creates a new one, which is how "build *or* update" works. Every
written row lands **pending** (`drafted_at` set).

| Tool | Effect |
|---|---|
| `create_topic{slug,lang,title,keywords,body_md}` | upsert a pending `ai_topics` row; existing slug → update |
| `create_product{ref,lang,name,price,description,category,data}` | upsert a pending `ai_products` row (per language); `price` is a **typed column** confirmed via `confirm_fact`; existing (ref,lang) → update |
| `upsert_tariff{ref,lang,name,price,limit_text,fee,pricing_type,advantages,disadvantages,...}` | upsert a pending `ai_tariffs` row (per language); `price`/`limit_text`/`fee` are **typed columns** each confirmed via `confirm_fact`; existing (ref,lang) → update |
| `attach_asset{ref,kind,owner_kind,owner_ref,description,material_id}` | pending `ai_assets` row (media owned by a topic\|product\|tariff) pointing at the material's bytes |
| `confirm_fact{table,slug,field,lang,value}` | popup — the human confirms an exact fact value; on accept it writes the **typed column** on `ai_tariffs`/`ai_products`/`ai_contacts` (never a digit in prose) |
| `set_contacts{lang,whatsapp,email,address,legal,callback_time}` | upsert the pending `ai_contacts` row (per language), each field via `confirm_fact` |
| `describe_media{material_id|asset_ref}` | popup asking the operator for a description (esp. **video** / failed extraction) |
| `choose_topic{asset_ref, candidates[]}` | popup — which owner does this asset belong to |
| `resolve_duplicate{kind, a, b}` | popup — merge or keep both (re-fed URL / overlapping topic/product/tariff) |
| `comment{text}` | a note the operator can answer; steers the next build turn |

**Guardrail (same as the runtime brain):** the agent **never bakes a digit into a topic body** — it
writes a `{{table.slug.field}}` token and raises a `confirm_fact` request; the real number enters the
typed fact column **only** on human confirmation. Numbers are the one thing never auto-written.

---

## Approve gate (deterministic)

Approve (one row or all) is the only pending → live path. It validates the **resulting LIVE set** —
the rows being approved plus the rows already live — and refuses unless:
- every `ai_assets` row (being approved or already live) has a non-empty `description`;
- every `{{table.slug.field}}` used in a **live** `ai_topics.body_md` resolves to a column value in the
  typed fact tables **for each required language** (the "fact-safety = 1.0 / every token resolves" bar);
- no owned media blob dangles (the blob exists);
- no `pending` builder-request remains (an undescribed asset / unconfirmed fact would fail above anyway);
- every referenced entity has a row for each required language (or a `*` fallback).

Approving a **single** row is blocked with a precise reason if it would leave a dangling token (e.g.
approving a topic whose `{{table.slug.field}}` is still pending) — approve that fact row too, or approve-all.

> v1 ships the **structural** gate above (buildable today). The golden-set / asset-precision eval gate
> (`8-ai-assistant.md` → Evals) layers on once the golden set exists — it is not a precondition for L1–L5.

On pass: **clear `drafted_at`** on the approved rows and reload the brain's live view. There is **no
new version, no copy-to-published, no atomic swap, and no rollback** — version history is a dropped
trade-off in v1.

---

## API surface (new — under `/xchats/api/v1/playground`)

```
GET    /playground/draft                      → the living KB: config + topics[+assets] + products + tariffs + contacts + requests + materials (LIVE and PENDING rows, each flagged by drafted_at)
POST   /playground/draft/topics               upsert a topic (lands pending)
DELETE /playground/draft/topics/:slug
POST   /playground/draft/products             upsert a product (lands pending)
DELETE /playground/draft/products/:ref
POST   /playground/draft/tariffs              upsert a tariff (lands pending)
DELETE /playground/draft/tariffs/:ref
POST   /playground/draft/assets               upload bytes + create asset (multipart: file + meta incl. owner_kind/owner_ref)
PATCH  /playground/draft/assets/:ref          edit description / reassign owner_kind+owner_ref
DELETE /playground/draft/assets/:ref
PATCH  /playground/draft/contacts             upsert the org contacts row (per lang): whatsapp/email/address/legal/callback_time
PATCH  /playground/draft/config               persona / mission / guardrails / language_policy
POST   /playground/draft/materials            drop material (Stage 1): bytes/url/text → ai_materials + enqueue extraction
GET    /playground/draft/materials            list materials + extraction status
POST   /playground/draft/approve              run gate over ALL pending rows → clear drafted_at → reload brain
POST   /playground/draft/approve/:kind/:id    approve ONE pending row (kind = topic|product|tariff|asset|contact) → gate → clear drafted_at → reload brain
POST   /playground/chat                       a builder-chat turn (instruction + material_ids) → synthesis pass
GET    /playground/requests                   the popup queue
POST   /playground/requests/:id/resolve       answer a popup → writes a pending row
```
> Denying a pending row is a plain `DELETE` on its `/playground/draft/*` endpoint (no review verb).

Realtime (existing SSE hub): `kb.material.updated` (extraction progress), `kb.row.changed`,
`kb.request.created`, `kb.request.resolved`, `kb.approved` — so chat + editor stay in sync for every viewer.

---

## Frontend

A new **Playground** page (route + NavRail entry), two panes over the **same living KB**:
- **Builder chat** (left): message list + composer with **file-drop + URL paste** (text/url/image/pdf/
  doc/video); dropped material shows an **extraction-status chip** (extracting → ready / needs-you);
  inline **request cards** (popups) rendered where they occur.
- **Editor** (right): config block; topic list (expand → `body_md` editor + media gallery, each asset
  showing kind/description and an **owner selector** — topic|product|tariff — to reassign, plus
  delete/replace); a **Products** list and a **Tariffs** list (each editable, with their **typed fact
  columns** — price/limit_text/fee — and owned media); a **Contacts** panel (whatsapp/email/address/legal/
  callback_time, per language). Every row carries a **pending badge** driven by `drafted_at`; **Approve**
  acts per-row and an **Approve all** button runs the gate and shows failures inline. No version/publish button.

Reuses the existing API envelope, SSE client, blob upload, and auth — no new infra.

---

## Risks & open questions (carry into build)

- **The brain never sees the media** — a bad description = the wrong file sent. Vision-captioning helps,
  but the operator must still review descriptions (it's the selection cue). Keep editing cheap.
- **Synthesis quality is the real risk** — cross-referencing N materials and deciding update-vs-create
  is the agent's hardest job and the one users judge. Measure it (L4 golden set), not just the plumbing.
- **Multimodal cost / data-boundary**: images, doc text, and fetched page text now go to an external
  LLM → settle the PII/cross-border stance before turning extraction on in production (doc 11 §9).
- **Concurrency**: v1 assumes a single active operator + optimistic-concurrency `409`s; the
  agent is turn-synchronous so its writes serialize within a turn. Revisit (row locking / changesets)
  only if multi-operator editing bites.
- **No version history / rollback** — the operator edits the *one* living KB in place, so a bad approve
  can't be rolled back; it must be re-edited and re-approved. This is an accepted v1 trade-off (the
  pending/live `drafted_at` hold-out is the only safety margin); reintroduce versioning later if undo
  proves necessary.
- **Prompt size**: media-as-knowledge grows the prompt fastest; if the KB outgrows the prompt, add
  `pgvector` retrieval later behind the same read contract (doc 8 → Scaling). Not now.

## Out of scope (v1)
- Video / audio **auto-transcription** (stored + operator-described instead).
- **Headless-browser URL rendering** (best-effort fetch + paste/screenshot fallback for now).
- `pgvector` retrieval; KB **version history / rollback**; git-like changesets (doc 11 §7 add-on).
- The `ai_drafts` → `ai_suggestions` runtime-storage rename (separate concern).
