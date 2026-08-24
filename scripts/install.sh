#!/usr/bin/env bash
# Bootstrap nitpub from the latest GitHub Release, then hand off to `nitpub install`.
#
#   curl -fsSL https://raw.githubusercontent.com/newtosh/nitpub/main/scripts/install.sh | sudo bash
#
# Extra args are forwarded to `nitpub install` (e.g. --yes --domain=... ).
set -euo pipefail

REPO="${NITPUB_GITHUB_REPO:-newtosh/nitpub}"
INSTALL_DIR="${NITPUB_INSTALL_DIR:-/usr/local/bin}"
BIN="${INSTALL_DIR}/nitpub"
API="https://api.github.com/repos/${REPO}/releases/latest"

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) suffix="linux-amd64" ;;
  aarch64|arm64) suffix="linux-arm64" ;;
  *)
    echo "unsupported architecture: ${arch} (need amd64 or arm64)" >&2
    exit 1
    ;;
esac

if [[ "$(id -u)" -ne 0 ]]; then
  echo "re-run as root (or: curl … | sudo bash)" >&2
  exit 1
fi

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "missing required command: $1" >&2
    exit 1
  }
}
need_cmd curl
need_cmd sha256sum
need_cmd install

echo "==> fetching latest release metadata for ${REPO}"
json="$(curl -fsSL -H 'Accept: application/vnd.github+json' -H 'User-Agent: nitpub-install-sh' "$API")"
tag="$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
if [[ -z "$tag" ]]; then
  echo "could not determine latest release tag (create a GitHub Release first)" >&2
  exit 1
fi

asset="nitpub-${tag}-${suffix}"
base="https://github.com/${REPO}/releases/download/${tag}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "==> downloading ${asset}"
curl -fsSL -o "${tmp}/${asset}" "${base}/${asset}"
curl -fsSL -o "${tmp}/SHA256SUMS" "${base}/SHA256SUMS"

echo "==> verifying checksum"
want="$(awk -v f="$asset" '$2 == f || $2 == "./"f {print $1; exit}' "${tmp}/SHA256SUMS")"
if [[ -z "$want" ]]; then
  echo "SHA256SUMS has no entry for ${asset}" >&2
  exit 1
fi
got="$(sha256sum "${tmp}/${asset}" | awk '{print $1}')"
if [[ "$got" != "$want" ]]; then
  echo "checksum mismatch for ${asset}: got ${got} want ${want}" >&2
  exit 1
fi

install -m 755 "${tmp}/${asset}" "$BIN"
echo "==> installed ${BIN} (${tag})"
echo "==> starting nitpub install"
exec "$BIN" install "$@"
