#!/usr/bin/env python3
"""GitLab adapter using the same SDK and conversation contract as GitHub."""

import argparse
import json
import os
from pathlib import Path

from agent_platform_client import Client


def write_json(path, value):
    target = Path(path)
    temporary = target.with_suffix(target.suffix + ".tmp")
    temporary.write_text(
        json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    temporary.replace(target)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--message-file")
    parser.add_argument("--output", default="agent-platform-result.json")
    parser.add_argument("--wait-seconds", type=float, default=600)
    args = parser.parse_args()
    client = Client(
        os.environ["AGENT_PLATFORM_URL"], os.environ["AGENT_PLATFORM_TOKEN"]
    )
    identity = client.me()
    print(f"[platform] Calling as {identity['user_id']}", flush=True)
    conversation = os.environ.get("PLATFORM_CONVERSATION_ID", "")
    request_id = os.environ.get("PLATFORM_REQUEST_ID") or (
        f"gitlab:{os.environ['CI_PROJECT_ID']}:{os.environ['CI_PIPELINE_ID']}:{os.environ['CI_JOB_NAME']}"
    )
    message = (
        Path(args.message_file).read_text(encoding="utf-8")
        if args.message_file
        else os.environ["PLATFORM_MESSAGE"]
    )
    receipt = client.invoke(
        message,
        request_id=request_id,
        agent_id=os.environ["AGENT_PLATFORM_AGENT_ID"],
        conversation_id=conversation,
        workspace_path="" if conversation else os.environ["PLATFORM_WORKSPACE_PATH"],
    )
    output = {"receipt": receipt}
    write_json(args.output, output)
    conversation = receipt["conversation_id"]
    print(
        f"[platform] {receipt['conversation_url']} (input {receipt['message_id']})",
        flush=True,
    )
    try:
        result = client.wait(
            conversation, message_id=receipt["message_id"], timeout=args.wait_seconds
        )
    except TimeoutError:
        result = client.conversation(conversation)
        print(
            "[platform] Observation timed out; execution continues on the platform",
            flush=True,
        )
    output["result"] = result
    write_json(args.output, output)
    submitted = next(m for m in result["messages"] if m["id"] == receipt["message_id"])
    print(
        f"[platform] Input: {submitted['status']}; conversation: {result['conversation']['status']}",
        flush=True,
    )
    for reply in result["messages"]:
        if reply.get("parent_id") == receipt["message_id"] and reply["kind"] == "reply":
            print("[agent] " + reply["content"], flush=True)
    if submitted["status"] in ("failed", "stopped") or (
        submitted["status"] != "completed"
        and result["conversation"]["status"] in ("failed", "stopped", "closed")
    ):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
