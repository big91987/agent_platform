"""Task/Leader-scoped stdio tools. No process-start or fixed-stage tool."""

import argparse
import json

from team_runtime import Team


def tool(name, description, fields, required=()):
    return {
        "name": name,
        "description": description,
        "inputSchema": {
            "type": "object",
            "properties": fields,
            "required": list(required),
            "additionalProperties": False,
        },
    }


STRING = {"type": "string"}
MESSAGE = tool(
    "send_message",
    "向leader、user或worker-N发送持久消息，不表示任务完成。",
    {"request_id": STRING, "to": STRING, "message": STRING},
    ("request_id", "to", "message"),
)
LEADER = [
    tool("get_board", "读取真实需求、任务状态、Worker结果、消息及可用Skill。", {}),
    tool(
        "create_task",
        "在需求下创建任务。key在需求内稳定唯一，重复同内容不会重复建任务。空闲Worker自动领取；依赖accepted后才执行。",
        {
            "request_id": STRING,
            "key": STRING,
            "title": STRING,
            "instructions": STRING,
            "skill": {
                "type": "string",
                "enum": ["research", "prd", "architecture", "development", "testing"],
            },
            "depends_on": {"type": "array", "items": STRING},
        },
        ("request_id", "key", "title", "instructions", "skill", "depends_on"),
    ),
    MESSAGE,
    tool(
        "accept_task",
        "明确接受已复核的Worker候选并合入本地实验分支。不推送、不创建PR、不部署。",
        {"task_id": STRING},
        ("task_id",),
    ),
    tool(
        "cancel_task",
        "取消非运行、未接受的任务并保留原因。需要调整时创建新任务，旧事实保留。",
        {"task_id": STRING, "reason": STRING},
        ("task_id", "reason"),
    ),
    tool(
        "finish_request",
        "向用户交付最终结果或明确需要补充的信息。completed要求任务均已接受或取消。",
        {
            "request_id": STRING,
            "status": {"type": "string", "enum": ["completed", "needs_input"]},
            "message": STRING,
        },
        ("request_id", "status", "message"),
    ),
]
WORKER = [
    tool("get_task", "读取当前分配、真实需求、依赖成果、所选Skill与收到的消息。", {}),
    MESSAGE,
    tool(
        "handoff",
        "提交结果或阻塞，调用一次后结束本轮。原生退出后保存候选，Leader决定接受。",
        {
            "status": {"type": "string", "enum": ["completed", "blocked"]},
            "message": STRING,
            "artifacts": {"type": "array", "items": STRING},
        },
        ("status", "message", "artifacts"),
    ),
]


def tools_for(actor):
    return LEADER if actor == "leader" else WORKER


def validate(schema, value):
    if (
        not isinstance(value, dict)
        or set(value) - set(schema["properties"])
        or set(schema["required"]) - set(value)
    ):
        raise ValueError("缺少或包含未定义的参数")
    for key, item in value.items():
        spec = schema["properties"][key]
        if spec["type"] == "string" and not isinstance(item, str):
            raise ValueError(key + "应为字符串")
        if spec["type"] == "array" and (
            not isinstance(item, list) or any(not isinstance(x, str) for x in item)
        ):
            raise ValueError(key + "应为字符串数组")
        if "enum" in spec and item not in spec["enum"]:
            raise ValueError(key + "取值无效")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--actor", required=True)
    parser.add_argument("--turn", required=True)
    args = parser.parse_args()
    team = Team(args.root)
    import sys

    for line in sys.stdin:
        try:
            req = json.loads(line)
            if not isinstance(req, dict) or "id" not in req:
                continue
            method = req.get("method")
            if method == "initialize":
                result = {
                    "protocolVersion": req.get("params", {}).get(
                        "protocolVersion", "2024-11-05"
                    ),
                    "capabilities": {"tools": {}},
                    "serverInfo": {
                        "name": "team_playground",
                        "version": "0.1-prototype",
                    },
                }
            elif method == "tools/list":
                result = {"tools": tools_for(args.actor)}
            elif method == "ping":
                result = {}
            elif method == "tools/call":
                try:
                    params = req.get("params", {})
                    chosen = next(
                        (
                            x
                            for x in tools_for(args.actor)
                            if x["name"] == params.get("name")
                        ),
                        None,
                    )
                    if not chosen:
                        raise ValueError("工具不属于当前执行身份")
                    validate(chosen["inputSchema"], params.get("arguments", {}))
                    value = team.call(
                        args.actor,
                        args.turn,
                        chosen["name"],
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
                except Exception as error:
                    result = {
                        "isError": True,
                        "content": [{"type": "text", "text": str(error)}],
                    }
            else:
                print(
                    json.dumps(
                        {
                            "jsonrpc": "2.0",
                            "id": req["id"],
                            "error": {"code": -32601, "message": "Unknown method"},
                        }
                    ),
                    flush=True,
                )
                continue
            print(
                json.dumps({"jsonrpc": "2.0", "id": req["id"], "result": result}),
                flush=True,
            )
        except (ValueError, TypeError, KeyError) as error:
            print(str(error), file=sys.stderr, flush=True)


if __name__ == "__main__":
    main()
