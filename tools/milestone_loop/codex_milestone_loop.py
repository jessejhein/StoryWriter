#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Codex milestone loop.

Automates this workflow:

1. High-reasoning reviewer checks whether the current branch satisfies a milestone.
2. If incomplete, reviewer emits structured remediation JSON.
3. This script writes a human-readable remediation plan file.
4. Smaller implementation model applies the plan.
5. Mini model writes a commit message.
6. This script runs git commit.
7. Repeat until complete or max iterations is reached.

Failure handling is deliberately loud:
- Writes .git/codex-milestone-loop/state.json
- Writes .git/codex-milestone-loop/latest_failure.md on failure
- Prints the iteration, stage, reason, and suggested resume command
- Does NOT use --ephemeral by default, so Codex sessions can be resumed
- Supports a read-only --status mode that reports the latest recorded loop
  state, per-iteration artifacts, and the last output preview.

Designed for local use in a dedicated repo/worktree.

Default safety profile:
- review stage: read-only
- implementation stage: workspace-write
- commit-message stage: read-only

For intentionally unsafe HTTP/integration-test runs, use run_milestone_loop.sh,
which sets implementation sandbox to danger-full-access explicitly.
"""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Literal


SandboxMode = Literal["read-only", "workspace-write", "danger-full-access"]


REVIEW_SCHEMA: dict[str, Any] = {
    "type": "object",
    "properties": {
        "complete": {"type": "boolean"},
        "safe_to_roll_into_next_milestone": {"type": "boolean"},
        "summary": {"type": "string"},
        "remaining_work": {
            "type": "array",
            "items": {"type": "string"},
        },
        "implementation_plan": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "task": {"type": "string"},
                    "requirement_refs": {
                        "type": "array",
                        "items": {"type": "string"},
                    },
                    "files": {
                        "type": "array",
                        "items": {"type": "string"},
                    },
                    "evidence": {
                        "type": "array",
                        "items": {"type": "string"},
                    },
                    "instructions": {"type": "string"},
                    "design_constraints": {
                        "type": "array",
                        "items": {"type": "string"},
                    },
                    "test_requirements": {
                        "type": "array",
                        "items": {"type": "string"},
                    },
                    "acceptance_checks": {
                        "type": "array",
                        "items": {"type": "string"},
                    },
                },
                "required": [
                    "task",
                    "requirement_refs",
                    "files",
                    "evidence",
                    "instructions",
                    "design_constraints",
                    "test_requirements",
                    "acceptance_checks",
                ],
                "additionalProperties": False,
            },
        },
        "verification_commands": {
            "type": "array",
            "items": {"type": "string"},
        },
        "risk_notes": {
            "type": "array",
            "items": {"type": "string"},
        },
        "handoff_notes": {
            "type": "array",
            "items": {"type": "string"},
        },
    },
    "required": [
        "complete",
        "safe_to_roll_into_next_milestone",
        "summary",
        "remaining_work",
        "implementation_plan",
        "verification_commands",
        "risk_notes",
        "handoff_notes",
    ],
    "additionalProperties": False,
}


REVIEW_PROMPT = """\
You are the milestone reviewer.

Authoritative milestone requirements:
{milestone}

Review scope:
Review the current branch against the authoritative requirements for this milestone.

Also review any prior remediation work if present, especially:
- {remediation_file}
- {prior_plan_glob}

Important:
Prior remediation files may be incomplete, stale, or wrong. The milestone requirements are the source of truth.

Your job:
Determine whether this branch is ready to move on from this milestone.

Review dimensions:

1. Requirements completeness
   - Are all milestone requirements implemented?
   - Are edge cases covered?
   - Are API contracts honored?

2. Tests
   - Are all important behaviors covered by automated tests?
   - Are API success paths tested?
   - Are bad API calls, invalid inputs, and expected failure responses tested?
   - Are previous defects now covered by tests so the same bug cannot silently return?
   - Treat manual-test-only coverage as a gap unless automation is genuinely impossible.

3. Design and maintainability
   - Is the implementation simple, cohesive, and understandable?
   - Does it follow SOLID principles where applicable?
   - Are responsibilities separated cleanly?
   - Is the code extensible without creating a maintenance trap?
   - Are abstractions helpful, or are they ceremony?
   - Is there duplication that will become painful?
   - Are names clear and consistent with the project style?
   - Is the implementation technically correct but likely to make future milestones harder?

4. Style and project conventions
   - Does the code follow existing project patterns?
   - Does it match AGENTS.md and local coding rules?
   - Are docs, comments, and public APIs clear?
   - Are errors and logs useful without being noisy?

5. Milestone transition
   - Is this milestone complete?
   - If incomplete, must the remaining work be fixed before starting the next milestone?
   - Or is it safe and sensible to roll the fixes into the next milestone?

Do not modify files.

{review_subagents}

Write the remediation plan as a handoff to an implementation agent that will not have your full conversation context.

For every finding, include enough evidence and reasoning for the implementer to act safely:
- what requirement is not satisfied
- what code or test evidence supports the finding
- what design or maintainability problem exists
- what change should be made
- what tests should prove it
- how to verify completion

Produce a remediation plan specific enough for a smaller implementation model to execute without interpretation.

