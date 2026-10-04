import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock

import permission_notifications as notifications


class PermissionNotificationsTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / "issues").mkdir()
        (self.root / "issues/100.json").write_text(
            json.dumps({"stages": {"requirements": {"conversation_id": "c1"}}})
        )
        self.settings = {"repository": "owner/repo", "registry": str(self.root)}
        self.state = {
            "started_at": "2026-10-04T00:00:00Z",
            "conversations": {},
            "published": {},
        }
        self.client = Mock()
        self.client.conversations.return_value = [
            {"id": "c1", "status": "running", "updated_at": "t1"},
            {"id": "unrelated", "status": "running", "updated_at": "t1"},
        ]
        self.approval = {
            "id": "a1",
            "message_id": 42,
            "created_at": "2026-10-04T01:00:00Z",
            "decision": "",
            "request": {
                "platform_permission_request": True,
                "method": "item/commandExecution/requestApproval",
                "reason": "访问官网以核验荐书来源",
                "command": 'curl -H "Authorization: Bearer SECRET" https://source.invalid',
            },
        }
        self.client.conversation.return_value = {"approvals": [self.approval]}
        self.publish = Mock()

    def sync(self):
        notifications.sync_once(
            self.client, self.settings, "http://platform", self.state, self.publish
        )

    def test_pending_and_resolution_each_publish_once_including_browser_turn(self):
        self.sync()
        self.sync()
        self.assertEqual(self.publish.call_count, 1)
        args = self.publish.call_args.args
        self.assertEqual(args[:2], ("owner/repo", 100))
        self.assertIn("访问官网", args[3])
        self.assertIn("http://platform/conversations/c1", args[3])
        self.assertNotIn("SECRET", args[3])
        self.client.conversation.assert_called_with("c1")
        self.client.invoke.assert_not_called()
        self.client.decide_tool.assert_not_called()
        self.approval["decision"] = "accept"
        self.sync()
        self.sync()
        self.assertEqual(self.publish.call_count, 2)
        self.assertIn("已批准", self.publish.call_args.args[3])
        # Browser continuation has a different input ID from the original Runner receipt.
        self.approval = {**self.approval, "id": "a2", "message_id": 99, "decision": ""}
        self.client.conversation.return_value = {"approvals": [self.approval]}
        self.sync()
        self.assertEqual(self.publish.call_count, 3)
        self.state = json.loads(json.dumps(self.state))
        self.sync()
        self.assertEqual(self.publish.call_count, 3)

    def test_failure_retries_without_losing_pending_notice(self):
        self.publish.side_effect = RuntimeError("GitHub unavailable")
        with self.assertRaises(RuntimeError):
            self.sync()
        self.assertEqual(self.state["published"], {})
        self.publish.side_effect = None
        self.sync()
        self.assertEqual(self.publish.call_count, 2)

    def test_old_history_and_unrelated_mcp_are_not_published(self):
        self.approval.update(decision="accept", created_at="2026-09-01T00:00:00Z")
        self.client.conversation.return_value = {
            "approvals": [
                self.approval,
                {"id": "mcp", "request": {"serverName": "tool"}, "decision": ""},
            ]
        }
        self.sync()
        self.publish.assert_not_called()

    def test_fast_decline_and_stop_have_clear_terminal_notices(self):
        for decision, label in [("decline", "已拒绝"), ("cancel", "已取消")]:
            self.approval.update(id=decision, decision=decision)
            self.sync()
            self.assertIn(label, self.publish.call_args.args[3])
            self.assertNotIn("已批准", self.publish.call_args.args[3])

    def test_remapped_stage_keeps_pending_original_issue_notification(self):
        self.sync()
        (self.root / "issues/100.json").write_text(json.dumps({"stages": {}}))
        self.client.conversations.return_value = []
        self.approval["decision"] = "cancel"
        self.sync()
        self.assertEqual(self.publish.call_count, 2)
        self.assertEqual(self.publish.call_args.args[1], 100)

    def test_failed_conversation_does_not_block_another_issue(self):
        (self.root / "issues/101.json").write_text(
            json.dumps({"stages": {"qa": {"conversation_id": "c2"}}})
        )
        self.client.conversations.return_value.append(
            {"id": "c2", "status": "running", "updated_at": "t2"}
        )
        self.client.conversation.side_effect = lambda cid: (
            (_ for _ in ()).throw(RuntimeError("unavailable"))
            if cid == "c1"
            else {"approvals": [self.approval]}
        )
        with self.assertRaises(RuntimeError):
            self.sync()
        self.assertEqual(self.publish.call_args.args[1], 101)
        self.assertNotIn("c1:a1:requested", self.state["published"])

    def test_public_reason_redacts_credentials_paths_and_mentions(self):
        self.approval["request"]["reason"] = (
            "token=super-secret Authorization: Bearer private-value /test-workspace/private @someone <script>"
        )
        self.sync()
        body = self.publish.call_args.args[3]
        for value in [
            "super-secret",
            "private-value",
            "/test-workspace",
            "@someone",
            "<script>",
        ]:
            self.assertNotIn(value, body)


if __name__ == "__main__":
    unittest.main()
