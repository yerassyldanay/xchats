# xchats — Hard Technical Implementation Review

## Position

I am reviewing this as the person who has to implement, debug, operate, and maintain the first version.

The idea is good: **AI should suggest replies, not auto-send them**. But the current documentation describes a much larger platform than the initial idea. It mixes:

1. WhatsApp transport infrastructure.
2. Team inbox / Chatwoot-like workspace.
3. AI assistant + KB CMS + media catalog + pricing engine.
4. Sync/reconcile/media pipeline.
5. Admin tooling and testing harness.

Technically, this is not one MVP. This is a platform roadmap.

My implementation decision: **do not implement the full plan as written.** Implement the smallest stable vertical slice that proves the product loop:

```text
Inbound WhatsApp message
→ normalized DB row
→ visible in UI
→ AI draft generated
→ human edits/approves
→ outbound WhatsApp send
→ delivery/read status updated
```

Everything else is either infrastructure support or a later phase.

---

# 1. The biggest technical problem

The current plan is internally well thought out, but it is **over-scoped at the integration boundary**.

The risky part is not Vue. The risky part is not even the LLM. The risky part is this chain:

```text
Evolution webhook payload
→ identity normalization
→ idempotent DB writes
→ realtime UI update
→ AI draft job
→ outbound send
→ status reconciliation
```

Until this chain is stable, building admin pages, media catalogs, full sync, publish/rollback, multi-account assignment, contact pages, and auto-response settings is premature.

If I implement this, I want the first milestone to answer one question:

> Can we reliably receive a real WhatsApp message, generate a useful draft, approve it, send it, and see the final status?

If the answer is no, the rest of the platform does not matter.

---

# 2. What to keep for implementation now

## 2.1 Backend architecture — keep

Keep the Go backend + Postgres architecture.

Reasons:

- Webhook processing needs strong idempotency and transaction control.
- Postgres is enough for product state, queue, drafts, events, sync state, and AI config.
- A separate queue technology is not needed yet.
- The backend must own product state; Evolution must remain transport only.

### Implement now

```text
backend/
  cmd/xchats/
  internal/config/
  internal/httpapi/
  internal/webhook/
  internal/evolution/
  internal/normalize/
  internal/store/
  internal/jobs/
  internal/realtime/
  internal/assistant/
  migrations/
```

### Do not implement now

- separate worker service;
- Kafka/NATS/Redis queue;
- plugin-style channel abstraction;
- multi-tenant architecture beyond one default org;
- complex permissions/RBAC.

Workers can run inside the same Go process. Split later only when there is load.

---

## 2.2 Postgres job queue — keep, but keep it boring

Use a simple durable Postgres job table from day one.

Do **not** use only an in-memory Go channel for webhook processing. Webhooks and outbound sends must survive restarts.

### Required job types for v1

```text
process_evolution_event
send_outbound_message
generate_ai_draft
```

### Defer job types

```text
sync_history
reconcile_recent_messages
download_media
transcribe_audio
summarize_conversation
```

### Minimum job fields

```text
job_id
job_type
payload jsonb
status: queued|running|succeeded|failed
attempt_count
max_attempts
next_attempt_at
last_error
created_at
updated_at
```

No fancy scheduler. No distributed locks beyond `FOR UPDATE SKIP LOCKED`.

---

## 2.3 Webhook receiver — absolutely keep

This is core product logic. Do not outsource it. Do not let the UI talk to Evolution. Do not read Evolution tables directly.

### Must implement

```text
POST /evolution/api/v1/webhook/{account_id}
```

Flow:

```text
1. verify shared webhook token
2. store raw event
3. dedupe raw event if possible
4. enqueue process_evolution_event
5. return 200 quickly
```

The webhook must not:

- call the LLM;
- download media;
- send SSE directly from the request path;
- perform heavy sync;
- block on Evolution calls.

---

## 2.4 Normalization — keep, this is non-negotiable

Normalization is the technical heart of the system.

### Must support in v1

- inbound text message;
- outbound message from our UI;
- outbound message sent from phone / WhatsApp Web (`fromMe`, but not from xchats);
- delivery/read status events;
- contact identity resolution;
- `@lid` vs phone JID mapping where available;
- duplicate webhook replay.

### Message identity rule

Primary key for dedupe:

```text
(account_id, evolution_message_id)
```

Fallback fingerprint only if Evolution message ID is missing:

```text
account_id + remote_jid + timestamp + direction + content_hash
```

But fallback should be treated as dangerous. Do not overuse it.

