# Playground — Build Plan (component 3, made buildable)

The **concrete, buildable design** for the Playground: a **chat where an operator drops a mix of
material (text, URLs, images, PDFs, docs, and video), the assistant extracts and understands it,
builds *or updates* the Knowledge Base, and a human reviews & publishes**. The *concept* and
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
> diffing against the existing KB), proposes new/updated topics · assets · value tokens, asks via
> **popups** when unsure, and the operator **reviews & publishes**.

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
3. **Build *or* update, over a draft cloned from the published KB.** Opening the playground clones the
   current published snapshot into a single working **draft**; synthesis *diffs* candidates against it
   (update an existing topic vs create a new one), so "update" is real, not blind append.
4. **Full draft → published with a rich publish window.** The operator edits the draft freely before
   publishing (topic/asset text, add/replace/delete files, (re)assign an asset's topic, edit values,
   accept/deny), then Publish runs the gate and swaps live.

These tighten — they don't replace — the design in 10/11.

---

## The gap today (why this is non-trivial)

The KB is currently a **Go literal**: [`backend/internal/brain/seed.go`](../backend/internal/brain/seed.go)
`SeedSnapshot()` returns an in-memory `*domain.Snapshot`. The KB tables designed in
[`9-database-schema.md`](9-database-schema.md) — `ai_snapshots / ai_topics / ai_assets / ai_values` —
**are not in any migration** (only `ai_drafts` / `ai_draft_assets`, the *runtime suggestion* storage,
exist). **A playground that writes the KB cannot write to a Go literal**, so the first thing this
feature must do is land the **writable KB DB layer** the design already specifies. The brain switches
from reading the literal to reading the *published* snapshot from the DB (literal kept as the seed +
fallback).

---

## Ingestion architecture — normalize any input to one common form

The single most important decision. **Don't let the builder agent deal with input types at all.** Put
a per-type normalize stage in front; the agent reasons only over a uniform shape. Two stages, decoupled
by the `NormalizedMaterial` contract:

```text
INPUTS (any mix)        STAGE 1 — INGEST ADAPTERS         STAGE 2 — SYNTHESIS (type-agnostic)
  text ─┐               per type, pluggable;              reads only NormalizedMaterial[]
  url  ─┤   ─────────►  each emits the SAME shape  ─────► + the operator's instruction
  image─┤               ┌── NormalizedMaterial ──┐        + the cloned DRAFT KB
  pdf  ─┤               │ source_type, source_ref │         • cross-reference the batch
  doc  ─┤               │ extracted_text  ◄── the │         • diff vs the draft → update | create
  video─┘               │ blob_id?  media_kind?   │         • tokenize numbers → confirm popups
                        │ status, provenance      │         • emit PROPOSED rows + popups (requests)
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
four topics), **diff against the cloned draft** (update the existing `pricing` topic or create one),
and emit proposed changes for review.

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
│  STAGE 2: Builder chat (LLM agent)       │        Draft editor (manual, deterministic)  │
│   • reads materials + cloned draft KB    │         • topic list → body + media + values │
│   • cross-reference + diff (update|create)│        • edit text / add·replace·delete files│
│   • emit tool calls → draft rows         │         • (re)assign asset.topic, edit values│
│   • emit popups (requests) when unsure   │         • accept / deny each proposed row    │
└───────────────┬──────────────────────────────────────────────┬────────────────────────┘
                │ writes (review_state, provenance)             │ resolves requests
                ▼                                               ▼
        ┌──── DRAFT snapshot (ai_*, state='draft'; cloned from published on open) ────────┐
        │  ai_snapshots · ai_topics · ai_assets · ai_values · ai_builder_requests · ai_materials │
        └───────────────────────────────────┬───────────────────────────────────────────┘
                                             │  [Publish] → gate (every asset described,
                                             │              every value token resolves, no open req.)
                                             ▼  atomic swap + brain reload
        ┌─────────────── PUBLISHED snapshot (ai_*, state='published') ───────────────────┐
        └───────────────────────────────────┬───────────────────────────────────────────┘
                                             │ reads only (existing brain, unchanged read contract)
                                             ▼
                                    BRAIN → suggests → human approves → customer
```

The brain's **read contract** (`*domain.Snapshot`) is unchanged; its *source* moves literal → DB, and
its value rendering moves to a faithful token-substitution model (below). Everything else new is on the
write side.

