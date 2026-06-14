# WhatsApp Connection Example

This document describes the flow for adding a WhatsApp account from our app UI, showing an always-refreshing QR code, syncing old chats/messages/contacts, receiving live messages during sync, and using that data for AI-assisted replies.

The UI never talks to Evolution directly. The UI talks to our backend. The backend owns product state and calls Evolution as the WhatsApp transport.

## Services Involved

```text
Vue UI
  - account setup
  - QR screen
  - sync progress
  - inbox/chat experience

Backend API
  - WhatsApp account CRUD
  - QR polling endpoint
  - chat/contact/message APIs
  - send-message endpoint

Evolution Adapter
  - backend module that calls Evolution REST API
  - normalizes Evolution responses

Webhook Receiver
  - receives Evolution events
  - stores raw payloads quickly
  - returns 200 quickly

Workers
  - process webhook events
  - sync old contacts/chats/messages
  - download media
  - trigger AI draft jobs
  - retry failed work

Realtime Gateway
  - WebSocket or SSE from backend to UI
  - pushes account status, sync progress, new messages, and AI drafts

AI Assistant
  - reads normalized app data
  - suggests replies, summaries, and media
```

## Evolution Endpoints We Use

Called only by backend:

```text
POST /instance/create
GET  /instance/connect/{instance}
GET  /instance/connectionState/{instance}
POST /webhook/set/{instance}
POST /chat/findContacts/{instance}
POST /chat/findChats/{instance}
POST /chat/findMessages/{instance}
POST /message/sendText/{instance}
POST /message/sendMedia/{instance}
```

Evolution supports instance creation with `syncFullHistory`, QR/pairing generation through `GET /instance/connect/{instance}`, webhook setup through `POST /webhook/set/{instance}`, chat/contact/message lookup through `chat/find*`, and sending through `message/sendText` / `message/sendMedia`.

## App Endpoints We Expose

```text
# WhatsApp accounts = simplified Evolution manager
GET  /xchats/api/v1/whatsapp-accounts                 # ALL Evolution instances, each with status + assigned flag
POST /xchats/api/v1/whatsapp-accounts                 # add: create a new Evolution instance (by name)
GET  /xchats/api/v1/whatsapp-accounts/{id}
POST /xchats/api/v1/whatsapp-accounts/{id}/qr         # (re)generate the QR to connect
GET  /xchats/api/v1/whatsapp-accounts/{id}/qr         # poll QR / connection status
POST /xchats/api/v1/whatsapp-accounts/{id}/assign     # start handling this instance for the org
POST /xchats/api/v1/whatsapp-accounts/{id}/unassign   # stop handling it
POST /xchats/api/v1/whatsapp-accounts/{id}/sync       # trigger (re)sync
GET  /xchats/api/v1/whatsapp-accounts/{id}/sync       # sync progress

# Evolution -> us (authenticated with the single shared token from .env)
POST /evolution/api/v1/webhook/{whatsapp_account_id}

# Inbox
GET  /xchats/api/v1/conversations
GET  /xchats/api/v1/conversations/{id}/messages
POST /xchats/api/v1/conversations/{id}/messages
POST /xchats/api/v1/conversations/{id}/assign

GET  /xchats/api/v1/realtime
```

Transport:

- QR refresh: UI polling.
- Evolution events: webhook to backend.
- Inbox updates: WebSocket or SSE.
- Send message: normal HTTP request from UI to backend.
- Old sync/resync: background worker with progress updates.

## Managing Instances (the simplified manager)

xchats **reuses a running Evolution**. The WhatsApp-accounts page mirrors Evolution's `/manager`,
with a simpler UX:

- `GET /xchats/api/v1/whatsapp-accounts` returns **all Evolution instances** (fetched from Evolution), each
  with connection status and an **assigned** flag.
- **Assign / unassign** toggles whether xchats handles an instance. Only **assigned** instances are
  processed; webhook events for unassigned instances are ignored. Old/pre-existing instances appear
  in the list with an **Assign** button.

## Flow: Add WhatsApp Account

### 1. User clicks "Add WhatsApp Account"

UI opens a modal for an **instance name** + scan instructions, then calls:

```text
POST /xchats/api/v1/whatsapp-accounts
```

```json
{ "display_name": "Sales WhatsApp", "instance_name": "sales", "sync_full_history": true }
```

Backend:

```text
1. creates whatsapp_accounts row (status=creating, organization_id=null until assigned)
2. calls Evolution POST /instance/create  (integration=WHATSAPP-BAILEYS, qrcode=true, syncFullHistory=true)
3. calls Evolution POST /webhook/set/{instance} pointing at our webhook edge
4. updates account status=qr_required
5. returns account id
```

The webhook Evolution is told to call (creds from `.env`, not per-account):

```text
url    = {WEBHOOK_PUBLIC_BASE_URL}/evolution/api/v1/webhook/{whatsapp_account_id}
auth   = single shared token from .env (Evolution sends it; backend verifies it)
events = MESSAGES_UPSERT, MESSAGES_UPDATE, SEND_MESSAGE, CONNECTION_UPDATE, QRCODE_UPDATED,
         CONTACTS_*, CHATS_*
```

