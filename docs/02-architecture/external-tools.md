# 外部工具注册与调用

状态：支持外部工具注册、发现、逐工具自动审批或每次确认。交接工具属于外部工具，平台不内置 submit_result 或 GitHub 回调业务。

## 复用结论

Dify 支持添加 MCP 服务、发现工具并让 Agent 自主调用，也可把工作流暴露为 MCP。Marketplace 专用插件依赖 Dify 插件运行时。这里采用标准 MCP 和官方 Go SDK，无需部署 Dify；Skill 继续使用原生目录发现和渐进加载。

来源：
- https://dify.ai/blog/v1-6-0-built-in-two-way-mcp-support
- https://github.com/langgenius/dify-plugin-daemon
- https://github.com/modelcontextprotocol/go-sdk
- https://developers.openai.com/codex/mcp/

## 平台责任

管理员在工具页面/API 注册外部 HTTP 或 stdio MCP 服务，保存连接配置，发现工具清单。在 Agent 上选择服务及具体工具，并设置自动审批（默认）或每次确认。平台把连接和 allowlist 写入 Codex 原生配置；调用由原生执行器执行，原始工具事件与会话一并保存。配置快照随会话保存，修改注册配置作用于新会话；外部服务负责凭据撤销。平台不增加业务结果状态机、提交结果函数或 HTTP 回调投递表。

同一 Go 进程和 SQLite 保存服务注册与 Agent 绑定。配置仅引用后端环境凭据，不在工具定义中放密钥。stdio 服务是受信任的管理员本机命令，注册不执行，点击发现或运行 Agent 时执行。

## 外部交接示例

独立 MCP 工具持有业务 Schema、交接规则、结果持久化及 GitHub/GitLab 请求。平台通过标准协议连接。Agent 在自然语言澄清中根据 Skill 和用户确认主动调用该工具；Pipeline 仍由 GitHub/GitLab 管理。

验收：页面/API 注册与发现；按 Agent 限制工具；重启后配置保留；真实 Codex 调用并恢复原会话；外部示例保存结果并发送 HTTP。自然语言聊天不触发回调。工具调用可在执行日志查看。

## 使用路径

1. 管理员打开 `/tools`，填写名称、标识和连接方式；HTTP 使用 Streamable HTTP MCP，stdio 使用命令、参数和可选启动目录。
2. 保存后点击“发现工具”，平台执行 MCP initialize / tools/list，展示外部服务提供的名称、说明和 inputSchema。
3. 在 Agent 配置中勾选具体工具。首次创建会话时，连接和工具范围进入配置快照；原生执行器负责调用，平台不代理或解释业务参数。
4. 用户继续自然语言对话，Agent 按自己的指令和 Skill 决定调用时机。工具日志进入现有会话事件。

API：管理员调用 `GET/POST /api/tool-servers`、`PUT /api/tool-servers/{id}`、`POST /api/tool-servers/{id}/discover`。Agent 的 `tool_servers` 示例为 `[{"server_id":"pipeline-handoff","tools":["submit_handoff"]}]`。`resolved_tools` 为服务端管理的会话快照字段，不接受管理 API 写入。

Python SDK 提供 `tool_servers()`、`register_tool_server(config)`、`discover_tool_server(id)`。发现不执行业务工具，但 stdio 启动的是管理员信任的本机程序。

凭据通过后端环境变量名引用。Agent 显式环境覆盖或清除优先于继承。不会在注册 API 返回凭据值。原生高级 TOML 配置仍可用；注册工具与其发生名称冲突时明确报错。

外部工具示例及责任边界见 [pipeline-tool](../../examples/pipeline-tool/README.md)。示例独立编译；没有注册时平台不加载它。Skill 继续通过 Agent 配置里的本机目录选择，由原生执行器渐进读取，无需新增 Skill Hub 服务。

## 调用审批

配置保存在 Agent 的工具绑定中：

```json
[{"server_id":"pipeline-handoff","tools":["submit_handoff"],"approvals":{"submit_handoff":"confirm"}}]
```

`auto` 为默认值，选择工具即允许 Agent 自主调用；`confirm` 表示每次调用都等待会话用户确认。只允许为已选工具配置这两个值。两种模式都由原生执行器调用外部服务，平台不解释业务参数。

- 自动审批：原生工具配置为 `approval_mode = "approve"`。这是操作者的预授权，不是让另一个模型判断。
- 每次确认：原生工具配置为 `approval_mode = "prompt"`，通过 Codex app-server 的 `mcpServer/elicitation/request` 接收确认请求。对话页显示原生调用说明及详情；会话所有者或平台管理员可以批准本次或拒绝本次。决定只针对一个请求，不持久授予更多权限。
- Codex 保持原 Session 与当前 turn，收到决定后继续；不重新提交用户消息，也不重放工具。用户拒绝后 Agent 能继续解释或调整计划。
- 请求和决定存入 SQLite，刷新页面仍可见。停止会话取消待确认请求；进程异常结束或平台重启后，遗留请求作废。重复、过期或跨用户批准均被拒绝。
- 所有会话统一使用双向 app-server，消息增量直接更新同一条回复；人工确认使用同一连接。保持 CODEX_HOME 和 thread ID，原有 exec 会话可直接接续。
- 手动确认仅覆盖原生 MCP 工具调用审批表单。不支持外部服务任意表单、URL 登录或新增 shell 提权审批；遇到不支持的请求会明确失败，不自动接受。

API：查询 `GET /api/conversations/{id}` 返回 `approvals`。对待确认 ID 调用 `POST /api/conversations/{id}/approvals/{approval}`，内容为 `{"decision":"accept"}` 或 `{"decision":"decline"}`。Python SDK 对应 `decide_tool(conversation_id, approval_id, decision)`。

会话继续沿用创建时的配置快照；修改审批配置用于新会话。人工确认期间原生进程和一个本机执行位置保持占用，这版不做跨机器迁移或释放进程再恢复。

原生协议在 Codex 0.151.0 验证，细粒度审批使用 experimentalApi capability。工具审批不替代需求确认，也不改变现有文件沙箱和 Hook 信任设置。高级原生 TOML 的工具不属于注册工具选择器；其审批配置仍由高级配置管理。

验证证据见 [外部工具验证](../03-delivery/external-tools-verification.md)。
