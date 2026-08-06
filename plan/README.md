# plan/

The design record this project was built from. [`DECISIONS.md`](DECISIONS.md)
is authoritative wherever another document here disagrees with it or with
the current code — see each document's own header for that pointer. This
directory predates most of the current implementation, so treat it as design
intent and rationale, not as a live description of the code; where a
document is known to have drifted (a completed migration, a status line that
hasn't been updated), that's noted below and, where practical, in the
document itself.

| Document | What it covers | Status |
|---|---|---|
| [`overview.md`](overview.md) | Purpose, boundaries, terms, document map. | Design target; mostly matches current shape. |
| [`architecture.md`](architecture.md) | Channel adapters, workers, storage, AI boundaries. | Design target. |
| [`database-schema.md`](database-schema.md) | Target tables, responsibilities, columns. | Design target — predates the SQLite port; see its own header for the type-syntax caveat (PostgreSQL terms throughout; [`backend/migrations/sqlite/`](../backend/migrations/sqlite/) is the current, actual schema). |
| [`knowledge-base.md`](knowledge-base.md) | The approved-KB prompt and response contract. | Design target; implemented (see [`backend/response/`](../backend/response/), [`backend/aiprompt/`](../backend/aiprompt/)). |
| [`playground.md`](playground.md) | Material-to-draft-to-live authoring flow. | Implemented — see [`docs/release/`](../docs/release/) and the app's `/knowledge-base` and `/playground` views for the current split (creation vs. review). |
| [`mcp.md`](mcp.md) | The MCP connector: OAuth, JSON-RPC tool contract, KB Manager widget. | Implemented — see [`backend/internal/mcpserver/`](../backend/internal/mcpserver/), [`backend/internal/mcpauth/`](../backend/internal/mcpauth/). |
| [`telegram-testing.md`](telegram-testing.md) | Manually verifying the Telegram channel: env vars, endpoints, curl walkthrough. | Reference, kept current. |
| [`EVALTOOL.md`](EVALTOOL.md) | Design/handoff for the evaluation platform. | Implemented — see [`evals/harness/`](../evals/harness/); this document is the rationale, not the current harness's own docs. |
| [`DECISIONS.md`](DECISIONS.md) | The authoritative design-decision record. | Living document — some entries predate implementation and are marked accordingly inline; trust the code and its tests over a stale entry here, and please fix the entry in the same PR if you notice one. |
| [`archive/`](archive/) | Superseded documents kept for history, not current design. | Historical only — do not follow as instructions. |

## `archive/`

- [`TRACK_A_SQLITE_HANDOFF.md`](archive/TRACK_A_SQLITE_HANDOFF.md) — the
  PostgreSQL-to-SQLite migration handoff. The migration is done; this
  describes files and a cutover sequence that no longer exist in that form.
- [`TRACK_A_BENCHMARKS.md`](archive/TRACK_A_BENCHMARKS.md) — benchmark
  results from that same migration effort.
