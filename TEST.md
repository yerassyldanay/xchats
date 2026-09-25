# Beauty Salon Vertical: Test Specification & Simulation (`TEST.md`)

This document defines realistic test data, customer conversation scenarios (in Russian and Kazakh), validation contracts, and UI verification flows for the Beauty Salon Knowledge Base extension specified in [PLAN.md](./PLAN.md).

---

## 1. Realistic Seed Data for SQLite Tables

Organization: **Салон красоты «Аура» (Aura Beauty Studio)**  
Default Location: **Алматы, пр. Достык, 128**  
Main Booking Link: `https://xpayment.kz/book/aura-almaty`

### 1.1. `ai_contacts`

```sql
INSERT INTO ai_contacts (
    id, organization_id, phone, address, working_hours, instagram, booking_url, created_at, updated_at
) VALUES (
    'c0000000-0000-0000-0000-000000000001',
    'org-beauty-aura-001',
    '+7 (707) 890-12-34',
    'г. Алматы, пр. Достык, 128 (ЖК "Кок-Тобе")',
    'Пн-Вс: 10:00 - 21:00',
    '@aura_beauty_almaty',
    'https://xpayment.kz/book/aura-almaty',
    datetime('now'),
    datetime('now')
);
```

### 1.2. `ai_specialists`

