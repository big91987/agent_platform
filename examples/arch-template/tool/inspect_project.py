#!/usr/bin/env python3
"""Retain one source-bound architecture inspection per invocation."""

import argparse
import fcntl
import json
import os
import subprocess
import sys
import uuid
from contextlib import ExitStack
from datetime import datetime, timezone
from pathlib import Path

RULE_NAMES = {
    "dependency": "依赖方向",
    "forbidden_import": "禁止的依赖",
    "external_dependency": "第三方依赖",
    "private_entry": "绕过公开入口",
    "cycle": "循环依赖",
}


def location(report, path, line, end=None):
    end = end or line
    span = str(line) if line == end else f"{line}-{end}"
    names = [
        symbol["name"]
        for file in report.get("facts", {}).get("files", [])
        if file["path"] == path
        for symbol in file.get("symbols", [])
        if symbol["line"] <= line and end <= symbol["end"]
    ]
    label = f"`{path}:{span}`"
    if names:
        label += " · " + "、".join(f"`{name}`" for name in names)
    return label


def render_score(report):
    from archcheck.scoring import score_report

    score = score_report(report)
    complete = score["completeness"]
    total = f"{score['total']:.1f} / 100" if score["total"] is not None else "暂不评分"
    lines = [
        "## 架构评分（模型初评）",
        "",
        f"**{total}** · 规则违规 {score['rule_violations']} 项 · 逐项评审 {complete['completed']} / {complete['expected']}。",
        f"固定检查点评审：{complete['criteria_completed']} / {complete['criteria_expected']}。",
        "",
        score["formula"],
        "",
        score["note"],
        "完成度依据模型逐项回报与引用核验，不是独立阅读或产品测试覆盖率。额外场景检查单列，不改变七个基础维度权重。",
        "",
        "| 维度 | 得分 | 判断与扣分依据 |",
        "| --- | ---: | --- |",
    ]
    for dimension in score["dimensions"]:
        points = dimension["points"]
        display = (
            f"{points} / 100"
            if points is not None
            else ("不适用" if dimension["status"] == "not_applicable" else "未完成")
        )
        basis = [dimension["reason"]]
        if points is not None and points < 100:
            basis.append("维度分为下列适用检查点的平均值；单个问题不自动使整维度归零。")
        seen = set()
        for deduction in dimension["deductions"]:
            key = (deduction["source"], deduction["finding"])
            if key in seen:
                continue
            seen.add(key)
            origin = "规则违规" if deduction["source"] == "rule" else "模型发现"
            severity = {"low": "轻微", "medium": "中度", "high": "严重"}[
                deduction["severity"]
            ]
            refs = "、".join(
                location(report, e["path"], e["line"], e.get("end"))
                for e in deduction["evidence"]
            )
            basis.append(
                f"{origin} {deduction['finding']}：{deduction['title']}（{severity}）；{refs}"
            )
        cell = "<br>".join(s.replace("|", "\\|").replace("\n", " ") for s in basis)
        lines.append(f"| {dimension['label']} | {display} | {cell} |")
    lines += [
        "",
        "### 固定检查点与计分依据",
        "",
        "| 维度 | 检查点 | 得分 | 判断、证据与关联问题 |",
        "| --- | --- | ---: | --- |",
    ]
    for dimension in score["dimensions"]:
        for point in dimension["criteria"]:
            value = point["points"]
            display = (
                f"{value} / 100"
                if value is not None
                else ("不适用" if point["status"] == "not_applicable" else "未完成")
            )
            basis = [point["reason"]]
            for d in point["deductions"]:
                origin = "规则违规" if d["source"] == "rule" else "模型发现"
                basis.append(f"{origin} {d['finding']}：{d['title']}")
            basis += [
                location(report, e["path"], e["line"], e.get("end"))
                for e in point["evidence"]
            ]
            cell = "<br>".join(s.replace("|", "\\|").replace("\n", " ") for s in basis)
            lines.append(
                f"| {dimension['label']} | {point['label']} | {display} | {cell} |"
            )
    lines += ["", "**评分档位（每个检查点）**", ""]
    for level, band in score["rubric"].items():
        label = {
            "conformant": "符合",
            "low": "轻微偏差",
            "medium": "中度偏差",
            "high": "严重偏差",
        }[level]
        lines.append(f"- {band['points']} 分 · {label}：{band['description']}")
    for reason in score["reasons"]:
        lines += ["", "暂不评分原因：" + reason]
    return lines + ["", f"评分口径版本：{score['rubric_version']}。", ""]


