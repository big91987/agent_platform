# 节点独立配置实施计划

目标：按用户已批准的节点独立配置方案执行，普通实现选择自主完成。

设计：[节点独立配置](../02-architecture/workflow-node-agent.md)。维护分支延续既有平台工作，不合并源 main；运行实例升级前备份并检查无在途任务，旧停止 Run 保留。

- [x] 后端：节点 agent 配置校验、冻结执行及会话创建，复用现有调度；测试无外部 Agent、配置隔离及旧定义兼容。
- [x] 授权与展示：工作流节点会话的读写/接续/恢复授权，普通工作流与 Run 响应裁剪管理配置；测试撤权、跨用户及敏感字段。
- [x] 编辑器：节点直接设置全部执行字段，旧引用保存转独立；测试草稿、迁移隔离及保存请求。
- [x] 模板安装：阶段配置内嵌、原 manifest 升级与漂移保护，保留旧 Agent；测试新装无额外 Agent、幂等升级。
- [x] 集成交付：真实节点执行、页面操作和升级验证；更新手册和证据，独立评审后提交现有草稿 PR。

重点失败场景：两个来源同时设置、伪造已解析工具、撤权后继续、未创建会话却伪造 agent_id 为空、普通用户读取原生配置、模板升级覆盖节点人工修改。

