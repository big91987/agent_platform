# Python SDK 与 Runner 验证

验证日期：2026-10-01。范围为现有本机平台、真实 Codex、Python SDK、GitHub Runner，以及本机执行的 GitLab 接入脚本。真实 GitLab 部署和云端沙箱未验证。

## 交付内容

- `sdk/python/`：可安装、无运行时第三方依赖的 Python 客户端，复用现有 API。
- `scripts/setup-runner.sh`：创建 Runner 虚拟环境并安装 SDK，可重复执行以升级。
- `examples/github/`：真实 GitHub 接入改用 SDK，按输入 ID 选择对应的最终回复，支持分页读取 Issue 会话关联。
- `examples/gitlab/`：准备独立的持久项目 clone、任务分支和选定的 `handoff/` 文件，再用 SDK 调用平台，保存 receipt 与本轮结果。
- `messages.parent_id`：向 API 暴露数据库已有的输入与回复关系，未增加表、迁移或新的状态机。

Runner 准备目录，平台负责身份、授权、保存输入及 Session 接续。SDK 不生成账号、不克隆仓库、不代替 Agent 作业务判断，不自动重试有副作用的请求。

## 实际运行证据

### SDK 与共享目录

创建阅读进度需求澄清会话 `07d99dccc9fc708a911e807f53e0deaa`，通过实际的 `prepare.sh` 和安装后的 SDK 提交：

1. 从真实产品仓库的输入提交创建独立 clone 和任务分支，复制 `handoff/goal.md`。
2. 删除模拟 CI 临时 checkout，再启动平台 Agent。Agent 实际读取交接目标、项目 AGENTS.md 和产品代码，询问进度单位、阅读中状态及更正行为；没有替用户决定或修改产品代码。
3. 相同输入及请求键重试返回相同会话和输入 ID、`duplicate: true`；消息数量不变，不启动第二次执行。
4. JSON 事件查询和带游标的 SSE 都取得完整原生事件外壳；实际下载交接文件并与工作区字节比较一致。
5. 在已空闲会话调用停止、继续队列，记录保留；重启平台后提交新问题，仍使用同一个原生 Session、同一工作目录，并正确复述未决需求。
6. 浏览器用调用账号打开该会话，显示两轮原生进展、回复、交接文件及累加事件。

目录准备另行实跑验证：重复准备保留 Agent 修改；输入提交不匹配、Git 元数据缺失时拒绝覆盖。目录不会因为 Job retry 被 reset 或重新 checkout。

### GitHub 正式 Runner

- [接入 PR #67](https://github.com/big91987/reading_list/pull/67)：已合入，工作流使用安装 SDK 的虚拟环境。
- [实际运行](https://github.com/big91987/reading_list/actions/runs/36840662517)：成功完成身份校验、提交、等待和 Issue 回复。
- [Issue #66 的本轮回复](https://github.com/big91987/reading_list/issues/66#issuecomment-5928314067)：只复述未确认的笔记数量、删除与保存规则，未擅自确认或修改产品代码。
- 接续原会话 `ad6319dbc2271e8c28c08690674d369b`，原生 Session 与工作目录保持不变。回复的 `parent_id` 对应本次输入 ID 57。
- 通过 SDK 下载实际需求决策文件，3,168 字节，与该项目文件完全一致。

## 自动检查

`scripts/verify.sh` 全部通过：逐个 shell 语法检查、Ruff lint/format、4 个 SDK HTTP 行为测试、3 个前端行为测试、Go vet、Go race 测试及构建。SDK 测试只覆盖请求/错误/事件/等待边界，不将 HTTP 测试替身视为真实原生执行证明。

SDK 拒绝凭据重定向、不自动重放 POST，并将响应截断报告为连接错误。实际无效 Token 返回 401。差异空白检查和共享资产 sanitization 扫描通过。构建有现有 macOS CGo 链接警告，未导致检查失败。

## 限制与接续方式

- 本机运行不是不可信代码沙箱；GitLab 示例尚未安装到真实 GitLab Runner。
- 共享目录必须被双方以同一路径访问并长期保留。文件共享不代替写入协调；提交后 Runner 不再修改该目录。
- SDK 的等待到期只停止观察，不停止 Agent。`idle` 表示当前没有执行，不代表业务任务完成。
- 失败或停止的输入不会自动重放；输入已保存但队列暂停时，用户查看原因后明确继续队列。
- 账号、Token、原生历史、临时 CI checkout 和本机验证截图都留在忽略的本地存储。

使用方法见 [SDK](../../sdk/python/README.md)、[GitHub](../../examples/github/README.md) 和 [GitLab](../../examples/gitlab/README.md)。
