#!/usr/bin/env python3
"""Copyable Code Review graphs using ordinary execution and Agent nodes."""

import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "platform-workflows"))

from sandbox_delivery import (  # noqa: E402
    definition as delivery_definition,
)
from sandbox_delivery import (  # noqa: E402
    preparation_code,
    python_command,
)

ARTIFACTS = [
    "preview.json",
    "rules.json",
    "source.json",
    "raw.json",
    "result.json",
    "report.md",
    "raw.stderr.log",
]


def program(tail):
    return (HERE / "runtime.py").read_text() + "\n" + tail + "\n"


def rule(target):
    return {
        "schema_scope": "value",
        "rules": [
            {
                "when": "$ >= 0",
                "target": target,
                "title": "查看审查结果",
                "content": "流程结束不代表审查通过，结论以完整报告为准。",
            }
        ],
        **{
            name: {"action": "stop"}
            for name in ("on_read", "on_parse", "on_invalid", "on_expression")
        },
    }


def execution(name, code, *, refs=None, artifacts=False):
    return {
        "id": name,
        "name": name,
        "kind": "execution",
        "execution_timeout_seconds": 1500,
        "execution": {
            "mode": "command",
            "command": python_command(code),
            "output": {"source": "exit_code"},
            "env_refs": refs or {},
            "artifacts": ["harness-output/code-review/" + p for p in ARTIFACTS]
            if artifacts
            else [],
        },
    }


def definition(repository, credential_ref, engine, skill="", model_refs=None, proxy=""):
    if engine not in ("cli", "delegate"):
        raise ValueError("choose cli or delegate")
    if engine == "delegate" and not Path(skill).is_absolute():
        raise ValueError("choose the installed official Delegate Skill directory")
    if engine == "cli" and set(model_refs or {}) != {
        "OCR_LLM_URL",
        "OCR_LLM_TOKEN",
        "OCR_LLM_MODEL",
        "OCR_LLM_PROTOCOL",
    }:
        raise ValueError("CLI requires four private model connection references")
    if any(
        not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", ref)
        for ref in (model_refs or {}).values()
    ):
        raise ValueError("model references must be environment variable names")
    original = delivery_definition(repository, credential_ref, proxy)
    prepare = original["nodes"][0]
    prepare["execution_timeout_seconds"] = 600
    code = preparation_code(repository) + program(
        "context=json.loads(Path(os.environ['WORKFLOW_INPUT_FILE']).read_text())\n"
        "(STATE).mkdir(parents=True,exist_ok=True)\n"
        f"(STATE/'rules.json').write_text({(HERE / 'rules.json').read_text()!r})\n"
        "preview(context['parameters']['review_base_commit'],context['parameters']['repository_commit'])"
    )
    code += "\nprint('CODE_REVIEW_SOURCE_SHA256='+hashlib.sha256((OUTPUT/'source.json').read_bytes()).hexdigest())\n"
    prepare["execution"]["command"] = python_command(code)
    prepare["rule_handoff"] = {
        **rule("review"),
        "rules": [
            {"when": "$ == 0", "target": "review", "title": "准备完成", "content": ""}
        ],
    }
    if engine == "cli":
        review = execution(
            "review",
            program("raise SystemExit(cli_review())"),
            refs={
                **{
                    key: value
                    for key, value in prepare["execution"]["env_refs"].items()
                    if key != "GITHUB_TOKEN"
                },
                **model_refs,
            },
            artifacts=True,
        )
    else:
        review = {
            "id": "review",
            "name": "Codex · 独立代码审查",
            "kind": "agent",
            "execution_timeout_seconds": 1800,
            "agent": {
                "executor": "codex",
                "model": "gpt-6.1-sol",
                "instructions": (HERE / "delegate.md").read_text(),
                "skills": [skill],
                "sandbox": "workspace-write",
                "network_access": True,
                "allow_elevation": False,
                "approvals_reviewer": "auto_review",
                "inherit_env": False,
                "env": {
                    "PATH": "/workspace/.git/agent-platform-review:/opt/agent-runtime/node_modules/.bin:/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"
                },
            },
        }
    review["name"] = "OCR CLI · 独立代码审查" if engine == "cli" else review["name"]
    nodes, edges = [prepare, review], []
    if engine == "delegate":
        validate = execution(
            "validate", program("raise SystemExit(delegate_finish())"), artifacts=True
        )
        validate["rule_handoff"] = rule("done")
        nodes.append(validate)
        edges.append(
            {
                "source": "review",
                "route": "next",
                "target": "validate",
                "mode": "handoff",
            }
        )
    else:
        review["rule_handoff"] = rule("done")
    nodes.append({"id": "done", "name": "结束", "kind": "end"})
    for i, node in enumerate(nodes):
        node.update(x=60 + i * 340, y=100)
        for j, route in enumerate(node.get("rule_handoff", {}).get("rules", [])):
            edges.append(
                {
                    "source": node["id"],
                    "route": f"rule_{j + 1}",
                    "target": route["target"],
                    "mode": "automatic",
                    "title": route["title"],
                    "description": route["content"],
                }
            )
    return {
        "name": f"Code Review · {repository} · {engine}",
        "enabled": True,
        "environment": "opensandbox",
        "context_version": 2,
        "entry": "prepare",
        "max_steps": 6,
        "nodes": nodes,
        "edges": edges,
    }


