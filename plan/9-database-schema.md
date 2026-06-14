# Database Schema (PostgreSQL)

The authoritative data model. **All state is PostgreSQL.** Design reference, **not** a migration —
no SQL files yet.

## Conventions

- All xchats tables live in a dedicated **schema `xchats`** (Evolution keeps its own separate
  schema in the same Postgres). Tables are referenced fully-qualified, e.g. `xchats.organizations`.
- **Full, descriptive table names** (`organizations`, `conversations`, `messages`, …). We use
  `members` rather than `users` because `user` is a reserved word; the schema qualifier handles the
  rest.
- Keys are `uuid` unless noted; timestamps are `timestamptz`; flexible documents are `jsonb`.
- Enum-like columns are `text` with the allowed values listed.
- **3NF by default**: a child table does **not** repeat an ancestor's `organization_id` when it is
  reachable by foreign keys (it is derived by join). Deliberate, documented denormalizations are
  listed at the end.
- **v1 scope marker:** this is the full data model; **v1 only populates a subset.**
  - **Removed** (not used in v1): `sync_jobs` (live-only — nothing to sync) and `message_media`
    (text-only).
  - **Defined but empty/unused** in v1: `wa_qr_sessions` (connect/QR deferred),
    `ai_audit_log`, `assignment_events`, `ai_draft_assets`, `ai_assets`, `ai_prices`. Don't pour
    migration/wiring effort into them until their phase (see `0.1-definition-of-done.md`).

---

## Identity & organization

### xchats.organizations
```
organization_id        uuid  PK
name                   text
respond_mode           text  -- 'NEVER' | 'CONFIGURE_TIME' | 'ALWAYS'
respond_window_start   time  -- atomic; used when respond_mode = CONFIGURE_TIME
respond_window_end     time
respond_window_tz      text
created_at, updated_at  timestamptz
```

### xchats.members  (people who log in; `user` is reserved → `members`)
```
member_id      uuid  PK
email          citext  UNIQUE
password_hash  text
display_name   text
created_at, updated_at
```

### xchats.organization_members  (junction)
```
organization_id  uuid  FK -> organizations
member_id        uuid  FK -> members
joined_at        timestamptz
PK (organization_id, member_id)
```

### xchats.sessions
```
session_id   text  PK
member_id    uuid  FK -> members
created_at, expires_at  timestamptz
INDEX (member_id)
```

## WhatsApp transport

### xchats.wa_accounts  (mirrors an Evolution instance)
```
account_id               uuid  PK
organization_id          uuid  FK -> organizations  NULL   -- NULL = unassigned (not handled)
display_name             text
evolution_instance_name  text  UNIQUE
evolution_instance_id    text
owner_jid                text
phone_number             text
connection_state         text  -- 'connecting'|'qr_required'|'connected'|'disconnected'|'error'
last_live_event_at       timestamptz
created_at, updated_at
```
> Live-only: `sync_state` / `history_state` / `last_synced_at` / `last_reconcile_at` are dropped —
> there is no sync to track.
> "Assigned" is **derived** (`organization_id IS NOT NULL`) — no stored flag. Shared Evolution
> credentials live in `.env`.

### xchats.wa_qr_sessions
```
qr_session_id  uuid  PK
account_id     uuid  FK -> wa_accounts
qr_code        text
pairing_code   text
qr_state       text  -- 'qr_required'|'connected'|'expired'
expires_at, consumed_at, created_at  timestamptz
```

## Contacts & inbox

### xchats.contacts  (org-level; a contact may reach several accounts)
```
contact_id       uuid  PK
organization_id  uuid  FK -> organizations          -- DIRECT (contact belongs to org), kept
display_name     text
phone_number     text
avatar_url       text
attributes       jsonb -- open profile keyset (business_type, monthly_volume, ...) — semi-structured by design
created_at, updated_at
```

### xchats.contact_identities  (resolves @lid / phone JID / phone / push name -> one contact)
```
identity_id      uuid  PK
contact_id       uuid  FK -> contacts               -- organization derived via contact (3NF: not stored)
account_id       uuid  FK -> wa_accounts      -- a real attribute (identities are per account)
identity_kind    text  -- 'phone'|'phone_jid'|'lid_jid'|'push_name'
identity_value   text
created_at
UNIQUE (contact_id, account_id, identity_kind, identity_value)
INDEX  (account_id, identity_value)                 -- lookup an inbound jid
```

