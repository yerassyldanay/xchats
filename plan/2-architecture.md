# Architecture

The product should be built as a WhatsApp-first messaging workspace with a clear boundary between transport, product state, realtime UI, and AI assistance.

## High-Level Flow

```text
Evolution API
  -> backend webhook receiver
  -> database normalization
  -> realtime broadcast
  -> Vue team inbox

Vue team inbox
  -> backend send-message API
  -> Evolution API
  -> webhook/status update
  -> realtime broadcast

Incoming message
  -> backend
  -> worker
  -> AI assistant
  -> draft/suggestion/action
  -> realtime broadcast
```

## Running Everything From One Place

The core requirement: **one command brings the whole system up** — Evolution, the backend
(API + workers + webhook receiver + AI), the UI, and the data stores it needs. "One place"
is an **orchestration layer**, not a single repository. The code may live in several repos
under the hood; the orchestration is what ties them together.

### Orchestration

- A single `docker compose` (plus a small `Makefile` / `make up`) is the one entry point that
  runs every component.
- Components can be separate repos, referenced by the orchestration as pinned images or build
  contexts (e.g. git submodules):
  - **evolution** — the upstream image, no fork.
  - **backend** — our Go service (API, workers, webhook receiver, AI assistant).
  - **ui** — our Vue app.
  - **deploy / orchestration** — the compose + env + Makefile that unifies them. This is "the
    one place." A single monorepo is equally valid — the orchestration matters, not the repo count.

### Evolution — how we run it

- Run the official `evoapicloud/evolution-api` image (pinned, currently `v2.3.7`) as a service.
  We do **not** fork or rebuild it. It is pure WhatsApp transport.
- The orchestration gives Evolution its own Postgres database/schema, a Redis cache, and a
  persistent volume (Baileys session).
- Our backend drives Evolution over REST (send, media, history) and receives its webhooks.
  Evolution's own Manager UI is internal/admin only — never the product UI.

### Webhook receiver — do we write it from scratch?

- **Yes — it is part of our backend, written by us.** Not a separate service, and not the old
  standalone webhook service (retired). The receiver is where Evolution events are verified,
  normalized, deduplicated, and persisted into our model — core product logic we must own.
- We reuse the proven parsing/normalization knowledge (the `scripts/evolution_client.py`
  reference and the captured real payloads), but the endpoint lives in the backend.
- On startup the backend **auto-configures Evolution** to POST its events to this receiver
  (one-command setup, no manual wiring).

### UI — one app

- A single UI app covers the whole product: connect WhatsApp (QR), the team inbox
  (conversations + send), and AI assistant setup. One login, one URL.
- Whether it is embedded in the backend binary or shipped as its own container is a deployment
  detail; conceptually there is exactly one product UI.

### Data stores — Postgres, Redis, files

- **PostgreSQL** is the product source of truth (our tables) and also hosts **Evolution's own
  database** (a separate schema/DB). One Postgres server can serve both.
- **Reuse-first, runnable standalone**: by default the stack reuses an existing Postgres/Redis;
  it can also run its own via **separate, optional compose scripts** (Postgres and Redis each in
  their own script), selected by env. Both modes are possible at the same time.
- **Redis** is used by Evolution (cache). The backend does **not** require Redis in v1 — its job
  queue is a Postgres table. Redis stays optional.
- **Media / files** are stored by us behind an abstraction (local disk in dev, object storage in
  prod) — never left only inside Evolution.
- **AI assistant** is a backend component using an OpenAI-compatible LLM provider (e.g.
  OpenRouter).

## Monorepo: Structure, Build, and Endpoints

### Why a monorepo

The Go backend and the Vue frontend live in **one repository as plain directories** (not git
submodules). Rationale:

- A product change usually touches **both** UI and API; one repo means atomic commits and a
  single version — no cross-repo coordination or pointer bumps.
- One place to clone, build, run, and test; the API contract has a single source of truth.
- It's the simplest "run everything from one place": one build produces one artifact.
- Submodules were rejected — they are *separate* repos in disguise (fiddly clones/HEADs,
  cross-cutting changes split across repos) with no benefit for a single product/team.

### Project structure

