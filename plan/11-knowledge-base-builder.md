# Knowledge Base Builder — the option (configure the assistant by chatting)

This is the **concrete, buildable implementation** of the experience sketched in
`10-knowledge-builder.md`. `10` is the *what/why* (conceptual UX). The `11.*` set is the *how*:
schema, API, the builder agent, the git-like diff system, the UI, and the phased build plan.

> **One line:** the operator **talks to a builder assistant** — drops files, links, notes, answers a
> few questions — and the assistant turns it into a reviewed **draft Snapshot** (identity · goal ·
> quality · support + topics-of-media + prices), showing a **git-style diff before any important
> change**, and never shipping anything to a customer until a human **publishes**.

---

## 1. What the user asked for → how this plan delivers it

| Requirement (from the brief) | How `11.*` delivers it | Where |
|---|---|---|
| Configure the assistant **through chat** | A **builder chat** (LLM tool-calling agent) that CRUDs the draft Snapshot via tools | `11.3` |
| Upload/attach files, send links, add notes | `attach` (multipart → blob) + `note`/`link` intake; a `Normalizer` per source type | `11.2`, `11.3` |
| Answer **clarifying questions**, approve what it learns | **Requests** (popups) as human-in-the-loop tool calls; accept/deny/edit per row | `11.2`, `11.4` |
| Managed through **structured blocks**: identity, goal, quality, support, topics | The Snapshot **config blocks** (`persona/mission/guardrails/support_policy`) + topics | `11.1`, §4 |
| **Topic = text + several media** (image/PDF/video/link/infographic), **each with its own description** | The existing **topic-as-container** model (`ai_topics.body_md` + N `ai_assets`, each `description`) | `11.1`, `8-ai-assistant.md` |
| As **automatic as possible**, but no expensive auto-extraction in v1 | Text/URL/PDF auto-structured; media → **`describe_media` popup** (operator describes); vision/transcription drop in later behind the same UX | `11.3` |
| **Proactive**: ask when ambiguous, suggest improvements | The agent's **proactive policy**: ask-don't-guess, propose merges, confirm prices, flag gaps | `11.3` |
| **Git-like change diffs** before important updates | A first-class **Changeset** model (typed change-ops with before/after) previewed as a diff, applied atomically | §5, `11.1`, `11.4` |
| Keep the KB **clean, useful, ready** for support/sales | Dedup/merge, provenance, coverage hints, and the **publish gate** (price-safety = 1.0, asset-precision ≥ 0.9) | `11.3`, `11.5` |

**Net:** almost nothing here is greenfield invention — it is the **authoring front-end** to the
brain's existing `ai_*` snapshot tables. The brain (`8-ai-assistant.md`) is **untouched on the hot
path**; the builder only writes the *draft* it will later read on publish.

---

## 2. Design principles (the best-practice spine)

1. **One source of truth, two speeds.** The builder chat and the editor page edit the **same draft
   Snapshot rows** — no parallel store, no sync problem (chat = fast/bulk, editor = precise). (`10`.)
2. **The builder proposes; humans dispose.** Auto-created rows are `proposed`; important changes go
   through a **changeset** the human approves. Three gates stand between a guess and a customer
   (draft-only → publish gate → suggest-and-approve runtime). A mistake must survive **all three**.
3. **Reuse the runtime model; don't fork it.** Same `ai_snapshots/ai_topics/ai_assets/ai_prices`,
   same `topic_slug` container link, same price-token discipline. **No brain logic change** beyond one
   tiny additive prompt field (the `support_policy` block — §4).
4. **Ask, don't guess.** Anything ambiguous or unextractable becomes a **request** (popup), not a
   silent row. This is what makes "as automatic as possible" *safe*.
5. **Numbers are sacred.** A price/limit is **never** auto-baked into prose — the agent writes a
   `{{token}}` and raises `confirm_price`; the real digit enters the price book only by human OK.
6. **Everything is provenance-tracked.** Every draft row and every change-op records *what produced
   it* (chat message / URL / file), shown in the diff and editor for one-click verification.
7. **Provider-neutral, same plumbing.** The builder agent speaks the same OpenAI-compatible API and
   `LLM_*` env as the brain; it is a *different prompt + tool set*, not a different stack.
