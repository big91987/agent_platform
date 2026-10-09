# 平台内研发交付示例

本例把研发流程保存在 Agent Platform 的图中，由持久 Run 推进。GitHub Issue 与平台网页都是任务入口；CI 只转发事件，平台推进研发节点。当前真实验收状态见验证记录，不能以代码或单元测试代替接单实测。平台本身不包含“需求／设计／研发”等业务阶段；这些是本目录提供的可编辑模板。

## 组成与边界

- `software-delivery.json`：准备分支 → Issue → 任务判断 → 必要的需求／设计 → 研发 → 项目测试 → 独立 QA → 交付说明 → 提交推送 → 草稿 PR。测试失败回研发；QA 可回需求、设计或研发；关键问题留在当前 Agent 会话澄清。所有回路保留在同一个 Run 中。
- `qa-rework.json`：从已有产物的独立 QA 开始，发现问题返回需求／设计／研发，修复后重新测试和 QA；不创建 Issue、分支或 PR，最后由验收人员选择结束或继续返工。
- `product-e2e.json`：独立工作区 → `【产品测试】` Issue → 用户旅程测试计划 → 独立文档评审 → 浏览器与测试准备 → 固定端到端命令 → 独立实操复核 → 测试报告。计划未通过回规划整改；实质计划变更重新评审。测试成功与失败均进入实操复核；只允许修订测试设施，不修改产品或自动发布。Run 完成表示报告交付，产品 Go/No-Go/Blocked 以报告为准。
- `prompts/`：阶段指令，复用 Harness Skill，按项目规模裁剪。普通选择记录推荐并继续；用户明确要求、关键歧义和高风险操作才等待确认。阶段不适用时说明依据再交接。
- `install.py`：使用公开管理 API 注册 stdio MCP、Connector 和带节点独立执行配置的图；安装清单用于重入与升级，不操作数据库，也不为阶段注册共享 Agent。
- `repository.py`：固定的准备分支／提交推送命令。每个 Run 使用 `workflow/<run_id>` 分支；拒绝接管脏工作区、错误仓库、错误分支和明显凭据文件；不 reset、不强推、不合并远端PR；已发布候选的本地基线整合见下文。
- `browser_tool.py`：复用 `../github/tooling/full_harness/browser` 的锁定浏览器运行时；只借用浏览器能力，不使用其 CI 控制器。检查 app 或设计原型，保存真实截图／检查结果；支持页面内受限检查脚本。

默认验证命令为 `npm test`，可用 `--test-command-json '["make", "verify"]'` 为 Go 等项目配置自己的固定验证入口。命令按 argv 执行，不隐式使用 shell，阶段指令使用同一配置。浏览器工具本地服务根为 `app/` 或 `docs/workflow/prototype/`。其他技术栈应修改图中管理员配置的测试命令与注册工具，不改平台引擎。浏览器可验证原生缩放和可访问树，不能把这些称为真实读屏器语音验收。Agent 需对未测项明确说明。

## 安装

需要运行中的平台（含节点 `agent` 内联配置及 Workflow/Connector API）、Python 3.11+、Node/npm、Git、可用的原生 Codex，以及独立项目 checkout。先升级平台二进制，再运行本版安装器；旧平台不支持内联配置。平台账户必须有管理权限。GitHub Token 放在平台服务环境中，至少能访问指定仓库的 Issue、内容与 PR；本例不需要 Actions 管理权限。

Skill 是独立资产：指定包含 `defining-platform-products-cn`、`platform-architecture-cn`、`managing-engineering-delivery-cn` 的目录，并单独指定 `validating-platform-releases-cn`。所有目录必须在平台主机可读。节点直接配置 executor/model，按本例设置阶段指令、Skill、工具和权限。新安装默认 executor 为 codex、model 为空（使用原生执行器默认模型），也可显式传入以下参数。

在平台主机从源码 checkout 运行（将占位符替换为实际路径；安装清单与证据不要提交）：

```sh
# 平台服务启动环境中设置 WORKFLOW_GITHUB_TOKEN；不要把 Token 写进 JSON 或命令参数。
# 用组织支持的凭据机制为 Git HTTPS/SSH 配置认证和提交身份。
# 命令 Connector 将服务端 WORKFLOW_GITHUB_TOKEN 映射为 GH_TOKEN，供 gh credential helper 使用。
# 在当前终端安全设置 PLATFORM_ADMIN_PASSWORD，然后执行：
python3 examples/platform-workflows/install.py \
  --platform-url <platform-url> \
  --executor codex --model <native-model> \
  --repository <owner>/<repository> \
  --workspace-root <dedicated-workspace-root> \
  --skill-root <harness-skill-directory> \
  --qa-skill <qa-skill-directory> \
  --evidence <private-evidence-directory-outside-workspaces> \
  --manifest <private-installation-manifest.json> \
  --prepare-browser
```

