import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from factory_store import Store


class MCPTests(unittest.TestCase):
    def test_real_stdio_handoff_and_message_keep_native_execution_running(self):
        with tempfile.TemporaryDirectory() as tmp:
            store = Store(tmp)
            store.initialize({"concurrency": 2})
            store.add_tasks(
                [
                    {
                        "id": "one",
                        "title": "one",
                        "instructions": "inspect",
                        "depends_on": [],
                        "write_paths": [],
                    }
                ]
            )
            store.claim()
            requests = [
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "method": "initialize",
                    "params": {"protocolVersion": "2024-11-05"},
                },
                {
                    "jsonrpc": "2.0",
                    "id": 2,
                    "method": "tools/call",
                    "params": {
                        "name": "send_message",
                        "arguments": {"to": "leader", "message": "need contract"},
                    },
                },
                {
                    "jsonrpc": "2.0",
                    "id": 3,
                    "method": "tools/call",
                    "params": {
                        "name": "handoff",
                        "arguments": {
                            "status": "completed",
                            "message": "inspected",
                            "artifacts": [],
                        },
                    },
                },
            ]
            run = subprocess.run(
                [
                    sys.executable,
                    str(Path(__file__).with_name("mcp_server.py")),
                    "--root",
                    tmp,
                    "--task",
                    "one",
                    "--attempt",
                    "1",
                ],
                input="\n".join(map(json.dumps, requests)) + "\n",
                capture_output=True,
                text=True,
            )
            self.assertEqual(run.returncode, 0, run.stderr)
            responses = [json.loads(line) for line in run.stdout.splitlines()]
            self.assertEqual([x["id"] for x in responses], [1, 2, 3])
            self.assertFalse(responses[2]["result"].get("isError", False))
            self.assertEqual(store.get("one")["submission"]["message"], "inspected")
            self.assertEqual(store.get("one")["status"], "running")
            self.assertEqual(store.snapshot()["messages"][0]["sender"], "one")

    def test_extra_identity_unknown_tool_and_stale_attempt_cannot_mutate_tasks(self):
        from mcp_server import call

        with tempfile.TemporaryDirectory() as tmp:
            store = Store(tmp)
            store.initialize({"concurrency": 1})
            store.add_tasks(
                [
                    {
                        "id": name,
                        "title": name,
                        "instructions": "inspect",
                        "depends_on": [],
                        "write_paths": [],
                    }
                    for name in ("one", "two")
                ]
            )
            store.claim()
            report = {"status": "completed", "message": "fixture", "artifacts": []}
            with self.assertRaises(ValueError):
                call(store, "one", 1, "handoff", {**report, "task_id": "two"})
            with self.assertRaises(ValueError):
                call(
                    store,
                    "one",
                    1,
                    "send_message",
                    {"sender": "two", "to": "leader", "message": "spoof"},
                )
            with self.assertRaises(ValueError):
                call(store, "one", 1, "unregistered", {})
            with self.assertRaises(ValueError):
                call(store, "one", 0, "handoff", report)
            self.assertIsNone(store.get("one")["submission"])
            self.assertEqual(store.get("one")["status"], "running")
            self.assertEqual(store.get("two")["status"], "pending")
            self.assertEqual(store.snapshot()["messages"], [])


if __name__ == "__main__":
    unittest.main()
