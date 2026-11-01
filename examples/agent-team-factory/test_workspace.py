import subprocess
import tempfile
import unittest
from pathlib import Path

from git_workspace import Workspace


class WorkspaceTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.git("init", "-b", "main")
        self.git("config", "user.email", "factory-test@example.invalid")
        self.git("config", "user.name", "Factory Test")
        (self.repo / "initial.txt").write_text("original\n")
        self.git("add", "initial.txt")
        self.git("commit", "-m", "initial")
        self.git("switch", "-c", "agent-team/experiment")
        self.workspace = Workspace(
            self.repo, self.root / "run", "agent-team/experiment"
        )

    def git(self, *args, cwd=None):
        return subprocess.check_output(
            ["git", *args], cwd=cwd or self.repo, text=True
        ).strip()

    def test_worktrees_and_commits_are_isolated_until_explicit_integration(self):
        first = self.workspace.prepare("first", 1)
        second = self.workspace.prepare("second", 1)
        self.assertIn("agent-team", str(first["path"]))
        (first["path"] / "feature.txt").write_text("first\n")
        self.assertFalse((second["path"] / "feature.txt").exists())
        task = {"id": "first", "title": "feature", "write_paths": ["feature.txt"]}
        commit = self.workspace.commit(task, first)
        self.assertFalse((self.repo / "feature.txt").exists())
        self.workspace.integrate(commit)
        self.assertEqual((self.repo / "feature.txt").read_text(), "first\n")
        self.assertEqual(self.git("show", "main:initial.txt"), "original")

    def test_out_of_scope_and_wrong_branch_are_preserved_without_commit(self):
        work = self.workspace.prepare("first", 1)
        (work["path"] / "outside.txt").write_text("preserve me")
        before = self.git("rev-parse", "HEAD", cwd=work["path"])
        with self.assertRaises(ValueError):
            self.workspace.commit(
                {"id": "first", "title": "feature", "write_paths": ["allowed.txt"]},
                work,
            )
        self.assertEqual(self.git("rev-parse", "HEAD", cwd=work["path"]), before)
        self.assertTrue((work["path"] / "outside.txt").exists())
        self.git("switch", "-c", "other", cwd=work["path"])
        with self.assertRaises(ValueError):
            self.workspace.commit(
                {"id": "first", "title": "feature", "write_paths": ["outside.txt"]},
                work,
            )

    def test_main_target_is_rejected_and_merge_conflict_keeps_evidence(self):
        with self.assertRaises(ValueError):
            Workspace(self.repo, self.root / "unsafe", "main")
        commits = []
        for name in ("first", "second"):
            work = self.workspace.prepare(name, 1)
            (work["path"] / "initial.txt").write_text(name)
            commits.append(
                self.workspace.commit(
                    {"id": name, "title": name, "write_paths": ["initial.txt"]}, work
                )
            )
        self.workspace.integrate(commits[0])
        with self.assertRaises(subprocess.CalledProcessError):
            self.workspace.integrate(commits[1])
        self.assertTrue(self.git("diff", "--name-only", "--diff-filter=U"))

    def test_commit_hook_cannot_expand_the_candidate_scope(self):
        work = self.workspace.prepare("hook", 1)
        (work["path"] / "allowed.txt").write_text("allowed")
        common = Path(self.git("rev-parse", "--git-common-dir"))
        if not common.is_absolute():
            common = self.repo / common
        hook = common / "hooks/pre-commit"
        hook.write_text(
            "#!/bin/sh\nprintf outside > outside.txt\ngit add outside.txt\n"
        )
        hook.chmod(0o700)
        with self.assertRaises(ValueError):
            self.workspace.commit(
                {"id": "hook", "title": "hook", "write_paths": ["allowed.txt"]}, work
            )
        self.assertFalse((self.repo / "outside.txt").exists())

    def test_new_worktree_inherits_sparse_checkout_without_fetching_historical_artifacts(
        self,
    ):
        (self.repo / "history").mkdir()
        (self.repo / "history/large.bin").write_bytes(b"historical fixture")
        self.git("add", "history/large.bin")
        self.git("commit", "-m", "historical fixture")
        self.git("sparse-checkout", "set", "--no-cone", "/initial.txt")
        work = self.workspace.prepare("sparse", 1)
        self.assertTrue((work["path"] / "initial.txt").exists())
        self.assertFalse((work["path"] / "history/large.bin").exists())
        self.assertEqual(self.git("status", "--porcelain", cwd=work["path"]), "")


if __name__ == "__main__":
    unittest.main()