`--prepare-browser` 通过已有 package-lock 执行 npm ci、安装 Chromium 并检查真实启动。若使用代理，需在安装器与平台服务环境中配置相同的 HTTP(S)_PROXY/NO_PROXY。仅这些显式代理值写入节点环境；GitHub Token 不传给 Agent。安装清单存配置和 ID，不存登录 Cookie/密码；不要在代理 URL 内嵌凭据。

原命令中的 `--base-agent` 为兼容保留，仅在没有历史配置时提供 executor/model 初始值；新安装无需该参数或任何共享 Agent。升级默认保留原 manifest 中各节点或旧阶段 Agent 的 executor/model 和已记录环境，显式 `--executor` / `--model` 才覆盖对应模型设置。已有节点配置后不再读取基础 Agent，其后续改动不影响节点。

`--base <branch>` 同时指定准备分支与草稿 PR 的目标基线，默认 main；适用于非 main 主线或隔离验收分支。升级时保持原基线参数，避免意外改回默认值。

安装不会启动 Agent 或创建 Issue。打开返回的流程链接，点击“运行”，输入普通需求和根目录内的**独立、干净 checkout**。每个独立 Run 必须使用不同 checkout，避免交叉修改。引擎拒绝占用未结束 Run 的同目录、父子目录或符号链接别名；停止／失败仍保留占用，因为之后可以恢复。完成或正式取消后释放。浏览器工具证据目录在工作区外，便于保留诊断；交付截图与报告仍写到项目的验证目录。

### 平台机器人自动评论

平台自动评论可以用独立 GitHub App installation 身份。先在 GitHub 创建 App，为评论用途授予 Issues read/write，并仅安装到所需仓库；此方式无需用户 OAuth 授权，也无需启用 Webhook。App 名称须在 GitHub 创建时确认可用。配置 App issuer（推荐 client ID，也支持数字 App ID）、installation ID 和下载的 RSA 私钥，私钥及注册表放平台主机的私有目录、权限0600，并置于原生 Agent 工作区之外。

平台服务环境的 `PLATFORM_GITHUB_APP_CONFIG` 指向私有 JSON 注册表，例如：

```json
{
  "AGENT_PLATFORM_GITHUB_BOT": {
    "app_id": "<client-id-or-app-id>",
    "installation_id": 123456,
    "private_key_file": "/absolute/private/path/app.pem"
  }
}
```

Connector 的 `token_env` 是 GitHub 凭据名称：注册表中该名称存在时，平台签署 App JWT，向固定 GitHub API 申请限定 Connector 仓库的短期安装令牌，在到期前自动重新申请；密钥和令牌不进图、Run、Agent 环境或回执。配置/权限错误停止动作，不降级为个人账号，不盲重发未知结果。名称未注册时沿用同名 Token 环境变量，已有 PAT 安装保持可用。注册表、安装或密钥变更后读取当前配置，不修改旧回执。

原安装命令增加 `--notification-token-env AGENT_PLATFORM_GITHUB_BOT`，只将自动评论 Connector 设为机器人身份。Issue/PR 与准备/推送仍使用原 `--token-env` 凭据；后续若需把这些也改为 App，先确认对应仓库权限与工程入口。省略通知参数升级保留已配置的评论身份，显式传回原凭据名称才恢复个人身份。仍须先安全升级平台，再按原清单 `--upgrade`；在途 Run 保留冻结的 Connector 选择，不为换作者改运行记录，新的 Run 使用新配置。创建 App 或本地测试通过不等于实际机器人评论已验证，应通过正式 Hook 核验可见 `[bot]` 作者、原生内容、去重和失败恢复。

参考：[App installation 认证](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation)、[安装令牌](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)。

## 日常操作

