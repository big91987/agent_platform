# 在另一个仓库复现 GitHub → Agent Platform 流程

本目录包含工作流模板、接入脚本、固定版本的验证运行时、安装入口和可选部署示例。除 Skill、目标项目及常规软件依赖外，不需要另行检出 `he_skeleton` 或 `reading_list`。平台必须已运行；这不是把 Agent 平台装进 GitHub Actions 的方案。

## 适用范围

当前完整示例面向同一台 macOS 上的 Agent Platform、Codex 和自托管 Runner，默认分支 `main`，产品为 `app/` 下的静态 HTML/CSS/JS，使用 Node 单元测试和真实 Chromium 回归。GitHub 仓库 Owner 发起流程。后台服务、构建型前端、其他分支或操作系统需要适配启动、检查和产物规则；本示例不宣称支持任意项目。

平台负责认证、会话、调度、原生执行和展示。此目录负责研发阶段、GitHub 路由、交接和检查策略。不要把这些业务阶段加进平台内核。

## 文件与依赖清单

应检出整个 `agent_platform` 仓库，不能只复制 `examples/github` 到另一台机器：

| 内容 | 本仓库位置 | 接入时怎么用 |
|---|---|---|
| 通用平台 | `cmd/`、`internal/`、`web/` | 宿主运行平台服务 |
| Python SDK | `sdk/python/` | 安装器自动装进 Runner 虚拟环境 |
| 交接、追加、撤回 MCP | `examples/pipeline-tool/` | 安装器构建；setup 注册到阶段 Agent |
| 阶段调度、QA 回退、PR 同步和审查 | `examples/github/*.py`、两个工作流 YAML | 宿主运行脚本；YAML 复制到目标仓库 |
| 浏览器和 Python 质量运行时 | `examples/github/tooling/` | 已随附，默认使用此固定快照 |
| JS 格式与 lint | `examples/github/quality/` | 安装器按锁文件安装 |
| Issue 与项目指令模板 | `examples/github/templates/` | 审查后复制或合并到目标项目 |
| 可选本机部署 | `examples/github/local-preview/` | 按其说明独立安装 |
| 安装入口 | `examples/github/install-tooling.sh`、`scripts/setup-runner.sh` | 都在本仓库，不需要额外脚手架仓库 |

仓库外仍需准备运行软件、Runner 注册、模型登录/凭据、Skill 资产和目标项目自身的代码/验收计划。它们不是遗漏的私有业务脚本。具体阶段 Skill 列表由 `setup.py` 配置，缺少时需安装资产或调整该列表。

## 1. 安装平台与工具

安装 Go 1.25+、C 编译器、Python 3.10+、Node/npm、Git、GitHub CLI，以及完成登录的 Codex CLI。可选本机部署需要支持 `tarfile` 提取过滤器的 Python（推荐 3.12+）。

```sh
git clone https://github.com/big91987/agent_platform.git
cd agent_platform
bash scripts/start.sh
bash examples/github/install-tooling.sh
```

工具安装脚本安装本仓库 Python SDK、构建 `pipeline-tool`、执行锁定版本的 npm 安装，并安装/探测 Chromium。首次需要网络。不会注册 Agent、发起 Issue 或部署产品。安装后可运行 `python3 examples/github/smoke-portability.py`，它在临时产品中通过 stdio MCP 实测浏览器和质量检查，不调用 GitHub 或平台 API。Python 产品校验还需安装与 `tooling/full_harness/ruff.toml` 兼容的 Ruff。

`tooling/` 是随仓库提供的固定源代码快照，不复制凭据或运行缓存。来源见 `tooling/SOURCE.json` 和 `tooling/NOTICE.md`。所有依赖安装目录均被忽略。

## 2. 准备目标项目

先准备目标项目的持久 Git clone，配置 origin 和 Git 提交身份。Runner 从这里 fetch main、为每个 Issue 创建独立 worktree；不要使用会被 CI 清理的临时 checkout。

目标项目应具备：

- `app/index.html`、产品 JS/CSS 及现有产品功能。
- 项目自己的 `AGENTS.md`；可参考 [模板](templates/AGENTS.example.md)，合并到现有规则，不直接覆盖。
- `docs/README.md`：项目和任务材料入口。按阶段建立 `docs/01-architecture/tasks/`、`docs/04-implementation/tasks/`、`docs/05-validation/tasks/`；由 Agent 按实际任务补齐文件。
- `tests/browser/core.json`：本项目真实核心用户旅程。动作协议来自随附 `tooling/full_harness/browser/check.cjs`。不要复制读书清单动作到其他产品；`.harness/reading-core.json` 仅为旧项目兼容回退路径。
- 可选 `tests/*.test.cjs`、Python 测试和 `.trellis/spec/` 项目规范。

