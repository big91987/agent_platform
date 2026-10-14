import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from sandbox_publish import publish


class PublicationBoundaryTest(unittest.TestCase):
    def fixture(self, root):
        def git(*args):
            return subprocess.check_output(
                ["git", "-C", str(root), *args], stderr=subprocess.DEVNULL, text=True
            ).strip()

        git("init", "--quiet")
        git("config", "user.name", "Fixture")
        git("config", "user.email", "fixture@example.invalid")
        (root / "reviewed.txt").write_text("base")
        (root / "unreviewed.txt").write_text("base")
        git("add", ".")
        git("commit", "--quiet", "-m", "base")
        head = git("rev-parse", "HEAD")
        git("checkout", "--quiet", "-b", "codex/run-" + "a" * 12)
        git("remote", "add", "origin", "https://github.com/owner/project.git")
        (root / ".git/agent-platform-prepared.json").write_text(
            json.dumps(
                {"repository": "owner/project", "commit": head, "run_id": "a" * 32}
            )
        )
        (root / "harness-output").mkdir()
        (root / "harness-output/qa.md").write_text("Independent QA fixture")
        report = root / "report.json"
        report.write_text(
            json.dumps(
                {
                    "title": "【Harness验证】验证共享文件",
                    "body": "Actual evidence",
                    "files": ["reviewed.txt"],
                }
            )
        )
        return git, report

    def test_precommitted_unreviewed_change_is_rejected_before_network(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            (root / "unreviewed.txt").write_text("changed")
            git("add", "unreviewed.txt")
            git("commit", "--quiet", "-m", "prior change")
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "unreviewed"):
                    publish("owner/project", report)

    def test_other_push_host_is_rejected_before_credentials_are_requested(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            git("config", "remote.origin.pushurl", "https://other.example/project.git")
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "destination overrides"):
                    publish("owner/project", report)

    def test_secret_committed_then_deleted_is_rejected_before_network(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            (root / ".env").write_text("private-fixture")
            git("add", ".env")
            git("commit", "--quiet", "-m", "temporary file")
            git("rm", "--quiet", ".env")
            git("commit", "--quiet", "-m", "delete temporary file")
            original = subprocess.run

            def safe_run(args, **kwargs):
                if "push" in args:
                    raise AssertionError("network reached before history validation")
                return original(args, **kwargs)

            with (
                patch("sandbox_publish.Path.cwd", return_value=root),
                patch.dict(os.environ, {"GITHUB_TOKEN": "fixture"}),
                patch("sandbox_publish.subprocess.run", side_effect=safe_run),
            ):
                with self.assertRaisesRegex(ValueError, "unreviewed|credential"):
                    publish("owner/project", report)

    def test_secret_blob_in_reviewed_file_then_reverted_is_still_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            (root / "reviewed.txt").write_text("github_pat_" + "a" * 36)
            git("add", "reviewed.txt")
            git("commit", "--quiet", "-m", "prior sensitive version")
            (root / "reviewed.txt").write_text("base")
            git("add", "reviewed.txt")
            git("commit", "--quiet", "-m", "restore")
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "private credential"):
                    publish("owner/project", report)

    def test_staged_secret_deleted_from_worktree_cannot_be_committed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git, report = self.fixture(root)
            (root / ".env").write_text("private-fixture")
            git("add", ".env")
            (root / ".env").unlink()
            with patch("sandbox_publish.Path.cwd", return_value=root):
                with self.assertRaisesRegex(ValueError, "unreviewed staged"):
                    publish("owner/project", report)