def add_cli_review(graph, model_refs):
    """Explicit template choice: fixed tests → OCR → QA, findings → development."""
    import copy

    graph = copy.deepcopy(graph)
    nodes = {n["id"]: n for n in graph["nodes"]}
    if "code_review" in nodes or not {"tests", "development", "qa"}.issubset(nodes):
        raise ValueError(
            "expected a development graph without an installed review node"
        )
    code = program(
        "import uuid\n"
        "STATE.mkdir(parents=True,exist_ok=True)\n"
        "if OUTPUT.exists(): OUTPUT.rename(STATE/('previous-'+uuid.uuid4().hex))\n"
        "context=json.loads(Path(os.environ['WORKFLOW_INPUT_FILE']).read_text())\n"
        f"(STATE/'rules.json').write_text({(HERE / 'rules.json').read_text()!r})\n"
        "base=context['parameters']['repository_commit']\n"
        "head=candidate_commit()\n"
        "preview(base,head)\n"
        "raise SystemExit(cli_review())"
    )
    node = execution(
        "code_review",
        code,
        refs={
            **{
                key: value
                for key, value in nodes["prepare"]["execution"]["env_refs"].items()
                if key != "GITHUB_TOKEN"
            },
            **model_refs,
        },
        artifacts=True,
    )
    node.update(name="独立代码审查", x=nodes["qa"].get("x", 1000) - 120, y=340)
    node["rule_handoff"] = {
        **rule("qa"),
        "rules": [
            {
                "when": "$ == 0",
                "target": "qa",
                "title": "审查完成",
                "content": "覆盖完整且没有报告发现，继续独立 QA。",
            },
            {
                "when": "$ == 10",
                "target": "development",
                "title": "发现问题",
                "content": "读取审查报告，核实问题并修复或给出有依据的异议。",
            },
        ],
    }
    for route in nodes["tests"]["rule_handoff"]["rules"]:
        if route["target"] == "qa":
            route["target"] = "code_review"
    for edge in graph["edges"]:
        if edge["source"] == "tests" and edge["target"] == "qa":
            edge["target"] = "code_review"
    for i, route in enumerate(node["rule_handoff"]["rules"]):
        graph["edges"].append(
            {
                "source": "code_review",
                "route": f"rule_{i + 1}",
                "target": route["target"],
                "mode": "automatic",
                "title": route["title"],
                "description": route["content"],
            }
        )
    graph["nodes"].insert(
        next(i for i, n in enumerate(graph["nodes"]) if n["id"] == "qa"), node
    )
    return graph
