"""Apply an owner's /network comment to the registered task and its conversations."""

import argparse
import fcntl
import json
import os
import time
from pathlib import Path

from agent_platform_client import APIError, Client
from network_policy import network_command
from pipeline import comment_once, save


def apply_once(client, task, enabled):
    """Return pending conversations without interrupting active native turns."""
    pending = []
    for receipt in task.get("stages", {}).values():
        cid = receipt["conversation_id"]
        data = client.conversation(cid)
        if data["conversation"]["status"] == "closed":
            continue
        if data.get("execution_permissions", {}).get("network_access") == enabled:
            continue
        if data["conversation"]["status"] in ("running", "stopping"):
            pending.append(cid)
            continue
        try:
            client.network_access(cid, enabled)
        except APIError as error:
            if error.status_code != 409:
                raise
            pending.append(cid)
    return pending


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True, type=Path)
    args = parser.parse_args()
    config = json.loads(args.config.read_text())
    settings = config["pipeline"]
    repo = settings["repository"]
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    choice = network_command(event, repo)
    # Re-runs must also be initiated by the owner, not another Actions user.
    owner = repo.split("/")[0]
    if (
        os.environ.get("GITHUB_ACTOR") != owner
        or os.environ.get("GITHUB_TRIGGERING_ACTOR") != owner
    ):
        raise PermissionError("Only the repository owner can execute this update")
    number = int(event["issue"]["number"])
    event_id = int(event["comment"]["id"])
    path = Path(settings["registry"]) / "issues" / f"{number}.json"
    enabled = (
        settings.get("network_access", True)
        if choice == "default"
        else choice == "allow"
    )
    client = Client(config["base_url"], config["token"])
    with path.with_suffix(".lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        task = json.loads(path.read_text())
        if event_id < task.get("network_comment_id", 0):
            return
        task["network_access"] = enabled
        task["network_comment_id"] = event_id
        save(path, task)
    label = "允许联网" if enabled else "禁止联网（同时关闭额外提权申请，避免绕过禁网）"
    marker = f"<!-- platform-network:{event_id}"
    comment_once(
        repo,
        number,
        marker + ":requested -->",
        f"已收到 Owner 设置：**{label}**，只作用于本 Issue。正在执行的轮次会先完成，再应用到原会话；不重建会话、不打断工具。后续阶段使用同一设置。",
    )
    deadline = time.monotonic() + 1200
    while True:
        # Never hold this lock while waiting: Agent handoff uses the same lock.
        with path.with_suffix(".lock").open("w") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            task = json.loads(path.read_text())
            if task.get("network_comment_id") != event_id:
                return
            pending = apply_once(client, task, enabled)
        if not pending:
            comment_once(
                repo,
                number,
                marker + ":applied -->",
                f"**{label}已生效。**已更新本 Issue 的已有会话和后续阶段设置；原 Session、工作区和历史保留。若 Agent 此前已结束并等待补充，可从原会话继续；本命令不重复下发任务。",
            )
            return
        if time.monotonic() >= deadline:
            comment_once(
                repo,
                number,
                marker + ":pending -->",
                "后续阶段设置已保存；当前原生轮次仍未结束，旧轮次权限尚未变更。请在它结束后重新运行本次权限更新 Workflow；未中断或重复执行任务。",
            )
            raise TimeoutError(
                "Active turn has not ended; rerun this permission update"
            )
        time.sleep(5)


if __name__ == "__main__":
    main()
