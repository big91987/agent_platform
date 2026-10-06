# 平台内研发交付示例

本例把研发流程保存在 Agent Platform 的图中，由持久 Run 推进。GitHub Issue 与平台网页都是任务入口；CI 只转发事件，平台推进研发节点。当前真实验收状态见验证记录，不能以代码或单元测试代替接单实测。平台本身不包含“需求／设计／研发”等业务阶段；这些是本目录提供的可编辑模板。

## 组成与边界

- `software-delivery.json`：准备分支 → Issue → 任务判断 → 必要的需求／设计 → 研发 → 项目测试 → 独立 QA → 交付说明 → 提交推送 → 草稿 PR。测试失败回研发；QA 可回需求、设计或研发；关键问题留在当前 Agent 会话澄清。所有回路保留在同一个 Run 中。
- `qa-rework.json`：从已有产物的独立 QA 开始，发现问题返回需求／设计／研发，修复后重新测试和 QA；不创建 Issue、分支或 PR，最后由验收人员选择结束或继续返工。
- `prompts/`：阶段指令，复用 Harness Skill，按项目规模裁剪。普通选择记录推荐并继续；用户明确要求、关键歧义和高风险操作才等待确认。阶段不适用时说明依据再交接。
- `install.py`：使用公开管理 API 注册 Agent、stdio MCP、Connector 和图；安装清单用于重入与升级，不操作数据库。
- `repository.py`：固定的准备分支／提交推送命令。每个 Run 使用 `workflow/<run_id>` 分支；拒绝接管脏工作区、错误仓库、错误分支和明显凭据文件；不 reset、不强推、不合并。
- `browser_tool.py`：复用 `../github/tooling/full_harness/browser` 的锁定浏览器运行时；只借用浏览器能力，不使用其 CI 控制器。检查 app 或设计原型，保存真实截图／检查结果；支持页面内受限检查脚本。

默认验证命令为 `npm test`，可用 `--test-command-json '["make", "verify"]'` 为 Go 等项目配置自己的固定验证入口。命令按 argv 执行，不隐式使用 shell，阶段指令使用同一配置。浏览器工具本地服务根为 `app/` 或 `docs/workflow/prototype/`。其他技术栈应修改图中管理员配置的测试命令与注册工具，不改平台引擎。浏览器可验证原生缩放和可访问树，不能把这些称为真实读屏器语音验收。Agent 需对未测项明确说明。

## 安装

需要运行中的平台（含 Workflow/Connector API）、Python 3.11+、Node/npm、Git、可用的原生 Codex，以及独立项目 checkout。平台账户必须有管理权限。GitHub Token 放在平台服务环境中，至少能访问指定仓库的 Issue、内容与 PR；本例不需要 Actions 管理权限。

Skill 是独立资产：指定包含 `defining-platform-products-cn`、`platform-architecture-cn`、`managing-engineering-delivery-cn` 的目录，并单独指定 `validating-platform-releases-cn`。所有目录必须在平台主机可读。安装器继承基础 Agent 的 executor/model，按本例重新配置阶段指令与权限；不继承其任意 native_config、Hooks 或整份环境。

在平台主机从源码 checkout 运行（将占位符替换为实际路径；安装清单与证据不要提交）：

```sh
# 平台服务启动环境中设置 WORKFLOW_GITHUB_TOKEN；不要把 Token 写进 JSON 或命令参数。
# 用组织支持的凭据机制为 Git HTTPS/SSH 配置认证和提交身份。
# 命令 Connector 将服务端 WORKFLOW_GITHUB_TOKEN 映射为 GH_TOKEN，供 gh credential helper 使用。
# 在当前终端安全设置 PLATFORM_ADMIN_PASSWORD，然后执行：
python3 examples/platform-workflows/install.py \
  --platform-url <platform-url> \
  --base-agent <existing-agent-id> \
  --repository <owner>/<repository> \
  --workspace-root <dedicated-workspace-root> \
  --skill-root <harness-skill-directory> \
  --qa-skill <qa-skill-directory> \
  --evidence <private-evidence-directory-outside-workspaces> \
  --manifest <private-installation-manifest.json> \
  --prepare-browser
```

`--prepare-browser` 通过已有 package-lock 执行 npm ci、安装 Chromium 并检查真实启动。若使用代理，需在安装器与平台服务环境中配置相同的 HTTP(S)_PROXY/NO_PROXY。仅这些显式代理值复制到阶段 Agent；GitHub Token 不传给 Agent。安装清单存配置和 ID，不存登录 Cookie/密码；不要在代理 URL 内嵌凭据。

