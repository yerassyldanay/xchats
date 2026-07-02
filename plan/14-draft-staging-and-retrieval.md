# 14 — Draft Staging, Table Independence, Language Scope & Retrieval — Architecture Decision

**Purpose:** capture the architecture agreed after Decision 13: how KB drafts are staged and
approved, how the tables are decoupled so partial approval is always safe, what language scope v1
ships with, and how the response side retrieves knowledge. This is a decision record, not an
implementation plan. Where this record conflicts with an older `plan/*.md` doc, **this record
wins**; affected docs carry a banner and are rewritten lazily (see "Docs affected" at the end).

---

## Problem

Three things needed settling after Decision 13:

- **Partial approval was entangled.** A topic body could embed a fact token
  (`{{tariff.growth.price}}`), so approving a topic without its tariff left a dangling token —
  forcing a dependency-checking gate ("approve that fact row too, or approve-all").
- **Multi-language rows cost effort before any second language exists.** Every fact table was
  designed for per-language rows (ru/kk/`*`) although v1 serves one org in Russian.
- **The response side's retrieval model was undefined.** Decision 13 rejected embeddings outright,
  but the product direction is: user prompt → embedding → similarity retrieval.

## Decision 1 — Draft staging: separate mirror tables (replaces `drafted_at`)

- The per-row **`drafted_at` timestamp lifecycle is removed** from the design.
- Draft rows live in **dedicated draft tables**, one **draft twin per KB table**, with the **same
  columns** as the live table. Same structure ⇒ the UI displays draft and live rows the same way.
- **Live tables are never edited directly** — not even a one-character fix. The playground writes
  drafts only; **approve is the only write path to live**. One writer path, always.
- The answering side reads **live tables only**; it physically cannot see a draft.

## Decision 2 — Approve = gate → copy → embed

- Approving (the whole draft or selected rows) runs the deterministic gate, then **copies** the
  approved rows into the live tables (upsert on the row's natural key, e.g. `(ref, lang)`).
- **Deletion** of a live row = a **delete-marker** on the draft row; applied at approve.
- **Embeddings are refreshed in the same approve step** for the affected topics, so the embedding
  index only ever contains approved content. (Embedding details deferred — see Decision 5 scope.)
- Rejecting a draft row is a plain delete of the draft row; live is untouched.

## Decision 3 — Fully independent tables: no fact tokens in topic bodies

- A topic `body_md` is **pure prose** — **no numbers and no `{{table.slug.field}}` tokens**.
  ("We have 4 plans, from starter to enterprise" — never "the Growth plan costs {{…}}".)
- Fact tokens appear **only in model replies**, emitted from the facts catalog in the prompt and
  resolved at reply time from the **live** fact tables — fail closed, exactly as in Decision 13.
- Consequence: **every table is independent**; approving a topic, a tariff, a product, or a contact
  row in any order can never dangle. The approve gate needs **no dependency check** and no
  "approve that fact row first" messaging.
- Media ownership (`ai_assets.owner_kind/owner_ref`) stays polymorphic and is exempt from the
  independence rule: an orphaned asset is clutter, not a wrong price.

## Decision 4 — v1 is single-language (Russian); language stays a row

- v1 stores the KB **in Russian only**. The **`lang` column stays** on every per-language table;
  only `ru` rows are filled. Adding Kazakh (or any language) later = **inserting rows**, no schema
  change and no rework — the whole point of "language is a row".
- **Prose:** the model may answer in the customer's language by translating Russian topics on the
  fly; the safety pipeline is language-agnostic.
- **Facts:** substituted **verbatim**, so a non-Russian reply may contain Russian units
  ("айына 25 000 ₸/мес") — **accepted for v1**. Neither code nor the model may reformat or
  translate a fact value (that is the `formatTenge` lossy bridge Decision 13 killed).
- Side effect: with one language, the facts catalog in the prompt is single-valued and
  **cache-stable** — the `[F]` block can sit in the cached prefix without per-message rebuilds.

## Decision 5 — Response side: embeddings retrieve the Knowledge lane only

- On a user prompt: convert the prompt to an embedding → **similarity search over live topics
  only** (pgvector, in the same Postgres) → the matched topics go into the model's prompt.
- **Facts are never retrieved by similarity.** The fact tables are tiny; they are included
  exactly / looked up by key. (A nearest-neighbor hit can return the *wrong* tariff — Decision 13's
  objection stands, now scoped to facts.)
- Retrieval **selects prompt context**; everything downstream — token emission, template render,
  number check, human review — is unchanged.
- The read path **never writes** the KB. Playground writes; prompts read. Two separate blocks.
- This **amends Decision 13 / Decision 8**: embeddings are no longer rejected outright — they are
  allowed **for the Knowledge lane only**. Embedding model choice, chunking, and index details are
  **deferred** — to be decided in a later record.

## Decision 6 — v1 safety = templates + number check + human review; judge deferred

- v1 ships pipeline steps: escalate gate → **template render** (facts) → **number check** →
  media validation → **human review**. These cover every fabricated-number failure at ~zero cost.
- The **prose grounding judge** (Decision 13, pipeline step 4) is **deferred from v1**. Human
  review covers unsupported prose until then. The judge becomes **mandatory before any auto-send**
  (Phase 4D) and is recommended as soon as reviewer load makes rubber-stamping likely.
- This amends **Decision 13 / Decision 6**: step 4 of the pipeline is marked deferred, not removed.

## Explicitly rejected / out of scope

- **`drafted_at` / one-living-KB row flags** — replaced by mirror draft tables (Decision 1).
- **Fact tokens or literal numbers in topic bodies** — topics are prose only (Decision 3).
- **Per-language columns** (`name_ru`, …) — still rejected; language remains a row (Decision 4).
- **Similarity search over facts** — still rejected; exact lookup only (Decision 5).
- **Auto-send** — still rejected; human review stays the final gate.
- **Embedding implementation details** (model, chunking, refresh strategy) — deferred.

---

## Docs affected (bannered now, rewritten lazily)

| Doc | What this record changes there |
|---|---|
| `9-database-schema.md` | `drafted_at` columns → draft twin tables; topic bodies carry no tokens; ru-only v1 |
| `11-ai-design-overview.md` | §4/§5 (tokens in bodies), §6 (no-vector-search), §11 (judge timing) amended |
| `12-playground-build.md` | L1 data model, approve gate, and API `drafted_at` semantics superseded |
| `5-ui-pages.md` | «Черновик» badge = "row exists in draft tables", not `drafted_at` |
| `7.1-endpoints.md` | playground draft/approve endpoints operate on draft tables |
| `8-ai-assistant.md` | judge step deferred; `[F]` single-language (cache-stable); topics token-free; retrieval per Decision 5 (its former subdocs `8.1`–`8.7` were merged into it; `10-knowledge-builder.md` and root `todo.md` were retired in the same consolidation) |
| `13-kb-facts-and-grounding.md` | **amended** (not superseded): Decision 6 step 4 deferred; Decision 8 embeddings scope; tokens out of topic bodies |

**Convention (first applied here):** a new architecture decision lands as one short numbered record
like this + one-line banners on affected docs. Full doc rewrites happen only when a doc is next
touched for its own sake. See `0-overview.md` → "How we write these plans".
