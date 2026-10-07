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
        for marker in (
            "<!-- agent-platform:run -->",
            "<!-- agent-platform-hook:receipt -->",
        ):
            self.event["comment"]["body"] = "自动回执 " + marker
            self.assertIsNone(event_input(self.config, self.env, self.event))
        self.event["comment"]["body"] = "请修改"
        self.event["comment"]["user"]["login"] = "stranger"
        with self.assertRaises(ValueError):
            event_input(self.config, self.env, self.event)


class WorkspaceTest(unittest.TestCase):
    def test_failed_clone_can_retry_with_configured_transport(self):
        import subprocess
        import tempfile
        from pathlib import Path
        from unittest.mock import patch

        from github_entry import prepare_workspace

        with tempfile.TemporaryDirectory() as temporary:
            config = {
                "repository": "owner/repo",
                "workspace_root": temporary,
                "git_proxy": "http://localhost:12345",
            }
            calls = []

            def run(argv, **kwargs):
                if "clone" not in argv:
                    return
                calls.append(argv)
                if len(calls) == 1:
                    raise subprocess.TimeoutExpired(argv, 180)
                Path(argv[-1]).mkdir()

            with patch("github_entry.subprocess.run", side_effect=run):
                with self.assertRaises(subprocess.TimeoutExpired):
                    prepare_workspace(config, 123)
                self.assertEqual(list(Path(temporary).iterdir()), [])
                result = prepare_workspace(config, 123)
            self.assertTrue(result.is_dir())
            self.assertIn("http.proxy=http://localhost:12345", calls[-1])

    def test_transport_reconfiguration_uses_real_local_checkout_only(self):
        import subprocess
        import tempfile
        from pathlib import Path

        from github_entry import prepare_workspace

        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary) / "github-issue-123"
            subprocess.run(
                ["git", "init", str(directory)], check=True, capture_output=True
            )
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(directory),
                    "remote",
                    "add",
                    "origin",
                    "https://github.com/owner/repo.git",
                ],
                check=True,
            )
            config = {
                "repository": "owner/repo",
                "workspace_root": temporary,
                "git_proxy": "http://localhost:12345",
            }
            self.assertEqual(prepare_workspace(config, 123), directory.resolve())
            output = subprocess.check_output(
                [
                    "git",
                    "-C",
                    str(directory),
                    "config",
                    "--local",
                    "--get",
                    "http.proxy",
                ],
                text=True,
            )
            self.assertEqual(output.strip(), config["git_proxy"])
            config["git_proxy"] = ""
            prepare_workspace(config, 123)
            self.assertEqual(
                subprocess.check_output(
                    [
                        "git",
                        "-C",
                        str(directory),
                        "config",
                        "--local",
                        "--get",
                        "http.proxy",
                    ],
                    text=True,
                ).strip(),
                "",
            )
            config["git_proxy"] = "http://private:secret@localhost:12345"
            with self.assertRaises(ValueError):
                prepare_workspace(config, 123)


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

    def test_rejected_comment_uses_fresh_run_state_without_replaying(self):
        import tempfile
        from pathlib import Path
        from unittest.mock import Mock, patch

        from github_entry import APIError, forward

        cases = [
            ({"status": "completed"}, "新 Issue", "再在 Actions"),
            ({"status": "stopped"}, "继续/回退", "新 Issue"),
            ({"status": "running"}, "等待交接", "不能继续或回退"),
            (ConnectionError("unavailable"), "状态未确认", "继续/回退"),
        ]
        for fresh, expected, forbidden in cases:
            with self.subTest(fresh=fresh), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                config = {
                    "repository": "owner/repo",
                    "workspace_root": str(root / "work"),
                    "state_root": str(root / "state"),
                    "base_url": "http://localhost",
                }
                issue = {"number": 7, "id": 123, "user": {"login": "owner"}}
                comment = {
                    "issue_url": "https://api.github.com/repos/owner/repo/issues/7",
                    "user": {"login": "owner", "type": "User"},
                    "body": "follow-up",
                }
                client = Mock()
                client.workflow_by_request.return_value = {
                    "id": "run",
                    "status": "running",
                }
                rejection = APIError(409, "input closed")
                client.workflow_message.side_effect = rejection
                if isinstance(fresh, Exception):
                    client.workflow_run.side_effect = fresh
                else:
                    client.workflow_run.return_value = {"id": "run", **fresh}
                with (
                    patch(
                        "github_entry.github",
                        side_effect=lambda path: (
                            comment if "comments/" in path else issue
                        ),
                    ),
                    patch("github_entry.notify") as notice,
                ):
                    with self.assertRaises(APIError) as caught:
                        forward(config, client, 7, 11)
                    self.assertIs(caught.exception, rejection)
                    client.workflow_run.assert_called_once_with("run")
                    client.workflow_message.assert_called_once_with(
                        "run", "follow-up", request_id="github:owner/repo:comment:11"
                    )
                    client.start_workflow.assert_not_called()
                    message = notice.call_args.args[3]
                    self.assertIn(expected, message)
                    self.assertNotIn(forbidden, message)
                    self.assertIn("/workflow-runs/run", message)
                    self.assertTrue((root / "state/comment-11.json").exists())
                    self.assertFalse((root / "state/comment-11-receipt.json").exists())


