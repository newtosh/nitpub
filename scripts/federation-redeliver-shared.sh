#!/usr/bin/env bash
# Re-send already-federated posts to followers (stable IDs; safe to retry).
#
# Usage:
#   NITPUB_URL=https://blog.example.com NITPUB_USER=admin NITPUB_PASS='...' bash scripts/federation-redeliver-shared.sh
set -euo pipefail

: "${NITPUB_URL:?set NITPUB_URL}"
: "${NITPUB_USER:?set NITPUB_USER}"
: "${NITPUB_PASS:?set NITPUB_PASS}"

base="${NITPUB_URL%/}"
cookie_jar="$(mktemp)"
trap 'rm -f "$cookie_jar"' EXIT

login_resp="$(curl -sS -c "$cookie_jar" -X POST "${base}/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"${NITPUB_USER}\",\"password\":\"${NITPUB_PASS}\"}")"

if ! echo "$login_resp" | jq -e '.ok == true' >/dev/null 2>&1; then
  echo "login failed: $login_resp" >&2
  exit 1
fi

resp="$(curl -sS -b "$cookie_jar" -X POST "${base}/api/admin/federation/redeliver-shared")"
echo "$resp" | jq .
sent="$(echo "$resp" | jq -r '.sent // empty')"
if [[ -z "$sent" ]]; then
  echo "redeliver failed" >&2
  exit 1
fi
echo "redelivered ${sent} post(s)"
