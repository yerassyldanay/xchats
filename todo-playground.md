# TODO — Playground (chat → builds the Knowledge Base)

Build the **Playground**: a chat where an operator drops material (images, video, PDF, docs, text),
the assistant structures it into the KB, and a human reviews & **publishes**. Design:
[`plan/12-playground-build.md`](plan/12-playground-build.md) (concept: `plan/10`, `plan/11`).

## Locked scope (this round)
- **Full chat-agent build** (LLM structures material + popups + accept/deny), not editor-only.
- **Multimodal auto-extraction for images & documents** (vision-caption images; extract+summarize
  PDF/doc). **Video = stored + operator-described** (no transcription in v1).
- **Full draft → published** with a rich publish window: edit text, add/replace/delete files, reassign
  an asset's topic, attach/detach values, accept/deny, then Publish (gated).

## The crux
The KB is a Go literal today ([`backend/internal/brain/seed.go`](backend/internal/brain/seed.go)); the
`ai_snapshots/topics/assets/values` tables are designed (`plan/9`) but **not migrated**. The playground
can't write a literal → **L1 lands the writable DB KB first**, brain switches literal → published
snapshot (literal kept as seed + fallback).

---

## L1 — KB contract: writable, DB-backed KB  *(unblocks everything)*
- [ ] Migration `0003_ai_kb.up/down.sql`: `ai_snapshots`, `ai_topics`, `ai_assets`, `ai_values` per
  `plan/9`, **plus** draft fields `review_state` + `provenance` and the `ai_builder_requests` queue.
- [ ] Seed migration: insert `SeedSnapshot()` "Demo Shop" content as **version 1, published** (brain
  keeps answering). Keep `SeedSnapshot()` as code fallback when the table is empty.
- [ ] `internal/kbstore/` — `KBStore`: `LoadPublished() (*domain.Snapshot, error)`; draft CRUD for
  topics/assets/values; request create/resolve; `Publish()` (atomic draft→published swap + version).
- [ ] **Value bridge**: build `domain.PriceBook` from `ai_values` (`price.*`/`limit.*`→Tariffs, else
  Placeholders) so `PriceBook.Render` is untouched; verify the demo seed round-trips unchanged.
- [ ] `cmd/xchats/main.go`: load published snapshot from `KBStore` into `domain.Content`; reload on
  publish; fallback to the literal if DB empty/unreachable.
- [ ] Note the `ai_drafts` (live) vs `ai_suggestions` (doc 9) naming divergence in the migration header.
- [ ] `go build ./... && go vet ./...`; `kbstore` round-trip test (seed → load → snapshot equals literal).

## L2 — Draft editor (deterministic, no LLM)  *(proves the write contract)*
- [ ] Read: `GET /playground/draft` → config + topics(+assets) + values + request queue.
- [ ] Write (draft-only, each stamps `review_state`+`provenance`): create/update/delete topic;
  upload+attach asset; PATCH asset (description / **reassign topic_slug**); delete asset; CRUD value;
  PATCH config (persona/mission/guardrails/language).
- [ ] Accept / deny per row (`proposed → approved | rejected`).
- [ ] **Publish gate** (deterministic): every approved asset described; every `{{token}}` in an approved
  body resolves in `ai_values`; no dangling blob; no `pending` request → atomic swap + brain reload.
- [ ] Tests: gate rejects undescribed asset / unresolved token / dangling blob; happy-path publish bumps
  version + reloads brain.

## L3 — Media ingestion + multimodal extraction
- [ ] `POST /playground/draft/assets` (multipart): bytes → blob, `asset_kind` from MIME, `proposed` row.
- [ ] Config: add `LLMVisionModel` (multimodal) next to `LLMFastModel`; document in `.env.example` /
  `config.example.yaml`.
- [ ] **Image** → vision call "what it shows + when to send it" → proposed `description` (editable).
- [ ] **PDF/doc** → extract text → LLM summarize + split into topic candidates (tokens, not digits) +
  per-file description; detected numbers → `confirm_price`/`propose_value` requests.
- [ ] **Video** → store asset + raise `describe_media` popup (operator writes description). No transcribe.
- [ ] All output is `proposed` with provenance; extraction unit tests with a **fake** multimodal client.

## L4 — Builder chat agent
- [ ] `internal/playground/builder`: input = chat turn + attached blob ids + draft summary; output =
  tool calls (`create_topic`, `attach_asset`, `propose_value`, `describe_media`, `choose_topic`,
  `resolve_duplicate`, `comment`) → `KBStore` mutations + `ai_builder_requests`.
- [ ] Guardrail: never bake a digit into a body — write `{{token}}` + a `propose_value` request.
- [ ] `POST /playground/chat` runs the agent; `GET /playground/requests` + `POST /requests/:id/resolve`.
- [ ] Realtime: broadcast `kb.row.changed` / `kb.request.created` / `kb.request.resolved` so chat +
  editor stay in sync for every viewer.
- [ ] Parity tests with a **fake** LLM: a price in material → token + confirm popup (no digit in body);
  a loose image → `choose_topic`; a re-fed topic → `resolve_duplicate`.

## L5 — Publish → brain
- [ ] Publish surfaces unresolved `pending` requests as blockers; on success swap live + reload brain;
  rollback = re-publish a prior version. Broadcast `kb.published`.

## Frontend
- [ ] New **Playground** route + NavRail entry; two panes over the same draft.
- [ ] **Builder chat** pane: messages + composer with file-drop (image/video/pdf/doc); inline request
  (popup) cards.
- [ ] **Draft editor** pane: config blocks; topic list (expand → body editor + media gallery, each asset
  with kind/description + **topic selector** + delete/replace); value book (editable); review-queue
  badge; **Publish** button showing gate failures inline.
- [ ] Reuse the existing API envelope, SSE client, blob upload, auth.

## Manual e2e (with a real LLM key)
- [ ] Drop a pricing image → vision description appears as a proposed asset under a topic.
- [ ] Drop a PDF catalog → topic candidates + a `confirm_price` popup; confirm → value token resolves.
- [ ] Drop a video → `describe_media` popup; write a description.
- [ ] Edit a topic body, reassign an asset's topic, deny one row, then **Publish** → gate passes,
  brain reloads, a customer "Сколько стоит?" now answers from the new published KB.
- [ ] Publish with an undescribed asset / unconfirmed price → gate blocks with a clear reason.

## Out of scope (v1)
- Video/audio auto-transcription; URL scraping auto-ingest; `pgvector` retrieval; branchable drafts /
  changesets; the `ai_drafts`→`ai_suggestions` rename.
