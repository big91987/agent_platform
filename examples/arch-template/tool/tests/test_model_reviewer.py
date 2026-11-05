"""A local HTTP fixture checks provider transport, not model reasoning quality."""

import importlib.util
import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path


class ModelTransportTests(unittest.TestCase):
    def test_model_receives_exact_bundle_and_malformed_response_fails(self):
        module_path = Path(__file__).resolve().parents[1] / "model_reviewer.py"
        self.assertTrue(module_path.exists(), "model transport is not implemented")
        spec = importlib.util.spec_from_file_location("model_reviewer", module_path)
        adapter = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(adapter)
        captured = []
        malformed = [False]
        finish = ["stop"]
        blank = [False]

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                captured.append(
                    (
                        self.headers.get("Authorization"),
                        json.loads(
                            self.rfile.read(int(self.headers["Content-Length"]))
                        ),
                    )
                )
                response = {
                    "choices": [
                        {
                            "finish_reason": finish[0],
                            "message": {
                                "content": json.dumps(
                                    dict(
                                        snapshot="abc",
                                        reviewed_files=["a.go"],
                                        findings=[],
                                        limitations=[],
                                    )
                                )
                            },
                        }
                    ],
                    "usage": {"total_tokens": 7},
                }
                if blank[0]:
                    response["choices"][0]["message"]["content"] = "   "
                if malformed[0]:
                    response["choices"][0]["message"]["content"] = "not JSON"
                body = json.dumps(response).encode()
                self.send_response(200)
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        server = HTTPServer(("127.0.0.1", 0), Handler)
        self.addCleanup(server.server_close)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(server.shutdown)
        result = adapter.review(
            {"snapshot": "abc", "sources": {"a.go": "package a"}},
            f"http://127.0.0.1:{server.server_port}/v1/chat/completions",
            "fixture-token",
            "fixture-model",
            stream=False,
        )
        self.assertEqual(result["snapshot"], "abc")
        self.assertEqual(captured[0][0], "Bearer fixture-token")
        self.assertEqual(captured[0][1]["model"], "fixture-model")
        self.assertEqual(
            json.loads(captured[0][1]["messages"][1]["content"])["sources"],
            {"a.go": "package a"},
        )
        self.assertNotIn("fixture-token", json.dumps(result))
        self.assertEqual(result["reviewer"]["usage"]["total_tokens"], 7)
        self.assertEqual(captured[0][1]["max_tokens"], 65536)
        finish[0] = "length"
        with self.assertRaisesRegex(ValueError, "finish_reason"):
            adapter.review(
                {},
                f"http://127.0.0.1:{server.server_port}/review",
                "fixture-token",
                "fixture-model",
                stream=False,
            )
        finish[0] = "stop"
        blank[0] = True
        with self.assertRaisesRegex(ValueError, "empty"):
            adapter.review(
                {},
                f"http://127.0.0.1:{server.server_port}/review",
                "fixture-token",
                "fixture-model",
                stream=False,
            )
        blank[0] = False
        malformed[0] = True
        with self.assertRaises(ValueError):
            adapter.review(
                {},
                f"http://127.0.0.1:{server.server_port}/v1/chat/completions",
                "fixture-token",
                "fixture-model",
                stream=False,
            )

    def test_stream_collects_content_and_usage_and_rejects_missing_done(self):
        module_path = Path(__file__).resolve().parents[1] / "model_reviewer.py"
        spec = importlib.util.spec_from_file_location("stream_reviewer", module_path)
        adapter = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(adapter)
        complete = [True]
        captured_requests = []

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                request = json.loads(
                    self.rfile.read(int(self.headers["Content-Length"]))
                )
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.end_headers()
                captured_requests.append(request)
                content = json.dumps(dict(snapshot="abc", findings=[]))
                events = [
                    dict(
                        model="real-provider-model",
                        choices=[
                            dict(
                                delta=dict(reasoning_content="private reasoning"),
                                finish_reason=None,
                            )
                        ],
                    ),
                    dict(
                        choices=[
                            dict(delta=dict(content=content[:10]), finish_reason=None)
                        ]
                    ),
                    dict(
                        choices=[
                            dict(delta=dict(content=content[10:]), finish_reason="stop")
                        ]
                    ),
                    dict(choices=[], usage=dict(total_tokens=12)),
                ]
                for event in events:
                    self.wfile.write(("data: " + json.dumps(event) + "\n\n").encode())
                if complete[0]:
                    self.wfile.write(b"data: [DONE]\n\n")

            def log_message(self, *args):
                pass

        server = HTTPServer(("127.0.0.1", 0), Handler)
        self.addCleanup(server.server_close)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        self.addCleanup(server.shutdown)
        url = f"http://127.0.0.1:{server.server_port}/review"
        result = adapter.review({}, url, "fixture-token", "fixture-model")
        self.assertEqual(result["snapshot"], "abc")
        self.assertTrue(captured_requests[0]["stream"])
        self.assertTrue(captured_requests[0]["stream_options"]["include_usage"])
        self.assertEqual(result["reviewer"]["model"], "real-provider-model")
        self.assertEqual(result["reviewer"]["usage"]["total_tokens"], 12)
        self.assertNotIn("private reasoning", json.dumps(result))
        complete[0] = False
        with self.assertRaisesRegex(ValueError, "incomplete"):
            adapter.review({}, url, "fixture-token", "fixture-model")

    def test_redirect_never_forwards_model_credentials(self):
        module_path = Path(__file__).resolve().parents[1] / "model_reviewer.py"
        spec = importlib.util.spec_from_file_location("model_reviewer", module_path)
        adapter = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(adapter)
        received = []

        class Receiver(BaseHTTPRequestHandler):
            def do_GET(self):
                received.append(self.headers.get("Authorization"))
                self.send_response(200)
                self.end_headers()
                self.wfile.write(
                    json.dumps({"choices": [{"message": {"content": "{}"}}]}).encode()
                )

            def log_message(self, *args):
                pass

        receiver = HTTPServer(("127.0.0.1", 0), Receiver)
        self.addCleanup(receiver.server_close)
        threading.Thread(target=receiver.serve_forever, daemon=True).start()
        self.addCleanup(receiver.shutdown)

        class Redirect(BaseHTTPRequestHandler):
            def do_POST(self):
                self.send_response(302)
                self.send_header(
                    "Location", f"http://localhost:{receiver.server_port}/other-origin"
                )
                self.end_headers()

            def log_message(self, *args):
                pass

        redirect = HTTPServer(("127.0.0.1", 0), Redirect)
        self.addCleanup(redirect.server_close)
        threading.Thread(target=redirect.serve_forever, daemon=True).start()
        self.addCleanup(redirect.shutdown)
        with self.assertRaises(ValueError):
            adapter.review(
                {},
                f"http://127.0.0.1:{redirect.server_port}/review",
                "FAKE_REVIEW_TOKEN",
                "fixture",
            )
        self.assertEqual(received, [])


if __name__ == "__main__":
    unittest.main()
