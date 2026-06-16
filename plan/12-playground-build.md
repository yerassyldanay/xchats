# Playground — Build Plan (component 3, made buildable)

The **concrete, buildable design** for the Playground: a **chat where an operator drops material
(images, video, PDFs, docs, plain descriptions), the assistant structures it into the Knowledge
Base, and a human reviews & publishes**. The *concept* and trade-offs already live in
[`10-knowledge-builder.md`](10-knowledge-builder.md) (the UX) and
[`11-ai-design-overview.md`](11-ai-design-overview.md) (the three components + the big decisions).
This doc turns that into layers, a data model, an agent tool-contract, and an endpoint list — the
thing the todo list ([`../todo-playground.md`](../todo-playground.md)) executes.

> Read 11 first for *why*. This doc is *how* and *in what order*.

---

## Locked decisions (this round)

Three forks were open in doc 11; they're now settled for v1:

1. **Full chat-agent build** (not the deterministic editor-only slice). The playground is a chat: the
   operator bulk-drops material, an LLM agent creates topics/assets/values, asks via popups, and the
   operator accepts/denies — per [`10-knowledge-builder.md`](10-knowledge-builder.md).
2. **Multimodal auto-extraction for images & documents** (vision-caption images; extract & summarize
   PDF/doc text). **Video is NOT auto-processed** — a video is stored as an asset and the operator
   writes its description (the `describe_media` popup). Auto-transcription of video is deferred.
3. **Full draft → published with a rich publish window.** The operator edits the draft freely before
   publishing: change topic/asset text, add / replace / delete attached files, add more, **assign and
   remove the topic of an asset, and the values** a topic uses. Publish runs the gate and swaps live.

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

## Target architecture

```text
  operator
     │ chat msgs + dropped files (image / video / pdf / doc / text)
     ▼
┌──────────────────────────── PLAYGROUND (the only WRITER) ─────────────────────────────┐
│  Builder chat (LLM agent)            Draft editor (manual, deterministic)              │
│   • normalize material → text          • topic list → body + media gallery + values   │
│   • multimodal extract (img/doc)       • edit text / add·replace·delete files          │
│   • emit tool calls → draft rows       • (re)assign asset.topic, attach/detach values  │
│   • emit popups (requests) when unsure • accept / deny each proposed row               │
└───────────────┬──────────────────────────────────────────────┬────────────────────────┘
                │ writes (review_state, provenance)             │ resolves requests
                ▼                                               ▼
        ┌──────────────────── DRAFT snapshot (ai_* , state='draft') ────────────────────┐
        │  ai_snapshots · ai_topics · ai_assets · ai_values · ai_builder_requests        │
        └───────────────────────────────────┬───────────────────────────────────────────┘
                                             │  [Publish]  → gate (every asset described,
                                             │              every value token resolves)
                                             ▼  atomic swap + brain reload
        ┌─────────────── PUBLISHED snapshot (ai_* , state='published') ──────────────────┐
        └───────────────────────────────────┬───────────────────────────────────────────┘
                                             │ reads only (existing brain, unchanged contract)
                                             ▼
                                    BRAIN → suggests → human approves → customer
```

The brain's read contract (`*domain.Snapshot`) is **unchanged**; only its *source* moves
literal → DB. Everything new is on the write side.

---

## Build layers (the de-risk order)

Follows doc 11's "lock the contract → seed → editor → chat → publish" sequence. Each layer is
shippable and testable on its own.

### L1 — KB contract: make the KB writable & DB-backed  *(unblocks everything)*
- Migration `0003_ai_kb.up.sql`: create `ai_snapshots`, `ai_topics`, `ai_assets`, `ai_values` exactly
  per [`9-database-schema.md`](9-database-schema.md), **plus draft-side columns** (below) and the
  `ai_builder_requests` queue.
- `internal/kbstore/` — a `KBStore` over these tables: load a published `*domain.Snapshot`; CRUD draft
  topics/assets/values; create/resolve requests; publish (swap draft→published atomically + version).
- **Seed migration**: insert `SeedSnapshot()`'s "Demo Shop" content as **version 1, published** so the
  brain keeps answering. `SeedSnapshot()` stays as the code-level fallback when the table is empty.
- **Brain source swap**: `cmd/xchats/main.go` loads the published snapshot from `KBStore` into
  `domain.Content`; reloads it on publish. Fallback to the literal if DB empty/unreachable.
- **Value model bridge**: load `ai_values` rows into the existing `PriceBook` shape at snapshot build
  (`price.*`/`limit.*` → `Tariffs`, everything else → `Placeholders`) so `PriceBook.Render` is
  untouched. (The generalized `ai_values` is the store of record; `PriceBook` is just its in-memory
  projection.)

