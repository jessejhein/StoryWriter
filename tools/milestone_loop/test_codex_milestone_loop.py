"""Regression tests for the milestone loop Codex command builder."""

from __future__ import annotations

import contextlib
import importlib.util
import io
import json
import tempfile
import sys
from pathlib import Path
import unittest
from unittest.mock import patch


MODULE_PATH = Path(__file__).with_name("codex_milestone_loop.py")
SPEC = importlib.util.spec_from_file_location("codex_milestone_loop", MODULE_PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"Could not load module from {MODULE_PATH}")

MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class BuildCodexExecCommandTests(unittest.TestCase):
    def test_places_approval_policy_before_exec(self) -> None:
        config = MODULE.CodexConfig(
            review_model="review",
            review_reasoning="high",
            implement_model="implement",
            implement_reasoning="medium",
            commit_model="commit",
            commit_reasoning="low",
            review_sandbox="read-only",
            implement_sandbox="workspace-write",
            commit_sandbox="read-only",
            ask_for_approval="never",
            allow_network=False,
            ephemeral_sessions=False,
        )

        cmd = MODULE.build_codex_exec_command(
            config=config,
            model="gpt-5.4-mini",
            reasoning="high",
            sandbox="read-only",
            output_file=Path("/tmp/output.json"),
        )

        self.assertEqual(
            cmd[:6],
            [
                "codex",
                "--ask-for-approval",
                "never",
                "exec",
                "--color",
                "never",
            ],
        )
        self.assertNotIn("--ask-for-approval", cmd[4:])

    def test_inserts_ephemeral_and_output_schema_in_expected_positions(self) -> None:
        config = MODULE.CodexConfig(
            review_model="review",
            review_reasoning="high",
            implement_model="implement",
            implement_reasoning="medium",
            commit_model="commit",
            commit_reasoning="low",
            review_sandbox="read-only",
            implement_sandbox="workspace-write",
            commit_sandbox="read-only",
            ask_for_approval="on-request",
            allow_network=True,
            ephemeral_sessions=True,
        )

        cmd = MODULE.build_codex_exec_command(
            config=config,
            model="gpt-5.4-mini",
            reasoning="high",
            sandbox="workspace-write",
            output_file=Path("/tmp/output.json"),
            output_schema=Path("/tmp/schema.json"),
        )

        self.assertEqual(
            cmd[:8],
            [
                "codex",
                "--ask-for-approval",
                "on-request",
                "exec",
                "--ephemeral",
                "--color",
                "never",
                "--sandbox",
            ],
        )
        self.assertEqual(
            cmd[
                cmd.index("--sandbox") + 1 : cmd.index("--sandbox") + 3
            ],
            ["workspace-write", "--model"],
        )
        self.assertIn("-c", cmd)
        self.assertIn("sandbox_workspace_write.network_access=true", cmd)
        self.assertEqual(
            cmd[-3:],
            ["--output-schema", "/tmp/schema.json", "-"],
        )


class StatusCommandTests(unittest.TestCase):
    def test_build_status_report_summarizes_state_and_last_output(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            repo = root / "repo"
            run_dir = repo / ".git" / "codex-milestone-loop"
            run_dir.mkdir(parents=True)

            state = {
                "timestamp_utc": "2026-07-07T04:26:48+00:00",
                "status": "failed",
                "iteration": 2,
                "stage": "review",
                "reason": "codex exited with status 1",
                "repo": str(repo),
                "branch": "milestone-8",
                "milestone_file": "docs/17_milestone_8_task_prompt.md",
                "remediation_file": ".plans/17_milestone_8_task_prompt_remediation.md",
                "review_file": ".git/codex-milestone-loop/review-2.json",
                "output_file": ".git/codex-milestone-loop/review-2.json",
                "resume_command": "tools/milestone_loop/resume_last_codex.sh review",
                "detail": "Codex exited with status 1 during iteration 2, stage 'review'.",
            }
            (run_dir / "state.json").write_text(
                json.dumps(state, indent=2) + "\n",
                encoding="utf-8",
            )
            (run_dir / "review-1.json").write_text("{}", encoding="utf-8")
            (run_dir / "implement-1.md").write_text("implementation", encoding="utf-8")
            (run_dir / "commit-1.raw.txt").write_text("commit raw", encoding="utf-8")
            (run_dir / "commit-1.txt").write_text("commit message", encoding="utf-8")
            (run_dir / "review-2.json").write_text(
                "first line\nsecond line\nthird line",
                encoding="utf-8",
            )

            paths = MODULE.Paths(
                repo=repo,
                run_dir=run_dir,
                schema_file=run_dir / "milestone-review.schema.json",
                state_file=run_dir / "state.json",
                latest_failure_file=run_dir / "latest_failure.md",
            )

            with (
                patch.object(MODULE, "current_branch", return_value="milestone-8"),
                patch.object(MODULE, "git_status", return_value=""),
            ):
                report = MODULE.build_status_report(paths)

        self.assertIn("Codex Milestone Loop Status", report)
        self.assertIn("Status   : failed", report)
        self.assertIn("Iteration: 2", report)
        self.assertIn("Stage    : review", report)
        self.assertIn("Resume command: tools/milestone_loop/resume_last_codex.sh review", report)
        self.assertIn("Iteration 1: review, implementation, commit-message-raw, commit-message", report)
        self.assertIn("Iteration 2: review", report)
        self.assertIn("Last output", report)
        self.assertIn("first line", report)
        self.assertIn("second line", report)
        self.assertIn("third line", report)

    def test_main_status_mode_prints_report_without_codex_requirement(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            repo = root / "repo"
            run_dir = repo / ".git" / "codex-milestone-loop"
            run_dir.mkdir(parents=True)

            with patch.object(MODULE, "resolve_repo", return_value=repo), patch.object(
                MODULE, "require_tool"
            ) as require_tool, patch.object(
                MODULE, "build_status_report", return_value="STATUS REPORT"
            ) as build_status_report, patch.object(
                sys,
                "argv",
                [
                    "codex_milestone_loop.py",
                    "--status",
                    "--repo",
                    str(repo),
                    "--run-dir",
                    str(run_dir),
                ],
            ):
                stdout = io.StringIO()
                with contextlib.redirect_stdout(stdout):
                    exit_code = MODULE.main()

        self.assertEqual(exit_code, 0)
        self.assertEqual(stdout.getvalue(), "STATUS REPORT\n")
        require_tool.assert_called_once_with("git")
        build_status_report.assert_called_once()


if __name__ == "__main__":
    unittest.main()
