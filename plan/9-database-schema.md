# Database Schema (PostgreSQL)

The authoritative data model. **All state is PostgreSQL.** Design reference, **not** a migration —
no SQL files yet.

## Conventions

- All xchats tables live in a dedicated **schema `xchats`** (Evolution keeps its own separate
  schema in the same Postgres). Tables are referenced fully-qualified, e.g. `xchats.organizations`.
- **Primary key is always `id`** (`uuid`, except `sessions.id` which is `text`). It is a **random**
  uuid **except `wa_accounts.id`**, which is a **deterministic** `uuidv5(owner_jid)` so a WhatsApp
  number keeps the same id across Evolution-instance churn (see `wa_accounts`). Foreign keys are
  descriptive — `<entity>_id` referencing `<table>.id` (e.g. `account_id` → `wa_accounts.id`,
  `chat_id` → `wa_chats.id`, `user_id` → `users.id`, `organization_id` → `organizations.id`).
- **WhatsApp-channel tables are prefixed `wa_`** (`wa_accounts`, `wa_contacts`, `wa_chats`,
  `wa_messages`). Each channel owns its own contacts/chats; a future channel adds its own `ig_*` /
  `tg_*` tables. Shared tables (`organizations`, `users`, `organization_users`, `sessions`) and AI
  tables (`ai_*`) are unprefixed.
- Keys are `uuid` unless noted; flexible documents are `jsonb`.
- **Time is always UTC.** Every timestamp is `timestamptz`, which Postgres stores as a **UTC instant**
  — it does **not** store a timezone (the type name is misleading). We read/write UTC everywhere; a
  user's local time is computed **only at the UI edge**. No column stores a timezone or a local-time
  instant.
- Enum-like columns are `text` with the allowed values listed.
- **3NF by default**: a child table does **not** repeat an ancestor's `organization_id` when it is
  reachable by foreign keys (it is derived by join). Deliberate, documented denormalizations are
  listed at the end.
- **Async work rides an in-memory queue behind a `Queue` port** (Go channels in v1; swappable to
  Redis/Kafka via `QUEUE_DRIVER`) — **not** a database table. The webhook only enqueues and returns
  200; workers consume and upsert. See `3-sync.md` → "Queue abstraction".
- **v1 scope marker:** this is the full data model; **v1 only populates a subset.**
  - **Removed** (not in v1): `evolution_events` and `jobs` (async work uses the in-memory queue, not
    DB tables); `wa_qr_sessions` (connect/QR deferred); `sync_jobs` (live-only).
  - **Build 0 delta — `message_media` is back** (the first build ships media; see `TODO.md`). DDL below; it
    carries its own dedup key `UNIQUE(message_id)` so the doubled webhook delivery can't write it twice. (The
    plan's original text-only v1 removed it; Build 0 un-defers it.)
  - **Defined but empty/unused** in v1: `ai_audit_log`, `assignment_events`. (The seeded KB —
    `ai_topics` / `ai_assets` / `ai_prices` — and `ai_draft_assets` **are** used: the brain suggests
    1–3 options, each with optional media drawn from the seeded asset catalog.) Don't pour
    migration/wiring effort into the empty ones until their phase (see `0.1-definition-of-done.md`).

---

## Identity & organization

### xchats.organizations
```
id                     uuid  PK
name                   text
respond_mode           text  -- 'NEVER' | 'CONFIGURE_TIME' | 'ALWAYS'
respond_window_start   time  -- UTC time-of-day; used when respond_mode = CONFIGURE_TIME
respond_window_end     time  -- UTC time-of-day
created_at, updated_at  timestamptz
```
> The auto-response window is stored in **UTC** (no timezone column) — business hours are converted to
> UTC at config time. A DST-observing org would re-set it at the switch; the pilot org (Asia/Almaty)
> has no DST, so it's lossless. Auto-response is deferred (`respond_mode` defaults `NEVER`).

### xchats.users  (people who log in)
```
id             uuid  PK
email          citext  UNIQUE
password_hash  text
display_name   text
created_at, updated_at
```

### xchats.organization_users  (junction)
```
organization_id  uuid  FK -> organizations
user_id          uuid  FK -> users
joined_at        timestamptz
PK (organization_id, user_id)
```

### xchats.sessions
```
id           text  PK
user_id      uuid  FK -> users
created_at, expires_at  timestamptz
INDEX (user_id)
```

## WhatsApp transport

