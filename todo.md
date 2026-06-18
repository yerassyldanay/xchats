# Architecture: `drafted_at` KB model + thin entities with polymorphic values & media

## Context

**Why this change.** Three needs converged:

1. **A simpler draft/approval model.** Replace versioned *snapshots* + per-row
   `review_state` with a single living KB per org where each row is flagged pending via a
   `drafted_at` timestamp and held out of the prompt until a human approves it.
2. **Structured catalog data** for many spheres: add **`ai_products`** (a sellable item)
   and **`ai_tariffs`** (a pricing *plan* — fixed/percentage, limits, pros/cons). The two
   are independent (no links).
3. **Media + numbers on any entity**, without fattening the entity tables or coupling
   reply templates to column names.

**The central design principle (Principle 0 below):** entity tables stay *thin and
descriptive*. Every embeddable number lives in `ai_values`, every media file lives in
`ai_assets`, and both attach back to their entity through one shared `(owner_kind,
owner_ref)` pair. Templates embed only `{{namespace.key}}` from `ai_values` — they never
reference a table column. This is what keeps the schema small and the embedding uniform.

**Decisions (with the user).**
- Draft = a per-row `drafted_at` timestamp; approval is per-element or all-at-once.
- `ai_products` and `ai_tariffs` are independent (no FKs between them).
- Numbers never become entity columns → no column-name templating.
- `limits` is jsonb; `advantages`/`disadvantages` are text columns; the rest of a tariff
  is a small descriptive spine + `data` jsonb.
- Accepted trade-off: no versioned history / rollback.

This round delivers the **architecture doc only**; migration + Go/Vue code follow.

---

## Principle 0 — Thin entities; values & media attach polymorphically; templates use tokens only

**The embedding question, answered:** there is exactly **one** way to put a value into
text — `{{namespace.key}}`, resolved from `ai_values` by `ValueBook.Render`
([content.go](backend/internal/brain/domain/content.go), regex `{{ns.key}}`). We do **not**
template column names. Therefore embeddable numbers must not live on entity tables.

```
ai_products  ref='nike-x'  name='Nike X'  category='Обувь'  data={size:'40-45'}   ← descriptive only
ai_values    token='price.nike_x'  value_text='25 000 ₸'  owner_kind='product' owner_ref='nike-x'
ai_assets    ref='img-42'  description='...'  owner_kind='product' owner_ref='nike-x'
```

- A reply template says `{{price.nike_x}}` → renders `25 000 ₸` verbatim. The renderer is
  unchanged and never sees the product table.
- An entity's numbers = `SELECT … FROM ai_values WHERE owner_kind=$1 AND owner_ref=$2`.
- An entity's media = the same query on `ai_assets`.
- `owner_kind ∈ {'topic','product','tariff',''}`; `''` = a global scalar (e.g.
  `contact.whatsapp`) or an unattached asset.

This single move (a) removes every numeric column from products/tariffs — fixing "too
many columns", (b) keeps one embedding mechanism, (c) reuses the polymorphic pattern for
both media and values, and (d) preserves the hallucination-safety the token system exists
for.

---

## Principle 1 — Draft state is a per-row `drafted_at`; the prompt reads only live rows

| `drafted_at` | Meaning | In prompt? | In `/playground`? |
|---|---|---|---|
| `NOT NULL` | pending (new or edited) | **no** | yes — edit + approve |
| `NULL` | approved / live | **yes** | yes |

- **Brain read** ([kbstore.go](backend/internal/kbstore/kbstore.go), `LoadPublished` →
  `LoadLive`): every content query gains `AND drafted_at IS NULL`.
- **Playground read** ([draft.go](backend/internal/kbstore/draft.go) `GetDraft`): all rows;
  `drafted_at` drives the "pending" badge.
- **Approve** = `UPDATE … SET drafted_at = NULL` for one row or all draft rows.
- **Add-on-top**: new media / values insert rows with `drafted_at = now()`.
- **Editing a live row** re-marks it `drafted_at = now()` (drops from prompt until
  re-approved; previous value overwritten — accepted trade-off).

**Schema delta from `0003`:** `ai_snapshots` → one living KB row per org (drop `version`,
`snapshot_state`, `published_at`; keep config + FK parent); all content tables **drop
`review_state`, add `drafted_at`**; `ai_materials` / `ai_builder_requests` unchanged.

---

## Principle 2 — Approval is gated for consistency

The deterministic gate ([kbstore.go](backend/internal/kbstore/kbstore.go) `gate`) **moves
from publish to approval**:

- **Approve all** → gate-check the resulting live set (every `{{token}}` in a live body
  resolves; every owned media blob exists; no literal currency in a topic body).
- **Approve one element** → block with a precise reason if it would leave a dangling token
  ("approve `price.nike_x` first, or use Approve all").

Reuses `ValueBook.Render` exactly as today. Token uniqueness across all `ai_values` is
enforced here.

---

## Proposed schema — migration `0004` (design)

Thin entities + two extended shared tables.

