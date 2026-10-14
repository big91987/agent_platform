import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from sandbox_prepare import prepare


class SandboxPreparationTest(unittest.TestCase):
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
