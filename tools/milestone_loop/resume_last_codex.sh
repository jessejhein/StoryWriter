#!/usr/bin/env bash
set -euo pipefail

# Resume the last persisted Codex exec session after a quota/token stop.
#
# This helper intentionally uses codex's "last session" behavior. On a dedicated
# automation box, that is usually fine. If you use the same account for other
# Codex sessions while the loop is stopped, inspect:
#
#   .git/codex-milestone-loop/state.json
#
# before trusting --last. Hope is not a scheduling primitive.

STAGE="${1:-}"

if [[ -z "$STAGE" ]]; then
  if [[ -f .git/codex-milestone-loop/state.json ]]; then
    STAGE="$(python3 - <<'PY'
import json
from pathlib import Path
state = json.loads(Path(".git/codex-milestone-loop/state.json").read_text())
print(state.get("stage") or "")
PY
)"
  fi
fi

case "$STAGE" in
  review)
    PROMPT="Continue the milestone review exactly where you left off. Do not modify files. Return the final result in the required JSON shape."
    ;;
  implementation)
    PROMPT="Continue implementing the remediation plan exactly where you left off. Inspect the current worktree first. Do not commit. Add/update tests as required. Finish with a summary of files changed, tests run, and remaining risks."
    ;;
  commit-message)
    PROMPT="Continue generating the commit message. Do not modify files. Return only the commit message."
    ;;
  *)
    echo "Usage: $0 review|implementation|commit-message" >&2
    echo "" >&2
    echo "Could not infer a resumable stage from .git/codex-milestone-loop/state.json" >&2
    exit 2
    ;;
esac

echo "Resuming last Codex session for stage: $STAGE" >&2
echo "" >&2

codex exec resume --last "$PROMPT"

echo "" >&2
echo "Resume command finished." >&2
echo "" >&2
echo "Next step:" >&2
echo "  Rerun the milestone loop." >&2
echo "" >&2
echo "If the worktree now has partial changes, add:" >&2
echo "  --allow-dirty-start" >&2
echo "" >&2
echo "If you only wanted to inspect before committing, add:" >&2
echo "  --no-commit" >&2
