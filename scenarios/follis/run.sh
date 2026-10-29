#!/usr/bin/env bash
# Runs the Follis pilot rehearsal end to end on a fresh SQLite database:
# builds the backend, starts it (mock AI by default) and the Vite frontend,
# runs simulate.mjs, then stops both. Output lands in scenarios/follis/out/.
#
#   scenarios/follis/run.sh              # mock AI (no network, deterministic)
#   scenarios/follis/run.sh --llm=real   # real model: needs LLM_API_KEY (+ LLM_PROVIDER,
#                                        # LLM_DEFAULT_MODEL) and network access to the provider
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
OUT="$HERE/out"
LLM="mock"
for arg in "$@"; do
  case "$arg" in
    --llm=*) LLM="${arg#--llm=}" ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done
if [[ "$LLM" == "real" && -z "${LLM_API_KEY:-}" ]]; then
  echo "--llm=real needs LLM_API_KEY (and LLM_PROVIDER / LLM_DEFAULT_MODEL) in the environment" >&2
  exit 2
fi

for port in 8080 5173; do
  if (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null; then
    echo "port $port is busy — stop whatever is listening there first (make kill-ports)" >&2
    exit 1
  fi
done

# The rehearsal is hermetic: environment overrides meant for a developer's
# own instance (public URLs, CORS, Langfuse tracing, Telegram webhooks) must
# not leak into it. config.yaml next to this script is the only boot config;
# the LLM_* variables stay, since --llm=real needs them.
unset CORS_ORIGINS FRONTEND_BASE_URL HTTP_ADDR TG_WEBHOOK_PUBLIC_BASE_URL \
  LANGFUSE_PUBLIC_KEY LANGFUSE_SECRET_KEY LANGFUSE_HOST LANGFUSE_BASE_URL SIMULATOR_ENABLED
# Vite's dev proxy and the backend's own link base both read this.
export API_BASE_URL=http://localhost:8080

rm -rf "$OUT"
mkdir -p "$OUT/data" "$OUT/logs"

echo "▶ building backend"
(cd "$REPO/backend" && go build -o "$OUT/xchats" ./cmd/xchats)
if [[ ! -d "$REPO/frontend/node_modules" ]]; then
  echo "▶ installing frontend dependencies"
  (cd "$REPO/frontend" && npm ci --no-audit --no-fund)
fi

pids=()
cleanup() { for p in "${pids[@]}"; do kill "$p" 2>/dev/null || true; done; wait 2>/dev/null || true; }
trap cleanup EXIT

backend_flags=()
[[ "$LLM" == "mock" ]] && backend_flags+=(-mock-externals)
echo "▶ starting backend ($LLM AI) on :8080"
(cd "$OUT" && XCHATS_ALLOW_FILE_CREDENTIALS=1 exec ./xchats -config ../config.yaml "${backend_flags[@]}" serve) >"$OUT/logs/backend.log" 2>&1 &
pids+=($!)
echo "▶ starting frontend on :5173"
(cd "$REPO/frontend" && exec ./node_modules/.bin/vite --port 5173 --strictPort) >"$OUT/logs/frontend.log" 2>&1 &
pids+=($!)

for _ in $(seq 1 60); do
  curl -sf http://localhost:8080/readyz >/dev/null && curl -sf http://localhost:5173/login >/dev/null && break
  sleep 1
done
curl -sf http://localhost:8080/readyz >/dev/null || { echo "backend did not start — see $OUT/logs/backend.log" >&2; exit 1; }
curl -sf http://localhost:5173/login >/dev/null || { echo "frontend did not start — see $OUT/logs/frontend.log" >&2; exit 1; }

echo "▶ running the Follis simulation"
status=0
node "$HERE/simulate.mjs" "--llm=$LLM" || status=$?

echo "▶ backend errors during the run:"
grep -E 'level=(ERROR|WARN)' "$OUT/logs/backend.log" | tail -20 || echo "  (none)"
exit $status
