# AI Assistant (the Brain) — Overview

The brain is the core of xchats. Given a customer conversation it produces **one reviewed reply
draft** — text and/or attached media, in the customer's language, grounded **only** in a curated
knowledge base. It never sends by itself (suggest-and-approve). In **v1 the auto-send path is not
built at all** (the `respond_mode` auto modes are deferred — see *v1 adapter mode* below).

It is **already implemented** and reused as-is — vendored into this repo as a git submodule:

- `examples/repos/xpayment-crm/` — the working brain. Entry: `examples/repos/xpayment-crm/IMPLEMENTATION.md`.
- Core code: `examples/repos/xpayment-crm/internal/usecase/assistant/{brain,prompt,ports}.go`,
  `internal/domain/{catalog,draft}.go`, `internal/infrastructure/llm/`.

xchats **ports** it: replace the Chatwoot reads with xchats Postgres reads, write the draft to an
`ai_drafts` row, and move config storage from SQLite to Postgres. The logic stays the same.

## v1 adapter mode (the minimal slice)

In v1 the brain runs in a reduced mode — the same ported logic, a smaller surface:
- **Text-only drafts.** `asset_refs` may be emitted but are **ignored/logged, not rendered or sent**
  (suggested media is deferred — `8.2`, `8.6`).
- **One active seeded Snapshot**, loaded from `0002_seed.sql`/markdown on boot. **No admin UI, no
  publish/rollback, no Playground** (the CMS is deferred — `8.6`, `5-ui-pages.md`).
- **On-demand trigger:** a draft is produced when the member presses **"Suggest reply"**, not on every
  inbound — this controls LLM spend and latency in v1. Auto-draft-on-inbound is a fast-follow.
- **No auto-send.** The human approves every send.

Everything beyond this (media, CMS, auto-send) is designed in `8.*` but staged to v2+ (see
`0.1-definition-of-done.md` Phases 4B–4D).

## Core idea in one line

A **stateless** call — `HandleMessage(window, profile, snapshot) -> Draft`: build a cache-stable
system prompt from the published knowledge **Snapshot**, add the recent **message window** + the
contact **profile**, force the model to return strict `emit_draft` JSON, then **post-process** it
(`escalate → resolve media refs → inject prices → merge profile → set stage`) into a final Draft.

## Non-negotiables (from the implemented system prompt)

- Answer **only** from the knowledge base; if it's not there → **escalate** (never guess).
- Prices/limits are **tokens** (e.g. `{{price.growth}}`), never digits; code fills real values.
- Attach media **only** by refs that exist in the catalog; **max 3**.
- Reply in the **customer's language** (Kazakh+Russian mix → Russian).
- Short (~120 words), warm, **one** clear next step or question. Never ask for passwords.
- **Suggest-only**: a human approves before anything is sent.

## Two things the port must NOT drop (the working brain had them)

- **A seeded KB + a quality gate.** The brain answers **only** from the published KB and escalates on
  any gap — so an empty KB makes every draft a useless escalation. Seeding the KB (and a golden set)
  by mining real chats is the *load-bearing first task*, and answer quality must be gated, not just
  "a row was produced". See `8.4-ai-assistant-knowledge-base.md`, `8.7-ai-evals.md`, and the Phase-4
  criteria in `0.1-definition-of-done.md`.
- **A compliance decision for the LLM data boundary.** Drafting sends the last ~15 messages + the
  contact profile to the LLM provider — cross-border personal data for a KZ-facing product. Decide
  in-region/self-host (`LLM_BASE_URL`) vs. consent + DPA **before any real send**. See
  `2-architecture.md` (LLM data boundary) and `8.5-ai-assistant-providers.md`.

## Detail docs

- **8.1-ai-assistant-prompt.md** — prompt structure (`[A]–[E]`), conversation flow (start/middle/end), language.
- **8.2-ai-assistant-responses.md** — the `emit_draft` output, post-processing, text/media/both, incoming media handling.
- **8.3-ai-assistant-profile.md** — how the contact profile is built and used.
- **8.4-ai-assistant-knowledge-base.md** — how the KB (topics, media catalog, prices, links) is organized.
- **8.5-ai-assistant-providers.md** — multi-provider LLM (openrouter / openai / gemini).
- **8.7-ai-evals.md** — the golden set, deterministic quality metrics, and the publish gate (regression net).