(`WEBHOOK_PUBLIC_BASE_URL`, the Evolution base URL + global API key, and the shared webhook token
all live in `.env`; tunables/seed live in `config.yaml` — see `2-architecture.md`.)

### 2. User clicks "Generate WhatsApp QR"

UI calls:

```text
POST /xchats/api/v1/whatsapp-accounts/{id}/qr
```

Backend:

```text
1. calls Evolution GET /instance/connect/{instance}
2. stores QR/pairing result in whatsapp_qr_sessions
3. returns QR data to UI
```

Response:

```json
{
  "status": "qr_required",
  "qr_code": "2@y8eK+bjtEjUWy9/FOM...",
  "pairing_code": "WZYEH1YY",
  "expires_at": "2026-06-13T12:00:00Z"
}
```

### 3. QR stays updated through polling

UI polls:

```text
GET /xchats/api/v1/whatsapp-accounts/{id}/qr
```

Every 2-3 seconds while account status is `qr_required`.

Backend:

```text
1. if account is connected, return connected
2. if cached QR is fresh, return cached QR
3. otherwise call Evolution GET /instance/connect/{instance}
4. store and return the newest QR
```

Evolution may also send `QRCODE_UPDATED` events. We store them if received, but the UI still uses polling because it is simple and reliable for this screen.

### 4. User scans QR

Evolution sends:

```text
CONNECTION_UPDATE
```

to:

```text
POST /evolution/api/v1/webhook/{whatsapp_account_id}
```

Backend:

```text
1. stores raw event in evolution_events
2. updates whatsapp_accounts.connection_status=connected
3. stores owner_jid / phone_number if available
4. marks QR session as consumed
5. auto-assigns the instance to the organization (it was just added to be handled)
6. starts initial sync job
7. broadcasts whatsapp_account.connected and sync.started
```

## Initial Sync

The initial sync imports historical data from Evolution:

```text
1. POST /chat/findContacts/{instance}
2. POST /chat/findChats/{instance}
3. POST /chat/findMessages/{instance} for each chat
4. upsert contacts, identities, conversations, messages
5. enqueue media download jobs
6. update sync progress
7. mark sync as complete, partial, or failed
```

UI can show:

```text
contacts synced: 412
chats synced: 128
messages synced: 18,920
media pending: 241
live messages during sync: 7
```

Progress comes from:

```text
GET /xchats/api/v1/whatsapp-accounts/{id}/sync
```

and realtime events:

```text
sync.progress
sync.completed
sync.partial
sync.failed
```

## Live Messages While Syncing

Live events must not wait for old sync to finish.

If a new WhatsApp message arrives while the account is syncing:

```text
1. Evolution sends MESSAGES_UPSERT.
2. Webhook receiver stores the raw event and returns 200.
3. Worker normalizes the event immediately.
4. If contact/conversation is not synced yet, create it from the live event.
5. Save the message with source=live_webhook.
6. Broadcast message.created to UI.
7. Mark conversation history_status=syncing.
8. Enqueue targeted chat sync for this remote_jid.
9. AI draft waits for targeted sync if possible.
```

The targeted sync calls:

```text
POST /chat/findMessages/{instance}
```

with the active `remoteJid`, then upserts recent history for that chat before AI drafts a response.

Important policy:

```text
During initial sync, AI may draft but should not auto-send.
```

This prevents AI from answering with incomplete context. The draft can include `context_status=partial` or `context_status=syncing`.

## Tables We Need

### organizations

Company/workspace. Carries the auto-response policy.

```text
id, name,
auto_response_mode,     -- NEVER | CONFIGURE_TIME | ALWAYS
auto_response_window,   -- time window used when mode = CONFIGURE_TIME
created_at, updated_at
```

### users

Application users.

```text
id, email, name, password_hash, created_at, updated_at
```

### organization_members

Membership. All members have equal permissions in v1.

```text
id, organization_id, user_id, created_at
```

### whatsapp_accounts

Mirrors an Evolution instance. `organization_id` is **null when unassigned** and set when
assigned — only assigned accounts are processed. Shared Evolution credentials (base URL, global
API key, webhook token) live in `.env`, **not** per row.

```text
id
organization_id           -- null = unassigned (not handled); set = assigned to the org
assigned                  -- convenience flag (organization_id IS NOT NULL)
display_name
evolution_instance_name
evolution_instance_id
owner_jid
phone_number
connection_status
sync_status
history_status
last_live_event_at
last_successful_sync_at
created_at
updated_at
```

### whatsapp_qr_sessions

Latest QR/pairing data shown to the user.

```text
id, whatsapp_account_id, qr_code, pairing_code, status, expires_at, created_at, consumed_at
```

### evolution_events

Raw Evolution webhook payloads for replay/debug/idempotency.

```text
id
organization_id
whatsapp_account_id
event_type
external_event_id
payload jsonb
processing_status
processed_at
processing_error
created_at
```

