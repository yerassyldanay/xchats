#!/usr/bin/env bash
# scripts/profile/bench.sh — `make profile-bench OUT=path/to/output.txt`:
# ten-sample baseline/candidate benchmarks for profile-compare (benchstat)
# to compare — see docs on the optimization loop this harness supports.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
# shellcheck source=lib.sh
source ./lib.sh

OUT="${OUT:?set OUT=path/to/bench-output.txt (e.g. OUT=.cache/profiles/baseline.txt)}"
mkdir -p "$(dirname "$OUT")"

log "profile-bench: go test -bench=. -benchmem -count=10 -p=1 ./... -> $OUT (this can take a while)"
(cd "$BACKEND_DIR" && go test -run='^$' -bench=. -benchmem -benchtime=1s -count=10 -p=1 ./...) | tee "$OUT"
log "profile-bench: wrote $OUT"
