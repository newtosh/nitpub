#!/usr/bin/env bash
# Seed fixture posts for local theme/UI rendering review.
# Requires: dev backend on :8080, admin account (default: admin / localdev).
#
# Usage:
#   ./scripts/seed-mock-posts.sh          # append fixtures
#   ./scripts/seed-mock-posts.sh --fresh  # delete all posts, then seed
#
# Production (2FA admin):
#   NITPUB_API=https://blog.example.com NITPUB_ADMIN_USER='you@example.com' \
#   NITPUB_ADMIN_PASS='…' NITPUB_ADMIN_TOTP='123456' \
#   ./scripts/seed-mock-posts.sh --fresh
#
# Or pull credentials from 1Password:
#   NITPUB_API=https://blog.example.com NITPUB_OP_ITEM=blog.example.com ./scripts/seed-mock-posts.sh --fresh
#
# Coverage matrix (12 posts):
#   note  plain | titled | blockquote | inline link | link card | long/truncated
#   article theme lab (callouts/table/code) | code blocks | CC figure | typography
#           plain publisher link | YouTube facade
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FIXTURES="$ROOT/scripts/seed-fixtures"
API="${NITPUB_API:-http://localhost:8080}"
USER="${NITPUB_ADMIN_USER:-}"
PASS="${NITPUB_ADMIN_PASS:-}"
OP_ITEM="${NITPUB_OP_ITEM:-}"
COOKIE_JAR="${TMPDIR:-/tmp}/nitpub-seed-cookies.txt"
FRESH=false

if [[ -n "$OP_ITEM" ]] && command -v op >/dev/null 2>&1; then
  [[ -z "$USER" ]] && USER="$(op item get "$OP_ITEM" --fields label=username 2>/dev/null || true)"
  [[ -z "$PASS" ]] && PASS="$(op item get "$OP_ITEM" --fields label=password 2>/dev/null || true)"
fi
USER="${USER:-admin}"
PASS="${PASS:-localdev}"

if [[ "${1:-}" == "--fresh" ]]; then
  FRESH=true
fi

resolve_totp() {
  if [[ -n "${NITPUB_ADMIN_TOTP:-}" ]]; then
    printf '%s' "$NITPUB_ADMIN_TOTP"
    return
  fi
  if [[ -n "$OP_ITEM" ]] && command -v op >/dev/null 2>&1; then
    op item get "$OP_ITEM" --otp 2>/dev/null || true
    return
  fi
}

verify_2fa() {
  local pending="$1"
  local method="${NITPUB_2FA_METHOD:-totp}"
  local code
  if [[ "$method" == "totp" ]]; then
    code="$(resolve_totp)"
    if [[ -z "$code" ]]; then
      echo "2FA required — set NITPUB_ADMIN_TOTP or NITPUB_OP_ITEM" >&2
      return 1
    fi
  else
    code="${NITPUB_ADMIN_BACKUP:-}"
    if [[ -z "$code" ]]; then
      echo "2FA required — set NITPUB_ADMIN_BACKUP for backup-code login" >&2
      return 1
    fi
  fi
  curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$API/api/auth/verify" \
    -H 'Content-Type: application/json' \
    -d "{\"pending_token\":\"$pending\",\"method\":\"$method\",\"code\":\"$code\"}" \
    -o /dev/null -w '%{http_code}'
}

login() {
  local resp http pending
  resp="$(curl -s -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$API/api/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" \
    -w $'\n%{http_code}')"
  http="${resp##*$'\n'}"
  body="${resp%$'\n'$http}"
  if [[ "$http" == "204" ]]; then
    printf '204'
    return
  fi
  if [[ "$http" != "200" ]]; then
    printf '%s' "$http"
    return
  fi
  pending="$(python3 - <<'PY' "$body"
import json, sys
data = json.loads(sys.argv[1])
if data.get("status") != "2fa_required":
    raise SystemExit(1)
print(data.get("pending_token", ""))
PY
)" || {
    printf '401'
    return
  }
  verify_2fa "$pending"
}

create_post() {
  local kind="$1"
  local file="$2"
  python3 - "$kind" "$file" <<'PY' | curl -s -b "$COOKIE_JAR" -X POST "$API/api/posts" \
    -H 'Content-Type: application/json' -d @- -w '\n%{http_code}\n'
import json, pathlib, sys
kind, path = sys.argv[1], pathlib.Path(sys.argv[2])
print(json.dumps({"kind": kind, "content": path.read_text()}))
PY
}

delete_all_posts() {
  echo "==> deleting existing posts"
  local slugs
  slugs="$(curl -sf -b "$COOKIE_JAR" "$API/api/posts" || echo '[]')"
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    code="$(curl -s -b "$COOKIE_JAR" -X DELETE "$API/api/posts/$slug" -o /dev/null -w '%{http_code}')"
    echo "  deleted $slug ($code)"
  done < <(python3 -c 'import json,sys
for post in json.load(sys.stdin):
    print(post["id"].rsplit("/", 1)[-1])
' <<<"$slugs")
}

fixtures=(
  'note|note-plain.md'
  'note|note-titled.md'
  'note|note-blockquote.md'
  'note|note-inline-link.md'
  'note|note-link-card.md'
  'note|note-long.md'
  'article|article-theme-lab.md'
  'article|article-code-blocks.md'
  'article|article-cc-photo.md'
  'article|article-plain-link.md'
  'article|article-youtube.md'
  'article|article-typography.md'
)

code="$(login)"
if [[ "$code" != "204" ]]; then
  echo "login failed (HTTP $code) — run: nitpub admin init" >&2
  exit 1
fi

if $FRESH; then
  delete_all_posts
fi

created=0
for entry in "${fixtures[@]}"; do
  kind="${entry%%|*}"
  file="${entry#*|}"
  path="$FIXTURES/$file"
  if [[ ! -f "$path" ]]; then
    echo "missing fixture: $path" >&2
    exit 1
  fi
  resp="$(create_post "$kind" "$path")"
  status="${resp##*$'\n'}"
  body="${resp%$'\n'$status}"
  if [[ "$status" == "201" ]]; then
    id="$(python3 -c 'import json,sys; print(json.loads(sys.stdin.read())["id"])' <<<"$body")"
    echo "created $kind ($file): $id"
    created=$((created + 1))
  else
    echo "failed $kind/$file (HTTP $status): $body" >&2
  fi
done

rm -f "$COOKIE_JAR"
echo "done — $created posts seeded"
