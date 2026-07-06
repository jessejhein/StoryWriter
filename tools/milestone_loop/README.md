# Codex Milestone Loop

Drop this directory into a repo as:

```text
tools/milestone_loop/
  codex_milestone_loop.py
  run_milestone_loop.sh
  resume_last_codex.sh
  README.md
```

The Python script is `uv run --script` ready via a PEP 723 header.

## Normal safer run

```bash
uv run --script tools/milestone_loop/codex_milestone_loop.py docs/milestone_task_prompt.md
```

Default sandboxes:

```text
review:          read-only
implementation:  workspace-write
commit message:  read-only
```

## Unsafe HTTP/integration-test preset

```bash
tools/milestone_loop/run_milestone_loop.sh docs/milestone_task_prompt.md
```

The Bash helper intentionally uses:

```text
review:          read-only
implementation:  danger-full-access
commit message:  read-only
```

This is for projects where the implementation agent must run HTTP servers,
browser tests, integration tests, or commands outside the workspace sandbox.

Use it only on a deliberately limited account / separate box / disposable
worktree. The point is not that YOLO is safe. The point is that YOLO is explicit.

## Loop count

The Python script defaults to:

```text
--max-iters 6
```

The unsafe Bash preset defaults to:

```text
MAX_ITERS=8
```

Override either way:

```bash
MAX_ITERS=12 tools/milestone_loop/run_milestone_loop.sh docs/milestone_task_prompt.md
```

```bash
tools/milestone_loop/run_milestone_loop.sh docs/milestone_task_prompt.md --max-iters 12
```

## Failure / token exhaustion behavior

The runner does **not** use `--ephemeral` by default. Codex sessions are persisted
so they can be resumed.

Every run writes a state file:

```text
.git/codex-milestone-loop/state.json
```

On failure it also writes:

```text
.git/codex-milestone-loop/latest_failure.md
```

The terminal output is intentionally loud and includes:

```text
Iteration
Stage
Reason
State file path
Failure report path
Suggested resume command, when applicable
```

Example stopped stage:

```text
Iteration : 3
Stage     : implementation
Reason    : Codex exited with status 1
Suggested resume command:
  tools/milestone_loop/resume_last_codex.sh implementation
```

To resume the last Codex session after a quota/token-regeneration stop:

```bash
tools/milestone_loop/resume_last_codex.sh implementation
```

or, if the state file has the stage and no other Codex sessions have happened:

```bash
tools/milestone_loop/resume_last_codex.sh
```

After resume finishes, rerun the milestone loop. If the worktree has partial
changes, use:

```bash
tools/milestone_loop/run_milestone_loop.sh docs/milestone_task_prompt.md --allow-dirty-start
```

For a cautious recovery run:

```bash
tools/milestone_loop/run_milestone_loop.sh docs/milestone_task_prompt.md \
  --allow-dirty-start \
  --no-commit
```

## Common overrides

```bash
IMPLEMENT_MODEL=gpt-5.5 tools/milestone_loop/run_milestone_loop.sh docs/milestone_task_prompt.md
```

```bash
tools/milestone_loop/run_milestone_loop.sh docs/milestone_task_prompt.md --no-commit
```

```bash
tools/milestone_loop/run_milestone_loop.sh docs/milestone_task_prompt.md \
  --remediation-file .plans/this_milestone_remediation.md \
  --prior-plan-glob ".plans/this_milestone*.md"
```

## What the loop does

1. High-reasoning reviewer checks the branch against the milestone document.
2. Reviewer evaluates:
   - requirements completeness
   - automated tests
   - API success and failure behavior
   - design, style, SOLID, maintainability, extensibility
   - readiness to move to the next milestone
3. Reviewer emits structured JSON.
4. Wrapper writes a human-readable remediation file under `.plans/`.
5. Implementation model receives the milestone, JSON plan, and remediation file.
6. Implementation model edits files and runs verification commands when possible.
7. Mini model writes a commit message.
8. Wrapper runs `git add -A` and `git commit`.
9. Loop repeats until the reviewer says the milestone is complete.

## Generated files

Temporary run artifacts go under:

```text
.git/codex-milestone-loop/
```

The remediation markdown goes under `.plans/` by default and is committed with
the implementation changes unless you change that behavior.

## Notes

- The implementer does not inherit the reviewer's full session transcript.
- The reviewer is asked to write a compressed handoff with evidence, design
  concerns, test requirements, and acceptance checks.
- The implementer is told to verify the plan against the actual code and
  milestone doc rather than obeying blindly.
- Use `--ephemeral-sessions` only when you explicitly do not care about resume.
