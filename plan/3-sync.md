# Sync And Events

This document describes how the product should handle live Evolution events, old chat sync, gaps, retries, media, and edge cases.

## Sync Model (High Level)

### The chain: WhatsApp account → Evolution → us

The WhatsApp account is linked to Evolution (Baileys); Evolution holds the live session and is
the **only** thing that talks to WhatsApp. We never touch WhatsApp directly. We get data from
Evolution two ways, and normalize both into our Postgres:

- **Push** — Evolution sends live events to our webhook as they happen.
- **Pull** — Evolution exposes existing history via `chat/findContacts`, `chat/findChats`,
  `chat/findMessages`, and media bytes via `getBase64FromMediaMessage`.

### Three sync flows

> **v1 builds only live sync.** Staging: **v1 = live sync only**; **v2 = targeted recent sync**
> (fetch the last ~10–20 messages for a conversation on first appearance, if Evolution makes it
> easy); **v3 = full initial sync + reconcile scheduler**. The design for all three stays here, but
> full backfill can become the whole project and can bury a fresh inbox under stale messages on
> connect — so it is staged, not built first. The AI must always carry the `history_state`
> (`live_only | partial | unknown`) so it never pretends to know older history.

1. **Live sync — keep new data in sync (steady state). [v1]**
   Evolution → webhook edge → store raw event → enqueue → worker normalizes → upsert
   contact / conversation / message / status. Every new inbound, outbound, and status change
   flows through here in real time.
2. **Initial (old) sync — collect history when an account connects. [v2 targeted / v3 full]**
   A background job pulls existing **contacts, chats, and messages** from Evolution's `find*`
   endpoints and upserts them through the **same code path** as live. Recent/active chats are
   imported first so the inbox is usable immediately; the rest backfills. Media is fetched
   lazily by a separate job.
3. **Reconcile — gap-fill for resilience. [v3]**
   A periodic job re-pulls recent chats/messages to catch anything missed during downtime or a
   dropped webhook. Because upserts are idempotent, re-pulling never creates duplicates.

### What keeps it correct

- **One upsert path** for live + initial + reconcile — duplicates are impossible and fields are
  enriched, never blindly overwritten.
- **Stable identity / dedup:** messages keyed `(whatsapp_account_id, evolution_message_id)`,
  conversations `(whatsapp_account_id, remote_jid)`, contacts via `contact_identities` —
  resolving `@lid` ↔ phone using the message key's `remoteJidAlt`.
- **History status** per conversation (`full | partial | live_only | syncing | unknown`) plus
  explicit gap markers, so the UI and the AI know when older history may be missing.
- **Status lifecycle** (sent → delivered → read) applied monotonically from `messages.update`
  events, so a late event never downgrades a message.

The sections below are the detailed Q&A for each concern.

## Q: How do we receive live events?

Evolution should be configured to send webhooks directly to the backend:

```text
POST /evolution/api/v1/webhook/{whatsapp_account_id}
```

The handler should:

```text
1. verify webhook secret/account token
2. store raw payload in evolution_events
3. acknowledge quickly with 200
4. enqueue processing job
```

The webhook request should not run AI or heavy media downloads directly.

## Q: Which Evolution events do we care about?

Minimum live events:

```text
CONNECTION_UPDATE
QRCODE_UPDATED
CONTACTS_SET
CONTACTS_UPSERT
CONTACTS_UPDATE
CHATS_SET
CHATS_UPSERT
CHATS_UPDATE
MESSAGES_SET
MESSAGES_UPSERT
MESSAGES_UPDATE
MESSAGES_DELETE
SEND_MESSAGE
```

The exact payload shape should be wrapped behind an internal normalizer so Evolution version changes do not leak into the whole app.

## Q: What if we add a WhatsApp account in the middle of existing conversations?

That is expected.

When an account is added, the app should:

```text
1. create whatsapp_accounts row
2. connect or create Evolution instance
3. configure webhook to the backend
4. enable history sync where available
5. start sync job for contacts/chats/messages
6. continue accepting live webhooks during sync
```

The conversation history may be partial. The app should mark the account/conversation with a history status:

```text
full
partial
live_only
unknown
```

AI should be told when history is partial.

## Q: How do we sync old chats and messages?

Old sync should be a background job.

It should import:

- contacts
- chats
- messages
- media metadata
- message statuses when available

Old sync and live webhooks must use the same upsert code path. This prevents duplicates when the same message appears both in a sync payload and a live event.

## Q: What if live events arrive while old sync is running?

Live events should be processed immediately.

Sync should never assume it owns the timeline. Every imported item should be upserted with unique keys:

```text
messages: unique(whatsapp_account_id, evolution_message_id)
conversations: unique(whatsapp_account_id, remote_jid)
contact_identities: unique(contact_id, whatsapp_account_id, identity_type, value)
```

