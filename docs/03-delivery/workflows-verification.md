# 平台内 Workflow 验证记录

基线：2026-10-05，分支 codex/platform-workflows，起点 3f3b2c8。

当前结果：方案二平替的完整用户旅程 **No-Go**，原因是人工合并后的部署与效果地址尚未通过；见文末纠偏。T1 编辑器、T2 Agent／人工反馈循环、命令与 GitHub Connector、截至草稿 PR 的正常交付及 QA 返工分别已有真实切片证据。不得把这些切片合并称为完整替代验收通过，也不得引用原 GitHub CI 方案的测试结果证明本方案通过。

| 验证范围 | 状态 | 证据 |
|---|---|---|
| 图编辑、保存、授权、版本快照（WF-01/09/10） | 部分通过 | 定义 API、权限与网页编辑通过；T2 用版本变更测试验证运行冻结旧图 |
| 真实 Agent 接力、人工确认与回退（WF-02～05/12） | 通过已列旅程 | T2 人工回退；T3 正常研发交付及 QA 缺陷返工，同一 Run 完成修复复验 |
| GitHub Connector 与新仓库闭环（WF-06） | 正常路径通过 | T3 真实命令、Issue、评论、草稿 PR 和研发模板已通过；GitHub 丢响应恢复仍待实测 |
| 停止、重复交接、重启及升级（WF-07～11） | 部分通过 | 原生停止／恢复与人工等待重启通过；幂等和旧回执用真实 SQLite／MCP 测试；完整升级与外部未知结果待 T4 |

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

用户确认验收目标是完整研发旅程：从一句话任务自动进入合适阶段，经过真实开发、测试和独立 QA，交付供用户验证的 PR；用户确认后自行合并，合并触发项目部署，最后能打开稳定地址查看实际版本。上方“完整旅程终验”的 Pass 只证明截至草稿 PR 的编排切片及其特殊路由，**不证明方案二平替通过**。此前对用户称“这轮平台能力验收已通过”扩大了结论，现更正为 **No-Go**。

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

平台交付回链补验：源提交 `88b5a8f` 增加单次 Run 的只读 GitHub 交付查询。仅对该 Run 已保存的草稿 PR 回执及仍有权限的当前 Connector 查询 PR；合并后才按准确 merge SHA 查询 GitHub Deployments 和状态，效果 URL 只在部署状态 `success` 时展示。若 PR head 在原 Run 后变化，页面提醒原 QA 不覆盖新提交。后端测试覆盖未创建 PR、跨用户拒绝、草稿更新及合并后的准确部署关联；前端测试覆盖未合并时不显示效果地址与不安全 URL 过滤，全仓 `scripts/verify.sh` 退出 0。升级前逐个工作流 Runs 页面核对 0 条在途、1 条按计划停止，M1 唯一 Run 已完成；私有备份后仅升级 8792，8788 未更改。升级后从真实浏览器进入 M1 Run，页面展示 PR #2、GitHub Actions 入口、`PR open · Draft，尚未合并；没有部署版本` 以及 `PR 在本次 Run 的 QA 后更新`；只读 GitHub 查询与页面一致。合并／部署状态因未发生仍未验收，不能用该空状态页充当成功发布证据。
