"""Test the SDK's HTTP boundary, not native Agent reasoning."""

import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from agent_platform_client import APIError, Client


class ClientTest(unittest.TestCase):
    def setUp(self):
        self.calls = []
        self.extra_length = 0
        self.response = (200, "application/json", b"{}")
        test = self

        class Handler(BaseHTTPRequestHandler):
            def handle_request(self):
                body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
                test.calls.append((self.command, self.path, dict(self.headers), body))
                status, content_type, content = test.response
                self.send_response(status)
                self.send_header("Content-Type", content_type)
                if status == 302:
                    self.send_header("Location", "/unexpected")
                self.send_header(
                    "Content-Length", str(len(content) + test.extra_length)
                )
                self.end_headers()
                self.wfile.write(content)

            do_GET = handle_request
            do_POST = handle_request

            def log_message(self, *_):
                pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.client = Client(
            f"http://127.0.0.1:{self.server.server_port}", "private-token"
        )

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def respond(self, data, status=200):
        self.response = (status, "application/json", json.dumps(data).encode())

    def test_workflow_entry_and_feedback_use_stable_event_keys(self):
        self.respond({"id": "run", "status": "running"}, 201)
        result = self.client.start_workflow(
            "graph",
            "修复",
            workspace_path="/work/issue",
            request_id="issue:1",
            parameters={"issue_number": "7"},
        )
        self.assertEqual(result["id"], "run")
        self.assertEqual(self.calls[-1][1], "/api/workflow-runs")
        self.assertEqual(
            json.loads(self.calls[-1][3])["parameters"], {"issue_number": "7"}
        )
        self.client.workflow_by_request("github:owner/repo:1")
        self.assertIn("request_id=github%3Aowner%2Frepo%3A1", self.calls[-1][1])
        self.client.workflow_message("run", "说明", request_id="comment:4")
        self.assertEqual(self.calls[-1][1], "/api/workflow-runs/run/messages")
        self.assertEqual(
            json.loads(self.calls[-1][3]),
            {"message": "说明", "request_id": "comment:4"},
        )
        with self.assertRaises(ValueError):
            self.client.start_workflow(
                "graph", "work", workspace_path="/work", request_id=""
            )
        with self.assertRaises(ValueError):
            self.client.workflow_command("run", "delete", seq=1)

    def test_network_scope_preserves_explicit_false(self):
        self.client.invoke(
            "offline", agent_id="a", request_id="network", network_access=False
        )
        self.assertIs(json.loads(self.calls[-1][3])["network_access"], False)
        self.client.network_access("abc", True)
        self.assertEqual(self.calls[-1][1], "/api/conversations/abc/network-access")
        self.assertEqual(json.loads(self.calls[-1][3]), {"enabled": True})

    def test_submit_preserves_caller_request_key_and_native_receipt(self):
        receipt = {
            "conversation_id": "c1",
            "message_id": 7,
            "status": "queued",
            "conversation_url": "http://platform/conversations/c1",
            "duplicate": False,
        }
        self.respond(receipt, 202)
        self.assertEqual(
            receipt,
            self.client.invoke(
                "请澄清需求",
                agent_id="a1",
                workspace_path="/workspaces/project/task",
                request_id="issue-1-input-1",
            ),
        )
        method, path, headers, body = self.calls[-1]
        self.assertEqual(("POST", "/api/invoke"), (method, path))
        self.assertEqual("Bearer private-token", headers["Authorization"])
        self.assertEqual(
            {
                "agent_id": "a1",
                "message": "请澄清需求",
                "workspace_path": "/workspaces/project/task",
                "request_id": "issue-1-input-1",
            },
            json.loads(body),
        )
        self.client.invoke("继续", conversation_id="c1", request_id="issue-1-input-2")
        self.assertNotIn("workspace_path", json.loads(self.calls[-1][3]))
        self.assertEqual("c1", json.loads(self.calls[-1][3])["conversation_id"])

    def test_external_tool_registration_keeps_mcp_schema_and_auth(self):
        server = {
            "id": "external",
            "name": "External service",
            "enabled": True,
            "connection": {"url": "https://tools.example/mcp"},
        }
        self.respond(server)
        self.assertEqual(server, self.client.register_tool_server(server))
        method, path, headers, body = self.calls[-1]
        self.assertEqual(("POST", "/api/tool-servers"), (method, path))
        self.assertEqual("Bearer private-token", headers["Authorization"])
        self.assertEqual(server, json.loads(body))
        server["tools"] = [{"name": "handoff", "inputSchema": {"type": "object"}}]
        self.respond(server)
        self.assertEqual(server, self.client.discover_tool_server("external"))
        self.assertEqual("/api/tool-servers/external/discover", self.calls[-1][1])
        self.respond([server])
        self.assertEqual([server], self.client.tool_servers())
        self.assertEqual("GET", self.calls[-1][0])

    def test_tool_confirmation_uses_the_same_conversation(self):
        self.respond({"ok": True})
        self.client.decide_tool("conversation", "approval", "decline")
        method, path, headers, body = self.calls[-1]
        self.assertEqual(
            (method, path),
            ("POST", "/api/conversations/conversation/approvals/approval"),
        )
        self.assertEqual(json.loads(body), {"decision": "decline"})
        self.assertEqual(headers["Authorization"], "Bearer private-token")

    def test_http_failure_is_visible_without_replay_or_credential_redirect(self):
        for status, data in [
            (401, {"error": "invalid credential"}),
            (409, {"error": "request conflicts"}),
            (503, {"error": "unavailable private-token"}),
        ]:
            with self.subTest(status=status):
                self.respond(data, status)
                before = len(self.calls)
                with self.assertRaises(APIError) as raised:
                    self.client.invoke(
                        "change files", agent_id="a1", request_id="stable-key"
                    )
                self.assertEqual(status, raised.exception.status_code)
                self.assertNotIn("private-token", str(raised.exception))
                self.assertEqual(before + 1, len(self.calls))
        self.response = (302, "text/plain", b"redirect")
        before = len(self.calls)
        with self.assertRaises(APIError):
            self.client.me()
        self.assertEqual(before + 1, len(self.calls))
        self.respond({"conversation_id": "c1", "private": "private-token"})
        self.extra_length = 20
        before = len(self.calls)
        with self.assertRaises(ConnectionError) as raised:
            self.client.invoke("change files", agent_id="a1", request_id="stable-key")
        self.assertNotIn("private-token", str(raised.exception))
        self.assertEqual(before + 1, len(self.calls))

    def test_stream_keeps_event_envelope_cursor_and_file_bytes(self):
        event = {
            "id": 12,
            "message_id": 7,
            "type": "item.completed",
            "raw": {
                "type": "item.completed",
                "item": {"type": "agent_message", "text": "已读取仓库"},
            },
            "created_at": "2026-10-01T00:00:00Z",
        }
        stream = ": heartbeat\n\nid: 12\ndata: " + json.dumps(event) + "\n\n"
        self.response = (200, "text/event-stream", stream.encode())
        self.assertEqual([event], list(self.client.stream("c1", after=11)))
        self.assertEqual("/api/conversations/c1/events?after=11", self.calls[-1][1])
        self.assertEqual("11", self.calls[-1][2]["Last-Event-Id"])
        image = b"\x89PNG\r\n\x1a\nreal-file"
        self.response = (200, "application/octet-stream", image)
        self.assertEqual(image, self.client.read_file("c1", "evidence/手机.png"))
        self.assertIn("%E6%89%8B%E6%9C%BA.png", self.calls[-1][1])

    def test_wait_tracks_submitted_message_and_timeout_does_not_stop_agent(self):
        data = {
            "conversation": {"id": "c1", "status": "running"},
            "messages": [
                {"id": 7, "role": "user", "status": "completed"},
                {"id": 9, "role": "user", "status": "running"},
            ],
            "artifacts": [],
        }
        self.respond(data)
        self.assertEqual(data, self.client.wait("c1", message_id=7, timeout=1))
        with self.assertRaises(TimeoutError):
            self.client.wait("c1", message_id=9, timeout=0)
        self.assertTrue(all(call[0] == "GET" for call in self.calls))
        data["conversation"]["status"] = "failed"
        data["messages"][1]["status"] = "queued"
        self.respond(data)
        self.assertEqual(data, self.client.wait("c1", message_id=9, timeout=1))


if __name__ == "__main__":
    unittest.main()
