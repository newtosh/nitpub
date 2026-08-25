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
PRIVATE_REPO_SLUG="${NITPUB_PRIVATE_REPO:-newtosh/nitpub-dev}"
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

# Build the PR body from what actually changed in private, not just a SHA —
# "build in public" means a reader on the public repo can see the real PR
# titles/descriptions that landed, not just an opaque sync marker.
# --grep's "^" anchors per-line within the full commit body, not just the
# message start — a merge commit's body can echo the squashed subject line
# and outrank it (newer, sorted first), so grep the raw body stream instead
# and take the first match, which is inherently the newest occurrence since
# `git log` emits newest-first.
LAST_SYNC_LINE="$(git log "$PUBLIC_REMOTE/main" --format='%B' 2>/dev/null \
  | grep -m1 -E '^sync: private main @ [0-9a-f]+$' || true)"
LAST_SYNC_SHA="${LAST_SYNC_LINE##* }"

BODY_FILE="$(mktemp)"
{
  echo "Synced from [\`nitpub-dev\`](https://github.com/${PRIVATE_REPO_SLUG}) (private) @ \`${PRIVATE_SHA}\`. Review before merging — this is the last check before private-only content would go public."
  echo
  if [[ -n "$LAST_SYNC_SHA" ]]; then
    PR_NUMS="$(git log "${LAST_SYNC_SHA}..${PRIVATE_REMOTE}/main" --format='%s' 2>/dev/null \
      | grep -oE '\(#[0-9]+\)$' | grep -oE '[0-9]+' | awk '!seen[$0]++' || true)"
    if [[ -n "$PR_NUMS" ]]; then
      echo "## Included PRs"
      echo
      while read -r num; do
        [[ -z "$num" ]] && continue
        PR_JSON="$(gh pr view "$num" --repo "$PRIVATE_REPO_SLUG" --json title,body,number 2>/dev/null || true)"
        [[ -z "$PR_JSON" ]] && continue
        # echo (this shell) rewrites literal "\n" in the JSON string to a
        # real newline byte, which then fails strict JSON parsing — use a
        # here-string, which never interprets backslash escapes.
        TITLE="$(jq -r '.title' <<< "$PR_JSON")"
        PR_BODY="$(jq -r '.body // "(no description)"' <<< "$PR_JSON")"
        echo "### ${TITLE} (nitpub-dev#${num})"
        echo
        echo "$PR_BODY"
        echo
      done <<< "$PR_NUMS"
    fi
  fi
} > "$BODY_FILE"

WORKTREE="$(mktemp -d)"
trap 'git worktree remove --force "$WORKTREE" 2>/dev/null || true; rm -rf "$WORKTREE" "$BODY_FILE"' EXIT

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
  --body-file "$BODY_FILE"
