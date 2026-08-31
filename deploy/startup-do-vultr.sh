#!/usr/bin/env bash
# Paste this into DigitalOcean's Droplet "User Data" field, or Vultr's
# instance "Startup Script" field, at instance creation. Runs as root on
# first boot; installs nitpub unattended via the public install.sh release
# path (see docsite/docs/guide/install.md for the interactive equivalent).
#
# Edit these two values before pasting:
NITPUB_DOMAIN="blog.example.com"
NITPUB_ACTOR="user"

set -euo pipefail

if [[ -z "$NITPUB_DOMAIN" || "$NITPUB_DOMAIN" == "blog.example.com" ]]; then
  echo "error: edit NITPUB_DOMAIN at the top of this script before pasting it in" >&2
  exit 1
fi
if [[ -z "$NITPUB_ACTOR" ]]; then
  echo "error: NITPUB_ACTOR must not be empty" >&2
  exit 1
fi

CONFIG_SECRET="$(openssl rand -hex 32)"

# Reuse an existing password file on rerun instead of regenerating: `nitpub
# install` skips admin creation once an admin already exists, so a fresh
# random value here would silently desync the saved file from the real
# account password on any second run (recovery, re-pasted script, etc.).
if [[ -f /root/nitpub-admin-password ]]; then
  ADMIN_PASSWORD="$(cat /root/nitpub-admin-password)"
else
  ADMIN_PASSWORD="$(openssl rand -hex 16)"
fi

# Admin password goes to a root-only file, not console/boot-log output -
# most providers retain that output in the dashboard/API well past instance
# creation, which would leave the credential recoverable by anyone with
# later read access to the account. Retrieve it over SSH:
#   ssh root@YOUR_DROPLET_IP cat /root/nitpub-admin-password
(umask 077; echo "$ADMIN_PASSWORD" >/root/nitpub-admin-password)
chmod 600 /root/nitpub-admin-password

curl -fsSL https://raw.githubusercontent.com/newtosh/nitpub/main/scripts/install.sh | bash -s -- \
  --yes \
  --domain "$NITPUB_DOMAIN" \
  --actor "$NITPUB_ACTOR" \
  --secret "$CONFIG_SECRET" \
  --password "$ADMIN_PASSWORD" \
  --with-caddy \
  --with-federation \
  --no-analytics

echo "==> nitpub installed. Log in at https://${NITPUB_DOMAIN}/login (admin password: /root/nitpub-admin-password)"
