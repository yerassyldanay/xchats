# Knowledge Builder — the automatic KB authoring experience

How an operator turns a pile of material (URLs, images, video, PDFs, plain descriptions) into a
publishable knowledge base **without filling forms** — the assistant builds it, the human reviews and
corrects. This is the authoring front-end to the brain's `xchats.ai_*` tables (runtime in
`8-ai-assistant.md`). It is the **deferred CMS** (designed here; built Phase 4B — see
`0.1-definition-of-done.md`), not v1.

> **This doc is the conceptual UX.** For the **bird's-eye view** of the whole AI side — the three
> components (brain · knowledge base · playground), the main design solutions, and their advantages &
> limitations — see **`11-design-overview.md`** (start there). The data model lives in
> `9-database-schema.md`; the brain in `8-ai-assistant.md`.

## Why automatic is safe here (the unlock)

An auto-generated KB row cannot reach a customer without passing **three independent gates**, so the
builder can be aggressive. There is **one living KB per org** — the brain reads the **live** rows; the
builder's new rows are held out as **pending** until a human lets them in:

1. **The builder writes rows as *pending*** (`drafted_at` set) — never live. A pending row shows in the
   playground/editor but is excluded from the prompt.
2. **Human approval.** Approving a row (or all) runs the deterministic gate (fact-safety = 1.0, asset
   precision ≥ 0.9 — `8-ai-assistant.md` → Evals) and is the **only** path pending → live (it clears
   `drafted_at`).
3. **Customer-facing is suggest-and-approve.** Every reply is human-approved before send.

A builder mistake must survive an editor, the approve gate, **and** a draft-reviewer to do harm.

---

## What it builds — the KB blocks

The KB is a few **config blocks** + a list of **topics** (each a container of text + media) +
**products** + **tariffs** + **contacts** (the typed **fact** tables — exact numbers as columns). The builder
produces all of it; the editor page exposes all of it. It is **one living KB** — rows are live or
pending (`drafted_at`), not draft vs. published copies.

```
KB  (one living KB per org — rows are live or pending via drafted_at)
├─ Identity     who the assistant is — persona, tone            (ai_snapshots.persona)
├─ Goal         what it must achieve — mission, what "good" is  (ai_snapshots.mission)
├─ Guardrails   quality & support rules — must / must-not       (ai_snapshots.guardrails, language_policy)
├─ Topics[]     the Knowledge lane — each a CONTAINER:
│   ├─ body_md      the answer prose (fact TOKENS, never digits) (ai_topics)
│   └─ media[]      OWNED assets, EACH WITH ITS OWN description  (ai_assets, owner_kind=topic)
├─ Products[]   the Facts lane — sellable items                 (ai_products: name, PRICE, description, category, data; one row per lang)
│   └─ media[]          OWNED assets                            (ai_assets, owner_kind=product)
├─ Tariffs[]    the Facts lane — pricing PLANS                  (ai_tariffs: name, PRICE, LIMIT_TEXT, FEE, pricing_type, advantages/disadvantages; one row per lang)
│   └─ media[]           OWNED assets                            (ai_assets, owner_kind=tariff)
└─ Contacts     the Facts lane — org support scalars            (ai_contacts: whatsapp, email, address, legal, callback_time; one row per lang)

   exact numbers are TYPED COLUMNS on the fact tables; quoted in any text only as {{table.slug.field}}
```

**Products, tariffs and contacts.** An `ai_products` row is a **sellable item**; its **price is a typed
column** (verbatim, one row per language), alongside descriptive `name`/`description`/`category`/`data
jsonb`; its photos are **OWNED `ai_assets` rows**. An `ai_tariffs` row is a **pricing plan** (`pricing_type`
∈ fixed/percentage/tiered, `advantages`/`disadvantages` text) whose quotable numbers (`price`,
`limit_text`, `fee`) are **typed columns**. An `ai_contacts` row holds org support scalars (`whatsapp`,
`email`, `address`, `legal`, `callback_time`) as typed columns. All three are the **Facts lane** — exact,
verbatim, quoted only as `{{table.slug.field}}` — and **independent** (no links between them).

