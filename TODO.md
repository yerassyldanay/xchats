# TODO — xchats v0 (working inbox: receive + send WhatsApp, with hardcoded drafts)

> **Goal in one line:** a WhatsApp **inbox** where a logged-in member sees incoming chats/messages
> (text **and** media), gets **3 hardcoded draft suggestions** per inbound message, and can **send**
> a reply (text and media) back to WhatsApp — wired to one real Evolution instance.
>
> This is the **first build**. The AI brain is **stubbed** (not the real port yet); everything else
> is real plumbing. "Done" = the demo loop below runs end-to-end against the live instance.

## The demo loop (acceptance)

1. A WhatsApp customer sends a text or image to the connected number.
2. It appears live in the chatboard (image rendered from our blob store).
3. The assistant panel shows **3 draft cards**, each = `"This is a test message from draft"` + a
   sample media file.
4. The member picks/edits one and clicks **Send** → it goes to WhatsApp via Evolution and shows as an
   outbound bubble.
5. The member can also type a free text/attach a file and send directly (no draft).
6. On boot, the **last 7 days** of chats/messages/contacts from the instance are already in the inbox.

---

## ⚠️ Scope notes — this v0 intentionally differs from `plan/` (flagging per your p.s.)

The `plan/` docs were just re-scoped to a "minimal slice" that **deferred** media and history sync and
**ported the real brain**. This v0 deliberately changes three of those:

| Topic | `plan/` says | **v0 (this TODO)** | Why / action |
|---|---|---|---|
| Media | placeholder only (deferred to 4C) | **in scope** — receive, store to blob, render, send | You asked for text+media both ways. |
| History sync | live only (deferred to v2) | **7-day initial sync in scope** | You asked for last-7-days sync. |
| AI brain | port the real tested brain (4A) | **hardcoded stub** returning 3 fixed drafts | You asked to hardcode it; de-risks AI entirely for now. |
| Draft trigger | on-demand "Suggest" button | **auto-generate 3 drafts on each inbound** | Stub is free (no LLM cost), and you want the draft to arrive with the message. |
| Drafts per message | one suggestion per inbound | **3 variants per inbound** | Schema delta below. |

These are fine as a "prove the plumbing" step. When the loop works, swapping the stub for the real
brain + flipping media/sync to their planned staging is straightforward.

---

## Config & secrets (do this first — don't commit secrets)

