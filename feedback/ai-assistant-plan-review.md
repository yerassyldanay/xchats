# Team-Lead Review — xchats AI Assistant Plan

**Date:** 2026-06-14
**Reviewer:** Team lead (Claude)
**Scope:** the whole `plan/` set, with focus on the AI assistant (the "suggest replies, don't auto-respond" core idea)
**Method:** read every plan doc + the vendored brain (`plan/examples/repos/xpayment-crm/`); ran a 6-lens critique (product/scope, architecture, UX, AI quality, missing-info, security/ops) and adversarially checked every finding against the actual docs. 38 raised → **23 confirmed, 14 overstated/already-covered, 0 invalid.**
**Status:** this document is the action plan. I'm implementing it.

---

## 0. Verdict

The plan is genuinely strong — better-specified than most products are *after* shipping. Normalized schema, named transport invariants (idempotent upsert, `@lid`↔phone, monotonic status), a real isolated-test strategy, and one fact that reframes everything:

> **The AI "brain" is not greenfield.** It is a tested Go service (`plan/examples/repos/xpayment-crm/`) running today against Chatwoot. This plan *ports* it (swap Chatwoot reads → Postgres reads, write to `ai_drafts`, SQLite → Postgres). The logic is reused as-is.

So the danger is **not** a flaw in what's written. It is:

1. **Three things the port silently dropped** from the source system (KB seeding, the eval/golden harness, the compliance gate).
2. **One sequencing choice** — the differentiator is validated last, sitting on top of a full inbox build.
3. **Two real correctness bugs** in the transport details (status-ID correlation, missing core fixtures).

**The key reframe:** the *stated* core idea (suggest-not-autosend) is a ~590-LOC tested port. To ship it you must first build a complete multi-account WhatsApp team inbox. **Phases 1–3 are the real project; Phase 4 is the easy part.**

---

## 1. Gates — resolve before / during the relevant phase

These are the items I will treat as hard gates. Each is cheap relative to its blast radius.

### G1 — Status correlation key is unverified and probably wrong `[transport / Phase 2]`
- **Claim in plan:** `plan/0.1-definition-of-done.md:40` correlates delivery/read status on `evo_message_id == keyId`.
- **Reality in the fixtures:** three different ID namespaces that do not line up —
  - outbound `send.message` `data.key.id` = `3EB063FA228F78C7FAE3CE` (22 chars)
  - `messages.update` `data.keyId` = `3EB06664456CEB540ACCEB31333296464363194A` (40 chars)
  - `messages.update` also carries a cuid `messageId`, matching the cuid `id` in `findMessages` records but **absent from `send.message`**.
- **Why it matters:** the join key the entire sent→delivered→read lifecycle depends on is asserted but validated by nothing. `evolution_client.py` never exercises the live `messages.update` path.
- **The captured `send_message.json` and `messages_update.json` are different conversations**, so the e2e *cannot* prove the correlation today.
- **Action:** capture ONE matched pair (a real outbound send followed by *its own* `messages.update`); confirm empirically which field correlates (most likely the cuid `messageId`). Plan to store **both** the WA `key.id` and the cuid on `xchats.messages`, index the chosen `status_correlation_id`, and replace the mismatched fixtures with the matched pair so `make test-e2e` asserts it end-to-end. Treat `0.1:40` as unverified until then.

### G2 — KB seeding + an answer-quality bar were dropped `[AI / Phase 4]`
- The brain answers **only** from the curated KB and escalates on any gap (`plan/8.4-ai-assistant-knowledge-base.md`, `plan/8.1-ai-assistant-prompt.md:50`, `plan/8.2-ai-assistant-responses.md:25`). **Empty KB ⇒ every draft is an escalation** — the product does nothing useful.
- The submodule calls "mine ~100 real chats to seed the KB and the golden set" *the load-bearing first task* (`IMPLEMENTATION.md`, `docs/07-testing-and-evals.md`). The port carries the **editing machinery** (admin UI, publish/rollback, Playground) but **not** the content-acquisition step **and not** the golden-set eval.
- Phase-4 "done" (`plan/0.1-definition-of-done.md:72`, `plan/8.6-port-checklist.md:55`) is satisfied by "a valid `ai_drafts` row" — and **an escalation IS a valid row** (`escalate bool` exists in `plan/9-database-schema.md:238`). So Phase 4 can be green while every real draft is useless.
- **Action:** (a) add a "first run = who + what KB" note naming the initial org + domain and a "seed the KB (N topics + prices + assets, ideally mined from real chats)" task; (b) add a Phase-4 DoD line: the brain must run against a **non-empty published Snapshot** and a handful of golden questions must produce **grounded, non-escalating** drafts. Also carry the submodule's `migrations/0002_seed.sql` starter snapshot into the port (see F12).

