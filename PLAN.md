# Beauty Salon Knowledge Base Extension

## Summary

Extend xchats with structured salon schedules, specialists, services, booking links, and native Knowledge Base management.

The booking page remains the only source of live availability. Remove `available_spots` completely. Do not change channel integrations, automation, inbox behavior, SSE, or appointment dispatch.

## Key Changes

### 1. Data model and validation

Create additive migration `0020_salon_kb.up.sql`.

Extend `ai_contacts` with:

- `booking_url TEXT NOT NULL DEFAULT ''`
- `schedule TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(schedule))`

Preserve the existing `working_hours` column and token for non-salon compatibility. Do not parse or backfill its free-text value into `schedule`. The structured schedule is authoritative in the salon prompt.

Create `ai_specialists` with:

- `id`, `organization_id`, unique organization-scoped `ref`
- `full_name`, `title`, `experience`
- `schedule`
- `booking_url`
- `portfolio_images` as a JSON UUID array
- `sales_status`, `created_at`, `updated_at`

Create `ai_services` with:

- `id`, `organization_id`, unique organization-scoped `ref`
- `parent_ref`
- `service_type`: `base`, `variant`, or `addon`
- `category`, `name`, `price`, `duration`, `description`
- `specialist_refs` as a JSON ref array
- `sales_status`, `created_at`, `updated_at`

Use this shared schedule contract:

```ts
type WeekdayRef = 'mon' | 'tue' | 'wed' | 'thu' | 'fri' | 'sat' | 'sun'

interface ScheduleBreak {
  start: string // HH:MM
  end: string   // HH:MM
}

interface ScheduleDay {
  ref: WeekdayRef
  day: string
  start: string
  end: string
  breaks: ScheduleBreak[]
}

type Schedule = ScheduleDay[]
```

Validation rules:

- A weekday may occur only once; missing weekdays are off-days.
- Times use valid 24-hour `HH:MM` values.
- V1 supports one work interval per day and multiple breaks.
- Overnight shifts are rejected.
- Every break must be inside the shift; breaks cannot overlap.
- Store weekdays and breaks in canonical chronological order.
- `ref` is authoritative for schedule logic; `day` is display/import metadata.
- Service refs use lowercase dash-separated slugs such as `manicure-french`.
- Base services have no parent; variants and add-ons require a base parent from the same organization.
- Only one parent-child level is supported.
- Duration is either null or a positive minute count.
- Specialist references must belong to the same organization.
- Archiving a base service atomically archives its active children; restoring children is explicit.
- Archiving a specialist preserves service relationships but removes that specialist from prompt-visible associations.

### 2. KB interfaces

Extend live views, draft views, imports, schema contracts, cached KB loading, and read/search projections with specialists and services.

Add live endpoints:

- `POST /kb/specialists`
- `POST /kb/services`
- `PATCH /kb/specialists/:ref/status`
- `PATCH /kb/services/:ref/status`
- Extend `PATCH /kb/contacts` with `booking_url` and `schedule`

Add equivalent `/playground/draft/...` writes so imports and MCP continue using the staged-review workflow. New entities use `sales_status=inactive` for archival; no hard-delete UI is introduced.

Add MCP operations:

- `kb_specialist_upsert`
- `kb_service_upsert`

Both accept `sales_status`, allowing archive and restore operations without destructive deletion.

### 3. Prompt, tokens, and booking behavior

Add `salon-kb@v1` with service-tree and specialist-roster blocks. Select it when structured salon data exists; organizations without salon data continue using the existing shop frame unchanged.

Render:

- Active services grouped by category, with variants and add-ons nested under their base service.
- Active specialists with `full_name`, title, experience, normalized schedules, service associations, booking handoff, and portfolio availability.
- A machine-readable schedule reasoning section containing weekday, shift, and break boundaries.

Register exact-value tokens:

- `{{contact.main.booking}}`
- `{{contact.main.schedule}}`
- `{{contact.main.schedule_mon}}` through `schedule_sun`
- `{{specialist.<ref>.booking}}`
- `{{specialist.<ref>.schedule}}`
- `{{specialist.<ref>.schedule_mon}}` through `schedule_sun`
- `{{service.<ref>.price}}`
- `{{service.<ref>.duration}}`

