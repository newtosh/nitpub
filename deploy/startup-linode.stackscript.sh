#!/bin/bash
# <UDF name="domain" label="Domain (public hostname pointed at this Linode)" />
# <UDF name="actor" label="Federation actor (handle username)" default="user" />
#
# Publish this as a Linode StackScript (Cloud Manager > StackScripts > Create)
# to get a shareable "Deploy" link with domain/actor as real form fields —
# see docsite/docs/guide/one-click-deploy.md. Runs as root on first boot;
# installs nitpub unattended via the public install.sh release path (see
# docsite/docs/guide/install.md for the interactive equivalent).

set -euo pipefail

CONFIG_SECRET="$(openssl rand -hex 32)"
ADMIN_PASSWORD="$(openssl rand -hex 16)"

# Admin password goes to a root-only file, not console/boot-log output —
# most providers retain that output in the dashboard/API well past instance
# creation, which would leave the credential recoverable by anyone with
# later read access to the account. Retrieve it over SSH:
#   ssh root@YOUR_LINODE_IP cat /root/nitpub-admin-password
umask 077
echo "$ADMIN_PASSWORD" >/root/nitpub-admin-password
chmod 600 /root/nitpub-admin-password

curl -fsSL https://raw.githubusercontent.com/newtosh/nitpub/main/scripts/install.sh | bash -s -- \
  --yes \
  --domain "$DOMAIN" \
  --actor "$ACTOR" \
  --secret "$CONFIG_SECRET" \
  --password "$ADMIN_PASSWORD" \
  --with-caddy \
  --with-federation \
  --no-analytics

echo "==> nitpub installed. Log in at https://${DOMAIN}/login (admin password: /root/nitpub-admin-password)"
