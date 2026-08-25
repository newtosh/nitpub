#!/usr/bin/env bash
# Sync private main's tracked files onto a branch of the public repo and
# open a PR there for review before anything goes world-visible.
#
# ponytail: content sync, not history sync — public keeps its own
# (already-orphaned) history; each run replaces its tracked tree with a
# filtered snapshot of private main, git-native (git archive + tar), no
# rsync/filter-repo dependency.
set -euo pipefail

PRIVATE_REMOTE="${NITPUB_PRIVATE_REMOTE:-origin}"
PUBLIC_REMOTE="${NITPUB_PUBLIC_REMOTE:-public}"
PUBLIC_REPO_SLUG="${NITPUB_PUBLIC_REPO:-newtosh/nitpub}"

EXCLUDES=(
  .cursor
  .github/workflows/deploy.yml
  docs/plans
  docs/tasks
  docs/ideation
)

git fetch "$PRIVATE_REMOTE" main --quiet
git fetch "$PUBLIC_REMOTE" main --quiet

PRIVATE_SHA="$(git rev-parse --short "$PRIVATE_REMOTE/main")"
BRANCH="sync/private-${PRIVATE_SHA}"

WORKTREE="$(mktemp -d)"
trap 'git worktree remove --force "$WORKTREE" 2>/dev/null || true; rm -rf "$WORKTREE"' EXIT

git worktree add --quiet -B "$BRANCH" "$WORKTREE" "$PUBLIC_REMOTE/main"

# Replace the whole tracked tree with private main's, then drop excludes —
# simplest way to keep public's tree an exact filtered mirror without
# manually diffing adds/deletes each run.
git -C "$WORKTREE" rm -rq .
git archive "$PRIVATE_REMOTE/main" | tar -x -C "$WORKTREE"
for path in "${EXCLUDES[@]}"; do
  rm -rf "${WORKTREE:?}/${path}"
done

git -C "$WORKTREE" add -A

if git -C "$WORKTREE" diff --cached --quiet; then
  echo "public already matches private main ($PRIVATE_SHA) — nothing to sync"
  git worktree remove --force "$WORKTREE"
  git branch -D "$BRANCH" 2>/dev/null || true
  exit 0
fi

git -C "$WORKTREE" commit --quiet -m "sync: private main @ ${PRIVATE_SHA}"
git -C "$WORKTREE" push --force-with-lease "$PUBLIC_REMOTE" "HEAD:${BRANCH}"

gh pr create --repo "$PUBLIC_REPO_SLUG" --base main --head "$BRANCH" \
  --title "sync: private main @ ${PRIVATE_SHA}" \
  --body "Automated content sync from the private repo's \`main\` at \`${PRIVATE_SHA}\`. Review before merging — this is the last check before private-only content would go public."