`--base <branch>` 同时指定准备分支与草稿 PR 的目标基线，默认 main；适用于非 main 主线或隔离验收分支。升级时保持原基线参数，避免意外改回默认值。

安装不会启动 Agent 或创建 Issue。打开返回的流程链接，点击“运行”，输入普通需求和根目录内的**独立、干净 checkout**。每个独立 Run 必须使用不同 checkout，避免交叉修改。引擎拒绝占用未结束 Run 的同目录、父子目录或符号链接别名；停止／失败仍保留占用，因为之后可以恢复。完成后才释放。浏览器工具证据目录在工作区外，便于保留诊断；交付截图与报告仍写到项目的验证目录。

## 日常操作

- 图中选择节点、连接输出路线、保存即产生新版本；已启动 Run 使用旧图快照。
- 运行详情能打开当前 Agent 会话。普通推荐自动推进；需要补充时在该会话回答。
- 人工调整路线：停止 → 等待停止完成 → 选择目标节点并写理由 → 返回执行。停止不会撤销已发生的文件修改或 GitHub 动作。
- 运行中的工具回执和历史可刷新查看。结束只表示这张图走到结束节点；本例把结束放在独立 QA 和草稿 PR 之后，不等于已人工批准合并。
- PR 含 `Closes #<issue>`，由人决定合并；本例不部署、不自动合并。

## 升级与恢复

升级维护源后，用同一清单及相同目标参数重跑，并加 `--upgrade`。安装器逐对象记录成功，重复安装无变更时不创建新对象／版本；API 返回的空默认字段不应触发伪升级。平台外编辑过的已安装对象会被拒绝覆盖，应先对照清单和当前配置合并改动，再选择维护方式。不可丢弃清单后盲目安装，否则可能无法确认已有对象归属；同名冲突会明确报错。

图和 Connector 配置对 Run 冻结；Agent 会话创建时冻结 Agent 配置，尚未创建的阶段读取当时的 Agent 定义。升级阶段指令前应完成／停止受影响的在途任务，或用另一个 prefix 安装新版本。**脚本和 Skill 的磁盘内容不受数据库快照冻结**：生产安装应使用不可变版本目录，并在新安装配置中引用新路径，保留旧目录直至相关 Run 结束。

命令执行被中断时，其副作用可能已经发生，平台不会自动重放；检查仓库和回执后通过正式停止／返回入口重新执行。准备分支可以识别同一 Run 已创建的分支；提交推送仅在正确分支上执行，重复无改动发布不会造空提交。GitHub 接口响应丢失时按 Run/节点标记核对已创建资源，不能凭空补成功状态。

安装中途失败：保留清单，修复明确错误后重跑。若 HTTP 写入返回不确定结果但清单未落盘，应先在管理页确认该对象并恢复清单归属，不能连点安装造成重复。安装不是跨多个 API 的原子事务；已成功的对象会保留。

## 验证入口

```sh
PYTHONPATH=sdk/python python3 -m unittest discover -s examples/platform-workflows -p '*_test.py' -v
ruff check examples/platform-workflows
ruff format --check examples/platform-workflows
```

本地 Git 测试覆盖脏工作区、分支隔离、真实提交推送与重试；它们不代替真实平台旅程。平台完整证据在 [验证记录](../../docs/03-delivery/workflows-verification.md)。发布前需要从网页实际走通一句话需求、工具执行、QA 回退、修复、GitHub 草稿 PR，以及停止／重启后的恢复。

## 已有产物的返工验收

用上述安装参数和原安装清单追加 `--template qa-rework`，注册独立的返工图，保留原交付图。此模板复用原阶段 Agent、浏览器工具及测试 Connector；启动前选择已有需求、设计和实现的独立 checkout。不要选用另一个未结束 Run 占用的工作区。需升级配置时仍使用 `--upgrade`，不绕过漂移检查。

从返回的图页面启动，输入验收范围。QA 根据实际证据选择回退目标；研发修复后通过项目测试返回独立 QA。末端人工节点用于检查返工证据，测试执行者可自行操作；它不表示正常交付模板需要额外的形式审批。故障注入只用于专用测试分支，保留原交付分支和失败证据。

## 隔离实例的真实 API 验收

