# Per-account auto-response

This document describes two coupled features on `/accounts`: debouncing the
draft pipeline, and an optional per-account scheduled auto-reply that fires
when nobody acts on a draft in time. `architecture.md` and the canonical
database schema remain authoritative for everything outside this feature.

## 1. Scope

An operator can, per account (WhatsApp or Telegram — the feature is
channel-neutral):

- define a weekly schedule of when auto-response is active, in the
  account's own timezone;
- pick a delay: how long an armed reply waits before sending, giving an
  operator a window to act on the suggestion first;
- choose what sends — the AI-suggested draft, or a fixed canned text;
- opt out when the AI escalated, or when the chat is already assigned.

Everything is off by default (`autoresponse.DefaultPolicy()`), so an account
with no configured row behaves exactly as before this feature existed.

Alongside it, inbound message bursts are debounced into a single draft
generation instead of one LLM call per message — unrelated to scheduling,
but it shares the same "durable row, not just a queue message" shape and the
same worker sweeper, so it lives in this document too.

## 2. Debounce

Before this feature, every inbound message published a `KindAIDraft` task
immediately. A customer typing three messages in a row produced three LLM
calls and three drafts flashing in the panel in quick succession.

`chat_draft_debounce` is one row per chat: `generate_after` is pushed forward
(capped) on every new inbound message, and only the FIRST message of a burst
survives as `trigger_message_id`. A draft actually generates once the chat
has been quiet for `quietPeriod` (5s) or `debounceCap` (60s) has elapsed,
whichever comes first.

Two mechanisms keep this working under both normal operation and a crash:

- an in-process `time.AfterFunc` timer per chat (`worker.Worker.debounceTimers`)
  fires promptly — a latency optimization only;
- the periodic sweeper (`RunDebounceSweep`) claims any row whose
  `generate_after` has passed and no timer is tracking — the row is the
  source of truth, so a process restart mid-burst still generates exactly
  one draft instead of zero.

`KindDebounceTouch` replaces `KindAIDraft` at the three inbound sites:
`worker.go`'s WhatsApp event handler (excluding Evolution's `messages.set`
history-backfill event, which is not a live customer message),
`telegram_webhook.go`'s new-message handler and `reenqueueMissingDraft`, and
`simulator.go`'s async branch. `handleSuggest` (an operator explicitly
asking for a fresh draft) and the simulator's synchronous branch
(`wait_for_response=true`) go straight to `KindAIDraft` — debouncing an
explicit, already-singular request would only add latency.

`WriteDraftSet` takes the same chat advisory lock (`lockChat`, see below) as
its first statement — without it, two debounce timers racing for the same
chat could both pass the "is there already a suggested draft" check and
collide on `ai_drafts_pending_uq` (SQLSTATE 23505).

## 3. Data model

Four tables, all under `xchats.`, added in migration `0017`:

```
chat_draft_debounce            -- one row per chat, described above
account_auto_response          -- one row per account: the policy scalars
account_auto_response_window   -- N rows per account: the weekly schedule
auto_response_jobs             -- the durable, claimable work item
```

`account_auto_response` and `auto_response_jobs` carry `account_id`/`chat_id`
as loose UUID references with no foreign key — those ids span `wa_accounts`/
`tg_accounts` and `wa_chats`/`tg_chats`, so there is no single parent table
to FK into. This is the same shape migration `0013` already gave `ai_drafts`.

`account_auto_response_window` stores only non-wrapping, same-weekday,
half-open `[starts_minute, ends_minute)` rows — `internal/autoresponse.Normalize`
is what guarantees a wrapping input ("18:00 to 09:00") lands as two rows,
never one row that spans midnight into the next weekday.

Two partial unique indexes on `auto_response_jobs` carry the correctness of
the whole feature:

- `auto_response_jobs_chat_pending_uq` on `(chat_id) WHERE state = 'pending'`
  — at most one pending job per chat. A newer debounced draft re-targets
  this same row (`ArmAutoResponseJob`'s `ON CONFLICT`) instead of piling up
  a duplicate.
- `auto_response_jobs_trigger_uq` on `(trigger_message_id) WHERE state IN
  ('pending', 'claimed', 'sent')` — **the** database-enforced invariant: one
  inbound message produces at most one auto-reply, ever. `claimed`/`sent`
  are included, not just `pending`, so a job that already fired (or is
  mid-send) still fences a redelivery of the same trigger message.

## 4. Scheduling semantics (`internal/autoresponse`)

A leaf package: no DB, no HTTP, so it is exhaustively unit tested on its own
(`autoresponse_test.go`, including a 168-hour brute-force coverage check
against a worked example schedule).

- **Weekday convention**: Go's `time.Weekday` (`Sunday = 0 .. Saturday = 6`),
  matching Postgres `EXTRACT(DOW ...)` exactly — never ISO's `Monday = 1`.
  Every layer (SQL, Go, TypeScript) uses this same convention; a silent
  switch anywhere would put schedules on the wrong day.
- **`Normalize`** never rejects a wrapping window; it splits a same-weekday
  wrap `[start, end)` with `start >= end` into `[0, end)` + `[start, 1440)`,
  sorts by `(weekday, start)`, and merges overlapping/touching intervals.
  Repeated edits are idempotent, never an accumulating mess.
- **`Covers(at, loc, windows)`** is the DST-safe membership test: it converts
  the instant `at` to local wall-clock via `at.In(loc)` — always
  well-defined for a real instant — and reads off weekday + minute-of-day.
  It never constructs a wall-clock time from components, which is the
  direction that is ambiguous (or invalid) across a DST transition.
- **`ParseClock`/`FormatClock`** handle `"HH:MM"`, with `"24:00"` as the one
  special case meaning a full day (`1440`) — `time.Parse("15:04", "24:00")`
  itself rejects this, and a Postgres `time` column cannot represent it at
  all, which is why the window table stores plain `smallint` minutes
  instead.

## 5. Arming

Arming (creating or re-targeting a chat's pending job) happens inside
`worker.handleAIDraft`, once a debounced draft is generated and only for the
newest (lowest-ordinal) draft in that batch — never inside `handleSuggest`,
and never for the simulator's synchronous branch. `armAutoResponse` runs a
guard chain in order, each one failing closed:

1. account has no organization → skip;
2. no configured policy, or `policy.Enabled == false` → skip;
3. the draft has no trigger message → skip;
4. the trigger message's provider timestamp is missing/zero → skip (never
   defaults to "now" — that would auto-arm on any inbound with no
   timestamp, provider glitch or not);
5. the trigger is older than `maxTriggerAge` (15 minutes) → skip (a
   redelivered/backfilled old message must not arm a fresh timer);
6. the policy's timezone fails to load → skip (never silently falls back to
   UTC);
7. `autoresponse.Covers` says the trigger's arrival, in local time, is
   outside every window → skip.

Only once all seven pass does `ArmAutoResponseJob` run.

## 6. Firing

One shared goroutine (`Worker.StartSweeper`) does, on a timer (`sweepEvery`,
10s) and once at startup: recover due debounce rows, claim and fire due
auto-response jobs, reap stale claims, and purge old terminal rows.

`ClaimDueAutoResponseJobs` claims pending jobs whose `due_at` has passed via
`FOR UPDATE SKIP LOCKED` — safe with more than one worker process. A job
whose `due_at` is older than `maxDeliveryLag` (10 minutes) is cancelled
(`too_late`) in the same statement instead of claimed, so a deploy that was
down for an hour does not wake up and flush a backlog of stale replies.

Each claimed job runs `fireAutoResponseJob`'s guard chain, in order — every
failure is a terminal cancel with a specific `cancel_reason`, and **nothing
here is ever retried**: a cancelled or failed job just leaves the suggestion
sitting in the panel for an operator, which is the fail-safe direction.

```
account_gone      -- account missing, or its org no longer matches the job's
policy_off        -- policy deleted or disabled since arming
account_gone      -- chat missing (reuses the same reason)
assigned          -- pause_when_assigned and the chat has an assignee
cooldown          -- an outbound (operator or otherwise) landed within CooldownSeconds
draft_stale       -- job has no draft_id
draft_stale       -- the draft itself is gone
escalated         -- see below
draft_stale       -- (ai mode only) draft is no longer 'suggested' — an operator already acted
```

