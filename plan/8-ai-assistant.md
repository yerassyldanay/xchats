# AI Assistant (the Brain)

> Single doc for the whole brain: prompt, responses, profile, knowledge base, providers, evals, and
> the port checklist (formerly `8.1`–`8.7`). Consistent with decision records
> [`13`](13-kb-facts-and-grounding.md) / [`14`](14-draft-staging-and-retrieval.md); on conflict they win.

The core of xchats. Given a customer conversation it produces **reviewed reply drafts** — text
(and, later, attached media) in the customer's language, grounded **only** in a curated knowledge
base. It never sends by itself (suggest-and-approve).

**Already implemented** and reused as-is — vendored as a git submodule at
`examples/repos/xpayment-crm/` (entry: `IMPLEMENTATION.md`; core in
`internal/usecase/assistant/{brain,prompt,ports}.go`, `internal/domain/{catalog,draft}.go`,
`internal/infrastructure/llm/`). xchats **ports** it: Chatwoot reads → xchats Postgres reads, draft
→ an `xchats.ai_suggestions` row, config storage SQLite → Postgres. The logic is unchanged. Full port
table in the *Porting* section below.

## v1 adapter mode (the minimal slice)

Same ported logic, smaller surface:
- **Text-only drafts.** `asset_refs` may be emitted but are **ignored/logged, not rendered/sent**.
- **One active seeded Snapshot**, loaded on boot from `0002_seed.sql`. **No authoring UI, no
  Playground** (the CMS — see `12-playground-build.md` — is deferred).
- **On-demand trigger:** a draft is produced when the user presses **"Suggest reply"**, not on every
  inbound (controls LLM spend/latency). Auto-draft-on-inbound is a fast-follow.
- **No auto-send.** A human approves every send.
- **Russian-only KB** (14 Decision 4): only `ru` rows are filled; the `lang` column stays so a new
  language later is inserted rows, not a schema change.

Everything past this (media rendering, the authoring CMS, auto-send) is designed here but staged to
v2+ (see `0.1-definition-of-done.md` Phases 4B–4D).

## Core idea in one line

A **stateless** call — `HandleMessage(window, snapshot) -> Draft`: build a cache-stable
system prompt from the live **Snapshot**, add the recent **message window** (last ~M messages),
force the model to return strict `emit_draft` JSON, then **post-process** it through the safety
pipeline (`escalate → render facts → number check → media refs → status`, judge deferred — see
*Post-processing*). **v1 sends no contact profile** — the model infers everything from the window
(see *Memory*).

---

## The prompt — cached prefix `[A]–[F]` + dynamic suffix

Implementation: `examples/repos/xpayment-crm/internal/usecase/assistant/prompt.go`.

### 1. System prefix `[A]–[F]` — cache-stable, rebuilt only on approve (`BuildSystem(snapshot)`)

```
┌──────────────────── CACHED PREFIX (stable across messages) ────────────────────┐
│ [A] FRAME      code-owned, never editable: role · JSON output contract · hard rules │
│ [B] IDENTITY   persona (+ mission) from the snapshot config                          │
│ [C] GUARDRAILS guardrails (+ language policy) from the snapshot config              │
│ [D] KNOWLEDGE  every topic: `# topic: <slug> (<lang>)` + keywords + body (pure prose) │
│ [E] MEDIA      the whole catalog as `ref | kind | topic | description` — the pick menu  │
│ [F] FACTS      the Facts lane: per fact `token | label | value` — emit the token, not the number │
├──────────────────────────  ⟵ cache breakpoint  ───────────────────────────────┤
│     DYNAMIC    WINDOW (~15 msgs, oldest first) · CURRENT MESSAGE   (no profile in v1)     │
└────────────────────────────────────────────────────────────────────────────────┘
```

Block by block:

- **[A] Frame** — hard, code-owned rules (never editable): you are a drafting engine that writes
  ONE reply for a human to review; **answer only from the KB** (else escalate — never guess);
  **facts (prices, limits, times, contacts) are tokens `{{table.slug.field}}`, never digits** — emit
  the token, never write the number; **attach media only by catalog refs, max 3**; **reply in the
  customer's language (KK+RU mix → Russian)**; **~120 words**, warm, **one** next step; never handle
  passwords; **must call `emit_draft`**.
- **[B] Identity** — `persona` (+ `mission`) from config.
- **[C] Guardrails** — `guardrails` (+ `language_policy`) from config.
- **[D] Knowledge base** — each topic rendered as `# topic: <slug> (<language>)` + keywords + body.
  Topic bodies are **pure prose — no digits and no fact tokens** (14 Decision 3); the model combines
  topic prose with `[F]` facts itself when drafting.