```sh
# 只对隔离验收实例运行，PLATFORM_ADMIN_PASSWORD 通过环境安全提供。
python3 examples/platform-workflows/verify_access.py \
  --platform-url <isolated-platform-url> \
  --evidence <private-evidence-file.json>
```

该入口通过真实 HTTP 创建两个临时调用账户及一张人工节点测试图，验证跨用户访问拒绝、重复提交、工作区占用、版本冻结、审批与停用；结束后停用测试账户和图。证据记录结果和资源 ID，不记录密码。异常时保留工作区和 Run 供排查，不能盲目删掉在途任务。它证明 API 行为，不代替网页旅程。


命令回执最多保留脱敏后开头与末尾各约 12 KB，中间省略会明确标注；短输出完整保留。凭据在截断前处理，跨输出分块的凭据不会因截断露出片段，UTF-8 边界不显示半个字符。退出码独立保留：开头可核对版本，末尾可诊断最终失败；不能把中间省略的用例结果推测成已通过。这是平台二进制能力，升级后对新的命令执行生效，历史回执不会补写。 工作流JSON通过标准输入提供上下文，固定验证命令可以不读取；平台并行交付输入与收集命令结果，不会因历史增长阻塞取回退出码。不读取或提前关闭输入本身不覆盖真实命令结果；执行被中断仍须检查副作用后显式恢复。

命令取消与强杀恢复另有独立入口：

```sh
# 在已启动的隔离实例上验证命令停止、拒绝隐式重放和显式重试。
python3 examples/platform-workflows/verify_command.py \
  --platform-url <isolated-platform-url> --evidence <private-evidence-file.json>
# 或由脚本创建临时实例，强杀该实例后重启验证；不重启任何既有平台。
python3 examples/platform-workflows/verify_command.py \
  --server-binary <built-agent-platform-binary> --evidence <private-restart-evidence.json>
```

两个模式都要求 `PLATFORM_ADMIN_PASSWORD`。脚本需在平台所在主机运行，因为固定命令引用同机脚本及临时工作区。夹具运行真实 Python 子进程并写入操作记录；停止后核实进程已消失、记录未重复，显式返回产生新执行，并验证非零退出按失败边路由。强杀模式只终止脚本自己创建的服务。临时数据、运行历史和操作记录留作证据，位置写入报告；图及 Connector 停用后可人工清理。此检查不调用 Agent，也不冒充 GitHub 网络丢响应验收。

GitHub 响应丢失验收为显式启用的 Go 集成测试。设置 `WORKFLOW_LIVE_TEST_REPOSITORY=<owner>/<isolated-test-repository>`、`WORKFLOW_LIVE_TEST_TOKEN`（通过安全环境提供）以及 `WORKFLOW_LIVE_TEST_EVIDENCE=<private-absolute-json-path>` 后执行：

```sh
go test -race ./internal/platform -run '^TestWorkflowGitHubLiveLostResponse$' -count=1 -v -timeout 180s
```

该测试启动独立平台 HTTP 服务及真实数据库，通过登录、创建 Connector／图／Run 和 resume API 验证。GitHub 请求使用真实服务和固定官方域名；专用客户端在实际 POST 成功后丢弃响应，模拟“外部已经创建、平台没有回执”。随后核验平台只查找原标记，不重复 POST，实际资源恰好一份。测试结束关闭自己创建的 Issue，保留链接供追溯；不合并代码。故障注入只存在于测试文件，不改变发布二进制。它验证真实外部写入与恢复协议，不代表物理断网或代理故障覆盖；没有设置仓库环境时明确跳过，不能把默认回归的 skip 当作通过。


## 智能体编排：路由与事件通知

