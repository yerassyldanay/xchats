# Isolated Development & Testing

How to develop and verify the whole app — and each element — in an **isolated environment** (a
sandbox / CI / the Claude Code remote environment, with no real WhatsApp and no external network)
and still trust the real run.

## Principle: pin the external contract to real captured data

The only true external is Evolution. Its REST responses and webhook payloads are pinned to **real
captures** (`captures/`), so we reproduce them deterministically. Everything else is either
real-but-local (Postgres) or a controllable fake (Evolution, the LLM).

## Test doubles & locals

- **Fake Evolution** — a small HTTP stub implementing the endpoints we call (`instance/create`,
  `instance/connect`, `message/sendText`, `message/sendMedia`,
  `getBase64FromMediaMessage`) with recorded responses, that can also **POST captured webhook
  events** at our webhook edge. It **records what we sent** so tests can assert send shapes
  (e.g. that we send to the phone, not the `@lid`). Today this lives **in-process** in the Go tests
  (`httptest`); see *Evolution simulator* below for promoting it to a standalone, runnable service.
- **Fake LLM** — the assistant falls back to a hardcoded **stub** drafter whenever `LLM_API_KEY` is
  empty, returning fixed 1–3 options, so AI-draft tests are deterministic and free.
- **Real local Postgres** — the **same engine as prod**, a throwaway DB/schema, so SQL runs for real
  (see *Database: Postgres, not SQLite* for why we don't substitute a different engine).
- **Default in-proc adapters** — local-disk blob store + the in-memory Go-channel queue (behind the
  `Queue` port).

## Layers of tests

- **Unit** — `normalize` against captured payloads (text/media, `@lid`↔phone, status 0–5); pure,
  no I/O.
- **Component** — the webhook handler (enqueue to in-memory queue + 200) and each worker
  (consume + dedup + upsert; media, status, send) against the fakes + local Postgres.
- **End-to-end** — drives the full loop: replay webhook → assert normalized rows
  (`wa_contacts`/`wa_chats`/`wa_messages`) → call send API → assert the fake Evolution received the
  right call → replay status → assert sent→delivered→read → media path → AI draft.
- **Frontend** — `npm run build` (`vue-tsc` typecheck + Vite build) is the current gate. A component
  runner (**Vitest**) and a browser **Playwright** suite against the backend wired to the fakes /
  simulator are **planned, not yet wired** (no test runner in `frontend/package.json` today).

## One command (today vs. the goal)

**Today** `make test-e2e` runs the Go end-to-end suite against a Postgres you point it at, with the
Evolution + LLM fakes **in-process** (no external network, no Docker required):

```bash
# the only external dependency is a Postgres reachable via DATABASE_URL
DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/xchats?sslmode=disable' make test-e2e
#  -> go test -p 1 -count=1 ./internal/httpapi/ ./internal/kbstore/ ./internal/playground/
#     (these packages each reset the shared xchats schema, so they run serially)
```

The DB-touching tests **skip** when `DATABASE_URL` is unset, so `make test-backend` (`go test ./...`)
is safe with no DB. The fixtures are byte-for-byte real Evolution output, so a green run proves
**normalizer/transport parity on the captured events**.

**Goal (not built yet):** a single containerized harness `deploy/compose.test.yaml`
(= postgres + backend + a standalone fake-evolution + fake-llm) that migrates, runs the suite, and
tears down — for hosts that have Docker. It does **not** exist today; on the Claude environment there
is no Docker daemon (see below), so the Go-suite-against-local-Postgres path above is the real one.

## Running & testing in the Claude Code (remote) environment

This is the practical runbook for *this* sandbox. Everything needed to **build and functionally
verify** the app runs here; only a live WhatsApp round-trip and Docker orchestration do not.

| Component | Runs here? | How |
|---|---|---|
| Frontend (Vite) | ✅ process | `make dev-frontend` (`:5173`, proxies `/xchats`→backend) / `npm run build` |
| Backend (Go) | ✅ process | `make dev-backend` (`:8080`); Go `1.25` toolchain auto-downloads on first build |
| **Postgres** | ✅ **local** | a cluster is **pre-installed** (v16); start it — ~1–2 s, ~80 MB on disk, tens of MB RAM |
| Evolution | ⛔ live / ✅ faked | in-process fake for tests; live needs an **external** Evolution **+ a phone to scan the QR** |
| LLM | ✅ stub / ➜ external | empty `LLM_API_KEY` ⇒ stub; a real key calls out (egress permitting) |
| Docker stack (`make up`) | ❌ | **no Docker daemon** in the container (`/var/run/docker.sock` absent) |

**Bootstrap (provision the local DB, then run):**

```bash
pg_ctlcluster 16 main start                              # start the pre-installed cluster
sudo -u postgres psql -c "ALTER USER postgres PASSWORD 'postgres';"
sudo -u postgres createdb xchats                         # idempotent
export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/xchats?sslmode=disable'

make test-e2e                                            # e2e suite (tests manage their own schema)
# — or run the live app —
make migrate && make seed                                # schema + org + admin + seeded WA account
make dev-backend &                                       # :8080
make dev-frontend                                        # :5173, log in with SEED_ADMIN_* from .env
```

**Verification ladder (what can be proven here, strongest last):**

1. `make test-frontend` + `go build ./...` — compile/typecheck (FE + BE).
2. `make test-backend` — Go unit/component (DB tests skip without `DATABASE_URL`).
3. `make test-e2e` — full backend loop vs **local Postgres**, Evolution + LLM faked: inbound→draft→
   approve→send→status, accounts, instances, auth, KB/playground.
4. Run the processes and exercise the live API with `curl`, replaying `captures/` webhooks to populate
   chats — manual functional checks against the running app.
5. **UI end-to-end** — *gap:* add **Playwright** to drive the browser headless, log in, fire
   simulated WhatsApp traffic, and assert UI flows. Planned.
6. **Real WhatsApp send/receive** — *out of scope here* (needs external Evolution + a human QR scan).
   The closest autonomous proof is asserting we emit the **correct transport calls** to the fake
   (which the e2e already does: "1 sendText, 2 sendMedia, to the phone not the `@lid`").

**Constraints to design around:**

- **Ephemeral** — the local Postgres and anything `apt`-installed vanish when the container is
  reclaimed; re-provision per session (a SessionStart hook can automate the bootstrap above).
  Persist nothing but committed code; for durable infra, point `DATABASE_URL` at a DB **you** host.
- **Network policy** — outbound egress is governed by the environment's network policy (chosen at
  creation; see https://code.claude.com/docs/en/claude-code-on-the-web). Observed here: the Go module
  proxy, GitHub, and the main Ubuntu archive are reachable; some third-party apt PPAs are blocked.
- **Credentials** — put any provided secrets (DB URL, Evolution URL/key, LLM key) in `.env`
  (gitignored) or env vars; never commit or echo them. With them, tests/app can target your real
  services instead of the local/faked ones — reachability still subject to the network policy.
- **Current status (at time of writing):** the suite is green against local Postgres **except** a
  pre-existing `TestDemoLoop` assertion (`fan-out: want 3 messages, got 2`) — deterministic and
  unrelated to the frontend work; flagged here so a first run isn't surprising.

## Database: Postgres, not SQLite (decision)

We **keep Postgres** for tests and dev rather than substituting SQLite. The deciding factors:

- **Postgres is cheap here**, not heavy — it's pre-installed, starts in ~1–2 s, and the cluster is
  ~80 MB. The friction we hit was *lifecycle* (it isn't running by default in an ephemeral box), which
  the bootstrap above fixes in three commands — not a reason to change engines.
- **The store is not abstracted.** `store.Store` is a concrete `pgx` struct (no `Store` interface;
  `httpapi`/`worker` hold `*store.Store` directly), and the schema is **Postgres-specific**: a
  dedicated `xchats` schema (≈177 qualified refs), `citext`, `uuid-ossp` + `uuid_generate_v4()`
  column defaults, `jsonb` (≈25), `timestamptz`/`time`. Swapping engines is a **real port** (introduce
  a port interface + a second SQL dialect + schema/UUID/jsonb/citext/time rewrites), not a config flip.
- **Parity is the point.** "Real local Postgres — the same engine as prod" is what makes a green
  suite trustworthy; SQLite would validate a *different* engine and could hide Postgres-only bugs.

If a hard driver ever appears (run on a tiny/offline box, or instant zero-dependency unit tests), the
correct shape is a **`Store` port with a SQLite adapter as a dev/test-only backend** (Postgres stays
the prod + CI engine, with dual-run CI to catch divergence) — never a wholesale replacement. A
lighter alternative that keeps full parity is **embedded-postgres** (a throwaway PG binary run
in-process, no server to manage).

## Evolution simulator (standalone — planned)

The in-process fake (above) is test-only. The plan is to **promote it to a standalone, runnable
simulator** so it can drive the **live** app + UI (dev / demo / Playwright), not just Go tests —
without a real Evolution or a phone. Because addressing is env-driven, it needs **no production code
changes**: point `EVOLUTION_BASE_URL` at the simulator, and the simulator POSTs to
`WEBHOOK_PUBLIC_BASE_URL`. It has two halves plus a control surface:

- **REST surface the backend calls** — `message/sendText`, `message/sendMedia`,
  `getBase64FromMediaMessage`, `instance/create|connect|connectionState|delete|logout`,
  `instance/fetchInstances`, `webhook/set`, `chat/whatsappNumbers` (for the add-account QR flow:
  return a fake QR, then flip `connectionState` to `open`).
- **Webhook emitter** — POST realistic events at our webhook: inbound text/media
  (`messages.upsert`), status transitions (`messages.update`: sent→delivered→read), and
  `connection.update` / QR.
- **Control API** — a small endpoint/script (e.g. "contact X sent 'hi'", "mark last delivered/read")
  so tests and Playwright can choreograph scenarios.

**Fidelity caveat:** it reuses `captures/` where possible, but the capture set is incomplete (missing
the core inbound `messages.upsert`, the `getBase64` response, and a matched send→`messages.update`
pair — see `captures/README.md`). For dev/demo the simulator synthesizes plausible events; full
contract trust still requires capturing those real events.

## Developing in isolation

Run the same stack and iterate: env points the backend at local Postgres + the fake/simulator
Evolution; exercise endpoints with the captured payloads; watch logs. Because **addressing is
env-driven**, the only difference from prod is which hosts/ports the env points at.

## Stack (v1 = one implementation per boundary)

v1 ships **one adapter each** behind the blob and queue ports (see `2-architecture.md`):

- blob: `local-disk` only (media bytes are a deferred surface anyway — v1 renders placeholders).
- queue/bus: the **in-memory `inmem` driver** only (a buffered Go channel + worker pool behind the
  `Queue` port; there is no Postgres `jobs` / `evolution_events` table).

**Deferred (v2+):** the `minio` blob adapter, the `redis`/`kafka` queue drivers (selected via
`QUEUE_DRIVER`), and the **adapter conformance suite** that runs across implementations. There is no
second adapter to conform to in v1 — don't build it until one exists.

## Real smoke (manual, outside isolation)

A tiny optional check against the live Evolution (`localhost:9700`) confirms reachability +
credentials. The fixtures are byte-for-byte real Evolution output, so a green isolated suite proves
**normalizer/transport parity on the captured events**. Full contract trust requires the complete
live event set — and the current `captures/` set is missing the core inbound `messages.upsert`, the
`getBase64` response, and a matched send→`messages.update` pair (see `captures/README.md`). Until
those are captured, "green" means "correct on what we've captured", not "the whole contract is
proven".
