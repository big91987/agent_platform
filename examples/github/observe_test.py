import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import observe


class ObserveTest(unittest.TestCase):
    def test_timeout_continues_same_message_then_posts_once_without_invoking(self):
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / "94.json"
            receipt = {"conversation_id": "existing", "message_id": 42}
            task = {"active_stage": "development", "stages": {"development": receipt}}
            path.write_text(json.dumps(task))
            client = Mock()
            client.wait.side_effect = [
                TimeoutError(),
                {
                    "conversation": {"status": "idle"},
                    "messages": [
                        {
                            "role": "agent",
                            "kind": "reply",
                            "parent_id": 42,
                            "content": "Verified result",
                        }
                    ],
                },
            ]
            settings = {"repository": "owner/repo"}
            with (
                patch.object(observe, "dispatch_observer") as dispatch,
                patch.object(observe, "comment_once") as comment,
            ):
                observe.wait_and_publish(
                    client, settings, path, 94, "development", 42, "http://localhost"
                )
                dispatch.assert_called_once_with(settings, 94, "development", 42)
                observe.wait_and_publish(
                    client, settings, path, 94, "development", 42, "http://localhost"
                )
                self.assertIn("Verified result", comment.call_args.args[-1])
                client.invoke.assert_not_called()
                # A replaced input must never publish an obsolete result or reschedule.
                task["stages"]["development"]["message_id"] = 43
                path.write_text(json.dumps(task))
                observe.wait_and_publish(
                    client, settings, path, 94, "development", 42, "http://localhost"
                )
                self.assertEqual(client.wait.call_count, 2)


if __name__ == "__main__":
    unittest.main()
