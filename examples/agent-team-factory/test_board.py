import json
import tempfile
import threading
import unittest
import urllib.error
import urllib.request

from board import server
from factory_store import Store


class BoardTests(unittest.TestCase):
    def test_live_board_serves_task_state_but_not_native_logs_or_foreign_hosts(self):
        with tempfile.TemporaryDirectory() as root:
            store = Store(root)
            store.initialize({"concurrency": 2})
            store.add_tasks(
                [
                    {
                        "id": "one",
                        "title": "<img src=x onerror=alert(1)>",
                        "instructions": "inspect",
                        "write_paths": [],
                        "depends_on": [],
                    }
                ]
            )
            instance = server(store, 0)
            thread = threading.Thread(target=instance.serve_forever, daemon=True)
            thread.start()
            try:
                base = f"http://127.0.0.1:{instance.server_port}"
                with urllib.request.urlopen(base + "/api/state", timeout=3) as response:
                    state = json.load(response)
                    self.assertEqual(state["tasks"][0]["id"], "one")
                    self.assertEqual(response.headers["Cache-Control"], "no-store")
                with urllib.request.urlopen(base, timeout=3) as response:
                    self.assertEqual(response.status, 200)
                for path, headers, expected in [
                    ("/native/events.jsonl", {}, 404),
                    ("/api/state", {"Host": "unrelated.example"}, 403),
                ]:
                    with self.assertRaises(urllib.error.HTTPError) as raised:
                        urllib.request.urlopen(
                            urllib.request.Request(base + path, headers=headers),
                            timeout=3,
                        )
                    self.assertEqual(raised.exception.code, expected)
                    raised.exception.close()
            finally:
                instance.shutdown()
                instance.server_close()
                thread.join(timeout=3)


if __name__ == "__main__":
    unittest.main()
