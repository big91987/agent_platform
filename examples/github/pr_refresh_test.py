import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import pipeline


class RefreshTest(unittest.TestCase):
    def test_ready_selection_uses_registered_delivery_not_pr_title(self):
        self.assertTrue(
            Path(__file__).with_name("pr_refresh.py").exists(),
            "automatic PR refresh is missing",
        )
        import pr_refresh

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            settings = {"registry": str(root), "repository": "owner/repo"}
            task = {
                "workspace": str(root / "work"),
                "branch": "codex/issue-1-platform",
                "stages": {},
            }
            pipeline.save(root / "issues/1.json", task)
            pipeline.save(
                pipeline.task_directory(settings, Path(task["workspace"]))
                / "delivery.json",
                {"pr_url": "https://github.com/owner/repo/pull/2"},
            )
            pr = {
                "number": 2,
                "state": "open",
                "draft": False,
                "html_url": "https://github.com/owner/repo/pull/2",
                "base": {"ref": "main"},
                "head": {"ref": task["branch"], "repo": {"full_name": "owner/repo"}},
            }
            self.assertEqual(pr_refresh.ready_issues(settings, [pr]), [1])
            pr["draft"] = True
            self.assertEqual(pr_refresh.ready_issues(settings, [pr]), [])
            pr["draft"] = False
            pr["head"]["repo"]["full_name"] = "fork/repo"
            self.assertEqual(pr_refresh.ready_issues(settings, [pr]), [])

    def test_final_status_never_accepts_old_head_or_old_base(self):
        self.assertTrue(
            Path(__file__).with_name("pr_refresh.py").exists(),
            "automatic PR refresh is missing",
        )
        import pr_refresh

        task = {
            "integration": {
                "base_sha": "base1",
                "published_head": "head1",
                "state": "review",
            }
        }
        self.assertTrue(pr_refresh.matches_review(task, "head1", "base1", "head1"))
        self.assertFalse(pr_refresh.matches_review(task, "head2", "base1", "head1"))
        self.assertFalse(pr_refresh.matches_review(task, "head1", "base2", "head1"))
        self.assertFalse(pr_refresh.matches_review(task, "head1", "base1", "head2"))

    def test_dispatch_targets_existing_workflow_issue_and_integration_stage(self):
        self.assertTrue(
            Path(__file__).with_name("pr_refresh.py").exists(),
            "automatic PR refresh is missing",
        )
        import pr_refresh

        with patch.object(pr_refresh, "github") as api:
            pr_refresh.dispatch({"repository": "owner/repo"}, 13, "integrate")
        self.assertEqual(
            api.call_args.args,
            (
                "repos/owner/repo/actions/workflows/agent-platform.yml/dispatches",
                {"ref": "main", "inputs": {"issue": "13", "after": "integrate"}},
            ),
        )


class CompletionTest(unittest.TestCase):
    def test_publish_dispatch_failure_can_retry_without_duplicate_agent_identity(self):
        import pr_refresh

        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "task.json"
            task = {"integration": {"state": "qa", "base_sha": "base"}}
            with (
                patch.object(pr_refresh, "github", return_value={"sha": "base"}),
                patch.object(pr_refresh, "status"),
                patch.object(
                    pr_refresh, "dispatch", side_effect=[RuntimeError("offline"), None]
                ) as send,
            ):
                with self.assertRaises(RuntimeError):
                    pr_refresh.published(
                        {"repository": "owner/repo"}, task, path, 1, "head"
                    )
                task = json.loads(path.read_text())
                self.assertEqual(task["integration"]["state"], "review")
                pr_refresh.published(
                    {"repository": "owner/repo"}, task, path, 1, "head"
                )
                self.assertEqual(send.call_args.args[1:], (1, "code_review"))
                self.assertEqual(task["integration"]["published_head"], "head")

    def test_base_moves_during_qa_or_review_requires_new_integration(self):
        import pr_refresh

        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "task.json"
            task = {"integration": {"state": "qa", "base_sha": "old"}}
            with (
                patch.object(pr_refresh, "github", return_value={"sha": "new"}),
                patch.object(pr_refresh, "status") as status,
                patch.object(pr_refresh, "dispatch") as send,
                patch.object(
                    pr_refresh, "current_pr", return_value={"head": {"sha": "head"}}
                ),
            ):
                pr_refresh.published(
                    {"repository": "owner/repo"}, task, path, 1, "head"
                )
                self.assertEqual(send.call_args.args[1:], (1, "integrate"))
                task["integration"]["state"] = "review"
                pr_refresh.reviewed({"repository": "owner/repo"}, task, path, 1, "head")
                self.assertEqual(send.call_args.args[1:], (1, "integrate"))
                self.assertTrue(
                    all(call.args[2] == "pending" for call in status.call_args_list)
                )
                self.assertEqual(
                    json.loads(path.read_text())["integration"]["state"], "superseded"
                )

    def test_verified_review_is_completed_once_without_merge(self):
        import pr_refresh

        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "task.json"
            task = {
                "integration": {
                    "state": "review",
                    "base_sha": "base",
                    "published_head": "head",
                }
            }
            with (
                patch.object(pr_refresh, "github", return_value={"sha": "base"}) as api,
                patch.object(pr_refresh, "status") as status,
                patch.object(pr_refresh, "dispatch") as send,
                patch.object(
                    pr_refresh, "current_pr", return_value={"head": {"sha": "head"}}
                ),
            ):
                pr_refresh.reviewed({"repository": "owner/repo"}, task, path, 1, "head")
                pr_refresh.reviewed({"repository": "owner/repo"}, task, path, 1, "head")
                status.assert_called_once()
                self.assertEqual(status.call_args.args[2], "success")
                send.assert_not_called()
                self.assertTrue(
                    all("merge" not in call.args[0] for call in api.call_args_list)
                )


class MaintenanceRoutingTest(unittest.TestCase):
    def test_unregistered_pr_gets_actionable_failure_without_agent_dispatch(self):
        import pr_refresh

        with tempfile.TemporaryDirectory() as tmp:
            settings = {"registry": tmp, "repository": "owner/repo"}
            pr = {
                "number": 99,
                "html_url": "https://github.com/owner/repo/pull/99",
                "state": "open",
                "draft": False,
                "base": {"ref": "main"},
                "head": {
                    "sha": "head",
                    "ref": "maintenance",
                    "repo": {"full_name": "owner/repo"},
                },
            }
            with (
                patch.object(pr_refresh, "github", return_value={"sha": "base"}),
                patch.object(pr_refresh, "status") as report,
                patch.object(pr_refresh, "dispatch") as dispatch,
            ):
                pr_refresh.refresh(settings, [pr])
                dispatch.assert_not_called()
                self.assertEqual(report.call_args.args[1:3], ("head", "failure"))
                self.assertIn("maintainer", report.call_args.args[3].lower())
