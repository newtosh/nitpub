#!/usr/bin/env bash
# Replace apex demo posts with intentional marketing fixtures.
# Usage:
#   NITPUB_API=https://nitpub.com NITPUB_OP_ITEM=nitpub.com ./scripts/seed-demo.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
API="${NITPUB_API:-https://nitpub.com}"
OP_ITEM="${NITPUB_OP_ITEM:-nitpub.com}"
USER="${NITPUB_ADMIN_USER:-}"
PASS="${NITPUB_ADMIN_PASS:-}"
COOKIE_JAR="${TMPDIR:-/tmp}/nitpub-demo-cookies.txt"
FIXTURES="$ROOT/scripts/demo-fixtures"

if [[ -n "$OP_ITEM" ]] && command -v op >/dev/null 2>&1; then
  [[ -z "$USER" ]] && USER="$(op item get "$OP_ITEM" --fields label=username --reveal 2>/dev/null || true)"
  [[ -z "$PASS" ]] && PASS="$(op item get "$OP_ITEM" --fields label=password --reveal 2>/dev/null || true)"
fi
USER="${USER:?set NITPUB_ADMIN_USER or NITPUB_OP_ITEM}"
PASS="${PASS:?set NITPUB_ADMIN_PASS or NITPUB_OP_ITEM}"
rm -f "$COOKIE_JAR"

resolve_totp() {
  if [[ -n "${NITPUB_ADMIN_TOTP:-}" ]]; then
    printf '%s' "$NITPUB_ADMIN_TOTP"
    return
  fi
  op item get "$OP_ITEM" --otp 2>/dev/null || true
}

resp="$(curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$API/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" -w $'\n%{http_code}')"
http="${resp##*$'\n'}"
body="${resp%$'\n'$http}"
if [[ "$http" == "200" ]]; then
  pending="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1]).get("pending_token",""))' "$body")"
  code="$(resolve_totp)"
  http="$(curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$API/api/auth/verify" \
    -H 'Content-Type: application/json' \
    -d "{\"pending_token\":\"$pending\",\"method\":\"totp\",\"code\":\"$code\"}" -o /dev/null -w '%{http_code}')"
fi
[[ "$http" == "204" ]] || { echo "login failed (HTTP $http)" >&2; exit 1; }

slugs="$(curl -sf -b "$COOKIE_JAR" "$API/api/posts")"
while IFS= read -r slug; do
  [[ -z "$slug" ]] && continue
  curl -s -b "$COOKIE_JAR" -X DELETE "$API/api/posts/$slug" -o /dev/null -w "deleted $slug (%{http_code})\n"
done < <(python3 -c 'import json,sys
for post in json.load(sys.stdin):
    print(post["id"].rsplit("/", 1)[-1])' <<<"$slugs")

for entry in 'note|note-welcome.md' 'article|article-why-nitpub.md'; do
  kind="${entry%%|*}"
  file="${entry##*|}"
  python3 - "$kind" "$FIXTURES/$file" <<'PY' | curl -s -b "$COOKIE_JAR" -X POST "$API/api/posts" \
    -H 'Content-Type: application/json' -d @- -w '\n%{http_code}\n'
import json, pathlib, sys
kind, path = sys.argv[1], pathlib.Path(sys.argv[2])
print(json.dumps({"kind": kind, "content": path.read_text()}))
PY
done
echo "demo seed complete"
