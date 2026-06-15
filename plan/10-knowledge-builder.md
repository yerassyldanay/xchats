# Knowledge Builder — the automatic KB authoring experience

How an operator turns a pile of material (URLs, images, video, PDFs, plain descriptions) into a
publishable knowledge base **without filling forms** — the assistant builds it, the human reviews and
corrects. This is the authoring front-end to the brain's `xchats.ai_*` tables (runtime in
`8-ai-assistant.md`). It is the **deferred CMS** (designed here; built Phase 4B — see
`0.1-definition-of-done.md`), not v1.

> **This doc is the conceptual UX.** The **concrete, buildable spec** — additive DDL, API + SSE, the
> chat builder agent + tools, the git-like change-diff (changeset) model, the UI, and the phased build
> plan — lives in the **`11-knowledge-base-builder.md`** set (`11`, `11.1`–`11.5`). Read `11` for *how*.

## Why automatic is safe here (the unlock)

An auto-generated KB row cannot reach a customer without passing **three independent gates**, so the
builder can be aggressive:

1. **The builder writes only a *draft* Snapshot** (`ai_snapshots.snapshot_state='draft'`) — never live.
2. **Human review + publish.** Publish runs the deterministic gate (price-safety = 1.0, asset precision
   ≥ 0.9 — `8-ai-assistant.md` → Evals) and is the only path draft → live.
3. **Customer-facing is suggest-and-approve.** Every reply is human-approved before send.

A builder mistake must survive an editor, a publish gate, **and** a draft-reviewer to do harm.

---

## What it builds — the KB blocks

A Snapshot is a few **config blocks** + a list of **topics** (each a container of text + media) + a
**price book**. The builder produces all of it; the editor page exposes all of it.

```
SNAPSHOT (draft → published)
├─ Identity     who the assistant is — persona, tone            (ai_snapshots.persona)
├─ Goal         what it must achieve — mission, what "good" is  (ai_snapshots.mission)
├─ Guardrails   quality & support rules — must / must-not       (ai_snapshots.guardrails, language_policy)
├─ Topics[]     the knowledge — each a CONTAINER:
│   ├─ body_md      the answer text (price TOKENS, never digits) (ai_topics)
│   └─ media[]      attached assets, EACH WITH ITS OWN description (ai_assets, topic_slug → topic)
└─ Prices       the single source of numbers, as tokens          (ai_prices)
```