---

## 2.5 Database schema — keep, but cut it down for v1

The existing schema direction is good, but implementation should start smaller.

### Required tables now

```text
organizations
members
organization_members
sessions
whatsapp_accounts
contacts
contact_identities
conversations
messages
evolution_events
jobs
ai_drafts
ai_snapshots
ai_topics
```

### Optional now, but can be empty/stubbed

```text
message_media
ai_assets
ai_prices
```

### Defer

```text
sync_jobs
ai_audit_log
conversation_assignments
message_revisions
conversation_participants
per-member unread tables
```

### Important change

Do not implement all columns just because they appear in the docs. Implement only columns used by the first flow. Unused columns become fake complexity.

For example, do not add detailed sync progress fields if there is no real sync implementation yet.

---

# 3. What to put aside

## 3.1 Full WhatsApp Accounts Manager — put aside

The docs describe listing all Evolution instances, assigning/unassigning them, creating new accounts, QR flow, old instances, and sync progress.

That is too much for v1.

### Implement instead

One of these:

### Option A — fastest

Configured account:

```yaml
whatsapp:
  account_id: "..."
  evolution_instance_name: "sales"
```

The backend bootstraps webhook and assumes the instance exists.

### Option B — still acceptable

Minimal single-account QR page:

```text
Connect account → show QR → connected
```

No list of all instances. No assign/unassign. No multi-account UI.

### Defer

- all-instance manager;
- assign/unassign existing Evolution instances;
- multiple WhatsApp accounts per org;
- per-account settings;
- account-level auto-response settings;
- sync progress UI.

Reason: account management is not the core AI suggestion loop.

---

## 3.2 Full old history sync — put aside

Full history sync is a large subsystem.

It requires:

- pagination;
- ordering;
- dedupe with live messages;
- partial history flags;
- gap markers;
- media metadata;
- retry behavior;
- performance limits;
- user-facing sync health.

### Implement now

Live webhook only.

Optional minimal targeted fetch:

```text
When a new conversation appears, fetch last 10-20 messages for that chat if Evolution makes it easy.
```

If this is not reliable, skip it.

### Required AI behavior

Always pass context state:

```text
history_state = live_only | unknown
```

The assistant must not pretend it knows older history.

### Defer

- full initial sync;
- historical backfill;
- reconcile scheduler;
- gap UI;
- manual resync;
- sync_jobs dashboard.

Reason: the project dies if we spend the first month building sync instead of the AI draft loop.

---

## 3.3 Media download and processing — put aside

Media will explode the scope.

It adds:

- base64 download;
- MIME validation;
- storage abstraction;
- thumbnails;
- large file limits;
- failed download retry;
- audio transcription;
- document extraction;
- image interpretation;
- outbound media send;
- UI cards for each file type.

### Implement now

For inbound media:

```text
message row saved
message_kind=image/audio/video/document
body=caption if any
media_state=not_supported|pending
UI shows "Media message received" placeholder
```

### Defer

- actual file download;
- thumbnail generation;
- transcription;
- document extraction;
- AI vision;
- outbound media;
- suggested media.

Reason: text suggestions prove 80% of the first product value.

---

## 3.4 Suggested media — put aside

The current assistant supports `asset_refs`. Good. Keep the field. Do not ship the feature yet.

### Implement now

- model may return `asset_refs`;
- backend resolves nothing or logs ignored refs;
- UI does not send assets.

### Defer

- media catalog UI;
- upload asset UI;
- asset picker;
- auto-attach assets;
- sending generated/suggested media.

Reason: sending the wrong file is a real operational mistake. Text can be edited quickly; media mistakes are less obvious.

---

## 3.5 Full AI Assistant Admin UI — put aside

Persona/topics/assets/prices/publish/rollback/playground is a CMS. It is not a small feature.

### Implement now

Seed one active KB snapshot from files or config:

```text
assistant/persona.md
assistant/topics/*.md
assistant/prices.yaml optional
```

Load this into Postgres on startup or via a CLI command:

```text
xchats assistant import ./assistant
```

### Minimal admin API if needed

```text
GET /xchats/api/v1/assistant/snapshot
POST /xchats/api/v1/assistant/reload
```

No UI first.

### Defer

- topic editor;
- media asset editor;
- price editor;
- publish workflow;
- rollback UI;
- audit log;
- playground UI;
- prompt debugging UI.

Reason: we can edit markdown faster than we can build a reliable KB CMS.

---

## 3.6 Auto-response — remove from v1

