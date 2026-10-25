# OpenSandbox 工作流改造实施计划

## 当前进度

更新：2026-10-25。112:8795平台实际版本 `d9a9564050d825b02c3ae00e2364d94b94828681`、正式Workflow `83f08f81f7040fcd0c12e6ff4e2acd2d` revision11。定时任务保持暂停；本轮由用户直接授权试验真实产品功能，从仓库Issue到功能PR，评估耗时/质量并留存QA证据。

当前任务 [Issue63：同模型上游低延迟优先路由与选择诊断](https://github.com/big91987/model-relay/issues/63)，2026-10-25T11:14:20Z创建（北京时间19:14:20）。先按用户此前明确的低延迟方向推进，用户若另定功能则正式调整；需求/设计/预注册测试及性能对照由Pipeline形成并wait_for_input检查，再实现。固定make verify、独立QA关键行为实操、正式确认、报告和同版draft PR闭合；业务PR不自动合并部署。QA报告/真实命令、截图、样本、缺陷和返工记录需归档，释放后再全新下载核验，不能只有QA通过摘要。

测试设施基线PR62 head0525d83245e38207cd417917e0a13917c9403581已按既有测试仓Harness构建合并授权设ready并精确合main `f06f05de20ffac5b8c8e772958c808379001f0e3`，安全检查成功；旧PR49/55不合。上一轮Issue60/Run91bc74已12步completed/released，139件留存及完整测试/QA证据保持，不当作新功能通过。平台源无PR/main合并、未发起Owner部署。

原opened入口Actions40188019710已success，正式日志与API均给出唯一Run `ba434da8039cca16f6829481112e3d68`，11:17:27.847Z创建，冻结main f06f05de/revision11。Issue→Run187.848秒（包含Runner队列）；prepare11:17:27.847Z–11:19:02.534Z真实exit0/94.686秒；development2/conversation03d91bdea8a1b9570ced063ac3db5375已产出v1计划并于11:26:21Z正式wait_for_input，原生结束/停止后waiting。Builder只读核计划原件后通过正式User Input16636要求修订SSE指标/样本年龄和接近总预算的实验编排；11:30:48Z同一会话已实际承接。尚不批准产品实现，未运行实验或固定门禁，暂无待审批；这计为设计评审返工，不是产品测试失败。

旧Issue51/Run6f3f26…仍waiting/seq2，快照environment.ready，但正式文件读取已返回OpenSandbox lookup404。停止该旧Run的动作被自动审批以缺少明确授权/可能影响接续拒绝，未执行；已放弃停止并保留旧记录。本轮用独立新Issue/环境关联接续，不偷偷重建、恢复或改库；旧等待TTL状态缺口仍未修。新任务不携带历史真实供应商密钥，真实供应商和生产性能与受控实验分开。

## 历史记录

以下保留各日期的原始证据；其中“当前/最新”指记录当时，实时现状以本页上方当前进度为准。

当前完成（2026-10-24）：正式工作流的研发、独立QA和交付报告三个Agent已采用分节Markdown提示词，职责、步骤、产物、交接和阻塞处理可逐项阅读。维护源3c0a601通过模板回归并标准导出151对象，测试仓模板PR58精确合并main12df9eb7；112正式配置已升revision10，完整预览与保存后重新读取均同源，六条既有Run保持。平台二进制仍ffcd93d，未启动新产品Run或改变权限；定时任务继续暂停。

当前完成（2026-10-24）：Agent 节点“输入预览”现展示完整的平台系统提示词（原生开发者指令、角色指令和平台追加约束），工作流用户消息单独展示。维护源 ffcd93d 已通过标准完整门禁、原生 start/resume 逐字核对和真实 Chromium，标准部署到112:8795；正式预览API核验3个Agent角色完整、草稿不落库，线上JS摘要与维护源一致。主图保持revision9、历史Run和产物/私有配置保持；定时任务仍暂停。下文旧状态保留为历史。

当前完成（2026-10-23）：按交接目标独立参数已正式部署112:8795/6c7f960，主图83f08f81f7040fcd0c12e6ff4e2acd2d/revision9。相同定义的代码参数、Agent另一套参数、无参数Agent三条真实Codex分支全部完成，7次原生handoff和8件产物核验通过；3环境已释放，产物仍可下载，临时图已删除、Run历史保留。标准模板PR57已精确合并main91a50516。完整源门禁与独立复审保持，未把Harness验证当产品或供应商验收；产品PR49/55未合部署，定时任务仍暂停。下文“待连接/待部署”为历史。


当前增量（2026-10-22）：按交接目标分别配置参数已实现并通过最终完整源码门禁与独立复审。节点 handoff_parameters 按 target 保存开关/Schema/说明；同一 handoff 工具按实际目标校验，界面按目标卡片配置，应用/取消与草稿保留。112 HTTP仍为fdedb0a/revision8；部署SSH被远端断开，直连22超时，当前未升级，也未启动本轮三分支原生验证，不能用前轮Run替代。已请求用户恢复ds112连接；标准部署/正式主图升级/原生验证/释放后留存/临时图清理待续。定时任务保持暂停。


当前状态（2026-10-22）：112:8795已通过标准Linux构建/安装升级至fdedb0a，正式工作流83f08f81f7040fcd0c12e6ff4e2acd2d/revision8，参数默认关闭。可选params、填写说明及Schema校验、节点应用/取消和执行配置留存已交付。真实Codex参数验证Run18a76ca18b9390ef2cb2c80c15e9ad67已4步completed，Agent→执行→独立Agent→结束通过，归档3文件在环境正式释放后仍可下载；临时工作流已删除，Run历史保留。测试仓标准模板PR56已精确合并main4eade601；产品PR49/55仍未合并部署。定时任务保持暂停；下文早期状态按历史保留。


当前推进（2026-10-15）：用户确认进入真实功能开发验证，已创建 [Issue51：同模型上游低延迟优先路由](https://github.com/big91987/model-relay/issues/51)。范围是同模型合格候选间依据近期响应表现选路，并与现有权重策略做效果对照；保留权限、优先级、限额、预算和SSE/取消边界。产品规划、设计、实现、固定测试和独立QA由Pipeline承担，Builder只组织受支持入口、检查证据与平台可靠性。第一轮先产出计划并正式wait_for_input检查，再实现；不做人为缺章/返工。当前main cb2cb1e89b3f765f56a83bbbe5ed3c1b02041f99，112实际8ad9394，Workflow83f08f81f7040fcd0c12e6ff4e2acd2d/revision4。Issue请求github:big91987/model-relay:issue:5857969219；Actions38661019216/38661019331已自动触发，两次均因入口仅接受Harness验证标题而拒绝；维护源80df20d已修复并标准导出，133模板与11导出entry回归/独立复审通过，Harness PR52已精确合main84c000f8945d7532043f7aedd29e62dc4e86b866。原Issue正式dispatch38662167083已受理，目前queued，无Run；同标签he-full的预览prepare38662139638正在model-relay-local执行固定验证，不重复提交或取消无关任务。原小时检查已恢复，仅重要变化通知。M1保持完成；本轮止于真实功能同版草稿PR、效果和瓶颈报告，不自动合并/部署产品。

历史里程碑状态（2026-10-14 21:30 UTC）：M1真实整链已完成。Issue48→唯一Run2b6a366cc7b3d48870e1b887757f229f（15步completed）→Pipeline自行发布草稿PR49，head a033eb20ea848d3aead77231da367703b1b43fb8。两轮完整冷make verify、独立QA实际缺章拒绝/返工/全量复核、共享工作区、原生gpt-6.1-sol角色交接和正式操作者确认均有原回执。GitHub 163源码991c79ca…、85交付文件、QA/正文/首拒绝及146冻结模板逐字节核验通过；86件最终本地归档4997769bytes。环境正式release请求响应超时后只读核对已released，未重复释放；入口已标准恢复main。草稿PR49未合并/部署，不代表全部产品、真实供应商或全部可靠性场景已通过。通用发布修复源a20c59b已开发分支提交推送，测试仓标准升级PR50精确合main cb2cb1e89b3f765f56a83bbbe5ed3c1b02041f99，原manifest将未来图升revision4，仅报告约定改变、当前冻结Run/私有配置不变；112二进制仍8ad9394。下一阶段M2/M3/M4待用户确认，不再自动启动新任务。

日期：2026-10-13。状态：实施中，按已确认的外部沙箱方向继续；用户已授权在当前隔离 worktree 实现、部署到 112，并通过测试仓新 Issue 类型启动独立 Pipeline。当前分支 `codex/opensandbox-workflow`，起点 `65b4d3e`。

**Goal:** 在 112 提供隔离部署的 Agent Platform，画布以 Agent、执行、人工确认、结束四类节点构建流程，使用 OpenSandbox 执行任务，并以测试仓独立入口验证真实交付。

**Architecture:** 平台保存流程、权限、Run 和回执；OpenSandbox 管理 Run 执行环境。执行节点支持代码、命令和官方 SDK 的 MCP 工具调用；Agent 与工程步骤使用同一任务环境，角色会话独立。已有本机 Run 和旧 Connector 定义保持兼容，不把运行历史原地转换。

**Tech Stack:** Go 平台；原生 Codex；OpenSandbox Docker 后端；Python/TypeScript 任务运行时；MCP 官方 SDK；现有 GitHub 入站与报告发布工具。

**Spec:** `docs/01-product/software-factory-assessment.md` 第 0 节，加本节记录的用户后续确定项：四类节点；Agent 内部 Hook 保留原生职责，外部动作复用执行能力；外部工具统一 MCP；项目脚本不限制 Go；Example 复制入项目后按需调整；112 实际部署及独立新 Issue 入口是完成条件。

## 当前里程碑与长期计划（2026-10-14）

用户要求持续推进到首次真实完整Run及PR，再确认后续阶段。普通选择自主处理；产品动作交Pipeline Agent，平台缺口先标准修复升级。

| 里程碑 | 可验收结果 | 当前状态 / 下一步 |
|---|---|---|
| M1 与旧版等价或更好的完整交付 | GitHub Issue→唯一OpenSandbox Run→原生交接/固定测试/独立QA返工复核→正式确认→报告→同版草稿PR | 已完成：Issue48/Run15步、PR49真实draft及远端同版核验、保存后环境released。通用发布修复已标准交付PR50/未来图revision4；具体证据及限制见下文。 |
| M2 恢复与失败处理可靠 | 相同输入去重、停止后所属进程确实安静、原会话/共享文件接续、外部失败和未知结果有界核对、同版材料保存及在途升级 | 基础隔离回归和native09已有部分证据；项目整链对应场景随M1检验，系统覆盖在M1确认后推进，不把基础绿替项目证明 |
| M3 可复用接入与升级 | 另一仓按项目复制定制、自有Actions/脚本和标准安装升级获得同样行为，无平台checkout路径依赖或协调者救援 | Model Relay应用为第一仓；第二仓待M1确认后确定，不提前造重复模板或任务 |
| M4 易用性与抽象减法 | 团队能用四类节点设置必要参数；基于真实使用证据消除重复配置和一次性衔接 | Agent基础/高级及画布衔接仍待评估；保留可信扩展点，不新增环境Profile/多层对象框架 |

第一里程碑的“通过”必须有完整原始回执和独立QA实际结论；固定测试、流程完成、PR、合并及部署分别报告。以下为早期状态（已被当前结论更新）：当时运行平台afebd27；项目main模板已通过PR46更新至bc43314，新Issue48沿候选30b4e13c使用该项目自有模板/冻结图revision3。旧Issue44的revision2冻结历史保留且已取消释放；不能把新Run或模板升级冒称旧Run发布成功。

## 早期实施状态快照（2026-10-14，历史保留）

| 部分 | 当前证据 | 仍缺什么 |
|---|---|---|
| 执行节点 | 112正式API同Run Python/TypeScript/Go项目命令、完整输入/共享文件、去重、人工确认及环境释放已过；MCP隔离HTTP/实际页面回归通过，任务/前序结果引用源码及浏览器通过，a000d1d已标准部署 | 原生同角色模型/接续已过；测试仓真实Pipeline仍待 |
| 112 OpenSandbox | 1.1.1/8095；SDK文件/取消、正式角色home的同进程实际Skill范围/停止/删除404通过；8795标准升级后正式基础Run五步完成并释放，实际Chromium页面通过 | 外部原生命令已通过；代理配置后模型实操、同thread澄清接续及释放已过；跨角色/返工/重启及服务登出重启仍待 |
| 四类画布与112平台8795 | a000d1d标准Linux/CGO升级exit0，Mac/112 health及网页源码同SHA，旧Run/配置/凭据保持；check-04正式五步完成并释放。早期1440画布/390Run截图证据保留 | 原生远程角色会话及模板真实Pipeline；用户服务logout/reboot |
| 测试仓独立入口 | 仓库自有Harness标准导出/安装候选、入站回归已过；测试仓草稿PR38已标准同步bb698c9，head acd96f7与140件源库存一致，当前GitGuardian success；真实新入口尚未启用 | 项目适配/安装、新Issue及真实原生整链 |

112 基础设施探针 ID `a2548a85-d279-4b82-93e2-4991d11feaa8`，已删除；报告6事件、SHA256 `671c0e833897ad2882a4f5951b323469652a46a4632465830f93b9bb776cfbe5`。当次 execd 镜像摘要 `6cf7dba2f21f0b536e100563d841ac58a9f31c2b0a081b7ac76796a24d6f47e2`，Python镜像摘要 `a6e34c598f2467ed0e9a8d349809fcd8b5c603269512df273a0bb1784edc11b1`。服务目录0700/配置0600已实测。此为基础设施回归，不是产品验收。

## 全局约束

- 保留现有 8788/8793 与测试仓现有流水线；候选使用 112 独立目录、数据库及端口，初选平台 8795、OpenSandbox 8095，启动前实际核占用。容器端口由 OpenSandbox 分配，不抢占宿主业务端口。
- 平台只在开发分支提交/推送，不创建平台 PR、不合 main。测试仓必要 PR/合并及验证入口在既有授权内；Harness 验证 Issue 标题明确类别，新触发用独立 label/模板路由，与旧入口互斥。
- 通用实现、部署与安装路径归源仓；项目 YAML/脚本归测试仓。不存在平台 checkout 的新仓也能按文档接入。
- 不转移整个宿主 HOME/秘密到容器；使用受信、最小的凭据引用。Issue 输入不能授予宿主挂载或执行权限。
- 不手改数据库、原生历史、回执或冻结 Run；未知写入结果不盲重试，不以宿主代执行掩盖沙箱不通。
- 新执行节点使用同一 Run 环境；原生会话重启不能无声换 ID。正式结果和需人审的材料有持久归档。
- Model Relay 产品实现/验收由 Pipeline Agent 进行；维护者仅验证 Harness 能力和证据。

## Review Focus

1. MCP 服务在调用后断线：保留不确定结果，禁止自动重复写入；停用/修改连接后不能借旧 Run 偷用新身份。
2. OpenSandbox 创建成功但平台未收到响应：通过 Run 关联核对，不重复创建；环境丢失不回退宿主。
3. 阻塞或取消中的 Agent/代码：实际所属进程停止、回执保留，不能仅把 UI 状态改为 stopped。
4. 旧工作区路径、未知代码语言、跨目录路径或缺运行时：明确拒绝/诊断，不吞配置或扩大访问范围。
5. 重复 Issue 投递、页面重载、平台重启：配置不丢失、Run/环境去重、原生会话身份连续。

## Task 1：执行能力与 MCP 调用

Files: `internal/platform/tools.go`、新增 `execution_mcp.go`/测试；后续 `workflow_execution.go`、`workflows.go`、`workflow_engine.go`、`workflow_runs.go`。

Interfaces: 复用 `RegisteredToolServer/MCPConnection`；抽出 SDK 连接函数 `connectMCP(ctx, connection) (*mcp.ClientSession, error)`，发现与调用共用；`callMCPTool(ctx, connection, name, arguments)` 返回原生工具结果。工具业务错误与传输错误分别记录，原始文本不重写。

- [x] 先写真实本地 MCP HTTP 服务回归：参数传递、结构化返回、工具业务失败、取消和不响应清理；运行确认红，再接通官方 SDK。
- [x] 引入执行节点配置：code/command/mcp，保存及启动校验；旧 connector 仅兼容。代码支持 Python/TS，命令允许项目自身语言/构建入口，不固定 Go。
- [x] 接入现有持久调度、取消、结果与失败线路，保存冻结的连接/工具契约；写入未知结果不重放。
- [x] MCP保存/重载/停用取消/重启回归通过；原始调用回执可见、连接快照不进入公开Run JSON。

## Task 2：OpenSandbox 与原生执行

Files: 新增 `internal/platform/sandbox*.go`、部署目录 `deploy/opensandbox/`；接入 `codex.go`、`codex_appserver.go`、命令执行与文件访问。

Interfaces: Run 关联实际 sandbox ID；受信环境配置声明服务与镜像。采用官方 SDK/API，具体接口以所固定发行版核验。原生 app-server 双向消息、工具审批与停止沿现有调度接口接入，不将 Codex 换成临时单次 shell Agent。

- [x] 在 112 核实 Docker、可用端口、网络及认证条件；固定 OpenSandbox 发行版本和镜像。（模型认证尚未核验）
- [x] 有界真实探针证明创建、Python/TS命令、文件、原生initialize双向通信及销毁；未证明原生命令沙箱/模型turn，探针不冒充 Pipeline 验收。
- [x] Run 创建/核对/释放持久化，执行节点输入与文件仅进入所属环境；实现保留与明确释放，不承诺内存恢复。（Agent产物API源码/隔离回归与112同源SDK读取已过；112正式角色会话仍待验证）
- [ ] 同环境 Agent + Python + TS + 项目命令 + 浏览器；停止、服务失败、平台重启恢复回归，补充不同 Run 隔离验证。

## Task 3：四类节点配置界面

Files: `web/workflow-model.js`、`web/workflow-panel.js`、`web/workflows.js`、`web/workflow-runs.js`、现有 MCP 管理页及对应测试。

- [x] 新建流程只呈现 Agent/执行/人工确认/结束；执行按 code/command/mcp 显示必要字段。（真实保存/重载与112画布截图已验；后续Agent真实整链另列。）
- [x] MCP 工具发现和 Schema 输入、上游参数/结果引用；短代码直接编辑，复杂逻辑引用项目脚本。（维护源/官方SDK HTTP/实际浏览器通过；本批已112标准升级且引用正式API实操通过；引用112网页已通过；真实新Pipeline仍待验证。）
- [ ] Agent 配置基础/高级分组，角色与任务分离；人工确认与权限审批区分；固定测试不能被人审跳过开关跳过。
- [ ] 浏览器实际保存重载、执行结果、失败/停止恢复及窄屏可用验证。

## Task 4：标准部署和仓库自有 Harness

Files: `deploy/opensandbox/`、`examples/platform-workflows/` 入站/安装工具及 SDK；测试仓复制后的独立 `.github/workflows/` 与项目 Harness 目录。

- [x] 112 新实例通过标准命令部署，独立数据/端口，健康版本与源码一致；最小凭据配置不公开。（489a07e；原生模型/Pipeline另列未完成。）
- [ ] 新 Example 入站仅使用仓库文件/固定工具版本与平台 API，申请环境后在其中准备项目；不执行平台源码路径。
- [ ] 检查既有 Issue/PR/Actions，再配置新 label/Issue 模板及互斥路由，旧类型不触发新 Pipeline，新类型不触发旧 Pipeline。

## Task 5：真实验证与交付

- [ ] 固定验证 `scripts/verify.sh`、gofmt、go vet、相关 race 测试；缺环境与失败如实记录并补齐受支持路径。
- [ ] 独立代码评审；修复实质缺陷后复测。
- [ ] 测试仓正式新 Issue 触发唯一新 Pipeline；Agent 在 OpenSandbox 实际承接任务，代码/命令/MCP、人工确认、交接及报告有真实回执。
- [ ] 重复入站、失败返工、停止/恢复至少各一条真实验证；不要用 Mock 通过替代。
- [ ] 用户可查看 112 平台、Issue、报告；三份既有权威进度/验证文档同步结果。逐项完成审计后才标记目标完成。

## 执行记录

- 2026-10-13：确认原 worktree 为 detached HEAD，保留原文档后创建 `codex/opensandbox-workflow`。现有工具管理已经使用官方 Go MCP SDK；固定节点仍将命令/GitHub 混在 Connector 中。先复用 SDK 与持久调度，避免再造协议。
- 当前未部署、未创建测试仓 Issue、未改变旧服务。112 使用既有 `ds112` 会话开始只读核验；端口只是候选，不宣称已分配。

- 2026-10-13 首批实现：MCP 基础层真实 HTTP/race 回归和 go vet 通过，尚未接入执行节点调度。112 标准部署 OpenSandbox 1.1.1/8095 healthy + 401认证检查；固定 pip25.3及 WorkingDirectory 格式修复已通过原安装器应用。已开始有界生命周期探针，112平台8795/新Pipeline仍待实现。

- 本轮最终复核：112 唯一基础设施探针 probe-01 exit0，实际创建→Running→DELETE→GET404；未创建产品任务，未执行产品验收。Go 平台全包 race exit0/58.225s，ruff/安装器2测试及 diff检查通过。scripts/verify.sh 已纳入安装器回归，但全套标准脚本仍未执行。

- 标准验证最终结果：按 README 的 `examples/github/install-tooling.sh` 安装依赖后，第二轮 `scripts/verify.sh` exit0；完整日志 116458 bytes/SHA256 `0c4d2203143df31b4e1420879ecc6ad43116ceb400f88caf4aff46256996d187`，位于 ignored `.data/opensandbox-verification/verify-02.log`。包括浏览器4项、安装器2项、SDK9项、GitHub工具96项、local-preview11项、preview控制器60项、workflow模板107项、Go vet/race/build，平台 race57.646s。首轮缺 Playwright 的原失败日志 verify-01.log 保留；未跳检查。新增 MCP 写入丢响应回归实际只收到一次写请求。

### 2026-10-13 — 执行节点 MCP 正式保存、调度与页面路径

- 新建画布展示 Agent／执行／人工确认／结束。旧 Connector 定义及回执保留，不转换冻结 Run；当前执行表单只开放已接通的 MCP，Python／TS／项目命令仍待 OpenSandbox Run 集成。
- 执行配置随图保存，启动时冻结工具连接和发现的 Schema。参数在保存及运行时校验；服务停用、连接或 Schema 变化明确失败。动作意图在调用前持久化；原始工具业务错误与传输错误分开留存，停止、重启不自动重放。
- 真实 HTTP 的6条核心回归含保存/重复请求、Schema与连接变化、业务失败线路、停止、防重复、数据库重开后下游原生输入以及在途停用取消；race exit0/7.466s。曾实际复现已保存回执未进入下游输入，已修共同输入来源后同层绿。
- 第三轮原完整 scripts/verify.sh exit0；116658bytes，SHA256 bba3f080c1375cf65ee432cc190a4fb14f4bbe132a7874bbf45b0cf266c6ba31，ignored .data/opensandbox-verification/verify-03.log。Go平台全包race62.772s；macOS链接器警告保留，未误判为失败。
- 随后新增的真实 Chromium→平台 API→MCP 页面回归 exit0/4.274s，实际保存/刷新、冻结90秒配置、只调用一次及原始结果展示，390px截图 output/playwright/workflow-execution.png 已检查。首失败是夹具两个节点同坐标遮挡，已诊断修夹具而非 force 点击；截图入场动画采集改为禁用动画。回归已纳入标准 verify 入口，不能把新增检查倒记为 verify-03 当时已执行。
- 112仍为 OpenSandbox8095基础设施已部署；平台8795、原生沙箱任务及测试仓新Issue/Pipeline尚未交付。此为 Harness 隔离回归，不代表 Model Relay 产品验收。

### 2026-10-13 — 112官方SDK命令探针首红，环境已释放

用户影响：候选执行节点的沙箱命令尚不能用于正式任务；环境能创建并就绪不代表程序能执行。SDK固定v1.1.1，已通过隔离HTTP的创建响应丢失不重放、归属、非零状态、取消前后边界及配置私有性6项race；尚未接Run或部署8795。

证据：112同源Linux探针二进制11806922bytes/SHA256 d252490eb64344faba71411bd627043797510529268b4870188bd10ff4d60f70；sdk-probe-01实际exit1。报告1559bytes/SHA256 6b5a85cd3b8388e7fd54b3d5fb1a4454d92644609ea8479b98e3f93e86349f7b，事件create_intent→created→running→python_result→release_intent→absence_confirmed。未收到command_event/init，错误为执行ID未返回、结果未确认；没有自动重放。真实文件/取消未执行，不能写Pass。

原因假设：固定execd镜像与Go SDK的原生argv字段兼容性、HTTP拒绝或流格式差异；目前未证实。已尝试：一次正式SDK调用、按所属Run元数据释放并GET404确认。发现本层无ID分支丢弃底层错误，下一步先修可复用诊断，核对固定execd协议/标签，再作最小有界诊断；保留原报告，不覆盖，不盲重试或宿主代执行。

112通过Jumpserver现有ds112连接；直接112SSH一次连接超时，未转移凭据。二进制传输摘要最终一致；额外结束标记在接收结束后进入shell得到command not found，未产生服务或业务副作用。平台8795/测试仓新入口仍未交付。


### 2026-10-13 — 固定execd协议的真实故障归因与通用适配

- 最小command-diagnosis-01确认HTTP400：固定execd v1.1.0要求command，不接受SDK原生argv。平台在第一次发送前逐项POSIX引用字面参数，统一转为command；不在失败后换格式重试。真实shell回归覆盖引号、换行及命令替换字面值。
- sdk-probe-02仍exit1，1624bytes/SHA256 2c734ef13a7ff4e12d84dd1ff347155d337365e5423f669fb0c07bfca97c9c91，环境删除404。command-diagnosis-02确认HTTP200及init.text执行ID；平台错误地读取init.id。真实响应格式回归先红，再修唯一解析源，原始事件先保留再验证身份。
- sdk-probe-03的Python实际exit0/停止确认已通过；上传HTTP500导致文件/取消门槛未完成。报告2288bytes/SHA256 ae806df7dd191c70e3ba9f6afbdb5d08b0fe465516c59419679b8f738616b0b3，环境删除404。file-diagnosis-01确认服务把权限整数按八进制数字解析：Go 0600序列化384导致ParseUint失败，文件可能已写但权限失败，未重放。
- 文件权限转换落在平台统一upload适配；二进制往返/真实文件0600回归先HTTP500红再绿。OpenSandbox八项race回归exit0/3.563s；macOS链接器警告保留。SDK固定v1.1.1，未升级运行镜像、未修改运行服务或产品。
- 当前仍待修复后112完整SDK探针及标准全套verify；此前verify-04执行到新增错误回归的红版本，exit1原日志保留。不能把隔离HTTP绿当真实112通过；Run环境、原生执行、平台8795及新测试仓入口尚未交付。


### 2026-10-13 — 112同源SDK完整基础设施验证通过

- Linux二进制8192184bytes/SHA256 8a97e5748d85833d42e57a0ce148084203033f814b29c2970235b0144f8fbc2a，远端传输摘要一致；sdk-probe-04真实exit0，19事件/3982bytes/SHA256 7966140026a991c5052ba6cf8eaa9c7ca8ae44eb70f4098b6313ad00539b9f5b。
- Python命令0154097e87554e7eb5f6c407655e06a1实际exit0/stopped；42bytes文件下载与上传逐字节相同，另一次沙箱Python stat证明0600；取消命令e6ffb984881a479a9d10045e51873409真实deadline、exit-1/stopped；所属环境219873b8-4932-44bc-8ba4-0da0c5f88e9b DELETE后GET404。完整原事件与三次失败均保留，不倒写旧回执。
- 三次协议诊断为临时、有界基础设施诊断，未作为永久交付：command-diagnosis-01 606bytes/e98468921ec4fd5e184dd4086ca161109c53caa734137fdc67508da6d1c28262，command-diagnosis-02 740bytes/ce315b0116732db8f21e625f616e95d8f5ae209fa32b97eb1037ee9cc8c7ed19，file-diagnosis-01 975bytes/a65839e62f1262752e0674cf4c1cd18b418a337e9bf5807193a497bd8628fe39。正式改动归唯一Go适配源码及标准探针/文档。
- 112只读核验：8795未占用，Codex不在PATH，当前用户私有auth.json存在（未读取内容，未证明登录有效）。下一步固定原生运行镜像/双向app-server、Run环境持久关联及标准8795部署。此次没有创建产品任务或代做产品QA。


- 本批源码最终标准验证verify-05真实exit0，116870bytes/SHA256 42f728f8692ecacb0a109038c38144726386c2ba64a5520503982f3678bee6e8；包含新增真实执行页面回归、安装器/SDK/模板/部署控制器原门禁、Go vet/race/build，平台全包race62.687s。测试期间源码保持稳定，未将旧verify-04红结果改为通过。基础设施适配完成；整体目标仍实施中。


### 2026-10-13 — Run环境持久关联与正式恢复入口

- 新流程可声明environment=opensandbox；Run自动分配其自己的/workspace，拒绝宿主路径。服务连接/镜像/TTL来自受信启动配置，任务不能传密钥；连接身份随Run冻结，变更时拒绝继续。旧本机流程的路径规范化、工作区占用及冻结历史保留。
- 创建意图先落库再发送，平台重启/响应丢失不重建；显式Owner核对只读查所属Run环境，成功后还须正式resume/return继续。活动Run不能释放；已停止/失败/完成/取消且执行静止后可显式释放。删除丢响应保留release_unconfirmed，重复释放拒绝重发，核对404后记released。释放后resume/return均拒绝，不伪造新环境。
- 环境生命周期/ID/失败原因在同一事务保留历史，服务身份摘要不公开。正式POST /api/workflow-runs/{id}/environment/reconcile和release已接权限边界；启动参数--sandbox-config/--sandbox-image/--sandbox-ttl不属于Agent输入。
- 六项真实HTTP隔离回归含Run去重与跨Run隔离、创建丢响应重启核对、删除丢响应与历史、创建意图后崩溃、宿主路径/服务变更拒绝及正式HTTP授权。全Go vet/race真实exit0，平台65.004s；日志615bytes/SHA256 439667461d23323bdc60c3a367eeb51f69f5bf3dae6004a1efec317794a27df3。早期释放回归误置在运行中，收紧后发现检查阶段错误；修正夹具在真正释放之后检查，未把该红归因为线上产品故障。
- 此为Harness环境生命周期实现和隔离回归，尚未在112平台8795通过正式Run应用；沙箱原生Agent仍明确拒绝启动，代码/项目命令、界面环境配置及独立测试仓入口待接通，不宣称整条Pipeline已完成。


### 2026-10-13 — 代码/项目命令、环境配置与完整输入回归

- 执行节点保存 code/command/mcp 三种互斥配置：Python/TypeScript直接写代码，其他仓库语言用项目命令（例如 go test、mvn verify），不限制项目为Go。新代码/命令要求OpenSandbox；无宿主回退。每个动作先持久化意图，实际命令ID、退出码、原始事件及私有完整日志进入共同回执/下游输入/日志API，重试必须检查副作用后正式return。
- 隔离HTTP与实际shell进程验证：前节点写文件，后节点从同一工作区读取；完整60000字节输入及32项参数通过私有0600 JSON文件 WORKFLOW_INPUT_FILE 传递，上游实际执行回执可读，含命令替换/引号的参数不被执行。旧整段环境变量契约回归先红；修复后通过，不截断输入。代码/命令源也经私有文件执行，避免引用扩张越过Linux单参数限制。上下文超过8MiB明确失败并要求引用产物；远端输出超过16MiB在SDK解析前失败，按已知ID中断并核对停止，不把截断EOF当成功；17MiB单事件回归原先误接受红，修后绿。
- 发现pending环境（尚未发送创建）被错误禁止resume/return，已修正式入口并验证恢复只创建一次、未创建时release零外部请求。新安装的image/TTL默认变化不再使旧Run失效，实际采用旧Run冻结值；服务身份变化仍拒绝。历史创建/删除响应未知、重启、归属/授权回归继续通过。
- 实际Chromium→平台API保存重载Python、TypeScript和任意项目命令、切换MCP、启动独立Run、仅一次MCP调用、释放环境及390px页面验证已纳入原scripts/verify.sh；生命周期为SDK HTTP夹具，不冒称112真实Run。已释放环境无继续/回退控件，也不再提示可回退。前一轮浏览器红为两个同名应用按钮定位歧义，已限定设置表单；未force点击。
- 当前完整scripts/verify.sh exit0：verify-06.log 117165bytes/SHA256 da26e7b1d0109946bc8d66dc044487af188d9e60552b9745933a8144bc4bfef1，Go平台全包race64.939s；原全部工程检查、真实浏览器、vet与构建均完成。独立相关race门禁7.841s绿、前端59项绿；日志均在ignored .data/opensandbox-verification。一次构建红为新增输入上限检查放在raw声明前，修正后实际重测；本机受限listener红与原测试失败日志保留。macOS链接器警告仍保留。
- 112 OpenSandbox8095的此前真实SDK完整probe-04仍是基础设施证据。平台8795尚未部署，原生Codex沙箱会话仍显式拒绝，TypeScript实际执行及测试仓独立新Issue/Pipeline尚未验证。当前不提交产品通过或上线结论；正在准备固定Linux运行镜像，之后接通原生双向会话与正式隔离部署。8788/8793及旧产品任务未改。


### 2026-10-13 — 112固定运行镜像首次构建失败（未部署）

用户影响：原生Agent与TypeScript/浏览器真实Run仍不能上线，不能创建测试仓任务冒称可执行。维护源新增Dockerfile/锁文件/标准build.sh/无凭据check.py，固定Node22.22.0、Go1.25.6的Linux/amd64官方镜像摘要，Codex0.151.0/tsx4.20.6/Playwright1.59.1及完整npm锁。112 source tar40960bytes/SHA c5d3cd5da33629324b577253f77583b49dc9e3da2f46c309ce3b1d65b1e61930逐字节核验，唯一runtime-build-01真实exit100。

实际证据：两个官方镜像摘要拉取成功；APT的三个http://deb.debian.org InRelease均80端口连接超时，update忽略失败后缺python3-venv候选。已证软件源未成功取得，不是包已被证明不存在。原build.log/status保留；未启动8795或真实产品任务。下一步最小核验同容器网络下官方HTTPS源；确认可达后改唯一Dockerfile与APT严格失败设置，再通过标准构建入口新attempt复验，不手装依赖/host network/删除原失败日志。当前未证HTTPS可达，镜像运行检查/原生登录/产品QA均NotRun。


### 2026-10-13 — 原生双向协议共用与网络根因确认

- 原生Codex0.151.0本机stdio与候选远端WebSocket复用同一 executeAppServerTransport：实际cwd、thread/resume身份、turn/start、引导回执、审批及原生消息处理不复制第二套协议。新增共用传输回归先缺接口红，接通后同线程恢复/交互审批与拒绝换线程通过；原全部原生/停止/引导相关race回归6.614s绿。
- 固定官方coder/websocket v1.8.15；真实HTTP/WebSocket帧与认证回归先缺接口红，接通后同原生相关race5.139s绿。只接受文本JSON帧，4MiB上限；拒绝无原生capability或转发OpenSandbox控制面API key；错误不公开endpoint或header，也不重放未知请求。此仍是隔离协议测试，未连接112真实原生Agent，不冒称实际模型执行。
- 修改后的原完整scripts/verify.sh exit0：verify-07.log 117184bytes/SHA256 024bc5171604ded217aef2025f4c1d2cb01ac953bee950b66cc7c29d16330c3c，平台全包race63.608s。原流程、审批、停止、恢复、浏览器、Python供应链门禁与构建保持通过。
- 112首runtime-build-01的APT失败继续保留。直连HTTPS单探针exit28/1963bytes/SHA c0d778071211374322ac7ec68c33cb4adf17d3422b474cfde401c41f99209549；宿主Debian/npm及容器npm/镜像源直连均超时，未盲改镜像源或host网络。随后只检查代理存在/是否含凭据：112原有小写http_proxy/https_proxy及Docker代理（均非回环、无凭据）。容器显式引用原代理后Debian/npm分别HTTP200，证明构建未带入现有代理的配置缺口。
- 标准build.sh只显式复用现有代理的Docker预定义build args（大小写兼容），不打印值、不写ENV、不拷贝宿主环境/凭据；官方Debian源改HTTPS，APT索引更新任何错误即失败、禁重试并限时。第二批源tar40960bytes/SHA e58eef714bfd1d4bb9fbc8f81ef2cc9b6043e56ae5bf7da581645000b57b2f8a已核对并进入独立runtime-build-02。目前实际构建/运行检查结果仍待核验；未跳检查，首失败目录未覆盖。
- 仍缺平台原生沙箱进程归属/停止持久恢复、角色文件/Skills/最小认证准备、正式112平台8795及测试仓独立新入口。沙箱Agent启动当前继续明确拒绝，不把协议SDK就绪当整条Pipeline已打通。


### 2026-10-13 — 112标准运行镜像真实构建/检查通过

- 第二次runtime-build-02实际exit0，源码tar与维护源一致。不可变镜像ID sha256:8b6ea4396af7a1775eba7dfd4ec3f9c217670eddb93c4b829217e916c7b665c1，amd64/2704127407bytes；context源清单395bytes/SHA 6985905984b35c5c55d92ff41a8898f54550ea9fff528c1f307e192998b3df43。最终镜像Config.Env中HTTP_PROXY/HTTPS_PROXY/NO_PROXY大小写六项均不存在。
- 原标准build.sh在无网络、2CPU/2Gi、256pids/no-new-privileges临时容器中完成check.py：实际Codex0.151.0、Node22.22.0、Python3.11.2、Go1.25.6、Git2.39.5、Chromium147.0.7727.15；Python/TypeScript/Go实际读取同一工作区文件，Go实际编译运行，浏览器真实启动/页面内容检查，原生app-server双向传输选项存在。所有程序来自锁定镜像；这不是完整SDK/平台Run或Model Relay产品验证。
- 原build.log44992bytes/SHA dcd5286a281c55977137d2f748d5da659ac572020a43923761057be92574aa36，实际check.jsonl353bytes/SHA e51f2d450b8c651d7cd77a5b79c26fdb27145f6a3c1ded2a59f86d4f04b51ca5，check.log为空；112私有独立runtime-build-02保留。首runtime-build-01 exit100及所有有界诊断均保留，没有覆写或手装包。
- Dockerfile/完整npm锁/标准build.sh/check.py/.dockerignore归唯一维护源；README给出标准构建、独立attempt、不可变image-id、构建代理及无凭据运行检查范围。ruff与格式检查5文件绿，bash语法绿。原verify-07覆盖当前Go/SDK/原生协议候选；之后构建脚本的代理大小写/HTTPS改动另有112实际构建和运行绿，未冒称前次verify已测后改动。
- 剩余：运行环境自身代理要通过受信部署配置提供；原生沙箱启动/取消/重启及角色文件/Skills/认证尚待正式接通。Codex模型登录和调用仍NotRun，平台8795仍未部署、测试仓独立新Issue/Pipeline尚未启动、全分支独立复审未完成。目标保持进行中，不把镜像/局部传输通过当整链交付。


### 2026-10-13 — Run会话归属/原生进程基础回归，112初始化首红

- 原有workflow_steps→Run关联现在用于沙箱会话和内部工作区排队键；没有新任务ID或独立环境注册表。可信引擎提交携带非JSON字段environmentRun，校验Owner、Run环境ready及/workspace，不经过宿主目录解析；普通API不能提供此内部授权。后续会话输入仍固定相同工作区。不同Run同名/workspace不再互相占用，原本机排队/停止兼容回归通过。
- focused回归先实际lstat /workspace不存在红，后一次夹具把人工确认当Agent而触发正确状态409；改为可信提交/关联的隔离单元夹具，核心归属及独立调度race6.351s绿。此不是正式Agent编排整链；公开沙箱Agent启动门禁仍保留。
- 固定SDK新增仅供原生app-server的后台启动、所属执行ID核验与显式停止；普通执行节点仍不允许background。启动响应未知不重发，已停进程再次核对不重复interrupt；ID限制阻止响应路径注入。原生8080使用固定Docker版本实际支持的映射端口；官方已安装networking实现只把8080与execd44772分别映射，未假设任意端口可用。
- 回执保存失败的原生清理先真实红后绿；认证拒绝原来被当就绪等待，10.11秒内99次握手红，改为有界TCP就绪检查后仅一次认证握手，同层绿。已有身份/授权/文件/取消/私有配置回归保持。probe CLI新增--runtime，在原SDK门禁后实测TS与无登录原生启动/认证initialize/停止，不送模型turn。
- 原完整scripts/verify.sh exit0：verify-08.log117176bytes/SHA d6855812dbcc03628d9bb7a5e4bbf977570e91d9d81adf8df15258ce16f94153，平台全包race65.275s，原全部门禁/vet/build通过。实际SDK探针结果单独列，不把标准本机绿当112通过。
- 112同源Linux探针12087518bytes/SHA c3d8c238c738ef8096e75e91b9922dd96c5fb8440ef676418e7d77710ac0c1aa，传输摘要一致。runtime-sdk-probe-01真实exit1，6061bytes/SHA 6f2c834f00216ea465cf90641a16c78fe7bedffe59c173b676a0087e668dcef0；Python/文件/取消前置通过，TS5a3ee6ce…实际exit0/stopped；native313e1f33…启动running但没有native_initialized。停止回执exit0/stopped，环境4c242c08-6829-4963-8e9d-c6ed085ef0d0 DELETE→GET404。原日志/事件全部保留，尚未证初始化原因，没有模型凭据或产品操作。
- 影响：还不能启用沙箱Agent/8795或触发新产品任务。下一步从此唯一首失败日志定位握手/原生协议原因，在维护源修正后新独立attempt复验，不重放同一次启动。原生进程持久归属/重启恢复、角色文件/Skills/最小认证及远程产物API仍待连接。


### 2026-10-13 — 原生启动退出码与缺失Home根因

- runtime-sdk-probe-02真实exit1，6378bytes/SHA 6f1b987e8add1665d7bdece4f00e0c6199255566fbbd0e554b5334e68541c379：同源TS exit0；原生dc47663b…首次Running，两次纯传输未就绪后同ID已exit1，未重启/重发initialize/模型turn。已停原生与环境ab8660b9…DELETE→GET404，原日志保留。
- 修正就绪检查：Docker代理TCP可连接不能证明Rust应用就绪；只对未发JSONRPC的瞬态传输失败，在同一所属ID仍Running时有界等待。实际HTTP拒绝仍一次即失败。相关就绪、认证不重试及回执失败清理回归绿。
- 无网络固定镜像最小诊断native-start-diagnostic-01真实exit1，226bytes/SHA 6b8be9cb57e19c7935be8c5d8b6cdc881ce1dd7e4728ce203300c940dd8bafc0：原始stderr明确CODEX_HOME目录不存在；这证明当前启动准备缺口，不把前两次已删除环境的stderr补造。下一步通过SDK正式准备私有Home、保留后台日志后新独立探针复验。
- 用户影响：沙箱Agent仍未开放，112平台8795和测试仓新Pipeline未交付。已有execute/MCP及镜像检查不能替代原生模型工作。


### 2026-10-13 — 112正式SDK原生初始化绿，命令隔离前提红

- nativeHome通过固定SDK CreateDirectory准备0700后再启动，后台日志在所属ID停止后、环境释放前通过官方GetCommandLogs保留，16MiB限制不会假成功截断；私有capability脱敏。定向race回归2.852s绿，包括Home、原输出/游标、超限、拒绝日志ID穿越及原生清理/认证不重试。首次普通受限运行因本机listener禁止失败，独立提权测试入口后通过，未修改用例。
- runtime-sdk-probe-03真实exit0，7852bytes/SHA 2e9540b351fc4dd008f4e732c9bdc9a6228a0d62a245570fa981d8500a2f75be；探针binary12113520/SHA 3a24f4c1cbfa267db42778ce296699cfb4fb996bcf281b7b43927d8115329150。TS d30ed40e…exit0；native7ecc23f7…一次启动，同一ID首传输未就绪后第二次authenticated WS initialize实际成功，Codex0.151.0/Linux确认。随后native exit0/stopped，完整日志720字节保留，环境c8aec298…DELETE→GET404。未带登录、未送模型turn、不是产品QA。
- 提前检验真正原生shell隔离：同镜像、无网络/no-new-privileges/无权限增加的native-sandbox-diagnostic-01实际exit1，414bytes/SHA f9fe3fb28b1a54108e183839424b01f5fbca1960c1538ac7ef6e88eea9bcff0f；bwrap报告无法创建namespace。112只读内核检查unprivileged_userns_clone=1、max_user_namespaces=4125642、apparmor_restrict_unprivileged_userns=0，不能归因内核全局关闭。下一步检查Docker限制与官方支持的最小运行配置；不启用privileged/hostnetwork、不把原生策略改成无限权限、不将initialize绿当命令执行通过。
- 原生Home/协议本次已实际闭合，但完整角色会话持久恢复、远端文件/Skills/最小认证、8795正式平台、新Issue/Pipeline仍待实现，公开Agent门禁继续保留。


- 本阶段最终标准scripts/verify.sh实际exit0：verify-10.log117163bytes/SHA 9a5992e51e51b67dbd33fd1915cfd38b0643a6e74a38860aee4e43ab29cdfa4b，平台race结果另见完整原日志；全部工程门禁/vet/build通过，不替代112模型执行。verify-09旧handle已失效，仅原日志证明race65.079s绿，未补造父进程exit回执。
- 官方固定OpenSandbox1.1.1源码确认bootstrap.execd.isolation=enable会自动增加SYS_ADMIN、移除AppArmor/seccomp限制并调整系统路径；不是普通目录准备。现阶段不启用该权限扩展。112内核允许userns而当前容器原生命令仍红，下一步优先验证更小的运行配置，若确需扩展则提供明确受信配置/影响再请求授权。无privileged、无宿主挂载/hostnetwork改动。


### 2026-10-13 — 原生意图持久化与只读恢复保护（尚未开放Agent）

- 新原生进程归属直接使用既有输入message_id，保存Run/会话/所属sandbox和实际command_id。启动意图及状态事件同事务保存；拒绝别的消息、会话、替换执行ID及已完成消息重发。重启中launching/running/stopping转unconfirmed，既有队列、继续和Run回退均不能越过待核对状态。
- Owner原环境reconcile只读核对相同执行ID：Running或404历史缺失保持未知，只有相同ID明确Stopped才闭合；不interrupt、不restart、不新建线程。正式release确认容器不存在后才闭合未知进程，明确个人exit状态不可得，不伪称exit0。Run/API保留native_processes，页面提示待核对并隐藏继续/回退，仍提供核对/释放入口。
- Scheduler对原生停止未知错误保留failed与原因，不再因用户Stop吞成stopped；原正常停止/队列接续保持。定向durable回归先缺接口红，再真实隔离HTTP/重启/误归属/只读/释放回归race3.152s绿；停止结果回归race2.519s、页面12项绿。此为Harness回归；原生Agent后台总入口尚未使用这些记录，仍待角色准备及真实整链，不冒称产品Pipeline已有此证明。
- 更低权限诊断native-sandbox-nonroot-01真实exit1，394bytes/SHA e0b13441631363df6f8d6458cc740a3a5ac1a093ba8f4579a473dfb4d3a4006f；非root且原安全策略仍拒绝namespace。未改宿主或扩大容器权限。README提供官方兼容扩展及实际SYS_ADMIN/AppArmor/seccomp变化、限定的一次无凭据诊断提案，当前未启用；正式权限入口和新Run冻结也未接入。


- 本阶段最终标准scripts/verify.sh实际exit0：verify-11.log117279bytes/SHA ef337cee10ff15d91933f51dbe30d0f34d97d400ecfe57d7f5cfb10c84c56391，平台全包race67.483s；原浏览器、Python供应链、所有流程/权限/停止恢复、vet及两项构建全部通过。仍是维护源门禁，不冒称112实际Agent/Pipeline已通过。
- 已就README中一次无凭据官方兼容模式诊断提出新增权限问题，等待用户明确选择；默认维持原权限，不把等待或预选项当授权。非依赖该权限的源码/部署准备可以继续；实际SYS_ADMIN/过滤变化尚未应用。


### 2026-10-13 — 独立8795标准安装候选

- 新增deploy/platform标准Linux CGO构建与用户服务安装入口；固定Go1.25.6，原go.sum校验，精确revision写入二进制--version和/api/health，不以旧0.1.0常量冒充源码版本。构建、来源、模块校验、二进制摘要和失败日志独立保留。
- 安装器核验源码revision、不可变沙箱image、私有服务配置、独立目录；首次生成私有账号环境文件，升级保留原密码/配置/数据/旧二进制。拒绝同revision不同字节、外部symlink、配置漂移、其他服务归属和占用端口，不更改其他服务或自动回滚DB。普通安装与明确--activate分开，激活须health精确版本证明。
- 四项升级/身份/数据/权限隔离回归先缺安装器红，修正macOS规范路径夹具与修改样本被fixture重写后实际4项绿1.359s；health回归实际缺revision红→绿0.512s。二进制读取限制单独256MiB，配置64KiB，不让大二进制升级被配置上限误拒。
- 112最新只读前置：10.1.20.112:8795空闲、独立目录不存在、目标unit未注册、剩余1173445120000bytes；宿主Go1.21.6、GCC11.4可用，标准脚本将显式获取固定Go1.25.6而不是使用错误宿主工具链。原生auth.json仅私有权限检查为真，未读取或验证登录。服务尚未部署；完整维护源门禁和112实际构建/安装待完成。
- SYS_ADMIN/AppArmor/seccomp新增边界问题仍等待用户明确回答；没有应用新权限、没有产品Task/Issue重放，公开沙箱Agent继续门禁保护。


- 安装候选完整标准scripts/verify.sh exit0：verify-12.log118087bytes/SHA cb7cba8079bb975e35ab4eeb61273910b5c86fe3e19e757a57ebf36c8d3dfd08，平台全包race66.086s；新增安装器4项、health版本、原浏览器和全部供应链/权限/恢复/vet/build均实际通过。接下来只部署此精确源码候选到独立8795，不改旧8788/8793。


### 2026-10-13 — 112独立8795标准构建与实际部署

- 经完整verify12通过的精确维护源489a07e72047d8c8bcbf5da0140b11eea28ce2c9，私有归档4003840bytes/SHA ee30a663d25f84d408b780fbbd6b604fede723deac2a27abd16eb29cc7646379经现有SSH传输、原始摘要重核后在112标准build.sh构建。实际exit0；固定Go1.25.6/Linux/CGO、go.sum校验均成功，未替换宿主Go1.21.6。
- 实际binary18945056bytes/SHA184344b5872f4e649a6804422f9b937d9bbeb760a13b9fe5acb2452b03db38fa；独立源码manifest11962bytes/SHAbd4cafe840ae5e4d890442ca4ea14d65b09b97edd279c9c238d4167769873953、模块验证与原构建日志保留在私有platform-build-01。原安装入口--activate真实exit0，agent-platform-8795.service已链接启动，仅独立目录/端口；未改8788/8793或既有账号。
- http://10.1.20.112:8795/api/health远端及Mac无代理实际HTTP200，revision为同489a07e、version0.1.0。初始admin凭据仅保存在新实例0600文件，未打印/发送；数据0700、不复制旧实例。用户服务logout/reboot持久性未验，不把启动成功当模型会话或产品验收。
- 新标准check.py与独立基础模板已传输30720bytes/SHAdfd0d2047bcb39eaf5ab6c4bd86d0fa10550af9befa55926e189ce296dfbfc11；通过正式API验证Python→TypeScript→人工确认→Go项目命令→结束/去重/环境释放，原报告platform-check-01.jsonl独立保留。当前运行结果待核，非原生Agent或产品QA。
- GitHub连接器读取测试仓404不当作仓库消失；同现有gh凭据仅进程内存经官方REST已确认big91987/model-relay私有仓存在，main为0c197c8b3d98346107e65dbf0c41ef5d6b73d437。gh原TLS超时及目录/Issue读取URLError保留，尚未创建新任务，不依赖旧快照重放。
- 容器命名空间扩展授权问题仍待答复；未启用SYS_ADMIN或放开过滤。公开沙箱Agent门禁保留，角色文件/原生生命周期总入口与测试仓独立Pipeline仍待连接。8795基础实例已部署，不宣称整链完成。


### 2026-10-13 — 8795正式基础链路与真实页面绿

- 原check-01唯一Run d0988dbd35a5218864aba04a8f4939e1/workflow97af7cbeb711b313bf282aa7ddd706c3真实exit0，报告32355bytes/SHAea6fb8312ee3dd8fd9488f2bc258bab0ec515d239781126acb95f95ef650dea1。重复相同request_id得到同Run；同一个sandbox5269f9f9-09fe-471a-bad1-dc80ea92b495中Python480/TypeScript485/Go691bytes完整未截断原日志、实际exit0。literal输入与全parameters/previous_results经私有文件传入，共享文件跨节点读取保持。
- seq3真实waiting后经正式decision进入seq4项目Go命令、seq5结束；人工确认者为Harness基础回归操作者，不是产品QA。之后原environment/release确认released，原history pending→creating→ready→releasing→released完整保留。没有后台Agent/model turn；不以该证明替代新测试仓独立Pipeline。
- 固定runtime镜像内真实Chromium147.0.7727.15检查原8795部署页面，browser-check-01 exit0/stderr0，四类节点及已保存Python/TypeScript/Go字段正确，Run completed和环境已释放、原命令输出及不再显示继续/回退均正确、page_errors空；1440画布PNG140987bytes/SHA1319bf207a2c754a879ef0e965cfb854d33fd6b075531d51d85a81a3f5f6d761，390Run PNG105421bytes/SHAb91aa8e8363f02a3eb2f6a650e07c3a39a381d664bf2fe409cca6ce192195d65已独立摘要一致后视觉复核。会话正式logout、浏览器及临时容器结束；凭据只在远端stdin/内存，不进入日志或截图。
- 首截图回传读取tmux250物理行不足，未重拍重发；只读扩至原10000行取得同份图像、核对摘要成功。原报告与原失败保留。视觉观察发现执行结果区域有两处同义退出码，后续展示简化候选；不影响本次链路结果，不隐去原回执。
- 8795访问链接http://10.1.20.112:8795；实际基础验证Run链接http://10.1.20.112:8795/workflow-runs/d0988dbd35a5218864aba04a8f4939e1。原生权限/生命周期总入口和测试仓新Pipeline仍未完成，既有审批问题待用户回答。


### 2026-10-13 — 原生事件保存失败保留真实执行ID

- 维护源新隔离HTTP用例实际复现：SDK已返回init执行ID，而onEvent失败前未解析ID，原startNative返回空ID/未知且未interrupt实际已启动进程。native-launch-persistence-red-01 exit1，收到1次launch/0次stop，未补写为已清理。
- 先解析真实执行身份再调用事件保存；回调失败时仅停止该已知ID，返回原失败和实际停止/exit回执，身份未收到时原未知不重放策略保持。定向原生/初始化/认证不重试/就绪/唯一launch回归race5.023s exit0，1次launch/1次interrupt/同ID停止-1真实证明；原macOS linker warning保留。
- 这是通用源码修复及隔离回归，尚未应用8795二进制；8795继续489a07e，不能把新维护源局部绿当已部署完整Agent。标准全量验证及后续同配置升级待执行。


- 本阶段标准scripts/verify.sh原进程明确exit0，verify-13.log118098bytes/SHA66365f0a765dfa767f57d3988246720e51c9c83cec2af641f1b5feabf69c88c6，平台全包race66.261s。部署检查Python lint/format与新浏览器脚本语法检查绿，原全套浏览器/供应链/SDK/权限/恢复/vet/build保持；新原生事件保存失败回归包含在全量。维护源后续提交包含该修复与可复用部署回归工具，8795实际部署仍为489a07e，不把提交完成冒充升级。
- 后续GitHub正式同身份只读读取已成功：Open Issue37/34/33/32及旧历史任务保持；仓库仅agent-platform-entry.yml/deploy-local.yml。当前旧entry blob5b3b98ae556954edc3ac5017391a564752985296仍以平台checkout路径运行github_entry.py，需新入口复制依赖为仓库自有并互斥路由；本次未修改旧入口或创建新Issue/PR，避免未接通Agent时制造空任务。


### 2026-10-13 — 8795标准升级8912141与原Run保留

- 源89121419d2e69bb341371af6a0567fa754b724c8归档4044800bytes/SHA6edf6b34c57a0ebb2dd156ec6cc5c43d057927f7b723f265c20b1b03189ee122传输ACK核对；源码解压后逐文件对原archive验证。原标准build-03 exit0/install-03 exit0，独立原日志均保留，不改宿主工具链/其他端口/命名空间权限。binary18945056bytes/SHA54be78ab72158bd17521b81fcf6d0b3e896928debb7e6381e9d50ee6c59841a8；远端及Mac /api/health HTTP200精确8912141。
- 操作者临时核验包装脚本02在构建前因http变量遮蔽模块UnboundLocalError中止，已核无build/install派发；03标准构建安装成功后，同名cookiejar引用造成AttributeError，未把该错误归因平台安装失败。原失败与阶段事实独立归档。随后仅读回，不重复构建/安装：原d0988dbd…Run全JSON与升级前正式API原文件逐值相同，completed/released保持、原web源码字节相同、配置值匹配及现有凭据实际登录/logout成功。
- 原包装脚本未持久化凭据文件的升级前字节，进程退出后该比较无法补造；报告明确exact_credentials_before_after=not_verified_after_operator_wrapper_failure，不以mtime/正常登录冒称文件字节前后相同。标准安装器保持凭据/配置的隔离回归和实际现有凭据登录分别成立。
- 可查看独立实例http://10.1.20.112:8795与原基础Run。原生启动事件保存失败的维护源修复已纳入本二进制，但公开Agent入口仍保护；原生模型/角色会话/真实新测试仓Pipeline未执行。新增SYS_ADMIN/AppArmor/seccomp诊断权限仍待用户明确选择。目标持续active，本轮实际部署/升级进展不等于整链完成。


### 2026-10-14 — 原生生命周期桥接与最小角色输入（尚未接通公开入口）

- 私有executeSandboxAppServer将官方SDK后台进程、同一Codex原生协议与既有message_id进程记录连接：init到达即持久化实际执行ID，认证WebSocket沿原thread/turn/approval协议，无新的任务ID。停止后保留实际回执与完整原诊断；未知launch/interrupt进入unconfirmed，阻止后续输入执行，不猜ID、不换线程/服务、不中断重放。连接须匹配Run冻结来源，工作区固定同Run/workspace，角色Home只用已有conversation ID派生。
- 隔离SDK/HTTP/WebSocket服务证明正常轮次、取消、launch响应丢失、stop响应丢失和事件保存失败；这是协议及生命周期回归，模拟的回复/模型不当作112真实账号、模型或Pipeline证据。早期夹具误把模型配置当作必然thread帧、又混读multipart metadata/file导致非稳定能力token已定位纠正，原红保持，不放宽认证或状态断言。
- 实际红native-bridge-red-03（716bytes/SHAd7ba93822cc409e87cb69c9a19c8a993e39964656041b47d14abf5d026b9af02）证明两层清理对仍Running的未知结果发送2次interrupt。结果对象保留同调用已发送中断事实，外层/标准runtime探针不再重发，实际只1次interrupt并保持unconfirmed。该字段是内部同调用事实，不新增公共权限/配置，重启仍依赖原持久记录核对。
- 普通执行命令也有事件写入先于ID解析导致漏清理：command-event-persistence-red-01（512bytes/SHA592037b8123fa1f7d5fe6c1caa942958ab6fac77ab070757cee45c8be1b83a91）实际launch1/stop0。现统一先保留实际ID、仍归档收到的原事件，日志失败时只清理同ID；原停止/输出上限/归属及未知不重放回归保持。
- 新私有角色包仅含所选Skill的规则及支持文件、原生配置和私有owned auth.json，不复制整个HOME或宿主环境。os.Root+NOFOLLOW+打开前后身份验证拒绝源外链接/非普通资产/非私有认证；每角色独立Home、可执行资源保持0700，其余0600，512文件/64MiB总界限。仅临时认证夹具被读取测试，不为此读取真实Mac认证。明确拒绝把宿主stdio程序当容器工具、角色seed替换共享工作区和全宿主继承/未接通的变量删除，不静默丢指令。
- native-role-inputs-green-03 race4.859s exit0；阶段标准verify14 exit0/118109bytes/SHA515559e9ae5b2cc3f039af49a609820a2389b1d9fd928db055498a1b38751a00、平台race66.246s，覆盖上述生命周期/角色输入及原完整供应链/浏览器/vet/build。此轮尚不包括之后的模型约束改动。
- 模块仍待正式角色文件准备标记、原生skills/list及skills/config/write复核、远端产物访问和main执行器入口连接；纯收集接口未保留无标记的盲覆盖上传入口。112仍8912141，公开沙箱Agent门禁保持，不冒称角色会话/测试仓新Pipeline已跑通。新增容器权限问题仍待明确回答，没有增加SYS_ADMIN或改变过滤。

### 2026-10-14 — 所选模型进入原生请求并拒绝替代模型

- 查阅官方App Server文档https://learn.chatgpt.com/docs/app-server，并由实际Codex0.151.0只生成实验Schema到私有临时目录；未登录/推理。固定Schema确认thread/start支持model/allowProviderModelFallback，resume支持model，二者响应含有效model；skills/config/write按path控制enabled并返回effectiveEnabled。文档的其他新字段不直接套入固定0.151协议。
- native-model-binding-red-01实际证明原请求没传所选model且收到不同model后仍发送用户turn。共同协议已对新建/接续显式传入快照Model，新建禁provider fallback；先保留真实原线程ID，再核对返回model，不符时在发送用户内容前失败。不能以此声称gpt-6.1-sol账号可用，实际推理仍未验。
- 新建及同原线程接续的匹配/不匹配四场景、原生审批/身份及桥接回归race2.953s exit0；原native-model-binding-green-01夹具缺固定Schema模型字段而红的证据保留，隔离服务已按实际响应补齐。因共同协议新改，完整verify15正在运行，完成回执待读取；不沿用verify14作为此版本完整绿。

- 标准verify15原进程已明确exit0，原日志118098bytes/SHAb0382b30e5971da206cdbc9c84b97c8b977576cd4cc507345da48a87dcc7aa60，平台全包race73.662s；包含上述模型约束四场景、生命周期/角色输入及原完整浏览器/供应链/vet/build。macOS linker warning保留。此结果是维护源完整验证，112仍8912141，未称新代码已部署或真实模型可用。

### 2026-10-14 — 同进程Skill核验与112首次就绪故障

- 角色桥接现显式传入所选Skill路径（空集合也须检查），同一已归属app-server依次skills/list、仅修正启用差异的skills/config/write、forceReload再读后才thread/start/resume。所选缺失、发现错误/错误cwd、启用回执缺失或拒绝、重读仍多启用均拒绝任务；缺enabled字段不默认为disabled。宿主既有预制路径保持原校验入口，未新增可编辑策略/任务ID。
- native-skill-scope-red-02实际证明未经原生范围核验就进入thread。green04隔离官方SDK/HTTP/WS race3.434s exit0，包括空所选集和同进程正常/取消/未知launch-stop；green02夹具未处理skills/list导致deadline红保留，补齐固定协议后green03与04绿。native-skill-scope-red-03证明缺enabled错误接受，修后绿。这是协议回归，不是实际112模型通过。
- 原完整verify16明确exit0、平台race66.293s，覆盖本次Skill路径与扩展标准无凭据探针；全文bytes/SHA待最终入账。此轮不含之后就绪等待缺陷修复。
- 新探针binary12126683bytes/SHA06429c27764e45bcdb305edbac173010a4edd6c8c8d1b0f610681d4f04326532，112传输ACK一致，基于10d00dc+本次未提交源码；单独库存同时涵盖新增未跟踪文件，diff摘要不能独自代表全部源码。保持原容器权限、无原生认证/模型turn，原8795未升级。
- 实际native-skill-runtime-01 exit1：报告911bytes/SHA4d4463bf6a2d20f5f68746b7fa852c07f57f131b16754e6f4e5b0b7c200212e4，首次create Running后execution readiness失败，尚未Python或Skill；环境38781c58-b769-4aca-b474-04031150a7da原DELETE及GET404确认，不保留为未知运行、也未重发创建。服务日志同时间容器启动成功，首Ping具体网络错误因现有安全错误摘要尚无法恢复，未假称已定位为namespace/Skill/模型故障。
- 用户影响：新环境创建偶发在execd可服务前即失败，平台/探针都仅在生命周期Running后做一次Ping。原因假设为端口或execd启动延迟；最小隔离复现检查相同ID只读就绪等待、实际HTTP拒绝不重试、取消/环境终止而不重建。新回归红结果待核验，原首次失败证据保持；不盲目重跑整探针或扩大权限。

- 补充原verify16全文118098bytes/SHA c62116722977dcf22775dfbb5c336434115ef75a00cc9f3a585e7480e2e53362，exit0、平台race66.293s。共享就绪路径后又改源码，verify17尚运行，不能用16证明之后版本。
- sandbox-readiness-red-01真实隔离HTTP复现首次Ping连接EOF立即失败，取消/环境终止检查也被同一早退遮蔽；SDK自身可对复用GET连接补一次新连接，夹具保留两次初始EOF。共享waitReady只在同一所属Running环境对连接拒绝/reset/EOF作有界只读等待；实际HTTP、TLS/超时/归属拒绝即失败，取消/过期不重建。原SDK及Run/探针统一入口，green01 race5.214s exit0，原取消、认证/未知及Skill回归绿。
- 根因修复后标准native-skill-runtime-02独立attempt真实exit0；同源binary12131993bytes/SHA5847b4473aeab0ba3a820d81de730e4340df87b63ed781d7dbef9f377f50597b与远端ACK一致。原报告47事件/22238bytes/SHA03da65d3d7c9686db3af2c0c2023851abf3072a171382a174d23ca1db0ce2c0d，重新只读检查原文件bytes/SHA与status一致，独立白名单readback同时保存。原01失败与清理未改；02实际首Ping就是ready，不能倒推01具体EOF或证明02发生过延迟。
- 112固定Codex0.151.0在同一个认证WebSocket进程实际skills/list发现所选/项目未选/六个系统Skill共8项原enabled=true；7次skills/config/write回执effectiveEnabled=false，forceReload再次读取只有所选保持true、其余7项false且errors=0。不是仅从上传配置推定启用范围，也不是第二个app-server进程。
- 同一新环境129c0dd0-ebd6-4862-aeaa-b656ff576aa2原Python/TS/42byte文件逐字节0600、取消原command e3be01d2…deadline/exit-1/stopped、native 38f10fd6…实际exit0/stopped；DELETE后GET404确认释放。实际原生用户代理为0.151.0/Linux；没有原生登录/模型turn/产品验收，权限不变。SYS_ADMIN/AppArmor/seccomp新增诊断仍待用户选择；公开Agent门禁、8795旧8912141及测试仓未创建新Pipeline的事实保持。

- 新版本标准verify17原进程已明确exit0：118100bytes/SHA407ebad416ca4b59147f010a15b4d5c282da2a03a5369bdd5d2463562669ed07，平台race66.281s，真实浏览器、供应链/SDK/部署/权限/恢复、go vet/race/build原门禁全部保留通过；包括共享就绪等待及同进程Skill范围原红→绿。准备维护源提交与标准8795升级，当前部署仍8912141，真实角色/模型/测试仓整链待完成。

### 2026-10-14 — 8795标准升级fad8ade并通过正式API回归

- 维护源提交10d00dc（原生桥接、模型约束和角色输入）及fad8adeb8eaae074f3833909fb755ce24f5fa37a（同进程Skill、共享就绪路径）。没有Platform PR/main合并或改另一脏工作树。精确源码tar4136960bytes/SHA02224bf0666634d1af535a58d0339b988936c1dfdb8d87be7fac5702763374e5与112传输ACK一致；逐文件对归档核验后标准Linux/CGO build-04 exit0、install-04 exit0。binary18965128bytes/SHA61552ab4f25e9420baa308c8452ae293877fcf60fdcf5d38151b4eb3ccb80aa4。
- 远端原升级前Run与配置/凭据摘要在副作用前持久化，原有凭据实际登录成功；本次同一进程升级前后原Run全JSON、配置文件、凭据文件逐字节相同，completed/released保持。此是本次真实比较，不补造03包装脚本丢失的旧比较。远端及Mac只读health均HTTP200精确fad8ade。112服务实例仍独立8795/8095，未改变容器权限/其他端口。
- 标准deploy/platform/check.py在已升级实例通过正式入口新建基础Workflow4769b9778f7bf636e5e1f622a6341d0f和Run96b7f9349c81e5719471caa895cd8c8d，原平台check-02 exit0；Python→TypeScript→Owner确认→Go项目命令→结束，共享同环境d61d40a7-2bab-4605-b552-628746a9008a，重复相同输入返回同Run，完成后正式release。可查看http://10.1.20.112:8795/workflow-runs/96b7f9349c81e5719471caa895cd8c8d。完整原报告摘要及每步回执正在独立只读核对；不以启动受理代表绿。
- 主原生Agent Execute入口尚未接通角色文件准备/产物读取，公开Agent门禁保持；真实Codex/gpt-6.1-sol推理及测试仓独立新Issue/Pipeline尚未执行。当前成果是可复用源码和标准升级/原生无凭据SDK/正式API基础设施证据，不是Model Relay产品验收。容器namespace新增权限一次无凭据诊断仍待用户明确选择；未自动增加SYS_ADMIN/AppArmor/seccomp权限，后续真实角色和测试仓入口继续按计划接通。

- 正式platform-check-02原报告32355bytes/SHA488108d6fbeeaa82277c3a055dafc9355dc0dbf9025e761171eee30f7a0bda35，独立只读核对步骤1/2/4各480/485/691bytes原日志均truncated=false、实际exit0/stopped，共5步completed、同环境最终released。scope明确native_agent/product_acceptance=not_run。该回归不是新产品Issue，不重复旧Run或另建状态源；当前权威计划继续保留未完成原生/测试仓部分。

### 2026-10-14 — 角色准备接续与正式Execute连接（公开Agent门禁保留）

- Ruling: 复用既有conversation/native目录的私有准备日志和远端最终文件标记，而不新增任务ID/公共环境对象或一套角色数据库。准备意图先独占写入/fsync，只有全部受支持SDK上传及最终标记读回相符后记prepared；中断/未知上传只读核对最终标记，不重放上传。缺/不符标记或已有未知home拒绝替换；原thread存在而准备记录缺失时拒绝新建home。代价是缺失最终标记的部分上传需要保留现场后正式处理/释放，不替操作者盲修。
- Run/实际sandbox/快照及初始资产摘要绑定现有c.ID；Role目录在共享/workspace之外，各角色原生状态独立，所选文件继续受32MiB/512件/64MiB边界。私有日志含配置/所选路径和初始摘要，不公开认证内容。原认证只在首次准备复制；接续不再次读取/覆盖host auth，保留sandbox里的原生认证更新。OAuth过期/多环境刷新及强杀断电仍未真实验证，不承诺自动续期。
- 独立权限API已有三个可改项NetworkAccess/AllowElevation/ApprovalsReviewer不作为角色资产身份；执行沿原thread/turn权限字段应用，其他角色内容不静默改写。六场景SDK隔离回归：正常复用、asset上传失联、final marker上传失联后只读确认、marker缺失、角色替换、原thread缺prepare记录。原编译红保持；green01 race4.436s exit0，后续读取时已移除宿主认证仍成功，外部写次数不增加。受限SDK文件夹具不是112实际角色/账号证据。
- 原正式Execute此前尚走宿主prepare；native-execute-route-red-02实际无远端launch/持久原生意图（旧宿主路径在/workspace失败），不冒称模型故障。Codex经启动注入已有Store查询workflow_steps既有归属；属于Run环境即按冻结连接准备角色、进入同源app-server桥接，否则保持既有本机执行。连接/归属/准备失败不回退宿主。
- execute主路径连同原bridge正常/取消/未知launch-stop、原模型绑定/身份/Skills回归race3.408s exit0；模拟Codex回复与认证仅用于SDK/协议边界。测试数据库连接绑定调整明确是临时隔离fixture，未修改112或产品数据库/检查点/历史。cmd标准入口已注入Store；标准verify18尚在原进程运行，最终结果待核。
- 112当前实际部署仍fad8ade、正式基础Run96b7f934…completed/released；本批新源码尚未标准升级。公开Agent门禁仍保留，须实际角色准备/模型和产物入口验证后再开放；测试仓新类型/独立Pipeline尚未启动。既有namespace新增权限诊断仍待明确选择，没有新增权限或改旧Run。

- 标准verify18原进程明确exit0，118095bytes/SHA8bb73a11bf41cf0010953075867891fe870abf84f3bd4c6f4eb00a251b3c7c85、平台race66.319s；原真实浏览器/供应链/SDK/部署/权限/停止恢复/vet/build均完成，本次正式Execute和角色准备隔离回归包含在全量。macOS linker warning保留。未因此开放公开Agent门禁或升级112。
- 已核固定release-1.1.1官方文件协议https://github.com/opensandbox-group/OpenSandbox/blob/release-1.1.1/specs/execd-api.yaml：/files/info缺路径为404，/files/download同样有404；只按404认定新home缺失，其他错误不当缺失重发。此为版本契约核对，不替112实际角色目录准备。下一项先通过无凭据受支持SDK入口实测私有角色准备/标记/接续，再补远端产物接口、真实模型及测试仓自有Harness/新类型互斥路由；新增namespace权限仍待原问题明确答复。


### 2026-10-14 — 112无凭据角色准备原层验证

- 可复用运行时探针通过正式SDK调用角色准备，并在删除宿主空auth夹具、修改宿主Skill之后确认接续复用原home/config/selected材料，没有重复上传。探针未发送模型turn，原initialize/Skill范围验证使用之前的app-server home，不能说该app-server消费了这次新角色材料；真实平台角色会话仍待验证。
- 同源Linux二进制12485594bytes/SHA256 0688f9309763d2cb4de1be3946ae77a72f9cb49f12a3a8b28f95fb7d771598c5；112 native-role-runtime-03实际exit0，49事件/22705bytes/SHA256 bfea78994030f3faa8d82126ae3fc55cf3dad69170303cec02528cc4e9bc388a，原报告和独立只读readback一致。native a129c2a5c656451eb2f9301bbd8c06f1 exit0/stopped，取消命令2aa735ec6bad47f492821ee616bbfd06 exit-1/stopped；环境a6d30e2b-4649-48c7-9f78-daa005c409b6已DELETE/GET404。没有真实凭据、没有更改容器权限。
- 本轮完整scripts/verify.sh真实exit0：verify-19.log 118090bytes/SHA256 ed85d1c9e577dcca8c2d9c5f315375b57de117976f01ade6a1aa2b234e0c5c97，平台全包race66.941s；目标回归先缺事件红、扩展夹具后绿3.882s。日志与原报告在ignored .data/opensandbox-verification，macOS链接器警告保留。
- 当前112平台仍fad8ade/8795；b814adf角色准备/正式Execute接线及此次探针源码尚未升级。产物接口仍读取宿主，下一步接通所属沙箱/workspace并验证路径、链接、权限及取消。公共Agent启动门槛保持；真实模型、测试仓新入口/整链尚未完成。此前namespace兼容模式需要的新增权限问题仍待用户答复，继续仅做现权限内独立工作。


### 2026-10-14 — 沙箱工作区产物API接通，112同源读取通过

用户影响与归因：会话文件接口原只读取宿主路径，沙箱的/workspace既无法正确展示，也不能借宿主同名路径代替。已根据现有step→Run归属连接所属环境，服务身份变化、环境未就绪/释放均明确失败；没有引入第二套工作区标识或产物存储服务。Ruling: 复用现有会话文件API及Run关联，在固定Python运行时用目录描述符读取 — 避免独立metadata检查后下载追随被替换的父链接 — 若后续支持不同运行时镜像，需重新验证或替换这条内部读取实现，不新增当前未用的公开后端配置。

- 正式列表/下载/HTML预览仍先授权；隔离HTTP+真实Python文件回归证明父目录和末端链接、越界/绝对/隐藏路径、不存在文件、超8MiB文件与释放状态拒绝。实际API读取远端报告/CSP保持opaque origin；跨用户及越界请求零远程读取。文件不可用独立返回artifact_error并显示，会话原消息与审批继续可读；不把文件失败变成会话不可查看。初次缺实现编译红及SDK事件类型编译红保留，目标green-03真实race5.587s通过。
- 固定读取仅在/workspace按fd/O_NOFOLLOW逐层打开，regular文件有界读取并检查读取前后size/mtime；目录5000项/64层/4096byte路径、文件8MiB、stdout12MiB/原SDK16MiB和30秒期限。此为即时查看，不是永久归档；释放/TTL前需要由流水线归档并核发布回执。产物持久交付及真实原生角色整链仍待后续验证，不声称文件永久留存。
- 同源无凭据112探针native-artifact-runtime-04实际exit0，二进制12502068bytes/SHA256 a29e992734f60f1382769db66a75138e681784b436e73bdd7336ea679e39583e（传输ACK最终一致，等待期间未重传/未往接收器发送命令）。原报告50事件/22876bytes/SHA256 f2ed2ccbc0e1d42cdc9e6218227fa7d32919ff3b1e4b83da3fc0f18c3b9aeea8，独立readback复核相同；同一产物读取实现列出两文件、sdk-proof.json42bytes逐字节正确。原native c5b6ec11e0844063b7ed5b20a559167c exit0/stopped，取消84ed320e20e945ceb3573266ab28cf27 exit-1/stopped；环境4cd83f34-193d-4159-b436-ba192b8f3491 DELETE/GET404。无模型turn、无真实认证、权限未改变；probe的platform_conversation仍NotRun，不能替代112正式会话API验收。
- 原完整scripts/verify.sh verify-20真实exit0，118101bytes/SHA256 9bfd27a10ca5ae16d1af6d6c44648ad5df1524504507b40e17097a4f63b806ef，平台全包race67.818s、vet/build及原浏览器/模板/安装测试通过。未因局部绿省略完整门禁，macOS链接警告保持。
- 1128795运行版仍fad8ade；本批角色/文件接线尚未标准升级，原完成Run保持。公共OpenSandbox Agent启动门槛未放开；Codex namespace兼容模式涉及新增权限的确认仍待答复。测试仓新Issue类型/仓库自有Harness/Pipeline和独立整链QA尚未完成；下一步维护通用仓库自有入站，待受信原生执行条件可用后从唯一新入口实测，不能复活旧Run或代产品验收。


### 2026-10-14 — 仓库自有Harness分发及独立沙箱入站

- 本轮先核正式GitHub：Model Relay旧入站blob仍5b3b98ae556954edc3ac5017391a564752985296，Open Issues37/34/33/32及历史项，无Open PR、无新同类任务。只读原结果在ignored model-relay-github-readonly-02.json；未创建Issue、PR、label或新的Actions，不把源码完成当远端应用完成。
- 标准export.py从固定Git对象导出SDK/Example及Go依赖文件，记录源commit/各文件bytes和SHA，拒绝链接、不规范路径及已有目录；不读未提交运行态。install_github_entry.py新安装自动附带项目.harness/agent-platform，已有工具包不自动重写；项目按源快照差异评审升级，项目定制由其Git管理。Ruling: 使用项目自有Git文件及固定导出记录，不新增平台模板分发服务/插件包注册表 — 去掉Runner对平台checkout的运行依赖，保留项目可调整的样例 — 若未来需要中央自动更新，应另行设计项目定制合并与版本治理，不能自动覆盖当前项目改动。
- 新run_entry.py用Python -I从同包SDK启动，两个入口在删除维护Git夹具后仍能--help；真实复制的Go MCP示例在同包go.mod/go.sum下构建成功。首次包漏Go依赖的真实回归红后修导出选择，定向0.669s绿；安装器漂移/重入/中断恢复仍在原门禁，安装器单测隔离export，固定Git对象/独立CLI构建由export回归实测，不能混称真实GitHub应用。
- sandbox_entry.py只把Owner授权的【Harness验证】Issue、repository与固定repository_commit交给平台，不在Runner克隆Agent工作区。label harness-opensandbox选择独立入口；旧模板和适配器排除此标签（含手动dispatch当前Issue校验）。Issue重投查原Run、未知受理不自动重发、拒绝评论不另起Run，受理不当完成；GitHub入站拒绝通知只发安全摘要。现有Owner信任边界不放宽；新入口模板需复制/按项目Runner调整后正式PR应用。官方actions/checkout v4 SHA经官方GitHub API核验为11d5960a326750d5838078e36cf38b85af677262，两个模板固定到该SHA并关闭持久Git凭据。
- WorkflowStart/SDK新增可选expected_environment匹配断言，平台先对当前或原Run冻结环境核对，拒绝沙箱请求误进本机；不授予权限或选择镜像。省略时omitempty保留旧请求指纹。编译缺字段红、一次验证误放读入口的编译红均保留，修回StartWorkflow后race2.072s绿；后续正式HTTP拒绝也纳入完整门禁。旧服务器DisallowUnknownFields会拒绝新参数，不能把未升级的112服务当新入口已可用。
- verify-21完整exit0/119478bytes/SHA256 4a3ef7ac4486b5a7054922259ec16ba1c6557801a2fd34f129ae1d89a0c4d5f4/race67.232s；增加Go包依赖回归及HTTP边界后verify-22完整exit0/119482bytes/SHA256 8f5ec520b5d8c2788053f40cc16443f0012b8987081e39b777bf52605b748043/race67.849s，原浏览器、vet/build、SDK与113模板测试通过。之后仅澄清入站失败提示不代表测试结果，ruff及5项入站定向再次绿，未改变平台执行语义。日志在ignored .data/opensandbox-verification，macOS链接器警告保留。
- 本批尚未通过112标准升级应用，也未给测试仓安装新label/YAML/私有配置。8795仍fad8ade，旧服务与已完成Run未改；公共Agent启动仍保留门槛，namespace新增权限确认未收到。后续先实际标准导出/安装核验和隔离8795升级，再按已验原生条件配置唯一新Pipeline；不能让协调者代产品实现/QA，也不能把仓库包或脚本测试绿冒称真实流水线验收。


- 仓库自有入站本批提交368feb2321b84747cbc417f63a056de689325b93后，实际通过标准install_github_entry.py在ignored独立Git项目夹具安装legacy和sandbox两入口；每种首次Installed/exit0，原命令重入Unchanged/exit0。真实导出140文件/1113879bytes，SOURCE.json源commit同368feb2且全部文件bytes/SHA重核；两个仓库内入口Python -I --help真实exit0。原回执repository-owned-install-01.json保存，夹具没有平台请求，也未改真实Model Relay仓库。此为标准安装/分发路径验证，不是GitHub部署或原生Pipeline通过；112仍待新版本标准升级。


### 2026-10-14 — 112标准升级86add21及正式API基础链路复验

- 当前112隔离平台8795已由标准Linux/CGO build-05及install-05升级到86add21dbe1ea0271be5da577547029c3717dbc5，两者实际exit0，远端及Mac /api/health均200同revision。源码归档4270080bytes/SHA f33b4253acb5101e3e32a191bb682c608e3e2acedb9021b9ee28dda90d204f0d与传输ACK一致；binary19363912bytes/SHA 7c1041e6da5d217f9d8567d0aca7e20f41d4085412ef4532e544fe79a91e1121。副作用前持久化的旧Run、配置及凭据逐字节/逐值不变，现有凭据实际登录成功，旧Run仍completed/released。未改变其他端口或容器权限。
- 标准deploy/platform/check.py实际exit0：Workflow a4cbc4e1049d5fab4501dc8a199bb361、Run 2d151104936beceb48cfd04a48cbe798，Python→TypeScript→Owner确认→Go项目命令→结束。重复请求同Run、共享文件与完整输入成立；步骤1/2/4日志480/485/691bytes、truncated=false，各实际exit0/stopped。所属环境bd15854f-841d-461c-a529-1de994f1b562由正式release进入released。原报告32359bytes/SHA c52c482c46565f2e0c2d90b7b57ab24dd4406556c8c475fb3fc8e96737c37c9f，独立原文件只读核对一致。查看：http://10.1.20.112:8795/workflow-runs/2d151104936beceb48cfd04a48cbe798 。
- 已安装API的expected_environment边界实测：向该OpenSandbox流程传local或unknown均400，对应稳定请求键查询404、没有创建Run，原完成Run不变。标准check-03正向请求省略该可选字段，保持旧调用兼容；不将此描述为非空expected_environment正向112验证。源码/SDK的正向字段及冻结去重已在verify-22中验证。
- 本轮已部署角色准备/正式Execute、沙箱产物与仓库自有入口源码；公开OpenSandbox Agent启动保护仍保留。真实Codex/gpt-6.1-sol模型turn、正式角色会话/交接、新测试仓Issue类型及Pipeline尚未执行，native_agent/product_acceptance明确not_run。原namespace权限扩展仍待明确授权；没有增加SYS_ADMIN或放开AppArmor/seccomp。现有SDK探针启动的app-server也未消费新角色home，此证据缺口仍需原层补验，不能把文件上传成功冒称角色执行通过。
- 原始回执保存在ignored .data/opensandbox-verification/platform-check-03-readback.json及112私有platform-upgrade-05/platform-check-03/platform-environment-guard-01文件；不另建公共状态源，不改旧回执。完整源码verify-22结果保持原119482bytes/SHA8f5ec520b5d8c2788053f40cc16443f0012b8987081e39b777bf52605b748043；此次仅部署与记录，不称新产品QA通过。


### 2026-10-14 — 正式角色材料被112同一原生进程读取

- 用户影响与归因：先前runtime探针把角色材料准备到正式home，但实际app-server仍使用/tmp探针home；文件接续成功不能证明角色材料被原生进程使用。已将标准探针改为先调用正式prepareSandboxRole，再在唯一原生进程中设置同角色HOME/CODEX_HOME；不新增模型替代、公共配置或第二个进程。宿主空auth删除/Skill改动后的接续仍复用原材料。生产执行源码未因探针改动改变。
- 定向回归真实红native-role-consumed-red-03（246bytes/SHA9f6aad00b478aa248a73b6314bfd3ad755524e2833ced43677c115d7048d9707）证明roleLaunched=false；green01 race2.226s/exit0。red01缺bytes import编译失败、red02受本机端口沙箱阻断，均单独保留，不冒充行为红。后续标准verify23实际exit0：119472bytes/SHA2e53a30bb7f7f3bf110b20b4e0f4bed530c425eb76998e773e17da278f2c5109，平台race67.091s；原浏览器/供应链/SDK/113模板、vet/race/build通过，macOS linker警告保持。
- 同源Linux探针12501684bytes/SHA7e6ec5781e9392ab6649f580a4ae1374801697e6db56d29f33e9ba020503ecb6与112传输ACK一致；native-role-process-runtime-05实际exit0，44事件/10425bytes/SHA c27024d4dcf5e5617c5ddd1f47560f0789852c3d81fe023bec7fc707e20541f0，原文件及独立只读readback同摘要。正式角色2fec7855…home中的selected-0/SKILL.md与原生skills/list返回路径完全相同；该同进程仅发现所选/项目未选两项，未选项effectiveEnabled=false，forceReload后所选仍true/未选false/errors0。没有把早期系统Skill七项禁用事实套用到此角色配置。
- 唯一native进程3acd5524b7c742509c4b110f13ba266a实际exit0/stopped；取消命令7f1082c83f32439f822bc0cf2b3a45fd实际deadline/exit-1/stopped；所属环境dbf150cd-a946-4c74-a0f1-4dbfbe656e65 DELETE后GET404。没有真实认证/模型turn/平台角色会话，此证据仅关闭“准备的Skill没有被原生进程实际读取”的探针缺口，不证明角色指令推理、原线程重启连续或产品验收。权限、镜像及已有8795实例保持。
- 最新112平台仍86add21（本轮探针改动尚未标准升级）；公开Agent门槛仍保留。真实模型、namespace权限兼容诊断、新测试仓Issue/Pipeline和独立整链QA仍未完成，原新增权限选择尚未收到明确回答。原报告、完整回归、readback保存在ignored .data/opensandbox-verification，不补造旧证据、不重试原native任务或恢复旧Run。


### 2026-10-14 — 测试仓自有入口草稿PR38与实际门禁发现

- 实时官方GitHub API核对Model Relay main仍0c197c8b3d98346107e65dbf0c41ef5d6b73d437，无既有同目标Open PR/新入口/标签。原入口blob5b3b98ae…仍执行平台checkout脚本。首次gh只读clone长期无fetch回执，精确所属进程终止后exit143；第二次90s有界HTTPS直连实际exit128，日志证实github.com443连接失败，而api.github.com读取成功。未推定凭据或产品故障、没有第三次盲重试。原未完成目录/日志保留；Git调试日志确认未出现Authorization头且失败发生在连接前。
- 改用正式GitHub Git Tree/Commit/Ref API，按当前main固定树生成Harness-only候选覆盖集，不冒称完整产品checkout或产品QA。标准安装器在API候选文件集生成仓库自有SDK/Example（140件、源b1032c9逐件bytes/SHA一致），两入口Python -I help exit0；21项入站回归通过。生成146文件/1146510bytes覆盖集，新tree ead4ba3bda0ec14876aa038213b941d28da2e14a逐件Git blob SHA/mode匹配、原业务blob全部保留。候选人工审阅原入口后以实际旧YAML摘要记录迁移来源，没有伪造平台回执/数据库。
- 测试仓分支codex/harness-opensandbox-entry、head997b08c345423b251867fe5ad9b7f09f671993b6，草稿PR38 https://github.com/big91987/model-relay/pull/38 已创建并附当前任务；main实际不变。远端146文件，现有两项修改为entry YAML/source manifest，其余为新增工程包及说明；旧Runner/配置变量/部署入口保持，新类型采用独立标签和配置。未创建标签/新Issue、未安装实际Agent Pipeline、未合并或部署，不能把草稿当流程已通过。
- PR38首head实际GitGuardian failure/check115483812620，1项base64_basic_auth、无行号annotation；原私有明细定位到复制的controller_test.py假git job-token夹具。证实是测试用身份与10字符假token编码，未接触真实认证；仍保留门禁红，不关闭扫描或加忽略规则。唯一维护源改为运行时base64生成，同一fetch认证/服务不含token断言保持，原2项认证回归exit0/2.530s。改动待固定导出同步同PR并正式检查，不能先写GitGuardian已绿。
- 额外最小诊断发现旧适配器可通过excluded_issue_labels=[]取消新标签隔离；实际sandbox-route-override-red-01返回(7,0)而应拒绝，813bytes/SHAe1869a7b34faa23cea64d3f0dd117932218152f3f38b20456db41dead0cbf343。维护源将保留标签隔离先于旧选中标签/可选排除项判断，legacy forward始终拒绝新类型；新适配器仍按所选新标签接入，空排除/双标签及手动路径覆盖。green01 22项exit0，425bytes/SHA52fca4feb8da6db037b89dddc0582fe5c4640ab63eab8d5a3e25ada99e76b57a。没有为某个Issue硬编码状态。
- 标准verify24真实exit0：119655bytes/SHA1cd751631cf88e65357c8509cc913ff5e54fba0cd91da815b90ec6bc9590de97，114模板测试及原浏览器/SDK/部署/vet/build通过。Go源码未改，race复用了有效缓存，不写成新跑67秒；前verify23实际race67.091s证据保持。最新112仍86add21，原namespace新增权限问题尚未获答复、真实模型/新Pipeline/独立整链QA待完成。下一步先标准重新导出本次通用修复、更新同PR38并核实际门禁，不重复PR/Issue或先启用未配置入口。


### 2026-10-14 — PR38标准同步完成与当前安全门禁绿

- 通用修复已提交唯一维护源bb698c9abb949e02b3c757f9924ad61b05f9d1ca；固定export.py导出新版本后，逐件核对原项目候选无漂移，只合并4件源差异及SOURCE.json。140件源文件当前bytes/SHA均匹配，项目副本22项入站测试exit0。测试仓同一PR38快进到48fbfe28…，新tree ef28b1d77099ae34d5ee87fc84b6dea3e0026a5b与源库存逐件Git blob/mode核对；没有新PR/Issue或修改main。
- 临时操作包装脚本在已确认ref更新后，对PATCH返回PR元数据的head/draft断言未过；原响应字段未落盘，不能把原因猜成缓存/异步或平台故障。只读reconcile实际确认ref与PR head均48fbfe28、正文完全相同、draft=true，未重复PATCH/提交。后续清理脚本的一次尾斜杠repo元数据URL404发生在任何写入前，原脚本保留，改官方规范URL后继续；不把此当仓库消失或权限升级理由。GitHub Connector独立认证读该私有仓仍404，已有CLI身份的官方API保持成功，未改变连接授权。
- 48fbfe28的GitGuardian/check115490164161仍failure；私有原check明细明确报告的是初稿997b08c的假常量，当前文件源码已不包含固定编码。没有关扫描/加忽略/假造成功。参照官方GraphQL RefUpdate.beforeOid原子前置条件，将自己唯一未合并草稿分支整理为同一tree的单提交acd96f752fa5dab0e009d597a12babe71d7bb765、parent为原main0c197c8b…；仅当实际旧head=48fbfe28时更新。真正回执及ref/main只读核对通过，所有源/项目文件内容相同，原失败check和本机原回执保持；没有无租约强推或修改运行/原生历史。官方接口依据：https://docs.github.com/en/graphql/reference/git 。
- PR38现远端head精确acd96f7、draft/open、146件范围保持，main仍0c197c8b…。当前GitGuardian Security Checks实际completed/success；其他combined status=pending、Actions列表空，不能统称全部CI/部署通过。当前正文同步实际源版本、22/114入站/模板回归及首失败处理，source24 Go未改/缓存命中与原23实际race67.091s分开。
- 当前状态：已实现、已做维护源/候选局部测试、已标准分发到真实仓库草稿分支、当前安全检查通过；未合并、未在112升级本批Example源码、未建立实际新Agent Workflow/配置/标签/Issue、未运行真实模型/Pipeline或独立整链QA。112平台86add21和原基础Run保持；原namespace兼容诊断新增权限仍待明确答复。后续仍需源计划中MCP参数/结果引用与节点配置核验、原生条件、正式新入口和真实研发交付验证，不把本次草稿准备缩减为最终目标。


### 2026-10-14 — 执行节点使用任务参数与前序结果（维护源验证）

- 用户影响：原 MCP 执行节点只有固定 JSON 参数，项目不能直接将任务输入或前序 Agent/工具的结果传给下一工具，容易退回每次人工拼接。Ruling: 在已有 execution 配置中增加可选 bindings（工具参数名→已有节点/JSON Pointer），复用 Run 输入和前序步骤；不新增变量注册表、表达式语言或第二套工作区身份。代码/命令共享现有 WORKFLOW_INPUT_FILE，新增 run_id/workspace_path/步骤seq与status；parameters仍是原字符串键值契约。
- 引用保持 JSON 类型，缺值不使用固定参数兜底；只读同 Run 最近一次前序执行，最新一次缺值或未结束时不回退到更早结果。保存校验节点与引用格式，运行时按实际值重新校验完整工具 Schema，校验失败前不写 action intent、不发外部请求。内部凭据、连接、Owner及Agent home不进入可引用输入。画布提供可选参数引用 JSON 编辑、示例和现有节点列表；保存/关闭重开/重绘/导出保持草稿，无效内容阻止保存。具体契约见 workflow-session-input.md。
- TDD：行为红 red-02真实exit1（236bytes/SHA2961f89d78a8b82dbb1eaffeac820eb5a9492cf3d7c8d8e6538d047e215aac8d），原节点在保存阶段丢引用/缺参数失败；green-01真实race2.848s/exit0（314bytes/SHAafba7bc24e479e1697554522fd112bd5755f6541dd79e08dd2dc4cd651fa8b5b），官方SDK真实本机MCP HTTP两次调用传递任务和前序原始结果，重复入口同Run。缺字段/对象替代string均失败且0外部调用/0intent。typed回归另覆盖0/false/null/数组转义路径、原始JSON整数精度和最新/未来步骤拒绝；不将原始精度隔离回归扩大为所有MCP服务的精度保证。首次测试stringmap类型编译红、green-02本机监听被沙箱拒绝分别保留，不冒充产品缺陷或有效业务红。
- UI回归red-01实际exit1（3982bytes/SHA17a9e04ee2756329e5cebccb53b3ffa3809b2da815668b72f402a70e835328a2），green-01实际exit0（5065bytes/SHAa3aa7a02ef9e58f50a5b31cdf759ee818ec0247459b86ecdd53d022b854e9a0a）。原实际Chromium测试新增引用保存重载→平台API启动→外部MCP收到input→原始回执展示→释放环境；390截图52036bytes/SHA6222d2a060b4be3a6bd459a03385f9c1d75683b48afcc9bdbfa9bda5ccef85a5已查看。此为隔离平台/真实HTTP工具服务回归，OpenSandbox服务为夹具，不是112原生Agent或产品QA。
- 标准verify25实际exit0：119764bytes/SHA28ff5a6d9e6854d854bea47bb1b92c5a2695b057b03c20375e17f5cb0f9066d5；Go race实际68.656s，原114模板、供应链/SDK/部署/浏览器、go vet及两binary构建通过，macOS linker警告保留。完整日志及截图在ignored存储，未补造历史回执。整个分支最终独立复审尚未进行。
- 实时核对：112 /api/health200仍86add21；PR38仍draft/head acd96f752fa5dab0e009d597a12babe71d7bb765，当前GitGuardian success/main未改/Actions为空。新参数引用源码尚未112标准升级或真实新Pipeline验证。namespace额外权限诊断仍无明确答复，公共Agent启动门槛保留；真实模型、新Issue/Pipeline及独立整链QA均未完成。下一步标准升级本批源码、112正式接口复验，然后继续原生条件与测试仓独立入口，不重复创建或恢复旧任务。


### 2026-10-14 — 参数引用版本112标准升级与正式执行复验

- 已提交维护源ae4a2e98be436636ab3d2ca8c9cc0a576fe4b54a。固定Git源码归档4352000bytes/SHA824915fa26d176b1b44bf352227b6efa981a3e7c72610198df7af90ab879d505，112完整传输ACK一致；标准Linux build-06、install-06实际exit0，仅升级8795实例。实际安装binary19383824bytes/SHA8008686bfdaaef8dfd9fdbbb5bf38b63813c5370df5bead336df733d7c73f256；远端独立只读readback及Mac /api/health均同ae4a2e9，旧Run/私有配置/凭据逐字节或逐值保持，原凭据登录成功。Mac实际服务workflows.js（54468bytes/SHA006decb3f2370368b277c55d540807a18b7b1f2be669498fe329e8f1c3cd1107）及workflow-model.js（16300bytes/SHAbeea0711ad691f5bd372104a0dd68320a166165ad19281142784627959ade869）逐字节同已验源码；不把安装成功当新增MCP引用的112执行通过。
- 标准check.py从新源码、真实平台API启动唯一基础设施Workflow63baf06928b62b97da17e8a5838d870c/Run39d8defcee76a31c1a6dee21644d9650，实际exit0。同环境4260dd97-bdab-4a02-9585-95e5cb361eb0完成Python/TS/Owner确认/Go项目命令共5步，重复入口原Run、最终released。原报告32345bytes/SHAd8753215494466a8d9c950b413d213da3573d5eb332472c43a6a78548dbabf73，独立readback核实际步骤1/2/4 exit0/stopped、日志480/485/691bytes/truncated=false。此轮验证共享输入契约变动后的真实沙箱基础路径；没有原生Agent、MCP引用112实操或产品QA，也没有修改旧Run/账号/模型权限。正式入口：http://10.1.20.112:8795/workflow-runs/39d8defcee76a31c1a6dee21644d9650 。
- 当前剩余：公共原生Agent门槛仍保留，namespace新增权限诊断待用户选择；模型turn/原生角色会话及恢复、新测试仓Issue/Pipeline、整链独立QA和整个分支最终评审未完成。PR38仍草稿，没有重复Issue/PR或擅自启用新类型。安装/readback/标准运行均为Harness证据，不替Pipeline执行产品验收。


### 2026-10-14 — 已部署112上的MCP任务/前序结果引用实操

- 计划Task3的引用112实操已通过。没有修改平台源码、旧服务、原生权限或产品；在112独立验证目录构建受控echo工具，复用维护源固定Go MCP SDK1.4.1及stdio协议，通过正式管理员API注册/发现，工具仅向自己的0600调用日志写入脱敏测试字符串。此为真实进程/SDK/平台API基础设施回归，受控工具服务在平台侧，不能描述为真实供应商或沙箱原生Agent验证。
- 正向Run3147ac52b0e6f8ba717b53fcedb82bc9实际completed/seq3：第一节点从本任务parameters/message取值，第二节点从第一节点最近回执structuredContent/text取值；原引号/换行/$(not shell)均逐值相同，固定literal兜底未被使用。相同request_id重复POST返回原Run，独立工具原日志确证总调用2次。此次通过真实HTTP启用expected_environment=opensandbox的正向输入，真实创建所属环境，后续正式release状态released。
- 反向Run6677a90913e8f53873db74c551b54aa8实际failed/seq1，错误为text引用不可用，execution_dispatched=false；工具原日志仍只有2件，即缺输入新增调用0。该Run所属环境也经正式release到released。两个Run原始失败/完成历史均保留；检查后唯一受控工具通过正式API停用，不删除回执或配置，未创建测试仓Issue。
- 有界原检查进程真实exit0；原report52644bytes/SHA208dbf7b7ccb86b4cf2704fc7a2ed655d55173e35514ae200a82fb4d5b887b33与独立文件readback核对。随后展示包装脚本把已关闭TextIOWrapper当Path调用read_text触发AttributeError；仅影响把日志打印到终端，不是平台/产品失败。原包装脚本和driver log保持，另做只读原文件及正式API核对，未重建工具、重复Run/调用或更改原回执。当前API independently confirms两Run上述状态/环境已释放、字面结果相同、tool.enabled=false、health ae4a2e9。readback位于ignored .data/opensandbox-verification。
- 完成范围：维护源全量verify25、实际浏览器隔离回归、112标准升级以及新增引用112正式实操。仍缺新增引用112网页编辑实操、原生模型会话/命令/恢复、新测试仓独立Issue/Pipeline和整个分支最终独立评审。公开Agent启动门槛及namespace权限阻塞保持，待用户对新增权限诊断选择；不能以这条受控MCP回归替代目标中的原生研发流水线。


### 2026-10-14 — 引用配置112实际网页保存重载与原生权限等待

- 基于已部署ae4a2e9及原受控MCP正向Run，通过固定镜像内Chromium147.0.7727.15访问真实112:8795页面；登录凭据仅私有stdin传入，没有写入脚本、日志、截图或Issue。只编辑自有验证Workflow的同值引用JSON并保存，实际PUT200、重载第一节点任务引用及第二节点前序结果引用逐值一致；原Run冻结定义/回执逐值保持不变，未重新启动Run或重复工具调用。
- 浏览器实际查看原第二步原始字面返回和已释放状态、390窄屏Run。browser-check06真实exit0/无stderr/page_errors=[]；1440画布PNG120679bytes/SHA c3d321f94242d064263fe98934e2a130f47bce774fcee79453ff21aa322675ca，390Run PNG59849bytes/SHA73bc2f71a32403f4663bc7dac0dbefb4141adc63c8af1e143eb0f5d7e35c4afe，两图按原摘要接收并查看。画布截图展示MCP配置上半部，引用字段的保存/重载由真实DOM及HTTP断言证明，不假称截图展示所有字段。仅在保存检查期间正式API启用自有受控工具，结束已停用；唯一带所属label的浏览器容器已退出/不存在，没有产品操作或原生modelturn。
- 原操作包装生成时一次过宽字符串替换造成Python语法错误，发生在向112发送任何命令之前；改为单独占位符并先compile后才执行。图片接收首包装仅取250行缺起始marker而IndexError，原传输已完成；随后只读现有2000行终端范围取得两张原图，同bytes/SHA，没有重跑浏览器或重发图片。原错误工具记录/脚本保留，这些不是平台/产品失败。
- 当前受支持网页、代码/命令、MCP与基础环境路径的已实现验证均保留；整套目标仍未完成。原生Agent启动校验仍拒绝，namespace权限已证阻塞，未扩大权限或使用宿主fallback。已向用户重新列明唯一一次无凭据诊断的具体边界（新容器/独立8095/官方isolation扩展/SYS_ADMIN与该容器AppArmor/seccomp；不改宿主/无凭据/不privileged/不挂载宿主，检查结束释放），等待明确选择；自动goal继续不当作权限答复。后续不能以更多基础探针替代实际模型会话/新测试仓独立Issue/Pipeline和整链验证。全分支最终独立评审尚待实现与实际门禁满足。


### 2026-10-14 — 持续原生权限阻塞审计

连续三轮目标检查均无实质进展：未收到新增容器权限诊断的明确用户答复，原生Agent启动门槛保持拒绝，真实模型会话/新测试仓Issue触发Pipeline无法继续。已实现部分的正式API、浏览器和标准部署证据未改变；最后只读检查112:8795健康200/revision ae4a2e98be436636ab3d2ca8c9cc0a576fe4b54a。PR38仍草稿，未合并或启用新入口，也未创建空任务或恢复旧Run来绕过门槛。

目标状态记录为受阻，保留现有服务、分支、草稿PR、原失败和验证回执。待用户明确选择一次新无凭据容器诊断（SYS_ADMIN及该容器AppArmor/seccomp放宽）或保持原权限评估更窄方案，再继续原生隔离、受信配置、真实模型/新Pipeline和整链验证。自动续跑不是答复；不继续添加基础探针来代替剩余目标，不宣称整链完成。已有权限问项仍待答复，没有自动审核拒绝事件；阻塞来自未授权的新权限边界及未完成的原生验证。


### 2026-10-14 — 用户确认由OpenSandbox承担任务隔离

用户明确同意按外层OpenSandbox任务隔离、平台授权/审批/Gate、Codex外部沙箱执行方向继续。原一次SYS_ADMIN/AppArmor/seccomp诊断不再采用，旧阻塞等待该诊断授权的后续动作被此决定替代，原失败历史保持。Codex0.151.0原生生成Schema包含turn/start及command/exec的externalSandbox策略；官方App Server文档明确用于已经隔离server的客户端跳过Codex自身沙箱：https://learn.chatgpt.com/docs/app-server#command-execution 。当前thread/start的sandbox枚举仍只有read-only/workspace-write/danger-full-access，不能把externalSandbox当CLI sandbox_mode字符串或关闭所有审批。

Ruling: 受信OpenSandbox执行入口在已有transport中显式传入externalSandbox策略；宿主stdio仍使用原workspaceWrite/readOnly，任务输入和Agent原生TOML不能自行选择外部边界。平台工具授权、审批、生命周期、原始回执与去重均保留。不新增通用权限Profile或Sandbox ID；复用所属Run/environment与角色快照。共享容器不能直接兑现节点级只读和断网，当前拒绝这些配置的外部执行并说明限制，不能继续显示限制但实际不执行。

实施顺序：协议边界TDD（实际turn策略、审批不变、缺强制边界的配置在副作用前拒绝）→同源无凭据官方command/exec实际验证默认容器策略、取消及释放→标准维护源升级→正式模型/角色会话→测试仓独立Issue入口与真实Pipeline/恢复/QA→最终独立整支评审。公开Agent门槛先保留，实际原生命令条件证明后以受支持源码校验替换，不以手改运行态开启。整个目标仍未完成；此次用户继续是新的明确方案授权，阻塞审计重新开始，不能沿用旧三轮计数。


### 2026-10-14 — 外部沙箱原生路径源码与112无凭据命令证据

- 唯一维护源接入externalSandbox：仅经过所属Run/环境/角色上下文核对的OpenSandbox Execute入口传入该策略，宿主stdio仍使用原sandbox策略；原模型绑定/原生会话身份/审批reviewer/Skills范围/生命周期及回执保持。read-only和禁联网的节点当前不能由共享外层兑现，在Role材料准备/上传、native意图或Run/环境创建之前拒绝，不悄悄弱化设置。画布同一Agent表单在OpenSandbox显示“允许修改任务环境”和限制说明，角色与任务输入仍分离。
- 原协议边界red-01真实exit1（1461bytes/SHA678817c7fcbd0de144142d8715397d229de021150822f0f66bcfc7128bbeecba），旧turn仍workspaceWrite/unsupported配置已联系环境；green-02真实race4.013s/exit0（314bytes/SHA8a4ffe679047a931f6049fd7a9af48447303ad0ef755d6f53da0088122e4f72e）。启动red-02真实拒绝支持配置“native executor integration is not yet configured”（684bytes/SHA58f5557ba564a8142d2c8275cded23feab6f2abd9e0d47424f8fca43e907f5b4），green-01 race4.334s/exit0（313bytes/SHA232aebc720c7badee462d269386f9e101d6be8c5e2d3792a865abd0eb55c0a2c）：支持新Run/重复去重，限制配置拒绝且0创建请求/无Run。首启动夹具edge模式不匹配和首probe选择器未匹配测试均保留，不冒充有效业务红。
- Runtime原测试red-02真实exit1：原probe未做外部原生命令检查（515bytes/SHAa59dfddd556f9f9820153ada6d9092eef4a6dc4333cbd967ad7fd327f3f07d77）；新增同一原生进程command/exec受控Python读写+Docker socket不存在断言，不发模型任务。Linux同源probe12511658bytes/SHAcd58e4c3a0259f5011ddd3553083834ea2cab77f2b1e9da00f70a97abb0ba3b1，112传输ACK一致；native-external-runtime-07实际exit0、45事件/10793bytes/SHAd8ae022f1e4131dff5cf8fc66a0035a7dabb99d69c2024ac77273b6e74fea1a2，原文件摘要与独立readback一致。原生command/exec明确externalSandbox/networkAccess enabled，exit0/stdout EXTERNAL_SANDBOX_COMMAND_PASS/stderr空。默认容器创建参数与容器权限保持，无SYS_ADMIN/AppArmor/seccomp放宽、无模型凭据。
- 原生进程09763ed4e6124e5a9cab40ae67bd2cf2实际exit0/stopped；受控SDK取消4bb846eaff334393b440ae57fefba9f3 deadline/exit-1/stopped；唯一环境62d5cbb3-b269-4a34-9c33-ed1e7af724e4删除后GET404。所选Skill/原角色home同进程核对及原文件/取消回归保持。本证据证明官方外部命令模式可用，不证明模型认证、正式平台角色会话/重启或产品QA。
- 中间verify26实际exit0/119766bytes/SHAf9f19b74fe2f7bba2bf6a4fa6e1ddc1060f9db5223320ec10bf2c0e55897db34/race71.818s；其后新增公开启动门槛修复与可见说明，最终verify27再完整执行actual exit0：119752bytes/SHAdeec669f8b603b0c443af1c1d3750af8f90b57c86bf95d30af6ee078f8bd7791，平台race72.606s、114模板/原浏览器/SDK/供应链/部署/vet和两binary构建通过。前26不当作后续候选验证。macOS linker警告保留；完整原日志/只读readback在ignored存储。
- 当前112平台仍ae4a2e9，接入外部原生执行/公开受支持启动的新源码尚待标准升级。旧提权诊断不再等待/执行；用户已确认新方向。后续立即标准升级到固定源码版本，正式模型/管理工具交接并按原完整目标继续测试仓新Issue/Pipeline与整链验证；整支最终独立评审仍未完成。未新建空Issue、未复活旧Run或虚报产品通过。


### 2026-10-14 — 外层隔离版本正式升级及原生接续验证开始

112:8795已通过标准build/install升级到74c2cbe8c3551ab6465b485b7c903c0dce103e7c，build/install均exit0；独立登录与API读回同revision，binary19388208bytes/SHAeea77c8fcef152d41a6bef2f97f96727161a1632717388d653ce3d0b7af281fb。instance配置/凭据逐字节保持，旧completed/released Run d0988dbd35a5218864aba04a8f4939e1完整对象不变。无新增容器权限、无旧服务改动。

唯一正式原生验证Workflow135f114b8228759aaefd7574bdda7151/Run a97ab0a4a49e6a927a5a65e2164c8a6e，使用Codex/gpt-6.1-sol、明确联网及auto_review，受控文件→原生wait_for_input→同会话补充输入→complete_node→Python核对共享产物→结束。任务仅Harness设施，禁止产品/供应商验收和读取秘密；稳定request_id重复提交返回同Run。最初诊断夹具误传节点Agent name被正式API400拒绝，先检查没有已有同名Workflow才删除非法身份字段重提；该400不是平台缺陷，不隐藏或冒充模型失败。当前首轮等待真实执行，不把创建/排队当成功。

源码回归复现轮次令牌误入角色资产摘要：Claim每轮更新WORKFLOW_NODE_TOKEN，旧prepare将其视为固定配置，接续会误拒绝。最小修复只在受信workflow_node绑定同名凭据引用时从资产摘要排除该动态值；启动仍带当前轮令牌，普通环境设置仍冻结，原Snapshot不修改。无扩大AGENT_PLATFORM_/CODEX_等保留变量权限；先前摘要中关于该令牌带AGENT_PLATFORM_前缀的推测经源码核对不成立，实际名称为WORKFLOW_NODE_TOKEN。首red受本机监听权限限制不作行为红；red02真实exit1仅轮次更新失败，green01 race2.682s/exit0；完整verify28进行中，修复未部署，旧准备日志不改写。模型认证、真实接续、测试仓新Issue/Pipeline及最终整支评审仍待证据。


### 2026-10-14 — 真实原生轮次网络阻塞及正式停止

Run a97ab0a4a49e6a927a5a65e2164c8a6e已通过所属环境准备、原生进程启动、thread/start及turn/start；原始事件中的thread为01a1383d-ae38-7451-9dce-a7b4d9bd3bb5、turn为01a1383d-ae73-76e1-be7e-e59693b7c889。原生多次responseStreamDisconnected/httpStatusCode null/Connection failed: error sending request重连，没有文件用例执行、wait/complete接续或产品结果。早先仅查Conversation.thread_id空不能倒推thread未启动；须以原始native事件为准。

已通过正式Owner stop(seq1)，平台Run stopped；唯一native process f06c6bbf007e439aa86dfa93e2cebbe7 independently confirmed stopped，环境2b2f6991-d651-4195-88b5-6deaeeb4d3b0保留ready/原TTL3600，不重启/换模型/无限重试。无凭据最小诊断先核Docker所属Run label/镜像/非host网络：112宿主和所属容器对chatgpt.com/api.openai.com均DNS成功、TLS失败OSError，无HTTP/HTTPS/ALL_PROXY；非容器独有故障。尚未证明网络策略、路由或代理哪一项是根因，不归因为凭据错误或产品故障。已询问112既有出网代理/放行方式，所需地址或配置位置未取得前不自建隧道/扩大网络边界。

本轮源码候选全量verify28 actual exit0/119756bytes/SHA945aa4d7995325a59cc6e8e8e5aed5ce5ff45b7a73e799aaad1d32a880e4dcc5，race73.416s及原浏览器/114模板/SDK/部署/vet/build保持绿。轮次capability隔离回归red02 628bytes/SHAea68466dd3d4dd52a30b2968f6200dc87073dc568714d07f36cb674fb14bb240真实exit1；green01 313bytes/SHA27feeacf6e6449443d2f9a1d49028f327a329c846c718f6f7354948c6dcb53a8/race2.682s。原准备journal保持冻结，不原地改其资产摘要以救在途会话；此前版本已准备的角色若摘要冲突，仍明确拒绝，沿Owner return建立明确新执行或新版本验证，不静默替换原生身份。当前新修复源码已验证，正式集成仍待标准升级，真实接续受网络阻塞尚未通过。


### 2026-10-14 — 接续修复标准部署与独立读回

维护源a000d1d21c1305559a89df2223ad40b75433ba10，经标准源码archive4433920bytes/SHA13e8b79d48294453f33e9d74fdbec0350c3f8e45b10549fb75c572f8182649ca传输ACK一致后，在112通过deploy/platform/build.sh与install.py升级8795，二者exit0。独立API健康同revision；实际installed binary19388880bytes/SHA870798fccb2f6727a1af972136eddb719ad9ccdda2e53bb0eb99779dc7c91a89。原配置/凭据逐字节保持，旧completed/released Run d0988dbd35a5218864aba04a8f4939e1及本轮stopped Run a97ab0a4a49e6a927a5a65e2164c8a6e均完整对象不变；独立原native process仍stopped。未改旧journal、未发送新模型轮次或替换会话。当前维护源修复已正式集成，真实模型接续仍受出网阻塞，不能以源码/部署绿判通过。

112私有原材料已独立重算：native-continuity-08b.jsonl9597bytes/SHAd78a7b5dbfabf6540da93ed7ff4f70a4633f701f567c49be6468dd76b60a4bb5；正式stop7516bytes/SHA54aa77d5839ae8ee3054b9aec45fd43eb308d9280cdb212ea7fc7ba3a203ab85；原事件19646bytes/SHA7e95f1645e7c9ce5d7d2553a280a6fa55649ded8b2b8e704c3a87fc6f980560e；无凭据network726bytes/SHA8f28290c2eb91beae0ac025026e5f98ecb7cc57f36c84f8a87ad12601d8cb97b。JSONL元数据只说明实际存档，不能补写未执行的产品用例或真实接续结果。

下一步依赖112可用的既有模型出网代理/网络放行信息，用户问项已发。取得后通过平台正式环境变量配置/新执行入口验证，处理旧role材料冻结兼容并保持原始身份，不手改配置快照或日志；随后仍须测试仓独立Issue/实际原生研发Pipeline、交接/QA/失败恢复和最终整个分支独立评审。当前没有新产品任务或真实产品结果；不再把之前SYS_ADMIN诊断当待授权项。


### 2026-10-14 — 用户提供代理后真实模型澄清/接续闭合

用户明确提供http_proxy/https_proxy=http://10.1.20.20:7890。通过既有Agent环境JSON正式API把大小写HTTP/HTTPS代理及仅平台112/loopback的NO_PROXY保存到自有Workflow135f114b8228759aaefd7574bdda7151/revision2，重载逐值相同，旧Run08冻结对象不变。不是手改服务/数据库/快照或继承全部宿主环境。宿主无凭据HTTPS经代理可收ChatGPT403/OpenAI421，此只说明TLS/HTTP可达，不当模型鉴权通过；后续原生模型实际执行才给出模型可用证据。

正式新Run7289bb0437b61703c5c1eda68fd0978c/revision2：Codex/gpt-6.1-sol在所属OpenSandbox创建字面fixture并调用wait_for_input(clarification)；平台waiting，原生进程2bea21f8419a4c1a8a90dea353000f84独立stopped。正式User Input NATIVE_CONTINUE_08接续，重复同request_id返回原message64；同conversation7cb83885cdf160316908160256a24528、同原生thread、私有轮次令牌确实更新，第二native process436287cd8f544cd785554c989a077bc2独立stopped。Agent读取原fixture并更改confirmed，通过complete_node进入固定Python检查，原完整output EOF/truncated=false含NATIVE_CONTINUITY_PASS，受支持artifact API读回{"phase":"confirmed","literal":"$(no shell)"}，结束seq3/completed并正式释放environment/released。

独立正式API读回同完成/释放，原Run08完整对象不变；native事件221件/native.error为空。原proxy配置/创建报告16879bytes/SHA03390857c48b7ecc346d4f20a58604dac5866c5beeabdbee1d8ca8311a19fe72；完成报告18644bytes/SHAee6a146d1fc53b442c6c1c598292106c31f268ab47c879eb56b80e100949b7b2。首核验包装误把平台刻意不公开的thread_id当API字段，KeyError发生在POST input之前；原报告4104bytes/SHA4fdb03488a0afffd09cd91464b1eeed31ef286836627638ffb8e860865c50079保留，先确认只有before_input才继续。原生身份与令牌只在本次独立核验通过mode=ro/query_only私有存储比较布尔值，不输出令牌或修改DB；产品逻辑及输入/交接/产物/停止/释放仍走正式API/原生工具。

模型出网旧阻塞已通过用户明确代理配置及真实轮次解决，不继续等代理或SYS_ADMIN答复。实际证明模型执行、澄清接续、共享文件、轮次令牌修复、去重、complete_node与环境释放；没有产品QA/DeepSeek调用，也没有把此设施Run当测试仓正式新Issue/Pipeline。112仍安装a000d1d。

测试仓实时读取首直连TLS超时未做写入；切换用户提供的代理后官方REST读回PR38仍draft/open/headacd96f752fa5dab0e009d597a12babe71d7bb765、146文件、GitGuardian success、main0c197c8b3d98346107e65dbf0c41ef5d6b73d437保持、Actions空。不盲合并或创建重复Issue。112只读凭据可用性核验：backend GH_TOKEN/GITHUB_TOKEN均未配置、gh CLI/标准hosts文件/credential helper均无；没有读取或转移Mac GitHub凭据。后续仍须完成私有仓库材料准备的受信配置与项目自有步骤，独立原生多角色/QA/返工/报告/新Issue触发整链和全分支最终评审，不以更多fixture替代这些尚未完成项。


### 2026-10-14 — 私有仓库初始化采用执行节点受信环境引用

真实原生接续09已完成，测试仓新Issue/Pipeline尚未启动，缺口是112执行步骤没有私有仓库认证入口。本轮复用Connector已有env_refs语义用于代码/项目命令：图中只保存运行时变量名→backend配置引用名，任务参数不选择凭据，值在action intent之前解析，缺失即拒绝，归档/展示按本次捕获值脱敏。MCP继续使用已有工具注册配置，不新增凭据Profile或项目专属平台脚本。Agent不继承此凭据。

Linux标准安装增加实例自有可选0600 runtime.env及受管理EnvironmentFile；升级仅接受原标准unit的精确旧格式，配置/账号/data保持，外部编辑仍拒绝。项目初始化脚本复制到测试仓后按项目调整，固定repo/commit、使用进程环境认证克隆到同Run /workspace；准备/验证/QA/发布仍各有真实回执。这是受信运行时安装配置，不把手工注入环境作为临时运行救援。功能定向红绿已记录，完整验证、标准112升级及测试仓实际入口仍待本轮后续证据。


### 2026-10-14 — 执行凭据、未知命令恢复与项目自有流水线源码闭合

维护源新增执行节点env_refs与画布保存/重载、选定值的跨SDK输出分片脱敏、缺凭据时零派发；标准Linux安装接入可选0600 runtime.env并仅迁移精确旧unit。新Example包含同Run固定仓库准备、研发/固定make verify/独立QA/人工Gate/报告/同版草稿PR，项目必须复制并调整，不在平台维护每仓脚本。准备与发布获得受信Git引用，Agent/tests不继承Git凭据；模型代理单独配置。QA/report改为非隐藏harness-output，正式发布后保留原文到docs/harness/run-*，草稿PR不代表合并部署。

唯一新鲜整支独立复审发现4项Important：非原生命令停止未知可返工重叠、Git pushurl导向、已提交文件逃逸发布范围、隐藏QA产物无法查看。修复及同一reviewer复核补充准备未派发/只读归属查询失败恢复、历史删除/还原秘密和暂存后工作区删除的边界；全部已在限定源码范围关闭，无剩余Critical/Important，真实Pipeline证据仍须另核。

命令复用原action(run,seq)记录command_id和command_quiet，在只读归属/endpoint查询之后、RunCommand之前持久化可能派发，init保存真实ID；实际stop或独立状态核对/正式环境释放证明quiet，原receipt不改写。已发后未知阻止Stop声明完成、Return/Resume/新工作；明确上传或前置GET失败仍可保留工作区正式返工。SDK endpoint有缓存，定向夹具改为同阶段真实归属GET503，不把原未触发endpoint注入误当功能通过。原red command-stop与undispatched保留；green06 race2.370s/exit0，SDK原接口保留。两次回调签名编译失败及一次夹具旧seq冲突均保留并纠正，不当产品失败。

发布检查固定base以来全部线性提交、实际index和有界blob，复用既有秘密扫描；拒绝合并历史另审。认证Git固定HTTPS仓库、禁重定向/Hook/本地URL及凭据覆盖。真实本地Git夹具验证预提交漏文件、历史秘密删除/还原、暂存秘密删除、外部pushurl均在网络前拒绝。自动审批首拒绝复制GitCLI token/App私钥，说明个人权限可能较广；用户明确选择允许这两项复制至112:8795用于本测试流水线后才继续。另一次大操作审批超时未执行，拆分已授权源码写入与需要监听的回归；没有绕过安全拒绝。112私有配置已按一致10240bytes传输回执安装600，App仍Issues-write原范围，临时传输包已删除；值未进图/Agent/日志。运行服务尚需标准新版安装读取该配置。

最终full-verify34 actual exit0，平台race73.526s，浏览器/SDK/模板/供应链/部署/vet/build全过；完整日志121681bytes/SHA792f697395784ab77091308fe7428070b1a3057e48ac042ffa12efee04399c68。full31/33为此前候选绿；full30导入lint、full32本机监听EPERM及其他定向编译/夹具失败保留，不能覆盖。当前112仍a000d1d，PR38仍draft，尚无新Issue/产品Pipeline结果；下一动作固定提交标准升级112，再同步项目自有模板/标签入口并启动唯一新Issue，不停在源码绿。


### 2026-10-14 — 实际Workflow安装暴露外部通知门槛缺口

112已标准部署0b482a7（build/install exit0，health同revision，binary19415088bytes/SHA728db1fedb583ace3b3ff34072bee1e85c305b3e2c5e3798db494c6a7153a7e6，配置/凭据/旧Run保持）。正式Installation注册caller model-relay-sandbox-8795/user_id93424060a89a41b464b1653bc2e21664及通知Connector成功，随后POST /api/workflows被400拒绝：sandbox workflows use execution nodes for external actions; host Connector hooks are not supported。未创建Workflow/Run/Issue，原caller/token/manifest保留，不重建。

用户影响：沙箱流水线不能配置GitHub原生消息/审批通知，缺少Issue当前卡点反馈。源码证据validateWorkflow blanket禁止Hook，且Connector校验假定宿主存在工作区；已有Hook实现其实仅支持Github Issue评论且拒绝body_file，不执行任务代码。最小修复为控制面外部通知保留现有授权、冻结repo/凭据引用及消息去重，OpenSandbox仍禁止宿主Connector节点/命令/文件读取。动态Issue编号复用既有issue_parameter扩展至评论；缺值拒绝。新增显式run.failed生命周期事件，只回写节点/运行入口，不公开私有错误、命令、路径或把设施失败解释为产品不通过；已恢复未发送的通知跳过。此缺口先进入唯一维护源及标准升级，再继续同一安装清单；不删Hook绕行或加临时通知脚本。

通知缺口定向真实red（旧Hook禁令）→green05 exit0，覆盖受信Issue参数、无宿主工作区、拒绝BodyFile、目的地修改/禁用、失败去重与正式Stop后跳过未发送通知。green01本机监听EPERM、green02遗漏请求准备路径及green04夹具未使用Connector新revision均保留并纠正；不当产品测试失败。最终full-verify35实际exit0/平台race76.266s，完整日志121675bytes/SHA163e56431f4801fc99c711077298552aaf3168f941ff152bab55a2c7e1062b14，web/浏览器/SDK/模板/供应链/部署/vet/build全过。静态复核控制面评论路径仅保留受信授权/冻结目标，未扩大执行或文件权限。待标准112升级及原清单复验；当前尚无新Issue/Run。

### 2026-10-14 — 隔离实例升级与项目自有入口正式集成

112:8795标准build/install11均exit0，health=3261b340a7d3b20b29f524739d346dd1b4025cda；实际binary19419592bytes/SHA19cf7d909a4c1735c4494046e0e24e0c4e3e1f20758b8e2f7fa5ad98e55b9491，源码归档4546560bytes/SHA0119cb12a6d225ba1a9ef986347c624627e1ef13f0e5f9ca6381e43777693917。配置/账号凭据/runtime.env/App私钥/CI token/原清单及三旧Run逐值保持。原Installation清单接续安装唯一8节点Workflow83f08f81f7040fcd0c12e6ff4e2acd2d/revision1，含Agent/执行/人工确认/结束；原通知Connector与caller复用，原400回执不改。独立caller读回只见该Workflow，旧Run访问拒绝；私有文件均非空600，仅统计布尔/字节不泄值。

标准源导出146件，项目副本隔离Python两入口help exit0、122项模板回归18.466s全过。原PR38精确新head17e889b1862000cc5f5c73ee783a751d98f5abe4核对146件Git库存/SOURCE及GitGuardian success，ready后授权精确合并main003b686ce4d32b5367c20b54333c3aa33c66bc00；不创建新PR，不合平台main。正文PATCH即时head断言未一致，保留原响应失败；随后只读回执确认分支/head/body已一致，不盲重发。平台源3261b34已推独立codex/opensandbox-workflow，无平台PR。专用harness-opensandbox标签已存在、新Workflow384118653 active；main自动prepare38537594630 in_progress，未作为新Pipeline或产品验收。

实质卡点：把112新caller token传回本机Runner私有配置被自动审批拒绝，理由为上一授权仅涵盖现有GitHub Token/App私钥向112复制，没有这枚新令牌反向传输目的地授权。该动作未执行，也未换命令或其他凭据绕行。已经单独请求用户授权；新caller仅用于此Workflow，不是admin且Agent/tests不继承。待明确许可后再加密传输、配置新仓库变量、核Runner受信SDK访问，再创建唯一新【Harness验证】Issue。当前未创建新Issue/Run，尚无Model Relay新流水线的实际研发/固定测试/独立QA/报告发布结果。实际native09是隔离Harness连续性回执，不能冒充新项目Pipeline。已准备具体Issue验收，含同工作区、明确QA故障注入返工、原固定make verify、人工确认与同版草稿PR产物；等待入口授权，不代产品。

### 2026-10-14 — 持续推进长期里程碑（用户确认）

用户确认测试仓Harness构建PR可合并，要求持续自主推进平台迭代与测试仓验证，普通选择不等待逐项确认。第一里程碑M1：通过项目自有GitHub Issue入口完成一轮真实OpenSandbox Run，实际私有仓库固定版本准备、原生研发/固定make verify/独立QA明确返工与重验/人工确认/报告与真实同版草稿PR达到旧版等价或更好，再报告并讨论下一阶段。M2：沿同一能力验证重复投递、停止恢复、外部失败、材料保存及在途升级，不重复任务或重放未知副作用。M3：另一仓按项目复制定制/标准安装升级复现，清除一次性衔接。M4：依据前述真实缺口做画布、参数和节点职责减法，维持最少必要模型，不预建通用框架。详细证据继续沿四现有文档，无重复状态源。

本会话已创建对应活跃目标；原暂停model-relay自动检查更新为本会话每小时Harness里程碑推进，正常等待保持静默，只有重大结果/修复或需介入的阻塞通知。112新caller传回Runner动作在用户新持续授权及已实测单Workflow范围证据下重新自动审查批准，同目的动作加密执行，没有换凭据绕过旧拒绝；私有目录700/配置和token600、不输出明文，临时加密私钥已删除。独立新仓库变量AGENT_PLATFORM_SANDBOX_CONFIG已读回一致，原变量保持，专用label此前无Issue。Runner正式SDK/HTTP预检新图环境opensandbox/8节点/只见一图/无Run。首探针误用SDK不存在call方法无HTTP，第二把caller安全脱敏env=null当可迭代值导致TypeError，两项均诊断后纠正；原配置并未缺失，不判平台或产品失败。当前准备从唯一新Issue接单，不以预检或目标受理代替M1。

### 2026-10-14 — Issue39真实入站首红与通用网络诊断修复

唯一Issue39/id5845759121已通过专用标签创建；opened/labeled正式Actions38546350797与38546350712均在SDK转发步骤失败，原请求键github:big91987/model-relay:issue:5845759121查404，无Run/产品副作用。首完整官方job log14222bytes/SHAfb533f165e50a1a43a450f63bcb91caa0b26ee837367c0db9822e08099153424保留；Issue原github-actions[bot]通知错误笼统为入站材料/配置不符，未公开底层类别，不能确认两次精确根因。协调者明确来源的去重卡点评论6166118614附原Actions/影响/证据/假设/修源后接续动作，标准marker防止被当成Owner新任务；不是Agent自报或产品QA。

最小只读实证：同机GitHub CLI默认读Issue39为transport_error，用户指定代理读成功。源码sandbox_entry未接既有git_proxy，顶层异常抹掉分类；red13两项明确失败→8项sandbox_entry green12通过，124项模板21.971s全绿。首red12夹具patch错误命名空间导致只读外部调用，已纠正后才取实际红；未写GitHub或平台。修共同Github子调用选择函数供旧/沙箱入口复用，主进程/平台API网络不改变；读取/固定源码/提交阶段安全记录，错误只保留HTTP/传输类别不泄响应/私有路径。专用Runner配置通过原标准save保存既有git_proxy选项，未改全局网络。真实只读预提交链已达submit并明确拒绝POST；此使用OwnerCLI只读而非Actions token，不冒称正式恢复通过。full36因import排序实际失败保留，已修后启动full37；待标准导出项目升级、精确Harness修复PR合并，再正式dispatch同Issue/原请求键，不盲重试或另建Issue。

full37最终actual exit0，完整日志122971bytes/SHAb68c05709cfdaabf1f25f0c4b016b0d981d17c881eb6e8d6a2205dc85e6d20ca，124模板及浏览器/SDK/供应链/部署/vet/build全过；Go源码未变化，race采用有效缓存（最近实际full35平台race76.266s），不伪称重新实跑。准备固定源提交/标准导出并应用项目入口；当前Issue39尚无Run，不以源绿冒称M1。

通用模板修复已提交并推平台开发分支f4ded463db0e401d5ad242b53cc1f3adb3fdab56，未创建平台PR或合源main。标准导出146件，项目副本124回归19.362s及两入口help通过；PR40只6件Harness文件/说明，精确head e5684fbfc189902626bef4efd74535d1ecb29f7b、GitGuardian success，授权ready+合main9e5531b1149e00748013f25e9672bd5dd6041dc4，146Git库存与SOURCE一致。原PR38保留已合历史，本PR40不是Pipeline生成的交付PR。平台Go二进制未变化，仍health3261b34；无需假装重装同代码。已查原请求仍404后正式单次dispatch同Issue39/source main9e5531，等待原回执。新dispatch受理不代表Run/固定测试/独立QA完成，不重试未知结果。

### 2026-10-14 — 真正原生整链首Run暴露浅历史准备缺陷

原恢复dispatch38550547157实际成功并创建唯一Runb0dfee0c70121cc2709e881fb94690e5/workflow revision1/source main9e5531；准备code1真实exit0/2636bytes，工作区归属/固定HEAD成立。研发2独立原生模型完成授权缺章说明及快照，Git凭据存在性false、无产品更改；固定tests3/5/7真实exit2且各有完整未截断回执，全在review-history baseline unavailable exit128，尚未到Node/Go产品回归或独立QA。研发8已主动wait_for_input(blocked)抑制第四次重复，原生4进程独立stopped；Owner正式stop8已读回stopped，未手改DB或图。当前阶段不是通过，也无交付PR。

原因实证：冻结prepare代码git fetch --depth=1，native8只读历史1/缺66f1328ada75ea90e5638fbf2bd58663ac86347e及9b6994ed80de2bdba79ff267419150dccaac1360；GitHub官方compare均behind0、确为固定HEAD祖先。首compare助手误把端点三点分隔当路径穿越，在发请求前拒绝；已只对白名单GET双40位SHA开放，未增加写权限。此为通用Harness准备缺陷，不改产品历史门禁、不给Agent/tests注入凭据。真实本地Git祖先夹具red exit128→移除浅fetch后3准备回归green，固定HEAD不变、同Run重入不重播fetch且未提交文件保持。

Ruling：旧Run的inline准备代码和图已冻结，不能用平台新源码冒充旧Run已应用修复，也不手工补git对象。先白名单归档原Agent产物/失败日志并正式取消释放，标准源/项目模板升级当前Workflow，再建明确替代关系的新验证Issue/Run（沿已认可的取消/新环境入口）；不扩造通用Run迁移模型、不复活旧39。第一M1以新图实际完整Run与PR为门槛，旧失败和关联保留。其代价是新增一次明确替代验证任务，不能拿旧Run改状态伪造恢复。正式源修复/新完整门禁/标准安装应用尚待后续。

旧39原20件Agent产物通过正式SDK逐一保留2055158bytes；三轮原state均gate_exit2/Source_before=after6e359667…不变，cleanup保持其原“attempted; candidate_removed”措辞，不补写强杀证明。源command logs seq1 2636/SHA08a8eb86…，seq3 665584/8bafdb20…，seq5 665461/26a37053…，seq7 665542/7a41647a…全部EOF/truncated=false；inner runner与outer SSE帧长度另列不混。文件列表API400泛化unavailable/changed/limit，单一声明文件read_file通过，同文档SHA775923a4…与native8一致；此API列表范围/错误分类缺口保留，不能声称目录浏览全部通过。按声明路径精确读取是受支持只读接口，无秘密或临时写入，旧候选与流程状态不变。

完整源full38actual exit0/123506bytes/SHAf546ebfbef8ca84a45727811ff2b5d0ff722b768a1ab3d34b50f085abbb5dbc6，125模板/浏览器/SDK/供应链/部署/vet/build通过，Go不变且race有效缓存。新模板默认固定HEAD加完整祖先历史，同已归属源重入仍不fetch/checkout或覆盖文件；研发SOP补实质修复/新证据才返tests，环境/凭据/冻结步骤缺入口就blocked。待固定提交/项目标准导出/Workflow正式升级；M1尚未通过，无Pipeline生成PR。

### 2026-10-14 — 完整历史准备标准升级与明确替代验证

平台/模板固定源2db4c4c68a05a1bf7b81297ea4111bb059250bac已标准build/install13 exit0，health同revision，binary19419592bytes/SHA96554205d7135ac94b7c63c9ed1a723223c5e34d9a637b2d0a8470f0f16c235f，原账号/配置/私有runtime和CI、三个旧探针Run及已取消释放39逐值保持。当前Workflow83f08f81f7040fcd0c12e6ff4e2acd2d经原Installation清单/在途检查显式upgrade变revision2；首缺upgrade参数的标准拒绝保留、未写Workflow，随后13b正式升级成功。未给在途Run改图。TTL/镜像/权限未改变，资源和环境寿命仍有界。

项目125模板回归通过，标准146源件及4个白名单旧失败证据文件应用PR41/head e761d045871ee632abcc803dc7c244098597e417；10文件范围与全部Git库存原字节核对、GitGuardian success后精确合main5ff456cb6de0ce8d45bb2faac4c964cda9efcbf0。两原AgentMarkdown原样、派生summary明确标注白名单而非改写回执；旧39原固定测试和QA未完成，材料发布不代表旧publish节点通过。

取消替代规则下新Issue42/id5847349237/【Harness验证】完整历史准备后验证工作区共享、QA返工与草稿PR已正式创建，body SHA7fba80cce141389a4b9c7128928e6ed008dbf7a5614a71f58a071bad58fa4a21；明确替代39，同原目标/故障注入，当前图revision2。39补正式替代评论后关闭state_reason=not_planned，不作Done/产品修复；旧Run取消释放8步、原回执和原生历史保留。新的实际Actions/Run以正式API更新；M1仍需新Run真正完整冷make verify、独立QA/返工复核、人工确认及Pipeline生成的同版PR。平台源仅开发分支，不创建源PR/合main。

Issue42原事件已实际进入唯一Rune65c964ce9faf29e35271270d751cedb/Workflow revision2/source5ff456cb6de0ce8d45bb2faac4c964cda9efcbf0，environment11916390-01f5-4451-8268-a9f92463e46d/ready。新prepare1 actual exit0，完整3036bytes/SHA89bddeca8628396840564f4e82ad59deaec1f0d9985e3360078e310770cdc434/EOF/truncated=false，新完整历史代码已冻结。development2/conversation04b5c850fb1364bc1a4b37b8948544b4正在执行；该受理/准备不冒称历史回归或产品QA通过。当前M1须继续完整固定测试→独立QA/返工→人工确认→报告→真实同版草稿PR，旧39取消/失败证据保持。

定时锚点更新被自动审查拒绝：认为先前停止后缺明确重新开启授权，本次更新未执行，未改接口绕行。实际文件只读仍为本轮此前已成功创建的ACTIVE/本会话绑定；此拒绝与运行阶段许可无关，原Run42和活跃目标正常继续，不能把该审查误写成产品审批或暂停执行。按里程碑结束后再确认定时检查的后续安排，不以此中断当前已授权研发。新Run42/prepare1和development2持续以正式API核验。


### 2026-10-14 — 新Run历史解析冷环境故障与正式同节点范围接续

Issue42/Rune65c964ce9faf29e35271270d751cedb原生development2有界传输重连后实际完成并handoff tests3，未新建会话或Owner盲resume。tests3原固定make verify真实exit2，完整667798bytes/SHAbf9b038e40f26a9cbea9ba9e5105c0d1fa1f0eb68e630df7cf9c06b80a5c39e0，正式API读EOF/truncated=false。历史基线现在可执行，失败从旧缺祖先变为historical HTTP unparsed baseline output；158件源6e359667…前后不变。Node/Go全部产品门禁、独立QA尚未完成，不能当新绿。

流水线自动failed→development4/conversation0adbb26f372fb3fc84e5983edff41453，实际最小诊断发现go test -json合并stdout/stderr，冷依赖下载非JSONstderr触发分类器解析拒绝；真实旧HTTP断言按预期失败，诊断不是固定测试通过。Agent主动报告范围外验证器需blocked，无重复全套求绿。Owner通过原workflow_message提交稳定request_id issue42:seq4:historical-json-stream-remediation，补充同任务最小项目验证器修复：Pipeline分离机器解析和完整stderr、覆盖噪声/无效stdout/真实执行故障，保留原断言/历史分类/全部门禁/期限和首轮缺章QA注入。此为项目测试设施修复，由Pipeline实现，不由协调者代写产品。输入已正式受理，尚不等于处理完成或交接。Issue42正文PATCH并远端逐值核对，明确现状/范围/未验，未新建重复任务。后续真实局部红绿→新源完整固定测试→独立QA实际返工/复核→确认→报告/同版PR，M1仍未通过。

正式范围输入此前已入队，研发4调用blocked时返回409 request conflicts with existing state，原生明确未冒称等待成功；随后同conversation自动承接入队输入并开始真实红例/最小修复。此为排队输入优先接续的受支持路径，不是Owner改状态、重新Run或手动恢复。原409与Agent说明保持，仍待修复handoff和完整测试。

研发4已实际完成9个命令边界回归红→绿，并真实历史HTTP重建通过；候选验证器main.go SHAeaa988821fccf011d9fc63795ea5933f3f7acb734ffe22830cbff8f6eafcf355、main_test.go SHAebdd0554efd7f73155324f9d5034e71a72fe1baaced6747c066485f5443a8bfa，Builder正式read_file只读保存，未代跑产品测试或QA。静态核对stdout只JSON解析、stderr完整诊断、非exit1执行失败拒绝、原历史预期断言与额外设施错误分类保持；全部门禁及后续独立QA仍须实际完成。缺章说明原SHAc0ec7667…未提前修复。Issue42实际8条回写含github-actions受理及big91987-agent-platform-bot[bot]节点/原生回复，固定tests3明确退出码2，研发阻塞/范围接续为真实通知，不由协调者模拟机器人。

研发4正式handoff tests5：新158件working_tree_sha256=12d91213e103d69050c427579d36ec20595d5284a9ab6d75926761380784de70；只改项目验证器两文件与实施文档，Makefile/锁/历史基线/产品行为不变。9场景race与旧assessment、定向vet及真实历史HTTP重建实际绿，局部结果不替tests5。正式read_file保留14个声明原件703857bytes，白名单manifest记录逐文件SHA/bytes；不把协调者本机副本当Pipeline正式报告发布。重复原范围输入request_id一次返回duplicate=true/message1599/原conversation，Run无新执行且已自然进入tests5，真实消息去重及同会话接续成立。M1完整原门禁/独立QA/人工确认/PR尚待。


### 2026-10-14 — 同Run新完整门禁暴露路由时间窗口

Issue42 tests5原固定make verify actualexit2，正式31页读至EOF/truncated=false，outer1010274bytes/SHA60424cbf22527af1423cdd7a79ebcda0ad9f3e638b927b7871df364f43576630（inner项目日志另列，不能混字节）。158件12d91213…源前后不变，历史JSON/原历史HTTP分类已过；实际当前失败仅TestRoutingRealHTTPSelectionFailoverAndRecovery“cooldown did not exclude actual primary”，历史健康断言红是预期基线重建，不混作当前产品缺陷。Node/全包部分执行不代表完整门禁或QA通过，browser尚未执行。

正式自动failed→development6/conversation629b1f4645116065a655c6bc7d4c824a；Agent发现既有15毫秒时钟窗口，尚未证实精确根因或产品缺陷。Owner正式稳定输入issue42:seq6:routing-failure-scope/message2307已受理，要求实际最小归因；仅已证测试fixture可移植性可由同Pipeline最小修，保留真实HTTP/冷却排除/回退/到期恢复断言，优先既有可控时钟与阶段边界、不增sleep或retry/弱assert。产品错行为须独立产品修复目标，不在Harness验证中静默改业务。原缺章/同版回执保留，下一步新源完整tests→独立QA，M1仍未完成。

研发6真实受控回退耗时实验：原deadline2026-10-14T12:46:50.634931478Z，主供应商实际selection_observed12:46:50.650582133Z/reasonselected/primary_delta1，原断言仍报cooldown未排除；同请求后续失败会重设冷却而出现新cooling记录，不把最后decision覆盖首选择事实。该实验支持测试真实15ms窗口到期后误判，未显示产品冷却决策错误。定向修后真实HTTP已绿，隔离候选“忽略冷却/拒绝到期恢复”反证仍执行中；全套及独立QA尚待。Issue42已补当前失败/范围说明并远端逐值核对，两个旧失败保留不升级Pass。


### 2026-10-14 — 有界资源声明缺口与正式停止保存

当前环境ready12:08:57Z、冻结TTL3600，实际多轮故障诊断后剩余寿命不足完成两轮完整测试和QA返工/报告。Owner向研发6提交寿命阻塞前先核实时状态，发现已自然进入tests7，因此旧seq6输入未发送；未误投或改图。正式stop7先stopping后stopped，原receipt exit-1/732771bytes/SHA65e34de812773a85c97294a453448dcdc286110c9940d7521412f77bf9752f2a、EOF/truncated=false。5个原生进程均实际stopped。先用正式read_file保留42个声明原件1792904bytes，再保留7内层state/log共735483bytes；共44件2528387bytes。state终12:57:47Z/gate_exit-1/exit1/log731984/SHA59c8ae42…，cleanup只保持原“attempted; candidate_removed”，不扩称全部子进程已清理。随后正式cancel7及环境release，实际Run cancelled/environment released；7步/原图/TTL和回执不改。未QA/未Pipeline PR，不能冒称M1完成。

研发6新源158件a6f4ec9a2a4e9f87f501aee7f48251ccede244d46082fc09b415ea9953a03b3e，仅项目3个测试设施文件加说明/实施记录；真实定向HTTP绿、忽略冷却/永不到期/期限减半3反例各红、vet绿。fixture通过配置1h及精确生成期限建立未到期阶段，无在途请求时锁下调整测试拥有的内存deadline建立到期阶段，保留真实HTTP/40并发SWRR/回退/计量及选择证据，移除sleep、不改业务。旧tests5当时没有选择时间戳，不能倒推唯一原因。扩展HTTP集14.569s signal:killed退出1保留；之后cgroup memory.max2147483648/peak2147495936/oom=1/oom_kill=1，证实环境曾触顶并OOM，但缺前计数不能精确对应某个子进程终止。仍需新完整原门禁与独立QA。

平台缺口：固定2CPU/2Gi及1h默认，对当前工作量没有可声明内存和受支持的已有实例寿命更新。最小修复落源：标准部署增加有界内存选择、显式--update-environment仅更新内存/寿命；省略时保留已有选择，新Run冻结内存、旧库0值保持2Gi及旧API原字节形状。CPU/容器权限不扩，未新增Profile/自动无限续期。安装持锁贯穿准备和激活，拒绝并发及外部unit修改，精确旧unit升级和同参数中断续装有回归。安装CLI缺选项真实2个red、实际SDK请求选8192MiB却仍2Gi red、旧Run误继承8192 red、并发安装覆盖和激活阶段失锁red分别保留；后续green05安装13回归及Go定向3绿，green02缩进错误为源码修复过程失败，未应用运行环境或误当产品结果。完整源门禁与标准112升级尚待。用户已授权普通选择自主推进，沿现有计划正式应用配置4h/8Gi到新任务；原Run不改生命周期参数。


源完整full39 actual exit0，日志124855bytes/SHAdfd2bd52d369c7808a014aac7ae184666403846f85a28ba71853412adf80616b，125模板/浏览器/SDK/供应链/部署/vet/build绿，Go race73.023s为本次实际执行而非缓存。唯一既有final_branch_review对新增资源配置整版只读复审C/I=0，冻结兼容/安装并发/激活/中断恢复与实现一致；不等于产品QA或M1。标准112升级及实际新环境资源/替代Pipeline仍待。

## 2026-10-14 15:22 UTC：资源配置已正式应用，Issue44接续候选

112:8795标准升级到 `f409c0a6f4e3208100291b8a88459a937ea6d4ca`，build/install均exit0；binary19414688bytes/SHA256 `45104bcad8da5332dcd490059e7f00c4d5f206e1c04873a9b270ac2c7526b9ca`，health同revision。显式标准更新仅改变新任务TTL14400与memory8192MiB，CPU仍2；旧Run、原生会话记录、私有runtime/CI/App凭据逐字节保持。源完整full39为exit0/124855bytes/`dfd2bd52d369c7808a014aac7ae184666403846f85a28ba71853412adf80616b`，125模板测试及实际Go race73.023s，独立源码复审C/I=0；不是产品QA。

正式基础探针Run `2ed0baa2cc460bd7abb8852f9576dae5`通过Python/TypeScript/Go共享工作区、同请求去重、操作者确认、结束与释放；真实cgroup memory.max=8589934592，初始oom/oom_kill=0，Run冻结ttl14400/memory8192。原报告32768bytes/SHA `5eb56d0c87db330b93a34c36764d72ad7785abfcd147d7536faee8aee4ae90d1`；该探针不含原生角色或产品验收。

Issue42已closed/not_planned，原7步及cancelled/released保持，最后中断不是Pass。Pipeline原写三个测试设施源码及两份研发说明按manifest原字节转存测试仓独立候选[草稿PR43](https://github.com/big91987/model-relay/pull/43)，head `1d581652736fbac4e8862b70367181c8a9f2f8b2`，6文件远端Git blob逐字节一致。此为未验收的候选接续材料，不合main、不算Pipeline最终PR；两份旧报告引用的私有harness-output未全部公开，接续说明明确限制。

既有可信入站配置通过标准save仅将base从main改为`codex/issue42-test-fixture-candidate`，本地Runner与112记录一致，其他字段/Token引用不变。唯一替代[Issue44](https://github.com/big91987/model-relay/issues/44)明确候选来源、原失败、4h/8Gi、首轮缺章→独立QA拒绝/返工→原固定测试→复核→人工确认→报告及主分支目标草稿PR。opened/labeled Actions `38588363589`/`38588364193`均success，正式request `github:big91987/model-relay:issue:5850470252`只关联Run `744c4329facce31be37c97e7caa2fea3`；prepare1完成、tests3归档副本误入编译失败后development4已修，tests5内层12分钟到期，development6预算与采样兼容修复已正式handoff，tests9完整冷门禁exit0，独立QA10审查中，环境 `afedacfd-e324-41c2-83f3-29c5239c9840`实际ready，冻结TTL14400/8Gi。下一步等待原生自然handoff及完整make verify，不代产品、不重跑未知副作用；M1仍未通过，无Pipeline生成PR、QA或新部署。

上次仅回读安装报告的自动审批因stream disconnected未执行；重新提交同一只读动作审批后成功，并非绕过拒绝。自动化配置沿已有状态保持，本轮未重建定时任务。

本轮续记15:36 UTC：Issue44 tests3原固定make verify真实exit2，711632bytes/SHA256 `57656ede7f32969ba6e8e62578bbec244989a9b5a507ee1a0da77f1a76fa178f`，正式output分页至EOF/truncated=false。原历史回归已越过；新失败为本轮研发把归档`.go`副本置于docs/first-round/candidate，`go vet ./...`递归扫描成不完整包，报undefined:fixture。源158件a6f4ec9a…前后不变，非产品断言失败。自动failed→development4已读原回执并自行归因修归档；Builder未代改项目。首轮清单11项按文件API逐SHA核对（目录读取400不代表文件丢失）。模板维护源补通用产物约定：证据副本不参与构建、保留原路径/字节摘要、handoff/files列具体文件；现有生成器隔离权限/交接回归exit0，未新增镜像措辞测试。模板项目副本和安装图升级尚待安全阶段，当前Run仍冻结revision2，不冒称已获得新模板指令。

本轮续记15:47 UTC：development4已自行将原证据副本置于独立归档模块边界，保留源码与首轮11项字节摘要；实际go list保留11个原根包、go vet ./...退出0，无产品/根门禁变化。正式handoff后tests5执行原make verify中，不能以定向绿或158件源码不变当全量通过；新增文档与归档边界也须QA审查。tests3内层state3969bytes/SHA16b152cc…与verify.log704694bytes/SHA9294aeca…逐字节摘要匹配，source/candidate unchanged；cleanup原文为attempted/candidate_removed，不扩大为全部历史进程消失。

维护源模板15bc98b已推送开发分支；标准export库存与旧2db仅sandbox_delivery.py/README两件变化，项目PR45精确head7b3891424b29258e6539c6010f988964ecc1cddf/三Git对象含SOURCE.json逐字节匹配，GitGuardian成功，按测试仓Harness构建授权合main0c1e22034a8599cc5ea21e31d9e110ae9ae7c46e。此为模板交付，不是候选PR43合并或产品部署。固定导出包1454080bytes/SHA cff419db6f54eef322302e12f6603178c150f74a9ad229843199a2ca8518accd已完整传到112，尚未更新安装图；标准check_upgrade_runs要求完成或正式停止当前Run后应用，待安全人工确认点执行并核旧Run冻结图不变。M1继续等待固定tests5及独立QA实证。

本轮续记15:54 UTC：tests5实际exit2，1075469bytes/SHA46e19fdbf713e234a00fadde4f3c88d0561a96b4e7b3849e8d49bae88f347f8d，正式output至EOF/truncated=false。主attempt20261014T154026Z-2753749201；已进入browser，migration15.408s/legacy-ui178.165s/routing214.066s均end oktrue，console刚开始后内层12分钟期限终止。不是Owner stop，不把原历史预期红当产品失败；后续完整browser/QA未验。自动failed→development6/CID4f30dad15a28ad1e5bf1607dabdf5797，正式输入request issue44:seq6:bounded-deadline-attribution/message3473提供已核证据，要求阶段关键路径及自身cgroup最小只读归因；禁止无证据增限、同版反复全测、改弱断言或容器内改冻结配置。待原生归因，不先认定CPU唯一原因。


### 2026-10-14 16:18 UTC：累计预算归因完成，正式输入有界接续

执行5唯一主attempt的state3968bytes/SHA `0f08a6944bf1ef200eba769d2b03c7838536b485e8ed772cefc2383352146cf2`与原verify.log1054470bytes/SHA `8361a3d64091f4557be14e80d436754d016bccfa1395b4f91bf3e041e1265735`由正式文件API完整读取并匹配。主gate_exit=-1，原12分钟context截止；自有gate/auxiliary groups absent、candidate removed，不能推及历史/组外进程。前置约322秒、三个成功browser阶段约408秒、console开始约5秒后被取消，未证单步卡死或产品断言错误。

development6先提出24分钟改动，但pending User Input 3473使handoff被正式拒绝；随后同会话处理纠正，全部验证器修改撤回、原12分钟与HEAD字节一致，并wait_for_input(blocked)。这是待输入优先及同会话接续的实际证据；旧提案定向绿不算当前候选。最终只读cgroup采样2CPU/8Gi符合标准，memory.events全0；累计节流101.809s没有执行5起点，不能作为该轮差值或唯一原因。完整纠正报告10258bytes/SHA `1e301257b5df1a52d27918fdbbfce08fa6c7abc2af7704907e56cb944ddabef9`保持。

维护者核对后明确替代“先归因、暂不增时”的临时限制：由原Pipeline Agent修项目累计门禁预算一次、上限24分钟，外层1800秒及全部用例/断言/单步期限/冷隔离不改，补真实短预算取消清理回归、实际预算回执及可用资源起止证据；不是改业务性能SLA或盲目增资源。若仍总超时必须blocked，不继续加时/重跑求绿。原resume用于stopped/failed，waiting调用收到409且无状态变化；随后按正确正式messages入口request `issue44:seq6:calibrated-total-budget-once`受理message4070/sameCID4f30dad15a28ad1e5bf1607dabdf5797。受理不等完成；等待新源码/完整固定测试和独立QA。Issue44正文已同步真实卡点、归因、决策与待交付项，M1未完成。


16:25 UTC补充只读复审：新资源采样草案无条件依赖/proc/self/cgroup并对不存在报错，会破坏原macOS固定make verify入口。通过同一development6正式User Input request `issue44:seq6:resource-observability-portability`/message4116交Pipeline修兼容性：不支持指标明确unsupported、不编造数据；Linux声明可用但读取/解析失败保留失败，独立QA复核。Builder没有代写或代跑项目。当前仍在研发修订，定向race/vet绿为Agent自报，不能当新全量绿。

工作区全量artifact listing返回“unavailable, changed during reading, or exceeds size limit”；精确文件API可读取上述完整日志。源reader有5000扫描条目上限并拒绝符号链接，当前尚未确定是条目限额还是并发变化，不把猜测当根因。M1须使用具体同版文件并核GitHub远端交付；列表呈现问题列为待诊断平台缺口，不放宽文件访问边界。


17:03 UTC续记：development6在待输入保护下再次被拒绝提前handoff，原生同会话自然接续4116；实际确认原deploy-local.yml/macOS ARM64完整make verify要求，修非Linux unsupported/无虚构指标和差值，Linux读取解析失败仍fatal。原生contextCompaction后同工作区核对清单继续，未新建会话。新定向race/vet及Darwin ARM64编译为Agent回执，不冒充Darwin实机或全量。最终5份源码+实施记录经维护者文件API读取并逐SHA与129项清单匹配，清单d8798a76f04c61ff4bb740dbd0981ee98c77d3a5effa22a0356cbb5e9f81be4e；161源码摘要e637c8f9a92859a8766e9c6e23e2c08f53d9b40d96b5c0a247c455e7e19bac9b。先前中途main_test短SHA与归档不同属后续修改，不倒写为同版。

正式handoff已被接受，tests7开始执行原make verify，内部24分钟/外层1800秒；完整结果未出。首轮缺章保留，仍需独立QA实际revise/研发补齐/再次固定测试/QA复核/正式确认/报告/真实草稿PR，M1未完成。Issue44已实际核对bot账号回写三轮提案/撤回/纠正与待输入拒绝，未只停留在平台本地消息。


17:08 UTC：当前兼容候选清单129项中，127个正式可读文件已全部经文件API重新读取并逐SHA匹配，共2690893bytes；清单SHA d8798a76…1be4e。`.github/workflows/deploy-local.yml`首个读取返回403，AGENTS.md同属明示不可见策略，后续不绕行，两项标为未由该API核验，不称129项全读。记录保存在ignored evidence receipt，最终Git提交对象仍需另核。tests7仍running，没有completed回执。

后续减法评估保留本轮成本：同一development6经历归因纠正、预算授权、兼容修复输入及原生contextCompaction，归档扩大到129项；必须将实际等待/返工/材料组织成本与隔离及正确交接收益分别评估，M1即使完成也不能据此声称工程效率提升已经证明。先完成实链，不在运行中新增通用编排层。


### 2026-10-14 17:18 UTC：tests7非超时失败，自动返development8

Issue44/Run744c4329…正式tests7退出2，外层1170997bytes/SHA `0c2dfc2cf2c31c948459bc8e729a4b2ffbc1e50ce92735570c6b13c20c85a9fd`完整分页EOF/truncated=false。唯一主attempt `20261014T170210Z-2990963391`原项目log1146109bytes/SHA `efd55d375788bd9620b35af4e370b5cd498a74325df22427839d648a383f40a9`与state经正式API完整读取匹配；不可混包级回归的嵌套短预算fixture。

实际主gate配置1440000ms、有效1439999ms，执行847083ms/exit2/cancelled=false；总量17:02:10Z–17:16:33Z，预算修改已实证生效，不是新超时或Owner stop。161件e637c8f9…源及副本前后相同；Linux available前后同cgroup/2CPU/8Gi，OOM各项差值0，usage1100252962us/throttled56440223us是共享cgroup采样窗口差值、非单进程/唯一性能归因。gate及各aux自有组absent、candidate removed；不扩称历史/组外进程。

migration15.360s/legacy-ui195.875s/routing224.273s/console31.165s均ok；full-console65.144s失败。1280 J01–07、visual-current-period、management-summary、console-visual十页已过，first-self-hosted在self-hosted-sse/proxy-call处failed/error_kind other/HTTP200，具体原错误尚不明，不能把HTTP200当SSE或用量费用已通过，也不把diagnostic secret_present布尔直接归因为泄漏。完整三视口/最终成功dependency probe/独立QA尚未完成。

正式failed→development8/CID97f586f6e857f16adeb9b00e8ba8bb86已原生承接读失败，不重复Run/恢复或重新增时。Issue44正文已同步非超时失败及最小归因下一步。预算及兼容修复不因此冒称整套验收；M1仍未完成，等待Pipeline实际定位/必要最小修复/固定门禁/QA缺章返工复核/确认/报告PR。


### 2026-10-14 17:31 UTC：SSE采集原层红绿，固定tests9已开始

原生development8在同一共享工作区完成最小归因：既有Go/UI/用户动作下，SSE response.text因浏览器协议protocol_error/no_data_found退出1；非产品HTTP失败。维护者通过正式文件API核验诊断原stdout4262bytes/SHA883595cc…、修后真实1280旅程stdout245bytes/SHAa9425518…/exit0、缺失DONE反例stdout4173bytes/SHAc132cd9c…/exit1且error_kind=assertion；五项采集单测stdout1042bytes/SHAd800ff14…/exit0。暖定向非完整冷验，原失败不覆盖。

项目最小实现仅journeys.mjs和新增stream-capture.mjs/test：测试页克隆同一次原生fetch Response收集SSE体，原Response交UI、finally恢复，无额外请求/伪造帧/重试；原状态、Request ID、dispatch、UI、data/DONE及账本断言保留。Builder只读审查及证据核对，没有代改/代跑项目。最终163源码摘要991c79ca919b3d0ccf148c066c3b24d993090fcb78ab864c0e4637cfff8040ba；交付清单164项SHA dcab3a6e7f3d6f2c7a991557a1cea0df1df7cbdf043d8ba31fb9393a96591448。正式handoff accepted，tests9于17:30:36.882Z执行原make verify；24分钟内部/1800秒外层不改，当前running无绿回执，仍待独立QA实际缺章返工和全部交付。

新交付清单164项中163项已逐文件正式API读取并逐SHA匹配，共5017211bytes；AGENTS.md按API可见性规则标未读取。它与“163件产品/设施源码摘要”恰好同数但不是同一集合，不混为一次全源码独立核验。最终Git对象与QA报告仍待。


### 2026-10-14 17:49 UTC：tests9完整冷绿，独立QA10实际承接

正式tests9 exit0，外层1179303bytes/SHA `fb2a741014592ac9474eb3e8d380b7f995660f3c10d025b4009ed0f95e9cacb1`完整分页EOF/truncated=false。唯一主attempt `20261014T173037Z-3756242626`，原项目verify.log1150513bytes/SHA `6b689697cbd4243c31d122feec3a7ee48fbfbf8e0107060a9538a18b5cc44254`与state独立读回匹配；17:30:37–17:47:01 UTC，主gate971282ms/exit0/cancelled=false，配置1440000ms。

实际冷起点node_modules/npm cache不存在、NODE_PATH unset、无人工预装；163件991c79ca…源/副本前后不变。283Node、原全包race/count1/parallel4（relay103.748s等真实非cached输出）、历史分类器、迁移/路由/console/full-console均过。1280/1440/390各J01–07、management-summary、十页console-visual和首次self-hosted旅程均passed；最终candidate/node_modules Playwright/core1.59.1和Chromium147.0.7727.15探针通过。gate与全部aux自有groups absent/candidate removed。Linux有效前后资源delta为共享cgroup窗口，OOM各项0，不能当单进程占用或Mac实机验证。

自动进入独立QA10/CIDb3243fbf4ecf62c2cb2f26f8bdbb56dc，原生已声明核对同版证据、测试改动及实际缺章；未由维护者代QA放行。Issue44已同步完整冷绿和QA仍待结论；首轮故意缺章仍须实际revise、补齐后再固定测试/QA复核，后续正式确认/报告/草稿PR未完成，M1不提前置为通过。真实供应商/支付/外部身份、Darwin实机仍NotRun。


### 2026-10-14 18:03 UTC：独立QA实际拒绝，development11正式返工

QA10正式route=revise已接受，Run744c4329…自动进入development11/CID2bd1349ec948f46d152843663017d2b5。QA10-D1确证首轮文档缺“停止、恢复与环境释放”，当前文档仍与首轮7792bfa…原快照一致；拒绝原文10074bytes/SHA bca2159b81a59ead2f8056cab5d64dedd33853106d9faabe95bfabcbd9a831c6，归档及harness-output副本同字节。QA独立定向race/HTTP/Node与材料复核通过不覆盖缺章阻断，也不冒充独立全量浏览器实操。tests9完整绿保留，尚无最终QA/确认/报告/新PR。

development11已自行补章及真实时间线，保留原缺章/QA拒绝，不改产品或测试源码。发现交接草案要求固定tests额外sha256sum，但冻结执行节点只运行make verify，不解释文字建议。维护者正式User Input5363/request issue44:seq11:fixed-tests-handoff-evidence-boundary纠正：QA10允许可审计等效依据，采用研发实际handoff前完整清单基线、平台顺序执行记录及后续独立QA逐项重核，维护者正式只读API旁证；不得称runner自身起止采样，不修改冻结命令或把本Run材料要求硬编码到项目验证器。最终是否充分仍由独立QA判断；若不足明确平台能力缺口，不能补造或靠未接入脚本。正在处理输入，不把受理当修复完成。


18:10 UTC续记：development11正式接续5363后采用QA允许的等效审计边界；旧未接入命令建议撤回，纠正前记录另存。最终handoff18:07:55Z accepted，tests12原make verify已running。新版214项清单位于development-11-correction/SHA256SUMS，SHA1ce553146e21a910a47454d9c4cf6efa83a8527e19711c41ca7bc46e3801fdb3；研发18:06:56.095–.123Z实际逐项核验exit0，新文档3d881a85…，163源码991c79…不变。此为研发交接前基线，未来QA须独立判断，不能改称固定runner采样。维护者首次读错旧development-11清单导致当前文档摘要不符，未归因为运行时篡改；读取正式handoff后纠正为214项清单开始正式API旁证，原错误保留。tests12完整结果/QA复核仍待。


18:14 UTC旁证：维护者18:08:57.909–18:13:14.034Z通过正式文件API重读最终214项清单，213可读文件6493619bytes全部逐SHA一致，唯一AGENTS.md按明确可见策略未读取；清单前后SHA1ce55314…fdb3相同、两端均seq12运行中。研发真实handoff前baseline.json69552bytes/SHA5a5de90b8f3f13249c5a4822bc94c5ac00327db1a1db9785f4f2dd2baafe6e39另读回吻合，UTC18:06:56.095–.123/214项/exit0。以上为研发基线及维护者运行期旁证，不称固定runner起止或全214独立读取；最终QA仍须判断等效依据。顺序逐文件API耗时约256秒，是材料交接效率待减法优化的实际成本，当前未为绕过限制新增通道或扩大权限。


### 2026-10-14 18:27 UTC：返工后tests12完整冷绿，独立QA13运行

正式tests12原make verify退出0，外层1184959bytes/SHA704b03399b155dd851d66388b6c9ac10b3aa95545a9080aaa6b546ad3ca9f8f5已分页EOF/truncated=false。唯一主attempt20261014T180756Z-2810915431，原log1155178bytes/SHA9856236d5b7cdd9e31970e46dc3a5020d9e166c3659f25626c366abfdb23dc82与state独立读回吻合；18:07:56.252–18:24:51.320Z，gate998870ms/exit0/cancelledfalse/配置1440000ms。163源码991c79…及副本前后不变，冷node_modules/npm cache不存在、NODE_PATHunset、无人工预装；gate/全部aux exit0和自有group absent/candidate removed。辅助字段为exit，不是exit_code；首次本地汇总用错键产生false已按原对象纠正，未误报成测试失败。

283Node、全包race（relay101.538s等非缓存）、五browser阶段全部ok；1280/1440/390分别J01–07/summary/visual十页/首次self-hosted12步均passed，受控HTTP不是供应商验收；candidate依赖Playwright/core1.59.1、Chrome147真实探针通过。Linux采样有效、OOM差值0，CPU统计仍是共享cgroup窗口，非单进程归因。自动进入QA13/CID4c0c0e62b86e56a54f77916c22924b3c。正式输入5901/request issue44:qa13:observed-fixed-tests-and-delivery-baseline仅提供维护者已发生的213可读文件核验与完整log旁证，明确研发基线/固定runner/QA职责区分；完成后第二次文件核验尚执行，不预报通过。QA10-D1最终关闭、正式确认/报告/PR仍待，M1未完成。


完成后旁证：维护者2026-10-14T18:25:42.405617+00:00–2026-10-14T18:30:01.886875+00:00再次正式API读取相同清单，213项6493619bytes逐文件与测试运行期全相同，清单1ce55314…fdb3不变，AGENTS.md仍按策略未读；两端均seq13。QA13原生进度已称全214及原回执匹配/定向复查通过，但正式结论及handoff尚未完成，不能用进度自报替最终QA。


### 2026-10-14 18:40 UTC：QA13正式放行，安全停顿/升级/恢复及操作者确认完成

QA13处理5901后正式approved→approval14，最终原文13628bytes/SHA82122446a992d746d16555369d1191d061c221f52afd91bfaf82b942142fa1b8；235项审批清单SHA75f5d969f357e6551e11e96e93b8621c737d9fdc2fb07822d30380e22ad1aed6。QA独立完整214项、研发baseline、固定tests12主log/state及定向实际回归核验，接受分角色同版等效依据，关闭QA10-D1；明确不是固定runner自身完整清单起止采样，不把维护者尚未送达的完成后旁证补写为已验。维护者正式读回15项QA交付原件及最终QA；tests12原log与QA归档1155178bytes/9856236d…同字节。

在approval14安全点正式stop→stopped，重复stop返回整个Run逐值相同，前13步/冻结定义/参数/输入/环境不变，全部native_processes status=stopped。通过既有标准Installation/check_upgrade_runs应用15bc98b模板，当前Workflow83f…revision3；运行平台仍f409c0a，旧Run冻结revision2逐值不变，runtime/privateconfig/凭据字节不变，healthok。首次回执读取因终端软换行JSON解析失败，仅用capture -J重读原回执，不重复升级。正式resume14恢复同14步waiting确认节点；QA原文、审批清单、新文档3d881a85…恢复后重新读回一致，不冒称所有235项都在恢复后由维护者重读。

按用户已有授权由Builder作为操作者正式decision14 next；首次本地Python多行字符串语法错误在HTTP前终止，修正后仅提交一次，异步原回执waiting随后实际approval14completed/report15running，不重复decision。报告输入带真实停止恢复/重复停止/标准升级事实、完成后213文件旁证、发布器具体文件≤100/5MiB契约及未验边界；这是授权操作者确认，不是用户本人点击。完整最终report、Pipeline草稿PR及远端同版核验仍待，未合并/部署、M1尚未完成。


### 2026-10-14 18:44 UTC：报告阶段发现通用确认上下文遗漏，正式停止修平台

用户影响：报告Agent无法得知已完成的正式操作者确认，不能正确形成发布报告；seq15实际read_command_output(seq14)返回record not found，因为approval没有命令日志。报告主动wait而非伪称确认通过。只读源码定位workflowAgentInput只附最近Agent结果和执行回执，遗漏approval的NodeResult；不是用户未确认。现有decision14结果仍完整保存在正式Run API，流转已发生但报告初始输入缺失。Builder先正式stop15保留报告现场，不再向正在运行副本补文件或把自然语言救援冒充永久修复。

处理计划：唯一平台源码按当前节点传递此前已完成的最新操作者确认（保留原文/路由，排除取消、将来及已被同节点后续确认替代的旧结果），明确不是命令输出且不扩权限；增加有行为差异的回归先红再绿，标准编译安装后从正式return入口重新进入report，核真实原生输入获得原decision14。冻结定义、前序QA/测试/审批记录不改，M1再次明确受该平台缺口阻断，尚未PR或部署。后续GitHub当前main材料缺口单独检查，不把无法读取远端误判为QA失败。


18:49 UTC修复验证：操作者确认上下文回归已真实红→绿，相关race通过；标准scripts/verify.sh完整exit0，独立源码复审无Critical/Important。当前Run正式stopped15，未发生publish。234份可读审批材料共8244399bytes已按SHA保留（复用此前已验原件加7份新读，非全量重新采样）；AGENTS.md保持API边界未读。待标准运行平台升级和新report真实输入复验，不以源码通过宣布M1完成。


### 2026-10-14 18:55 UTC：确认上下文通用修复标准上线，原Run实际复验

平台afebd27经scripts/verify.sh exit0/124865bytes/SHA3ee9ccc9589c897b6d7df7a03b7667fe40c081675ea8bea0aff0bc1f2bf3b975、独立review无Critical/Important；git archive源码2c996660…经既有112会话传输并校验，标准build16/install16均exit0，health实际revision afebd27b016d6dc80a5c4c8004ddf13f5612535e，binary824ad091…。安装前后Run整体逐值相同、私有配置/凭据保持；没有手改运行副本。正式return15→report16，旧报告取消记录保持；新原生输入6522实际包含自动生成的approval14确认块与原summary逐字匹配，前14步不变。GitHub正式compare同步确认main0c1e2203相对候选仅3个.harness标准模板文件变化，以正式return附只读patch给无Git凭据的报告Agent，区分维护者材料和Agent自行取证。没有给Agent新增Git权限、重跑产品或重复审批。实际草稿PR仍待。


### 2026-10-14 19:03 UTC：正式发布被模板测试字符串误报阻断

report16已生成90项报告清单并保持235审批清单；publish17真实exit1/1204bytes/SHA2c4cdb03d2128960819590b982e976b41314ab3573c2099b482310371a6dc5cd，固定check_publish_files在Git提交前拒绝.harness/.../report_publication_test.py。唯一原因是测试源码直接含PRIVATE KEY头部样例；不是泄露了真实私钥。Agent对选定文件扫描通过不能冒充全仓固定发布成功。Builder正式stop17，未重跑发布。通用源最小修复只拆开测试字符串，运行时原样，凭据扫描规则/边界不动；标准export集成回归真实红→绿，实际.env仍被拒绝，独立review无Critical/Important。首次直接跑全模板未设置SDK路径导致2个导入错误，改跑原scripts/verify.sh，不将设施导入错误算产品失败。待标准升级及新入口实际复验；旧Run冻结项目模板不手补，环境TTL前保留报告/证据，M1未完成。


19:15 UTC正式交付记录：bc43314模板修复标准full41 exit0/124875bytes/SHAaefc8488a7c4b69c4dd6d3f7d1b22b09d6852b18441fb3b52660a0a83f523368，独立review无Critical/Important；PR46 head eb77336c9fd673d8a75ab69c0d0fec9908b7ef6f三文件远端逐字节核对/GitGuardian success后精确merge main6d2552d331fd7a0ec8d205f5d18ba3bcd34d6995。平台运行二进制仍afebd27，无需因项目模板测试文件变化重装平台。报告90项+QA共91文件6416584bytes已正式API原字节保存，报告JSON SHA7791903253c3118f8b443ed02c8f789ed2d800f877e52cb57899b7e5f2392b8d；与已存234项历史区别记录。Run44 stopped/nativequiet并核GitHub无分支/PR后正式cancel；release响应超时，未盲重发，随后正式GET确认cancelled17/released/nativequiet。旧冻结图/历史保留，M1没有完成。


### 2026-10-14 19:18 UTC：标准模板修复接入新Issue48，原生整链待重新验证

PR47是Builder经正式文件API原字节转存的接续材料，不是Pipeline最终PR；94对象已Git tree逐SHA核对，163源码库存991c79ca919b3d0ccf148c066c3b24d993090fcb78ab864c0e4637cfff8040ba全Git blob独立重算，缺章快照/QA10拒绝与原文件匹配。PR43按被47继承关闭未合并；PR47保持草稿。标准save仅将本地Runner及112可信entry base由issue42候选改为codex/issue44-reviewed-candidate，其他配置/凭据保持。新Issue48/id5854043575明确替代44、新文档首轮缺章/本轮重验、角色职责和最多100具体发布文件；旧材料是已提交基线，不要求重复塞进新files但正文覆盖相对main实际变化。44关闭not_planned不标通过。新opened/labeled Actions排队同稳定request，尚无API Run，不代起任务；唯一Runner online/busy，真实占用为main6d2552d3自动prepare38621212545，未人为取消或触碰旧服务。


19:22 UTC新入口：原main自动prepare38621212545 completed/success后释放唯一Runner，未人为中断或另开Runner；两个Issue48事件实际success，经by-request github:big91987/model-relay:issue:5854043575只关联Run2b6a366cc7b3d48870e1b887757f229f。环境f7ab…/ready19:20:32Z/原镜像8b6ea…/14400s/8192MiB，新图revision3、max40，三Agent仍codex/gpt-6.1-sol。prepare1尚执行，受理不算准备或整链完成。文档db849ce首次直连push超时128，远端只读仍bc43314，进程已结束后才通过既有代理有界push并读回db849ce成功；未盲重复未知写。


Issue48 prepare1实际exit0/2236bytes/SHA7eb647dd67ca15dfdfd6fd4800d4e90e80dd036deb86142b86bd4446e0ec8336，正式output EOF/truncatedfalse，进入development2。维护者尝试正式file API读取.harness/SOURCE.json被403拒绝，未绕过或改权限；因此尚不能声称通过该API读回容器隐藏目录。候选Git对象和准备固定commit已核，Agent正常工作区中的原生核验及固定测试仍待。


Issue48原生development2命令事件18765真实exit0：existing_publish_scan PASS（offline、未发布）；项目模板SOURCE bc43314、146项manifest mismatches空；declared_main_ancestor PASS，源码163项991c79…；冷起点node_modules/.cache/npm/.cache/go-build/bin/test-results均不存在。此前维护者隐藏目录file API403仍保留，不把Agent原生执行输出说成维护者独立读隐藏文件；标准模板在新Run的原故障预检已通过，固定发布全链尚未验证。


19:30 UTC：Issue48 development2正式route tests→tests3原make verify执行中。首轮新增12个具体文件，既有跟踪源码/测试/模板及旧文档不变；文档35e5eb5af37a83c81344cbad95cb89b866193179605ba5b9c772dc2dc1562be9故意缺停止恢复章节，首轮快照保留，交接清单自身536ec78e5374f23d752a4f9be66e3d6c52a56a04752e60a9606b2a24ea8f09f4。原生归档前后offline全仓扫描及byte_integrity退出0，未预装/未代跑make verify，独立QA尚未开始。GitHub入站回写github-actions[bot]，准备/研发节点回写big91987-agent-platform-bot[bot]真实评论及链接已核。


### 2026-10-14 19:47 UTC：Issue48首轮完整固定门禁通过，独立QA4开始

本轮tests3真实make verify exit0，正式output 1174629bytes/SHA754f5242f9a567dd52b2f40b10667b34c18819099b6094d28940e48f661d310e分页EOF/truncated=false。唯一主attempt20261014T192852Z-1089328441，原层log1144558bytes/SHAafdbb354a9181d8fd950d2d16159283c9379d56552269eadbe4cd3f283ed101a与正式API读回state匹配；19:28:52.320–19:45:49.177Z，gate1000606ms/exit0/cancelledfalse。163源码991c79ca…前后及副本不变，冷node_modules/npm cache不存在、NODE_PATHunset、无预装；gate与10个辅助命令exit0/自有group absent，candidate removed，候选Playwright/core1.59.1及Chrome147实探针通过。审计摘要最初读取不存在的auxiliary键为null，已依据原state的auxiliary_commands纠正，无改原始回执。自动进入QA4/ddf4232ef699679b0d450019ff9018d1；缺章返工、第二轮固定测试、新独立QA、正式确认、报告和Pipeline草稿PR仍待，不能把首轮绿等同M1。


19:54 UTC：QA4正式revise已交development5，缺章QA04-D01真实复现且首轮35e5eb5a…快照不变；另保留QA04-E01：独立QA只读取seq3外层首32768bytes/eof=false，不能声称已完整审查外层。内层完整原log/state已独立归档、与维护者正式API原件逐字节相同。维护者另保存15份不可变QA材料1180548bytes，拒绝报告9606bytes/SHA1d25ef3827de7846b4af52493832a1dba74ba2e3fb295ef118903d1c45f0f1f4；不将维护者已读外层EOF替成QA已读。研发5当前自报补章并归档，正式handoff/修订后完整固定复测/最终独立QA待完成。没有重复Run或手改状态。


19:58 UTC：development5正式交接tests6，修订文档12999bytes/SHAd1a7112600467ae85d9af9000e3349f78ee9590e0317fc935cd4e1560d5f4209；维护者正式API保存17产物1221185bytes，独立确认新文档以首轮六章原字节为前缀、QA04拒绝原件不变。QA04-E01继续Open：研发仅补读外层尾页，不能据首尾EOF倒推中间已读，后续QA必须实际完整补审。共享工作区有上轮test-results及Go缓存，研发如实记录；本轮固定入口自身须再次证明隔离候选冷起点，不要求或手动清空共享工作区。tests6原make verify运行中，源码、模板及门禁未改，M1尚未完成。


### 2026-10-14 20:14 UTC：Issue48返工后tests6完整绿，独立QA7复核

正式原make verify exit0，外层1177433bytes/SHAec2540257de3d6a6c0717f165a045ceee8712b30380bb1cba4c56691e4bf6caf分页EOF/truncatedfalse。唯一主attempt20261014T195523Z-3543396171，内层1147466bytes/SHA61dadc32700ffe860c9a03800b30e13b6c406f2a56dd43bedc1d71c14c3ba797与state原字节核对；19:55:23.562–20:12:24.157Z，gate1004725ms/exit0，10辅助均exit0/自有group absent，candidate removed。163源码991c79ca…与候选副本前后一致，隔离候选node_modules/npm cache为空、NODE_PATHunset/无预装；候选依赖Playwright/core1.59.1及Chrome147实际探针。283Node、五browser阶段全部ok，三视口各7连续旅程及12步首次self-hosted全部passed；受控HTTP不冒称真实供应商。自动进入独立QA7，QA04-D01与E01最终闭合待正式结论，尚未确认/发布。


### 2026-10-14 20:26 UTC：QA7发现完整外层日志无法可靠交付给Agent的通用缺口

QA7关闭缺章实质问题并独立核内层日志、9项限定回归，但正式wait blocked，未交approval。维护者完整seq3/6分页EOF及摘要通过正式User Input7697补充后，QA仍如实区分旁证和独立读取；追加seq3 32768→65536后确认工具呈现省略且无原字节文件引用/导出，不能拼接省略文本冒充原件。此前revise因pending user input被拒绝并正常接续，未产生重复研发节点。具体用户影响：固定验证已成功但QA无法完整独立取证归档，交付停在证据链。原因是平台read_command_output仅分页文本面向模型，不提供共享工作区中的完整保留原档；不是产品回归失败，不重跑同版tests。

修复设计：既有工具新增显式save_to_workspace选项，不增加节点/权限变量或新日志对象。仅当前有效Agent读取本Run已结束步骤；平台固定导出路径，原始已脱敏日志<=8MiB，返回seq/path/bytes/SHA/truncated。本机和OpenSandbox均安全写入、目录/文件链接拒绝、已存在内容相同可复用、冲突不覆盖，失败不得声称完整。先真实回归红绿及独立review，再标准二进制升级；旧冻结图/测试/QA历史保持，通过同QA正式恢复取证，不由协调者复制材料或修改产品。


### 2026-10-14：QA7完整日志取证阻塞及平台导出修复

当前Issue48/Run2b6a366cc7b3d48870e1b887757f229f通过Owner正式stop停在seq7，所有原生进程已确认stopped，原固定tests3/6、QA4拒绝和development5补章历史保持。QA7没有判通过：capability-blocker.md为3417bytes/SHA8bc5084acfdeb87271c4fe05e9200bcfc21465b30fe1d35347b91c151d95fc39，指出分页工具的模型呈现省略不能变成完整原件。维护者未复制日志进工作区、未代QA关闭证据缺口。

通用源码在既有read_command_output增加save_to_workspace；固定本Run/原步骤路径、原始脱敏字节及SHA/bytes/truncated，前缀一致才恢复分块，冲突和链接拒绝覆盖。独立review发现三项Important：沙箱helper不受Stop/未知执行门禁约束、Python从项目目录加载模块可能伪造成功、只读会话仍可写。均进入维护源修复：复用connectorJobs取消和已有Run恢复/释放入口，持久记录最后helper已派发身份/quiet，未知结果不续派不放行；python3 -I -S；读取当前有效工作区权限。追加发现迟到reconciliation可覆盖新空ID intent，改严格匹配已查询命令ID的CAS。隔离HTTP回归覆盖Stop、丢launch/status、重启保留、只读、Python污染及迟到核对；不冒充产品验收。平台完整verify42 exit0/124870bytes/SHA5f85406cfe90db1aa58a39e47c109efd9f317f7ba6736facd80d20bd8302618f是前轮源码证据，不能替最后生命周期修复的完整检查。待最终完整检查/复审及标准部署后，由同QA7正式恢复调用导出并独立核档，不重跑已绿产品测试，不修改冻结图或凭据。

最终维护源标准verify44 exit0/124864bytes/SHAfc1bee1b6a70fd8986071dbfecccc339a005448b3b50e9a5e1b5d2f76de2949b，八件代码/测试源码前后一致；定向race实际3.493s绿，verify43全平台race实际73.513s绿，44复用同Go输入缓存明确保留。最后独立review确认严格CAS及原三项Important关闭，无新增Critical/Important。标准升级及原QA实际导出仍待，不将源码验证替真实Run证据。

20:46 UTC标准交付：维护源8ad93945da127d5da7cdeefde68b92dccf9a2d15已仅开发分支提交推送。git archive4853760bytes/SHA8ebe4c79a899693cb1487db34cd681824c926ef9fd2d33c3737c3fb876f56fef经既有SSH传输重核；112原build.sh/install.py均exit0，health同revision，binarySHAbd334e43ae974214c17475d4714014d54b6413f47ee5ac1763222f3853472906，升级前后完整stopped7 Run逐值一致、私有配置保留。随后正式resume同QA7受理running，要求Agent自己导出seq3/6完整原档并按原门槛独立复核；受理不代表已导出/QA通过。

原入口实证：同QA7恢复后原生MCP事件22564/22568分别实际完成read_command_output(save_to_workspace=true,seq3/6)，耗时20.685/20.627秒；共享workflow-output固定原件1174629/1177433bytes，SHA754f5242…/ec254025…与保留原档一致、truncated=false。不是协调者复制文件或手工触发helper。当前Agent独立复核进行中，导出成功不等于最终QA通过；原blocked/拒绝/两次固定测试历史保持。

验证边界：本次实际导出验证覆盖seq3/6各约1.17MB原日志，两份工作区文件经正式文件API逐字节等于原输出。8MiB归档上限、单次导出两分钟期限和中断恢复为实现边界；尚未在112对满8MiB/强杀平台做端到端试验，留在M2，不泛称全部失败恢复已实测。

同QA7已正式approved→approval8 waiting：最终独立QA关闭QA04-D01/E01、QA07-E01，实际整份原档复核/内外层映射/历史红灯归因及9项限定回归证据保留。最终QA承认最初量化时间校验严格等值导致的1ms检查脚本失败并保留原件，修订量化界限不改原测试/预算。维护者正式API归档30件4794292bytes，首轮缺章和QA04拒绝原字节与旧档一致；推荐81件当前未提交交付文件，每件≤5MiB，100件上限仍有余量，不是总包5MiB。当前尚未decision/新PR。

正式approval8控制验证：stop/重复stop去重→stopped→同seq8resume waiting，前7步、冻结definition/input/environment逐值不变，全部native stopped；恢复后四份QA/文档/摘要/首拒绝原件重读保持。首次decision误用approved路由被HTTP400拒绝，GET确认仍waiting8/result空；核冻结出边next后正式decision受理，拒绝证据保留，未改图/绕门禁/重放未知副作用。确认主体是既有授权的Harness Builder通过Owner API，不是用户本人点击；仅允许report/固定草稿PR，不授权产品合并部署。下游应直接收到保存的确认结果，尚待实际报告和PR。

report9首次原生turn在任何command/MCP调用之前serverOverloaded/Selected model is at capacity失败，全部原生进程stopped，未产生报告或发布副作用；固定模型不换。实际report初始User Input已包含approval8平台保存的操作者确认，证明确认上下文修复在本Run接通，不靠另发确认文本。后续仅同report9正式有界恢复一次，原失败保持，不重跑tests/QA/确认。

20:59:56 UTC report9同会话有界恢复后再次serverOverloaded，原生命令均结束、无publish节点/PR。已实际执行10条命令，其中最后一条写入三份报告草稿：report.json30474bytes/SHA3f2a43e516dce4b6a1f551e77efab0944460ede414a39d5f91ad25e118146e86（JSON可解析/85files）、report.md22803/440476a2f6cd45c9cb852157a2deef334be9e78e4962df151d2c08abee34a45a、确认上下文3164/e37a54f228f12a5926690c4fb0698daefb8bd4fd519fa648188e93e9bfb173c5。最终validation及handoff未发生，不能概括为无写入或已报告完成。三份已通过正式API保存。暂停连续恢复；至少五分钟后仅允许最后一次同会话有界恢复，先重核现有副作用，失败则保留原现场与明确容量原因，不换模型/无限重试。

Issue回写核验：平台机器人已于首次容量失败即时发布“执行失败，流水线已暂停”及处理链接；同节点再次失败通知去重。该自动通知未细分serverOverloaded及部分写入，维护者已追加明确署名Harness Builder的单条去重说明（marker issue48:report9-capacity-20261014），列具体影响、已尝试处理/部分产物和五分钟后最后一次恢复计划，不冒充用户或QA。安全错误分类摘要与恢复结果更新的自动回写细化留M2，不能把此次人工补充说成平台全自动说明原因。

最后一次有界报告恢复：第二次容量失败后实际间隔304.008557秒，正式API重读三份部分报告与保留档完全一致，全部native stopped；同report9正式resume受理running，前8步和冻结definition逐值不变。明确只有完成原校验及handoff，无测试/确认重放；若再次容量失败则停止继续重试、保留场景。受理不代表报告通过。


### 2026-10-14 21:12Z — Issue48 发布清单失败，保留现场并正式返报告

报告9完成（JSON 30877bytes/7bda5d05…），publish10在git add前后边界因清单含Git忽略的harness-output/qa.md和report.json返回exit1。原完整输出8433bytes/SHA49f9b07128016d7a3c8baf54c3c493000440c7444886e764ae1337efdb171618保留；85项原报告及文件已归档4993968bytes。Git add可能部分修改index，不能称无副作用。GitHub原Run分支404且关联PR空，尚无远端发布。现有模板本来将QA/正文复制到docs/harness/run-*，运行输入不应重复列为仓库提交文件；报告角色指令未明确区分交接artifacts与发布files，固定发布器也缺少修改index前的忽略项预检。通过Owner stop→return report核对HEAD/index及已生成报告，修正清单、保持QA/产品/正文原字节与原测试确认，不强制添加忽略文件或重跑旧门禁；通用角色约定及预检在唯一模板源补充后标准交付。当前M1未完成。


发布返工补充：stop10→return首请求错误使用message字段被409拒绝，GET仍stopped10；核SDK/API应使用summary后正式return到report11，原前9步及definition保持，不重复stop/放宽门禁。报告11原生实际核HEAD30b4e13未新增commit、85项index字节与已审材料一致，只从files删两项忽略输入，保留原title/body/QA/源码。83项新清单离线全仓/历史/暂存扫描及Git忽略检查通过，正式complete→publish12。源码模板增加所选文件和固定输出的忽略预检，发生在任何交付文件写入/index变更前；2项真实Git回归先红（原git add报错）再绿，7项发布器回归通过，独立review无Critical/Important。该源码预检尚未标准应用当前冻结Run，当前接续使用原发布器与纠正后的有效输入，不能混称已在当前Run验证新预检。对外含内部地址和执行细节的Issue评论被自动审批拒绝，改仅公开失败/返工/未生成PR状态已通过，详细证据保留私有平台；没有绕行发送被拒绝内容。


publish12仍exit1/908bytes/SHA909304d1bbe11ce1a6fb6566a3713aafa6090d995169887baa1c751dab4e1775，原已生成且暂存的docs/harness/run-*/qa.md及report.md在发布器自动加入files之前，被tracked-change检查误作未审文件拒绝。report11已真实证明85个index文件=83清单+2固定原文、HEAD不变；仅删忽略项的恢复说明不充分，未声称修复成功。远端再次核无分支/PR。下一步通过正式stop/return报告把这两份已确认固定原文明确列入files，先真实核完整preflight再交接；源修复需让固定输出在范围检查前加入并逐字节验证，保留不同内容冲突及未知文件拒绝。


通用发布恢复修复：增加两个真实Git回归，已暂存的固定输出→正常提交→同内容再次接续保持同HEAD，以及不同QA字节拒绝且index不变；原实现两项均红，修复后发布器9tests全绿。固定输出的路径与现有字节先核验，再纳入history/tracked/staged统一范围；历史同路径也要求同版字节，仍保留所有凭据和未知文件检查。独立复审无Critical/Important，建议补充历史不同QA后还原的专项负例列后续覆盖。verify45完整exit0/125212bytes/SHA0d3fcf4d8520b75e343905831ad448f29763b28d1e2e5d298bb53d67c2fcc008为恢复修复前版本；最终完整verify46进行中，不以前轮绿替新版本。


最终模板验证：verify46完整exit0/125790bytes/SHA5c0932f789c99f2ccd34f9a350f05d4a8e8b55a0c315f90b25f1f4d38c9fc902；原Go输入未变，race缓存明确标记，旧实际race43保留。review建议历史不同QA后还原的负例已补且通过；完整suite加载后追加该用例，最终单独10项真实Git发布回归2.723s全绿及ruff检查通过，无实现改动。Issue48 report13明确将两份固定原文纳入85项清单、同版核对后，publish14正式exit0/9719bytes/SHA61c1d4b14a8b5b4aef8ccbe0ad3b1dc3d08b49f6c0b9fc9c80a01627718bf6b4，Pipeline已push a033eb20ea848d3aead77231da367703b1b43fb8并创建草稿PR49；done15 completed。当前正在远端逐字节复核及归档，尚未释放环境，不提前宣布M1验收完成。前publish10/12两次失败回执保留。


### 2026-10-14 — M1交付与保存后释放

隔离改造当前状态（2026-10-14 21:30 UTC）：M1真实整链已完成。Issue48→唯一Run2b6a366cc7b3d48870e1b887757f229f（15步completed）→Pipeline自行发布草稿PR49，head a033eb20ea848d3aead77231da367703b1b43fb8。两轮完整冷make verify、独立QA实际缺章拒绝/返工/全量复核、共享工作区、原生gpt-6.1-sol角色交接和正式操作者确认均有原回执。GitHub 163源码991c79ca…、85交付文件、QA/正文/首拒绝及146冻结模板逐字节核验通过；86件最终本地归档4997769bytes。环境正式release请求响应超时后只读核对已released，未重复释放；入口已标准恢复main。草稿PR49未合并/部署，不代表全部产品、真实供应商或全部可靠性场景已通过。通用发布修复源a20c59b已开发分支提交推送，测试仓标准升级PR50精确合main cb2cb1e89b3f765f56a83bbbe5ed3c1b02041f99，原manifest将未来图升revision4，仅报告约定改变、当前冻结Run/私有配置不变；112二进制仍8ad9394。下一阶段M2/M3/M4待用户确认，不再自动启动新任务。

PR49实际发布14 exit0/9719bytes（61c1d4b1…），最终report13 JSON SHA9a1f74bde063d195f587a4eefd61a00c08e87089560dd712a2801c679b60808c。报告9→11→13正文/title保持，仅files移除2运行输入、增加2同版固定输出；前10/12失败和报告节点容量故障保留。首次远端对象只读核验连接断开，单次有界续读仅用校验后的Git对象缓存，最终全部通过，不重放写入。平台release POST响应超时后正式GET confirmed released/原steps及definition保持，不能把响应超时当未执行。

标准修复交付：固定导出146件、包1464320bytes/SHAe9a7f5cf6fa1d54bf824fdd2944910437cb9378b2c009da1f47fa83517ab94c9；导出后10项实际Git回归通过。PR50精确head66b43abfb7cffe525a88b08b53a2334afb808b08、5个Git对象同导出/SOURCE、GitGuardian成功后授权合并。原Installation manifest upgrade回执revision4/frozen_run_unchanged/private_configuration_unchanged=true；只改报告角色指令，运行二进制无需因模板改变重启。新预检的源码与标准导出/仓库升级已验证，但未另起新真实Run来宣称所有新失败恢复场景已过；M1使用冻结旧模板和正式清单返工完成，这个边界保留。

M2优先：脱离协调者的发布错误解释及恢复指引、8MiB完整日志导出/强杀恢复、未知远端副作用核对；M3再验证第二仓复制升级，M4依据实际配置做减法。当前不启动这些阶段；报告与草稿PR可供用户确认。真实供应商/支付/外部身份、生产隔离/多租户服务及所有旧产品缺陷不由本Harness里程碑替代验收。


按用户“跑通后再找我确认”的里程碑边界，model-relay heartbeat已通过automation_update正式PAUSED并读回确认；保持原每小时频率和配置，仅暂停继续推进，不自动开启M2。Issue48里程碑回写只含GitHub报告/QA/PR链接并使用现有agent-platform回写标记防止当新任务输入；PR49保持draft/未合并。材料PR47在核候选→最终PR49 ahead1/behind0后关闭为被替代，材料分支不删除。此为受托Builder确认，不冒称用户本人执行产品QA。

### 2026-10-15 — 以真实低延迟路由功能评估研发效果与瓶颈

用户要求在真实新功能上验证Pipeline，并分析效率或功能瓶颈。已只读核对仓库main、全部现有Issue/开放PR、正式Workflow和Run：现有routing.go按优先级筛选后平滑加权选择，observe记录故障冷却，没有近期延迟选路。Issue18是旧多上游/权重能力，不重复建设；无同目标新任务。Issue51从当前main正式入站，普通选择按项目已有自动推荐授权；可选方向询问已发，未收到变更时按已说明的低延迟切片推进。

本轮交付顺序：
1. Pipeline先形成紧凑需求/设计/测试计划，明确普通/SSE指标、观测过期、探索恢复、配置兼容和权限边界；在实现前确定对照样本/场景/阈值。正式wait_for_input供受托操作者检查，不以Builder代产品设计。现有模板没有独立需求/设计节点，本轮是development内先产计划的受支持检查点，不能声称已存在独立设计角色。
2. 实际Go服务/受控HTTP与SSE、真实管理页配置→调用→诊断→切回原策略，明确预期收益/代价；完整固定make verify与独立QA关键实操/审查分列。产品代码和测试由Pipeline写，通用设施失败修维护源并标准升级。
3. 正式approval/report/publisher交付草稿PR与同版可访问结果，Builder核远端版本及材料，统计效率/质量/功能满足度，再向用户报告。本轮不自动部署用户环境，不继承旧聊天明文provider凭据；真实供应商无正式私有引用时Not Run。

先行瓶颈分析（基于已保存Issue48实际Run created_at→updated_at，包含等待/恢复，不能称净执行时间）：整轮119.45分钟，prepare0.90、development10.94、tests34.13、QA46.62、approval2.55、report16.92、publish7.38。固定测试两轮各约17分钟是真实开销；QA包含原生完整日志取证受阻/平台修复及恢复，report包含模型容量失败，publish包含清单失败与返工。故不能把QA46.62分钟当正常审查成本，也不能由这些数据认定Agent编码是瓶颈。记录的两次完整固定测试、两次发布失败、容量失败和Builder干预是可核事实；无可靠原生费用数据不估造。

新轮按阶段驻留、可识别执行、等待/外部失败、真正产品返工、平台救援分别统计；功能效果按事前计划比较相同场景的权重/低延迟策略。不同任务不直接计算提效百分比。候选须给AC逐项结果、遗漏、QA缺陷和仍需人工动作，不能以PR存在证明产品成熟。

已知依赖：PR49未合并，main缺少其11项Linux/OpenSandbox测试设施变更（verify资源和期限、流读取、历史材料校验等）。Issue明确要求Pipeline计划阶段比较所需差异、必要时最小移植并保留来源/新验证，不盲复制全部旧材料、不自动合旧PR，不反复诊断已知问题。这是本轮环境准备/可复用交付瓶颈的观察项。没有为新功能改冻结图、预算或权限。

准备阶段量化观察：GitHub main完整树（truncated=false）共5606文件，逻辑内容501023998bytes，去重blob内容453380519bytes；docs/workflow/runs占5193文件497324594bytes（99.26%逻辑字节），产品internal约650635bytes、tests约701844bytes。此为树对象内容大小，不是Git压缩包/网络传输字节。入站Actions在00:36:50Z开始checkout、00:39Z仍未到SDK提交；两者相关性尚未证明因果，需实际步骤日志定位。历史证据与工作区下载耦合列为候选瓶颈，本轮不删除历史、改弱门禁或以未验证的稀疏检出绕过依赖。

本轮已证入站缺陷：Actions38661019216/38661019331均failure，checkout最终成功，SDK阶段在读取Issue后拒绝（日志安全摘要invalid input）。唯一维护源sandbox_entry.py只接受【Harness验证】，而既有AGENTS要求真实产品开发使用【产品功能】。首次创建前Builder漏查此限制，记录为协调者准备遗漏；不能把仓库体积当本次失败根因。正式by-request404且前置拒绝，无Run/产品副作用；原自动事件均完成，不盲重试。已按系统诊断/TDD新增六类别入站与原请求去重、非法标题/外部作者拒绝回归：旧实现5种类别真实失败，最小修复后10项entry回归实际0.679s通过，完整平台门禁和独立代码复审进行中。修复保持Owner/label/正文输出过滤/旧路由互斥/环境匹配；不把任务改名Harness来绕过。Issue已有原去重失败通知，追加Builder安全说明；后续必须标准源导出/测试仓Harness修复PR精确合并后才从同Issue正式dispatch复验。

入口分类修复验证：原类别硬编码回归5项失败；兼容性复审发现旧标题类别后空格/长目标不应阻断已受理评论，新增原层红后改为捕获非换行目标并校验strip非空，无新增长度限制。独立复审Critical/Important为0。修复初版完整scripts/verify.sh真实exit0，128745bytes/SHA6676769726782ffb357492148247eecc03786b3a46f4370e19763cb07f414f52（Go无改动、race有效缓存，链接器既有LC_DYSYMTAB警告如实保留）。最终兼容性版模板全套第一次失败是新增测试夹具漏填workflow_message回执，不是产品失败；补真实形状的conversation_url后重跑。标准导出/真实GitHub复验仍待。首轮Actions检出187秒，紧随其后的同仓缓存检出3秒；两次SDK分别4秒/2秒拒绝。该对照说明准备成本有环境复用差异，仍不把全部187秒归因文件体积。

通用修复正式交付：最终模板133项20.322s exit0/34721bytes/SHA b8dae997eb66150cc15f5a7b8dc7998b30bd4cf885b8c2515f73804ff517406f；源码80df20d76e16b5f1d2335ccdd79718c77129d085已推独立开发分支，标准export146文件，导出副本11项entry回归0.320s通过。测试仓Harness PR52/head49149d2a210260fec24069104251168d8b22d7f9仅三模板文件及SOURCE，4个Git对象与导出逐字节一致、GitGuardian success、精确授权合main84c000f8945d7532043f7aedd29e62dc4e86b866。平台二进制8ad9394与图revision4无变更，入站适配器由项目自有Actions执行，因此无需为此重启112或改私有配置。

再次by-request404确认无产品副作用后，00:49:00Z对原Issue51从新main正式workflow_dispatch一次，HTTP204已受理。待真实Run与原生承接确认；不把dispatch受理冒称入口已通过。

修复后的真实入口待执行：原Issue51 workflow_dispatch38662167083 queued/forward未分配runner；同标签[self-hosted,macOS,ARM64,he-full]的push prepare38662139638在model-relay-local执行“Verify and prepare exact main commit”。此为现有共享Runner池下的排队观察，不能当产品或Agent失败，不取消既有prepare/创建第二个入口。源与测试仓标准修复已完成，仍待Run受理/原生研发计划才证明本次真实路径通过。小时自动化锚点已更新，继续跟踪到真实功能草稿PR与效果/瓶颈报告。


### 2026-10-15 — 管理员密码最低长度调整

按用户明确要求，将账号管理 API 与服务启动配置的自定义密码下限从 12 改为 8，保留既有默认管理员兼容和上限。真实 API 边界回归先红后绿：普通账号/管理员 7 位拒绝、8 位保存后可登录；临时独立真实进程验证 7 位启动拒绝、8 位启动和登录成功。标准 scripts/verify.sh exit0（129058 bytes / SHA256 838b5a500e3e1ad5f2f0c84d4ff33389cb74956b811460d499e32373cc235463），平台 race78.421s、vet/build及原模板/浏览器门禁通过；保留已有 macOS LC_DYSYMTAB 链接警告。独立复审无 Critical/Important。当前待标准升级112:8795并用正式账号API修改密码；不修改数据库或其他实例。


部署复验：38ff741b91d1983946cbd30db3e7116de2f8adb7 已通过112标准build.sh/install.py升级8795（两者exit0），正式health同revision。管理员密码经/api/users修改为用户指定值，新会话/api/login与/api/me成功，私有0600凭据引用同步，不在文档记录值；配置、私有运行/CI/App资料、历史已完成Run与当前已停止Run逐值保持。升级前通过Owner stop将Issue51 Run6f3f26f6207ab1b6bfbc5ccd246797de/seq2正式停止并确认原生进程stopped；升级后Owner resume受理，仍seq2及原conversation4fc456bceb7debd872d395a3bbf5a9b2/冻结definition，状态running，非新Run或产品完成。源码直连GitHub首次push网络失败，使用用户既有10.1.20.20:7890代理后push成功；无Platform PR/main合并、无旧8793/8788改动。


### 2026-10-15 — 工作流画布完整呈现

用户截图对应 OpenSandbox 原生会话接续验证08，正式API确认3节点坐标均为(0,0)，真实浏览器证实相互覆盖，只显示最后的结束节点。修复唯一平台Web源码：未布置节点按入口/连线次序作视图布局，打开页面不改服务端图；显式整理、拖动或坐标编辑才写入可保存布局。增加显示全图、整理布局和缩放工具，节点/连线/指针统一留白坐标；按两维边界适配窗口，标题与操作区换行，窄屏无根页面横向溢出。

原层浏览器回归先红（unplaced nodes must not cover each other），修复后1440/1024/390全节点可见、保存按钮可达；64%缩放拖动40px后保存重载坐标63、pointercancel恢复位置和未保存标记、整理保存重载通过。最终真实Go-backed浏览器门禁5.563s、32项编辑器单测通过；完整scripts/verify.sh exit0、平台race77.489s、vet/build及模板门禁通过，完整日志129056bytes/SHA256 9836f19068389bc1ede40e5ab6254a9a55fa80c5fa06ae52349ca72d52705c93。最终小调整另重跑编辑器单测和真实浏览器；独立最终复审无Critical/Important。已有macOS链接器LC_DYSYMTAB警告保留。

源码8fef116；当前待标准build/install升级112:8795与实际页面复验；不在服务器手改坐标、不迁移冻结Run。Issue51已自然等待设计确认，定时任务保持暂停，本次不接续产品研发。


部署与原页面复验：源码1a8013524be52e0d96645ba84879df3521a758c3经112标准build.sh/install.py均exit0，8795 health返回同revision，二进制SHA256 fbe5ee6964c3220a40c9ff44fe656594b272c15dcc8f5ce461d6383d7dbbcb2f。当前waiting Run6f3f26f6207ab1b6bfbc5ccd246797de、历史completed Run、实例配置与私有凭据逐值保持；未恢复产品任务。原问题流程135f114b8228759aaefd7574bdda7151真实浏览器1440/1024/390下3节点均在画布可视区域、保存按钮可达、document.scrollWidth等于视口宽；原工作流revision2及三个(0,0)持久坐标在查看前后相同。实际截图output/playwright/canvas-after-{1440,1024,390}.png，安装回执与页面几何结果保存在忽略目录.data/opensandbox-verification/canvas-ui。仅独立开发分支提交推送，无Platform PR/main合并；8788/8793未改，定时任务保持暂停。


### 2026-10-15 — 详情覆盖层与画布可用空间

用户指出关闭右侧详情时仍像预留了大块空间。本轮检查区分两点：当前版本详情已为覆盖层，真实浏览器开关前后画布宽度/节点位置不变；历史CSS仍叠加三栏和流内详情规则，已从唯一维护源移除。另一个实证问题是全图仅贴左上角：旧版真实居中断言失败，新版以对称安全边界、视图偏移使全图居中，节点、SVG、拖拽/放置和入口换算一致，不改持久坐标。面板保持覆盖，不新增设置。

真实浏览器回归7.173s通过：1440/1024/390两节点居中、面板开关几何不变、overlay相交/无空右栏；十节点三列多行及7000起始坐标全图可见、回到入口与原图坐标保持；原缩放拖动取消/保存重载及实际MCP执行/环境释放仍通过。32项编辑器单测通过。完整scripts/verify.sh exit0、race78.789s、vet/build及模板门禁通过，日志129058bytes/SHA256 51b8e61c2766cea884bf63b0a8179bc380e04dbfde30fa9df81b9ac9d2ec2cd5，已知macOS链接器警告保持。首次受限环境无法listen属于测试环境限制，提升到隔离宿主测试后才得到真实布局红；不把listen失败算UI故障。独立复审无Critical/Important。

待标准升级112:8795并复验实际多节点流程，旧8793/8788不改，定时任务继续暂停。


正式升级及实际多节点复验：4e4a962e5759fb05bb701dea5be6a78adf0e976c已由标准build.sh/install.py部署112:8795（exit0），health同revision，binary SHA256 cd1fb80083fcdf74c4361b0cd0b8e47c6252dab40d9b6bd31528bb3adcbd2d7b。两个工作流、当前waiting研发Run、旧completed Run和私有配置/凭据均保持。正式8节点研发流程83f08f81f7040fcd0c12e6ff4e2acd2d/revision4在1440/1024/390下节点全部可见，画布右侧预留间隙0，节点包围框与视口中心偏差均低于0.04px；详情覆盖且开/关前后画布和节点几何不变。前后API全工作流逐值相同，无自动保存或产品任务推进。真实截图output/playwright/canvas-panel-{closed,open}-{1440,1024,390}.png，安装和浏览器回执在忽略目录.data/opensandbox-verification/canvas-panel。全图保持原节点布局；长横排仍需缩放查看细节，未自行改排用户持久坐标。定时任务保持暂停，未更新旧8793/8788。


### 2026-10-15 — Agent 节点上下文组装说明

用户要求明确角色指令与任务输入的关系，并能在节点上查看。Agent 节点详情标题旁新增问号，解释角色配置（Codex developerInstructions）、首次进入节点的规则→任务→已处理修正/回退→最近交接→各确认/执行节点最新回执、同会话后续消息、跨节点新会话与共享工作区，以及预览不包含全部原生上下文。旧式输入按其实际分支说明。此前“首轮 User Input 示例”改为工作流提示词；没有改变提示词拼接、权限或运行状态。

浏览器回归先因缺问号真实失败，新增后验证 Enter 打开、Esc 关闭/焦点返回、窄屏无溢出、帮助不改草稿，以及原画布和执行流程；删除临时测试节点前漏切交接页签的夹具操作已纠正，最终真实回归7.073s通过。32项编辑器测试通过，完整scripts/verify.sh exit0（race77.985s，vet/build及模板回归通过），日志129050bytes/SHA256 299f42855a2a6548ecdb6c864ad4ec603fb52aa21c9e08690cd9b4e923c6bf2e。独立复审无Critical/Important，正文与实际代码核对一致。待通过标准build/install升级112:8795并核实际页面；定时任务仍暂停，不推进产品Run。


正式交付：源码ff540bf3f16db6a46b2de59ce7508550e9e8e1e9已通过标准build.sh/install.py部署112:8795，health同版本，binary SHA256 be72336dc4672d818c8b4464edb575dd7d5b41fac145d0450629b7ee3aaa39cc。两个工作流、当前waiting Run、历史completed Run及私有配置/凭据保持。真实管理页面刷新后，研发Agent节点问号可打开完整说明、Esc关闭，输入预览使用“工作流提示词（Prompt）”并包含任务占位；原版本4保持已保存，无运行或保存动作。浏览器最初在升级前加载旧JS，未刷新时找不到新按钮；刷新后通过，未把该现象当作新版本故障。截图output/playwright/context-help-deployed.png；安装回执和完整验证日志在忽略目录.data/opensandbox-verification/context-help。只推开发分支，未建Platform PR或合main，旧环境及暂停的自动化保持。


### 2026-10-16 — 工作流画布快捷缩放与平移

按用户批准的小范围需求，编辑画布保留显示全图和加减按钮，增加点击百分比选择常用比例或输入2.5%–200%、Ctrl/Cmd+滚轮按鼠标位置缩放、空格拖动画布。Chromium触控板捏合采用Ctrl+wheel事件路径，Safari gesture事件提供兼容处理但尚无真实设备实测。缩放/滚动余量与偏移只属于当前视图，不保存到图，不产生dirty；表单滚动/键盘输入及节点键盘操作保持，节点拖动期间不接受缩放。运行详情的同类查看交互仍为后续范围。

复用现有CSS zoom及画布坐标换算，未引入画布框架或后端配置。原浏览器先因缺比例入口失败，新功能绿后补上下限、鼠标锚点往返、普通滚动不缩放、全图状态平移、节点上Space拖动不误改、取消/失焦恢复、表单不受影响、拖节点不缩放及原保存重载/执行路径。独立复审发现节点拖动失焦可能留下操作锁，已在blur/离开/重开/lostpointercapture清理并恢复坐标；补正在拖节点时失焦后仍可缩放的实际浏览器用例，最终定向7.487s通过，32项编辑器测试通过，复审无剩余Critical/Important。完整工程验证及标准112:8795升级回执待补；不恢复自动化或推进产品Run。


完整scripts/verify.sh exit0，日志129051bytes/SHA256 4d21b7e84fe4e18afb6993738d708f9e8b4766c652971feaef93c9e6423552ae，平台race76.731s、vet/build及模板回归通过。复审整改后最终浏览器回归7.970s、32编辑器测试重新通过；包括真实点击空白画布获得焦点后Space拖动，不抢按钮或表单Space语义。无剩余Critical/Important。下一步标准构建安装112:8795并实际页面核对，不将本地测试冒充已部署。


正式升级和页面验证：源码78bd46f6a8eec6ef34076d294c52b27f29900db9已标准build.sh/install.py部署112:8795（exit0），health同版本，binary SHA256 a31fd9ee114e56becdf9dbf1908d055001197183185cbdaa99bf9d563a3895ae。当前waiting Run、历史completed Run、两个工作流及私有配置凭据保持。正式8节点研发图真实浏览器125%预设、87.5%自定义、Control+鼠标滚轮及Space鼠标拖动通过，锚点误差0.349px以内、平移误差0；390px选择器无横向溢出。完整工作流API前后逐值一致，revision4/已保存保持，没有保存图或启动Run。源码、安装、页面版本一致；Safari真实触控板和物理设备捏合未实测，不扩大Chromium事件验证结论。截图output/playwright/canvas-zoom-deployed.png和canvas-zoom-selector-390.png；完整回执在忽略目录.data/opensandbox-verification/canvas-zoom。原SSH归档接收尚未完成时第一次摘要检查停止，待收到正确摘要后只提交一次构建/安装，没有盲重传。仅开发分支推送，无Platform PR/main合并，8793/8788及暂停自动化不变。


### 2026-10-16 — Chrome 触控板捏合响应

用户明确反馈 Mac Chrome 双指捏合需要较大幅度（不是卡顿）。原像素滚轮指数系数0.002改为0.01，相同缩放比例所需累计手势量降为原来的1/5；这不是帧率提升5倍。每帧累积输入并应用一次，跨帧保留浮点世界坐标锚点，避免滚动位置取整累积漂移。显示全图、手动比例、拖拽开始、失焦和离开编辑器取消待执行帧；普通滚动和表单、节点拖拽及2.5%–200%范围保持。复用现有画布实现，没有增加配置或后端协议。

独立Go/Chromium回归：50%起点120个-0.5像素事件，修改前scale1.12738/layout480/锚点误差31.892px并在响应断言真实失败；修改后scale1.82201/layout4/误差0.540px以内。40帧来回捏合恢复原比例，最大误差0.533px；全图与blur取消通过；合成Safari绝对scale1.2未被误累乘。最终定向8.67s、32编辑器测试通过。实际物理Mac手势体验仍待用户复验，合成事件不代表设备实测；完整工程验证、独立复审及标准112:8795升级回执随后补充。证据位于忽略目录.data/opensandbox-verification/pinch-smooth，定时任务保持暂停，产品Run未推进。

完整scripts/verify.sh exit0（vet/race/build及模板/浏览器全部通过），日志129058bytes/SHA256 eaff28a190ebf81ae2453dacdfa5c40b55bdc0760f6329030ad4a77b631c6aab。独立只读复审无Critical/Important；补齐复审建议的RAF后上下限断言和移动鼠标/滚动后锚点回归，最终定向8.50s绿（移动锚点误差0.355px以内）。本轮实际标准部署回执待补。

正式交付：e0136ca8830a357eb1d7e822c3bb03c9d3971ed8已标准build.sh/install.py部署112:8795（exit0），health同版，binary SHA256 a45cfcd3ab5e4e73c7d1d6c571d1e578162cf8464eb980b8091a6ea45f8c3180。当前waiting Run、旧completed Run、两个工作流及配置/凭据保持。刷新后的真实8节点研发页面：120个小事件scale1.82201、锚点误差0.066px；实际Control+mousewheel误差0.293px以内，平移误差0，比例预设/自定义及390px选择器通过；完整工作流API前后逐值不变、revision4/已保存保持。截图output/playwright/pinch-smooth-deployed.png已查看，完整回执在忽略目录.data/opensandbox-verification/pinch-smooth。这里只证明浏览器输入路径与部署，物理Mac触控板主观手感待用户体验；无Platform PR/main合并，不改变旧环境和暂停自动化。


### 2026-10-16 — 画布捏合边界阻尼与回弹

按用户确认的交互方向，正常范围继续使用Chromium手势比例反变换exp(-deltaY/100)，不添加速度加速或正常范围缓动；仅在2.5%–200%边界外提供对数空间有界阻尼（最多约8%视觉超伸），反向输入立即生效，无累计超伸债务。Chrome以140ms输入停顿判结束，160ms短回弹可被新手势打断；Safari使用gestureend，系统减少动态效果时直接限制在有效范围。比例输入/按钮、显示全图、普通滚动、拖动和失焦取消弹性，视图不保存到工作流。此为参考Apple交互原则的自定义实现，不声称复刻Apple私有曲线。

原边界浏览器断言先红，最终定向9.74s通过；事件拆分等价、上下界阻尼/反向、回弹中wheel及Safari手势接续、fit和blur取消、按钮有效比例、减少动态效果均覆盖，原锚点/平移/节点拖动/保存重载和执行回归保留。33项编辑器测试及完整scripts/verify.sh exit0，完整日志129156bytes/SHA256 9eaad972c069c1ffe5f2b64a94966a3f1d4bfd891c2dd21b70b61e2b94d3ed89，浏览器10.448s、Go race78.065s、vet/build及模板回归绿。独立复审Minor（±误用临时超伸比例）已修并补回归。实际标准112:8795部署及页面核对待补；合成Chromium事件不代替物理Mac触控板或Safari设备实测。证据在忽略目录.data/opensandbox-verification/zoom-elastic，自动化保持暂停，产品Run未推进。


正式交付：独立复审最终无Critical/Important，源码13cbfa8af3121921051cbb81781c791312e30354已标准build.sh/install.py升级112:8795，health同版、binary SHA256 43f581035622104bed611a3f1d7c6d007d9a211a87f97b5759fca892299a0a63；waiting/旧completed Run、两个工作流及配置凭据保持。真实部署页上界2.15239→反向2.10977→回弹中2.10397→新输入2.11766→最终2，下界0.02323→0.025；减少动态效果直接钳位。正常120事件scale1.82201/锚点误差0.066px、真实Control+mousewheel误差0.293px以内，平移/预设/自定义/390px选择器通过。完整工作流API前后逐值不变，revision4及已保存保持，未运行或保存工作流。截图output/playwright/zoom-elastic-deployed.png已查看；回执在.data/opensandbox-verification/zoom-elastic。实际物理Mac捏合及Safari设备未实测，不能将合成手势回归当作设备体验结论。仅独立开发分支推送，未创建平台PR/合main，定时任务保持暂停。


### 2026-10-16 — 清理临时验证工作流并补正式删除入口

用户指出临时工作流未清理。正式API只读核112:8795共9定义，8个临时基础执行/MCP引用/内存/原生接续验证流程，其Run为completed/stopped/failed；正式研发83f08f81f7040fcd0c12e6ff4e2acd2d/revision4及waiting Run6f3f26f6207ab1b6bfbc5ccd246797de保持。平台原无DELETE入口，不能用手改数据库删除。维护源补管理员DELETE+revision，事务内拒绝有活动/等待运行或过期版本；列表提供明确确认，历史默认折叠。定义逻辑删除保留最后授权用于原生历史读取，普通Owner和撤销授权边界不放宽；新建/旧编辑/Resume/Return均不能复活删除对象。Resume与Return写事务再次核定义，未删除流程原stopped行政收尾能力保持。删除不删除Run/日志/产物/环境，不触发供应商或产品任务。

API原405与浏览器缺按钮真实红→基本回归绿；独立复审查出硬删除损坏Owner历史和恢复竞态，两项均增加红→绿，补旧库迁移/重启、跨用户与撤销、终态写入拒绝、已删除stopped/complete恢复不变。首轮完整检查因只读列表夹具无querySelectorAll失败，已将删除绑定限定管理员；修复后的完整检查通过，后续复审修正纳入最终完整检查。旧stopped管理员收尾回归已保持，最终针对性集1.828s通过。标准升级及8个真实删除尚待最终完整门禁，不将源码绿冒充已清理；证据在.data/opensandbox-verification/workflow-cleanup，定时任务继续暂停。


最终完整scripts/verify.sh exit0，日志129148bytes/SHA256 c5c60549934ecea290c5cbbd7b9266419709785ca8c20e0d0e569528de207206，Go race80.031s、vet/build及真实浏览器/模板检查通过。独立复审原两项Important及stopped行政收尾兼容性回归均关闭，无剩余C/I；最终实现已在完整Go检查前完成。下一步标准升级112并执行精确8项清理。


正式升级与清理完成：d42c018fb6fed6a6eefb4b3d8878483f40136013经标准build/install部署112:8795，health同版、binary SHA256 b0701298bc934cca7bf3df21b0b265652a5e70fd0eee75351321243c4d2bfabe，升级前后配置/凭据及原Run保持。随后以名称+ID+revision白名单，通过正式DELETE API逐项清理8个临时定义（基础执行01–04、MCP check05正/缺输入、内存14、原生接续08），没有数据库或检查点手改。当前GET /api/workflows只余正式研发83f08f81…/revision4，14个Run详情逐值不变，8组历史入口保留。真实浏览器主列表只显示正式流程，历史默认折叠且原7289bb…Run链接可打开，waiting Run6f3f26…保持；截图output/playwright/workflows-cleaned.png已查看。未删除产物/会话、未释放环境、未恢复任务；自动化保持暂停。源码和最终维护记录仅开发分支推送，无平台PR/main合并。归档传输第一次核验仍在接收时已停止构建提交，确认摘要后只构建一次，无盲重传。后续临时验证结束应通过正式入口清理定义并保留历史；后续安装核对以当前正式列表为准，不再假设135f114…临时定义仍存在。


### 2026-10-20：规则交接原型进入平台实施

按用户批准的原型，执行节点增加「交接策略（基于规则）」：节点拥有 JSON Schema、条件/可选末尾 else、标题说明、目标和两类异常处理；画布仅投影实线，悬浮显示条件，多规则同目标分道。表达式从 Schema 联想字段及布尔/枚举值，保存与试算使用服务端同源校验；标准输出/工作区 JSON 文件/MCP structured result 接入实际回执，再按首条命中推进，读取/类型/求值错误不混入 else。既有无规则图及冻结 Run 不隐式迁移，转换需明确操作。

定向真实 MCP HTTP Run 验证冻结图与重复请求不重放；真实 shell/Python 经隔离 OpenSandbox API 夹具验证 stdout/file 路由，不冒充真实112容器验收。浏览器正式嵌入资源初漏新 JS 导致红，已修 web/assets.go；文件夹具未映射共享目录导致红，纠正夹具后回归通过。最终定向 Go0.746s、Chromium10.939s，包括保存重载、布尔联想、错误 Schema 拒绝、试算与真实 MCP 交接、1440/390呈现；整套标准验证进行中。

112 原 ds112 SSH 连接已超时断开，原跳板映射返回未发现匹配的用户名 moffett，直连112:22超时；未修改运行副本、图或在途任务，待恢复受支持连接后标准升级。用户已通过异步问题获知缺少连接。本轮不恢复定时任务，不推进产品Pipeline。使用契约见用户手册「执行节点：交接策略（基于规则）」。

最终验证：完整 scripts/verify.sh exit0，129580bytes/SHA256 2dbd88f870ca49b48e5132226a8fd3fa631cb5ee43e48dac412f74874df64796；Go race81.128s、vet/build、模板与浏览器全过。收尾补 Schema 字段路径冒充防护回归、校验报错名称及画布引导措辞后，定向 race3.647s、54项前端测试、Chromium10.835s再次通过；不将其描述为第二次完整验证。实现复核修正 MCP 下游输入覆盖，条件异常安全失败；无独立 Agent 评审声明。实际平台隔离预览8898/工作流43a15c669eecb75e0257b961c234010c已通过正式API保存及浏览器打开，页签、Schema、规则/else和异常实线可见，截图output/playwright/rule-handoff-local.png。本地预览未接112容器，实际工具执行由隔离HTTP回归验证，不能冒充112部署。112部署仍因SSH账户映射受阻，未改变其线上定义/在途任务；待连接恢复后标准安装。完整证据保存在忽略目录.data/opensandbox-verification/rule-handoff。


2026-10-20 部署复验发现规则试算竞态：c050dbf已标准build/install至112:8795，health同版且工作流/等待与历史Run/私有配置保持。实际浏览器快速改规则后立即试算，Schema错误输入未更新结果，仍显示上一条路由；重复点击后服务端正确拒绝类型错误。影响是用户可能误读过期试算结果，不是已证运行时校验绕过。假设为450ms配置防抖与显式试算共享请求标识，旧定时校验覆盖新试算；正在用延迟响应的隔离浏览器最小复现。未保存现场草稿、未启动产品Run，原失败回执保留，修唯一前端源后标准升级复验。

竞态根因与源码修复：字段变更的450ms防抖校验与显式试算共用结果标识，后触发的后台校验会使正在返回的试算作废；卡片重绘还遗留旧定时器。已在编辑器维护单一防抖定时器，试算/重绘清理旧定时器，旧面板回调不执行；修改样例输入使旧请求失效并清空过期结果。隔离真实浏览器延迟响应回归修前10.93s稳定红，修后12.556s绿，另覆盖旧样例响应与卡片重绘。第一次绿检脚本未展开重绘后的试算面板，已修正操作并保留失败日志。54项前端测试通过，完整scripts/verify.sh exit0/129582bytes/SHA256 9ee3c22b367f759dc0003525c2cb73525cfc2f773ad63211c6bc89b2bcca8384，浏览器12.755s、Go race80.273s、vet/build及133模板回归通过。待标准112更新和原操作复验；无新产品任务或临时工作流。


2026-10-20 用户确认 Schema 必须是完整交接输入，不能只展示 result 或隐去 output 根层。维护源调整为新规则的 schema_scope=input（兼容标记不作为界面选项）：默认完整 object→output→exit_code/is_error 与 result 示例，表达式路径与 Schema 路径完全相同，联想只列已声明字段；试算填写完整 input，和实际回执转换后的输入共用校验及路由。旧无标记 result-only 冻结定义保留原语义；编辑器展示等价完整 Schema，正式保存才记录新格式，不修改在途 Run。旧原型/平台的 scope 分裂已在前后端最小回归中实证红。57项前端、RuleHandoff定向与真实浏览器保存/重载/试算/MCP交接通过；新增完整输入MCP冻结/去重、根字段缺失/额外字段/元数据类型错误/不声明exit_code就不能引用回归。3a08712仅构建未激活；待本轮完整门禁后统一正式升级112，未创建临时工作流或产品任务。

完整根 Schema 收尾：标准scripts/verify.sh exit0/129833bytes/SHA256 53fd2eee878db42d65990ee5b1addd352ff6b1a427be621e37d57817771f59ae，Go race81.684s、vet/build及模板/浏览器回归通过，定向浏览器与Rules13.439s、57项前端测试通过。实际交接与试算共用完整输入校验，空范围保留旧Run语义；没有新增用户需要配置的兼容选项。接续从精确提交标准升级112并核实际页面，旧截图及c050dbf安装回执保留，不把中间版视为本轮最终完成。


2026-10-20 完整交接输入 Schema 已正式部署112:8795，版本5bd218fcebeac1c83e50e398b314b22c615efa75。经标准build/install激活exit0，二进制SHA256 31785e4fe0fa5438bc959ab99bf01f6b14b3cfdb013df81b54a48adc4c06cffb；等待Run、历史completed Run、原工作流与私有配置凭据前后保持。真实部署页面验证完整output根Schema、exit_code表达式、布尔与0/1枚举联想、成功→独立QA、else→研发、类型错误停止及缺少output根输入拒绝全部通过；规则实线及条件tooltip、390窄屏无横向溢出通过。首次浏览器脚本对缺根错误要求出现output字样，实际服务端已按额外根属性正确拒绝；修正该文案断言后完整复验exit0，原日志保留。正式工作流API前后逐值相同，未保存验证草稿、未创建新工作流或产品Run；独立验证浏览器已关闭。安装回执与浏览器证据位于忽略目录.data/opensandbox-verification/rule-handoff-complete，截图output/playwright/rule-handoff-complete-default.png及rule-handoff-deployed*.png。用户通过执行节点的「交接策略（基于规则）」→「配置规则交接」查看和配置；既有定义不自动改写。本机平台服务和定时任务不恢复。本轮证明平台规则功能及部署页面，不冒充新产品Pipeline验收。


2026-10-20 按用户确认的执行数据交接设计整改：命令/代码节点直接选择退出码、STDOUT、STDERR或工作区文件；退出码为整数，其他来源默认原始UTF-8文本，保留换行。可选JSON Schema校验同时启用解析，规则用$引用原始值，不再注入output包装；读取/解析/Schema/条件计算分别配置停止或目标，else仅用于正常未命中。旧空scope与input scope冻结定义保持原语义，本轮不拆分既有MCP节点。新增行为用例先红（旧来源/范围不支持、默认仍包装、原模板隐式分支），实现后定向真实进程/HTTP回归及Chromium15.228s通过；含真实make通过/失败、两路分离、文件、错误路由、冻结与去重，隔离HTTP设施不冒充112容器。浏览器覆盖来源/开关/文件路径保存重载、原始值试算、解析失败目标、旧MCP、1440/390。首次运行受本机监听权限限制后通过授权测试入口完成；夹具编辑规则时使done不可达已纠正为变更条件，保留原失败。标准sandbox_delivery.py固定make verify已改成显式退出码0→QA/非0→研发，项目不需JSON输出；补真实模板生成与Go路由验证。完整标准验证及112标准升级/真实沙箱复验进行中，不恢复定时任务或已有产品Run。

完整scripts/verify.sh本轮exit0，Go race82.908s及vet/build、真实浏览器、133模板测试通过；最终补充原始文本脱敏/UTF-8与大小上限、预览API和模板0/1/2/137路由定向0.426s通过。接续标准部署和112并发同路径隔离实测。


### 2026-10-20：原始数据交接部署与真实并发隔离复验

维护源8dd6344dffb334cac9417d082d9254ed85bbbab9已仅开发分支推送，并通过标准build/install部署112:8795；二进制SHA256 b487c2320609feb53b141ec9eedd69c2e02af6f00bd50fe28f17c98f3c7e1237。完整源码门禁exit0/129923bytes/SHA256 5e86b78e021b6845b1720640b37b3bd1d687ba6599e8f8fc95c8e1293e9d2452（Go race82.908s、vet/build、浏览器、133模板测试）。安装前后等待产品Run、历史Run、工作流及私有配置逐值保持。

用户补充文件隔离要求，已通过正式API在真实OpenSandbox同时启动A/e2fad7e5ee2df4ab91009bbf03d674ba和B/f02fb9aa0dd8ec6829ee94213afdb55f；两容器ID不同，相同/workspace/handoff-check/result.txt分别写A/B，写后持有时间实际交叠9.998515844秒，各自读取原值并进入对应分支。两次重复提交各返回原Run，未多建环境。同Run内节点共享文件；不同Run隔离，本机共用宿主目录不提供此保证。两Run均18步completed：真实make退出0/2、stdout/stderr分离、原始文件、JSON布尔/整数/对象、解析/Schema/表达式异常、缺文件、最终符号链接和父目录链接越界均按配置路由；else不接管这些异常。时间回执核验脚本起初按每行JSON解码，实际事件文本相邻拼接导致Extra data；改为逐值解码已有日志后确认时间交叠，没有重跑补造。

另外从当前产品Run的固定repository_commit=84c000f8945d7532043f7aedd29e62dc4e86b866复制准备节点和原make verify到隔离Harness Run38e6a404773bdb17bf80c0482564318d：prepare成功，tests3真实exit2，rule_2进入project_fail人工核对节点。完整输出21页至EOF，667779bytes/SHA256 aa786ac12ff5278f9eb43c4709fe178c54c3006368c3fee8b8572ad440428d70，truncated=false。报错review historical HTTP rejected: unparsed baseline output，158件源6e359667…前后未变；与历史同版测试设施失败一致，不冒称原项目全量绿、不代产品修复、不重跑求绿。此项证明实际项目非零交接与完整失败保留；成功分支是独立Harness真实make夹具证明，不是本次产品完整通过。

正式工作流83f08f81f7040fcd0c12e6ff4e2acd2d通过PUT从revision4升5：只从同版标准sandbox_delivery.py提取tests数据来源及规则，0→qa/非0→development，并投影两条实线；其余节点不变。原等待产品Run6f3f26f6207ab1b6bfbc5ccd246797de冻结定义与回执逐值不变、未resume。生成规则时占位凭据引用带连字符被模板拒绝，发生在PUT之前；改为合法占位标识后正式保存，不涉及实际凭据或权限改动。

通过正式decision记录产品门禁失败但Harness核验结束，三个验证Run均completed/环境released；临时工作流c0a5749a1feb6a8cb681f796c6a615d8已正式DELETE，运行历史仍可访问。实际部署页面检查退出码0/2、原始文本换行差异、stderr JSON布尔、解析/Schema错误、文件及开关、1440/390无根横溢出；界面草稿未保存。浏览器最初使用错误button/label选择器及离开确认影响测试脚本，改用实际tab/textbox并分开处理导航，没有修改业务断言。证据在忽略目录.data/opensandbox-verification/rule-selected-data，远端同名rule-selected-*回执与完整日志保留，截图output/playwright/selected-data-112-*.png。自动化及本机平台保持停止；没有新产品Issue、PR或部署。


### 2026-10-20 · Run 产物持久留存（实现与验证中）

用户要求释放沙箱后仍可按 Run 取得产物。确认旧实现仅从活容器读取文件，并非留存。已收敛为现有交接 `artifacts` 和执行节点可选文件清单：在本轮结束、下一节点开始前将指定文件保存到平台数据目录，按 Run/执行序号隔离，保留大小、SHA-256 与复制时间；不引入独立产物任务或对象存储。新 Run 默认启用，旧冻结 Run 不改写。

归档失败阻止交接；专用重试只复制文件，不重放原命令或 Agent。释放前检查声明文件副本；下载先验 Run 权限及文件摘要。运行页提供留存状态、下载和隔离 HTML 预览。边界为指定普通文件，不包含完整工作区、运行时内存或秘密；外部 TTL 仍可能先于未完成归档到期。

已先复现“未留存就推进”和“文件缺失仍推进”两个红例，再通过针对性回归验证修复、同名跨 Run 隔离、同 Run 多版本、原生回合结束前不采集、缺失文件重试、删除工作区后读取、路径/链接/大小限制、摘要损坏及跨用户拒绝。112 标准升级、真实容器释放后读取和实际页面验证待完成；不得用本地回归替代集成结论。


#### 本轮交付与实证完成

- 通用实现提交 `7ccc2a8ae5ecad69639bb5f8fdb6e750b60c2587`；UI 收尾 `076b62bfe5e35eed3b8b6a3b0a26041db618c5e9` 已按标准 build/install 部署 **112:8795**，实际 health 同版。平台仅开发分支提交推送，无平台 PR/main 合并。
- 标准 `scripts/verify.sh` 最终真实 exit 0，完整日志 129925 bytes / SHA-256 `8dd46dd93e3ab4986f2b007a955c4b52220f95d9149c39eda43c658e28a548e5`；含浏览器配置往返、133 模板测试、60 部署控制器测试、go vet、完整 Go race（平台 85.599s）与构建。此前一轮因测试名称在执行中改动导致构建中断，保留失败日志，不算通过。UI 收尾另经 12 项页面逻辑回归及112真实浏览器下载/预览、1440/390宽度验证，页面小文件显示真实B、时间可读。
- 正式隔离 Run A [`537fe7af1a30d614959fb0f365e5b971`](http://10.1.20.112:8795/workflow-runs/537fe7af1a30d614959fb0f365e5b971)、B `75b8a107cf7271276e4fc84ce036b08c`：共享各自 `/workspace`、同一文件名分别产生两版，四文件/Run 的实际 bytes/SHA/content 匹配；重复请求命中同一 Run。两个实际环境均正式释放后再从平台下载，旧版没有被新内容覆盖。
- 失败恢复 Run `caa2a624613de67bf61aa9eef5dc5140`：原命令 exit 0，但声明文件缺失使归档失败并停在 seq1；release 被拒绝且环境仍 ready。Owner 正式 stop→return 到预设修复节点，仅补缺文件并检查计数仍为1；原执行没有重放。随后通过归档 API 留存原步骤文件，保持实际晚采集时间及原失败/取消记录，再正式释放并下载成功。此晚采集不冒称原失败时已有完整快照。
- 同版标准重装/重启及最终 UI 版本标准升级后，三个 Run 的归档清单、SHA 和下载内容保持；17 个保留文件（含manifest）权限均0600。旧产品等待 Run `6f3f26f6207ab1b6bfbc5ccd246797de`、正式工作流 `83f08f81f7040fcd0c12e6ff4e2acd2d`、旧完成 Run及私有配置逐值保持。
- 临时验证图 `bd640ac819f809be403bcc0442ed8770` 已携原revision通过正式API删除，三个环境均 released，Run历史与归档保留。图最初因未连通修复节点被400拒绝、尚未创建Run，补齐交接后才启动；清理首次漏revision同样被400拒绝，读取原版本后正式删除，未绕过校验。UI截图定位曾误匹配多层section，已收窄选择器并重验；这些均非产品验收结果。

证据在忽略目录 `.data/opensandbox-verification/workflow-artifacts/` 和 `output/playwright/workflow-artifacts-*.png`；真实浏览器点击下载的HTML为第一版内容。当前只保证声明普通文件的持久快照与现有日志，未实现全工作区/内存检查点、对象存储、自动过期清理；外部TTL提前到期、断电时未完成归档的材料不保证恢复。未将这些Harness回归表述为Model Relay产品验收，也未恢复定时任务。


### 2026-10-20 · 执行节点产物保存说明

已在执行节点详情标题旁提供“?”（产物保存策略），复用现有帮助弹窗，解释文件清单、交接前保存、按 Run/执行次数隔离、失败后的归档重试、释放后访问和文件限制。明确用于规则判断的文件仍须声明留存，产物不包含进程内存/环境变量，归档不延长沙箱 TTL，当前没有自动到期删除。只增加说明，不修改工作流配置或保存策略。

实现 `430c9e8e521b0715f0683bf6c1460169e0202d07` 已标准 build/install 到112:8795，health同版；等待/完成Run、正式工作流、私有配置逐值保持，三个已释放Run的归档及下载摘要不变。45项页面逻辑测试和真实浏览器配置回归通过（14.417s）；112实际页面打开/关闭说明、1440/390宽度检查通过，截图 `output/playwright/artifact-help-{1440,390}.png`。浏览器唯一控制台401来自登录前的身份检查，登录后帮助正常；专用浏览器会话已关闭。未启动新Run、未恢复定时任务。


### 2026-10-21 · 统一执行节点交接配置入口

用户发现准备节点只展示成功去向和“配置规则交接”按钮，而固定测试节点直接显示数据来源及规则。根因是界面依据是否已存 `rule_handoff` 分成两种交互。现将旧 next/failed 路径按等价条件投影到同一规则表单，浏览/关闭/只读查看不修改图；实际编辑后才写入当前草稿，显式保存后生效。保留目标、说明及原结果文件收集设置；无法识别的自定义路径不猜条件或丢弃，显示问题供核对。

实现 `661b574bc068df99a2b11981d3298f7129046eb5` 已通过标准 build/install 部署112:8795并核health同版。复现检查先红后绿；55项页面/规则回归、18项图模型/面板回归及真实浏览器配置往返14.781s通过。112正式图上准备/测试两个节点均直接显示数据来源及规则；准备节点实际试算0进入研发、2停止，未保存图或启动Run。等待Run、正式图revision5、私有配置和三个已释放Run的产物摘要/下载保持；无本地常驻服务、无定时恢复。截图 `output/playwright/unified-handoff-{prepare,tests}.png`，本轮日志 `.data/opensandbox-verification/unified-execution-handoff/`。此次为平台UI与配置保存验证，不宣称新增产品整链验收。


### 2026-10-21 · 删除执行节点旧交接配置与兼容投影

用户明确要求除非另行指定，不保留旧配置兼容分支。此前 `661b574` 的 next/failed 表单投影方案被本次实现取代；历史记录保留，不代表继续支持旧执行方式。工程约定已写入 AGENTS.md。

- 实现 `3cb6b90a35c45cfaf9aa6c820fd943115a94b817` 删除执行节点旧配置投影和隐式成功/失败跳转，新增、保存与执行均要求显式 `rule_handoff`；缺失时明确拒绝，不从连线猜条件。Agent、人工确认与外部 Connector 的原有交接语义不在此次删除范围。
- 标准交付模板及 OpenSandbox 示例已同步显式退出码规则。112 正式工作流 `83f08f81f7040fcd0c12e6ff4e2acd2d` 经 revision 校验 API 从5更新到6：prepare的 `$ == 0`→development、publish的 `$ == 0`→done；未匹配停止，保留原意。冻结等待Run未改写或恢复。
- 标准 `scripts/verify.sh` 最终 exit0，日志130021 bytes / SHA-256 `45045ab64e02ef68bf482a6697deb70f0c8890e411804801d39ad29b6c75587e`，含133模板测试21.166s、实际浏览器配置往返13.717s、go vet、完整Go race（平台85.167s）与构建。前期测试夹具重复字段、浏览器目标/选择器修正及未设PYTHONPATH的单独调用失败均保留原日志，未当产品失败或通过。
- 已标准 build/install 到112:8795，health同版，二进制SHA-256 `7e7041b062af9cfa9ab5771a67a02487d2f1403366a333d50beee95eb8cf2cb1`。原私有配置、凭据、等待/完成Run及三个已释放Run的归档和下载保持。
- 正式隔离Harness Run [`f3fe21067c69b8cbb4d2f8a375deb049`](http://10.1.20.112:8795/workflow-runs/f3fe21067c69b8cbb4d2f8a375deb049) completed：缺规则的旧配置先被400拒绝；有效配置用同一共享工作区真实执行exit0→下一节点、exit7→结束，重复请求返回同一Run。正式释放沙箱后result.txt仍可下载且为verified；临时验证工作流已正式删除。此为平台规则与产物能力回归，不是Model Relay产品验收。
- 112真实浏览器检查准备、固定测试、发布三个节点均直接显示数据来源与条件规则，无“配置规则交接”按钮，未保存或修改图。证据位于忽略目录 `.data/opensandbox-verification/explicit-execution-rules/`、`output/playwright/explicit-rules-prepare.png`。未恢复定时任务、未启动本机常驻平台、未创建平台PR或合并main。


### 2026-10-21 · 节点标题直接编辑

用户要求节点详情顶部标题可编辑。实现 `a04c91c7d891fe9b6b3a38b0bf641cf7f6969bbc` 采用标题样式的原生文本输入，提供铅笔提示与焦点边框：回车或失焦确认、Esc取消，拒绝空白和超长名称；确认后画布同步、随工作流保存。移除配置页重复名称字段，各类节点共用；只读详情保持静态标题。

87项前端测试、真实浏览器配置往返14.192s、go vet及构建通过。浏览器覆盖标题位置、回车、Esc不关闭面板、空名称拒绝、交接页改名及保存刷新；首轮不可编辑红例保留，随后用例还原名称未保存引起的“取消拖动不应变脏”失败已修正测试收尾，原断言保持。此UI范围未重复全量后端race，不宣称新产品整链验收。

已标准build/install至112:8795，health同提交，二进制SHA-256 `b6fdf234c19a0cede7c9c69615bc1b4b473af9a80215ad0048695783344e5acc`。正式工作流revision6、等待/完成Run、私有配置与三个已释放Run的归档下载保持。112真实浏览器验证取消编辑及1440/390宽度，无正式改名保存、无新Run；截图 `output/playwright/inline-node-title-{1440,390}.png`，日志 `.data/opensandbox-verification/inline-node-title/`。专用浏览器会话关闭，未恢复定时任务或本机常驻平台。


### 2026-10-21 · 标题编辑图标与悬浮显示

按用户提供的形状，将节点标题编辑图标替换为线框SVG；默认隐藏，仅鼠标位于标题区域时显示，移出后即隐藏，输入框聚焦不使图标常驻。提交 `e70b36fa67956609a41335623bc194876189d412` 已标准build/install至112:8795并核health；正式工作流revision6、历史Run、私有配置和已有产物保持。页面逻辑回归通过，112真实浏览器检验隐藏→悬停显示→聚焦后移出隐藏，截图 `output/playwright/node-title-icon-{hover,hidden}.png`；未保存正式配置或启动Run。本次只改图标与CSS，未重跑产品验收。专用浏览器已关闭。


### 2026-10-21 · 执行节点高级设置与有界重试

按用户批准的节点控制方案实现独立「高级设置」页签：代码、项目命令和 MCP 共用默认 60 秒的单次超时，自动重试默认关闭（0–10 次，间隔 1–3600 秒），执行异常默认停止，可配置目标及连线说明。普通退出码/工具业务结果仍进入规则；读取、解析、Schema 错误保持独立。每次重试是新步骤，独立动作意图/日志/回执，计入原最大执行次数，并持久化等待时间。结果未知、取消和权限/配置失败不能通过重试或异常去向绕过。

已实证 MCP 连接失败→等待→重试成功仅调用工具一次，tools/call 后真实 HTTP 超时不重放；新引擎保留等待、停止不派发、总步数上限、耗尽异常交接及历史回执回归通过。真实浏览器完成高级页默认值、编辑、保存刷新和异常实线投影。87项前端通过；最终 MCP/执行 race 定向12.226s通过。首轮标准完整验证在既有运行分页用例失败：RFC3339Nano可变小数位的尾随Z使字符串时序反转，已固定时间戳复现红例并修正分页/分组排序，未改历史记录；定向转绿，完整复验进行中。证据保留 `.data/opensandbox-verification/execution-limits/`。当前尚未标准部署或112真实沙箱复验；不冒充产品Pipeline验收。


完成回执：实现提交 `b7d469ed537d7f2dc233b3a08206bd9413cc8202` 已开发分支推送并标准build/install至112:8795，health核同版；二进制SHA-256 `9b32d74b6e765af26cf468211292c3e2fd3e8da9755b4479333f35d7647dff4f`。最终标准verify exit0，Go race92.296s，前端87项、实际浏览器及安装/模板/控制器检查通过；完整日志130118bytes/SHA-256 `d05c636279a7417961093d9480dc1e3ed611ecc69d3620dcc5bb92d9604c1425`。首次全验分页失败与固定时间戳红绿记录仍保留，不将其写成一次通过。

112正式API隔离Run `7ec15e623d41b44084d50d167b68a930` 完成6步：3秒命令超时并确认停止→第二次执行成功；另一个命令两次超时耗尽→rule_execution异常目标；后续命令exit7按明确规则完成。五次原层日志分别228/367/228/228/406bytes，均通过format=text完整读取并核bytes、计算SHA；Run环境释放后recovery.txt仍可下载。另一Run `d6e82291d8769c2f26138b71c5abd5b4` 在第2步等待重试时正式stop，未派发新命令，环境释放。临时Workflow `be5f017e56f195b563b03aab4923394c` 已通过revision API删除，仅保留Run历史与原层回执。原始记录在112 `execution-limits-live.json` 和本地ignored验证目录。

真实浏览器核112正式Workflow revision6新增高级页，prepare原240秒保持、重试0；未保存正式工作流。截图 `output/playwright/execution-limits-112.png`，专用浏览器已关闭。标准升级前后既有等待/完成Run、正式工作流、私有配置及3个已释放Run的归档和下载逐项保持。MCP真实HTTP超时/连接恢复证明来自隔离平台回归，112现场证明来自真实OpenSandbox命令；二者均是Harness能力验证，不是Model Relay产品验收、供应商调用或新整链研发。未启用定时任务、未启动本机常驻平台。

### 2026-10-22 · 统一 Agent 交接工具与整链验证计划

用户已确认并授权实施：Agent 只使用 `handoff`，所有调用必填合法 `target`（包括单目标和结束节点）；删除固定/自主模式选择及 `complete_node` 工具。公共参数统一为 `target`、`message`、可选结构化 `context`、`artifacts`。交接信息 Schema 对所有目标数量一致生效，去重、用户输入竞争、原生退出、归档及执行次数门禁保留。执行节点规则与人工确认保持各自行为。

本轮按 writing-plans 拆解并在当前会话实施：
- [x] 通用运行契约：以真实 MCP 回归先复现单目标仍需 target、旧参数拒绝、嵌套 context/Schema、越权目标、相同/冲突提交及原生未结束不可推进，再统一工具入口。旧历史只读保留，旧配置明确拒绝或经受支持配置入口显式升级，不保留第二套工具。
- [x] Agent 详情：目标卡片直接编辑，删除模式选择；交接信息格式独立配置，问号为简短独立弹窗；高级设置与执行节点风格一致。覆盖草稿、保存刷新、增删目标、结构化数据、窄屏与帮助。
- [x] 标准模板及说明同步，固定报告节点改为显式 target handoff；源完整 verify、代码审查、标准 build/install 112:8795，核历史/私有配置/留存产物保持，再通过 revision API 升级正式工作流。
- [x] 先核正式 API/GitHub 现状和重复请求，再通过项目自有 Issue 入口进行新版本完整 Pipeline 验证：共享工作区、原生 Agent、固定 make verify、独立 QA、正式确认、报告和草稿 PR。产品工作交 Pipeline，Builder 不代 QA；产物与源摘要核一致。若受阻记录实际原因及安全恢复路径，不把局部绿/受理当整链完成。

风险检查：遗漏 target 与空目标配置不能变成结束；多目标不能绕过流程授权；类型/Schema校验失败不写结果；待送达用户输入不能被交接越过；同一执行不能推进两次。停用的定时任务保持停用；平台仅独立开发分支提交推送，不合main。

### 2026-10-22 · 统一 Agent handoff 源码回归

用户确认所有目标均由 Agent 显式填写 target，当前实现删除 complete_node 与双模式界面，统一 handoff(target,message,context,artifacts)。context 支持类型化对象及可选 handoff_schema，单一、多目标和结束节点共用目标配置；交接条件直接在节点面板编辑，高级参数独立分页，问号为简短模态说明。权限、用户输入队列、重复/冲突与原生结束后归档推进保持。历史 exit_mode/completion_* 只作冻结记录读取，新保存/启动/恢复拒绝旧契约，正式工作流待通过 API 显式升级。

定向真实 MCP 测试先红后绿，覆盖缺目标/非法目标/旧字段/额外 route/空 message、类型化 context、重复提交及原生未结束不推进。真实 Chromium 14.891s 通过，含目标增删、Schema/参数保存重载和帮助弹窗。完整 scripts/verify.sh exit0：133 模板、60 部署控制器、前端与安装/SDK回归、go vet、Go race 91.839s 及构建通过。Mac链接器 LC_DYSYMTAB warning 未阻止测试/构建。初期失败是旧契约夹具与新增浏览器图不可达/操作分页不一致，保留日志，修正后未降低产品断言。

正式完整日志 130053 bytes / SHA256 b274ee6d50aea1307c0b6659801eb8df9374b7b325e95e584ffad426b459e7fe，位于 ignored unified-agent-handoff/verify-second.log。112只读核验仍b7d469e/工作流revision6，原Issue51 Run仍waiting且无活跃conversation；没有部署本次版本或启动新Run。独立评审、标准安装升级、项目模板同步及真实Issue整链正在推进，不把本机回归当Pipeline验收。

2026-10-22 独立评审发现并关闭旧冻结Run执行边界P1：仅Start/Resume/startAgent校验不足，既有会话续跑、输入、accepted回执advance、Return及Claim仍可触发旧契约。新增统一冻结图交接校验：后台跳过保留历史；显式输入/交接/确认/回退/继续入口拒绝；Claim按消息顺序跳过旧候选且继续选正常任务，停止/取消不受阻。定向实际Store/Engine/MCP回归0.664s exit0，覆盖waiting及accepted回执不变化、原队列保持和正常任务不饿死。独立复审关闭P1、无新增C/I；重新Go vet/race验证中。没有在112改库或检查点。

- 2026-10-22：统一 Agent handoff 的历史 Run 执行边界回归完成；独立复审确认原 P1 已关闭，无新增 Critical/Important。最终 go vet ./... 与 go test -race ./... exit 0，platform 包 92.240s；历史排队消息跳过且不阻塞新任务、旧 Run 不变、停止入口保留均有隔离回归。完整 scripts/verify.sh 已通过；正式 112 升级及 GitHub Issue 整链尚待执行，不把回归通过写成 Pipeline 完成。

- 2026-10-22 01:04 UTC：统一交接实现提交9b03a305aa838f43b4db88e73ccdb9e15e8f608e，开发分支已推送；两处角色说明inputs→context补正63e06e592896248091cc3ba5c31190b198b86066，图契约3测试通过。112标准build/install成功，实际health revision9b03a30，binary SHA256602988ff44bff020077edc64cfabdca673fb8a763b6647c0ef19fd242dd32256。旧等待/完成Run、私有配置/凭据/工作流保持；3个已释放环境的归档及下载逐文件摘要保持。正式API保存工作流83f08f81f7040fcd0c12e6ff4e2acd2d由rev6升rev7，历史Issue51冻结Run不改。真实Chromium登录检查新版内联交接页、独立帮助窗口，截图output/playwright/unified-handoff-112.png（ignored）；未登录api/me401正常。
- 标准导出模板PR53 https://github.com/big91987/model-relay/pull/53，147个导出对象（含SOURCE）逐字节匹配63e06e5，head c5720e69cf61631ac5b393153eb7744765379ce4/GitGuardian成功后精确合并main868530b3e5c15d65bee0ed4ff6af69e96d428b81。平台无PR/无main合并。独立整链Issue54 https://github.com/big91987/model-relay/issues/54，请求github:big91987/model-relay:issue:5971076683；自动入站39703355037 queued、另labeled39703356253 pending，共享Runner先执行push prepare39703323199。此时正式by-request404，没有Run，不重复dispatch。任务目标为真实工具交接/固定make verify/独立QA/正式确认/报告/草稿PR；尚无整链通过结论，不恢复Issue51，不合并旧PR49，不部署产品。

- 2026-10-22 01:24 UTC：push prepare39703323199 success，deploy job skipped；不属于本次沙箱产品验收。Issue54 opened入站39703355037和labeled39703356253均success，同一真实Run197ab169bc1cad067466300038ab6fac（01:15:18Z，rev7），prepare1完成→development2/0d535a2e8561d11910495c0e0791f75a running；共享/workspace、原生Codex/gpt-6.1-sol、无待审批。入站排队约12分钟是共享Runner驻留，不是Agent净耗时。管理员by-request404受身份作用域影响，不能单独判未受理；采用Actions返回ID及直接Run API核验。平台机器人已回写prepare交接/研发启动。
- 研发原生事件26688实际make verify exit2，2026-10-22T01:18:08Z–01:19:34Z；main868530b，报review historical HTTP rejected: unparsed baseline output。原生事件26986读取state证据：source/candidate unchanged、owned_group_absent true、candidate_removed，runner664425bytes/SHA c8e1374c55d9a71b5d13b3a378f2dbf57310154d4d7d8d44ab7ed8cc3c717bba。此为Pipeline研发阶段原门禁失败/诊断中，尚非固定tests节点通过，亦非handoff工具失败；协调者未代产品修改或QA放行。原始API事件和会话材料私有保存，待正式产物归档复核。

- 2026-10-22 01:43 UTC：首个真实统一MCP handoff于01:29:37Z accepted，development2明确target=tests，message/context/artifacts均真实提交，context包含嵌套对象、数组、布尔和数字；01:29:47Z原生执行结束后归档，01:30:18Z才派发tests3。29个研发产物1411242bytes经正式下载API逐文件bytes/SHA核验。tests3完整输出1007152bytes/SHA3c317c58b3dae1c7cab8f8aa4531bf966a60b68cfbf9f4d5cbc8ecf2aecfe021已分页保存至EOF；exit2→rule_2→development4新会话3b6076fcd497d55585a946c3499ad23f。实际主失败TestRoutingRealHTTPSelectionFailoverAndRecovery冷却未排除上游，历史health红例有expected-red分类，不混为主故障。Pipeline正在修测试时间窗口，局部旧红新绿为原生自报，完整新测试/独立QA未完成。没有协调者代改产品或QA放行。
- Builder一次正式User Input message9738/request_id issue54:builder:task-reference:1补足Issue54 URL/编号，返回duplicate=false；原因是原入站正文没有提供当前Issue编号，Agent明确拒绝猜测。此为任务信息补充，不是修改冻结定义或绕门禁；是否已消费以原生消息/交接为准，后续模板入口应直接带入业务关联信息，记录为一次Builder介入。旧PR49候选不在当前clone且匿名读取404，Pipeline如实记录不可读，未伪称比较或自动合并。

- 2026-10-22 02:16 UTC：统一handoff的真实待输入门禁已复验：development4于01:44:36提交被拒，正常结束后同一会话收到User Input9738并处理，01:46:56再次提交accepted；01:47:05原生结束后再派发tests5，没有绕过排队或重复全测。正式归档API已下载seq2/4/6共164文件16167470bytes，全部bytes/SHA一致，旧归档未改变。
- tests5实际exit2，完整33页输出1074530bytes/SHA e7c2bcd4cff176b3907b1135099516cd89811a9c96c61ee8ca399228d4b64700，EOF且truncated=false。1280路由浏览器通过，390恢复阶段随12分钟总门禁截止而终止；不能记为完整绿，历史预期红夹具与主失败分开。development6在原固定make verify中将项目验证器总时限12→25分钟，补父deadline/取消/过期边界测试，外层执行1800秒与Makefile/race/业务断言保持；此为Pipeline基于阶段耗时的一次有界调整，合理性待独立QA复核，非协调者代写或无限增限。
- 新158源库存773ecb3d9f47655bfc3e1cf3c99f204e341f2a6b1559982c18c42c04d88a16d9；development6于02:10:22正式handoff(target=tests)受理，02:10:31原生执行结束，tests7于02:11:56启动。当前完整测试仍在运行，没有独立QA、正式确认、报告或新草稿PR完成结论。定时任务继续暂停。

- 2026-10-22 02:34 UTC：tests7于02:27:36退出2，非25分钟截止；原API完整34页输出1090507bytes/SHA3bb3ed307415d790e0fbfde139a6e71d48ac4bf8209e8ccef94085f23c332f12，EOF/truncated=false。实际full-console在first-self-hosted 1440px的SSE/proxy-call失败，1280同旅程及1440管理旅程先通过，source摘要773ecb…前后相同、gate_exit2、原清理尾部保留。自动rule_2返回development8/273dda67cdbc6cc708ecd76619af26b2；原生02:32:40定点复现自报Chromium无法回读SSE响应体，拟改为同请求旁路观察且保留断言，尚未归档/完整复验/独立QA。平台bot已在Issue54回写失败和研发启动，没有隐去红例或提前进入发布。

- 2026-10-22 02:45 UTC：development8真实handoff于02:40:30Z accepted、02:40:41原生结束，归档后02:42:49才启动tests9。新160源库存44107d4db03a944c27a7d9b36b03e1802a770b3219c9bae61b19f0d81d4795fd；本轮仅tests/browser/journeys.mjs及新增call-response模块/测试，原断言/生产代码/Makefile/race/预算保持。原生回执记录定点旧exit1 response-body不可读→同请求response.clone观察后三视口真实Go/受控HTTP exit0（56.623秒）、285前端测试绿，无第二请求或重试；完整tests9仍在执行，局部结果不代替门禁。诊断中间源码摘要缺失、按实际编辑重建的快照已明确标注，最终快照逐字节对应源码，不补造原始证据。四轮正式归档共284文件29073565bytes，下载API逐文件SHA/bytes相符，旧归档不变；独立QA/确认/PR未完成。

- 2026-10-22 03:06 UTC：tests9正式exit0→rule_1→QA10；完整34页1093069bytes/SHA868edba207d2a972ce2c0b67c3ce5ec2485ed49f61334908d6a7cae30bf0e4ca保存至EOF，truncated=false。主attempt20261022T024249Z-1811890136，full-console 244090ms/ok=true，三视口首次自托管旅程通过，原Go/race七包通过；源160件44107d…前后相同，CLEAN_END gate_exit0，候选Playwright/core1.59.1与Chromium147.0.7727.15实际探针通过。QA10于02:59:55创建独立会话7b70d2cc1fa48416c05136d2fb144c89，实际原生事件32265执行observer/runner-history/HTTP/diff/source前后均exit0，32269核8最终源码快照与实际源码一致，并读三视口10页结果。此时QA仍在检查，尚无最终Go、确认或发布；不把局部QA命令冒充完整独立浏览器验收。

- 2026-10-22 03:18 UTC：QA10正式No-Go/target=development，结论为技术验证通过但缺平台原生执行/会话/工具证据，非产品缺陷。其59个正式归档文件10399625bytes已通过下载API逐文件核验；报告明确未另跑整套浏览器或逐图视觉验收。03:08:54 handoff受理、03:09:15原生结束，03:10:18进入development11/90fae940c0e240b783d4381a2e826e73。首次补证脚本语法错误未执行，修正后旧QA轮次已交接而返回409，没有输入写入或状态绕行。
- Builder经正式User Input向development11提交11016（平台配置/会话/thread/受理投影）和11328（development8完整原生MCP参数及返回对象，14434bytes/SHA b4dd0cc847bf73335cfe14f2e87fafd6a9b4dc4fe24d8a860b65cd80b2afee07），均duplicate=false；这是平台侧外部证据，非QA自行API采集。原生thread.started未存model字段，明确不补造；安装源9b03a30在thread/start返回与所选model不一致时禁止发送turn/start，可形成配置+真实执行+校验代码链，后续报告角色尚未发生。当前冻结development仅能交tests，补证后直接回QA缺少自然出口；记录模板效率缺口，待核支持的正式恢复路径，不修改冻结图或无故重跑同版测试。全链/PR仍未完成。

- 2026-10-22 03:36 UTC：补证输入11016/11328/11625依次在development11原会话消费，期间两次handoff因待投递输入被正式拒绝，未派发测试。最终168文件/167摘要清单及源44107d…不变核验完成，03:30:11正式waiting，原生idle。一次正式stop异步进入stopping，初次采集断言预期立即stopped不成立，未执行return；核实际stopped（03:32:39）后只调用return，03:35:04成功创建QA12/bf5e04e20a37c7f1462fcbe16c50a8b7。未重复stop、改库/冻结图/预算或补造成功handoff；development11保留人工回退cancelled语义。QA12需独立复核原QA-10-E1，报告/批准/发布仍未完成。此轮实证揭示平台审计材料缺少直接交给QA的入口，以及模板仅补证返工缺少直接QA出口，不能写成无人干预整链。

- 2026-10-22 03:50 UTC：QA12正式PASS_TO_APPROVAL，QA-10-E1按来源明确的平台外部证据关闭；原QA10 No-Go原字节保留。175个归档文件17136275bytes均经正式下载API逐项SHA/bytes核验；初次读取归档仍pending/0文件，未当成成功，等归档完成才确认。QA报告明确模型未持久化字段/未直接QA平台API/未供应商鉴定、未另跑全浏览器或逐图视觉，不扩大通过范围。03:46:23进入approval13，Builder依据用户既有授权经正式decision(route=next)确认，仅允许报告和真实草稿PR，不批准合并部署。报告14于03:49:09启动，独立会话928ff2c0ddd2110c301368452ab7d0a2；当前报告及发布尚未结束。没有恢复定时任务。


### 2026-10-22 04:06 UTC · 统一 handoff 完整交付与释放后留存

Issue54 / Run197ab169bc1cad067466300038ab6fac已16步completed（01:15:18.392Z–03:57:55.348Z，162.62分钟含等待、归档与人工恢复）。report14真实handoff(target=publish)于03:54:25.500Z accepted，原生03:54:33.439Z结束；179个报告阶段归档17236979bytes核验后，固定publish15于03:57:44–03:57:55 exit0，完整1688bytes回执确认draft PR55：https://github.com/big91987/model-relay/pull/55 。head32213cf63995f6e3b06488bd0ec48c4ed7bc1b24，base868530b3e5c15d65bee0ed4ff6af69e96d428b81；远端递归Git树仅12个文件变化，253526bytes的对象摘要全部匹配已QA锁定8个测试设施源文件、2个研发材料及固定复制的QA/报告。标题相同，正文去掉固定publisher署名后与原报告逐字节相同；GitGuardian success。首次正文直接SHA比较因该署名前缀不等，核唯一发布器源码后按实际契约重核，不将不等误记为报告损坏或悄悄忽略。

真实报告与QA分别在PR的docs/harness/run-197ab169bc1c/report.md、qa.md；报告中提到完整RunID目录作为固定QA/报告目的地是文档路径误写，实际完整RunID目录仅存development.md及evidence-manifest.json。报告的“发布待完成”是report14生成时点，最终发布由publish15真实回执与远端对象证明；没有改写已锁定原报告。PR保持draft，未合并、未部署，不改变旧产品环境。

04:04:56正式POST environment/release，04:04:57.536Z实际released；随后在沙箱已释放状态经正式下载API重读全部7批697文件/73846444bytes，每件bytes/SHA及既有副本一致，证明材料留存在平台而非依赖运行中的沙箱。没有手工删除容器或修改状态库。标准升级、Issue去重入口、原生交接、规则失败返工、排队输入门禁、独立QA No-Go/复核、正式人工确认、报告/草稿PR及产物留存链已实际闭合；终点target的运行时分支由隔离回归覆盖，本真实模板的结束节点来自publish规则流转，不能冒称Agent直交结束已在产品Run发生。

效果与限制：4轮固定测试阶段驻留49.07分钟，最终完整make verify实际1024.360秒；3次测试失败返工、1次QA补证返工均保留。开发2/4/6/8驻留54.47分钟，QA10/12合计21.71分钟；development11补证与正式人工回退24.76分钟，approval2.77、report8.58、publish0.19分钟。驻留包含原生结束、等待和归档，不是Agent净耗时；没有可得token/费用，不估造。协调者通过正式输入补Issue关联、提供平台审计证据，并在仅补材料时使用正式停止/回退QA，不能称无人干预。后续需改善模板入口自动携带Issue关联、QA按权限获取完整平台审计材料、仅材料返工直接回QA的合法路径；本轮不修改冻结图掩盖缺口。独立QA执行了局部HTTP/race/观察器反向用例并审阅完整门禁，未另跑整套浏览器或逐图视觉验收；真实供应商仍未调用。定时任务保持暂停，平台仅开发分支提交推送。


### 2026-10-22 · 可选交接参数与节点应用事务

用户确认结构化交接用于下游程序参数化处理，统一名称params，不再使用context。节点新增handoff_params开关，默认关闭；开启时要求type=object的handoff_schema，提供handoff_instructions填写说明，Schema进入工具定义且同一份用于平台接受前校验。关闭后保留Schema/说明，工具不暴露params；context不提供兼容别名。固定target/message/artifacts解释由平台提供。参数保存为result.params，Agent输入展示同一内容，执行代码通过WORKFLOW_INPUT_FILE读取previous_results，MCP绑定路径/result/params/字段。真实HTTP MCP回归证明Agent受控交接→工具参数原样消费→下一Agent收到执行回执，包含旧字段/类型错误拒绝、冻结契约和未结束不能推进；该隔离回归控制原生推理，不冒称本轮真实Codex产品验收。

节点编辑改为独立副本，名称/角色/交接目标/执行/规则在应用后才更新画布草稿，顶部保存才持久化。取消放弃，关闭或切换节点时明确是否放弃未应用内容；无效JSON不覆盖已应用配置。原根因是输入直接写图对象，切换方式重建execution并删除原始JSON草稿。现在代码/命令/MCP、结果来源、文件路径、JSON Schema分别保留；execution_drafts保留未选表单并随正式保存刷新，只有当前execution参与运行。MCP初次输出为空的切换、修改流程环境后草稿仍读旧环境的关联问题一并实测修正。正式模板/安装默认及文档同步，不在运行副本手工修复。

定向回归先红（未应用即修改、来源切回文件路径丢失）后绿；63项编辑器/模型/规则测试通过。标准scripts/verify.sh exit0：真实Chromium完整配置/保存刷新/实际MCP调用/环境释放14.787秒、133模板21.091秒、控制器/SDK/安装回归、Go vet及全race93.823秒、构建均通过。完整130192bytes/SHA256 f28eb3dbc429abcfa15604ce8d5aff1368fed57a2d4ab665bc5f39a2e21b1743私有留存。Mac链接器LC_DYSYMTAB warning保持为warning，不冒充无告警。浏览器验证中曾因仍使用旧自动生效断言、旧环境读取、空MCP输出及重复对话框处理失败，保留具体修正和最终完整绿，不将失败隐藏为通过。

112本轮HTTP健康接口连接5秒超时，现有ds112 SSH pane发送只读检查及无副作用探针未返回，没有发送安装或配置写入。最后已核安装为9b03a30/主工作流revision7，此处只是上轮事实，不当成本轮现状。新源码尚未标准部署112，也尚未完成本轮真实原生Codex参数交接；升级包可从开发分支按标准build/install准备，连通后先核活跃任务再升级，并通过正式API升级保存的工作流，不改旧Run或DB。定时任务保持暂停。8897仅本机配置预览，直接复用正式web源码；配置原型不执行Agent/命令，不作为远端或产品验收结果。


### 2026-10-22 · 112正式升级与真实原生params验证

用户恢复112连接后先经正式API核实旧版9b03a30、主图revision7且无活跃会话。唯一维护源fdedb0a6649da873adff9edf339955e82cc48277的Git归档5457920bytes/SHA256 f9d5860b7962f96ed3925c47493b5fa9d4170fc8f887227ed30507f11062b5f3通过现有连接传输，标准deploy/platform/build.sh和install.py均exit0。实际health为fdedb0a，Linux二进制SHA256 77d3939c392bb88e567f072ce95b839d0a3f86ff28370371f62c4c2d60ad09e2；实际HTTP workflows.js与源码同SHA256 81ed353137c8a0f257d4d76600287a668db32c538e8a76187f42cee8e6881b51。旧等待/完成Run、Issue54完成Run及归档、私有配置/凭据和原工作流安装前后保持。随后正式PUT把主图7升8，所有普通Agent参数默认关闭，无原Schema需要转换，拓扑/执行权限不变；不改历史冻结Run。

隔离原生验证经正式API唯一启动Run18a76ca18b9390ef2cb2c80c15e9ad67（08:10:41.360Z–08:13:44.911Z，约3.06分钟驻留，非Agent净耗时），重复相同请求返回同Run。两个独立Codex/gpt-6.1-sol会话完成：author开启params并按Schema/说明提交字符串、整数3、布尔true及proof.txt；Python执行节点从WORKFLOW_INPUT_FILE读取result.params和共享文件，严格核对原样字符串/类型/内容后生成receipt.json，真实exit0；review关闭params，独立读文件和完整执行日志，生成review.md并handoff(target=done)。原生事件中两次handoff均accepted，review真实参数未包含params；最后结束节点完成。不是受控Claim模拟，也不冒称产品测试/真实供应商联调或新一轮完整业务Pipeline验收。旧Issue54完整交付结论保持独立。

执行完整日志587bytes、EOF=true/truncated=false，SHA256 1371bce3ece102c0f9690f340fe48f5e2315c4d0729a1c8f02ed97e844343839。最初Builder核验探针把完整日志误按单个JSON解析而报Extra data，原失败保留，未重跑节点；只读核实日志含OpenSandbox init/ping/execution_complete和stdout对象后，按全部连续JSON记录严格核对唯一业务对象与receipt.json一致。Agent读取seq1固定命令输出得到不存在，不作通过证据；Agent原生事件另行保留，不混同执行节点日志。3批3文件2621bytes经正式下载逐项SHA/bytes核验；正式请求释放后实查environment=released，再重读全部归档仍一致。临时工作流2820ad1e5e01d2f3457f3031ecfb2fb8经正式DELETE删除，完成Run和归档仍可查看。

项目自有模板从同一维护源标准导出，147对象逐Git blob核对，无项目定制被覆盖。Harness PR56 https://github.com/big91987/model-relay/pull/56 head a7ae5948c4cc6ecddb9acb0a5b29dfe3ceca59cb，11文件仅模板/安装默认/角色说明与SOURCE；精确授权合并main4eade601b7a46c0cb62dc37f4ba0e7c4a8155675。该head无GitHub checks/status记录，不把无检查写成GitGuardian通过；验证依据为已记录维护源完整门禁、Linux标准构建安装、147对象和本轮原生实证。未改产品或合产品草稿PR49/55，平台仅开发分支提交推送。私有证据位于ignored handoff-params-editor目录及112部署证据目录；定时任务仍暂停。


### 2026-10-22 · 按交接目标独立参数契约（源码完成，远端待连接）

用户要求同一Agent的不同交接目标各自决定是否接收参数，下一节点可以是Agent或代码执行，不按类型硬编码。实现采用节点handoff_parameters，以已有目标ID作键，每项enabled/schema/instructions；参数开关、格式和填写说明放在对应目标卡片，连线仍只呈现交接策略。handoff保持target/message/artifacts/params四项，无新增工具或隐式目标；完整工具Schema按target分支要求对应params，平台接受前按实际target再校验。关闭目标拒绝params，开启目标要求存在且匹配自身Schema；不混用其他分支格式。参数仍原样进入result.params，供后续Agent、代码或显式MCP绑定消费，不拼Shell。

节点级旧参数执行逻辑已移除，仅保留历史JSON事实读取和明确拒绝新配置/执行的字段；标准模板删除旧默认字段，安装和说明同步。不能自动猜测分支需求或改历史Run。目标修改保留该卡片配置，移除目标清理其参数；未触及目标保留，原始JSON草稿独立，所有目标校验成功才应用。

回归先证当前实现错误拒绝合法分支params，再完成实现。66项编辑器/模型/规则回归通过；独立复审发现合法constructor/toString节点/目标名读到对象继承属性导致草稿串用，以及空object Schema对typed nil map校验缺失。已分别用自有属性检查和显式nil拒绝修复，保留{}合法，回归及复审确认无新增Critical/Important。首次新增VM夹具改节点ID后未更新选中节点导致无onchange，修正夹具后通过；未将夹具失败冒称产品故障。测试首次因沙箱不能监听临时HTTP端口失败，按测试所需正常授权环境重跑，不绕业务门禁。

最终标准scripts/verify.sh exit0，完整130493bytes/SHA256 15b0029ac5a678dc80130437fea53263a0bc536ba1033431dfead0682b11a7ca，真实Chromium15.107s、133模板20.966s、Go vet、全race96.538s与构建通过。前一轮门禁执行中还有复审修正，以独立最终完整门禁为准；Mac LC_DYSYMTAB警告仍按警告记录。真实HTTP/MCP验证包含不同目标格式混用拒绝、关闭目标带参数拒绝、冻结契约不随保存改变、实际参数绑定消费和下一Agent收到执行结果；不把受控Claim回归写成本轮真实Codex运行。

部署前只读检查时，ds112 SSH被jumpserver远端关闭；其他现有112会话也已退出，本机直连10.1.20.112:22五秒超时。HTTP健康仍200/fdedb0a。没有向本机shell误发安装、没有新112构建或配置写入，也没有重复任务。已通过异步输入请求用户恢复原连接；三条原生验证（代码目标独立版本参数、Agent目标独立审批参数、无参数Agent目标）脚本已准备但尚未启动，不预填结果。后续先核活跃任务，再标准build/install、正式API升级主图8→9、真实分支验证及归档释放/删除临时图。平台仅开发分支提交推送，未建平台PR或合main；测试仓同步PR待标准导出，产品PR49/55未动，自动检查仍暂停。

- 本轮源码6c7f960e1415294a70ffe23212a1c4d1b97d020c已开发分支推送，GitHub Connector独立compare确认远端相同。标准导出147对象和5498880bytes源码包已准备，包SHA256 665732ec737ac23673e76da94ded69efe269d49bd119f1a42442c9ff5b7e96fb。测试仓同步首次读取main在任何写入前TLS握手超时；一次有界重试完成旧SOURCE/项目Git树核对，但创建Git tree请求再次TLS握手超时，仅保留tree_intent，未收到树对象回执，未执行后续commit/ref/PR创建，不盲重发未知写入。备用Connector读取测试仓该main返回404，单独记录权限可见性结果，不据此改写仓库或网络原因。模板尚未正式集成，112新安装及原生三分支验证仍待SSH恢复，不能标记交付闭环。


### 2026-10-23 · 按目标参数正式部署与三分支闭环完成

用户恢复tmux后先经正式API核对112旧版fdedb0a、主图revision8、无活跃会话，且本次包/构建/Run均不存在，未重复启动。精确6c7f960源码包5498880bytes/SHA256 665732ec737ac23673e76da94ded69efe269d49bd119f1a42442c9ff5b7e96fb传输核对后，标准Linux build/install均exit0。实际health为6c7f960e1415294a70ffe23212a1c4d1b97d020c，二进制SHA256 85515dc3beff6d65b5655c5cda705b99b174cfa3bbd19e8929eaa7ad535c62a8；112实际返回workflows.js与维护源同SHA256 5086fcb4ffbfd0693836aa1f526d632e8fcebd6cfe869a345bb3dde060891e7e。旧等待/完成Run、Issue54和上一参数验证Run及归档、配置/凭据保持。主图正式PUT由8升9，确认原参数为空后删除旧字段；无目标需求被推测、拓扑和权限不变，没有改历史冻结定义。

临时Harness图7950d126852224dafe4229ea7ae07fe5的同一冻结定义，正式API各启动一次三条Run并实测同请求去重。所有角色仍为原生Codex/gpt-6.1-sol，不由Builder代交接或代QA：

- execute / Run7081a0f7ddac2b8ba8eee2a18a831660：author→代码执行→独立review→结束，4步completed（06:42:49.886Z–06:45:52.235Z）。提交commit=fixture-commit、limit为整数3，代码从WORKFLOW_INPUT_FILE读取真实params和proof.txt，严格核对后真实exit0，生成receipt.json；review实际核验后直接handoff到done。3文件1135bytes。
- peer / Runc70f3455b2e558d740896b435cc68f2e：author→另一个Agent→独立review→结束，4步completed（06:42:49.893Z–06:47:04.738Z）。该分支Schema要求topic=agent-branch、approved为布尔true，与代码分支格式不同；下游Agent真实读取参数和共享文件、写receipt.json再复核。3文件1287bytes。
- review / Run80715f424e3a41483436807c6b20e72a：author→review→结束，3步completed（06:42:49.900Z–06:45:39.991Z）。只交message与proof.txt，真实工具参数中不含params，不要求执行回执文件。2文件947bytes。

原生事件逐次核对共7次handoff、均唯一accepted；author的target与选择分支一致，前两分支参数内容及字符串/整数/布尔真实类型正确，后续非参数目标均不含params。不是只看Agent自报；平台结果、实际文件、原生工具arguments/result和执行退出码相互核对。三条Run各自proof内容、receipt run_id和参数一致；完整8件3369bytes归档经正式API逐项大小/SHA核验。正式请求释放3个环境，实际released后重读全部归档仍一致；临时图正式DELETE删除，完成Run和归档仍能读取。原生执行期间没有请求审批/重启/恢复/改冻结图或人工补输入。本轮是Harness能力验证，不是产品用户旅程、真实供应商测试或再跑完整业务研发Pipeline。

前轮TLS中断只记录到tree_intent；恢复时先只读确认无模板分支/PR，重新核旧SOURCE和项目Git树，再重建确定性未引用Git树，逐步保存tree/commit/ref回执，没有盲重放PR或合并。标准导出147对象与远端逐Git blob匹配，8个Harness/安装/说明/SOURCE文件，唯一PR57 https://github.com/big91987/model-relay/pull/57 / headdb7756b62235a118c958c7ef22943c5d5b98f9e7；GitGuardian SUCCESS且mergeable clean后按精确head授权合main91a50516da2bc80b75a60f4b55b4f0a775f7fe8f。未创建平台PR/合平台main，未合产品草稿PR49/55或发起产品部署。私有运行与归档回执在ignored target-handoff-params目录及112部署证据目录保留；旧网络失败、源码红绿及复审整改记录不覆盖。定时任务仍暂停。

### 2026-10-24 · 完整系统提示词预览正式交付

用户反馈 Agent 输入预览只展示部分内容。根因是预览接口只生成工作流用户消息片段，没有读取角色指令，也没有展示原生配置中的开发者指令和平台追加约束。

- 维护源 `ffcd93d106d15ad673297c1939133caa8e8d26bc`：`system_prompt` 由实际原生 `thread/start` / `thread/resume` 共用的组装函数生成；`input` 单独表示工作流用户消息示例。界面保留未应用草稿预览和过期请求隔离，问号说明两部分的来源和拼接顺序。不移动真实提示词的发送位置，不将原生内置提示词、项目指令、Skills、工具定义及历史会话冒称为配置预览中已加载的内容。
- 普通用户按其获授权的已保存工作流生成完整预览，不使用 HTTP 返回的已删减执行配置；共享 Agent 仍检查调用权限，已知凭据脱敏，不返回整个私有原生配置。无运行材料的最初协议明确引导查看运行记录，不输出不完整的系统预览；移除预览中不再使用的 `session_prompt` 重复字段。
- TDD 原始失败确认为系统预览缺失；修后原生新建/恢复和模型拒绝回退四种传输路径逐字核对通过，权限/凭据及前端草稿回归通过。真实 Chromium 最终门禁15.372s通过，完整 `bash scripts/verify.sh` exit0，含133模板测试22.637s、Go vet、Go race96.123s及构建。完整日志130491bytes，SHA256 `e357d7c4f4f3a026d16819fa51ff88373d112354c719882b7c012d56a932d155`，保存在忽略目录 `.data/opensandbox-verification/full-system-preview/verify-final.log`。
- 新增浏览器用例最初漏展开高级配置，随后在未连入入口的节点上请求预览，分别留下失败/诊断日志；调整用例以使用合法已保存图，平台原图校验不放宽。独立复查发现普通用户预览缺角色和历史权限可能误当已发送内容两项P2：前者已修复并回归；后者撤回新增的历史系统提示词重建，保持原历史展示。复查无剩余Important/Critical。
- 通过精确源码归档5529600bytes/SHA256 `52445fb752df87becf3786afa9410649ae3662fc190b31e25dda403b889d0abe`，在112执行标准Linux CGO build和install，均exit0；二进制SHA256 `2dace2f24103629ec0c100b87e88ad4dd91056c2ad2496852c06f9ae48f93bbd`。实际health revision为ffcd93d。正式主图 `83f08f81f7040fcd0c12e6ff4e2acd2d` revision9保持，原waiting/完成Run、最新三条分支Run及归档、账号/凭据/instance配置前后保持；不操作数据库或冻结状态。
- 线上正式预览API核对development/qa/report分别642/587/758字符，完整角色均保留；临时草稿同时含原生开发者指令和角色、用户消息分离，正式图前后相同。实际 `/workflows.js` SHA256 `c15c171a49afeeadebd43373c1026360f592a52b323695c06918e000772ffc6f` 与已验维护源一致。线上回执私有保存于部署目录 `full-system-preview-live.json`。本机真实Chromium与部署后的API/字节核验分列；桌面浏览器连接一次30秒超时后停止，不声称本轮独立登录线上浏览器实测。
- 仅平台开发分支提交/推送，无Platform PR/main合并；模板定义未变化，不重复测试仓升级PR或新建产品Run。既有产品PR49/55及产品/供应商验收结论不变，定时任务保持暂停。

### 2026-10-24 · 三个Agent角色提示词改为可维护Markdown

用户指出当前维护工作流的提示词是一整段，不便阅读。维护源 `3c0a601cabc187ea25308839def30ff18d6e8a77` 把研发、独立QA和交付报告的角色指令分别放入 `prompts/sandbox-development.md`、`sandbox-qa.md`、`sandbox-report.md`，公共归档要求放入 `sandbox-common.md`。生成器读取正文并写入节点角色指令；职责、步骤、失败处理、产物清单、报告字段和交接目标通过标题及列表组织，不引入新阶段或配置层。

- 按原指令逐项核对语义；独立复查指出QA应继续禁止修改全部产品，已将“不修改产品代码”恢复为“不修改产品”。模型、权限、工具、代码和连线均不改。
- 现有133项工作流模板回归24.379s、25项沙箱相关回归5.817s及最终生成/导出2项1.665s均通过，ruff检查/格式通过。日志保存在忽略目录 `.data/opensandbox-verification/markdown-agent-prompts/`：模板日志34721bytes/SHA256 `c14a9ead114762f2b1531d9feae111ff1f62a4d447d18a86a21ef8f78a439d7d`；沙箱日志8940bytes/SHA256 `2fa6c50a7a7cc9607fade64f31b44a19da9f5c0bcdeab2d3018039d50365de8b`；最终日志477bytes/SHA256 `f4b9ea304d60187ab676000136df52b20ce47eae8906d8d54d44b3c7b6d57cfb`。本次未改平台Go/前端，不把前轮完整门禁当本轮新执行。
- 先核112正式图revision9的三份原指令摘要与维护源一致、没有执行中会话，再通过正式PUT更新为revision10；只改三份 `agent.instructions`。研发/QA/报告正文分别604/472/693字符，完整系统预览均包含原文及平台约束。再次GET逐字核对已保存Markdown标题、换行和列表；6条旧Run完整快照前后相同。平台实际health仍为 `ffcd93d106d15ad673297c1939133caa8e8d26bc`，未重启或修改数据库。私有升级/预览回执留在112部署目录 `markdown-agent-prompts-upgrade.json`。
- 唯一维护源标准导出151对象，新增四份Markdown文件；逐项检查旧SOURCE/项目Git对象，未覆盖测试仓定制。[模板PR58](https://github.com/big91987/model-relay/pull/58) head `eaf370f87ceb7c875eb39a00eddbb3f3b183e1a5`、7文件，GitGuardian SUCCESS，已按Harness升级授权精确合并为 `12df9eb7a8522a679e3ccf98ca4e22b019774c5d`。源仓仅开发分支提交/推送，无Platform PR或main合并。
- 这是提示词结构整理、模板装载和正式配置预览核验，没有新增产品Pipeline运行、产品验收或供应商实测结论；旧产品PR49/55及冻结任务状态不变，定时任务保持暂停。

### 2026-10-24 · 执行节点工作目录与环境变量实施计划

**Goal:** 用户在执行节点直接看到执行环境和工作区位置，能为代码/项目命令配置工作区内的子目录及本次进程环境；运行记录显示实际目录和变量来源。

**Architecture:** 沿用 Run 的共享工作区和 OpenSandbox。`execution.cwd` 为可选工作区相对目录，`execution.env` 保存普通字符串变量，现有 `env_refs` 继续引用管理员值；同名冲突和保留名拒绝。MCP 不接受这些本地进程设置。结果文件及归档路径仍基于 Run 工作区根目录，旧Run和配置不迁移。

**Tech Stack:** Go、现有OpenSandbox SDK、Python目录检查及exec、现有原生前端和Chromium回归。

**Spec:** 本轮用户已确认的cwd/环境变量方案及“更直观理解工作区位置”。

- [x] 先写失败回归，覆盖越界cwd、环境变量保留名/引用冲突、配置保存及节点草稿留存。
- [x] 后端增加cwd与普通变量，校验相对目录和环境边界；实际执行前检查目录存在及链接，工作区外目录不启动项目命令。回执保存目录和变量名，不复制凭据值。
- [x] 前端直接展示工作区与完整执行目录，提供普通变量及凭据引用；Apply/Cancel、执行模式切换和重载保持配置，MCP清楚说明由工具服务管理环境。运行记录使用本次回执和冻结配置展示。
- [x] 验证真实进程cwd、变量值与下一节点隔离、非法/缺失/链接目录及MCP限制，保留结果文件/归档的根目录语义；执行标准门禁与独立复查。
- [x] 标准构建/安装112，核现有图和历史保持；通过正式API启动隔离Harness执行回归，核回执/持久产物/释放后下载，清理临时图，记录证据。


### 2026-10-24 · 执行位置与节点环境正式交付

执行节点现在直接展示执行环境、Run工作区根目录、可编辑的工作区相对cwd及完整执行路径；普通变量和管理员引用分开配置。`execution.cwd`、`env`、`env_refs`在保存/重载、应用/取消和模式切换中保持，MCP明确由工具服务管理位置与环境。运行记录展示本次派发目录及变量名/来源，不返回引用的值。结果文件与归档仍以Run工作区根目录为基准。

维护源 `4d466cbb42093624e87ee997b95d8da586f7f3c8` 已推送独立开发分支并标准构建安装至112:8795，实际health与三份前端资源逐字一致；正式主图revision10、旧waiting/完成Run、归档和私有配置保持。通过正式API创建唯一隔离Harness Run `72238948c5937bce4fc7172c394a2126`：7步completed，真实子目录执行、普通值原文传入、引用存在、下一节点回根目录且不继承变量；缺失/链接目录两步预期exit1并证实未运行项目命令。4件377bytes产物逐文件核对内容及SHA，正式释放环境后再次下载一致，临时工作流已删除、Run和归档历史保留。

本轮完整标准门禁通过；末尾前端草稿键处理增量又单独通过75项前端及真实Chromium16.060s，独立复查无剩余Important/Critical。完整测试与部署细节见 `workflows-verification.md` 同日记录。本轮是平台执行能力回归，没有新增产品或供应商验收，没有Platform PR/main合并，定时任务保持暂停。


### 2026-10-24 · 工作目录收敛为一个可编辑路径框

用户指出根目录、相对工作目录和完整执行目录重复展示。维护源 `d9a9564050d825b02c3ae00e2364d94b94828681` 删除这组重复事实与提示，只保留“工作目录”输入框，默认值 `/workspace`，直接编辑沙箱内完整路径；目录共享/隔离说明放入现有问号帮助。界面保存时转换为原有工作区相对cwd，执行边界和归档基准不变；不增加新配置格式或兼容分支。非法路径不能应用，模式切换保留未完成的输入，取消不影响已保存配置。

75项前端回归及真实Chromium15.325s通过，截图已检查；112标准build/install与实际health均为d9a9564，主图revision10及历史/配置保持。正式API隔离Run `b2857346f804bd017dd104938ab47f33` 7步completed：子目录真实执行，两个错误目录预期exit1，4件377bytes产物释放环境后仍可下载且摘要一致；临时图已删除，历史保留。线上前端与精确源码逐字一致。源码仅开发分支提交推送，未发产品PR/部署或改变产品QA结论，定时任务继续暂停。详细证据见workflows-verification.md本节。


### 2026-10-24 · Issue59新版完整研发流水线复验（进行中）

用户要求从头到尾测试当前流程。先核112实际health `d9a9564050d825b02c3ae00e2364d94b94828681`、正式图revision10、无执行中会话；旧Issue54 Run已completed，Issue51旧Run仍waiting，未复活或改写。测试仓最新main `12df9eb7a8522a679e3ccf98ca4e22b019774c5d`，旧PR49/55仍draft未合。

独立关联Issue59 https://github.com/big91987/model-relay/issues/59 经项目自有opened/labeled Actions40103846715/40103846692均success，去重为唯一Run `09ce99d8fe8bf76d8e4f5024ab74e293`，入口冻结同一main。14:44:58Z创建→14:46:02Z准备完成（约64秒阶段驻留，非净Agent耗时），共享/workspace内准备成功；development2原生conversation `ca82a9c5057ede55b26f5b8fdcd30ebe` running，模型Codex/gpt-6.1-sol，无待审批。受理及running不是完成。

验收范围为本轮真实prepare→development→固定make verify（自然失败按规则返研发）→独立QA（真实缺陷才返工）→正式确认→report→固定publish草稿PR，核对原生handoff arguments/受理、完整测试EOF及同版源码、QA实际材料、发布Git对象与报告一致；最后正式release并下载核对归档。Builder只检查Harness证据，不代产品/QA，不人为注入缺章，不合旧草稿或部署产品。不将本轮正常链当作已覆盖全部并发/停止恢复/真实供应商场景；此前隔离证据保留其范围。当前结果为进行中，私有回执在ignored full-delivery-current目录及112部署证据目录，原定时任务继续暂停。


#### Issue59 · 首次真实返工与执行位置澄清

研发2于15:09:05Z真实handoff(target=tests) accepted，参数中无关闭目标不需要的params；原生15:09:17Z结束/停止，25件4509395bytes归档后15:09:44Z自动进入tests3。研发阶段驻留约23.70分钟（含自检、文档和归档，非净推理耗时）。全部25件逐size/SHA下载匹配；归档的研发完整自检日志1052318bytes/SHA256 `111b87a33790134f22fb7bc633a40c567aa389bf7f7e7635934e3e7260abc15a` 与state逐值匹配：make2/gate-1/runner1，12分钟预算超时，源码前后同 `82162f6b3dbd02ffc24858c119f8cca3542458bcc6748721e9e3e66ba5a6d54a`，candidate_removed及owned_group_absent真实记录；不能把局部278Node/race/build/部分浏览器绿写成完整通过。

tests3真实固定make verify于15:09:44Z–15:22:01Z完成，exit2，正式API完整导出1076353bytes/SHA256 `f1e472420845c02b583fde48e3f67147e94f9f4a25d958f3b37ea6ad4af6b537`、EOF=true/truncated=false，规则自动返回development4/conversation `cc8c8c3675ea6937871f531115a8928a`，没有手改状态或提前QA。运行中完整output API因尚无最终receipt.Log返回404，维护者只读tail作为在途诊断，结束后以正式完整API为准，不混同证据。

发现冻结仓AGENTS.md第15行仍将固定测试描述为host-only，研发据此误称将换宿主验证；实际Run是OpenSandbox、固定command=make verify/cwd默认同一/workspace。维护源全仓未找到这句旧项目规则，不把项目旧说明误当当前平台实现。Builder首次投递前置检查因节点已自然交接而拒绝，未写入；在返工Agent阶段通过正式User Input唯一request `issue59:builder:execution-boundary:1` 实际受理message13961、duplicate=false，供Pipeline修正项目规则/报告，保留真实失败且不据环境名称推断产品无缺陷。受理不等于原生已消费，后续核验消息/事件。Issue59具名评论6385850562已反馈具体影响，标记为Builder通知避免重复自动入站。此为一次平台事实澄清介入，不是代写修复/QA或改冻结图、权限、预算。当前整链仍未通过，QA/确认/报告/PR/释放后下载尚未完成。


### 2026-10-25 · Issue59补充输入接续、拒绝路径及修正版本固定门禁

UTC10-24 15:49:05研发4的wait_for_input被拒，原始工具只返回request conflicts with existing state；维护源waitWorkflowNode在有queued/steering/steered新输入时返回通用ErrConflict，提示未说明应结束当前回合先处理输入，记为平台提示P3待改，不冒称等待已受理。运行没有被错误改成waiting。15:50:05前回合结束后，15:50:06自动承接已排队message13961；两次真实turn属于同一原生thread `01a16e81-316e-7a91-b8f6-a3509b5c7adc`、同conversation cc8c…，两个输入均completed，无人工resume、新Run或冻结图改动。Builder只有一次已投递事实澄清；最初因阶段变化未投递的检查不计已送输入。

研发处理澄清后修正项目AGENTS.md及本轮说明，区分同一OpenSandbox中的原生与固定命令通道。16:00:11最终handoff accepted；此前15:57:15含根AGENTS.md的69件交接被产物路径校验拒绝，未推进。保持根规则文件原位，按已有源码快照/原路径与字节摘要映射另存文本，70件有效文件交接成功，原生16:00:26停止，归档后16:01:41自动进入tests5。此处是正确拒绝与修正路径，不是取消限制。70件11027959bytes新增归档，连同seq2共95件15537354bytes逐文件size/SHA核对；两批清单规范JSON摘要12b7e71e9e42919b9908e923aefdd92e2ea12134c06bd3bb59bf008b4438c8f5，后续释放后下载仍待本Run完成再核。

源码实质修复由Pipeline完成：历史输出流、已聚焦目标的键盘绕行、冷却HTTP测试时序及相应回归；不改网关实现。返工本地最终make verify仍exit2，原12分钟时限在console阶段终止，full-console未运行；源码4b4e4c07…的本地完整日志1055204bytes/SHA87de2528685e7897b722c4f400301e213a8f6f90e832ad46ecacfb85790a94ce为失败，不能以局部回归绿覆盖。当前tests5是这些实质改动后的首次正式固定评估；仍未进入独立QA/确认/报告/草稿PR，也未合并部署。

原生usage按回合保留原始计数：seq2末totalTokens6775605（cachedInput6606208）；seq4第一回合5282536（cachedInput5097472）、接续回合2253495（cachedInput2215680）。同一thread接续后计数回落，不能拿最后值当整阶段累计，也不把cachedInput重复相加或推算费用。首次研发交接受理至tests3约38.53秒，返工交接受理至tests5约89.58秒，包含原生结束和文件留存；不是纯调度耗时。当前重大耗时仍在研发自检、完整门禁和返工。私有原生/工具/归档证据保留于full-delivery-current目录；定时任务继续暂停。


#### Issue59 · 第二次正式固定测试仍超时，等待执行时限裁决

UTC10-24 16:01:41Z–16:13:58Z的tests5真实exit2，正式API全文1081662bytes、SHA256 `9e5c0beb54ce930986e8616a1f87d607f135566570ab94a0ce5b23cb74890c39`、EOF=true/truncated=false。迁移15.951秒、旧界面和两种视口路由完成，路由238.951秒并于16:13:43Z结束，console仅获约十余秒，full-console未运行。自动返回development6/conversation `5d2fe210cdfe50d4156a8a31c5210114`；研发16:16:18Z原生说明归档证据、暂无已证修复且不改预算，不再次同版全套求绿。该结果为No-Go/整链受阻，不能把命令节点completed当门禁通过。

维护者只读核对冻结main的tests/verify/main.go:179，12分钟由项目验证器硬编码；已挂载docs/workflow/requirements.md要求真实make verify及完整回执，没有12分钟整套性能AC。当前2CPU是容量线索，未证明唯一慢因；OpenSandbox当前SDK不提供原Run资源调整入口，未使用Docker热改、手改图/检查点、换环境或扩权。原Issue明确禁止改变原固定门禁，已向用户提出是否允许按实测制定显式有界时限的选择，未收到选择前不执行依赖动作；即使后续批准，也保留本次12分钟失败，不能改判历史或删弱断言。

Issue59具名Builder阻塞通知已发布并取得远端回执 https://github.com/big91987/model-relay/issues/59#issuecomment-6386571212；本轮尚无独立QA、确认、同版报告/新草稿PR或释放后下载结果。归档95件保持，当前待处理材料仍在同Run环境，不能当成已归档或擅自释放。定时任务保持暂停。


16:20:04Z研发6的wait_for_input(kind=blocked)真实accepted，16:20:14回合结束、16:20:15原生停止，Run/step6正式waiting、会话idle、无待审批；这次等待与此前有queued输入时的拒绝分别保留。平台机器人于16:20:17Z将原生阻塞回复写至Issue59评论6386569189，并附原会话入口；Builder说明评论6386571212已远端核验。没有新增resume/同版全套重跑。

经正式conversation文件API另行读取本节点6件2170347bytes原始材料，与Agent自带inventory逐size/SHA匹配（清单SHA42260dfb010050e9054da0170bdea82a0256565cb70e9016177f1fd80b4c6003），另存私有维护诊断目录。第二次固定runner日志1060657bytes/SHA40d85c22fd8e1f9f5a067488e2c13fc63dbc151dd23eff88eaa7451ebdb2a885与state完全匹配，source_before=after=4b4e4c07…，source/candidate_unchanged、owned_group_absent为真，candidate_removed；gate-1/runner1/make2保持。此为维护者只读保存，**不是**本节点handoff归档或释放后下载；等待阶段没有Result.artifacts，现有正式archive仅覆盖此前95件，未释放仍待接续的环境。当前结论整链Blocked/固定门禁未过，后续QA/确认/PR不可跳过；需先获得执行时限选择再继续依赖动作。


### 2026-10-25 · 用户授权分开审查用例/环境并调整串流程时限，Issue60接续

用户要求两方面并行处理：确认用例合理性或环境原因；为验证整链可以调大执行超时。已复核平台固定节点原本1800秒，真正先触发的是项目验证器内层12分钟；新任务明确授权Pipeline将项目全量执行时限调整为25分钟并记录生效值，外层30分钟及断言、完整阶段、权限、隔离和清理要求保持。25分钟是有界执行条件变更，不是已证明性能改善，不改判Issue59原失败；若仍红先定位，不继续逐次增限。本轮要求研发做针对性检查后交固定测试，避免在同一环境先自跑完整门禁再同版重复固定全套。

接续前正式文件API返回OpenSandbox environment lookup HTTP404，旧环境TTL14400秒且等待已超过此期，而Run快照仍为ready；这不是产品用例失败，记录为平台环境有效性/等待保存缺口，尚未修复。未偷偷重建旧环境、复制修改DB或恢复旧会话。Issue59原Run09ce…通过正式stop入口停止，API已确认stopped/seq6；旧完整日志与归档保持，等待材料仍只算维护者私有只读保存。停止诊断曾过量打印完整响应，观察封装未能确认结束；随后只读API核实停止结果，未重发停止请求。

已核现有Issue/PR和main12df9eb7a8522a679e3ccf98ca4e22b019774c5d，新建明确替代Issue60 https://github.com/big91987/model-relay/issues/60，经项目opened/labeled正式入口。Actions40145433959成功回执给出唯一Run91bc74a22a9f2c71662a6b9ee832b275，00:07:15Z创建；平台API核对issue_number60及原工作流revision10，prepare完成，development2/conversation d568a1dcbf4a0630b0ab3d554ae07ef2真实running、无待审批。初次管理员by-request404不能证明其他Owner任务不存在，已使用成功入口给出的RunID核实，未重复dispatch。Issue59正文已明确用户新授权和Issue60接续关系，旧Run不能写为completed。

Builder只读核对冻结Makefile、验证器与浏览器驱动：全量门禁有真实Go/HTTP/页面、迁移、路由权重、切换、SSE取消等实质行为；journey键盘驱动先Tab即使已有焦点，旧轮实质修正方向有据。上次进入浏览器前267秒、migration15.951、legacy185.691、routing238.951，console开始前已约707.6秒，原720秒只能留约12秒。2CPU/8GiB是容量事实，尚无因果对照；全量键盘操作和多次npm依赖准备是需Pipeline核对的开销，不据静态代码删除覆盖。本轮初步不是“所有用例不合理”或“全是环境问题”，最终用例与同版结果由独立QA审查；现在只有Agent已开始修正的进展，不是已实现/已测/已QA/PR完成。

平台二进制仍d9a9564、正式图revision10未热改，无Platform PR/main合并，无产品合并部署。当前四权威docs继续作为唯一维护进度源，原定时任务保持暂停。


Issue60 opened/labeled Actions40145433959/40145434298均success，两个GitHub日志逐值给出同一Run91bc74…，去重真实成立。原Issue59沙箱lookup404后，从正式归档API重新下载全部95件15537354bytes（新目录、不用原缓存），逐size/SHA一致，清单摘要仍12b7e71e9e42919b9908e923aefdd92e2ea12134c06bd3bb59bf008b4438c8f5。此证明确认归档不依赖已消失沙箱；不是本轮新Run的正式release或QA完成证据。


Issue60研发00:18:50Z handoff(target=tests)真实accepted、无params、19件实际产物；原生结束和归档后00:19:18Z正式tests3启动。研发阶段00:08:14Z–00:19:18Z约11.07分钟驻留，包含阅读、修正、针对性回归和材料，不是净推理耗时。19件393456bytes已逐size/SHA核对，平台清单摘要2e96c8a06245fd77977c5bec62b2978a62542abfbdc97794016badceb83ecc25；源码158件工作树摘要fab5f6d957c7a269a55b41689c43916ff1301382260242ca45240c0efc376ded，基于原main12df9eb。仅README、真实HTTP路由夹具、journey/UI回归、review-history及verify/回归共8个维护文件实质修改，无产品业务变化。

已读取原始针对性回执：runner/history race、门禁成功/取消/200ms期限的真实子进程及候选目录删除、13Node键盘回归、真实HTTP路由race/count3、格式/vet及依赖失败即停均exit0；这些不等于全量产品通过。Pipeline保留受控上游身份、31秒自然冷却、focus trap、正负断言及原依赖/历史门槛；没有提前重复全量自测。25分钟为项目入口明确固定预算，无新增运行时绕过开关，回执增加预算与gate起止/取消字段；平台外层1800秒不变。实际固定在途GATE_START已核timeout_ms=1500000、00:19:40Z，不只根据README或Agent自报认定生效。迁移浏览器15.921秒、旧界面178.582秒已结束，后续路由/console/full-console仍在执行；无独立QA或PR结论。

旧Issue59在stopped且95件归档核对后，正式release因原环境已不存在返回lookup404并记录release_unconfirmed；随后正式reconcile再次确认缺失，成功收敛released。没有重复Delete、重建环境、改历史失败或DB；不把这种消失后的核对写成正常主动释放成功。环境过期时快照仍ready、等待材料未自动归档/续期的根因缺口仍未修；当前测试继续使用新Run，不热改运行平台。


#### Issue60 · 25分钟条件下首次固定结果：非超时真实失败

00:19:18Z启动的tests3正式输出逐页读至EOF、truncated=false，1095386bytes/SHA2563380efc701bc204e26de2b38d304f0a89ebfe90dcf41d30675fa9ba4d83b0983；实际make exit2。主GATE_START00:19:40.480Z，GATE_END00:33:17.868Z为exit2/cancelled=false，区别于Issue59的gate-1/期限终止。migration15.921秒、legacy178.582秒、routing241.328秒、console36.347秒均ok=true；full-console75.878秒后ok=false，安全诊断明确stage=self-hosted-sse、width1280/error_kindother。secret_present仅诊断输入状态，不据此冒称秘密泄漏或产品根因；本轮未查看含秘密页面或代产品QA。

完整输出及源码前后摘要fab5f6…保留，自动失败规则返回development4/conversation8ce1db8f0af8abcb1bb28c1ad1a6b781，无手改、重复同版或继续增时限。25分钟确让后段用例执行到原12分钟之后，但不是全量通过或已修复性能；后续由Pipeline定位驱动/产品/环境归属，实质变化后再进入固定门禁。现阶段无QA/确认/报告/新PR。

旧Run正式release→reconcile收敛后再次从归档API全新下载95件15537354bytes，逐size/SHA仍完全一致，清单摘要12b7e71e…不变。此为旧Run释放后留存证据，本轮新Run的释放后校验仍待流程完成。


#### Issue60 · SSE采集原层红绿复验与第二次固定门禁

首红原回执已独立核对SSE HTTP200/dispatch_delta1，研发在隔离临时副本真实Go/HTTP/Chromium复现，补足原安全日志未保留的错误：response.text / Network.getResponseBody 无resource数据；不能把补充诊断当原正式回执自带详情。只修改journeys.mjs并新增stream-capture及其回归，共三个增量测试文件；读取原fetch响应clone，保持一次请求和同Request ID、真实UI/流内容/用量费用等断言，不从额外请求或CDP补造结果。5项负向/清理与成功回归、285Node、相关HTTP race及1280/1440/390每视口12步真实自托管旅程原始exit0已核；临时rework_test.go包装只存为go.txt证据，未留编译目录，隔离目录/浏览器已清理，正式根node_modules/npm cache仍absent。针对性绿不等于完整门禁或独立QA通过。

00:46:38Z真实handoff tests accepted、45件、无params；00:46:47回合结束、00:46:48原生停止，归档后00:47:36Z正式tests5启动。返工阶段约14.28分钟驻留，含定位/定向测试/材料，不是净推理。两批64件5786772bytes逐size/SHA核对，清单766b0d9da3f0c1d63628dd7fbe5e97cadd49935bcb2cd6520d61ad3f6971750e；本轮160源码摘要3864d081e13800056ea0f849c598373c1eac8c03443990453752cd70b4be7c4d，旧158/fab5版及失败文件保持。

已核首红runner1071019bytes/SHAf38e0e3bd4f3fc08f1575d4146e275e5f67c0842a4489deaebd2092d520d993d与state匹配、source_before=after、source/candidate_unchanged、owned_group_absent及candidate_removed均真。tests5在途GATE_START00:47:50Z/1500000ms已核，当前只是正式全量复验进行中，不能提前交QA。重新查询实际API health revision仍d9a9564，当前正式图和冻结Run均revision10，研发/QA/report均Codex gpt-6.1-sol，没有热改图、权限、再增时限、重放失败请求或Builder代产品修复。


#### Issue60 · 第二次正式固定全量exit0，进入独立QA

tests5实际make verify exit0，完整正式输出1103560bytes/SHAe01f395f0276bba35df538064d762ba7aea68c4edaf806a32b1fc484dfdf4f35，逐页EOF/truncated=false。从正式conversation文件API只读取得主attempt state及verify.log，独立核对runner日志1074290bytes/SHAda15351c27805125c035937e9ab438a17da5f764fb0e9a822245fefe92c31b40与state匹配，gate_exit=runner_exit=0、gate_cancelled=false；160源码3864d081…前后相同，source/candidate_unchanged、owned_group_absent及candidate_removed为真，node_modules/npm cache起点false。该证据为真实完成回执，不是仅看Agent自报；Go模块缓存仍不称全冷。

主gate00:47:50.951Z–01:04:11.032Z约980.081秒（16分20秒），runner00:47:36.911Z–01:04:12.046Z；平台节点还含派发与落盘。浏览器migration15.802、legacy178.558、routing240.367、console36.175、full-console243.808秒均ok=true。由此说明原720秒不足以容纳这套当前实测门禁；没有做跨CPU资源因果对照，不把修改执行预算写成产品性能优化。网关自身超时、限额、费用等业务断言未放宽。

运行自动进入qa6/conversation f15cee7b1902da8d4f5a3e9b04001948，原生真实承接并开始核源码、回执和独立针对性复验，审批0。全量绿色是技术门禁结果；QA、人工确认、同版报告、真实draft PR和新Run释放后下载仍未完成。

另由Builder只读GitHub API核对旧draft PR55的head32213cf63995f6e3b06488bd0ec48c4ed7bc1b24（2026-10-22创建后未更新、open/draft/未合）：已有25分钟verificationGateContext和同fetch响应clone的SSE采集改动，尚未进main。本轮未在起跑前把具体材料准备给Agent，导致重复定位，属于Builder输入准备效率遗漏，不能归为新产品故障或全是环境原因。没有把旧草稿测试结果替代本轮，未合旧PR、未指示Agent抄整包；当前160源码独立实现/重新实测事实保持。计划在正式Owner确认summary向report提供来源事实，不另外投递迟到UserInput打断QA，不改历史“当时未取得”记录或当前Go结论。


#### Issue60 · 独立QA通过并正式进入交付报告

QA6于01:11:17.561Z实际handoff(target=approval) accepted，result.route=approved、23件产物，原生结束/停止及归档后进入approval7。QA独立读完整固定输出、160源码摘要、11个修改代码和22件开发材料；实际执行runner/history race、真实HTTP路由race、126项Node及diff检查。首次组合命令因login PATH缺Go导致两条Go命令未执行，虽然末尾Node exit0，QA保留首日志并明确不将其当Go通过；使用已安装Go正确PATH和fail-fast后针对性重跑成功，未重跑整套make verify。QA复核固定浏览器原始JSON/PNG并抽查窄屏同请求SSE详情，没有声称独立重复全套浏览器实操。原失败、真实供应商/生产性能未验和旧等待TTL问题保留。

Builder通过正式归档API核对3阶段87件30818063bytes逐size/SHA匹配，清单SHA6f6b1c147891de9b4de4b66f2ce2db734428ca85787f9dbb6baa261801975c6d；并独立重组3片20403005bytes的原浏览器包，342个原始成员37700340bytes逐size/SHA与QA清单一致，含路径/类型/重复检查。包SHA68218942c1997db4f118daf65423c86d2e28f9b47d0e926d1027f62927c9c81c。此为留存完整性核验，不替代产品QA。

正式decision(seq7,route=next)已accepted，summary SHA df90670fb427a2c5fd8e36c5a15d5d69ac273841b1537b457cb58d34164d02e8，随后API确认approval7 completed、report8/conversation a367d60e36c94774928dacb4dd27e6c1 running。确认明确为Builder按用户既有授权代Owner，不是用户本人点击；只进入报告/草稿PR，不批准合并部署。旧PR55来源核对通过标准decision summary传给report，Agent原生进展确认将保留来源边界；没有另发迟到UserInput、修改冻结图或代产品实现。报告、实际PR及新Run释放后全新下载尚待完成。


#### Issue60 · 固定发布拒绝与通用模板修复

report8于01:21:34.979Z正式handoff(target=publish) accepted、24件，01:21:42原生结束/停止。全部111件55858492bytes归档size/SHA核验通过，清单4866d467d340372037060674de8eb805ca5a9759b0467959dfa8e8ea1aff68b5。固定publish9未通过字段校验，exit1/843bytes/truncated=false，错误report must describe the actual change and evidence；报告标题用了不受支持的【测试设施】，且正文因未拿到当前Issue链接而未关联#60。错误位于Git写入/提交/推送/PR前，GitHub正式读取无codex/run-91bc74a22a9f匹配PR。不是产品/测试/QA失败，也不是权限或超时，不盲重跑发布。

根因是通用报告模板只写【类别】未列允许值，入口消息仅title/body、当前Issue虽然保存为参数却未写进Agent可见任务输入。维护源ea38d07d8c296ea4d4af16799ea9e12b7ceea67d补入受信当前Issue链接、六类标准及缺关联先澄清的要求。入口可见链接回归先红，修后133模板测试21.731秒exit0，ruff检查/格式通过；首次运行遗漏SDK PYTHONPATH仅是测试启动错误，正确环境后取得真实六类缺链接的红。标准导出151对象、4个模板差异，不改平台二进制或产品候选。

112正式API将主图revision10升11，只改report指令；保存重读/完整预览一致、当前冻结Run逐值不变。第一次升级前置断言误用了旧记录中的角色摘要而拒绝，在任何写入前停止；只读核实当前角色实际逐字等于旧维护源（245b91ec…）后才正式应用，未覆盖自定义。当前Run仍冻结revision10，通过用户既有授权正式stop9，核stopped后return(report)创建10；旧publish9保留失败回执、取消/人工回退语义，前8步及测试/QA/确认不变。回退反馈给出Issue60真实关联和同源报告约束，仅准修材料后再交固定publisher；不热改图、手改文件/回执/DB或重跑已绿全测。此为显式人工恢复，不能称无人干预或首次发布通过。


维护源ea38d07已开发分支推送；标准Harness PR61 https://github.com/big91987/model-relay/pull/61 的head586e6c76cd67a4eead0287d4a7b9135e984976af，4文件/151对象逐Git blob一致、GitGuardian SUCCESS，按既有Harness构建授权精确合为main a1230fd470faf6ed0e6a16e7a4bf03e21b58d359。测试仓新入口以后自动携带当前Issue链接，当前冻结Run不追溯修改，产品/测试候选仍基于12df9eb。没有Platform PR或main合并、没有产品合并部署。Issue60评论6391948472已以Harness Builder身份反馈发布卡点及处理；原平台机器人失败/重启回调真实存在且身份独立。


#### Issue60 · 真实草稿PR、正式释放及释放后留存闭合

report10于01:29:45.389Z handoff publish真实accepted、28件，01:29:52原生结束/停止；固定publish11实际exit0，完整1413bytes/SHA08f30cf37e406165d2d086236ce85c2b98c922f64ee53fcf6f5860eb21a67a9c，EOF/truncated=false。真实PR62 https://github.com/big91987/model-relay/pull/62，head0525d83245e38207cd417917e0a13917c9403581、open/draft、未合；运行01:30:33.046Z进入done12/completed。原publish9仍cancelled且真实exit1/843bytes/SHA13641f4829a4e71c3682c900d323324bfe3b76915e0f5d5b0d098b6ec45eee4f，旧报告8原字节另存；人工回退不改判第一次发布。

Builder独立从GitHub exact head逐Git blob读取并重算160件source，3864d081e13800056ea0f849c598373c1eac8c03443990453752cd70b4be7c4d同tests5/QA；相对冻结main12df9eb只有11项QA确认变更加docs/harness/run-91bc74a22a9f/qa.md与report.md共13件。11项bytes/SHA全部匹配development4/QA清单，PR正文逐字同报告10body（SHA3c33b545d6f588ffc1e1d2ceba7f33fbd46ed383f2d34500d4626e138591c336），QA原文64298f8fb73dd5a2270ce218aab76f00c2e87d7b7c8eb691d6d1da3c3f440f55；GitGuardian SUCCESS。PR当前base因独立模板升级为a1230fd4，候选仍冻结12df9eb，不把模板main合并当产品草稿合并；两PR均已附当前聊天。

139件80922540bytes均正式retained、清单SHA986925532264f5e0aef15af1feaa48ba8084a92751084ea29797ff44a9dee51e。核对后仅一次正式environment/release，当前completed/seq12保持、环境released、原生进程全stopped；再从正式artifact文件API全新下载所有139件（不读缓存），逐size/SHA完全匹配。342浏览器成员原始完整性核验已记上文，不把释放后普通文件核验重复描述成另一次视觉QA。此闭合当前真实整链和持久留存，不证明旧TTL等待缺口已修或新revision11已无人介入全程跑过。

阶段驻留（分钟，含阅读/结束/归档/等待及人工诊断）：prepare0.98、development2 11.07、tests3失败14.02、development4返工14.28、tests5成功16.61、QA6 7.67、approval7 6.62、report8 3.65、publish9失败至回退3.99、report10修订4.24、publish11 0.15；总83.29分钟。不能称Agent净耗时或与Issue48不同任务直接换算提效。主要可证开销是两次全量测试、测试驱动重新定位、报告合约返工与Owner核对；2CPU/8GiB无资源因果对照。旧草稿材料未前置造成重复定位已明确为Builder准备不足。原生token累计计数已私有保留（含缓存，不能把cached输入再重复相加），无可得费用，不估造金额。

当前源码回归与模板升级、实际Pipeline验证、独立QA、人工恢复、草稿PR、平台部署版本分别报告。完整链有显式人工回退与Builder输入补充，不宣称全部可靠性或自主交付通过；旧失败/供应商NotRun保持。四权威docs同步当前事实，停止本阶段继续加任务，定时任务仍暂停。


### 2026-10-25 · 真实功能研发/QA留存评测 Issue63启动

用户新授权从repo Issue开始直至功能PR，评估实际总时长和质量，特别要求QA记录留存。已核所有打开Issue/PR和正式Run，既有同目标Issue51只有2026-10-15设计等待，无产品修改/测试/PR；当前file API HTTP400、OpenSandbox lookup404，不能沿原现场恢复。未另指定功能时沿用此前低延迟优先路由方向，形成明确替代关联的新评测Issue63/id6023793493，不重复恢复旧请求键。自动审批拒绝了旧Run stop（理由：未明确授权停止该具体Run、可能损失接续环境）；该命令整个未执行，安全替代为旧Run原样保留，新环境继续，不绕拒绝。

已验测试设施PR62精确head0525d83245e38207cd417917e0a13917c9403581重读open/draft/mergeable clean、GitGuardian SUCCESS，按用户既有测试仓Harness构建合并授权ready并精确合main f06f05de20ffac5b8c8e772958c808379001f0e3。它仅是新功能的正常测试设施基线；不将过去绿色作为本轮验收、不合旧PR49/55或产品候选、不发Owner部署。新Issue从11:14:20Z开始计时；基线准备另列，不能从用户总体等待中隐去。

Issue63正文先写用户能力，再写先计划/冻结实验后实现、真实Go/HTTP/SSE和桌面/390px连续UI、权限/限额/非零RPM/TPM/预算/取消/迟到状态边界、独立QA实际执行/原始回执/可视化对照和同版产物。产品实现与验收交Pipeline；Builder核完整性和正式证据，不代写实现或代QA过关。原make verify25分钟/外层30分钟保持；若覆盖预计超限先说明，不删覆盖或静默增限。QA记录类型、归档路径与PR可访问报告有明确要求，原始大文件遵守标准publisher/归档限制。

自动入口opened40188019710/label40188020080已真实queued/pending，旧通用入口40188019708 skipped，正式API目前未有新Run；共享he-full/model-relay-local Runner正执行基线push prepare40187805174。此为入口/供应链队列候选效率瓶颈，不是Agent开始研发，后续自然受理后核唯一Run和冻结版本。已准备以Harness Builder具名/标准平台输出标记在Issue回写，不把评论当新用户输入。


Issue63原入口Actions40188019710成功：19:17:16（北京时间）获得Runner，19:17:27.847（UTC11:17:27.847Z）创建唯一Run ba434da8039cca16f6829481112e3d68，正式GitHub日志和管理员Run列表按issue_number63一致。原默认入口skipped，未重复dispatch。prepare实际94.686秒后development2/03d91bdea8a1b9570ced063ac3db5375在11:19:02Z启动，原生实际读取材料/源码，pending approvals0。正文当前Issue链接由标准入口提供，冻结source f06f05de/revision11及14,400秒环境TTL已核；旧Run不改。已从Git对象核对基线11个已验设施文件与PR62同字节，151个模板对象与维护源ea38d07一致。队列/prepare独立记录，计划完成后再正式设计检查，不提前判功能或QA通过。Issue队列说明评论6396949643使用标准平台输出标记以免误作新UserInput。


Issue63研发首轮11:26:21.257Z wait_for_input accepted、11:26:29.945Z回合结束/11:26:30.171Z原生停止，正式waiting/idle。Pipeline真实生成frozen-input.md9050bytes/48822df0…、plan.md12104bytes/72d3a2a3a73a440f7e5b013c1a1a918000d6e59a407d8f5e11e472f72f403b13、experiment.md5714bytes/dfaa3d34c9887d4581aa9678a3d27530dd3acf4e4ea53b2e6d023052348a562f、execution.md3493bytes/792a22d1…及manifest1026bytes/79fe8572…；Builder从正式conversation文件API只读保存5件31387bytes并逐manifest核对。该保存为维护检查副本，不冒称等待阶段已经完成handoff/平台归档。原生git status仅新增本Run文档，明确产品未改、实验未跑。

第一轮设计完整性检查要求修订：SSE以首传输字节评分可能偏爱早心跳而非早内容；2240顺序请求加150ms SSE尾部及新旅程/回归使既有980.081秒门禁接近1500秒、余量不足；长流测量时刻与完成采纳/TTL起点需要一致。Builder不代写算法或调整产品阈值，要求Pipeline保留v1、给有据的新版本及反例/预算后再正式检查，不改源码/权限/25或30分钟、不观察后降门槛。这是设计审查返工，不是独立QA结论或已证产品Bug。

通过正式workflow messages的request_id issue63:builder:design-review:1一次保存User Input16636/duplicate=false，仍同conversation03d91b…，11:30:48原生progress已承接核v1与门禁阶段，未用resume或改库/冻结图。opened40188019710和labeled40188020080日志逐值同一Run，二者success；本轮入口去重真实成立。后续计划修订、产品实现、完整测试、独立QA、正式确认、报告/草稿PR及留存均待实际发生。
