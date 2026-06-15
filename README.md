# xchats

A WhatsApp-first **team inbox with an AI assistant**, runnable from one place.

The **implementation plan** (design docs + reference assets) lives under [`plan/`](plan/);
**Build 0** — the runnable first version — is implemented in [`backend/`](backend/) (Go) and
[`frontend/`](frontend/) (Vue 3), orchestrated by [`deploy/`](deploy/) + the [`Makefile`](Makefile).

Plan entry point → **[`plan/0-overview.md`](plan/0-overview.md)** · Build 0 scope → **[`TODO.md`](TODO.md)**.

## Build 0 — run it

A logged-in user sees **live** WhatsApp chats/messages (text + media), can **send** replies
(text + media), and the AI is a **hardcoded stub** returning 1–3 constant draft options — the
end-to-end plumbing of the inbound→draft→approve→send→status loop. Follows the plan's `xchats`
schema, `{payload, errcode}` envelope, in-memory queue, and deterministic `wa_accounts.id`.

```bash
cp .env.example .env            # secrets (Evolution key, webhook token, DB DSN, admin login)
cp config.example.yaml config.yaml
make up                         # Postgres + backend (:8080) + frontend (:8081), one command
make webhook-set                # register our webhook on the live Evolution instance (once)
```

Local dev (no Docker): `make dev-backend` (`:8080`) and `make dev-frontend` (`:5173`).

```bash
make test        # Go unit/component + normalizer-vs-captures + frontend typecheck/build
make test-e2e    # full demo loop against a Postgres (DATABASE_URL=...): ingest→dedup→media,
                 # send fan-out to the phone JID, echo-collapse, monotonic status, suggest→approve guard
```

### Layout

```
backend/    Go: cmd/xchats + internal/{config,store,queue,blob,evolution,normalize,
            webhook→httpapi,worker,realtime,assistant,dto} + migrations/ (embedded)
frontend/   Vue 3 + TS (Vite, Pinia, Tailwind): Login + Chatboard (list · thread · assistant)
deploy/     docker-compose.yaml (Postgres + backend + frontend; Evolution reused)
plan/       the design docs + captures (the source of truth)
```

## What's in here

```
plan/
  0-overview.md              entry point: vision, architecture diagram, build order, principles
  0.1-definition-of-done.md  per-phase acceptance criteria
  1-concept.md               what & why, glossary, product principles
  2-architecture.md          components, monorepo layout, env addressing, v1 decisions, config, testing
  3-sync.md                  sync model (live / initial / reconcile) + detailed Q&A
  4-wa-connection-example.md account/QR + assign-manager flows, and the Postgres schema
  5-ui-pages.md              the frontend pages (ref: ui-chatboard.png)
  6-isolated-testing.md      how to build & test the whole app in isolation (one command)
  7-api-contracts.md         path convention, {payload,errcode} envelope, error codes, HTTP statuses, endpoints
  7.1-endpoints.md           per-endpoint request/response parameters + entity schemas
  8-ai-assistant.md          the AI brain (overview) ...
  8.1..8.5-ai-assistant-*.md ... prompt/flow, responses/media, profile, knowledge base, providers
  8.6-port-checklist.md      porting the brain from the submodule into the backend
  9-database-schema.md       full PostgreSQL schema (schema xchats, fully-named tables), keys, constraints, normalization
  scripts/evolution_client.py   Evolution client + normalization (the oracle)
  captures/                  real Evolution v2.3.7 webhook payloads (test fixtures)
  examples/repos/xpayment-crm   the working AI brain, as a git submodule (reference implementation)
```

## Reading order

`0-overview` → `1-concept` → `2-architecture` → `3-sync` → `4-wa-connection-example` →
`5-ui-pages` → `6-isolated-testing` → `7-api-contracts` → `7.1-endpoints` → `8-ai-assistant`
(+ `8.1`–`8.5`).

## Get the reference implementation (submodule)

The AI brain lives in a submodule; fetch it before reading `plan/8-*`:

```bash
git submodule update --init --recursive
```

## What this describes (in one line)

Reuse a running **Evolution** WhatsApp gateway; build a Go **backend** (webhook ingest + UI API +
workers + AI) and a Vue **frontend** as separate env-addressed services; **PostgreSQL** for all
state; **suggest-and-approve** AI ported from the submodule; everything brought up by one
orchestration and verifiable by an isolated, one-command test harness.