The project idea is suggestion, not auto-response. Auto-response mode introduces risk and product ambiguity.

### Change

Remove `ALWAYS` and `CONFIGURE_TIME` behavior from v1 implementation.

Keep only:

```text
respond_mode = NEVER
```

Or rename it to:

```text
ai_mode = suggest_only
```

### Defer

- auto-send;
- time windows;
- per-account auto-response;
- per-conversation auto-response;
- confidence-based auto-response;
- after-hours auto-response.

Reason: auto-send changes the safety model, testing scope, UI copy, logs, and customer trust.

---

## 3.7 Contact page / Settings / Interaction history — put aside

These are secondary screens.

### Implement now

Only the Chatboard.

Required UI:

```text
left: conversation list
center: chat messages + composer
right: AI draft panel
```

### Defer

- Contacts page;
- Settings page;
- detailed profile page;
- interaction history;
- custom fields editor;
- reports;
- macros;
- SLA;
- campaigns.

Reason: every extra page creates API, state, empty states, permissions, and testing overhead.

---

## 3.8 Assignments and multi-member workflow — simplify hard

The docs include assignments. Keep the column if it is already in schema, but do not build workflows around it.

### Implement now

- one organization;
- multiple logins allowed;
- no strict assignment enforcement;
- optional “assigned_to” display only if easy.

### Defer

- assignment required before reply;
- assignment history;
- collision handling;
- agent presence;
- per-member unread counts;
- roles and permissions.

Reason: team workflow becomes complex quickly and is not required to validate AI drafts.

---

## 3.9 Multi-provider LLM — put aside

The docs support OpenRouter/OpenAI/Gemini via OpenAI-compatible API. Good abstraction, but do not overbuild routing.

### Implement now

One provider configured by env:

```text
LLM_BASE_URL
LLM_API_KEY
LLM_MODEL
```

The interface should exist, but no provider dashboard.

### Defer

- fast vs thinking model routing;
- provider fallback;
- per-org provider config;
- model comparison;
- cost dashboard.

Reason: one reliable provider is enough to debug prompt quality.

---

# 4. What I would change in the plan

## 4.1 Rename the first milestone

Current shape:

```text
Phase 1 Foundation
Phase 2 Transport
Phase 3 UI
Phase 4 AI Assistant
```

Problem: AI comes too late and Phase 4 is too huge.

### Replace with implementation milestones

```text
Milestone 1 — Local runnable skeleton
Milestone 2 — Live message ingestion
Milestone 3 — Minimal inbox + send
Milestone 4 — AI draft loop
Milestone 5 — Reliability hardening
Milestone 6 — Optional account connect UI
Milestone 7 — Optional KB admin
Milestone 8 — Optional media/sync expansion
```

The AI draft loop should arrive earlier, even if the UI is ugly.

---

## 4.2 Change “Phase 4 AI Assistant” into a smaller gate

Current Phase 4 requires:

- port brain;
- Postgres AI config;
- admin UI;
- publish/rollback;
- playground;
- inbound draft job;
- UI approve/send;
- pricing token injection;
- media refs;
- context flags;
- auto-send gates.

This is too much.

### New AI v1 gate

Done when:

```text
1. inbound text fixture creates xchats.messages row
2. generate_ai_draft job runs
3. assistant reads last N messages + contact attributes + one active KB snapshot
4. assistant writes xchats.ai_drafts
5. SSE emits ai_draft.created
6. UI shows draft
7. user edits and sends draft
8. outbound message uses normal send pipeline
9. status update reaches UI
```

Everything else is v2.

---

## 4.3 Change assistant storage

Do not begin with full versioned snapshots and admin editor.

### Better v1

Use file-backed source + Postgres active snapshot:

```text
assistant/
  persona.md
  guardrails.md
  topics/
    pricing.md
    product.md
    onboarding.md
```

Import into:

```text
ai_snapshots
ai_topics
```

Keep schema compatible with future publish/rollback, but do not build the UI yet.

---

## 4.4 Change UI scope

Current UI scope is too wide.

### Build only this

```text
/login
/chatboard
```

Chatboard must include:

- conversation list;
- message timeline;
- composer;
- AI draft card;
- edit draft;
- approve/send;
- regenerate draft;
- escalation label if needed.

No separate AI Assistant page. No Contacts page. No Settings page. No WhatsApp Accounts page unless needed for connection.

---

## 4.5 Change media behavior

The docs treat media as first-class. I would not.

### v1 rule

```text
Text is first-class.
Media is visible metadata only.
AI does not reason over media unless there is text/caption/transcript.
```

