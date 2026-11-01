"""A task-scoped stdio MCP endpoint; identity comes from controller arguments."""

import argparse
import json
import sys

from factory_store import Store

TOOLS = [
    {
        "name": "get_task",
        "description": "Read your assigned task, dependency results and incoming messages. Task state is authoritative; messages are discussion.",
        "inputSchema": {
            "type": "object",
            "properties": {},
            "additionalProperties": False,
        },
    },
    {
        "name": "send_message",
        "description": "Send a persistent question or coordination message to leader or another task. This does not complete or assign a task and does not grant authorization.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "to": {
                    "type": "string",
                    "description": "leader or an existing task ID",
                },
                "message": {
                    "type": "string",
                    "description": "Concrete question, relevant evidence and what is needed next",
                },
            },
            "required": ["to", "message"],
            "additionalProperties": False,
        },
    },
    {
        "name": "handoff",
        "description": "Submit your task outcome to the Leader. completed means your assigned work and local checks are finished; blocked means required input or capability is missing. Include exact evidence and limitations. Call once, then end your turn without further edits. The controller will preserve output and finalize only after native execution exits.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "status": {
                    "type": "string",
                    "enum": ["completed", "blocked"],
                    "description": "Whether the assigned work is finished or waiting on a concrete blocker",
                },
                "message": {
                    "type": "string",
                    "description": "Result, actual checks, evidence and unresolved issues; distinguish verified facts from proposals",
                },
                "artifacts": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Existing worktree-relative evidence paths; never include credentials or private native histories",
                },
            },
            "required": ["status", "message", "artifacts"],
            "additionalProperties": False,
        },
    },
]


def call(store, task_id, attempt, name, arguments):
    task = store.get(task_id)
    if task["status"] != "running" or task["attempt"] != attempt:
        raise ValueError("execution is no longer current")
    tool = next((tool for tool in TOOLS if tool["name"] == name), None)
    if not tool or not isinstance(arguments, dict):
        raise ValueError("unknown tool or invalid arguments")
    schema = tool["inputSchema"]
    if set(arguments) - set(schema["properties"]) or set(
        schema.get("required", [])
    ) - set(arguments):
        raise ValueError("unexpected or missing tool parameters")
    if name == "get_task":
        snapshot = store.snapshot()
        return {
            "task": task,
            "dependencies": [store.get(x) for x in task["depends_on"]],
            "messages": [
                message
                for message in snapshot["messages"]
                if message["recipient"] == task_id
            ],
        }
    if name == "send_message":
        store.message(task_id, arguments["to"], arguments["message"])
        return {"accepted": True}
    store.submit(task_id, arguments, attempt=attempt)
    return {
        "accepted": True,
        "task_id": task_id,
        "instruction": "End this native turn now. The submission is preserved; it does not claim integration or product acceptance.",
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--task", required=True)
    parser.add_argument("--attempt", type=int, required=True)
    args = parser.parse_args()
    store = Store(args.root)
    for line in sys.stdin:
        try:
            request = json.loads(line)
            if not isinstance(request, dict) or "id" not in request:
                continue
            method = request.get("method")
            params = request.get("params", {})
            if method == "initialize":
                result = {
                    "protocolVersion": params.get("protocolVersion", "2024-11-05"),
                    "capabilities": {"tools": {}},
                    "serverInfo": {"name": "agent-team-factory", "version": "0.1.0"},
                }
            elif method == "ping":
                result = {}
            elif method == "tools/list":
                result = {"tools": TOOLS}
            elif method == "tools/call":
                try:
                    value = call(
                        store,
                        args.task,
                        args.attempt,
                        params.get("name"),
                        params.get("arguments", {}),
                    )
                    result = {
                        "content": [
                            {
                                "type": "text",
                                "text": json.dumps(value, ensure_ascii=False),
                            }
                        ]
                    }
                except (ValueError, TypeError, KeyError) as error:
                    result = {
                        "isError": True,
                        "content": [{"type": "text", "text": str(error)}],
                    }
            else:
                print(
                    json.dumps(
                        {
                            "jsonrpc": "2.0",
                            "id": request["id"],
                            "error": {"code": -32601, "message": "method not found"},
                        }
                    ),
                    flush=True,
                )
                continue
            print(
                json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": result}),
                flush=True,
            )
        except (ValueError, TypeError, KeyError) as error:
            print(str(error), file=sys.stderr, flush=True)


if __name__ == "__main__":
    main()