### xchats.conversations  (one account + one contact/remote jid)
```
conversation_id   uuid  PK
account_id        uuid  FK -> wa_accounts     -- organization derived via account (3NF: not stored)
contact_id        uuid  FK -> contacts
remote_jid        text  -- canonical key (phone JID preferred; from remoteJidAlt)
lid_jid           text  -- alias if the contact also appears as @lid
conversation_state text -- 'open'|'pending'|'resolved'
assignee_member_id uuid FK -> members  NULL
stage             text  -- brain conversation stage
ai_summary        text  -- reserved (future rolling summary)
last_message_at   timestamptz          -- (cached aggregate — see denormalizations)
last_message_preview text               -- (cached aggregate)
unread_count      int                   -- (cached aggregate)
created_at, updated_at
UNIQUE (account_id, remote_jid)
```

### xchats.messages
```
message_id           uuid  PK
account_id           uuid  FK -> wa_accounts  -- kept for the dedup unique (see denormalizations)
conversation_id      uuid  FK -> conversations
direction            text  -- 'in'|'out'
sender_kind          text  -- 'contact'|'member'|'ai'|'external_account'
sender_member_id     uuid  FK -> members  NULL
evolution_message_id text  -- Evolution key.id (dedupe natural key; status correlates here too)
participant_jid      text  -- group sender (groups deferred v1); contact/remote derived via conversation
message_kind         text  -- 'conversation'|'extendedTextMessage'|'imageMessage'|...
body                 text  -- text or media caption
delivery_state       text  -- 'queued'|'sent'|'delivered'|'read'|'failed'
source               text  -- 'live_webhook'|'app'
raw                  jsonb -- the original event document (audit/replay)
message_ts           timestamptz
created_at, updated_at
UNIQUE (account_id, evolution_message_id)
INDEX  (conversation_id, message_ts)
```
> **Status correlation:** apply delivery/read by matching `messages.update.data.keyId` →
> `messages.evolution_message_id` — verified equal in the captures (v2.3.7), so no separate
> correlation column is needed. See `captures/README.md` finding 4.

> **`message_media` removed in v1** (text-only). Media bodies are ignored; the table returns 1:1
> with messages when the media phase lands.

### xchats.assignment_events  (history; current assignee lives on conversations)
```
assignment_event_id uuid PK
conversation_id     uuid FK -> conversations
previous_member_id  uuid FK -> members  NULL
next_member_id      uuid FK -> members  NULL
actor_member_id     uuid FK -> members  NULL
created_at
```

## AI assistant (ported brain — normalized per snapshot version)

### xchats.ai_snapshots  (a versioned config the prompt is built from)
```
snapshot_id      uuid  PK
organization_id  uuid  FK -> organizations          -- DIRECT, kept
version          int
snapshot_state   text  -- 'draft'|'published'
persona          text
mission          text
guardrails       text
language_policy  text
published_at, created_at  timestamptz
UNIQUE (organization_id, version)
```

### xchats.ai_topics  (KB; belongs to a snapshot version)
```
topic_id     uuid  PK
snapshot_id  uuid  FK -> ai_snapshots               -- organization derived via snapshot (3NF)
slug         text
lang         text
keywords     text
body_md      text
created_at, updated_at
UNIQUE (snapshot_id, slug)
```

### xchats.ai_assets  (media catalog; belongs to a snapshot version)
```
asset_id     uuid  PK
snapshot_id  uuid  FK -> ai_snapshots
ref          text  -- stable id the model uses in asset_refs
asset_kind   text  -- 'image'|'video'|'document'|'audio'
topic_slug   text
description  text
asset_url    text
UNIQUE (snapshot_id, ref)
```

### xchats.ai_prices  (price tokens; belongs to a snapshot version)
```
price_id     uuid  PK
snapshot_id  uuid  FK -> ai_snapshots
token        text  -- e.g. 'price.growth'
amount_text  text
UNIQUE (snapshot_id, token)
```

### xchats.ai_drafts  (one suggestion per inbound)
```
draft_id           uuid  PK
conversation_id    uuid  FK -> conversations         -- organization derived via conversation (3NF)
trigger_message_id uuid  FK -> messages
draft_text         text   -- prices already injected
sent_message_id    uuid  FK -> messages  NULL  -- the message a member actually sent (final text after edits); NULL until approved. Makes draft-acceptance / edit-distance computable from day one (the v1 success metric)
context_state      text   -- 'full' (v1 live-only; no partial/syncing context)
confidence         numeric
escalate           bool
escalation_reason  text
draft_state        text   -- 'suggested'|'approved'|'rejected'|'sent'|'superseded'
created_at, updated_at
PARTIAL UNIQUE (conversation_id) WHERE draft_state='suggested'  -- one pending draft per conversation (replaces the brain's in-process keyedMutex); approve via conditional UPDATE … WHERE draft_state='suggested' → 409 on conflict
```

