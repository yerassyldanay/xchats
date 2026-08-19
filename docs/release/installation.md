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

There is no `.env` to create and no override file to add — `make up` on a
fresh clone is the whole install. If one of those ports is already taken,
export `BACKEND_PORT`/`FRONTEND_PORT` for that run (e.g.
`BACKEND_PORT=8090 make up`); everything else is configured in
[`deploy/config.docker.yaml`](../../deploy/config.docker.yaml). See
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
   migration on open — embedded in the binary, nothing to run by hand),
   including seeding the default admin account below.
2. Four internal secrets (session signing, Telegram token encryption, MCP
   signing, the Telegram webhook secret) are generated and durably stored —
   see [`credentials.md`](credentials.md).

Log in with the default admin account:

```
email:    admin@xchat.kz
password: xchat-admin-change-me
```

1. **Change the default password.** It's public — printed in this repo's
   README and committed in its migration history — so treat it as already
   compromised. There's no persistent in-app "change password" screen (the
   old forced-first-login flow no longer triggers), so change it via the
   API instead:
   ```bash
   curl -c jar -s -X POST http://localhost:8080/xchats/api/v1/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"email":"admin@xchat.kz","password":"xchat-admin-change-me"}'
   curl -b jar -s -X POST http://localhost:8080/xchats/api/v1/auth/password \
     -H 'Content-Type: application/json' \
     -d '{"current_password":"xchat-admin-change-me","new_password":"<a strong password>"}'
   ```
   Locked out later on? `xchats reset-admin-password` (Docker:
   `docker compose exec backend /xchats reset-admin-password`), then
   restart, restores this same default password.
   `XCHATS_BOOTSTRAP_ADMIN_PASSWORD` overrides what gets restored through
   that recovery path — it has no effect on a fresh install, since
   migrations already leave a real password hash in place before that env
   var is ever consulted.
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