### xchats.wa_accounts  (one WhatsApp number; mirrors an Evolution instance)
```
id                       uuid  PK   -- DERIVED: uuidv5(XCHATS_WA_NS, owner_jid) — NOT random
organization_id          uuid  FK -> organizations  NULL   -- NULL = unassigned (not handled)
display_name             text
owner_jid                text  UNIQUE  -- the account's own JID; the stable identity (id is derived from it)
phone_number             text
evolution_instance_name  text         -- ephemeral label (the instance can be deleted/recreated)
evolution_instance_id    text         -- ephemeral
connection_state         text  -- 'connecting'|'qr_required'|'connected'|'disconnected'|'error'
last_live_event_at       timestamptz
created_at, updated_at
```
> **Identity = the WhatsApp number, not the Evolution instance.** `id = uuidv5(owner_jid)` (canonical
> phone-JID form, lowercased/trimmed; fixed `XCHATS_WA_NS` namespace), so the **same number always
> yields the same `id`** — deleting an instance and creating a new one for that number lands on the
> **same `account_id`**, and every `wa_chats` / `wa_messages` / `wa_contacts` row (FK `account_id`)
> stays attached: no lookup, no merge, no orphaning. The row is created/finalized at connect
> (`CONNECTION_UPDATE`, when `owner_jid` is known) via an idempotent `INSERT … ON CONFLICT (id) DO
> UPDATE` — a re-add just refreshes the instance fields. Children FK with `ON DELETE RESTRICT` so they
> can never be silently orphaned.
> "Assigned" is **derived** (`organization_id IS NOT NULL`) — no stored flag. Shared Evolution
> credentials live in `.env`. Live-only: no sync/history columns.

## WhatsApp identifiers (JID / LID)

WhatsApp inherits XMPP addressing — every party is a **JID** (`user@server`):

- **phone JID** — `<phone>@s.whatsapp.net` (e.g. `77058686509@s.whatsapp.net`). The address derived
  from the phone number. This is our `phone_jid` / `remote_jid`.
- **group JID** — `<id>@g.us` (groups; dropped in v1).
- **LID** — `<n>@lid` (e.g. `5231387607239@lid`) = **L**inked **ID**: an opaque, privacy-preserving
  identifier WhatsApp assigns so a contact can be addressed **without exposing their phone number**
  (its newer privacy model, communities/groups). The number inside a `@lid` is **not** a phone number
  and **not** derivable from one.

The same person can appear as **both**: message events carry the phone JID (+ `remoteJidAlt` = phone),
while status / `chats.*` / `contacts.*` events carry the `@lid` (verified in `captures/`).

**Why `wa_contacts` keys on `phone_jid`, not `lid_jid`:** the phone JID is on every inbound *message*
event (the events that build chats), it is human-meaningful, and it is exactly what `sendText` needs to
reply. The `@lid` only appears on cosmetic events and isn't always present. So the key is
`UNIQUE (account_id, phone_jid)`, with `lid_jid` kept as a secondary alias + `INDEX (account_id,
lid_jid)` to resolve an inbound `@lid` back to the contact. A `@lid` *could* be unique (it is stable +
unique per account), but it would key the contact on a privacy id we rarely receive and cannot reply
to — so `phone_jid` is the better natural key.

## Contacts & inbox

### xchats.wa_contacts  (one row per phone, per WhatsApp account)
```
id             uuid  PK
account_id     uuid  FK -> wa_accounts          -- organization derived via account (3NF)
phone_number   text
phone_jid      text   -- '<phone>@s.whatsapp.net' (stable identity; from remoteJidAlt)
lid_jid        text   -- '<n>@lid' alias, if the contact also appears as @lid
push_name      text
display_name   text
avatar_url     text
attributes     jsonb  -- open profile keyset, e.g. {"business_type":"retail","monthly_volume_tenge":4000000,"city":"Almaty","lang":"ru"}
created_at, updated_at
UNIQUE (account_id, phone_jid)
INDEX  (account_id, lid_jid)                    -- resolve an inbound @lid back to the contact
```
> Replaces the old org-level `contacts` + `contact_identities`. **Each channel owns its contacts** —
> no cross-account/cross-channel unification. The `@lid`↔phone mapping collapses into this one row
> (phone is the key; `lid_jid` is stored alongside when seen).

### xchats.wa_chats  (one account + one contact/remote jid)
```
id                uuid  PK
account_id        uuid  FK -> wa_accounts     -- organization derived via account (3NF)
contact_id        uuid  FK -> wa_contacts
remote_jid        text  -- canonical chat key (phone JID; from remoteJidAlt)
chat_state        text  -- 'open'|'pending'|'resolved'
assignee_user_id  uuid  FK -> users  NULL
stage             text  -- brain conversation stage
ai_summary        text  -- reserved (future rolling summary)
last_message_at   timestamptz          -- (cached aggregate — see denormalizations)
last_message_preview text               -- (cached aggregate)
unread_count      int                   -- (cached aggregate)
created_at, updated_at
UNIQUE (account_id, remote_jid)
```

