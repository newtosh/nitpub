#!/usr/bin/env bash
# Simulate Mastodon's remote account resolution (WebFinger → actor document).
# Run before manual mobile tests to get high-confidence signal.
#
# Usage:
#   DOMAIN=blog.example.com ACTOR=you bash scripts/federation-resolve.sh
#
# Optional live probe from *your* Mastodon instance (needs user access token):
#   MASTODON_INSTANCE=https://mastodon.social MASTODON_TOKEN=... \
#     DOMAIN=blog.example.com ACTOR=you bash scripts/federation-resolve.sh
#
# Note: /api/v1/accounts/lookup only finds accounts already stored on that
# instance. With a token we use /api/v2/search?resolve=true (same as mobile
# "Go to @user@domain").
#
# Optional: comma-separated stale handles that should 404 (negative-cache traps):
#   STALE_ACTORS=nitpub bash scripts/federation-resolve.sh
set -euo pipefail

DOMAIN="${DOMAIN:?set DOMAIN to your public hostname}"
ACTOR="${ACTOR:?set ACTOR to your config actor username}"
BASE="https://${DOMAIN}"
HANDLE="@${ACTOR}@${DOMAIN}"
ACCT="acct:${ACTOR}@${DOMAIN}"
ACTOR_CI="${ACTOR^}"
UA="${FEDERATION_UA:-Mastodon/4.3.0 (nitpub federation-resolve)}"

fail=0
warn=0

step() { printf '  %-40s' "$1"; }
ok() { echo "ok"; }
bad() { echo "FAIL — $1" >&2; fail=1; }
note() { echo "warn — $1" >&2; warn=1; }

header_code() {
  curl -sS -o /dev/null -w "%{http_code}" -H "User-Agent: $UA" "$@"
}

header_value() {
  curl -sS -D - -o /dev/null -H "User-Agent: $UA" "$@" | awk -F': ' 'tolower($1)=="content-type" {print $2; exit}' | tr -d '\r'
}

echo "==> Federation resolve: ${HANDLE}"
echo "    (simulates Mastodon ResolveAccountService / WebFinger chain)"
echo ""

# --- Step 1: WebFinger ---
step "webfinger HTTP 200"
code=$(header_code "${BASE}/.well-known/webfinger?resource=${ACCT}")
if [[ "$code" == "200" ]]; then ok; else bad "HTTP $code"; fi

step "webfinger content-type"
ct=$(header_value "${BASE}/.well-known/webfinger?resource=${ACCT}")
if [[ "$ct" == *"jrd+json"* || "$ct" == *"json"* ]]; then ok; else bad "got ${ct:-<none>}"; fi

WF=$(curl -fsS -H "User-Agent: $UA" "${BASE}/.well-known/webfinger?resource=${ACCT}")
step "webfinger subject"
if echo "$WF" | jq -e --arg s "acct:${ACTOR}@${DOMAIN}" '.subject == $s' >/dev/null; then ok; else bad "subject mismatch"; fi

ACTOR_HREF=$(echo "$WF" | jq -r '.links[] | select(.rel=="self") | .href' | head -1)
step "webfinger self link"
if [[ -n "$ACTOR_HREF" && "$ACTOR_HREF" == "${BASE}/actor" ]]; then ok; else bad "href=${ACTOR_HREF:-<empty>}"; fi

step "webfinger case-insensitive"
code=$(header_code "${BASE}/.well-known/webfinger?resource=acct:${ACTOR_CI}@${DOMAIN}")
if [[ "$code" == "200" ]]; then ok; else bad "HTTP $code for acct:${ACTOR_CI}@${DOMAIN}"; fi

# --- Step 2: Actor document ---
step "actor HTTP 200"
code=$(header_code -H "Accept: application/activity+json" "${ACTOR_HREF}")
if [[ "$code" == "200" ]]; then ok; else bad "HTTP $code"; fi

step "actor content-type"
ct=$(header_value -H "Accept: application/activity+json" "${ACTOR_HREF}")
if [[ "$ct" == *"activity+json"* ]]; then ok; else bad "got ${ct:-<none>} (Mastodon requires activity+json)"; fi

ACTOR_JSON=$(curl -fsS -H "User-Agent: $UA" -H "Accept: application/activity+json" "${ACTOR_HREF}")
step "actor id"
if echo "$ACTOR_JSON" | jq -e --arg id "${BASE}/actor" '.id == $id' >/dev/null; then ok; else bad; fi

step "actor preferredUsername"
if echo "$ACTOR_JSON" | jq -e --arg u "$ACTOR" '.preferredUsername == $u' >/dev/null; then ok; else bad; fi

step "actor inbox/outbox"
if echo "$ACTOR_JSON" | jq -e --arg i "${BASE}/inbox" --arg o "${BASE}/outbox" '.inbox == $i and .outbox == $o' >/dev/null; then ok; else bad; fi

step "actor publicKey #main-key"
if echo "$ACTOR_JSON" | jq -e '.publicKey.id | endswith("#main-key")' >/dev/null; then ok; else bad; fi

step "actor publicKey PEM (PUBLIC KEY)"
if echo "$ACTOR_JSON" | jq -r '.publicKey.publicKeyPem' | grep -q 'BEGIN PUBLIC KEY'; then ok; else bad 'expected BEGIN PUBLIC KEY per Mastodon spec'; fi

