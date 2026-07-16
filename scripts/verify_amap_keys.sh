#!/usr/bin/env bash
# Verify each Amap key in etc/matrix.yaml against the driving API.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONFIG="${1:-$ROOT/etc/matrix.yaml}"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 required to parse yaml keys" >&2
  exit 1
fi

KEYS=$(python3 - <<PY
import re, pathlib
text = pathlib.Path("$CONFIG").read_text()
m = re.search(r'Keys:\s*(.+)', text)
if not m:
    raise SystemExit("Keys: not found in $CONFIG")
print(m.group(1).strip())
PY
)

ORIGIN="116.397428,39.90923"
DEST="116.407428,39.91923"
OK=0
FAIL=0

echo "Checking keys from $CONFIG"
echo "---"

IFS=',' read -ra ARR <<< "$KEYS"
for key in "${ARR[@]}"; do
  key="${key// /}"
  [ -z "$key" ] && continue
  resp=$(curl -sS --max-time 8 \
    "http://restapi.amap.com/v3/direction/driving?key=${key}&origin=${ORIGIN}&destination=${DEST}&strategy=11&output=json")
  status=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status','?'))")
  info=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('info','?'))")
  code=$(echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin).get('infocode','?'))")
  prefix="${key:0:8}"
  if [ "$status" = "1" ]; then
    echo "OK   ${prefix}…  infocode=${code}  ${info}"
    OK=$((OK + 1))
  else
    echo "FAIL ${prefix}…  infocode=${code}  ${info}"
    FAIL=$((FAIL + 1))
  fi
done

echo "---"
echo "${OK} ok, ${FAIL} failed"
[ "$OK" -gt 0 ] || exit 1
