# Knowledge Base & AI — Design Overview (bird's-eye)

The whole AI side of xchats at altitude: the **three components**, the **handful of big decisions** that
shape everything, and — honestly — **what each buys us and what it costs**. This is the map and the
rationale, not the blueprint. Schema lives in `9-database-schema.md`, brain logic in `8-ai-assistant.md`,
the original conceptual UX in `10-knowledge-builder.md`. Read this one to understand *all of it* and the
choices; dive into those only when you implement.

---

## The big picture — three components, one contract

```text
   ┌──────────────────────┐   writes (DRAFT)    ┌────────────────────┐   reads (PUBLISHED)   ┌─────────┐
   │  PLAYGROUND          │ ──────────────────► │   KNOWLEDGE BASE    │ ────────────────────► │  BRAIN  │
   │  chat + editor       │                     │   (versioned;       │                       │  (AI)   │
   │  the only WRITER     │ ◄──── questions ───  │    draft+published) │                       │ suggests│
   └──────────────────────┘                     └────────────────────┘                       └────┬────┘
                                                                                                   ▼ drafts (a human approves every send)
                                                                                                customer
```

| Component | One-line job | Touches the KB how |
|---|---|---|
| **Brain (AI)** | conversation + KB → **reply drafts**; never sends on its own | **reads only**, and only the **published** KB |
| **Knowledge Base** | the single store of product knowledge: prose blocks + topics + **media** + prices; **versioned** | **the contract** both sides agree on |
| **Playground** | where a human curates the KB: a **chat** (builds on its own, asks questions) + an **editor** (manual) | **the only writer**, and only to the **draft** |

**The rule that keeps it clean:** *only the playground writes; only the brain reads; they meet at the
KB.* The two never talk directly — they're decoupled **through** the KB. Get the KB right and each side
can be built and tested on its own.

**Core idea in one line:** *curate a small, human-approved knowledge base by chatting; the brain answers
customers only from its published version, as suggestions a human approves.*

---

## The main solutions & their trade-offs

Each is a real decision. **Buys us** = advantage; **Costs us** = limitation.

### 1. Decouple the three components through the KB
- **Buys us:** independent build & test (the brain needs only the *read* contract; the playground only
  the *write* contract); one place to reason about knowledge; either side is swappable.
- **Costs us:** the KB schema becomes the coupling point — if it's wrong, **both** sides hurt; no
  brain↔playground shortcuts (everything must pass through stored knowledge).

### 2. KB has two layers — **draft → published** — with a publish gate
- **Buys us:** the brain always reads a **complete, frozen, consistent** view (a half-built topic can't
  leak into a live answer); editing never disturbs ongoing conversations; **versioning → rollback**;
  it's the main safety wall (nothing reaches a customer unpublished).
- **Costs us:** a publish step (friction); a draft/published copy to keep in sync; a "one active draft
  per org" rule; version storage.
- **Alternative rejected:** edit the live KB directly — a half-typed price or topic would corrupt live
  answers instantly.

### 3. A topic is a **container of text + several media**, each media with **its own description**
- **Buys us:** matches reality — for "what are your prices?" you might send a card, a PDF, or a 90-sec
  video depending on the moment; the per-file **description is exactly the cue** the brain picks on;
  "answer from a video's content" works by adding a companion text summary — **no schema change**.
- **Costs us:** the brain **never sees the media** — it trusts the human's text, so a **bad description
  = the wrong file sent**; a big media catalog inflates the prompt.

### 4. Values (prices, limits, counts, …) are **tokens, never digits** in prose
- **Buys us:** numbers are always correct and **centrally editable**; the model **cannot invent** a
  price (or any value); change it in one place — in `ai_values` — and every answer updates.
- **Costs us:** indirection (the author/agent must tokenise); every token needs a **confirmed** value;
  the system must fail safe on an unknown/leftover token (refuse, never ship a half-rendered price).

### 5. Knowledge is **curated-in-prompt** (no vector search yet)
- **Buys us:** deterministic, cheap, **no retrieval errors** — the model sees the whole (small) KB and
  selects.
