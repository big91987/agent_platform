"""Boundary tests for event forwarding; not real GitHub/Agent acceptance."""

import copy
import unittest

from github_entry import event_input


class FailureDiagnosticTest(unittest.TestCase):
    def test_github_http_failure_keeps_status_without_response_or_arguments(self):
        import subprocess

        from github_entry import failure_diagnostic

        failure = subprocess.CalledProcessError(
            1,
            ["gh", "api", "private-argument"],
            output="private-body",
            stderr="gh: private-token (HTTP 403)",
        )
        message = failure_diagnostic(failure)
        self.assertIn("HTTP 403", message)
        self.assertNotIn("private", message)
        failure.stderr = "gh: rejected value contains HTTP 200 (HTTP 403)"
        self.assertIn("HTTP 403", failure_diagnostic(failure))
        self.assertNotIn("HTTP 200", failure_diagnostic(failure))

    def test_transport_failure_does_not_guess_http_or_expose_endpoint(self):
        import subprocess

        from github_entry import failure_diagnostic

        failure = subprocess.CalledProcessError(
            1,
            ["gh", "api", "private-argument"],
            stderr="proxyconnect tcp: dial tcp private-endpoint: connect: connection refused",
        )
        self.assertIn("transport", failure_diagnostic(failure))
        failure.stderr = "unrecognized private details"
        self.assertIn("unknown", failure_diagnostic(failure))
        self.assertNotIn("private", failure_diagnostic(failure))


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

    def test_platform_created_issue_is_not_a_second_development_request(self):
        self.event["issue"]["body"] = (
            "验收任务\n<!-- agent-platform:" + "a" * 32 + ":1 -->"
        )
        self.assertIsNone(event_input(self.config, self.env, self.event))

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
    def test_configured_proxy_applies_only_to_github_subprocesses(self):
        import json
        import os
        import subprocess
        import tempfile
        from pathlib import Path
        from unittest.mock import Mock, patch

        from github_entry import forward

        issue = {"number": 7, "id": 123, "user": {"login": "owner"}}
        client = Mock()
        client.workflow_by_request.return_value = {
            "id": "accepted",
            "status": "completed",
        }
        calls = []
        original_proxy = os.environ.get("HTTPS_PROXY")

        def run(argv, **kwargs):
            calls.append(kwargs)
            output = (
                "[]"
                if "--paginate" in argv
                else ("{}" if "--method" in argv else json.dumps(issue))
            )
            return subprocess.CompletedProcess(argv, 0, output, "")

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = {
                "repository": "owner/repo",
                "state_root": str(root / "state"),
                "workspace_root": str(root / "work"),
                "base_url": "http://localhost",
                "git_proxy": "http://localhost:12345",
            }
            with patch("requirements.subprocess.run", side_effect=run):
                forward(config, client, 7)
            self.assertEqual(len(calls), 3)
            self.assertTrue(
                all(
                    c.get("env", {}).get("HTTPS_PROXY") == config["git_proxy"]
                    for c in calls
                )
            )
            self.assertEqual(os.environ.get("HTTPS_PROXY"), original_proxy)
            client.start_workflow.assert_not_called()

    def test_notification_failure_retains_accepted_run_link_without_restarting(self):
        import contextlib
        import io
        import json
        import os
        import subprocess
        import tempfile
        from pathlib import Path
        from unittest.mock import Mock, patch

        from github_entry import forward

        issue = {"number": 7, "id": 123, "user": {"login": "owner"}}
        client = Mock()
        client.workflow_by_request.return_value = {
            "id": "accepted",
            "status": "completed",
        }
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = {
                "repository": "owner/repo",
                "state_root": str(root / "state"),
                "workspace_root": str(root / "work"),
                "base_url": "http://localhost",
            }
            summary = root / "summary.md"
            output = io.StringIO()
            with (
                patch("github_entry.github", return_value=issue),
                patch(
                    "github_entry.notify",
                    side_effect=subprocess.CalledProcessError(1, ["gh", "api"]),
                ),
                patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(summary)}),
                contextlib.redirect_stdout(output),
                self.assertRaises(subprocess.CalledProcessError),
            ):
                forward(config, client, 7)
            self.assertIn("accepted", output.getvalue())
            self.assertIn("http://localhost/workflow-runs/accepted", output.getvalue())
            self.assertIn("/workflow-runs/accepted", summary.read_text())
            self.assertEqual(
                json.loads((root / "state/123-run.json").read_text())["id"], "accepted"
            )
            client.start_workflow.assert_not_called()

    def test_dispatch_cannot_start_from_platform_generated_issue(self):
        import tempfile
        from pathlib import Path
        from unittest.mock import Mock, patch

        from github_entry import forward

        issue = {
            "number": 7,
            "id": 123,
            "user": {"login": "owner"},
            "state": "open",
            "title": "validation",
            "body": "验收任务\n<!-- agent-platform:" + "a" * 32 + ":1 -->",
        }
        client = Mock()
        client.start_workflow.return_value = {"id": "duplicate", "status": "running"}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = {
                "repository": "owner/repo",
                "state_root": str(root / "state"),
                "workspace_root": str(root / "work"),
                "workflow_id": "graph",
                "base_url": "http://localhost",
            }
            with (
                patch("github_entry.github", return_value=issue),
                patch("github_entry.accepted_run", return_value=None),
                patch("github_entry.prepare_workspace", return_value=root / "work"),
                patch("github_entry.notify"),
            ):
                with self.assertRaisesRegex(ValueError, "existing platform Run"):
                    forward(config, client, 7)
        client.start_workflow.assert_not_called()

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
