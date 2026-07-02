# Database Schema (PostgreSQL)

> ⚠️ **Partially superseded by [`14-draft-staging-and-retrieval.md`](14-draft-staging-and-retrieval.md).**
> The `drafted_at` columns are replaced by separate **draft twin tables**; topic `body_md` carries **no
> fact tokens** (pure prose); v1 fills **ru rows only**. This doc is updated lazily; 14 wins on conflict.

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
  - **Build 0 delta — `message_media` is back** (the first build ships media). DDL below; it
    carries its own dedup key `UNIQUE(message_id)` so the doubled webhook delivery can't write it twice. (The
    plan's original text-only v1 removed it; Build 0 un-defers it.)
  - **Defined but empty/unused** in v1: `ai_audit_log`, `assignment_events`. (The seeded KB —
    `ai_topics` / `ai_assets` / `ai_tariffs` / `ai_products` / `ai_contacts` (the typed **fact** tables) — and `ai_suggestions` **are** used: each Suggest writes one
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

## AI assistant (ported brain — ONE living KB per org)

### xchats.ai_snapshots  (the single per-org assistant config)
```
id               uuid  PK
organization_id  uuid  FK -> organizations          -- DIRECT, kept
persona          text
mission          text
guardrails       text
language_policy  text
reply_max_words  int
created_at, updated_at  timestamptz
UNIQUE (organization_id)
```
> **One living KB per org — no versions, no snapshots, no rollback.** There is exactly **one** row
> per org holding the assistant config (persona / mission / guardrails / language_policy /
> reply_max_words). The table name `ai_snapshots` and the FK `snapshot_id` are **legacy** — kept to
> avoid churn across the schema and migrations; nothing here is versioned anymore. The old
> `version` / `snapshot_state` / `published_at` columns and `UNIQUE(organization_id, version)` are
> dropped; the key is now `UNIQUE(organization_id)` (one row per org).
>
> **Draft is a per-row timestamp.** Every content row below carries `drafted_at timestamptz NULL`:
> `drafted_at IS NULL` → **LIVE** (the brain reads it; included in the prompt); `drafted_at IS NOT
> NULL` → **PENDING** (shown in the playground with editors, **excluded** from the prompt). This
> replaces the old `review_state` enum. Each content row also carries `provenance jsonb` (source
> tracking). **Approve = `UPDATE … SET drafted_at = NULL`** (per-row or all-at-once); the
> deterministic gate runs **at approve time** — no publish/swap, no version copy, no rollback.

### xchats.ai_topics  (KB; belongs to the org's KB)
```
id           uuid  PK
snapshot_id  uuid  FK -> ai_snapshots               -- organization derived via snapshot (3NF)
slug         text
lang         text
keywords     text
body_md      text
drafted_at   timestamptz  NULL  -- NULL = LIVE (in prompt); NOT NULL = PENDING (playground only)
provenance   jsonb              -- source tracking (where this row came from)
created_at, updated_at
UNIQUE (snapshot_id, slug)
```

### xchats.ai_assets  (media catalog; attaches to ANY entity)
```
id           uuid  PK
snapshot_id  uuid  FK -> ai_snapshots
ref          text  -- stable id the model uses in asset_refs
asset_kind   text  -- 'image'|'video'|'document'|'audio'
owner_kind   text  -- 'topic'|'product'|'tariff'|''  ('' = unattached)
owner_ref    text  -- the owner's ref/slug; '' = unattached
description  text
asset_url    text
drafted_at   timestamptz  NULL  -- NULL = LIVE; NOT NULL = PENDING
provenance   jsonb
UNIQUE (snapshot_id, ref)
```
> **Polymorphic media.** A media blob attaches to **any** entity via the shared `(owner_kind,
> owner_ref)` pair: an entity's media = `ai_assets WHERE owner_kind/owner_ref match`. The old
> `topic_slug` is gone — a topic is now just `owner_kind='topic'`, `owner_ref=<slug>`.

### KB facts — the **Facts lane** (typed tables; `ai_values` is removed)

> **Two lanes (see `8.4` / `11`).** Exact facts — prices, tariffs, limits, times, hours, addresses,
> phones — are **never authored by the model**: they live in **typed tables with concrete columns** and
> code substitutes the stored value **verbatim**. Explanatory prose lives in `ai_topics` (the Knowledge
> lane) and is grounded by a judge. The old generic value bag **`ai_values` is removed** — a nearest-key
> lookup can return the *wrong* fact; a typed column cannot.
>
> **Language is a row, never a column.** Every fact table keys on `(snapshot_id, ref, lang)` (contacts on
> `(snapshot_id, lang)`): one row per `(entity, language)`, plus a `*` row for language-neutral values
> (phones, e-mails, addresses). Adding a language = **inserting rows**, never a schema change — there are
> no `name_ru`/`name_kk` columns. This is the same per-language shape `ai_topics` already uses, so the
> whole KB is uniform.
>
> **Reference model — `{{table.slug.field}}`.** A fact is quoted in a topic body or a reply only as a
> token `{{table.slug.field}}`: `table` selects the fact table, `slug` the row, `field` the column —
> e.g. `{{tariff.growth.price}}`, `{{tariff.growth.limit_text}}`, `{{product.nike_x.price}}`,
> `{{contact.support.whatsapp}}`. Code resolves the token against the typed table **for the reply's
> language** (falling back reply-language → org-default → `*`) and substitutes the stored value verbatim
> (units included; code never reformats a number). The model **never emits a digit** for a known fact —
> it emits the token. **Fail closed:** an unresolved token never ships — it becomes a holding reply /
> manual review. Values are typed **columns on the entity**; there is no separate value store to own.
>
> **Worked example — the xPayment facts** (`{{tariff.growth.price}}` etc. appear in the `tariffs` topic
> body and in replies; code fills them per reply language):
>
> | table.slug | lang | columns (stored verbatim) |
> |---|---|---|
> | `tariff.trial` | ru / kk | name «Пробный»/«Тегін», price «Бесплатно»/«Тегін», limit_text «3 дня»/«3 күн» |
> | `tariff.growth` | ru | name «Рост», price «25 000 ₸/мес», limit_text «до 2 000 платежей/мес» |
> | `tariff.growth` | kk | name «Өсу», price «25 000 ₸/ай», limit_text «айына 2 000 төлемге дейін» |
> | `tariff.scale` | ru | name «Масштаб», price «60 000 ₸/мес», limit_text «безлимит платежей» |
> | `tariff.pro` | * | fee «1.5 % за транзакцию» |
> | `contact.support` | * | whatsapp «+7 702 976-65-09», email «support@xpayment.kz», address «г. Шымкент, ул. Аргынбеков, 29/4», legal «ИП «XGroup», РК» |
> | `contact.support` | ru / kk | callback_time «1 час» / «1 сағат» |

### xchats.ai_contacts  (typed org-support facts — one singleton entity, `slug='support'`)
```
id            uuid  PK
snapshot_id   uuid  FK -> ai_snapshots
slug          text  -- singleton 'support' — keeps the 3-part token grammar {{contact.support.<field>}}
lang          text  -- 'ru' | 'kk' | '*'  — '*' = language-neutral (phones, e-mails, addresses)
whatsapp      text  -- support WhatsApp — language-neutral (lives on the '*' row)
email         text  -- support e-mail — language-neutral
address       text  -- legal / office address — language-neutral
legal         text  -- legal entity / requisites — language-neutral
callback_time text  -- how soon we call back ('1 час' / '1 сағат') — language-bearing (ru/kk rows)
drafted_at    timestamptz  NULL  -- NULL = LIVE; NOT NULL = PENDING
provenance    jsonb
UNIQUE (snapshot_id, lang)
```
> **A dedicated typed table for org-level scalars** — support contacts + callback time — not a generic
> bag. Token resolution is **per field** with language fallback: `{{contact.support.whatsapp}}` finds its
> value on the `*` row; `{{contact.support.callback_time}}` finds it on the `ru`/`kk` row. A **new**
> category of org fact (e.g. working hours) gets its **own typed table**, never a key–value row.

### xchats.ai_products  (typed fact table — one row per (product, language))
```
id           uuid  PK
snapshot_id  uuid  FK -> ai_snapshots               -- organization derived via snapshot (3NF)
ref          text  -- stable key, e.g. 'nike-x'
lang         text  -- 'ru' | 'kk' | '*'  — language is a ROW (one row per product per language)
name         text  -- verbatim, per language
price        text  -- the confirmed price, verbatim WITH units ('25 000 ₸'); code never reformats
description  text
category     text
data         jsonb -- sphere-specific descriptive attrs: {size, color, area_m2, …}
status       text  -- 'active'|'inactive'
drafted_at   timestamptz  NULL  -- NULL = LIVE; NOT NULL = PENDING
provenance   jsonb
created_at, updated_at  timestamptz
UNIQUE (snapshot_id, ref, lang)
```
> **A product's price is a typed column, quoted verbatim.** `price` (and any other exact field) is a
> concrete column stored **verbatim with units**; the model quotes it only as `{{product.nike_x.price}}`,
> resolved for the reply's language — never a digit it writes itself. **Language is a row**
> (`UNIQUE (snapshot_id, ref, lang)`); a `*` row carries language-neutral values. Media still attach
> **polymorphically** as `ai_assets` rows (`owner_kind='product'`, `owner_ref=<ref>`) — only *values*
> moved onto the entity as columns.

### xchats.ai_tariffs  (typed fact table — one row per (tariff, language))
```
id            uuid  PK
snapshot_id   uuid  FK -> ai_snapshots
ref           text  -- 'trial'|'start'|'growth'|'scale'|'pro'
lang          text  -- 'ru' | 'kk' | '*'  — language is a ROW
name          text  -- verbatim, per language ('Рост' / 'Өсу')
price         text  -- confirmed price, verbatim with units ('25 000 ₸/мес' / '25 000 ₸/ай'); '' if fee-based
limit_text    text  -- confirmed limit, verbatim ('до 2 000 платежей/мес' / 'безлимит платежей')
fee           text  -- per-transaction fee for percentage plans ('1.5 % за транзакцию'); '' otherwise
summary       text
pricing_type  text  -- 'fixed'|'percentage'|'tiered'|'hybrid'
advantages    text
disadvantages text
data          jsonb -- descriptive prose the model may paraphrase: conditions / billing_period / tiers
status        text  -- 'active'|'inactive'
drafted_at    timestamptz  NULL  -- NULL = LIVE; NOT NULL = PENDING
provenance    jsonb
created_at, updated_at  timestamptz
UNIQUE (snapshot_id, ref, lang)
```
> **A tariff's quotable numbers are typed columns, verbatim.** `price`, `limit_text`, `fee` are concrete
> columns; the assistant quotes them only as `{{tariff.growth.price}}`, `{{tariff.growth.limit_text}}`,
> `{{tariff.pro.fee}}`, resolved for the reply's language. **Language is a row**
> (`UNIQUE (snapshot_id, ref, lang)`) — `Рост`/`Өсу` and `…/мес`/`…/ай` are two rows, not two columns.
> Only *descriptive* prose the model may paraphrase (conditions, tiers) stays in `data` jsonb. The two
> entity tables have **no foreign keys between them** — each is an independent typed descriptor. A
> cross-tariff limit (e.g. an all-plans cashier cap) repeats per row, or becomes its own typed table if it
> grows into a category.
>
> **Facts vs prose.** Products, tariffs and contacts are the **Facts lane** — exact, code-substituted,
> **exempt** from the "no literal amount in a topic body" rule, because a column value is a confirmed fact,
> not an invented one. A `topic` body is the **Knowledge lane** — prose the model writes and the grounding
> judge checks; any number in it must be a `{{table.slug.field}}` token, never a digit.

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
action           text   -- 'approve'|'edit' — a KB-lifecycle event
actor_user_id    uuid FK -> users  NULL              -- who did it (NULL = system, e.g. seed on boot)
version          int                                 -- LEGACY/UNUSED: KB is no longer versioned
note             text                                -- free-text detail ('approved 3 topics', 'edited tariff.growth.price')
created_at
```
> **Append-only history of the KB's lifecycle** — *not* per-message or per-draft activity. One row per
> significant action (a row/batch **approve** = `drafted_at` cleared, an **edit**): who, when, why. It
> answers "who approved the price that went out last Tuesday?" — the provenance/compliance trail behind
> the per-row approve gate. There is no publish/rollback (the KB is one living copy, not versioned), so
> the `version` column is **legacy/unused** — kept only to avoid churn. It is **never read on the hot
> path** and the brain never touches it. **v1: defined but empty** — nothing writes it until the
> approve flow exists; the seed-on-boot may write one row. Distinct from `wa_messages` (conversation)
> and `ai_suggestions` (suggestions).

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
(`organization_users`, `ai_topics/assets`, `ai_products`, `ai_tariffs`, `ai_contacts`, `assignment_events`) —
no array/CSV columns, **except `ai_suggestions.options`** (a deliberate jsonb document holding the 1–3
reply variants as one atomic suggestion — see denormalizations).

**2NF** — no partial dependencies. Junctions (`organization_users`) carry only attributes that
depend on the whole composite key (`joined_at`). Every other table has a single-column PK (`id`), so
2NF is automatic.

**3NF** — no transitive dependencies: a child table does **not** store an ancestor's
`organization_id` when it's reachable by FK. `organization_id` is stored **only** where it's a
direct fact — `organizations`, `organization_users`, `wa_accounts` (the assignment), `ai_snapshots`,
`ai_audit_log` — and **derived by join** everywhere else (`wa_contacts`, `wa_chats`, `wa_messages`,
`ai_topics/assets`, `ai_products`, `ai_tariffs`, `ai_contacts`, `ai_suggestions`) — the new KB tables reach
`organization_id` via their `snapshot_id`. The `assigned` flag was removed (derived from
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
   + its `suggested_status`/`suggested_callback`, `ai_products.data`, `ai_tariffs.data`,
   plus the `provenance` source-tracking columns on every KB row) — intentional
   semi-structured/document storage (open keyset, raw audit document, the 1–3 reply variants kept as
   one atomic suggestion, sphere-specific product attrs, descriptive tariff ranges/conditions), not
   repeating groups — a deliberate choice, not a 1NF violation. The `options` array trades per-option
   queryability for one-row supersede + no join to render a suggestion; the success metric stays
   computable (chosen option text vs the sent message).

> Trade-off note: if you later want strict tenant isolation / row-level security, denormalizing
> `organization_id` onto every child table is a valid, common alternative — at the cost of the 3NF
> purity above. The schema is written 3NF-first; that switch is additive.