- **Costs us:** only works while the KB **fits the prompt**; media-as-knowledge grows it faster; past
  that we must add `pgvector` retrieval (later, behind the same contract — not now).

### 6. The playground is **chat + editor over the *same* draft**
- **Buys us:** two speeds with **no sync problem** (identical rows) — fast bulk authoring by chat,
  precise fixes by hand.
- **Costs us:** two UIs to build; simultaneous edits (agent + human) need basic conflict handling.

### 7. Interactive build = **questions (popups) + per-item accept/deny** (the core)
- **Buys us:** the chat **builds on its own yet stays safe** — when unsure it **asks** (describe this
  file? confirm this price? which topic?) instead of guessing; simple mental model (accept/deny);
  proactive (it can flag gaps and suggest topics).
- **Costs us:** LLM nondeterminism, cost, latency; needs tuning of *"ask vs. guess"*; lots of small
  approvals get tedious on a big import.
- **Optional add-on:** *git-like changesets* (atomic, reviewable before/after diffs) for bulk or risky
  edits — more power, more complexity. **Use only if** per-item approval becomes painful; not v1 core.

### 8. Media auto-extraction is **phased** — v1: operator describes; vision/transcription later
- **Buys us:** **fully chat-driven from day one** with zero vision/transcription dependency; cheap;
  future auto-extraction drops in **behind the same popup** (it just pre-fills the description) — no UX
  or schema change.
- **Costs us:** manual effort per file in v1; description quality depends on the operator.

### 9. Reuse existing infra · provider-neutral LLM · **suggest-and-approve everywhere**
- **Buys us:** minimal new surface (same blob store, realtime stream, queue, API envelope, LLM client);
  switch LLM provider by **config, not code**; a human approves **every** customer send → safety.
- **Costs us:** the v1 queue is in-memory (not durable — accepted); sending conversation + profile to an
  external LLM is a **cross-border / PII decision** to settle before going live (the data boundary).

---

## Open decisions (worth settling before building)

These are genuine forks, not yet locked:

1. **Publish-gate strictness** — hard "every price token resolves" + "every media described" + "no open
   questions"? What asset-coverage bar? (Stricter = safer but harder to publish.)
2. **One draft per org vs. branchable drafts** — v1 leans to one; branches add power + complexity.
3. **When (if ever)** to add changesets (§7) and `pgvector` retrieval (§5) — only on real pain.
4. **Auto-extraction timing & provider** (§8), and the **PII/data-boundary stance** for the builder LLM.
5. **Concurrency** when the chat agent and a human edit the same draft at once.

---

## Build sequence (the order that de-risks)

```text
1. Lock + migrate the KB contract   ← the one artifact both sides depend on (entities + draft/published + gate)
2. Seed knowledge manually          ← brain boots usable AND the playground has real content to edit
3. Editor first (no LLM)            ← proves the KB write contract with a simple, deterministic writer
4. Chat next (the agent)            ← the interactive build, riding the already-proven contract
5. Publish wires draft → brain      ← the only path to the live brain, through the gate
```

**Editor before chat** on purpose: it forces the KB contract to be correct against a simple writer
*before* the harder LLM agent depends on it. **KB contract first** because it's small and unblocks both
the real brain and the playground at once.

---

## Where the detail lives (so this stays a map)

- **`8-ai-assistant.md`** — the brain: the prompt, how it grounds, the publish/eval gate.
- **`9-database-schema.md`** — the full data model the KB maps onto (`ai_snapshots/topics/assets/values`).
- **`10-knowledge-builder.md`** — the original conceptual playground UX (popups, topic-as-container).
- **This doc** — the three components, the main solutions, and the trade-offs. Start here.

> *This overview replaces the earlier detailed split (`11.1`–`11.5`, `12`) — deliberately one file at
> altitude, per the "no pile of detailed md files" steer. Detail is added back into `8`/`9` only when a
> piece is actually built.*