If a live message already exists, sync updates missing fields only.

## Q: What if we miss an event?

The app should assume missed events can happen.

Reasons:

- backend downtime
- Evolution restart
- network failure
- webhook timeout
- deployment restart
- queue processing error

Mitigation:

```text
1. store raw webhook events before processing
2. make event processing idempotent
3. retry failed event jobs
4. periodically reconcile recent chats/messages from Evolution
5. expose account sync health in UI
```

For each WhatsApp account, keep:

```text
last_live_event_at
last_successful_sync_at
last_reconcile_at
sync_status
sync_error
```

## Q: What if there is a gap between synced messages and live messages?

Mark the gap explicitly.

Example:

```text
synced_until: 2026-06-10 10:00
first_live_message_at: 2026-06-13 12:30
gap_detected: true
```

The UI can show a small system note:

```text
Some earlier messages may be missing.
```

AI should receive this as context and avoid pretending to know the missing history.

## Q: What if Evolution returns only partial old history?

Treat partial history as normal.

The app should not block the account. It should:

- import what is available
- mark sync as partial
- continue live operation
- allow manual resync
- preserve raw sync errors

## Q: How do we avoid duplicate messages?

Every message should have a stable external identity.

Use:

```text
whatsapp_account_id
evolution_message_id
remote_jid
from_me
timestamp
```

Primary dedupe should be:

```text
unique(whatsapp_account_id, evolution_message_id)
```

If Evolution message ID is missing, use a fallback fingerprint:

```text
whatsapp_account_id + remote_jid + timestamp + direction + content_hash
```

Fallback fingerprints should be treated carefully because two messages can have the same text.

## Q: How do we handle `@lid`, phone JIDs, and phone numbers?

Do not store only one identifier.

Use `contact_identities`:

```text
lid_jid
phone_jid
phone
push_name
```

When a new event arrives:

```text
1. extract all identifiers from the payload
2. find existing contact by any identity
3. add missing identities to the same contact
4. create conversation under the correct whatsapp_account_id and remote_jid
```

This prevents the same customer from becoming separate contacts because one event uses `@lid` and another uses `@s.whatsapp.net`.

## Q: What if the same contact writes to two different WhatsApp accounts?

The contact can be shared, but conversations should be separate.

Example:

```text
Contact: +77000000000
Conversation A: contact with sales WhatsApp account
Conversation B: contact with support WhatsApp account
```

Assignments and statuses belong to conversations, not directly to contacts.

## Q: How do we send messages?

All outbound messages should go through the backend:

```text
POST /xchats/api/v1/conversations/{conversation_id}/messages
```

Flow:

```text
1. create local message with status=queued
2. call Evolution sendText/sendMedia
3. store Evolution message ID if returned
4. update status=sent or failed
5. broadcast realtime update
6. later update delivered/read from Evolution status events
```

Human replies and AI replies should share the same send pipeline.

## Q: What if a message is sent from another app (WhatsApp mobile or Web)?

This is normal and must stay in sync. WhatsApp is multi-device: the operator may reply from the
phone app or WhatsApp Web instead of xchats. Because Evolution is a linked device on the same
account, it still sees that activity and emits it to us — so xchats reflects the **full**
conversation regardless of where the reply was typed.

How Evolution reports it:

- An outbound `messages.upsert` with `key.fromMe: true` (and/or a `send.message` event), carrying
  a `source` such as `ios`, `android`, or `web`.

How we handle it — the **same upsert path** as everything else:

```text
1. normalize and upsert into the conversation as an OUTBOUND message
2. sender_type = external_account   (sent outside xchats — not a known member, not the AI)
3. source = live_webhook
4. if the chat/contact is new (operator messaged someone we hadn't seen), create the
   conversation + contact from this event (resolve lid/phone via remoteJidAlt)
5. update conversation last_message_at / ordering and broadcast message.created
```

Delivery/read status for these arrive via `messages.update` like any other outbound message.
Anything sent from another app while xchats was down is picked up by initial sync / reconcile
through the same idempotent upsert (deduped on `evolution_message_id`), so the chats and messages
lists converge no matter which device sent. In short: **`fromMe` but not from us → outbound /
external**, and it appears in the inbox exactly like an app-sent reply.

## Q: How do we handle AI responses?

AI should run after the inbound message is stored.

Default mode:

```text
incoming message
  -> save
  -> enqueue AI draft job
  -> generate draft
  -> save ai_draft
  -> broadcast to UI
```

Optional auto-reply mode:

```text
incoming message
  -> save
  -> rules decide auto-reply is allowed
  -> AI generates reply
  -> backend sends through same outbound pipeline
```

Auto-reply should be opt-in per account or conversation.

