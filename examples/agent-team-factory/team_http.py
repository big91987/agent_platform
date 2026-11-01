"""Private live API. Browser access is via the same-origin local proxy."""

import hmac
import json
import re
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import parse_qs, urlsplit

from git_workspace import git
from team_runtime import skill_body
from team_store import SKILLS


def turn_data(team, turn_id):
    turns = team.store.snapshot()["turns"]
    turn = next((x for x in turns if x["id"] == turn_id), None)
    if turn is None:
        raise ValueError("执行轮次不存在")
    directory = team.root / "native" / turn_id
    out = {"turn": turn, "input": "", "events": [], "stderr": ""}
    if (directory / "input.md").is_file():
        out["input"] = (directory / "input.md").read_text()
    if (directory / "events.jsonl").is_file():
        for line in (directory / "events.jsonl").open():
            try:
                out["events"].append(json.loads(line))
            except ValueError:
                pass  # A trailing event can still be in flight.
    if (directory / "stderr.log").is_file():
        out["stderr"] = (directory / "stderr.log").read_text()[-60000:]
    return out


def handler(team, token):
    class Handler(BaseHTTPRequestHandler):
        def response(self, status, value):
            body = json.dumps(value, ensure_ascii=False).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Cache-Control", "no-store")
            self.send_header("X-Content-Type-Options", "nosniff")
            self.end_headers()
            self.wfile.write(body)

        def authorized(self):
            return hmac.compare_digest(
                self.headers.get("Authorization", ""), "Bearer " + token
            )

        def do_GET(self):
            if not self.authorized():
                return self.response(401, {"error": "无访问权限"})
            try:
                url = urlsplit(self.path)
                query = parse_qs(url.query)
                if url.path == "/api/state":
                    state = team.store.snapshot()
                    state["config"] = {
                        key: team.config.get(key)
                        for key in [
                            "repository",
                            "integration_branch",
                            "concurrency",
                            "model",
                        ]
                    }
                    state["skills"] = list(SKILLS)
                    state["integration_head"] = git(team.repo, "rev-parse", "HEAD")
                    state["integration_blocked"] = (
                        team.root / "integration-pending.json"
                    ).exists()
                    return self.response(200, state)
                if url.path == "/api/trace":
                    return self.response(
                        200, turn_data(team, query.get("turn", [""])[0])
                    )
                if url.path == "/api/skill":
                    return self.response(
                        200, {"body": skill_body(query.get("name", [""])[0])}
                    )
                if url.path == "/api/artifact":
                    task = team.store.task(query.get("task", [""])[0])
                    relative = query.get("path", [""])[0]
                    if relative not in task["artifacts"]:
                        raise ValueError("只能读取已登记产物")
                    work = Path(task["worktree"]).resolve()
                    p = (work / relative).resolve()
                    if (
                        not p.is_relative_to(work)
                        or not p.is_file()
                        or p.stat().st_size > 512000
                    ):
                        raise ValueError("文件不可读或超过512KB")
                    return self.response(200, {"path": relative, "body": p.read_text()})
                if url.path == "/api/diff":
                    task = team.store.task(query.get("task", [""])[0])
                    if not task["candidate"]:
                        return self.response(200, {"body": "尚无候选提交"})
                    return self.response(
                        200,
                        {
                            "body": git(
                                team.repo,
                                "diff",
                                "--stat",
                                task["base_commit"],
                                task["candidate"],
                            )
                            + "\n\n"
                            + git(
                                team.repo,
                                "diff",
                                task["base_commit"],
                                task["candidate"],
                            )[:300000]
                        },
                    )
                self.response(404, {"error": "入口不存在"})
            except (ValueError, KeyError, OSError, UnicodeDecodeError) as error:
                self.response(400, {"error": str(error)})

        def do_POST(self):
            if not self.authorized():
                return self.response(401, {"error": "无访问权限"})
            try:
                size = int(self.headers.get("Content-Length", "0"))
                if (
                    not 0 < size <= 100000
                    or self.headers.get_content_type() != "application/json"
                ):
                    raise ValueError("请求格式或大小无效")
                args = json.loads(self.rfile.read(size))
                if not isinstance(args, dict):
                    raise ValueError("参数应为对象")
                if self.path == "/api/requests":
                    return self.response(
                        201, team.store.add_request(args["text"], args["client_id"])
                    )
                match = re.fullmatch(
                    r"/api/requests/(req-[a-f0-9]{12})/messages", self.path
                )
                if match:
                    return self.response(
                        200,
                        team.store.message("user", "leader", args["text"], match[1]),
                    )
                if self.path == "/api/pause":
                    team.store.pause(args["paused"])
                    return self.response(200, {"saved": True})
                if self.path == "/api/leader/continue":
                    if not team.store.snapshot()["requests"]:
                        raise ValueError("尚无用户需求")
                    team.store.continue_leader()
                    return self.response(200, {"saved": True})
                self.response(404, {"error": "入口不存在"})
            except (ValueError, TypeError, KeyError, OSError) as error:
                self.response(400, {"error": str(error)})

        def log_message(self, *_):
            pass

    return Handler


def serve(team, host, port, token):
    return ThreadingHTTPServer((host, port), handler(team, token))
