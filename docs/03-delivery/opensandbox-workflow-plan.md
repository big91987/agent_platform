# OpenSandbox 工作流改造实施计划

日期：2026-10-13。状态：实施中；用户已授权在当前隔离 worktree 实现、部署到 112，并通过测试仓新 Issue 类型启动独立 Pipeline。当前分支 `codex/opensandbox-workflow`，起点 `65b4d3e`。

**Goal:** 在 112 提供隔离部署的 Agent Platform，画布以 Agent、执行、人工确认、结束四类节点构建流程，使用 OpenSandbox 执行任务，并以测试仓独立入口验证真实交付。

**Architecture:** 平台保存流程、权限、Run 和回执；OpenSandbox 管理 Run 执行环境。执行节点支持代码、命令和官方 SDK 的 MCP 工具调用；Agent 与工程步骤使用同一任务环境，角色会话独立。已有本机 Run 和旧 Connector 定义保持兼容，不把运行历史原地转换。

**Tech Stack:** Go 平台；原生 Codex；OpenSandbox Docker 后端；Python/TypeScript 任务运行时；MCP 官方 SDK；现有 GitHub 入站与报告发布工具。

**Spec:** `docs/01-product/software-factory-assessment.md` 第 0 节，加本节记录的用户后续确定项：四类节点；Agent 内部 Hook 保留原生职责，外部动作复用执行能力；外部工具统一 MCP；项目脚本不限制 Go；Example 复制入项目后按需调整；112 实际部署及独立新 Issue 入口是完成条件。

## 当前实施状态（2026-10-13）

| 部分 | 当前证据 | 仍缺什么 |
|---|---|---|
| 执行节点 | 112正式API同Run Python/TypeScript/Go项目命令、完整输入/共享文件、去重、人工确认及环境释放已过；MCP隔离HTTP/实际页面回归通过 | 原生Agent及测试仓真实Pipeline |
| 112 OpenSandbox | 1.1.1/8095；SDK文件/取消及认证原生initialize/停止/删除404通过；8795正式基础Run五步完成并释放，实际Chromium页面通过 | 原生命令namespace权限待答复、模型角色会话/恢复、服务登出重启 |
| 四类画布与112平台8795 | 8912141标准Linux/CGO构建升级exit0，Mac/112 health同SHA、原完成Run保留；真实1440画布和390Run截图核验通过 | 原生远程角色会话及模板真实Pipeline；用户服务logout/reboot |
| 测试仓独立入口 | 尚未创建新Issue/Pipeline | 仓库自有Harness、新类型去重、真实整链 |

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
- [x] Run 创建/核对/释放持久化，执行节点输入与文件仅进入所属环境；实现保留与明确释放，不承诺内存恢复。（Agent产物API待接入）
- [ ] 同环境 Agent + Python + TS + 项目命令 + 浏览器；停止、服务失败、平台重启恢复回归，补充不同 Run 隔离验证。

## Task 3：四类节点配置界面

Files: `web/workflow-model.js`、`web/workflow-panel.js`、`web/workflows.js`、`web/workflow-runs.js`、现有 MCP 管理页及对应测试。

- [ ] 新建流程只呈现 Agent/执行/人工确认/结束；执行按 code/command/mcp 显示必要字段。
- [ ] MCP 工具发现和 Schema 输入、上游参数/结果引用；短代码直接编辑，复杂逻辑引用项目脚本。
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
