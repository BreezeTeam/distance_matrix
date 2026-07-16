#!/usr/bin/env bash
# Simple matrix load smoke test (requires running server).
set -euo pipefail
URL="${1:-http://127.0.0.1:8888/v1/matrix}"
N="${2:-20}"
BODY='{"points":[[116.40,39.90],[116.41,39.91],[116.42,39.92],[116.43,39.93]],"coordinate":"gcj02"}'

echo "POST $URL x $N"
for i in $(seq 1 "$N"); do
  curl -sf -o /dev/null -w "%{http_code} %{time_total}s\n" \
    -X POST "$URL" \
    -H "Content-Type: application/json" \
    -H "X-Tenant-Id: loadtest" \
    -d "$BODY" &
done
wait
echo done
