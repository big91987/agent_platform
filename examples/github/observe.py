"""Reconnect to one persisted input after a Runner wait expires; never invoke an Agent."""

import argparse
import fcntl
import json
import os
from pathlib import Path

from agent_platform_client import Client
from pipeline import comment_once, delivery_token
from requirements import github


def dispatch_observer(settings, number, stage, message_id):
    os.environ["GH_TOKEN"] = delivery_token(settings)
    return github(
        f"repos/{settings['repository']}/actions/workflows/agent-platform.yml/dispatches",
        {
            "ref": settings.get("observer_ref", "main"),
            "inputs": {
                "issue": str(number),
                "after": "observe",
                "target_stage": stage,
                "message_id": str(message_id),
            },
        },
    )


def matching(task, stage, message_id):
    receipt = task.get("stages", {}).get(stage, {})
    return (
        not task.get("replacement")
        and receipt.get("message_id") == message_id
        and (stage == "review" or receipt.get("round", "") == task.get("round", ""))
    )


def wait_and_publish(client, settings, path, number, stage, message_id, base_url):
    task = json.loads(path.read_text())
    if not matching(task, stage, message_id):
        return
    receipt = task["stages"][stage]
    conversation = receipt["conversation_id"]
    url = base_url.rstrip("/") + "/conversations/" + conversation
    try:
        result = client.wait(conversation, message_id=message_id, timeout=900)
    except TimeoutError:
        result = None
    with path.with_suffix(".lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        task = json.loads(path.read_text())
        if not matching(task, stage, message_id):
            return
        repo = settings["repository"]
        if result is None:
            dispatch_observer(settings, number, stage, message_id)
            comment_once(
                repo,
                number,
                f"<!-- platform-wait:{message_id} -->",
                f"{stage} 仍在执行，已安排继续接收同一轮结果，不会重新执行任务。[查看实时进度]({url})",
            )
            return
        replies = [
            m
            for m in result["messages"]
            if m["role"] == "agent"
            and m["kind"] == "reply"
            and m.get("parent_id") == message_id
        ]
        if replies:
            comment_once(
                repo,
                number,
                f"<!-- platform-reply:{message_id} -->",
                replies[-1]["content"] + f"\n\n[在平台继续]({url})",
            )
            if stage == "review":
                from pipeline import task_directory

                delivery = json.loads(
                    (
                        task_directory(settings, Path(task["workspace"]))
                        / "delivery.json"
                    ).read_text()
                )
                pr = int(delivery["pr_url"].rstrip("/").split("/")[-1])
                comment_once(
                    repo,
                    pr,
                    f"<!-- platform-code-review:{message_id} -->",
                    replies[-1]["content"]
                    + f"\n\n[独立审查会话]({url})\n\n由人工决定修改与合并。",
                )
            if stage == "review" and task.get("integration"):
                import pr_refresh
                from pipeline import product_digest, require_verified_product

                workspace = Path(task["workspace"])
                require_verified_product(task, workspace)
                if receipt.get("product_sha256") != product_digest(workspace):
                    raise ValueError("Product changed during independent review")
                pr_refresh.reviewed(settings, task, path, number, receipt["head_sha"])
        elif result["conversation"]["status"] in ("failed", "stopped"):
            comment_once(
                repo,
                number,
                f"<!-- platform-stopped:{message_id} -->",
                f"{stage} 执行已停止或失败。[查看实际错误并继续原会话]({url})",
            )
            raise RuntimeError("Agent stopped or failed")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument(
        "--stage",
        choices=("requirements", "design", "development", "qa", "review"),
        required=True,
    )
    parser.add_argument("--message-id", type=int, required=True)
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text())
    settings = config["pipeline"]
    if os.environ["GH_REPO"] != settings["repository"]:
        raise ValueError("Unregistered repository")
    number = int(os.environ["ISSUE_NUMBER"])
    issue = github(f"repos/{settings['repository']}/issues/{number}")
    if "pull_request" in issue or issue["state"] != "open":
        return
    path = Path(settings["registry"]) / "issues" / f"{number}.json"
    wait_and_publish(
        Client(config["base_url"], config["token"]),
        settings,
        path,
        number,
        args.stage,
        args.message_id,
        config["base_url"],
    )


if __name__ == "__main__":
    main()
