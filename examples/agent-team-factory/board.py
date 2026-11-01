"""Read-only local experiment board. Native logs are never served."""

import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


def server(store, port=8977):
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            allowed = {
                f"127.0.0.1:{self.server.server_port}",
                f"localhost:{self.server.server_port}",
            }
            if self.headers.get("Host") not in allowed:
                self.send_error(403)
                return
            if self.path == "/":
                content = Path(__file__).with_name("board.html").read_bytes()
                kind = "text/html; charset=utf-8"
            elif self.path == "/api/state":
                state = store.snapshot()
                config = store.settings()
                state["config"] = {
                    key: config.get(key)
                    for key in (
                        "repository",
                        "integration_branch",
                        "concurrency",
                        "model",
                    )
                }
                content = json.dumps(state, ensure_ascii=False).encode()
                kind = "application/json; charset=utf-8"
            else:
                self.send_error(404)
                return
            self.send_response(200)
            self.send_header("Content-Type", kind)
            self.send_header("Content-Length", str(len(content)))
            self.send_header("Cache-Control", "no-store")
            self.send_header("X-Content-Type-Options", "nosniff")
            self.send_header(
                "Content-Security-Policy",
                "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'",
            )
            self.end_headers()
            self.wfile.write(content)

        def log_message(self, *_):
            pass

    return ThreadingHTTPServer(("127.0.0.1", port), Handler)


def serve(store, port):
    instance = server(store, port)
    print(f"Agent Team board: http://127.0.0.1:{instance.server_port}/", flush=True)
    try:
        instance.serve_forever()
    finally:
        instance.server_close()
