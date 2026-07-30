# xchats

A WhatsApp-first **team inbox with an AI assistant**, runnable from one place.

The concise target design lives under [`plan/`](plan/), with
[`DECISIONS.md`](DECISIONS.md) authoritative;
**Build 0** — the runnable first version — is implemented in [`backend/`](backend/) (Go) and
[`frontend/`](frontend/) (Vue 3), orchestrated by [`deploy/`](deploy/) + the [`Makefile`](Makefile).

Plan entry point → **[`plan/overview.md`](plan/overview.md)**.

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
plan/       five design docs + reference captures and images
```

## What's in here

```
plan/
  overview.md                purpose, boundaries, terms, and document map
  architecture.md            channel adapters, workers, storage, and AI boundaries
  database-schema.md         target tables, responsibilities, and columns
  playground.md              material-to-draft-to-live authoring flow
  knowledge-base.md          approved-KB prompt and response example
  telegram-testing.md        verifying the Telegram channel: env vars, endpoints,
                             response contracts, curl walkthrough, no committed tooling
```

## Reading order

[`overview`](plan/overview.md) → [`architecture`](plan/architecture.md) →
[`database schema`](plan/database-schema.md) → [`playground`](plan/playground.md)
→ [`knowledge base`](plan/knowledge-base.md) →
[`telegram testing`](plan/telegram-testing.md).

## What this describes (in one line)

Reuse a running **Evolution** WhatsApp gateway; build a Go **backend** (webhook ingest + UI API +
workers + AI) and a Vue **frontend** as separate env-addressed services; **PostgreSQL** for all
state; **suggest-and-approve** AI; everything brought up by one
orchestration and verifiable by an isolated, one-command test harness.
