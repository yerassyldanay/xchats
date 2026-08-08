# Docker Compose stack

`deploy/docker-compose.yaml` runs two services — no database container,
no message-queue container, no external WhatsApp gateway. SQLite lives on a
volume; WhatsApp is a direct connection (`go.mau.fi/whatsmeow`) from inside
the backend process.

## Services

**`backend`** — built from `backend/Dockerfile` (`golang:1.25` build stage →
`gcr.io/distroless/static-debian12` runtime; `CGO_ENABLED=0`, since
`modernc.org/sqlite` is a pure-Go SQLite driver — no `libsqlite3` to install
in the runtime image). Listens on `:8080` inside the container. Entrypoint is
the built binary; `CMD ["serve"]` supplies the default subcommand (see
[`backup-restore.md`](backup-restore.md) for the others — `backup`, `check`,
`restore` — run via `docker compose run` when needed).

**`frontend`** — built from `frontend/Dockerfile` (`node:22` build stage →
`nginx:1.27-alpine` runtime serving the static Vite bundle). `nginx.conf`
proxies `/xchats/*` to the backend service so the app and its API are
same-origin from the browser's perspective — no CORS round-trip for normal
use. Listens on `:80` inside the container.

## Ports

| Service  | Container port | Published (default)        | Override           |
|----------|-----------------|-----------------------------|---------------------|
| backend  | 8080            | `${BACKEND_PORT:-8080}`     | `BACKEND_PORT`      |
| frontend | 80              | `${FRONTEND_PORT:-8081}`    | `FRONTEND_PORT`     |

There is exactly one compose file and no `.env` to create — `make up` works
on a fresh clone with nothing exported. For a port already taken on your
machine, export `BACKEND_PORT`/`FRONTEND_PORT` for that run:

```bash
BACKEND_PORT=8090 make up
```

Those two are the only interpolated values left in the compose file. Every
application setting lives in [`deploy/config.docker.yaml`](../../deploy/config.docker.yaml),
which is mounted into the container as `/config.yaml`.

## Volumes

| Volume       | Mounted at       | Contents                                                        |
|--------------|-------------------|-------------------------------------------------------------------|
| `sqlitedata` | `/data`           | `xchats.db` (+ WAL/SHM), `whatsmeow.db`, and (see below) `appdata`/`appconfig` |
| `blobdata`   | `/data/blob`      | Uploaded/received media (images, documents, voice notes)         |

Both are named Docker volumes — `docker compose down` leaves them intact;
only `docker volume rm` (or `down -v`) destroys them. See
[`data-locations-and-privacy.md`](data-locations-and-privacy.md) for what
each file inside them actually is, and
[`backup-restore.md`](backup-restore.md) before you ever consider removing
one.

## Configuration

`deploy/config.docker.yaml` is mounted read-only at `/config.yaml` and
selected via the backend's `-config` flag baked into how `xchats serve`
resolves its config path (`$XCHATS_CONFIG` → `./config.yaml` → the OS config
dir; the compose file's `XCHATS_CONFIG=/config.yaml` env var wins first).
It's the same schema as the repo-root `config.yaml`, with paths pointed at
the container's mounted volumes instead of a local checkout
(`db_path: /data/xchats.db`, `blob_dir: /data/blob`, ...).

The `backend` service's own `environment:` block in `docker-compose.yaml`
carries only what genuinely needs a per-deployment override at container-run
time — not secrets, not LLM/provider config (all of that is auto-provisioned
or lives in the Settings UI, see [`credentials.md`](credentials.md)):

- `XCHATS_ALLOW_FILE_CREDENTIALS=1` — a container has no desktop session for
  an OS keychain to run in, so this opts into the file-backed credential
  store fallback by default. See [`credentials.md`](credentials.md).
- `XCHATS_DATA_DIR` / `XCHATS_CONFIG_DIR` — pinned under the persistent
  `/data` volume rather than the container's own ephemeral `$HOME`.
- `XCHATS_CONFIG=/config.yaml` — selects the mounted config file.

That is the whole list, and deliberately so. Origin plumbing
(`api_base_url`, `frontend_base_url`, `cors_origins`), Telegram mode and
webhook origin, and the simulator gate all live in `config.docker.yaml`
instead. They must NOT be re-declared as environment variables here:
`internal/config.Load` applies env *after* yaml, so a compose-level
`VAR: "${VAR:-default}"` is always set and would silently override the file,
which is what previously made `config.docker.yaml` dead weight.

Every one of those settings still accepts an environment override at run
time (the `env:` struct tags are unchanged) — that escape hatch is for
`docker run`/orchestrator deployments, not for the committed compose file.

## Common operations

```bash
make up          # build + start detached
make up-fg       # same, foreground (Ctrl-C to stop)
make down        # stop + remove containers (volumes survive)
make logs        # tail logs from both services
make ps          # container status
make kill-ports  # free host ports a crashed prior run left bound
```

Or drive Compose directly — the Makefile's `COMPOSE` variable is just
`docker compose -p xchats -f deploy/docker-compose.yaml` (plus the override
file, if present):

```bash
docker compose -p xchats -f deploy/docker-compose.yaml run --rm backend check
```

## Running a CLI subcommand against the Docker stack

The backend image's entrypoint is the `xchats` binary itself, so any
subcommand (`backup`, `check`, `restore`, `migrate`) runs the same way —
substitute the container's own config/volume paths, not your host's:

```bash
docker compose -p xchats -f deploy/docker-compose.yaml run --rm backend backup /data/backup.db
```

`restore` additionally requires the target not be open by a running
`backend` container (see [`backup-restore.md`](backup-restore.md)) — stop
the stack (`make down`) first.
