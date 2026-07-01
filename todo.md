# Architecture: `drafted_at` KB model + typed fact tables + polymorphic media

> **⚠️ Superseded in part by `plan/13-kb-facts-and-grounding` (Decision 13).** The `drafted_at` lifecycle,
> `ai_products`/`ai_tariffs`, and **polymorphic media** below still hold. What **changed**: exact facts are
> no longer a generic `ai_values` token bag with polymorphic `owner_kind/owner_ref` — they are now **typed
> columns** on typed fact tables (`ai_tariffs`/`ai_products`/`ai_contacts`), quoted as `{{table.slug.field}}`,
> with **language as a row**. `ai_values` is removed. Principle 0 and the DDL below are updated to match;
> media stays polymorphic. Decision 13 also adds a runtime **number check** + **prose grounding judge** over
> each draft (see `plan/8.2` / `plan/8.7`).

## Context

**Why this change.** Three needs converged:

1. **A simpler draft/approval model.** Replace versioned *snapshots* + per-row
   `review_state` with a single living KB per org where each row is flagged pending via a
   `drafted_at` timestamp and held out of the prompt until a human approves it.
2. **Structured catalog data** for many spheres: add **`ai_products`** (a sellable item)
   and **`ai_tariffs`** (a pricing *plan* — fixed/percentage, limits, pros/cons). The two
   are independent (no links).
3. **Media on any entity** (polymorphic), and **exact facts as typed columns** on typed fact tables.

**The central design principle (Principle 0 below):** exact facts live as **typed columns** on typed fact
tables (`ai_tariffs`/`ai_products`/`ai_contacts`), stored verbatim, **one row per language**; every media
file lives in `ai_assets` and attaches to its entity through one shared `(owner_kind, owner_ref)` pair.
Reply templates quote a fact only as `{{table.slug.field}}` (table→row→column). This keeps facts exact and
un-inventable while media stays uniform.

**Decisions (with the user).**
- Draft = a per-row `drafted_at` timestamp; approval is per-element or all-at-once.
- `ai_products` and `ai_tariffs` are independent (no FKs between them).
- Exact facts ARE typed columns (`price`/`limit_text`/`fee`/`whatsapp`/…); the token `{{table.slug.field}}`
  resolves a column for the reply language. The generic `ai_values` bag is **removed**; **language is a row**.
- `advantages`/`disadvantages` are text columns; descriptive prose (conditions/tiers) is `data` jsonb.
- Accepted trade-off: no versioned history / rollback.

This round delivers the **architecture doc only**; migration + Go/Vue code follow.

---

## Principle 0 — Typed fact tables (values as columns); media attaches polymorphically

**The embedding question, answered (per Decision 13):** an exact fact is a **typed column** on a typed
fact table; the one way to put it into text is a `{{table.slug.field}}` token, resolved for the reply's
language by `Render` ([content.go](backend/internal/brain/domain/content.go)). **Language is a row**, not a
column; the old generic `ai_values` token bag is **removed** (a nearest-key lookup can return the *wrong*
tariff).

```
ai_products  ref='nike-x'  lang='ru'  name='Nike X'  price='25 000 ₸'  category='Обувь'  data={size:'40-45'}
ai_assets    ref='img-42'  description='...'  owner_kind='product'  owner_ref='nike-x'   ← media stays polymorphic
```

- A reply template says `{{product.nike_x.price}}` → renders `25 000 ₸` verbatim for the reply language.
- An entity's exact facts = its **typed columns** (`price`, `limit_text`, `fee`, `whatsapp`, …).
- An entity's media = `SELECT … FROM ai_assets WHERE owner_kind=$1 AND owner_ref=$2` (polymorphic).
- `owner_kind ∈ {'topic','product','tariff',''}` applies to **media only**; `''` = an unattached asset.

This (a) makes exact facts typed and un-inventable, (b) keeps one uniform token grammar
`{{table.slug.field}}`, (c) keeps media polymorphic, and (d) makes language a row (a new language is new
rows, no schema change) — preserving the hallucination-safety the token system exists for.

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

