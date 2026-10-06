# 平台内 Workflow 验证记录

基线：2026-10-05，分支 codex/platform-workflows，起点 3f3b2c8。

当前结果：**Pass（本轮 GitHub Issue → 研发 → 草稿 PR 端到端验收）**。通用 SDK/API、自动接单、原 Issue 绑定与评论接续、bug 短路径、新需求分派、真实项目测试、独立 QA 及返工、停止／恢复／重启和去重均有下方真实证据。此前遗漏 Issue 入站而外推的整体 Go 结论不沿用；本轮按用户要求不新增部署和 code review 验收。源仓库 PR 仍 Draft/Open、未合并；真实供应商及其它聊天渠道未测／未接入。

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
