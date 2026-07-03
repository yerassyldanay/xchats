# UI Pages & States

The frontend is a single **Vue 3 SPA** (see `2-architecture.md`). It talks only to the backend
(`/api` + SSE). **v1 ships six routed pages** — **Login**, **Chatboard**, **WhatsApp Accounts**,
**Instances Maintenance**, **Конструктор базы знаний** (`/playground`), **Редактор базы знаний**
(`/knowledge-base`). The first four are **live**; the two Knowledge-Base pages are **backend-ready, UI
in progress**. **Contacts** and **Settings** are **deferred** (designed here, not yet routed).

> **This doc is organized by *cases* (states), not just pages.** Each page renders differently depending
> on the data — empty vs populated, a background job running, a draft spanning several tables, a
> multi-language entity, an error. Every **case is a separate screen to generate an image for**: generate
> it, review it visually, keep the description. Drop your example renders into **[`./ui/`](ui/)** using the
> suggested filenames.

> **Two hard rules.**
> 1. **Style is constant; only content varies.** Every case uses the *same* design language (below). When
>    generating images, reuse the **STYLE preamble** verbatim and change only the **CASE body**. "Stick to
>    the style, vary what's displayed."
> 2. **Every element is backed.** Nothing on a screen may show data we can't retrieve or trigger an action
>    we don't expose. Each case lists **▸ Backed by** — the exact endpoint(s)/field(s) from
>    `7.1-endpoints.md` / `9-database-schema.md`. Anything not backed is either **removed** or called out
>    as an inert **UI stub**.

> **KB storage model (plan 15).** The draft KB is the **`kbd_draft` jsonb blob** (one per org); the brain
> reads the live **`ai_` tables**. The UI never touches tables — it reads one merged **`DraftView`** where
> each entity is **LIVE** (already in the live KB) or **«Черновик»** (pending, not yet approved).
> Approving **materializes** a pending entity into the live KB. Consistent with `9`/`12`/`15`.

---

## Design language — the constant (never changes across cases)

Rebuilt on **shadcn-vue** in a **Linear-style minimal** direction: refined cool neutrals, **one**
confident accent used sparingly, tight radii, hairline borders over heavy shadows, crisp dense type,
**no gradients**.

- **Design system:** **shadcn-vue** (Reka UI primitives + Tailwind v3), components owned under
  `src/components/ui/` (Button, Input, Textarea, Dialog, DropdownMenu, Select, Badge, Avatar, Skeleton,
  Tabs, Tooltip, Separator) + inline `icons/WhatsappIcon.vue`. Colors are **HSL CSS variables**.
- **Brand:** XChats — a **solid** indigo "X" mark (flat, no gradient).
- **One accent — `--primary` indigo `#4F46E5`:** buttons, active nav, focus rings, links, unread badges.
  **WhatsApp-green `#22C55E`** is used **only** for outbound message bubbles, the WhatsApp glyph, and the
  "connected" status dot — **never on buttons** (send/approve buttons are indigo).
- **Surfaces:** cool near-white `--background` `#F8FAFC`, white `--card`, ink `--foreground` `#0F172A`,
  secondary text `--muted-foreground` `#64748B`, hairline `--border`. Inbound bubble = white + hairline;
  outbound bubble = green.
- **Nav rail:** flat **slate-900 `#0F172A`** icon rail (no gradient/glow).
- **Dark mode:** class-based (`.dark`), deep cool-charcoal + lifted indigo. **Default light**; no toggle
  UI in v1 (every screen is themed via semantic tokens, so dark works out of the box).
