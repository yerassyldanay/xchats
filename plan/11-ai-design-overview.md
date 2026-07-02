# Knowledge Base & AI — Design Overview (bird's-eye)

> ⚠️ **Partially superseded by [`14-draft-staging-and-retrieval.md`](14-draft-staging-and-retrieval.md).**
> §4–§5: topic bodies carry **no fact tokens** (tables fully independent); §6: embeddings retrieval is
> now **allowed for the Knowledge lane** (topics), never for facts; §11: the grounding judge is
> **deferred from v1**; the `drafted_at` lifecycle → separate draft tables. Updated lazily; 14 wins.

The whole AI side of xchats at altitude: the **three components**, the **handful of big decisions** that
shape everything, and — honestly — **what each buys us and what it costs**. This is the map and the
rationale, not the blueprint. Schema lives in `9-database-schema.md`, brain logic in `8-ai-assistant.md`,
the original conceptual UX in `10-knowledge-builder.md`. Read this one to understand *all of it* and the
choices; dive into those only when you implement.

---

## The big picture — three components, one contract

```text
   ┌──────────────────────┐   writes (PENDING)  ┌────────────────────┐  reads LIVE rows      ┌─────────┐
   │  PLAYGROUND          │ ──────────────────► │   KNOWLEDGE BASE    │  (drafted_at IS NULL) │  BRAIN  │
   │  chat + editor       │                     │   (one living KB;   │ ────────────────────► │  (AI)   │
   │  the only WRITER     │ ◄──── questions ───  │   rows live/pending)│                       │ suggests│
   └──────────────────────┘                     └────────────────────┘                       └────┬────┘
                                                                                                   ▼ drafts (a human approves every send)
                                                                                                customer
```

| Component | One-line job | Touches the KB how |
|---|---|---|
| **Brain (AI)** | conversation + KB → **reply drafts**; never sends on its own | **reads only** the **LIVE** rows (`drafted_at IS NULL`) |
| **Knowledge Base** | the single store of product knowledge: prose blocks + topics + **media** + **typed facts** (prices/limits/contacts); **one living set, rows live or pending** | **the contract** both sides agree on |
| **Playground** | where a human curates the KB: a **chat** (builds on its own, asks questions) + an **editor** (manual) | **the only writer**; new/edited rows land **pending** until approved |

**The rule that keeps it clean:** *only the playground writes; only the brain reads; they meet at the
KB.* The two never talk directly — they're decoupled **through** the KB. Get the KB right and each side
can be built and tested on its own.

**Core idea in one line:** *curate a small, human-approved knowledge base by chatting; the brain answers
customers only from its **live** (approved) rows, as suggestions a human approves.*

---

## The main solutions & their trade-offs

Each is a real decision. **Buys us** = advantage; **Costs us** = limitation.

### 1. Decouple the three components through the KB
- **Buys us:** independent build & test (the brain needs only the *read* contract; the playground only
  the *write* contract); one place to reason about knowledge; either side is swappable.
- **Costs us:** the KB schema becomes the coupling point — if it's wrong, **both** sides hurt; no
  brain↔playground shortcuts (everything must pass through stored knowledge).

### 2. The KB is **one living set**; pending rows carry `drafted_at` and are held out of the prompt until approved
- **Buys us:** the brain only ever sees **approved/live** rows (`drafted_at IS NULL`) — a half-built
  topic can't leak, because it's still drafted; editing never disturbs live answers (an edit re-marks the
  row **pending**); a **single, simple mechanism** (one flag, not a copy); the playground shows
  **everything** (live + pending) in one place.
- **Costs us:** **no versioned rollback** — there's no published snapshot to revert to; editing a live row
  **pulls it out of the prompt** until re-approved, and overwrites the old value **in place**; the gate
  must run **at approve time** over the resulting live set.
- **Alternative rejected:** a draft/published two-copy model with versioning — buys rollback, but costs a
  publish step, a second copy to keep in sync, and version storage; the `drafted_at` flag gives the same
  safety wall (nothing unapproved reaches a customer) with one row, not two.