- `edges[].mode=handoff` 是 Agent 根据当前上下文选择的交接，画布用虚线；`description` 给出选择依据。Agent 调用 `handoff(target, summary, inputs, artifacts)`，平台校验目标、权限及当前节点。
- `mode=automatic` 是固定完成路由，用实线。Agent 完成时调用 `complete_node(summary, artifacts)`，不填目标；Connector 按真实成功/失败结果选配置路由。Agent 可有多个自主交接出口，但只有一个固定完成出口。
- 普通澄清留在当前会话，不交接、不为每次提问增加图节点；必须审批的流程使用显式人工确认节点。自动推荐写在 Agent 策略中，不等于取消固定审批。
- 老模板缺少 `mode` 时继续兼容；运行冻结定义快照。模板更新只影响新运行，不能假设在途任务自动切换定义。
- `hooks` 将 `node.started`（Agent 会话已创建）、`agent.reply.completed`、`handoff.after`、`run.completed` 映射到已注册 Connector。首版支持 GitHub Issue 评论动作，GitLab 适配和阻塞型 before Hook 尚未实现。
- 通知正文使用字段白名单：`event`、`run_id`、`node`、`node_name`、`seq`、`text`、`summary`、`target`、`artifacts`、`conversation_url`、`run_url`。不执行表达式，不从自由文本推断“已批准”。Issue 由 `input.issue_node` 引用创建节点的真实回执，或用 `issue_number` 显式指定。
- 通知持久化去重，失败与执行状态分开显示。发送结果未知时只读查证原评论，不盲目重复发布。画布“通知 Hook”可以编辑，运行页可以查看链接、错误及重试/复核。

用原安装命令加 `--upgrade` 同步模板、Agent 策略和评论 Connector，继续使用原 manifest。安装器会拒绝覆盖安装后在界面另行编辑过的对象；应先核对差异。服务升级使用正常源码构建和数据库迁移，不手改运行记录。

### 通用协作验收

同一安装命令增加 `--template collaboration-check --upgrade` 可安装独立验收图。运行时选隔离空目录，输入“写一份发布说明”，在会话回复版本号。此测试故意令编写节点首次漏掉验收章节；独立校验节点必须真实读取文件后交回，修正再交接，并通过固定完成线结束。检查 Issue 中启动、提问、交接和最终完成通知与平台链接。这里的缺失章节是明确的故障注入，不代表产品验收已通过。


## 局部修复与指定起点

交付模板始终先准备独立任务分支、关联 Issue，再由“判断任务与起点” Agent 读取任务和必要仓库事实。它用同一原生 Agent 能力和 handoff 工具选择 requirements、design 或 development；平台引擎没有研发意图分类规则，也不根据标题关键字跳阶段。

- 输入“修复计数包含已删除项的问题，预期只计当前列表”，范围和复现可确定时直接进入开发。需求、设计节点不会运行。
- 输入“从开发开始，修复部署脚本的启动失败；保留数据”，Agent 确认已有证据足够后采用；这属于自然语言指定起点，不是平台下拉框的无条件越过前置节点。
- 已有需求但需要结构调整时进入设计；“推荐不太对”且仓库上下文不足时，入口先澄清，在同一会话回答后继续。

入口通过交接记录目标、验收依据、起点和跳过理由，后续按实际执行阶段的 Skill 生成产物并引用真实路径。短路径仍经过真实开发验证、项目测试、独立 QA、交付说明和草稿 PR。修复影响到哪里就验证哪里；不得为了省步骤省掉相关失败恢复，也不为脚本修复要求无关的全产品 UI 验收。

该模板故意不把研发节点放进 start_nodes：那个通用参数表示直接从指定节点启动，会略过分支和 Issue 准备。需要绕过业务阶段时用上述入口路由；已有工作区的 QA 返工使用 qa-rework 模板。

用原清单加 --upgrade 安装此次更新，会注册任务判断 Agent 并升级模板和阶段策略。在途 Run 的图不变；先完成或停止受影响任务，再升级共享 Agent 定义。新仓库用相同安装入口即可获得短路径，不需手改运行数据。

启动脚本等任务需要监听本地端口时，开发 Agent 的原生沙箱可能不具备该能力。该模板由管理员配置项目测试 Connector 在宿主执行固定验证入口：开发节点提交实现、测试和沙箱失败证据，随后由项目测试节点实际执行，失败回研发，成功再进独立 QA。确认稳定的沙箱环境限制后，不因代码或文档变化重复运行必然在相同限制处退出的命令；保留失败命令、代码版本及受限范围，完成可运行检查并交接当前代码，由宿主完整验证。代码、用例和依赖错误不能当作环境限制跳过；相关执行环境改变后须重新核验。QA 独立核对用例和真实回执，有缺口继续返工。不得修改测试为恒通过或临时放开 Agent 权限。这不授权任意未配置的宿主命令；其他测试入口由管理员配置相应 Connector。