### L2 — Draft editor (deterministic, no LLM)  *(proves the write contract)*
- Read endpoints for the active draft: config blocks, topic list (each topic → body + its media
  gallery, each asset with description + kind + assigned topic), the value book, the request queue.
- Write endpoints (all draft-only): create/update/delete topic; upload+attach asset; update asset
  description / **reassign `topic_slug`** / delete asset; create/update/delete value; edit
  persona/mission/guardrails/language. Each write stamps `review_state` + `provenance`.
- **Accept / deny** per row (`proposed → approved | rejected`).
- The **Publish gate** (deterministic): every approved asset has a description, every `{{token}}` used
  in an approved topic body resolves in `ai_values`, no dangling media URL, no `pending` blocking
  request → atomic swap to a new published version + brain reload. This layer is a complete, usable KB
  CMS *before* any LLM touches it.

### L3 — Media ingestion + multimodal extraction
- Reuse blob store + `POST /media` for bytes; record `asset_kind` from MIME.
- **Image** → multimodal LLM vision call: "describe what this shows **and when to send it**" → fills
  `ai_assets.description` as a *proposed* value (operator can edit).
- **PDF / doc** → extract text (Go extractor or hand the file/pages to the multimodal LLM) → LLM
  summarizes & **splits into topic candidates** (`body_md` with value tokens) + a per-file description;
  detected numbers become `confirm_price`/`propose_value` requests, never digits in the body.
- **Video** → store the asset only + raise a `describe_media` popup; operator writes the description.
  No transcription in v1 (drops in later behind the same popup — it just pre-fills).
- All extraction output is **proposed** draft rows with provenance ("source: <file/msg>"); nothing is
  silently trusted.
- Config: add a `LLMVisionModel` (multimodal-capable) alongside the existing `LLMFastModel`.

### L4 — Builder chat agent (the LLM that drives L2/L3 from chat)
- A new `internal/playground/builder` agent: input = the operator's chat turn + any attached material +
  the current draft summary; output = **tool calls** that mutate the draft via `KBStore`, plus
  **requests** (popups) when unsure. Tool contract below.
- Rides the existing realtime hub: each created/updated row and each new `pending` request is
  broadcast so the chat **and** the editor update live over the same draft.
- Proactive: confirms prices, proposes merges (`resolve_duplicate`), asks which topic a loose image
  belongs to (`choose_topic`), requests missing descriptions before a row is "done".

### L5 — Publish → brain (wire the last mile)
- The L2 Publish button, now also surfacing unresolved `pending` requests as publish blockers; on
  success swaps live + reloads the brain snapshot; supports rollback (re-publish a prior version).

---

## Data-model deltas (additive to doc 9)

The four KB tables are exactly doc 9's DDL. The playground adds **draft-side** fields + one queue —
all additive; the live runtime tables and the brain are untouched.

```text
ai_topics  / ai_assets / ai_values  gain:
   review_state  text    -- 'proposed' | 'approved' | 'rejected'   (default 'proposed' for agent rows;
                         --  'approved' for manual editor rows)
   provenance    jsonb   -- { "source":"chat_msg|file|url", "ref":"msg-id|filename|url", "at":"…" }

ai_builder_requests  (NEW — the popup/HITL queue, keyed to the draft snapshot)
   id            uuid  PK
   snapshot_id   uuid  FK -> ai_snapshots (the DRAFT)
   req_type      text  -- 'describe_media'|'confirm_price'|'approve_topic'|'approve_asset'
                       --  |'resolve_duplicate'|'choose_topic'|'comment'
   prompt        text  -- what the popup asks
   context       jsonb -- thumbnail ref / detected value / candidate topic — whatever the popup renders
   target        jsonb -- which draft row it resolves into (topic_slug / asset ref / token)
   state         text  -- 'pending' | 'resolved' | 'dismissed'
   resolution    jsonb NULL  -- the operator's answer (becomes the row mutation)
   created_at, resolved_at
```

> **Naming note to settle in L1.** Doc 9 calls the runtime suggestion table `ai_suggestions`; the
> live migration uses `ai_drafts` / `ai_draft_assets`. The playground only touches the **KB** tables
> (`ai_snapshots/topics/assets/values`), so it's unaffected — but record the divergence when writing
> `0003` so reviewers aren't confused.

---

## Builder agent — tool contract

The agent never writes SQL; it calls these (each maps to a `KBStore` mutation or a request). Mirrors
doc 10's request table.