### 3. A topic is a **container of text + several media**, each media with **its own description**
- **Buys us:** matches reality — for "what are your prices?" you might send a card, a PDF, or a 90-sec
  video depending on the moment; the per-file **description is exactly the cue** the brain picks on;
  "answer from a video's content" works by adding a companion text summary — **no schema change**.
- **Costs us:** the brain **never sees the media** — it trusts the human's text, so a **bad description
  = the wrong file sent**; a big media catalog inflates the prompt.
- **Generalizes:** topics aren't the only media owners — **products, tariffs and contacts** own media
  too, through the same polymorphic pattern, and carry their exact facts as **typed columns** (see §4–§5).

### 4. Facts (prices, limits, times, contacts, …) are **typed columns**, quoted as **tokens, never digits**
- **The two lanes.** The KB splits into a **Facts lane** (exact, code-substituted) and a **Knowledge
  lane** (prose the model writes, then a judge checks). This split *is* the anti-hallucination strategy
  (decision record `13`).
- **Buys us:** every exact fact is a **typed column** on a typed table (`ai_tariffs` / `ai_products` /
  `ai_contacts`), stored **verbatim with units**; the model **cannot invent** a price — it quotes the
  column only as a token `{{table.slug.field}}` (e.g. `{{tariff.growth.price}}`), which code resolves for
  the reply's language. Change the value in one place — the column — and every answer updates. **Language
  is a row, not a column** — a new language is new rows, no schema change.
- **Costs us:** indirection (the author/agent must tokenise); every column needs a **confirmed** value;
  the system must fail safe on an unresolved token (refuse, never ship a half-rendered fact).
- **Rejected:** the old generic `ai_values` token bag (a nearest-key lookup can return the *wrong*
  tariff) and per-language columns (`name_ru`/`name_kk`).

### 5. Structured entities (products, tariffs, contacts) are **typed fact tables**; media attaches **polymorphically**
- The Facts lane is `ai_products` (a sellable item), `ai_tariffs` (a pricing **plan** — fixed/percentage,
  advantages/disadvantages), and `ai_contacts` (org-support scalars), **independent of each other** (no
  links). Each **holds its exact numbers as typed columns** (`price`, `limit_text`, `fee`, `whatsapp`, …),
  stored verbatim, **one row per `(entity, language)`**. Media still attaches to **any** entity by **one
  shared `(owner_kind, owner_ref)` pair** on `ai_assets`.
- **Buys us:** many spheres (e-commerce, real-estate, services) **reuse the same per-language shape**;
  facts resolve by **exact** `{{table.slug.field}}` lookup (no wrong-tariff risk); media attaches to **any**
  entity uniformly; a new fact category is a **new typed table**, never a generic bag.
- **Costs us:** more rows (one per language); the `data jsonb` escape hatch (descriptive prose only) is
  **loosely typed**.

### 6. Knowledge is **curated-in-prompt** (no vector search yet)
- **Buys us:** deterministic, cheap, **no retrieval errors** — the model sees the whole (small) KB and
  selects.
- **Costs us:** only works while the KB **fits the prompt**; media-as-knowledge grows it faster; past
  that we must add `pgvector` retrieval (later, behind the same contract — not now).

### 7. The playground is **chat + editor over the *same* living KB**
- **Buys us:** two speeds with **no sync problem** (identical rows) — fast bulk authoring by chat,
  precise fixes by hand.
- **Costs us:** two UIs to build; simultaneous edits (agent + human) need basic conflict handling.

### 8. Interactive build = **questions (popups) + per-item accept/deny** (the core)
- **Buys us:** the chat **builds on its own yet stays safe** — when unsure it **asks** (describe this
  file? confirm this price? which topic?) instead of guessing; simple mental model (accept/deny);
  proactive (it can flag gaps and suggest topics).
- **Costs us:** LLM nondeterminism, cost, latency; needs tuning of *"ask vs. guess"*; lots of small
  approvals get tedious on a big import.
