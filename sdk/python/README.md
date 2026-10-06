# Python SDK

薄封装现有 HTTP API，Python 3.10+，无运行时第三方依赖。SDK 不创建账号、不授予权限、不管理原生 Session，也不复制工作目录。

## 安装

在自己的虚拟环境中安装：

```sh
python3 -m venv .venv
.venv/bin/python -m pip install <platform-repository>/sdk/python
```

平台自带的 Runner 示例使用 `scripts/setup-runner.sh` 创建 `.data/runner-venv` 并安装 SDK。SDK 升级后重新运行此脚本，业务 Pipeline 不必每次安装。

## 创建与接续

```python
import os
from agent_platform_client import Client

client = Client(os.environ["AGENT_PLATFORM_URL"], os.environ["AGENT_PLATFORM_TOKEN"])
identity = client.me()  # 平台验证 Token；返回用户身份。

receipt = client.invoke(
    "请读取现有代码，澄清读书笔记需求。",
    agent_id="<agent-id>",
    workspace_path="/workspaces/<project>/<task>",
    request_id="<source-event-key>",
)
conversation_id = receipt["conversation_id"]
print(receipt["conversation_url"])
result = client.wait(conversation_id, message_id=receipt["message_id"], timeout=600)

followup = client.invoke(
    "每本书先保留一篇笔记。",
    conversation_id=conversation_id,
    request_id="<next-source-event-key>",
)
```

`workspace_path` 是平台机器看到的目录，不是调用方任意的本地目录。创建后固定，接续省略它。Token 绑定用户；平台仍检查 Agent 授权和会话归属。不要把 Token 放进 URL、命令行参数、产物或日志。

`request_id` 在 SDK 中必填，应使用外部系统稳定的输入事件标识。同一输入的 HTTP 重试必须使用同一个值；新输入使用新值。SDK 不生成新的标识去掩盖响应丢失，不自动重放 POST。输入被接受后，用户可以在网页或 API 中继续同一会话。

`wait` 返回当前 API 快照，不转换 Agent 的业务状态。`idle` 仅表示当前没有执行；Agent 仍可能在等待澄清。指定 `message_id` 后，该输入结束即可返回，不必等待随后排队的其他输入。

## 进展与产物

```python
from contextlib import closing
from pathlib import Path

last_event_id = 0  # 调用方保存观察游标。
with closing(client.stream(conversation_id, after=last_event_id)) as stream:
    for event in stream:
        last_event_id = event["id"]
        print(event)  # 包括平台事件外壳与 raw 原生事件，不只取 message。
        if event["message_id"] == followup["message_id"] and event["type"] in (
            "platform.execution.completed",
            "platform.execution.failed",
            "platform.execution.stopped",
        ):
            break

events = client.events(conversation_id, after=last_event_id)
files = client.artifacts(conversation_id)
Path("prd.md").write_bytes(client.read_file(conversation_id, "docs/prd.md"))
```

事件流忽略心跳，保留完整事件。断线后以最后收到的事件 ID 重连；不会提交消息或重启 Agent。退出读取时关闭迭代器；HTTP timeout 控制网络等待，不限制后台任务寿命。已结束的会话要查历史，可用 `events` 或 `conversation`，不必永久等待 SSE。

Agent 回复的 `parent_id` 指向它对应的用户输入 ID；Runner 可据此选择本轮回复，避免误用其他轮次。文件访问依然由平台校验归属、路径及符号链接边界。

## 查询、停止与失败

- `me()`、`agents()`：身份与授权 Agent。
- `conversations()`、`conversation(id)`：已有会话及状态、消息、文件。
- `stop(id)`：终止当前执行，保留队列和历史。
- `stop(id, expected_message_id=..., discard_queued=True)`：只有最新用户输入仍是指定 ID 时停止并将待执行输入标为停止；否则返回 409，不更改执行。用于调用方明确撤回一次工作，保留历史。
- `steer(id, message_id)`：将已排队输入引导给运行中的 Agent；不新建输入。不能引导时返回 409，输入仍在队列。
- `workspace_access(id, read_only=True)`：在无执行时切换工作区访问模式；活跃执行返回 409。只读会话可与同目录写会话并行，写会话仍互斥。原生 Codex 使用 read-only sandbox，外部工具权限仍按注册与审批配置。恢复写入用 `read_only=False`。
- `continue_queue(id)`：明确释放已暂停队列；不重放已失败或已停止的输入。
- `close(id)`：关闭会话，保留记录。