This should be explicit in the docs.

---

## 4.6 Change auto-response language

The docs mention suggest-and-approve, but also carry auto-response mode.

This creates conceptual and technical noise.

### v1 rule

```text
No auto-send exists in code paths.
```

Not just disabled. Not hidden. Not half-built.

Add it later only after draft quality is measured.

---

# 5. What is technically wrong or dangerous

## 5.1 Too many “correct” abstractions too early

Ports, swappable queues, blob stores, provider-neutral LLM, channel abstraction, future omnichannel support — all are reasonable. But if implemented too early, they slow us down.

Rule:

```text
Create interfaces where external systems touch us.
Do not create abstractions for imaginary second implementations.
```

Required interfaces now:

```text
EvolutionClient
LLMDrafter
JobQueue
RealtimePublisher
BlobStore only if media download is implemented
```

Not required now:

```text
ChannelProvider
MultiProviderRouter
AssetDeliveryEngine
WorkflowEngine
PermissionEngine
SyncPlanner
```

---

## 5.2 The docs understate Evolution uncertainty

Evolution/Baileys payloads can be inconsistent. JIDs, `@lid`, phone identity, fromMe, source, status ordering, and old sync can behave unexpectedly.

Implementation must assume:

- duplicate events happen;
- status arrives before message;
- live event arrives during sync;
- outgoing message may appear twice;
- phone/Web sends messages outside our UI;
- media download fails;
- account disconnects silently;
- history is partial.

Therefore, the first technical priority is not UI polish. It is idempotent normalization.

---

## 5.3 AI depends on data quality

If conversation history is wrong, the AI draft will be wrong.

Do not tune prompts before the message store is trustworthy.

Required assistant inputs:

```text
last 15 messages
contact attributes
conversation stage
history_state
current inbound message
active KB snapshot
```

Do not add summaries yet. Summaries can hide data quality bugs.

---

## 5.4 Full sync can bury the team

Full old sync sounds necessary, but it can become the whole project.

My rule:

```text
Live correctness first. Historical completeness second.
```

If live inbound/outbound/status works, the product can be tested. If full sync works but live send/status is buggy, the product is useless.

---

## 5.5 Admin UI before stable behavior is waste

A KB admin UI is expensive because every field needs validation, versioning, preview, errors, empty states, and rollback.

Use markdown first.

When the assistant draft quality is proven, then build the admin UI around real usage patterns.

---

# 6. Revised implementation plan

## Milestone 1 — Runnable skeleton

Goal: local app boots.

Implement:

- monorepo;
- Go backend;
- Vue frontend;
- migrations;
- config loading;
- sessions;
- one default org;
- health checks;
- docker compose;
- `make up`;
- `make test`.

Done when:

```text
User can log in and see empty chatboard.
```

---

## Milestone 2 — Webhook ingestion and normalization

Goal: captured Evolution message becomes DB rows.

Implement:

- webhook endpoint;
- raw event table;
- jobs table;
- worker loop;
- normalizer;
- contact/conversation/message upsert;
- dedupe tests;
- status update tests.

Done when:

```text
Replay captured messages.upsert twice → one message row.
Replay status events out of order → delivery state never downgrades.
```

---

## Milestone 3 — Minimal inbox and outbound send

Goal: user can read and reply.

Implement:

- conversations API;
- messages API;
- send text API;
- Evolution sendText client;
- outbound job;
- SSE for message.created/message.updated;
- chatboard UI.

Done when:

```text
Inbound fixture appears in UI.
User sends text.
Fake Evolution receives send to phone JID, not @lid.
Status updates appear in UI.
```

---

## Milestone 4 — AI draft loop

Goal: AI suggests one draft.

Implement:

- port assistant core;
- Postgres reader for last N messages + profile;
- file/imported KB snapshot;
- generate_ai_draft job;
- ai_drafts table;
- SSE ai_draft.created;
- UI draft card;
- edit/approve/send draft.

Done when:

```text
Inbound fixture → AI draft → approve → outbound send → status update.
```

---

## Milestone 5 — Reliability hardening

Goal: stop losing messages.

Implement:

- retry logic;
- failed jobs view/log endpoint;
- idempotency improvements;
- account disconnected handling;
- basic rate limiting;
- structured logs;
- manual real Evolution smoke.

Done when:

```text
The system survives duplicate webhooks, worker restart, LLM failure, Evolution send failure, and account disconnect without corrupting data.
```

---

## Milestone 6 — Only then expand