```sql
-- A sellable item: descriptive only. Its price/numbers are owned ai_values rows.
CREATE TABLE xchats.ai_products (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    kb_id       uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    ref         text NOT NULL,                       -- stable key 'nike-x'
    name        text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    category    text NOT NULL DEFAULT '',
    data        jsonb NOT NULL DEFAULT '{}'::jsonb,   -- sphere-specific attrs (size, color…)
    lang        text NOT NULL DEFAULT 'ru',
    status      text NOT NULL DEFAULT 'active',
    drafted_at  timestamptz,
    provenance  jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kb_id, ref)
);

-- A pricing PLAN / tariff: small descriptive spine. Its numbers are owned ai_values rows.
CREATE TABLE xchats.ai_tariffs (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    kb_id         uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    ref           text NOT NULL,                      -- 'standard' | 'pro'
    name          text NOT NULL DEFAULT '',
    summary       text NOT NULL DEFAULT '',
    pricing_type  text NOT NULL DEFAULT 'fixed',      -- 'fixed'|'percentage'|'tiered'|'hybrid'
    advantages    text NOT NULL DEFAULT '',           -- text input
    disadvantages text NOT NULL DEFAULT '',           -- text input
    limits        jsonb NOT NULL DEFAULT '{}'::jsonb, -- context ranges {monthly_cap, per_tx_max}
    data          jsonb NOT NULL DEFAULT '{}'::jsonb, -- description, conditions, billing_period…
    lang          text NOT NULL DEFAULT 'ru',
    status        text NOT NULL DEFAULT 'active',
    drafted_at    timestamptz,
    provenance    jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kb_id, ref)
);

-- Shared tables gain the polymorphic owner pair (ai_assets also drops topic_slug):
ALTER ai_values  ADD owner_kind text NOT NULL DEFAULT '',  ADD owner_ref text NOT NULL DEFAULT '',
                 DROP review_state, ADD drafted_at timestamptz;
ALTER ai_assets  DROP topic_slug,
                 ADD owner_kind text NOT NULL DEFAULT '',  ADD owner_ref text NOT NULL DEFAULT '',
                 DROP review_state, ADD drafted_at timestamptz;
ALTER ai_topics  DROP review_state, ADD drafted_at timestamptz;
```

Notes:
- A tariff's rate / fee / caps that the assistant must quote exactly → owned `ai_values`
  (`owner_kind='tariff'`); purely illustrative ranges can stay in `limits` jsonb as
  context the model paraphrases. `tiers` (for tiered plans) live in `data` jsonb.
- The exact descriptive-column set on tariffs is tunable (could promote
  `description`/`conditions`/`billing_period` out of `data` into columns) — the embedding
  design doesn't depend on it.

---

## Integration touchpoints (spec for the follow-up implementation round)

1. **Domain** — [content.go](backend/internal/brain/domain/content.go): add `Product`,
   `Tariff`, `snap.Products`, `snap.Tariffs`; `Value` and `Asset` gain `OwnerKind/OwnerRef`;
   all owned values still flow into the one `ValueBook`.
2. **Load** — [kbstore.go](backend/internal/kbstore/kbstore.go) `loadSnapshotContent`:
   filter `drafted_at IS NULL`; add product/tariff scans; values/assets scans read owners.
3. **System prompt** — [prompt.go](backend/internal/brain/prompt.go) `BuildSystem`: add
   `PRODUCT CATALOG:` + `TARIFFS:` sections that list each entity with its owned **value
   tokens** (so the model emits `{{price.nike_x}}`) and owned media refs; `MEDIA CATALOG:`
   line becomes `ref | kind | owner | description`.
4. **Approval gate** — relocate `gate` to approve-all / approve-element (Principle 2).
5. **Draft store + API** — [draft.go](backend/internal/kbstore/draft.go),
   [playground.go](backend/internal/httpapi/playground.go),
   [server.go](backend/internal/httpapi/server.go): `GetDraft` returns all rows incl.
   `drafted_at`; CRUD for products & tariffs mirroring `UpsertValue`; value/asset upserts
   accept `owner_kind`/`owner_ref`; approve routes (`POST /playground/approve`,
   `…/approve/:kind/:id`).
6. **Frontend** — [KnowledgeBase.vue](frontend/src/views/KnowledgeBase.vue),
   [types.ts](frontend/src/types.ts): new **"Товары"** / **"Тарифы"** tabs; each entity
   editor shows its **owned values** (the embeddable tokens) and **owned media** grouped
   under it, plus an "add media / add value" affordance; "pending" badges + per-row/bulk
   **Approve** driven by `drafted_at`.
7. **Synthesis (future)** —
   [playground/builder.go](backend/internal/playground/builder.go) emits product/tariff
   rows + owned values from materials, created with `drafted_at` set for approval.

---

## Deliverable for this round

Architecture doc only. Create **`docs/products-tariffs-and-media.md`** (new `docs/` dir at
the xchats root, matching the `docs/NN` convention the brain comments reference). It
contains Principles 0–2, the `0004` DDL + schema delta, and the integration checklist.
**No migration is run, no Go/Vue code changes this round.**

## Verification (doc round)

- Confirm the `0004` DDL is consistent with
  [0003_ai_kb.up.sql](backend/migrations/0003_ai_kb.up.sql) (FK + cascade, `provenance`,
  `(kb_id, ref)` keys, polymorphic `owner_kind/owner_ref` on values + assets).
- Confirm the embedding path is column-free: a `{{price.nike_x}}` token resolves through
  `ai_values` only; no entity column is referenced anywhere in template text.
- Walk one end-to-end example: create product `nike-x` (thin) + value `price.nike_x` +
  photo `img-42`, all `owner_ref='nike-x'`, as drafts → playground shows them grouped &
  pending → approve-all (gate passes) → a reply with `{{price.nike_x}}` renders `25 000 ₸`.

## Open sub-decisions (in the doc, not blockers)

- Media/value reuse across entities (single owner now vs a join table later).
- Which tariff descriptors stay in `data` jsonb vs get promoted to columns.
- Config drafting shape (`pending jsonb` overlay vs shadow row).
- Whether `inactive` products/tariffs are dropped from the prompt entirely (lean yes).
