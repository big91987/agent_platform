# 平台内研发交付示例

本例把研发流程保存在 Agent Platform 的图中，由持久 Run 推进。GitHub 只保存 Issue、代码和草稿 PR，不启动 CI 编排。平台本身不包含“需求／设计／研发”等业务阶段；这些是本目录提供的可编辑模板。

## 组成与边界

- `software-delivery.json`：准备分支 → Issue → 需求 → 设计 → 研发 → 项目测试 → 独立 QA → 交付说明 → 提交推送 → 草稿 PR。测试失败回研发；QA 可回需求、设计或研发；关键问题进入人工选择节点。所有回路保留在同一个 Run 中。
- `qa-rework.json`：从已有产物的独立 QA 开始，发现问题返回需求／设计／研发，修复后重新测试和 QA；不创建 Issue、分支或 PR，最后由验收人员选择结束或继续返工。
- `prompts/`：阶段指令，复用 Harness Skill，按项目规模裁剪。普通选择记录推荐并继续；用户明确要求、关键歧义和高风险操作才等待确认。阶段不适用时说明依据再交接。
- `install.py`：使用公开管理 API 注册 Agent、stdio MCP、Connector 和图；安装清单用于重入与升级，不操作数据库。
- `repository.py`：固定的准备分支／提交推送命令。每个 Run 使用 `workflow/<run_id>` 分支；拒绝接管脏工作区、错误仓库、错误分支和明显凭据文件；不 reset、不强推、不合并。
- `browser_tool.py`：复用 `../github/tooling/full_harness/browser` 的锁定浏览器运行时；只借用浏览器能力，不使用其 CI 控制器。检查 app 或设计原型，保存真实截图／检查结果；支持页面内受限检查脚本。

当前示例适合带 `npm test` 入口的静态 Web 小项目，浏览器工具本地服务根为 `app/` 或 `docs/workflow/prototype/`。其他技术栈应修改图中管理员配置的测试命令与注册工具，不改平台引擎。浏览器可验证原生缩放和可访问树，不能把这些称为真实读屏器语音验收。Agent 需对未测项明确说明。

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
python3 -m unittest discover -s examples/platform-workflows -p '*_test.py' -v
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