同一宿主失败反复出现或修复未改变实测状态时，研发节点先收集能区分原因的最小诊断，区分产品、测试驱动、时序与环境，再据证据修改；模拟夹具的行为不代替原生机制证明。诊断保留原断言、限制输出并进入原固定验证入口，不通过换成更弱测试或临时扩权放行。 工具回执有容量限制，关键边界应进入精简摘要；完整白名单证据超出容量时保存到项目既有验证产物目录并交接路径，不依赖被截断的控制台输出。该阶段策略由同一安装器加载，新安装直接获得；已有安装按原 manifest 加 `--upgrade` 同步，仍须先完成或停止受影响任务。

### 通知失败后的处理

在运行页的通知记录中，`失败` 可单独重试，不重跑 Agent 或节点。发送前的查重查询失败会按此处理；`结果未知` 只查询外部回执，避免 POST 已成功后重复发评论。升级必须同时更新平台二进制和模板，已有 Run 的定义及记录保留。旧版本已记为结果未知、且无法证明发送阶段的通知不会自动重发，应结合 Issue 实际评论查证。

Go 项目安装示例：在原安装参数中加入 `--test-command-json '["make", "verify"]'`。项目负责实现该入口，执行必要的 Go 检查、HTTP 集成测试及真实后端浏览器旅程。测试 Connector 仅获得显式配置的代理变量，不获得 GitHub Token。升级时保留同一命令参数。现有静态 browser.check 可验证原型；完整后端 UI 必须在项目自己的验证入口或真实网页上验证。

## 共享浏览器工具与运行入口

浏览器能力统一注册为 `browser-validation`（页面名称“浏览器验证”），各项目 Agent 引用同一注册，不再按项目 prefix 创建工具。平台在新会话准备时将管理员配置参数中的 `{{workspace}}` 绑定为已核验的会话工作区，同时固定 stdio 启动目录；浏览器仍强制校验 `--workspace-root`，模型不能提供其他工作区。证据按工作区摘要隔离，不向模型开放证据目录选择。

此安装器要求包含工作区参数绑定的新版平台二进制。先完成或停止受影响在途 Run，再按正常构建／备份／升级流程更新平台，使用原项目 manifest 加 `--upgrade`。默认共享 manifest 和证据在维护源 `.data/` 下；部署到不可变版本目录时，显式指定稳定的 `--browser-manifest <private-shared-manifest>` 和 `--browser-evidence <private-shared-evidence-root>`，所有项目使用相同参数。共享 manifest 是该注册的唯一升级来源；项目 manifest 只保存引用。配置漂移仍拒绝覆盖，不能删除清单重新安装。

旧浏览器配置在项目升级后，仅当没有 Agent 引用且没有该安装的可恢复 Run 时停用；仍在使用时保留以保障恢复，不删除历史。旧版项目 `--evidence` 参数仍接受，供已有命令兼容；共享浏览器证据使用 `--browser-evidence`。静态页面验证的能力边界不变。

查看执行进度：左侧“智能体编排” → 对应工作流卡片的“运行记录” → 点击某次运行 → 当前节点“进入 Agent 会话”。编排详情也提供“运行记录”；每个工作流拥有独立 Runs 页面，不在总览混列全部运行。运行状态、节点历史和会话是实际执行进度，不按阶段数量推算完成百分比。

升级保护从 manifest 的 Agent 配置识别实际角色，不依赖角色名称前缀。安装器通过运行清单 API 的 `before` 游标逐页检查完整历史，每页最多 200 条；分页期间新增运行不会挤掉旧记录。旧版平台若忽略游标并重复返回同一页，安装器会明确拒绝继续，应先升级平台二进制，不能删历史或手改 manifest 绕过保护。

## GitHub Issue 入口与 Workflow SDK

同一张研发图接受两种输入：网页输入任务时创建 Issue；GitHub 入口传入 `parameters.issue_number` 时核对并关联原 Issue。平台记录真实 GitHub 回执，后续评论和 PR 使用同仓原 Issue。CI 不等待 Agent 完成，不保存原生会话，也不逐阶段触发 Actions。

在受信平台主机的 Runner Python 环境安装维护源 SDK（Python 3.10+）：

```sh
<runner-python> -m pip install <platform-source>/sdk/python
```

为该仓库创建平台 caller 账户，从平台用户管理页生成 API Token，保存到 Agent 工作区以外的私有文件（权限 0600）。不要放到 Issue、工作流 YAML 或 Git 仓库。使用原安装命令和原 manifest，追加 `--upgrade --authorized-user <platform-user-id> --github-config <private-ci-config.json> --github-token-file <private-token-file>`；首次安装不加 `--upgrade`。安装器同步授权 Agent、Connector 和 Workflow，导出配置并记录在原 manifest。以后不显式覆盖授权时继承原值，标准升级同时维护已导出的 CI 配置。已有在途运行先完成或停止。

