#!/usr/bin/env python3
"""Project browser integration, registered as stdio MCP; no platform dependency."""

import argparse
import hashlib
import json
import sys
from pathlib import Path

from tooling import tooling_source


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument(
        "--stage", choices=("design", "development", "qa", "review"), required=True
    )
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text())["pipeline"]
    sys.path.insert(0, str(tooling_source(config)))
    from full_harness import browser

    check = browser.check

    tool = {
        "name": "check",
        "description": "Run real browser checks on this task's product or prototype. Write a project-relative JSON plan of fill(label,value), click(role,name), visible(text), absent(text), reload, viewport(width), key(key), snapshot_storage/unchanged_storage, storage_write_failure(enabled: boolean), fail_download, download(role/name or label, expected JSON, optional filename/suffix) actions. storage_write_failure controls localStorage.setItem failure until disabled or the page reloads; it does not modify stored data. Additional actions: hover(locator, optional duration_ms 0..2000), pointer(x,y), tap(locator), device(touch:boolean,width:320..1920) as first step to create a touch-capable context, snapshot_geometry/unchanged_geometry(same locator, selector supported), unchanged_storage_writes after snapshot_storage (observes attempts, including across reload). All locators also accept selector. Save desktop/mobile screenshots and browser.json; resized screenshots alone do not prove touch. Inspect failed results and fix before retrying.",
        "inputSchema": {
            "type": "object",
            "properties": {"root": {"type": "string"}, "plan": {"type": "string"}},
            "required": ["root", "plan"],
            "additionalProperties": False,
        },
    }
    tool["description"] += getattr(browser, "EXTRA_ACTIONS_DESCRIPTION", "")
    verify_tool = {
        "name": "verify",
        "description": "Run the owner-configured product quality and browser checks on the host, outside the Agent shell sandbox. No command arguments. Returns current real outcomes before handoff. On failure fix the product or report the actual limitation. Does not approve or hand off.",
        "inputSchema": {
            "type": "object",
            "properties": {},
            "additionalProperties": False,
        },
    }
    for line in sys.stdin:
        request = json.loads(line)
        if "id" not in request:
            continue
        try:
            method = request["method"]
            if method == "initialize":
                result = {
                    "protocolVersion": request["params"]["protocolVersion"],
                    "capabilities": {"tools": {}},
                    "serverInfo": {"name": "project-browser", "version": "1"},
                }
            elif method == "tools/list":
                result = {"tools": [tool, verify_tool]}
            elif method == "ping":
                result = {}
            elif method == "tools/call" and request["params"]["name"] in (
                "check",
                "verify",
            ):
                workspace = Path.cwd().resolve()
                directory = (
                    Path(config["registry"])
                    / hashlib.sha256(str(workspace).encode()).hexdigest()
                )
                registration = json.loads(
                    (directory / (args.stage + ".json")).read_text()
                )
                if registration["workspace"] != str(workspace):
                    raise ValueError("Workspace does not match registration")
                number = int(registration["inputs"]["issue"])
                if request["params"]["name"] == "verify":
                    from full_harness.common import controls
                    from verify import verify

                    if request["params"].get("arguments", {}):
                        raise ValueError("verify takes no arguments")
                    if controls(workspace) != registration["controls"]:
                        raise ValueError("Protected execution files changed")
                    verify(config, workspace, number)
                    result = {
                        "content": [
                            {
                                "type": "text",
                                "text": "Product checks passed. Current records: docs/05-validation/tasks/"
                                + str(number)
                                + "/delivery-checks/checks.json",
                            }
                        ]
                    }
                    print(
                        json.dumps(
                            {"jsonrpc": "2.0", "id": request["id"], "result": result}
                        ),
                        flush=True,
                    )
                    continue
                evidence = directory / (args.stage + "-browser")
                evidence.mkdir(exist_ok=True)
                context = {
                    "workspace": str(workspace),
                    "source": str(tooling_source(config)),
                    "task": {"number": number},
                    "stage": args.stage,
                    "state": {"turn": 1},
                    "evidence": str(evidence),
                    "controls": registration["controls"],
                    "config": {
                        "browser_roots": [
                            "app",
                            f"docs/04-implementation/tasks/{number}/prototype",
                        ],
                        "check_timeout": 120,
                        "environment": {
                            "inherit": ["PATH", "HOME", "PLAYWRIGHT_BROWSERS_PATH"],
                            "set": {},
                        },
                    },
                }
                context_file = evidence / "context.json"
                context_file.write_text(json.dumps(context))
                data = check(context_file, request["params"].get("arguments", {}))
                result = {
                    "content": [
                        {"type": "text", "text": json.dumps(data, ensure_ascii=False)}
                    ],
                    "isError": not data["passed"],
                }
            else:
                raise ValueError("Unsupported MCP method")
        except Exception as error:
            result = {
                "isError": True,
                "content": [{"type": "text", "text": str(error)}],
            }
        print(
            json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": result}),
            flush=True,
        )


if __name__ == "__main__":
    main()