### xchats.wa_messages
```
id                   uuid  PK
account_id           uuid  FK -> wa_accounts  -- kept for the dedup unique (see denormalizations)
chat_id              uuid  FK -> wa_chats
direction            text  -- 'in'|'out'
sender_kind          text  -- 'contact'|'user'|'ai'|'external_account'
sender_user_id       uuid  FK -> users  NULL
evolution_message_id text  -- Evolution key.id (dedupe natural key; status correlates here too)
participant_jid      text  -- group sender (groups deferred v1); contact/remote derived via chat
message_kind         text  -- 'conversation'|'extendedTextMessage'|'imageMessage'|...
body                 text  -- text or media caption
delivery_state       text  -- 'queued'|'sent'|'delivered'|'read'|'failed'
source               text  -- 'live_webhook'|'app'
raw                  jsonb -- the original event document (audit/replay)
message_ts           timestamptz
created_at, updated_at
UNIQUE (account_id, evolution_message_id)
INDEX  (chat_id, message_ts)
```
> **Status correlation:** apply delivery/read by matching `messages.update.data.keyId` →
> `wa_messages.evolution_message_id` — verified equal in the captures (v2.3.7), so no separate
> correlation column is needed. See `captures/README.md` finding 4.

### xchats.message_media  (Build 0: media is shipped — one row per media message)
```
id              uuid  PK
message_id      uuid  FK -> wa_messages   UNIQUE   -- dedup: doubled webhook delivery can't insert twice
media_type      text  -- 'image'|'video'|'audio'|'document'|'sticker'
mimetype        text
file_name       text
file_size       int
storage_url     text  -- blob path; served via GET /xchats/api/v1/media/{id}
download_status text  -- 'pending'|'ready'|'failed'
created_at, updated_at  timestamptz
UNIQUE (message_id)
```
> `Message.media` is exposed to the UI as a **list of URLs** (one per media row) pointing at
> `GET /media/{id}`. The ingest worker writes the blob + this row **only when the `wa_messages` upsert was a
> genuine INSERT** (idempotent against the doubled delivery). `duration_ms`/`thumbnail_url` from the API
> `MessageMedia` shape are not stored in Build 0 (derive/empty).

