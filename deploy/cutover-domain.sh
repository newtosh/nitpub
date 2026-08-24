#!/usr/bin/env bash
# Reconfigure nitpub droplet for a real domain + TLS (run after DNS propagates).
set -euo pipefail

DOMAIN="${1:?usage: deploy/cutover-domain.sh <domain>}"
DROPLET_HOST="${DROPLET_HOST:?set DROPLET_HOST to your SSH host alias}"
NITPUB_PORT="${NITPUB_PORT:-8080}"
NITPUB_ACTOR="${NITPUB_ACTOR:-user}"
CONFIG_FILE="/etc/nitpub/config.toml"

echo "Cutting over $DROPLET_HOST to https://$DOMAIN (actor: $NITPUB_ACTOR) ..."

ssh "$DROPLET_HOST" "DOMAIN='$DOMAIN' NITPUB_PORT='$NITPUB_PORT' NITPUB_ACTOR='$NITPUB_ACTOR' CONFIG_FILE='$CONFIG_FILE' bash -s" <<'REMOTE'
set -euo pipefail
DOMAIN="${DOMAIN:?}"
NITPUB_PORT="${NITPUB_PORT:-8080}"
NITPUB_ACTOR="${NITPUB_ACTOR:-user}"
CONFIG_FILE="${CONFIG_FILE:-/etc/nitpub/config.toml}"

cat > /etc/caddy/Caddyfile <<EOF
$DOMAIN, www.$DOMAIN {
	encode gzip
	header Referrer-Policy "strict-origin-when-cross-origin"
	reverse_proxy localhost:$NITPUB_PORT
}
EOF

mkdir -p /etc/nitpub
if [[ ! -f "$CONFIG_FILE" ]]; then
  SECRET="$(openssl rand -hex 32)"
  cat >"$CONFIG_FILE" <<EOF
domain = "$DOMAIN"
port = $NITPUB_PORT
data_dir = "/var/lib/nitpub"
actor = "$NITPUB_ACTOR"
secret = "$SECRET"
http = false
system_user = "nitpub"
EOF
  chmod 640 "$CONFIG_FILE"
  chown root:nitpub "$CONFIG_FILE"
  echo "created $CONFIG_FILE"
else
  # Update domain/port in place; preserve secret and data_dir.
  sed -i "s/^domain = .*/domain = \"$DOMAIN\"/" "$CONFIG_FILE"
  sed -i "s/^port = .*/port = $NITPUB_PORT/" "$CONFIG_FILE"
  if grep -q '^http = true' "$CONFIG_FILE"; then
    sed -i 's/^http = true/http = false/' "$CONFIG_FILE"
  fi
  SECRET="$(awk -F'"' '/^secret =/ {print $2; exit}' "$CONFIG_FILE")"
  if [[ "$SECRET" == "dev-secret-change-me" || "$SECRET" == "CHANGE-ME" ]]; then
    SECRET="$(openssl rand -hex 32)"
    sed -i "s/^secret = .*/secret = \"$SECRET\"/" "$CONFIG_FILE"
    echo "warning: rotated placeholder secret in $CONFIG_FILE"
  fi
fi

install -m 644 /var/lib/nitpub/src/deploy/nitpub.service /etc/systemd/system/nitpub.service 2>/dev/null \
  || install -m 644 "$(dirname "$0")/nitpub.service" /etc/systemd/system/nitpub.service 2>/dev/null \
  || true

systemctl daemon-reload
systemctl reload caddy
systemctl restart nitpub
sleep 3
systemctl is-active caddy nitpub
curl -fsS "https://$DOMAIN/healthz" || curl -fsS "http://$DOMAIN/healthz"
REMOTE

echo "Done. Verify:"
echo "  curl https://$DOMAIN/healthz"
echo "  curl 'https://$DOMAIN/.well-known/webfinger?resource=acct:$NITPUB_ACTOR@$DOMAIN'"
