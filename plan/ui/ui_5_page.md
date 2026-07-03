# Knowledge Base Builder UI Canvas

## Scope

This canvas describes the four frontend states of the **Knowledge Base Builder**:

* **5a — Empty**
* **5b — Materials processing**
* **5c — Draft generated**
* **5d — AI requests**

All four screens use the same route:

`/playground`

The layout and visual style remain constant. Only the content and available actions change according to the current knowledge-base state.

---

# 1. Product purpose

The Knowledge Base Builder allows an operator to configure the AI assistant without manually filling every database field.

The operator can:

* upload files;
* attach images, PDFs, audio, or video;
* add URLs;
* paste text;
* write instructions;
* review knowledge suggested by the system;
* confirm individual entities;
* answer clarification requests;
* save approved knowledge to the live knowledge base.

The page behaves like a chat workspace, but it is not a normal AI chatbot.

The central conversation represents a sequence of knowledge-building actions:

1. the operator provides materials or instructions;
2. the system processes the materials;
3. the builder creates proposed entities;
4. uncertain facts are returned as AI requests;
5. the operator confirms the entities;
6. the draft is saved into the live knowledge base.

---

# 2. Shared visual style

## General direction

The interface uses a restrained **Linear-style minimal design** based on shadcn-vue and Tailwind.

The page should feel:

* professional;
* dense but not overloaded;
* calm;
* technical;
* predictable;
* implementation-oriented.

Avoid decorative dashboard elements that do not represent real backend data.

## Color system

### Main surfaces

* Application background: `#F8FAFC`
* Cards and working surfaces: white
* Primary text: `#0F172A`
* Secondary text: `#64748B`
* Borders: light cool grey
* Navigation rail: `#0F172A`

### Primary accent

Use indigo `#4F46E5` for:

* primary buttons;
* active navigation;
* links;
* focus rings;
* progress bars;
* selected controls.

### Status colors

Use status colors only where they convey meaning:

* Green: successfully processed or ready
* Amber: draft, warning, or manual action required
* Red: unresolved requests or errors
* Grey: queued, disabled, or inactive
* Indigo: active processing or primary action

Do not use gradients.

## Typography

Use Inter or a similar neutral sans-serif.

Recommended hierarchy:

* Page title: 28–32 px, semibold
* Section title: 16–18 px, semibold
* Main content: 14–16 px
* Secondary metadata: 12–14 px
* Status badges: 12 px

## Shape and borders

* Card radius: 8 px
* Buttons: 8 px
* Inputs: 8 px
* Border: 1 px
* Shadows: only for dialogs or floating elements
* Normal cards should use borders instead of heavy shadows

---

# 3. Shared page shell

Every case uses the same page skeleton.

## 3.1 Left navigation rail

A narrow dark vertical rail is fixed on the left.

It contains four icons:

1. Inbox
2. WhatsApp
3. AI / builder
4. Knowledge base

The Builder icon is active while the user is on `/playground`.

The active navigation item uses an indigo background or indigo indicator.

A circular user avatar is positioned at the bottom.

Use initials instead of a profile photo.

Example:

`AK`

## 3.2 Header

The header contains:

### Left side

Page title:

**Конструктор базы знаний**

### Right side

Two actions:

* **Сохранить в базу**
* **Отменить изменения**

The visual state of these buttons depends on the current case.

### Save button

Enabled only when there are pending entities that can be approved.

### Cancel button

Enabled when the current draft contains unsaved changes.

Both buttons are disabled in the initial empty state.

## 3.3 Main workspace

The central area is a chat-like knowledge-building timeline.

It can contain:

* operator messages;
* AI assistant messages;
* uploaded-material cards;
* extraction statuses;
* proposed-change cards;
* assistant explanations;
* contextual actions.

The workspace must not look like a normal WhatsApp conversation.

It should resemble an activity thread where the operator and the builder collaborate on knowledge.

## 3.4 Composer

The composer remains visible at the bottom in every state.

It contains:

* paperclip attachment button;
* text input or autosizing textarea;
* send action.

Placeholder:

**Напишите инструкцию или добавьте материалы...**

The user can use the composer to:

* provide an instruction;
* paste text;
* add a URL;
* upload a file;
* describe a failed material.

## 3.5 Right information rail

The right rail contains four stacked sections:

1. **Обзор базы знаний**
2. **Запросы AI**
3. **Последние изменения**
4. **Готовность**