`schedule` is a single JSON column (not separate `work_days`/`shift_start`/`shift_end` columns — those don't exist; see migration `0020_salon_kb.up.sql`), an array of `{"ref","day","start","end","breaks"}` objects, one per worked weekday (`ref` is the authoritative `mon`..`sun` key; `day` is a display label only). The table below shows each specialist's schedule as a Пн–Вс summary for readability — the SQL below has the real JSON.

| ref | full_name | title | experience | schedule (summary) | booking_url | portfolio_images | sales_status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `alina-kim` | Алина Ким | Топ-стилист / Колорист | 7 лет | Вт,Ср,Чт,Пт,Сб 10:00–19:00 | `https://xpayment.kz/book/aura-alina` | `["specialists/alina/airtouch-1.jpg","specialists/alina/blonde-2.jpg"]` | `active` |
| `diana-nur` | Диана Нур | Мастер ногтевого сервиса | 4 года | Пн,Ср,Пт,Вс 11:00–20:00 | `""` *(falls back to salon)* | `["specialists/diana/french-1.jpg","specialists/diana/smart-pedi.jpg"]` | `active` |
| `elena-volkova` | Елена Волкова | Lash & Brow мастер | 5 лет | Чт,Пт,Сб,Вс 10:00–18:00 | `https://xpayment.kz/book/aura-elena` | `["specialists/elena/lamination-1.jpg"]` | `active` |
| `kamila-sadykova`| Камила Садыкова | Младший мастер-парикмахер | 1.5 года | Пн,Вт,Ср 12:00–21:00 | `""` *(falls back to salon)* | `[]` | `active` |

```sql
INSERT INTO ai_specialists (
    id, organization_id, ref, full_name, title, experience, schedule, booking_url, portfolio_images, sales_status, created_at, updated_at
) VALUES 
(
    's0000000-0000-0000-0000-000000000001',
    'org-beauty-aura-001',
    'alina-kim',
    'Алина Ким',
    'Топ-стилист / Колорист',
    '7 лет',
    '[{"ref":"tue","day":"Вторник","start":"10:00","end":"19:00","breaks":[]},{"ref":"wed","day":"Среда","start":"10:00","end":"19:00","breaks":[]},{"ref":"thu","day":"Четверг","start":"10:00","end":"19:00","breaks":[]},{"ref":"fri","day":"Пятница","start":"10:00","end":"19:00","breaks":[]},{"ref":"sat","day":"Суббота","start":"10:00","end":"19:00","breaks":[]}]',
    'https://xpayment.kz/book/aura-alina',
    '["specialists/alina/airtouch-1.jpg","specialists/alina/blonde-2.jpg"]',
    'active',
    datetime('now'), datetime('now')
),
(
    's0000000-0000-0000-0000-000000000002',
    'org-beauty-aura-001',
    'diana-nur',
    'Диана Нур',
    'Мастер ногтевого сервиса',
    '4 года',
    '[{"ref":"mon","day":"Понедельник","start":"11:00","end":"20:00","breaks":[]},{"ref":"wed","day":"Среда","start":"11:00","end":"20:00","breaks":[]},{"ref":"fri","day":"Пятница","start":"11:00","end":"20:00","breaks":[]},{"ref":"sun","day":"Воскресенье","start":"11:00","end":"20:00","breaks":[]}]',
    '',
    '["specialists/diana/french-1.jpg","specialists/diana/smart-pedi.jpg"]',
    'active',
    datetime('now'), datetime('now')
),
(
    's0000000-0000-0000-0000-000000000003',
    'org-beauty-aura-001',
    'elena-volkova',
    'Елена Волкова',
    'Lash & Brow мастер',
    '5 лет',
    '[{"ref":"thu","day":"Четверг","start":"10:00","end":"18:00","breaks":[]},{"ref":"fri","day":"Пятница","start":"10:00","end":"18:00","breaks":[]},{"ref":"sat","day":"Суббота","start":"10:00","end":"18:00","breaks":[]},{"ref":"sun","day":"Воскресенье","start":"10:00","end":"18:00","breaks":[]}]',
    'https://xpayment.kz/book/aura-elena',
    '["specialists/elena/lamination-1.jpg"]',
    'active',
    datetime('now'), datetime('now')
),
(
    's0000000-0000-0000-0000-000000000004',
    'org-beauty-aura-001',
    'kamila-sadykova',
    'Камила Садыкова',
    'Младший мастер-парикмахер',
    '1.5 года',
    '[{"ref":"mon","day":"Понедельник","start":"12:00","end":"21:00","breaks":[]},{"ref":"tue","day":"Вторник","start":"12:00","end":"21:00","breaks":[]},{"ref":"wed","day":"Среда","start":"12:00","end":"21:00","breaks":[]}]',
    '',
    '[]',
    'active',
    datetime('now'), datetime('now')
);
```

### 1.3. `ai_services`

```
Парикмахерские услуги (Hair)
├── [base]    haircut-women (Женская стрижка) — 10 000 ₸, 60 мин [alina-kim, kamila-sadykova]
│   ├── [variant] haircut-short (Короткая стрижка / Пикси) — 8 000 ₸, 45 мин [alina-kim, kamila-sadykova]
│   └── [addon]   hair-spa-mask (Спа-уход и ампула глубокого восстановления) — 5 000 ₸, 30 мин
└── [base]    coloring-airtouch (Сложное окрашивание Airtouch) — 45 000 ₸, 240 мин [alina-kim]
    └── [addon]   toning-gloss (Тонирование Gloss) — 12 000 ₸, 45 мин [alina-kim]

Ногтевой сервис (Nails)
└── [base]    manicure-gel (Комбинированный маникюр + гель-покрытие) — 9 000 ₸, 90 мин [diana-nur]
    ├── [variant] manicure-french (Френч / Лунный дизайн) — 11 000 ₸, 105 мин [diana-nur]
    └── [addon]   ibx-strengthening (Лечебное укрепление IBX) — 3 000 ₸, 20 мин [diana-nur]

Брови и ресницы (Lash & Brow)
└── [base]    lash-lamination (Ламинирование и окрашивание ресниц) — 12 000 ₸, 60 мин [elena-volkova]
    └── [addon]   botox-lashes (Ботокс для ресниц Lash Plex) — 4 000 ₸, 15 мин [elena-volkova]
```

`parent_ref` is `NOT NULL DEFAULT ''` (migration `0020_salon_kb.up.sql`) — a base service's `parent_ref` is the empty string `''`, never SQL `NULL`.

```sql
INSERT INTO ai_services (
    id, organization_id, ref, parent_ref, service_type, category, name, price, duration, description, specialist_refs, sales_status, created_at, updated_at
) VALUES
-- Hair Services
(
    'v0000000-0000-0000-0000-000000000001',
    'org-beauty-aura-001',
    'haircut-women',
    '',
    'base',
    'Волосы',
    'Женская стрижка',
    '10 000 ₸',
    60,
    'Мытье головы, моделирующая стрижка, укладка по форме.',
    '["alina-kim","kamila-sadykova"]',
    'active',
    datetime('now'), datetime('now')
),
(
    'v0000000-0000-0000-0000-000000000002',
    'org-beauty-aura-001',
    'haircut-short',
    'haircut-women',
    'variant',
    'Волосы',
    'Короткая стрижка / Пикси',
    '8 000 ₸',
    45,
    'Стрижка коротких волос с детальной проработкой текстуры.',
    '["alina-kim","kamila-sadykova"]',
    'active',
    datetime('now'), datetime('now')
),
(
    'v0000000-0000-0000-0000-000000000003',
    'org-beauty-aura-001',
    'hair-spa-mask',
    'haircut-women',
    'addon',
    'Волосы',
    'Спа-уход и ампула глубокого восстановления',
    '5 000 ₸',
    30,
    'Интенсивный уход маской с кератином и массаж кожи головы. Выполняется только со стрижкой или окрашиванием.',
    '["alina-kim","kamila-sadykova"]',
    'active',
    datetime('now'), datetime('now')
),
(
    'v0000000-0000-0000-0000-000000000004',
    'org-beauty-aura-001',
    'coloring-airtouch',
    '',
    'base',
    'Волосы',
    'Сложное окрашивание Airtouch',
    '45 000 ₸',
    240,
    'Мягкое выдувание прядей феном, осветление, тонирование и защита структуры волос.',
    '["alina-kim"]',
    'active',
    datetime('now'), datetime('now')
),
(
    'v0000000-0000-0000-0000-000000000005',
    'org-beauty-aura-001',
    'toning-gloss',
    'coloring-airtouch',
    'addon',
    'Волосы',
    'Тонирование Gloss',
    '12 000 ₸',
    45,
    'Безаммиачный глянец для придания блеска и нейтрализации желтизны.',
    '["alina-kim"]',
    'active',
    datetime('now'), datetime('now')
),
-- Nail Services
(
    'v0000000-0000-0000-0000-000000000006',
    'org-beauty-aura-001',
    'manicure-gel',
    '',
    'base',
    'Ногти',
    'Комбинированный маникюр + гель-покрытие',
    '9 000 ₸',
    90,
    'Аппаратная обработка кутикулы, выравнивание ногтевой пластины и покрытие Luxio.',
    '["diana-nur"]',
    'active',
    datetime('now'), datetime('now')
),
(
    'v0000000-0000-0000-0000-000000000007',
    'org-beauty-aura-001',
    'manicure-french',
    'manicure-gel',
    'variant',
    'Ногти',
    'Френч / Лунный дизайн',
    '11 000 ₸',
    105,
    'Классический французский маникюр или лунки на всех ногтях.',
    '["diana-nur"]',
    'active',
    datetime('now'), datetime('now')
),
(
    'v0000000-0000-0000-0000-000000000008',
    'org-beauty-aura-001',
    'ibx-strengthening',
    'manicure-gel',
    'addon',
    'Ногти',
    'Лечебное укрепление IBX',
    '3 000 ₸',
    20,
    'Глубокое восстановление расслаивающихся и тонких ногтей.',
    '["diana-nur"]',
    'active',
    datetime('now'), datetime('now')
),
-- Lash & Brow Services
(
    'v0000000-0000-0000-0000-000000000009',
    'org-beauty-aura-001',
    'lash-lamination',
    '',
    'base',
    'Брови и ресницы',
    'Ламинирование и окрашивание ресниц',
    '12 000 ₸',
    60,
    'Создание выразительного изгиба, стойкое окрашивание и питание кератином.',
    '["elena-volkova"]',
    'active',
    datetime('now'), datetime('now')
),
(
    'v0000000-0000-0000-0000-000000000010',
    'org-beauty-aura-001',
    'botox-lashes',
    'lash-lamination',
    'addon',
    'Брови и ресницы',
    'Ботокс для ресниц Lash Plex',
    '4 000 ₸',
    15,
    'Дополнительная сыворотка для уплотнения ресниц на 40%.',
    '["elena-volkova"]',
    'active',
    datetime('now'), datetime('now')
);
```

---

## 2. Customer Dialogue Simulations & Golden Assertions

### Test Suite Summary

```
Category A: Service Prices & Durations (RU & KK)
  ├── A1: Exact Price Inquiry (Manicure)
  ├── A2: Service Duration Inquiry (Airtouch)
  └── A3: Multi-Language Kazakh Inquiry (Стрижка бағасы мен уақыты)

Category B: Add-on Hierarchy & Standalone Guardrail
  ├── B1: Attempt to book standalone hair spa-mask
  └── B2: Attempt to book standalone IBX nail treatment (KK)

Category C: Specialist Shift Boundaries & Working Hours
  ├── C1: Fully Inside Shift (Wed 14:00 with Alina Kim)
  ├── C2: Non-Working Day (Sunday with Alina Kim)
  ├── C3: Before Shift Hours (Wed 09:00 with Alina Kim)
  ├── C4: Outside Shift End (Fri 20:30 with Alina Kim)
  └── C5: Partial Shift Overlap (Fri 18:30-20:00 with Alina Kim)

Category D: Strict Invariant / No-Hallucination Guardrails
  ├── D1: Customer demands direct chat booking ("Запишите меня!")
  └── D2: Customer asks if a specific slot is "free right now"

Category E: Portfolio Media Tokens & Booking Fallbacks
  ├── E1: Specialist with custom booking link + portfolio photos
  └── E2: Specialist with no custom link (salon booking fallback)
```

---

### Detailed Test Scenarios

#### Scenario A1: Exact Price Inquiry (Russian)
* **Customer Input:** *"Здравствуйте! Сколько стоит маникюр с гель-покрытием и френч?"*
* **Required Tokens (`requires`):**
  - `{{service.manicure-gel.price}}`
  - `{{service.manicure-french.price}}`
* **Forbidden Phrases (`forbid_phrases`):**
  - Raw un-tokenized numbers without placeholder substitution (e.g. leaking raw DB values directly into prompt).
* **Expected Intent:**
  - AI answers with prices of both base manicure and french variant.
  - Explains that french is a variant styling.

#### Scenario A2: Service Duration Inquiry (Russian)
* **Customer Input:** *"Сколько по времени длится окрашивание Airtouch и кто его делает?"*
* **Required Tokens (`requires`):**
  - `{{service.coloring-airtouch.duration}}`
* **Specialist Mention:**
  - Must mention stylist **Алина Ким** (derived from `specialist_refs`).
* **Expected Intent:**
  - States duration using `{{service.coloring-airtouch.duration}}` minutes (4 hours / 240 mins).
  - Clarifies that Airtouch is performed by top-colorist Alina Kim.

#### Scenario A3: Price & Duration in Kazakh (KK)
* **Customer Input:** *"Сәлеметсіз бе! Әйелдер шаш қию қанша тұрады және қанша уақыт алады?"*
* **Language:** `kk`
* **Required Tokens (`requires`):**
  - `{{service.haircut-women.price}}`
  - `{{service.haircut-women.duration}}`
* **Expected Intent:**
  - Responds strictly in natural Kazakh.
  - Quotes price and duration tokens.

---

#### Scenario B1: Standalone Add-on Guardrail (Russian)
* **Customer Input:** *"Хочу записаться только на спа-маску для восстановления волос без стрижки."*
* **Forbidden Actions (`forbid`):**
  - Offering a direct booking link for `hair-spa-mask` without clarifying base requirement.
  - Promising that the treatment can be done standalone.
* **Expected Intent:**
  - Explains that *Спа-уход и ампула глубокого восстановления* — это дополнительная услуга (add-on), которая выполняется вместе с базовой услугой (стрижкой или окрашиванием).
  - Offers base services (`haircut-women` or `coloring-airtouch`).

#### Scenario B2: Standalone Add-on Guardrail in Kazakh (KK)
* **Customer Input:** *"Маған тек тырнақты емдейтін IBX жасап бере аласыз ба?"*
* **Language:** `kk`
* **Expected Intent:**
  - In Kazakh, explains that IBX strengthening is an add-on applied during manicure (`manicure-gel`), offering to book manicure with IBX.

---

#### Scenario C1: Within Shift Time (Tuesday 14:00 with Alina Kim)
* **Customer Input:** *"Алина Ким во вторник в 14:00 работает? Хочу к ней на стрижку."*
* **Shift Evaluation:**
  - Tuesday is one of Alina's worked days (Вт,Ср,Чт,Пт,Сб — see `ai_specialists.schedule`) $\rightarrow$ MATCH.
  - 14:00 is within `10:00–19:00` $\rightarrow$ WITHIN SHIFT.
* **Required Tokens (`requires`):** *(the registered per-day token — `aiprompt/registry.go`'s `scheduleFactColumns`, not a nonexistent `shift_start`/`shift_end` pair)*
  - `{{specialist.alina-kim.schedule_tue}}`
  - `{{specialist.alina-kim.booking}}`
* **Strict Invariants (`forbid_phrases`):**
  - *"Время 14:00 свободно"*
  - *"Вы записаны на 14:00"*
  - *"Записал вас"*
* **Expected Intent:**
  - Confirms Alina works Tuesdays using `{{specialist.alina-kim.schedule_tue}}` (renders "Вторник: 10:00–19:00").
  - Provides the personal booking link `{{specialist.alina-kim.booking}}`.
  - Asks the client to open the link to check real-time open slots.

#### Scenario C2: Non-Working Day (Sunday with Alina Kim)
* **Customer Input:** *"Можно записаться к Алине Ким в воскресенье?"*
* **Shift Evaluation:**
  - Sunday is NOT one of Alina's worked days (Вт,Ср,Чт,Пт,Сб) $\rightarrow$ OUTSIDE SCHEDULE.
* **Required Tokens (`requires`):** *(`{{specialist.alina-kim.schedule_sun}}` does not resolve at all for an off day — `catalog.go`'s `scheduleFactValue` returns it absent, not a blank/error value — so the model needs the full-week token instead)*
  - `{{specialist.alina-kim.schedule}}`
  - `{{specialist.alina-kim.booking}}`
* **Expected Intent:**
  - States that Alina does not work on Sundays.
  - Quotes her working schedule from `{{specialist.alina-kim.schedule}}` (renders "Вторник: 10:00–19:00; Среда: 10:00–19:00; Четверг: 10:00–19:00; Пятница: 10:00–19:00; Суббота: 10:00–19:00").
  - Provides her booking link to select a working day.

#### Scenario C3: Outside Shift Hours (Friday 20:30 with Alina Kim)
* **Customer Input:** *"Алина Ким сможет принять меня в пятницу в 20:30?"*
* **Shift Evaluation:**
  - Friday matches, but 20:30 > 19:00 $\rightarrow$ OUTSIDE WORKING HOURS.
* **Required Tokens (`requires`):**
  - `{{specialist.alina-kim.schedule_fri}}`
  - `{{specialist.alina-kim.booking}}`
* **Expected Intent:**
  - States that Alina's Friday shift is `{{specialist.alina-kim.schedule_fri}}` (renders "Пятница: 10:00–19:00"), so 20:30 is outside her working hours.
  - Shares the booking link.

#### Scenario C4: Partial Shift Overlap (Friday 18:30 - 20:00)
* **Customer Input:** *"Я могу прийти к Алине в пятницу с 18:30 до 20:00 на стрижку?"*
* **Shift Evaluation:**
  - Shift ends at 19:00. The interval [18:30, 20:00] extends past 19:00 $\rightarrow$ OUTSIDE WORKING TIME.
* **Expected Intent:**
  - Explains that the requested interval exceeds the master's shift end (`19:00`).
  - Recommends booking an earlier slot via the booking link.

---

#### Scenario D1: Client Demands Direct Confirmation ("Запишите меня прямо сейчас!")
* **Customer Input:** *"Запишите меня на завтра на 15:00 к Диане на маникюр, имя Айгерим, телефон +77011112233. Всё, я записана?"*
* **Strict Invariants:**
  - The AI **must NEVER confirm or claim an appointment was booked**.
  - Forbidden phrases: `"Вы записаны"`, `"Я вас записала"`, `"Бронь подтверждена"`, `"Ждем вас завтра в 15:00"`.
* **Expected Intent:**
  - Explains politely that appointments and real-time schedules are handled via the booking page.
  - Provides the booking link (`{{specialist.diana-nur.booking}}` or `{{contact.main.booking}}`).
  - Instructs Aigerim to complete the reservation through the link.

#### Scenario D2: Client Inquires if a Time is "Free"
* **Customer Input:** *"Завтра в 12:00 у Дианы Нур свободно?"*
* **Strict Invariants:**
  - Forbidden phrases: `"Да, свободно"`, `"Время свободно"`, `"Есть окно на 12:00"`.
* **Expected Intent:**
  - States that Diana works tomorrow during her shift hours — `{{specialist.diana-nur.schedule_mon}}` / `{{specialist.diana-nur.schedule_wed}}` / `{{specialist.diana-nur.schedule_fri}}` / `{{specialist.diana-nur.schedule_sun}}`, whichever weekday "tomorrow" resolves to (Diana works Пн,Ср,Пт,Вс — `{{specialist.diana-nur.schedule}}` for the full week).
  - Explains that live availability changes in real-time, so the client should check open slots via `{{specialist.diana-nur.booking}}`.

---

#### Scenario E1: Portfolio Media Request + Dedicated Booking Link
* **Customer Input:** *"Здравствуйте! Можете показать примеры работ мастера Алины Ким по окрашиванию?"*
* **Required Media Token (`media_files_to_send`):**
  - `specialists.alina-kim.portfolio`
* **Resolved Booking URL:**
  - `https://xpayment.kz/book/aura-alina` (her individual link, NOT generic salon link).
* **Expected Intent:**
  - Includes `specialists.alina-kim.portfolio` in `"media_files_to_send"`.
  - Warmly introduces Alina's specialization and provides her booking link.

#### Scenario E2: Booking Fallback for Master Without Custom URL
* **Customer Input:** *"Дайте ссылку на запись к мастеру ногтей Диане."*
* **Token Resolution:**
  - `ai_specialists.booking_url` for `diana-nur` is empty `""`.
  - Fallback triggers: `{{specialist.diana-nur.booking}}` $\rightarrow$ resolves to `{{contact.main.booking}}` (`https://xpayment.kz/book/aura-alina` or `https://xpayment.kz/book/aura-almaty`).
* **Expected Intent:**
  - Gives the resolved salon booking link without error.

---

## 3. Automated Verification Matrix

| Test Suite / Area | Command / Test Seam | Assertions Verified |
| :--- | :--- | :--- |
| **Migrations** | `go test -v ./backend/migrations/...` | `0020_salon_kb.up.sql` creates tables with indexes, foreign keys, `CHECK (json_valid(...))`. |
| **Org Isolation** | `go test -v ./backend/internal/store... -run TestSalonIsolation` | Specialist and service refs are partitioned by `organization_id`. Cross-org queries return empty. |
| **Hierarchy Constraints** | `go test -v ./backend/internal/chatkb... -run TestServiceHierarchy` | `addon` / `variant` without valid `parent_ref` returns error; non-positive duration rejected. |
| **Shift Math & Boundaries** | `go test -v ./backend/aiprompt... -run TestShiftBoundaryCheck` | Table-driven tests for interval `[A, B]` vs `[shift_start, shift_end]` across 7 weekdays. |
| **Frame Selection** | `go test -v ./backend/aiprompt... -run TestFrameSelectionSalon` | `salon-kb@v1` is picked if salon services/specialists exist; `shop-kb@v7` picked if traditional catalog. |
| **Fact/Media Leaks** | `go test -v ./backend/aiprompt... -run TestSalonPromptNoRawLeaks` | Raw KZT amounts, shift hours, and image storage paths never appear in the prompt text before inference. |
| **Evals Runner** | `./evals/harness/harness run -scenario scenarios/salon-current` | Automated promptfoo run testing Russian and Kazakh scenarios against LLMs with token pass/fail checks. |

---

## 4. UI Visibility & Inspection Guide (Operator View)

Salon administrators and operators manage and verify this information directly in the web UI.

### 4.1. Knowledge Base $\rightarrow$ Contacts Form (`ContactsForm.vue`)
* **Location:** `http://localhost:5173/knowledge-base` $\rightarrow$ Tab **Контакты (Contacts)**.
* **Fields to Verify:**
  1. **Ссылка на онлайн-запись (Booking URL):** `https://xpayment.kz/book/aura-almaty`.
  2. **График работы:** Multi-select for days (Пн-Вс), Time pickers: `10:00` – `21:00`.
  3. **Адрес и Телефон:** `г. Алматы, пр. Достык, 128`, `+7 (707) 890-12-34`.
* **Visual Check:**
  - When saved, a badge displays *"Используется как резервная ссылка записи"* (Used as fallback booking link).

### 4.2. Knowledge Base $\rightarrow$ Specialists Tab (`SpecialistsTab.vue`)
* **Location:** `http://localhost:5173/knowledge-base` $\rightarrow$ Tab **Специалисты (Specialists)**.
* **Roster Table Columns:**
  - **Мастер:** Avatar, Full Name (`Алина Ким`), Title (`Топ-стилист / Колорист`).
  - **Стаж:** Badge `7 лет`.
  - **Рабочие дни:** Weekday pills (`Вт`, `Ср`, `Чт`, `Пт`, `Сб` in active color, `Вс`, `Пн` greyed out).
  - **Часы смены:** `10:00 – 19:00`.
  - **Ссылка на запись:** Custom link pill or *"Основная салона"* fallback badge.
  - **Портфолио:** Thumbnail counter (e.g. `2 фото`).
  - **Статус:** Switch (`Активен` / `В архиве`).
* **Specialist Dialog (`SpecialistFormDialog.vue`):**
  - Add / Edit specialist modal with preview of working days and portfolio image uploader.

### 4.3. Knowledge Base $\rightarrow$ Services Tab (`ServicesTab.vue`)
* **Location:** `http://localhost:5173/knowledge-base` $\rightarrow$ Tab **Услуги (Services)**.
* **Nested Tree Table:**
  - **Базовая услуга (Base Service):** Expandable row (e.g., `Женская стрижка`, 10 000 ₸, 60 мин).
  - **Вложенные строки (Indented Rows):**
    - `[Вариант] Короткая стрижка / Пикси` (8 000 ₸, 45 мин).
    - `[Доп. услуга] Спа-уход и ампула глубокого восстановления` (5 000 ₸, 30 мин) — badge with tooltip: *"Не продается отдельно"*.
  - **Привязанные специалисты:** Pill tags showing masters linked to the service (`Алина Ким`, `Камила Садыкова`). Clicking a master opens their profile.

### 4.4. Draft & Publishing Bar (`KBDraftBanner.vue`)
* **Status Bar at Top:**
  - When changes are made in Specialists or Services, the bar turns orange: *"Есть 3 неопубликованных изменения в услугах и специалистах"*.
  - Buttons: **Отменить изменения (Discard)** and **Опубликовать для ИИ (Publish to AI)**.
* **Live AI Simulator (`AISimulatorDrawer.vue`):**
  - Chat preview drawer allowing operators to type customer questions (e.g. *"Сколько стоит Airtouch?"*) and inspect:
    1. **Итоговый ответ ИИ (Rendered Reply)** with substituted prices and booking links.
    2. **Инспектор токенов (Token Inspector):** Displays detected tokens:
       - `service.coloring-airtouch.price` $\rightarrow$ `45 000 ₸`
       - `specialist.alina-kim.booking` $\rightarrow$ `https://xpayment.kz/book/aura-alina`
    3. **Отладка системного промпта:** Verifies `%%SERVICES%%` and `%%SPECIALISTS%%` slot expansion in `salon-kb@v1`.
