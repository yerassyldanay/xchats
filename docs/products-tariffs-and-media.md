# `drafted_at` KB model + thin entities with polymorphic values & media

> **Status:** architecture / design doc. **No migration is run and no Go/Vue code changes
> in this round** — this doc is the contract the follow-up implementation round executes.
> It establishes the `docs/` directory the brain's `docs/NN` comments already reference
> (the brain was ported from `xpayment-crm`, whose doc folder this mirrors). Companion
> design docs live under [`plan/`](../plan): the data model in
> [`plan/9-database-schema.md`](../plan/9-database-schema.md), the playground lifecycle in
> [`plan/12-playground-build.md`](../plan/12-playground-build.md).

## Context

Three needs converged into one change.

1. **A simpler draft / approval model.** Replace the versioned **snapshot** lifecycle
   (`version`, `snapshot_state`, publish/rollback) + per-row `review_state` with a single
   **living KB per org**, where each row is flagged pending via a `drafted_at` timestamp and
   held out of the prompt until a human approves it.
2. **Structured catalog data** for many spheres: add **`ai_products`** (a sellable item) and
   **`ai_tariffs`** (a pricing *plan* — fixed / percentage, limits, pros / cons). The two are
   **independent** (no FKs between them).
3. **Media + numbers on any entity**, without fattening the entity tables or coupling reply
   templates to column names.

### Decisions (settled with the user)

- **Draft = a per-row `drafted_at` timestamp.** Approval is per-element or all-at-once.
- **`ai_products` and `ai_tariffs` are independent** — no foreign keys between them.
- **Numbers never become entity columns** → there is no column-name templating.
- A tariff's `limits` is `jsonb`; `advantages` / `disadvantages` are plain `text` columns; the
  rest of a tariff is a small descriptive spine + a `data` `jsonb` bag.
- **Accepted trade-off: no versioned history and no rollback.** Editing a live row overwrites
  the previous value (it re-enters the draft state until re-approved). There is no snapshot to
  roll back to.

---

## Principle 0 — Thin entities; values & media attach polymorphically; templates use tokens only

**The embedding question, answered.** There is exactly **one** way to put a value into reply
text: the token `{{namespace.key}}`, resolved from `ai_values` by `ValueBook.Render`
([`backend/internal/brain/domain/content.go`](../backend/internal/brain/domain/content.go)).
We do **not** template column names. Therefore embeddable numbers **must not** live on the
entity tables.

```
ai_products  ref='nike-x'  name='Nike X'  category='Обувь'  data={size:'40-45'}   ← descriptive only
ai_values    token='price.nike_x'  value_text='25 000 ₸'  owner_kind='product' owner_ref='nike-x'
ai_assets    ref='img-42'  description='…'                 owner_kind='product' owner_ref='nike-x'
```

- A reply template says `{{price.nike_x}}` → renders `25 000 ₸` **verbatim**. The renderer is
  unchanged and never sees the product table.
- An entity's **numbers** = `SELECT … FROM ai_values WHERE owner_kind = $1 AND owner_ref = $2`.
- An entity's **media** = the same query against `ai_assets`.
- `owner_kind ∈ {'topic', 'product', 'tariff', ''}`; `''` = a **global** scalar (e.g.
  `contact.whatsapp`) or an **unattached** asset.

This one move:

- **(a)** removes every numeric column from products / tariffs — fixing "too many columns";
- **(b)** keeps **one** embedding mechanism (the existing token renderer);
- **(c)** reuses the same polymorphic `(owner_kind, owner_ref)` pattern for **both** media and
  values; and
- **(d)** preserves the hallucination-safety the token system exists for — a number the
  assistant quotes is always a confirmed `ai_values` row, never free text.

### Token grammar is independent of the owner ref

`tokenRE` in `content.go` is `\{\{\s*([a-zA-Z_]+)\.([a-zA-Z0-9_]+)\s*\}\}`. So a token's
**key** segment matches `[a-zA-Z0-9_]+` — **underscores, not hyphens**. The product's
`owner_ref` (`nike-x`) is a *free-form stable key* and may contain hyphens; the **token**
that embeds its price (`price.nike_x`) must use a key the regex accepts. They are linked by
the `owner_ref` column, **not** by string equality — `{{price.nike-x}}` would never render.
Convention: derive the token key from the ref by replacing `-`/spaces with `_`.