- **[E] Media catalog** — rows `ref | kind | topic | description`.
- **[F] Facts catalog** — the **Facts lane**: per fact, a row `token | label (its meaning) | value`,
  drawn from the typed fact tables (`ai_tariffs` / `ai_products` / `ai_contacts`). The value is shown
  **so the model can pick the right fact** (e.g. which tariff fits the customer); the model must still
  output the **token** `{{table.slug.field}}`, never the number. **v1 is ru-only**, so `[F]` is
  single-valued and fully cache-stable; once more languages exist, values resolve per reply language
  with the fallback chain reply-language → org default → `*` (neutral) row.

The prefix is rebuilt only when the KB changes (on approve); mark the cache breakpoint after `[F]`.

### 2. User block — per message (`BuildUser(profile, window, current)`)

`PROFILE` (JSON of known facts — **empty in v1**, see *Memory*) → `CONVERSATION` (the recent window,
**oldest first**, as `role: content`) → `CURRENT MESSAGE`.

### Language — how it knows what to answer in

The **[A] rule** ("reply in the customer's language; KZ+RU → Russian"), the config **language
policy** ([C]), and each topic's `language` tag ([D]). The model also returns `reply_language` for
observability. **v1 stores the KB in Russian only** (14 Decision 4): the model may translate topic
prose into the customer's language on the fly, while fact values substitute **verbatim** (a
non-Russian reply may contain Russian units — accepted for v1; neither code nor model may reformat
or translate a fact value). Once more languages exist: per-language fact rows resolve with the
fallback chain above, a missing fact for the language ⇒ escalate, and the approve-time
**completeness check** ensures every referenced entity has a row per required language (or `*`).

### Conversation flow — start / middle / end

There is **no hardcoded state machine**. The flow is driven by the **persona/guardrails** plus the
model's `suggested_status.stage` (stored on the chat), with the message window giving continuity.
The shape we encode in the persona:

- **Start (greeting)** — brief warm intro; ask what the customer needs / which product or question.
- **Middle (qualify → present)** — ask the missing qualifying fact (one at a time), answer from the
  KB, attach the right media, quote facts by token.
- **End (closing)** — confirm the next step / thank them / "write any time"; optionally propose a
  follow-up via `suggested_callback`.

Each turn the model picks the `stage` (`greeting → qualifying → presenting → closing`); it is stored
on the conversation so the next turn knows where things stand. And each turn: **ask** one concise
question when a qualifying fact is missing; **suggest/answer** from the KB (+ optional media, + fact
tokens) when it's covered; **escalate** (`escalate: true` + a short holding reply) when it isn't —
never guess.

---

## The output contract & post-processing

Implementation: `examples/repos/xpayment-crm/internal/domain/draft.go` and
`internal/usecase/assistant/brain.go`.

### Model output — `RawDraft` (the `emit_draft` JSON)

```jsonc
{
  "reply_text": "uses {{table.slug.field}} fact tokens, never numerals",
  "reply_language": "ru | kk",
  "asset_refs": ["catalog refs only, max 3; [] if none"],
  "profile_patch": { "interested_tariff": "growth" },   // deferred with the profile — empty in v1
  "suggested_callback": { "due_at": "…|null", "note": "…" } | null,
  "suggested_status": { "stage": "qualifying" } | null,
  "confidence": 0.0, "escalate": false, "escalation_reason": ""
}
```

### Post-processing → `Draft` — the safety pipeline (order matters)