- One Evolution instance (already running): base `http://localhost:9700`.
  - Dashboard shows instance **id** `acf950c6-8dd6-4466-a81f-37d54646df01`, but Evolution API paths
    use the instance **name** (the python client defaults to `xpayment`). **Verify the name** via
    `GET /instance/fetchInstances` (header `apikey`) and use it as `EVO_INST`.
  - `apikey`: provided in chat — **store it in an untracked `config.yaml` / `.env`, never in git**.
    (It looked ~35 chars; double-check it's complete.)
- Add `config.yaml`, `.env` to `.gitignore`; commit only `config.example.yaml` / `.env.example` with
  placeholders. Env catalog: `EVO_BASE`, `EVO_KEY`, `EVO_INST`, `WHATSAPP_ACCOUNT_ID`, `DATABASE_URL`,
  `SESSION_SECRET`, `WEBHOOK_TOKEN`, `BLOB_DIR`, `API_BASE_URL`, plus a seed `ADMIN_EMAIL`/`ADMIN_PASSWORD`.

---

## Backend (Go + Postgres) — `backend/`

Reference oracle for all Evolution calls + normalization: `plan/scripts/evolution_client.py`
(tested against `evoapicloud/evolution-api:v2.3.7`). Exact endpoints below are taken from it.

### B1. Scaffold + config + DB
- Go module `backend/`, layout per `plan/2-architecture.md` (`cmd/xchats`, `internal/{config,httpapi,
  webhook,evolution,normalize,store,jobs,realtime,assistant,blob}`, `migrations/`).
- Postgres connection + migrations. **Subset of tables** to create now (from `plan/9-database-schema.md`):
  `organizations, members, sessions, whatsapp_accounts, contacts, contact_identities, conversations,
  messages, message_media, evolution_events, jobs, ai_drafts, ai_draft_assets`.
  (Skip `ai_snapshots/ai_topics/ai_assets/ai_prices/sync_jobs/assignment_events/ai_audit_log` — the
  brain is hardcoded.)
- Seed one org + one member (email+password, argon2id/bcrypt — **not sha256**) from config on boot.

### B2. Auth
- `POST /xchats/api/v1/auth/login` (email+password) → session cookie (`HttpOnly`, `SameSite=Lax`,
  `Secure` in prod). `GET /me`. `POST /logout`. Middleware guards the API.

### B3. Evolution client (`internal/evolution`) — port the python script
- `SendText(inst, number, text)` → `POST /message/sendText/{inst}` `{number, text}`.
- `SendMedia(inst, number, mediatype, mimetype, base64, fileName, caption?)` → `POST /message/sendMedia/{inst}`.
- `DownloadMedia(inst, messageId)` → `POST /chat/getBase64FromMediaMessage/{inst}`
  `{message:{key:{id}}, convertToMp4:false}` → `{base64, fileName, mimetype}`.
- `FindChats/FindContacts/FindMessages(inst, ...)` → `POST /chat/find{Chats,Contacts,Messages}/{inst}`
  (used by the 7-day sync). `FetchInstances()` to confirm the instance name.
- Set the webhook on the instance to point at us (Evolution `webhook/set` for the instance, with
  events `messages.upsert, messages.update, send.message, contacts.*, chats.*` and our `WEBHOOK_TOKEN`).

### B4. Webhook receiver (`internal/webhook`) — receive text + media
- `POST /evolution/api/v1/webhook/{whatsapp_account_id}`: verify `WEBHOOK_TOKEN` (**header only**),
  store raw to `evolution_events`, enqueue a `process_event` job, return 200 fast. Idempotent on
  `(account, event_kind, external_id)`.
- Worker normalizes (reuse `evolution_client.py` `normalize()` logic): upsert `contacts`
  (+`contact_identities`, `@lid`↔phone via `remoteJidAlt`), `conversations` (keyed on phone JID),
  `messages` (`direction` from `fromMe`, dedupe on `evolution_message_id`).
- **Media:** for `imageMessage`/`audioMessage`/`videoMessage`/`documentMessage`, enqueue
  `download_media` → call `DownloadMedia` → write bytes to the **blob store** (`internal/blob`,
  local-disk dir `BLOB_DIR`) → `message_media` row (`media_kind, mimetype, store_url, dl_state`).
- Emit SSE `message.created` / `message.updated` so the UI updates live.
- **Drop `@g.us` (group) events.**

### B5. Send pipeline (`internal/jobs` + httpapi) — send text + media from UI
- `POST /xchats/api/v1/conversations/{id}/messages` body `{text?, media?[]}`:
  insert a local `messages` row (`direction=out, sender_kind=member, delivery_state=queued`) **first**,
  then enqueue `outbound_send`.
- Worker resolves the **phone JID** (not `@lid`) and calls `SendText` or `SendMedia`; stores the
  returned Evolution `key.id`; updates `delivery_state`.
- **File upload:** `POST /xchats/api/v1/media` (multipart) → blob → returns a media handle the send
  body references. One send pipeline for member sends and approved-draft sends.

### B6. Hardcoded brain (`internal/assistant`) — 3 drafts per inbound
- A `Drafter` interface with a **stub** implementation: given an inbound message, return **3 variants**,
  each `text = "This is a test message from draft"` + 1 sample media file (seed 2-3 files under
  `backend/assistant/sample-media/`, e.g. an image, a pdf, an audio).
- On every normalized **inbound** message, enqueue `generate_ai_draft` → write **3** `ai_drafts` rows
  (variant 0/1/2) + their `ai_draft_assets` → emit SSE `ai_draft.created`.
- `GET /conversations/{id}/ai-drafts` → the 3 cards.
- `POST /ai-drafts/{id}/approve` body `{edited_text?}` → sends via the **B5 send pipeline** (text +
  the draft's media), stamps `sent_message_id`, sets that draft `sent` and its 2 siblings `superseded`
  (→ SSE `ai_draft.updated`).

### B7. 7-day initial sync (run after the live loop works)
- On boot (or a `make sync` command / one-shot job): `FindChats` → for each chat `FindMessages`
  (filter `messageTimestamp >= now-7d`) → upsert through the **same normalize+upsert path** as live
  (idempotent, so live + sync never duplicate). `FindContacts` to enrich contact names.
- Media in synced messages: download lazily (same `download_media` job) or skip bytes and keep the
  metadata — **decide:** lazy-download is heavier; metadata-only is faster. *Recommend metadata-only
  for synced history, download on first open.*

### B8. Realtime
- `GET /xchats/api/v1/realtime` (SSE): `message.created`, `message.updated`, `ai_draft.created`,
  `ai_draft.updated`. (Polling is an acceptable fallback if SSE slows the build.)

### Schema delta for 3 drafts (`ai_drafts`)
- Add `variant_index int` (0/1/2). Replace the one-pending-per-conversation unique with
  `UNIQUE (trigger_message_id, variant_index)`. Approving one variant supersedes its siblings.
- Draft media via `ai_draft_assets` (already in schema) → resolves to blob `store_url`.

---

## Frontend (Vue 3 SPA) — `frontend/`

- **Login page** — email + password → session; redirect to chatboard.
- **Chatboard** (`/chatboard`), three areas:
  - **Sidebar / conversation list** — chats with name, last-message preview, unread; live via SSE.
  - **Chat thread** — inbound/outbound bubbles, **media cards** (image preview / file / audio from our
    `/media/{id}`), composer with **text + attach file + send**.
  - **Assistant panel** — the **3 draft cards** for the latest inbound (text + media preview); per card:
    **Approve** (send as-is) and **Edit** (loads text into the composer, keeps media), → uses the send
    API. Show a simple "no drafts yet" empty state.
- Talks only to `/xchats/api/v1` (+ SSE) via `API_BASE_URL`.

---

## Deploy — `deploy/` + `Makefile`

- `docker-compose` for Postgres (+ reuse the existing Evolution at `localhost:9700`, not containerized
  here). Backend + frontend run via Make.
- Makefile targets:
  - `make up` — start Postgres, run migrations, start backend + frontend.
  - `make migrate` — apply migrations. `make seed` — seed org+user.
  - `make sync` — run the 7-day initial sync once.
  - `make webhook-set` — register our webhook on the Evolution instance.
  - `make test` — backend unit tests + a normalize test against `plan/captures/` fixtures.
  - `make smoke` — manual: send a text+media to the instance and confirm round-trip.

---

## Gaps / decisions you didn't specify (flagging per your p.s.)

1. **Instance name vs id** — API needs the **name** (likely `xpayment`), not the dashboard UUID.
   Confirm via `fetchInstances`. *(Blocking for any Evolution call.)*
2. **Delivery/read ticks** — you said "send + receive," not status. Cheap to store `messages.update`
   while we're here. **Recommend:** store + show basic sent/delivered/read; skip if it slows us.
3. **Draft media source** — the 3 hardcoded drafts attach media; we need **sample files committed**
   (image/pdf/audio under `backend/assistant/sample-media/`). Confirm that's fine, or point at specific
   files.
4. **3 drafts = same or different?** Assumed same text, **different attached sample media** per card so
   they're visibly distinct. Say if you want 3 different texts instead.
5. **Webhook reachability** — Evolution at `localhost:9700` must reach our backend's webhook URL. If
   both are local this is fine; if Evolution is remote/containerized we need a tunnel. Confirm topology.
6. **Auto-draft on inbound** — chosen because the stub is free. If you'd rather keep the on-demand
   "Suggest" button from the plan, it's a one-line trigger change.
7. **Scale** — single org, single number, low volume assumed. In-process worker (no Kafka). Fine for v0.
8. **`fromMe` from phone/Web** — messages you send from the actual phone also arrive via webhook
   (`fromMe=true`); store them as outbound `external_account` so the thread stays correct.

---

## Suggested build order (fastest path to the demo loop)

1. **B1–B2 + FE Login + empty chatboard** → log in, see empty inbox. (`make up`, `make migrate/seed`.)
2. **B3 + B4** (webhook receive, text first, then media→blob) + **B8 SSE** + FE thread →
   *real inbound messages appear live, images render.*
3. **B5** + FE composer → *send text + media back to WhatsApp.*
4. **B6** + FE assistant panel → *3 hardcoded drafts per inbound; approve sends.*
5. **B7** (`make sync`) → *last 7 days backfilled.*
6. **Makefile polish + `make smoke`** → one-command run + manual round-trip check.

> Order matters: get the **live** receive→send loop solid (steps 2–3) before sync (step 5); sync
> reuses the same normalize+upsert path, so it's safest to build last.
