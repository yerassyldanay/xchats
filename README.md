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
cp .env.example .env            # secrets (DB paths, session secret, LLM keys, admin login)
make up                         # backend (:8080) + frontend (:8081), one command
```

The committed root [`config.yaml`](config.yaml) carries only non-secret boot/infra tunables
(listen address, on-disk paths, logging, worker counts) — copy it only if you want to change one
of those from its defaults; `.env` is still where secrets and provider keys live for now.

WhatsApp connects directly via [`go.mau.fi/whatsmeow`](https://github.com/tulir/whatsmeow) — no
separate gateway to run or configure. Pair a number from the UI's QR flow (`/accounts` →
**add**).

Local dev (no Docker): `make dev-backend` (`:8080`) and `make dev-frontend` (`:5173`).

## Build 1 — WhatsApp accounts manager (pair via QR · logout · clean)

On top of Build 0's single seeded number, a logged-in user can **manage numbers from the UI**
(`/accounts`): **pair** a number (scan a live-polled **QR**, rendered by `internal/whatsmeow` —
the row is written on connect with `id = uuidv5(owner_jid)`), see **all numbers' chats together**
in one inbox (labelled + filterable by number, with a "from number" picker in the composer),
**logout** a session (same row, history intact, ready to re-pair), and **clean** a number
(logout + soft-delete the row — re-adding the same number **revives** it with its history).
Identity is the WhatsApp number, so a logout/re-pair cycle never loses chats. Endpoints:
`GET /whatsapp-accounts`, `POST /wa-accounts/pair`, `GET /wa-accounts/pair/{session_id}`,
`POST /wa-accounts/{id}/logout`, `GET /wa-accounts/{id}/status`, `DELETE /whatsapp-accounts/{id}`.

```bash
make test        # Go unit/component + normalizer-vs-captures + frontend typecheck/build
make test-e2e    # full demo loop against a Postgres (DATABASE_URL=...): ingest→dedup→media,
                 # send fan-out to the phone JID, echo-collapse, monotonic status, suggest→approve guard
```

### Layout

```
backend/    Go: cmd/xchats + internal/{config,store,queue,blob,whatsapp,whatsmeow,
            httpapi,worker,realtime,assistant,dto} + migrations/ (embedded)
frontend/   Vue 3 + TS (Vite, Pinia, Tailwind): Login + Chatboard (list · thread · assistant)
deploy/     docker-compose.yaml (backend + frontend; WhatsApp via whatsmeow, no gateway)
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

Connect WhatsApp directly via **whatsmeow** (no external gateway); build a Go **backend**
(direct WhatsApp connection + UI API + workers + AI) and a Vue **frontend** as separate
env-addressed services; **SQLite** for all state; **suggest-and-approve** AI; everything brought up by one
orchestration and verifiable by an isolated, one-command test harness.