| Tool | Effect |
|---|---|
| `create_topic{slug,lang,title,keywords,body_md}` | upsert a draft `ai_topics` row (`proposed`) |
| `attach_asset{ref,kind,topic_slug,description,blob_id}` | draft `ai_assets` row pointing at uploaded bytes |
| `propose_value{token,lang,value_text,description}` | raises `confirm_price` → on accept, a draft `ai_values` row |
| `describe_media{asset_ref}` | popup asking the operator for an asset's description (esp. **video**) |
| `choose_topic{asset_ref, candidates[]}` | popup — which topic does this asset belong to |
| `resolve_duplicate{kind, a, b}` | popup — merge or keep both (re-fed URL / overlapping topic) |
| `comment{text}` | a note the operator can answer; steers the next build turn |

**Guardrail (same as the runtime brain):** the agent **never bakes a digit into a topic body** — it
writes a `{{token}}` and a `propose_value`/`confirm_price` request; the real number enters `ai_values`
only on human confirmation. Prices are the one thing never auto-written.

---

## Publish gate (deterministic)

Publish is the only draft → published path. It refuses unless, over **approved** rows:
- every `ai_assets` row has a non-empty `description`;
- every `{{namespace.key}}` used in an approved `ai_topics.body_md` resolves in `ai_values` (the
  "price-safety = 1.0 / every token resolves" bar);
- no asset URL dangles (the blob exists);
- no `pending` request remains (an undescribed asset / unconfirmed price would fail above anyway).

On pass: create a new `version`, copy approved rows into a `published` snapshot, atomically swap, bump
the brain's `domain.Content`. Rollback = re-publish a prior version.

---

## API surface (new — under `/xchats/api/v1/playground`)

```
GET    /playground/draft                      → active draft (config + topics[+assets] + values + requests)
POST   /playground/draft/topics               create/update a draft topic
DELETE /playground/draft/topics/:slug
POST   /playground/draft/assets               upload bytes + create asset (multipart: file + meta)
PATCH  /playground/draft/assets/:ref          edit description / reassign topic_slug
DELETE /playground/draft/assets/:ref
POST   /playground/draft/values               create/update a value token
DELETE /playground/draft/values/:token
PATCH  /playground/draft/config               persona / mission / guardrails / language_policy
POST   /playground/draft/rows/:id/review      accept | deny  (review_state)
POST   /playground/chat                       a builder-chat turn (text + attached blob ids) → agent runs
GET    /playground/requests                   the popup queue
POST   /playground/requests/:id/resolve       answer a popup → mutates the draft
POST   /playground/publish                    run gate → swap live → reload brain
```
Realtime (existing SSE hub): `kb.row.changed`, `kb.request.created`, `kb.request.resolved`,
`kb.published` so chat + editor stay in sync for every viewer.

---

## Frontend

A new **Playground** page (route + NavRail entry), two panes over the **same draft**:
- **Builder chat** (left): message list + composer with file-drop (image/video/pdf/doc); inline
  **request cards** (popups) rendered where they occur.
- **Draft editor** (right): config blocks; topic list (expand → `body_md` editor + media gallery, each
  asset showing kind/description and a **topic selector** to reassign, plus delete/replace); the value
  book (token → value, editable); a **review-queue badge**; a **Publish** button that shows gate
  failures inline.

Reuses the existing API envelope, SSE client, blob upload, and auth — no new infra.

---

## Risks & open questions (carry into build)

- **The brain never sees the media** — a bad description = the wrong file sent. Vision-captioning images
  helps, but the operator must still review descriptions (it's the selection cue). Keep editing cheap.
- **Multimodal cost / data-boundary**: images & doc text now go to an external LLM → settle the
  PII/cross-border stance before turning extraction on in production (doc 11 §9).
- **Value-model bridge**: loading `ai_values` into `PriceBook` (Tariffs/Placeholders) is a projection;
  if a value doesn't fit `price.*`/`limit.*`, it must land as a placeholder — verify the demo seed
  round-trips through the bridge unchanged.
- **Concurrency**: agent + human editing one draft at once (doc 11 §6/open-#5) — v1 leans on
  last-write-wins + realtime refresh; revisit if it bites.
- **Prompt size**: media-as-knowledge grows the prompt fastest; if the KB outgrows the prompt, add
  `pgvector` retrieval later behind the same read contract (doc 8 → Scaling). Not now.

## Out of scope (v1)
- Video / audio **auto-transcription** (stored + operator-described instead).
- URL scraping auto-ingest (paste-the-key-points fallback for now).
- `pgvector` retrieval; branchable drafts; git-like changesets (doc 11 §7 add-on).
- The `ai_drafts` → `ai_suggestions` runtime-storage rename (separate concern).
