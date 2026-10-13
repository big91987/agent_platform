"""Entry boundaries; these are not native Agent or GitHub acceptance tests."""

import copy
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


if __name__ == "__main__":
    unittest.main()
