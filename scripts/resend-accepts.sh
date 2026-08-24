#!/usr/bin/env bash
# Re-deliver Accept activities to all stored followers (fixes pending follow on Mastodon).
#
# Usage:
#   NITPUB_URL=https://blog.example.com NITPUB_USER=admin NITPUB_PASS='...' bash scripts/resend-accepts.sh
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

resp="$(curl -sS -b "$cookie_jar" -X POST "${base}/api/admin/federation/resend-accepts")"
echo "$resp" | jq .
sent="$(echo "$resp" | jq -r '.sent // empty')"
if [[ -z "$sent" ]]; then
  echo "resend failed" >&2
  exit 1
fi
echo "resent Accept to ${sent} follower(s)"
