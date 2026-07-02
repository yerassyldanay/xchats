# xchats

A WhatsApp-first **team inbox with an AI assistant**, runnable from one place.

The **implementation plan** (design docs + reference assets) lives under [`plan/`](plan/);
**Build 0** — the runnable first version — is implemented in [`backend/`](backend/) (Go) and
[`frontend/`](frontend/) (Vue 3), orchestrated by [`deploy/`](deploy/) + the [`Makefile`](Makefile).

Plan entry point → **[`plan/0-overview.md`](plan/0-overview.md)**.

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

## Build 1 — WhatsApp accounts manager (add via QR · reconnect · clean)

On top of Build 0's single seeded number, a logged-in user can **manage numbers from the UI**
(`/accounts`): **add** a number (create an Evolution instance → scan a live-polled **QR** → the row
is written on connect with `id = uuidv5(owner_jid)`), see **all numbers' chats together** in one
inbox (labelled + filterable by number, with a "from number" picker in the composer),
**reconnect** a broken session (same row, history intact), and **clean** a number (delete the
instance + soft-delete the row — re-adding the same number **revives** it with its history). A
separate **`/instances`** view sweeps stray/stale Evolution instances. Identity is the WhatsApp
number, so instance churn never loses chats. Endpoints: `GET/POST /whatsapp-accounts`,
`GET …/qr`, `POST …/{id}/reconnect`, `DELETE …/{id}`, `GET/DELETE /whatsapp-instances`.

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
  0-overview.md              entry point: vision, architecture diagram, glossary, build order, principles
  0.1-definition-of-done.md  per-phase acceptance criteria
  2-architecture.md          components, monorepo layout, env addressing, v1 decisions, config, testing
  3-sync.md                  sync model (live / initial / reconcile) + detailed Q&A
  4-wa-connection-example.md account/QR + assign-manager flows + the Evolution send-API appendix
  5-ui-pages.md              the frontend pages (ref: ui-chatboard.png)
  6-isolated-testing.md      how to build & test the whole app in isolation (one command)
  7-api-contracts.md         path convention, {payload,errcode} envelope, error codes, HTTP statuses, endpoints
  7.1-endpoints.md           per-endpoint request/response parameters + entity schemas
  8-ai-assistant.md          the AI brain, end to end: prompt, responses, profile, KB, providers, evals, port checklist
  9-database-schema.md       full PostgreSQL schema (schema xchats, fully-named tables), keys, constraints, normalization
  11-ai-design-overview.md   bird's-eye view of the AI side: components, decisions, trade-offs
  12-playground-build.md     the buildable Playground (KB authoring) design
  13-*.md / 14-*.md          decision records (typed facts & grounding; draft staging & retrieval)
  scripts/evolution_client.py   Evolution client + normalization (the oracle)
  captures/                  real Evolution v2.3.7 webhook payloads (test fixtures)
  examples/repos/xpayment-crm   the working AI brain, as a git submodule (reference implementation)
```

## Reading order

`0-overview` → `2-architecture` → `3-sync` → `4-wa-connection-example` →
`5-ui-pages` → `6-isolated-testing` → `7-api-contracts` → `7.1-endpoints` → `8-ai-assistant`;
for the AI/KB side then `11` → `12` → the decision records `13`/`14`.

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
