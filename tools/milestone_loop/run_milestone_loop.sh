#!/usr/bin/env bash
set -euo pipefail

# Unsafe preset for projects that need to run HTTP servers, browsers, integration
# tests, or other commands that do not fit inside Codex workspace-write sandboxing.
#
# This intentionally uses:
#   review:          read-only
#   implementation:  danger-full-access
#   commit message:  read-only
#
# Run this only on a deliberately limited account / separate box / disposable
# worktree. The wrapper makes YOLO explicit rather than hiding it in defaults.

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 MILESTONE_DOCUMENT [extra codex_milestone_loop.py args...]" >&2
  exit 2
fi

MILESTONE_DOC="$1"
shift

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
LOOP_SCRIPT="${SCRIPT_DIR}/codex_milestone_loop.py"

if [[ ! -x "$LOOP_SCRIPT" ]]; then
  echo "Missing or non-executable wrapper: $LOOP_SCRIPT" >&2
  echo "Run: chmod +x $LOOP_SCRIPT" >&2
  exit 2
fi

exec uv run --script "$LOOP_SCRIPT" "$MILESTONE_DOC" \
  --base-branch "${BASE_BRANCH:-main}" \
  --max-iters "${MAX_ITERS:-8}" \
  --review-model "${REVIEW_MODEL:-gpt-5.5}" \
  --review-reasoning "${REVIEW_REASONING:-high}" \
  --implement-model "${IMPLEMENT_MODEL:-gpt-5.4}" \
  --implement-reasoning "${IMPLEMENT_REASONING:-medium}" \
  --commit-model "${COMMIT_MODEL:-gpt-5.4-mini}" \
  --commit-reasoning "${COMMIT_REASONING:-low}" \
  --review-sandbox read-only \
  --implement-sandbox danger-full-access \
  --commit-sandbox read-only \
  --ask-for-approval never \
  --review-subagents \
  "$@"
