# 平台内 Workflow 交付计划

日期：2026-10-05。分支 `codex/platform-workflows`，基线 `3f3b2c8`，独立 worktree。负责人：平台实施者；用户负责最终体验验收和源仓库合并。

权威输入：[需求](../01-product/workflows.md)、[G1](../01-product/workflows-review.md)、[架构与 G2](../02-architecture/workflows/platform-native.md)。本文件维护任务状态；验收证据写入 [验证记录](workflows-verification.md)。

当前状态（2026-10-09 07:10）：Issue30/Run2574d6be275f9eb630f8738837507e1e已12步completed，report9→publish10 exit0→pr11 PR31→done12真实完成。PR31/head8c8b155c007686311706ee1f8cec8262a1258bb9经维护者150件Git对象与tests7/QA8源码0384f115…逐字节及26QA归档SHA/bytes核证、树干净、GitGuardian成功；15任务分支证据链接远端非空。已按授权ready并精确SHA合并main3f9cee1615e87e002a2c2300f2d29778072ec3f5。正式push prepare37858024406 in_progress、Owner dispatch37858041523 pending（preview/main3f9cee），不要重复合并/触发或复活完成Run。controller2d456…仍标准安装，新600私有verify-3f9cee…-jt63zgxe.log由真实prepare创建但尚未完整结束；目前5545仍9bf6ecaf/schema3健康，5546旧9851177/schema2保持，未新部署。下一步核实际Actions/Deployment/新health/page；若新红保留首回执最小归因不盲试。R30完成契约候选已测/QA通过，旧R28-04 Open/P2与动态独立QA/供应商/登录未测保持；成功部署后推进长期阶段2其他P3，不停留完成Run。

阶段快照（2026-10-07）：用户视角的Harness典型路径验收 **In Progress／No-Go**。测试产品是交付质量证据，不替代平台用户体验及使用手册。路线1/2已正式发布；旧Issue20 Run保持stopped/seq13。当前用已冻结design-v0.2.0原型/PRD包经真实Issue21、Actions和唯一Run2f589073fb909da91a3e6f76a8ba5430验证中途接续，材料准备/接单/需求承接已发生，设计执行中。用户授权原型与普通测试选择自主判断，执行模型Codex gpt-6.1-sol。测试仓允许合并/正式部署，源PR5不合main；完整实现/QA/发布与其余路径尚未验收。

## 里程碑与任务

| 里程碑／任务 | 交付结果、范围与依赖 | AC 与退出证据 | 状态 |
|---|---|---|---|
| M1 / T1 可编辑可保存的流程 | workflow 存储、图校验、权限和 HTTP；网页拖拽节点／连线／编辑／导入导出。修改 internal/platform/workflow*.go 与独立 web/workflows.js/css；复用身份。依赖需求／架构基线 | WF-01、09；保存重开、并发更新拒绝、循环和非法图测试、网页操作 | In Review |
| M2 / T2 同一运行中的 Agent 接力 | Run 和节点执行事务、冻结定义、显式完成工具、持久会话、人工确认与循环。依赖 T1；不得用 final 文本判断交接 | WF-02～05、10、12；真实 Codex 澄清／交接／回退，刷新历史 | In Review |
| M3 / T3 外部仓库闭环 | Connector 配置／检查／执行；独立私有测试仓库与可导入研发模板；网页发起真实工作并回写 Issue／PR。依赖 T2 | WF-06；真实 GitHub 链接、动作失败与恢复证据；普通文本任务无需懂框架 | In Progress |
| M4 / T4 可恢复的完整首版 | 停止与继续、重启核验、重复／迟到回执、升级脚本与公开说明；复验前三里程碑。依赖 T1～T3 | WF-07～11；故障注入、正式恢复入口、原平台回归、所有 AC 证据 | In Progress |

关键路径：T1 → T2 → T3 → T4。每项按测试先行实现，完成后更新契约与证据；涉及新失败先补复现测试。允许任务内迭代，不以横向组件完成代替里程碑。

## 验证与交付纪律

- `go test ./...`、`go test -race ./internal/platform`、`go vet ./...`；前端沿用已有 Node 测试与语法检查入口，并增加针对图编辑／运行变化的行为验证。
- 新增数据目录从零启动；原数据库副本执行新增表迁移并检查旧会话可查，禁止对正式数据做实验性变更。
- 浏览器实际操作覆盖配置、拖动、连线、保存、发起、进入 Agent 会话、批准／回退、停止、刷新；所有成功声明关联真实页面与运行。
- 独立测试仓库不得复用 reading_list。在新仓库创建可辨认测试任务，验证文件修改、测试失败、修复、交接和 PR 回执。模拟器仅用于隔离测试，不能充当实测。
- 对未知外部效果只能经正式核验／恢复入口处理；不得伪造结果、手改状态或注入临时权限。
- 修复维护源为本仓库，模板和安装说明同时交付。源 PR 留给用户合并。原平台未提交文件、正在运行的旧任务均保留。

## 完成标准

任务 Done 要求对应 AC、回归与真实切片证据齐全。最终交付提供可打开的隔离实例、测试仓库、源 PR、安装／升级说明，以及通过／失败／未测清单。当前 T1 图定义、HTTP 权限和网页编辑器已实现；真实页面完成拖放、端口连线、保存、刷新、JSON 导入导出往返。文件下载／上传于2026-10-06补验：实际下载JSON→文件选择载入→保存停用副本，节点/边/Hook与原定义逐项一致、授权不复制、原工作流未变；详见验证记录。T2 已完成两条真实网页与 Codex 旅程：人工拒绝回环，以及人工指定回退／澄清／执行中停止／重启后原会话恢复。T3 已通过真实一句话需求到草稿 PR 的正常链路，QA 真实缺陷已自动退回研发并完成独立复验；T4 已通过真实 HTTP 权限与幂等、命令中断与强杀重启恢复，仍需 GitHub 未知响应恢复及最终发布验收。

## 智能体编排演进实施计划

目标：复刻方案二并在同一运行内提供更清晰的主动交接、固定流转和可配置通知。按用户明确授权在当前分支原地实现，不等待逐项审批；不合 main，不创建新 PR。

- [x] E1 路由契约：修改 workflows.go、workflow_runs.go、workflow_http.go、workflow_engine.go；边 mode 与说明、通用 handoff 入口、固定完成线。先验证旧图兼容、非法自主目标、普通回复不推进、双重交接和分支选择，再实现。
- [x] E2 低代码界面：修改 workflow-model.js、workflows.js/css、运行页；智能体编排名、虚实线、连线属性与焦点视图。模型行为测试及真实页面保存/重开/运行。
- [x] E3 通知 Hook：复用 Connector 注册能力，持久事件及投递记录、字段映射、独立失败恢复、运行页可查；模板与正式安装路径同步。测试隔离/重复/未知效果；专用 GitHub Issue 实测回写。
- [x] E4 模板和真实用户旅程：收敛研发主路径、SOP 自主澄清和回退，验证新实例安装及原实例升级。先修复已发现的真实 GitHub 丢响应恢复问题，再测交接、用户续聊及回退；不以 Mock 代替。
- [x] E5 部署与交付：完整回归，备份并通过正式启动入口更新 8792，8788 保持；记录版本、运行链接、实际截图、通过/失败/未测。

重点审查：旧图省略字段；混合自主和固定出口；交接期间新用户输入；通知重复/外部结果未知；模型权限不因 Hook 扩大。现有 AGENTS 未提交改动及未完成 GitHub 实测代码保留。

