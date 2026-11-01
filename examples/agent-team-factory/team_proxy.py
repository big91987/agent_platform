"""Loopback UI over an authenticated G112 terminal relay; no LAN API."""

import argparse
import base64
import json
import re
import subprocess
import threading
import time
import uuid
import zlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


class TerminalTransport:
    def __init__(self, target):
        self.target = target
        self.lock = threading.Lock()

    def send(self, value):
        subprocess.run(
            ["tmux", "send-keys", "-t", self.target, "-l", json.dumps(value)],
            check=True,
            capture_output=True,
        )
        subprocess.run(
            ["tmux", "send-keys", "-t", self.target, "Enter"],
            check=True,
            capture_output=True,
        )

    def wait(self, pattern):
        deadline = time.monotonic() + 35
        while time.monotonic() < deadline:
            pane = subprocess.run(
                ["tmux", "capture-pane", "-p", "-J", "-t", self.target, "-S", "-300"],
                capture_output=True,
                text=True,
                check=True,
            ).stdout
            found = re.search(pattern, pane)
            if found:
                return found
            time.sleep(0.05)
        raise TimeoutError("执行连接回执未确认，请先查看状态，不盲目重复提交")

    def request(self, method, path, body):
        with self.lock:
            nonce = uuid.uuid4().hex
            payload = base64.b64encode(
                zlib.compress(
                    json.dumps({"method": method, "path": path, "body": body}).encode()
                )
            ).decode()
            parts = [payload[i : i + 2000] for i in range(0, len(payload), 2000)]
            for index, part in enumerate(parts):
                self.send(
                    {"id": nonce, "index": index, "parts": len(parts), "data": part}
                )
                self.wait(r"TEAM_IN " + nonce + " " + str(index) + r" END")
            result = bytearray()
            index = 0
            while True:
                item = self.wait(
                    r"TEAM_DATA "
                    + nonce
                    + " "
                    + str(index)
                    + r" (\d+) ([A-Za-z0-9+/=]+) END"
                )
                result.extend(base64.b64decode(item[2]))
                if index + 1 == int(item[1]):
                    break
                index += 1
                self.send({"id": nonce, "part": index})
            return json.loads(zlib.decompress(result))


def server(transport, port):
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
                status, kind = 200, "text/html; charset=utf-8"
            elif self.path.startswith("/api/"):
                try:
                    size = int(self.headers.get("Content-Length", "0"))
                    if size < 0 or size > 100000:
                        return self.send_error(413)
                    if (
                        self.command == "POST"
                        and self.headers.get_content_type() != "application/json"
                    ):
                        return self.send_error(415)
                    data = (
                        json.loads(self.rfile.read(size))
                        if self.command == "POST"
                        else None
                    )
                    response = transport.request(self.command, self.path, data)
                    status, body = (
                        response["status"],
                        json.dumps(response["body"], ensure_ascii=False).encode(),
                    )
                except Exception:
                    status, body = (
                        502,
                        json.dumps(
                            {
                                "error": "执行连接回执未确认。先刷新查看状态；提交标识已保留，不要盲目重复操作。"
                            },
                            ensure_ascii=False,
                        ).encode(),
                    )
                kind = "application/json; charset=utf-8"
            else:
                return self.send_error(404)
            self.send_response(status)
            for key, value in {
                "Content-Type": kind,
                "Content-Length": str(len(body)),
                "Cache-Control": "no-store",
                "X-Content-Type-Options": "nosniff",
                "Content-Security-Policy": "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'",
            }.items():
                self.send_header(key, value)
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_):
            pass

    return ThreadingHTTPServer(("127.0.0.1", port), Handler)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--tmux-target", required=True)
    parser.add_argument("--port", type=int, default=8990)
    args = parser.parse_args()
    server(TerminalTransport(args.tmux_target), args.port).serve_forever()
