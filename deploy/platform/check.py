#!/usr/bin/env python3
"""Bounded supported-API regression of an installed platform's sandbox code flow.

No model turn or product acceptance is performed. Report paths are exclusive;
unknown writes are never replayed. The operator is the human-gate test actor.
"""

import argparse
import datetime
import http.cookiejar
import json
import os
import time
import urllib.error
import urllib.request
from pathlib import Path

from install import private_file


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        raise RuntimeError("platform redirects are refused")


def check(root, template, report):
    settings = json.loads(private_file(root / "instance.json"))
    base = settings["base_url"]
    password = private_file(root / "credentials.env").decode().strip().split("=", 1)[1]
    with report.open("x") as output:
        report.chmod(0o600)

        def record(event, value):
            output.write(
                json.dumps(
                    dict(
                        event=event,
                        time=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                        value=value,
                    ),
                    ensure_ascii=False,
                )
                + "\n"
            )
            output.flush()
            os.fsync(output.fileno())

        opener = urllib.request.build_opener(
            urllib.request.ProxyHandler({}),
            urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()),
            NoRedirect(),
        )

        def api(path, method="GET", value=None):
            data = json.dumps(value).encode() if value is not None else None
            request = urllib.request.Request(
                base + path,
                data=data,
                method=method,
                headers={"Content-Type": "application/json", "Origin": base},
            )
            try:
                with opener.open(request, timeout=15) as response:
                    raw = response.read(8 * 1024 * 1024 + 1)
                if len(raw) > 8 * 1024 * 1024:
                    raise RuntimeError("platform response exceeds its size limit")
                return json.loads(raw)
            except urllib.error.HTTPError as error:
                raise RuntimeError(
                    f"platform API returned HTTP {error.code} for {path}"
                ) from None

        def wait(run_id, expected):
            deadline = time.monotonic() + 120
            while time.monotonic() < deadline:
                run = api("/api/workflow-runs/" + run_id)
                if run["status"] == expected:
                    return run
                if run["status"] in ("failed", "cancelled", "completed", "stopped"):
                    raise RuntimeError(
                        "Run reached " + run["status"] + ": " + run.get("error", "")
                    )
                time.sleep(0.25)
            raise RuntimeError(
                "Run has not reached the expected state before the deadline; inspect the original report"
            )

        health = api("/api/health")
        record("health", health)
        api("/api/login", "POST", {"username": "admin", "password": password})
        password = ""
        graph = json.loads(template.read_text())
        graph["name"] += " · " + report.stem
        record("workflow_create_intent", {"name": graph["name"]})
        saved = api("/api/workflows", "POST", graph)
        record("workflow_saved", saved)
        payload = {
            "workflow_id": saved["id"],
            "input": "Literal $(no shell) workflow input",
            "parameters": {"check": "shared-workspace"},
            "request_id": "platform-runtime-check:" + report.stem,
        }
        record("run_start_intent", payload)
        run = api("/api/workflow-runs", "POST", payload)
        record("run_created", run)
        run_id = run["id"]
        # A repeated identical submission is an intentional deduplication case,
        # performed only after the authoritative first Run receipt was saved.
        duplicate = api("/api/workflow-runs", "POST", payload)
        if duplicate["id"] != run_id:
            raise RuntimeError("duplicate input created another Run")
        record("duplicate_same_run", {"id": run_id})
        run = wait(run_id, "waiting")
        record("at_human_gate", run)
        environment_id = run["environment"]["id"]
        if run["seq"] != 3 or run["steps"][-1]["node_id"] != "confirm":
            raise RuntimeError("Run waited at an unexpected node")
        api(
            "/api/workflow-runs/" + run_id + "/decision",
            "POST",
            {
                "seq": run["seq"],
                "route": "next",
                "summary": "Harness operator confirms infrastructure regression only; not product acceptance",
            },
        )
        run = wait(run_id, "completed")
        record("completed", run)
        if (
            run["environment"]["id"] != environment_id
            or run["steps"][-1]["node_id"] != "done"
        ):
            raise RuntimeError("environment changed or wrong completion node")
        for seq, marker in (
            (1, "PLATFORM_PYTHON_PASS"),
            (2, "PLATFORM_TYPESCRIPT_PASS"),
            (4, "PLATFORM_PROJECT_COMMAND_PASS"),
        ):
            offset = 0
            parts = []
            while True:
                page = api(
                    f"/api/workflow-runs/{run_id}/steps/{seq}/output?offset={offset}"
                )
                record("output_page", {"seq": seq, "page": page})
                parts.append(page["output"])
                if page.get("truncated"):
                    raise RuntimeError("verification output was truncated")
                if page["eof"]:
                    break
                if page["next_offset"] <= offset:
                    raise RuntimeError("output pagination did not advance")
                offset = page["next_offset"]
            if marker not in "".join(parts):
                raise RuntimeError("original execution proof missing")
        record("release_intent", {"id": environment_id})
        api("/api/workflow-runs/" + run_id + "/environment/release", "POST", {})
        after = api("/api/workflow-runs/" + run_id)
        record("released", after)
        if after["environment"]["status"] != "released":
            raise RuntimeError("environment absence was not confirmed")
        api("/api/logout", "POST", {})
        record(
            "scope",
            {
                "code_flow": "pass",
                "duplicate": "pass",
                "operator_gate": "pass",
                "release": "pass",
                "native_agent": "not_run",
                "product_acceptance": "not_run",
            },
        )
        return {
            "workflow_id": saved["id"],
            "run_id": run_id,
            "environment_id": environment_id,
            "url": base + "/workflow-runs/" + run_id,
            "scope": "platform code-flow infrastructure regression",
        }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--template", type=Path, required=True)
    parser.add_argument("--report", type=Path, required=True)
    args = parser.parse_args()
    os.umask(0o077)
    print(json.dumps(check(args.root, args.template, args.report)))


if __name__ == "__main__":
    main()