Runner 的 Git HTTPS 连接需要代理时，在同一安装命令中指定 `--git-proxy <proxy-url>`；配置保存在私有配置及原 manifest 中，后续升级继承。仅允许无凭据的代理端点；入口将它写入专属 checkout 的 Git 配置，使准备和发布使用相同网络路径。对已失败或停止的原 Run 重跑入口时可更新该配置，但不会自动重新执行节点；随后通过 Run 的停止／回退入口重试失败步骤。不要将主机地址写入项目 YAML 或修改全局 Git 配置。克隆失败不会留下可执行的任务工作区，恢复网络配置后通过 Actions 重跑同一事件。

在项目 checkout 安装入口：

```sh
python3 <platform-source>/examples/platform-workflows/install_github_entry.py \
  --project <project-repository> --repository <owner>/<repository>
```

将生成的 `.github/workflows/agent-platform-entry.yml` 及 `.github/agent-platform-entry-source.json` 通过项目正常 PR 合入默认分支。后续通过同一命令加 `--upgrade` 更新；手工改动会被漂移保护拒绝覆盖。仓库 Actions variables 设置 `AGENT_PLATFORM_WORKFLOW_ROOT=<platform-source>`、`AGENT_PLATFORM_WORKFLOW_CONFIG=<private-ci-config.json>`。复用能访问平台主机的现有 Runner；分发模板默认标签为 self-hosted/macOS/ARM64/he-full，安装位置必须包含 `.data/runner-venv/bin/python`。现有 Runner 标签不同的部署应在维护源模板中适配并通过安装器分发，不手改项目副本。

当前信任边界与旧方案相同：只接受仓库 Owner 本人创建的 Issue 和评论，Actions 原触发者及重跑者都须为 Owner；不是面向匿名公共仓库的自动执行服务。普通 Issue 评论转发到当前 Agent。平台回写带受识别标记，不触发输入回环。GitHub API 再读取 Issue/评论校验归属，不把 PR 当 Issue，不执行事件文本里的命令。

Actions 完成只表示事件已交给平台；研发进度在原 Issue 和 Run。必要澄清可在原 Issue 评论回复。无法接收的评论返回明确失败通知，并重新读取Run状态给出恢复指引：交接或停止中先等待并核对可接收输入的Agent节点；停止/失败时先按页面继续或回退，再在Actions输入原Issue和`comment_id`重试。已完成Run不能重新打开，新增研发工作应创建关联原Run/PR的新Issue；状态查询失败时只提示核对，不建议重放或重建。对已接收评论的重试返回原回执；修改旧评论不产生新指令，需要新增评论。 若交接时已有入队补充，平台拒绝提前交接并自动在同一会话接续；Agent 应正常结束当前轮，处理补充后重试交接，不要求用户重发，也不循环调用交接工具。该恢复提示随平台二进制升级生效，旧冻结图与会话仍沿用原身份。首次接单/评论的快照与锁由适配器保存在私有 state_root，未知响应通过稳定事件键找回原 Run。恢复时保留这些状态文件及平台数据库，不能删除它们来绕过去重。

SDK 的 `start_workflow`、`workflow_by_request`、`workflow_run(s)`、`workflow_message`、`workflow_command` 与 `wait_workflow` 均复用公开 API；企业微信、钉钉以后实现各自的身份与事件适配即可，当前未实现这两类渠道。平台参数为有界字符串映射，固定命令通过 stdin 获取，不进行 shell 插值。


## 任务文档与 Trellis（新版安装契约）

文档内容、必需产物和适用裁剪遵循各阶段挂载的 Skill。模板只指定任务公共文档根目录 `docs/workflow/runs/<run_id>/`；在其中保留 Skill 的目录层次与文件职责。已有项目级文档和 `.trellis/spec/` 仍在原位维护。交接使用真实 artifacts/inputs 路径，下游不得猜测固定文件名。PR 正文输出到当前根目录的 `pr.md`，发布检查本轮 QA 实际交接文件，不强制 `qa.md`。旧 Run 的固定路径只作为旧协议兼容，不自动移动旧证据。

