#!/usr/bin/env bash
# scripts/profile/server.sh — `make profile-server`: build the backend with
# symbols (PROFILE_RACE=1 adds -race), seed a freshly isolated database with
# demo data (mock-externals mode, so no real credentials are ever needed),
# and run it in the foreground with the dedicated pprof listener enabled.
#
# Env overrides: PROFILE_DATA_DIR (use this exact directory instead of
# auto-generating one under .cache/profiles/ — never auto-deleted),
# PROFILE_CLEAN=1 (delete an AUTO-CREATED run directory on exit),
# PROFILE_RACE=1 (build with -race).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=lib.sh
source ./lib.sh

resolve_run_dir
log "profile-server: run directory: $RUN_DIR"
if [ "$RUN_DIR_AUTO_CREATED" = "1" ]; then
	log "profile-server: auto-created — preserved after shutdown unless you set PROFILE_CLEAN=1"
else
	log "profile-server: using your own PROFILE_DATA_DIR — never automatically deleted"
fi

BUILD_FLAGS=()
if [ "${PROFILE_RACE:-0}" = "1" ]; then
	BUILD_FLAGS+=(-race)
	log "profile-server: PROFILE_RACE=1 — building with -race (slower, catches data races)"
fi

mkdir -p "$RUN_DIR/bin" "$RUN_DIR/data" "$RUN_DIR/profiles" "$RUN_DIR/reports"
BIN="$RUN_DIR/bin/xchats-profile"
log "profile-server: building $BIN"
(cd "$BACKEND_DIR" && go build "${BUILD_FLAGS[@]}" -o "$BIN" ./cmd/xchats)

# Every path this run touches is isolated under RUN_DIR/data — never the
# repo's own ./data, and never shared with a real deployment or another
# profiling run.
export DB_PATH="$RUN_DIR/data/xchats.db"
export WA_DEVICE_DB_PATH="$RUN_DIR/data/whatsmeow.db"
export BLOB_DIR="$RUN_DIR/data/blobdata"
export XCHATS_DATA_DIR="$RUN_DIR/data/appdata"
export XCHATS_ALLOW_FILE_CREDENTIALS=1
export ENVIRONMENT=development
export MOCK_EXTERNALS=true
export SIMULATOR_ENABLED=true
export HTTP_ADDR="$PROFILE_HTTP_ADDR"
export PPROF_ADDR="$PROFILE_PPROF_ADDR"
mkdir -p "$XCHATS_DATA_DIR"

log "profile-server: seeding demo data (mock-externals mode — includes mock channel credentials)"
"$BIN" seed >"$RUN_DIR/seed.log" 2>&1 || {
	log "profile-server: seed failed — see $RUN_DIR/seed.log"
	tail -n 40 "$RUN_DIR/seed.log" >&2 || true
	exit 1
}

write_meta_json "\"scenario\": {\"seed_log\": \"seed.log\", \"server_log\": \"server.log\"}"

log "profile-server: starting — http://$PROFILE_HTTP_ADDR  pprof on $PROFILE_PPROF_ADDR (loopback only)"
"$BIN" serve >"$RUN_DIR/server.log" 2>&1 &
SERVER_PID=$!

cleanup() {
	clear_active_marker "$SERVER_PID"
	kill "$SERVER_PID" 2>/dev/null || true
	wait "$SERVER_PID" 2>/dev/null || true
	cleanup_run_dir
}
trap cleanup EXIT INT TERM

# Wait for readiness before publishing the marker — a capture target must
# never see a marker for a server that isn't actually answering yet.
for _ in $(seq 1 100); do
	if curl -fsS --max-time 1 "http://$PROFILE_HTTP_ADDR/healthz" >/dev/null 2>&1; then
		break
	fi
	if ! kill -0 "$SERVER_PID" 2>/dev/null; then
		log "profile-server: server exited during startup — see $RUN_DIR/server.log"
		tail -n 40 "$RUN_DIR/server.log" >&2 || true
		exit 1
	fi
	sleep 0.1
done

write_active_marker "$SERVER_PID"
log "profile-server: ready (pid $SERVER_PID) — tailing server.log; Ctrl-C to stop"
log "profile-server: in another terminal: make profile-load"

# Foreground: block here, streaming the log, until the server exits or the
# operator interrupts (both drive the EXIT trap above).
tail -f "$RUN_DIR/server.log" &
TAIL_PID=$!
wait "$SERVER_PID" || true
kill "$TAIL_PID" 2>/dev/null || true
