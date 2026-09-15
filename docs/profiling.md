# Hermetic Profiling and Load Harness

This is a **development-only** tool for finding and fixing real performance
bottlenecks in the xchats backend, without needing live LLM/WhatsApp/
Telegram/Meta credentials and without ever risking a real message, a real
LLM bill, or a real outbound call. It is not part of the product; it never
runs in production (`system.mock_externals` refuses to boot when
`environment=production`).

## What "hermetic" means here

Setting `system.mock_externals` (`MOCK_EXTERNALS=true` / `--mock-externals`)
replaces every outbound-network integration — LLM providers, WhatsApp/
Telegram/Instagram/Messenger/WhatsApp Cloud, Meta OAuth, speech-to-text,
ngrok, the KB-import extractors, credential "test connection" probes, MCP
client-metadata discovery, GitHub update checks, and the worker's direct
CDN media fetch — with in-memory fakes that always succeed with realistic,
unique values. Nothing else changes: the real HTTP server, session auth,
SQLite, the in-process queue, every worker, prompt construction, JSON
contract validation, and outbound-message assembly all run completely
unmodified. The composition root (`backend/cmd/xchats/main.go`) is the only
place that knows which bundle — real or mock — is selected; no handler or
worker contains a mock-specific branch.

This means a CPU or heap profile captured under `MOCK_EXTERNALS=true` is a
profile of **xchats' own code** — the response engine's prompt/validation
pipeline, the store layer, the queue, JSON encoding — with the network and
inference-latency variance of a real LLM/WhatsApp/Meta call removed from
the picture entirely.

## Quick start

```sh
# terminal 1 — builds, seeds a fresh isolated database (mock-externals mode,
# including a demo credential for every seeded channel so outbound sends
# actually reach a channel sender), and runs in the foreground
make profile-server

# terminal 2 — once terminal 1 logs "ready"
make profile-load
```

`profile-load` authenticates, validates every route, then drives 32
closed-loop workers through the default request mix for a 5s warm-up + 35s
measured run, capturing a 30-second CPU profile plus heap/goroutine/mutex/
block snapshots that overlap the load. It finishes by opening the CPU
profile's interactive pprof UI (flame graph, source view, call graph) —
pass `PROFILE_VIEW=none` for a headless capture-only run (CI, a remote box).

Every artifact lands under `.cache/profiles/<UTC-timestamp>-<pid>/`
(gitignored), preserved by default after `profile-server` exits. Set
`PROFILE_CLEAN=1` to delete an auto-created run directory on exit; a
`PROFILE_DATA_DIR=...` you pass yourself is never deleted automatically.

## Make targets

| Target | What it does |
| --- | --- |
| `make profile-server` | Build with symbols (`PROFILE_RACE=1` adds `-race`), seed an isolated DB in mock-externals mode, run in the foreground with the pprof listener on `127.0.0.1:6060`. |
| `make profile-load` | Find the active `profile-server`, run steady load, capture CPU (30s) + heap/goroutine/mutex/block, write `go tool pprof -top` reports, open the CPU flame graph. |
| `make profile-view PROFILE=cpu\|heap-alloc\|heap-inuse\|mutex\|block` | Reopen any already-captured profile's interactive UI, from the active run or the most recent one. |
| `make profile-trace` | Run load, capture+validate a 10s execution trace, launch `go tool trace`. |
| `make profile-bench OUT=path.txt` | Ten-sample `go test -bench` run (see the optimization loop below). |
| `make profile-compare BASE=old.txt NEW=new.txt` | `benchstat` comparison of two `profile-bench` outputs. |

`LOAD_CONCURRENCY`, `LOAD_WARMUP`, `LOAD_DURATION`, `LOAD_SEED` override the
load harness's own defaults for `profile-load`/`profile-trace`;
`PROFILE_CPU_SECONDS`/`PROFILE_TRACE_SECONDS` override the capture window.

The load harness itself (`scripts/load/`, a standard-library-only Go module
with its own tests) can also be run directly against any mock-mode server:

```sh
cd scripts/load && go run . -base-url http://127.0.0.1:8099 -concurrency 32
```

See its own `-h` output for every flag (base URL, credentials, concurrency,
warm-up, duration, seed, request-mix weights, a `--ready-file` to
synchronize against a server still booting, and `--json-out` for a machine-
readable report).

## The optimization loop

Once a profile points at a real hotspot, follow this loop rather than
guessing:

1. **Baseline.** `make profile-bench OUT=.cache/profiles/baseline.txt` —
   ten samples, so noise doesn't masquerade as a win. Alongside it, capture
   a full `profile-server` + `profile-load` run and keep its directory.
2. **Corroborate.** A hotspot is worth chasing only if it shows up in more
   than one signal — e.g. a function high in the CPU profile that is *also*
   a top heap allocator or a top contended mutex in the same run. A single
   profile's top line can be sampling noise; agreement across CPU,
   allocation, and contention data is what makes it real.
3. **Add or refine a focused benchmark** that isolates exactly that
   hotspot, if one doesn't already exist for it.
4. **Change one thing.** Make the smallest change that addresses the
   corroborated hotspot. Resist bundling unrelated cleanup into the same
   change — it makes the next step's comparison meaningless.
5. **Revalidate.** `go test ./...`, `go test -race -count=1 ./...`, and a
   fresh `profile-server` + `profile-load` pass, all green.
6. **Compare, don't eyeball.** `make profile-bench OUT=.cache/profiles/candidate.txt`
   (ten samples again) then `make profile-compare BASE=... NEW=...`. Accept
   the change as a real improvement only when `benchstat` reports a
   statistically supported delta — not because the mean moved.

Repeat from step 2 against the next-largest corroborated hotspot. This
harness deliberately ships with **no** production optimization already
applied — the loop above is how one gets found and justified, not assumed.

## What this harness is not

- Not a correctness or security test — `go test ./...` and the deterministic
  suites already cover that; this harness only measures performance
  characteristics of code paths those suites already exercise correctly.
- Not a substitute for testing against a real LLM/WhatsApp/Meta integration
  before shipping a channel-facing change — mock responses are
  contract-valid, not semantically representative of a real model's
  behavior.
- Not automated: profiling and load runs are a manual, on-demand loop (the
  Make targets above); nothing here runs in CI.
