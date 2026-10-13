"""Credential-free runtime checks; these are infrastructure, not product QA."""

import json
import os
import subprocess
import tempfile
from pathlib import Path


def run(*command, cwd=None):
    return subprocess.run(
        command, check=True, capture_output=True, text=True, timeout=90, cwd=cwd
    ).stdout.strip()


with tempfile.TemporaryDirectory(prefix="runtime-check-", dir="/workspace") as folder:
    root = Path(folder)
    versions = {
        "codex": run("codex", "--version"),
        "node": run("node", "--version"),
        "python": run("python3", "--version"),
        "go": run("go", "version"),
        "git": run("git", "--version"),
    }
    if versions["codex"] != "codex-cli 0.151.0":
        raise RuntimeError("Unexpected native executor version")
    help_text = run("codex", "app-server", "--help")
    if "--listen" not in help_text or "--ws-auth" not in help_text:
        raise RuntimeError("Native bidirectional transport is unavailable")
    source = root / "proof.json"
    source.write_text(json.dumps({"value": "shared runtime"}))
    os.chmod(source, 0o600)
    python = run(
        "python3",
        "-c",
        "import json; print(json.load(open('proof.json'))['value'])",
        cwd=folder,
    )
    typescript = run(
        "tsx",
        "-e",
        "import fs from 'node:fs'; const value: string = "
        "JSON.parse(fs.readFileSync('proof.json','utf8')).value; console.log(value)",
        cwd=folder,
    )
    (root / "main.go").write_text(
        'package main\nimport "fmt"\nfunc main(){fmt.Print("shared runtime")}\n'
    )
    compiled = run("go", "run", "main.go", cwd=folder)
    if [python, typescript, compiled] != ["shared runtime"] * 3:
        raise RuntimeError("Programs did not share their working directory")
    browser = run(
        "node",
        "-e",
        "const {chromium}=require('playwright'); (async()=>{"
        "const browser=await chromium.launch({headless:true,args:['--no-sandbox']});"
        "try{const page=await browser.newPage({viewport:{width:390,height:844}});"
        "await page.setContent('<title>Runtime check</title><p>Browser ready</p>');"
        "if(await page.title()!=='Runtime check')throw Error('browser content lost');"
        "console.log(browser.version());}finally{await browser.close()}})()"
        ".catch(e=>{console.error(e);process.exit(1)});",
        cwd=folder,
    )
    print(
        json.dumps(
            {
                "event": "runtime_check_pass",
                "versions": versions,
                "chromium": browser,
                "python_typescript_go_shared_files": True,
                "native_transport_available": True,
                "native_login": "not_run",
                "product_qa": "not_run",
            }
        ),
        flush=True,
    )
