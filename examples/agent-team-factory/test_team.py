"""Isolated infrastructure fixtures only; never submit to the user's playground."""

import base64
import concurrent.futures
import json
import secrets
import subprocess
import sys
import tempfile
import threading
import unittest
import zlib
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import Request, urlopen

from git_workspace import git
from native import Native
from team_http import serve, turn_data
from team_mcp import LEADER, WORKER, validate
from team_runtime import Team
from team_store import TeamStore


class TeamTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        base = Path(self.tmp.name)
        self.repo = base / "repo"
        self.repo.mkdir()
        git(self.repo, "init", "-b", "agent-team/playground")
        git(self.repo, "config", "user.name", "Infrastructure fixture")
        git(self.repo, "config", "user.email", "fixture@example.invalid")
        (self.repo / "README.md").write_text("isolated fixture\n")
        git(self.repo, "add", ".")
        git(self.repo, "commit", "-m", "fixture")
        self.store = TeamStore(base / "runtime")
        self.store.initialize(
            {
                "repository": str(self.repo),
                "integration_branch": "agent-team/playground",
                "concurrency": 3,
                "codex": sys.executable,
                "model": "",
                "timeout_seconds": 2,
            }
        )
        self.team = Team(self.store.root)

    def tearDown(self):
        self.tmp.cleanup()

    def request(self):
        return self.store.add_request("isolated infrastructure fixture", "fixture-key")[
            "id"
        ]

    def task(self, req, key, deps=None):
        return self.store.create_task(
            {
                "request_id": req,
                "key": key,
                "title": key,
                "instructions": "fixture only",
                "skill": "research",
                "depends_on": deps or [],
            }
        )

    def test_starts_empty_and_idempotent_user_and_leader_writes(self):
        self.assertEqual(self.store.snapshot()["requests"], [])
        self.assertIsNone(self.store.claim_leader())
        req = self.request()
        self.assertEqual(self.request(), req)
        self.assertEqual(self.task(req, "one")["id"], self.task(req, "one")["id"])
        with self.assertRaises(ValueError):
            self.store.add_request("different", "fixture-key")

    def test_fixed_consumers_claim_unique_and_require_accepted_dependencies(self):
        req = self.request()
        first = self.task(req, "first")
        child = self.task(req, "child", [first["id"]])
        self.task(req, "other")
        with concurrent.futures.ThreadPoolExecutor(3) as pool:
            values = list(
                pool.map(self.store.claim, ["worker-1", "worker-2", "worker-3"])
            )
        claimed = [x["task"]["id"] for x in values if x]
        self.assertEqual(len(claimed), 2)
        self.assertEqual(len(set(claimed)), 2)
        self.assertNotIn(child["id"], claimed)

    def test_result_wakes_leader_and_does_not_skip_revision_arriving_mid_turn(self):
        req = self.request()
        turn = self.store.claim_leader()
        self.task(req, "one")
        current = self.store.claim("worker-1")
        self.store.finish_task(current["task"]["id"], "blocked", "fixture needs input")
        self.store.finish_turn(turn, {"returncode": 0}, "fixture reply")
        self.assertIsNotNone(self.store.claim_leader())

    def test_user_correction_remains_visible_after_many_worker_messages(self):
        req = self.request()
        self.store.message("user", "leader", "real correction fixture", req)
        for n in range(65):
            self.store.message("worker-1", "leader", "worker fixture " + str(n), req)
        board = self.team.board_for_leader()
        self.assertTrue(
            any(
                x["sender"] == "user" and x["body"] == "real correction fixture"
                for x in board["messages"]
            )
        )

    def test_peer_message_also_wakes_leader_to_coordinate_idle_consumers(self):
        req = self.request()
        turn = self.store.claim_leader()
        self.store.finish_turn(turn, {"returncode": 0}, "fixture")
        self.assertIsNone(self.store.claim_leader())
        self.store.message("worker-1", "worker-2", "fixture collaboration", req)
        self.assertIsNotNone(self.store.claim_leader())

    def test_manual_continuation_refuses_uncertain_native_termination(self):
        self.request()
        self.store.execution("leader", termination_error="isolated uncertainty fixture")
        with self.assertRaises(ValueError):
            self.store.continue_leader()

    def test_role_scope_extra_identity_and_expired_turn_are_rejected(self):
        req = self.request()
        self.task(req, "one")
        current = self.store.claim("worker-1")
        with self.assertRaises(ValueError):
            self.team.call(
                "worker-1", current["turn_id"], "create_task", {"request_id": req}
            )
        schema = next(t for t in WORKER if t["name"] == "send_message")["inputSchema"]
        with self.assertRaises(ValueError):
            validate(
                schema,
                {"request_id": req, "to": "leader", "message": "x", "sender": "user"},
            )
        self.store.finish_turn(current["turn_id"], {"returncode": 0}, "")
        with self.assertRaises(ValueError):
            self.team.call("worker-1", current["turn_id"], "get_task", {})

    def test_candidate_acceptance_is_explicit_and_releases_dependency(self):
        req = self.request()
        first = self.task(req, "one")
        child = self.task(req, "two", [first["id"]])
        work = self.team.workspace.prepare(first["id"], 1)
        (work["path"] / "fixture.txt").write_text("fixture artifact\n")
        candidate = self.team.workspace.commit({**first, "write_paths": ["**"]}, work)
        self.store.finish_task(
            first["id"], "review", "fixture result", ["fixture.txt"], candidate
        )
        self.assertIsNone(self.store.claim("worker-1"))
        self.team.accept(first["id"])
        self.assertEqual(self.store.claim("worker-1")["task"]["id"], child["id"])
        self.assertEqual((self.repo / "fixture.txt").read_text(), "fixture artifact\n")
        self.assertEqual(self.team.accept(first["id"])["status"], "accepted")

    def test_conflict_preflight_does_not_dirty_integration_or_create_pending(self):
        req = self.request()
        task = self.task(req, "conflict")
        work = self.team.workspace.prepare(task["id"], 1)
        (work["path"] / "README.md").write_text("candidate\n")
        candidate = self.team.workspace.commit({**task, "write_paths": ["**"]}, work)
        self.store.finish_task(task["id"], "review", "fixture", candidate=candidate)
        (self.repo / "README.md").write_text("integration\n")
        git(self.repo, "commit", "-am", "integration fixture")
        with self.assertRaises(ValueError):
            self.team.accept(task["id"])
        self.assertEqual(git(self.repo, "status", "--porcelain"), "")
        self.assertFalse((self.store.root / "integration-pending.json").exists())

    def test_inspection_catalog_uses_same_role_tools_without_dispatching(self):
        from team_proxy import server
        from team_store import SKILLS

        class NoDispatch:
            def request(self, *_):
                raise AssertionError("Catalog inspection must not reach execution")

        instance = server(NoDispatch(), 0)
        thread = threading.Thread(target=instance.serve_forever, daemon=True)
        thread.start()
        try:
            url = f"http://127.0.0.1:{instance.server_port}/api/catalog"
            with urlopen(url) as response:
                catalog = json.load(response)
            self.assertEqual(catalog["leader_tools"], LEADER)
            self.assertEqual(catalog["worker_tools"], WORKER)
            self.assertEqual(catalog["skills"], list(SKILLS))
            self.assertEqual(self.store.snapshot()["turns"], [])
        finally:
            instance.shutdown()
            instance.server_close()
            thread.join()

    def test_private_live_api_requires_key_and_does_not_invent_requests(self):
        instance = serve(self.team, "127.0.0.1", 0, "fixture-token")
        thread = threading.Thread(target=instance.serve_forever, daemon=True)
        thread.start()
        url = f"http://127.0.0.1:{instance.server_port}/api/state"
        try:
            with self.assertRaises(HTTPError) as error:
                urlopen(url)
            self.assertEqual(error.exception.code, 401)
            with urlopen(
                Request(url, headers={"Authorization": "Bearer fixture-token"})
            ) as response:
                self.assertEqual(json.load(response)["requests"], [])
        finally:
            instance.shutdown()
            instance.server_close()
            thread.join()

    def test_stdio_relay_preserves_multiframe_request_and_response(self):
        instance = serve(self.team, "127.0.0.1", 0, "fixture-token")
        thread = threading.Thread(target=instance.serve_forever, daemon=True)
        thread.start()
        (self.store.root / "http-key").write_text("fixture-token")
        p = subprocess.Popen(
            [
                sys.executable,
                str(Path(__file__).with_name("team_relay.py")),
                "--root",
                str(self.store.root),
                "--port",
                str(instance.server_port),
            ],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        try:
            self.assertEqual(p.stdout.readline().strip(), "TEAM_RELAY_READY")
            body = "中" * 20000 + secrets.token_hex(2000)
            data = base64.b64encode(
                zlib.compress(
                    json.dumps(
                        {
                            "method": "POST",
                            "path": "/api/requests",
                            "body": {"text": body, "client_id": "relay-fixture"},
                        }
                    ).encode()
                )
            ).decode()
            parts = [data[i : i + 2000] for i in range(0, len(data), 2000)]
            nonce = "a" * 32
            for n, part in enumerate(parts):
                p.stdin.write(
                    json.dumps(
                        {"id": nonce, "index": n, "parts": len(parts), "data": part}
                    )
                    + "\n"
                )
                p.stdin.flush()
                self.assertEqual(
                    p.stdout.readline().strip(), f"TEAM_IN {nonce} {n} END"
                )
            output = bytearray()
            n = 0
            while True:
                frame = p.stdout.readline().split()
                self.assertEqual(frame[:3], ["TEAM_DATA", nonce, str(n)])
                output.extend(base64.b64decode(frame[4]))
                if n + 1 == int(frame[3]):
                    break
                n += 1
                p.stdin.write(json.dumps({"id": nonce, "part": n}) + "\n")
                p.stdin.flush()
            reply = json.loads(zlib.decompress(output))
            self.assertEqual(reply["status"], 201)
            self.assertEqual(reply["body"]["body"], body)
            self.assertNotIn("fixture-token", str(reply))
            self.assertGreater(n, 0)
        finally:
            p.stdin.close()
            p.wait(timeout=5)
            p.stdout.close()
            p.stderr.close()
            instance.shutdown()
            instance.server_close()
            thread.join()

    def test_native_bridge_scopes_custom_tools_without_enlarging_shell_permissions(
        self,
    ):
        fixture = Path(self.tmp.name) / "native-fixture"
        fixture.write_text(
            '#!/usr/bin/env python3\nimport sys,json\nsys.stdin.read()\nprint(json.dumps({"type":"thread.started","thread_id":"fixture-session"}))\nprint(json.dumps({"type":"turn.completed","usage":{}}))\n'
        )
        fixture.chmod(0o700)
        with self.store.connect() as db:
            config = self.store.settings()
            config["codex"] = str(fixture)
            db.execute("UPDATE settings SET config=?", (json.dumps(config),))
        self.request()
        turn = self.store.claim_leader()
        result = Native(self.store).run(
            turn,
            self.repo,
            "isolated protocol fixture",
            task={"id": "leader", "attempt": 1, "write_paths": []},
            mcp={
                "name": "team_playground",
                "entry": __file__,
                "args": [],
                "tools": [x["name"] for x in LEADER],
            },
        )
        self.assertEqual(result["returncode"], 0)
        argv = json.loads(
            (self.store.root / "native" / turn / "invocation.json").read_text()
        )["argv"]
        self.assertIn('sandbox_mode="read-only"', argv)
        self.assertIn(
            'mcp_servers.team_playground.tools.create_task.approval_mode="approve"',
            argv,
        )
        self.assertFalse(any("dangerously" in x for x in argv))
        trace = turn_data(self.team, turn)
        self.assertEqual(trace["input"], "isolated protocol fixture")
        self.assertEqual(trace["invocation"]["cwd"], str(self.repo))
        self.assertTrue(trace["events"])

    def test_real_stdio_endpoint_keeps_worker_scope_and_checks_current_turn(self):
        req = self.request()
        self.task(req, "stdio")
        current = self.store.claim("worker-1")
        inputs = [
            {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}},
            {"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
            {
                "jsonrpc": "2.0",
                "id": 3,
                "method": "tools/call",
                "params": {"name": "get_task", "arguments": {}},
            },
            {
                "jsonrpc": "2.0",
                "id": 4,
                "method": "tools/call",
                "params": {"name": "create_task", "arguments": {}},
            },
        ]
        p = subprocess.run(
            [
                sys.executable,
                str(Path(__file__).with_name("team_mcp.py")),
                "--root",
                str(self.store.root),
                "--actor",
                "worker-1",
                "--turn",
                current["turn_id"],
            ],
            input="\n".join(json.dumps(x) for x in inputs) + "\n",
            text=True,
            capture_output=True,
            timeout=10,
        )
        self.assertEqual(p.returncode, 0, p.stderr)
        replies = [json.loads(x)["result"] for x in p.stdout.splitlines()]
        self.assertEqual(
            {x["name"] for x in replies[1]["tools"]},
            {"get_task", "send_message", "handoff"},
        )
        self.assertEqual(
            json.loads(replies[2]["content"][0]["text"])["task"]["id"],
            current["task"]["id"],
        )
        self.assertTrue(replies[3]["isError"])

    def test_empty_repo_bootstrap_uses_supported_entry_and_refuses_existing_product(
        self,
    ):
        repo = Path(self.tmp.name) / "empty-repo"
        repo.mkdir()
        git(repo, "init")
        cmd = [
            sys.executable,
            str(Path(__file__).with_name("playground.py")),
            "--root",
            str(Path(self.tmp.name) / "bootstrap-runtime"),
            "bootstrap",
            "--repository",
            str(repo),
        ]
        p = subprocess.run(cmd, text=True, capture_output=True, timeout=10)
        self.assertEqual(p.returncode, 0, p.stderr)
        self.assertEqual(
            git(repo, "ls-files").splitlines(), [".gitignore", "AGENTS.md", "README.md"]
        )
        before = git(repo, "rev-parse", "HEAD")
        p = subprocess.run(cmd, text=True, capture_output=True, timeout=10)
        self.assertNotEqual(p.returncode, 0)
        self.assertEqual(git(repo, "rev-parse", "HEAD"), before)


if __name__ == "__main__":
    unittest.main()
