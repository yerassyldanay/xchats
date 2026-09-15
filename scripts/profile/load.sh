#!/usr/bin/env bash
# scripts/profile/load.sh — `make profile-load`: find the active
# profile-server, run the load harness against it, capture a 30-second CPU
# profile (overlapping the load run's own measured phase) followed by
# heap/goroutine/mutex/block snapshots and a goroutine?debug=2 stack dump,
# write go tool pprof -top text reports, then open the CPU profile's
# interactive pprof UI (PROFILE_VIEW=none for headless use).
#
# Env overrides: LOAD_CONCURRENCY, LOAD_WARMUP, LOAD_DURATION, LOAD_SEED,
# PROFILE_CPU_SECONDS (default 30), PROFILE_VIEW=none.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=lib.sh
source ./lib.sh
require_go_tool

require_active_server
log "profile-load: using active run: $RUN_DIR"
BIN="$RUN_DIR/bin/xchats-profile"
[ -x "$BIN" ] || die "expected binary $BIN from the active profile-server, but it's missing"

mkdir -p "$RUN_DIR/profiles" "$RUN_DIR/reports"

CPU_SECONDS="${PROFILE_CPU_SECONDS:-30}"
WARMUP="${LOAD_WARMUP:-5s}"
WARMUP_SECONDS="${WARMUP%s}"
# Cover warm-up + the CPU capture window + a trailing buffer, so the
# capture sits entirely inside the load run's own MEASURED phase.
DEFAULT_DURATION_SECONDS=$((WARMUP_SECONDS + CPU_SECONDS + 15))
DURATION="${LOAD_DURATION:-${DEFAULT_DURATION_SECONDS}s}"
CONCURRENCY="${LOAD_CONCURRENCY:-32}"
SEED="${LOAD_SEED:-42}"

LOAD_BIN="$RUN_DIR/bin/xchats-load"
log "profile-load: building the load tool"
(cd "$ROOT/scripts/load" && go build -o "$LOAD_BIN" .)

log "profile-load: running load — concurrency=$CONCURRENCY warmup=$WARMUP duration=$DURATION seed=$SEED"
"$LOAD_BIN" \
	-base-url "http://$HTTP_ADDR" \
	-concurrency "$CONCURRENCY" \
	-warmup "$WARMUP" \
	-duration "$DURATION" \
	-seed "$SEED" \
	-json-out "$RUN_DIR/reports/load.json" \
	>"$RUN_DIR/reports/load.txt" 2>&1 &
LOAD_PID=$!

# Let the load tool clear its own warm-up (plus a short buffer) before
# capturing, so the CPU profile window sits inside the measured phase.
sleep "$((WARMUP_SECONDS + 1))"

if ! kill -0 "$LOAD_PID" 2>/dev/null; then
	cat "$RUN_DIR/reports/load.txt" >&2 || true
	die "load run exited before the capture window even started — see reports/load.txt above"
fi

log "profile-load: capturing ${CPU_SECONDS}s CPU profile (blocks while load keeps running)"
curl -fsS --max-time "$((CPU_SECONDS + 10))" \
	"http://$PPROF_ADDR/debug/pprof/profile?seconds=$CPU_SECONDS" \
	-o "$RUN_DIR/profiles/cpu.pprof"

log "profile-load: capturing heap/goroutine/mutex/block snapshots"
curl -fsS "http://$PPROF_ADDR/debug/pprof/heap" -o "$RUN_DIR/profiles/heap.pprof"
curl -fsS "http://$PPROF_ADDR/debug/pprof/goroutine" -o "$RUN_DIR/profiles/goroutine.pprof"
curl -fsS "http://$PPROF_ADDR/debug/pprof/mutex" -o "$RUN_DIR/profiles/mutex.pprof"
curl -fsS "http://$PPROF_ADDR/debug/pprof/block" -o "$RUN_DIR/profiles/block.pprof"
curl -fsS "http://$PPROF_ADDR/debug/pprof/goroutine?debug=2" -o "$RUN_DIR/profiles/goroutine-debug2.txt"

log "profile-load: waiting for the load run to finish"
LOAD_EXIT=0
wait "$LOAD_PID" || LOAD_EXIT=$?
cat "$RUN_DIR/reports/load.txt"
if [ "$LOAD_EXIT" -ne 0 ]; then
	log "profile-load: WARNING — the load run reported errors (exit $LOAD_EXIT); see reports/load.txt / load.json"
fi

log "profile-load: writing go tool pprof -top text reports"
go_tool pprof -top -nodecount=30 "$BIN" "$RUN_DIR/profiles/cpu.pprof" >"$RUN_DIR/reports/cpu-top.txt" 2>&1 || true
go_tool pprof -top -nodecount=30 -alloc_space "$BIN" "$RUN_DIR/profiles/heap.pprof" >"$RUN_DIR/reports/heap-alloc-top.txt" 2>&1 || true
go_tool pprof -top -nodecount=30 -inuse_space "$BIN" "$RUN_DIR/profiles/heap.pprof" >"$RUN_DIR/reports/heap-inuse-top.txt" 2>&1 || true
go_tool pprof -top -nodecount=30 "$BIN" "$RUN_DIR/profiles/goroutine.pprof" >"$RUN_DIR/reports/goroutine-top.txt" 2>&1 || true
go_tool pprof -top -nodecount=30 "$BIN" "$RUN_DIR/profiles/mutex.pprof" >"$RUN_DIR/reports/mutex-top.txt" 2>&1 || true
go_tool pprof -top -nodecount=30 "$BIN" "$RUN_DIR/profiles/block.pprof" >"$RUN_DIR/reports/block-top.txt" 2>&1 || true

log "profile-load: done — run directory: $RUN_DIR"
log "profile-load: re-open any captured profile later with 'make profile-view PROFILE=cpu' (or heap-alloc, heap-inuse, mutex, block)"

if [ "${PROFILE_VIEW:-}" = "none" ]; then
	log "profile-load: PROFILE_VIEW=none — skipping the interactive pprof UI"
	exit "$LOAD_EXIT"
fi
log "profile-load: opening the CPU profile's interactive pprof UI (Ctrl-C to stop; PROFILE_VIEW=none to skip)"
go_tool pprof -http=127.0.0.1:0 "$BIN" "$RUN_DIR/profiles/cpu.pprof"
exit "$LOAD_EXIT"
