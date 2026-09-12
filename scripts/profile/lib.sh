#!/usr/bin/env bash
# scripts/profile/lib.sh — shared shell functions for the hermetic profiling
# harness's Make targets (profile-server/profile-load/profile-view/
# profile-trace). Sourced, never executed directly.
#
# Run-directory layout, under .cache/profiles/<UTC-timestamp>-<pid>/:
#   meta.json          git revision/dirty state, Go version, process config,
#                      scenario parameters — written once by profile-server.
#   bin/xchats-profile the binary profile-server built (with symbols; -race
#                      when PROFILE_RACE=1).
#   data/              this run's ENTIRELY ISOLATED SQLite DB, whatsmeow
#                      device DB, blob dir, and app data dir — never the
#                      repo's own ./data.
#   server.log         the profiled server's stdout/stderr.
#   profiles/          raw pprof profiles + the trace, native Go formats.
#   reports/           `go tool pprof -top` text reports + load metrics.
#
# .cache/profiles/active.env is a plain KEY=VALUE file (shell-sourceable,
# no JSON dependency) naming the currently-running profile-server, if any —
# see write_active_marker/require_active_server.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_DIR="$ROOT/backend"
PROFILES_ROOT="$ROOT/.cache/profiles"
ACTIVE_MARKER="$PROFILES_ROOT/active.env"

# profile_http_addr/profile_pprof_addr are fixed, not overridable via env —
# the spec names 127.0.0.1:6060 as THE profiling pprof target, and a
# dedicated, distinct HTTP port keeps `make profile-server` from colliding
# with an ordinary `make dev-backend`/`make up` already running on :8080.
PROFILE_HTTP_ADDR="127.0.0.1:8099"
PROFILE_PPROF_ADDR="127.0.0.1:6060"

log() { printf '%s\n' "$*" >&2; }
die() {
	log "profile: $*"
	exit 1
}

# resolve_run_dir sets RUN_DIR and RUN_DIR_AUTO_CREATED (1 or 0). A
# user-supplied PROFILE_DATA_DIR always wins and is NEVER treated as
# auto-created (PROFILE_CLEAN=1 must never delete it — see cleanup_run_dir).
resolve_run_dir() {
	if [ -n "${PROFILE_DATA_DIR:-}" ]; then
		RUN_DIR="$(cd "$(dirname "$PROFILE_DATA_DIR")" 2>/dev/null && pwd)/$(basename "$PROFILE_DATA_DIR")" || RUN_DIR="$PROFILE_DATA_DIR"
		RUN_DIR_AUTO_CREATED=0
		mkdir -p "$RUN_DIR"
		return
	fi
	local stamp
	stamp="$(date -u +%Y%m%dT%H%M%SZ)"
	RUN_DIR="$PROFILES_ROOT/${stamp}-$$"
	RUN_DIR_AUTO_CREATED=1
	mkdir -p "$RUN_DIR"
}

# cleanup_run_dir removes RUN_DIR only when PROFILE_CLEAN=1 AND this
# invocation itself created it — a user-supplied PROFILE_DATA_DIR is never
# automatically deleted, regardless of PROFILE_CLEAN.
cleanup_run_dir() {
	if [ "${PROFILE_CLEAN:-0}" = "1" ] && [ "${RUN_DIR_AUTO_CREATED:-0}" = "1" ]; then
		log "PROFILE_CLEAN=1: removing auto-created run directory $RUN_DIR"
		rm -rf "$RUN_DIR"
	else
		log "preserving run directory: $RUN_DIR"
	fi
}

git_revision() { git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo unknown; }
git_dirty() {
	if git -C "$ROOT" diff --quiet --ignore-submodules HEAD 2>/dev/null; then
		echo false
	else
		echo true
	fi
}

# write_meta_json records this run's provenance — called once by
# profile-server right after RUN_DIR is resolved. Extra key=value pairs
# (scenario parameters) are passed as arguments, already JSON-safe.
write_meta_json() {
	local extra="$1"
	cat >"$RUN_DIR/meta.json" <<EOF
{
  "run_dir": "$RUN_DIR",
  "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "git_revision": "$(git_revision)",
  "git_dirty": $(git_dirty),
  "go_version": "$(go env GOVERSION 2>/dev/null || go version)",
  "http_addr": "$PROFILE_HTTP_ADDR",
  "pprof_addr": "$PROFILE_PPROF_ADDR",
  "race": ${PROFILE_RACE:-0},
  $extra
}
EOF
}

# write_active_marker records the live profile-server for profile-load/
# profile-view/profile-trace to find. Called by profile-server right
# before it execs the server binary (so PID matches the exec'd process on
# platforms where exec replaces the shell's own PID, i.e. always true for
# bash exec).
write_active_marker() {
	local server_pid="$1"
	mkdir -p "$PROFILES_ROOT"
	cat >"$ACTIVE_MARKER" <<EOF
RUN_DIR=$RUN_DIR
PID=$server_pid
HTTP_ADDR=$PROFILE_HTTP_ADDR
PPROF_ADDR=$PROFILE_PPROF_ADDR
AUTO_CREATED=$RUN_DIR_AUTO_CREATED
EOF
}

# clear_active_marker removes the marker IF it still points at pid — so a
# profile-server that lost a race with a newer one (or was already
# superseded) never clobbers the newer marker on its own way out.
clear_active_marker() {
	local pid="$1"
	[ -f "$ACTIVE_MARKER" ] || return 0
	if grep -q "^PID=$pid\$" "$ACTIVE_MARKER" 2>/dev/null; then
		rm -f "$ACTIVE_MARKER"
	fi
}

# require_active_server sources the marker (if any), validates the PID is
# still alive and the server actually answers /healthz, and dies with a
# clear message otherwise rather than silently writing into a stale or
# wrong run. On success, RUN_DIR/HTTP_ADDR/PPROF_ADDR are set in the
# caller's shell.
require_active_server() {
	[ -f "$ACTIVE_MARKER" ] || die "no active profile-server — run 'make profile-server' first (in another terminal; it runs in the foreground)"
	# shellcheck disable=SC1090
	source "$ACTIVE_MARKER"
	if ! kill -0 "$PID" 2>/dev/null; then
		die "active-run marker is stale (pid $PID is not running) — restart with 'make profile-server'"
	fi
	if ! curl -fsS --max-time 2 "http://$HTTP_ADDR/healthz" >/dev/null 2>&1; then
		die "active-run marker points at pid $PID but http://$HTTP_ADDR/healthz did not answer — restart with 'make profile-server'"
	fi
}

# require_pprof_tool checks `go tool pprof` is available before a capture
# target does real work — a missing Go toolchain component should fail
# fast with a clear message, not a confusing mid-capture error.
require_go_tool() {
	command -v go >/dev/null 2>&1 || die "go is not on PATH"
}

# go_tool runs `go tool <args>` from within BACKEND_DIR, so Go's toolchain
# auto-selection (per backend/go.mod's own `go` directive) resolves the
# SAME toolchain version that built/ran the profiled binary — go tool
# trace's wire format is not stable across Go versions (confirmed: a
# version-mismatched ambient `go` fails with "unknown or unsupported trace
# version"), and this keeps every `go tool pprof`/`go tool trace` call
# aligned with it rather than whatever `go` happens to resolve to from the
# caller's own working directory.
go_tool() {
	(cd "$BACKEND_DIR" && go tool "$@")
}
