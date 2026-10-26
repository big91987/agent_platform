#!/usr/bin/env python3
"""Pinned OCR execution and verifiable reports; no publishing or source edits."""

import hashlib
import json
import os
import platform
import re
import subprocess
import time
import urllib.request
from pathlib import Path

VERSION = "v1.13.3"
BINARIES = {
    "linux-amd64": "843be0e56fdff0a5ca6f86c5866f942d6a372dff1c0ca19e7893d72495abf45c",
    "linux-arm64": "d9934312a629438d7517a250c8cb739b96282c8e7d04fc75e5b2e5931807318c",
    "darwin-arm64": "fa9b8530c0a35e6106a9ab2e81c21d98ef26d3f31c1f39ad3ef4c2528047c4f4",
    "darwin-amd64": "093d0a402aff9715c31c02903268fb73880f8a9350cc2a5813a939ee186e5545",
}
OUTPUT = Path("harness-output/code-review")
STATE = Path(".git/agent-platform-review")


def save(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")


def git(root, *args):
    return subprocess.check_output(
        ["git", "--no-pager", "-c", "core.hooksPath=/dev/null", "-C", str(root), *args],
        timeout=90,
    )


def candidate_snapshot(root=Path(".")):
    files = git(root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
    inventory = {}
    for raw in sorted(set(files.split(b"\0")) - {b""}):
        name = os.fsdecode(raw)
        if name.startswith("harness-output/"):
            continue
        file = root / name
        if file.is_symlink():
            data = ("symlink:" + os.readlink(file)).encode()
        elif file.is_file():
            data = file.read_bytes()
        else:
            data = b"deleted"
        inventory[name] = hashlib.sha256(data).hexdigest()
    return inventory


def install_ocr(directory=STATE):
    architecture = {"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(
        platform.machine()
    )
    target = platform.system().lower() + "-" + str(architecture)
    expected = BINARIES.get(target)
    if not expected:
        raise ValueError("unsupported OCR runtime platform")
    directory.mkdir(parents=True, exist_ok=True)
    binary = directory / "ocr"
    if not binary.exists():
        url = f"https://github.com/alibaba/open-code-review/releases/download/{VERSION}/opencodereview-{target}"
        with urllib.request.urlopen(url, timeout=120) as response:
            data = response.read(100 * 1024 * 1024 + 1)
        if hashlib.sha256(data).hexdigest() != expected:
            raise ValueError("OCR binary digest differs from pinned release")
        binary.write_bytes(data)
        binary.chmod(0o700)
    if (
        binary.is_symlink()
        or hashlib.sha256(binary.read_bytes()).hexdigest() != expected
    ):
        raise ValueError("installed OCR binary differs from pinned release")
    return str(binary.resolve())


def assess(value, exit_code, expected):
    comments = value.get("comments")
    valid_comments = comments is None or (
        isinstance(comments, list) and all(isinstance(c, dict) for c in comments)
    )
    result = {
        "status": "incomplete",
        "comments": comments if valid_comments and comments is not None else [],
        "summary": value.get("summary", {}),
        "llm": value.get("llm", {}),
    }
    if exit_code:
        return dict(result, status="error")
    if value.get("warnings") or not valid_comments or not expected:
        return result
    manifest = value.get("manifest", {})
    if (
        manifest.get("schema_version") != "ocr.run-manifest/v1"
        or manifest.get("terminal_state") != "complete"
        or value.get("status") != "complete"
    ):
        return result
    coverage = manifest.get("coverage", {})
    if coverage.get("failed") or coverage.get("waived") or manifest.get("run_failure"):
        return result
    selected = [v["path"] for v in coverage.get("selected", [])]
    done = [
        v["path"] for v in coverage.get("completed", []) + coverage.get("reused", [])
    ]
    if sorted(selected) != sorted(expected) or sorted(done) != sorted(expected):
        return result
    for comment in result["comments"]:
        if (
            comment.get("path") not in expected
            or not comment.get("content")
            or comment.get("severity") not in ("critical", "high", "medium", "low")
        ):
            return dict(result, comments=[])
    return dict(result, status="findings" if result["comments"] else "clean")


def run_tool(arguments, destination, *, llm=False, timeout=120):
    allowed = (
        "PATH",
        "LANG",
        "LC_ALL",
        "HOME",
        "TMPDIR",
        "SSL_CERT_FILE",
        "SSL_CERT_DIR",
        "http_proxy",
        "https_proxy",
        "no_proxy",
        "HTTP_PROXY",
        "HTTPS_PROXY",
        "NO_PROXY",
    )
    env = {key: value for key, value in os.environ.items() if key in allowed}
    env.update(
        OCR_ENABLE_TELEMETRY="0",
        GIT_TERMINAL_PROMPT="0",
        GIT_CONFIG_NOSYSTEM="1",
        GIT_CONFIG_GLOBAL="/dev/null",
    )
    if llm:
        for key in (
            "OCR_LLM_URL",
            "OCR_LLM_TOKEN",
            "OCR_LLM_MODEL",
            "OCR_LLM_PROTOCOL",
        ):
            if not os.environ.get(key):
                raise ValueError("CLI model connection is not configured")
            env[key] = os.environ[key]
        env["OCR_LLM_TIMEOUT"] = "300"
    started = time.monotonic()
    try:
        result = subprocess.run(
            arguments,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout,
        )
        code, stdout, stderr = result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired as error:
        code, stdout, stderr = 124, error.stdout or b"", error.stderr or b""
    secret = os.environ.get("OCR_LLM_TOKEN", "").encode()
    if secret:
        stdout, stderr = (
            stdout.replace(secret, b"[redacted]"),
            stderr.replace(secret, b"[redacted]"),
        )
    destination.write_bytes(stdout)
    destination.with_suffix(".stderr.log").write_bytes(stderr)
    return code, round(time.monotonic() - started, 3)


def candidate_commit(root=Path(".")):
    """Freeze worktree changes without altering HEAD or the caller's index."""
    import tempfile

    directory = root / ".git/agent-platform-review"
    directory.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=directory) as temporary:
        env = dict(
            os.environ,
            GIT_INDEX_FILE=str((Path(temporary) / "index").resolve()),
            GIT_AUTHOR_NAME="Code Review",
            GIT_AUTHOR_EMAIL="review@localhost",
            GIT_COMMITTER_NAME="Code Review",
            GIT_COMMITTER_EMAIL="review@localhost",
        )

        def command(*args):
            return (
                subprocess.check_output(
                    ["git", "-C", str(root), *args], env=env, timeout=60
                )
                .decode()
                .strip()
            )

        command("read-tree", "HEAD")
        command("add", "-A", "--", ".", ":!harness-output")
        tree = command("write-tree")
        return command(
            "commit-tree", tree, "-p", "HEAD", "-m", "Local immutable review candidate"
        )


def preview(base, head, *, binary=None):
    OUTPUT.mkdir(parents=True, exist_ok=False)
    (OUTPUT / "raw.stderr.log").write_text("")
    STATE.mkdir(parents=True, exist_ok=True)
    binary = binary or install_ocr()
    before = candidate_snapshot()
    # No PR-controlled rule file or external tools are loaded for this check.
    command = [
        binary,
        "delegate",
        "preview",
        "--format",
        "json",
        "--from",
        base,
        "--to",
        head,
        "--rule",
        str((STATE / "rules.json").resolve()),
    ]
    code, _ = run_tool(command, OUTPUT / "preview.json")
    if code:
        raise ValueError("OCR file selection failed; inspect retained output")
    value = json.loads((OUTPUT / "preview.json").read_text())
    files = [v["path"] for v in value["reviewable_files"]]
    save(
        OUTPUT / "source.json",
        {"base": base, "head": head, "files": before, "expected": files},
    )
    code, _ = run_tool(
        [
            binary,
            "delegate",
            "rule",
            "--format",
            "json",
            "--rule",
            str((STATE / "rules.json").resolve()),
            *files,
        ],
        OUTPUT / "rules.json",
    )
    if code:
        raise ValueError("OCR rule resolution failed; inspect retained output")
    return binary


def trusted_source(*, required=False):
    """Use the platform's captured prepare result, never an Agent-edited file."""
    if os.environ.get("WORKFLOW_INPUT_FILE"):
        context = json.loads(Path(os.environ["WORKFLOW_INPUT_FILE"]).read_text())
        prepare = [
            step for step in context["previous_results"] if step["node"] == "prepare"
        ]
        if prepare:
            receipt = prepare[-1].get("execution_receipt", {})
            # OpenSandbox may omit stream chunk separators in the captured log.
            captured = re.findall(
                r"CODE_REVIEW_SOURCE_SHA256=([0-9a-f]{64})(?![0-9a-f])",
                receipt.get("output", ""),
            )
            if (
                len(captured) == 1
                and receipt.get("exit_code") == 0
                and not receipt.get("log", {}).get("truncated", False)
            ):
                raw = (OUTPUT / "source.json").read_bytes()
                if hashlib.sha256(raw).hexdigest() != captured[0]:
                    raise ValueError(
                        "source baseline differs from platform-captured digest"
                    )
                return json.loads(raw)
    if required:
        raise ValueError("missing platform-captured preparation result")
    # In-process CLI node/local probe: no Agent runs between selection and finish.
    return json.loads((OUTPUT / "source.json").read_text())


def finish(engine, value, code=0, elapsed=None):
    source = trusted_source(required=engine == "codex-delegate")
    result = (
        assess_delegate(value, source["expected"])
        if engine == "codex-delegate"
        else assess(value, code, source["expected"])
    )
    result.update(
        engine=engine,
        version=VERSION,
        base=source["base"],
        head=source["head"],
        elapsed_seconds=elapsed,
    )
    if (
        candidate_snapshot() != source["files"]
        or json.loads((OUTPUT / "source.json").read_text()) != source
    ):
        result.update(status="error", reason="review changed candidate source")
    save(OUTPUT / "result.json", result)
    lines = [
        "# Code Review",
        "",
        f"Result: **{result['status']}**",
        f"Engine: {engine}",
        f"Range: `{source['base']}` → `{source['head']}`",
        "",
        "## Findings",
    ]
    for comment in result["comments"]:
        lines += [
            "",
            f"- {comment.get('severity', 'unclassified')} · {comment.get('path', '')}:{comment.get('start_line', '')}",
            str(comment.get("content", "")),
        ]
    if not result["comments"]:
        lines += ["", "No findings reported. Incomplete/error is not approval."]
    lines += [
        "",
        "Full review output, selected/excluded files, rules and source inventory are retained with this Run. AI review does not replace tests, QA or human merge approval.",
    ]
    (OUTPUT / "report.md").write_text("\n".join(lines) + "\n")
    print(json.dumps({k: v for k, v in result.items() if k != "comments"}))
    return {"clean": 0, "findings": 10, "incomplete": 20, "error": 30}[result["status"]]


def cli_review(binary=None):
    source = trusted_source()
    code, elapsed = run_tool(
        [
            binary or install_ocr(),
            "review",
            "--format",
            "json",
            "--audience",
            "human",
            "--from",
            source["base"],
            "--to",
            source["head"],
            "--rule",
            str((STATE / "rules.json").resolve()),
            "--effort",
            "low",
            "--concurrency",
            "2",
            "--timeout",
            "20",
            "--max-tokens-budget",
            "500000",
        ],
        OUTPUT / "raw.json",
        llm=True,
        timeout=1800,
    )
    try:
        value = json.loads((OUTPUT / "raw.json").read_text())
        if not isinstance(value, dict):
            raise ValueError()
    except (ValueError, UnicodeError):
        value, code = {}, code or 1
    return finish("ocr-cli", value, code, elapsed)


def assess_delegate(value, expected):
    result = {"status": "incomplete", "comments": value.get("comments", [])}
    files = value.get("files", [])
    if (
        value.get("engine") != "codex-delegate"
        or not expected
        or sorted(v.get("path", "") for v in files) != sorted(expected)
        or any(v.get("status") != "reviewed" for v in files)
        or not isinstance(result["comments"], list)
    ):
        return result
    for comment in result["comments"]:
        if (
            comment.get("path") not in expected
            or not comment.get("content")
            or comment.get("severity") not in ("critical", "high", "medium", "low")
        ):
            return result
    return dict(result, status="findings" if result["comments"] else "clean")


def delegate_finish():
    value = json.loads((OUTPUT / "raw.json").read_text())
    return finish("codex-delegate", value)
