#!/usr/bin/env bash
# Point the VPS checkout at GitHub and sync to origin/main.
# Run on the VPS after gh auth (as nitpub user or with credentials in ~/.config/gh).
set -euo pipefail

REPO_DIR="${NITPUB_REPO_DIR:-/var/lib/nitpub/src}"
REPO_SLUG="${NITPUB_GITHUB_REPO:-newtosh/nitpub}"

cd "$REPO_DIR"

if ! command -v gh >/dev/null 2>&1; then
  echo "error: gh not in PATH (run deploy/prepare-user.sh first)" >&2
  exit 1
fi
gh auth status
gh auth setup-git

if git remote get-url origin >/dev/null 2>&1; then
  git remote set-url origin "https://github.com/${REPO_SLUG}.git"
else
  git remote add origin "https://github.com/${REPO_SLUG}.git"
fi

git fetch origin
git checkout -B main origin/main
git branch --set-upstream-to=origin/main main
echo "Synced $REPO_DIR to origin/main ($(git rev-parse --short HEAD))"
