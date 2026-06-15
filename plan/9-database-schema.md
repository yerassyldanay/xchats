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
    `ai_topics` / `ai_assets` / `ai_values` — and `ai_suggestions` **are** used: each Suggest writes one
    row whose `options` jsonb holds 1–3 variants, each with optional media from the seeded catalog.) Don't pour
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
attributes     jsonb  -- open profile keyset, e.g. {"business_type":"retail","monthly_volume_tenge":4000000,"city":"Almaty","lang":"ru"}  -- manual/CRM only in v1; the brain does NOT read it (see 8-ai-assistant.md → Memory)
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

### xchats.ai_values  (named value tokens — prices AND any other confirmed number/value; belongs to a snapshot version)
```
id           uuid  PK
snapshot_id  uuid  FK -> ai_snapshots
token        text  -- namespace.key — e.g. 'price.growth', 'limit.growth', 'time.api_setup', 'contact.whatsapp'
lang         text  -- 'ru' | 'kk' | '*'  — '*' = language-neutral (numbers, phones, addresses, e-mails)
value_text   text  -- the confirmed value, as text — '25 000 ₸/мес', 'до 2 000 платежей/мес', '5 минут' (any unit; code substitutes verbatim)
description  text  NULL  -- what this value means, for the editor/builder UX & the confirm popup; NOT injected into the prompt
UNIQUE (snapshot_id, token, lang)
```
> **Generalized from the old `ai_prices`.** This was always a **token → confirmed-value** store; the
> reason it exists is the *token discipline* — the model must never invent a factual number/fact and you
> want it centrally editable — which applies to **any** value (limits, durations, SLAs, contacts,
> addresses, min/max), not just prices. `price.*` stays the canonical namespace; new namespaces
> (`limit.*`, `time.*`, `contact.*`, `trial.*`, `min.*`/`max.*`, …) need **no schema change** — code does
> a pure string substitution of `{{namespace.key}}` → `value_text`. The publish gate's "price-safety =
> 1.0" reads as "**every token resolves**". `value_text` is text precisely so any unit fits. The
> `description` is **human-only** (editor / `confirm_price` popup / reviewer); the model never sees it.
>
> **Language-aware.** Rendering differs by language (`Бесплатно`/`Тегін`, `…/мес`/`…/ай`), so values are
> keyed by `lang` too: injection resolves `(token, reply_language)` first, then falls back to the **`'*'`
> language-neutral** row (phone numbers, addresses, e-mails, bare counts that read the same either way);
> if neither exists the token is unresolved → `PricingError`/escalate (never ship a half-rendered value).
> Mirrors `ai_topics.lang` and the brain's "reply in the customer's language" rule.

> **Worked example — the xPayment value book** (one published snapshot; the `tariffs` topic body uses
> `{{price.growth}}`, `{{limit.growth}}`, `{{time.api_setup}}`, `{{contact.whatsapp}}`, …):
>
> | token | lang | value_text | description |
> |---|---|---|---|
> | `price.trial` | ru | Бесплатно | Пробный доступ — стоимость |
> | `price.trial` | kk | Тегін | то же, по-казахски |
> | `price.start` | ru | 10 000 ₸/мес | Тариф «Старт» — фикс. цена в месяц |
> | `price.growth` | ru | 25 000 ₸/мес | Тариф «Рост» (основной) — фикс. цена в месяц |
> | `price.scale` | ru | 60 000 ₸/мес | Тариф «Масштаб» — фикс. цена в месяц |
> | `trial.days` | ru | 3 дня | Длительность бесплатного пробного периода |
> | `limit.growth` | ru | до 2 000 платежей/мес | Лимит платежей/мес на «Рост» |
> | `limit.scale` | ru | безлимит платежей | Лимит платежей на «Масштаб» |
> | `limit.cashiers` | ru | до 5 виртуальных касс | Макс. число виртуальных касс (все тарифы) |
> | `time.api_setup` | ru | 5 минут | Время на подключение Kaspi Pay API |
> | `time.callback` | ru | 1 час | В течение какого времени перезвонят |
> | `contact.whatsapp` | * | +7 702 976-65-09 | WhatsApp поддержки (язык-нейтрально) |
> | `contact.email` | * | support@xpayment.kz | E-mail поддержки |
> | `contact.address` | * | г. Шымкент, ул. Аргынбеков, 29/4 | Юридический адрес |
> | `contact.legal` | * | ИП «XGroup», Республика Казахстан | Реквизиты |