def render(report):
    rules = report.get("rules", {})
    model = report.get("model_review", {})
    findings = model.get("findings", [])
    rule_status = {
        "conformant": "未发现规则违规（仅限已声明的规则）",
        "violations": "存在规则违规",
    }.get(rules.get("status"), "规则检查未完成")
    model_status = {
        "not_run": "未执行模型评审",
        "no_findings": "模型未提出候选问题",
        "findings": f"发现 {len(findings)} 项待复核的架构问题",
        "incomplete": "模型评审未完成",
    }.get(model.get("status"), "未执行模型评审")
    lines = [
        "# 代码与架构规范检查",
        "",
        f"- 规则扫描：{rule_status}。",
        f"- 模型评审：{model_status}。",
        "",
        "规则检查依据已声明的依赖约束；模型意见需复核。本报告不判定产品业务功能通过或失败。",
        "",
    ]
    from archcheck.structure import render_structure

    lines += render_score(report) + render_structure(report) + ["## 规则违规", ""]
    for finding in rules.get("findings", []):
        name = RULE_NAMES.get(finding["rule"], finding["rule"])
        lines += [
            f"- **{name}** · " + location(report, finding["source"], finding["line"]),
            "",
            "  " + finding["message"],
            "",
        ]
    if not rules.get("findings"):
        lines.append(
            "声明范围内未发现规则违规。"
            if rules.get("status") == "conformant"
            else "没有可列出的规则违规；不能据此判定通过。"
        )
    for error in report.get("errors", []):
        lines += ["", "检查未完成：" + error]
    lines += ["", "## 模型架构发现（待复核）", ""]
    for index, finding in enumerate(findings, 1):
        severity = {"high": "高", "medium": "中", "low": "低"}.get(
            finding["severity"], finding["severity"]
        )
        lines += [
            f"### {index}. {finding['title']}（{severity}）",
            "",
            "**涉及位置与源码对象**",
            "",
        ]
        for evidence in finding["evidence"]:
            lines.append(
                "- "
                + location(report, evidence["path"], evidence["line"], evidence["end"])
            )
        for label, key in (
            ("问题依据", "principle"),
            ("影响", "impact"),
            ("调整建议", "recommendation"),
            ("复查建议", "validation"),
        ):
            lines += ["", f"**{label}**", "", finding[key]]
        lines += ["", "**原文证据**", ""]
        for evidence in finding["evidence"]:
            # Indent source so embedded Markdown fences cannot hide later findings.
            lines += [
                location(report, evidence["path"], evidence["line"], evidence["end"]),
                "",
                *("    " + line for line in evidence["quote"].splitlines()),
                "",
            ]
    if not findings:
        lines.append(model_status + "。")
    lines += ["", "## 逐项评审与限制", ""]
    status_names = {
        "conformant": "符合",
        "deviation": "存在偏差",
        "not_applicable": "不适用",
        "insufficient_evidence": "证据不足",
    }
    for check in model.get("checks", []):
        status = status_names.get(check["status"], check["status"])
        lines.append(f"- `{check['name']}`：{status}。{check['reason']}")
    for key, label in (("scope_notes", "范围说明"), ("blockers", "未完成原因")):
        for note in model.get(key, []):
            lines.append(f"- {label}：{note}")
    lines += [
        "",
        "## 执行记录与检查范围",
        "",
        f"工具版本：{report.get('tool_version', '未记录')}",
        f"Git HEAD：{report.get('git_head') or '无'}",
        f"源码摘要：{report.get('snapshot', '未生成')}",
        "",
        "```json",
        json.dumps(report.get("coverage", {}), ensure_ascii=False, indent=2),
        "```",
        "",
    ]
    return "\n".join(lines)


def inspect_project(
    root,
    config,
    output,
    review_command=None,
    review_timeout=660,
    pass_env=(),
    max_bytes=2_000_000,
    rules_only=False,
):
    if rules_only and review_command is not None:
        raise ValueError("rules-only cannot be combined with a model reviewer")
    if not rules_only and review_command is None:
        review_command = [
            sys.executable,
            "-I",
            str(Path(__file__).resolve().parent / "model_reviewer.py"),
        ]
        required = ("CODE_REVIEW_URL", "CODE_REVIEW_TOKEN", "CODE_REVIEW_MODEL")
        optional = tuple(
            k
            for k in ("CODE_REVIEW_MAX_TOKENS", "CODE_REVIEW_STREAM")
            if k in os.environ
        )
        pass_env = tuple(dict.fromkeys((*pass_env, *required, *optional)))
    if type(max_bytes) is not int or max_bytes <= 0:
        raise ValueError("max_bytes must be a positive integer")
    root = root.resolve(strict=True)
    if output.is_absolute() or ".." in output.parts:
        raise ValueError("output must be a repository-relative directory")
    folder = root / output
    folder.mkdir(parents=True, exist_ok=True)
    if not folder.resolve().is_relative_to(root):
        raise ValueError("output escapes repository")
    folder = folder / uuid.uuid4().hex
    folder.mkdir()
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    report = {"status": "incomplete", "model_review": {"status": "not_run"}}
    with ExitStack() as resources:
        try:
            manifest = Path(__file__).parent / "SOURCE.json"
            if manifest.exists():
                lock = resources.enter_context(manifest.open("rb"))
                fcntl.flock(lock, fcntl.LOCK_SH | fcntl.LOCK_NB)
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
                if len(json.dumps(request).encode()) > max_bytes:
                    raise ValueError(
                        "review bundle exceeds max bytes; no source was truncated"
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
        top = subprocess.run(
            ["git", "-C", str(root), "rev-parse", "--show-toplevel"],
            capture_output=True,
            text=True,
        )
        report["git_head"] = (
            head.stdout.strip()
            if head.returncode == 0
            and top.returncode == 0
            and Path(top.stdout.strip()).resolve() == root
            else None
        )
        report["max_input_bytes"] = max_bytes
        report["checked_at"] = datetime.now(timezone.utc).isoformat()
        report["scope_note"] = (
            "Static architecture review of declared source and context; product acceptance was not performed."
            if review_command is not None
            else "Only declared dependency rules were checked; model review and product acceptance were not performed."
        )
        from archcheck.scoring import score_report

        report["score"] = score_report(report)
        raw = folder / "report.json"
        raw.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        summary = folder / "report.md"
        summary.write_text(render(report))
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
        "score": report["score"]["total"],
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
    parser.add_argument("--max-bytes", type=int, default=2_000_000)
    parser.add_argument(
        "--rules-only",
        action="store_true",
        help="Explicitly skip model assessment and scoring",
    )
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
            args.max_bytes,
            args.rules_only,
        )
    except (ValueError, OSError) as error:
        result, code = {"status": "incomplete", "errors": [str(error)]}, 2
    print(json.dumps(result, ensure_ascii=False))
    return code


if __name__ == "__main__":
    raise SystemExit(main())
