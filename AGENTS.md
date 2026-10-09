# Agent Platform

Build the smallest useful generic Agent platform. Go backend, same-machine native executors, platform-owned persistent conversations, API and browser continuity. Do not hardcode software-development stages or webpage output.

Use the product PRD and architecture as the shared contract. Native executors own reasoning, Skills and Hooks; the platform owns authorization, persisted input, scheduling, process lifecycle and presentation.

## 测试平台与 Harness 模板时的验证策略

当使用测试仓验证 Agent Platform 或 Harness 模板时，协调者以 Harness Builder 角色工作：让对应 Pipeline Agent 执行产品需求、实现、用户旅程、供应商调用及验收，用真实执行检验流水线、工具、权限、交接、去重、停止恢复、失败处理和交付效果。此规则限定平台测试的方法，不限定协调者在其他任务中的职责；其他工作按用户授权和任务目标执行。

在这种平台测试中，协调者配置并启动受支持的流水线、提供用户已授权的目标和私有凭据引用、检查执行及交付证据；不要代写产品测试、操作业务页面完成用例或代 QA 判产品通过，以免掩盖流水线能力缺口。产品失败交产品 Pipeline；工具或执行能力失败修唯一 Harness 维护源，再通过标准安装升级及原流水线复验。构建 Harness 的隔离回归与运行探针用于证明平台能力，不能冒充产品验收；一次产品测试通过也不能单独证明模板的可靠性和可复用性。

关联 Issue 必须能反映真实卡点，不能只在平台会话中等待。执行审批由平台基于实际审批状态及时通知，附当前节点、处理入口及审批后的结果，不等 Agent 回合结束；其他执行失败、缺输入或外部阻塞由正式原生消息/受信状态回写，写清影响、已尝试处理与下一步。不得把排队、审批受理或恢复受理当成测试完成，也不得公开审批命令、私有路径、凭据或敏感理由。相同卡点去重；已处理而尚未发送的待审批通知跳过；发送结果未知先核对外部回执，不盲重发。

产品测试结束后，协调者依据 Pipeline 报告及已复核缺陷创建或更新 `【产品修复】` Issue，附同版测试报告、复现证据及相关脱敏产物，通过受支持的研发入口直接交给 development。先核已有任务并去重；没有产品缺陷不造修复任务，未执行/凭据问题不冒称产品故障，Harness/测试设施问题分开处理。分支准备、材料核验和 Issue 关联仍须完成；明确问题和原验收依据齐全时跳过需求/设计，后续保留固定测试、独立 QA 和交付，不能用改运行状态模拟阶段交接。

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

## 测试仓 Issue 与 PR 写作规范

本项目 Agent、协调者及标准测试流程在任何测试仓创建 Issue 或 PR 时，标题必须使用 `【类别】具体目标`。这项要求适用于后续接入的所有测试仓，不只适用于 Model Relay。

| 类别 | 用途 | 标题示例 |
|---|---|---|
| `【产品功能】` | 实现测试仓产品的用户能力 | `【产品功能】支持租户成员邀请与模型授权` |
| `【产品修复】` | 修复测试仓产品自身的行为或安装问题 | `【产品修复】保存模型后等待当前租户列表刷新完成` |
| `【产品测试】` | 由专用测试 Pipeline 验证产品的真实用户旅程及外部集成 | `【产品测试】验证真实供应商下用户授权、Key配额与调用诊断` |
| `【Harness验证】` | 验证 Agent Platform 的编排、工具、权限、恢复或交付流程 | `【Harness验证】验证用户澄清后交接与 QA 返工接续` |
| `【Harness修复】` | 修复平台、通用模板或可复用工程工具，包括在测试仓应用标准升级 | `【Harness修复】升级部署控制器以保留每次验证的独立日志` |
| `【交付维护】` | 单独维护发布、交付文档或任务状态，不新增产品能力 | `【交付维护】同步已部署版本及被替代任务的状态` |

- 类别按实际目标和能力归属选择，不按所在仓库选择。借用产品仓验证 Harness 仍属 `【Harness验证】`；产品自身的依赖准备缺陷属 `【产品修复】`。功能随附的必要测试无需另开 Harness 验证任务。
- 类别后的文字必须说明要实现、修复或验证的具体行为及结果。不得仅写“全新 Pipeline”“新版编排回归”“完成研发交付”“优化体验”等无法判断工作内容的标题；端口、模型、Run ID 和协议版本放正文，除非它们本身就是问题对象。
- 产品研发、Harness 验证和 Harness 修复分别立项；正文写清工作归属、用户结果、验收依据及关联 Issue/PR。Harness 通用修复仍先进入唯一维护源，再通过标准安装升级应用到测试仓，不因分类而允许运行副本补丁。
- 创建前检查已有任务，继续同一目标时更新原任务。因取消或新环境必须重新起任务时，在正文明确替代关系和旧任务状态，不把重跑描述成新产品功能。
- PR 标题也必须带类别，并描述最终实际改动；与关联 Issue 的实际范围一致。范围变化时同步标题和说明，不沿用过时目标或把测试完成冒称产品功能完成。发布前复核标题、正文、关联关系和真实完成状态。

