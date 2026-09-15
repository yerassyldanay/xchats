#!/usr/bin/env bash
# scripts/profile/view.sh — `make profile-view PROFILE=cpu|heap-alloc|
# heap-inuse|mutex|block`: reopen a captured profile's interactive pprof UI.
#
# Run selection, in order: an explicit RUN_DIR env var; else the active
# profile-server's own run (if one is live); else the most recently
# created run under .cache/profiles/.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=lib.sh
source ./lib.sh
require_go_tool

PROFILE="${PROFILE:-}"
[ -n "$PROFILE" ] || die "set PROFILE=cpu|heap-alloc|heap-inuse|mutex|block (e.g. 'make profile-view PROFILE=heap-alloc')"

if [ -n "${RUN_DIR:-}" ]; then
	log "profile-view: using explicit RUN_DIR=$RUN_DIR"
elif [ -f "$ACTIVE_MARKER" ] && (
	# shellcheck disable=SC1090
	source "$ACTIVE_MARKER" && kill -0 "$PID" 2>/dev/null
); then
	# shellcheck disable=SC1090
	source "$ACTIVE_MARKER"
	log "profile-view: using the active profile-server's run: $RUN_DIR"
else
	RUN_DIR="$(ls -1dt "$PROFILES_ROOT"/*/ 2>/dev/null | head -n1 | sed 's:/$::' || true)"
	[ -n "$RUN_DIR" ] || die "no active profile-server and no prior run found under $PROFILES_ROOT — run 'make profile-server' and 'make profile-load' first"
	log "profile-view: no active server — using the latest run: $RUN_DIR"
fi

BIN="$RUN_DIR/bin/xchats-profile"
[ -x "$BIN" ] || die "expected binary $BIN in $RUN_DIR — is this really a profile-server run directory?"

case "$PROFILE" in
cpu)
	PPROF_FLAGS=()
	FILE="$RUN_DIR/profiles/cpu.pprof"
	;;
heap-alloc)
	PPROF_FLAGS=(-alloc_space)
	FILE="$RUN_DIR/profiles/heap.pprof"
	;;
heap-inuse)
	PPROF_FLAGS=(-inuse_space)
	FILE="$RUN_DIR/profiles/heap.pprof"
	;;
mutex)
	PPROF_FLAGS=()
	FILE="$RUN_DIR/profiles/mutex.pprof"
	;;
block)
	PPROF_FLAGS=()
	FILE="$RUN_DIR/profiles/block.pprof"
	;;
*)
	die "unknown PROFILE=$PROFILE (want cpu, heap-alloc, heap-inuse, mutex, or block)"
	;;
esac

[ -s "$FILE" ] || die "$FILE is missing or empty — run 'make profile-load' against this run first"

log "profile-view: opening $FILE (PROFILE=$PROFILE) — Ctrl-C to stop the UI"
go_tool pprof -http=127.0.0.1:0 "${PPROF_FLAGS[@]}" "$BIN" "$FILE"