8. **Additive only.** Every schema change is a new table or a nullable column. The live runtime tables
   and the brain stay byte-compatible; the builder is a clean bolt-on (`11.1`).

---

## 3. Architecture — where the builder sits

```text
                          ┌──────────────────────────── BUILDER (new) ────────────────────────────┐
 operator's browser       │                                                                        │
   builder chat  ───────► │  POST /assistant/builder/messages  (text · note · link)                │
   + file drop  ───────►  │  POST /assistant/builder/attachments (multipart → blob)                │
   popups  ◄────────────► │  GET/POST …/requests/{id}   (describe_media · confirm_price · …)        │
   diff cards ◄─────────► │  GET …/changesets/{id}      POST …/changesets/{id}/apply               │
        ▲                 │        │                                                                 │
        │ SSE builder.*   │        ▼                                                                 │
        │                 │  ┌───────────────┐   tool calls   ┌──────────────────────────────────┐  │
        └─────────────────┼─ │ BUILDER AGENT │ ─────────────► │ DRAFT SNAPSHOT (ai_*,            │  │
                          │  │ (LLM loop)    │ ◄───────────── │  snapshot_state='draft')         │  │
                          │  └───────┬───────┘   ask()/diff   │  + review_state + provenance      │  │
                          │          │ Normalizer (url/pdf/…) └──────────────┬───────────────────┘  │
                          │          ▼                                       │ Publish (human)        │
                          └──────────┼───────────────────────────────────────┼───────────────────────┘
                                     │ blob bytes                            │ eval gate (price=1.0,
                                     ▼                                       ▼  asset≥0.9) → atomic swap
                              ┌─────────────┐                        ┌────────────────┐
                              │ blob store  │                        │ LIVE SNAPSHOT  │ ──► the brain
                              │ (existing)  │                        │ (ai_*, publ.)  │     (8-ai-assistant)
                              └─────────────┘                        └────────────────┘
```

- **Reuses:** the blob store (`internal/blob`), the SSE hub (`internal/realtime`), the LLM client
  (shared with the brain), the queue (`internal/queue`) for slow intake jobs, the envelope/error
  codes (`7-api-contracts.md`).
- **Adds:** `internal/assistant/builder` (the agent + tools + normalizer), the draft-side tables
  (`11.1`), the builder API group (`11.2`), and two UI surfaces (`11.4`).

---

## 4. The config blocks — the 5 structured blocks the user named

The brief names five blocks: **identity, goal, quality standards, support behavior, topics.** Four are
prose config on the Snapshot; the fifth (topics) is the KB body. Mapping:

| Brief block | Stored on `ai_snapshots` | Prompt slot (`8-ai-assistant.md`) | Builder tool |
|---|---|---|---|
| **Identity** — who the assistant is (persona, tone) | `persona` | `[B] IDENTITY` | `set_block("identity", …)` |
| **Goal** — what it must achieve (mission, "good" =) | `mission` | `[B] IDENTITY` (mission line) | `set_block("goal", …)` |
| **Quality standards** — must / must-not, accuracy bar | `guardrails` | `[C] GUARDRAILS` | `set_block("quality", …)` |
| **Support behavior** — how it handles support/sales: escalation, next-step, warmth, when to attach media | **`support_policy`** *(new, nullable col)* | `[C] GUARDRAILS` (appended) | `set_block("support", …)` |
| **Topics** — the knowledge | `ai_topics[]` + `ai_assets[]` | `[D] KNOWLEDGE` + `[E] MEDIA` | `upsert_topic` / `attach_asset` |

> **The one brain touch.** `support_policy` is a single nullable column added to `ai_snapshots`, and
> `BuildSystem(snapshot)` concatenates it into block `[C]` after `guardrails` (and before
> `language_policy`). That is the *entire* runtime change — additive, ~3 lines, no logic change. The
> existing `language_policy` stays code/default-managed (a sub-rule of support), so the operator sees a
> clean four-block model: **Identity · Goal · Quality · Support.** Everything else the builder writes is
> a topic, an asset, or a price.

### A topic is a container of media — each asset has its own description (the core model)

Unchanged from `8-ai-assistant.md` / `10` — restated because it is the heart of "topic = text + several
media, each with its own description":