本轮 E1–E5 切片的实际证据与限制见 [验证记录](workflows-verification.md#e1e5-智能体编排主动交接与通知2026-10-05)。此勾选表示本次路由／通知迭代已交付到开发实例，不代替全部平台最终发布验收。


## 典型旅程补齐

- [x] E6 对齐 WF-13／14：研发模板支持明确 bug、脚本／配置修复的真实短路径，兼容必要工作区准备及任务关联；同时验证显式起点与 Agent 自主选择。
- [x] E7 对齐 WF-14／15：覆盖信息不足不能跳过、结构变更需要设计、交接后用户纠正目标与迟到结果隔离；将每项实际结果回填验证矩阵。

E6／E7 的短路径、澄清和交接后纠正场景已实测，结果见验证记录 E6／E7 节。此勾选不外推为此前列出的全部平台能力或所有模型输入均已验收。

## 开发实例升级与完整旅程验收

用户授权：当前 Model Relay M1 运行结束后，应用共享浏览器与独立 Runs 页面改动，持续完成基本流程及特殊跳转验收，达到可查看效果后通知。沿用同一产品 Run；源分支不合 main。验收基线锁定实际部署提交，历史证据保留但不能替代受影响路径的新版本复验。

- [x] 当前 M1 正常交付：真实项目测试、独立 QA、GitHub 进展及草稿 PR，核对产物与测试版本。
- [x] 安全升级：确认无在途执行、备份、升级二进制、用原 manifest 升级两个项目引用的共享工具，核验重复安装及旧运行恢复边界。
- [x] 入口与呈现：编排卡片／详情 → 本工作流 Runs → 单次运行 → Agent 会话；工具、流式输出及历史顺序正确。
- [x] WF-13／14：明确 bug／配置修复直接开发、显式起点、信息不足先澄清、结构变化进入设计；保留必要准备和真实测试。
- [x] WF-04／05／15：QA 返工、人工回退、交接后纠正目标，旧执行隔离且产物与原因可追溯。
- [x] WF-07～11：停止／恢复／重启、重复提交、迟到交接、外部失败恢复、权限和版本冻结的受影响回归。
- [x] 在原发现层关闭缺陷；既有验证记录逐项登记 Pass／Fail／Blocked／Not Run，完成基本流程与特殊跳转后提供效果入口。

共享工具、独立 Runs 和活跃运行保护已在开发分支实现；源回归、隔离页面验证、正式新装／升级／重入、两个项目原生 Agent 调用共享工具均已通过。完整历史分页补验通过，具体版本与证据见验证记录。8792 已升级并经原 manifest 重入，Model Relay M1、独立 QA 与草稿 PR 完成；每工作流 Runs、共享工具及历史授权导航已从页面复验。专项故事使用既有真实 Run 的冻结记录、升级后页面复查和当前图出口核对，不重复创建 GitHub Issue／产品 Run；各项的 UI／API／外部证据与限制见验证记录。真实供应商联调没有凭据及响应，仍为 Not Run；旧验收 Run 有一条按计划保持停止，历史兼容工具保留供恢复。

## 方案二平替纠偏：完成复验

此前把“至草稿 PR 的编排旅程”误报为完整平台验收，现已补齐选定测试仓库的合并后自动准备、Owner 手动部署、固定地址、更新与失败恢复。用户明确授权验收方自行合并测试仓 PR；源仓库 PR 仍保留评审、未合入 main。此前将本轮切片误记为平替 **Go**；GitHub Issue 接单未覆盖，当前整体结论改为 **No-Go**，逐项 UI／API／外部证据与限制见 [验证记录](workflows-verification.md#完整旅程完成复验2026-10-05)。

- [x] 对照旧方案锁定两类真实任务的用户入口和最终结果；保留新功能、明确 bug、配置修复、必要澄清和返工的已验证证据。
- [x] 从同一交付记录进入 PR，授权验收方以精确 head 合并测试仓 PR，并在原 Run 页追溯其合并 SHA；正式产品仍由 Owner 决定合并。
- [x] 安装测试项目正式部署 Pipeline，验证首次部署、更新、两类真实 Actions 失败修复、受控启动失败恢复、LaunchAgent 重启和稳定效果地址。
- [x] 从用户 Run 页连续核对部署状态、版本和效果 URL，并与 GitHub Deployment、实际 `/healthz` 和页面一致。
- [x] 通用修复落在平台维护源及原安装升级路径，测试仓 Workflow/说明经 PR 交付；真实供应商联调仍标 Not Run。

## GitHub Issue 接单补齐（WF-17，In Progress）

用户明确要求对齐方案二的研发入口，已授权普通实现选择自主推进。旧方案 `examples/github/pipeline.yml` 使用 `issues.opened` 触发，经受信自托管 Runner 进入平台；当前编排模板仅有平台向 GitHub 的出站动作，缺少入站接单与原 Issue 关联。

实施方向：保留平台网页入口，复用任务启动和反馈的通用能力；GitHub 作为并列入口，通过现有 Runner 传递事件，正式持久保存原 Issue 与 Run 关联并推进图；Runner 不重新承担逐阶段调度。传输、关联和回复处理进入可复用维护源；项目配置通过标准 manifest 安装升级。用户任务文本不进入 shell，外部消息必须核验身份，事件重试使用稳定键。

- [x] 对照旧入口，修正产品契约和验收结论；保留已有真实研发及部署证据的原范围。
- [ ] 入站接单和原 Issue 关联：身份、仓库边界、唯一 Run、工作区准备、未知响应与重试。
- [ ] 同一 Issue 反馈接续：必要澄清、纠正、交接竞争、回写去环、停止和完成状态的明确处理。
- [ ] 标准安装升级：共享来源、项目绑定、原 manifest 重入、历史 Run 兼容、复用已安装 Runner。
- [ ] 从 GitHub 正式新建需求与 bug Issue 连续验证至研发、测试、QA 和交付；补充评论接续、事件重投和失败恢复证据。

未通过上述真实旅程前不恢复平替 Go；源仓库不合 main，不改原实例，不重启已暂停的旧定时任务。

多入口设计约束：企业微信、钉钉是用户明确提出的后续渠道，本轮不实现。入站身份、外部事件去重、任务关联和回传放在渠道接入边界；通用 Run 启动与反馈接口不强制依赖 GitHub Issue，也不根据相同文本自动合并不同渠道的任务。GitHub 入口补齐须同时回归网页发起，不能用替换入口的方式交付。

### WF-17 实施接口与验证步骤

采用 inline 执行，不等待逐项确认。基线为现有开发分支；不创建第二维护源。

1. 通用 API/SDK：`WorkflowStart.parameters` 保存有界字符串参数；按调用者的 `request_id` 查询原 Run；`POST /api/workflow-runs/{id}/messages` 在同一提交事务内选择当前 Agent、校验所有权并去重，返回持久消息回执。SDK 提供 `start_workflow`、`workflow_run`、`workflow_runs`、`workflow_by_request`、`workflow_message`、`workflow_command`、`wait_workflow`。先测参数冻结/旧请求兼容、跨用户隔离、重复消息及交接后的重试，再实现。
2. GitHub 集成：`github.issue` Connector 用配置的参数名读取已有 Issue；缺少该参数时保留创建行为。读回真实 GitHub Issue 回执，评论/PR 仅关联同仓回执；拒绝 PR 冒充 Issue。事件适配器由受信 Runner 使用 SDK 调用，按原 Issue ID 唯一接单、独立克隆、保存首次事件及回执、评论去重并拒绝回写循环。先验证恶意事件/未授权作者、请求重投、克隆失败、评论交接竞争，再实现。
3. 标准安装：原平台 manifest 升级 Connector 和模板；版本摘要安装器分发最小 Actions 工作流，只转交 `issues.opened` / `issue_comment.created` / 显式恢复事件。工作流不 checkout 或执行 Issue 提供的脚本，不调度研发阶段。SDK 从维护源正式安装。测试新装、升级、漂移拒绝及重复安装。
4. 真实验证：完整源回归及独立代码评审后，核对无在途任务、备份并升级开发实例。测试仓经正式安装器与已授权 PR 合并安装事件入口；从 GitHub 新建 bug 与需求 Issue，验证原 Issue↔Run、澄清评论、短路径、真实测试/QA/PR；重跑 Actions 核对不重复任务，记录失效/停止时的明确反馈。网页旧入口同步回归。

重点边界：原请求的返回值不能因图升级或节点推进而改变；未知网络结果不能盲目重发不同输入；未经授权的仓库/作者不能触发宿主执行；参数与事件文本只能作为数据；模板升级不能破坏历史 Run。每项测试先观察预期失败再实现，单元与隔离 HTTP 通过不代替真实 GitHub/原生执行器证据。

### 实施记录

- API/SDK：参数持久化、按调用者事件键找回 Run、消息原子路由、交接后重复事件回执测试通过。SDK 0.2.0 完成 Workflow 薄封装，保留原单 Agent 接口。
- GitHub：原 Issue 读取回执、PR 冒充拒绝、无参数创建兼容、Owner-only 事件与评论防回环、未知接单响应和评论编辑后重试已有隔离证据。不是实际外部端到端通过。
- 首轮 `scripts/verify.sh` 全部通过；后续新增边界测试和安装继承调整待最后全仓复验。
- 独立只读评审发现安装器两文件写入中断后不能重入（P2）。先复现失败，再修为原子文件替换和精确可信源摘要恢复；首次安装与升级中断测试通过，不覆盖用户自定义漂移。显式 token_file 被继承值覆盖会使凭据轮换不生效，按实际影响修正为仅补未指定值。
- 评审未执行真实 GitHub/Runner/Agent 旅程及远端写入响应未知，维持待验；组织其他作者仍按明确 Owner-only 范围排除。

- 真实 GitHub 入站联调揭示三项此前隔离测试未覆盖的问题：Runner clone 和后续 fetch 的网络配置不一致；安装命令缺少环境变量时会清空原配置；Hook 回写使用另一种标记而被误当评论。修复已落共享适配器、仓库操作工具、安装器与阶段指令，分别保留失败回执、红灯回归和正式重试。原 Run 不另建，自动输出隔离与原 Issue 澄清继续实测。
- 补充独立评审发现代理变量被无条件登记为必需引用，会破坏无代理的新部署；已改为只登记明确配置的引用并在升级时继承原值。无代理、仅 HTTPS_PROXY、原映射继承与显式覆盖回归通过。没有要求部署方补造空环境变量。

- WF-17 本轮研发入口验收完成：真实 Issue #8/#9 分别自动交付草稿 PR #10/#11。已验明确 bug 短路径、必要澄清与原评论接续、QA 回退补证、完整项目测试和双视口浏览器、停止／恢复／平台重启、完成态去重及迟到输入拒收。标题缺陷亦已修复共享 Connector，并由第二个 PR 的真实创建复验。精确版本、回执和限制见 workflows-verification.md 的 WF-17 最终验收；本轮不扩展部署／code review，源 PR 保持 Draft 未合并。


## 文档契约与 Trellis 修正（2026-10-06，完成）

用户确认：编排模板只约束文档位置；内容、必需产物与裁剪遵循所挂载 Skill。PR #11 当前冲突允许临时人工解决，不能记为 Pipeline 自动同步通过。正式修复后用新 Issue 真实验证。

- 旧任务：保留 Issue #8/#9 两项产品变更与文档证据，完整项目验证后更新原 PR #11。
- 维护源：任务文档按 Run 隔离；Skill 内部的文档结构保留，交接引用实际路径，发布不再强制 QA 文件名。冻结的旧安装继续兼容。
- 研发 Agent：在交付管理 Skill 之外挂载 Trellis before-dev/check/spec-bootstrap/update-spec；核验项目初始化、真实规范读取、实施和检查，不能只凭 Skill 挂载宣称执行成功。
- 正式升级：原 manifest 更新，保留历史 Run 和在途安全规则，不改主实例。
- 验收：新 Issue 从 GitHub 接单，至少两项任务文档互不覆盖，Skill 必需产物完整，真实测试/QA/PR 链路通过；检查新增文档合回主线的冲突情况。完成情况以 workflows-verification.md 本轮完成审计为准，不沿用此前草稿 PR 验收结论。


本轮结果：PR #11 已按授权临时处理并合入；通用模板与 Trellis 修复纳入源 Draft PR #5，标准新装/升级路径验证完成。新 Issue #12 的唯一 Run 完成 12 次节点执行，经真实测试与独立 QA 自动交付 PR #13，随后在测试仓无冲突合并；79 份原工作流文件逐字节保留。已知 P3 历史日志写入副作用和回执截断限制均保留，不计自动冲突修复、源主线发行或新版本部署。无需恢复定时任务。


完成审计发现公共任务状态未随 QA 闭环：为 report 挂载研发交付管理 Skill，并用新真实维护任务验证报告、任务和里程碑事实一致后再完成目标。新任务同时修复已确认的测试历史日志写入副作用，保持固定门禁和原历史证据。


收尾复验完成：report 的交付 Skill 由原 manifest 正式升级，并在新 Issue #14 的真实 Run 中读取和执行，公共任务/里程碑/总览的当前状态已对齐宿主测试与独立 QA；原失败记录保留，后续发布不预报完成。新维护任务在实际写入层修复历史日志副作用，研发与 QA 分别验证旧版红灯/修复绿灯，宿主完整门禁通过，106 个旧文件原文未变且无需人工恢复。自动创建 PR #15 后按测试仓授权无冲突合入；源 PR 仍 Draft，详细版本、证据及限制见 workflows-verification.md 的收尾修复最终验收。


## 真实产品演进验收（2026-10-06，In Progress）

目标：以测试产品的连续交付检验 Harness 是否能稳定承担实际研发。沿用团队自用模型网关定位，参考 New API 和 LiteLLM 的公开功能与交互；原实现保持独立。产品路线和业务 AC 由产品仓维护，本节只记录平台交付及验收责任，不建立第二份产品 Backlog。

用户授权持续自主推进普通选择、测试仓 PR 合并和测试部署；平台源 PR 仍保留评审，不合 main。此前删除的定时任务保持删除，以本聊天长周期目标续接。通用缺陷先修维护源代码、模板、标准安装升级与文档，再通过原 manifest 应用；不靠运行副本或测试仓临时补丁达成验收。

| 结果 | 产品工作入口 | 平台验收关注点 | 当前状态 |
| --- | --- | --- | --- |
| 首次配置到可追踪调用的控制台体验 | 测试产品 Issue #16 / PR #17 | 新需求正确进入需求/设计，真实交互契约、Trellis、完整测试、独立 QA、公共状态闭环、PR/部署 | Accepted：受控上游完整旅程、正式部署和浏览器版本一致；供应商 Not Run |
| 多上游配置、路由、故障与恢复 | 测试产品 Issue #18 / Run 82227c5986e5f41875895d8016c686a6 | 新数据契约与迁移、QA 返工、失败恢复、正式模板升级兼容 | 产品发布闭环完成：原PR返工/合并、正式非空升级与健康失败恢复、preview页面一致；其余平台联合负例继续补验 |
| 用量诊断、应用访问与额度治理 | Issue #20 / Run `9e6d05f409675e0ef65c6486691462df` | 权限、并发和状态一致性；缺陷从原层关闭；小修正确选择短路径 | In Progress：需求/设计完成、研发中 |
| 可复现运营和最终版本验收 | 产品发布任务及复验缺口 | 合并 SHA、Actions/Deployment、healthz、连续 UI 旅程、备份/升级/回退一致 | Planned |

每轮退出条件：原 Issue 接单及去重成立；阶段实际使用挂载 Skill；任务证据位于当前 Run 根并保留历史；共享项目测试与独立 QA 对应本轮代码；真实 PR 可审查且冲突有正式处理；授权合并后 Actions 准备/部署成功；运行页面、GitHub 最终事实和产品版本一致；页面原始用户路径通过。单个切片通过不等于整体验收完成。

特殊路由沿用既有验证主题，按改动影响复验：明确修复跳过需求/设计、自然语言指定起点、必要澄清、QA 回退、人工回退、交接后纠正、迟到结果隔离、停止/恢复/重启、外部失败恢复。不得把旧证据静默替换成新版 Pass。

首次配置必须从空状态由 UI 完成。受控上游只能作为有版本、可重复的测试夹具；真实供应商凭据缺失的联调保持 Not Run，不能在正式服务注入测试权限或伪造健康。真实浏览器、API、外部状态证据分别登记。最终以承诺用户旅程、质量门禁和公开限制判定，不以页面数量或 Agent 自报完成判定。

启动基线：维护源 `f4c14bf`；产品 main `d62cd51e8de2127548e1ac7469d349443d1bfe3d`。初查预览落后于 main；已通过现有 `deploy-local.yml` 正式 workflow_dispatch 发布，Actions `37389639182` 的 prepare/deploy 均 success，healthz 返回同 SHA，浏览器概览显示同 SHA。部署导致旧内存会话失效，经正常重新登录恢复，不以隐藏状态修复。产品 Issue #16 已由 GitHub Actions `37389861030` 接入唯一 Run `f1ff775cb6fee4d609353655cae5e180`，prepare 基于上述 main 且沿用 Trellis 0.6.15。当前没有本轮产品功能通过结论。


当前模板集成状态：维护源 `5a2b055` 已在本轮 Run completed 且正式 API 核对安装对象无在途执行后，备份并通过原 manifest 的 `install.py --upgrade` 应用。development 的重复失败诊断/证据预算策略与 report 的唯一当前状态/Skill 状态模型/外部证据链接规则已回读匹配；第二次升级无对象写入，全部对象 ID 保持，原 38 步 Run 完整 JSON 与升级前一致。下一条真实产品 Issue 继续验证实际 Agent 行为，安装通过不等于行为通过。

GitHub 入口原安装器 `--upgrade` 返回 Unchanged，仓库变量仍引用唯一维护源；409 后的状态指引修复已随维护源生效，completed 状态通知已由原 Issue 发布收尾评论触发真实复验；stopped/活动/查询失败分支仍只有回归证据。运行二进制无变更，不为模板升级重启服务。


第二切片发布接缝：原Issue #18评论6010003185/6010041466已在原设计会话处理，形成窄部署契约并保留至少一条精确旧版UI非空升级主旅程，现已交接seq6研发。平台维护源已实现契约绑定、显式upgrade、有限子进程、阶段记录/快照恢复及安全安装检查，38项隔离回归和独立代码复审通过；源d0564d0已通过原安装器升级，重复安装及原Actions旧版已部署SHA识别通过；真实新旧Go及正式Actions非空迁移/失败恢复尚待复验，不能据此关闭发布缺口。产品不复制控制器，不临时手工迁移。


第二切片宿主回退揭示诊断证据缺口：seq7固定门禁exit2自动进入seq8研发；24KB首尾回执截掉了中间实际失败断言。维护源增加有界脱敏命令日志、Run授权API/用户入口和阶段专属分页工具；先完成跨写入脱敏、容量、中文分页、跨用户和过期节点拒绝回归，再在静止窗口升级，下一真实测试回退复验。旧日志无法恢复，不将新实现的隔离测试计为当前Run已有日志或现场通过。


正式失败恢复入口采用现有部署Workflow的预配置隔离目标：默认预览保持，隔离安装使用原安装器的独立根/端口/服务，Owner显式选择后仍走同一精确main和完整测试门禁。维护源294ddb3已通过原安装器升级两个独立根，产品seq15通过原install_workflow.py --upgrade同步模板/manifest并验证重入；隔离旧安装的对象、密钥和两条真实历史已从UI准备并只读对账。维护源6e10c67新增有界端口故障夹具，53项回归与独立复审通过；等待产品QA和正式main候选后，从同一Actions分别验证非空升级及隔离恢复。真实注入/恢复仍Not Run，不把目录选择或夹具回归算恢复通过。

## 用户视角的典型路径与质量回溯（2026-10-07）

主要结果是可复用Harness用户路径及[使用手册](../04-guides/agent-platform-user-guide.md)，测试成品是同等重要的质量证据。不能以原型好看、Agent交接或草稿PR代替用户闭环，也不能只交产品而缺平台配置/接入说明。由平台维护者模拟用户、记录效果与瓶颈，阶段Agent负责真实产品产出。

| 里程碑 | 结果与退出证据 | 当前状态 |
|---|---|---|
| U1 已有原型接续得到可信成品 | 材料冻结/接收/逐阶段引用，PRD与原型/AC一致，成品真实UI/功能、固定tests/独立QA、原PR/合并/正式部署一致 | Active：8793全新Issue25/Run4cd62ced6a4f98f50e3cca6f7596eb06已进入seq6研发；需求/设计交接与典型返工恢复实测，完整测试/QA/发布尚未完成 |
| U2 不同研发路径可按手册使用 | 从头、原型接续、Bug、Code Review、部署的输入/配置/起点/出口/恢复明确；新仓库标准安装及各路径实际入口证据 | Active：手册初版已形成，历史短路径参考有效，专用Code Review仍未验 |
| U3 一句话需求到交互质量闭环 | 对照U1定位瓶颈；需求/规则/原型/AC追踪，必要可运行原型及浏览器检查进入通用模板，再通过原安装升级与新案例验证 | Planned：当前仅为待验证假设，尚不能归因需求或设计 |
| U4 完整考试与可复用交付 | 手册每路径状态与证据匹配，关键恢复和质量无P0/P1未关闭；源PR交付、正式版本和明确未测限制 | Planned：不沿用局部Go判完整通过 |

研究中同步手册的实测步骤和当前能力边界；只读Agent、QA和Code Review、图结束和部署分别说明。模板缺陷先维护源修复，原manifest安全升级；在途任务保留定义/现场。不得把一次临时救援写成用户标准流程。

U1→U3→U4为质量回溯关键路径，U2手册与既有路径复验可并行；涉及同一运行配置的升级必须等静止窗口。各用户路径用同一验证记录登记输入、操作、版本、结果与恢复，不再另建同内容状态表。当前计划自查结论Ready with Non-blocking Gaps：U1已分发并有真实输入证据，U2初版可审阅；专用Review、上游可运行原型及最终成品仍未验，不能标Accepted。

## 当前框架改造：角色与会话输入

已接受设计见 [会话输入](../02-architecture/workflow-session-input.md)，实施与回归见 [实施计划](workflow-session-input-plan.md)。维护源先修复角色/项目规则分离、Markdown Session Prompt/交接预览、明确等待及有界续跑，再沿原 manifest 升级。旧产品 Run 正式冻结，未修改其工作区或冻结输入。原产品质量与用户旅程目标继续 In Progress/No-Go，不能将这一框架验收切片称为产品全链通过。


## 半小时检查与持续推进（2026-10-07）

用户重新明确授权在本会话每30分钟检查测试仓model-relay的研发进展、完成度及卡点，并通过当前8793 Harness持续推进。当前绑定Issue25/Run `4cd62ced6a4f98f50e3cca6f7596eb06`，先核对实时状态，再采取正式handoff、固定测试、独立QA、返工或恢复动作，不重复建Run。定时任务 `model-relay` 已在应用中启用，挂在本会话；既有无关自动化保持不变。

最终产品目标由用户再次强调为：基本功能与交互达到商用模型中转管理控制台水平，对齐New API、LiteLLM的公开能力和用户旅程。当前design-v0.2.0仍是这一轮冻结实现/验收输入；完成度需分别列实现、测试、QA和发布事实，并形成与公开参考的覆盖/差距对照，不能以现有冻结材料有限范围自动证明全面对齐。发现尚未涵盖的必要能力，通过后续需求/设计交接完善，保留版本与用户决定。不得读取本机New API源码、数据和秘密。

卡点在原验证记录登记：实际状态、用户影响、证据、最小诊断、处理尝试、复验结论和未解决原因；能修复的通用问题先落唯一维护源，经标准安装升级复验。暂时解决不了时保留现场和明确下一步，不绕门禁、伪造回执或反复盲试。只在重要阶段结果、实质卡点、处理结论或需要用户介入时通知。流程验证优先，平台页面和跳转优化随后推进。


2026-10-08 00:18（Asia/Shanghai）首次半小时检查：主Run running/seq6 development，五份architecture-r1设计产物已实际存在并通过正式handoff交研发。研发已修改身份/租户/Key/额度费用/控制台API及迁移相关源码，原生事件完整分页核对至16:20UTC仍活跃；身份/Key轮换/微美元精度相关局部Go回归有exit0原生命令证据，不能外推完整make verify。当前没有Open PR、没有本轮独立QA或发布。用户对New API/LiteLLM的功能与交互对齐要求已通过正式User Input送入原研发会话（message1548、稳定request_id），要求在本Run文档记录覆盖/差距并交给QA；未重建Run、未改冻结材料。当前无必须人工救援的阻塞。

2026-10-08 00:48：主Run与原研发会话继续运行，完整事件核对至16:48UTC。三视口真实Go控制台旅程源码已接入既有make verify/browser入口；新增计费局部回归exit0，实际HTTP监听因原生沙箱边界失败，完整HTTP/UI验证待正常handoff至宿主固定tests Connector。没有Open PR或本轮QA/发布。message1548仍queued，尚不能认定商业覆盖补充已被处理；本次不重复输入。具体证据与处理记录见统一验证文档，U1/Task5仍Active。

2026-10-08 01:18：研发原会话仍活跃。首次本轮make verify实际执行exit2，在health-history因原生监听禁止而中止，失败日志已留在本Run validation/attempt-01；宿主固定tests尚未执行。go vet及计费/身份/治理/邀请的定向回归有exit0，治理回归保留编译和断言失败后修正历史，不能外推完整验收。商业补充仍queued；无PR、QA或发布。继续既有阶段和正式宿主测试交接，U1/Task5保持Active，卡点与证据在统一验证记录。

2026-10-08 01:48：seq6继续活动，新增资源/成员范围、迁移守恒和代际截止定向回归通过。成员读取Key范围有产品代码修复；资源Revision和schema1身份断言属于测试前置条件/入口修正，未将测试修正描述为迁移产品故障修复。全仓空选择仅编译检查。固定宿主门禁、独立QA、PR及发布仍待执行，商业补充仍queued；U1/Task5保持Active，具体失败/修正及限制在统一验证记录。

2026-10-08 02:18：implementation-r1候选、T001 In Review及逐AC待验报告已形成。首次正式tests handoff因待送达输入被正常拒绝，Agent结束turn后在原会话处理message1548；队列已空、不要求用户重发。商业对照检查又推动详情/列表搜索/Token汇总修正，新增前端失败保留，attempt-08的127项Node及计费race等补充检查exit0。完整宿主tests尚未接受交接、QA及发布未开始；待研发更新最终指纹/对照文档后正常推进。U1/Task5仍Active，事实见统一验证记录。

2026-10-08 02:48：seq7宿主make verify实际执行，真实HTTP/SSE、进程崩溃恢复及全仓race/构建完成，browser因旧迁移驱动期待schema2而exit2，正式failed路径自动返seq8研发。产品Agent保留旧迁移守恒层并新增生产schema3迁移/browser旅程；目标契约及迁移守恒的局部race通过，完整宿主门禁需重跑。商业/J01–J07对照与冻结范围外差距已落原验收矩阵，输入已处理。无QA、PR或发布；U1/Task5仍Active，失败和修复证据见统一验证记录。

2026-10-08 03:18：seq9宿主复验旧兼容迁移UI通过，但生产schema3 Key检查失败exit2，自动返seq10。研发定位driver误读data包络（正式为items），保留数量/身份/启停断言并加契约及安全诊断回归；135项Node和指定race等检查通过，当前seq11第三次宿主完整make verify running。未取得完整门禁退出码或QA/PR/发布，U1/Task5仍Active；失败、候选指纹和复验在统一验证记录。

2026-10-08 03:48：seq11生产schema1/2→3迁移及三视口原生管理页、旧UI和多上游旅程实际通过；新控制台在服务核验处失败，原failed边返seq12。前端补expected_execution_revision，受控列表fixture补object=list；137项Node及指定race等通过，当前seq13第四次宿主完整门禁running。无QA、PR或发布，U1/Task5保持Active；完整日志、根因、红绿回归及新指纹在统一验证记录。

2026-10-08 04:18：seq13确认服务核验200且版本/success/applicable一致，原服务修复真实生效；模型表单driver标签子串歧义导致门禁exit2，自动返seq14。驱动精确匹配及唯一性回归修正，138项Node/非监听检查通过，当前seq15第五次宿主完整make verify running。无QA/PR/发布，U1/Task5保持Active；失败、fixture修正及当前指纹见统一验证记录。

2026-10-08 04:48：seq15模型定位修复生效并推进至Key旅程，但粗阶段超时导致完整门禁exit2。seq16只增加11阶段安全诊断、141项Node及非监听检查通过，未猜改产品或宣称Key修复；当前seq17第六次宿主完整门禁running，根因待具体回执。U1/Task5仍Active，QA/PR/发布未开始，现场/假设/下一步在统一验证记录。

2026-10-08 05:18：seq17定位Key签发成功但dialog close误清转交值，seq18修生命周期与跨epoch归属；seq19真实Key/调用/同ID详情及1280后续旅程越过原卡点，键盘断言失败返seq20。driver新增异步editor可见性等待及保留原断言的分段诊断，152项Node等通过；当前seq20仍running，完整宿主重验/独立QA/发布待执行。U1/Task5保持Active，具体根因与实测边界见统一验证记录。

本轮结束前复核：seq20已正式交接，当前seq21第八次宿主完整tests running，尚无退出码；独立QA及发布继续待验。

2026-10-08 05:48：seq21首次完整make verify exit0，新控制台三视口实际通过，正式进入seq22独立QA。QA发现P1强制UI旅程覆盖不足及P2 Key列表状态问题，结论No-Go，报告与整改交接已部分落盘。模型容量故障使Run failed；本轮通过既有CI所有者正式resume接续原QA会话/同native线程，实际继续活动，无换模型或重建任务。当前QA仍running，待正式返研发补覆盖，再门禁/QA；PR及发布未开始。U1/Task5仍Active，证据分层和恢复详见统一验证记录。

2026-10-08 06:18：恢复后的seq22独立QA已completed，正式No-Go交接development接受，seq23研发running。产品Agent新增独立空库、测试专用受控时钟及三视口J01–J07连续UI旅程并接入原browser固定入口；Key状态/授权/预算展示和无租户加载态已形成修复候选。156项Node及7项定向race等非监听检查通过，新122文件指纹cb51d80c…的完整宿主make verify/新旅程尚未执行，QA22两项仍Open。真实容量故障恢复已完成原线程报告→正式返工交接，未换模型/重建Run。U1/Task5保持Active，无PR/合并/新部署；详细边界见统一验证记录。

2026-10-08 06:48：seq23最终157项Node/指纹18207762…正式交tests；seq24第九次宿主make verify exit2，旧三视口控制台等通过，新continuous-console在J01初始状态失败，自动返seq25研发。已定位driver把全新安装误作含legacy租户的迁移安装；Agent修零集合断言和安全诊断，6项Node及新安装in-process定向race通过，完整宿主仍待复验。上一轮“init保留空legacy”的维护者记录判断不准确，已在验证记录明确更正；生产初始化未为测试修改。QA22缺陷仍Open，U1/Task5保持Active，无PR/新部署。

2026-10-08 07:18：seq26第十次宿主完整门禁exit2，真实初始三集合均零，已越过上一断点；连续J01首次调用502，原failed边返seq27。实际受控上游响应缺assistant role，产品Agent只修fixture，保留生产协议校验/已知未知收费规则，实际夹具正反回归及159项Node等通过；当前seq28第十一次完整make verify running。没有J01–J07完成证据或新QA通过，QA22问题保持Open；U1/Task5仍Active，PR/合并/发布未开始，细节见统一验证记录。

2026-10-08 早间用户询问累计成果时复核：seq28完整门禁exit2，但新增continuous1280的J01–J04已实际passed，暂停恢复J05费用守恒断言失败；seq29仅增加分段安全诊断及回归，正式交接后seq30完整tests running。当前不能称J05根因已修复或七旅程/独立QA完成；U1/Task5继续Active，待真实比较金额回执再最小归因。完整日志证据与边界见统一验证记录。

2026-10-08 08:47：seq30第十二次完整门禁exit2，新增1280/J01–J04及原三视口控制台继续通过。诊断实测费用126→126、调用数9→9等全部守恒，早间把粗J05标签判断为费用断言失败不准确；失败边界在后续成员操作，seq31正在拆分。源码/延迟回归确认driver刷新可能在租户同步完成前读取旧选项，已补候选等待与12项driver回归通过，真实原断点尚待宿主归因/复验。原Run正常running，QA22两项Open，U1/Task5未完成，未PR/发布；GitHub首次TLS读取失败后REST核验成功，非产品或Harness阻断。

2026-10-08 09:17：seq32第十三次完整门禁exit2，但新增1280/J01–J05及J07实际passed，成员刷新/隔离旅程越过原断点。J06账单核对driver误读首条不可改写的零费用账目为累计费用21；seq33仅修累计账本读取并加强原项/快照/追加差额断言，165项Node与定向race等通过。当前seq34第十四次完整make verify running；新指纹50fb764a…完整结果/其他视口/独立QA待验，QA22问题未关闭，U1/Task5仍Active，未PR/合并/部署。

2026-10-08 09:47：seq34完整make verify exit0，指纹50fb764a…三个视口×J01–J07共21/21实际通过；seq35独立QA核对指纹/18产物，关闭QA22-02状态展示，保留QA22-01故障/并发覆盖残项并新增QA35-01 P1：写结果不确定时首次安全GET失败清掉读回重试按钮且不显示操作标识。真实生产函数Contract复现exit1，尚非Chromium复现；原QA正式No-Go返seq36研发，当前running，正在修恢复流程并补AC08/09/16，未获得修复重验结果。Harness门禁成功→QA→正确返工正常，无新平台阻断；U1/Task5继续Active，Open PR0、近期Actions仅Issue入口，5545仍旧main9851177/schema2，未新合并部署。具体原因、证据和真实外部验证限制见统一验证记录。

2026-10-08 10:17：seq36安全读回修复候选已正式交tests，当前seq37第十五次原完整make verify running。生产editor保留独立操作引用/重试与unknown提交锁；169项Node、新两层五维/共享竞争12分例in-process race及原6项race实际通过，三视口轮换丢响应/双发送与实际HTTP新矩阵已接原门禁但尚无宿主回执。当前127文件指纹3ee2122e…，不继承旧50fb完整绿；QA35 No-Go及两项未关残项保留，QA22-02仍Closed。Harness接受/派发正常，无新平台阻断；U1/Task5 Active，近期Actions仅Issue入口，5545仍旧main/schema2，未新发布。真实UI旧行为隔离重建不是原QA Chromium红灯，详细证据与测试前置失败纠正见统一验证记录。

2026-10-08 10:47：seq37完整make verify exit0/101748 bytes，当前127文件3ee2122e…三视口故障与21主旅程、新ActualHTTP资源矩阵全部通过；QA38同指纹独立复验/63产物核验，关闭QA35-01和QA22-01指定覆盖主题，QA22-02保持Closed。仍No-Go：原强制AC01十页批准原型与实际页面逐页对照未齐，仅h1导航烟测不能放行；已正式返seq39补采集/映射，当前running，不改冻结输入或重写已通过功能。Harness关单→补证返工路由正常；U1/Task5 Active，Open PR0，Actions本轮TLS读取失败未知，5545仍旧main/schema2，未新合并部署。缺口是验收证据，不冒称新产品P1，详见统一验证记录。

2026-10-08 11:17：seq39 completed并正式交tests，seq40第十六次原完整make verify running。新增十页真实页面/详情/编辑采集和逐页映射接原J旅程，计划三视口63PNG/3JSON尚未作为真实产物通过；Key额度复用资源摘要及详情中文标签候选、173Node与非监听检查实际通过。新129文件bb30f817…不能继承旧完整门禁绿，QA38-01仍待宿主/独立QA判断，旧缺陷关闭事实保持。Harness派发正常，U1/Task5 Active，Open PR0，旧5545/schema2保持，无新合并部署；设计差异和证据限制见统一验证记录。


2026-10-08 11:47～12:12：**测试仓**seq40原完整make verify exit2/101576 bytes；173Node、全仓race/build、原三视口控制台/故障及新增1280七旅程绿，首visual-overview采集断言失败，具体字段待真实安全DOM诊断。QA38-01仍Open/No-Go，旧三项关闭事实保留，不把旧绿或计划截图作当前完成。**Harness**同时耗尽原冻结40次上限，无正式有界恢复入口；维护源新增授权检查原因/目标/有限max_steps的return，保留冻结图/历史/回执/workspace，不自动重放。Go/API与Node红绿、完整scripts/verify.sh exit0后，无在途备份升级8793；标准manifest升级连续两次幂等、同ID，新模板100、原Run40不变。正式CI Token return seq40→41 development HTTP202，有效预算60/冻结40/原40步深比较一致，原生Codex已实际运行定位采集断点。未手改DB/回执/权限/冻结输入，产品实现仍由Pipeline Agent；U1/Task5及产品新PR/合并/发布未完成。源只在开发分支交付，不创建Platform PR/合main；其余未测典型负向不因本次恢复关闭。详证见统一验证记录本轮条目。


2026-10-08 12:18：恢复后seq41 development仍running，尚无已接受handoff。产品visual-boundary-r1只补采集断言前安全DOM/overview GET白名单/准确check留证，原.empty/字段/角色/范围/尺寸/秘密门槛及生产业务保持；最终176Node和静态检查实际exit0，新129文件fb766c0a…仍待原完整宿主诊断/独立QA。空态原因未证明，QA38-01 Open/No-Go，U1/Task5/发布未完成。Harness有效60/冻结40稳定，无新平台阻断，维护源abe0dd6已推送开发分支、无Platform PR/main合并；前轮检查/标准升级证据保持，不重复任务/门禁或盲目放宽。原生事件/检查与归因边界见统一验证记录。


本轮结束核验：seq41已completed，原生handoff事件41905正式accepted=true/target=tests，新seq42第十七次原完整make verify running且connector_dispatched=true，无error/当前结果；未手动追加测试、Run或输入。交接携当前fb766c0a…诊断指纹、真实旧失败、host-retest及QA38原返工依据，明确保留门槛，不能预期诊断回合一定绿。5545本轮health200/schema2/version9851177仍旧部署，Open PR0。后续等待seq42正式完整结果，再按准确安全事实归因；不把正常派发当已修复或通过。


2026-10-08 12:48：seq42完整诊断门禁exit2/103232 bytes，真实overview GET200/scope匹配、字段角色尺寸均通过，唯独合法近7日无调用empty触发非空采集门槛，根因已证为J06跨月后采集缺当前周期活动。seq43产品Agent仅补原生UI真实调用及当前周期费用21/成本7/唯一charge/旧结账完整对象守恒，保留所有原采集/生产门槛，179Node/静态检查局部绿；新129文件31bbf962…正式handoff事件43126 accepted至seq44第十八次原完整tests，当前running。真实十页/全部视口/独立QA仍待验，QA38-01 Open/No-Go，U1/Task5/新发布未完成。Harness有界恢复后测试失败→正确返研发→再次测试实际正常，无新通用故障；底层模型事件无model字段不新增验证结论，旧H边界保留。当前Actions仅Issue入口、health仍旧9851177/schema2，Open PR本轮读取失败未知，具体根因与红绿层级见统一验证记录。


2026-10-08 13:18：seq44当前周期UI活动真实通过，采集到limits-editor后因按钮在heading、旧driver限于content导致timeout，正式返seq45最小修locator owner，所有原门槛/生产业务保持。当前129文件7e0b20dd…seq46第十九次原完整make verify真实exit0/104299 bytes，182Node、全仓race、原三视口故障及21主旅程、新十页三视口全部通过；维护者独立核验新63PNG引用/SHA一致，人工查看两张实际图。已进入seq47独立QA；正在判断模型/租户主要信息与冻结设计是否齐全，当前无正式最终结论，QA38-01仍Open，不以采集绿代设计验收。Harness原失败返工→tests绿→QA正常，无新平台故障；U1/Task5/新PR合并部署未完成，旧5545健康/schema2版本9851177不变，Open PR0，近期Actions本轮读取失败未知。证据与候选/已测/QA在途层级见统一验证记录。


2026-10-08 本轮 Issue25 实时核对：QA58已正式完成，产品候选Go with known issues；QA47-01 P1与QA47-02 P2原层Closed，新增QA58-01 P3技术枚举/窄列断词Open非阻断。seq57完整make verify exit0/106669 bytes/truncated=false；report59完成，publish60正式Connector exit0，真实提交66f1328ada75ea90e5638fbf2bd58663ac86347e已推送workflow/4cd62ced6a4f98f50e3cca6f7596eb06。维护者独立逐件git对象核对134文件与QA58清单及fb4066fa37d1e7865d3a18f72dd78fef98ae79bde17e20875af373b24771c3a6源戳完全一致；GitHub远端ref及固定提交QA报告可访问。

实际卡点：有效预算60耗尽，Run failed/seq60，error为maximum node executions reached；pr尚未派发、Open PR0，不是产品回归失败。检查冻结边publish→pr→done及正式已推送回执，余下为两个明确节点，无返工循环；准备按已有自主接续和测试仓PR授权通过Owner正式return(seq60,target=pr,max_steps=64)，不重放已成功publish、不新建Run/Agent、不改冻结图/历史/DB/权限。恢复结果待后续真实API回读记录。5545仍旧9851177/schema2，无新合并部署；H及真实供应商/支付/外部身份边界继续未验，定时任务保持PAUSED。


正式恢复回执（2026-10-08 19:39 Asia/Shanghai）：Owner return HTTP202，seq60→61 pr，有效max_steps64。原始60步及冻结definition经管理员正式API深比较完全不变；CI return响应含调用者权限相关配置脱敏，与管理员响应不可直接作为同字段比较，先前核对脚本误报AssertionError已通过同Caller回读排除。pr61由正式github.pull_request Connector创建草稿PR27，head_sha66f1328ada75ea90e5638fbf2bd58663ac86347e；URL https://github.com/big91987/model-relay/pull/27 ，已附到当前会话。done62完成，Run completed/error空，不重放publish。Run completed仅表示编排已交付PR，不等于用户部署目标完成；PR尚未合并、正式新Actions部署/health/schema3/实际用户页面未完成，旧5545版本保持。产品Go with known issues与P3保留，定时仍PAUSED。

用户询问预算和Web验证方式：原Run冻结40，协调者上次恢复设60，本轮明确检查只余pr→done后正式设64；是节点执行总次数，包含Agent/Connector/结束节点和返工，不是模型消息或工具调用次数。当前通用模板100，未因此改写老Run预算。seq57宿主make verify经go run ./tests/browser启动真实Go/隔离SQLite/受控HTTP-SSE上游并用Playwright浏览器执行实际UI；1280/1440/390全部J01–J07通过。QA58自身未重新操作浏览器全链：独立核验版本源戳/固定完整回执、设计比对30页+8关键图，独立193Node/5非监听Go race，关闭原层缺陷。区分宿主UI-E2E、独立证据/视觉验收和未完成的正式部署UI，不能声称QA亲自完整复跑或供应商联调已验。


## Model Relay持续交付（2026-10-08）

目标：从当前design-v0.2.0候选推进到可实际使用、可稳定运营的模型中转管理控制台，逐步对齐New API/LiteLLM公开基本能力和用户旅程；每个阶段形成可部署、可验收的结果。不承诺一次做完参考产品全功能，不编造日历期限或完成百分比。当前冻结输入、代码和QA58是首轮发布基线，后续范围经正式User Input→需求/设计→研发/测试/QA承接。

维护约定：本节维护长期里程碑/依赖和下一批工作；workflow-node-agent-plan.md维护Harness通用改进，workflows-verification.md维护真实证据和卡点。产品本身的任务/基线/验证由Pipeline写入测试仓现有docs/workflow/runs/<run>/delivery，Issue关联该权威产物，不由协调者手改产品工作树或另建重复状态源。已完成旧Run不恢复，下一阶段走当前标准模板正式新Issue入口，引用已合并产品提交和明确范围。

| 顺序/阶段结果 | 当前可执行任务及负责人 | 依赖/进入条件 | 验收退出条件 | 状态 |
|---|---|---|---|---|
| 1 当前多租户控制台真实上线 | 现有Issue25/PR27：只读合并前复审、精确head授权合并、正式main prepare/Owner preview deploy；协调者/Connector负责发布核验 | seq57+QA58同134文件源戳，现有授权；复审无未修Critical/Important | PR/merge/Actions/Deployment环境URL、health version/schema3同版，旧schema2经正式升级保留数据和身份，实际登录页面验收；T001仍In Review至发布事实闭合 | 已正式部署3f9cee16/schema3；已登录后独立实操待 |
| 2 管理页面可理解、操作连续 | 依据QA58-01 P3做术语/窄列/状态文案完善；补独立self_hosted首次网页分支、真实管理入口与错误恢复；产品Agent实现，独立QA验收 | 阶段1已合并版本；QA原编号和截图；问题输入已有明确AC，复杂偏差返需求/设计 | 从页面完整完成平台管理员与租户管理员主要旅程，三视口、键盘/弹层/范围/错态可用；完整make verify与新独立QA、PR和正式部署；不改费用/权限口径来换界面绿 | Active，Issue30/PR31同步修复已部署3f9cee；Issue32/Runb7f93c24接续QA58-01 P3与首次自托管网页旅程 |
| 3 商用基本能力按用户旅程补齐 | 先在需求节点沿已有商用五层矩阵核对接入/模型/路由/Key/团队/预算/费用/日志；明确已支持、已验证和真正缺口，一次选一个可发布纵向切片，由设计/研发接力 | 官方公开参考与本产品实测，不复制本机New API资料；新承诺由正式需求/AC明确 | 新用户能够接入→授权→调用→诊断→对账；适用故障切换/限流/成员生命周期等能力各有确定语义、页面和同版验证；新增范围独立版本化，不改冻结v0.2.0 | Planned，缺口排序输入待需求节点 |
| 4 升级、恢复及运行可维护 | 正式validation目标上的非空升级/失败恢复、备份/旧数据守恒、重启未知请求和费用处理；性能/容量目标先测量再立基线；产品Agent及受支持运维流程负责 | 阶段1真实schema3部署；影响故障演练只用已授权隔离目标，不对preview做破坏实验 | 标准新装/升级/回滚步骤可复现，源记录不被预检查改写，恢复后Key/历史/账本守恒，故障证据明确RPO；已证明的容量/边界和告警动作可定位 | Planned |
| 5 外部联调与运营能力按需要扩展 | 真实供应商/已有推理端点联调；外部身份、支付充值等先由需求明确场景和边界，再设计实现 | 凭据、外部账号/费用授权或受信维护者输入实际存在；缺失时其他阶段继续 | 实际请求/报价/身份/支付回执与页面一致，失败和权限反例有证据；没有真实事实继续Not Run，不把受控上游当商业联调 | Planned，外部依赖条件未满足 |

关键路径：当前PR复审→合并→官方prepare/deploy→真实页面→下一阶段正式Issue/需求；阶段2可预备问题包，阶段3仅做只读差距调查，不用准备工作冒称Ready开发。阶段4的运行测试可在隔离目标与产品迭代并行，修改同一迁移/计费契约时顺序集成。暂不规定固定WIP/迭代日期。

公开参考（2026-10-08核对）：[New API用户文档入口](https://docs.newapi.pro/en)列明渠道、令牌、模型和实例管理；[LiteLLM管理UI](https://docs.litellm.ai/docs/proxy/ui)、[Key](https://docs.litellm.ai/docs/proxy/virtual_keys)、[预算](https://docs.litellm.ai/docs/proxy/users)、[用量](https://docs.litellm.ai/docs/proxy/cost_tracking)及[路由/故障切换](https://docs.litellm.ai/docs/routing-load-balancing)。这些仅用于能力/用户旅程对照，不自动将所有协议、支付、SSO、HA或新发布特性纳入当前产品承诺；现有QA58商用矩阵沿用并由后续需求节点补差距。

每遇卡点先记录用户影响、实际Run/提交/回执、原因假设、已尝试动作和下一步；做最小复现，失败保留现场不循环盲试。产品缺陷交正式development/QA返工；Harness通用缺陷先修唯一平台源码、依赖/模板/安装升级，再正式应用、复验故障和相关恢复。发布成功、受控测试、独立实操QA和真实外部联调分别记录。

计划审查：沿研发交付清单检查无循环硬依赖、无重复Issue/状态源、无新冻结范围或隐含外部授权、未验不标Done；当前发布Ready with Non-blocking Gaps（QA58 P3及明确外部未验），后续能力阶段待受支持需求/设计基线。


最新合并前复审：PR27发现零软预算及回拨过期凭据两项源路径可达P1，复现待产品Pipeline；当前发布暂缓，保留原QA58证据并正式返development补回归/最小修复/完整测试/独立QA，详见workflows-verification.md本轮发现记录。阶段1继续Active，其余里程碑状态保持，不因新发现擅改冻结需求或手改产品。


用户最新要求恢复持续定期推进（2026-10-08）：通过原automation_update更新model-relay为ACTIVE，保留原30分钟频率、当前thread和通知策略；同步长期阶段、PR27/研发63返工、新旧测试版本及QA浏览器边界进任务prompt，不创建重复定时任务。配置正式回读ACTIVE。当前同Run seq63 development running，原生会话a61735cc14d2675688203cc090f2e44f，自报“内进程已复现两项缺陷；继续补重启、期限和窗口守恒回归”；这是Agent执行进展，不等于原完整宿主测试或独立QA通过，PR27仍待修复版重新验证再合并。定期检查先核实时态再推进受支持入口，卡点记录并诊断修复；重要结果/问题通知，正常未变保持安静。此前PAUSED记录是历史，不代表当前自动任务状态。


### 2026-10-09 05:40 原生容量失败的有界接续

Issue30研发4已完成诊断切片但在归档/交接前容量失败，未完成目标复现或产品修复。一次Owner正式resume保留原节点、会话及前序副作用，不重启Run/重放写入；后续应沿原handoff进入tests，固定make verify首次真实诊断红需返研发最小归因，不能预期红即Done或重复全套求绿。当前服务保持正式9bf6ecaf健康，未新发布。


同次fresh回读：恢复input17494已running，原生新progress17495/parent17494于21:43:56Z明确承接“核对现有诊断与失败回执、补归档和源码戳，不重放写入/重跑完整门禁”。这证明同会话恢复后原生实际接续，仍未证明最终handoff或宿主目标复现完成。


### 2026-10-09 06:10 页面同步契约取得原层证据

真实修前hold/release四case证明完成判断缺口，正式固定tests5失败后自然返development6最小修复，不将预期红当通过。新候选须完整冷门禁和独立QA，既有服务保持已部署9bf6ecaf，未发新PR/部署；旧R28-04精确历史断言原因继续未知，不倒推闭合。


同轮候选材料变动后candidate-06-after为150件d187194806f62af8628b4f68216a509ec0d6240fa0474ddc71aaf6475cc54c89；47a199为中途版本，不混成新宿主通过。development6仍running、归档及handoff待，既有首红自动返工保持。


同次独立只读复审完成：22:14:02Z按tests/evidence库存算法重算150件0384f115adbbd545e8ce01021f238010ba66c89eb06e6acea08ce872ce0fa474，22:14:13Z candidate-06-after同戳；47a199/d187均为归档中途，重点6代码/spec SHA两次读取完全一致。复用既有operation/epoch与busy、读失败独立提示、旧epoch不释放新锁、secret/invite分离、严格断言及错误身份保持，未见新增Critical/Important。reviewer未运行测试，静态结果仅允许下一正式门禁，不等于新宿主/QA绿或可发布。最终fresh研发6仍running；下一tests需以实际冻结源戳核验，不预报handoff。


### 2026-10-09 06:40 同源完整验证与独立QA完成

原层修前tests5红转新tests7完整冷绿，独立QA8关闭已证R30完成契约缺口并保留旧R28-04边界。正式report9进行中，下一出口仍是Pipeline publish/pr精确源绑定、授权合并及Owner Actions，当前服务未更新。已登录预览/动态独立QA和供应商未测不写成通过，不创建重复Issue或恢复旧Run。


### 2026-10-09 07:10 精确发布合并与正式部署在途

Pipeline完成PR31，不复活已completed Run；已验150源及26QA文件精确发布映射、远端任务分支证据可访问，授权合并main3f9cee后由既有Owner Actions启动preview，prepare尚执行/dispatch pending。完整实际结果与health版本仍待，现网9bf6ecaf保持；下一长期阶段待本次部署闭合后按已有QA58-01 P3正式Issue入口承接，不停旧Run或重复功能。


### 2026-10-09 07:40 同步修复正式上线并接续页面改进

PR31合并版本3f9cee1615e87e002a2c2300f2d29778072ec3f5已正式prepare/deploy成功，Deployment6949052209/local-preview/用户入口http://127.0.0.1:5545/admin/；实际health200/schema3/version一致，实际workspace.js与已验发布对象字节相同。账号、租户、成员及业务配置和正式备份只读保持，console_clock正常推进有差异，不能说整库所有行不变。原生动态独立QA与预览已登录实操仍待，供应商联调仍NotRun。

检查GitHub当前Open Issue无重复P3任务、原30/28/25 Run均completed后，新建唯一Issue32（https://github.com/big91987/model-relay/issues/32），沿现有阶段2、冻结design-v0.2.0和已合并main3f9cee接续术语/窄屏可读性及self_hosted首次网页分支。自动入站Actions37861181092进入Runb7f93c24bc1ea4d0fe182f12aa7622ae，prepare1/issue2已completed、intake3/dc330734fa1ba6376edb70c44b47ca57 running/error空，max100；未重复dispatch、恢复旧Run或由协调者写产品。新候选必须原完整make verify/独立QA/精确PR合并和正式部署，本次创建任务不是产品改进已完成。


本轮最终回读：入站Actions37861181092 completed/success；新Run32仍intake3 running/error空/max100/workspace github-issue-5770505233。automation_update正式回读ACTIVE/原30分钟频率、新Issue32/Run锚点生效；未重复启动或扩大预算。


### 2026-10-09 08:10 Issue32研发已交接正式门禁

正式intake3完成并直接development（冻结范围内P3与已有分支补验，不另建PRD），development4通过原handoff→tests5 running/00:08:41Z。候选实际包含中文状态/角色、未知值、完整长值及表格横滚、独立self_hosted网页旅程与TCP关闭/401纠正恢复；仅已实现待验收。维护者独立按tests/evidence算法复算152件d78781505e2ca5bfd3fe5d357ea3379cb9a414d7cd127ea0248c2491bc633180同handoff，未代跑产品。新Run真实delivery/任务/里程碑/术语盘点由Pipeline新建；intake关于资料“缺失”是新Run尚未创建文件的历史现场，不要求恢复不存在产物或复制旧状态。原生受限尝试非新候选完整通过，必须正式宿主/独立QA后继续发布。当前预览仍3f9cee/schema3，未新PR/合并/部署。


本轮独立只读复审完成（00:12:02Z）：同152件d787815…，未发现新增Critical/Important/Minor明确缺陷；没有代跑测试或编辑产品。正式tests5仍running/error空、完整回执和QA待；原automation_update回读ACTIVE/30分钟/最新候选及tests锚点生效，不增加预算/权限或重复任务。


### 2026-10-09 平台测试策略与专用产品测试流水线

用户澄清：Harness Builder是协调者测试Agent Platform和Harness模板时的角色，影响验证策略，不是协调者所有工作的唯一职责。在这种测试中让Pipeline Agent执行具体产品任务、页面操作、供应商调用及验收，用实际执行检验流水线的可靠性、能力缺口与效果，不由协调者代做用例掩盖问题。其他工作按用户授权和任务目标执行；不恢复已停止的30分钟自动任务。AGENTS.md已加入限定范围的测试策略及【产品测试】类别，Issue/PR正文仍遵循问题、目标、范围和实际证据规则。

本轮Harness能力切片：新增标准product-e2e模板（源6d1c57b）及显式network/elevation授权、真实浏览器Skill挂载和可保留/明确撤销的标准升级。入口独立工作区→产品测试Issue→旅程计划→测试准备→固定Python实际执行→独立浏览器复核→报告；测试失败仍到复核，test_repair仅测试设施，不代产品修复/发布。产品测试范围来自用户：真实DeepSeek、两个指定模型、用户/租户角色/模型授权/应用Key配额/普通SSE与诊断恢复；测试计划/脚本/产品判断由测试Agent制定。凭据只在本机私有忽略文件，不写公共材料。

已标准安装新编排3155ddd1206d4ebc19da94e5b489091f（http://127.0.0.1:8793/workflows/3155ddd1206d4ebc19da94e5b489091f），实际GET确认原生Codex/gpt-6.1-sol、network_access/allow_elevation true、执行/复核挂载Playwright Skill；省略可选授权和浏览器参数的标准upgrade后同ID/revision/配置逐值保持。原研发编排8f497…revision2/权限均false不变，没有修改在途Run。

正式新Run c41e8047001414b96d327c39fb9c1eeb首次prepare失败暴露通用materials空parameters缺陷，先修唯一源28b6469并完整回归/独立复审，再标准upgrade及stop→return同Run续验，prepare2真实exit0。Pipeline自动创建唯一【产品测试】Issue33（https://github.com/big91987/model-relay/issues/33），e2e_plan4/c81a84d7933adfb6592450d5f68d0fc4 running。此状态证明Harness支持启动/恢复/自动Issue，不等于已执行DeepSeek或产品通过；实际浏览器、外部调用及独立实操效果由后续Pipeline证据核验。本次平台测试中协调者处理执行能力和故障/复验，产品缺陷包交产品研发流水线，不亲自改用例或操作产品完成验收；该分工不扩大为其他任务的全局限制。


### 2026-10-09 产品测试报告接续产品修复

用户授权测试完成后，将已复核产品问题创建/更新【产品修复】Issue，附测试报告及相关产物，直接进入研发development实现。本次沿用现有prepare→issue→intake→development短路径，准备/材料校验/关联保持，跳过无关需求/设计，后续固定测试/独立QA/交付保持。不把新问题塞进无关在途Run，不由Builder代产品修复。

维护源e2e_report要求实际问题包与文件级artifacts：受测版本、用户影响、复现与预期/实际、首失败回执、相关脱敏文件、原AC与回归条件、已有Issue关系；intake明确已复核输入足够时直交development。报告/证据通过原SHA-256 ZIP材料入口传新工作区，不能只贴本机路径，秘密不随包发布。先查既有任务按修复范围去重；未执行/凭据问题不自动算产品故障，无已确认缺陷不造任务，Harness与测试设施单列。

当前测试Run c41e8047001414b96d327c39fb9c1eeb仍e2e_plan4 running/error空。正式User Input request_id model-relay:product-e2e:repair-development-handoff:20261009已实际接收message20418/running；补充覆盖当前冻结Run，不修改图或升级在途流程。测试未完成，暂无最终报告、修复Issue或development接单；这些须据后续真实产物核验。原model-relay定时仍PAUSED；尝试新增报告完成后周期跟进被自动审批拒绝（用户已停止定时），未创建新自动任务，不用其他方式绕过。是否采用本次自动完成跟进待用户明确选择。

最终正式API回读：测试Run c41e8047001414b96d327c39fb9c1eeb已从e2e_plan4进入e2e_execute5/running/error空；这是测试执行准备接单，不是测试完成。修复Issue、跨Run材料接收及development接单仍待实际报告；未新增定时任务。交接标准源9dc693a已推开发分支，原在途图保持。


### 后续优化：GitHub发布身份与消息来源

用户在Issue33截图反馈：平台Hook自动回写和协调者通过工具提交都显示big91987，难以区分本人输入、协调者操作和Agent Platform/具体节点回复。目标是在GitHub可见作者与正文来源上明确区分三者，并保留Run/节点/会话关联，不要求用户查看隐藏HTML标记猜来源。列入后续Harness优化，本轮仅记录，不切换凭据、不修改在途任务或旧评论。

现状：Connector由token_env读取Bearer凭据调用GitHub，Hook正文已有隐藏agent-platform-hook标记作去重/防反馈，未提供用户可见来源。独立平台用户ID不能改变GitHub评论作者。候选方向为GitHub App installation身份：安装令牌的动作归属App bot，而非用户身份令牌；目前平台仅读取已提供令牌，App凭据/安装/短期令牌生成续期/标准升级尚待实现与验证。先实现清楚的来源展示，后续接独立发布身份时还须覆盖最小仓库权限、令牌过期恢复、评论去重/结果未知核查、Hook不反触发新Run及原生输出保持。官方依据：https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/differences-between-github-apps-and-oauth-apps 和 https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation 。

2026-10-09 用户授权直接创建，名称固定为 `agent-platform`。维护源已实现服务私有 GitHub App 注册表、RS256 installation token 签发、限定 Connector 仓库、临近过期续期与并发取消；复用原凭据引用，不在工作流或原生输入传私钥。安装器新增 `--notification-token-env`，只分离自动评论身份，保留 Git/Issue/PR 原凭据，普通升级保留已配置身份。目标注册权限仅 Issues read/write、只安装 model-relay；未授权扩大到其他仓库。

代码定向回归与独立复审通过；复审发现 JSON null 注册表可能回退个人身份，已真实红→绿并关闭 Important。最终全量 scripts/verify.sh exit0，包含前端/安装升级/SDK/Controller、go vet、全包 race 与构建。Chrome GitHub 创建入口停在 Confirm access，需用户直接完成网页二次验证；尚未创建 App、下载私钥、安装仓库或切换运行服务。App 实际作者、正式 Hook 去重/失败恢复及标准实际升级仍待验证，不能把本地测试当机器人已上线。原定时任务保持暂停。

后续用户完成 Confirm access；真实创建表单填 agent-platform、Issues read/write、强制 Metadata read-only、Webhook 关闭、仅本人账号安装。提交后 GitHub 明确拒绝：`Name is reserved for the account @agent-platform`，App 未创建。名称为用户指定，不自行改名，已询问替代名称（候选 agent-platform-bot / big91987-agent-platform 可用性尚未核验）。源码49f1aa8已推开发分支；实际安装、私钥、服务切换和机器人评论仍待。

用户随后选择 agent-platform-bot；真实提交再次拒绝 `Name is already taken`，仍未创建 App。已请求是否可用账号前缀及必要后缀，不能把候选名当可注册。接入前只读 API 核到8793两条 running（产品测试seq5、研发seq17），运行服务未重启、冻结图未改；实际应用需安全窗口或受支持独立验证安装。

用户随后指定 `big91987-agent-platform-bot`，真实注册成功，App ID 5245414，Owner big91987。当前 Key pairs 0 / Client secrets 0，未安装仓库或应用到服务。浏览器操作规则要求生成长期私钥及实际仓库授权时确认，已提交合并确认（仅 model-relay、Issues 读写/Metadata 只读），尚待用户答复；不扩大权限或跳过确认。源码与本机完整门禁完成不同于机器人真实发布完成。

后续用户自行生成私钥并完成 installation169456895。安装最初为 All repositories，按此次既定范围通过正式 GitHub 设置收窄并保存为唯一 big91987/model-relay，权限 Issues write/Metadata read 保持。下载文件两份，仅原文件 SHA256公钥指纹与 GitHub key_pairs 一致，匹配私钥复制到忽略的私有目录（key/registry0600、目录0700），未发送给 Agent 或提交仓库。

同已验平台二进制在8794独立数据/工作区通过公开 API 走 approval→end→run.completed Hook。私有验证配置无效 issuer 实际401，通知failed但Run completed；恢复合法配置并正式 retry 后succeeded，实际评论6073250016作者 big91987-agent-platform-bot[bot]/Bot。再次retry实际409，正常重启8794后同通知/评论唯一，GitHub Actions37876468415 skipped、8793未新增反馈Run。证据 .data/github-app-validation/acceptance.json。8793仍产品测试seq5、研发seq20两条running，未重启或升级在途模板；主服务切换与标准安装器实际notification身份升级仍待安全窗口。不得将8794验证成功说成8793已换作者。


### 2026-10-09 审批卡点在 Issue 不可见：实际补报与通用修复

用户指出产品测试停在执行审批，但 Issue33 看不到卡点。正式 API 核到 Run c41e804…仍 e2e_execute/seq5/running；两项审批已 accept，随后新审批4df8e6d…待处理。原通道只配置 Agent 回合完成回写，原生审批发生在回合中，因而不会及时生成 Issue 通知。14条旅程/31项覆盖是已写测试计划，固定tests/独立QA/最终报告尚未进入，不据计划宣称产品完成。

按本次明确要求，通过8794隔离平台正式 Hook 补充准确状态到 https://github.com/big91987/model-relay/issues/33#issuecomment-6074170155 ，GitHub正式 API 作者 big91987-agent-platform-bot[bot]、Bot；仅补报当前卡点，不代产品验证。评论未经核验的时间标签已纠正，保留去重marker，原创建04:18:38Z/编辑04:20:57Z均按外部回执留证。

通用修复落维护源：既有 Agent 回写通道携带 execution approval requested/resolved；依实际审批ID去重，待办附处理会话，结果区分批准/拒绝/失效；显式节点规则优先，允许独立结果通知。旧已处理审批不由隐式通道回灌；发送前在GitHub查重及App令牌获取之后重核实际决定，已处理待办skipped；发送未知只读核查。原生输出、权限与冻结图不改写。AGENTS及标准说明同步，后续项目复用相同运行能力，无产品阶段硬编码。

8793同时有产品测试和研发在途，最近研发已到tests28；当前不强行停止固定门禁。源码验证与真实隔离回写不等于8793完成升级，服务与评论凭据切换仍待安全窗口；原定时保持PAUSED。


最终源码标准门禁 `scripts/verify.sh` exit0，Go全包race50.643s；日志108929bytes/SHA c373550391810fd279e5c72ff69414fd453866d65162babef1031c2ccc19186d。隔离真实GitHub生命周期验证TestWorkflowGitHubLiveApprovalLifecycle exit0/4.665s：受控AwaitToolApproval回调、公开管理员decision HTTP200、原Hook/凭据路径真实发送requested与resolved，重复收集后两通知/两marker评论恰各1，评论6074242941和6074243190均独立核到big91987-agent-platform-bot[bot]/Bot。证据 .data/github-app-validation/approval-lifecycle.json，实际测试Run7c41c7a2b767695b5121876aeac569cb；该临时Store不是8793产品Run，也没有真实模型或产品用例。8794补报验证服务已正常停止。

AGENTS标准和安装说明已同步；已有软件交付/product-e2e模板配置回复通道，可在同维护源运行升级后复用审批通知，不为各仓新建审批配置。当前8793仍旧服务、在途源图/原权限/模型及账号不变，标准运行升级和8793实际审批通知尚待安全窗口，不能把隔离验证称为主服务已上线。


### 2026-10-09 频繁审批：浏览器运行环境与授权粒度诊断（未修复）

用户要求正常测试尽量自动化，不反复提示审批。正式API核到同一e2e_execute5输入20889连续3条命令审批：Playwright open（accept）、goto（accept）、run-code读取独立实例登录密码并登录（pending）。冻结Agent为Codex/gpt-6.1-sol、workspace-write、network_access=true、allow_elevation=true；不是网络授权未保存或模型选择错误。这三条均由原生commandExecution/requestApproval进入平台，不能把业务测试步骤当作权限扩大三次。

实现事实：nativeApprovalPolicy的allow_elevation仅允许sandbox_approval/request_permissions请求；nativePermissionApproval和公开审批接口只返回单次accept/decline/cancel，未支持原生proposedExecpolicyAmendment的持续规则选择。turnSandbox仍workspaceWrite/networkAccess，不能将“允许提权”理解为已授予宿主自由执行。已安装Playwright CLI registry源码在macOS默认使用用户Library/Caches（支持PLAYWRIGHT_DAEMON_SESSION_DIR覆盖）；本Run命令只移了npm_config_cache，未同时隔离浏览器daemon/session缓存。CLI登录与导航继续走提权命令，批准前一条不会批准下一条。缓存导致初次越界及单次授权导致后续重复已具证据；仅迁移缓存能否解决Chromium启动/本机IPC限制尚未实测，不能宣称已定位全部原生沙箱失败。

排查验证：TestNativePermissionDecisions/TestNetworkPolicyKeepsWorkspaceSandbox实际exit0/0.640s，确认现有逐请求策略，而非自动浏览器授权修复。未批准当前申请、改冻结权限、改HOME、绕过沙箱或执行任何产品登录/provider请求。

优先整改归属【Harness修复】：复用已有浏览器工具及锁定运行时，优先验证任务独立的daemon/cache和标准安装准备；若原生沙箱仍不能启动/连接浏览器，则补既有受信browser工具对动态独立服务的支持，由明确任务范围授权承载连续网页操作，不能用全宿主无沙箱或自动accept任意shell替代。现有browser MCP只app/静态原型不足以承担本Run，缺口保持Open。验收必须经真实原生入口连续打开/导航/登录/浏览器操作无需重复越权申请，同时跨任务隔离、停止清理与超范围拒绝成立；异常提权才保留审批。权限与工具新实现须进入唯一维护源和标准安装升级，并让Pipeline原生Agent复验；当前仅诊断及整改方向，未修改Runner执行行为。


### 2026-10-09 原生自动审核：频繁浏览器审批修复

- 用户要求修复 Issue33 产品测试中每条浏览器命令都需要管理员审批。根因是平台线程启动/恢复与 turn 启动硬编码 `approvalsReviewer=user`；`allow_elevation` 只允许申请，一次 accept 不授予后续命令。
- 唯一维护源增加 Agent `approvals_reviewer=user|auto_review`，独立 Agent/节点共用表单、保存校验、执行协议及标准安装器 `--agent-approvals-reviewer`。旧配置默认 user；省略升级参数保留原值。工作区沙箱、联网与允许申请提权仍分别生效；不全盘放行、不改变原生审核策略。
- 管理员正式会话权限 API 可在轮次之间显式选择自动审核，原线程、模型、指令、工具与工作目录保持。运行中返回409，普通用户403；Apply Agent 当前权限可以撤销。只读自动审核禁止提权，旧 prepared 原生目录存在自定义审核策略时明确拒绝恢复并保留现场。
- 真实隔离 Harness 回归：Codex/gpt-6.1-sol 原生 Agent 操作 HTML fixture，Chromium 打开/导航/虚拟登录三项均产生实际截图/结果，4次实际HTTP访问；原生三次 `item/autoApprovalReview` 均 approved，平台人工审批回调0，250.96秒通过。私有 evidence 在临时目录 `platform-reviewer-browser-3124073872`；不是 Model Relay 产品验收，不使用真实凭据/provider。
- 完整 `scripts/verify.sh` exit0，Go race53.224秒、安装器78项、前端/浏览器与其余标准门禁通过；独立复审发现并关闭只读边界、旧 prepared 策略两项问题，无剩余Critical/Important。全量日志 `/private/tmp/agent-platform-auto-review-source-verify.log`；首次 native fixture 因未按正式prepare初始化失败已保留，纠正后实际回归通过，不改写为原故障。
- 运行环境应用与当前 E2E 接续尚待安全升级回执；不把源码及隔离回归绿当8793已生效或产品全旅程通过。当前实际任务保持唯一Run，不创建替代测试任务。

- 补充实际兼容验证：原生自动审核会连同 MCP `prompt` 一起审核。第一次真实确认工具回归64.91秒失败（工具写入2次、人工回调0），未掩盖历史。最小修复为禁止 auto_review 与显式每次确认工具混用，含注册配置、冻结 NativeConfig 和旧 prepared 目录；第二次真实回归28.51秒通过，冲突在执行前拒绝，工具副作用计数保持1。证据为 `/private/tmp/agent-platform-auto-review-mcp-confirm.log` 和 `-r2.log`。
- Apply 权限兼容回归先红后绿：共享 Agent 改自动审核时，旧会话若仍有原生 prompt 工具，API 在保存前拒绝，Snapshot 保持不变。独立复审关闭该 Important，无剩余 Critical/Important。完整第二轮 scripts/verify.sh exit0（Go race54.337秒、安装器78项）；最终 Apply 修复后 Go vet/race/build再次 exit0，race54.623秒，定向权限回归1.059秒。运行环境应用仍待正式回执。

- 正式应用：维护源 `2145fb3` 已提交并推送开发分支。等待主线固定 tests38 自然结束进入 development39 后，Owner 正式 stop 两个 Agent，旧服务27655排空；备份后8793由80613运行新二进制，SHA256 `22395b596f0324dccbb8ba279f184ac183120815fbe990e2961c510f1cec2d87`。冻结步骤、图、Connector、参数、工作目录和预算重核不变，8788/产品5545均未操作；升级保护脚本遇到省略空 parameters 字段停止，修正比较后继续，未手改数据。
- 管理员正式 PATCH 将唯一 E2E 会话 `4ccfaaa83ab911d6293de0acc293f78c` 改为 auto_review，network/elevation原true保持。旧待审批由正式 stop 取消。原安装器 --upgrade --agent-approvals-reviewer auto_review 成功更新同一产品测试工作流3155ddd1206d4ebc19da94e5b489091f，四个未来 Agent 节点均自动审核；通知凭据通过标准 --notification-token-env AGENT_PLATFORM_GITHUB_BOT 更新未来模板。第一次安装缺 PLATFORM_ADMIN_PASSWORD 在任何安装写入前拒绝，原日志保留，补同一管理员环境后成功。当前旧Run冻结通知配置及后续节点权限不静默改写，当前会话显式修改是独立权限选择。
- Owner正式resume：E2E原seq5/Run c41e8047001414b96d327c39fb9c1eeb 与主线seq39/Run b7f93c24bc1ea4d0fe182f12aa7622ae 同 conversation/native thread 接续，主线权限仍 user/network=false/elevation=false。实际部署共享表单字节同源；E2E接续后2次原生自动审核 source=agent/status=approved/risk=medium/userAuthorization=high，2个目标命令 exit0/completed，人工待审批0。原生 guardianWarning 是 approved 通知，不是审核失败或降级。Pipeline自执行原浏览器会话核验和独立实例登录；协调者未操作产品或调用provider，不冒称完整旅程通过。
- 私有正式回执目录 `.data/fresh-8793/check-20261009-auto-review/` 含升级、安装、权限、原生连续性和审核退出状态。当前E2E仍执行中；旧Run后续节点保留冻结权限，新任务获模板选择。产品测试/真实供应商结果、剩余旅程及新缺陷须看本Run原生报告，未因平台回归或两次批准记为全部Pass。

### 2026-10-09 执行权限作为平台产品配置

用户要求权限策略有可见、可保存、能影响真实执行的产品入口，而不是协调者逐Run救援。现有独立Agent编辑与Pipeline节点配置共用执行权限表单，本轮补齐会话侧栏“执行权限”：管理员正式PATCH保存联网、允许申请提权和人工/自动审核；普通用户只读，运行/排队/停止处理中/关闭时锁定；保存后显示实际值，刷新保留草稿，可应用独立Agent当前权限或恢复节点冻结权限。工作流会话从关联Run页停止/接续，明确只改当前节点、后续节点保持启动时设置。

真实Chromium隔离页面回归先红（缺少权限入口）→4项全绿，覆盖保存/重新打开、自动刷新保留选择、节点恢复、普通用户只读及运行锁定；原API权限隔离/原生执行回归沿用已验证实现，未扩大后端权限。独立复审发现并关闭旧PATCH在A→B→A导航后清空新草稿/解除新保存锁的Important，真实app.js行为回归先红后绿，37项Node通过。标准scripts/verify.sh exit0，Go race55.052秒、安装器78项及全部前端/浏览器门禁通过；当前小范围Run跳转说明随后单独Node/Chromium复验及最终构建，不重跑未变产品用例。没有产品验证或真实供应商调用。运行环境应用尚待安全升级回执。

用户认可Pipeline的Agent节点配置，也偏好提交Issue/启动API时按任务控制。已评估现有WorkflowStart.parameters与GitHub入站：可以复用每Run参数传递固定的“权限审核模式”，不建通用共享变量框架。Issue使用专用结构化选择，启动API/网页使用同一含义；平台在启动时核验调用者、Pipeline已允许的执行权限及工具确认冲突，再记录本次实际选择并作用于相关原生节点。Issue不应授权超出管理员已配置的网络/文件/工具权限；原有“自主推进”控制业务确认，不能借它跳过权限审批。GitHub现入口已要求repo owner actor/triggering actor与原Issue作者，继续保留。不存在字段时沿用节点默认，旧Run不追溯改变。任务级Issue/API开关本轮仅完成评估、尚未实现或实测，不能写成已交付；后续需覆盖Issue→唯一Run→实际原生审核、拒绝/恢复、去重及跨Run隔离。

- 本轮正式应用完成：源码 fe7f69c 已推送开发分支；最终37项Node/4项Chromium通过并构建。8793从80613升级为99555，新binary SHA256 a8e8290e36179e77611cd5b5bc119cba43e775fb64282185e565373d5d7edf6d；实际 /agent-config.js 与 /app.js 均逐字节同源。Owner正式stop/backup/restart/resume，E2E原seq7/e2e_review/conversation125c0df7668d689b2fe8c08ed62e988c及主线seq41/development/conversation866d02e43efaf2249e2101b237416692接续。冻结图、步骤、Connector、参数、预算和原权限不变，未调用本轮权限PATCH或代做产品测试。私有回执目录 .data/fresh-8793/check-20261009-permissions-ui/；未使用的临时平台浏览器Cookie已删除。Issue/API任务级开关仍是上述已评估待实现范围，不与已部署UI混记为完成。

- 实际产品界面闭环补证：部署后旧E2E QA7仍按冻结user模式产生审批e6e367…，原Issue通知已真实发送（等待/处理/新等待可在Run通知记录核对）。本轮使用用户已授权的通用UI入口：Run页“停止当前运行”→会话侧栏“执行权限”→选择auto_review并“保存权限”→重开仍auto_review→Run页带明确不重放说明“继续原节点”。此次是界面发起正式权限PATCH，显式只修改QA会话审批方式；不是部署自动修改旧Run。原native thread一致、network/elevation保持true，后续原生审核approved/risk medium/authorization high，人工待审批0；业务脚本/供应商和产品结论仍归Pipeline。真实界面截图 .data/fresh-8793/check-20261009-permissions-ui/platform-execution-permissions.jpg 不含产品秘密。浏览器保留该配置页供用户查看；任务级Issue/API开关仍待正式实现。


### 2026-10-09 测试计划的独立文档评审与核心限额覆盖

用户指出真实产品测试不能只列旅程数，RPM/TPM等核心能力应有可执行计划并基于PRD/AC/设计由独立节点评审。本轮只读核验：现有e2e_plan4已经规划J05/J09/T09，含RPM/TPM/并发零拒绝，但没有充分展开非零阈值、窗口/恢复、计量和共享隔离的用例；e2e_plan直接交e2e_execute，原planning-checks是规划者自查，缺执行前独立计划评审。不是完全未提限流，也不是已验证。独立QA现有报告明确只有零次数配额子集通过，RPM/TPM等Not Run；正式tests6仍exit1/Incomplete-No-Go，当前QA7尚未完成最终交接。

通用维护源product-e2e增加独立e2e_plan_review，依据实际权威文档核需求—用例—预期—证据，READY才交执行、NOT READY回规划；执行实质变更经plan_review回评审。原验收Skill复用，不增加平台状态或产品规则引擎，不代具体产品断言。强化规划/执行职责与AGENTS/README标准，零次数429不替RPM/TPM，缺材料/规则/可信计量/安全调用条件必须Blocked，不能自增预算或删除用户目标。新增安装路由及旧版同ID升级/重复无变更回归旧红→绿。标准完整回归和独立复审正在进行；标准应用及真实新计划评审回路尚未证明。

原Run c41e8047001414b96d327c39fb9c1eeb仍冻结旧图，不新增或回填阶段。Owner正式messages提交request_id model-relay:product-e2e:plan-review-rpm-tpm:20261009，message26490/duplicatefalse/statusrunning到QA7同conversation125c0df7668d689b2fe8c08ed62e988c，要求独立补充计划审查并在原Issue反馈缺口与影响，报告未验范围及整改交接；接收不等于已处理或限流已验。定时仍暂停、产品实现与实际模型调用由Pipeline负责。

- 源码验证完成：标准scripts/verify.sh exit0，平台Go race55.573秒、模板安装79项及前端/浏览器/Python/vet/build通过；独立复审无Critical/Important/Minor，另跑安装17项/模板契约3项通过。证明路由与安装接口，不证明新节点实际评审质量或RPM/TPM产品结果。

- 正式应用完成：维护源684f302已推送开发分支。Owner正式stop QA7后，通过原product-e2e/install-args.json与--upgrade标准安装，同工作流3155ddd1206d4ebc19da94e5b489091f升revision3，新增e2e_plan_review；新节点Codex/gpt-6.1-sol/auto_review核验。逐值核原Run冻结definition/connectors/parameters/workspace/max、前6步及当前节点身份保持，主研发工作流8f497…完全不变；未重建任务、改权限、启动替代Run或操作产品/供应商。原QA7同conversation正式resume接续，人工待审批0。私有回执check-20261009-plan-review/application.json及标准install.log。原生message26785已明确“正式承接RPM/TPM计划缺口，补做独立文档评审”；这只是实际承接，不是补充评审完成或RPM/TPM通过。新节点首次真实READY/NOT READY返工回路尚未运行，仍须由后续Pipeline产物验证；原Run不回填新节点历史。


### 2026-10-09 测试报告GitHub可见、Dashboard与正式研发交接

用户要求将报告形成研发Issue交Pipeline整改，并质疑报告仅本机Markdown、缺可视化。原测试Run c41e804…于seq9完成：No-Go，独立RPM/TPM计划评审Rejected/PLAN-01–12，确认产品缺陷0，安全控件候选因服务端默认线索降为Needs Evidence。用户此次明确委托研发复现/整改，允许对待验候选和项目测试缺口进行有证据的处理，不将漏测/模型缺名/预算未知写成已确认产品故障。

原design-v0.2.0 GitHub资产617014535和SHA256SUMS恢复，ZIP摘要5f51d35fdc15ed7214749f9413879b4238097f1d6b6bbe2cc0d508d8e103cef7匹配；保留19份原文逐字节及原manifest。合并Pipeline原报告和首失败/截图/脚本/诊断白名单61文件，生成新交接包product-test33-handoff-r1，摘要a13fb6b9089cefbb9d2b6f9a333830e0ee6866bd7ff45d5ba881da97e9a66a49，710785bytes，经现有材料验证器检查。新包版本不冒充新PRD或改写冻结设计，凭据不入包。GitHub Release test-report-33-20261009已发布原始report.md正文及资产624807738，远端bytes/digest匹配；该Release是报告页面，不是产品版本上线。原Issue33评论6080204048给出真实报告/证据链接及发布来源。

创建唯一Issue34【产品修复】复核真实模型调用安全路径并补齐RPM/TPM验收缺口；正式入站37925548923首次外部接单评论POST非零，保留日志，底层HTTP/网络原因未定位。最初管理员by-request404被过早解释为无Run，后通过正式全Run查询纠正：唯一Run637705186406b80a2ee4afaf0d6b2167已在11:44:24Z创建，prepare1真实exit0下载/校验61文件、issue2关联34、intake3正式交development4/conversation909a8fe39901e725c379dc16b994ddaf，main/受测HEAD3f9cee。没有重复创建或dispatch；核已有副作用/无通知marker后对原Actions一次有界failed重试，第二次仍failure，通知恢复尚未闭合，禁止循环盲试，不能称入站Actions全绿。原Hooks已实际回写阶段承接。

报告展示返工采用平台已有completed-return正式入口，不手改历史或新建测试Run。首次POST误用reason得到HTTP400无副作用；读正式summary契约后return9→report10/conversation4ce424ef532e1108da38bc20092e3ff7，seq1–9保留。原冻结报告角色继承user模式导致静态Chromium检查待审批50d4b…；正式stop/权限API选择auto_review/resume，network/elevation原true保持，不扩大权限。Pipeline自己生成Dashboard、GitHub-ready正文及48项白名单ZIP，并以Chromium145在1280/390验证18行结果/12项整改/6份原文、筛选搜索展开、30个相对链接、零外部网络/页面错误/横溢，保存截图。包扫描误匹配自身规则的原失败保留，最小修正后仅package-only，不伪造产品绿或重跑供应商。协调者逐文件bytes/SHA/ZIP对照48项一致；原report与189历史文件保持，具体产品判断仍由Pipeline。Dashboard摘要7228f5ec97199dc909b66228473100132b5437bae28c2145a134b9b7ea63f363，ZIP2623618bytes/SHAe4d8670039475d5dee3afd0520c933a1ccc421854f89dcd1faadd2e6a826853f。GitHub仓为private，保持私有Release附件，不擅自创建公开Pages。Dashboard上传及远端核验进行中。

维护源e2e_report/README/AGENTS补齐完整报告、HTML看板、白名单、浏览器展示检查、授权GitHub发布与实际回执职责；现有模板没有无人值守上传/托管Connector，此轮报告内容由Pipeline产出、发布由获授权Builder完成，不能称自动发布流水线已交付。模板相关79回归/6.478秒通过，源码标准应用与提交还待后续记录；定时仍暂停，Platform源只开发分支，不建源PR或合main。

- 正式交付补证：报告展示返工report10→done11已实际完成，原seq1–9及189历史不变，产品结论仍No-Go。GitHub已上传6个资产，Dashboard资产624829907/154947bytes远端digest与7228f5ec…一致，48证据ZIP资产624829904/2623618bytes/digest e4d86700…一致；旧61文件材料资产未覆盖。原Issue33同评论6080204048更新为真实报告、HTML下载、ZIP和研发Run链接，不新刷评论，保持private可见范围。没有在线Dashboard站点；浏览器展示检查不改变产品验收或真实供应商Not Run。
- 通用报告交付职责fea7565已提交推送开发分支，79模板回归/独立源码契约复审无Critical/Important。报告Run完成后原安装器--upgrade成功应用同产品测试工作流revision3→4；完整旧Run逐值保持、主研发图不变，未来报告节点明确HTML/白名单/GitHub回执并保持auto_review。私有回执check-20261009-report-handoff/{published-assets,presentation-audit,standard-upgrade}.json。当前发布由用户授权Builder完成，模板未实现无人值守上传/托管Connector，不伪称自动发布能力已交付。
- 最终实时研发核验：Issue34唯一Run637705…仍development4 running，报告包/原设计已正式承接，尚无新候选make verify/QA/PR/合并/部署。Actions37925548923第二次尝试仍failure，原GH评论POST底层原因未解决，不再盲试；已有正式Hook记录intake直接development及材料校验。任务承接、材料完整性和直达研发已证，外部接单通知可靠性未闭合，后续须最小诊断与通用维护源修复。


### 2026-10-09 按小时核验：两候选已发布，任务关联不等于完成

正式 API 核到 Issue34/Run637705186406b80a2ee4afaf0d6b2167 已14步 completed，PR36 head8917f942cb38af2f56aa93914ab62e763bf9cc6e；Issue32/Runb7f93c24bc1ea4d0fe182f12aa7622ae 已61步 completed，PR35 head794e493e69a91f7aae90878e1c27be551e4a5883。不继续引用 development4/report58 为现状，不新增任务或重放产品执行。PR36 是受控计量/安全回归资产，不是生产算法缺陷修复，确认生产缺陷仍0；真实供应商、两屏非零限额与费用实扣仍 Blocked/Not Run，原Issue33 No-Go保留，历史预算未知不清零。

维护者通过正式 output API 分页读 tests9/56 至 EOF：各499596/729568字节、truncated=false、exit0；Connector日志与runner原档分别核验。PR36 runner499499字节/SHA c93e9625…、PR35 runner729471字节/SHA5f9b70c0…均与state匹配。用两个精确提交的git对象独立重算154件源码，分别2dfe14a3c305c13621f23b7dd287a8313b7e4f39ab64b8281d283450d125d8ed、dc667be6f0f5db892748ac8a214d799264c2d451004da8c80a5e06e6c38f338c，与固定测试/QA候选同版，冷起点/source前后/清理均有真实证据；两个工作树干净。QA10仅M01受控回归Go，QA57仅Issue32受控旅程Go with known issues，不把独立材料审查当QA亲自完整浏览器实操。合并及新部署未发生，5545仍正式3f9cee/schema3健康。

发现通用发布缺陷：github.pull_request Connector 根据Issue关联无条件添加 Closes，PR36因此会误关仍有未验项的Issue34。最小设计只默认Refs关联，正文显式关闭声明保持，不新增开关/任务状态机。隔离回归先编译字段纠错再真实旧红→新绿，独立源码复审无Critical/Important/Minor，四项PR/丢回执回归通过。维护源完整scripts/verify.sh已真实exit0（Go race及前端/浏览器/模板/安装/vet/build完整门禁，日志/private/tmp/agent-platform-pr-association-verify.log），正式运行应用与原PR支持入口复验待后续记录；不把源码绿冒充8793已应用。外部PR35/36分类/发布事实已纠正，默认关闭声明已移除，原Agent报告及receipt不手改。私有核验 check-20261009-hourly-1439。

通知故障证据纠正：37925548923 attempt1为评论POST失败，attempt2实际上在读取Issue GET即失败，不是再次评论POST失败。两轮只保留通用subprocess非零且stderr未展示，底层HTTP/传输原因未知；不得据此宣称权限或网络已定位、通知恢复或入站Actions全绿。不再盲重试。任务接单与通知结果分开，已有唯一研发任务和Hooks成功事实保持；下一步需安全诊断能力而非补造原失败细节。无人值守报告发布仍未交付，不趁本轮新增通用发布框架。

- 通用修复正式应用：620b523已提交并推开发分支，完整scripts/verify.sh exit0/Go race54.715s。8793无在途Run/会话后备份、替换为标准构建二进制并按既有命令启动，PID99555→99080，二进制SHA73d537a449d6b236da5168a29466b2fcdf7262e587d0e8293ec2dbe1956ff8f7；重启前后全部Run逐值一致。仅为必要PR交付补正使用原completed-return14→pr15→done16，PR36确实更新同编号/同head并使用Refs，不重跑tests/publish/provider；前14步及冻结图、Connector、参数、预算、工作区保持。现场PR分类与发布事实保留正确说明。原Issue34真实未验仍未关闭，Issue33 No-Go不变；通知故障及自动报告发布缺口未因该修复关闭。回执check-20261009-hourly-1439/runtime-upgrade.json、pr-return-reverification.json。