- 图中选择节点、连接输出路线、保存即产生新版本；已启动 Run 使用旧图快照。
- 运行详情能打开当前 Agent 会话。普通推荐自动推进；需要补充时在该会话回答。
- 人工调整路线：停止 → 等待停止完成 → 选择目标节点并写理由 → 返回执行。停止不会撤销已发生的文件修改或 GitHub 动作。
- 运行中的工具回执和历史可刷新查看。结束只表示这张图走到结束节点；本例把结束放在独立 QA 和草稿 PR 之后，不等于已人工批准合并。
- PR 默认以 `Refs #<issue>` 关联任务，不根据流程结束自动声明完成；确实解决任务时由交付正文明确写出关闭关系。由人决定合并；本例不部署、不自动合并。

## 升级与恢复

升级维护源后，用同一清单及相同目标参数重跑，并加 `--upgrade`。安装器逐对象记录成功，重复安装无变更时不创建新对象／版本；API 返回的空默认字段不应触发伪升级。平台外编辑过的已安装对象会被拒绝覆盖，应先对照清单和当前配置合并改动，再选择维护方式。不可丢弃清单后盲目安装，否则可能无法确认已有对象归属；同名冲突会明确报错。

图、节点内联 Agent 与 Connector 配置对 Run 冻结。旧 manifest 使用原命令加 `--upgrade`，在原工作流的新版本中迁移阶段配置；原工作流和 Connector ID 保留，旧共享 Agent 不修改、不停用、不删除，已启动 Run 和已有会话不改写。旧 `agent_id` 冻结图继续按旧兼容规则执行。安装器同时按工作流 ID 和旧 Agent 引用检查在途 Run，running／waiting／stopping 时拒绝升级；请先完成或正式停止。旧阶段 Agent 或节点配置存在外部编辑时仍拒绝覆盖。**脚本和 Skill 的磁盘内容不受数据库快照冻结**：生产安装应使用不可变版本目录，并在新安装配置中引用新路径，保留旧目录直至相关 Run 结束。

命令执行被中断时，其副作用可能已经发生，平台不会自动重放；检查仓库和回执后通过正式停止／返回入口重新执行。准备分支可以识别同一 Run 已创建的分支；提交推送仅在正确分支上执行，重复无改动发布不会造空提交。GitHub 接口响应丢失时按 Run/节点标记核对已创建资源，不能凭空补成功状态。

安装中途失败：保留清单，修复明确错误后重跑。若 HTTP 写入返回不确定结果但清单未落盘，应先在管理页确认该对象并恢复清单归属，不能连点安装造成重复。安装不是跨多个 API 的原子事务；已成功的对象会保留。

## 验证入口

