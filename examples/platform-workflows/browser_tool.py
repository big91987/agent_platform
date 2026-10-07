#!/usr/bin/env python3
"""Workflow browser MCP adapter over the existing locked Harness browser runtime."""

import argparse
import hashlib
import json
import sys
from pathlib import Path

SOURCE = Path(__file__).resolve().parents[1] / "github/tooling"
sys.path.insert(0, str(SOURCE))
from full_harness import browser  # noqa: E402
from full_harness.common import controls  # noqa: E402


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--workspace-root", required=True, type=Path)
    parser.add_argument("--evidence", required=True, type=Path)
    args = parser.parse_args()
    tool = {
        "name": "check",
        "description": "Run a real browser journey against app or docs/workflow/prototype in the current workspace. Write a project-relative JSON plan array first. Actions: fill(label,value), click(role,name), visible(text), absent(text), reload, viewport(width), key(key), snapshot_storage/unchanged_storage; locators also support selector. Creates screenshots and browser.json; inspect failures. No shell or arbitrary network. "
        + browser.EXTRA_ACTIONS_DESCRIPTION.replace("<issue>", "workflow"),
        "inputSchema": {
            "type": "object",
            "properties": {
                "root": {"type": "string", "enum": ["app", "docs/workflow/prototype"]},
                "plan": {"type": "string"},
            },
            "required": ["root", "plan"],
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
                    "serverInfo": {"name": "workflow-browser", "version": "1"},
                }
            elif method == "tools/list":
                result = {"tools": [tool]}
            elif method == "ping":
                result = {}
            elif method == "tools/call" and request["params"]["name"] == "check":
                workspace = Path.cwd().resolve()
                if not workspace.is_relative_to(args.workspace_root.resolve()):
                    raise ValueError("Current workspace is outside the configured root")
                evidence = (
                    args.evidence.resolve()
                    / hashlib.sha256(str(workspace).encode()).hexdigest()
                )
                evidence.mkdir(parents=True, exist_ok=True)
                context = {
                    "workspace": str(workspace),
                    "source": str(SOURCE),
                    "task": {"number": "workflow"},
                    "stage": "check",
                    "state": {"turn": 1},
                    "evidence": str(evidence),
                    "controls": controls(workspace),
                    "config": {
                        "browser_roots": ["app", "docs/workflow/prototype"],
                        "check_timeout": 120,
                        "environment": {
                            "inherit": ["PATH", "HOME", "PLAYWRIGHT_BROWSERS_PATH"],
                            "set": {},
                        },
                    },
                }
                path = evidence / "context.json"
                path.write_text(json.dumps(context))
                data = browser.check(path, request["params"].get("arguments", {}))
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