The drafted reply runs through a fixed pipeline (13 Decision 6, amended by 14 Decision 6). Steps
1–3 and 5–6 ship in v1; step 4 is deferred.

1. **escalate gate** — if `escalate` is `true` (KB gap / low confidence), stop → human; ship only
   the short holding reply.
2. **template render (facts)** — replace every fact token `{{table.slug.field}}` with the stored
   column value (v1: the `ru` row; later: reply-language → org-default → `*`). **Fail closed:** any
   unresolved token → `PricingError` (the caller posts a "check facts manually" holding reply instead
   of a half-rendered value). The model never emitted a digit — code fills the number.
3. **number check (deterministic)** — every currency-/unit-adjacent number in the reply must trace
   back to a value injected in step 2; a number that appears from nowhere → **escalate**. A cheap,
   exact backstop that also catches any digit the model wrote inline against the rules. Numbers are
   guarded **deterministically** — we do *not* rely on an LLM judge for numeric errors.
4. **prose grounding judge (LLM)** — ***deferred from v1*** (14 Decision 6; human review covers
   prose until then; **mandatory before any auto-send**). When built: a separate, cheap model checks
   that every **non-numeric** claim is supported by the injected topics; unsupported → **escalate**.
   Biased to escalate — on doubt or error it defers to a human, never auto-approves.
5. **media validation** — `Snapshot.ResolveAssets`: keep only refs that exist in the catalog,
   **cap 3**; unknown refs are dropped + logged (`DroppedRefs`) → no hallucinated files. Result is
   media URLs.
6. **human review** — the final gate. Drafts are **never auto-sent**.

Bookkeeping alongside the spine (both deferred with the profile in v1): **profile** — merge
`profile_patch` onto the contact (the `stage` key is stripped out → status); **status** — set
`suggested_status.stage` and any `suggested_callback` (status ships in v1).

Final `Draft`: `ReplyText` (facts injected), `Media[]` (resolved `{ref, kind, url}`),
`ProfilePatch`, `SuggestedStatus`, `SuggestedCallback`, `Confidence`, `Escalate`.

### Text vs. media vs. both

Chosen by the model via `asset_refs`: **text only** (non-empty `reply_text`, `asset_refs: []`),
**media only** (minimal text + refs), or **both** (the text is the caption). Only refs that exist in
the catalog survive; **max 3**; we send **approved catalog assets by reference** — never generated
or arbitrary files.

### Incoming media (audio, image, …)

We **receive** several inbound media types, but the brain reasons over **text** and replies with
**text + catalog media only** (never synthesized audio/images). Per inbound type:

- **image** — stored by the media worker; its **caption** (if any) enters the window as text.
  (Optional, later: a vision/caption step adds a short description of the image to the window.)
- **audio / voice note** — stored; (optional, later) transcription adds the spoken text to the
  window. Until then the turn reads "voice message received", so the assistant asks a clarifying
  question or escalates rather than guessing.
- **document / video / sticker** — stored; the window notes the type and any caption.

---

## The Knowledge Base — what the model answers from

Implementation: the `Snapshot` in `examples/repos/xpayment-crm/internal/domain/` (`catalog.go`,
`draft.go`), rendered into the prompt by `prompt.go` (blocks `[D]`, `[E]`, `[F]`).

### Two lanes — Facts vs Knowledge

The KB holds two kinds of information, handled differently — this split *is* the anti-hallucination
strategy, owned by decision records [`13`](13-kb-facts-and-grounding.md) /
[`14`](14-draft-staging-and-retrieval.md): **Facts** (prices, tariffs, limits, times, contacts) are
exact, verbatim, typed columns that **code substitutes** — the model never writes the number;
**Knowledge** (policies, descriptions, how-things-work) is prose in `ai_topics` that the model
paraphrases, checked (once built) by the grounding judge. Exact things are never authored by the
model; explanatory things are authored but checked.

### Lifecycle — draft tables, approve, live