### xchats.ai_draft_assets  (the media a draft suggested — normalized list)
```
draft_asset_id uuid PK
draft_id       uuid FK -> ai_drafts
asset_ref      text
media_kind     text
media_url      text   -- resolved at draft time (frozen)
ordinal        int
UNIQUE (draft_id, ordinal)
```

### xchats.ai_audit_log
```
audit_id         uuid PK
organization_id  uuid FK -> organizations            -- DIRECT, kept
action           text
actor_member_id  uuid FK -> members  NULL
version          int
note             text
created_at
```

## Infrastructure

### xchats.evolution_events  (raw webhooks; store-first, replay, idempotency)
```
event_id           uuid PK
account_id         uuid FK -> wa_accounts  NULL  -- organization derived via account (3NF)
evolution_event_kind text  -- 'messages.upsert'|'messages.update'|'send.message'|...
external_event_id  text  -- derived (key.id / keyId+status / payload hash)
payload            jsonb
process_state      text  -- 'pending'|'processed'|'failed'
processed_at, process_error, created_at
UNIQUE (account_id, evolution_event_kind, external_event_id)   -- duplicate delivery = no-op
INDEX  (process_state)
```

### xchats.jobs  (queue / message bus — swappable adapter)
```
job_id        uuid PK
job_kind      text  -- v1: 'process_event'|'ai_draft'|'outbound_send'  (media_download later)
payload       jsonb -- polymorphic job args
job_state     text  -- 'pending'|'running'|'done'|'failed'
run_after     timestamptz
attempts, max_attempts  int
last_error    text
locked_at     timestamptz
created_at, updated_at
INDEX (job_state, run_after)            -- worker poll: FOR UPDATE SKIP LOCKED
```

> **`sync_jobs` removed in v1** — live-only, nothing to sync.

## The three load-bearing constraints

```
UNIQUE (account_id, remote_jid)                                  on xchats.conversations
UNIQUE (account_id, evolution_message_id)                        on xchats.messages
UNIQUE (contact_id, account_id, identity_kind, identity_value)   on xchats.contact_identities
```

---

## Normalization review

**1NF** — every column is atomic. The auto-response window is split into atomic
`respond_window_start/end/tz` (no composite). Repeating lists are their own tables
(`organization_members`, `contact_identities`, `ai_topics/assets/prices`, `ai_draft_assets`,
`assignment_events`) — no array/CSV columns.

**2NF** — no partial dependencies. Junctions (`organization_members`) carry only attributes that
depend on the whole composite key (`joined_at`). Every other table has a single-column PK, so 2NF
is automatic.

**3NF** — no transitive dependencies: a child table does **not** store an ancestor's
`organization_id` when it's reachable by FK. `organization_id` is stored **only** where it's a
direct fact — `organizations`, `organization_members`, `wa_accounts` (the assignment),
`contacts`, `ai_snapshots`, `ai_audit_log` — and **derived by join** everywhere else
(`contact_identities`, `conversations`, `messages`, `ai_topics/assets/prices`, `ai_drafts`,
`evolution_events`). The `assigned` flag was removed (derived from `organization_id`).

### Deliberate, documented denormalizations (3 exceptions, each justified)

1. **`messages.account_id`** — transitively derivable via `conversation_id`, but kept because the
   dedup key `UNIQUE (account_id, evolution_message_id)` and high-volume idempotent upserts must
   resolve without a join (and a message can be upserted from a webhook before its conversation row
   is touched). Controlled redundancy for the natural key.
2. **`conversations.last_message_at` / `last_message_preview` / `unread_count`** — **cached
   aggregates** of `messages`, maintained on write, for fast inbox listing. Source of truth is
   `messages`; they can be recomputed.
3. **`jsonb` document columns** (`contacts.attributes`, `messages.raw`, `evolution_events.payload`,
   `jobs.payload`) — intentional semi-structured/document storage (open keyset, raw audit document,
   polymorphic args), not repeating groups — a deliberate choice, not a 1NF violation.

> Trade-off note: if you later want strict tenant isolation / row-level security, denormalizing
> `organization_id` onto every child table is a valid, common alternative — at the cost of the 3NF
> purity above. The schema is written 3NF-first; that switch is additive.
