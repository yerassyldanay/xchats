# Installation

Two ways to run xchats: Docker Compose (recommended — one command, nothing
else to install) or from source (Go + Node, for local development). Both
produce the same app: a Go backend (`:8080`) and a Vue frontend (`:8081` in
Docker, `:5173` in dev).

## Option A — Docker Compose

**Prerequisites:** Docker with the Compose plugin (`docker compose version`).
Nothing else — the backend and frontend images are built from source on
first `up`.

```bash
make up
```

This builds and starts both services detached. Default addresses:

- Frontend: http://localhost:8081
- Backend API: http://localhost:8080

Both are overridable (`BACKEND_PORT`/`FRONTEND_PORT`, a root `.env`, or a
local `deploy/docker-compose.override.yaml` — gitignored on purpose, for
machine-specific remaps when a default port is already taken) — see
[`docker.md`](docker.md).

Other useful targets: `make down` (stop), `make logs` (tail), `make ps`
(container status), `make kill-ports` (free ports a crashed prior run left
bound). Run `make help` for the full list.

## Option B — from source (local dev)

**Prerequisites:** Go 1.25+, Node 22+.

```bash
make dev-backend    # backend on :8080 (go run ./cmd/xchats -config ../config.yaml)
make dev-frontend   # frontend on :5173 (vite dev server)
```

Open http://localhost:5173 — Vite proxies API calls to the backend. Both
commands run in the foreground; use two terminals, or `make up` if you'd
rather not manage two processes yourself.

## First run

There is no `.env` file to create and nothing to configure before booting —
see [`credentials.md`](credentials.md) for what "nothing to configure"
actually means under the hood. On first boot:

1. Migrations run automatically (`internal/store.New` applies every pending
   migration on open — embedded in the binary, nothing to run by hand).
2. Migration `0006_init_admin` seeds one admin login on an empty database:
   `admin@xchat.kz` / `xchat-admin-change-me`.
3. Four internal secrets (session signing, Telegram token encryption, MCP
   signing, the Telegram webhook secret) are generated and durably stored —
   see [`credentials.md`](credentials.md).

Log in with the seeded admin account, then:

1. **Change the bootstrap credential.** There is no self-service password
   change yet — add your own admin account from **Settings → Team
   Management** and treat `admin@xchat.kz` as a bootstrap login to retire,
   not a permanent one.
2. **Complete the first-run setup wizard** (shown automatically to an admin
   until dismissed): pick an LLM provider and paste its API key. Every other
   integration (ngrok, Langfuse, additional providers) is optional and
   configurable later from **Settings**.
3. **Pair WhatsApp.** `/accounts` → **add** → scan the QR code with the
   WhatsApp number you want to connect. xchats talks to WhatsApp directly via
   [`whatsmeow`](https://github.com/tulir/whatsmeow) — no separate gateway
   service to run.
4. **Optionally connect Telegram.** Configure a bot token from **Settings →
   Integrations**; see [`troubleshooting.md`](troubleshooting.md) for the
   webhook-vs-polling mode decision.

## Verifying it's running

```bash
curl -s http://localhost:8080/xchats/api/v1/settings/update-check
```

A JSON `{payload: {...}, errcode: "", message: ""}` envelope back means the
backend is up and answering. The frontend's own health is simplest to check
by loading it in a browser — it should redirect to `/login` when logged out.

## Next steps

- [`docker.md`](docker.md) — the Compose stack in detail: ports, volumes,
  environment variables.
- [`credentials.md`](credentials.md) — how secrets and provider API keys are
  stored, and how to override them for a specific deployment.
- [`data-locations-and-privacy.md`](data-locations-and-privacy.md) — exactly
  what xchats writes to disk and where.
- [`backup-restore.md`](backup-restore.md) — before you put real data in,
  know how to get it back out.