在测试仓发起本模板的任务及创建 PR 时，遵守维护源 [AGENTS.md 的写作规范](../../AGENTS.md#测试仓-issue-与-pr-写作规范)：使用 `【类别】具体目标`，区分产品功能、产品修复、产品测试、Harness 验证、Harness 修复及交付维护。正文开头先说清问题、要做的具体工作和完成后的变化，再说明范围、可观察验收条件及旧任务关系；PR 描述最终实际改动和已执行验证，不能复制任务愿望当交付结论。执行元数据与完整证据放后，不把流程重跑当新增功能。该规则适用于本项目发起工作的所有测试仓，不是平台对任意用户输入的通用限制。

## 专用产品端到端测试

使用测试仓验证平台和本模板时，由测试 Pipeline Agent 定义、执行和验收产品旅程，协调者以 Harness Builder 角色建设并核验工具、权限、交接及回执，用真实产品任务检验模板的可靠性与效果。这是平台测试策略，不限制协调者在其他任务中的职责。使用 `--template product-e2e` 配合独立 `--prefix`、工作区和 manifest 安装，不升级正在研发的流程。配置 `--test-command-json` 为该项目的固定 Python/浏览器入口；执行节点负责创建测试入口和准备独立受测实例，不能要求协调者代写产品用例。

规划与计划评审使用不同 Agent 节点。评审依据实际 PRD、AC、设计和业务规则，承接 PM/研发对范围、规则与可测性的核对；要求逐项对应可执行用例和证据，尤其检查核心能力的非零阈值、边界、隔离、并发及失败恢复。文档缺失、规则不明或核心用例缺漏必须列出阻断，不以旅程数量、零配额拒绝或局部成功放行。评审 READY 经 handoff 才进入执行，NOT READY 回规划；执行时改变范围/断言经 `plan_review` 再审。路由约束保证经过评审节点，评审内容质量仍需检查其实际产物；它不是自动语义判定器，也不等于产品验证通过。新节点通过原 `--upgrade` 应用于同一工作流的未来任务，在途 Run 保留冻结旧图，不回填计划评审历史。

需要真实网页与外部供应商时，安装显式指定 `--agent-network-access --allow-agent-elevation`，并用 `--browser-skill /path/to/playwright` 挂载现有真实浏览器 Skill。浏览器 Skill 挂载在省略参数升级时同样保留，显式 `--clear-browser-skill` 才删除。默认新安装不授予这两项权限；省略参数升级时保留原授权，可用对应 `--no-...` 参数显式撤销。联网允许原生 Agent 网络操作；elevation 允许申请所需执行权限，不是自动接受所有请求。为已授权的自动化任务显式增加 `--agent-approvals-reviewer auto_review`，由 Codex 原生审核权限申请；默认 `user` 保持管理员逐次审批。省略参数升级时保留原审批方式，显式 `--agent-approvals-reviewer user` 可恢复人工审批。自动审核不改变工作区沙箱，不能与外部工具的“每次确认”混用；冲突须选择人工模式或明确修改工具设置，原生拒绝或失败须保留真实记录并在 Issue 反馈，不循环绕行。浏览器缓存尽量使用任务目录；Chromium 仍可能需要原生审核。Python/Chromium/系统浏览器运行仍需实际探针和合法审批回执，不能以两个配置值判运行成功。受信固定 Connector 的宿主执行与 Agent 亲自操作分别记录。

私有供应商配置放本次工作区的忽略目录或用户指定私有文件，只把文件引用交给 Agent。秘密不放 Issue、PR、命令行、普通环境说明、日志、截图或 trace/HAR；准备与独立复核分别检查脱敏。真实调用必须有用户授权和有界额度，不默默换模型或退回受控上游。工作流本身不内置某家供应商或某个产品的旅程。

本图创建的 Issue 带平台标记，既有 GitHub 入口会忽略，不再启动另一研发 Run。未配置本图独立 GitHub 入站，因此启动通过正式平台 Run API/页面，保持当前研发入口不变。产品缺陷在报告中交付可复现问题包，修复交另一个产品研发任务；测试设施缺陷才走 `test_repair`，保留旧失败与新回执。

报告节点交付Markdown完整报告、GitHub可读正文、自包含HTML Dashboard及白名单附件清单（文件路径、bytes、SHA256与来源）。Dashboard展示实际逐项结果、整改和证据，由Pipeline进行真实浏览器展示检查；展示通过不改变产品测试结果。授权发布方在本仓Issue/Release或已有报告站点发布，核对实际回执和同版文件后回填链接；仅本机路径不算GitHub交付。HTML下载附件与在线Dashboard分别说明，私有仓库保持现有可见范围，不能为托管页面擅自公开证据。本模板尚无自动上传/托管Connector，报告准备和授权发布交接不能描述为已自动发布；需要无人值守发布时，应实现通用受信发布入口并真实验证。

### 测试报告交给开发修复

等独立复核与报告实际交付后，协调者读取最终 handoff 中的报告、问题包和文件级产物，按已复核产品缺陷创建或更新 `【产品修复】具体行为` Issue。开头说明用户遇到的问题及修复后的结果，附受测版本、复现步骤、预期/实际、首失败回执、原验收条件与回归要求，并关联产品测试 Issue/Run；原因假设和未执行范围单列。先检查已有任务，同一问题沿原 Issue 接续，无产品缺陷时不创建修复任务。

报告与相关脱敏截图、日志、测试文件通过现有带 SHA-256 的材料 ZIP 入口传递（见下方“带原型和文档的Issue输入”），在问题正文写 `agent-platform-material` 描述，不能只给另一个工作区的本机路径。选择已确认缺陷所需的具体文件，包内清单记录实际文件摘要和来源；不带私有 Provider 配置、密码、未脱敏 trace/HAR 或缓存，材料作为输入不自动执行。准备节点实际成功下载/校验后再引用其回执路径交接，不把上传成功当研发已接单。

修复 Issue 明确指定从 `development` 开始，经现有研发入站创建独立 Run，`prepare → issue → intake → development`。准备和任务判断完成输入接收，不重做需求/设计；intake 核资料足够后通过原生 handoff 直接交开发。开发再走原固定测试、独立 QA、交付和 PR。不要用 `start_node=development` 跳过分支/材料准备，也不将新的修复塞进仍在执行的无关 Run。登记实际 Issue、修复 Run、development 接单和报告/材料关联；请求结果未知先查去重记录，避免重复启动。

```sh
PYTHONPATH=sdk/python python3 -m unittest discover -s examples/platform-workflows -p '*_test.py' -v
ruff check examples/platform-workflows
ruff format --check examples/platform-workflows
```

本地 Git 测试覆盖脏工作区、分支隔离、真实提交推送与重试；它们不代替真实平台旅程。平台完整证据在 [验证记录](../../docs/03-delivery/workflows-verification.md)。发布前需要从网页实际走通一句话需求、工具执行、QA 回退、修复、GitHub 草稿 PR，以及停止／重启后的恢复。

## 已发布候选更新上游基线

同一个草稿PR等待交付期间主线改变时，管理员可在检查原任务、候选分支及副作用后，从运行页正式回退到“准备任务分支”。继续同一个未合并目标，不新建重复Run或用完成Run承接新功能。准备Connector核对本Run最近成功publish回执中的分支与HEAD；未发布的开发中准备重试保持原继续行为。已发布候选才fetch配置的base并本地`merge --no-commit --no-ff`，返回精确`integration_base`，不自行提交或推送。新候选仍经过研发、原固定测试、独立QA、报告与publish/PR，不沿用旧候选通过结论。

准备前有未提交产品改动、暂存改动、HEAD不同或最近发布失败/回执不确定会拒绝并保留现场；本Run文档中的未暂存阻塞证据可以保留。合并冲突以精确冲突路径返回，保留文件和MERGE_HEAD交研发；研发编辑文件后交固定tests Connector，Connector先以Git差异检查拒绝残余冲突标记，再只暂存已解决的冲突文件并运行原门禁，不提交或推送。发布器拒绝尚有未合并索引的候选。准备回执丢失后的重试复用原待提交merge，不重新fetch移动基线或重放合并。普通首次任务、执行权限、节点图及已有验证预算不变。能力由本维护源repository.py和原安装命令加`--upgrade`交付；冻结旧Run不改定义或历史，只有明确正式回退到prepare才取得新整合行为。运行安装引用维护根脚本时按维护根版本应用，不能让协调者代执行产品合并。确认新冷门禁和独立QA对应同版候选后再合并远端PR。

## 已有产物的返工验收

用上述安装参数和原安装清单追加 `--template qa-rework`，注册带独立节点配置的返工图，保留原交付图，复用浏览器工具及测试 Connector；启动前选择已有需求、设计和实现的独立 checkout。不要选用另一个未结束 Run 占用的工作区。需升级配置时仍使用 `--upgrade`，不绕过漂移检查。

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

配置了 `agent.reply.completed` 的通知通道也接收执行审批状态：真实原生审批挂起时发送“等待执行审批”及会话入口，处理后发送批准、拒绝或失效结果。通知不替用户审批、不解析自由文本，也不公开审批请求中的命令、原因或环境。可显式配置 `approval.requested` / `approval.resolved` 替换对应默认正文；`approval_status` 是结果字段。没有回复通知通道且没有显式审批规则时不对外发布。

该能力随平台二进制标准升级生效，复用冻结 Run 已授权的评论 Connector 与 Issue 来源，不改图、工作区或执行权限。不会回补升级前已处理的审批；审批已处理而等待通知尚未发出时，该通知显示“已跳过”，避免再提示用户处理失效申请。发送结果未知仍只查外部回执，后续审批处理通知不能证明此前发送失败。新装和升级沿原安装器及 manifest；运行服务升级先检查在途执行，按正式停止/备份/升级/恢复路径保护当前任务。

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

用原清单加 --upgrade 安装此次更新，会配置任务判断节点并升级模板和阶段策略。在途 Run 的图不变；先完成或停止受影响任务，再升级工作流。新仓库用相同安装入口即可获得短路径，不需手改运行数据。

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

Runner 的 Git HTTPS 连接需要代理时，在同一安装命令中指定 `--git-proxy <proxy-url>`；配置保存在私有配置及原 manifest 中，后续升级继承。仅允许无凭据的代理端点；入口将它写入专属 checkout 的 Git 配置，并用于读取/回写 GitHub API 的子进程，使接单、通知、准备和发布使用同一配置的 GitHub 网络路径；不修改平台 API 客户端或全局进程环境。对已失败或停止的原 Run 重跑入口时可更新该配置，但不会自动重新执行节点；随后通过 Run 的停止／回退入口重试失败步骤。不要将主机地址写入项目 YAML 或修改全局 Git 配置。克隆失败不会留下可执行的任务工作区，恢复网络配置后通过 Actions 重跑同一事件。

在项目 checkout 安装入口：

```sh
python3 <platform-source>/examples/platform-workflows/install_github_entry.py \
  --project <project-repository> --repository <owner>/<repository>
```

将生成的 `.github/workflows/agent-platform-entry.yml` 及 `.github/agent-platform-entry-source.json` 通过项目正常 PR 合入默认分支。后续通过同一命令加 `--upgrade` 更新；手工改动会被漂移保护拒绝覆盖。仓库 Actions variables 设置 `AGENT_PLATFORM_WORKFLOW_ROOT=<platform-source>`、`AGENT_PLATFORM_WORKFLOW_CONFIG=<private-ci-config.json>`。复用能访问平台主机的现有 Runner；分发模板默认标签为 self-hosted/macOS/ARM64/he-full，安装位置必须包含 `.data/runner-venv/bin/python`。现有 Runner 标签不同的部署应在维护源模板中适配并通过安装器分发，不手改项目副本。

当前信任边界与旧方案相同：只接受仓库 Owner 本人创建的 Issue 和评论，Actions 原触发者及重跑者都须为 Owner；不是面向匿名公共仓库的自动执行服务。普通 Issue 评论转发到当前 Agent。平台回写带受识别标记，不触发输入回环。GitHub API 再读取 Issue/评论校验归属，不把 PR 当 Issue，不执行事件文本里的命令。

平台通过网页运行创建的 Issue 也是既有 Run 的输出。正文中的 `<!-- agent-platform:` / `<!-- agent-platform-hook:` 是保留协议标记，opened 事件忽略此类 Issue；手动 dispatch 再读取真实 Issue 后同样拒绝创建第二条 Run。此类任务从原 Run 页面继续；新需求使用新 Issue，不复制这些标记。标记只用于识别平台输出，不能代替上述身份与仓库授权校验。

Actions 完成只表示事件已交给平台；研发进度在原 Issue 和 Run。 原 Run 的保存回执先写入日志及 Actions 摘要，接单通知失败仍保留可打开的链接并使该 Actions 失败；失败诊断只输出可确认的 GitHub HTTP 状态、传输类别或 unknown，不公开响应、命令参数或私有端点。结果未知时先核原 Run 和外部评论，不因错误消息重新创建任务。必要澄清可在原 Issue 评论回复。无法接收的评论返回明确失败通知，并重新读取Run状态给出恢复指引：交接或停止中先等待并核对可接收输入的Agent节点；停止/失败时先按页面继续或回退，再在Actions输入原Issue和`comment_id`重试。已完成Run不能重新打开，新增研发工作应创建关联原Run/PR的新Issue；状态查询失败时只提示核对，不建议重放或重建。对已接收评论的重试返回原回执；修改旧评论不产生新指令，需要新增评论。 若交接时已有入队补充，平台拒绝提前交接并自动在同一会话接续；Agent 应正常结束当前轮，处理补充后重试交接，不要求用户重发，也不循环调用交接工具。该恢复提示随平台二进制升级生效，旧冻结图与会话仍沿用原身份。首次接单/评论的快照与锁由适配器保存在私有 state_root，未知响应通过稳定事件键找回原 Run。恢复时保留这些状态文件及平台数据库，不能删除它们来绕过去重。

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

## 带原型和文档的 Issue 输入

首次接单支持一个ZIP材料包，包内可含PRD、交互原型、截图和验收说明。先上传到同仓库的GitHub Release Asset，再在Issue正文填写：

```agent-platform-material
{"url":"https://api.github.com/repos/<owner>/<repository>/releases/assets/<asset-id>","sha256":"<64 lowercase hex>","version":"<design-version>"}
```

也支持浏览器上传后得到的 `https://github.com/user-attachments/files/<id>/<name>.zip`；两种路径分别验证，不能把Release资产下载通过说成Issue拖拽上传通过。私有Release由准备Connector的既有GitHub身份下载，公开附件不带该令牌，重定向仅允许GitHub下载主机。本文没有提供自动上传Issue文件的REST入口；GitHub Issue创建API不含上传参数。

ZIP根目录必须有UTF-8 `manifest.json`，包含version和files；每个文件记录path、bytes、sha256。version必须等于Issue声明，整个ZIP和每文件都核验。文件路径必须相对，无点段、链接、重复，全部目录前缀不得有大小写或Unicode规范化碰撞；下载16MiB、单文件16MiB、总展开64MiB、256文件。不执行包内脚本或安装依赖，不让材料改变工具授权。

入口首次快照冻结材料描述，重试或修改原Issue不换输入。prepare成功回执包含material的实际版本/摘要/root/manifest_path，所有阶段从该回执引用资料；文件保存在工作区忽略目录 `.workflow-input/<run_id>/<sha256>/files/`。测试和发布前再次对照原包摘要复验，修改展开manifest也不能自证完整；固定门禁失败时不执行项目测试或发布。阶段正式产物仍在本Run文档根按Skill维护，不将私有输入自动提交到PR。

通过原 `install.py --upgrade` 更新prepare、tests、publish及阶段指令；GitHub入口同步使用维护源github_entry.py，新项目仍走原 `install_github_entry.py` 安装。旧Run沿用冻结命令，升级不会改其输入。没有材料的任务保持原行为。私有资产权限或网络暂时失败修复后用原Run正式回退/恢复；错误版本或SHA不能靠修改已接受Issue自动改包，需要明确的需求修正流程，首期不支持偷偷替换冻结材料。

SDK使用已有 `start_workflow(..., parameters={"material": json.dumps(description)})`；平台JSON API使用同一个parameters字段，不新增上传服务或依赖SDK专用附件状态。真实退出要求见 `docs/02-architecture/workflow-materials.md`；本地ZIP回归不等于正式Issue→Run→Agent→部署通过。

准备工具使用POSIX文件锁，当前运行契约为macOS/Linux。中断后在下一次同Run准备中清理失去执行者的临时目录；并发重试不会删除仍在下载的内容。已验证材料始终复验，不自动覆盖。

## 角色指令、任务输入与持续推进

正式安装器生成 `context_version=2` 工作流：节点 `agent.instructions` 保存角色、阶段职责和共同协作约定，具体任务、资料、修正通过入口 User Input 和 handoff 传递。Agent 节点不配置独立 `prompt`，不使用 `{{handoff}}` 占位符；交接工具说明直接根据连线和策略生成。人工确认节点仍可配置确认内容。

独立智能体与节点复用同一配置表单；节点额外配置交接、等待和自动继续。Run 每个 Agent 执行可查看冻结角色、原生项目规则发现来源、工具与首轮输入。所有节点使用启动时的 `workspace_path`，Codex 原生读取其 AGENTS.md，平台不重复拼接该文件，也不引入通用变量页。

原 manifest 加 `--upgrade` 会重建维护源拥有的角色配置并升级工作流版本；人工编辑漂移仍拒绝覆盖。冻结 v0/v1 Run、旧 Agent、原生会话与消息保持原样。自建旧图不会自动丢弃非空 prompt：长期规则转入角色指令，任务内容改由启动输入提供后，才能明确采用 v2。

正常回复结束不代表节点完成。必须调用合法 `handoff` 或 `complete_node`。确需用户回答时使用 `wait_for_input(kind=clarification, reason=具体问题)`，真实外部阻塞使用 `kind=blocked`；再向用户说明。无需等待且未完成的正常收尾，会在同一原生会话追加继续指令，默认最多 3 次，持续推进期限为节点开始后的 14400 秒。失败或用户停止不自动续跑；达到上限显示具体原因，保留现场。新的用户回复解除旧等待声明，不重置该节点的续跑预算。

安装升级仍使用原 manifest 和 `--upgrade`。安装器要求关联在途 Run 完成或正式停止；先保护在途会话与工作区，不能通过重建 Run 或修改数据库绕过检查。升级后新 Run 使用新协议，旧冻结图和已准备的原生会话保留旧语义。

详见 [输入设计](../../docs/02-architecture/workflow-session-input.md) 和 [用户指南](../../docs/04-guides/agent-platform-user-guide.md)。

标准节点显式配置 `allow_user_input: true`，可在节点设置关闭；关闭后不注册 wait_for_input，服务端也拒绝调用。manifest 升级比较保留显式 false：人工关闭与字段缺失不是同一配置，外部改动会被报告，不能由安装器静默重新开放。已启动 Run 的定义与停止状态保持冻结。

## 取消不再需要的运行

先 `POST /api/workflow-runs/{id}/stop` 并等待 stopped，再 `POST /api/workflow-runs/{id}/cancel`，两次均传页面/GET 返回的当前 `seq`。网页 Run 控制区和 Python `workflow_command(run_id, "cancel", seq=seq)` 使用同一路径。failed 且进程已退出也可取消；运行中/停止中和过期 seq 返回冲突。

取消保留文件、消息与外部回执，关闭当前会话，释放工作区；无法继续或回退。重新工作须建新的任务/Run。研发准备步骤仍拒绝脏目录，需先明确保留和交接已有成果，不能手改数据库或删除入口状态文件绕过去重。GitHub 旧 Issue 的重复事件仍关联原已取消 Run，不自动重启；新工作使用引用原 Issue/Run 的新 Issue。

### Agent 交接方式与固定输出

Agent 节点的“交接”页可选择两种方式：

- **自主交接（handoff）**：为各目标填写交接策略，Agent 选择目标并提交 summary、inputs、artifacts。研发与返工模板使用此方式。
- **固定流转（complete_node）**：设置唯一后续目标、输出 JSON Schema 和输出填写说明。Schema 约束工具的 `inputs` 对象，字段描述与填写说明一起提供给 Agent。调用不合规则返回错误，修正后才能继续。`summary`、`artifacts` 仍分别记录结论和产物路径。

例如，下游脚本需要一个布尔结论和文件列表：

```json
{"type":"object","properties":{"passed":{"type":"boolean","description":"实际检查是否全部通过"},"files":{"type":"array","items":{"type":"string"}}},"required":["passed","files"],"additionalProperties":false}
```

填写说明可以是：“运行约定检查，全部通过才将 passed 设为 true；files 只填写实际产生的工作区相对路径。”命令 Connector 的标准输入仍使用 `previous_results`，从对应节点的 `result.inputs` 读取这些值；布尔、数组、对象与数字不转成字符串。

支持对象、数组、基础类型、required、enum/const、allOf/anyOf/oneOf/not、长度/数量/数值范围及 pattern。Schema 根必须是 object；不支持引用、远程加载、默认值注入或 format 等未列出的关键字，保存时明确报错。格式留空保留默认字符串键值对象，适用于无需固定业务字段的简单场景。

新节点显式选择一种方式；切换到固定流转且有多个目标时，先选择保留目标。已运行任务始终使用自己的冻结配置；旧版混合定义读取时保持原行为，重新选择模式后只影响后续运行。标准安装器更新模板时继续检查 manifest 漂移，不覆盖用户自行修改的配置。

数值须能在运行时 JSON 数值表示中无损往返；例如 `9007199254740993` 或 `1.0000000000000001` 会明确报错，不会静默改值。高精度金额或长数字编号应将字段配置为 `string`。同样的检查适用于 Schema 中的数值约束与枚举。

### 达到运行执行次数上限后继续

平台仍在有限次数处停止自动推进。先检查完整命令回执、原生交接及历次返工，确认有明确的新处理动作；在运行页填写回退原因、目标节点和新的总执行上限。授权用户也可调用 `POST /api/workflow-runs/{id}/return`，提交 `seq`、`target`、`summary` 和 `max_steps`。仅在原预算耗尽时接受更大上限，最多1000次；不自动追加，也不重放已结束的命令。普通中断继续使用原resume/stop/return语义。

新的运行预算与冻结图分别保存。原图、节点结果、Connector回执及工作区保留，新节点记录调用者、预算变化和检查原因；旧seq重复请求拒绝，新上限耗尽仍停止。首次失败发生在新节点时仍须走完整tests/独立QA，不把恢复当放行。

服务按正常构建、备份、无在途执行检查及二进制升级获得该入口；启动时自动创建Run预算存储，重复启动不改旧记录，旧Run初始上限仍取自己的冻结配置。不得手改数据库或旧图。研发交付模板的新安装默认100次，已有安装沿原manifest加`--upgrade`同步，仍检查配置漂移及在途执行；模板升级不追加已有Run预算，已有Run只能经上述显式受支持入口检查后恢复。

PR默认标题取所选正文文件第一条非空行的一级标题（`# 实际交付目标`），便于交付范围变化后保持标题与正文一致；显式Connector标题仍优先，没有一级标题的旧正文保持任务标题。正文原样发布，平台不改写产品结论。报告应区分编写时的验证事实与后续Git发布，不将报告时待执行的步骤写成发布后仍然成立的当前状态。
