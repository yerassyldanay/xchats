#!/usr/bin/env bash
# scripts/profile/trace.sh — `make profile-trace`: find the active
# profile-server, run load, capture a 10-second execution trace, validate
# it (go tool trace -pprof=sched must parse it cleanly), then launch the
# interactive `go tool trace` UI.
#
# Env overrides: LOAD_CONCURRENCY, LOAD_WARMUP, LOAD_DURATION, LOAD_SEED,
# PROFILE_TRACE_SECONDS (default 10).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=lib.sh
source ./lib.sh
require_go_tool

require_active_server
log "profile-trace: using active run: $RUN_DIR"
BIN="$RUN_DIR/bin/xchats-profile"
[ -x "$BIN" ] || die "expected binary $BIN from the active profile-server, but it's missing"

mkdir -p "$RUN_DIR/profiles" "$RUN_DIR/reports"

TRACE_SECONDS="${PROFILE_TRACE_SECONDS:-10}"
WARMUP="${LOAD_WARMUP:-5s}"
WARMUP_SECONDS="${WARMUP%s}"
DEFAULT_DURATION_SECONDS=$((WARMUP_SECONDS + TRACE_SECONDS + 10))
DURATION="${LOAD_DURATION:-${DEFAULT_DURATION_SECONDS}s}"
CONCURRENCY="${LOAD_CONCURRENCY:-32}"
SEED="${LOAD_SEED:-42}"

LOAD_BIN="$RUN_DIR/bin/xchats-load"
log "profile-trace: building the load tool"
(cd "$ROOT/scripts/load" && go build -o "$LOAD_BIN" .)

log "profile-trace: running load — concurrency=$CONCURRENCY warmup=$WARMUP duration=$DURATION seed=$SEED"
"$LOAD_BIN" \
	-base-url "http://$HTTP_ADDR" \
	-concurrency "$CONCURRENCY" \
	-warmup "$WARMUP" \
	-duration "$DURATION" \
	-seed "$SEED" \
	-json-out "$RUN_DIR/reports/load-trace.json" \
	>"$RUN_DIR/reports/load-trace.txt" 2>&1 &
LOAD_PID=$!

sleep "$((WARMUP_SECONDS + 1))"
if ! kill -0 "$LOAD_PID" 2>/dev/null; then
	cat "$RUN_DIR/reports/load-trace.txt" >&2 || true
	die "load run exited before the trace capture window even started — see reports/load-trace.txt above"
fi

log "profile-trace: capturing ${TRACE_SECONDS}s execution trace"
curl -fsS --max-time "$((TRACE_SECONDS + 10))" \
	"http://$PPROF_ADDR/debug/pprof/trace?seconds=$TRACE_SECONDS" \
	-o "$RUN_DIR/profiles/trace.out"

log "profile-trace: waiting for the load run to finish"
LOAD_EXIT=0
wait "$LOAD_PID" || LOAD_EXIT=$?
cat "$RUN_DIR/reports/load-trace.txt"
if [ "$LOAD_EXIT" -ne 0 ]; then
	log "profile-trace: WARNING — the load run reported errors (exit $LOAD_EXIT); see reports/load-trace.txt"
fi

log "profile-trace: validating the trace (go tool trace -pprof=sched)"
if ! go_tool trace -pprof=sched "$RUN_DIR/profiles/trace.out" \
	>"$RUN_DIR/reports/trace-sched.pprof" 2>"$RUN_DIR/reports/trace-validate.log"; then
	cat "$RUN_DIR/reports/trace-validate.log" >&2 || true
	die "trace validation failed — see reports/trace-validate.log"
fi
[ -s "$RUN_DIR/reports/trace-sched.pprof" ] || die "trace validated with no error but produced an empty sched profile"
log "profile-trace: trace validated OK — reports/trace-sched.pprof"

log "profile-trace: launching go tool trace (Ctrl-C to stop)"
go_tool trace "$RUN_DIR/profiles/trace.out"
exit "$LOAD_EXIT"
