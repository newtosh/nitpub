#!/usr/bin/env bash
# U9 status: preflight + optional VPS inspect (followers, inbox, RSS).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOMAIN="${DOMAIN:?set DOMAIN to your public hostname}"
ACTOR="${ACTOR:?set ACTOR to your config actor username}"
SSH_HOST="${SSH_HOST:-nitpub}"

echo "==> Automated preflight"
DOMAIN="$DOMAIN" ACTOR="$ACTOR" bash "$ROOT/scripts/federation-interop-preflight.sh"

if [[ "${SKIP_SSH:-}" == "1" ]]; then
  echo "SKIP_SSH=1 — not checking VPS state"
  exit 0
fi

echo ""
echo "==> VPS federation state (stops nitpub briefly)"
ssh "$SSH_HOST" "sudo systemctl stop nitpub && cd /var/lib/nitpub/src && sudo -u nitpub env PATH=/var/lib/nitpub/.local/go/bin:/usr/bin:/bin NITPUB_CONFIG=/etc/nitpub/config.toml go run scripts/federation-inspect.go; echo '--- RSS (KB) ---'; ps -o rss= -C nitpub 2>/dev/null || echo '(stopped)'; sudo systemctl start nitpub && sleep 0.5 && systemctl is-active nitpub"