step "actor open follows"
if echo "$ACTOR_JSON" | jq -e '.manuallyApprovesFollowers == false' >/dev/null; then ok; else bad 'set manuallyApprovesFollowers=false for instant follows'; fi

step "actor discoverable"
if echo "$ACTOR_JSON" | jq -e '.discoverable == true' >/dev/null; then ok; else bad 'set discoverable=true for Mastodon directory/search'; fi

step "actor url"
if echo "$ACTOR_JSON" | jq -e --arg u "$BASE" '.url == $u' >/dev/null; then ok; else bad "url should be blog homepage ($BASE)"; fi

step "homepage rel=me site"
HOME=$(curl -fsS -H "User-Agent: $UA" "$BASE/")
if echo "$HOME" | grep -Fq 'rel="me"' && echo "$HOME" | grep -Fq "$BASE"; then
  ok
else
  bad "missing <link rel=\"me\" href=\"${BASE}\"> on homepage"
fi

step "actor website attachment"
if echo "$ACTOR_JSON" | jq -e '.attachment[]? | select(.type=="PropertyValue") | .value | contains("rel=") and contains("me nofollow")' >/dev/null; then
  ok
else
  bad "missing Website PropertyValue with rel=me in actor attachment"
fi

step "actor @context (AS2 + security)"
CTX=$(echo "$ACTOR_JSON" | jq -c '.["@context"] // .context // empty')
if echo "$CTX" | grep -q 'activitystreams' && echo "$CTX" | grep -q 'security/v1'; then
  ok
else
  bad "missing ActivityStreams + security contexts"
fi

# --- Step 3: Stale handles (negative-cache traps) ---
if [[ -n "${STALE_ACTORS:-}" ]]; then
  IFS=',' read -ra stale <<<"$STALE_ACTORS"
  for s in "${stale[@]}"; do
    s="${s// /}"
    [[ -z "$s" || "$s" == "$ACTOR" ]] && continue
    step "stale handle @${s}@${DOMAIN} → 404"
    code=$(header_code "${BASE}/.well-known/webfinger?resource=acct:${s}@${DOMAIN}")
    if [[ "$code" == "404" ]]; then ok; else bad "HTTP $code (expected 404; wrong handle may be cached on Mastodon)"; fi
  done
fi

# --- Step 4: Optional Mastodon instance resolve (needs token) ---
MASTODON_INSTANCE="${MASTODON_INSTANCE:-}"
if [[ -n "$MASTODON_INSTANCE" && -n "${MASTODON_TOKEN:-}" ]]; then
  inst="${MASTODON_INSTANCE%/}"
  step "mastodon search resolve (mobile \"Go to\")"
  search_url="${inst}/api/v2/search?q=${HANDLE}&resolve=true&type=accounts&limit=1"
  resp=$(curl -sS -w "\n%{http_code}" -G \
    -H "Authorization: Bearer ${MASTODON_TOKEN}" \
    --data-urlencode "q=${HANDLE}" \
    --data-urlencode "resolve=true" \
    --data-urlencode "type=accounts" \
    --data-urlencode "limit=1" \
    "${inst}/api/v2/search")
  body="${resp%$'\n'*}"
  code="${resp##*$'\n'}"
  if [[ "$code" == "200" ]] && echo "$body" | jq -e --arg u "$ACTOR" --arg d "$DOMAIN" '.accounts[0].username == $u and (.accounts[0].acct | contains("@" + $d))' >/dev/null; then
    ok
  elif [[ "$code" == "200" ]] && echo "$body" | jq -e '.accounts | length == 0' >/dev/null; then
    bad "search returned no accounts — likely negative cache (~1h) after earlier wrong handle, or origin issue"
  elif [[ "$code" == "401" || "$code" == "403" ]]; then
    bad "HTTP $code — token needs read:search scope"
  else
    bad "HTTP $code from ${inst}/api/v2/search"
  fi
elif [[ -n "$MASTODON_INSTANCE" ]]; then
  step "mastodon search resolve"
  note "skipped — set MASTODON_TOKEN (Settings → Development) to test from ${MASTODON_INSTANCE}"
  ok
fi

echo ""
if [[ "$fail" -eq 0 ]]; then
  echo "VERDICT: Origin discovery looks correct for ${HANDLE}."
  echo ""
  echo "Mastodon mobile notes:"
  echo "  • Search is NOT a global directory — use the full handle: ${HANDLE}"
  echo "  • Tap \"Go to ${HANDLE}\" (not \"People matching…\") to force WebFinger resolve"
  echo "  • If you previously searched wrong handles (@nitpub@…, Nit without @, etc.),"
  echo "    Mastodon caches \"not found\" for ~1 hour on YOUR instance"
  echo "  • Workarounds: wait ~1h, try Follow → paste ${BASE}/actor, or test from another instance"
  if [[ -z "${MASTODON_INSTANCE:-}" || -z "${MASTODON_TOKEN:-}" ]]; then
    echo ""
    echo "Optional live check from your Mastodon server (same as mobile \"Go to\"):"
    echo "  MASTODON_INSTANCE=https://mastodon.social MASTODON_TOKEN=<token> \\"
    echo "    DOMAIN=${DOMAIN} ACTOR=${ACTOR} bash scripts/federation-resolve.sh"
  fi
  exit 0
fi

echo "VERDICT: Discovery chain failed — fix origin before trying mobile again." >&2
exit 1
