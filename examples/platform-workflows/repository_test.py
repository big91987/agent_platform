"""Real local Git repositories exercise publish boundaries; no remote API mock."""

import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path

SPEC = importlib.util.spec_from_file_location(
    "repository", Path(__file__).with_name("repository.py")
)
repo = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(repo)


class RepositoryTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.remote = self.root / "remote.git"
        subprocess.run(
            ["git", "init", "--bare", str(self.remote)], check=True, capture_output=True
        )
        self.workspace = self.root / "workspace"
        subprocess.run(
            ["git", "clone", str(self.remote), str(self.workspace)],
            check=True,
            capture_output=True,
        )
        for key, value in [
            ("user.name", "Workflow Test"),
            ("user.email", "workflow@example.invalid"),
        ]:
            repo.git(self.workspace, "config", key, value)
        repo.git(self.workspace, "checkout", "-b", "main")
        (self.workspace / "README.md").write_text("Example\n")
        repo.git(self.workspace, "add", "README.md")
        repo.git(self.workspace, "commit", "-m", "initial")
        repo.git(self.workspace, "push", "origin", "main")
        self.run = {
            "run_id": "a" * 32,
            "input": "Add a feature",
            "workspace_path": str(self.workspace),
        }

    def test_prepare_and_publish_preserve_main_and_reject_wrong_branch(self):
        repo.prepare(self.workspace, self.run, "main")
        (self.workspace / "feature.txt").write_text("implemented\n")
        docs = self.workspace / "docs/workflow"
        docs.mkdir(parents=True)
        (docs / "qa.md").write_text("Validation evidence\n")
        (docs / "pr.md").write_text("Background and behavior\n")
        repo.publish(self.workspace, self.run)
        self.assertEqual(
            repo.git(self.workspace, "show", "origin/main:README.md"), "Example"
        )
        self.assertEqual(
            repo.git(
                self.workspace, "show", "origin/workflow/" + "a" * 32 + ":feature.txt"
            ),
            "implemented",
        )
        original = repo.git(self.workspace, "rev-parse", "HEAD")
        repo.publish(self.workspace, self.run)
        self.assertEqual(repo.git(self.workspace, "rev-parse", "HEAD"), original)
        repo.git(self.workspace, "checkout", "main")
        with self.assertRaises(ValueError):
            repo.publish(self.workspace, self.run)

    def test_dirty_initial_workspace_and_secret_are_rejected(self):
        (self.workspace / "leftover.txt").write_text("preserve me")
        with self.assertRaises(ValueError):
            repo.prepare(self.workspace, self.run, "main")
        self.assertEqual((self.workspace / "leftover.txt").read_text(), "preserve me")
        (self.workspace / "leftover.txt").unlink()
        repo.prepare(self.workspace, self.run, "main")
        (self.workspace / ".env").write_text("PASSWORD=private")
        with self.assertRaises(ValueError):
            repo.check_publish_files(self.workspace)
        self.assertEqual(
            repo.git(self.workspace, "diff", "--cached", "--name-only"), ""
        )

    def test_remote_identity_normalizes_only_github_ssh_and_https(self):
        self.assertEqual(
            repo.remote_repository("git@github.com:owner/repo.git"), "owner/repo"
        )
        self.assertEqual(
            repo.remote_repository("https://github.com/owner/repo.git"), "owner/repo"
        )
        for value in [
            "https://secret@github.com/owner/repo",
            "https://evil.example/owner/repo",
            "../local",
        ]:
            with self.assertRaises(ValueError):
                repo.remote_repository(value)


if __name__ == "__main__":
    unittest.main()