```text
topic: tariffs   body_md: "4 tariffs … {{price.start}} / {{price.growth}} …"   (price TOKENS, never digits)
  ├─ asset tariffs_overview (image)  "All 4 tariffs on one card — for a general 'what are your prices?'."
  ├─ asset tariffs_growth   (image)  "The Рост card — when the customer is focused on the Рост plan."
  ├─ asset tariffs_compare  (pdf)    "Full side-by-side PDF — when they ask to compare in detail."
  ├─ asset tariffs_link     (link)   "Public pricing page — when they want to read it themselves."
  └─ asset tariffs_explainer(video)  "90-sec walkthrough — when they prefer watching over reading."
```

The brain sees the topic body **and** every asset's description and picks the right asset(s) by `ref`
(max 3) for the moment. **Each asset's description is its selection cue** — that's why every asset needs
its own, and why the builder's per-file job is *"store the bytes + write that one sentence"* (auto, or
by asking). A **link** is just an asset whose "bytes" are a URL (`asset_kind='link'`, `asset_url=<url>`)
— infographics/videos/PDFs/links are all assets; only the `asset_kind` differs.

---

## 5. The headline feature — git-like change diffs (Changesets)

The brief calls out *"show git-like change diffs before applying important updates."* This is a
**first-class model**, not a UI afterthought.

### The model

A **Changeset** is a named, atomic bundle of typed **change-ops** against the draft Snapshot — the KB's
equivalent of a git commit (preview = the diff; apply = the commit).

```text
CHANGESET  "Add Рост tariff card + bump price"   state: proposed → applied | discarded
 ├─ op  update  block:quality        before:"…"            after:"…"            (field-level diff)
 ├─ op  create  topic:tariffs                               after:{slug,body_md,…}
 ├─ op  update  price:price.growth    before:"25 000 ₸"     after:"28 000 ₸"     (⚠ price → needs confirm)
 └─ op  create  asset:tariffs_growth                        after:{kind:image,description:…,url:…}
```

- **op** ∈ `create | update | delete`; **target_kind** ∈ `block | topic | asset | price`.
- Each op stores `before_json` / `after_json` → the UI renders a **field-level red/green diff** exactly
  like a git hunk (`11.4`).
- **Apply** is one DB transaction: mutate the draft rows, stamp `provenance`, mark the changeset
  `applied`. **Discard** drops it untouched. Apply is **idempotent** (a conditional update keyed on the
  changeset state → `CONFLICT` if already applied).

### When a change needs a changeset vs. applies directly

Not every edit deserves a diff card — that would be noise. The split:

| Change | Path | Why |
|---|---|---|
| Add a topic, attach an asset, write a description, add keywords | **direct apply**, row marked `proposed` (accept/deny later in the editor) | low blast-radius, easily reverted |
| Edit a **config block** (identity/goal/quality/support) | **changeset** (diff required) | changes *every* future reply |
| **Price/limit** create or change | **changeset** + a `confirm_price` request | shippable harm if wrong |
| **Delete** anything, or **merge** topics/assets | **changeset** (diff required) | destructive / structural |
| A **bulk** intake (a PDF that becomes 6 topics) | **one changeset** grouping all ops | review the batch as a unit, accept/reject per op |

So "important update" = config block, price, delete/merge, or a bulk batch → **always a diff first**.
Everything small streams in as `proposed` rows the editor shows with accept/deny. This keeps the chat
fast *and* makes the consequential edits deliberate. (DDL in `11.1`, UX in `11.4`.)

### Why this over per-row `review_state` alone

`10` proposed a per-row `review_state` (proposed/approved/rejected). We **keep** that for the
low-stakes stream, and **add** changesets for the high-stakes, multi-row, or destructive edits.
Changesets give three things `review_state` can't: **atomicity** (apply 6 ops or none), **a real
before/after diff** (not just a "new row" badge), and **a reviewable unit** (the operator approves an
intent — "add the Рост tariff" — not 4 disconnected rows). They compose: applying a changeset *creates*
`proposed` rows.

---

## 6. The interaction primitives (chat-driven, human-in-the-loop)

Two primitives carry the whole "configure by chat" experience. Both are **structured tool calls the
agent makes**, rendered by the UI, whose human answer is the tool result that lets the agent continue.