安装前在受信 Runner 安装 `npm install -g @mindfoldhq/trellis@0.6.15`，或通过 `--trellis-executable` 指定该版本 CLI。安装器记录绝对可执行路径与版本，prepare 会复核；升级仍用原 manifest。首次任务通过官方 `trellis init --codex --yes --skip-existing --user workflow` 初始化，不覆盖已有项目文件。生成的本机适配器和个人运行状态被忽略；公共脚本、规范与配置可正常审查。研发使用挂载的 `trellis-spec-bootstrap` 填写真实项目规范，随后 `trellis-before-dev`、实现、`trellis-check`，必要时 `trellis-update-spec`。空白模板或只挂载 Skill 不构成通过证据。平台负责阶段交接和 Git 发布，Trellis 会话自动提交关闭。

已有 `.trellis` 安装沿用，但项目 `.version` 必须与受信 CLI 版本一致；不完整或旧版本安装显式失败，先用固定版本 CLI 的 `trellis update --dry-run` 检查，再通过 `trellis update --create-new` 保留本地修改并完成必要合并，复核后重试。Trellis 初始化和项目上下文脚本不继承 Git 写令牌。新目录契约通过 Connector 的 `--task-docs` 开启，旧冻结 Connector 参数不变，发布仍兼容旧路径。请在没有在途任务时升级；用新 Issue 验证，不用旧任务已有的 QA 结论冒充新版验证。


报告节点也挂载研发交付管理 Skill：在独立 QA 后根据真实证据闭环公共任务、里程碑与总览，再整理 PR 正文。公共总览、任务和里程碑各自在原位维护唯一当前状态，遵循 Skill 各自的状态模型；历史结论与失败证据保留并引用，避免重复追加相互矛盾的“当前状态”。正文生成不能替代公共状态同步；Git 发布、合并和部署仍由对应后续入口决定，报告不得提前声称发生。升级通过原 manifest 应用同一 Skill 挂载与节点指令。

PR正文由GitHub Connector原样发布，报告阶段应核对对外证据链接使用实际仓库及本任务分支/文件的明确URL；正文所在仓库路径的相对链接不能直接作为PR页面链接。新产物未推送前不宣称远端可访问，发布后仍从真实PR复核。此规则随原manifest升级应用，不需要改写历史仓库文档。


### 命令日志诊断

平台新版为新命令执行保存最多 8 MiB 的脱敏日志；运行页每条命令回执旁可打开“查看命令日志”。研发/QA 的当前节点可用 `read_command_output` 按原测试 seq 分页读取，不必重复测试只为取回被简短回执截掉的失败断言。`truncated=true` 明确仍有未保存输出，旧回执没有日志时继续保留未测/证据不足判断。SDK 调用 `workflow_command_output(run_id, seq, offset=0)` 并跟随 `next_offset` 至 `eof`。该能力来自平台二进制的标准升级，安装模板不复制日志实现；升级前先等待或按正式入口停止在途工作，旧 Run 历史保持原样。


项目完整测试默认预算 900 秒，可用 `--test-timeout-seconds <1..1800>` 明确配置并在升级时保留。预算是当前管理员策略，新执行采样后固定；它不改变已运行命令的 deadline，也不替换已有 Run 的冻结命令。超时须先检查副作用，经 stop/静止/return 原测试节点显式重试，不能靠改预算自动重放。回执的 `timeout_seconds` 为该次实际预算。升级先确认无在途执行，通过原 manifest 应用预算及原固定测试命令。


## 交付后审查发现问题

原 Run 已结束、但产物仍需修订时，从该 Run 页面填写回退原因并选择研发或 QA 节点。已完成历史保留；新增节点继续使用原工作区、冻结图、任务文档根目录与分支。平台先检查工作区未被其他未结束 Run 占用。不要直接续聊已交接的旧会话，也不要重复创建 Issue。返工后的项目测试、QA、报告和发布按原图执行；同一 PR 节点更新原打开的 PR 说明与回执。已关闭 PR 需要另行确认新的任务范围，平台不会自动新建或重开。

这一入口是运行时能力，无数据库迁移，不需重装模板或覆盖原 manifest。已有安装先核对无在途执行，备份数据与旧二进制，运行固定验证入口，再按服务安装流程升级二进制和内嵌页面。原图、Agent/Connector IDs 与 Run 记录保留；恢复启动后通过页面返工复验。模板/Agent 配置改动仍走原 manifest 的 `install.py --upgrade`。
