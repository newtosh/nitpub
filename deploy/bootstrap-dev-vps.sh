#!/usr/bin/env bash
# One-time VPS setup for git-pull deploy workflow during active development.
# Run ON the VPS as root (or with sudo):
#   curl -fsSL ... | bash   OR   bash deploy/bootstrap-dev-vps.sh
#
# Requires:
#   NITPUB_REPO  — GitHub repo slug, e.g. jonn/nitpub (private OK after gh auth)
#   NITPUB_DOMAIN — public hostname, e.g. blog.example.com
#   NITPUB_ACTOR   — federation username (default: user) → acct:{actor}@{domain}
set -euo pipefail

NITPUB_REPO="${NITPUB_REPO:?set NITPUB_REPO e.g. jonn/nitpub}"
NITPUB_DOMAIN="${NITPUB_DOMAIN:?set NITPUB_DOMAIN e.g. blog.example.com}"
NITPUB_ACTOR="${NITPUB_ACTOR:-user}"
NITPUB_REPO_DIR="${NITPUB_REPO_DIR:-/var/lib/nitpub/src}"
INSTALL_BIN="${NITPUB_INSTALL_BIN:-/usr/local/bin/nitpub}"
DATA_DIR="${NITPUB_DATA_DIR:-/var/lib/nitpub}"
CONFIG_DIR="/etc/nitpub"
CONFIG_FILE="$CONFIG_DIR/config.toml"

echo "==> Installing system packages (Debian/Ubuntu)"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq git curl ca-certificates golang nodejs npm caddy openssl

if ! command -v gh >/dev/null 2>&1; then
  echo "==> Installing GitHub CLI"
  curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | tee /usr/share/keyrings/githubcli-archive-keyring.gpg >/dev/null
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | tee /etc/apt/sources.list.d/github-cli.list >/dev/null
  apt-get update -qq
  apt-get install -y -qq gh
fi

if ! id nitpub >/dev/null 2>&1; then
  echo "==> Creating nitpub system user"
  useradd --system --home "$DATA_DIR" --shell /usr/sbin/nologin nitpub
fi

mkdir -p "$DATA_DIR" "$CONFIG_DIR" "$(dirname "$NITPUB_REPO_DIR")"
chown nitpub:nitpub "$DATA_DIR"

if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "==> Writing $CONFIG_FILE"
  SECRET="$(openssl rand -hex 32)"
  cat >"$CONFIG_FILE" <<EOF
domain = "$NITPUB_DOMAIN"
port = 8080
data_dir = "$DATA_DIR"
actor = "$NITPUB_ACTOR"
secret = "$SECRET"
http = false
system_user = "nitpub"
EOF
  chmod 640 "$CONFIG_FILE"
  chown root:nitpub "$CONFIG_FILE"
  echo "save secret from config: grep '^secret' $CONFIG_FILE"
fi

if [[ ! -d "$NITPUB_REPO_DIR/.git" ]]; then
  echo "==> Authenticate gh for private repo access (one-time)"
  echo "    Run as a user with repo access, then re-run bootstrap if clone fails."
  if ! gh auth status >/dev/null 2>&1; then
    gh auth login
  fi
  echo "==> Cloning github.com/$NITPUB_REPO"
  gh repo clone "$NITPUB_REPO" "$NITPUB_REPO_DIR"
  chown -R nitpub:nitpub "$NITPUB_REPO_DIR"
fi

echo "==> Installing systemd unit"
install -m 644 "$(dirname "$0")/nitpub.service" /etc/systemd/system/nitpub.service
systemctl daemon-reload
systemctl enable nitpub

echo "==> Building and starting"
NITPUB_REPO_DIR="$NITPUB_REPO_DIR" NITPUB_INSTALL_BIN="$INSTALL_BIN" bash "$(dirname "$0")/update.sh"

echo ""
echo "Bootstrap complete."
echo "  config:  $CONFIG_FILE"
echo "  binary:  $INSTALL_BIN"
echo "  source:  $NITPUB_REPO_DIR"
echo ""
echo "Admin CLI (same config file, no extra env):"
echo "  sudo -u nitpub nitpub admin init --username YOU"
echo ""
echo "After you push changes:"
echo "  ssh your-host 'cd $NITPUB_REPO_DIR && git pull && bash deploy/update.sh'"
