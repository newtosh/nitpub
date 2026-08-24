#!/usr/bin/env bash
# Publish U9 probe note + article via nitpub API (requires author credentials).
# Usage:
#   NITPUB_URL=https://blog.example.com NITPUB_USER=admin NITPUB_PASS='...' bash scripts/federation-u9-publish.sh
set -euo pipefail

BASE="${NITPUB_URL:?set NITPUB_URL to your instance base URL}"
USER="${NITPUB_USER:?set NITPUB_USER}"
PASS="${NITPUB_PASS:?set NITPUB_PASS}"
TOTP="${NITPUB_TOTP:-}"
STAMP="${U9_STAMP:-$(date -u +%Y-%m-%d)}"
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$COOKIE_JAR"' EXIT

login() {
  local resp status pending_token
  resp=$(curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" \
    "$BASE/api/auth/login")
  # A 204 No Content login (no 2FA) leaves $resp empty.
  [ -z "$resp" ] && return 0

  status=$(printf '%s' "$resp" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')
  if [ "$status" != "2fa_required" ]; then
    echo "unexpected login response: $resp" >&2
    return 1
  fi
  : "${TOTP:?login requires 2FA — set NITPUB_TOTP='123456' (e.g. \$(op item get <id> --otp))}"
  pending_token=$(printf '%s' "$resp" | python3 -c 'import json,sys; print(json.load(sys.stdin)["pending_token"])')
  curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H 'Content-Type: application/json' \
    -d "{\"pending_token\":\"$pending_token\",\"method\":\"totp\",\"code\":\"$TOTP\"}" \
    "$BASE/api/auth/verify" >/dev/null
}

publish() {
  local kind="$1" content="$2"
  curl -fsS -c "$COOKIE_JAR" -b "$COOKIE_JAR" \
    -H 'Content-Type: application/json' \
    -d "{\"kind\":\"$kind\",\"content\":$content,\"federate\":true}" \
    "$BASE/api/posts"
}

echo "==> Login $BASE as $USER"
login

export STAMP BASE
NOTE_JSON=$(python3 -c 'import json, os
stamp = os.environ["STAMP"]
base = os.environ["BASE"]
print(json.dumps(f"U9 note probe {stamp}\n\n**bold** and [nitpub]({base})"))')
ARTICLE_JSON=$(python3 -c 'import json, os
stamp = os.environ["STAMP"]
print(json.dumps(f"U9 article probe {stamp}\n\nFirst paragraph for Mastodon summary.\n\nMore body on nitpub permalink."))')
echo "==> Publish note (federate)"
publish note "$NOTE_JSON"
echo ""
echo "==> Publish article (federate)"
publish article "$ARTICLE_JSON"
echo ""
echo "Done. Verify delivery on your Mastodon home timeline."