Escalation is checked above the ai/fixed mode switch, but the two modes
react to it differently:

- **fixed mode**: a canned away-message has no dependency on the LLM having
  worked, so `SkipWhenEscalated` only blocks it for a model's own evaluated
  escalation — never for an `engine_error:` one (`response.EngineErrorPrefix`).
- **ai mode**: `engine_error:` is checked **unconditionally**, regardless of
  `SkipWhenEscalated`. `holdingDraft` stamps this prefix on any hard
  KB-load/LLM failure; without the unconditional check, an LLM provider
  outage would broadcast the generic holding text to every customer on
  every auto-response-enabled account.

## 7. Sending

`SendAutoResponse` is the single, guarded transaction that turns a claimed
job into a real outbound message. It takes the same chat advisory lock as
every cancellation path, then makes three conditional writes, any of which
matching zero rows aborts the whole transaction (`ErrAutoResponseAborted`,
treated as a normal send failure — never retried):

1. flip the job `claimed -> sent`, but only if no outbound message has
   landed on the chat since the trigger arrived — this is where "quiet
   since armed" is actually enforced, inside the transaction; a pre-check
   outside it would just move the race, not close it;
2. ai mode: flip the draft `suggested -> sent`, returning its text; fixed
   mode: flip it `suggested -> superseded` instead (never "sent" — a stray
   click on the now-stale panel card must not send it a second time);
3. insert the outbound message (`insertOutboundTx`, extracted from
   `InsertOutbound` for exactly this) with `resetUnread = false` — an
   unattended auto-reply must **not** clear the chat's unread badge, or the
   operator arrives the next morning to a conversation that looks
   untouched.

## 8. Cancellation

Five sites cancel a chat's pending job, all under the same chat advisory
lock so a cancel can never interleave with a concurrent sweeper claim:

- `handleApprove` — an operator approved the draft manually;
- `handleSendMessage` — an operator sent a message before the timer fired;
- `UpsertAutoResponse` with `enabled = false` — turning the policy off must
  not let an already-armed job fire under the old configuration;
- both account-delete handlers (WhatsApp, Telegram) — a deleted/disconnected
  account can never auto-send after the fact.

## 9. HTTP API

`PUT /xchats/api/v1/accounts/:id/auto-response` — channel-neutral
(`orgAnyAccount`, unlike the WhatsApp-only `orgAccount`). Validation
(`parseAutoResponsePolicy`), each a 400 with a specific reason:

- `reply_mode` must be `"ai"` or `"fixed"`;
- `timezone` required and must `time.LoadLocation`;
- `delay_seconds` in `[0, 3600]`, `cooldown_seconds` in `[0, 86400]`
  (deliberately no "cooldown must be >= delay" rule — they answer different
  questions);
- every window's clock strings must parse (`ParseClock`, so `"24:00"` is
  accepted) and `weekday` must be `0..6`;
- if `enabled`: at least one window after normalization (so a same-weekday
  wrap that collapses to real coverage still counts), and fixed mode
  requires non-empty `fixed_text`.

`dto.Account.auto_response` is non-nullable — `GET /accounts` always
includes a policy, materializing `autoresponse.DefaultPolicy()` for an
account with no configured row, so the frontend never special-cases
"missing" separately from "disabled".

## 10. Frontend

`AutoResponseDialog.vue` (opened from a new «Автоответ» button on each
account card in `Accounts.vue`) models each weekday as one of four MODES —
`Не отвечать` / `Круглосуточно` / `Вне интервала HH:MM–HH:MM` / `В
интервале HH:MM–HH:MM` — never a raw window: `<input type="time">` cannot
hold `"24:00"`, and a mode is what an operator actually means to say.
`lib/autoResponse.ts` (pure, unit-tested independently of the component)
owns the conversion both ways:

- `windowsToDayStates` reads the canonical windows GET /accounts returns and
  re-joins a split pair (`[00:00,X)` + `[Y,24:00)` on the same weekday) back
  into `Вне интервала` — a schedule saved as "outside" round-trips into the
  same mode, not two "inside" rows the UI would otherwise show as if
  authored elsewhere;
