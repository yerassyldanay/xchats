# AI Assistant (the Brain)

The core of xchats. Given a customer conversation it produces **reviewed reply drafts** — text
(and, later, attached media) in the customer's language, grounded **only** in a curated knowledge
base. It never sends by itself (suggest-and-approve).

**Already implemented** and reused as-is — vendored as a git submodule at
`examples/repos/xpayment-crm/` (entry: `IMPLEMENTATION.md`; core in
`internal/usecase/assistant/{brain,prompt,ports}.go`, `internal/domain/{catalog,draft}.go`,
`internal/infrastructure/llm/`). xchats **ports** it: Chatwoot reads → xchats Postgres reads, draft
→ `xchats.ai_drafts` rows, config storage SQLite → Postgres. The logic is unchanged. Full port table
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

A **stateless** call — `HandleMessage(window, profile, snapshot) -> Draft`: build a cache-stable
system prompt from the published **Snapshot**, add the recent **message window** + the contact
**profile**, force the model to return strict `emit_draft` JSON, then **post-process** it
(`escalate → resolve media refs → inject prices → merge profile → set stage`) into a final Draft.

---

## The prompt — cached prefix `[A]–[E]` + dynamic suffix

The prefix is rebuilt only on publish (`BuildSystem(snapshot)`); mark the cache breakpoint after `[E]`.

