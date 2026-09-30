# GitHub requirements 接入示例

本示例把 GitHub Runner 作为调用方。真实 Agent 在本机平台后台执行，Runner 只负责请求、结果查询和 Issue 回复。产品 Issue 本身不成为平台内部研发状态机。

## 一次配置

1. 管理员在平台创建需求用途的 Agent，配置指令和原生 Skill。
2. 创建只授权该 Agent 的调用方凭据。
3. 在平台本机的 `.data/github-runner.json` 保存私有配置，并设置文件仅当前用户可读：

```json
{"base_url":"http://127.0.0.1:8788","agent_id":"<agent-id>","token":"<caller-api-token>"}
```

4. 测试项目复制 `requirements.yml` 到 `.github/workflows/agent-platform.yml`，按实际本机 Runner 标签调整 `runs-on`。
5. 配置项目变量 `AGENT_PLATFORM_ROOT=<platform-repository>`。Runner 与平台同机，该目录提供本示例脚本及私有配置。不要把 API token 写入 YAML、Issue、产物或仓库。
6. 关闭同一 Issue 的旧自动执行工作流，避免两个系统同时处理同一输入。

## 用户旅程

- 用户提出真实产品功能 Issue。
- Owner 从 Actions 手动运行 Agent Platform requirements，输入 Issue 号。
- Runner 取得 Issue 的标题、正文和发起人，转换为 API 输入，使用 `github:<login>` 作为 user_id。初次请求用稳定的业务键去重。
- Agent 自己决定是否澄清；Runner 等待当前轮完成，把原始最终回复和 1 小时会话入口贴到 Issue。
- 发起者打开入口，无需管理员账号，在同一个 Agent 会话回复。平台负责 resume；Runner 不持有原生 Session ID，也不维持等待人的长 Job。
- 再次运行 Workflow，message 留空只读取最新结果并刷新入口，不新增输入或执行；有 message 时向关联会话提交新输入。

平台 API 文档页面详细列出 agent_id、user_id、conversation_id、message、request_id、查询游标和临时入口期限。执行失败原样报告，不自动重放未知副作用。当前脚本保存关联的事实是 Issue 评论中的隐藏标记，平台仍是消息与原生会话的事实源。

临时链接是访问凭证，持有者能接续指定会话；公开 Issue 并不能证明点击者是发起者。当前本机实验验证入口与隔离，部署到多人环境须选择合适的链接投递可见性或外部登录方式。
