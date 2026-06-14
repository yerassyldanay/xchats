# Captures — real Evolution v2.3.7 payloads (test fixtures)

These are **real** payloads captured from a live `evoapicloud/evolution-api:v2.3.7` instance.
They are the deterministic fixtures for the isolated test harness (see
`../6-isolated-testing.md`): the fake Evolution replays the webhook bodies, and the normalizer is
asserted against them. Secrets (`apikey`, `messageSecret`) are redacted.

- `samples/webhook_bodies/send_message.json` — outbound echo (`send.message`); `data.key.id` is the
  evolution_message_id; `status: PENDING`. Note top-level envelope fields (`instance`, `sender`,
  `server_url`, `date_time`, `apikey`).
- `samples/webhook_bodies/messages_update.json` — delivery/read status (`messages.update`); status
  is keyed by **`data.keyId`** (not `data.key.id`); `status: DELIVERY_ACK`.
- `samples/webhook_bodies/chats_upsert.json` — `chats.upsert`; `data` is an **array**, keyed by `@lid`.
- `samples/messages_sample.json` — two `findMessages` records (text + image); note each `key` has
  **both** `remoteJid` (`@lid`) and `remoteJidAlt` (the real phone) + `addressingMode: "lid"`.

> **This is a seed, not full coverage — and it is missing the core inbound event.** The e2e suite
> cannot be trusted until these are captured and committed (a green run today does *not* prove the
> contract). Record by pointing a logging tee / the webhook receiver at a live instance, and add:
>
> - **`messages.upsert` inbound — text** (the primary event the whole product is built on; absent here).
> - **`messages.upsert` inbound — `imageMessage`**, plus the matching **`getBase64FromMediaMessage`**
>   response (the media path is asserted against an uncaptured shape; the contract is only pinned by
>   `scripts/evolution_client.py` today).
> - **A matched pair: one outbound `send.message` followed by *its own* `messages.update`.** The two
>   existing files are *different* conversations, so they cannot prove status correlation. `key.id`
>   (22 chars) ≠ `keyId` (40 chars); the matched pair settles which field status actually keys on
>   (see `../9-database-schema.md` → `status_correlation_id`).
> - **A `chats.upsert` (`@lid`-only) that precedes the first message** — to exercise the lid-only
>   ordering case (no `remoteJidAlt`/phone present; see `../3-sync.md`).
> - `connection.update`, `qrcode.updated`, `contacts.*`.