### contacts

Customer profile.

```text
id, organization_id, display_name, phone_number, profile_picture_url, created_at, updated_at
```

### contact_identities

Normalized identities to solve `@lid`, phone JID, and phone-number mapping.

```text
id
organization_id
contact_id
whatsapp_account_id
identity_type
value
created_at
```

Identity types:

```text
phone
phone_jid
lid_jid
push_name
```

### conversations

Chat thread for one WhatsApp account and one contact/remote JID.

```text
id
organization_id
whatsapp_account_id
contact_id
remote_jid
status
assignee_member_id
last_message_at
last_message_preview
unread_count
history_status
created_at
updated_at
```

Unique:

```text
unique(whatsapp_account_id, remote_jid)
```

### messages

Normalized inbound/outbound message.

```text
id
organization_id
whatsapp_account_id
conversation_id
contact_id
direction
sender_type
sender_member_id
evolution_message_id
remote_jid
participant_jid
message_type
content
timestamp
status
source
raw_payload jsonb
created_at
updated_at
```

Unique:

```text
unique(whatsapp_account_id, evolution_message_id)
```

### message_media

Media metadata and storage result.

```text
id
message_id
media_type
mimetype
file_name
file_size
duration_ms
evolution_media_url
storage_url
thumbnail_url
transcription
download_status
created_at
updated_at
```

### sync_jobs

Initial sync, resync, targeted chat sync.

```text
id
organization_id
whatsapp_account_id
job_type
status
progress_current
progress_total
contacts_synced
chats_synced
messages_synced
last_error
started_at
finished_at
```

### ai_drafts

Suggested response from AI.

```text
id
organization_id
conversation_id
trigger_message_id
draft_text
suggested_media jsonb
context_status
status
created_at
updated_at
```

## Handling Incoming Events

### Incoming message

Evolution event:

```text
MESSAGES_UPSERT
```

Backend:

```text
1. store raw event
2. normalize identifiers
3. upsert contact and contact identities
4. upsert conversation
5. upsert message
6. update last_message_at and unread_count
7. broadcast message.created
8. enqueue targeted sync if account is still syncing
9. enqueue AI draft job
```

### Message status

Evolution events:

```text
MESSAGES_UPDATE
SEND_MESSAGE
```

Backend:

```text
1. find message by whatsapp_account_id + status_correlation_id
   (messages.update keys on data.keyId / data.messageId, NOT the dedup key data.key.id —
    correlation is UNVERIFIED; confirm with a matched send->update capture, see 9-database-schema.md)
2. update status: pending, sent, delivered, read, failed (monotonic — never downgrade on a late event)
3. broadcast message.updated
```

> The outbound `send.message` stores `key.id` as `evolution_message_id` (the dedup key) **and** must
> also store the id that `messages.update` will later key on (`status_correlation_id`) so the two
> paths actually join. In the current fixtures `key.id` (22 chars) ≠ `keyId` (40 chars), so do not
> assume they match — settle it with the matched-pair capture first.

### Contact/chat sync events

Evolution events:

```text
CONTACTS_SET
CONTACTS_UPSERT
CHATS_SET
CHATS_UPSERT
```

Backend:

```text
1. upsert contacts and identities
2. upsert conversations
3. preserve assignment/status
4. broadcast conversation.created or conversation.updated
```

## Sending Responses

Human sends from UI:

```text
POST /xchats/api/v1/conversations/{id}/messages
```

Backend:

```text
1. create local message with status=queued
2. broadcast message.created
3. call Evolution POST /message/sendText/{instance}
   or POST /message/sendMedia/{instance}
4. store returned evolution_message_id
5. update status=sent or failed
6. broadcast message.updated
```

AI-assisted send uses the same endpoint. The difference is that the message starts from an `ai_draft` approved by a member.

Optional future auto-send:

```text
1. rules allow auto-send
2. AI creates reply
3. backend creates outbound message with sender_type=ai
4. backend sends through Evolution
5. status updates return through webhooks
```

Auto-send should be disabled while the account or conversation history is still syncing.

## Handling Duplicates And Gaps

Same message can arrive from live webhook and old sync. We avoid duplicates with:

```text
unique(whatsapp_account_id, evolution_message_id)
```

If a live event creates a conversation before sync sees it, sync later enriches the same rows.

If old history is incomplete:

```text
history_status=partial
```

AI receives this context and should not pretend full history is available.

## Example Timeline

```text
12:00:00 user clicks Add WhatsApp Account
12:00:02 backend creates Evolution instance
12:00:04 UI starts polling QR
12:00:12 user scans QR
12:00:13 Evolution sends CONNECTION_UPDATE=open
12:00:15 backend starts initial sync
12:00:16 customer sends new WhatsApp message
12:00:17 backend stores live message and shows it in inbox
12:00:18 backend starts targeted sync for that chat
12:00:22 AI creates draft using targeted history + live message
12:03:00 initial sync completes or marks partial
```

This makes the account useful immediately after connection while still building historical context for agents and AI.

