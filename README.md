<div align="center">

# xchats

**A shared team inbox for WhatsApp, Telegram, Instagram and Messenger, with an
AI assistant that drafts every reply from your own knowledge base — and never
sends one without a human.**

[![License: AGPL v3](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![CI](https://github.com/yerassyldanay/xchats/actions/workflows/ci.yml/badge.svg)](https://github.com/yerassyldanay/xchats/actions/workflows/ci.yml)
[![CodeQL](https://github.com/yerassyldanay/xchats/actions/workflows/codeql.yml/badge.svg)](https://github.com/yerassyldanay/xchats/actions/workflows/codeql.yml)

**English** · [Русский](README.ru.md) · [Қазақша](README.kk.md)

</div>

xchats is self-hosted software you run yourself. It is one Go binary and one
SQLite file — no database server, no cloud account, no per-message fee.

![The xchats inbox: conversations on the left, the thread in the middle, and the assistant's suggested reply on the right waiting for approval](docs/images/inbox.png)

---

## How it works

1. A customer messages you on WhatsApp, Telegram, Instagram or Messenger.
2. Everything lands in **one inbox**, shared by your whole team.
3. The assistant writes a **draft reply** using only facts you approved.
4. A person reads the draft and presses send — or edits it, or throws it away.

**The assistant is never told your prices.** It writes replies containing
placeholders like `{{product.coffee.price}}`; the backend then substitutes the
real stored value. A wrong number is therefore not a risk you mitigate, it is a
bug that cannot occur. If your approved knowledge doesn't cover the question,
the assistant escalates to a human instead of guessing.

![How a reply is built: a message arrives, a prompt is assembled from approved knowledge with exact values masked as placeholders, the model returns strict JSON, code validates every token and substitutes the stored values, and a human approves before sending](docs/images/grounding.svg)

The whole thing is one Go backend ([`backend/`](backend/)) serving the API, the
channel adapters and the MCP server; one Vue 3 frontend
([`frontend/`](frontend/)); and one SQLite file. The full design is in
[`plan/architecture.md`](plan/architecture.md).

---

## Run it

Pick one of three ways. All three run the same app.

### 1. Docker — one command, nothing to install

```bash
git clone https://github.com/yerassyldanay/xchats.git
cd xchats
make up
```

Open **http://localhost:8081**. (The API is on `:8080`.) There is no `.env` to
create. `make down` stops it, `make logs` tails it.

> Ports busy? `BACKEND_PORT=9080 FRONTEND_PORT=9081 make up`.

### 2. From source — for development

Needs **Go 1.25+** and **Node 22+**. Two terminals:

```bash
# one-off after cloning — install the locked frontend dependencies
npm --prefix frontend ci

# terminal 1 — backend on :8080
XCHATS_ALLOW_FILE_CREDENTIALS=1 make dev-backend

# terminal 2 — frontend on :5173
make dev-frontend
```

Open **http://localhost:5173**.

> **`XCHATS_ALLOW_FILE_CREDENTIALS=1` is not optional on a machine with no OS
> keychain** — a server, a container, WSL, or any plain SSH session. Without it
> the backend still starts, but it has nowhere to put secrets: **saving an LLM
> API key fails**, Telegram accounts can't be connected, and the ngrok tunnel is
> off. You'll see `no secure credential store available` in the log at boot.
> On a macOS or Linux desktop with a logged-in keychain you can leave it out.
> The Docker stack sets it for you.

### 3. Desktop app — the same binary in a window

Windows, macOS and Linux, built with Wails. Nothing is published for download
yet, so build it yourself — see [`docs/desktop.md`](docs/desktop.md):

```bash
make desktop-tools     # one-off: install the pinned Wails CLI
make desktop-build     # → backend/cmd/xchats/build/bin/
```

---

## Your first ten minutes

Sign in with the seeded admin account:

| | |
|---|---|
| **Email** | `admin@xchat.kz` |
| **Password** | `xchat-admin-change-me` |

> ⚠️ **This password is public.** It is printed here and committed in this
> repo's migration history. Anyone can look it up.

Then, in this order:

**1. Change the password.** There is currently **no way to do this in the UI** —
the account menu has only "Log out". Use the API:

```bash
curl -c jar -s -X POST http://localhost:8080/xchats/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@xchat.kz","password":"xchat-admin-change-me"}'

curl -b jar -s -X POST http://localhost:8080/xchats/api/v1/auth/password \
  -H 'Content-Type: application/json' \
  -d '{"current_password":"xchat-admin-change-me","new_password":"YOUR-NEW-PASSWORD"}'
```

Locked out later? Stop the server and run `xchats reset-admin-password` — note
that this restores the *same public default* above, so change it again.

**2. Choose your language.** The app defaults to **Russian**, with complete
Russian, English and Kazakh catalogs. Switch from the public landing page, the
sign-in page, or the account menu after signing in; the preference is remembered
in this browser.

**3. Add an LLM key.** A setup wizard opens on first login and asks for an
**OpenRouter** key. OpenAI and Google Gemini work too — add either under
**Settings → AI Engine**, which is also where you pick the default model. Keys
are verified against the provider on save, so a typo is rejected on the spot
rather than failing silently later.

**4. Connect a channel.** **Channels → Connect a channel**, then pick one of the
five cards; each has its own three numbered steps.

**5. Fill the knowledge base.** **Knowledge Base** — products, tariffs, delivery
zones, contacts, policies, plus the assistant's persona and guardrails. Until
this has content the assistant has nothing to answer from and will escalate
every question to a human. To see what a filled one looks like, stop the server
and run `make seed-kb-demo`.

**6. Test it.** **Simulator** — ask the assistant a customer question without
touching a live account.

---

## Executables

Everything ships as **one binary**, `xchats`. The desktop app is that same
binary with a window attached; the Docker backend image is that same binary in a
container.

### Building it

```bash
make build      # → backend/bin/xchats  (server binary)
                # → frontend/dist       (web UI bundle)
```

| Artifact | Where it comes from | Where it lands |
|---|---|---|
| Server binary | `make build` | `backend/bin/xchats` |
| Web UI bundle | `make build` | `frontend/dist/` |
| Desktop app | `make desktop-build` | `backend/cmd/xchats/build/bin/` |
| Container images | GitHub Actions on a `v*` tag | `ghcr.io/yerassyldanay/xchats-backend`, `-frontend` |

Desktop archives (`xchats-desktop-linux-amd64.tar.gz`,
`-macos-universal.zip`, `-windows-amd64.zip`) are attached to GitHub Releases
built from a tag. The current `v0.1.0` release has no binaries attached — build
from source for now.

### `xchats` commands

Run with `-config <path>` (defaults to `$XCHATS_CONFIG`, then `./config.yaml`,
then the OS config directory). With no command it runs `serve`.

| Command | What it does |
|---|---|
| `serve` | Start the HTTP API, channel adapters and MCP server. **Default.** |
| `migrate` | Apply pending DB migrations. `serve` already does this at startup. |
| `seed` | Create the default organization and admin login. |
| `seed-kb-demo` | Insert demo knowledge-base content. No-op if the org already has some. |
| `check` | Run a database integrity check. |
| `backup <dest>` | Write a consistent snapshot to `<dest>`. |
| `restore <backup> <dest>` | Restore a snapshot. Offline; validates before touching `<dest>`. |
| `kb-load -file <json> [-remove]` | Load (or delete) knowledge-base records from a JSON file. |
| `simulate-message -token … -contact … -conversation … -text …` | Inject a message into a *running* server. |
| `reset-admin-password` | Reset the admin password to the public default above. |
| `admin-credential show` | Print the stored admin bootstrap password. |

> `xchats -help` lists only the `-config` flag — the commands above are not in
> its output. This table is the reference.

> **SQLite allows one process to hold the database.** `migrate`, `seed`,
> `seed-kb-demo`, `check`, `backup`, `restore`, `kb-load` and
> `reset-admin-password` all need it exclusively, so **stop the server first**
> or you get `database is already open by another process`. Only
> `simulate-message` (an HTTP client) and `admin-credential show` (a file read)
> work against a running instance. To back up a *live* instance use
> **Settings → Data & Backup → Download backup** instead.

### `make` targets

`make help` prints them all. The ones you'll actually use:

| Target | What it does |
|---|---|
| `make up` / `make down` | Start / stop the Docker stack (`:8081` UI, `:8080` API). |
| `make logs` / `make ps` | Tail logs / show container status. |
| `make dev-backend` | Run the backend from source on `:8080`. |
| `make dev-frontend` | Run the Vite dev server on `:5173`. |
| `make build` | Build the server binary and the web bundle. |
| `make seed-kb-demo` | Load demo knowledge-base content (stop the server first). |
| `make test` | Backend + frontend test suites. |
| `make lint` | Every linter CI runs. |
| `make kill-ports` | Free `:8080 :8090 :5173 :8081` after a crashed run. |
| `make desktop-tools` / `desktop-build` | Install the Wails CLI / build the desktop app. |

---

## What's in the app

| Page | What it's for |
|---|---|
| **Inbox** | Every conversation from every channel, with the assistant's draft beside each thread. Assign to a teammate, resolve, or reply by hand. |
| **Customers** | A contact record per person: status, tags, owner, notes, and a timeline of everything that happened. |
| **Follow-ups** | Scheduled next steps, bucketed overdue / today / tomorrow / this week. |
| **Channels** | Connect and monitor accounts. Per channel: automation mode and debounce. |
| **Campaigns** | Bulk outbound to a pasted or uploaded recipient list, rate-limited and confined to a send window. Replies come back to the Inbox. |
| **Draft** | Staging area for knowledge-base edits — shows exactly what publishing would add, update and remove. Also where you import from a URL or a document. |
| **Knowledge Base** | The live, approved facts the assistant answers from. |
| **Simulator** | Try the assistant against a question with no live account attached. |
| **Settings** *(admin)* | LLM provider and model, parsers, monitoring, ngrok, channels, team and roles, backups. |

### Channels

| Channel | How it connects |
|---|---|
| **WhatsApp** | Scan a QR code, like WhatsApp Web. No Meta approval, no Business API fee. |
| **Telegram** | Paste a bot token from [@BotFather](https://t.me/BotFather). Webhook or polling, chosen automatically. |
| **WhatsApp Cloud API** | Meta OAuth — the official API, for a supported number and templates. |
| **Instagram Direct** | Meta OAuth. DMs to a professional Instagram account. |
| **Facebook Messenger** | Meta OAuth. DMs to a Facebook Page. |

The three Meta channels need your own Meta Developer App ID and secret, entered
once under **Channels → Channel setup**, and a public HTTPS address for their
webhooks. If you don't have a hostname, the built-in ngrok tunnel supplies one
and every webhook URL is derived from it automatically.

Per channel you choose how much the assistant is allowed to do: **off** (no
drafts), **suggestions** (draft, never send — the default), or **scheduled
auto-send** inside a recurring time window, falling back to suggestions outside
it.

> ⚠️ **The WhatsApp QR channel is unofficial.** It uses
> [whatsmeow](https://github.com/tulir/whatsmeow), a reverse-engineered client —
> not WhatsApp's Business API. A connected number can be banned at Meta's
> discretion with no recourse. Start with a test number, or use the Cloud API
> row instead.

### Also in the box

- **MCP connector** — point ChatGPT or Claude at your knowledge base and edit it
  from the chat client you already use. OAuth 2.1 + PKCE, no shared API key.
  Set it up under **Draft → ChatGPT / Claude**.
- **Evaluation harness** ([`evals/`](evals/)) — runs the assistant over a curated
  scenario set and grades the output, so a prompt or model change is measured
  rather than guessed at.

---

## Configuration

Two places, deliberately:

- **[`config.yaml`](config.yaml)** — committed, and free of secrets. Only what
  the app needs to boot: listen address, database and blob paths, log level,
  worker count, default debounce, webhook base URLs. Any value can be overridden
  by an environment variable of the matching name — that's how Docker and
  Kubernetes pin them. Docker uses
  [`deploy/config.docker.yaml`](deploy/config.docker.yaml) instead.
- **The app itself** — everything an operator actually changes. **Settings**
  holds the LLM provider and key, ngrok, Langfuse, your team and their roles, and
  backups; **Channels** holds the Telegram bot token and Meta app credentials.

Session signing, credential encryption, MCP signing and the Telegram webhook
secret are generated on first boot and kept in the credential store. You never
write one down.

Your data lives in SQLite plus a blob directory — `./data/` and `./blobdata/`
from source, Docker volumes under `make up`. See
[`docs/release/data-locations-and-privacy.md`](docs/release/data-locations-and-privacy.md).

---

## Documentation

- [`docs/release/installation.md`](docs/release/installation.md) — every install path in detail
- [`docs/desktop.md`](docs/desktop.md) — the desktop app
- [`docs/release/`](docs/release/) — deploying, credentials, backups, upgrades, troubleshooting
- [`plan/`](plan/) — the design record, starting at [`plan/README.md`](plan/README.md)
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — dev setup, conventions, how to open a PR
- [`SECURITY.md`](SECURITY.md) — reporting a vulnerability

## License

[AGPL-3.0-only](LICENSE). Chosen after a dependency review found a GPL-3.0
dependency ([`go.mau.fi/libsignal`](https://github.com/tulir/libsignal-protocol-go),
pulled in transitively through whatsmeow) statically linked into the backend
binary — see [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md). GPL-3.0 would
also have been compatible; AGPL-3.0 was chosen so that running a modified
version as a network service carries the same share-back obligation as
distributing it does.
