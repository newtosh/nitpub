#!/usr/bin/env bash
# User-space VPS prep — no root, no /etc, no systemd.
# Run as the nitpub service user (or any unprivileged user with a writable home).
#
#   bash deploy/prepare-user.sh
#
# Optional env:
#   NITPUB_REPO_DIR     — git checkout (default: ~/nitpub-src)
#   NITPUB_GITHUB_REPO  — default newtosh/nitpub
#   GO_VERSION          — default 1.26.5
#   NODE_VERSION        — default 22.17.0
set -euo pipefail

NITPUB_GITHUB_REPO="${NITPUB_GITHUB_REPO:-newtosh/nitpub}"
REPO_DIR="${NITPUB_REPO_DIR:-$HOME/nitpub-src}"
LOCAL_BIN="${HOME}/.local/bin"
LOCAL_ROOT="${HOME}/.local"
GO_VERSION="${GO_VERSION:-1.26.5}"
NODE_VERSION="${NODE_VERSION:-22.17.0}"
PATH="${LOCAL_BIN}:${LOCAL_ROOT}/go/bin:${PATH}"
export PATH

mkdir -p "$LOCAL_BIN" "$REPO_DIR"

log() { printf '==> %s\n' "$*"; }

install_gh() {
  if command -v gh >/dev/null 2>&1; then
    log "gh already installed: $(gh --version | head -1)"
    return
  fi
  log "Installing GitHub CLI to $LOCAL_BIN"
  arch="$(uname -m)"
  case "$arch" in
    x86_64) gh_arch=amd64 ;;
    aarch64|arm64) gh_arch=arm64 ;;
    *) echo "unsupported arch: $arch" >&2; exit 1 ;;
  esac
  tag="$(curl -fsSL https://api.github.com/repos/cli/cli/releases/latest | grep tag_name | cut -d\" -f4)"
  tmp="$(mktemp -d)"
  curl -fsSL "https://github.com/cli/cli/releases/download/${tag}/gh_${tag#v}_linux_${gh_arch}.tar.gz" \
    | tar -xz -C "$tmp"
  install -m 755 "$tmp"/gh_*/bin/gh "$LOCAL_BIN/gh"
  rm -rf "$tmp"
}

install_go() {
  if [[ -x "$LOCAL_ROOT/go/bin/go" ]]; then
    log "Go already installed: $($LOCAL_ROOT/go/bin/go version)"
    return
  fi
  log "Installing Go $GO_VERSION to $LOCAL_ROOT/go"
  arch="$(uname -m)"
  case "$arch" in
    x86_64) go_arch=amd64 ;;
    aarch64|arm64) go_arch=arm64 ;;
    *) echo "unsupported arch: $arch" >&2; exit 1 ;;
  esac
  tmp="$(mktemp)"
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" -o "$tmp"
  rm -rf "$LOCAL_ROOT/go"
  tar -C "$LOCAL_ROOT" -xzf "$tmp"
  rm -f "$tmp"
}

install_node() {
  if [[ -x "$LOCAL_ROOT/node/bin/node" ]]; then
    log "Node already installed: $($LOCAL_ROOT/node/bin/node -v)"
    return
  fi
  log "Installing Node $NODE_VERSION to $LOCAL_ROOT/node"
  arch="$(uname -m)"
  case "$arch" in
    x86_64) node_arch=x64 ;;
    aarch64|arm64) node_arch=arm64 ;;
    *) echo "unsupported arch: $arch" >&2; exit 1 ;;
  esac
  tmp="$(mktemp)"
  curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node_arch}.tar.xz" -o "$tmp"
  rm -rf "$LOCAL_ROOT/node"
  tar -C "$LOCAL_ROOT" -xJf "$tmp"
  mv "$LOCAL_ROOT/node-v${NODE_VERSION}-linux-${node_arch}" "$LOCAL_ROOT/node"
  rm -f "$tmp"
  ln -sf "$LOCAL_ROOT/node/bin/node" "$LOCAL_BIN/node"
  ln -sf "$LOCAL_ROOT/node/bin/npm" "$LOCAL_BIN/npm"
  ln -sf "$LOCAL_ROOT/node/bin/npx" "$LOCAL_BIN/npx"
}

ensure_path_snippet() {
  local rc="$HOME/.profile"
  local line='export PATH="$HOME/.local/bin:$HOME/.local/go/bin:$PATH"'
  if [[ -f "$rc" ]] && ! grep -qF '.local/go/bin' "$rc"; then
    printf '\n# nitpub user tools\n%s\n' "$line" >>"$rc"
  fi
}

ensure_repo() {
  if [[ -d "$REPO_DIR/.git" ]]; then
    log "Repo checkout exists at $REPO_DIR"
    return
  fi
  if [[ -f "$REPO_DIR/go.mod" ]]; then
    log "Initializing git repo at $REPO_DIR (rsync checkout)"
    git -C "$REPO_DIR" init -b main >/dev/null
    return
  fi
  if [[ -n "${NITPUB_GITHUB_REPO:-}" ]] && command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
    log "Cloning github.com/$NITPUB_GITHUB_REPO"
    gh repo clone "$NITPUB_GITHUB_REPO" "$REPO_DIR"
    return
  fi
  log "No git repo at $REPO_DIR yet"
  log "  — rsync from laptop: rsync -av --exclude node_modules --exclude data ./ nitpub:$REPO_DIR/"
  log "  — or set NITPUB_GITHUB_REPO and run: gh auth login"
  mkdir -p "$REPO_DIR"
}

write_user_config() {
  local cfg_dir="$HOME/.config/nitpub"
  local cfg_file="$cfg_dir/config.toml"
  mkdir -p "$cfg_dir"
  local secret_line=""
  if [[ -n "${NITPUB_SECRET:-}" ]]; then
    secret_line="secret = \"${NITPUB_SECRET}\""
  fi
  if [[ -f "$cfg_file" ]] && [[ -z "${NITPUB_SECRET:-}" ]]; then
    log "User config exists: $cfg_file"
    return
  fi
  log "Writing user config $cfg_file"
  local domain="${NITPUB_DOMAIN:?set NITPUB_DOMAIN e.g. blog.example.com}"
  local actor="${NITPUB_ACTOR:-user}"
  cat >"$cfg_file" <<EOF
domain = "$domain"
port = 8080
data_dir = "/var/lib/nitpub"
actor = "$actor"
${secret_line}
http = false
system_user = "nitpub"
EOF
  chmod 600 "$cfg_file"
}

install_gh
install_go
install_node
ensure_path_snippet
ensure_repo
write_user_config

log "User-space prep complete"
log "PATH includes: $LOCAL_BIN"
command -v gh >/dev/null && gh auth status 2>&1 || log "Run: gh auth login  (for private repo clone/pull)"
command -v gh >/dev/null && gh auth status >/dev/null 2>&1 && gh auth setup-git 2>/dev/null || true
if [[ -f "$REPO_DIR/deploy/update.sh" ]]; then
  log "After root installs systemd + /etc/nitpub/config.toml:"
  log "  NITPUB_REPO_DIR=$REPO_DIR bash $REPO_DIR/deploy/update.sh"
fi
