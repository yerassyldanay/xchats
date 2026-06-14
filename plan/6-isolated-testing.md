# Isolated Development & Testing

How to develop and verify the whole app — and each element — in an **isolated environment** (a
sandbox / CI with no real WhatsApp and no external network) and still trust the real run.

## Principle: pin the external contract to real captured data

The only true external is Evolution. Its REST responses and webhook payloads are pinned to **real
captures** (`captures/`), so we reproduce them deterministically. Everything else is either
real-but-local (Postgres) or a controllable fake (Evolution, the LLM).

## Test doubles & locals

- **Fake Evolution** — a small HTTP stub implementing the endpoints we call (`instance/create`,
  `instance/connect`, `message/sendText`, `message/sendMedia`,
  `getBase64FromMediaMessage`) with recorded responses, that can also **POST captured webhook
  events** at our webhook edge. It **records what we sent** so tests can assert send shapes
  (e.g. that we send to the phone, not the `@lid`).
- **Fake LLM** — an OpenAI-compatible stub returning a fixed `emit_draft`, so AI-draft tests are
  deterministic and free.
- **Real local Postgres** — the same engine as prod, a throwaway DB/schema, so SQL runs for real.
- **Default in-proc adapters** — local-disk blob store + Go-channel / Postgres queue.

## Layers of tests

- **Unit** — `normalize` against captured payloads (text/media, `@lid`↔phone, status 0–5); pure,
  no I/O.
- **Component** — the webhook handler (store-raw + 200 + enqueue + dedup) and each worker (upsert,
  media, status, send) against the fakes + local Postgres.
- **End-to-end (one command)** — drives the full loop: replay webhook → assert normalized rows →
  call send API → assert the fake Evolution received the right call → replay status → assert
  sent→delivered→read → media path → AI draft.
- **Frontend** — Vitest component tests; optional Playwright against the backend wired to the fakes.

## One command

```text
make test-e2e     # brings up the isolated stack, runs all layers, tears it down — no external network
```

Backed by `deploy/compose.test.yaml` = **postgres + backend + fake-evolution + fake-llm**. It
migrates, runs the suite, and stops. This is exactly what runs in Claude Code's isolated
environment / CI.

## Developing in isolation

Run the same stack and iterate: env points the backend at local Postgres + fake Evolution;
exercise endpoints with the captured payloads; watch logs. Because **addressing is env-driven**,
the only difference from prod is which hosts/ports the env points at.

## Stack (v1 = one implementation per boundary)

v1 ships **one adapter each** behind the blob and queue ports (see `2-architecture.md`):

- blob: `local-disk` only (media bytes are a deferred surface anyway — v1 renders placeholders).
- queue/bus: the **Postgres job table** only (an in-process Go channel is for tests only).

**Deferred (v2+):** the `minio`/`kafka` adapters and the **adapter conformance suite** that runs
across implementations. There is no second adapter to conform to in v1 — don't build it until one
exists.

## Real smoke (manual, outside isolation)

A tiny optional check against the live Evolution (`localhost:9700`) confirms reachability +
credentials. The fixtures are byte-for-byte real Evolution output, so a green isolated suite proves
**normalizer/transport parity on the captured events**. Full contract trust requires the complete
live event set — and the current `captures/` set is missing the core inbound `messages.upsert`, the
`getBase64` response, and a matched send→`messages.update` pair (see `captures/README.md`). Until
those are captured, "green" means "correct on what we've captured", not "the whole contract is
proven".
```

