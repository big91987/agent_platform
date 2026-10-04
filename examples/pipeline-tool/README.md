# 外部交接工具示例

这是独立的 MCP 程序，不是平台内置工具。平台不依赖此包，也不知道 PRD、阶段、审批或 GitHub。集成维护者可单独构建、部署、替换工具；也可以用其他语言实现标准 MCP。

## 运行与注册

```sh
go build -o <tool-bin>/pipeline-tool ./examples/pipeline-tool
```

由集成维护者准备配置，保存在 Agent 工作区之外。以下为 GitHub workflow_dispatch 示例；URL、工作流与参数名由集成决定。

```json
{
  "workspace": "<shared-root>/task-workspace",
  "result_file": "<shared-root>/handoffs/task-123.json",
  "dispatch_url": "https://api.github.com/repos/<owner>/<repo>/actions/workflows/<workflow>.yml/dispatches",
  "token_env": "PIPELINE_TOKEN",
  "ref": "main",
  "inputs": {"issue_number": "123"}
}
```

结果目录须已存在；`result_file` 对应一次交接。平台和本机 Runner 均可读取该共享路径。接收工作流声明 `issue_number`、`result_file`、`result_sha256` 输入，读取保存的文档快照作为后续阶段输入。工具只确认请求被接收，不代表后续工作流执行成功。

在平台“外部工具”注册 stdio：

```json
{
  "id": "pipeline-handoff",
  "name": "项目交接工具",
  "enabled": true,
  "connection": {
    "command": "<tool-bin>/pipeline-tool",
    "args": ["-config", "<integration-config>/task-123.json"],
    "env_vars": ["PIPELINE_TOKEN"]
  }
}
```

`PIPELINE_TOKEN` 由平台进程环境提供，不写入配置。点击“发现工具”，在 Agent 配置勾选 `submit_handoff`。程序仅在发现或执行时由原生 MCP 客户端启动，无需额外常驻服务。

此示例把一个交接目标固定在一份注册配置上，适合单任务闭环验证；多个任务同时运行时必须分配各自的配置与结果路径，不能共用结果文件。更广泛的业务集成可自行提供 HTTP MCP 服务管理业务目标，不需要修改平台。

## Agent 指令或 Skill 中的约定

> 先通过自然语言澄清。完成文档并取得用户对交接的明确确认后，调用 submit_handoff，传入 summary 与文档相对路径 artifacts。以工具返回值报告真实交接结果；未调用、失败或结果 uncertain 时，不宣称后续流程已启动。

这是具体 Agent 的配置，不能放进平台所有 Agent 的全局提示词。

## 执行行为

1. 从受限工作区读取最多 5 MiB 的 UTF-8 文档，保存内容快照、摘要和 SHA-256；拒绝越出工作区的文件。
2. 先持久化结果，再由工具自身发送 HTTP 请求到配置的 GitHub API；不会接受模型传入目标地址或凭据。
3. 工具持有独占锁。已 accepted 的相同交接不再发送；不同内容不能覆盖交接证据。
4. 连接中断、HTTP 拒绝或进程中断保留 uncertain。可用完全相同的参数显式重试；接收脚本必须按交接文件、摘要、当前阶段和轮次去重。此保证依赖配套 pipeline.py，不能把旧的、不去重的接收端用于可重试发送。
5. QA 的配置 inputs.after=qa 时，必须明确传 target_stage=requirements/design/development 返工，或 report 完成交付；缺失目标会拒绝，不默认交付。工具校验允许的目标，原始值纳入摘要并交给接收端；其他产品阶段也可指定任一更早阶段，或明确指定紧邻的下一阶段；不能跳过阶段。用户明确要求回退时直接执行，Agent 自行建议回退先请用户确认；summary 记录依据、修改要求与受影响结论。

例子使用 macOS/Linux 文件锁。当前示例不是通用可靠消息投递系统，也不支持自动工作流结果回传。GitLab 对应 API 的请求格式应由另一个外部工具实现。

For reusable stage Agents, use `--registry <private-directory> --stage requirements`
(or `design` / `development`) instead of a fixed `--config` file. The external
Runner writes `<sha256(canonical-workspace)>/<stage>.json`. The stdio server
inherits the Agent's working directory and resolves only that workspace/stage;
unregistered workspaces fail explicitly. Tool discovery does not need a task.

The receiving Runner can validate the exact payload using
`pipeline-tool --verify-result <receipt> --expected-sha256 <digest>`, which uses
the same canonical digest implementation as the producer. This mode performs no
network calls. It does not replace checking that current workspace documents
still match the accepted snapshot before starting the next stage.


### 无新增需求或设计工作

`requirements` / `design` 的正常相邻阶段交接可以使用空 `artifacts` 数组；`summary` 必须记录本阶段不适用的依据、沿用约束及下游工作，仍进入不可变回执、哈希校验和重复调用去重。已有真实文档可继续附上。此兼容扩展不改变路由，不豁免研发/QA 的交付证据；其他阶段仍拒绝空附件。阶段适用性属于外部 pipeline 的提示词策略，工具不会通过关键词猜测任务类型或伪造用户批准。
