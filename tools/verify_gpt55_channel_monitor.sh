#!/usr/bin/env bash
set -euo pipefail

APP_URL="${APP_URL:-http://127.0.0.1:18080}"
ADMIN_EMAIL="${ADMIN_EMAIL:?ADMIN_EMAIL is required}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:?ADMIN_PASSWORD is required}"
OPENAI_API_KEY="${OPENAI_API_KEY:?OPENAI_API_KEY is required}"
MODEL="${MODEL:-gpt-5.5}"
MONITOR_ENDPOINT="${MONITOR_ENDPOINT:-https://api.openai.com}"
MONITOR_NAME="${MONITOR_NAME:-OpenAI ${MODEL} Acceptance}"
MONITOR_ID="${MONITOR_ID:-}"
INTERVAL_SECONDS="${INTERVAL_SECONDS:-60}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

post_json() {
  local url="$1"
  local body="$2"
  local out="$3"
  curl -sS -o "$out" -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -X POST "$url" \
    -d "$body"
}

auth_post_json() {
  local url="$1"
  local body="$2"
  local out="$3"
  curl -sS -o "$out" -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${TOKEN}" \
    -X POST "$url" \
    -d "$body"
}

auth_put_json() {
  local url="$1"
  local body="$2"
  local out="$3"
  curl -sS -o "$out" -w '%{http_code}' \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${TOKEN}" \
    -X PUT "$url" \
    -d "$body"
}

echo "[1/4] Logging in to ${APP_URL} ..."
login_payload="$(python3 - <<'PY'
import json, os
print(json.dumps({
    "email": os.environ["ADMIN_EMAIL"],
    "password": os.environ["ADMIN_PASSWORD"],
    "turnstile_token": ""
}))
PY
)"
login_out="${tmp_dir}/login.json"
login_code="$(post_json "${APP_URL}/api/v1/auth/login" "${login_payload}" "${login_out}")"

if [[ "${login_code}" != "200" ]]; then
  echo "Login failed with HTTP ${login_code}" >&2
  cat "${login_out}" >&2
  exit 1
fi

TOKEN="$(python3 - <<'PY' "${login_out}"
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as f:
    data = json.load(f)
print(data["data"]["access_token"])
PY
)"

monitor_payload="$(python3 - <<'PY'
import json, os
print(json.dumps({
    "name": os.environ["MONITOR_NAME"],
    "provider": "openai",
    "endpoint": os.environ["MONITOR_ENDPOINT"],
    "api_key": os.environ["OPENAI_API_KEY"],
    "primary_model": os.environ["MODEL"],
    "extra_models": [],
    "group_name": "",
    "enabled": True,
    "interval_seconds": int(os.environ["INTERVAL_SECONDS"]),
}))
PY
)"

monitor_out="${tmp_dir}/monitor.json"
if [[ -n "${MONITOR_ID}" ]]; then
  echo "[2/4] Updating monitor id=${MONITOR_ID} ..."
  monitor_code="$(auth_put_json "${APP_URL}/api/v1/admin/channel-monitors/${MONITOR_ID}" "${monitor_payload}" "${monitor_out}")"
else
  echo "[2/4] Creating monitor ..."
  monitor_code="$(auth_post_json "${APP_URL}/api/v1/admin/channel-monitors" "${monitor_payload}" "${monitor_out}")"
fi

if [[ "${monitor_code}" != "200" && "${monitor_code}" != "201" ]]; then
  echo "Create/update failed with HTTP ${monitor_code}" >&2
  cat "${monitor_out}" >&2
  exit 1
fi

MONITOR_ID="$(python3 - <<'PY' "${monitor_out}"
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as f:
    data = json.load(f)
print(data["data"]["id"])
PY
)"

echo "[3/4] Running monitor id=${MONITOR_ID} model=${MODEL} ..."
run_out="${tmp_dir}/run.json"
run_code="$(auth_post_json "${APP_URL}/api/v1/admin/channel-monitors/${MONITOR_ID}/run" '{}' "${run_out}")"

if [[ "${run_code}" != "200" ]]; then
  echo "Run failed with HTTP ${run_code}" >&2
  cat "${run_out}" >&2
  exit 1
fi

echo "[4/4] Interpreting result ..."
python3 - <<'PY' "${run_out}" "${MONITOR_ID}" "${MODEL}"
import json
import sys

run_path, monitor_id, model = sys.argv[1:4]
with open(run_path, "r", encoding="utf-8") as f:
    data = json.load(f)

results = data.get("data", {}).get("results", [])
if not results:
    print("No run results returned", file=sys.stderr)
    sys.exit(1)

first = results[0]
status = first.get("status", "")
message = first.get("message", "")
latency = first.get("latency_ms")

print(json.dumps({
    "monitor_id": monitor_id,
    "model": model,
    "status": status,
    "latency_ms": latency,
    "message": message,
}, ensure_ascii=False, indent=2))

if status not in {"operational", "degraded"}:
    sys.exit(1)
PY
