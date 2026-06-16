# TODO — Playground (chat → builds/updates the Knowledge Base)

> **Build status (backend landed).** L1–L4 are implemented and tested against a
> real Postgres (`make test-e2e`):
> - **L1** ✅ generic `ai_values` model (lossy `PriceBook` removed), migration
>   `0003_ai_kb`, `internal/kbstore` (load/seed/draft/publish-gate/rollback),
>   brain source-swap (DB snapshot + literal fallback, hot-reload on publish).
> - **L2** ✅ draft editor API + deterministic publish gate (`/playground/*`).
> - **L3** ✅ ingest adapters (text/url/media) on the queue; multimodal
>   `llm.Vision` client behind `LLM_VISION_MODEL` (nil → describe popups).
> - **L4** ✅ `internal/playground` builder with a deterministic `RuleSynthesizer`
>   (tokenizes prices → confirm popups, one topic per batch, update-vs-create diff).
>   An **LLM synthesizer** behind the same `Synthesizer` interface is the next step.
> - **Remaining:** the **frontend** Playground page; the **LLM synthesizer** (real
>   cross-material judgment); the live multimodal/golden-set evals. See "Frontend"
>   and the manual-e2e section below.


Build the **Playground**: a chat where an operator drops **any mix of material** (text, URLs, images,
PDFs, docs, video), the assistant **extracts** it, **builds or updates** the KB, and a human reviews &
**publishes**. Design: [`plan/12-playground-build.md`](plan/12-playground-build.md) (concept: `plan/10`,
`plan/11`).

## Locked scope (this round)
- **Full chat-agent build** (LLM synthesizes material + popups + accept/deny). L2's editor is built
  first as the deterministic proof of the write contract.
- **Normalize any input → one common form, then synthesize.** Per-type ingest adapters → one
  `NormalizedMaterial`; one type-agnostic synthesis pass. Extraction depth is phased:
  - **Images & docs** auto-extract (vision / text).
  - **URLs** best-effort fetch, with a **paste/screenshot** fallback (URL = just one adapter).
  - **Video/audio** stored + operator-described (no transcription in v1).
- **Build *or* update over a draft cloned from published** — synthesis diffs candidates against the
  existing KB, so "update" is real, not blind append.
- **Full draft → published** with a rich publish window: edit text, add/replace/delete files, reassign
  an asset's topic, edit values, accept/deny, then Publish (gated).

## The crux
The KB is a Go literal today ([`backend/internal/brain/seed.go`](backend/internal/brain/seed.go)); the
`ai_snapshots/topics/assets/values` tables are designed (`plan/9`) but **not migrated**. A playground
can't write a literal → **L1 lands the writable DB KB + draft lifecycle first**; the brain switches
literal → published snapshot (literal kept as seed + fallback). The other crux is **uniform ingestion**:
every input type is normalized to `ai_materials` before any KB reasoning (L3), so the synthesis agent
(L4) deals with one shape, not N.

---

## L1 — KB contract: writable DB KB + draft lifecycle  *(unblocks everything)*
- [ ] Migration `0003_ai_kb.up/down.sql`: `ai_snapshots`, `ai_topics`, `ai_assets`, `ai_values` per
  `plan/9`, **plus** draft fields `review_state` + `provenance`, the `ai_builder_requests` queue, the
  `ai_materials` staging table, and the one-draft guard `UNIQUE (organization_id) WHERE snapshot_state='draft'`.
- [ ] **Generic value model (fix the lossy bridge):** `ai_values` = `(token, lang) → value_text`; render
  by **pure `{{token}}` → `value_text`** substitution (reply-lang, then `'*'` fallback; unresolved →
  escalate). Replace `PriceBook`'s typed `Tariff`/`formatTenge` path (it corrupts unit-bearing values,
  e.g. `"25 000 ₸/мес"` → `"25 000 ₸"`). Keep the `*domain.Snapshot` shape; change its `Values` carrier
  + `Render` internals. Re-express the Demo Shop seed as `ai_values` rows.
- [ ] Seed migration: insert seed content as **version 1, published** (brain keeps answering); keep
  `SeedSnapshot()` as code fallback when the table is empty/unreachable.
- [ ] `internal/kbstore/` — `KBStore`: `LoadPublished(orgID)`; `OpenDraft(orgID)` (clone published →
  single draft); draft CRUD topics/assets/values; material create + extraction-update; request
  create/resolve; `Publish(orgID)` (atomic draft→published + version); `Rollback(orgID, version)`;
  `DiscardDraft(orgID)`.
- [ ] `cmd/xchats/main.go`: load published snapshot from `KBStore` into `domain.Content`; reload on
  publish; fallback to the literal if DB empty/unreachable. v1 = single seeded org.
- [ ] Note the `ai_drafts` (live) vs `ai_suggestions` (doc 9) naming divergence in the migration header.
- [ ] `go build ./... && go vet ./...`; `kbstore` round-trip test (seed → load → renders identically,
  **including unit-bearing values**).

