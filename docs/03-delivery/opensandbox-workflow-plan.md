# OpenSandbox 工作流改造实施计划

日期：2026-10-13。状态：实施中；用户已授权在当前隔离 worktree 实现、部署到 112，并通过测试仓新 Issue 类型启动独立 Pipeline。当前分支 `codex/opensandbox-workflow`，起点 `65b4d3e`。

**Goal:** 在 112 提供隔离部署的 Agent Platform，画布以 Agent、执行、人工确认、结束四类节点构建流程，使用 OpenSandbox 执行任务，并以测试仓独立入口验证真实交付。

**Architecture:** 平台保存流程、权限、Run 和回执；OpenSandbox 管理 Run 执行环境。执行节点支持代码、命令和官方 SDK 的 MCP 工具调用；Agent 与工程步骤使用同一任务环境，角色会话独立。已有本机 Run 和旧 Connector 定义保持兼容，不把运行历史原地转换。

**Tech Stack:** Go 平台；原生 Codex；OpenSandbox Docker 后端；Python/TypeScript 任务运行时；MCP 官方 SDK；现有 GitHub 入站与报告发布工具。

**Spec:** `docs/01-product/software-factory-assessment.md` 第 0 节，加本节记录的用户后续确定项：四类节点；Agent 内部 Hook 保留原生职责，外部动作复用执行能力；外部工具统一 MCP；项目脚本不限制 Go；Example 复制入项目后按需调整；112 实际部署及独立新 Issue 入口是完成条件。

## 当前实施状态（2026-10-13）

| 部分 | 当前证据 | 仍缺什么 |
|---|---|---|
| MCP 调用基础 | 真实 HTTP/race、结构化结果、业务错误、取消、关闭超时和丢响应不重放通过 | 节点配置、冻结快照、调度与 UI 接入 |
| 112 OpenSandbox | 1.1.1，127.0.0.1:8095 healthy；未认证401；单个 sandbox 创建→Running→删除404 | Run/Agent/文件/命令集成、停止恢复、服务登出重启 |
| 四类画布与112平台8795 | 尚未实现部署 | 配置、执行、页面、正式部署核验 |
| 测试仓独立入口 | 尚未创建新Issue/Pipeline | 仓库自有Harness、新类型去重、真实整链 |

112 基础设施探针 ID `a2548a85-d279-4b82-93e2-4991d11feaa8`，已删除；报告6事件、SHA256 `671c0e833897ad2882a4f5951b323469652a46a4632465830f93b9bb776cfbe5`。当次 execd 镜像摘要 `6cf7dba2f21f0b536e100563d841ac58a9f31c2b0a081b7ac76796a24d6f47e2`，Python镜像摘要 `a6e34c598f2467ed0e9a8d349809fcd8b5c603269512df273a0bb1784edc11b1`。服务目录0700/配置0600已实测。此为基础设施回归，不是产品验收。

## 全局约束

- 保留现有 8788/8793 与测试仓现有流水线；候选使用 112 独立目录、数据库及端口，初选平台 8795、OpenSandbox 8095，启动前实际核占用。容器端口由 OpenSandbox 分配，不抢占宿主业务端口。
- 平台只在开发分支提交/推送，不创建平台 PR、不合 main。测试仓必要 PR/合并及验证入口在既有授权内；Harness 验证 Issue 标题明确类别，新触发用独立 label/模板路由，与旧入口互斥。
- 通用实现、部署与安装路径归源仓；项目 YAML/脚本归测试仓。不存在平台 checkout 的新仓也能按文档接入。
- 不转移整个宿主 HOME/秘密到容器；使用受信、最小的凭据引用。Issue 输入不能授予宿主挂载或执行权限。
- 不手改数据库、原生历史、回执或冻结 Run；未知写入结果不盲重试，不以宿主代执行掩盖沙箱不通。
- 新执行节点使用同一 Run 环境；原生会话重启不能无声换 ID。正式结果和需人审的材料有持久归档。
- Model Relay 产品实现/验收由 Pipeline Agent 进行；维护者仅验证 Harness 能力和证据。

## Review Focus

1. MCP 服务在调用后断线：保留不确定结果，禁止自动重复写入；停用/修改连接后不能借旧 Run 偷用新身份。
2. OpenSandbox 创建成功但平台未收到响应：通过 Run 关联核对，不重复创建；环境丢失不回退宿主。
3. 阻塞或取消中的 Agent/代码：实际所属进程停止、回执保留，不能仅把 UI 状态改为 stopped。
4. 旧工作区路径、未知代码语言、跨目录路径或缺运行时：明确拒绝/诊断，不吞配置或扩大访问范围。
5. 重复 Issue 投递、页面重载、平台重启：配置不丢失、Run/环境去重、原生会话身份连续。

## Task 1：执行能力与 MCP 调用

Files: `internal/platform/tools.go`、新增 `execution_mcp.go`/测试；后续 `workflow_execution.go`、`workflows.go`、`workflow_engine.go`、`workflow_runs.go`。

Interfaces: 复用 `RegisteredToolServer/MCPConnection`；抽出 SDK 连接函数 `connectMCP(ctx, connection) (*mcp.ClientSession, error)`，发现与调用共用；`callMCPTool(ctx, connection, name, arguments)` 返回原生工具结果。工具业务错误与传输错误分别记录，原始文本不重写。

