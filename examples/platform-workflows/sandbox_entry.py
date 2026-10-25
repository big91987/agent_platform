#!/usr/bin/env python3
"""Forward a labelled Issue to its platform-owned sandbox Run; no host clone."""

import argparse
import json
import os
import re
import subprocess
import sys
import urllib.parse
from pathlib import Path

from agent_platform_client import APIError, Client
from github_entry import (
    accepted_run,
    configured_github_call,
    event_input,
    failure_diagnostic,
    github,
    issue_parameters,
    notify,
    platform_output,
    report_run,
)


def forward(config, client, number, comment_id=0, *, github_call=None):
    call = github_call or configured_github_call(config, github)
    repo = config["repository"]
    owner = repo.split("/")[0]
    label = config["issue_label"]
    if not isinstance(label, str) or not label.strip():
        raise ValueError("A dedicated Issue label is required")
    print("Sandbox entry: read current GitHub Issue", flush=True)
    issue = call(f"repos/{repo}/issues/{number}")
    if (
        "pull_request" in issue
        or issue["number"] != number
        or issue["user"]["login"] != owner
    ):
        raise ValueError("Expected an owner-authored Issue")
    if label not in [x["name"] for x in issue.get("labels", [])]:
        raise ValueError("Issue does not select this sandbox Pipeline")
    title = re.fullmatch(
        r"【(?:产品功能|产品修复|产品测试|Harness验证|Harness修复|交付维护)】([^\r\n]+)",
        issue["title"],
    )
    if not title or not title[1].strip() or platform_output(issue.get("body")):
        raise ValueError("Use a classified Issue with a concrete goal")
    key = f"github:{repo}:issue:{issue['id']}"
    print("Sandbox entry: resolve original platform Run", flush=True)
    run = accepted_run(client, key)
    if run and (
        run["workflow_id"] != config["workflow_id"]
        or run["definition"].get("environment") != "opensandbox"
        or run["workspace_path"] != "/workspace"
    ):
        raise ValueError("Accepted Run does not belong to this sandbox Pipeline")
    if comment_id:
        if not run:
            raise ValueError("Issue has no accepted sandbox Run")
        comment = call(f"repos/{repo}/issues/comments/{comment_id}")
        if (
            comment.get("issue_url")
            != f"https://api.github.com/repos/{repo}/issues/{number}"
            or comment["user"]["login"] != owner
            or comment["user"].get("type") != "User"
        ):
            raise ValueError("Comment does not belong to this authorized Issue")
        if platform_output(comment.get("body")):
            return run
        receipt = client.workflow_message(
            run["id"], comment["body"], request_id=key + f":comment:{comment_id}"
        )
        notify(
            repo,
            number,
            key + f":comment:{comment_id}",
            "补充已保存到当前 Agent 会话：[查看会话]("
            + receipt["conversation_url"]
            + ")。",
            github_call=call,
        )
    elif not run:
        if issue["state"] != "open":
            raise ValueError("Only an open Issue can start a new Run")
        ref = urllib.parse.quote(config.get("base", "main"), safe="")
        print("Sandbox entry: read fixed GitHub source", flush=True)
        commit = call(f"repos/{repo}/commits/{ref}")["sha"]
        if not re.fullmatch(r"[0-9a-f]{40}", commit):
            raise ValueError("GitHub did not return a fixed source commit")
        parameters = issue_parameters(issue, repo)
        parameters.update(repository=repo, repository_commit=commit)
        print("Sandbox entry: submit original Issue to platform", flush=True)
        run = client.start_workflow(
            config["workflow_id"],
            issue["title"]
            + f"\n\nIssue：https://github.com/{repo}/issues/{number}\n\n"
            + (issue.get("body") or ""),
            workspace_path="",
            request_id=key,
            parameters=parameters,
            expected_environment="opensandbox",
        )
    report_run(config, run)
    notify(
        repo,
        number,
        key + ":sandbox-accepted",
        "沙箱任务已受理：[查看运行进度]("
        + config["base_url"]
        + "/workflow-runs/"
        + run["id"]
        + ")。受理不代表测试完成；材料、当前卡点与恢复入口见运行页面。",
        github_call=call,
    )
    return run


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True, type=Path)
    args = parser.parse_args()
    config = json.loads(args.config.read_text())
    client = Client(config["base_url"], Path(config["token_file"]).read_text().strip())
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    incoming = event_input(config, os.environ, event)
    if incoming is None:
        return
    number, comment = incoming
    github_call = configured_github_call(config, github)
    try:
        forward(config, client, number, comment, github_call=github_call)
    except (
        APIError,
        ConnectionError,
        ValueError,
        subprocess.CalledProcessError,
        subprocess.TimeoutExpired,
    ) as error:
        # Publish no rejected command, private path, source input or response body.
        reason = (
            ("平台 HTTP " + str(error.status_code))
            if isinstance(error, APIError)
            else (
                "平台连接失败，受理结果尚需核对"
                if isinstance(error, ConnectionError)
                else (
                    failure_diagnostic(error)
                    if isinstance(error, subprocess.SubprocessError)
                    else "入站条件或材料不符合配置"
                )
            )
        )
        key = f"sandbox-entry:{config['repository']}:{number}:{comment}:rejected"
        notify(
            config["repository"],
            number,
            key,
            "沙箱任务入口被阻断："
            + reason
            + "。请查看本次 Actions 和平台状态；入口失败不代表测试结果，以原 Run 回执为准，不自动重新创建 Run。",
            github_call=github_call,
        )
        raise


if __name__ == "__main__":
    try:
        main()
    except (
        APIError,
        ConnectionError,
        ValueError,
        KeyError,
        subprocess.CalledProcessError,
        subprocess.TimeoutExpired,
    ) as error:
        if isinstance(error, APIError):
            message = (
                f"Platform API failed: HTTP {error.status_code} (response withheld)"
            )
        elif isinstance(error, ConnectionError):
            message = "Platform API failed: transport error (details withheld)"
        elif isinstance(error, subprocess.SubprocessError):
            message = failure_diagnostic(error)
        else:
            message = (
                "Sandbox entry input or configuration is invalid (details withheld)"
            )
        sys.exit(message)
