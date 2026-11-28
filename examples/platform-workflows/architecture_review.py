#!/usr/bin/env python3
"""Execute model-backed architecture review on one unchanged candidate; never publish."""

import argparse
import hashlib
import importlib.util
import json
import os
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import uuid
from collections import Counter
from contextlib import chdir, contextmanager
from pathlib import Path

from asset_snapshot import tree_digest, verify_arch_parsers


def decision(arch_code, model_status, unchanged):
    if (
        not unchanged
        or arch_code not in (0, 1)
        or model_status not in ("no_findings", "findings")
    ):
        return 20
    return 10 if arch_code == 1 or model_status == "findings" else 0


def rule_identity(finding):
    return tuple(finding[field] for field in ("rule", "source", "target", "message"))


def finding_identity(finding):
    return (
        tuple(sorted(finding["checks"])),
        tuple(sorted((item["path"], item["quote"]) for item in finding["evidence"])),
    )


def compare_findings(base_input, candidate_input, candidate_report, base_report=None):
    same_input = base_input == candidate_input
    rules = Counter(
        rule_identity(finding) for finding in base_input["rules"]["findings"]
    )
    dispositions = []
    for finding in candidate_input["rules"]["findings"]:
        key = rule_identity(finding)
        existing = rules[key] > 0
        if existing:
            rules[key] -= 1
        dispositions.append(dict(
            kind="rule", disposition="existing" if existing else "new_or_extended",
            finding=finding,
        ))

    severity = {"low": 1, "medium": 2, "high": 3}
    baseline = {}
    same_standards = base_input["review_checks"] == candidate_input["review_checks"]
    if base_report and same_standards:
        for f in base_report["model_review"]["findings"]:
            key = finding_identity(f)
            baseline[key] = max(baseline.get(key, 0), severity[f["severity"]])
    for finding in candidate_report["model_review"]["findings"]:
        anchors = finding_identity(finding)
        source_counts = [
            base_input["sources"].get(path, "").count(quote) for path, quote in anchors[1]
        ]
        previous = baseline.get(anchors, 0)
        if same_input:
            value = "existing"
        elif 0 in source_counts:
            value = "new_or_extended"
        elif previous and all(count == 1 for count in source_counts):
            value = "worsened" if severity[finding["severity"]] > previous else "existing"
        else:
            # An unchanged quotation alone does not establish that a problem
            # existed before, or that a change elsewhere did not make it worse.
            value = "unresolved"
        dispositions.append(dict(kind="model", disposition=value, finding=finding))
    return dispositions


def review_feedback(run):
    """Hand back findings from the immediately preceding completed review."""
    steps = run["previous_results"]
    previous = steps[-1] if steps else {}
    receipt = previous.get("connector_receipt") or {}
    feedback = {"reason": "invalid_review_receipt"}

    if (
        previous.get("node_id") != "code_review"
        or previous.get("status") != "completed"
        or (previous.get("result") or {}).get("route") != "failed"
        or receipt.get("kind") != "command"
    ):
        return feedback

    feedback.update(
        review_seq=previous["seq"], review_exit_code=receipt.get("exit_code")
    )
    code = receipt.get("exit_code")
    if code != 10:
        return dict(feedback, reason="review_incomplete")
    if (receipt.get("log") or {}).get("truncated") is not False:
        return feedback

    try:
        summary = json.loads(receipt["output"])
        if not isinstance(summary, dict):
            return feedback
        codes = summary["tool_exit_codes"]
        if (
            summary.get("exit_code") != 10
            or summary.get("source_unchanged") is not True
            or type(summary.get("regressions")) is not int
            or summary["regressions"] <= 0
            or not isinstance(codes, dict)
            or set(codes) != {"archcheck"}
            or any(type(value) is not int for value in codes.values())
            or decision(
                codes["archcheck"],
                summary.get("architecture_model_status"),
                True,
            )
            != 10
            or any(
                not isinstance(summary.get(field), str)
                or not re.fullmatch("[0-9a-f]{" + str(length) + "}", summary[field])
                for field, length in (
                    ("candidate", 40),
                    ("base", 40),
                    ("source_inventory_sha256", 64),
                )
            )
        ):
            return feedback
    except (KeyError, ValueError, TypeError):
        return feedback

    feedback.update(
        candidate=summary["candidate"],
        base=summary["base"],
        source_inventory_sha256=summary["source_inventory_sha256"],
        tool_exit_codes=codes,
        architecture_model_status=summary["architecture_model_status"],
        regressions=summary["regressions"],
        report_files={
            "archcheck": "harness-output/architecture/latest-report.json",
            "comparison": "harness-output/architecture-disposition.json",
        },
    )
    return dict(feedback, reason="review_findings")


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
    head = value.get("integration_base", value.get("head", ""))
    if not isinstance(head, str) or not re.fullmatch("[0-9a-f]{40}", head):
        raise ValueError("preparation receipt lacks fixed head")
    return head