### G3 — Compliance / LLM data boundary `[go-live gate]`
- Every inbound (last ~15 messages + `contacts.attributes` profile) is sent verbatim to a foreign LLM (OpenRouter/OpenAI/Gemini) — customer PII leaving Kazakhstan.
- The submodule flags this verbatim as the **highest-priority open item, "blocks go-live."** A grep of the xchats plan for compliance/residency/PII/consent/cross-border returns **zero hits**.
- The technical exposure is inherited but the *gate* is dropped. `LLM_BASE_URL` (`plan/8.5-ai-assistant-providers.md:16`) already lets you point at an in-region/self-hosted model, so the fix is a **documented decision**, not a build.
- **Action:** add a short "LLM data boundary" note to `plan/2-architecture.md` (Security) and `plan/8-ai-assistant.md` (what leaves the boundary, the residency/legal-basis stance, whether a DPA is needed); reframe `LLM_BASE_URL` as the compliant default (point at in-region/self-host); add a Phase-4 DoD line: "lawful basis decided + documented before any real customer send." Build-and-test work is not blocked; the first real send is.

### G4 — Validate the differentiator earlier (Phase 2.5) `[sequencing]`
- The core idea is proven **last**. The AI is Phase 4, gated behind Phase 1 (foundation), Phase 2 (transport), Phase 3 (full 3-pane inbox + SSE). You won't know if suggestions are any good until after the largest, riskiest build.
- The brain is standalone and mockable (`ports.go`: "the whole pipeline runs with zero external services"; 5 `brain_test.go` cases), so an early slice is cheap.
- **Action:** add a **Phase 2.5** to `plan/0-overview.md` build order and `plan/0.1-definition-of-done.md`: once normalized messages exist (Phase 2), port just `HandleMessage` + the new Postgres Window/Profile reader and run it against `captures/` fixtures (or wire the already-designed Playground to Phase-2 rows), dumping drafts to a log / minimal view. Answers "are the suggestions useful?" weeks before the full UI.

---

## 2. Confirmed gaps (fix during the owning phase)

### Transport & architecture

**F1 — Fixtures contradict the "green suite = contract correct" claim `[Phase 2]`**
`captures/` has only 4 files and **no `messages.upsert`** (the primary inbound event the whole product is built on) and **no `getBase64FromMediaMessage` response**. Yet `plan/2-architecture.md:159,172` and `plan/6-isolated-testing.md:67` claim full contract trust. `messages_sample.json` is a `findMessages` REST shape, not a webhook envelope — not a substitute.
**Fix:** soften the trust language until fixtures exist; promote `captures/README.md`'s "expand before relying on the suite" into a hard Phase-2 prerequisite (capture: inbound `messages.upsert` text + image, the matching `getBase64` response, plus `connection.update` / `qrcode.updated` / `contacts.*`); correct `2-architecture.md:159` which lists `messages.upsert` as already captured.

**F2 — `@lid`-only events have no phone to key a conversation on, and no merge path `[Phase 2]`**
Conversations are `UNIQUE(account_id, remote_jid)` keyed on the phone JID, but `chats.upsert` in the fixtures carries **only** `remoteJid: …@lid` (no `remoteJidAlt`, no phone). The plan repeats "resolve via `remoteJidAlt`" as if every event has it; chat/contact/status events don't. There is **no documented merge/re-key path** to fold a lid-first conversation into the phone-keyed row (`lid_jid` exists as a column but its lifecycle is unspecified).
**Fix:** add a "what if a chat/contact/status event arrives before any message (lid-only)?" Q&A to `plan/3-sync.md` + a matching note in `plan/4-wa-connection-example.md`. Simplest robust policy: don't create a phone-keyed conversation from a lid-only event — either buffer/skip until a message resolves the phone, or create provisionally on `@lid` and document the re-key/merge when the first `remoteJidAlt` arrives. Add a `contact_identities` fallback lookup on the raw `@lid` value, and add the ordering fixture.