- **Optional add-on:** *git-like changesets* (atomic, reviewable before/after diffs) for bulk or risky
  edits — more power, more complexity. **Use only if** per-item approval becomes painful; not v1 core.

### 9. Media auto-extraction is **phased** — v1: operator describes; vision/transcription later
- **Buys us:** **fully chat-driven from day one** with zero vision/transcription dependency; cheap;
  future auto-extraction drops in **behind the same popup** (it just pre-fills the description) — no UX
  or schema change.
- **Costs us:** manual effort per file in v1; description quality depends on the operator.

### 10. Reuse existing infra · provider-neutral LLM · **suggest-and-approve everywhere**
- **Buys us:** minimal new surface (same blob store, realtime stream, queue, API envelope, LLM client);
  switch LLM provider by **config, not code**; a human approves **every** customer send → safety.
- **Costs us:** the v1 queue is in-memory (not durable — accepted); sending conversation + profile to an
  external LLM is a **cross-border / PII decision** to settle before going live (the data boundary).

### 11. Two guardrails over every draft: a **number check** (deterministic) + a **prose grounding judge** (LLM)
- **Buys us:** a two-part safety net matching the two lanes — every currency/unit number in a draft must
  trace to an injected fact value (deterministic, exact), and every non-numeric claim must be supported by
  the injected topics (a cheap LLM judge, **biased to escalate**). Numbers are guarded deterministically,
  prose by the judge; neither auto-approves. With human review as the final gate, a fabricated specific is
  impossible and unsupported prose is caught (see `8.2` pipeline, `8.7` evals, decision record `13`).
- **Costs us:** the judge adds a cheap per-draft LLM call and some deliberate false-positive escalations
  (on doubt it defers to a human).

---

## Open decisions (worth settling before building)

These are genuine forks, not yet locked:

1. **Approval-gate strictness** — approving runs the deterministic gate over the resulting **live** set:
   hard "every `{{table.slug.field}}` resolves (for each required language)" + "every owned media exists" +
   "no literal currency in a topic body" + "no open questions"? What asset-coverage bar? (Stricter = safer
   but harder to approve.)
2. **Per-row vs. all-at-once approval UX** — approve a single pending row, or sweep all pending at once?
   (One living KB already makes "one set per org" automatic — branchable drafts are off the table.)
3. **When (if ever)** to add changesets (§8) and `pgvector` retrieval (§6) — only on real pain.
4. **Auto-extraction timing & provider** (§9), and the **PII/data-boundary stance** for the builder LLM.
5. **Concurrency** when the chat agent and a human edit the same row at once.

---

## Build sequence (the order that de-risks)

```text
1. Lock + migrate the one-living-KB contract   ← entities + drafted_at + typed products/tariffs/contacts
                                                  + polymorphic media owner on assets + approve gate
2. Seed knowledge manually                     ← brain boots usable AND the playground has real content to edit
3. Editor first (no LLM)                       ← proves the KB write contract with a simple, deterministic writer
4. Chat next (the agent)                       ← the interactive build, riding the already-proven contract
5. Approve wires pending → live                ← drafted_at cleared; brain reloads — through the gate
```

**Editor before chat** on purpose: it forces the KB contract to be correct against a simple writer
*before* the harder LLM agent depends on it. **KB contract first** because it's small and unblocks both
the real brain and the playground at once.

---

## Where the detail lives (so this stays a map)

- **`8-ai-assistant.md`** — the brain: the prompt, how it grounds (live rows only), the approve/eval gate.
- **`9-database-schema.md`** — the full data model the KB maps onto (`ai_topics/products/tariffs/contacts/assets`,
  the `drafted_at` flag, typed fact columns, and the polymorphic `(owner_kind, owner_ref)` media pair).
- **`10-knowledge-builder.md`** — the original conceptual playground UX (popups, topic-as-container).
- **This doc** — the three components, the main solutions, and the trade-offs. Start here.

> *This overview replaces the earlier detailed split (`11.1`–`11.5`, `12`) — deliberately one file at
> altitude, per the "no pile of detailed md files" steer. Detail is added back into `8`/`9` only when a
> piece is actually built.*