The rail remains visible in every case, but its content changes.

---

# 4. Shared components

## Knowledge overview

Shows current entity counts:

* Темы
* Товары
* Тарифы
* Медиа-ресурсы
* Контакты

Counts represent the merged draft view.

When drafts exist, the component may show:

* total count;
* pending delta.

Example:

`Товары 6  +1`

The main value represents the total visible count.

The amber delta represents pending entities.

## AI requests

Displays unresolved requests that require operator input.

Possible request types:

* confirm a fact;
* describe media;
* clarify a field;
* provide missing information.

When there are no requests, show:

**ещё нет запросов**

## Recent changes

Shows a compact activity list derived from draft entities and material updates.

Each row contains:

* entity or material icon;
* short label;
* timestamp.

## Readiness

Displays the number of unresolved drafts and optionally a progress bar.

Examples:

* `Черновиков: 0`
* `Черновиков: 6`

The progress bar should represent readiness for saving, not arbitrary completion.

---

# 5. Case 5a — Empty state

## Purpose

This is the first-run state.

The organization has not yet added:

* topics;
* products;
* tariffs;
* media;
* contacts;
* materials;
* AI requests.

The page must teach the user how to start.

## Header

### Save button

Disabled.

Label:

**Сохранить в базу**

### Cancel button

Disabled.

Label:

**Отменить изменения**

## Main workspace

The conversation timeline is empty.

Show a large centered onboarding card.

### Illustration

Use a simple line illustration:

* open box;
* document;
* message fragment;
* sparkles.

The illustration should be subtle and neutral.

Do not use a complex hero illustration.

### Title

**База знаний пуста**

### Supporting text

**Перетащите материалы сюда или добавьте первую тему, товар или тариф**

### Primary action

**Добавить первую тему**

This action begins a manual topic-creation flow.

### Secondary action

**Загрузить материалы**

Include a paperclip or upload icon.

This action opens the file picker.

## Drag-and-drop behavior

The entire central workspace may act as a drop zone.

When a file is dragged over the page:

* show a subtle indigo border;
* change the central message to a drop instruction;
* do not obscure the right rail.

## Composer

Visible but inactive-looking.

The user may still enter a text instruction or attach a file.

## Right rail

### Overview

All values are zero:

* Темы — 0
* Товары — 0
* Тарифы — 0
* Медиа-ресурсы — 0
* Контакты — 0

### AI requests

Show:

**ещё нет запросов**

### Recent changes

Show:

**пока пусто**

### Readiness

Empty progress bar.

Show:

**Черновиков: 0**

## Main frontend condition

Render this state when:

* all entity arrays are empty;
* materials are empty;
* requests are empty;
* no pending entities exist.

---

# 6. Case 5b — Materials processing

## Purpose

The operator has uploaded materials.

The system is processing them in the background.

No proposed knowledge changes have been generated yet.

## Header

### Save button

Disabled or visually inactive.

There are no draft entities ready to save.

### Cancel button

May be enabled when uploaded materials can be removed from the draft.

## Main workspace

Replace the empty onboarding card with a material-processing thread.

At the top, show an assistant message:

**Материалы приняты. Извлекаю информацию и готовлю предложения.**

Below it, show individual material cards.

Each material must have its own independent state.

## Material card anatomy

Each card contains:

* source icon or thumbnail;
* filename or URL;
* media type;
* file size when available;
* current status;
* timestamp.

## Processing example

### PDF

Name:

`прайс.pdf`

Metadata:

`PDF · 2.4 МБ`

Status:

**Обработка…**

Visual treatment:

* indigo spinner;
* light indigo status badge.

## Queued example

### Image

Name:

`баннер_тарифы.webp`

Metadata:

`Изображение · 512 КБ`

Status:

**В очереди**

Visual treatment:

* grey badge;
* image thumbnail.

## Ready example

### URL

Name:

`example.kz`

Type:

`Ссылка`

Status:

**Готово**

Visual treatment:

* green badge;
* link icon.

A ready material may show a compact extracted-data summary:

**Извлечено: 3 темы, 2 тарифа, 1 контакт**

This summary should remain collapsed by default.

## Failed example

### Image or scanned document

Name:

`скан-договор.jpg`

Metadata:

`Изображение · 1.1 МБ`

Status:

**Не удалось — опишите вручную**

Visual treatment:

* amber badge;
* document warning icon.

Below the failed material, show an assistant message:

**1 материал не удалось обработать автоматически. Опишите его вручную, чтобы я смог добавить данные в базу.**

The user can respond through the composer.

## Important behavior

Do not show a **Предложенные изменения** card yet.

The builder has not finished synthesis.

Material states can update independently through SSE.

One failed material must not prevent other ready materials from completing.

## Right rail

### Overview

Show the current live or previously saved knowledge-base counts.

Example:

* Темы — 8
* Товары — 5
* Тарифы — 3
* Медиа-ресурсы — 10
* Контакты — 1

These numbers should not include entities that have not yet been synthesized.

### Processing indicator

Show:

**Обрабатывается: 2**

This should be a subtle helper row, not a large card.

### AI requests

Show:

**ещё нет запросов**

### Recent changes

Show material activity, for example:

* example.kz — Добавлен материал
* баннер_тарифы.webp — Загружен в очередь

### Readiness

Show:

**Черновиков: 0**

No proposed entities exist yet.

## Main frontend condition

Render this state when at least one material has status:

* pending;
* extracting;
* failed;

and no synthesized pending entities exist yet.

---

# 7. Case 5c — Draft generated

## Purpose

The builder has processed the materials and generated proposed knowledge entities.

The proposal includes several knowledge-base sections.

The operator can review and confirm each entity separately or save all pending changes.

## Header

### Save button

Enabled.

Primary indigo button:

**Сохранить в базу**

### Cancel button

Enabled.

Secondary outline button:

**Отменить изменения**

## Main workspace

Show the operator’s original instruction as a message aligned to the right.

Example:

**Я загрузил материалы: прайс, описание тарифов и новости об обновлениях. Обработай их и предложи структуру базы.**

Use the operator avatar:

`AK`

Below it, show an assistant message.

### Assistant label

**AI ассистент**

### Assistant explanation

**Я проанализировал материалы и сформировал черновики новых сущностей в базе знаний. Проверьте и подтвердите предложенные изменения.**

## Proposed changes card

Show a bordered card titled:

**Предложенные изменения**

The card contains proposed entities across different entity kinds.

## Proposed row anatomy

Each row contains:

* entity-type icon;
* entity label;
* entity name or main value;
* amber **Черновик** badge;
* **Подтвердить** action;
* chevron for opening details.

## Example rows

### Topic

**Тема: Тарифы**

### Product

**Товар: Nike X — 25 000 ₸**

### Tariff

**Тариф: Рост**

### Media

**Медиа: прайс.pdf**

Each row has an individual:

**Подтвердить**

This allows the operator to approve one entity without approving all others.

## Row interaction

Clicking the row or chevron opens a detail drawer.

The drawer may show:

* generated fields;
* provenance;
* source material;
* language;
* owned media;
* editable values.

The main screen should show only summary information.

## Right rail

### Overview

Show total counts and draft deltas.

Example:

* Темы — 9, `+1`
* Товары — 6, `+1`
* Тарифы — 4, `+1`
* Медиа-ресурсы — 12, `+2`
* Контакты — 1

The delta must use the amber draft color.

### AI requests

If no confirmation requests have been generated, show:

**ещё нет запросов**

### Recent changes

Example:

* Тема «Тарифы»
* Товар «Nike X — 25 000 ₸»
* Тариф «Рост»
* Медиа «прайс.pdf»

### Readiness

Show a visible progress bar.

Example:

`67%`

Below it:

**Черновиков: 6**

The progress bar should become more complete as drafts are resolved or approved.

## Main frontend condition

Render this state when:

* synthesized pending entities exist;
* there are no unresolved AI requests requiring the primary focus.

---

# 8. Case 5d — AI requests

## Purpose

The builder has generated draft knowledge but cannot complete some entities safely without operator input.

The system asks the operator to:

* confirm a fact;
* describe media.

The right rail becomes the main focus.

## Header

### Save button

May remain enabled if valid draft entities already exist.

However, unresolved requests should prevent invalid entities from being approved.

### Cancel button

Enabled.

## Main workspace

Keep the conversation visible, but make it secondary to the request panel.

Show an operator message such as:

**Добавьте тему «Тарифы» и черновик содержания тарифа «Рост».**

Below it, show an assistant response:

**Создал черновик темы «Тарифы» и тариф «Рост». Для завершения нужны подтверждения от оператора.**

## Assistant status card

Show a compact checklist with three rows.

### Completed item

**Создан черновик тарифа «Рост»**

