# TODO — xchats Build 0 (runnable scaffold + live inbox, hardcoded AI)

> **Goal in one line:** a runnable backend + frontend where a logged-in user sees **live** WhatsApp
> chats/messages (**text and media**), can **send** replies (**text and media**) back to WhatsApp via
> one real Evolution instance, and the AI is a **hardcoded stub** returning constant draft option(s).
>
> Build 0 = prove the plumbing end-to-end. The real ported brain, history sync, and the multi-account
> connect/QR manager are **later** (see `plan/0.1-definition-of-done.md`).

## Acceptance — the demo loop

1. Log in (email + password) → land on the Chatboard.
2. A WhatsApp customer sends **text or an image** to the connected number → it appears **live** in the
   chat thread (image rendered from our blob store).
3. Press **"Подсказать ответ"** → the assistant panel shows **1–3 hardcoded option cards** (constant
   text + a sample media file).
4. Pick/edit one → **Send** → it reaches WhatsApp via Evolution and shows as an outbound bubble;
   delivery/read ticks update.
5. Or type free text / attach a file and **send directly** (no draft).
6. A message you send from the **actual phone** also appears (outbound `external_account`).

No history sync in Build 0 — **live only**. The inbox starts empty and fills going forward.

## Relation to `plan/` (read this — it's the source of truth)

Build 0 **follows the plan's architecture and schema**: the dedicated **`xchats` schema** with `wa_*`
prefixes, `id` PKs (`wa_accounts.id = uuidv5(XCHATS_WA_NS, owner_jid)`), the **in-memory queue behind a
`Queue` port** (no `evolution_events`/`jobs`/`sync_jobs` tables), live-only ingest, and the **1–3 options**
draft model. See `plan/9-database-schema.md`, `plan/3-sync.md`, `plan/7.1-endpoints.md`, `plan/5-ui-pages.md`.

Two **deliberate deltas** from the plan's text-only/real-brain v1:

| Topic | plan/v1 | **Build 0** |
|---|---|---|
| Media | deferred to Phase 4C (text-only; `message_media` removed) | **included & owned** — receive→blob→serve, and send. `message_media` + blob are back **with a dedup key** (B1/B6). To the UI, `Message.media` is a **list of URLs**; sending N files = **N separate WhatsApp messages** (one Evolution `sendMedia` per file). |
| AI brain | port the real tested brain (Phase 4A) | **hardcoded stub** returning constant 1–3 options — **but** the Phase-0 brain port-risk gate (B0) is run first so the port isn't validated last. |

Everything else (queue, names, live-only, deterministic account id, on-demand "Suggest") is **per the
plan**. Because media is back, **Build 0 ≈ Phase 1 + 2 + 3 + Phase-4C media − the real brain** — it is
*not* the plan's "minimal text-only slice." The plan docs are reconciled to match (media un-deferred in
`plan/3-sync.md` / `plan/9-database-schema.md`; send body + `POST /media` in `plan/7.1-endpoints.md`;
operator attach noted in `plan/5-ui-pages.md`).

---

## Tech stack

**Backend — Go**
- **Routing:** **Gin** (`gin-gonic/gin`) — all HTTP + SSE.
- **Postgres:** **pgx v5** (`jackc/pgx/v5` + `pgxpool`) — a connection pool, plain SQL (no ORM).
- **Migrations:** **golang-migrate** — SQL files in `backend/migrations/`, run by `make migrate`.
- **Logging:** stdlib **`log/slog`** with a **logfmt** handler (slog's `TextHandler` emits
  `key=value` logfmt). One structured line per HTTP request, webhook event, queue task, and Evolution
  call; `log_level` / `log_format=logfmt` from config. Example:
  `ts=2026-06-15T10:00:00Z level=info msg="event processed" kind=messages.upsert wa_account_id=… chat_id=… dur_ms=4`
- **Auth:** cookie sessions; **argon2id** password hashing (`golang.org/x/crypto/argon2`).
- **Config:** env (`caarlos0/env`) + `gopkg.in/yaml.v3` (no Viper).
- **Queue:** in-memory (Go channels) behind the `Queue` port (`internal/queue`).
- **Evolution client / blob:** stdlib `net/http` + `os` (local-disk blob).
- **Testing:** stdlib `testing` + **testify**; **testcontainers-go** (ephemeral Postgres); fake
  Evolution via `net/http/httptest`.

