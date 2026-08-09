# Credentials and secrets

xchats has no `.env` file and nothing to configure before first boot. This
document explains what's generated automatically, what's operator-configured
(and where), and how to override either for a specific deployment.

## Two different kinds of secret

**System secrets** (4 of them) are internal plumbing — signing keys and
encryption keys the app needs for itself, never shown in the UI because
there's nothing for an operator to *do* with them:

| Key                          | Purpose                                                | Legacy env var (adopted once, see below) |
|-------------------------------|---------------------------------------------------------|--------------------------------------------|
| `system.session_secret`       | HMAC key signing the MCP OAuth CSRF token                | `SESSION_SECRET`          |
| `system.tg_credentials_key`   | AES-256-GCM key protecting stored Telegram bot tokens at rest (`internal/secretbox`) | `TG_CREDENTIALS_ENC_KEY` |
| `system.mcp_jwt_key`          | Seeds the Ed25519 keypair signing MCP OAuth access/refresh tokens | `MCP_JWT_SIGNING_KEY`    |
| `system.webhook_secret`       | The `secret_token` registered with Telegram's `setWebhook` and checked on every inbound webhook call | `TG_WEBHOOK_SECRET` |

`internal/credentials.Provision` resolves each on every boot: already in the
credential store → use it (idempotent across restarts). Not yet stored, but
the legacy env var is set → adopt it once, write it to the store, and never
need the env var again. Neither → generate 32 random bytes (64 hex chars)
and store that. This means an existing deployment upgrading from an
env-var-only version keeps working unchanged, and a brand-new deployment
never needs to think about these four values at all.

**Integration credentials** are what an operator actually manages, from
**Settings → Integrations**:

| Provider     | Fields                                    | Validated live? |
|--------------|--------------------------------------------|------------------|
| OpenRouter   | `openrouter.api_key`                        | Yes (`GET /key`) |
| OpenAI       | `openai.api_key`                            | Yes (`GET /v1/models`) |
| Google Gemini| `gemini.api_key`                            | Yes (`GET /v1/models`) |
| Langfuse     | `langfuse.public_key`, `langfuse.secret_key`| Yes (`GET /api/public/projects`) |
| ngrok        | `ngrok.authtoken`                           | No live check — the only real test is opening a tunnel with it |
| Firecrawl    | `firecrawl.api_key`                         | Yes (`GET /v2/team/credit-usage`) |
| LlamaParse   | `llamaparse.api_key`                        | Yes (`GET /api/v1/parsing/supported_file_extensions`) |

Firecrawl and LlamaParse are the structured knowledge base import pipeline's
document/URL *extraction* providers, configured from their own **Settings →
Парсеры и краулеры** tab — separate from the response-drafting model
providers under AI Engine — they never register an LLM client, so they can
never accidentally become a chat/draft model choice.

Saving a credential that fails validation with a clear rejection (401/403,
or Gemini's `API_KEY_INVALID` body on a 400) is refused outright. One that
can't be *checked* right now (network trouble, provider outage) is saved
only if you explicitly confirm ("save anyway") — a credential that's merely
unverifiable is never silently treated as invalid, and is never silently
force-saved either.

## Where credentials are actually stored

`internal/credentials.Open` picks a backend, in order:

1. **The OS keychain** (`zalando/go-keyring` — Keychain on macOS, Secret
   Service on Linux, Credential Manager on Windows), if a real round-trip
   probe proves it's usable. Preferred whenever a desktop session exists.
2. **A file-backed store**, only if `XCHATS_ALLOW_FILE_CREDENTIALS=1` is set
   — never chosen silently. A container has no desktop session for a
   keychain to run in, so the Docker image sets this by default (see
   [`docker.md`](docker.md)); a bare-metal install must opt in deliberately,
   and the Settings UI surfaces an explicit acknowledgment
   (`credential_file_fallback_accepted`) the first time this path is used.

If neither applies, the app still boots — every credential already supplied
via an environment variable keeps working (see below) — but the Settings UI
can't persist new ones, and says so.

Whichever backend is chosen, an **environment variable overlay** always sits
in front of it: a value from the environment always wins over one saved
through the Settings UI, so a container that injects `OPENROUTER_API_KEY` at
deploy time is never shadowed by a stale Settings-UI value. `GET
/settings/integrations` reports which source is currently answering for each
field (`env` vs. `keyring`/`file`) so the UI can show "managed by
environment" instead of a misleadingly-editable field — and refuses to
delete an env-sourced value from the UI, since doing so would have no
visible effect anyway.

## Environment variable overrides

A credential key like `openrouter.api_key` maps to `OPENROUTER_API_KEY`
(dots → underscores, uppercased). Three ways to supply it, checked in order:

1. **`$OPENROUTER_API_KEY_FILE`** — path to a file holding the value (the
   Docker/Kubernetes "secret mounted as a file" convention). Setting this
   variable is an explicit promise that the file is readable; a missing or
   unreadable file is a hard error, not a silent fall-through.
2. **`$XCHATS_SECRETS_DIR/openrouter.api_key`** — a file named after the raw
   key inside a shared secrets directory (the Docker Compose/Swarm secrets
   mount convention). Simply not being present here is normal and falls
   through to the next tier.
3. **`$OPENROUTER_API_KEY`** — the literal environment variable.

The same three-tier resolution applies to every provider field and to the
four system secrets' legacy env vars.

## File-backed store details

When the file fallback is active, two files live under the resolved data
directory (see [`data-locations-and-privacy.md`](data-locations-and-privacy.md)
for exact paths):

- **`credentials.key`** — a generated 32-byte key, written once with `0600`
  permissions, read back on every subsequent boot. Overridable with
  `$XCHATS_CREDENTIALS_KEY` (64 hex chars) instead of a generated file — an
  advanced option for a deployment that manages its own key material.
- **`credentials.enc`** — every credential value, sealed independently with
  that key (AES-256-GCM, random 96-bit nonce per value), `0600` permissions.

Losing `credentials.key` makes every value in `credentials.enc`
unrecoverable — there is no separate backup of it, and (deliberately) it is
**never** included in a Settings UI backup download, see
[`backup-restore.md`](backup-restore.md). Back it up separately if you rely
on the file-backed store, by the same discipline you'd apply to an SSH
private key.

## Rotation

There is no one-click rotation UI today. To rotate a system secret or an
integration credential manually: delete the corresponding key from the
credential store (Settings UI, for an integration credential) or stop the
app and remove/edit the relevant file/keychain entry, then restart — a
system secret regenerates automatically; an integration credential needs to
be re-entered from the Settings UI. Rotating `TG_CREDENTIALS_ENC_KEY`
specifically invalidates every already-stored Telegram bot token (it can no
longer decrypt), so re-save those tokens immediately after.
