# Testing the Telegram channel

This is a reference, not a script: everything here is meant to be read by a
coding agent (or a human) with no prior context on this repo, so it can write
its own throwaway curl commands, a one-off script, or a five-minute fake Bot
API — without the repo committing and maintaining that tooling itself. For how
the channel is *built*, see [`architecture.md`](architecture.md) and
[`database-schema.md`](database-schema.md); this doc is only about *verifying
it works*.

## Env vars needed to boot

Authoritative source: [`.env.example`](../.env.example) and
[`backend/internal/config/config.go`](../backend/internal/config/config.go).
The ones specific to Telegram:

| Var | Purpose |
|---|---|
| `TG_WEBHOOK_PUBLIC_BASE_URL` | The public HTTPS URL Telegram will POST updates to. **Must be real public HTTPS** — Telegram's `setWebhook` refuses anything else, including `http://localhost`. For local testing without a real bot, point this at whatever base URL your fake Bot API expects to have been registered with (it doesn't have to be reachable — nothing calls back into it except the real Telegram, which you won't be using in that mode). |
| `TG_API_BASE_URL` | Overrides `https://api.telegram.org`. Point this at `http://127.0.0.1:<port>` to redirect the Bot API client at a fake server instead. Empty = talk to the real Telegram. |
| `TG_WEBHOOK_SECRET` | The `secret_token` registered with `setWebhook` and checked on every inbound webhook call (header `X-Telegram-Bot-Api-Secret-Token`). Falls back to `WEBHOOK_TOKEN` if unset. Telegram's charset rule applies: `A-Za-z0-9_-`, max 256 chars. |
| `TG_CREDENTIALS_ENC_KEY` | AES-256-GCM key encrypting bot tokens at rest (`xchats.tg_credentials`). Generate with `openssl rand -hex 32`. Without it, connecting a bot 400s. |
| `OPENROUTER_API_KEY` (or `OPENAI_API_KEY`/`GEMINI_API_KEY` matching `LLM_DEFAULT_PROVIDER`) | Boot fatals without *some* value here — see `buildLLMRegistry` in `backend/cmd/xchats/main.go`. A dummy string boots fine; AI-draft generation will then fail server-side and only be logged (see "AI drafts" below), which is not a sign the Telegram channel itself is broken. |

Booting from just exported env vars with no host `.env`/`config.yaml`
interference: `go run ./cmd/xchats -env /dev/null -config /dev/null serve`
(both flags default to reading `.env`/`config.yaml` relative to cwd if
present — pointing them at `/dev/null` gives a clean, reproducible boot from
only what you export).

## The endpoint catalogue

All under `/xchats/api/v1` unless noted. Every response is wrapped
`{payload, errcode, message}`; treat anything with `errcode != "OK"` as a
failure regardless of HTTP status. Auth is a session cookie
(`xchats_session`) from `POST /auth/login {email, password}` — use a cookie
jar (`curl -c jar -b jar ...`).

**Lifecycle** (source: `backend/internal/httpapi/telegram_accounts.go`)

| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/telegram-accounts` | `{bot_token, display_name, drop_pending_backlog?}` | Validates via `getMe`, claims the account, calls `setWebhook`. 201 on success. Reconnecting the same bot token revives a soft-deleted row (see "Gotchas"). |
| POST | `/telegram-accounts/:id/check` | — | Calls `getMe` + `getWebhookInfo`, reconciles connection state. Response adds `pending_update_count`, `expected_webhook_url` next to the usual `{account, connection_state}`. |
| POST | `/telegram-accounts/:id/retry-webhook` | — | Re-runs `setWebhook` with the stored token. Never drops the pending backlog. |
| PUT | `/telegram-accounts/:id/token` | `{bot_token}` | Replaces the token. 409 if the new token belongs to a *different* bot (`getMe`'s `id` must match the account's `bot_id`). |
| DELETE | `/telegram-accounts/:id` | — | Disconnect state machine: marks `disconnect_pending`, calls `deleteWebhook`, only purges the token on confirmed removal. |

The connect/check responses share a shape:
`{account: <Account>, connection_state, pending_update_count?, expected_webhook_url?}`.

**Webhook ingress** (source: `telegram_webhook.go`) — outside the auth group,
this is what Telegram itself calls:

```
POST /telegram/api/v1/webhook/:account_id
X-Telegram-Bot-Api-Secret-Token: <TG_WEBHOOK_SECRET, or the WEBHOOK_TOKEN fallback>
```

`:account_id` is returned by the connect call. Body is a raw Telegram
`Update` JSON (see the example below). Commits synchronously before
acking — a 200 means the message is already in the DB, not just queued.
Wrong/missing secret → 401. Unknown or soft-deleted account → 200 (ack so
Telegram stops retrying) with nothing stored.

**Inbox surface** (channel-neutral; source: `backend/internal/httpapi/inbox.go`,
`accounts.go`)

| Method | Path | Notes |
|---|---|---|
| GET | `/accounts` | `{items: [<Account>]}` — every channel, one shape. |
| GET | `/chats?account_id=<uuid>` | `{items: [<Chat>]}`. `account_id` is the neutral filter; `wa_account_id` is a deprecated alias kept for old clients. |
| GET | `/chats/:id/messages` | `{items: [<Message>]}` |
| POST | `/chats/:id/messages` | `{text?, media_ids?}` — operator/API-composed send, independent of the AI-draft flow. |
| GET | `/media/:id` | Raw bytes. 502 `blob missing` while a media download is still pending; 200 once the worker has fetched it via `getFile`/download. |

## Response shapes (exact fields)

Pulled directly from `backend/internal/dto/dto.go` — treat this table, not
guesswork, as the contract:

**Account** (`MapNeutralAccount`): `id, channel, display_name, external_handle,
connection_state, assigned, instance_name, last_live_event_at, created_at`,
plus, **only when `channel == "telegram"`**: `webhook_url,
webhook_registered_at, webhook_last_checked_at, webhook_last_error` (present
and `null` on every other channel, not just omitted).

**Chat** (`MapChat`): `id, channel, account_id, contact, status,
assignee_user_id, unread_count, last_message_at, last_message_preview`. A
`wa_account_id` field is *also* present, but only when `channel != "telegram"`
(deprecated alias for old WhatsApp-only clients).

**Contact** (nested in Chat): `id, display_name, phone_number, phone_jid,
lid_jid, push_name`. For Telegram, `phone_number` is empty and `phone_jid`
carries the Telegram numeric user id as a string — there is no phone number.

**Message** (`MapMessage`): `id, chat_id, direction, sender_type,
evolution_message_id, message_type, content, media, status, source,
timestamp`. `evolution_message_id` is a legacy field name kept for
compatibility — for a Telegram message it holds the Telegram message id, not
an Evolution one.

**Media** (each entry in a Message's `media[]`): `id, url, media_type,
mimetype, file_name, file_size`.

## Faking the Bot API (no committed mock — write your own)

If you don't have a real bot token + public tunnel, point `TG_API_BASE_URL`
at a throwaway server you spin up for the session (Python's
`http.server`-based handler, a five-line Go `net/http` server, whatever's
fastest) and tear it down when done. It only needs to answer what
`backend/internal/telegram/client.go` actually calls:

- **Two URL roots, same token in the path**: `POST /bot<token>/<method>` for
  every API call, and a *separate* root `GET /file/bot<token>/<file_path>`
  for downloading media bytes (not under `/bot<token>/...`).
- **Envelope**: success is `{"ok": true, "result": <payload>}`; failure is
  `{"ok": false, "error_code": <int>, "description": "<string>"}`.
- **Minimum method set** to exercise connect → inbound → outbound → media:
  - `getMe` → `{"id": <int>, "is_bot": true, "first_name": "...", "username": "..."}`. Deriving the id from the token (e.g. the digits before `:`) is a convenient way to make "a different bot's token" produce a different id, if you want to exercise the replace-token 409 path.
  - `setWebhook` (JSON body `{url, secret_token, allowed_updates, drop_pending_updates}`) → `{"ok": true, "result": true}`.
  - `getWebhookInfo` → `{"url": ..., "pending_update_count": 0, ...}` — echo back whatever `setWebhook` was last called with, so `check`'s reconciliation logic sees a match.
  - `sendMessage` (JSON `{chat_id, text}`) → `{"message_id": <int>, "chat": {"id": <chat_id>, "type": "private"}}`. Record what you're sent somewhere you can inspect afterward (a log line is enough) — that's how you confirm an outbound reply actually reached "Telegram".
  - `getFile` (JSON `{file_id}`) → `{"file_id": ..., "file_path": "whatever", "file_size": <n>}`.
  - The download root then just needs to return 200 with some bytes for that path.
- Anything else Telegram would need for a fuller test — `deleteWebhook`,
  the `sendPhoto`/`sendDocument`/`sendVideo`/`sendAudio`/`sendVoice` family
  (multipart uploads, field name = the method name minus `send`, lowercased)
  — follows the same envelope; add only what your test actually exercises.

## A worked example flow

```bash
# 1. Log in
curl -c jar -s -X POST localhost:8080/xchats/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"<SEED_ADMIN_PASSWORD>"}'

# 2. Connect a bot (real token, or a throwaway fake's — either way it must
#    pass getMe)
curl -b jar -s -X POST localhost:8080/xchats/api/v1/telegram-accounts \
  -H 'Content-Type: application/json' \
  -d '{"bot_token":"123456:TESTTOKEN","display_name":"Test Bot"}'
# → capture payload.account.id as $ACCOUNT_ID

