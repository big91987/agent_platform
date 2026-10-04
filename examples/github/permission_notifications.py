"""Integration-owned permission notices; never approve or invoke an Agent."""

import argparse
import fcntl
import json
import os
import re
import time
from datetime import datetime, timezone
from pathlib import Path

from agent_platform_client import Client
from pipeline import comment_once, delivery_token, save


def public_reason(text):
    # Public Issue comments must not contain shell commands, local paths or tokens.
    text = re.sub(r"(?i)\bBearer\s+\S+", "Bearer [redacted]", str(text))
    text = re.sub(
        r"(?i)\b(token|password|secret|api[_-]?key|authorization)\s*[:=]\s*\S+",
        r"\1=[redacted]",
        text,
    )
    text = re.sub(r"https?://\S+", "[链接请在平台查看]", text)
    text = re.sub(r"(?<!\w)(?:/\S+|[A-Za-z]:\\\S+)", "[本机路径]", text)
    text = text.replace("@", "＠").replace("<", "＜").replace(">", "＞")
    text = re.sub(r"[\r\n]+", " ", text)
    for char in "\\`*_[]()!":
        text = text.replace(char, "\\" + char)
    return text[:400]


def notice(approval, stage, url):
    req = approval["request"]
    decision = approval.get("decision", "")
    state = {
        "": "等待管理员审批",
        "accept": "已批准",
        "decline": "已拒绝",
        "cancel": "已取消",
    }[decision]
    method = req.get("method", "")
    permissions = req.get("permissions") or req.get("additionalPermissions") or {}
    if method == "item/permissions/requestApproval":
        scopes = []
        if (permissions.get("network") or {}).get("enabled"):
            scopes.append("网络访问")
        if permissions.get("fileSystem"):
            scopes.append("额外文件访问（具体路径见平台）")
        scope = "、".join(scopes) or "额外执行权限（详见平台）"
        duration = "仅本轮有效"
    elif method == "item/fileChange/requestApproval":
        scope, duration = "文件修改授权（具体变更见平台）", "仅本次操作有效"
    else:
        scope, duration = "沙箱外命令执行（完整命令见平台）", "仅本次命令有效"
    reason = public_reason(req.get("reason") or "执行所需操作超出当前预设权限")
    followup = {
        "": "任务正在等待这项审批。请使用平台管理员账号打开下方链接，核对权限并批准或拒绝；在 Issue 回复“同意”不会授予权限。",
        "accept": "原生 Agent 将接收批准结果并接续原会话；这不代表阶段验收已通过。",
        "decline": "本次权限请求未获批准；Agent 会收到拒绝结果。若仍缺必需能力，任务会说明阻塞，不会自动扩大权限。",
        "cancel": "本次审批已失效（例如执行停止或平台重启），没有因此新增授权。",
    }[decision]
    return (
        f"### Agent 提权：{state}\n\n"
        f"- 阶段：`{stage}`\n- 原因：{reason}\n- 请求权限：{scope}\n"
        f"- 有效范围：{duration}，不修改 Agent 默认权限。\n\n"
        f"{followup}\n\n[在平台查看权限申请与执行情况]({url})"
    )


def timestamp(value):
    return datetime.fromisoformat(value.replace("Z", "+00:00"))


def sync_once(client, settings, base_url, state, publish=comment_once):
    known = state.setdefault("conversations", {})
    published = state.setdefault("published", {})
    for path in (Path(settings["registry"]) / "issues").glob("*.json"):
        if not path.stem.isdigit():
            continue
        task = json.loads(path.read_text())
        for stage, receipt in task.get("stages", {}).items():
            cid = receipt.get("conversation_id")
            if cid:
                known.setdefault(cid, {"issue": int(path.stem), "stage": stage})
    conversations = {c["id"]: c for c in client.conversations()}
    failures = []
    for cid, binding in known.items():
        try:
            c = conversations.get(cid)
            if not c and not binding.get("pending"):
                continue
            if (
                c
                and c["status"] not in ("running", "queued", "stopping")
                and not binding.get("pending")
                and "updated" in binding
                and binding["updated"] == c["updated_at"]
            ):
                continue
            pending = False
            for approval in client.conversation(cid).get("approvals", []):
                if not approval.get("request", {}).get("platform_permission_request"):
                    continue
                aid = approval["id"]
                decision = approval.get("decision", "")
                if decision not in ("", "accept", "decline", "cancel"):
                    continue
                requested = f"{cid}:{aid}:requested"
                key = f"{cid}:{aid}:{decision or 'requested'}"
                pending = pending or not decision
                if key in published:
                    continue
                if (
                    decision
                    and requested not in published
                    and timestamp(approval["created_at"])
                    < timestamp(state["started_at"])
                ):
                    continue
                url = base_url.rstrip("/") + "/conversations/" + cid
                publish(
                    settings["repository"],
                    binding["issue"],
                    f"<!-- platform-permission:{key} -->",
                    notice(approval, binding["stage"], url),
                )
                published[key] = True
            binding["pending"] = pending
            if c:
                binding["updated"] = c["updated_at"]
        except Exception as error:
            failures.append(type(error).__name__)
    if failures:
        raise RuntimeError("Some permission notices remain unsent; retry required")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True, type=Path)
    parser.add_argument("--interval", type=float, default=5)
    parser.add_argument("--once", action="store_true")
    args = parser.parse_args()
    if args.interval < 1:
        parser.error("interval must be at least one second")
    config = json.loads(args.config.read_text())
    settings = config["pipeline"]
    os.environ["GH_TOKEN"] = delivery_token(settings)
    client = Client(config["base_url"], config["token"])
    registry = Path(settings["registry"])
    path = registry / "permission-notifications.json"
    with (registry / "permission-notifications.lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        state = (
            json.loads(path.read_text())
            if path.exists()
            else {
                "started_at": datetime.now(timezone.utc).isoformat(),
                "conversations": {},
                "published": {},
            }
        )
        print(
            f"Permission notifications active for {settings['repository']}; interval={args.interval}s",
            flush=True,
        )
        # Persist the start boundary before any remote write, including after failures.
        save(path, state)
        while True:
            try:
                sync_once(client, settings, config["base_url"], state)
                save(path, state)
            except Exception as error:
                # Exceptions from HTTP/gh can contain request arguments; do not log them.
                print(
                    f"Permission notification sync failed: {type(error).__name__}; will retry",
                    flush=True,
                )
                if args.once:
                    raise SystemExit(1) from None
            if args.once:
                return
            time.sleep(args.interval)


if __name__ == "__main__":
    main()
