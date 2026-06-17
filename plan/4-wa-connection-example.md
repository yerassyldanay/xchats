# WhatsApp Connection Example

How a WhatsApp account gets connected from our UI, and how data starts flowing.

> **v1 note:** v1 uses a **single pre-connected account from config** — the connect/QR flow below is
> **deferred to v2** (see `0.1-definition-of-done.md`, `2-architecture.md`). It's kept here as the
> reference design for when the accounts manager is built. The *sync mechanism* it relies on
> (`3-sync.md`) is the same in every version.

The UI never talks to Evolution. **UI → our backend → Evolution.** The backend owns product state;
Evolution is just the WhatsApp transport.

## Services

```
Vue UI            account setup, QR screen, inbox/chat
Backend API       account CRUD, QR polling, chat/message APIs, send endpoint
Evolution adapter backend module that calls Evolution's REST API + normalizes responses
Webhook receiver  catches Evolution events, enqueues them, returns 200 fast (no DB write)
Workers           process events, run AI drafts, retry
Realtime gateway  WebSocket/SSE → UI (account status, new messages, drafts)
AI assistant      reads normalized app data, suggests replies
```

## Endpoints

Evolution (called only by backend):

```
POST /instance/create               POST /webhook/set/{instance}
GET  /instance/connect/{instance}   GET  /instance/connectionState/{instance}
POST /message/sendText/{instance}     POST /message/sendMedia/{instance}
```
(Build 0 builds **both** `sendText` and `sendMedia`; `sendWhatsAppAudio`/`sendSticker`/`sendReaction` later.)
The full outbound surface (text/media/audio/sticker/reaction bodies) is in `4.1-evolution-send-api.md`.

Ours:

```
# WhatsApp accounts = a simplified Evolution manager
GET  /xchats/api/v1/whatsapp-accounts            # all Evolution instances + status + assigned flag
POST /xchats/api/v1/whatsapp-accounts            # add: create a new instance
GET/POST .../whatsapp-accounts/{id}/qr           # generate / poll QR
POST .../whatsapp-accounts/{id}/assign|unassign  # start / stop handling an instance

# Evolution → us  (authenticated with the single shared token from .env)
POST /evolution/api/v1/webhook/{account_id}

# Inbox
GET  /xchats/api/v1/chats
GET/POST .../chats/{id}/messages
POST .../chats/{id}/assign
GET  /xchats/api/v1/realtime
```

Transport: QR = UI polling; events = webhook; inbox updates = WebSocket/SSE; send = plain HTTP.

## The accounts page is a thin Evolution manager

xchats **reuses a running Evolution**. The page mirrors Evolution's `/manager` with simpler UX:
`GET .../whatsapp-accounts` lists **all** Evolution instances with status and an **assigned** flag.
**Assign/unassign** toggles whether xchats handles an instance — only assigned instances are
processed; webhook events for unassigned ones are ignored.

## Connect flow

### 1. Add account

UI asks for an instance name, then `POST /xchats/api/v1/whatsapp-accounts`:

```json
{ "display_name": "Sales WhatsApp", "instance_name": "sales" }
```

Backend:

```
1. Evolution POST /instance/create   (WHATSAPP-BAILEYS, qrcode=true, syncFullHistory=false — live-only)
2. Evolution POST /webhook/set/{instance}  → points at our webhook, subscribes to the events in 3-sync.md
3. show QR; status=qr_required
   (the wa_accounts row is created at connect, once owner_jid → id is known — step 4)
```

The webhook URL and the shared token come from `.env`, not per account:

```
url    = {WEBHOOK_PUBLIC_BASE_URL}/evolution/api/v1/webhook/{account_id}
auth   = single shared token from .env  (Evolution sends it; backend verifies it)
```

### 2–3. Show & refresh the QR

`POST .../qr` calls `GET /instance/connect/{instance}` and returns the QR straight from Evolution
(it is shown to the user, **not** persisted — QR is fetched live, never stored):

