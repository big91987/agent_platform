"""Fixed browser checks exposed to stage Agents as one Runner-owned MCP tool."""

import argparse
import hashlib
import json
import subprocess
import sys
from pathlib import Path

if __package__ in (None, ""):
    sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from full_harness.common import (
    clean_env,
    controls,
    read_json,
    relative_file,
    run_process,
    write_json,
)

EXTRA_ACTIONS_DESCRIPTION = " Additional actions: zoom(factor:0.5..3) uses native Chromium tab zoom, saves actual zoom metrics and screenshot; accessibility(contains:optional text) saves the real accessibility tree and asserts visible-to-AX text, not screen-reader audio. For checks not covered by fixed actions, write a zero-argument JS function expression in docs/05-validation/tasks/<issue>/browser-scripts/*.js and call page_script(script:project-relative path). It runs in the isolated page (DOM APIs only, no Node/shell/filesystem), throws or returns false to fail, returns measurements for evidence. Same local-only network policy. Supplement existing checks; never rewrite trusted gates. Host scripts are not accepted."


def prepare(source, environment=None):
    """Reproducible installation; only called by the controller, never by task code."""
    runtime = Path(source) / "full_harness/browser"
    stamp = runtime / "node_modules/.harness-lock"
    expected = hashlib.sha256((runtime / "package-lock.json").read_bytes()).hexdigest()
    env = clean_env(environment)
    if not stamp.exists() or stamp.read_text() != expected:
        subprocess.run(
            ["npm", "ci", "--ignore-scripts", "--prefix", str(runtime)],
            env=env,
            check=True,
            timeout=180,
        )
        stamp.write_text(expected)
    subprocess.run(
        [str(runtime / "node_modules/.bin/playwright"), "install", "chromium"],
        env=env,
        check=True,
        timeout=180,
    )
    subprocess.run(
        ["node", str(runtime / "check.cjs"), "--probe"], env=env, check=True, timeout=30
    )


def check(context, arguments):
    c = read_json(context)
    workspace = Path(c["workspace"])
    if controls(workspace) != c["controls"]:
        raise ValueError("Protected execution files changed")
    if set(arguments) != {"root", "plan"}:
        raise ValueError("Expected root and plan")
    roots = [
        n.replace("{task}", str(c["task"]["number"]))
        for n in c["config"]["browser_roots"]
    ]
    if arguments["root"] not in roots:
        raise ValueError("Browser root is not configured by the repository owner")
    root = relative_file(workspace, arguments["root"])
    plan = relative_file(workspace, arguments["plan"])
    steps = read_json(plan)
    if not isinstance(steps, list) or len(steps) > 80:
        raise ValueError("Expected at most 80 browser steps")
    evidence = Path(c["evidence"])
    receipt = evidence / "browser.json"
    runs = read_json(receipt) if receipt.exists() else []
    name = f"docs/05-validation/tasks/{c['task']['number']}/browser/{c['stage']}-{c['state']['turn']}-{len(runs) + 1}"
    output = relative_file(workspace, name)
    prepared = []
    scripts_root = (
        workspace / f"docs/05-validation/tasks/{c['task']['number']}/browser-scripts"
    )
    for step in steps:
        if not isinstance(step, dict) or "source" in step:
            raise ValueError("Expected action data; inline source is not accepted")
        if step.get("action") == "page_script":
            script_file = relative_file(workspace, step.get("script", ""))
            if (
                not script_file.is_relative_to(scripts_root)
                or script_file.suffix != ".js"
            ):
                raise ValueError(
                    "Script must be in this task browser-scripts directory"
                )
            source = script_file.read_text()
            if len(source.encode()) > 65536:
                raise ValueError("Page script exceeds 64 KiB")
            step = {**step, "source": source}
        prepared.append(step)
    output.mkdir(parents=True, exist_ok=False)
    # Capture the exact assertions alongside evidence for independent QA/replay.
    prepared_plan = output / "executed-plan.json"
    write_json(prepared_plan, prepared)
    script = Path(c["source"]) / "full_harness/browser/check.cjs"
    log = evidence / f"browser-{len(runs) + 1}.log"
    code = run_process(
        ["node", str(script), str(root), str(output), str(prepared_plan)],
        workspace,
        clean_env(c["config"].get("environment")),
        log,
        c["config"]["check_timeout"],
    )
    result = (
        read_json(output / "browser.json")
        if (output / "browser.json").exists()
        else {"passed": False, "failure": log.read_text()[-3000:]}
    )
    result["passed"] = code == 0 and result.get("passed") is True
    artifacts = [
        p.relative_to(workspace).as_posix()
        for p in sorted(output.iterdir())
        if p.is_file()
    ]
    record = {
        "root": arguments["root"],
        "plan": arguments["plan"],
        "passed": result["passed"],
        "artifacts": artifacts,
    }
    runs.append(record)
    write_json(receipt, runs)
    return {**record, "result": result}


def serve(context):
    c = read_json(context)
    roots = [
        n.replace("{task}", str(c["task"]["number"]))
        for n in c["config"]["browser_roots"]
    ]
    tool = {
        "name": "check",
        "description": "Run real browser checks on an owner-configured local app/prototype and save desktop/mobile screenshots and results. Use this tool instead of launching a browser in the shell sandbox. plan is a project-relative JSON array: fill/click/visible/absent/reload, viewport(width), key(key), snapshot_storage/unchanged_storage, storage_write_failure(enabled: boolean; localStorage.setItem fault until disabled or page reload), fail_download, download(role/name or label, expected JSON, optional filename/suffix). Additional actions: hover(locator,duration_ms optional 0..2000), pointer(x,y), tap(locator), first-step device(touch:boolean,width:320..1920), snapshot_geometry/unchanged_geometry(same locator), unchanged_storage_writes after snapshot_storage. Locators also accept selector. Touch uses a dedicated context; resized screenshots alone do not prove touch. No arbitrary JavaScript or shell. Include returned artifacts in your reply; passing proves only these checks.",
        "annotations": {
            "readOnlyHint": False,
            "destructiveHint": False,
            "openWorldHint": False,
        },
        "inputSchema": {
            "type": "object",
            "properties": {
                "root": {"type": "string", "enum": roots},
                "plan": {"type": "string"},
            },
            "required": ["root", "plan"],
            "additionalProperties": False,
        },
    }
    tool["description"] = (
        tool["description"].replace(
            "No arbitrary JavaScript or shell.", "No host JavaScript or shell."
        )
        + EXTRA_ACTIONS_DESCRIPTION
    )
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
                    "serverInfo": {"name": "harness_browser", "version": "1.0"},
                }
            elif method == "tools/list":
                result = {"tools": [tool]}
            elif method == "ping":
                result = {}
            elif method == "tools/call" and request["params"]["name"] == "check":
                data = check(context, request["params"].get("arguments", {}))
                result = {
                    "content": [
                        {"type": "text", "text": json.dumps(data, ensure_ascii=False)}
                    ],
                    "isError": not data["passed"],
                }
            else:
                raise ValueError("Unsupported browser method")
            reply = {"jsonrpc": "2.0", "id": request["id"], "result": result}
        except Exception as error:
            reply = {
                "jsonrpc": "2.0",
                "id": request["id"],
                "result": {
                    "isError": True,
                    "content": [{"type": "text", "text": str(error)}],
                },
            }
        print(json.dumps(reply, ensure_ascii=False), flush=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("context", type=Path)
    serve(parser.parse_args().context)