`APIError.status_code` 保留 HTTP 拒绝原因，例如 401、403、409。网络问题抛出 `ConnectionError`；等待到期抛出 `TimeoutError`，只结束观察，不停止 Agent、不删 Session。失败／停止的会话直接返回快照，调用方决定后续操作。SDK 拒绝跟随 HTTP 重定向，避免把用户 Token 转发给另一地址。

## 注册外部 MCP（管理员）

管理员 Token 可注册并发现工具；普通调用用户不能管理工具来源。

```python
server = client.register_tool_server(
    {
        "id": "external-tools",
        "name": "外部工具服务",
        "enabled": True,
        "connection": {"url": "https://tools.example/mcp"},
    }
)
inventory = client.discover_tool_server(server["id"])
print([tool["name"] for tool in inventory.get("tools", [])])
```

然后在智能体配置中选择该服务的具体工具。注册保存的是连接配置，工具代码与参数 Schema 由外部服务提供；平台不安装 Dify 专用插件。Skill 仍选择本机 Skill 目录并由原生执行器加载。

Agent 的工具绑定可配置 `approvals: {"tool_name": "auto"}`（默认）或 `"confirm"`。确认请求从 `conversation(id)["approvals"]` 查询，使用 `decide_tool(id, approval_id, "accept")` 或 `"decline"` 决定本次调用；网页也有同样按钮。完整说明见 [外部工具设计](../../docs/02-architecture/external-tools.md)。

## Workflow entry (0.2.0)

```python
run = client.start_workflow(
    "<workflow-id>",
    "Task text",
    workspace_path="<dedicated-clean-checkout>",
    request_id="<stable-source-task-key>",
    parameters={"issue_number": "42"},  # optional, interpreted by configured connectors
)
run = client.workflow_by_request("<stable-source-task-key>")
receipt = client.workflow_message(
    run["id"], "Additional requirements", request_id="<stable-source-message-key>"
)
state = client.workflow_run(run["id"])
```

`POST /api/workflow-runs` starts a persisted graph; `GET /api/workflow-runs/by-request?request_id=...` resolves only the caller's event key. Parameters are optional and frozen with the Run. Existing callers need no changes. The caller prepares an independent checkout within configured Connector roots; the GitHub adapter does this automatically. GitHub-specific values are not required by the core API.

`POST /api/workflow-runs/{id}/messages` atomically selects the current Agent and saves an input with its event key. A duplicate returns its original message receipt even after handoff. Optional `seq` pins the intended execution. HTTP 409 means the current step cannot safely receive a new message; inspect state and use supported recovery, do not guess a conversation or recreate the Run.

`workflow_runs(workflow_id=..., before=...)` returns one page (at most 200); use its last ID for the next cursor. `workflow_command(run_id, action, seq=..., ...)` supports stop/resume/return/decision. Control commands are explicit and are not automatically replayed after an unknown transport result. `wait_workflow` returns when waiting, failed, stopped or completed; waiting may mean clarification, not task success. CI can return immediately after the start/message receipt; native execution continues on the platform.


Retained command diagnostics: `client.workflow_command_output(run_id, seq, offset=0)` returns one page with `output`, `next_offset`, `eof`, and `truncated`. Follow offsets until `eof`; `truncated` still means the 8 MiB archive cap omitted later output. The Run owner's token is required. A legacy receipt without `log` metadata has no archive and returns 404. This read never reruns a command and does not interpret its exit status. Requires a platform version with command log archives; upgrading the SDK alone cannot recover old omitted logs.
