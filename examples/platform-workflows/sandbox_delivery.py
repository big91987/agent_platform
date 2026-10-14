#!/usr/bin/env python3
"""Build the project-copyable OpenSandbox development/QA/Gate workflow example."""

import argparse
import json
import re
from pathlib import Path


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

    def agent(key, name, instructions, complete=False):
        return {
            "id": key,
            "name": name,
            "kind": "agent",
            "exit_mode": "complete" if complete else "handoff",
            "allow_user_input": True,
            "continuation_limit": 3,
            "execution_timeout_seconds": 1800,
            "agent": {
                "executor": "codex",
                "model": "gpt-6.1-sol",
                "instructions": instructions,
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
    prepare = (
        Path(__file__)
        .with_name("sandbox_prepare.py")
        .read_text()
        .split("if __name__ ==")[0]
        + f"\nprepare(json.loads(Path(os.environ['WORKFLOW_INPUT_FILE']).read_text()), {repository!r})\n"
    )
    prepare += "Path('harness-output').mkdir(exist_ok=True)\nwith Path('.git/info/exclude').open('a') as out:out.write('\\nharness-output/\\n')\n"
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
                "execution": {
                    "mode": "code",
                    "language": "python",
                    "code": prepare,
                    "env_refs": refs,
                },
            },
            agent(
                "development",
                "研发实现",
                "在当前共享工作区完成本任务。先读取项目AGENTS.md和冻结输入；只修改本Issue范围。不读取凭据，不执行push/PR/发布。归档真实文件和实施记录后使用原生handoff交给tests；收到QA返工或用户纠正时核对既有文件再继续，不重建工作区。固定tests失败先读完整回执定位，只有实质修复或新的可复验证据才再次交tests；同版重复失败且需修环境、凭据或冻结准备步骤时，用wait_for_input blocked明确影响、证据和下一步，不循环求绿。",
            ),
            {
                "id": "tests",
                "name": "固定项目测试",
                "kind": "execution",
                "execution_timeout_seconds": 1800,
                "execution": {
                    "mode": "command",
                    "command": "make verify",
                    "env_refs": {k: v for k, v in refs.items() if k != "GITHUB_TOKEN"},
                },
            },
            agent(
                "qa",
                "独立QA",
                "独立核对任务验收依据、研发改动、固定tests原始结果和具体产物。在同一工作区独立复查必要行为，但不能修改产品。失败或缺证据时写明影响/复现/返工要求，通过handoff交development。通过时把独立QA报告保存harness-output/qa.md，明确未验/限制/遗留，再handoff给approval。不把任务结束或局部测试当产品通过。",
            ),
            {
                "id": "approval",
                "name": "核对交付后允许发布",
                "kind": "approval",
                "prompt": "检查本版完整测试、独立QA与产物，确认后进入报告和草稿PR发布。",
            },
            agent(
                "report",
                "形成交付报告",
                "根据本Issue与同版固定tests/独立QA形成可独立阅读的PR报告。生成harness-output/report.json，字段title必须【类别】具体目标，body写明问题/实际改动/已验版本/结果/限制/真实Issue关联，files列本版经QA确认需要提交的仓库相对文件。不要修改产品、不要提交构建目录/凭据/原生历史。通过complete_node交付report.json及QA文件。",
                True,
            ),
            {
                "id": "publish",
                "name": "发布同版草稿PR",
                "kind": "execution",
                "execution_timeout_seconds": 240,
                "execution": {
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
                ("prepare", "next", "development", "automatic"),
                ("development", "tests", "tests", "handoff"),
                ("tests", "next", "qa", "automatic"),
                ("tests", "failed", "development", "automatic"),
                ("qa", "revise", "development", "handoff"),
                ("qa", "approved", "approval", "handoff"),
                ("approval", "next", "report", "automatic"),
                ("report", "next", "publish", "automatic"),
                ("publish", "next", "done", "automatic"),
            ]
        ],
    }
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
