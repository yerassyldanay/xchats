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

> This is a representative seed. Expand it with a full live capture (all event types: messages.upsert
> inbound text+media, connection.update, qrcode.updated, contacts.*) before relying on the e2e suite
> for full coverage — record by pointing a logging tee / the webhook receiver at a live instance.
