# The Knowledge Base is the Contract — the three-component rethink

A clean reframing of xchats' AI around **three components**, with the **knowledge base (KB) as the
shared spine**. This is the *organizing* doc — it supersedes the **framing** scattered across `8`/`9`/
`10`/`11`, and points to them for detail (the underlying data model and specs still hold where noted).
Read this first; it is the answer to *"how do we achieve all this?"*

---

## 1. The three components

```text
   ┌──────────────────────┐    writes (draft)     ┌───────────────────┐   reads (published)   ┌─────────┐
   │  PLAYGROUND           │ ────────────────────► │  KNOWLEDGE BASE    │ ────────────────────► │  BRAIN  │
   │  chat + editor        │                       │  (versioned;       │                       │ (AI)    │
   │  (the only WRITER)    │ ◄──── questions ─────  │   draft+published) │                       │ suggest │
   └──────────────────────┘                        └───────────────────┘                       └────┬────┘
                                                                                                     │ drafts
                                                                                                     ▼  (suggest-and-approve)
                                                                                                  customer
```

| Component | Job | Relationship to the KB | Doc |
|---|---|---|---|
| **Brain (AI assistant)** | Reads conversation + KB → produces reply **suggestions / drafts**. Never sends by itself. | **Reads only** — and only the **published** KB. | `8-ai-assistant.md` |
| **Knowledge Base** | The single store of all product knowledge: prose config + topics + **media files** + prices. Versioned. | **It is the contract** — the one thing both sides agree on. | this doc + `9-database-schema.md` |
| **Playground** | Where a human curates the KB: a **chat** (interactive, builds/updates on its own, asks questions) + an **editor** (manual changes). | **The only writer** — and only to the **draft** KB. | `11.*` |

> **The principle that makes this clean:** *only the playground writes; only the brain reads; they meet
> at the KB.* Neither component talks to the other — they are decoupled **through** the KB. Get the KB
> right and both sides can be built and tested independently.

---

## 2. Why design the KB first — and from the playground's side (your instinct, made concrete)

Both components depend on the KB, but for **opposite** reasons:

- The **brain** wants it **complete, frozen, fast to read, never half-built** (an incomplete row makes a
  bad suggestion).
- The **playground** wants it **editable, incomplete-tolerant, question-friendly** (a media file *will*
  exist for a moment before anyone has described it; an item *will* sit "proposed" awaiting approval).

If we design the KB for only one side, the other fights it. So we design it to satisfy **both** — and we
**derive its shape from the playground**, because the playground has the *harder* requirements (it must
represent half-built, ambiguous, pending, and questioned states). Then we check the brain is served.
That's the whole method: **playground-derived, brain-validated, one KB.**

---

## 3. Step 1 — what the PLAYGROUND needs the KB to support

Read each playground element as a list of demands on the KB.

### The chat element ("upload media + text; builds/updates on its own; asks questions")
1. **Write through a few clear operations** an agent can call as tools: *add/append a topic · add a media
   file to a topic · set a media file's description · set a price · edit a config block.* (Small, total,
   composable — so "builds on its own" = the agent chaining these.)
2. **Represent incomplete items.** A media file can exist with **no description yet** — the chat will ask
   for one. → every item carries a **completeness** state (`complete | incomplete`).
3. **Represent pending items** awaiting **accept/deny**. Auto-built items are **proposed** until a human
   accepts. → every item carries a **review** state (`proposed | accepted | rejected`).
4. **Carry open questions** (the popups). "Describe this file?", "Confirm this price → token?", "Which
   topic does this belong to?", "Accept this topic?" → a small **questions queue** tied to items.
5. **Read "what's here and what's missing"** so the chat can be **proactive** (ask about the gap, suggest
   the missing topic). → a cheap **status view** of the draft.
6. **Track provenance** — *made from which file / which message* — so the editor and the user can verify.
   → every item records its **origin**.

### The editor element ("manual changes")
7. **CRUD every element** (blocks, topics, media files, prices) and **see each item's status**
   (complete/incomplete, proposed/accepted) and **group media under its topic**.

**Distilled, the KB must carry per item:** `content + completeness + review-state + provenance`, plus a
side **questions queue**. That is the playground's contract.

---

## 4. Step 2 — validate the same KB serves the BRAIN

The brain needs: a **complete, validated, immutable** read view — config blocks + topics (text with
**price tokens**, never raw digits) + the **media catalog** (each with its **description = the selection
cue** for "which file to send when") + the **price book** — and **versioning** so editing never disturbs
a live conversation.

Crucially, the brain's needs are a **subset** of the draft: *the accepted + complete items, frozen.* So
the two sides are reconciled by a single move:

## 5. The model — one KB, two layers, one gate

```text
KNOWLEDGE BASE  =  a versioned SNAPSHOT, existing in two layers:

  DRAFT layer        the PLAYGROUND edits it          mutable · may be incomplete · has proposed items + open questions
      │
      │  PUBLISH  ── the gate ──  validates: every media described · every price token resolves ·
      │                            no open questions · no leftover 'proposed' · blocks present
      ▼
  PUBLISHED layer    the BRAIN reads it               immutable · complete · validated · versioned
```

