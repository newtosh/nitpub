#!/usr/bin/env bash
# Automated preflight for Federation Core U9 (public discovery + TLS).
# Manual Mastodon follow/reply steps: docs/federation-interop.md
set -euo pipefail

DOMAIN="${DOMAIN:?set DOMAIN to your public hostname (e.g. blog.example.com)}"
ACTOR="${ACTOR:?set ACTOR to your config actor username (federation handle)}"
BASE="https://${DOMAIN}"

fail=0

check() {
  local name="$1"
  shift
  if "$@"; then
    echo "  ok  ${name}"
  else
    echo "  FAIL  ${name}" >&2
    fail=1
  fi
}

echo "==> Federation preflight for ${DOMAIN} (actor: ${ACTOR})"

check "healthz" curl -fsS "${BASE}/healthz" | grep -q '"status":"ok"'

WF=$(curl -fsS "${BASE}/.well-known/webfinger?resource=acct:${ACTOR}@${DOMAIN}")
check "webfinger subject" bash -c "echo '${WF}' | grep -q '\"subject\":\"acct:${ACTOR}@${DOMAIN}\"'"
check "webfinger self link" bash -c "echo '${WF}' | grep -q '${BASE}/actor'"
actor_ci="${ACTOR^}"
check "webfinger case-insensitive" bash -c "curl -fsS '${BASE}/.well-known/webfinger?resource=acct:${actor_ci}@${DOMAIN}' | grep -q '\"subject\":\"acct:${ACTOR}@${DOMAIN}\"'"

check "host-meta lrdd" bash -c "curl -fsS '${BASE}/.well-known/host-meta' | grep -q 'rel=\"lrdd\"'"
check "host-meta webfinger template" bash -c "curl -fsS '${BASE}/.well-known/host-meta' | grep -q '${BASE}/.well-known/webfinger'"

ACTOR_JSON=$(curl -fsS "${BASE}/actor")
check "actor id" bash -c "echo '${ACTOR_JSON}' | grep -q '\"id\":\"${BASE}/actor\"'"
check "actor inbox" bash -c "echo '${ACTOR_JSON}' | grep -q '\"inbox\":\"${BASE}/inbox\"'"
check "actor preferredUsername" bash -c "echo '${ACTOR_JSON}' | grep -q '\"preferredUsername\":\"${ACTOR}\"'"
check "actor url" bash -c "echo '${ACTOR_JSON}' | grep -q '\"url\":\"${BASE}\"'"
check "actor publicKey #main-key" bash -c "echo '${ACTOR_JSON}' | grep -q '#main-key'"
check "actor @context" bash -c "echo '${ACTOR_JSON}' | grep -qE '\"@context\"|\"context\"'"

check "outbox GET" bash -c "[[ \$(curl -fsS -o /dev/null -w '%{http_code}' '${BASE}/outbox') == '200' ]]"

check "inbox rejects unsigned POST" bash -c "[[ \$(curl -s -o /dev/null -w '%{http_code}' -X POST '${BASE}/inbox' -H 'Content-Type: application/activity+json' -d '{}') == '401' ]]"

echo ""
if [[ "${fail}" -eq 0 ]]; then
  echo "Preflight passed."
else
  echo "Preflight failed." >&2
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
echo ""
echo "==> Mastodon-style resolve check"
STALE_ACTORS="${STALE_ACTORS:-}" DOMAIN="$DOMAIN" ACTOR="$ACTOR" bash "$ROOT/scripts/federation-resolve.sh"
