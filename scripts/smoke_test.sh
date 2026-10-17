#!/usr/bin/env bash
# End-to-end smoke test against a running java-service (default mock Go client).
# Usage: ./scripts/smoke_test.sh [baseUrl]
set -euo pipefail

BASE_URL="${1:-http://localhost:8000}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "== health =="
HEALTH=$(curl -fsS "$BASE_URL/health")
echo "$HEALTH"
echo "$HEALTH" | grep -q '"status":"ok"'

check_generate() {
  local label="$1"
  local payload="$2"
  local expected_hours="$3"
  echo "== generate: $label =="
  local tmp
  tmp=$(mktemp)
  local code
  code=$(curl -sS --max-time 600 -o "$tmp" -w "%{http_code}" \
    -X POST "$BASE_URL/api/generate" \
    -H "Content-Type: application/json" \
    -d "$payload")
  echo "HTTP $code"
  if [[ "$code" != "200" ]]; then
    cat "$tmp" >&2
    rm -f "$tmp"
    echo "FAIL: expected HTTP 200" >&2
    exit 1
  fi

  python3 - "$tmp" "$expected_hours" <<'PY'
import json, sys
path, expected = sys.argv[1], float(sys.argv[2])
with open(path, encoding="utf-8") as f:
    body = json.load(f)
prog = body["program"]
header = float(prog["header"]["totalHours"])
totals = float(prog["curriculum"]["totals"]["total"])
# Sum modules + final attestation
mod_sum = sum(float(m["hours"]["total"]) for m in prog["curriculum"]["modules"])
fa = float(prog["finalAttestation"]["hours"]["total"])
computed = mod_sum + fa
print(f"header={header} totals={totals} modules+fa={computed} expected={expected}")
assert body.get("status") == "validated", body.get("status")
assert abs(header - expected) < 1e-6, (header, expected)
assert abs(totals - expected) < 1e-6, (totals, expected)
assert abs(computed - expected) < 1e-6, (computed, expected)
# contact = lecture+practice+lab on totals
t = prog["curriculum"]["totals"]
contact = float(t["lecture"]) + float(t["practice"]) + float(t["lab"])
assert abs(contact - float(t["contact"])) < 1e-6, (contact, t["contact"])
print("hours OK")
PY
  rm -f "$tmp"
}

check_generate "DPP 72h" \
  '{"request":"ДПП ПК Анализ данных на Python, 72 ч","answers":{"entryLevel":"Базовый Python"}}' \
  72

check_generate "DOOP 24h" \
  '{"request":"ДООП Робототехника для начинающих, 24 ч","answers":{"entryLevel":"нулевой"}}' \
  24

echo "== list programs =="
curl -fsS "$BASE_URL/programs" | head -c 200
echo
echo "SMOKE OK"
