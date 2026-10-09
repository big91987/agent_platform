#!/usr/bin/env python3
"""GitHub Runner adapter: user-token API and an account-login conversation link.

Configuration is local private JSON: base_url, agent_id, token.
The platform API token never enters Issue comments or GitHub artifacts.
"""

import argparse
import json
import os
import re
import subprocess
from pathlib import Path

from agent_platform_client import Client


def github(path, body=None, *, paginate=False, env=None):
    command = ["gh", "api", path]
    if paginate:
        command += ["--paginate", "--slurp"]
    if body is not None:
        command += ["--method", "POST", "--input", "-"]
    result = subprocess.run(
        command,
        input=None if body is None else json.dumps(body),
        text=True,
        capture_output=True,
        check=True,
        timeout=60,
        env=env,
    )
    value = json.loads(result.stdout) if result.stdout.strip() else None
    return [item for page in value for item in page] if paginate else value


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text())
    base = config["base_url"].rstrip("/")
    client = Client(base, config["token"])
    identity = client.me()
    print(f"Platform user: {identity['user_id']}", flush=True)

    repo, number = os.environ["GH_REPO"], os.environ["ISSUE_NUMBER"]
    issue = github(f"repos/{repo}/issues/{number}")
    if "pull_request" in issue:
        raise ValueError("Use a product Issue, not a pull request")
    # Durable external association; platform owns all native Session state.
    comments = github(
        f"repos/{repo}/issues/{number}/comments?per_page=100", paginate=True
    )
    marker = (
        rf"<!-- agent-platform:{re.escape(config['agent_id'])}:([a-f0-9]{{32}}) -->"
    )
    conversation = next(
        (
            match.group(1)
            for comment in reversed(comments)
            if comment["user"]["login"] in (repo.split("/")[0], "github-actions[bot]")
            and (match := re.search(marker, comment.get("body", "")))
        ),
        "",
    )
    message = os.environ.get("PLATFORM_MESSAGE", "").strip()
    receipt = None
    if not conversation or message:
        payload = {
            "agent_id": config["agent_id"],
            "message": message or (issue["title"] + "\n\n" + (issue.get("body") or "")),
            "request_id": f"{repo}:issue:{issue['id']}:{config['agent_id']}:"
            + (os.environ.get("GITHUB_RUN_ID", "local") if conversation else "initial"),
        }
        if conversation:
            payload["conversation_id"] = conversation
        else:
            workspace = os.environ.get(
                "PLATFORM_WORKSPACE_PATH", ""
            ).strip() or config.get("workspace_path", "")
            if not workspace:
                raise ValueError(
                    "Set workspace_path in the Runner configuration or workflow input to the prepared local project directory"
                )
            payload["workspace_path"] = workspace
        text = payload.pop("message")
        receipt = client.invoke(text, **payload)
        conversation = receipt["conversation_id"]
        print(f"Input saved. Conversation: {conversation}", flush=True)
    try:
        result = client.wait(
            conversation, message_id=receipt["message_id"] if receipt else None
        )
    except TimeoutError:
        result = client.conversation(conversation)
    status = result["conversation"]["status"]
    replies = [
        m
        for m in result["messages"]
        if m["role"] == "agent"
        and m["kind"] == "reply"
        and (not receipt or m.get("parent_id") == receipt["message_id"])
    ]
    content = (
        replies[-1]["content"]
        if replies
        else "Agent 正在处理，请进入会话查看实时进展。"
    )
    if status == "failed" and not replies:
        content = "执行失败：" + result["conversation"].get(
            "error", "请进入平台查看日志"
        )
    if receipt:
        submitted = next(
            m for m in result["messages"] if m["id"] == receipt["message_id"]
        )
        if submitted["status"] == "queued" and status in ("failed", "stopped"):
            content = "输入已保存；会话当前已暂停，请进入平台查看原因并继续队列。"
    fixed_url = base + "/conversations/" + conversation
    body = (
        f"<!-- agent-platform:{config['agent_id']}:{conversation} -->\n"
        + content
        + f"\n\n[登录平台，查看与继续会话]({fixed_url})"
        + f"\n\n会话 ID：`{conversation}`。链接长期有效；未登录时使用平台账号登录，随后自动打开原会话。"
        + "API Token 只用于调用，不放进网页链接或评论。"
    )
    comment = github(f"repos/{repo}/issues/{number}/comments", {"body": body})
    print("Issue response: " + comment["html_url"], flush=True)
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a") as output:
            output.write(
                f"### Agent Platform requirements\n\n状态：{status}\n\n"
                f"[打开 Issue 中的会话入口]({comment['html_url']})\n"
            )


if __name__ == "__main__":
    main()
