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
- [ ] 写并观察失败测试：双连接并发领取唯一、满额不领取、依赖阻断、重复任务拒绝、结果及消息重开可读。
- [ ] 实现 SQLite 事务，任务 ID 和依赖显式验证，结果不静默覆盖。
- [ ] 运行本项测试并记录结果。

## Task 2: 独立工作区与受限汇合

**Files:** `git_workspace.py`, `test_workspace.py`.
**Interfaces:** `Workspace(repo, root, integration)`, `prepare(task_id, attempt)`, `commit(task, path)`, `integrate(task, commit)`。
- [ ] 真实临时 Git 仓测试先红：分支/worktree 标记、路径范围、脏/错误分支拒绝、合并冲突保留。
- [ ] 实现任务工作区准备和提交，拒绝主线目标与不安全路径。
- [ ] 运行测试，核原始基线和无关文件保持。

## Task 3: 真实 Leader、Worker 和交接入口

**Files:** `native.py`, `mcp_server.py`, `factory.py`, `schemas/plan.json`.
**Interfaces:** CLI `init`, `plan`, `run`, `status`, `message`, `integrate`, `serve`; stdio MCP `handoff`, `send_message`, `get_task`。
- [ ] 测试进程退出无 handoff、不一致重复 handoff、非法 MCP 请求、取消与超时保留。
- [ ] 实现原生 CLI 事件留存、模型不擅换、Worker 控制器调度及 MCP 交接。
- [ ] 在真实 Model Relay 版本上由 Leader 拆任务，启动至少两名真实 Worker，核会话与时间重叠。

## Task 4: 公共看板与实验说明

**Files:** `board.py`, `board.html`, README updates.
- [ ] 提供 loopback 只读任务/消息/结果看板，页面展示实际执行状态与峰值。
- [ ] 记录受支持安装、启动、停止、显式恢复与实验分支汇合命令。
- [ ] 验证输出转义、私有日志隔离和实际浏览器显示。

## Task 5: 端到端并发实验与报告

**Files:** `docs/03-delivery/agent-team-factory-experiment-2026-11-01.md`; ignored runtime evidence.
- [ ] 创建并核验两仓专用实验分支；测试仓创建去重后的 `【Harness验证】` 跟踪 Issue。
- [ ] 真实需求 → Leader → 并发 Worker → 结果交接 → 独立复核 → 实验分支汇合。
- [ ] 分级提高并发，记录实测峰值与失败，不以配置值宣称 20 并发已验。
- [ ] 由实验 Agent 执行 Model Relay 验证；控制器核版本、回执和保留证据。
- [ ] 记录任务完成量、墙钟、等待、返工、Token/资源、集成与质量局限。
- [ ] 源分支完整检查与独立只读复审；只推送专用实验分支。

## Progress

- 2026-11-01：长期目标已创建；已在现有隔离 worktree 创建源码实验分支。原有工作区改动保留。原生 CLI 0.151.0 可用、ChatGPT 登录状态已确认（未读取凭据）。Model Relay 独立 clone 进行中。
- Ruling: 用户已明确授权实验原型和执行，本计划作为可续接记录，按原型例外直接推进，不另设重复批准步骤。
- 2026-11-01：维护源首个原型提交/推送 `2425710`；两仓远端实验分支及 Model Relay Issue74 已创建。G112 已按用户原有入口接通；独立 Linux 克隆、固定原生 CLI 与已有登录核验完成。源码归档逐摘要核对，经维护源安装器安装，不改旧服务。
- 2026-11-01：任务库、Git/MCP/原生调度/只读看板和版本化安装已实现；十九项隔离回归在本机与执行主机最终通过，独立评审发现的四项 Important 及 stdin_closed/重复清理问题已修正。完整实际状态见 `docs/03-delivery/agent-team-factory-experiment-2026-11-01.md`。
- 2026-11-01：真实 Leader job `agent-team-factory-leader-01` 已从受支持 CLI 启动，初始请求选择最多三个既有产品范围的小任务。尚未把原生读代码事件冒称拆分完成或真实 Worker 并发成功。
- 2026-11-01：Leader 60.708秒完成三项拆分；第一批真实 Worker 实际三并发。三项最终均因 CLI Never 拒绝 MCP 而无有效交接，failed 原记录保留；产品未提交/汇合。维护源已加入窄范围逐工具授权、正式同SID/worktree恢复、替代SID/旧PGID拒绝及静默重放防护，二十三项隔离回归最终通过；待精确版本部署后原任务接续。
