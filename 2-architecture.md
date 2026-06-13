# Architecture

The product should be built as a WhatsApp-first messaging workspace with a clear boundary between transport, product state, realtime UI, and AI assistance.

## High-Level Flow

```text
Evolution API
  -> backend webhook receiver
  -> database normalization
  -> realtime broadcast
  -> Vue team inbox

Vue team inbox
  -> backend send-message API
  -> Evolution API
  -> webhook/status update
  -> realtime broadcast

Incoming message
  -> backend
  -> worker
  -> AI assistant
  -> draft/suggestion/action
  -> realtime broadcast
```

## Apps And Components

### Evolution Core

Evolution API is the WhatsApp transport layer.

Responsibilities:

- connect WhatsApp accounts
- maintain WhatsApp sessions
- expose QR/connect status
- send text and media messages
- emit webhook events for contacts, chats, messages, status updates, and connection updates
- expose available historical chats/messages where possible

Evolution should not be treated as the main product database.

### Backend API

The backend is the source of truth for the product.

Suggested stack: Go + PostgreSQL.

Responsibilities:

- organization and member management
- WhatsApp account registration
- webhook receiver for Evolution events
- normalized contacts, conversations, and messages
- assignment/reassignment
- send-message API
- media metadata
- AI draft endpoints
- realtime event publishing
- idempotency, retries, and error tracking

### Workers

Workers process work that should not block user requests or webhook responses.

Responsibilities:

- process raw Evolution events
- sync old contacts/chats/messages
- download and store media
- update delivery/read statuses
- run AI draft jobs
- retry failed outbound sends
- reconcile gaps after downtime

Workers can initially run inside the same Go binary as background goroutines. They can later be split into separate processes if load grows.

### Realtime Gateway

The realtime layer pushes updates from the backend to connected UI clients.

Options:

- Server-Sent Events for a simple first version
- WebSockets for richer bidirectional realtime behavior

Events:

- `message.created`
- `message.updated`
- `conversation.created`
- `conversation.updated`
- `assignment.changed`
- `ai_draft.created`
- `whatsapp_account.status_changed`
- `sync.progress`

### UI

The UI is a lightweight Chatwoot-style team inbox.

Suggested stack: Vue 3 + TypeScript.

Core screens:

- login
- organization workspace
- WhatsApp accounts/settings
- conversation list
- chat view
- assignment control
- contact panel
- AI suggestions panel
- media picker

The first version should avoid complex helpdesk features such as SLA, campaigns, detailed permissions, macros, reports, billing, and omnichannel automation.

### AI Assistant

The AI assistant helps members respond faster.

Responsibilities:

- generate reply drafts
- suggest media files
- summarize long conversations
- suggest next action
- detect missing information
- optionally auto-reply only when explicitly enabled by rules

The assistant should read from the app database, not directly from Evolution tables.

### Database

PostgreSQL should store product state.

Core tables:

```text
organizations
users
organization_members
whatsapp_accounts
contacts
contact_identities
conversations
messages
message_media
conversation_assignments
ai_drafts
evolution_events
sync_jobs
```

Important constraints:

```text
unique(whatsapp_account_id, remote_jid) on conversations
unique(whatsapp_account_id, evolution_message_id) on messages
unique(contact_id, whatsapp_account_id, identity_type, value) on contact_identities
```

### Media Store

Media files should not live only inside Evolution.

The app should store:

- original Evolution media metadata
- local/object-storage URL
- MIME type
- size
- duration for audio/video
- thumbnail when available
- transcription when available
- download status

Local disk can be used for development. Object storage should be used for production.

### Queue

The first version can use a PostgreSQL-backed job table.

Later, Redis, NATS, or a dedicated queue can be added if throughput requires it.

## Suggested Deployment Shape

Simple v1:

```text
frontend
backend-api
backend-worker
postgres
object-storage
evolution-api
```

For development, `backend-api` and `backend-worker` can be one process.

## Data Ownership

Evolution owns WhatsApp transport details.

The app owns:

- organizations
- members
- connected account configuration
- contacts
- conversations
- assignments
- normalized messages
- media references
- AI drafts
- sync state
- raw event history

