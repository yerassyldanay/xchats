# xchats — Overview

Entry point for anyone joining the project. Read this first, then the numbered docs.

## What we're building

A WhatsApp-first **team inbox with an AI assistant**: businesses connect WhatsApp accounts,
users handle customer chats together, and the AI suggests replies (suggest-and-approve
by default — humans stay in control). The whole system **runs from one place** (one
orchestration) and is built as a **monorepo**.

## The shape

```text
+--------------------- runs from one place: docker compose / make up ----------------------+
|                                                                                          |
|   user's browser                                                                         |
|        | HTTP + SSE                                                                       |
|        v                                                                                  |
|   +------------------+    /api + SSE      +------------------- backend (Go) -----------+  |
|   | frontend (Vue 3) | -----------------> | api edge      /xchats/api/v1/* (+SSE)      |  |
|   |  SPA, own port   |  (API_BASE_URL env)| webhook edge  /evolution/api/v1/webhook    |  |
|   +------------------+                    | workers   <-- in-memory queue (the seam)    |  |
|                                          | AI assistant (LLM, OpenAI-compatible)       |  |
|                                          +------+-----------------------+--------------+  |
|                          webhook events         | SQL                   | media bytes     |
|                            (POST)               v                       v                 |
|  WhatsApp  <=>  +---------------+        +----------------+     +------------------+        |
|  phone / web /  |  Evolution    | -----> |  PostgreSQL    |     |  blob store      |        |
|  mobile app     |  reused :9700 | <----- |  (only store)  |     |  disk -> object  |        |
|                 +---------------+  REST  |  product state |     +------------------+        |
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
- **0.1-definition-of-done.md** — per-phase acceptance criteria (what "done" means for phases 1–4).
- **1-concept.md** — what & why, glossary, product principles (non-chatbot, suggest-and-approve).
- **2-architecture.md** — components, monorepo layout, env-driven addressing, **v1 decisions**
  (users/org seed, auto-response, WhatsApp-accounts manager, Postgres-only, security, frontend),
  the **two-file config model**, and the **testing strategy** (one-command isolated e2e).
- **3-sync.md** — live message handling: enqueue → worker idempotent upsert, dedup, `@lid`↔phone,
  status correlation, and **multi-device (phone/Web) sync** (v1 is live-only).
- **4-wa-connection-example.md** — concrete live flows: incoming/outgoing handling and sending
  (account connect/QR kept as the deferred reference design).
- **5-ui-pages.md** — the frontend pages (Login, Chatboard, Contacts, WhatsApp Accounts, AI
  Assistant, Settings); reference screenshot `./ui-chatboard.png`.
- **6-isolated-testing.md** — how to develop & verify the whole app (and each element) in an
  isolated environment: fake Evolution, captured fixtures, one-command e2e, swappable-adapter tests.
- **7-api-contracts.md** — the HTTP contract: endpoint list, the unified `{payload, errcode}`
  envelope, unified error codes, HTTP statuses & their meaning, and the `/xchats/api/v1` ·
  `/evolution/api/v1` path convention (authoritative; supersedes the shorthand `/api/...`).
- **7.1-endpoints.md** — per-endpoint request & response parameters (bodies, query, payload shapes,
  shared entity schemas).
- **8-ai-assistant.md** — the AI brain, end to end (the core of the product): the `[A]–[F]` prompt,
  the `emit_draft` → `escalate → render facts → number check → grounding judge → media → human review`
  pipeline, the knowledge base (text/media/typed facts, media-as-knowledge via companion topics, the
  chat authoring flow), the contact profile,
  providers + data boundary, evals + publish gate, and the submodule port checklist.
- **9-database-schema.md** — the full PostgreSQL schema in a dedicated `xchats` schema (fully-named
  tables, e.g. `xchats.wa_chats`), keys, indexes, constraints, and a normalization review.
- **10-knowledge-builder.md** — the automatic KB authoring experience (deferred CMS): a builder chat
  that turns URLs/media/text into draft topics + per-asset media, the editor page, and the
  popup/request primitive (describe-media · confirm-price · accept/deny/comment). *Conceptual UX.*
- **11-design-overview.md** — the **bird's-eye view** of the AI side: the three components (brain ·
  knowledge base · playground), the main design solutions, and **what each buys us vs. costs us**
  (trade-offs), plus the KB-first build sequence. **Start here** to understand the whole AI/KB picture;
  it consolidates the earlier detailed split into one map.
- **examples/repos/xpayment-crm/** — the working brain, vendored as a git submodule (the
  implementation the AI docs above describe; entry `IMPLEMENTATION.md`).
- **scripts/evolution_client.py** — working Evolution client + normalization (the oracle).
- **captures/** — real captured Evolution webhook payloads (fixtures for the e2e tests).

## Components (who owns what)

- **Evolution** — WhatsApp transport only (reused, not forked): connect accounts, send, emit events.
- **Backend (Go)** — the source of truth. Two HTTP edges (webhook ingest + UI API) + background
  workers + the AI assistant. An **in-memory queue behind a `Queue` port is the seam** between the
  edges and the workers (Go channels in v1; swappable to Redis/Kafka via `QUEUE_DRIVER`).
- **Frontend (Vue 3 + TS)** — the single UI (connect WhatsApp, inbox, AI setup); talks only to the backend.
- **Postgres / Redis** — reused infra (Postgres = product state + Evolution's own DB; Redis = Evolution cache).

## Where to start

Reading order: **0 (this) → 1-concept → 2-architecture → 3-sync → 4-wa-connection-example.**

Build order (phased — v1 is the minimal slice; see `0.1-definition-of-done.md`):
0. **Prerequisites (blocking)** — init + verify the brain submodule (`go test ./...` passes); capture
   the missing inbound `messages.upsert` + a matched `send.message`→`messages.update` pair; seed a
   **non-empty** KB. Phases 2 and 4 are guesses until these land.
1. **Foundation** — monorepo scaffold (`backend/`, `frontend/`, `deploy/`), one-command run,
   env-driven addressing, DB migrations; backend reaches the reused Evolution + Postgres.
2. **Transport** — webhook receiver → normalize → DB; send text; delivery/read status (inbound media
   is a placeholder in v1). Reuse `scripts/evolution_client.py` + the captured payloads as the oracle.
2.5. **AI dry-run (de-risk the differentiator early)** — once normalized messages exist, port just
   `HandleMessage` + a new Postgres Window/Profile reader and run it against `captures/` fixtures,
   dumping drafts to a log / minimal view. Cheap (the brain is standalone and mockable), and answers
   "are the suggestions any good?" **weeks before** the full UI.
3. **UI (slice)** — Login + Chatboard only (chat list + thread + composer + AI draft card).
   One **pre-connected** account; the multi-account/QR connect manager is deferred.
4. **AI (slice = draft loop)** — port the assistant in **v1 adapter mode** (text-only, one seeded
   snapshot, on-demand "Suggest" trigger, no media refs, no admin UI, no playground, no auto-send);
   drafts appear in the inbox; the KB is seeded from `0002_seed.sql`/markdown, not edited in a UI.

> **Where the risk actually is:** the highest-risk, least-proven v1 work is **WhatsApp
> transport** (multi-device reconciliation, `@lid`↔phone identity, monotonic
> status). The AI brain is a **vendored, tested port** (`8-ai-assistant.md`, submodule now
> initialized) and is comparatively de-risked — it is the product's *value*, not where the build
> effort or risk concentrates. **Verify once before relying on it:**
> `cd plan/examples/repos/xpayment-crm && go test ./...` must pass.

## The final point (done =) — the v1 vertical slice

v1 is **one ruthless vertical slice**, not a platform. One `make up` brings up Evolution + backend +
frontend, and against a **single pre-connected** WhatsApp account a user can: **receive a WhatsApp
text message, see it in the inbox, get one grounded AI-suggested text reply, edit/approve it, send it,
and see delivery/read status** — surviving duplicate webhooks and retries without corrupting data.

Out of the v1 bar (deferred, design kept — see each doc's "deferred" markers): media send/receive
beyond a placeholder, the multi-account/QR connect manager, the KB admin CMS,
auto-send, and the Contacts/Settings pages. The rule: *if a feature can't break the
inbound→draft→approve→send→status loop, it isn't in v1.*

## Non-negotiable principles

- Evolution is transport; the **backend is the source of truth** (never read Evolution's tables directly).
- Webhook **ingests fast** (enqueue + 200); **workers consume and upsert**; **one
  idempotent upsert path** for live ingest + retries (dedup on `evolution_message_id`).
- **Stable identity:** resolve `@lid` ↔ phone via `remoteJidAlt`; key chats on the phone identity.
- **Env-driven addressing**; reuse existing Postgres / Redis / Evolution.
- **Suggest-and-approve AI** — in v1 the auto-send path is **not built** (not just disabled); the
  human approves every send.