The KB lives in `xchats.ai_snapshots / ai_topics / ai_assets / ai_tariffs / ai_products /
ai_contacts` (see `9-database-schema.md`). The Playground stages changes in **separate draft twin
tables**; **approve = gate → copy to live → embed** (14 Decisions 1–2) — the only write path to
live. The brain reads **live rows only**, loads them into one **immutable in-memory snapshot**
(keyed per org), and **never queries the DB on the hot path**: the cached prefix `[A]–[F]` is built
**once** — on boot and on approve, not per message. An approve builds a new snapshot and
**atomically swaps the pointer**, so a message handler never sees a half-updated KB.

**One principle resolves how text, images, and video are stored: the model only ever reasons over
*text*. It never sees an image or watches a video** — everything it "knows" about a piece of media
is the *text you wrote for it*. So content lives in three forms:

| Form | Stored as | Goes in | The model's use |
|---|---|---|---|
| **Text knowledge** (topics) | `ai_topics`: `slug`, `lang`, `keywords`, `body_md` — **pure prose, no digits/tokens** | `[D]` | reads it; answers **only** from it |
| **Media** (image/video/doc/audio) | `ai_assets`: `ref`, `kind`, `topic_slug`, `description`, `url` | `[E]` | picks a `ref` (max 3); bytes attached at send |
| **Facts** (prices, limits, times, contacts) | typed columns on `ai_tariffs` / `ai_products` / `ai_contacts`, verbatim, language a row | `[F]` | picks the right fact by label + value; emits the token `{{table.slug.field}}` in the **reply**, never the number |

### Topics — the Knowledge lane (`[D]`)

Each topic has: `slug`, `language`, `keywords`, `body` (markdown). Rendered as
`# topic: <slug> (<language>)` + keywords + body. Keep topics small and answer-shaped; the model
answers **only** from these. **Links** belong in the topic body and the model includes them in the
reply when relevant. Bodies are **pure prose** — a topic never inlines a price or a fact token; the
model combines topic prose with `[F]` facts when drafting (14 Decision 3 — this is what makes every
KB table independently approvable).

### Media catalog — assets (`[E]`)

Each asset has: `ref` (stable id used in `asset_refs`), `kind` (`image | video | document | audio`),
`topic` (the slug it supports), `description` (what it is / when to send), and a `url` (resolved at
send time by the media store — local disk → object storage, see `2-architecture.md`). The model
selects assets **by `ref`** (max 3); only catalog refs resolve (`ResolveAssets`), so it cannot
attach anything not curated.

### Media is a knowledge source — via a companion summary topic

We want the assistant to answer from what's *inside* a video/image, not just attach it. Since the
model reads only text, the rule is:

> **Every knowledge-bearing image/video has both (a) an `ai_assets` row for the bytes and (b) a
> companion `ai_topics` row whose `body_md` captures its content as text, linked by `topic_slug`.**

No schema change — it leans on the existing `topic_slug` link. Two text fields, two jobs, never
conflated: the **topic body** is the *content the model answers from*; the **asset description** is
the one-sentence *selection-menu entry* ("step-by-step video for 'how do I add a cashier'") the
model picks on. Companion topics are **answer-shaped summaries (~80–150 words)**, not verbatim
transcripts — keeping the cached prefix lean. (If verbatim recall from long media is ever needed,
that's a trigger for Knowledge-lane retrieval — see *Scaling*.)

### Facts — the Facts lane (`[F]`)

Every exact fact is a **column on a typed table** (`ai_tariffs`, `ai_products`, `ai_contacts` — see
`9-database-schema.md`), stored **verbatim with units**. In replies it is referenced only as a token
`{{table.slug.field}}` (e.g. `{{tariff.growth.price}}`, `{{contact.support.whatsapp}}` — table
selects the fact table, slug the row, field the column); code substitutes the stored value **after**
drafting. **Language is a row, not a column** (one row per `(entity, language)`, `*` for
language-neutral; v1 fills `ru` only). The old generic `ai_values` bag is **removed** (a nearest-key
lookup can return the *wrong* tariff). **Fail closed:** an unresolved token never ships — it becomes
a holding reply for manual review. Any factual number stays correct, centrally editable, and
impossible for the model to invent.

### Authoring — the Playground

The authoring model (deferred CMS, designed now): an operator drops material (URLs, images, videos,
PDFs, raw text) into a **builder chat** which turns it into **draft-table rows** — topics, typed
facts, identity/goal/guardrails edits — asking via **popups** when it needs a description or a fact
value confirmed; a **KB editor page** exposes everything for accept/deny/edit, and **approve** runs
the gate and copies to live. Full design: `12-playground-build.md` (+ decisions in 13/14). Nothing
goes live unreviewed.

### Cold start — the load-bearing first task

The brain answers **only** from the KB, so an **empty KB makes every draft a useless escalation** —
the product does nothing on day one (safe but useless: suggest-only means nothing wrong reaches a
customer). Seeding is therefore not optional polish:

- **Who + what:** name the first org and the domain its KB covers (see `0.1-definition-of-done.md`
  Phase 4). Decide explicitly: **reuse** the xpayment-crm KB, or **mine this org's own ~100 recent
  chats** into a starter topic list + typed fact rows + media catalog.
- **Starter snapshot:** carry the submodule's `migrations/0002_seed.sql` (a minimal *published*
  persona/topics/facts/assets) into the Postgres `xchats.ai_*` tables on first boot — the admin opens
  onto a working example to edit, not a blank form. Seed topic bodies must be **pure prose** (strip
  the old inline tokens), facts as typed `ru` rows.