# 3. Deliver an inbound text update (this is the exact JSON shape Telegram
#    itself sends; adjust ids freely)
curl -s -X POST localhost:8080/telegram/api/v1/webhook/$ACCOUNT_ID \
  -H "X-Telegram-Bot-Api-Secret-Token: $TG_WEBHOOK_SECRET" \
  -d '{
    "update_id": 1,
    "message": {
      "message_id": 100,
      "date": 1781460000,
      "from": {"id": 500100, "is_bot": false, "first_name": "Aigul"},
      "chat": {"id": 500100, "type": "private"},
      "text": "hello"
    }
  }'
# → 200. Row is already committed by the time this returns.

# 4. Same shape for a photo, add a "photo" array instead of/alongside "text":
#    "photo": [{"file_id":"abc","file_unique_id":"u1","width":90,"height":90,"file_size":100}]
# and/or a "caption" field instead of "text".

# 5. Confirm it landed in the inbox
curl -b jar -s "localhost:8080/xchats/api/v1/chats?account_id=$ACCOUNT_ID"
# → capture the chat's id as $CHAT_ID
curl -b jar -s "localhost:8080/xchats/api/v1/chats/$CHAT_ID/messages"

# 6. Reply — this exercises the outbound send path through to your fake
#    Bot API's sendMessage
curl -b jar -s -X POST "localhost:8080/xchats/api/v1/chats/$CHAT_ID/messages" \
  -H 'Content-Type: application/json' -d '{"text":"reply"}'

# 7. Lifecycle
curl -b jar -s -X POST "localhost:8080/xchats/api/v1/telegram-accounts/$ACCOUNT_ID/check"
curl -b jar -s -X DELETE "localhost:8080/xchats/api/v1/telegram-accounts/$ACCOUNT_ID"
```

## Gotchas worth knowing up front

- **Public HTTPS is non-negotiable for a real bot.** `TG_WEBHOOK_PUBLIC_BASE_URL`
  must be reachable over HTTPS from Telegram's servers — a tunnel (ngrok,
  cloudflared) is required for local dev against the real Bot API. This is
  the single most common reason "the bot connects but never receives
  anything": the webhook registered fine, but Telegram can't actually reach
  the URL it was given (see the Docker section below for the container-specific
  version of this).
- **Account id is deterministic**: `uuidv5(namespace, "telegram:bot:<bot_id>")`.
  Reconnecting the same bot token lands on the *same* account row — history
  intact, soft-delete reversed — rather than creating a new one.
  (`config.ChannelAccountID`/`TelegramOwnerRef` in `backend/internal/config/config.go`.)
  If you're testing repeatedly against a real, persistent database, redelivering
  the *same* `update_id` for an account that already processed it is a no-op
  (`UNIQUE(account_id, telegram_update_id)`) — use a fresh `update_id` (and
  `message_id`) each time you want to prove a *new* message actually arrives,
  not just that the row already existed from a previous run.
- **AI drafts are best-effort.** A draft is enqueued asynchronously after
  ingest; if the LLM call fails (e.g. a dummy `OPENROUTER_API_KEY`), that
  failure is only logged — it doesn't fail the webhook and it isn't a signal
  the Telegram channel itself is broken. Check `GET /chats/:id/ai-drafts`
  only if you actually care about draft generation.

## Diagnosing "bot connects but doesn't receive messages"

In rough order of likelihood:

1. **`TG_WEBHOOK_PUBLIC_BASE_URL` isn't actually reachable from the public
   internet.** Confirm what Telegram itself believes is registered:
   `POST /telegram-accounts/:id/check` returns `expected_webhook_url` (what
   xchats thinks it registered) — compare against what `getWebhookInfo`
   reports Telegram actually has (surfaced as `connection_state ==
   "webhook_error"` plus `webhook_last_error` on the account if they don't
   match). If everything matches but messages still don't arrive, the URL
   Telegram has is not reachable *from Telegram's network* even if it works
   from your machine (see Docker note below).
2. **Wrong/missing secret.** `TG_WEBHOOK_SECRET` (or its `WEBHOOK_TOKEN`
   fallback) must be the same value at registration time and at ingress
   time; a value changed after registering leaves the ingress rejecting
   Telegram's retries with 401 until `retry-webhook` re-registers.
3. **Container networking**, if running via `docker-compose`: the backend
   container needs `TG_WEBHOOK_PUBLIC_BASE_URL` to be a URL Telegram's
   servers can reach — this is never `http://backend:8080` (compose-internal
   DNS) or `http://localhost:...` (resolves to the container itself). It
   must be the public tunnel/domain in front of whatever reverse-proxies
   into the compose stack. A container log showing `dial tcp ...: connection
   refused` for an *outbound* call (e.g. to Evolution, to the LLM provider)
   is a different problem — that's the container failing to reach something
   *it* calls out to, not Telegram failing to reach the container's inbound
   webhook; don't conflate the two when debugging.
