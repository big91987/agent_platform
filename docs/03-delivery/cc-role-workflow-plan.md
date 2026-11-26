# CC 分角色研发与 Issue 入口实施计划

用户于 2026-11-26 明确批准开始调整、新建工作流与仓库入口，保留既有工作流后继续网关迭代。此前确认的角色选择即本计划需求依据。

## 约束与方案

沿现有 Agent/Workflow/Run/Executor 模型增加 Claude Code CLI 执行，不以执行节点包壳冒充 Agent。消息、工具结果、原生会话、权限、停止恢复及交接走正式平台路径；保持原 Codex 行为。原生技能仅加载节点显式选择。需求 defining-platform-products-cn + batch-grill-me，架构 platform-architecture-cn，开发无 Skill；QA 基于 validating-platform-releases-cn，补 SaaS/MaaS/UI 必测范围。两套审查 alibaba/open-code-review 与 archcheck 使用 deepseek-flash，各自保留原始结果、同候选证据，incomplete/error 不得通过。CC 沿用用户现有配置，不强改模型。

不修改冻结 Run，不手改数据库、原生历史或回执，不把本机 CC 连通当远端执行通过。本次新服务运行于本机127.0.0.1:8797（创建前复核空闲），独立数据目录，沿用本机CC模型连接；不修改112或原平台服务。平台开发分支提交推送，禁止平台 main 合并/新 PR；测试仓可按授权 PR 合并。产品实现与验收由 Pipeline Agent 执行。凭据仅私有引用。

### Task 1: Claude Code 原生执行器

负责 internal/platform、cmd、web、deploy 的执行器支持。复用现有 Executor Execute(ctx,Conversation,Message,onEvent) 和审批/状态生命周期。新增 claude 选择及校验、原生 stream-json 消息事件及精确 session-id 恢复、MCP 交接、角色 Skills、同 Run 工作区、停止/超时及安全拒绝不支持的设置。CLI 采用已安装 claude；模型可默认。不得沿用 Codex app-server wire protocol、静默重建丢失会话、跳过审批。本次只支持本机Claude；OpenSandbox中选择Claude必须明确拒绝，远端支持不在本次范围。

- [ ] 先添加真实行为失败测试：executor 保存/恢复、输出、两轮会话、拒绝与取消、不确定启动不重放、Skills 作用域。
- [ ] 实现最小适配；保留 Codex 回归。
- [ ] gofmt/go vet/相关测试及真实 CC 本机探针，分别记录边界。
- [ ] 标准安装与 UI 同步；独立审查后开发分支提交。

### Task 2: 可复用角色流程、双审查和新 Issue 入口

负责 examples/platform-workflows、examples/code-review 及 QA 附件。基于 Task1 executor=claude 创建新图，不覆盖旧图。prepare→requirements→architecture→development→tests→code_review→qa_plan→qa_plan_review→qa→approval→report→publish_check→publish→done，返工/缺输入走现有 handoff/wait。报告节点不用额外 Skill。编排角色指令不复制项目 AGENTS.md。每种审查分别处理规则/模型失败及发现，以同一候选绑定。 本机双审查与发布核对使用管理员配置的固定命令 Connector，仅退出码 0 时继续。发现问题或源码变化时，有权限的操作者检查回执后从正式入口返回开发；模型或工具不完整时先处理实际故障。新入口用独立 Issue form/label 和专用受信 Actions 路由，确保旧入口不重复受理；使用 request_id 幂等。本机服务使用显式选择的Skill资产，并记录版本/摘要。仓库现有本机Runner转发Issue到loopback；不暴露端口到外网。

- [ ] 配置生成与入站路由/去重测试；QA 补充 RPM/TPM/次数/金额/并发各自阈值、租户隔离、真实普通/SSE、UI 首次/窄屏/恢复。
- [ ] 实现并独立审查模板与安装升级，密钥不入图/Issue/源码。
- [ ] 正式创建新工作流/仓库入口并核 API/GitHub 回执。

### Task 3: 正式验证并继续网关

先核 Issue80–后续现存需求去重与在途状态；不重发已创建任务。通过新 Issue 入口验证 CC 角色实际加载、需求/设计产物、人工交接、双审查与 QA 报告。按依赖调度独立需求，可并行但同仓合并前核精确候选及主干变化。记录提交、受理、各阶段及合并时间。完成须正式测试/QA/PR证据，任何工具故障留证，不以流程结束冒称成品通过。

- [ ] 部署独立本机平台服务，新图创建后读取保存配置一致。
- [ ] 隔离 Harness 探针真实输入→交接→恢复可追溯。
- [ ] 更新 Issue81 索引及新入口，首批无冲突需求正式运行。
- [ ] 三份权威交付文档记录实际进度与未验项。

用户后续明确修正：停止112方案，新起本机127服务。用户口述GitLab，但本任务仓库与现有入口实际为GitHub，沿现有GitHub自托管Runner接入，不创建无依据的GitLab集成。