For each required fix include:
- task name
- requirement or design issue being addressed
- files likely involved
- exact behavior to implement
- tests to add or strengthen
- bad-path/API tests required
- design/style constraints
- verification commands
- acceptance criteria

If this milestone is complete, say so clearly.

If this milestone is incomplete but safe to roll into the next milestone, say so clearly, but still provide the full remediation plan.

Return only JSON matching the requested schema.
"""


REVIEW_SUBAGENTS_TEXT = """\
If useful, spawn read-only subagents for parallel review:
- one focused on requirements and API behavior
- one focused on tests and failure paths
- one focused on design, maintainability, SOLID, and extensibility

Wait for all subagents before producing the final result.
"""


IMPLEMENT_PROMPT = """\
You are the implementation agent.

Implement the remediation plan below.

Authoritative milestone requirements:
{milestone}

Remediation file:
{remediation_file}

Plan JSON:
{plan_json}

Authoritative rule:
The plan is your task list, but the milestone requirements remain the source of truth.

Rules:
- Read AGENTS.md and local project conventions first.
- Read the milestone requirements and remediation file before editing.
- Make the smallest maintainable changes that satisfy the plan.
- Do not broaden scope beyond this milestone.
- Do not commit.
- Add or strengthen tests for every behavior changed.
- Add tests for API failure paths, bad requests, invalid inputs, and expected error responses where required.
- Replace manual-test-only coverage with automated tests whenever practical.
- If test automation requires a small architecture change, make that change.
- Preserve or improve design quality.
- Avoid fixes that are technically correct but make future maintenance or extension harder.
- Follow SOLID principles where they apply naturally.
- Prefer clear, boring code over clever code.
- Do not create abstractions unless they reduce real duplication or clarify responsibilities.
- Run the relevant verification commands.

The review plan is a handoff, not gospel.

Use it as your task list, but verify against:
- the milestone document
- the actual code
- existing tests
- AGENTS.md and project conventions

If the plan seems inconsistent with the code or requirements:
- implement the safest clearly supported fix
- record the inconsistency in IMPLEMENTATION_NOTES.md

If a requested item is impossible:
- Do not ask for clarification.
- Write a BLOCKED section in IMPLEMENTATION_NOTES.md explaining:
  - what could not be done
  - why it could not be done
  - what evidence you found
  - what a human should decide next
- Continue with any remaining unblocked work.

Before finishing, summarize:
- files changed
- tests added or updated
- design improvements made
- verification commands run
- remaining risks or blocked items
"""


COMMIT_PROMPT = """\
You are the commit message agent.

Review the current uncommitted diff and produce a concise commit message.