- **Quality bar before exposure:** a held-out set of real questions produces grounded,
  non-escalating drafts (the golden set below), not just "a row was produced".

### Scaling — curated-in-prompt now, Knowledge-lane retrieval next

The KB is small and hand-curated, so it fits **entirely in the prompt** → deterministic, cheap, no
retrieval errors. As it grows, **pgvector similarity retrieval over live topics** selects which
topics enter the prompt (14 Decision 5) — same Postgres, behind the same context port, without
changing the brain. **Facts are never retrieved by similarity** (a nearest-neighbor can return the
wrong tariff); the fact tables are tiny and always included exactly. Topic embeddings are refreshed
at approve time, so the index only ever contains approved content.

---

## Memory — the window only (v1)

| Horizon | Lives on | Scope | Role |
|---|---|---|---|
| **Window** | the chat | last ~M messages (~15) / 48h of *this* conversation | the brain's **only** memory in v1 |

**v1 decision: no contact profile is fed to the brain.** Each call sends only the recent message
window; the model infers who they are, what they want, and the stage from those messages. There is
**no `profile_patch` merge** and no long-term memory. This keeps the call simple and stateless and
avoids a PII profile leaving the system. The cost, stated plainly: a returning customer is **not
"remembered"** across conversations. `wa_contacts.attributes` still exists for **manual/CRM** use,
but the brain neither reads nor writes it in v1.

### The contact profile — designed, deferred

When long-term memory is wanted, reintroduce the profile **behind the reader/writer ports** without
touching the core (the seam is deliberately kept). The design (implemented in the submodule —
`profile_patch` in `domain/draft.go`, the profile step in `brain.go`):

- **What it is:** a small JSON of **known facts** about the contact plus a conversation `stage`.
  Example keys: `business_type`, `monthly_volume`, `interested_tariff`, `current_payment_method`,
  `preferred_language`. Stored on the contact; injected as the prompt's `PROFILE` block every turn.
- **How it's built:** each turn the model returns `profile_patch` with **only facts it is newly
  confident about** (the prompt forbids inventing fields). Post-processing **merges** the patch onto
  the contact (read-modify-write — never blind-clobber). The special **`stage`** key is **stripped**
  and stored as the conversation status — it describes the conversation, not the contact.
- **How it's used:** fed back as `PROFILE` so the assistant doesn't re-ask known facts; drives the
  next question; populates the contact panel in the UI.
- **Notes:** attribute keys are **pre-defined** (UI/columns stay stable); unknown keys are ignored.
  The window is short-term memory; the profile is durable memory. An optional rolling summary can
  ride the same seam.

---

## Providers & the data boundary

Implementation: `examples/repos/xpayment-crm/internal/infrastructure/llm/openrouter.go`.