**Entities (identical in both layers; the lifecycle fields only *matter* in the draft):**

| Entity | Fields | Maps to (`9-database-schema.md`) |
|---|---|---|
| **Snapshot** | `version`, `state: draft\|published\|archived` | `ai_snapshots` |
| **Config blocks** | identity · goal · quality · support (prose) | `ai_snapshots.{persona,mission,guardrails,support_policy}` |
| **Topic** (a container) | `slug`, `lang`, `keywords`, `body` (price **tokens**, no digits) | `ai_topics` |
| **Media file** (belongs to a topic) | `kind: image\|video\|pdf\|link\|infographic\|audio`, **`description`** (selection cue), `url` | `ai_assets` |
| **Price** | `token → value` | `ai_prices` |
| *draft-only* per item | `completeness`, `review_state`, `provenance` | additive columns (`11.1`) |
| *draft-only* | **Questions queue** (the popups) | `ai_builder_requests` (`11.1`) |

> **This is not a teardown.** The data model already designed in `9-database-schema.md` (and detailed as
> DDL in `11.1`) **is** this KB. The rethink is the **framing**: name the KB the explicit contract, make
> the **draft/published two-layer split the organizing idea**, and ensure the **lifecycle fields +
> questions queue** (the playground's needs from §3) are first-class. One migration realizes it (`11.1`'s
> `0003` + the lifecycle fields).

---

## 6. The access contract (the one agreement both sides code against)

| Actor | Reads | Writes | Layer |
|---|---|---|---|
| **Brain** | blocks · topics · media catalog · prices | *nothing* | **published** only |
| **Playground — editor** | draft + per-item status | items (CRUD) | **draft** |
| **Playground — chat** | draft + status + questions | items (via the §3.1 ops) + questions | **draft** |
| **Publish action** | the draft | flips draft → published **iff** the gate passes | the gate |

Because the brain only ever sees a **published, validated, frozen** snapshot, **a playground mistake
cannot reach a customer** without (a) being accepted by a human, (b) passing the publish gate, and (c)
surviving the brain's own suggest-and-approve (a human approves every send). Aggressive auto-building in
the chat is therefore safe.

---

## 7. The two interactive primitives (kept simple)

Everything in *"interactive · ask questions · clarify · describe media · accept/deny popup"* reduces to
**two** primitives over the draft:

- **Question (popup)** — the chat asks; the human answers with **accept/deny** or a short text. Resolving
  it **fills the item** (sets the description, confirms the price→token, picks the topic) and lets the
  chat continue. Types: `describe_media · confirm_price · choose_topic · approve_item`.
- **Per-item accept/deny** — an auto-built item lands as **proposed**; the human **accepts** (it becomes
  part of the draft) or **denies** (it's dropped) — from either the chat or the editor.

> **Deliberate simplification vs. the earlier `11.*` draft.** `11` introduced *git-like changesets* for
> bulk/important edits. That stays as an **optional enhancement** for large batches and risky edits (it
> composes on top — applying a changeset just creates `proposed` items), but the **core** model is the
> simpler **per-item proposed/accepted + questions queue** above — closer to your "popup accept/deny"
> framing. **Build the simple core first; add changesets only if bulk editing proves painful.**

---

## 8. The sequence (your plan, refined)

1. **Finalize the KB schema as the contract** *(the accent you asked for).* Lock the entities (§5), the
   draft-only lifecycle fields, the questions queue, and the **publish gate**. One migration realizes
   `ai_snapshots/topics/assets/prices` + the lifecycle fields + the questions table (`11.1` → `0003`).
   *This is the single artifact both other components depend on — get it right and they decouple.*
2. **Seed knowledge manually.** Load a real **published** snapshot (reuse `0002_seed.sql` or the pilot
   org's content) so the **brain boots usable** *and* the **playground has real content to edit** — the
   cold-start fix (`8-ai-assistant.md`).
3. **Build the playground** (viewed from §3):
   - **3a. Editor first** — CRUD over the draft + per-item status. *No LLM needed*; it directly exercises
     and proves the KB contract (the cheapest validation that the schema is right for the writer side).
   - **3b. Chat next** — the agent that chains the write-ops and raises questions; the interactive build.
4. **Publish** wires draft → brain through the gate (the only path to the live brain).

Doing the **editor before the chat** (3a→3b) is the safest order: it forces the KB contract to be
correct against a simple, deterministic writer *before* the harder LLM agent rides on it.

---

## 9. What this doc changes vs. keeps

- **Keeps (still authoritative for detail):** `8-ai-assistant.md` (brain logic + prompt + publish gate),
  `9-database-schema.md` (schema), `11.1` (DDL), `11.2` (API), `11.3` (agent), `11.4` (UI), `11.5` (build
  plan).
- **Reframes:** the KB is now the **explicit center**; the draft/published split is the organizing idea;
  the playground's **core** is per-item accept/deny + questions (changesets demoted to optional, §7).
- **Next concrete step:** finalize + migrate the KB contract (§8.1). It is small, unblocks both the real
  brain and the playground, and is the right place to start.
