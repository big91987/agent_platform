#!/usr/bin/env python3
"""Install independent inspection using public, revisioned platform APIs."""

import argparse
import json
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path[:0] = [str(HERE), str(HERE.parents[1] / "sdk/python")]
from agent_platform_client import Client  # noqa: E402
from workflow import inspection_definition  # noqa: E402


def comparable(graph):
    return {
        key: (graph.get(key) if graph.get(key) is not None else [])
        if key in ("hooks", "start_nodes")
        else graph[key]
        for key in (
            "name",
            "enabled",
            "environment",
            "context_version",
            "authorized_users",
            "entry",
            "max_steps",
            "nodes",
            "edges",
            "hooks",
            "start_nodes",
        )
    }


def has_active_runs(client, workflow_id):
    before = ""
    while True:
        page = client.workflow_runs(workflow_id=workflow_id, before=before)
        if not page:
            return False
        if any(run["status"] in ("running", "waiting", "stopping") for run in page):
            return True
        next_cursor = page[-1]["id"]
        if next_cursor == before:
            raise ValueError("run pagination did not advance")
        before = next_cursor


def install(client, source_id, name, manifest):
    source = client._json("/api/workflows/" + source_id)
    desired = inspection_definition(source, name)
    if manifest.exists():
        old = json.loads(manifest.read_text())
        actual = client._json("/api/workflows/" + old["id"])
        if comparable(actual) != comparable(old):
            raise ValueError("inspection workflow has external edits")
        if comparable(desired) == comparable(old["requested"]):
            return old["id"]
        if has_active_runs(client, old["id"]):
            raise ValueError(
                "finish or formally cancel inspection Runs before upgrading"
            )
        result = client._json(
            "/api/workflows/" + old["id"],
            {**desired, "revision": actual["revision"]},
            method="PUT",
        )
    else:
        if any(graph["name"] == name for graph in client._json("/api/workflows")):
            raise ValueError(
                "workflow exists without installation manifest; reconcile first"
            )
        result = client._json("/api/workflows", desired)
    result["requested"] = desired
    manifest.parent.mkdir(parents=True, exist_ok=True)
    manifest.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n")
    manifest.chmod(0o600)
    return result["id"]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--platform-url", required=True)
    parser.add_argument("--token-file", required=True, type=Path)
    parser.add_argument("--source-workflow", required=True)
    parser.add_argument("--name", required=True)
    parser.add_argument("--manifest", required=True, type=Path)
    args = parser.parse_args()
    client = Client(args.platform_url, args.token_file.read_text().strip())
    print(install(client, args.source_workflow, args.name, args.manifest))


if __name__ == "__main__":
    main()