```json
{ "status": "qr_required", "qr_code": "2@y8eK...", "pairing_code": "WZYEH1YY",
  "expires_at": "2026-06-13T12:00:00Z" }
```

UI polls `GET .../qr` every 2–3s while `qr_required`: if connected → say so; else re-call Evolution
and return the newest QR. (Evolution also emits `QRCODE_UPDATED`, but the screen uses polling because
it's simpler and reliable.)

### 4. User scans → connected

Evolution sends `CONNECTION_UPDATE` to our webhook. Backend:

```
1. enqueue the raw event (no DB write); a worker handles it
2. compute account id = uuidv5(owner_jid); upsert wa_accounts
   (INSERT … ON CONFLICT (id) DO UPDATE) — a re-added number lands on the SAME account_id,
   so its existing chats/messages stay attached; nothing is lost
3. connection_state=connected; save phone_number; refresh evolution_instance_name/id
4. stop polling/showing the QR (it was never stored)
5. auto-assign the instance to the org (it was just added to be handled)
6. broadcast wa_account.connected
```

**Deleting + re-adding the same number is safe:** because `id = uuidv5(owner_jid)` doesn't change,
the recreated instance resolves to the same `wa_accounts.id`, and all its `wa_chats` / `wa_messages`
stay attached (see `9-database-schema.md` → `wa_accounts`).

**From this moment, Evolution pushes live events and they're stored immediately** (see `3-sync.md`).
The account is usable right away — there is nothing else to wait for.

## What happens after the scan

**One thing happens: live push.** Every new message/reply/status Evolution sees is pushed to the
webhook, stored raw, then a worker upserts it (`3-sync.md` → "The one mechanism"). The inbox starts
empty and fills in going forward from live events.

```
1. Evolution pushes MESSAGES_UPSERT
2. webhook enqueues the raw event, returns 200     ← in-memory queue; idempotent upserts cover re-delivery
3. worker consumes the queue and upserts immediately (raw kept on wa_messages.raw)
4. if the wa_contact/wa_chat is new → create it from the live event
5. broadcast message.created
6. enqueue an on-demand AI draft (no auto-send in v1)
```

## Sending

```
POST /xchats/api/v1/chats/{id}/messages
  1. create local message status=queued; broadcast message.created
  2. Evolution POST /message/sendText (or sendMedia)
  3. store returned evolution_message_id
  4. status=sent or failed; broadcast message.updated
  5. later: delivered/read arrive via messages.update  (matched on evolution_message_id — see 3-sync.md)
```

Human and AI replies share this pipeline; an AI send just starts from a user-approved `ai_draft`.
Auto-send is a deferred feature (v1 = on-demand drafts only).

## Example timeline

```
12:00:00  user clicks Add WhatsApp Account
12:00:02  backend creates Evolution instance, sets webhook
12:00:04  UI polls QR
12:00:12  user scans
12:00:13  CONNECTION_UPDATE = connected  → live push starts; inbox is empty
12:00:16  a customer messages in → stored & shown in the inbox immediately
12:00:22  AI drafts using the live message (+ any messages since connect)
```

Useful the instant it connects; each chat builds up from live events going forward.

## Tables

The authoritative data model lives in **`9-database-schema.md`** (schema `xchats`). The ones this
flow touches: `wa_accounts`, `wa_contacts`, `wa_chats`, `wa_messages`, `ai_suggestions`. (QR is fetched
live from Evolution and never stored, so there is no QR table; the raw event rides the in-memory
queue, so there is no `evolution_events` / `jobs` table either.) Load-bearing constraints (the reason
live upserts never duplicate):

```
UNIQUE (account_id, remote_jid)            on wa_chats
UNIQUE (account_id, evolution_message_id)  on wa_messages
UNIQUE (account_id, phone_jid)             on wa_contacts
```
