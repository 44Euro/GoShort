#!/usr/bin/env bash
# วัด before/after ด้วย binary ตัวเดียว สลับด้วย GOSHORT_SYNC_MODE เท่านั้น
set -euo pipefail

cd "$(dirname "$0")/.."

PORT="${PORT:-8099}"
BASE="http://localhost:${PORT}"
VUS="${VUS:-100}"
DURATION="${DURATION:-30s}"
OUT="load-test/results"

export DATABASE_URL="${DATABASE_URL:-postgres://goshort:goshort@localhost:5432/goshort?sslmode=disable}"
export REDIS_URL="${REDIS_URL:-redis://localhost:6379/9}"
export JWT_SECRET="${JWT_SECRET:-benchmark}"
export ADMIN_PASSWORD="${ADMIN_PASSWORD:-goshort-demo}"
export PORT

mkdir -p "$OUT"
go build -o /tmp/goshort-bench ./cmd/api

start() {
  GOSHORT_SYNC_MODE="$1" /tmp/goshort-bench > "$OUT/server-$2.log" 2>&1 &
  echo $! > /tmp/goshort-bench.pid
  for _ in $(seq 1 40); do
    curl -sf "$BASE/health" >/dev/null 2>&1 && return 0
    sleep 0.5
  done
  echo "server did not come up" >&2
  exit 1
}

stop() {
  kill "$(cat /tmp/goshort-bench.pid)" 2>/dev/null || true
  wait "$(cat /tmp/goshort-bench.pid)" 2>/dev/null || true
}
trap stop EXIT

seed_codes() {
  local codes=""
  for i in 1 2 3 4 5; do
    local c
    c=$(curl -s -X POST "$BASE/api/links" -H 'content-type: application/json' \
      -d "{\"long_url\":\"https://go.dev/bench/$i/$RANDOM\"}" |
      python3 -c 'import sys,json;print(json.load(sys.stdin)["code"])')
    codes="${codes:+$codes,}$c"
  done
  echo "$codes"
}

run() {
  local mode="$1" label="$2"
  echo "── $label ──"
  start "$mode" "$label"
  local codes
  codes=$(seed_codes)
  # อุ่น cache ก่อนในโหมด async เพื่อวัดสภาพที่ระบบตั้งใจให้เป็น
  for c in ${codes//,/ }; do curl -s -o /dev/null "$BASE/$c"; done
  BASE_URL="$BASE" CODES="$codes" VUS="$VUS" DURATION="$DURATION" \
    k6 run --summary-trend-stats "avg,med,p(95),p(99),max" \
    --summary-export "$OUT/$label.json" load-test/redirect.js 2>&1 |
    tee "$OUT/$label.txt"
  stop
  sleep 2
}

run 1 sync-nocache
run 0 async-cache

python3 - "$OUT" <<'PY'
import json, sys, pathlib
out = pathlib.Path(sys.argv[1])
rows = []
for label, name in [("sync-nocache", "Sync write, no cache"), ("async-cache", "Async write + Redis cache")]:
    d = json.loads((out / f"{label}.json").read_text())
    m = d["metrics"]
    rows.append((name, m["http_reqs"]["rate"], m["http_req_duration"]["med"],
                 m["http_req_duration"]["p(95)"], m["http_req_duration"]["p(99)"]))

w = ["| Mode | req/s | p50 | p95 | p99 |", "|---|---:|---:|---:|---:|"]
for n, r, p50, p95, p99 in rows:
    w.append(f"| {n} | {r:,.0f} | {p50:.2f} ms | {p95:.2f} ms | {p99:.2f} ms |")
table = "\n".join(w)
(out / "table.md").write_text(table + "\n")
print("\n" + table)
PY