- **Approve all** → gate-check the resulting live set (every `{{table.slug.field}}` in a live body
  resolves to a column value **for each required language**; every owned media blob exists; no literal
  currency in a topic body).
- **Approve one element** → block with a precise reason if it would leave a dangling token
  ("approve the `tariff.growth` row first, or use Approve all").

Reuses the fact-token `Render` path. Per-language completeness is enforced here.

---

## Proposed schema — migration `0004` (design)

Thin entities + two extended shared tables.

```sql
-- A sellable item (typed FACT table): its price is a typed column, verbatim, one row per language.
CREATE TABLE xchats.ai_products (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    kb_id       uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    ref         text NOT NULL,                       -- stable key 'nike-x'
    lang        text NOT NULL DEFAULT 'ru',          -- language is a ROW
    name        text NOT NULL DEFAULT '',
    price       text NOT NULL DEFAULT '',            -- verbatim WITH units ('25 000 ₸'); {{product.<ref>.price}}
    description text NOT NULL DEFAULT '',
    category    text NOT NULL DEFAULT '',
    data        jsonb NOT NULL DEFAULT '{}'::jsonb,   -- sphere-specific DESCRIPTIVE attrs (size, color…)
    status      text NOT NULL DEFAULT 'active',
    drafted_at  timestamptz,
    provenance  jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kb_id, ref, lang)
);

-- A pricing PLAN / tariff (typed FACT table): quotable numbers are typed columns, verbatim, one row per language.
CREATE TABLE xchats.ai_tariffs (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    kb_id         uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    ref           text NOT NULL,                      -- 'growth' | 'pro'
    lang          text NOT NULL DEFAULT 'ru',         -- language is a ROW
    name          text NOT NULL DEFAULT '',
    price         text NOT NULL DEFAULT '',           -- verbatim ('25 000 ₸/мес'); '' if fee-based
    limit_text    text NOT NULL DEFAULT '',           -- verbatim ('до 2 000 платежей/мес')
    fee           text NOT NULL DEFAULT '',           -- percentage plans ('1.5 % за транзакцию')
    summary       text NOT NULL DEFAULT '',
    pricing_type  text NOT NULL DEFAULT 'fixed',      -- 'fixed'|'percentage'|'tiered'|'hybrid'
    advantages    text NOT NULL DEFAULT '',           -- text input
    disadvantages text NOT NULL DEFAULT '',           -- text input
    data          jsonb NOT NULL DEFAULT '{}'::jsonb, -- descriptive prose: conditions, billing_period, tiers
    status        text NOT NULL DEFAULT 'active',
    drafted_at    timestamptz,
    provenance    jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kb_id, ref, lang)
);

-- Media stays polymorphic on ai_assets; ai_values is REMOVED (facts are typed columns above).
ALTER ai_assets  DROP topic_slug,
                 ADD owner_kind text NOT NULL DEFAULT '',  ADD owner_ref text NOT NULL DEFAULT '',
                 DROP review_state, ADD drafted_at timestamptz;
ALTER ai_topics  DROP review_state, ADD drafted_at timestamptz;
DROP TABLE IF EXISTS xchats.ai_values;

-- Org-level support scalars: a dedicated typed fact table (singleton slug 'support', one row per language).
CREATE TABLE xchats.ai_contacts (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    kb_id         uuid NOT NULL REFERENCES xchats.ai_snapshots(id) ON DELETE CASCADE,
    slug          text NOT NULL DEFAULT 'support',
    lang          text NOT NULL DEFAULT '*',          -- '*' = language-neutral (phones/e-mails/addresses)
    whatsapp      text NOT NULL DEFAULT '',
    email         text NOT NULL DEFAULT '',
    address       text NOT NULL DEFAULT '',
    legal         text NOT NULL DEFAULT '',
    callback_time text NOT NULL DEFAULT '',           -- language-bearing ('1 час' / '1 сағат')
    drafted_at    timestamptz,
    provenance    jsonb NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE (kb_id, lang)
);
```

Notes:
- A tariff's rate / fee / caps that the assistant must quote exactly → **typed columns**
  (`price`/`limit_text`/`fee`); purely descriptive ranges the model may paraphrase stay in `data`
  jsonb (with `tiers` for tiered plans).
