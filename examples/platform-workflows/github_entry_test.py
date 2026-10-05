"""Boundary tests for event forwarding; not real GitHub/Agent acceptance."""

import copy
import unittest

from github_entry import event_input


class EventInputTest(unittest.TestCase):
    def setUp(self):
        self.config = {"repository": "owner/repo"}
        self.env = {
            "GITHUB_REPOSITORY": "owner/repo",
            "GITHUB_ACTOR": "owner",
            "GITHUB_TRIGGERING_ACTOR": "owner",
            "GITHUB_EVENT_NAME": "issues",
        }
        self.event = {
            "action": "opened",
            "repository": {"full_name": "owner/repo"},
            "sender": {"login": "owner"},
            "issue": {"number": 7, "id": 123, "user": {"login": "owner"}},
        }

    def test_open_issue_is_the_entry_not_a_new_issue(self):
        self.assertEqual(event_input(self.config, self.env, self.event), (7, 0))

    def test_rejects_wrong_repository_actor_and_pr(self):
        for field, value in [
            ("GITHUB_REPOSITORY", "evil/repo"),
            ("GITHUB_ACTOR", "stranger"),
            ("GITHUB_TRIGGERING_ACTOR", "stranger"),
        ]:
            env = {**self.env, field: value}
            with self.assertRaises(ValueError):
                event_input(self.config, env, self.event)
        event = copy.deepcopy(self.event)
        event["issue"]["pull_request"] = {}
        with self.assertRaises(ValueError):
            event_input(self.config, self.env, event)

    def test_comment_requires_human_owner_and_ignores_platform_output(self):
        self.env["GITHUB_EVENT_NAME"] = "issue_comment"
        self.event.update(
            action="created",
            comment={
                "id": 11,
                "body": "实际应该这样",
                "user": {"login": "owner", "type": "User"},
            },
        )
        self.assertEqual(event_input(self.config, self.env, self.event), (7, 11))
        self.event["comment"]["body"] = "自动回执 <!-- agent-platform:run -->"
        self.assertIsNone(event_input(self.config, self.env, self.event))
        self.event["comment"]["body"] = "请修改"
        self.event["comment"]["user"]["login"] = "stranger"
        with self.assertRaises(ValueError):
            event_input(self.config, self.env, self.event)


class ForwardTest(unittest.TestCase):
    def test_lost_start_response_recovers_original_run_without_second_clone(self):
        import tempfile
        from pathlib import Path
        from unittest.mock import patch

        from agent_platform_client import APIError
        from github_entry import forward

        class Client:
            result = None
            starts = 0

            def workflow_by_request(self, key):
                if self.result is None:
                    raise APIError(404, "not found")
                return self.result

            def start_workflow(self, *args, **kwargs):
                self.starts += 1
                self.result = {"id": "original", "status": "running"}
                raise ConnectionError("response lost after acceptance")

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            config = {
                "repository": "owner/repo",
                "workspace_root": str(root / "work"),
                "state_root": str(root / "state"),
                "workflow_id": "graph",
                "base_url": "http://localhost",
            }
            issue = {
                "number": 7,
                "id": 123,
                "user": {"login": "owner"},
                "state": "open",
                "title": "original",
                "body": "task",
            }
            client = Client()
            with (
                patch("github_entry.github", return_value=issue),
                patch(
                    "github_entry.prepare_workspace",
                    return_value=root / "work" / "task",
                ) as clone,
                patch("github_entry.notify"),
            ):
                with self.assertRaises(ConnectionError):
                    forward(config, client, 7)
                issue["title"] = "edited after acceptance"
                result = forward(config, client, 7)
                self.assertEqual(result["id"], "original")
                self.assertEqual(client.starts, 1)
                self.assertEqual(clone.call_count, 1)

    def test_comment_retries_preserve_original_text_and_event_key(self):
        import tempfile
        from pathlib import Path
        from unittest.mock import patch

        from github_entry import forward

        class Client:
            calls = []

            def workflow_by_request(self, key):
                return {"id": "run", "status": "waiting"}

            def workflow_message(self, run, message, *, request_id):
                self.calls.append((run, message, request_id))
                if len(self.calls) == 1:
                    raise ConnectionError("response lost")
                return {
                    "conversation_url": "http://localhost/conversations/c",
                    "message_id": 1,
                }

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            config = {
                "repository": "owner/repo",
                "workspace_root": str(root / "work"),
                "state_root": str(root / "state"),
                "workflow_id": "graph",
                "base_url": "http://localhost",
            }
            issue = {"number": 7, "id": 123, "user": {"login": "owner"}}
            comment = {
                "id": 11,
                "issue_url": "https://api.github.com/repos/owner/repo/issues/7",
                "user": {"login": "owner", "type": "User"},
                "body": "original feedback",
            }
            client = Client()

            def remote(path):
                return comment if "comments/" in path else issue

            with (
                patch("github_entry.github", side_effect=remote),
                patch("github_entry.notify"),
            ):
                with self.assertRaises(ConnectionError):
                    forward(config, client, 7, 11)
                comment["body"] = "edited feedback"
                forward(config, client, 7, 11)
                self.assertEqual(client.calls[0], client.calls[1])
                self.assertEqual(
                    client.calls[0],
                    ("run", "original feedback", "github:owner/repo:comment:11"),
                )


if __name__ == "__main__":
    unittest.main()
