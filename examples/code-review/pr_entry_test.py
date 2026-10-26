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