### 正文先说清要做的事

Issue 和 PR 都面向没有看过当前聊天的读者。前两段必须用直接的业务语言讲清对象、问题和结果；不能以端口、提交摘要、Run ID、模板版本、授权记录或大段历史开篇。执行元数据和完整证据链接放在相关说明之后，不让读者从日志中猜目标。

Issue 写的是本次要完成的工作，至少讲清：

- **为什么做**：谁在什么操作或场景下遇到什么问题，当前行为及具体影响；新功能说明当前缺少的能力。猜测与已证实事实分开。
- **要做什么**：具体改变哪些行为，完成后用户如何使用或观察结果。产品任务用用户能力表达；Harness 验证说明被测的平台能力、测试仓承担的作用，以及是否修改产品，不用“把流程跑通”代替具体验证场景。
- **范围与边界**：本次包含哪些内容，哪些相关工作另属其他任务。产品功能、产品缺陷和平台问题不能混成一个不分归属的目标；同一目标的必要回归可以随任务交付。
- **怎样算完成**：列可观察、可复验的验收条件，包括适用的正常路径及失败恢复。要部署的任务写明目标环境与实际验收入口；流程执行结束、生成 PR 或测试数量不能单独代表业务完成。
- **与已有任务的关系**：引用真实关联项，明确延续、替代或新目标。背景只保留解释当前任务所需的内容，不复制整段旧任务和协调过程。

PR 写的是最终实际改动，至少讲清：

- 修复什么问题或增加什么能力，具体触发场景下的改动前后行为；随后说明采用的必要实现和范围。
- 实际完成了什么验证、结果如何、证据对应哪个版本；自报、局部测试、完整测试、独立 QA、合并和部署分别表述。未执行、受限、失败及遗留问题如实列明，不把计划写成结果。
- 关联哪些 Issue，本 PR 解决哪些部分、哪些仍未完成。若最终范围变化，重写说明；不能直接复制 Issue 的愿望、执行指令或旧版本验收结果作为 PR 交付结论。

按复杂度组织正文，简单任务可用两段说明加验证，不机械堆砌章节。必要权限和工程约束放在明确位置，不用重复的“已授权”“不允许”等协调者指令淹没工作目标。发布前做独立阅读检查：只读标题和开头就能回答“为什么做、具体做什么、完成后有什么变化”；继续读正文能知道“如何验收、与旧任务有什么关系”。回答不出来就先改写，再创建或更新 Issue/PR。


## 自动化测试的执行审批

产品测试必须先制定可执行计划，再由独立节点依据实际 PRD/AC/设计和业务规则评审后进入执行。核心能力逐项映射正常、非零阈值与边界、隔离、并发、失败恢复和证据；次数配额、RPM、TPM、并发与金额预算不能互相替代。缺文档、规则或必跑用例不判计划 READY，执行阶段实质改变范围或断言须重新评审。规划、计划评审、实操及结果复核分别报告，不能把文档评审或用例数量当产品通过。通用模板提供职责和交接门槛，具体产品断言仍由 Pipeline 依据本任务文档制定。

测试报告交付应让用户可查看：Pipeline产生完整正文、可视化结果看板、真实证据及白名单摘要，通过已授权GitHub入口发布并核对远端回执。不要只给本机路径；区分HTML下载附件和已部署在线页面，保持仓库原可见范围。展示检查、文件生成、发布成功和产品验收分别报告，不因看板做出来而改变No-Go或未验结果。

已授权的自动化测试可显式选择 Codex 原生 `auto_review`，由原生审核器判断具体权限申请；默认人工审批及任务沙箱保持。模板安装/升级须记录该选择并保留省略参数时的旧设置，在途会话只通过管理员正式权限入口于轮次之间应用，不改冻结图或原生历史。自动审核拒绝、超时或中断应保留原始证据并在关联 Issue 说明具体影响和后续动作，不自动接受、换命令绕行或循环重试。隔离 Harness 回归与产品 Pipeline 验收分开记录。

执行权限策略应作为平台可见配置交付：独立 Agent 与 Pipeline 节点共用字段和校验，已有会话通过界面使用正式权限 API；明确配置作用于新任务还是当前会话。任务级 Issue/API 选择由受信入口和平台核验、保存并执行，不依赖 Agent 阅读正文自行授予权限，也不只靠协调者临时改某个 Run。未接通或未真实验证的入口明确记录为待完成。