```text
xchats/
  backend/     Go module — cmd/ (entry points), internal/ (api, webhook, worker, assistant,
               normalize, store, queue, blob, realtime), migrations/
  frontend/    Vue 3 + TypeScript (Vite) — src/, dist/ (build output, embedded into the binary)
  deploy/      docker compose + env + Makefile  — the one place to run it
  docs/        1-concept, 2-architecture, 3-sync, 4-wa-connection-example
  scripts/     evolution_client.py (normalization oracle), captures/ (real payload fixtures)
```

### Build & run — separate services, addressed by env

Backend and frontend live in one repo but are **built and run as separate services, each on its
own port**. This keeps running and exposing them simple: in prod you map a domain/subdomain (or
an IP:port) to each; locally each is just a port on `localhost` or a docker-network hostname.
No service is hidden behind another's port.

- **Dev:** backend `go run ./backend/cmd/xchats serve` → `:8080`; frontend `vite dev` → `:5173`.
  The frontend reads the backend URL from env (`API_BASE_URL=http://localhost:8080`); the backend
  allows the frontend origin via configurable CORS.
- **Prod:** two containers built from the monorepo — `xchats-backend` (Go, e.g. `:8080`) and
  `xchats-frontend` (built Vue served by nginx, e.g. `:80`). Each is reachable on its own
  host:port and fronted by its own domain/subdomain (e.g. `app.xchats.…` → frontend,
  `api.xchats.…` → backend) or plain IP:port.

### Addressing & discovery — everything is env-driven

Principle: **every service is reachable by host + port, and every cross-service URL is an
environment variable.** Nothing assumes "same origin"; you move between localhost, the docker
network, and production domains by changing env only.

- Each service binds a configurable address/port (`HTTP_ADDR` / `PORT`).
- Frontend → backend: `API_BASE_URL` = `http://localhost:8080` (local) /
  `http://xchats-backend:8080` (docker network) / `https://api.xchats.example` (prod).
- Evolution → webhook: `WEBHOOK_PUBLIC_BASE_URL` set so Evolution can reach the backend
  (docker-network hostname or public domain) + the webhook path.
- CORS allowed origins are configured via env to match the frontend's address.

### Testing

- Backend: `go test ./...` — the normalizer is tested against the **captured real Evolution
  payloads**; handlers/workers unit-tested; an integration layer exercises webhook → DB.
- Frontend: component/unit tests (Vitest); optional end-to-end (Playwright) against a running backend.
- The two are tested independently; an e2e layer covers the full loop (UI → API → worker).

#### One-command end-to-end test in an isolated environment

We must be able to validate the **whole app — including Evolution integration and webhook
handling — in an isolated environment with no real WhatsApp**, and trust that the real run will
behave the same. The key: the Evolution contract is pinned to **real captured payloads**, so we
can reproduce it deterministically.

- **Fake Evolution** — a small stub HTTP server implementing the Evolution endpoints we call
  (`instance/create`, `instance/connect`, `message/sendText`, `message/sendMedia`, `chat/find*`,
  `getBase64FromMediaMessage`) with canned responses, and able to **POST captured webhook events**
  to our webhook edge.
- **Fixtures** — the real payloads in `captures/` (messages.upsert/update/send.message, text +
  media) are the webhook inputs and the expected REST shapes.
- **Test stack (one run)** — `make test-e2e` brings up **Postgres + backend + fake-Evolution**
  (e.g. `deploy/compose.test.yaml`), runs migrations, then executes the suite that:
  1. replays captured webhook events → asserts normalized rows (contacts/conversations/messages),
     dedup, and `@lid`↔phone resolution;
  2. calls the backend send API → asserts the fake Evolution received the correct call (phone, not `@lid`);
  3. replays status events → asserts `sent → delivered → read` transitions;
  4. exercises the media path against the fake `getBase64`.
  Then it tears the stack down. **No external network** — this runs fully inside Claude Code's
  isolated environment from one command.
- **Real smoke (manual, outside isolation)** — a tiny optional check against the live Evolution
  (`localhost:9700`) confirms reachability/credentials; everything else is covered by the fixtures,
  which ARE byte-for-byte real Evolution output.

### Where the endpoints live

