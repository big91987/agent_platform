"""Crash recovery of refresh operations with real local Git and fake platform I/O."""

import json
import unittest
from unittest.mock import patch

import pipeline
import pr_merge
import pr_merge_test
import pr_refresh


class Client:
    def conversation(self, conversation_id):
        return {"conversation": {"status": "idle"}}

    def workspace_access(self, conversation_id, read_only):
        pass


class RecoveryTest(unittest.TestCase):
    git = pr_merge_test.MergeTest.git
    identity = pr_merge_test.MergeTest.identity
    commit = pr_merge_test.MergeTest.commit
    advance_main = pr_merge_test.MergeTest.advance_main

    def setUp(self):
        pr_merge_test.MergeTest.setUp(self)
        self.advance_main({"main.txt": "latest\n"})
        self.task = {
            "workspace": str(self.workspace),
            "branch": "task/1",
            "stages": {"qa": {"conversation_id": "qa", "round": "old"}},
        }
        self.task_path = self.root / "task.json"
        self.settings = {"repository": "owner/repo"}
        self.pr = {"head": {"sha": self.head}}
        for target, value in (
            ("pr_refresh.current_pr", self.pr),
            ("pr_refresh.github", {"sha": self.base}),
            ("pr_refresh.status", None),
            ("pipeline.delivery_token", "fake"),
        ):
            patcher = patch(target, return_value=value)
            patcher.start()
            self.addCleanup(patcher.stop)

    def start(self, run="100"):
        return pr_refresh.start(self.settings, self.task, self.task_path, Client(), run)

    def reload(self):
        self.task = json.loads(self.task_path.read_text())

    def remote_update(self):
        self.git(self.workspace, "push", "origin", "task/1")
        web = self.root / "web"
        self.git(self.root, "clone", "-b", "task/1", str(self.remote), str(web))
        self.identity(web)
        self.git(web, "merge", "--no-edit", "origin/main")
        self.git(web, "push", "origin", "task/1")
        self.pr["head"]["sha"] = self.git(web, "rev-parse", "HEAD")

    def test_github_update_branch_fast_forwards_clean_workspace_and_starts_new_qa(self):
        self.remote_update()
        self.assertEqual(self.start(), "qa")
        self.assertEqual(
            self.git(self.workspace, "rev-parse", "HEAD"), self.pr["head"]["sha"]
        )
        self.assertEqual(
            self.task["integration"]["expected_head"], self.pr["head"]["sha"]
        )
        self.reload()
        self.assertEqual(self.start(), "qa")

    def test_retry_after_fast_forward_before_saved_round(self):
        self.remote_update()
        pipeline.save(self.task_path, self.task)
        with patch(
            "pipeline.save", side_effect=RuntimeError("interrupted before save")
        ):
            with self.assertRaises(RuntimeError):
                self.start()
        self.reload()
        self.assertEqual(self.start(), "qa")
        self.assertEqual(
            self.git(self.workspace, "rev-parse", "HEAD"), self.pr["head"]["sha"]
        )

    def test_remote_head_changed_during_sync_does_not_move_local_branch(self):
        self.remote_update()
        with patch(
            "pr_refresh.current_pr", side_effect=[self.pr, {"head": {"sha": "newer"}}]
        ):
            with self.assertRaisesRegex(ValueError, "PR changed during sync"):
                self.start()
        self.assertEqual(self.git(self.workspace, "rev-parse", "HEAD"), self.head)

    def test_remote_update_preserves_dirty_workspace_and_reports_failure(self):
        self.remote_update()
        (self.workspace / "other.txt").write_text("local edit")
        with self.assertRaises(ValueError), patch("pr_refresh.status") as status:
            try:
                self.start()
            finally:
                self.assertEqual(status.call_args.args[2], "failure")
        self.assertEqual((self.workspace / "other.txt").read_text(), "local edit")
        self.assertEqual(self.git(self.workspace, "rev-parse", "HEAD"), self.head)

    def test_remote_update_preserves_divergent_local_commit(self):
        self.remote_update()
        self.commit(self.workspace, {"local.txt": "local commit"})
        head = self.git(self.workspace, "rev-parse", "HEAD")
        with self.assertRaises(ValueError):
            self.start()
        self.assertEqual(self.git(self.workspace, "rev-parse", "HEAD"), head)

    def test_retries_after_merge_commit_before_registry_save(self):
        prepare = pr_merge.prepare_merge

        def crash(*args):
            prepare(*args)
            raise RuntimeError("process terminated after merge")

        with patch("pr_merge.prepare_merge", side_effect=crash):
            with self.assertRaises(RuntimeError):
                self.start()
        merged_head = self.git(self.workspace, "rev-parse", "HEAD")
        self.reload()
        self.assertEqual(self.start("101"), "qa")
        self.assertEqual(self.task["round"], "100")
        self.assertEqual(self.git(self.workspace, "rev-parse", "HEAD"), merged_head)

    def test_retries_after_document_commit_before_registry_save(self):
        (self.workspace / "docs").mkdir()
        (self.workspace / "docs/review.md").write_text("previous review\n")
        real_run = pipeline.run

        def crash(argv, cwd=None):
            result = real_run(argv, cwd)
            if "commit" in argv:
                raise RuntimeError("process terminated after docs commit")
            return result

        with patch("pipeline.run", side_effect=crash):
            with self.assertRaises(RuntimeError):
                self.start()
        self.reload()
        self.assertEqual(self.start(), "qa")
        self.assertEqual(
            (self.workspace / "docs/review.md").read_text(), "previous review\n"
        )

    def test_retries_after_conflict_before_registry_save(self):
        self.commit(self.workspace, {"app.txt": "branch\n"})
        self.pr["head"]["sha"] = self.git(self.workspace, "rev-parse", "HEAD")
        self.advance_main({"app.txt": "main\n"})
        with patch("pr_refresh.github", return_value={"sha": self.base}):
            prepare = pr_merge.prepare_merge

            def crash(*args):
                prepare(*args)
                raise RuntimeError("process terminated with merge open")

            with patch("pr_merge.prepare_merge", side_effect=crash):
                with self.assertRaises(RuntimeError):
                    self.start()
            self.reload()
            self.assertEqual(self.start(), "development")
        self.assertEqual(self.task["integration"]["conflicts"], ["app.txt"])
        self.assertEqual(self.git(self.workspace, "rev-parse", "MERGE_HEAD"), self.base)

    def test_saved_stage_without_invocation_is_resumed_in_original_round(self):
        self.assertEqual(self.start(), "qa")
        self.reload()
        self.assertEqual(self.start("101"), "qa")
        self.assertEqual(self.task["round"], "100")
        self.task["stages"]["qa"]["round"] = "100"
        self.assertIsNone(self.start("101"))
        self.assertEqual(self.start("100"), "qa")

    def test_refresh_does_not_restart_qa_during_or_after_rework(self):
        self.assertEqual(self.start(), "qa")
        self.task["stages"]["qa"]["round"] = "100"
        self.task["round"] = "101"
        self.task["active_stage"] = "development"
        self.assertIsNone(self.start("200"))
        self.assertIsNone(self.start("100"))
        self.task["active_stage"] = "qa"
        self.assertIsNone(self.start("200"))
        self.assertEqual(self.task["round"], "101")

    def test_preparing_rejects_unrelated_head_change(self):
        with patch("pr_merge.prepare_merge", side_effect=RuntimeError("crash")):
            with self.assertRaises(RuntimeError):
                self.start()
        self.commit(self.workspace, {"other.txt": "unexpected\n"})
        self.reload()
        with self.assertRaises(ValueError):
            self.start()

    def test_prepare_commit_failure_can_finish_merge_with_no_conflicted_paths(self):
        with patch("pr_merge._commit", side_effect=RuntimeError("commit unavailable")):
            with self.assertRaises(RuntimeError):
                self.start()
        self.reload()
        self.assertEqual(self.start(), "development")
        self.assertEqual(self.task["integration"]["conflicts"], [])
        pr_refresh.finish_conflicts(self.task, self.workspace, self.task_path)
        self.assertEqual(self.task["integration"]["state"], "qa")
        self.assertEqual(
            self.git(self.workspace, "show", "-s", "--format=%P", "HEAD"),
            f"{self.head} {self.base}",
        )

    def test_finish_recovers_commit_before_registry_save(self):
        # Allowed app conflict, inherited main configuration, and extra docs.
        self.commit(self.workspace, {"app/value.txt": "branch\n"})
        original = self.git(self.workspace, "rev-parse", "HEAD")
        self.advance_main({"app/value.txt": "main\n", "AGENTS.md": "new rules\n"})
        merged = pr_merge.prepare_merge(self.workspace, "task/1", original, self.base)
        self.task["integration"] = {
            "state": "development",
            "base_sha": self.base,
            "merge_head": original,
            "conflicts": merged["conflicts"],
        }
        pipeline.save(self.task_path, self.task)
        (self.workspace / "app/value.txt").write_text("resolved\n")
        finish = pr_merge.finish_merge

        def crash(*args):
            finish(*args)
            raise RuntimeError("crash after commit")

        with patch("pr_merge.finish_merge", side_effect=crash):
            with self.assertRaises(RuntimeError):
                pr_refresh.finish_conflicts(self.task, self.workspace, self.task_path)
        self.reload()
        head = self.git(self.workspace, "rev-parse", "HEAD")
        pr_refresh.finish_conflicts(self.task, self.workspace, self.task_path)
        self.reload()
        self.assertEqual(self.task["integration"]["state"], "qa")
        self.assertEqual(self.task["integration"]["merge_head"], head)
        pr_refresh.finish_conflicts(self.task, self.workspace, self.task_path)
        self.assertEqual(self.git(self.workspace, "rev-parse", "HEAD"), head)


if __name__ == "__main__":
    unittest.main()
