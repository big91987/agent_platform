# Agent Team Factory Experiment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在独立实验分支实现并实测可复用的最小软件工厂，重点验证原生 Agent 的真实并发、任务交接和代码汇合。

**Architecture:** Python 标准库实现 SQLite 任务库、原生 Codex CLI 调度、stdio MCP 任务交接与只读看板。Leader 提出实际任务；Worker 使用独立 worktree。任务提交和本地汇合由控制器执行，所有目标限定 agent-team 实验分支，不借用旧 Workflow 状态。

**Tech Stack:** Python 3.10+、SQLite、Git worktree、原生 Codex CLI、HTML。

**Spec:** `examples/agent-team-factory/README.md` 与本会话用户已授权的实验原型。

## Global Constraints

- 仅本次新增文件进入源码实验提交，保留原工作区已有未提交改动。
- source: `codex/agent-team-factory-experiment`; integration: `agent-team/experiment`; task prefix: `agent-team/task-`。
- 不合主线，不部署产品，不恢复原定时任务；运行材料必须 Git ignored。
- 使用现有原生登录，不读取或复制凭据；不使用跳过沙箱或审批的选项。
- 产品实现与验收由实际实验 Agent 执行；控制器只管理队列、环境、版本和证据。
- 同一任务一个有效领取；结果先保留，进程退出后才提交；失败/中断不自动重试。

## Review Focus

- 双控制器同时启动时，并发限制和任务领取仍由数据库原子保证。
- 不确定的原生执行结果保留，不把重启当作重新执行授权。
- 任务路径越界、符号链接、错误分支、脏基线与合并冲突不会污染其他任务或主线。
- 原生结果、提交、任务与运行时间对应；隔离探针不能充当真实 Agent 完成证据。
- 看板默认只绑定 loopback，转义输出，不暴露原生私有日志或凭据。

## Task 1: 持久任务、依赖、消息和原子领取

**Files:** `factory_store.py`, `test_store.py` in `examples/agent-team-factory/`.
**Interfaces:** `Store(root)`, `initialize(settings)`, `add_tasks(tasks)`, `claim()`, `finish(...)`, `submit(...)`, `message(...)`, `snapshot()`。
- [x] 写并观察失败测试：双连接并发领取唯一、满额不领取、依赖阻断、重复任务拒绝、结果及消息重开可读。
- [x] 实现 SQLite 事务，任务 ID 和依赖显式验证，结果不静默覆盖。
- [x] 运行本项测试并记录结果。

## Task 2: 独立工作区与受限汇合

**Files:** `git_workspace.py`, `test_workspace.py`.
**Interfaces:** `Workspace(repo, root, integration)`, `prepare(task_id, attempt)`, `commit(task, path)`, `integrate(task, commit)`。
- [x] 真实临时 Git 仓测试先红：分支/worktree 标记、路径范围、脏/错误分支拒绝、合并冲突保留。
- [x] 实现任务工作区准备和提交，拒绝主线目标与不安全路径。
- [x] 运行测试，核原始基线和无关文件保持。

## Task 3: 真实 Leader、Worker 和交接入口

**Files:** `native.py`, `mcp_server.py`, `factory.py`, `schemas/plan.json`.
**Interfaces:** CLI `init`, `plan`, `run`, `status`, `message`, `integrate`, `serve`; stdio MCP `handoff`, `send_message`, `get_task`。
- [x] 测试进程退出无 handoff、不一致重复 handoff、非法 MCP 请求、取消与超时保留。
- [x] 实现原生 CLI 事件留存、模型不擅换、Worker 控制器调度及 MCP 交接。
- [x] 在真实 Model Relay 版本上由 Leader 拆任务，启动至少两名真实 Worker，核会话与时间重叠。

## Task 4: 公共看板与实验说明

**Files:** `board.py`, `board.html`, README updates.
- [x] 提供 loopback 只读任务/消息/结果看板，页面展示实际执行状态与峰值。
- [x] 记录受支持安装、启动、停止、显式恢复与实验分支汇合命令。
- [x] 验证输出转义、私有日志隔离和实际浏览器显示。

## Task 5: 端到端并发实验与报告

**Files:** `docs/03-delivery/agent-team-factory-experiment-2026-11-01.md`; ignored runtime evidence.
- [x] 创建并核验两仓专用实验分支；测试仓创建去重后的 `【Harness验证】` 跟踪 Issue。
- [ ] 真实需求 → Leader → 并发 Worker → 结果交接 → 独立复核 → 实验分支汇合。
- [ ] 分级提高并发，记录实测峰值与失败，不以配置值宣称 20 并发已验。
- [ ] 由实验 Agent 执行 Model Relay 验证；控制器核版本、回执和保留证据。
- [ ] 记录任务完成量、墙钟、等待、返工、Token/资源、集成与质量局限。
- [x] 源分支完整检查与独立只读复审；只推送专用实验分支。

## Progress

