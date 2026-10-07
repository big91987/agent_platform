#!/usr/bin/env python3
"""Real HTTP isolation/deduplication check. Use an isolated test platform only."""

import argparse
import json
import os
import secrets
import shutil
import tempfile
import time
from pathlib import Path

from install import API


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform-url", required=True)
    parser.add_argument("--evidence", required=True, type=Path)
    args = parser.parse_args()
    api = API(args.platform_url, "admin", os.environ["PLATFORM_ADMIN_PASSWORD"])
    suffix = secrets.token_hex(5)
    users, run, graph = [], None, None
    workspace = None
    proof = {"checks": [], "users": [], "workflow_id": None, "run_id": None}

    def save():
        args.evidence.parent.mkdir(parents=True, exist_ok=True)
        args.evidence.write_text(json.dumps(proof, ensure_ascii=False, indent=2) + "\n")

    def denied(client, method, path, body, code, label):
        try:
            client.call(method, path, body)
        except RuntimeError as error:
            if f"HTTP {code}:" not in str(error):
                raise
        else:
            raise AssertionError(label + " unexpectedly accepted")
        proof["checks"].append(label)
        save()

    try:
        clients = []
        for role in ("owner", "other"):
            password = secrets.token_urlsafe(32)
            user = api.call(
                "POST",
                "/api/users",
                {
                    "username": f"wf-test-{role}-{suffix}",
                    "user_id": f"wf-test-{role}-{suffix}",
                    "role": "caller",
                    "enabled": True,
                    "password": password,
                },
            )
            users.append(user)
            proof["users"].append(user["username"])
            save()
            clients.append(API(args.platform_url, user["username"], password))
        owner, other = clients
        graph = api.call(
            "POST",
            "/api/workflows",
            {
                "name": "HTTP 权限验收 " + suffix,
                "enabled": True,
                "authorized_users": [u["user_id"] for u in users],
                "entry": "approval",
                "nodes": [
                    {"id": "approval", "kind": "approval", "name": "验收"},
                    {"id": "done", "kind": "end", "name": "结束"},
                ],
                "edges": [{"source": "approval", "route": "accept", "target": "done"}],
            },
        )
        proof["workflow_id"] = graph["id"]
        save()
        workspace = tempfile.mkdtemp(prefix="workflow-access-")
        proof["workspace"] = workspace
        body = {
            "workflow_id": graph["id"],
            "input": "权限与幂等检查",
            "workspace_path": workspace,
            "request_id": "same-request",
        }
        run = owner.call("POST", "/api/workflow-runs", body)
        proof["run_id"] = run["id"]
        save()
        assert owner.call("POST", "/api/workflow-runs", body)["id"] == run["id"]
        proof["checks"].append("duplicate start preserves one Run")
        denied(
            owner,
            "POST",
            "/api/workflow-runs",
            {**body, "input": "changed"},
            409,
            "changed request rejected",
        )
        denied(
            owner,
            "POST",
            "/api/workflow-runs",
            {**body, "request_id": "second"},
            409,
            "overlapping workspace rejected",
        )
        route = "/api/workflow-runs/" + run["id"]
        denied(other, "GET", route, None, 403, "other owner cannot read Run")
        for action, payload in [
            ("decision", {"seq": 1, "route": "accept"}),
            ("stop", {"seq": 1}),
            ("return", {"seq": 1, "target": "done", "summary": "test"}),
            ("resume", {"seq": 1, "message": "test"}),
        ]:
            denied(
                other,
                "POST",
                route + "/" + action,
                payload,
                403,
                "other owner cannot " + action,
            )
        graph["nodes"][0]["x"] = 220
        graph = api.call("PUT", "/api/workflows/" + graph["id"], graph)
        assert owner.call("GET", route)["definition"]["revision"] == 1
        proof["checks"].append("existing Run retains graph revision 1")
        owner.call(
            "POST",
            route + "/decision",
            {"seq": 1, "route": "accept", "summary": "检查通过"},
        )
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            if owner.call("GET", route)["status"] == "completed":
                break
            time.sleep(0.2)
        else:
            raise AssertionError("approved Run failed to finish")
        proof["checks"].append("owner approval completes Run")
        graph["enabled"] = False
        graph = api.call("PUT", "/api/workflows/" + graph["id"], graph)
        denied(
            owner,
            "POST",
            "/api/workflow-runs",
            {**body, "request_id": "disabled"},
            400,
            "disabled graph cannot start",
        )
        proof["passed"] = True
    finally:
        if workspace and proof.get("passed"):
            shutil.rmtree(workspace)
        for user in users:
            api.call("POST", "/api/users", {**user, "enabled": False})
        if graph and graph["enabled"]:
            graph["enabled"] = False
            api.call("PUT", "/api/workflows/" + graph["id"], graph)
        save()
    print(json.dumps(proof, ensure_ascii=False))


if __name__ == "__main__":
    main()