The brain is **provider-neutral**: it speaks the OpenAI-compatible `chat/completions` API with a
**forced `emit_draft` tool call**, so `openrouter | openai | gemini` (or any compatible/self-hosted
model) differ only by base URL, key, and model names. The response is **defensively parsed**; if a
provider lacks tool calls, it falls back to parsing JSON from the message content. **Switching
providers is a config change — no code change.**

**Config (env — `LLM_*` only):**
- `LLM_PROVIDER` — `openrouter | openai | gemini` (selects the default base URL).
- `LLM_API_KEY` — the provider key.
- `LLM_BASE_URL` — optional override (self-host / proxy / a provider not in the list).
- `LLM_FAST_MODEL` — a cheap/quick model; `LLM_THINKING_MODEL` — a stronger one.
- `LLM_MAX_TOKENS`, `LLM_TEMPERATURE`.

Per the config split (see `2-architecture.md`): the **key** lives in `.env`; model names and limits
can live in `config.yaml`.

**Compliance (decide before any real send):** each call ships the **last ~15 messages** (plus the
profile, once it exists) to `{base}` — customer personal data leaving our infrastructure,
**cross-border** for a KZ-facing product when the base URL is foreign. **`LLM_BASE_URL` is the
compliance lever**: an **in-region / self-hosted** model is the compliant default and is config, not
code; a foreign provider requires consent + PII minimization + a DPA. A documented go-live decision,
not a build task — see `2-architecture.md` (LLM data boundary), `0.1-definition-of-done.md` Phase 4.

---

## Evals & the quality gate

Ported from the working brain's design (`examples/repos/xpayment-crm/docs/07-testing-and-evals.md`).
The brain answers **only** from the KB and **escalates** on any gap — so draft quality is a direct
function of KB coverage, and a structurally-valid draft row can still be a useless escalation.
Mechanical "a row was produced" tests do **not** measure this; quality is **measured**, not assumed.

### The golden set

A small set of **real cases** (20–40 to start), checked into git, mined from real conversations
(the same mining that seeds the KB). Each case:

```jsonc
{
  "name": "growth_tariff_price_ru",
  "window": [ { "role": "customer", "content": "Сколько стоит тариф growth?" } ],
  "profile": { "interested_tariff": "growth" },
  "expect": {
    "must_include": ["{{tariff.growth.price}}"], // rendered fact token must appear
    "must_exclude": ["1990", "2990"],         // no hand-typed digits leaking past the token system
    "asset_refs": ["price_list_pdf"],          // expected catalog refs (precision/recall vs actual)
    "reply_language": "ru",
    "escalate": false
  }
}
```

### Metrics (deterministic first, judge last)

**Deterministic (no live LLM — run offline against stubbed/recorded drafts; gates approve & CI):**

- **fact-safety (number check)** — every fact token `{{table.slug.field}}` renders to its stored
  column value, **and every currency-/unit-adjacent number in the draft traces back to an injected
  value**; **no bare digits** that should have been a token. Numbers are guarded deterministically,
  never by a judge (13 Decision 4). **Target: 1.0 (hard).**
- **asset-ref precision / recall** — `asset_refs` vs expected; no hallucinated/unknown refs survive
  (`ResolveAssets` already drops them). **Target: precision ≥ 0.9.**
- **language match** — `reply_language` correct (RU→`ru`, KK→`kk`, mixed→`ru`), plus a cheap
  script-class cross-check.
- **escalation correctness** — `escalate` matches expectation (escalates on KB-gap, answers when covered).
- **must-include / must-exclude** — required phrases present, forbidden ones absent.

**LLM-as-judge (live — nightly/manual, off the PR path):** a 1–5 rubric on tone/helpfulness/
grounding, **target mean ≥ 4.0**; reported, never a commit gate (needs a live key). It is the
offline analogue of the runtime prose grounding judge (deferred from v1 — 14 Decision 6).

### The approve/publish gate