**Identity / Goal / Guardrails** are the "what must this assistant achieve" blocks the operator writes
in plain language (e.g. Goal = "qualify the lead and present the right tariff"; Guardrails = "always
warm, never pushy, escalate on anything off-KB"). They become prompt blocks `[B]/[C]`.

### A topic is a container of media — each asset has its own description

This is the core model. A single topic groups everything about one subject, and holds **several media
assets**, because *which one to send depends on the moment*:

```
topic: tariffs   body_md: "4 tariffs … {{price.start}} / {{price.growth}} …"
  ├─ asset tariffs_overview (image)  "All 4 tariffs on one card — for a general 'what are your prices' question."
  ├─ asset tariffs_growth   (image)  "The Рост card — when the customer is focused on the Рост plan."
  ├─ asset tariffs_compare  (pdf)    "Full side-by-side PDF — when they ask to compare in detail."
  └─ asset tariffs_explainer(video)  "90-sec walkthrough — when they prefer watching over reading."
```

At answer time the brain sees the topic body **and** every asset's description, and picks the right
asset(s) by `ref` (max 3) — a video for one customer, an image for another. **Each asset's description
is its selection cue** (what it shows + when to send it); that's why every asset needs its own, and why
the builder's job per file is "store the bytes + write that one sentence" (auto, or by asking — below).

---

## Two surfaces over one draft

The chat and the editor edit the **same** draft Snapshot — no sync problem, just two speeds.

```
   material (URLs, media, text)
            │
            ▼
   ┌──────────────────┐   creates / updates    ┌────────────────────────────┐
   │  BUILDER CHAT    │──────  ai_* rows  ─────▶│  draft Snapshot (ai_*)      │
   │  bulk intake     │                         └────────────────────────────┘
   │  + proactive Qs  │◀── popups (requests) ──▶            ▲
   └──────────────────┘                                     │ CRUD + accept/deny/edit
                                                            │
                                          ┌────────────────────────────────────┐
                                          │  KB EDITOR PAGE                      │
                                          │  topics · media · identity/goal ·   │
                                          │  prices · provenance · Publish      │
                                          └────────────────────────────────────┘
```

- **Builder chat** — the fast path. Drop material, the assistant creates many rows and asks when unsure.
- **KB editor page** — the precise path. A structured view of the draft: the config blocks, the topic
  list (expand a topic → its body + its media gallery, each asset with its description), the price book.
  Every row is editable and shows **provenance** ("created from: chat msg #12 / source: <url>").
  A **Publish** button runs the gate and swaps the snapshot live.

---

## The builder loop

Every input, whatever its type, is driven to **text**, then structured into the blocks above:

1. **Ingest** — accept a URL, file, or message.
2. **Normalize to text** — extract the content (table below).
3. **Structure** — create/append a topic, attach assets, propose price tokens, suggest identity/goal
   edits. Each generated row is tagged `proposed` with its provenance.
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
input; the popup is that tool's UI; the operator's answer is the tool result, which mutates the draft
and lets the builder continue.

A request = `{ id, type, prompt, context (thumbnail/topic/detected value), target draft row, state }`,
`state ∈ {pending, resolved, dismissed}`.

| Request type | Popup asks | Resolves into |
|---|---|---|
| `describe_media` | text — "Describe what this shows and when to send it." | `ai_assets.description` (+ kind/topic) |
| `confirm_price` | accept / edit — "Map '25 000 ₸' → `{{price.growth}}`?" | a `ai_prices` token + value (never a digit in text) |
| `approve_topic` / `approve_asset` | **accept · deny · edit** | flips the row's `review_state` |
| `resolve_duplicate` | merge · keep both | dedup / merge of topics or assets |
| `choose_topic` | pick / create — "Which topic does this image belong to?" | the asset's `topic_slug` |
| `comment` | free text the builder reads next turn | a note steering the next build step |

**Where they appear:** inline as cards in the builder chat **and** as a **review-queue badge** on the
editor page; clicking either opens the modal. **Unresolved `pending` requests are surfaced at publish**
(e.g. an asset with no description, or an unconfirmed price blocks publish — they'd fail the gate
anyway). Resolving a request updates the draft row, marks the request `resolved`, and (via the realtime
channel) nudges the builder to proceed.

**Row review states** make accept/deny first-class: every auto-created row starts `proposed`; the
operator can **accept** (`approved`), **edit** (`approved`, edited), or **deny** (`rejected`). Publish
includes approved rows only; rejected rows are kept for provenance but excluded.

> Storage: the draft `ai_topics` / `ai_assets` / `ai_prices` rows gain two lightweight, **draft-side**
> fields — `review_state` and `provenance` (source ref) — and requests live in a small `ai_builder_requests`
> queue keyed to the draft snapshot. All additive; the **live** runtime tables and the brain are
> untouched. Exact DDL folds into `9-database-schema.md` when this phase lands.

---

## Prices — the one thing never auto-written

Numbers are the only place an extraction error is shippable harm (a wrong/stale price). So the builder
**never bakes a digit into a topic body**: on detecting a price/limit it writes a `{{token}}` and raises
a `confirm_price` popup; the real number lands in the price book **only** by human confirmation. This is
the same token discipline the runtime enforces (`8-ai-assistant.md` → Prices).

## Provenance & dedup

- **Provenance** — every generated row records what produced it (chat message, URL, file), shown in the
  editor so a reviewer can check a summary against its source in one click.
- **Dedup / merge** — re-feeding the same URL or an overlapping topic raises `resolve_duplicate` rather
  than silently creating a twin; topics are containers, so a new tariff image **appends** to the
  existing `tariffs` topic instead of forking it.

## Publish → live

Publish validates the draft (every asset description present, every price token resolves, no dangling
media URLs — the deterministic gate), then atomically swaps it live and reloads the brain's snapshot.
Rollback re-publishes a prior version. From there the brain answers from it, suggest-only.

---

## Build notes / sequencing

- **Reuses, doesn't change, the runtime.** The builder writes the same `ai_*` blocks the brain already
  reads; the `topic_slug` container model and per-asset `description` already exist (the xpayment seed's
  `tariffs` topic with its four cards is the worked example). No brain changes.
- **First build (Phase 4B):** the builder chat + editor page + the request/popup primitive, with
  **text/URL auto-structuring** and **`describe_media` popups** for media (operator-described). This is
  fully chat-driven without any vision/transcription dependency.
- **Later:** auto-vision/transcription/scraping fill descriptions automatically (drop in behind the same
  popups — they just pre-fill), and `pgvector` retrieval if the KB outgrows the prompt
  (`8-ai-assistant.md` → Scaling).
