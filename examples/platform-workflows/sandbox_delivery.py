#!/usr/bin/env python3
"""Build the project-copyable OpenSandbox development/QA/Gate workflow example."""

import argparse
import json
import re
import shlex
from pathlib import Path


def python_command(code):
    """Trusted template programs must not import code from the reviewed workspace."""
    return shlex.join(["python3", "-I", "-c", code])


def preparation_code(repository):
    prepare = (
        Path(__file__)
        .with_name("sandbox_prepare.py")
        .read_text()
        .split("if __name__ ==")[0]
        + f"\nprepare(json.loads(Path(os.environ['WORKFLOW_INPUT_FILE']).read_text()), {repository!r})\n"
    )
    prepare += "Path('harness-output').mkdir(exist_ok=True)\nwith Path('.git/info/exclude').open('a') as out:out.write('\\nharness-output/\\n')\n"
    return prepare


def definition(repository, credential_ref, proxy="", no_proxy="127.0.0.1,localhost"):
    if not re.fullmatch(
        r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repository
    ) or not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", credential_ref):
        raise ValueError("choose repository and backend credential reference")
    env = {}
    if proxy:
        env = {
            key: proxy
            for key in ["http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY"]
        }
        env.update(
            no_proxy=no_proxy,
            NO_PROXY=no_proxy,
        )

    prompts = Path(__file__).with_name("prompts")
    common = (prompts / "sandbox-common.md").read_text().strip()

    def agent(key, name):
        return {
            "id": key,
            "name": name,
            "kind": "agent",
            "allow_user_input": True,
            "continuation_limit": 3,
            "execution_timeout_seconds": 1800,
            "agent": {
                "executor": "codex",
                "turn_timeout_seconds": {"development": 5400, "qa": 3600}.get(
                    key, 1800
                ),
                "model": "gpt-6.1-sol",
                "instructions": (prompts / ("sandbox-" + key + ".md"))
                .read_text()
                .strip()
                + "\n\n"
                + common,
                "sandbox": "workspace-write",
                "network_access": True,
                "allow_elevation": False,
                "approvals_reviewer": "auto_review",
                "inherit_env": False,
                "env": env,
            },
        }

    refs = {"GITHUB_TOKEN": credential_ref}
    if proxy:
        refs.update(
            {
                key: "PIPELINE_" + key.upper()
                for key in ["http_proxy", "https_proxy", "no_proxy"]
            }
        )
    graph = {
        "name": "OpenSandbox · " + repository + " 研发与独立QA",
        "enabled": True,
        "environment": "opensandbox",
        "context_version": 2,
        "entry": "prepare",
        "max_steps": 40,
        "nodes": [
            {
                "id": "prepare",
                "name": "准备固定仓库版本",
                "kind": "execution",
                "execution_timeout_seconds": 240,
                "rule_handoff": {
                    "schema_scope": "value",
                    "rules": [
                        {
                            "when": "$ == 0",
                            "title": "准备完成",
                            "content": "",
                            "target": "development",
                        }
                    ],
                    "on_read": {"action": "stop"},
                    "on_parse": {"action": "stop"},
                    "on_invalid": {"action": "stop"},
                    "on_expression": {"action": "stop"},
                },
                "execution": {
                    "output": {"source": "exit_code"},
                    "mode": "command",
                    "command": python_command(preparation_code(repository)),
                    "env_refs": refs,
                },
            },
            agent("development", "研发实现"),
            {
                "id": "tests",
                "name": "固定项目测试",
                "kind": "execution",
                "execution_timeout_seconds": 1800,
                "execution": {
                    "mode": "command",
                    "command": "make verify",
                    "output": {"source": "exit_code"},
                    "env_refs": {k: v for k, v in refs.items() if k != "GITHUB_TOKEN"},
                },
                "rule_handoff": {
                    "schema_scope": "value",
                    "rules": [
                        {
                            "when": "$ == 0",
                            "title": "测试通过",
                            "content": "命令退出码为 0，交给独立 QA 复核。",
                            "target": "qa",
                        },
                        {
                            "when": "$ != 0",
                            "title": "测试失败",
                            "content": "命令返回非零退出码，交给研发检查完整回执。",
                            "target": "development",
                        },
                    ],
                    "on_read": {"action": "stop"},
                    "on_parse": {"action": "stop"},
                    "on_invalid": {"action": "stop"},
                    "on_expression": {"action": "stop"},
                },
            },
            agent("qa", "独立QA"),
            {
                "id": "approval",
                "name": "核对交付后允许发布",
                "kind": "approval",
                "prompt": "检查本版完整测试、独立QA与产物，确认后进入报告和草稿PR发布。",
            },
            agent("report", "形成交付报告"),
            {
                "id": "publish",
                "name": "发布同版草稿PR",
                "kind": "execution",
                "execution_timeout_seconds": 240,
                "rule_handoff": {
                    "schema_scope": "value",
                    "rules": [
                        {
                            "when": "$ == 0",
                            "title": "发布完成",
                            "content": "",
                            "target": "done",
                        }
                    ],
                    "on_read": {"action": "stop"},
                    "on_parse": {"action": "stop"},
                    "on_invalid": {"action": "stop"},
                    "on_expression": {"action": "stop"},
                },
                "execution": {
                    "output": {"source": "exit_code"},
                    "mode": "command",
                    "command": "python3 .harness/agent-platform/examples/platform-workflows/sandbox_publish.py --repository "
                    + repository,
                    "env_refs": refs,
                },
            },
            {"id": "done", "name": "结束", "kind": "end"},
        ],
        "edges": [
            {"source": a, "route": route, "target": b, "mode": mode}
            for a, route, b, mode in [
                ("development", "tests", "tests", "handoff"),
                ("qa", "revise", "development", "handoff"),
                ("qa", "approved", "approval", "handoff"),
                ("approval", "next", "report", "automatic"),
                ("report", "next", "publish", "handoff"),
            ]
        ],
    }
    for node in graph["nodes"]:
        for index, rule in enumerate(node.get("rule_handoff", {}).get("rules", [])):
            graph["edges"].append(
                {
                    "source": node["id"],
                    "route": f"rule_{index + 1}",
                    "target": rule["target"],
                    "mode": "automatic",
                    "title": rule["title"],
                    "description": rule["content"],
                }
            )
    for index, node in enumerate(graph["nodes"]):
        node.update(x=60 + index * 320, y=100)
    return graph


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository", required=True)
    parser.add_argument("--credential-ref", required=True)
    parser.add_argument("--proxy", default="")
    parser.add_argument("--no-proxy", default="127.0.0.1,localhost")
    args = parser.parse_args()
    print(
        json.dumps(
            definition(args.repository, args.credential_ref, args.proxy, args.no_proxy),
            ensure_ascii=False,
            indent=2,
        )
    )