验收结果见 [验证记录](workflows-verification.md#节点独立配置与会话授权2026-10-07)。本次完成节点独立配置范围，不将独立节点小任务外推为完整产品端到端验收。

## 配置一致性与研发流程回归（2026-10-07，新分支）

状态：实施中。分支 `codex/workflow-node-config-ui`，封版基线 `d7bd602`，源 PR #5 不合并 main。以下用户确认取代上阶段对独立 Session Prompt 的设计。按用户最新指令改为在8793使用全新数据目录、新装编排和新的测试仓Pipeline；8788保持不动。

### 已确认的产品约定

- 独立智能体和编排节点使用同一组执行设置：名称、执行器、模型、角色指令、Skills、外部工具及审批、文件/网络权限、原生配置、Hook 与环境。节点独立配置，不引入智能体模板或同步对象。
- 名称能直接编辑，卡片和详情展示名称、执行器和模型；每个可编辑设置都必须贯通表单、保存、重载和实际执行。授权和启用由各自所属的智能体/编排管理。
- 取消单独的 Session Prompt/工作说明。具体任务、资料、修正和上游交接都通过 User Input 提供；长期角色职责属于角色指令。原生执行器读取挂载目录中的 AGENTS.md，平台不重复拼入其内容。
- 一次 Run 共享一个启动时确定且运行中不变的工作区。网页/API 可指定已有路径；标准研发入口准备仓库后使用同一能力。暂不建设通用变量、表达式或变量管理页面。
- 编排额外配置交接目标/策略、完成方式、等待和有界自动继续。平台生成所需工具使用说明，用户不再维护交接占位符。
- 停止可以恢复；取消是终态，保留历史与文件，释放工作区占用。两个旧产品 Run 经正式接口取消，不手改数据库。

### Task 1：统一配置与可编辑展示

- [x] 提取 `web/agent-config.js`，独立页与节点复用表单、读取和环境/工具校验；保留各自名称、授权与启用语义。
- [x] 名称实时更新并保存重载；卡片同时显示执行器和模型；抽屉关闭后画布利用全宽，打开覆盖画布，长表单可滚动。
- [x] 有意义的表单回归：两入口读取结果一致、无效 JSON 不丢失、不可用工具保留、未修改关闭不产生脏状态。
- [x] 浏览器验证独立页和节点的修改→保存→重载、长表单和抽屉关闭。
- [ ] 窄屏复验：本轮浏览器 viewport override 返回成功但实际仍为1280px，不将此记作760px通过。

### Task 2：任务输入与共享工作区

- [x] 在既有版本字段上增加新输入语义；移除新配置中的节点 Session Prompt，不改写冻结旧 Run。老配置中有用户文字时不可静默丢弃，升级明确保留其语义或报告需要处理的具体字段。
- [x] `workflow_context.go` 从角色之外的 Run 输入、用户修正、上游 handoff 和 Connector 回执组装实际 User Input；自动附上交接/完成工具规则，无用户占位符。
- [x] 默认新编排、预览与模板安装使用新语义。研发模板的长期阶段职责进入角色指令，具体产品要求留在入口输入/资料中。
- [x] 覆盖冻结兼容、实际输入、合法工具、同一 Run 工作区一致、不同 Run 隔离/占用、网页/API 启动路径；更新架构和用户手册。

### Task 3：正式取消与旧运行收尾

- [x] 引擎、HTTP、SDK 和 UI 增加取消；处理权限、过期 seq、执行未退出、重复取消、重启和禁止接续。
- [x] 保留所有历史和产物，取消后可合法创建使用该工作区的新 Run；已取消会话不能绕过 Run 接收新输入。
- [x] 备份运行状态后经正式接口取消 `9e6d05f409675e0ef65c6486691462df` 和 `2f589073fb909da91a3e6f76a8ba5430`；记录结果与文件保留情况。

### Task 4：标准交付与平台验收

- [x] Go 格式/静态检查及相关行为测试、前端测试、模板新装/原 manifest 升级/漂移保护测试。
- [x] 独立代码评审、修复发现、提交并推送开发分支。实现提交 `1830f81`；用户明确本阶段只在分支上开发，Platform 不需要新建 PR，也不合并分支。测试仓的 PR/合并/部署仍按原授权和真实流程验证。
- [x] 无在途任务时备份并升级 8792 二进制，通过标准安装器更新原 manifest；验证幂等和冻结历史，不把临时诊断脚本作为交付机制。
- [x] 真实 Codex/gpt-6.1-sol 验证配置生效、工作区 AGENTS.md、User Input、handoff、等待/接续和取消；更新安装/升级及操作手册。

### Task 5：model-relay 完整研发回归

- [x] 核对测试仓 Issue/现有产物；为取消后的重新启动明确工作范围和引用证据，不伪造迁移或丢弃旧工作。
- [ ] 通过真实受支持入口启动最新编排，使用冻结 design-v0.2.0 产品材料；实际完成研发→固定测试→独立 QA，失败必须返工。
- [ ] 验证 PR 与证据绑定，按已授予的测试仓权限合并/部署，检查 5545 的版本和实际用户流程。
- [ ] 记录仓库提交、Run、测试命令、浏览器结果、失败恢复及未验证边界；仅以上述实际证据判定整条流程是否完成。

执行原则：按任务推进并持续维护此表。Platform 当前在 `codex/workflow-node-config-ui` 直接开发、提交和推送，不自行新建 PR；后续源仓集成另按用户指令。单元测试、局部探针和平台的小任务通过不等于产品研发链路通过；源平台交付和测试仓产品交付分别留证。没有证据的项目保持未完成。

当前完整回归入口：[model-relay #24](https://github.com/big91987/model-relay/issues/24)。两个旧 Run 已取消、文件保留；新任务从标准 main checkout 开始，不将旧未验收代码当作已交付基线。真实证据见[配置一致性验收](workflows-verification.md#配置一致性输入与取消2026-10-07)。

## 交接模式与固定输出实施计划（2026-10-07）

> 执行：按superpowers:executing-plans在当前会话逐项实现；沿用用户对普通实现选择和直接推进的授权。设计见[节点配置](../02-architecture/workflow-node-agent.md#交接模式与固定输出2026-10-07)。不重复要求审批已经明确的两种模式及Schema方案。

目标：界面可选handoff/固定流转，固定程序得到经Schema校验的类型化输入。Go使用已有jsonschema-go依赖，前端沿用图模型/抽屉；运行快照、单一结果及原有调度不变。只在当前开发分支交付。

评审重点：旧冻结混合图；无效Schema草稿保存/关闭；切换多个目标的意外丢线；嵌套类型到命令stdin不失真；不合规结果不能先存后验。

- [x] Task 1 后端契约：workflow_completion.go负责Schema、模式和工具参数；WorkflowNode增加exit_mode/completion_schema/completion_instructions；NodeResult.inputs支持JSON类型。先加真实Store/MCP测试：不合规结果拒绝且不推进、合法结果原类型到Connector、冻结旧图和模式拒绝，再实现并跑相关Go测试。
- [x] Task 2 节点表单：workflow-model.js负责模式切换、单目标约束；workflows.js提供交接模式选择及两种配置页，固定Schema/说明/目标可保存重载预览，错误草稿保持。增加模型/表单行为测试，禁止连接器/连线对话框旁路混用模式。
- [x] Task 3 模板交付：install.py/三个模板升级显式模式，协作夹具不再混合；旧manifest漂移保护/冻结Run保持，更新手册与输入文档，验证标准新装升级。
- [ ] Task 4 真验与发布：独立代码复审、完整Go/vet/race及前端/Python回归；真实浏览器设置Schema和说明、真实Codex错误修正后固定命令消费、handoff模式切换。当前源码构建部署至8793全新服务，标准新装编排后用新的测试仓Issue验证；不继续迁移旧配置，不碰8788。提交推送分支，维护证据与剩余边界。


交接模式验收进度：后端契约、表单与模板源码已完成。最终 `scripts/verify.sh` exit0（Go race49.667s），随后前端数值草稿保真回归与构建通过（63项Node测试）。8792已升级；真实Codex固定输出Run `b8250145f04f6ce079d3322fcf0e5e4e` 完成，原生事件记录额外字段拒绝、修正接受，命令消费布尔/数字/数组/对象且exit0。原协作安装标准升级为revision5并幂等重装，历史Run不变。原研发模板因Run仍waiting由安装器正常阻止升级，保持revision8冻结；等待任务结束后再走原manifest升级，不绕过保护。完整model-relay #24研发验收仍未完成。


2026-10-07最新部署决定：用户明确要求新端口、全新服务和新Pipeline，取代继续升级旧流程的方案。8793全新数据与独立工作区经标准安装器安装当前模板，编排 `8f497228b60a8ea46b7d37be59ebe578` revision1/context2，六个Agent均为Codex/gpt-6.1-sol和显式handoff。测试仓标准入口切换至新服务，[Issue25](https://github.com/big91987/model-relay/issues/25) 经真实 [Actions37643773819](https://github.com/big91987/model-relay/actions/runs/37643773819) 创建唯一 [Run4cd62ced6a4f98f50e3cca6f7596eb06](http://127.0.0.1:8793/workflow-runs/4cd62ced6a4f98f50e3cca6f7596eb06)。准备与Issue节点已完成，19文件设计包摘要核验通过，原生intake已完成handoff，seq4需求节点正在执行。完整研发、QA、合并与部署仍未完成，保持Task5未完成。


典型场景验证（8793）：正常阶段handoff、GitHub入口重复投递去重、澄清等待、等待态页面停止/原线程恢复、独立校验返工、完成后人工回退、Schema拒绝纠正、固定命令exit1自动返工/exit0复验、类型化消费和跨用户读取隔离已实际验证。两条专用验收Run完成，Issue26关闭，主Issue25仍在设计。详细矩阵见[验证记录](workflows-verification.md#8793典型场景实测2026-10-07)。下一批继续主产品make verify/独立QA/PR发布，并补进程执行中中断、服务重启与外部失败恢复；页面和跳转优化安排在流程验证之后。


半小时检查2026-10-08 00:18：主Pipeline已从设计正式推进到seq6研发，局部Go回归已实际执行；Task5完整产品回归仍未完成。商业对齐补充经原会话User Input接收，详细进展和缺口在统一验证记录维护。

半小时检查2026-10-08 00:48：seq6研发持续活动，三视口控制台旅程已接入固定测试源码入口，计费局部Go回归通过；真实HTTP监听受原生沙箱限制，等待既有宿主tests Connector执行完整make verify。商业补充message1548仍queued，接收不代表处理完成；无新PR/QA/发布。Task5保持未完成，继续正常交接与证据核验，不重建任务或绕过测试。

半小时检查2026-10-08 01:18：seq6仍在实现与局部修复；实际make verify在原生环境exit2，health-history监听禁止，日志已保留。go vet及新增治理/邀请限速定向回归实际通过；完整宿主门禁、UI、独立QA和发布未执行。商业补充仍queued，不重投输入。Task5未完成，后续核验正式tests交接和更新后的研发任务记录。

半小时检查2026-10-08 01:48：新增资源/成员范围、迁移守恒及代际截止的局部命令通过；核对并分别记录产品权限修复与测试Revision/身份入口修正，保留失败历史。研发仍活跃，未交固定宿主tests；Task5的完整门禁、QA及发布仍未完成，商业补充仍queued，不重建任务。

半小时检查2026-10-08 02:18：研发候选和AC待验报告已形成，商业输入message1548已在原会话送达/处理中；待输入交接拒绝→正常结束turn→原会话接续路径实际验证。补充检查扩大至127项Node和指定race，新增前端失败/修正历史保留；完整宿主tests尚未接受，最终指纹及商业对照文档待更新核验，Task5仍未完成。

半小时检查2026-10-08 02:48：正式seq7宿主完整门禁执行，后台HTTP/SSE和真实崩溃恢复等全仓race分项通过；迁移browser驱动目标不符导致exit2，原failed边自动返seq8研发。Agent正修测试驱动，保留历史schema2层并增加生产schema3旅程，局部目标契约/迁移race通过，待完整重跑。商业覆盖/旅程/后续差距已写入原矩阵，Task5仍未完成，QA及发布未开始。

半小时检查2026-10-08 03:18：seq9宿主browser继续发现Key集合包络错配，原failed边返seq10，driver修正及135项Node/指定race回归通过。当前seq11第三次宿主完整门禁running，原迁移/数量/身份/启停断言保留，无放宽权限或生产兼容补丁。完整QA、PR及发布尚未开始，Task5仍未完成。

半小时检查2026-10-08 03:48：生产schema3迁移三视口、旧UI/多上游旅程正式通过，新控制台服务核验失败自动返seq12。前端版本参数与受控列表协议修正、137项Node和指定race通过；当前seq13第四次完整宿主测试running。完整门禁/独立QA/发布未通过，Task5仍未完成；后台guard、历史与原断言保持。

半小时检查2026-10-08 04:18：服务核验修复在宿主UI实测生效，模型driver歧义失败返seq14；精确label匹配及唯一性回归修正、138项Node等通过。当前seq15第五次完整宿主门禁running，原断言和产品配置保留，QA/发布未开始，Task5仍未完成。

半小时检查2026-10-08 04:48：seq15模型旅程越过原失败点，但Key粗阶段超时。seq16只补11动作阶段/安全观测，141项Node及非监听检查通过；当前seq17第六次完整宿主测试running，Key根因未知，不称已修复。完整门禁/QA/发布未完成，Task5保持未完成。

半小时检查2026-10-08 05:18：Key转交误清值根因由产品Agent正式修复，seq19宿主越过Key/调用/详情等原卡点，停在键盘断言。seq20补driver异步准备等待及安全焦点诊断，152项Node等通过，当前仍研发running；完整宿主/独立QA/发布待完成，Task5未完成。

结束前最新复核：seq20正式handoff已接受，seq21第八次宿主make verify running，完整门禁/QA/发布仍未完成。

半小时检查2026-10-08 05:48：seq21完整宿主门禁首次exit0、新控制台三视口通过；独立QA识别旅程覆盖P1/列表状态P2，尚未完整验收。seq22模型容量错误已通过正式resume接回原会话/线程且实际继续执行，当前QA running，待正式整改handoff。Task5仍未完成，PR/合并/部署未开始；真实外部失败恢复证据已留统一验证记录。

半小时检查2026-10-08 06:18：seq22原线程恢复后完成No-Go报告并正式handoff研发，当前seq23 running。新J01–J07连续UI旅程（三视口、独立临时库/真实Go/测试时钟）已接入固定browser入口，旧断言保留；Key列表状态/信息和空租户加载态修复候选、156项Node与指定race等通过。新指纹cb51d80c…完整宿主/独立QA仍待执行，QA22-01/02仍Open；不复用seq21绿作为新候选Pass。Task5未完成，PR/合并/发布未开始。

半小时检查2026-10-08 06:48：seq24当前最终指纹18207762…完整门禁exit2，旧三视口控制台通过但新增J01空态断言失败，正式返seq25。源码及新增in-process回归确认全新安装零租户，driver误期望迁移legacy；Agent修driver/分段诊断，6项Node和定向race通过，未改初始化/业务权限。上一轮init保留legacy判断已更正，当前完整宿主仍待重验，QA22两项Open，Task5及发布未完成。

半小时检查2026-10-08 07:18：seq26证实初态修正生效，continuous J01首次调用502返seq27；实际受控上游缺assistant role的契约红灯复现，fixture修正后定向回归/159项Node通过，生产validator不放宽。当前seq28第十一次宿主完整make verify running，新HTTP持久化及三视口七旅程尚待回执，QA22两项仍Open。Task5/发布未完成，正式返工保持原Run与冻结材料。

2026-10-08早间累计成果复核：seq28新增连续1280/J01–J04实际通过，J05恢复后的费用守恒断言失败，完整make verify仍exit2；seq29仅诊断候选已正式交接，当前seq30完整tests running。不可称暂停恢复根因已修复，QA22两项及Task5仍未关闭，七旅程/独立QA/发布待验。

半小时检查2026-10-08 08:47：seq30诊断确认费用/预留/调用数等守恒，粗J05阶段实际还覆盖后续成员操作，前次“费用断言失败”归因更正。seq31研发running，补成员分段和延迟刷新准备回归，12项driver及PeopleScope定向race通过；只修候选driver等待，不改生产计费/权限。实际原断点及完整宿主/独立QA待验，Task5/QA22/发布均未完成，原failed路由正常。

半小时检查2026-10-08 09:17：seq32新增1280连续J01–J05/J07实际通过，J06读取首账目与累计费用混淆导致exit2，正式返seq33。driver累计读取修正保留fee21并加强成本/Token/原项/价格快照/追加revision断言，165项Node及定向race通过；当前seq34第十四次宿主完整门禁running。原handoff/failed路由正常，QA22/Task5/完整三视口/发布仍未完成，不把新增六旅程分项绿外推全部Go。

半小时检查2026-10-08 09:47：seq34第十四次完整门禁exit0，三个视口的七旅程实际21/21通过；独立QA35核对同指纹50fb764a…和18产物，QA22-02 Closed。QA22-01保留故障/并发覆盖残项，新增QA35-01读回首次失败丢按钮/操作标识P1，生产函数Contract真实exit1复现，待真实Chromium回归。QA No-Go通过原handoff正确返回seq36 development，当前running；Agent修恢复路径和AC08/09/16，不重复任务/手动恢复。当前Harness这条测试成功→QA→返工路径无跳转错误，新平台源码保持WIP；Task5及新PR/合并/部署未完成，旧health/schema2未更新，不能称发布或全部异常流程通过。证据与故障恢复范围见统一验证记录。

半小时检查2026-10-08 10:17：seq36 completed，正式handoff accepted至tests，seq37原完整make verify running。读回恢复修复/unknown去重、169项Node及新12分例与原6顶层Go定向race已实际完成；新增真实HTTP/三视口故障旅程仍待宿主，新127文件指纹3ee2122e…不跨用旧seq34绿。QA35 No-Go仍有效，QA35-01/QA22-01待新完整结果及独立QA关单；既有QA22-02关闭保留。Harness交接/派发无错，未手动重复Run或测试；Task5/新发布未完成，源平台WIP保持。细节、原红灯与新driver前置失败、更明确的旧editor隔离重建证据边界见统一验证记录。

半小时检查2026-10-08 10:47：seq37当前3ee2122e…完整门禁exit0，三视口恢复/双发送、新ActualHTTP资源竞争及21主旅程实测通过；独立QA38关闭QA35-01/QA22-01指定范围并保持QA22-02关闭。因原AC01十页设计实际对照证据未齐，正式No-Go补证返seq39，当前running，无新已执行产品P0/P1。Harness测试成功→QA关旧项→正确返补证正常，Task5及新PR/合并/部署尚未完成。源平台WIP/在途服务保持，H与发布未验边界继续由维护者负责；完整版本/63产物校验及原型对照责任见统一验证记录。

半小时检查2026-10-08 11:17：seq39正式交tests，seq40原完整门禁running。十页当前页面/角色/scope/详情编辑采集和原型映射已接原入口，Key额度/详情可读性候选及173Node/非监听检查通过；新129文件bb30f817…实际截图/完整结果/独立QA仍待验，QA38-01未闭。原handoff/派发正常，无新平台阻断；Task5及新发布未完成，源平台WIP/在途服务保持，具体证据与可读性和设计差异判断边界见统一验证记录。


2026-10-08 11:47～12:12：**测试仓**seq40原完整make verify exit2/101576 bytes；173Node、全仓race/build、原三视口控制台/故障及新增1280七旅程绿，首visual-overview采集断言失败，具体字段待真实安全DOM诊断。QA38-01仍Open/No-Go，旧三项关闭事实保留，不把旧绿或计划截图作当前完成。**Harness**同时耗尽原冻结40次上限，无正式有界恢复入口；维护源新增授权检查原因/目标/有限max_steps的return，保留冻结图/历史/回执/workspace，不自动重放。Go/API与Node红绿、完整scripts/verify.sh exit0后，无在途备份升级8793；标准manifest升级连续两次幂等、同ID，新模板100、原Run40不变。正式CI Token return seq40→41 development HTTP202，有效预算60/冻结40/原40步深比较一致，原生Codex已实际运行定位采集断点。未手改DB/回执/权限/冻结输入，产品实现仍由Pipeline Agent；U1/Task5及产品新PR/合并/发布未完成。源只在开发分支交付，不创建Platform PR/合main；其余未测典型负向不因本次恢复关闭。详证见统一验证记录本轮条目。


2026-10-08 12:18：恢复后seq41 development仍running，尚无已接受handoff。产品visual-boundary-r1只补采集断言前安全DOM/overview GET白名单/准确check留证，原.empty/字段/角色/范围/尺寸/秘密门槛及生产业务保持；最终176Node和静态检查实际exit0，新129文件fb766c0a…仍待原完整宿主诊断/独立QA。空态原因未证明，QA38-01 Open/No-Go，U1/Task5/发布未完成。Harness有效60/冻结40稳定，无新平台阻断，维护源abe0dd6已推送开发分支、无Platform PR/main合并；前轮检查/标准升级证据保持，不重复任务/门禁或盲目放宽。原生事件/检查与归因边界见统一验证记录。


本轮结束核验：seq41已completed，原生handoff事件41905正式accepted=true/target=tests，新seq42第十七次原完整make verify running且connector_dispatched=true，无error/当前结果；未手动追加测试、Run或输入。交接携当前fb766c0a…诊断指纹、真实旧失败、host-retest及QA38原返工依据，明确保留门槛，不能预期诊断回合一定绿。5545本轮health200/schema2/version9851177仍旧部署，Open PR0。后续等待seq42正式完整结果，再按准确安全事实归因；不把正常派发当已修复或通过。


2026-10-08 12:48：seq42完整诊断门禁exit2/103232 bytes，真实overview GET200/scope匹配、字段角色尺寸均通过，唯独合法近7日无调用empty触发非空采集门槛，根因已证为J06跨月后采集缺当前周期活动。seq43产品Agent仅补原生UI真实调用及当前周期费用21/成本7/唯一charge/旧结账完整对象守恒，保留所有原采集/生产门槛，179Node/静态检查局部绿；新129文件31bbf962…正式handoff事件43126 accepted至seq44第十八次原完整tests，当前running。真实十页/全部视口/独立QA仍待验，QA38-01 Open/No-Go，U1/Task5/新发布未完成。Harness有界恢复后测试失败→正确返研发→再次测试实际正常，无新通用故障；底层模型事件无model字段不新增验证结论，旧H边界保留。当前Actions仅Issue入口、health仍旧9851177/schema2，Open PR本轮读取失败未知，具体根因与红绿层级见统一验证记录。


2026-10-08 13:18：seq44当前周期UI活动真实通过，采集到limits-editor后因按钮在heading、旧driver限于content导致timeout，正式返seq45最小修locator owner，所有原门槛/生产业务保持。当前129文件7e0b20dd…seq46第十九次原完整make verify真实exit0/104299 bytes，182Node、全仓race、原三视口故障及21主旅程、新十页三视口全部通过；维护者独立核验新63PNG引用/SHA一致，人工查看两张实际图。已进入seq47独立QA；正在判断模型/租户主要信息与冻结设计是否齐全，当前无正式最终结论，QA38-01仍Open，不以采集绿代设计验收。Harness原失败返工→tests绿→QA正常，无新平台故障；U1/Task5/新PR合并部署未完成，旧5545健康/schema2版本9851177不变，Open PR0，近期Actions本轮读取失败未知。证据与候选/已测/QA在途层级见统一验证记录。


2026-10-08 13:48～14:21：独立QA47正式关闭QA38-01采集缺口，但新QA47-01 P1主要管理摘要缺失使AC01仍Fail/No-Go；原始/确认Token口径QA47-02 P2 Open不单独阻断。正式返seq48摘要候选/局部检查→tests49完整exit2/98848bytes，service-create等待响应超时→seq50锁生命周期候选/187Node局部绿，133文件5b0822a0…源戳/正式handoff一致，14:31核对seq51原完整tests仍running/dispatched，独立QA仍待验。Harness QA返工及failed路由正常，无新通用平台故障；QA绿/新发布不能沿用seq46旧指纹。用户要求最新效果并明确不要替Harness，正式message9179送入同Run，继续原QA/report/publish/pr；已有Deploy Model Relay locally和在线Runner可用，协调者只核验授权合并/正式Actions激活，不代改产品或重复任务。旧5545/schema2/9851177保持，新PR/合并/部署未发生，Task5/U1仍Active，详证见统一验证记录。


部署请求结束核验：message9179由queued→running→completed，原生Agent实际回复已写入原任务/带入下游，seq50正式handoff被接受后进入seq51 tests（connector_dispatched=true/running、无error）。维护者未代替Harness测试、产出PR或启动服务；当前最新候选仍需本轮完整门禁与独立QA，未有可授权激活的新main。旧5545不能作为此次最新效果链接；正式部署Workflow已有，不重复创建。


2026-10-08 14:51用户停止定时检查：model-relay heartbeat正式PAUSED并复核，原产品Run未取消。seq51第21次完整tests exit2/98283bytes/5b0822a0…，187Node/Go race/迁移等分项绿，新editor-held-readback测试失败，正式返seq52 development。分页query未被测试精确URL拦截的锁版本诊断已有证据，尚待真实宿主复验；QA47 P1/P2继续Open，Task5/U1及最新PR合并部署未完成。Harness主接力/Schema纠错与固定消费/返工/澄清与等待恢复/去重/权限切片实测已通过，仍不能外推全部当前异常或完整交付。Open PR0、5545旧9851177/schema2，停止定时不关闭服务、不代替Agent、不绕门禁。完整状态与后续步骤见统一验证记录本节。


2026-10-08 15:17即时部署请求：定时仍PAUSED，用户另行授权当前会话部署两项。平台8793已运行最新功能代码，运行binary/实际四项前端资源与维护源一致，后续HEAD仅文档，不重启在途任务。Model Relay seq53同6b6564c9…完整exit2/98085bytes；真实1280中断回归已通过，随后路由重复continue失败，正确返seq54研发清理时序。产品QA47 P1/P2及新PR/部署未闭合；协调者只核验原Harness交付产物并准备正式Owner Actions、非空隔离升级/恢复与最终preview，不代改产品、不绕门禁。详证见统一验证记录。


2026-10-08 15:30按用户纠正完成“较新版本可看”目标：8793平台最新功能binary/四项实际资源一致，真实管理员页面可打开；Model Relay最近已合并PR19/main9851177经正式Owner deploy Pipeline37743445224 success，Already deployed同版本核验/未重复激活，5545 health/deployed精确同SHA/schema2，真实登录页可打开。不是design-v0.2.0整改版本验收/发布，原QA47/Task5/U1未完成事实保持。PR合main仅prepare，不是每个PR自动发布；定时保持PAUSED，原Run继续，不代替Harness或执行新候选隔离故障。详证见统一验证记录。


2026-10-08 15:48用户查看在研版：Issue25 Open/最近Hook回写6055263862与Run seq57 tests running一致。seq55完整失败经原路由返56修J06日志详情driver，193Node/静态局部绿，新134文件fb4066fa…正在正式完整复验；QA47 P1/P2、新PR与部署未闭合。10/6 PR19只有早期六项网关管理，实际页面与当前多租户控制台产品目标差距很大，不用旧版或候选自报冒称产品化完成。用户查看入口和证据见统一验证记录，定时仍PAUSED，不新增任务或代写产品。


2026-10-08 16:00Issue及原型核对：PR19发布后7Issue20–26，四个产品任务/三个Harness验收；实际Run20/21 cancelled，24旧waiting，22/23/26 completed，只有25在推进，GitHub旧Open不代表并行开发。本轮seq57同fb4066fa…完整exit0/106669bytes、193Node及新管理摘要/七旅程/十页三视口正式通过，seq58独立QA在途，QA47关单/新发布仍待证据。冻结design-v0.2.0工作台/租户/Key截图原件摘要核对并供用户展示，设计演示数据不当产品实测；design承接原包，没有重新画原型。定时仍PAUSED，不关闭旧Issue/恢复旧Run或代改产品。详证见统一验证记录。


2026-10-08 本轮 Issue25 实时核对：QA58已正式完成，产品候选Go with known issues；QA47-01 P1与QA47-02 P2原层Closed，新增QA58-01 P3技术枚举/窄列断词Open非阻断。seq57完整make verify exit0/106669 bytes/truncated=false；report59完成，publish60正式Connector exit0，真实提交66f1328ada75ea90e5638fbf2bd58663ac86347e已推送workflow/4cd62ced6a4f98f50e3cca6f7596eb06。维护者独立逐件git对象核对134文件与QA58清单及fb4066fa37d1e7865d3a18f72dd78fef98ae79bde17e20875af373b24771c3a6源戳完全一致；GitHub远端ref及固定提交QA报告可访问。

实际卡点：有效预算60耗尽，Run failed/seq60，error为maximum node executions reached；pr尚未派发、Open PR0，不是产品回归失败。检查冻结边publish→pr→done及正式已推送回执，余下为两个明确节点，无返工循环；准备按已有自主接续和测试仓PR授权通过Owner正式return(seq60,target=pr,max_steps=64)，不重放已成功publish、不新建Run/Agent、不改冻结图/历史/DB/权限。恢复结果待后续真实API回读记录。5545仍旧9851177/schema2，无新合并部署；H及真实供应商/支付/外部身份边界继续未验，定时任务保持PAUSED。


正式恢复回执（2026-10-08 19:39 Asia/Shanghai）：Owner return HTTP202，seq60→61 pr，有效max_steps64。原始60步及冻结definition经管理员正式API深比较完全不变；CI return响应含调用者权限相关配置脱敏，与管理员响应不可直接作为同字段比较，先前核对脚本误报AssertionError已通过同Caller回读排除。pr61由正式github.pull_request Connector创建草稿PR27，head_sha66f1328ada75ea90e5638fbf2bd58663ac86347e；URL https://github.com/big91987/model-relay/pull/27 ，已附到当前会话。done62完成，Run completed/error空，不重放publish。Run completed仅表示编排已交付PR，不等于用户部署目标完成；PR尚未合并、正式新Actions部署/health/schema3/实际用户页面未完成，旧5545版本保持。产品Go with known issues与P3保留，定时仍PAUSED。

用户询问预算和Web验证方式：原Run冻结40，协调者上次恢复设60，本轮明确检查只余pr→done后正式设64；是节点执行总次数，包含Agent/Connector/结束节点和返工，不是模型消息或工具调用次数。当前通用模板100，未因此改写老Run预算。seq57宿主make verify经go run ./tests/browser启动真实Go/隔离SQLite/受控HTTP-SSE上游并用Playwright浏览器执行实际UI；1280/1440/390全部J01–J07通过。QA58自身未重新操作浏览器全链：独立核验版本源戳/固定完整回执、设计比对30页+8关键图，独立193Node/5非监听Go race，关闭原层缺陷。区分宿主UI-E2E、独立证据/视觉验收和未完成的正式部署UI，不能声称QA亲自完整复跑或供应商联调已验。


## 持续研发中的Harness改进（2026-10-08）

本节承接用户长期推进授权，服务Model Relay持续交付（详见workflows-plan.md），不把产品实现转给协调者。当前平台8793、唯一维护源本仓及codex/workflow-node-config-ui；原生Codex/gpt-6.1-sol配置保持，Platform仅开发分支提交推送，不建新PR/不合源main。

| 工作 | 当前事实/用户影响 | 最小处理与验收 | 状态 |
|---|---|---|---|
| QA独立真实网页入口 | QA确有browser-validation.check/auto工具，但browser_tool.py公开合同仅root=app或docs/workflow/prototype；当前Go动态服务不在其能力内，native监听限制下QA不能自己重跑真实Web | 先核对现有受信浏览器运行时/fixture生命周期，复用标准工具边界支持当前工作区的隔离真实服务；QA经原生工具自行fill/click/错误恢复并核对同版facts。限定受信目标和资源清理，不临时开网络/提权、不把静态原型绿当后端E2E。先修源码/模板/安装，再以真实Pipeline复验 | Ready for diagnosis，未实现 |
| 长流程预算和卡点呈现 | 原40、恢复60在publish后耗尽；正式return已恢复到64并完成PR，当前标准模板100。用户能配置max_steps，不是每消息/工具计数 | 复用现有配置/告警/恢复语义检查新模板默认和用户可见剩余次数是否合理；以实际返工成本支持默认选择，停止原因和明确接续可操作，不自动无限扩预算、不重放已结束副作用 | 有界恢复已验；默认及呈现优化Planned |
| 正式失败/重复/权限与版本证据 | 既有典型路径部分已验，QA58 H02/H03-runtime/H04/H08仍有证据缺口 | 沿原编号做针对性真实验证：重复入站唯一Run、材料缺失/错SHA拒绝与恢复、实际模型证据可获得性、模板升级新装/重入和旧Run保持；只验证缺口，不为填表重建重复产品任务 | Planned |
| 可复用交付与定位效率 | 产品多次返工涉及driver、采集和证据版本；宿主全量测试仍必需但不能以重复大日志代替最小归因 | 保留正式完整门禁；定位时先准确失败阶段/请求ID/同版最小案例，再由产品Agent修；交接携带源码摘要、事实与未验边界。标准安装/升级覆盖新仓和已有仓，工具日志/截图只按授权呈现 | 持续执行 |

每项通用改动必须有原故障、修复正常路径与失败恢复证据，并同步维护源/模板/依赖/手册；当前Task5/U1不能因PR节点done直接改Accepted，正式部署与未验范围保持可见。UI与跳转优化按用户先流程可靠后页面的顺序推进。


最新合并前复审：PR27发现零软预算及回拨过期凭据两项源路径可达P1，复现待产品Pipeline；当前发布暂缓，保留原QA58证据并正式返development补回归/最小修复/完整测试/独立QA，详见workflows-verification.md本轮发现记录。阶段1继续Active，其余里程碑状态保持，不因新发现擅改冻结需求或手改产品。


长期推进当前动作：Owner正式return已接受，seq63原生development会话a61735cc14d2675688203cc090f2e44f running；候选两项Important由产品Agent复现/修复，源模板/QA浏览器改进保持独立责任，未擅改产品权限。当前预算64尚未耗尽，若下一正式门禁后触顶按现有支持入口检查接续。


用户最新要求恢复持续定期推进（2026-10-08）：通过原automation_update更新model-relay为ACTIVE，保留原30分钟频率、当前thread和通知策略；同步长期阶段、PR27/研发63返工、新旧测试版本及QA浏览器边界进任务prompt，不创建重复定时任务。配置正式回读ACTIVE。当前同Run seq63 development running，原生会话a61735cc14d2675688203cc090f2e44f，自报“内进程已复现两项缺陷；继续补重启、期限和窗口守恒回归”；这是Agent执行进展，不等于原完整宿主测试或独立QA通过，PR27仍待修复版重新验证再合并。定期检查先核实时态再推进受支持入口，卡点记录并诊断修复；重要结果/问题通知，正常未变保持安静。此前PAUSED记录是历史，不代表当前自动任务状态。


2026-10-08 22:40：产品Agent已完成budget-clock-r1整改候选，seq64原完整tests正在运行；handler/SQLite旧红与当前局部race真实日志已读取。QA/正式HTTP全量与PR更新未完成，64预算待真实结果后检查恢复，不提前放宽、不代替Harness执行产品测试。平台通用浏览器缺口仍Ready for diagnosis，未新增实现/升级结论；詳证见统一验证记录本轮条目。


2026-10-08 22:45：budget-clock-r1新完整seq64宿主门禁exit0/110897 bytes，ActualHTTP原层红→绿及三视口全旅程通过；原64预算耗尽已沿Owner正式return到QA65，总上限100对齐标准模板，原64步/冻结图无变化。原生QA09572eed8c075180d4e8b4492aeb7a4f running；产品缺陷尚未独立关闭、PR未更新/合并/部署。修复候选只读复审进行中，源平台通用浏览器能力仍未实现、不新增QA实操声明。


2026-10-08 23:10 当前进度：QA65独立关闭budget-clock-r1两项问题，六组非监听race24.750s与193Node通过，并复核seq64原完整门禁及同版UI证据；维护者候选只读复审无新增Critical/Important。Run正在report66，尚待正式publish/同PR27/合并/Actions部署。原生QA动态网页实操仍Blocked，当前受控宿主浏览器通过不能填补独立QA实操；通用浏览器能力仍Ready for diagnosis，未实现、未升级。总预算100未触顶，不新增恢复、任务或限额。


2026-10-08 23:40：正式publish67在副作用前拒绝QA65 artifacts中的host证据目录，文件契约原已明确，未放宽安全门禁。只读复现/140件源戳一致后正式stop→return到QA68纠正交接；直接普通failed return会409，按现有协议处理，失败回执和此前66步/冻结图保持。当前预算100有余，无新平台代码改动；该次产物约定错误及正式恢复详见统一验证记录。通用动态浏览器能力仍待实现，不能以产物路径修复宣称QA独立浏览器已通。


2026-10-09 00:10：文件级QA68/report69纠正经正式publish70 exit0、pr71同PR27、done72闭合，140件提交源戳与QA65一致；原seq67目录拒绝现场保留。这证明该次停止/回退/接续及文件门禁恢复，不等于通用动态浏览器已修复或所有H验收完成。测试仓PR27已授权精确合并，正式prepare/dispatch部署在途；平台仍只有开发分支文档提交，不新增源PR/main合并。


2026-10-09 00:20：正式干净main部署拒绝依赖顺序错误，旧工作区缓存掩盖frontend-test缺playwright-core，产品根因由新Issue28/Run3e7496815b9922704072482cf44e73e2承担；独立冷安装验收需进入持续交付要求，不能把旧工作区绿色当标准首次安装通过。另定位平台controller同SHA verify日志w覆盖历史；维护源改每次尝试独立600私有日志，不新增API/权限或领域对象，真实git/make双失败→成功回归旧版1!=2红、新版绿，完整控制器/安装/隔离/故障/Workflow60测试58.517s exit0。只读复审/原安装器升级待闭合；不能补回已覆盖的首次日志或宣称实际正式双失败重试已被新控制器验证。QA浏览器通用缺口仍Ready for diagnosis。


日志修复标准集成完成：维护源789cc90，独立复审无Critical/Important，原install.py以原根/仓库/端口/代理参数升级preview5545和validation5546；source/controller/controller-install摘要一致2d456ca50265cdd6db89ee4b0f3d0e85695bec45adff7a2b2665c826d48fab79。原配置/activation committed/可得旧失败日志字节保持，两产品health均旧9851177/schema2健康，没有部署或启停新产品。真实新控制器Actions尝试及日志绑定仍待Issue28候选，测试夹具通过不能替正式Pipeline。Issue28当前development4原生93e40ccfc93d89bd4400be032eb6947b running。