A daily schedule token resolves to localized customer-ready wording containing the shift and breaks. The full schedule token resolves the complete working week. Only scheduled days receive daily tokens.

Register media token:

- `specialists.<ref>.portfolio` → `portfolio_images`

Booking resolution:

- Use the specialist's link when present.
- Otherwise fall back to `{{contact.main.booking}}`.
- If neither link exists, omit the token and escalate when booking handoff is required.
- Re-read the current row during substitution and reject stale schedule tokens when the normalized schedule changed after prompt construction.
- Extend literal-leak validation to reject model-authored schedule times, including individual shift and break boundaries.

Schedule reasoning uses half-open intervals:

- An interval `[A,B)` is on-shift when `A < B`, `shift.start <= A`, `B <= shift.end`, and it does not overlap any break.
- Break overlap is `A < break.end && B > break.start`.
- A point time is on-shift when `start <= time < end` and it is outside every break.
- Partial shift or break overlap is not a valid complete interval.
- Ambiguous day, time, specialist, or interval produces a clarification question.

Response rules:

- On-shift: state only that the specialist is working then, include the relevant daily schedule token, provide the booking token, and direct the customer to the booking page.
- Break overlap: state that the requested period intersects a scheduled break, include the daily schedule token showing the working periods, and provide the booking token.
- Off-day/outside shift: state that the specialist is not working for the requested period, include the full or relevant daily schedule token, and provide the booking token.
- Never claim that a time is free, reserve a slot, or confirm an appointment.
- Remove every `available_spots` field, token, prompt instruction, fixture, and test.

### 4. Native Knowledge Base UI

Use Vue 3 Composition API with typed `<script setup>` components and extend the existing `playground` Pinia store rather than introducing a parallel salon store.

- Extend `ContactsForm.vue` with booking URL and a shared seven-day `ScheduleEditor.vue`. Retain the legacy `working_hours` input for existing non-salon tenants.
- Add `SpecialistsTab.vue` and `SpecialistFormDialog.vue` with add, edit, archive, restore, portfolio media picker, and per-day shift/break editing.
- Add `ServicesTab.vue` and `ServiceFormDialog.vue` with category grouping, base-service trees, parent selection, service type, price, duration, description, and active-specialist selection.
- Add always-visible Specialists and Services tabs to `KnowledgeBase.vue`.
- Default lists to active records with a "show archived" toggle.
- Keep refs immutable after creation.
- Derive service trees with computed state rather than mutating store rows.
- Extend draft review rendering so staged salon changes from imports or MCP can be reviewed and published.
- Add RU/KK labels, validation messages, empty states, and archive confirmations.

## Test Plan

- Migration, schema-contract, JSON validity, organization isolation, and default-value tests.
- Schedule validation tests for duplicate days, malformed times, overnight shifts, invalid breaks, overlap, ordering, and empty schedules.
- Service hierarchy, parent status, archive cascade, duration, and specialist-reference tests.
- Live API, draft approval, import, MCP, archive, restore, and cache-invalidation tests.
- Prompt snapshots for service trees, specialist rosters, schedules, breaks, booking fallback, and portfolio tokens.
- Token substitution and stale-schedule rejection tests.
- Working-time scenarios covering off-days, exact boundaries, partial overlap, point times, break start/end, break overlap, missing links, and ambiguous requests.
- Generation evaluations asserting that replies never claim live availability or confirm appointments.
- Vue DOM tests for contact schedules, specialist CRUD/archive, portfolio selection, service trees, parent filtering, specialist selection, and validation errors.
- Regression tests proving organizations without salon data retain the existing prompt and KB behavior.

## Assumptions and Defaults

- V1 supports one salon location per organization.
- Weekly schedules repeat; dated exceptions, holidays, time zones, multiple shifts per day, and overnight work are deferred.
- The external booking page is the only authoritative live calendar.
- No calendar synchronization or appointment creation is included.
- Existing RU/KK reply behavior remains unchanged; customer-facing weekday labels are rendered from `ref`.
- Existing messaging, automation, inbox, SSE, CRM, and response-dispatch code remains untouched.
- Portfolio media is stored, selectable, and registered in the media catalog; changes to automatic media delivery remain outside this scope.