- **Request (popup)** — *"I need a human decision."* `describe_media`, `confirm_price`,
  `approve_topic/asset`, `resolve_duplicate`, `choose_topic`, `comment`. Renders inline in the chat
  **and** as a review-queue badge on the editor. Unresolved requests **block publish**. (`11.2` table.)
- **Changeset (diff card)** — *"I'm about to make an important change; approve the diff."* Renders as
  an expandable git-style diff with **Apply / Discard**. (§5, `11.4`.)

Everything else — adding a topic, writing a description from a note — just **streams in as `proposed`
rows** with a one-line "added topic *tariffs*" chat acknowledgement and a provenance tag.

---

## 7. The builder loop (one turn)

```text
1. INGEST     operator sends text / note / link / file(s)               → ai_builder_messages (+ attachments)
2. NORMALIZE  drive each input to TEXT (url→extract, pdf→text, media→describe_media popup)   (11.3)
3. PLAN       agent decides: which block? which topic? new or append? price? duplicate?
4. ACT        emit tools: set_block · upsert_topic · attach_asset · propose_price · merge_topics
                 - low-stakes  → direct apply, rows = 'proposed'
                 - high-stakes → open_changeset + add ops + request_review  (diff card)
                 - unsure      → ask() → a request popup (no row yet)
5. REPLY      a short chat turn: what it did + what it still needs ("I added 2 topics; please describe
                 the 2nd image and confirm the Рост price").
```

The agent **cannot block** waiting on a human, so unknowns become requests/diffs and the turn ends;
resolving one (via the realtime channel) **nudges the agent to continue**. (`11.3` has the full prompt,
tool catalog, and proactive policy.)

---

## 8. Safety model (why aggressive automation is safe here)

```text
auto-extracted row  ──gate 1──►  draft only      (snapshot_state='draft'; never read by the brain)
                    ──gate 2──►  human review     (changesets + accept/deny + describe/confirm popups)
                    ──gate 3──►  publish gate      (deterministic: price-safety=1.0, asset-precision≥0.9)
                    ──gate 4──►  suggest-and-approve runtime (every customer reply is human-approved)
```

A builder mistake must survive **four** independent gates to reach a customer. Therefore the builder can
be aggressive about *proposing*; the cost of a wrong proposal is a rejected diff, never a bad send.
Prices get an extra lock (token + `confirm_price` + the hard 1.0 publish gate).

---

## 9. Reading guide — the `11.*` set

- **`11-knowledge-base-builder.md`** (this) — the option: decisions, architecture, block model, the
  changeset concept, the loop, the safety model.
- **`11.1-kb-schema.md`** — the **additive DDL**: migrate the deferred `ai_snapshots/topics/assets/
  prices` tables, add `support_policy`, add draft-side `review_state`/`provenance`, and the new
  `ai_builder_sessions / messages / attachments / requests` + `ai_changesets / ai_change_ops`.
- **`11.2-kb-api.md`** — the builder API group under `/xchats/api/v1/assistant/builder/*`, the editor
  CRUD, publish/rollback, the request & changeset endpoints, and the `builder.*` SSE events — all in the
  `{payload, errcode}` envelope.
- **`11.3-kb-builder-agent.md`** — the agent: the system prompt, the **tool catalog**, the **proactive
  policy** (ask-don't-guess, suggest improvements), the **normalization pipeline** per source type, and
  dedup/merge + price discipline.
- **`11.4-kb-ui.md`** — the two surfaces (builder chat + KB editor), the **diff card** and **popup**
  UX, provenance display, and the publish flow — image-prompt-ready like `5-ui-pages.md`.
- **`11.5-kb-build-plan.md`** — the phased build (11A–11E), file-level work breakdown, the isolated
  test strategy (most of it needs **no live LLM**), DoD per phase, and risks.

> **Scope:** this is the **deferred CMS** (DoD Phase 4B). v1 still seeds the snapshot from
> `0002_seed.sql`. Nothing here changes the v1 vertical slice; it is the next build after the inbox +
> draft loop are proven. Phase **11A** (migrate the snapshot tables + the editor read/CRUD) is the
> foundation everything else rides on and is the recommended first step.
