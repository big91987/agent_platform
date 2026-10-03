#!/usr/bin/env python3
"""Integration-owned management of dispatched handoffs. No platform stage logic."""

import argparse
import fcntl
import hashlib
import json
import os
import subprocess
import sys
from pathlib import Path

from agent_platform_client import APIError, Client
from pipeline import FORWARD, STAGES, result_path, save, task_directory
from requirements import github


def hash_result(settings, path):
    return subprocess.run(
        [settings["tool_binary"], "--hash-result", str(path)],
        text=True,
        capture_output=True,
        check=True,
        timeout=10,
    ).stdout.strip()


def dispatch_replacement(settings, path, cfg, body):
    config_path = path.with_suffix(".config.json")
    save(config_path, cfg)
    result = subprocess.run(
        [settings["tool_binary"], "--config", str(config_path), "--submit-json"],
        input=json.dumps(body),
        text=True,
        capture_output=True,
        timeout=45,
    )
    if result.returncode:
        raise RuntimeError(result.stderr.strip())
    return json.loads(result.stdout)


def current_target(task, directory, source, file, saved):
    target = saved["handoff"].get("target_stage") or FORWARD[source]
    if target not in STAGES:
        raise ValueError("Published delivery cannot be withdrawn with an Agent handoff")
    if task.get("active_stage") == source and str(
        result_path(directory, task, source)
    ) == str(file):
        return target, None
    transition = task.get("transition", {})
    if task.get("active_stage") != target or transition.get("file") != str(file):
        raise ValueError(
            "Task has advanced beyond this handoff; inspect its current stage before redirecting"
        )
    outgoing = result_path(directory, task, target)
    if (
        outgoing.exists()
        and json.loads(outgoing.read_text()).get("delivery") == "accepted"
    ):
        raise ValueError("Task has advanced: the target has already handed off")
    receipt = task.get("stages", {}).get(target)
    # A retried dispatch may have another GitHub run ID. The immutable
    # transition file identifies the invocation that actually started.
    if receipt is None:
        raise ValueError(
            "Target start is uncertain; retry the receiving workflow to recover its execution receipt before managing it"
        )
    if target == source:
        raise ValueError(
            "The target is now the current stage; continue it and use submit_handoff for its next transition"
        )
    if receipt.get("round", "") != task.get("round", ""):
        raise ValueError("Task has advanced to a different execution")
    return target, receipt