新增功能计划由研发阶段写到 `docs/05-validation/tasks/<issue>/browser-plan.json`。可执行的质量门禁在 `verify.py`，包括格式、lint、JS 语法、核心/新增功能浏览器计划及已有测试。切换项目技术栈时必须调整它，同时检查 `pipeline.py` 中允许发布的产品目录；当前不允许将任意目录默认提交。

## 3. 配置身份与仓库隔离

在平台创建调用用户并生成 Token，创建一个获该用户授权的需求 Agent 作为初始配置。Model、Skill 和运行授权在平台配置；Skill 作为外部资产提供，不在本接入包内复制。

每个接入仓库使用一份独立的私有 Runner JSON，例如 `.data/project-a-runner.json`，内容结构如下（替换占位符）：

```json
{
  "base_url": "http://127.0.0.1:8788",
  "token": "<platform-user-token>",
  "agent_id": "<initial-requirements-agent-id>",
  "pipeline": {
    "repository": "owner/project-a",
    "namespace": "owner-project-a",
    "checkout": "<persistent-product-clone>",
    "workspaces": "<project-a-task-workspace-root>",
    "registry": "<project-a-private-state-root>",
    "delivery_token_file": "<private-github-token-file>",
    "python": "<platform-root>/.data/runner-venv/bin/python",
    "skills_root": "<skill-assets-root>",
    "agents": {}
  }
}
```

- `checkout` 现在只需目标产品仓库，不需要其中有 `full_harness/`。
- `namespace` 为每个仓库使用不同值，隔离注册的工具 ID 和 Agent 显示名称。初始需求 Agent 也应每个仓库独立创建，不共用同一个 ID。
- `registry`、`workspaces`、配置文件和凭据不要跨仓库共用。Issue 编号只在各仓库自己的状态目录中唯一。
- 配置和凭据文件权限设为 `0600`，放在任务工作区之外，不能提交。Runner 和平台在同一用户下可访问这些路径。
- 用 `gh auth login` 配置宿主 GitHub 身份。交接工具在宿主读取它；交付脚本从 `delivery_token_file` 读取无人值守凭据。权限需覆盖目标仓库的 Actions dispatch、Issue 回复、内容/PR 写入和状态写入。不要把 Token 传给模型或写进 YAML。
- 不配置 `browser_source` 时默认使用本目录 `tooling/`；高级用户可显式指向自己审查过的完整 Harness 工具快照，不能指向任务中可被 Agent 修改的工具代码。

```sh
python3 examples/github/setup.py --config '<private-runner-json>'
```

该命令提示平台管理员密码，注册外部 MCP、设置五个阶段 Agent 及研发 Stop Hook。当前阶段 Skill 名称清单见 `setup.py`。旧配置没有 namespace 时保留旧名称；新仓库应设置 namespace。已经创建的会话保留配置快照，更新绑定后用新 Issue 验证。

## 4. 安装目标仓库的 GitHub 配置

复制（已有文件时审查合并，不覆盖自定义逻辑）：

| 本目录文件 | 目标仓库路径 |
|---|---|
| `pipeline.yml` | `.github/workflows/agent-platform.yml` |
| `pr-refresh.yml` | `.github/workflows/pr-refresh.yml` |
| `templates/.github/ISSUE_TEMPLATE/agent-task.yml` | `.github/ISSUE_TEMPLATE/agent-task.yml` |

仓库 Actions Variables：

- `AGENT_PLATFORM_ROOT`：宿主平台代码绝对路径。
- `AGENT_PLATFORM_CONFIG`：这个仓库专用 Runner JSON 的绝对路径。留空仅用于兼容旧的 `.data/github-runner.json`，多仓库必须显式设置。

注册自托管 Runner，默认标签为 `self-hosted, macOS, ARM64, he-full`；其他环境修改两个 YAML 的 runs-on 和 PATH。保留 Owner 检查与 fork 隔离。默认 Owner 创建 Issue 自动触发；其他人的 Issue 由 Owner 用 workflow_dispatch 启动。仅复制模板不会自动注册 Runner。

## 5. 试运行与验收

1. Owner 用一句产品需求创建 Issue；可勾选“按推荐方案自主推进”。
2. Actions requirements Job 发布平台会话链接，登录后持续澄清。
3. Agent 调用外部 MCP 交接，下一次 Run 接收交接并启动下一阶段；同一 Issue 共用任务工作区，不靠恢复上一阶段会话传递材料。
4. 验证设计、研发、独立 QA；通过后生成草稿 PR，Issue 中可见链接。
5. 标记 PR ready 后，同步主线并按配置执行独立 QA/代码审查；合并由人决定。
6. 至少验证一次回退、网页接续、工具与 thinking 显示、结果回写、同一请求去重。不能把单元测试通过等同于新仓库的完整用户旅程已验收。

