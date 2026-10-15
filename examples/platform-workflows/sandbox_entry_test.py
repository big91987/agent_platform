"""Entry boundaries; these are not native Agent or GitHub acceptance tests."""

import copy
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

from agent_platform_client import APIError
from github_entry import event_input
from github_entry import forward as legacy_forward
from sandbox_entry import forward


class SandboxEntryTest(unittest.TestCase):
    def setUp(self):
        self.config = {
            "repository": "owner/repo",
            "workflow_id": "graph",
            "issue_label": "harness-opensandbox",
            "base_url": "http://platform",
        }
        self.issue = {
            "number": 7,
            "id": 123,
            "state": "open",
            "user": {"login": "owner"},
            "title": "【Harness验证】验证角色交接与停止接续",
            "body": "Verify the agreed task",
            "labels": [{"name": "harness-opensandbox"}],
        }
        self.run = {
            "id": "run",
            "workflow_id": "graph",
            "status": "running",
            "definition": {"environment": "opensandbox"},
            "workspace_path": "/workspace",
        }
        self.comments = []
        self.paths = []
        self.client = Mock()
        self.client.workflow_by_request.side_effect = APIError(404, "missing")
        self.client.start_workflow.return_value = copy.deepcopy(self.run)

    def github(self, path, body=None, **kwargs):
        self.paths.append(path)
        if path.endswith("/comments?per_page=100"):
            return self.comments
        if path.endswith("/comments"):
            self.comments.append(
                {"body": body["body"], "user": {"login": "github-actions[bot]"}}
            )
            return {}
        if "/commits/" in path:
            return {"sha": "a" * 40}
        if path.endswith("/issues/7"):
            return copy.deepcopy(self.issue)
        if path.endswith("/issues/comments/44"):
            return {
                "issue_url": "https://api.github.com/repos/owner/repo/issues/7",
                "user": {"login": "owner", "type": "User"},
                "body": "check the retained result",
            }
        raise AssertionError(path)

    def test_configured_proxy_scopes_all_github_calls_without_changing_parent(self):
        self.config["git_proxy"] = "http://proxy.example:7890"
        calls = []

        def request(path, body=None, **kwargs):
            calls.append(kwargs.get("env", {}))
            return self.github(path, body, **kwargs)

        parent = dict(os.environ)
        with patch("sandbox_entry.github", side_effect=request):
            forward(self.config, self.client, 7)
        self.assertGreaterEqual(len(calls), 4)
        self.assertTrue(
            all(x.get("HTTPS_PROXY") == self.config["git_proxy"] for x in calls)
        )
        self.assertEqual(dict(os.environ), parent)
        self.client.start_workflow.assert_called_once()

    def test_entry_cli_reports_safe_transport_classification(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            token = root / "caller.token"
            token.write_text("fixture-caller-value")
            config = {**self.config, "token_file": str(token)}
            (root / "config.json").write_text(json.dumps(config))
            event = {
                "repository": {"full_name": "owner/repo"},
                "sender": {"login": "owner"},
                "issue": self.issue,
                "action": "opened",
            }
            (root / "event.json").write_text(json.dumps(event))
            fake = root / "gh"
            fake.write_text(
                "#!"
                + sys.executable
                + "\nimport sys\nsys.stderr.write('gh: private-response (HTTP 403)\\n')\nsys.exit(1)\n"
            )
            fake.chmod(0o700)
            result = subprocess.run(
                [
                    sys.executable,
                    "-I",
                    str(Path(__file__).with_name("run_entry.py")),
                    "sandbox",
                    "--config",
                    str(root / "config.json"),
                ],
                env={
                    **os.environ,
                    "PATH": str(root) + os.pathsep + os.environ.get("PATH", ""),
                    "GITHUB_EVENT_PATH": str(root / "event.json"),
                    "GITHUB_EVENT_NAME": "issues",
                    "GITHUB_REPOSITORY": "owner/repo",
                    "GITHUB_ACTOR": "owner",
                    "GITHUB_TRIGGERING_ACTOR": "owner",
                },
                capture_output=True,
                text=True,
                timeout=10,
            )
            self.assertEqual(result.returncode, 1)
            self.assertIn("GitHub API failed: HTTP 403", result.stderr)
            self.assertNotIn("private-response", result.stderr)
            self.assertNotIn(str(root), result.stderr)

    def test_new_entry_freezes_commit_and_never_prepares_host_workspace(self):
        with patch(
            "github_entry.prepare_workspace", side_effect=AssertionError("host clone")
        ):
            first = forward(self.config, self.client, 7, github_call=self.github)
            args = self.client.start_workflow.call_args
            self.assertEqual(args.kwargs["expected_environment"], "opensandbox")
            self.assertEqual(args.kwargs["workspace_path"], "")
            self.assertEqual(args.kwargs["parameters"]["repository_commit"], "a" * 40)
            self.client.workflow_by_request.side_effect = None
            self.client.workflow_by_request.return_value = first
            self.issue["body"] = "edited after acceptance"
            forward(self.config, self.client, 7, github_call=self.github)
        self.client.start_workflow.assert_called_once()
        self.assertEqual(len(self.comments), 1)

    def test_classified_product_task_preserves_input_and_reuses_same_run(self):
        for category in (
            "产品功能",
            "产品修复",
            "产品测试",
            "Harness验证",
            "Harness修复",
            "交付维护",
        ):
            with self.subTest(category=category):
                self.setUp()
                self.issue["title"] = f"【{category}】支持同模型上游低延迟优先路由"
                first = forward(self.config, self.client, 7, github_call=self.github)
                args = self.client.start_workflow.call_args
                self.assertEqual(
                    args.args[1], self.issue["title"] + "\n\n" + self.issue["body"]
                )
                self.assertEqual(
                    args.kwargs["request_id"], "github:owner/repo:issue:123"
                )
                self.client.workflow_by_request.side_effect = None
                self.client.workflow_by_request.return_value = first
                forward(self.config, self.client, 7, github_call=self.github)
                self.client.start_workflow.assert_called_once()

    def test_invalid_title_or_author_never_resolves_or_starts_task(self):
        for title, owner in (
            ("没有类别的任务", "owner"),
            ("【未知】实现路由", "owner"),
            ("【产品功能】", "owner"),
            ("【产品功能】   ", "owner"),
            ("【产品功能】实现路由", "other"),
        ):
            with self.subTest(title=title, owner=owner):
                self.setUp()
                self.issue.update(title=title, user={"login": owner})
                with self.assertRaises(ValueError):
                    forward(self.config, self.client, 7, github_call=self.github)
                self.client.workflow_by_request.assert_not_called()
                self.client.start_workflow.assert_not_called()

    def test_existing_spaced_harness_title_keeps_comment_continuity(self):
        self.issue["title"] = "【Harness验证】 验证交接" + "与恢复" * 65
        self.client.workflow_by_request.side_effect = None
        self.client.workflow_by_request.return_value = self.run
        self.client.workflow_message.return_value = {
            "conversation_url": "http://platform/conversations/retained"
        }
        forward(self.config, self.client, 7, 44, github_call=self.github)
        self.client.start_workflow.assert_not_called()
        self.client.workflow_message.assert_called_once_with(
            "run",
            "check the retained result",
            request_id="github:owner/repo:issue:123:comment:44",
        )

    def test_unrelated_labels_and_other_pipeline_runs_do_not_start(self):
        self.issue["labels"] = []
        with self.assertRaises(ValueError):
            forward(self.config, self.client, 7, github_call=self.github)
        self.client.workflow_by_request.assert_not_called()
        self.issue["labels"] = [{"name": "harness-opensandbox"}]
        old = copy.deepcopy(self.run)
        old["definition"]["environment"] = ""
        self.client.workflow_by_request.side_effect = None
        self.client.workflow_by_request.return_value = old
        with self.assertRaises(ValueError):
            forward(self.config, self.client, 7, github_call=self.github)
        self.client.start_workflow.assert_not_called()

    def test_uncertain_start_is_not_replayed_and_later_resolution_reuses_run(self):
        self.client.start_workflow.side_effect = ConnectionError("lost response")
        with self.assertRaises(ConnectionError):
            forward(self.config, self.client, 7, github_call=self.github)
        self.client.workflow_by_request.side_effect = None
        self.client.workflow_by_request.return_value = self.run
        forward(self.config, self.client, 7, github_call=self.github)
        self.client.start_workflow.assert_called_once()

    def test_comment_rejection_does_not_revive_or_create_a_run(self):
        self.client.workflow_by_request.side_effect = None
        self.client.workflow_by_request.return_value = self.run
        self.client.workflow_message.side_effect = APIError(409, "inspect current node")
        with self.assertRaises(APIError):
            forward(self.config, self.client, 7, 44, github_call=self.github)
        self.client.start_workflow.assert_not_called()

    def test_new_label_is_mutually_exclusive_even_for_legacy_manual_dispatch(self):
        env = {
            "GITHUB_REPOSITORY": "owner/repo",
            "GITHUB_ACTOR": "owner",
            "GITHUB_TRIGGERING_ACTOR": "owner",
            "GITHUB_EVENT_NAME": "issues",
        }
        event = {
            "repository": {"full_name": "owner/repo"},
            "sender": {"login": "owner"},
            "issue": self.issue,
            "action": "labeled",
            "label": {"name": "harness-opensandbox"},
        }
        self.assertEqual(event_input(self.config, env, event), (7, 0))
        self.assertIsNone(event_input({"repository": "owner/repo"}, env, event))
        with tempfile.TemporaryDirectory() as tmp:
            old = {
                "repository": "owner/repo",
                "state_root": tmp,
                "workspace_root": str(Path(tmp) / "workspace"),
            }
            with (
                patch("github_entry.github", side_effect=self.github),
                self.assertRaises(ValueError),
            ):
                legacy_forward(old, self.client, 7)
        self.client.workflow_by_request.assert_not_called()

    def test_legacy_configuration_cannot_clear_the_reserved_sandbox_route(self):
        env = {
            "GITHUB_REPOSITORY": "owner/repo",
            "GITHUB_ACTOR": "owner",
            "GITHUB_TRIGGERING_ACTOR": "owner",
            "GITHUB_EVENT_NAME": "issues",
        }
        event = {
            "repository": {"full_name": "owner/repo"},
            "sender": {"login": "owner"},
            "issue": self.issue,
            "action": "opened",
        }
        with tempfile.TemporaryDirectory() as tmp:
            old = {
                "repository": "owner/repo",
                "state_root": tmp,
                "workspace_root": str(Path(tmp) / "workspace"),
                "excluded_issue_labels": [],
            }
            self.assertIsNone(event_input(old, env, event))
            self.issue["labels"].append({"name": "development"})
            self.assertIsNone(
                event_input({**old, "issue_label": "development"}, env, event)
            )
            with patch("github_entry.github", side_effect=self.github):
                with self.assertRaisesRegex(ValueError, "another Pipeline"):
                    legacy_forward(old, self.client, 7)
        self.client.workflow_by_request.assert_not_called()


if __name__ == "__main__":
    unittest.main()
