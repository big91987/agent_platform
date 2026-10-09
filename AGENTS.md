# Agent Platform

Build the smallest useful generic Agent platform. Go backend, same-machine native executors, platform-owned persistent conversations, API and browser continuity. Do not hardcode software-development stages or webpage output.

Use the product PRD and architecture as the shared contract. Native executors own reasoning, Skills and Hooks; the platform owns authorization, persisted input, scheduling, process lifecycle and presentation.

Keep real behavior verifiable. Repair supported paths rather than patching individual conversations. Never silently replace a missing native session or replay uncertain side effects. Secrets, native histories, generated workspaces and screenshots from local experiments stay in ignored storage.

Use gofmt and go vet. Keep tests small and focused on meaningful behavior: isolation, queue ordering, cancellation, restart, deduplication, authorization and real executor continuity. Do not pile up assertions about constants, prose or implementation structure.

## Native output presentation

Display native Agent messages without rewriting, summarizing, or interpreting business fields. Preserve tool calls and results as execution events; do not synthesize narrative progress from them. Platform indicators describe only execution lifecycle, connection, queue and pending tool approval. Externally registered handoff tools use the same presentation and approval path as other tools.

## Product interface style

Organize pages around the user's task, choices, results and next actions. Keep compatibility handling, protocol versions, migration history and implementation commentary in code or maintenance documentation unless the user must act on them.

Show help or warnings only when they explain a current choice, a meaningful consequence or a real problem the user can resolve. Describe settings by their observable behavior in plain language; do not explain tool registration or server enforcement in ordinary field help. Necessary technical configuration can retain precise technical names.

Compatibility must not add routine configuration work. When migration or a conflict requires user action, explain the concrete effect and provide the relevant action at that point. Do not add reassurance about normal behavior or narrate development decisions.

Before delivering interface changes, remove explanations, duplicate status, internal identifiers and controls that do not help the user decide or act. These rules govern platform-authored interface content; preserve native Agent output under the rules above.

## Agent configuration and task input

Treat an Agent as a collaborator with a name, executor, model, role, Skills, tools and execution permissions. Reuse the same execution configuration form and validation in standalone Agents and workflow nodes. Each editable setting must survive save/reload and affect the supported execution path; show both executor and model, and let users edit displayed names.

Concrete work arrives through User Input, corrections and handoff. Do not add a separate Session Prompt or node work-description field for new workflows. Native workspace AGENTS.md supplies project rules; do not copy it into platform prompts. One Run shares its startup workspace. Add shared workflow variables only for a demonstrated need beyond this workspace, not as a general configuration framework.

Keep handoff/completion/wait controls specific to orchestration. Preserve frozen legacy runs during upgrades, and reject incompatible or externally edited configuration rather than silently dropping user instructions.

## 测试仓 Issue 与 PR 标题规范

本项目 Agent、协调者及标准测试流程在任何测试仓创建 Issue 或 PR 时，标题必须使用 `【类别】具体目标`。这项要求适用于后续接入的所有测试仓，不只适用于 Model Relay。

| 类别 | 用途 | 标题示例 |
|---|---|---|
| `【产品功能】` | 实现测试仓产品的用户能力 | `【产品功能】支持租户成员邀请与模型授权` |
| `【产品修复】` | 修复测试仓产品自身的行为或安装问题 | `【产品修复】保存模型后等待当前租户列表刷新完成` |
| `【Harness验证】` | 验证 Agent Platform 的编排、工具、权限、恢复或交付流程 | `【Harness验证】验证用户澄清后交接与 QA 返工接续` |
| `【Harness修复】` | 修复平台、通用模板或可复用工程工具，包括在测试仓应用标准升级 | `【Harness修复】升级部署控制器以保留每次验证的独立日志` |
| `【交付维护】` | 单独维护发布、交付文档或任务状态，不新增产品能力 | `【交付维护】同步已部署版本及被替代任务的状态` |

- 类别按实际目标和能力归属选择，不按所在仓库选择。借用产品仓验证 Harness 仍属 `【Harness验证】`；产品自身的依赖准备缺陷属 `【产品修复】`。功能随附的必要测试无需另开 Harness 验证任务。
- 类别后的文字必须说明要实现、修复或验证的具体行为及结果。不得仅写“全新 Pipeline”“新版编排回归”“完成研发交付”“优化体验”等无法判断工作内容的标题；端口、模型、Run ID 和协议版本放正文，除非它们本身就是问题对象。
- 产品研发、Harness 验证和 Harness 修复分别立项；正文写清工作归属、用户结果、验收依据及关联 Issue/PR。Harness 通用修复仍先进入唯一维护源，再通过标准安装升级应用到测试仓，不因分类而允许运行副本补丁。
- 创建前检查已有任务，继续同一目标时更新原任务。因取消或新环境必须重新起任务时，在正文明确替代关系和旧任务状态，不把重跑描述成新产品功能。
- PR 标题也必须带类别，并描述最终实际改动；与关联 Issue 的实际范围一致。范围变化时同步标题和说明，不沿用过时目标或把测试完成冒称产品功能完成。发布前复核标题、正文、关联关系和真实完成状态。
