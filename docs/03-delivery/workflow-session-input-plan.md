# 工作流会话输入改造实施计划

> 实施：superpowers:executing-plans；独立执行器映射与 UI 工作可按 dispatching-parallel-agents 并行，最终独立评审。

Goal：分离角色与任务，交接配置可预览，未完成正常收尾能够有界继续。
Architecture：复用 Agent.instructions、WorkflowNode.prompt、edges 和现有 executor；context_version 冻结兼容协议，原会话消息承载任务。
Tech Stack：Go/SQLite、原生 Codex app-server、原生 JS、Python manifest 安装器。
Spec：[设计](../02-architecture/workflow-session-input.md)

## 全局约束

维护源唯一；不改仓库项目规则，不迁移旧 Run，不改其他实例、不新增供应商适配器、不读本地 New API、不恢复定时任务、不合并源 main。测试不代替真实链路证据。

## Review Focus

- 草稿连线删除/换目标后预览和实际候选必须一致。
- 混合/旧协议不静默丢边或替换在途会话。
- 等待标记与并发用户输入、用户停止的竞争不能自动越权继续。
- 任务修正来自各节点历史用户消息，不混入平台自动继续指令。
- 查看输入需 Run 权限并脱敏，不能暴露 env/token/native secrets。

### Task 1：输入生成与生命周期

Files：internal/platform/workflow_context.go、workflow_control.go、workflows.go、workflow_engine.go、workflow_http.go、workflow_runs.go；对应测试。
Interfaces：workflowSessionPrompt(Workflow, WorkflowNode) (string,error) 共用；context_version=1；preview POST /api/workflows/preview {workflow,node_id} 返回 {session_prompt,input}；actual GET /api/workflow-runs/{id}/steps/{seq}/context 返回 {role_instructions,project_instructions,tools,input,context_version,workflow_revision}；WorkflowStep 增加 wait_kind/wait_reason/continuations。
- [x] 写行为测试：角色无任务、修正及近期交接输入、占位符错误、候选工具、正常收尾续跑/等待/次数/停止/重启。
- [x] 运行测试确认 RED；实现上述真实 API/事务路径，保留 legacy。
- [x] Go 受影响与全套测试 GREEN；提交。

### Task 2：执行器角色与项目规则共存

Files：internal/platform/codex.go、codex_test.go；接入说明。
- [x] 默认/外部工作区准备测试先 RED：seed AGENTS 原样，角色配置一致。
- [x] 统一角色 native 配置注入，移除写 AGENTS 的分支，保留原生发现与已有 developer 指令。
- [x] 受影响测试 GREEN，原生真实路径由 Task 4 核验。

### Task 3：用户配置与预览

Files：web/workflows.js、workflow-model.js、workflow-runs.js、app.js 相关测试。
Consumes：Task 1 preview/context API，字段 context_version、continuation_limit、execution_timeout_seconds。
- [x] 图编辑测试先 RED：新图版本/Session Prompt/交接策略与边同步。
- [x] 节点集中编辑角色关联、Session Prompt、目标策略、持续推进限制及只读预览；明确共享角色编辑；混合兼容提示。
- [x] Run 查看冻结实际输入来源与等待原因；Node/浏览器验证。

### Task 4：正式安装与真实验收

Files：examples/platform-workflows/install.py、prompts、模板、README.md、docs/03-delivery/workflows-verification.md、docs/04-guides/agent-platform-user-guide.md。
- [x] 模板/安装行为测试先 RED：角色指令与节点工作说明分开，新协议与默认持续推进。
- [x] 修正式模板/manifest 路径，备份并升级开发实例；核对旧冻结 Run 不变。
- [x] 从页面创建验收定义，通过正式 API/原生 Codex 测提前收尾→续跑、明确等待→回复、合法交接→下一节点、停止隔离与实际指令共存。
- [x] 新安装/重复 manifest 升级、Go vet/测试、Node、Python 验证；记录每条 AC 实际证据与未测项。
- [x] 独立评审 Critical/Important RED→GREEN，源草稿 PR 更新与交付，禁止合并源 main。

## 实施记录

- 采用既有隔离 worktree 和分支，未新建副本。Task 1 两个原故障先 RED 后 GREEN：角色混入任务、自然收尾不续跑；明确等待/用户回复/预算/重启/停止/修正回退回归通过。
- Task 2 默认/外部项目规则共存先 RED 后 GREEN；全部 Go 回归与 vet 通过。
- Task 3 图模型与界面测试先 RED 后 GREEN，30 项 Node 回归通过；浏览器待验。
- Task 4 正式 installer stage_prompts 缺失先 RED 后 GREEN，13 项安装行为通过；旧 Run 已经正式 stop API 冻结，配置和会话保存。
- Ruling：持续推进由现有平台执行生命周期做完成前检查，不额外复制 Codex 专用 hook 脚本；保留原生受信 Hook。代价：当前旧协议节点仍不自动续跑，不能静默迁移旧会话。

- Final review：独立评审两项 Important（重复目标歧义、实际 Return 反馈遗漏）以及一项模板契约矛盾；模板矛盾按实际澄清误续跑风险提升为 Important。三项均先 RED，随后正式源修复并复验；新协议重复目标明确拒绝并要求合并策略，旧协议保留路线兼容。

- 实测补充修复：真实回复接受后清等待状态（RED→GREEN）；API空env_refs=null升级兼容（RED→GREEN）。
- Task 4 complete：实际页面/原生链路、默认工作区、真实上限、同Run停止恢复、命令失败与独立只读QA回退、新安装/重复manifest升级均保存证据；Go race/vet、31Node、56Python、Ruff/diff通过。
- 明确边界：未重新演练GitHub网络未知副作用；4小时实等/其他执行器/供应商Not Run；旧产品Run冻结未恢复；整体产品目标不完成。
