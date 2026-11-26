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


def configure(graph):
    result = copy.deepcopy(graph)
    result["name"] = "CC 分角色研发 · 双工具审查与独立验收"
    nodes = {node["id"]: node for node in result["nodes"] if node["id"] != "intake"}
    nodes["tests"]["name"] = "自动化回归"
    nodes["qa"]["name"] = "独立验收"
    nodes["approval"] = {
        "id": "approval",
        "name": "人工确认交付",
        "kind": "approval",
        "prompt": "核对自动化回归、双工具审查和独立验收的同版证据，确认后允许交付。",
    }
    for node_id, label in (
        ("code_review", "代码审查"),
        ("publish_check", "核对待发布源码"),
    ):
        nodes[node_id] = dict(
            id=node_id, name=label, kind="connector", connector_id=node_id
        )
    order = (
        "prepare",
        "issue",
        "requirements",
        "design",
        "development",
        "tests",
        "code_review",
        "qa",
        "approval",
        "report",
        "publish_check",
        "publish",
        "pr",
        "done",
    )
    result["nodes"] = [nodes[node_id] for node_id in order]
    for i, node in enumerate(result["nodes"]):
        node.update(x=60 + (i % 4) * 340, y=70 + (i // 4) * 260)

    result["edges"] = [
        {
            "source": source,
            "route": route,
            "target": target,
            "mode": "handoff" if nodes[source]["kind"] == "agent" else "automatic",
            "description": label,
        }
        for source, route, target, label in (
            ("prepare", "next", "issue", "工作区就绪"),
            ("issue", "next", "requirements", "任务已登记"),
            ("requirements", "next", "design", "需求与验收标准就绪"),
            ("design", "next", "development", "设计就绪"),
            ("development", "next", "tests", "实现完成"),
            ("tests", "next", "code_review", "回归通过"),
            ("code_review", "next", "qa", "双审查通过"),
            ("qa", "next", "approval", "验收通过"),
            ("approval", "next", "report", "同意交付"),
            ("report", "next", "publish_check", "交付说明就绪"),
            ("publish_check", "next", "publish", "源码与审查一致"),
            ("publish", "next", "pr", "任务分支已推送"),
            ("pr", "next", "done", "草稿 PR 已创建"),
            ("design", "requirements", "requirements", "补充需求"),
            ("development", "design", "design", "修订设计"),
            ("tests", "failed", "development", "回归失败"),
            ("qa", "development", "development", "验收问题需处理"),
            ("report", "qa", "qa", "补齐验收证据"),
        )
    ]
    return result
