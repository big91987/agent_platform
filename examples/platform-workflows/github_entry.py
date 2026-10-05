#!/usr/bin/env python3
"""Trusted CI adapter: forward an Issue event, never schedule development stages."""

import argparse
import fcntl
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
import urllib.parse
from pathlib import Path

from agent_platform_client import APIError, Client

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "github"))
from requirements import github  # noqa: E402


def save(path, value):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False) as output:
        json.dump(value, output, ensure_ascii=False, indent=2)
        temporary = Path(output.name)
    temporary.chmod(0o600)
    temporary.replace(path)


def platform_output(body):
    return any(
        marker in (body or "")
        for marker in ("<!-- agent-platform:", "<!-- agent-platform-hook:")
    )


def event_input(config, env, event):
    repo = config["repository"]
    if not re.fullmatch(r"[\w.-]+/[\w.-]+", repo):
        raise ValueError("Invalid configured repository")
    owner = repo.split("/")[0]
    if (
        env.get("GITHUB_REPOSITORY") != repo
        or event.get("repository", {}).get("full_name") != repo
        or env.get("GITHUB_ACTOR") != owner
        or env.get("GITHUB_TRIGGERING_ACTOR") != owner
        or event.get("sender", {}).get("login") != owner
    ):
        raise ValueError("Only this repository's owner may forward events")
    kind = env.get("GITHUB_EVENT_NAME")
    if kind == "workflow_dispatch":
        inputs = event.get("inputs", {})
        number, comment = inputs.get("issue", ""), inputs.get("comment_id", "")
        if (
            not str(number).isdigit()
            or int(number) < 1
            or (comment and (not str(comment).isdigit() or int(comment) < 1))
        ):
            raise ValueError("Positive Issue/comment numbers required")
        return int(number), int(comment or 0)
    issue = event.get("issue", {})
    if (
        "pull_request" in issue
        or not isinstance(issue.get("number"), int)
        or issue["number"] < 1
    ):
        raise ValueError("A product Issue is required")
    if kind == "issues" and event.get("action") == "opened":
        if issue.get("user", {}).get("login") != owner:
            raise ValueError("Issue author is not authorized")
        return issue["number"], 0
    if kind == "issue_comment" and event.get("action") == "created":
        comment = event["comment"]
        if (
            comment.get("user", {}).get("login") != owner
            or comment.get("user", {}).get("type") != "User"
        ):
            raise ValueError("Comment author is not authorized")
        if platform_output(comment.get("body")):
            return None
        if not isinstance(comment.get("id"), int) or comment["id"] < 1:
            raise ValueError("Invalid comment ID")
        return issue["number"], comment["id"]
    raise ValueError("Unsupported GitHub event")


def validate_proxy(config):
    proxy = config.get("git_proxy", "")
    if proxy:
        parsed = urllib.parse.urlsplit(proxy)
        if (
            parsed.scheme not in ("http", "https", "socks5", "socks5h")
            or not parsed.hostname
            or parsed.username
            or parsed.password
        ):
            raise ValueError("Git proxy must be an endpoint without credentials")


def configure_transport(config, directory):
    # Persist only a credential-free endpoint in this installation's checkout.
    validate_proxy(config)
    if "git_proxy" in config:
        subprocess.run(
            [
                "git",
                "-C",
                str(directory),
                "config",
                "--local",
                "http.proxy",
                config["git_proxy"],
            ],
            check=True,
            capture_output=True,
        )


def prepare_workspace(config, issue_id):
    validate_proxy(config)
    root = Path(config["workspace_root"]).resolve(strict=True)
    directory = root / ("github-issue-" + str(issue_id))
    remote = "https://github.com/" + config["repository"] + ".git"
    if directory.exists():
        if directory.is_symlink() or not directory.resolve().is_relative_to(root):
            raise ValueError("Issue checkout escapes configured workspace root")
        actual = subprocess.check_output(
            ["git", "-C", str(directory), "remote", "get-url", "origin"], text=True
        ).strip()
        if actual != remote:
            raise ValueError("Existing checkout belongs to another repository")
        configure_transport(config, directory)
        return directory
    base = config.get("base", "main")
    subprocess.run(
        ["git", "check-ref-format", "--branch", base], check=True, capture_output=True
    )
    # Rename only after clone succeeds. A failed clone never becomes a Run workspace.
    with tempfile.TemporaryDirectory(prefix=".issue-clone-", dir=root) as temporary:
        checkout = Path(temporary) / "checkout"
        subprocess.run(
            [
                "git",
                *(
                    ["-c", "http.proxy=" + config["git_proxy"]]
                    if config.get("git_proxy")
                    else []
                ),
                "-c",
                "credential.helper=",
                "-c",
                "credential.helper=!gh auth git-credential",
                "clone",
                "--single-branch",
                "--no-tags",
                "--branch",
                base,
                "--",
                remote,
                str(checkout),
            ],
            check=True,
            capture_output=True,
            timeout=180,
        )
        configure_transport(config, checkout)
        checkout.rename(directory)
    return directory


def accepted_run(client, key):
    try:
        return client.workflow_by_request(key)
    except APIError as error:
        if error.status_code != 404:
            raise
        return None


