#!/usr/bin/env bash
# Copy example site content into the configured data_dir (local dev).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EXAMPLE="${ROOT}/scripts/example-site"
DATA_DIR="${NITPUB_DATA_DIR:-${ROOT}/data}"

if [[ ! -d "${EXAMPLE}" ]]; then
  echo "missing ${EXAMPLE}" >&2
  exit 1
fi

mkdir -p "${DATA_DIR}/site"
cp -a "${EXAMPLE}/." "${DATA_DIR}/site/"
echo "seeded site content into ${DATA_DIR}/site"
