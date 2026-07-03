# 15 — Table Groups & Prefixes, Org Keys & the Draft Blob — Architecture Decision

**Purpose:** lock the KB's table taxonomy — **three prefix-delimited groups** (`ai_` / `kbd_` / `rp_`)
— plus two schema simplifications: key every KB table on `organization_id` (retire `snapshot_id`), and
store the draft KB as **one jsonb blob per org** (retire `drafted_at` and the twin-table design). This
is a decision record, not an implementation plan. Where it conflicts with an older `plan/*.md`, **this
record wins**; affected docs carry a banner and are rewritten lazily (see "Docs affected").

> **Amends [`14`](14-draft-staging-and-retrieval.md):** Decision 1's *draft twin tables* are
> **replaced** by a single draft jsonb blob (Decision 3 below). Decision 2's *approve = gate → copy →
> embed* **stands**, with the copy source now the blob, not a twin table. Everything else in 14 holds.
>
> **Note — docs vs code.** These are the *target* names for the design docs. The real backend today
> uses a different model (`ai_values` bag, `ai_snapshots.snapshot_state='draft'`, `ai_drafts`); moving
> the code to this record's names is a **separate backend migration**, out of scope for the doc pass.

---

## Problem

Three things were still wrong or unsettled after Decision 14:

- **`snapshot_id` is a fossil.** It dates from the versioned-snapshot design (many snapshots per org).
  That design is dead — the KB is **one living set per org** — so `snapshot_id` is now a 1:1 stand-in
  for `organization_id`: a pointless join hop wearing a name that implies versioning that no longer
  exists.
- **`drafted_at` never died in the prose.** Decision 14 removed the per-row draft flag in favour of
  separate draft storage, but the flag still litters the docs. And the twin-table shape 14 chose is
  heavier than the draft actually needs.
- **The table taxonomy was implicit.** Which tables are the live KB, which are its draft, which are the
  per-chat response — never stated as groups, and nothing in the name told them apart.

## Decision 1 — Key on `organization_id`; rename `ai_snapshots` → `ai_assistants`

- Every KB table keys on **`organization_id`** directly (FK → `organizations`). **`snapshot_id` is
  retired** across `ai_topics` / `ai_assets` / `ai_tariffs` / `ai_products` / `ai_contacts` /
  `kbd_materials` / `kbd_requests`.
- The config table `ai_snapshots` is renamed **`ai_assistants`** — one config row per org
  (`persona` / `mission` / `guardrails` / `language_policy` / `reply_max_words`),
  `UNIQUE (organization_id)`. It is **not** a snapshot of anything; the old name misled.
- No more "org derived via snapshot join." `organization_id` is a **direct** column on every KB table.
- **If** multiple assistants/KBs per org is ever needed, that is a future additive `assistant_id` —
  not a reason to keep `snapshot_id` now.

## Decision 2 — `drafted_at` is removed everywhere

- The per-row `drafted_at timestamptz NULL` lifecycle is **deleted** from every table. Live tables
  hold **live rows only** — nothing pending, no flag to filter on. The brain reads a live table whole.
- The per-row `provenance jsonb` column also leaves the live tables — provenance is an authoring
  concern that lives in the draft blob (below); approve history lives in `ai_audit_log`.

## Decision 3 — Draft KB = one jsonb blob per org (`kbd_draft`), not twin tables

- The entire pending KB is **one `jsonb` document**, one row per org, in a dedicated table
  `kbd_draft`. The playground reads/writes this blob as a whole working copy; the **brain never
  touches it**. This **overrides Decision 14's twin-table shape**.
- Blob shape mirrors the live typed tables — one array per entity kind:

```jsonc
// kbd_draft.draft
{
  "config":   { "persona": "…", "mission": "…", "guardrails": "…", "language_policy": "…", "reply_max_words": 120 },
  "topics":   [ { "slug": "pricing", "lang": "ru", "keywords": "…", "body_md": "…prose only…", "provenance": {…} } ],
  "assets":   [ { "ref": "price_pdf", "asset_kind": "document", "owner_kind": "topic", "owner_ref": "pricing", "description": "…", "asset_url": "…" } ],
  "tariffs":  [ { "ref": "growth", "lang": "ru", "name": "Рост", "price": "25 000 ₸/мес", "limit_text": "…", "fee": "", "pricing_type": "fixed", "…": "…" } ],
  "products": [ { "ref": "nike-x", "lang": "ru", "name": "…", "price": "25 000 ₸", "…": "…" } ],
  "contacts": [ { "lang": "*", "whatsapp": "+7 …", "email": "…", "address": "…", "legal": "…" } ],
  "deletes":  [ { "kind": "tariff", "ref": "old_plan", "lang": "ru" } ]   // delete-markers, applied at approve
}
```

- Why a blob, not twins: the draft is only ever read/written **as a whole document** by one operator's
  playground session — it needs no relational querying, no per-row indexes, no schema kept in lockstep
  with N live tables. "One separate place" is literally one column.
- Rejecting a pending edit = drop that entity from the blob. Deleting a live entity = a `deletes[]`
  marker, applied at approve.

## Decision 4 — Approve = validate blob → materialize into live tables → clear (+ embed)

- Approve takes the **whole draft** or a **selected subset of entities**. It runs the deterministic
  gate over the resulting live set, then **upserts** the approved entities into the **live typed
  tables** (on their natural key, e.g. `(organization_id, ref, lang)`), applies any `deletes[]`,
  **removes those entities from the blob**, and **refreshes embeddings** for approved topics
  (Decision 14 Decision 5 stands). One approve writes an `ai_audit_log` row.
- Approve stays the **only write path to live**. The brain reloads its live snapshot after approve.
- Partial approve is a blob selection, not a row flag — pick keys out of the one draft document.

## Decision 5 — Three table-prefix groups (the taxonomy)

Every AI/KB table carries a prefix that names its **group**, so the schema reads at a glance:

| Prefix | Group | Who writes | Who reads | Tables |
|---|---|---|---|---|
| **`ai_`** | **Live knowledge base** | approve only | the **brain** | `ai_assistants` (config) · `ai_topics` · `ai_assets` · `ai_tariffs` · `ai_products` · `ai_contacts` · `ai_audit_log` |
| **`kbd_`** | **Draft + playground staging** | the **playground** | the playground | `kbd_draft` (one jsonb blob/org) · `kbd_materials` (ingest staging) · `kbd_requests` (popup queue) |
| **`rp_`** | **Response suggestions** (per chat) | the brain worker | the inbox UI | `rp_suggestions` |

- **`ai_` — live KB** the brain answers from: pure live rows, keyed `organization_id`. `ai_audit_log`
  (approve/edit history) rides along here as KB-lifecycle history.
- **`kbd_` — draft + staging**: the one working draft (`kbd_draft`, Decision 3) **plus** the
  playground-only scratch that tracks job status — `kbd_materials` (dropped files being extracted;
  `status` = pending/extracting/ready/needs_human/failed) and `kbd_requests` (the popup/human-in-the-loop
  queue; `state` = pending/resolved/dismissed). None are read by the brain; all discarded once resolved.
  Approve turns `kbd_` → `ai_` (Decision 4).
- **`rp_` — response suggestions**: the per-chat reply the brain produces from the `ai_` KB — tied to a
  `wa_chats` row, not to the KB. One in-flight suggestion per chat
  (`PARTIAL UNIQUE (chat_id) WHERE state IN ('generating','suggested')`), 1–3 variants in `options`.
- Existing conventions unchanged: **`wa_`** = WhatsApp transport; **unprefixed** = shared identity
  (`organizations` / `users` / `organization_users` / `sessions`).

## Decision 6 — Settle the suggestion-table name: `rp_suggestions`

- The runtime response table is **`rp_suggestions`**. This replaces the docs' `ai_suggestions` **and**
  the live migration's `ai_drafts` / `ai_draft_assets` — one name, no divergence. There are no
  per-option rows (the 1–3 variants live in `options` jsonb), so `ai_draft_assets` disappears.