The **backend** service exposes these route groups:

| Consumer | Routes | Notes |
|---|---|---|
| Frontend (members) | `/xchats/api/v1/*` and `/xchats/api/v1/realtime` (SSE) | authenticated; queries, commands, live stream |
| Evolution webhook | `/evolution/api/v1/webhook/{account}/...` | token-secured; only Evolution calls it; store-raw + 200-fast + enqueue |
| Ops | `/healthz`, `/readyz`, `/metrics` | health & monitoring |

The **frontend** service serves the UI (built Vue app) on its own port and only consumes the
backend's `/api`. **Background workers expose no public HTTP endpoints** — they are goroutines
(or, later, a separate worker process) that consume the Postgres job queue; their interface is
the queue, not HTTP (a split-out worker would expose only `/healthz`/`/metrics`).

## v1 Scope & Decisions

### Users & organization — seeded, then self-served

- **One default organization** in v1; every member belongs to it.
- The service is **bootstrapped from a config file** (JSON or YAML) that defines the default
  organization and an initial list of users (email + password). On startup the backend **upserts**
  them (idempotent — safe to re-run). The idea: configure the user list, then run the service.
- After boot, any user can **create more users** (email + password) from the UI/API; new users
  join the default organization. Login = email + password + session.

### Organization settings — auto-response

- The organization carries an **auto-response mode**: `NEVER | CONFIGURE_TIME | ALWAYS`, plus a
  **time window** used when the mode is `CONFIGURE_TIME`. This governs when the AI may respond on
  its own (default-safe: `NEVER` → suggest-and-approve only).

### WhatsApp accounts — a simplified Evolution `/manager`

Same idea as Evolution's `/manager` page, but a simpler UX:

- **List all Evolution instances** (fetched from Evolution) with their connection status.
- **Assign / unassign** each instance to the organization. Only **assigned** instances have their
  messages handled by xchats; events for unassigned instances are ignored/not surfaced.
- **Add WhatsApp account**: click → enter an instance name + scan instructions → backend creates
  the instance and shows the **QR code** → user scans → on success it is **assigned** to the org
  (with an **unassign** button). Existing/old instances also show assign/unassign controls.
- Detailed connect/QR flow: see `4-wa-connection-example.md`.

### Storage — PostgreSQL only

- **Everything xchats owns lives in PostgreSQL** — product state, the job queue, **and** the AI
  assistant's config/snapshots (the ported brain moves off SQLite). Different databases or schemas
  are fine, but it is the **same Postgres server, only Postgres** — no SQLite, no other store.
