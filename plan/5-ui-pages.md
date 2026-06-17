# UI Pages

The frontend is a single Vue 3 SPA (see `2-architecture.md`). It talks only to the backend
(`/api` + SSE). **v1 ships four routed pages — Login, Chatboard, WhatsApp Accounts, and Instances
Maintenance.** Contacts, AI Assistant, and Settings are **deferred** (designed here, not yet routed).

> **Hard rule (every element is backed).** Nothing on a page may show data we can't retrieve or
> trigger an action we don't expose. Each region below lists **▸ Backed by:** the exact
> endpoint(s)/field(s) (`7.1-endpoints.md`, `9-database-schema.md`). If it isn't backed, it isn't on
> the page. A few intentional **UI stubs** (e.g. the chat-header "Решить"/overflow actions) are called
> out explicitly as not-yet-backed. These specs are image-generation-ready (layout sketch + tiers +
> visual cues + prompt).

## Design language

The UI was rebuilt on a **shadcn-vue** design system in a **Linear-style minimal** direction:
refined cool neutrals, **one** confident accent used sparingly, tight radii, hairline borders over
heavy shadows, crisp dense type, **no gradients**.

- **Design system:** **shadcn-vue** (Reka UI primitives + Tailwind v3). The components are
  copy-paste and **owned** under `src/components/ui/` — Button, Input, Textarea, Dialog,
  DropdownMenu, Select, Badge, Avatar, Skeleton, Tabs, Tooltip, Separator — plus an inline
  `icons/WhatsappIcon.vue`. A `cn()` helper (`clsx` + `tailwind-merge`) merges classes; colors are
  **HSL CSS variables** (`src/style.css`) mapped to semantic Tailwind tokens in `tailwind.config.js`.
- **Brand:** XChats. **Solid** indigo "X" mark (flat — the old indigo→violet gradient is gone).
- **Color (semantic tokens, one accent):** the single accent is **`--primary` indigo `#4F46E5`** —
  buttons, active states, focus rings (`--ring`), links, unread badges. **WhatsApp-green `#22C55E`**
  is retained **only** for: outbound message bubbles, the WhatsApp glyph, and the "connected" status
  dot — **never on buttons** (the send / approve buttons are **primary indigo**, not green; this
  removes the old two-accent "candy" look). Surfaces: cool near-white `--background`, white `--card`,
  ink `--foreground`, secondary text `--muted-foreground` (`#64748B`), hairline `--border`.
  Inbound bubble = `--card` + hairline border; outbound bubble = green.
- **Dark mode:** class-based (`.dark` on `<html>`) with a deep cool-charcoal palette and a lifted
  indigo. **Default light**, persisted via `localStorage('theme')`; no toggle UI in v1 (every screen
  is themed with semantic tokens so dark works out of the box).
- **Icons:** **lucide-vue-next** line icons everywhere (spinners = `<LoaderCircle class="animate-spin"/>`),
  plus the inline `WhatsappIcon` SVG (lucide has no brand glyph). **FontAwesome (CDN) removed.**
- **Avatars:** **initials on a colored circle** (Reka `Avatar`) — we do **not** fetch WhatsApp profile
  pictures in v1 (the `@lid`-keyed `contacts.update` events that carry them are ignored).
