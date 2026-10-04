import unittest

from network_policy import issue_network, network_command


class NetworkPolicyTest(unittest.TestCase):
    def test_explicit_form_and_defaults(self):
        self.assertTrue(issue_network("one sentence"))
        self.assertFalse(issue_network("one sentence", False))
        self.assertTrue(issue_network("### 联网权限\n\n允许\n", False))
        self.assertFalse(issue_network("### 联网权限\n\n禁止\n"))
        self.assertFalse(issue_network("### 联网权限\n\n沿用仓库默认\n", False))
        for body in (
            "```\n### 联网权限\n允许\n```",
            "> ### 联网权限\n> 允许",
            "<!--\n### 联网权限\n允许\n-->",
        ):
            self.assertFalse(issue_network(body, False))
        for body in (
            "### 联网权限\n允许\n禁止",
            "### 联网权限\n允许\n### 联网权限\n允许",
            "### 联网权限\n其他",
        ):
            with self.assertRaises(ValueError):
                issue_network(body)

    def test_only_owner_exact_command_not_issue_text(self):
        event = {
            "repository": {"full_name": "owner/repo"},
            "sender": {"login": "owner"},
            "comment": {"user": {"login": "owner"}, "body": "/network allow"},
            "action": "created",
            "issue": {"number": 1},
        }
        self.assertEqual(network_command(event, "owner/repo"), "allow")
        event["comment"]["body"] = "please /network allow"
        with self.assertRaises(ValueError):
            network_command(event, "owner/repo")
        event["comment"]["body"] = "/network allow"
        event["sender"]["login"] = "outsider"
        with self.assertRaises(PermissionError):
            network_command(event, "owner/repo")