- 2026-11-01：长期目标已创建；已在现有隔离 worktree 创建源码实验分支。原有工作区改动保留。原生 CLI 0.151.0 可用、ChatGPT 登录状态已确认（未读取凭据）。Model Relay 独立 clone 进行中。
- Ruling: 用户已明确授权实验原型和执行，本计划作为可续接记录，按原型例外直接推进，不另设重复批准步骤。
- 2026-11-01：维护源首个原型提交/推送 `2425710`；两仓远端实验分支及 Model Relay Issue74 已创建。G112 已按用户原有入口接通；独立 Linux 克隆、固定原生 CLI 与已有登录核验完成。源码归档逐摘要核对，经维护源安装器安装，不改旧服务。
- 2026-11-01：任务库、Git/MCP/原生调度/只读看板和版本化安装已实现；十九项隔离回归在本机与执行主机最终通过，独立评审发现的四项 Important 及 stdin_closed/重复清理问题已修正。完整实际状态见 `docs/03-delivery/agent-team-factory-experiment-2026-11-01.md`。
- 2026-11-01：真实 Leader job `agent-team-factory-leader-01` 已从受支持 CLI 启动，初始请求选择最多三个既有产品范围的小任务。尚未把原生读代码事件冒称拆分完成或真实 Worker 并发成功。
- 2026-11-01：Leader 60.708秒完成三项拆分；第一批真实 Worker 实际三并发。三项最终均因 CLI Never 拒绝 MCP 而无有效交接，failed 原记录保留；产品未提交/汇合。维护源已加入窄范围逐工具授权、正式同SID/worktree恢复、替代SID/旧PGID拒绝及静默重放防护，二十三项隔离回归最终通过；待精确版本部署后原任务接续。

- 2026-11-01续：第六轮原三个Worker已实际推进至浏览器完整旅程，原生PID确认存活；每个候选的2480个受控延迟样本均完成、失败0，仍非完整门禁/独立QA结论。后续4项独立QA任务由真实Leader规划并等待合入。另已提出8项互不重复的仓库工程静态审查，明确其不是8个产品功能或实操QA，用于下一阶段真实并发及工程质量评估。
- 2026-11-01续：新增100待办/20领取位置、40连接争用的隔离压力回归，实际按序且不重复排空，数据库领取峰值20、原生进程峰值0，不能作为20个真实Agent证据。另补正式run的SIGTERM/后代清理/中断状态保留，以及MCP身份额外字段、未注册工具与过期attempt不得改状态。Task1–4按实现、隔离回归及实际浏览器/原生接续证据更新；Task5的真实扩大并发、独立验收与最终汇合继续保留未完成。

## 受信控制端发布补充

实测112没有GitHub写凭据，普通push受控失败；不把控制端凭据复制给Worker。为本例补一个明确的交付路径：控制器在无在途任务时按精确HEAD导出仅实验分支的增量Git bundle；可信控制端按bundle SHA-256与分支/HEAD核对，在隔离bare仓导入并正常非强制push、读回远端SHA。GitHub模式复用控制端gh认证助手，凭据不进入bundle或执行机。每次发布保留独立回执，未知push结果靠下次先查远端收敛，不盲重推。

- [x] 真实Git回归：bundle保持提交SHA、仅目标实验分支、完整性/版本不匹配拒绝、远端分叉拒绝、重复发布先核远端。
- [x] 实现正式bundle导出与控制端发布CLI，说明复用既有认证及原生执行凭据隔离。

2026-11-01续：20项静态审查被错误挂在产品QA计划评审后，产生无关等待。新增正式管理员依赖修正入口：仅未领取的pending任务可修改、共享控制器锁、全图校验并留存原因及前后依赖；已执行任务冻结。通过此入口解耦静态审查与产品实操QA，产品测试仍受独立READY评审约束，不直接改任务库或把审查完成冒称放行。
- [ ] 版本化安装后在Model Relay实验分支真实发布并核对远端对象。

2026-11-01续：20个真实静态审查Worker实际峰值20，另有Leader时全部原生峰值21；首轮191.086秒、17/20真实handoff。3个exit0但无handoff被拒绝，正式同SID接续后2项完成、1项仍失败（维护源Issue7）。未将其算为20个功能或可靠性全过。原3候选全量verify绿色仍被独立评审发现Bearer合同错误，Restore设施又暴露最终rename失败删除已有目标的真实红断言。QA计划v3第二次独立评审仍BLOCKED，缺commit-only故障入口和可执行内容版本绑定，已安排实际Worker设施与v4计划/独立评审，产品实操不提前启动。首批精确实验HEAD aade8d2已通过正式bundle发布器发布并读回；后续成果仍须再汇合发布。

2026-11-01续：实际交付检查发现开发设施文档含私有路径，原Worker正式接续清理且保留原Native历史。维护源汇合统一改squash最终内容，公开版本使用integration_commit；独立Git回归先证明旧merge泄漏父历史，再补merge-tree预期树、提交后父链/分支/树校验及pending隔离，防止hook重新引入私有内容。完整41项本机回归及独立原复现验证通过，待正式版本升级及真实设施汇合复验。

2026-11-01续：固定CLI实际启用延迟MCP发现，成功原生会话使用exec/ALL_TOOLS，失败评审零工具调用。修正未直列即停的指引，并把本任务用户恢复输入在MCP首调前直接投递，消除get_task不可用时读不到建议的循环。独立评审发现user任务名可伪装系统sender，已禁止user/leader任务名并对既有身份冲突明确拒绝。44项本机回归及独立复现复核通过，真实20并发工具发现与原失败会话接续待部署后验证；不擦除旧失败。