- [x] 先写真实本地 MCP HTTP 服务回归：参数传递、结构化返回、工具业务失败、取消和不响应清理；运行确认红，再接通官方 SDK。
- [ ] 引入执行节点配置：code/command/mcp，保存及启动校验；旧 connector 仅兼容。代码支持 Python/TS，命令允许项目自身语言/构建入口，不固定 Go。
- [ ] 接入现有持久调度、取消、结果与失败线路，保存冻结的连接/工具契约；写入未知结果不重放。
- [ ] 保存/重载/权限撤销/重启回归通过；实际 MCP 调用日志可见且不泄认证。

## Task 2：OpenSandbox 与原生执行

Files: 新增 `internal/platform/sandbox*.go`、部署目录 `deploy/opensandbox/`；接入 `codex.go`、`codex_appserver.go`、命令执行与文件访问。

Interfaces: Run 关联实际 sandbox ID；受信环境配置声明服务与镜像。采用官方 SDK/API，具体接口以所固定发行版核验。原生 app-server 双向消息、工具审批与停止沿现有调度接口接入，不将 Codex 换成临时单次 shell Agent。

- [ ] 在 112 核实 Docker、可用端口、网络及认证条件；固定 OpenSandbox 发行版本和镜像。
- [ ] 有界真实探针证明创建、命令、文件、原生双向通信及销毁；探针不冒充 Pipeline 验收。
- [ ] Run 创建/核对/释放持久化，输入与文件仅进入所属环境；实现保留与明确释放，不承诺内存恢复。
- [ ] 同环境 Agent + Python + TS + 项目命令 + 浏览器；停止、服务失败、平台重启恢复回归，补充不同 Run 隔离验证。

## Task 3：四类节点配置界面

Files: `web/workflow-model.js`、`web/workflow-panel.js`、`web/workflows.js`、`web/workflow-runs.js`、现有 MCP 管理页及对应测试。

- [ ] 新建流程只呈现 Agent/执行/人工确认/结束；执行按 code/command/mcp 显示必要字段。
- [ ] MCP 工具发现和 Schema 输入、上游参数/结果引用；短代码直接编辑，复杂逻辑引用项目脚本。
- [ ] Agent 配置基础/高级分组，角色与任务分离；人工确认与权限审批区分；固定测试不能被人审跳过开关跳过。
- [ ] 浏览器实际保存重载、执行结果、失败/停止恢复及窄屏可用验证。

## Task 4：标准部署和仓库自有 Harness

Files: `deploy/opensandbox/`、`examples/platform-workflows/` 入站/安装工具及 SDK；测试仓复制后的独立 `.github/workflows/` 与项目 Harness 目录。

- [ ] 112 新实例通过标准命令部署，独立数据/端口，健康版本与源码一致；最小凭据配置不公开。
- [ ] 新 Example 入站仅使用仓库文件/固定工具版本与平台 API，申请环境后在其中准备项目；不执行平台源码路径。
- [ ] 检查既有 Issue/PR/Actions，再配置新 label/Issue 模板及互斥路由，旧类型不触发新 Pipeline，新类型不触发旧 Pipeline。

## Task 5：真实验证与交付

- [ ] 固定验证 `scripts/verify.sh`、gofmt、go vet、相关 race 测试；缺环境与失败如实记录并补齐受支持路径。
- [ ] 独立代码评审；修复实质缺陷后复测。
- [ ] 测试仓正式新 Issue 触发唯一新 Pipeline；Agent 在 OpenSandbox 实际承接任务，代码/命令/MCP、人工确认、交接及报告有真实回执。
- [ ] 重复入站、失败返工、停止/恢复至少各一条真实验证；不要用 Mock 通过替代。
- [ ] 用户可查看 112 平台、Issue、报告；三份既有权威进度/验证文档同步结果。逐项完成审计后才标记目标完成。

## 执行记录

- 2026-10-13：确认原 worktree 为 detached HEAD，保留原文档后创建 `codex/opensandbox-workflow`。现有工具管理已经使用官方 Go MCP SDK；固定节点仍将命令/GitHub 混在 Connector 中。先复用 SDK 与持久调度，避免再造协议。
- 当前未部署、未创建测试仓 Issue、未改变旧服务。112 使用既有 `ds112` 会话开始只读核验；端口只是候选，不宣称已分配。

- 2026-10-13 首批实现：MCP 基础层真实 HTTP/race 回归和 go vet 通过，尚未接入执行节点调度。112 标准部署 OpenSandbox 1.1.1/8095 healthy + 401认证检查；固定 pip25.3及 WorkingDirectory 格式修复已通过原安装器应用。已开始有界生命周期探针，112平台8795/新Pipeline仍待实现。

- 本轮最终复核：112 唯一基础设施探针 probe-01 exit0，实际创建→Running→DELETE→GET404；未创建产品任务，未执行产品验收。Go 平台全包 race exit0/58.225s，ruff/安装器2测试及 diff检查通过。scripts/verify.sh 已纳入安装器回归，但全套标准脚本仍未执行。

- 标准验证最终结果：按 README 的 `examples/github/install-tooling.sh` 安装依赖后，第二轮 `scripts/verify.sh` exit0；完整日志 116458 bytes/SHA256 `0c4d2203143df31b4e1420879ecc6ad43116ceb400f88caf4aff46256996d187`，位于 ignored `.data/opensandbox-verification/verify-02.log`。包括浏览器4项、安装器2项、SDK9项、GitHub工具96项、local-preview11项、preview控制器60项、workflow模板107项、Go vet/race/build，平台 race57.646s。首轮缺 Playwright 的原失败日志 verify-01.log 保留；未跳检查。新增 MCP 写入丢响应回归实际只收到一次写请求。
