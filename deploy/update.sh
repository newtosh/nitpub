#!/usr/bin/env bash
# Build and install nitpub from a git checkout, then restart the service.
# Run as root on the VPS, or as nitpub if passwordless sudo is configured.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="${NITPUB_REPO_DIR:-$(cd "$SCRIPT_DIR/.." && pwd)}"
INSTALL_BIN="${NITPUB_INSTALL_BIN:-/usr/local/bin/nitpub}"
SERVICE="${NITPUB_SERVICE:-nitpub}"
RUN_USER="${NITPUB_RUN_USER:-nitpub}"
BUILD_BIN="${REPO_DIR}/.build/nitpub"
NITPUB_HOME="${NITPUB_HOME:-/var/lib/nitpub}"
USER_PATH="${NITPUB_HOME}/.local/bin:${NITPUB_HOME}/.local/go/bin:/usr/bin:/bin"

if [[ ! -d "$REPO_DIR/.git" ]]; then
  echo "error: $REPO_DIR is not a git checkout" >&2
  echo "set NITPUB_REPO_DIR or run from the repo: bash deploy/update.sh" >&2
  exit 1
fi

run_as() {
  if [[ "$(id -un)" == "$RUN_USER" ]]; then
    "$@"
  else
    sudo -u "$RUN_USER" "$@"
  fi
}

as_root() {
  if [[ "$(id -un)" == "root" ]]; then
    "$@"
  elif sudo -n true 2>/dev/null; then
    sudo "$@"
  else
    echo "error: need root to $* (ssh as root, or passwordless sudo)" >&2
    exit 1
  fi
}

cd "$REPO_DIR"
run_as mkdir -p "${REPO_DIR}/.build"

echo "==> Pulling latest"
run_as git pull --ff-only
run_as git fetch --tags --force

# Production always reports a semver tag, never a commit hash — fail
# loudly (set -e) if none exists rather than silently shipping "dev".
VERSION="$(run_as git describe --tags --abbrev=0)"
echo "==> Version: $VERSION"

echo "==> Building PWA"
# A stray root-owned file under node_modules (e.g. from a one-off manual
# `npm install` run as root) makes `npm ci`'s own cleanup fail with
# ENOTEMPTY, since the sudo -u "$RUN_USER" install can delete its own
# files but not root's. Reassert ownership every run so this self-heals
# instead of failing deploys until someone notices and chowns by hand.
as_root chown -R "$RUN_USER":"$RUN_USER" "$REPO_DIR/web"
run_as env PATH="${USER_PATH}" bash -c "cd '$REPO_DIR/web' && npm ci && npm run build"

echo "==> Building nitpub"
run_as env PATH="${USER_PATH}" go build \
  -ldflags "-X github.com/newtosh/nitpub/internal/version.Version=${VERSION}" \
  -o "$BUILD_BIN" ./cmd/nitpub

echo "==> Installing $INSTALL_BIN"
as_root install -m 755 "$BUILD_BIN" "$INSTALL_BIN"

UNIT_SRC="$SCRIPT_DIR/nitpub.service"
UNIT_DST="/etc/systemd/system/${SERVICE}.service"
if [[ -f "$UNIT_SRC" ]]; then
  echo "==> Installing $UNIT_DST"
  as_root install -m 644 "$UNIT_SRC" "$UNIT_DST"
  as_root systemctl daemon-reload
fi

CONFIG_DST="/etc/nitpub/config.toml"
CONFIG_SRC="${NITPUB_CONFIG_SRC:-$NITPUB_HOME/.config/nitpub/config.toml}"
if [[ ! -f "$CONFIG_DST" ]]; then
  if [[ -f "$CONFIG_SRC" ]]; then
    echo "==> Installing $CONFIG_DST from $CONFIG_SRC"
    as_root mkdir -p /etc/nitpub
    as_root install -m 640 -o root -g "$RUN_USER" "$CONFIG_SRC" "$CONFIG_DST"
  elif [[ -f "$SCRIPT_DIR/config.toml.example" ]]; then
    echo "==> Installing $CONFIG_DST from example (edit secret before production)"
    as_root mkdir -p /etc/nitpub
    as_root install -m 640 -o root -g "$RUN_USER" "$SCRIPT_DIR/config.toml.example" "$CONFIG_DST"
  fi
fi

# Every enabled nitpub*.service unit shares this one binary (see
# docs/tasks/2026-07-17-multi-instance-deployment.md) — restart all of
# them, not just $SERVICE, so a second instance doesn't silently keep
# running the pre-update binary after a deploy.
mapfile -t UNITS < <(systemctl list-unit-files --type=service --no-legend 'nitpub*.service' 2>/dev/null | awk '{print $1}')
if [[ ${#UNITS[@]} -eq 0 ]]; then
  echo "no nitpub*.service units found — binary installed to $INSTALL_BIN"
else
  for unit in "${UNITS[@]}"; do
    if ! systemctl is-enabled "$unit" >/dev/null 2>&1; then
      echo "==> Skipping $unit (not enabled)"
      continue
    fi
    echo "==> Restarting $unit"
    as_root systemctl restart "$unit"
    sleep 2
    systemctl is-active "$unit"
    # grep -o legitimately finds nothing for units without NITPUB_CONFIG
    # set (the primary instance uses the default path) — under pipefail
    # that's a non-zero pipeline exit, which set -e would treat as fatal
    # and abort mid-loop right after the restart. `|| true` makes "no
    # match" fall through to the default below instead of killing the
    # script.
    CONFIG_PATH="$(systemctl show "$unit" -p Environment --value 2>/dev/null | grep -o 'NITPUB_CONFIG=[^ ]*' | cut -d= -f2- || true)"
    CONFIG_PATH="${CONFIG_PATH:-/etc/nitpub/config.toml}"
    PORT="$(awk -F= '/^port =/ {gsub(/ /,"",$2); print $2; exit}' "$CONFIG_PATH" 2>/dev/null || echo 8080)"
    curl -fsS "http://127.0.0.1:${PORT}/healthz" || echo "warning: healthz check failed for $unit on port $PORT"
  done
fi

echo "Done."