## Q: How does AI use old synced context?

AI reads from the app database:

```text
last N messages
conversation summary
contact metadata
business knowledge
available media assets
history_status
gap indicators
```

For long conversations, workers should maintain a rolling summary so prompts do not become too large.

## Q: How do we handle media files?

Media should be processed asynchronously.

Message is saved first:

```text
message_type=image/audio/video/document/sticker
media_status=pending
```

Worker then:

```text
1. downloads media from Evolution
2. validates type/size
3. stores file in local/object storage
4. generates thumbnail or metadata where useful
5. updates media_status=ready or failed
6. broadcasts update
```

## Q: How do we handle images?

Store original image and thumbnail.

AI can use image captions or metadata if available. Do not assume the AI can inspect every image unless image analysis is explicitly enabled.

## Q: How do we handle audio and voice notes?

Store the audio file and duration.

Optionally run transcription:

```text
media_status=ready
transcription_status=pending/ready/failed
```

AI should use the transcript when available. Until then, the UI should show the audio message normally.

## Q: How do we handle videos?

Store the original video, size, duration, and thumbnail if possible.

Transcription can be optional if the video contains speech. Video processing should have strict size and timeout limits.

## Q: How do we handle stickers?

Store stickers as media messages.

Do not rely on stickers for AI context unless they can be converted to an image preview and interpreted. In most cases, stickers should be treated as non-text reactions.

## Q: How do we handle documents?

Store the file, filename, MIME type, and size.

Text extraction can be added later for PDFs or documents. Until extraction exists, AI should only know that a document was sent.

## Q: What if media download fails?

Keep the message.

Set:

```text
media_status=failed
media_error=<reason>
```

Allow manual retry. The conversation should not break because one media file failed.

## Q: What if a message is deleted?

Keep a local record but mark it deleted.

```text
deleted_at
deleted_by_remote=true
content_hidden=true
```

This preserves auditability and avoids confusing message gaps.

## Q: What if a message is edited?

If Evolution emits edits, store current content and optionally keep previous content in a message revisions table.

For v1, updating the message content and marking `edited_at` is enough.

## Q: What if delivery/read status arrives before the message exists?

Store the raw event and retry processing.

The processor can create a placeholder message or wait until the original message arrives. Prefer retry first.

## Q: What if the outbound send succeeds but the backend times out?

This is a classic uncertain state.

Mitigation:

- store local queued message before sending
- use idempotency key where possible
- reconcile recent outbound messages from Evolution
- show status as `unknown` or `pending_confirmation` until resolved

## Q: How do we handle webhook retries?

Webhook processing must be idempotent.

Evolution may retry, and the backend may process the same payload more than once. Raw events can duplicate; normalized messages should not.

## Q: How do we handle worker failure?

Every event/job should have:

```text
attempt_count
next_attempt_at
last_error
processed_at
```

After max attempts, mark the job failed and expose it in an admin/debug screen.

## Q: How do we handle Evolution disconnects?

Use connection events to update account status:

```text
connected
connecting
qr_required
disconnected
error
```

The UI should show account health. Sending should be blocked or queued when the account is disconnected.

## Q: How do we handle multiple members working at the same time?

Assignments are conversation-level state.

When assignment changes:

```text
1. update conversation assignee
2. write assignment event
3. broadcast assignment.changed
```

Sending messages should not require assignment in v1 unless the product decides to enforce that later.

## Q: How do we handle unread counts?

Unread counts should be app-level, not Evolution-level.

Start simple:

- incoming customer message increments unread count
- opening conversation by a member can mark it read for that member
- outgoing member reply can mark the conversation read for that member

Later, unread state can become per-member.

## Q: How do we handle groups?

**v1: drop `@g.us` events at the webhook/normalizer before upsert** — `process_event` and `ai_draft`
never run for group JIDs (a group message would otherwise produce a nonsensical single-contact
draft). This is the cheapest safe default; no `conversation_type`/participants schema is needed in v1.

Group chats introduce participants, mentions, sender identity per message, and different assignment
behavior. **If later included (v2+),** conversations need:

```text
conversation_type=direct/group
participants table
```

Direct chats are completed first regardless.

## Q: How do we safely change Evolution versions?

Keep Evolution-specific payload parsing isolated in one package/module.

Store raw events so old payloads can be replayed against updated normalizers during development.

## Q: What is the minimum reliable sync strategy for v1?

Use this:

```text
1. webhook receiver stores raw events
2. processor upserts normalized data
3. sync job imports available old contacts/chats/messages
4. periodic reconcile checks recent chats/messages
5. UI shows sync status and gaps
6. AI receives history_status and gap context
```

This gives a practical balance: live operation works immediately, old history improves context when available, and missing data is visible instead of hidden.