```
┌──────────────────── CACHED PREFIX (stable across messages) ────────────────────┐
│ [A] FRAME      code-owned, never editable: role · JSON output contract · hard rules │
│ [B] IDENTITY   persona (+ mission) from the snapshot config                          │
│ [C] GUARDRAILS guardrails (+ language policy) from the snapshot config              │
│ [D] KNOWLEDGE  every topic: `# topic: <slug> (<lang>)` + keywords + body (tokens intact) │
│ [E] MEDIA      the whole catalog as `ref | kind | topic | description` — the pick menu  │
├──────────────────────────  ⟵ cache breakpoint  ───────────────────────────────┤
│     DYNAMIC    PROFILE (known facts) · WINDOW (~15 msgs, oldest first) · CURRENT MESSAGE │
└────────────────────────────────────────────────────────────────────────────────┘
```

**`[A]` the hard rules (never editable):** answer **only** from the KB `[D]`, else **escalate** —
never guess; prices/limits are **tokens** (`{{price.growth}}`), never digits — code fills them after;
attach media **only** by refs that exist in `[E]`, **max 3**; reply in the **customer's language**
(KK+RU mix → Russian); **~120 words**, warm, **one** next step; never handle passwords; `profile_patch`
holds only newly-confident facts; **must call `emit_draft`**.

**Conversation flow** is not a state machine — the persona + the model's `suggested_status.stage`
(`greeting → qualifying → presenting → closing`, stored on the chat) drive it, with the window giving
continuity. Each turn: **ask** one concise question when a qualifying fact is missing; **answer** from
the KB (+ optional media, + price tokens) when it's covered; **escalate** when it isn't.

---

## The output contract & post-processing

The model returns exactly this (structured `emit_draft`):

```jsonc
{
  "reply_text": "uses {{price.*}}/{{limit.*}} tokens, never numerals",
  "reply_language": "ru | kk",
  "asset_refs": ["catalog refs only, max 3; [] if none"],
  "profile_patch": { "only newly-confident fields": "..." },
  "suggested_callback": { "due_at": "…|null", "note": "…" } | null,
  "suggested_status": { "stage": "qualifying" } | null,
  "confidence": 0.0, "escalate": false, "escalation_reason": ""
}
```

Post-processing, **in order** (`escalate → refs → prices → profile → status`):
1. **escalate** — if true (KB gap / low confidence), flag for a human; ship only a short holding reply.
2. **refs** — `ResolveAssets`: keep only refs that exist, **cap 3**; unknown refs dropped + logged →
   no hallucinated files. Result is media URLs.
3. **prices** — replace every `{{…}}` token from the price book in the target language; an
   unknown/leftover token → `PricingError` (post "check pricing manually", never a half-rendered price).
4. **profile** — **additively merge** `profile_patch` onto the contact: add/overwrite newly-confident
   fields, **never null** a known one; strip the `stage` key (that's status).
5. **status** — apply `suggested_status.stage` (+ any `suggested_callback`).

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
normalized per version in `xchats.ai_snapshots / ai_topics / ai_assets / ai_prices` (see
`9-database-schema.md`). The brain loads it into one immutable in-memory snapshot and never queries the
DB on the hot path.

**One principle resolves how text, images, and video are stored: the model only ever reasons over
*text*. It never sees an image or watches a video** — everything it "knows" about a piece of media is
the *text you wrote for it*. So content lives in three forms:

| Form | Stored as | Goes in | The model's use |
|---|---|---|---|
| **Text knowledge** (topics) | `ai_topics`: `slug`, `lang`, `keywords`, `body_md` | `[D]` | reads it; answers **only** from it |
| **Media** (image/video/doc/audio) | `ai_assets`: `ref`, `kind`, `topic_slug`, `description`, `url` | `[E]` | picks a `ref` (max 3); bytes attached at send |
| **Prices/numbers** | `ai_prices`: `token` → `amount_text` | tokens in text | never sees the number; code fills it after |

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

### Prices — tokens, not numbers

`{{price.growth}}`, `{{limit.growth}}`, `{{pay.start}}` are referenced in topics/replies; the price book
holds the real values; code injects them **after** drafting. Prices stay correct, centrally editable,
and impossible for the model to invent. The namespace selects the field; the key selects the row.

### Authoring — a chat where the assistant builds the KB

The authoring model (deferred CMS, designed now): an operator opens a **builder chat**, drops material
(URLs, images, videos, PDFs, raw text), and the **builder assistant turns it into a draft Snapshot on
its own** — topics (each a container of media, every asset with its own send-description), price tokens,
and identity/goal/guardrails edits — asking via **popups** when it needs a description or a price
confirmed. A **KB editor page** exposes everything it created for accept/deny/edit, and **publish** runs
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

## Memory — two horizons, no summary

| Horizon | Lives on | Scope | Role |
|---|---|---|---|
| **Window** | the chat | last ~15 messages / 48h of *this* conversation | short-term memory |
| **Profile** | the contact (`wa_contacts.attributes`) | durable facts about the person/business | long-term memory |

A returning customer starting a *new* conversation has a short window but a full profile — so the brain
"remembers" them with no stored conversation state. The profile is a small JSON of known facts (+ the
`stage`); it's grown by the additive `profile_patch` merge each turn (pre-defined keys; unknown keys
ignored; never re-asks a known fact). No running summary in v1; if long threads later need it, add one
rolling-summary attribute behind the reader/writer ports without touching the core.

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

**Deterministic metrics (offline, no live LLM — gate publish & CI):** price-safety (every token
rendered, no bare digits) **target 1.0 hard**; asset-ref precision/recall **≥ 0.9**; language match
(RU→`ru`, KK→`kk`, mixed→`ru`); escalation correctness; must-include/exclude. **LLM-as-judge** (1–5
rubric on tone/grounding, **mean ≥ 4.0**) is reported but **does not gate** — it needs a live key, so it
runs nightly/manual. `POST …/assistant/publish` refuses a snapshot unless **price-safety = 1.0** and
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
  (mapped to `domain.Message`); `Profile` = `wa_contacts.attributes` (+ chat `stage`).
- **Draft persistence:** an `ai_draft` worker (in-memory queue, no DB `jobs` table) takes the returned
  **1–3 options** and inserts 1–3 `ai_drafts` rows (+ `ai_draft_assets` for suggested media) + emits
  `ai_draft.created`. Escalation / `PricingError` / low confidence → draft flags, not a note. Producing
  up to 3 text variants is the one logic adaptation (one structured call returning ≤3 options).
- **Config/snapshot store:** reimplement on Postgres (`ai_snapshots/ai_topics/ai_assets/ai_prices/
  ai_audit_log`); keep publish/rollback/dedup semantics.
- **Seed snapshot:** carry `0002_seed.sql` into the `ai_*` tables on first boot (the cold-start fix).
- **Eval harness:** port the golden set + deterministic metrics + publish gate (incl. the language
  assertion).
- `asset_url` resolves to xchats' **media store** (local disk → object storage), not Chatwoot.

**Drop** (Chatwoot-specific / replaced): `internal/infrastructure/chatwoot/*`,
`internal/usecase/whatsapp/*`, the Chatwoot webhook handler, the old v2.2.3 Evolution client — xchats
has its own accounts/QR manager, webhook, and Evolution client.

**One pending suggestion per chat:** enforce with the partial unique
`(chat_id, option_ordinal) WHERE draft_state='suggested'` (≤3 options) and/or a per-chat advisory lock —
not the submodule's in-process mutex. Approve sends the chosen option (conditional `UPDATE … WHERE
draft_state='suggested'`; siblings → `superseded`).

**Auto-send (deferred — Phase 4D):** v1 does not build it; `respond_mode` defaults `NEVER`. The send
path is gated on `escalate=false`, `PricingError=false`, `confidence ≥ threshold` (calibrated later),
and an active snapshot that passed the golden gate.

### Port gate (v1 / Phase 4A — done when)

- `go test ./internal/assistant/...` passes after the module-path rewrite (parity check).
- A seeded Snapshot loads from `0002_seed.sql` on boot (no admin UI).
- Pressing **Suggest** on an inbound fixture produces an `ai_drafts` row + `ai_draft.created`
  end-to-end, and — **against the non-empty seed** — the draft is **grounded** (not an escalation) while
  an off-KB question correctly escalates. A bare escalation row does **not** satisfy the gate.
- The golden-set deterministic metrics pass offline.

Deferred (Phase 4B): a `Playground` call returns a `Draft` from a Postgres-loaded snapshot; the chat
authoring flow + `POST …/assistant/publish` enforce the quality gate at publish time.
