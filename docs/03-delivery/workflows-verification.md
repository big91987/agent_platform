# 平台内 Workflow 验证记录

基线：2026-10-05，分支 codex/platform-workflows，起点 3f3b2c8。

当前结果：T1 编辑器与 T2 平台内 Agent／人工反馈循环已通过真实切片；命令与 GitHub Issue／评论 Connector 已通过真实切片；独立测试仓库完整正常交付已通过，QA 真实缺陷返工闭环也已通过，发布验收继续执行。不得引用原 GitHub CI 方案的测试结果证明本方案通过。

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
