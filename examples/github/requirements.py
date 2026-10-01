#!/usr/bin/env python3
"""GitHub Runner adapter: persistent conversation API and a scoped web link.

Configuration is local private JSON: base_url, agent_id, token.
The platform API token never enters Issue comments or GitHub artifacts.
"""
import argparse
import json
import os
import re
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from datetime import datetime
from zoneinfo import ZoneInfo


def github(path, body=None):
    command = ["gh", "api", path]
    if body is not None:
        command += ["--method", "POST", "--input", "-"]
    result = subprocess.run(
        command, input=None if body is None else json.dumps(body),
        text=True, capture_output=True, check=True, timeout=60,
    )
    return json.loads(result.stdout)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text())
    base = config["base_url"].rstrip("/")

    def api(path, body=None):
        request = urllib.request.Request(
            base + path, data=None if body is None else json.dumps(body).encode(),
            headers={"Authorization": "Bearer " + config["token"],
                     "Content-Type": "application/json"},
        )
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            raise RuntimeError(f"Platform API {error.code}: {error.read().decode()}") from None

    repo, number = os.environ["GH_REPO"], os.environ["ISSUE_NUMBER"]
    issue = github(f"repos/{repo}/issues/{number}")
    if "pull_request" in issue:
        raise ValueError("Use a product Issue, not a pull request")
    user = "github:" + issue["user"]["login"]
    # Durable external association; platform owns all native Session state.
    comments = github(f"repos/{repo}/issues/{number}/comments?per_page=100")
    marker = rf"<!-- agent-platform:{re.escape(config['agent_id'])}:([a-f0-9]{{32}}) -->"
    conversation = next((match.group(1) for comment in reversed(comments)
                         if comment["user"]["login"] in (repo.split("/")[0], "github-actions[bot]")
                         and (match := re.search(marker, comment.get("body", "")))), "")
    message = os.environ.get("PLATFORM_MESSAGE", "").strip()
    if not conversation or message:
        payload = {"agent_id": config["agent_id"], "user_id": user,
                   "message": message or (issue["title"] + "\n\n" + (issue.get("body") or "")),
                   "request_id": f"{repo}:issue:{issue['id']}:{config['agent_id']}:"
                                 + (os.environ.get("GITHUB_RUN_ID", "local") if conversation else "initial")}
        if conversation:
            payload["conversation_id"] = conversation
        receipt = api("/api/invoke", payload)
        conversation = receipt["conversation_id"]
        print(f"Input saved. Conversation: {conversation}", flush=True)
    query = "?user_id=" + urllib.parse.quote(user)
    deadline = time.monotonic() + 600
    while True:
        result = api("/api/conversations/" + conversation + query)
        status = result["conversation"]["status"]
        if status not in ("queued", "running", "stopping"):
            break
        if time.monotonic() > deadline:
            break
        time.sleep(3)
    link = api("/api/conversations/" + conversation + "/web-access",
               {"user_id": user, "expires_in": 3600})
    replies = [m for m in result["messages"] if m["role"] == "agent" and m["kind"] == "reply"]
    content = replies[-1]["content"] if replies else "Agent 正在处理，请进入会话查看实时进展。"
    if status == "failed":
        content = "执行失败：" + result["conversation"].get("error", "请进入平台查看日志")
    fixed_url = base + "/conversations/" + conversation
    refresh_url = f"https://github.com/{repo}/actions/workflows/agent-platform.yml"
    expiry = datetime.fromisoformat(link["expires_at"].replace("Z", "+00:00"))
    expiry = expiry.astimezone(ZoneInfo("Asia/Shanghai")).strftime("%Y-%m-%d %H:%M（北京时间）")
    body = (f"<!-- agent-platform:{config['agent_id']}:{conversation} -->\n"
            + content + f"\n\n[临时入口：无需账号，接着澄清]({link['url']})"
            + f" · [账号登录后打开同一会话]({fixed_url})"
            + f"\n\n临时入口有效至 {expiry}。账号入口不随临时链接过期，仍需账号具备会话权限。"
            + f"\n\n[刷新临时入口]({refresh_url})：点击 Run workflow，Issue 填 `{number}`，"
            + "message 留空。只刷新授权和读取结果，不会重建会话或重新执行 Agent。")
    comment = github(f"repos/{repo}/issues/{number}/comments", {"body": body})
    print("Issue response: " + comment["html_url"], flush=True)
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a") as output:
            output.write(f"### Agent Platform requirements\n\n状态：{status}\n\n"
                         f"[打开 Issue 中的会话入口]({comment['html_url']})\n")


if __name__ == "__main__":
    main()
