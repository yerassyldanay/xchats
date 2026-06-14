# xchats — Overview

Entry point for anyone joining the project. Read this first, then the numbered docs.

## What we're building

A WhatsApp-first **team inbox with an AI assistant**: businesses connect WhatsApp accounts,
members handle customer conversations together, and the AI suggests replies (suggest-and-approve
by default — humans stay in control). The whole system **runs from one place** (one
orchestration) and is built as a **monorepo**.

## The shape

```text
+--------------------- runs from one place: docker compose / make up ----------------------+
|                                                                                          |
|   member's browser                                                                       |
|        | HTTP + SSE                                                                       |
|        v                                                                                  |
|   +------------------+    /api + SSE      +------------------- backend (Go) -----------+  |
|   | frontend (Vue 3) | -----------------> | api edge      /xchats/api/v1/* (+SSE)      |  |
|   |  SPA, own port   |  (API_BASE_URL env)| webhook edge  /evolution/api/v1/webhook    |  |
|   +------------------+                    | workers   <-- Postgres job queue (the seam) |  |
|                                          | AI assistant (LLM, OpenAI-compatible)       |  |
|                                          +------+-----------------------+--------------+  |
|                          webhook events         | SQL                   | media bytes     |
|                            (POST)               v                       v                 |
|  WhatsApp  <=>  +---------------+        +----------------+     +------------------+        |
|  phone / web /  |  Evolution    | -----> |  PostgreSQL    |     |  blob store      |        |
|  mobile app     |  reused :9700 | <----- |  (only store)  |     |  disk -> object  |        |
|                 +---------------+  REST  |  product+queue |     +------------------+        |
|                  (send, media,  find*)   |  + AI config   |                                |
|                                          +----------------+                                |
|                                                                                          |
|  config:  .env (hosts/ports/passwords/keys/webhook token)                                 |
|           config.yaml (timeouts, min/max limits, auth, seed users + default org)          |
+------------------------------------------------------------------------------------------+
```

Backend and frontend are **separate services, each on its own port**; every service is reached
by host+port and every cross-service URL is an **env variable** (localhost / docker hostname /
domain — switch by changing env). Postgres and Evolution are **reused**, not bundled.

## Document map — where an agent/dev finds what

- **0-overview.md** (this) — vision, the architecture picture, where to start, principles.
- **1-concept.md** — what & why, glossary, product principles (non-chatbot, suggest-and-approve).
- **2-architecture.md** — components, monorepo layout, env-driven addressing, **v1 decisions**
  (users/org seed, auto-response, WhatsApp-accounts manager, Postgres-only, security, frontend),
  the **two-file config model**, and the **testing strategy** (one-command isolated e2e).
- **3-sync.md** — the sync model + detailed Q&A: live events, initial/old sync, reconcile/gaps,
  dedup, media, `@lid`↔phone, and **multi-device (phone/Web) sync**.
- **4-wa-connection-example.md** — concrete flows + the **Postgres schema**: account connect/QR,
  initial sync, incoming/outgoing handling, table DDL.
- **5-ui-pages.md** — the frontend pages (Login, Chatboard, Contacts, WhatsApp Accounts, AI
  Assistant, Settings); reference screenshot `./ui-chatboard.png`.
- **6-isolated-testing.md** — how to develop & verify the whole app (and each element) in an
  isolated environment: fake Evolution, captured fixtures, one-command e2e, swappable-adapter tests.
- **7-api-contracts.md** — the HTTP contract: endpoint list, the unified `{payload, errcode}`
  envelope, unified error codes, HTTP statuses & their meaning, and the `/xchats/api/v1` ·
  `/evolution/api/v1` path convention (authoritative; supersedes the shorthand `/api/...`).
- **7.1-endpoints.md** — per-endpoint request & response parameters (bodies, query, payload shapes,
  shared entity schemas).
- **8-ai-assistant.md** — the AI brain overview (the core of the product): `HandleMessage`, the
  `emit_draft` → `escalate→refs→prices→profile→status` pipeline, ~15-message window, suggest-only.
  - **8.1-ai-assistant-prompt.md** — prompt `[A]–[E]`, conversation flow (start/middle/end), language.
  - **8.2-ai-assistant-responses.md** — `emit_draft` output, text/media/both, incoming media handling.
  - **8.3-ai-assistant-profile.md** — building & using the contact profile.
  - **8.4-ai-assistant-knowledge-base.md** — topics, media catalog, price tokens, links.
  - **8.5-ai-assistant-providers.md** — multi-provider LLM (openrouter/openai/gemini).
- **examples/repos/xpayment-crm/** — the working brain, vendored as a git submodule (the
  implementation the AI docs above describe; entry `IMPLEMENTATION.md`).
- **scripts/evolution_client.py** — working Evolution client + normalization (the oracle).
- **captures/** — real captured Evolution webhook payloads (fixtures for the e2e tests).

## Components (who owns what)

- **Evolution** — WhatsApp transport only (reused, not forked): connect accounts, send, emit events.
- **Backend (Go)** — the source of truth. Two HTTP edges (webhook ingest + UI API) + background
  workers + the AI assistant. A **Postgres-backed job queue is the seam** between the edges and
  the workers.
- **Frontend (Vue 3 + TS)** — the single UI (connect WhatsApp, inbox, AI setup); talks only to the backend.
- **Postgres / Redis** — reused infra (Postgres = product state + Evolution's own DB; Redis = Evolution cache).

## Where to start

Reading order: **0 (this) → 1-concept → 2-architecture → 3-sync → 4-wa-connection-example.**

Build order (phased):
1. **Foundation** — monorepo scaffold (`backend/`, `frontend/`, `deploy/`), one-command run,
   env-driven addressing, DB migrations; backend reaches the reused Evolution + Postgres.
2. **Transport** — webhook receiver → normalize → DB; send text/media; delivery/read status.
   Reuse `scripts/evolution_client.py` + the captured payloads as the oracle.
3. **UI** — connect WhatsApp (QR), conversation inbox, send, live updates over SSE.
4. **AI** — port the assistant; drafts appear in the inbox; persona/knowledge configurable in the UI.

## The final point (done =)

One `make up` brings up Evolution + backend + frontend. From the UI a member can: **connect a
WhatsApp number (QR), receive and send text + media, see sent/delivered/read status, and get
AI-suggested replies they approve and send** — with the assistant configurable in the same UI.

## Non-negotiable principles

- Evolution is transport; the **backend is the source of truth** (never read Evolution's tables directly).
- Webhook **ingests fast** (store-raw + 200 + enqueue); **workers do the real work**; **one
  idempotent upsert path** for live + sync (dedup on `evolution_message_id`).
- **Stable identity:** resolve `@lid` ↔ phone via `remoteJidAlt`; key conversations on the phone identity.
- **Env-driven addressing**; reuse existing Postgres / Redis / Evolution.
- **Suggest-and-approve AI** by default — no uncontrolled auto-send.
