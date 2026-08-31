#!/usr/bin/env bash
# Bakes the nitpub binary onto the Marketplace image at build time.
# Does NOT run `nitpub install` here — that needs a domain, and a
# Marketplace image boots generically with no domain known yet. The
# non-interactive Packer shell provisioner also has no TTY, so the
# interactive install wizard (huh form) can't run during the build
# anyway. First-boot config happens over SSH after the customer
# deploys from this image — see README.md for the documented flow
# and the MOTD this script installs.
set -euo pipefail

REPO="newtosh/nitpub"
INSTALL_DIR="/usr/local/bin"
BIN="${INSTALL_DIR}/nitpub"
API="https://api.github.com/repos/${REPO}/releases/latest"

echo "==> installing prerequisites"
apt-get update -qq
apt-get install -y -qq curl ca-certificates openssl

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) suffix="linux-amd64" ;;
  aarch64|arm64) suffix="linux-arm64" ;;
  *)
    echo "unsupported architecture: ${arch}" >&2
    exit 1
    ;;
esac

echo "==> fetching latest release metadata for ${REPO}"
json="$(curl -fsSL -H 'Accept: application/vnd.github+json' -H 'User-Agent: nitpub-marketplace-build' "$API")"
tag="$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
if [[ -z "$tag" ]]; then
  echo "could not determine latest release tag" >&2
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
got="$(sha256sum "${tmp}/${asset}" | awk '{print $1}')"
if [[ -z "$want" || "$got" != "$want" ]]; then
  echo "checksum mismatch for ${asset}" >&2
  exit 1
fi

install -m 755 "${tmp}/${asset}" "$BIN"
echo "==> installed ${BIN} (${tag})"

echo "==> writing first-boot MOTD"
cat >/etc/motd <<'EOF'

nitpub is installed but not yet configured.

Run the setup wizard now:

    nitpub install

It will ask for your domain, federation actor, and admin password,
then configure systemd + Caddy and create your admin account.
See https://docs.nitpub.com/guide/install for details.

EOF