- **Shape:** tight radii (`--radius: 0.5rem`; `rounded-lg`/`rounded-xl`), hairline `--border`
  dividers, shadows reserved for popovers / dialogs / the FAB. **Russian UI** (v1 is Russian-only —
  no language switcher; i18n isn't built).

## The motto — three visibility tiers (apply to every page)

> *"Every piece of information is on the page, but one click away; only basic, core data is shown."*

- **Tier 1 — Core:** always visible.
- **Tier 2 — One click/hover:** an affordance (chevron, "⋯", "Подробнее", tab, hover) — not the data.
- **Tier 3 — Elsewhere:** full detail in its own drawer.

---

## 1. Login — `/login`

Split screen: **flat** dark brand panel (left) + white form (right). The old blurred gradient "orbs"
are removed — the panel is a solid dark slate surface.

```
┌───────────────────────┬───────────────────────────┐
│  ▣ XChats              │   Вход в аккаунт           │
│  Командный инбокс      │   Email   [✉ __________]   │
│  и ИИ-ассистент        │   Пароль  [🔒 _________]   │
│  • Единый инбокс       │   [      Войти      ]      │
│  • ИИ-ответы           │                            │
│  • Безопасность        │   Нет аккаунта? — админ    │
└───────────────────────┴───────────────────────────┘
```

- **Core:** Email, Пароль, **Войти**. Static footer "Нет аккаунта? Свяжитесь с администратором"
  (users are created by an admin via the Users API — there is no self-signup).
- **Visual cues (new):** flat **slate-900** brand panel; **solid indigo** "X" logo tile; three
  feature rows with lucide icons in subtle `white/5` tiles (`WhatsappIcon` green, `WandSparkles`,
  `ShieldCheck`); shadcn **Input** with a leading lucide icon (`Mail` / `Lock`); full-width **primary
  Button** with a `LoaderCircle` spinner while submitting; errors as `CircleAlert` + `text-destructive`.
- **Removed (not backed):** ~~Войти через Google~~ (no OAuth), ~~Запомнить меня~~, ~~Забыли пароль?~~
  (no password-reset endpoint), ~~language switcher~~ (no i18n in v1).
- **▸ Backed by:** `POST /auth/login {email,password}` → session cookie. That's the whole page.

**Image prompt:** *Clean modern SaaS login, split layout, Linear-minimal. Left half a flat dark slate
(#0F172A, NO gradient, NO glow orbs): a small solid-indigo rounded square "X" logo next to "XChats",
headline "Командный инбокс и ИИ-ассистент", three small line-icon feature rows (WhatsApp glyph in
green, sparkles, shield). Right half white, centered: title "Вход в аккаунт", an Email input and a
Пароль input each with a small grey leading line-icon, a single full-width INDIGO "Войти" button, and
small grey footer "Нет аккаунта? Свяжитесь с администратором". NO social-login, NO "remember me", NO
"forgot password", NO language selector. Inter font, tight 8px corners, hairline borders, generous
whitespace.*

---

## 2. Chatboard — `/` (the main page)

Four regions on a strict diet. Multi-account **is** supported: every chat avatar carries a small
green WhatsApp badge, and with more than one connected number a per-chat account label and an account
filter appear.

```
┌──┬────────────────┬──────────────────────────┬───────────────────┐
│N │ Chat list       │ Chat view                 │ Assistant panel   │
│a │ search + filter │ header · timeline         │ 1–3 AI options    │
│v │ chat rows       │ composer                  │ contact summary   │
└──┴────────────────┴──────────────────────────┴───────────────────┘
```

### Nav rail (icon-only, far left, `w-[68px]`, flat dark slate)
- **Core:** **two** nav icons — **Inbox** (`/`, active = solid indigo) and **WhatsApp accounts**
  (`/accounts`, the `WhatsappIcon`); a round user **Avatar** pinned at the bottom. Icons carry
  **Tooltips** ("Инбокс", "Номера WhatsApp").
- **One-click:** the avatar is a **DropdownMenu** trigger → menu with **name · email · org** and a
  destructive **Выйти** item.
- The rail is flat `slate-900` (the old indigo gradient + glow is gone). Instances Maintenance is not
  a rail destination — it's reached from the Accounts page.
- **▸ Backed by:** `GET /me` (name/email/org), `POST /auth/logout`.

### Chat list (pane 1) — `bg-card`, `w-[340px]`, rows capped at **two lines**
- **Core per row:** **initials Avatar** with a small green **WhatsApp badge**, contact name, one-line
  last-message preview, time, **unread Badge** (primary). The active row is tinted (`bg-primary/10`)
  with a primary left indicator bar.
- **One-click:** a shadcn **Input** search on top; a segmented filter **Мои · Неназначенные · Все**
  built on **Tabs**; and — **only with >1 connected number** — an account-filter **Select** ("Все
  номера" + each number) and a per-row account label.
- **Compose:** a header pencil button (`SquarePen`) and a **primary FAB** (`size=icon`, `Plus`) both
  open the New-message dialog. (The old green FAB is now primary indigo.)
- **▸ Backed by:** `GET /chats {q?, assignee=me|unassigned, wa_account_id?, page}` →
  `Chat{contact.display_name, last_message_preview, last_message_at, unread_count, wa_account_id}`;
  number list from `GET /whatsapp-accounts`; live via SSE `chat.*`, `message.created`.

### Chat view (pane 2) — `bg-background` canvas
- **Core:** message timeline (inbound left = `bg-card` + hairline; outbound right = **green** bubble),
  **delivery/read ticks** (lucide `Check` / `CheckCheck` / `Clock` / `TriangleAlert`, colored for the
  green bubble), composer.
- **Composer:** ghost **attach** icon button (`Paperclip`) + autosize **Textarea** (`v-autosize`,
  borderless inside a `bg-muted` framed wrapper with a focus ring) + a **primary "Отправить"** Button
  (`Send` icon — **not green**; green is reserved for bubbles). Attaching N files sends N separate
  messages (`POST …/media` upload + `media_ids[]`).
- **Header:** contact **Avatar** (with a green connected dot) + **name + phone**; action **outline
  Buttons** "Назначить" (`UserPlus`) and "Решить" (`CircleCheck`, green icon); and an "Ещё"
  **DropdownMenu** (`⋯`).
- **UI stubs (not backed yet):** "Назначить" maps to the assign endpoint but the user-picker isn't
  wired in v1; "Решить" has no status endpoint; the "Ещё" items ("Профиль контакта", "Отметить
  непрочитанным") are placeholders pending endpoints. They're styled affordances, intentionally inert.
- **Removed (not backed):** ~~online/last-seen presence~~, ~~profile-picture avatars~~ (initials only).
- **▸ Backed by:** `GET /chats/{id}/messages` → `Message{direction, content, media(urls), status,
  timestamp}`; `POST /chats/{id}/messages {text?, media_ids?[]}` (+ `POST /media` upload); read-on-open
  via `POST /chats/{id}/read`; ticks from `message.status` over SSE. *(Assign picker / resolve await
  their endpoints.)*

### Assistant panel (pane 3) — `bg-card`, `w-[340px]` — **1–3 reply options, text + media**
The product's wedge.

- **Header:** a **solid indigo** tile (`WandSparkles`) + "ИИ-помощник" (the old indigo→violet
  gradient tile is gone); a ghost regenerate button (`RotateCw`, spins while generating).
- **Generating state:** a **Skeleton** shimmer card ("ИИ готовит ответ…").
- **Option cards:** "Рекомендуемый ответ" / "Вариант N", each compact:
  - **Text** — editable inline in a **Textarea** (`v-autosize`).
  - **Confidence** — a **Badge** (`NN%`, tinted green/amber/rose by score).
  - **Media chips** — the option's suggested file(s) with a lucide type icon (`Image`/`Mic`/`Film`/
    `FileText`) and an **×** to **detach** (attaching a *new* file needs the asset-catalog UI —
    **deferred**; v1 only detaches).
  - **Actions:** **primary "Отправить"** (`Send` — **not green**) approves & sends; a `PenLine` icon
    button ("В поле ввода") loads text into the composer; an outline `RotateCw` regenerates. Picking
    one option supersedes the others; a bottom **"Отклонить"** clears the set.
- **Below:** a collapsed **contact mini-profile** — "Контакт", name, phone, **2–3 attributes**.
- **▸ Backed by:** `POST /chats/{id}/ai-drafts` (→ 1–3 `AiDraft`), `GET …/ai-drafts`,
  `POST /ai-drafts/{id}/approve {edited_text?, media_ids?}`; fields `AiDraft{ordinal, draft_text,
  media[], confidence, escalate, escalation_reason}`; contact summary from `Chat.contact`; live via
  SSE `ai_draft.created`/`ai_draft.updated`.

**Image prompt:** *Modern team-inbox web app (Linear-minimal, shadcn-style), four columns, light
theme, Inter font, Russian UI, NO gradients, tight 8px corners, hairline borders, one indigo accent.
Far-left slim FLAT dark-slate icon rail with TWO line icons (an active indigo inbox, a WhatsApp glyph)
and a round user avatar at the bottom. Column 2 "Чаты" on white: a search input, a segmented filter
"Мои · Неназначенные · Все", and chat rows — a colored circle with white initials (NOT a photo) and a
tiny green WhatsApp badge, bold name, one-line grey preview, time, a small indigo unread count badge;
the selected row faintly indigo-tinted with a thin indigo left bar. Column 3 chat view on a cool
near-white canvas: header with contact avatar, name and phone and two small outline buttons "Назначить"
/ "Решить" plus a "⋯" menu; a thread with white hairline-bordered inbound bubbles left and green
(#22C55E) outbound bubbles right, blue double read-ticks; bottom composer with a paperclip icon, a
text field, and an INDIGO "Отправить" button (not green). Column 4 panel "ИИ-помощник" with a solid
indigo sparkles tile: two suggestion cards "Рекомендуемый ответ" / "Вариант 2", each with editable
text, a small green confidence badge, one document chip with an ×, an INDIGO "Отправить" button and a
small pencil icon; below, a collapsed "Контакт" block with name, phone, two attributes. Soft shadows
only on menus.*

---

## 3. WhatsApp Accounts — `/accounts` *(implemented)*

Connect and manage numbers. Two columns: account manager (left) + an always-visible "how to connect"
instructions panel (right, `lg+`).

```
┌──┬──────────────────────────────────────┬──────────────────┐
│N │ WhatsApp аккаунты   [Обслуж.] [＋ ... ]│  Как подключить  │
│a │ [✓ Подключено][▣ QR][⚡ Не подкл.]     │  1 … 2 … 3 … 4    │
│v │ ▢ карточки номеров (статус-бейдж, ⟳ 🗑)│  Подсказки       │
└──┴──────────────────────────────────────┴──────────────────┘
```

- **Core:** three stat cards — **Подключено** (`CircleCheck`, green tile), **Требуют QR** (`QrCode`,
  amber), **Не подключено** (`Unplug`, red); then a grid of **account cards** — a **solid green**
  WhatsApp tile (`WhatsappIcon`; the old green gradient is gone) with a colored initials corner badge,
  display name, phone, instance name (`Server`), and a **status Badge** with a colored dot
  (Подключён=green, Нужен QR=amber, Подключение…=sky, Отключён=grey, Ошибка=red).
- **One-click:** top-right **primary "Подключить аккаунт"** (`Plus`) and an outline "Обслуживание
  инстансов" link (`Wrench`); per card, a ghost **reconnect** (`RotateCw`, when not connected) and a
  ghost **delete** (`Trash2`, with a `window.confirm`).
- **Add / reconnect flow:** a **Reka Dialog** — name the instance → poll the **QR** (rendered PNG,
  pairing code, auto-refresh) → on `connected`, a green `CircleCheck` "Номер подключён!" and close.
  The dialog's create/QR button is **primary** (not green); the WA glyph tile is `bg-wa/10`.
- **▸ Backed by:** `GET /whatsapp-accounts` (→ `WhatsAppAccount{instance_name, display_name,
  connection_status, phone_number}`), `POST /whatsapp-accounts {display_name, instance_name}`,
  `GET /whatsapp-accounts/qr?instance=`, `POST …/{id}/reconnect`, `DELETE …/{id}`. Status label/tone
  via `connStatus()` (`format.ts`) → a `{label, tone}` discriminant the view maps to Badge/dot classes.

**Image prompt:** *WhatsApp accounts manager in the XChats shell (flat dark slate rail), Linear-minimal,
light theme, NO gradients, hairline borders, tight corners. Header "WhatsApp аккаунты" with a primary
indigo "Подключить аккаунт" button and an outline "Обслуживание инстансов" link. A row of three stat
cards (green check "Подключено", amber QR "Требуют QR", red unplug "Не подключено"), then a grid of
account cards — each a SOLID green rounded square with a white WhatsApp glyph and a small colored
initials badge, the display name, phone, instance name, and a soft status pill with a colored dot
(green "Подключён"); per card a small refresh and trash icon button. Right side an "Как подключить"
panel with numbered indigo step bullets and tips. Inter font, soft shadow only on hover.*

---

## 4. Instances Maintenance — `/instances` *(implemented)*

The "broom": every raw Evolution instance, with **managed** (we hold an account for it) and **stale**
(not connected, old) flags. Managed instances can't be deleted here — clean the account instead.
Reached from the Accounts header.

```
┌──┬──────────────────────────────────────────────────────┐
│N │ ← Обслуживание инстансов        [🗑 Удалить устаревшие]│
│a │ ┌ Инстансы Evolution ─────────────────────── N всего ┐│
│  │ │ ▤ name  [наш][устарел]   статус-бейдж        🗑     ││
└──┴─┴────────────────────────────────────────────────────┘
```

- **Core:** a back **Button** (`ArrowLeft`) + title; a list **card** of instances — a `Server` tile,
  the instance **name** (mono), optional **Badges** "наш" (primary tint) / "устарел" (amber), created
  time + owner, a **status Badge** (same `connStatus` tones as Accounts), and a ghost **delete**
  (`Trash2`, disabled + tooltip for managed rows).
- **One-click:** a **destructive Button** "Удалить устаревшие (N)" (shown only when stale instances
  exist) bulk-deletes the unmanaged stale ones (with `window.confirm`).
- **Errors:** an inline `bg-destructive/10` banner (`CircleAlert`).
- **▸ Backed by:** `GET /whatsapp-instances` (→ `EvolutionInstance{name, connection_status, owner_jid,
  phone_number, created_at, managed, stale}`), `DELETE /whatsapp-instances/{name}`.

**Image prompt:** *An "instances maintenance" admin page in the XChats shell (flat dark slate rail),
Linear-minimal, light theme, NO gradients, hairline borders. Header with a back-arrow icon button,
title "Обслуживание инстансов", and a red "Удалить устаревшие (3)" button. A single bordered card
"Инстансы Evolution" listing rows: a grey server-icon tile, a monospace instance name with small
"наш" (indigo) and "устарел" (amber) pills, a created-date line, a soft status pill with a colored
dot, and a small red trash icon button at the right (greyed-out for managed rows). Clean, dense,
Inter font.*

---

## 5. Contacts — `/contacts` *(deferred; spec for later)*

List + detail drawer.
- **Core:** searchable table — initials Avatar, name, phone, last-contact time.
- **One-click (drawer):** identities (phone / `@lid` / push name), attributes, the contact's chats.
- **▸ Backed by:** `GET /contacts {q?, page}`, `GET /contacts/{id}` (→ `Contact{display_name,
  phone_number, phone_jid, lid_jid, push_name, attributes}` + recent chats).

**Image prompt:** *Contacts page in the XChats shell (flat dark slate rail), Linear-minimal, light
theme, NO gradients, hairline borders, one indigo accent. Center: a table "Контакты" with a search
input and columns initials-avatar, name, phone, "последний контакт". An open right detail drawer
showing a contact's initials avatar, name, phone, a few key-value attributes, an "Идентификаторы" list
(phone, @lid, push name), and that contact's chats. Inter font, tight corners.*

---

## 6. AI Assistant — `/assistant` *(deferred; v1 seeds the KB from SQL/markdown, no UI)*

- **Core:** left sub-nav (Персона · Знания · Цены · Медиа-ассеты · Плейграунд) + the editor.
- **One-click:** **Опубликовать** (eval-gated); a **Плейграунд** that shows the same **1–3 option
  cards** as the Chatboard assistant panel.
- **▸ Backed by (deferred — Phase 4B):** `GET|PUT /assistant/config`, `POST /assistant/publish`,
  `POST /assistant/playground`; data in `ai_snapshots/ai_topics/ai_assets/ai_values`.

**Image prompt:** *AI assistant config page in the XChats shell (flat dark slate rail), Linear-minimal,
light theme, NO gradients, hairline borders, one indigo accent. Left sub-nav (Персона, Знания, Цены,
Медиа-ассеты, Плейграунд). Center "Персона" editor: text-area fields and a primary indigo
"Опубликовать" button. Right "Плейграунд" strip with a sample-chat input and 1–3 generated suggestion
cards (indigo "Отправить" buttons, small green confidence badges). Inter font, tight corners.*

---

## 7. Settings — `/settings` *(deferred)*

Tabbed: **Организация · Пользователи** (only these two — no "General"/theme/language tab, nothing
backs it).
- **Организация:** name, auto-response mode (NEVER / CONFIGURE_TIME / ALWAYS) + a time window
  (**UTC**, shown with a hint). **▸ Backed by:** `GET|PATCH /organization`.
- **Пользователи:** table of users; **＋ Добавить пользователя** (email + password → joins the org).
  **▸ Backed by:** `GET /users`, `POST /users {email,password,name?}`.

**Image prompt:** *Settings page in the XChats shell (flat dark slate rail), Linear-minimal, light
theme, NO gradients, hairline borders, one indigo accent. Two tabs "Организация · Пользователи".
Active "Организация": fields for org name, an auto-response mode segmented control
(Никогда/По расписанию/Всегда), a start–end time window with a small "UTC" hint, and a primary indigo
"Сохранить" button. Clean light form, Inter font, tight corners.*

---

## Out of scope (v1)

Reports/analytics, campaigns, SLA, macros, billing, granular permissions, self-signup, OAuth,
password reset, presence, i18n (per `1-concept.md`). Not shown, not stubbed.