---

## Principle 1 — Draft state is a per-row `drafted_at`; the prompt reads only live rows

| `drafted_at` | Meaning | In the prompt? | In `/playground`? |
|---|---|---|---|
| `NOT NULL` | **pending** (new or edited) | **no** | yes — edit + approve |
| `NULL` | **approved / live** | **yes** | yes |

- **Brain read** — [`backend/internal/kbstore/kbstore.go`](../backend/internal/kbstore/kbstore.go),
  `LoadPublished` → renamed **`LoadLive`**: every content query gains `AND drafted_at IS NULL`,
  and the snapshot lookup reads the **single living KB row per org** (no `snapshot_state` /
  `version`).
- **Playground read** — [`backend/internal/kbstore/draft.go`](../backend/internal/kbstore/draft.go),
  `GetDraft`: returns **all** rows; `drafted_at` drives the "pending" badge.
- **Approve** = `UPDATE … SET drafted_at = NULL` for one row, or for all draft rows at once.
- **Add-on-top** = new media / values / products / tariffs insert with `drafted_at = now()`.
- **Editing a live row** re-marks it `drafted_at = now()` — it drops from the prompt until
  re-approved; the previous value is overwritten (accepted trade-off, no history).

### Schema delta from `0003`

- **`ai_snapshots` → one living KB row per org.** Drop `version`, `snapshot_state`,
  `published_at` (and the partial "one draft per org" index + the `UNIQUE(org, version)`
  constraint). Keep the config spine (persona / mission / guardrails / language_policy /
  reply_max_words) and the `organization_id` FK parent. New invariant: one row per org.
- **All content tables drop `review_state` and add `drafted_at timestamptz`** (NULL = live).
- **`ai_materials` / `ai_builder_requests`** are otherwise unchanged (they already key off the
  snapshot row, which still exists — only its lifecycle columns change).

