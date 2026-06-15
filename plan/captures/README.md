# Captures — real Evolution v2.3.7 payloads (test fixtures)

These are **real** payloads captured from a live `evoapicloud/evolution-api:v2.3.7` instance
(captured via the webhook receiver's raw-capture middleware, see
`../../../evolution/src/core/app.py` → `RawCaptureMiddleware`). They are the deterministic fixtures
for the isolated test harness (`../6-isolated-testing.md`): the fake Evolution replays these bodies
and the normalizer is asserted against them. Secrets (`apikey`, `messageSecret`, `mediaKey`) and the
multi-MB inline media `base64` are redacted/truncated.

## Files

| File | Event | What it pins |
|---|---|---|
| `samples/webhook_bodies/messages_upsert_text.json` | `messages.upsert` (inbound, text) | the primary event — `message.conversation` string |
| `samples/webhook_bodies/messages_upsert_image.json` | `messages.upsert` (inbound, image) | `message.imageMessage`, `caption: null`, inline `message.base64` |
| `samples/webhook_bodies/messages_upsert_image_caption.json` | `messages.upsert` (inbound, image + text) | `imageMessage.caption` carries the text — **one** message, not two |
| `samples/webhook_bodies/send_message.json` | `send.message` (outbound echo) | `data.key.id` = evolution_message_id; `status: PENDING` |
| `samples/webhook_bodies/messages_update.json` | `messages.update` (status) | delivery/read; keyed by `data.keyId`; `status: DELIVERY_ACK` |
| `samples/webhook_bodies/chats_upsert.json` | `chats.upsert` | `data` is an **array**, keyed by `@lid` |

Every webhook body has the same envelope: `event`, `instance`, `data`, and top-level
`destination`, `date_time`, `sender`, `server_url`, `apikey`.

### Outbound API calls (request + response) — `samples/api_calls/`

Real `POST /message/{action}/{instance}` calls captured against the live instance (large `media` /
`sticker` / `base64` strings truncated). Each file is `{endpoint, http_status, request, response}`.

| File | action | sends |
|---|---|---|
| `samples/api_calls/sendText.json` | `sendText` | text — **v1** |
| `samples/api_calls/sendMedia.json` | `sendMedia` | image + caption (base64) |
| `samples/api_calls/sendSticker.json` | `sendSticker` | webp sticker (base64) |

**Send-response findings (all three):**
- HTTP **201**, `response.status: "PENDING"`, `response.key.fromMe: true`,
  `response.key.remoteJid` = the phone JID. **Store `response.key.id` as `evolution_message_id`** —
  delivery/read then arrive via `messages.update` keyed on that id.
- `sendMedia` → `messageType: "imageMessage"`; `sendSticker` → `messageType: "stickerMessage"`
  (`mimetype: image/webp`). Both **echo the full media metadata back** (`url`, `directPath`,
  `mediaKey`, `fileSha256`, `fileLength`) **plus the inline `base64`** — same shape as inbound media.
- **Normalizer note:** binary fields (`mediaKey`, `fileSha256`, `fileEncSha256`) come as
  index-keyed objects (`{"0":237,"1":125,...}` = a byte array), and `fileLength`/timestamps are Longs
  (`{low, high, unsigned}`). Reconstruct accordingly.
- See `4.1-evolution-send-api.md` for the request bodies and full endpoint surface.

---

## Findings (what to expect) — verified against the capture

### 1. Inbound gives us the phone number — the reply concern is RESOLVED ✅

For every inbound `messages.upsert`, the `data.key` carries:

```json
"key": {
  "remoteJid":    "77000000000@s.whatsapp.net",   // PHONE jid
  "remoteJidAlt": "77000000000@s.whatsapp.net",   // also phone
  "fromMe": false,
  "id": "3A1FE00DBC50780B05E2",
  "participant": "",            // empty for direct chats; set for groups
  "addressingMode": "lid"
}
```

So to reply we always have a usable recipient: `sendText { "number": "77000000000" }`. Note
`addressingMode: "lid"` is just metadata — the actual JIDs are the **phone**, not `@lid`.

### 2. The `@lid` ↔ phone split is per-event-type (and mostly harmless for v1)

| Event | identifier in payload |
|---|---|
| `messages.upsert` (inbound) | **phone** (`remoteJid` + `remoteJidAlt`) |
| `send.message` (outbound) | **phone** (`key.remoteJid`) |
| `messages.update` (status) | **`@lid`** (`data.remoteJid`, e.g. `5200000000000@lid`) |
| `chats.upsert/update`, `contacts.update` | **`@lid`** only |

