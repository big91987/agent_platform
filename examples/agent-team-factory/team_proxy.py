"""Loopback UI gateway. Upstream access key never enters browser JS or URLs."""

import argparse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import ProxyHandler, Request, build_opener


def server(upstream, key, port):
    transport = build_opener(ProxyHandler({}))

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            self.handle_request()

        def do_POST(self):
            self.handle_request()

        def handle_request(self):
            allowed = {
                f"127.0.0.1:{self.server.server_port}",
                f"localhost:{self.server.server_port}",
            }
            if self.headers.get("Host") not in allowed or self.headers.get(
                "Origin", "http://" + self.headers.get("Host", "")
            ) not in {"http://" + host for host in allowed}:
                return self.send_error(403)
            if self.command == "GET" and self.path == "/":
                body = Path(__file__).with_name("team.html").read_bytes()
                self.send_response(200)
                self.send_header("Content-Type", "text/html; charset=utf-8")
            elif self.path.startswith("/api/"):
                try:
                    size = int(self.headers.get("Content-Length", "0"))
                    if size < 0 or size > 100000:
                        return self.send_error(413)
                    if (
                        self.command == "POST"
                        and "application/json"
                        not in self.headers.get("Content-Type", "")
                    ):
                        return self.send_error(415)
                    data = self.rfile.read(size) if self.command == "POST" else None
                    request = Request(
                        upstream.rstrip("/") + self.path,
                        data=data,
                        method=self.command,
                        headers={
                            "Authorization": "Bearer " + key,
                            "Content-Type": "application/json",
                        },
                    )
                    try:
                        with transport.open(request, timeout=30) as response:
                            body, status = response.read(), response.status
                    except HTTPError as error:
                        body, status = error.read(), error.code
                    self.send_response(status)
                    self.send_header("Content-Type", "application/json; charset=utf-8")
                except (ValueError, URLError, TimeoutError):
                    return self.send_error(502, "执行服务暂时不可达")
            else:
                return self.send_error(404)
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Cache-Control", "no-store")
            self.send_header("X-Content-Type-Options", "nosniff")
            self.send_header(
                "Content-Security-Policy",
                "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'",
            )
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_):
            pass

    return ThreadingHTTPServer(("127.0.0.1", port), Handler)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--upstream", required=True)
    parser.add_argument("--key-file", required=True)
    parser.add_argument("--port", type=int, default=8990)
    args = parser.parse_args()
    server(
        args.upstream, Path(args.key_file).read_text().strip(), args.port
    ).serve_forever()
