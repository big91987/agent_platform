#!/usr/bin/env python3
"""Execute both model-backed reviews on one unchanged candidate; never publish."""

import argparse
import hashlib
import importlib.util
import json
import os
import re
import subprocess
import sys
import uuid
from contextlib import redirect_stdout
from pathlib import Path

from asset_snapshot import tree_digest, verify_arch_parsers


def decision(ocr_code, arch_code, model_status, unchanged):
    if (
        not unchanged
        or ocr_code not in (0, 10)
        or arch_code not in (0, 1)
        or model_status not in ("no_findings", "findings")
    ):
        return 20
    return 10 if ocr_code == 10 or arch_code == 1 or model_status == "findings" else 0


def prepared_head(context):
    rows = [
        row
        for row in context["previous_results"]
        if row.get("node_id", row.get("node")) == "prepare"
    ]
    if not rows or rows[-1].get("status") != "completed":
        raise ValueError("missing completed preparation receipt")
    receipt = rows[-1].get("connector_receipt") or {}
    if receipt.get("exit_code") != 0 or receipt.get("log", {}).get("truncated"):
        raise ValueError("preparation receipt failed or truncated")
    value = json.loads(receipt["output"])
    if not re.fullmatch("[0-9a-f]{40}", str(value.get("head", ""))):
        raise ValueError("preparation receipt lacks fixed head")
    return value["head"]


def load_ocr_runtime():
    source = Path(__file__).resolve().parents[1] / "code-review/runtime.py"
    spec = importlib.util.spec_from_file_location("trusted_ocr_runtime", source)
    ocr = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(ocr)
    return ocr


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--arch-tool-root", type=Path, required=True)
    parser.add_argument("--arch-digest", required=True)
    args = parser.parse_args()
    ocr = load_ocr_runtime()
    source = Path(ocr.__file__)
    arch = args.arch_tool_root.resolve(strict=True)
    if tree_digest(arch) != args.arch_digest:
        raise ValueError("Installed architecture tool differs from its recorded digest")
    verify_arch_parsers(arch)
    output = Path("harness-output")
    output.mkdir(exist_ok=True)
    if ocr.OUTPUT.exists():
        ocr.OUTPUT.rename(output / ("previous-code-review-" + uuid.uuid4().hex))
    ocr.STATE.mkdir(parents=True, exist_ok=True)
    (ocr.STATE / "rules.json").write_bytes(source.with_name("rules.json").read_bytes())
    context = json.load(sys.stdin)
    if Path(context["workspace_path"]).resolve() != Path.cwd():
        raise ValueError("Run workspace and process directory differ")
    base = prepared_head(context)
    before = ocr.candidate_snapshot()
    candidate = ocr.candidate_commit()
    codes = {"ocr": 30, "archcheck": 2}
    errors = []
    try:
        ocr.preview(base, candidate)
        with (ocr.OUTPUT / "summary.stdout.log").open("w") as log, redirect_stdout(log):
            codes["ocr"] = ocr.cli_review()
    except (OSError, ValueError, KeyError, subprocess.SubprocessError) as exc:
        errors.append("Open Code Review incomplete: " + type(exc).__name__)
    arch_output = output / "architecture"
    arch_output.mkdir(exist_ok=True)
    for name in ("latest-report.json", "latest-report.md"):
        p = arch_output / name
        if p.exists():
            p.rename(arch_output / (uuid.uuid4().hex + "-" + name))
    env = {
        k: v
        for k, v in os.environ.items()
        if k not in ("GH_TOKEN", "GITHUB_TOKEN") and not k.startswith("OCR_LLM_")
    }
    command = [
        sys.executable,
        "-I",
        str(arch / "inspect_project.py"),
        "--root",
        str(Path.cwd()),
        "--output",
        "harness-output/architecture",
        "--review-command",
        json.dumps([sys.executable, "-I", str(arch / "model_reviewer.py")]),
    ]
    for key in (
        "CODE_REVIEW_URL",
        "CODE_REVIEW_TOKEN",
        "CODE_REVIEW_MODEL",
        "HTTP_PROXY",
        "HTTPS_PROXY",
        "NO_PROXY",
    ):
        if key in env:
            command += ["--pass-env", key]
    try:
        with (
            (arch_output / "stdout.log").open("wb") as stdout,
            (arch_output / "stderr.log").open("wb") as stderr,
        ):
            codes["archcheck"] = subprocess.run(
                command, env=env, stdout=stdout, stderr=stderr, timeout=900
            ).returncode
    except (OSError, subprocess.SubprocessError) as exc:
        errors.append("archcheck incomplete: " + type(exc).__name__)
    report = {}
    try:
        report = json.loads((arch_output / "latest-report.json").read_text())
    except (OSError, ValueError):
        pass
    after = ocr.candidate_commit()
    unchanged = (
        before == ocr.candidate_snapshot()
        and ocr.git(Path("."), "rev-parse", candidate + "^{tree}")
        == ocr.git(Path("."), "rev-parse", after + "^{tree}")
        and tree_digest(arch) == args.arch_digest
    )
    verify_arch_parsers(arch)
    status = report.get("model_review", {}).get("status")
    code = decision(codes["ocr"], codes["archcheck"], status, unchanged)
    summary = {
        "candidate": candidate,
        "base": base,
        "source_inventory_sha256": hashlib.sha256(
            json.dumps(before, sort_keys=True).encode()
        ).hexdigest(),
        "source_unchanged": unchanged,
        "tool_exit_codes": codes,
        "architecture_model_status": status,
        "errors": errors,
        "exit_code": code,
    }
    (output / "review-pair.json").write_text(json.dumps(summary, indent=2) + "\n")
    print(json.dumps(summary))
    return code


if __name__ == "__main__":
    sys.exit(main())
