#!/usr/bin/env python3
"""Exercise real command cancellation and explicit recovery on an isolated host."""

import argparse
import json
import os
import secrets
import socket
import subprocess
import sys
import tempfile
import time
from pathlib import Path

from install import API


def command_fixture():
    request = json.load(sys.stdin)
    record = {"pid": os.getpid(), "seq": request["seq"]}
    with Path("effects.jsonl").open("a") as log:
        log.write(json.dumps(record) + "\n")
        log.flush()
        os.fsync(log.fileno())
    if request["seq"] == 1:
        time.sleep(120)
    # A real nonzero exit must follow the graph's failure edge.
    print("fixture exit 7", flush=True)
    raise SystemExit(7)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    target = parser.add_mutually_exclusive_group(required=True)
    target.add_argument("--platform-url")
    target.add_argument(
        "--server-binary",
        type=Path,
        help="Start an isolated server and test SIGKILL recovery",
    )
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    server, server_log, server_dir = None, None, None
    password = os.environ["PLATFORM_ADMIN_PASSWORD"]

    def start_server():
        nonlocal server
        server = subprocess.Popen(
            [
                str(args.server_binary.resolve()),
                "-listen",
                address,
                "-base-url",
                args.platform_url,
                "-data",
                str(server_dir),
            ],
            env={**os.environ, "AGENT_PLATFORM_PASSWORD": password},
            stdout=server_log,
            stderr=server_log,
            start_new_session=True,
        )
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            if server.poll() is not None:
                raise RuntimeError("isolated server exited; inspect " + str(server_dir))
            try:
                return API(args.platform_url, "admin", password)
            except OSError:
                time.sleep(0.2)
        raise RuntimeError("isolated server startup timed out")

    if args.server_binary:
        server_dir = Path(tempfile.mkdtemp(prefix="workflow-restart-"))
        server_log = (server_dir / "server.log").open("a")
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            address = "127.0.0.1:" + str(listener.getsockname()[1])
        args.platform_url = "http://" + address
        api = start_server()
    else:
        api = API(args.platform_url, "admin", password)
    workspace = Path(tempfile.mkdtemp(prefix="workflow-command-"))
    suffix = secrets.token_hex(5)
    connector, graph, route = None, None, None
    proof = {
        "workspace": str(workspace),
        "checks": [],
        "mode": "server-kill" if server else "stop",
        "server_data": str(server_dir) if server_dir else None,
    }

    def save():
        args.evidence.parent.mkdir(parents=True, exist_ok=True)
        args.evidence.write_text(json.dumps(proof, ensure_ascii=False, indent=2) + "\n")

    def until(predicate):
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            value = predicate()
            if value:
                return value
            time.sleep(0.2)
        raise AssertionError("timed out waiting for the real command lifecycle")

    def run():
        return api.call("GET", route)

    try:
        connector = api.call(
            "POST",
            "/api/connectors",
            {
                "name": "command-recovery-" + suffix,
                "kind": "command",
                "enabled": True,
                "workspace_root": str(workspace),
                "executable": sys.executable,
                "args": [str(Path(__file__).resolve()), "--fixture"],
                "timeout_seconds": 180,
            },
        )
        graph = api.call(
            "POST",
            "/api/workflows",
            {
                "name": "command-recovery-" + suffix,
                "enabled": True,
                "entry": "command",
                "nodes": [
                    {
                        "id": "command",
                        "kind": "connector",
                        "name": "真实命令",
                        "connector_id": connector["id"],
                    },
                    {"id": "inspect", "kind": "approval", "name": "检查失败证据"},
                    {"id": "done", "kind": "end", "name": "结束"},
                ],
                "edges": [
                    {"source": "command", "route": "next", "target": "done"},
                    {"source": "command", "route": "failed", "target": "inspect"},
                    {"source": "inspect", "route": "accept", "target": "done"},
                ],
            },
        )
        r = api.call(
            "POST",
            "/api/workflow-runs",
            {
                "workflow_id": graph["id"],
                "input": "isolated command cancellation acceptance",
                "workspace_path": str(workspace),
                "request_id": suffix,
            },
        )
        route = "/api/workflow-runs/" + r["id"]
        proof.update(
            run_id=r["id"], workflow_id=graph["id"], connector_id=connector["id"]
        )
        save()
        effects = workspace / "effects.jsonl"
        until(lambda: effects.exists() and effects.stat().st_size)
        pid = json.loads(effects.read_text().splitlines()[0])["pid"]
        os.kill(pid, 0)
        proof["checks"].append("real child process started and wrote one effect")
        if server:
            server.kill()
            server.wait(timeout=10)
            api = start_server()
            until(lambda: run()["status"] == "failed")
            assert len(effects.read_text().splitlines()) == 1
            proof["after_restart"] = run()
            proof["checks"].append(
                "SIGKILL restart retained Run and marked unknown execution failed without replay"
            )
        api.call("POST", route + "/stop", {"seq": 1})
        until(lambda: run()["status"] == "stopped")

        def dead():
            try:
                os.kill(pid, 0)
            except ProcessLookupError:
                return True
            return False

        until(dead)
        proof["checks"].append(
            "stop killed command process and retained original execution"
        )
        try:
            api.call(
                "POST", route + "/resume", {"seq": 1, "message": "inspect before retry"}
            )
        except RuntimeError as error:
            assert "this command was dispatched" in str(error), str(error)
        else:
            raise AssertionError("uncertain command was automatically replayed")
        assert len(effects.read_text().splitlines()) == 1
        proof["checks"].append("resume refused replay and effect count remained one")
        api.call(
            "POST",
            route + "/return",
            {
                "seq": 1,
                "target": "command",
                "summary": "verified only one fixture write; deliberately retry",
            },
        )
        until(lambda: run()["seq"] == 3 and run()["status"] == "waiting")
        proof["before_approval"] = run()
        assert proof["before_approval"]["steps"][-1]["node_id"] == "inspect"
        assert len(effects.read_text().splitlines()) == 2
        assert proof["before_approval"]["steps"][1]["result"]["route"] == "failed"
        proof["checks"].append(
            "explicit return made one new execution; nonzero exit routed to inspection"
        )
        api.call(
            "POST",
            route + "/decision",
            {
                "seq": 3,
                "route": "accept",
                "summary": "expected failure and cancellation verified",
            },
        )
        until(lambda: run()["status"] == "completed")
        proof["final"] = run()
        proof["effects"] = [
            json.loads(line) for line in effects.read_text().splitlines()
        ]
        proof["passed"] = True
    finally:
        cleanup_errors = []
        for collection, obj in (("workflows", graph), ("connectors", connector)):
            if obj:
                try:
                    api.call(
                        "PUT",
                        "/api/" + collection + "/" + obj["id"],
                        {**obj, "enabled": False},
                    )
                except Exception as error:
                    cleanup_errors.append(str(error))
        if cleanup_errors:
            proof["cleanup_errors"] = cleanup_errors
            proof["passed"] = False
        save()
        if server and server.poll() is None:
            server.terminate()
            try:
                server.wait(timeout=15)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait(timeout=5)
        if server_log:
            server_log.close()
    print(
        json.dumps(
            {
                k: v
                for k, v in proof.items()
                if k not in ("before_approval", "after_restart", "final")
            },
            ensure_ascii=False,
        )
    )

    if not proof.get("passed"):
        raise SystemExit(1)


if __name__ == "__main__":
    if sys.argv[1:] == ["--fixture"]:
        command_fixture()
    else:
        main()
