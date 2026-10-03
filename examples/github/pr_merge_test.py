"""Exercise PR synchronization against local bare Git repositories."""

import subprocess
import tempfile
import unittest
from pathlib import Path

try:
    import pr_merge
except ImportError:
    pr_merge = None


class MergeTest(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(pr_merge, "PR merge support is not implemented")
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.remote = self.root / "remote.git"
        self.git(self.root, "init", "--bare", str(self.remote))
        self.main = self.root / "main"
        self.git(self.root, "clone", str(self.remote), str(self.main))
        self.identity(self.main)
        self.git(self.main, "checkout", "-b", "main")
        self.commit(
            self.main,
            {
                "app.txt": "original\n",
                "other.txt": "original\n",
                "AGENTS.md": "original\n",
            },
        )
        self.git(self.main, "push", "origin", "main")
        self.workspace = self.root / "workspace"
        self.git(
            self.root, "clone", "-b", "main", str(self.remote), str(self.workspace)
        )
        self.identity(self.workspace)
        self.git(self.workspace, "checkout", "-b", "task/1")
        self.commit(self.workspace, {"feature.txt": "feature\n"})
        self.head = self.git(self.workspace, "rev-parse", "HEAD")
        self.base = self.git(self.main, "rev-parse", "HEAD")

    def git(self, cwd, *args):
        result = subprocess.run(
            ["git", "-C", str(cwd), *args],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout.strip()

    def identity(self, repo):
        self.git(repo, "config", "user.name", "Merge Test")
        self.git(repo, "config", "user.email", "merge-test@example.invalid")

    def commit(self, repo, files):
        for name, content in files.items():
            path = repo / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content)
        self.git(repo, "add", "--all")
        self.git(repo, "commit", "-m", "fixture")

    def advance_main(self, files):
        self.commit(self.main, files)
        self.git(self.main, "push", "origin", "main")
        self.git(self.workspace, "fetch", "origin", "main")
        self.base = self.git(self.main, "rev-parse", "HEAD")

    def prepare(self):
        return pr_merge.prepare_merge(self.workspace, "task/1", self.head, self.base)

    def conflict(self, name="app.txt"):
        self.commit(self.workspace, {name: "branch\n"})
        self.head = self.git(self.workspace, "rev-parse", "HEAD")
        self.advance_main({name: "main\n"})
        return self.prepare()

    def test_creates_two_parent_merge_and_inherits_nonconflicting_configuration(self):
        self.advance_main({"main.txt": "new\n", ".github/workflows/ci.yml": "new\n"})
        result = self.prepare()
        self.assertEqual(result["state"], "merged")
        self.assertEqual(
            self.git(self.workspace, "show", "-s", "--format=%P", "HEAD"),
            f"{self.head} {self.base}",
        )
        self.assertEqual(
            (self.workspace / ".github/workflows/ci.yml").read_text(), "new\n"
        )
        self.assertEqual(self.git(self.workspace, "status", "--porcelain"), "")

    def test_ancestor_does_not_create_commit(self):
        self.assertEqual(
            self.prepare(), {"state": "unchanged", "conflicts": [], "head": self.head}
        )

    def test_fast_forward_is_still_merge_commit(self):
        self.git(self.workspace, "merge", "--ff-only", "origin/main")
        self.git(self.main, "fetch", str(self.workspace), "task/1")
        self.git(self.main, "merge", "--ff-only", "FETCH_HEAD")
        self.advance_main({"main.txt": "new\n"})
        self.assertEqual(self.prepare()["state"], "merged")
        self.assertEqual(
            self.git(self.workspace, "show", "-s", "--format=%P", "HEAD"),
            f"{self.head} {self.base}",
        )

    def test_conflict_preserved_and_can_be_finished_with_staged_documents(self):
        result = self.conflict()
        self.assertEqual(
            result, {"state": "conflicts", "conflicts": ["app.txt"], "head": self.head}
        )
        self.assertEqual(self.git(self.workspace, "rev-parse", "MERGE_HEAD"), self.base)
        (self.workspace / "app.txt").write_text("resolved\n")
        (self.workspace / "docs").mkdir()
        (self.workspace / "docs/handoff.md").write_text("review\n")
        self.git(self.workspace, "add", "docs/handoff.md")
        head = pr_merge.finish_merge(self.workspace, ["app.txt"])
        self.assertEqual(head, self.git(self.workspace, "rev-parse", "HEAD"))
        self.assertEqual(
            self.git(self.workspace, "show", "-s", "--format=%P", "HEAD"),
            f"{self.head} {self.base}",
        )

    def test_retry_resumes_same_merge_without_overwriting_resolution(self):
        self.conflict()
        (self.workspace / "app.txt").write_text("resolved\n")
        self.assertEqual(self.prepare()["state"], "conflicts")
        self.assertEqual((self.workspace / "app.txt").read_text(), "resolved\n")
        self.git(self.workspace, "add", "app.txt")
        self.assertEqual(self.prepare()["conflicts"], [])
        pr_merge.finish_merge(self.workspace, ["app.txt"])
        with self.assertRaises(pr_merge.MergeError):
            self.prepare()

    def test_does_not_overwrite_dirty_or_untracked_work(self):
        self.advance_main({"other.txt": "main\n"})
        for name in ("app.txt", "untracked.txt"):
            with self.subTest(name=name):
                path = self.workspace / name
                path.write_text("local work\n")
                with self.assertRaises(pr_merge.MergeError):
                    self.prepare()
                self.assertEqual(path.read_text(), "local work\n")
                path.unlink()
                if name == "app.txt":
                    self.git(self.workspace, "restore", "app.txt")

    def test_rejects_changed_head_or_wrong_branch(self):
        self.commit(self.workspace, {"other.txt": "new\n"})
        with self.assertRaises(pr_merge.MergeError):
            self.prepare()
        self.head = self.git(self.workspace, "rev-parse", "HEAD")
        self.git(self.workspace, "checkout", "-b", "other")
        with self.assertRaises(pr_merge.MergeError):
            self.prepare()

    def test_protected_conflict_aborts_only_our_merge(self):
        for name in (
            "AGENTS.md",
            ".github/workflows/ci.yml",
            "harness/policy.md",
            "full_harness/policy.md",
            "scripts/build.sh",
            "nested/AGENTS.md",
        ):
            with self.subTest(name=name):
                self.commit(self.workspace, {name: "branch\n"})
                self.head = self.git(self.workspace, "rev-parse", "HEAD")
                self.advance_main({name: "main\n"})
                with self.assertRaises(pr_merge.ManualResolutionRequired):
                    self.prepare()
                self.assertEqual(self.git(self.workspace, "status", "--porcelain"), "")
                self.assertEqual(
                    self.git(self.workspace, "rev-parse", "HEAD"), self.head
                )
                self.assertFalse((self.workspace / ".git/MERGE_HEAD").exists())

    def test_finish_rejects_conflict_markers_without_staging_them(self):
        self.conflict()
        with self.assertRaises(pr_merge.MergeError):
            pr_merge.finish_merge(self.workspace, ["app.txt"])
        self.assertEqual(
            self.git(self.workspace, "diff", "--name-only", "--diff-filter=U"),
            "app.txt",
        )
        self.assertTrue((self.workspace / ".git/MERGE_HEAD").exists())

    def test_finish_requires_complete_conflict_allowlist(self):
        self.conflict()
        (self.workspace / "app.txt").write_text("resolved\n")
        with self.assertRaises(pr_merge.MergeError):
            pr_merge.finish_merge(self.workspace, [])
        with self.assertRaises(pr_merge.MergeError):
            pr_merge.finish_merge(self.workspace, ["../outside"])

    def test_finish_rejects_unstaged_additional_work(self):
        self.conflict()
        (self.workspace / "app.txt").write_text("resolved\n")
        (self.workspace / "AGENTS.md").write_text("unauthorized\n")
        with self.assertRaises(pr_merge.MergeError):
            pr_merge.finish_merge(self.workspace, ["app.txt"])
        self.assertEqual((self.workspace / "AGENTS.md").read_text(), "unauthorized\n")

    def test_finish_accepts_resolution_by_deletion_even_when_already_staged(self):
        self.conflict()
        self.git(self.workspace, "rm", "app.txt")
        pr_merge.finish_merge(self.workspace, ["app.txt"])
        self.assertFalse((self.workspace / "app.txt").exists())
        self.assertEqual(self.git(self.workspace, "status", "--porcelain"), "")

    def test_retry_rejects_different_target_merge(self):
        self.conflict()
        self.advance_main({"other.txt": "later\n"})
        with self.assertRaises(pr_merge.MergeError):
            self.prepare()
        self.assertTrue((self.workspace / ".git/MERGE_HEAD").exists())


if __name__ == "__main__":
    unittest.main()