---

## Build layers (the de-risk order)

Follows doc 11's "lock the contract → seed → editor → chat → publish" sequence. Each layer is
shippable and testable on its own.

### L1 — KB contract: writable, DB-backed KB + draft lifecycle  *(unblocks everything)*
- Migration `0003_ai_kb.up.sql`: `ai_snapshots`, `ai_topics`, `ai_assets`, `ai_values` per
  [`9-database-schema.md`](9-database-schema.md), **plus** the draft-side columns + `ai_builder_requests`
  + `ai_materials` (data model below). Add the **one-draft-per-org** guard:
  `UNIQUE (organization_id) WHERE snapshot_state='draft'`.
- **Generic value model (fixes the lossy bridge).** `ai_values` is `(token, lang) → value_text` (free
  text, any unit). Snapshot rendering becomes **pure substitution**: `{{namespace.key}}` → `value_text`
  for the reply language, falling back to the `'*'` row; an unresolved token → escalate (never ship a
  half-rendered value). This **replaces** `PriceBook`'s typed `Tariff{PriceTenge int64}` + `formatTenge`
  path, which silently corrupts values with units (`"25 000 ₸/мес"` → `"25 000 ₸"`). Re-express the
  Demo Shop seed as `ai_values` rows (`price.basic | ru | "9 900 ₸"`, …). The `*domain.Snapshot` shape
  the brain consumes stays; only its `Values` carrier + `Render` internals change.
- `internal/kbstore/` — a `KBStore`: `LoadPublished(orgID) (*domain.Snapshot, error)`;
  `OpenDraft(orgID)` (clone published → single draft); draft CRUD for topics/assets/values; material
  create/extract-update; request create/resolve; `Publish(orgID)` (atomic draft→published + version);
  `Rollback(orgID, version)`; `DiscardDraft(orgID)`.
- **Seed migration**: insert the seed content as **version 1, published** so the brain keeps answering.
  `SeedSnapshot()` stays as the code-level fallback when the table is empty/unreachable.
- **Brain source swap**: `cmd/xchats/main.go` loads the published snapshot from `KBStore` into
  `domain.Content`; reloads on publish; falls back to the literal if DB empty/unreachable.
- **Org scope (v1):** single org — the seeded org. `LoadPublished`/`OpenDraft` take an `orgID`; v1
  passes the one seeded org.
- Note the `ai_drafts` (live) vs `ai_suggestions` (doc 9) naming divergence in the migration header.
- `go build ./... && go vet ./...`; `kbstore` round-trip test (seed → load → snapshot renders identically
  to the literal, **including unit-bearing values**).

### L2 — Draft editor (deterministic, no LLM)  *(proves the write contract)*
- Read endpoints for the active draft: config blocks, topic list (each topic → body + its media
  gallery, each asset with description + kind + assigned topic), the value book, the request queue.
- Write endpoints (all draft-only, each stamps `review_state` + `provenance`): create/update/delete
  topic; upload+attach asset; update asset description / **reassign `topic_slug`** / delete asset;
  create/update/delete value; edit persona/mission/guardrails/language. Open / discard the draft.
- **Accept / deny** per row (`proposed → approved | rejected`). Manual editor rows default `approved`;
  agent rows default `proposed`.
