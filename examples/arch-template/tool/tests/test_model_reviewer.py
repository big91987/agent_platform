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
                            "message": {
                                "content": json.dumps(
                                    dict(
                                        snapshot="abc",
                                        reviewed_files=["a.go"],
                                        findings=[],
                                        limitations=[],
                                    )
                                )
                            }
                        }
                    ],
                    "usage": {"total_tokens": 7},
                }
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
        malformed[0] = True
        with self.assertRaises(ValueError):
            adapter.review(
                {},
                f"http://127.0.0.1:{server.server_port}/v1/chat/completions",
                "fixture-token",
                "fixture-model",
            )

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