- The exact fact-column set is tunable per sphere (add columns as the domain needs) — the token
  grammar `{{table.slug.field}}` doesn't change when you do.

---

## Integration touchpoints (spec for the follow-up implementation round)

1. **Domain** — [content.go](backend/internal/brain/domain/content.go): add `Product`,
   `Tariff`, `Contact` (typed fact rows with `Price`/`LimitText`/`Fee`/… columns, keyed by
   `(ref,lang)`), `snap.Products`/`snap.Tariffs`/`snap.Contacts`; `Asset` gains `OwnerKind/OwnerRef`;
   drop the `ValueBook`.
2. **Load** — [kbstore.go](backend/internal/kbstore/kbstore.go) `loadSnapshotContent`:
   filter `drafted_at IS NULL`; add product/tariff/contact scans (per `ref,lang`); the assets scan reads owners.
3. **System prompt** — [prompt.go](backend/internal/brain/prompt.go) `BuildSystem`: add
   `PRODUCT CATALOG:` + `TARIFFS:` + `FACTS:` sections that list each entity's fact columns as
   `{{table.slug.field}}` tokens (so the model emits `{{product.nike_x.price}}`) with a human label +
   value, plus owned media refs; `MEDIA CATALOG:` line becomes `ref | kind | owner | description`.
4. **Approval gate** — relocate `gate` to approve-all / approve-element (Principle 2).
5. **Draft store + API** — [draft.go](backend/internal/kbstore/draft.go),
   [playground.go](backend/internal/httpapi/playground.go),
   [server.go](backend/internal/httpapi/server.go): `GetDraft` returns all rows incl.
   `drafted_at`; CRUD for products/tariffs/contacts (typed fact columns); asset upserts
   accept `owner_kind`/`owner_ref`; approve routes (`POST /playground/approve`,
   `…/approve/:kind/:id`).
6. **Frontend** — [KnowledgeBase.vue](frontend/src/views/KnowledgeBase.vue),
   [types.ts](frontend/src/types.ts): new **"Товары"** / **"Тарифы"** / **"Контакты"** tabs; each
   entity editor shows its **typed fact fields** (price/limit_text/fee, verbatim) and **owned media**
   grouped under it, plus an "add media" affordance; "pending" badges + per-row/bulk **Approve**
   driven by `drafted_at`.
7. **Synthesis (future)** —
   [playground/builder.go](backend/internal/playground/builder.go) emits product/tariff/contact
   rows with typed fact columns from materials (each exact value via a `confirm_fact` popup),
   created with `drafted_at` set for approval.

---

## Deliverable for this round

Architecture doc only. Create **`docs/products-tariffs-and-media.md`** (new `docs/` dir at
the xchats root, matching the `docs/NN` convention the brain comments reference). It
contains Principles 0–2, the `0004` DDL + schema delta, and the integration checklist.
**No migration is run, no Go/Vue code changes this round.**

## Verification (doc round)

- Confirm the `0004` DDL is consistent with
  [0003_ai_kb.up.sql](backend/migrations/0003_ai_kb.up.sql) (FK + cascade, `provenance`,
  `(kb_id, ref, lang)` keys, polymorphic `owner_kind/owner_ref` on **assets (media) only** — `ai_values`
  dropped, facts are typed columns).
- Confirm facts are typed columns: `{{product.nike_x.price}}` resolves to the `price` column for the
  reply language; `ai_values` is gone; no generic token bag remains.
- Walk one end-to-end example: create product `nike-x` (ru row with `price='25 000 ₸'`) +
  photo `img-42` (`owner_ref='nike-x'`) as drafts → playground shows them grouped &
  pending → approve-all (gate passes) → a reply with `{{product.nike_x.price}}` renders `25 000 ₸`.

## Open sub-decisions (in the doc, not blockers)

- Media reuse across entities (single owner now vs a join table later).
- Which tariff descriptors stay in `data` jsonb vs get promoted to typed columns.
- Config drafting shape (`pending jsonb` overlay vs shadow row).
- Whether `inactive` products/tariffs are dropped from the prompt entirely (lean yes).