Use a green check.

### Completed item

**Найдены источники**

Use a green check.

### Pending item

**Требуются подтверждения оператора**

Use a red or amber badge:

`2`

## Follow-up assistant message

Show:

**Есть запросы на подтверждение. Проверьте их, пожалуйста.**

Add an outline action:

**Перейти к запросам**

Clicking it should scroll or focus the right-side request section.

## Right rail

### Overview

Show non-zero counts.

Example:

* Темы — 12
* Товары — 28
* Тарифы — 9
* Медиа-ресурсы — 42
* Контакты — 18

### AI requests header

Title:

**Запросы AI**

Add a red count badge:

`2`

This section should be visually stronger than in the previous cases.

---

## Request type 1 — Confirm fact

Show a request card with a small technical chip:

`confirm_fact`

Add request number:

`1`

### Prompt

**Подтвердите цену тарифа «Рост»**

### Editable field

Prefilled value:

`25 000 ₸/мес`

### Primary action

**Подтвердить**

### Secondary action

**Пропустить**

The operator may edit the value before confirmation.

---

## Request type 2 — Describe media

Show a request card with technical chip:

`describe_media`

Add request number:

`2`

### Media preview

Show a small image thumbnail.

### Prompt

**Опишите, что на изображении**

### Description field

Prefilled or empty text example:

**Баннер тарифа «Рост» с графиком ...**

### Primary action

**Сохранить**

The saved description becomes part of the media asset.

---

## Recent changes

Show compact entries such as:

* Тариф «Рост» — черновик
* Медиа «прайс.pdf»

## Readiness

Show progress and unresolved draft count.

Example:

`60%`

**Черновиков: 3**

Resolving requests should immediately update:

* readiness;
* draft entities;
* recent changes;
* AI request count.

## Main frontend condition

Render this state when at least one request has:

`state: open`

The request section should take priority over the normal activity feed.

---

# 9. State transitions

## 5a → 5b

Triggered when the user:

* uploads a file;
* adds a URL;
* sends material through the composer.

The empty-state card disappears.

Uploaded material cards appear.

## 5b → 5c

Triggered when:

* enough materials reach `ready`;
* the builder synthesizes pending entities;
* no blocking request is created.

Show proposed changes.

Enable **Сохранить в базу**.

## 5b → 5d

Triggered when extraction or synthesis creates unresolved requests.

Example:

* uncertain price;
* image without a usable description.

## 5c → 5d

Triggered when one of the generated entities requires confirmation.

The proposed changes remain in the draft, while AI requests become visible.

## 5d → 5c

Triggered when all open requests are resolved.

The request count becomes zero.

The screen returns to the normal draft-review state.

## 5c → Saved state

Triggered when the user clicks:

**Сохранить в базу**

The frontend should:

1. submit the approval request;
2. disable the button while saving;
3. show progress;
4. update entities from draft to live;
5. clear approved draft badges;
6. update overview counts;
7. update readiness.

---

# 10. Frontend component structure

Recommended component hierarchy:

```text
KnowledgeBuilderPage
├── AppNavigationRail
├── KnowledgeBuilderHeader
│   ├── SaveKnowledgeButton
│   └── CancelDraftButton
├── BuilderWorkspace
│   ├── EmptyKnowledgeState
│   ├── BuilderTimeline
│   │   ├── OperatorMessage
│   │   ├── AssistantMessage
│   │   ├── MaterialCard
│   │   ├── MaterialStatusBadge
│   │   ├── ProposedChangesCard
│   │   ├── ProposedEntityRow
│   │   └── AssistantStatusCard
│   └── BuilderComposer
└── KnowledgeBuilderRail
    ├── KnowledgeOverviewCard
    ├── AiRequestsCard
    │   ├── ConfirmFactRequest
    │   └── DescribeMediaRequest
    ├── RecentChangesCard
    └── ReadinessCard
```

---

# 11. Rules that apply to every case

* Never combine two cases in one screen.
* Keep the layout constant between states.
* Only data and actions should change.
* Do not show unsupported analytics.
* Do not show history or version controls.
* Do not show a language switcher for the application.
* Do not show profile photos.
* Do not show decorative information that is not available from the backend.
* Show full entity details in a drawer, not directly in the timeline.
* Keep primary information visible and secondary detail one click away.
* Preserve the composer in every state.
* Preserve the right rail in every state.
* Use the same spacing, typography, borders, and icon language across all four screens.