> **`wa_messages.raw` example** (the original Evolution event; full shapes in `captures/`):
> ```json
> { "event": "messages.upsert", "instance": "sales",
>   "sender": "77011111111@s.whatsapp.net",
>   "data": { "key": { "remoteJid": "77058686509@s.whatsapp.net",
>                      "remoteJidAlt": "77058686509@s.whatsapp.net",
>                      "fromMe": false, "id": "3A1FE00DBC50780B05E2" },
>             "pushName": "Ербол", "message": { "conversation": "Сколько стоит?" },
>             "messageType": "conversation", "messageTimestamp": 1781459144, "source": "ios" } }
> ```
> **Owner vs customer:** the **top-level `sender`** is the *account's own* JID (`owner_jid` → `wa_accounts.id =
> uuidv5(XCHATS_WA_NS, owner_jid)`). `data.key.remoteJid`/`remoteJidAlt` is the **customer** (the contact/chat
> key). Do not confuse them — using `remoteJid` as `owner_jid` yields a different account id per customer.

### xchats.assignment_events  (history; current assignee lives on wa_chats)
```
id                uuid PK
chat_id           uuid FK -> wa_chats
previous_user_id  uuid FK -> users  NULL
next_user_id      uuid FK -> users  NULL
actor_user_id     uuid FK -> users  NULL
created_at
```

## AI assistant (ported brain — normalized per snapshot version)

### xchats.ai_snapshots  (a versioned config the prompt is built from)
```
id               uuid  PK
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
id           uuid  PK
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
id           uuid  PK
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
id           uuid  PK
snapshot_id  uuid  FK -> ai_snapshots
token        text  -- e.g. 'price.growth'
amount_text  text
UNIQUE (snapshot_id, token)
```

### xchats.ai_drafts  (one suggested reply OPTION; a "Suggest" writes 1–3 per inbound)
```
id                 uuid  PK
chat_id            uuid  FK -> wa_chats              -- organization derived via chat (3NF)
trigger_message_id uuid  FK -> wa_messages           -- the inbound this answers; GROUPS the 1–3 options
option_ordinal     int    -- 1..3 — the option's position in the suggestion
draft_text         text   -- the option's text (prices already injected); user may edit before sending
sent_message_id    uuid  FK -> wa_messages  NULL  -- the message actually sent (final text after edits); set on the CHOSEN option. NULL until approved. Makes draft-acceptance / edit-distance computable from day one (the v1 success metric)
context_state      text   -- 'full' (v1 live-only; no partial/syncing context)
confidence         numeric
escalate           bool
escalation_reason  text
draft_state        text   -- 'suggested'|'sent'|'rejected'|'superseded'
created_at, updated_at
PARTIAL UNIQUE (chat_id, option_ordinal) WHERE draft_state='suggested'  -- the active suggestion holds ≤3 pending options (ordinals 1–3); a new Suggest supersedes the old set; approving one option sends it and supersedes its siblings (conditional UPDATE … WHERE draft_state='suggested' → 409 on conflict)
```
> A "Suggest" writes 1–3 option rows sharing `(chat_id, trigger_message_id)`. Each option's suggested
> media lives in `ai_draft_assets` (attach/detach before sending). **Responses are text + media
> only.** Approve sends the chosen option's (possibly edited) text + final media via the outbound
> pipeline. If the brain escalates, it writes **one** option with `escalate=true` + `escalation_reason`
> and empty text (the UI shows "reply manually").

### xchats.ai_draft_assets  (media attached to a draft option; attach/detach before send)
```
id             uuid PK
draft_id       uuid FK -> ai_drafts
asset_ref      text
media_kind     text
media_url      text   -- resolved at draft time (frozen)
ordinal        int
UNIQUE (draft_id, ordinal)
```

### xchats.ai_audit_log
```
id               uuid PK
organization_id  uuid FK -> organizations            -- DIRECT, kept
action           text
actor_user_id    uuid FK -> users  NULL
version          int
note             text
created_at
```

## Infrastructure

### Async queue (no table)

Async work — inbound event processing (`wa_event`) and on-demand AI drafts (`ai_draft`) — rides an
**in-memory queue behind a `Queue` port** (Go channels in v1), **not** a Postgres table. It is a
swappable adapter: set `QUEUE_DRIVER` to move to Redis/Kafka later (which add durability) without
changing producers (the webhook/API) or consumers (workers). See `3-sync.md` → "Queue abstraction".

> **Removed (replaced by the queue):** `evolution_events` and `jobs`. The webhook no longer stores
> raw events in the DB; it enqueues and returns 200, and a worker upserts (raw kept on
> `wa_messages.raw`).

## The three load-bearing constraints

```
UNIQUE (account_id, remote_jid)            on xchats.wa_chats
UNIQUE (account_id, evolution_message_id)  on xchats.wa_messages
UNIQUE (account_id, phone_jid)             on xchats.wa_contacts
```

---

## Normalization review

**1NF** — every column is atomic. The auto-response window is split into atomic
`respond_window_start/end` (UTC; no composite). Repeating lists are their own tables
(`organization_users`, `ai_topics/assets/prices`, `ai_draft_assets`, `assignment_events`) — no
array/CSV columns.

**2NF** — no partial dependencies. Junctions (`organization_users`) carry only attributes that
depend on the whole composite key (`joined_at`). Every other table has a single-column PK (`id`), so
2NF is automatic.

**3NF** — no transitive dependencies: a child table does **not** store an ancestor's
`organization_id` when it's reachable by FK. `organization_id` is stored **only** where it's a
direct fact — `organizations`, `organization_users`, `wa_accounts` (the assignment), `ai_snapshots`,
`ai_audit_log` — and **derived by join** everywhere else (`wa_contacts`, `wa_chats`, `wa_messages`,
`ai_topics/assets/prices`, `ai_drafts`). The `assigned` flag was removed (derived from
`organization_id`).

### Deliberate, documented denormalizations (3 exceptions, each justified)

1. **`wa_messages.account_id`** — transitively derivable via `chat_id`, but kept because the dedup
   key `UNIQUE (account_id, evolution_message_id)` and high-volume idempotent upserts must resolve
   without a join (and a message can be upserted by the worker before its chat row is touched).
   Controlled redundancy for the natural key.
2. **`wa_chats.last_message_at` / `last_message_preview` / `unread_count`** — **cached aggregates**
   of `wa_messages`, maintained on write, for fast inbox listing. Source of truth is `wa_messages`;
   they can be recomputed.
3. **`jsonb` document columns** (`wa_contacts.attributes`, `wa_messages.raw`) — intentional
   semi-structured/document storage (open keyset, raw audit document), not repeating groups — a
   deliberate choice, not a 1NF violation.

> Trade-off note: if you later want strict tenant isolation / row-level security, denormalizing
> `organization_id` onto every child table is a valid, common alternative — at the cost of the 3NF
> purity above. The schema is written 3NF-first; that switch is additive.