def notify(repo, number, key, body):
    marker = (
        "<!-- agent-platform:entry:" + hashlib.sha256(key.encode()).hexdigest() + " -->"
    )
    comments = github(
        f"repos/{repo}/issues/{number}/comments?per_page=100", paginate=True
    )
    if any(
        marker in (comment.get("body") or "")
        and comment["user"]["login"] in (repo.split("/")[0], "github-actions[bot]")
        for comment in comments
    ):
        return
    github(f"repos/{repo}/issues/{number}/comments", {"body": body + "\n\n" + marker})


def forward(config, client, number, comment_id=0):
    repo = config["repository"]
    owner = repo.split("/")[0]
    issue = github(f"repos/{repo}/issues/{number}")
    if (
        "pull_request" in issue
        or issue["number"] != number
        or issue["user"]["login"] != owner
    ):
        raise ValueError("Expected an owner-authored product Issue")
    key = f"github:{repo}:issue:{issue['id']}"
    state = Path(config["state_root"]).resolve()
    workspace_root = Path(config["workspace_root"]).resolve()
    if state.is_relative_to(workspace_root) or workspace_root.is_relative_to(state):
        raise ValueError("Adapter state must be outside Agent workspaces")
    state.mkdir(parents=True, exist_ok=True, mode=0o700)
    lockpath = state / (str(issue["id"]) + ".lock")
    with lockpath.open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        current = accepted_run(client, key)
        if comment_id:
            if not current:
                raise ValueError(
                    "Issue has no Run; start it before forwarding comments"
                )
            comment = github(f"repos/{repo}/issues/comments/{comment_id}")
            if (
                comment.get("issue_url")
                != f"https://api.github.com/repos/{repo}/issues/{number}"
                or comment["user"]["login"] != owner
                or comment["user"].get("type") != "User"
            ):
                raise ValueError("Comment does not belong to this authorized Issue")
            if platform_output(comment.get("body")):
                return current
            event_key = f"github:{repo}:comment:{comment_id}"
            snapshot = state / ("comment-" + str(comment_id) + ".json")
            payload = {"message": comment.get("body") or "", "run_id": current["id"]}
            if snapshot.exists():
                payload = json.loads(snapshot.read_text())
            else:
                save(snapshot, payload)
            if payload["run_id"] != current["id"]:
                raise ValueError("Comment association changed")
            try:
                receipt = client.workflow_message(
                    current["id"], payload["message"], request_id=event_key
                )
            except APIError as error:
                if error.status_code == 409:
                    notify(
                        repo,
                        number,
                        event_key + ":rejected",
                        f"这条补充尚未进入执行：当前节点正在交接、已停止或已结束。请从 [Run]({config['base_url']}/workflow-runs/{current['id']}) 核对并继续/回退，再在 Actions 用原评论 ID 重试。",
                    )
                raise
            save(state / ("comment-" + str(comment_id) + "-receipt.json"), receipt)
            notify(
                repo,
                number,
                event_key,
                f"补充已保存到当前 Agent 会话：[查看会话]({receipt['conversation_url']})。",
            )
        else:
            if not current:
                if issue.get("state") != "open":
                    raise ValueError("Only an open Issue may start a new Run")
                snapshot = state / (str(issue["id"]) + "-start.json")
                if snapshot.exists():
                    payload = json.loads(snapshot.read_text())
                else:
                    payload = {
                        "workflow_id": config["workflow_id"],
                        "message": issue["title"] + "\n\n" + (issue.get("body") or ""),
                        "parameters": {"issue_number": str(number)},
                    }
                    save(snapshot, payload)
                workspace = prepare_workspace(config, issue["id"])
                current = client.start_workflow(
                    payload["workflow_id"],
                    payload["message"],
                    workspace_path=str(workspace),
                    request_id=key,
                    parameters=payload["parameters"],
                )
            elif current["status"] in ("failed", "stopped"):
                expected = Path(config["workspace_root"]).resolve() / (
                    "github-issue-" + str(issue["id"])
                )
                if Path(current["workspace_path"]).resolve() != expected:
                    raise ValueError("Run workspace does not match the original Issue")
                prepare_workspace(config, issue["id"])
            save(
                state / (str(issue["id"]) + "-run.json"),
                {"id": current["id"], "request_id": key},
            )
            notify(
                repo,
                number,
                key,
                f"研发任务已接单：[查看运行进度]({config['base_url']}/workflow-runs/{current['id']})。可直接在本 Issue 评论补充要求；无需再到平台创建任务。",
            )
        return current


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", type=Path, required=True)
    args = parser.parse_args()
    config = json.loads(args.config.read_text())
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    entry = event_input(config, os.environ, event)
    if entry is None:
        print("Platform output ignored")
        return
    token = Path(config["token_file"]).read_text().strip()
    client = Client(config["base_url"], token, timeout=60)
    number, comment_id = entry
    result = forward(config, client, number, comment_id)
    print(f"Run {result['id']} · {result['status']}")
    if summary := os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(summary, "a") as output:
            output.write(
                f"[打开平台 Run]({config['base_url']}/workflow-runs/{result['id']})\n"
            )


if __name__ == "__main__":
    try:
        main()
    except (ValueError, APIError, ConnectionError, subprocess.SubprocessError) as error:
        # Subprocess errors omit stdout/stderr (which may carry private context).
        sys.exit(str(error))
