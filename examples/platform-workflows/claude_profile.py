"""Explicit local Claude role profile; preserves the generic workflow primitives."""

import copy
import re
from pathlib import Path

from asset_snapshot import tree_digest

HERE = Path(__file__).resolve().parent


def role_skills(root, qa):
    return {
        "requirements": [
            root / "defining-platform-products-cn",
            root / "batch-grill-me",
        ],
        "design": [root / "platform-architecture-cn"],
        "development": [],
        "qa": [qa],
        "report": [],
    }


def review_connector(arch_tool_root, model_prefix, arch_digest):
    root = Path(arch_tool_root).resolve(strict=True)
    for name in ("inspect_project.py", "model_reviewer.py"):
        if not (root / name).is_file():
            raise ValueError("architecture tool installation is incomplete")
    if not re.fullmatch("[A-Z][A-Z0-9_]*", model_prefix):
        raise ValueError("model prefix must be an environment variable prefix")
    refs = {
        key: model_prefix + "_" + suffix
        for key, suffix in {
            "OCR_LLM_URL": "URL",
            "OCR_LLM_TOKEN": "TOKEN",
            "OCR_LLM_MODEL": "MODEL",
            "OCR_LLM_PROTOCOL": "PROTOCOL",
            "CODE_REVIEW_URL": "URL",
            "CODE_REVIEW_TOKEN": "TOKEN",
            "CODE_REVIEW_MODEL": "MODEL",
        }.items()
    }
    return [
        str(HERE / "review_pair.py"),
        "--arch-tool-root",
        str(root),
        "--arch-digest",
        arch_digest or tree_digest(root),
    ], refs


def configure(graph, common=None):
    result = copy.deepcopy(graph)
    result["name"] = "CC 分角色研发 · 双工具审查与独立QA"
    # Explicit role separation without an additional deciding Agent.
    result["nodes"] = [n for n in result["nodes"] if n["id"] != "intake"]
    result["edges"] = [e for e in result["edges"] if e["source"] != "intake"]
    for edge in result["edges"]:
        if edge["target"] == "intake":
            edge["target"] = "requirements"
        if edge["source"] == "tests" and edge["target"] == "qa":
            edge["target"] = "code_review"
        if edge["source"] == "qa" and edge["target"] == "report":
            edge["target"] = "approval"
    for node in result["nodes"]:
        if node["id"] == "design":
            node["agent"]["instructions"] += (
                "\n架构约束使用项目 architecture.json；依据真实语言和已确认设计声明范围，不套用不适用的前端模板。此契约供 archcheck 核对。"
            )
        if node["id"] == "qa":
            node["agent"]["instructions"] += (
                "\n\n" + (HERE / "prompts/qa-saas-maas.md").read_text()
            )
            node["agent"]["instructions"] += (
                "\n同时复核 harness-output/code-review/report.md 和 harness-output/architecture/latest-report.md 两套真实报告；未关闭的有效发现返回开发，工具未执行或不完整不能判通过。"
            )
    qa = next(node for node in result["nodes"] if node["id"] == "qa")
    qa_scope = (HERE / "prompts/qa-saas-maas.md").read_text()
    if common is None:
        common = (HERE / "prompts/common.md").read_text()
    evidence_versions = (
        "每次计划、评审和执行交接都使用本次 Run 文档根目录下的新证据路径。"
        "使用递增轮次或其他可区分的文件名或子目录，保留历史文件，不覆盖前版。"
        "交接列出本轮实际新增文件，后续按最新交接读取。"
    )
    for role, label, instructions in (
        (
            "qa_plan",
            "制定 QA 计划",
            "只制定可执行计划，提交计划路径、SHA-256 和依据给独立评审。禁止执行用例。",
        ),
        (
            "qa_plan_review",
            "独立评审 QA 计划",
            "只评审最新计划和摘要。逐项对照 PRD、AC、设计及业务规则。缺项、规则不明或实质改动须走 revise；明确 READY 才走 next，交接固定计划路径和 SHA-256。禁止代写计划或执行用例。",
        ),
    ):
        stage = copy.deepcopy(qa)
        stage.update(id=role, name=label)
        stage["agent"]["instructions"] = "\n\n".join(
            (common, instructions, evidence_versions, qa_scope)
        )
        result["nodes"].append(stage)
    qa["agent"]["instructions"] += (
        "\n执行前核对独立评审的 READY 交接和计划 SHA-256。"
        "缺失、摘要不符或实质改变范围和断言时走 replan，重新独立评审后再执行。"
        "\n" + evidence_versions
    )
    for edge in result["edges"]:
        if edge["source"] == "report" and edge["target"] == "qa":
            edge["target"] = "qa_plan"
        if edge["source"] == "report" and edge["target"] == "publish":
            edge["target"] = "publish_check"
    for source, route, target in (
        ("qa_plan", "next", "qa_plan_review"),
        ("qa_plan_review", "next", "qa"),
        ("qa_plan_review", "revise", "qa_plan"),
        ("qa", "replan", "qa_plan"),
    ):
        result["edges"].append(
            dict(source=source, route=route, target=target, mode="handoff")
        )
    for node_id, label, target in (
        ("code_review", "代码审查 · OCR + archcheck", "qa_plan"),
        ("publish_check", "核对待发布源码", "publish"),
    ):
        result["nodes"].append(
            dict(id=node_id, name=label, kind="connector", connector_id=node_id)
        )
        result["edges"].append(
            dict(source=node_id, route="next", target=target, mode="automatic")
        )
    result["nodes"].append(
        {
            "id": "approval",
            "name": "核对同版证据后交付",
            "kind": "approval",
            "prompt": "核对固定测试、双工具审查、独立QA与同版产物后允许发布。",
        }
    )
    result["edges"].append(
        dict(source="approval", route="next", target="report", mode="automatic")
    )
    order = [
        "prepare",
        "issue",
        "requirements",
        "design",
        "development",
        "tests",
        "code_review",
        "qa_plan",
        "qa_plan_review",
        "qa",
        "approval",
        "report",
        "publish_check",
        "publish",
        "pr",
        "done",
    ]
    result["nodes"].sort(key=lambda n: order.index(n["id"]))
    for i, node in enumerate(result["nodes"]):
        node.update(x=60 + (i % 4) * 320, y=70 + (i // 4) * 230)
    return result
