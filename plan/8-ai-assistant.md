# AI Assistant (the Brain)

> ⚠️ **Partially superseded by [`14-draft-staging-and-retrieval.md`](14-draft-staging-and-retrieval.md).**
> The grounding-judge pipeline step is **deferred from v1**; the `[F]` facts block is single-language in
> v1 (cache-stable); topic bodies carry **no fact tokens**; knowledge retrieval per 14 Decision 5.
> Updated lazily; 14 wins on conflict.

The core of xchats. Given a customer conversation it produces **reviewed reply drafts** — text
(and, later, attached media) in the customer's language, grounded **only** in a curated knowledge
base. It never sends by itself (suggest-and-approve).

**Already implemented** and reused as-is — vendored as a git submodule at
`examples/repos/xpayment-crm/` (entry: `IMPLEMENTATION.md`; core in
`internal/usecase/assistant/{brain,prompt,ports}.go`, `internal/domain/{catalog,draft}.go`,
`internal/infrastructure/llm/`). xchats **ports** it: Chatwoot reads → xchats Postgres reads, draft
→ an `xchats.ai_suggestions` row, config storage SQLite → Postgres. The logic is unchanged. Full port table
in the *Porting* section below.

## v1 adapter mode (the minimal slice)

Same ported logic, smaller surface:
- **Text-only drafts.** `asset_refs` may be emitted but are **ignored/logged, not rendered/sent**.
- **One active seeded Snapshot**, loaded on boot from `0002_seed.sql`. **No authoring UI, no
  publish/rollback, no Playground** (the CMS — including the chat-authoring flow below — is deferred).
- **On-demand trigger:** a draft is produced when the user presses **"Suggest reply"**, not on every
  inbound (controls LLM spend/latency). Auto-draft-on-inbound is a fast-follow.
- **No auto-send.** A human approves every send.

Everything past this (media rendering, the authoring CMS, auto-send) is designed here but staged to
v2+ (see `0.1-definition-of-done.md` Phases 4B–4D).

## Core idea in one line

A **stateless** call — `HandleMessage(window, snapshot) -> Draft`: build a cache-stable
system prompt from the published **Snapshot**, add the recent **message window** (last ~M messages),
force the model to return strict `emit_draft` JSON, then **post-process** it
(`escalate → render facts → number check → grounding judge → media refs → status`) into a final Draft.
**v1 sends no contact profile** — the model infers everything from the window (see *Memory*).

---

## The prompt — cached prefix `[A]–[F]` + dynamic suffix

The prefix is rebuilt only on publish (`BuildSystem(snapshot)`); mark the cache breakpoint after `[F]`.

