# Troubleshooting

## "Settings can't save credentials" / no keychain available

`GET /settings/credential-storage` or the Settings UI's own messaging will
say no secure credential store is available. This means
`internal/credentials.Open` found neither a usable OS keychain nor an
explicit opt-in to the file-backed fallback. Fix:

- **Docker:** shouldn't happen — the image sets
  `XCHATS_ALLOW_FILE_CREDENTIALS=1` by default. If you've overridden the
  backend's `environment:` block and dropped that variable, restore it.
- **Bare-metal Linux without a Secret Service provider running** (common on
  a headless server): set `XCHATS_ALLOW_FILE_CREDENTIALS=1` explicitly. The
  Settings UI will ask you to acknowledge the file-backed store's weaker
  guarantee (encrypted on disk, but without OS-level access control) the
  first time you save a credential this way.

See [`credentials.md`](credentials.md) for the full resolution order.

## Port already in use

```bash
make kill-ports          # frees 8080 8090 5173 8081 by default
make kill-ports PORTS="8080 5173"   # or name specific ports
```

For the Docker stack specifically, prefer `make down` over killing ports
directly — a port held by a Compose-managed container needs the container
stopped, not just the listener killed. If a `go run ./cmd/xchats` process
from a previous session is still holding a port after you thought you
killed it: `go run` spawns a separate child binary that can outlive the
`go run` process itself being killed — find and kill the actual binary
(`ps aux | grep xchats`, or `pgrep -f cmd/xchats`), not just the `go run`
invocation.

## ngrok tunnel won't start

**Settings → Remote Access → Start** failing, or `POST
/settings/tunnel/start` returning a 502 with a `last_error`:

- **"authtoken" errors** — the ngrok authtoken credential (Settings →
  Integrations → ngrok) is missing or was rejected. Get a fresh one from
  https://dashboard.ngrok.com/get-started/your-authtoken and re-save it.
- **Region/domain errors** — a reserved domain (Settings → Remote Access)
  that isn't actually reserved on your ngrok account, or doesn't match your
  account's plan, fails at connect time. Clear it and retry with no domain
  set (ngrok assigns a random one) to isolate the problem.
- Remember the tunnel serves the **entire application**, login page
  included — not just an API surface. This is a deliberate, documented
  design choice (see `plan/DECISIONS.md`'s 2026-08 tunnel amendment), not a
  bug: expect the whole app to be reachable at the public URL for as long as
  the tunnel is running, and stop it when you don't need remote access.

## WhatsApp pairing stuck / QR keeps expiring

- The QR code is short-lived by WhatsApp's own protocol; if it visibly
  expires before you scan it, refresh (`/accounts` → **add** again) rather
  than waiting on a specific one.
- A pairing that starts but never completes usually means the phone lost
  its own internet connection mid-scan, or you scanned with a WhatsApp
  Business/regular app mismatch versus what the number is already paired
  as elsewhere. Retry the pairing flow from scratch.
- A previously-paired number that stops receiving messages: check
  `/accounts` for its status — a `logout`ged session (manual, or WhatsApp
  invalidating it remotely) needs re-pairing; the row and its chat history
  are preserved either way and a re-pair **revives** them rather than
  starting over.

## Telegram: webhook vs. polling, and which one is active

`internal/config.Config.TelegramResolvedMode()` decides automatically unless
you set `telegram.mode` explicitly (`webhook` or `polling` in config.yaml,
or the `TELEGRAM_MODE` env var):

- A configured `TG_WEBHOOK_PUBLIC_BASE_URL` (a public HTTPS URL Telegram can
  reach) → **webhook** mode.
- No public base URL configured → **polling** mode (the zero-config path;
  no inbound connectivity required at all).

If messages aren't arriving:

- **Webhook mode:** confirm `TG_WEBHOOK_PUBLIC_BASE_URL` is actually
  reachable from the internet (not just from your machine) — Telegram calls
  it directly. A tunnel (Settings → Remote Access) is one way to get a
  reachable URL without a static IP/domain. Check that the webhook secret
  (auto-provisioned, see [`credentials.md`](credentials.md)) matches what
  Telegram has registered — re-saving the bot token from Settings
  re-registers the webhook with the current secret.
- **Polling mode:** confirm the bot token is valid (Settings → Integrations
  → test it) and that outbound HTTPS to `api.telegram.org` (or your
  configured `TG_API_BASE_URL`, for a self-hosted Bot API server) isn't
  blocked by a firewall/proxy.

## "Could not verify the credential" vs. "credential was rejected"

The Settings UI distinguishes two different failures when saving/testing an
integration credential, and it matters which one you're seeing:

- **"Credential was rejected by the provider"** (422) — the provider itself
  said no (401/403, or a recognized invalid-key response body). The
  credential is wrong; nothing you can force past this. Get a fresh key.
- **"Could not verify the credential"** (409 on save, 503 on test) — the
  *check itself* didn't complete (network trouble, the provider's own API
  was down, a timeout). This is not evidence the credential is bad. On
  **save**, you can explicitly confirm "save anyway" to store it despite the
  unverified state; on **test** (an already-saved credential), just retry
  once the provider/network issue clears.

## Provider health badge shows unhealthy but the credential looks fine

`internal/providerhealth` only flips a provider to unhealthy on an actual
401/403 from a live LLM completion call — not on a timeout, a 500, or any
other transient failure (avoiding false-alarm flapping). If a provider
outage caused real request failures, the badge should self-clear on the
next successful call once the provider recovers; it does not need a manual
reset. If it stays unhealthy after you've confirmed the key works (Settings
→ Integrations → test), check whether a *different* key is actually
configured for the model your default LLM settings point at.

## "403 Forbidden" on Settings pages

Every `/settings/**` route requires the **admin** role on your
organization membership — a `member`-role account gets a 403 by design, not
a bug. Have an existing admin add you as an admin from **Settings → Team
Management**, or log in with an account that already has the role.

## Restore fails with "currently open by another process"

Expected — `xchats restore` is offline-only. Stop the server
(`make down`, or kill the `xchats serve` process) first; see
[`backup-restore.md`](backup-restore.md).
