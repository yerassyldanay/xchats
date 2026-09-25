# Minimal Beauty-Salon KB Extension

## Summary

Implement the salon vertical strictly as a knowledge-base extension:

1. Extend `ai_contacts`.
2. Add `ai_specialists` and `ai_services`.
3. Render salon prompt blocks.
4. Register deterministic fact and media tokens.
5. Extend existing KB APIs, draft import, and MCP tools.

Do not change channels, inbox, debounce, automation, draft dispatch, SSE, CRM, or the response JSON contract.

Assessment: 9/10 for the agreed scope. Date-based slot expiry and automatic portfolio attachment would be needed for 10/10, but both are explicitly deferred.

## Data Model and Authoring

Create additive SQLite migration `0020_salon_kb.up.sql`.

### `ai_contacts`

Add:

- `booking_url TEXT NOT NULL DEFAULT ''`
- `available_spots TEXT NOT NULL DEFAULT ''`

Tokens:

- `{{contact.main.booking}}`
- `{{contact.main.available_spots}}`

### `ai_specialists`

Use repository conventions: UUID `id` primary key plus `UNIQUE (organization_id, ref)`, not a globally unique `ref`.

Fields:

- `id`, `organization_id`, `ref`
- `name`, `title`, `experience`
- `work_days`, `shift_start`, `shift_end`
- `available_spots`, `booking_url`
- `portfolio_images TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(portfolio_images))`
- `sales_status`, `created_at`, `updated_at`

### `ai_services`

Fields:

- `id`, `organization_id`, `ref`
- `parent_ref` nullable
- `service_type`: `base | variant | addon`
- `category`, `name`, `price`
- `duration INTEGER` representing minutes
- `description`
- `specialist_refs TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(specialist_refs))`
- `sales_status`, `created_at`, `updated_at`

Validation:

- Refs follow the existing pattern `[a-z0-9][a-z0-9_-]*`; dots are forbidden because dots separate token segments. Use `manicure-french`, not `manicure.french`.
- Base services have no parent.
- Variants and add-ons require an existing base parent in the same organization.
- Only one hierarchy level is allowed.
- Add-ons cannot be presented as standalone services.
- Every `specialist_ref` must resolve to an active specialist in the same organization.
- Duration must be positive when present.
- Specialist deletion is rejected while an active service references that specialist.

Extend the existing live/draft KB views, import synthesis, and cache loading with `specialists` and `services`. Add matching REST and MCP operations:

- `kb_specialist_upsert`, `kb_specialist_delete`
- `kb_service_upsert`, `kb_service_delete`
- Existing KB read/search tools recognize both new entity types.

No Vue forms are added; administrators use existing KB API/import/MCP workflows.

## Prompt and Token Implementation

Add `aiprompt.Specialist` and `aiprompt.Service` types, extend `Contacts`, `KB`, and `PromptInput`, and load the new rows through the existing repository.

Add a versioned `salon-kb@v1` frame with `%%SERVICES%%` and `%%SPECIALISTS%%` slots. Select it automatically when contacts contain salon booking/availability data or the KB has specialists/services; otherwise retain `shop-kb@v7` unchanged.

Register facts:

- `contact.main.booking`
- `contact.main.available_spots`
- `specialist.<ref>.work_days`
- `specialist.<ref>.shift_start`
- `specialist.<ref>.shift_end`
- `specialist.<ref>.available_spots`
- `specialist.<ref>.booking`
- `service.<ref>.price`
- `service.<ref>.duration`

Register media token:

- `specialists.<ref>.portfolio`

The media token maps internally to `portfolio_images`; material IDs and storage locations never enter the prompt.

Resolution rules:

- `specialist.<ref>.booking` resolves to the specialist URL when present, otherwise to `contact.main.booking`.
- If neither URL exists, the token is absent and any attempted use fails closed.
- Specialist availability never falls back to general salon availability.
- Blank exact fields produce no token.
- Prices, durations, schedules, URLs, and available spots appear only as placeholders in the prompt.
- Service and specialist names/descriptions may appear as trusted prose.
- `duration` resolves as an integer; the model adds the appropriate minutes unit.

Prompt rules:

- Quote available spots only through the corresponding token.
- Do not infer an unlisted time, alternative slot, or appointment confirmation.
- Direct booking requests to the resolved booking URL.
- Never state that the client has been booked.
- Ask for the base service when an add-on is requested alone.
- Match specialists to services only through validated `specialist_refs`.
- Keep the existing response schema and language behavior unchanged.

## Test Plan and Assumptions

Tests:

- Migration and schema-contract tests for both tables and contact columns.
- Organization isolation, ref validation, JSON-array validation, parent rules, and specialist-reference validation.
- Live and staged KB CRUD, publish, delete, import, and MCP tests.
- Prompt snapshots for service hierarchy, specialist roster, schedules, spots, booking links, and empty fields.
- Catalog/substitution tests for every new token.
- Booking fallback tests: specialist URL, salon fallback, and both missing.
- Media-catalog tests for valid, missing, cross-organization, and wrong-type portfolio materials.
- Regression tests proving non-salon organizations still render the byte-pinned shop frame.
- Eval cases for prices, durations, add-ons, specialist schedules, listed/unlisted spots, portfolio requests, and booking-link handoff.

Assumptions:

- V1 supports one salon location per organization; the earlier multi-branch design is dropped.
- Available spots are manually maintained text with no date or expiry enforcement. The stored value remains authoritative until an operator updates or clears it.
- No calendar synchronization or appointment creation is included.
- No frontend changes are included.
- The existing RU/KK response contract remains unchanged; the earlier English expansion is deferred.
- Portfolio tokens are registered and validated, but automatic attachment delivery is not added: the current response engine validates `media_files_to_send` and then drops the resolved media before draft persistence. Changing that would require the reply-dispatch work explicitly excluded from this scope.
