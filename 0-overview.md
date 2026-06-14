# xchats — Overview

Entry point for anyone joining the project. Read this first, then the numbered docs.

## What we're building

A WhatsApp-first **team inbox with an AI assistant**: businesses connect WhatsApp accounts,
members handle customer conversations together, and the AI suggests replies (suggest-and-approve
by default — humans stay in control). The whole system **runs from one place** (one
orchestration) and is built as a **monorepo**.

## The shape

```text
WhatsApp  ⇄  Evolution (reused, transport only)  ⇄  xchats-backend (Go)
                                                      ├─ webhook edge  (Evolution → us)
                                                      ├─ API edge      (UI → us, + SSE)
                                                      ├─ workers       (queue consumers)
                                                      └─ AI assistant
   xchats-backend  ⇄  Postgres (source of truth)  +  Redis (Evolution cache)
   xchats-frontend (Vue)  →  backend /api + SSE
```

Backend and frontend are **separate services, each on its own port**; every service is reached
by host+port and every cross-service URL is an **env variable** (localhost / docker hostname /
domain — switch by changing env). Postgres, Redis, and Evolution are **reused**, not bundled.

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