**Identity / Goal / Guardrails** are the "what must this assistant achieve" blocks the operator writes
in plain language (e.g. Goal = "qualify the lead and present the right tariff"; Guardrails = "always
warm, never pushy, escalate on anything off-KB"). They become prompt blocks `[B]/[C]`.

### A topic is a container of media — each asset has its own description

This is the core model, and it is **polymorphic**: **media** attaches to **any** entity — a topic,
a product, or a tariff — via one shared `(owner_kind, owner_ref)` pair on `ai_assets`. (Exact **facts** are
typed columns on the entity, not polymorphic.) So "add media to a topic / product / tariff" is **one mechanism**. A single topic groups everything
about one subject, and holds **several media assets**, because *which one to send depends on the moment*:

```
topic: tariffs   body_md: "4 tariffs … {{tariff.start.price}} / {{tariff.growth.price}} …"
  ├─ asset tariffs_overview (image)  "All 4 tariffs on one card — for a general 'what are your prices' question."
  ├─ asset tariffs_growth   (image)  "The Рост card — when the customer is focused on the Рост plan."
  ├─ asset tariffs_compare  (pdf)    "Full side-by-side PDF — when they ask to compare in detail."
  └─ asset tariffs_explainer(video)  "90-sec walkthrough — when they prefer watching over reading."
```

At answer time the brain sees the topic body **and** every asset's description, and picks the right
asset(s) by `ref` (max 3) — a video for one customer, an image for another. **Each asset's description
is its selection cue** (what it shows + when to send it); that's why every asset needs its own, and why
the builder's job per file is "store the bytes + write that one sentence" (auto, or by asking — below).
The same mechanism applies one level out: **a product can own its photos** exactly this way — each photo
is an `ai_assets` row with `owner_kind=product` and its own description.

---

## Two surfaces over one living KB

The chat and the editor edit the **same** living KB — no sync problem, just two speeds. New rows land
**pending** (`drafted_at` set), held out of the prompt until approved.

```
   material (URLs, media, text)
            │
            ▼
   ┌──────────────────┐   creates / updates    ┌────────────────────────────┐
   │  BUILDER CHAT    │──────  ai_* rows  ─────▶│  LIVING KB (ai_*)           │
   │  bulk intake     │       (as pending)      │  live rows + pending rows   │
   │  + proactive Qs  │◀── popups (requests) ──▶└────────────────────────────┘
   └──────────────────┘                                     ▲
                                                            │ CRUD + approve/delete/edit
                                                            │
                                          ┌────────────────────────────────────┐
                                          │  KB EDITOR PAGE                      │
                                          │  topics · products · tariffs ·       │
                                          │  media · identity/goal · values ·    │
                                          │  provenance · Approve                │
                                          └────────────────────────────────────┘
```

- **Builder chat** — the fast path. Drop material, the assistant creates many (pending) rows and asks
  when unsure.
- **KB editor page** — the precise path. A structured view of the KB: the config blocks, the topic
  list (expand a topic → its body + its media gallery, each asset with its description), products,
  tariffs, and the values. Pending rows are visibly flagged. Every row is editable and shows
  **provenance** ("created from: chat msg #12 / source: <url>"). An **Approve** button runs the gate and
  clears `drafted_at` on the selected rows (or all), letting the brain read them.

---

## The builder loop

Every input, whatever its type, is driven to **text**, then structured into the blocks above:

1. **Ingest** — accept a URL, file, or message.
2. **Normalize to text** — extract the content (table below).
3. **Structure** — create/append a topic, attach assets to any entity, register products, tariffs &
   contacts, confirm fact values (typed columns), suggest identity/goal edits. Each generated row is written **pending**
   (`drafted_at` set) with its provenance.
4. **Ask when unsure** — anything ambiguous or unextractable becomes a **request** (a popup), not a
   guess. The builder is **proactive**: it confirms prices, proposes merges, and requests missing
   descriptions before the row is considered done.

| Input | Normalize step | Fallback if we can't / it's too costly |
|---|---|---|
| **Plain text / description** | use directly → topic body | — |
| **URL** | fetch → extract main content → summarize | ask the operator to paste the key points |
| **Image** | (later) vision-caption; else store bytes | **`describe_media` popup** — "what does this show / when to send it?" |
| **Video / audio** | (later) transcribe → summarize | **`describe_media` popup** + store/keep URL |
| **PDF / doc** | extract text → split into topics | ask which sections matter |

> Per the scope decision: auto-extraction of media is **optional and phased**. When we can't caption a
> file (or it's expensive), the builder simply **asks the operator to describe it** via a popup — so the
> experience is fully chat-driven from day one, and auto-vision/transcription drops in later behind the
> same UX without changing it.

---

## The interaction primitive — popups (Builder Requests)

The builder can't block waiting on a human, so it emits structured **requests** that the UI renders as
**popups/modals**. This is a human-in-the-loop tool call: the builder "calls a tool" that needs human
input; the popup is that tool's UI; the operator's answer is the tool result, which mutates the pending
row and lets the builder continue.

A request = `{ id, type, prompt, context (thumbnail/topic/detected value), target row, state }`,
`state ∈ {pending, resolved, dismissed}`.

| Request type | Popup asks | Resolves into |
|---|---|---|
| `describe_media` | text — "Describe what this shows and when to send it." | `ai_assets.description` (+ kind, owner) |
| `confirm_fact` | accept / edit — "Set `tariff.growth.price` = '25 000 ₸/мес'?" | writes the **typed column** on `ai_tariffs`/`ai_products`/`ai_contacts` for that language (never a digit in prose) |
| `resolve_duplicate` | merge · keep both | dedup / merge of topics, products, tariffs, or assets |
| `choose_topic` | pick / create — "Which entity does this belong to?" | the asset's owner (`owner_kind/owner_ref` → topic, product, or tariff) |
| `comment` | free text the builder reads next turn | a note steering the next build step |

**Where they appear:** inline as cards in the builder chat **and** as a **review-queue badge** on the
editor page; clicking either opens the modal. **Unresolved `pending` requests are surfaced at approve**
(e.g. an asset with no description, or an unconfirmed fact blocks approving its row — it'd fail the gate
anyway). Resolving a request updates the pending row, marks the request `resolved`, and (via the realtime
channel) nudges the builder to proceed.

**Approve / delete** is the row-level control, expressed through `drafted_at`: every auto-created row
starts **pending** (`drafted_at` set). The operator **approves** — per-row or all — which clears
`drafted_at` after running the deterministic gate, the only path pending → live; or **deletes** (reject =
delete the pending row). Editing a pending row leaves it pending until approved.

> Storage: a pending `ai_topics` / `ai_products` / `ai_tariffs` / `ai_contacts` / `ai_assets` row carries
> `drafted_at` (NULL = live, set = pending) and `provenance` (source ref) — there is **no** `review_state`
> enum; requests live in a small `ai_builder_requests` queue. All additive on the **one living KB**; the
> brain simply filters `drafted_at IS NULL`. Exact DDL folds into `9-database-schema.md` when this phase lands.

---

## Facts — the one thing never auto-written

Numbers are the only place an extraction error is shippable harm (a wrong/stale price). This applies to
**every** fact — product prices, tariff rates, limits, contacts — not just topic prices. Exact facts are
**typed columns** on the fact tables (`ai_tariffs` / `ai_products` / `ai_contacts`), stored verbatim; the
only way to put one into any text is a `{{table.slug.field}}` token, resolved from that column for the
reply's language. So the builder **never bakes a digit anywhere**: on detecting a price/rate/limit it
writes a `{{table.slug.field}}` token and raises a `confirm_fact` popup; the real number enters the typed
column **only** by human confirmation. This is the same token discipline the runtime enforces
(`8-ai-assistant.md` → Facts).

## Provenance & dedup

- **Provenance** — every generated row records what produced it (chat message, URL, file), shown in the
  editor so a reviewer can check a summary against its source in one click.
- **Dedup / merge** — re-feeding the same URL or an overlapping entity raises `resolve_duplicate` rather
  than silently creating a twin; topics (and products/tariffs) are containers, so a new tariff image
  **appends** to the existing `tariffs` topic instead of forking it.

## Approve → live

There is no atomic snapshot swap and no rollback — the KB is one living set of rows. **Approving**
validates the pending rows being approved (every asset has a description, every `{{table.slug.field}}`
resolves to a column value per required language, no dangling media, no literal currency in topic bodies —
the deterministic gate), then
**clears `drafted_at`** on those rows. The brain reloads its live view (`drafted_at IS NULL`) and answers
from it, suggest-only.

---

## Build notes / sequencing

- **Reuses, doesn't change, the runtime.** The builder writes the same `ai_*` blocks the brain already
  reads; the `(owner_kind, owner_ref)` container model and per-asset `description` already exist (the
  xpayment seed's `tariffs` topic with its four cards is the worked example). The brain just filters
  `drafted_at IS NULL`. No brain changes.
- **First build (Phase 4B):** the builder chat + editor page + the request/popup primitive, with
  **text/URL auto-structuring** and **`describe_media` popups** for media (operator-described). This is
  fully chat-driven without any vision/transcription dependency.
- **Later:** auto-vision/transcription/scraping fill descriptions automatically (drop in behind the same
  popups — they just pre-fill), and `pgvector` retrieval if the KB outgrows the prompt
  (`8-ai-assistant.md` → Scaling).
