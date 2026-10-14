import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from sandbox_prepare import prepare


class SandboxPreparationTest(unittest.TestCase):
    def test_preparation_keeps_pinned_head_and_history_required_by_verification(self):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            source, workspace = base / "source", base / "workspace"
            workspace.mkdir()
            real_run = subprocess.run

            def git(*args):
                return subprocess.check_output(
                    ["git", "-C", str(source), *args],
                    stderr=subprocess.DEVNULL,
                    text=True,
                ).strip()

            real_run(["git", "init", "--quiet", str(source)], check=True)
            git("config", "user.name", "fixture")
            git("config", "user.email", "fixture@example.invalid")
            (source / "historical.txt").write_text("historical gate fixture")
            git("add", ".")
            git("commit", "--quiet", "-m", "historical baseline")
            historical = git("rev-parse", "HEAD")
            (source / "current.txt").write_text("current source")
            git("add", ".")
            git("commit", "--quiet", "-m", "current source")
            commit = git("rev-parse", "HEAD")
            context = {
                "run_id": "a" * 32,
                "parameters": {
                    "repository": "owner/project",
                    "repository_commit": commit,
                },
            }

            def local_fetch(command, **kwargs):
                command = list(command)
                if "fetch" in command:
                    self.assertEqual(command[-2:], ["origin", commit])
                    command[-2] = source.as_uri()
                return real_run(command, **kwargs)

            with (
                patch.dict(os.environ, {"GITHUB_TOKEN": "fixture"}),
                patch("sandbox_prepare.subprocess.run", side_effect=local_fetch),
            ):
                prepare(context, "owner/project", workspace)
            head = subprocess.check_output(
                ["git", "-C", str(workspace), "rev-parse", "HEAD"], text=True
            ).strip()
            self.assertEqual(head, commit)
            history = real_run(
                [
                    "git",
                    "-C",
                    str(workspace),
                    "cat-file",
                    "-e",
                    historical + ":historical.txt",
                ],
                capture_output=True,
            )
            self.assertEqual(
                history.returncode, 0, "fixed verification lost its historical baseline"
            )
            (workspace / "work-in-progress.md").write_text(
                "preserve on repeated preparation"
            )
            with (
                patch.dict(os.environ, {"GITHUB_TOKEN": "fixture"}),
                patch("sandbox_prepare.subprocess.run") as command,
            ):
                prepare(context, "owner/project", workspace)
                command.assert_not_called()
            self.assertEqual(
                (workspace / "work-in-progress.md").read_text(),
                "preserve on repeated preparation",
            )

    def test_foreign_repository_and_unowned_workspace_never_dispatch_git(self):
        context = {
            "run_id": "a" * 32,
            "parameters": {
                "repository": "owner/project",
                "repository_commit": "b" * 40,
            },
        }
        with (
            tempfile.TemporaryDirectory() as directory,
            patch.dict(os.environ, {"GITHUB_TOKEN": "fixture"}),
            patch("sandbox_prepare.subprocess.run") as command,
        ):
            root = Path(directory)
            with self.assertRaises(ValueError):
                prepare(context, "other/project", root)
            (root / "important").write_text("preserve")
            with self.assertRaises(ValueError):
                prepare(context, "owner/project", root)
            command.assert_not_called()
            self.assertEqual((root / "important").read_text(), "preserve")

    def test_owned_preparation_is_reused_without_replaying_fetch(self):
        context = {
            "run_id": "a" * 32,
            "parameters": {
                "repository": "owner/project",
                "repository_commit": "b" * 40,
            },
        }
        with (
            tempfile.TemporaryDirectory() as directory,
            patch.dict(os.environ, {"GITHUB_TOKEN": "fixture"}),
            patch("sandbox_prepare.subprocess.run") as command,
        ):
            root = Path(directory)
            (root / ".git").mkdir()
            (root / ".git/agent-platform-prepared.json").write_text(
                json.dumps(
                    {
                        "repository": "owner/project",
                        "commit": "b" * 40,
                        "run_id": "a" * 32,
                    }
                )
            )
            prepare(context, "owner/project", root)
            command.assert_not_called()
            context["parameters"]["repository_commit"] = "c" * 40
            with self.assertRaises(ValueError):
                prepare(context, "owner/project", root)