### xchats.ai_suggestions  (one row per "Suggest"; the 1–3 reply OPTIONS live in the `options` jsonb)
```
id                   uuid  PK
chat_id              uuid  FK -> wa_chats            -- organization derived via chat (3NF)
trigger_message_id   uuid  FK -> wa_messages         -- the inbound this answers
requested_by_user_id uuid  FK -> users  NULL          -- provenance only (who pressed "Suggest"); NULL = system/auto-draft. NOT part of the key
state                text  -- 'generating'|'suggested'|'resolved'|'superseded'  (a failed generation → 'superseded', freeing the lock)
reply_language       text  -- 'ru'|'kk' — the turn's language; drives value injection
confidence           numeric
escalate             bool
escalation_reason    text
options              jsonb -- the 1..3 variants, each with its own nested media:
                     --   [{ "ordinal":1, "text":"…final text, values already injected…",
                     --      "assets":[{ "ref":"tariffs_overview","kind":"image","url":"/media/…","ordinal":1 }] }, …]
suggested_status     jsonb -- { "stage":"qualifying" } | null
suggested_callback   jsonb -- { "due_at":"…", "note":"…" } | null
chosen_ordinal       int   NULL  -- which option the agent approved
sent_message_id      uuid  FK -> wa_messages  NULL   -- the outbound created on approve (final text after edits)
context_state        text  -- 'full' (v1 live-only; no partial/syncing context)
created_at, updated_at  timestamptz
PARTIAL UNIQUE (chat_id) WHERE state IN ('generating','suggested')  -- the GENERATE LOCK: at most ONE in-flight-or-pending suggestion per chat (shared; not per agent)
```
> **One row per "Suggest"; options in `jsonb`.** A press runs the brain and writes **one** row whose
> `options` array holds the 1–3 variants (each its own text + nested media) — no per-option rows, no
> separate `ai_draft_assets` table. Keyed on **`chat_id` alone**: at most one active suggestion per
> chat (shared, not per agent), so the suggestion is a property of the conversation's current state, not
> of whoever clicked. A re-press (by anyone) **supersedes the chat's pending row** (one-row `UPDATE …
> SET state='superseded' WHERE chat_id=? AND state IN ('generating','suggested')`). `requested_by_user_id`
> is provenance only (NULL = system) — keeping it out of the key keeps this forward-compatible with the
> deferred **auto-draft-on-inbound** mode, which has no user.
>
> **Generate is a two-step lock (multi-agent safe).** A "Generate" click conditionally INSERTs a
> `state='generating'` row; because the partial unique covers `('generating','suggested')`, a second
> concurrent click (from the same or another agent) hits the constraint → `409`, and the realtime
> channel broadcasts **`ai_draft.generating`** so *every* viewer of the chat sees a spinner + a disabled
> button. The worker fills `options` and flips the row to `suggested` → broadcasts **`ai_draft.created`**
> (cards appear for everyone); a failed generation flips it to `superseded`, freeing the lock. The DB —
> not in-memory UI state — is the coordination point, so it holds across users and processes.
> **Approve:** set `chosen_ordinal` + `sent_message_id`, `state='resolved'`, send the chosen option's
> (possibly edited) text + media via the outbound pipeline. **Escalation** = one row, `escalate=true` +
> `escalation_reason`, empty `options` (UI shows "reply manually"). **Responses are text + media only.**
>
> The v1 success metric still computes exactly — edit-distance / acceptance = compare
> `options[chosen_ordinal].text` against the body of `sent_message_id`. Turn-level facts (language,
> escalate, confidence, status, callback) live **once** on the row, never repeated per
> option. The `options` jsonb is a deliberate document column (see *Normalization* → denormalizations).

### xchats.ai_audit_log
```
id               uuid PK
organization_id  uuid FK -> organizations            -- DIRECT, kept
action           text   -- 'publish'|'rollback'|'snapshot_created'|'edit' — a KB-lifecycle event
actor_user_id    uuid FK -> users  NULL              -- who did it (NULL = system, e.g. seed on boot)
version          int                                 -- which ai_snapshots.version the action targeted
note             text                                -- free-text detail ('published v4', 'rolled back v4→v3')
created_at
```
> **Append-only history of the KB's lifecycle** — *not* per-message or per-draft activity. One row per
> significant snapshot action (publish, rollback, snapshot created/edited): who, which version, when,
> why. It answers "who published the price that went out last Tuesday, and what did it replace?" — the
> provenance/compliance trail behind the `draft → published` gate, and the record a rollback reads. It is
> **never read on the hot path** and the brain never touches it. **v1: defined but empty** — nothing
> writes it until the publish/rollback flow exists (Phase 4B); the seed-on-boot may write one
> `snapshot_created` row. Distinct from `wa_messages` (conversation) and `ai_suggestions` (suggestions).

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
(`organization_users`, `ai_topics/assets/values`, `assignment_events`) — no array/CSV columns,
**except `ai_suggestions.options`** (a deliberate jsonb document holding the 1–3 reply variants as one
atomic suggestion — see denormalizations).

**2NF** — no partial dependencies. Junctions (`organization_users`) carry only attributes that
depend on the whole composite key (`joined_at`). Every other table has a single-column PK (`id`), so
2NF is automatic.

**3NF** — no transitive dependencies: a child table does **not** store an ancestor's
`organization_id` when it's reachable by FK. `organization_id` is stored **only** where it's a
direct fact — `organizations`, `organization_users`, `wa_accounts` (the assignment), `ai_snapshots`,
`ai_audit_log` — and **derived by join** everywhere else (`wa_contacts`, `wa_chats`, `wa_messages`,
`ai_topics/assets/values`, `ai_suggestions`). The `assigned` flag was removed (derived from
`organization_id`).

### Deliberate, documented denormalizations (3 exceptions, each justified)

1. **`wa_messages.account_id`** — transitively derivable via `chat_id`, but kept because the dedup
   key `UNIQUE (account_id, evolution_message_id)` and high-volume idempotent upserts must resolve
   without a join (and a message can be upserted by the worker before its chat row is touched).
   Controlled redundancy for the natural key.
2. **`wa_chats.last_message_at` / `last_message_preview` / `unread_count`** — **cached aggregates**
   of `wa_messages`, maintained on write, for fast inbox listing. Source of truth is `wa_messages`;
   they can be recomputed.
3. **`jsonb` document columns** (`wa_contacts.attributes`, `wa_messages.raw`, `ai_suggestions.options`
   + its `suggested_status`/`suggested_callback`) — intentional semi-structured/document
   storage (open keyset, raw audit document, the 1–3 reply variants kept as one atomic suggestion), not
   repeating groups — a deliberate choice, not a 1NF violation. The `options` array trades per-option
   queryability for one-row supersede + no join to render a suggestion; the success metric stays
   computable (chosen option text vs the sent message).

> Trade-off note: if you later want strict tenant isolation / row-level security, denormalizing
> `organization_id` onto every child table is a valid, common alternative — at the cost of the 3NF
> purity above. The schema is written 3NF-first; that switch is additive.
