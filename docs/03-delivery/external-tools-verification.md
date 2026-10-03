# 外部工具注册验证

## 已验证

- HTTP MCP：官方 Go MCP SDK 服务的 initialize/tools/list 发现、工具 Schema 保存、普通用户管理接口拒绝、Agent 选择范围及未知工具拒绝。
- 数据恢复：关闭并重开 SQLite 后，服务清单、Agent 绑定及会话快照保持；连接变更使旧发现清单失效。
- stdio MCP：真实浏览器注册独立 pipeline-tool 程序，发现 submit_handoff 名称与参数，未调用外部写操作。
- Python SDK：注册、发现、查询经 HTTP 保留认证与原始 Schema。
- 真实 Codex：原生配置加载选定只读 MCP 工具，实际调用两次，中间通过保存的 thread ID 恢复同一 Session。Skill 引用渐进读取与原生 Hook 同时通过。命令：`AGENT_PLATFORM_LIVE=1 go test ./internal/platform -run TestLiveNativeSkillHookAndResume -v -count=1 -timeout 5m`，通过，耗时约 75 秒。
- 外部交接示例：测试 HTTP 接收端收到一次请求；文档内容先保存为快照；重复交接不重放；非成功响应保留 uncertain；变更内容不能覆盖已交接证据。

## 工具审批

用户明确选择逐工具配置、默认自动审批后，已实现 `auto` / `confirm`。

- 真实 Codex 调用带写入行为的外部 MCP：自动模式直接调用；确认模式先发原生审批请求，批准后才写入临时记录；恢复同一 Session 后拒绝下一次调用，记录保持不变。命令：`AGENT_PLATFORM_LIVE=1 go test ./internal/platform -run TestLiveToolApprovalAndResume -v -count=1 -timeout 5m`。
- 权限与恢复：会话所有者能处理请求，其他用户不能处理；刷新可读到待确认请求；重复和过期决定被拒绝；停止及启动清理会使待确认请求作废。
- 真实浏览器：隔离实例使用真实 Codex 发起 MCP 确认；刷新页面保留请求，批准按钮释放调用并实际保存标记；同一会话第二轮拒绝后标记不变，界面显示已拒绝且停止。Agent 编辑页正确回显每次确认和默认自动选项。
- 原生 Hook：确认模式继续执行操作者已信任的 Stop Hook。原生验证通过，耗时约 85 秒；停止期间的进程级测试通过。
- 工程检查：gofmt、Go vet/race/test/build、前端行为测试、Python SDK 测试与 Ruff 均通过。已更新本机服务，原有会话保留。

## 验证边界

真实 GitHub 工作流交接尚未验证。真实外部写工具与示例 HTTP 投递分别验证，不能据此声称 GitHub dispatch 已成功。

stdio 示例当前采用集成预配置的单次交接目标。多任务需要各自的目标配置和结果文件，不能共享同一个结果文件。生产级投递重试、回执和多租户隔离不在此验证范围。

原生历史、日志与本地截图存放在忽略的运行目录，不提交本机路径、凭据或私人 Session 到仓库。
