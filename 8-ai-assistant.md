# AI Assistant (the Brain) — Overview

The brain is the core of xchats. Given a customer conversation it produces **one reviewed reply
draft** — text and/or attached media, in the customer's language, grounded **only** in a curated
knowledge base. It never sends by itself (suggest-and-approve; auto-send is gated by the org
`auto_response_mode`).

It is **already implemented** and reused as-is — vendored into this repo as a git submodule:

- `examples/repos/xpayment-crm/` — the working brain. Entry: `examples/repos/xpayment-crm/IMPLEMENTATION.md`.
- Core code: `examples/repos/xpayment-crm/internal/usecase/assistant/{brain,prompt,ports}.go`,
  `internal/domain/{catalog,draft}.go`, `internal/infrastructure/llm/`.

xchats **ports** it: replace the Chatwoot reads with xchats Postgres reads, write the draft to an
`ai_drafts` row, and move config storage from SQLite to Postgres. The logic stays the same.

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

## Detail docs

- **8.1-ai-assistant-prompt.md** — prompt structure (`[A]–[E]`), conversation flow (start/middle/end), language.
- **8.2-ai-assistant-responses.md** — the `emit_draft` output, post-processing, text/media/both, incoming media handling.
- **8.3-ai-assistant-profile.md** — how the contact profile is built and used.
- **8.4-ai-assistant-knowledge-base.md** — how the KB (topics, media catalog, prices, links) is organized.
- **8.5-ai-assistant-providers.md** — multi-provider LLM (openrouter / openai / gemini).