**Frontend — Vue 3**
- **Build/lang:** **Vite** + **TypeScript**. **Router:** vue-router. **State:** **Pinia**.
- **Styling:** **Tailwind CSS** (matches the mockups). **Realtime:** native **`EventSource`** (SSE);
  HTTP via `fetch`.
- **Logging:** a small **logfmt** logger util (`key=value` → console; one line per route change, API
  call, SSE event, send/approve action) — **same format as the backend**, e.g.
  `ts=… level=info msg="draft approved" chat_id=… draft_id=…`.
- **Testing:** **Vitest** + **Vue Test Utils** (component); **Playwright** for e2e (optional).

> Both sides emit **logfmt**. Backend → stdout (so `docker logs` / a collector parse it); frontend →
> browser console (Build 0 keeps it client-side; shipping logs to a backend sink is a later add).

---

## Config & secrets (do first — never commit secrets)

`.gitignore` `config.yaml` + `.env`; commit only `config.example.yaml` + `.env.example` with placeholders.

**`.env` (secrets):**
```
EVOLUTION_BASE_URL=http://localhost:9700
EVOLUTION_API_KEY=<from chat — verify it's complete>
EVOLUTION_INSTANCE=xpayment           # the instance NAME (confirm via GET /instance/fetchInstances)
WEBHOOK_TOKEN=<shared token; Evolution sends it, we verify (header only)>
WEBHOOK_PUBLIC_BASE_URL=<URL Evolution uses to reach our webhook>
DATABASE_URL=postgres://...
SESSION_SECRET=<random>
SEED_ADMIN_EMAIL=...                   # seeds the one login
SEED_ADMIN_PASSWORD=...
```
**`config.yaml` (non-secret tunables + seed):** `blob_dir`, `api_base_url`, `queue_driver: inmem`,
`log_format: logfmt`, `log_level: info`, `org_name`, `wa_account_display_name`, **`wa_owner_jid`** (the
connected number's own JID, e.g. `77011111111@s.whatsapp.net`). The account **`id` is derived
deterministically** as `uuidv5(XCHATS_WA_NS, canonical(wa_owner_jid))` and the `wa_accounts` row is
**seeded at boot** (B1) — so the webhook URL id is known *before* the first event (see B4). If `wa_owner_jid`
is unset, learn it once via `FetchInstances` at boot, then persist it.

---

## Backend (Go + Postgres) — `backend/`

Reference oracle for all Evolution calls + normalization: `plan/scripts/evolution_client.py`
(tested vs `evoapicloud/evolution-api:v2.3.7`); real payload shapes in `plan/captures/`.

**Response convention (all `/xchats/api/v1/*`):** every response (success or error) uses the unified
`{payload, errcode, message}` envelope and the error-code table in `plan/7-api-contracts.md` — the FE branches
on `errcode` (and the logfmt logger logs it). Ops (`/healthz`, `/readyz`) and the Evolution webhook are the
only routes outside the envelope. New codes (e.g. `DRAFT_STALE`) are added to that table first.

### B0. Brain port-risk gate (do first — retire the biggest unknown before the UI rides on it)
Build 0 ships a **stub** `Drafter` (B8), but the *real* product risk is whether the vendored brain is even
portable. Run this gate **before/parallel to** the UI plumbing — per `plan/0.1-definition-of-done.md` Phase 0:
- `cd plan/examples/repos/xpayment-crm && go test ./...` **passes**; **reuse/licensing confirmed**. If it
  fails, the AI is a *build*, not a *port* — re-scope before building the assistant panel on its contract.
- One **offline dry-run**: `HandleMessage` against a `plan/captures/` inbound fixture + a seeded snapshot →
  the produced `Draft` dumped to a log (no UI). Earliest honest answer to "is the brain even wireable?".
- Deliverable is the gate result, not wired code: B8 still uses the hardcoded stub for the demo loop.

### B1. Scaffold + config + DB + migrations
- Go module `backend/`, layout per `plan/2-architecture.md`: `cmd/xchats`,
  `internal/{config,httpapi,webhook,evolution,normalize,store,queue,realtime,assistant,blob}`,
  `migrations/`.
- **Schema namespace (Evolution shares this Postgres):** first migration
  `CREATE SCHEMA IF NOT EXISTS xchats` and `CREATE EXTENSION IF NOT EXISTS citext`; create **every** table
  fully-qualified as `xchats.<name>` and set `search_path=xchats,public` on the pool. Never create in `public`
  (`plan/9-database-schema.md` convention #1).
- **Write the migrations** (DDL from `plan/9-database-schema.md`) for the **Build 0 subset**:
  `organizations, users, organization_users, sessions, wa_accounts, wa_contacts, wa_chats,
  wa_messages, message_media, ai_drafts, ai_draft_assets`.
  Skip `ai_snapshots/ai_topics/ai_assets/ai_prices/ai_audit_log/assignment_events` (AI is stubbed).
  **No** `evolution_events`/`jobs`/`sync_jobs` tables (the queue is in-memory).
- **Load-bearing constraints to include** (not just the table list):
  - `users.email` is **`citext UNIQUE`** (case-insensitive login identity — without the extension/type
    `Foo@Bar.com` and `foo@bar.com` become two users).
  - The three dedup uniques: `wa_messages UNIQUE(account_id, evolution_message_id)`,
    `wa_chats UNIQUE(account_id, remote_jid)`, `wa_contacts UNIQUE(account_id, phone_jid)`.
  - `ai_drafts` **`PARTIAL UNIQUE(chat_id, option_ordinal) WHERE draft_state='suggested'`** (the single
    pending-suggestion invariant the approve guard in B8 relies on).
  - `message_media` carries a dedup key **`UNIQUE(message_id)`** (B6 is idempotent against double-delivery);
    columns `media_type, mimetype, file_name, file_size, storage_url, download_status` (API vocabulary).
  - FKs to `wa_accounts` are `ON DELETE RESTRICT` (children can't be orphaned).
- Seed one `organizations` + one `users` (email+password, **argon2id** — not sha256) from config on boot.
- **Pin the single WhatsApp account at boot** (so the webhook URL id is known before any event — see B4):
  read `owner_jid` from config (or `FetchInstances`), canonicalize (lowercase/trim, phone-JID form), compute
  `id = uuidv5(XCHATS_WA_NS, owner_jid)` with a **fixed** `XCHATS_WA_NS` namespace constant, and
  `INSERT … ON CONFLICT (id) DO UPDATE` the `wa_accounts` row.
- Serve **`GET /healthz`** (200) and **`GET /readyz`** (DB reachable) — unversioned, no envelope
  (`plan/0.1-definition-of-done.md` Phase 1; `plan/7-api-contracts.md` Ops).

### B2. Queue port (`internal/queue`)
- The `Queue` interface (`Publish` / `Consume`) + an **`inmem`** driver (buffered channel + worker pool)
  per `plan/3-sync.md` → "Queue abstraction". Message kinds: `wa_event`, `media_download`,
  `outbound_send`, `ai_draft`. Producers/consumers depend only on the interface (swap to Redis later).
- **Failure policy (state it — the demo depends on one message surviving):** a `handle()` error is **logged
  and the message dropped** (Build 0 accepts the loss window per `plan/3-sync.md`); the worker goroutine
  **recovers from panics** so one bad payload (e.g. a `normalize()` crash) can't kill the pool and stall the
  whole inbox. In tests the queue drains **synchronously** (drain-and-assert).

### B3. Auth (`internal/httpapi`)
- `POST /xchats/api/v1/auth/login` (email+password) → session cookie (`HttpOnly`, `SameSite=Lax`,
  `Secure` in prod). `GET /me`. `POST /auth/logout`. Middleware guards `/xchats/api/v1/*`.

### B4. Evolution client (`internal/evolution`) — port the python script
- `SendText(inst, number, text)` → `POST /message/sendText/{inst}`.
- `SendMedia(inst, number, mediatype, mimetype, base64, fileName, caption?)` → `POST /message/sendMedia/{inst}`.
- `GetBase64(inst, messageId)` → `POST /chat/getBase64FromMediaMessage/{inst}` → `{base64, fileName, mimetype}`
  (fallback when an inbound media event has no inline `message.base64`).
- `FetchInstances()` to confirm the instance **name** **and read the connected number's `owner_jid`** (used
  to seed the account + derive its id at boot — B1).
- `make webhook-set`: register our webhook on the instance (events `messages.upsert, messages.update,
  send.message, connection.update, contacts.*, chats.*`) with `WEBHOOK_TOKEN`, pointing at
  `{WEBHOOK_PUBLIC_BASE_URL}/evolution/api/v1/webhook/{wa_account_id}` — where **`{wa_account_id}` is the
  boot-seeded `uuidv5(XCHATS_WA_NS, owner_jid)`** (known before any event, so no chicken-and-egg and the edge
  never sees an "unknown account" for the first inbound).

### B5. Webhook → queue → normalize/upsert (live receive)
- `POST /evolution/api/v1/webhook/{wa_account_id}`: verify `WEBHOOK_TOKEN` (**header only**),
  **enqueue the raw event** onto the in-memory queue (`wa_event`), return **200** — **no DB write**.
- Worker consumes `wa_event`, normalizes (reuse `evolution_client.py` `normalize()`), and upserts
  (idempotent on the unique keys — `plan/3-sync.md`):
  - **resolve the account** to the **boot-seeded** row (B1). The account's own `owner_jid` is the event's
    **top-level envelope `sender`** field (e.g. `77011111111@s.whatsapp.net`) — **not** `data.key.remoteJid`
    (that is the *customer*). `id = uuidv5(XCHATS_WA_NS, canonical(owner_jid))`; with the single seeded
    account this is a lookup, not a create.
  - upsert `wa_contacts` (by `phone_jid` from `remoteJidAlt`; store `lid_jid`/`push_name`),
    `wa_chats` (by `remote_jid`), `wa_messages` (`direction` from `key.fromMe`, dedupe on
    `evolution_message_id`, raw on `wa_messages.raw`).
  - **Drop `@g.us` (group) events** at the door.
  - **Outbound provenance & own-send echo (avoid duplicate bubbles).** Both our own app-sends *and* phone/Web
    sends echo back as `fromMe=true` (`send.message` / `messages.upsert`). Rule:
    - if a **local `wa_messages` row already exists** for that `evolution_message_id` (or a queued
      `sender_kind='user'` row that B7 stamped with this `key.id`) → the echo is **enrichment**: update that
      row (don't insert a second one).
    - else → it's a genuine **external** reply: insert outbound `sender_kind='external_account'`.
    This is load-bearing because the queued `user` row has an **empty `evolution_message_id` until SendText
    returns**; B7 stamps it, and the matching here collapses the echo. (Captures: each event fires twice —
    `plan/captures/README.md` finding 3 — so this path runs constantly.)
  - **Maintain the chat aggregates** (`plan/3-sync.md` worker step 5 — the chat list renders only these):
    on each upserted inbound, update `wa_chats.last_message_at`, `last_message_preview`, and bump
    `unread_count`; outbound/`read` reset it.
- **Emit SSE**: `message.created` (+ `message.updated` for echo enrichment / status); `chat.created` when the
  worker **inserts** a new `wa_chats` row, else `chat.updated` (carrying the refreshed aggregates).

### B5a. Inbox read endpoints (hydrate the UI — SSE is only the *live* layer)
Live-only means no history backfill from Evolution; it does **not** mean "no read API for rows already in our
Postgres". Without these, a page reload or opening an existing chat shows an empty pane.
- `GET /xchats/api/v1/chats` — `{status?, assignee?(me|unassigned|uuid), q?, page, page_size}` →
  `{items:[Chat], page, page_size, total}`. `Chat` carries `contact, last_message_preview, last_message_at,
  unread_count` (the cached aggregates from B5).
- `GET /xchats/api/v1/chats/{id}/messages` — `{before?, limit?}` cursor → `{items:[Message], next_before}`.
  `Message.media` is a **list of URLs** (B6) to `GET /media/{id}`.
- `POST /xchats/api/v1/chats/{id}/read` — zeroes `unread_count`, emits `chat.updated` (called on chat-open;
  pairs with the unread badge that B5 increments). Without it the badge can never clear.
- The FE chat-list/thread **fetch these on mount + chat-select**, then append `message.*` / `chat.*` SSE deltas.

### B6. Media pipeline (receive) — `internal/blob`
- **Idempotent against double-delivery** (every event fires twice): only run the media work when the
  `wa_messages` upsert was a **genuine INSERT** (`RETURNING`/`xmax=0`), and write the blob to a
  **deterministic, message-id-derived path** so a re-delivery overwrites the same bytes instead of writing a
  second ~1 MB blob; `message_media` has **`UNIQUE(message_id)`** so the row insert is a no-op on replay.
- For `imageMessage/audioMessage/videoMessage/documentMessage`: if the event carries inline
  `data.message.base64` (the real captures do — `plan/captures/README.md`), write it straight to the blob
  store; else enqueue `media_download` → `GetBase64` → write bytes to `BLOB_DIR`. Then a `message_media`
  row using **API column names** (`media_type, mimetype, file_name, file_size, storage_url, download_status`).
  Emit `message.updated`.
- Serve bytes via `GET /xchats/api/v1/media/{id}` (streams the blob; `Content-Type` per the asset;
  `404` unknown, `502 MEDIA_UNAVAILABLE` if not downloadable). `Message.media` to the UI is the **list of these
  URLs**.

### B7. Send pipeline (`outbound_send`) — send text + media
- `POST /xchats/api/v1/chats/{id}/messages` body `{text?, media_ids?[]}`. **Fan-out** (Evolution sends one
  file per call): produce **one outbound message per part** — the `text` (if present) is one message, and
  **each `media_id` is its own message** → each gets its **own `wa_messages` row** + its own `outbound_send`
  task. Insert each row (`direction=out, sender_kind=<provenance>, delivery_state='queued'`) **first**,
  broadcast `message.created`, then enqueue.
- **Provenance is a parameter:** manual composer sends pass `sender_kind='user'`; **B8 approve passes
  `sender_kind='ai'`** (do not hardcode `user`).
- Worker resolves the **phone JID** (not `@lid`), calls `SendText` or `SendMedia` (media from blob → base64),
  **stamps the returned `key.id` onto this row's `evolution_message_id`** (this is what lets B5 collapse the
  `fromMe=true` echo — see B5), sets `delivery_state=sent|failed`.
- **Upload:** `POST /xchats/api/v1/media` (multipart) → blob → `{payload:{media_id}, errcode:"OK"}`; the
  `media_id` is then referenced in a send body. (Documented in `plan/7.1-endpoints.md`.)
- Status: `messages.update` (via the same `wa_event` path) advances `delivery_state`
  sent→delivered→read **monotonically**, matched on `evolution_message_id` (`plan/captures` finding 4).

### B8. Hardcoded AI stub (`internal/assistant`)
- A `Drafter` interface + **stub** impl: ignore the brain; return **1–3 constant options**, each
  `draft_text = "Это тестовый вариант ответа"` + one **sample media** file (commit 2–3 under
  `backend/assistant/sample-media/` — an image, a pdf, an audio).
- `POST /xchats/api/v1/chats/{id}/ai-drafts` (on-demand "Suggest") → enqueue `ai_draft` → write **1–3**
  `ai_drafts` rows (`option_ordinal` 1..3) + `ai_draft_assets` → SSE `ai_draft.created`. **Idempotent:** if a
  pending suggestion already exists, return it (`409 CONFLICT`) rather than creating a second; a *new* Suggest
  first **supersedes** the prior pending set (so the `PARTIAL UNIQUE(chat_id, option_ordinal) WHERE
  draft_state='suggested'` index never trips).
- `GET /xchats/api/v1/chats/{id}/ai-drafts` → the options.
- `POST /xchats/api/v1/ai-drafts/{id}/approve` `{edited_text?, media_ids?}` — **guarded single-send**:
  conditional `UPDATE ai_drafts SET draft_state='sent', sent_message_id=… WHERE id=$1 AND
  draft_state='suggested'`; **0 rows → `409 CONFLICT`** (already approved) or **`409 DRAFT_STALE`**
  (superseded by a newer inbound). **Only when the UPDATE wins**, send via **B7 with `sender_kind='ai'`**;
  mark siblings `superseded`; SSE `ai_draft.updated`. (Prevents a double-click sending the same option to a
  real customer twice.)

### B9. Realtime
- `GET /xchats/api/v1/realtime` (SSE): `message.created`, `message.updated`, `chat.created`,
  `chat.updated`, `ai_draft.created`, `ai_draft.updated`. (Polling is an acceptable fallback.)

---

## Frontend (Vue 3 SPA) — `frontend/` (build the two pages per `plan/5-ui-pages.md`)

- **Login** — email + password → session; redirect to Chatboard. (No Google/remember/forgot — not built.)
- **Chatboard** — three working areas (initials avatars; single account so **no account dot**). Each pane
  **hydrates from a GET on mount/select, then appends SSE deltas** (SSE is the live layer, not the data source):
  - **Chat list** — name, last-message preview, time, unread. Hydrate `GET /chats`; live via `chat.*` /
    `message.created` SSE. The **Мои/Неназначенные/Все** filter is **visible-but-inert in Build 0** (single
    user, no assignment) — wire `assignee=` only when multi-user lands. (Keep or hide, but don't imply backing.)
  - **Chat thread** — inbound/outbound bubbles, **media cards** (image/file/audio from `/media/{id}`),
    delivery/read ticks, composer (**text + attach file + Send**). Hydrate `GET /chats/{id}/messages` on open;
    call `POST /chats/{id}/read` on open. Attaching N files sends **N separate messages** (B7 fan-out).
  - **Assistant panel** — **"Подсказать ответ"** → 1–3 option cards (text + media chip with **detach**);
    per card **Принять и отправить** + **Редактировать** (loads into composer). Empty state otherwise.
    Handle approve `409 CONFLICT`/`DRAFT_STALE` by clearing the stale cards (don't resend).
- Talks only to `/xchats/api/v1` (+ SSE) via `API_BASE_URL`; all responses are the `{payload, errcode}` envelope.

---

## Deploy — `deploy/` + `Dockerfile`s + `Makefile`

- `deploy/docker-compose.yaml`: Postgres (reuse the existing Evolution at `localhost:9700`, not
  containerized here). `Dockerfile` for backend (Go) and frontend (Vite build → static serve).
- Makefile: `make up` (Postgres + migrate + seed + backend + frontend), `make migrate`, `make seed`,
  `make webhook-set` (register our webhook), `make test` (unit + component), `make test-e2e` (the full
  isolated loop vs `plan/captures/` + fake Evolution), `make smoke` (manual: real send round-trip).

---

## Testing in isolation (no real WhatsApp) — per `plan/6-isolated-testing.md`

The only true external is Evolution; we **fake it** and replay the **real captured payloads** in
`plan/captures/`. Build 0 has **no LLM** (the AI is hardcoded), so **no fake LLM is needed** — the
whole suite runs offline and deterministically.

- **Fake Evolution** (`httptest` server): implements the REST we call — `message/sendText`,
  `message/sendMedia`, `chat/getBase64FromMediaMessage`, `instance/fetchInstances` — with canned
  responses; **records every request** (so tests assert *"we sent to the phone JID, not the `@lid`"*
  and the correct body); and can **POST captured webhook events** at our
  `/evolution/api/v1/webhook/{wa_account_id}` edge.
- **Ephemeral Postgres** via **testcontainers-go** (or `deploy/postgres.compose.yaml`); migrations
  applied fresh each run; the **in-memory queue** runs synchronously in tests (drain-and-assert).
- **Layers**
  1. **Unit** — `normalize()` against `plan/captures/` (text / image / image+caption) → expected rows;
     the `Queue` inmem driver; the send-body builder.
  2. **Component** — webhook handler (enqueue + 200, idempotent on replay), the worker (consume →
     upsert, **no duplicates** on re-delivery), media download → temp `BLOB_DIR` + `message_media`.
  3. **E2E — one command (`make test-e2e`)**: fresh Postgres + fake Evolution + the backend. Replay a
     captured inbound (text, then image) → assert `wa_contacts`/`wa_chats`/`wa_messages` rows + SSE **and**
     that **`GET /chats` + `GET /chats/{id}/messages` repaint the same state** (refresh survives);
     `POST` a send with `text` + **2 `media_ids` → assert 3 separate outbound rows** and the fake Evolution
     received `sendText`/`sendMedia` **to the phone JID**; replay the `fromMe=true` **echo → assert no
     duplicate bubble** (the queued `user` row absorbed it, no `external_account` row); replay
     `messages.update` → ticks advance `sent→delivered→read` **monotonically**; press **Suggest** → 3
     hardcoded options → **approve** → assert it sent **with `sender_kind='ai'`** and siblings superseded,
     and **approve again → `409` with exactly one send** at the fake Evolution.
  4. **Frontend** — **Vitest** component tests (chat list / thread / assistant panel) against a mocked
     API + a fake `EventSource`; optional **Playwright** e2e driving the real backend + fake Evolution.
- **Determinism:** real captured fixtures, no network, no real number. `make test` runs unit+component;
  `make test-e2e` runs the full loop. The **only** non-isolated check is `make smoke` (manual, once,
  against the live instance).
- **Fixture honesty (don't over-claim):** the committed image captures' inline base64 is a **truncated
  placeholder** (not a decodable JPEG), and `getBase64FromMediaMessage` is **uncaptured**. So the offline e2e
  asserts on **`message_media` rows / byte-length / SSE**, *not* on a rendered image — **commit one small
  valid image fixture** if you want a true blob-render assertion; "image rendered from our blob store" is a
  `make smoke` (live) claim. Also note: while the AI is a stub the Suggest→approve e2e proves **plumbing
  only** — answer-quality is gated later (B0 is the earliest real signal).

---

## Open operational items (confirm during B1–B5)

1. **Instance name** — confirm `xpayment` via `GET /instance/fetchInstances` (API uses the name, not the
   dashboard UUID). *Blocking for any Evolution call.*
2. **Webhook reachability** — Evolution (`localhost:9700`) must reach `WEBHOOK_PUBLIC_BASE_URL`. If both
   are local, fine; if Evolution is remote/containerized, a tunnel is needed. Confirm topology.
3. **Sample media files** — commit an image/pdf/audio under `backend/assistant/sample-media/` for the
   stub's attachments. (Also commit **one small valid image** for the media e2e — see testing.)
4. **API key** — verify it's complete (~35 chars) and lives only in `.env`.
5. **Owner JID** — confirm the connected number's own `owner_jid` (the envelope top-level `sender`, e.g.
   `77011111111@s.whatsapp.net`) via `FetchInstances`; it seeds the account + derives its id + the webhook
   URL. *Blocking — get this wrong and the account id becomes `uuidv5(customer_phone)`.*
6. **Brain submodule** — `git submodule status` initialized and `go test ./...` passes under
   `plan/examples/repos/xpayment-crm` (the **B0** gate); reuse/licensing confirmed.

## Build order (fastest path to the demo loop)

0. **B0 brain gate** (in parallel) → `go test ./...` on the submodule + one offline dry-run; retire the port
   risk before the assistant panel rides on it.
1. **B1–B3 + FE Login + empty Chatboard** → log in, see empty inbox (account seeded at boot; `xchats` schema).
2. **B4–B5 + B5a + B9 (SSE) + FE thread** → real inbound **text** appears live **and survives refresh**
   (`GET /chats` + `GET messages` hydrate).
3. **B6 + FE media cards** → inbound **images** render from the blob (idempotent on double-delivery).
4. **B7 + FE composer** → send **text + media** back to WhatsApp (N files = N messages); ticks update; **no
   echo dupes**.
5. **B8 + FE assistant panel** → **Подсказать** shows 1–3 hardcoded options; approve sends as `sender_kind='ai'`
   (guarded single-send).
6. **Dockerfiles + Makefile polish + `make smoke`** → one-command run + manual round-trip.

> Get the **live receive→send** loop solid (steps 2–4) first; the AI stub (step 5) rides the same send
> pipeline. History sync, the real brain, and the accounts/QR manager are **out of Build 0**.