- Media **bytes** still go to the blob store (local disk → object storage); their **metadata** is
  in Postgres. (Redis stays Evolution's cache — reused, not used by xchats directly.)

### Security — single shared token

- The Evolution webhook is protected by a **single shared token set in `.env`** (a general token,
  not per-account). Evolution includes it; the backend verifies it on every webhook call.

### Frontend — fast SPA

- Vue 3 + Vite **single-page app** (Pinia + Vue Router), optimized purely for a fast, snappy UX —
  no SEO/SSR/geo concerns. Talks to the backend over `/api` + SSE.

### Configuration — two files

Configuration is split by **secrecy and change-rate**:

- **`.env`** — credentials and environment wiring: hosts, ports, passwords, the Postgres DSN, the
  Evolution base URL + API key, the shared webhook token, the LLM API key, `API_BASE_URL`. Secret;
  never committed (an `.env.example` is).
- **`config.yaml`** — non-secret app setup and tunables: timeouts, retry/backoff, min/max values
  (rate limits, queue concurrency, page sizes), auth settings (session TTL, password policy), sync
  intervals, auto-response defaults, and the **seed: default organization + initial users**
  (email + password), upserted on startup.

Rule of thumb: **secrets → `.env`; behavior, limits, and seed data → `config.yaml`.** Both load at
startup; `.env` may override `config.yaml` for ops. Because the seed block contains passwords, a
populated `config.yaml` is treated as sensitive (gitignored; a `config.example.yaml` is committed).

## Apps And Components

### Evolution Core

Evolution API is the WhatsApp transport layer.

Responsibilities:

- connect WhatsApp accounts
- maintain WhatsApp sessions
- expose QR/connect status
- send text and media messages
- emit webhook events for contacts, chats, messages, status updates, and connection updates
- expose available historical chats/messages where possible

Evolution should not be treated as the main product database.

### Backend API

The backend is the source of truth for the product.

Suggested stack: Go + PostgreSQL.

Responsibilities:

- organization and member management
- WhatsApp account registration
- webhook receiver for Evolution events
- normalized contacts, conversations, and messages
- assignment/reassignment
- send-message API
- media metadata
- AI draft endpoints
- realtime event publishing
- idempotency, retries, and error tracking

### Workers

Workers process work that should not block user requests or webhook responses.

Responsibilities:

- process raw Evolution events
- sync old contacts/chats/messages
- download and store media
- update delivery/read statuses
- run AI draft jobs
- retry failed outbound sends
- reconcile gaps after downtime

Workers can initially run inside the same Go binary as background goroutines. They can later be split into separate processes if load grows.

### Realtime Gateway

The realtime layer pushes updates from the backend to connected UI clients.

Options:

- Server-Sent Events for a simple first version
- WebSockets for richer bidirectional realtime behavior

Events:

- `message.created`
- `message.updated`
- `conversation.created`
- `conversation.updated`
- `assignment.changed`
- `ai_draft.created`
- `whatsapp_account.status_changed`
- `sync.progress`

### UI

The UI is a lightweight Chatwoot-style team inbox.

Suggested stack: Vue 3 + TypeScript.

Core screens:

- login
- organization workspace
- WhatsApp accounts/settings
- conversation list
- chat view
- assignment control
- contact panel
- AI suggestions panel
- media picker

The first version should avoid complex helpdesk features such as SLA, campaigns, detailed permissions, macros, reports, billing, and omnichannel automation.

### AI Assistant

The AI assistant helps members respond faster.

Responsibilities:

- generate reply drafts
- suggest media files
- summarize long conversations
- suggest next action
- detect missing information
- optionally auto-reply only when explicitly enabled by rules

The assistant should read from the app database, not directly from Evolution tables.

### Database

PostgreSQL should store product state.

Core tables:

```text
organizations
users
organization_members
whatsapp_accounts
contacts
contact_identities
conversations
messages
message_media
conversation_assignments
ai_drafts
evolution_events
sync_jobs
```

Important constraints:

```text
unique(whatsapp_account_id, remote_jid) on conversations
unique(whatsapp_account_id, evolution_message_id) on messages
unique(contact_id, whatsapp_account_id, identity_type, value) on contact_identities
```

### Media Store

Media files should not live only inside Evolution.

The app should store:

- original Evolution media metadata
- local/object-storage URL
- MIME type
- size
- duration for audio/video
- thumbnail when available
- transcription when available
- download status

Local disk can be used for development. Object storage should be used for production. Both sit
behind a **blob-store interface** (`Put` / `Get` / `URL`); the local-disk and **S3 / MinIO**
implementations are selected by config — no caller changes.

> **Swappable stack (principle):** storage and the queue/bus are **ports with interchangeable
> adapters**, chosen by config. Defaults are lightweight and in-isolation friendly (local disk +
> Go channel / Postgres); production can switch to MinIO/S3 and Kafka without touching call sites.
> A shared adapter conformance test runs against every implementation.

### Queue / message bus (swappable)

Inbound processing and outbound sending flow through a **queue behind an interface**, so the
implementation is chosen by config — never hard-wired:

- v1: an in-process **Go channel** (simplest) or a **PostgreSQL-backed job table** (durable —
  default for work that must survive restarts).
- later: **Kafka / NATS / Redis** for higher throughput or cross-process fan-out — same interface,
  no caller changes.

## Suggested Deployment Shape

Simple v1:

```text
frontend
backend-api
backend-worker
postgres
object-storage
evolution-api
```

For development, `backend-api` and `backend-worker` can be one process.

## Data Ownership

Evolution owns WhatsApp transport details.

The app owns:

- organizations
- members
- connected account configuration
- contacts
- conversations
- assignments
- normalized messages
- media references
- AI drafts
- sync state
- raw event history

