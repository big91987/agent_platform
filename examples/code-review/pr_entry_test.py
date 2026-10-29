import copy
import unittest

from pr_entry import request, submit


class PullReviewEntryTest(unittest.TestCase):
    config = {
        "repository": "owner/project",
        "review_workflow_ids": {"cli": "cli-id", "delegate": "delegate-id"},
    }
    pr = {
        "number": 12,
        "state": "open",
        "user": {"login": "owner"},
        "head": {"sha": "a" * 40, "repo": {"full_name": "owner/project"}},
        "base": {"sha": "b" * 40, "repo": {"full_name": "owner/project"}},
        "title": "Review me",
        "html_url": "https://github.com/owner/project/pull/12",
    }

    def test_exact_base_head_and_engine_are_deduplication_identity(self):
        original, parameters = request(self.config, self.pr, "cli")
        self.assertEqual(parameters["repository_commit"], "a" * 40)
        self.assertEqual(parameters["review_base_commit"], "b" * 40)
        changed = copy.deepcopy(self.pr)
        changed["head"]["sha"] = "c" * 40
        self.assertNotEqual(original, request(self.config, changed, "cli")[0])
        changed = copy.deepcopy(self.pr)
        changed["base"]["sha"] = "c" * 40
        self.assertNotEqual(original, request(self.config, changed, "cli")[0])
        self.assertNotEqual(original, request(self.config, self.pr, "delegate")[0])

    def test_fork_and_nonowner_and_closed_are_rejected(self):
        for mutate in (
            lambda p: p["head"]["repo"].update(full_name="other/fork"),
            lambda p: p["user"].update(login="other"),
            lambda p: p.update(state="closed"),
        ):
            changed = copy.deepcopy(self.pr)
            mutate(changed)
            with self.assertRaises(ValueError):
                request(self.config, changed, "cli")

    def test_accepted_run_is_reused_without_starting(self):
        from unittest.mock import Mock, patch

        client = Mock()
        _, parameters = request(self.config, self.pr, "cli")
        original = {"id": "original", "workflow_id": "cli-id", "parameters": parameters}
        with patch("pr_entry.accepted_run", return_value=original):
            self.assertEqual(submit(self.config, client, self.pr, "cli"), original)
        client.start_workflow.assert_not_called()
        original["parameters"] = {}
        with (
            patch("pr_entry.accepted_run", return_value=original),
            self.assertRaises(ValueError),
        ):
            submit(self.config, client, self.pr, "cli")

    def test_permission_failure_reports_http_status_without_raw_response(self):
        import contextlib
        import io
        import subprocess
        from unittest.mock import patch

        from pr_entry import run

        failure = subprocess.CalledProcessError(
            1,
            ["gh", "api", "repos/owner/project/pulls/12"],
            stderr="private diagnostic\ngh: Resource not accessible by integration (HTTP 403)\n",
        )
        out = io.StringIO()
        with (
            patch("pr_entry.main", side_effect=failure),
            contextlib.redirect_stderr(out),
        ):
            self.assertEqual(run(), 2)
        self.assertIn("HTTP 403", out.getvalue())
        self.assertNotIn("private diagnostic", out.getvalue())

    def test_closed_pr_skips_only_queued_automatic_event_without_status_or_new_work(
        self,
    ):
        import contextlib
        import io
        import json
        import tempfile
        from pathlib import Path
        from unittest.mock import Mock, patch

        from pr_entry import main

        closed = dict(self.pr, state="closed")
        with tempfile.TemporaryDirectory() as folder:
            config = Path(folder) / "config.json"
            token = Path(folder) / "token"
            token.write_text("test-only")
            config.write_text(
                json.dumps(
                    dict(
                        self.config,
                        base_url="http://platform.invalid",
                        token_file=str(token),
                    )
                )
            )
            for event in ("pull_request_target", "workflow_dispatch"):
                output = io.StringIO()
                github = Mock(return_value=closed)
                with (
                    self.subTest(event=event),
                    patch.dict(
                        "os.environ",
                        {
                            "GITHUB_REPOSITORY": "owner/project",
                            "GITHUB_ACTOR": "owner",
                            "GITHUB_TRIGGERING_ACTOR": "owner",
                            "GITHUB_EVENT_NAME": event,
                        },
                    ),
                    patch(
                        "sys.argv",
                        [
                            "pr_entry",
                            "--config",
                            str(config),
                            "--pr",
                            "12",
                            "--engine",
                            "cli",
                        ],
                    ),
                    patch("pr_entry.configured_github_call", return_value=github),
                    patch("pr_entry.Client") as client,
                    contextlib.redirect_stdout(output),
                ):
                    if event == "pull_request_target":
                        self.assertEqual(main(), 0)
                        self.assertEqual(
                            json.loads(output.getvalue())["status"], "skipped"
                        )
                        client.assert_not_called()
                    else:
                        with self.assertRaisesRegex(ValueError, "Only an open"):
                            main()
                        client.return_value.start_workflow.assert_not_called()
                    github.assert_called_once_with("repos/owner/project/pulls/12")
