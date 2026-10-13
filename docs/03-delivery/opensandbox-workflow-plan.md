# OpenSandbox 工作流改造实施计划

日期：2026-10-13。状态：实施中；用户已授权在当前隔离 worktree 实现、部署到 112，并通过测试仓新 Issue 类型启动独立 Pipeline。当前分支 `codex/opensandbox-workflow`，起点 `65b4d3e`。

**Goal:** 在 112 提供隔离部署的 Agent Platform，画布以 Agent、执行、人工确认、结束四类节点构建流程，使用 OpenSandbox 执行任务，并以测试仓独立入口验证真实交付。

**Architecture:** 平台保存流程、权限、Run 和回执；OpenSandbox 管理 Run 执行环境。执行节点支持代码、命令和官方 SDK 的 MCP 工具调用；Agent 与工程步骤使用同一任务环境，角色会话独立。已有本机 Run 和旧 Connector 定义保持兼容，不把运行历史原地转换。

**Tech Stack:** Go 平台；原生 Codex；OpenSandbox Docker 后端；Python/TypeScript 任务运行时；MCP 官方 SDK；现有 GitHub 入站与报告发布工具。

**Spec:** `docs/01-product/software-factory-assessment.md` 第 0 节，加本节记录的用户后续确定项：四类节点；Agent 内部 Hook 保留原生职责，外部动作复用执行能力；外部工具统一 MCP；项目脚本不限制 Go；Example 复制入项目后按需调整；112 实际部署及独立新 Issue 入口是完成条件。

## 当前实施状态（2026-10-13）

| 部分 | 当前证据 | 仍缺什么 |
|---|---|---|
| 执行节点 | MCP、Python代码、项目命令同Run编排/回执/下游完整文件输入，隔离HTTP及实际页面回归通过；TypeScript保存重载通过 | TypeScript真实运行、原生Agent、112实际Pipeline |
| 112 OpenSandbox | 1.1.1/8095；实际SDK Python/文件0600/取消/删除404通过；Run归属持久化与恢复隔离回归通过 | 原生Agent、正式平台Run、服务登出重启 |
| 四类画布与112平台8795 | 代码/命令/MCP及环境配置保存重载、运行环境核对/释放和390px页面回归通过；8795未部署 | 原生远程会话、正式部署核验 |
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
- [ ] 引入执行节点配置：code/command/mcp，保存及启动校验；旧 connector 仅兼容。代码支持 Python/TS，命令允许项目自身语言/构建入口，不固定 Go。
- [ ] 接入现有持久调度、取消、结果与失败线路，保存冻结的连接/工具契约；写入未知结果不重放。
- [x] MCP保存/重载/停用取消/重启回归通过；原始调用回执可见、连接快照不进入公开Run JSON。

## Task 2：OpenSandbox 与原生执行

Files: 新增 `internal/platform/sandbox*.go`、部署目录 `deploy/opensandbox/`；接入 `codex.go`、`codex_appserver.go`、命令执行与文件访问。

Interfaces: Run 关联实际 sandbox ID；受信环境配置声明服务与镜像。采用官方 SDK/API，具体接口以所固定发行版核验。原生 app-server 双向消息、工具审批与停止沿现有调度接口接入，不将 Codex 换成临时单次 shell Agent。

- [ ] 在 112 核实 Docker、可用端口、网络及认证条件；固定 OpenSandbox 发行版本和镜像。
- [ ] 有界真实探针证明创建、命令、文件、原生双向通信及销毁；探针不冒充 Pipeline 验收。
- [ ] Run 创建/核对/释放持久化，输入与文件仅进入所属环境；实现保留与明确释放，不承诺内存恢复。
- [ ] 同环境 Agent + Python + TS + 项目命令 + 浏览器；停止、服务失败、平台重启恢复回归，补充不同 Run 隔离验证。

## Task 3：四类节点配置界面

Files: `web/workflow-model.js`、`web/workflow-panel.js`、`web/workflows.js`、`web/workflow-runs.js`、现有 MCP 管理页及对应测试。

- [ ] 新建流程只呈现 Agent/执行/人工确认/结束；执行按 code/command/mcp 显示必要字段。
- [ ] MCP 工具发现和 Schema 输入、上游参数/结果引用；短代码直接编辑，复杂逻辑引用项目脚本。
- [ ] Agent 配置基础/高级分组，角色与任务分离；人工确认与权限审批区分；固定测试不能被人审跳过开关跳过。
- [ ] 浏览器实际保存重载、执行结果、失败/停止恢复及窄屏可用验证。

## Task 4：标准部署和仓库自有 Harness

Files: `deploy/opensandbox/`、`examples/platform-workflows/` 入站/安装工具及 SDK；测试仓复制后的独立 `.github/workflows/` 与项目 Harness 目录。

- [ ] 112 新实例通过标准命令部署，独立数据/端口，健康版本与源码一致；最小凭据配置不公开。
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