The same person is `77000000000@s.whatsapp.net` *and* `5200000000000@lid`, and one is **not**
derivable from the other. But for **live-only v1 this barely matters**, because we never need to
join those two id-spaces by JID:

- **Chats** are built from message events → keyed by the **phone** `remoteJid`.
- **Status updates** correlate by message id (`keyId`), **not** by JID — see finding 4.
- **`@lid`-keyed `chats.*` / `contacts.*`** events are cosmetic (unread/profile pic). v1 can ignore
  them; mapping `@lid`→phone is only needed later for enrichment.

So the `@lid`↔phone mapping lives on the single `wa_contacts` row (phone is the key; `lid_jid`
stored alongside), each account owns its contacts (no cross-account sharing), but v1 functions
without resolving `@lid` for chat/contact events.

### 3. Each message is delivered TWICE — dedup is mandatory and works ✅

3 messages produced **6** `messages.upsert` events: each `key.id` appears twice (this instance also
has Chatwoot integration — note the `chatwootMessageId/InboxId/ConversationId` fields — a likely
source of the double fire). This is the real-world proof of the dedup design:
`UNIQUE (account_id, evolution_message_id)` on `key.id` collapses the duplicate to a no-op upsert.

### 4. Status correlates on `key.id` (== `keyId`) — `status_correlation_id` likely unneeded

In the matched outbound capture, `messages.update.data.keyId` **equalled** the `send.message`
`data.key.id` (both 22 chars, 120/120 matched). The older note that `key.id` (22) ≠ `keyId` (40) was
wrong for v2.3.7. So delivery/read can be applied by matching `messages.update.keyId` to
`messages.evolution_message_id`; the separate `status_correlation_id` column appears redundant.
(`messages.update` also carries a cuid `data.messageId`, e.g. `cmqcmwq9y03hit75qg4ux2du1` — an
Evolution-internal id we don't need.)

### 5. Message-content shapes

```
text          -> data.message.conversation                 (string)   ; data.messageType="conversation"
image only    -> data.message.imageMessage  (caption=null)            ; data.messageType="imageMessage"
image + text  -> data.message.imageMessage.caption = "<the text>"     ; data.messageType="imageMessage"
```

**Image + text is a single message**, not a text message plus a media message. The `caption` is stored as
`body`; a pure image has `caption: null` → empty body. The original text-only v1 ignored the image bytes;
**Build 0 ships media**, so it also writes the inline `data.message.base64` to the blob + a `message_media`
row (see `TODO.md` B6).

### 6. Inline media — full bytes arrive in the webhook

With this config the `imageMessage` webhook includes `data.message.base64` = the **entire image**
base64 (this is what made the raw capture ~1 MB/image; truncated in the fixtures). Useful later:
the media phase may not even need `getBase64FromMediaMessage`. The media contract fields:

```
data.message.imageMessage.mimetype       "image/jpeg"
data.message.imageMessage.caption        text or null
data.message.imageMessage.fileLength     Long: {low, high, unsigned}  -> bytes = low + high*2^32
data.message.imageMessage.mediaKey       32-char b64 decryption key (redacted in fixtures)
data.message.imageMessage.url            encrypted WhatsApp CDN url (needs mediaKey to decrypt)
data.message.imageMessage.directPath     relative CDN path
data.message.imageMessage.jpegThumbnail  small inline b64 preview (truncated in fixtures)
data.message.base64                       full image bytes, b64 (truncated in fixtures)
```

> **Normalizer note:** several numeric fields are serialized **protobuf Longs** (`{low, high,
> unsigned}`), e.g. `fileLength` — reconstruct as `low + high * 2**32`. `messageTimestamp` here was a
> plain int (`1781459144`). The normalizer must tolerate both.

### 7. Other observed fields

`pushName` = sender display name ("Test Contact"); `source` = sender platform (`ios`); `status` =
`DELIVERY_ACK`; `messageContextInfo` = E2E crypto metadata (ignore); `contextInfo` present on media
(quoted message etc.).

---

## Still uncaptured (nice-to-have, not blocking v1)

- `getBase64FromMediaMessage` response shape (media phase — base64 already arrives inline anyway).
- `connection.update` full lifecycle (`open`/`close`/`connecting`) and `qrcode.updated` (connect UI — deferred).
- A group (`@g.us`) message, to confirm `participant` population (groups dropped at the door in v1).
