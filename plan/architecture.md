# Target Architecture

[`DECISIONS.md`](DECISIONS.md) is authoritative. This document connects its
decisions into implementable component and data-flow boundaries.

## System context

```text
Customer/provider                                        Operator
       │ webhook / send                                      │ HTTPS/SSE
       ▼                                                     ▼
 ChannelAdapter ── normalized event ──► Go modular backend ◄── Vue frontend
       ▲                                  │   │   │
       │ provider API                     │   │   └── ModelGateway ──► aggregator
       └──────────────────────────────────┘   │       (OpenAI-compatible)
                                              │
                       PostgreSQL ◄───────────┼──────────► Queue interface
                         metadata,            │             workers
                         live/draft state      ▼
                                        StorageAdapter
                                        file bytes
```

The initial deployment is one Go application plus workers, not independently
deployed microservices. Package boundaries and interfaces keep channel, queue,
storage, and model integrations replaceable without adding distributed-system
coordination before scale requires it.

## Component responsibilities

### Channel adapters

`ChannelAdapter` owns provider authentication, connection setup, webhook
verification, webhook normalization, provider media download, outbound sending,
delivery-status mapping, and provider capability checks. Provider payloads and
identifiers stay in the adapter and channel-prefixed transport tables.

WhatsApp connects directly via `go.mau.fi/whatsmeow` and `wa_*`. Pairing a
number displays its QR, and idempotently materializes `wa_accounts` when the
connection event arrives. A future Instagram or Telegram adapter adds `ig_*` or
`tg_*` transport state but reuses the shared inbox, knowledge, suggestion,
approval, and audit paths.

### Application and workers

The Go backend authenticates users, enforces organization membership, serves the
frontend API, persists state, and owns every deterministic validator. Webhook
handlers authenticate, normalize, enqueue, and return quickly. Workers perform
idempotent upserts, downloads, extraction, synthesis, response generation, and
processing-status transitions.

The queue is an interface. Go channels are acceptable initially, but durable
workflow state stays in PostgreSQL. On process start, the backend must find
`kbd_materials` whose `processing_status` is `uploaded` or timed-out
`extracting` and re-enqueue the same rows. Moving to a durable broker must not
change producers or worker contracts.

### Data and integration adapters

- PostgreSQL schema `xchats` is authoritative for tenants, conversations,
  materials, pending knowledge, live knowledge, requests, suggestions, and
  audit records.
- `StorageAdapter` maps `storage_backend + storage_key` to bytes. Live KB rows
  refer to stable `kbd_materials.id` values, so moving a blob changes only its
  material row.
- `ModelGateway` is one OpenAI-compatible aggregator integration (initially
  OpenRouter). Model IDs are configuration. V1 does not use direct vendor SDKs.
- Provider prompt caching is an optimization of the stable live-KB prefix. It
  may change latency and cost, never prompt contents or answer correctness.

## Runtime customer-response flow

```text
1. provider webhook
2. ChannelAdapter authenticates and normalizes the event
3. persist/enqueue idempotently; return to provider
4. worker loads organization policy, conversation context, and approved ai_* only
5. PromptBuilder emits cached live prefix + dynamic conversation suffix
6. ModelGateway returns the canonical JSON contract: reply_text, reply_language,
   media_files_to_send, escalate, escalation_reason, and confidence
7. ResponseValidator validates placeholders and the semantic media-token allowlist
8. code substitutes exact live values and resolves approved media through
   kbd_materials
9. rp_suggestions stores suggestion/approval state
10. organization response policy or operator approval authorizes sending
11. ChannelAdapter checks capabilities and sends; delivery state is persisted
```

The response model has no authority over exact numbers, internal material IDs,
or storage. An unknown placeholder or `media_files_to_send` token rejects the
whole response; it is never partially sent. V1 trusted KB prose and replies are
Russian-only. Details of prompt and token construction live in
[`knowledge-base.md`](knowledge-base.md).

## Knowledge-authoring flow

```text
submit materials
  → persist one kbd_materials row per input (and file bytes before extraction)
  → pass 1: extract each material independently, in parallel
  → wait until every submitted material is terminal
  → pass 2: synthesize parsed evidence + full draft/live model-safe views
  → validate handles/schema/values
  → atomically merge complete entries into the organization's one kbd_draft
  → review once
  → transactionally approve selected/all entries into ai_*
  → remove approved entries from draft, append audit row, reload prompt brain
```

Pass 2 is text-only and side-effect-free while the model runs. It receives
request-scoped handles, never internal IDs or storage locators. Only validated
output reaches `kbd_draft`. Approval never translates between schemas: draft
entries already mirror live business rows. Full lifecycle detail lives in
[`playground.md`](playground.md).

## Transaction and consistency boundaries

- Every table access is organization-scoped; cross-organization material and
  media references are rejected.
- A draft merge locks or compare-and-swaps the organization
  `kbd_draft.base_version`.
  Stale writers receive `409 Conflict`; natural-key merges make retries
  idempotent.
- Candidate pass-2 output is validated before the draft transaction. The draft
  blob and corresponding request mutations commit atomically.
- Approval locks the draft, revalidates selected complete entries and their
  materials, upserts/deletes live rows, removes only those entries from the
  draft, and appends `ai_audit_log` in one database transaction. Brain reload is
  triggered only after commit.
- Suggestion validation and send are fail-closed. Adapter-level idempotency must
  prevent retries from duplicating provider messages.

## Security and trust boundaries

Uploaded/fetched content is untrusted evidence, not instructions to the backend.
URL extraction accepts HTTP(S), applies SSRF defenses, and does not recursively
crawl in v1. Authorization is checked at API entry and again when resolving
materials. Logs and model requests must omit storage credentials, paths, public
object URLs, and internal material IDs. The production model/data-residency
boundary remains an explicit pre-launch decision.

## Operations and scale posture

Instrument webhook rate/error/latency, queue depth and age, extraction duration
and failure by type, model errors/cost, draft conflicts, approval failures,
placeholder/token rejection, and provider send failures. Expose separate
liveness and readiness checks; readiness includes PostgreSQL and required
adapters.

Start with one PostgreSQL primary and the modular monolith. Add replicas,
external queues, caches, or sharding only in response to measured bottlenecks.
No traffic, latency, availability, retention, RPO, or RTO targets have been
agreed, so this plan intentionally does not invent capacity numbers.