> Because there is no published copy and no clone, opening the playground no longer duplicates
> anything (contrast `OpenDraft`'s clone-on-open in `0003`). Synthesis writes pending rows
> **directly** into the living KB and diffs candidates against the live (`drafted_at IS NULL`)
> rows — see [`plan/12`](../plan/12-playground-build.md).

---

## Principle 2 — Approval is gated for consistency

The deterministic gate
([`kbstore.go`](../backend/internal/kbstore/kbstore.go) `gate`, today called from `Publish`)
**moves from publish to approval**. It reuses `ValueBook.Render` exactly as today.

- **Approve all** → gate-check the resulting **live** set:
  - every `{{token}}` in a live topic / product / tariff body resolves (`ValueBook.Render`);
  - every owned media blob exists (`blob.Exists`);
  - no literal currency amount sits in a topic body (`rawCurrencyRE` — it must be a value token).
- **Approve one element** → block with a precise reason when it would leave a **dangling
  token**: e.g. approving the topic that says `{{price.nike_x}}` while `price.nike_x` is still
  pending → *"approve `price.nike_x` first, or use Approve all."*
- **Token uniqueness across all `ai_values`** is enforced here. The DB already guarantees one
  value per `(kb, token, lang)` via the existing unique key (the owner is descriptive, **not**
  part of the key), so a token resolves to exactly one value per language regardless of who
  owns it; the gate adds the cross-row resolvability check on top.

The old publish-gate is otherwise unchanged in spirit — it is the same pure, testable function
over the set of rows that are about to become live; only *when* it runs (approve, not publish)
and *which* set it sees (the post-approve live set) differ.

---

## Proposed schema — migration `0004` (design)

Thin entities + two extended shared tables. Style mirrors
[`0003_ai_kb.up.sql`](../backend/migrations/0003_ai_kb.up.sql): `xchats` schema,
`uuid_generate_v4()` PKs, `ON DELETE CASCADE` to the KB parent, `provenance jsonb`, and a
`(kb, ref)` natural key.

```sql
SET search_path = xchats, public;

-- A sellable item: descriptive only. Its price / numbers are owned ai_values rows;
-- its photos are owned ai_assets rows. No numeric columns (Principle 0).
CREATE TABLE IF NOT EXISTS xchats.ai_products (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    kb_id       uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    ref         text NOT NULL,                        -- stable key, e.g. 'nike-x'
    name        text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    category    text NOT NULL DEFAULT '',
    data        jsonb NOT NULL DEFAULT '{}'::jsonb,    -- sphere-specific attrs (size, color…)
    lang        text NOT NULL DEFAULT 'ru',
    status      text NOT NULL DEFAULT 'active',        -- 'active' | 'inactive'
    drafted_at  timestamptz,                           -- NULL = live; NOT NULL = pending
    provenance  jsonb NOT NULL DEFAULT '{}'::jsonb,    -- { source, material_id, at }
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kb_id, ref)
);

-- A pricing PLAN / tariff: a small descriptive spine. Its rates / fees / caps that the
-- assistant must quote exactly are owned ai_values rows (Principle 0).
CREATE TABLE IF NOT EXISTS xchats.ai_tariffs (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    kb_id         uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    ref           text NOT NULL,                       -- 'standard' | 'pro'
    name          text NOT NULL DEFAULT '',
    summary       text NOT NULL DEFAULT '',
    pricing_type  text NOT NULL DEFAULT 'fixed',       -- 'fixed'|'percentage'|'tiered'|'hybrid'
    advantages    text NOT NULL DEFAULT '',            -- free text input
    disadvantages text NOT NULL DEFAULT '',            -- free text input
    limits        jsonb NOT NULL DEFAULT '{}'::jsonb,  -- illustrative ranges {monthly_cap, per_tx_max}
    data          jsonb NOT NULL DEFAULT '{}'::jsonb,  -- description, conditions, billing_period, tiers…
    lang          text NOT NULL DEFAULT 'ru',
    status        text NOT NULL DEFAULT 'active',       -- 'active' | 'inactive'
    drafted_at    timestamptz,
    provenance    jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kb_id, ref)
);

-- Shared tables gain the polymorphic owner pair + drafted_at, and drop review_state.
-- ai_assets also drops topic_slug (its single-parent link is now the generic owner pair).
ALTER TABLE xchats.ai_values
    ADD COLUMN owner_kind text NOT NULL DEFAULT '',    -- 'topic'|'product'|'tariff'|''
    ADD COLUMN owner_ref  text NOT NULL DEFAULT '',    -- the owner's ref ('' = global scalar)
    ADD COLUMN drafted_at timestamptz,
    DROP COLUMN review_state;

ALTER TABLE xchats.ai_assets
    DROP COLUMN topic_slug,
    ADD COLUMN owner_kind text NOT NULL DEFAULT '',
    ADD COLUMN owner_ref  text NOT NULL DEFAULT '',
    ADD COLUMN drafted_at timestamptz,
    DROP COLUMN review_state;

ALTER TABLE xchats.ai_topics
    ADD COLUMN drafted_at timestamptz,
    DROP COLUMN review_state;

-- ai_snapshots becomes the single living KB per org (drop the versioned lifecycle).
DROP INDEX IF EXISTS xchats.ai_snapshots_one_draft_uq;
ALTER TABLE xchats.ai_snapshots
    DROP CONSTRAINT IF EXISTS ai_snapshots_organization_id_version_key,  -- old UNIQUE(org, version)
    DROP COLUMN version,
    DROP COLUMN snapshot_state,
    DROP COLUMN published_at,
    ADD CONSTRAINT ai_snapshots_one_per_org UNIQUE (organization_id);

-- Owner lookups (entity → its values/media) and live reads. ai_values/ai_assets are
-- existing 0003 tables, so they keep their snapshot_id parent column (only the new
-- products/tariffs tables use kb_id — see the naming note below).
CREATE INDEX IF NOT EXISTS ai_values_owner_idx ON xchats.ai_values(snapshot_id, owner_kind, owner_ref);
CREATE INDEX IF NOT EXISTS ai_assets_owner_idx ON xchats.ai_assets(snapshot_id, owner_kind, owner_ref);
-- Optional: partial indexes WHERE drafted_at IS NULL accelerate the brain's live read if the
-- KB grows large; skip until measured (the per-kb row counts are small in v1).
```

> ⚠️ **Naming note (`snapshot_id` vs `kb_id`) — flagged, not silently resolved.** `0003`'s
> content tables key off `snapshot_id`, and the `plan/9` convention is `<table>_id` (the parent
> table is still named `ai_snapshots`). This doc's **new** tables use `kb_id` to reflect the
> snapshot→living-KB rename (matching the verification's "`(kb_id, ref)` keys"), so the
> migration as written mixes `snapshot_id` (topics / assets / values / materials / requests)
> and `kb_id` (products / tariffs) against the **same** parent. That is intentional for this
> round but is an inconsistency — see the open sub-decision *"unify the parent-FK name"*. The
> embedding design does not depend on the choice; pick one before `0004` ships.

### `0004` down (sketch, FK order: children first)

A faithful reverse is **structurally** possible but **lossy** (the snapshot→KB collapse and the
`review_state`→`drafted_at` swap discard information). The down drops the new tables, restores
the dropped columns with their `0003` defaults, and re-adds the versioned-lifecycle columns:

```sql
SET search_path = xchats, public;
DROP TABLE IF EXISTS xchats.ai_tariffs;
DROP TABLE IF EXISTS xchats.ai_products;
ALTER TABLE xchats.ai_topics  ADD COLUMN review_state text NOT NULL DEFAULT 'approved', DROP COLUMN drafted_at;
ALTER TABLE xchats.ai_assets  ADD COLUMN review_state text NOT NULL DEFAULT 'approved', ADD COLUMN topic_slug text NOT NULL DEFAULT '',
                              DROP COLUMN drafted_at, DROP COLUMN owner_kind, DROP COLUMN owner_ref;
ALTER TABLE xchats.ai_values  ADD COLUMN review_state text NOT NULL DEFAULT 'approved',
                              DROP COLUMN drafted_at, DROP COLUMN owner_kind, DROP COLUMN owner_ref;
ALTER TABLE xchats.ai_snapshots DROP CONSTRAINT IF EXISTS ai_snapshots_one_per_org,
    ADD COLUMN version int NOT NULL DEFAULT 0,
    ADD COLUMN snapshot_state text NOT NULL DEFAULT 'draft',
    ADD COLUMN published_at timestamptz;
-- (UNIQUE(org, version) + the partial one-draft index would also be recreated.)
```

### Notes on the entity / value split

- A tariff's rate / fee / caps that the assistant **must quote exactly** → owned `ai_values`
  (`owner_kind='tariff'`). Purely **illustrative** ranges can stay in `limits` jsonb as context
  the model paraphrases (it never quotes them as exact). `tiers` for tiered plans live in `data`
  jsonb.
- The exact descriptive-column set on tariffs is tunable — `description` / `conditions` /
  `billing_period` could be promoted out of `data` into columns — without touching the embedding
  design. Captured as an open sub-decision.

---

## Consistency with `0003` (verification checklist 1)

| Property | `0003` (`ai_topics`/`assets`/`values`) | `0004` (`ai_products`/`ai_tariffs`) | Match |
|---|---|---|---|
| Schema | `xchats` | `xchats` | ✅ |
| PK | `uuid … DEFAULT uuid_generate_v4()` | same | ✅ |
| Parent FK + cascade | `… REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE` | same (col named `kb_id`) | ✅ FK/cascade · ⚠️ name |
| `provenance` | `jsonb NOT NULL DEFAULT '{}'::jsonb` | same | ✅ |
| Natural key | `UNIQUE (snapshot_id, slug/ref/token,lang)` | `UNIQUE (kb_id, ref)` | ✅ (per `plan/9`) |
| Draft flag | `review_state` (→ dropped) | `drafted_at timestamptz` | ✅ (new model) |
| Timestamps | `created_at` / `updated_at timestamptz DEFAULT now()` | same | ✅ |
| Polymorphic owner | added to `ai_values` + `ai_assets` (`owner_kind`,`owner_ref`) | products/tariffs are *owners*, referenced by `owner_ref` | ✅ |

The only deliberate divergence is the parent-FK **name** (`kb_id` vs `snapshot_id`), flagged
above.

---

## Integration touchpoints (spec for the follow-up implementation round)

> Reference only — **no code changes this round.** File links resolve to current line counts.

1. **Domain** —
   [`backend/internal/brain/domain/content.go`](../backend/internal/brain/domain/content.go):
   add `Product` and `Tariff` structs; `Snapshot` gains `Products []Product` and
   `Tariffs []Tariff`; `Value` and `Asset` gain `OwnerKind` / `OwnerRef` (and `Asset` drops
   `TopicSlug`); **all owned values still flow into the one `ValueBook`** — `Render` is
   untouched.
2. **Load** —
   [`kbstore.go`](../backend/internal/kbstore/kbstore.go) `loadSnapshotContent`: replace the
   `review_state='approved'` filter with `drafted_at IS NULL`; add product / tariff scans; the
   values / assets scans select `owner_kind` / `owner_ref`. Rename `LoadPublished → LoadLive`
   (reads the one KB row per org). `insertContent` writes `drafted_at` (NULL for seed/clone,
   `now()` for proposals) instead of `review_state`, and writes the owner pair on values /
   assets. `Publish` / `Rollback` are **removed** (no versions); `SeedIfEmpty` seeds the single
   live KB row.
3. **System prompt** —
   [`backend/internal/brain/prompt.go`](../backend/internal/brain/prompt.go) `BuildSystem`: add
   a `PRODUCT CATALOG:` section and a `TARIFFS:` section that list each entity together with its
   owned **value tokens** (so the model emits `{{price.nike_x}}`, never a digit — frame rule 2
   is unchanged) and its owned **media refs**. The `MEDIA CATALOG:` header line changes from
   `ref | kind | topic | description` to `ref | kind | owner | description`.
4. **Approval gate** —
   relocate `gate` from `Publish` to **approve-all** / **approve-element** (Principle 2); reuse
   `ValueBook.Render` and `rawCurrencyRE`; enforce token uniqueness.
5. **Draft store + API** —
   [`draft.go`](../backend/internal/kbstore/draft.go),
   [`playground.go`](../backend/internal/httpapi/playground.go),
   [`server.go`](../backend/internal/httpapi/server.go): `GetDraft` returns **all** rows
   including `drafted_at` (rows expose `drafted_at` instead of `review_state`); add CRUD for
   products & tariffs mirroring `UpsertValue` / `UpsertTopic`; value / asset upserts accept
   `owner_kind` / `owner_ref`; **new routes** `POST /playground/approve` (all) and
   `POST /playground/approve/:kind/:id` (one) **replace** the current
   `POST /playground/draft/review/:kind/:id`, `POST /playground/publish`, and
   `POST /playground/rollback`. Add-on-top writes set `drafted_at = now()`; editing a live row
   re-marks it `drafted_at = now()`.
6. **Frontend** —
   [`frontend/src/views/KnowledgeBase.vue`](../frontend/src/views/KnowledgeBase.vue),
   [`frontend/src/types.ts`](../frontend/src/types.ts): new **"Товары"** and **"Тарифы"** tabs
   alongside the existing Темы / Медиа-ресурсы / Значения tabs; each entity editor shows its
   **owned values** (the embeddable tokens) and **owned media** grouped under it, plus an
   "add media / add value" affordance; "pending" badges + per-row / bulk **Approve** driven by
   `drafted_at` (replacing the `review_state === 'proposed'` logic, including the Правки tab).
   `types.ts`: drop `ReviewState` in favor of `drafted_at: string | null`; add `Product` /
   `Tariff` interfaces; `ValueRow` / `AssetRow` gain `owner_kind` / `owner_ref`; `DraftView`
   gains `products` / `tariffs`.
7. **Synthesis (future)** —
   [`backend/internal/playground/builder.go`](../backend/internal/playground/builder.go): the
   `Builder` / `RuleSynthesizer` emit product / tariff proposals + owned values from materials,
   created with `drafted_at = now()` (pending) instead of `ReviewState: "proposed"`.

---

## End-to-end walkthrough (verification checklist 3)

The column-free embedding path, start to finish:

1. **Create, as drafts** (each `drafted_at = now()`):
   - `ai_products`: `ref='nike-x'`, `name='Nike X'`, `category='Обувь'`, `data={size:'40-45'}`
     — *thin, no numbers.*
   - `ai_values`: `token='price.nike_x'`, `value_text='25 000 ₸'`, `owner_kind='product'`,
     `owner_ref='nike-x'`.
   - `ai_assets`: `ref='img-42'`, `description='Кроссовки Nike X, вид сбоку'`,
     `owner_kind='product'`, `owner_ref='nike-x'`.
2. **Playground** (`GetDraft`) shows all three **grouped under `nike-x`** and badged
   **pending** (`drafted_at` is set). The brain does **not** see them yet (`LoadLive` filters
   `drafted_at IS NULL`).
3. **Approve all** → the gate runs over the resulting live set: `price.nike_x` resolves, the
   `img-42` blob exists, no literal currency sits in a topic body → **pass** →
   `UPDATE … SET drafted_at = NULL` on all three rows.
4. A topic / product body containing `{{price.nike_x}}` is now live. At reply time
   `ValueBook.Render(body, lang)` substitutes the token and emits **`25 000 ₸`** verbatim —
   the renderer touched only `ai_values`, **never** the `ai_products` table (verification
   checklist 2: the embedding path is column-free).

Counter-example the gate blocks: **Approve only** the topic while `price.nike_x` is still
pending → `ValueBook.Render` fails on the dangling token → *"approve `price.nike_x` first, or
use Approve all."*

---

## Verification (this doc round)

- ✅ **`0004` DDL is consistent with `0003`** — FK + `ON DELETE CASCADE` to the KB parent,
  `provenance jsonb DEFAULT '{}'`, `(kb_id, ref)` natural keys, and the polymorphic
  `owner_kind` / `owner_ref` pair added to **both** `ai_values` and `ai_assets`. The single
  deliberate divergence (parent-FK **name** `kb_id` vs `0003`'s `snapshot_id`) is called out in
  the *Naming note* and the *Consistency with 0003* table.
- ✅ **The embedding path is column-free** — a `{{price.nike_x}}` token resolves through
  `ai_values` only (`ValueBook.Render`, `tokenRE`); no entity column is referenced anywhere in
  template text, and no numeric column exists on `ai_products` / `ai_tariffs` to reference.
- ✅ **End-to-end example walks** — create thin `nike-x` + value `price.nike_x` + photo
  `img-42` (all `owner_ref='nike-x'`, all drafts) → playground shows them grouped & pending →
  approve-all (gate passes) → a reply with `{{price.nike_x}}` renders `25 000 ₸`.

---

## Open sub-decisions (captured, not blockers)

1. **Media / value reuse across entities.** Today an asset / value has a **single** owner
   (`owner_kind` + `owner_ref`). If a photo or a number must belong to several entities, a later
   migration adds a join table (`ai_owner_links`) without changing the token mechanism. Lean:
   single owner now.
2. **Tariff descriptors: `data` jsonb vs columns.** `description` / `conditions` /
   `billing_period` could be promoted from `data` jsonb into first-class columns. The embedding
   design is indifferent; promote only what the UI / queries need to filter or sort on.
3. **Config drafting shape.** The KB **config** (persona / mission / guardrails / …) has no
   `drafted_at` of its own. Options: a `pending jsonb` overlay on the KB row, or a shadow
   config row. Lean: a `pending jsonb` overlay, decided when config editing lands.
4. **Drop `inactive` entities from the prompt entirely.** Whether `status='inactive'`
   products / tariffs are excluded from `BuildSystem` (in addition to being pending-gated).
   Lean: **yes** — inactive means "not for sale", so keep it out of the model's context.
5. **Unify the parent-FK name.** Resolve `kb_id` (new tables) vs `snapshot_id` (existing
   tables) before `0004` ships — either rename existing columns to `kb_id` (and consider
   renaming the `ai_snapshots` table to `ai_kb`) for uniformity, or keep `snapshot_id`
   everywhere and treat `kb_id` as the documented exception. Lean: unify on `kb_id` alongside
   the snapshot→KB rename.
```