**F3 — The port is bigger than "swap the reader" in two real places `[Phase 4]`**
(ChatID being Chatwoot-int64-shaped is *not* a problem — it's an opaque passthrough; generalizing it is mechanical edge work.) The real gaps:
- **Profile/status persistence is unspecified.** The brain still emits `profile_patch` + `suggested_status`, but `xchats.ai_drafts` (`plan/9-database-schema.md:230-242`) has **no column** for either. It's undecided whether they apply immediately on draft generation or are frozen and applied on approve, and nobody is named to write `contacts.attributes` / `conversations.stage`.
- **The in-process `keyedMutex` won't survive multi-worker.** The source serializes per-contact profile writes with an in-memory mutex; under the Postgres job queue with a separate worker process (`plan/2-architecture.md:185-187`) it serializes nothing. No DB advisory lock or unique-pending-draft constraint exists (the "one suggestion per inbound" comment has no constraint behind it).
**Fix:** decide + document in `8.6` and `9-database-schema.md` how `profile_patch`/`suggested_status` persist under suggest-only (add frozen `profile_patch jsonb` / `suggested_stage text` columns and apply on approve, OR apply immediately — and name the writer). Replace the mutex with a per-conversation Postgres advisory lock and/or a `UNIQUE` partial index enforcing one pending `ai_draft` per conversation.

**F4 — Webhook account-resolution from path vs payload is unspecified `[Phase 2, low]`**
The path binds events via `{whatsapp_account_id}` (xchats uuid); payloads identify by `instance`/`instanceId` (Evolution's uuid — a different space). No documented check resolves account from path, rejects payload mismatches, or rejects unassigned accounts at the edge.
**Fix:** add ~5 lines to the webhook spec: resolve account from path; reject/ignore (log + 200) when payload `instance`/`instanceId` ≠ resolved account; reject/ignore when `organization_id IS NULL`. Keep raw storage regardless; skip enqueue on mismatch.

**F5 — Media pipeline: no `getBase64` fixture, undefined size/timeout, expiry edge `[Phase 2, low]`**
Response shape is pinned by `evolution_client.py` `cmd_download` (`base64`/`fileName`/`mimetype`), so not "unknown" — but uncaptured. Max-size/timeout deferred to `config.yaml` is fine (by design). The narrow real gap: manual retry behavior on **expired** CDN media (`.enc`) is undocumented (retry re-calls `getBase64` → Evolution re-fetches from CDN; only fails after CDN expiry, fallback = reconcile re-pull).
**Fix:** capture one real `getBase64` response (image + audio) when expanding fixtures; add a one-line clause to `3-sync.md` on retry/expiry; pin concrete `max_size`/`timeout` defaults in `config.example.yaml`.

### UX & UI — the approve loop is the product, and it's the least-specified screen

**F6 — Approve / edit ergonomics are undefined `[Phase 3/4]`**
`plan/5-ui-pages.md:30` says only "approve → sends." Unspecified: is approve one tap or an edit box? Keyboard shortcut? Does the draft drop into the *composer* (familiar) or edit inside pane 3? Do suggested media survive an edit? The API accepts `edited_text?` (`plan/7.1-endpoints.md:149`) but carries no edited-media field. These choices decide whether the loop beats just typing — the entire value prop. The submodule explicitly flags this friction; you own the composer now and can remove it.
**Fix:** add ~5 lines to `5-ui-pages.md` pane 3: one-tap unedited fast path; "Edit" loads text + media into the pane-2 composer with Enter-to-send; media stays attached unless removed; reject/ignore dismisses (`draft_state → rejected`). Decide whether media is editable in v1 and write it down.

**F7 — Stale-draft handling undefined `[Phase 4]`**
A draft is pinned to one `trigger_message_id`. If the customer sends another message before approval, nothing marks the prior draft stale — an agent can approve a now-wrong reply. `context_state` is about sync history, not "a newer inbound arrived."
**Fix:** on `message.created (direction='in')` with an open `suggested` draft, supersede the prior draft (add `draft_state 'superseded'` or `superseded_by_message_id`). UI shows only the latest active card. Cheapest sufficient form: the approve endpoint rejects a draft whose `trigger_message_id` ≠ the conversation's latest inbound, returning `DRAFT_STALE`.

**F8 — Double-approve race in a shared inbox `[Phase 4]`**
Two agents can both approve the same draft and double-send. `POST /ai-drafts/{id}/approve` declares no `CONFLICT`; there's no `ai_draft.updated` SSE to retract the card from other screens. (Presence/"X is typing" and human-composer double-send are reasonable v2 deferrals — only the AI-draft double-approve is a true correctness bug.)
**Fix:** make approve idempotent via conditional `UPDATE … WHERE draft_state='suggested'` returning `409 CONFLICT` (code already exists); add an `ai_draft.updated` SSE event carrying the new state so the first approve greys/removes the card everywhere. Both are schema-compatible; put them in Phase-4 DoD.

**F9 — Escalation / PricingError / low-confidence rendering underspecified + a data contradiction `[Phase 4]`**
`5-ui-pages.md` describes only the happy path. The data layer mostly exists (`escalate`, `escalation_reason`, `confidence`, `context_state`), so this is largely a rendering spec — **except** PricingError, where `8.2:30` ("posts a note") and `8.6:29` ("maps to draft flags") **contradict each other** and there is no `pricing_error` field anywhere.
**Fix:** add pane-3 states (escalated → show reason + "reply manually"; partial/syncing → soft note; low confidence → badge). Separately reconcile `8.2` vs `8.6` on PricingError and, if it surfaces in the UI, add the field to `ai_drafts`/`AiDraft`.

### AI / LLM quality

**F10 — No eval/golden harness (regression net) `[Phase 4]`**
The submodule fully designs a quality system (golden set, deterministic metrics — asset-ref precision/recall, price-safety, language match — LLM-as-judge, and a publish gate: media-precision ≥ 0.9, price-safety = 1.0, judge-mean ≥ 4.0). The port carries **none** of it; the port gate is purely mechanical. v1 is suggest-only (human is the safety net) and the catastrophic case (wrong price) is prevented in code (tokens-only) — so this is a regression net, not a per-reply blocker.
**Fix:** new doc `plan/8.7-ai-evals.md` porting `docs/07`; add to Phase-4 DoD a small golden set (20–40 real cases) in git + the deterministic-metric runner behind a `//go:build eval` tag (runs offline, no live LLM); make `price-safety = 1.0` + a media-precision threshold a precondition on `POST /assistant/publish`. Defer LLM-as-judge prose scoring to nightly/manual.

**F11 — Voice/media turns: promised "clarify-or-escalate" can't fire `[Phase 4]`**
`8.2` says a voice note becomes "voice message received" so the brain asks/escalates — but it never specifies the placeholder string, there's no golden case, and the inherited reader **drops empty-content messages** (`chatwoot/client.go: if m.Content == "" { continue }`). So a no-caption voice note silently vanishes from the window. In a WhatsApp market this is a frequent failure.
**Fix:** in `8.6`, require the Postgres Window reader to emit a placeholder for media-with-empty-body (`[voice note, 0:42, no transcript]` / `[image, no caption]`) from `message_media.media_kind` + `duration_ms` — do NOT inherit the drop-empty behavior. Write the exact placeholder strings in `8.2`. Add a media-only golden case. Flag transcription (`message_media.transcript` column already exists) as the cheapest large quality win if voice volume is high — but keep it out of v1.

**F12 — Auto-send gate has no confidence/golden precondition `[Phase 3 prereq, write now]`**
The source gates auto-send hard (calibrated confidence threshold + golden gate + pacing/daily-cap/quiet-hours). The port's only documented auto-send safety is "disabled while syncing" + escalation. A low-confidence-but-non-escalated draft could be auto-sent once an org flips `respond_mode=ALWAYS`.
**Fix:** in `8.6` + `0.1`, state that enabling auto-send requires (in addition to `ALWAYS` + not-syncing): `escalate=false` AND `PricingError=false`, `confidence ≥ configurable threshold`, AND the golden gate passed for the active snapshot. Note pacing/daily-cap/quiet-hours as Phase-3 prereqs. Write it now even though auto-send is deferred, so "just flip ALWAYS" can't ship unreviewed.

**F13 — Language detection fully model-driven, no deterministic fallback `[Phase 4, minor]`**
Three soft layers all interpreted by the model; `reply_language` is "observability" only and defaults to `ru`. A mis-detected reply has no catcher and can even render prices in the wrong language. The source's language behavioral test isn't ported.
**Fix:** port the language assertion (RU→ru, KK→kk, mixed→ru) into the eval set; add a cheap deterministic script-class heuristic on the latest customer message to cross-check `reply_language` and flag a mismatch on the draft (suggest-only ⇒ a logged warning is enough for v1).

**F14 — Memory window: narrow real residue only `[minor]`**
Mostly overstated (rolling summary IS reserved: `ai_summary` in schema + `3-sync.md`; profile fields *can* be overwritten on new confidence). Genuine residue: a field can be overwritten but not cleared to null.
**Fix:** add one line to `8.3` noting the never-null limitation with the human-editable profile panel as the v1 escape hatch; add a golden category for "fact stated >15 turns back" as a regression target for the eventual summary work. Implementing `ai_summary` stays deferred.

### Missing info / decisions to capture

**F15 — No success metric ("is the assistant good?") `[decide now — schema-level]`**
No doc defines quality. The natural primary is **draft-acceptance rate** — the exact signal the source names as the gate for future auto-send. `ai_drafts` does **not** store the final sent text, so edit-distance/acceptance is unrecoverable later.
**Fix:** decide the v1 metric (acceptance rate). Capture the final sent text on the `ai_drafts` row (or link the sent message) **now** so it's computable from day one. State that the full golden/judge harness is deferred (don't drop it silently).

**F16 — Group chats deferred without a guard `[Phase 2/4]`**
`3-sync.md` leaves "disable or store separately" open. The rest of the design hard-assumes 1 contact/conversation; the `ai_draft` worker has no group guard, so a `@g.us` message would produce a nonsensical single-contact draft. No `conversation_type` column exists.
**Fix:** close the "or" in `3-sync.md`. Cheapest safe default: drop `@g.us` events at the webhook/normalizer before upsert; add a matching line to `8.6` ("`process_event`/`ai_draft` never runs for group JIDs"). "Store but never draft" needs an `is_group` column — confirm before choosing.

**F17 — LLM-failure behavior undefined `[Phase 4, minor]`**
Largely covered (`AI_UNAVAILABLE` 503 exists; `ai_draft` is a normal retryable job; the human composer is always present). The one thing to write: the exact on-failure behavior.
**Fix:** one sentence in `8.6`/`8.2`: on LLM fail/timeout the job fails gracefully, no draft row (or one with an error flag), the human sees no recommendation and replies normally; decide whether to surface "couldn't suggest" in the inbox or only the admin/debug screen.

**F18 — First customer/persona unstated `[clarify]`**
No doc names the first pilot org or what its KB covers. (The governance half — "any member can rewrite live prices" — is *already* a documented deferral via `5-ui-pages.md:60`, mitigated by `ai_audit_log` + publish/rollback.)
**Fix:** name the first pilot/dogfood org (likely the owner's own business) so the persona is concrete; one-line confirm "equal permissions accepted for config publish in v1."

### Security / ops

**F19 — Ops gaps: backup/DR, metrics contents, escalation surfacing `[minor]`**
Postgres is the single source of truth for product state + queue + AI config, but there's no backup/DR line. `/metrics` is listed with no contents. An unanswered escalation has no surfacing — in a suggest-only product, a dropped escalation = a customer silently waiting.
**Fix:** short ops section: nightly `pg_dump` to object storage + restore-test per release; an explicit `/metrics` list (webhook-auth-rejection rate, queue depth, job failures, send failures, LLM error/latency); an inbox filter/badge for "escalated / needs human" reusing `draft_state` + `escalate`.

**F20 — Data lifecycle: retention/erasure/encryption unspecified `[low]`**
Full message content stored twice (`messages.raw` + `evolution_events.payload`), kept indefinitely; no retention, hard-delete, erasure, or encryption-at-rest note. Schema is erasure-friendly (FK cascades), so this is additive later.
**Fix:** one-paragraph data-lifecycle note: TTL for the audit-only `evolution_events.payload` / `messages.raw`; a documented hard-delete-by-contact procedure; one line that encryption-at-rest is deployment-provided + media store enforces access control; whether raw payloads need PII redaction. Fine to defer if listed alongside the other v1 deferrals.

**F21 — Webhook token hardening `[low — right-size it]`**
Single shared static token, header OR query param, no HMAC/rotation. Deliberate v1 tradeoff and same-host topology blunts it, but cheap wins exist.
**Fix:** make the token header-only (drop the query-param option so it never lands in logs); reject fast at the edge if path account is unassigned (turn the downstream "ignore unassigned" into an edge 401/404 — overlaps F4); add a one-line rotation note. Full per-account-token + HMAC + IP-allowlist is **not** warranted for v1.

**F22 — Auth contract inconsistency: orphan `FORBIDDEN` `[low]`**
`FORBIDDEN`/403 is a first-class error with a UI handling rule, but no endpoint produces it and no permission model exists. (Flat permissions are intentional — don't build RBAC.)
**Fix (pick one):** remove `FORBIDDEN`/403 from `7-api-contracts.md`, OR (preferred, additive) add a single `is_admin` boolean to `xchats.members` and gate just two surfaces — `POST /users` and assistant config `PUT`/`publish` (the customer-facing persona/prices) — marking those endpoints' errors with `FORBIDDEN`.

**F23 — Session/password hardening underspecified `[low]`**
New greenfield member auth surface; cookie flags, session TTL, hash algorithm, CSRF, login throttling, and `SESSION_SECRET` (absent from the `.env` catalog) all unspecified. Note: the brain hashes its admin password with **sha256** — do NOT inherit that.
**Fix:** one-line auth-hardening note in `2-architecture.md`: cookie flags (HttpOnly, SameSite=Lax, Secure in prod); name the hash (argon2id/bcrypt — explicitly not sha256); a min password-length floor; CSRF strategy; login throttling. Add `SESSION_SECRET` to the `.env` catalog.

---

## 3. Overstated — do NOT over-invest in these

Adversarial verification downgraded these. Capturing them so we don't burn v1 effort:

- **"Transport is riskiest but plan calls AI the core."** True literally, but the plan already builds AI last and invests the most design depth in `3-sync.md`. → Just add **one** risk-callout line to `0-overview.md` ("highest-risk v1 work is WhatsApp transport+sync; the AI brain is a vendored, tested port").
- **Naming drift** (`sender_kind`/`sender_type`, `respond_mode`/`auto_response_mode`). → Doc hygiene. The doc the implementer follows (`8.6`) already uses the authoritative names. One-line sweep aligning to `9-database-schema.md`; reconcile `4-wa-connection-example.md`'s stale `users`→`members` while there.
- **No reject/regenerate path.** → "Ignore" is the accepted no-op. Either drop `'rejected'` from the enum until a writer exists, or add a thin `/reject`. Defer `/regenerate` (conflicts with "one suggestion per inbound").
- **Config page too heavy.** → The submodule ships `migrations/0002_seed.sql` (a working starter snapshot). Just add it to the `8.6` "Adapt" list so first boot is "edit a working example," not a blank form.
- **Cost ceiling / per-org budget.** → Single-org v1; output already capped (`LLM_MAX_TOKENS=1024`); default model (`anthropic/claude-sonnet-4`) ships with the port. Only worthwhile add: a single instance-level daily draft-count/token cap so a webhook storm can't run up calls.
- **"No authz" broad framing.** → Flat permissions are deliberate. Do F22 (the narrow real bit) only.
- **Server-rendered admin templates "reuse as-is."** → Contradicts the SPA + JSON-API the plan already commits to (`7.1` defines `/assistant/{config,publish,playground}`). Move the HTML templates/HTTP-admin out of "reuse as-is" into drop/adapt; keep the admin use-case/service layer, expose via the existing JSON endpoints consumed by the Vue page. (Correct "20 templates" → 11.)
- **Reference screenshot shows deferred features.** → Add one sentence noting `ui-chatboard.png` is aspirational and that multiple variants / `Шаблоны` macros / payment-booking action widgets are NOT v1. Do **not** trim the Profile/intent/next-step block — that's real ported brain output.

---

## 4. Clarifying questions for the owner

1. **First pilot org + KB domain** — who, and what does the KB cover? (drives G2/F18)
2. **LLM provider/region + lawful basis** — in-region/self-host via `LLM_BASE_URL`, or accept cross-border with documented consent? (G3)
3. **v1 success metric** — confirm draft-acceptance rate as primary; OK to store final sent text on the draft now? (F15)
4. **Group chats** — drop at webhook (default), or store-but-never-draft (needs a column)? (F16)
5. **Cutover sequencing** — the brain runs on Chatwoot today. Ship the xchats inbox first to replace Chatwoot and keep the brain on its current rails briefly, or cut over both at once? (consolidation strategy)
6. **AI config publish** — confirm "equal permissions, accepted risk," or gate publish behind `is_admin` (F22)?

---

## 5. Implementation plan (doc edits — no code yet, per high-level-first)

Ordered. Each is a doc change unless noted.

| # | Edit | Files |
|---|------|-------|
| 1 | Add **Phase 2.5** (early draft-quality slice) to build order + DoD | `plan/0-overview.md`, `plan/0.1-definition-of-done.md` |
| 2 | Add risk-callout line (transport is the long pole; AI is a port) | `plan/0-overview.md` |
| 3 | Status-correlation: mark `evo_message_id == keyId` unverified; require a matched send→update capture; plan to store both IDs | `plan/0.1-definition-of-done.md`, `plan/9-database-schema.md`, `plan/4-wa-connection-example.md`, `plan/captures/README.md` |
| 4 | Soften "green suite = correct"; make fixture expansion a hard Phase-2 prereq; fix the `messages.upsert`-already-captured overstatement | `plan/2-architecture.md`, `plan/6-isolated-testing.md`, `plan/0.1-definition-of-done.md`, `plan/captures/README.md` |
| 5 | Lid-only event ordering + merge/re-key policy | `plan/3-sync.md`, `plan/4-wa-connection-example.md` |
| 6 | Profile/status persistence columns + concurrency guard (advisory lock / unique pending draft) | `plan/8.6-port-checklist.md`, `plan/9-database-schema.md` |
| 7 | Webhook account-resolution + edge reject-unassigned + header-only token + rotation note | `plan/3-sync.md`, `plan/7.1-endpoints.md`, `plan/2-architecture.md` |
| 8 | KB seeding ("who + what KB" + mine-chats task) + Phase-4 quality DoD line + carry `0002_seed.sql` | `plan/0.1-definition-of-done.md`, `plan/8.4-ai-assistant-knowledge-base.md`, `plan/8.6-port-checklist.md` |
| 9 | New eval doc + offline deterministic metric runner + publish gate | **new** `plan/8.7-ai-evals.md`, `plan/0.1-definition-of-done.md`, `plan/7.1-endpoints.md` |
| 10 | Compliance / LLM data-boundary note + Phase-4 go-live gate | `plan/2-architecture.md`, `plan/8-ai-assistant.md`, `plan/8.5-ai-assistant-providers.md`, `plan/0.1-definition-of-done.md` |
| 11 | Approve/edit ergonomics + stale-draft + double-approve (idempotent approve, `ai_draft.updated` SSE) + escalation/PricingError rendering | `plan/5-ui-pages.md`, `plan/7.1-endpoints.md`, `plan/9-database-schema.md`, `plan/8.2-ai-assistant-responses.md`, `plan/8.6-port-checklist.md` |
| 12 | Voice/media placeholder (no drop-empty) + golden case | `plan/8.2-ai-assistant-responses.md`, `plan/8.6-port-checklist.md` |
| 13 | Auto-send confidence/golden gate (write now, deferred feature) | `plan/8.6-port-checklist.md`, `plan/0.1-definition-of-done.md` |
| 14 | Success metric: store final sent text on the draft | `plan/9-database-schema.md`, `plan/0.1-definition-of-done.md` |
| 15 | Group-chat guard (drop `@g.us` at webhook) | `plan/3-sync.md`, `plan/8.6-port-checklist.md` |
| 16 | Ops: backup/DR, `/metrics` contents, escalation surfacing | `plan/2-architecture.md`, `plan/0.1-definition-of-done.md` |
| 17 | Data-lifecycle note (retention/erasure/encryption) | `plan/2-architecture.md` or `plan/9-database-schema.md` |
| 18 | Auth hardening + `SESSION_SECRET` + `is_admin` gate / `FORBIDDEN` fix | `plan/2-architecture.md`, `plan/9-database-schema.md`, `plan/7.1-endpoints.md`, `plan/7-api-contracts.md` |
| 19 | Language: port assertion + deterministic cross-check | `plan/8.6-port-checklist.md`, `plan/8.7-ai-evals.md` |
| 20 | Hygiene sweep: naming drift, server-rendered admin templates → drop/adapt, screenshot "aspirational" note, reject-enum, instance-cap | `plan/3-sync.md`, `plan/4-wa-connection-example.md`, `plan/7.1-endpoints.md`, `plan/8.6-port-checklist.md`, `plan/5-ui-pages.md` |

**Suggested order of execution:** items 1–4 (sequencing + the two transport correctness items) → 8–10 (the dropped-port trio: KB, eval, compliance) → 11–12 (the approve loop = the product) → the rest. Hygiene (20) last.