Approving KB changes runs the **deterministic** metrics against the candidate live set before the
copy (see 14 Decision 2; endpoint detail in `7.1-endpoints.md`). Refused unless **fact-safety =
1.0** and **asset-ref precision ≥ 0.9**. The judge mean is reported but does not block. Thresholds
live in `config.yaml` so they are tunable without code.

### How it runs

- Deterministic metrics live behind a `//go:build eval` tag and run against the **Fake LLM**
  (recorded `emit_draft` outputs) — no network, runs in the isolated harness
  (`6-isolated-testing.md`). This is the part that gates approve and CI.
- The live judge run is a separate `make eval-judge` target requiring `LLM_API_KEY`.
- **In v1:** golden set + deterministic metrics + the fact-safety/asset-precision gate. **Deferred:**
  LLM-as-judge on the PR path (nightly/manual); calibrating the auto-send confidence threshold
  (belongs to Phase 4D — see *Porting → Auto-send gate*).

---

## Porting (submodule → xchats backend)

The brain's core is decoupled by ports: `assistant.HandleMessage(ctx, ChatID, inbound) -> Draft`
reads via a conversation reader, drafts via an LLM `Drafter`, and **writes nothing** — the caller
persists. So the port is small: swap the reader + the persistence side, keep everything else.
Reference: `examples/repos/xpayment-crm/IMPLEMENTATION.md`.

> **v1 adapter mode:** port the brain but run the **minimal slice** — text-only drafts, one seeded
> Snapshot (no CMS), **on-demand** trigger, no media refs rendered, no auto-send. The "Reuse as-is"
> rows below are still copied; the **adminui** row and the approve gate are deferred to **Phase 4B**
> for UI exposure.

### Reuse as-is (copy; only rewrite the Go module path)

Rewrite import path `github.com/yessaliyev/xpayment-crm` → the xchats backend module during copy.

| From (submodule) | To (xchats backend) | Notes |
|---|---|---|
| `internal/domain/{catalog,draft,message,content,validate}.go` | `internal/assistant/domain/` | `RawDraft`, `Draft`, `Snapshot`, `ResolveAssets` — pure, mostly unchanged; `PriceBook.Render` is **replaced** by the typed-fact-table token render (Decision 13 — the lossy `formatTenge` bridge is retired) |
| `internal/usecase/assistant/{brain,prompt,ports,errors}.go` (+ `brain_test.go`) | `internal/assistant/` | `HandleMessage`, `BuildSystem` `[A]–[F]`, the post-processing pipeline above — logic reused; the `[F]` facts block + the number check are added per Decision 13 (judge deferred per 14) |
| `internal/infrastructure/llm/openrouter.go` | `internal/assistant/llm/` | provider-neutral `Drafter` (openrouter/openai/gemini), forced `emit_draft` |
| `internal/usecase/admin/*`, `internal/ports/http/admin/*` (the **service layer** only — HTML templates are dropped; xchats is an SPA + JSON API) | `internal/assistant/adminui/` | persona/KB/fact CRUD, the approve flow, Playground — **deferred to Phase 4B**; v1 seeds the snapshot from `0002_seed.sql`, no editing UI |

### Adapt

