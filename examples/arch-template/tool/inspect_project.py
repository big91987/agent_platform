#!/usr/bin/env python3
"""Retain one source-bound architecture inspection per invocation."""

import argparse
import fcntl
import json
import subprocess
import sys
import uuid
from contextlib import ExitStack
from datetime import datetime, timezone
from pathlib import Path


def inspect_project(
    root, config, output, review_command=None, review_timeout=660, pass_env=()
):
    root = root.resolve(strict=True)
    if output.is_absolute() or ".." in output.parts:
        raise ValueError("output must be a repository-relative directory")
    folder = root / output
    folder.mkdir(parents=True, exist_ok=True)
    if not folder.resolve().is_relative_to(root):
        raise ValueError("output escapes repository")
    folder = folder / uuid.uuid4().hex
    folder.mkdir()
    report = {"status": "incomplete", "model_review": {"status": "not_run"}}
    with ExitStack() as resources:
        try:
            manifest = Path(__file__).parent / "SOURCE.json"
            if manifest.exists():
                lock = resources.enter_context(manifest.open("rb"))
                fcntl.flock(lock, fcntl.LOCK_SH | fcntl.LOCK_NB)
            sys.path.insert(0, str(Path(__file__).resolve().parent))
            from archcheck.engine import IGNORED, analyze, load_config

            if review_command is not None:
                declared = load_config(root, config)
                for source_root in declared["source_roots"]:
                    selected = (root / source_root).resolve()
                    if folder.is_relative_to(selected) and not any(
                        part in IGNORED for part in folder.relative_to(selected).parts
                    ):
                        raise ValueError(
                            "review output inside source roots must use the reserved harness-output or .harness-output directory"
                        )
            report, texts = analyze(root, config)
            if review_command is not None:
                from archcheck.review import execute, make_request, validate

                request = make_request(report, texts, config, root)
                if len(json.dumps(request).encode()) > 2_000_000:
                    raise ValueError(
                        "review bundle exceeds 2 MB; narrow scope explicitly"
                    )
                (folder / "review-input.json").write_text(
                    json.dumps(request, ensure_ascii=False, indent=2) + "\n"
                )
                result = execute(
                    root,
                    request,
                    review_command,
                    review_timeout,
                    pass_env,
                    output_directory=folder,
                )
                (folder / "model-result.json").write_text(
                    json.dumps(result, ensure_ascii=False, indent=2) + "\n"
                )
                report = validate(root, request, result)
        except (
            OSError,
            ValueError,
            KeyError,
            TypeError,
            subprocess.SubprocessError,
        ) as error:
            report["status"] = "incomplete"
            if review_command is not None:
                report["model_review"] = dict(status="incomplete")
            report["errors"] = [str(error)]
        head = subprocess.run(
            ["git", "-C", str(root), "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
        )
        report["git_head"] = head.stdout.strip() if head.returncode == 0 else None
        report["checked_at"] = datetime.now(timezone.utc).isoformat()
        report["scope_note"] = (
            "Static architecture review of declared source and context; product acceptance was not performed."
            if review_command is not None
            else "Only declared dependency rules were checked; model review and product acceptance were not performed."
        )
        raw = folder / "report.json"
        raw.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        lines = [
            "# 架构检查",
            "",
            f"结果：{report['status']}",
            f"Git HEAD：{report['git_head'] or '无'}",
            f"源码摘要：{report.get('snapshot', '未生成')}",
            "",
            "仅检查 architecture.json 声明的范围；规则通过不代表领域划分合理或产品验收通过。",
            "",
            "## 规则扫描",
            "",
            "结果：" + report.get("rules", {}).get("status", "incomplete"),
            "",
        ]
        for finding in report.get("rules", {}).get("findings", []):
            lines.append("- " + json.dumps(finding, ensure_ascii=False))
        for error in report.get("errors", []):
            lines.append("- 分析未完成：" + error)
        if not report.get("rules", {}).get("findings") and not report.get("errors"):
            lines.append("声明范围内未发现规则违规。")
        model = report.get("model_review", {})
        lines += ["", "## 模型评审", "", "结果：" + model.get("status", "not_run"), ""]
        for check in model.get("checks", []):
            lines.append(
                "- " + check["name"] + "：" + check["status"] + " — " + check["reason"]
            )
        for finding in model.get("findings", []):
            lines += ["", "### " + finding["severity"] + " · " + finding["title"], ""]
            for key in ("principle", "impact", "recommendation", "validation"):
                lines.append(key + "：" + finding[key])
            for evidence in finding["evidence"]:
                lines += [
                    "",
                    f"{evidence['path']}:{evidence['line']}-{evidence['end']}",
                    "```",
                    evidence["quote"],
                    "```",
                ]
        for key in ("scope_notes", "blockers"):
            for note in model.get(key, []):
                lines.append("- " + key + "：" + note)
        lines += [
            "",
            "## 检查范围",
            "",
            "```json",
            json.dumps(report.get("coverage", {}), ensure_ascii=False, indent=2),
            "```",
            "",
        ]
        summary = folder / "report.md"
        summary.write_text("\n".join(lines))
        # Workflow archives accept fixed regular files, and retain each step
        # independently. UUID copies also preserve repeated local inspections.
        for name, source in [
            ("latest-report.json", raw),
            ("latest-report.md", summary),
        ]:
            target = folder.parent / name
            if target.is_symlink():
                raise ValueError("latest report must not be a symlink")
            temporary = folder / name
            temporary.write_bytes(source.read_bytes())
            temporary.replace(target)
    code = {"conformant": 0, "violations": 1, "review_required": 1}.get(
        report["status"], 2
    )
    return {
        "status": report["status"],
        "snapshot": report.get("snapshot"),
        "report": raw.relative_to(root).as_posix(),
        "summary": summary.relative_to(root).as_posix(),
    }, code


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("."))
    parser.add_argument("--config", default="architecture.json")
    parser.add_argument(
        "--output", type=Path, default=Path("harness-output/architecture")
    )
    parser.add_argument(
        "--review-command",
        help="JSON argv of the trusted provider-independent reviewer",
    )
    parser.add_argument("--review-timeout", type=float, default=660)
    parser.add_argument("--pass-env", action="append", default=[])
    args = parser.parse_args()
    try:
        result, code = inspect_project(
            args.root,
            args.config,
            args.output,
            json.loads(args.review_command)
            if args.review_command is not None
            else None,
            args.review_timeout,
            args.pass_env,
        )
    except (ValueError, OSError) as error:
        result, code = {"status": "incomplete", "errors": [str(error)]}, 2
    print(json.dumps(result, ensure_ascii=False))
    return code


if __name__ == "__main__":
    raise SystemExit(main())