- **Optimistic concurrency:** writes carry the row's `updated_at`/version; a stale write → `409`
  (cheap protection so a human edit and a late agent write don't silently clobber). v1 otherwise
  assumes a **single active operator** per draft.
- The **Publish gate** (deterministic) → atomic swap to a new published version + brain reload. This
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
    surface later as `confirm_price`/`propose_value`, never digits in a body.
  - **video / audio** → store bytes only + `describe_media` popup. No transcription in v1.
- Config: add `LLMVisionModel` (multimodal) alongside `LLMFastModel`; document in `.env.example` /
  `config.example.yaml`.
- All extraction output lands on the **material** (text + status + provenance); nothing is silently
  trusted. Extraction unit tests use a **fake** multimodal client.

### L4 — Builder chat agent (Stage 2: synthesis over the common form)
- A new `internal/playground/builder` agent: input = the operator's chat turn + the **ready materials**
  for this build + a **summary of the current draft** (topics/values, capped); output = **tool calls**
  that mutate the draft via `KBStore`, plus **requests** (popups) when unsure. Tool contract below.
- **Synthesis, not per-file captioning:** cross-reference the materials into coherent topics/assets,
  **diff against the draft** (update an existing topic/value vs create), tokenize numbers. The agent
  decides update-vs-create; ambiguity → `resolve_duplicate` / `choose_topic`.
- **Turn budget:** the agent runs a bounded tool loop per turn (cap N tool calls; stop when no new
  material is unprocessed) — no open-ended looping. The draft summary, not the full draft, is its context.
- Rides the existing realtime hub: each created/updated row and each new `pending` request is broadcast
  so the chat **and** the editor update live over the same draft.
- Tests: **parity** with a fake LLM (price in material → token + `confirm_price`, never a digit in the
  body; a loose image → `choose_topic`; a re-fed topic → `resolve_duplicate`) **and** a small **golden
  set** of `material → expected draft structure` cases (judged loosely) so the agent's *judgment* — not
  just its plumbing — is measured.

### L5 — Publish → brain (wire the last mile)
- The L2 Publish button, now also surfacing unresolved `pending` requests as publish blockers; on
  success swaps live + reloads the brain snapshot; rollback re-publishes a prior version. Broadcast
  `kb.published`.

---

## Data-model deltas (additive to doc 9)

The four KB tables are exactly doc 9's DDL. The playground adds **draft-side** fields + two tables
(the popup queue and the ingest staging area) — all additive; the same KB tables hold both draft and
published rows (distinguished by their snapshot's state), and the brain ignores the draft columns.

```text
ai_snapshots                           gains the one-draft-per-org guard:
   UNIQUE (organization_id) WHERE snapshot_state='draft'

ai_topics / ai_assets / ai_values      gain:
   review_state  text    -- 'proposed' | 'approved' | 'rejected'   (default 'proposed' for agent rows;
                         --  'approved' for manual editor rows)
   provenance    jsonb   -- { "source":"material", "material_id":"…", "at":"…" }

ai_builder_requests  (the popup / human-in-the-loop queue, keyed to the DRAFT)
   id            uuid  PK
   snapshot_id   uuid  FK -> ai_snapshots (the DRAFT)
   material_id   uuid  FK -> ai_materials  NULL  -- the input that raised it, if any
   req_type      text  -- 'describe_media'|'confirm_price'|'approve_topic'|'approve_asset'
                       --  |'resolve_duplicate'|'choose_topic'|'comment'
   prompt        text  -- what the popup asks
   context       jsonb -- thumbnail ref / detected value / candidate topics — what the popup renders
   target        jsonb -- which draft row it resolves into (topic_slug / asset ref / token)
   state         text  -- 'pending' | 'resolved' | 'dismissed'
   resolution    jsonb NULL  -- the operator's answer (becomes the row mutation)
   created_at, resolved_at

ai_materials  (Stage-1 ⇄ Stage-2 staging: one row per dropped input — the NormalizedMaterial contract)
   id             uuid  PK
   snapshot_id    uuid  FK -> ai_snapshots (the DRAFT this build targets)
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

The agent never writes SQL; it calls these (each maps to a `KBStore` mutation or a request). `create_topic`
and `propose_value` are **upserts** — the same call updates an existing row or creates a new one, which
is how "build *or* update" works.

| Tool | Effect |
|---|---|
| `create_topic{slug,lang,title,keywords,body_md}` | upsert a draft `ai_topics` row (`proposed`); existing slug → update |
| `attach_asset{ref,kind,topic_slug,description,material_id}` | draft `ai_assets` row pointing at the material's bytes |
| `propose_value{token,lang,value_text,description}` | raises `confirm_price` → on accept, upsert a draft `ai_values` row |
| `describe_media{material_id|asset_ref}` | popup asking the operator for a description (esp. **video** / failed extraction) |
| `choose_topic{asset_ref, candidates[]}` | popup — which topic does this asset belong to |
| `resolve_duplicate{kind, a, b}` | popup — merge or keep both (re-fed URL / overlapping topic/value) |
| `comment{text}` | a note the operator can answer; steers the next build turn |

**Guardrail (same as the runtime brain):** the agent **never bakes a digit into a topic body** — it
writes a `{{token}}` and a `propose_value`/`confirm_price` request; the real number enters `ai_values`
only on human confirmation. Numbers are the one thing never auto-written.

---

## Publish gate (deterministic)

Publish is the only draft → published path. It refuses unless, over **approved** rows:
- every `ai_assets` row has a non-empty `description`;
- every `{{namespace.key}}` used in an approved `ai_topics.body_md` resolves in `ai_values` (the
  "price-safety = 1.0 / every token resolves" bar);
- no asset URL dangles (the blob exists);
- no `pending` request remains (an undescribed asset / unconfirmed value would fail above anyway).

> v1 ships the **structural** gate above (buildable today). The golden-set / asset-precision eval gate
> (`8-ai-assistant.md` → Evals) layers on once the golden set exists — it is not a precondition for L1–L5.

On pass: create a new `version`, copy approved rows into a `published` snapshot, atomically swap, bump
the brain's `domain.Content`. Rollback = re-publish a prior version.

---

## API surface (new — under `/xchats/api/v1/playground`)

```
GET    /playground/draft                      → active draft (config + topics[+assets] + values + requests + materials)
POST   /playground/draft                      open a draft (clone the published snapshot) — idempotent (one per org)
DELETE /playground/draft                      discard the working draft
POST   /playground/draft/topics               upsert a draft topic
DELETE /playground/draft/topics/:slug
POST   /playground/draft/assets               upload bytes + create asset (multipart: file + meta)
PATCH  /playground/draft/assets/:ref          edit description / reassign topic_slug
DELETE /playground/draft/assets/:ref
POST   /playground/draft/values               upsert a value token
DELETE /playground/draft/values/:token
PATCH  /playground/draft/config               persona / mission / guardrails / language_policy
POST   /playground/draft/materials            drop material (Stage 1): bytes/url/text → ai_materials + enqueue extraction
GET    /playground/draft/materials            list materials + extraction status
POST   /playground/draft/topics/:id/review    accept | deny  (review_state)   (likewise assets/:id, values/:id)
POST   /playground/chat                       a builder-chat turn (instruction + material_ids) → synthesis pass
GET    /playground/requests                   the popup queue
POST   /playground/requests/:id/resolve       answer a popup → mutates the draft
POST   /playground/publish                    run gate → swap live → reload brain
POST   /playground/rollback                   { version } → re-publish a prior version
```
Realtime (existing SSE hub): `kb.material.updated` (extraction progress), `kb.row.changed`,
`kb.request.created`, `kb.request.resolved`, `kb.published` — so chat + editor stay in sync for every viewer.

---

## Frontend

A new **Playground** page (route + NavRail entry), two panes over the **same draft**:
- **Builder chat** (left): message list + composer with **file-drop + URL paste** (text/url/image/pdf/
  doc/video); dropped material shows an **extraction-status chip** (extracting → ready / needs-you);
  inline **request cards** (popups) rendered where they occur.
- **Draft editor** (right): config blocks; topic list (expand → `body_md` editor + media gallery, each
  asset showing kind/description and a **topic selector** to reassign, plus delete/replace); the value
  book (token → value, editable); a **review-queue badge**; a **Publish** button that shows gate
  failures inline.

Reuses the existing API envelope, SSE client, blob upload, and auth — no new infra.

---

## Risks & open questions (carry into build)

- **The brain never sees the media** — a bad description = the wrong file sent. Vision-captioning helps,
  but the operator must still review descriptions (it's the selection cue). Keep editing cheap.
- **Synthesis quality is the real risk** — cross-referencing N materials and deciding update-vs-create
  is the agent's hardest job and the one users judge. Measure it (L4 golden set), not just the plumbing.
- **Multimodal cost / data-boundary**: images, doc text, and fetched page text now go to an external
  LLM → settle the PII/cross-border stance before turning extraction on in production (doc 11 §9).
- **Concurrency**: v1 assumes a single active operator per draft + optimistic-concurrency `409`s; the
  agent is turn-synchronous so its writes serialize within a turn. Revisit (row locking / changesets)
  only if multi-operator editing bites.
- **Prompt size**: media-as-knowledge grows the prompt fastest; if the KB outgrows the prompt, add
  `pgvector` retrieval later behind the same read contract (doc 8 → Scaling). Not now.

## Out of scope (v1)
- Video / audio **auto-transcription** (stored + operator-described instead).
- **Headless-browser URL rendering** (best-effort fetch + paste/screenshot fallback for now).
- `pgvector` retrieval; branchable drafts; git-like changesets (doc 11 §7 add-on).
- The `ai_drafts` → `ai_suggestions` runtime-storage rename (separate concern).