- `dayStatesToWindows` is the inverse: `Вне интервала` submits ONE raw
  wrapping window (`start`/`end` swapped) — the split into two canonical
  rows is `internal/autoresponse.Normalize`'s job, done once, server-side,
  not duplicated in TypeScript.

A computed 7×24 coverage strip (`coverageStrip`) is the honest display of
what the per-day rules actually add up to, and catches an empty schedule
before save. `saveAutoResponse` in `stores/accounts.ts` PATCHes the account
row in place from the PUT response rather than reloading the list — a full
`GET /accounts` re-probes live connection status and could visibly flicker
a healthy connection for a change that has nothing to do with it.

## 11. Operational tuning

All defaults live as named consts in `cmd/xchats/main.go`, wired onto
`Worker` fields (a harness/test that leaves a field zero falls back to the
same value via `internal/worker/autoresponse.go`'s tuning-default helpers):

| const                 | value    | meaning                                          |
|-----------------------|----------|---------------------------------------------------|
| `quietPeriod`         | 5s       | debounce: silence required before generating       |
| `debounceCap`         | 60s      | debounce: max wait for a chat that never quiets    |
| `sweepEvery`          | 10s      | sweeper tick interval                              |
| `sweepBatch`          | 50       | rows claimed per sweep, per table                  |
| `claimTimeout`        | 5m       | reaper cutoff for a worker that died mid-send      |
| `maxDeliveryLag`      | 10m      | job older than this claims as `too_late`, not sent |
| `jobRetention`        | 30d      | terminal job rows are purged after this long       |
| `sweeperShutdownWait` | 3s       | shutdown: bound on waiting for sweepers to settle   |

Shutdown is two-phase in both `main.go` and the test harness, in this
order: (1) stop anything that could publish a *new* task — both sweepers
observe `ctx.Done()` and `StopDebounceTimers()` cancels every pending timer,
waiting for any already-firing callback to finish its publish; (2) only
once phase 1 has settled, drain the queue (`q.Wait()`) before `q.Close()` —
otherwise a task a timer just published in phase 1 could still be
mid-process on a worker goroutine when `q.Close()` (and the following
`st.Close()`) runs out from under it. `queue.InMem.Publish` additionally
recovers a send-on-closed-channel panic into a plain error, as defense in
depth against the same race.

## 12. Testing

- `internal/autoresponse/autoresponse_test.go` — pure unit tests: the
  168-hour coverage check, `Normalize` round-trip/merge, `ParseClock`/
  `FormatClock` including `"24:00"`, the weekday convention, DST transitions
  against `Europe/Berlin`, and an `Asia/Almaty == UTC+5` guard (Kazakhstan
  moved off UTC+6 on 2024-03-01 — a regression here silently mis-schedules
  every Kazakhstan account).
- `internal/httpapi/autoresponse_test.go` — `DATABASE_URL`-gated
  integration tests against a real Postgres: debounce coalescing and
  restart-survival, arming in/out of schedule, delay → send with the draft
  flip and a fake channel receipt, cancellation via approve/manual-send,
  escalation skip (including the unconditional `engine_error:` case), fixed
  mode superseding the draft, the stale-claim reaper, unread-count
  preservation, and the PUT endpoint's round-trip/validation/cross-org
  behavior.
- `frontend/src/lib/autoResponse.test.ts` — vitest coverage of the mode↔
  windows conversions, the coverage strip, and the badge summary.
- `frontend/tests/e2e/accountsAutoResponse.spec.ts` — a route-mocked
  Playwright spec (`accountsMock.ts`) driving the real dialog component,
  independent of any live backend.

## References

- `internal/autoresponse/autoresponse.go` — scheduling primitives.
- `internal/store/debounce.go`, `internal/store/autoresponse.go` — durable
  state and the guarded transactions above.
- `internal/worker/debounce.go`, `internal/worker/autoresponse.go` — timers,
  the sweeper, and the firing guard chain.
- `internal/httpapi/autoresponse.go` — the PUT endpoint and validation.
- `migrations/0017_account_auto_response.up.sql` — full schema.