## L2 — Draft editor (deterministic, no LLM)  *(proves the write contract)*
- [ ] Read: `GET /playground/draft` → config + topics(+assets) + values + requests + materials.
- [ ] Draft lifecycle: `POST /playground/draft` (open = clone published, idempotent) · `DELETE` (discard).
- [ ] Write (draft-only, each stamps `review_state`+`provenance`): upsert/delete topic; upload+attach
  asset; PATCH asset (description / **reassign topic_slug**); delete asset; upsert/delete value; PATCH
  config. Manual rows default `approved`; agent rows `proposed`.
- [ ] Accept / deny per row (`proposed → approved | rejected`).
- [ ] **Optimistic concurrency:** writes carry `updated_at`/version; stale write → `409`.
- [ ] **Publish gate** (deterministic): every approved asset described; every `{{token}}` in an approved
  body resolves; no dangling blob; no `pending` request → atomic swap + brain reload.
- [ ] Tests: gate rejects undescribed asset / unresolved token / dangling blob; happy-path publish bumps
  version + reloads brain.

## L3 — Ingest adapters (Stage 1: normalize any input → `ai_materials`)
- [ ] `POST /playground/draft/materials`: create an `ai_materials` row per input (`pending`), store bytes
  to blob (`media_kind` from MIME), enqueue an **extraction job** per material on the in-memory queue.
- [ ] Adapters (fill `extracted_text` or flag `needs_human` → `describe_media` popup):
  - [ ] **text** → pass through.
  - [ ] **url** → best-effort fetch → readable text; on 403/JS-empty → paste/screenshot popup.
  - [ ] **image** → vision call "what it shows + when to send it" → proposed description.
  - [ ] **pdf/doc** → extract text (Go extractor or vision-OCR pages); numbers → later `confirm_price`.
  - [ ] **video/audio** → store bytes only + `describe_media` popup. No transcription.
- [ ] Config: add `LLMVisionModel` (multimodal) next to `LLMFastModel`; document in `.env.example` /
  `config.example.yaml`.
- [ ] Extraction output lands on the material (text + status + provenance); unit tests with a **fake**
  multimodal client (per adapter incl. the needs_human fallback).

## L4 — Builder chat agent (Stage 2: synthesis over the common form)
- [ ] `internal/playground/builder`: input = chat turn + **ready materials** + a **draft summary**
  (capped); output = tool calls (`create_topic`/`propose_value` **upsert**, `attach_asset`,
  `describe_media`, `choose_topic`, `resolve_duplicate`, `comment`) → `KBStore` + `ai_builder_requests`.
- [ ] **Synthesis, not per-file captioning:** cross-reference materials → coherent topics/assets; **diff
  vs the draft** → update existing vs create; tokenize numbers. Ambiguity → `resolve_duplicate`/`choose_topic`.
- [ ] **Turn budget:** bounded tool loop (cap N calls; stop when no unprocessed material) — no open loop.
- [ ] Guardrail: never bake a digit into a body — write `{{token}}` + a `propose_value`/`confirm_price`.
- [ ] `POST /playground/chat` runs the synthesis pass; `GET /playground/requests` +
  `POST /requests/:id/resolve`.
- [ ] Realtime: broadcast `kb.material.updated` / `kb.row.changed` / `kb.request.created|resolved`.
- [ ] Tests: **parity** (price → token + confirm popup, no digit; loose image → `choose_topic`; re-fed
  topic → `resolve_duplicate`) **and** a small **golden set** (`material → expected structure`, judged
  loosely) so the agent's *judgment* is measured, not just its plumbing.

## L5 — Publish → brain
- [ ] Publish surfaces unresolved `pending` requests as blockers; on success swap live + reload brain;
  rollback = re-publish a prior version. Broadcast `kb.published`.

## Frontend
- [ ] New **Playground** route + NavRail entry; two panes over the same draft.
- [ ] **Builder chat** pane: messages + composer with **file-drop + URL paste**; each material shows an
  **extraction-status chip** (extracting → ready / needs-you); inline request (popup) cards.
- [ ] **Draft editor** pane: config blocks; topic list (expand → body editor + media gallery, each asset
  with kind/description + **topic selector** + delete/replace); value book (editable); review-queue
  badge; **Publish** button showing gate failures inline.
- [ ] Reuse the existing API envelope, SSE client, blob upload, auth.

## Manual e2e (with a real LLM key)
- [ ] **The flagship flow:** drop a **pricing URL + several tariff images** with "here are my tariffs,
  build the KB" → materials extract → **one** `pricing` topic + asset cards proposed, prices as tokens +
  `confirm_price` popups. Confirm → tokens resolve.
- [ ] Drop a URL that 403s / is JS-only → paste/screenshot fallback popup; screenshot routes through the
  image adapter.
- [ ] Drop a PDF catalog → topic candidates + a `confirm_price` popup.
- [ ] Drop a video → `describe_media` popup; write a description.
- [ ] Re-drop overlapping material → `resolve_duplicate` (update, not a twin); the existing topic updates.
- [ ] Edit a topic body, reassign an asset's topic, deny one row, then **Publish** → gate passes, brain
  reloads, a customer "Сколько стоит?" answers from the new published KB.
- [ ] Publish with an undescribed asset / unconfirmed value → gate blocks with a clear reason.

## Out of scope (v1)
- Video/audio auto-transcription; headless-browser URL rendering (best-effort + paste/screenshot for
  now); `pgvector` retrieval; branchable drafts / changesets; the `ai_drafts`→`ai_suggestions` rename.