Rules:
- Do not modify files.
- Do not run git commit.
- Return only the commit message.
- Use one subject line and an optional short body.
- Prefer conventional commit style when it fits.
- Mention tests only if tests were actually added or changed.
"""


RESUME_COMMAND_TEMPLATE = "tools/milestone_loop/resume_last_codex.sh {stage}"


@dataclass(frozen=True)
class CodexConfig:
    review_model: str
    review_reasoning: str
    implement_model: str
    implement_reasoning: str
    commit_model: str
    commit_reasoning: str
    review_sandbox: SandboxMode
    implement_sandbox: SandboxMode
    commit_sandbox: SandboxMode
    ask_for_approval: str
    allow_network: bool
    ephemeral_sessions: bool


@dataclass(frozen=True)
class Paths:
    repo: Path
    run_dir: Path
    schema_file: Path
    state_file: Path
    latest_failure_file: Path


class LoopError(RuntimeError):
    pass


class StageError(LoopError):
    def __init__(
        self,
        *,
        iteration: int,
        stage: str,
        reason: str,
        detail: str,
        resume_command: str | None = None,
    ) -> None:
        super().__init__(detail)
        self.iteration = iteration
        self.stage = stage
        self.reason = reason
        self.detail = detail
        self.resume_command = resume_command


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


def run(
    args: list[str],
    *,
    cwd: Path,
    input_text: str | None = None,
    capture: bool = False,
    check: bool = True,
) -> subprocess.CompletedProcess[str]:
    kwargs: dict[str, Any] = {
        "cwd": cwd,
        "text": True,
        "check": check,
    }

    if input_text is not None:
        kwargs["input"] = input_text

    if capture:
        kwargs["stdout"] = subprocess.PIPE
        kwargs["stderr"] = subprocess.PIPE

    try:
        return subprocess.run(args, **kwargs)
    except subprocess.CalledProcessError as exc:
        cmd = " ".join(args)
        stderr = exc.stderr or ""
        stdout = exc.stdout or ""
        raise LoopError(
            f"Command failed: {cmd}\n\nSTDOUT:\n{stdout}\n\nSTDERR:\n{stderr}"
        ) from exc


def git(repo: Path, *args: str, capture: bool = True) -> str:
    result = run(["git", *args], cwd=repo, capture=capture)
    return result.stdout.strip() if capture and result.stdout else ""


def require_tool(name: str) -> None:
    if shutil.which(name) is None:
        raise LoopError(f"Required tool not found on PATH: {name}")


def resolve_repo(path: Path) -> Path:
    result = run(
        ["git", "-C", str(path), "rev-parse", "--show-toplevel"],
        cwd=Path.cwd(),
        capture=True,
    )
    return Path(result.stdout.strip()).resolve()


def git_status(repo: Path) -> str:
    return git(repo, "status", "--porcelain")


def current_branch(repo: Path) -> str:
    return git(repo, "branch", "--show-current")


def git_path(repo: Path, name: str) -> Path:
    value = git(repo, "rev-parse", "--git-path", name)
    return (repo / value).resolve()


def prepare_paths(repo: Path, run_dir_arg: Path | None) -> Paths:
    return resolve_paths(repo, run_dir_arg, create=True)


def resolve_paths(repo: Path, run_dir_arg: Path | None, *, create: bool) -> Paths:
    if run_dir_arg is None:
        run_dir = git_path(repo, "codex-milestone-loop")
    else:
        run_dir = run_dir_arg.resolve()

    if create:
        run_dir.mkdir(parents=True, exist_ok=True)
        schema_file = run_dir / "milestone-review.schema.json"
        schema_file.write_text(json.dumps(REVIEW_SCHEMA, indent=2), encoding="utf-8")
    else:
        schema_file = run_dir / "milestone-review.schema.json"

    return Paths(
        repo=repo,
        run_dir=run_dir,
        schema_file=schema_file,
        state_file=run_dir / "state.json",
        latest_failure_file=run_dir / "latest_failure.md",
    )


def rel_display(path: Path, repo: Path) -> str:
    try:
        return str(path.relative_to(repo))
    except ValueError:
        return str(path)


def write_state(
    paths: Paths,
    *,
    status: str,
    iteration: int,
    stage: str,
    reason: str,
    milestone_file: Path | None = None,
    remediation_file: Path | None = None,
    review_file: Path | None = None,
    output_file: Path | None = None,
    resume_command: str | None = None,
    detail: str | None = None,
) -> None:
    branch = ""
    try:
        branch = current_branch(paths.repo)
    except Exception:
        branch = ""

    state = {
        "timestamp_utc": utc_now_iso(),
        "status": status,
        "iteration": iteration,
        "stage": stage,
        "reason": reason,
        "repo": str(paths.repo),
        "branch": branch,
        "milestone_file": rel_display(milestone_file, paths.repo) if milestone_file else None,
        "remediation_file": rel_display(remediation_file, paths.repo) if remediation_file else None,
        "review_file": rel_display(review_file, paths.repo) if review_file else None,
        "output_file": rel_display(output_file, paths.repo) if output_file else None,
        "resume_command": resume_command,
        "detail": detail,
    }
    paths.state_file.write_text(json.dumps(state, indent=2) + "\n", encoding="utf-8")


def write_failure_report(paths: Paths, error: StageError) -> None:
    lines = [
        "# Codex Milestone Loop Stopped\n",
        "\n",
        f"- **When:** {utc_now_iso()}\n",
        f"- **Iteration:** {error.iteration}\n",
        f"- **Stage:** `{error.stage}`\n",
        f"- **Reason:** {error.reason}\n",
        f"- **State file:** `{rel_display(paths.state_file, paths.repo)}`\n",
        "\n",
        "## What happened\n",
        "\n",
        error.detail.strip() + "\n",
        "\n",
    ]

    if error.resume_command:
        lines.extend(
            [
                "## Suggested resume command\n",
                "\n",
                "```bash\n",
                f"{error.resume_command}\n",
                "```\n",
                "\n",
                "After the resumed Codex command finishes, rerun the milestone loop. "
                "If the worktree has partial changes, use `--allow-dirty-start`.\n",
                "\n",
            ]
        )
    else:
        lines.extend(
            [
                "## Resume\n",
                "\n",
                "This stop does not have an automatic Codex-session resume command. "
                "Inspect the reason above and rerun the milestone loop after fixing it.\n",
                "\n",
            ]
        )

    lines.extend(
        [
            "## Current git status\n",
            "\n",
            "```text\n",
            git_status(paths.repo) or "(clean)",
            "\n```\n",
        ]
    )

    paths.latest_failure_file.write_text("".join(lines), encoding="utf-8")


def load_loop_state(paths: Paths) -> dict[str, Any] | None:
    if not paths.state_file.exists():
        return None

    try:
        return json.loads(paths.state_file.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise LoopError(f"Could not parse loop state file {paths.state_file}: {exc}") from exc


def collect_iteration_artifacts(run_dir: Path) -> dict[int, list[str]]:
    artifacts: dict[int, set[str]] = {}
    patterns = [
        (re.compile(r"^review-(\d+)\.json$"), "review"),
        (re.compile(r"^implement-(\d+)\.md$"), "implementation"),
        (re.compile(r"^commit-(\d+)\.raw\.txt$"), "commit-message-raw"),
        (re.compile(r"^commit-(\d+)\.txt$"), "commit-message"),
    ]

    if not run_dir.exists():
        return {}

    for path in run_dir.iterdir():
        if not path.is_file():
            continue
        name = path.name
        for pattern, label in patterns:
            match = pattern.match(name)
            if match:
                iteration = int(match.group(1))
                artifacts.setdefault(iteration, set()).add(label)
                break

    order = {
        "review": 0,
        "implementation": 1,
        "commit-message-raw": 2,
        "commit-message": 3,
    }
    return {
        iteration: sorted(labels, key=lambda label: order.get(label, 99))
        for iteration, labels in sorted(artifacts.items())
    }


def preview_output(path: Path, *, max_chars: int = 2000) -> str:
    if not path.exists():
        return "(missing)"

    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        return f"(unreadable: {exc})"

    if len(text) <= max_chars:
        return text.rstrip()

    return text[:max_chars].rstrip() + "\n...[truncated]..."


def build_status_report(paths: Paths) -> str:
    state = load_loop_state(paths)
    branch = ""
    git_state = "(unavailable)"
    try:
        branch = current_branch(paths.repo)
    except Exception:
        branch = "(unavailable)"
    try:
        git_state = git_status(paths.repo) or "(clean)"
    except Exception:
        git_state = "(unavailable)"

    lines: list[str] = []
    lines.append("Codex Milestone Loop Status\n")
    lines.append("\n")
    lines.append(f"Repository : {paths.repo}\n")
    lines.append(f"Branch     : {branch}\n")
    lines.append(f"Run dir    : {paths.run_dir}\n")
    lines.append(f"State file : {rel_display(paths.state_file, paths.repo)}\n")
    lines.append(f"Git status : {git_state}\n")

    if state is None:
        lines.append("\nNo loop state file was found.\n")
        lines.append("Next step: run the milestone loop once to create state.\n")
        return "".join(lines)

    lines.append("\nCurrent state\n")
    lines.append(f"  Status   : {state.get('status', '(unknown)')}\n")
    lines.append(f"  Iteration: {state.get('iteration', '(unknown)')}\n")
    lines.append(f"  Stage    : {state.get('stage', '(unknown)')}\n")
    lines.append(f"  Reason   : {state.get('reason', '(unknown)')}\n")
    lines.append(f"  Updated  : {state.get('timestamp_utc', '(unknown)')}\n")
    if state.get("branch"):
        lines.append(f"  Recorded branch: {state.get('branch')}\n")
    if state.get("detail"):
        lines.append(f"  Detail   : {state.get('detail')}\n")
    if state.get("milestone_file"):
        lines.append(f"  Milestone: {state.get('milestone_file')}\n")
    if state.get("remediation_file"):
        lines.append(f"  Remediation: {state.get('remediation_file')}\n")
    if state.get("review_file"):
        lines.append(f"  Review file: {state.get('review_file')}\n")
    if state.get("output_file"):
        lines.append(f"  Last output file: {state.get('output_file')}\n")
    if state.get("resume_command"):
        lines.append(f"  Resume command: {state.get('resume_command')}\n")

    artifacts = collect_iteration_artifacts(paths.run_dir)
    if artifacts:
        lines.append("\nPerformed loops\n")
        for iteration, labels in artifacts.items():
            lines.append(f"  Iteration {iteration}: {', '.join(labels)}\n")

    output_file = state.get("output_file")
    if isinstance(output_file, str) and output_file:
        output_path = (paths.repo / output_file).resolve() if not Path(output_file).is_absolute() else Path(output_file)
        lines.append("\nLast output\n")
        lines.append(f"  Path: {output_file}\n")
        lines.append("  Preview:\n")
        preview = preview_output(output_path)
        for line in preview.splitlines() or [""]:
            lines.append(f"    {line}\n")

    lines.append("\nNext steps\n")
    if state.get("resume_command"):
        lines.append(f"  Run: {state.get('resume_command')}\n")
        lines.append(
            "  After it finishes, rerun the milestone loop. Use --allow-dirty-start if partial changes remain.\n"
        )
    elif state.get("status") == "complete":
        lines.append("  The loop reported completion. Move to the next milestone prompt if you are continuing.\n")
    else:
        lines.append("  Inspect the recorded state and rerun the loop after addressing the stop reason.\n")

    return "".join(lines)


def print_failure_banner(paths: Paths, error: StageError) -> None:
    border = "=" * 78
    print("\n" + border, file=sys.stderr)
    print("CODEX MILESTONE LOOP STOPPED", file=sys.stderr)
    print(border, file=sys.stderr)
    print(f"Iteration : {error.iteration}", file=sys.stderr)
    print(f"Stage     : {error.stage}", file=sys.stderr)
    print(f"Reason    : {error.reason}", file=sys.stderr)
    print(f"State     : {rel_display(paths.state_file, paths.repo)}", file=sys.stderr)
    print(f"Failure   : {rel_display(paths.latest_failure_file, paths.repo)}", file=sys.stderr)
    if error.resume_command:
        print("\nSuggested resume command:", file=sys.stderr)
        print(f"  {error.resume_command}", file=sys.stderr)
        print(
            "\nAfter that finishes, rerun the milestone loop. "
            "Use --allow-dirty-start if partial changes remain.",
            file=sys.stderr,
        )
    print(border + "\n", file=sys.stderr)


def codex_exec(
    *,
    paths: Paths,
    config: CodexConfig,
    prompt: str,
    model: str,
    reasoning: str,
    sandbox: SandboxMode,
    output_file: Path,
    iteration: int,
    stage: str,
    milestone_file: Path,
    remediation_file: Path,
    review_file: Path | None = None,
    output_schema: Path | None = None,
) -> None:
    resume_command = None if config.ephemeral_sessions else RESUME_COMMAND_TEMPLATE.format(stage=stage)

    write_state(
        paths,
        status="running",
        iteration=iteration,
        stage=stage,
        reason="codex stage started",
        milestone_file=milestone_file,
        remediation_file=remediation_file,
        review_file=review_file,
        output_file=output_file,
        resume_command=resume_command,
    )

    cmd = build_codex_exec_command(
        config=config,
        model=model,
        reasoning=reasoning,
        sandbox=sandbox,
        output_file=output_file,
        output_schema=output_schema,
    )

    print(f"\n== Iteration {iteration}: {stage} ==", flush=True)
    print(f"$ {' '.join(cmd[:-1])} < prompt", flush=True)

    proc = subprocess.run(
        cmd,
        cwd=paths.repo,
        input=prompt,
        text=True,
    )

    if proc.returncode != 0:
        detail = (
            f"Codex exited with status {proc.returncode} during iteration "
            f"{iteration}, stage '{stage}'.\n\n"
            "If this was a usage/quota/token-regeneration stop, use the resume command.\n"
            "If this was context exhaustion or a real tool/test failure, inspect the terminal "
            "output above and the current git status before continuing."
        )
        write_state(
            paths,
            status="failed",
            iteration=iteration,
            stage=stage,
            reason=f"codex exited with status {proc.returncode}",
            milestone_file=milestone_file,
            remediation_file=remediation_file,
            review_file=review_file,
            output_file=output_file,
            resume_command=resume_command,
            detail=detail,
        )
        raise StageError(
            iteration=iteration,
            stage=stage,
            reason=f"Codex exited with status {proc.returncode}",
            detail=detail,
            resume_command=resume_command,
        )

    write_state(
        paths,
        status="stage-complete",
        iteration=iteration,
        stage=stage,
        reason="codex stage completed",
        milestone_file=milestone_file,
        remediation_file=remediation_file,
        review_file=review_file,
        output_file=output_file,
        resume_command=resume_command,
    )


def build_codex_exec_command(
    *,
    config: CodexConfig,
    model: str,
    reasoning: str,
    sandbox: SandboxMode,
    output_file: Path,
    output_schema: Path | None = None,
) -> list[str]:
    cmd = [
        "codex",
        "--ask-for-approval",
        config.ask_for_approval,
        "exec",
        "--color",
        "never",
        "--sandbox",
        sandbox,
        "--model",
        model,
        "-c",
        f'model_reasoning_effort="{reasoning}"',
        "--output-last-message",
        str(output_file),
        "-",
    ]

    if config.ephemeral_sessions:
        cmd.insert(4, "--ephemeral")

    if output_schema is not None:
        insert_at = len(cmd) - 1
        cmd[insert_at:insert_at] = ["--output-schema", str(output_schema)]

    if config.allow_network:
        insert_at = cmd.index("--output-last-message")
        cmd[insert_at:insert_at] = [
            "-c",
            "sandbox_workspace_write.network_access=true",
        ]

    return cmd


def parse_json_file(path: Path, *, paths: Paths, iteration: int) -> dict[str, Any]:
    stage = "review-parse"
    text = path.read_text(encoding="utf-8").strip()

    try:
        data = json.loads(text)
    except json.JSONDecodeError:
        match = re.search(r"\{.*\}", text, re.DOTALL)
        if not match:
            raise StageError(
                iteration=iteration,
                stage=stage,
                reason="could not parse reviewer JSON",
                detail=f"Could not parse JSON from {path}:\n\n{text}",
                resume_command=None,
            )
        try:
            data = json.loads(match.group(0))
        except json.JSONDecodeError as exc:
            raise StageError(
                iteration=iteration,
                stage=stage,
                reason="could not parse reviewer JSON",
                detail=f"Could not parse JSON from {path}: {exc}\n\n{text}",
                resume_command=None,
            ) from exc

    if not isinstance(data, dict):
        raise StageError(
            iteration=iteration,
            stage=stage,
            reason="reviewer output was not a JSON object",
            detail=f"Expected JSON object in {path}, got {type(data).__name__}",
            resume_command=None,
        )

    return data


def markdown_list(items: list[str], *, indent: str = "") -> str:
    if not items:
        return f"{indent}- None noted.\n"
    return "".join(f"{indent}- {item}\n" for item in items)


def write_remediation_markdown(path: Path, review: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)

    lines: list[str] = []
    lines.append("# Milestone Remediation Plan\n\n")
    lines.append("Generated by `tools/milestone_loop/codex_milestone_loop.py`.\n\n")
    lines.append("## Status\n\n")
    lines.append(f"- Complete: `{review.get('complete')}`\n")
    lines.append(
        f"- Safe to roll into next milestone: `{review.get('safe_to_roll_into_next_milestone')}`\n\n"
    )

    lines.append("## Summary\n\n")
    lines.append(str(review.get("summary", "")).strip() + "\n\n")

    lines.append("## Remaining Work\n\n")
    lines.append(markdown_list(list(review.get("remaining_work") or [])))
    lines.append("\n")

    lines.append("## Implementation Plan\n\n")
    plan = review.get("implementation_plan") or []
    if not plan:
        lines.append("No remediation tasks were returned.\n\n")
    else:
        for idx, item in enumerate(plan, start=1):
            lines.append(f"### {idx}. {item.get('task', 'Untitled task')}\n\n")

            lines.append("**Requirement references**\n\n")
            lines.append(markdown_list(list(item.get("requirement_refs") or [])))
            lines.append("\n")

            lines.append("**Likely files**\n\n")
            lines.append(markdown_list(list(item.get("files") or [])))
            lines.append("\n")

            lines.append("**Evidence**\n\n")
            lines.append(markdown_list(list(item.get("evidence") or [])))
            lines.append("\n")

            lines.append("**Instructions**\n\n")
            lines.append(str(item.get("instructions", "")).strip() + "\n\n")

            lines.append("**Design constraints**\n\n")
            lines.append(markdown_list(list(item.get("design_constraints") or [])))
            lines.append("\n")

            lines.append("**Test requirements**\n\n")
            lines.append(markdown_list(list(item.get("test_requirements") or [])))
            lines.append("\n")

            lines.append("**Acceptance checks**\n\n")
            lines.append(markdown_list(list(item.get("acceptance_checks") or [])))
            lines.append("\n")

    lines.append("## Verification Commands\n\n")
    lines.append(markdown_list(list(review.get("verification_commands") or [])))
    lines.append("\n")

    lines.append("## Risk Notes\n\n")
    lines.append(markdown_list(list(review.get("risk_notes") or [])))
    lines.append("\n")

    lines.append("## Handoff Notes\n\n")
    lines.append(markdown_list(list(review.get("handoff_notes") or [])))
    lines.append("\n")

    path.write_text("".join(lines), encoding="utf-8")


def normalize_commit_message(text: str) -> str:
    message = text.strip()

    fence = re.fullmatch(r"```(?:text|markdown)?\s*(.*?)\s*```", message, re.DOTALL)
    if fence:
        message = fence.group(1).strip()

    message = re.sub(r"^\s*(commit message|message)\s*:\s*", "", message, flags=re.I)

    if not message:
        raise LoopError("Commit message agent returned an empty message.")

    return message + "\n"


def derive_default_remediation_file(repo: Path, milestone_file: Path) -> Path:
    stem = milestone_file.stem
    safe = re.sub(r"[^A-Za-z0-9_.-]+", "_", stem).strip("_") or "milestone"
    return repo / ".plans" / f"{safe}_remediation.md"


def run_loop(
    *,
    paths: Paths,
    config: CodexConfig,
    milestone_file: Path,
    remediation_file: Path,
    prior_plan_glob: str,
    base_branch: str,
    max_iters: int,
    allow_base_branch: bool,
    allow_dirty_start: bool,
    no_commit: bool,
    review_subagents: bool,
) -> int:
    repo = paths.repo

    write_state(
        paths,
        status="starting",
        iteration=0,
        stage="preflight",
        reason="preflight checks starting",
        milestone_file=milestone_file,
        remediation_file=remediation_file,
    )

    branch = current_branch(repo)
    if not branch:
        raise StageError(
            iteration=0,
            stage="preflight",
            reason="detached HEAD",
            detail="Detached HEAD. Refusing to run automated changes.",
            resume_command=None,
        )

    if branch == base_branch and not allow_base_branch:
        raise StageError(
            iteration=0,
            stage="preflight",
            reason=f"on protected base branch {base_branch!r}",
            detail=(
                f"On base branch {base_branch!r}. Refusing to run automated changes there. "
                "Use --allow-base-branch if you really mean it."
            ),
            resume_command=None,
        )

    initial_status = git_status(repo)
    if initial_status and not allow_dirty_start:
        raise StageError(
            iteration=0,
            stage="preflight",
            reason="working tree is not clean",
            detail=(
                "Working tree is not clean. Commit/stash first, or use --allow-dirty-start.\n\n"
                f"{initial_status}"
            ),
            resume_command=None,
        )

    milestone_text = milestone_file.read_text(encoding="utf-8")

    for iteration in range(1, max_iters + 1):
        review_file = paths.run_dir / f"review-{iteration}.json"
        review_prompt = REVIEW_PROMPT.format(
            milestone=milestone_text,
            remediation_file=rel_display(remediation_file, repo),
            prior_plan_glob=prior_plan_glob,
            review_subagents=REVIEW_SUBAGENTS_TEXT if review_subagents else "",
        )

        codex_exec(
            paths=paths,
            config=config,
            prompt=review_prompt,
            model=config.review_model,
            reasoning=config.review_reasoning,
            sandbox=config.review_sandbox,
            output_file=review_file,
            iteration=iteration,
            stage="review",
            milestone_file=milestone_file,
            remediation_file=remediation_file,
            review_file=review_file,
            output_schema=paths.schema_file,
        )

        review = parse_json_file(review_file, paths=paths, iteration=iteration)
        write_remediation_markdown(remediation_file, review)

        print("\nReview summary:")
        print(str(review.get("summary", "")).strip())
        print(f"\nRemediation file: {rel_display(remediation_file, repo)}")

        if review.get("complete") is True:
            write_state(
                paths,
                status="complete",
                iteration=iteration,
                stage="complete",
                reason="reviewer reported milestone complete",
                milestone_file=milestone_file,
                remediation_file=remediation_file,
                review_file=review_file,
            )
            print("\nMilestone complete.")
            return 0

        plan = review.get("implementation_plan") or []
        if not plan:
            raise StageError(
                iteration=iteration,
                stage="review",
                reason="reviewer returned no implementation plan",
                detail=(
                    f"Review said milestone is incomplete but returned no implementation plan. "
                    f"See {review_file}"
                ),
                resume_command=None,
            )

        before_status = git_status(repo)
        implement_file = paths.run_dir / f"implement-{iteration}.md"
        implement_prompt = IMPLEMENT_PROMPT.format(
            milestone=milestone_text,
            remediation_file=rel_display(remediation_file, repo),
            plan_json=json.dumps(review, indent=2),
        )

        codex_exec(
            paths=paths,
            config=config,
            prompt=implement_prompt,
            model=config.implement_model,
            reasoning=config.implement_reasoning,
            sandbox=config.implement_sandbox,
            output_file=implement_file,
            iteration=iteration,
            stage="implementation",
            milestone_file=milestone_file,
            remediation_file=remediation_file,
            review_file=review_file,
        )

        after_status = git_status(repo)
        if after_status == before_status:
            raise StageError(
                iteration=iteration,
                stage="implementation",
                reason="implementation made no working-tree changes",
                detail=(
                    "Implementation made no working-tree changes. Stopping to avoid a spin loop.\n"
                    f"Implementation output: {implement_file}"
                ),
                resume_command=None,
            )

        if no_commit:
            write_state(
                paths,
                status="stopped",
                iteration=iteration,
                stage="implementation",
                reason="--no-commit requested",
                milestone_file=milestone_file,
                remediation_file=remediation_file,
                review_file=review_file,
                output_file=implement_file,
            )
            print("\n--no-commit set; stopping after implementation.")
            print(f"Review JSON: {review_file}")
            print(f"Implementation output: {implement_file}")
            return 0

        commit_raw_file = paths.run_dir / f"commit-{iteration}.raw.txt"
        commit_msg_file = paths.run_dir / f"commit-{iteration}.txt"

        codex_exec(
            paths=paths,
            config=config,
            prompt=COMMIT_PROMPT,
            model=config.commit_model,
            reasoning=config.commit_reasoning,
            sandbox=config.commit_sandbox,
            output_file=commit_raw_file,
            iteration=iteration,
            stage="commit-message",
            milestone_file=milestone_file,
            remediation_file=remediation_file,
            review_file=review_file,
        )

        commit_message = normalize_commit_message(
            commit_raw_file.read_text(encoding="utf-8")
        )
        commit_msg_file.write_text(commit_message, encoding="utf-8")

        print("\nCommit message:")
        print(commit_message)

        write_state(
            paths,
            status="running",
            iteration=iteration,
            stage="git-commit",
            reason="staging and committing changes",
            milestone_file=milestone_file,
            remediation_file=remediation_file,
            review_file=review_file,
            output_file=commit_msg_file,
        )

        try:
            git(repo, "add", "-A", capture=False)

            diff_cached = run(
                ["git", "diff", "--cached", "--quiet"],
                cwd=repo,
                capture=True,
                check=False,
            )
            if diff_cached.returncode == 0:
                raise StageError(
                    iteration=iteration,
                    stage="git-commit",
                    reason="no staged changes",
                    detail="No staged changes after git add -A. Refusing empty commit.",
                    resume_command=None,
                )

            git(repo, "commit", "-F", str(commit_msg_file), capture=False)
        except StageError:
            raise
        except LoopError as exc:
            raise StageError(
                iteration=iteration,
                stage="git-commit",
                reason="git commit failed",
                detail=str(exc),
                resume_command=None,
            ) from exc

        write_state(
            paths,
            status="stage-complete",
            iteration=iteration,
            stage="git-commit",
            reason="commit completed",
            milestone_file=milestone_file,
            remediation_file=remediation_file,
            review_file=review_file,
            output_file=commit_msg_file,
        )
        print(f"\nCommitted iteration {iteration}.")

    write_state(
        paths,
        status="stopped",
        iteration=max_iters,
        stage="max-iters",
        reason=f"reached max iterations: {max_iters}",
        milestone_file=milestone_file,
        remediation_file=remediation_file,
    )
    print(f"\nReached max iterations: {max_iters}", file=sys.stderr)
    return 3


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Run a Codex reviewer/implementer/committer milestone loop."
    )

    parser.add_argument(
        "milestone_file",
        nargs="?",
        type=Path,
        help="Path to the milestone description file.",
    )

    parser.add_argument(
        "--status",
        action="store_true",
        help="Show the latest recorded loop status and exit without running Codex.",
    )

    parser.add_argument(
        "--repo",
        type=Path,
        default=Path.cwd(),
        help="Repository path. Defaults to current directory.",
    )
    parser.add_argument(
        "--base-branch",
        default="main",
        help="Base branch to avoid modifying directly. Default: main.",
    )
    parser.add_argument(
        "--max-iters",
        type=int,
        default=6,
        help="Maximum review/implement/commit cycles. Default: 6.",
    )
    parser.add_argument(
        "--run-dir",
        type=Path,
        default=None,
        help=(
            "Directory for temporary run artifacts. "
            "Default: inside .git/codex-milestone-loop so it will not dirty the worktree."
        ),
    )

    parser.add_argument("--review-model", default="gpt-5.5")
    parser.add_argument("--review-reasoning", default="high")
    parser.add_argument("--implement-model", default="gpt-5.4")
    parser.add_argument("--implement-reasoning", default="medium")
    parser.add_argument("--commit-model", default="gpt-5.4-mini")
    parser.add_argument("--commit-reasoning", default="low")

    parser.add_argument(
        "--review-sandbox",
        choices=["read-only", "workspace-write", "danger-full-access"],
        default="read-only",
    )
    parser.add_argument(
        "--implement-sandbox",
        choices=["read-only", "workspace-write", "danger-full-access"],
        default="workspace-write",
    )
    parser.add_argument(
        "--commit-sandbox",
        choices=["read-only", "workspace-write", "danger-full-access"],
        default="read-only",
    )

    parser.add_argument(
        "--ask-for-approval",
        choices=["untrusted", "on-request", "never"],
        default="never",
        help=(
            "Codex approval mode. For automation, 'never' avoids hanging/failing on prompts. "
            "Default: never."
        ),
    )
    parser.add_argument(
        "--allow-network",
        action="store_true",
        help="Allow network access for Codex workspace-write commands.",
    )
    parser.add_argument(
        "--ephemeral-sessions",
        action="store_true",
        help=(
            "Pass --ephemeral to codex exec. This keeps Codex sessions from persisting, "
            "but disables useful resume behavior. Default: sessions are persisted."
        ),
    )
    parser.add_argument(
        "--allow-base-branch",
        action="store_true",
        help="Allow running directly on the base branch. Dangerous, but sometimes you are the danger.",
    )
    parser.add_argument(
        "--allow-dirty-start",
        action="store_true",
        help="Allow starting with existing uncommitted changes.",
    )
    parser.add_argument(
        "--no-commit",
        action="store_true",
        help="Run review and implementation, but do not commit.",
    )
    parser.add_argument(
        "--review-subagents",
        action="store_true",
        help="Ask the high-reasoning reviewer to spawn read-only review subagents if useful.",
    )
    parser.add_argument(
        "--remediation-file",
        type=Path,
        default=None,
        help=(
            "Path to the generated remediation markdown file. "
            "Default: .plans/<milestone_file_stem>_remediation.md"
        ),
    )
    parser.add_argument(
        "--prior-plan-glob",
        default=".plans/*.md",
        help="Glob/pattern describing prior milestone/remediation notes to consider.",
    )

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    paths: Paths | None = None

    try:
        require_tool("git")

        repo = resolve_repo(args.repo)

        if args.status:
            paths = resolve_paths(repo, args.run_dir, create=False)
            print(build_status_report(paths))
            return 0

        require_tool("codex")

        milestone_file = args.milestone_file
        if milestone_file is None:
            raise LoopError("Milestone file is required unless --status is set.")
        if not milestone_file.is_absolute():
            milestone_file = (Path.cwd() / milestone_file).resolve()

        if not milestone_file.exists():
            raise LoopError(f"Milestone file does not exist: {milestone_file}")

        paths = prepare_paths(repo, args.run_dir)

        remediation_file = args.remediation_file
        if remediation_file is None:
            remediation_file = derive_default_remediation_file(repo, milestone_file)
        elif not remediation_file.is_absolute():
            remediation_file = (repo / remediation_file).resolve()

        config = CodexConfig(
            review_model=args.review_model,
            review_reasoning=args.review_reasoning,
            implement_model=args.implement_model,
            implement_reasoning=args.implement_reasoning,
            commit_model=args.commit_model,
            commit_reasoning=args.commit_reasoning,
            review_sandbox=args.review_sandbox,  # type: ignore[arg-type]
            implement_sandbox=args.implement_sandbox,  # type: ignore[arg-type]
            commit_sandbox=args.commit_sandbox,  # type: ignore[arg-type]
            ask_for_approval=args.ask_for_approval,
            allow_network=args.allow_network,
            ephemeral_sessions=args.ephemeral_sessions,
        )

        return run_loop(
            paths=paths,
            config=config,
            milestone_file=milestone_file,
            remediation_file=remediation_file,
            prior_plan_glob=args.prior_plan_glob,
            base_branch=args.base_branch,
            max_iters=args.max_iters,
            allow_base_branch=args.allow_base_branch,
            allow_dirty_start=args.allow_dirty_start,
            no_commit=args.no_commit,
            review_subagents=args.review_subagents,
        )

    except StageError as exc:
        if paths is not None:
            write_failure_report(paths, exc)
            print_failure_banner(paths, exc)
        else:
            print(f"\nERROR: {exc}", file=sys.stderr)
        return 2
    except LoopError as exc:
        print(f"\nERROR: {exc}", file=sys.stderr)
        return 2
    except KeyboardInterrupt:
        print("\nInterrupted.", file=sys.stderr)
        return 130


if __name__ == "__main__":
    raise SystemExit(main())