Choose one direction based on real pain:

- account connect UI;
- KB admin;
- media support;
- old sync;
- assignments;
- contact profile improvements;
- auto-response.

Do not build all at once.

---

# 7. Concrete cut list

## Build now

```text
Go backend
Postgres schema/migrations
Webhook receiver
Raw event storage
Postgres jobs
Evolution client: webhook/set, sendText, maybe connect status
Normalizer for text/status/fromMe
Contacts/conversations/messages
SSE
Minimal Vue chatboard
AI core port
ai_drafts
Markdown/config KB import
Fake Evolution
Fake LLM
End-to-end test
```

## Build later

```text
Full Evolution manager UI
Multi-account org support
Assign/unassign existing instances
Full old history sync
Reconcile scheduler
Media download pipeline
Thumbnails/transcription/document extraction
Suggested media
Media catalog UI
AI Assistant admin UI
Publish/rollback UI
Playground UI
Pricing editor
Settings page
Contacts page
Interaction history
Assignments workflow
Per-member unread counts
Auto-send
Multi-provider routing
Omnichannel abstraction
Reports/billing/macros/SLA
```

---

# 8. Specific document changes I would make

## 8.1 `0-overview.md`

Change the final point from broad platform completion to the first vertical slice.

Current direction says the user can connect WhatsApp, receive/send text + media, see statuses, and get AI replies.

I would rewrite v1 as:

```text
From the UI a member can receive a WhatsApp text message, see it in the inbox, get one AI suggested text reply, edit/approve it, send it, and see delivery/read status.
```

Remove media from v1 final bar.

---

## 8.2 `0.1-definition-of-done.md`

Split Phase 4.

Current Phase 4 is too large. Break it into:

```text
Phase 4A — AI draft loop
Phase 4B — KB admin and playground
Phase 4C — media suggestions and prices
Phase 4D — optional auto-send
```

Only Phase 4A should be part of first delivery.

---

## 8.3 `2-architecture.md`

Clarify that “swappable” does not mean “implemented now.”

Add:

```text
Interfaces exist only at external boundaries in v1. Additional adapters are not implemented until required.
```

Also remove or demote Kafka/NATS/Kafka-like future language from v1.

---

## 8.4 `3-sync.md`

Move full sync/reconcile to later phase.

Keep the design, but explicitly mark:

```text
v1 = live sync only
v2 = targeted recent sync
v3 = full initial sync + reconcile
```

---

## 8.5 `5-ui-pages.md`

Rewrite v1 pages:

```text
V1 pages:
- Login
- Chatboard

Deferred:
- Contacts
- WhatsApp Accounts
- AI Assistant
- Settings
```

Chatboard right panel should be only:

```text
AI Draft
Contact mini-profile
```

Not auto-responder, full profile, interaction history, settings, etc.

---

## 8.6 AI docs

Keep the brain design, but define “v1 adapter mode”:

```text
- text-only drafts
- one active snapshot
- no media refs rendered
- no admin UI
- no playground UI
- no auto-send
```

---

# 9. Implementation rules I would enforce

## Rule 1 — no AI in webhook request path

AI runs only from a job after message commit.

## Rule 2 — every webhook can be replayed

Raw event storage is mandatory.

## Rule 3 — every processor is idempotent

Reprocessing must not duplicate contacts, conversations, messages, or drafts.

## Rule 4 — outbound send starts as local queued message

Never send first and store later.

## Rule 5 — one send pipeline

Human send and AI-approved send use the same pipeline.

## Rule 6 — no auto-send in v1

Not hidden. Not disabled. Not implemented.

## Rule 7 — text before media

Media is visible but not a core AI input/output yet.

## Rule 8 — markdown KB before admin CMS

Build the CMS only after real operators use the draft flow.

## Rule 9 — fake Evolution is required before real Evolution confidence

If a case cannot be replayed with fixtures, it is not tested.

## Rule 10 — every async failure is visible

Failed jobs must be queryable from logs/API, even if there is no admin UI.

---

# 10. Final technical decision

If I am implementing this, I would **not** start with “team inbox with AI assistant.”

I would start with:

```text
AI-assisted WhatsApp reply loop
```

The first version is successful when:

```text
1. A real WhatsApp customer writes.
2. The message appears in our UI.
3. The AI suggests a grounded reply.
4. The operator edits/approves it.
5. The reply is sent through WhatsApp.
6. Delivery/read status returns.
7. Duplicate webhooks and retries do not corrupt data.
```

That is the real technical product.

Everything else is later.

