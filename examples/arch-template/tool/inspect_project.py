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


def inspect_project(root, config, output):
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
            from archcheck.engine import analyze

            report, _ = analyze(root, config)
        except (
            OSError,
            ValueError,
            KeyError,
            TypeError,
            subprocess.SubprocessError,
        ) as error:
            report["errors"] = [str(error)]
        head = subprocess.run(
            ["git", "-C", str(root), "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
        )
        report["git_head"] = head.stdout.strip() if head.returncode == 0 else None
        report["checked_at"] = datetime.now(timezone.utc).isoformat()
        report["scope_note"] = (
            "Only the declared architecture scope was checked. Model review and product acceptance were not performed."
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
            "## 发现",
            "",
        ]
        for finding in report.get("findings", []):
            lines.append("- " + json.dumps(finding, ensure_ascii=False))
        for error in report.get("errors", []):
            lines.append("- 分析未完成：" + error)
        if not report.get("findings") and not report.get("errors"):
            lines.append("声明范围内未发现规则违规。")
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
    code = {"conformant": 0, "violations": 1}.get(report["status"], 2)
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
    args = parser.parse_args()
    try:
        result, code = inspect_project(args.root, args.config, args.output)
    except (ValueError, OSError) as error:
        result, code = {"status": "incomplete", "errors": [str(error)]}, 2
    print(json.dumps(result, ensure_ascii=False))
    return code


if __name__ == "__main__":
    raise SystemExit(main())