def load_candidate_runtime():
    source = Path(__file__).resolve().parents[1] / "code-review/runtime.py"
    spec = importlib.util.spec_from_file_location("trusted_candidate_runtime", source)
    ocr = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(ocr)
    return ocr


@contextmanager
def output_directory(parent, name):
    try:
        os.mkdir(name, mode=0o700, dir_fd=parent)
    except FileExistsError:
        pass
    fd = os.open(name, os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
    try:
        yield fd
    finally:
        os.close(fd)


def copy_artifacts(source, destination):
    # Only the private review processes have populated source. Recreate their
    # regular files in fresh entries, without ever opening a workspace target
    # for truncation or following an artifact link.
    for entry in source.iterdir():
        mode = entry.lstat().st_mode
        if stat.S_ISDIR(mode):
            with output_directory(destination, entry.name) as child:
                copy_artifacts(entry, child)
        elif stat.S_ISREG(mode):
            fd = os.open(
                entry.name,
                os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW,
                0o600,
                dir_fd=destination,
            )
            with os.fdopen(fd, "wb") as out, entry.open("rb") as original:
                shutil.copyfileobj(original, out)
        else:
            raise ValueError("Unexpected review artifact type")


def expose_artifacts(workspace_fd, output):
    with output_directory(workspace_fd, "harness-output") as dest:
        stage = ".review-" + uuid.uuid4().hex
        os.mkdir(stage, mode=0o700, dir_fd=dest)
        with output_directory(dest, stage) as staged:
            copy_artifacts(output, staged)
            for name in os.listdir(staged):
                try:
                    os.rename(
                        name,
                        "previous-" + uuid.uuid4().hex + "-" + name,
                        src_dir_fd=dest,
                        dst_dir_fd=dest,
                    )
                except FileNotFoundError:
                    pass
                os.rename(name, name, src_dir_fd=staged, dst_dir_fd=dest)
        os.rmdir(stage, dir_fd=dest)


def review(args, candidate_git, workspace, private):
    arch = args.arch_tool_root.resolve(strict=True)
    if tree_digest(arch) != args.arch_digest:
        raise ValueError("Installed architecture tool differs from its recorded digest")
    verify_arch_parsers(arch)
    context = json.load(sys.stdin)
    if Path(context["workspace_path"]).resolve() != workspace:
        raise ValueError("Run workspace and process directory differ")
    base = prepared_head(context)
    before = candidate_git.candidate_snapshot(workspace)
    candidate = candidate_git.candidate_commit(workspace)
    checkout = private / "candidate"
    checkout.mkdir()
    candidate_git.git(checkout, "init", "--quiet")
    candidate_git.git(
        checkout, "fetch", "--quiet", "--no-tags", "--", workspace.as_uri(), candidate
    )
    candidate_git.git(checkout, "checkout", "--quiet", "--detach", candidate)
    # The checkout can already track old runtime files, including links.
    # Move their top-level entry without traversing it before the reviewer writes.
    output = checkout / "harness-output"
    if os.path.lexists(output):
        output.rename(private / "tracked-output")
    output.mkdir(mode=0o700)
    with chdir(checkout):
        return run_archcheck(
            args, candidate_git, arch, output, workspace, base, before, candidate
        )


def inspect_architecture(arch, checkout, output):
    arch_exit_code = 2
    errors = []
    arch_output = output / "architecture"
    arch_output.mkdir(exist_ok=True)
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
        str(checkout),
        "--output",
        arch_output.relative_to(checkout).as_posix(),
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
            arch_exit_code = subprocess.run(
                command, env=env, stdout=stdout, stderr=stderr, timeout=900
            ).returncode
    except (OSError, subprocess.SubprocessError) as exc:
        errors.append("archcheck incomplete: " + type(exc).__name__)
    report = {}
    try:
        report = json.loads((arch_output / "latest-report.json").read_text())
    except (OSError, ValueError):
        pass
    return arch_exit_code, report, errors


def compare_review(arch, checkout, output, candidate_git, base, report):
    inputs = list((output / "architecture").glob("*/review-input.json"))
    if len(inputs) != 1:
        raise ValueError("Expected one candidate architecture input")
    candidate_input = json.loads(inputs[0].read_text())
    if candidate_input["snapshot"] != report["snapshot"]:
        raise ValueError("Architecture report does not belong to the candidate input")

    baseline = checkout.parent / "baseline"
    candidate_git.git(checkout, "worktree", "add", "--detach", str(baseline), base)
    baseline_input = json.loads(subprocess.check_output(
        [sys.executable, "-I", str(arch / "archcheck.py"), "bundle", "--root", str(baseline)],
        timeout=120,
    ))
    if baseline_input.get("kind") != "architecture-review":
        raise ValueError("Architecture baseline input failed")
    same_input = baseline_input == candidate_input
    baseline_report = None
    if not same_input and report["model_review"]["findings"]:
        baseline_output = baseline / "harness-output"
        if os.path.lexists(baseline_output):
            baseline_output.rename(baseline.parent / "tracked-baseline-output")
        code, baseline_report, errors = inspect_architecture(arch, baseline, baseline_output)
        if (errors or baseline_report.get("git_head") != base
                or baseline_report.get("snapshot") != baseline_input["snapshot"]
                or decision(code, baseline_report.get("model_review", {}).get("status"), True) == 20):
            raise ValueError("Architecture baseline review is incomplete")
        (output / "architecture-baseline.json").write_text(
            json.dumps(baseline_report, ensure_ascii=False, indent=2) + "\n"
        )

    findings = compare_findings(baseline_input, candidate_input, report, baseline_report)
    counts = Counter(finding["disposition"] for finding in findings)
    evidence = {
        "base": base, "candidate": report["git_head"], "same_input": same_input,
        "base_snapshot": baseline_input["snapshot"], "candidate_snapshot": candidate_input["snapshot"],
        "findings": findings,
    }
    (output / "architecture-disposition.json").write_text(
        json.dumps(evidence, ensure_ascii=False, indent=2) + "\n"
    )
    return dict(
        baseline_findings=counts["existing"],
        regressions=counts["new_or_extended"] + counts["worsened"],
        unresolved=counts["unresolved"],
        baseline_input_identical=same_input,
    )


def run_archcheck(args, candidate_git, arch, output, workspace, base, before, candidate):
    checkout = Path.cwd()
    frozen = candidate_git.candidate_snapshot()
    arch_exit_code, report, errors = inspect_architecture(arch, checkout, output)
    status = report.get("model_review", {}).get("status")
    code = decision(arch_exit_code, status, report.get("git_head") == candidate)
    comparison = {}
    if code == 10:
        try:
            comparison = compare_review(arch, checkout, output, candidate_git, base, report)
        except (OSError, KeyError, ValueError, TypeError, subprocess.SubprocessError) as exc:
            errors.append("Architecture comparison incomplete: " + str(exc))
            code = 20
        else:
            if comparison["regressions"]:
                code = 10
            elif comparison["unresolved"]:
                code = 20
            else:
                code = 0

    after = candidate_git.candidate_commit(workspace)
    unchanged = (
        frozen == candidate_git.candidate_snapshot()
        and before == candidate_git.candidate_snapshot(workspace)
        and candidate_git.git(workspace, "rev-parse", candidate + "^{tree}")
        == candidate_git.git(workspace, "rev-parse", after + "^{tree}")
        and tree_digest(arch) == args.arch_digest
    )
    verify_arch_parsers(arch)
    if not unchanged:
        code = 20
    summary = {
        "candidate": candidate,
        "base": base,
        "source_inventory_sha256": hashlib.sha256(
            json.dumps(before, sort_keys=True).encode()
        ).hexdigest(),
        "source_unchanged": unchanged,
        "tool_exit_codes": {"archcheck": arch_exit_code},
        "architecture_model_status": status,
        **comparison,
        "errors": errors,
        "exit_code": code,
    }
    (output / "review.json").write_text(json.dumps(summary, indent=2) + "\n")
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--arch-tool-root", type=Path, required=True)
    parser.add_argument("--arch-digest", required=True)
    args = parser.parse_args()
    workspace = Path.cwd()
    fd = os.open(".", os.O_DIRECTORY)
    try:
        with tempfile.TemporaryDirectory(prefix="platform-review-") as tmp:
            private = Path(tmp).resolve()
            if private.is_relative_to(workspace):
                raise ValueError("Review attempt must be outside the Run workspace")
            summary = review(args, load_candidate_runtime(), workspace, private)
            expose_artifacts(fd, private / "candidate/harness-output")
    finally:
        os.close(fd)
    print(json.dumps(summary))
    return summary["exit_code"]


if __name__ == "__main__":
    sys.exit(main())
