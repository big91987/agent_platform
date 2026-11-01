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
        base = self.git("rev-parse", "HEAD")
        self.assertEqual(self.workspace.integrate(base), base)
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

    def private_report_candidate(self):
        base = self.git("rev-parse", "HEAD")
        self.git("update-ref", "refs/remotes/origin/agent-team/experiment", base)
        work = self.workspace.prepare("report", 1)
        task = {"id": "report", "title": "report", "write_paths": ["report.md"]}
        (work["path"] / "report.md").write_text("private diagnostic placeholder")
        private = self.workspace.commit(task, work)
        work["base"] = private
        (work["path"] / "report.md").write_text("sanitized report")
        clean = self.workspace.commit(task, work)
        return work, private, clean

    def test_integration_exports_clean_content_without_private_candidate_history(self):
        work, private, clean = self.private_report_candidate()
        base = self.git("rev-parse", "refs/remotes/origin/agent-team/experiment")
        head = self.workspace.integrate(clean)
        self.assertEqual(self.git("show", head + ":report.md"), "sanitized report")
        self.assertNotEqual(head, clean)
        exported = self.root / "exported"
        self.git("init", "-b", "agent-team/experiment", str(exported))
        self.git("fetch", str(self.repo), base, cwd=exported)
        self.git("reset", "--hard", "FETCH_HEAD", cwd=exported)
        bundle = self.workspace.bundle(head)
        self.git("fetch", bundle["bundle"], "agent-team/experiment", cwd=exported)
        self.assertEqual(
            self.git("show", "FETCH_HEAD:report.md", cwd=exported), "sanitized report"
        )
        for private_object in [private, self.git("rev-parse", private + ":report.md")]:
            result = subprocess.run(
                ["git", "cat-file", "-e", private_object],
                cwd=exported,
                capture_output=True,
            )
            self.assertNotEqual(
                result.returncode, 0, "private candidate object was exported"
            )
        self.assertEqual(
            self.git("rev-list", "--parents", "-n", "1", head).split(), [head, base]
        )
        self.assertEqual(
            self.git("show", private + ":report.md", cwd=work["path"]),
            "private diagnostic placeholder",
        )

    def test_integration_hook_cannot_reintroduce_private_ancestry(self):
        _, _, clean = self.private_report_candidate()
        hook = self.repo / ".git/hooks/post-commit"
        hook.write_text(f"#!/bin/sh\ngit reset --soft {clean}\n")
        hook.chmod(0o700)
        with self.assertRaises(ValueError):
            self.workspace.integrate(clean)
        with self.assertRaises(ValueError):
            self.workspace.bundle(self.git("rev-parse", "HEAD"))
        with self.assertRaises(ValueError):
            self.workspace.prepare("later", 1)

    def test_integration_hook_cannot_change_the_approved_tree(self):
        _, _, clean = self.private_report_candidate()
        hook = self.repo / ".git/hooks/post-commit"
        hook.write_text(
            '#!/bin/sh\nrm "$0"\nprintf tampered > report.md\ngit add report.md\ngit commit --amend --no-edit\n'
        )
        hook.chmod(0o700)
        with self.assertRaises(ValueError):
            self.workspace.integrate(clean)
        with self.assertRaises(ValueError):
            self.workspace.bundle(self.git("rev-parse", "HEAD"))

    def test_merge_hook_cannot_replace_clean_content_with_private_diagnostics(self):
        _, private, clean = self.private_report_candidate()
        hook = self.repo / ".git/hooks/post-merge"
        hook.write_text(
            f"#!/bin/sh\ngit read-tree {private}^{{tree}}\ngit checkout-index -a -f\n"
        )
        hook.chmod(0o700)
        with self.assertRaises(ValueError):
            self.workspace.integrate(clean)
        with self.assertRaises(ValueError):
            self.workspace.bundle(self.git("rev-parse", "HEAD"))

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
