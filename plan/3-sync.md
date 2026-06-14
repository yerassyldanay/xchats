# Message Handling (Live)

How WhatsApp messages get into our Postgres. **v1 is live-only: we store what Evolution sends us,
as it arrives — nothing more.**

## The whole thing in one paragraph

We never talk to WhatsApp. **Evolution** is a linked device on the WhatsApp account (like WhatsApp
Web) and is the only thing that touches WhatsApp. After an account is connected, Evolution
**pushes** every event — new message, outbound reply, status change — to our webhook, and we store
it. That's the entire system: *a message arrives → we store it.*

```
 Evolution ──webhook──► store raw ──► worker ──► upsert ──► broadcast to UI
                        evolution_events        (contacts / conversations / messages)
```

## The one mechanism

Every write is an **idempotent upsert on a stable unique key**:

```
messages       UNIQUE (account_id, evolution_message_id)
conversations  UNIQUE (account_id, remote_jid)
identities     UNIQUE (contact_id, account_id, identity_kind, identity_value)
```

Upsert = insert if new, otherwise enrich missing fields (never blind-overwrite). With this rule,
the things that would otherwise be "edge cases" cost no extra code:

- **A webhook delivered twice / an Evolution retry** → second write hits the unique key, no-op.
  (Confirmed: in the capture each message arrived **twice** — 3 messages → 6 `messages.upsert` —
  same `key.id`. Dedup is not optional; it's load-bearing. See `captures/README.md` finding 3.)
- **Events arriving out of order** → upsert doesn't care about order.
- **A message for a chat we've never seen** → the worker just creates the conversation.

## Receiving is split from processing (why nothing is lost)

The webhook does the **minimum** and returns fast:

```
POST /evolution/api/v1/webhook/{account_id}
  1. verify shared webhook token
  2. INSERT raw payload into evolution_events
  3. return 200            ← event is now durably stored
  4. enqueue a process_event job
```

It does **not** run AI or wait on anything. So an incoming message is captured the instant it
arrives; if a worker is busy or the app crashes, the raw event is on disk and gets processed on
restart. A worker then normalizes and upserts:

```
process_event:
  1. extract identifiers from payload
  2. upsert contact + identities      (resolve @lid ↔ phone, see below)
  3. upsert conversation              (create if the chat is new)
  4. upsert message                   (dedup on the unique key)
  5. update conversation last_message_at / preview / unread_count
  6. broadcast message.created to the UI
```

## Identity (`@lid` vs phone) — verified against real captures

WhatsApp identifies the same person two ways and one is **not** derivable from the other
(`77000000000@s.whatsapp.net` ≠ `5200000000000@lid`). But the real capture
(`captures/README.md`) shows the split is **per-event-type**, and for live-only v1 it's mostly
harmless:

| Event | identifier it carries |
|---|---|
| `messages.upsert` (inbound), `send.message` (outbound) | **phone** (`key.remoteJid` + `remoteJidAlt`) |
| `messages.update` (status) | `@lid` |
| `chats.*`, `contacts.*` | `@lid` only |

Why v1 survives without resolving `@lid`:

- **Conversations** are built from message events → always keyed on the **phone** `remote_jid`.
  Consistent; no fragmentation.
- **Replies** always have a recipient: the inbound `key.remoteJid` is the phone → `sendText`.
- **Status** correlates by message id (`keyId`), **not** by JID (see below) — so the `@lid` on
  `messages.update` never needs mapping.
- **`@lid`-keyed `chats.*`/`contacts.*`** events are cosmetic (unread, profile pic). v1 ignores
  them; mapping `@lid`→phone is deferred (later enrichment).

`contact_identities` still earns its place for that later enrichment (and so one shared contact can
reach several accounts — **contact is shared, conversations are separate**, one per
account + remote_jid). It is just not load-bearing for the core v1 loop.

## Status lifecycle (sent → delivered → read)

Outbound status arrives later via `messages.update`. Apply it **monotonically** — a late event
never downgrades `read` back to `sent`.

> **Status correlation:** apply delivery/read by matching `messages.update.data.keyId` →
> `messages.evolution_message_id` — verified equal in the captures (v2.3.7), so no separate
> correlation id is needed. See `captures/README.md` finding 4.

## Replies sent from another device

WhatsApp is multi-device. If the operator replies from the phone or WhatsApp Web, Evolution (a
linked device on the same account) still sees it and pushes an outbound event (`messages.upsert`
with `key.fromMe: true`). Same upsert path: store as an OUTBOUND message with
`sender_kind = external_account`, create the contact/conversation if new, broadcast. The inbox
reflects the full conversation no matter which device sent.

## Events we subscribe to

```
CONNECTION_UPDATE
CONTACTS_UPSERT / CONTACTS_UPDATE
CHATS_UPSERT / CHATS_UPDATE
MESSAGES_UPSERT / MESSAGES_UPDATE / MESSAGES_DELETE
SEND_MESSAGE
```

Parse all of this behind **one normalizer module** so an Evolution version change can't leak into
the whole app. Because we store raw events, old payloads can be replayed against an updated
normalizer.

> The `*_SET` events (`MESSAGES_SET`, `CHATS_SET`, `CONTACTS_SET`) are **ignored** — v1 handles only
> the live `*_UPSERT` / `*_UPDATE` / `SEND_MESSAGE` events listed above.

## Sending

```
POST /xchats/api/v1/conversations/{id}/messages
  1. create local message status=queued; broadcast message.created
  2. Evolution POST /message/sendText
  3. store returned evolution_message_id
  4. status=sent or failed; broadcast message.updated
  5. later: delivered/read arrive via messages.update  (matched on evolution_message_id)
```

Human and AI replies share this pipeline; an AI send just starts from a member-approved `ai_draft`.
(v1 is on-demand drafts, no auto-send.)

## What v1 builds

Just the mechanism — three cheap pieces:

```
1. webhook stores raw events, returns 200 fast
2. worker upserts normalized data on the unique keys (idempotent)
3. identity resolution (@lid ↔ phone) so one customer stays one contact
```

Plus two trivial guards: **drop `@g.us` (group) events at the door** (one `if` — no group schema in
v1), and **text-only** (ignore media messages, or store the message with body empty).

## Deferred (each is additive on the same upsert path)

None of these are needed for correct live operation; add them in their phase, not before:

| Deferred | Note |
|---|---|
| Media (download, thumbnail, transcription) | v1 text-only. (`message_media` table removed until then.) |
| Groups (`@g.us`) | Dropped at the webhook in v1 — no participants schema. |
| Per-member unread, auto-reply/auto-send | Product features, not message correctness. |

See `4-wa-connection-example.md` for the connect/QR flow (deferred) and `9-database-schema.md` for
the authoritative tables.