class MaterialEntryTest(unittest.TestCase):
    def test_explicit_material_is_checked_and_old_issues_are_compatible(self):
        import json

        from github_entry import issue_parameters

        descriptor = {
            "url": "https://api.github.com/repos/owner/repo/releases/assets/123",
            "sha256": "a" * 64,
            "version": "v1",
        }
        issue = {
            "number": 7,
            "body": "Requirements\n```agent-platform-material\n"
            + json.dumps(descriptor)
            + "\n```",
        }
        params = issue_parameters(issue, "owner/repo")
        self.assertEqual(json.loads(params["material"]), descriptor)
        self.assertEqual(params["issue_number"], "7")
        self.assertEqual(
            issue_parameters({"number": 7, "body": "ordinary task"}, "owner/repo"),
            {"issue_number": "7"},
        )
        for body in [
            issue["body"] + "\n" + issue["body"],
            "```agent-platform-material\n{}",
            "```agent-platform-material\n{}\n```",
        ]:
            with self.subTest(body=body), self.assertRaises(ValueError):
                issue_parameters({"number": 7, "body": body}, "owner/repo")

    def test_lost_response_freezes_material_with_original_issue(self):
        import json
        import tempfile
        from pathlib import Path
        from unittest.mock import Mock, patch

        from github_entry import APIError, forward

        descriptor = {
            "url": "https://api.github.com/repos/owner/repo/releases/assets/123",
            "sha256": "a" * 64,
            "version": "v1",
        }
        issue = {
            "number": 7,
            "id": 123,
            "title": "task",
            "body": "```agent-platform-material\n" + json.dumps(descriptor) + "\n```",
            "user": {"login": "owner"},
            "state": "open",
        }
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            client = Mock()
            client.workflow_by_request.side_effect = APIError(404, "missing")
            client.start_workflow.side_effect = ConnectionError("lost response")
            config = {
                "repository": "owner/repo",
                "workflow_id": "graph",
                "state_root": str(root / "state"),
                "workspace_root": str(root / "work"),
                "base_url": "http://localhost",
            }
            with (
                patch("github_entry.github", return_value=issue),
                patch("github_entry.notify"),
                patch(
                    "github_entry.prepare_workspace",
                    return_value=root / "work" / "task",
                ),
            ):
                with self.assertRaises(ConnectionError):
                    forward(config, client, 7)
                original = client.start_workflow.call_args
                issue["body"] = "Edited without attachment"
                with self.assertRaises(ConnectionError):
                    forward(config, client, 7)
                self.assertEqual(original, client.start_workflow.call_args)
                self.assertEqual(
                    json.loads(original.kwargs["parameters"]["material"]), descriptor
                )


if __name__ == "__main__":
    unittest.main()