---

## Final table catalog (authoritative, post-15)

```text
GROUP ai_  — LIVE KB (brain reads; organization_id key)
  ai_assistants   organization_id PK · persona · mission · guardrails · language_policy · reply_max_words   (was ai_snapshots)
  ai_topics       organization_id · slug · lang · keywords · body_md (pure prose)                UNIQUE(organization_id, slug)
  ai_assets       organization_id · ref · asset_kind · owner_kind · owner_ref · description · asset_url      UNIQUE(organization_id, ref)
  ai_tariffs      organization_id · ref · lang · name · price · limit_text · fee · pricing_type · advantages · disadvantages · data · status   UNIQUE(organization_id, ref, lang)
  ai_products     organization_id · ref · lang · name · price · description · category · data · status       UNIQUE(organization_id, ref, lang)
  ai_contacts     organization_id · lang · whatsapp · email · address · legal · callback_time                UNIQUE(organization_id, lang)
  ai_audit_log    organization_id · action · actor_user_id · note · created_at

GROUP kbd_ — DRAFT + PLAYGROUND STAGING (playground only)
  kbd_draft       organization_id PK · draft jsonb · base_version · updated_at · updated_by                   (whole draft KB, 1 row/org)
  kbd_materials   organization_id · source_type · source_ref · blob_id · extracted_text · media_kind · status · extraction   (was ai_materials)
  kbd_requests    organization_id · material_id · req_type · prompt · context · target · state · resolution   (was ai_builder_requests)

GROUP rp_  — RESPONSE SUGGESTIONS (per chat; runtime output)
  rp_suggestions  chat_id · trigger_message_id · state · reply_language · options jsonb · …   PARTIAL UNIQUE(chat_id) WHERE state IN ('generating','suggested')   (was ai_suggestions / ai_drafts)
```

`ai_values` (the old generic value bag) stays **removed** — facts are typed columns only.

---

## Explicitly rejected / out of scope

- **`snapshot_id` indirection** — retired; `organization_id` is direct (Decision 1).
- **`drafted_at` per-row flag** — removed; drafts live in the blob (Decisions 2–3).
- **Draft twin tables** (14 Decision 1) — replaced by the blob (Decision 3).
- **Per-row `provenance` on live tables** — moved into the blob + `ai_audit_log` (Decision 2).
- **Versioning / rollback of the live KB** — still out (one living set); the blob is a working draft,
  not a version history.
- **`assistant_id` / multiple KBs per org** — future additive change, not v1.
- **The backend code migration** (rewrite `0003_ai_kb`, `kbstore.go`, `seed.go` to these names) — a
  separate effort; this record governs the **docs**.

---

## Docs affected (bannered now, rewritten lazily)

| Doc | What this record changes there |
|---|---|
| `9-database-schema.md` | prefix convention in Conventions; `kbd_draft`/`kbd_materials`/`kbd_requests`; `rp_suggestions`; the three groups visually sectioned |
| `12-playground-build.md` | data model, layers, approve gate, endpoints, frontend: `kbd_draft` blob (not `drafted_at`), `kbd_materials`/`kbd_requests`, org keys, `rp_suggestions` |
| `11-ai-design-overview.md` | §2 (`drafted_at`) → `kbd_draft` blob; the diagram's `drafted_at IS NULL` → live tables |
| `5-ui-pages.md` | «Черновик» badge = "entity present in `kbd_draft.draft`", not `drafted_at` |
| `8-ai-assistant.md` | lifecycle wording: `kbd_draft` blob not twin tables; `ai_assistants` name; `rp_suggestions`; org keys |
| `7.1-endpoints.md` | playground draft/approve endpoints operate on the `kbd_draft` blob; `snapshot_id` gone; `rp_suggestions` |
| `2-architecture.md` | `ai_snapshots` mention → `ai_assistants` |
| `13` / `14` | banners: 14's twin tables and any `drafted_at` are superseded here; names are `kbd_`/`rp_` |
