#!/usr/bin/env python3
"""Exercise bundled tools on a temporary product, without GitHub or a platform."""

import json
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "sdk/python"))


def main():
    from pipeline import prepare_registration
    from verify import quality_commands

    with tempfile.TemporaryDirectory(prefix="pipeline-portability-") as temporary:
        base = Path(temporary).resolve()
        workspace = base / "product"
        app = workspace / "app"
        app.mkdir(parents=True)
        (app / "index.html").write_text(
            '<!doctype html><html lang="en"><head><title>Portable fixture</title>'
            '<link rel="stylesheet" href="styles.css"></head><body>'
            '<h1>Portable fixture</h1><button>Continue</button><p id="status">Ready</p>'
            '<script src="app.js"></script></body></html>'
        )
        (app / "app.js").write_text(
            'document.querySelector("button").addEventListener("click", () => {'
            'document.querySelector("#status").textContent = "Done";});'
        )
        (app / "styles.css").write_text("body { color: #123; }")
        actions = [
            {"action": "visible", "text": "Ready"},
            {"action": "click", "role": "button", "name": "Continue"},
            {"action": "visible", "text": "Done"},
        ]
        plan = workspace / "docs/05-validation/tasks/1/browser-plan.json"
        core = workspace / "tests/browser/core.json"
        for path in (plan, core):
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(json.dumps(actions))
        subprocess.run(["git", "init", "-q", str(workspace)], check=True)
        for command in quality_commands(fix=True):
            subprocess.run(command, cwd=workspace, check=True, capture_output=True)
        config_file = base / "runner.json"
        settings = {
            "checkout": str(workspace),
            "registry": str(base / "registry"),
            "repository": "fixture/product",
            "python": sys.executable,
            "config_path": str(config_file),
        }
        config_file.write_text(json.dumps({"pipeline": settings}))
        prepare_registration(settings, {"workspace": str(workspace)}, "development", 1)
        requests = [
            {
                "jsonrpc": "2.0",
                "id": 1,
                "method": "tools/call",
                "params": {
                    "name": "check",
                    "arguments": {
                        "root": "app",
                        "plan": str(plan.relative_to(workspace)),
                    },
                },
            },
            {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "tools/call",
                "params": {"name": "verify", "arguments": {}},
            },
        ]
        process = subprocess.run(
            [
                sys.executable,
                str(ROOT / "examples/github/browser_tool.py"),
                "--config",
                str(config_file),
                "--stage",
                "development",
            ],
            cwd=workspace,
            input="".join(json.dumps(request) + "\n" for request in requests),
            text=True,
            capture_output=True,
            timeout=180,
            check=True,
        )
        responses = [json.loads(line) for line in process.stdout.splitlines()]
        if len(responses) != 2 or any(r["result"].get("isError") for r in responses):
            raise RuntimeError(process.stdout + process.stderr)
        results = list((workspace / "docs").rglob("browser.json"))
        if len(results) != 3 or not all(
            json.loads(p.read_text())["passed"] for p in results
        ):
            raise RuntimeError("Expected MCP, core and feature browser evidence")
        if len(list((workspace / "docs").rglob("*.png"))) != 6:
            raise RuntimeError("Expected desktop and mobile screenshots for all checks")
        print(
            "PASS: bundled stdio MCP check + verify, 3 real browser runs, 6 screenshots; no external Harness checkout."
        )


if __name__ == "__main__":
    main()
