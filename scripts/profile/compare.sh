#!/usr/bin/env bash
# scripts/profile/compare.sh — `make profile-compare BASE=… NEW=…`: run
# benchstat over two profile-bench outputs, pinned to a Go-compatible
# golang.org/x/perf release so this never depends on the ambient toolchain
# having benchstat installed.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=lib.sh
source ./lib.sh

BASE="${BASE:?set BASE=path/to/baseline-bench-output.txt (from make profile-bench)}"
NEW="${NEW:?set NEW=path/to/candidate-bench-output.txt (from make profile-bench)}"
[ -f "$BASE" ] || die "BASE file not found: $BASE"
[ -f "$NEW" ] || die "NEW file not found: $NEW"

BENCHSTAT_PKG='golang.org/x/perf/cmd/benchstat@v0.0.0-20260813145340-fd4a688df892'
log "profile-compare: go run $BENCHSTAT_PKG $BASE $NEW"
go run "$BENCHSTAT_PKG" "$BASE" "$NEW"
