# UI Pages

The frontend is a single Vue 3 SPA (see `2-architecture.md`). It talks only to the backend
(`/api` + SSE). **v1 ships exactly two pages — Login and Chatboard.** Contacts, WhatsApp Accounts,
AI Assistant, and Settings are **deferred** (designed here, not in v1's nav).

> **Hard rule (every element is backed).** Nothing on a page may show data we can't retrieve or
> trigger an action we don't expose. Each region below lists **▸ Backed by:** the exact
> endpoint(s)/field(s) (`7.1-endpoints.md`, `9-database-schema.md`). If it isn't backed, it isn't on
> the page. These specs are image-generation-ready (layout sketch + tiers + visual cues + prompt).

## Design language

- **Brand:** XChats. Indigo "X" mark. Modern SaaS, Inter-like font, comfortable density.
- **Color:** primary **indigo** `#4F46E5`; **WhatsApp-green** `#22C55E` for send + outbound bubbles;
  white workspace; light-grey panels `#F8FAFC`; dark navy `#0F172A` for the login brand panel.
  Inbound bubble = light grey; outbound = green.
- **Avatars:** **initials on a colored circle** — we do **not** fetch WhatsApp profile pictures in v1
  (the `@lid`-keyed `contacts.update` events that carry them are ignored).
- **Shape:** rounded corners (8–12px), soft shadows, thin `#E2E8F0` dividers. **Russian UI** (v1 is
  Russian-only — no language switcher; i18n isn't built).

## The motto — three visibility tiers (apply to every page)

> *"Every piece of information is on the page, but one click away; only basic, core data is shown."*

- **Tier 1 — Core:** always visible.
- **Tier 2 — One click/hover:** an affordance (chevron, "⋯", "Подробнее", tab, hover) — not the data.
- **Tier 3 — Elsewhere:** full detail in its own drawer.

---

## 1. Login — `/login`

Split screen: dark brand panel (left) + white form (right).

```
┌───────────────────────┬───────────────────────────┐
│  XChats                │   Вход в аккаунт           │
│  Командный инбокс      │   Email   [__________]     │
│  и ИИ-ассистент        │   Пароль  [__________]     │
│  • Единый инбокс       │   [      Войти      ]      │
│  • ИИ-ответы           │                            │
│  • Безопасность        │   Нет аккаунта? — админ    │
└───────────────────────┴───────────────────────────┘
```

- **Core:** Email, Пароль, **Войти**. Static footer "Нет аккаунта? Свяжитесь с администратором"
  (users are created by an admin via the Users API — there is no self-signup).
- **Removed (not backed):** ~~Войти через Google~~ (no OAuth), ~~Запомнить меня~~, ~~Забыли пароль?~~
  (no password-reset endpoint), ~~language switcher~~ (no i18n in v1).
- **▸ Backed by:** `POST /auth/login {email,password}` → session cookie. That's the whole page.

**Image prompt:** *Clean modern SaaS login, split layout. Left half dark navy (#0F172A): indigo
"XChats" logo, headline "Командный инбокс и ИИ-ассистент", three small line-icon feature rows. Right
half white, centered: title "Вход в аккаунт", an Email input and a Пароль input, a single full-width
indigo "Войти" button, and small grey footer text "Нет аккаунта? Свяжитесь с администратором". NO
social-login buttons, NO "remember me", NO "forgot password", NO language selector. Inter font,
rounded inputs, generous whitespace.*

---

## 2. Chatboard — `/` (the main page)

Four regions on a strict diet. v1 runs against a **single pre-connected account**, so there is **no
per-chat account indicator** (every chat is the same number).

```
┌──┬────────────────┬──────────────────────────┬───────────────────┐
│N │ Chat list       │ Chat view                 │ Assistant panel   │
│a │ search + filter │ header · timeline         │ 1–3 AI options    │
│v │ chat rows       │ composer                  │ contact summary   │
└──┴────────────────┴──────────────────────────┴───────────────────┘
```

### Nav rail (icon-only, far left)
- **Core:** Inbox icon (active, indigo); user avatar pinned at bottom.
- **One-click:** clicking the avatar → small menu showing **name · email · org** and **Выйти**.
- v1 nav has **only Inbox** — deferred destinations aren't shown until built (no dead placeholders).
- **▸ Backed by:** `GET /me` (name/email/org), `POST /auth/logout`.

### Chat list (pane 1) — rows capped at **two lines**
- **Core per row:** **initials avatar**, contact name, one-line last-message preview, time, unread badge.
- **One-click:** a segmented filter **Мои · Неназначенные · Все** + a search field on top.
- **Removed (not backed):** ~~WhatsApp-account dot~~ (single account in v1), ~~status tabs~~ (no
  endpoint sets a chat to resolved).
- **▸ Backed by:** `GET /chats {q?, assignee=me|unassigned, page}` → `Chat{contact.display_name,
  last_message_preview, last_message_at, unread_count}`; live via SSE `chat.*`, `message.created`.

### Chat view (pane 2)
- **Core:** message timeline (inbound left/grey, outbound right/green), date separators, **delivery/
  read ticks**, composer (text + green **Отправить**).
- **One-click:** header = contact **name + phone** only; **Назначить** is one icon-button → a user
  picker. **Build 0 delta:** the composer **does** support a file attach (backed by `POST …/media` upload +
  `media_ids[]` send — see `TODO.md`); attaching N files sends N separate messages. *(The plan's original
  text-only-composer rule — "media only travels with an AI option" — is superseded for Build 0.)*
- **Removed (not backed):** ~~online/last-seen presence~~ (we don't track presence), ~~Решить/Resolve~~
  (no status endpoint), ~~profile-picture avatars~~ (initials only).
- **▸ Backed by:** `GET /chats/{id}/messages` → `Message{direction, content, media(list of urls), status,
  timestamp}`; `POST /chats/{id}/messages {text?, media_ids?[]}` (+ `POST /media` upload); `POST
  /chats/{id}/assign {user_id}` (+ `GET /users` for the picker); read-on-open via `POST /chats/{id}/read`;
  ticks from `message.status` over SSE.

### Assistant panel (pane 3) — **1–3 reply options, text + media**
The product's wedge; collapsible.

- **Empty state:** one primary **"Подсказать ответ"** button.
- **After "Suggest":** **1–3 option cards** (Вариант 1/2/3), each compact:
  - **Text** — editable inline (pencil).
  - **Media chips** — the option's suggested file(s) as chips, each with **×** to **detach**.
    (Attaching a *new* file needs the asset-catalog UI — **deferred**; v1 only detaches.)
  - **Actions:** **Принять и отправить** (green) + **Редактировать** (loads text + kept media into the
    composer). Picking one option sends it; the others are superseded — no separate reject button.
  - a small **confidence** dot when low.
- **Escalation state:** one card *"ИИ не знает — ответьте вручную"* + reason (no options).
- **Below:** a collapsed **contact mini-profile** — name, phone, **2–3 attributes**, **"Подробнее →"**
  opening a contact drawer (Tier 3).
- **▸ Backed by:** `POST /chats/{id}/ai-drafts` (→ 1–3 `AiDraft` options), `GET …/ai-drafts`,
  `POST /ai-drafts/{id}/approve {edited_text?, media_ids?}`; fields `AiDraft{ordinal, draft_text,
  media[], confidence, escalate, escalation_reason}`; contact summary from `Chat.contact`
  (`display_name, phone_number, attributes`); live via SSE `ai_draft.created`/`ai_draft.updated`.

**Image prompt:** *Modern team-inbox web app (Chatwoot-style), four columns, light theme, Inter font,
Russian UI. Far-left slim indigo icon rail with a single active chat icon and a round user avatar at
the bottom (no other nav icons). Column 2 "Чаты": search box, a segmented filter "Мои · Неназначенные
· Все", and chat rows — a colored circle with white initials (NOT a photo), bold contact name,
one-line grey preview, time, small unread count badge (no account dot). Column 3 chat view: header
with contact name and phone number and a single "назначить" person icon (no online status), a
scrollable thread with light-grey inbound bubbles left and green (#22C55E) outbound bubbles right,
blue double read-ticks, date separators, and a bottom composer with a plain text field and a green
"Отправить" button (no attachment icon). Column 4 panel "Подсказки ИИ": two stacked suggestion cards
"Вариант 1/2", each with 2–3 lines of editable text, one small document chip with an × to detach, a
green "Принять и отправить" button and a "Редактировать" link; below them a small collapsed "Контакт"
block with name, phone, two attributes and a "Подробнее →" link. Rounded corners, soft shadows,
whitespace.*

---

## 3. Contacts — `/contacts` *(deferred; spec for later)*

List + detail drawer.
- **Core:** searchable table — initials avatar, name, phone, last-contact time.
- **One-click (drawer):** identities (phone / `@lid` / push name), attributes, the contact's chats.
- **▸ Backed by:** `GET /contacts {q?, page}`, `GET /contacts/{id}` (→ `Contact{display_name,
  phone_number, phone_jid, lid_jid, push_name, attributes}` + recent chats).

**Image prompt:** *Contacts page in the XChats shell (slim indigo icon rail). Center: a table
"Контакты" with a search box and columns initials-avatar, name, phone, "последний контакт". An open
right detail drawer showing one contact's initials avatar, name, phone, a few key-value attributes, an
"Идентификаторы" list (phone, @lid, push name), and that contact's chats. Light theme, Inter, rounded.*

---

## 4. WhatsApp Accounts — `/accounts` *(deferred; v1 uses one pre-connected account)*

- **Core:** table of Evolution instances — name, **connection status** (colored dot), phone number,
  **Assigned** toggle.
- **One-click:** **＋ Добавить аккаунт** → modal (instance name → **QR** → on scan, auto-assigned);
  row "⋯" → reconnect / unassign.
- **▸ Backed by:** `GET /whatsapp-accounts` (→ `WhatsAppAccount{instance_name, connection_status,
  assigned, phone_number}`), `POST /whatsapp-accounts`, `GET|POST …/{id}/qr`, `POST …/{id}/assign|unassign`.

**Image prompt:** *WhatsApp accounts manager in the XChats shell. Table "WhatsApp аккаунты": columns
instance name, status pill (green "Подключён" / amber "QR" / grey "Отключён"), phone number, and an
"Назначен" toggle. Top-right primary "＋ Добавить аккаунт". One row's "⋯" menu open (Переподключить,
Снять назначение). Light theme, indigo accents, rounded.*

---

## 5. AI Assistant — `/assistant` *(deferred; v1 seeds the KB from SQL/markdown, no UI)*

- **Core:** left sub-nav (Персона · Знания · Цены · Медиа-ассеты · Плейграунд) + the editor.
- **One-click:** **Опубликовать** (eval-gated); a **Плейграунд** that shows the same **1–3 option
  cards** as the Chatboard.
- **▸ Backed by (deferred — Phase 4B):** `GET|PUT /assistant/config`, `POST /assistant/publish`,
  `POST /assistant/playground`; data in `ai_snapshots/ai_topics/ai_assets/ai_values`.

**Image prompt:** *AI assistant config page in the XChats shell. Left sub-nav (Персона, Знания, Цены,
Медиа-ассеты, Плейграунд). Center "Персона" editor: text-area fields and a green "Опубликовать"
button. Right "Плейграунд" strip with a sample-chat input and 1–3 generated suggestion cards. Clean,
light, indigo accents.*

---

## 6. Settings — `/settings` *(deferred)*

Tabbed: **Организация · Пользователи** (only these two — no "General"/theme/language tab, nothing
backs it).
- **Организация:** name, auto-response mode (NEVER / CONFIGURE_TIME / ALWAYS) + a time window
  (**UTC**, shown with a hint). **▸ Backed by:** `GET|PATCH /organization`.
- **Пользователи:** table of users; **＋ Добавить пользователя** (email + password → joins the org).
  **▸ Backed by:** `GET /users`, `POST /users {email,password,name?}`.

**Image prompt:** *Settings page in the XChats shell with two tabs "Организация · Пользователи".
Active "Организация": fields for org name, an auto-response mode segmented control
(Никогда/По расписанию/Всегда), a start–end time window with a small "UTC" hint, and an indigo
"Сохранить" button. Clean light form.*

---

## Out of scope (v1)

Reports/analytics, campaigns, SLA, macros, billing, granular permissions, self-signup, OAuth,
password reset, presence, i18n (per `1-concept.md`). Not shown, not stubbed.
