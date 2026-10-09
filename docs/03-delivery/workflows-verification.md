# 平台内 Workflow 验证记录

基线：2026-10-05，分支 codex/platform-workflows，起点 3f3b2c8。

当前长周期目标：**No-Go／In Progress（2026-10-06）**。路线1首次调用体验已正式发布；路线2多上游/有限切换经 Issue #18 原 Run 的固定门禁、独立 QA 返工、PR #19 合并，正式 main `98511771871cf0951ecef716bbb55deb13e18da5` 已部署。自动 prepare、隔离目标真实健康失败后的旧数据恢复、显式重试 schema1→2 及预览发布均有 Actions/API/UI 证据，见文末。路线3 Issue #20 已由 GitHub Actions/SDK 自动进入唯一 Run `9e6d05f409675e0ef65c6486691462df`，需求/设计已完成并交接研发；路线4待执行。DEP02–04 剩余真实 Go 联合负例、DEP05 未覆盖安装边缘及后续产品路线未全部通过。真实供应商联调 Not Run；源 PR #5 仍 Draft/Open、未合并。历史小节保留当时事实，不能把历史片段或单项通过外推整体完成。

历史切片结果：**Pass（GitHub Issue → 研发 → 草稿 PR 验收）**。通用 SDK/API、自动接单、原 Issue 绑定与评论接续、bug 短路径、新需求分派、真实项目测试、独立 QA 及返工、停止／恢复／重启和去重均有下方真实证据。此前遗漏 Issue 入站而外推的整体 Go 结论不沿用；本轮按用户要求不新增部署和 code review 验收。源仓库 PR 仍 Draft/Open、未合并；真实供应商及其它聊天渠道未测／未接入。

| 验证范围 | 状态 | 证据 |
|---|---|---|
| 图编辑、保存、授权、版本快照（WF-01/09/10） | 部分通过 | 定义 API、权限与网页编辑通过；T2 用版本变更测试验证运行冻结旧图 |
| 真实 Agent 接力、人工确认与回退（WF-02～05/12） | 通过已列旅程 | T2 人工回退；T3 正常研发交付及 QA 缺陷返工，同一 Run 完成修复复验 |
| GitHub Connector 与新仓库闭环（WF-06） | 通过已列旅程 | T3 真实命令、Issue、评论、草稿 PR 与研发模板；E1～E6 外部丢响应先核验后恢复；测试仓 PR 合并及 Actions 见文末 |
| 停止、重复交接、重启及升级（WF-07～11） | 通过已列旅程 | 正式停止／恢复、幂等、迟到结果隔离、外部核验与恢复、旧安装升级及真实服务重启；物理断网未测 |

隔离原则：独立 worktree、独立数据目录和服务端口；不更改旧平台现场；测试仓库单独创建。敏感运行数据和截图保留本地忽略目录，公共证据只记录脱敏步骤、版本、结果和外部测试资源链接。

## T1 定义持久化切片

- 真实组件：Go HTTP handler、现有登录与用户 Token、SQLite；没有模拟存储或授权。
- 先运行新增 `TestWorkflow*`：全部因缺失入口返回 405 而失败。
- 实现后 `go test ./internal/platform -run '^TestWorkflow' -count=1` 通过。覆盖带退出路径的反馈循环、歧义路由／无出口／失效节点拒绝、版本冲突及跨用户读取限制。
- `go test ./... -timeout 120s` 通过（平台、pipeline-tool）；`go vet ./...` 通过；`git diff --check` 通过。
- 范围限制：本轮测试仅证明图定义和 HTTP 访问行为。Connector 执行、运行状态机、Agent 交接、UI、安装与重启恢复均未通过此结果证明。
- `go test -race ./internal/platform -timeout 120s` 通过；macOS 链接器输出 `LC_DYSYMTAB` 警告，未导致链接或测试失败。新增文档本地链接核验通过。

## T1 可视化编辑器与真实网页验证

日期：2026-10-05。隔离实例启动命令：`./bin/agent-platform -listen 127.0.0.1:8792 -base-url http://127.0.0.1:8792 -data .data/workflow-demo`。未变更原 8788 实例。

真实网页操作创建 `草稿审核 · 编排验收`：

1. 管理员登录，在工作流页新建空白图。
2. 从节点面板拖入 Agent，配置现有 Codex；加入人工确认和结束节点。
3. 通过端口建立 next／approved／changes 路由，changes 回到 Agent；包含真实鼠标端口拖拽，不只修改 JSON。
4. 保存后取得 `456bd401841d315a4f3d6f75fae12b77`；刷新后 3 个节点、3 条连线保留。
5. 拖动 Agent 从 (77,47) 到 (141,78)，再次保存和刷新，坐标保持；之后重建回退路径并保存到版本 3。
6. 导出面板展示完整 JSON（无源 ID、版本和用户授权）；从正式导入入口粘贴，独立保存副本 `782c122df45559f911d656d4b9d7540e`，版本 1，原定义保留版本 3。
7. 隔离服务重启后原定义仍可打开。截图在本地忽略目录 `.data/workflow-evidence/editor.jpg`。

修复：页面资源回归测试发现新 JS 未被 HTTP 静态白名单提供。移除重复名单，按明确的 embed 资源集提供公共静态文件；全套 Go 回归恢复通过。点击连续添加重叠节点问题先补失败行为测试，再实现避让并通过。

验证：15 项 Node 行为测试通过；`go test ./... -timeout 120s`、`go vet ./...`、`git diff --check` 通过。浏览器未记录页面控制台错误。

限制：上述流程只配置图，未启动真实 Agent Run。Connector 在页面标明尚未开放。文件下载事件等待超时，尚未取得下载文件，不能宣称文件导入／导出已验收；已验证的是正式 JSON 文本往返。运行版本快照、审批／回退执行、停止和未知外部效果恢复留待 T2～T4。

## 独立测试仓库

已通过 GitHub 正式入口创建并查询核实 [agent-platform-workflow-demo](https://github.com/big91987/agent-platform-workflow-demo)，可见性 PRIVATE，默认分支 main。初始化阶段仅含 README；后续 T3 已使用该仓库完成真实交付，禁止把 reading_list 的旧成功记录算作新方案验证。

## T2 持久执行与真实反馈循环

日期：2026-10-05。在 T1 独立实例上升级正式二进制，保留原定义与数据；原平台未变更。独立测试仓库已克隆，使用 `test/workflow-draft-feedback` 分支，产物由真实 Codex 写入。

### 旅程 A：人工反馈形成循环

Run：`f0aeaf74912bc9d4bccdaf748782f420`，冻结定义版本 3。

1. 从网页“运行”提交一句话：“帮我给这个示例项目写一份面向新用户的快速上手说明。”
2. Agent 在会话 `bbc04560779a4e888c98c8daae040dc5` 读取仓库、生成 QUICKSTART.md，并通过真实 MCP `complete_node(next)` 交接；网页按原生事件显示工具与 Thinking。
3. 等 Agent 结束才进入人工确认；在此处受控重启服务并刷新页面，仍是相同 Run 的第 2 次节点执行。
4. 网页输入修改意见，点击 changes 回到草稿：删去内部工具名、压缩成面向普通试用者的三步。
5. 第 3 次执行会话 `d30cb7f7090c6a621a690680215d5ed4` 收到真实反馈并修改原文件，再调用完成工具回到人工节点。
6. 读取实际文件核对通过，再从网页批准；同一个 Run 达到第 5 次执行的 end，刷新后仍为 completed。

### 旅程 B：人工指定回退、澄清、停止与原会话恢复

Run：`5ff2da857f2bbe8a6b9dee66bf6922e0`，同样冻结版本 3。

1. 网页提交简短发布说明任务，版本号稍后提供。Agent 先生成了含待补充版本号的草稿并交接。此结果只证明草稿交接；不把它算作自动识别必要澄清通过。
2. 在人工节点停止运行，通过正式“回到节点”入口选择草稿，输入版本号不可占位、先询问用户的要求。旧人工执行保留为 cancelled，新执行为序号 3。
3. 会话 `2c6b0a1ba7c2128afd1e7d6678c6857f` 询问版本号，未调用完成工具，Run 保持 waiting；网页回复 v0.1.0 后继续。
4. 在原生 Codex 执行期间从运行页停止；Run 留在序号 3，没有分发下一节点。
5. 在已停止状态重启服务，通过网页“继续原节点”输入明确继续说明。使用原会话、原工作区接续，先检查已生成文件再交接。
6. 正式 API／事件记录核对：原会话的三次用户输入状态为 completed／stopped／completed，原生 thread.started 记录只涉及一个 thread ID。未重新创建会话或重复生成文件。
7. 实际 RELEASE_NOTES_DRAFT.md 含 v0.1.0 和新增快速上手说明；网页批准后进入 end。

### 实现约束、回归与证据

- 先补缺失 Run 类型／API／引擎的失败测试，再实现事务、调度与 MCP；另补内部指令不充当用户发言、重启不重放不确定轮次等测试。
- 15 项 `TestWorkflow*` 覆盖真实 SQLite 和 HTTP／MCP 协议：版本冻结、有退出路径循环、最大执行数、重复启动与变更冲突、重复完成／旧回执拒绝、跨用户拒绝、过期审批、队列清空后交接、停机恢复和权限收回。测试中的手工 Claim 用于隔离状态机验证，不能代替上述原生旅程。
- `go test -race ./... -timeout 120s`、`go vet ./...`、15 项 Node 行为测试通过；链接器仍有已知 LC_DYSYMTAB 警告，测试退出码为 0。
- 真实页面发现并修复：内部执行上下文挤进用户气泡、会话缺少返回运行页入口；现在任务原文与私有节点配置分开保存。
- 本地忽略目录 `.data/workflow-evidence` 保存两个 Run、相关会话和原生事件 JSON；`run-completed.png` 保存旅程 A 完成截图。API 导出不包含私有 Agent 快照或节点凭据。
- 本轮没有更改旧 reading_list 仓库，没有启动旧 CI、没有合并任何源 PR。测试产物尚未通过 Connector 创建 GitHub PR。

未测／未完成：Connector 配置与 GitHub／命令真实执行、外部副作用未知结果核验、无人工反馈的研发模板、全套安装升级验证、进程强杀时真实原生恢复。文件上传／下载仍沿用 T1 未测限制。标准 `scripts/verify.sh` 中依赖旧 Python／浏览器环境的检查尚未整套执行；不得将本节 Go／Node 结果称为全仓供应链验收。

## T3 Connector 首个真实外部切片

日期：2026-10-05，独立 8792 实例升级；此前两个 T2 Run 已完成，升级后历史仍可查。

1. 从网页创建三个 Connector：固定 `git status --short` 命令、GitHub Issue 创建、GitHub 评论。绑定同一个独立测试仓库，管理员权限，不修改全局 Agent 配置。
2. 网页保存并检查命令配置；导入四节点图“查看工作区 → 记录测试任务 → 反馈已有产物 → 完成”，图 ID `92f6d67fa9e0fd56862b464a603e8990`、版本 1。
3. 从正式网页运行入口提交普通任务。Run `a003ae3a5e9659a396f7223ae613debb` 完成全部 4 个节点，始终为同一 Run，无 CI 重新分发。
4. 命令真实退出码 0，输出工作区中两个未跟踪文档；随后创建 [测试 Issue #1](https://github.com/big91987/agent-platform-workflow-demo/issues/1)，再读取真实发布说明文件生成 [一条反馈评论](https://github.com/big91987/agent-platform-workflow-demo/issues/1#issuecomment-5982886730)。用独立 GitHub 查询核对标题、正文、关联标记和仅一条评论。
5. Run 页面保存对应退出码、Issue／评论链接、调用回执和完成历史。截图与正式 API 导出保存在忽略目录 `.data/workflow-evidence`。

回归：`go test ./...`、`go test -race ./... -timeout 120s`、`go vet ./...`、15 项 Node 行为测试通过。先写失败的 Connector 行为测试再实现；覆盖配置快照、授权隔离、工作区边界、停用、真实命令非零路由／进程停止、凭据脱敏、未知命令不自动重放、GitHub 回执丢失后只读核对。GitHub 丢响应测试使用隔离 Transport，仅证明协议恢复逻辑；尚未模拟真实网络中断。真实命令测试执行本机程序，不生成假成功回执。静态页面资源测试发现新增 Connector JS 未嵌入，已补入明确 embed 集后通过。

限制：本节不是完整研发闭环；尚未通过 Connector 创建草稿 PR、未验证从一句话需求到代码／QA 修复再交付的模板，未实测 GitHub 写入中断后的正式恢复。节点参数已改为直接表单；网页保存正文文件与 Issue 节点引用到图版本 2、刷新后保留；已完成 Run 仍保留冻结版本 1。没有把单元测试通过描述为整套 Pipeline 通过。

## T3 研发模板安装与首条交付旅程（进行中）

维护源为 `examples/platform-workflows/`。正式安装器使用管理 API 注册 5 个阶段 Agent、5 个 Connector、stdio 浏览器工具和 12 节点图；资产路径外置，未改数据库或旧测试仓库。采用已锁定的 Harness 浏览器依赖，安装与真实 Chromium 启动探针通过。

- 同一安装清单再次执行：对象 ID 和图版本不变，没有重复创建。初次重入发现 Go API 空默认字段造成伪升级，已修复结构比较，并通过同一入口复验。
- 本地 Git 回归三项通过：脏目录拒绝、真实提交推送／重复发布无空提交、错误分支与凭据文件拒绝。
- 从网页提交一句话需求“做一个本地待办网页，可以新增、完成、删除和筛选待办，刷新后保留数据。”；Run `53d0ed8b44d1b6eae020996ad84d1d55`，图 `123cf05ae9959c092ea0ea15f99abb6d` 版本 1。
- 准备 Connector 真实创建隔离分支，Issue Connector 创建 [测试 Issue #2](https://github.com/big91987/agent-platform-workflow-demo/issues/2)。需求会话 `42ffa386394c78e8fbc61fca56a31e0b` 和设计会话 `91207b9ac848189f78b8604897f90ad7` 自主记录推荐及 G1/G2 自查，正常工具交接，无重复人工批准。研发会话 `9198b1ad9a2571160ca2dfde0f6e4099` 已启动。
- 工具与 Thinking 在对应 Agent 名下按时间显示，已完成调用默认折叠。当前研发、独立 QA、代码推送与草稿 PR 尚在验证，不声明完整交付通过。

工作区隔离补充：测试先复现两个未结束 Run 共用目录被接受，再在启动事务中拒绝同目录、子目录和符号链接别名；停止状态仍占用，正式结束后可再用。针对 Workflow 测试及完整 Go race 回归通过。原 Connector 取消测试先按正式审批路径结束前一任务，再开始后一任务。此变更尚未替换在途验收服务；当前真实 Run 运行于 `153803f` 引擎。

标准入口 `bash scripts/verify.sh` 在本轮修正后完整退出 0：前端 17 项（含真实浏览器回归 2 项）、Python SDK 7 项、GitHub 示例 96 项、本地部署 11 项、新模板 Git 操作 3 项，ruff／gofmt／go vet、全仓 Go race 与二进制构建通过。原始日志保存在 `.data/workflow-evidence/verify-software-delivery-final.log`。macOS 链接器 LC_DYSYMTAB 警告未导致失败。以上回归不替代仍在进行的真实 T3 交付旅程。


## T3 正常交付完成与运行版本复核

Run `53d0ed8b44d1b6eae020996ad84d1d55` 已 completed，共 11 次节点执行。需求、设计、研发、命令测试、独立 QA、交付说明、提交推送和 PR 创建均通过真实执行完成。独立 QA 会话 `4960f4e07f8629091bf509d17f0ddf8d` 重新运行 10 项 Node 测试及 6 次注册浏览器检查；最终由 Connector 创建 [草稿 PR #3](https://github.com/big91987/agent-platform-workflow-demo/pull/3)，对应 [Issue #2](https://github.com/big91987/agent-platform-workflow-demo/issues/2)。GitHub 查询核对 PR 为 OPEN/Draft，head 为 `6e5fd92f32013636e27dc8432767790bae8b4084`，与平台推送回执一致，未合并。

补充网页操作验证新增、去除首尾空白、完成、筛选、刷新保留、关闭再打开、删除并刷新。此处是独立浏览器复核，不把 API 查询当作页面验收。服务更新到 `1ce18ea` 后再次读取四条已完成 Run，图、历史和回执保留；原在途运行结束后才更新二进制。工作区互斥修复已进入当前运行版本。

## T3 真实缺陷返工（已完成）

在已交付版本上新建专用 `test/qa-return-delete` 分支，提交 `68fa413` 故意移除删除按钮的事件绑定。它是明确标识的故障注入；没有改动原 PR 分支。使用维护源 `qa-rework.json` 和安装 API 注册返工图，正式 Run `85eb42ce941a5007d6c7e5da0b94637c` 从 QA 开始。

首轮 QA 会话 `48c2d63ad386a77b5100d80b409a9b86` 在真实 Chromium 中重现“点击删除后仍存在，刷新也仍存在”，记录 P1 QA-DEL-001、两次失败回执和截图。10 项模型测试通过但未覆盖这个 UI 缺陷，因此 QA 明确 No-Go，通过真实 `complete_node(development)` 交接到研发会话 `aadef61d219f28502aeca0e5d6b997e9`。研发恢复删除绑定、先补红灯回归再修复；命令 Connector 独立执行 11 项测试通过。第二轮 QA 会话 `7795590c407c8151737de5f6571a1739` 重新执行 check-1-20～26 共 7 条浏览器旅程，全部 passed=true，原网页删除失败关闭。实际 app.js SHA256 为 `411930d9cd2140ee19e23c1c2c799468616c146dbf5d5c53193f94d64d8bc53e`，已直接核对文件及浏览器回执。旧失败证据保留。

测试执行者核对原红灯、修复、命令与独立浏览器回执后，经正式 decision API 选择 accept；Run 最终 completed，6 次执行为 QA → development → tests → QA → decision → done，始终同一 Run。此确认仅结束验收记录，不合并任何代码。

此返工 Run 通过正式 API 启动；本轮浏览器控制通道及文件选择器超时，因此不将其记作网页发起通过。正常交付与 T2 人工回退已由网页实际操作完成；文件上传／下载仍未验收。

## T4 真实 HTTP 权限与幂等

维护源入口 `examples/platform-workflows/verify_access.py` 在隔离实例运行，证据 `.data/workflow-evidence/live-api-access.json`。11 项检查通过：重复请求只保留同一 Run；同请求改内容冲突；工作区重复占用拒绝；另一用户读取／审批／停止／回退／恢复均拒绝；更新图后已有 Run 仍冻结版本 1；所有者批准完成；停用图禁止新启动。初轮 Run `485e0b419eeb6f6bb36e8a07de0c1bf8` 完成；清理逻辑更新后复验 Run `7027736faa0c825b589b39aa03933e2c` 同样通过，两个临时账户及图已停用。没有直接修改数据库或构造回执。

这只证明真实 HTTP 层，未代替跨用户网页测试。外部副作用未知结果的真实中断恢复、文件上传／下载、完整发布审查仍未完成。所有改动继续在开发分支迭代，不合入 main；此前过早创建的源 PR 不作为交付完成证据。


## T4 真实命令停止与服务强杀恢复

可复用入口为 `examples/platform-workflows/verify_command.py`，分别使用正式 HTTP 的现有隔离实例模式及脚本自建临时服务模式。没有操作持久表或补造回执。

- 停止模式 Run `32dc01fd126123123b21cfba777c1e74`：真实子进程写入一次记录后等待；正式停止入口杀掉该进程；resume 拒绝重放，操作数仍为 1。明确 return 到命令节点后才新增第二次执行，真实退出码 7 按 failed 路由到人工节点，再由测试执行者确认完成。
- 强杀模式首轮 Run `4c7c580b6071b996595a0ca307d379df`，清理错误处理补充后复验 Run `739641be01c42ce23b1c96c5e6eb5eda` 同样通过：只对脚本新建临时实例执行 SIGKILL，再用相同数据目录启动。Run 从真实运行转为 failed，说明缺少确认结果；原进程已终止，记录仍只有一次。正式停止／显式返回后完成同样的失败边验证。原 QA 所在实例未重启。
- 原始证据分别为 `.data/workflow-evidence/live-command-recovery.json`、`live-command-restart.json`，包含每次执行和操作数；临时实例已停止、验收资产已停用。
- `scripts/verify.sh` 本轮完整退出 0，日志 `.data/workflow-evidence/verify-qa-rework.log`；仍有非致命 macOS 链接器警告。当前仅分支迭代，不把本轮检查称作最终发布通过。

剩余：浏览器文件导入／导出、真实 GitHub 写入丢响应恢复、最终 AC 审查。浏览器控制工具本轮再次超时，明确列为该 UI 用例阻塞，不用 API 结果替换网页证据。


## E1–E5 智能体编排、主动交接与通知（2026-10-05）

本轮继续开发分支，更新隔离 8792 实例；未合 main，未操作 reading_list 的在途任务。公开 UI 名称改为“智能体编排”，持久化 API 的 workflow 标识保持兼容。

### 真实交接与澄清

首轮 Run `c15fc2073b9e8c6213606d73a40eaee5`、复验 Run `64af6cfedc8bce880ecc8a40eedd4c1a` 均从网页输入一句话任务启动。Agent 先询问版本号，测试执行者在原会话分别回复 0.2.0、0.2.1；等待期间没有交接。之后真实文件写入、注册 MCP handoff、独立读取校验、返工、修正、再次 handoff、complete_node 固定完成，全程同一 Run，共 6 次节点执行。

该用例明确故意令编写节点首次漏掉验收章节，校验节点没有代写或伪造成功；其 inputs 传递缺失项，返工节点保留用户版本信息并补齐。两轮均 completed。可复用图在 `examples/platform-workflows/collaboration-check.json`，通过正式 `install.py --template collaboration-check` 安装。不是产品功能验收，也不是 Mock 执行器。

- [首轮测试 Issue #9](https://github.com/big91987/agent-platform-workflow-demo/issues/9)。发现首版 Hook 投递顺序受配置遍历顺序影响，启动通知可能超前于上一节点交接；原历史保留。
- [修复后测试 Issue #10](https://github.com/big91987/agent-platform-workflow-demo/issues/10)。按节点序号、启动／最终回复／交接／运行完成的顺序采集，持久化串行投递。真实 GitHub 查询确认 15 条评论与 15 条投递记录一一对应且顺序完全一致，无重复。
- 本地私有证据：`.data/workflow-evidence/collaboration-run-first.json`、`collaboration-run-final.json`；包含正式 API 结果、交接输入及实际 GitHub 评论。

### GitHub 写入结果未知恢复

维护源 opt-in 测试 `workflow_github_live_test.go` 对真实 GitHub POST 的成功响应注入客户端传输丢失，发现 GitHub 列表可暂时返回写入前结果。修复为只读复核时禁止使用旧缓存、逐次新查询及有界等待；绝不再次 POST 来“恢复”。

真实通过 Run `548466605f9dc5c6e4356b87a553e6b4`，资源为 [测试 Issue #8](https://github.com/big91987/agent-platform-workflow-demo/issues/8)。POST 数量为 1，找到原资源后继续，唯一资源数为 1，测试资源已关闭。证据 `.data/workflow-evidence/live-github-response-loss.json`。这是现实服务写入＋客户端故障注入，不声称进行了物理断网实验。

### 网页、兼容与回归

- 真实网页验证虚线自主交接、实线固定规则、节点聚焦、连线编辑、通知规则字段映射、保存后版本更新、JSON 文本导入导出。导入是新副本，没有覆盖原图。文件选择器和下载到磁盘仍未验收。
- 两轮会话均展示 Agent 进度、真实工具和最终回复；交接后会话关闭，运行页保留链接，澄清前用户可以继续回复。
- 首版 1280 宽度页面及最后 767 宽度响应式页面已观察。补充适应宽度与恢复原尺寸操作，窄屏将设置面板放到画布下方。工具指定 1440 视口未改变实际 767 视口，因此不把它记为 1440 通过。临时导入页发生浏览器句柄卡住，重新获取该页后已关闭；服务 HTTP 保持正常。
- 空数据目录正式安装及重复安装通过：7 个 Agent（含默认 Agent）、6 个 Connector、1 张验收图、1 个工具服务，第二次数量不变；临时服务已关闭。证据 `.data/workflow-evidence/fresh-install.json`。
- 缩放至约 55% 后实际拖动节点 20×10 CSS px，画布坐标按比例变化 37×18；通过表单恢复原坐标并保存。
- 正常升级前确认无在途 Run，备份数据后替换开发二进制；既有 Run 的冻结定义和历史仍可读。标准安装器升级研发图、返工图、Agent 策略与评论 Connector。网页保存同值配置后再次升级成功，没有绕过漂移检查。
- `scripts/verify.sh` 完整退出 0，证据 `.data/workflow-evidence/verify-orchestration-final.log`，包括前端行为、Python、Go vet/race 和构建。之后仅调整画布缩放与模板坐标，补做 JS 语法、5 项模型测试、构建及真实页面检查。macOS 链接器警告非致命。
- 延迟事件采集的顺序重建、未结束回复不外发、重复采集不重复通知、通知失败不结束 Agent、跨用户访问拒绝，以及主动/固定交接边界均有行为回归。

### 当前交付边界

本轮完成可运行的主动交接与通知切片，不能称为全部平台能力最终发布验收。固定人工门禁仍使用显式确认节点；阻塞型 before Hook、任意边上的 Gate、结构化 request_user_input、GitLab 通知动作和并行多分支尚未实现。循环当前按最大执行节点次数限制，不是按环的墙钟时长。通知结果未知的查询路径复用已验证 GitHub 恢复，但没有对真实评论执行进程强杀实验。

已有正常研发到草稿 PR 的证据见 T3，本轮新路由契约完整复验的是轻量协作闭环，没有重新声称跑过一份全新完整产品开发。主流程模板及本轮回归应结合这些明确限制评审。


## 典型分支旅程覆盖补审（2026-10-05，实施前基线）

对照 PRD WF-13～15，原先“核心闭环通过”的结论不能外推为全部研发旅程通过。

| AC／场景 | 当前实现与证据 | 结论／缺口 |
|---|---|---|
| AC-13.1 明确 bug 真正跳过需求／设计 | common SOP 提及阶段不适用，但 software-delivery 的 start_nodes 为空，需求出口仅到设计；原 QA 返工证明的是已有任务回环 | **未覆盖**；需要模板短路径和专用 bug 新任务的真实网页验收 |
| AC-13.2 脚本／配置修复的适量验证 | 已有 Connector 支持固定命令与成功／失败分支 | **未验收该旅程**；不能用普通产品开发测试替代部署脚本修复及恢复测试 |
| AC-14.1 显式起点与 Agent 自主选择 | 内核有允许起点和 handoff 目标校验；现有轻量用例验证交接选择 | **部分能力具备**；研发模板的起点选择、准备步骤及关联完整性未闭环 |
| AC-14.2 信息不足不能盲跳 | 两轮版本号澄清已验证等待和接续机制 | **机制已测、业务反例未测**；仍需模糊 bug、结构变更的真实路由判断用例 |
| AC-15.1 交接后纠正目标 | T2 有人工指定回退与停止／恢复证据 | **部分覆盖**；交接后自然语言纠正、在途目标隔离和迟到交接组合场景尚无完整实测 |
| QA 自动返工、人工拒绝回环、停止恢复 | 已列 T2、T3、T4 及 E1–E5 运行证据 | 已覆盖所列旅程，不能替代本表中的短路径与边界场景 |

后续验收必须分别记录每项真实入口、路由选择依据、执行节点序列、产物、测试结果和失败恢复；不得仅补提示词后将这些条目标为通过。


## E6／E7 典型分支旅程实测（2026-10-05）

本节更新上方实施前缺口。维护源为 `examples/platform-workflows` 的图、SOP、安装器和对应核心通知源码；不在业务仓库或数据库里打补丁。当前开发分支迭代，未合并任何 main。

| AC／场景 | 真实路径与证据 | 结果 |
|---|---|---|
| AC-13.1 明确 bug 跳过 PRD／设计 | 网页一句话发起 Run `82c5788827d042fec63f0518b7e00d72`，关联 [Issue #11](https://github.com/big91987/agent-platform-workflow-demo/issues/11)。prepare → issue → intake → development → tests → qa → report → publish → pr → done；没有执行 requirements 或 design | 通过，交付 [草稿 PR #14](https://github.com/big91987/agent-platform-workflow-demo/pull/14)，未合并 |
| AC-13.2 脚本／配置修复 | 网页输入补齐 npm start、127.0.0.1、PORT 与占用错误，Run `dfb98adc8b98053d51170129e17a7bef`，关联 [Issue #13](https://github.com/big91987/agent-platform-workflow-demo/issues/13)。直接开发，固定宿主 tests，独立 QA | 通过，交付 [草稿 PR #16](https://github.com/big91987/agent-platform-workflow-demo/pull/16)，未合并 |
| AC-14.1 显式起点和自主判断 | bug 用例由 intake 自主选 development；配置用例用户明确“从开发开始”，intake 核对资料后采用。两者都保留 prepare 和 issue，没有绕过测试／QA | 通过 |
| AC-14.2 模糊需求／结构变化 | Run `beee6e43aed58ce27470989bab0d1798`，[Issue #12](https://github.com/big91987/agent-platform-workflow-demo/issues/12)。输入“待办这里有点不对，帮我修一下”，Agent 询问操作、实际和预期，等待期间不交接；回复云端账户同步、离线冲突并要求从设计开始后才选 design | 通过路由与等待行为；未实施云同步功能 |
| AC-15.1 交接后纠正目标 | 同一 Run 的设计 seq 4 启动后，经网页停止、选择 requirements 并输入自然语言原因；旧设计取消，保留产物，新需求 seq 5 询问个人独立／多人共享这个缺口后等待 | 通过；验收后主动停止测试 Run |
| 迟到操作隔离 | 旧 seq 4 停止请求返回 409；旧设计会话输入返回 409。使用该专用测试旧节点实际 MCP 凭据发起真实 handoff 到 development，返回冲突；前后当前 Run 完全一致 | 通过；凭据未输出或持久化。MCP 负例在新需求已停止时执行，不声称覆盖所有并发竞态 |

### 明确 bug 的验证与失败恢复

已在原版本复现未知筛选值被误当作未完成：全部 2 条，未知值只返回 1 条。研发先得到 3 条红灯回归，修复后 14 项 Node 测试通过、21 步浏览器检查通过；独立 QA 对旧源码复现 15 次失败、当前源码 0 次失败，独立执行 14 项测试与 24 步浏览器检查，浏览器错误数与存储写入数均为 0。QA 前后产品／测试摘要不变。

首次推送遇到真实 GitHub HTTP 408；核对远端该任务分支不存在、本地提交完整后，在网页停止并显式返回 publish，复用同一提交 `c08d0304f7ce835d4ed9eac18a648d3a762c5636`，未重跑开发／QA。最终同一 Run 完成 11 次节点执行。此处核对真实远端状态后才重试写入，不伪造 Connector 回执。

### 配置修复的执行责任边界

首轮原生沙箱拒绝监听端口（EPERM），且新增启动测试未进入标准 npm test，任务停在开发。修复维护源 SOP 后通过安装器升级，在网页显式返回 development，保留旧失败。新版要求把必要测试纳入管理员已配置的固定 tests Connector，不临时扩大 Agent 权限。

宿主 npm test 真正执行 13 项、13 通过、0 跳过、退出 0；覆盖 npm start、资源字节／MIME、PORT、非法端口、端口占用、释放后恢复和静态路径边界。独立 QA 检查实现与测试、核对原页面／数据／测试命令未变，自身沙箱复跑 11 通过、2 项 EPERM；报告区分宿主执行和独立复核，没有把环境阻塞写成独立启动成功。该证据验证启动配置修复，不代表真实生产部署或数据库迁移已验收。

### 通知恢复修复

bug Run 的 report 最终回复通知在 GET 查重时遭遇 TLS 握手超时，旧代码把所有错误记录为 unknown，后续只查询而无法发送。根因为错误处理没有区分 POST 前与 POST 后。新增回归先确认 `lookup failure status=unknown, want failed`，再修正为类型化发送前失败；仅能证明尚未 POST 时清除未使用请求以允许正式重试。POST 丢响应仍只查询。

真实 GitHub opt-in 验证通过 [测试 Issue #15](https://github.com/big91987/agent-platform-workflow-demo/issues/15)：首次查询在传输前注入失败，正式通知 retry 接口后实际 POST 数为 1、GitHub 匹配评论数为 1，测试 Issue 已关闭。注入的是读请求失败，后续 GitHub 写入和回执是真实服务；没有假服务器。单元回归同时覆盖 POST 成功后丢响应的恢复不重复写入。

旧 bug Run 的那一条 unknown 保留原样：后来查证没有匹配评论，但旧记录不能证明从未写入，因此没有手改记录或盲目补发。其余通知及最终完成通知已成功，完整交付说明可在 PR 中查看；不能声称该 Run 的通知全部通过。

### 安装、升级与工程回归

- 安装器新增 intake 角色及 `--base`；准备分支和 PR 使用同一配置基线。短路径仍从图入口开始，避免直接 start_nodes 绕过必要准备。测试使用已有专用验证分支作为基线，不需要合入测试仓 main。
- 全新临时实例正式安装、再次安装均为 7 Agent／6 Connector／1 编排／1 工具服务，数量不变；自定义 base 同时进入 prepare 和 PR。临时实例已停止。原开发实例通过正式 `--upgrade` 更新到图版本 8，既有 Run 保留冻结版本 7。
- `scripts/verify.sh` 完整退出 0：Go vet/race、JS 行为与浏览器、Python 测试、格式和构建通过；macOS 链接器既有警告非致命。
- 确认无执行中 Run／会话后，以 SQLite backup 备份开发数据，使用正式二进制参数重启 8792，健康检查 200；8788 未动。随后网页刷新仍可看到全部历史，在原 report 节点恢复模型容量错误，没有重跑 QA。
- 私有原始证据位于 `.data/workflow-evidence/short-path-bug-run.json`、`short-path-config-run.json`、`short-path-stale-controls.json`、`short-path-stale-handoff.json`、`short-path-fresh-install.json`、`hook-preflight-live.json` 和 `verify-short-path-final.log`。

本次证明模板短路径、判断与用户纠正组合场景；不覆盖所有模型输入。此前列出的文件上传／下载、阻塞 before Hook、边上 Gate、结构化 request_user_input、GitLab、并行分支、按时间限制循环等缺口仍保留，不宣称全平台发布完成。

配置修复最终交付：report 节点曾因模型服务 `serverOverloaded` 失败；升级后经网页“继续原节点”恢复同一会话，核对 QA 快照后只更新交付文档，未重跑开发或 QA。随后 publish → pr → done 完成，仍为同一 Run、共 11 次节点执行，提交 `c9e1d222b0bb483067460688bb690f0165a8d562`、[草稿 PR #16](https://github.com/big91987/agent-platform-workflow-demo/pull/16)。10 条通知均 succeeded，GitHub 核对 PR 为 OPEN/Draft、base 为专用测试分支。新配置与旧冻结运行兼容；没有合入 main 或触发生产部署。

## Go 正式产品接入（2026-10-05）

用户指定以独立正式产品 Model Relay 作为后续场景，后端全部 Go。产品维护源为 `big91987/model-relay`；人工路线图与阶段任务在该仓库，不纳入通用平台业务逻辑。现有演示仓库保留为历史验收证据，不再作为这个产品的代码来源。

发现安装模板固定 npm test；维护源增加 `--test-command-json`，argv 直接交固定 Connector 执行，阶段策略同步使用同一验证入口。默认仍为 npm test，Go 项目配置 make verify。参数拒绝空命令、非字符串参数和 env 选项／赋值开头，不隐式执行 shell。测试 Connector 可继承显式代理，但不获得 GitHub Token。阶段指令明确静态浏览器工具不等于真实后端 UI 验收。

先出现缺少 verification_command 的红灯，再实现并通过 8 项 Python 回归与 ruff 检查。正式安装及同参数重复安装均保留 6 个阶段 Agent、6 个 Connector 和同一张图；API 核对测试 argv 为 make verify，所有阶段指令无未展开占位符，测试环境无 GH_TOKEN。通过网页启动 Run `c1089fdc410a9dcce0725c30ad190df4`，准备分支和真实 [Issue #1](https://github.com/big91987/model-relay/issues/1) 已完成。此记录只证明产品接入和任务启动；Go 产品功能、实际 make verify 与独立 QA 的结果尚待该 Run 完成，不预先声称通过。

补充结果：`scripts/verify.sh` 完整退出 0（`.data/workflow-evidence/verify-go-project-install.log`）。网页确认 intake 没有把已写路线图误当成完整 PRD，而是依据新产品和用户明确要求交给 requirements；当前 seq 4 需求节点执行中。未创建另一份重复 Issue／Run，未启动后续里程碑。

接续核验：通过真实运行页确认同一 Run 已完成 requirements（seq 4）及 design（seq 5），自动进入 development（seq 6），没有重复创建 Issue 或启动第二个 Run。设计产物为产品仓库 docs/workflow/design.md，G2 自查 Ready for Development；这只是阶段交接结论，不代表实现验收通过。运行页显示前三个 Agent 阶段的启动／最终回复通知及研发启动通知均已发送，并链接 Issue #1 的真实评论。研发会话显示原生命令和 Thinking 按执行顺序归属同一 Agent，已结束工具卡片默认折叠；当前仍在实现，make verify、独立 QA、真实后端浏览器旅程和草稿 PR 未完成，供应商联调未测。

工具命名核查：安装器把项目 prefix 与“浏览器验证”组合成 MCP 配置名称，工具页直接展示该名称。model-relay 是该配置的项目范围标识，不是工具实现的产品专用能力；维护源仍为同一 browser_tool.py。已向用户解释此边界；名称呈现尚未调整，不通过单独修改运行配置绕过安装清单的漂移检查。

## 共享浏览器与工作流运行记录（2026-10-05）

用户确认通用浏览器注册不应随项目重复，并纠正运行入口：每个工作流有自己的 Runs 页面，而非在总览混列运行。维护源已实现编排卡片／详情的“运行记录”、独立 `workflows/<id>/runs` 页面，以及单次运行返回所属记录列表。API 在所有者授权范围内先按 workflow_id 筛选，再取最近 200 条；旧全局 API 调用兼容。

浏览器统一注册 browser-validation，共享 manifest 负责配置升级，项目 manifest 保存引用。平台把显式 `{{workspace}}` 参数绑定到已核验会话的最终工作区，并固定 stdio cwd；原必需 workspace-root 校验保留。旧安装通过同一安装器升级；活跃 Run 阻止共享阶段 Agent 升级，仍被 Agent／可恢复 Run 使用的旧工具保留，其余旧配置停用，历史不删除。

验证：共享注册回归先因缺少实现失败；工作区绑定及工作流／所有者筛选回归先编译失败，再全部通过。最终 scripts/verify.sh 退出 0（私有日志 `.data/workflow-evidence/verify-shared-browser-final.log`）。隔离真实平台通过安装器共享注册入口，两个项目清单引用唯一 MCP 注册并成功发现工具；真实 API 两个工作流各创建一条人工等待 Run，按工作流查询只返回自己的记录。CUA 实际点击卡片“运行记录”→独立列表→单次运行→返回列表，没有混入另一工作流。浏览器适配器在两个隔离工作区执行真实浏览器检查均通过，越界 cwd 均拒绝；这只是工具组件验证，不冒充完整 Agent Pipeline。

应用边界：8792 的 Model Relay Run 仍在 development（seq 6），本次没有重启服务或升级其在途 Agent。新源码在独立实例通过验证，8792 新页面与两份既有项目注册的迁移尚未应用；完整项目安装／升级、共享工具经过原生 Agent 调用的端到端复验仍待安全升级时完成。产品 make verify、QA 和供应商联调状态沿用实际 Run，不因平台回归通过而预先通过。

### 共享工具安装与原生链路补验

在等待产品研发时发现安装保护缺陷：manifest 用 development／qa 等角色键记录 Agent，3717d9f 却按 agent- 前缀取 ID，造成活跃运行检查及旧工具引用检查漏判。先复现未拒绝升级和可恢复工具被尝试停用，再改为按已保存 spec.executor 识别 Agent。历史列表达到 API 的 200 条上限时，无法证明完整性，升级明确拒绝、旧工具保留；不把截断列表当完整证据。

| 验证 | 结果与证据 |
|---|---|
| 原开发实例只读保护核查 | 原 manifest 正确识别 6 个 Agent，当前产品 Run 被检查函数拒绝，未写配置；证据 `.data/workflow-evidence/shared-browser-upgrade-guard.json`。未在该在途实例执行升级命令 |
| 正式新装与重复安装 | 隔离实例通过原 install.py CLI 安装两个项目并重复安装，13 Agent／12 Connector／2 图／1 工具注册，第二轮对象 ID 不变；真实 GitHub 仓库检查通过，无 Issue／PR 写入。证据 `.data/shared-tools-e2e/install-proof.json` |
| 旧版升级与重入 | 从源码 1a87da9 的独立验收副本正式安装，再以原 manifest 升级；所有对象 ID 保留，design／development／qa 引用 browser-validation，无引用旧工具停用；重复升级无配置变化。证据同目录 `upgrade-proof.json` |
| 正式升级入口的活跃运行拒绝 | 专用人工等待 Run `79710f72445f8cebb90dee43a5d723cd` 引用已安装 Agent，执行 install.py --upgrade 退出 1 且全部配置前后一致；通过 decision 正式结束夹具。证据同目录 `upgrade-guard-cli-proof.json` |
| 原生 Agent → 共享工具 → 完成节点 | 专用 Run `f88e4d4aa8367eeaf343afddb6d2f55c`、`7663d5a7588b6cf5792b185879219ade` 均 completed；真实 mcpToolCall 调用同一 registered_browser-validation.check，分别断言各自页面 acceptance-a／acceptance-b 可见；两份 browser.json passed=true、errors 为空。原生配置中 cwd／workspace-root 分别绑定实际工作区，证据目录摘要不同。证据同目录 `native-proof.json`、两份 Run 和原生 events 记录 |

这些是明确标注的隔离安装／工具验收夹具，不代替 Model Relay 完整产品旅程，也不证明供应商联调。修复后 11 项模板／安装／仓库 Python 回归、ruff 和 diff 检查通过；Go／网页未修改，沿用同提交组件的完整回归结果。8792 升级仍等待当前产品 Run 完成。

### 完整历史分页核查

安装器逐页读取正式运行 API，使用 before=上一页末尾运行 ID；查询按创建时间和 ID 稳定排序，避免分页期间新增运行挤掉旧记录。升级保护与旧工具引用检查共用该读取路径；旧平台忽略游标而重复返回时明确拒绝继续，保留配置。先前达到 200 条即拒绝的保守限制由此替代。

回归先复现缺少分页方法及无法发现第二页活跃运行，再实现。覆盖跨页新增、工作流与所有者隔离、HTTP 游标传递、旧平台忽略游标；13 项模板 Python 测试及 scripts/verify.sh 全部通过（`.data/workflow-evidence/verify-run-pagination.log`）。

独立运行实例经公开 HTTP 入口创建历史夹具：共 204 条运行，最新 200 条均 completed；安装器在第二页找到旧活跃 Run `b3111c10c2a155a9ee38870647eb05a5` 并拒绝升级，游标无重复。使用正式 decision 结束该夹具，无数据库修改。初次夹具试用相同工作区被独占规则正常拒绝，随后为每次运行使用独立目录。证据 `.data/shared-tools-e2e/pagination-proof.json`。这是正式 HTTP／安装器核查路径，不冒充用户页面或完整产品交付验收。

### Model Relay 宿主验证与自动返工

同一产品 Run `c1089fdc410a9dcce0725c30ad190df4` 的首次研发于 seq 6 交接固定 make verify。seq 7 的宿主真实执行中，Go/race、HTTP/SSE、取消、故障、CLI、异常进程终止及备份恢复测试通过，构建通过；真实 Go 服务浏览器旅程 locator.waitFor 超时，完整入口退出 2。图按 failed 出口自动进入 seq 8 development，没有新建 Issue 或 Run。

seq 8 保留全部原断言，修复测试回执仅保留第一行而丢失断言位置的诊断缺口。seq 9 再次由宿主执行完整 make verify，Go 和构建通过，浏览器仍失败；新诊断定位桌面宽度 1280 的“上游已保存”可见断言，未见配置 PUT 请求。系统再次按 failed 自动进入 seq 10 development。研发复现加载中表单提前可编辑、异步回填覆盖输入的问题，修复及后续宿主确认仍按实际节点结果推进。

真实页面点验：当前运行保留两轮 failed 回执及每次研发会话；点击当前“进入 Agent 会话”到 seq 10 会话，再点击“返回编排运行”回到同一 Run。工具卡片按执行顺序显示，已完成调用默认折叠，当前进展持续更新。此观察是升级前运行页和会话页的实际行为，不替代新版每工作流 Runs 入口验收。

以上证明真实项目测试失败能够自动携带回执返回研发；不代表浏览器验收、独立 QA、最终交付或平台升级已通过。私有原始状态 `.data/workflow-evidence/model-relay-current-run.json` 包含各次 Connector 真实退出码和回执；产品工作区 delivery.md 保存对应代码摘要与修复记录。

### 首次完整宿主通过与 QA 恢复

seq 11 的上游保存成功，但旅程在拒绝保存后切换页面时即时读取隐藏表单旧值而失败。seq 12 修正测试等待加载完成的同步，并保留地址恢复和凭据不回显断言。seq 13 完整 make verify 退出 0：前端状态回归、Go/race/真实 HTTP/SSE/CLI/恢复、构建、实际 Go 服务 1280px 和 390px 连续浏览器旅程全部通过。对应工作树摘要为 `97efa7f179cbc32edd3b99e59ee2b9902f12fce46866b9d3847a196f7abcef38`，原始回执 `.data/workflow-evidence/model-relay-host-verify-pass.json`。受控上游通过不等同于真实供应商联调。

seq 14 QA 因原生模型 serverOverloaded 失败；从实际运行页填写继续说明并点击“继续原节点”，同一 Run、seq 14 和会话 `760025d187048d551e662c6e39506352` 恢复执行，前序成功测试未重跑。页面验证仍能进入相同 QA 会话，出现后续真实命令。QA 尚未放行，PR 尚未交付。

### 窄窗口页面修复

在实际 669px 宽浏览器面板发现整页被导航撑至约 847px，右侧聊天内容被裁掉。DOM 确认根因为 grid 子项侧栏的自动最小宽度；维护源给 sidebar 设置 min-width:0，导航自身横向滚动。隔离实例实际点验修复后 669px 会话页 document.scrollWidth=669，聊天卡宽 641；390px 运行详情和所属工作流 Runs 列表 document.scrollWidth 均为 390，链接路径正确。既有真实浏览器回归通过。未改 8792 的在途执行，正式实例随安全升级应用。


## 开发实例升级与完整旅程终验（2026-10-05）

维护源代码版本 `7040597d18365d5a3899d21d1debc6e736d97d47`，分支 `codex/platform-workflows`。独立审查发现历史 Run 所有者失去当前 Workflow 授权后新导航 403；已在维护源修复。回归先复现页面失败，随后 HTTP 用真实登录身份验证“当前图 403、本人 Run/列表 200、他人分组为空”，前端分组与页面测试通过。隔离真实浏览器验收：原授权用户发起并完成审批 Run，管理员撤销其当前图授权后，用户仍从编排总览的“历史运行”卡片进入专属 Runs → 单次 Run → 返回列表；无权打开当前图的链接不再出现。隔离验收服务已关闭，原始记录在 `.data/workflow-evidence/history-navigation-fixture.json`；截图在本地私有证据，不作为仓库资产。

### M1 交付与原实例升级

- 唯一 Model Relay M1 Run `c1089fdc410a9dcce0725c30ad190df4` 已完成，共 21 次节点执行。seq 16 的宿主固定 `make verify` 退出 0，32 文件摘要 `fbc07625a4d9819bba5aa809de5dfea636450cf42489d7d7c72ab66ccba06efe`；独立 QA seq 17 核对同一摘要并关闭 AC-04／15 补测缺口，结论仅限 M1/v0.1 受控验收 Go。seq 19 推送提交 `851c949f9cd1b284b48351e1cc8b5ca1f7b2dce8`；该提交重新计算同一 32 文件摘要，GitHub [草稿 PR #2](https://github.com/big91987/model-relay/pull/2) 为 OPEN/Draft、base main、head 与推送回执一致。原始 Run 在 `.data/workflow-evidence/model-relay-final-run.json`；真实供应商联调仍 Not Run。
- 升级前正式 API 查询 14 个 Run、43 个会话；除专用 `beee6e43aed58ce27470989bab0d1798` 按计划 stopped，均无在途执行／会话。精确核对 8792 服务进程后停止；以文件系统 clone 备份原数据目录（含原生历史）、旧二进制及两个原 manifest，副本数据库 `integrity_check=ok`。使用相同端口、数据目录、管理员账户与 Connector 环境启动当前维护源二进制；健康 200，M1 仍 completed，历史 14 Run／43 会话可读。未操作 8788。
- 经原 `install.py --upgrade` 和原项目 manifest 升级 Model Relay 与 software-delivery，保持各自仓库、工作区、测试命令及 base。两份清单所有原对象 ID 不变，design/development/qa 均绑定同一个 `browser-validation`；项目清单引用同一个共享 manifest。二次执行正式升级后 manifest 哈希、Agent、Workflow、Run 快照全部不变，私有证据 `.data/workflow-evidence/upgrade-repeat-proof.json`。正常新装、旧版升级、活跃 Run 拒绝升级、200 条后旧活跃 Run 检出及两个项目原生 Agent 各自调用共享工具的独立实测证据见上文“共享工具安装与原生链路补验”和“完整历史分页核查”。
- 外部工具主页只突出显示当前 Agent 实际绑定的一个“浏览器验证”；两条旧项目注册在可展开的“未分配给当前 Agent 的连接”中。一条已停用；另一条因上述 stopped Run 的恢复边界仍启用，但当前 Agent 均不绑定它。旧定义和原生会话没有被删除或迁移。当前 Run 的图快照版本 7 保留；当前图版本 8 仍有 intake → requirements/design/development 及 QA → requirements/design/development 出口。共享工具注册本身没有按项目复制实现。

### 页面入口和特殊路由复查

| 场景 | 此轮页面／API／外部证据 | 结论 |
|---|---|---|
| 工作流专属 Runs | 实际浏览器从编排卡片进入 Model Relay `/workflows/fbdbfd95ea6c6992bf45bf697b0769e9/runs`，仅列本工作流 M1；再进单次 Run、独立 QA 会话、返回 Run／列表。software-delivery 卡片进入自己的 Runs，列 4 条本图运行；总览不再混列所有 Run。669px 页面无横向溢出；原生 QA 会话展示 13 张按时间排序的真实工具卡及最终回复。运行页展示 seq 16 测试退出码、seq 17 QA、seq 20 草稿 PR 链接与通知顺序。原实时流接收证据见此前 M1 执行中记录，此轮升级后核验的是历史恢复，不把静态历史说成新实时流 | Pass，UI＋API；工具原生新绑定的实际调用在独立实例通过 |
| 明确 bug 直接开发 | 网页进入旧 Run `82c5788827d042fec63f0518b7e00d72`：prepare → issue → intake → development → tests → QA → publish（首次取消后同一 Run 恢复）→ PR → done，没有需求／设计节点。当前图版本 8 仍有 development 出口；[Issue #11](https://github.com/big91987/agent-platform-workflow-demo/issues/11)、[草稿 PR #14](https://github.com/big91987/agent-platform-workflow-demo/pull/14) 仍可查，PR OPEN/Draft | Pass；真实执行发生在图版本 7，此轮在新 UI 复查冻结历史 |
| 显式开发起点／配置修复 | 页面进入 Run `dfb98adc8b98053d51170129e17a7bef`：用户自然语言“从开发开始”，intake → development，保留准备、Issue、宿主 npm test、独立 QA、同一 Run 的停止返回和 [草稿 PR #16](https://github.com/big91987/agent-platform-workflow-demo/pull/16)，PR OPEN/Draft。测试范围与端口占用恢复见上文 E6/E7 | Pass；图版本 7 原旅程，新 UI 复查 |
| 模糊需求、设计与纠正目标 | 页面进入专用 Run `beee6e43aed58ce27470989bab0d1798`：原会话先澄清，收到“多人云同步、离线冲突”后 intake 选 design；用户纠正“应先回需求”，网页停止并回退，design seq 4 cancelled，requirements seq 5 询问共享规则。Run 按验收计划保持 stopped，不把未实现云同步当通过。旧 MCP handoff、旧会话输入与重复停止均 409，记录见上文 E6/E7 | 路由／等待／旧结果隔离 Pass；云同步实现不在本次任务 |
| QA 与人工回退 | M1 seq 14 QA 将两项 AC 缺口交回 development seq 15，宿主完整复验 seq 16，QA seq 17 Go；另有专用真实 UI 缺陷 Run `85eb42ce941a5007d6c7e5da0b94637c` 经 QA → development → tests → QA → 人工 accept → done。人工拒绝回环与原会话续聊见上文 T2。新版 API 读取这些冻结序列保持一致 | Pass，原层级失败与复验回执保留 |
| 停止／恢复／重启／幂等／外部失败 | T2 原网页停止和恢复同一会话；T4 正式 HTTP 幂等／版本冻结／权限，真实命令停止和强杀后原数据恢复；E6 GitHub HTTP 408 后先核对远端再从 publish 接续；E1–E5 真实 GitHub POST 成功响应丢失只读核对、不重复 POST；E6 通知发送前查询失败后正式 retry。此轮重启 8792 后，原 14 Run／43 会话及已停止 Run 状态不变；二次安装无新对象。外部资源的 OPEN/Draft 状态由 GitHub 另行查询 | 对列出的故障模型 Pass；物理断网及真实供应商调用未测 |

源回归 `scripts/verify.sh` 在最终代码版本完整退出 0（`.data/workflow-evidence/verify-final-7040597.log`），包括 Node、真实浏览器、Python、Go vet/race 和构建；历史授权问题另保留修复前红灯与修复后隔离真人页复验。以上各项按 UI、正式 API、GitHub 外部事实分别表述；隔离夹具只证明对应边界，不代替产品 M1 的真实交付。

**仍未完成的全平台项目**：文件选择／下载到磁盘、阻塞型 before Hook、任意边 Gate、结构化 request_user_input、GitLab、并行分支、循环墙钟限制；它们未纳入这轮已承诺的正常交付和特殊路由验收。供应商联调没有凭据与真实响应，维持 Not Run；不得把受控上游旅程写成供应商兼容性通过。

## 方案二平替的端到端验收纠偏（2026-10-05）

用户确认验收目标是完整研发旅程：从一句话任务自动进入合适阶段，经过真实开发、测试和独立 QA，交付供审查的 PR；合并到 main 后自动准备，再由 Owner 明确手动部署，最后打开稳定地址查看实际版本。此节记录首次部署前的 **No-Go** 纠偏；上方截至草稿 PR 的证据当时不足以证明方案二平替。后续完成结果见文末。

| 必要环节 | 当前事实 | 平替验收 |
|---|---|---|
| 任务按类型进入开发 | 明确 bug、配置修复、新功能和必要澄清已有真实 Run；各自阶段、测试及 QA 证据见上文 | 已覆盖列出的路径 |
| 用户检查与合并 | 模板在“交付完成 · 待人工合并”结束。测试 PR 保持 Draft/Open；尚未从平台运行记录连续验证用户审查、就绪、合并及合并后的状态 | Not Run |
| 合并后自动部署 | 既有 reading_list 方案有独立部署 Workflow；当前 Model Relay 测试仓库没有部署 Workflow，也没有常驻产品部署。临时手工启动的本机 5544 预览不算部署 Pipeline | Fail，核心旅程断开 |
| 部署结果和效果地址 | 8792 的 Run 页有草稿 PR 链接，但没有这次项目部署的版本、状态与稳定效果地址；真实供应商 URL／凭据也未配置 | Not Run |

下一轮必须在可复用维护源与项目正式接入路径中补齐合并后部署及结果回链，至少选一个适配该项目技术栈的真实部署目标。保留人工合并决定，不以自动合并代替用户动作；从用户入口连续核对实际 main SHA、部署制品 SHA、健康检查和页面效果，覆盖部署失败与恢复。真实供应商联调仍单独标 Not Run。未获得这些证据前，不再称整套智能体编排可平替方案二。

### 合并后部署接入进展

维护源新增 `examples/github/model-relay-preview/`：独立 GitHub Actions 在 main 更新后按精确 SHA 准备 Go 服务，运行项目完整 `make verify`；Owner 手动触发部署后，由安装在主机私有目录的可信控制器备份、切换、核对健康版本并在失败时恢复。项目 Workflow 通过版本摘要安装器写入测试仓草稿 PR 分支；平台引擎不内置 Go/SQLite 部署逻辑，不自动合并 PR。控制器隔离测试已验证过期 main 拒绝、重复准备、目标文件漂移拒绝以及失败激活后旧数据与版本恢复。另用真实 Model Relay 提交 `851c949` 在隔离 bare main 和独立工作树运行控制器 prepare，项目完整 `make verify` 退出 0（含 Go/race、实际 Go 服务桌面/窄屏浏览器旅程）；私有原始回执保留在本机隔离验收目录。该阶段只证明准备，尚未经过真实 GitHub Actions 及用户合并。

续验：Model Relay 私有仓库创建 Required reviewer 规则时 GitHub 返回 422（当前套餐不支持）。维护源将部署门禁改为 `push main` 只自动准备、Owner 在 Actions 手动选择 main 并勾选 `deploy` 才激活；`local-preview` Environment 已限制 main，旧的空 Environment 已删除。宿主安装器从维护源安装了可信控制器，源码与安装副本 SHA256 一致；仓库两项 Actions variables 已配置。此时仓库尚无可用自托管 Runner，不能声称 Actions Pipeline 已执行。

Runner 迁移续验：经所有者明确授权，从已离线且无常驻服务的 `he_skeleton` 测试 Runner 正式注销，再将同一 Runner 安装目录注册为 `model-relay-local`。GitHub 官方 API 在目标仓库显示 1 台 `online`、`busy=false`，标签 `self-hosted/macOS/ARM64/he-full`；原仓库的离线注册已移除，未改动仍在线的 `reading_list` Runner。安装控制器的私有仓库 `git ls-remote origin refs/heads/main` 在本机非交互模式成功，证明当时主机具备读取权限；这不是 Actions 作业通过证据。此举只消除了 Actions 的 Runner 前置阻塞；产品 PR #2 仍 Draft/Open 且未合并，main 更新触发的准备、手动部署、部署记录与固定 URL 仍 **Not Run**，方案二平替仍 **No-Go**。

隔离真实服务验证：精确 main 提交 `173670387c9cce44d2780d9d42b74241e993afec` 经控制器 `prepare` 跑完项目 `make verify` 后，由真实 LaunchAgent `activate`；`/admin/` 返回 200，`/healthz` 返回同一提交 SHA，LaunchAgent 重启后版本不变。随后只在隔离 bare 仓库提交 `5c5bf941b1b834f55e427dae0d187dec88da37dc` 注入预览端口启动失败；它通过 `make verify`，正式 `activate` 因健康检查超时失败。控制器保存正式 backup、隔离失败数据，用产品 `restore` 恢复并重启旧版本；`deployed.json` 和 `/healthz` 仍为 `1736703`，旧 LaunchAgent 运行。隔离 LaunchAgent 测完已停止并删除，5545 已释放。这证明控制器在受控失败下可恢复，**不是** GitHub Actions、用户合并或稳定发布的通过证据；方案二平替仍 **No-Go**。

真实 Actions 首次合并验收：用户明确授权测试仓库由验收方自行合并。当前产品 head `820a5f6` 重新运行项目完整 `make verify` 退出 0，工作区无未提交改动；将草稿 PR #2 标记 Ready 后以精确 head SHA 合并，GitHub 记录 merge commit `1bfb3f9e24175e3ac8122beed2010427bafed57c`。合并触发的 [Actions Run 37303366337](https://github.com/big91987/model-relay/actions/runs/37303366337) 在 prepare 步骤因 `git fetch` 无法读取私有仓库而失败；没有部署。根因是原模板依赖 Runner 的个人 Git 认证，宿主交互式 `ls-remote` 通过不能证明常驻 Runner 可用。维护源改为使用 GitHub 自动生成、仅有 `contents: read` 的作业令牌，仅在精确 main fetch 子进程中作为临时认证头；令牌在运行项目验证前从环境移除。失败回归先重现无认证及令牌泄露到 `make verify`，修复后通过；隔离 HOME 的私有仓库 Git 探针验证认证头有效，`scripts/verify.sh` 全仓通过。仍须经 manifest 正式升级项目 Workflow、可信控制器及再次真实 Actions 复验，当前仍 **No-Go**。

认证修复的真实续验：通过正式安装器升级可信控制器（旧副本备份在私有临时目录，安装副本与维护源 SHA256 一致），再由版本摘要安装器升级测试仓 Workflow；[产品修复 PR #3](https://github.com/big91987/model-relay/pull/3) 经已授权的测试仓路径合并，main 为 `8d3998103b89dfe0f04ce89211943bf80c3d62db`。[自动准备 Run 37304881191](https://github.com/big91987/model-relay/actions/runs/37304881191) 在真实 Runner 上成功，`prepare` 运行 2m49s，部署作业按设计 skipped；私有目录 ready SHA 与 main 一致，尚未部署。[Owner 手动 Run 37305305137](https://github.com/big91987/model-relay/actions/runs/37305305137) 的准备步骤再次成功，但 `deploy` 的切换前 main 复核仍调用未经认证的 `git fetch`，明确失败；5545 未发布。第二轮维护源补充切换作业同样使用独立的短期只读令牌，并在服务 init/backup/restore 前清除；失败回归先复现，修复后通过。仍需标准升级和真实 Actions 复验；不得把自动准备成功称为端到端部署通过。

平台交付回链补验：源提交 `88b5a8f` 增加单次 Run 的只读 GitHub 交付查询。仅对该 Run 已保存的草稿 PR 回执及仍有权限的当前 Connector 查询 PR；合并后才按准确 merge SHA 查询 GitHub Deployments 和状态，效果 URL 只在部署状态 `success` 时展示。若 PR head 在原 Run 后变化，页面提醒原 QA 不覆盖新提交。后端测试覆盖未创建 PR、跨用户拒绝、草稿更新及合并后的准确部署关联；前端测试覆盖未合并时不显示效果地址与不安全 URL 过滤，全仓 `scripts/verify.sh` 退出 0。升级前逐个工作流 Runs 页面核对 0 条在途、1 条按计划停止，M1 唯一 Run 已完成；私有备份后仅升级 8792，8788 未更改。升级后从真实浏览器进入 M1 Run，页面展示 PR #2、GitHub Actions 入口、`PR open · Draft，尚未合并；没有部署版本` 以及 `PR 在本次 Run 的 QA 后更新`；只读 GitHub 查询与页面一致。合并／部署状态因未发生仍未验收，不能用该空状态页充当成功发布证据。


### 完整旅程完成复验（2026-10-05）

测试仓库的合并、部署由用户在本轮明确授权验收方执行；平台维护源 PR #5 仍 Draft/Open，未合入 main。Model Relay 原 [PR #2](https://github.com/big91987/model-relay/pull/2) 经全量 `make verify` 复跑后以精确 head 合并，merge commit `1bfb3f9e24175e3ac8122beed2010427bafed57c`。前两次正式 Actions 故障分别是 Runner 读取私有仓库缺少非交互认证、切换前 main 复核未携带作业只读令牌；失败 Run [37303366337](https://github.com/big91987/model-relay/actions/runs/37303366337) 与 [37305305137](https://github.com/big91987/model-relay/actions/runs/37305305137) 均保留。修复先进入平台维护源的受信控制器、测试与安装升级路径，再经测试仓 [PR #3](https://github.com/big91987/model-relay/pull/3)／[PR #4](https://github.com/big91987/model-relay/pull/4) 正式升级 Workflow；[自动准备 37306472116](https://github.com/big91987/model-relay/actions/runs/37306472116) 成功且 deploy 跳过，[手动部署 37306893618](https://github.com/big91987/model-relay/actions/runs/37306893618) 成功。GitHub Deployment `6858456190` 为 `local-preview/success`，SHA `cabe0748fd74af74895117cf567e5a1ca19c5a5a`，服务 `/healthz` 曾返回同一 SHA，管理页 HTTP 200。

为验证已有发布后的更新路径，测试仓文档中“仍是草稿、未部署”的过期状态通过 [PR #5](https://github.com/big91987/model-relay/pull/5) 修正；该 head 的完整 `make verify` 通过，包含 Go race/真实 HTTP 与 1280px、390px 的实际 Go 服务浏览器旅程。精确 head `8cc3bbec3ecca82e31bb912442bfa0df94de20f6` 合并为 main `40b3618080242dd909f46163544bdbbf4c95a61f`。[自动准备 37309982431](https://github.com/big91987/model-relay/actions/runs/37309982431) 成功，部署按门禁跳过；Owner [手动部署 37310396660](https://github.com/big91987/model-relay/actions/runs/37310396660) 的 prepare/deploy 均成功。GitHub Deployment `6859068654` 最新状态 `success`，`environment_url` 为 `http://127.0.0.1:5545/admin/`；真实 `/healthz` 返回 `status=ok` 和该 main SHA，管理页 HTTP 200。正式 LaunchAgent 重启后健康版本不变。更新是测试仓真实 PR/main/Actions/部署动作，文档修改未冒充产品功能新增或供应商联调。

平台 Run 的交付回链在现行 GitHub REST 版本中暴露 `merge_commit_sha` 缺失，原实现因此报错；且原 PR 合并后另有修复提交，部署 SHA 是其后代，不应要求与原合并 SHA 相等。维护源通过 GitHub GraphQL `mergeCommit.oid` 读取合并提交，再以官方 compare API 核对部署 SHA 包含该提交并且部署 ref 是目标分支；错误和超长历史均 fail closed。先有红灯回归，修复后全仓 `scripts/verify.sh` 通过。升级前从各工作流 Runs 页面核对无在途执行、仅一条按计划停止的 Run；备份历史数据和旧二进制、校验备份数据库完整性后只重启 8792，8788 仍为原进程。真实页面 [M1 Run](http://127.0.0.1:8792/workflow-runs/c1089fdc410a9dcce0725c30ad190df4) 展示已合并 PR、原合并 SHA、先失败后两次成功的 Deployment 及当前版本 `40b3618` 的效果入口；点击确实打开管理页。历史成功记录不再把当前固定 URL 误标为旧版本地址。

| 约定的平替旅程 | 最终结论与证据边界 |
|---|---|
| 一句话按类型分派；新功能、bug、配置修复、必要澄清和指定起点 | Pass：本记录前文的真实 Run、冻结序列及升级后页面入口；未重复创建产品 Issue/Run/Agent |
| 真实研发、固定项目测试、独立 QA、QA／人工回退及迟到结果隔离 | Pass：M1 共 21 次节点执行，宿主 `make verify` 成功、QA 先 No-Go 后复验 Go；其它专用 Run 覆盖纠正与恢复；原失败仍可追溯 |
| GitHub 草稿 PR、人工决定合并、main 自动准备 | Pass：平台 Run → PR #2 → 精确 head 合并；真实 Actions push main 只准备；测试仓合并由用户明确授权验收方操作 |
| 手动部署、首次发布、后续更新、固定地址及重启 | Pass：两次真实 Deployment success、两次 `/healthz` 版本核对、管理页打开及 LaunchAgent 重启；平台 Run 页链接当前成功版本 |
| 外部失败恢复 | Pass 于列出的故障模型：两次真实 Actions 失败后从通用维护源修复并按正式升级路径重跑；受控服务启动失败使用项目 backup/restore 恢复旧数据与版本。未做正式已发布实例的破坏性故障注入 |
| 真实供应商 URL／凭据／响应 | Not Run：未提供，不能把受控上游旅程说成供应商兼容性验收 |

此前将上述平台发起的研发与部署切片记为“方案二平替” **Go**，此结论因遗漏 GitHub Issue 接单入口而撤回。上述逐项真实证据仍成立，整体结论以本文开头和下方 WF-17 纠偏为准。

## GitHub Issue 入口纠偏（2026-10-05，WF-17，引入前基线）

核对旧方案 `examples/github/pipeline.yml`：其 `issues.opened` 事件由受信 Runner 调用平台；当前 `examples/platform-workflows/software-delivery.json` 则是先由平台创建 Run，再经 `github.issue_create` 创建新 Issue。两者用户入口和关联方向不同，不能称为研发流程平替。

| 验收项 | 当前事实／证据层级 | 状态 |
|---|---|---|
| GitHub 新建需求／bug 自动接单 | 代码核对：当前编排模板没有 GitHub 入站事件到 Run 的正式接入 | Fail，未实现 |
| 复用用户原 Issue，不再创建新 Issue | 当前模板的 issue 节点调用创建接口；没有原 Issue 绑定入口 | Fail，未实现 |
| Issue 评论补充后接续当前任务 | 当前用户输入来自平台会话，未提供原 Issue 评论接续 | Fail，未实现 |
| 事件重复／未知响应／重启恢复 | 既有 Run 和出站通知幂等有证据，不能外推到不存在的入站路径 | Not Run |
| 其他仓库标准接入 | 现有安装器可配置 repository、workspace_root、base 和测试命令；尚不能安装 Issue 入站链路 | 部分实现，WF-17 Not Run |

本节基于维护源代码和用户契约核对，未创建新测试 Issue、Run 或 Agent；不声称已经跑过新入口。不以增加部署测试或重复平台手工启动补齐本项。


## WF-17 真实入站续验（2026-10-06，过程记录）

维护源 `58216c8` 提供通用 Workflow API 与 SDK 0.2.0；`b2c88b8` 修复真实入站发现的 Git 网络配置、安装继承和 Hook 评论回环。开发实例保留原数据库与历史 Run；所有写入均经 API、原 manifest 安装器及 GitHub Actions，未改数据库。源 PR 保持 Draft/Open，未合并。

- 标准安装：测试仓 [PR #7](https://github.com/big91987/model-relay/pull/7) 合入 main，merge SHA `9b6994ed80de2bdba79ff267419150dccaac1360`。只分发薄 Actions 入口与源摘要，复用既有 Runner。维护源升级在两条 Run 正式停止后进行，重复安装无新增对象／配置改动。
- 真实 bug：[Issue #8](https://github.com/big91987/model-relay/issues/8) 的 `issues.opened` [Actions 37336260987](https://github.com/big91987/model-relay/actions/runs/37336260987) 自动接单，Run `1382a96a9e697c58ba68fe3c3c26f34b`。首次 clone 网络超时，正式配置升级后重跑同一事件成功；重复入口仍返回该 Run。准备节点首次 fetch 超时作为执行 1 保留，经正式停止／回退，执行 2 成功；原 Issue 回执为 #8，未创建替代 Issue。入口按明确 bug 跳过需求／设计，执行 5 进入研发。
- 真实需求：[Issue #9](https://github.com/big91987/model-relay/issues/9) 的 `issues.opened` [Actions 37338122454](https://github.com/big91987/model-relay/actions/runs/37338122454) 自动接单，Run `509e5c927cb88a6bbcefa274656ac6d2`。原 Issue 收到真实澄清，未先开发；评论 `5998327275` 回答展示位置、取值和失败行为。
- 停止／评论恢复：需求 Run 停止时上述评论通过 [Actions 37338660137](https://github.com/big91987/model-relay/actions/runs/37338660137) 得到 409 和原 Issue 失败通知。恢复同一 Run／会话后重跑原事件，消息 `8758` 接收到原评论；没有新 Run 或替代评论。重复提交继续核验。
- 回环缺陷：平台 Hook 标记为 `agent-platform-hook:`，入口原先只识别 `agent-platform:`，真实输出曾误入用户队列，阻止交接。共享适配器现识别两种标记，红灯回归后通过；已有误入记录保留，后续真实开发节点通知不再生成输入。阶段指令明确 Issue 由平台同步，Agent 只在当前会话提出澄清，避免自行调用缺凭据的 GitHub 接口。
- UI：从工作流独立 Runs 列表点击 bug Run，可见原失败、显式回退、原 Issue 链接、自动 development 路由及 Agent 会话入口。API 文档显示 Workflow SDK 和启动／查询／反馈／控制接口。

完整 `scripts/verify.sh` 已通过；随后代理清除边界的相关 23 项 Python 测试、ruff 与 diff 检查通过。以上只证明列出的入口与恢复行为；两个任务的最终测试／QA／PR 尚在执行，不计通过。真实供应商调用未纳入这两个无供应商依赖任务。


### WF-17 最终验收（2026-10-06）

| 真实业务旅程 | 正式执行结果 | 外部交付与版本 |
|---|---|---|
| bug 自动接单、跳过需求／设计、研发、测试、QA 回退、补证复验 | Run `1382a96a9e697c58ba68fe3c3c26f34b` completed，共 14 次节点执行；seq 6/9 完整 `make verify` 均 exit 0；QA 第一次阻断，第二次 Go | [原 Issue #8](https://github.com/big91987/model-relay/issues/8) → [草稿 PR #10](https://github.com/big91987/model-relay/pull/10)，head `8ba11842072b74e03a48562328db1b8f0d6f167e`；35 文件摘要 `edb4224576eb1ef488c33d9e8ab354bdba90a8a39668bb75f0c548a616f7dd48` |
| 新需求自动接单、必要澄清、原 Issue 回复、需求／设计／研发／测试／独立 QA | Run `509e5c927cb88a6bbcefa274656ac6d2` completed，共 12 次节点执行；seq 7 完整 `make verify` exit 0；QA Go；真实 Go 服务浏览器在 1280px/390px 验证版本读取、刷新、失败／重入、非阻塞和文本布局 | [原 Issue #9](https://github.com/big91987/model-relay/issues/9) → [草稿 PR #11](https://github.com/big91987/model-relay/pull/11)，head `943d053d82efb1ba3c3a3b9f7b413b1c89bd095b`；33 文件摘要 `f96017d3daf52e8372e1ef106397bfb31224e049010bb53a6cd033d69f27d414` |

验收方在发布后独立运行两个 checkout 的 `go run ./tests/evidence`，摘要均与对应 QA／宿主测试一致，工作树干净；GitHub API 核对 PR 均为 Draft/Open，head 与 publish／PR Connector 回执一致，正文分别 `Closes #8`／`Closes #9`。原 Issue 的完成 Hook 均已真实送达。浏览器从工作流 Runs 页依次进入已完成 Run，看到原 Issue、节点回执、QA、会话及 PR 链接；API、UI、外部事实分别核对。

恢复与隔离的最终证据：

- 已失败入口重试始终关联原 Issue 和 Run；两个 Issue 仅产生两个 CI 所有的 Run。真实 clone/fetch 失败与正式回退记录保留。
- 停止期间评论明确 409，并回写失败通知；恢复后重跑同一 Actions 接收一次。交接后用旧 seq=3 提交新输入明确 409；用原事件键重试已接收评论，仍返回原 intake 会话的消息 `8758`。两个 Run 完成并重启后再复验，同样返回 `8758`，没有投递到后续阶段。
- Hook 回环修复经真实 [Actions 37338544207](https://github.com/big91987/model-relay/actions/runs/37338544207) 显示 `Platform output ignored`；原误入输入保留为失败历史，后续节点输出未再进入输入队列。
- 标题缺陷在真实 PR #10 暴露：显式 `{{input}}` 模板把多段需求放进标题。共享 Connector 修复后回归覆盖三种 GitHub 创建类型，完整正文保留。已存在的 #10 通过 GitHub 编辑接口修正；#11 由修复后的正式平台自动创建为单行标题，没有手工修标题。
- 所有在途 Run 正式停止后备份并升级到维护源 `a3b91b8d95bc32f906e1a1584c6ca1301f66f239`，二进制 SHA-256 `1914f451e608987e2dd4237e0ad3b83a2c3fa4aeadb6ab4986c79c21a9a38323`。重启前后 16 条历史 Run 的 ID／状态完全一致，最后一个 report 原会话恢复后正常交付。原 manifest 升级重入无变更，未创建额外 Agent／Connector／Workflow。
- 完成态 [原 Issue #8 Actions 37336260987](https://github.com/big91987/model-relay/actions/runs/37336260987) 再次重跑成功，仍返回原完成 Run。最终两个 Run 都 completed，没有 queued/running 消息；CI 使用独立非管理员 caller。

源验证：`scripts/verify.sh` 在共享标题修复后的代码通过，含 Go vet/race/build、浏览器、Python SDK、25 项工作流模板／安装回归。标题回归先在旧实现失败后通过；可选代理的新装／继承回归通过。所有通用修复在同一维护源及标准安装升级路径；旧服务实例未改动。源 PR 保持 [Draft #5](https://github.com/big91987/agent_platform/pull/5)，不视为已合并发行。

边界：此结论覆盖本轮约定的研发入口至草稿 PR；本轮两个产物未合并、未部署，原有部署历史证据不等于新版本部署验证。当前事件入口仅接受个人仓库 Owner；组织多人授权、企业微信／钉钉未实现。第二个真实仓库尚未执行本轮同等外部旅程，仓库可配置与新装／升级边界由安装器测试验证。真实供应商缺 URL／凭据，Not Run；远端写入响应丢失只做隔离恢复测试，不冒充实际 GitHub 断线演练。旧版 HTTP 红灯为修复后隔离重建补证，原始沙箱失败没有改写。旧定时续验保持 PAUSED。


## Skill 产物契约与 Trellis 验收（2026-10-06）

当前结论：**本轮模板／Skill／Trellis 目标通过，交付状态和历史日志收尾缺陷已真实复验关闭**。用户要求模板只约束文档位置、内容遵循 Skill，并允许先临时解除 PR 冲突，再用新 Issue 验证正式修复。以下保留执行过程及当时状态，最终结论和证据以本节末尾的完成审计为准。

- 临时修复：[PR #11](https://github.com/big91987/model-relay/pull/11) 原 head `943d053` 与 main `6f1ec16` 在 task/delivery/qa/pr 四份任务文档冲突，产品代码无文本冲突。人工临时集成提交 `f26590c0589ede73edf38604ba5a415e50f68e15` 保留 Issue #8 文档原文到历史目录、原位置保留 Issue #9 文档；两侧四份文档逐字节核对。完整 `make verify` exit 0，含健康方法、版本展示、Go race/HTTP/CLI 与两个视口真实 Go 服务浏览器旅程。GitHub 确认 CLEAN/MERGEABLE；按既有测试仓授权合入 main，merge SHA `73681a7df4c5cbe5914056ac13348154d58726e0`。这是人工临时方案，**不是 Pipeline 自动冲突恢复通过**；未手动部署。
- 正式维护源 `df527c3`：按 Run 隔离公共任务文档，阶段指令不再定义固定内容模板；发布使用实际 QA 产物，保留旧冻结 Connector 的固定路径兼容；Trellis 0.6.15 官方初始化进入 prepare，研发挂载 before-dev/check/spec-bootstrap/update-spec。`scripts/verify.sh` 全量 exit 0（含 27 项 workflow 示例测试及 Go race），脱敏扫描通过。
- 独立真实 CLI 检查：Trellis 0.6.15 新装、保留已有 AGENTS、重复 prepare 均通过。这不是 Agent 实际使用 Skill 的证据。
- 开发实例升级：通过正式 API 确认零 running/waiting 后备份；原 manifest 升级并重复安装通过，16 条历史 Run 的状态和 seq 一致。独立主实例未操作。备份和完整日志在忽略的本机验证目录。
- 新验收任务：[Issue #12](https://github.com/big91987/model-relay/issues/12)，GitHub 真实 `issues` 事件触发 [Actions 37365376422](https://github.com/big91987/model-relay/actions/runs/37365376422)。待核对实际 Skill/reference 读取、Trellis 规范准备与检查、独立文档路径、原文档保留、测试/QA/PR 与合入结果。不得将已安装或隔离测试当作该真实旅程通过。


Issue #12 首次真实接单：Run `836b817833886b3188f263800b0f092f`，prepare seq 1 已返回独立 document_root、官方 Trellis 0.6.15 初始化成功，Issue seq 2 关联原 #12；intake seq 3 在模型执行前失败：`native Skill scope did not match configuration`。未创建新 Run。定位为原生执行器将 Skill 禁用配置写成目录，Codex 实际按发现的 SKILL.md 文件路径匹配，生成的项目本地 Skills 因此仍启用。隔离的真实 app-server skills/list 复现：目录配置下 12 个仍启用，文件配置下全部关闭；加入同名显式挂载后可仅启用指定源 Skill。修复保留规范化身份比较，但配置使用发现路径；回归先复现相同失败再通过，Go vet、全量 race 测试和构建通过。待升级后恢复原 Run 继续真实验收，当前仍未通过。


原生范围修复版本 `a75def0` 已安全升级开发实例，并通过正式 resume API 接续同一 Run、同一 intake 会话；intake 已真实完成，seq 4 requirements 正在执行。原失败输入仍保留为 failed，恢复输入 completed，未重建 Issue/Run。需求节点真实 command_execution 记录已读取挂载需求 Skill 与 requirements-levels/product-definition-contract 等 references，并准备按 Skill 分别写 PRD/G1；此时尚无研发、Trellis before-dev/check 或 QA 通过结论。

补充回归 `test_two_task_branches_merge_without_document_conflicts`：真实本地 bare Git，两条任务分支从同一 main 起步，分别按不同 Run 根目录发布实际 QA 文件和 PR 正文，再依次合入 main；两份证据保持原文且无冲突。6 项 repository 测试通过。该测试覆盖原固定文档热点的 Git 行为，不替代新 Issue 的 Agent 端到端验收，也不宣称产品源码永远无冲突。


新安装补验：在临时空白平台通过正式 API 创建基础 Agent，再使用原安装器全新安装与重复安装，五项研发 Skill、prepare 的 Trellis 版本参数、publish 的任务目录协议和 PR 正文 Run 路径均核对通过；未启动 Agent、未产生 GitHub 写操作，临时实例已退出。原始回执 `.data/workflow-evidence/fresh-skill-contract-install.json`。

Issue #12 的 requirements seq 4、design seq 5 已实际完成。独立产物为 `product-definition.md`、`g1-review.md`、`password-visibility-architecture.md`、`g2-review.md`，都位于本 Run 根目录；需求及架构 Skill/reference 读取有真实工具记录，10 份原任务文档哈希继续一致。设计依据 Skill 做小型增量裁剪，并单独记录安全专家模块的本地应用。Run 已进入 development seq 6（会话 `edfdc6d47f8bf9181d654aaa19e14d87`）；Trellis 实际实施检查、宿主完整测试、独立 QA 与新 PR 仍待核验，当前目标未完成。


Trellis 升级边界补验：发现原准备 helper 只检查 CLI 版本而未检查项目 `.trellis/.version`，且将 Git Connector 的 `GH_TOKEN` 继承给项目上下文脚本。新增两项失败回归后，维护源校验项目资产版本并在 Trellis 子进程环境移除该令牌；旧版本显式拒绝，文档给出官方 dry-run/create-new 升级路径以保留本地修改。30 项 workflow 示例测试通过；真实官方 CLI 新装、重复准备、原 AGENTS 保留和旧资产拒绝通过。此改动仅影响后续准备调用，不重跑当前 Run 已完成的 prepare，不用手工初始化替代现有记录。


Issue #12 研发与宿主测试续验：development seq 6 已完成实际规范 bootstrap、before-dev、update-spec 与 check。工具事件可核对 Skill/reference 读取、packages 查询、索引与具体规则读取、实现后的差异和规范检查。形成 8 份有源码依据的 Trellis 规范；公共交付保留 README／milestones／tasks／verification 层次，未退回旧固定 task/delivery 文件。研发原生沙箱的完整门禁因监听限制退出 2；27 项前端及非监听检查成功，两类结果分别保留。tests seq 7 通过既有宿主 Connector 执行固定 `make verify`，退出 0，源码摘要 `0f6ee2d6894067323697f632231ee0d62b098be0261a4da7e975edb6e1efb37b` 与最终研发工作树一致。回执文本在 24 KB 截断，完整命令退出码与串行 fail-fast 验证入口可确认门禁结果，不能宣称拥有截断部分的逐项原始输出。

独立 QA seq 8 正在执行，使用验收 Skill 的 acceptance 目录与旅程契约；其自身完整复跑仍受沙箱监听限制，不能混同为宿主成功。旧项目的 health-history 测试会重写固定历史日志，Agent 已将本轮生成证据归档到当前 Run 后恢复旧路径；验收方读取 Git 差异确认原任务文档与全部原日志未改动。这是处理既有项目测试副作用，不是模板继续要求共写固定文档。最终 QA 结论、PR 发布与新文档合入仍待验证。本轮未部署，源 PR 仍 Draft/Open，最新可复用修复提交 `ddaaee9`；该提交跟踪文件脱敏扫描无命中。


### 新 Issue 产物和集成复核

新 [Issue #12](https://github.com/big91987/model-relay/issues/12) 的唯一 Run `836b817833886b3188f263800b0f092f` 已 completed，共 12 次节点执行。独立 QA seq 8 为 Go with known issues，report seq 9 读取实际 QA 产物生成 PR 正文；publish seq 10 正式提交推送 head `15b7c7373978ac1a9e0bfead1c6c8011de158c19`，pr seq 11 自动创建 [PR #13](https://github.com/big91987/model-relay/pull/13)。原 Issue 的阶段与完成 Hook 均已发送，未另建替代 Issue/Run。

| 用户要求 | 当前事实与证明 | 结论 |
|---|---|---|
| 先临时解决已有冲突 | PR #11 临时人工合并保留两项功能及两侧历史文档，完整门禁通过并已合入测试 main；不计自动冲突恢复 | 完成 |
| 编排模板只约束文档位置，内容由 Skill 决定 | 源模板按 Run 指定根目录、通过实际 artifacts/inputs 交接；真实产物包含需求 PRD/G1、架构/G2、研发 README/milestones/tasks/verification、QA acceptance 报告/矩阵/旅程/缺陷表。发布不要求固定 qa.md | 通过 |
| 实现使用 Trellis | 受支持 prepare 使用官方 0.6.15 初始化；真实研发工具记录覆盖 spec-bootstrap、before-dev、update-spec、check 和参考文件读取，产出 8 份源码规范；未仅以挂载当作使用证明 | 通过 |
| 模板修复后开新 Issue 验证 | GitHub issues 事件 → 原 Issue/唯一 Run → 需求/设计/研发 → 固定宿主 make verify 退出 0 → 独立 QA → 平台提交推送/草稿 PR 全链完成；原生 Skill 故障保留并正式恢复原 Run | 通过 |
| 不再覆盖旧任务文档，检查合入冲突 | 验收方发布后逐字节比较 main 基线下全部 79 个原 workflow 文件，无变化；工作树干净，源码摘要与 QA/宿主一致。GitHub CLEAN/MERGEABLE；按用户对测试仓授权合入 PR #13，merge `066f035bc551b2b4d031166cebdab0849084afd4`，Issue #12 CLOSED。GitHub Git tree `6863e426d06cf806bd7997b8604c03f22faf2adb` 在 PR head 和 merge 相同，无额外集成改写 | 通过 |
| 可复用维护源和标准升级 | 修复在源 PR #5，运行代码 a75def0、示例/helper 最新修复 ddaaee9；原 manifest 完成后重入字节不变，17 条历史 Run 的状态/seq 均保留；空白平台新装和重复安装另有真实 API 证据，CLI 新装/旧资产拒绝有真实官方工具证据 | 通过 |
| 用户页面可追踪 | 从该工作流 Runs 列表点击新任务，页面展示 completed/12 个节点、真实测试回执、独立 QA、原失败和恢复会话、PR #13 链接及通知记录；与 API/GitHub 分别核对 | 通过 |

本轮没有把全部工程链路宣称无缺陷：既有测试脚本的 P3 `QA12-HISTORY-WRITE` 仍 Open，测试会写旧日志，本轮由流水线 Agent 归档新结果并精确恢复基线；后续执行仍需该保护。宿主回执截断保留为诊断限制，完整命令退出 0 和同版本源码/未改门禁是通过依据；QA 自己的监听受限尝试仍标 Blocked/Not Run。源 PR #5 未合并发行；测试仓合并由验收方依据既有授权执行，未触发手动部署。自动 PR 冲突修复、第二真实仓库全链及真实供应商联调不在本轮通过结论内。删除的定时续验未恢复。


合入后补验：[main 自动准备 37370705742](https://github.com/big91987/model-relay/actions/runs/37370705742) 已 success，精确 SHA 为 `066f035bc551b2b4d031166cebdab0849084afd4`，prepare success／deploy skipped。受支持预览控制器的该 SHA 私有完整验证日志末尾包含 1280px、390px 的真实 Go 服务浏览器旅程和版本回归 PASS；这份合入后证据补充诊断可见性，但不改写原 Run 截断回执或 QA 受限执行。用户页面交付回链显示同一已合并 SHA，明确尚无包含该提交的部署记录；未触发部署。


完成审计追加发现：PR #13 已合并，但公共任务 T001 仍 In Review、M01 仍 Ready for Review，总览保留“待宿主测试”等当前状态。需求/实现/测试与 QA 证据成立，但交付管理 Skill 的公共事实闭环缺责任人；原 report 只汇总 PR，未挂载该 Skill。不能以 QA 链接覆盖任务文件事实源的过期状态，因此本轮目标重新保持未完成。维护源将研发交付 Skill 同时挂载 report，由其按 Skill 更新公共交付事实，模板只规定职责与位置，不自定内容或强制 Done。待原 manifest 升级和新的真实任务复验；旧历史不手改成成功。


收尾缺陷续验版本 `bc25568`：report 已挂载研发交付管理 Skill，30 项模板/安装回归通过。零在途时备份原 manifest，正式升级并重复安装无变化，17 条历史 Run 保留。额外空白平台新装经正式 agents 列表 API 核对 report Skill，重复安装不变，无 Agent 任务或 GitHub 写入；证据 `.data/workflow-evidence/fresh-report-skill-install.json`。首次检查脚本误用不存在的单 Agent GET 路径返回 404，已改为受支持列表接口重新完成，不将该探针错误归为平台执行失败。

新 [Issue #14](https://github.com/big91987/model-relay/issues/14) 的真实 [issues Actions 37371672475](https://github.com/big91987/model-relay/actions/runs/37371672475) 关联唯一 Run `cf691e39d382439f92a3ac12f5facfff`。prepare 复用 main `066f035` 中已有 Trellis 0.6.15（initialized=false），输出独立任务根目录；intake 按明确验证工程 bug 跳过需求/设计，development seq 4 已开始从根因修复历史日志副作用，当前尚无最终测试/QA/报告状态闭环/PR 结论。


### 收尾修复最终验收

[Issue #14](https://github.com/big91987/model-relay/issues/14) 的唯一 Run `cf691e39d382439f92a3ac12f5facfff` completed，共 10 次节点执行：prepare/原 Issue → intake 直接 development → 固定 tests → 独立 QA → report → publish/PR → done。源维护版本 `bc25568` 的 report 新挂载在真实原生元数据中启用 `managing-engineering-delivery-cn`，工具记录完整读取 Skill、public-delivery-contract 和 delivery-review-checklist。正式流水线自动交付 [PR #15](https://github.com/big91987/model-relay/pull/15)，head `17b21dd99a410f267632d6c3837bc420af25052a`。

- 固定宿主 tests seq 5 的 `make verify` 退出 0；研发、QA、报告与验收方发布后独立执行 evidence 得到相同 36 文件摘要 `b33aa9f6f848a8cda1bf5af08ba9c2e540d0aa09f91090930ccafa6d4e636702`。QA 自己的监听受限执行仍保留 exit 2，不混作本地完整成功。
- 历史日志副作用从项目验证程序根因修复：移除固定历史文件写入，仅输出 stdout 并传播输出错误；不改 Makefile、历史分类器或产品行为。研发和独立 QA 均先对旧实现得到覆盖历史/吞输出错误的红灯，再对修复版得到成功、分类拒绝、关闭 stdout 三种真实子进程绿灯；受控子命令夹具仅证明输出契约，不冒充网络验证。原健康/版本/Go race/浏览器仍由宿主完整门禁执行。QA 将原 `QA12-HISTORY-WRITE` 在本 Run 的缺陷表关闭，旧 Run 原文保留。
- 在宿主正常执行后，验收方逐字节比较 main 基线下全部 106 个原 workflow 文件，全部不变；QA 前后哈希也一致。没有人工恢复旧日志，本次新增任务文档只在自己的 Run 根目录，必要的项目说明和 Trellis 规范原位更新。
- 报告按 Skill 同步了 T001 Done、M01 Accepted、总览 Delivered，明确仅指工程修复/验收/公共说明，并记录报告时点；实际未发生的 Git 发布/合并/部署仍留给对应后续节点。原阶段 Blocked/Not Run 与失败回执保留在验证记录中；当前状态不再等待已完成的宿主测试或 QA。报告更新的任务、里程碑、总览、验证和 pr.md 有真实工具记录与发布差异，不是仅写承诺。
- 验收方检查 PR 为 CLEAN/MERGEABLE、代码摘要不变、工作树干净、新增/修改文件脱敏无命中后，依既有测试仓授权合入 main：merge `d62cd51e8de2127548e1ac7469d349443d1bfe3d`；Issue #14 CLOSED。GitHub Git tree `ca34e9e19daadcbfa0d1dbf6479e35facc84ca4a` 在 PR head 与 merge 完全一致。
- UI 从工作流自己的 Runs 列表进入该任务，实际展示 completed/10 次执行、短路径路由、测试/QA/报告会话及 PR #15 回执；正式 API 确认唯一 Issue #14 Run、共 18 条历史 Run 保留、没有 running/waiting。新装、原 manifest 升级和重复安装已有独立 API 证据，未以直接改数据库或补造回执完成。

最终边界：本轮只接受用户要求的模板位置/Skill 内容契约、Trellis 真实使用及新 Issue 验证与收尾闭环。PR #11 的旧冲突仍是获准的临时人工修复，不计 Pipeline 自动冲突恢复；本轮两个新 PR 均无文档冲突合入。源 PR #5 保持 Draft/Open，未合并发行；没有触发手动部署或真实供应商调用，第二真实仓库/多人入口不在本轮通过范围。原宿主回执文本截断保留为诊断限制，不伪造后段日志；QA 本地网络限制不改写。旧定时任务未恢复。


PR #15 合入后正式 [Actions 37374381315](https://github.com/big91987/model-relay/actions/runs/37374381315) success，精确 SHA `d62cd51e8de2127548e1ac7469d349443d1bfe3d`，prepare success／deploy skipped。受支持控制器的该 SHA 完整私有验证日志给出相同摘要 b33aa9f6…、27 前端测试通过、三个 TestHistoryCommandOutput 子场景通过、1280px/390px 真实 Go browser PASS。该证据属于合入后再次验证，不补造原 Run 被截断的输出。UI 交付回链显示相同合并 SHA 及“尚无包含此次合并提交的部署记录”。最终源跟踪文件脱敏扫描无命中；验收记录与通用改动纳入源 Draft PR #5，目标所要求的修复和复验完成。


## 持续产品演进：首次调用体验（2026-10-06，执行中）

产品目标已扩展为真实团队模型网关的连续交付，参考公开 New API／LiteLLM 行为；本轮 Issue #16 只做首次配置至可追踪调用的纵向体验，后续多上游、应用治理与可复现运维仍为已授权持续目标。维护源计划版本 `ebfe137`；原产品 main `d62cd51e8de2127548e1ac7469d349443d1bfe3d` 经正式 Actions `37389639182` 完成 prepare/deploy，healthz 和浏览器概览显示同一 SHA。无真实供应商凭据，联调仍未测。

Issue #16 由正式 GitHub 入口关联唯一 Run `f1ff775cb6fee4d609353655cae5e180`；prepare 沿用 Trellis 0.6.15、基于上述 main。intake 判断为完整新需求，自动进入 requirements，未误走修复短路径。requirements 形成 8 条 Story／18 条 AC、G1 和公开参考记录，并原位更新项目契约。此为需求产物，不是功能通过证据。

原 Issue 评论 `6005796483` 经入口保存为同一需求会话的输入 `14268`，没有创建第二个 Run。输入尚未处理时交接被拒绝，当前轮结束后自动执行该输入；正式 API 显示原输入 `14200` 和补充 `14268` 都 completed。需求交接包含修正后的长期范围、测试仓合并/部署授权及源仓禁止自动合并边界，Run 已进入 seq 5 design。产品文档也已消除“需要新需求出现才进入后续路线”的歧义；无需手改数据库或要求用户重复发送。

该旅程暴露一项 P3 提示缺陷：交接冲突仅报告 pending input，Agent 曾要求用户再次提供已经入队的内容。源码 `submitWorkflowResult` 的错误提示已补齐正常结束当前轮、等待同会话自动接续、处理后再交接及不重复索取/轮询的指引；状态保护和冲突类型保持。既有排队、交接封口、MCP 范围和入口去重定向回归通过，`go vet ./...`、构建通过。该改动尚未应用到在途开发实例，待本 Run 结束后按正常备份/二进制升级，再在后续真实 Issue 补充接续路径复验；当前不能写成已完成现场关闭。

当前结果：需求及 GitHub 评论接续有真实证据，产品功能/独立 QA/PR/本轮发布仍待执行，长周期整体验收未完成。


设计交接续验：seq 5 于 2026-10-06 完成 G2 并进入 seq 6 development。正式结果交接包含架构、六页等价交互原型、18 AC 矩阵及文档检查；实际文档检查为 7 文件、22 本地引用、18/18 AC 设计行，退出 0。后续持续目标与测试仓授权随输入保留。研发会话已经恢复真实 Trellis 规范；此时尚未取得产品实现或功能验证结论。

部署页面回链补验：从已完成 Issue #14 的 Run 页面点击“刷新部署状态”，查询完成后显示 `local-preview · success`、完整版本 `d62cd51e8de2127548e1ac7469d349443d1bfe3d`、“打开效果地址”及正式 [部署 job 112031459510](https://github.com/big91987/model-relay/actions/runs/37389639182/job/112031459510)。此前页面加载的“尚无部署记录”是部署前快照，正式刷新后更新；未发现需手改状态的部署回链故障。这是上一基线的真实部署证据，不代表 Issue #16 的改进已发布。


研发在途复核与输入接续：固定 `make verify` 首次实际退出 2，历史 HTTP 复现因原生沙箱无法监听本机端口提前退出，后续门禁未运行；单独真实 Go 浏览器入口亦因受控上游监听受限退出 1。另一分项命令 format-check / vet / frontend-test / build 退出 0，其中 41 项前端检查通过；三个新增 Console handler 检查通过，但分项 Go 命令仍因崩溃恢复监听受限整体退出 1。各事实分别登记，不能把分项或受限执行当作宿主完整成功。

维护者将当前实现与既有设计逐条对照，发现工具结果摘要测试只断言“无文本输出”，弱于 AC-10 的明确摘要要求；会话检查失败时也未按架构契约清除敏感内容。通过原 [Issue 评论 6006569509](https://github.com/big91987/model-relay/issues/16#issuecomment-6006569509) 回传两项具体偏差及相应验证要求，没有直接修改产品实现或验收断言。正式 API 已核对补充输入 `15186` queued 到当前 development 会话 `577f36008c3136cc444ca8b9e1d58795`，原输入 `14997` 仍 running，Run 仍 seq 6。此时只证明评论进入正确的在途会话；修复、补充输入完成、宿主复验与 QA 均尚待发生。


在途补充接续复验：development 的交接因输入 `15186` 尚未处理而被正式拒绝，未推进 tests；原输入 `14997` 随后 completed，同一会话自动领取 `15186` 为 running，Agent 明确开始补齐工具摘要和会话检查。本次没有要求重发、没有新 Run，也没有数据库修补；仍使用原运行二进制，因此不能用此结果宣称尚未部署的提示改动已复验。用户页面从每工作流 Runs 列表进入本轮，再进入研发会话，可见原补充、handoff 失败记录与自动接续内容，UI 与 API 状态一致。另逐字节核对当前基线的 131 份历史 workflow 文件，无一改变；该结果为在途快照，发布后仍需最终复核。产品修复和宿主完整测试尚未完成。


补充输入 `15186` 已 completed，同一研发回合归档了工具摘要、可见长流30秒检查和异常清理实现；最终分项记录包含48项前端检查通过，但完整 `make verify` 与真实浏览器入口依然因原生沙箱监听受限未通过。不能据此关闭所有会话边界。维护者在现有计时器夹具中对真实前端脚本做隔离诊断：已知45秒到期，30秒的session检查保持pending，移除已触发的一次性timer后推进到46秒，结果无剩余session timer、未取消生成、未清应用密钥，期望清理的断言失败。该证据只证明组合缺陷，不是浏览器/供应商验收。

该缺陷及普通检查成功时误导性的恢复提示经原 [Issue 评论 6006907755](https://github.com/big91987/model-relay/issues/16#issuecomment-6006907755) 反馈，正式输入 `15601` 已在同一会话 running。前一次交接被未处理补充保护拒绝后自动接续，Run 仍 seq 6；研发正实现独立到期清理与一次性timer回归，维护者没有直接补产品代码或替换门禁。修复及宿主/独立QA关闭仍待核对。


到期缺陷隔离复核：研发新增独立到期计时器后，维护者用原诊断场景复跑真实前端脚本，退出 0；会话检查悬挂且到期时已返回登录、取消生成并清除应用密钥。此红转绿仅覆盖确定性前端诊断，宿主完整门禁、真实浏览器及独立 QA 尚未完成。

执行指引改进：本轮研发反复在已知、未变化的原生沙箱监听限制处重跑固定门禁。维护源 development 指引现在明确记录命令/版本/退出码/受限范围、完成可运行检查后交宿主，不因代码或文档调整重复执行必然受同一环境限制的命令；代码/用例/依赖失败仍须修复复验，环境变更须重新核验。宿主完整门禁与 QA 必经路由保留。README 的示例验证命令缺少 SDK 模块路径，首次执行因导入失败未通过；按正式 verify 脚本补齐 `PYTHONPATH=sdk/python` 后30项回归通过，包含安装重入/漂移、在途升级保护、原Issue入口、必经测试/QA路由。指引通过标准安装器分发，待无在途时原 manifest 升级及下一真实 Issue 复验；不能称本 Run 已应用该改动。


宿主失败诊断缺陷（In Progress）：Issue #16 的 seq 7 于 2026-10-06 01:02–01:04 UTC 真实执行固定 `make verify`，源码摘要 `9254855737fbe400acf1d8c3a038767ca7807e0e1ea43160b7b5ed87d5456d0a`，退出 2。旧平台只保留前24 KB，输出全被预期的历史红测占用；末尾实际失败原因缺失，不能将历史红测当作当前产品缺陷。图正确沿 failed 边进入 seq 8 development；返工 Agent 复核后等待失败尾部，没有猜测修复或放宽门禁。

维护源补齐命令回执的有界首尾保留，并在截断前流式脱敏配置凭据；短输出、UTF-8 边界、真实退出码与既有取消/副作用规则保持。新增测试先在旧实现确认大输出丢失失败尾部，再在修复实现通过；真实子进程测试覆盖 stdout/stderr、非零退出、截断边界凭据，隔离写入测试覆盖不同分块及最终未满块脱敏。定向 race 回归通过。该证据仅证明通用日志实现，不是产品门禁通过。

Run 在 seq 8 waiting 后经正式 stop 入口进入 stopped；确认没有执行中 Run 后，优雅停止开发实例并私有备份运行数据、原二进制、产品工作树和安装清单。准备执行完整平台门禁、二进制升级、原 manifest 升级与同 Run 恢复。原 seq 7 回执保留原样，缺失日志不能补造；真实失败尾部须由后续正式命令重跑获得。

平台完整 `bash scripts/verify.sh` 退出 0：前端/浏览器、SDK、GitHub与预览安装、工作流30项回归、Go vet/race/构建全部通过。macOS链接器报告既有LC_DYSYMTAB警告，但测试与构建成功；changed-file脱敏扫描4文件及diff检查通过。以下现场升级和新命令结果另行登记，不将单元/集成测试等同现场关闭。

现场升级完成：维护源提交 `d5e1403481f8d761d49eb0286e1f8267b89a087d` 已推送源 Draft PR #5，源主线未合并；以该干净提交构建并于01:19 UTC启动开发实例，构建摘要存私有升级记录。原 manifest 升级及重复升级均成功，Agent/Connector/Workflow对象身份保持；正式API对照备份确认原Run seq、冻结图、Connector和全部历史steps逐值相等。通过resume恢复同一seq8、同一研发会话 `789d45c3f91817bd78abd129c1f7726b`，输入16587 running；页面显示第8次执行的研发节点进行中。Agent已明确不重复稳定的监听受限检查，准备核对原工作树并交接宿主。命令首尾日志实际复验、产品失败定位与QA仍待下一正式执行；提示改动和新版模板在新建真实Issue中的效果仍待验证。

命令诊断现场复验通过：seq8正式交接后，seq9以原固定 `make verify` 重跑相同56文件摘要 `9254855737fbe400acf1d8c3a038767ca7807e0e1ea43160b7b5ed87d5456d0a`，真实退出2。新回执保留24053字节并明确中段省略，开头包含原源码指纹，末尾包含Go测试成功、构建及浏览器实际失败：1280px的password hidden检查在`journey.mjs:43:10`触发AssertionError，随后make browser/verify传播退出2。未推测中间省略的逐项结果。用户Run页展开seq9“调用回执”后可见同一省略提示及最终错误，API与UI一致；旧seq7回执仍为旧格式。失败边自动进入seq10 development会话 `49b4a9ee3cf82e66bda1c5f20abf1b7e`，Agent开始区分密码显隐实现和原生键盘测试问题。通用日志缺陷已在原故障场景关闭，产品浏览器缺陷尚未修复/验收，整体验收继续No-Go。


大历史标准输入缺陷（In Progress）：seq10在保留密码显隐全部断言的前提下增加逐次原生输入事件等待及白名单诊断，经58项VM与非监听分项检查后交接seq11。该宿主执行约121秒后被平台标failed，错误为`write |1: broken pipe`，回执只有command kind，没有退出码/输出，不能判断本次项目门禁是否通过。根因是平台在Wait前同步写入nonce和全部历史JSON，固定make验证命令不消费stdin，历史增长填满管道，命令退出触发EPIPE后走提前返回路径，丢掉实际结果。

维护源回归先真实复现不读取大输入的非零命令回执丢失、取消时中断语义被pipe错误覆盖；读取大输入的cat对照正常。修复将小的监督启动握手与上下文交付分开，后台交付JSON同时Wait收集真实退出码和日志；命令忽略或关闭可选输入时不覆盖真实结果，取消/未知副作用保护保持。定向Connector race回归通过。seq11已通过正式stop停止，确认无在途执行后备份并停止开发服务，准备完整门禁、升级和检查后显式返回tests；不补造seq11日志或自动隐式重放。

补充根因量化：按正式请求字段重建的紧凑UTF-8上下文估算，seq9约54,229字节，seq11约83,183字节，与跨过管道容量后出现阻塞的机制一致。修复后完整`bash scripts/verify.sh`再次退出0，Connector专门回归及平台全部门禁通过；4文件脱敏/diff检查无问题，现场新命令回执仍待升级复验。

大输入缺陷现场复验通过：维护源 `6e87503408f727865feb729356b8d7de07ba1348` 已推送源Draft PR并构建，01:40 UTC开发实例升级健康，原manifest同步成功。停止快照对照确认原Run/冻结图/Connector/历史steps在返回前逐值不变。核对固定验证入口和锁文件未改、浏览器数据使用隔离临时目录后，通过正式return tests创建同Run seq12；seq11保留broken pipe并追加显式返回理由，不伪造成功或隐式重放。UI显示第12次测试与原错误/返回原因。

seq12上下文包含更长历史，仍正常保存实际退出码2及首尾回执，源码摘要与seq10交接的 `4387d20a8cfcbbbe57f01d67541a901de7d8d0279377cf6b21cf1d3a25c7208c` 一致。末尾包含Go包测试成功和浏览器失败：1280px密码显隐受控输入第8次click，实际type=text、toggle事件9、submit事件1、焦点不在输入或切换按钮，原断言失败；未输出密码值。按failed边自动进入seq13 development会话 `71d3339aac3e03e7c2bf2bec32834826`。平台大输入回执丢失缺陷已在同Run正式重试关闭；产品显隐/测试事件原因仍待修复，独立QA、PR和本轮发布未发生。另在seq11执行期间逐字节核对基线全部131份历史workflow文件未改动，发布后仍需最终复核。

产品浏览器返工续验：seq13分析确认seq12的passwordProbe沿用了最后一次显隐操作标签，toggle=9/submit=1包含随后预期的错误密码登录，不是多触发证据。旧旅程等待任意“请”字，先前会话提示已满足条件，导致真实登录响应前检查清密。研发改为延迟并放行真实Go登录请求、核对401与本次明确错误提示后检查清密，保留原显隐/AX/焦点断言，补充59项VM及规范。seq14以摘要 `37b9d52f1d2458e5333a4675648ca28c13a513af8eeb094a37120a46d78de861` 实际退出2，1280px已越过登录与runtime version旅程，随后在通用keyboardAction焦点断言失败；390px尚未到达，不能将局部推进当全链通过。

seq15正式返工将程序化focus改为等待目标可见后真实Tab导航到达，再断言实际焦点、focus-visible及轮廓，保留有界不含值的诊断；无产品实现、固定门禁或锁文件改动。59项VM与无监听分项通过后正式交接seq16 tests，当前执行中，实际浏览器结果待定。两项平台回执缺陷已现场复验；本轮产品仍No-Go，独立QA/PR/合并/部署尚未发生，后续路线2–4不提前启动。


首次完整门禁通过与独立QA拦截：seq16真实退出2，1280px完整旅程已通过，390px在登录后概览断言失败。seq17定位为两屏复用同一受控服务时，窄屏已具有宽屏成功调用证据，旧断言仍等待空配置“上游”提示；修正为等待本次真实overview响应并对照空配置/已有成功配置的数量及请求身份，不重置数据库、不修改产品文案绕过断言。60项前端检查后正式交接seq18。

seq18于02:08–02:11 UTC运行原固定`make verify`，真实退出0；56文件工作树摘要`dc9990248951dbfa54fda356975ffbef0dcfc13fef4add95e4a001eda74d69f4`，基线HEAD仍`d62cd51e8de2127548e1ac7469d349443d1bfe3d`。回执尾部保留Go/race包通过、构建以及1280px/390px实际Go服务浏览器旅程通过。首尾之间明确省略，不推导省略段逐行输出。维护者独立计算得到相同摘要，逐字节核对基线131份历史workflow文件未改。宽屏从业务空配置全UI完成首调用；窄屏复用宽屏留下的受控数据继续旅程，不描述成两次独立空库首配。受控上游仍不是供应商联调。

工作流自动进入seq19独立QA会话`09a9b09a2350cd819d18412b4296cab5`，页面与正式API均显示测试退出0、QA执行中。QA实际读取挂载验收Skill及其引用，核对版本和逐项AC，并在独立前端契约入口复现4项失败：上游保存被拒绝后秘密未清；写入成功但随后读取失败未区分已提交事实；恢复会话后概览成功证据未带入同请求定位；应用调用被拒绝后密钥/消息未清。第二轮隔离测试退出1，保留首轮夹具字段修正记录；这些是确定性契约复现，不冒充原生浏览器失败证据。另记录全程键盘和启停后恢复等尚缺真实UI证据。当前QA报告正在形成，返工交接/修复复验待发生；完整门禁绿色不等于独立验收放行。本轮产品及长周期目标继续No-Go，尚无本轮PR/合并/正式发布。


QA正式回退续验：seq19于02:27 UTC完成，route=development，交接携带实际存在的报告、18AC分层矩阵、连续旅程、整改包和新增失败入口/日志；其最终检查110本地引用无缺失、指纹仍dc999024…，没有改业务或放宽基线。缺陷归并为QA-D01失败清密/P1、QA-D03同请求动作/P1、QA-D02写读反馈/P2，另QA-G01为必测UI证据缺口；4条Contract红灯不混同4类缺陷或真实浏览器红灯。系统自动启动同Run seq20 development会话`cb07b046736e1a626bbeb789b6483a87`，研发明确开始恢复QA证据与规范。用户页面刷新后显示第20次节点执行、QA的development交接路径和新会话入口；GitHub原Issue评论`6008080388`同步No-Go结论，`6008080760`同步研发启动，未新建Issue/Run或人工重发整改。该证据证明实际独立QA拦截与自动研发回退/通知闭环；修复、新宿主门禁、再次QA及本轮发布仍未发生。


QA返工后正式门禁续验：seq20完成清密、写读分离、概览独立精确动作与两屏键盘/依赖恢复/异常/分页回归，74项VM、3项Go race契约及分项检查通过；原QA脚本未改，其中01/02/04转绿，03仍因驱动泛导航而非新增独立证据动作失败，研发明确留给独立QA核对，不伪称原4项全绿。维护者重新计算56文件摘要`6cdd533c1dd449f9aa28dcf1422228c399e94ed2a589175f465ff16839cd7a52`，与正式交接及后续宿主一致；基线131份历史workflow文件仍逐字节不变。

seq21于02:46–02:49 UTC原样执行`make verify`，实际退出2。回执尾部包含当前Go包/race测试与构建通过，1280px runtime version旅程通过，随后在新键盘复制密钥场景等待notice“已复制，请保管”超时；390px及后续新增完整旅程未完成。回执包含安全的checkpoint/调用路径/状态码，没有密钥值。系统按failed边自动进入seq22 development会话`785d83caf61ab8a0d08ce205f57fdcc4`，未创建新Run或人工改状态。本轮仍No-Go；复制失败属于环境、产品反馈还是用例不当尚待研发诊断，不依据超时预判根因或放宽断言。


剪贴板用例返工与继续失败：seq22明确原回执不能确证OS拒绝或产品根因，确定性缺口是用例未声明浏览器能力却只接受成功。原项目journey增加显式注入拒绝→手动恢复提示/重试，恢复原API后仅对临时服务origin授予隔离浏览器context读写权限，真实键盘触发并在页面内boolean比较实际readText，finally清空受控值并撤销测试权限；不是产品权限修改，也不把注入拒绝当原生OS拒绝。74VM及分项通过，正式交接新摘要`7e9d396e8fa48ff4386fe89c9e81f44068d6b7dcb368bb57ddc31a17f10dc826`。

seq23于02:55–02:57 UTC固定门禁实际退出2。当前Go/race/build通过，1280px越过剪贴板成功/拒绝恢复，随后上游错误恢复等待“上游地址不符合”超时，尾部保留真实PUT/GET和200/400/503状态。390px及剩余新增旅程未完成。自动进入seq24 development会话`da1d3010553d2256342d4dc5e63cc8d4`；研发定位为“重新读取”只等旧凭据提示，未等本次读取返回，使后续草稿可能被迟到读取覆盖，正在修正本次响应屏障，未放宽原错误断言。

维护者继续按既有交互设计§3核对成功行，发现当前UI仍缺调用时间和上游配置版本；后端call_evidence已经提供time/upstream_revision，不需新对象/迁移。通过原Issue评论[6008424387](https://github.com/big91987/model-relay/issues/16#issuecomment-6008424387)补充权威来源、真实字段和无证据/失败隔离要求，明确仅静态对照发现，未冒充浏览器红转绿。正式入口已将输入19186 queued到同一seq24会话，原19131仍running；不重复发送、不重建Run，等待当前回合结束后接续。新完整门禁、补充处理与独立QA仍待执行，当前No-Go。


待输入交接提示现场关闭：seq24完成重读屏障返工后尝试handoff(next)，事件57136于03:03 UTC被正式保护拒绝，运行未越过未处理输入。当前升级代码返回明确指引：输入已保存、正常结束本轮后平台在同一会话投递、处理后再交接、不要索取重发或轮询handoff。Agent最终回复如实说明交接未被接受并正常结束，未要求维护者重发；API核对原19131 completed、补充19186 running，同一会话`da1d3010553d2256342d4dc5e63cc8d4`、同Run seq24自动接续，Agent已开始补齐证据时间和配置版本。此为此前`10ee4d9`通用提示修复在已升级开发实例上的真实复验，关闭该提示行为的现场待测；不是人工注入提示、数据库修补或新Run。新版阶段模板在下一独立Issue的安装/使用仍需按原计划复验，产品修复及新完整门禁尚未完成。


补充接续与两屏续验：输入19186在原seq24会话完成，实现概览调用时间/配置版本仅取该次call_evidence；修复前77项VM中2失败保留，修复后77/77通过，并扩展真实UI与同ID精确记录的time/revision比较及未知/配置变化/失败清旧值。原19131重读屏障修复保持，补充处理后才正式进入seq25 tests；新56文件摘要`7f6faaac87ad3501cf76108b850db068439f0c6a47c8e10003e5caa4694775c9`。

seq25于03:09–03:12 UTC固定make verify真实退出2。当前Go/race/build通过，1280px新增QA恢复、旧成功精确定位、started/unknown/404/明确注入503、原生分页及整条真实Go旅程通过；390px版本旅程通过，随后`keyboardModel`在journey.mjs:161模型选择值断言失败，未完成窄屏全链。回执checkpoint仍是前一个通用keyboard动作，不将其误诊成焦点失败；实际定位由对应源码行核对。系统自动进入seq26 development会话`843f8c92efe627e1be173adf912c7eb1`，继续原Run。宽屏推进不能代替两屏验收或第二轮QA；本轮仍No-Go、尚无PR/合并/正式发布，后续路线2–4仍待执行。


原生模型选择续验：seq26仅修改项目浏览器选择helper及诊断/契约测试，保留真实Tab、焦点可见、选项启用和精确值断言；81项VM/helper检查通过，不能证明原生弹出选择行为。seq27对新56文件摘要`9ebdb69472beb0ffb66565f1ec6d3bd47ea0d76b573fdb6338377ce5cc00a6f6`运行固定make verify，真实退出2。Go/race/build、1280px完整新增恢复/分页旅程及390px版本旅程通过，390px选择目标仍失败；新modelProbe明确显示after commit、optionCount=4、targetIndex=1、selectedIndex=0、targetEnabled=true、matchesTarget=false。该证据排除目标缺失/停用，不能单独证明OS弹出窗口机制；不以模拟契约转绿关闭原生故障。系统自动进入seq28研发会话`936239399cac5aa89eab92180320968f`，页面与API一致，同Run历史保留。

维护者将既有交互设计的收尾静态核对通过原Issue评论[6008669722](https://github.com/big91987/model-relay/issues/16#issuecomment-6008669722)提交：§4撤销上游公开草稿时清未保存凭据且零写入、§7进行中参数保持可见且不可改写、§6使用真实created显示密钥创建时间，以及§1行启停重绘后的真实键盘焦点。前三项源码对照存在缺口；焦点仍须实际复核，不凭静态推测报原生失败。允许等价交互，不按线框按钮数量新增缺陷，明确QA-03旧泛导航驱动与新独立证据入口须独立裁定。输入20067已经queued到seq28原会话，20049仍running；补充接续、修复后门禁与再次QA尚未发生。本轮及长周期目标继续No-Go，不提前创建后续路线Issue或发布。


四项交互补充正式接续：seq28首次输入20049完成闭合选择修正，handoff事件60198因已保存输入而被拒绝；Agent正常结束，未索取重发。补充20067随后在同一会话自动running，GitHub评论6008671606确认原会话关联，6008717201同步未交接事实。研发据原交互契约修复上游保存/恢复共享忙态与清凭据、当前调用四参数锁定及旧finally隔离、同ID密钥真实created、同对象行操作焦点恢复；pagehide/会话不明的忙态清理也纳入生命周期检查，不增加对象或迁移。

当前101项VM/helper及3项Go in-process race契约通过；四项实现前未捕获红测，研发报告明确未补造。原QA脚本一度因项目helper中的import.meta进入其历史fixture前缀而SyntaxError，失败回执保留；仅调整项目测试布局，原QA文件与断言未改。恢复执行后仍3/4，QA-03泛导航与独立精确入口差异仍待独立QA裁定。真实Go两屏旅程补入恢复零PUT/即时清密/共享忙锁、进行中原值与禁改、真实GET created和行启停后即时焦点断言，尚不能据VM称原生通过。

维护者独立计算当前56文件摘要`475091a58b28ca779261cb8c1dd24a55118d9c74df1c4d1be8fb5aa4fc82c63a`，与source-closure.json一致；基线131份历史workflow文件仍逐字节未改。GitHub main仍d62cd51，开放PR列表为空，正式部署入口仍先prepare再显式deploy；未提前合并或发布。上述对象完整宿主门禁及第二轮QA待执行，本轮与总目标继续No-Go。


seq29于03:40 UTC进入宿主固定make verify，回执开头确认上述475091a5…摘要，真实退出2。Go/race/build、1280px含新增交互补充的完整旅程通过，390px运行版本通过但模型选择仍在journey.mjs:168失败：targetIndex=1、enabledTargetIndex=1、selectedIndex=0、targetEnabled=true。闭合方向键方案未在该场景解决问题，不能把101项模拟检查或宽屏首项成功视为原生选择修复。用户Run页与API均曾显示第29步外部执行中，实际make和browser进程存活；完整回执正常保存后按失败边自动进入seq30研发会话`830b0ed4ff74253c1cd4beb9feae4d1a`，未因等待重启或丢失大历史结果。

维护者通过原Issue评论[6008903077](https://github.com/big91987/model-relay/issues/16#issuecomment-6008903077)要求先补原生逐边界诊断：同select实际焦点/是否重绘、离焦前后索引、导航读取与标题焦点的时序，再按证据修复，不再仅凭模拟中定义的按键行为更换组合；导航/焦点竞争明确是待证假设。输入20797已queued到seq30原会话，20754 running。两屏精确值、焦点与后续真实路由断言保持，不能改用程序化selectOption或删掉窄屏场景放行。第二轮QA及本轮PR/合并/部署仍未发生，总目标继续No-Go。


重复失败诊断的通用模板补强（源已验证，现场待升级）：根据seq25/27/29同一窄屏选择状态持续失败且多个模拟helper检查转绿的实际经历，在唯一维护源development阶段指令增加“先取得能区分原因的最小诊断，再据证据修改”。要求区分产品、测试驱动、异步时序与环境；模拟定义不能证明原生机制，诊断保留原层断言、限制输出并进入固定宿主入口，不扩大权限或弱化测试。README同步新装与原manifest升级路径；没有向通用引擎加入业务规则，也没有硬编码该Issue或控件。

现有平台工作流30项回归全部通过（3.739s），包含安装授权/配置保留、在途任务保护、文档隔离与强制QA路由；未增加断言提示词常量的测试，也不把这些回归称为Agent行为现场通过。当前Run仍在执行，故未升级共享Agent定义；待该Run结束、无在途执行时通过原manifest同步，再在下一条真实产品Issue中验证新版阶段策略。当前原Issue补充仅证明已有输入接续，不替代新版模板安装和行为验收。


诊断载体补充：seq30在原会话自动开始处理输入20797，撤回未经宿主验证的别名键入方案，先观测既有闭合方向键过程。当前草稿40条完整白名单trace按字段估算约13.1 KiB，已超过命令尾部12 KiB，外层信息还会增加长度；这是静态容量量化，不是原生根因。维护者通过原Issue评论[6008989056](https://github.com/big91987/model-relay/issues/16#issuecomment-6008989056)补充要求保留关键阶段的有界摘要，必要时完整白名单证据存既有忽略验证目录并交接路径；输入21094已queued至原seq30会话，20797仍running。通用development模板与README同步此证据传递规则，不扩大平台日志上限，不修改在途定义；仍待本轮结束后的原manifest升级和下一真实Issue验证。


seq31诊断门禁于04:07–04:10 UTC执行，56文件摘要`11c3732c72f61b19512bcf83523c87d2c5dd35bc4b971016b370177bdc9b0067`，真实退出2。当前Go/race/build及1280px完整旅程通过，390px模型选择仍失败。8 KiB预算内摘要完整保留关键边界，并指向execution96972的白名单JSONL；维护者独立核对窄屏行共21,986字节（含换行）、SHA-256为`92132f4369afafdd97177e5f9fde9b83e7ced900add813d9a9c9d924daba5515`，与正式回执一致。证据载体没有再因尾部截断丢失关键阶段，但该事实不等于产品通过。

此次原生观测中目标索引1且启用，所有方向键边界实际焦点在同一select、documentFocused=true，控件及选项身份不变，重绘/替换计数0；navigation12/load13稳定，models14/14、overview17/17读取完成且无在途/失败。ArrowDown的keydown/keyup已到达select，索引仍0，Tab正常移走焦点。因此该复现不支持先前失焦/读取覆盖/重绘竞争假设，转向测试所假设的原生选择机制；不将浏览器实现推测写成已确证根因。系统自动进入seq32研发会话`525cd5bee3ab13740c928f441151fa47`，同Issue/Run处理。研发已归档诊断并改用原生文字选择，108项helper/VM契约通过；新对象完整宿主测试、第二次独立QA及发布仍待完成。整体继续No-Go。


seq33完整宿主门禁通过：04:17–04:21 UTC，原样make verify真实退出0，当前56文件摘要`1b2f7026692d0633f6f050d585cec0bb67571a7ecf411848d8213d2fd609cdc6`与研发交接及维护者独立计算一致。回执保留Go/race/build通过，以及1280px和390px运行版本、QA恢复、旧成功精确定位、started/unknown/404/明确注入503、原生分页及完整真实Go服务旅程通过；仍是受控上游，不是供应商联调。当前原生execution98536的390px首个非首项选择实际targetIndex=1、selectedIndex=1、matchesTarget=true、同控件/选项及documentFocused=true；对应完整行17,982字节（含LF）、SHA-256 `f68ef1b18ecc610733b0424271ebea9d9011a138a2e5b554a539a8ea5bbff9a1`。这证明修改后的实际键入路径完成了选择，不据此声称所有OS箭头机制已定位。

维护者逐字节核对基线131份workflow历史文件仍未改动。系统无需人工派发自动启动同Run seq34第二轮独立QA，会话`6da434e991029b15651eea1b291ad234`，已开始恢复验收契约、首轮缺陷及当前宿主回执。第二轮QA尚无结论，QA-03旧驱动与独立精确入口语义仍需裁定，本轮PR/合并/正式部署未发生；完整门禁绿色不等于第一切片发布通过，更不等于路线2–4或长周期目标完成。源通用模板升级继续等待本Run无在途执行后经原manifest应用。


第二轮独立QA于04:37 UTC正式完成并自动交接：seq34 route=next，进入seq35 report会话`f6fd7365acdc3f28d83fd9d623a10230`。最终56文件摘要仍`1b2f7026692d0633f6f050d585cec0bb67571a7ecf411848d8213d2fd609cdc6`，QA新增report-02、18AC分层matrix-02和journeys-02，175本地引用、语法/diff、固定门禁/锁文件不变和最终指纹检查通过。QA实际独立108VM/helper、3项Go race与5项明确证据动作契约通过；用同轮宿主seq33真实两屏同层证据核对UI，不将QA沙箱未运行浏览器改为Pass。

QA-D01/P1失败清密、QA-D03/P1概览同ID、QA-D02/P2写读反馈和QA-G01覆盖缺口正式关闭。旧QA-03把泛records导航当概览明确动作，原脚本仍3/4退出1，不删除或改绿；新增独立动作5项与真实UI精确GET/刷新/重登录/旧ID分页证明所需用户结果，报告明确裁决差异。首轮No-Go、夹具异步应答失败、zsh只读变量导致的收尾命令失败均保留，最终独立命令退出码另存。公共T001–T003 Done、M01–M03 Accepted仅指第一条实现与验收切片，不是Published。当前Go允许继续报告/PR，本轮合并/正式部署与部署后体验仍Not Run，总目标继续未完成。

发布准备只读核验：既有model-relay-local Runner online/idle且标签满足原Workflow；产品main仍d62cd51，开放PR为空。维护源部署README澄清已获授权维护者可以通过正式workflow_dispatch设置main/deploy=true，不必要求用户亲自重复点击；Owner校验、main限制、固定门禁与显式发布机制不变，测试仓授权不扩大到源仓。该文档修订已纳入源Draft PR，产品已有对应授权说明，无运行配置变更。新版通用阶段模板仍待本Run结束后经原manifest升级。


报告收尾的通用模板修正（现场待升级）：seq35恢复公共任务时发现多个“当前独立QA状态”标题及任务/里程碑状态混用，虽原证据真实，接手者仍需判断哪个当前。维护源report指令和README明确：公共总览、任务包、里程碑各自在原位维护唯一当前状态，使用挂载Skill规定的各自状态模型；历史失败/报告保留并引用，不靠反复追加“当前/最新”段落维护进度。无新文档层、ID或引擎规则，不改在途定义。

首次源回归命令遗漏已文档化的SDK导入路径，24项运行含1个ModuleNotFoundError；按现有标准命令`PYTHONPATH=sdk/python python3 -m unittest discover -s examples/platform-workflows -p '*_test.py' -v`重跑，30项全部通过（3.463s），diff检查通过。失败与最终日志分别保留；不把安装/路由回归或提示词文字变化当成实际Agent行为通过。此项与此前development诊断/证据预算指令一起待本Run完成后通过原manifest升级并用下一真实Issue验证。


第一切片完成原发布图：seq35仅同步三份进度文档，56文件摘要从QA对象1b2f7026…变为`abf372c5dbf156b61c4eb28a16a38b9b0cf5053047b06f5e8ca0d092354bf79b`，逐文件核对其余53文件不变；新摘要未伪称重跑完整门禁。seq36原publish Connector退出0并推送`b2f271c595740e88c7413a1b60181fc8985e6a24`，seq37真实创建[草稿PR #17](https://github.com/big91987/model-relay/pull/17)，seq38结束；无重复Issue/Run/PR。当前合并、正式部署及部署后体验尚未发生。

交付链接缺陷：PR正文原样发布后，GitHub body_html回读确认11个文档/截图href仍为本地相对路径。维护者经正式PR编辑入口仅将这些目标改为已发布b2f271c完整SHA下的blob URL；每个目标均核对该提交Git tree存在，回读body及渲染href确认，head不变。仓库内pr.md相对引用仍可用，没有改产品或回写已完成节点。原Issue补充评论[6009514713](https://github.com/big91987/model-relay/issues/16#issuecomment-6009514713)晚于节点交接，正式入口明确拒绝，未进入会话或重放；旧提示却建议对已完成Run继续/回退，与引擎实际不支持矛盾。

通用修复落在维护源：report模板要求对外PR证据链接使用实际仓库/任务分支及正确转义的明确URL，不依赖正文文件相对位置、不把新产物指向旧main。GitHub入口遇409后重新读取Run：completed指引关联新Issue，stopped/failed按页面恢复后重试原评论，活动交接等待核对，状态查询失败只提示核对；保持原异常、原快照、事件键和通知去重，不创建新Run或自动重发。新增同一forward入口的竞争/停止/活动/查询失败回归，修前4子项失败，修后全部31项通过（最终3.530s）；Ruff格式/检查与diff通过，错误Ruff路径尝试保留并按标准PATH修正。模板/适配器尚待受支持升级及现场复验，不把该单元回归称新恢复指引已在GitHub验收。


第一切片正式交付闭环（2026-10-06 04:52–05:06 UTC）：授权维护者在 PR #17 的固定 head `b2f271c595740e88c7413a1b60181fc8985e6a24`、GitGuardian success、无冲突条件下标记就绪并合并，得到 `85f6d17c07c5fddeb2d6a39a5a067769a11bb4cc`。push main 自动 [prepare 37415786989](https://github.com/big91987/model-relay/actions/runs/37415786989) 于04:57完成；固定 make verify 对该 merge SHA 真正运行，日志包含Go/race/build及1280/390两屏完整受控上游旅程。后续显式 [deploy 37416309483](https://github.com/big91987/model-relay/actions/runs/37416309483) 的 prepare复用相同SHA校验产物，deploy于04:59成功；不把缓存复用描述成再次完整测试。计划中的产物SHA-256为 `58a159c2aaf7c67119c9e491bde81c395eeacbb72142a6b0fbebf569d36933e5`。

外部 GitHub Deployment `6876076706` 的 local-preview success、平台正式 delivery API 的 PR/merge/deployment、实际 healthz 200/version及浏览器概览均返回同一85f6d17c完整SHA。用户Run页面显示“已合并PR”“local-preview success”“打开效果地址”和部署记录入口。正式预览重启后旧会话明确过期，通过正常登录恢复；从空概览“接入上游”进入配置页，从调用测试看到三项缺失依赖、对应修复入口、发送禁用和真实Base URL，再回概览成功。未向正式服务注入测试上游或伪造配置；完整配置/普通与SSE/故障恢复仍由同mergeSHA的固定双屏真实Go旅程证明，真实供应商保持Not Run。第一切片可进入下一轮，整条四轮目标尚未完成。

通用模板正式升级：先经API确认安装对象无在途执行，保存原manifest、共享浏览器manifest、入口配置、只读SQLite备份、完整已完成Run和源版本（私有受限目录，不提交运行数据）；以原选项执行install.py --upgrade，仅development/report spec变化，所有对象ID不变。重复升级无进一步变更；正式agents集合API回读两阶段完整投影与manifest一致，原Run 38步完整JSON逐值不变。一次探查不存在的agent详情GET返回404，按安装器已使用的集合API完成正确回读，没有据404误判升级失败。GitHub入口安装器返回Unchanged，仓库源路径变量指向唯一维护源，原SDK已支持workflow_run查询。该记录只关闭安装/历史保留验收，新的阶段策略和409通知现场行为仍需下一真实事件复验。


第二切片正式接单与终态指引复验：原 Issue #16 发布收尾评论 `6009708202` 仅记录实际部署结果；入口Actions `37416728939`如约返回拒绝/失败，随后评论 `6009710157`明确“Run已结束，不能继续或回退；新增工作用关联新Issue；重试不重开”，未入会话或修改已完成历史。这是新版409 completed分支的真实现场证据；其余状态分支仍只有源回归，不扩大Pass范围。

在第一切片正式部署与页面核验完成后创建[Issue #18](https://github.com/big91987/model-relay/issues/18)，正文承接精确合并基线、QA和Actions证据，定义多上游/公共模型路由、有限重试/输出后不重放、健康恢复、同请求诊断、旧数据迁移与连续UI结果。正式入口Actions `37416760795`成功，仅生成Run `82227c5986e5f41875895d8016c686a6`；接单评论 `6009714426`提供用户进度入口。seq1 prepare退出0，任务分支head为85f6d17c完整SHA，沿用Trellis0.6.15而不重新初始化；seq2绑定原Issue #18，seq3自动启动intake会话 `afb2f6c0faef217a8ee1ee354d7c64cb`。新Issue/Run只代表第二轮已开始，未预报需求、实现、QA或新发布通过。


第二切片G1交接：seq4于05:18 UTC以next正式进入seq5 design，会话 `3b4584f1aafde6915f9bf980cbe1f29e`，未创建新的Run。需求原生工具事件69547/69551证明实际读取挂载Skill及references，随后形成当前Run的product-definition、g1-review、reference-observations，并原位承接项目唯一路线；没有覆盖上一Run历史。维护者只读核对13组P0 Story/AC和7项规则：真实路由分配、有界不同上游切换、响应提交/取消后零重放、候选冷却恢复、多尝试未知用量、两屏连续UI与精确旧main非空数据升级/备份回退都有明确证据层。G1 Ready for Architecture仅证明需求就绪；设计、实现、固定门禁、独立QA及第二轮发布尚未通过。


第二切片设计审查发现发布接缝缺口：初稿ADR-001要求显式upgrade并让新serve拒绝schema1；维护者对照平台examples/github/model-relay-preview/controller.py的activate，确认当前只做旧binary backup、切换/start及失败时旧binary restore，无候选upgrade调用。若不接入，schema2候选会在正式部署启动失败；这是静态契约核对，未冒充实际故障部署。另初稿使用--data但基线CLI实际为--data-dir。原[Issue补充6010003185](https://github.com/big91987/model-relay/issues/18#issuecomment-6010003185)要求统一接口并明确备份→迁移→启动、失败恢复、幂等与旧版兼容；正式Actions37418716622 success，输入23302 queued至原设计会话，原23251继续running。平台源修复、产品设计接续及实测均待完成，不临时迁移或重复发任务。


第二切片设计接续与部署维护源修复：原设计输入23251、补充23302和23332均由正式API核对为completed；Agent先正常结束以接收已保存输入，处理完两项补充后才next到seq6研发，会话`129ff36b81a8bc7e39c1dc811e3844e0`。用户Run页显示第6步研发执行中及进入会话入口。第二条[补充6010041466](https://github.com/big91987/model-relay/issues/18#issuecomment-6010041466)关闭了验证草稿把API可替代旧UI主旅程的歧义：至少一条精确旧main用户页面配置/调用的非空升级主链保持，API仅补计数/错误/并发边界。G2及CLI契约文档就绪不等于产品实现或迁移通过。

平台唯一维护源增加prepare绑定4KiB有界只读部署契约、activate重核并先backup→candidate upgrade→start→实际version/schema健康；只对已登记且摘要/指针不变的legacy旧发布兼容，不把任意无契约候选视作schema1。失败与中断保存必要阶段及原快照摘要，隔离候选数据，以旧binary正式restore到空暂存目录再原子恢复；未确认/失败restore保留，后续显式Actions重入先恢复原发布，不重新备份可能已升级的数据。监督子进程继承锁且有限超时，父进程死亡后的不确定写入不会与新恢复并行。首次不明数据拒绝；同SHA重复操作检查实际健康。安装器先检查服务配置，未完成activation拒绝覆盖控制器，标准安装记录控制器摘要。

回归证据分层：第一批新增迁移测试在原控制器12项中2失败/1错误，恢复边界新增后19项中3失败/1错误，安装安全新增3项中2失败/1错误，均保留真实红灯日志。修复后最终34项全部通过（37.102s）：本地真实Git/产品子进程夹具、父控制器实际SIGKILL后的继承锁和限时退出、HTTP health类型/版本检查、新装/已有安装及Workflow安装器；launchctl和大部分产品健康仍为替身，数据为脚本夹具，不称真实Go或Actions端到端通过。当前产品seq6尚执行中；维护源正式安装、真实新旧Go联调、原Actions非空迁移及隔离失败恢复均仍Not Run。源PR保持Draft，整条目标No-Go。


控制器独立代码审查整改：源`0b70d6e`提交后、安装前，requesting-code-review流程发现两项Important：监督者异常死亡被当普通CLI失败，可能让持有同一锁的父控制器与存活产品命令并行恢复；deployed记录已写但最后committed未落盘时，重入会错误回滚健康新发布。两项均用新增真实回归在旧实现报错，未直接带缺陷安装。修复后监督者只有明确完成进程组收尾才返回保留的安全失败状态；异常退出、信号或无法确认wait均按ProductStillRunning保留现场。candidate_healthy只在精确部署记录、固定二进制/指针及实际version/schema健康匹配时无停服补齐提交，不健康仍走恢复。最终38项通过（42.072s）；独立复审另外在临时目录运行4项新回归全部通过，并验证无法确认child wait时的保守分类，无新增Critical/Important/Minor。仅批准继续受控安装与联调；真实Go/launchd/Actions/浏览器未由代码审查证明。


部署控制器标准升级现场：固定维护源`d0564d05e11eec271d7707ee7a444346e7be09ae`已推送源Draft PR #5，未合main。正式API/Actions核对无部署在途，并取得私有部署锁、备份旧controller/config/deployed/LaunchAgent及摘要后，执行原install.py参数两次均成功。已安装控制器SHA-256为`fc51d9e7b329f3fdf9bc4cbe46fc0541fb411e096a6fbccf4aecff3a21fd226a`，controller-install记录与源/安装文件一致；既有preview配置、deployed记录及LaunchAgent逐字节不变，current指针不变，实际旧Go healthz仍status=ok/version=85f6d17c完整SHA。此步骤没有重启、初始化或迁移产品。

原部署Workflow YAML与本轮产品文件逐字节一致。经正式workflow_dispatch选择main/deploy=false， [Actions37422891093](https://github.com/big91987/model-relay/actions/runs/37422891093) 于06:17 UTC完成：prepare success，真实日志Already deployed 85f6d17c，deploy skipped。它证明标准安装后原入口、受限仓库fetch、已有旧Go身份/健康校验及无副作用重复准备有效；没有再跑make verify、候选upgrade或发布，不计真实新旧Go/非空迁移/失败恢复通过。产品Run仍seq6研发中，这些未测项继续保留，平台源升级事实通过原Issue补充交接。


第二切片首次宿主测试与自动回退：研发seq6原输入24076及平台升级补充24298均完成，交接78617被接受后自动进入seq7固定 `make verify`。80文件摘要 `39c1a53b0851952ced57519c97a5e5a146c3c0a53f49158df24a676a035287b9` 与研发03回执一致；宿主exit2，沿既有失败边进入seq8研发，会话 `eea8ee6766619c46e99f90d6c2863796`。页面、正式API及GitHub阶段通知均能定位原Run；没有重建Issue/Run。研发seq6真实读取交付管理、Trellis before-dev/check/update-spec等Skill，111前端VM及非监听局部检查通过不替代宿主/浏览器门禁。

seq7诊断边界：平台首尾24KB保留了旧版故障预期红测和末尾relay/历史分类PASS，但截掉了中间实际失败断言。只能确认整体Go测试FAIL/make退出2，不把旧红灯或Agent本地监听限制误判为宿主根因。seq8通过代码与真实handler最小诊断确认CLI安装驱动仍使用旧单上游接口，修改当前驱动为新接口同时保留进程/端口/重启/密码恢复等断言。研发04摘要 `5b30123a1570e5cef7c1cf18ddf6243753d6e5518d2962523984f2cdef4743e0`，维护者独立重算80文件一致；seq9已自动执行原完整门禁，尚未取得通过结果。不能据局部修复声称首个宿主根因唯一确定或全部消除。

命令日志维护源补强进行中：真实回执缺口驱动新增每执行8MiB有界脱敏日志及现有Run/seq授权读取，保留首尾窗口和真实退出码。新增回归先因能力缺失失败，中文分页再暴露字节截断丢字符并修复；命令/日志/MCP的聚焦race验证通过，包含真实子进程中段保留、跨写入凭据脱敏、上限、旧回执404、用户隔离、根目录逃逸拒绝、当前节点只读前序及交接后立即失效。完整维护源检查与独立审查进行中；尚未安装到在途8792，不称当前seq7已有可恢复日志或真实新日志链已验。旧缺失日志无法追补。


seq9宿主复验达到原300秒预算后被正确终止：回执保留Go测试及build通过、精确旧UI非空schema1→2及旧binary回滚隔离旅程通过（2 aliases、2 keys、2历史请求、4受控访问）、1280浏览器主旅程通过，以及390版本展示检查通过，之后中断。整体仍Fail/Not Run，不能据部分PASS放行；正式Actions迁移、独立QA及完整390旅程仍待执行。原Run failed，未自动重放。

维护源日志与预算修复已完成隔离回归：日志独立审查无Critical/Important，发现一项Minor为纯文本响应后续读盘失败静默截断；新增首块后移除日志的回归并补固定“不完整”标记，复审关闭。命令预算采用现有管理员策略/执行seq/回执，不新建恢复对象：原冻结命令身份保持，分发时采样当前1–1800秒预算，执行中配置修改不改deadline，实际值写入回执；原resume拒绝重放，必须stop/静止/return。新真实进程回归旧实现再次超时红灯（5.05s），修复后race通过，验证旧回执不变、新seq使用新预算且管理员修改exec未替换冻结命令。模板参数默认900秒，并支持标准新装/升级。

最终 `scripts/verify.sh` exit0：网页/真实隔离浏览器、Python SDK/共享工具/安装器/部署控制器回归、ruff、Go vet与全仓race（platform39.448s）、build均通过；sanitize新增源码与文档差异通过。独立复审确认日志中断Minor关闭及预算变化未引入新Critical/Important/Minor。该结果仅证明维护源回归；此记录时8792尚未升级、原Run尚未恢复，新日志与新预算的真实链路仍待下步核验。源PR不合main。


源`f23bc3e9a5bd27e2216f1a2e3cf9334e568b7163`已推送Draft PR #5、未合main。正式API核对无running/waiting/stopping Run后，私有备份原失败Run、SQLite在线只读备份（integrity_check=ok）及原manifest；回退二进制由原运行源码6e875034重建，明确不是已被构建替换的旧进程inode副本。原Run先经stop API静止，仅重启8792，8788监听身份不变。首次启动遗漏既有WORKFLOW_GITHUB_TOKEN引用，标准安装check因此拒绝；Run仍stopped，无重试副作用。按已登记引用修正服务环境后重新启动，原Run完整JSON不变。

已装平台二进制SHA-256 `4ef0392dd5591acffcf4ebf0c56e4c4e3adad1b558773fa470d5330fde3a9975`；原manifest使用固定make verify及test-timeout-seconds=900升级，仅connector-tests变化，所有对象ID不变，重复安装无变化；正式API核对当前预算900、Run冻结原配置300及历史JSON不变。旧seq7日志接口404，未伪造之前缺失输出。确认平台无剩余子进程后，从原Run用户页面选择“运行项目测试”并填写已核对副作用/版本/预算的原因，点击“从该节点继续”；页面与API确认原Run seq10 tests running，seq9保留中断退出码及人工回退原因。新命令完整结果、日志读取及下一阶段真实MCP消费仍待发生。


新日志/预算真实链路复验：seq10使用900秒预算，完整命令exit2且保存74,586字节日志（truncated=false）；正式API分3页读取，拼接摘要`0cb5a4388c304856c488ed5968377afdbe9ba9fe7c120a86064bc5e77fafad39`，原04源码摘要一致。Go/CLI、旧UI非空迁移及旧binary回滚、1280/390既有主旅程通过；新增routing浏览器在create candidate set 1280失败，不能外推为全部路由AC通过。工作流自动进入seq11研发，会话`766766edfc4f4a62008ab01026842415`，输入25255。真实原生工具事件79835/79836调用read_command_output(seq=10,offset=68000)，error=null且结果含该失败断言，证明新阶段快照实际挂载并消费正式只读能力。用户Run页点击“查看命令日志”也打开对应纯文本页并显示真实失败；API/UI/原生Agent三层分别核对。旧seq7仍404，未补造历史。

日志审查补充的写入失败/取消证据现已纳入维护源回归：文件写入失败不打断命令输出收集，Log=nil且首尾回执保留；真实取消保存实际退出码和日志元数据。聚焦race exit0（2.059s）；这些测试没有改变已安装f23bc3e的产品代码。当前总目标仍In Progress/No-Go，seq11继续定位新增路由UI失败；QA、正式新版本合并/部署、真实Go联合控制器、正式非空Actions升级/失败恢复仍未验收。


第二切片seq12复验exit2：900秒预算、74,970字节完整日志，源码05摘要`f251735964e9fcc330e4f70387e3a3b073fd63537f3960e6cbd8a4daca560510`。旧UI迁移/回滚和既有1280/390主旅程通过，新路由候选primary选择1280仍失败。诊断确认标签唯一命中、Tab后selectedIndex仍0而期望1；标签修正不足以关闭原问题，不能推断更换按键即可修复。原Run自动seq13研发，会话`f05152cc21b44bda48b5570a17de6361`，保留按键与断言补逐键焦点/选项/重绘脱敏轨迹，114项前端回归及静态检查通过后自动交seq14宿主完整门禁。此处seq14仍running，未到QA或发布。

正式隔离部署入口维护源：原Actions增加预配置target=preview/validation，默认preview、push只prepare；两目标沿同一Owner/main/固定测试/prepare/activate与串行规则，GitHub Environment分别记录。validation根缺失不回退预览，两阶段传protected-root并拒绝根别名/嵌套、共享端口或服务、敏感状态树软链接/跨根硬链接、发布执行路径逃逸。无任意SHA/目录/命令输入、产品故障开关或测试权限。

隔离变更先新增CLI能力缺失红灯；独立评审发现仅检查顶层目录会放过内层数据库软/硬链接，Important经真实临时文件红灯复现后修正；发布source别名另有红转绿。最终45项控制器/安装/真实Actions shell参数回归通过（48.584s），ruff与diff检查通过。独立复审6项隔离回归及合法current指针通过，无剩余Critical/Important/Minor，可进入标准集成。此记录时新控制器未安装、新Workflow未同步，真实Go/Actions非空升级与恢复仍Not Run；回归不是正式部署证据。


隔离入口标准集成进展：固定维护源`294ddb3`已推送源Draft PR #5。确认无Actions部署在途并取得部署锁后，私有备份旧控制器与记录，原install.py对既有预览执行两次均成功。控制器实际摘要`e73b92b46ea62a7520d6e93c2bf306e8cdc5dee0322fff4d83c406e51de93fba`与源/manifest一致；preview配置、deployed/current/key保持不变，health仍为旧85f6d17c。新建预配置local-validation Environment并限制main，独立ROOT/URL变量读回正确，原preview变量未改。此时产品Workflow尚未同步/合并，不能据环境配置声称Actions路径通过。

原[Issue补充6012006592](https://github.com/big91987/model-relay/issues/18#issuecomment-6012006592)经入口Actions37433355966 success保存为seq15会话`bab911e0c63e66d87ff5d993d977d264`的输入25911 queued；原输入25844继续running。补充要求研发通过固定维护源原install_workflow.py --upgrade同步模板/摘要并重入核对，不直接编辑产品在途文件。seq14已真实exit2并保留77,652字节日志；原生按键到达、控件无重绘、选择仍0的轨迹由失败边交seq15继续处理。失败未闭环、不推进QA/合并。

隔离旧安装准备使用历史正式维护源`657ec4f2e9ec4349210289d15f88154a4b534441`的未修改install.py/controller.py和当前精确旧main85f6d17c，从独立私有根/未占用端口运行原prepare及make verify；不复制既有数据/key、不手写deployed记录、不增加产品测试权限。此记录时prepare仍执行中，尚未activate、UI建数据或升级控制器。它是明确标记的旧安装准备，不是正式Actions候选升级或恢复证据。


模板升级输入真实接续完成：seq15原输入25844尝试handoff时因25911已排队被拒绝，Agent先结束本轮，随后真实读取补充、确认当前维护源模板目录与固定294ddb3逐字节一致且无本地漂移。原install_workflow.py --upgrade首次Installed、第二次Unchanged，受管理YAML及manifest同步；回执保存于产品Run的platform-template-provenance-01.json等记录。两个输入均completed后才正式进入seq16 tests。同Run未新建，说明排队补充未被提前交接越过。当前产品08指纹82文件`c340664b231d83d153f0d2569065db08c8be34661a314f51e7831f74f808b27e`，116项局部检查不代替当前仍running的固定门禁。

隔离旧安装基线准备完成：历史657ec4f控制器的正式prepare完整make verify exit0（1280/390旧版真实浏览器旅程均PASS），原activate成功且health为精确85f6d17c；这是CLI基线准备，不是正式Actions候选发布。随后原新版install.py重复升级两次，deployed/current/key不变、隔离边界实际检查通过，安装摘要同e73b92b46ea62a7520d6e93c2bf306e8cdc5dee0322fff4d83c406e51de93fba。没有通过复制预览数据库或构造deployed/activation记录创建旧基线。

独立旧安装页面从正常登录与空概览开始：不可达.invalid地址被旧安全规则拒绝，未放宽权限；改为停用上游及非供应商占位凭据，随后真实页面创建两个模型。停用其中一模型时In-app Browser控制超时，原确认状态不可读取；新标签页概览确认仍2模型/启用2、上游停用、0应用密钥。截图保留私有证据，UI停用、密钥和历史准备仍未完成，不能记为非空完整主旅程或迁移通过。真实供应商从未调用；原预览根/服务保持不变。后续需经受支持浏览器路径完成准备，再分别验证Go联合控制器及正式Actions升级/恢复。


seq16完整宿主门禁首次通过：2026-10-06 08:14–08:20 UTC，exit0、900秒预算、74,864字节完整保留日志；正式分页API拼接SHA-256 `2776ab4ad6550835479d3655ab9b40f930ca3bb6f546a798a2600feeed94561c`，与08指纹`c340664b231d83d153f0d2569065db08c8be34661a314f51e7831f74f808b27e`一致。旧UI非空迁移/旧binary回滚、既有1280/390旅程及新增两屏多上游连续旅程均真实PASS，涵盖GET核验、优先级/3:1分配、停用、切换/未知用量、被动恢复、SSE/取消不重放；受控上游范围保持，不推断真实供应商或正式Actions通过。维护者另独立逐字节核对目标Workflow与固定平台源及manifest摘要匹配。

工作流自动进入seq17独立QA，会话`7a68aa00259a7b461f303119c53388b2`、输入26452。QA核对版本与宿主回执后新增隔离契约回归，connection-contract-01.log真实exit1（5.851s）：核验预算30秒超过设计10秒；错误Content-Type/缺少或错误list对象/缺少或空model id被误判success；malformed JSON、429、5xx及网络错误被判unknown而契约要求failed。QA正在形成No-Go/返工记录，不能用seq16绿色覆盖这些遗漏；独立QA尚未通过，不合并或部署候选。

旧安装UI准备仍未完成：已有模型停用确认导致原临时标签页控制超时；新标签页可读页面，点击导航/签发未产生可验证结果，工具重置和替代浏览器创建也出现超时。正式管理API只读对账并撤销诊断会话，最终仍为停用上游、2启用模型、0密钥；未用API补做UI创建，未弱化安全规则。实际调用/历史准备、原生确认与连续UI不计通过。该观察是浏览器工具/旧页面交互限制，尚无证据归因为当前M2产品缺陷，不将其混入QA已确认三项契约问题。


QA正式返工完成：seq17结束后沿原图进入seq18 development，会话`01515859e9663c2ecd2b865633e2565e`，输入26767。独立report-01/remediation-01最终列四项P1：三项已运行红控的核验协议/10秒/失败状态，以及同版本真实截图/静态对照确认的完成步骤折叠合同缺口；另要求补取消核验、完整两屏原生键盘和缺失竞态覆盖。未把静态缺口冒称真实浏览器负控已运行。研发已收到原整改包，按原规格修复，不退回需求放宽；仍不放行PR或新版本部署。

隔离旧安装非空准备推进：失效临时标签页被清理后，原IAB页面真实签发2个应用密钥。首次读取password框得到工具遮罩值，使用该值的实际请求`k58dymNFKX3SGy5LvU7XgA`被网关401拒绝并落盘；此为操作诊断，不是产品密钥缺陷。第二次经页面“显示新密钥”取得实际46字符测试凭据，关闭展示后从调用测试发起请求，再点“查看本次记录”；请求`kUQ9uxNy6oWu9NUD216_zQ`识别密钥ID `G0Rk4Ke1StyzswKVfs2z_Q`、模型migration-enabled并记录上游HTTP405、未知用量。上游为公开示例域名及明确非秘密占位值，不是AI供应商，不据此宣称模型生成或供应商联调通过。没有改数据库或注入测试权限，首次停用确认的未完成状态仍保留。

原生页面与正式管理API只读对账：2模型、2密钥、2历史（旧API分页字段data）；原存量都来自UI，API仅核对。私有baseline-api摘要`195eb3e8685bab4645e3f53da2046dd2a9b38a8c779f26a63fc83cfcd9a268e3`、截图及测试密钥保存在安装/私有证据中，秘密不进仓库和回执。该基线用于后续正式Actions非空迁移/恢复，当前还没有候选升级、回退或全部旧UI旅程Pass；完整成功旧UI数据迁移仍由产品seq16受控上游证据单列。


维护源新增有界端口冲突演练入口 `examples/github/model-relay-preview/tests/port_conflict.py`，用于已授权的独立validation安装。执行前要求正式候选main SHA、已安装控制器摘要一致、旧服务健康、无在途activation、与preview根隔离；只读观察目标新轮backup_complete后绑定loopback空闲端口。它不写activation/数据库/备份/部署回执，不改产品权限或二进制。返回固定503夹具，只计算已观察upgrade_confirmed后的GET /healthz；首请求起保持25秒，升级等待130秒和首请求15秒各有限界，单次socket读写各0.1秒，不等待完整请求头。未命中窗口明确不完整，恢复字段始终not_checked。

独立评审发现保持起点过早与慢速请求头拖延释放两项Important，均先用真实临时socket红控复现（延迟首请求无法连接、部分请求头等待超时），随后修复。8项针对回归及完整53项控制器/安装/隔离回归通过（46.401s）；ruff检查、格式及diff通过，独立复审另跑8项通过（3.019s），无剩余阻断项。上述只有临时根/端口夹具；尚未对真实安装注入故障，DEP-07正式Actions失败恢复和数据守恒仍Not Run。README明确等待产品QA放行及正式候选后，从原Actions validation入口演练，再单独无故障发布；不替代其他backup/upgrade/restore失败用例。


seq18按原QA整改完成，产品10指纹85文件`ff456d824299075fe8d98104a2cd51671ca41f08421f18cb1dd053d80efe93c8`；119项局部前端回归与专项检查不作为整轮放行。seq19宿主900秒原make verify于09:02–09:09 UTC真实exit2：QA核验协议/10秒预算负控及真实HTTP头/body预算、工具输出后EOF/idle/cancel边界均通过，原两屏旅程和旧版非空UI迁移仍Pass；新增1280px“completed task fold and keyboard review”TimeoutError，390px新增旅程未到达。固定门禁整体Fail，不用前面通过覆盖后段失败。正式分页API取得83,109字节完整日志、truncated=false，SHA-256 `966690092c2a17f2c331da8e30a5dca4141281f1952aeee74a52714627a890a4`；原selection轨迹显示目标index4/实际4，不能沿用先前选择器故障归因。

原失败边自动进入seq20研发会话`7adceb28a67020960e705a7caacaeea9`；其公开诊断指出详情正文缺少驱动等待的“请求”字样，拟改为核对同一Request ID并细分失败阶段。此为研发诊断，修复后新宿主门禁/独立QA仍待验；未重复创建Issue/Run或人工修改产品。预览与隔离旧安装只读核验仍健康、deployed/health均为85f6d17c，两个controller摘要与维护源一致。未运行故障夹具或部署候选，正式恢复仍Not Run。


seq20将回看驱动的错误文字等待修正为当前概览证据ID→请求详情API身份→页面同ID，保留步骤数量、默认收起、键盘展开/收起和可见焦点断言；未改产品行为或降低原需求。11指纹85文件`852f7bfc90323b6253658c8b9c2167fa22c43a5f59389f0bbcb92539865120e7`，119项局部回归通过后正式交接seq21。seq21于09:19–09:27 UTC完整make verify exit0（900秒预算），原Go race/CLI、QA负控、旧UI非空迁移/旧binary回退、既有两屏和新增1280/390原生键盘/核验/折叠回看/重启旅程全部执行成功。正式日志分页API取得79,939字节、truncated=false，SHA-256 `e1decdc833ef8f2167d55013ed24d1b2718fe0af0fee85b17a4403a4483829d7`；维护者核对两屏新截图时间属于本轮，完成步骤实际位于可展开回看区、下一步与已完成分开、窄屏布局和焦点可见。截图是测试构建dev，不能当正式部署SHA证明。

原图自动进入seq22独立QA，会话`84cf7708dcaf51134cdc05044fc5717c`。需由其核对原四项缺陷及缺失覆盖，不从完整门禁直接推断Accepted或全部AC通过。原Issue已真实同步seq20启动评论6013053758及上一研发交接，不重建Issue/Run。当前没有M2 PR/合并/正式候选升级或故障演练；DEP02～04/06～07与供应商限制继续保留，整目标No-Go。


## M2 PR 交付与合并前复核退回（2026-10-06）

- 原 Run `82227c5986e5f41875895d8016c686a6` seq22 QA Go 只覆盖当前产品实现；seq23 报告、seq24 发布、seq25 PR Connector、seq26 end 均完成，生成 [Draft PR #19](https://github.com/big91987/model-relay/pull/19)，head `8ae4e5173aa0dc45c28ad61dbc64df20afe229a6`，base `85f6d17c07c5fddeb2d6a39a5a067769a11bb4cc`。没有合并或候选部署。
- seq21 固定完整门禁 exit0，保留日志 SHA256 `e1decdc833ef8f2167d55013ed24d1b2718fe0af0fee85b17a4403a4483829d7`；QA11 85文件快照 `852f7bfc90323b6253658c8b9c2167fa22c43a5f59389f0bbcb92539865120e7`。发布前逐文件比较，仅产品简述、路线图、当前部署契约及路由验证四份报告文档变化；实现/测试/依赖未变，不能把报告后快照当新一轮完整门禁。
- 独立代码审查固定上述 base/head，发现两项 Important：超256KiB SSE首行被归为可切换传输错误，实际额外访问备用上游；DONE下行写入失败后请求/attempt转取消但成功上游字段未清空。在独立副本的 Handler/隔离Transport/真实SQLite层，两个确定性红控均失败。真实HTTP红控受审查环境监听限制，未计真实网络通过。产品合并结论 **No-Go**，待原Pipeline补用例、修复、宿主完整门禁及QA复验。
- 恢复入口审计发现 completed Run 无法回退，PR Connector重新进入会尝试再次创建同分支PR。修复位于平台维护源：复用显式return、事务重取工作区、原完成记录不变；后续发布更新该Run原确认PR，独立marker核验PATCH丢响应。针对性回归先红后绿；独立复核补出反馈被失败覆盖和撤权重占目录两项，已追加失败信息、在completed重新激活前检查当前权限；保留管理员结束stopped旧Run的原清理能力。
- 这里不把平台回归或审查通过计为产品修复、真实重入发布、正式候选升级、故障恢复或供应商联调通过；后续记录实际页面、API和外部回执。

平台返工入口最终固定 `scripts/verify.sh` exit0：网页/隔离浏览器、SDK及安装器回归、Go vet/race/build全部通过。独立代码复核的两个 Important 已关闭，且补回撤权 stopped Run 的管理员清理回归；源码差异脱敏与 diff 检查通过。门禁日志 SHA256 `0cacc4105095f02ba6177676d8d7b41e576301b517f734856187840aadac710e`。此处仍未计运行环境升级或产品返工通过。


### 完成后返工的现场升级与原 Run 页面复验

维护源 `5758a176d3eba58b239beaf78c3606ccdb35e9e0` 已推送源 Draft PR #5（未合main）。通过正式 API 确认20个Run及会话无在途执行后，备份旧二进制、原manifest及停止后的runtime目录，再按原服务CLI路径重启8792。新二进制SHA256 `2c2e7ee6decddc806f85b9c6663832ff52a013164dceb84f6b8f9ca90e4ffca8`；20个Run完整JSON与升级前一致，原manifest逐字节不变；不涉及数据库迁移、模板覆盖或产品服务部署。

从原 Run 页面实际选择“研发实现”，填写两项审查红控、契约和验证要求后点击“从该节点继续”。页面显示执行中，正式API核对同Run seq27、development会话 `889e53d26ae84efed1213ab4d9ea54eb`，原26步/冻结图/工作区均未改。返工原因已持久化；原Issue收到真实 [seq27启动通知](https://github.com/big91987/model-relay/issues/18#issuecomment-6014148090)。没有新建Issue、Run、Agent或PR。此证据证明完成态经页面进入真实原生研发阶段；产品缺陷关闭、宿主门禁、独立QA、原PR再次发布及正式部署仍待发生。


### M2 合并前返工完成与 seq28 新完整门禁

seq27 研发按正式 handoff 完成两项 SSE 修复，沿原 next 进入 seq28 tests。新增确定性Writer/Transport/SQLite红控实际复现，并在项目固定入口加入真实HTTP超长行的提交前/后停止边界；原回归、双屏旅程和旧验收报告保留。最终产品源13摘要 `e3cde10304e0bbce0dcebf671f9469ab7e5c494d8524cb9a7b7e04228d71d54f`、86文件，HEAD仍为已审旧提交 `8ae4e5173aa0dc45c28ad61dbc64df20afe229a6` 加本轮未发布修复。维护者同步保存86文件逐项哈希，供发布前核对后续文档变化。

独立代码复审在固定副本核对两处实现及新增回归，原两个Important关闭、无新增阻断；原红控及新增8场景race通过21.117秒，相邻健康代次/超时取消/提交前切换回归通过11.059秒。固定实现哈希：proxy.go `f6302151a1e602b311afa418f49698d22008be1ea8e2f67f7d954958c4cb7baa`，routed_proxy.go `60034cff2d2e87b999172710d57b78a670c2173e93e08a117ca73d1f12fa7671`。最终新增测试只将t标识符重命名为testContext（逐文本替换比较一致），最终哈希 `55e3cdd28858bd6adfc0d01a4b30368c6f83231f44a7ec577ca695a2adf4fac1`。审查环境真实HTTP仍因监听受限未测，未以局部绿灯替代宿主执行。

**seq28正式宿主门禁通过**：10:29:58–10:38:32 UTC，固定make verify，900秒预算，exit0。正式分页日志API取得81349字节完整脱敏输出，truncated=false，SHA256 `0b8190a4521048f19436d66b667133ecda7325bddf1e90397d7b9bbe5fa1d9a6`；开头指纹与源13一致。新增 `TestSSERealHTTPOverlongLineNeverSwitchesOrCools` 两分支真实执行通过4.88秒，确定性超长行4.87秒，DONE四终态9.74秒；119VM、Go race/构建及原完整门禁通过。真实旧85f6d17c非空UI迁移/旧二进制回退、原首次调用和多上游1280/390两屏旅程均PASS。受控上游不计供应商验收，项目内迁移回退不计正式Actions控制器联合恢复。

已自动进入seq29独立QA会话 `6839bfc44c99be8bad57bb666b6191df`，原Issue收到 [QA启动通知](https://github.com/big91987/model-relay/issues/18#issuecomment-6014482368)。此时独立QA新结论、报告、原PR再次更新、合并及正式部署仍未发生；原PR19仍Draft/Open、旧head，既有Runner在线空闲。整体仍In Progress/No-Go。


### M2 原 PR 再次发布、合并与正式自动准备

seq29独立QA形成report-03/matrix-03产品切片Go，两项SSE Important/P1在原发现层关闭；旧四项P1未复发。seq30只同步公共交付投影，产品源13的86项逐文件清单中仅四份状态文档变化；两处实现及新增回归与独立复审的固定摘要完全一致，工作树在发布后干净。其余QA/报告产物单独保留，未把报告后摘要当新门禁结果。

原图seq31正式提交推送新head `23aff17f6b8e7e0834dbf1d751d05e66cab69366`，seq32 github.pull_request回执指向同一个[PR #19](https://github.com/big91987/model-relay/pull/19)，number19及head一致；外部API确认正文含新执行marker seq32。seq33 end完成。原26步及历史Issue/PR保留，未新建PR或Run；这关闭完成态页面返工→真实研发→固定tests→独立QA→报告→原PR更新的正常重入链。PATCH丢响应仍只有隔离回归，未进行真实GitHub网络故障注入。

维护者核对源码/测试/规范/锁/模板与seq28及独立审查身份、GitGuardian通过和精确base/head后，按既有测试仓授权标记就绪并以match-head-commit合并。GitHub确认2026-10-06 10:54:22 UTC合并，main为 `98511771871cf0951ecef716bbb55deb13e18da5`；源平台PR #5未合并。正式push触发[Actions自动准备37452871127](https://github.com/big91987/model-relay/actions/runs/37452871127)，此记录时in_progress，只prepare、不deploy。效果服务及独立验证安装仍旧版本；正式非空升级、隔离故障恢复及部署后浏览器尚未完成，DEP02–04/06–07与其他未测边缘和供应商限制保留，总目标In Progress/No-Go。


### M2 正式非空升级、失败恢复与效果发布（2026-10-06）

精确main `98511771871cf0951ecef716bbb55deb13e18da5` 的[自动prepare 37452871127](https://github.com/big91987/model-relay/actions/runs/37452871127)成功，deploy按push规则skipped；正式完整项目日志81355字节，SHA256 `ff4a245bf0d06b8221680e440504560e6afc0a8902812f5717da81fc29d349f3`。两屏首次配置/多上游、真实旧版非空UI迁移回退和新SSE回归均通过；计划绑定schema2契约及制品SHA256 `5255e42b9e8ec617b6794eaa18654d61ec73c65b5f3250ede997a11df745a116`。准备成功未自动停旧或发布。

独立validation根按原Actions入口执行[正式故障轮37454004343](https://github.com/big91987/model-relay/actions/runs/37454004343)，prepare独立完整通过（81352字节，SHA256 `a2f632c90acc5aaf435a4621d76e7459545e74990c62eeac086b30f1a37aa23a`）。维护源有界端口夹具实际观察新attempt `1791285276838380000` backup_complete→upgrade_confirmed，首健康请求后占用25秒、90次健康请求，最终自动释放；夹具只声称fault-injection，恢复另行核验。Actions deploy真实exit1，Deployment6882450407 local-validation/failure，诊断service health未达预期。

控制器实际写入rolled_back；current/deployed/旧二进制恢复85f6d17c且原主密钥摘要不变，原回退归档摘要匹配。只读SQLite核验failed-data保留schema2，正式API恢复旧schema1并与UI私有基线核对2模型/2密钥/2历史全部一致；原preview在整个演练中未变。没有手改activation、计划、数据库或部署回执。真实UI重新登录→调用测试→输入原密钥→发送→查看本次记录，新增请求`RWeOUAcw6p22qFKm-9A92w`仍归属原密钥ID并映射原模型，示例域名405明确不算供应商生成。该额外请求作为后续正常升级前的第三条历史保留。

故障释放并核对恢复后，沿同一正式入口执行[无故障validation 37455472388](https://github.com/big91987/model-relay/actions/runs/37455472388)，Deployment6882521991 success，attempt `1791285501622378000` committed。prepare复用原已验证不可变制品并复核main/摘要/契约，不冒称再次完整跑门禁。实际health为精确main及schema2，原模型映射/密钥/历史API守恒；UI原管理员重新登录、两旧模型显示原上游候选、全部三条旧历史可见。原密钥从页面新发请求`uAhZEw91cfrtnWx19yxYLg`归属相同身份、远端映射和1次上游尝试，未知用量保持未知；示例域名405，真实供应商仍Not Run。

随后[正式preview 37455730994](https://github.com/big91987/model-relay/actions/runs/37455730994)成功，Deployment6882565695 local-preview/success；实际deployed/current/binary/health/version/schema与精确main及上述制品摘要一致。管理页真实重新登录→概览→接入上游→回概览可用，显示新SHA与多上游空状态，无注入演示供应商；没有凭据，完整生成不在该常驻实例冒称通过。原Run页点击刷新部署状态，真实显示已合并SHA、preview成功及效果链接、validation成功与此前failure，失败未被成功覆盖。截图留私有证据，不进入共享仓。

**分层结论**：产品路线2发布闭环完成；DEP06正式非空schema1→2与DEP07隔离正式健康失败恢复已有真实Actions/API/UI/数据证据。DEP02顺序、DEP03健康失败恢复、DEP04已知旧无契约二进制backup/restore子范围获得真实联合补强；其余backup/upgrade/restore错误、超时/崩溃、未知契约/降级等真实Go联合负例及DEP05未覆盖安装边缘仍未测，不能整行自动置Pass。供应商Not Run。整体目标继续In Progress/No-Go，源PR #5仍Draft且未合main。

路线3按既有授权创建[Issue #20：应用访问生命周期与用量控制](https://github.com/big91987/model-relay/issues/20)，绑定上述实际基线/QA/正式发布与限制；通过标准GitHub Issue→Actions→SDK入口接单，不在平台另建重复Run或手改产品。此记录时仅Issue已创建，接单和后续执行另核验。


路线3接单已确认：[Issues opened Actions37456112193](https://github.com/big91987/model-relay/actions/runs/37456112193)实际success，经正式SDK创建唯一Run `9e6d05f409675e0ef65c6486691462df`。原Issue [接单回执6015213052](https://github.com/big91987/model-relay/issues/20#issuecomment-6015213052)提供Run入口，[分派启动6015214058](https://github.com/big91987/model-relay/issues/20#issuecomment-6015214058)指向原生会话 `190543a0f27ab0aab9d9e1b2c5d3c7ca`。没有手工建平台任务。当前只证明接单/分派，尚不声称需求/研发或下一产品验收完成。


## 2026-10-06：真实 Go 与控制器联合负例回归；路线3设计中

新增可复用入口 `examples/github/model-relay-preview/tests/real_product_lifecycle.py`，用全新私有根、只读产品Git对象及精确旧 `85f6d17c07c5fddeb2d6a39a5a067769a11bb4cc`／候选 `98511771871cf0951ecef716bbb55deb13e18da5`，两版均执行未修改的完整 `make verify`。历史控制器从源提交 `657ec4f2e9ec4349210289d15f88154a4b534441` 提取，由其实际prepare/activate形成旧安装；真实管理API创建禁用上游、启用/停用各一模型、一密钥及一条401历史，无上游访问。仅替换服务管理器为受控子进程，Go CLI、健康、SQLite及控制器状态均实际执行。第一轮在服务替身错误读取deployed记录时失败；独立审查确认P1，改为跟随current链接后从全新目录重跑，未复用或覆盖失败现场。

修订入口SHA256 `423ece686c46c3bd02af01a1f53e3644cddb55f88b19353f56dc54598d5a7a32`，当前控制器 `e73b92b46ea62a7520d6e93c2bf306e8cdc5dee0322fff4d83c406e51de93fba`，历史控制器 `4f9fd39c30ae73ae82c7f794100baa7751fdabb529a974a2c51c807f66a653eb`。真实整套退出0，8项检查通过：

1. 已知未改旧无契约安装、非空API基线及相同SHA准备保持数据。
2. 实际backup因输出碰撞拒绝，未调用候选upgrade，原服务与数据恢复。
3. 实际upgrade因输出碰撞拒绝，经旧二进制restore恢复原数据。
4. 候选实际监听端口失败，再由真实restore拒绝非空目标，记录recovery_pending且不冒充健康；显式activate重入使用原备份恢复，保留部分恢复目录，备份摘要不变。
5. 真实upgrade完成且持久化upgrade_confirmed后注入中断；prepare仅安排恢复，显式activate恢复原快照，不用旧binary重新backup新schema。
6. 正常schema1→2实际升级，管理员可登录、原密钥身份/模型启停与映射/历史/主密钥摘要保持。
7. 健康同SHAactivate不执行服务或产品命令。
8. schema2安装拒绝无契约旧候选，无停服副作用。

旧完整门禁日志66814字节，SHA256 `ebe8e59782b03f27fd9ccce5c79fe42eb766280563be0ceb4ab09e7fb0a1f8aa`；新81557字节，SHA256 `42119433012de8848829846a666a48c13aecf9600dd1b079ccf2998b4f9976c1`。新binary摘要 `5255e42b9e8ec617b6794eaa18654d61ec73c65b5f3250ede997a11df745a116` 与正式发布一致。脱敏evidence JSON摘要 `a1efa65802793a0419a84072193a55532f5ac869fc1956c12cc22feaf2b64ea0`，标记evidence_kind=controller-real-go-isolated、result=passed及执行入口摘要。退出后临时服务端口已关闭；正式preview/validation实际health仍为原schema2合并SHA。私有完整日志与安装留本机，不提交凭据或数据。

复查关闭唯一P1；编译、ruff检查/格式、diff和改动范围秘密/本机路径扫描通过。此入口和使用说明纳入唯一维护源，不更改运行控制器或产品副本。DEP02–04上述实际CLI拒绝/恢复/重入/降级子场景Pass；注入BaseException不等于OS父进程死亡或断电，输出碰撞不代表所有I/O错误；真实Go超时/强杀组合、畸形契约及DEP05剩余安装边缘仍须另验。服务替身、API基线和本层成功不替代正式launchd/GitHub/UI/供应商证据；前文正式Actions证据独立保留。

路线3[Issue #20](https://github.com/big91987/model-relay/issues/20)已经真实Actions `37456112193` 成功并经SDK绑定唯一Run `9e6d05f409675e0ef65c6486691462df`，原Issue接单回执6015213052。原页面核验prepare→原issue→intake→requirements完成，design seq5执行中；需求结束通知6015388486、设计启动6015389059均回写原Issue。需求PRD/G1完成，设计草稿识别schema3与现有控制器只接受1/2的发布前置缺口；设计尚未交接，不据草稿宣称平台支持或产品实现完成。下一步等待冻结契约，在平台维护源扩展明确支持范围及回归，再经原安装器应用，不允许产品复制控制器或手工预迁移。整体仍In Progress／No-Go。


## 2026-10-06：路线3冻结schema3契约与平台源支持

原Run seq5设计完成，经正式handoff自动进入seq6研发（会话 `17cef7d978a872596b4686b1759d4ce7`）。产品唯一deployment-contract已冻结profile：contract_version1、init3、serve[3]、upgrade_from[1,2]、explicit_upgrade=true、backup/restore[1,2,3]，不换协议或扩大任意版本；产品研发与平台维护职责分开，正式schema3联调未执行。

平台 `validate_contract` 最小扩展已知1/2/3集合及对应升级矩阵，旧profile、固定命令、停服前降级拒绝和旧binary恢复保持。5项新增回归在实现前全部因unsupported storage contract失败；实现后完整58项首轮暴露1处新增测试误取backup键，按真实attempt路径修正，补充3→3不同SHA发布的already_current路径。修订后完整58项回归通过（见本节后续执行摘要）；独立复审关闭该P2，未发现生产实现Important。覆盖1→3、2→3、首次3、3→3/同SHA、候选健康失败恢复2、降级停服前拒绝及能力缺失/未知4拒绝。产品CLI使用文本数据夹具，服务健康为替身；此层不能证明真实schema3数据库或Pipeline已验。

README同步精确支持范围和发布前置；标准安装器与Workflow内容无需改动，使用原安装路径交付。当前本记录时仅维护源实现/回归通过，可信常驻控制器尚待备份及原install.py升级，真实产品schema3门禁/QA/联合CLI/Actions/API/UI均未因此改为Pass。整目标仍In Progress／No-Go。


schema3平台交付补记：源提交 `add852abb8dd76bcf9cbf5082f7aaea3d103b380` 已推至源草稿PR #5，controller SHA256 `782d4861df5aa3e93c241e037904dd3c58d9f4772f790170f6328aa5c62307a2`。完整58项耗时60.624s、日志SHA256 `138645e2ef11548b1912d853ff2d5036e45b289403d607e14e9ed718ee24474e`。确认无在途部署、原activation为committed后，分别私有备份并经原install.py升级preview/validation，再重复安装；实际manifest摘要匹配源，重复安装manifest不变，preview.json/deployed/activation/主密钥摘要/current指针保持。两实际health均仍为 `98511771871cf0951ecef716bbb55deb13e18da5`／schema2，未迁移产品。

正式Actions只准备核验：[preview 37460765087](https://github.com/big91987/model-relay/actions/runs/37460765087)、[validation 37460769786](https://github.com/big91987/model-relay/actions/runs/37460769786)均success、deploy skipped，日志均Already deployed当前schema2 SHA。这验证新安装控制器的既有版本路径，不是重跑make verify或schema3迁移。源版本/安装/58项夹具与真实schema3未测边界已从[原Issue评论6015883622](https://github.com/big91987/model-relay/issues/20#issuecomment-6015883622)交还原流程；评论入口Actions `37461024848` success，原Run仍seq6研发执行中。平台源支持及标准安装已完成，真实schema3产品/联合/正式发布仍待实际候选后验证；不修改历史结论或当前Run状态。


## 2026-10-06：真实 Go 超时回收与安装身份拒绝补验

在既有真实Go联合入口加入可重复的阻塞读取场景：仅候选upgrade进程的key-file参数指向私有FIFO，真实旧/候选二进制、正式数据及原主密钥不改；写端保持打开且不提供字节，实际Go读取因此阻塞。控制器使用缩短的1秒执行期限；只有确认读端已退出（写入得到EPIPE）后，才允许按普通命令失败路径调用旧binary restore。进程回收不确定、读端仍存活或夹具线程未退出均保留ProductStillRunning语义，禁止降级成可恢复普通错误。独立审查先发现两处异常覆盖风险，修订后关闭Important。前两轮在旧版门禁阶段主动终止以修订入口，均未执行产品激活、未计Pass，现场分别保留。

最终全新隔离轮真实exit0，9项检查全部通过，包括前节8项及本次超时检查。执行入口SHA256 `59052947f1db6e8a1c1cc38d3807446455ea7863c225842414f3b2389a5afdb3`；实际控制器为schema3支持版 `782d4861df5aa3e93c241e037904dd3c58d9f4772f790170f6328aa5c62307a2`；两版产品仍固定旧85f6d17c与候选9851177（schema1→2）。旧完整make verify日志66809字节／SHA256 `5febb3b7efe26d670109f786630c978ecdaa2c9de0e6afea140a421988a1b731`，候选81554字节／`2db340ce46ddd4b218438493f5e75373c2ac9f0520207d8bd5ec32cb44ed3dbe`；候选binary摘要仍与正式M2一致。evidence JSON1479字节／`f137f85d7d78648b1465704bff38e4ce6c1d470dccaaa55c56d86d0d04ae1e07`，result=passed；退出后临时服务端口已关闭。

另以真实install.py子进程验证既有安装拒绝不同端口和不同Git origin：明确非零及对应错误，controller、安装manifest、设置和LaunchAgent四文件的字节/inode/mtime_ns均未变化。该文件4项测试通过0.669s；测试只在临时HOME/安装根执行，不启动服务，不等同正式部署或全部文件系统无副作用。独立审查通过，ruff与diff检查通过。

上述补验仅关闭DEP超时及安装身份拒绝的具体子范围；FIFO为阻塞读取夹具、期限缩短，不代表真实磁盘故障、迁移写入中断、OS父进程死亡或断电，服务管理仍为子进程替身。未把本层回归计为launchd、GitHub、UI、真实供应商或schema3产品迁移通过。原Issue #20／Run 9e6d05f409675e0ef65c6486691462df仍处于seq6研发；平台支持说明已核实送达该原生研发会话。整体继续In Progress／No-Go。

路线3正式升级前另存只读API私有基线（未覆盖历史基线），SHA256 `9fea8e863677e1a9a3a8e53344ec23065904589f801fefb11d59ae276d2763a9`：实际health/deployed仍为9851177、schema2，2模型均启用、2密钥均启用、4条调用历史；模型名包含disabled不作为状态证据。原历史基线摘要仍为 `195eb3e8685bab4645e3f53da2046dd2a9b38a8c779f26a63fc83cfcd9a268e3`。本轮浏览器停用按钮未得到成功状态，API确认未改变；不把尝试当已准备禁用样本。原生应用检查被工具安全规则拒绝，未绕过；此项保留未验证，不据此归因产品缺陷。真实产品固定门禁仍须按AC-11独立准备并验证旧版启用/禁用对象。基线仅用于之后正式schema2→3对账，未计升级完成。


## 2026-10-06：工作流文件往返页面补验与路线3首次QA返工

在既有8792管理页从原工作流“导出→下载JSON”实际得到7700字节文件，SHA256 `5d5293a782daa32002ae6dd16adca95ae49b93932cf32d96ed09acdf98f33277`。浏览器download事件等待超时，但下载目录中对应时间的新文件实际存在；按下载文件内容另与正式API逐字段核验，不把事件超时当下载未发生或重新反复下载。随后“导入副本→选择JSON文件”通过正式filechooser载入；工具调用返回延迟约101分钟，导入内容确已出现在表单，未据延迟推断产品异常。将副本明确改名为文件往返验收、停用，再经页面导入及保存，实际新ID `6bc0b3a6915bdc9421fd2f7fe907e17f`，revision1。刷新前API回读确认12节点、21边、3Hook、entry/start_nodes/max_steps与下载源一致，authorized_users为空、enabled=false；原工作流全部JSON仍与导入前一致、revision3。副本保留供复核，不启动任务。完成从页面下载文件→文件上传解析→保存副本的真实步骤，非手填JSON代替；不把同实例引用保持等同跨实例Agent/Connector自动迁移。截图私有保存。

路线3原Run `9e6d05f409675e0ef65c6486691462df` 的seq7固定make verify实际exit2（900秒预算；内部Go测试600秒超时），失败完整日志90215字节／SHA256 `5e33a2952bc9299120eacddcdb9bbe4369a189b5e105cde65f0633804277c556`。自动沿failed进入seq8研发，seq9重新完整make verify exit0，104文件指纹 `125ae584daabf62e7e9e410a6aaa90cbe01101582b5490e0ce9e1602d4461529`；日志83637字节／`b349eb3ef721c1a45c57441372d145c9b4b49c9f492b86b06068091dc24dc164`。两份均从正式分页API读至eof且truncated=false，保留原失败；旧1及精确旧2非空真UI→schema3显式CLI迁移/旧binary回退、原两屏旅程与新权限首次链确在seq9执行。它仍不等于控制器/正式Actions联合迁移通过。

seq10独立QA给出No-Go并沿原development边返回：R01–R04为强制场景证据缺口，分别涉及双模型授权/期限与轮换异常UI、独立Q/L/C并发和跨窗口/回拨/迟到、app/key历史分页作用域与迟到结果、同一请求三次尝试的实际/未知用量。不是把未来正式发布责任强行前置给研发，也不是已复现四组实现故障。seq11补测试时真实红控发现process_interrupted且Finished为空的attempt可被迟到success覆盖，修正finishAttempt的终态更新条件并保留原success→canceled校正；旧QA快照/失败日志保留。原研发正式handoff提供107文件新指纹 `b807c2d9834d806f3b0578e99aa31d91fe5c0f485d9484bbabb5f2c1df56719f`，自动进入seq12固定tests。本记录时seq12仍执行，新QA及产品PR/合并/部署未发生；旧seq9绿灯不得外推新版本。维护者已对该冻结工作树另取只读快照启动合并前代码复核，结果待定。整体仍In Progress／No-Go。

### 2026-10-07：新版原型与材料入站核查（未实施Run）

本地独立设计包design-v0.1.0-draft已交付：前端现代后台、PRD、交互、AC、材料交接要求及浏览器走查截图。ZIP16文件、211596字节，SHA-256 `bef4ce741c8c9fd3f02e3eadcdca79056602532ce9d21bc5787af37cba4cf1d3`，重复打包一致且所有manifest文件摘要复核通过。这里只证明设计包，不是产品后台或供应商证据；运营模式默认内部成本分账，待用户评审。

正式API再次核对旧M3 Run `9e6d05f409675e0ef65c6486691462df`：stopped、seq13，保留原工作区，无恢复或新建任务。`GET /api/agents`显示现有model-relay研发Agent executor=codex、model为空、native_config为空；此前PRD明确DSH适配是后续目标。当前证据不足以确认用户指定DSH，已请求精确产品名称/本机启动方式，不能据此擅自换执行器并判定验收通过。

源码核查：github_entry首次快照只接收Issue标题/正文与issue_number，附件链接作为普通文字，未实现下载、完整性校验和输入版本锁定。既有SDK/API参数可承载材料描述，Connector回执与阶段inputs/artifacts可传递实际文件，无需另建上传服务。通用设计和计划位于 `docs/02-architecture/workflow-materials.md`、`docs/03-delivery/workflow-materials-plan.md`，当前待实现；没有宣称正式安装或真实附件路径已完成。旧部署未改变，本轮Issue、执行、QA、PR、部署继续Not Run。

### 2026-10-07：材料工具与通用模板源码（安装前）

维护源新增materials.py，Issue正文显式材料块冻结到parameters；prepare下载并核验ZIP/manifest及每文件，输出实际材料路径；固定项目测试和发布先检查原包摘要与展开树。安装器统一生成新命令，旧Run命令不变。源码提供标准安装/升级和SDK通用参数说明，不创建第二套上传服务。

相关模板回归53项、SDK9项、ruff、go vet及diff检查通过。独立审查发现2项Important：macOS父目录大小写碰撞导致失败包先发布并毒化重试；SIGKILL留下未清理临时输入。分别先复现失败，再修正全部目录前缀/Unicode碰撞和发布前复验、Run文件锁下孤儿清理。材料回归16项含真实SIGKILL、双进程并发、Git发布漂移拒绝和输入不入提交；原manifest首次/重复/升级/漂移保护回归通过（模拟API库存，非正式实例升级）。没有保留审查阻断项，但真实Pipeline仍待验。

已将design-v0.1.0-draft包发布为私有测试仓草稿Release（未批准实施基线），真实附件API：`https://api.github.com/repos/big91987/model-relay/releases/assets/616037961`。GitHub返回211596字节及相同SHA，受信下载工具通过官方API再次下载，核对16文件manifest和SHA均一致。该证据是实际传输校验探针，不是Workflow回执；没有创建新的产品Issue/Run或触发部署。浏览器Issue拖拽附件与阶段Agent实际引用仍未验证。

原manifest官方升级安全检查通过：没有引用本安装对象的在途Run；旧M3保持stopped/seq13。下一步原manifest备份和正式升级，随后等待原型反馈及指定执行器信息推进真实实施，整体目标继续No-Go。

### 2026-10-07：材料模板正式升级（产品链路仍未开始）

源版本 `ea332aa` 已推送既有源Draft PR #5，未合main。先通过官方升级安全检查确认无引用本安装对象的在途执行，将原manifest、CI入口配置、API对象定义及旧Run记录备份到私有维护备份目录；本次仅升级工具/配置，无二进制或存储迁移。使用原manifest、原工作区/仓库/prefix、原验证命令make verify及原预算执行install.py --upgrade，再执行完全相同命令；对象ID集合保持一致，没有额外Agent/Connector/Workflow。官方API对照旧M3 Run仍stopped/seq13，冻结connectors和parameters与备份逐项相同。

安装完成只证明新任务能获得版本材料准备和门禁配置，不证明真实Issue已入站、执行阶段实际引用、DSH实现、独立QA、PR或部署。这些要求仍待用户审阅原型与补充DSH身份后完成，当前目标继续No-Go；没有恢复旧Run或新建产品任务。

### 2026-10-07：用户指定 Codex / gpt-6.1-sol

用户明确将本轮被考核执行器由待确认的DSH改为Codex，模型指定为 `gpt-6.1-sol`；这解除执行器身份阻塞，不再要求DSH适配。通过原安装manifest的Installation升级入口和正式API，将既有intake、requirements、design、development、qa、report六个阶段配置统一为executor=codex、model=gpt-6.1-sol，保留对象ID、权限、Skill和工具绑定。升级前备份manifest和旧Run，升级安全检查确认无在途任务；API回读全部六个阶段模型一致，重复应用没有改变对象。旧M3 Run保持stopped/seq13且整份API记录与备份一致。

这是已保存的执行配置；尚无本轮原生执行会话，不能据配置声称模型实际执行或产品验收通过。原型与设计包仍为待用户评审草案，新的Issue/实施/QA/合并/部署未启动。整体端到端目标未完成，后续按用户指定Codex模型核验实际会话。

### 2026-10-07：产品基线自查与首次负责人原型修正

自查设计包PRD、交互、AC与必要前端行为，初始16文件均与已发布原manifest一致。发现创建租户直接将负责人标为active，跳过PRD要求的接受邀请。独立本地原型已按原产品规则修正：负责人邮箱必填，创建后invited，演示接受后active；浏览器先复现旧偏差，再验证缺失邮箱拒绝、待接受→已加入及最后有效管理员移除拒绝，error/warn为空。它仍不是真实身份/邮件/后端权限证据，未写产品实现或启动Run。

更新草案design-v0.1.1-draft，18文件、233983字节，SHA-256 `ffe8bb1ccbf0a4e3cb77a6765b1a1c184fb0d5bf9f3ad06ac9c9e2cc9d3569be`；重复打包相同，ZIP与manifest/源文件逐项核对，原v0.1.0发布ZIP摘要仍相同，新包未发布到GitHub。包内baseline-review.md明确G1自查NOT READY：开发者应用/Key分配与成员退出规则、账单结账/更正入口与权限、失败/未知多尝试的租户收费口径还需在需求阶段闭合。没有将自查当独立QA，不删除P0范围。

已集中询问用户是否以A工作台、内部多租户成本分账、API/已有端点管理、USD测试计价作为方向；该问题是待确认业务基线，与已解决的执行器选择分开。真实Issue材料交接和本轮实施、QA、合并部署未发生，目标继续未完成。

### 2026-10-07：用户授权自主冻结，真实Issue材料路径启动

用户明确允许设计负责人自行衡量测试原型，不再等待运营方式/布局批准。本轮design-v0.2.0冻结A工作台、内部成本分账、供应商API/已有推理端点、USD测试计价，PRD闭合开发者Key诊断分配/退出、平台管理员结账/追加更正、全部dispatch已知用量收费/未知待确认保守预留。原型补账单结账与更正、Key诊断成员编辑：实际浏览器验证当前周期/有未知用量拒绝结账、已结束无未知正常结账、-0.07更正保留原1661.58和调整周期，收回Key诊断范围保留已用842.32/预算1200且秘密不回显；JS语法、页面error/warn通过。这里只是原型证据，不是后端权限/精度/结算通过。

新包19文件、261636字节、SHA-256 `5f51d35fdc15ed7214749f9413879b4238097f1d6b6bbe2cc0d508d8e103cef7`，本地ZIP/manifest/源逐项核对后发布私有仓库prerelease [design-v0.2.0](https://github.com/big91987/model-relay/releases/tag/design-v0.2.0)。正式资产ID617014535、GitHub摘要和大小一致，原草案包保持原摘要。真实 [Issue #21](https://github.com/big91987/model-relay/issues/21) 引用精确REST资产URL、版本/SHA和全部AC01–AC18/H01–H08。原#20旧Run继续stopped，独立新Issue工作区不会并发写旧现场。

[Issues Actions37550125713](https://github.com/big91987/model-relay/actions/runs/37550125713)实际成功：原model-relay-local Runner通过维护源SDK正式入口接单，唯一新[Run2f589073fb909da91a3e6f76a8ba5430](http://127.0.0.1:8792/workflow-runs/2f589073fb909da91a3e6f76a8ba5430)。正式prepare seq1 exit0、保留日志845字节/truncated=false，回执实际下载核验19文件至该Run独立`.workflow-input`，材料版本/SHA与Issue相同；不是维护者手塞包或传输探针。seq2关联原Issue；seq3真实intake读取manifest、PRD/交互/AC/决定及最终整改，核对摘要并沿requirements交接，明确不得删产品范围或把旧AGENTS首轮范围覆盖新委托。seq4需求阶段正在执行。

管理员cookie按request键查询404是该接口按调用者owner隔离；改用原CI身份的SDK `workflow_by_request`实际返回上述同一Run，没有重建。页面通过原工作流的“运行记录”列表实际进入新Run详情，同列表保留旧暂停任务。原生intake与requirements各1个sessions JSONL的turn_context均记录model=gpt-6.1-sol、session_meta provider=platform_http；该证据证明原生会话选用模型，不推断供应商最终推理实现。原生证据仅私有归档身份字段，无认证文件。

自动审批拒绝运行中的workflow_dispatch重放，理由是可能重复或并发接单，保留现场约束优先；未绕过拒绝或重复提交，待本Run终结后再验证正式同事件重试。当前只完成真实材料准备、接单与一次交接，需求/设计/实现/固定tests/独立QA/PR/合并部署未完成，本轮整体仍In Progress/No-Go。

## 2026-10-07：角色、会话输入、交接预览与有界持续推进

**结论：本轮框架改造验收通过；产品/全部用户旅程目标仍 In Progress/No-Go。** 改造依据 [输入设计](../02-architecture/workflow-session-input.md)，实施记录见 [计划](workflow-session-input-plan.md)。真实执行器为 Codex，三个阶段原生 turn_context 均为 gpt-6.1-sol；未验证其他候选执行器或真实供应商。

### 维护源与安装

原业务 Run `2f589073fb909da91a3e6f76a8ba5430` 于 seq6 空闲等待，无执行进程，通过正式 stop API 冻结为 stopped。其工作区未改，已有快照/原生会话和冻结定义保留；原 manifest、运行二进制与 SQLite 在线只读一致备份已保存私有证据目录。仅重启开发实例，其他实例监听 PID 保持。新二进制/源码版本以本节交付提交和私有构建摘要为准，health 的通用 0.1.0 字段不代表此次提交。

正式原 manifest `--upgrade` 保留六个业务 Agent、全部 Connector 和工作流 ID；新工作流 revision5、context_version1、默认续跑3次/期限14400秒。需求/设计/研发/QA/报告/intake 预览全部实际通过。重启第一轮遗漏既有 HTTPS_PROXY 引用，由 Connector check 明确拒绝；从原 manifest 恢复同一已记录引用后沿同一升级恢复，未创建业务对象。原业务 Run 完整 JSON 与正式冻结后快照一致。

另通过明确命名的安装验收 manifest 新装完整模板；无 Issue/业务 Run/分支/PR 创建。实际重复升级发现 API 保存空 env_refs 为 null，旧 helper 不兼容，先 RED 后修复维护源 `proxy_env_refs`；同一 manifest 恢复、再次升级对象 ID/工作流 JSON 均不变，验收定义完成后停用。新安装和已有安装均获得角色/Session Prompt 分离及同一新协议。

### 页面与真实原生执行

[用户页面启动的 Run](http://127.0.0.1:8792/workflow-runs/e31cf26268c945cb4e4269f35e286441) 首次走需求→设计→研发→结束，seq4完成。需求故意正常收尾且未交接，平台在原会话追加一次 `workflow_continue`；原生同一 Session 连续3个 turn（初轮、自动继续、用户澄清），没有创建替代会话。第二轮真实调用 wait_for_input，明确问题后保持 idle/waiting，不继续循环。页面从 Run 进入会话回复 v2.1/CNY，修正原始美元默认；三个实际产物通过真实命令核对。设计和研发首轮实际输入都收到用户修正，而非仅靠模型猜测摘要。

需求/设计自主工具只有 handoff、wait_for_input、read_command_output；研发固定模式无 handoff，仅 complete_node、wait_for_input、read_command_output。角色实际 developer 中没有任务或 Run，每个会话原生项目规则正文仅注入一次，角色标记一次；三个已准备工作区的 AGENTS.md 摘要不变。默认托管工作区另用正式 invoke 调用真实 Codex，角色/项目双标记出现，seed AGENTS 字节保持，证明默认和外部路径共同生效。未读取原生私有推理。

同一 Run 完成后经正式 return 回需求（seq5）保留人工纠正；页面正式停止后未自动继续，正式 resume 沿同 Run/seq/原生会话接续。新用户要求更新 v2.2/CNY，需求→设计→研发再次真实产出并于seq8完成；旧 v2.1 只作为历史，三个产物采用v2.2。等待声明在接受用户输入/节点结果时清理，当前节点不再显示已解决问题。首次旧构建保留的历史等待记录显示为历史，不改写旧回执。

[真实有界续跑演练](http://127.0.0.1:8792/workflow-runs/19afcd9b38d15bae7822e7841bcc993a) 配置一次续跑，两个真实正常 turn 后明确 waiting/limit，continuations=1、无节点 Result，未推进结束。观察并保存后通过正式 stop 停止演练。节点时间期限另有真实1秒计时的引擎回归（原生turn结束后不再续跑）；未做4小时实等演练，不将次数证明替代时间边界实等证据。

[真实门禁失败与独立 QA 回退 Run](http://127.0.0.1:8792/workflow-runs/d63da2df56ad471ca3b13131d5c566d2) 使用独立测试工作区、真实文件与固定命令，故意安排缺项，不是产品成品验收。固定命令实际退出1→研发收到失败回执并修复→退出0→独立只读 QA 发现第二缺项→handoff 回研发（seq5）携带缺项和文件摘要→修复后命令退出0→独立 QA 实际核验通过→seq9完成。QA 两次独立会话使用 read-only 原生沙箱，无实现写入；最终真实文件摘要 `0cd059d12f5bf596c8ccfdef2d2d4a7def3aafbefdf428fa0f9fd2ce9c02d72e`。

### 修复与回归

原缺陷 RED→GREEN：角色混入任务、未完成正常 turn 无续跑。独立评审两项 Important 均有真实引擎 RED→GREEN：重复自主目标无法仅凭target选择，正式 stopped/Return 丢人工反馈。新协议同一目标只允许一条策略，旧协议路线兼容保持；回退读取取消执行的实际反馈。模板旧等待说明按误续跑风险提高为 Important，正式安装生成路径 RED→GREEN。实测又发现用户回复后旧等待声明残留及新安装 null 引用，均先 RED 后修正式事务/安装源。

最终受影响全套：Go race 全包通过、go vet通过；Node31项通过；平台工作流Python56项通过；Ruff全SDK/examples检查与格式通过、diff检查通过。Go链接器有既有Darwin LC_DYSYMTAB非致命警告，测试退出0。既有停止、授权、重复交接、迟到结果、重启与Connector恢复回归保留，没有删除门禁或以文字扫描替代完成。

| AC | 证据与边界 |
| --- | --- |
| 01–03 | 三节点实际输入/原生来源核对、默认与外部规则共存；无累计 previous_results JSON |
| 04 | 正式六节点预览，页面展开，图模型/UI同步回归，未知/重复占位符及重复目标拒绝 |
| 05 | 实际工具列表及两次固定完成；旧模式兼容回归 |
| 06–07 | 同原生会话澄清；v2.1→v2.2人工回退/停止恢复；真实命令失败与独立QA回退 |
| 08 | 新协议真实停止/恢复、原生重启后回退，既有去重/迟到/授权/外部恢复回归；本轮没有额外制造GitHub网络未知副作用 |
| 09 | 真实未完成续跑、明确澄清、用户停止及一次预算达到；4小时实等未测 |
| 10 | Run的实际输入窗口从权限API读取冻结角色/来源/工具/输入；不返回env、native配置或bearer；既有凭据脱敏逻辑复用 |
| 11 | 原manifest升级及独立新安装/同manifest恢复重复验证；旧Run冻结JSON不变 |
| 12 | 两条真实Codex链路与默认工作区；其他执行器/真实供应商未测 |

旧业务 Run仍正式 stopped，未静默升级其冻结协议或声称已自动恢复产品研发。升级后旧协议尚未进入的新业务节点直到发布的连续交付未复验，不能用本轮独立验收替代它。当前改造不新增其他供应商适配器，不恢复定时任务，不合并平台源 main。

## 2026-10-07：节点用户输入权限与生命周期补齐

结论：本节生命周期场景通过；整个测试产品与全部用户旅程继续 No-Go。复用上节真实证据，只补节点开关、迟到等待、上限呈现、原生 Hook 协同和真实请求失败。所有新增 Run 使用独立测试工作区，没有恢复旧 stopped 产品 Run、创建 GitHub Issue/业务交付或触发部署。

新增节点字段 allow_user_input 在页面、预览、冻结定义、MCP 注册和服务端事务一致。新节点/模板明确 true；旧新版定义缺失字段兼容之前开放语义，false 明确关闭。仅保留 wait_for_input 一个公开工具。轮次工具凭据在 Claim 原事务更新，旧原生进程不能向新轮提交等待/完成；有排队或 steering 回复时不能恢复旧等待。明确丢弃及投递不确定消息不作为下游需求，保留审计。manifest canonical 保留权限的 false，外部人工关闭不能被升级比较忽略。

### 新增真实路径

[节点权限/提问/回复 Run](http://127.0.0.1:8792/workflow-runs/8dab0ffd0a6e00cc3dcaaaab4c284f15)：从用户工作流页面启动，研发节点工具列表无 wait_for_input，明确缺少账单周期，真实 handoff 到允许提问的澄清节点。澄清真实 wait_for_input，Run 显示“请选择账单周期：按月或按年”。页面进入原会话回答按月，沿同一 native_record 继续，实际 billing.txt 为“账单周期：按月”，命令读取核对后 complete_node，seq3 completed。研发一条原始输入，澄清原始输入加真实回复，均无 workflow_continue；不是手写回执或模型口述推断。

[原生 Stop 与平台检查协同 Run](http://127.0.0.1:8792/workflow-runs/c04efb77dd0819bad777ec30fd064f63)：明确受信的独立验收 Hook 第一轮真实返回 decision:block，第二次 stop_hook_active=true 返回允许结束。Hook 两次日志属于同一 native turn；Agent 接续后真实 complete_node。平台队列只有一条原始用户输入，自动继续数为0，seq2 completed，不发生双方各排队一轮或重复推进。平台未注册原生 Hook 的普通 Codex 路径已有真实有界接续证据。本测试 Hook 不构成正式产品质量门禁或任何供应商通用适配保证。

[真实执行请求失败与正式恢复 Run](http://127.0.0.1:8792/workflow-runs/4803e87d1612960343f93cc9b386f3c6)：测试角色明确配置不可用本地端点，实际 native turn failed、请求错误502，节点 failed、无 Result、自动继续数0。保存失败事实后，通过正式 Agent 编辑修正配置；核对失败无文件/外部业务副作用，正式 stop→return 同一 worker，沿同一 Run/工作区的新 seq2 执行。旧失败执行保留为 cancelled，人工回退原因进入新的真实输入；Codex 实际写入并读取 recovery.txt，再 complete_node，seq3 completed。不是自动重放不确定请求，不创建替代 Run。

### LC 验证矩阵

| LC | 结果与证据层级 |
| --- | --- |
| 01 | 通过：上节真实自然收尾→同会话 workflow_continue；本轮引擎未完成去重回归继续通过 |
| 02 | 通过：真实 handoff/complete_node；重复完成只接受同结果、迟到等待不能盖已接受结果，Go事务回归 |
| 03 | 通过：本节真实节点提问、页面回答、同 native_record 完成；无多余平台消息 |
| 04 | 通过：真实关闭节点无工具并合法交回澄清；服务端拒绝false回归；无出口时具体阻塞/上限保留现场的SOP与页面语义，不声称已演练全部无出口业务 |
| 05 | 通过：上节页面停止→正式resume；本轮重启核对 stopped业务及上限演练仍stopped，Go停止/重启回归 |
| 06 | 通过：本节真实native请求失败→正式编辑/停止/回退恢复；上节固定命令失败和独立QA回退。GitHub网络未知副作用未在本轮重复制造 |
| 07 | 通过：上节真实次数达到，Go真实1秒时限；本轮UI上限不再写等待澄清，不标完成。4小时实等未测 |
| 08 | 通过：Go真实Store事务测试覆盖排队回复、跨轮迟到等待/完成、完成后迟到等待、重复tick、丢弃补充、失败/停止恢复；既有原生重启及本轮真实多轮工具证据。未做高并发压力演练 |
| 09 | 通过：本节受信原生Hook真实接续一次、平台不重复排队；无Hook普通Codex遵循平台检查。其他执行器未测 |
| 10 | 通过：页面关闭/预览、真实工具列表；当前模板独立新装及原manifest升级，字段/ID/重复安装一致。旧冻结业务Run完整JSON保持，独立安装测试对象停用 |

新增缺陷均先 RED→正式源修复→GREEN：关闭工具仍暴露/接受；排队回复和旧轮等待盖回；丢弃补充进入下游；预算上限误称澄清；manifest 忽略 explicit false。执行失败与重复当前结果等已经满足的场景补回归，不将其伪称新增 RED。最后全包 Go race/vet、34项 Node、58项平台模板 Python、全 SDK/examples Ruff格式与diff检查通过；独立只读评审未发现新增Critical/Important/Minor。Darwin既有链接器警告不影响测试退出0。

新安装需先准备已存在的工作区根；第一次缺该目录被安装器拒绝且未创建对象，准备独立根后用同一manifest继续。页面运行同样需要已存在目录，缺目录时未创建Run；准备后从原页面成功提交一次。没有绕过这些检查。

交付维护源接续提交 `fix: enforce node input permissions and isolate lifecycle callbacks`（继898e426之后）；开发工作流revision6，最终二进制SHA256 `d4479ad8c01e6e77ae5db56bb339cc338d66a3a4811ea0cf7c5320b6c75318d1`，源码草稿PR #5保留Open/Draft，未合并。

## 2026-10-07：画布与 Run 的右侧节点详情

本节仅验收节点信息呈现改造，整套产品与全部用户旅程仍 No-Go。参考 [Coze 官方节点开发说明](https://github.com/coze-dev/coze-studio/wiki/10.-Add-new-workflow-node-types-%28frontend%29) 的画布摘要/侧栏完整表单分工和 [Dify 官方面板实现](https://github.com/langgenius/dify/blob/main/web/app/components/workflow/panel/index.tsx) 的选中节点右侧面板；未复制其源码。

维护源新增一个仅负责呈现的共用面板，编辑器默认关闭，点击节点显示配置、交接及适用的输入预览。Run 的节点标题打开输入、结果、日志与记录；完整摘要与结构化回执收进详情，列表保留摘要。预览及冻结输入复用原正式 API 和权限，不改变工作流协议、安装 manifest 或运行状态。

真实浏览器验证主工作流 revision6 与已完成权限验收 Run：节点点击、页签、预览展开、关闭/重开草稿、方向键及 Esc、桌面与窄屏右侧布局、真实冻结输入、完整结果及原会话日志入口。Connector 来源分支在未保存草稿修改后关闭/重开仍保留；随后恢复原值并丢弃本页草稿，未保存业务定义，未执行 Connector。浏览器自动化在两次导航时超时，通过同浏览器新页继续核对；不将超时描述为产品操作成功。

独立只读评审发现 Connector 专用参数未同步导致草稿丢失，以及旧/失败预览响应覆盖新内容，均补充真实 RED→GREEN 回归并修复；同时修正历史等待提示和刷新打断阅读。41 项 Node 回归全部通过，Go 全包测试及 vet 通过，嵌入资产构建、JavaScript 语法与 diff 检查通过。没有新增 Run、Issue、PR 或业务执行，也未用本轮 UI 验证替代原业务链路验收。

升级使用原开发实例 CLI 和维护源二进制路径，升级前确认无在途执行，核对备份与冻结记录；旧 stopped Run 完整 JSON 不变，8788 实例不变。UI 资产随标准 Go embed 构建交付，标准新安装及既有实例更新二进制即可获得；本轮未重跑独立新安装（manifest 未变），此前安装/升级证据仍按原版本保留。已验二进制 SHA256：`1d107e5893056c86da5efec38ca0accd403d957810f8a56065384763f8a662f6`。源码草稿 PR #5 保持 Open/Draft，不合并 main。


## 节点独立配置与会话授权（2026-10-07）

工作流节点直接持有执行器、模型、角色指令、Skill、工具、权限、原生配置与环境。没有复制共享智能体入口，也没有新增模板库。旧引用图保留读取兼容，编辑保存及原manifest升级转为内嵌配置；旧Agent和已有Run不改写。普通调用方读取Workflow/Run时不返回管理配置。用户明确授权独立节点会话按当前工作流授权与Run所有者校验，旧共享Agent维持原规则。

后端红控复现无共享Agent无法启动，以及撤权后排队消息仍被Claim的问题。修复后，创建与步骤绑定同事务，读取、重复请求、回复、自动接续、MCP与恢复沿相同边界；Claim调度前再次检查账户与当前授权，撤权队列标失败。测试覆盖配置冻结、无隐藏Agent、跨用户及撤权、未关联空agent_id拒绝、非法配置、HTTP敏感字段、等待/停止/同会话恢复、冻结权限重置。前端修复删除节点遗留环境草稿、导出泄漏及非法导入；安装器比较完整执行字段，旧manifest遗漏但后加的工具也作为漂移拒绝。独立代码复审无剩余Critical/Important。

验证分层：

- 自动化：`go test ./... -timeout 120s`、`go test -race ./internal/platform -timeout 180s`、`go vet ./...`通过；完整前端55项与标准工作流安装器69项通过；Ruff、格式、diff检查通过。测试中的模拟API只证明对应隔离契约。
- 标准升级：先核验无在途Run/会话，备份数据库、二进制、manifest和API定义，再升级开发实例。原manifest把工作流 `fbdbfd95ea6c6992bf45bf697b0769e9` 从版本6升到7，六个节点内嵌配置，模型保持 `gpt-6.1-sol`；全部输入预览通过，重复升级图和revision不变。原共享Agent完整响应及旧停止Run `2f589073fb909da91a3e6f76a8ba5430` 完整JSON保持不变，另一实例监听进程未变。
- 新安装：原标准安装器创建独立验收安装，工作流 `b8ff315919e1de285bba9ce22da5343c`，不指定base Agent，显式指定模型；新增共享Agent为0，重复安装保持同一图。仅安装验证，未运行其产品交付流程。
- 页面：独立验收图 `d0d93ed62fb93431619374282f8e2109` 右侧直接配置，关闭详情保留名称草稿，保存版本2并刷新后仍存在；由页面运行按钮启动唯一Run `bff43a2d7a7f5718ebd6483613c74429`。原项目图版本7也实开研发节点，直接显示Codex、模型和角色配置。
- 真实执行：上述Run使用 `gpt-6.1-sol`，正式wait_for_input进入等待澄清；页面停止到stopped，再经继续原节点提交版本号。全程会话 `206e8dac89cf04a66049a15faefcea87` 未变，agent_id为空。Codex实际创建release.md，cat/sed/test/grep校验标题与版本 `v0.7.0-node-config`，命令exit0，scoped complete_node接受，Run完成。API与实际文件交叉核验，已有31个共享Agent没有新增或变更。

开发实例二进制SHA256：`5b67f10b0a8985f49cb92a487257c8de14d5f46a62451506eaeff8607ee2da78`。私有备份及API快照位于忽略的 `.data/workflow-evidence/node-agent-upgrade/`。页面入口：`<platform-url>/workflows/fbdbfd95ea6c6992bf45bf697b0769e9`；实测Run入口：`<platform-url>/workflow-runs/bff43a2d7a7f5718ebd6483613c74429`。

范围限制：本次通过节点配置独立、安装升级、等待和同会话恢复小任务；普通用户权限由真实Store/HTTP自动化覆盖，未另做普通账号浏览器旅程。本次未重跑完整GitHub→研发→QA→PR→部署链路，不改变既有整体验收结论。源改动交付现有草稿PR，不合并源main，不恢复定时任务。

## 配置一致性、输入与取消（2026-10-07）

范围结论：独立智能体/节点公共配置、v2输入、Run取消和小型真实协作通过；完整 model-relay 产品研发仍 In Progress / No-Go。维护分支 `codex/workflow-node-config-ui`，基线 `d7bd602`；只升级8792，8788进程保持不变，不合并平台源main。

### 可复用变更与验证

- 独立页与节点复用表单和读取/校验；名称、执行器、模型、角色、Skill、工具审批、权限、原生设置及环境贯通保存。拒绝数组环境，保留不可用的已选工具和错误草稿；打开关闭未修改节点不再产生脏状态。新节点没有 Session Prompt，也不提供实际不会生效的工作区模板目录。
- 新编排使用 context_version=2，平台自动生成交接/完成/等待规则；任务、修正和handoff进入User Input，原生执行器读取AGENTS.md。旧v0/v1与冻结Run保留。标准模板将长期职责迁入角色配置，一次Run仍只用启动时确定的工作区，不新增变量系统。
- 正式 stop→cancel 支持引擎、HTTP、SDK及网页，校验归属、seq和真实执行退出；历史/文件保留，当前会话关闭，终态禁止恢复/回退/继续，工作区占用释放。自动化覆盖幂等、重启、越权、迟到及排队状态。
- 新版标准安装、原manifest升级及重复升级真实通过：主图 `fbdbfd95ea6c6992bf45bf697b0769e9` revision7→8、六个Codex/gpt-6.1-sol节点；重复升级仍revision8。旧Run冻结定义和历史跨二进制重启不变。共享验收图新装和原manifest升级均通过，人工名称测试先经网页恢复，再标准升级，未篡改manifest绕过漂移保护。
- 自动化：完整前端59项、模板/入口Python71项、SDK9项通过；全Go测试、race及vet通过，gofmt/Ruff/格式/diff检查通过。SDK首次在沙箱不能bind，获得宿主执行权限后原测试通过；不将环境拒绝误记为产品回归。race的Darwin既有链接器警告未影响退出0。独立只读代码复审及增量复审均无剩余阻断项。

### 真实入口证据

| 路径 | 事实 |
| --- | --- |
| 独立智能体配置 | 网页创建“配置一致性验收”，数组环境被拒绝；保存/重载模型与对象环境成功。真实会话 `200a85814c94ca8f1a630bd18475d73f` 读取指定环境、写并读回文件，原生项目规则标记出现。该专用Agent已通过网页停用，保留历史。 |
| 网页配置与画布 | 验收图名称修改、保存重载成功；抽屉打开/关闭画布均宽892px，右栏覆盖而不占列；主图显示名称、Codex与模型，长配置可滚动。截图留私有证据。窄屏override调用后实际仍1280px，本轮不记作760px验收；上阶段窄屏证据不替代本轮。 |
| 首轮协作 | Run `ae6c20638add3ea9c70c41caa81820e6` 完成等待→回复→真实缺失→返工→复验。发现协作夹具错误继承研发make verify，真实exit2准确保留，不声称该命令通过；源模板随后排除研发专属指导。 |
| 最终模板复验 | Run `4fb12e7b2ba713ef656ad969466677b1`，独立工作区、同会话 `f422684e6fab8e2736852e0b8a90da90` 提问/答复。reviewer实际发现缺章节及项目规则标记并交回writer；补齐后独立命令逐项PASS，seq6 completed。真实release.md含v2.0.7-final、PROJECT_RULE_V2_FINAL及验收章节。无make verify跨模板要求。 |
| 旧任务取消 | Run `9e6d05f409675e0ef65c6486691462df` seq13、`2f589073fb909da91a3e6f76a8ba5430` seq6均通过正式API成为cancelled，旧工作区及未提交代码保留。无数据库/检查点改写。 |

GitHub双向入口实测发现：平台建立的测试Issue #22又被opened入口接为新研发Run。维护源修复为忽略正文带保留平台标记的opened事件，forward再次读取GitHub事实后拒绝手动dispatch；原仓库/操作者校验不变。真实重跑[Actions 37613210087 attempt2](https://github.com/big91987/model-relay/actions/runs/37613210087)记录 `Platform output ignored`，没有新增重复Run；随后模板复验Issue #23同样没有反射创建研发Run。标记是输出协议而非身份认证；用户复制此标记同样拒绝，手册已说明原Run页面继续。

已产生的重复Run `a34ed977d6aafc90531af1d683f49910` 通过正式接口停止，仍stopped。其额外取消被自动审批以“两个旧Run的取消授权未明确覆盖这个新Run”拒绝；未重试、未绕过，已询问用户。它与新产品任务工作区独立，不阻塞后续验证。

运行二进制SHA256 `3fe723f60485d6aa69d037459f696cabfadb368974ec93510feb04f75538295c`。本机备份在忽略的 `.data/maintenance-backups/20261007-config-input-v2*`，API/文件证据在 `.data/workflow-evidence/config-input-v2/`；辅助备份脚本不是正式部署机制。源码交付以构建二进制、原manifest升级和既有GitHub入口标准安装为准。

### 完整产品链路进行中

[model-relay #24](https://github.com/big91987/model-relay/issues/24) 明确关联旧已取消任务，保留其未验收现场，从标准main checkout重新承接同一冻结design-v0.2.0材料。[入站Actions 37615595429](https://github.com/big91987/model-relay/actions/runs/37615595429)成功，唯一新Run `713a325b560ddc53be37ff3a0bbc5014` revision8/context_version2已完成prepare和原Issue关联，开始intake。设计材料摘要仍为 `5f51d35fdc15ed7214749f9413879b4238097f1d6b6bbe2cc0d508d8e103cef7`。

后续固定测试、独立QA/返工、PR、授权合并、正式部署及5545实际用户旅程仍须逐项留证；上述小任务不能替代产品端到端验收。供应商、其他执行器及高并发压力未测试，不外推保证。

平台实现已提交 `1830f81` 并推送 [草稿 PR #6](https://github.com/big91987/agent_platform/pull/6)，base为已封版 `codex/platform-workflows`；源main未合并。产品Run已通过intake进入requirements，当前结果仍不能替代最终交付。

用户随后纠正：Platform 此阶段只在新分支开发，不需要创建PR。上述PR #6（`codex/workflow-node-config-ui` → `codex/platform-workflows`）已关闭，未合并、未删除分支或提交。后续平台改动直接提交/推送当前开发分支；测试仓研发流程的PR、授权合并和部署要求保持。

### 产品回归：材料与接单幂等补证

原Run `713a325b560ddc53be37ff3a0bbc5014` 已完成requirements，实际产物为本Run的product-definition.md、acceptance-matrix.md、g1-review.md；矩阵完整保留AC01–AC18/H01–H08，明确产品项待实现/待验证。G1只判Ready for Architecture，未冒充产品Go。设计节点seq5正在执行；原生进程存活，已落architecture.md/runtime-contracts.md，阶段尚未完成，不预判设计验收。

真实重跑[原Issue #24入站Actions 37615595429 attempt2](https://github.com/big91987/model-relay/actions/runs/37615595429)成功，正式API查询issue_number=24仍仅一个Run、ID不变；没有重新创建工作区/Run或替换当前阶段。接单、需求、设计三节点分别通过正式context API与原生turn_context交叉核对，配置和实际模型均为gpt-6.1-sol，执行器Codex，冻结revision8/context_version2。原生记录分别为01a11629-dc4b-7890-82f1-c0e8038eb015、01a1162b-f34a-79f1-99d8-a7aad51e567e、01a11632-c6b6-7090-80f9-d7ef1acf2ee5；输入SHA256和原生身份另存私有product-model-input-evidence.json。这些是材料承接/幂等/执行身份的分项证据，研发、QA和发布仍未通过。


### Agent 交接模式与固定输出（2026-10-07）

Agent 节点配置增加互斥的自主交接/固定流转；后者配置唯一目标、`inputs` JSON Schema与填写说明。配置进入冻结节点，生成工具参数与首轮输入；校验不通过不保存、不推进。旧版混合图与在途运行保持原行为。切换多目标需明确选择保留路径，无效JSON草稿关闭后仍保留并阻止保存。显式空对象保持 `{}`；数值在UI/原始MCP/存储解码之前做十进制往返检查，不可无损表示的数字明确拒绝，精确长编号/高精度值使用string。

独立复审发现并以RED→GREEN关闭空对象丢字段、MCP/存储数值舍入；同步修正handoff说明使用真实必填summary。最终标准`scripts/verify.sh` exit0：Go vet、全仓race（platform49.667s）、Ruff、安装器/Python/SDK和浏览器回归通过；之后追加Schema草稿数值保真测试，63项Node回归及构建通过。未把测试文本中的临时路径作为平台默认配置。

仅升级8792，PID28917，二进制SHA256 `5c83de9b4018ae821b138079e06be0fa63253326f69252a02fd407ed817a538f`。升级前确认无运行/排队原生会话，备份数据库与已核对的旧二进制；升级后原研发waiting Run及两个取消历史逐JSON一致，8788 PID82011不变。

真实浏览器在[独立验收编排](http://127.0.0.1:8792/workflows/aa204b67bfc09f261283330b2ae30f5d)切换固定流转，输入无效JSON、关闭面板后保存被拒绝；重新打开保留原草稿。填写Schema/说明后保存revision2，重载与输入预览均显示冻结前的正确契约。[真实Run](http://127.0.0.1:8792/workflow-runs/b8250145f04f6ce079d3322fcf0e5e4e) completed seq3；Codex会话`b5ac37c8feb625176267f0db646a4788`原生事件133348明确拒绝unexpected字段，133373接受修正结果。Agent结束后Command Connector读取原始previous_results，实测布尔true、整数2、字符串数组和嵌套metadata对象，并校验Agent实际文件，exit0；消费回执由命令生成。没有用工具调用文字代替真实执行。

原协作manifest标准升级成功，编排`94dbd79c540c732568f0de1bb41e9715` revision5；重复安装不变，两条历史Run保持冻结。原研发manifest升级被在途Run保护拒绝，未绕过保护、未停止或取消它；原研发模板仍revision8，待原任务结束后升级。该研发Run `713a325b560ddc53be37ff3a0bbc5014` seq10等待原生沙箱之外的受支持宿主回归，完整产品尚未验收。平台本功能局部通过不等于完整研发流程完成。

私有原始回执、升级备份、原生事件与安装器日志位于ignored `.data/workflow-evidence/completion-contract/` 与 `.data/maintenance-backups/20261007-completion-contract/`。截图保留在私有临时目录，不提交原生历史或环境配置。


## 全新服务与测试仓Pipeline（2026-10-07）

用户要求不继续保留旧配置路径，部署新端口并在测试仓重新起Pipeline。当前源码构建为独立二进制，服务监听8793，使用新的平台数据、CI账户、安装manifest、浏览器工具证据目录和任务工作区；旧服务未参与这次验证。GitHub入口YAML及来源manifest摘要与维护源一致，仓库变量通过正式接口切换到新服务CI配置。

标准安装器新装编排 `8f497228b60a8ea46b7d37be59ebe578` revision1/context2，六阶段Agent直接内嵌Codex/gpt-6.1-sol及handoff配置。真实浏览器在实际研发节点确认可编辑名称、执行器、模型、角色和统一权限/Skills/工具表单，交接页具有自主交接和固定流转两种选择，无独立Session Prompt。

[model-relay #25](https://github.com/big91987/model-relay/issues/25) 触发 [Actions37643773819](https://github.com/big91987/model-relay/actions/runs/37643773819)，正式SDK创建唯一Run `4cd62ced6a4f98f50e3cca6f7596eb06`。prepare exit0：main基线 `98511771871cf0951ecef716bbb55deb13e18da5`，独立任务分支，design-v0.2.0摘要 `5f51d35fdc15ed7214749f9413879b4238097f1d6b6bbe2cc0d508d8e103cef7`、19文件，材料回执传入后续阶段；Issue节点完成，seq3 intake原生会话 `2484b357ad325a0d06346eeba69e4688` 已完成真实handoff至requirements，seq4需求会话 `e49f561b3423651e08efd1ce59635d20` 正在执行。此证据只证明全新标准入口和原生执行开始，不代表完整产品研发、QA或正式部署已通过。运行与安装证据保存在忽略目录 `.data/fresh-8793/`。


## 8793典型场景实测（2026-10-07）

本轮在测试仓 `big91987/model-relay` 的独立checkout验流程，不修改产品源码或门禁。部署基线为维护分支 `codex/workflow-node-config-ui` 的HEAD `9931ff6cafe2b2a01d0f2ae1a603fbec563c35da` 加未提交的completion/统一UI改动，运行二进制SHA256 `b7b2eeec1a1ef7bd6b49781eb35d0fd367237ee6d726bca0c13e34dfe32039b8`。不是已发布源版本；私有 `baseline.json`、`source.patch` 和新增completion源文件记录此次快照。测试期间未更换二进制、未重启服务或修改主产品Run。

准备边界：标准安装器安装collaboration-check，GitHub入口已有checkout逻辑准备两个独立测试仓目录；固定输出场景通过正式配置API预制图和只读校验Connector，是明确的流程验收夹具。固定校验只检查release.md和结构化输入，不替代产品make verify，不宣称完整产品功能通过。两条Run均从浏览器运行表单输入任务/工作区启动；停止、恢复及人工回退均由同一管理员浏览器会话完成。未验证从空状态通过UI创建全部配置的旅程。

| 场景 | 操作与最终事实 | 证据类型 | 结果 |
|---|---|---|---|
| 正常阶段交接 | 主Run接入→需求→设计；材料摘要/路径经真实handoff传递，当前seq5设计执行中 | API-E2E、原生工具、External State | Pass（仅已执行阶段） |
| GitHub重复接单 | 正式workflow_dispatch重放Issue25，Actions37644991582成功返回原Run；主编排Run仍恰好1条 | External State、API-E2E | Pass |
| 澄清等待 | 标准writer未获版本号，调用wait_for_input并显示等待澄清；未自行编造版本或推进 | UI-E2E、原生工具 | Pass |
| 等待态停止与恢复 | 页面停止到stopped、填继续说明答复版本号，再继续原节点；会话ID不变，前后原生thread.started均为 `01a11701-b3c7-7713-86ba-1a74bbbfa540` | UI-E2E、Data Reconciliation | Pass |
| 独立校验返工 | reviewer实际读文件发现缺章节→handoff writer修复→reviewer命令复验→done；release.md最终符合版本/章节/文字要求 | UI-E2E、原生工具、文件核对 | Pass |
| 完成后人工回退 | 同一Run完成seq6后，页面选择reviewer并填写原因；seq7重新核验、seq8done；前6步历史保留，未重建Issue | UI-E2E、API-E2E | Pass |
| Schema错误可纠正 | complete_node含unexpected时原生事件1947拒绝，事件2134纠正后接受；错误输出未产生下一步，最终只保存合规输入 | 原生工具、API-E2E | Pass |
| 固定命令失败返工 | 原命令真实exit1（缺少验收章节）→repair读取原日志并修正文件→同一Connector原命令exit0→done | UI-E2E、External State、API-E2E | Pass |
| 类型化消费 | 宿主命令实际断言布尔/整数/数组/嵌套对象，consumer-receipt.json记录原类型及执行seq4 | External State、文件核对 | Pass |
| 跨运行所有者隔离 | 使用已授权CI caller读取admin拥有的验收Run和会话，两入口均403 | API-E2E | Pass |
| 主产品完整研发/QA/PR/合并部署 | 主Issue25仍在设计；当前未产生本轮完整产品验收或发布结果 | — | Not Run（后续阶段） |
| 正在执行的进程中断、服务重启恢复、外部响应丢失 | 本轮未中断主研发服务，等待态停止不可替代这些场景 | — | Not Run |

实测Run：[澄清与返工](http://127.0.0.1:8793/workflow-runs/c4afbc7f6ad82e65b9a9ddd5eff81bb6) 完成8步；[Schema与固定命令返工](http://127.0.0.1:8793/workflow-runs/dd9d5ec416dfa23a42458ef843f9ab09) 完成5步。平台生成Issue26及Hook评论均未引发反射Run，主Issue25重复入口仅返回原任务。测试Issue26已正式关闭；测试checkout只新增release.md/consumer-receipt.json，无产品源码修改或提交，验收文件留在忽略存储供复查；无在途测试Run。主产品Run继续执行。

证据目录：维护源忽略目录 `.data/fresh-8793/scenarios/`，包含基线、两条Run完整JSON、各节点原生事件、命令退出回执、跨用户拒绝及GitHub通知；页面截图 `/private/tmp/agent-platform-8793-typical-scenarios.png`。不提交原生历史或测试仓本机数据。

本轮没有在已执行场景发现流程阻断。质量较上一轮从“创建Run和入口显示”推进到真实等待/恢复、主动与固定交接、两类返工和人工回退闭环；整条产品交付仍In Progress，不能据此判断全面Go。后续页面优化候选：失败命令在节点生命周期上显示“已完成”，同时另列exit1/failed，对用户不够直观；仅记录，按用户要求先完成流程验证再优化页面及跳转。


## 半小时检查：研发与公开参考覆盖（2026-10-08 00:18）

Issue25主Run已由seq5 design正式handoff至seq6 development，会话 `3ea939a86321137ea64d7c4f6f6451df`，原生thread `01a11712-97dd-7b70-b438-288da4552303`。五份设计文档实际存在，26行AC/H矩阵完整。未建新任务；GitHub当前Open PR为0。研发工作树已有身份、Key、费用、控制台API和迁移改动，源码产出与需求/设计文档分开记录；研发原生消息报告开始接入工作台及迁移。

原生事件接口每页最多300条，本次按after完整分页得到2929条，最新事件2026-10-07 16:20:03UTC，conversation仍running。第一页最新时间15:56不能用于判断停滞。局部Go命令证据包括 `TestConsoleIdentityAndKeys` / `TestMicroDollarPrecision` exit0（原生event5696，运行日志实际耗时1.323s）；这只是当时局部工作树回归，未取得最终make verify/固定tests Connector、独立QA、PR或正式部署证据。后续改动必须在同版本重新验收。配置模型为gpt-6.1-sol；当前节点自身没有独立供应商实际模型回执，不把配置声明改写成外部模型验真。

公开对照检查采用材料已有官方来源，不读取本机New API：

| 公开参考的用户能力 | 当前冻结设计承接 | 当前实测/缺口 |
|---|---|---|
| [New API渠道管理](https://docs.newapi.pro/en/docs/guide/feature-guide/admin/channel)：渠道表格、配置对话框、自定义端点、模型映射及核验 | 服务接入/核验与状态、模型发布/授权、真实工作台 | 设计覆盖，研发中；未验真实UI。参考中的批量运营、多个Key轮询、参数覆盖等不能自动视为当前范围已覆盖，列后续差距 |
| [LiteLLM Admin UI](https://docs.litellm.ai/docs/proxy/ui)：连贯模型、Key及费用入口，人员邀请/登录 | A工作台与成员身份/接受邀请/服务端scope | 身份相关局部Go回归有exit0；全旅程UI/权限/部署未验 |
| [LiteLLM Virtual Keys](https://docs.litellm.ai/docs/proxy/virtual_keys)：模型权限与团队/用户边界、预算及限流 | 稳定Key轮换、租户/Key独立资源约束、共享额度及历史 | Key轮换局部Go回归有exit0；预算并发、未知用量、完整管理旅程待固定tests与QA |

用户最新对齐目标已正式提交至原Run的User Input，稳定request_id `model-relay-heartbeat-20261008T0018-alignment`，message1548，接收后沿原研发会话处理。要求本Run文档明确实现/局部测试/宿主完整测试/QA/发布分别对应的真实事实、公开能力差距与后续需求，并将实际路径交QA；不静默扩写冻结材料、不以对照文档代替代码和门禁。无需重复要求用户授权普通推荐。

检查证据保存在忽略目录 `.data/fresh-8793/heartbeat-20261008*.json`：正式Run、Issue、完整原生分页、局部命令退出结果和补充输入回执。当前未发现需救援的流程阻塞，状态为真实研发持续推进；整链In Progress，完整产品完成度不报虚构百分比。

## 半小时检查：真实工作台测试接入（2026-10-08 00:48）

正式API核对主Run仍running/seq6 development，原会话仍running、无错误或等待原因。完整分页取得3554条原生事件，最新时间2026-10-07 16:48:09UTC，研发持续活动。GitHub当前Open PR为0；尚无本轮产品QA、合并或部署。上一轮补充User Input message1548实际仍queued：接口接收成功不等于Agent已处理，本次不重复提交，也不把商业覆盖对照算已产出。

源码核对：新增 `tests/browser/console.go`、`console.mjs`，现有 `tests/browser/main.go` 已调用 `consoleJourney()` 并在失败时退出。JS旅程访问实际Go服务 `/admin/`，覆盖1280、1440、390三种宽度及移动菜单；既有 `make verify` 的browser目标仍通过 `go run ./tests/browser` 调用。因此三视口旅程已写入固定测试入口，但本轮未取得实际浏览器执行结果，状态为已实现/待测试，不能依据Agent自报“已加入门禁”判Pass。

新增局部命令证据：原生event6430执行 `go test ./... -run '^(TestConsoleIdentityAndKeys|TestMicroDollarPrecision|TestInProcessContract|TestDeploymentContractAndIdempotentUpgradeCLI)$' -count=1`，exit0，relay2.115s/CLI2.777s；event6466及6668执行 `TestConsoleAccountingInProcess`，均exit0，最新1.179s。这些是限定选择的回归，未覆盖完整suite、race或最终make verify。

| 卡点/限制 | 具体影响与证据 | 原因及处理 | 当前结论/后续 |
|---|---|---|---|
| 原生沙箱禁止真实HTTP监听 | event6390：`TestConsoleActualHTTPQuotaBillingAndSSE` exit1，`listen tcp 127.0.0.1:0: bind: operation not permitted`；无法在原生执行器内完成该HTTP验证 | 已知执行边界，Agent明确记录不计通过，继续可执行的局部计费回归；完整测试使用既有宿主tests Connector | 待研发完成后经正常handoff触发固定 `make verify`；未临时扩大权限、删测试或修改门禁，未在持续改动中的工作树抢跑宿主验证 |

本轮无新的Harness根因缺陷或需人工救援的停滞。检查证据在忽略目录 `.data/fresh-8793/check-20261008-0048/`，包括Run、conversation、完整原生分页、局部测试退出结果及Issue。继续等正式固定测试及独立QA，按同一版本记录完整结果；商业覆盖补充待原会话处理后核验实际文档路径。

## 半小时检查：固定门禁失败留证与局部修复（2026-10-08 01:18）

主Run仍running/seq6 development，原会话无错误、未等待，5597条完整分页事件核对至2026-10-07 17:17:04UTC。研发继续修改多尝试计费、权限、迁移及浏览器旅程，没有Open PR或新的产品发布Actions。message1548仍queued，不重复创建输入/任务，也不认定商业对齐文档已完成。

本次获得实际固定命令失败：event7695在17:05:56UTC执行make verify，exit2，于health-history中止，真实HTTP监听 `bind: operation not permitted` 导致历史HTTP回归拒绝。日志实际保存在产品本Run `validation/attempt-01/make-verify.log`。这是原生执行器内的失败尝试，没有宿主tests Connector回执；后续全量测试及UI未因此通过。研发任务T001已明确稳定监听限制不重复运行、完整门禁由宿主Connector负责；原生消息同样明确未通过，继续可运行实现和检查。目前仍研发中，待正式handoff进入既有tests节点，不抢跑持续修改中的工作树，不删门禁或增加执行权限。

新增可核对的局部事实：event7961执行 `go vet ./...` exit0；event8358执行身份/Key、金额精度及计费三个定向测试，exit0（relay1.302s）。新治理/邀请限速回归先在event8585编译失败，再在event8678断言失败，研发修改后event8698的 `TestConsoleGovernanceInProcess|TestConsoleInvitationRateLimit` 定向命令exit0（0.970s），失败历史保留。以上均是当时工作树上的局部检查；产品还在改动，没有完整race/UI/独立QA，不能标记对应完整AC已验收。

完成度：有持续产品代码和局部测试产出；固定门禁本次Fail（原生监听限制），宿主完整验证Not Run；独立QA、PR、合并、正式部署及health/UI版本核对仍Not Run。研发任务T001的实施事实段仍为初始化文字，暂不能用该段判断实际代码完成度，继续以原生命令/源码/正式Run核对，并待交接时核验更新后的任务记录。证据保存在忽略目录 `.data/fresh-8793/check-20261008-0118/`。

## 半小时检查：资源、成员范围及迁移定向回归（2026-10-08 01:48）

正式Run仍running/seq6 development，原会话无错误或等待，完整分页6883条事件至2026-10-07 17:47:38UTC。原生消息报告在同步运维说明、Trellis契约及版本证据；当前主Run尚未handoff固定tests。GitHub Open PR仍0，无新产品部署Actions；商业补充message1548仍queued，不重复投递。

核对失败到修正的实际内容，区分产品修复与测试修正：

| 原失败 | 后续改动与实际证据 | 结论边界 |
|---|---|---|
| event8719资源回归`key limit 409` | 测试按已存在的资源版本填写Revision1，再更新用Revision2；event8744定向回归exit0（1.653s） | 测试修正版本前置条件，不能称为产品限额故障已修复 |
| 同一event8719成员回归`assigned key read 403` | console_keys.go允许非管理员GET，写操作仍限制；同轮补Key更新幂等处理，注销入口移至业务写权限判断之前；event8744成员范围定向回归exit0 | 有产品源码修复和局部检查，实际HTTP、UI与独立权限QA仍未验 |
| event8846 schema1迁移回归`legacy digest identity lost` | event8975将schema1断言从appID改为generationIdentity，schema2仍走appID；迁移定向命令exit0（0.981s），覆盖备份失败保持原版本、迁移事实、schema2正式历史费用核对及备份恢复 | 通过来自调整测试所调用的身份入口；此处没有迁移产品代码修复证据。schema1真实HTTP身份旅程仍需宿主/独立QA核验 |

event9473扩大定向选择至InProcess、身份/Key、金额精度、迁移守恒、邀请限速及Key代际截止，exit0（3.237s）。event9663及9699的JS语法/Go vet检查exit0。event8993使用 `go test ./... -run '^$'`，仅编译检查、没有测试执行，不算全仓回归。新增迁移/崩溃/browser测试仍需实际宿主门禁回执，Agent自报“加入门禁”不能替代执行。

本轮未重复运行已知禁止监听的make verify，也未停止持续活动的研发进程。完整宿主门禁、独立QA、PR、合并、正式部署、health版本及实际用户页面仍Not Run；本Run研发任务T001实施事实段尚为初始化内容，待正式交接核验更新和真实产物。原始证据在忽略目录 `.data/fresh-8793/check-20261008-0148/`；持续按原Harness推进。

## 半小时检查：研发候选、追加输入送达与前端修正（2026-10-08 02:18）

主Run仍running/seq6 development；原会话无错误，完整分页8608条事件至2026-10-07 18:17:59UTC。T001已更新为In Review、implementation-r1，研发验证report与逐AC源码/用例/待验映射实际存在，原初始化实施段已替换。所有完整验收复选框仍未勾选，没有Open PR、独立QA或产品部署。候选源码仍未提交，HEAD不能代替工作树指纹。

追加输入接续已真实发生：原生event10609尝试正式handoff宿主tests，被拒绝原因是用户输入已保存待送达，工具明确要求正常结束当前turn、先处理输入再重试。Agent正常结束后，原会话收到message1548，当前状态running且队列为空，明确回复将补功能/旅程对照，不要求重发、未创建新Run或会话。此处验证了在途追加输入不会被交接丢弃；宿主tests尚未接受，不能把尝试交接算阶段已推进。

非监听补充证据已公开保存在本Runvalidation：attempt-02格式exit2后修正；attempt-03格式/module/vet/JS/119项原Node测试/指定Go race/build/diff-check exit0；attempt-04历史未定价风险展示及迁移守恒race exit0，并保存来源指纹。两者仅证明各自当时工作树的补充检查，真实HTTP/SSE、kill/restart、三视口浏览器与完整make verify仍待宿主。

处理商业补充时，Agent发现详情抽屉误用限额摘要，继续修正详情展示、列表搜索及Token汇总，并增加前端回归。attempt-05、07补充检查exit2，attempt-06 exit0；attempt-07为新增跨列表分组搜索用例未找到row-29。源码新增搜索后，event11725复验仍失败；event11845修正测试DOM夹具读取textContent的优先顺序，attempt-08实际exit0：127项Node测试、格式/vet/build/JS检查及计费in-process race（4.027s），没有把失败断言删除。测试修正与产品改动分开记录，这仍不是实际浏览器旅程通过。

继续动作：研发完成商业覆盖/缺口文档，更新新增改动后的来源指纹与待验映射，再正式handoff既有宿主tests；当前对照文档尚未核验完成，旧attempt-04指纹不能覆盖后续改动。首个完整make verify的监听限制失败历史保留，完整宿主门禁、独立QA、PR/合并/正式发布及health/UI一致性仍Not Run。检查与后续复核原始证据在忽略目录 `.data/fresh-8793/check-20261008-0218/`。

## 半小时检查：宿主完整测试失败并正式返工（2026-10-08 02:48）

正式API核对seq6已completed并接受handoff；seq7 tests真实执行 `make verify`、exit2、route=failed，自动推进seq8 development（新会话 `c9dd3fc093c183bd6e3f683803acbde5`，running）。无手动回退、重建任务或数据修补。首次宿主测试指纹 `93b2b67d21d7e1dfbeffd02b2f26a3b414cca7fc0dd781c358c3c91871e44fc2`，基线HEAD仍9851177；后续返工版本必须重新运行完整门禁。

简短Connector回执只有有界输出，不能用其缺少测试名判断未执行。本次经正式 `/steps/7/output?offset=...` 三页读取至eof，保留日志82637 bytes、truncated=false（UTF-8解码82319字符）。完整日志确认127项Node、Go vet、全仓 `go test -race -count=1 -v ./...` 和构建实际完成；relay325.006s。新增真实HTTP治理、资源维度、人员scope、额度/计费/SSE、真实进程崩溃恢复、迁移守恒、身份/Key等测试有明确PASS，首次将这些从Not Run推进为该指纹下的实际宿主分项通过。完整make verify仍Fail，因为browser在迁移步骤中止；不能外推后续三视口新控制台旅程通过。

| 卡点 | 用户影响与证据 | 原因/处理 | 当前复验及后续 |
|---|---|---|---|
| 迁移浏览器驱动与生产升级目标不一致 | seq7日志 `migration browser verification failed: candidate upgrade response invalid`，完整门禁无法完成，独立QA/发布未进入 | seq8读取原命令日志，定位旧驱动期待schema2，而正式CLI升级至schema3；产品Agent修改仓库测试驱动，保留schema1→2历史UI/数据守恒检查，用明确test-only schema2兼容入口承载该层，另增加生产CLI schema1/2→3、身份/历史/映射守恒、真实调用及三视口新管理页旅程 | attempt-09非监听补充exit0：目标契约race7.607s、迁移守恒race5.048s及Node127项/vet/build/JS检查。实际完整浏览器尚待正式宿主tests重跑；测试源码存在不算通过，不降级生产迁移目标 |

源码核对 `migrationJourney()` 仍由原browser入口调用；新增生产分支实际调用候选CLI upgrade、要求schema3、逐表核对身份/历史/映射、受控HTTP调用，再运行 `migration-console.mjs`。这是产品测试驱动修复，不是平台权限或监听修补，不由维护者代写产品。原失败日志和本次回归已保留；尚无独立QA裁决，仍需核对驱动与断言实际执行。

商业覆盖/缺口及J01–J07旅程对照现已实际写入原 `acceptance-matrix.md`，区分实现、局部测试、完整宿主、QA和发布，列协议/渠道批量运营、企业身份、自动轮换、外部财务和高可用等冻结范围外差距，没有静默改写design-v0.2.0或声明完全商用对齐。message1548已处理完成，原研发会话closed。下游QA须对照同冻结交互材料，不能把范围内缺陷归为未来差距。当前Open PR为0，QA、合并、部署及health/UI版本一致性仍Not Run。

证据在忽略目录 `.data/fresh-8793/check-20261008-0248/`：Run、原生会话/事件、正式完整分页命令日志、当前返工检查。主流程正在真实失败→研发修正闭环中，继续沿原handoff重验固定门禁，无需用户输入。

## 半小时检查：迁移Key列表契约修正与第三次宿主门禁（2026-10-08 03:18）

主Run真实推进：seq8返工已completed，seq9宿主tests再次exit2并走failed边；seq10研发修正已completed并接受handoff，当前seq11 tests running。没有waiting/error或待处理输入，未人工重建任务、重复启动测试或停止进程。Open PR仍0；最新GitHub Actions为Issue入口，不是产品部署。

seq9完整留存日志经正式output API三页读至eof，83947 bytes、truncated=false；绑定117文件、工作树SHA `f059c17ab3a1b0edf181d612d2907b0a755ba50605bca737901061abe99739ff`。全仓Go race/构建再次完成，20条包含TestConsole的PASS记录。browser确认旧schema1→2兼容UI迁移、两个模型映射/Key/历史及旧二进制回退通过；生产schema3控制台在keys-1280失败，不能认定该视口及之后三视口旅程通过。粗粒度阶段只能定位至Key检查，不能单凭此日志认定唯一失败原因。

| 卡点 | 根因证据及修复 | 复验及限制 |
|---|---|---|
| 迁移控制台Key检查失败 | seq10源码核对正式列表包络为items/total，旧driver读取data；仅修migration-console.mjs及共用断言，保留恰好两个Key、total及启停身份对应检查。增加实际Go handler/SQLite集合契约测试，8项Node回归拒绝旧包络、缺/多Key、错误total或身份/启停变化；增加逐视口/HTTP/契约阶段的安全诊断 | 候选migration-driver-r3；135项Node、指定Key集合/迁移守恒race（7.542s）、格式/module/vet/build/JS检查exit0。第三次完整宿主make verify正在seq11执行，尚无最终退出码或实际browser通过证据 |

原迁移驱动断言仍实际调用 `verifyMigratedKeys(body)`；health要求schema3、两行Key可见、历史待确认非零、页面无横向溢出和三视口循环均保留。此次修复属于测试包络错配，没有改产品API、迁移CLI/数据、依赖或权限，不把旧包络兼容加回生产契约。源测试纳入既有固定门禁，未skip或减少断言。

当前候选来源：119文件、Go1.25.6、HEAD9851177（未提交候选不能仅按HEAD识别），工作树SHA `bd81e49e060f7e0494e1217947a0e9367fab394b19a64bfe0845b427addbecc6`；产品本Run `validation/attempt-10/` 保留checks.md、source-evidence.json、host-seq09-summary.md和补充日志。独立QA、PR、合并、正式部署、health/UI版本一致性仍Not Run，完整门禁未通过。商业对照保持同一矩阵/冻结材料，无新增范围重写。原始检查及seq9全日志在忽略目录 `.data/fresh-8793/check-20261008-0318/`，继续等正式门禁回执并按原路径返工或进入QA。

## 半小时检查：生产迁移实测通过、服务核验修正（2026-10-08 03:48）

seq11第三次宿主make verify真实exit2；原failed边进入seq12研发，修正后已正式handoff，当前seq13 tests running（第四次宿主完整门禁）。主Run无错误/等待，原任务与历史保留；无重复建Run或额外手动测试。Open PR为0，最新GitHub Actions仍为Issue入口，QA/发布未开始。

seq11完整日志经正式output API三页读取至eof，86204 bytes、truncated=false，指纹bd81e49e…与上一候选一致。全仓race有CLI8.782s、relay330.653s、browser包7.066s、health-history5.370s实际通过。生产CLI的schema1→3和schema2→3均通过身份/历史/映射守恒、受控HTTP及1280/1440/390原生管理页旅程；旧schema1→2兼容UI/回退仍通过。旧Go UI、版本/恢复/日志分页旅程和多上游/失败恢复/SSE取消在1280及390也通过，均仅受控上游证据。新增完整控制台旅程停在service-1280，后续新增十页/调用/权限旅程仍不能判Pass，完整门禁仍Fail。

| 卡点/影响 | 定位及修正 | 复验与剩余动作 |
|---|---|---|
| 新控制台服务核验不能通过，阻止门禁及QA | seq11仅报告service-1280 AssertionError；seq12源码/新回归确认服务行核验POST遗漏expected_execution_revision，修workspace.js显式传入实际execution_revision，保留Go版本guard；受控models列表fixture另补协议必需object=list。两个原因分开记录，不归因沙箱 | 修复前Node新增2例失败、Go核验用例因fixture列表缺标记失败，原red日志保留；137项Node及核验/Key集合/迁移守恒的指定race9.808s、格式/module/vet/build/JS检查exit0。当前seq13宿主完整make verify重新执行，未取得退出码或新控制台实际browser通过 |

修复只涉及前端核验契约、夹具及对应测试/安全诊断，没有改变后台版本校验、迁移CLI、权限或固定门禁，也未删原断言。产品Agent维护本Run `validation/attempt-12/` checks、红/绿日志、材料回执及source-evidence；最终119文件、Go1.25.6、HEAD9851177、候选工作树SHA `312cae2097f665d7e50ab6ac11290dd54fe444c6c6a37ed5dda233108d8bc101`。旧指纹的成功分项不可移作此修正版本的完整通过。

继续核验seq13正式回执：门禁失败沿原development返工，仅exit0后进入独立QA；冻结矩阵和商业/J01–J07差距保持，不将局部成功作商用对齐结论。独立QA、PR、合并、正式Actions部署及health/UI一致性仍Not Run；真实供应商/支付/外部身份无凭据仍Not Run。当前原始证据在忽略目录 `.data/fresh-8793/check-20261008-0348/`，本轮无需用户介入。

## 半小时检查：服务核验实测闭合与模型驱动精确匹配（2026-10-08 04:18）

seq13第四次宿主make verify exit2，原failed边自动进入seq14研发；seq14修复已completed，当前seq15第五次宿主tests running，无错误/等待。原任务/日志保留，未额外启动测试。GitHub Open PR仍0、最新Actions为Issue入口；独立QA、PR/合并、正式部署及health/UI一致性未开始。

seq13完整日志经正式output API三页读至eof，86640 bytes、truncated=false；指纹312cae20…与service-check-r1候选一致。全仓race实际通过（CLI9.372s、relay330.853s、browser包8.816s、health-history6.459s），生产两种schema3迁移三视口、旧UI及多上游旅程继续通过。新控制台失败阶段已从service推进至model-1280：安全诊断显示创建服务201、核验200、success/applicable true且execution_revision存在/一致，确认前轮服务核验修复已在真实页面生效；该记录不能证明模型创建及后续旅程通过。

| 卡点 | 最小定位/改动 | 实际复验与限制 |
|---|---|---|
| 新控制台模型表单driver选择歧义 | 产品模型编辑器有“远端模型名”及包含该子串的附加路由标签，driver模糊getByLabel匹配2处；精确匹配仅1处。只改console.mjs为exact:true并断言唯一，保留原模型POST/租户授权/后续断言，不改产品标签/后台、不用first或API预置替代操作 | 原驱动新增回归exit1（2匹配），驱动修正后VM缺assert注入再失败，修正fixture后138项Node及格式/module/vet/build/JS检查exit0。本地Chromium启动诊断仍permission denied，未到DOM/网络，明确不是原生验证；正式浏览器待seq15宿主结果 |

核对新增回归执行实际modelEditor字段与driver片段，确认只写alias/upstream/remote三字段，不操作附加routes。分段诊断保留安全枚举及remote_label_matches，不泄露输入/响应/秘密。seq14产品本Run `validation/attempt-14/` 保留红/绿日志、checks、材料及source-evidence；候选model-driver-r1，119文件、Go1.25.6、HEAD9851177、工作树SHA `34f3a82f0c7687194a60e0cbce6af66e6badff55b0b50a373a0690764e6e7fa3`，冻结材料不变。

完整宿主门禁尚未通过；seq15若失败继续按真实证据返工，只有exit0后进入独立QA。商业功能/J01–J07对照与范围外差距仍在原矩阵，未改验收范围。检查及正式seq13全日志在忽略目录 `.data/fresh-8793/check-20261008-0418/`。本轮无新的平台源码缺陷或必须人工救援的停滞。

## 半小时检查：Key超时保留现场与最小诊断（2026-10-08 04:48）

seq15第五次宿主make verify exit2，正式failed边进入seq16；seq16最小诊断候选已completed并接受handoff，当前seq17第六次完整tests running。主Run无error/wait，未重复启动测试或要求重发输入。GitHub Open PR仍0，最新Actions为Issue入口；QA、合并、正式部署及health/UI一致性仍Not Run。

正式output API三页取得seq15完整86806 bytes日志、eof/truncated=false；绑定model-driver-r1指纹34f3a82f…，全仓race实际通过（relay332.565s），生产迁移三视口、旧UI及多上游旅程继续通过。新增控制台已越过model进入key-1280，remote_label_matches=1，表明模型精确定位修复实际生效；最后一条TimeoutError的粗阶段同时覆盖编辑/签发/秘密/调用/详情，不能据此判断Key创建失败，也不能借已有write_status201推断Key签发成功。

| 当前卡点 | 最小处理及已尝试 | 尚未解决/下步 |
|---|---|---|
| 新控制台Key旅程超时，阻止完整验收与QA | seq16仅在原console.mjs拆成11个Key动作阶段，添加catalog/members/签发/call/detail数值状态、page枚举/布尔及控件计数；观察器不读响应body、不输出秘密/账号/ID/URL/原异常，不改变页面。141项Node诊断投影/observer及格式/module/vet/build/JS检查exit0；原定位、动作、断言保留，无sleep、重试或产品修补 | 根因仍未知，待seq17真实宿主stage/status/DOM投影。可能边界包括编辑依赖/选项、提交、一次秘密转交、调用或详情；这些仅是假设，不能将源码close/navigate顺序或VM检查写成真实浏览器时序结论 |

该候选名key-diagnostic-r1，**不是Key修复**。本轮未改产品UI/Go协议/SQLite、依赖、Makefile或权限；原生监听/Chromium限制已知，不再从原生环境重复必败尝试。产品本Run `validation/attempt-16/` 保留checks、host-seq15-summary、最终non-listening-final日志及source-evidence；119文件、Go1.25.6、HEAD9851177、工作树SHA `66b873dcdae343c12265140898fddf3fed9d5c298db1b89b507d6d806d029995`，两次来源核对一致。完整门禁及新控制台后续三视口尚未通过，不认定Key根因已解决。

继续由原宿主tests取得具体失败边界，再由产品Agent做最小修复/复验；若退出成功仍须独立QA逐AC/H裁决。商业对照与冻结材料不变，真实供应商/支付/外部身份仍无联调事实。检查和seq15全日志在忽略目录 `.data/fresh-8793/check-20261008-0448/`；当前无需用户补充权限或输入。

## 半小时检查：Key转交根因闭合与键盘准备状态（2026-10-08 05:18）

正式状态已推进至seq20 development running：seq17第六次宿主make verify exit2→seq18产品修复→seq19第七次宿主make verify exit2→seq20。无error/wait或待输入，原任务持续活动，不重复启动命令。GitHub Open PR仍0，独立QA/发布未开始。seq17/19完整日志分别经正式output API三页读至eof，87698/88101 bytes、truncated=false，不能沿用简短回执作为完整证据。

| 卡点 | 真实定位/修复 | 实际复验边界 |
|---|---|---|
| Key调用超时 | seq17指纹66b873dc…的安全日志定位key-call-http-1280：签发201、有secret/id、已在playground，却call_key_nonempty=false、无call状态。seq18确认dialog close处理全局清理，误清转交后的call-key；修workspace.js关闭仅清本dialog，显式转交捕获epoch避免旧导航迟到复原秘密 | key-transfer-r1，四项受控close/transfer/重开/跨epoch回归及145项Node通过。seq19指纹b6139420…真实越过原卡点：签发/转交、正常及SSE调用、同Request ID详情、轮换、邀请、丢回执读回、限额拒绝及1280十页导航完成，最终停keyboard-1280；日志request_detail_status200、最终call_status429是限额拒绝，不是首次调用失败。该候选Key闭环已在此视口实际执行，但三视口/整套验收仍未通过 |
| 键盘旅程断言失败 | seq19粗keyboard阶段AssertionError不能区分首次Tab、圈定、Escape或回触发点。seq20仅细分阶段/最多16条布尔焦点轨迹；源码进一步确认keyEditor依赖异步加载，driver点击后立即Tab而未等editor可见，添加locator.waitFor(visible)及延迟打开RED→GREEN回归 | 最终152项Node/格式/module/vet/build/JS检查exit0；产品dialog/Go/权限未改，保留首次Tab/12循环/Escape不可见/准确返回触发按钮全部断言，无强制focus、sleep或重试。确定的driver准备缺口已修，但仍不认定是seq19唯一根因或产品焦点已通过，待正式宿主重验 |

seq18候选Key来源SHA `b61394208bd1f0c4fd58d466390fbfc794c703c5eafeec47b6fe903ce6bc5bdf`；seq20 keyboard-ready-r1当前119文件、Go1.25.6、HEAD9851177、工作树SHA `71be54206d9850fa442f33c18ddd3c7aaba5590019543a78498a128ae0ab8350`。产品本Runattempt-18保留Key实际红绿及清理边界说明，attempt-20保留诊断/延迟准备回归、最终非监听日志和来源一致性。受控调度/VM不作为真实Chromium证据，旧指纹分项成功不可外推当前完整Pass。

下一步seq20完成正式handoff后由原tests完整复验，根据keyboard分段与焦点投影做最小判断；只有当前候选make verify exit0后进入独立QA。冻结AC/H和商业/J01–J07范围不变。完整三视口新控制台、QA、PR/合并、正式Actions部署及health/UI一致性仍未完成。检查/正式全日志在忽略目录 `.data/fresh-8793/check-20261008-0518/`，本轮无需用户介入。

本轮结束前正式API再次核对：seq20已completed并接受handoff，seq21 tests running，第八次宿主完整make verify正在执行；尚无退出回执。最新状态保存在同证据目录run-final.json。

## 半小时检查：固定门禁首次通过、QA缺口与容量故障恢复（2026-10-08 05:48）

seq21第八次宿主完整 `make verify` **exit0**，正式进入seq22独立QA。正式output API三页读至eof，88327 bytes、truncated=false；119文件、Go1.25.6、HEAD9851177、工作树SHA `71be54206d9850fa442f33c18ddd3c7aaba5590019543a78498a128ae0ab8350`。全仓race/build实际通过（CLI8.917s、relay332.902s、browser包7.969s、health-history5.854s），152项Node及现有后端、恢复、生产迁移、旧UI/多上游断言完成。新console-real-go-ui在1280/1440/390均明确result=passed，覆盖脚本现有十页、签发/调用/费用/限额、弹层键盘及秘密清理。此前keyboard路径在此版本通过；未增加sleep/强制focus/删断言。

这是一份**固定门禁通过**，不是全部冻结产品验收通过。QA独立读取同材料和seq21完整日志、复核来源及局部非监听检查后，实际落盘 `acceptance/acceptance-matrix.md`、`acceptance/remediation-handoff.md` 和host-seq21-review：

| QA发现 | 当前用户影响/证据层级 | 接续要求 |
|---|---|---|
| QA22-01，P1，Open：强制连续UI旅程覆盖不足 | 现有脚本无法证明真实租户管理员完整授权收窄/修正、1h/24h过渡、费用硬预算耗尽/提高恢复、两租户暂停恢复、账单核对/关闭/更正、分页/迟到范围恢复等；是验收覆盖缺陷，不宣称这些功能已实测失败 | 保持冻结J01–J07及verification-design要求；产品Agent补真实浏览器连续步骤/角色/起始状态/最终事实，保留现有断言，再完整宿主门禁/独立QA。不得用API预填隐藏UI步骤或SQL改账/时间 |
| QA22-02，P2，Open：Key列表状态不准确 | 静态审查实际formatter仅按revoked，未按enabled/expires区分，页面无法明确辨认停用/过期状态；当前是Static Review，不冒充已在浏览器制造过期Key复现 | 根据服务端安全元数据呈现状态并补真实列表用例；该P2单项不作为No-Go依据 |

QA当前结论No-Go，报告/正式development交接尚未完成时遭原生外部容量故障：seq22及Run failed，`codexErrorInfo:serverOverloaded / Selected model is at capacity`（21:38:12UTC），无产品门禁回退或权限问题。本轮先保存原Run/会话/事件及已落盘报告，不换指定模型、不重跑已成功测试；利用既有CI所有者凭据经正式 `/resume`，seq22 + 继续说明提交。202 Accepted，原会话 `01a6ef633f44ccf8bfb2bb9148366a28` running、Run running，消息4132承接已写现场、要求完成报告并正式handoff整改。

恢复后原生thread.started再次为 `01a11845-346f-75c1-a3e3-427b87476028`，与故障前相同；新的原生消息/命令及177条新事件核对至21:50:13UTC，暂无再次容量错误，QA继续整理No-Go报告。此证据验证**真实外部容量失败→正式恢复→原线程继续执行**；不是所有外部故障已保证恢复，也不是QA已通过。未手改数据库/检查点、未注入新权限或重建Run。

下一步QA完成正式返工交接，按原Pipeline修覆盖和状态、重新固定测试再独立QA；本次exit0指纹不得挪用到后续改动。GitHub Open PR为0，最新Actions仍Issue入口，PR/授权合并/正式部署及health/UI版本一致性未完成；真实供应商/支付/外部身份Not Run。维护源无新通用缺陷被证明，容量故障通过既有受支持恢复处理。原始证据及正式恢复回执在忽略目录 `.data/fresh-8793/check-20261008-0548/`，不保存公开凭据。当前无需用户介入。

## 半小时检查：独立QA正式返工与连续旅程候选（2026-10-08 06:18）

seq22独立QA已completed、原会话closed；完整报告、逐AC/H矩阵、J01–J07及整改交接已落本Run，结论仍No-Go。原生handoff事件22443正式接受development，当前seq23研发running、会话`2c461ab639cea990b90cd0dd29b20b13`，无error/wait。容量故障经正式resume后已完成同native线程接续→报告→正式返工交接，恢复场景闭合；不等于产品QA通过，也不保证所有外部失败恢复。没有重建Run、换模型、重复User Input或人工启动另一门禁。

| QA缺口/新发现 | 候选处理及维护者核对 | 实测状态与剩余动作 |
|---|---|---|
| QA22-01 P1连续UI覆盖 | 产品Agent新增journeys.go/mjs：每视口独立production InitConsole/Open临时SQLite及真实Go服务，J01–J07从UI创建/邀请/接受/登录并持续操作；涵盖授权拒绝修正、代际过渡/期限、预算恢复、双租户暂停与成员权限、账单核对/结账/更正、日志分页及迟到范围。原browser main追加fullConsoleJourneys，原迁移/控制台等断言保留 | 新浏览器旅程全部Not Run，不能按源码存在计Pass。需当前指纹完整make verify及3视口×7旅程回执/安全失败现场，再独立QA复审；QA22-01保持Open |
| QA22-02 P2列表状态/信息 | workspace.js新增启用/停用/过期/过渡/撤销/未知状态与应用、授权、已用/预留/预算列；新增真实函数契约回归 | 156项Node通过，其中新增状态/驱动契约及空租户回归；真实过期/停用列表待浏览器执行。QA22-02保持Open |
| 新发现：移除成员后无租户仍显示加载 | 源码确认needTenant append空态未移除旧加载占位，产品Agent改replaceChildren并补原函数回归；请求详情增加当前租户/Request ID日志入口 | 属源码/受控回归证据，未称本地真实浏览器复现；相关连续成员/日志旅程待宿主 |

夹具边界经源码核对：每1280/1440/390实例分别创建临时目录，production init自带空legacy租户如实保留，没有SQL写入/修改日期/补业务数据。另一随机loopback端口以随机capability保护测试专用clock推进和dispatch计数；仅注入已有Config.Clock，无生产调试路由，认证会话仍真实wall clock。UI写操作均由实际表单/按钮提交，API仅读回；受控上游/读失败时序不冒充真实供应商。结束关闭所有server/store/context及临时目录。测试受控时间证明的是产品期限/账期行为，不扩展为人员会话时间验证。

当前continuous-journeys-r1候选122文件、Go1.25.6、HEAD9851177，工作树SHA `cb51d80c2e424c6953d66cdfd2c319ad07e3744158c38ee5153b4c18cc98a4f0`；候选未提交，不能仅按HEAD识别。attempt-23保存checks、frontend-test/final-static、checks-final、source-evidence及material-source。156项Node、7项定向Go race、format/module/vet/build/JS/diff检查exit0；`go test -run '^$' ./tests/browser`仅编译，不是浏览器执行，也不是全仓race。seq21旧指纹71be…的完整绿保留，不能挪用为此新候选通过。首次指纹生成缓存访问失败在checks记录，重做所得非空JSON才作为来源；未把组合shell末尾exit0当失败子命令通过。

当前GitHub Open PR0，最新Actions仍Issue入口，产品5545仍旧main9851177/schema2；无本轮PR/合并/正式部署/health与新页面一致证据。冻结design-v0.2.0及原商业/旅程矩阵保持，真实供应商/支付/外部身份Not Run。主Run正常活动，继续原handoff→固定tests→独立QA，无需人工救援或新增权限。平台维护源未发现需新补丁的通用根因，仍保留当前开发分支WIP、不新建Platform PR。正式实时快照/原生事件在忽略目录`.data/fresh-8793/check-20261008-0618/`，不提交原生历史或秘密。

## 半小时检查：连续旅程初始状态错配与正式返工（2026-10-08 06:48）

seq23已completed并正式handoff，最终新增详情日志筛选检查使Node总数157，122文件最终工作树SHA `182077621a4dbc04d73b8517a40f70811c9c8e46f435d7d9596edcfb5018bab2`，与seq24宿主来源一致；06:18的cb51d80c…只是交接前检查快照，不作为本次门禁版本。seq24第九次完整make verify真实exit2，正式failed边自动进入seq25 development，当前会话`7103f664dd712bccad7dba7403c5dfb3` running，无error/wait。未重复启动门禁或发送相同输入。

正式output API三页读至eof，89307 bytes、truncated=false。全仓race实际通过（CLI8.626s、relay330.410s、browser包7.365s、health-history5.399s），build、历史健康预期红灯、旧兼容迁移/两种生产schema3迁移三视口、旧UI及多上游旅程继续通过。console-real-go-ui在1280/1440/390均passed，证明前轮新增Key展示/空态等候选未使既有门禁失败，但不能据此证明新规定全部状态/期限旅程通过。新continuous-console在1280的`J01-empty-state / authoritative-read` assertion失败，checkpoints=[]，未到任何J01完成或后续J02–J07/其他视口，不补造通过记录。安全失败JSON实际存在test-results/continuous-1280-82376.json，无秘密。

| 卡点及用户影响 | 最小诊断/已尝试处理 | 复验与后续 |
|---|---|---|
| 新连续旅程错误初态断言阻止完整验收 | seq25读取原完整日志/规范，源码明确InitConsole的schema3新安装在同初始化事务移除legacy；旧driver却要求一个legacy租户，再读其Key。真正迁移保留legacy与新安装为空是两种入口。只修journeys.mjs为upstreams/models/tenants均零、选择器空，按资源拆阶段和记录安全计数；新增真实InitConsole/Open+handler集合回归，生产初始化/数据库/权限未改 | 原生命令事件25239实际node_exit=0/go_exit=0；attempt-25 journey-test.log 6/6通过，TestConsoleFreshInstallationEmptyCollections三资源定向race4.041s通过（in-process，非真实网络）。当前为driver修复候选，不是完整浏览器Pass；仍需原固定make verify及独立QA |

**维护者记录更正：**06:18夹具审计中的“production init自带空legacy租户如实保留”判断不准确，当时把driver/docs的预期当成了初始化事实。现明确核对store.go InitConsole/initSchema源码和定向真实初始化测试：全新schema3无租户；legacy属于升级兼容数据。保留上一轮历史以便追溯，本节替代其该项事实判断。测试夹具不手改DB的边界仍成立；不能把生产初始化事务内的正式清理误写成测试造数据。研发attempt-23同样遗留了该描述，正在本轮整改记录中更正，不据此静默改变冻结输入。

QA22-01/02继续Open，当前新连续旅程仍未通过。seq25正常活动并已明确根因/执行定向回归，沿原handoff接完整宿主重验，无需人工resume或换模型。GitHub Open PR0，最新37698335357/37697341708仅Issue入口success，未发布产品；PR/合并/Actions部署及新health/UI一致性仍未完成，真实供应商/支付/外部身份Not Run。当前证据在忽略目录`.data/fresh-8793/check-20261008-0648/`，平台源分支/在途服务保持，不新建Platform PR。

## 半小时检查：新安装初态实测闭合与受控响应角色修正（2026-10-08 07:18）

seq25正式交接后，seq26第十次宿主完整make verify exit2；原failed边进入seq27，整改已completed并正式handoff，当前seq28第十一次完整tests running、Run running，无error/wait。不重复创建任务或手动启动另一测试，不需User Input/恢复操作。正式seq26日志经output API三页读至eof，89976 bytes、truncated=false，122文件/Go1.25.6/HEAD9851177，工作树SHA `ed726cdd9609135b02904e2022ef20faa65e1b0b1796e723f9d1af1293a8f071`，与seq25交接一致。

seq26全仓race（CLI8.788s、relay334.661s、browser包7.662s、health-history5.466s）、158项Node/build、旧/生产迁移与旧UI/多上游继续通过，console-real-go-ui三视口均passed。新增continuous-console实际越过初态，安全回执initial_counts为upstreams/models/tenants各0，停在1280 `J01-responsibility-chain / proxy-call`，last_status502、checkpoints=[]。这证明前轮新安装driver修正已生效，但尚无任何J01完成、后续J02–J07或新增1440/390通过；502是实际调用断点，不归为原生沙箱限制。

| 卡点与用户影响 | 最小诊断/处理证据 | 当前结论与后续 |
|---|---|---|
| 新连续旅程首调用502，阻止完整验证/QA | seq27追踪实际journeys.go受控上游→生产validResponse：known/unknown两响应均缺message.role，而validator要求assistant。提取同一实际handler供runner/回归共用，两分支原fixture回归先红（原生事件26470日志独立exit_code1，外层收集命令exit0不作测试绿）；仅补两响应role，已知3/4/7和未知usage缺省保留，未放宽生产parser/身份/计费/SQL | 修后实际fixture、生产缺role/错role拒绝及assistant接受、原空集合/会计in-process定向race四顶层通过，事件26658；159项Node及format/module/vet/build/browser compile/JS/diff exit0，事件26797。这是非监听证据，未证明宿主唯一原因或新UI通过，待seq28实际回执 |

新TestContinuousUpstreamRelayHTTP已接入原Go门禁：真实临时受控上游HTTP、schema3管理/调用handler与SQLite对账，验证已知费14/成本7及未知pending1；本地因既知监听边界未执行，测试源码存在不是Pass，也不称为整个relay服务网络/browser实测。失败driver保留原200/dispatch/旅程断言，追加5秒有界GET同Request ID的安全终态、错误白名单、上游状态、最多3尝试数值摘要；GET诊断失败不替代原status断言，不输出ID/地址/正文/秘密。Node安全投影正反例已执行。

当前controlled-completion-r1候选124文件、Go1.25.6、HEAD9851177，工作树SHA `9fd82d0300ce6e3b0879662a8194522f7beb9b1a99e69ad3a32b153c9825a687`，以attempt-27/source-evidence及正式handoff为准；checks、expected-red、protocol-fixture、journey-test、material-source及host-seq26-summary均在本Run。QA22-01/02仍Open，原No-Go有效，旧指纹分项绿不能作为新候选完整通过。新增3视口×7旅程、独立QA逐AC/H及正式发布继续待验；冻结design-v0.2.0/商业矩阵不改，真实供应商/外部身份/支付Not Run。

GitHub Open PR0，最新37701331059/37700157826/37699171748仅Issue入口success，未新建PR/合并/部署，产品当前仍旧main9851177/schema2；新版本health/UI一致性未完成。平台源无新通用根因修复，保持开发分支WIP与8793在途运行，不新建Platform PR、不碰其他服务/工作树。完整日志、Run/会话/原生验证事件在忽略目录`.data/fresh-8793/check-20261008-0718/`，无必须用户介入的问题。

## 早间累计成果复核：四条连续旅程通过，暂停恢复继续诊断（2026-10-08）

用户询问整夜累计解决问题时再次读取正式实时API：seq28完整make verify已completed/exit2，seq29研发completed，当前seq30 tests running，无error。seq28正式output三页读至eof，92097 bytes、truncated=false，完整日志保存忽略目录`.data/fresh-8793/check-20261008-morning-summary/seq28-full-command.log`。原生产迁移/旧UI/多上游与console-real-go-ui三视口继续通过；新增continuous1280明确J01/J02/J03/J04各passed，覆盖新安装到责任链调用、授权收窄/修正、轮换/期限及预算拒绝恢复。受控响应role修正已推进越过前轮首调用断点。

新增旅程停在`J05-resume / authoritative-read`，HTTP200、checkpoints仅J01–J04；原费用守恒断言未通过，日志尚无比较金额，不能直接归因生产收费错误，也不能称J05已通过。seq29正式交接明确pause-conservation-diagnostic-r1仅诊断候选：保留原费用断言，增加暂停拒绝/Beta调用/恢复的before/after白名单诊断及双租户双Key定向回归；未改生产计费/权限/DB。当前候选124文件SHA `6263734f52bb1631801f5db0f27d44f7a12de0e64f1fad4ffdf003c244b6dd92`，局部检查以该节点回执为准，完整实测待seq30。新增J05后续/J06/J07及1440/390连续仍未完成，QA22-01/02未关闭、原No-Go有效，不能按先前固定门禁绿宣称产品整链完成。继续真实回执→最小归因→正式返工或独立QA，发布仍待完成。

## 半小时检查：费用守恒实测成立，成员刷新边界继续定位（2026-10-08 08:47）

seq30第十二次宿主完整make verify exit2，原failed边正式进入seq31 development，当前会话`46816e3282988da7f8d2a59f77c9ecfb` running、原生事件分页核对仍活跃，无error/wait。完整seq30日志经正式output API三页读至eof，92708 bytes、truncated=false，绑定124文件SHA `6263734f52bb1631801f5db0f27d44f7a12de0e64f1fad4ffdf003c244b6dd92`，未重复启动测试/Run。161项Node、全仓race（CLI8.775s、relay346.581s、browser9.828s、health-history4.915s）、build、迁移/旧UI/多上游及console-real-go-ui三个视口实际通过；TestContinuousUpstreamRelayHTTP known/unknown在宿主通过2.53s，属于受控上游HTTP+生产handler/持久化，不冒充真实供应商。

新增continuous1280的J01–J04继续passed，最终仍粗`J05-resume / authoritative-read` assertion。新增安全usage_facts显示同账期，费用126→126微美元、预留0→0、调用9→9、pending0→0、并发0→0；因此实际已执行的费用守恒成立。**更正早间记录：**仅凭粗stage和最后一次read200不能断言失败的是费用守恒；该标签之后还有成员降权/移除/诊断范围/UI租户选项/跨租户隔离断言。此前“费用守恒断言失败”保留为当时错误归因，本节替代该判断。不把守恒成立外推整个J05通过，真实精确失败位置仍待细分回执。

| 当前卡点/影响 | 最小诊断及已尝试 | 复验与下步 |
|---|---|---|
| J05后续成员边界失败，阻止连续验收 | seq31保持生产Go/UI/权限/计费/DB不变，拆分member-lookup/role-change/readback/diagnostic-recovery/remove/removed-key-scope/membership/UI-scope/application-survives/other-tenant-request阶段；新增安全读取状态、成员角色/选项计数及限定源文件行列定位，不打印原错误堆栈或秘密 | PeopleScope in-process定向race11.598s实际通过，事件29523，非浏览器机制或全链结果 |
| 已证driver刷新准备缺口（实际原断点仍待确认） | 产品refresh按钮先异步syncTenants，再进入页面加载；旧driver点击后仅ready可能在旧内容仍可见时提前结束。实际driver片段延迟回归先红，refresh-red.log/事件29765记录独立exit_code1；候选在成员移除场景等待明确租户选项数0后再ready，保留原零选项断言，不sleep/重试/强制改DOM | driver文件12项回归exit0，事件29773。只证明确定性准备缺口，不能称seq30唯一根因或成员撤权真实页面已Pass；完整候选尚在研发，待原handoff/tests获得分段原始事实 |

本轮产品源码修改仍由Pipeline Agent承担。平台Harness的固定tests失败→正式development返工持续正常，未发现需要新通用平台补丁的阻断，未手改数据库/检查点或临时扩大权限。QA22-01/02仍Open，J05后续/J06/J07及其他视口continuous未通过、独立QA/PR/合并/部署未完成，冻结输入/商业矩阵保持，真实供应商/支付/外部身份Not Run。

外部状态查询：首次gh GraphQL TLS handshake timeout且后续Actions串行命令未执行，未将失败读取记为PR0；改正式GitHub REST分别只读核验，PR列表空、近期37704106006/37703181902/37702285408均Issue entry success，并非部署。一次读取恢复成功，无循环盲试/新权限/外部写入。原始Run/完整日志/seq31原生消息与事件在忽略目录`.data/fresh-8793/check-20261008-0847/`，当前无需用户介入；平台/产品事实分别记录。

## 半小时检查：成员与日志旅程实测通过，追加账本驱动修正（2026-10-08 09:17）

seq31已正式交tests，seq32第十三次完整make verify exit2，原failed边自动返seq33；seq33 completed并正式handoff，当前seq34第十四次完整tests running，Run无error/wait。正式seq32输出三页读至eof，93546 bytes、truncated=false，124文件Go1.25.6、HEAD9851177、工作树SHA `de6d4d88d809bedacf06d00693c51cb4fb7159b2d2c986c3bf80a025a270b978`。164项Node、全仓race（CLI10.091s、relay356.337s、browser11.101s、health-history5.303s）、build、生产迁移/旧UI/多上游及原console-real-go-ui三视口实际通过；扩展PeopleScopeActualHTTP通过9.84s。

新增continuous1280明确J01/J02/J03/J04/J05/J07各passed。J05成员安全投影role=finance、diagnostic_count=0、退出后tenant_count/tenant_option_count=0，费用126→126等守恒；前轮刷新等待修正在宿主越过原断点，双租户暂停恢复、成员降权/移除/应用Key保留及日志范围旅程达到该脚本完成点。J07在J06前执行以保持同账期分页，真实执行顺序保留；不是漏跑账单或把后续拼成通过。新增1440/390尚未到达，六旅程通过只属于当前1280版本。

| 当前卡点/用户影响 | 根因与最小修正 | 回归及待验 |
|---|---|---|
| J06账单核对driver读错累计费用，阻止完整验收 | 正式失败boundary为journeys.mjs:412:137，read200；旧driver期望charge_entries[0].fee=21。生产追加账本初始未知用量项确认费用0，核对保留原项并追加差额。实际driver片段对该追加账本先红0≠21；seq33只改累计读取，不改生产费用/API/DB。保留fee21并加强cost7/Tokens3+4、pending0/reserve0、原项及价快照不变、新delta ordinal/revision/原因/证据；安全ledger_facts仅金额/条数 | ledger-red.log原13项中1fail，ledger-green-final13/13通过；扩展Governance测试首次误期望revision1失败保留，依admission revision+1改预期后定向race4.392s绿。当前全部165项Node/四顶层in-process race19.433s及format/module/vet/build/browser compile/JS/diff exit0，完整宿主与实际J06仍待seq34 |

候选reconciliation-ledger-r1、124文件、Go1.25.6、HEAD9851177，工作树SHA `50fb764aac5443687e376c18906b36b765f02e9b05788fa4d5ac73617cbc98a4`，attempt-33/source-evidence和正式handoff一致。代码核对helper累计BigInt并逐项检查原账目、价格快照和revision，而不是删金额断言或取第一项放行；新增Go回归分别用既有GovernanceInProcess/ActualHTTP入口，后者新增强部分仍待当前宿主。局部/VM结果不证明浏览器机制或完整账单旅程，旧候选六旅程绿不能直接移为当前完整Pass。

平台Harness失败→development→tests持续正常，当前无新通用平台缺陷被证明，未重复Run/测试、未新User Input、未手改检查点/回执或扩大权限。QA22-01/02与No-Go仍Open，新三视口七旅程/独立QA/PR/合并/部署未完成，冻结design-v0.2.0及商业矩阵保持，供应商/支付/外部身份Not Run。GitHub REST核验Open PR0，近期37711920711/37711213262/37710289218均Issue entry success、旧main9851177，并非正式新产品部署。当前证据`.data/fresh-8793/check-20261008-0917/`，无需用户介入；测试仓事实与平台可靠性分别留证。

## 半小时检查：三视口七旅程全绿，独立QA发现首次读回失败恢复缺陷（2026-10-08 09:47）

seq34第十四次宿主完整make verify真实exit0，正式进入seq35独立QA；QA完成No-Go并正式handoff至seq36 development，结束前复核Run/seq36仍running、无error/wait。未重复Run、测试或User Input，不需要手动resume。正式seq34 output三页读至eof，94335 bytes、truncated=false，124文件、Go1.25.6、HEAD9851177、工作树SHA `50fb764aac5443687e376c18906b36b765f02e9b05788fa4d5ac73617cbc98a4`，与研发交接一致。165项Node、全仓race（CLI9.244s、relay365.518s、browser9.897s、health-history5.311s）、build及原迁移/旧UI/多上游/控制台断言实际通过。

新增continuous-console在1280/1440/390各有J01–J07完成回执，合计21/21，顺序为J01/J02/J03/J04/J05/J07/J06，以保持日志分页在账期推进之前。累计账本driver修正已在真实受控Go/HTTP/Chromium入口越过J06断点，不是只凭局部回归。QA35检查前后124文件指纹不变，绑定18份实际JSON/截图产物并留material-integrity/artifact-index；维护者另查看390最终Key页面截图，能见启用/过期/撤销状态，但单张截图不替代完整视觉验收或故障覆盖。以上为受控上游证据，真实供应商/支付/外部身份仍Not Run。

独立QA报告已迁移当前入口到本Run acceptance/acceptance-report-35.md、acceptance-matrix-35.md、user-journeys-35.md、remediation-handoff-35.md，QA22历史报告保留并指向当前报告。QA22-02 Key状态展示P2已Closed；QA22-01主旅程覆盖P1部分闭合，但原强制故障/并发覆盖仍不足。QA35独立165项Node及7项定向Go race/非监听检查exit0，不以这些绿取代缺失的浏览器/HTTP故障验证。

| 卡点与用户影响 | 根因/可复验诊断/已尝试 | 后续与状态 |
|---|---|---|
| QA35-01 P1：签发或轮换已提交、写响应丢失后，首次安全GET失败会丢失重试入口，页面也不显示操作标识，用户无法从当前页面继续安全确认结果 | 独立QA执行真实production workspace.js的edit函数，catch中的output.textContent清掉先前附加的retry按钮；operation id仅保留在闭包。attempt-35/readback-recovery.test.mjs以DOM替代层复现mutation1/read1/retryfalse/operationvisiblefalse；readback-red.log和原生事件33173保存实际test exit1。外层收集shell exit0不当通过，此证据是生产函数Contract复现，尚非Chromium复现 | QA正式返工已接受；seq36 Agent已确认定位并限定本轮修恢复流程及AC08/09/16回归，当前无修复通过或完整重验结果。保留操作引用/显式只读重试，不自动重放写入或重新生成秘密；之后固定make verify及独立QA |
| AC08/AC16/AC09残余强制故障和并发覆盖 | 已有21旅程不覆盖：轮换响应丢失及首次GET503/abort/401/404后重试、快速双保存/Enter；普通与SSE双发送、延迟上游、取消pending及单次计量；Key独立RPM/TPM/并发/费用的zero/null/positive边界、双Key争用租户额度和新旧代际共同并发、无部分扣减及其他租户隔离 | 纳入原QA35整改handoff而非改写冻结输入；要求真实受控Go/HTTP/Chromium回执，保留旧用量/历史和单operation/admission/dispatch证明。当前仍Open，No-Go有效 |

Harness的完整tests成功→独立QA→No-Go正确返development路径实际成立，本轮没有发现平台跳错节点或新通用平台阻断；产品缺陷仍由Pipeline Agent修改。平台源码保持开发分支WIP、不重启在途8793，不新建Platform PR/合源main。执行中进程中断、当前服务在途重启等尚未实测边界不因本轮绿自动关闭。

GitHub首次读取TLS超时、Actions未随串行命令执行，随后Open PR正式REST只读重查为空；本轮Actions读取先遇网络限制，再通过获准只读网络成功核验37714582530/37714580485/37712863518均Issue entry success，旧main9851177，并非部署。产品5545实际health200、schema2、version98511771871cf0951ecef716bbb55deb13e18da5。本轮新候选未PR/合并/部署，Task5/U1仍Active，不能以21旅程或完整门禁绿宣称整链/商用对齐完成。现场及最新Run/health证据在忽略目录`.data/fresh-8793/check-20261008-0947/`，无必须用户介入的问题。

## 半小时检查：安全读回修复候选与两层资源矩阵交完整复验（2026-10-08 10:17）

seq36 development已completed，原生handoff事件36175正式accepted=true/target=tests，seq37第十五次原完整make verify已派发，10:09:58开始、当前running，Run无error/wait。未手动重复门禁、投递任务或resume。新候选safe-readback-resource-matrix-r1，127文件、Go1.25.6、HEAD98511771871cf0951ecef716bbb55deb13e18da5，工作树SHA `3ee2122eadac33a67342523a082f791d8a81154cc86b7321d53de856866a15ef`，交接前source-evidence/source-after一致，原生事件36083实际source comparison/diff/format exit0。旧50fb指纹的seq34全绿和QA结论保持历史，不作为新版本通过。

维护者只读核对生产workspace.js edit实现：未知结果时分别附加操作引用、状态节点和“安全读回”按钮；GET失败只更改状态节点，不再清父节点；unknown锁住再次form提交并隐藏保存，GET忙去重，epoch/dialog-open隔离迟到结果。产品修复由Pipeline Agent落DOM责任层，未改Go鉴权/计费/迁移或DB规则，未引入新生产依赖。QA35-01仍Open，当前只能称修复候选，不能称真实Chromium恢复或独立QA关单。

| 目标/用户影响 | 实际改动及局部证据 | 真实宿主/后续边界 |
|---|---|---|
| 未知写入首次读回失败无法继续确认 | 原QA生产函数红灯修改前重放exit1保留；新增生产editor Contract覆盖按钮/引用保留、未知提交锁与读回去重，frontend最终169/169通过。原生事件36009实际169 pass、fail0及来源摘要 | 原console新增三视口签发201/轮换200提交后丢响应→503→404→同operation200恢复；要求单mutation、无秘密重放、usage/原请求价快照不变、过渡409不叠加，仍待seq37真实结果 |
| AC09两层各维度及多Key/代际争用缺口 | 新ResourceMatrixInProcess租户/Key各calls/RPM/TPM/concurrency/budget的0→正有限→null共10分例，SharedResourcesInProcess两分例barrier8争2、6拒绝、另租户1次；resource-check.log实际race32.295s/exit0。两次测试前置失败（租户revision错用2、无界fixture仍强求max_tokens100）保留，只纠正新driver/fixture，原fixture默认断言不放宽 | 同名ActualHTTP入口自动纳入原go test，要求拒绝无跨scope部分扣减、历史/金额不重置；本地handler+SQLite+受控Transport不等于实际HTTP通过 |
| AC16普通/SSE快速重复发送覆盖 | 新console-faults.mjs接原console.mjs；受控上游350ms有界延迟/测试专用dispatch事实，native双点击/Enter，断言一个POST/准入/同ID/dispatch及Tokens3+4，原取消/费用未知不重放保留 | 当前仅代码/语法及局部检查，不计真实Chromium双发送通过。真实三视口fault JSON/安全截图必须由宿主实际执行产生 |

新增tests/browser/readback.test.mjs由原frontend-test glob、新console_resource_matrix_test.go由原go test收集，Makefile原完整门禁本轮未改。既有Governance/ResourceDimensions/PeopleScope/PauseUsage/KeyCollection/FreshInstallation六顶层race23.879s实际exit0，format/modules/vet/build等局部检查exit0；不以数量替代AC实证或替代宿主。冻结archive5f51…和19材料逐项完整性匹配，商业对照/AC规则不改。

新浏览器driver会在隔离测试页重建旧editor、要求重试按钮消失，然后解除asset route/reload当前产品再验证修复。此为**修复后隔离重建旧行为**，尚未执行；不是原QA35已经存在的Chromium红灯，也不是当前产品Pass。当前原QA35红灯仍为实际生产函数Contract。原旅程21检查点/J06累计账本/Governance守恒/旧迁移、进程恢复及键盘门槛仍保留。

运行中首次读取seq37正式output返回404 record not found，当前没有已保存的完整回执，不能记为exit0/失败/日志截断或据此判平台错误；仅保存该读取边界，待正式节点完成再按原output分页读至eof。seq37已connector_dispatched=true，不额外起同命令。审计脚本遇个别native item输出null后停止解析，完整2471事件已先写忽略证据；调整只读解析null处理后成功核验命令及handoff，不将审计脚本错误算作产品门禁故障。

QA35 No-Go/QA35-01/QA22-01残余仍未关闭，QA22-02旧关闭事实保留；Task5/U1 Active。Harness的正式development→tests接受/派发正常，无新通用平台根因得到证明；在途服务/源分支WIP保持，无新Platform PR/源main合并。近期GitHub37716527895/37714582530/37714580485仅Issue入口success、旧main9851177；5545实际health200/schema2/版本9851177，当前候选未部署。真实供应商/支付/外部身份及尚未实测平台负向继续分层Not Run。现场在忽略目录`.data/fresh-8793/check-20261008-1017/`，无需用户介入，继续等待原完整门禁并由独立QA核验关单。

本轮结束前10:20复核seq37仍running，无error。GitHub PR列表第一次及一次有界只读重查均TLS handshake timeout；本轮Open PR数量未知，不能沿用09:47的0作为现状。Actions和产品health独立读取成功如上，暂不构成产品或Harness阻断，不循环重试。

## 半小时检查：安全恢复及资源竞争独立QA关闭，十页设计对照待补证（2026-10-08 10:47）

seq37第十五次原完整make verify真实exit0，正式进入seq38独立QA；seq38已completed并No-Go正式返seq39 development，当前running，无error/wait。原生handoff事件38144 accepted=true/target=development，未重复创建任务/投递输入/手动resume。当前独立QA关闭与新补证责任以acceptance-report-38.md、acceptance-matrix-38.md、user-journeys-38.md、visual-review-38.md、remediation-handoff-38.md为准，旧QA35保留历史。

正式seq37 output四页读至eof，101748 bytes、truncated=false，127文件、Go1.25.6、HEAD9851177、工作树SHA `3ee2122eadac33a67342523a082f791d8a81154cc86b7321d53de856866a15ef`，与研发交接及QA前后指纹相同。169Node、全仓race（CLI8.834s、relay412.191s、browser9.688s、health-history5.700s）、build和原迁移/进程/HTTP/SSE/旧UI/键盘等实际通过。原健康隔离重建红灯是预期历史证据，不是本轮门禁失败。

新增ResourceMatrixActualHTTP在tenant/Key五维共10分例实际通过24.90s；SharedResourcesActualHTTP两个分例各8争2/6拒绝/另租户1次、real_http=true，通过5.11s。三视口console-faults均passed：签发201/轮换200已提交丢响应→GET503/404/200，mutation1、不重放secret、叠加轮换409；普通/SSE双发送均proxyPOST/admission/attempt/actual dispatch各1、Tokens3+4。原continuous三视口七旅程21/21保持绿。每视口另有清楚标记的旧editor隔离重建expected_failure，不能追溯为原QA35的Chromium红灯，也不计当前修复资产失败。

独立QA38实际169Node/0skip（原生36825）、七顶层Go race48.879s/exit0（37021）及带set-e的format/modules/vet/build通过；source unchanged事件37769，材料archive/19文件一致。归档63件宿主真实JSON/PNG，本轮维护者再次按artifact-index对63件原文件及副本逐一核对bytes/SHA，全部一致。另人工查看390轮换首次503截图，操作标识、未知状态和“安全读回”按钮仍可触达；单图不替代十页完整设计对照。

| 项目 | 当前结论/用户影响 | 后续责任 |
|---|---|---|
| QA35-01读回恢复P1 | **Closed**，真实当前asset签发/已用Key轮换三视口故障链确认按钮/引用保留、安全恢复、只一次写，历史/usage/价快照不变 | 保留原Contract红及当前真实复验，后续新指纹不跨用旧绿 |
| QA22-01原列明的主链/AC08/09/16覆盖主题P1 | **Closed（该主题指定范围）**，21旅程/两层五维/共享资源/双发送实测补齐 | 不外推所有可能故障组合或整套AC/H全部完成 |
| QA22-02状态展示P2 | **Closed（保持）** | 原启停/过期/撤销/过渡断言继续保留 |
| QA38-01强制AC01完整十页批准设计实际对照 | **Open / Not Run**，是证据门槛缺口，非已证明的新产品P1。当前十页循环只检查h1/根宽度，现存图主要Key/账单，不能证明工作台/模型/服务/成员/预算/分析/日志/调用等主要信息与动作符合批准设计 | 已正式交seq39，在既有真实Go+Chromium入口补逐页page/role/scope、字段/关键操作及适用空态/非空态截图和原型映射；优先复用已有旅程，不重写已通过功能、不要求像素复刻、不变冻结输入。测试源变化后新指纹原完整门禁/独立QA |

seq39已在原会话明确只补十页真实页面采集与映射，保留原门禁；当前没有需求/设计歧义或新增权限请求，不需要用户再批准。QA本次No-Go来自冻结AC01/verification-design原强制门槛（QA35矩阵已Not Run），不是新发现运行故障或给装饰建议加门禁。未进入report/PR/合并/部署。产品供应商/外部身份/支付仍Not Run，不把受控上游语义绿称商用全面对齐。

Harness的tests成功→QA独立关旧缺陷→正确No-Go补证返development路径正常，本轮无跳错节点或新通用平台阻断。平台侧H02/H03/H04/H07/H08证据由维护者负责，QA无法自行核对不等于全部功能不支持；已有入站去重等证据留在典型场景记录，实际模型/在途重启/完整发布与安装升级等缺口仍按原矩阵分层，不让产品Agent造平台回执。源平台开发分支WIP/在途8793保持，不新Platform PR或合源main。

GitHub本轮正式REST核验Open PR0，Actions首次及一次有界重查均TLS handshake timeout，近期Actions现状未知，不沿用旧入口成功当当前部署证明；未循环盲试。5545实际health200、schema2/version98511771871cf0951ecef716bbb55deb13e18da5，仍为旧部署。Task5/U1 Active，整链新版本发布未完成。当前完整日志/QA原生事件/Run/health证据在忽略目录`.data/fresh-8793/check-20261008-1047/`，无需用户介入。

## 半小时检查：十页实际页面采集交完整门禁，可读性候选待验（2026-10-08 11:17）

seq39 development已completed，原生handoff事件41244正式accepted=true/target=tests，当前seq40第十六次原完整make verify running（11:11:19开始），Run无error/wait。原QA38 No-Go/QA38-01仍Open；QA35-01/QA22-01指定范围/QA22-02旧关闭事实保留，但不把旧3ee2122…绿色跨用到新候选。当前ten-page-evidence-r1，129文件、Go1.25.6、HEAD98511771871cf0951ecef716bbb55deb13e18da5，工作树SHA `bb30f8179c08e5e9dad248382132ec1018959c934efcd5951b1cab55e5b486da`，source-evidence/source-after及正式交接一致，原生41152记录实际摘要、材料19件匹配和非监听检查exit0。

| 目标/影响 | 实际处理及局部证据 | 本轮未验/下步 |
|---|---|---|
| QA38-01十页批准设计实际对照缺证据 | 新console-visual.mjs接原连续J01–J07之后，复用已由真实UI创建的双租户/双模型/多Key/核对请求；逐页角色/scope、多对象列、摘要、筛选、主要操作、5详情/6编辑，界面落定及秘密清除后采集。page-mapping.md逐页列批准信息→正式入口→实际断言和布局差异，不自行判合理或关单 | 预期每视口21PNG、三视口63PNG/3JSON尚未取得正式宿主生成结果；这些是采集计划，不能计实际截图或AC01通过。待seq40原完整门禁及独立QA真实逐页对照 |
| Key配额原JSON不便日常管理，详情存在内部英文key | 产品Agent复用现有limitSummary呈现Key五维额度；实际attempts和四费率字段中文化，未改值/零/无限/未知费用口径或后端权限/预算/账本。既有workspace源函数回归和新采集Contract纳入原frontend-test | 173项Node实际exit0（原生40093/40875），语法/format/module/vet/build/diff各exit0（41152）；VM及源码/构建不替代真实Chromium可读性/全部AC符合 |

完整逐页映射仍公开待判差异：工作台以真实条形趋势呈现、模型部分来源/价格/延迟位于关联服务或详情/日志，服务模型数/优先级未聚合到列表，租户关联计数不重复、分析分布分置工作台，调用测试只收无秘密合理空表单。不能因新增自动断言绿就把缺字段解释成装饰；独立QA按原冻结AC01判主要信息/动作是否满足，必要缺口再最小返工。未改变design-v0.2.0、未执行包内脚本、不以API预制完成态/改DB或重放业务副作用来补画面。

原Makefile及既有console烟测/keyboard/fault/实际HTTP/Go/迁移/J01–J07/J06守恒门槛保留。QA38的63件历史产物不冒作本轮新图；新代码/测试指纹变化后完整宿主和独立QA仍须重新核对。没有重复原生已知监听失败，也没有重复Run/测试/输入、手动resume或临时扩大权限。当前完整门禁暂无正式结果，U1/Task5 Active，产品实现与测试资产完成不等于已验收/发布。

Harness正式development→tests接受/派发正常，本轮无跳错或新通用平台阻断，源开发分支WIP/在途8793保持、不新Platform PR或合源main。GitHub本轮Open PR正式REST为0，Actions初次TLS超时后仅作一次有界只读重查，不把读取失败判部署成功；5545实际health200/schema2/旧main9851177，新候选尚未部署。供应商/支付/外部身份及平台H未验边界仍分层保留，不宣称全面商用对齐。现场`.data/fresh-8793/check-20261008-1117/`，无必须用户介入问题，沿原门禁/QA推进。

本轮Actions有界重查实际成功：37721532229/37719369766/37719365917均Issue entry success、旧main9851177，非正式新部署。该读取恢复不涉及重试业务写入或扩大权限。


## 半小时检查：采集首断点与运行次数耗尽的正式恢复（2026-10-08 11:47～12:12）

**测试仓进度。** seq40第十六次原完整make verify真实exit2；正式output四页读至eof，共101576 bytes、truncated=false，129文件、Go1.25.6、HEAD9851177、候选SHA `bb30f8179c08e5e9dad248382132ec1018959c934efcd5951b1cab55e5b486da`。173Node、全仓race（CLI9.731s、relay413.975s、browser10.738s、health-history5.779s）、build，原迁移/旧UI/多上游/console/fault三视口均实际通过；新continuous1280 J01–J07均passed，随后新增visual-overview首断言失败。`test-results/visual-1280/review.json` pages=[]/phase=overview/result=failed，新十页截图与新1440/390采集未取得，QA38-01继续Open、QA38 No-Go有效。QA35-01/QA22-01指定主题/QA22-02原关闭事实保留，不能跨用旧指纹绿。

采集失败具体字段仍未知：failure_boundary.available=false，新模块断言未被旧诊断行过滤捕获；last_status404来自先前J06跨租户账单预期读取，不是overview HTTP404证据。源码可复验假设是采集ready一律拒绝.empty，而J06推进测试钟至新月，工作台近7日可能合理无调用；尚未以真实DOM事实证明，不按猜测降门槛。正式返工输入要求先保存断言前安全字段/角色/scope/尺寸/适用空态和准确异常来源，禁止秘密、虚构截图或改冻结材料，产品及driver最小修复仍交Pipeline Agent。

**Harness卡点与修复。** seq40结果已接受route=failed/exit2，原上限保护将Run置failed，error为`maximum node executions reached; inspect loop before starting another run`，没有派发seq41。冻结图max_steps40；原resume禁止重放已结束命令，原return拒绝超限，缺少审查后有界继续入口。用户影响是有明确新返工动作也无法保留原历史继续，并非handoff跳错或测试命令未执行。

维护源实现授权显式return的可选max_steps：只有耗尽原预算时可追加更大有限总上限（最多1000），要求当前seq、非空检查原因、合法目标、静止执行及有效授权；与新节点同事务记录调用者/40→60/原因。预算存于平台启动幂等创建的run limits表，与冻结图分离，不自动追加/不改旧结果/不自动重放命令。当前Run有效预算通过GET显示；达到新上限仍停止。运行页直接显示检查原因、总上限和目标表单，默认选已接受结果的合法后继；上限耗尽无需再停止。标准研发模板新装默认100，已有Run不随模板升级涨预算。

| 验证与应用 | 本轮实际证据 | 限制 |
|---|---|---|
| API/恢复行为红绿 | 新workflow_limit_test先HTTP400 unknown max_steps红灯；修复后非法0/等值/1001、非法目标不改变历史，未授权403、过期seq409；原图/历史/workspace不变，真实Store关闭重开后有效预算保留，新增预算耗尽再停止 | Go隔离回归不单独当真实Pipeline恢复 |
| 页面行为与实际页面 | workflow-runs Node行为先缺表单红、修复后通过；实际IAB刷新当前Run，40次停止提示、总上限60、默认研发实现、原因与继续按钮可见。原有已打开页刷新前仍显示旧资源，刷新后新入口可核对 | 此次提交用正式API，而非浏览器表单点击；不把VM当窄屏/完整体验验收 |
| 全维护源检查 | `bash scripts/verify.sh`完整exit0，平台全race50.460s、模板/安装/工作流72Python测试6.278s及所有原入口通过；最后仅调整停止提示中文，Node再次exit0、重新go build恢复binary exit0。初次额外build被沙箱Go cache访问阻止，获准正常构建后成功，无代码失败 | 非致命macOS链接warning保存；完整检查不证明所有实际异常/商用目标完成 |
| 8793标准二进制升级 | 通过正式API确认active_runs=[]/active_native=[]；核对PID36295仅8793，SIGTERM退出后备份platform.db/原binary/manifest/配置，再替换并同端口/同数据/原凭据启动PID27655。旧binarySHA b7b2eeec…，新d926462e50e118de886a990cf60d6c1b6e29e21c4226df63f44afcb2f765f905；逐项比对40步、冻结图、workspace、参数和Connector快照一致 | 本轮是无在途升级；不能外推为在途服务重启验证。没有手改数据库或旧检查点，启动由维护源正常创建存储 |
| 标准安装升级 | 原install.json加--upgrade安装软件交付模板，同workflow/Connector/browser IDs；连续两次exit0，第二次无变化、manifest完全一致，新模板上限100。原Run冻结图/有效上限40及40步仍完全一致 | 新安装通过标准模板/安装回归；当前实际环境验证是已有安装升级，不另造重复Run |
| 正式恢复 | 原model-relay-ci Token通过POST return seq40/target=development/max_steps60/检查原因，实际HTTP202进入seq41；原40步深比较一致、冻结图40、有效预算60、原workspace一致。新会话842bbd6702fd65a920ecac1a8630c848 status=running，Agent实际回复“我先恢复 seq40 完整失败证据和规范，再定位采集断言” | 当前只是恢复执行，不是产品采集修复完成/独立QA/发布；20次额外预算耗尽仍须审查，不能盲目反复追加 |

本轮维护源唯一源码、手册、模板、升级路径与实际Run恢复已对应；未改测试仓产品、手工回执、DB或检查点，未增加权限或切换模型。现有8792/8788不动。源开发分支包含此前completion Schema/统一配置WIP及本次修复，完整检查覆盖当前代码；不新Platform PR或合main。运行新节点沿冻结图原策略执行，与新模板配置分层。

12:11 GitHub正式Actions只读返回37726214624/37721532229/37719369766均Issue entry success、旧main9851177，非新产品部署。Open PR本轮读取TLS超时，当前数量未知，不沿用11:17的0。5545实际health200/schema2/version98511771871cf0951ecef716bbb55deb13e18da5，新候选未合并部署；8793未提供/healthz（404），其就绪与升级以真实登录、Run API、页面和原生执行证明，不造health版本。U1/Task5仍Active，整链发布与平台其余未测负向继续保持未验。完整现场/红绿/检查/备份/标准升级/正式恢复收据在忽略目录`.data/fresh-8793/check-20261008-1147/`。


## 半小时检查：诊断回执候选已实现，原门槛保持（2026-10-08 12:18）

正式API核对同Run seq41 development仍running，原生会话842bbd6702fd65a920ecac1a8630c848 running，无error/等待/已接受结果；有效上限60、冻结40保持，没有重复投递输入/新任务/原命令。恢复后的Agent已实际读取seq40完整101576字节并归档原continuous/visual失败，确认404属于J06且当前采集具体失败断言不可归因。无需新增用户输入。

**测试仓。** 新visual-boundary-r1诊断候选只修`console-visual.mjs`失败留证责任：导航前pending_capture、断言前安全DOM事实/实际尺寸/空态分类/角色scope/字段动作布尔值；监听精确overview GET，只留资源白名单、status和scope匹配（最多16条），不存URL/query/body/凭据；保存准确check/固定断言消息，导航失败facts=null，退出移除监听。原ready拒绝.empty及所有角色/范围/字段/操作/宽度/秘密/行数门槛均保持，生产workspace.js/Go/API/账本/测试钟本轮未改，Makefile原门禁保持。当前空态仍是合理假设，不能把Contract人工facts当seq40真实DOM或宣布采集修复完成。

维护者只读核对源码、attempt-41/checks.md和host-retest.md及原生实际事件：41721 frontend175/175 exit0，41747最终176/176 exit0（原173保留，新增3项实际失败留证/导航失败脱敏/HTTP白名单Contract），41731/41767 modules/vet/build等exit0。129文件新指纹 `fb766c0aca5f33fbb0960e13286a13461d20999485e93a19a841eb51114f92a4`、HEAD9851177、Go1.25.6；19材料/原archive核验一致。局部检查通过只是诊断候选，尚无该指纹原完整宿主门禁、新十页PNG/独立QA、PR合并发布；QA38-01仍Open/No-Go，旧关闭事实按旧版本范围保留。关键路径为真实宿主获得准确边界→有证据最小修复（必要时）→原完整门禁→独立逐页QA，产品Agent沿原handoff到tests，不放宽断言或改冻结输入。

**Harness。** 已推送开发分支提交abe0dd6f9c6ad91254b00eed515eb1c440330d1f，源码工作树干净；8793当前binarySHA d926462e…，真实恢复seq41正常，未发现新路由/权限/等待故障。前轮完整平台检查、无在途升级、两次标准安装幂等及原40步深比较证据保持，不重复运行这些已通过检查。本次诊断不是新平台通用缺陷；尚未实测的在途重启、实际模型或完整发布边界继续未验，不因恢复生效宣称所有流程无问题。首次源码push明确网络超时，远端只读仍9931ff6后经已有7897代理一次有界重试成功，远端tracking与本地abe0dd6一致，无Platform PR/main合并。

GitHub本轮正式Open PR读取为0；Actions首次TLS timeout，近期状态暂未知，不用旧Issue entry成功证明本轮部署，未循环重试。源平台和产品资产/回执保持原范围，无需用户介入；现场及原生事件保存在忽略目录`.data/fresh-8793/check-20261008-1218/`。Task5/U1 Active。


本轮结束核验：seq41已completed，原生handoff事件41905正式accepted=true/target=tests，新seq42第十七次原完整make verify running且connector_dispatched=true，无error/当前结果；未手动追加测试、Run或输入。交接携当前fb766c0a…诊断指纹、真实旧失败、host-retest及QA38原返工依据，明确保留门槛，不能预期诊断回合一定绿。5545本轮health200/schema2/version9851177仍旧部署，Open PR0。后续等待seq42正式完整结果，再按准确安全事实归因；不把正常派发当已修复或通过。


## 半小时检查：真实空态断点已证，当前周期活动候选交完整复验（2026-10-08 12:48）

Run实时seq44 tests running/connector_dispatched=true、有效上限60、无error；seq42第十七次原完整make verify exit2已正常沿failed回seq43，seq43完成visual-current-period-r1并通过原生handoff事件43126 accepted=true/target=tests进入seq44第十八次原完整门禁（12:42:35开始）。未重放命令、另建Run、重复投递、增加权限或模型切换；新节点仍沿冻结图的原策略执行。

**测试仓证据与根因。** seq42原日志由维护者四页0→32768→65536→98304→103232读至eof，103232 bytes/truncated=false；129文件、Go1.25.6、HEAD9851177、诊断指纹fb766c0a…；176Node、全仓race（CLI9.491s、relay410.099s、browser10.722s、health-history5.613s）、build，原迁移/旧UI/多上游/控制台/故障三视口通过，continuous1280 J01–J07全部检查点passed后仍visual-overview失败。诊断回合故意保持原门槛，红灯不是该回合没有获得有效结果。

新实际overview GET200且scope_matches=true；title/scope/平台管理员role、7正文/3主要动作及enabled、rootFits/secretsClear全部成立；loading=false/error_nodes=0/notice=false。真实总/可见empty各1、seven_day_no_calls=1/no_data=0/other=0；viewport/root1280、content990。review.failure固定ready断言`no loading/failure/empty masquerading as data`，pages=[]。由此原空态假设得到真实宿主证明：J06为结账推进测试钟到下个月，生产overview以当前UTC日/近7日聚合，当前周期没有请求，合法空态与非空视觉采集要求冲突。责任是journeys的采集前活动准备，不是业务返回404、供应商故障或已证明的生产页面错误；last_status404仍是原J06跨租户账单预期读。诊断回执、正式日志与源码相互一致，未把Contract人工facts当实际DOM。

| 处理与证据 | 实际状态 | 未完成边界 |
|---|---|---|
| 当前周期真实活动准备 | 产品Agent只改journeys.mjs/test及既有Trellis契约，新增prepareVisualActivity置J06全部原断言/跨租户404之后。先读当天0调用/空趋势，再经原生调用测试UI与现有轮换Key发起真实受控调用，要求POST成功、dispatch增1；同Request ID唯一charge fee21/cost7/pending0/current period；overview当天1调用/100%成功率/当前费21/当日趋势1；已结束账单原完整对象深等，导航清秘密，再执行原十页采集 | 代码及绑定只读核验完成，当前候选尚待宿主完成；未使用API seed/SQL/倒退钟/改DOM/演示金额。新增活动不抹除J06旧历史或价格快照守恒 |
| 原门槛 | console-visual原ready/所有字段/行/角色scope/尺寸/秘密门槛保持；生产Go/UI、Makefile、DB和业务计费本轮未改，沿原入口 | 不以放宽全部.empty消除红，不把合法空态本身判产品P1 |
| 局部真实执行 | 原生42438定向16项exit0；42861 frontend179/179 exit0（原176+3项准备函数/拒绝旧周期错误费用/拒绝旧账单变化Contract），语法/format/modules/vet/build/diff检查exit0。首次包装用了zsh只读status导致shell1、测试16/16绿，纠正包装后真实命令exit0，原失败保留 | VM Contract不是实际Chromium/HTTP准备或供应商证明，不以179Node关闭QA38-01 |
| 版本/交接 | 新129文件SHA `31bbf962868170e32d089a83217b8f057afce3a359fa9f0d5b25c1f4089d6bc5`，source-evidence/source-after/正式handoff一致，原archive5f51…及19件材料核验一致。seq44原完整门禁已派发一次，当前running | 新三视口十页PNG/完整结果/独立QA仍待验，不能跨用seq37旧绿或seq42局部分项绿作新版本通过 |

QA38-01继续Open/QA38 No-Go，QA35-01/QA22-01指定主题/QA22-02旧关闭事实按旧版本保留。Task5/U1 Active；失败后按新精确check最小返工，完整通过才交独立QA对冻结A原型/AC01逐页判断。材料、需求与架构未改变，没有新的用户澄清/权限请求。真实供应商、支付、外部身份、平台尚未验证边界继续Not Run，未宣称商用全面对齐。

**Harness进度。** 显式有界恢复之后，seq41→42诊断tests→43正确failed返研发→44候选tests均实际正常，未发现新的跳错/重复派发/通用平台故障。维护源20e1a97干净、8793代码binary仍d926462e…；本轮只有证据记录更新，不重启在途服务或重复已通过平台全回归。seq43原生thread-start事件有Codex thread/CLI/provider元数据，但没有实际model字段；冻结配置指定gpt-6.1-sol，不能据此新增“底层模型已独立验证”的结论，原H边界保持。

GitHub本轮Actions37728783413/37727970399/37727067377都是Issue entry success/旧main9851177，非新发布。Open PR首次读取TLS timeout，当前数量未知，不沿用12:18的0；不循环盲试。5545实际health200/schema2/version98511771871cf0951ecef716bbb55deb13e18da5，新候选未合并部署。完整原日志/当前Run/原生1361事件与health在忽略目录`.data/fresh-8793/check-20261008-1248/`；产品证据在同Run attempt43原文件，维护者没有修改产品或回执。


## 半小时检查：十页三视口真实采集与完整门禁通过，独立设计QA在途（2026-10-08 13:18）

Run实时seq47 qa running，无error/等待，有效预算60/冻结图40；seq44完整测试exit2正确返seq45，seq45 visual-editor-owner-r1已正式交tests，seq46第十九次原完整make verify真实exit0，13:13:35进入独立QA47。没有重复Run、输入、命令、手动恢复、服务重启或权限扩大；产品及采集驱动由Pipeline Agent修正。

**两个采集断点实际验证。** 维护者分别将seq44和seq46正式output四页读至eof：103687/104299 bytes、均truncated=false。seq44旧候选31bbf962…的1280当前周期UI真实活动requests0→1/trend0→1/fee21/closed_bill_unchanged=true已通过，越过overview原空态断点，采集overview/models/services/keys/tenants/limits六页后停limits-editor/open-editor timeout。实际limits role/scope/ready/字段/操作/非空/布局/秘密均通过、调整租户限额enabled、editor_open=false；源码按钮由mainAction置.page-heading/#page-action，采集却在#content中寻找。责任是locator所属容器错误，不是新预算业务失败、监听限制或未知异步等待。

seq45只修console-visual.mjs/test及既有Trellis契约：limits editor按声明heading owner找exact唯一按钮；其他5editor保留current row/content，不跨容器fallback、不提高超时。原采集门槛/生产Go/UI/账本/测试钟/Makefile均保持。旧owner隔离重建的EXPECTED RED是Contract层预期红评估exit0，不称修改前Chromium红灯；实际旧宿主红来自seq44。局部8/8及182Node/静态检查exit0，129文件新指纹 `7e0b20ddbdea4585959365d8ad3b6121abaa91cd3f450fa2c6c21f7afc05a9ab`、HEAD9851177、Go1.25.6，source-evidence/source-after/正式seq45交接一致，19件材料/原archive5f51…保持。

**当前同版完整测试已通过。** seq46源戳为上述7e0b20dd…，182Node/0fail，全仓race（CLI8.975s、relay405.581s、browser10.801s、health-history5.925s）、build，原迁移schema1/2→3/进程/实际HTTP/旧UI/多上游/键盘/三视口console及fault全部通过。三视口故障链仍单mutation、无秘密重放、轮换过渡409；双普通/SSE仍proxyPOST/admission/attempt/dispatch各1、Tokens3+4。新continuous三视口J01–J07共21检查点均passed；当前周期UI活动三视口均真实passed且旧结账对象保持。新console-visual三个视口各result=passed/pages10，越过limits editor并完成所有后续采集。受控真实HTTP/SSE只证明本项目语义，不是供应商联调。

维护者独立从三个新review.json递归核验其引用PNG：每视口10页+5详情+6编辑=21张，共63个唯一实际路径，全部存在且SHA256与review逐项匹配，结果保存在visual-artifact-verification.json。没有用文件夹总数量或旧同名图冒当前证据。人工查看当前390工作台和1280租户限额editor，当前月1调用/100%/费用0.000021、趋势及预算编辑实际呈现；仅查看两图不替代全部冻结设计对照或独立QA。旧QA38的63件fault/连续证据与本轮63件visual不同，不混为同一批。

**独立QA尚未结束。** seq47已读取同材料/当前源戳/正式门禁与三视口截图，正在独立对照；原生消息指出模型和租户摘要可能缺冻结设计关键信息，当前只是QA在途发现，未有已接受结果或正式整改单，不预判其Go/No-Go/缺陷等级，也不由维护者代改产品。QA38-01继续Open直到独立QA正式关闭；自动化采集通过与63图齐全不足以保证AC01全部信息/主要动作符合。若明确缺口，沿现有正式handoff返工，不静默改设计或新增用户审批。QA35-01/QA22-01指定主题/QA22-02旧关闭事实按范围保持。

**Harness。** 有界恢复后原failed→研发→tests→独立QA路由及同Run预算实际正常，无跳错/重复派发或新的通用缺陷。源开发分支a96aa9d干净、8793 binary仍d926462e…；本轮只更新现有证据，不重跑已通过平台完整检查、不重启在途服务。底层实际模型/在途重启/完整新发布等原未验边界保持，不以产品门禁绿外推所有Harness异常场景通过。

GitHub本轮正式Open PR0；Actions读取TLS timeout，当前近期Actions未知，不沿用旧入口成功作发布证明。5545实际health200/schema2/version98511771871cf0951ecef716bbb55deb13e18da5，仍旧部署，新产品未PR/合并/正式Actions部署。Task5/U1 Active；当前关键路径为独立逐页QA→必要返工或报告发布→测试仓授权合并/正式部署/health和页面一致。真实供应商/支付/外部身份及冻结外商业功能仍未验证/后续缺口。现场、两份完整原日志、当前review、63文件摘要、QA原生事件及health在忽略目录`.data/fresh-8793/check-20261008-1318/`，没有必须用户介入事项。


## 最新效果部署请求：正式Pipeline已存在，独立QA返工继续（2026-10-08 13:48～14:21）

用户明确要求通过Model Relay部署Pipeline查看最新效果，并要求协调者不要替Harness做实现。本次核对现有正式Actions `Deploy Model Relay locally`（workflow ID375348269，`.github/workflows/deploy-local.yml`）active；main push只准备，Owner workflow_dispatch设置deploy=true才激活，preview固定5545，validation隔离5546。Runner model-relay-local online/busy=false，标签self-hosted/macOS/ARM64/he-full；四项部署变量已有配置。无需重复创建部署Workflow、修改产品源码或启动演示服务。已有模板/控制器schema3支持及标准安装证据保持；真实本轮产品schema3联合/Actions升级与最终页面仍待验，不能用旧控制器夹具绿替代。

**测试仓正式结论。** QA47对seq46同7e0b20dd…指纹、原完整104299bytes/exit0及75件宿主产物（含63PNG）独立审核，正式No-Go并于13:31:27沿handoff返seq48 development。QA38-01 Closed仅表示十页三视口采集缺口闭合；新的QA47-01 P1/AC01 Fail是实际冻结PRD§7/interactions要求的主要管理信息缺失：模型来源/当前价/延迟与上下文操作、租户负责人/授权模型/Key/预算摘要、服务模型关联/核验时间、工作台模型状态。既有采集断言只按当前实现字段选取，没有覆盖这些冻结结果；不是新增高级商业范围或像素风格争议。QA47-02 P2 Open是同Request ID分析确认3+4、日志未标明口径的Token总量未知，需区分不可变原始上游事实与核对计费用量；不判账本错误、不单独阻断。QA35-01/QA22-01指定范围/QA22-02旧关闭事实保留。

产品Agent seq48已形成management-summaries-r1：只读scope内聚合+正式UI摘要/上下文操作，原始与确认用量分列/详情/CSV；没有新表、迁移或账本写入修改。133文件新指纹2b1297e865a8b595af98cd7594900d4a6c0506729c0e46de5ba3a05640a2a962，19件冻结材料一致，184Node及定向Go/静态局部检查通过。新真实ActualHTTP/三视口management-summary/J06对账接原固定入口；源码存在与局部通过不能关QA47。

**新完整回归仍失败。** seq49原make verify/900s exit2，维护者正式output四页0→32768→65536→98304→98848/eof，truncated=false。实际全仓race（relay444.647s/browser11.153s/health-history5.976s）、build、原schema迁移/旧UI/多上游及1280 console-faults/console-real-go-ui通过；后续console.mjs:97 service-create等待upstreams POST响应30s超时，尚未完成全部新摘要/连续/visual三视口。不复用seq46绿放行新版本。原failed边正确返seq50。

seq50恢复原失败后定位编辑器Save锁生命周期：旧提交已确认后render/sync被导航epoch替换，旧finally不能解锁，新编辑器又未重置disabled。实际生产函数VM红绿复现新弹层遗留disabled及旧finally不能解锁后一个提交；候选只修锁所有权/原测试等待，187Node和静态检查局部通过；133文件最终指纹5b0822a01c8d0a92236fdf1a70d5e4e1fe6114782ddb643b86e002c0dee3d836与source-evidence/正式handoff一致，14:31实际seq51 tests running/dispatched，尚无完成回执。新增真实宿主三视口有界回归拟hold同步GET，在真实201后导航、开下一编辑器断言Save enabled/写POST仍1；当前尚无新完整宿主通过，不能把VM当原超时浏览器追溯证据或保证唯一根因。产品源码/测试由Pipeline Agent负责，协调者未代改。

**用户部署要求已进入Harness。** 当前Run正式POST messages经CI Owner收到HTTP202，稳定request_id=model-relay-deploy-latest-20261008-user、message9179、原seq50 conversation25e95df704ec0858380f2c3ca6c56ce1、duplicate=false。要求携真实版本/门禁/QA、部署步骤、页面与首次登录指引，经原report→publish→pr交接；协调者只核验、按已有授权合并并触发正式部署。接收不等于Agent处理完毕；不重建Run或重复投递，也不重新部署旧main冒充最新。未改冻结输入或质量门禁。

**Harness状态与边界。** 原QA No-Go→研发→tests failed→研发路由及有限预算60/冻结40正常，本轮没有跳错、重复派发或新的通用平台缺陷。当前8793 binary仍d926462e…，无需重启或补丁；源760f5be之后仅更新交付记录。Harness完整未验负向、实际原生模型H03-runtime、在途升级等继续分层未验；新部署请求不能成为绕过门禁/临时权限的理由。

GitHub只读Open PR初次TLS超时，已有7897代理一次有界重查为空；Actions14:31核对37737849943/37737604474/37736226292均旧main Issue entry success，非部署。5545实际health200/schema2/version98511771871cf0951ecef716bbb55deb13e18da5，仍旧版本。Task5/U1 Active；当前关键路径为seq50→原完整tests→独立QA→正式report/publish/pr→授权合并→Actions隔离/正式preview→health与实际页面一致。真实供应商/支付/外部身份仍未验，无需新增用户输入。原QA与attempt48/50证据保留，维护者Run/完整seq49日志、部署Input回执在忽略目录`.data/fresh-8793/check-deploy-request/`；13:48原核对在check-20261008-1348，14:31无新阻断或发布的实时核对在check-20261008-1431。


部署请求结束核验：message9179由queued→running→completed，原生Agent实际回复已写入原任务/带入下游，seq50正式handoff被接受后进入seq51 tests（connector_dispatched=true/running、无error）。维护者未代替Harness测试、产出PR或启动服务；当前最新候选仍需本轮完整门禁与独立QA，未有可授权激活的新main。旧5545不能作为此次最新效果链接；正式部署Workflow已有，不重复创建。


## 用户停止定时任务与整体状态核对（2026-10-08 14:51）

用户明确要求停止定时任务并汇报。通过automation_update正式将model-relay heartbeat置PAUSED，返回status=PAUSED；只读保存automation.toml复核一致，原prompt/30分钟频率/当前thread保持。停止的是协调者定时检查，不取消仍在执行的产品Run，也不关闭8793或产品服务。后续不会定时自动推进；当前请求仅核对、保留现场和汇报。

**测试仓最新事实。** 正式API同Run running/seq52 development，会话f3567dfcd643092a5762fdd0b2d5ab5a，预算60/冻结40、无error/等待。seq51第21次原make verify exit2、900s入口，维护者三页0→32768→65536→98283/eof、truncated=false完整恢复日志；133文件指纹5b0822a01c8d0a92236fdf1a70d5e4e1fe6114782ddb643b86e002c0dee3d836与正式seq50交接一致。187Node/0fail/0skip、全仓race（CLI9.877s、relay440.840s、browser10.876s、health-history5.635s）、build、两种生产schema3迁移三视口、旧UI/多上游继续通过。不能因为这些分项通过将整套exit2改成Pass。

首失败为新增console-real-go-ui的editor-held-readback-1280：实际write_status201、catalog200/members200、page_error0、request_failed1，editor_open=false/save_disabled=false，安全snapshot存在；尚未完成其余新故障/管理摘要/J01–J07/visual门禁。seq52原生正在处理；锁版本Playwright URL matcher原精确/admin/api/tenants只能匹配无query URL，实际分页?page=1&page_size=100&limit=100及cursor均不匹配，修正后这两类匹配且members/tenants-extra继续拒绝。该诊断明确标native_network_execution=false，不当实际宿主新回归通过，也不据此反推旧201服务写入失败或证明唯一产品根因。产品Agent修测试匹配，协调者只读核对，不替代实现或重复运行命令。

上一版seq46完整exit0/63PNG/21主旅程仍是旧7e0b20dd…指纹证据；QA47正式No-Go的主要管理摘要P1与Token口径P2保持Open，seq48/50已有候选实现不替代新完整通过和独立QA关单。当前核心多租户身份/成员、模型服务/路由、Key/共享预算、计费用量/核对结账、十页控制台与迁移已有实现和多层实测；产品未正式验收，不能报商用对齐或完成百分比。

**Harness分层结论。** 已有8793典型实测覆盖自主handoff、complete_node不合Schema拒绝/纠正后接受、真实布尔/整数/数组/嵌套对象固定消费、命令失败返工、QA返工、澄清/等待态停止与原thread继续、完成后人工回退、Issue去重和跨所有者拒绝。当前主Run的新失败也正确返回development，无已观察跳错/重复派发。统一配置/两出口、停止与取消语义、覆盖式节点抽屉、有界继续和授权恢复都已在维护源交付；40次预算耗尽后的正式显式恢复60、历史/工作区保持、无在途标准升级和幂等安装有真实证据。不能把这些证据外推为当前版本所有权限/在途进程强停重启/外部未知响应/最终部署异常已全验；native实际model字段证据缺口仍保留。平台页面/跳转体验后续优化尚未全面完成。

外部只读核对Open PR0，37738944460/37737849943/37737604474/37736226292均旧main的Issue entry success，并非部署。5545实际health200/schema2/version98511771871cf0951ecef716bbb55deb13e18da5仍旧部署；最新实现仍在当前Run工作区，没有正式PR、合并、Actions激活或新页面版本一致证据。部署Workflow/在线Runner/控制器已备好，不重新部署旧main冒充最新。真实供应商、支付/外部身份等未验范围保持。后续关键路径仍为当前driver修复→原完整tests→独立QA→report/publish/pr→授权合并→正式Actions/health/页面对账；此次停止定时后不自动执行这些协调者动作。原现场在忽略目录`.data/fresh-8793/check-stop-summary/`，没有手改DB/检查点/回执或新增权限。


## 即时授权部署两项最新版本：平台已是最新功能，产品门禁在途（2026-10-08 15:17）

用户停止定时任务后再次明确要求部署Agent Platform与Model Relay，后者必须走最新正式部署Pipeline，完成后通知。本轮作为即时任务继续，heartbeat保持PAUSED，不自动恢复。用户此前“不替Harness”要求保持：产品实现/固定tests/独立QA/report/publish/pr由原Run完成；协调者核验正式产物后处理已有授权的测试仓合并/正式Actions。

平台当前PID27655仅8793，binary SHA d926462e50e118de886a990cf60d6c1b6e29e21c4226df63f44afcb2f765f905与标准升级receipt一致。维护源HEAD1287e36到实际代码commit abe0dd6仅三份交付文档变更，无功能/模板变更；agent-config.js/workflow-model.js/workflows.js/workflow-runs.js实际HTTP200且逐字节匹配最新维护源。真实登录API与在途Run继续正常，故最新功能已经部署，不为文档commit重启在途Agent。无源main合并/新Platform PR/8788变更。

原seq52 candidate editor-readback-route-r1正式交tests53，133文件指纹6b6564c93b0c15bfefcd33955c3affd852fb8716963dea08e9600004a67b4bab。seq53完整make verify exit2、98085bytes/truncated=false，维护者正式output三页读到eof。前端187项、原Go/build/迁移/旧UI/多上游分项继续通过，新增console-editor-interruption1280真实controlled_get_hold=true/new_editor_save_enabled=true/service_posts1/canceled_editor_posts0，越过seq51漏分页匹配断点；之后console.mjs:90报route.continue Route is already handled，完整三视口/全部新摘要及独立QA未通过。seq54 development active，原生实际说明清理时并行撤路由引发重复继续，拟先结清受控请求再撤路由，原断言保持；当前只是返工，尚无原层新完整通过。QA47-01/P1、QA47-02/P2保持Open，未放宽门禁或代改测试。

正式GitHub Owner big91987、main deploy-local.yml blob434407b…核验，preview/validation安装控制器均SHA782d4861…与维护源一致，activation stage均committed。旧preview/validation仍9851177/schema2；只读SQLite基线preview models0/requests0、validation models2/requests4，原列摘要保存在ignored，用于实际CLI升级/恢复对账，不替代UI/API验收。尝试preview既有initial-password正式登录HTTP401，未读业务API/未重置密码/未改业务数据；后续保留现有凭据，不以初始化文件有效性替代实际登录。CUA inventory超时，未据此编造页面核验。需要实际部署后再核对health与页面，可沿受支持浏览器入口。

Model Relay本轮尚无Open PR或新Deployment；最近Actions仍Issue entry/旧main，不能触发旧main并称最新效果。待当前完整测试与QA通过，经原Connector产物、授权合并后使用同一正式Actions先在隔离环境验证非空升级/失败恢复，再发布preview并核对版本/实际页面。平台验证/产品原log/基线/Controller核验在ignored `.data/fresh-8793/check-deploy-both/`，没有手改DB/检查点/收据或新权限。


## 用户接受较新已合并版本：查看入口核验完成（2026-10-08 15:30）

用户纠正“必须等最新在研版”的理解，明确较新版本也可，只要部署后能看效果。本次查看目标按此指令完成，不等待design-v0.2.0整改，也不放宽其质量门禁。GitHub正式API查全状态PR：最近已合并为PR19（2026-10-06 18:54:22 Asia/Shanghai），多上游/有限故障切换控制台，merge/main SHA98511771871cf0951ecef716bbb55deb13e18da5；PR17及更早也已合并，没有当前在研版本的新PR。main commit核验相同，用户本次接受的“较新可部署版本”就是PR19。

Owner big91987通过正式`deploy-local.yml` workflow_dispatch ref=main/deploy=true/target=preview触发一次，[Actions37743445224](https://github.com/big91987/model-relay/actions/runs/37743445224) completed/success。prepare及Verify and prepare exact main commit成功，原日志明确Already deployed:9851177；deploy job skipped，因为既有实际binary/pointer/health同版已经成立。不能称本轮重新构建、重新跑make verify或激活新版本；正式Pipeline完成同版本核验并保留既有部署，未停止服务/迁移数据/换密码。最近PR并非每次自动发布：main push只prepare，显式Owner deploy才激活。

实际5545 `/healthz` HTTP200，status=ok/storage_schema2/version完整9851177，`deployed.json`同SHA/schema2，binarySHA5255e42b9e8ec617b6794eaa18654d61ec73c65b5f3250ede997a11df745a116；`/admin/`HTTP200。真实Playwright新标签Page Title Model Relay/Page URL5545/admin，管理登录页可呈现；未登录现有生产账户，不冒充authenticated十页UI验收。初始密码文件此前HTTP401事实保持，不重置用户密码。CLI结果有延迟但已实际返回，未强杀服务或浏览器。Codex打开该URL工具返回queued，用户可直接用链接打开，不把queued当已展示。

Agent平台8793最新功能源码abe0dd6已部署；后来HEAD只交付文档。binary d926462e…/维护源四项HTTP资源一致，真实Playwright管理员登录成功进入主编排页，标题和节点页面实际可查；第一次匿名api/me401是正常未登录，不当页面故障。平台保持当前8793，不为文档commit重启在途Run。源码只开发分支提交/推送、不建Platform PR、不合main。

本次两个查看入口已核对：[Agent平台](http://127.0.0.1:8793/workflows/8f497228b60a8ea46b7d37be59ebe578)、[Model Relay PR19版](http://127.0.0.1:5545/admin/)。当前在研design-v0.2.0/QA47仍属于未交付的原Run，不把次新发布核验当其Go/合并部署；Task5/U1保持Active。按新查看目标不再等新版发布，也不执行此前为schema3新候选拟定的隔离故障部署。定时保持PAUSED，原Pipeline不取消，协调者本轮没有代写产品/造QA/手改DB或新增权限。证据在ignored check-deploy-both，原失败/基线/尝试全部保留。


## 用户查看旧版差距与在研Issue进展（2026-10-08 15:48）

用户认为10/6版与当前目标差距很大，随后要求看在研版本Issue和进展。实际读取用户当前5545/admin/#overview：运行9851177，六项导航概览/上游/模型/应用密钥/调用测试/调用记录；上游未配置、模型0/Key0，下一步向导与技术说明占主要区域。它是早期团队网关，不能作为完整多租户管理控制台产品化效果。当前Issue25设计明确承接A明亮管理后台、稳定侧栏、紧凑表格、统一编辑/详情，六类多租户/身份/模型/Key/共享资源/计费用量/账单/迁移功能；有实现候选不等于最终产品化验收。

正式[Issue25](https://github.com/big91987/model-relay/issues/25) Open，title“全新 Pipeline：在 8793 新编排完成 design-v0.2.0 研发交付”；latest公共Hook comment6055263862/07:47:53UTC与正式Run seq57测试一致。seq55原make verify exit2/103324bytes，seq56正式handoff明确三视口编辑中断清理/console已过，continuous1280 J01–J05/J07过，在J06日志页误用仅call页存在按钮，响应等待未处理拒绝超时；此为当前正式交接/Agent恢复结论，本轮尚未另读seq55完整原日志，不把该分项升级成独立维护者完整复验。

seq56 candidate records-detail-entry-r1只修驱动为精确Request ID行详情/同时观察响应与点击，保留30s/200/ID/原始未知和确认3/4/CSV/账单守恒及全部门禁；定向17/20红→20/20绿，193Node/vet/build等局部绿、同19件材料摘要。134文件fb4066fa37d1e7865d3a18f72dd78fef98ae79bde17e20875af373b24771c3a6正式交seq57原完整tests，当前running/dispatched，预算60/冻结40，无error。新完整通过和QA47-01/P1、QA47-02/P2关闭均待实际证据；无开发版新PR/合并/Deployment，不能拿PR19页面作新成果。

用户可查看本Issue、[实际Run](http://127.0.0.1:8793/workflow-runs/4cd62ced6a4f98f50e3cca6f7596eb06)、[最近研发会话](http://127.0.0.1:8793/conversations/53745b666e3dbfb5992ce61ea267925e)。尝试在IAB打开GitHubIssue超时，未宣称已展示；GitHub正式API内容与链接已核对。没有新增任务、User Input、Issue评论、产品改动或恢复动作；定时保持PAUSED、原Run继续。核对现场在ignored check-development-progress/。


## 发布后Issue清点与冻结原型截图（2026-10-08 16:00）

用户要求核对10/6后创建Issue、哪个仍开发，并查看design原型截图。以PR19 merge时间2026-10-06 18:54:22 Shanghai为起点，真实仓库过滤非PR并核对created_at，共Issue20–26七个；20于10/6 19:23创建，其余六个均10/7创建。产品任务20访问生命周期、21完整design-v0.2.0、24新版编排回归、25全新8793 Pipeline四个；22/23/26为Harness澄清/返工/通知验收自动输出三件，不是七个独立产品版本。

只读8792正式API，不恢复或改旧运行：20 cancelled/seq13，21 cancelled/seq6，24 waiting/seq10旧服务待验；22/23 completed/seq6，26前轮completed/seq8且GitHub已closed。GitHub20–25仍Open，不能据Open数宣称多个产品并行开发；旧Issue收尾状态与实际Run并未全部同步。本次未擅自关闭Issue或取消24。当前只有25在新服务实际推进，22/23/26不再执行产品源码。

本轮实际Run25已进入seq58 qa。seq57原完整make verify真实exit0/106669bytes/truncated=false，维护者四页0→32768→65536→98304→106669/eof完整读取；134文件fb4066fa37d1e7865d3a18f72dd78fef98ae79bde17e20875af373b24771c3a6，193Node/0fail/0skip。三视口编辑中断与清理、J01–J07、visual-current-period、management-summary（2模型/2租户/1服务/5Key、api_values_match/context_actions true）、十页console-visual都正式通过；受控HTTP仍不是供应商联调。QA47 P1/P2只能由当前独立QA关闭，尚无Go/新PR/合并/部署，不拿这次绿灯改写旧失败或商用整体完成。

当前design architecture-r1实际承接prepare冻结包design-v0.2.0/SHA5f51…，没有重新生成一份UI原型。正式manifest原件docs/screenshots/overview.jpg、tenant.jpg、keys.jpg、billing-baseline.png均存在，逐件bytes/SHA匹配后仅复制到ignored check-issue-history/prototype/供用户查看；工作台/租户详情/Key三图已人工检查，保留设计演示数据与原型标识，不改图、不当已实现页面或真实费用。设计包内原型静态角标仍含早期v0.1文本，包版本和原件摘要以manifest为准，不改冻结内容。实际设计阶段产物为architecture/implementation-contract/migration-and-operations/verification-design/g2-review，原型来自权威输入。

用户可从Issue25、现Run和[design-v0.2.0发布材料](https://github.com/big91987/model-relay/releases/tag/design-v0.2.0)查看。原照片与正式seq57原日志、Run snapshot在ignored check-issue-history/。定时PAUSED、旧Run不恢复/原工作区不读写、不新增Issue或新输入；本轮只清点/核验/展示原件与更新既有三份记录。


2026-10-08 本轮 Issue25 实时核对：QA58已正式完成，产品候选Go with known issues；QA47-01 P1与QA47-02 P2原层Closed，新增QA58-01 P3技术枚举/窄列断词Open非阻断。seq57完整make verify exit0/106669 bytes/truncated=false；report59完成，publish60正式Connector exit0，真实提交66f1328ada75ea90e5638fbf2bd58663ac86347e已推送workflow/4cd62ced6a4f98f50e3cca6f7596eb06。维护者独立逐件git对象核对134文件与QA58清单及fb4066fa37d1e7865d3a18f72dd78fef98ae79bde17e20875af373b24771c3a6源戳完全一致；GitHub远端ref及固定提交QA报告可访问。

实际卡点：有效预算60耗尽，Run failed/seq60，error为maximum node executions reached；pr尚未派发、Open PR0，不是产品回归失败。检查冻结边publish→pr→done及正式已推送回执，余下为两个明确节点，无返工循环；准备按已有自主接续和测试仓PR授权通过Owner正式return(seq60,target=pr,max_steps=64)，不重放已成功publish、不新建Run/Agent、不改冻结图/历史/DB/权限。恢复结果待后续真实API回读记录。5545仍旧9851177/schema2，无新合并部署；H及真实供应商/支付/外部身份边界继续未验，定时任务保持PAUSED。


正式恢复回执（2026-10-08 19:39 Asia/Shanghai）：Owner return HTTP202，seq60→61 pr，有效max_steps64。原始60步及冻结definition经管理员正式API深比较完全不变；CI return响应含调用者权限相关配置脱敏，与管理员响应不可直接作为同字段比较，先前核对脚本误报AssertionError已通过同Caller回读排除。pr61由正式github.pull_request Connector创建草稿PR27，head_sha66f1328ada75ea90e5638fbf2bd58663ac86347e；URL https://github.com/big91987/model-relay/pull/27 ，已附到当前会话。done62完成，Run completed/error空，不重放publish。Run completed仅表示编排已交付PR，不等于用户部署目标完成；PR尚未合并、正式新Actions部署/health/schema3/实际用户页面未完成，旧5545版本保持。产品Go with known issues与P3保留，定时仍PAUSED。

用户询问预算和Web验证方式：原Run冻结40，协调者上次恢复设60，本轮明确检查只余pr→done后正式设64；是节点执行总次数，包含Agent/Connector/结束节点和返工，不是模型消息或工具调用次数。当前通用模板100，未因此改写老Run预算。seq57宿主make verify经go run ./tests/browser启动真实Go/隔离SQLite/受控HTTP-SSE上游并用Playwright浏览器执行实际UI；1280/1440/390全部J01–J07通过。QA58自身未重新操作浏览器全链：独立核验版本源戳/固定完整回执、设计比对30页+8关键图，独立193Node/5非监听Go race，关闭原层缺陷。区分宿主UI-E2E、独立证据/视觉验收和未完成的正式部署UI，不能声称QA亲自完整复跑或供应商联调已验。


## 长期推进计划与QA浏览器能力核对（2026-10-08）

用户继续授权基于8793迭代Model Relay、维护长期计划并记录/修复卡点。计划已在现有workflows-plan.md增补五个结果阶段，workflow-node-agent-plan.md承接通用改进；不另建平行状态目录、不自行恢复此前PAUSED定时任务。

实时GitHub：PR27 open/draft，base9851177/head66f1328ada75ea90e5638fbf2bd58663ac86347e，mergeable=true/clean；GitGuardian check success。尚无新版合并/部署事实，合并前只读复审进行中，不把扫描通过替代代码评审。当前PR已绑定QA58/seq57同134源文件fb4066fa…指纹；已有正式Connector恢复及Run完成证据保留。

新确认的通用缺口：管理员API回读冻结QA配置含browser-validation.check(auto)，network_access=false/sandbox=workspace-write；browser_tool.py的当前真实合同仅app及docs/workflow/prototype，并创建静态浏览器check。产品Go动态服务不能通过该入口独立验收。seq57宿主真实UI通过和QA58独立截图/证据审查维持原层结论；根因是工具能力覆盖不足，不能说“QA没有工具”或“QA已经独立操作全链”。改进进入正式诊断/工具来源，不修改当前已验候选、不静默扩大QA权限。

公开官方资料核对入口已写入计划，只作为后续能力对照；本机New API源码/数据/秘密未读取。已有商用矩阵与freeze不改写，外部联调和正式页面仍待事实。


合并前复审发现与处理（2026-10-08）：PR27暂不合并。只读代码复审发现两项源路径可达P1，尚未由Pipeline实测复现，不拿静态发现冒称测试结果。

1. 显式零软预算仍可能dispatch：internal/relay/accounting.go:290-292只在Hard=true检查Budget，console_api.go允许持久化budget_micro_usd=0/hard=false；契约implementation-contract.md:95/97明确NULL无限、0禁止受理。按当前revision用正式PUT tenant或Key limits设0/false，其余cap null，再用已授权且已定价Key发chat；预期429且上游dispatch=0、费用/预留不增加，代码当前跳过预算检查。现有console_resource_matrix_test零矩阵强制Hard=true，未覆盖此组合。修复必须保留正软预算告警语义与NULL无上限，不直接把软预算改成硬预算。
2. 过期秘密可能随时钟回拨复活：console_keys.go:395/accounting.go:135/routing.go:177向generationIdentity传raw store.now，console_keys.go:407-411只比当前时间/不退休旧代，store.go:28-32无高水位；admissionClock在鉴权以后且只根据admissions。契约implementation-contract.md:114要求Key/代际/邀请及所有窗口同有效UTC时钟，回拨不复活。让1h过渡旧Key在精确截止后被拒，再把测试clock回拨到截止前，旧Key仍应401且零dispatch；邀请与Key期限、跨进程重启、分钟/月窗口同样需核验。现有GenerationExactDeadline用二次rotation先retire才断言，未覆盖回拨未retire；当前console_contract_test回拨后期待旧月窗口也需核对契约。

发现影响分别是预算停止意图未生效、过期应用权限可能恢复。候选QA58受控验收事实保留，但新发现使发布判定暂不放行；不删除或重写旧Go报告。准备经Owner正式return同当前Run到development，携完整受支持HTTP复现/原契约，产品Agent负责红绿回归和最小修复→原完整tests→独立QA→report→publish更新同PR27。完成Run的正式评审返工是当前交付接续，不恢复取消旧Run；冻结材料、历史与产品源码均不由协调者手改。


返工入口已实测接受：POST /return(seq62,target=development) HTTP202，同Run seq63 running，原生研发会话a61735cc14d2675688203cc090f2e44f已回复“先恢复候选、项目规范及两项复审线索，再补回归并修复”。原definition及前62步用同管理员API深比较完全保持。复审最终分类为零软预算P1、时钟回拨过期凭据P2，两项均Important、必须修复/反证后再放行；前初报使用两项P1仅为初步风险分类，最终以可复验报告和独立QA为准。当前新红绿/完整测试/QA尚未发生，不声称已修复。有效总预算仍64，当前未耗尽，现有恢复合同不允许提前增加；若在正式tests64完成后耗尽，沿原回执检查后正式有界接续，不自动放大、不手改DB。下一次发布更新同PR27，产品代码和公共状态由Pipeline维护。


用户最新要求恢复持续定期推进（2026-10-08）：通过原automation_update更新model-relay为ACTIVE，保留原30分钟频率、当前thread和通知策略；同步长期阶段、PR27/研发63返工、新旧测试版本及QA浏览器边界进任务prompt，不创建重复定时任务。配置正式回读ACTIVE。当前同Run seq63 development running，原生会话a61735cc14d2675688203cc090f2e44f，自报“内进程已复现两项缺陷；继续补重启、期限和窗口守恒回归”；这是Agent执行进展，不等于原完整宿主测试或独立QA通过，PR27仍待修复版重新验证再合并。定期检查先核实时态再推进受支持入口，卡点记录并诊断修复；重要结果/问题通知，正常未变保持安静。此前PAUSED记录是历史，不代表当前自动任务状态。


## 定期检查：预算/时钟整改候选交完整门禁（2026-10-08 22:40）

当前正式API：seq63 development completed，候选budget-clock-r1，HEAD66f1328ada75ea90e5638fbf2bd58663ac86347e+未提交工作树，140件SHA256 ca8ec50aa168cf16ac2e24edfe214aa44d06a05846c34deadbc3c7cb6c02904a；seq64 tests running/connector_dispatched=true，无结果或error。不沿用旧seq57/QA58放行新代码。

维护者已读取attempt-63/checks.md/source-evidence.json及真实red.log、race-current.log：生产handler/SQLite旧代码tenant/key零软预算均status200/dispatch1，过期未retire代际回拨status200/dispatch1；当前内进程/跨进程/正式backup-restore和锁排队期限的局部race实际exit0、85.696s，静态/193Node局部绿。新代码显式零预算独立于Hard拒绝，NULL和正软预算语义保留；增加schema3持久有效UTC高水位，锁取得/受理/dispatch前采样及普通init/Open/恢复兼容。测试调整失败与原日志保留，19冻结材料一致。该层是内进程handler及子进程回归，不冒称真实网络HTTP；新增review-history会在宿主按准确PR27旧提交重建实际HTTP红灯，随后完整原门禁验证当前绿，旧红重建不等于原开发时已执行网络。

当前预算64未结束，不提前加预算、不打断或重跑正在运行的完整测试；完成后若达到上限，按原正式回执/路由检查后有界接续QA或development。GitHub PR27仍Open/Draft且head旧66f1328，无新发布；最新Actions37793164457等仅Issue入口success，5545 health200/schema2/version9851177仍旧部署。QA独立动态Go浏览器与其他H/真实供应商/外部身份支付缺口保持未验。私有本轮API现场保存在.data/fresh-8793/check-20261008-2240，自动任务保持ACTIVE。


seq64完成（2026-10-08 22:44）：原make verify timeout900s实际exit0，110897 bytes/truncated=false；维护者经正式output API偏移0/32768/65536/98304完整读取至110897/eof，140件ca8ec50a…源戳与研发63一致。准确PR27旧提交历史实际HTTP红灯重建观察tenant/key零软预算200/dispatch1和旧代际回拨200/dispatch1；当前正式HTTP及内进程预算/回拨、锁排队精确期限、跨进程恢复回归均PASS，全仓relay race458.373s，Node193通过。1280/1440/390各J01–J07/management-summary/实际周期活动/十页visual通过。冻结材料保持。完整受控测试通过不等于独立QA、真实供应商或正式上线。

同时当前Run failed/seq64，error明确为maximum node executions reached，旧门禁节点completed/result.next，不是产品测试失败。无未明副作用或继续返工环；已检查当前候选、绿色回执与尚余QA→report→publish→同PR→done。准备沿Owner正式return seq64→qa，新的有限总预算100对齐当前标准software-delivery模板，保留原64步/冻结定义/工作区；不重跑已成功tests、不重建Issue/Run、不改回执或权限。恢复结果待实际回读。


有界恢复实际闭合（2026-10-08 22:45）：Owner return(seq64,target=qa,max_steps100) HTTP202，管理员回读Run running/seq65/max_steps100/error空；原64步及冻结definition逐值完全相等。独立QA原生会话09572eed8c075180d4e8b4492aeb7a4f running，已明确核对当前候选与完整宿主回执再形成独立结论，无新正式QA结果。恢复没有重放成功测试或创建Run、修改冻结配置/产品源码/权限。新总预算100有界且对齐当前标准模板，检查依据是有效新候选、明确完整绿门禁和有限QA/report/publish/同PR/done剩余路径，不因定时触发自动增限。

修复候选额外只读合并前复审已交同reviewer，检查零软预算/持久UTC/初始化及恢复/锁排队/事务边界，禁止代产品测试或修代码，结论待返回。正式PR27目前旧head66f1328仍未合并，修复候选尚未publish，不进行Actions新部署。下一步等待QA65及修复复审；缺陷再经原development返工，只有同版门禁与独立验收成立才report→更新同PR→授权合并→正式部署。自动任务prompt已同步当前QA65/预算100/新门禁，30分钟频率及ACTIVE保持。


### 2026-10-08 23:10 修复候选独立QA与合并前复审

正式API回读：QA65 completed，原生09572eed8c075180d4e8b4492aeb7a4f closed；report66/a5800bfff49d592e0c115b16d16b0783 running，Run running/error空，总预算100。QA65 acceptance-report-65/matrix-65/report-handoff-65及attempt-65/integrity、host-seq64-review均为正式产物。维护者独立按QA65 source_files逐项核对140文件字节数与SHA，重算源戳ca8ec50aa168cf16ac2e24edfe214aa44d06a05846c34deadbc3c7cb6c02904a匹配；当前源码未代改或代测。19件冻结材料及archive仍匹配。

独立QA产品结论Go with known issues：零软预算与过期未retire代际回拨复活Closed，无新增P0/P1；亲自六组非监听race24.750s exit0和193Node通过，原完整seq64回执110897bytes读至EOF。宿主当前ActualHTTP、跨进程/CLI备份恢复、三视口J01–J07及十页证据复核成立；51件当前UI文件留存并核摘要，QA亲自查看3件PNG，错误历史截图选择明确拒绝，不声称逐页亲自浏览或把证据审查标独立浏览器实操。原生QA UI仍Blocked；QA58-01 P3非阻断，独立self_hosted首次UI、真实供应商/支付/外部身份、H02/H03/H04/H08及正式部署H07保留Not Run。

既有合并前reviewer对修复工作树追加只读复审：显式非NULL零预算独立拒绝、NULL/正软预算语义保留；持久max(previous,wall)有效UTC经正常schema3 init/Open加载，锁后及dispatch前刷新，时钟事务先于业务事务、失败返回503，没有发现新的Critical/Important或反向锁依赖。该结论只读且限140件ca8ec50a…候选；未重跑测试、未证明吞吐，不能放行旧PR27 head66f1328本身。

GitHub正式回读PR27 open/Draft/head66f1328ada75ea90e5638fbf2bd58663ac86347e，尚不含修复；报告节点正在更新原公共进度和本Run pr.md，等待正式publish/pr，再核对新提交与QA65一致后授权合并和verified-main Actions部署。产品5545仍9851177/schema2，未上线新候选；不重放测试、不建重复Run/PR、不改回执或权限。私有核对记录在ignored .data/fresh-8793/check-20261008-2310，公共记录只写结论和可追踪正式产物。


### 2026-10-08 23:40 QA产物目录导致正式发布拒绝

实时Run failed/seq67 publish，exit1/978bytes/truncated=false：repository.py delivery_artifacts拒绝QA65 artifacts中的acceptance/evidence/attempt-65/host目录；report66已完成并保持140件ca8ec50a…未变。用户影响：新修复尚未推送到PR27，合并/正式部署不能推进；GitHub仍Draft/head66f1328，不能把QA候选当已上线。

已按systematic-debugging读取完整回执与维护源，进行只读最小复现：QA65全部产物仅该项不是文件；目录下证据确实存在。当前通用QA/common指令明确要求实际文件，发布器只接受同Run文档根内非空文件，未发生协议不一致；这是Agent交接路径不合约，不是产品测试失败或需要放宽目录权限。发布校验在git add/commit/push之前抛错，工作区HEAD66f1328、暂存为空；140件逐项字节摘要及总源戳再核匹配，未有未明提交/推送副作用。

后续处理：沿Owner正式return seq67→qa，要求只纠正产物交接，列出实际文件或已有ui-artifact-index.json引用其留存清单，保留QA65原证据与Blocked/Not Run边界，不补造测试、不改产品/门禁。新QA应核同版源码及真实材料后交接report→publish→同PR；源码变化才需受影响tests/QA重新验证。当前总预算100有余，不增加限额、改冻结定义或回执。目录拒绝属于现有安全契约，本轮不把发布器改成递归接受目录。正式恢复及实际再次发布结果待回读。

正式恢复回读：普通failed seq67直接return按现有约束返回409、未改变状态；源码确认普通失败须先stop到stopped（执行次数耗尽的特例除外）。Owner stop HTTP202→stopped，再return seq67→qa HTTP202，Run running/seq68、原生QA ee880602e02ccb47c27283b35322d291 running/error空。管理员同调用者核对前66步/冻结definition完全不变，seq67失败connector_receipt逐值保持；其状态由正式回退记录为cancelled并附原因，不补造通过。纠正由Pipeline QA执行，尚未有新的有效交接或发布回执。原生源码与安装指令同源，问题不需要目录许可、数据库编辑或新配置；源平台通用能力无永久修复新结论。


### 2026-10-09 00:10 文件级恢复完成、精确合并与正式部署分发

正式Run completed/seq72/error空：QA68与report69只纠正交接和公共说明，report69携带75件实际文件清单；publish70 exit0/124bytes/truncated=false，pushed=true、head e613038e6cc4a3ce63782a95d35cd42f74cb354c；pr71正式绑定同PR27及该head，没有创建重复PR，done72 completed。原67失败仍保存，其取消状态来自正式回退，不能解释为发布成功。

维护者合并前逐项读取新提交git对象而非仅查未提交工作树：140件字节数/SHA及总源戳ca8ec50aa168cf16ac2e24edfe214aa44d06a05846c34deadbc3c7cb6c02904a全部与QA65 integrity一致，checkout干净，GitHub PR27确认同head/base9851177、mergeable_state clean及GitGuardian success。原完整make verify/QA65和修复只读复审适用于该提交；没有代产品实现或重跑门禁。

按既有测试仓授权将原PR27 ready，再以sha=e613038…执行精确合并，正式API merged=true/mergeSHA cd413b5aa24a372a19c19af999a6ef61b5c41572，main ref同值。未创建Platform PR或合并源main。正式Deploy Model Relay locally（375348269）：push prepare [37806816549](https://github.com/big91987/model-relay/actions/runs/37806816549) in_progress；Owner workflow_dispatch main/deploy=true/target=preview已发送，[37806849658](https://github.com/big91987/model-relay/actions/runs/37806849658) queued，两者headSHA均mergeSHA。自动push只准备；只有dispatch activate成功及真实Deployment/healthz/schema3/admin页面闭合才认最新正式部署。没有手启临时服务/改数据/绕门禁；当前仍待prepare完整门禁、激活和实际页面核验，不提前宣称上线。后续不重复dispatch或继续旧完成Run盲循环；当前阶段1剩余外部发布事实核验后才选择长期后续阶段。


正式准备失败（2026-10-09 00:14）：push prepare37806816549 failure、dispatch37806849658 failure，均未进入activate，5545实际health200/旧9851177/schema2仍健康。前述in_progress/queued是分发时点历史，不是当前部署状态。读取Actions失败日志及正式私有verify-cd413b5…日志，当前精确merge源戳仍ca8ec50a…。首次prepare私有日志被同SHA第二prepare以w模式覆盖，保留第二次完整可得日志至ignored check-20261009-0010/deploy-verify-failed.log；不得声称已恢复首次全部失败原因。第二次真实失败为frontend-test 192例/191pass/1fail，route-cleanup.test.mjs加载playwright-core/package.json报MODULE_NOT_FOUND，未到全量Go/browser/activate。历史HealthMethodsHTTP红灯重建已按分类器PASS，不能把其中FAIL误认为当前失败原因。

最小只读诊断：产品Makefile先frontend-test、test、build，browser最后才npm ci；新增route-cleanup.test.mjs在frontend-test中require.resolve依赖。此前seq64/QA工作区已有node_modules，使干净安装问题未暴露。现象不是权限或供应商错误；本轮不在运行副本手装包或跳过例子。产品依赖/固定入口根因由新main Pipeline Agent修复；新的真实Issue应从合并main开始，用标准锁定安装和干净工作树验证，原Issue25/PR27保持已完成历史，不继续旧Run盲返工。通用部署器按SHA复用日志覆盖不同尝试是维护源可复验缺陷，平台另行处理唯一源码、标准安装/升级；本轮先保留可得证据，不更改在途控制器。

已创建唯一新Bug [Issue28](https://github.com/big91987/model-relay/issues/28)，目标从当前main修复干净安装依赖顺序并完成正式部署；未重复原Issue25。自动入站[37807379538](https://github.com/big91987/model-relay/actions/runs/37807379538)失败于gh api读取Issue28，尚未clone/Start Run，正式API仅原完成Run；Actions仅给出CLI非零，没有保留HTTP原因，不能归因权限或供应商。之前相同入口多次真实成功，本轮Owner gh读取正常；计划对原入站Actions作一次有界rerun，保留同Issue/幂等request入口，不改凭据/权限/配置。重试若继续失败保留新日志和未知原因，不反复盲试。

Issue28入站第二次实际结果：Actions37807379538 run_attempt2 success，正式API唯一Run3e7496815b9922704072482cf44e73e2 running/error空，issue2 completed→intake3/ccdf61b328f2751386b7efe433469851 running。新checkout由原SDK prepare创建，从合并main开始；本协调者没有代写产品、临时装依赖或发重复任务。原首轮GH读取非零原因未知保留，后续通过不倒写成已定位网络原因。

通用日志修复采用已授权的最小有界方案：源码controller.prepare每次真正make verify用NamedTemporaryFile在原private logs中独立600文件，失败后保留，随后stamp仍追加同次路径；ready候选复用不假装重跑。现有正常/升级/停服/数据协议不变。TDD真实git/make连续两次失败保留及一次成功回归：旧版仅1日志vs2预期失败，修复后3日志且两失败字节不变、权限无group/other读取、计划成功绑定SHA；完整*_test.py 60件58.517s exit0（默认test*.py无匹配的初次发现不算测试，已纠正实际命令）。README记录私有日志和标准升级，证据在ignored controller-log-red/green/suite.log。修复只读复审和标准安装升级待实际回读，正式新版本Actions复验仍待产品Issue28候选。

日志修复只读独立复审完成：无Critical/Important，确认mkstemp排他创建/0600/delete=False、失败保留、重试不删logs、ready复用不重跑，原部署锁/令牌隔离/标准安装升级保持。reviewer未代跑测试；启动异常/父进程强杀可能不打印已创建日志路径、断电持久性未测，保留限制。新Issue28冻结定义实际max_steps100，与当前标准模板一致，无提前增限。

标准升级与实际状态回读（2026-10-09 00:20后）：部署两目标activation均committed，失败Actions已完成，无在途activate。维护源789cc90原install.py以原参数升级两个私有根，未手改运行副本；source/两controller.py/两controller-install.json SHA均2d456ca50265cdd6db89ee4b0f3d0e85695bec45adff7a2b2665c826d48fab79。前后原preview.json/activation stage及尚可得verify-cd413b5…字节摘要完全相等，两health均200/9851177/schema2。安装器未init、部署或start产品、未替换实际数据/密钥。新装/升级回归含在60项中，既有两安装标准升级已应用；新控制器真实Actions尝试尚待产品修复候选，不宣称完整新部署成功或补回首次覆盖日志。源平台只开发分支提交。

Issue28当前正式API：intake3 completed→development4/93e40ccfc93d89bd4400be032eb6947b running/error空，总预算100。已进入原生研发，不是仅创建Issue或说明计划；源码实现/干净固定门禁/独立QA/新PR及最新正式部署仍待真实结果。当前工作锚点切换Issue28/Run3e7496815b9922704072482cf44e73e2，原Issue25/Run72/PR27merged保留历史。


### 2026-10-09 00:40 Issue28冷安装候选进入正式固定门禁

实时正式API：development4 completed，tests5 running/connector_dispatched=true，创建时间16:33:28Z，Run running/error空，总预算100。新工作区github-issue-5765183967，基线cd413b5；source-after.json为142件8e82aaa123df25a4758a1939fb2d018713831507aee337e18dfa6100c49f619e。研发候选只改共享verification-deps/README/回归和Trellis说明，package锁未改，不改业务/UTC/预算/平台控制器。维护者只读核clean-before.log：node_modules absent、NODE_PATH unset及Node26/npm11.12.1/Go1.25.6，读原frontend红与顺序回归红/绿记录。原生npm ci受EPERM exit2、测试未开始是真实限制，不记原生完整绿；正式宿主测试自然安装后node_modules当前存在，也不等于完整通过。

tests5尚运行时output API offset0返回404 record not found，无新完整命令回执；未读取私有临时输出伪装正式回执、不盲重试或代跑make。待正式completed再读完整日志至EOF，检查冷入口、142件指纹、旧全部回归及三视口，然后独立QA。额外候选只读合并前复审已交既有reviewer，结论待返回，不代Pipeline测试。GitHub暂无open PR、新正式部署；最近两次cd413b5 prepare/dispatch仍failure历史，预览尚未更新。不因定时检查自动启动副本或扩大预算。

候选只读复审结果：无Critical/Important，frontend-test/browser消费者前锁定安装、失败停止成立；同一次make共享phony前置目标，verify递归步骤串行。verify顺序安装两次的缓存开销保留，不与自身测试并发；同目录独立多make进程无跨进程安装锁，当前正式串行入口不触发，未扩大声称并行支持。reviewer未跑产品测试，尚不能据此放行，tests5仍running/无完整回执。


### 2026-10-09 01:10 固定门禁通过但干净环境证据未放行

正式API：tests5 completed exit0/111871bytes/truncated=false，维护者read output offset0/32768/65536/98304至111871 EOF，原make verify保留193Node/Go预算UTC迁移/三视口J01–J07/十页绿。142件8e82aaa…与QA6同版。QA6真实Blocked/发布No-Go：缺宿主开始时独立候选/无node_modules/NODE_PATH unset/无预装的原层证明，历史完整日志只有平台回执与审查投影，原生完整入口监听失败仍保持受限；没有把安装后已有依赖倒推冷启动。

QA已通过handoff返回development7，研发只新增本Run临时隔离采集脚本host-clean-verify.sh，建议宿主执行bash该文件，未改固定Makefile；随后tests8自17:05:37Z running。维护者只读核 frozen tests Connector argv仍repository.py verify --test-command [make,verify]，run_tests直接执行原固定命令，既有development模板也已要求必要诊断进入原入口。交接字段中的建议命令不会改变授权argv，因此tests8仍测试已有依赖工作区，无法关闭冷安装AC；并非工具应执行任意Agent推荐脚本的缺陷。

计划最小正式恢复：保存seq5实际完整回执与新限制作证据，停止当前无发布副作用的tests8，待Connector quiet后Owner return development。要求产品Agent将必要隔离/前后环境和完整日志采集落实到项目可复用固定make verify入口，保留所有原门禁；不能让协调者临时运行脚本/装包、改冻结Connector或权限、单独硬编码本Issue作为交付。新的记录不得伪造seq5开始状态或完整文件副本；过去日志可继续以实际平台全文出口引用，冷安装新证据须实际产生。固定命令接口不需要放宽，按既有源模板和受支持恢复纠正Agent交接；停止/再测试结果待回读。

正式恢复结果：Owner stop8 HTTP202→stopped，connector_receipt exit-1/log102563bytes/truncated=false，error command interrupted/effects may be partial，维护者通过output API读取至EOF。末段证明热入口Go/Node/部分CLI迁移和browser1280/390在中断前执行，不是完整通过；没有commit/push/activate节点，5545实际health200旧9851177/schema2保持。测试资源仍属于临时受控夹具，停止回执不证明所有外部残留已清理，已要求下一研发检查自己现场、不盲杀他进程。

Owner return8→development HTTP202，Run running/seq9，原生87226f924596c3a7559de78d37116e6c running/error空；管理员同调用者前7步/冻结definition逐值不变，seq8中断receipt逐值保留、正式回退记cancelled原因。原tests5出口全文URL及read_command_output页是真实可追踪历史，不虚构其开始状态/仓库副本；新完整冷证据需实际固定入口生成。没有新增权限、预算、重复Run或产品手工修改。阶段9新代码/实际冷门禁/QA再审和正式PR部署均待真实结果，仍No-Go。


### 2026-10-09 01:40 新隔离固定入口前置失败与复制边界复审

正式tests10完成exit2/239bytes/truncated=false，原make verify已实际调用tests/verify，打印唯一test-results/verification/20261008T173346Z-400132109目录；失败在npm prerequisite，未进入原全量门禁，不冒充冷绿。读ignored副本verify.log只有40bytes失败摘要，旧入口没有原始npm stderr，诊断不足保留。平台failed路由正常自动到development11/f8e0283249bd7db33e710c60b4963cba running，无手工重启或额外Run。研发11真实最小npm --version复现NPM_CONFIG_USERCONFIG/GLOBALCONFIG同/dev/null导致double-loading config、exit1，与首次无法归因的原层日志不同；正在改两个隔离空文件并采集原层诊断。定向日志有顺序/失败/采集夹具绿，不能当完整冷make通过。当前144件源码戳4d484086e12d71308e42b963ec0266f5f756924f8073defd1c5622a6260c9025为待测试候选，旧142件审查/seq5绿不代新入口验收。

新入口只读reviewer发现Important/P2待复现：copyFiles词法相对路径和copyFile最终文件Lstat不能拒绝父目录链接；若原tracked路径父目录如internal被替为指向根外目录的符号链接，最终普通文件可被穿过父链接读取/复制，违背新规范只复制候选/拒绝path escapes。最终文件symlink测试未覆盖该情况。影响是隔离副本可能包含工作区根外内容，必须由Pipeline最小真实文件复制复现、修复和根内正常/最终链接/父链接/边界恢复回归；不放宽权限、不临时删链接绕过、不由协调者改产品。复审未执行复制探针，当前为静态可达发现，不能虚报已实测根外泄漏；将经正式User Input交当前研发处理后再固定宿主与独立QA。其余日志/取消/缓存边界复审继续。

复审最终共3项Important/P2：父目录symlink复制逃逸、sourceEvidence/依赖Chromium probe失败stderr未进完整日志、辅助Git/源码/探针未纳入ctx及有界进程取消。审查main.go摘要8436e8bf…e1bf/main_test902de886…e848期间稳定；仅静态结论，未进行根外读取或取消实验。npm两个独立空配置文件的修复已在当前代码，HOME仍继承，隔离范围不是整个主目录。

准备向development11发正式消息前实时seq检查发现已到tests12，断言在POST前退出，未发送、不丢失队列或错误标记为已输入。需按本次已知Important发现对当前tests12正式stop，检查回执/副作用并Owner return development，交产品Agent最小复现及修复；不等整套长测试结束后才处理已知边界，不提前放宽任何门禁或把未复现发现当实际泄漏。

Owner stop12 HTTP202→stopped，exit-1/61016bytes/truncated=false、原错误effects may be partial，output API读至EOF；193Node通过、Go race启动后中断不算全量绿。执行器ignored state 20261008T174328Z-4124892975仍cleanup=not_started/log_bytes0，不能用外层stopped补造内层完成；先保留状态与源版本。只读ps探针因非UTF8进程文本首次失败，容错解码后含model-relay-verify/clean目录参数匹配空；这只是有范围的观察，不证明全部无路径子进程/端口已净，无广泛kill。5545实际health200/9851177/schema2保持，没有Git发布或activate副作用。

Owner return12→development HTTP202，Run running/seq13 pending/error空；管理员同调用者前11步/冻结definition逐值保持、seq12 connector_receipt逐值不变。反馈同时包含3项静态发现及复现要求、state未闭合和其原因未确定边界；源码真实修复、辅助失败/取消回归、下一完整冷门禁与独立QA均待实际结果。所有普通选择与测试仓新PR/合并/正式部署授权保持，未扩权限/预算或重建任务。当前工作不是产品上线，禁止以旧seq5或仅夹具绿放行新入口。


### 2026-10-09 02:10 三项P2整改进入新完整冷门禁

正式API：development13 completed，tests14 running/connector_dispatched=true，自2026-10-08T18:06:06Z，Run running/error空/max_steps100。144件新源码戳dc73ac404db4daaf7e264552c0940abb294b2b1b48684e792eec90822b6eb665，main.go a57141093ee44755e7f138bdf9a0cf90889a6bce2fe74ff390ce546a9d344126、main_test.go a3a2040ae4f5d2c9d7bdff35b05600b4581063d8305d253e9462a89b0db42cdc。读取研发remediation-development-13及原层记录：父目录链接复制红、辅助stdout/stderr失败丢诊断红、50ms取消仍等待约1.0375s红均有真实最小复现。实现os.Root/逐层校验、共用execute捕获/解析分离、辅助ctx/两分钟有界监督；定向两包race、真实SIGTERM与三次取消重复绿。中间旧格式断言失败和测试自身跨goroutine读取Cmd.Process竞态失败保留，后从就绪文件获取归属PID整改；不将这些定向结果当冷安装/完整HTTP/browser/供应商验收。

实际新runner目录test-results/verification/20261008T180607Z-863663337开始state记录clean_start_measured=true、node_modules_before/npm_cache_before=false、NODE_PATH unset、manual_preinstall none及dc73ac候选源戳；前置Git/evidence/version命令exit0/owned_group_absent=true。运行中finished_utc空/cleanup not_started等只是尚未完成状态，不当失败或清理已完成。私有实时日志尾部193Node通过、Go race仍执行，不能代完整正式Connector回执。待正式completed后经output API至EOF，核对新source before/after/完整日志摘要/所有结束清理状态及原全部门禁，再新独立QA。旧seq5/10/12结果不放行此候选。新候选已发既有reviewer独立只读复审，结论尚待。

GitHub实际Open PR为空，尚无新publish/merge/activate。5545/5546实际health200/version98511771871cf0951ecef716bbb55deb13e18da5；未替代部署Pipeline或修改产品工作树。唯一Run继续自然执行，原预算100不扩大。平台源仅更新既有计划/证据，旧controller安装版本与其实际下一Actions复验待办保持。

新候选复审结果：审查前后main.go/main_test.go摘要与development13指定dc73ac候选一致。三项P2在当前源码及回归中静态闭合，未发现新增Critical/Important。reviewer未跑测试/修改文件；HOME继承、历史或脱离当前组的进程不在该证明内、强杀可能留未完成receipt等限制保持。正式tests14仍running，独立QA尚未开始，不据此宣布完整通过或部署。


### 2026-10-09 02:40 完整冷入口内层时限失败与原层诊断

正式APItests14 completed/route failed/exit2/log249788bytes/truncated=false，18:18:11Z结束。维护者通过output API offset0/32768/65536/98304/131072/163840/196608/229376至249788 EOF，私有完整保存，不用工具截断投影声称全文。产品已归档原runner verify.log249576bytes/SHA256 e37932ca114a297ca2e0cd69b8ca3efb6fbe1d14f1d5a866d478e5c7cb616e5e，维护者实际重算与state一致，两种日志长度/范围不混同。state始18:06:07.567014Z、终18:18:11.700773Z，冷起点无node_modules/npm cache/NODE_PATH unset/未预装，144件dc73ac源码前后不变，gate_exit=-1/exit_code1，全部辅助及gate owned_group_absent=true，candidate_removed。末段signal terminated/context deadline exceeded与源码12分钟withTimeout一致，不是原缺包/Owner stop；三视口J01–J07/十页未全完，不能Pass。当前本轮清理证据不反写旧8/12或宣称所有全局进程已净。

历史分类器、193Node/Go/race/build及部分迁移/浏览器路由已执行；390视口failover阶段的泛化routing Error出现在父期限终止后，不能倒推产品断言先失败。选择日志153条仅证明当时选项/焦点边界，不证明后续请求/记录/冷却结束；累计冷构建/正常浏览器总耗时、动作阻塞或产品异步状态尚未归因。Pipeline自动failed→development15 completed→tests16 running，自18:33:16Z，原预算100。研发15新增BROWSER_PHASE UTC/elapsed、routing_call各原生边界durations及一次pending观测，147件候选24725bbb7833cfc2665d44b67de4982cc045aa375ad5ec65c68c6c7d6f2b044e；未改12min/Connector900秒、原门禁/断言/动作/请求次数/31秒冷却/SSE、业务/DB/锁。局部194Node（原193加诊断1）、Go阶段及两包race/static绿只证明诊断本身，不证明新浏览器机制或冷全绿。新诊断只读复审待结果。

当前tests16真实新目录test-results/verification/20261008T183317Z-269232865，开始同147件源戳和冷状态，尚缺正常结束完整回执；未将实时局部日志当正式通过。再次失败应携阶段UTC/耗时/调用边界数据返工，先最小归因再修，不增限盲试、删依赖/手装包、执行推荐脚本或协调者代写产品。GitHub实际Open PR空，未发布/合并/activate；完整冷门禁、新独立QA与正式部署仍待，旧preview未冒称最新。

诊断候选只读复审已核同147件24725bbb源码戳，Makefile及runner未变，无新增Critical/Important。动作一次/await原返回与异常、先注册响应再press再等待、请求次数/31秒恢复/SSE与原断言均保留；timer仅白名单输出且finally清理，没有重试/增限。当前实时原层BROWSER_PHASE migration在18:41:56Z开始、18:42:09Z结束（13139ms），相对18:33:17Z新冷入口到首browser阶段约8分39秒；这是新诊断的实际耗时观察，尚不能认定单步卡住或所有剩余旅程的根因。需最终全阶段数据，不把局部时间改写为通过或先行放宽。


### 2026-10-09 03:10 累计耗时归因与有界独立场景调度

正式tests16 completed/failed/exit2/294002bytes/truncated=false。协调者output API offset0/32768/65536/98304/131072/163840/196608/229376/262144至294002 EOF保存完整，非Agent尾页工具投影。产品原runner归档293790bytes/SHA a17eceef5a36d90431fdf1353c7a85ff32ca6f78305c01409bea4d5fa9fbfdb2，实际重算与state一致，区别两种日志长度。原147件24725bbb冷起点/源码前后不变、有效起终UTC、自有组消失/candidate removed保持；gate_exit-1/exit1，内层deadline非Owner stop/缺包；无最终成功Chromium探针/全链QA。

实际日志internal/relay race450.421秒；首migration距入口519.258秒、13.139s结束；legacy-ui88.239s；routing18:43:38Z启动未end。149次routing调用start/end对应、0pending，最后390px调用18:45:08Z结束后冷却渲染被父期限终止；因此不能猜测单调用/DOM产品缺陷，也不能把后续console旅程当完成。Pipeline自动failed→development17：只在独立Go场景增T.Parallel及原全包race/count1命令parallel4，fixture每例独立TempDir/SQLite/Server/clock/计数，原断言/HTTP/历史预期红/预算UTC迁移/原生浏览器动作/31秒恢复/SSE/冷缓存/锁/12分钟及Connector900秒保持。局部13状态码race32.244→11.765秒、五组31.548s绿，父0秒不当总性能。宽局部browser HTTP监听仍原生sandbox bind拒绝/panic exit1保留，不提权或重复；194Node/runner/依赖及无监听阶段/static绿不代宿主完整验证。当前147件新源28571f199f257d5c0bb09fc679150ddb8c5beff822c1a8c813a3bb54c064eeb7，新并行隔离独立静态复审进行中。

正式tests18自19:02:33Z running，Run error空/max_steps100。实际独立receipt20261008T190233Z-3317990147起点19:02:33.573258Z，首migration19:08:48.202642Z（374.629秒，较16提前144.629秒），13.174秒完成；legacy-ui88.362秒绿，routing19:10:29Z开始。实时局部时序仅证明前置变短，缺新完整Connector/原全旅程/锁Chromium最终probe/结束清理/独立QA不能Pass，不保证deadline已解决。当前GitHubOpen PR空、无publish/merge/activate；未代Harness写产品或执行替代命令。仍沿自然固定门禁、必要失败原层诊断和正式交接推进，不无限增限盲跑。

维护者对seq16正式完整日志独立重数149start/149end、末次390px sequence73 elapsed152ms/oktrue，并核450.421s及各BROWSER_PHASE时序，与研发分析一致。新候选reviewer重核147件28571f…，八测试文件纯增9行Parallel，各例Parallel后再建fixture、cleanup绑实际子test；无parent defer提前关闭或全局环境/client修改，静态无新增Critical/Important。并行上限明确是每包testing parallel槽，不限制原go test跨包-p并发或每场景内部goroutine，不宣称全宿主四进程。reviewer未跑真实HTTP/race/门禁，tests18仍需实际证据。5545实时health200/version9851177，未当最新部署。

同轮03:14正式回读：tests18 completed/failed/exit2/307458bytes/truncated=false，output十页至307458 EOF完整保存；pipeline自动到development19/c436887a5bd5a1f60da22b514693595e running，无重复恢复或手工任务。实际runner307276bytes/SHA cb92549be85d96006a9d9131a12067982e41abccf5e75147c7353f0803e7c430与state一致，始2026-10-08T19:02:33.573258Z、终2026-10-08T19:13:01.656265Z，源/副本不变、gate_exit2/exit1、自有组消失/candidate_removed。不是context deadline，全部BROWSER_PHASE migration13.174s、legacy-ui88.362s、routing122.444s、console15.157s结束ok=true；full-console13.735s结束ok=false。continuous-console1280 J01–J07及visual-current-period已输出pass，随后summary-model-context-toggle/form-submit/assertion，last_status200/read expected/actual200，editor关闭/workspace可见；这些事实不足以给断言通过，未知错误等待边界或业务实现原因须Agent最小复现，不臆断。后续三视口完整连续旅程、最终Chromium probe和新独立QA未通过。累计前置调度改善不能当整体交付完成；保留原16deadline与18具体断言不同失败，不用放宽时限/删除断言或新功能绕过。正式新发布仍未发生。


### 2026-10-09 03:40 正式完整冷门禁通过与遗留断言质量评估

development19只新增summary_cell的row/column/value白名单、content_loading布尔和固定文件位置，不改产品同步、原断言/error身份/动作/时限。147件新候选f8befd158a543afeb1ba77778e4caf72e876f6e4b0b988dd4e6ab73de5e5eaab。原save后的dialog关闭早于sync/render是源码可达时序假设，seq18 failure_boundary不足，尚无真实原层证据证明产品/驱动/异步等待哪种根因；不因新诊断将原问题关闭。

正式tests20 completed/next/exit0/310815bytes/truncated=false；协调者output API offset0/32768/65536/98304/131072/163840/196608/229376/262144/294912至310815 EOF完整保存，SHA2e5e4b152f9d6240f886059e00373f4013a7bae0304c78f262a8167423095314。必须按本次实际stdout verification_receipt=test-results/verification/20261008T192336Z-2658854976定位，不取目录latest（QA自身新受限尝试另有记录）。对应runner310718bytes/SHA7c4d3952943721deae9188a6f52f860572037ac8ea39ba55d278d6bf2c7abb7b实算与state一致；始19:23:36.894123Z、终19:34:44.195826Z，冷开始无node_modules/npm cache、NODE_PATH unset/无预装，147件f8befd源/副本前后不变，gate_exit0/exit0，gate及所有辅助owned_group_absent=true、candidate_removed。最终probe成功且依赖从candidate/node_modules解析、playwright/core1.59.1/Chromium147.0.7727.15。原全部门禁、197Node/Go race/预算UTC迁移历史分类器、1280/1440/390全部J01–J07/summary/visual十页成功；受控HTTP非真实供应商。第一次原入口冷完整成功是真实事实，不等于旧间歇断言已修复。

pipeline自然进入QA21/ca57395d6cb44be7adb749ec59717783 running，原生正在归档并独立复验隔离/失败清理。Coordinator已fresh确认seq21后Owner POST messages HTTP202/request_id model-relay:issue28:qa21:seq18-summary-closure-review，要求独立核对旧18断言、新19仅诊断的差别及根因未知的发布影响/分级，必要时正式handoff最小复现及同步契约回归；不以一次绿写作修复闭合，不盲目重跑全套或放弱断言。普通选择已授权，未改图/状态/receipt/权限/预算，不重复任务。新诊断reviewer只读评估进行中。动态原生浏览器仍Blocked，供应商Not Run；nativeQA受限尝试的独立目录与host20区分，不能混成回归失败或第二次全绿。GitHub实际Open PR空、新QA结论/PR/合并/Actions/Deployment/新health与页面仍待。

新诊断复审已重核147件f8befd源戳：原唯一行/列存在/值相同断言及原error重抛保持，无新等待/重试/操作改变，诊断仅白名单计数/布尔和启用停用other，不泄漏原值/异常/凭据；无新增Critical/Important。该静态结果亦不关闭旧18间歇失败。正式QA21仍running、其原生“可进入精确发布”进度自报不代最终handoff/报告与遗留评估；后续须核Owner新增input是否正式承接后才按最终结果推进。

正式conversation回读：Owner遗留评估input id16161/user/queued，QA仍running；现有“可进入精确发布”进度消息关联初始parent16072，尚不能认定已处理新增input。不重复投递，等待原生排队接续及最终QA报告/工具handoff后核评估结果。未在queued状态自行合并或改检查点。


### 2026-10-09 04:10 精确候选合并并进入正式部署

实时Run completed/error空/25步。QA21已处理queued Owner input，summary-assessment正式保留R28-04 Open/P2根因未知，仅已证当次验收延期、未实证产品持续错状态/P1。源码save缺post-write刷新代次屏障只是静态解释，既没有把一次20绿标成修复，也没有虚造用户已接受风险；QA建议Go with known issues进入精确正式prepare，任一后续门禁红必须No-Go、按实际GET hold/release/同scope DOM提交契约最小重现，实证核心结果错误须升P1返研发。动态独立browser仍Blocked、供应商Not Run。报告22公开同边界并只改交付文档；publish23 exit0、pr24确认PR29及head7f0b2e72b442332fd48a3b86982fae5e834f45a2，已attach当前chat，不恢复旧Run25或旧PR27。

维护者在published HEAD通过git ls-tree/cat-file --batch按tests/evidence同库存重算147件源f8befd158a543afeb1ba77778e4caf72e876f6e4b0b988dd4e6ab73de5e5eaab，每件git对象与工作树原字节一致、树干净；QA报告/summary评估/host20state也与发布对象字节一致。GitHub实际mergeable/CLEAN/GitGuardian成功，同源新诊断/调度/runner各审查无新增Critical/Important。按既有测试仓合并授权ready PR29并PUT merge以精确sha约束，返回merged=true/main9bf6ecaf53758ca2258498968b830c535e02e1c0。Platform未提PR/合main。Issue28随Closes关闭不表示Deployment已完成，发布页面/存量升级/原密码保持仍要实际验证。

正式Actions：push prepare37837649432（20:12:03Z）in_progress，Owner workflow375348269 dispatch deploy=true/target=preview/main触发37837659544（20:12:07Z）pending，同新main9bf6ecaf。不重复触发或协调者代执行make/activate。prepare job Verify and prepare exact main commit尚执行；旧preview health200/version9851177维持，未宣布新服务上线。若成功ready复用不应假装第二次verify；失败完整日志/状态应保留，再红须按QA契约诊断/受支持新阶段，而非盲试/增限。

通用controller真实集成首证据：安装SHA2d456ca50265cdd6db89ee4b0f3d0e85695bec45adff7a2b2665c826d48fab79，实际新日志verify-9bf6ecaf53758ca2258498968b830c535e02e1c0-adk17kh_.log mode0600/当时147832bytes；唯一实际attempt文件由正式prepare创建，旧verify-cd413b5….log与先前私有归档逐字节相同/SHA1fcb76adac11cd22cb06f92a7787750d64fecddf6e132cd3d769ec210adf94fc。这证明真实入口新日志命名/权限与旧证据保留，尚不证明当前完整verify、后续激活成功或连续两个正式失败复验。此前源码红绿/60项测试及标准install upgrade保持历史，不新增手工补丁或重跑已通过测试。


### 2026-10-09 04:40 正式部署成功与下一管理操作迭代

正式push37837649432 success；Owner部署37837659544 success，prepare20:23:33–42Z、deploy20:23:47–56Z，activate日志实际Deployed9bf6ecaf53758ca2258498968b830c535e02e1c0/URL5545admin。GitHub Deployment6945948215/local-preview于20:23:58Z状态success/environment_url http://127.0.0.1:5545/admin/。actual health5545=200/statusok/storage_schema3/version同新main；隔离5546=200/旧9851177/schema2保持，没碰8788或其它工作树。

真实prepare单独verify-9bf6ecaf…-adk17kh_.log0600/331922bytes/SHA b1056a7cf52e1cd9886eab614288ee56db1b46ec3bd0ab2ca14e126ee4496689，实际全文读取197Node/预算UTC迁移历史分类器/全Go race/全部migration/legacy/routing/console/full-console阶段及三视口summary visual和最终Chromium探针绿，CLEAN_END0/源不变。Owner prepare在ready复用分支9秒结束，不造第二次verify绿。此为controller标准升级后真实正式入口成功集成，旧cd413b5失败log字节保持；未实测两个正式失败或强杀断电持久性。

activation stagecommitted，正式回滚backup113152bytes/SHA与journal匹配，master.key摘要与升级前相同。只读旧schema2备份与current schema3逐表旧列行比较：admin1→1全列相同（含凭据行），app_keys/requests/upstream/models/route_candidates/upstream_checks/request_attempts/request_route_decisions均原0→0相同；不把空表比较冒称非空业务升级全面验收。曾尝试内存deserialize读取备份遇SQLite WAL格式unable to open，未成功写任何live数据；改用私有0600暂存原备份字节、immutable只读旧副本及mode=ro/query_only当前库，确认后删除私有DB副本，仅保存计数/相等布尔，未导出密码/密钥或修改数据库。此证明既有行/凭据存储保持，不替实际账号登录功能或复杂非空迁移。

CUA getState两次30秒timeout/kernel reset，停止重试。已安装npx/PlaywrightCLI受支持入口打开真实5545/admin，title Model Relay管理工作台；显示账号admin/登录表单，未提交旧密码/重置数据。401/api/session属于独立未登录context；favicon404保留观察，未误当核心部署失败。保存ignored output/playwright/model-relay-9bf6ecaf-login.png后关闭独立会话；实际用户已登录后的管理操作仍Not Run，不能用部署health或受控hostUI替代。

长期阶段2正式新Issue30“修复模型保存后的页面刷新完成同步契约”已创建，链接 https://github.com/big91987/model-relay/issues/30 ，当前main9bf6ecaf，原materialdesign-v0.2.0不改。输入要求R28-04真实GEThold/release最小归因、同scope post-write DOM完成屏障、双向启停/窄屏/读失败/迟到scope及错误状态负例，原固定门禁/独立QA/精确新发布保持；不由协调者代写产品或恢复完成Run28，旧未完成验收Issue仅历史不复刻。自动Issue入站Actions37842267940 success，唯一新Run2574d6be275f9eb630f8738837507e1e/workspacegithub-issue-5768698365从新main：prepare1 exit0/issue2确认原30，然后intake3/conversation8b1f6454489fabc4f4633af3a7548883 serverOverloaded/Selected model is at capacity失败，原始错误保存，尚无产品副作用。

Owner一次正式resume(seq3/message保持原Codex/gpt-6.1-sol/原任务)HTTP202，fresh回读Run running/error空/intake3running，同conversation，原前两步/definition逐值相等/max100保持。没有额外dispatch/新Run/换模型/临时权限/预算增加；容量恢复是否实际完成由新原生消息/结果后续核，不将running排程单独称故障根治。当前下一动作核新intake真实承接→最小原层回归研发，不永远停在旧完成Run；若外部容量再失败保留边界有界恢复，不循环盲试。

最后fresh回读：intake3同会话已真实完成回复并正式handoff development4/b106c858ccb3f8b0ca53d308648e099e running。核已部署main、干净workspace/材料一致，无新需求/架构决策，沿Bug短路径直接研发；旧同步假设仍待真实hold/release。证明本次serverOverloaded有界resume已在原生实际完成接续，不只排程running；未来容量稳定性不扩大。


### 2026-10-09 05:40 Issue30研发容量失败与同会话恢复

实时API：Run failed/seq4 development failed，conversation b106c858ccb3f8b0ca53d308648e099e updated2026-10-08T21:03:19Z，serverOverloaded/Selected model is at capacity。原生progress已记录静态诊断切片、监听限制与尚无实际Chromium结论；实际工作树新增model-save-readback.mjs/test.mjs，journeys挂接和workspace-contracts及本Run文档，没有产品workspace.js改动或commit/push/新PR。不能把静态同步假设当原历史18实证归因。

native-verify-04-state.json源149件a47fd28fc07b82e7fc9f2f15d610084c864f5cddf678c4448956e18cf19c58cf，起止20:57:56.730301Z–20:58:24.150854Z，原make verify冷副本；实际完整日志148817bytes/SHA071064bd7defc9013482d0c637461c3608184bbdd599e2c7bb9f099bec4adec4与state重核一致。historical HTTP rejected: historical listener unavailable; not business red，gate_exit2/exit1、自有groups absent/candidate removed、source unchanged。不是宿主tests Connector完成或浏览器早返回证据；诊断目标尚未运行，不伪报红绿。

Owner fresh确认同seq4容量失败后一次POST resume HTTP202，沿同会话/原任务续归档与正式handoff；前3步及definition逐值不变，预算100/模型配置保持。实际新API running/error空仅排程状态，后续须检查原生真实新消息和handoff，不把受理等同恢复完成，不循环追加resume。具体私有API/恢复回执见ignored .data/fresh-8793/check-20261009-0540。GitHub Issue30仍Open/Open PR为空；actual5545health200/version9bf6ecaf/schema3，5546旧9851177/schema2健康；新版本未发布，旧Run28不复活。


同次fresh回读：恢复input17494已running，原生新progress17495/parent17494于21:43:56Z明确承接“核对现有诊断与失败回执、补归档和源码戳，不重放写入/重跑完整门禁”。这证明同会话恢复后原生实际接续，仍未证明最终handoff或宿主目标复现完成。


### 2026-10-09 06:10 Issue30修前真实复现与自动返工

恢复input17494 completed，原生development4归档35项局部契约结果与源码戳后正式handoff tests，reply17603已确认；不再只凭running声称恢复。正式tests5 completed/failed、Connector exit2/329434bytes/truncated=false，维护者output offset0至327680，末next_offset329434/eoftrue共11页，全文SHA3685f0d819987c7358f138f8ac246ebb860da6d39f49845c03619445f9518a10。stdout首verification_receipt对应test-results/verification/20261008T214711Z-2559882295，不混其后的故障夹具attempt目录。实际runner329252bytes/SHAa5a3c4b6d12086c04108c7c474a9484232414e27051522fd303072ffcae3ab59和state重算相符、归档host-tests-05文件与原文件逐字节一致；21:47:11.167634Z→21:57:26.682523Z，gate_exit2/exit1，冷node_modules/npm cache空/NODE_PATH unset/无预装，149件ba8816e8f4cb38bb3b6c782195da2324bd16964f98cc2ff00448d91e78940d6f源前后相同、自有全部groups absent/candidate removed。旧a47是此前native诊断版本，交接规范材料变动后正式5源戳不同，不混版本。

实际migration13.170s/legacy88.296s/routing122.141s/console15.105s均oktrue；full-console2.210s okfalse，但目标probe已完整四case。1280/390各false→true与true→false，在真实PATCH200后对同scope tenants/catalog GET真实route.fetch/原样response hold/release：tenants held时dialog已关、loadingfalse、save_completedtrue、严格单行状态旧值；catalog held时loadingtrue、save_completedtrue、row_count0。每次释放后最终状态严格匹配/cleanuptrue。四case JSON无body/header/秘密，宿主受控HTTP非真实供应商；最终重抛首个原错误导致主exit2，failure总述stage最后enable不替第一具体case证据。机制实证保存驱动缺同次刷新完成屏障及短暂旧事实呈现，不证明持久数据错/P1或旧18历史精确根因已定位。

自动返development6/f6164fa7fc573c1e4d7fab6eb8dfe769 running，150件candidate47a199b482ec7f9ab1a815401fda902a6b5d3d57f917e068c1049ceab7374b74，产品Agent改现有edit/busy/readback、syncTenants epoch及renderPage明确DOM结果，驱动关联既有operation并保留严格断言；增加真实双向/窄屏读回、读失败/迟到scope相反写及局部生命周期回归。独立只读复审进行中，没有新宿主完整绿/独立QA/PR/合并/部署。当前health5545仍9bf6ecaf/schema3健康，5546旧9851177/schema2保持；平台维护源仅进度证据，本轮无源码或安装升级。


同轮候选材料变动后candidate-06-after为150件d187194806f62af8628b4f68216a509ec0d6240fa0474ddc71aaf6475cc54c89；47a199为中途版本，不混成新宿主通过。development6仍running、归档及handoff待，既有首红自动返工保持。


同次独立只读复审完成：22:14:02Z按tests/evidence库存算法重算150件0384f115adbbd545e8ce01021f238010ba66c89eb06e6acea08ce872ce0fa474，22:14:13Z candidate-06-after同戳；47a199/d187均为归档中途，重点6代码/spec SHA两次读取完全一致。复用既有operation/epoch与busy、读失败独立提示、旧epoch不释放新锁、secret/invite分离、严格断言及错误身份保持，未见新增Critical/Important。reviewer未运行测试，静态结果仅允许下一正式门禁，不等于新宿主/QA绿或可发布。最终fresh研发6仍running；下一tests需以实际冻结源戳核验，不预报handoff。


### 2026-10-09 06:40 新候选完整冷绿与独立QA正式交接

Run当前running/error空/max100。development6 completed/正式handoff tests7；Connector7 completed/next/exit0/337132bytes/truncated=false，维护者output API十一页offset0至327680，末337132EOF，全文SHA3442bc0351e146435f97ab7aea4c5e7f11dbeba04ecd8709b91d44819d3406f0。按stdout第一主attempt test-results/verification/20261008T221951Z-283933941定位，不混其后cancel/故障夹具预期红。实际22:19:51.041183Z–22:30:55.912316Z，runner337036bytes/SHA9197fff5ef1e77fd658f1a5a4c2a9e89b33598808f33e8708919967cee457c3f与state独立重算相符；150件0384f115adbbd545e8ce01021f238010ba66c89eb06e6acea08ce872ce0fa474源/副本前后相同、冷node_modules/npm cache空/NODE_PATH unset/manualpreinstallnone、gate_exit0/exit0、主及全部辅助ownedgroups absent/candidate removed。最终dependency receipt解析candidate/node_modules的Playwright/core1.59.1、Chrome147.0.7727.15。维护者按tests/evidence库存独立重算150件同SHA，未代跑产品测试。

实际migration13.230s/legacy88.280s/routing122.419s/console15.124s/full-console50.754s全oktrue，213Node、全包race/count1/parallel4/真实HTTP/build、历史红分类器/零软预算/有效UTC/迁移及三视口1280/1440/390 J01–07/summary/visual通过。三probe JSON各7case：每套4组启停及窄屏×2真实tenants/catalog hold均save_completedfalse/content_loadingtrue/readbackpending/locktrue，释放后最终严格DOM全部正确；每套2组读故障先真实GET200后受控转发503，保存严格拒绝、重试读取恢复，不冒称真实服务天然503；每套1组切scope后第二次相反写、旧200晚到不覆盖新结果。合计12保存组24暂停阶段、6故障恢复和3迟到相反操作均cleanuptrue；Chromium147实际版本记录。QA归档3个JSON和state/log与原attempt/browser逐字节一致；不在源码根误找临时已隔离browser产物。

QA8/850f0b1bf32914d2273f5382c658d5a5 closed/正式next/handoff report9，reply18113及result.qa_decision明确Go with known issues仅精确候选。独立75Node契约、材料19件摘要、源码前后/实际回执/三张安全模型页截图检查；维护者核26个archive-index具体产物bytes/SHA/原文件全相等。已证R30-SYNC-01完成契约缺口Closed，但旧R28-04 Open/P2精确根因仍未知，不以新成功倒推旧失败已根治。顶层ClockRecoveryWorker明确skip/NotRun，父InProcess及ActualHTTP子进程CLOCK_RECOVERY_PASS为实际同轮恢复门槛，不把skip改Pass。native独立动态QA Blocked，供应商/支付/身份/Owner登录预览/新正式部署NotRun分列；没有扩大权限或虚造用户风险接受。

当前report9/4798281a4474b1226cc842adc454736a running；GitHub Open PR空，尚无新提交/合并/部署。旧5545health200/version9bf6ecaf/schema3、5546旧9851177/schema2健康，不冒称当前main已含修复。等待Pipeline原publish/pr产生精确提交，与QA/host150件逐git对象核对后授权merge/Owner正式Actions/Deployment及实际health页面；任何新红或源不同须No-Go，不循环全套求绿。平台本轮仅三份进度证据，无代码/配置升级。


### 2026-10-09 07:10 PR31精确已验提交合并并正式部署

实时Run completed/error空/max100/12步，report9完成后publish10 Connector exit0/124bytes/truncatedfalse、pr11实际URL https://github.com/big91987/model-relay/pull/31 、done12；PR已attach当前chat。GitHub新head8c8b155c007686311706ee1f8cec8262a1258bb9/base main9bf6ecaf、Draft、MERGEABLE/CLEAN，GitGuardian Security Checks SUCCESS。维护者git ls-tree/cat-file --batch按tests/evidence相同库存对150blob重算0384f115adbbd545e8ce01021f238010ba66c89eb06e6acea08ce872ce0fa474，每件git对象与工作树字节相同；26QA归档对象bytes/SHA/原文件全部匹配，QA报告/矩阵/缺陷/pr.md发布对象字节一致、工作树干净。远端GitHub head tree不截断，15新增任务分支完整URL目标均非空；旧评估链接为原已存在基线。不用当前main旧HEAD当候选。

按用户测试仓合并授权ready31、PUT merge明确sha8c8b155…，返回mergedtrue/main3f9cee1615e87e002a2c2300f2d29778072ec3f5。未自行创建Platform PR或合源main。现PR/Issue关闭仅表示版本Git发布事实，未宣布部署/页面全面验收。正式push prepare37858024406/23:11:31Z in_progress，Owner375348269 dispatch deploytrue/targetpreview/main37858041523/23:11:42Z pending，同main3f9cee；原入口已触发一次，不重复dispatch或协调者代跑make/activate。

实际controller源码安装仍2d456ca50265cdd6db89ee4b0f3d0e85695bec45adff7a2b2665c826d48fab79，prepare已建verify-3f9cee1615e87e002a2c2300f2d29778072ec3f5-jt63zgxe.log/mode600/当时155654bytes，是独立实际attempt但尚无完整成功/激活证据；旧日志不作新attempt。health5545仍200/9bf6ecaf/schema3，5546旧9851177/schema2健康，未碰8788。后续需实际正式Actions完整log/ready或reuse、Deployment及新health/version/schema3/页面一致，不把运行中局部记录当完成；任何新红保留唯一首日志再最小实证，不连续盲重试。旧R28-04未知根因与native动态QA Blocked/供应商Owner登录NotRun不因合并改Pass。当前私有证据ignored check-20261009-0710，原失败/QA/冷验历史保持。


合并树补充复核：PR head与main合并提交的真实commit.tree.sha均4d16c018fda3d7ab59ad81bf93842a0a12260c84。初次将GitHub git/trees端点以commit alias请求返回的sha8c8b155…误与真实tree SHA比较，布尔false是不同字段口径，非源变更；立即改两个commit.tree.sha同口径核对true，证据merge-tree-check.json保留，不伪称曾发生源码不一致。


### 2026-10-09 07:40 PR31正式部署闭合与下一唯一任务

实时push prepare37858024406 completed/success（job113586815699/23:11:34–23:22:55Z）；Owner dispatch37858041523 completed/success，prepare113590348783/23:22:59–23:23:08Z为ready复用，deploy113590410939/23:23:13–23:23:22Z成功。Deployment6949052209/sha3f9cee1615e87e002a2c2300f2d29778072ec3f5/local-preview于23:23:22Z success，实际environment_url http://127.0.0.1:5545/admin/。新5545 health200/statusok/schema3/version3f9cee，旧5546 health200/9851177/schema2保持；未碰8788。实际/admin/ title为Model Relay · 管理工作台，workspace.js与PR已验对象逐字节相同/SHAe1bba2c41135506387873f2f7e080d5a7b0bacdb49ce481ed0494e85a786a5b1。这是正式服务入口核验，不是已登录后实际操作验收。

标准controller安装源摘要2d456ca…一致，唯一verify-3f9cee1615e87e002a2c2300f2d29778072ec3f5-jt63zgxe.log mode0600/341351bytes/SHAac835a91c9c3324507ebba495eda08a739d91aff744b1af8c07b8079b9f30fab。完整原make verify、213Node/Go/全部主BROWSER_PHASE oktrue、冷主CLEAN_END gate_exit0/source_unchangedtrue/候选Playwright-core1.59.1+Chromium147；故障夹具预期exit23不混主验收失败。migration13.187s/legacy88.273s/routing122.168s/console15.115s/full-console51.207s，本次实际绿支持正式activate；ready复用不假称第二次门禁。旧失败日志保持，尚非两个正式失败attempt/强杀断电持久性验证。

activation.json stage committed；本次rollback363008bytes/digest匹配、master.key SHA与journal一致。正式备份的relay.db仅复制到0600私有临时文件immutable只读，与live mode=ro/query_only全表比对后删除；admin/users/tenants/memberships/resource_limits及meta非空行相同，其余原空业务表仍空。console_clock effective由20:44:15.355978Z推进至23:41:54.515903Z，源码advanceClock的单调运行水位，与备份不等属实际运行事实，不手改；不宣称整库全行一致或非空业务全迁移/密码登录实测。只输出布尔/计数及非秘密时钟，不输出凭据值。证据ignored check-20261009-0740/{upgrade-preservation,verify-log-metadata,health5545,health5546,prepare,deploy,deployment-statuses}.json。

正式新Issue32已创建，当前Open旧20–24没有重复该P3任务，旧Run30/28/25均completed未恢复。自动Actions37861181092（23:45:24Z）入站，API确认唯一Runb7f93c24bc1ea4d0fe182f12aa7622ae/23:45:34Z，prepare1与issue2成功，intake3/dc330734fa1ba6376edb70c44b47ca57 running/error空。新输入沿冻结v0.2.0、main3f9cee、QA58-01服务success/技术枚举/窄列断词及首次self_hosted UI NotRun，要求最小现有UI改进和原固定门禁完整真实旅程，产品由Pipeline实现，不重复PR/dispatch或代产品。实际原生承接内容/后续handoff待回读，不以running当开发完成。动态独立QA Blocked、预览已登录操作和供应商/支付/身份NotRun及旧R28-04未知根因保持。

本轮初并行读取因证据目录尚未创建，3个GH命令重定向先失败而未发网络请求；目录建立后顺序读取成功。这是协调读取依赖错误，非GitHub/部署失败，没有重复部署副作用。


本轮最终回读：入站Actions37861181092 completed/success；新Run32仍intake3 running/error空/max100/workspace github-issue-5770505233。automation_update正式回读ACTIVE/原30分钟频率、新Issue32/Run锚点生效；未重复启动或扩大预算。


### 2026-10-09 08:10 Issue32新候选与正式宿主验证运行中

API实时Runb7f93c24bc1ea4d0fe182f12aa7622ae running/error空/max100；prepare1/issue2/intake3/development4 completed，tests5 dispatched/running/00:08:41.982124Z。intake按实存源码、冻结material/archive SHA和原PRD/交互/AC摘要完成route development，未伪造新requirements/design。development正式next包含11代码/spec路径及新delivery文件级产物，T001 In Review/M01 Ready for Review不是Accepted。维护者实际VCS/源码清单与同算法库存独立复算152件d78781505e2ca5bfd3fe5d357ea3379cb9a414d7cd127ea0248c2491bc633180，与source-final/handoff一致，源码已实现不等于宿主/QA绿。

本候选为上下文状态中文、未知原值、原生键盘完整标识/表格横滚和三视口独立self_hosted安装角色/UI创建核验/模型价格授权/Key/普通SSE/同ID费用及TCP关闭/401纠正恢复；原R30 ownership/严格summary负例、共享旅程、费用与权限口径保持待完整检验。研发自报86Node、非监听Go race/静态检查属于局部证据，不当新完整门禁。实际native-verify.log156959bytes/SHA cfc8c1149f39c3ef4df254ea1738ace98b7dd44231ff4e76f0c6444f011168fd与state逐字节/长度重算吻合；原生attempt23:56:58Z–23:57:25Z/旧152件6ba7643…前后不变/gate_exit2，review-history historical listener unavailable/not business red，清理owned group absent/candidate removed。它是归档前受限尝试，不是最终d787候选完整验收，也非目标UI红；不反复原生求绿。

正式tests5实际唯一主attempt20261009T000843Z-3220031292/00:08:43.023306Z开始；冷起点node_modules/npm cache不存在/NODE_PATH unset/manualpreinstallnone，152件同d787，前置辅助exit0/owned group absent。00:11:19Z读取日志218996bytes仍Go阶段，finished_utc空；state source_unchanged=false/candidate_unchanged=false/gate_exit-1/exit1/log_bytes0/cleanupnot_started为尚未结束初始值，不拿来断言失败/泄露/源码改动，也不当清理通过。结束后必须通过正式output完整EOF和最终state重算源前后、日志SHA/bytes、全部旧新门禁及清理；若新增旅程使真实时限失败，以阶段实际证据最小归因，不先增限/删断言。

独立只读代码复审已发给既有release_review_27，待结论；reviewer不编辑/代跑。GitHub Open PR空，Issue32原生启动/实际handoff评论与API一致，5545health200/version3f9cee/schema3，不误当新改动已部署。QA58-01、self_hosted网页分支尚未验收关闭；旧R28-04未知历史根因、动态独立QA Blocked/Owner预览登录/供应商支付身份NotRun保持。私有证据ignored check-20261009-0810，未改DB/回执/冻结图/权限/预算或旧Run。


本轮独立只读复审完成（00:12:02Z）：同152件d787815…，未发现新增Critical/Important/Minor明确缺陷；没有代跑测试或编辑产品。正式tests5仍running/error空、完整回执和QA待；原automation_update回读ACTIVE/30分钟/最新候选及tests锚点生效，不增加预算/权限或重复任务。


### 2026-10-09 专用产品端到端测试 Harness：标准安装、权限保持与真实失败恢复

用户要求新增【产品测试】，并澄清Builder是测试平台与Harness模板时的角色：由Pipeline执行具体产品验证，协调者以真实任务核验流水线能力、可靠性与效果，不代做用例掩盖缺口；其他任务职责按用户授权执行。维护源6d1c57b新增独立product-e2e模板/四角色及network/elevation显式授予/省略升级保留/明确撤销、现有Playwright Skill。图无产品研发/发布PR节点：prepare→issue→plan→execute测试准备→tests固定Python→review成功或失败均进入→report→done，test_repair仅设施。报告结束不自动写产品Pass。固定1800秒宿主Connector与原生network/granular permission approval分开，未改平台引擎/核内沙箱，不开放全机无限权限。

新权限参数/新模板先实际缺参数红，源实现后模板回归绿。独立review发现浏览器Skill参数省略升级移除P2，新增两次安装无writes/clear撤销用例在旧实现真实红后最小修复，独立只读复审关闭无新Critical/Important。完整标准PYTHONPATH=sdk/python Python模板/仓库/API测试75项6.467s/exit0及ruff/diff通过；随后参数null修复后76项7.496s/exit0。最初完整测试未设置文档规定PYTHONPATH而SDK import失败，正确标准入口重跑通过，属Builder执行命令错误，不当平台/产品故障；新增测试插入缩进错误已修，首语法错误不冒充目标红。

标准install.py使用独立prefix model-relay-e2e、独立manifest/workspace root，安装Workflow3155ddd1206d4ebc19da94e5b489091f；实际API GET四原生Agent Codex/gpt-6.1-sol、network_access/allow_elevation true，execute/review Playwright Skill实际存在。浏览器/联网真实操作尚待Pipeline，不以配置字段判实操Pass。再次正式--upgrade省略授权及browser-skill，同Workflow ID/revision/完整配置保持；旧软件研发8f497…revision2/network与elevation均false、没有重复研发Run，定时状态PAUSED保持。标准新装/升级证据ignored product-e2e/{installed-workflow,standard-upgrade,isolation-check}。

通过正式Owner Run API request_id model-relay:product-e2e:deepseek-user-journey:20261009创建唯一Run c41e8047001414b96d327c39fb9c1eeb。首次prepare1失败完整output EOF：materials.install_material对parameters:null使用dict.get抛AttributeError，原准备分支已创建，未进产品测试或创建Issue。源materials.py install/verify两个guard改(run.get(parameters) or {})，实际None无材料/无文件回归旧红→绿，独立复审无新增重要问题；提交28b6469后原manifest标准upgrade应用（路径加载修复源码，无需换图/engine）。Builder首次stop未传seq得到409、未变状态，核正式API契约后携带seq1停止；真实stopped后return(seq1,targetprepare,原因)HTTP202进入seq2，冻结definition/工作区逐值保持，原首失败+正式返工原因保留，未新Run或补造回执。prepare2真实exit0后issue3创建唯一 https://github.com/big91987/model-relay/issues/33 ，标题【产品测试】验证真实DeepSeek下用户、模型授权、Key配额与调用诊断；GitHub实际title/平台marker匹配、无凭据，API核默认研发入口无重复Run。

e2e_plan4/c81a84d7933adfb6592450d5f68d0fc4 running/error空；新Run总预算30有界，测试生成请求跨attempt最多24次/每次输出64/90秒，具体计数及用例由Pipeline落实。用户提供的密钥仅存忽略目录0600私有配置，Task/Issue只含引用、端点和模型名，不输出值。Builder未操作产品网页、调用真实Provider、代写测试脚本或给产品Go/No-Go；后续测试Agent亲自浏览器/固定宿主/独立实操证据分别保留。此轮证明安装/授权保持、真实准备红→源修→标准恢复绿、自动Issue及原生接单，未宣称整条E2E及所有权限已实操完成。


### 2026-10-09 测试报告→development修复交接要求

已更新维护源AGENTS、e2e_report、intake及README：已复核产品缺陷才立【产品修复】，交报告、首红与同版脱敏产物，既有任务去重，沿原SHA材料入口和研发prepare→issue→intake→development；固定测试、独立QA/交付保留。不实现新分类引擎，不略过准备，不手改Run。产品判断仍由测试Pipeline形成，Builder只组织既有事实的正式交接。

模板图与标准安装配置回归17项/0.166s/exit0，git diff --check通过。该回归证明原路由及安装接口没有破坏，不证明新报告内容或实际修复接单。当前正式message20418 request_id model-relay:product-e2e:repair-development-handoff:20261009 duplicate=false，回读原生input running/approval0；原Run seq4/e2e_plan running/error空，暂无完整产品报告或可据此创建的问题。未创建修复Issue、上传材料或启动第二Run。

新的周期完成跟进请求被自动审批拒绝，原因是原用户已停止定时且仅授权一次交接；工具isError/拒绝回执明确，没有新定时任务创建，旧model-relay仍PAUSED。不借其他工具绕过；本次完成后自动跟进是否允许已向用户说明并待明确选择。真实产品完成报告→材料SHA核验→唯一修复Issue→development原生接单仍待验证。