- **Icons:** **lucide-vue-next** line icons (spinners = `LoaderCircle` spin) + the inline `WhatsappIcon`.
- **Avatars:** **initials on a colored circle** — we do **not** fetch WhatsApp profile pictures in v1.
- **Shape:** tight radii (`--radius: 0.5rem`), hairline dividers, shadows reserved for popovers / dialogs
  / the FAB. **Russian UI** (v1 is Russian-only — no language switcher; i18n isn't built).

### The three visibility tiers (apply to every page)

> *"Every piece of information is on the page, but one click away; only basic, core data is shown."*

- **Tier 1 — Core:** always visible.
- **Tier 2 — One click/hover:** an affordance (chevron, "⋯", "Подробнее", tab, hover) — not the data.
- **Tier 3 — Elsewhere:** full detail in its own drawer.

---

## Generating page images — the shared recipe

**Final prompt = `STYLE` preamble + (for non-Login pages) the `APP SHELL` block + the case's `Image
prompt` body.** Keep STYLE identical every time; that is what makes the 30 screens read as one product.

**STYLE preamble** (prepend to every case):
```
High-fidelity flat UI mockup, straight-on, no perspective, no device frame. Product: "XChats", a
WhatsApp team-inbox + AI-assistant web app. Style: Linear-minimal, shadcn/Tailwind. LIGHT theme: cool
near-white background #F8FAFC, white cards, ink text #0F172A, grey secondary text #64748B. ONE accent —
indigo #4F46E5 — for buttons, active nav, focus rings, links, unread badges. WhatsApp green #22C55E ONLY
for outbound chat bubbles, the WhatsApp glyph, and the "connected" status dot — never on buttons.
Hairline 1px borders, tight 8px rounded corners, NO gradients, NO heavy shadows (soft shadow only on
menus/dialogs). Inter font, crisp dense typography. Russian-language UI. Avatars = white initials on a
colored circle (never photos). Desktop web, landscape 3:2.
```

**APP SHELL block** (prepend for every page **except Login**):
```
APP SHELL: a slim flat dark-slate (#0F172A) vertical icon rail on the far left with four line icons
(inbox, WhatsApp glyph, blocks/bot, library) and a round user-avatar pinned at the bottom; the active
icon is indigo.
```

**Tips**
- **Landscape 3:2 (~1536×1024). One case per image.** Never combine two states in one render.
- **Feed the matching `./ui/*.png` as a reference image** where one exists — it locks the style far
  better than text. For cases without a reference, generate text-only, then reuse your best output as the
  reference for its sibling cases (e.g. Playground-empty → reference for Playground-jobs).
- **Cyrillic may partially garble** — you're capturing *layout + style*, not final copy. Exact Russian
  labels are in each prompt to nudge it; real text is set in code.
- **Order:** render Login + Chatboard first to lock the look, then reference them for the rest.
- **Dark variant (optional):** append *"DARK theme: deep cool-charcoal #0B0F17 surfaces, lifted indigo
  accent, same layout"* to any prompt.

**Reference-image filenames** live in [`./ui/`](ui/). Existing renders: `auth.png`, `chatboard.png`,
`whatsapp-accounts.png`, `contacts.png`, `settings.png`, `ai-playground.png`, `ai-knowledge-base.png`.
Each case below names the file to add.

---

## 1. Login — `/login`

Split screen: **flat** dark brand panel (left) + white form (right). No self-signup (admin creates users).

```
┌───────────────────────┬───────────────────────────┐
│  ▣ XChats              │   Вход в аккаунт           │
│  Командный инбокс      │   Email   [✉ __________]   │
│  и ИИ-ассистент        │   Пароль  [🔒 _________]   │
│  • Единый инбокс       │   [      Войти      ]      │
│  • ИИ-ответы           │   Нет аккаунта? — админ    │
└───────────────────────┴───────────────────────────┘
```

- **Core (Tier 1):** Email, Пароль, full-width **primary "Войти"**. Static footer "Нет аккаунта?
  Свяжитесь с администратором".
- **Removed (not backed):** social login, "Запомнить меня", "Забыли пароль?", language switcher.
- **▸ Backed by:** `POST /auth/login {email,password}` → session cookie + `{user, organization}`.

**Cases**

| Case | File | What differs |
|---|---|---|
| **1a · Default** | `ui/auth.png` *(exists)* | clean empty form |
| **1b · Error** | `ui/login-error.png` | invalid-credentials inline error under the button |

- **1a Image prompt:** *Split-screen login. LEFT half flat dark slate (#0F172A, no gradient): a small
  solid-indigo rounded-square "X" beside "XChats", headline "Командный инбокс и ИИ-ассистент", three
  small line-icon feature rows (green WhatsApp glyph, sparkles, shield). RIGHT half white, centered:
  title "Вход в аккаунт", an Email field and a Пароль field each with a small grey leading icon, one
  full-width INDIGO "Войти" button, small grey footer "Нет аккаунта? Свяжитесь с администратором". No
  social login, no remember-me, no forgot-password, no language selector.*
- **1b Image prompt:** *…same login, but the Пароль field has a thin red (#EF4444) border and a small red
  "Неверный email или пароль" message with a circle-alert icon sits between the fields and the "Войти"
  button; the button is enabled.*

---

## 2. Chatboard — `/` (the main page)

Four regions on a strict diet: nav rail · chat list · chat view · assistant panel. The assistant panel
is the product's wedge (1–3 reply drafts, human-approved).

```
┌──┬────────────────┬──────────────────────────┬───────────────────┐
│N │ Chat list       │ Chat view                 │ Assistant panel   │
│a │ search + filter │ header · timeline         │ 1–3 AI options    │
│v │ chat rows       │ composer                  │ contact summary   │
└──┴────────────────┴──────────────────────────┴───────────────────┘
```

**Persistent regions** (Tier 1, present in most cases):
- **Chat list** (`w-[340px]`): search **Input**, segmented filter **Мои · Неназначенные · Все** (Tabs),
  rows capped at two lines — initials **Avatar** + green WhatsApp badge, name, one-line preview, time,
  indigo **unread Badge**; active row indigo-tinted with a left bar. A `SquarePen` header button + a
  **primary FAB** (`Plus`) open the new-message dialog.
  **▸ Backed by:** `GET /chats {status?,assignee?,wa_account_id?,q?,page}` → `Chat{contact.display_name,
  last_message_preview, last_message_at, unread_count, wa_account_id}`; SSE `chat.*`, `message.created`.
- **Chat view:** header (contact **Avatar** + name + phone; outline "Назначить"/"Решить"; "⋯" menu),
  message timeline (inbound left = white+hairline, outbound right = **green**), delivery ticks
  (`Check`/`CheckCheck`/`Clock`/`TriangleAlert`), composer (`Paperclip` + autosize **Textarea** +
  **primary "Отправить"**). **Stubs (not backed):** "Решить" (no status endpoint), "⋯" items.
  **▸ Backed by:** `GET /chats/{id}/messages` → `Message{direction,content,media,status,timestamp}`;
  `POST /chats/{id}/messages {text?,media_ids?}` (+ `POST /media`); `POST /chats/{id}/read`; ticks via SSE
  `message.updated`.
- **Assistant panel** (`w-[340px]`): indigo `WandSparkles` tile + "ИИ-помощник"; below, a collapsed
  contact mini-profile (name, phone, 2–3 attributes from `Chat.contact.attributes`).

**Cases**

| Case | File | What differs |
|---|---|---|
| **2a · Empty inbox** | `ui/chatboard-empty.png` | no chats yet; empty-state in the list + a blank canvas |
| **2b · AI options** *(hero)* | `ui/chatboard.png` *(exists)* | chat selected, thread, **1–3 AI draft cards** |
| **2c · AI generating** | `ui/chatboard-generating.png` | assistant panel shows a **Skeleton shimmer** "ИИ готовит ответ…" |
| **2d · AI escalation** | `ui/chatboard-escalation.png` | brain has no KB answer → an "ответьте вручную" card |
| **2e · Multi-account** | `ui/chatboard-multiaccount.png` | account-filter **Select** + per-row account labels appear |

- **2a** — Empty inbox. Chat list shows a soft empty-state ("Пока нет чатов — подключите номер и напишите
  клиенту"); the chat view is a centered placeholder; the assistant panel is idle.
  **▸ Backed by:** `GET /chats` → empty `items`. If **no account is connected**, the placeholder links to
  `/accounts` (`GET /whatsapp-accounts` empty).
- **2b** — The hero. A chat is open with a real thread; the assistant panel shows **"Рекомендуемый
  ответ" + "Вариант 2"** cards: editable **Textarea** text, a small **confidence Badge** (green/amber/rose
  by score), a media chip with an **×** (detach only), a **primary "Отправить"**, a `PenLine`
  ("В поле ввода") and a `RotateCw` (regenerate). **▸ Backed by:** `GET /chats/{id}/ai-drafts` →
  `AiDraft{ordinal, draft_text, media[], confidence, escalate, status}`; `POST …/ai-drafts/{id}/approve
  {chosen_ordinal, edited_text?, media_ids?}`; SSE `ai_draft.created`/`ai_draft.updated`.
- **2c** — Generating. The panel is a single **Skeleton** card; the regenerate icon spins. Triggered by
  "Сгенерировать ответ". **▸ Backed by:** `POST /chats/{id}/ai-drafts` → `202`; SSE `ai_draft.generating`
  (disables the button for all viewers) until `ai_draft.created`.
- **2d** — Escalation. One card: a muted "ИИ не нашёл ответа в базе знаний — ответьте вручную" with the
  `escalation_reason`, no send action. **▸ Backed by:** `AiDraft{escalate:true, escalation_reason}`.
- **2e** — Multi-account. With **>1 connected number**, the list gains an account-filter **Select** ("Все
  номера" + each number) and each row shows a small account label; chat avatars keep the green WA badge.
  **▸ Backed by:** `GET /whatsapp-accounts` (>1 item) → `Chat.wa_account_id`.

- **Base Image prompt (2b hero):** *Four-column team inbox. Col 1 the dark icon rail. Col 2 "Чаты": a
  search input, a segmented filter "Мои · Неназначенные · Все", chat rows — colored circle with white
  initials + tiny green WhatsApp badge, bold name, one-line grey preview, time, small indigo unread
  badge; selected row faintly indigo-tinted with a thin indigo left bar. Col 3 chat view: header with
  contact avatar, name, phone, two small outline buttons "Назначить"/"Решить" and a "⋯" menu; a thread of
  white hairline inbound bubbles (left) and green #22C55E outbound bubbles (right) with blue double
  read-ticks; bottom composer = paperclip + text field + INDIGO "Отправить". Col 4 "ИИ-помощник" with a
  solid-indigo sparkles tile: two suggestion cards "Рекомендуемый ответ"/"Вариант 2", each with editable
  text, a small green confidence badge, a document chip with an ×, an INDIGO "Отправить" and a pencil
  icon; below, a collapsed "Контакт" block with name, phone, two attributes.*
- **Case deltas** (keep everything else identical): **2a** empty — replace the chat rows with a soft
  "Пока нет чатов" empty-state and the chat view + AI panel with light placeholders. **2c** — replace the
  two AI cards with one grey shimmer skeleton card labelled "ИИ готовит ответ…" and a spinning refresh
  icon. **2d** — replace the AI cards with one muted card "ИИ не нашёл ответа в базе знаний — ответьте
  вручную" and no send button. **2e** — add an account-filter dropdown "Все номера" above the chat rows
  and a tiny grey account label on each row.

---

## 3. WhatsApp Accounts — `/accounts` *(live)*

Connect and manage numbers. Account manager (left) + an always-visible "how to connect" panel (right, `lg+`).

```
┌──┬──────────────────────────────────────┬──────────────────┐
│N │ WhatsApp аккаунты   [Обслуж.] [＋ ... ]│  Как подключить  │
│a │ [✓ Подключено][▣ QR][⚡ Не подкл.]     │  1 … 2 … 3 … 4    │
│v │ ▢ карточки номеров (статус-бейдж, ⟳ 🗑)│  Подсказки       │
└──┴──────────────────────────────────────┴──────────────────┘
```

- **Core:** three stat cards (**Подключено** green, **Требуют QR** amber, **Не подключено** red); a grid
  of **account cards** — solid-green `WhatsappIcon` tile + initials badge, display name, phone, instance
  name, **status Badge** + colored dot (Подключён green / Нужен QR amber / Подключение… sky / Отключён
  grey / Ошибка red). Top-right **primary "Подключить аккаунт"** + outline "Обслуживание инстансов".
- **▸ Backed by:** `GET /whatsapp-accounts` → `WhatsAppAccount{instance_name, display_name,
  connection_status, phone_number, assigned}`; `POST /whatsapp-accounts`; `GET …/{id}/qr`; `POST
  …/{id}/reconnect`; `DELETE …/{id}`; SSE `wa_account.status_changed`.

**Cases**

| Case | File | What differs |
|---|---|---|
| **3a · Empty** | `ui/accounts-empty.png` | no numbers yet; stat cards read 0; onboarding "Подключить первый номер" |
| **3b · Populated** | `ui/whatsapp-accounts.png` *(exists)* | mixed statuses across the grid |
| **3c · QR dialog** | `ui/accounts-qr.png` | the add/reconnect **Dialog** showing a live **QR** + pairing code |
| **3d · Connected** | `ui/accounts-connected.png` | the dialog flips to a green `CircleCheck` "Номер подключён!" |

- **3a Image prompt:** *Accounts page, header "WhatsApp аккаунты" with an indigo "Подключить аккаунт"
  button; three stat cards all reading 0 (Подключено 0, Требуют QR 0, Не подключено 0); a large centered
  empty-state card with a soft WhatsApp-in-a-circle line illustration, "Нет подключённых номеров" and a
  primary "Подключить первый номер"; the right "Как подключить" panel with numbered indigo steps.*
- **3b Image prompt:** *…header with primary "Подключить аккаунт" + outline "Обслуживание инстансов";
  three stat cards (green "Подключено", amber "Требуют QR", red "Не подключено"); a grid of account cards
  — each a solid-green rounded square with a white WhatsApp glyph + small colored initials badge, display
  name, phone, instance name, and a soft status pill with a colored dot (green "Подключён", amber "Нужен
  QR", grey "Отключён"); per card a small refresh and trash icon; the right "Как подключить" panel.*
- **3c Image prompt:** *…same page dimmed behind a centered modal dialog: title "Подключение номера", a
  large black-and-white QR code, a monospace pairing code "WZYE-3K7Q", grey helper text "Отсканируйте QR
  в WhatsApp → Связанные устройства", a small spinner "Ожидание подключения…", and an INDIGO button.*
- **3d Image prompt:** *…same dialog but the QR is replaced by a large green circle-check, heading "Номер
  подключён!", the phone number, and an indigo "Готово" button.*

---

## 4. Instances Maintenance — `/instances` *(live)*

The "broom": every raw Evolution instance, flagged **managed** (we hold an account) and **stale** (not
connected, old). Managed instances can't be deleted here. Reached from the Accounts header.

- **Core:** back **Button** + title; a list card of instances — `Server` tile, mono **name**, optional
  **Badges** "наш" (indigo) / "устарел" (amber), created time + owner, **status Badge**, ghost **delete**
  (disabled + tooltip for managed rows). A **destructive "Удалить устаревшие (N)"** appears only when
  stale unmanaged instances exist.
- **▸ Backed by:** `GET /whatsapp-instances` → `EvolutionInstance{name, connection_status, owner_jid,
  phone_number, created_at, managed, stale}`; `DELETE /whatsapp-instances/{name}`.

**Cases**

| Case | File | What differs |
|---|---|---|
| **4a · Has stale** | `ui/instances-stale.png` | mix of наш/устарел rows; red "Удалить устаревшие (3)" shown |
| **4b · Clean** | `ui/instances-clean.png` | all managed/healthy; the bulk-delete button is **hidden** |

- **4a Image prompt:** *Maintenance page. Header: a back-arrow icon button, title "Обслуживание
  инстансов", and a red "Удалить устаревшие (3)" button. One bordered card "Инстансы Evolution" listing
  rows: a grey server tile, a monospace instance name with small pills "наш" (indigo) and "устарел"
  (amber), a created-date + owner line, a soft status pill with a colored dot, and a small red trash icon
  at the right (greyed-out on "наш" rows). Dense, clean.*
- **4b Image prompt:** *…identical, but every row carries only the "наш" (indigo) pill with a green
  "Подключён" status, no "устарел" pills, and there is NO red bulk-delete button in the header.*

---

## 5. Конструктор базы знаний — `/playground` *(backend-ready · UI to build)*

The KB **builder**: enrich the assistant's knowledge in a chat-like flow — **drop materials**, run the
builder, answer **«Запросы AI»** popups — then **«Сохранить в базу»** approves the pending entities into
the live KB. **One KB, no versions** (version/history chrome intentionally dropped).

```
┌──┬────────────────────────────────────┬──────────────────────┐
│N │ Конструктор   [Сохранить][Отменить] │ Обзор базы знаний     │
│a │ chat: operator · AI ассистент       │  tiles (темы/товары/  │
│v │ material bubbles + «Предложенные…»  │   тарифы/медиа/конт.)  │
│  │ «Черновик» chips on new rows        │ Запросы AI (popups)   │
│  │ [ composer: текст + 📎 ]            │ Последние изменения   │
└──┴────────────────────────────────────┴──────────────────────┘
```

- **Left = chat thread** (operator turns + **«AI ассистент»** turns; material-upload bubbles with
  thumbnails; **«Предложенные изменения»** summaries whose new rows carry an amber **«Черновик»** badge) +
  a **composer** (text + attach).
- **Right rail:** **«Обзор базы знаний»** tiles (Темы · Товары · Тарифы · Медиа-ресурсы · Контакты),
  **«Запросы AI»** popup cards, **«Последние изменения»**, and a **«Готовность»** bar (how many
  «Черновик» remain).
- **v1 builder is deterministic** (`RuleSynthesizer`): the «AI ассистент» turn concatenates `ready`
  materials into a topic and regex-detects ₸ values (raising `confirm_fact` popups) — it **summarizes what
  it synthesized as «Черновик» rows**, it is **not** a conversational LLM.
- **▸ Backed by:** `GET /playground/draft` → **`DraftView`** (config + topics + products + tariffs +
  assets + contacts + **materials** + **requests**); an entity is **«Черновик»** iff pending, **LIVE** iff
  already in the live KB. `POST /playground/chat {instruction}` → `{result, draft}`; `POST
  /playground/draft/materials` (multipart `file` **or** `{source_type,text,url}`) + SSE
  `kb.material.updated`; `GET /playground/requests` → `KbRequest[]`; `POST /playground/requests/{id}/resolve`;
  `POST /playground/draft/approve` («Сохранить в базу» — gate → materialize pending → live, **422** on
  gate failure) + `POST /playground/draft/approve/{kind}/{id}` (approve one). Overview tiles / «Последние
  изменения» / readiness are **derived client-side** from the `DraftView` (entity counts + which are
  pending + `updated_at`). Live via `kb.row.changed` / `kb.material.updated` / `kb.approved`.

**Cases** — *this is the page you most want case-images for.*

| Case | File | What differs |
|---|---|---|
| **5a · Empty** | `ui/playground-empty.png` | first run: onboarding canvas, all tiles 0 |
| **5b · Jobs running** | `ui/playground-jobs.png` | materials just dropped, **extraction jobs in progress** |
| **5c · Draft filled** | `ui/ai-playground.png` *(exists)* | builder produced a draft **across several tables + media** |
| **5d · AI requests** | `ui/playground-requests.png` | pending **«Запросы AI»** popups (confirm price / describe media) |

- **5a · Empty.** Centered empty-state card in the thread — sparkles/inbox illustration, **«База знаний
  пуста»**, subtext **«Перетащите материалы сюда или добавьте первую тему, товар или тариф»**, a primary
  **«Добавить первую тему»** + a ghost **«Загрузить материалы»** (📎). Header **«Сохранить в базу»** and
  **«Отменить»** are **disabled**. Right rail tiles all **0**; «Запросы AI» → «ещё нет запросов»;
  «Последние изменения» → «пока пусто»; readiness empty.
  **▸ Backed by:** `GET /playground/draft` returns an empty `DraftView` (zero topics/products/tariffs/
  assets/contacts, no materials, no requests). First upload/write transitions to a filled state.
  - **Image prompt:** *Knowledge-base "builder" page. Header "Конструктор базы знаний" with a DISABLED
    greyed "Сохранить в базу" and disabled "Отменить изменения". Center: a large empty canvas with a soft
    line-art illustration (sparkles over an open box), heading "База знаний пуста", grey subtext
    "Перетащите материалы сюда или добавьте первую тему, товар или тариф", a primary indigo "Добавить
    первую тему" and a ghost "Загрузить материалы" with a paperclip; a bottom composer. Right column: an
    "Обзор базы знаний" card whose tiles all read 0 (Темы 0, Товары 0, Тарифы 0, Медиа-ресурсы 0,
    Контакты 0), a "Запросы AI" card "ещё нет запросов", a "Последние изменения" card "пока пусто", an
    empty readiness bar. Calm onboarding feel.*

- **5b · Jobs running (background extraction).** The operator just dropped a PDF, an image and a URL. The
  thread shows **material bubbles** each with a **status chip**: a spinning `LoaderCircle` + **«Обработка…»**
  (extracting), a grey **«В очереди»** (pending), or a green **«Готово»** (ready); a failed one shows an
  amber **«Не удалось — опишите вручную»**. No «Предложенные изменения» yet (synthesis waits for `ready`).
  Right rail overview still reflects the **current** KB (unchanged); a small "Обрабатывается: 2" note.
  **▸ Backed by:** `POST /playground/draft/materials` → `KbMaterial{source_type, status:
  pending|extracting|ready|failed, media_kind, extraction}`; `GET /playground/draft/materials`; live
  **`kb.material.updated {material_id, status}`** flips each chip. (Text materials are born `ready`; file/
  url enqueue an extraction pass.)
  - **Image prompt:** *…same builder layout. Center thread shows three uploaded-material bubbles: (1) a PDF
    tile "прайс.pdf" with a small spinning loader and grey label "Обработка…"; (2) an image thumbnail with
    a grey "В очереди" chip; (3) a link card "example.kz" with a green "Готово" chip. No suggestion card
    yet. A bottom composer. Right column: the "Обзор базы знаний" tiles show existing non-zero counts
    (e.g. Темы 8, Товары 5), a small grey note "Обрабатывается: 2", and "Запросы AI: ещё нет". Convey
    active background work via the spinner and status chips.*

- **5c · Draft filled across tables + media.** The builder produced a **«Предложенные изменения»** card in
  the thread summarizing what it created; multiple **«Черновик»** rows now exist **across kinds** — new
  **topics**, **products** (with prices), **tariffs**, and attached **media**. Right-rail tiles show the
  bumped counts with small amber "+N черновик" deltas; readiness bar reads e.g. **«Черновиков: 6»**; each
  pending item offers a **«Подтвердить»**; header **«Сохранить в базу»** is **enabled**.
  **▸ Backed by:** `POST /playground/chat` result + `DraftView` now carrying pending `topics/products/
  tariffs/assets` entries (each with `provenance`); `POST /playground/draft/approve/{kind}/{id}` per row;
  `POST /playground/draft/approve` for all.
  - **Image prompt:** *…same builder layout, populated. Center thread: an operator message on the right,
    an "AI ассистент" message on the left with a small bot/sparkles tile and a light "Предложенные
    изменения" card listing chips — "Тема: Тарифы", "Товар: Nike X — 25 000 ₸", "Тариф: Рост", "Медиа:
    прайс.pdf" — each chip with a small amber "Черновик" badge and a tiny "Подтвердить" link; a bottom
    composer. Right column: an "Обзор базы знаний" card of stat tiles with non-zero counts and small amber
    "+2" deltas (Темы 9, Товары 6, Тарифы 4, Медиа-ресурсы 12, Контакты 1), a "Последние изменения" list,
    and a "Готовность" progress bar reading "Черновиков: 6". Header "Сохранить в базу" is a solid indigo
    (enabled) button.*

- **5d · AI requests (popups).** The right rail **«Запросы AI»** holds pending popup cards: a
  **`confirm_fact`** — *"Подтвердите цену тарифа «Рост»"* with a prefilled value **«25 000 ₸/мес»**, an
  editable field and a primary **«Подтвердить»**; and a **`describe_media`** — a thumbnail + *"Опишите, что
  на изображении"* with a text field. A small red count badge sits on the «Запросы AI» header.
  **▸ Backed by:** `GET /playground/requests` → `KbRequest{req_type: confirm_fact|describe_media, prompt,
  context, target, state:open}`; `POST /playground/requests/{id}/resolve` (`confirm_fact` →
  `{table,slug,field,lang,value}`; `describe_media` → `{description}`).
  - **Image prompt:** *…same builder layout. Right column "Запросы AI" is prominent with a small red "2"
    badge and two popup cards: card one "Подтвердите цену тарифа «Рост»" with a text field prefilled "25
    000 ₸/мес" and a primary indigo "Подтвердить" plus a ghost "Пропустить"; card two shows a small image
    thumbnail, "Опишите, что на изображении", a short text field and a "Сохранить" button. The center chat
    thread is lightly visible behind. Convey "the assistant is asking the operator to confirm facts".*

---

## 6. Редактор базы знаний — `/knowledge-base` *(backend-ready · UI to build)*

The KB **editor**: a tabbed, structural view of the **same living KB** — see everything and edit it
directly, then **«Сохранить в базу»** to approve pending edits. **One KB, no versions.** Tabs: **Обзор ·
Темы · Товары · Тарифы · Медиа-ресурсы · Контакты · Правки**.

> **Facts are typed columns, quoted as tokens.** A price/limit is a **typed column** stored verbatim, one
> row **per language**, on `ai_products` / `ai_tariffs` / `ai_contacts`. In any prose it appears only as a
> `{{table.slug.field}}` token — never a raw number. Topic bodies are pure prose (no tokens, no digits).

- **Core:** stat cards (Темы / Товары / Тарифы / Медиа-ресурсы / Контакты / Правки) + the tab strip; each
  tab lists rows (pending rows carry an amber **«Черновик»** badge) with an inline editor. **Removed vs the
  old mockup:** ~~История~~ tab, ~~Версия~~ card — one KB, no version history/rollback in the UI.
- **▸ Backed by:** everything reads **`GET /playground/draft`** (`DraftView`); edits: topics
  `POST/DELETE /playground/draft/topics[/{slug}]`; products `…/products[/{ref}]`; tariffs
  `…/tariffs[/{ref}]`; assets `POST`(multipart)`/PATCH/DELETE …/assets[/{ref}]` with `owner_kind`/`owner_ref`;
  contacts `PATCH …/contacts` (per `lang`); config `PATCH …/config`. Approve: `POST
  /playground/draft/approve/{kind}/{id}` (one) / `POST /playground/draft/approve` (all). Stat counts +
  readiness derived from `DraftView`; live via `kb.row.changed` / `kb.approved`. Writes send optional
  `If-Match` (draft `updated_at`) → `409 DRAFT_STALE`.

**Cases** — *this is the KB you want empty / populated / multi-language / edits variants for.*

| Case | File | What differs |
|---|---|---|
| **6a · Empty** | `ui/kb-empty.png` | first run: every tab empty, all stat cards 0 |
| **6b · Обзор populated** | `ui/ai-knowledge-base.png` *(exists)* | filled stat cards + recent changes |
| **6c · Товары editor** | `ui/kb-products.png` | product list + inline editor (typed price, `data`, owned media) |
| **6d · Тарифы editor** | `ui/kb-tariffs.png` | tariff editor (price/limit_text/fee, pricing_type, adv/disadv) |
| **6e · Multi-language entity** | `ui/kb-multilang.png` | one entity shown with its **ru + kk** rows |
| **6f · Медиа-ресурсы** | `ui/kb-media.png` | asset grid, each showing its **owner** (тема/товар/тариф/global) |
| **6g · Контакты** | `ui/kb-contacts.png` | org support scalars, **per language** |
| **6h · Правки** | `ui/kb-edits.png` | the list of **all pending «Черновик»** entities across kinds |

- **6a · Empty.** Stat cards all **0**; the active tab body shows a **per-tab empty-state** (Товары →
  «Добавьте первый товар», Тарифы → «Создайте первый тариф», Медиа → «Прикрепите первый файл», Контакты →
  «Укажите контакты поддержки») with a primary **«Добавить…»**; the **Обзор** tab links to the
  **Конструктор** to drop materials. Header **«Сохранить в базу»** disabled.
  **▸ Backed by:** `GET /playground/draft` → empty `DraftView`.
  - **Image prompt:** *Knowledge-base "editor" page. Header "Редактор базы знаний" with a DISABLED greyed
    "Сохранить в базу". A row of stat cards all reading 0: Темы 0, Товары 0, Тарифы 0, Медиа-ресурсы 0,
    Контакты 0, Правки 0. Tab strip "Обзор · Темы · Товары · Тарифы · Медиа-ресурсы · Контакты · Правки"
    with Товары active. Below, a centered per-tab empty state: a soft line-art illustration, "Здесь пока
    пусто", grey "Добавьте первый товар", a primary indigo "Добавить товар". Right column: empty "Быстрый
    доступ", "Последние изменения — пока пусто", a "Готовность" bar "Черновиков: 0".*
- **6b · Обзор populated.** **▸ Backed by:** counts + recents from a populated `DraftView`.
  - **Image prompt:** *…header with a primary "Сохранить в базу" (NO "История", NO "Версия"). Stat cards
    with counts: Темы 12, Товары 9, Тарифы 4, Медиа-ресурсы 15, Контакты 8, Правки 4. Tab strip with
    "Обзор" active showing summary cards (a "Последние изменения" list and a "Готовность к публикации" bar
    reading "Черновиков: 4"). Dense, calm.*
- **6c · Товары editor.** Two-pane: a left list of products (name + category, some with an amber
  «Черновик» badge and a «Подтвердить» link) and a right form — **name**, a **typed PRICE** field (verbatim,
  "25 000 ₸"), **description**, **category**, a **`data` key-value editor**, and **owned-media chips**.
  **▸ Backed by:** `DraftView.products` (`ProductRow{ref,lang,name,price,description,category}`) + attached
  `assets` where `owner=product`; `POST /playground/draft/products`.
  - **Image prompt:** *…"Товары" tab active. Two-pane editor: LEFT a list of product rows (name + small
    grey category, a couple with an amber "Черновик" badge and a "Подтвердить" link); RIGHT a form for the
    selected product — a name field "Nike X", a PRICE field showing "25 000 ₸" (verbatim), a description
    textarea, a category field, a small key-value "data" editor (size / color rows), and a row of
    owned-media thumbnail chips. Right column "Быстрый доступ" + "Готовность (Черновиков: 4)".*
- **6d · Тарифы editor.** Right form — **name**, **summary**, a **pricing_type** segmented control
  (fixed / percentage / tiered), typed **PRICE / LIMIT_TEXT / FEE** fields (verbatim), **advantages** &
  **disadvantages** text, owned media. **▸ Backed by:** `DraftView.tariffs` (`TariffRow{ref,lang,name,
  price,limit_text,fee,pricing_type,advantages,disadvantages}`); `POST /playground/draft/tariffs`.
  - **Image prompt:** *…"Тарифы" tab active. Two-pane: LEFT tariff rows (Пробный, Рост, Масштаб, Про);
    RIGHT a form for "Рост" — name, summary, a segmented "fixed / percentage / tiered" with "fixed"
    selected, a PRICE field "25 000 ₸/мес", a LIMIT_TEXT field "до 2 000 платежей/мес", an empty FEE
    field, two text areas "Преимущества" and "Недостатки", and owned-media chips. Amber "Черновик" badge
    on the row header, a "Подтвердить" link.*
- **6e · Multi-language entity (the "different languages" case).** The editor exposes **language as a
  row**: the selected tariff shows a small **language switch «RU / KK»** (and a `+ язык` affordance), and
  the two language rows sit side by side — **ru**: name «Рост», price «25 000 ₸/мес»; **kk**: name «Өсу»,
  price «25 000 ₸/ай». Language-neutral fields (a phone) show a **`*`** marker. v1 fills **ru** only; kk is
  shown here to illustrate the design (UI chrome stays Russian — no app-language switcher).
  **▸ Backed by:** every KB row carries **`lang`** ('ru'|'kk'|'*'); upserts take an optional `lang`
  (`POST /playground/draft/{topics|products|tariffs}`, `PATCH …/contacts`) — one row per `(entity, lang)`.
  - **Image prompt:** *…"Тарифы" tab, editing tariff "Рост". At the top of the right form a small
    segmented language switch "RU · KK" plus a faint "+ язык"; below it two stacked language rows: a RU row
    (name "Рост", price "25 000 ₸/мес", limit "до 2 000 платежей/мес") and a KK row (name "Өсу", price "25
    000 ₸/ай", limit "айына 2 000 төлемге дейін"), each labelled with a small "RU"/"KK" pill; a
    language-neutral WhatsApp field marked with a grey "*". Convey "one row per language". Everything else
    identical to the editor.*
- **6f · Медиа-ресурсы.** A grid/list of assets; each item shows a type icon (`Image`/`Film`/`FileText`/
  `Mic`), title, description, and an **owner** pill (**тема / товар / тариф / глобально**) with a control
  to reassign. **▸ Backed by:** `DraftView.assets` (`AssetRow{ref,kind,description,url,owner_kind,
  owner_ref,lang}`); `PATCH /playground/draft/assets/{ref}` (`description?`, `owner`).
  - **Image prompt:** *…"Медиа-ресурсы" tab active. A grid of media cards, each a thumbnail or type-icon
    tile with a title, a one-line description, and a small owner pill ("Тема: Тарифы", "Товар: Nike X",
    "Глобально"); a couple carry an amber "Черновик" badge. A primary "Загрузить файл" button top-right.*
- **6g · Контакты.** A single form of org support scalars **per language**: whatsapp, email, address,
  legal, callback_time — with the same «RU / KK / \*» language control (phones/emails on the `*` row).
  **▸ Backed by:** `DraftView.contacts` (`ContactRow{slug:'support',lang,whatsapp,email,address,legal,
  callback_time}`); `PATCH /playground/draft/contacts`.
  - **Image prompt:** *…"Контакты" tab active. A single card "Контакты поддержки" with fields WhatsApp
    (+7 702 976-65-09), Email, Адрес, Юр. лицо (all marked language-neutral "*") and a "Время обратного
    звонка" field with a small "RU/KK" switch (RU: "1 час"). A primary "Сохранить" button. Calm form.*
- **6h · Правки.** A flat list of **every pending «Черновик»** entity across kinds — each row: a kind icon
  + label (Тема/Товар/Тариф/Медиа/Контакт), the entity name, a small `provenance` hint (откуда), and a
  **«Подтвердить»** / **«Отклонить»** pair; a header **«Подтвердить все»**. **▸ Backed by:** the pending
  subset of the `DraftView`; `POST /playground/draft/approve/{kind}/{id}` / `…/approve`.
  - **Image prompt:** *…"Правки" tab active (its stat card reads "Правки 4"). A list of four pending rows,
    each with a small kind icon and label ("Товар — Nike X", "Тариф — Рост", "Тема — Доставка", "Медиа —
    прайс.pdf"), a faint source hint, an amber "Черновик" badge, and "Подтвердить"/"Отклонить" buttons; a
    header "Подтвердить все" primary button and a "Готовность (Черновиков: 4)" bar on the right.*

---

## 7. Contacts — `/contacts` *(deferred; spec for later)*

List + detail drawer.
- **Core:** searchable table — initials Avatar, name, phone, last-contact time.
- **One-click (drawer):** identities (phone / `@lid` / push name), attributes, the contact's chats.
- **▸ Backed by:** `GET /contacts {q?,page}`, `GET /contacts/{id}` → `Contact{display_name, phone_number,
  phone_jid, lid_jid, push_name, attributes}` + recent chats.

**Cases**

| Case | File | What differs |
|---|---|---|
| **7a · List + drawer** | `ui/contacts.png` *(exists)* | table with an open detail drawer |

- **Image prompt:** *Contacts page. Center: a table "Контакты" with a search input and columns —
  initials avatar, name, phone, "последний контакт". An open right-side detail drawer showing a contact's
  initials avatar, name, phone, a few key-value attributes, an "Идентификаторы" list (phone, @lid, push
  name), and that contact's chats.*

---

## 8. Settings — `/settings` *(deferred)*

Two tabs — **Организация · Пользователи** (no "General"/theme/language tab; nothing backs it).

- **Организация:** name, auto-response mode (Никогда / По расписанию / Всегда) + a **UTC** time window
  (with a hint). **▸ Backed by:** `GET|PATCH /organization`.
- **Пользователи:** a table of users; **＋ Добавить пользователя** (email + password). **▸ Backed by:**
  `GET /users`, `POST /users {email,password,name?}`.

**Cases**

| Case | File | What differs |
|---|---|---|
| **8a · Организация** | `ui/settings.png` *(exists)* | org form + auto-response window |
| **8b · Пользователи** | `ui/settings-users.png` | user table + add-user row |

- **8a Image prompt:** *Settings page, two tabs "Организация · Пользователи" with "Организация" active:
  fields for org name, an auto-response mode segmented control (Никогда / По расписанию / Всегда), a
  start–end time window with a small "UTC" hint, and a primary indigo "Сохранить" button. Clean light
  form.*
- **8b Image prompt:** *…"Пользователи" tab active: a simple table with columns Имя, Email, Добавлен, and
  a primary "Добавить пользователя" button top-right; below the table an inline add row with an email
  field and a password field. Clean, dense.*

---

## Out of scope (v1)

Reports/analytics, campaigns, SLA, macros, billing, granular permissions, self-signup, OAuth, password
reset, presence/last-seen, profile-picture avatars, app-language switching / i18n (per `0-overview.md`).
**Not shown, not stubbed** — because none of it is backed.
