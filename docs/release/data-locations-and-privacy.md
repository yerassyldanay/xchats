# Data locations and privacy

xchats is self-hosted: your data stays on whatever disk/volume you point it
at, and the only outbound network calls it makes on its own are the small
set documented below. This page is a factual inventory, not a legal privacy
policy — see `proposals/PRIVACY-proposal.md`
for a starting point toward one.

## Where things live

Two roots, resolved by `internal/appdirs`, both override-first via
environment variable so a container or an unusual host setup can redirect
either without touching code:

| Root         | Override             | Linux (XDG)                  | macOS                                   | Windows            |
|--------------|------------------------|-------------------------------|-------------------------------------------|----------------------|
| Config dir   | `$XCHATS_CONFIG_DIR`   | `$XDG_CONFIG_HOME/xchats` (else `~/.config/xchats`) | `~/Library/Application Support/xchats`   | `%APPDATA%\xchats` (roaming) |
| Data dir     | `$XCHATS_DATA_DIR`     | `$XDG_DATA_HOME/xchats` (else `~/.local/share/xchats`) | `~/Library/Application Support/xchats` (same as config — macOS doesn't split the two) | `%LOCALAPPDATA%\xchats` (local, not roaming — secrets/settings should not silently follow a roaming profile) |

In the Docker Compose stack both env vars are pinned under the persistent
`/data` volume (`XCHATS_DATA_DIR=/data/appdata`,
`XCHATS_CONFIG_DIR=/data/appconfig`) rather than the container's own
ephemeral `$HOME` — see [`docker.md`](docker.md). Every directory
`internal/credentials`/`internal/settings` create are `0700` (owner-only).

## What's stored, and where

| File                                  | Location                              | Contents                                                                 |
|-----------------------------------------|-----------------------------------------|-----------------------------------------------------------------------------|
| `xchats.db` (+ `-wal`/`-shm` sidecars)   | `storage.db_path` (config.yaml)         | Everything except WhatsApp's own device/session state and blob bytes: chats, messages, KB content, users/orgs, MCP OAuth grants, Telegram poll offsets, etc. |
| `whatsmeow.db`                          | `storage.wa_device_db_path`             | WhatsApp's own end-to-end-encryption session/device keys (`whatsmeow`'s SQLite store) |
| Blob files                              | `storage.blob_dir`                      | Media bytes: photos, documents, voice notes sent or received over WhatsApp/Telegram |
| `credentials.enc` + `credentials.key`   | data dir (file-backed credential store only) | Encrypted system secrets + integration API keys — see [`credentials.md`](credentials.md) |
| `settings.json`                         | data dir                                | Non-secret settings: default LLM provider/model, ngrok region/domain, onboarding flags |

`storage.db_path`/`wa_device_db_path`/`blob_dir` are set in `config.yaml` —
by default, relative paths under the working directory (`./data/xchats.db`,
`./blobdata/`) for a source checkout, or `/data/...` volume paths in Docker.
None of the config.yaml defaults are secrets, which is why that file is
committed to the repo (see its own header comment).

## What's NOT collected or sent anywhere

xchats has no telemetry of its own — no analytics SDK, no usage
beaconing, no crash reporter phoning home. The only outbound calls the
backend makes without an operator explicitly configuring an integration are:

- **The update checker** (`internal/updatecheck`) — an unauthenticated `GET
  https://api.github.com/repos/yerassyldanay/xchats/releases/latest` on
  Settings page load, cached for an hour. This sends nothing about your
  deployment — no version, no identifier, no usage data — it only reads
  GitHub's public releases list. See its result surface at **Settings →
  Data & Backup**.
- **Whatever LLM/tunnel/tracing provider you configure** — OpenRouter,
  OpenAI, Gemini, ngrok, Langfuse — each only ever contacted for the calls
  that integration exists to make (a chat completion, a tunnel connection, a
  trace export), never anything beyond what the feature you turned on
  implies. All are opt-in: none are contacted until you save a credential
  for them.

Message content and any KB material you build necessarily leaves your
deployment when you configure an LLM provider — that's the provider making
the inference call your feature needs. Which fields of a conversation
actually get sent in that prompt is a product-logic question outside this
document's scope; see `plan/knowledge-base.md` and `plan/DECISIONS.md`.

## Backups carry the same data (minus credentials)

A backup (CLI or the Settings UI's one-click download) bundles the database
snapshot, blob files, and non-secret settings — see
[`backup-restore.md`](backup-restore.md). It **deliberately never** includes
`credentials.enc`/`credentials.key`, so a backup file leaking is not the same
severity event as a credential-store leak; the two need to be protected
(and rotated, if compromised) independently.

## Deleting everything

Stop the app, then remove the data dir, config dir, and (Docker) the
`sqlitedata`/`blobdata` volumes. There is no separate "purge my data" button
in the UI today — this is a filesystem-level operation.