def manage(settings, client, number, request):
    operation = request["operation"]
    data = request["input"]
    source = request["source_stage"]
    if (
        operation not in ("supplement_handoff", "replace_handoff")
        or source not in STAGES
    ):
        raise ValueError("Unknown operation or stage")
    run_id = data.get("run_id")
    if (
        not isinstance(run_id, int)
        or isinstance(run_id, bool)
        or run_id <= 0
        or not data.get("content", "").strip()
    ):
        raise ValueError("A returned run_id and nonempty content are required")
    if operation == "replace_handoff" and data.get("target_stage") not in STAGES:
        raise ValueError("Replacement requires a target stage")
    task_path = Path(settings["registry"]) / "issues" / f"{number}.json"
    with task_path.with_suffix(".lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        task = json.loads(task_path.read_text())
        workspace = Path(task["workspace"]).resolve()
        if workspace != Path(request["workspace"]).resolve():
            raise ValueError("Workspace does not belong to this task")
        directory = task_directory(settings, workspace)
        matches = [
            (p, json.loads(p.read_text()))
            for p in directory.glob(source + "-*result.json")
        ]
        matches = [(p, v) for p, v in matches if v.get("run_id") == run_id]
        if len(matches) != 1:
            raise ValueError(
                "This run is not a handoff issued by this stage in this task"
            )
        file, saved = matches[0]
        key = hashlib.sha256(json.dumps(request, sort_keys=True).encode()).hexdigest()
        operation_path = directory / "operations" / f"{run_id}-{key}.json"
        op = json.loads(operation_path.read_text()) if operation_path.exists() else None
        if op and op.get("result"):
            return op["result"]
        if not op:
            if saved.get("revoked") or task.get("replacement"):
                raise ValueError(
                    "Handoff withdrawn or another replacement is in progress"
                )
            target, receipt = current_target(task, directory, source, file, saved)
            op = {"request": request, "target": target, "receipt": receipt}
            if receipt:
                state = client.conversation(receipt["conversation_id"])
                op["expected_message_id"] = max(
                    m["id"] for m in state["messages"] if m["role"] == "user"
                )
            save(operation_path, op)
        target, receipt = op["target"], op["receipt"]
        if operation == "supplement_handoff":
            # A failed/retried network request must not route to a newer stage.
            target, receipt = current_target(task, directory, source, file, saved)
            if saved.get("revoked") or task.get("replacement"):
                raise ValueError("Handoff was withdrawn")
            addition = {"content": data["content"], "request_id": key}
            response = {
                "run_id": run_id,
                "target_stage": target,
                "status": "saved_for_start",
            }
            if receipt:
                cid = receipt["conversation_id"]
                state = client.conversation(cid)["conversation"]["status"]
                if state in ("failed", "stopped", "stopping", "closed"):
                    raise ValueError(
                        "Target is stopped or failed; recover it or replace the handoff"
                    )
                delivered = client.invoke(
                    "上游交接补充（继续当前任务）：\n" + data["content"],
                    conversation_id=cid,
                    request_id=f"handoff:{run_id}:supplement:{key}",
                )
                response.update(
                    status="queued",
                    conversation_id=cid,
                    message_id=delivered["message_id"],
                )
                if state == "running":
                    try:
                        client.steer(cid, delivered["message_id"])
                        response["status"] = "steering"
                    except APIError as error:
                        if error.status_code != 409:
                            raise
            additions = saved.setdefault("supplements", [])
            if not any(a["request_id"] == key for a in additions):
                additions.append(addition)
            save(file, saved)
            op["result"] = response
            save(operation_path, op)
            return response

        # Durable invalidation precedes either cancellation. Retrying resumes the
        # same operation, never a new run or a different target.
        if not op.get("invalidated"):
            target, receipt = current_target(task, directory, source, file, saved)
            op["target"], op["receipt"] = target, receipt
            if receipt and "expected_message_id" not in op:
                state = client.conversation(receipt["conversation_id"])
                op["expected_message_id"] = max(
                    m["id"] for m in state["messages"] if m["role"] == "user"
                )
            save(operation_path, op)
            saved["revoked"] = True
            saved["replacement_operation"] = str(operation_path)
            save(file, saved)
            task["replacement"] = {"operation": str(operation_path)}
            save(task_path, task)
            op["invalidated"] = True
            save(operation_path, op)
        elif task.get("replacement", {}).get("operation") not in (
            None,
            str(operation_path),
        ):
            raise ValueError("Another replacement is already in progress")
        if not op.get("stopped"):
            if receipt:
                cid = receipt["conversation_id"]
                try:
                    client.stop(
                        cid,
                        expected_message_id=op["expected_message_id"],
                        discard_queued=True,
                    )
                except APIError as error:
                    if error.status_code != 409:
                        raise
                    # A definite precondition rejection made no changes. Restore
                    # the old handoff; do not silently broaden authorization.
                    saved.pop("revoked", None)
                    saved.pop("replacement_operation", None)
                    save(file, saved)
                    task.pop("replacement", None)
                    save(task_path, task)
                    op["result"] = {
                        "status": "conflict",
                        "run_id": run_id,
                        "message": "Newer input exists; nothing was cancelled. Review it before issuing a new replacement request.",
                    }
                    save(operation_path, op)
                    return op["result"]
                stopped = client.wait(cid, timeout=30)
                if stopped["conversation"]["status"] != "stopped":
                    raise RuntimeError(
                        "Old Agent has not stopped; replacement was not dispatched"
                    )
            run_ids = {run_id}
            if receipt and receipt.get("run_id"):
                run_ids.add(int(receipt["run_id"]))
            for ident in sorted(run_ids):
                status = github(f"repos/{settings['repository']}/actions/runs/{ident}")
                if status["status"] != "completed":
                    subprocess.run(
                        [
                            "gh",
                            "api",
                            "--method",
                            "POST",
                            f"repos/{settings['repository']}/actions/runs/{ident}/cancel",
                        ],
                        text=True,
                        capture_output=True,
                        check=True,
                        timeout=30,
                    )
            op["stopped"] = True
            save(operation_path, op)
        new_file = directory / f"{source}-replace-{run_id}-{key[:16]}-result.json"
        if not op.get("prepared"):
            documents = {}
            for name in saved["documents"]:
                document = (workspace / name).resolve()
                if not document.is_relative_to(workspace):
                    raise ValueError("Artifact escapes workspace")
                documents[name] = document.read_text()
            body = {
                "summary": saved["handoff"]["summary"]
                + "".join(
                    "\n\n此前交接补充：\n" + item["content"]
                    for item in saved.get("supplements", [])
                )
                + "\n\n用户撤回并修正交接：\n"
                + data["content"],
                "target_stage": data["target_stage"],
                "artifacts": list(documents),
            }
            value = {
                "handoff": body,
                "documents": documents,
                "delivery": "uncertain",
                "source_stage": source,
            }
            save(new_file, value)
            value["sha256"] = hash_result(settings, new_file)
            save(new_file, value)
            transition = {
                "file": str(new_file),
                "sha256": value["sha256"],
                "stage": data["target_stage"],
            }
            if task.get("replacement") != transition:
                task["round"] = str(int(task.get("round", "0") or "0") + 1)
                task["replacement"] = transition
                save(task_path, task)
            op["prepared"] = True
            save(operation_path, op)
        value = json.loads(new_file.read_text())
        cfg = json.loads((directory / (source + ".json")).read_text())
        cfg.update(
            result_file=str(new_file),
            task_file="",
            inputs={"issue": str(number), "after": "redirect", "source_stage": source},
        )
        # source_stage is receipt metadata; it is not an extra workflow input.
        cfg["source_stage"] = source
        cfg["inputs"].pop("source_stage")
        out = (
            value
            if value.get("delivery") == "accepted" and value.get("run_id")
            else dispatch_replacement(settings, new_file, cfg, value["handoff"])
        )
        if not out.get("run_id"):
            raise RuntimeError(
                "Dispatch accepted without a run handle; inspect its receipt before retrying"
            )
        op["result"] = {
            "run_id": out["run_id"],
            "run_url": out.get("run_url", ""),
            "replaces_run_id": run_id,
            "target_stage": data["target_stage"],
            "status": "dispatched",
        }
        save(operation_path, op)
        return op["result"]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument("--issue", type=int, required=True)
    args = parser.parse_args()
    config = json.loads(Path(args.config).read_text())
    if os.environ.get("PIPELINE_TOKEN"):
        os.environ["GH_TOKEN"] = os.environ["PIPELINE_TOKEN"]
    try:
        out = manage(
            config["pipeline"],
            Client(config["base_url"], config["token"]),
            args.issue,
            json.load(sys.stdin),
        )
        print(json.dumps(out, ensure_ascii=False))
    except Exception as error:
        # No HTTP bodies, subprocess stdout or configuration values in MCP errors.
        message = str(error)
        for secret in (config.get("token"), os.environ.get("PIPELINE_TOKEN")):
            if secret:
                message = message.replace(secret, "[redacted]")
        print(message, file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