定位问题：Actions 看 Runner 调度/脚本错误；平台会话看原生执行；目标工作区的 `docs/05-validation/tasks/<issue>/` 看浏览器与质量证据。

## 6. 可选：合并后本机预览部署

见 [local-preview/README.md](local-preview/README.md)。该目录含 YAML、安装器、部署控制器和测试，独立于研发交接流程；无需从 reading_list 再复制源码。静态站点部署不是后端数据库迁移框架。

## 升级和已知限制

模板复制到业务仓库后不会自动更新；升级时对照 diff 同步。可信工具固定随平台版本更新；独立 `browser_source` 覆盖项需自行升级。部署控制器安装时复制到运行目录，源码更新后要重新执行安装器，单改 YAML 不会更新已安装的控制器。

本次整理不迁移现有 reading_list 配置或重启其工作流。其旧 `browser_source`、私有配置和已安装部署控制器仍按原值运行。平台已经能隔离配置与任务目录，但这些是受信的同机执行，不提供容器或恶意代码沙箱隔离。

## 本次打包验证记录（2026-10-04）

- `install-tooling.sh`：SDK、MCP 二进制、锁定 npm 依赖安装通过，Chromium 启动和渲染探测通过。
- `smoke-portability.py`：临时产品无外部 Harness checkout，经 stdio MCP 完成 check 和 verify；3 次真实浏览器检查通过，生成 6 张截图。
- `scripts/verify.sh`：前端 10 项、SDK 6 项、GitHub 适配器 61 项、部署 8 项通过，Ruff、Go vet/race tests/build 通过。
- 注册测试覆盖两个独立仓库与旧配置共存、重复注册不改变 Agent ID；随附 9 个源文件的 SHA-256 与清单一致；发布前脱敏检查通过。

尚未用这份打包版本在另一个 GitHub 仓库完成需求到 PR 的全阶段运行。新项目上线前按第 5 节验证，不将上述本机验证当成该项目的端到端验收。

## 验证工具不足时

参见 [补充浏览器验证](BROWSER_CHECKS.md)：原生缩放、可访问性树及 Agent 任务级页面断言，含旧安装升级方式。

## 已关闭任务的迟到交接

PR 合并可能自动关闭 Issue，而 Agent 的当轮工作尚未结束。已登记任务的后续 Run 发现 Issue 已关闭时，会在 Actions Summary 明确记录跳过，不再启动 Agent 或发布 PR，也不修改此前的 QA／集成检查状态。等待 Agent 后再次检查 Issue，避免等待期间关闭任务造成同样错误。未登记的已关闭 Issue 仍不允许启动新任务。

Run 正常结束只表示迟到请求已处理，并不代表新的 QA 放行。合并前的必需检查和部署版本校验仍需独立执行，不能用此跳过结果作为部署依据。


## PR 交付说明

新建 PR 必须让未参与讨论的审阅者直接看懂交付，无需先回 Issue 拼接上下文。
QA 在交付时整理 `docs/05-validation/tasks/<issue>/pull-request.md` 并附入交接 artifacts：

- 第一行 `# 具体功能标题`；不使用“实现 Issue #编号”作为标题。
- `## 背景`：用户问题和本次目的。
- `## 实现内容`：实际交付的行为、关键规则及必要的范围说明。
- `## 验证结果`：真实执行的检查、结果与证据，明确未测项。
- `## 风险与限制`：已知限制、兼容性或数据影响；确无已知限制也需明确说明。
- `## 界面效果`：UI 变化的截图或预览入口；无 UI 变化则注明不适用及原因。

正文根据需求基线、最终代码和 QA 证据编写，不直接复制一句 Issue 标题或整个 PRD。
链接须使用仓库中可访问的 GitHub 地址，不能放本机文件路径或凭据。
发布脚本在提交/推送前检查标题与必需章节，读取这份内容创建草稿 PR，并补充 Issue 和现有验证文档链接。
格式检查不能代替对内容真实性的审查。缺少文档时 QA 补齐后重试，不回退为固定套话。
已有 PR 不自动改写，也不因新文档契约阻塞重跑。后续 Ready 集成与独立审查仍以当前提交的检查状态为准。

### 需要访问外部资料的任务

在平台的 Agent 配置中为需要联网的阶段勾选“允许联网”。也可保留联网关闭、勾选“权限不足时允许申请管理员审批”，由原生 Agent 发起一次性提权请求，管理员在会话页面批准或拒绝。自主推进参数只管理业务决策，不授予网络或主机权限。已有会话需要在空闲时从会话信息侧栏应用 Agent 当前权限，再接续原会话；不要复制会话或手改原生配置绕过审批。