| Item | Action |
|---|---|
| The conversation/profile reader port (in the submodule, `ChatwootReader` with `Window` + `Profile`) | Implement a **new xchats Postgres reader** with the same interface: `Window` = last ~15 `wa_messages` rows for the chat (mapped to `domain.Message{Role: customer if direction='in' else agent, Content, ...}`). **No profile in v1** — the conversation `stage` rides on `wa_chats.stage`; `wa_contacts.attributes` is not read. |
| Draft persistence (submodule writes a Chatwoot private note) | An `ai_draft` worker (in-memory queue, no DB `jobs` table) takes the returned **1–3 options** and inserts **one** `ai_suggestions` row whose `options` jsonb holds the variants (each with nested media) + emits `ai_draft.created`. Escalation / `PricingError` / low confidence → row flags, not a note. Producing up to 3 text variants is the one logic adaptation (one structured call returning ≤3 options). |
| Config/snapshot store (`internal/infrastructure/sqlite/*`) | Reimplement on **Postgres**: live tables `xchats.ai_snapshots / ai_topics / ai_assets / ai_tariffs / ai_products / ai_contacts` + their **draft twins** + `ai_audit_log` (see `9-database-schema.md`, 14 Decisions 1–2). |
| **Seed snapshot** (`migrations/0002_seed.sql`) | Carry it: load a minimal **published** starter persona/topics/facts/assets into the Postgres `xchats.ai_*` tables on first boot, so the service "boots usable" and the admin edits a working example, not a blank form (the cold-start fix — see *Cold start* above). Decide reuse-xpayment-KB vs. mine-this-org's-chats. |
| **Eval / golden harness** (`docs/07-testing-and-evals.md`) | Port it — golden set + deterministic metrics + the gate (see *Evals* above), incl. the **language** assertion as a deterministic golden metric. |
| Config catalog (`internal/infrastructure/config`) | Keep `LLM_*`, `Admin`, media settings; drop the Chatwoot/Evolution-provisioning blocks. Fold into the xchats `internal/config` (`.env` + `config.yaml`). |
| `asset_url` resolution | Asset refs resolve to xchats' **media store** URLs (local disk → object storage), not Chatwoot attachments. |

### Drop (Chatwoot-specific / replaced)

- `internal/infrastructure/chatwoot/*` — the Chatwoot REST adapter.
- `internal/usecase/whatsapp/*`, `internal/ports/http/admin/.../whatsapp.html` — Chatwoot-flavored
  Evolution provisioning (xchats has its own accounts/QR manager).
- `internal/ports/http/webhook.go` — Chatwoot webhook handler (xchats has its own `internal/webhook`).
- The old `internal/infrastructure/evolution/client.go` (v2.2.3-era) — xchats uses its own client
  (port of `scripts/evolution_client.py`).

### Wiring in xchats

- **Trigger (v1 = on-demand):** the member presses **"Suggest reply"** → the API enqueues an
  `ai_draft` job → `HandleMessage(window, snapshot)` → writes the suggestion row → `ai_draft.created`.
  (Auto-draft on every normalized inbound is a deferred fast-follow — keep the job decoupled so the
  trigger is just a different caller.)
- **One pending suggestion per chat:** enforce with the partial unique
  `(chat_id) WHERE state='suggested'` (and/or a per-conversation advisory lock) — **not** the
  submodule's in-process `keyedMutex`, which serializes nothing across worker processes. A re-press
  (by anyone) supersedes the chat's pending row (one-row `UPDATE … SET state='superseded'`). Approve
  is idempotent (conditional `UPDATE … WHERE state='suggested'`), sets `chosen_ordinal` +
  `sent_message_id`, `state='resolved'`, and sends the chosen option — see `7.1-endpoints.md`,
  `9-database-schema.md`.
- **Don't draft on incomplete context:** while `history_state` is `syncing/partial`, mark the draft
  `context_state` accordingly (the brain already accepts this signal).
- **Auto-send (deferred — Phase 4D):** v1 does not build it; `respond_mode` defaults `NEVER`.
  `CONFIGURE_TIME|ALWAYS` and their send path are Phase 4D, gated on `escalate=false`,
  `PricingError=false`, `confidence ≥ threshold` (calibrated later), the **grounding judge built and
  passing**, and an active snapshot that passed the golden gate.

### Port gate (v1 / Phase 4A — done when)

- `go test ./internal/assistant/...` passes after the module-path rewrite (the submodule's
  `brain_test`/domain tests are the parity check).
- A seeded Snapshot loads from `0002_seed.sql` on boot (no admin UI).
- Pressing **Suggest** on an inbound fixture (`captures/`) produces a suggestion row + an
  `ai_draft.created` event end-to-end, and — **against the non-empty seed** — the draft is
  **grounded** (not an escalation) while an off-KB question correctly escalates. A bare escalation
  row does **not** satisfy the gate.
- The golden-set deterministic metrics pass offline.

Deferred (Phase 4B): a `Playground` call returns a `Draft` from a Postgres-loaded snapshot; the
authoring flow enforces the fact-safety / asset-precision gate at approve time.
