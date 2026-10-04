import unittest
from unittest.mock import Mock

from agent_platform_client import APIError
from issue_network import apply_once


class ApplyNetworkTest(unittest.TestCase):
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