```
┌──────────────────── CACHED PREFIX (stable across messages) ────────────────────┐
│ [A] FRAME      code-owned, never editable: role · JSON output contract · hard rules │
│ [B] IDENTITY   persona (+ mission) from the snapshot config                          │
│ [C] GUARDRAILS guardrails (+ language policy) from the snapshot config              │
│ [D] KNOWLEDGE  every topic: `# topic: <slug> (<lang>)` + keywords + body (tokens intact) │
│ [E] MEDIA      the whole catalog as `ref | kind | topic | description` — the pick menu  │
│ [F] FACTS      the Facts lane: per fact `token | label | value(reply-lang)` — emit the token │
├──────────────────────────  ⟵ cache breakpoint  ───────────────────────────────┤
│     DYNAMIC    WINDOW (~15 msgs, oldest first) · CURRENT MESSAGE   (no profile in v1)     │
└────────────────────────────────────────────────────────────────────────────────┘
```

**`[A]` the hard rules (never editable):** answer **only** from the KB `[D]`, else **escalate** —
never guess; facts (prices, limits, times, contacts) are **tokens** (`{{table.slug.field}}`, e.g. `{{tariff.growth.price}}`), never digits — code fills them after;
attach media **only** by refs that exist in `[E]`, **max 3**; reply in the **customer's language**
(KK+RU mix → Russian); **~120 words**, warm, **one** next step; never handle passwords; **must call
`emit_draft`**.

**Conversation flow** is not a state machine — the persona + the model's `suggested_status.stage`
(`greeting → qualifying → presenting → closing`, stored on the chat) drive it, with the window giving
continuity. Each turn: **ask** one concise question when a qualifying fact is missing; **answer** from
the KB (+ optional media, + fact tokens) when it's covered; **escalate** when it isn't.

---

## The output contract & post-processing

The model returns exactly this (structured `emit_draft`):

```jsonc
{
  "reply_text": "uses {{table.slug.field}} fact tokens, never numerals",
  "reply_language": "ru | kk",
  "asset_refs": ["catalog refs only, max 3; [] if none"],
  "suggested_callback": { "due_at": "…|null", "note": "…" } | null,
  "suggested_status": { "stage": "qualifying" } | null,
  "confidence": 0.0, "escalate": false, "escalation_reason": ""
}
```

Post-processing, **in order** (the safety spine of decision record `13`, Decision 6):
1. **escalate gate** — if true (KB gap / low confidence), flag for a human; ship only a short holding reply.
2. **render facts** — replace every `{{table.slug.field}}` token with the typed column value in the reply
   language (fallback reply-language → org-default → `'*'`); an unresolved token → `PricingError` (post
   "check facts manually", never a half-rendered value).
3. **number check** — every currency-/unit-adjacent number must trace to a value injected in step 2; a
   number from nowhere → **escalate** (deterministic backstop; also catches any inline digit).
4. **grounding judge** — a cheap LLM checks every non-numeric claim is supported by the topics; unsupported
   → **escalate**. Biased to escalate; never auto-approves.
5. **media refs** — `ResolveAssets`: keep only refs that exist, **cap 3**; unknown refs dropped + logged →
   no hallucinated files. Result is media URLs.
6. **status** — apply `suggested_status.stage` (+ any `suggested_callback`); then **human review** is the
   final gate (drafts are never auto-sent).

**Text / media / both** is the model's choice via `asset_refs` (text-only = `[]`; media-only = minimal
text + refs; both = text is the caption). We only ever send **approved catalog assets by reference** —
never generated or arbitrary files.

**Incoming media:** the brain reasons over **text** and replies with **text + catalog media only**. An
inbound image's caption enters the window as text; audio/video/docs are stored and noted by type (a
later vision/transcription step can add a description to the window). It never synthesizes audio/images.

---

## The Knowledge Base — what the model answers from

A published **Snapshot** = config + topics + media catalog + prices. Edited, **versioned**, and
**published** (publish swaps the live snapshot atomically; rollback restores a prior version). Stored
normalized in `xchats.ai_snapshots / ai_topics / ai_assets / ai_tariffs / ai_products / ai_contacts` (see
`9-database-schema.md`). The brain loads it into one **immutable in-memory snapshot** (keyed per org)
and **never queries the DB on the hot path**: the cached prefix `[A]–[F]` is built **once** — on boot and
on publish, not per message. Per inbound message it reads only the small **dynamic suffix** (window +
profile). Updates never mutate the live snapshot in place; **publish builds a new one and atomically
swaps the pointer**, so a message handler never sees a half-updated KB.

**One principle resolves how text, images, and video are stored: the model only ever reasons over
*text*. It never sees an image or watches a video** — everything it "knows" about a piece of media is
the *text you wrote for it*. So content lives in three forms:

| Form | Stored as | Goes in | The model's use |
|---|---|---|---|
| **Text knowledge** (topics) | `ai_topics`: `slug`, `lang`, `keywords`, `body_md` | `[D]` | reads it; answers **only** from it |
| **Media** (image/video/doc/audio) | `ai_assets`: `ref`, `kind`, `topic_slug`, `description`, `url` | `[E]` | picks a `ref` (max 3); bytes attached at send |
| **Facts** (prices, limits, times, contacts) | typed columns on `ai_tariffs` / `ai_products` / `ai_contacts`, verbatim, language a row | `{{table.slug.field}}` tokens in text (`[F]`) | picks the right fact by label + value; emits the token, never the number |

### Media is a knowledge source — via a companion summary topic

We want the assistant to answer from what's *inside* a video/image, not just attach it. Since the model
reads only text, the rule is:

> **Every knowledge-bearing image/video has both (a) an `ai_assets` row for the bytes and (b) a
> companion `ai_topics` row whose `body_md` captures its content as text, linked by `topic_slug`.**

This needs **no schema change** — it leans on the existing `topic_slug` link. Two text fields, two
jobs, never conflated: the **topic body** is the *content the model answers from*; the **asset
description** is the one-sentence *selection-menu entry* ("step-by-step video for 'how do I add a
cashier'") the model picks on. Companion topics are **answer-shaped summaries (~80–150 words)**, not
verbatim transcripts — keeping the cached prefix lean and the system deterministic. (If verbatim recall
from long media is ever needed, that's the trigger to add pgvector retrieval — see *Scaling* below.)

### Facts (prices, limits, times, contacts) — the Facts lane; typed columns, not literals

`{{tariff.growth.price}}`, `{{tariff.growth.limit_text}}`, `{{product.nike_x.price}}`,
`{{contact.support.whatsapp}}` are referenced in topics/replies; each value is a **typed column** on a
typed fact table (`ai_tariffs` / `ai_products` / `ai_contacts`), stored **verbatim with units**; code
substitutes it **after** drafting, for the reply's language (**language is a row, not a column**). **Any**
factual number stays correct, centrally editable, and impossible for the model to invent — not just
prices. The token grammar is uniform: `{{table.slug.field}}` — table selects the fact table, slug the row,
field the column. The old generic `ai_values` bag is **removed** (a nearest-key lookup can return the
*wrong* tariff). (Prices are the canonical case and the one the publish gate hard-checks.)

### Authoring — a chat where the assistant builds the KB

The authoring model (deferred CMS, designed now): an operator opens a **builder chat**, drops material
(URLs, images, videos, PDFs, raw text), and the **builder assistant turns it into a draft Snapshot on
its own** — topics (each a container of media, every asset with its own send-description), typed facts
(prices/limits/contacts as columns), and identity/goal/guardrails edits — asking via **popups** when it
needs a description or a fact value confirmed. A **KB editor page** exposes everything it created for accept/deny/edit, and **publish** runs
the gate (below). Full UX — the blocks, the topic-as-media-container model, the popup/request primitive
— is **`10-knowledge-builder.md`**. It writes the same `ai_*` tables; nothing goes live unreviewed.

### Cold start — the load-bearing first task

The brain answers **only** from the KB, so an **empty KB makes every draft a useless escalation** — the
product does nothing on day one. The seed snapshot (`0002_seed.sql`) is carried into the Postgres `ai_*`
tables on first boot so the service "boots usable". The first real task is filling the KB to cover the
org's top questions (reuse the xpayment KB, or build the org's own via the builder chat) — gated by
real-question quality, not "a row was produced".

### Scaling — why curated-in-prompt, not vector RAG (for now)

The KB is small and hand-curated, so it fits **entirely in the prompt** → deterministic, cheap, no
retrieval errors; the model sees every topic and the whole catalog at once and selects. When it
outgrows the prompt (media-as-knowledge grows it faster), add **pgvector** retrieval (top-k per message)
behind the same context port, in the same Postgres, **without changing the brain**.

---

## Memory — the window only (v1)

| Horizon | Lives on | Scope | Role |
|---|---|---|---|
| **Window** | the chat | last ~M messages (~15) / 48h of *this* conversation | the brain's **only** memory in v1 |

**v1 decision: no contact profile is fed to the brain.** Each call sends only the recent message
window; the model infers who they are, what they want, and the stage from those messages. There is **no
`profile_patch`** and no long-term-memory merge. This keeps the call simple and stateless and avoids a
PII profile leaving the system.

The cost, stated plainly: a returning customer is **not "remembered"** across conversations — a new
conversation starts cold with only its own window. `wa_contacts.attributes` still exists for
**manual/CRM** use, but the brain neither reads nor writes it in v1. If long-term memory is wanted
later, reintroduce a profile (and an optional rolling summary) **behind the reader/writer ports**
without touching the core — the seam is deliberately kept.

---

## Providers & the data boundary

Provider-neutral: the brain speaks the OpenAI-compatible `chat/completions` API with a **forced
`emit_draft` tool**, so `openrouter | openai | gemini` (or any compatible/self-hosted model) differ only
by base URL, key, and model name — **switching providers is config, not code** (`LLM_*` env). The
response is parsed defensively; if a provider lacks tool calls, it falls back to parsing JSON from the
content.

**Compliance (decide before any real send):** each call ships the **last ~15 messages + the contact
profile** to the provider — customer personal data, **cross-border** for a KZ-facing product when the
base URL is foreign. `LLM_BASE_URL` is the lever: an **in-region / self-hosted** model is the compliant
default; a foreign provider requires consent + PII minimization + a DPA. A go-live decision, not a build
task — see `2-architecture.md` (LLM data boundary), `0.1-definition-of-done.md` Phase 4.

---

## Evals & the publish gate

Draft quality is a direct function of KB coverage, and a structurally-valid draft row can still be a
useless escalation — so quality is **measured**, not assumed. A **golden set** (20–40 real cases, mined
from real chats, checked into git): each has a `window` + `profile` + `expect` (`must_include` /
`must_exclude`, `asset_refs`, `reply_language`, `escalate`).

**Deterministic metrics (offline, no live LLM — gate publish & CI):** fact-safety (every
`{{table.slug.field}}` token renders to its typed column value for the reply language, and every
currency-/unit number in the draft traces to an injected value — no bare digits) **target 1.0 hard**;
asset-ref precision/recall **≥ 0.9**; language match (RU→`ru`, KK→`kk`, mixed→`ru`); escalation
correctness; must-include/exclude. The runtime **prose grounding judge** (a cheap LLM, **biased to
escalate**) guards non-numeric claims; its offline analogue is the **LLM-as-judge** (1–5 rubric on
tone/grounding, **mean ≥ 4.0**), reported but **does not gate** — it needs a live key, so it runs
nightly/manual. `POST …/assistant/publish` refuses a snapshot unless **fact-safety = 1.0** and
**asset precision ≥ 0.9** (thresholds in `config.yaml`).

---

## Porting (submodule → xchats backend)

The brain's core is decoupled by ports (`HandleMessage` reads via a chat/profile reader, drafts via an
LLM `Drafter`, **writes nothing** — the caller persists), so the port is small.

**Reuse as-is** (copy; only rewrite the Go module path `github.com/yessaliyev/xpayment-crm` → xchats):
`internal/domain/{catalog,draft,message,content,validate}.go`,
`internal/usecase/assistant/{brain,prompt,ports,errors}.go` (+ `brain_test.go`),
`internal/infrastructure/llm/openrouter.go`. The admin **service layer** is reused but **UI exposure is
deferred to Phase 4B** (HTML templates dropped — xchats is an SPA + JSON API).

**Adapt:**
- **Reader port** (`ChatwootReader` → xchats Postgres): `Window` = last ~15 `wa_messages` for the chat
  (mapped to `domain.Message`). **No profile in v1** — the conversation `stage` still rides on
  `wa_chats.stage`; `wa_contacts.attributes` is not read.
- **Draft persistence:** an `ai_draft` worker (in-memory queue, no DB `jobs` table) takes the returned
  **1–3 options** and inserts **one** `ai_suggestions` row whose `options` jsonb holds the variants (each
  with nested media) + emits `ai_draft.created`. Escalation / `PricingError` / low confidence → row
  flags, not a note. Producing up to 3 text variants is the one logic adaptation (one structured call
  returning ≤3 options).
- **Config/snapshot store:** reimplement on Postgres (`ai_snapshots/ai_topics/ai_assets/ai_tariffs/
  ai_products/ai_contacts/ai_audit_log`); keep publish/rollback/dedup semantics.
- **Seed snapshot:** carry `0002_seed.sql` into the `ai_*` tables on first boot (the cold-start fix).
- **Eval harness:** port the golden set + deterministic metrics + publish gate (incl. the language
  assertion).
- `asset_url` resolves to xchats' **media store** (local disk → object storage), not Chatwoot.

**Drop** (Chatwoot-specific / replaced): `internal/infrastructure/chatwoot/*`,
`internal/usecase/whatsapp/*`, the Chatwoot webhook handler, the old v2.2.3 Evolution client — xchats
has its own accounts/QR manager, webhook, and Evolution client.

**One pending suggestion per chat:** enforce with the partial unique
`(chat_id) WHERE state='suggested'` — not the submodule's in-process mutex. A re-press (by anyone)
supersedes the chat's pending row (one-row `UPDATE … SET state='superseded'`). Approve sets
`chosen_ordinal` + `sent_message_id`, `state='resolved'`, and sends the chosen option.

**Auto-send (deferred — Phase 4D):** v1 does not build it; `respond_mode` defaults `NEVER`. The send
path is gated on `escalate=false`, `PricingError=false`, `confidence ≥ threshold` (calibrated later),
and an active snapshot that passed the golden gate.

### Port gate (v1 / Phase 4A — done when)

- `go test ./internal/assistant/...` passes after the module-path rewrite (parity check).
- A seeded Snapshot loads from `0002_seed.sql` on boot (no admin UI).
- Pressing **Suggest** on an inbound fixture produces an `ai_suggestions` row + `ai_draft.created`
  end-to-end, and — **against the non-empty seed** — the draft is **grounded** (not an escalation) while
  an off-KB question correctly escalates. A bare escalation row does **not** satisfy the gate.
- The golden-set deterministic metrics pass offline.

Deferred (Phase 4B): a `Playground` call returns a `Draft` from a Postgres-loaded snapshot; the chat
authoring flow + `POST …/assistant/publish` enforce the quality gate at publish time.
