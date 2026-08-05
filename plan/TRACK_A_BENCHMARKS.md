# Track A (SQLite cutover) — Phase 4 benchmark results

This documents the benchmark suite added for the SQLite cutover
(`claude/sqlite-cutover-track-a-gwhu8u`, Track A of the local-service plan) and
the numbers it currently produces, so anyone changing `internal/dbx` or the
packages built on it can tell whether a change made things faster or slower
without re-deriving a baseline from scratch.

No throughput, latency, or capacity target exists to grade these against —
`architecture.md`'s "Operations and scale posture" section says so explicitly
("No traffic, latency, availability, retention, RPO, or RTO targets have been
agreed"). This document is a baseline and a methodology, not a pass/fail gate.

## Running the benchmarks

```sh
cd backend
go test -run=^$ -bench=. -benchmem ./internal/store/...
go test -run=^$ -bench=. -benchmem ./internal/kbstore/...
go test -run=^$ -bench=. -benchmem ./internal/responsestore/...
go test -run=^$ -bench=. -benchmem ./internal/dbops/...
```

Each benchmark opens its own fresh, migrated SQLite database under `b.TempDir()`
(via `internal/dbtest`, whose fixture helpers now accept `testing.TB` instead of
`*testing.T` specifically so they work from both `Test*` and `Benchmark*`
functions) — there is no shared state between benchmarks and no external
service dependency.

## What's covered, and why

The four benchmarked call sites are the application's actual hot paths, not an
exhaustive sweep of every exported method:

- **`internal/store`** (`bench_test.go`): `UpsertInbound` — every inbound
  webhook delivery, split into `_NewChat` (first message from a contact: pays
  the full contact+chat creation cost) and `_ExistingChat` (steady-state
  append cost) — plus `_ExistingChat_Parallel`, which measures the throughput
  `internal/dbx`'s single-writer-serialization design (`MaxOpenConns(1)` +
  `_txlock=immediate`, see its package doc) actually sustains when many
  goroutines contend for the same connection and the same chat's aggregate
  row at once. `ChatByID`/`MessagesForChat` cover the two reads
  `GET /chats/:id` makes.
- **`internal/kbstore`**: `LoadLive` (the brain's own KB read) and `Draft`
  (the Playground's live-plus-pending merged view).
- **`internal/responsestore`**: `KnowledgeBaseRepo.Load` — explicitly called
  out in `kb.go`'s own doc comment as "the response engine's hot path (every
  customer reply)."
- **`internal/dbops`**: `Backup` and `IntegrityCheck` at 0/1,000/10,000 seed
  rows — not a request-path hot path, but `Backup` holds the database's one
  physical connection for its whole duration (see `backup.go`'s package doc),
  so its cost is the window of added latency a live backup imposes on
  everything else; an operator deciding how often to run it needs this
  number.

## Results

Captured on this environment's CPU (`Intel(R) Xeon(R) Processor @ 2.10GHz`,
4 logical CPUs — `cpu:` line in `go test`'s own output) — absolute numbers
will differ on other hardware; what matters for future comparisons is the
shape (which operations are cheap, which scale with data volume, the
parallel/sequential ratio) more than the exact nanosecond figures. Run with
`go test -run=^$ -bench=. -benchmem -benchtime=2s`:

```text
BenchmarkUpsertInbound_NewChat-4                    3681    805931 ns/op    5495 B/op    161 allocs/op
BenchmarkUpsertInbound_ExistingChat-4               4773    677040 ns/op    6236 B/op    185 allocs/op
BenchmarkUpsertInbound_ExistingChat_Parallel-4      5532    717190 ns/op    6381 B/op    188 allocs/op
BenchmarkChatByID-4                                30492     76710 ns/op    2928 B/op     74 allocs/op
BenchmarkMessagesForChat-4                          4412    539674 ns/op   45965 B/op    673 allocs/op

BenchmarkLoadLive-4                                16364    147757 ns/op   24852 B/op    473 allocs/op
BenchmarkDraft-4                                    8743    270543 ns/op   38912 B/op    868 allocs/op

BenchmarkKnowledgeBaseRepo_Load-4                   8576    279267 ns/op   60651 B/op   1375 allocs/op

BenchmarkBackup/rows=0-4                            1684   1198424 ns/op     870 B/op     17 allocs/op
BenchmarkBackup/rows=1000-4                         1496   1602957 ns/op     869 B/op     17 allocs/op
BenchmarkBackup/rows=10000-4                         608   3771092 ns/op     866 B/op     17 allocs/op
BenchmarkIntegrityCheck/rows=0-4                  248620      9352 ns/op     472 B/op     16 allocs/op
BenchmarkIntegrityCheck/rows=1000-4                22640    102277 ns/op     472 B/op     16 allocs/op
BenchmarkIntegrityCheck/rows=10000-4                2521    938821 ns/op     472 B/op     16 allocs/op
```

## Reading the numbers

- **Message ingest sustains roughly 1,400–1,500 writes/sec sequentially**
  (677–806µs/op) on this hardware, whether each message creates a new chat or
  appends to an existing one — the two-step upsert paths (see
  `internal/store/ingest.go`'s `upsertChatTwoStep`) do not meaningfully widen
  that gap.
- **The parallel benchmark confirms the serialization design behaves as
  intended, not as a hidden bottleneck**: per-operation latency under
  concurrent load (717µs/op, `GOMAXPROCS=4`) sits close to the sequential
  number rather than degrading — every writer queues in Go's own connection
  pool (see `internal/dbx`'s package doc) instead of thrashing against
  SQLite's busy-retry loop. The aggregate throughput ceiling this implies
  (~1,400 writes/sec) is a property of "one physical connection," not of this
  benchmark's specific contention pattern — more concurrent callers queue
  longer, they do not raise the ceiling. If ingest throughput ever needs to
  exceed that, the fix is architectural (the documented-but-unbuilt
  `ReadQuery`/`BeginRead` split-reader-pool extension point in
  `internal/dbx`'s package doc only helps reads, not this write path), not a
  tuning knob within the current design.
- **The response engine's actual hot path is cheap relative to the LLM call
  it precedes**: `KnowledgeBaseRepo.Load` at ~279µs is several orders of
  magnitude below typical LLM response latency (hundreds of milliseconds to
  several seconds) — the SQLite read is not a meaningful contributor to
  reply latency at this data volume.
- **Backup and IntegrityCheck both scale roughly linearly with row count and
  stay in the low single-digit milliseconds even at 10,000 rows** — cheap
  enough to run frequently (backup on a schedule, integrity-check even more
  often) without materially affecting the rest of the app, though Backup's
  cost is still a real, measurable window during which every other operation
  queues behind it (see `internal/dbops/backup.go`'s package doc) — plan
  backup frequency with that in mind rather than treating it as free.

## What this does not cover

This is not a load test and not a capacity plan — no target QPS, dataset
size, or SLO exists yet to test against (see `architecture.md`, quoted
above). It also does not benchmark `internal/mcpauth` (its operations are
infrequent — OAuth token issuance/rotation, not per-message hot paths) or
`internal/dbx` in isolation from the packages built on it (the four packages
above already exercise the facade under realistic call shapes). If a real
capacity target is ever set, re-run this suite against it rather than
assuming these numbers still hold — dataset size, schema changes, and
hardware all shift them.
