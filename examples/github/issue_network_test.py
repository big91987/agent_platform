import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

from agent_platform_client import APIError
from issue_network import apply_once, main


class ApplyNetworkTest(unittest.TestCase):
    def test_owner_entry_works_with_non_admin_agent_summaries(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "issues").mkdir()
            task_path = root / "issues/100.json"
            task_path.write_text(
                json.dumps(
                    {
                        "active_stage": "design",
                        "stages": {"design": {"conversation_id": "existing"}},
                    }
                )
            )
            config = root / "config.json"
            config.write_text(
                json.dumps(
                    {
                        "base_url": "http://platform",
                        "token": "fixture",
                        "pipeline": {
                            "repository": "owner/repo",
                            "registry": str(root),
                            "agents": {"design": "agent"},
                        },
                    }
                )
            )
            event = root / "event.json"
            event.write_text(
                json.dumps(
                    {
                        "action": "created",
                        "repository": {"full_name": "owner/repo"},
                        "sender": {"login": "owner"},
                        "issue": {"number": 100},
                        "comment": {
                            "id": 42,
                            "user": {"login": "owner"},
                            "body": "/network allow",
                        },
                    }
                )
            )
            client = Mock()
            # This is the real public API shape: absence of private Agent config
            # must not be interpreted as a denied grant. The update API enforces it.
            client.agents.return_value = [
                {"id": "agent", "name": "Design", "enabled": True, "executor": "codex"}
            ]
            client.conversation.return_value = {
                "conversation": {"status": "idle"},
                "execution_permissions": {"network_access": False},
            }
            notices = []
            with (
                patch("sys.argv", ["issue_network.py", "--config", str(config)]),
                patch.dict(
                    os.environ,
                    {
                        "GITHUB_EVENT_PATH": str(event),
                        "GITHUB_ACTOR": "owner",
                        "GITHUB_TRIGGERING_ACTOR": "owner",
                    },
                ),
                patch("issue_network.Client", return_value=client),
                patch(
                    "issue_network.comment_once",
                    side_effect=lambda *args: notices.append(args),
                ),
            ):
                main()
            saved = json.loads(task_path.read_text())
            self.assertIs(saved["network_access"], True)
            self.assertEqual(saved["stages"]["design"]["conversation_id"], "existing")
            client.network_access.assert_called_once_with("existing", True)
            self.assertTrue(any(":applied -->" in notice[2] for notice in notices))

    def test_running_is_deferred_and_idle_keeps_same_session(self):
        client = Mock()
        task = {"stages": {"design": {"conversation_id": "existing"}}}
        client.conversation.return_value = {
            "conversation": {"status": "running"},
            "execution_permissions": {"network_access": False},
        }
        self.assertEqual(apply_once(client, task, True), ["existing"])
        client.network_access.assert_not_called()
        client.conversation.return_value["conversation"]["status"] = "idle"
        self.assertEqual(apply_once(client, task, True), [])
        client.network_access.assert_called_once_with("existing", True)
        client.invoke.assert_not_called()

    def test_deny_also_closes_existing_elevation_path(self):
        client = Mock()
        client.conversation.return_value = {
            "conversation": {"status": "idle"},
            "execution_permissions": {"network_access": False, "allow_elevation": True},
        }
        apply_once(client, {"stages": {"qa": {"conversation_id": "c"}}}, False)
        client.network_access.assert_called_once_with("c", False)

    def test_claim_race_retries_but_forbidden_is_not_hidden(self):
        client = Mock()
        task = {"stages": {"qa": {"conversation_id": "c"}}}
        client.conversation.return_value = {
            "conversation": {"status": "idle"},
            "execution_permissions": {"network_access": False},
        }
        client.network_access.side_effect = APIError(409, "busy")
        self.assertEqual(apply_once(client, task, True), ["c"])
        client.network_access.side_effect = APIError(403, "forbidden")
        with self.assertRaises(APIError):
            apply_once(client, task, True)
