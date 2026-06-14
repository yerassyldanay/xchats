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
Vue UI            account setup, QR screen, sync progress, inbox/chat
Backend API       account CRUD, QR polling, chat/message APIs, send endpoint
Evolution adapter backend module that calls Evolution's REST API + normalizes responses
Webhook receiver  catches Evolution events, stores raw, returns 200 fast
Workers           process events, pull history, download media, run AI drafts, retry
Realtime gateway  WebSocket/SSE → UI (account status, sync progress, new messages, drafts)
AI assistant      reads normalized app data, suggests replies
```

## Endpoints

Evolution (called only by backend):

```
POST /instance/create               POST /webhook/set/{instance}
GET  /instance/connect/{instance}   GET  /instance/connectionState/{instance}
POST /message/sendText/{instance}
```
(`chat/findContacts|findChats|findMessages` and `sendMedia` are deferred — no history pull, text-only.)
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
GET  /xchats/api/v1/conversations
GET/POST .../conversations/{id}/messages
POST .../conversations/{id}/assign
GET  /xchats/api/v1/realtime
```

Transport: QR = UI polling; events = webhook; inbox updates = WebSocket/SSE; send = plain HTTP.
(No history pull in v1 — `chat/find*` and the `/sync` endpoints are deferred.)

## The accounts page is a thin Evolution manager

xchats **reuses a running Evolution**. The page mirrors Evolution's `/manager` with simpler UX:
`GET .../whatsapp-accounts` lists **all** Evolution instances with status and an **assigned** flag.
**Assign/unassign** toggles whether xchats handles an instance — only assigned instances are
processed; webhook events for unassigned ones are ignored.

## Connect flow

### 1. Add account

UI asks for an instance name, then `POST /xchats/api/v1/whatsapp-accounts`:

```json
{ "display_name": "Sales WhatsApp", "instance_name": "sales", "sync_full_history": true }
```

Backend:

```
1. create wa_accounts row (connection_state=creating, organization_id=null until assigned)
2. Evolution POST /instance/create   (WHATSAPP-BAILEYS, qrcode=true, syncFullHistory=true)
3. Evolution POST /webhook/set/{instance}  → points at our webhook, subscribes to the events in 3-sync.md
4. connection_state=qr_required
```

The webhook URL and the shared token come from `.env`, not per account:

```
url    = {WEBHOOK_PUBLIC_BASE_URL}/evolution/api/v1/webhook/{account_id}
auth   = single shared token from .env  (Evolution sends it; backend verifies it)
```

### 2–3. Show & refresh the QR

`POST .../qr` calls `GET /instance/connect/{instance}`, stores the result in
`wa_qr_sessions`, returns it:

```json
{ "status": "qr_required", "qr_code": "2@y8eK...", "pairing_code": "WZYEH1YY",
  "expires_at": "2026-06-13T12:00:00Z" }
```

UI polls `GET .../qr` every 2–3s while `qr_required`: if connected → say so; if the cached QR is
fresh → return it; else re-call Evolution and return the newest. (Evolution also emits
`QRCODE_UPDATED`, which we store — but the screen uses polling because it's simpler and reliable.)

### 4. User scans → connected

Evolution sends `CONNECTION_UPDATE` to our webhook. Backend:

```
1. store raw event
2. connection_state=connected; save owner_jid / phone_number
3. mark the QR session consumed
4. auto-assign the instance to the org (it was just added to be handled)
5. broadcast wa_account.connected
```

**From this moment, Evolution pushes live events and they're stored immediately** (see `3-sync.md`).
The account is usable right away — there is nothing else to wait for.

## What happens after the scan

**One thing happens: live push.** Every new message/reply/status Evolution sees is pushed to the
webhook, stored raw, then a worker upserts it (`3-sync.md` → "The one mechanism"). The inbox starts
empty and fills in going forward. There is **no history import in v1** — chats that existed before
the scan are not pulled. (When history is wanted later it's purely additive on the same upsert path;
see `3-sync.md` → "Deferred".)

```
1. Evolution pushes MESSAGES_UPSERT
2. webhook stores raw event, returns 200          ← durable, nothing lost
3. worker upserts it immediately
4. if the contact/conversation is new → create it from the live event
5. broadcast message.created
6. enqueue an AI draft job (on-demand; no auto-send in v1)
```

## Sending

```
POST /xchats/api/v1/conversations/{id}/messages
  1. create local message status=queued; broadcast message.created
  2. Evolution POST /message/sendText (or sendMedia)
  3. store returned evolution_message_id
  4. status=sent or failed; broadcast message.updated
  5. later: delivered/read arrive via messages.update  (status_correlation_id — see 3-sync.md)
```

Human and AI replies share this pipeline; an AI send just starts from a member-approved `ai_draft`.
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

Useful the instant it connects; each conversation accumulates history going forward from live
events. No backfill, no blocking.

## Tables

The authoritative data model lives in **`9-database-schema.md`** (schema `xchats`). The ones this
flow touches: `wa_accounts`, `wa_qr_sessions` (deferred with the connect UI),
`evolution_events`, `jobs`, `contacts`, `contact_identities`, `conversations`, `messages`,
`ai_drafts`. Load-bearing constraints (the reason live upserts never duplicate):

```
UNIQUE (account_id, remote_jid)                                on conversations
UNIQUE (account_id, evolution_message_id)                      on messages
UNIQUE (contact_id, account_id, identity_kind, identity_value) on contact_identities
```
