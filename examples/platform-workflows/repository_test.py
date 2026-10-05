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

    def test_task_artifacts_use_skill_filenames_and_do_not_overwrite_other_runs(self):
        receipt = repo.prepare(self.workspace, self.run, "main", True)
        directory = self.workspace / receipt["document_root"]
        directory.mkdir(parents=True)
        other = self.workspace / "docs/workflow/runs" / ("b" * 32)
        other.mkdir()
        (other / "acceptance-matrix.md").write_text("Other task evidence")
        artifact = directory / "verification" / "acceptance-matrix.md"
        artifact.parent.mkdir()
        artifact.write_text("Actual Skill report")
        (directory / "pr.md").write_text("PR delivery")
        self.run["previous_results"] = [
            {
                "node_id": "qa",
                "status": "completed",
                "result": {
                    "route": "next",
                    "artifacts": [str(artifact.relative_to(self.workspace))],
                },
            }
        ]
        repo.publish(self.workspace, self.run, True)
        self.assertEqual(
            (other / "acceptance-matrix.md").read_text(), "Other task evidence"
        )
        self.assertFalse((directory / "qa.md").exists())
        self.assertEqual(
            repo.prepare(self.workspace, self.run, "main", True)["document_root"],
            receipt["document_root"],
        )
        self.run["previous_results"][0]["result"]["artifacts"] = [
            str((other / "acceptance-matrix.md").relative_to(self.workspace))
        ]
        with self.assertRaisesRegex(ValueError, "this task"):
            repo.publish(self.workspace, self.run, True)

    def test_two_task_branches_merge_without_document_conflicts(self):
        branches = []
        for identity in ("a" * 32, "b" * 32):
            repo.git(self.workspace, "checkout", "main")
            run = {**self.run, "run_id": identity}
            prepared = repo.prepare(self.workspace, run, "main", True)
            directory = self.workspace / prepared["document_root"]
            directory.mkdir(parents=True)
            report = directory / "acceptance-matrix.md"
            report.write_text("Evidence for " + identity)
            (directory / "pr.md").write_text("Delivery for " + identity)
            run["previous_results"] = [
                {
                    "node_id": "qa",
                    "status": "completed",
                    "result": {
                        "route": "next",
                        "artifacts": [str(report.relative_to(self.workspace))],
                    },
                }
            ]
            repo.publish(self.workspace, run, True)
            branches.append(prepared["branch"])
        repo.git(self.workspace, "checkout", "main")
        for branch in branches:
            repo.git(self.workspace, "merge", "--no-edit", branch)
        for identity in ("a" * 32, "b" * 32):
            self.assertEqual(
                (
                    self.workspace
                    / "docs/workflow/runs"
                    / identity
                    / "acceptance-matrix.md"
                ).read_text(),
                "Evidence for " + identity,
            )
        self.assertEqual(
            repo.git(self.workspace, "diff", "--name-only", "--diff-filter=U"), ""
        )

    def test_publication_rejects_missing_or_newer_failed_qa(self):
        repo.prepare(self.workspace, self.run, "main", True)
        with self.assertRaisesRegex(ValueError, "QA handoff"):
            repo.publish(self.workspace, self.run, True)
        self.run["previous_results"] = [
            {
                "node_id": "qa",
                "status": "completed",
                "result": {"route": "next", "artifacts": ["old.md"]},
            },
            {
                "node_id": "qa",
                "status": "completed",
                "result": {"route": "development"},
            },
        ]
        with self.assertRaisesRegex(ValueError, "QA handoff"):
            repo.publish(self.workspace, self.run, True)

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
