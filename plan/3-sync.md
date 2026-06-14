# Message Handling (Live)

How WhatsApp messages get into our Postgres. **v1 is live-only: we store what Evolution sends us,
as it arrives — nothing more.**

## The whole thing in one paragraph

We never talk to WhatsApp. **Evolution** is a linked device on the WhatsApp account (like WhatsApp
Web) and is the only thing that touches WhatsApp. After an account is connected, Evolution
**pushes** every event — new message, outbound reply, status change — to our webhook, and we store
it. That's the entire system: *a message arrives → we store it.*

```
 Evolution ──webhook──► enqueue ──► worker ──► upsert ──► broadcast to UI
            (return 200) in-mem queue     (wa_contacts / wa_chats / wa_messages)
```

## The one mechanism

Every write is an **idempotent upsert on a stable unique key**:

```
wa_messages   UNIQUE (account_id, evolution_message_id)
wa_chats      UNIQUE (account_id, remote_jid)
wa_contacts   UNIQUE (account_id, phone_jid)
```

Upsert = insert if new, otherwise enrich missing fields (never blind-overwrite). With this rule,
the things that would otherwise be "edge cases" cost no extra code:

- **A webhook delivered twice / an Evolution retry** → second write hits the unique key, no-op.
  (Confirmed: in the capture each message arrived **twice** — 3 messages → 6 `messages.upsert` —
  same `key.id`. Dedup is not optional; it's load-bearing. See `captures/README.md` finding 3.)
- **Events arriving out of order** → upsert doesn't care about order.
- **A message for a chat we've never seen** → the worker just creates the chat.

## Receiving is split from processing (why nothing is lost)

The webhook does the **minimum** and returns fast — it never touches Postgres:

```
POST /evolution/api/v1/webhook/{account_id}
  1. verify shared webhook token
  2. enqueue the raw event onto the in-memory queue
  3. return 200            ← immediate; no DB write, no processing
```

It does **not** run AI, upsert, or wait on anything. A worker then consumes the queue, normalizes,
and upserts:

```
worker (consumes the queue):
  1. extract identifiers from payload
  2. upsert wa_contacts   (by phone_jid; capture lid_jid / push_name)
  3. upsert wa_chats      (by remote_jid; create if the chat is new)
  4. upsert wa_messages   (dedup on the unique key; raw payload on .raw)
  5. update wa_chats last_message_at / preview / unread_count
  6. broadcast message.created to the UI
```

The queue is in-process (Go channels) in v1, so the `200` returns **before** processing — an event
still in the queue is lost on a restart (Evolution already acked). v1 accepts this; the queue is a
swappable port (below), so moving to a durable broker needs no change to the webhook or worker.
Idempotent upserts make any re-delivery a no-op.

## Queue abstraction (Go channels now; Redis/Kafka later)

The queue is a **port** — producers (the webhook, the API) and consumers (workers) depend only on an
interface, never on channels directly, so the backing driver swaps via config:

```go
// internal/queue
type Message struct { Kind string; Payload []byte }   // Kind: "wa_event" | "ai_draft" | ...
type Queue interface {
    Publish(ctx context.Context, m Message) error            // webhook/API: non-blocking enqueue
    Consume(ctx context.Context, handle func(Message) error) // worker pool: invokes handle per msg
}
```

- **v1 driver — `inmem`:** a buffered Go channel + a small worker-goroutine pool. Default; also used
  by tests (deterministic, no infra).
- **Later — `redis` / `kafka`:** implement the same interface, selected by `QUEUE_DRIVER`; they add
  the durability the in-mem driver lacks. Swapping needs **no change** to the webhook or workers.
- One bus, multiple `Kind`s: `wa_event` (inbound processing) and `ai_draft` (on-demand). No DB
  `jobs` / `evolution_events` tables.

## Identity (`@lid` vs phone) — verified against real captures

WhatsApp identifies the same person two ways and one is **not** derivable from the other
(`77000000000@s.whatsapp.net` ≠ `5200000000000@lid`; see `9-database-schema.md` →
"WhatsApp identifiers (JID / LID)" for what these are). But the real capture
(`captures/README.md`) shows the split is **per-event-type**, and for live-only v1 it's mostly
harmless:

| Event | identifier it carries |
|---|---|
| `messages.upsert` (inbound), `send.message` (outbound) | **phone** (`key.remoteJid` + `remoteJidAlt`) |
| `messages.update` (status) | `@lid` |
| `chats.*`, `contacts.*` | `@lid` only |

Why v1 survives without resolving `@lid`:

- **Chats** are built from message events → always keyed on the **phone** `remote_jid`.
  Consistent; no fragmentation.
- **Replies** always have a recipient: the inbound `key.remoteJid` is the phone → `sendText`.
- **Status** correlates by message id (`keyId`), **not** by JID (see below) — so the `@lid` on
  `messages.update` never needs mapping.
- **`@lid`-keyed `chats.*`/`contacts.*`** events are cosmetic (unread, profile pic). v1 ignores
  them; mapping `@lid`→phone is deferred (later enrichment).

The `@lid`↔phone mapping lives on the single `wa_contacts` row (phone is the key; `lid_jid` stored
alongside when seen) — no separate identities table. **Each account owns its contacts**; there is no
cross-account sharing. Resolving `@lid` for `chats.*`/`contacts.*` events is deferred enrichment, not
load-bearing for the core v1 loop.

## Status lifecycle (sent → delivered → read)

Outbound status arrives later via `messages.update`. Apply it **monotonically** — a late event
never downgrades `read` back to `sent`.

> **Status correlation:** apply delivery/read by matching `messages.update.data.keyId` →
> `wa_messages.evolution_message_id` — verified equal in the captures (v2.3.7), so no separate
> correlation id is needed. See `captures/README.md` finding 4.

## Replies sent from another device

WhatsApp is multi-device. If the operator replies from the phone or WhatsApp Web, Evolution (a
linked device on the same account) still sees it and pushes an outbound event (`messages.upsert`
with `key.fromMe: true`). Same upsert path: store as an OUTBOUND message with
`sender_kind = external_account`, create the wa_contact/wa_chat if new, broadcast. The inbox
reflects the full chat no matter which device sent.

## Events we subscribe to

```
CONNECTION_UPDATE
CONTACTS_UPSERT / CONTACTS_UPDATE
CHATS_UPSERT / CHATS_UPDATE
MESSAGES_UPSERT / MESSAGES_UPDATE / MESSAGES_DELETE
SEND_MESSAGE
```

Parse all of this behind **one normalizer module** so an Evolution version change can't leak into
the whole app. Message payloads are kept on `wa_messages.raw`, so a message can be re-normalized
after a normalizer fix.

> The `*_SET` events (`MESSAGES_SET`, `CHATS_SET`, `CONTACTS_SET`) are **ignored** — v1 handles only
> the live `*_UPSERT` / `*_UPDATE` / `SEND_MESSAGE` events listed above.

## Sending

```
POST /xchats/api/v1/chats/{id}/messages
  1. create local message status=queued; broadcast message.created
  2. Evolution POST /message/sendText
  3. store returned evolution_message_id
  4. status=sent or failed; broadcast message.updated
  5. later: delivered/read arrive via messages.update  (matched on evolution_message_id)
```

Human and AI replies share this pipeline; an AI send just starts from a user-approved `ai_draft`.
(v1 is on-demand drafts, no auto-send.)

## What v1 builds

Just the mechanism — three cheap pieces:

```
1. webhook enqueues events, returns 200 fast (no DB write)
2. worker consumes the queue, upserts normalized data on the unique keys (idempotent)
3. identity resolution (@lid ↔ phone) so one customer stays one wa_contact
```

Plus two trivial guards: **drop `@g.us` (group) events at the door** (one `if` — no group schema in
v1), and **text-only** (ignore media messages, or store the message with body empty).

## Deferred (each is additive on the same upsert path)

None of these are needed for correct live operation; add them in their phase, not before:

| Deferred | Note |
|---|---|
| Media (download, thumbnail, transcription) | v1 text-only. (`message_media` table removed until then.) |
| Groups (`@g.us`) | Dropped at the webhook in v1 — no participants schema. |
| Per-user unread, auto-reply/auto-send | Product features, not message correctness. |

See `4-wa-connection-example.md` for the connect/QR flow (deferred) and `9-database-schema.md` for
the authoritative tables.
