"""Regression tests for the milestone loop Codex command builder."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
import unittest


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


if __name__ == "__main__":
    unittest.main()
